//go:build integration
// +build integration

package cache_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/cache"
)

// portCounter is used to generate unique ports for each test.
// Each Olric node also uses port+2 for memberlist, so we increment by 10
// to leave headroom and avoid port collisions.
var portCounter atomic.Int32

func init() {
	// Start from a higher port to avoid conflicts with cluster tests
	// that start from 14320 (see testutil.go).
	portCounter.Store(15000)
}

// getNextPort returns a unique port for testing.
// Increments by 10 to leave room for memberlist port (port+2).
func getNextPort() int {
	return int(portCounter.Add(10))
}

// newTestOlricCache creates an embedded Olric cache for testing.
// Uses embedded mode so we don't need a running cluster.
func newTestOlricCache(t *testing.T) *cache.OlricCacheT {
	t.Helper()

	port := getNextPort()
	cfg := cache.DefaultTestOlricConfig(
		fmt.Sprintf("test-dmap-%d", port),
		fmt.Sprintf("127.0.0.1:%d", port),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testCache, err := cache.NewOlricCacheForTest(ctx, &cfg)
	if err != nil {
		t.Fatalf("failed to create olric cache: %v", err)
	}

	t.Cleanup(func() {
		if closeErr := testCache.Close(); closeErr != nil {
			t.Errorf("failed to close cache: %v", closeErr)
		}
	})

	return testCache
}

func TestOlricCacheGetSet(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)
	ctx := context.Background()

	// Test set and get
	key := "test-key"
	value := []byte("test-value")

	err := testCache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := testCache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !bytes.Equal(got, value) {
		t.Errorf("Get returned %q, want %q", got, value)
	}

	// Test cache miss
	_, err = testCache.Get(ctx, "nonexistent-key")
	if !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("Get nonexistent key returned %v, want ErrNotFound", err)
	}
}

func TestOlricCacheSetWithTTLExpires(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)
	ctx := context.Background()

	key := "ttl-key"
	value := []byte("ttl-value")
	ttl := 500 * time.Millisecond

	err := testCache.SetWithTTL(ctx, key, value, ttl)
	if err != nil {
		t.Fatalf("SetWithTTL failed: %v", err)
	}

	// Should exist immediately after set
	got, err := testCache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get immediately after SetWithTTL failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get returned %q, want %q", got, value)
	}

	// Wait for TTL to expire
	time.Sleep(ttl + 500*time.Millisecond)

	// Should not exist after TTL expires
	_, err = testCache.Get(ctx, key)
	if !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("Get after TTL expired returned %v, want ErrNotFound", err)
	}
}

func TestOlricCacheDelete(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)
	ctx := context.Background()

	key := "delete-key"
	value := []byte("delete-value")

	// Set a value
	err := testCache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify it exists
	_, err = testCache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get after Set failed: %v", err)
	}

	// Delete it
	err = testCache.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should not exist after delete
	_, err = testCache.Get(ctx, key)
	if !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("Get after Delete returned %v, want ErrNotFound", err)
	}

	// Delete nonexistent key should succeed (idempotent)
	err = testCache.Delete(ctx, "nonexistent-key")
	if err != nil {
		t.Errorf("Delete nonexistent key failed: %v", err)
	}
}

func TestOlricCacheExists(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)
	ctx := context.Background()

	key := "exists-key"
	value := []byte("exists-value")

	// Should not exist before set
	exists, err := testCache.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Exists returned true for nonexistent key")
	}

	// Set a value
	err = testCache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should exist after set
	exists, err = testCache.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists returned false for existing key")
	}
}

func TestOlricCacheClose(t *testing.T) {
	t.Parallel()
	port := getNextPort()
	cfg := cache.DefaultTestOlricConfig(
		fmt.Sprintf("close-test-dmap-%d", port),
		fmt.Sprintf("127.0.0.1:%d", port),
	)

	ctx := context.Background()

	testCache, err := cache.NewOlricCacheForTest(ctx, &cfg)
	if err != nil {
		t.Fatalf("failed to create olric cache: %v", err)
	}

	// Set a value before close
	err = testCache.Set(ctx, "key", []byte("value"))
	if err != nil {
		t.Fatalf("Set before close failed: %v", err)
	}

	// Close the cache
	err = testCache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// All operations should return ErrClosed after close
	_, err = testCache.Get(ctx, "key")
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Get after Close returned %v, want ErrClosed", err)
	}

	err = testCache.Set(ctx, "key", []byte("value"))
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Set after Close returned %v, want ErrClosed", err)
	}

	err = testCache.SetWithTTL(ctx, "key", []byte("value"), time.Minute)
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("SetWithTTL after Close returned %v, want ErrClosed", err)
	}

	err = testCache.Delete(ctx, "key")
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Delete after Close returned %v, want ErrClosed", err)
	}

	_, err = testCache.Exists(ctx, "key")
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Exists after Close returned %v, want ErrClosed", err)
	}

	// Close is idempotent
	err = testCache.Close()
	if err != nil {
		t.Errorf("Second Close returned %v, want nil", err)
	}
}

