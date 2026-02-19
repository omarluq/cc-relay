package keypool_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers

func newTestPool(numKeys int, strategy string) *keypool.KeyPool {
	keys := make([]keypool.KeyConfig, numKeys)
	for idx := range numKeys {
		keys[idx] = keypool.KeyConfig{
			APIKey:    fmt.Sprintf("sk-test-key-%d", idx),
			RPMLimit:  50,
			ITPMLimit: 30000,
			OTPMLimit: 30000,
			Priority:  1,
			Weight:    1,
		}
	}

	cfg := keypool.PoolConfig{
		Strategy: strategy,
		Keys:     keys,
	}

	pool, err := keypool.NewKeyPool("test-provider", cfg)
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
	t.Parallel()
	t.Run("creates pool with valid config", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(3, "least_loaded")

		assert.NotNil(t, pool)
		assert.Equal(t, "test-provider", pool.GetProvider())
		assert.Len(t, pool.GetKeys(), 3)
		assert.Len(t, pool.GetKeyMap(), 3)
		assert.Equal(t, 3, pool.GetLimitersLen())
		assert.NotNil(t, pool.GetSelector())
	})

	t.Run("returns error with no keys", func(t *testing.T) {
		t.Parallel()
		cfg := keypool.PoolConfig{
			Strategy: "least_loaded",
			Keys:     []keypool.KeyConfig{},
		}

		pool, err := keypool.NewKeyPool("test-provider", cfg)

		assert.Nil(t, pool)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no keys configured")
	})

	t.Run("creates selector matching strategy", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(2, "round_robin")

		assert.NotNil(t, pool.GetSelector())
		assert.Equal(t, "round_robin", pool.GetSelector().Name())
	})

	t.Run("initializes all keys with limiters", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(3, "least_loaded")

		for _, key := range pool.GetKeys() {
			assert.NotEmpty(t, key.ID)
			assert.NotEmpty(t, key.APIKey)

			limiter, ok := pool.GetLimiters()[key.ID]
			assert.True(t, ok)
			assert.NotNil(t, limiter)
		}
	})
}

func TestGetKeySuccess(t *testing.T) {
	t.Parallel()
	t.Run("returns key when capacity available", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(3, "least_loaded")
		ctx := context.Background()

		keyID, apiKey, err := pool.GetKey(ctx)

		assert.NoError(t, err)
		assert.NotEmpty(t, keyID)
		assert.NotEmpty(t, apiKey)
		assert.Contains(t, apiKey, "sk-test-key-")
	})

	t.Run("returns keyID and apiKey", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		keyID, apiKey, err := pool.GetKey(ctx)

		require.NoError(t, err)

		// Verify keyID matches one of the keys in pool
		key, ok := pool.GetKeyMap()[keyID]
		require.True(t, ok)
		assert.Equal(t, key.APIKey, apiKey)
	})

	t.Run("selector strategy is respected", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(3, "least_loaded")
		ctx := context.Background()

		// Deplete first key
		firstKey := pool.GetKeys()[0]
		headers := newTestHeaders(0, time.Now().Add(time.Minute))
		err := pool.UpdateKeyFromHeaders(firstKey.ID, headers)
		require.NoError(t, err)

		// Should pick a different key (one with higher capacity)
		keyID, _, err := pool.GetKey(ctx)
		require.NoError(t, err)
		assert.NotEqual(t, firstKey.ID, keyID)
	})
}

func TestGetKeyAllExhausted(t *testing.T) {
	t.Parallel()
	t.Run("all keys at capacity returns error", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		// Exhaust rate limiters by consuming all capacity
		// Each key has burst=50, so 2 keys * 50 = 100 total capacity
		for range 100 {
			if _, _, exhaustErr := pool.GetKey(ctx); exhaustErr != nil {
				break
			}
		}

		// Next request should fail (all keys exhausted)
		_, _, err := pool.GetKey(ctx)
		assert.ErrorIs(t, err, keypool.ErrAllKeysExhausted)
	})

	t.Run("all keys unhealthy returns error", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		// Mark all keys unhealthy
		for _, key := range pool.GetKeys() {
			key.MarkUnhealthy(fmt.Errorf("test error"))
		}

		_, _, err := pool.GetKey(ctx)
		assert.ErrorIs(t, err, keypool.ErrAllKeysExhausted)
	})

	t.Run("all keys in cooldown returns error", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		// Put all keys in cooldown
		for _, key := range pool.GetKeys() {
			pool.MarkKeyExhausted(key.ID, 10*time.Second)
		}

		_, _, err := pool.GetKey(ctx)
		assert.ErrorIs(t, err, keypool.ErrAllKeysExhausted)
	})
}

