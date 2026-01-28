// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

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
)

const (
	testAPIKey    = "test-key"
	okBackendBody = `{"status":"ok"}`
)

func newTestConfig(apiKey string) *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			APIKey: apiKey,
		},
	}
}

func newTestConfigWithListen(apiKey, listen string) *config.Config {
	cfg := newTestConfig(apiKey)
	cfg.Server.Listen = listen
	return cfg
}

func newAuthConfig(auth config.AuthConfig) *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Auth: auth,
		},
	}
}

func newOKBackend(t *testing.T) *httptest.Server {
	t.Helper()
	return newBackendServer(t, okBackendBody)
}

func TestSetupRoutesCreatesHandler(t *testing.T) {
	t.Parallel()

	cfg := newTestConfigWithListen(testAPIKey, "127.0.0.1:0")
	provider := newTestProvider(anthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestSetupRoutesAuthMiddlewareApplied(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(testAPIKey)
	provider := newTestProvider(anthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Request without API key should return 401
	req := newMessagesRequestWithHeaders("{}")
	rec := serveRequest(t, handler, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSetupRoutesAuthMiddlewareWithValidKey(t *testing.T) {
	t.Parallel()

	// Create mock backend server
	backend := newOKBackend(t)

	cfg := newTestConfig(testAPIKey)
	provider := newTestProvider(backend.URL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Request with valid API key should pass auth and reach backend
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: apiKeyHeader, value: testAPIKey},
	)

	rec := serveRequest(t, handler, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected auth to pass, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutesNoAuthWhenAPIKeyEmpty(t *testing.T) {
	t.Parallel()

	// Create mock backend server
	backend := newBackendServer(t, `{"status":"ok"}`)

	cfg := newTestConfig("") // No auth configured
	provider := newTestProvider(backend.URL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Request without API key should NOT return 401 when auth is disabled
	req := newMessagesRequestWithHeaders("{}")
	rec := serveRequest(t, handler, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected no auth when APIKey is empty, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutesHealthEndpoint(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(testAPIKey) // Auth enabled
	provider := newTestProvider(anthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Health endpoint should work without auth
	req := httptest.NewRequest("GET", "/health", http.NoBody)
	rec := serveRequest(t, handler, req)

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
	provider := newTestProvider(anthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Health endpoint should work even when server has auth enabled
	// (health check should never require auth)
	req := httptest.NewRequest("GET", "/health", http.NoBody)
	// Intentionally NOT setting x-api-key header
	rec := serveRequest(t, handler, req)

	if rec.Code != http.StatusOK {
		t.Errorf("health endpoint should not require auth, got status %d", rec.Code)
	}
}

func TestSetupRoutesOnlyPOSTToMessages(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig("") // No auth for simpler test
	provider := newTestProvider(anthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// GET to /v1/messages should not be handled
	req := httptest.NewRequest("GET", "/v1/messages", http.NoBody)
	rec := serveRequest(t, handler, req)

	// Should return 405 Method Not Allowed (Go 1.22+ router behavior)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %d", rec.Code)
	}
}

func TestSetupRoutesWithLiveKeyPoolsRoutingDebugToggles(t *testing.T) {
	t.Parallel()

	backend := newBackendServer(t, `{"ok":true}`)

	provider := newTestProvider(backend.URL)
	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: func() bool { return true }},
	}

	cfgA := &config.Config{
		Server:  config.ServerConfig{APIKey: ""},
		Routing: config.RoutingConfig{Debug: true},
	}
	cfgB := &config.Config{
		Server:  config.ServerConfig{APIKey: ""},
		Routing: config.RoutingConfig{Debug: false},
	}
	runtimeCfg := config.NewRuntime(cfgA)
	handler := newLiveKeyPoolsHandler(t, runtimeCfg, provider, providerInfos)

	req := newMessagesRequestWithHeaders(`{"model":"test","messages":[]}`)
	rec := serveRequest(t, handler, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-CC-Relay-Strategy"))

	runtimeCfg.Store(cfgB)

	req2 := newMessagesRequestWithHeaders(`{"model":"test","messages":[]}`)
	rec2 := serveRequest(t, handler, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Empty(t, rec2.Header().Get("X-CC-Relay-Strategy"))
}

func TestSetupRoutesWithLiveKeyPoolsAuthToggle(t *testing.T) {
	t.Parallel()

	backend := newBackendServer(t, `{"ok":true}`)

	provider := newTestProvider(backend.URL)
	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: func() bool { return true }},
	}

	cfgA := newTestConfig(testAPIKey)
	cfgB := newTestConfig("")

	runtimeCfg := config.NewRuntime(cfgA)
	handler := newLiveKeyPoolsHandler(t, runtimeCfg, provider, providerInfos)

	unauthReq := newMessagesRequestWithHeaders("{}")
	unauthRec := serveRequest(t, handler, unauthReq)
	assert.Equal(t, http.StatusUnauthorized, unauthRec.Code)

	runtimeCfg.Store(cfgB)

	okReq := newMessagesRequestWithHeaders("{}")
	okRec := serveRequest(t, handler, okReq)
	assert.Equal(t, http.StatusOK, okRec.Code)
}

type nilRuntimeConfigGetter struct{}

func (nilRuntimeConfigGetter) Get() *config.Config {
	return nil
}

func TestSetupRoutesWithLiveKeyPoolsNilConfigProvider(t *testing.T) {
	t.Parallel()

	provider := newTestProvider("http://example.com")
	routerInstance, err := router.NewRouter(router.StrategyRoundRobin, 5*time.Second)
	require.NoError(t, err)

	handler, err := SetupRoutesWithLiveKeyPools(&RoutesOptions{
		ConfigProvider:    nilRuntimeConfigGetter{},
		Provider:          provider,
		ProviderInfosFunc: func() []router.ProviderInfo { return nil },
		ProviderRouter:    routerInstance,
		ProviderKey:       "",
		Pool:              nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		AllProviders:      []providers.Provider{provider},
		HealthTracker:     nil,
		SignatureCache:    nil,
	})
	require.Error(t, err)
	assert.Nil(t, handler)
}

func TestSetupRoutesOnlyGETToHealth(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig("")
	provider := newTestProvider(anthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// POST to /health should not be handled
	req := httptest.NewRequest("POST", "/health", http.NoBody)
	rec := serveRequest(t, handler, req)

	// Should return 405 Method Not Allowed
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST to /health, got %d", rec.Code)
	}
}

func setupRoutesHandler(t *testing.T, cfg *config.Config, provider providers.Provider) http.Handler {
	t.Helper()

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
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

	handler, err := SetupRoutesWithLiveKeyPools(&RoutesOptions{
		ConfigProvider:    runtimeCfg,
		Provider:          provider,
		ProviderInfosFunc: func() []router.ProviderInfo { return providerInfos },
		ProviderRouter:    routerInstance,
		ProviderKey:       "",
		Pool:              nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		AllProviders:      []providers.Provider{provider},
		HealthTracker:     nil,
		SignatureCache:    nil,
	})
	require.NoError(t, err)

	return handler
}

func TestSetupRoutesInvalidProviderBaseURL(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(testAPIKey)

	// Create provider with invalid base URL
	provider := newTestProvider("://invalid-url")

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
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
	provider := newTestProvider(anthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Unknown path should return 404
	req := httptest.NewRequest("GET", "/unknown", http.NoBody)
	rec := serveRequest(t, handler, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown path, got %d", rec.Code)
	}
}

func TestSetupRoutesMessagesPathMustBeExact(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig("")
	provider := newTestProvider(anthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// /v1/messages/extra should not match the route
	req := httptest.NewRequest("POST", "/v1/messages/extra", http.NoBody)
	rec := serveRequest(t, handler, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-exact path, got %d", rec.Code)
	}
}

// Tests for new multi-auth middleware (Bearer + API key support)

func TestSetupRoutesMultiAuthWithBearerToken(t *testing.T) {
	t.Parallel()

	backend := newBackendServer(t, `{"status":"ok"}`)

	cfg := newAuthConfig(config.AuthConfig{
		AllowBearer:  true,
		BearerSecret: "test-bearer-token",
	})
	provider := newTestProvider(backend.URL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Request with valid Bearer token should pass
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: "Authorization", value: "Bearer test-bearer-token"},
	)
	rec := serveRequest(t, handler, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected auth to pass with Bearer token, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutesMultiAuthWithInvalidBearerToken(t *testing.T) {
	t.Parallel()

	cfg := newAuthConfig(config.AuthConfig{
		AllowBearer:  true,
		BearerSecret: "correct-token",
	})
	provider := newTestProvider(anthropicBaseURL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Request with invalid Bearer token should fail
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: "Authorization", value: "Bearer wrong-token"},
	)
	rec := serveRequest(t, handler, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid Bearer token, got %d", rec.Code)
	}
}

func TestSetupRoutesMultiAuthBothMethods(t *testing.T) {
	t.Parallel()

	backend := newOKBackend(t)

	cfg := newAuthConfig(config.AuthConfig{
		APIKey:       "test-api-key",
		AllowBearer:  true,
		BearerSecret: "test-bearer-token",
	})
	provider := newTestProvider(backend.URL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Test 1: Bearer token should work
	t.Run("bearer token works", func(t *testing.T) {
		t.Parallel()
		req := newMessagesRequestWithHeaders("{}",
			headerPair{key: "Authorization", value: "Bearer test-bearer-token"},
		)
		rec := serveRequest(t, handler, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected Bearer auth to pass, got 401: %s", rec.Body.String())
		}
	})

	// Test 2: API key should work
	t.Run("api key works", func(t *testing.T) {
		t.Parallel()
		req := newMessagesRequestWithHeaders("{}",
			headerPair{key: apiKeyHeader, value: "test-api-key"},
		)
		rec := serveRequest(t, handler, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected API key auth to pass, got 401: %s", rec.Body.String())
		}
	})

	// Test 3: No credentials should fail
	t.Run("no credentials fails", func(t *testing.T) {
		t.Parallel()
		req := newMessagesRequestWithHeaders("{}")
		rec := serveRequest(t, handler, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 with no credentials, got %d", rec.Code)
		}
	})
}

func TestSetupRoutesMultiAuthBearerWithoutSecret(t *testing.T) {
	t.Parallel()

	backend := newOKBackend(t)

	cfg := newAuthConfig(config.AuthConfig{
		AllowBearer:  true,
		BearerSecret: "", // Any token accepted
	})
	provider := newTestProvider(backend.URL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Any Bearer token should work when no secret is configured
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: "Authorization", value: "Bearer any-random-token"},
	)
	rec := serveRequest(t, handler, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected any Bearer token to pass when no secret, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutesLegacyAPIKeyFallback(t *testing.T) {
	t.Parallel()

	backend := newOKBackend(t)

	// Use legacy Server.APIKey without Auth config
	cfg := newTestConfig("legacy-key")
	provider := newTestProvider(backend.URL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Legacy API key should still work
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: apiKeyHeader, value: "legacy-key"},
	)
	rec := serveRequest(t, handler, req)

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
		anthropicBaseURL,
		[]string{"claude-sonnet-4-5-20250514"},
	)
	zaiProvider := providers.NewZAIProviderWithModels(
		"zai-primary",
		"",
		[]string{"glm-4"},
	)

	allProviders := []providers.Provider{anthropicProvider, zaiProvider}

	handler, err := SetupRoutesWithProviders(cfg, anthropicProvider, "backend-key", nil, allProviders)
	if err != nil {
		t.Fatalf("SetupRoutesWithProviders failed: %v", err)
	}

	// Models endpoint should work without auth (no auth required for discovery)
	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	rec := serveRequest(t, handler, req)

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

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "",
		},
	}
	provider := providers.NewAnthropicProviderWithModels(
		"test",
		anthropicBaseURL,
		[]string{"claude-sonnet-4-5-20250514"},
	)

	handler, err := SetupRoutesWithProviders(cfg, provider, "backend-key", nil, []providers.Provider{provider})
	if err != nil {
		t.Fatalf("SetupRoutesWithProviders failed: %v", err)
	}

	// POST to /v1/models should not be handled
	req := httptest.NewRequest("POST", "/v1/models", http.NoBody)
	rec := serveRequest(t, handler, req)

	// Should return 405 Method Not Allowed
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST to /v1/models, got %d", rec.Code)
	}
}

func TestSetupRoutesModelsEndpointEmptyProviders(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig("")
	provider := newTestProvider(anthropicBaseURL)

	// Call with empty allProviders
	handler, err := SetupRoutesWithProviders(cfg, provider, "backend-key", nil, nil)
	if err != nil {
		t.Fatalf("SetupRoutesWithProviders failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	rec := serveRequest(t, handler, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestSetupRoutesSubscriptionTokenAuth(t *testing.T) {
	t.Parallel()

	backend := newOKBackend(t)

	// Test that allow_subscription works as an alias for allow_bearer
	cfg := newAuthConfig(config.AuthConfig{
		AllowSubscription: true, // User-friendly config option
		// BearerSecret empty = passthrough mode (any token accepted)
	})
	provider := newTestProvider(backend.URL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Subscription token (sent as Bearer) should work
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: "Authorization", value: "Bearer claude-subscription-token-abc123"},
	)
	rec := serveRequest(t, handler, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected subscription token to pass with allow_subscription, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutesSubscriptionAndAPIKeyBothWork(t *testing.T) {
	t.Parallel()

	backend := newOKBackend(t)

	// Test that both subscription and API key auth work together
	cfg := newAuthConfig(config.AuthConfig{
		APIKey:            "test-api-key",
		AllowSubscription: true,
	})
	provider := newTestProvider(backend.URL)

	handler := setupRoutesHandler(t, cfg, provider)

	// Test 1: Subscription token should work
	t.Run("subscription token works", func(t *testing.T) {
		t.Parallel()
		req := newMessagesRequestWithHeaders("{}",
			headerPair{key: "Authorization", value: "Bearer subscription-token"},
		)
		rec := serveRequest(t, handler, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected subscription token to pass, got 401: %s", rec.Body.String())
		}
	})

	// Test 2: API key should work
	t.Run("api key works", func(t *testing.T) {
		t.Parallel()
		req := newMessagesRequestWithHeaders("{}",
			headerPair{key: apiKeyHeader, value: "test-api-key"},
		)
		rec := serveRequest(t, handler, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected API key to pass, got 401: %s", rec.Body.String())
		}
	})
}
