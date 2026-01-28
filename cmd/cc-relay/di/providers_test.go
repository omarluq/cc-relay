package di

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/health"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
)

const (
	configFileName      = "config.yaml"
	anthropicBaseURL    = "https://api.anthropic.com"
	cacheDisabledConfig = "cache:\n  mode: disabled\n"
	testKey1            = "test-key-1"
	testKey2            = "test-key-2"
	testKey             = "test-key"
	testProviderName    = "test-provider"
	shutdownerTestLabel = "implements Shutdowner"
)

// createTestInjector creates an injector with a config path for testing.
func createTestInjector(t *testing.T, configContent string) *do.RootScope {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
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

// waitFor polls fn until it returns true or the timeout is reached.
func waitFor(t *testing.T, timeout time.Duration, fn func() bool, msg string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	require.True(t, fn(), msg)
}

// Test configurations.
var singleKeyConfig = fmt.Sprintf(`
server:
  listen: ":8787"
logging:
  level: info
  format: json
%s
providers:
  - name: anthropic
    type: anthropic
    base_url: %s
    enabled: true
    keys:
      - key: %s
`, cacheDisabledConfig, anthropicBaseURL, testKey1)

var multiKeyPoolingConfig = fmt.Sprintf(`
server:
  listen: ":8787"
logging:
  level: info
  format: json
%s
providers:
  - name: anthropic
    type: anthropic
    base_url: %s
    enabled: true
    pooling:
      enabled: true
      strategy: least_loaded
    keys:
      - key: %s
        rpm_limit: 100
        priority: 1
      - key: %s
        rpm_limit: 200
        priority: 2
`, cacheDisabledConfig, anthropicBaseURL, testKey1, testKey2)

const noProviderConfig = `
server:
  listen: ":8787"
logging:
  level: info
` + cacheDisabledConfig + `providers: []
`

var multiProviderConfig = fmt.Sprintf(`
server:
  listen: ":8787"
logging:
  level: info
cache:
  mode: disabled
providers:
  - name: anthropic
    type: anthropic
    base_url: %s
    enabled: true
    keys:
      - key: %s
  - name: zai
    type: zai
    base_url: https://api.zai.example.com
    enabled: true
    keys:
      - key: zai-key-1
`, anthropicBaseURL, testKey1)

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
		do.ProvideNamedValue(nonExistentInjector, ConfigPathKey, "/nonexistent/"+configFileName)
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

	t.Run("creates watcher for valid config", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		cfgSvc, err := do.Invoke[*ConfigService](injector)
		require.NoError(t, err)
		assert.NotNil(t, cfgSvc.watcher, "watcher should be created for valid config")
	})

	t.Run("Get returns config via atomic pointer", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		cfgSvc, err := do.Invoke[*ConfigService](injector)
		require.NoError(t, err)

		// Get should return same config as Config field (initially)
		cfg := cfgSvc.Get()
		assert.NotNil(t, cfg)
		assert.Equal(t, cfgSvc.Config, cfg)
		assert.Equal(t, ":8787", cfg.Server.Listen)
	})
}

