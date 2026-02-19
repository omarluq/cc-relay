package keypool_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/omarluq/cc-relay/internal/keypool"
)

// Property-based tests for KeyPool - split into focused test functions to reduce cognitive complexity.

func TestKeyPoolTerminationProperty(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("GetKey terminates", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			pool := createTestPoolWithNKeys(keyCount)
			ctx := context.Background()

			_, _, err := pool.GetKey(ctx)
			return err != nil || true
		},
		gen.IntRange(1, 20),
	))

	properties.Property("selected key belongs to pool", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			pool := createTestPoolWithNKeys(keyCount)
			ctx := context.Background()

			keyID, _, err := pool.GetKey(ctx)
			if err != nil {
				return true
			}

			return pool.ContainsKeyID(keyID)
		},
		gen.IntRange(1, 20),
	))

	properties.Property("GetKey returns non-empty API key on success", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			pool := createTestPoolWithNKeys(keyCount)
			ctx := context.Background()

			_, resultAPIKey, err := pool.GetKey(ctx)
			if err != nil {
				return true
			}

			return resultAPIKey != ""
		},
		gen.IntRange(1, 20),
	))

	properties.TestingRun(t)
}

func TestKeyPoolStatsProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("stats total equals key count", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			pool := createTestPoolWithNKeys(keyCount)
			stats := pool.GetStats()

			return stats.TotalKeys == keyCount
		},
		gen.IntRange(1, 50),
	))

	properties.Property("available plus exhausted equals total", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			pool := createTestPoolWithNKeys(keyCount)
			stats := pool.GetStats()

			return stats.AvailableKeys+stats.ExhaustedKeys == stats.TotalKeys
		},
		gen.IntRange(1, 50),
	))

	properties.TestingRun(t)
}

func TestKeyPoolConfigValidationProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("empty pool config returns error", prop.ForAll(
		func(_ bool) bool {
			cfg := keypool.PoolConfig{
				Strategy: "least_loaded",
				Keys:     []keypool.KeyConfig{},
			}

			pool, err := keypool.NewKeyPool("test", cfg)
			return pool == nil && err != nil
		},
		gen.Bool(),
	))

	properties.Property("healthy keys have positive capacity", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			pool := createTestPoolWithNKeys(keyCount)
			keys := pool.Keys()

			for _, key := range keys {
				if key.IsAvailable() && key.GetCapacityScore() <= 0 {
					return false
				}
			}
			return true
		},
		gen.IntRange(1, 20),
	))

	properties.TestingRun(t)
}

// safeGetKey calls pool.GetKey in a goroutine and reports panic via done channel.
func safeGetKey(pool *keypool.KeyPool, done chan<- bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			done <- false
			return
		}
		done <- true
	}()
	ctx := context.Background()
	if _, _, getKeyErr := pool.GetKey(ctx); getKeyErr != nil {
		return
	}
}

func TestKeyPoolConcurrentGetKeyProperty(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	properties.Property("concurrent GetKey is safe", prop.ForAll(
		func(goroutines int) bool {
			if goroutines <= 0 || goroutines > 100 {
				return true
			}

			pool := createTestPoolWithNKeys(5)
			done := make(chan bool, goroutines)

			for goroutineIdx := 0; goroutineIdx < goroutines; goroutineIdx++ {
				go safeGetKey(pool, done)
			}

			for goroutineIdx := 0; goroutineIdx < goroutines; goroutineIdx++ {
				if !<-done {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 50),
	))

	properties.TestingRun(t)
}

func TestKeyPoolConcurrentGetStatsProperty(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	properties.Property("concurrent GetStats is safe", prop.ForAll(
		func(goroutines int) bool {
			if goroutines <= 0 || goroutines > 100 {
				return true
			}

			pool := createTestPoolWithNKeys(5)
			done := make(chan bool, goroutines)

			for goroutineIdx := 0; goroutineIdx < goroutines; goroutineIdx++ {
				go func() {
					defer func() {
						if recovered := recover(); recovered != nil {
							done <- false
							return
						}
						done <- true
					}()
					_ = pool.GetStats()
				}()
			}

			for goroutineIdx := 0; goroutineIdx < goroutines; goroutineIdx++ {
				if !<-done {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 50),
	))

	properties.TestingRun(t)
}

// Helper functions

func createTestPoolWithNKeys(numKeys int) *keypool.KeyPool {
	keys := make([]keypool.KeyConfig, numKeys)
	for idx := 0; idx < numKeys; idx++ {
		keys[idx] = keypool.KeyConfig{
			APIKey:    fmt.Sprintf("sk-test-property-%d", idx),
			RPMLimit:  100, // High limit to avoid rate limiting in tests
			ITPMLimit: 100000,
			OTPMLimit: 100000,
			Priority:  1,
			Weight:    1,
		}
	}

	cfg := keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys:     keys,
	}

	pool, err := keypool.NewKeyPool("test-property", cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create property test pool: %v", err))
	}

	return pool
}
