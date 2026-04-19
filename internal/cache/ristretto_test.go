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

	// Wait for TTL to expire, then poll to confirm value is gone
	deadline := time.Now().Add(ttl + 200*time.Millisecond)
	for time.Now().Before(deadline) {
		_, err = ristrettoCache.Get(ctx, key)
		if errors.Is(err, cache.ErrNotFound) {
			return // Success: value expired as expected
		}
		time.Sleep(10 * time.Millisecond)
	}
	// If we exit loop, value didn't expire in time
	_, err = ristrettoCache.Get(ctx, key)
	t.Fatalf("Get after %v TTL should return ErrNotFound, got %v", ttl, err)
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
	for i := range 10 {
		key := fmt.Sprintf("key-%d", i)
		value := []byte("value")
		err := ristrettoCache.Set(ctx, key, value)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}
	cache.RistrettoWait(ristrettoCache)

	// Get some values (some hits, some misses)
	for i := range 5 {
		key := fmt.Sprintf("key-%d", i)
		if _, err := ristrettoCache.Get(ctx, key); err != nil && !errors.Is(err, cache.ErrNotFound) {
			t.Errorf("Get(%q) unexpected error = %v", key, err)
		}
	}
	for i := range 3 {
		key := fmt.Sprintf("key-%d", 25-i)
		if _, err := ristrettoCache.Get(ctx, key); err != nil && !errors.Is(err, cache.ErrNotFound) {
			t.Errorf("Get(%q) unexpected error = %v", key, err)
		}
	}

	assertRistrettoStatsPopulated(t, ristrettoCache.Stats())
}

