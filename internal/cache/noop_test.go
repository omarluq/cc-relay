package cache_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/cache"
)

func TestNoopCacheGetReturnsNotFound(t *testing.T) {
	t.Parallel()
	noopCache := cache.NewNoopCacheForTest()
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	ctx := context.Background()
	_, err := noopCache.Get(ctx, "any-key")

	if !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestNoopCacheSetReturnsNil(t *testing.T) {
	t.Parallel()
	noopCache := cache.NewNoopCacheForTest()
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	ctx := context.Background()
	if err := noopCache.Set(ctx, "key", []byte("value")); err != nil {
		t.Errorf("Set() error = %v, want nil", err)
	}

	// Verify the value was not actually stored (Get still returns ErrNotFound)
	_, err := noopCache.Get(ctx, "key")
	if !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("Get() after Set() error = %v, want ErrNotFound", err)
	}
}

func TestNoopCacheSetWithTTLReturnsNil(t *testing.T) {
	t.Parallel()
	noopCache := cache.NewNoopCacheForTest()
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	ctx := context.Background()
	if err := noopCache.SetWithTTL(ctx, "key", []byte("value"), 5*time.Minute); err != nil {
		t.Errorf("SetWithTTL() error = %v, want nil", err)
	}

	// Verify the value was not actually stored
	_, err := noopCache.Get(ctx, "key")
	if !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("Get() after SetWithTTL() error = %v, want ErrNotFound", err)
	}
}

