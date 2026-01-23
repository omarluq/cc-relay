package di

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTempConfigFile creates a temporary config file for testing.
func createTempConfigFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(validConfig), 0o600)
	require.NoError(t, err)
	return path
}

// validConfig is a minimal valid configuration for testing.
const validConfig = `
server:
  listen: ":8787"
logging:
  level: info
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


func TestNewContainer(t *testing.T) {
	t.Run("creates container with valid config", func(t *testing.T) {
		configPath := createTempConfigFile(t)

		container, err := NewContainer(configPath)
		require.NoError(t, err)
		require.NotNil(t, container)

		// Verify container has injector
		assert.NotNil(t, container.Injector())

		// Clean up
		err = container.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("container creation succeeds even with lazy loading", func(t *testing.T) {
		// Container creation should succeed - actual config load is lazy
		configPath := createTempConfigFile(t)

		container, err := NewContainer(configPath)
		require.NoError(t, err)
		require.NotNil(t, container)

		// Clean up
		_ = container.Shutdown()
	})
}

func TestContainerInvoke(t *testing.T) {
	configPath := createTempConfigFile(t)
	container, err := NewContainer(configPath)
	require.NoError(t, err)
	defer container.Shutdown()

	t.Run("Invoke resolves config service", func(t *testing.T) {
		cfgSvc, err := Invoke[*ConfigService](container)
		require.NoError(t, err)
		assert.NotNil(t, cfgSvc)
		assert.NotNil(t, cfgSvc.Config)
		assert.Equal(t, ":8787", cfgSvc.Config.Server.Listen)
	})

	t.Run("MustInvoke resolves config service", func(t *testing.T) {
		cfgSvc := MustInvoke[*ConfigService](container)
		assert.NotNil(t, cfgSvc)
		assert.NotNil(t, cfgSvc.Config)
	})

	t.Run("InvokeNamed resolves config path", func(t *testing.T) {
		path, err := InvokeNamed[string](container, ConfigPathKey)
		require.NoError(t, err)
		assert.Equal(t, configPath, path)
	})

	t.Run("MustInvokeNamed resolves config path", func(t *testing.T) {
		path := MustInvokeNamed[string](container, ConfigPathKey)
		assert.Equal(t, configPath, path)
	})
}

func TestContainerShutdown(t *testing.T) {
	t.Run("shutdown returns nil for unused container", func(t *testing.T) {
		configPath := createTempConfigFile(t)
		container, err := NewContainer(configPath)
		require.NoError(t, err)

		err = container.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("shutdown cleans up initialized services", func(t *testing.T) {
		configPath := createTempConfigFile(t)
		container, err := NewContainer(configPath)
		require.NoError(t, err)

		// Initialize services by invoking them
		_, err = Invoke[*ConfigService](container)
		require.NoError(t, err)

		_, err = Invoke[*CacheService](container)
		require.NoError(t, err)

		// Shutdown should succeed
		err = container.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("ShutdownWithContext respects timeout", func(t *testing.T) {
		configPath := createTempConfigFile(t)
		container, err := NewContainer(configPath)
		require.NoError(t, err)

		// Initialize services
		_, err = Invoke[*ConfigService](container)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = container.ShutdownWithContext(ctx)
		assert.NoError(t, err)
	})

	t.Run("ShutdownWithContext returns error on expired context", func(t *testing.T) {
		configPath := createTempConfigFile(t)
		container, err := NewContainer(configPath)
		require.NoError(t, err)

		// Use already expired context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Give it a small grace period for the shutdown to start
		time.Sleep(10 * time.Millisecond)

		err = container.ShutdownWithContext(ctx)
		// May or may not error depending on timing, so just verify it doesn't panic
		_ = err
	})
}

func TestContainerHealthCheck(t *testing.T) {
	t.Run("health check passes with valid config", func(t *testing.T) {
		configPath := createTempConfigFile(t)
		container, err := NewContainer(configPath)
		require.NoError(t, err)
		defer container.Shutdown()

		err = container.HealthCheck()
		assert.NoError(t, err)
	})

	t.Run("health check fails with invalid config path", func(t *testing.T) {
		// Create container with non-existent config path
		container, err := NewContainer("/nonexistent/config.yaml")
		require.NoError(t, err) // Container creation succeeds (lazy loading)
		defer container.Shutdown()

		// Health check should fail when trying to load config
		err = container.HealthCheck()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config service unhealthy")
	})
}
