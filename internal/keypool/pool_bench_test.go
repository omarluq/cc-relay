package keypool_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/omarluq/cc-relay/internal/keypool"
)

// createBenchPool creates a pool with n keys for benchmarking.
func createBenchPool(tb testing.TB, numKeys int) *keypool.KeyPool {
	tb.Helper()
	cfg := keypool.PoolConfig{
		Strategy: keypool.StrategyLeastLoaded,
		Keys:     make([]keypool.KeyConfig, numKeys),
	}
	for idx := range cfg.Keys {
		cfg.Keys[idx] = keypool.KeyConfig{
			APIKey:    "sk-test-key-" + string(rune('A'+idx%26)),
			RPMLimit:  50,
			ITPMLimit: 30000,
			OTPMLimit: 30000,
			Priority:  1,
			Weight:    1,
		}
	}
	pool, poolErr := keypool.NewKeyPool("bench-provider", cfg)
	if poolErr != nil {
		tb.Fatal(fmt.Errorf("createBenchPool: %w", poolErr))
	}
	return pool
}

// BenchmarkKeyPoolGetKey benchmarks key selection with various pool sizes.
func BenchmarkKeyPoolGetKey(b *testing.B) {
	sizes := []int{3, 10, 50, 100}

	for _, size := range sizes {
		pool := createBenchPool(b, size)
		ctx := context.Background()

		b.Run("size="+string(rune('0'+size/10))+string(rune('0'+size%10)), func(b *testing.B) {
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				if _, _, getKeyErr := pool.GetKey(ctx); getKeyErr != nil {
					continue
				}
			}
		})
	}
}

// BenchmarkKeyPoolGetKeyParallel benchmarks concurrent key selection.
func BenchmarkKeyPoolGetKeyParallel(b *testing.B) {
	pool := createBenchPool(b, 10)
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, _, parallelErr := pool.GetKey(ctx); parallelErr != nil {
				continue
			}
		}
	})
}

// BenchmarkKeyPoolUpdateFromHeaders benchmarks header parsing.
func BenchmarkKeyPoolUpdateFromHeaders(b *testing.B) {
	pool := createBenchPool(b, 10)

	// Get a key ID to update
	ctx := context.Background()
	keyID, _, keyErr := pool.GetKey(ctx)
	if keyErr != nil {
		b.Fatal(keyErr)
	}

	// Create headers with rate limit info
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-requests-limit", "50")
	headers.Set("anthropic-ratelimit-requests-remaining", "45")
	headers.Set("anthropic-ratelimit-requests-reset", "2026-01-22T22:00:00Z")
	headers.Set("anthropic-ratelimit-input-tokens-limit", "30000")
	headers.Set("anthropic-ratelimit-input-tokens-remaining", "28000")
	headers.Set("anthropic-ratelimit-output-tokens-limit", "30000")
	headers.Set("anthropic-ratelimit-output-tokens-remaining", "29000")

	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if updateErr := pool.UpdateKeyFromHeaders(keyID, headers); updateErr != nil {
			b.Fatal(updateErr)
		}
	}
}

// BenchmarkKeyPoolGetStats benchmarks stats aggregation.
func BenchmarkKeyPoolGetStats(b *testing.B) {
	sizes := []int{3, 10, 50, 100}

	for _, size := range sizes {
		pool := createBenchPool(b, size)

		b.Run("size="+string(rune('0'+size/10))+string(rune('0'+size%10)), func(b *testing.B) {
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				_ = pool.GetStats()
			}
		})
	}
}

// BenchmarkKeyPoolGetEarliestResetTime benchmarks reset time calculation.
func BenchmarkKeyPoolGetEarliestResetTime(b *testing.B) {
	sizes := []int{3, 10, 50, 100}

	for _, size := range sizes {
		pool := createBenchPool(b, size)

		b.Run("size="+string(rune('0'+size/10))+string(rune('0'+size%10)), func(b *testing.B) {
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				_ = pool.GetEarliestResetTime()
			}
		})
	}
}

// benchmarkSelector is a shared helper for benchmarking selector implementations.
func benchmarkSelector(b *testing.B, selector keypool.KeySelector) {
	b.Helper()
	sizes := []int{3, 10, 50, 100}

	for _, size := range sizes {
		keys := make([]*keypool.KeyMetadata, size)
		for idx := range keys {
			keys[idx] = keypool.NewKeyMetadata("sk-test-"+string(rune('A'+idx%26)), 50, 30000, 30000)
		}

		b.Run("size="+string(rune('0'+size/10))+string(rune('0'+size%10)), func(b *testing.B) {
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				if _, selectErr := selector.Select(keys); selectErr != nil {
					b.Fatal(selectErr)
				}
			}
		})
	}
}

// BenchmarkLeastLoadedSelector benchmarks the least-loaded selector directly.
func BenchmarkLeastLoadedSelector(b *testing.B) {
	benchmarkSelector(b, keypool.NewLeastLoadedSelector())
}

// BenchmarkRoundRobinSelector benchmarks the round-robin selector directly.
func BenchmarkRoundRobinSelector(b *testing.B) {
	benchmarkSelector(b, keypool.NewRoundRobinSelector())
}
