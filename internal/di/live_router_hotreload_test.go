package di_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/omarluq/cc-relay/internal/di"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
)

// TestLiveRouter_HotReloadStrategyChange verifies that changing the routing
// strategy takes effect without rebuilding the handler.
//
// This is a regression test for F1 in claude-code-action-items.md.
func TestLiveRouterHotReloadStrategyChange(t *testing.T) {
	t.Parallel()

	// Start with round_robin strategy
	cfg1 := di.MustTestConfig()
	cfg1.Routing = di.MustTestRoutingConfig("round_robin")

	cfgSvc := di.NewConfigServiceWithConfig(&cfg1)
	routerSvc := di.NewRouterServiceWithConfigService(cfgSvc)
	liveRouter := router.NewLiveRouter(routerSvc.GetRouterAsFunc())

	// Create test providers with full ProviderInfo
	p1 := providers.NewAnthropicProvider("p1", "https://api.p1.example.com")
	p2 := providers.NewAnthropicProvider("p2", "https://api.p2.example.com")
	p3 := providers.NewAnthropicProvider("p3", "https://api.p3.example.com")

	providerInfos := []router.ProviderInfo{
		di.MustTestProviderInfo(p1, 1, 1),
		di.MustTestProviderInfo(p2, 2, 2),
		di.MustTestProviderInfo(p3, 3, 3),
	}

	// With round_robin, should distribute across providers
	roundRobinSelections := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		selected, err := liveRouter.Select(context.Background(), providerInfos)
		assert.NoError(t, err)
		roundRobinSelections = append(roundRobinSelections, selected.Weight)
	}

	// Round-robin should cycle: 1, 2, 3, 1
	assert.Equal(t, []int{1, 2, 3, 1}, roundRobinSelections,
		"Round-robin should distribute across providers")

	// Now change strategy to failover
	cfg2 := di.MustTestConfig()
	cfg2.Routing = di.MustTestRoutingConfig("failover")
	cfgSvc.GetConfigAtomic().Store(&cfg2)
	cfgSvc.Config = &cfg2

	// With failover, should always select highest priority (higher priority number = higher priority)
	// Provider with Priority 3 (Weight 3) should always be selected
	failoverSelections := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		selected, err := liveRouter.Select(context.Background(), providerInfos)
		assert.NoError(t, err)
		failoverSelections = append(failoverSelections, selected.Weight)
	}

	// Failover always selects provider with Priority 3 (Weight 3)
	assert.Equal(t, []int{3, 3, 3, 3}, failoverSelections,
		"Failover should always select highest priority provider")

	// Verify the live router's name reflects the change
	assert.Equal(t, "failover", liveRouter.Name(),
		"Live router name should reflect strategy change")
}

// TestLiveRouter_HotReloadTimeoutChange verifies that changing the failover
// timeout takes effect without rebuilding the handler.
func TestLiveRouterHotReloadTimeoutChange(t *testing.T) {
	t.Parallel()

	cfg1 := di.MustTestConfig()
	cfg1.Routing = di.MustTestRoutingConfig("failover")
	cfg1.Routing.FailoverTimeout = 5000

	cfgSvc := di.NewConfigServiceWithConfig(&cfg1)
	routerSvc := di.NewRouterServiceWithConfigService(cfgSvc)

	// Get initial router
	rr1 := routerSvc.GetRouter()
	assert.Equal(t, "failover", rr1.Name())

	// Change timeout
	cfg2 := di.MustTestConfig()
	cfg2.Routing = di.MustTestRoutingConfig("failover")
	cfg2.Routing.FailoverTimeout = 10000

	cfgSvc.GetConfigAtomic().Store(&cfg2)
	cfgSvc.Config = &cfg2

	// Live router should pick up new timeout on next Select
	rr2 := routerSvc.GetRouter()
	assert.NotSame(t, rr1, rr2, "Router should be rebuilt when timeout changes")

	// Verify via live router
	liveRouter := router.NewLiveRouter(routerSvc.GetRouterAsFunc())
	assert.Equal(t, "failover", liveRouter.Name())
}
