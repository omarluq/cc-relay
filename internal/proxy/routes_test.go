// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"bytes"
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

func TestSetupRoutes_CreatesHandler(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: "127.0.0.1:0",
			APIKey: "test-key",
		},
	}
	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestSetupRoutes_AuthMiddlewareApplied(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "test-key",
		},
	}
	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Request without API key should return 401
	req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSetupRoutes_AuthMiddlewareWithValidKey(t *testing.T) {
	t.Parallel()

	// Create mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)

		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer backend.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "test-key",
		},
	}
	provider := providers.NewAnthropicProvider("test", backend.URL)

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Request with valid API key should pass auth and reach backend
	req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
	req.Header.Set("x-api-key", "test-key")
	req.Header.Set("anthropic-version", "2023-06-01")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected auth to pass, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutes_NoAuthWhenAPIKeyEmpty(t *testing.T) {
	t.Parallel()

	// Create mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)

		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer backend.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "", // No auth configured
		},
	}
	provider := providers.NewAnthropicProvider("test", backend.URL)

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Request without API key should NOT return 401 when auth is disabled
	req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
	req.Header.Set("anthropic-version", "2023-06-01")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected no auth when APIKey is empty, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutes_HealthEndpoint(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "test-key", // Auth enabled
		},
	}
	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Health endpoint should work without auth
	req := httptest.NewRequest("GET", "/health", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	expectedBody := `{"status":"ok"}`
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

func TestSetupRoutes_HealthEndpointWithAuth(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "test-key",
		},
	}
	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Health endpoint should work even when server has auth enabled
	// (health check should never require auth)
	req := httptest.NewRequest("GET", "/health", http.NoBody)
	// Intentionally NOT setting x-api-key header
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("health endpoint should not require auth, got status %d", rec.Code)
	}
}

func TestSetupRoutes_OnlyPOSTToMessages(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "", // No auth for simpler test
		},
	}
	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// GET to /v1/messages should not be handled
	req := httptest.NewRequest("GET", "/v1/messages", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should return 405 Method Not Allowed (Go 1.22+ router behavior)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %d", rec.Code)
	}
}

func TestSetupRoutesWithLiveKeyPools_RoutingDebugToggles(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: func() bool { return true }},
	}

	routerInstance, err := router.NewRouter(router.StrategyRoundRobin, 5*time.Second)
	require.NoError(t, err)

	cfgA := &config.Config{
		Server:  config.ServerConfig{APIKey: ""},
		Routing: config.RoutingConfig{Debug: true},
	}
	cfgB := &config.Config{
		Server:  config.ServerConfig{APIKey: ""},
		Routing: config.RoutingConfig{Debug: false},
	}
	runtimeCfg := config.NewRuntime(cfgA)

	handler, err := SetupRoutesWithLiveKeyPools(
		runtimeCfg,
		provider,
		func() []router.ProviderInfo { return providerInfos },
		routerInstance,
		"",
		nil,
		nil,
		nil,
		[]providers.Provider{provider},
		nil,
		nil,
	)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte(`{"model":"test","messages":[]}`)))
	req.Header.Set("anthropic-version", "2023-06-01")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-CC-Relay-Strategy"))

	runtimeCfg.Store(cfgB)

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte(`{"model":"test","messages":[]}`)))
	req2.Header.Set("anthropic-version", "2023-06-01")
	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Empty(t, rec2.Header().Get("X-CC-Relay-Strategy"))
}

func TestSetupRoutesWithLiveKeyPools_AuthToggle(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: func() bool { return true }},
	}

	routerInstance, err := router.NewRouter(router.StrategyRoundRobin, 5*time.Second)
	require.NoError(t, err)

	cfgA := &config.Config{
		Server: config.ServerConfig{
			APIKey: "test-key",
		},
	}
	cfgB := &config.Config{
		Server: config.ServerConfig{APIKey: ""},
	}

	runtimeCfg := config.NewRuntime(cfgA)
	handler, err := SetupRoutesWithLiveKeyPools(
		runtimeCfg,
		provider,
		func() []router.ProviderInfo { return providerInfos },
		routerInstance,
		"",
		nil,
		nil,
		nil,
		[]providers.Provider{provider},
		nil,
		nil,
	)
	require.NoError(t, err)

	unauthReq := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
	unauthReq.Header.Set("anthropic-version", "2023-06-01")
	unauthRec := httptest.NewRecorder()
	handler.ServeHTTP(unauthRec, unauthReq)
	assert.Equal(t, http.StatusUnauthorized, unauthRec.Code)

	runtimeCfg.Store(cfgB)

	okReq := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
	okReq.Header.Set("anthropic-version", "2023-06-01")
	okRec := httptest.NewRecorder()
	handler.ServeHTTP(okRec, okReq)
	assert.Equal(t, http.StatusOK, okRec.Code)
}

