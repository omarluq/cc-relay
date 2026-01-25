package router

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/omarluq/cc-relay/internal/providers"
)

// mockProvider implements providers.Provider for testing.
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string                                 { return m.name }
func (m *mockProvider) BaseURL() string                              { return "http://test" }
func (m *mockProvider) Owner() string                                { return "test" }
func (m *mockProvider) Authenticate(_ *http.Request, _ string) error { return nil }
func (m *mockProvider) ForwardHeaders(_ http.Header) http.Header     { return http.Header{} }
func (m *mockProvider) SupportsStreaming() bool                      { return true }
func (m *mockProvider) SupportsTransparentAuth() bool                { return false }
func (m *mockProvider) ListModels() []providers.Model                { return nil }
func (m *mockProvider) GetModelMapping() map[string]string           { return nil }
func (m *mockProvider) MapModel(model string) string                 { return model }

func (m *mockProvider) TransformRequest(body []byte, endpoint string) (newBody []byte, targetURL string, err error) {
	return body, "http://test" + endpoint, nil
}
func (m *mockProvider) TransformResponse(_ *http.Response, _ http.ResponseWriter) error { return nil }
func (m *mockProvider) RequiresBodyTransform() bool                                      { return false }
func (m *mockProvider) StreamingContentType() string {
	return providers.ContentTypeSSE
}

func TestWeightedRoundRobinRouter_Select_NoProviders(t *testing.T) {
	t.Parallel()

	r := NewWeightedRoundRobinRouter()
	_, err := r.Select(context.Background(), []ProviderInfo{})

	if !errors.Is(err, ErrNoProviders) {
		t.Errorf("Select() error = %v, want ErrNoProviders", err)
	}
}

func TestWeightedRoundRobinRouter_Select_AllUnhealthy(t *testing.T) {
	t.Parallel()

	r := NewWeightedRoundRobinRouter()
	infos := []ProviderInfo{
		{Provider: &mockProvider{"a"}, IsHealthy: func() bool { return false }, Weight: 1},
		{Provider: &mockProvider{"b"}, IsHealthy: func() bool { return false }, Weight: 1},
	}

	_, err := r.Select(context.Background(), infos)

	if !errors.Is(err, ErrAllProvidersUnhealthy) {
		t.Errorf("Select() error = %v, want ErrAllProvidersUnhealthy", err)
	}
}

func TestWeightedRoundRobinRouter_Select_EqualWeights(t *testing.T) {
	t.Parallel()

	r := NewWeightedRoundRobinRouter()
	infos := []ProviderInfo{
		{Provider: &mockProvider{"a"}, IsHealthy: func() bool { return true }, Weight: 1},
		{Provider: &mockProvider{"b"}, IsHealthy: func() bool { return true }, Weight: 1},
		{Provider: &mockProvider{"c"}, IsHealthy: func() bool { return true }, Weight: 1},
	}

	// With equal weights, distribution should be even
	counts := map[string]int{"a": 0, "b": 0, "c": 0}
	iterations := 300 // Exact multiple of 3 for perfect distribution

	for i := 0; i < iterations; i++ {
		p, err := r.Select(context.Background(), infos)
		if err != nil {
			t.Fatalf("Select() unexpected error: %v", err)
		}
		counts[p.Provider.Name()]++
	}

	// With Nginx smooth algorithm, equal weights should give exactly equal distribution
	expected := iterations / 3
	for name, count := range counts {
		if count != expected {
			t.Errorf("Provider %s selected %d times, want exactly %d", name, count, expected)
		}
	}
}

