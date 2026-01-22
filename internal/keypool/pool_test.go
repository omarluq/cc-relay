package keypool

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers

func newTestPool(numKeys int, strategy string) *KeyPool {
	keys := make([]KeyConfig, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = KeyConfig{
			APIKey:    fmt.Sprintf("sk-test-key-%d", i),
			RPMLimit:  50,
			ITPMLimit: 30000,
			OTPMLimit: 30000,
			Priority:  1,
			Weight:    1,
		}
	}

	cfg := PoolConfig{
		Strategy: strategy,
		Keys:     keys,
	}

	pool, err := NewKeyPool("test-provider", cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create test pool: %v", err))
	}

	return pool
}

func newTestHeaders(remaining int, reset time.Time) http.Header {
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-requests-limit", "50")
	headers.Set("anthropic-ratelimit-requests-remaining", fmt.Sprintf("%d", remaining))
	headers.Set("anthropic-ratelimit-requests-reset", reset.Format(time.RFC3339))
	return headers
}

// Tests

func TestNewKeyPool(t *testing.T) {
	t.Run("creates pool with valid config", func(t *testing.T) {
		pool := newTestPool(3, "least_loaded")

		assert.NotNil(t, pool)
		assert.Equal(t, "test-provider", pool.provider)
		assert.Len(t, pool.keys, 3)
		assert.Len(t, pool.keyMap, 3)
		assert.Len(t, pool.limiters, 3)
		assert.NotNil(t, pool.selector)
	})

	t.Run("returns error with no keys", func(t *testing.T) {
		cfg := PoolConfig{
			Strategy: "least_loaded",
			Keys:     []KeyConfig{},
		}

		pool, err := NewKeyPool("test-provider", cfg)

		assert.Nil(t, pool)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no keys configured")
	})

	t.Run("creates selector matching strategy", func(t *testing.T) {
		pool := newTestPool(2, "round_robin")

		assert.NotNil(t, pool.selector)
		assert.Equal(t, "round_robin", pool.selector.Name())
	})

	t.Run("initializes all keys with limiters", func(t *testing.T) {
		pool := newTestPool(3, "least_loaded")

		for _, key := range pool.keys {
			assert.NotEmpty(t, key.ID)
			assert.NotEmpty(t, key.APIKey)

			limiter, ok := pool.limiters[key.ID]
			assert.True(t, ok)
			assert.NotNil(t, limiter)
		}
	})
}

func TestGetKey_Success(t *testing.T) {
	t.Run("returns key when capacity available", func(t *testing.T) {
		pool := newTestPool(3, "least_loaded")
		ctx := context.Background()

		keyID, apiKey, err := pool.GetKey(ctx)

		assert.NoError(t, err)
		assert.NotEmpty(t, keyID)
		assert.NotEmpty(t, apiKey)
		assert.Contains(t, apiKey, "sk-test-key-")
	})

	t.Run("returns keyID and apiKey", func(t *testing.T) {
		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		keyID, apiKey, err := pool.GetKey(ctx)

		require.NoError(t, err)

		// Verify keyID matches one of the keys in pool
		key, ok := pool.keyMap[keyID]
		require.True(t, ok)
		assert.Equal(t, key.APIKey, apiKey)
	})

	t.Run("selector strategy is respected", func(t *testing.T) {
		pool := newTestPool(3, "least_loaded")
		ctx := context.Background()

		// Deplete first key
		firstKey := pool.keys[0]
		headers := newTestHeaders(0, time.Now().Add(time.Minute))
		err := pool.UpdateKeyFromHeaders(firstKey.ID, headers)
		require.NoError(t, err)

		// Should pick a different key (one with higher capacity)
		keyID, _, err := pool.GetKey(ctx)
		require.NoError(t, err)
		assert.NotEqual(t, firstKey.ID, keyID)
	})
}

