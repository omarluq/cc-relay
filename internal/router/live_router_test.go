package router_test

import (
	"context"
	"errors"
	"testing"

	"github.com/omarluq/cc-relay/internal/router"
)

func TestLiveRouterName(t *testing.T) {
	t.Parallel()

	routerFactory := func() router.ProviderRouter {
		return router.NewRoundRobinRouter()
	}
	live := router.NewLiveRouter(routerFactory)

	if live.Name() != router.StrategyRoundRobin {
		t.Errorf("Name() = %q, want %q", live.Name(), router.StrategyRoundRobin)
	}
}

func TestLiveRouterSelect(t *testing.T) {
	t.Parallel()

	routerFactory := func() router.ProviderRouter {
		return router.NewRoundRobinRouter()
	}
	live := router.NewLiveRouter(routerFactory)

	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, router.AlwaysHealthy()),
		router.NewTestProviderInfo("p2", 1, 1, router.AlwaysHealthy()),
	}

	result, err := live.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if result.Provider == nil {
		t.Error("Select() returned nil provider")
	}
}

func TestLiveRouterDelegatesToCurrent(t *testing.T) {
	t.Parallel()

	callCount := 0
	routerFactory := func() router.ProviderRouter {
		callCount++
		return router.NewRoundRobinRouter()
	}
	live := router.NewLiveRouter(routerFactory)

	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, router.AlwaysHealthy()),
	}

	// Multiple selects should call the function each time
	for range 3 {
		_, err := live.Select(context.Background(), providers)
		_ = err // We only care about side effect (callCount increment)
	}

	if callCount != 3 {
		t.Errorf("Provider function called %d times, want 3", callCount)
	}
}

func TestLiveRouterSwitchesStrategy(t *testing.T) {
	t.Parallel()

	useRoundRobin := true
	routerFactory := func() router.ProviderRouter {
		if useRoundRobin {
			return router.NewRoundRobinRouter()
		}
		return router.NewShuffleRouter()
	}
	live := router.NewLiveRouter(routerFactory)

	if live.Name() != router.StrategyRoundRobin {
		t.Errorf("Name() = %q, want %q", live.Name(), router.StrategyRoundRobin)
	}

	useRoundRobin = false

	if live.Name() != router.StrategyShuffle {
		t.Errorf("Name() = %q, want %q", live.Name(), router.StrategyShuffle)
	}
}

func TestLiveRouterSelectNoProviders(t *testing.T) {
	t.Parallel()

	routerFactory := func() router.ProviderRouter {
		return router.NewRoundRobinRouter()
	}
	live := router.NewLiveRouter(routerFactory)

	_, err := live.Select(context.Background(), []router.ProviderInfo{})
	if !errors.Is(err, router.ErrNoProviders) {
		t.Errorf("Select() error = %v, want %v", err, router.ErrNoProviders)
	}
}

func TestLiveRouterAcquireReleaseWithLeastLoaded(t *testing.T) {
	t.Parallel()

	provider := router.NewTestProvider("p1")
	llr := router.NewLeastLoadedRouter()

	routerFactory := func() router.ProviderRouter {
		return llr
	}
	live := router.NewLiveRouter(routerFactory)

	liveRouter, ok := live.(*router.LiveRouter)
	if !ok {
		t.Fatal("NewLiveRouter did not return *LiveRouter")
	}

	router.LiveRouterAcquire(liveRouter, provider)
	count := router.GetLeastLoadedCount(llr, provider)
	if count != 1 {
		t.Errorf("Count after Acquire = %d, want 1", count)
	}

	router.LiveRouterRelease(liveRouter, provider)
	countAfter := router.GetLeastLoadedCount(llr, provider)
	if countAfter != 0 {
		t.Errorf("Count after Release = %d, want 0", countAfter)
	}
}

func TestLiveRouterAcquireReleaseWithNonTrackerRouter(t *testing.T) {
	t.Parallel()

	provider := router.NewTestProvider("p1")

	routerFactory := func() router.ProviderRouter {
		// RoundRobin doesn't implement ProviderLoadTracker
		return router.NewRoundRobinRouter()
	}
	live := router.NewLiveRouter(routerFactory)

	liveRouter, ok := live.(*router.LiveRouter)
	if !ok {
		t.Fatal("NewLiveRouter did not return *LiveRouter")
	}

	// Should not panic - router doesn't support tracking
	router.LiveRouterAcquire(liveRouter, provider)
	router.LiveRouterRelease(liveRouter, provider)
}

// Verify interface compliance.
var _ router.ProviderRouter = (*router.LiveRouter)(nil)
