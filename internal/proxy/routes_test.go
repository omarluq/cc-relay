// Package proxy implements the HTTP proxy server for cc-relay.
package proxy_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/proxy"
)

const (
	testAPIKey = "test-key"
)

func TestSetupRoutesWithLiveKeyPoolsRoutingDebugToggles(t *testing.T) {
	t.Parallel()

	backend := proxy.NewBackendServer(t, `{"ok":true}`)

	provider := proxy.NewTestProvider(backend.URL)
	providerInfos := []router.ProviderInfo{
		proxy.TestProviderInfoWithHealth(provider, func() bool { return true }),
	}

	routingDebugOn := proxy.TestRoutingConfig()
	routingDebugOn.Debug = true
	cfgA := proxy.TestConfig("")
	cfgA.Routing = routingDebugOn
	cfgB := proxy.TestConfig("")
	runtimeCfg := config.NewRuntime(cfgA)
	handler := newLiveKeyPoolsHandler(t, runtimeCfg, provider, providerInfos)

	req := proxy.NewMessagesRequestWithHeaders(`{"model":"test","messages":[]}`)
	rec := proxy.ServeRequest(t, handler, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-CC-Relay-Strategy"))

	runtimeCfg.Store(cfgB)

	req2 := proxy.NewMessagesRequestWithHeaders(`{"model":"test","messages":[]}`)
	rec2 := proxy.ServeRequest(t, handler, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Empty(t, rec2.Header().Get("X-CC-Relay-Strategy"))
}

func TestSetupRoutesWithLiveKeyPoolsAuthToggle(t *testing.T) {
	t.Parallel()

	backend := proxy.NewBackendServer(t, `{"ok":true}`)

	provider := proxy.NewTestProvider(backend.URL)
	providerInfos := []router.ProviderInfo{
		proxy.TestProviderInfoWithHealth(provider, func() bool { return true }),
	}

	cfgA := proxy.TestConfig(testAPIKey)
	cfgB := proxy.TestConfig("")

	runtimeCfg := config.NewRuntime(cfgA)
	handler := newLiveKeyPoolsHandler(t, runtimeCfg, provider, providerInfos)

	unauthReq := proxy.NewMessagesRequestWithHeaders("{}")
	unauthRec := proxy.ServeRequest(t, handler, unauthReq)
	assert.Equal(t, http.StatusUnauthorized, unauthRec.Code)

	runtimeCfg.Store(cfgB)

	okReq := proxy.NewMessagesRequestWithHeaders("{}")
	okRec := proxy.ServeRequest(t, handler, okReq)
	assert.Equal(t, http.StatusOK, okRec.Code)
}

type nilRuntimeConfigGetter struct{}

func (nilRuntimeConfigGetter) Get() *config.Config {
	return nil
}

func TestSetupRoutesWithLiveKeyPoolsNilConfigProvider(t *testing.T) {
	t.Parallel()

	provider := proxy.NewTestProvider("http://example.com")
	routerInstance, err := router.NewRouter(router.StrategyRoundRobin, 5*time.Second)
	require.NoError(t, err)

	handler, err := proxy.SetupRoutesWithLiveKeyPools(&proxy.RoutesOptions{
		ConfigProvider:     nilRuntimeConfigGetter{},
		Provider:           provider,
		ProviderInfosFunc:  func() []router.ProviderInfo { return nil },
		ProviderRouter:     routerInstance,
		ProviderKey:        "",
		Pool:               nil,
		GetProviderPools:   nil,
		GetProviderKeys:    nil,
		AllProviders:       []providers.Provider{provider},
		HealthTracker:      nil,
		SignatureCache:     nil,
		ProviderPools:      nil,
		ProviderKeys:       nil,
		GetAllProviders:    nil,
		ConcurrencyLimiter: nil,
		ProviderInfos:      nil,
	})
	require.Error(t, err)
	assert.Nil(t, handler)
}

func newLiveKeyPoolsHandler(
	t *testing.T,
	runtimeCfg config.RuntimeConfigGetter,
	provider providers.Provider,
	providerInfos []router.ProviderInfo,
) http.Handler {
	t.Helper()

	routerInstance, err := router.NewRouter(router.StrategyRoundRobin, 5*time.Second)
	require.NoError(t, err)

	handler, err := proxy.SetupRoutesWithLiveKeyPools(&proxy.RoutesOptions{
		ConfigProvider:     runtimeCfg,
		Provider:           provider,
		ProviderInfosFunc:  func() []router.ProviderInfo { return providerInfos },
		ProviderRouter:     routerInstance,
		ProviderKey:        "",
		Pool:               nil,
		GetProviderPools:   nil,
		GetProviderKeys:    nil,
		AllProviders:       []providers.Provider{provider},
		HealthTracker:      nil,
		SignatureCache:     nil,
		ProviderPools:      nil,
		ProviderKeys:       nil,
		GetAllProviders:    nil,
		ConcurrencyLimiter: nil,
		ProviderInfos:      nil,
	})
	require.NoError(t, err)

	return handler
}
