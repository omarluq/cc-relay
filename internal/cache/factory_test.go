package cache

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewModeSingleCreatesRistretto(t *testing.T) {
	cfg := Config{
		Mode: ModeSingle,
		Ristretto: RistrettoConfig{
			NumCounters: 1000,
			MaxCost:     1 << 20, // 1 MB
			BufferItems: 64,
		},
	}

	ctx := context.Background()
	c, err := New(ctx, &cfg)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	defer c.Close()

	// Verify it's a working cache by using it
	key := "test-key"
	value := []byte("test-value")

	err = c.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set() error = %v, want nil", err)
	}

	// Ristretto is async, wait for write to complete
	time.Sleep(10 * time.Millisecond)

	got, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get() = %q, want %q", got, value)
	}

	// Verify it implements StatsProvider (Ristretto does)
	if _, ok := c.(StatsProvider); !ok {
		t.Error("expected cache to implement StatsProvider")
	}
}

func TestNewModeDisabledCreatesNoop(t *testing.T) {
	cfg := Config{
		Mode: ModeDisabled,
	}

	ctx := context.Background()
	c, err := New(ctx, &cfg)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	defer c.Close()

	// Verify noop behavior: Set succeeds but Get returns ErrNotFound
	key := "test-key"
	value := []byte("test-value")

	err = c.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set() error = %v, want nil", err)
	}

	_, err = c.Get(ctx, key)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}

	// Verify Exists returns false
	exists, err := c.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() error = %v, want nil", err)
	}
	if exists {
		t.Error("Exists() = true, want false for noop cache")
	}
}

func TestNewModeHACreatesOlric(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping HA test in short mode (requires embedded Olric)")
	}

	// Use embedded mode for testing (no external Olric cluster needed)
	cfg := Config{
		Mode: ModeHA,
		Olric: OlricConfig{
			Embedded: true,
			BindAddr: "127.0.0.1:3320", // Default Olric port
			DMapName: "test-cache",
		},
	}

	ctx := context.Background()
	c, err := New(ctx, &cfg)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	defer c.Close()

	// Verify it's a working cache by using it
	key := "test-key"
	value := []byte("test-value")

	err = c.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set() error = %v, want nil", err)
	}

	got, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get() = %q, want %q", got, value)
	}
}

func TestNewInvalidModeReturnsError(t *testing.T) {
	cfg := Config{
		Mode: Mode("invalid-mode"),
	}

	ctx := context.Background()
	_, err := New(ctx, &cfg)
	if err == nil {
		t.Fatal("New() error = nil, want error for invalid mode")
	}

	// Check error message mentions the invalid mode
	if !containsString(err.Error(), "invalid-mode") {
		t.Errorf("error message %q should mention 'invalid-mode'", err.Error())
	}
}

func TestNewInvalidConfigReturnsError(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		cfg     Config
	}{
		{
			name: "empty mode",
			cfg: Config{
				Mode: "",
			},
			wantErr: "mode is required",
		},
		{
			name: "single mode with zero max_cost",
			cfg: Config{
				Mode: ModeSingle,
				Ristretto: RistrettoConfig{
					NumCounters: 1000,
					MaxCost:     0,
					BufferItems: 64,
				},
			},
			wantErr: "max_cost must be positive",
		},
		{
			name: "single mode with zero num_counters",
			cfg: Config{
				Mode: ModeSingle,
				Ristretto: RistrettoConfig{
					NumCounters: 0,
					MaxCost:     1 << 20,
					BufferItems: 64,
				},
			},
			wantErr: "num_counters must be positive",
		},
		{
			name: "ha mode without addresses and not embedded",
			cfg: Config{
				Mode: ModeHA,
				Olric: OlricConfig{
					Embedded:  false,
					Addresses: nil,
				},
			},
			wantErr: "addresses required",
		},
		{
			name: "ha mode embedded without bind_addr",
			cfg: Config{
				Mode: ModeHA,
				Olric: OlricConfig{
					Embedded: true,
					BindAddr: "",
				},
			},
			wantErr: "bind_addr required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := New(ctx, &tt.cfg)
			if err == nil {
				t.Fatal("New() error = nil, want error")
			}
			if !containsString(err.Error(), tt.wantErr) {
				t.Errorf("error message %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNewDefaultConfigWorks(t *testing.T) {
	// Test that DefaultRistrettoConfig produces a valid single-mode config
	cfg := Config{
		Mode:      ModeSingle,
		Ristretto: DefaultRistrettoConfig(),
	}

	ctx := context.Background()
	c, err := New(ctx, &cfg)
	if err != nil {
		t.Fatalf("New() with DefaultRistrettoConfig error = %v, want nil", err)
	}
	defer c.Close()

	// Verify basic operations work
	key := "default-test"
	value := []byte("default-value")

	err = c.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set() error = %v, want nil", err)
	}

	// Wait for async write
	time.Sleep(10 * time.Millisecond)

	got, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get() = %q, want %q", got, value)
	}
}

// containsString checks if a string contains a substring (case-insensitive not needed here).
func containsString(s, substr string) bool {
	if substr == "" {
		return true
	}
	if s == "" {
		return false
	}
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
