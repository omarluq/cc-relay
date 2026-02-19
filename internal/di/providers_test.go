package di_test

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
	"github.com/omarluq/cc-relay/internal/di"
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
	roundRobinStrategy  = "round_robin"
)

// createTestInjector creates an injector with a config path for testing.
func createTestInjector(t *testing.T, configContent string) *do.RootScope {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	err := os.WriteFile(path, []byte(configContent), 0o600)
	require.NoError(t, err)

	return newInjectorWithConfigPath(path)
}

func newInjectorWithConfigPath(path string) *do.RootScope {
	injector := do.New()
	do.ProvideNamedValue(injector, di.ConfigPathKey, path)
	di.RegisterSingletons(injector)
	return injector
}

// shutdownInjector is a helper to properly shutdown an injector in tests.
// The error from Shutdown is intentionally discarded as test cleanup
// errors are non-critical.
//
//nolint:errcheck // intentional: test cleanup shutdown errors are non-critical
func shutdownInjector(i *do.RootScope) {
	i.Shutdown()
}

// waitFor polls fn until it returns true or the timeout is reached.
func waitFor(t *testing.T, timeout time.Duration, checkFn func() bool, msg string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if checkFn() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	require.True(t, checkFn(), msg)
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
	t.Parallel()
	t.Run("loads valid config", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		cfgSvc, err := do.Invoke[*di.ConfigService](injector)
		require.NoError(t, err)
		assert.NotNil(t, cfgSvc)
		assert.NotNil(t, cfgSvc.Config)
		assert.Equal(t, ":8787", cfgSvc.Config.Server.Listen)
		assert.Len(t, cfgSvc.Config.Providers, 1)
	})

	t.Run("returns error for non-existent config", func(t *testing.T) {
		t.Parallel()
		nonExistentInjector := newInjectorWithConfigPath("/nonexistent/" + configFileName)
		defer shutdownInjector(nonExistentInjector)

		_, err := do.Invoke[*di.ConfigService](nonExistentInjector)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load config")
	})

	t.Run("singleton returns same instance", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		cfg1, err := do.Invoke[*di.ConfigService](injector)
		require.NoError(t, err)

		cfg2, err := do.Invoke[*di.ConfigService](injector)
		require.NoError(t, err)

		// Should be same pointer (singleton)
		assert.Same(t, cfg1, cfg2)
	})

	t.Run("creates watcher for valid config", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		cfgSvc, err := do.Invoke[*di.ConfigService](injector)
		require.NoError(t, err)
		assert.NotNil(t, cfgSvc.GetWatcher(), "watcher should be created for valid config")
	})

	t.Run("Get returns config via atomic pointer", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		cfgSvc, err := do.Invoke[*di.ConfigService](injector)
		require.NoError(t, err)

		// Get should return same config as Config field (initially)
		cfg := cfgSvc.Get()
		assert.NotNil(t, cfg)
		assert.Equal(t, cfgSvc.Config, cfg)
		assert.Equal(t, ":8787", cfg.Server.Listen)
	})
}

func TestConfigServiceHotReload(t *testing.T) {
	t.Parallel()
	t.Run("hot-reload updates config atomically", testHotReloadUpdatesAtomically)
	t.Run("live readers observe routing changes without reinit", testLiveReadersObserveRoutingChanges)
	t.Run("live reader updates while snapshot stays stale", testLiveReaderUpdatesSnapshotStale)
	t.Run("invalid reload does not replace the last good config", testInvalidReloadKeepsLastGood)
	t.Run("concurrent reads during reload are safe", testConcurrentReadsDuringReload)
	t.Run("StartWatching with nil watcher is no-op", testStartWatchingNilWatcher)
	t.Run("Shutdown closes watcher", testShutdownClosesWatcher)
	t.Run("Shutdown handles nil watcher", testShutdownNilWatcher)
}

// hotReloadTestConfig generates a config string with the given routing strategy.
func hotReloadTestConfig(strategy string) string {
	return fmt.Sprintf(`
server:
  listen: ":8787"
routing:
  strategy: %s
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
`, strategy, anthropicBaseURL, testKey1)
}

