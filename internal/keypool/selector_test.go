package keypool

import (
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestNewKeyMetadata verifies key metadata initialization.
func TestNewKeyMetadata(t *testing.T) {
	apiKey := "sk-ant-test-key-12345"
	rpm := 50
	itpm := 30000
	otpm := 30000

	key := NewKeyMetadata(apiKey, rpm, itpm, otpm)

	if key.APIKey != apiKey {
		t.Errorf("APIKey = %q, want %q", key.APIKey, apiKey)
	}

	if key.RPMLimit != rpm {
		t.Errorf("RPMLimit = %d, want %d", key.RPMLimit, rpm)
	}

	if key.ITPMLimit != itpm {
		t.Errorf("ITPMLimit = %d, want %d", key.ITPMLimit, itpm)
	}

	if key.OTPMLimit != otpm {
		t.Errorf("OTPMLimit = %d, want %d", key.OTPMLimit, otpm)
	}

	// Should start at full capacity
	if key.RPMRemaining != rpm {
		t.Errorf("RPMRemaining = %d, want %d", key.RPMRemaining, rpm)
	}

	if key.ITPMRemaining != itpm {
		t.Errorf("ITPMRemaining = %d, want %d", key.ITPMRemaining, itpm)
	}

	if key.OTPMRemaining != otpm {
		t.Errorf("OTPMRemaining = %d, want %d", key.OTPMRemaining, otpm)
	}

	// Should be healthy
	if !key.Healthy {
		t.Error("Healthy = false, want true")
	}

	// Should have normal priority
	if key.Priority != 1 {
		t.Errorf("Priority = %d, want 1", key.Priority)
	}

	// ID should be 8 character hex string
	if len(key.ID) != 8 {
		t.Errorf("ID length = %d, want 8", len(key.ID))
	}
}

// TestGetCapacityScore verifies capacity score calculation.
func TestGetCapacityScore(t *testing.T) {
	tests := []struct {
		name          string
		rpm           int
		rpmRemaining  int
		itpm          int
		itpmRemaining int
		otpm          int
		otpmRemaining int
		healthy       bool
		cooldown      bool
		wantScore     float64
	}{
		{
			name:          "full capacity",
			rpm:           50,
			rpmRemaining:  50,
			itpm:          30000,
			itpmRemaining: 30000,
			otpm:          30000,
			otpmRemaining: 30000,
			healthy:       true,
			cooldown:      false,
			wantScore:     1.0,
		},
		{
			name:          "half capacity",
			rpm:           50,
			rpmRemaining:  25,
			itpm:          30000,
			itpmRemaining: 15000,
			otpm:          30000,
			otpmRemaining: 15000,
			healthy:       true,
			cooldown:      false,
			wantScore:     0.5,
		},
		{
			name:          "quarter capacity",
			rpm:           50,
			rpmRemaining:  12,
			itpm:          30000,
			itpmRemaining: 7500,
			otpm:          30000,
			otpmRemaining: 7500,
			healthy:       true,
			cooldown:      false,
			wantScore:     0.25,
		},
		{
			name:          "unhealthy key",
			rpm:           50,
			rpmRemaining:  50,
			itpm:          30000,
			itpmRemaining: 30000,
			otpm:          30000,
			otpmRemaining: 30000,
			healthy:       false,
			cooldown:      false,
			wantScore:     0.0,
		},
		{
			name:          "key in cooldown",
			rpm:           50,
			rpmRemaining:  50,
			itpm:          30000,
			itpmRemaining: 30000,
			otpm:          30000,
			otpmRemaining: 30000,
			healthy:       true,
			cooldown:      true,
			wantScore:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := NewKeyMetadata("test-key", tt.rpm, tt.itpm, tt.otpm)
			key.RPMRemaining = tt.rpmRemaining
			key.ITPMRemaining = tt.itpmRemaining
			key.OTPMRemaining = tt.otpmRemaining
			key.Healthy = tt.healthy

			if tt.cooldown {
				key.CooldownUntil = time.Now().Add(1 * time.Minute)
			}

			score := key.GetCapacityScore()
			// Allow small floating point error tolerance
			epsilon := 0.01
			if diff := score - tt.wantScore; diff < -epsilon || diff > epsilon {
				t.Errorf("GetCapacityScore() = %f, want %f (diff=%f)", score, tt.wantScore, diff)
			}
		})
	}
}

