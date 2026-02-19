package router_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/omarluq/cc-relay/internal/router"
)

func TestRoundRobinRouterName(t *testing.T) {
	t.Parallel()

	rtr := router.NewRoundRobinRouter()
	if rtr.Name() != router.StrategyRoundRobin {
		t.Errorf("Name() = %q, want %q", rtr.Name(), router.StrategyRoundRobin)
	}
}

func TestRoundRobinRouterEmptyProviders(t *testing.T) {
	t.Parallel()

	rtr := router.NewRoundRobinRouter()
	_, err := rtr.Select(context.Background(), []router.ProviderInfo{})

	if !errors.Is(err, router.ErrNoProviders) {
		t.Errorf("Select() error = %v, want router.ErrNoProviders", err)
	}
}

func TestRoundRobinRouterAllUnhealthy(t *testing.T) {
	t.Parallel()

	rtr := router.NewRoundRobinRouter()
	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Weight: 0, Priority: 0, IsHealthy: func() bool { return false }},
		{Provider: router.NewTestProvider("p2"), Weight: 0, Priority: 0, IsHealthy: func() bool { return false }},
		{Provider: router.NewTestProvider("p3"), Weight: 0, Priority: 0, IsHealthy: func() bool { return false }},
	}

	_, err := rtr.Select(context.Background(), providers)

	if !errors.Is(err, router.ErrAllProvidersUnhealthy) {
		t.Errorf("Select() error = %v, want router.ErrAllProvidersUnhealthy", err)
	}
}

func TestRoundRobinRouterEvenDistribution(t *testing.T) {
	t.Parallel()

	rtr := router.NewRoundRobinRouter()
	providers := createTestProviders(3, true)

	numRequests := 9 // 3 providers * 3 rounds
	selectionCounts := make(map[int]int)

	for idx := 0; idx < numRequests; idx++ {
		selected, err := rtr.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}

		// Find which provider was selected
		for provIdx, prov := range providers {
			if prov.Weight == selected.Weight && prov.Priority == selected.Priority {
				selectionCounts[provIdx]++
				break
			}
		}
	}

	// Each provider should have been selected exactly 3 times
	for provIdx, count := range selectionCounts {
		if count != 3 {
			t.Errorf("Provider %d selected %d times, want 3", provIdx, count)
		}
	}
}

func TestRoundRobinRouterSequentialOrder(t *testing.T) {
	t.Parallel()

	rtr := router.NewRoundRobinRouter()
	providers := createTestProviders(3, true)

	// Track selection sequence
	var selections []int
	for idx := 0; idx < 6; idx++ {
		selected, err := rtr.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}

		// Find which provider was selected by matching weight (used as identifier)
		for provIdx, prov := range providers {
			if prov.Weight == selected.Weight {
				selections = append(selections, provIdx)
				break
			}
		}
	}

	// Should cycle: 0, 1, 2, 0, 1, 2
	expected := []int{0, 1, 2, 0, 1, 2}
	for idx, want := range expected {
		if selections[idx] != want {
			t.Errorf("Selection %d = %d, want %d", idx, selections[idx], want)
		}
	}
}

func TestRoundRobinRouterSkipsUnhealthy(t *testing.T) {
	t.Parallel()

	rtr := router.NewRoundRobinRouter()
	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Weight: 1, Priority: 0, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p2"), Weight: 2, Priority: 0, IsHealthy: func() bool { return false }},
		{Provider: router.NewTestProvider("p3"), Weight: 3, Priority: 0, IsHealthy: func() bool { return true }},
	}

	// With provider 1 unhealthy, should only select from 0 and 2
	for iteration := 0; iteration < 4; iteration++ {
		selected, err := rtr.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}

		if selected.Weight == 2 {
			t.Errorf("Selected unhealthy provider with weight 2 on iteration %d", iteration)
		}
	}
}

func TestRoundRobinRouterConcurrentSafety(t *testing.T) {
	t.Parallel()

	rtr := router.NewRoundRobinRouter()
	providers := createTestProviders(3, true)

	var waitGroup sync.WaitGroup
	numGoroutines := 10
	requestsPerGoroutine := 100

	waitGroup.Add(numGoroutines)
	for gIdx := 0; gIdx < numGoroutines; gIdx++ {
		go func() {
			defer waitGroup.Done()
			for reqIdx := 0; reqIdx < requestsPerGoroutine; reqIdx++ {
				_, err := rtr.Select(context.Background(), providers)
				if err != nil {
					t.Errorf("Concurrent Select() error = %v", err)
				}
			}
		}()
	}

	waitGroup.Wait()
}

func TestRoundRobinRouterImplementsInterface(t *testing.T) {
	t.Parallel()

	// Compile-time interface compliance check
	var _ router.ProviderRouter = (*router.RoundRobinRouter)(nil)
}

func TestRoundRobinRouterNilIsHealthyTreatedAsHealthy(t *testing.T) {
	t.Parallel()

	rtr := router.NewRoundRobinRouter()
	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Weight: 1, Priority: 0, IsHealthy: nil},
		{Provider: router.NewTestProvider("p2"), Weight: 2, Priority: 0, IsHealthy: func() bool { return true }},
	}

	// Both should be selectable
	selected1, err := rtr.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	selected2, err := rtr.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	// Should have selected both providers
	if selected1.Weight == selected2.Weight {
		t.Error("Expected to select different providers, got same weight")
	}
}

// createTestProviders creates N providers with unique weights for identification.
// If allHealthy is true, all providers are marked healthy.
func createTestProviders(n int, allHealthy bool) []router.ProviderInfo {
	providers := make([]router.ProviderInfo, n)
	for idx := 0; idx < n; idx++ {
		healthy := allHealthy
		providers[idx] = router.ProviderInfo{
			Provider:  router.NewTestProvider(string(rune('a' + idx))),
			Weight:    idx + 1, // Use weight as identifier (1, 2, 3, ...)
			Priority:  idx,
			IsHealthy: func() bool { return healthy },
		}
	}
	return providers
}
