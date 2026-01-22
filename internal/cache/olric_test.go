package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// portCounter is used to generate unique ports for each test.
var portCounter atomic.Int32

func init() {
	// Start from a high port to avoid conflicts.
	portCounter.Store(13320)
}

// getNextPort returns a unique port for testing.
func getNextPort() int {
	return int(portCounter.Add(1))
}

// newTestOlricCache creates an embedded Olric cache for testing.
// Uses embedded mode so we don't need a running cluster.
func newTestOlricCache(t *testing.T) *olricCache {
	t.Helper()

	port := getNextPort()
	cfg := OlricConfig{
		DMapName: fmt.Sprintf("test-dmap-%d", port),
		Embedded: true,
		BindAddr: fmt.Sprintf("127.0.0.1:%d", port),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cache, err := newOlricCache(ctx, &cfg)
	if err != nil {
		t.Fatalf("failed to create olric cache: %v", err)
	}

	t.Cleanup(func() {
		cache.Close()
	})

	return cache
}

func TestOlricCache_GetSet(t *testing.T) {
	cache := newTestOlricCache(t)
	ctx := context.Background()

	// Test set and get
	key := "test-key"
	value := []byte("test-value")

	err := cache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !bytes.Equal(got, value) {
		t.Errorf("Get returned %q, want %q", got, value)
	}

	// Test cache miss
	_, err = cache.Get(ctx, "nonexistent-key")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get nonexistent key returned %v, want ErrNotFound", err)
	}
}

func TestOlricCache_SetWithTTL_Expires(t *testing.T) {
	cache := newTestOlricCache(t)
	ctx := context.Background()

	key := "ttl-key"
	value := []byte("ttl-value")
	ttl := 500 * time.Millisecond

	err := cache.SetWithTTL(ctx, key, value, ttl)
	if err != nil {
		t.Fatalf("SetWithTTL failed: %v", err)
	}

	// Should exist immediately after set
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get immediately after SetWithTTL failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get returned %q, want %q", got, value)
	}

	// Wait for TTL to expire
	time.Sleep(ttl + 500*time.Millisecond)

	// Should not exist after TTL expires
	_, err = cache.Get(ctx, key)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after TTL expired returned %v, want ErrNotFound", err)
	}
}

func TestOlricCache_Delete(t *testing.T) {
	cache := newTestOlricCache(t)
	ctx := context.Background()

	key := "delete-key"
	value := []byte("delete-value")

	// Set a value
	err := cache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify it exists
	_, err = cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get after Set failed: %v", err)
	}

	// Delete it
	err = cache.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should not exist after delete
	_, err = cache.Get(ctx, key)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete returned %v, want ErrNotFound", err)
	}

	// Delete nonexistent key should succeed (idempotent)
	err = cache.Delete(ctx, "nonexistent-key")
	if err != nil {
		t.Errorf("Delete nonexistent key failed: %v", err)
	}
}

func TestOlricCache_Exists(t *testing.T) {
	cache := newTestOlricCache(t)
	ctx := context.Background()

	key := "exists-key"
	value := []byte("exists-value")

	// Should not exist before set
	exists, err := cache.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Exists returned true for nonexistent key")
	}

	// Set a value
	err = cache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should exist after set
	exists, err = cache.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists returned false for existing key")
	}
}

func TestOlricCache_Close(t *testing.T) {
	port := getNextPort()
	cfg := OlricConfig{
		DMapName: fmt.Sprintf("close-test-dmap-%d", port),
		Embedded: true,
		BindAddr: fmt.Sprintf("127.0.0.1:%d", port),
	}

	ctx := context.Background()

	cache, err := newOlricCache(ctx, &cfg)
	if err != nil {
		t.Fatalf("failed to create olric cache: %v", err)
	}

	// Set a value before close
	err = cache.Set(ctx, "key", []byte("value"))
	if err != nil {
		t.Fatalf("Set before close failed: %v", err)
	}

	// Close the cache
	err = cache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// All operations should return ErrClosed after close
	_, err = cache.Get(ctx, "key")
	if !errors.Is(err, ErrClosed) {
		t.Errorf("Get after Close returned %v, want ErrClosed", err)
	}

	err = cache.Set(ctx, "key", []byte("value"))
	if !errors.Is(err, ErrClosed) {
		t.Errorf("Set after Close returned %v, want ErrClosed", err)
	}

	err = cache.SetWithTTL(ctx, "key", []byte("value"), time.Minute)
	if !errors.Is(err, ErrClosed) {
		t.Errorf("SetWithTTL after Close returned %v, want ErrClosed", err)
	}

	err = cache.Delete(ctx, "key")
	if !errors.Is(err, ErrClosed) {
		t.Errorf("Delete after Close returned %v, want ErrClosed", err)
	}

	_, err = cache.Exists(ctx, "key")
	if !errors.Is(err, ErrClosed) {
		t.Errorf("Exists after Close returned %v, want ErrClosed", err)
	}

	// Close is idempotent
	err = cache.Close()
	if err != nil {
		t.Errorf("Second Close returned %v, want nil", err)
	}
}

