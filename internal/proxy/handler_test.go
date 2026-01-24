package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/health"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
)

// newTestHandler is a helper that creates a handler with common test defaults.
//
//nolint:unparam // pool and healthTracker are provided for interface consistency with NewHandler
func newTestHandler(
	t *testing.T,
	provider providers.Provider,
	providerInfos []router.ProviderInfo,
	providerRouter router.ProviderRouter,
	apiKey string,
	pool *keypool.KeyPool,
	routingDebug bool,
	healthTracker *health.Tracker,
) *Handler {
	t.Helper()
	handler, err := NewHandler(
		provider, providerInfos, providerRouter,
		apiKey, pool, nil, nil, nil,
		config.DebugOptions{}, routingDebug, healthTracker,
	)
	require.NoError(t, err)
	return handler
}

func TestNewHandler_ValidProvider(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := NewHandler(
		provider, nil, nil, "test-key", nil, nil, nil, nil,
		config.DebugOptions{}, false, nil,
	)
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	if handler == nil {
		t.Error("Expected non-nil handler")
	}
}

func TestNewHandler_InvalidURL(t *testing.T) {
	t.Parallel()
	// Create a mock provider with invalid URL
	provider := &mockProvider{baseURL: "://invalid-url"}

	_, err := NewHandler(provider, nil, nil, "test-key", nil, nil, nil, nil, config.DebugOptions{}, false, nil)
	if err == nil {
		t.Error("Expected error for invalid base URL, got nil")
	}
}

func TestHandler_ForwardsAnthropicHeaders(t *testing.T) {
	t.Parallel()

	// Create mock backend that echoes headers
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for anthropic headers
		if r.Header.Get("Anthropic-Version") != "2023-06-01" {
			t.Errorf("Expected Anthropic-Version header, got %q", r.Header.Get("Anthropic-Version"))
		}

		if r.Header.Get("Anthropic-Beta") != "test-feature" {
			t.Errorf("Expected Anthropic-Beta header, got %q", r.Header.Get("Anthropic-Beta"))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Create provider pointing to mock backend
	provider := providers.NewAnthropicProvider("test", backend.URL)

	handler, err := NewHandler(provider, nil, nil, "test-key", nil, nil, nil, nil, config.DebugOptions{}, false, nil)
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	// Create request with anthropic headers
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	req.Header.Set("Anthropic-Version", "2023-06-01")
	req.Header.Set("Anthropic-Beta", "test-feature")

	// Serve request
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandler_HasErrorHandler(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := NewHandler(provider, nil, nil, "test-key", nil, nil, nil, nil, config.DebugOptions{}, false, nil)
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	// Verify ProviderProxy exists and has ErrorHandler configured
	pp, ok := handler.providerProxies[provider.Name()]
	if !ok {
		t.Error("Expected provider proxy to be configured")
		return
	}
	if pp.Proxy.ErrorHandler == nil {
		t.Error("ErrorHandler should be configured")
	}
}

func TestHandler_StructureCorrect(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := NewHandler(provider, nil, nil, "test-key", nil, nil, nil, nil, config.DebugOptions{}, false, nil)
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	// Verify handler has providerProxies map
	if handler.providerProxies == nil {
		t.Error("handler.providerProxies is nil")
	}

	// Verify provider proxy exists
	pp, ok := handler.providerProxies[provider.Name()]
	if !ok {
		t.Error("expected provider proxy to be configured")
		return
	}

	// Verify FlushInterval is set to -1
	if pp.Proxy.FlushInterval != -1 {
		t.Errorf("FlushInterval = %v, want -1", pp.Proxy.FlushInterval)
	}

	// Verify provider is set
	if pp.Provider == nil {
		t.Error("provider proxy's Provider is nil")
	}

	// Verify apiKey is set
	if pp.APIKey != "test-key" {
		t.Errorf("provider proxy APIKey = %q, want %q", pp.APIKey, "test-key")
	}
}

func TestHandler_PreservesToolUseId(t *testing.T) {
	t.Parallel()

	// Create mock backend that echoes request body
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		// Echo the body back
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer backend.Close()

	// Create provider pointing to mock backend
	provider := providers.NewAnthropicProvider("test", backend.URL)

	handler, err := NewHandler(provider, nil, nil, "test-key", nil, nil, nil, nil, config.DebugOptions{}, false, nil)
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	// Request body with tool_use_id
	requestBody := `{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"test"}],` +
		`"tools":[{"name":"test","input_schema":{}}],` +
		`"tool_choice":{"type":"tool","name":"test","tool_use_id":"toolu_123"}}`

	// Create request
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte(requestBody)))
	req.Header.Set("Content-Type", "application/json")

	// Serve request
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify response contains tool_use_id
	responseBody := w.Body.String()
	if !bytes.Contains([]byte(responseBody), []byte("toolu_123")) {
		t.Errorf("Expected response to contain tool_use_id, got: %s", responseBody)
	}
}

