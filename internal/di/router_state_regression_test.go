package di_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/omarluq/cc-relay/internal/di"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
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
	cfg := di.MustTestConfig()
	cfg.Routing = di.MustTestRoutingConfig("round_robin")
	cfgSvc := di.NewConfigServiceWithConfig(&cfg)

	// Create router service
	routerSvc := di.NewRouterServiceWithConfigService(cfgSvc)

	// Create test providers with unique weights for identification
	p1 := providers.NewAnthropicProvider("p1", "https://api.p1.example.com")
	p2 := providers.NewAnthropicProvider("p2", "https://api.p2.example.com")
	p3 := providers.NewAnthropicProvider("p3", "https://api.p3.example.com")

	providerInfos := []router.ProviderInfo{
		di.MustTestProviderInfo(p1, 1, 0),
		di.MustTestProviderInfo(p2, 2, 0),
		di.MustTestProviderInfo(p3, 3, 0),
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
	for range 6 {
		selected, err := rr1.Select(context.Background(), providerInfos)
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

	cfg1 := di.MustTestConfig()
	cfg1.Routing = di.MustTestRoutingConfig("round_robin")
	cfgSvc := di.NewConfigServiceWithConfig(&cfg1)

	routerSvc := di.NewRouterServiceWithConfigService(cfgSvc)

	// Get initial router
	rr1 := routerSvc.GetRouter()
	assert.Equal(t, "round_robin", rr1.Name())

	// Update config with different strategy - should rebuild
	cfg2 := di.MustTestConfig()
	cfg2.Routing = di.MustTestRoutingConfig("failover")
	cfgSvc.GetConfigAtomic().Store(&cfg2)
	cfgSvc.Config = &cfg2

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

	cfg1 := di.MustTestConfig()
	cfg1.Routing = di.MustTestRoutingConfig("failover")
	cfg1.Routing.FailoverTimeout = 5000
	cfgSvc := di.NewConfigServiceWithConfig(&cfg1)

	routerSvc := di.NewRouterServiceWithConfigService(cfgSvc)

	rr1 := routerSvc.GetRouter()

	// Update timeout
	cfg2 := di.MustTestConfig()
	cfg2.Routing = di.MustTestRoutingConfig("failover")
	cfg2.Routing.FailoverTimeout = 10000
	cfgSvc.GetConfigAtomic().Store(&cfg2)
	cfgSvc.Config = &cfg2

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
	cfg := di.MustTestConfig()
	cfg.Routing = di.MustTestRoutingConfig("round_robin")
	cfgSvc := di.NewConfigServiceWithConfig(&cfg)

	routerSvc := di.NewRouterServiceWithConfigService(cfgSvc)
	liveRouter := router.NewLiveRouter(routerSvc.GetRouter)

	p1 := providers.NewAnthropicProvider("p1", "https://api.p1.example.com")
	p2 := providers.NewAnthropicProvider("p2", "https://api.p2.example.com")
	p3 := providers.NewAnthropicProvider("p3", "https://api.p3.example.com")

	providerInfos := []router.ProviderInfo{
		di.MustTestProviderInfo(p1, 1, 0),
		di.MustTestProviderInfo(p2, 2, 0),
		di.MustTestProviderInfo(p3, 3, 0),
	}

	// Perform multiple selections via LiveRouter
	selectedWeights := make([]int, 0, 6)
	for range 6 {
		selected, err := liveRouter.Select(context.Background(), providerInfos)
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

	cfg := di.MustTestConfig()
	cfg.Routing = di.MustTestRoutingConfig("round_robin")
	cfgSvc := di.NewConfigServiceWithConfig(&cfg)

	routerSvc := di.NewRouterServiceWithConfigService(cfgSvc)

	// Concurrently access router
	done := make(chan struct{})
	go func() {
		for range 1000 {
			_ = routerSvc.GetRouter()
		}
		close(done)
	}()

	// Also access from main goroutine
	for range 100 {
		_ = routerSvc.GetRouter()
	}

	<-done

	// Router should still work correctly
	roundRobin := routerSvc.GetRouter()
	p1 := providers.NewAnthropicProvider("p1", "https://api.p1.example.com")
	p2 := providers.NewAnthropicProvider("p2", "https://api.p2.example.com")

	providerInfos := []router.ProviderInfo{
		di.MustTestProviderInfo(p1, 1, 0),
		di.MustTestProviderInfo(p2, 2, 0),
	}

	// Verify round-robin state is preserved
	selectedWeights := make([]int, 0, 4)
	for range 4 {
		selected, err := roundRobin.Select(context.Background(), providerInfos)
		assert.NoError(t, err)
		selectedWeights = append(selectedWeights, selected.Weight)
	}

	expected := []int{1, 2, 1, 2}
	assert.Equal(t, expected, selectedWeights, "Router state should be preserved after concurrent access")
}
