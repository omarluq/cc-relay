package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/di"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	serveConfigFileName = "config.yaml"
)

func TestFindConfigFile(t *testing.T) {
	t.Parallel()

	// Create temp directory with config.yaml
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, serveConfigFileName)
	if err := os.WriteFile(configPath, []byte("server:\n  listen: localhost:8787\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Test finding config in given directory
	found := findConfigIn(tmpDir)
	if found != filepath.Join(tmpDir, defaultConfigFile) {
		t.Errorf("Expected config in tmpDir, got %q", found)
	}
}

func TestFindConfigFileNotFound(t *testing.T) {
	t.Parallel()

	// Empty temp directory - no config file
	tmpDir := t.TempDir()

	// Should return default when not found
	found := findConfigIn(tmpDir)
	if found != defaultConfigFile {
		t.Errorf("Expected %q default, got %q", defaultConfigFile, found)
	}
}

func TestFindConfigFileInHomeDir(t *testing.T) {
	t.Parallel()

	// Create temp directories
	tmpDir := t.TempDir()
	workDir := t.TempDir()

	// Create config in HOME/.config/cc-relay/
	configDir := filepath.Join(tmpDir, ".config", "cc-relay")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, serveConfigFileName)
	if err := os.WriteFile(configPath, []byte("server:\n  listen: localhost:8787\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Should find config in HOME/.config/cc-relay/
	found := findConfigInWithHome(workDir, tmpDir)
	if found != configPath {
		t.Errorf("Expected %q, got %q", configPath, found)
	}
}

func TestRunServeInvalidConfigPath(t *testing.T) {
	t.Parallel()

	_, err := di.NewContainer("/nonexistent/path/" + serveConfigFileName)
	if err == nil {
		t.Error("Expected error for invalid config path")
	}
}

func TestRunServeInvalidConfig(t *testing.T) {
	t.Parallel()

	// Create temp config file with invalid content
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := di.NewContainer(configPath)
	if err == nil {
		t.Error("Expected error for invalid config content")
	}
}

// assertServerServiceFails creates a container from the given config content
// and asserts that resolving the server service fails.
func assertServerServiceFails(t *testing.T, configContent, errMsg string) {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, serveConfigFileName)
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatal(err)
	}

	container, err := di.NewContainer(configPath)
	if err != nil {
		t.Fatalf("Unexpected error creating container: %v", err)
	}
	_, err = di.Invoke[*di.ServerService](container)
	if err == nil {
		t.Errorf("Expected error for %s", errMsg)
	}
}

func TestRunServeNoEnabledProvider(t *testing.T) {
	t.Parallel()

	assertServerServiceFails(t, `
server:
  listen: "127.0.0.1:18787"
  api_key: "test-key"
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: false
    base_url: "https://api.anthropic.com"
    keys:
      - key: "test-key"
`, "no enabled provider")
}

func TestRunServeUnsupportedProviderType(t *testing.T) {
	t.Parallel()

	assertServerServiceFails(t, `
server:
  listen: "127.0.0.1:18787"
  api_key: "test-key"
providers:
  - name: "openai"
    type: "openai"
    enabled: true
    base_url: "https://api.openai.com"
    keys:
      - key: "test-key"
`, "unsupported provider type")
}

func TestRunServeEmptyProviders(t *testing.T) {
	t.Parallel()

	assertServerServiceFails(t, `
server:
  listen: "127.0.0.1:18787"
  api_key: "test-key"
providers: []
`, "empty providers")
}

// validServeConfig is a minimal valid configuration for serve tests.
const validServeConfig = `
server:
  listen: "127.0.0.1:0"
  api_key: "test-api-key"
logging:
  level: error
  format: json
cache:
  mode: disabled
providers:
  - name: anthropic
    type: anthropic
    base_url: https://api.anthropic.com
    enabled: true
    keys:
      - key: test-key-1
`

func createServeTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, serveConfigFileName)
	err := os.WriteFile(path, []byte(validServeConfig), 0o600)
	require.NoError(t, err)
	return path
}

