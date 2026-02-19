// Package proxy implements the HTTP proxy server for cc-relay.
package proxy_test

import (
	"net/http"
	"net/http/httptest"
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
	testAPIKey    = "test-key"
	okBackendBody = `{"status":"ok"}`
)

func newTestConfig(apiKey string) *config.Config {
	return proxy.TestConfig(apiKey)
}

func newTestConfigWithListen(apiKey, listen string) *config.Config {
	cfg := newTestConfig(apiKey)
	cfg.Server.Listen = listen
	return cfg
}

func newAuthConfig(auth config.AuthConfig) *config.Config {
	cfg := proxy.TestConfig("")
	cfg.Server.Auth = auth
	return cfg
}

func newAuthHandler(t *testing.T, backend *httptest.Server, auth config.AuthConfig) http.Handler {
	t.Helper()
	cfg := newAuthConfig(auth)
	provider := proxy.NewTestProvider(backend.URL)
	return setupRoutesHandler(t, cfg, provider)
}

func newOKBackend(t *testing.T) *httptest.Server {
	t.Helper()
	return proxy.NewBackendServer(t, okBackendBody)
}

func TestSetupRoutesCreatesHandler(t *testing.T) {
	t.Parallel()

	cfg := newTestConfigWithListen(testAPIKey, "127.0.0.1:0")
	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestSetupRoutesAuthMiddlewareApplied(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(testAPIKey)
	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Request without API key should return 401
	req := proxy.NewMessagesRequestWithHeaders("{}")
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSetupRoutesAuthMiddlewareWithValidKey(t *testing.T) {
	t.Parallel()

	// Create mock backend server
	backend := newOKBackend(t)

	cfg := newTestConfig(testAPIKey)
	provider := proxy.NewTestProvider(backend.URL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Request with valid API key should pass auth and reach backend
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: proxy.APIKeyHeader, Value: testAPIKey},
	)

	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected auth to pass, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutesNoAuthWhenAPIKeyEmpty(t *testing.T) {
	t.Parallel()

	// Create mock backend server
	backend := proxy.NewBackendServer(t, `{"status":"ok"}`)

	cfg := newTestConfig("") // No auth configured
	provider := proxy.NewTestProvider(backend.URL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Request without API key should NOT return 401 when auth is disabled
	req := proxy.NewMessagesRequestWithHeaders("{}")
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected no auth when APIKey is empty, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutesHealthEndpoint(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(testAPIKey) // Auth enabled
	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Health endpoint should work without auth
	req := httptest.NewRequest("GET", "/health", http.NoBody)
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	expectedBody := `{"status":"ok"}`
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

func TestSetupRoutesHealthEndpointWithAuth(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(testAPIKey)
	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Health endpoint should work even when server has auth enabled
	// (health check should never require auth)
	req := httptest.NewRequest("GET", "/health", http.NoBody)
	// Intentionally NOT setting x-api-key header
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code != http.StatusOK {
		t.Errorf("health endpoint should not require auth, got status %d", rec.Code)
	}
}

func TestSetupRoutesOnlyPOSTToMessages(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig("") // No auth for simpler test
	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// GET to /v1/messages should not be handled
	req := httptest.NewRequest("GET", "/v1/messages", http.NoBody)
	rec := proxy.ServeRequest(t, handler, req)

	// Should return 405 Method Not Allowed (Go 1.22+ router behavior)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %d", rec.Code)
	}
}

func TestSetupRoutesWithLiveKeyPoolsRoutingDebugToggles(t *testing.T) {
	t.Parallel()

	backend := proxy.NewBackendServer(t, `{"ok":true}`)

	provider := proxy.NewTestProvider(backend.URL)
	providerInfos := []router.ProviderInfo{
		proxy.TestProviderInfoWithHealth(provider, func() bool { return true }),
	}

	routingDebugOn := proxy.TestRoutingConfig()
	routingDebugOn.Debug = true
	cfgA := &config.Config{
		Providers: nil,
		Server:    proxy.TestServerConfig(""),
		Routing:   routingDebugOn,
		Logging:   proxy.TestLoggingConfig(),
		Health:    proxy.TestHealthConfig(),
		Cache:     proxy.TestCacheConfig(),
	}
	cfgB := &config.Config{
		Providers: nil,
		Server:    proxy.TestServerConfig(""),
		Routing:   proxy.TestRoutingConfig(),
		Logging:   proxy.TestLoggingConfig(),
		Health:    proxy.TestHealthConfig(),
		Cache:     proxy.TestCacheConfig(),
	}
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

	cfgA := newTestConfig(testAPIKey)
	cfgB := newTestConfig("")

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

func TestSetupRoutesOnlyGETToHealth(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig("")
	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// POST to /health should not be handled
	req := httptest.NewRequest("POST", "/health", http.NoBody)
	rec := proxy.ServeRequest(t, handler, req)

	// Should return 405 Method Not Allowed
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST to /health, got %d", rec.Code)
	}
}

func setupRoutesHandler(t *testing.T, cfg *config.Config, provider providers.Provider) http.Handler {
	t.Helper()

	handler, err := proxy.SetupRoutes(cfg, provider, "backend-key", nil)
	require.NoError(t, err)
	return handler
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

func TestSetupRoutesInvalidProviderBaseURL(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(testAPIKey)

	// Create provider with invalid base URL
	provider := proxy.NewTestProvider("://invalid-url")

	handler, err := proxy.SetupRoutes(cfg, provider, "backend-key", nil)
	if err == nil {
		t.Fatal("expected error for invalid provider base URL, got nil")
	}

	if handler != nil {
		t.Errorf("expected nil handler on error, got %v", handler)
	}
}

func TestSetupRoutes404ForUnknownPath(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig("")
	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Unknown path should return 404
	req := httptest.NewRequest("GET", "/unknown", http.NoBody)
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown path, got %d", rec.Code)
	}
}

func TestSetupRoutesMessagesPathMustBeExact(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig("")
	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// /v1/messages/extra should not match the route
	req := httptest.NewRequest("POST", "/v1/messages/extra", http.NoBody)
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-exact path, got %d", rec.Code)
	}
}

// Tests for new multi-auth middleware (Bearer + API key support)

func TestSetupRoutesMultiAuthWithBearerToken(t *testing.T) {
	t.Parallel()

	backend := proxy.NewBackendServer(t, `{"status":"ok"}`)

	handler := newAuthHandler(t, backend, config.AuthConfig{
		APIKey:            "",
		AllowBearer:       true,
		BearerSecret:      "test-bearer-token",
		AllowSubscription: false,
	})

	// Request with valid Bearer token should pass
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: "Authorization", Value: "Bearer test-bearer-token"},
	)
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected auth to pass with Bearer token, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutesMultiAuthWithInvalidBearerToken(t *testing.T) {
	t.Parallel()

	handler := newAuthHandler(t, newOKBackend(t), config.AuthConfig{
		APIKey:            "",
		AllowBearer:       true,
		BearerSecret:      "correct-token",
		AllowSubscription: false,
	})

	// Request with invalid Bearer token should fail
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: "Authorization", Value: "Bearer wrong-token"},
	)
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid Bearer token, got %d", rec.Code)
	}
}

