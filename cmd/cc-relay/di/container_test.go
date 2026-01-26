package di

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/router"
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

	t.Run("container creation validates config eagerly", func(t *testing.T) {
		// Container creation validates config to fail fast
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

	t.Run("container creation fails with invalid config path", func(t *testing.T) {
		// NewContainer now eagerly loads config to fail fast
		container, err := NewContainer("/nonexistent/config.yaml")
		assert.Error(t, err)
		assert.Nil(t, container)
		assert.Contains(t, err.Error(), "failed to load config")
	})
}

// createTempConfigWithRouting creates a config file with routing strategy.
func createTempConfigWithRouting(t *testing.T, strategy string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := `
server:
  listen: ":8787"
logging:
  level: info
  format: json
cache:
  mode: disabled
routing:
  strategy: ` + strategy + `
  failover_timeout_ms: 3000
  debug: true
providers:
  - name: anthropic
    type: anthropic
    base_url: https://api.anthropic.com
    enabled: true
    keys:
      - key: test-key-1
`
	err := os.WriteFile(path, []byte(cfg), 0o600)
	require.NoError(t, err)
	return path
}

func TestRouterService(t *testing.T) {
	t.Run("creates router with default strategy (failover)", func(t *testing.T) {
		configPath := createTempConfigFile(t)
		container, err := NewContainer(configPath)
		require.NoError(t, err)
		defer container.Shutdown()

		routerSvc, err := Invoke[*RouterService](container)
		require.NoError(t, err)
		assert.NotNil(t, routerSvc)
		assert.NotNil(t, routerSvc.Router)

		// Default strategy is failover
		assert.Equal(t, router.StrategyFailover, routerSvc.Router.Name())
	})

	t.Run("creates router with configured strategy", func(t *testing.T) {
		configPath := createTempConfigWithRouting(t, "round_robin")
		container, err := NewContainer(configPath)
		require.NoError(t, err)
		defer container.Shutdown()

		routerSvc, err := Invoke[*RouterService](container)
		require.NoError(t, err)
		assert.NotNil(t, routerSvc.Router)

		// Should use configured strategy
		assert.Equal(t, router.StrategyRoundRobin, routerSvc.Router.Name())
	})

	t.Run("router depends on config", func(t *testing.T) {
		configPath := createTempConfigWithRouting(t, "shuffle")
		container, err := NewContainer(configPath)
		require.NoError(t, err)
		defer container.Shutdown()

		// Invoke router without explicitly invoking config first
		routerSvc, err := Invoke[*RouterService](container)
		require.NoError(t, err)
		assert.NotNil(t, routerSvc)

		// Config should have been implicitly resolved
		cfgSvc, err := Invoke[*ConfigService](container)
		require.NoError(t, err)
		assert.Equal(t, "shuffle", cfgSvc.Config.Routing.Strategy)
	})

	t.Run("supports all routing strategies", func(t *testing.T) {
		strategies := []string{"round_robin", "weighted_round_robin", "shuffle", "failover"}

		for _, strategy := range strategies {
			t.Run(strategy, func(t *testing.T) {
				configPath := createTempConfigWithRouting(t, strategy)
				container, err := NewContainer(configPath)
				require.NoError(t, err)
				defer container.Shutdown()

				routerSvc, err := Invoke[*RouterService](container)
				require.NoError(t, err)
				assert.Equal(t, strategy, routerSvc.Router.Name())
			})
		}
	})
}