func TestGetKeySkipsUnavailable(t *testing.T) {
	t.Parallel()
	t.Run("skips unhealthy keys", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(3, "least_loaded")
		ctx := context.Background()

		// Mark first two keys unhealthy
		pool.GetKeys()[0].MarkUnhealthy(fmt.Errorf("test error 1"))
		pool.GetKeys()[1].MarkUnhealthy(fmt.Errorf("test error 2"))

		// Should return third key
		keyID, _, err := pool.GetKey(ctx)
		require.NoError(t, err)
		assert.Equal(t, pool.GetKeys()[2].ID, keyID)
	})

	t.Run("skips cooldown keys", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(3, "least_loaded")
		ctx := context.Background()

		// Put first key in cooldown
		pool.MarkKeyExhausted(pool.GetKeys()[0].ID, 10*time.Second)

		// Should skip first key
		keyID, _, err := pool.GetKey(ctx)
		require.NoError(t, err)
		assert.NotEqual(t, pool.GetKeys()[0].ID, keyID)
	})

	t.Run("skips rate-limited keys", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		// Exhaust first key
		firstKey := pool.GetKeys()[0]
		for range 60 { // Exhaust burst capacity
			limiter := pool.GetLimiters()[firstKey.ID]
			_ = limiter.Allow(ctx)
		}

		// Should pick second key
		keyID, _, err := pool.GetKey(ctx)
		require.NoError(t, err)
		assert.Equal(t, pool.GetKeys()[1].ID, keyID)
	})
}

func TestUpdateKeyFromHeaders(t *testing.T) {
	t.Parallel()

	headerUpdateTests := []struct {
		setHeaders func(http.Header)
		assertions func(*testing.T, *keypool.KeyMetadata)
		name       string
	}{
		{
			name: "updates key limits from headers",
			setHeaders: func(headers http.Header) {
				headers.Set("anthropic-ratelimit-requests-limit", "100")
				headers.Set("anthropic-ratelimit-input-tokens-limit", "50000")
				headers.Set("anthropic-ratelimit-output-tokens-limit", "50000")
			},
			assertions: func(t *testing.T, key *keypool.KeyMetadata) {
				t.Helper()
				assert.Equal(t, 100, key.GetRPMLimit())
				assert.Equal(t, 50000, key.GetITPMLimit())
				assert.Equal(t, 50000, key.GetOTPMLimit())
			},
		},
		{
			name: "updates remaining capacity from headers",
			setHeaders: func(headers http.Header) {
				headers.Set("anthropic-ratelimit-requests-remaining", "42")
				headers.Set("anthropic-ratelimit-input-tokens-remaining", "27000")
				headers.Set("anthropic-ratelimit-output-tokens-remaining", "27000")
			},
			assertions: func(t *testing.T, key *keypool.KeyMetadata) {
				t.Helper()
				assert.Equal(t, 42, key.GetRPMRemaining())
				assert.Equal(t, 27000, key.GetITPMRemaining())
				assert.Equal(t, 27000, key.GetOTPMRemaining())
			},
		},
	}

	for _, headerTest := range headerUpdateTests {
		t.Run(headerTest.name, func(t *testing.T) {
			t.Parallel()
			pool := newTestPool(1, "least_loaded")
			key := pool.GetKeys()[0]

			headers := http.Header{}
			headerTest.setHeaders(headers)

			err := pool.UpdateKeyFromHeaders(key.ID, headers)
			require.NoError(t, err)

			headerTest.assertions(t, key)
		})
	}

	t.Run("updates reset time from headers", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(1, "least_loaded")
		key := pool.GetKeys()[0]

		resetTime := time.Now().Add(time.Minute)
		headers := http.Header{}
		headers.Set("anthropic-ratelimit-requests-reset", resetTime.Format(time.RFC3339))

		err := pool.UpdateKeyFromHeaders(key.ID, headers)
		require.NoError(t, err)

		assert.WithinDuration(t, resetTime, key.GetRPMResetAt(), time.Second)
	})

	t.Run("returns error for unknown keyID", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(1, "least_loaded")

		headers := http.Header{}
		err := pool.UpdateKeyFromHeaders("unknown-key-id", headers)

		assert.ErrorIs(t, err, keypool.ErrKeyNotFound)
	})
}

