package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNoopCache_Get_ReturnsNotFound(t *testing.T) {
	c := newNoopCache()
	defer c.Close()

	ctx := context.Background()
	_, err := c.Get(ctx, "any-key")

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestNoopCache_Set_ReturnsNil(t *testing.T) {
	c := newNoopCache()
	defer c.Close()

	ctx := context.Background()
	err := c.Set(ctx, "key", []byte("value"))

	if err != nil {
		t.Errorf("Set() error = %v, want nil", err)
	}

	// Verify the value was not actually stored (Get still returns ErrNotFound)
	_, err = c.Get(ctx, "key")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() after Set() error = %v, want ErrNotFound", err)
	}
}

func TestNoopCache_SetWithTTL_ReturnsNil(t *testing.T) {
	c := newNoopCache()
	defer c.Close()

	ctx := context.Background()
	err := c.SetWithTTL(ctx, "key", []byte("value"), 5*time.Minute)

	if err != nil {
		t.Errorf("SetWithTTL() error = %v, want nil", err)
	}

	// Verify the value was not actually stored
	_, err = c.Get(ctx, "key")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() after SetWithTTL() error = %v, want ErrNotFound", err)
	}
}

func TestNoopCache_Delete_ReturnsNil(t *testing.T) {
	c := newNoopCache()
	defer c.Close()

	ctx := context.Background()

	// Delete non-existent key should succeed
	err := c.Delete(ctx, "non-existent-key")
	if err != nil {
		t.Errorf("Delete() error = %v, want nil", err)
	}

	// Delete after Set should also succeed
	_ = c.Set(ctx, "key", []byte("value"))
	err = c.Delete(ctx, "key")
	if err != nil {
		t.Errorf("Delete() after Set() error = %v, want nil", err)
	}
}

func TestNoopCache_Exists_ReturnsFalse(t *testing.T) {
	c := newNoopCache()
	defer c.Close()

	ctx := context.Background()

	// Non-existent key returns false
	exists, err := c.Exists(ctx, "any-key")
	if err != nil {
		t.Errorf("Exists() error = %v, want nil", err)
	}
	if exists {
		t.Error("Exists() = true, want false")
	}

	// Even after Set, Exists returns false
	_ = c.Set(ctx, "key", []byte("value"))
	exists, err = c.Exists(ctx, "key")
	if err != nil {
		t.Errorf("Exists() after Set() error = %v, want nil", err)
	}
	if exists {
		t.Error("Exists() after Set() = true, want false")
	}
}

func TestNoopCache_Close_Idempotent(t *testing.T) {
	c := newNoopCache()

	// First close should succeed
	err := c.Close()
	if err != nil {
		t.Errorf("Close() first call error = %v, want nil", err)
	}

	// Second close should also succeed
	err = c.Close()
	if err != nil {
		t.Errorf("Close() second call error = %v, want nil", err)
	}

	// Third close should also succeed
	err = c.Close()
	if err != nil {
		t.Errorf("Close() third call error = %v, want nil", err)
	}
}

func TestNoopCache_OperationsAfterClose_ReturnErrClosed(t *testing.T) {
	c := newNoopCache()
	ctx := context.Background()

	// Close the cache
	err := c.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// All operations should return ErrClosed
	t.Run("Get", func(t *testing.T) {
		_, err := c.Get(ctx, "key")
		if !errors.Is(err, ErrClosed) {
			t.Errorf("Get() after Close() error = %v, want ErrClosed", err)
		}
	})

	t.Run("Set", func(t *testing.T) {
		err := c.Set(ctx, "key", []byte("value"))
		if !errors.Is(err, ErrClosed) {
			t.Errorf("Set() after Close() error = %v, want ErrClosed", err)
		}
	})

	t.Run("SetWithTTL", func(t *testing.T) {
		err := c.SetWithTTL(ctx, "key", []byte("value"), time.Minute)
		if !errors.Is(err, ErrClosed) {
			t.Errorf("SetWithTTL() after Close() error = %v, want ErrClosed", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := c.Delete(ctx, "key")
		if !errors.Is(err, ErrClosed) {
			t.Errorf("Delete() after Close() error = %v, want ErrClosed", err)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		_, err := c.Exists(ctx, "key")
		if !errors.Is(err, ErrClosed) {
			t.Errorf("Exists() after Close() error = %v, want ErrClosed", err)
		}
	})
}

func TestNoopCache_Stats_ReturnsZero(t *testing.T) {
	c := newNoopCache()
	defer c.Close()

	stats := c.Stats()

	if stats.Hits != 0 {
		t.Errorf("Stats().Hits = %d, want 0", stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("Stats().Misses = %d, want 0", stats.Misses)
	}
	if stats.KeyCount != 0 {
		t.Errorf("Stats().KeyCount = %d, want 0", stats.KeyCount)
	}
	if stats.BytesUsed != 0 {
		t.Errorf("Stats().BytesUsed = %d, want 0", stats.BytesUsed)
	}
	if stats.Evictions != 0 {
		t.Errorf("Stats().Evictions = %d, want 0", stats.Evictions)
	}

	// Stats should work after some operations too
	ctx := context.Background()
	_ = c.Set(ctx, "key", []byte("value"))
	_, _ = c.Get(ctx, "key")

	stats = c.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.KeyCount != 0 || stats.BytesUsed != 0 || stats.Evictions != 0 {
		t.Error("Stats() should always return zero values")
	}
}

func TestNoopCache_ConcurrentAccess(_ *testing.T) {
	c := newNoopCache()
	defer c.Close()

	ctx := context.Background()
	const goroutines = 100
	const operations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < operations; j++ {
				key := "key"

				// Mix of all operations
				switch j % 6 {
				case 0:
					_, _ = c.Get(ctx, key)
				case 1:
					_ = c.Set(ctx, key, []byte("value"))
				case 2:
					_ = c.SetWithTTL(ctx, key, []byte("value"), time.Minute)
				case 3:
					_ = c.Delete(ctx, key)
				case 4:
					_, _ = c.Exists(ctx, key)
				case 5:
					_ = c.Stats()
				}
			}
		}()
	}

	wg.Wait()
}

func TestNoopCache_ImplementsInterfaces(t *testing.T) {
	t.Helper()
	c := newNoopCache()
	defer c.Close()

	// Verify Cache interface
	var _ Cache = c

	// Verify StatsProvider interface
	var _ StatsProvider = c
}