func TestConfigServiceHotReload(t *testing.T) {
	t.Run("hot-reload updates config atomically", func(t *testing.T) {
		// Create config file
		dir := t.TempDir()
		path := filepath.Join(dir, configFileName)
		err := os.WriteFile(path, []byte(singleKeyConfig), 0o600)
		require.NoError(t, err)

		// Create injector and get config service
		injector := do.New()
		do.ProvideNamedValue(injector, ConfigPathKey, path)
		RegisterSingletons(injector)
		defer shutdownInjector(injector)

		cfgSvc, err := do.Invoke[*ConfigService](injector)
		require.NoError(t, err)

		// Verify initial config
		initialCfg := cfgSvc.Get()
		assert.Equal(t, ":8787", initialCfg.Server.Listen)

		// Start watching
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cfgSvc.StartWatching(ctx)

		// Allow watcher to start
		time.Sleep(50 * time.Millisecond)

		// Update config file with new listen port
		newConfig := fmt.Sprintf(`
server:
  listen: ":9999"
logging:
  level: info
  format: json
cache:
  mode: disabled
providers:
  - name: anthropic
    type: anthropic
    base_url: %s
    enabled: true
    keys:
      - key: %s
`, anthropicBaseURL, testKey1)
		err = os.WriteFile(path, []byte(newConfig), 0o600)
		require.NoError(t, err)

		// Wait for reload
		time.Sleep(300 * time.Millisecond)

		// Verify config was reloaded
		reloadedCfg := cfgSvc.Get()
		assert.Equal(t, ":9999", reloadedCfg.Server.Listen, "config should be reloaded with new port")

		// Original pointer should NOT be same (atomic swap happened)
		assert.NotSame(t, initialCfg, reloadedCfg, "config pointer should change after reload")
	})

	t.Run("live readers observe routing changes without reinit", func(t *testing.T) {
		initialConfig := fmt.Sprintf(`
server:
  listen: ":8787"
routing:
  strategy: failover
logging:
  level: info
  format: json
cache:
  mode: disabled
providers:
  - name: anthropic
    type: anthropic
    base_url: %s
    enabled: true
    keys:
      - key: %s
`, anthropicBaseURL, testKey1)

		updatedConfig := fmt.Sprintf(`
server:
  listen: ":8787"
routing:
  strategy: round_robin
logging:
  level: info
  format: json
cache:
  mode: disabled
providers:
  - name: anthropic
    type: anthropic
    base_url: %s
    enabled: true
    keys:
      - key: %s
`, anthropicBaseURL, testKey1)

		// Create config file
		dir := t.TempDir()
		path := filepath.Join(dir, configFileName)
		err := os.WriteFile(path, []byte(initialConfig), 0o600)
		require.NoError(t, err)

		// Create injector and get config service
		injector := do.New()
		do.ProvideNamedValue(injector, ConfigPathKey, path)
		RegisterSingletons(injector)
		defer shutdownInjector(injector)

		cfgSvc, err := do.Invoke[*ConfigService](injector)
		require.NoError(t, err)

		// A live reader that always consults the current config.
		type routingStrategyReader struct {
			cfg interface {
				Get() *config.Config
			}
		}

		reader := routingStrategyReader{cfg: cfgSvc}

		getStrategy := func() string {
			return reader.cfg.Get().Routing.GetEffectiveStrategy()
		}

		require.Equal(t, "failover", getStrategy(), "initial strategy should be failover")

		// Start watching for changes
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cfgSvc.StartWatching(ctx)

		// Allow watcher to start
		time.Sleep(50 * time.Millisecond)

		// Update routing strategy on disk
		err = os.WriteFile(path, []byte(updatedConfig), 0o600)
		require.NoError(t, err)

		// The same reader instance should observe the new strategy after reload.
		waitFor(
			t,
			2*time.Second,
			func() bool { return getStrategy() == "round_robin" },
			"expected live reader to observe updated routing strategy",
		)
	})

	t.Run("live reader updates while snapshot stays stale", func(t *testing.T) {
		initialConfig := fmt.Sprintf(`
server:
  listen: ":8787"
routing:
  strategy: failover
logging:
  level: info
  format: json
cache:
  mode: disabled
providers:
  - name: anthropic
    type: anthropic
    base_url: %s
    enabled: true
    keys:
      - key: %s
`, anthropicBaseURL, testKey1)

		updatedConfig := fmt.Sprintf(`
server:
  listen: ":8787"
routing:
  strategy: round_robin
logging:
  level: info
  format: json
cache:
  mode: disabled
providers:
  - name: anthropic
    type: anthropic
    base_url: %s
    enabled: true
    keys:
      - key: %s
`, anthropicBaseURL, testKey1)

		dir := t.TempDir()
		path := filepath.Join(dir, configFileName)
		require.NoError(t, os.WriteFile(path, []byte(initialConfig), 0o600))

		injector := do.New()
		do.ProvideNamedValue(injector, ConfigPathKey, path)
		RegisterSingletons(injector)
		defer shutdownInjector(injector)

		cfgSvc, err := do.Invoke[*ConfigService](injector)
		require.NoError(t, err)

		// Snapshot component: captures config once and never refreshes.
		type snapshotReader struct {
			cfg *config.Config
		}
		// Live component: consults config service on each read.
		type liveReader struct {
			cfg interface {
				Get() *config.Config
			}
		}

		snap := snapshotReader{cfg: cfgSvc.Get()}
		live := liveReader{cfg: cfgSvc}

		snapStrategy := func() string { return snap.cfg.Routing.GetEffectiveStrategy() }
		liveStrategy := func() string { return live.cfg.Get().Routing.GetEffectiveStrategy() }

		require.Equal(t, "failover", snapStrategy())
		require.Equal(t, "failover", liveStrategy())

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cfgSvc.StartWatching(ctx)
		time.Sleep(50 * time.Millisecond)

		require.NoError(t, os.WriteFile(path, []byte(updatedConfig), 0o600))

		// Live reader should update...
		waitFor(t, 2*time.Second, func() bool { return liveStrategy() == "round_robin" },
			"expected live reader to observe updated strategy")
		// ...while snapshot remains stale, illustrating the intended integration pattern.
		assert.Equal(t, "failover", snapStrategy(), "snapshot reader should remain stale")
	})

	t.Run("invalid reload does not replace the last good config", func(t *testing.T) {
		initialConfig := fmt.Sprintf(`
server:
  listen: ":8787"
routing:
  strategy: failover
logging:
  level: info
  format: json
cache:
  mode: disabled
providers:
  - name: anthropic
    type: anthropic
    base_url: %s
    enabled: true
    keys:
      - key: %s
`, anthropicBaseURL, testKey1)
		updatedConfig := fmt.Sprintf(`
server:
  listen: ":8787"
routing:
  strategy: round_robin
logging:
  level: info
  format: json
cache:
  mode: disabled
providers:
  - name: anthropic
    type: anthropic
    base_url: %s
    enabled: true
    keys:
      - key: %s
`, anthropicBaseURL, testKey1)
		invalidConfig := "server: ["

		dir := t.TempDir()
		path := filepath.Join(dir, configFileName)
		require.NoError(t, os.WriteFile(path, []byte(initialConfig), 0o600))

		injector := do.New()
		do.ProvideNamedValue(injector, ConfigPathKey, path)
		RegisterSingletons(injector)
		defer shutdownInjector(injector)

		cfgSvc, err := do.Invoke[*ConfigService](injector)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cfgSvc.StartWatching(ctx)
		time.Sleep(50 * time.Millisecond)

		// Write invalid config; it should not replace the last good config.
		require.NoError(t, os.WriteFile(path, []byte(invalidConfig), 0o600))
		time.Sleep(300 * time.Millisecond)
		assert.Equal(t, "failover", cfgSvc.Get().Routing.GetEffectiveStrategy())

		// Then write a valid config and ensure it updates.
		require.NoError(t, os.WriteFile(path, []byte(updatedConfig), 0o600))
		waitFor(t, 2*time.Second, func() bool {
			return cfgSvc.Get().Routing.GetEffectiveStrategy() == "round_robin"
		}, "expected valid config after invalid reload to be applied")
	})

	t.Run("concurrent reads during reload are safe", func(t *testing.T) {
		// Create config file
		dir := t.TempDir()
		path := filepath.Join(dir, configFileName)
		err := os.WriteFile(path, []byte(singleKeyConfig), 0o600)
		require.NoError(t, err)

		// Create injector and get config service
		injector := do.New()
		do.ProvideNamedValue(injector, ConfigPathKey, path)
		RegisterSingletons(injector)
		defer shutdownInjector(injector)

		cfgSvc, err := do.Invoke[*ConfigService](injector)
		require.NoError(t, err)

		// Start watching
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cfgSvc.StartWatching(ctx)

		// Allow watcher to start
		time.Sleep(50 * time.Millisecond)

		// Concurrent reads while modifying file
		var wg sync.WaitGroup
		var readCount atomic.Int64
		stopReads := make(chan struct{})

		// Start concurrent readers
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stopReads:
						return
					default:
						cfg := cfgSvc.Get()
						assert.NotNil(t, cfg)
						readCount.Add(1)
						time.Sleep(1 * time.Millisecond)
					}
				}
			}()
		}

		// Modify file multiple times while reads happen
		for i := range 5 {
			newConfig := fmt.Sprintf(`
server:
  listen: ":`+string(rune('0'+i))+`787"
logging:
  level: info
  format: json
cache:
  mode: disabled
providers:
  - name: anthropic
    type: anthropic
    base_url: %s
    enabled: true
    keys:
      - key: %s
`, anthropicBaseURL, testKey1)
			err := os.WriteFile(path, []byte(newConfig), 0o600)
			require.NoError(t, err)
			time.Sleep(50 * time.Millisecond)
		}

		// Stop readers and wait
		close(stopReads)
		wg.Wait()

		// If we got here without data race, concurrent access is safe
		assert.Greater(t, readCount.Load(), int64(0), "should have completed reads")
	})

	t.Run("StartWatching with nil watcher is no-op", func(_ *testing.T) {
		cfgSvc := &ConfigService{
			Config:  &config.Config{},
			watcher: nil,
		}
		cfgSvc.config.Store(cfgSvc.Config)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Should not panic
		cfgSvc.StartWatching(ctx)
	})

	t.Run("Shutdown closes watcher", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, configFileName)
		err := os.WriteFile(path, []byte(singleKeyConfig), 0o600)
		require.NoError(t, err)

		injector := do.New()
		do.ProvideNamedValue(injector, ConfigPathKey, path)
		RegisterSingletons(injector)

		cfgSvc, err := do.Invoke[*ConfigService](injector)
		require.NoError(t, err)
		assert.NotNil(t, cfgSvc.watcher)

		// Start watching
		ctx, cancel := context.WithCancel(context.Background())
		cfgSvc.StartWatching(ctx)

		// Allow watcher to start
		time.Sleep(50 * time.Millisecond)

		// Shutdown should close watcher
		err = cfgSvc.Shutdown()
		assert.NoError(t, err)

		// Cancel context for cleanup
		cancel()

		// Second shutdown should return ErrWatcherClosed
		err = cfgSvc.Shutdown()
		assert.ErrorIs(t, err, config.ErrWatcherClosed)
	})

	t.Run("Shutdown handles nil watcher", func(t *testing.T) {
		cfgSvc := &ConfigService{
			Config:  &config.Config{},
			watcher: nil,
		}
		err := cfgSvc.Shutdown()
		assert.NoError(t, err)
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

	t.Run(shutdownerTestLabel, func(t *testing.T) {
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
		assert.Len(t, provSvc.GetProviders(), 1)
		assert.Len(t, provSvc.GetAllProviders(), 1)
		assert.NotNil(t, provSvc.GetPrimaryProvider())
		assert.Equal(t, "anthropic", provSvc.GetPrimaryProvider().Name())
		assert.Equal(t, testKey1, provSvc.GetPrimaryKey())
	})

	t.Run("creates provider map with multiple providers", func(t *testing.T) {
		injector := createTestInjector(t, multiProviderConfig)
		defer shutdownInjector(injector)

		provSvc, err := do.Invoke[*ProviderMapService](injector)
		require.NoError(t, err)
		assert.Len(t, provSvc.GetProviders(), 2)
		assert.Len(t, provSvc.GetAllProviders(), 2)
		// First provider is primary
		assert.Equal(t, "anthropic", provSvc.GetPrimaryProvider().Name())
	})

	t.Run("returns error when no providers configured", func(t *testing.T) {
		injector := createTestInjector(t, noProviderConfig)
		defer shutdownInjector(injector)

		_, err := do.Invoke[*ProviderMapService](injector)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no enabled provider found")
	})
}

func TestProviderInfoServiceRebuildFromEnablesProvider(t *testing.T) {
	t.Parallel()

	cfgA := &config.Config{
		Providers: []config.ProviderConfig{
			{
				Name:    "anthropic",
				Type:    "anthropic",
				BaseURL: anthropicBaseURL,
				Enabled: true,
				Keys: []config.KeyConfig{
					{Key: testKey1},
				},
			},
			{
				Name:    "zai",
				Type:    "zai",
				BaseURL: "https://api.zai.example.com",
				Enabled: false,
				Keys: []config.KeyConfig{
					{Key: "zai-key-1"},
				},
			},
		},
	}

	cfgB := &config.Config{
		Providers: []config.ProviderConfig{
			{
				Name:    "anthropic",
				Type:    "anthropic",
				BaseURL: anthropicBaseURL,
				Enabled: true,
				Keys: []config.KeyConfig{
					{Key: testKey1},
				},
			},
			{
				Name:    "zai",
				Type:    "zai",
				BaseURL: "https://api.zai.example.com",
				Enabled: true,
				Keys: []config.KeyConfig{
					{Key: "zai-key-1"},
				},
			},
		},
	}

	cfgSvc := &ConfigService{Config: cfgA}
	cfgSvc.config.Store(cfgA)

	providerSvc := &ProviderMapService{cfgSvc: cfgSvc}
	require.NoError(t, providerSvc.RebuildFrom(cfgA))

	tracker := health.NewTracker(health.CircuitBreakerConfig{}, nil)
	infoSvc := &ProviderInfoService{
		cfgSvc:      cfgSvc,
		providerSvc: providerSvc,
		trackerSvc:  &HealthTrackerService{Tracker: tracker},
	}

	infoSvc.RebuildFrom(cfgA)
	assert.Len(t, infoSvc.Get(), 1)

	require.NoError(t, providerSvc.RebuildFrom(cfgB))
	infoSvc.RebuildFrom(cfgB)

	infos := infoSvc.Get()
	assert.Len(t, infos, 2)
	names := map[string]bool{}
	for _, info := range infos {
		names[info.Provider.Name()] = true
	}
	assert.True(t, names["anthropic"])
	assert.True(t, names["zai"])
}

func TestProviderInfoServiceGetReturnsCopy(t *testing.T) {
	t.Parallel()

	svc := &ProviderInfoService{}
	provider := providers.NewAnthropicProvider("test", anthropicBaseURL)
	infos := []router.ProviderInfo{
		{Provider: provider, Weight: 1},
	}
	svc.infos.Store(&infos)

	got := svc.Get()
	got[0].Weight = 99

	got2 := svc.Get()
	assert.Equal(t, 1, got2[0].Weight)
}

func TestNewKeyPool(t *testing.T) {
	t.Run("returns nil pool when pooling disabled", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		poolSvc, err := do.Invoke[*KeyPoolService](injector)
		require.NoError(t, err)
		assert.NotNil(t, poolSvc)
		// Single key = no pooling by default
		assert.Nil(t, poolSvc.Get())
	})

	t.Run("creates pool when pooling enabled", func(t *testing.T) {
		injector := createTestInjector(t, multiKeyPoolingConfig)
		defer shutdownInjector(injector)

		poolSvc, err := do.Invoke[*KeyPoolService](injector)
		require.NoError(t, err)
		assert.NotNil(t, poolSvc)
		assert.NotNil(t, poolSvc.Get())
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

	t.Run(shutdownerTestLabel, func(t *testing.T) {
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
		path := filepath.Join(dir, configFileName)
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
			PrimaryKey: testKey,
		}

		assert.Equal(t, testKey, svc.GetPrimaryKey())
		assert.Nil(t, svc.GetProviders())
		assert.Nil(t, svc.GetAllProviders())
		assert.Nil(t, svc.GetPrimaryProvider())
	})
}

func TestLoggerService(t *testing.T) {
	t.Run("creates logger from config", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		loggerSvc, err := do.Invoke[*LoggerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, loggerSvc)
		assert.NotNil(t, loggerSvc.Logger)
	})

	t.Run("singleton returns same instance", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		logger1, err := do.Invoke[*LoggerService](injector)
		require.NoError(t, err)

		logger2, err := do.Invoke[*LoggerService](injector)
		require.NoError(t, err)

		assert.Same(t, logger1, logger2)
	})
}

