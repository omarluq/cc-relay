package router

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestRoundRobinRouter_Name(t *testing.T) {
	t.Parallel()

	router := NewRoundRobinRouter()
	if router.Name() != StrategyRoundRobin {
		t.Errorf("Name() = %q, want %q", router.Name(), StrategyRoundRobin)
	}
}

func TestRoundRobinRouter_EmptyProviders(t *testing.T) {
	t.Parallel()

	router := NewRoundRobinRouter()
	_, err := router.Select(context.Background(), []ProviderInfo{})

	if !errors.Is(err, ErrNoProviders) {
		t.Errorf("Select() error = %v, want ErrNoProviders", err)
	}
}

func TestRoundRobinRouter_AllUnhealthy(t *testing.T) {
	t.Parallel()

	router := NewRoundRobinRouter()
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

func TestRoundRobinRouter_EvenDistribution(t *testing.T) {
	t.Parallel()

	router := NewRoundRobinRouter()
	providers := createTestProviders(3, true)

	numRequests := 9 // 3 providers * 3 rounds
	selectionCounts := make(map[int]int)

	for i := 0; i < numRequests; i++ {
		selected, err := router.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}

		// Find which provider was selected
		for idx, p := range providers {
			if p.Weight == selected.Weight && p.Priority == selected.Priority {
				selectionCounts[idx]++
				break
			}
		}
	}

	// Each provider should have been selected exactly 3 times
	for idx, count := range selectionCounts {
		if count != 3 {
			t.Errorf("Provider %d selected %d times, want 3", idx, count)
		}
	}
}

func TestRoundRobinRouter_SequentialOrder(t *testing.T) {
	t.Parallel()

	router := NewRoundRobinRouter()
	providers := createTestProviders(3, true)

	// Track selection sequence
	var selections []int
	for i := 0; i < 6; i++ {
		selected, err := router.Select(context.Background(), providers)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}

		// Find which provider was selected by matching weight (used as identifier)
		for idx, p := range providers {
			if p.Weight == selected.Weight {
				selections = append(selections, idx)
				break
			}
		}
	}

	// Should cycle: 0, 1, 2, 0, 1, 2
	expected := []int{0, 1, 2, 0, 1, 2}
	for i, want := range expected {
		if selections[i] != want {
			t.Errorf("Selection %d = %d, want %d", i, selections[i], want)
		}
	}
}

func TestRoundRobinRouter_SkipsUnhealthy(t *testing.T) {
	t.Parallel()

	router := NewRoundRobinRouter()
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

func TestRoundRobinRouter_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	router := NewRoundRobinRouter()
	providers := createTestProviders(3, true)

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

func TestRoundRobinRouter_ImplementsInterface(t *testing.T) {
	t.Parallel()

	// Compile-time interface compliance check
	var _ ProviderRouter = (*RoundRobinRouter)(nil)
}

func TestRoundRobinRouter_NilIsHealthyTreatedAsHealthy(t *testing.T) {
	t.Parallel()

	router := NewRoundRobinRouter()
	providers := []ProviderInfo{
		{Weight: 1, IsHealthy: nil}, // nil = healthy
		{Weight: 2, IsHealthy: func() bool { return true }},
	}

	// Both should be selectable
	selected1, err := router.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	selected2, err := router.Select(context.Background(), providers)
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
func createTestProviders(n int, allHealthy bool) []ProviderInfo {
	providers := make([]ProviderInfo, n)
	for i := 0; i < n; i++ {
		healthy := allHealthy
		providers[i] = ProviderInfo{
			Weight:    i + 1, // Use weight as identifier (1, 2, 3, ...)
			Priority:  i,
			IsHealthy: func() bool { return healthy },
		}
	}
	return providers
}
