package config_test

import (
	"github.com/omarluq/cc-relay/internal/config"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	strategyRoundRobin = "round_robin"
	strategyFailover   = "failover"
)

// TestRuntime_GetStore verifies atomic config storage and retrieval.
func TestRuntimeGetStore(t *testing.T) {
	t.Parallel()

	cfg1 := config.MakeTestConfig()
	cfg1.Routing.Strategy = strategyRoundRobin

	runtime := config.NewRuntime(cfg1)

	// Initial config should be retrievable
	retrieved := runtime.Get()
	assert.Equal(t, cfg1, retrieved, "Initial config should be retrievable")
	assert.Equal(t, strategyRoundRobin, retrieved.Routing.Strategy)

	// Store a new config
	cfg2 := config.MakeTestConfig()
	cfg2.Routing.Strategy = strategyFailover
	runtime.Store(cfg2)

	// New config should be retrievable
	retrieved2 := runtime.Get()
	assert.Equal(t, cfg2, retrieved2, "New config should be retrievable")
	assert.Equal(t, strategyFailover, retrieved2.Routing.Strategy)
}

// TestRuntime_ConcurrentAccess verifies thread-safe config access.
func TestRuntimeConcurrentAccess(t *testing.T) {
	t.Parallel()

	cfg := config.MakeTestConfig()
	cfg.Routing.Strategy = strategyRoundRobin
	runtime := config.NewRuntime(cfg)

	// Concurrent reads and writes with WaitGroup to ensure both goroutines complete
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)

	go func() {
		defer waitGroup.Done()
		for idx := 0; idx < 1000; idx++ {
			_ = runtime.Get()
		}
	}()

	go func() {
		defer waitGroup.Done()
		for idx := 0; idx < 100; idx++ {
			cfg := config.MakeTestConfig()
			cfg.Routing.Strategy = strategyFailover
			runtime.Store(cfg)
		}
	}()

	waitGroup.Wait()

	// Final retrieval should work
	finalCfg := runtime.Get()
	assert.NotNil(t, finalCfg)
}

// TestRuntime_ImplementsRuntimeConfigGetter verifies interface compliance.
func TestRuntimeImplementsRuntimeConfigGetter(t *testing.T) {
	t.Parallel()

	var _ config.RuntimeConfigGetter = (*config.Runtime)(nil)

	runtime := config.NewRuntime(config.MakeTestConfig())
	assert.Implements(t, (*config.RuntimeConfigGetter)(nil), runtime)
}