// mockProvider is a mock implementation of Provider for testing.
type mockProvider struct {
	baseURL string
}

func (m *mockProvider) Name() string {
	return "mock"
}

func (m *mockProvider) BaseURL() string {
	return m.baseURL
}

func (m *mockProvider) Authenticate(_ *http.Request, _ string) error {
	return nil
}

func (m *mockProvider) ForwardHeaders(originalHeaders http.Header) http.Header {
	headers := make(http.Header)

	for key, values := range originalHeaders {
		canonicalKey := http.CanonicalHeaderKey(key)
		if len(canonicalKey) >= 10 && canonicalKey[:10] == "Anthropic-" {
			headers[canonicalKey] = append(headers[canonicalKey], values...)
		}
	}

	headers.Set("Content-Type", "application/json")

	return headers
}

func (m *mockProvider) SupportsStreaming() bool {
	return true
}

func (m *mockProvider) SupportsTransparentAuth() bool {
	return false // Mock provider doesn't support transparent auth (like Z.AI)
}

func (m *mockProvider) Owner() string {
	return "mock"
}

func (m *mockProvider) ListModels() []providers.Model {
	return nil
}

func (m *mockProvider) GetModelMapping() map[string]string {
	return nil
}

func (m *mockProvider) MapModel(model string) string {
	return model
}

// TestHandler_WithKeyPool tests handler with key pool integration.
func TestHandler_WithKeyPool(t *testing.T) {
	t.Parallel()

	// Create mock backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Return success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test","type":"message"}`))
	}))
	defer backend.Close()

	// Create key pool with test keys
	pool, err := keypool.NewKeyPool("test-provider", keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			{APIKey: "test-key-1", RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
			{APIKey: "test-key-2", RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
		},
	})
	require.NoError(t, err)

	// Create handler with key pool
	provider := providers.NewAnthropicProvider("test", backend.URL)
	handler, err := NewHandler(provider, nil, nil, "", pool, nil, nil, nil, config.DebugOptions{}, false, nil)
	require.NoError(t, err)

	// Make request
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify x-cc-relay-* headers are set
	assert.NotEmpty(t, w.Header().Get(HeaderRelayKeyID))
	assert.Equal(t, "2", w.Header().Get(HeaderRelayKeysTotal))
	assert.Equal(t, "2", w.Header().Get(HeaderRelayKeysAvail))
}