func TestWeightedRoundRobinRouter_Select_ProportionalWeights(t *testing.T) {
	t.Parallel()

	r := NewWeightedRoundRobinRouter()
	// A:3, B:2, C:1 = total 6
	// Expected: A~50%, B~33%, C~17%
	infos := []ProviderInfo{
		{Provider: &mockProvider{"a"}, IsHealthy: func() bool { return true }, Weight: 3},
		{Provider: &mockProvider{"b"}, IsHealthy: func() bool { return true }, Weight: 2},
		{Provider: &mockProvider{"c"}, IsHealthy: func() bool { return true }, Weight: 1},
	}

	counts := map[string]int{"a": 0, "b": 0, "c": 0}
	iterations := 600 // Exact multiple of total weight (6) for perfect distribution

	for i := 0; i < iterations; i++ {
		p, err := r.Select(context.Background(), infos)
		if err != nil {
			t.Fatalf("Select() unexpected error: %v", err)
		}
		counts[p.Provider.Name()]++
	}

	// With Nginx smooth algorithm, distribution should be exactly proportional
	// A: 3/6 * 600 = 300
	// B: 2/6 * 600 = 200
	// C: 1/6 * 600 = 100
	expectedA := 300
	expectedB := 200
	expectedC := 100

	if counts["a"] != expectedA {
		t.Errorf("Provider A selected %d times, want exactly %d", counts["a"], expectedA)
	}
	if counts["b"] != expectedB {
		t.Errorf("Provider B selected %d times, want exactly %d", counts["b"], expectedB)
	}
	if counts["c"] != expectedC {
		t.Errorf("Provider C selected %d times, want exactly %d", counts["c"], expectedC)
	}
}

func TestWeightedRoundRobinRouter_Select_DefaultWeight(t *testing.T) {
	t.Parallel()

	r := NewWeightedRoundRobinRouter()
	// Provider with Weight=0 should get default weight of 1
	infos := []ProviderInfo{
		{Provider: &mockProvider{"a"}, IsHealthy: func() bool { return true }, Weight: 2},
		{Provider: &mockProvider{"b"}, IsHealthy: func() bool { return true }, Weight: 0}, // Default to 1
	}

	counts := map[string]int{"a": 0, "b": 0}
	iterations := 300 // Multiple of total weight (3)

	for i := 0; i < iterations; i++ {
		p, err := r.Select(context.Background(), infos)
		if err != nil {
			t.Fatalf("Select() unexpected error: %v", err)
		}
		counts[p.Provider.Name()]++
	}

	// A: 2/3 * 300 = 200, B: 1/3 * 300 = 100
	expectedA := 200
	expectedB := 100

	if counts["a"] != expectedA {
		t.Errorf("Provider A selected %d times, want exactly %d", counts["a"], expectedA)
	}
	if counts["b"] != expectedB {
		t.Errorf("Provider B (weight=0 -> default 1) selected %d times, want exactly %d", counts["b"], expectedB)
	}
}

func TestWeightedRoundRobinRouter_Select_NegativeWeight(t *testing.T) {
	t.Parallel()

	r := NewWeightedRoundRobinRouter()
	// Provider with negative weight should get default weight of 1
	infos := []ProviderInfo{
		{Provider: &mockProvider{"a"}, IsHealthy: func() bool { return true }, Weight: 2},
		{Provider: &mockProvider{"b"}, IsHealthy: func() bool { return true }, Weight: -5}, // Default to 1
	}

	counts := map[string]int{"a": 0, "b": 0}
	iterations := 300

	for i := 0; i < iterations; i++ {
		p, err := r.Select(context.Background(), infos)
		if err != nil {
			t.Fatalf("Select() unexpected error: %v", err)
		}
		counts[p.Provider.Name()]++
	}

	// A: 2/3 * 300 = 200, B: 1/3 * 300 = 100
	expectedA := 200
	expectedB := 100

	if counts["a"] != expectedA {
		t.Errorf("Provider A selected %d times, want exactly %d", counts["a"], expectedA)
	}
	if counts["b"] != expectedB {
		t.Errorf("Provider B (negative weight -> default 1) selected %d times, want exactly %d", counts["b"], expectedB)
	}
}