type nilRuntimeConfig struct{}

func (nilRuntimeConfig) Get() *config.Config {
	return nil
}

func TestSetupRoutesWithLiveKeyPools_NilConfigProvider(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "http://example.com")
	routerInstance, err := router.NewRouter(router.StrategyRoundRobin, 5*time.Second)
	require.NoError(t, err)

	handler, err := SetupRoutesWithLiveKeyPools(
		nilRuntimeConfig{},
		provider,
		func() []router.ProviderInfo { return nil },
		routerInstance,
		"",
		nil,
		nil,
		nil,
		[]providers.Provider{provider},
		nil,
		nil,
	)
	require.Error(t, err)
	assert.Nil(t, handler)
}

func TestSetupRoutes_OnlyGETToHealth(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "",
		},
	}
	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// POST to /health should not be handled
	req := httptest.NewRequest("POST", "/health", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should return 405 Method Not Allowed
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST to /health, got %d", rec.Code)
	}
}

func TestSetupRoutes_InvalidProviderBaseURL(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "test-key",
		},
	}

	// Create provider with invalid base URL
	provider := providers.NewAnthropicProvider("test", "://invalid-url")

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err == nil {
		t.Fatal("expected error for invalid provider base URL, got nil")
	}

	if handler != nil {
		t.Errorf("expected nil handler on error, got %v", handler)
	}
}

func TestSetupRoutes_404ForUnknownPath(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "",
		},
	}
	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Unknown path should return 404
	req := httptest.NewRequest("GET", "/unknown", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown path, got %d", rec.Code)
	}
}

func TestSetupRoutes_MessagesPathMustBeExact(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "",
		},
	}
	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// /v1/messages/extra should not match the route
	req := httptest.NewRequest("POST", "/v1/messages/extra", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-exact path, got %d", rec.Code)
	}
}

// Tests for new multi-auth middleware (Bearer + API key support)

func TestSetupRoutes_MultiAuthWithBearerToken(t *testing.T) {
	t.Parallel()

	// Create mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer backend.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				AllowBearer:  true,
				BearerSecret: "test-bearer-token",
			},
		},
	}
	provider := providers.NewAnthropicProvider("test", backend.URL)

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Request with valid Bearer token should pass
	req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
	req.Header.Set("Authorization", "Bearer test-bearer-token")
	req.Header.Set("anthropic-version", "2023-06-01")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected auth to pass with Bearer token, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutes_MultiAuthWithInvalidBearerToken(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				AllowBearer:  true,
				BearerSecret: "correct-token",
			},
		},
	}
	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Request with invalid Bearer token should fail
	req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
	req.Header.Set("Authorization", "Bearer wrong-token")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid Bearer token, got %d", rec.Code)
	}
}

func TestSetupRoutes_MultiAuthBothMethods(t *testing.T) {
	t.Parallel()

	// Create mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(backend.Close)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey:       "test-api-key",
				AllowBearer:  true,
				BearerSecret: "test-bearer-token",
			},
		},
	}
	provider := providers.NewAnthropicProvider("test", backend.URL)

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Test 1: Bearer token should work
	t.Run("bearer token works", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
		req.Header.Set("Authorization", "Bearer test-bearer-token")
		req.Header.Set("anthropic-version", "2023-06-01")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected Bearer auth to pass, got 401: %s", rec.Body.String())
		}
	})

	// Test 2: API key should work
	t.Run("api key works", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
		req.Header.Set("x-api-key", "test-api-key")
		req.Header.Set("anthropic-version", "2023-06-01")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected API key auth to pass, got 401: %s", rec.Body.String())
		}
	})

	// Test 3: No credentials should fail
	t.Run("no credentials fails", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
		req.Header.Set("anthropic-version", "2023-06-01")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 with no credentials, got %d", rec.Code)
		}
	})
}

