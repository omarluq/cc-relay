package di

import (
	"context"
	"testing"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/router"
	"github.com/stretchr/testify/assert"
)

// TestRouterService_CachesRoundRobinState verifies that RouterService caches
// router instances to preserve state for stateful routers like round_robin.
//
// This is a regression test for a bug where RouterService.GetRouter() created
// a new router on every call via LiveRouter, causing round_robin to reset its
// counter and always return the first provider.
//
// See: claude-code-action-items.md F1) Router state resets on every request.
func TestRouterServiceCachesRoundRobinState(t *testing.T) {
	t.Parallel()

	// Create a config service with round_robin strategy
	cfg := &config.Config{
		Routing: config.RoutingConfig{
			Strategy: "round_robin",
		},
	}
	cfgSvc := &ConfigService{
		Config: cfg,
	}
	cfgSvc.config.Store(cfg)

	// Create router service
	routerSvc := &RouterService{cfgSvc: cfgSvc}

	// Create test providers with unique weights for identification
	providers := []router.ProviderInfo{
		{Weight: 1, Priority: 0, IsHealthy: func() bool { return true }},
		{Weight: 2, Priority: 0, IsHealthy: func() bool { return true }},
		{Weight: 3, Priority: 0, IsHealthy: func() bool { return true }},
	}

	// Get router multiple times and verify it's the same cached instance
	rr1 := routerSvc.GetRouter()
	rr2 := routerSvc.GetRouter()
	rr3 := routerSvc.GetRouter()

	// Should return the same router instance (same counter state)
	assert.Same(t, rr1, rr2, "GetRouter() should return cached router")
	assert.Same(t, rr2, rr3, "GetRouter() should return cached router")

	// Perform selections and verify round-robin distributes across providers
	selectedWeights := make([]int, 0, 6)
	for i := 0; i < 6; i++ {
		selected, err := rr1.Select(context.Background(), providers)
		assert.NoError(t, err)
		selectedWeights = append(selectedWeights, selected.Weight)
	}

	// Should cycle through all 3 providers: 1, 2, 3, 1, 2, 3
	// If router was recreated each time, we'd get: 1, 1, 1, 1, 1, 1
	expected := []int{1, 2, 3, 1, 2, 3}
	assert.Equal(t, expected, selectedWeights, "Round-robin should distribute across providers")
}

// TestRouterService_RebuildsOnConfigChange verifies that RouterService rebuilds
// the router when strategy or timeout changes, but preserves state otherwise.
func TestRouterServiceRebuildsOnConfigChange(t *testing.T) {
	t.Parallel()

	cfg1 := &config.Config{
		Routing: config.RoutingConfig{
			Strategy: "round_robin",
		},
	}
	cfgSvc := &ConfigService{
		Config: cfg1,
	}
	cfgSvc.config.Store(cfg1)

	routerSvc := &RouterService{cfgSvc: cfgSvc}

	// Get initial router
	rr1 := routerSvc.GetRouter()
	assert.Equal(t, "round_robin", rr1.Name())

	// Update config with different strategy - should rebuild
	cfg2 := &config.Config{
		Routing: config.RoutingConfig{
			Strategy: "failover",
		},
	}
	cfgSvc.config.Store(cfg2)

	rr2 := routerSvc.GetRouter()
	assert.Equal(t, "failover", rr2.Name())
	// Router instance should be different after config change
	assert.NotSame(t, rr1, rr2, "Router should be rebuilt when strategy changes")

	// Get router again with same config - should return cached instance
	rr3 := routerSvc.GetRouter()
	assert.Same(t, rr2, rr3, "Should use cached router when config unchanged")
}

