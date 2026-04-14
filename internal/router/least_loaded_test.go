package router_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/omarluq/cc-relay/internal/router"
)

func TestLeastLoadedRouterName(t *testing.T) {
	t.Parallel()

	rtr := router.NewLeastLoadedRouter()
	if rtr.Name() != router.StrategyLeastLoaded {
		t.Errorf("Name() = %q, want %q", rtr.Name(), router.StrategyLeastLoaded)
	}
}

func TestLeastLoadedRouterSelectNoProviders(t *testing.T) {
	t.Parallel()

	rtr := router.NewLeastLoadedRouter()
	_, err := rtr.Select(context.Background(), []router.ProviderInfo{})

	if !errors.Is(err, router.ErrNoProviders) {
		t.Errorf("Select() error = %v, want %v", err, router.ErrNoProviders)
	}
}

func TestLeastLoadedRouterSelectAllUnhealthy(t *testing.T) {
	t.Parallel()

	rtr := router.NewLeastLoadedRouter()
	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, router.NeverHealthy()),
		router.NewTestProviderInfo("p2", 2, 2, router.NeverHealthy()),
	}

	_, err := rtr.Select(context.Background(), providers)
	if !errors.Is(err, router.ErrAllProvidersUnhealthy) {
		t.Errorf("Select() error = %v, want %v", err, router.ErrAllProvidersUnhealthy)
	}
}

func TestLeastLoadedRouterSelectSingleProvider(t *testing.T) {
	t.Parallel()

	rtr := router.NewLeastLoadedRouter()
	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, router.AlwaysHealthy()),
	}

	result, err := rtr.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if result.Provider.Name() != "p1" {
		t.Errorf("Select() returned %q, want p1", result.Provider.Name())
	}
}

func TestLeastLoadedRouterSelectLeastLoaded(t *testing.T) {
	t.Parallel()

	rtr := router.NewLeastLoadedRouter()
	provider1 := router.NewTestProvider("p1")
	provider2 := router.NewTestProvider("p2")

	// Simulate load: p1 has 5, p2 has 2, p3 has 0
	for range 5 {
		rtr.Acquire(provider1)
	}
	for range 2 {
		rtr.Acquire(provider2)
	}

	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, router.AlwaysHealthy()),
		router.NewTestProviderInfo("p2", 1, 1, router.AlwaysHealthy()),
		router.NewTestProviderInfo("p3", 1, 1, router.AlwaysHealthy()),
	}

	// p3 should be selected (least loaded)
	result, err := rtr.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if result.Provider.Name() != "p3" {
		t.Errorf("Select() returned %q, want p3 (least loaded)", result.Provider.Name())
	}

	// Cleanup
	for range 5 {
		rtr.Release(provider1)
	}
	for range 2 {
		rtr.Release(provider2)
	}
}

func TestLeastLoadedRouterTieBreakerPriority(t *testing.T) {
	t.Parallel()

	rtr := router.NewLeastLoadedRouter()

	// Both have same load (0), but p2 has higher priority
	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, router.AlwaysHealthy()),
		router.NewTestProviderInfo("p2", 2, 2, router.AlwaysHealthy()), // Higher priority
	}

	result, err := rtr.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if result.Provider.Name() != "p2" {
		t.Errorf("Select() returned %q, want p2 (higher priority on tie)", result.Provider.Name())
	}
}

func TestLeastLoadedRouterAcquireRelease(t *testing.T) {
	t.Parallel()

	rtr := router.NewLeastLoadedRouter()
	provider := router.NewTestProvider("test")

	// Initial count should be 0
	countBefore := router.GetLeastLoadedCount(rtr, provider)
	if countBefore != 0 {
		t.Errorf("Initial count = %d, want 0", countBefore)
	}

	// Acquire 3 times
	rtr.Acquire(provider)
	rtr.Acquire(provider)
	rtr.Acquire(provider)

	countAfterAcquire := router.GetLeastLoadedCount(rtr, provider)
	if countAfterAcquire != 3 {
		t.Errorf("Count after 3 acquires = %d, want 3", countAfterAcquire)
	}

	// Release once
	rtr.Release(provider)

	countAfterRelease := router.GetLeastLoadedCount(rtr, provider)
	if countAfterRelease != 2 {
		t.Errorf("Count after 1 release = %d, want 2", countAfterRelease)
	}

	// Release remaining
	rtr.Release(provider)
	rtr.Release(provider)

	countFinal := router.GetLeastLoadedCount(rtr, provider)
	if countFinal != 0 {
		t.Errorf("Final count = %d, want 0", countFinal)
	}
}

func TestLeastLoadedRouterConcurrent(t *testing.T) {
	t.Parallel()

	rtr := router.NewLeastLoadedRouter()
	provider := router.NewTestProvider("test")

	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("test", 1, 1, router.AlwaysHealthy()),
	}

	var waitGroup sync.WaitGroup
	var errorCount atomic.Int32
	var selections atomic.Int32

	// Simulate concurrent selections and releases
	for i := 0; i < 10; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			// Acquire
			rtr.Acquire(provider)

			// Select
			_, err := rtr.Select(context.Background(), providers)
			if err != nil {
				errorCount.Add(1)
			} else {
				selections.Add(1)
			}

			// Release
			rtr.Release(provider)
		}()
	}

	waitGroup.Wait()

	if errorCount.Load() != 0 {
		t.Errorf("Select() returned %d errors, want 0", errorCount.Load())
	}
	if selections.Load() != 10 {
		t.Errorf("Select() succeeded %d times, want 10", selections.Load())
	}
}

func TestLeastLoadedRouterSkipsUnhealthy(t *testing.T) {
	t.Parallel()

	rtr := router.NewLeastLoadedRouter()
	provider1 := router.NewTestProvider("p1")

	// p1 has load, p2 has none but is unhealthy
	for range 2 {
		rtr.Acquire(provider1)
	}

	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, router.AlwaysHealthy()),
		router.NewTestProviderInfo("p2", 1, 1, router.NeverHealthy()),
	}

	result, err := rtr.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if result.Provider.Name() != "p1" {
		t.Errorf("Select() returned %q, want p1 (only healthy option)", result.Provider.Name())
	}

	for range 2 {
		rtr.Release(provider1)
	}
}

// Test that LeastLoadedRouter implements ProviderRouter and ProviderLoadTracker.
func TestLeastLoadedRouterImplementsInterfaces(t *testing.T) {
	t.Parallel()

	var _ router.ProviderRouter = (*router.LeastLoadedRouter)(nil)
	var _ router.ProviderLoadTracker = (*router.LeastLoadedRouter)(nil)
}