func TestOlricCachePing(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)
	ctx := context.Background()

	// Ping should succeed for healthy cache
	err := testCache.Ping(ctx)
	if err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestOlricCacheStats(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)
	ctx := context.Background()

	// Set some values
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte("value")
		if err := testCache.Set(ctx, key, value); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	// Get some values (some hits, some misses)
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		if _, err := testCache.Get(ctx, key); err != nil {
			t.Logf("Get hit failed for key %s: %v", key, err)
		}
	}
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("miss-key-%d", i)
		if _, err := testCache.Get(ctx, key); err != nil && !errors.Is(err, cache.ErrNotFound) {
			t.Logf("Get miss returned unexpected error for key %s: %v", key, err)
		}
	}

	stats := testCache.Stats()

	// Stats should be populated (Olric may not track all stats in embedded mode,
	// so we just verify it doesn't panic and returns a valid struct)
	t.Logf("Stats: hits=%d, misses=%d, keys=%d, bytes=%d, evictions=%d",
		stats.Hits, stats.Misses, stats.KeyCount, stats.BytesUsed, stats.Evictions)
}

// TestOlricCacheContextTimeout tests Olric-specific context cancellation behavior.
// Unlike Ristretto which is purely in-memory, Olric checks context before each
// distributed operation, making context cancellation observable at the cache API level.
// This test validates that Olric properly propagates context cancellation through
// its distributed operations layer.
func TestOlricCacheContextTimeout(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)

	// Create already canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// All operations should return context error
	_, err := testCache.Get(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Get with canceled context returned %v, want context.Canceled", err)
	}

	err = testCache.Set(ctx, "key", []byte("value"))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Set with canceled context returned %v, want context.Canceled", err)
	}

	err = testCache.SetWithTTL(ctx, "key", []byte("value"), time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("SetWithTTL with canceled context returned %v, want context.Canceled", err)
	}

	err = testCache.Delete(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Delete with canceled context returned %v, want context.Canceled", err)
	}

	_, err = testCache.Exists(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Exists with canceled context returned %v, want context.Canceled", err)
	}

	// Olric-specific: also verify that concurrent operations with canceled
	// context fail quickly without blocking on distributed coordination.
	// This is important for graceful shutdown scenarios.
	var waitGroup sync.WaitGroup
	for goroutineIdx := 0; goroutineIdx < 5; goroutineIdx++ {
		waitGroup.Add(1)
		go func(n int) {
			defer waitGroup.Done()
			key := fmt.Sprintf("concurrent-key-%d", n)
			_, getErr := testCache.Get(ctx, key)
			cache.IgnoreCacheErr(getErr)
		}(goroutineIdx)
	}
	waitGroup.Wait()
}

// TestOlricCacheConcurrentAccess tests Olric-specific concurrent access patterns.
// Unlike Ristretto which is purely in-memory, Olric has distributed synchronization
// that may exhibit different behavior under concurrent load. This test uses a higher
// goroutine count to stress the distributed coordination path.
func TestOlricCacheConcurrentAccess(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)
	ctx := context.Background()

	const (
		numGoroutines = 20
		numOperations = 20
	)

	var waitGroup sync.WaitGroup
	waitGroup.Add(numGoroutines)

	for goroutineIdx := 0; goroutineIdx < numGoroutines; goroutineIdx++ {
		go func(id int) {
			defer waitGroup.Done()
			for opIdx := 0; opIdx < numOperations; opIdx++ {
				key := fmt.Sprintf("key-%d", (id+opIdx)%26)
				value := []byte("value")

				// Mix of operations - errors intentionally ignored via
				// ignoreCacheErr for concurrent stress testing where
				// transient failures are expected and acceptable.
				switch opIdx % 5 {
				case 0:
					cache.IgnoreCacheErr(testCache.Set(ctx, key, value))
				case 1:
					cache.IgnoreCacheErr(testCache.SetWithTTL(ctx, key, value, time.Minute))
				case 2:
					_, getErr := testCache.Get(ctx, key)
					cache.IgnoreCacheErr(getErr)
				case 3:
					_, existsErr := testCache.Exists(ctx, key)
					cache.IgnoreCacheErr(existsErr)
				case 4:
					cache.IgnoreCacheErr(testCache.Delete(ctx, key))
				}
			}
		}(goroutineIdx)
	}

	waitGroup.Wait()

	// If we get here without race detector complaints or panics, test passes
}

