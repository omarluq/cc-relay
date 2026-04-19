package cache_test

import (
	"context"
	"errors"
	"testing"

	"github.com/omarluq/cc-relay/internal/cache"
)

func TestNoopCacheGet(t *testing.T) {
	t.Parallel()

	noop := cache.NewNoopCacheForTest()
	_, err := noop.Get(context.Background(), "key")
	if !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("Get() error = %v, want %v", err, cache.ErrNotFound)
	}
}

func TestNoopCacheGetAfterClose(t *testing.T) {
	t.Parallel()

	noop := cache.NewNoopCacheForTest()
	closeErr := noop.Close()
	if closeErr != nil {
		t.Fatalf("Close() unexpected error: %v", closeErr)
	}

	_, err := noop.Get(context.Background(), "key")
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Get() after close error = %v, want %v", err, cache.ErrClosed)
	}
}

func TestNoopCacheSet(t *testing.T) {
	t.Parallel()

	noop := cache.NewNoopCacheForTest()
	err := noop.Set(context.Background(), "key", []byte("value"))
	if err != nil {
		t.Errorf("Set() unexpected error: %v", err)
	}
}

func TestNoopCacheSetAfterClose(t *testing.T) {
	t.Parallel()

	noop := cache.NewNoopCacheForTest()
	closeErr := noop.Close()
	if closeErr != nil {
		t.Errorf("Close() unexpected error: %v", closeErr)
	}

	err := noop.Set(context.Background(), "key", []byte("value"))
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Set() after close error = %v, want %v", err, cache.ErrClosed)
	}
}

func TestNoopCacheSetWithTTL(t *testing.T) {
	t.Parallel()

	noop := cache.NewNoopCacheForTest()
	err := noop.SetWithTTL(context.Background(), "key", []byte("value"), 0)
	if err != nil {
		t.Errorf("SetWithTTL() unexpected error: %v", err)
	}
}

func TestNoopCacheSetWithTTLAfterClose(t *testing.T) {
	t.Parallel()

	noop := cache.NewNoopCacheForTest()
	closeErr := noop.Close()
	if closeErr != nil {
		t.Errorf("Close() unexpected error: %v", closeErr)
	}

	err := noop.SetWithTTL(context.Background(), "key", []byte("value"), 0)
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("SetWithTTL() after close error = %v, want %v", err, cache.ErrClosed)
	}
}

func TestNoopCacheDelete(t *testing.T) {
	t.Parallel()

	noop := cache.NewNoopCacheForTest()
	err := noop.Delete(context.Background(), "key")
	if err != nil {
		t.Errorf("Delete() unexpected error: %v", err)
	}
}

func TestNoopCacheDeleteAfterClose(t *testing.T) {
	t.Parallel()

	noop := cache.NewNoopCacheForTest()
	closeErr := noop.Close()
	if closeErr != nil {
		t.Errorf("Close() unexpected error: %v", closeErr)
	}

	err := noop.Delete(context.Background(), "key")
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Delete() after close error = %v, want %v", err, cache.ErrClosed)
	}
}

func TestNoopCacheExists(t *testing.T) {
	t.Parallel()

	noop := cache.NewNoopCacheForTest()
	exists, err := noop.Exists(context.Background(), "key")
	if err != nil {
		t.Errorf("Exists() unexpected error: %v", err)
	}
	if exists {
		t.Error("Exists() = true, want false for noop cache")
	}
}

func TestNoopCacheExistsAfterClose(t *testing.T) {
	t.Parallel()

	noop := cache.NewNoopCacheForTest()
	closeErr := noop.Close()
	if closeErr != nil {
		t.Errorf("Close() unexpected error: %v", closeErr)
	}

	_, err := noop.Exists(context.Background(), "key")
	if !errors.Is(err, cache.ErrClosed) {
		t.Errorf("Exists() after close error = %v, want %v", err, cache.ErrClosed)
	}
}

func TestNoopCacheClose(t *testing.T) {
	t.Parallel()

	noop := cache.NewNoopCacheForTest()
	err := noop.Close()
	if err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}

	// Second close should be idempotent
	err = noop.Close()
	if err != nil {
		t.Errorf("second Close() unexpected error: %v", err)
	}
}

func TestNoopCacheStats(t *testing.T) {
	t.Parallel()

	noop := cache.NewNoopCacheForTest()
	stats := noop.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.KeyCount != 0 || stats.BytesUsed != 0 || stats.Evictions != 0 {
		t.Errorf("Stats() = %+v, want zero stats", stats)
	}
}

func TestNewNoopCacheWithLog(t *testing.T) {
	t.Parallel()

	_, logPtr := cache.NewTestLogger(0)
	noop := cache.NewNoopCacheWithLogger(logPtr)
	if noop == nil {
		t.Fatal("newNoopCacheWithLog() returned nil")
	}

	// Verify it works as a noop cache
	err := noop.Set(context.Background(), "key", []byte("value"))
	if err != nil {
		t.Errorf("Set() unexpected error: %v", err)
	}
	_, err = noop.Get(context.Background(), "key")
	if !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("Get() error = %v, want %v", err, cache.ErrNotFound)
	}
}