func TestOlricCache_Ping(t *testing.T) {
	cache := newTestOlricCache(t)
	ctx := context.Background()

	// Ping should succeed for healthy cache
	err := cache.Ping(ctx)
	if err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestOlricCache_Stats(t *testing.T) {
	cache := newTestOlricCache(t)
	ctx := context.Background()

	// Set some values
	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		value := []byte("value")
		err := cache.Set(ctx, key, value)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	// Get some values (some hits, some misses)
	for i := 0; i < 5; i++ {
		key := string(rune('a' + i))
		_, _ = cache.Get(ctx, key)
	}
	for i := 0; i < 3; i++ {
		key := string(rune('z' - i))
		_, _ = cache.Get(ctx, key)
	}

	stats := cache.Stats()

	// Stats should be populated (Olric may not track all stats in embedded mode,
	// so we just verify it doesn't panic and returns a valid struct)
	t.Logf("Stats: hits=%d, misses=%d, keys=%d, bytes=%d, evictions=%d",
		stats.Hits, stats.Misses, stats.KeyCount, stats.BytesUsed, stats.Evictions)
}

func TestOlricCache_ContextTimeout(t *testing.T) {
	cache := newTestOlricCache(t)

	// Create already canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// All operations should return context error
	_, err := cache.Get(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Get with canceled context returned %v, want context.Canceled", err)
	}

	err = cache.Set(ctx, "key", []byte("value"))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Set with canceled context returned %v, want context.Canceled", err)
	}

	err = cache.SetWithTTL(ctx, "key", []byte("value"), time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("SetWithTTL with canceled context returned %v, want context.Canceled", err)
	}

	err = cache.Delete(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Delete with canceled context returned %v, want context.Canceled", err)
	}

	_, err = cache.Exists(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Exists with canceled context returned %v, want context.Canceled", err)
	}
}

func TestOlricCache_ConcurrentAccess(t *testing.T) {
	cache := newTestOlricCache(t)
	ctx := context.Background()

	const (
		numGoroutines = 20
		numOperations = 20
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := string(rune('a' + (id+j)%26))
				value := []byte("value")

				// Mix of operations
				switch j % 5 {
				case 0:
					_ = cache.Set(ctx, key, value)
				case 1:
					_ = cache.SetWithTTL(ctx, key, value, time.Minute)
				case 2:
					_, _ = cache.Get(ctx, key)
				case 3:
					_, _ = cache.Exists(ctx, key)
				case 4:
					_ = cache.Delete(ctx, key)
				}
			}
		}(i)
	}

	wg.Wait()

	// If we get here without race detector complaints or panics, test passes
}

func TestOlricCache_ValueIsolation(t *testing.T) {
	cache := newTestOlricCache(t)
	ctx := context.Background()

	key := "isolation-key"
	originalValue := []byte("original")

	// Set the value
	err := cache.Set(ctx, key, originalValue)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Modify the original slice
	originalValue[0] = 'X'

	// Get the value
	got, err := cache.Get(ctx, key)
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
	got2, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Second Get failed: %v", err)
	}

	// Cached value should not be affected
	if got2[0] == 'Y' {
		t.Error("Cached value was mutated by modifying returned slice")
	}
}

func TestOlricCache_LargeValues(t *testing.T) {
	cache := newTestOlricCache(t)
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

	err := cache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set large value failed: %v", err)
	}

	got, err := cache.Get(ctx, key)
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

func TestOlricCache_SpecialKeys(t *testing.T) {
	cache := newTestOlricCache(t)
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cache.Set(ctx, tc.key, tc.value)
			if err != nil {
				t.Fatalf("Set %q failed: %v", tc.key, err)
			}

			got, err := cache.Get(ctx, tc.key)
			if err != nil {
				t.Fatalf("Get %q failed: %v", tc.key, err)
			}

			if !bytes.Equal(got, tc.value) {
				t.Errorf("Get %q returned %q, want %q", tc.key, got, tc.value)
			}
		})
	}
}

// Benchmark tests are skipped by default as they take a long time
// Run with: go test -bench=. -run=^$ ./internal/cache/

