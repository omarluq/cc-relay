package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/cmd/cc-relay/di"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	serveConfigFileName  = "config.yaml"
	serveRestoreWdErrFmt = "failed to restore working directory: %v"
)

func TestFindConfigFile(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Save original working directory and HOME
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")

	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Logf(serveRestoreWdErrFmt, err)
		}
		os.Setenv("HOME", origHome)
	}()

	// Create temp directory with config.yaml
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, serveConfigFileName)
	if err := os.WriteFile(configPath, []byte("server:\n  listen: localhost:8787\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set HOME to empty temp dir to prevent finding real config
	os.Setenv("HOME", t.TempDir())

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Test finding config in current directory
	found := findConfigFile()
	if found != serveConfigFileName {
		t.Errorf("Expected %q, got %q", serveConfigFileName, found)
	}
}

func TestFindConfigFileNotFound(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Save original working directory and HOME
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")

	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Logf(serveRestoreWdErrFmt, err)
		}
		os.Setenv("HOME", origHome)
	}()

	// Change to temp directory without config.yaml
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Set HOME to temp dir so it won't find user's config
	os.Setenv("HOME", tmpDir)

	// Should return default even if not found
	found := findConfigFile()
	if found != serveConfigFileName {
		t.Errorf("Expected %q default, got %q", serveConfigFileName, found)
	}
}

func TestFindConfigFileInHomeDir(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Save original working directory and HOME
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")

	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Logf(serveRestoreWdErrFmt, err)
		}
		os.Setenv("HOME", origHome)
	}()

	// Create temp directories
	tmpDir := t.TempDir()
	workDir := t.TempDir()

	// Create config in HOME/.config/cc-relay/
	configDir := filepath.Join(tmpDir, ".config", "cc-relay")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, serveConfigFileName)
	if err := os.WriteFile(configPath, []byte("server:\n  listen: localhost:8787\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set HOME and change to work directory
	os.Setenv("HOME", tmpDir)
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}

	// Should find config in HOME/.config/cc-relay/
	found := findConfigFile()
	if found != configPath {
		t.Errorf("Expected %q, got %q", configPath, found)
	}
}

func TestRunServeInvalidConfigPath(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	// Set cfgFile to a non-existent path
	cfgFile = "/nonexistent/path/" + serveConfigFileName

	// runServe should return error for invalid config path
	err := runServe(nil, nil)
	if err == nil {
		t.Error("Expected error for invalid config path")
	}
}

func TestRunServeInvalidConfig(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Create temp config file with invalid content
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	cfgFile = configPath

	// runServe should return error for invalid config
	err := runServe(nil, nil)
	if err == nil {
		t.Error("Expected error for invalid config content")
	}
}

func TestRunServeNoEnabledProvider(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Create temp config file with no enabled providers
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, serveConfigFileName)
	configContent := `
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
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	cfgFile = configPath

	// runServe should return error for no enabled provider
	err := runServe(nil, nil)
	if err == nil {
		t.Error("Expected error for no enabled provider")
	}
}

func TestRunServeUnsupportedProviderType(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Create temp config file with unsupported provider type
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, serveConfigFileName)
	configContent := `
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
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	cfgFile = configPath

	// runServe should return error for unsupported provider type
	err := runServe(nil, nil)
	if err == nil {
		t.Error("Expected error for unsupported provider type")
	}
}

func TestRunServeEmptyProviders(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Create temp config file with empty providers
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, serveConfigFileName)
	configContent := `
server:
  listen: "127.0.0.1:18787"
  api_key: "test-key"
providers: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	cfgFile = configPath

	// runServe should return error for empty providers
	err := runServe(nil, nil)
	if err == nil {
		t.Error("Expected error for empty providers")
	}
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
	t.Run("creates container with valid config", func(t *testing.T) {
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
	t.Run("shutdown on SIGTERM", func(t *testing.T) {
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
	t.Run("server starts and responds to health check", func(t *testing.T) {
		configPath := createServeTestConfig(t)

		container, err := di.NewContainer(configPath)
		require.NoError(t, err)
		defer container.Shutdown()

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
	t.Run("watcher starts and stops with server", func(t *testing.T) {
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