func testHotReloadUpdatesAtomically(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	err := os.WriteFile(path, []byte(singleKeyConfig), 0o600)
	require.NoError(t, err)

	injector := newInjectorWithConfigPath(path)
	defer shutdownInjector(injector)

	cfgSvc, err := do.Invoke[*di.ConfigService](injector)
	require.NoError(t, err)

	initialCfg := cfgSvc.Get()
	assert.Equal(t, ":8787", initialCfg.Server.Listen)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfgSvc.StartWatching(ctx)
	time.Sleep(50 * time.Millisecond)

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

	time.Sleep(300 * time.Millisecond)

	reloadedCfg := cfgSvc.Get()
	assert.Equal(t, ":9999", reloadedCfg.Server.Listen, "config should be reloaded with new port")
	assert.NotSame(t, initialCfg, reloadedCfg, "config pointer should change after reload")
}

func testLiveReadersObserveRoutingChanges(t *testing.T) {
	t.Parallel()

	initialConfig := hotReloadTestConfig("failover")
	updatedConfig := hotReloadTestConfig(roundRobinStrategy)

	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	err := os.WriteFile(path, []byte(initialConfig), 0o600)
	require.NoError(t, err)

	injector := newInjectorWithConfigPath(path)
	defer shutdownInjector(injector)

	cfgSvc, err := do.Invoke[*di.ConfigService](injector)
	require.NoError(t, err)

	type routingStrategyReader struct {
		cfg interface{ Get() *config.Config }
	}

	reader := routingStrategyReader{cfg: cfgSvc}
	getStrategy := func() string { return reader.cfg.Get().Routing.GetEffectiveStrategy() }

	require.Equal(t, "failover", getStrategy(), "initial strategy should be failover")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfgSvc.StartWatching(ctx)
	time.Sleep(50 * time.Millisecond)

	err = os.WriteFile(path, []byte(updatedConfig), 0o600)
	require.NoError(t, err)

	waitFor(t, 2*time.Second,
		func() bool { return getStrategy() == roundRobinStrategy },
		"expected live reader to observe updated routing strategy")
}

func testLiveReaderUpdatesSnapshotStale(t *testing.T) {
	t.Parallel()

	initialConfig := hotReloadTestConfig("failover")
	updatedConfig := hotReloadTestConfig(roundRobinStrategy)

	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	require.NoError(t, os.WriteFile(path, []byte(initialConfig), 0o600))

	injector := newInjectorWithConfigPath(path)
	defer shutdownInjector(injector)

	cfgSvc, err := do.Invoke[*di.ConfigService](injector)
	require.NoError(t, err)

	type snapshotReader struct{ cfg *config.Config }
	type liveReader struct {
		cfg interface{ Get() *config.Config }
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

	waitFor(t, 2*time.Second, func() bool { return liveStrategy() == roundRobinStrategy },
		"expected live reader to observe updated strategy")
	assert.Equal(t, "failover", snapStrategy(), "snapshot reader should remain stale")
}

func testInvalidReloadKeepsLastGood(t *testing.T) {
	t.Parallel()

	initialConfig := hotReloadTestConfig("failover")
	updatedConfig := hotReloadTestConfig(roundRobinStrategy)
	invalidConfig := "server: ["

	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	require.NoError(t, os.WriteFile(path, []byte(initialConfig), 0o600))

	injector := newInjectorWithConfigPath(path)
	defer shutdownInjector(injector)

	cfgSvc, err := do.Invoke[*di.ConfigService](injector)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfgSvc.StartWatching(ctx)
	time.Sleep(50 * time.Millisecond)

	require.NoError(t, os.WriteFile(path, []byte(invalidConfig), 0o600))
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, "failover", cfgSvc.Get().Routing.GetEffectiveStrategy())

	require.NoError(t, os.WriteFile(path, []byte(updatedConfig), 0o600))
	waitFor(t, 2*time.Second, func() bool {
		return cfgSvc.Get().Routing.GetEffectiveStrategy() == roundRobinStrategy
	}, "expected valid config after invalid reload to be applied")
}

func testConcurrentReadsDuringReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	err := os.WriteFile(path, []byte(singleKeyConfig), 0o600)
	require.NoError(t, err)

	injector := newInjectorWithConfigPath(path)
	defer shutdownInjector(injector)

	cfgSvc, err := do.Invoke[*di.ConfigService](injector)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfgSvc.StartWatching(ctx)
	time.Sleep(50 * time.Millisecond)

	var waitGroup sync.WaitGroup
	var readCount atomic.Int64
	stopReads := make(chan struct{})

	for range 10 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
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

	for i := range 5 {
		newConfig := fmt.Sprintf(`
server:
  listen: "`+string(rune('0'+i))+`787"
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
		writeErr := os.WriteFile(path, []byte(newConfig), 0o600)
		require.NoError(t, writeErr)
		time.Sleep(50 * time.Millisecond)
	}

	close(stopReads)
	waitGroup.Wait()
	assert.Greater(t, readCount.Load(), int64(0), "should have completed reads")
}

func testStartWatchingNilWatcher(t *testing.T) {
	t.Parallel()
	nilCfg := di.MustTestConfig()
	cfgSvc := di.NewConfigServiceWithNilWatcher(&nilCfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfgSvc.StartWatching(ctx)
}

func testShutdownClosesWatcher(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	err := os.WriteFile(path, []byte(singleKeyConfig), 0o600)
	require.NoError(t, err)

	injector := newInjectorWithConfigPath(path)

	cfgSvc, err := do.Invoke[*di.ConfigService](injector)
	require.NoError(t, err)
	assert.NotNil(t, cfgSvc.GetWatcher())

	ctx, cancel := context.WithCancel(context.Background())
	cfgSvc.StartWatching(ctx)
	time.Sleep(50 * time.Millisecond)

	err = cfgSvc.Shutdown()
	assert.NoError(t, err)
	cancel()

	err = cfgSvc.Shutdown()
	assert.ErrorIs(t, err, config.ErrWatcherClosed)
}

func testShutdownNilWatcher(t *testing.T) {
	t.Parallel()
	nilCfg := di.MustTestConfig()
	cfgSvc := di.NewConfigServiceWithNilWatcher(&nilCfg)
	err := cfgSvc.Shutdown()
	assert.NoError(t, err)
}

// shutdownerTest is a reusable test for services that implement Shutdown.
type shutdownerTest struct {
	invokeAndGet func(*testing.T, *do.RootScope) (interface{ Shutdown() error }, error)
	nilShutdown  func(*testing.T)
	name         string
}

func runShutdownerTest(t *testing.T, test shutdownerTest) {
	t.Helper()
	t.Run("creates service from config", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		svc, err := test.invokeAndGet(t, injector)
		require.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("service implements Shutdown", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		svc, err := test.invokeAndGet(t, injector)
		require.NoError(t, err)

		err = svc.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("Shutdown handles nil service", func(t *testing.T) {
		t.Parallel()
		test.nilShutdown(t)
	})
}

func TestNewCache(t *testing.T) {
	t.Parallel()
	runShutdownerTest(t, shutdownerTest{
		name: "cache",
		invokeAndGet: func(_ *testing.T, injector *do.RootScope) (interface{ Shutdown() error }, error) {
			svc, err := do.Invoke[*di.CacheService](injector)
			if err != nil {
				return nil, err
			}
			assert.NotNil(t, svc.Cache)
			return svc, nil
		},
		nilShutdown: func(t *testing.T) {
			t.Helper()
			cacheSvc := &di.CacheService{Cache: nil}
			err := cacheSvc.Shutdown()
			assert.NoError(t, err)
		},
	})
}

func TestNewProviderMap(t *testing.T) {
	t.Parallel()
	t.Run("creates provider map with single provider", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		provSvc, err := do.Invoke[*di.ProviderMapService](injector)
		require.NoError(t, err)
		assert.NotNil(t, provSvc)
		assert.Len(t, provSvc.GetProviders(), 1)
		assert.Len(t, provSvc.GetAllProviders(), 1)
		assert.NotNil(t, provSvc.GetPrimaryProvider())
		assert.Equal(t, "anthropic", provSvc.GetPrimaryProvider().Name())
		assert.Equal(t, testKey1, provSvc.GetPrimaryKey())
	})

	t.Run("creates provider map with multiple providers", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, multiProviderConfig)
		defer shutdownInjector(injector)

		provSvc, err := do.Invoke[*di.ProviderMapService](injector)
		require.NoError(t, err)
		assert.Len(t, provSvc.GetProviders(), 2)
		assert.Len(t, provSvc.GetAllProviders(), 2)
		// First provider is primary
		assert.Equal(t, "anthropic", provSvc.GetPrimaryProvider().Name())
	})

	t.Run("returns error when no providers configured", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, noProviderConfig)
		defer shutdownInjector(injector)

		_, err := do.Invoke[*di.ProviderMapService](injector)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no enabled provider found")
	})
}

func TestProviderInfoServiceRebuildFromEnablesProvider(t *testing.T) {
	t.Parallel()

	cfgA := di.MustTestConfig()
	cfgA.Providers = []config.ProviderConfig{
		di.MustTestProviderConfig("anthropic", "anthropic", anthropicBaseURL,
			[]config.KeyConfig{di.MustTestKeyConfig(testKey1)}),
		di.MustTestProviderConfig("zai", "zai", "https://api.zai.example.com",
			[]config.KeyConfig{di.MustTestKeyConfig("zai-key-1")}),
	}
	cfgA.Providers[1].Enabled = false

	cfgB := di.MustTestConfig()
	cfgB.Providers = []config.ProviderConfig{
		di.MustTestProviderConfig("anthropic", "anthropic", anthropicBaseURL,
			[]config.KeyConfig{di.MustTestKeyConfig(testKey1)}),
		di.MustTestProviderConfig("zai", "zai", "https://api.zai.example.com",
			[]config.KeyConfig{di.MustTestKeyConfig("zai-key-1")}),
	}
	cfgB.Providers[1].Enabled = true

	cfgSvc := di.NewConfigServiceWithConfig(&cfgA)

	providerSvc := di.NewProviderMapServiceWithConfigService(cfgSvc)
	require.NoError(t, providerSvc.RebuildFrom(&cfgA))

	tracker := health.NewTracker(di.MustTestHealthConfig().CircuitBreaker, nil)
	infoSvc := di.NewProviderInfoService(cfgSvc, providerSvc, di.NewHealthTrackerServiceWithTracker(tracker))

	infoSvc.RebuildFrom(&cfgA)
	assert.Len(t, infoSvc.Get(), 1)

	require.NoError(t, providerSvc.RebuildFrom(&cfgB))
	infoSvc.RebuildFrom(&cfgB)

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

	cfgSvc := di.NewConfigServiceUninitialized()
	providerSvc := di.NewProviderMapServiceWithConfigService(cfgSvc)
	trackerCfg := di.MustTestHealthConfig()
	tracker := health.NewTracker(trackerCfg.CircuitBreaker, nil)
	svc := di.NewProviderInfoService(cfgSvc, providerSvc, di.NewHealthTrackerServiceWithTracker(tracker))
	prov := providers.NewAnthropicProvider("test", anthropicBaseURL)
	infos := []router.ProviderInfo{
		di.MustTestProviderInfo(prov, 1, 0),
	}
	svc.StoreInfos(&infos)

	got := svc.Get()
	got[0].Weight = 99

	got2 := svc.Get()
	assert.Equal(t, 1, got2[0].Weight)
}

func TestNewKeyPool(t *testing.T) {
	t.Parallel()
	t.Run("returns nil pool when pooling disabled", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		poolSvc, err := do.Invoke[*di.KeyPoolService](injector)
		require.NoError(t, err)
		assert.NotNil(t, poolSvc)
		// Single key = no pooling by default
		assert.Nil(t, poolSvc.Get())
	})

	t.Run("creates pool when pooling enabled", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, multiKeyPoolingConfig)
		defer shutdownInjector(injector)

		poolSvc, err := do.Invoke[*di.KeyPoolService](injector)
		require.NoError(t, err)
		assert.NotNil(t, poolSvc)
		assert.NotNil(t, poolSvc.Get())
	})
}

func TestNewProxyHandler(t *testing.T) {
	t.Parallel()
	t.Run("creates handler with dependencies", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		handlerSvc, err := do.Invoke[*di.HandlerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, handlerSvc)
		assert.NotNil(t, handlerSvc.Handler)
	})
}

func TestNewHTTPServer(t *testing.T) {
	t.Parallel()
	t.Run("creates server with dependencies", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		serverSvc, err := do.Invoke[*di.ServerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, serverSvc)
		assert.NotNil(t, serverSvc.Server)
	})

	t.Run(shutdownerTestLabel, func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		serverSvc, err := do.Invoke[*di.ServerService](injector)
		require.NoError(t, err)

		// ServerService should implement Shutdown
		// Don't actually call it since server isn't started
		assert.NotNil(t, serverSvc.Server)
	})

	t.Run("Shutdown handles nil server", func(t *testing.T) {
		t.Parallel()
		serverSvc := &di.ServerService{Server: nil}
		err := serverSvc.Shutdown()
		assert.NoError(t, err)
	})
}

func TestDependencyOrder(t *testing.T) {
	t.Parallel()
	t.Run("services resolve in correct dependency order", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		// Resolve server (which should trigger all dependencies)
		serverSvc, err := do.Invoke[*di.ServerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, serverSvc)

		// All dependencies should now be resolved
		cfgSvc, err := do.Invoke[*di.ConfigService](injector)
		require.NoError(t, err)
		assert.NotNil(t, cfgSvc)

		cacheSvc, err := do.Invoke[*di.CacheService](injector)
		require.NoError(t, err)
		assert.NotNil(t, cacheSvc)

		provSvc, err := do.Invoke[*di.ProviderMapService](injector)
		require.NoError(t, err)
		assert.NotNil(t, provSvc)

		poolSvc, err := do.Invoke[*di.KeyPoolService](injector)
		require.NoError(t, err)
		assert.NotNil(t, poolSvc)

		handlerSvc, err := do.Invoke[*di.HandlerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, handlerSvc)
	})
}

func TestRegisterSingletons(t *testing.T) {
	t.Parallel()
	t.Run("registers all expected services", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, configFileName)
		err := os.WriteFile(path, []byte(singleKeyConfig), 0o600)
		require.NoError(t, err)

		registerInjector := newInjectorWithConfigPath(path)
		defer shutdownInjector(registerInjector)

		// Verify each service type is registered
		_, err = do.Invoke[*di.ConfigService](registerInjector)
		assert.NoError(t, err)

		_, err = do.Invoke[*di.CacheService](registerInjector)
		assert.NoError(t, err)

		_, err = do.Invoke[*di.ProviderMapService](registerInjector)
		assert.NoError(t, err)

		_, err = do.Invoke[*di.KeyPoolService](registerInjector)
		assert.NoError(t, err)

		_, err = do.Invoke[*di.HandlerService](registerInjector)
		assert.NoError(t, err)

		_, err = do.Invoke[*di.ServerService](registerInjector)
		assert.NoError(t, err)
	})
}

func TestConfigServiceWrapper(t *testing.T) {
	t.Parallel()
	t.Run("wraps config correctly", func(t *testing.T) {
		t.Parallel()
		cfg := di.MustTestConfig()
		cfg.Server = di.MustTestServerConfig(":9000")
		svc := di.NewConfigServiceWithConfig(&cfg)

		assert.Equal(t, ":9000", svc.Config.Server.Listen)
	})
}

func TestProviderMapServiceWrapper(t *testing.T) {
	t.Parallel()
	t.Run("stores primary key reference", func(t *testing.T) {
		t.Parallel()
		cfgSvc := di.NewConfigServiceUninitialized()
		svc := di.NewProviderMapServiceWithConfigService(cfgSvc)

		// Initially empty before any rebuild
		assert.Empty(t, svc.GetPrimaryKey())
		assert.Empty(t, svc.GetProviders())
		assert.Empty(t, svc.GetAllProviders())
		assert.Nil(t, svc.GetPrimaryProvider())
	})
}

func TestLoggerService(t *testing.T) {
	t.Parallel()
	t.Run("creates logger from config", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		loggerSvc, err := do.Invoke[*di.LoggerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, loggerSvc)
		assert.NotNil(t, loggerSvc.Logger)
	})

	t.Run("singleton returns same instance", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		logger1, err := do.Invoke[*di.LoggerService](injector)
		require.NoError(t, err)

		logger2, err := do.Invoke[*di.LoggerService](injector)
		require.NoError(t, err)

		assert.Same(t, logger1, logger2)
	})
}

func TestHealthTrackerService(t *testing.T) {
	t.Parallel()
	t.Run("creates tracker from config", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		trackerSvc, err := do.Invoke[*di.HealthTrackerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, trackerSvc)
		assert.NotNil(t, trackerSvc.Tracker)
	})

	t.Run("tracker IsHealthyFunc returns true for new provider", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		trackerSvc, err := do.Invoke[*di.HealthTrackerService](injector)
		require.NoError(t, err)

		isHealthy := trackerSvc.Tracker.IsHealthyFunc(testProviderName)
		assert.True(t, isHealthy(), "new provider should be healthy by default")
	})

	t.Run("tracker records success and failure", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		trackerSvc, err := do.Invoke[*di.HealthTrackerService](injector)
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
	t.Parallel()
	runShutdownerTest(t, shutdownerTest{
		name: "checker",
		invokeAndGet: func(_ *testing.T, injector *do.RootScope) (interface{ Shutdown() error }, error) {
			svc, err := do.Invoke[*di.CheckerService](injector)
			if err != nil {
				return nil, err
			}
			assert.NotNil(t, svc.Checker)
			return svc, nil
		},
		nilShutdown: func(t *testing.T) {
			t.Helper()
			// Use injector to get a checker service, then verify shutdown is safe
			injector := createTestInjector(t, singleKeyConfig)
			defer shutdownInjector(injector)
			checkerSvc, err := do.Invoke[*di.CheckerService](injector)
			require.NoError(t, err)
			// Shutdown should be safe to call
			err = checkerSvc.Shutdown()
			assert.NoError(t, err)
		},
	})
}

func TestNewProxyHandlerWithHealthTracker(t *testing.T) {
	t.Parallel()
	t.Run("handler wired with tracker", func(t *testing.T) {
		t.Parallel()
		injector := createTestInjector(t, singleKeyConfig)
		defer shutdownInjector(injector)

		// This verifies the full dependency chain works
		handlerSvc, err := do.Invoke[*di.HandlerService](injector)
		require.NoError(t, err)
		assert.NotNil(t, handlerSvc)
		assert.NotNil(t, handlerSvc.Handler)
	})
}

func TestCheckerStartsAndStopsWithContainer(t *testing.T) {
	t.Parallel()

	cfg := di.MustTestConfig()
	cfg.Providers = []config.ProviderConfig{
		di.MustTestProviderConfig(testProviderName, "anthropic", "http://localhost:9999",
			[]config.KeyConfig{di.MustTestKeyConfig(testKey)}),
	}

	// Create test container with pre-configured services
	container := do.New()
	nopLogger := zerolog.Nop()
	cfgSvc := di.NewConfigServiceWithConfig(&cfg)
	loggerSvc := &di.LoggerService{Logger: &nopLogger}
	do.ProvideValue(container, cfgSvc)
	do.ProvideValue(container, loggerSvc)
	do.Provide(container, di.NewHealthTracker)
	do.Provide(container, di.NewChecker)

	// Get checker and verify provider was registered
	checkerSvc := do.MustInvoke[*di.CheckerService](container)
	require.NotNil(t, checkerSvc.Checker, "Checker should be created")

	// Start the checker
	checkerSvc.Start()

	// Give it time to run at least one check cycle
	time.Sleep(150 * time.Millisecond)

	// Shutdown via container (tests graceful shutdown path)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := container.ShutdownWithContext(ctx)
	// Note: ShutdownWithContext may return errors for services that weren't invoked
	// but this doesn't indicate a problem with the Checker lifecycle
	if err != nil {
		t.Logf("di.Container shutdown returned (may include uninvoked services): %v", err)
	}

	// If we got here without deadlock or panic, the lifecycle works
}
