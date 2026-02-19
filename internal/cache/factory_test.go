package cache_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/cache"
)

const factoryTestKey = "test-key"

func TestNewModeSingleCreatesRistretto(t *testing.T) {
	t.Parallel()
	cfg := cache.Config{
		Mode:      cache.ModeSingle,
		Olric:     cache.ZeroOlricConfig(),
		Ristretto: cache.SmallTestRistrettoConfig(),
	}

	ctx := context.Background()
	cacheInst, err := cache.New(ctx, &cfg)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if closeErr := cacheInst.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	key := factoryTestKey
	value := []byte("test-value")

	if setErr := cacheInst.Set(ctx, key, value); setErr != nil {
		t.Fatalf("Set() error = %v, want nil", setErr)
	}

	// Ristretto is async, wait for write to complete
	time.Sleep(10 * time.Millisecond)

	got, err := cacheInst.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get() = %q, want %q", got, value)
	}

	if _, ok := cacheInst.(cache.StatsProvider); !ok {
		t.Error("expected cache to implement StatsProvider")
	}
}

func TestNewModeDisabledCreatesNoop(t *testing.T) {
	t.Parallel()
	cfg := cache.Config{
		Mode:      cache.ModeDisabled,
		Olric:     cache.ZeroOlricConfig(),
		Ristretto: cache.ZeroRistrettoConfig(),
	}

	ctx := context.Background()
	cacheInst, err := cache.New(ctx, &cfg)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if closeErr := cacheInst.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	key := factoryTestKey
	value := []byte("test-value")

	if setErr := cacheInst.Set(ctx, key, value); setErr != nil {
		t.Fatalf("Set() error = %v, want nil", setErr)
	}

	_, err = cacheInst.Get(ctx, key)
	if !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}

	exists, err := cacheInst.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() error = %v, want nil", err)
	}
	if exists {
		t.Error("Exists() = true, want false for noop cache")
	}
}

func TestNewModeHACreatesOlric(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping HA test in short mode (requires embedded Olric)")
	}

	cfg := cache.Config{
		Mode:      cache.ModeHA,
		Olric:     cache.DefaultTestOlricConfig("test-cache", "127.0.0.1:3320"),
		Ristretto: cache.ZeroRistrettoConfig(),
	}

	ctx := context.Background()
	cacheInst, err := cache.New(ctx, &cfg)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if closeErr := cacheInst.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	key := factoryTestKey
	value := []byte("test-value")

	if setErr := cacheInst.Set(ctx, key, value); setErr != nil {
		t.Fatalf("Set() error = %v, want nil", setErr)
	}

	got, err := cacheInst.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get() = %q, want %q", got, value)
	}
}

func TestNewInvalidModeReturnsError(t *testing.T) {
	t.Parallel()
	cfg := cache.Config{
		Mode:      cache.Mode("invalid-mode"),
		Olric:     cache.ZeroOlricConfig(),
		Ristretto: cache.ZeroRistrettoConfig(),
	}

	ctx := context.Background()
	_, err := cache.New(ctx, &cfg)
	if err == nil {
		t.Fatal("New() error = nil, want error for invalid mode")
	}

	if !cache.ContainsString(err.Error(), "invalid-mode") {
		t.Errorf("error message %q should mention 'invalid-mode'", err.Error())
	}
}

func TestNewInvalidConfigReturnsError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		wantErr string
		cfg     cache.Config
	}{
		{
			name:    "empty mode",
			cfg:     cache.Config{Mode: "", Olric: cache.ZeroOlricConfig(), Ristretto: cache.ZeroRistrettoConfig()},
			wantErr: "mode is required",
		},
		{
			name: "single mode with zero max_cost",
			cfg: cache.Config{
				Mode:      cache.ModeSingle,
				Olric:     cache.ZeroOlricConfig(),
				Ristretto: cache.RistrettoConfig{NumCounters: 1000, MaxCost: 0, BufferItems: 64},
			},
			wantErr: "max_cost must be positive",
		},
		{
			name: "single mode with zero num_counters",
			cfg: cache.Config{
				Mode:      cache.ModeSingle,
				Olric:     cache.ZeroOlricConfig(),
				Ristretto: cache.RistrettoConfig{NumCounters: 0, MaxCost: 1 << 20, BufferItems: 64},
			},
			wantErr: "num_counters must be positive",
		},
		{
			name: "ha mode without addresses and not embedded",
			cfg: cache.Config{
				Mode:      cache.ModeHA,
				Olric:     cache.ZeroOlricConfig(),
				Ristretto: cache.ZeroRistrettoConfig(),
			},
			wantErr: "addresses required",
		},
		{
			name: "ha mode embedded without bind_addr",
			cfg: cache.Config{
				Mode:      cache.ModeHA,
				Olric:     cache.DefaultTestOlricConfig("", ""),
				Ristretto: cache.ZeroRistrettoConfig(),
			},
			wantErr: "bind_addr required",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			_, err := cache.New(ctx, &testCase.cfg)
			if err == nil {
				t.Fatal("New() error = nil, want error")
			}
			if !cache.ContainsString(err.Error(), testCase.wantErr) {
				t.Errorf("error message %q should contain %q", err.Error(), testCase.wantErr)
			}
		})
	}
}

func TestNewDefaultConfigWorks(t *testing.T) {
	t.Parallel()
	// Test that DefaultRistrettoConfig produces a valid single-mode config
	cfg := cache.Config{
		Mode:      cache.ModeSingle,
		Olric:     cache.ZeroOlricConfig(),
		Ristretto: cache.DefaultRistrettoConfig(),
	}

	ctx := context.Background()
	cacheInst, err := cache.New(ctx, &cfg)
	if err != nil {
		t.Fatalf("New() with DefaultRistrettoConfig error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if closeErr := cacheInst.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	key := "default-test"
	value := []byte("default-value")

	if setErr := cacheInst.Set(ctx, key, value); setErr != nil {
		t.Fatalf("Set() error = %v, want nil", setErr)
	}

	// Wait for async write
	time.Sleep(10 * time.Millisecond)

	got, err := cacheInst.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get() = %q, want %q", got, value)
	}
}