func TestWeightedRoundRobinRouter_Select_SkipsUnhealthy(t *testing.T) {
	t.Parallel()

	r := NewWeightedRoundRobinRouter()
	// A:3 healthy, B:2 unhealthy, C:1 healthy
	// Only A and C should be selected, proportional to their weights (3:1)
	infos := []ProviderInfo{
		{Provider: &mockProvider{"a"}, IsHealthy: func() bool { return true }, Weight: 3},
		{Provider: &mockProvider{"b"}, IsHealthy: func() bool { return false }, Weight: 2},
		{Provider: &mockProvider{"c"}, IsHealthy: func() bool { return true }, Weight: 1},
	}

	counts := map[string]int{"a": 0, "b": 0, "c": 0}
	iterations := 400 // Multiple of 4 (3+1)

	for i := 0; i < iterations; i++ {
		p, err := r.Select(context.Background(), infos)
		if err != nil {
			t.Fatalf("Select() unexpected error: %v", err)
		}
		counts[p.Provider.Name()]++
	}

	// B should never be selected
	if counts["b"] != 0 {
		t.Errorf("Unhealthy provider B selected %d times, want 0", counts["b"])
	}

	// A: 3/4 * 400 = 300, C: 1/4 * 400 = 100
	expectedA := 300
	expectedC := 100

	if counts["a"] != expectedA {
		t.Errorf("Provider A selected %d times, want exactly %d", counts["a"], expectedA)
	}
	if counts["c"] != expectedC {
		t.Errorf("Provider C selected %d times, want exactly %d", counts["c"], expectedC)
	}
}

func TestWeightedRoundRobinRouter_Select_ReinitializesOnProviderChange(t *testing.T) {
	t.Parallel()

	r := NewWeightedRoundRobinRouter()

	// Initial providers
	infos1 := []ProviderInfo{
		{Provider: &mockProvider{"a"}, IsHealthy: func() bool { return true }, Weight: 1},
		{Provider: &mockProvider{"b"}, IsHealthy: func() bool { return true }, Weight: 1},
	}

	// Make some selections to build up state
	for i := 0; i < 10; i++ {
		_, err := r.Select(context.Background(), infos1)
		if err != nil {
			t.Fatalf("Select() unexpected error: %v", err)
		}
	}

	// Change provider list
	infos2 := []ProviderInfo{
		{Provider: &mockProvider{"c"}, IsHealthy: func() bool { return true }, Weight: 1},
		{Provider: &mockProvider{"d"}, IsHealthy: func() bool { return true }, Weight: 1},
		{Provider: &mockProvider{"e"}, IsHealthy: func() bool { return true }, Weight: 1},
	}

	// Should work without issue - state reinitializes
	counts := map[string]int{"c": 0, "d": 0, "e": 0}
	iterations := 300

	for i := 0; i < iterations; i++ {
		p, err := r.Select(context.Background(), infos2)
		if err != nil {
			t.Fatalf("Select() unexpected error: %v", err)
		}
		counts[p.Provider.Name()]++
	}

	// Should have even distribution with new providers
	expected := iterations / 3
	for name, count := range counts {
		if count != expected {
			t.Errorf("Provider %s selected %d times, want exactly %d", name, count, expected)
		}
	}
}

func TestWeightedRoundRobinRouter_Select_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	r := NewWeightedRoundRobinRouter()
	infos := []ProviderInfo{
		{Provider: &mockProvider{"a"}, IsHealthy: func() bool { return true }, Weight: 2},
		{Provider: &mockProvider{"b"}, IsHealthy: func() bool { return true }, Weight: 1},
	}

	var wg sync.WaitGroup
	goroutines := 10
	selectionsPerGoroutine := 100

	mu := sync.Mutex{}
	counts := map[string]int{"a": 0, "b": 0}

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < selectionsPerGoroutine; j++ {
				p, err := r.Select(context.Background(), infos)
				if err != nil {
					t.Errorf("Select() unexpected error: %v", err)
					return
				}
				mu.Lock()
				counts[p.Provider.Name()]++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	total := goroutines * selectionsPerGoroutine
	if counts["a"]+counts["b"] != total {
		t.Errorf("Total selections = %d, want %d", counts["a"]+counts["b"], total)
	}

	// Verify approximate proportions (allowing some variance due to concurrent execution)
	// A should be roughly 2/3, B roughly 1/3
	expectedAMin := int(float64(total) * 0.5) // At least 50% for A (relaxed tolerance)
	expectedAMax := int(float64(total) * 0.8) // At most 80% for A

	if counts["a"] < expectedAMin || counts["a"] > expectedAMax {
		t.Errorf("Provider A selected %d times (%.1f%%), want between 50%%-80%% of %d",
			counts["a"], float64(counts["a"])/float64(total)*100, total)
	}
}

