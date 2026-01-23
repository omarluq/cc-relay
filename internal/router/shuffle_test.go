package router

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestShuffleRouter_Name(t *testing.T) {
	t.Parallel()

	router := NewShuffleRouter()
	if router.Name() != StrategyShuffle {
		t.Errorf("Name() = %q, want %q", router.Name(), StrategyShuffle)
	}
}

func TestShuffleRouter_EmptyProviders(t *testing.T) {
	t.Parallel()

	router := NewShuffleRouter()
	_, err := router.Select(context.Background(), []ProviderInfo{})

	if !errors.Is(err, ErrNoProviders) {
		t.Errorf("Select() error = %v, want ErrNoProviders", err)
	}
}

func TestShuffleRouter_AllUnhealthy(t *testing.T) {
	t.Parallel()

	router := NewShuffleRouter()
	providers := []ProviderInfo{
		{IsHealthy: func() bool { return false }},
		{IsHealthy: func() bool { return false }},
		{IsHealthy: func() bool { return false }},
	}

	_, err := router.Select(context.Background(), providers)

	if !errors.Is(err, ErrAllProvidersUnhealthy) {
		t.Errorf("Select() error = %v, want ErrAllProvidersUnhealthy", err)
	}
}

func TestShuffleRouter_DealingCards_EachGetsOneBeforeSeconds(t *testing.T) {
	t.Parallel()

	router := NewShuffleRouter()
	providers := createShuffleTestProviders(3)

	// First round: each provider should get exactly 1 request
	firstRound := make(map[int]int) // weight -> count
	for i := 0; i < 3; i++ {
		selected, err := router.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		firstRound[selected.Weight]++
	}

	// Verify each provider got exactly 1 request
	for weight, count := range firstRound {
		if count != 1 {
			t.Errorf("Provider with weight %d got %d requests in first round, want 1", weight, count)
		}
	}

	// Second round: again each should get exactly 1 more
	secondRound := make(map[int]int)
	for i := 0; i < 3; i++ {
		selected, err := router.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		secondRound[selected.Weight]++
	}

	// Verify each provider got exactly 1 request in second round
	for weight, count := range secondRound {
		if count != 1 {
			t.Errorf("Provider with weight %d got %d requests in second round, want 1", weight, count)
		}
	}
}

func TestShuffleRouter_ReshufflesWhenExhausted(t *testing.T) {
	t.Parallel()

	router := NewShuffleRouter()
	providers := createShuffleTestProviders(2)

	// Exhaust the deck (2 requests)
	for i := 0; i < 2; i++ {
		_, err := router.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
	}

	// Third request should work (reshuffled)
	_, err := router.Select(context.Background(), providers)
	if err != nil {
		t.Errorf("Select() after exhaustion should work after reshuffle, got error = %v", err)
	}
}

func TestShuffleRouter_ReshufflesWhenProviderCountChanges(t *testing.T) {
	t.Parallel()

	router := NewShuffleRouter()

	// Start with 2 providers
	providers2 := createShuffleTestProviders(2)
	_, err := router.Select(context.Background(), providers2)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	// Switch to 3 providers - should reshuffle
	providers3 := createShuffleTestProviders(3)

	// Make 3 requests - should work and distribute across all 3
	counts := make(map[int]int)
	for i := 0; i < 3; i++ {
		selected, err := router.Select(context.Background(), providers3)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		counts[selected.Weight]++
	}

	// Each of the 3 providers should have been selected once
	if len(counts) != 3 {
		t.Errorf("Expected 3 unique providers after count change, got %d", len(counts))
	}
}

func TestShuffleRouter_SkipsUnhealthy(t *testing.T) {
	t.Parallel()

	router := NewShuffleRouter()
	providers := []ProviderInfo{
		{Weight: 1, IsHealthy: func() bool { return true }},
		{Weight: 2, IsHealthy: func() bool { return false }}, // unhealthy
		{Weight: 3, IsHealthy: func() bool { return true }},
	}

	// With provider 1 unhealthy, should only select from 0 and 2
	for i := 0; i < 4; i++ {
		selected, err := router.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}

		if selected.Weight == 2 {
			t.Errorf("Selected unhealthy provider with weight 2 on iteration %d", i)
		}
	}
}

func TestShuffleRouter_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	router := NewShuffleRouter()
	providers := createShuffleTestProviders(3)

	var wg sync.WaitGroup
	numGoroutines := 10
	requestsPerGoroutine := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				_, err := router.Select(context.Background(), providers)
				if err != nil {
					t.Errorf("Concurrent Select() error = %v", err)
				}
			}
		}()
	}

	wg.Wait()
}

func TestShuffleRouter_ImplementsInterface(t *testing.T) {
	t.Parallel()

	// Compile-time interface compliance check
	var _ ProviderRouter = (*ShuffleRouter)(nil)
}

func TestShuffleRouter_NilIsHealthyTreatedAsHealthy(t *testing.T) {
	t.Parallel()

	router := NewShuffleRouter()
	providers := []ProviderInfo{
		{Weight: 1, IsHealthy: nil}, // nil = healthy
		{Weight: 2, IsHealthy: func() bool { return true }},
	}

	// Both should be selectable - do 2 rounds (4 requests)
	counts := make(map[int]int)
	for i := 0; i < 4; i++ {
		selected, err := router.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		counts[selected.Weight]++
	}

	// Both providers should be selected (2 rounds of 2)
	if counts[1] != 2 || counts[2] != 2 {
		t.Errorf("Expected each provider selected twice, got counts = %v", counts)
	}
}

func TestShuffleRouter_SingleProvider(t *testing.T) {
	t.Parallel()

	router := NewShuffleRouter()
	providers := []ProviderInfo{
		{Weight: 42, IsHealthy: func() bool { return true }},
	}

	for i := 0; i < 5; i++ {
		selected, err := router.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		if selected.Weight != 42 {
			t.Errorf("Select() returned weight %d, want 42", selected.Weight)
		}
	}
}

func TestShuffleRouter_EvenDistributionOverManyRounds(t *testing.T) {
	t.Parallel()

	router := NewShuffleRouter()
	providers := createShuffleTestProviders(4)

	// Run 40 requests (10 complete rounds)
	counts := make(map[int]int)
	for i := 0; i < 40; i++ {
		selected, err := router.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		counts[selected.Weight]++
	}

	// Each provider should have exactly 10 requests
	for weight, count := range counts {
		if count != 10 {
			t.Errorf("Provider with weight %d got %d requests, want 10", weight, count)
		}
	}
}

// createShuffleTestProviders creates N healthy providers with unique weights for identification.
func createShuffleTestProviders(n int) []ProviderInfo {
	providers := make([]ProviderInfo, n)
	for i := 0; i < n; i++ {
		providers[i] = ProviderInfo{
			Weight:    i + 1, // Use weight as identifier (1, 2, 3, ...)
			Priority:  i,
			IsHealthy: func() bool { return true },
		}
	}
	return providers
}