func TestSetupRoutesMultiAuthBothMethods(t *testing.T) {
	t.Parallel()

	backend := newOKBackend(t)

	handler := newAuthHandler(t, backend, config.AuthConfig{
		APIKey:            "test-api-key",
		AllowBearer:       true,
		BearerSecret:      "test-bearer-token",
		AllowSubscription: false,
	})

	// Test 1: Bearer token should work
	t.Run("bearer token works", func(t *testing.T) {
		t.Parallel()
		req := proxy.NewMessagesRequestWithHeaders("{}",
			proxy.HeaderPair{Key: "Authorization", Value: "Bearer test-bearer-token"},
		)
		rec := proxy.ServeRequest(t, handler, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected Bearer auth to pass, got 401: %s", rec.Body.String())
		}
	})

	// Test 2: API key should work
	t.Run("api key works", func(t *testing.T) {
		t.Parallel()
		req := proxy.NewMessagesRequestWithHeaders("{}",
			proxy.HeaderPair{Key: proxy.APIKeyHeader, Value: "test-api-key"},
		)
		rec := proxy.ServeRequest(t, handler, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected API key auth to pass, got 401: %s", rec.Body.String())
		}
	})

	// Test 3: No credentials should fail
	t.Run("no credentials fails", func(t *testing.T) {
		t.Parallel()
		req := proxy.NewMessagesRequestWithHeaders("{}")
		rec := proxy.ServeRequest(t, handler, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 with no credentials, got %d", rec.Code)
		}
	})
}