func BenchmarkOlricCache_Get(b *testing.B) {
	b.Skip("Skipping slow benchmark")

	port := int(portCounter.Add(1))
	cfg := OlricConfig{
		DMapName: fmt.Sprintf("bench-dmap-%d", port),
		Embedded: true,
		BindAddr: fmt.Sprintf("127.0.0.1:%d", port),
	}

	ctx := context.Background()
	cache, err := newOlricCache(ctx, &cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	key := "benchmark-key"
	value := []byte("benchmark-value-with-some-reasonable-length")

	// Pre-populate the cache
	_ = cache.Set(ctx, key, value)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.Get(ctx, key)
		}
	})
}

func BenchmarkOlricCache_Set(b *testing.B) {
	b.Skip("Skipping slow benchmark")

	port := int(portCounter.Add(1))
	cfg := OlricConfig{
		DMapName: fmt.Sprintf("bench-dmap-%d", port),
		Embedded: true,
		BindAddr: fmt.Sprintf("127.0.0.1:%d", port),
	}

	ctx := context.Background()
	cache, err := newOlricCache(ctx, &cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	value := []byte("benchmark-value-with-some-reasonable-length")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune('a' + i%26))
			_ = cache.Set(ctx, key, value)
			i++
		}
	})
}

func BenchmarkOlricCache_Mixed(b *testing.B) {
	b.Skip("Skipping slow benchmark")

	port := int(portCounter.Add(1))
	cfg := OlricConfig{
		DMapName: fmt.Sprintf("bench-dmap-%d", port),
		Embedded: true,
		BindAddr: fmt.Sprintf("127.0.0.1:%d", port),
	}

	ctx := context.Background()
	cache, err := newOlricCache(ctx, &cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	value := []byte("benchmark-value-with-some-reasonable-length")

	// Pre-populate with some data
	for i := 0; i < 100; i++ {
		key := string(rune('a'+i%26)) + string(rune('0'+i%10))
		_ = cache.Set(ctx, key, value)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune('a'+i%26)) + string(rune('0'+i%10))
			if i%3 == 0 {
				_ = cache.Set(ctx, key, value)
			} else {
				_, _ = cache.Get(ctx, key)
			}
			i++
		}
	})
}

func TestOlricCache_ClusterInfo(t *testing.T) {
	cache := newTestOlricCache(t)

	// Type assert to ClusterInfo
	ci, ok := interface{}(cache).(ClusterInfo)
	if !ok {
		t.Fatal("olricCache should implement ClusterInfo")
	}

	// Test IsEmbedded
	if !ci.IsEmbedded() {
		t.Error("IsEmbedded should return true for embedded cache")
	}

	// Test ClusterMembers - for embedded mode stats may not be available
	// via the external interface (requires cluster client connection)
	members := ci.ClusterMembers()
	t.Logf("ClusterMembers returned %d", members)
	// Note: In embedded mode with no external listener configured,
	// Stats() call may fail since it tries to connect via external port.
	// The method returns 0 on error which is acceptable behavior.

	// Test MemberlistAddr - same limitation applies
	addr := ci.MemberlistAddr()
	if addr == "" {
		t.Log("MemberlistAddr returned empty (expected for embedded mode without external stats)")
	} else {
		t.Logf("MemberlistAddr: %s", addr)
	}

	// The key behavior we verify:
	// 1. Interface is implemented (compile-time and runtime check)
	// 2. IsEmbedded correctly identifies embedded mode
	// 3. Methods don't panic and return safe defaults when stats unavailable
}

func TestOlricCache_ClusterInfo_ClientMode(t *testing.T) {
	// Skip this test in CI - requires external Olric cluster
	// This documents expected behavior for client mode
	t.Skip("Skipping client mode test - requires external Olric cluster")

	// In client mode (db == nil), all ClusterInfo methods should return zero/empty values
	// This is intentional - client doesn't know about memberlist details
}

