package cache_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/cache"
)

func newTestRistrettoCache(t *testing.T) *cache.RistrettoCacheT {
	t.Helper()
	cfg := cache.RistrettoConfig{
		NumCounters: 100_000,
		MaxCost:     10 << 20, // 10 MB
		BufferItems: 64,
	}
	ristrettoCache, err := cache.NewRistrettoCacheForTest(cfg)
	if err != nil {
		t.Fatalf("failed to create ristretto cache: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			t.Errorf("failed to close cache: %v", closeErr)
		}
	})
	return ristrettoCache
}

func TestRistrettoCacheGetSet(t *testing.T) {
	t.Parallel()
	ristrettoCache := newTestRistrettoCache(t)
	ctx := context.Background()

	// Test set and get
	key := "test-key"
	value := []byte("test-value")

	err := ristrettoCache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Wait for async set to complete
	cache.RistrettoWait(ristrettoCache)

	got, err := ristrettoCache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !bytes.Equal(got, value) {
		t.Errorf("Get returned %q, want %q", got, value)
	}

	// Test cache miss
	_, err = ristrettoCache.Get(ctx, "nonexistent-key")
	if !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("Get nonexistent key returned %v, want ErrNotFound", err)
	}
}

func TestRistrettoCacheSetWithTTLExpires(t *testing.T) {
	t.Parallel()
	ristrettoCache := newTestRistrettoCache(t)
	ctx := context.Background()

	key := "ttl-key"
	value := []byte("ttl-value")
	ttl := 100 * time.Millisecond

	err := ristrettoCache.SetWithTTL(ctx, key, value, ttl)
	if err != nil {
		t.Fatalf("SetWithTTL failed: %v", err)
	}

	// Wait for async set to complete
	cache.RistrettoWait(ristrettoCache)

	// Should exist immediately after set
	got, err := ristrettoCache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get immediately after SetWithTTL failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get returned %q, want %q", got, value)
	}

	// Wait for TTL to expire
	time.Sleep(ttl + 100*time.Millisecond)

	// Should not exist after TTL expires
	_, err = ristrettoCache.Get(ctx, key)
	if !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("Get after TTL expired returned %v, want ErrNotFound", err)
	}
}

func TestRistrettoCacheDelete(t *testing.T) {
	t.Parallel()
	ristrettoCache := newTestRistrettoCache(t)
	ctx := context.Background()

	key := "delete-key"
	value := []byte("delete-value")

	// Set a value
	err := ristrettoCache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	cache.RistrettoWait(ristrettoCache)

	// Verify it exists
	_, err = ristrettoCache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get after Set failed: %v", err)
	}

	// Delete it
	err = ristrettoCache.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should not exist after delete
	_, err = ristrettoCache.Get(ctx, key)
	if !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("Get after Delete returned %v, want ErrNotFound", err)
	}

	// Delete nonexistent key should succeed (idempotent)
	err = ristrettoCache.Delete(ctx, "nonexistent-key")
	if err != nil {
		t.Errorf("Delete nonexistent key failed: %v", err)
	}
}

func TestRistrettoCacheExists(t *testing.T) {
	t.Parallel()
	ristrettoCache := newTestRistrettoCache(t)
	ctx := context.Background()

	key := "exists-key"
	value := []byte("exists-value")

	// Should not exist before set
	exists, err := ristrettoCache.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Exists returned true for nonexistent key")
	}

	// Set a value
	err = ristrettoCache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	cache.RistrettoWait(ristrettoCache)

	// Should exist after set
	exists, err = ristrettoCache.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists returned false for existing key")
	}
}

func TestRistrettoCacheClose(t *testing.T) {
	t.Parallel()
	cfg := cache.RistrettoConfig{
		NumCounters: 100_000,
		MaxCost:     10 << 20,
		BufferItems: 64,
	}
	ristrettoCache, err := cache.NewRistrettoCacheForTest(cfg)
	if err != nil {
		t.Fatalf("failed to create ristretto cache: %v", err)
	}

	ctx := context.Background()

	// Set a value before close
	err = ristrettoCache.Set(ctx, "key", []byte("value"))
	if err != nil {
		t.Fatalf("Set before close failed: %v", err)
	}
	cache.RistrettoWait(ristrettoCache)

	// Close the cache
	err = ristrettoCache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// All operations should return ErrClosed after close
	_, err = ristrettoCache.Get(ctx, "key")
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Get after Close returned %v, want ErrClosed", err)
	}

	err = ristrettoCache.Set(ctx, "key", []byte("value"))
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Set after Close returned %v, want ErrClosed", err)
	}

	err = ristrettoCache.SetWithTTL(ctx, "key", []byte("value"), time.Minute)
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("SetWithTTL after Close returned %v, want ErrClosed", err)
	}

	err = ristrettoCache.Delete(ctx, "key")
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Delete after Close returned %v, want ErrClosed", err)
	}

	_, err = ristrettoCache.Exists(ctx, "key")
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Exists after Close returned %v, want ErrClosed", err)
	}

	// Close is idempotent
	err = ristrettoCache.Close()
	if err != nil {
		t.Errorf("Second Close returned %v, want nil", err)
	}
}

