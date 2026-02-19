package di_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/di"
	"github.com/omarluq/cc-relay/internal/router"
)

// mustTestConfigWithStrategy creates a full config.Config with the given routing strategy and timeout.
func mustTestConfigWithStrategy(strategy string, failoverTimeout int) *config.Config {
	cfg := di.MustTestConfig()
	cfg.Routing = di.MustTestRoutingConfig(strategy)
	cfg.Routing.FailoverTimeout = failoverTimeout
	return &cfg
}

// TestHotReload_RoutingStrategy verifies that changing routing strategy
// in config and triggering reload updates the router without restart.
func TestHotReloadRoutingStrategy(t *testing.T) {
	t.Parallel()

	// Start with config A: round_robin strategy
	configA := mustTestConfigWithStrategy(router.StrategyRoundRobin, 5000)

	// Create config service with initial config
	cfgSvc := di.NewConfigServiceUninitialized()
	cfgSvc.GetConfigAtomic().Store(configA)
	cfgSvc.Config = configA

	// Create router service
	routerSvc := di.NewRouterServiceWithConfigService(cfgSvc)

	// Verify initial strategy is round_robin
	router1 := routerSvc.GetRouter()
	assert.Equal(t, router.StrategyRoundRobin, router1.Name(),
		"Initial router should use round_robin strategy")

	// Update to config B: failover strategy
	configB := mustTestConfigWithStrategy(router.StrategyFailover, 3000)

	// Simulate hot-reload by storing new config
	cfgSvc.GetConfigAtomic().Store(configB)
	cfgSvc.Config = configB

	// Verify router now uses failover strategy
	router2 := routerSvc.GetRouter()
	assert.Equal(t, router.StrategyFailover, router2.Name(),
		"Router after reload should use failover strategy")
}

// TestHotReload_LiveRouter verifies that LiveRouter delegates
// to the current router after config changes.
func TestHotReloadLiveRouter(t *testing.T) {
	t.Parallel()

	configA := mustTestConfigWithStrategy(router.StrategyShuffle, 5000)
	configB := mustTestConfigWithStrategy(router.StrategyRoundRobin, 5000)

	cfgSvc := di.NewConfigServiceUninitialized()
	cfgSvc.GetConfigAtomic().Store(configA)
	cfgSvc.Config = configA

	routerSvc := di.NewRouterServiceWithConfigService(cfgSvc)

	// Create a LiveRouter that uses GetRouter
	liveRouter := router.NewLiveRouter(routerSvc.GetRouter)

	// Initial state: shuffle
	assert.Equal(t, router.StrategyShuffle, liveRouter.Name(),
		"LiveRouter should initially use shuffle")

	// Hot-reload to round_robin
	cfgSvc.GetConfigAtomic().Store(configB)
	cfgSvc.Config = configB

	// LiveRouter should now delegate to round_robin
	assert.Equal(t, router.StrategyRoundRobin, liveRouter.Name(),
		"LiveRouter should use round_robin after reload")
}

// TestHotReload_ConcurrentAccess verifies that concurrent config reads
// during hot-reload don't cause races or panics.
//
//nolint:cyclop // test requires multiple concurrent paths
func TestHotReloadConcurrentAccess(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	initialCfg := mustTestConfigWithStrategy(router.StrategyRoundRobin, 0)
	cfgSvc := di.NewConfigServiceUninitialized()
	cfgSvc.GetConfigAtomic().Store(initialCfg)

	routerSvc := di.NewRouterServiceWithConfigService(cfgSvc)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Goroutine 1: continuously read router
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_ = routerSvc.GetRouter()
			}
		}
	}()

	// Goroutine 2: continuously update config
	updateDone := make(chan struct{})
	go func() {
		defer close(updateDone)
		strategies := []string{
			router.StrategyRoundRobin,
			router.StrategyShuffle,
			router.StrategyFailover,
		}
		idx := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
				newCfg := mustTestConfigWithStrategy(strategies[idx%len(strategies)], 0)
				cfgSvc.GetConfigAtomic().Store(newCfg)
				cfgSvc.Config = newCfg
				idx++
			}
		}
	}()

	// Wait for timeout
	<-ctx.Done()

	// Verify both goroutines completed without panic
	select {
	case <-readDone:
		// Reader completed
	case <-time.After(time.Second):
		t.Fatal("Reader goroutine did not complete")
	}

	select {
	case <-updateDone:
		// Updater completed
	case <-time.After(time.Second):
		t.Fatal("Updater goroutine did not complete")
	}

	// Final router should be valid
	finalRouter := routerSvc.GetRouter()
	assert.NotNil(t, finalRouter, "Final router should not be nil")
	assert.NotEmpty(t, finalRouter.Name(), "Final router should have a name")
}

// TestConfigService_GetVsDirect verifies that Get() returns the current
// config while direct Config field may become stale after hot-reload.
func TestConfigServiceGetVsDirect(t *testing.T) {
	t.Parallel()

	cfgSvc := di.NewConfigServiceUninitialized()
	initialCfg := mustTestConfigWithStrategy(router.StrategyRoundRobin, 0)
	cfgSvc.GetConfigAtomic().Store(initialCfg)
	cfgSvc.Config = initialCfg // Both point to same initially

	// Initially both should return the same
	assert.Equal(t, cfgSvc.Config, cfgSvc.Get(),
		"Initially Config and Get() should return same")

	// Simulate hot-reload: update atomic pointer but not Config field
	newCfg := mustTestConfigWithStrategy(router.StrategyFailover, 0)
	cfgSvc.GetConfigAtomic().Store(newCfg)
	cfgSvc.Config = newCfg // Also update Config field (as the watcher does)

	// Get() returns the new config
	assert.Equal(t, router.StrategyFailover, cfgSvc.Get().Routing.Strategy,
		"Get() should return new config after hot-reload")

	// Config field should also point to new config
	assert.Equal(t, router.StrategyFailover, cfgSvc.Config.Routing.Strategy,
		"Config field should also be updated after hot-reload")
}

// BenchmarkHotReload_GetRouter benchmarks the per-request router creation.
// This establishes a baseline for hot-reload performance overhead.
func BenchmarkHotReloadGetRouter(b *testing.B) {
	cfgSvc := di.NewConfigServiceUninitialized()
	benchCfg := mustTestConfigWithStrategy(router.StrategyRoundRobin, 5000)
	cfgSvc.GetConfigAtomic().Store(benchCfg)
	cfgSvc.Config = cfgSvc.GetConfigAtomic().Load()

	routerSvc := di.NewRouterServiceWithConfigService(cfgSvc)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = routerSvc.GetRouter()
	}
}

// BenchmarkHotReload_AtomicStore benchmarks the config swap operation.
func BenchmarkHotReloadAtomicStore(b *testing.B) {
	cfgSvc := di.NewConfigServiceUninitialized()
	storeCfg := di.MustTestConfig()
	cfgSvc.GetConfigAtomic().Store(&storeCfg)
	_ = cfgSvc.GetConfigAtomic().Load() // Initialize Config field (unused in benchmark)

	newCfg := mustTestConfigWithStrategy(router.StrategyFailover, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfgSvc.GetConfigAtomic().Store(newCfg)
	}
}