// TestIsAvailable verifies availability checks.
func TestIsAvailable(t *testing.T) {
	tests := []struct {
		cooldownUntil time.Time
		name          string
		healthy       bool
		want          bool
	}{
		{
			name:          "healthy and no cooldown",
			healthy:       true,
			cooldownUntil: time.Time{},
			want:          true,
		},
		{
			name:          "unhealthy",
			healthy:       false,
			cooldownUntil: time.Time{},
			want:          false,
		},
		{
			name:          "in cooldown",
			healthy:       true,
			cooldownUntil: time.Now().Add(1 * time.Minute),
			want:          false,
		},
		{
			name:          "cooldown expired",
			healthy:       true,
			cooldownUntil: time.Now().Add(-1 * time.Minute),
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := NewKeyMetadata("test-key", 50, 30000, 30000)
			key.Healthy = tt.healthy
			key.CooldownUntil = tt.cooldownUntil

			got := key.IsAvailable()
			if got != tt.want {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestUpdateFromHeaders verifies response header parsing.
func TestUpdateFromHeaders(t *testing.T) {
	key := NewKeyMetadata("test-key", 50, 30000, 30000)

	headers := http.Header{}
	headers.Set("anthropic-ratelimit-requests-limit", "100")
	headers.Set("anthropic-ratelimit-requests-remaining", "75")
	headers.Set("anthropic-ratelimit-requests-reset", "2026-01-21T19:42:00Z")
	headers.Set("anthropic-ratelimit-input-tokens-limit", "40000")
	headers.Set("anthropic-ratelimit-input-tokens-remaining", "35000")
	headers.Set("anthropic-ratelimit-input-tokens-reset", "2026-01-21T19:42:00Z")
	headers.Set("anthropic-ratelimit-output-tokens-limit", "40000")
	headers.Set("anthropic-ratelimit-output-tokens-remaining", "35000")
	headers.Set("anthropic-ratelimit-output-tokens-reset", "2026-01-21T19:42:00Z")

	err := key.UpdateFromHeaders(headers)
	if err != nil {
		t.Fatalf("UpdateFromHeaders() error = %v", err)
	}

	// Check updated limits
	if key.RPMLimit != 100 {
		t.Errorf("RPMLimit = %d, want 100", key.RPMLimit)
	}

	if key.RPMRemaining != 75 {
		t.Errorf("RPMRemaining = %d, want 75", key.RPMRemaining)
	}

	if key.ITPMLimit != 40000 {
		t.Errorf("ITPMLimit = %d, want 40000", key.ITPMLimit)
	}

	if key.ITPMRemaining != 35000 {
		t.Errorf("ITPMRemaining = %d, want 35000", key.ITPMRemaining)
	}

	if key.OTPMLimit != 40000 {
		t.Errorf("OTPMLimit = %d, want 40000", key.OTPMLimit)
	}

	if key.OTPMRemaining != 35000 {
		t.Errorf("OTPMRemaining = %d, want 35000", key.OTPMRemaining)
	}

	// Check reset time was parsed
	expectedReset, _ := time.Parse(time.RFC3339, "2026-01-21T19:42:00Z")
	if !key.RPMResetAt.Equal(expectedReset) {
		t.Errorf("RPMResetAt = %v, want %v", key.RPMResetAt, expectedReset)
	}
}

// TestUpdateFromHeadersPartial verifies handling of missing headers.
func TestUpdateFromHeadersPartial(t *testing.T) {
	key := NewKeyMetadata("test-key", 50, 30000, 30000)

	// Set only some headers
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-requests-remaining", "42")

	err := key.UpdateFromHeaders(headers)
	if err != nil {
		t.Fatalf("UpdateFromHeaders() error = %v", err)
	}

	// Original limit should be unchanged
	if key.RPMLimit != 50 {
		t.Errorf("RPMLimit = %d, want 50 (unchanged)", key.RPMLimit)
	}

	// Remaining should be updated
	if key.RPMRemaining != 42 {
		t.Errorf("RPMRemaining = %d, want 42", key.RPMRemaining)
	}
}

// TestUpdateFromHeadersInvalid verifies handling of invalid header values.
func TestUpdateFromHeadersInvalid(t *testing.T) {
	key := NewKeyMetadata("test-key", 50, 30000, 30000)

	headers := http.Header{}
	headers.Set("anthropic-ratelimit-requests-limit", "invalid")
	headers.Set("anthropic-ratelimit-requests-remaining", "-10")
	headers.Set("anthropic-ratelimit-requests-reset", "not-a-time")

	err := key.UpdateFromHeaders(headers)
	if err != nil {
		t.Fatalf("UpdateFromHeaders() error = %v", err)
	}

	// Invalid values should be ignored, original values preserved
	if key.RPMLimit != 50 {
		t.Errorf("RPMLimit = %d, want 50 (invalid ignored)", key.RPMLimit)
	}

	if key.RPMRemaining != 50 {
		t.Errorf("RPMRemaining = %d, want 50 (negative ignored)", key.RPMRemaining)
	}
}

// TestSetCooldown verifies cooldown setting.
func TestSetCooldown(t *testing.T) {
	key := NewKeyMetadata("test-key", 50, 30000, 30000)

	cooldownUntil := time.Now().Add(5 * time.Minute)
	key.SetCooldown(cooldownUntil)

	// Should not be available during cooldown
	if key.IsAvailable() {
		t.Error("IsAvailable() = true during cooldown, want false")
	}

	// Cooldown should be set correctly
	key.mu.RLock()
	if !key.CooldownUntil.Equal(cooldownUntil) {
		t.Errorf("CooldownUntil = %v, want %v", key.CooldownUntil, cooldownUntil)
	}
	key.mu.RUnlock()
}

// TestMarkUnhealthy verifies unhealthy marking.
func TestMarkUnhealthy(t *testing.T) {
	key := NewKeyMetadata("test-key", 50, 30000, 30000)

	testErr := http.ErrServerClosed
	key.MarkUnhealthy(testErr)

	if key.Healthy {
		t.Error("Healthy = true after MarkUnhealthy, want false")
	}

	if !errors.Is(key.LastError, testErr) {
		t.Errorf("LastError = %v, want %v", key.LastError, testErr)
	}

	if key.LastErrorAt.IsZero() {
		t.Error("LastErrorAt not set after MarkUnhealthy")
	}

	// Should not be available when unhealthy
	if key.IsAvailable() {
		t.Error("IsAvailable() = true when unhealthy, want false")
	}
}

// TestMarkHealthy verifies health recovery.
func TestMarkHealthy(t *testing.T) {
	key := NewKeyMetadata("test-key", 50, 30000, 30000)

	// Mark unhealthy first
	key.MarkUnhealthy(http.ErrServerClosed)

	// Then mark healthy
	key.MarkHealthy()

	if !key.Healthy {
		t.Error("Healthy = false after MarkHealthy, want true")
	}

	if key.LastError != nil {
		t.Errorf("LastError = %v after MarkHealthy, want nil", key.LastError)
	}

	// Should be available again
	if !key.IsAvailable() {
		t.Error("IsAvailable() = false after MarkHealthy, want true")
	}
}

// TestLeastLoadedSelector verifies least-loaded selection strategy.
func TestLeastLoadedSelector(t *testing.T) {
	selector := NewLeastLoadedSelector()

	if selector.Name() != StrategyLeastLoaded {
		t.Errorf("Name() = %q, want %q", selector.Name(), StrategyLeastLoaded)
	}

	// Create keys with different capacity levels
	keys := []*KeyMetadata{
		NewKeyMetadata("key1", 50, 30000, 30000), // Full capacity
		NewKeyMetadata("key2", 50, 30000, 30000), // Will set to half
		NewKeyMetadata("key3", 50, 30000, 30000), // Will set to quarter
	}

	// Set different remaining capacities
	keys[1].RPMRemaining = 25
	keys[1].ITPMRemaining = 15000
	keys[1].OTPMRemaining = 15000

	keys[2].RPMRemaining = 12
	keys[2].ITPMRemaining = 7500
	keys[2].OTPMRemaining = 7500

	// Should select key with most capacity (key1)
	selected, err := selector.Select(keys)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if selected.APIKey != "key1" {
		t.Errorf("Select() selected %q, want key1", selected.APIKey)
	}
}

// TestLeastLoadedSelectorSkipsUnhealthy verifies unhealthy keys are skipped.
func TestLeastLoadedSelectorSkipsUnhealthy(t *testing.T) {
	selector := NewLeastLoadedSelector()

	keys := []*KeyMetadata{
		NewKeyMetadata("key1", 50, 30000, 30000),
		NewKeyMetadata("key2", 50, 30000, 30000),
	}

	// Mark key1 (higher capacity) as unhealthy
	keys[0].MarkUnhealthy(http.ErrServerClosed)

	// Set key2 to lower capacity but healthy
	keys[1].RPMRemaining = 10
	keys[1].ITPMRemaining = 5000
	keys[1].OTPMRemaining = 5000

	// Should select key2 even though it has less capacity
	selected, err := selector.Select(keys)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if selected.APIKey != "key2" {
		t.Errorf("Select() selected %q, want key2", selected.APIKey)
	}
}

// TestLeastLoadedSelectorAllExhausted verifies error when all keys unavailable.
func TestLeastLoadedSelectorAllExhausted(t *testing.T) {
	selector := NewLeastLoadedSelector()

	keys := []*KeyMetadata{
		NewKeyMetadata("key1", 50, 30000, 30000),
		NewKeyMetadata("key2", 50, 30000, 30000),
	}

	// Mark all keys unhealthy
	keys[0].MarkUnhealthy(http.ErrServerClosed)
	keys[1].MarkUnhealthy(http.ErrServerClosed)

	_, err := selector.Select(keys)
	if !errors.Is(err, ErrAllKeysExhausted) {
		t.Errorf("Select() error = %v, want ErrAllKeysExhausted", err)
	}
}

// TestRoundRobinSelector verifies round-robin selection strategy.
func TestRoundRobinSelector(t *testing.T) {
	selector := NewRoundRobinSelector()

	if selector.Name() != StrategyRoundRobin {
		t.Errorf("Name() = %q, want %q", selector.Name(), StrategyRoundRobin)
	}

	keys := []*KeyMetadata{
		NewKeyMetadata("key1", 50, 30000, 30000),
		NewKeyMetadata("key2", 50, 30000, 30000),
		NewKeyMetadata("key3", 50, 30000, 30000),
	}

	// Should cycle through keys in order
	selected1, err := selector.Select(keys)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	selected2, err := selector.Select(keys)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	selected3, err := selector.Select(keys)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	selected4, err := selector.Select(keys)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	// Should have selected all three keys, then wrapped to first
	selectedKeys := []string{
		selected1.APIKey,
		selected2.APIKey,
		selected3.APIKey,
		selected4.APIKey,
	}

	// First three should be unique
	seen := make(map[string]bool)
	for i := 0; i < 3; i++ {
		if seen[selectedKeys[i]] {
			t.Errorf("Selected duplicate key %q in first 3 selections", selectedKeys[i])
		}
		seen[selectedKeys[i]] = true
	}

	// Fourth should match first (wrap around)
	if selectedKeys[3] != selectedKeys[0] {
		t.Errorf("Fourth selection = %q, want %q (wrap around)", selectedKeys[3], selectedKeys[0])
	}
}

// TestRoundRobinSelectorSkipsUnavailable verifies skipping unavailable keys.
func TestRoundRobinSelectorSkipsUnavailable(t *testing.T) {
	selector := NewRoundRobinSelector()

	keys := []*KeyMetadata{
		NewKeyMetadata("key1", 50, 30000, 30000),
		NewKeyMetadata("key2", 50, 30000, 30000),
		NewKeyMetadata("key3", 50, 30000, 30000),
	}

	// Mark key2 as in cooldown
	keys[1].SetCooldown(time.Now().Add(5 * time.Minute))

	// Should skip key2
	for i := 0; i < 4; i++ {
		selected, err := selector.Select(keys)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}

		if selected.APIKey == "key2" {
			t.Errorf("Selected unavailable key2 on iteration %d", i)
		}
	}
}

// TestRoundRobinSelectorConcurrent verifies thread-safety.
func TestRoundRobinSelectorConcurrent(t *testing.T) {
	selector := NewRoundRobinSelector()

	keys := []*KeyMetadata{
		NewKeyMetadata("key1", 50, 30000, 30000),
		NewKeyMetadata("key2", 50, 30000, 30000),
		NewKeyMetadata("key3", 50, 30000, 30000),
	}

	var wg sync.WaitGroup
	numGoroutines := 100
	selectionsPerGoroutine := 10

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < selectionsPerGoroutine; j++ {
				_, err := selector.Select(keys)
				if err != nil {
					t.Errorf("Concurrent Select() error = %v", err)
				}
			}
		}()
	}

	wg.Wait()
}

// TestRoundRobinSelectorAllExhausted verifies error when all keys unavailable.
func TestRoundRobinSelectorAllExhausted(t *testing.T) {
	selector := NewRoundRobinSelector()

	keys := []*KeyMetadata{
		NewKeyMetadata("key1", 50, 30000, 30000),
		NewKeyMetadata("key2", 50, 30000, 30000),
	}

	// Mark all keys unhealthy
	keys[0].MarkUnhealthy(http.ErrServerClosed)
	keys[1].MarkUnhealthy(http.ErrServerClosed)

	_, err := selector.Select(keys)
	if !errors.Is(err, ErrAllKeysExhausted) {
		t.Errorf("Select() error = %v, want ErrAllKeysExhausted", err)
	}
}

// TestNewSelector verifies selector factory.
func TestNewSelector(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		wantType string
		wantErr  bool
	}{
		{
			name:     "least_loaded",
			strategy: StrategyLeastLoaded,
			wantType: StrategyLeastLoaded,
			wantErr:  false,
		},
		{
			name:     "round_robin",
			strategy: StrategyRoundRobin,
			wantType: StrategyRoundRobin,
			wantErr:  false,
		},
		{
			name:     "empty defaults to least_loaded",
			strategy: "",
			wantType: StrategyLeastLoaded,
			wantErr:  false,
		},
		{
			name:     "unknown strategy",
			strategy: "unknown",
			wantType: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector, err := NewSelector(tt.strategy)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSelector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if selector.Name() != tt.wantType {
					t.Errorf("NewSelector() type = %q, want %q", selector.Name(), tt.wantType)
				}
			}
		})
	}
}

// Helper function to create test keys with specific capacities.
func createTestKeys(capacities []float64) []*KeyMetadata {
	keys := make([]*KeyMetadata, len(capacities))
	for i, cap := range capacities {
		key := NewKeyMetadata("key"+string(rune('1'+i)), 100, 60000, 60000)
		// Set remaining based on capacity percentage
		key.RPMRemaining = int(float64(key.RPMLimit) * cap)
		key.ITPMRemaining = int(float64(key.ITPMLimit) * cap)
		key.OTPMRemaining = int(float64(key.OTPMLimit) * cap)
		keys[i] = key
	}
	return keys
}

// TestLeastLoadedSelectorVariousCapacities tests selection with various capacity levels.
func TestLeastLoadedSelectorVariousCapacities(t *testing.T) {
	selector := NewLeastLoadedSelector()

	// Create keys with 100%, 75%, 50%, 25% capacity
	keys := createTestKeys([]float64{1.0, 0.75, 0.5, 0.25})

	// Should always select the first key (100% capacity)
	for i := 0; i < 5; i++ {
		selected, err := selector.Select(keys)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}

		if selected.APIKey != "key1" {
			t.Errorf("Select() iteration %d selected %q, want key1", i, selected.APIKey)
		}
	}
}