func TestRistrettoCacheStats(t *testing.T) {
	t.Parallel()
	ristrettoCache := newTestRistrettoCache(t)
	ctx := context.Background()

	// Set some values
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte("value")
		err := ristrettoCache.Set(ctx, key, value)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}
	cache.RistrettoWait(ristrettoCache)

	// Get some values (some hits, some misses)
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		_, _ = ristrettoCache.Get(ctx, key) //nolint:errcheck,gosec // testing cache hits
	}
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("key-%d", 25-i)
		_, _ = ristrettoCache.Get(ctx, key) //nolint:errcheck,gosec // testing cache misses
	}

	stats := ristrettoCache.Stats()

	// Verify stats are populated
	if stats.Hits == 0 {
		t.Error("Stats.Hits is 0, expected some hits")
	}
	if stats.Misses == 0 {
		t.Error("Stats.Misses is 0, expected some misses")
	}
	if stats.KeyCount == 0 {
		t.Error("Stats.KeyCount is 0, expected some keys")
	}
	if stats.BytesUsed == 0 {
		t.Error("Stats.BytesUsed is 0, expected some bytes")
	}
}

func TestRistrettoCacheContextCancellation(t *testing.T) {
	// Ristretto-specific context cancellation test
	t.Parallel()
	ristrettoCache := newTestRistrettoCache(t)

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// All operations should return context error
	_, err := ristrettoCache.Get(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Get with canceled context returned %v, want context.Canceled", err)
	}

	err = ristrettoCache.Set(ctx, "key", []byte("value"))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Set with canceled context returned %v, want context.Canceled", err)
	}

	err = ristrettoCache.SetWithTTL(ctx, "key", []byte("value"), time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("SetWithTTL with canceled context returned %v, want context.Canceled", err)
	}

	err = ristrettoCache.Delete(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Delete with canceled context returned %v, want context.Canceled", err)
	}

	_, err = ristrettoCache.Exists(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Exists with canceled context returned %v, want context.Canceled", err)
	}
}

func TestRistrettoCacheConcurrentAccess(t *testing.T) {
	// Ristretto-specific concurrent access test
	t.Parallel()
	ristrettoCache := newTestRistrettoCache(t)
	ctx := context.Background()

	const (
		numGoroutines = 100
		numOperations = 100
	)

	var waitGroup sync.WaitGroup
	waitGroup.Add(numGoroutines)

	for goroutineIdx := 0; goroutineIdx < numGoroutines; goroutineIdx++ {
		go func(id int) {
			defer waitGroup.Done()
			for opIdx := 0; opIdx < numOperations; opIdx++ {
				key := fmt.Sprintf("key-%d", (id+opIdx)%26)
				value := []byte("value")

				// Mix of operations
				switch opIdx % 5 {
				case 0:
					_ = ristrettoCache.Set(ctx, key, value) //nolint:errcheck,gosec // concurrent test
				case 1:
					//nolint:errcheck,gosec // concurrent test
					_ = ristrettoCache.SetWithTTL(ctx, key, value, time.Minute)
				case 2:
					_, _ = ristrettoCache.Get(ctx, key) //nolint:errcheck,gosec // concurrent test
				case 3:
					_, _ = ristrettoCache.Exists(ctx, key) //nolint:errcheck,gosec // concurrent test
				case 4:
					_ = ristrettoCache.Delete(ctx, key) //nolint:errcheck,gosec // concurrent test
				}
			}
		}(goroutineIdx)
	}

	waitGroup.Wait()

	// If we get here without race detector complaints or panics, test passes
}

func TestRistrettoCacheValueIsolation(t *testing.T) {
	t.Parallel()
	ristrettoCache := newTestRistrettoCache(t)
	ctx := context.Background()

	key := "isolation-key"
	originalValue := []byte("original")

	// Set the value
	err := ristrettoCache.Set(ctx, key, originalValue)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	cache.RistrettoWait(ristrettoCache)

	// Modify the original slice
	originalValue[0] = 'X'

	// Get the value
	got, err := ristrettoCache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Cached value should not be affected by modification
	if got[0] == 'X' {
		t.Error("Cached value was mutated by modifying original slice")
	}

	// Modify the returned slice
	got[0] = 'Y'

	// Get again
	got2, err := ristrettoCache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Second Get failed: %v", err)
	}

	// Cached value should not be affected
	if got2[0] == 'Y' {
		t.Error("Cached value was mutated by modifying returned slice")
	}
}