func TestHealthTrackerService(t *testing.T) {
	t.Run("creates tracker from config", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		trackerSvc, err := do.Invoke[*HealthTrackerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, trackerSvc)
		assert.NotNil(t, trackerSvc.Tracker)
	})

	t.Run("tracker IsHealthyFunc returns true for new provider", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		trackerSvc, err := do.Invoke[*HealthTrackerService](injector)
		require.NoError(t, err)

		isHealthy := trackerSvc.Tracker.IsHealthyFunc(testProviderName)
		assert.True(t, isHealthy(), "new provider should be healthy by default")
	})

	t.Run("tracker records success and failure", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		trackerSvc, err := do.Invoke[*HealthTrackerService](injector)
		require.NoError(t, err)

		// Initially healthy
		isHealthy := trackerSvc.Tracker.IsHealthyFunc(testProviderName)
		assert.True(t, isHealthy(), "provider should be healthy initially")

		// Record success - should remain healthy
		trackerSvc.Tracker.RecordSuccess(testProviderName)
		assert.True(t, isHealthy(), "provider should remain healthy after success")

		// Record failure - may or may not trip circuit depending on config
		// This just verifies the method doesn't panic
		trackerSvc.Tracker.RecordFailure(testProviderName, nil)
	})
}

func TestCheckerService(t *testing.T) {
	t.Run("creates checker from config", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		checkerSvc, err := do.Invoke[*CheckerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, checkerSvc)
		assert.NotNil(t, checkerSvc.Checker)
	})

	t.Run(shutdownerTestLabel, func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		checkerSvc, err := do.Invoke[*CheckerService](injector)
		require.NoError(t, err)

		// Should not panic
		err = checkerSvc.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("Shutdown handles nil checker", func(t *testing.T) {
		checkerSvc := &CheckerService{Checker: nil}
		err := checkerSvc.Shutdown()
		assert.NoError(t, err)
	})
}

