package config

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRuntime_GetStore verifies atomic config storage and retrieval.
func TestRuntime_GetStore(t *testing.T) {
	t.Parallel()

	cfg1 := &Config{
		Routing: RoutingConfig{
			Strategy: "round_robin",
		},
	}

	runtime := NewRuntime(cfg1)

	// Initial config should be retrievable
	retrieved := runtime.Get()
	assert.Equal(t, cfg1, retrieved, "Initial config should be retrievable")
	assert.Equal(t, "round_robin", retrieved.Routing.Strategy)

	// Store a new config
	cfg2 := &Config{
		Routing: RoutingConfig{
			Strategy: "failover",
		},
	}
	runtime.Store(cfg2)

	// New config should be retrievable
	retrieved2 := runtime.Get()
	assert.Equal(t, cfg2, retrieved2, "New config should be retrievable")
	assert.Equal(t, "failover", retrieved2.Routing.Strategy)
}

// TestRuntime_ConcurrentAccess verifies thread-safe config access.
func TestRuntime_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	runtime := NewRuntime(&Config{
		Routing: RoutingConfig{Strategy: "round_robin"},
	})

	// Concurrent reads and writes with WaitGroup to ensure both goroutines complete
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = runtime.Get()
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			runtime.Store(&Config{
				Routing: RoutingConfig{Strategy: "failover"},
			})
		}
	}()

	wg.Wait()

	// Final retrieval should work
	cfg := runtime.Get()
	assert.NotNil(t, cfg)
}

// TestRuntime_ImplementsRuntimeConfig verifies interface compliance.
func TestRuntime_ImplementsRuntimeConfig(t *testing.T) {
	t.Parallel()

	var _ RuntimeConfig = (*Runtime)(nil)

	runtime := NewRuntime(&Config{})
	assert.Implements(t, (*RuntimeConfig)(nil), runtime)
}