func TestGetKey_AllExhausted(t *testing.T) {
	t.Run("all keys at capacity returns error", func(t *testing.T) {
		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		// Exhaust rate limiters by consuming all capacity
		// Each key has burst=50, so 2 keys * 50 = 100 total capacity
		for i := 0; i < 100; i++ {
			_, _, _ = pool.GetKey(ctx)
		}

		// Next request should fail (all keys exhausted)
		_, _, err := pool.GetKey(ctx)
		assert.ErrorIs(t, err, ErrAllKeysExhausted)
	})

	t.Run("all keys unhealthy returns error", func(t *testing.T) {
		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		// Mark all keys unhealthy
		for _, key := range pool.keys {
			key.MarkUnhealthy(fmt.Errorf("test error"))
		}

		_, _, err := pool.GetKey(ctx)
		assert.ErrorIs(t, err, ErrAllKeysExhausted)
	})

	t.Run("all keys in cooldown returns error", func(t *testing.T) {
		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		// Put all keys in cooldown
		for _, key := range pool.keys {
			pool.MarkKeyExhausted(key.ID, 10*time.Second)
		}

		_, _, err := pool.GetKey(ctx)
		assert.ErrorIs(t, err, ErrAllKeysExhausted)
	})
}

func TestGetKey_SkipsUnavailable(t *testing.T) {
	t.Run("skips unhealthy keys", func(t *testing.T) {
		pool := newTestPool(3, "least_loaded")
		ctx := context.Background()

		// Mark first two keys unhealthy
		pool.keys[0].MarkUnhealthy(fmt.Errorf("test error 1"))
		pool.keys[1].MarkUnhealthy(fmt.Errorf("test error 2"))

		// Should return third key
		keyID, _, err := pool.GetKey(ctx)
		require.NoError(t, err)
		assert.Equal(t, pool.keys[2].ID, keyID)
	})

	t.Run("skips cooldown keys", func(t *testing.T) {
		pool := newTestPool(3, "least_loaded")
		ctx := context.Background()

		// Put first key in cooldown
		pool.MarkKeyExhausted(pool.keys[0].ID, 10*time.Second)

		// Should skip first key
		keyID, _, err := pool.GetKey(ctx)
		require.NoError(t, err)
		assert.NotEqual(t, pool.keys[0].ID, keyID)
	})

	t.Run("skips rate-limited keys", func(t *testing.T) {
		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		// Exhaust first key
		firstKey := pool.keys[0]
		for i := 0; i < 60; i++ { // Exhaust burst capacity
			limiter := pool.limiters[firstKey.ID]
			_ = limiter.Allow(ctx)
		}

		// Should pick second key
		keyID, _, err := pool.GetKey(ctx)
		require.NoError(t, err)
		assert.Equal(t, pool.keys[1].ID, keyID)
	})
}

func TestUpdateKeyFromHeaders(t *testing.T) {
	t.Run("updates key limits from headers", func(t *testing.T) {
		pool := newTestPool(1, "least_loaded")
		key := pool.keys[0]

		headers := http.Header{}
		headers.Set("anthropic-ratelimit-requests-limit", "100")
		headers.Set("anthropic-ratelimit-input-tokens-limit", "50000")
		headers.Set("anthropic-ratelimit-output-tokens-limit", "50000")

		err := pool.UpdateKeyFromHeaders(key.ID, headers)
		require.NoError(t, err)

		key.mu.RLock()
		assert.Equal(t, 100, key.RPMLimit)
		assert.Equal(t, 50000, key.ITPMLimit)
		assert.Equal(t, 50000, key.OTPMLimit)
		key.mu.RUnlock()
	})

	t.Run("updates remaining capacity from headers", func(t *testing.T) {
		pool := newTestPool(1, "least_loaded")
		key := pool.keys[0]

		headers := http.Header{}
		headers.Set("anthropic-ratelimit-requests-remaining", "42")
		headers.Set("anthropic-ratelimit-input-tokens-remaining", "27000")
		headers.Set("anthropic-ratelimit-output-tokens-remaining", "27000")

		err := pool.UpdateKeyFromHeaders(key.ID, headers)
		require.NoError(t, err)

		key.mu.RLock()
		assert.Equal(t, 42, key.RPMRemaining)
		assert.Equal(t, 27000, key.ITPMRemaining)
		assert.Equal(t, 27000, key.OTPMRemaining)
		key.mu.RUnlock()
	})

	t.Run("updates reset time from headers", func(t *testing.T) {
		pool := newTestPool(1, "least_loaded")
		key := pool.keys[0]

		resetTime := time.Now().Add(time.Minute)
		headers := http.Header{}
		headers.Set("anthropic-ratelimit-requests-reset", resetTime.Format(time.RFC3339))

		err := pool.UpdateKeyFromHeaders(key.ID, headers)
		require.NoError(t, err)

		key.mu.RLock()
		assert.WithinDuration(t, resetTime, key.RPMResetAt, time.Second)
		key.mu.RUnlock()
	})

	t.Run("returns error for unknown keyID", func(t *testing.T) {
		pool := newTestPool(1, "least_loaded")

		headers := http.Header{}
		err := pool.UpdateKeyFromHeaders("unknown-key-id", headers)

		assert.ErrorIs(t, err, ErrKeyNotFound)
	})
}

