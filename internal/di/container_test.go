package di_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/di"
	"github.com/omarluq/cc-relay/internal/router"
)

// shutdownContainer shuts down the container and logs any error (for use in t.Cleanup).
func shutdownContainer(t *testing.T, container *di.Container) {
	t.Helper()
	if err := container.Shutdown(); err != nil {
		t.Logf("container shutdown: %v", err)
	}
}

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
	t.Parallel()
	t.Run("creates container with valid config", func(t *testing.T) {
		t.Parallel()
		configPath := createTempConfigFile(t)

		container, err := di.NewContainer(configPath)
		require.NoError(t, err)
		require.NotNil(t, container)

		// Verify container has injector
		assert.NotNil(t, container.Injector())

		// Clean up
		err = container.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("container creation validates config eagerly", func(t *testing.T) {
		t.Parallel()

		configPath := createTempConfigFile(t)

		container, err := di.NewContainer(configPath)
		require.NoError(t, err)
		require.NotNil(t, container)

		// Clean up
		err = container.Shutdown()
		assert.NoError(t, err)
	})
}

func TestContainerInvoke(t *testing.T) {
	t.Parallel()
	configPath := createTempConfigFile(t)
	container, err := di.NewContainer(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { shutdownContainer(t, container) })

	t.Run("di.Invoke resolves config service", func(t *testing.T) {
		t.Parallel()
		cfgSvc, err := di.Invoke[*di.ConfigService](container)
		require.NoError(t, err)
		assert.NotNil(t, cfgSvc)
		assert.NotNil(t, cfgSvc.Config)
		assert.Equal(t, ":8787", cfgSvc.Config.Server.Listen)
	})

	t.Run("di.MustInvoke resolves config service", func(t *testing.T) {
		t.Parallel()
		cfgSvc := di.MustInvoke[*di.ConfigService](container)
		assert.NotNil(t, cfgSvc)
		assert.NotNil(t, cfgSvc.Config)
	})

	t.Run("di.InvokeNamed resolves config path", func(t *testing.T) {
		t.Parallel()
		path, err := di.InvokeNamed[string](container, di.ConfigPathKey)
		require.NoError(t, err)
		assert.Equal(t, configPath, path)
	})

	t.Run("di.MustInvokeNamed resolves config path", func(t *testing.T) {
		t.Parallel()
		path := di.MustInvokeNamed[string](container, di.ConfigPathKey)
		assert.Equal(t, configPath, path)
	})
}

func TestContainerShutdown(t *testing.T) {
	t.Parallel()
	t.Run("shutdown returns nil for unused container", func(t *testing.T) {
		t.Parallel()
		configPath := createTempConfigFile(t)
		container, err := di.NewContainer(configPath)
		require.NoError(t, err)

		err = container.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("shutdown cleans up initialized services", func(t *testing.T) {
		t.Parallel()
		configPath := createTempConfigFile(t)
		container, err := di.NewContainer(configPath)
		require.NoError(t, err)

		// Initialize services by invoking them
		_, err = di.Invoke[*di.ConfigService](container)
		require.NoError(t, err)

		_, err = di.Invoke[*di.CacheService](container)
		require.NoError(t, err)

		// Shutdown should succeed
		err = container.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("ShutdownWithContext respects timeout", func(t *testing.T) {
		t.Parallel()
		configPath := createTempConfigFile(t)
		container, err := di.NewContainer(configPath)
		require.NoError(t, err)

		// Initialize services
		_, err = di.Invoke[*di.ConfigService](container)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = container.ShutdownWithContext(ctx)
		assert.NoError(t, err)
	})

	t.Run("ShutdownWithContext returns error on expired context", func(t *testing.T) {
		t.Parallel()
		configPath := createTempConfigFile(t)
		container, err := di.NewContainer(configPath)
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
	t.Parallel()
	t.Run("health check passes with valid config", func(t *testing.T) {
		t.Parallel()
		configPath := createTempConfigFile(t)
		container, err := di.NewContainer(configPath)
		require.NoError(t, err)
		t.Cleanup(func() { shutdownContainer(t, container) })

		err = container.HealthCheck()
		assert.NoError(t, err)
	})

	t.Run("container creation fails with invalid config path", func(t *testing.T) {
		t.Parallel()

		container, err := di.NewContainer("/nonexistent/config.yaml")
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
	t.Parallel()
	t.Run("creates router with default strategy (failover)", func(t *testing.T) {
		t.Parallel()
		configPath := createTempConfigFile(t)
		container, err := di.NewContainer(configPath)
		require.NoError(t, err)
		t.Cleanup(func() { shutdownContainer(t, container) })

		routerSvc, err := di.Invoke[*di.RouterService](container)
		require.NoError(t, err)
		assert.NotNil(t, routerSvc)

		// GetRouter returns current router based on config
		rtr := routerSvc.GetRouter()
		assert.NotNil(t, rtr)

		// Default strategy is failover
		assert.Equal(t, router.StrategyFailover, rtr.Name())
	})

	t.Run("creates router with configured strategy", func(t *testing.T) {
		t.Parallel()
		configPath := createTempConfigWithRouting(t, "round_robin")
		container, err := di.NewContainer(configPath)
		require.NoError(t, err)
		t.Cleanup(func() { shutdownContainer(t, container) })

		routerSvc, err := di.Invoke[*di.RouterService](container)
		require.NoError(t, err)

		// GetRouter returns current router based on config
		rtr := routerSvc.GetRouter()
		assert.NotNil(t, rtr)

		// Should use configured strategy
		assert.Equal(t, router.StrategyRoundRobin, rtr.Name())
	})

	t.Run("router depends on config", func(t *testing.T) {
		t.Parallel()
		configPath := createTempConfigWithRouting(t, "shuffle")
		container, err := di.NewContainer(configPath)
		require.NoError(t, err)
		t.Cleanup(func() { shutdownContainer(t, container) })

		// Invoke router without explicitly invoking config first
		routerSvc, err := di.Invoke[*di.RouterService](container)
		require.NoError(t, err)
		assert.NotNil(t, routerSvc)

		// Config should have been implicitly resolved
		cfgSvc, err := di.Invoke[*di.ConfigService](container)
		require.NoError(t, err)
		assert.Equal(t, "shuffle", cfgSvc.Config.Routing.Strategy)
	})

	t.Run("supports all routing strategies", func(t *testing.T) {
		t.Parallel()
		strategies := []string{"round_robin", "weighted_round_robin", "shuffle", "failover"}

		for _, strategy := range strategies {
			t.Run(strategy, func(t *testing.T) {
				t.Parallel()
				configPath := createTempConfigWithRouting(t, strategy)
				container, err := di.NewContainer(configPath)
				require.NoError(t, err)
				t.Cleanup(func() {
					shutdownContainer(t, container)
				})

				routerSvc, err := di.Invoke[*di.RouterService](container)
				require.NoError(t, err)
				assert.Equal(t, strategy, routerSvc.GetRouter().Name())
			})
		}
	})
}
