package di

import (
	"context"
	"testing"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/router"
	"github.com/stretchr/testify/assert"
)

// TestLiveRouter_HotReloadStrategyChange verifies that changing the routing
// strategy takes effect without rebuilding the handler.
//
// This is a regression test for F1 in claude-code-action-items.md.
func TestLiveRouterHotReloadStrategyChange(t *testing.T) {
	t.Parallel()

	// Start with round_robin strategy
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
	liveRouter := router.NewLiveRouter(routerSvc.GetRouterAsFunc())

	providers := []router.ProviderInfo{
		{Weight: 1, Priority: 1, IsHealthy: func() bool { return true }},
		{Weight: 2, Priority: 2, IsHealthy: func() bool { return true }},
		{Weight: 3, Priority: 3, IsHealthy: func() bool { return true }},
	}

	// With round_robin, should distribute across providers
	roundRobinSelections := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		selected, err := liveRouter.Select(context.Background(), providers)
		assert.NoError(t, err)
		roundRobinSelections = append(roundRobinSelections, selected.Weight)
	}

	// Round-robin should cycle: 1, 2, 3, 1
	assert.Equal(t, []int{1, 2, 3, 1}, roundRobinSelections,
		"Round-robin should distribute across providers")

	// Now change strategy to failover
	cfg2 := &config.Config{
		Routing: config.RoutingConfig{
			Strategy: "failover",
		},
	}
	cfgSvc.config.Store(cfg2)

	// With failover, should always select highest priority (higher priority number = higher priority)
	// Provider with Priority 3 (Weight 3) should always be selected
	failoverSelections := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		selected, err := liveRouter.Select(context.Background(), providers)
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

	// Get initial router
	rr1 := routerSvc.GetRouter()
	assert.Equal(t, "failover", rr1.Name())

	// Change timeout
	cfg2 := &config.Config{
		Routing: config.RoutingConfig{
			Strategy:        "failover",
			FailoverTimeout: 10000, // 10 seconds
		},
	}
	cfgSvc.config.Store(cfg2)

	// Live router should pick up new timeout on next Select
	rr2 := routerSvc.GetRouter()
	assert.NotSame(t, rr1, rr2, "Router should be rebuilt when timeout changes")

	// Verify via live router
	liveRouter := router.NewLiveRouter(routerSvc.GetRouterAsFunc())
	assert.Equal(t, "failover", liveRouter.Name())
}
