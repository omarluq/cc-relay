package config_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/config"
)

func TestNewWatcherPathResolution(t *testing.T) {
	t.Parallel()

	fix := config.NewTestWatcher(t)

	// Path should be absolute
	absPath, err := filepath.Abs(fix.ConfigPath)
	if err != nil {
		t.Fatalf("filepath.Abs failed: %v", err)
	}
	if fix.Watcher.Path() != absPath {
		t.Errorf("Expected path %s, got %s", absPath, fix.Watcher.Path())
	}
}

func TestNewWatcherInvalidPath(t *testing.T) {
	t.Parallel()

	// Path with non-existent directory should fail
	watcher, err := config.NewWatcher("/nonexistent/path/to/config.yaml")
	if err == nil {
		if closeErr := watcher.Close(); closeErr != nil {
			t.Errorf("watcher.Close failed: %v", closeErr)
		}
		t.Fatal("Expected error for non-existent path")
	}
}

func TestWatcherOnReload(t *testing.T) {
	t.Parallel()

	fix := config.NewTestWatcher(t)

	var callCount atomic.Int32
	callbackDone := make(chan struct{}, 1)

	fix.Watcher.OnReload(func(_ *config.Config) error {
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
		if watchErr := fix.Watcher.Watch(ctx); watchErr != nil && !errors.Is(watchErr, context.Canceled) {
			t.Errorf("watcher.Watch failed: %v", watchErr)
		}
	}()

	// Allow watcher to initialize
	time.Sleep(50 * time.Millisecond)

	// Modify the file
	writeTestConfig(t, fix.ConfigPath)

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

	// Use 200ms debounce to make test more reliable
	fix := config.NewTestWatcher(t, config.WithDebounceDelay(200*time.Millisecond))

	var callCount atomic.Int32

	fix.Watcher.OnReload(func(_ *config.Config) error {
		callCount.Add(1)
		return nil
	})

	// Start watching
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if watchErr := fix.Watcher.Watch(ctx); watchErr != nil && !errors.Is(watchErr, context.Canceled) {
			t.Errorf("watcher.Watch failed: %v", watchErr)
		}
	}()

	// Allow watcher to initialize
	time.Sleep(50 * time.Millisecond)

	// Rapid writes - 5 writes in quick succession
	for i := range 5 {
		writeTestConfigWithContent(t, fix.ConfigPath, i)
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

	fix := config.NewTestWatcher(t)

	ctx, cancel := context.WithCancel(context.Background())
	watchDone := make(chan struct{})

	go func() {
		if err := fix.Watcher.Watch(ctx); err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("watcher.Watch failed: %v", err)
		}
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

	fix := config.NewTestWatcher(t)
	otherPath := filepath.Join(filepath.Dir(fix.ConfigPath), "other.yaml")

	var callCount atomic.Int32

	fix.Watcher.OnReload(func(_ *config.Config) error {
		callCount.Add(1)
		return nil
	})

	// Start watching
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if watchErr := fix.Watcher.Watch(ctx); watchErr != nil && !errors.Is(watchErr, context.Canceled) {
			t.Errorf("watcher.Watch failed: %v", watchErr)
		}
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

	fix := config.NewTestWatcher(t)

	var callCount atomic.Int32

	fix.Watcher.OnReload(func(_ *config.Config) error {
		callCount.Add(1)
		return nil
	})

	// Start watching
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if watchErr := fix.Watcher.Watch(ctx); watchErr != nil && !errors.Is(watchErr, context.Canceled) {
			t.Errorf("watcher.Watch failed: %v", watchErr)
		}
	}()

	// Allow watcher to initialize
	time.Sleep(50 * time.Millisecond)

	// Write invalid YAML
	err := os.WriteFile(fix.ConfigPath, []byte("invalid: yaml: :::"), 0o600)
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

	fix := config.NewTestWatcher(t)

	var cb1Count, cb2Count, cb3Count atomic.Int32
	allDone := make(chan struct{}, 3)

	// Create a callback helper that counts and signals
	makeCallback := func(counter *atomic.Int32) func(*config.Config) error {
		return func(_ *config.Config) error {
			counter.Add(1)
			select {
			case allDone <- struct{}{}:
			default:
			}
			return nil
		}
	}

	fix.Watcher.OnReload(makeCallback(&cb1Count))
	fix.Watcher.OnReload(makeCallback(&cb2Count))
	fix.Watcher.OnReload(makeCallback(&cb3Count))

	// Start watching
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if watchErr := fix.Watcher.Watch(ctx); watchErr != nil && !errors.Is(watchErr, context.Canceled) {
			t.Errorf("watcher.Watch failed: %v", watchErr)
		}
	}()

	// Allow watcher to initialize
	time.Sleep(50 * time.Millisecond)

	// Modify the file
	writeTestConfig(t, fix.ConfigPath)

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
	assertCallbackInvoked(t, &cb1Count, "Callback 1")
	assertCallbackInvoked(t, &cb2Count, "Callback 2")
	assertCallbackInvoked(t, &cb3Count, "Callback 3")
}

func TestWatcherClose(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configPath)

	watcher, err := config.NewWatcher(configPath)
	if err != nil {
		t.Fatalf("config.NewWatcher failed: %v", err)
	}

	// Close should not error
	if err := watcher.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestWatcherConcurrentCallbackRegistration(t *testing.T) {
	t.Parallel()

	fix := config.NewTestWatcher(t)

	// Concurrent registration should be safe
	var waitGroup sync.WaitGroup
	for range 10 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			fix.Watcher.OnReload(func(_ *config.Config) error {
				return nil
			})
		}()
	}
	waitGroup.Wait()
}

func TestWithDebounceDelay(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configPath)

	customDelay := 500 * time.Millisecond
	watcher, err := config.NewWatcher(configPath, config.WithDebounceDelay(customDelay))
	if err != nil {
		t.Fatalf("config.NewWatcher failed: %v", err)
	}

	// Verify the delay was set (internal check via timing behavior)
	// We can't directly access the field, but the debounce test validates behavior
	if err := watcher.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// Helper functions

func assertCallbackInvoked(t *testing.T, counter *atomic.Int32, name string) {
	t.Helper()
	if counter.Load() < 1 {
		t.Errorf("%s not invoked", name)
	}
}

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
	err := os.WriteFile(path, []byte(content), 0o600)
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

	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}
}