// TestRouterService_RebuildsOnTimeoutChange verifies that router is rebuilt
// when timeout configuration changes.
func TestRouterServiceRebuildsOnTimeoutChange(t *testing.T) {
	t.Parallel()

	cfg1 := &config.Config{
		Routing: config.RoutingConfig{
			Strategy:        "failover",
			FailoverTimeout: 5000, // 5 seconds
		},
	}
	cfgSvc := &ConfigService{
		Config: cfg1,
	}
	cfgSvc.config.Store(cfg1)

	routerSvc := &RouterService{cfgSvc: cfgSvc}

	rr1 := routerSvc.GetRouter()

	// Update timeout
	cfg2 := &config.Config{
		Routing: config.RoutingConfig{
			Strategy:        "failover",
			FailoverTimeout: 10000, // 10 seconds
		},
	}
	cfgSvc.config.Store(cfg2)

	rr2 := routerSvc.GetRouter()
	assert.NotSame(t, rr1, rr2, "Router should be rebuilt when timeout changes")
}

// TestLiveRouter_DoesNotResetRoundRobinState is a direct regression test for
// the bug where LiveRouter created a new router on each Select() call.
//
// Before fix: NewRoundRobinRouter was called on every Select(), resetting
// the counter to 0, so all requests went to provider 0.
//
// After fix: RouterService caches the router, so the counter is preserved.
func TestLiveRouterDoesNotResetRoundRobinState(t *testing.T) {
	t.Parallel()

	// Create a live router that always calls GetRouter()
	cfg := &config.Config{
		Routing: config.RoutingConfig{
			Strategy: "round_robin",
		},
	}
	cfgSvc := &ConfigService{
		Config: cfg,
	}
	cfgSvc.config.Store(cfg)

	routerSvc := &RouterService{cfgSvc: cfgSvc}
	liveRouter := router.NewLiveRouter(routerSvc.GetRouter)

	providers := []router.ProviderInfo{
		{Weight: 1, Priority: 0, IsHealthy: func() bool { return true }},
		{Weight: 2, Priority: 0, IsHealthy: func() bool { return true }},
		{Weight: 3, Priority: 0, IsHealthy: func() bool { return true }},
	}

	// Perform multiple selections via LiveRouter
	selectedWeights := make([]int, 0, 6)
	for i := 0; i < 6; i++ {
		selected, err := liveRouter.Select(context.Background(), providers)
		assert.NoError(t, err)
		selectedWeights = append(selectedWeights, selected.Weight)
	}

	// Should distribute: 1, 2, 3, 1, 2, 3
	// Bug would cause: 1, 1, 1, 1, 1, 1
	expected := []int{1, 2, 3, 1, 2, 3}
	assert.Equal(t, expected, selectedWeights, "LiveRouter should preserve router state")
}

// TestRouterService_ConcurrentAccess verifies that cached router access
// is thread-safe under concurrent GetRouter() calls.
func TestRouterServiceConcurrentAccess(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Routing: config.RoutingConfig{
			Strategy: "round_robin",
		},
	}
	cfgSvc := &ConfigService{
		Config: cfg,
	}
	cfgSvc.config.Store(cfg)

	routerSvc := &RouterService{cfgSvc: cfgSvc}

	// Concurrently access router
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			_ = routerSvc.GetRouter()
		}
		close(done)
	}()

	// Also access from main goroutine
	for i := 0; i < 100; i++ {
		_ = routerSvc.GetRouter()
	}

	<-done

	// Router should still work correctly
	rr := routerSvc.GetRouter()
	providers := []router.ProviderInfo{
		{Weight: 1, Priority: 0, IsHealthy: func() bool { return true }},
		{Weight: 2, Priority: 0, IsHealthy: func() bool { return true }},
	}

	// Verify round-robin state is preserved
	selectedWeights := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		selected, err := rr.Select(context.Background(), providers)
		assert.NoError(t, err)
		selectedWeights = append(selectedWeights, selected.Weight)
	}

	expected := []int{1, 2, 1, 2}
	assert.Equal(t, expected, selectedWeights, "Router state should be preserved after concurrent access")
}