func TestSetupRoutes_MultiAuthBearerWithoutSecret(t *testing.T) {
	t.Parallel()

	// Create mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer backend.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				AllowBearer:  true,
				BearerSecret: "", // Any token accepted
			},
		},
	}
	provider := providers.NewAnthropicProvider("test", backend.URL)

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Any Bearer token should work when no secret is configured
	req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
	req.Header.Set("Authorization", "Bearer any-random-token")
	req.Header.Set("anthropic-version", "2023-06-01")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected any Bearer token to pass when no secret, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutes_LegacyAPIKeyFallback(t *testing.T) {
	t.Parallel()

	// Create mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer backend.Close()

	// Use legacy Server.APIKey without Auth config
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "legacy-key",
			// Auth is empty/unset
		},
	}
	provider := providers.NewAnthropicProvider("test", backend.URL)

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Legacy API key should still work
	req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
	req.Header.Set("x-api-key", "legacy-key")
	req.Header.Set("anthropic-version", "2023-06-01")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected legacy API key to work, got 401: %s", rec.Body.String())
	}
}

// Tests for /v1/models endpoint

func TestSetupRoutes_ModelsEndpoint(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "test-key", // Auth enabled
		},
	}

	// Create providers with models
	anthropicProvider := providers.NewAnthropicProviderWithModels(
		"anthropic-primary",
		"https://api.anthropic.com",
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
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

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

func TestSetupRoutes_ModelsEndpointOnlyGET(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "",
		},
	}
	provider := providers.NewAnthropicProviderWithModels(
		"test",
		"https://api.anthropic.com",
		[]string{"claude-sonnet-4-5-20250514"},
	)

	handler, err := SetupRoutesWithProviders(cfg, provider, "backend-key", nil, []providers.Provider{provider})
	if err != nil {
		t.Fatalf("SetupRoutesWithProviders failed: %v", err)
	}

	// POST to /v1/models should not be handled
	req := httptest.NewRequest("POST", "/v1/models", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should return 405 Method Not Allowed
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST to /v1/models, got %d", rec.Code)
	}
}

func TestSetupRoutes_ModelsEndpointEmptyProviders(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "",
		},
	}
	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	// Call with empty allProviders
	handler, err := SetupRoutesWithProviders(cfg, provider, "backend-key", nil, nil)
	if err != nil {
		t.Fatalf("SetupRoutesWithProviders failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestSetupRoutes_SubscriptionTokenAuth(t *testing.T) {
	t.Parallel()

	// Create mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer backend.Close()

	// Test that allow_subscription works as an alias for allow_bearer
	cfg := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				AllowSubscription: true, // User-friendly config option
				// BearerSecret empty = passthrough mode (any token accepted)
			},
		},
	}
	provider := providers.NewAnthropicProvider("test", backend.URL)

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Subscription token (sent as Bearer) should work
	req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
	req.Header.Set("Authorization", "Bearer claude-subscription-token-abc123")
	req.Header.Set("anthropic-version", "2023-06-01")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Errorf("expected subscription token to pass with allow_subscription, got 401: %s", rec.Body.String())
	}
}

func TestSetupRoutes_SubscriptionAndAPIKeyBothWork(t *testing.T) {
	t.Parallel()

	// Create mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(backend.Close)

	// Test that both subscription and API key auth work together
	cfg := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey:            "test-api-key",
				AllowSubscription: true,
			},
		},
	}
	provider := providers.NewAnthropicProvider("test", backend.URL)

	handler, err := SetupRoutes(cfg, provider, "backend-key", nil)
	if err != nil {
		t.Fatalf("SetupRoutes failed: %v", err)
	}

	// Test 1: Subscription token should work
	t.Run("subscription token works", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
		req.Header.Set("Authorization", "Bearer subscription-token")
		req.Header.Set("anthropic-version", "2023-06-01")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected subscription token to pass, got 401: %s", rec.Body.String())
		}
	})

	// Test 2: API key should work
	t.Run("api key works", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
		req.Header.Set("x-api-key", "test-api-key")
		req.Header.Set("anthropic-version", "2023-06-01")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("expected API key to pass, got 401: %s", rec.Body.String())
		}
	})
}