func TestMarkKeyExhausted(t *testing.T) {
	t.Run("sets cooldown period", func(t *testing.T) {
		pool := newTestPool(1, "least_loaded")
		key := pool.keys[0]

		retryAfter := 30 * time.Second
		pool.MarkKeyExhausted(key.ID, retryAfter)

		key.mu.RLock()
		cooldown := key.CooldownUntil
		key.mu.RUnlock()

		assert.WithinDuration(t, time.Now().Add(retryAfter), cooldown, time.Second)
	})

	t.Run("key becomes unavailable during cooldown", func(t *testing.T) {
		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		firstKey := pool.keys[0]
		pool.MarkKeyExhausted(firstKey.ID, 10*time.Second)

		// Should skip exhausted key
		keyID, _, err := pool.GetKey(ctx)
		require.NoError(t, err)
		assert.NotEqual(t, firstKey.ID, keyID)
	})

	t.Run("key becomes available after cooldown expires", func(t *testing.T) {
		pool := newTestPool(1, "least_loaded")
		ctx := context.Background()

		key := pool.keys[0]
		pool.MarkKeyExhausted(key.ID, 100*time.Millisecond)

		// Should be unavailable immediately
		assert.False(t, key.IsAvailable())

		// Wait for cooldown to expire
		time.Sleep(150 * time.Millisecond)

		// Should be available now
		assert.True(t, key.IsAvailable())

		// Should be able to get key
		keyID, _, err := pool.GetKey(ctx)
		require.NoError(t, err)
		assert.Equal(t, key.ID, keyID)
	})
}

func TestGetEarliestResetTime(t *testing.T) {
	t.Run("returns time to earliest reset", func(t *testing.T) {
		pool := newTestPool(3, "least_loaded")

		// Set different reset times
		resetTimes := []time.Time{
			time.Now().Add(60 * time.Second),
			time.Now().Add(30 * time.Second), // Earliest
			time.Now().Add(90 * time.Second),
		}

		for i, key := range pool.keys {
			headers := newTestHeaders(25, resetTimes[i])
			err := pool.UpdateKeyFromHeaders(key.ID, headers)
			require.NoError(t, err)
		}

		duration := pool.GetEarliestResetTime()

		// Should be ~30 seconds (with some tolerance)
		assert.InDelta(t, 30.0, duration.Seconds(), 2.0)
	})

	t.Run("returns 60s default when no reset times set", func(t *testing.T) {
		pool := newTestPool(2, "least_loaded")

		duration := pool.GetEarliestResetTime()

		assert.Equal(t, 60*time.Second, duration)
	})
}