func TestMarkKeyExhausted(t *testing.T) {
	t.Parallel()
	t.Run("sets cooldown period", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(1, "least_loaded")
		key := pool.GetKeys()[0]

		retryAfter := 30 * time.Second
		pool.MarkKeyExhausted(key.ID, retryAfter)

		cooldown := key.GetCooldownUntil()

		assert.WithinDuration(t, time.Now().Add(retryAfter), cooldown, time.Second)
	})

	t.Run("key becomes unavailable during cooldown", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		firstKey := pool.GetKeys()[0]
		pool.MarkKeyExhausted(firstKey.ID, 10*time.Second)

		// Should skip exhausted key
		keyID, _, err := pool.GetKey(ctx)
		require.NoError(t, err)
		assert.NotEqual(t, firstKey.ID, keyID)
	})

	t.Run("key becomes available after cooldown expires", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(1, "least_loaded")
		ctx := context.Background()

		key := pool.GetKeys()[0]
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
	t.Parallel()
	t.Run("returns time to earliest reset", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(3, "least_loaded")

		// Set different reset times
		resetTimes := []time.Time{
			time.Now().Add(60 * time.Second),
			time.Now().Add(30 * time.Second), // Earliest
			time.Now().Add(90 * time.Second),
		}

		for idx, key := range pool.GetKeys() {
			headers := newTestHeaders(25, resetTimes[idx])
			err := pool.UpdateKeyFromHeaders(key.ID, headers)
			require.NoError(t, err)
		}

		duration := pool.GetEarliestResetTime()

		// Should be ~30 seconds (with some tolerance)
		assert.InDelta(t, 30.0, duration.Seconds(), 2.0)
	})

	t.Run("returns 60s default when no reset times set", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(2, "least_loaded")

		duration := pool.GetEarliestResetTime()

		assert.Equal(t, 60*time.Second, duration)
	})
}

func TestGetStats(t *testing.T) {
	t.Parallel()
	t.Run("returns correct counts", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(3, "least_loaded")

		stats := pool.GetStats()

		assert.Equal(t, 3, stats.TotalKeys)
		assert.Equal(t, 3, stats.AvailableKeys)
		assert.Equal(t, 0, stats.ExhaustedKeys)
		assert.Equal(t, 150, stats.TotalRPM) // 3 keys * 50 RPM
	})

	t.Run("updates after key state changes", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(3, "least_loaded")

		// Mark one key unhealthy
		pool.GetKeys()[0].MarkUnhealthy(fmt.Errorf("test error"))

		// Put one key in cooldown
		pool.MarkKeyExhausted(pool.GetKeys()[1].ID, 10*time.Second)

		stats := pool.GetStats()

		assert.Equal(t, 3, stats.TotalKeys)
		assert.Equal(t, 1, stats.AvailableKeys)
		assert.Equal(t, 2, stats.ExhaustedKeys)
	})

	t.Run("tracks remaining capacity", func(t *testing.T) {
		t.Parallel()
		pool := newTestPool(2, "least_loaded")

		// Update one key's remaining capacity
		headers := newTestHeaders(25, time.Now().Add(time.Minute))
		err := pool.UpdateKeyFromHeaders(pool.GetKeys()[0].ID, headers)
		require.NoError(t, err)

		stats := pool.GetStats()

		assert.Equal(t, 100, stats.TotalRPM)
		assert.Equal(t, 75, stats.RemainingRPM) // 25 + 50
	})
}

