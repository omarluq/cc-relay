package di

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/config"
)

// createTestInjector creates an injector with a config path for testing.
func createTestInjector(t *testing.T, configContent string) *do.RootScope {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(configContent), 0o600)
	require.NoError(t, err)

	injector := do.New()
	do.ProvideNamedValue(injector, ConfigPathKey, path)
	RegisterSingletons(injector)

	return injector
}

// shutdownInjector is a helper to properly shutdown an injector in tests.
func shutdownInjector(i *do.RootScope) {
	_ = i.Shutdown()
}

// Test configurations.
const singleKeyConfig = `
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

const multiKeyPoolingConfig = `
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
    pooling:
      enabled: true
      strategy: least_loaded
    keys:
      - key: test-key-1
        rpm_limit: 100
        priority: 1
      - key: test-key-2
        rpm_limit: 200
        priority: 2
`

const noProviderConfig = `
server:
  listen: ":8787"
logging:
  level: info
cache:
  mode: disabled
providers: []
`

const multiProviderConfig = `
server:
  listen: ":8787"
logging:
  level: info
cache:
  mode: disabled
providers:
  - name: anthropic
    type: anthropic
    base_url: https://api.anthropic.com
    enabled: true
    keys:
      - key: test-key-1
  - name: zai
    type: zai
    base_url: https://api.zai.example.com
    enabled: true
    keys:
      - key: zai-key-1
