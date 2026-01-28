package keypool

import (
	"context"
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property-based tests for KeyPool

func TestKeyPoolProperties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: GetKey always terminates (returns key or error, never hangs)
	properties.Property("GetKey terminates", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true // Skip invalid cases
			}

			pool := createTestPoolWithNKeys(keyCount)
			ctx := context.Background()

			// Should complete without hanging
			_, _, err := pool.GetKey(ctx)

			// Either we get a key or an error - both are valid termination
			return err != nil || true
		},
		gen.IntRange(1, 20),
	))

	// Property 2: Selected key is always in the pool
	properties.Property("selected key belongs to pool", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			pool := createTestPoolWithNKeys(keyCount)
			ctx := context.Background()

			keyID, _, err := pool.GetKey(ctx)
			if err != nil {
				return true // No key selected, property holds vacuously
			}

			// Verify the key ID exists in the pool
			return pool.containsKeyID(keyID)
		},
		gen.IntRange(1, 20),
	))

	// Property 3: GetKey returns valid API key (non-empty) on success
	properties.Property("GetKey returns non-empty API key on success", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			pool := createTestPoolWithNKeys(keyCount)
			ctx := context.Background()

			_, apiKey, err := pool.GetKey(ctx)
			if err != nil {
				return true // Error is fine
			}

			return apiKey != ""
		},
		gen.IntRange(1, 20),
	))

	// Property 4: GetStats totals are correct
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

	// Property 5: Available + Exhausted = Total
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

	// Property 6: Empty pool returns error
	properties.Property("empty pool config returns error", prop.ForAll(
		func(_ bool) bool {
			cfg := PoolConfig{
				Strategy: "least_loaded",
				Keys:     []KeyConfig{},
			}

			pool, err := NewKeyPool("test", cfg)
			return pool == nil && err != nil
		},
		gen.Bool(),
	))

	// Property 7: All healthy keys have non-zero capacity score
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

func TestKeyPoolConcurrentAccessProperties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	// Property: Concurrent GetKey calls don't panic
	properties.Property("concurrent GetKey is safe", prop.ForAll(
		func(goroutines int) bool {
			if goroutines <= 0 || goroutines > 100 {
				return true
			}

			pool := createTestPoolWithNKeys(5)
			ctx := context.Background()

			done := make(chan bool, goroutines)

			for i := 0; i < goroutines; i++ {
				go func() {
					defer func() {
						if r := recover(); r != nil {
							done <- false
							return
						}
						done <- true
					}()
					_, _, _ = pool.GetKey(ctx)
				}()
			}

			// Wait for all goroutines
			for i := 0; i < goroutines; i++ {
				if !<-done {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 50),
	))

	// Property: Concurrent GetStats doesn't panic
	properties.Property("concurrent GetStats is safe", prop.ForAll(
		func(goroutines int) bool {
			if goroutines <= 0 || goroutines > 100 {
				return true
			}

			pool := createTestPoolWithNKeys(5)
			done := make(chan bool, goroutines)

			for i := 0; i < goroutines; i++ {
				go func() {
					defer func() {
						if r := recover(); r != nil {
							done <- false
							return
						}
						done <- true
					}()
					_ = pool.GetStats()
				}()
			}

			for i := 0; i < goroutines; i++ {
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

func createTestPoolWithNKeys(n int) *KeyPool {
	keys := make([]KeyConfig, n)
	for i := 0; i < n; i++ {
		keys[i] = KeyConfig{
			APIKey:    fmt.Sprintf("sk-test-property-%d", i),
			RPMLimit:  100, // High limit to avoid rate limiting in tests
			ITPMLimit: 100000,
			OTPMLimit: 100000,
			Priority:  1,
			Weight:    1,
		}
	}

	cfg := PoolConfig{
		Strategy: "least_loaded",
		Keys:     keys,
	}

	pool, err := NewKeyPool("test-property", cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create property test pool: %v", err))
	}

	return pool
}

// containsKeyID checks if a key ID exists in the pool (for property tests).
func (p *KeyPool) containsKeyID(keyID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, ok := p.keyMap[keyID]
	return ok
}