// TestHandler_AllKeysExhausted tests 429 response when all keys exhausted.
func TestHandler_AllKeysExhausted(t *testing.T) {
	t.Parallel()

	// Create key pool with single key and very low limit
	pool, err := keypool.NewKeyPool("test-provider", keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			{APIKey: "test-key", RPMLimit: 1, ITPMLimit: 1, OTPMLimit: 1},
		},
	})
	require.NoError(t, err)

	// Exhaust the key by making a request
	_, _, err = pool.GetKey(context.Background())
	require.NoError(t, err)

	// Create handler
	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")
	handler, err := NewHandler(provider, nil, nil, "", pool, nil, nil, nil, config.DebugOptions{}, false, nil)
	require.NoError(t, err)

	// Make request (should return 429)
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify 429 response
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Verify Retry-After header exists
	assert.NotEmpty(t, w.Header().Get("Retry-After"))

	// Verify response body matches Anthropic error format
	var errResp ErrorResponse
	err = json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "error", errResp.Type)
	assert.Equal(t, "rate_limit_error", errResp.Error.Type)
	assert.Contains(t, errResp.Error.Message, "rate limit")
}

// TestHandler_KeyPoolUpdate tests that handler updates key state from response headers.
func TestHandler_KeyPoolUpdate(t *testing.T) {
	t.Parallel()

	// Create mock backend that returns rate limit headers
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("anthropic-ratelimit-requests-limit", "100")
		w.Header().Set("anthropic-ratelimit-requests-remaining", "99")
		w.Header().Set("anthropic-ratelimit-input-tokens-limit", "50000")
		w.Header().Set("anthropic-ratelimit-input-tokens-remaining", "49000")
		w.Header().Set("anthropic-ratelimit-output-tokens-limit", "20000")
		w.Header().Set("anthropic-ratelimit-output-tokens-remaining", "19000")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Create key pool
	pool, err := keypool.NewKeyPool("test-provider", keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			{APIKey: "test-key", RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
		},
	})
	require.NoError(t, err)

	// Create handler
	provider := providers.NewAnthropicProvider("test", backend.URL)
	handler, err := NewHandler(provider, nil, nil, "", pool, nil, nil, nil, config.DebugOptions{}, false, nil)
	require.NoError(t, err)

	// Make request
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify response OK
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify key state was updated (check via stats)
	stats := pool.GetStats()
	assert.Equal(t, 1, stats.TotalKeys)
	assert.Equal(t, 1, stats.AvailableKeys)
}

