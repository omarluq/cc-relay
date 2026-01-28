package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewWatcherPathResolution(t *testing.T) {
	t.Parallel()

	// Create temp directory with a config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configPath)

	// Create watcher with relative path
	relPath := filepath.Join(tmpDir, "config.yaml")
	w, err := NewWatcher(relPath)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Close()

	// Path should be absolute
	absPath, _ := filepath.Abs(relPath)
	if w.Path() != absPath {
		t.Errorf("Expected path %s, got %s", absPath, w.Path())
	}
}

func TestNewWatcherInvalidPath(t *testing.T) {
	t.Parallel()

	// Path with non-existent directory should fail
	w, err := NewWatcher("/nonexistent/path/to/config.yaml")
	if err == nil {
		w.Close()
		t.Fatal("Expected error for non-existent path")
	}
}

func TestWatcherOnReload(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configPath)

	w, err := NewWatcher(configPath)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Close()

	var callCount atomic.Int32
	callbackDone := make(chan struct{}, 1)

	w.OnReload(func(_ *Config) error {
		callCount.Add(1)
		select {
		case callbackDone <- struct{}{}:
		default:
		}
		return nil
	})

	// Start watching
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = w.Watch(ctx)
	}()

	// Allow watcher to initialize
	time.Sleep(50 * time.Millisecond)

	// Modify the file
	writeTestConfig(t, configPath)

	// Wait for callback
	select {
	case <-callbackDone:
		// Callback invoked
	case <-time.After(2 * time.Second):
		t.Fatal("Callback not invoked within timeout")
	}

	cancel()

	if callCount.Load() < 1 {
		t.Errorf("Expected at least 1 callback, got %d", callCount.Load())
	}
}

func TestWatcherDebounce(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configPath)

	// Use 200ms debounce to make test more reliable
	w, err := NewWatcher(configPath, WithDebounceDelay(200*time.Millisecond))
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Close()

	var callCount atomic.Int32

	w.OnReload(func(_ *Config) error {
		callCount.Add(1)
		return nil
	})

	// Start watching
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = w.Watch(ctx)
	}()

	// Allow watcher to initialize
	time.Sleep(50 * time.Millisecond)

	// Rapid writes - 5 writes in quick succession
	for i := range 5 {
		writeTestConfigWithContent(t, configPath, i)
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for debounce to settle + some margin
	time.Sleep(400 * time.Millisecond)

	cancel()

	// With debouncing, we expect 1-2 callbacks (not 5)
	count := callCount.Load()
	if count > 2 {
		t.Errorf("Expected at most 2 callbacks due to debouncing, got %d", count)
	}
	if count < 1 {
		t.Errorf("Expected at least 1 callback, got %d", count)
	}
}

func TestWatcherContextCancellation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configPath)

	w, err := NewWatcher(configPath)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	watchDone := make(chan struct{})

	go func() {
		_ = w.Watch(ctx)
		close(watchDone)
	}()

	// Allow watcher to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Watch should return promptly
	select {
	case <-watchDone:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Watch did not return after context cancellation")
	}
}

func TestWatcherIgnoresOtherFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	otherPath := filepath.Join(tmpDir, "other.yaml")
	writeTestConfig(t, configPath)

	w, err := NewWatcher(configPath)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Close()

	var callCount atomic.Int32

	w.OnReload(func(_ *Config) error {
		callCount.Add(1)
		return nil
	})

	// Start watching
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = w.Watch(ctx)
	}()

	// Allow watcher to initialize
	time.Sleep(50 * time.Millisecond)

	// Write to a different file in the same directory
	writeTestConfig(t, otherPath)

	// Wait a bit to ensure no callback triggered
	time.Sleep(200 * time.Millisecond)

	cancel()

	if callCount.Load() != 0 {
		t.Errorf("Expected 0 callbacks for other file changes, got %d", callCount.Load())
	}
}

func TestWatcherInvalidConfigDoesNotCallback(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configPath)

	w, err := NewWatcher(configPath)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Close()

	var callCount atomic.Int32

	w.OnReload(func(_ *Config) error {
		callCount.Add(1)
		return nil
	})

	// Start watching
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = w.Watch(ctx)
	}()

	// Allow watcher to initialize
	time.Sleep(50 * time.Millisecond)

	// Write invalid YAML
	err = os.WriteFile(configPath, []byte("invalid: yaml: :::"), 0o644)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	// Wait for potential callback
	time.Sleep(200 * time.Millisecond)

	cancel()

	// Invalid config should not trigger callback
	if callCount.Load() != 0 {
		t.Errorf("Expected 0 callbacks for invalid config, got %d", callCount.Load())
	}
}

func TestWatcherMultipleCallbacks(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configPath)

	w, err := NewWatcher(configPath)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Close()

	var cb1Count, cb2Count, cb3Count atomic.Int32
	allDone := make(chan struct{}, 3)

	w.OnReload(func(_ *Config) error {
		cb1Count.Add(1)
		select {
		case allDone <- struct{}{}:
		default:
		}
		return nil
	})

	w.OnReload(func(_ *Config) error {
		cb2Count.Add(1)
		select {
		case allDone <- struct{}{}:
		default:
		}
		return nil
	})

	w.OnReload(func(_ *Config) error {
		cb3Count.Add(1)
		select {
		case allDone <- struct{}{}:
		default:
		}
		return nil
	})

	// Start watching
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = w.Watch(ctx)
	}()

	// Allow watcher to initialize
	time.Sleep(50 * time.Millisecond)

	// Modify the file
	writeTestConfig(t, configPath)

	// Wait for all callbacks
	timeout := time.After(2 * time.Second)
	for range 3 {
		select {
		case <-allDone:
		case <-timeout:
			t.Fatal("Not all callbacks invoked within timeout")
		}
	}

	cancel()

	// All three callbacks should have been called
	if cb1Count.Load() < 1 {
		t.Errorf("Callback 1 not invoked")
	}
	if cb2Count.Load() < 1 {
		t.Errorf("Callback 2 not invoked")
	}
	if cb3Count.Load() < 1 {
		t.Errorf("Callback 3 not invoked")
	}
}

func TestWatcherClose(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configPath)

	w, err := NewWatcher(configPath)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	// Close should not error
	if err := w.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestWatcherConcurrentCallbackRegistration(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configPath)

	w, err := NewWatcher(configPath)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Close()

	// Concurrent registration should be safe
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.OnReload(func(_ *Config) error {
				return nil
			})
		}()
	}
	wg.Wait()
}

func TestWithDebounceDelay(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configPath)

	customDelay := 500 * time.Millisecond
	w, err := NewWatcher(configPath, WithDebounceDelay(customDelay))
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}

	// Verify the delay was set (internal check via timing behavior)
	// We can't directly access the field, but the debounce test validates behavior
	w.Close()
}

// Helper functions

func writeTestConfig(t *testing.T, path string) {
	t.Helper()
	content := `
server:
  listen: "127.0.0.1:8787"
  timeout_ms: 60000

providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "sk-ant-test"
        rpm_limit: 60
        tpm_limit: 100000

logging:
  level: "info"
  format: "json"
`
	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}
}

func writeTestConfigWithContent(t *testing.T, path string, variant int) {
	t.Helper()
	content := fmt.Sprintf(`
server:
  listen: "127.0.0.1:8787"
  timeout_ms: %d

providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "sk-ant-test"
        rpm_limit: 60
        tpm_limit: 100000

logging:
  level: "info"
  format: "json"
`, 60000+variant)

	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}
}