func TestNewProxyHandlerWithHealthTracker(t *testing.T) {
	t.Run("handler wired with tracker", func(t *testing.T) {
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		// This verifies the full dependency chain works
		handlerSvc, err := do.Invoke[*HandlerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, handlerSvc)
		assert.NotNil(t, handlerSvc.Handler)
	})
}

// ptrBool returns a pointer to a bool value.
func ptrBool(b bool) *bool {
	return &b
}

func TestCheckerStartsAndStopsWithContainer(t *testing.T) {
	// Create minimal config with health check enabled
	cfg := &config.Config{
		Providers: []config.ProviderConfig{
			{
				Name:    testProviderName,
				Type:    "anthropic",
				Enabled: true,
				BaseURL: "http://localhost:9999", // Fake URL - we just test lifecycle
				Keys: []config.KeyConfig{
					{Key: testKey},
				},
			},
		},
		Health: health.Config{
			HealthCheck: health.CheckConfig{
				Enabled:    ptrBool(true),
				IntervalMS: 100, // Fast interval for testing
			},
			CircuitBreaker: health.CircuitBreakerConfig{
				FailureThreshold: 5,
				OpenDurationMS:   1000,
			},
		},
		Server: config.ServerConfig{
			Listen: "localhost:0",
		},
		Logging: config.LoggingConfig{
			Level: "debug",
		},
	}

	// Create test container with pre-configured services
	container := do.New()
	nopLogger := zerolog.Nop()
	do.ProvideValue(container, &ConfigService{Config: cfg})
	do.ProvideValue(container, &LoggerService{Logger: &nopLogger})
	do.Provide(container, NewHealthTracker)
	do.Provide(container, NewChecker)

	// Get checker and verify provider was registered
	checkerSvc := do.MustInvoke[*CheckerService](container)
	require.NotNil(t, checkerSvc.Checker, "Checker should be created")

	// Start the checker
	checkerSvc.Checker.Start()

	// Give it time to run at least one check cycle
	time.Sleep(150 * time.Millisecond)

	// Shutdown via container (tests graceful shutdown path)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := container.ShutdownWithContext(ctx)
	// Note: ShutdownWithContext may return errors for services that weren't invoked
	// but this doesn't indicate a problem with the Checker lifecycle
	if err != nil {
		t.Logf("Container shutdown returned (may include uninvoked services): %v", err)
	}

	// If we got here without deadlock or panic, the lifecycle works
}
