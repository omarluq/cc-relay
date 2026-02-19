package router_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/omarluq/cc-relay/internal/router"
)

func TestShuffleRouterName(t *testing.T) {
	t.Parallel()

	rtr := router.NewShuffleRouter()
	if rtr.Name() != router.StrategyShuffle {
		t.Errorf("Name() = %q, want %q", rtr.Name(), router.StrategyShuffle)
	}
}

func TestShuffleRouterEmptyProviders(t *testing.T) {
	t.Parallel()

	rtr := router.NewShuffleRouter()
	_, err := rtr.Select(context.Background(), []router.ProviderInfo{})

	if !errors.Is(err, router.ErrNoProviders) {
		t.Errorf("Select() error = %v, want router.ErrNoProviders", err)
	}
}

func TestShuffleRouterAllUnhealthy(t *testing.T) {
	t.Parallel()

	rtr := router.NewShuffleRouter()
	// Test that shuffle router correctly handles case where no providers are healthy.
	// Even with multiple unhealthy providers in the pool, should return proper error.
	prov1 := router.ProviderInfo{
		Provider:  router.NewTestProvider("a"),
		Weight:    5,
		Priority:  1,
		IsHealthy: func() bool { return false },
	}
	prov2 := router.ProviderInfo{
		Provider:  router.NewTestProvider("b"),
		Weight:    10,
		Priority:  2,
		IsHealthy: func() bool { return false },
	}
	providers := []router.ProviderInfo{prov1, prov2}

	_, err := rtr.Select(context.Background(), providers)

	if !errors.Is(err, router.ErrAllProvidersUnhealthy) {
		t.Errorf("Select() error = %v, want router.ErrAllProvidersUnhealthy", err)
	}
}

func TestShuffleRouterSkipsUnhealthyDistinct(t *testing.T) {
	t.Parallel()

	// This test differs from round_robin by testing the shuffle-specific behavior
	// that unhealthy providers don't affect the shuffle order
	rtr := router.NewShuffleRouter()

	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Weight: 1, Priority: 0, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p2"), Weight: 2, Priority: 0, IsHealthy: func() bool { return false }},
		{Provider: router.NewTestProvider("p3"), Weight: 3, Priority: 0, IsHealthy: func() bool { return true }},
	}

	// Select multiple times and verify we never get the unhealthy one
	selectedWeights := make(map[int]bool)
	for range 20 {
		prov, err := rtr.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		selectedWeights[prov.Weight] = true
	}

	// Should only have selected weights 1 and 3, not 2 (unhealthy)
	if selectedWeights[2] {
		t.Error("Selected unhealthy provider with weight 2")
	}
	if !selectedWeights[1] || !selectedWeights[3] {
		t.Error("Should have selected healthy providers with weights 1 and 3")
	}
}

func TestShuffleRouterDealingCardsEachGetsOneBeforeSeconds(t *testing.T) {
	t.Parallel()

	rtr := router.NewShuffleRouter()
	providers := createShuffleTestProviders(3)

	// First round: each provider should get exactly 1 request
	firstRound := make(map[int]int) // weight -> count
	for range 3 {
		selected, err := rtr.Select(context.Background(), providers)
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
	for range 3 {
		selected, err := rtr.Select(context.Background(), providers)
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

func TestShuffleRouterReshufflesWhenExhausted(t *testing.T) {
	t.Parallel()

	rtr := router.NewShuffleRouter()
	providers := createShuffleTestProviders(2)

	// Exhaust the deck (2 requests)
	for range 2 {
		_, err := rtr.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
	}

	// Third request should work (reshuffled)
	_, err := rtr.Select(context.Background(), providers)
	if err != nil {
		t.Errorf("Select() after exhaustion should work after reshuffle, got error = %v", err)
	}
}

func TestShuffleRouterReshufflesWhenProviderCountChanges(t *testing.T) {
	t.Parallel()

	rtr := router.NewShuffleRouter()

	// Start with 2 providers
	providers2 := createShuffleTestProviders(2)
	_, err := rtr.Select(context.Background(), providers2)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	// Switch to 3 providers - should reshuffle
	providers3 := createShuffleTestProviders(3)

	// Make 3 requests - should work and distribute across all 3
	counts := make(map[int]int)
	for range 3 {
		selected, err := rtr.Select(context.Background(), providers3)
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

func TestShuffleRouterConcurrentSafety(t *testing.T) {
	t.Parallel()

	rtr := router.NewShuffleRouter()
	providers := createShuffleTestProviders(3)

	var waitGroup sync.WaitGroup
	numGoroutines := 10
	requestsPerGoroutine := 100

	waitGroup.Add(numGoroutines)
	for range numGoroutines {
		go func() {
			defer waitGroup.Done()
			for range requestsPerGoroutine {
				_, err := rtr.Select(context.Background(), providers)
				if err != nil {
					t.Errorf("Concurrent Select() error = %v", err)
				}
			}
		}()
	}

	waitGroup.Wait()
}

func TestShuffleRouterImplementsInterface(t *testing.T) {
	t.Parallel()

	// Compile-time interface compliance check
	var _ router.ProviderRouter = (*router.ShuffleRouter)(nil)
}

func TestShuffleRouterNilIsHealthyTreatedAsHealthy(t *testing.T) {
	t.Parallel()

	rtr := router.NewShuffleRouter()
	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Weight: 1, Priority: 0, IsHealthy: nil},
		{Provider: router.NewTestProvider("p2"), Weight: 2, Priority: 0, IsHealthy: func() bool { return true }},
	}

	// Both should be selectable - do 2 rounds (4 requests)
	counts := make(map[int]int)
	for range 4 {
		selected, err := rtr.Select(context.Background(), providers)
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

func TestShuffleRouterSingleProvider(t *testing.T) {
	t.Parallel()

	rtr := router.NewShuffleRouter()
	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("only"), Weight: 42, Priority: 0, IsHealthy: func() bool { return true }},
	}

	for range 5 {
		selected, err := rtr.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		if selected.Weight != 42 {
			t.Errorf("Select() returned weight %d, want 42", selected.Weight)
		}
	}
}

func TestShuffleRouterEvenDistributionOverManyRounds(t *testing.T) {
	t.Parallel()

	rtr := router.NewShuffleRouter()
	providers := createShuffleTestProviders(4)

	// Run 40 requests (10 complete rounds)
	counts := make(map[int]int)
	for range 40 {
		selected, err := rtr.Select(context.Background(), providers)
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
func createShuffleTestProviders(n int) []router.ProviderInfo {
	providers := make([]router.ProviderInfo, n)
	for idx := range n {
		providers[idx] = router.ProviderInfo{
			Provider:  router.NewTestProvider(string(rune('a' + idx))),
			Weight:    idx + 1, // Use weight as identifier (1, 2, 3, ...)
			Priority:  idx,
			IsHealthy: func() bool { return true },
		}
	}
	return providers
}