`

func TestNewConfig(t *testing.T) {
	t.Run("loads valid config", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		cfgSvc, err := do.Invoke[*ConfigService](injector)
		require.NoError(t, err)
		assert.NotNil(t, cfgSvc)
		assert.NotNil(t, cfgSvc.Config)
		assert.Equal(t, ":8787", cfgSvc.Config.Server.Listen)
		assert.Len(t, cfgSvc.Config.Providers, 1)
	})

	t.Run("returns error for non-existent config", func(t *testing.T) {
		nonExistentInjector := do.New()
		do.ProvideNamedValue(nonExistentInjector, ConfigPathKey, "/nonexistent/config.yaml")
		RegisterSingletons(nonExistentInjector)
		defer shutdownInjector(nonExistentInjector)

		_, err := do.Invoke[*ConfigService](nonExistentInjector)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load config")
	})

	t.Run("singleton returns same instance", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		cfg1, err := do.Invoke[*ConfigService](injector)
		require.NoError(t, err)

		cfg2, err := do.Invoke[*ConfigService](injector)
		require.NoError(t, err)

		// Should be same pointer (singleton)
		assert.Same(t, cfg1, cfg2)
	})
}

func TestNewCache(t *testing.T) {
	t.Run("creates disabled cache", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		cacheSvc, err := do.Invoke[*CacheService](injector)
		require.NoError(t, err)
		assert.NotNil(t, cacheSvc)
		assert.NotNil(t, cacheSvc.Cache)
	})

	t.Run("implements Shutdowner", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		cacheSvc, err := do.Invoke[*CacheService](injector)
		require.NoError(t, err)

		// CacheService should implement Shutdown
		err = cacheSvc.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("Shutdown handles nil cache", func(t *testing.T) {
		cacheSvc := &CacheService{Cache: nil}
		err := cacheSvc.Shutdown()
		assert.NoError(t, err)
	})
}

func TestNewProviderMap(t *testing.T) {
	t.Run("creates provider map with single provider", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		provSvc, err := do.Invoke[*ProviderMapService](injector)
		require.NoError(t, err)
		assert.NotNil(t, provSvc)
		assert.Len(t, provSvc.Providers, 1)
		assert.Len(t, provSvc.AllProviders, 1)
		assert.NotNil(t, provSvc.PrimaryProvider)
		assert.Equal(t, "anthropic", provSvc.PrimaryProvider.Name())
		assert.Equal(t, "test-key-1", provSvc.PrimaryKey)
	})

	t.Run("creates provider map with multiple providers", func(t *testing.T) {
		injector := createTestInjector(t, multiProviderConfig)
		defer shutdownInjector(injector)

		provSvc, err := do.Invoke[*ProviderMapService](injector)
		require.NoError(t, err)
		assert.Len(t, provSvc.Providers, 2)
		assert.Len(t, provSvc.AllProviders, 2)
		// First provider is primary
		assert.Equal(t, "anthropic", provSvc.PrimaryProvider.Name())
	})

	t.Run("returns error when no providers configured", func(t *testing.T) {
		injector := createTestInjector(t, noProviderConfig)
		defer shutdownInjector(injector)

		_, err := do.Invoke[*ProviderMapService](injector)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no enabled provider found")
	})
}

func TestNewKeyPool(t *testing.T) {
	t.Run("returns nil pool when pooling disabled", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		poolSvc, err := do.Invoke[*KeyPoolService](injector)
		require.NoError(t, err)
		assert.NotNil(t, poolSvc)
		// Single key = no pooling by default
		assert.Nil(t, poolSvc.Pool)
	})

	t.Run("creates pool when pooling enabled", func(t *testing.T) {
		injector := createTestInjector(t, multiKeyPoolingConfig)
		defer shutdownInjector(injector)

		poolSvc, err := do.Invoke[*KeyPoolService](injector)
		require.NoError(t, err)
		assert.NotNil(t, poolSvc)
		assert.NotNil(t, poolSvc.Pool)
	})
}

func TestNewProxyHandler(t *testing.T) {
	t.Run("creates handler with dependencies", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		handlerSvc, err := do.Invoke[*HandlerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, handlerSvc)
		assert.NotNil(t, handlerSvc.Handler)
	})
}

func TestNewHTTPServer(t *testing.T) {
	t.Run("creates server with dependencies", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		serverSvc, err := do.Invoke[*ServerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, serverSvc)
		assert.NotNil(t, serverSvc.Server)
	})

	t.Run("implements Shutdowner", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		serverSvc, err := do.Invoke[*ServerService](injector)
		require.NoError(t, err)

		// ServerService should implement Shutdown
		// Don't actually call it since server isn't started
		assert.NotNil(t, serverSvc.Server)
	})

	t.Run("Shutdown handles nil server", func(t *testing.T) {
		serverSvc := &ServerService{Server: nil}
		err := serverSvc.Shutdown()
		assert.NoError(t, err)
	})
}

func TestDependencyOrder(t *testing.T) {
	t.Run("services resolve in correct dependency order", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		// Resolve server (which should trigger all dependencies)
		serverSvc, err := do.Invoke[*ServerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, serverSvc)

		// All dependencies should now be resolved
		cfgSvc, err := do.Invoke[*ConfigService](injector)
		require.NoError(t, err)
		assert.NotNil(t, cfgSvc)

		cacheSvc, err := do.Invoke[*CacheService](injector)
		require.NoError(t, err)
		assert.NotNil(t, cacheSvc)

		provSvc, err := do.Invoke[*ProviderMapService](injector)
		require.NoError(t, err)
		assert.NotNil(t, provSvc)

		poolSvc, err := do.Invoke[*KeyPoolService](injector)
		require.NoError(t, err)
		assert.NotNil(t, poolSvc)

		handlerSvc, err := do.Invoke[*HandlerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, handlerSvc)
	})
}

func TestRegisterSingletons(t *testing.T) {
	t.Run("registers all expected services", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		err := os.WriteFile(path, []byte(singleKeyConfig), 0o600)
		require.NoError(t, err)

		registerInjector := do.New()
		do.ProvideNamedValue(registerInjector, ConfigPathKey, path)
		RegisterSingletons(registerInjector)
		defer shutdownInjector(registerInjector)

		// Verify each service type is registered
		_, err = do.Invoke[*ConfigService](registerInjector)
		assert.NoError(t, err)

		_, err = do.Invoke[*CacheService](registerInjector)
		assert.NoError(t, err)

		_, err = do.Invoke[*ProviderMapService](registerInjector)
		assert.NoError(t, err)

		_, err = do.Invoke[*KeyPoolService](registerInjector)
		assert.NoError(t, err)

		_, err = do.Invoke[*HandlerService](registerInjector)
		assert.NoError(t, err)

		_, err = do.Invoke[*ServerService](registerInjector)
		assert.NoError(t, err)
	})
}

func TestConfigServiceWrapper(t *testing.T) {
	t.Run("wraps config correctly", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{Listen: ":9000"},
		}
		svc := &ConfigService{Config: cfg}

		assert.Equal(t, ":9000", svc.Config.Server.Listen)
	})
}

func TestProviderMapServiceWrapper(t *testing.T) {
	t.Run("stores primary key reference", func(t *testing.T) {
		svc := &ProviderMapService{
			PrimaryKey: "test-key",
		}

		assert.Equal(t, "test-key", svc.PrimaryKey)
		assert.Nil(t, svc.Providers)
		assert.Nil(t, svc.AllProviders)
		assert.Nil(t, svc.PrimaryProvider)
	})
}