func TestNoopCacheDeleteReturnsNil(t *testing.T) {
	t.Parallel()
	noopCache := cache.NewNoopCacheForTest()
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	ctx := context.Background()

	// Delete non-existent key should succeed
	if err := noopCache.Delete(ctx, "non-existent-key"); err != nil {
		t.Errorf("Delete() error = %v, want nil", err)
	}

	// Delete after Set should also succeed
	if err := noopCache.Set(ctx, "key", []byte("value")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if delErr := noopCache.Delete(ctx, "key"); delErr != nil {
		t.Errorf("Delete() after Set() error = %v, want nil", delErr)
	}
}

func TestNoopCacheExistsReturnsFalse(t *testing.T) {
	t.Parallel()
	noopCache := cache.NewNoopCacheForTest()
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	ctx := context.Background()

	// Non-existent key returns false
	exists, err := noopCache.Exists(ctx, "any-key")
	if err != nil {
		t.Errorf("Exists() error = %v, want nil", err)
	}
	if exists {
		t.Error("Exists() = true, want false")
	}

	// Even after Set, Exists returns false
	if setErr := noopCache.Set(ctx, "key", []byte("value")); setErr != nil {
		t.Fatalf("Set() error = %v", setErr)
	}
	exists, err = noopCache.Exists(ctx, "key")
	if err != nil {
		t.Errorf("Exists() after Set() error = %v, want nil", err)
	}
	if exists {
		t.Error("Exists() after Set() = true, want false")
	}
}

func TestNoopCacheCloseIdempotent(t *testing.T) {
	t.Parallel()
	noopCache := cache.NewNoopCacheForTest()

	// First close should succeed
	if err := noopCache.Close(); err != nil {
		t.Errorf("Close() first call error = %v, want nil", err)
	}

	// Second close should also succeed
	if err := noopCache.Close(); err != nil {
		t.Errorf("Close() second call error = %v, want nil", err)
	}

	// Third close should also succeed
	if err := noopCache.Close(); err != nil {
		t.Errorf("Close() third call error = %v, want nil", err)
	}
}

func TestNoopCacheOperationsAfterCloseReturnErrClosed(t *testing.T) {
	t.Parallel()
	noopCache := cache.NewNoopCacheForTest()
	ctx := context.Background()

	// Close the cache
	if err := noopCache.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// All operations should return ErrClosed
	t.Run("Get", func(t *testing.T) {
		t.Parallel()
		_, err := noopCache.Get(ctx, "key")
		if !errors.Is(err, cache.ErrClosed) {
			t.Errorf("Get() after Close() error = %v, want ErrClosed", err)
		}
	})

	t.Run("Set", func(t *testing.T) {
		t.Parallel()
		err := noopCache.Set(ctx, "key", []byte("value"))
		if !errors.Is(err, cache.ErrClosed) {
			t.Errorf("Set() after Close() error = %v, want ErrClosed", err)
		}
	})

	t.Run("SetWithTTL", func(t *testing.T) {
		t.Parallel()
		err := noopCache.SetWithTTL(ctx, "key", []byte("value"), time.Minute)
		if !errors.Is(err, cache.ErrClosed) {
			t.Errorf("SetWithTTL() after Close() error = %v, want ErrClosed", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		err := noopCache.Delete(ctx, "key")
		if !errors.Is(err, cache.ErrClosed) {
			t.Errorf("Delete() after Close() error = %v, want ErrClosed", err)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()
		_, err := noopCache.Exists(ctx, "key")
		if !errors.Is(err, cache.ErrClosed) {
			t.Errorf("Exists() after Close() error = %v, want ErrClosed", err)
		}
	})
}

func verifyNoopStatsZero(t *testing.T, stats cache.Stats, label string) {
	t.Helper()
	if stats.Hits != 0 {
		t.Errorf("%s: Stats().Hits = %d, want 0", label, stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("%s: Stats().Misses = %d, want 0", label, stats.Misses)
	}
	if stats.KeyCount != 0 {
		t.Errorf("%s: Stats().KeyCount = %d, want 0", label, stats.KeyCount)
	}
	if stats.BytesUsed != 0 {
		t.Errorf("%s: Stats().BytesUsed = %d, want 0", label, stats.BytesUsed)
	}
	if stats.Evictions != 0 {
		t.Errorf("%s: Stats().Evictions = %d, want 0", label, stats.Evictions)
	}
}

func TestNoopCacheStatsReturnsZero(t *testing.T) {
	t.Parallel()
	noopCache := cache.NewNoopCacheForTest()
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	verifyNoopStatsZero(t, noopCache.Stats(), "initial")

	// Stats should work after some operations too
	ctx := context.Background()
	if err := noopCache.Set(ctx, "key", []byte("value")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if _, err := noopCache.Get(ctx, "key"); !errors.Is(err, cache.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}

	verifyNoopStatsZero(t, noopCache.Stats(), "after operations")
}

//nolint:gocyclo,cyclop // test helper dispatches cache operations by index
func runNoopCacheOp(ctx context.Context, t *testing.T, noopCache *cache.NoopCacheT, opIdx int) {
	t.Helper()
	key := "key"

	switch opIdx % 6 {
	case 0:
		if _, err := noopCache.Get(ctx, key); err != nil && !errors.Is(err, cache.ErrNotFound) {
			t.Errorf("Get() unexpected error = %v", err)
		}
	case 1:
		if err := noopCache.Set(ctx, key, []byte("value")); err != nil {
			t.Errorf("Set() error = %v", err)
		}
	case 2:
		if err := noopCache.SetWithTTL(ctx, key, []byte("value"), time.Minute); err != nil {
			t.Errorf("SetWithTTL() error = %v", err)
		}
	case 3:
		if err := noopCache.Delete(ctx, key); err != nil {
			t.Errorf("Delete() error = %v", err)
		}
	case 4:
		if _, err := noopCache.Exists(ctx, key); err != nil {
			t.Errorf("Exists() error = %v", err)
		}
	case 5:
		_ = noopCache.Stats()
	}
}

func TestNoopCacheConcurrentAccess(t *testing.T) {
	t.Parallel()
	noopCache := cache.NewNoopCacheForTest()
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	ctx := context.Background()
	const goroutines = 100
	const operations = 100

	var waitGroup sync.WaitGroup
	waitGroup.Add(goroutines)

	for range goroutines {
		go func() {
			defer waitGroup.Done()
			for operationIdx := range operations {
				runNoopCacheOp(ctx, t, noopCache, operationIdx)
			}
		}()
	}

	waitGroup.Wait()
}

func TestNoopCacheImplementsInterfaces(t *testing.T) {
	t.Parallel()
	noopCache := cache.NewNoopCacheForTest()
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	// Verify Cache interface
	var _ cache.Cache = noopCache

	// Verify StatsProvider interface
	var _ cache.StatsProvider = noopCache
}
