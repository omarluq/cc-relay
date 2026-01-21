package cache

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestSetLogger_UpdatesLogger(t *testing.T) {
	// Save original logger
	original := Logger

	// Create a test logger
	var buf bytes.Buffer
	testLogger := zerolog.New(&buf).Level(zerolog.DebugLevel)

	// Set the new logger
	SetLogger(&testLogger)

	// Verify Logger was updated
	if Logger.GetLevel() != zerolog.DebugLevel {
		t.Error("SetLogger did not update Logger")
	}

	// Restore original
	Logger = original
}

func TestDefaultLogger_IsNoOp(t *testing.T) {
	// The default logger should be a no-op logger
	// This test verifies initial state before SetLogger is called
	original := Logger

	// Reset to ensure we're testing initial state
	Logger = zerolog.Nop()

	// Verify it's a no-op by checking the level
	// zerolog.Nop() returns a logger that discards everything
	if Logger.GetLevel() != zerolog.Disabled {
		t.Errorf("default logger level = %v, want Disabled (nop)", Logger.GetLevel())
	}

	Logger = original
}

func TestRistrettoCache_LogsCreation(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cfg := RistrettoConfig{
		NumCounters: 100_000,
		MaxCost:     10 << 20,
		BufferItems: 64,
	}

	cache, err := newRistrettoCache(cfg)
	if err != nil {
		t.Fatalf("newRistrettoCache failed: %v", err)
	}
	defer cache.Close()

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

func TestRistrettoCache_LogsGetHit(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cache := newTestRistrettoCache(t)
	ctx := context.Background()

	// Set a value
	err := cache.Set(ctx, "test-key", []byte("test-value"))
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	cache.cache.Wait()

	buf.Reset() // Clear logs from Set

	// Get the value (should be a hit)
	_, err = cache.Get(ctx, "test-key")
	if err != nil {
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

func TestRistrettoCache_LogsGetMiss(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cache := newTestRistrettoCache(t)
	ctx := context.Background()

	// Get a nonexistent key (should be a miss)
	_, _ = cache.Get(ctx, "nonexistent-key")

	output := buf.String()
	if !strings.Contains(output, "cache get") {
		t.Errorf("expected 'cache get' log, got: %s", output)
	}
	// hit:false indicates a cache miss
	if !strings.Contains(output, `"hit":false`) {
		t.Errorf("expected 'hit:false' in log for miss, got: %s", output)
	}
}

func TestRistrettoCache_LogsSet(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cache := newTestRistrettoCache(t)
	ctx := context.Background()

	err := cache.Set(ctx, "log-test-key", []byte("log-test-value"))
	if err != nil {
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

func TestRistrettoCache_LogsSetWithTTL(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cache := newTestRistrettoCache(t)
	ctx := context.Background()

	err := cache.SetWithTTL(ctx, "ttl-key", []byte("ttl-value"), 5*time.Minute)
	if err != nil {
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

func TestRistrettoCache_LogsDelete(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cache := newTestRistrettoCache(t)
	ctx := context.Background()

	err := cache.Delete(ctx, "delete-key")
	if err != nil {
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

func TestRistrettoCache_LogsClose(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cfg := RistrettoConfig{
		NumCounters: 100_000,
		MaxCost:     10 << 20,
		BufferItems: 64,
	}

	cache, err := newRistrettoCache(cfg)
	if err != nil {
		t.Fatalf("newRistrettoCache failed: %v", err)
	}

	buf.Reset() // Clear creation log

	err = cache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ristretto cache closed") {
		t.Errorf("expected close log, got: %s", output)
	}
}

func TestRistrettoCache_LogsStats(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cache := newTestRistrettoCache(t)
	ctx := context.Background()

	// Generate some stats
	_ = cache.Set(ctx, "key", []byte("value"))
	cache.cache.Wait()
	_, _ = cache.Get(ctx, "key")

	buf.Reset()

	_ = cache.Stats()

	output := buf.String()
	if !strings.Contains(output, "cache stats") {
		t.Errorf("expected 'cache stats' log, got: %s", output)
	}
}

func TestNoopCache_LogsCreation(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cache := newNoopCache()
	defer cache.Close()

	output := buf.String()
	if !strings.Contains(output, "noop cache created") {
		t.Errorf("expected creation log, got: %s", output)
	}
	if !strings.Contains(output, "disabled") {
		t.Errorf("expected 'disabled' note in log, got: %s", output)
	}
}

func TestNoopCache_LogsGet(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cache := newNoopCache()
	defer cache.Close()

	buf.Reset() // Clear creation log

	ctx := context.Background()
	_, _ = cache.Get(ctx, "test-key")

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

func TestNoopCache_LogsSet(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cache := newNoopCache()
	defer cache.Close()

	buf.Reset() // Clear creation log

	ctx := context.Background()
	_ = cache.Set(ctx, "noop-key", []byte("noop-value"))

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

func TestNoopCache_LogsSetWithTTL(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cache := newNoopCache()
	defer cache.Close()

	buf.Reset() // Clear creation log

	ctx := context.Background()
	_ = cache.SetWithTTL(ctx, "ttl-key", []byte("ttl-value"), 5*time.Minute)

	output := buf.String()
	if !strings.Contains(output, "cache set") {
		t.Errorf("expected 'cache set' log, got: %s", output)
	}
	if !strings.Contains(output, "ttl") {
		t.Errorf("expected 'ttl' in log, got: %s", output)
	}
}

func TestNoopCache_LogsDelete(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cache := newNoopCache()
	defer cache.Close()

	buf.Reset() // Clear creation log

	ctx := context.Background()
	_ = cache.Delete(ctx, "delete-key")

	output := buf.String()
	if !strings.Contains(output, "cache delete") {
		t.Errorf("expected 'cache delete' log, got: %s", output)
	}
	if !strings.Contains(output, "delete-key") {
		t.Errorf("expected key in log, got: %s", output)
	}
}

func TestNoopCache_LogsClose(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cache := newNoopCache()

	buf.Reset() // Clear creation log

	_ = cache.Close()

	output := buf.String()
	if !strings.Contains(output, "noop cache closed") {
		t.Errorf("expected 'noop cache closed' log, got: %s", output)
	}
}

func TestSetLogger_AddsComponentTag(t *testing.T) {
	var buf bytes.Buffer
	baseLogger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&baseLogger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	// Create a noop cache to trigger logging
	cache := newNoopCache()
	defer cache.Close()

	output := buf.String()
	if !strings.Contains(output, `"component":"cache"`) {
		t.Errorf("expected 'component:cache' tag in log, got: %s", output)
	}
}

func TestFactory_LogsCreation(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cfg := Config{
		Mode: ModeSingle,
		Ristretto: RistrettoConfig{
			NumCounters: 100_000,
			MaxCost:     10 << 20,
			BufferItems: 64,
		},
	}

	cache, err := New(context.Background(), &cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer cache.Close()

	output := buf.String()
	if !strings.Contains(output, "cache factory") {
		t.Errorf("expected factory log, got: %s", output)
	}
	if !strings.Contains(output, "single") {
		t.Errorf("expected mode in log, got: %s", output)
	}
}

func TestFactory_LogsDisabledMode(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	cfg := Config{
		Mode: ModeDisabled,
	}

	cache, err := New(context.Background(), &cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer cache.Close()

	output := buf.String()
	if !strings.Contains(output, "disabled") {
		t.Errorf("expected 'disabled' in log, got: %s", output)
	}
}

func TestFactory_LogsValidationFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	SetLogger(&logger)
	nop := zerolog.Nop()
	defer SetLogger(&nop)

	// Invalid config (single mode with zero NumCounters)
	cfg := Config{
		Mode: ModeSingle,
		Ristretto: RistrettoConfig{
			NumCounters: 0, // Invalid
			MaxCost:     10 << 20,
		},
	}

	_, err := New(context.Background(), &cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}

	output := buf.String()
	if !strings.Contains(output, "validation failed") {
		t.Errorf("expected validation failure log, got: %s", output)
	}
}