func assertRistrettoStatsPopulated(t *testing.T, stats cache.Stats) {
	t.Helper()
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

func runRistrettoReadOp(ctx context.Context, ristrettoCache *cache.RistrettoCacheT, key string) error {
	if _, err := ristrettoCache.Get(ctx, key); err != nil && !errors.Is(err, cache.ErrNotFound) {
		return fmt.Errorf("Get(%q): %w", key, err)
	}
	if _, err := ristrettoCache.Exists(ctx, key); err != nil {
		return fmt.Errorf("Exists(%q): %w", key, err)
	}
	return nil
}

func runRistrettoWriteOp(ctx context.Context, ristrettoCache *cache.RistrettoCacheT, key string, value []byte) error {
	if err := ristrettoCache.Set(ctx, key, value); err != nil {
		return fmt.Errorf("Set(%q): %w", key, err)
	}
	if err := ristrettoCache.SetWithTTL(ctx, key, value, time.Minute); err != nil {
		return fmt.Errorf("SetWithTTL(%q): %w", key, err)
	}
	if err := ristrettoCache.Delete(ctx, key); err != nil {
		return fmt.Errorf("Delete(%q): %w", key, err)
	}
	return nil
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

	for goroutineIdx := range numGoroutines {
		go func(goroutine int) {
			defer waitGroup.Done()
			for opIdx := range numOperations {
				key := fmt.Sprintf("key-%d", (goroutine+opIdx)%26)
				value := []byte("value")

				var err error
				if opIdx%2 == 0 {
					err = runRistrettoReadOp(ctx, ristrettoCache, key)
				} else {
					err = runRistrettoWriteOp(ctx, ristrettoCache, key, value)
				}
				if err != nil {
					t.Errorf("goroutine %d operation %d: %v", goroutine, opIdx, err)
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
	if err := ristrettoCache.Set(ctx, key, value); err != nil {
		b.Fatal(err)
	}
	cache.RistrettoWait(ristrettoCache)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := ristrettoCache.Get(ctx, key); err != nil {
				b.Fatalf("Get failed: %v", err)
			}
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
		idx := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", idx%26)
			if err := ristrettoCache.Set(ctx, key, value); err != nil {
				b.Fatalf("Set(%q) failed: %v", key, err)
			}
			idx++
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
	for i := range 5 {
		key := fmt.Sprintf("key-%d", i)
		if setErr := ristrettoCache.Set(ctx, key, []byte("value")); setErr != nil {
			t.Fatalf("Set(%q) error = %v", key, setErr)
		}
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
	for i := range 1000 {
		key := fmt.Sprintf("key-%d-%d", i%26, i%10)
		if setErr := ristrettoCache.Set(ctx, key, value); setErr != nil {
			b.Fatalf("Set(%q) error = %v", key, setErr)
		}
	}
	cache.RistrettoWait(ristrettoCache)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d-%d", idx%26, idx%10)
			benchmarkMixedOp(ctx, b, ristrettoCache, key, value, idx)
			idx++
		}
	})
}

func benchmarkMixedOp(
	ctx context.Context,
	b *testing.B,
	ristrettoCache *cache.RistrettoCacheT,
	key string,
	value []byte,
	idx int,
) {
	b.Helper()
	if idx%3 == 0 {
		if err := ristrettoCache.Set(ctx, key, value); err != nil {
			b.Fatalf("Set(%q) failed: %v", key, err)
		}
	} else {
		if _, err := ristrettoCache.Get(ctx, key); err != nil {
			b.Fatalf("Get(%q) failed: %v", key, err)
		}
	}
}

// Test for newRistrettoCacheWithLog path.
func TestNewRistrettoCacheWithLogger(t *testing.T) {
	t.Parallel()
	_, logPtr := cache.NewTestLogger(0)

	cfg := cache.SmallTestRistrettoConfig()
	ristrettoCache, err := cache.NewRistrettoCacheWithLogger(cfg, logPtr)
	if err != nil {
		t.Fatalf("NewRistrettoCacheWithLogger() error = %v, want nil", err)
	}
	defer func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	ctx := context.Background()
	err = ristrettoCache.Set(ctx, "test", []byte("value"))
	if err != nil {
		t.Errorf("Set() error = %v, want nil", err)
	}
}

// Test concurrent operations during close to hit early closed check.
func TestRistrettoCacheConcurrentClose(t *testing.T) {
	t.Parallel()
	cfg := cache.SmallTestRistrettoConfig()
	ristrettoCache, err := cache.NewRistrettoCacheForTest(cfg)
	if err != nil {
		t.Fatalf("NewRistrettoCacheForTest() error = %v", err)
	}

	ctx := context.Background()

	// Pre-populate cache
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		if setErr := ristrettoCache.Set(ctx, key, []byte("value")); setErr != nil {
			t.Fatalf("Set(%q) error = %v", key, setErr)
		}
	}
	cache.RistrettoWait(ristrettoCache)

	// Start many goroutines doing operations while we close
	const numGoroutines = 50
	var waitGroup sync.WaitGroup
	waitGroup.Add(numGoroutines + 1)

	// Some goroutines doing reads
	for range numGoroutines / 2 {
		go func() {
			defer waitGroup.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d", j%10)
				// Errors expected during concurrent close; intentionally ignored.
				_, getErr := ristrettoCache.Get(ctx, key)
				_ = getErr
			}
		}()
	}

	// Some goroutines doing writes
	for range numGoroutines / 2 {
		go func() {
			defer waitGroup.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d", j%10)
				// Errors expected during concurrent close; intentionally ignored.
				setErr := ristrettoCache.Set(ctx, key, []byte("value"))
				_ = setErr
			}
		}()
	}

	// Close the cache while operations are in flight
	go func() {
		defer waitGroup.Done()
		// Small delay to let some operations start
		time.Sleep(5 * time.Millisecond)
		// Error intentionally ignored during concurrent close test.
		closeErr := ristrettoCache.Close()
		_ = closeErr
	}()

	waitGroup.Wait()

	// After close, all operations should return ErrClosed
	_, err = ristrettoCache.Get(ctx, "key-0")
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Get after close returned %v, want ErrClosed", err)
	}

	err = ristrettoCache.Set(ctx, "key-new", []byte("value"))
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Set after close returned %v, want ErrClosed", err)
	}
}