func TestWeightedRoundRobinRouter_Select_SmoothDistribution(t *testing.T) {
	t.Parallel()

	// Test that the Nginx smooth algorithm produces even distribution
	// rather than clustering (e.g., not AAABBC pattern but ABACAB pattern)
	r := NewWeightedRoundRobinRouter()
	infos := []ProviderInfo{
		{Provider: &mockProvider{"a"}, IsHealthy: func() bool { return true }, Weight: 2},
		{Provider: &mockProvider{"b"}, IsHealthy: func() bool { return true }, Weight: 1},
	}

	// Get 6 selections - should see smooth pattern
	selections := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		p, err := r.Select(context.Background(), infos)
		if err != nil {
			t.Fatalf("Select() unexpected error: %v", err)
		}
		selections = append(selections, p.Provider.Name())
	}

	// Count consecutive same provider selections (clustering)
	maxConsecutive := 1
	currentConsecutive := 1
	for i := 1; i < len(selections); i++ {
		if selections[i] == selections[i-1] {
			currentConsecutive++
			if currentConsecutive > maxConsecutive {
				maxConsecutive = currentConsecutive
			}
		} else {
			currentConsecutive = 1
		}
	}

	// Smooth algorithm should not have more than 2 consecutive same selections
	// for this weight distribution (2:1)
	if maxConsecutive > 2 {
		t.Errorf("Distribution not smooth: got %v with max %d consecutive same selections",
			selections, maxConsecutive)
	}
}

func TestWeightedRoundRobinRouter_Select_SingleProvider(t *testing.T) {
	t.Parallel()

	r := NewWeightedRoundRobinRouter()
	infos := []ProviderInfo{
		{Provider: &mockProvider{"only"}, IsHealthy: func() bool { return true }, Weight: 5},
	}

	for i := 0; i < 10; i++ {
		p, err := r.Select(context.Background(), infos)
		if err != nil {
			t.Fatalf("Select() unexpected error: %v", err)
		}
		if p.Provider.Name() != "only" {
			t.Errorf("Select() = %s, want 'only'", p.Provider.Name())
		}
	}
}

func TestWeightedRoundRobinRouter_Name(t *testing.T) {
	t.Parallel()

	r := NewWeightedRoundRobinRouter()
	if r.Name() != StrategyWeightedRoundRobin {
		t.Errorf("Name() = %q, want %q", r.Name(), StrategyWeightedRoundRobin)
	}
}

func TestGetEffectiveWeight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		weight   int
		expected int
	}{
		{"positive weight", 5, 5},
		{"weight of 1", 1, 1},
		{"zero weight defaults to 1", 0, 1},
		{"negative weight defaults to 1", -3, 1},
		{"large weight", 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := ProviderInfo{Weight: tt.weight}
			if got := getEffectiveWeight(p); got != tt.expected {
				t.Errorf("getEffectiveWeight() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestStringSliceEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{"both empty", []string{}, []string{}, true},
		{"both nil", nil, nil, true},
		{"equal single", []string{"a"}, []string{"a"}, true},
		{"equal multiple", []string{"a", "b", "c"}, []string{"a", "b", "c"}, true},
		{"different length", []string{"a"}, []string{"a", "b"}, false},
		{"different content", []string{"a", "b"}, []string{"a", "c"}, false},
		{"different order", []string{"a", "b"}, []string{"b", "a"}, false},
		{"one empty one not", []string{}, []string{"a"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := stringSliceEqual(tt.a, tt.b); got != tt.expected {
				t.Errorf("stringSliceEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

// Verify interface compliance at compile time.
var _ ProviderRouter = (*WeightedRoundRobinRouter)(nil)