func TestOlricCacheValueIsolation(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)
	ctx := context.Background()

	key := "isolation-key"
	originalValue := []byte("original")

	// Set the value
	err := testCache.Set(ctx, key, originalValue)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Modify the original slice
	originalValue[0] = 'X'

	// Get the value
	got, err := testCache.Get(ctx, key)
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
	got2, err := testCache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Second Get failed: %v", err)
	}

	// Cached value should not be affected
	if got2[0] == 'Y' {
		t.Error("Cached value was mutated by modifying returned slice")
	}
}

func TestOlricCacheLargeValues(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)
	ctx := context.Background()

	// Test with a moderately large value (64KB)
	// Note: Olric's default table size limits very large values.
	// For values larger than the table size, Olric returns ErrEntryTooLarge.
	// In production, configure DMaps.MaxInuse in olric config for larger values.
	key := "large-key"
	value := make([]byte, 64*1024) // 64KB
	for i := range value {
		value[i] = byte(i % 256)
	}

	err := testCache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set large value failed: %v", err)
	}

	got, err := testCache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get large value failed: %v", err)
	}

	if len(got) != len(value) {
		t.Errorf("Large value length mismatch: got %d, want %d", len(got), len(value))
	}

	// Verify content
	for i := 0; i < len(value); i++ {
		if got[i] != value[i] {
			t.Errorf("Large value content mismatch at position %d: got %d, want %d", i, got[i], value[i])
			break
		}
	}
}

func TestOlricCacheSpecialKeys(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)
	ctx := context.Background()

	testCases := []struct {
		name  string
		key   string
		value []byte
	}{
		{"empty value", "key-empty", []byte{}},
		{"unicode key", "key-unicode-\u4e2d\u6587", []byte("unicode")},
		{"spaces in key", "key with spaces", []byte("spaces")},
		{"special chars", "key:with/special-chars_123", []byte("special")},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := testCache.Set(ctx, testCase.key, testCase.value)
			if err != nil {
				t.Fatalf("Set %q failed: %v", testCase.key, err)
			}

			got, err := testCache.Get(ctx, testCase.key)
			if err != nil {
				t.Fatalf("Get %q failed: %v", testCase.key, err)
			}

			if !bytes.Equal(got, testCase.value) {
				t.Errorf("Get %q returned %q, want %q", testCase.key, got, testCase.value)
			}
		})
	}
}

// Benchmark tests are skipped by default as they take a long time
// Run with: go test -bench=. -run=^$ ./internal/cache/

func BenchmarkOlricCacheGet(b *testing.B) {
	b.Skip("Skipping slow benchmark")

	port := int(portCounter.Add(10))
	cfg := cache.DefaultTestOlricConfig(
		fmt.Sprintf("bench-dmap-%d", port),
		fmt.Sprintf("127.0.0.1:%d", port),
	)

	ctx := context.Background()
	benchCache, err := cache.NewOlricCacheForTest(ctx, &cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	b.Cleanup(func() {
		if closeErr := benchCache.Close(); closeErr != nil {
			b.Errorf("failed to close cache: %v", closeErr)
		}
	})

	key := "benchmark-key"
	value := []byte("benchmark-value-with-some-reasonable-length")

	// Pre-populate the cache
	if err := benchCache.Set(ctx, key, value); err != nil {
		b.Fatalf("pre-populate Set failed: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, getErr := benchCache.Get(ctx, key)
			cache.IgnoreCacheErr(getErr)
		}
	})
}

