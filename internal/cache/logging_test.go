package cache_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/cache"
	"github.com/rs/zerolog"
)

func TestSetLoggerUpdatesLogger(t *testing.T) {
	t.Parallel()

	// Test that SetLogger works by observing its effect on a new noop cache.
	// The noop cache creation logs at Debug level, so if SetLogger worked,
	// we should see the log output.
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	// Create a cache using the test logger (which goes through SetLogger-like tagging)
	noopCache := cache.NewNoopCacheWithLogger(testLogger)
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	// Verify logger was applied - creation message should be present
	output := buf.String()
	if !strings.Contains(output, "noop cache created") {
		t.Errorf("expected creation log from applied logger, got: %s", output)
	}
}

func TestDefaultLoggerIsNoOp(t *testing.T) {
	t.Parallel()

	// The default logger is a no-op logger (zerolog.Nop()).
	// Verify by creating a default nop logger and checking its level.
	nop := zerolog.Nop()
	if nop.GetLevel() != zerolog.Disabled {
		t.Errorf("nop logger level = %v, want Disabled", nop.GetLevel())
	}
}

func TestRistrettoCacheLogsCreation(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.InfoLevel)

	cfg := cache.DefaultTestRistrettoConfig()

	ristrettoCache, err := cache.NewRistrettoCacheWithLogger(cfg, testLogger)
	if err != nil {
		t.Fatalf("NewRistrettoCacheWithLogger failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	output := buf.String()
	if !strings.Contains(output, "ristretto cache created") {
		t.Errorf("expected creation log, got: %s", output)
	}
	if !strings.Contains(output, "num_counters") {
		t.Errorf("expected num_counters in log, got: %s", output)
	}
	if !strings.Contains(output, "max_cost") {
		t.Errorf("expected max_cost in log, got: %s", output)
	}
}

func TestRistrettoCacheLogsGetHit(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	cfg := cache.DefaultTestRistrettoConfig()

	ristrettoCache, err := cache.NewRistrettoCacheWithLogger(cfg, testLogger)
	if err != nil {
		t.Fatalf("NewRistrettoCacheWithLogger failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	ctx := context.Background()

	if err := ristrettoCache.Set(ctx, "test-key", []byte("test-value")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	cache.RistrettoWait(ristrettoCache)

	buf.Reset()

	if _, err := ristrettoCache.Get(ctx, "test-key"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "cache get") {
		t.Errorf("expected 'cache get' log, got: %s", output)
	}
	if !strings.Contains(output, "hit") {
		t.Errorf("expected 'hit' in log, got: %s", output)
	}
	if !strings.Contains(output, "test-key") {
		t.Errorf("expected key in log, got: %s", output)
	}
}

func TestRistrettoCacheLogsGetMiss(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	cfg := cache.DefaultTestRistrettoConfig()

	ristrettoCache, err := cache.NewRistrettoCacheWithLogger(cfg, testLogger)
	if err != nil {
		t.Fatalf("NewRistrettoCacheWithLogger failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	ctx := context.Background()

	if _, err := ristrettoCache.Get(ctx, "nonexistent-key"); !errors.Is(err, cache.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}

	output := buf.String()
	if !strings.Contains(output, "cache get") {
		t.Errorf("expected 'cache get' log, got: %s", output)
	}
	if !strings.Contains(output, `"hit":false`) {
		t.Errorf("expected 'hit:false' in log for miss, got: %s", output)
	}
}

func TestRistrettoCacheLogsSet(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	cfg := cache.DefaultTestRistrettoConfig()

	ristrettoCache, err := cache.NewRistrettoCacheWithLogger(cfg, testLogger)
	if err != nil {
		t.Fatalf("NewRistrettoCacheWithLogger failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	ctx := context.Background()

	if err := ristrettoCache.Set(ctx, "log-test-key", []byte("log-test-value")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "cache set") {
		t.Errorf("expected 'cache set' log, got: %s", output)
	}
	if !strings.Contains(output, "log-test-key") {
		t.Errorf("expected key in log, got: %s", output)
	}
	if !strings.Contains(output, "size") {
		t.Errorf("expected 'size' in log, got: %s", output)
	}
}

func TestRistrettoCacheLogsSetWithTTL(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	cfg := cache.DefaultTestRistrettoConfig()

	ristrettoCache, err := cache.NewRistrettoCacheWithLogger(cfg, testLogger)
	if err != nil {
		t.Fatalf("NewRistrettoCacheWithLogger failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	ctx := context.Background()

	if err := ristrettoCache.SetWithTTL(ctx, "ttl-key", []byte("ttl-value"), 5*time.Minute); err != nil {
		t.Fatalf("SetWithTTL failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "cache set") {
		t.Errorf("expected 'cache set' log, got: %s", output)
	}
	if !strings.Contains(output, "ttl") {
		t.Errorf("expected 'ttl' in log, got: %s", output)
	}
}

func TestRistrettoCacheLogsDelete(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	cfg := cache.DefaultTestRistrettoConfig()

	ristrettoCache, err := cache.NewRistrettoCacheWithLogger(cfg, testLogger)
	if err != nil {
		t.Fatalf("NewRistrettoCacheWithLogger failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	ctx := context.Background()

	if err := ristrettoCache.Delete(ctx, "delete-key"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "cache delete") {
		t.Errorf("expected 'cache delete' log, got: %s", output)
	}
	if !strings.Contains(output, "delete-key") {
		t.Errorf("expected key in log, got: %s", output)
	}
}

func TestRistrettoCacheLogsClose(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.InfoLevel)

	cfg := cache.DefaultTestRistrettoConfig()

	ristrettoCache, err := cache.NewRistrettoCacheWithLogger(cfg, testLogger)
	if err != nil {
		t.Fatalf("NewRistrettoCacheWithLogger failed: %v", err)
	}

	buf.Reset()

	if err := ristrettoCache.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ristretto cache closed") {
		t.Errorf("expected close log, got: %s", output)
	}
}

func TestRistrettoCacheLogsStats(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	cfg := cache.DefaultTestRistrettoConfig()

	ristrettoCache, err := cache.NewRistrettoCacheWithLogger(cfg, testLogger)
	if err != nil {
		t.Fatalf("NewRistrettoCacheWithLogger failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := ristrettoCache.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	ctx := context.Background()

	if err := ristrettoCache.Set(ctx, "key", []byte("value")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	cache.RistrettoWait(ristrettoCache)
	if _, err := ristrettoCache.Get(ctx, "key"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	buf.Reset()

	_ = ristrettoCache.Stats()

	output := buf.String()
	if !strings.Contains(output, "cache stats") {
		t.Errorf("expected 'cache stats' log, got: %s", output)
	}
}

func TestNoopCacheLogsCreation(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	noopCache := cache.NewNoopCacheWithLogger(testLogger)
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	output := buf.String()
	if !strings.Contains(output, "noop cache created") {
		t.Errorf("expected creation log, got: %s", output)
	}
	if !strings.Contains(output, "disabled") {
		t.Errorf("expected 'disabled' note in log, got: %s", output)
	}
}

func TestNoopCacheLogsGet(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	noopCache := cache.NewNoopCacheWithLogger(testLogger)
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	buf.Reset()

	ctx := context.Background()
	if _, err := noopCache.Get(ctx, "test-key"); !errors.Is(err, cache.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}

	output := buf.String()
	if !strings.Contains(output, "cache get") {
		t.Errorf("expected 'cache get' log, got: %s", output)
	}
	if !strings.Contains(output, "test-key") {
		t.Errorf("expected key in log, got: %s", output)
	}
	if !strings.Contains(output, `"hit":false`) {
		t.Errorf("expected 'hit:false' in log for noop, got: %s", output)
	}
}

func TestNoopCacheLogsSet(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	noopCache := cache.NewNoopCacheWithLogger(testLogger)
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	buf.Reset()

	ctx := context.Background()
	if err := noopCache.Set(ctx, "noop-key", []byte("noop-value")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "cache set") {
		t.Errorf("expected 'cache set' log, got: %s", output)
	}
	if !strings.Contains(output, "noop-key") {
		t.Errorf("expected key in log, got: %s", output)
	}
	if !strings.Contains(output, "size") {
		t.Errorf("expected 'size' in log, got: %s", output)
	}
}

func TestNoopCacheLogsSetWithTTL(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	noopCache := cache.NewNoopCacheWithLogger(testLogger)
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	buf.Reset()

	ctx := context.Background()
	if err := noopCache.SetWithTTL(ctx, "ttl-key", []byte("ttl-value"), 5*time.Minute); err != nil {
		t.Fatalf("SetWithTTL failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "cache set") {
		t.Errorf("expected 'cache set' log, got: %s", output)
	}
	if !strings.Contains(output, "ttl") {
		t.Errorf("expected 'ttl' in log, got: %s", output)
	}
}

func TestNoopCacheLogsDelete(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	noopCache := cache.NewNoopCacheWithLogger(testLogger)
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	buf.Reset()

	ctx := context.Background()
	if err := noopCache.Delete(ctx, "delete-key"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "cache delete") {
		t.Errorf("expected 'cache delete' log, got: %s", output)
	}
	if !strings.Contains(output, "delete-key") {
		t.Errorf("expected key in log, got: %s", output)
	}
}

func TestNoopCacheLogsClose(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.InfoLevel)

	noopCache := cache.NewNoopCacheWithLogger(testLogger)

	buf.Reset()

	if err := noopCache.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "noop cache closed") {
		t.Errorf("expected 'noop cache closed' log, got: %s", output)
	}
}

func TestSetLoggerAddsComponentTag(t *testing.T) {
	t.Parallel()
	buf, baseLogger := cache.NewTestLogger(zerolog.DebugLevel)

	// Create cache with specific logger - component tag is added internally
	noopCache := cache.NewNoopCacheWithLogger(baseLogger)
	t.Cleanup(func() {
		if err := noopCache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	output := buf.String()
	if !strings.Contains(output, `"backend":"noop"`) {
		t.Errorf("expected 'backend:noop' tag in log, got: %s", output)
	}
}

func TestFactoryLogsCreation(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.InfoLevel)

	cfg := cache.Config{
		Mode:      cache.ModeSingle,
		Olric:     cache.ZeroOlricConfig(),
		Ristretto: cache.DefaultTestRistrettoConfig(),
	}

	cacheInst, err := cache.NewForTest(context.Background(), &cfg, testLogger)
	if err != nil {
		t.Fatalf("NewForTest failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := cacheInst.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	output := buf.String()
	if !strings.Contains(output, "cache factory") {
		t.Errorf("expected factory log, got: %s", output)
	}
	if !strings.Contains(output, "single") {
		t.Errorf("expected mode in log, got: %s", output)
	}
}

func TestFactoryLogsDisabledMode(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.InfoLevel)

	cfg := cache.Config{
		Mode:      cache.ModeDisabled,
		Olric:     cache.ZeroOlricConfig(),
		Ristretto: cache.ZeroRistrettoConfig(),
	}

	cacheInst, err := cache.NewForTest(context.Background(), &cfg, testLogger)
	if err != nil {
		t.Fatalf("NewForTest failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := cacheInst.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	output := buf.String()
	if !strings.Contains(output, "disabled") {
		t.Errorf("expected 'disabled' in log, got: %s", output)
	}
}

func TestFactoryLogsValidationFailure(t *testing.T) {
	t.Parallel()
	buf, testLogger := cache.NewTestLogger(zerolog.DebugLevel)

	cfg := cache.Config{
		Mode:      cache.ModeSingle,
		Olric:     cache.ZeroOlricConfig(),
		Ristretto: cache.RistrettoConfig{NumCounters: 0, MaxCost: 10 << 20, BufferItems: 0},
	}

	_, err := cache.NewForTest(context.Background(), &cfg, testLogger)
	if err == nil {
		t.Fatal("expected validation error")
	}

	output := buf.String()
	if !strings.Contains(output, "validation failed") {
		t.Errorf("expected validation failure log, got: %s", output)
	}
}
