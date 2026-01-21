package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func newTestRistrettoCache(t *testing.T) *ristrettoCache {
	t.Helper()
	cfg := RistrettoConfig{
		NumCounters: 100_000,
		MaxCost:     10 << 20, // 10 MB
		BufferItems: 64,
	}
	cache, err := newRistrettoCache(cfg)
	if err != nil {
		t.Fatalf("failed to create ristretto cache: %v", err)
	}
	t.Cleanup(func() {
		cache.Close()
	})
	return cache
}

func TestRistrettoCache_GetSet(t *testing.T) {
	cache := newTestRistrettoCache(t)
	ctx := context.Background()

	// Test set and get
	key := "test-key"
	value := []byte("test-value")

	err := cache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Wait for async set to complete
	cache.cache.Wait()

	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(got) != string(value) {
		t.Errorf("Get returned %q, want %q", got, value)
	}

	// Test cache miss
	_, err = cache.Get(ctx, "nonexistent-key")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get nonexistent key returned %v, want ErrNotFound", err)
	}
}

func TestRistrettoCache_SetWithTTL_Expires(t *testing.T) {
	cache := newTestRistrettoCache(t)
	ctx := context.Background()

	key := "ttl-key"
	value := []byte("ttl-value")
	ttl := 100 * time.Millisecond

	err := cache.SetWithTTL(ctx, key, value, ttl)
	if err != nil {
		t.Fatalf("SetWithTTL failed: %v", err)
	}

	// Wait for async set to complete
	cache.cache.Wait()

	// Should exist immediately after set
	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get immediately after SetWithTTL failed: %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("Get returned %q, want %q", got, value)
	}

	// Wait for TTL to expire
	time.Sleep(ttl + 100*time.Millisecond)

	// Should not exist after TTL expires
	_, err = cache.Get(ctx, key)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after TTL expired returned %v, want ErrNotFound", err)
	}
}

func TestRistrettoCache_Delete(t *testing.T) {
	cache := newTestRistrettoCache(t)
	ctx := context.Background()

	key := "delete-key"
	value := []byte("delete-value")

	// Set a value
	err := cache.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	cache.cache.Wait()

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

func TestRistrettoCache_Exists(t *testing.T) {
	cache := newTestRistrettoCache(t)
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
	cache.cache.Wait()

	// Should exist after set
	exists, err = cache.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists returned false for existing key")
	}
}

func TestRistrettoCache_Close(t *testing.T) {
	cfg := RistrettoConfig{
		NumCounters: 100_000,
		MaxCost:     10 << 20,
		BufferItems: 64,
	}
	cache, err := newRistrettoCache(cfg)
	if err != nil {
		t.Fatalf("failed to create ristretto cache: %v", err)
	}

	ctx := context.Background()

	// Set a value before close
	err = cache.Set(ctx, "key", []byte("value"))
	if err != nil {
		t.Fatalf("Set before close failed: %v", err)
	}
	cache.cache.Wait()

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

func TestRistrettoCache_Stats(t *testing.T) {
	cache := newTestRistrettoCache(t)
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
	cache.cache.Wait()

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

func TestRistrettoCache_ContextCancellation(t *testing.T) {
	cache := newTestRistrettoCache(t)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// All operations should return context error
	_, err := cache.Get(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Get with cancelled context returned %v, want context.Canceled", err)
	}

	err = cache.Set(ctx, "key", []byte("value"))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Set with cancelled context returned %v, want context.Canceled", err)
	}

	err = cache.SetWithTTL(ctx, "key", []byte("value"), time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("SetWithTTL with cancelled context returned %v, want context.Canceled", err)
	}

	err = cache.Delete(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Delete with cancelled context returned %v, want context.Canceled", err)
	}

	_, err = cache.Exists(ctx, "key")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Exists with cancelled context returned %v, want context.Canceled", err)
	}
}

func TestRistrettoCache_ConcurrentAccess(t *testing.T) {
	cache := newTestRistrettoCache(t)
	ctx := context.Background()

	const (
		numGoroutines = 100
		numOperations = 100
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

func TestRistrettoCache_ValueIsolation(t *testing.T) {
	cache := newTestRistrettoCache(t)
	ctx := context.Background()

	key := "isolation-key"
	originalValue := []byte("original")

	// Set the value
	err := cache.Set(ctx, key, originalValue)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	cache.cache.Wait()

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

func BenchmarkRistrettoCache_Get(b *testing.B) {
	cfg := RistrettoConfig{
		NumCounters: 1_000_000,
		MaxCost:     100 << 20,
		BufferItems: 64,
	}
	cache, err := newRistrettoCache(cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	key := "benchmark-key"
	value := []byte("benchmark-value-with-some-reasonable-length")

	// Pre-populate the cache
	_ = cache.Set(ctx, key, value)
	cache.cache.Wait()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.Get(ctx, key)
		}
	})
}

func BenchmarkRistrettoCache_Set(b *testing.B) {
	cfg := RistrettoConfig{
		NumCounters: 1_000_000,
		MaxCost:     100 << 20,
		BufferItems: 64,
	}
	cache, err := newRistrettoCache(cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
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

func BenchmarkRistrettoCache_Mixed(b *testing.B) {
	cfg := RistrettoConfig{
		NumCounters: 1_000_000,
		MaxCost:     100 << 20,
		BufferItems: 64,
	}
	cache, err := newRistrettoCache(cfg)
	if err != nil {
		b.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	value := []byte("benchmark-value-with-some-reasonable-length")

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		key := string(rune('a'+i%26)) + string(rune('0'+i%10))
		_ = cache.Set(ctx, key, value)
	}
	cache.cache.Wait()

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