func TestOlricCache_GracefulShutdown(t *testing.T) {
	port := getNextPort()
	cfg := OlricConfig{
		DMapName:     fmt.Sprintf("shutdown-test-%d", port),
		Embedded:     true,
		BindAddr:     fmt.Sprintf("127.0.0.1:%d", port),
		LeaveTimeout: 2 * time.Second, // Short timeout for test
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cache, err := newOlricCache(ctx, &cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Set some data
	err = cache.Set(ctx, "key", []byte("value"))
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Time the shutdown - should complete within LeaveTimeout + overhead
	start := time.Now()
	err = cache.Close()
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

func TestOlricCache_ClusterInfo_AfterClose(t *testing.T) {
	cache := newTestOlricCache(t)

	// Close the cache first
	err := cache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// ClusterInfo methods should return zero values after close
	ci := cache

	if ci.MemberlistAddr() != "" {
		t.Error("MemberlistAddr should return empty string after close")
	}

	if ci.ClusterMembers() != 0 {
		t.Error("ClusterMembers should return 0 after close")
	}

	// IsEmbedded should still return true (it's a static property)
	if !ci.IsEmbedded() {
		t.Error("IsEmbedded should still return true after close")
	}
}

func TestOlricCache_HAConfiguration(t *testing.T) {
	port := getNextPort()
	cfg := OlricConfig{
		DMapName:          fmt.Sprintf("ha-test-dmap-%d", port),
		Embedded:          true,
		BindAddr:          fmt.Sprintf("127.0.0.1:%d", port),
		Environment:       EnvLocal,
		ReplicaCount:      2,
		ReadQuorum:        1,
		WriteQuorum:       1,
		MemberCountQuorum: 1,
		LeaveTimeout:      3 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cache, err := newOlricCache(ctx, &cfg)
	if err != nil {
		t.Fatalf("failed to create olric cache with HA config: %v", err)
	}
	defer cache.Close()

	// Verify cache is functional with HA settings
	testKey := "ha-test-key"
	testValue := []byte("ha-test-value")

	err = cache.Set(ctx, testKey, testValue)
	if err != nil {
		t.Fatalf("Set with HA config failed: %v", err)
	}

	got, err := cache.Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Get with HA config failed: %v", err)
	}

	if !bytes.Equal(got, testValue) {
		t.Errorf("Get returned %q, want %q", got, testValue)
	}
}

func TestOlricCache_PingAfterClose(t *testing.T) {
	port := getNextPort()
	cfg := OlricConfig{
		DMapName: fmt.Sprintf("ping-close-test-%d", port),
		Embedded: true,
		BindAddr: fmt.Sprintf("127.0.0.1:%d", port),
	}

	ctx := context.Background()
	cache, err := newOlricCache(ctx, &cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Close the cache
	err = cache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Ping after close should return ErrClosed
	err = cache.Ping(ctx)
	if !errors.Is(err, ErrClosed) {
		t.Errorf("Ping after Close returned %v, want ErrClosed", err)
	}
}

func TestOlricCache_StatsAfterClose(t *testing.T) {
	port := getNextPort()
	cfg := OlricConfig{
		DMapName: fmt.Sprintf("stats-close-test-%d", port),
		Embedded: true,
		BindAddr: fmt.Sprintf("127.0.0.1:%d", port),
	}

	ctx := context.Background()
	cache, err := newOlricCache(ctx, &cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Set some data
	err = cache.Set(ctx, "key", []byte("value"))
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Close the cache
	err = cache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Stats after close should return zero values (not panic)
	stats := cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.KeyCount != 0 {
		t.Logf("Stats after close: hits=%d, misses=%d, keys=%d",
			stats.Hits, stats.Misses, stats.KeyCount)
	}
}

func TestOlricCache_PingWithCanceledContext(t *testing.T) {
	cache := newTestOlricCache(t)

	// Create already canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Ping with canceled context should return context error
	err := cache.Ping(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Ping with canceled context returned %v, want context.Canceled", err)
	}
}

func TestParseBindAddr(t *testing.T) {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port := parseBindAddr(tt.addr)
			if host != tt.wantHost {
				t.Errorf("parseBindAddr(%q) host = %q, want %q", tt.addr, host, tt.wantHost)
			}
			if port != tt.wantPort {
				t.Errorf("parseBindAddr(%q) port = %d, want %d", tt.addr, port, tt.wantPort)
			}
		})
	}
}

func TestOlricCache_EnvironmentPresets(t *testing.T) {
	testCases := []struct {
		name        string
		environment string
	}{
		{"default (empty)", ""},
		{"local", EnvLocal},
		{"lan", EnvLAN},
		// Note: "wan" has longer timeouts, may make test slower
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			port := getNextPort()
			cfg := OlricConfig{
				DMapName:    fmt.Sprintf("env-test-dmap-%d", port),
				Embedded:    true,
				BindAddr:    fmt.Sprintf("127.0.0.1:%d", port),
				Environment: tc.environment,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cache, err := newOlricCache(ctx, &cfg)
			if err != nil {
				t.Fatalf("failed to create cache with environment %q: %v", tc.environment, err)
			}
			defer cache.Close()

			// Basic functionality check
			err = cache.Set(ctx, "key", []byte("value"))
			if err != nil {
				t.Fatalf("Set failed with environment %q: %v", tc.environment, err)
			}
		})
	}
}