func TestSetupRoutesMultiAuthBearerWithoutSecret(t *testing.T) {
	t.Parallel()

	backend := newOKBackend(t)

	handler := newAuthHandler(t, backend, config.AuthConfig{
		APIKey:            "",
		AllowBearer:       true,
		BearerSecret:      "", // Any token accepted
		AllowSubscription: false,
	})

	// Any Bearer token should work when no secret is configured
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: "Authorization", Value: "Bearer any-random-token"},
	)
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected any Bearer token to pass when no secret, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutesLegacyAPIKeyFallback(t *testing.T) {
	t.Parallel()

	backend := newOKBackend(t)

	// Use legacy Server.APIKey without Auth config
	cfg := newTestConfig("legacy-key")
	provider := proxy.NewTestProvider(backend.URL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Legacy API key should still work
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: proxy.APIKeyHeader, Value: "legacy-key"},
	)
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected legacy API key to work, got 401: %s", rec.Body.String())
	}
}

// Tests for /v1/models endpoint

func TestSetupRoutesModelsEndpoint(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(testAPIKey) // Auth enabled

	// Create providers with models
	anthropicProvider := providers.NewAnthropicProviderWithModels(
		"anthropic-primary",
		proxy.AnthropicBaseURL,
		[]string{"claude-sonnet-4-5-20250514"},
	)
	zaiProvider := providers.NewZAIProviderWithModels(
		"zai-primary",
		"",
		[]string{"glm-4"},
	)

	allProviders := []providers.Provider{anthropicProvider, zaiProvider}

	handler, err := proxy.SetupRoutesWithProviders(cfg, anthropicProvider, "backend-key", nil, allProviders)
	if err != nil {
		t.Fatalf("proxy.SetupRoutesWithProviders failed: %v", err)
	}

	// Models endpoint should work without auth (no auth required for discovery)
	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify Content-Type
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type=application/json, got %s", rec.Header().Get("Content-Type"))
	}

	// Verify response contains both providers' models
	body := rec.Body.String()
	if body == "" {
		t.Error("Response body is empty")
	}
}

func TestSetupRoutesModelsEndpointOnlyGET(t *testing.T) {
	t.Parallel()

	cfg := proxy.TestConfig("")
	provider := providers.NewAnthropicProviderWithModels(
		"test",
		proxy.AnthropicBaseURL,
		[]string{"claude-sonnet-4-5-20250514"},
	)

	handler, err := proxy.SetupRoutesWithProviders(cfg, provider, "backend-key", nil, []providers.Provider{provider})
	if err != nil {
		t.Fatalf("proxy.SetupRoutesWithProviders failed: %v", err)
	}

	// POST to /v1/models should not be handled
	req := httptest.NewRequest("POST", "/v1/models", http.NoBody)
	rec := proxy.ServeRequest(t, handler, req)

	// Should return 405 Method Not Allowed
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST to /v1/models, got %d", rec.Code)
	}
}

func TestSetupRoutesModelsEndpointEmptyProviders(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig("")
	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)

	// Call with empty allProviders
	handler, err := proxy.SetupRoutesWithProviders(cfg, provider, "backend-key", nil, nil)
	if err != nil {
		t.Fatalf("proxy.SetupRoutesWithProviders failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestSetupRoutesSubscriptionTokenAuth(t *testing.T) {
	t.Parallel()

	backend := newOKBackend(t)

	// Test that allow_subscription works as an alias for allow_bearer
	handler := newAuthHandler(t, backend, config.AuthConfig{
		APIKey:            "",
		BearerSecret:      "",
		AllowBearer:       false,
		AllowSubscription: true, // User-friendly config option
	})

	// Subscription token (sent as Bearer) should work
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: "Authorization", Value: "Bearer claude-subscription-token-abc123"},
	)
	rec := proxy.ServeRequest(t, handler, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected subscription token to pass with allow_subscription, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutesSubscriptionAndAPIKeyBothWork(t *testing.T) {
	t.Parallel()

	backend := newOKBackend(t)

	// Test that both subscription and API key auth work together
	handler := newAuthHandler(t, backend, config.AuthConfig{
		APIKey:            "test-api-key",
		BearerSecret:      "",
		AllowBearer:       false,
		AllowSubscription: true,
	})

	// Test 1: Subscription token should work
	t.Run("subscription token works", func(t *testing.T) {
		t.Parallel()
		req := proxy.NewMessagesRequestWithHeaders("{}",
			proxy.HeaderPair{Key: "Authorization", Value: "Bearer subscription-token"},
		)
		rec := proxy.ServeRequest(t, handler, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected subscription token to pass, got 401: %s", rec.Body.String())
		}
	})

	// Test 2: API key should work
	t.Run("api key works", func(t *testing.T) {
		t.Parallel()
		req := proxy.NewMessagesRequestWithHeaders("{}",
			proxy.HeaderPair{Key: proxy.APIKeyHeader, Value: "test-api-key"},
		)
		rec := proxy.ServeRequest(t, handler, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected API key to pass, got 401: %s", rec.Body.String())
		}
	})
}