func BenchmarkOlricCacheSet(b *testing.B) {
	b.Skip("Skipping slow benchmark")

	port := int(portCounter.Add(10))
	cfg := cache.DefaultTestOlricConfig(
		fmt.Sprintf("bench-dmap-%d", port),
		fmt.Sprintf("127.0.0.1:%d", port),
	)

	ctx := context.Background()
	benchCache, err := cache.NewOlricCacheForTest(ctx, &cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	b.Cleanup(func() {
		if closeErr := benchCache.Close(); closeErr != nil {
			b.Errorf("failed to close cache: %v", closeErr)
		}
	})

	value := []byte("benchmark-value-with-some-reasonable-length")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%26)
			cache.IgnoreCacheErr(benchCache.Set(ctx, key, value))
			i++
		}
	})
}

func BenchmarkOlricCacheMixed(b *testing.B) {
	b.Skip("Skipping slow benchmark")

	port := int(portCounter.Add(10))
	cfg := cache.DefaultTestOlricConfig(
		fmt.Sprintf("bench-dmap-%d", port),
		fmt.Sprintf("127.0.0.1:%d", port),
	)

	ctx := context.Background()
	benchCache, err := cache.NewOlricCacheForTest(ctx, &cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	b.Cleanup(func() {
		if closeErr := benchCache.Close(); closeErr != nil {
			b.Errorf("failed to close cache: %v", closeErr)
		}
	})

	value := []byte("benchmark-value-with-some-reasonable-length")

	// Pre-populate with some data
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d-%d", i%26, i%10)
		if err := benchCache.Set(ctx, key, value); err != nil {
			b.Fatalf("pre-populate Set failed: %v", err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d-%d", idx%26, idx%10)
			if idx%3 == 0 {
				cache.IgnoreCacheErr(benchCache.Set(ctx, key, value))
			} else {
				_, getErr := benchCache.Get(ctx, key)
				cache.IgnoreCacheErr(getErr)
			}
			idx++
		}
	})
}

func TestOlricCacheClusterInfo(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)

	// Type assert to cache.ClusterInfo
	var clusterInfo interface{} = testCache
	_, ok := clusterInfo.(cache.ClusterInfo)
	if !ok {
		t.Fatal("olricCache should implement cache.ClusterInfo")
	}

	// Note: In external test package, we verify the interface is implemented.
	// Direct ClusterInfo method testing is done in internal tests.
}

func TestOlricCacheClusterInfoClientMode(t *testing.T) {
	t.Parallel()

	// Skip this test in CI - requires external Olric cluster
	// This documents expected behavior for client mode
	t.Skip("Skipping client mode test - requires external Olric cluster")

	// In client mode (db == nil), all ClusterInfo methods should return zero/empty values
	// This is intentional - client doesn't know about memberlist details
}

func TestOlricCacheGracefulShutdown(t *testing.T) {
	t.Parallel()
	port := getNextPort()
	cfg := cache.DefaultTestOlricConfig(
		fmt.Sprintf("shutdown-test-%d", port),
		fmt.Sprintf("127.0.0.1:%d", port),
	)
	cfg.LeaveTimeout = 2 * time.Second // Short timeout for test

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testCache, err := cache.NewOlricCacheForTest(ctx, &cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Set some data
	if setErr := testCache.Set(ctx, "key", []byte("value")); setErr != nil {
		t.Fatalf("Set failed: %v", setErr)
	}

	// Time the shutdown - should complete within LeaveTimeout + overhead
	start := time.Now()
	err = testCache.Close()
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Shutdown should complete reasonably quickly for single node
	// (no leave broadcast needed when no other members)
	if duration > 5*time.Second {
		t.Errorf("Close took %v, expected < 5s for single node", duration)
	}

	t.Logf("Graceful shutdown completed in %v", duration)
}

func TestOlricCacheClusterInfoAfterClose(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)

	// Close the cache first
	err := testCache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Note: Can't directly call ClusterInfo methods from external test package.
	// The internal implementation handles this case.
}