// TestHandler_Backend429 tests that handler marks key exhausted on backend 429.
func TestHandler_Backend429(t *testing.T) {
	t.Parallel()

	// Create mock backend that returns 429
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"rate limit"}}`))
	}))
	defer backend.Close()

	// Create key pool
	pool, err := keypool.NewKeyPool("test-provider", keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			{APIKey: "test-key", RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
		},
	})
	require.NoError(t, err)

	// Create handler
	provider := providers.NewAnthropicProvider("test", backend.URL)
	handler, err := NewHandler(provider, nil, nil, "", pool, nil, nil, nil, config.DebugOptions{}, false, nil)
	require.NoError(t, err)

	// Make request
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify 429 is passed through
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Wait a bit for async update
	time.Sleep(10 * time.Millisecond)

	// Verify key is marked as exhausted (all keys should be unavailable)
	stats := pool.GetStats()
	assert.Equal(t, 1, stats.ExhaustedKeys)
}

// TestHandler_SingleKeyMode tests backwards compatibility with nil pool.
func TestHandler_SingleKeyMode(t *testing.T) {
	t.Parallel()

	// Create mock backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header uses single key
		assert.Equal(t, "test-single-key", r.Header.Get("X-Api-Key"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Create handler without key pool (nil)
	provider := providers.NewAnthropicProvider("test", backend.URL)
	handler, err := NewHandler(
		provider, nil, nil, "test-single-key", nil, nil, nil, nil,
		config.DebugOptions{}, false, nil,
	)
	require.NoError(t, err)

	// Make request
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify response OK
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify no x-cc-relay-* headers (single key mode)
	assert.Empty(t, w.Header().Get(HeaderRelayKeyID))
	assert.Empty(t, w.Header().Get(HeaderRelayKeysTotal))
}

// TestHandler_UsesFallbackKeyWhenNoClientAuth tests that configured provider keys
// are used when client provides no auth headers.
func TestHandler_UsesFallbackKeyWhenNoClientAuth(t *testing.T) {
	t.Parallel()

	var receivedAuthHeader string
	var receivedAPIKeyHeader string

	// Create mock backend that captures headers
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		receivedAPIKeyHeader = r.Header.Get("x-api-key")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	handler, err := NewHandler(
		provider, nil, nil, "our-fallback-key", nil, nil, nil, nil,
		config.DebugOptions{}, false, nil,
	)
	require.NoError(t, err)

	// Create request WITHOUT any auth headers
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	req.Header.Set("Anthropic-Version", "2024-01-01")
	// NO Authorization, NO x-api-key

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Client Authorization should be empty (none provided)
	assert.Empty(t, receivedAuthHeader)

	// Our fallback key should be used
	assert.Equal(t, "our-fallback-key", receivedAPIKeyHeader)
}

// TestHandler_ForwardsClientAuthWhenPresent tests that client Authorization header
// is forwarded unchanged when present (transparent proxy mode).
func TestHandler_ForwardsClientAuthWhenPresent(t *testing.T) {
	t.Parallel()

	var receivedAuthHeader string
	var receivedAPIKeyHeader string

	// Create mock backend that captures headers
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		receivedAPIKeyHeader = r.Header.Get("x-api-key")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Create handler with a configured fallback key
	provider := providers.NewAnthropicProvider("test", backend.URL)
	handler, err := NewHandler(
		provider, nil, nil, "fallback-key", nil, nil, nil, nil,
		config.DebugOptions{}, false, nil,
	)
	require.NoError(t, err)

	// Create request WITH client Authorization header
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer sub_12345")
	req.Header.Set("Anthropic-Version", "2024-01-01")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify response OK
	assert.Equal(t, http.StatusOK, w.Code)

	// CRITICAL: Client Authorization header should be forwarded UNCHANGED
	assert.Equal(t, "Bearer sub_12345", receivedAuthHeader)

	// Our fallback key should NOT be added
	assert.Empty(t, receivedAPIKeyHeader, "fallback key should not be added when client has auth")
}

// TestHandler_ForwardsClientAPIKeyWhenPresent tests that client x-api-key header
// is forwarded unchanged when present (transparent proxy mode).
func TestHandler_ForwardsClientAPIKeyWhenPresent(t *testing.T) {
	t.Parallel()

	var receivedAuthHeader string
	var receivedAPIKeyHeader string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		receivedAPIKeyHeader = r.Header.Get("x-api-key")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	handler, err := NewHandler(
		provider, nil, nil, "fallback-key", nil, nil, nil, nil,
		config.DebugOptions{}, false, nil,
	)
	require.NoError(t, err)

	// Create request WITH client x-api-key header
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	req.Header.Set("x-api-key", "sk-ant-client-key")
	req.Header.Set("Anthropic-Version", "2024-01-01")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Client x-api-key should be forwarded UNCHANGED
	assert.Equal(t, "sk-ant-client-key", receivedAPIKeyHeader)

	// No Authorization header should be added
	assert.Empty(t, receivedAuthHeader)
}

// TestHandler_TransparentModeSkipsKeyPool tests that key pool is skipped
// when client provides auth (rate limiting is their problem).
func TestHandler_TransparentModeSkipsKeyPool(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Create key pool with test keys
	pool, err := keypool.NewKeyPool("test-provider", keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			{APIKey: "pool-key-1", RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
		},
	})
	require.NoError(t, err)

	provider := providers.NewAnthropicProvider("test", backend.URL)
	handler, err := NewHandler(provider, nil, nil, "", pool, nil, nil, nil, config.DebugOptions{}, false, nil)
	require.NoError(t, err)

	// Create request WITH client auth
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer client-token")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// x-cc-relay-* headers should NOT be set (key pool was skipped)
	assert.Empty(t, w.Header().Get(HeaderRelayKeyID), "key pool should be skipped in transparent mode")
	assert.Empty(t, w.Header().Get(HeaderRelayKeysTotal))
}

// TestHandler_FallbackModeUsesKeyPool tests that key pool is used
// when client provides no auth.
func TestHandler_FallbackModeUsesKeyPool(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	pool, err := keypool.NewKeyPool("test-provider", keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			{APIKey: "pool-key-1", RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
		},
	})
	require.NoError(t, err)

	provider := providers.NewAnthropicProvider("test", backend.URL)
	handler, err := NewHandler(provider, nil, nil, "", pool, nil, nil, nil, config.DebugOptions{}, false, nil)
	require.NoError(t, err)

	// Create request WITHOUT client auth
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	// NO Authorization, NO x-api-key

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// x-cc-relay-* headers SHOULD be set (key pool was used)
	assert.NotEmpty(t, w.Header().Get(HeaderRelayKeyID), "key pool should be used in fallback mode")
	assert.Equal(t, "1", w.Header().Get(HeaderRelayKeysTotal))
}

// TestHandler_TransparentModeForwardsAnthropicHeaders tests that anthropic-* headers
// are forwarded in transparent mode.
func TestHandler_TransparentModeForwardsAnthropicHeaders(t *testing.T) {
	t.Parallel()

	var receivedVersion string
	var receivedBeta string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedVersion = r.Header.Get("Anthropic-Version")
		receivedBeta = r.Header.Get("Anthropic-Beta")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	handler, err := NewHandler(
		provider, nil, nil, "fallback-key", nil, nil, nil, nil,
		config.DebugOptions{}, false, nil,
	)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer client-token")
	req.Header.Set("Anthropic-Version", "2024-01-01")
	req.Header.Set("Anthropic-Beta", "extended-thinking-2024-01-01")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "2024-01-01", receivedVersion)
	assert.Equal(t, "extended-thinking-2024-01-01", receivedBeta)
}

// TestHandler_NonTransparentProviderUsesConfiguredKeys tests that providers
// that don't support transparent auth (like Z.AI) use configured keys even
// when client sends Authorization header.
func TestHandler_NonTransparentProviderUsesConfiguredKeys(t *testing.T) {
	t.Parallel()

	var receivedAPIKey string
	var receivedAuthHeader string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("x-api-key")
		receivedAuthHeader = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Z.AI provider does NOT support transparent auth
	provider := providers.NewZAIProvider("test-zai", backend.URL)
	handler := newTestHandler(t, provider, nil, nil, "zai-configured-key", nil, false, nil)

	// Client sends Authorization header (like Claude Code does)
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer client-anthropic-token")
	req.Header.Set("Anthropic-Version", "2024-01-01")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// CRITICAL: Client Authorization should NOT be forwarded to Z.AI
	// Instead, our configured x-api-key should be used
	assert.Empty(t, receivedAuthHeader, "client Authorization should not be forwarded to non-transparent provider")
	assert.Equal(t, "zai-configured-key", receivedAPIKey, "configured key should be used for Z.AI")
}

// TestHandler_NonTransparentProviderWithKeyPool tests that non-transparent providers
// use key pool even when client sends auth headers.
func TestHandler_NonTransparentProviderWithKeyPool(t *testing.T) {
	t.Parallel()

	var receivedAPIKey string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("x-api-key")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Create key pool
	pool, err := keypool.NewKeyPool("test-zai", keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			{APIKey: "zai-pool-key-1", RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
		},
	})
	require.NoError(t, err)

	// Z.AI provider does NOT support transparent auth
	provider := providers.NewZAIProvider("test-zai", backend.URL)
	handler, err := NewHandler(provider, nil, nil, "", pool, nil, nil, nil, config.DebugOptions{}, false, nil)
	require.NoError(t, err)

	// Client sends Authorization header
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer client-anthropic-token")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Key pool should be used (relay headers present)
	assert.NotEmpty(t, w.Header().Get(HeaderRelayKeyID), "key pool should be used for non-transparent provider")

	// Configured pool key should be sent, not client auth
	assert.Equal(t, "zai-pool-key-1", receivedAPIKey)
}

// TestParseRetryAfter tests the parseRetryAfter helper function.
func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		header   string
		expected time.Duration
	}{
		{
			name:     "integer seconds",
			header:   "60",
			expected: 60 * time.Second,
		},
		{
			name:     "missing header",
			header:   "",
			expected: 60 * time.Second, // default
		},
		{
			name:     "invalid format",
			header:   "invalid",
			expected: 60 * time.Second, // default
		},
		{
			name:     "zero seconds",
			header:   "0",
			expected: 0,
		},
		{
			name:     "large value",
			header:   "3600",
			expected: 3600 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			headers := make(http.Header)
			if tt.header != "" {
				headers.Set("Retry-After", tt.header)
			}

			result := parseRetryAfter(headers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// mockRouter implements router.ProviderRouter for testing.
type mockRouter struct {
	err      error
	name     string
	selected router.ProviderInfo
}

func (m *mockRouter) Select(_ context.Context, _ []router.ProviderInfo) (router.ProviderInfo, error) {
	return m.selected, m.err
}

func (m *mockRouter) Name() string {
	return m.name
}

// TestHandler_SingleProviderMode tests that handler works without router (backwards compat).
func TestHandler_SingleProviderMode(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	// No router (nil), no providers list (nil) - single provider mode
	handler, err := NewHandler(provider, nil, nil, "test-key", nil, nil, nil, nil, config.DebugOptions{}, false, nil)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// No routing debug headers in single provider mode
	assert.Empty(t, w.Header().Get("X-CC-Relay-Strategy"))
	assert.Empty(t, w.Header().Get("X-CC-Relay-Provider"))
}

// TestHandler_MultiProviderModeUsesRouter tests that handler uses router for selection.
func TestHandler_MultiProviderModeUsesRouter(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider1 := providers.NewAnthropicProvider("provider1", backend.URL)
	provider2 := providers.NewAnthropicProvider("provider2", backend.URL)

	providerInfos := []router.ProviderInfo{
		{Provider: provider1, IsHealthy: func() bool { return true }},
		{Provider: provider2, IsHealthy: func() bool { return true }},
	}

	// Mock router that always selects provider2
	mockR := &mockRouter{
		name: "test_strategy",
		selected: router.ProviderInfo{
			Provider:  provider2,
			IsHealthy: func() bool { return true },
		},
	}

	// routingDebug=true to get debug headers
	handler := newTestHandler(t, provider1, providerInfos, mockR, "test-key", nil, true, nil)

	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Debug headers should be present
	assert.Equal(t, "test_strategy", w.Header().Get("X-CC-Relay-Strategy"))
	assert.Equal(t, "provider2", w.Header().Get("X-CC-Relay-Provider"))
}

// TestHandler_DebugHeadersDisabledByDefault tests that debug headers are not added when disabled.
func TestHandler_DebugHeadersDisabledByDefault(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: func() bool { return true }},
	}

	mockR := &mockRouter{
		name: "failover",
		selected: router.ProviderInfo{
			Provider:  provider,
			IsHealthy: func() bool { return true },
		},
	}

	// routingDebug=false (default)
	handler := newTestHandler(t, provider, providerInfos, mockR, "test-key", nil, false, nil)

	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// No debug headers
	assert.Empty(t, w.Header().Get("X-CC-Relay-Strategy"))
	assert.Empty(t, w.Header().Get("X-CC-Relay-Provider"))
}

// TestHandler_DebugHeadersWhenEnabled tests debug headers are added when routing.debug=true.
func TestHandler_DebugHeadersWhenEnabled(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test-provider", backend.URL)
	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: func() bool { return true }},
	}

	mockR := &mockRouter{
		name: "round_robin",
		selected: router.ProviderInfo{
			Provider:  provider,
			IsHealthy: func() bool { return true },
		},
	}

	handler := newTestHandler(t, provider, providerInfos, mockR, "test-key", nil, true, nil)

	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Debug headers present
	assert.Equal(t, "round_robin", w.Header().Get("X-CC-Relay-Strategy"))
	assert.Equal(t, "test-provider", w.Header().Get("X-CC-Relay-Provider"))
}

// TestHandler_RouterSelectionError tests error handling when router fails.
func TestHandler_RouterSelectionError(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")
	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: func() bool { return false }},
	}

	// Mock router that returns error
	mockR := &mockRouter{
		name: "failover",
		err:  router.ErrAllProvidersUnhealthy,
	}

	handler := newTestHandler(t, provider, providerInfos, mockR, "test-key", nil, false, nil)

	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should return 503 Service Unavailable
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var errResp ErrorResponse
	decodeErr := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, decodeErr)
	assert.Equal(t, "error", errResp.Type)
	assert.Equal(t, "api_error", errResp.Error.Type)
	assert.Contains(t, errResp.Error.Message, "failed to select provider")
}

// TestHandler_SelectProviderSingleMode tests selectProvider in single provider mode.
func TestHandler_SelectProviderSingleMode(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	// No router, no providers - single provider mode
	handler, err := NewHandler(provider, nil, nil, "test-key", nil, nil, nil, nil, config.DebugOptions{}, false, nil)
	require.NoError(t, err)

	info, err := handler.selectProvider(context.Background(), "", false)
	require.NoError(t, err)
	assert.Equal(t, "test", info.Provider.Name())
	assert.True(t, info.Healthy()) // Always healthy in single mode
}

// TestHandler_SelectProviderMultiMode tests selectProvider uses router.
func TestHandler_SelectProviderMultiMode(t *testing.T) {
	t.Parallel()

	provider1 := providers.NewAnthropicProvider("provider1", "https://api.anthropic.com")
	provider2 := providers.NewAnthropicProvider("provider2", "https://api.anthropic.com")

	providerInfos := []router.ProviderInfo{
		{Provider: provider1, IsHealthy: func() bool { return true }},
		{Provider: provider2, IsHealthy: func() bool { return true }},
	}

	mockR := &mockRouter{
		name: "test",
		selected: router.ProviderInfo{
			Provider:  provider2,
			IsHealthy: func() bool { return true },
		},
	}

	handler := newTestHandler(t, provider1, providerInfos, mockR, "test-key", nil, false, nil)

	info, err := handler.selectProvider(context.Background(), "", false)
	require.NoError(t, err)
	// Router should have selected provider2, not provider1
	assert.Equal(t, "provider2", info.Provider.Name())
}

// TestHandler_HealthHeaderWhenEnabled tests X-CC-Relay-Health debug header.
func TestHandler_HealthHeaderWhenEnabled(t *testing.T) {
	t.Parallel()

	// Create mock backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_123","type":"message"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	logger := zerolog.Nop()
	tracker := health.NewTracker(health.CircuitBreakerConfig{FailureThreshold: 5}, &logger)

	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: tracker.IsHealthyFunc("test")},
	}

	mockR := &mockRouter{
		name: "round_robin",
		selected: router.ProviderInfo{
			Provider:  provider,
			IsHealthy: tracker.IsHealthyFunc("test"),
		},
	}

	// routingDebug=true to enable X-CC-Relay-Health header
	handler, err := NewHandler(
		provider, providerInfos, mockR, "test-key", nil, nil, nil, nil,
		config.DebugOptions{}, true, tracker,
	)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should have health header
	healthHeader := rr.Header().Get("X-CC-Relay-Health")
	assert.NotEmpty(t, healthHeader)
	assert.Equal(t, "closed", healthHeader) // New provider circuit is closed (healthy)
}

// TestHandler_ReportOutcome_Success tests successful responses record to tracker.
func TestHandler_ReportOutcome_Success(t *testing.T) {
	t.Parallel()

	// Create mock backend that returns 200
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_123","type":"message"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	logger := zerolog.Nop()
	tracker := health.NewTracker(health.CircuitBreakerConfig{FailureThreshold: 2}, &logger)

	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: tracker.IsHealthyFunc("test")},
	}

	mockR := &mockRouter{
		name: "test",
		selected: router.ProviderInfo{
			Provider:  provider,
			IsHealthy: tracker.IsHealthyFunc("test"),
		},
	}

	handler, err := NewHandler(
		provider, providerInfos, mockR, "test-key", nil, nil, nil, nil,
		config.DebugOptions{}, true, tracker,
	)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Provider should still be healthy after successful request
	assert.True(t, tracker.IsHealthyFunc("test")())
}

// TestHandler_ReportOutcome_Failure5xx tests 5xx responses count as failures.
func TestHandler_ReportOutcome_Failure5xx(t *testing.T) {
	t.Parallel()

	// Create mock backend that returns 500
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test-500", backend.URL)
	logger := zerolog.Nop()
	// Low threshold to trigger circuit opening quickly
	tracker := health.NewTracker(health.CircuitBreakerConfig{FailureThreshold: 2}, &logger)

	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: tracker.IsHealthyFunc("test-500")},
	}

	mockR := &mockRouter{
		name: "test",
		selected: router.ProviderInfo{
			Provider:  provider,
			IsHealthy: tracker.IsHealthyFunc("test-500"),
		},
	}

	handler, err := NewHandler(
		provider, providerInfos, mockR, "test-key", nil, nil, nil, nil,
		config.DebugOptions{}, true, tracker,
	)
	require.NoError(t, err)

	// Make multiple requests to trip the circuit
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	}

	// After multiple 500s, circuit should be open (unhealthy)
	assert.False(t, tracker.IsHealthyFunc("test-500")())
}

// TestHandler_ReportOutcome_Failure429 tests rate limit responses count as failures.
func TestHandler_ReportOutcome_Failure429(t *testing.T) {
	t.Parallel()

	// Create mock backend that returns 429
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate_limited"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test-429", backend.URL)
	logger := zerolog.Nop()
	tracker := health.NewTracker(health.CircuitBreakerConfig{FailureThreshold: 2}, &logger)

	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: tracker.IsHealthyFunc("test-429")},
	}

	mockR := &mockRouter{
		name: "test",
		selected: router.ProviderInfo{
			Provider:  provider,
			IsHealthy: tracker.IsHealthyFunc("test-429"),
		},
	}

	handler, err := NewHandler(
		provider, providerInfos, mockR, "test-key", nil, nil, nil, nil,
		config.DebugOptions{}, true, tracker,
	)
	require.NoError(t, err)

	// Make multiple requests to trip the circuit
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	}

	// After multiple 429s, circuit should be open (unhealthy)
	assert.False(t, tracker.IsHealthyFunc("test-429")())
}

// TestHandler_ReportOutcome_4xxNotFailure tests 4xx (except 429) don't count as failures.
func TestHandler_ReportOutcome_4xxNotFailure(t *testing.T) {
	t.Parallel()

	// Create mock backend that returns 400
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad_request"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test-400", backend.URL)
	logger := zerolog.Nop()
	tracker := health.NewTracker(health.CircuitBreakerConfig{FailureThreshold: 2}, &logger)

	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: tracker.IsHealthyFunc("test-400")},
	}

	mockR := &mockRouter{
		name: "test",
		selected: router.ProviderInfo{
			Provider:  provider,
			IsHealthy: tracker.IsHealthyFunc("test-400"),
		},
	}

	handler, err := NewHandler(
		provider, providerInfos, mockR, "test-key", nil, nil, nil, nil,
		config.DebugOptions{}, true, tracker,
	)
	require.NoError(t, err)

	// Make multiple 400 requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	}

	// 400s should NOT trip the circuit - provider should remain healthy
	assert.True(t, tracker.IsHealthyFunc("test-400")())
}