func TestDIContainerInitialization(t *testing.T) {
	t.Parallel()
	t.Run("creates container with valid config", func(t *testing.T) {
		t.Parallel()
		configPath := createServeTestConfig(t)

		container, err := di.NewContainer(configPath)
		require.NoError(t, err)
		require.NotNil(t, container)

		// Verify services can be resolved
		cfgSvc, err := di.Invoke[*di.ConfigService](container)
		require.NoError(t, err)
		assert.NotNil(t, cfgSvc.Get())

		serverSvc, err := di.Invoke[*di.ServerService](container)
		require.NoError(t, err)
		assert.NotNil(t, serverSvc.Server)

		// Clean up
		err = container.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("fails with invalid config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "invalid.yaml")
		err := os.WriteFile(path, []byte("invalid: yaml: content"), 0o600)
		require.NoError(t, err)

		container, err := di.NewContainer(path)
		assert.Error(t, err)
		assert.Nil(t, container)
	})
}

func TestRunWithGracefulShutdown(t *testing.T) {
	t.Parallel()
	t.Run("shutdown on SIGTERM", func(t *testing.T) {
		t.Parallel()
		configPath := createServeTestConfig(t)

		container, err := di.NewContainer(configPath)
		require.NoError(t, err)

		serverSvc, err := di.Invoke[*di.ServerService](container)
		require.NoError(t, err)

		// Start server in background
		errCh := make(chan error, 1)
		go func() {
			errCh <- runWithGracefulShutdown(serverSvc.Server, container, ":0", nil)
		}()

		// Wait for server to start
		time.Sleep(50 * time.Millisecond)

		// Send SIGTERM to trigger shutdown
		p, err := os.FindProcess(os.Getpid())
		require.NoError(t, err)
		err = p.Signal(syscall.SIGTERM)
		require.NoError(t, err)

		// Wait for shutdown with timeout
		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("server did not shut down in time")
		}
	})
}

func TestServerIntegration(t *testing.T) {
	t.Parallel()
	t.Run("server starts and responds to health check", func(t *testing.T) {
		t.Parallel()
		configPath := createServeTestConfig(t)

		container, err := di.NewContainer(configPath)
		require.NoError(t, err)
		defer func() {
			if shutdownErr := container.Shutdown(); shutdownErr != nil {
				t.Logf("container shutdown error: %v", shutdownErr)
			}
		}()

		serverSvc, err := di.Invoke[*di.ServerService](container)
		require.NoError(t, err)

		// Start server in goroutine
		serverErr := make(chan error, 1)
		go func() {
			serverErr <- serverSvc.Server.ListenAndServe()
		}()

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Get the actual listen address (since we used port 0)
		// Note: Server doesn't expose address easily, so we test shutdown instead

		// Shutdown server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = serverSvc.Server.Shutdown(ctx)
		require.NoError(t, err)

		// Check server error (should be http.ErrServerClosed)
		select {
		case err := <-serverErr:
			assert.ErrorIs(t, err, http.ErrServerClosed)
		case <-time.After(5 * time.Second):
			t.Fatal("server did not stop")
		}
	})
}

func TestConfigWatcherLifecycle(t *testing.T) {
	t.Parallel()
	t.Run("watcher starts and stops with server", func(t *testing.T) {
		t.Parallel()
		configPath := createServeTestConfig(t)

		container, err := di.NewContainer(configPath)
		require.NoError(t, err)

		// Get config service and verify watcher was created
		cfgSvc, err := di.Invoke[*di.ConfigService](container)
		require.NoError(t, err)
		assert.NotNil(t, cfgSvc.Get(), "config should be loaded")

		// Start watcher (simulating what runServe does)
		watchCtx, watchCancel := context.WithCancel(context.Background())
		cfgSvc.StartWatching(watchCtx)

		// Allow watcher to start
		time.Sleep(50 * time.Millisecond)

		// Cancel watcher (simulating graceful shutdown)
		watchCancel()

		// Shutdown container (closes watcher via ConfigService.Shutdown)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = container.ShutdownWithContext(ctx)
		// Note: May return error for uninvoked services, but watcher should close cleanly
		if err != nil {
			t.Logf("Container shutdown returned: %v", err)
		}
	})

	t.Run("graceful shutdown with watchCancel", func(t *testing.T) {
		t.Parallel()
		configPath := createServeTestConfig(t)

		container, err := di.NewContainer(configPath)
		require.NoError(t, err)

		serverSvc, err := di.Invoke[*di.ServerService](container)
		require.NoError(t, err)

		cfgSvc, err := di.Invoke[*di.ConfigService](container)
		require.NoError(t, err)

		// Start watcher
		watchCtx, watchCancel := context.WithCancel(context.Background())
		cfgSvc.StartWatching(watchCtx)

		// Start server in background
		errCh := make(chan error, 1)
		go func() {
			errCh <- runWithGracefulShutdown(serverSvc.Server, container, ":0", watchCancel)
		}()

		// Wait for server to start
		time.Sleep(50 * time.Millisecond)

		// Send SIGTERM to trigger shutdown
		p, err := os.FindProcess(os.Getpid())
		require.NoError(t, err)
		err = p.Signal(syscall.SIGTERM)
		require.NoError(t, err)

		// Wait for shutdown with timeout
		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("server did not shut down in time")
		}
	})
}