func TestGetStats(t *testing.T) {
	t.Run("returns correct counts", func(t *testing.T) {
		pool := newTestPool(3, "least_loaded")

		stats := pool.GetStats()

		assert.Equal(t, 3, stats.TotalKeys)
		assert.Equal(t, 3, stats.AvailableKeys)
		assert.Equal(t, 0, stats.ExhaustedKeys)
		assert.Equal(t, 150, stats.TotalRPM) // 3 keys * 50 RPM
	})

	t.Run("updates after key state changes", func(t *testing.T) {
		pool := newTestPool(3, "least_loaded")

		// Mark one key unhealthy
		pool.keys[0].MarkUnhealthy(fmt.Errorf("test error"))

		// Put one key in cooldown
		pool.MarkKeyExhausted(pool.keys[1].ID, 10*time.Second)

		stats := pool.GetStats()

		assert.Equal(t, 3, stats.TotalKeys)
		assert.Equal(t, 1, stats.AvailableKeys)
		assert.Equal(t, 2, stats.ExhaustedKeys)
	})

	t.Run("tracks remaining capacity", func(t *testing.T) {
		pool := newTestPool(2, "least_loaded")

		// Update one key's remaining capacity
		headers := newTestHeaders(25, time.Now().Add(time.Minute))
		err := pool.UpdateKeyFromHeaders(pool.keys[0].ID, headers)
		require.NoError(t, err)

		stats := pool.GetStats()

		assert.Equal(t, 100, stats.TotalRPM)
		assert.Equal(t, 75, stats.RemainingRPM) // 25 + 50
	})
}

//nolint:gocognit // Test function complexity is acceptable for comprehensive coverage
func TestConcurrency(t *testing.T) {
	t.Run("multiple goroutines calling GetKey", func(_ *testing.T) {
		pool := newTestPool(5, "least_loaded")
		ctx := context.Background()

		const numGoroutines = 50
		const requestsPerGoroutine = 10

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < requestsPerGoroutine; j++ {
					_, _, _ = pool.GetKey(ctx)
				}
			}()
		}

		wg.Wait()
		// If we got here without data races, test passed
	})

	t.Run("multiple goroutines calling UpdateKeyFromHeaders", func(_ *testing.T) {
		pool := newTestPool(3, "least_loaded")

		const numGoroutines = 30

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(iteration int) {
				defer wg.Done()

				keyIdx := iteration % len(pool.keys)
				key := pool.keys[keyIdx]

				headers := newTestHeaders(25, time.Now().Add(time.Minute))
				_ = pool.UpdateKeyFromHeaders(key.ID, headers)
			}(i)
		}

		wg.Wait()
		// If we got here without data races, test passed
	})

	t.Run("no race conditions", func(_ *testing.T) {
		pool := newTestPool(5, "least_loaded")
		ctx := context.Background()

		const numGoroutines = 20

		var wg sync.WaitGroup
		wg.Add(numGoroutines * 2)

		// Half reading, half writing
		for i := 0; i < numGoroutines; i++ {
			// Reader goroutine
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					pool.GetKey(ctx)
					pool.GetStats()
					pool.Keys()
				}
			}()

			// Writer goroutine
			go func(iteration int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					keyIdx := iteration % len(pool.keys)
					key := pool.keys[keyIdx]

					headers := newTestHeaders(25, time.Now().Add(time.Minute))
					pool.UpdateKeyFromHeaders(key.ID, headers)
					pool.MarkKeyExhausted(key.ID, 1*time.Millisecond)
				}
			}(i)
		}

		wg.Wait()
		// Run with -race flag to verify no data races
	})

	t.Run("fair distribution under load", func(t *testing.T) {
		pool := newTestPool(3, "round_robin")
		ctx := context.Background()

		keyUsage := make(map[string]int)
		var mu sync.Mutex

		// Use fewer requests to avoid exhausting rate limiters
		// Each key has burst=50, so 3 keys * 50 = 150 capacity
		const numRequests = 120 // Stay under total capacity

		var wg sync.WaitGroup
		wg.Add(numRequests)

		for i := 0; i < numRequests; i++ {
			go func() {
				defer wg.Done()
				keyID, _, err := pool.GetKey(ctx)
				if err == nil {
					mu.Lock()
					keyUsage[keyID]++
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		// With round-robin, distribution should be relatively fair
		// Each key should get roughly 40 requests (120/3)
		totalUsed := 0
		for _, count := range keyUsage {
			totalUsed += count
		}

		// All requests should succeed (no rate limiting)
		assert.Equal(t, numRequests, totalUsed, "Some requests failed due to rate limiting")

		// Each key should get roughly equal usage (±50% tolerance due to concurrency)
		expectedPerKey := numRequests / 3
		tolerance := float64(expectedPerKey) * 0.5
		for keyID, count := range keyUsage {
			assert.InDelta(t, expectedPerKey, count, tolerance,
				"Key %s usage out of expected range", keyID)
		}
	})
}