func BenchmarkRistrettoCacheGet(b *testing.B) {
	cfg := cache.RistrettoConfig{
		NumCounters: 1_000_000,
		MaxCost:     100 << 20,
		BufferItems: 64,
	}
	ristrettoCache, err := cache.NewRistrettoCacheForTest(cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	defer func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			b.Errorf("failed to close cache: %v", closeErr)
		}
	}()

	ctx := context.Background()
	key := "benchmark-key"
	value := []byte("benchmark-value-with-some-reasonable-length")

	// Pre-populate the cache
	_ = ristrettoCache.Set(ctx, key, value) //nolint:errcheck,gosec // benchmark setup
	cache.RistrettoWait(ristrettoCache)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = ristrettoCache.Get(ctx, key) //nolint:errcheck,gosec // benchmark loop
		}
	})
}

func BenchmarkRistrettoCacheSet(b *testing.B) {
	cfg := cache.RistrettoConfig{
		NumCounters: 1_000_000,
		MaxCost:     100 << 20,
		BufferItems: 64,
	}
	ristrettoCache, err := cache.NewRistrettoCacheForTest(cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	defer func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			b.Errorf("failed to close cache: %v", closeErr)
		}
	}()

	ctx := context.Background()
	value := []byte("benchmark-value-with-some-reasonable-length")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%26)
			_ = ristrettoCache.Set(ctx, key, value) //nolint:errcheck,gosec // benchmark loop
			i++
		}
	})
}

func TestRistrettoCacheStatsAfterClose(t *testing.T) {
	t.Parallel()
	cfg := cache.RistrettoConfig{
		NumCounters: 100_000,
		MaxCost:     10 << 20,
		BufferItems: 64,
	}
	ristrettoCache, err := cache.NewRistrettoCacheForTest(cfg)
	if err != nil {
		t.Fatalf("failed to create ristretto cache: %v", err)
	}

	ctx := context.Background()

	// Set some values
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		_ = ristrettoCache.Set(ctx, key, []byte("value")) //nolint:errcheck,gosec // stats-after-close test setup
	}
	cache.RistrettoWait(ristrettoCache)

	// Close the cache
	err = ristrettoCache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Stats should return zero values after close (not panic)
	stats := ristrettoCache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.KeyCount != 0 || stats.BytesUsed != 0 {
		t.Logf("Stats after close: hits=%d, misses=%d, keys=%d, bytes=%d",
			stats.Hits, stats.Misses, stats.KeyCount, stats.BytesUsed)
	}
}

func TestNewRistrettoCacheDefaultBufferItems(t *testing.T) {
	t.Parallel()
	// Test that zero buffer_items uses default

	cfg := cache.RistrettoConfig{
		NumCounters: 100_000,
		MaxCost:     10 << 20,
		BufferItems: 0, // Should default to 64
	}

	ristrettoCache, err := cache.NewRistrettoCacheForTest(cfg)
	if err != nil {
		t.Fatalf("newRistrettoCache() error = %v, want nil", err)
	}
	defer func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			t.Errorf("failed to close cache: %v", closeErr)
		}
	}()

	// Verify cache works
	ctx := context.Background()
	err = ristrettoCache.Set(ctx, "test", []byte("value"))
	if err != nil {
		t.Errorf("Set() error = %v, want nil", err)
	}
}

func TestNewRistrettoCacheNegativeBufferItems(t *testing.T) {
	t.Parallel()
	// Test that negative buffer_items uses default

	cfg := cache.RistrettoConfig{
		NumCounters: 100_000,
		MaxCost:     10 << 20,
		BufferItems: -1, // Should default to 64
	}

	ristrettoCache, err := cache.NewRistrettoCacheForTest(cfg)
	if err != nil {
		t.Fatalf("newRistrettoCache() error = %v, want nil", err)
	}
	defer func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			t.Errorf("failed to close cache: %v", closeErr)
		}
	}()

	// Verify cache works
	ctx := context.Background()
	err = ristrettoCache.Set(ctx, "test", []byte("value"))
	if err != nil {
		t.Errorf("Set() error = %v, want nil", err)
	}
}

func BenchmarkRistrettoCacheMixed(b *testing.B) {
	cfg := cache.RistrettoConfig{
		NumCounters: 1_000_000,
		MaxCost:     100 << 20,
		BufferItems: 64,
	}
	ristrettoCache, err := cache.NewRistrettoCacheForTest(cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	defer func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			b.Errorf("failed to close cache: %v", closeErr)
		}
	}()

	ctx := context.Background()
	value := []byte("benchmark-value-with-some-reasonable-length")

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d-%d", i%26, i%10)
		_ = ristrettoCache.Set(ctx, key, value) //nolint:errcheck,gosec // benchmark setup
	}
	cache.RistrettoWait(ristrettoCache)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d-%d", idx%26, idx%10)
			if idx%3 == 0 {
				_ = ristrettoCache.Set(ctx, key, value) //nolint:errcheck,gosec // benchmark loop
			} else {
				_, _ = ristrettoCache.Get(ctx, key) //nolint:errcheck,gosec // benchmark loop
			}
			idx++
		}
	})
}