func TestOlricCacheHAConfiguration(t *testing.T) {
	t.Parallel()
	port := getNextPort()
	cfg := cache.DefaultTestOlricConfig(
		fmt.Sprintf("ha-test-dmap-%d", port),
		fmt.Sprintf("127.0.0.1:%d", port),
	)
	// Override with HA-specific settings
	cfg.Environment = cache.EnvLocal
	cfg.ReplicaCount = 2
	cfg.ReadQuorum = 1
	cfg.WriteQuorum = 1
	cfg.LeaveTimeout = 3 * time.Second
	cfg.MemberCountQuorum = 1

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testCache, err := cache.NewOlricCacheForTest(ctx, &cfg)
	if err != nil {
		t.Fatalf("failed to create olric cache with HA config: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := testCache.Close(); closeErr != nil {
			t.Errorf("failed to close cache: %v", closeErr)
		}
	})

	// Verify cache is functional with HA settings
	testKey := "ha-test-key"
	testValue := []byte("ha-test-value")

	err = testCache.Set(ctx, testKey, testValue)
	if err != nil {
		t.Fatalf("Set with HA config failed: %v", err)
	}

	got, err := testCache.Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Get with HA config failed: %v", err)
	}

	if !bytes.Equal(got, testValue) {
		t.Errorf("Get returned %q, want %q", got, testValue)
	}
}

func TestOlricCachePingAfterClose(t *testing.T) {
	t.Parallel()
	port := getNextPort()
	cfg := cache.DefaultTestOlricConfig(
		fmt.Sprintf("ping-close-test-%d", port),
		fmt.Sprintf("127.0.0.1:%d", port),
	)

	ctx := context.Background()
	testCache, err := cache.NewOlricCacheForTest(ctx, &cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Close the cache
	err = testCache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Ping after close should return ErrClosed
	err = testCache.Ping(ctx)
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Ping after Close returned %v, want ErrClosed", err)
	}
}

func TestOlricCacheStatsAfterClose(t *testing.T) {
	t.Parallel()
	port := getNextPort()
	cfg := cache.DefaultTestOlricConfig(
		fmt.Sprintf("stats-close-test-%d", port),
		fmt.Sprintf("127.0.0.1:%d", port),
	)

	ctx := context.Background()
	testCache, err := cache.NewOlricCacheForTest(ctx, &cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Set some data
	err = testCache.Set(ctx, "key", []byte("value"))
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Close the cache
	err = testCache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Stats after close should return zero values (not panic)
	stats := testCache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.KeyCount != 0 {
		t.Logf("Stats after close: hits=%d, misses=%d, keys=%d",
			stats.Hits, stats.Misses, stats.KeyCount)
	}
}

func TestOlricCachePingWithCanceledContext(t *testing.T) {
	t.Parallel()
	testCache := newTestOlricCache(t)

	// Create already canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Ping with canceled context should return context error
	err := testCache.Ping(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Ping with canceled context returned %v, want context.Canceled", err)
	}
}

func TestParseBindAddr(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		addr     string
		wantHost string
		wantPort int
	}{
		{"host:port", "127.0.0.1:3320", "127.0.0.1", 3320},
		{"host only", "127.0.0.1", "127.0.0.1", 0},
		{"ipv6 with port", "[::1]:3320", "::1", 3320},
		{"ipv6 without port", "::1", "::1", 0},
		{"hostname with port", "localhost:3320", "localhost", 3320},
		{"hostname only", "localhost", "localhost", 0},
		{"invalid port", "127.0.0.1:invalid", "127.0.0.1", 0},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			host, port := cache.ParseBindAddrForTest(testCase.addr)
			if host != testCase.wantHost {
				t.Errorf("parseBindAddr(%q) host = %q, want %q", testCase.addr, host, testCase.wantHost)
			}
			if port != testCase.wantPort {
				t.Errorf("parseBindAddr(%q) port = %d, want %d", testCase.addr, port, testCase.wantPort)
			}
		})
	}
}

func TestOlricCacheEnvironmentPresets(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		environment string
	}{
		{"default (empty)", ""},
		{"local", cache.EnvLocal},
		{"lan", cache.EnvLAN},
		// Note: "wan" has longer timeouts, may make test slower
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			port := getNextPort()
			cfg := cache.DefaultTestOlricConfig(
				fmt.Sprintf("env-test-dmap-%d", port),
				fmt.Sprintf("127.0.0.1:%d", port),
			)
			cfg.Environment = testCase.environment

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			envCache, err := cache.NewOlricCacheForTest(ctx, &cfg)
			if err != nil {
				t.Fatalf("failed to create cache with environment %q: %v", testCase.environment, err)
			}
			t.Cleanup(func() {
				if closeErr := envCache.Close(); closeErr != nil {
					t.Errorf("failed to close cache: %v", closeErr)
				}
			})

			// Basic functionality check
			if setErr := envCache.Set(ctx, "key", []byte("value")); setErr != nil {
				t.Fatalf("Set failed with environment %q: %v", testCase.environment, setErr)
			}
		})
	}
}