func TestConcurrencyGetKey(t *testing.T) {
	t.Parallel()
	pool := newTestPool(5, "least_loaded")
	ctx := context.Background()

	const numGoroutines = 50
	const requestsPerGoroutine = 10

	var waitGroup sync.WaitGroup
	waitGroup.Add(numGoroutines)

	for range numGoroutines {
		go func() {
			defer waitGroup.Done()
			for range requestsPerGoroutine {
				if _, _, getKeyErr := pool.GetKey(ctx); getKeyErr != nil {
					continue
				}
			}
		}()
	}

	waitGroup.Wait()
	// If we got here without data races, test passed
}

func TestConcurrencyUpdateHeaders(t *testing.T) {
	t.Parallel()
	pool := newTestPool(3, "least_loaded")

	const numGoroutines = 30

	var waitGroup sync.WaitGroup
	waitGroup.Add(numGoroutines)

	for idx := range numGoroutines {
		go func(iteration int) {
			defer waitGroup.Done()

			keyIdx := iteration % len(pool.GetKeys())
			key := pool.GetKeys()[keyIdx]

			headers := newTestHeaders(25, time.Now().Add(time.Minute))
			if updateErr := pool.UpdateKeyFromHeaders(key.ID, headers); updateErr != nil {
				return
			}
		}(idx)
	}

	waitGroup.Wait()
	// If we got here without data races, test passed
}

func TestConcurrencyNoRace(t *testing.T) {
	t.Parallel()
	pool := newTestPool(5, "least_loaded")
	ctx := context.Background()

	const numGoroutines = 20

	var waitGroup sync.WaitGroup
	waitGroup.Add(numGoroutines * 2)

	// Half reading, half writing
	for idx := range numGoroutines {
		// Reader goroutine
		go func() {
			defer waitGroup.Done()
			for range 10 {
				if _, _, getErr := pool.GetKey(ctx); getErr != nil {
					continue
				}
				pool.GetStats()
				pool.Keys()
			}
		}()

		// Writer goroutine
		go func(iteration int) {
			defer waitGroup.Done()
			for range 10 {
				keyIdx := iteration % len(pool.GetKeys())
				key := pool.GetKeys()[keyIdx]

				headers := newTestHeaders(25, time.Now().Add(time.Minute))
				if updateErr := pool.UpdateKeyFromHeaders(key.ID, headers); updateErr != nil {
					continue
				}
				pool.MarkKeyExhausted(key.ID, 1*time.Millisecond)
			}
		}(idx)
	}

	waitGroup.Wait()
	// Run with -race flag to verify no data races
}

func TestConcurrencyFairDistribution(t *testing.T) {
	t.Parallel()
	pool := newTestPool(3, "round_robin")
	ctx := context.Background()

	keyUsage := make(map[string]int)
	var keyUsageMu sync.Mutex

	// Use fewer requests to avoid exhausting rate limiters
	// Each key has burst=50, so 3 keys * 50 = 150 capacity
	const numRequests = 120 // Stay under total capacity

	var waitGroup sync.WaitGroup
	waitGroup.Add(numRequests)

	for range numRequests {
		go func() {
			defer waitGroup.Done()
			keyID, _, err := pool.GetKey(ctx)
			if err == nil {
				keyUsageMu.Lock()
				keyUsage[keyID]++
				keyUsageMu.Unlock()
			}
		}()
	}

	waitGroup.Wait()

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
}

// Tests for mo.Result-based methods

