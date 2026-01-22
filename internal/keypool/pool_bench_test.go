package keypool

import (
	"context"
	"net/http"
	"testing"
)

// createBenchPool creates a pool with n keys for benchmarking.
func createBenchPool(n int) *KeyPool {
	cfg := PoolConfig{
		Strategy: StrategyLeastLoaded,
		Keys:     make([]KeyConfig, n),
	}
	for i := range cfg.Keys {
		cfg.Keys[i] = KeyConfig{
			APIKey:    "sk-test-key-" + string(rune('A'+i%26)),
			RPMLimit:  50,
			ITPMLimit: 30000,
			OTPMLimit: 30000,
			Priority:  1,
			Weight:    1,
		}
	}
	pool, _ := NewKeyPool("bench-provider", cfg)
	return pool
}

// BenchmarkKeyPoolGetKey benchmarks key selection with various pool sizes.
func BenchmarkKeyPoolGetKey(b *testing.B) {
	sizes := []int{3, 10, 50, 100}

	for _, size := range sizes {
		pool := createBenchPool(size)
		ctx := context.Background()

		b.Run("size="+string(rune('0'+size/10))+string(rune('0'+size%10)), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, _ = pool.GetKey(ctx)
			}
		})
	}
}

// BenchmarkKeyPoolGetKeyParallel benchmarks concurrent key selection.
func BenchmarkKeyPoolGetKeyParallel(b *testing.B) {
	pool := createBenchPool(10)
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, _ = pool.GetKey(ctx)
		}
	})
}

// BenchmarkKeyPoolUpdateFromHeaders benchmarks header parsing.
func BenchmarkKeyPoolUpdateFromHeaders(b *testing.B) {
	pool := createBenchPool(10)

	// Get a key ID to update
	ctx := context.Background()
	keyID, _, _ := pool.GetKey(ctx)

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
	for i := 0; i < b.N; i++ {
		_ = pool.UpdateKeyFromHeaders(keyID, headers)
	}
}

// BenchmarkKeyPoolGetStats benchmarks stats aggregation.
func BenchmarkKeyPoolGetStats(b *testing.B) {
	sizes := []int{3, 10, 50, 100}

	for _, size := range sizes {
		pool := createBenchPool(size)

		b.Run("size="+string(rune('0'+size/10))+string(rune('0'+size%10)), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = pool.GetStats()
			}
		})
	}
}

// BenchmarkKeyPoolGetEarliestResetTime benchmarks reset time calculation.
func BenchmarkKeyPoolGetEarliestResetTime(b *testing.B) {
	sizes := []int{3, 10, 50, 100}

	for _, size := range sizes {
		pool := createBenchPool(size)

		b.Run("size="+string(rune('0'+size/10))+string(rune('0'+size%10)), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = pool.GetEarliestResetTime()
			}
		})
	}
}

// BenchmarkLeastLoadedSelector benchmarks the least-loaded selector directly.
func BenchmarkLeastLoadedSelector(b *testing.B) {
	sizes := []int{3, 10, 50, 100}

	for _, size := range sizes {
		selector := NewLeastLoadedSelector()
		keys := make([]*KeyMetadata, size)
		for i := range keys {
			keys[i] = NewKeyMetadata("sk-test-"+string(rune('A'+i%26)), 50, 30000, 30000)
		}

		b.Run("size="+string(rune('0'+size/10))+string(rune('0'+size%10)), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = selector.Select(keys)
			}
		})
	}
}

// BenchmarkRoundRobinSelector benchmarks the round-robin selector directly.
func BenchmarkRoundRobinSelector(b *testing.B) {
	sizes := []int{3, 10, 50, 100}

	for _, size := range sizes {
		selector := NewRoundRobinSelector()
		keys := make([]*KeyMetadata, size)
		for i := range keys {
			keys[i] = NewKeyMetadata("sk-test-"+string(rune('A'+i%26)), 50, 30000, 30000)
		}

		b.Run("size="+string(rune('0'+size/10))+string(rune('0'+size%10)), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = selector.Select(keys)
			}
		})
	}
}