func TestKeyPoolGetKeyResult(t *testing.T) {
	t.Parallel()

	t.Run("returns Ok with keypool.KeySelection on success", func(t *testing.T) {
		t.Parallel()

		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		result := pool.GetKeyResult(ctx)
		require.True(t, result.IsOk(), "Expected Ok, got Err: %v", result.Error())

		selection, err := result.Get()
		require.NoError(t, err, "Expected Get() to succeed")
		assert.NotEmpty(t, selection.KeyID, "KeyID should not be empty")
		assert.NotEmpty(t, selection.APIKey, "APIKey should not be empty")
		assert.Contains(t, selection.APIKey, "sk-test-key-", "APIKey should match test key format")
	})

	t.Run("returns Err when all keys exhausted", func(t *testing.T) {
		t.Parallel()

		// Create pool with 1 key and exhaust it
		pool := newTestPool(1, "least_loaded")
		ctx := context.Background()

		// Mark the only key as unavailable by putting it in cooldown
		for _, key := range pool.Keys() {
			key.SetCooldown(time.Now().Add(1 * time.Minute))
		}

		result := pool.GetKeyResult(ctx)
		require.True(t, result.IsError(), "Expected Err when all keys exhausted")

		err := result.Error()
		assert.ErrorIs(t, err, keypool.ErrAllKeysExhausted)
	})

	t.Run("supports Map transformation", func(t *testing.T) {
		t.Parallel()

		pool := newTestPool(2, "least_loaded")
		ctx := context.Background()

		// Use Map to add prefix to APIKey
		transformed := pool.GetKeyResult(ctx).Map(func(s keypool.KeySelection) (keypool.KeySelection, error) {
			s.APIKey = "transformed:" + s.APIKey
			return s, nil
		})

		require.True(t, transformed.IsOk(), "Expected Ok after Map")

		selection := transformed.MustGet()
		assert.True(t, len(selection.APIKey) > len("transformed:"), "APIKey should be transformed")
		assert.Contains(t, selection.APIKey, "transformed:", "APIKey should have prefix")
	})

	t.Run("supports OrElse default", func(t *testing.T) {
		t.Parallel()

		pool := newTestPool(1, "least_loaded")
		ctx := context.Background()

		// Mark key as unavailable
		for _, key := range pool.Keys() {
			key.SetCooldown(time.Now().Add(1 * time.Minute))
		}

		defaultSelection := keypool.KeySelection{KeyID: "default-id", APIKey: "default-key"}
		selection := pool.GetKeyResult(ctx).OrElse(defaultSelection)

		assert.Equal(t, "default-id", selection.KeyID)
		assert.Equal(t, "default-key", selection.APIKey)
	})
}

func TestKeyPoolUpdateKeyFromHeadersResult(t *testing.T) {
	t.Parallel()

	t.Run("returns Ok on success", func(t *testing.T) {
		t.Parallel()

		pool := newTestPool(1, "least_loaded")

		// Get the key ID first
		keyID, _, keyErr := pool.GetKey(context.Background())
		require.NoError(t, keyErr)

		headers := http.Header{}
		headers.Set("anthropic-ratelimit-requests-limit", "100")
		headers.Set("anthropic-ratelimit-requests-remaining", "50")

		result := pool.UpdateKeyFromHeadersResult(keyID, headers)
		require.True(t, result.IsOk(), "Expected Ok, got Err: %v", result.Error())
	})

	t.Run("returns Err for unknown key", func(t *testing.T) {
		t.Parallel()

		pool := newTestPool(1, "least_loaded")

		headers := http.Header{}
		result := pool.UpdateKeyFromHeadersResult("unknown-key-id", headers)

		require.True(t, result.IsError(), "Expected Err for unknown key")
		assert.ErrorIs(t, result.Error(), keypool.ErrKeyNotFound)
	})
}

func TestKeySelection(t *testing.T) {
	t.Parallel()

	t.Run("struct fields accessible", func(t *testing.T) {
		t.Parallel()

		testKeyValue := "test-value-for-unit-test"       // #nosec G101 -- test data, not a real credential
		testKeyIdentifier := "test-key-id-for-unit-test" // #nosec G101 -- test data
		selection := keypool.KeySelection{
			KeyID:  testKeyIdentifier,
			APIKey: testKeyValue,
		}

		assert.Equal(t, testKeyIdentifier, selection.KeyID)
		assert.Equal(t, testKeyValue, selection.APIKey)
	})
}
