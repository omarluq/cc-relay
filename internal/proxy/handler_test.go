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

const (
	poolKey1               = "pool-key-1"
	localBaseURL           = "http://localhost:9999"
	initialKey             = "initial-key"
	testKey                = "test-key"
	testSingleKey          = "test-single-key"
	testProviderName       = "test-provider"
	anthropicVersionHeader = "Anthropic-Version"
	fallbackKey            = "fallback-key"
	test500ProviderName    = "test-500"
	test429ProviderName    = "test-429"
	test400ProviderName    = "test-400"
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
	handler, err := NewHandler(&HandlerOptions{
		Provider:       provider,
		ProviderInfos:  providerInfos,
		ProviderRouter: providerRouter,
		APIKey:         apiKey,
		Pool:           pool,
		DebugOptions:   config.DebugOptions{},
		RoutingDebug:   routingDebug,
		HealthTracker:  healthTracker,
	})
	require.NoError(t, err)
	return handler
}

func newHandlerWithAPIKey(t *testing.T, provider providers.Provider) *Handler {
	t.Helper()
	return newTestHandler(t, provider, nil, nil, testKey, nil, false, nil)
}

func serveJSONMessages(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	return serveJSONMessagesBody(t, handler, `{}`)
}

func newJSONMessagesRequest(body string) *http.Request {
	return newMessagesRequestWithHeaders(body,
		headerPair{key: contentTypeHeader, value: jsonContentType},
	)
}

func serveJSONMessagesBody(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serveRequest(t, handler, newJSONMessagesRequest(body))
}

func newKeyPool(t *testing.T, keys []keypool.KeyConfig) *keypool.KeyPool {
	t.Helper()
	pool, err := keypool.NewKeyPool(testProviderName, keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys:     keys,
	})
	require.NoError(t, err)
	return pool
}

func newHandlerWithPool(t *testing.T, provider providers.Provider, pool *keypool.KeyPool) *Handler {
	t.Helper()
	handler, err := NewHandler(&HandlerOptions{
		Provider: provider,
		Pool:     pool,
	})
	require.NoError(t, err)
	return handler
}

func serveMessages(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := newMessagesRequest(bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func newTrackedHandler(
	t *testing.T,
	providerName, backendURL, routerName string,
	failureThreshold int,
) (*Handler, *health.Tracker) {
	t.Helper()

	provider := newNamedProvider(providerName, backendURL)
	logger := zerolog.Nop()
	tracker := health.NewTracker(health.CircuitBreakerConfig{FailureThreshold: failureThreshold}, &logger)

	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: tracker.IsHealthyFunc(providerName)},
	}

	mockR := &mockRouter{
		name: routerName,
		selected: router.ProviderInfo{
			Provider:  provider,
			IsHealthy: tracker.IsHealthyFunc(providerName),
		},
	}

	handler, err := NewHandler(&HandlerOptions{
		Provider:       provider,
		ProviderInfos:  providerInfos,
		ProviderRouter: mockR,
		APIKey:         testKey,
		DebugOptions:   config.DebugOptions{},
		RoutingDebug:   true,
		HealthTracker:  tracker,
	})
	require.NoError(t, err)
	return handler, tracker
}

func TestNewHandlerValidProvider(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(anthropicBaseURL)
	handler := newHandlerWithAPIKey(t, provider)

	if handler == nil {
		t.Error("Expected non-nil handler")
	}
}

func TestNewHandlerInvalidURL(t *testing.T) {
	t.Parallel()
	// Create a mock provider with invalid URL
	provider := &mockProvider{baseURL: "://invalid-url"}

	_, err := NewHandler(&HandlerOptions{
		Provider: provider,
		APIKey:   testKey,
	})
	if err == nil {
		t.Error("Expected error for invalid base URL, got nil")
	}
}

func TestNewHandlerWithLiveProvidersNilProviderInfosFunc(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(anthropicBaseURL)

	// Should not panic with nil providerInfosFunc
	handler, err := NewHandlerWithLiveProviders(&HandlerOptions{
		Provider: provider,
		APIKey:   testKey,
		// nil ProviderInfosFunc - should be guarded
	})
	require.NoError(t, err)
	require.NotNil(t, handler)

	// Verify the handler works (single provider mode)
	assert.NotNil(t, handler.defaultProvider)
	assert.Len(t, handler.providerProxies, 1)
}

func TestHandlerForwardsAnthropicHeaders(t *testing.T) {
	t.Parallel()

	// Create mock backend that echoes headers
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for anthropic headers
		if r.Header.Get(anthropicVersionHeader) != anthropicVersion {
			t.Errorf("Expected Anthropic-Version header, got %q", r.Header.Get(anthropicVersionHeader))
		}

		if r.Header.Get("Anthropic-Beta") != "test-feature" {
			t.Errorf("Expected Anthropic-Beta header, got %q", r.Header.Get("Anthropic-Beta"))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Create provider pointing to mock backend
	provider := newTestProvider(backend.URL)
	handler := newHandlerWithAPIKey(t, provider)

	// Create request with anthropic headers
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: anthropicVersionHeader, value: anthropicVersion},
		headerPair{key: "Anthropic-Beta", value: "test-feature"},
	)
	w := serveRequest(t, handler, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlerHasErrorHandler(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(anthropicBaseURL)
	handler := newHandlerWithAPIKey(t, provider)

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

func TestHandlerStructureCorrect(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(anthropicBaseURL)
	handler := newHandlerWithAPIKey(t, provider)

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
	if pp.APIKey != testKey {
		t.Errorf("provider proxy APIKey = %q, want %q", pp.APIKey, testKey)
	}
}

func TestHandlerPreservesToolUseId(t *testing.T) {
	t.Parallel()

	// Create mock backend that echoes request body
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		// Echo the body back
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer backend.Close()

	// Create provider pointing to mock backend
	provider := newTestProvider(backend.URL)

	handler, err := NewHandler(&HandlerOptions{
		Provider: provider,
		APIKey:   testKey,
	})
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	// Request body with tool_use_id
	requestBody := `{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"test"}],` +
		`"tools":[{"name":"test","input_schema":{}}],` +
		`"tool_choice":{"type":"tool","name":"test","tool_use_id":"toolu_123"}}`

	// Create request
	req := newMessagesRequestWithHeaders(requestBody,
		headerPair{key: contentTypeHeader, value: jsonContentType},
	)
	w := serveRequest(t, handler, req)

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

	headers.Set(contentTypeHeader, jsonContentType)

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

func (m *mockProvider) TransformRequest(body []byte, endpoint string) (newBody []byte, targetURL string, err error) {
	return body, m.baseURL + endpoint, nil
}

func (m *mockProvider) TransformResponse(_ *http.Response, _ http.ResponseWriter) error {
	return nil
}

func (m *mockProvider) RequiresBodyTransform() bool {
	return false
}

func (m *mockProvider) StreamingContentType() string {
	return providers.ContentTypeSSE
}

// TestHandler_WithKeyPool tests handler with key pool integration.
func TestHandlerWithKeyPool(t *testing.T) {
	t.Parallel()

	// Create mock backend
	backend := newJSONBackend(t, `{"id":"test","type":"message"}`)

	// Create key pool with test keys
	pool := newKeyPool(t, []keypool.KeyConfig{
		{APIKey: "test-key-1", RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
		{APIKey: "test-key-2", RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
	})

	// Create handler with key pool
	provider := newTestProvider(backend.URL)
	handler := newHandlerWithPool(t, provider, pool)
	w := serveMessages(t, handler)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify x-cc-relay-* headers are set
	assert.NotEmpty(t, w.Header().Get(HeaderRelayKeyID))
	assert.Equal(t, "2", w.Header().Get(HeaderRelayKeysTotal))
	assert.Equal(t, "2", w.Header().Get(HeaderRelayKeysAvail))
}

// TestHandler_AllKeysExhausted tests 429 response when all keys exhausted.
func TestHandlerAllKeysExhausted(t *testing.T) {
	t.Parallel()

	// Create key pool with single key and very low limit
	pool := newKeyPool(t, []keypool.KeyConfig{
		{APIKey: testKey, RPMLimit: 1, ITPMLimit: 1, OTPMLimit: 1},
	})

	// Exhaust the key by making a request
	_, _, err := pool.GetKey(context.Background())
	require.NoError(t, err)

	// Create handler
	provider := newTestProvider(anthropicBaseURL)
	handler := newHandlerWithPool(t, provider, pool)
	w := serveMessages(t, handler)

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
func TestHandlerKeyPoolUpdate(t *testing.T) {
	t.Parallel()

	// Create mock backend that returns rate limit headers
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("anthropic-ratelimit-requests-limit", "100")
		w.Header().Set("anthropic-ratelimit-requests-remaining", "99")
		w.Header().Set("anthropic-ratelimit-input-tokens-limit", "50000")
		w.Header().Set("anthropic-ratelimit-input-tokens-remaining", "49000")
		w.Header().Set("anthropic-ratelimit-output-tokens-limit", "20000")
		w.Header().Set("anthropic-ratelimit-output-tokens-remaining", "19000")
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Create key pool
	pool := newKeyPool(t, []keypool.KeyConfig{
		{APIKey: testKey, RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
	})

	// Create handler
	provider := newTestProvider(backend.URL)
	handler := newHandlerWithPool(t, provider, pool)
	w := serveMessages(t, handler)

	// Verify response OK
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify key state was updated (check via stats)
	stats := pool.GetStats()
	assert.Equal(t, 1, stats.TotalKeys)
	assert.Equal(t, 1, stats.AvailableKeys)
}

// TestHandler_Backend429 tests that handler marks key exhausted on backend 429.
func TestHandlerBackend429(t *testing.T) {
	t.Parallel()

	// Create mock backend that returns 429
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"rate limit"}}`))
	}))
	defer backend.Close()

	// Create key pool
	pool := newKeyPool(t, []keypool.KeyConfig{
		{APIKey: testKey, RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
	})

	// Create handler
	provider := newTestProvider(backend.URL)
	handler := newHandlerWithPool(t, provider, pool)
	w := serveMessages(t, handler)

	// Verify 429 is passed through
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Wait a bit for async update
	time.Sleep(10 * time.Millisecond)

	// Verify key is marked as exhausted (all keys should be unavailable)
	stats := pool.GetStats()
	assert.Equal(t, 1, stats.ExhaustedKeys)
}

// TestHandler_SingleKeyMode tests backwards compatibility with nil pool.
func TestHandlerSingleKeyMode(t *testing.T) {
	t.Parallel()

	// Create mock backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header uses single key
		assert.Equal(t, testSingleKey, r.Header.Get("X-Api-Key"))
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Create handler without key pool (nil)
	provider := newTestProvider(backend.URL)
	handler, err := NewHandler(&HandlerOptions{
		Provider: provider,
		APIKey:   testSingleKey,
	})
	require.NoError(t, err)

	// Make request
	req := newMessagesRequestWithHeaders("{}")
	w := serveRequest(t, handler, req)

	// Verify response OK
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify no x-cc-relay-* headers (single key mode)
	assert.Empty(t, w.Header().Get(HeaderRelayKeyID))
	assert.Empty(t, w.Header().Get(HeaderRelayKeysTotal))
}

// TestHandler_UsesFallbackKeyWhenNoClientAuth tests that configured provider keys
// are used when client provides no auth headers.
func TestHandlerUsesFallbackKeyWhenNoClientAuth(t *testing.T) {
	t.Parallel()

	var receivedAuthHeader string
	var receivedAPIKeyHeader string

	// Create mock backend that captures headers
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		receivedAPIKeyHeader = r.Header.Get("x-api-key")

		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := newTestProvider(backend.URL)
	handler, err := NewHandler(&HandlerOptions{
		Provider: provider,
		APIKey:   "our-fallback-key",
	})
	require.NoError(t, err)

	// Create request WITHOUT any auth headers
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: anthropicVersionHeader, value: anthropicVersion2024},
	)
	// NO Authorization, NO x-api-key

	w := serveRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Client Authorization should be empty (none provided)
	assert.Empty(t, receivedAuthHeader)

	// Our fallback key should be used
	assert.Equal(t, "our-fallback-key", receivedAPIKeyHeader)
}

// TestHandler_ForwardsClientAuthWhenPresent tests that client Authorization header
// is forwarded unchanged when present (transparent proxy mode).
func TestHandlerForwardsClientAuthWhenPresent(t *testing.T) {
	t.Parallel()

	var receivedAuthHeader string
	var receivedAPIKeyHeader string

	// Create mock backend that captures headers
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		receivedAPIKeyHeader = r.Header.Get("x-api-key")

		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Create handler with a configured fallback key
	provider := newTestProvider(backend.URL)
	handler, err := NewHandler(&HandlerOptions{
		Provider: provider,
		APIKey:   fallbackKey,
	})
	require.NoError(t, err)

	// Create request WITH client Authorization header
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: "Authorization", value: "Bearer sub_12345"},
		headerPair{key: anthropicVersionHeader, value: anthropicVersion2024},
	)
	w := serveRequest(t, handler, req)

	// Verify response OK
	assert.Equal(t, http.StatusOK, w.Code)

	// CRITICAL: Client Authorization header should be forwarded UNCHANGED
	assert.Equal(t, "Bearer sub_12345", receivedAuthHeader)

	// Our fallback key should NOT be added
	assert.Empty(t, receivedAPIKeyHeader, "fallback key should not be added when client has auth")
}

// TestHandler_ForwardsClientAPIKeyWhenPresent tests that client x-api-key header
// is forwarded unchanged when present (transparent proxy mode).
func TestHandlerForwardsClientAPIKeyWhenPresent(t *testing.T) {
	t.Parallel()

	var receivedAuthHeader string
	var receivedAPIKeyHeader string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		receivedAPIKeyHeader = r.Header.Get("x-api-key")

		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := newTestProvider(backend.URL)
	handler, err := NewHandler(&HandlerOptions{
		Provider: provider,
		APIKey:   fallbackKey,
	})
	require.NoError(t, err)

	// Create request WITH client x-api-key header
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: "x-api-key", value: "sk-ant-client-key"},
		headerPair{key: anthropicVersionHeader, value: anthropicVersion2024},
	)
	w := serveRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Client x-api-key should be forwarded UNCHANGED
	assert.Equal(t, "sk-ant-client-key", receivedAPIKeyHeader)

	// No Authorization header should be added
	assert.Empty(t, receivedAuthHeader)
}

// TestHandler_TransparentModeSkipsKeyPool tests that key pool is skipped
// when client provides auth (rate limiting is their problem).
func TestHandlerTransparentModeSkipsKeyPool(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Create key pool with test keys
	pool, err := keypool.NewKeyPool(testProviderName, keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			{APIKey: poolKey1, RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
		},
	})
	require.NoError(t, err)

	provider := newTestProvider(backend.URL)
	handler, err := NewHandler(&HandlerOptions{
		Provider: provider,
		Pool:     pool,
	})
	require.NoError(t, err)

	// Create request WITH client auth
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: "Authorization", value: "Bearer client-token"},
	)
	w := serveRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// x-cc-relay-* headers should NOT be set (key pool was skipped)
	assert.Empty(t, w.Header().Get(HeaderRelayKeyID), "key pool should be skipped in transparent mode")
	assert.Empty(t, w.Header().Get(HeaderRelayKeysTotal))
}

// TestHandler_FallbackModeUsesKeyPool tests that key pool is used
// when client provides no auth.
func TestHandlerFallbackModeUsesKeyPool(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	pool, err := keypool.NewKeyPool(testProviderName, keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			{APIKey: poolKey1, RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000},
		},
	})
	require.NoError(t, err)

	provider := newTestProvider(backend.URL)
	handler, err := NewHandler(&HandlerOptions{
		Provider: provider,
		Pool:     pool,
	})
	require.NoError(t, err)

	// Create request WITHOUT client auth
	req := newMessagesRequestWithHeaders("{}")
	// NO Authorization, NO x-api-key
	w := serveRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// x-cc-relay-* headers SHOULD be set (key pool was used)
	assert.NotEmpty(t, w.Header().Get(HeaderRelayKeyID), "key pool should be used in fallback mode")
	assert.Equal(t, "1", w.Header().Get(HeaderRelayKeysTotal))
}

// TestHandler_TransparentModeForwardsAnthropicHeaders tests that anthropic-* headers
// are forwarded in transparent mode.
func TestHandlerTransparentModeForwardsAnthropicHeaders(t *testing.T) {
	t.Parallel()

	var receivedVersion string
	var receivedBeta string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedVersion = r.Header.Get(anthropicVersionHeader)
		receivedBeta = r.Header.Get("Anthropic-Beta")

		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := newTestProvider(backend.URL)
	handler, err := NewHandler(&HandlerOptions{
		Provider: provider,
		APIKey:   fallbackKey,
	})
	require.NoError(t, err)

	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: "Authorization", value: "Bearer client-token"},
		headerPair{key: anthropicVersionHeader, value: anthropicVersion2024},
		headerPair{key: "Anthropic-Beta", value: anthropicBetaExtendedThinking},
	)
	w := serveRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, anthropicVersion2024, receivedVersion)
	assert.Equal(t, anthropicBetaExtendedThinking, receivedBeta)
}

// TestHandler_NonTransparentProviderUsesConfiguredKeys tests that providers
// that don't support transparent auth (like Z.AI) use configured keys even
// when client sends Authorization header.
func TestHandlerNonTransparentProviderUsesConfiguredKeys(t *testing.T) {
	t.Parallel()

	var receivedAPIKey string
	var receivedAuthHeader string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("x-api-key")
		receivedAuthHeader = r.Header.Get("Authorization")

		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Z.AI provider does NOT support transparent auth
	provider := providers.NewZAIProvider("test-zai", backend.URL)
	handler := newTestHandler(t, provider, nil, nil, "zai-configured-key", nil, false, nil)

	// Client sends Authorization header (like Claude Code does)
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: "Authorization", value: "Bearer client-anthropic-token"},
		headerPair{key: anthropicVersionHeader, value: anthropicVersion2024},
	)
	w := serveRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// CRITICAL: Client Authorization should NOT be forwarded to Z.AI
	// Instead, our configured x-api-key should be used
	assert.Empty(t, receivedAuthHeader, "client Authorization should not be forwarded to non-transparent provider")
	assert.Equal(t, "zai-configured-key", receivedAPIKey, "configured key should be used for Z.AI")
}

// TestHandler_NonTransparentProviderWithKeyPool tests that non-transparent providers
// use key pool even when client sends auth headers.
func TestHandlerNonTransparentProviderWithKeyPool(t *testing.T) {
	t.Parallel()

	var receivedAPIKey string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("x-api-key")

		w.Header().Set(contentTypeHeader, jsonContentType)
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
	handler, err := NewHandler(&HandlerOptions{
		Provider: provider,
		Pool:     pool,
	})
	require.NoError(t, err)

	// Client sends Authorization header
	req := newMessagesRequestWithHeaders("{}",
		headerPair{key: "Authorization", value: "Bearer client-anthropic-token"},
	)
	w := serveRequest(t, handler, req)

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
func TestHandlerSingleProviderMode(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := newTestProvider(backend.URL)
	// No router (nil), no providers list (nil) - single provider mode
	handler, err := NewHandler(&HandlerOptions{
		Provider: provider,
		APIKey:   testKey,
	})
	require.NoError(t, err)

	req := newMessagesRequestWithHeaders("{}")
	w := serveRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// No routing debug headers in single provider mode
	assert.Empty(t, w.Header().Get("X-CC-Relay-Strategy"))
	assert.Empty(t, w.Header().Get("X-CC-Relay-Provider"))
}

// TestHandler_MultiProviderModeUsesRouter tests that handler uses router for selection.
func TestHandlerMultiProviderModeUsesRouter(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider1 := newNamedProvider("provider1", backend.URL)
	provider2 := newNamedProvider("provider2", backend.URL)

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
	handler := newTestHandler(t, provider1, providerInfos, mockR, testKey, nil, true, nil)

	req := newMessagesRequestWithHeaders("{}")
	w := serveRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Debug headers should be present
	assert.Equal(t, "test_strategy", w.Header().Get("X-CC-Relay-Strategy"))
	assert.Equal(t, "provider2", w.Header().Get("X-CC-Relay-Provider"))
}

func TestHandlerLazyProxyForNewProvider(t *testing.T) {
	t.Parallel()

	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"a"}`))
	}))
	defer backendA.Close()

	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"provider":"b"}`))
	}))
	defer backendB.Close()

	providerA := newNamedProvider("provider-a", backendA.URL)
	providerB := newNamedProvider("provider-b", backendB.URL)

	infos := []router.ProviderInfo{
		{Provider: providerA, IsHealthy: func() bool { return true }},
	}
	providerInfosFunc := func() []router.ProviderInfo { return infos }

	mockR := &mockRouter{
		name:     "mock",
		selected: router.ProviderInfo{Provider: providerB, IsHealthy: func() bool { return true }},
	}

	handler, err := NewHandlerWithLiveProviders(&HandlerOptions{
		Provider:          providerA,
		ProviderInfosFunc: providerInfosFunc,
		ProviderRouter:    mockR,
		APIKey:            testKey,
		DebugOptions:      config.DebugOptions{},
		RoutingDebug:      false,
	})
	require.NoError(t, err)

	// Simulate reload: provider B becomes enabled
	infos = []router.ProviderInfo{
		{Provider: providerA, IsHealthy: func() bool { return true }},
		{Provider: providerB, IsHealthy: func() bool { return true }},
	}

	req := newMessagesRequestWithHeaders(`{"model":"test","messages":[]}`)
	rec := serveRequest(t, handler, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"provider":"b"`)
}

// TestHandler_DebugHeadersDisabledByDefault tests that debug headers are not added when disabled.
func TestHandlerDebugHeadersDisabledByDefault(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := newTestProvider(backend.URL)
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
	handler := newTestHandler(t, provider, providerInfos, mockR, testKey, nil, false, nil)

	req := newMessagesRequestWithHeaders("{}")
	w := serveRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// No debug headers
	assert.Empty(t, w.Header().Get("X-CC-Relay-Strategy"))
	assert.Empty(t, w.Header().Get("X-CC-Relay-Provider"))
}

// TestHandler_DebugHeadersWhenEnabled tests debug headers are added when routing.debug=true.
func TestHandlerDebugHeadersWhenEnabled(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := newNamedProvider(testProviderName, backend.URL)
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

	handler := newTestHandler(t, provider, providerInfos, mockR, testKey, nil, true, nil)

	req := newMessagesRequestWithHeaders("{}")
	w := serveRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Debug headers present
	assert.Equal(t, "round_robin", w.Header().Get("X-CC-Relay-Strategy"))
	assert.Equal(t, testProviderName, w.Header().Get("X-CC-Relay-Provider"))
}

// TestHandler_RouterSelectionError tests error handling when router fails.
func TestHandlerRouterSelectionError(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(anthropicBaseURL)
	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: func() bool { return false }},
	}

	// Mock router that returns error
	mockR := &mockRouter{
		name: "failover",
		err:  router.ErrAllProvidersUnhealthy,
	}

	handler := newTestHandler(t, provider, providerInfos, mockR, testKey, nil, false, nil)

	req := newMessagesRequestWithHeaders("{}")
	w := serveRequest(t, handler, req)

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
func TestHandlerSelectProviderSingleMode(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(anthropicBaseURL)

	// No router, no providers - single provider mode
	handler, err := NewHandler(&HandlerOptions{
		Provider: provider,
		APIKey:   testKey,
	})
	require.NoError(t, err)

	info, err := handler.selectProvider(context.Background(), "", false)
	require.NoError(t, err)
	assert.Equal(t, "test", info.Provider.Name())
	assert.True(t, info.Healthy()) // Always healthy in single mode
}

// TestHandler_SelectProviderMultiMode tests selectProvider uses router.
func TestHandlerSelectProviderMultiMode(t *testing.T) {
	t.Parallel()

	provider1 := newNamedProvider("provider1", anthropicBaseURL)
	provider2 := newNamedProvider("provider2", anthropicBaseURL)

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

	handler := newTestHandler(t, provider1, providerInfos, mockR, testKey, nil, false, nil)

	info, err := handler.selectProvider(context.Background(), "", false)
	require.NoError(t, err)
	// Router should have selected provider2, not provider1
	assert.Equal(t, "provider2", info.Provider.Name())
}

// TestHandler_HealthHeaderWhenEnabled tests X-CC-Relay-Health debug header.
func TestHandlerHealthHeaderWhenEnabled(t *testing.T) {
	t.Parallel()

	// Create mock backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_123","type":"message"}`))
	}))
	defer backend.Close()

	handler, _ := newTrackedHandler(t, "test", backend.URL, "round_robin", 5)
	rr := serveJSONMessages(t, handler)

	// Should have health header
	healthHeader := rr.Header().Get("X-CC-Relay-Health")
	assert.NotEmpty(t, healthHeader)
	assert.Equal(t, "closed", healthHeader) // New provider circuit is closed (healthy)
}

// TestHandler_ReportOutcome_Success tests successful responses record to tracker.
func TestHandlerReportOutcomeSuccess(t *testing.T) {
	t.Parallel()

	backend := newStatusBackend(t, http.StatusOK, `{"id":"msg_123","type":"message"}`, nil)

	handler, tracker := newTrackedHandler(t, "test", backend.URL, "test", 2)
	rr := serveJSONMessages(t, handler)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Provider should still be healthy after successful request
	assert.True(t, tracker.IsHealthyFunc("test")())
}

// TestHandler_ReportOutcome_Failure5xx tests 5xx responses count as failures.
func TestHandlerReportOutcomeFailure5xx(t *testing.T) {
	t.Parallel()

	backend := newStatusBackend(t, http.StatusInternalServerError, `{"error":"internal"}`, nil)

	// Low threshold to trigger circuit opening quickly
	handler, tracker := newTrackedHandler(t, test500ProviderName, backend.URL, "test", 2)

	// Make multiple requests to trip the circuit
	for i := 0; i < 3; i++ {
		rr := serveJSONMessages(t, handler)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	}

	// After multiple 500s, circuit should be open (unhealthy)
	assert.False(t, tracker.IsHealthyFunc(test500ProviderName)())
}

// TestHandler_ReportOutcome_Failure429 tests rate limit responses count as failures.
func TestHandlerReportOutcomeFailure429(t *testing.T) {
	t.Parallel()

	backend := newStatusBackend(t, http.StatusTooManyRequests, `{"error":"rate_limited"}`, map[string]string{
		"Retry-After": "60",
	})

	handler, tracker := newTrackedHandler(t, test429ProviderName, backend.URL, "test", 2)

	// Make multiple requests to trip the circuit
	for i := 0; i < 3; i++ {
		rr := serveJSONMessages(t, handler)
		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	}

	// After multiple 429s, circuit should be open (unhealthy)
	assert.False(t, tracker.IsHealthyFunc(test429ProviderName)())
}

// TestHandler_ReportOutcome_4xxNotFailure tests 4xx (except 429) don't count as failures.
func TestHandlerReportOutcome4xxNotFailure(t *testing.T) {
	t.Parallel()

	backend := newStatusBackend(t, http.StatusBadRequest, `{"error":"bad_request"}`, nil)

	handler, tracker := newTrackedHandler(t, test400ProviderName, backend.URL, "test", 2)

	// Make multiple 400 requests
	for i := 0; i < 5; i++ {
		rr := serveJSONMessages(t, handler)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	}

	// 400s should NOT trip the circuit - provider should remain healthy
	assert.True(t, tracker.IsHealthyFunc(test400ProviderName)())
}

// trackingRouter records the providers it receives for selection.
type trackingRouter struct {
	name              string
	receivedProviders []string
}

func (r *trackingRouter) Select(_ context.Context, provs []router.ProviderInfo) (router.ProviderInfo, error) {
	r.receivedProviders = append(r.receivedProviders, "")
	for i, p := range provs {
		if i == 0 {
			r.receivedProviders[len(r.receivedProviders)-1] = p.Provider.Name()
		}
	}
	if len(provs) == 0 {
		return router.ProviderInfo{}, router.ErrNoProviders
	}
	return provs[0], nil
}

func (r *trackingRouter) Name() string {
	return r.name
}

// TestHandler_ThinkingAffinity_UsesConsistentProvider tests that thinking requests use deterministic selection.
func TestHandlerThinkingAffinityUsesConsistentProvider(t *testing.T) {
	t.Parallel()

	// Create two mock backends
	backend1 := newJSONBackend(t, `{"id":"test1"}`)
	backend2 := newJSONBackend(t, `{"id":"test2"}`)

	provider1 := newNamedProvider("provider1", backend1.URL)
	provider2 := newNamedProvider("provider2", backend2.URL)

	providerInfos := []router.ProviderInfo{
		{Provider: provider1, IsHealthy: func() bool { return true }},
		{Provider: provider2, IsHealthy: func() bool { return true }},
	}

	tracker := &trackingRouter{name: "tracking"}

	handler, err := NewHandler(&HandlerOptions{
		Provider:       provider1,
		ProviderInfos:  providerInfos,
		ProviderRouter: tracker,
		APIKey:         testKey,
		ProviderPools:  map[string]*keypool.KeyPool{"provider1": nil, "provider2": nil},
		ProviderKeys:   map[string]string{"provider1": "key1", "provider2": "key2"},
		DebugOptions:   config.DebugOptions{},
		RoutingDebug:   true,
	})
	require.NoError(t, err)

	// Request body with thinking signature
	thinkingBody := `{
		"model": "claude-sonnet-4-20250514",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Hello"}]},
			{
				"role": "assistant",
				"content": [
					{"type": "thinking", "thinking": "...", "signature": "sig123xyz"}
				]
			},
			{"role": "user", "content": [{"type": "text", "text": "Continue"}]}
		]
	}`

	// Make multiple requests with thinking - should always get first healthy provider
	for i := 0; i < 5; i++ {
		rr := serveJSONMessagesBody(t, handler, thinkingBody)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// All requests should have received only 1 provider (the first healthy one)
	// since thinking affinity reduces candidates to [:1]
	for i, provName := range tracker.receivedProviders {
		assert.Equal(t, "provider1", provName, "request %d should use provider1", i)
	}

	// Check thinking affinity header is set
	rr := serveJSONMessagesBody(t, handler, thinkingBody)
	assert.Equal(t, "true", rr.Header().Get("X-CC-Relay-Thinking-Affinity"))
}

// TestHandler_ThinkingAffinity_FallsBackToSecondProvider tests fallback when first provider unhealthy.
func TestHandlerThinkingAffinityFallsBackToSecondProvider(t *testing.T) {
	t.Parallel()

	backend1 := newJSONBackend(t, `{"id":"test1"}`)
	backend2 := newJSONBackend(t, `{"id":"test2"}`)

	provider1 := newNamedProvider("provider1", backend1.URL)
	provider2 := newNamedProvider("provider2", backend2.URL)

	// Provider1 is unhealthy, provider2 is healthy
	providerInfos := []router.ProviderInfo{
		{Provider: provider1, IsHealthy: func() bool { return false }}, // UNHEALTHY
		{Provider: provider2, IsHealthy: func() bool { return true }},
	}

	tracker := &trackingRouter{name: "tracking"}

	handler, err := NewHandler(&HandlerOptions{
		Provider:       provider1,
		ProviderInfos:  providerInfos,
		ProviderRouter: tracker,
		APIKey:         testKey,
		ProviderPools:  map[string]*keypool.KeyPool{"provider1": nil, "provider2": nil},
		ProviderKeys:   map[string]string{"provider1": "key1", "provider2": "key2"},
		DebugOptions:   config.DebugOptions{},
		RoutingDebug:   false,
	})
	require.NoError(t, err)

	// Request with thinking signature
	thinkingBody := `{
		"messages": [
			{"role": "assistant", "content": [{"type": "thinking", "signature": "sig123"}]}
		]
	}`

	rr := serveJSONMessagesBody(t, handler, thinkingBody)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Should use provider2 (first healthy after filtering)
	assert.Equal(t, "provider2", tracker.receivedProviders[0])
}

// TestHandler_NoThinking_UsesNormalRouting tests non-thinking requests use normal routing.
func TestHandlerNoThinkingUsesNormalRouting(t *testing.T) {
	t.Parallel()

	backend1 := newJSONBackend(t, `{"id":"test1"}`)
	backend2 := newJSONBackend(t, `{"id":"test2"}`)

	provider1 := newNamedProvider("provider1", backend1.URL)
	provider2 := newNamedProvider("provider2", backend2.URL)

	providerInfos := []router.ProviderInfo{
		{Provider: provider1, IsHealthy: func() bool { return true }},
		{Provider: provider2, IsHealthy: func() bool { return true }},
	}

	// Tracker that counts how many providers were passed
	callCount := 0
	providerCounts := []int{}
	countingRouter := &countingMockRouter{
		name:           "counting",
		providerCounts: &providerCounts,
		callCount:      &callCount,
		fallbackResult: providerInfos[0],
	}

	handler, err := NewHandler(&HandlerOptions{
		Provider:       provider1,
		ProviderInfos:  providerInfos,
		ProviderRouter: countingRouter,
		APIKey:         testKey,
		ProviderPools:  map[string]*keypool.KeyPool{"provider1": nil, "provider2": nil},
		ProviderKeys:   map[string]string{"provider1": "key1", "provider2": "key2"},
		DebugOptions:   config.DebugOptions{},
		RoutingDebug:   true,
	})
	require.NoError(t, err)

	// Request WITHOUT thinking signature (normal text conversation)
	noThinkingBody := `{
		"model": "claude-sonnet-4-20250514",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Hello"}]},
			{"role": "assistant", "content": [{"type": "text", "text": "Hi!"}]},
			{"role": "user", "content": [{"type": "text", "text": "Continue"}]}
		]
	}`

	// Make request without thinking
	rr := serveJSONMessagesBody(t, handler, noThinkingBody)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Should receive ALL providers (2), not just 1
	require.Len(t, providerCounts, 1)
	assert.Equal(t, 2, providerCounts[0], "non-thinking request should receive all 2 providers")

	// No thinking affinity header should be set
	assert.Empty(t, rr.Header().Get("X-CC-Relay-Thinking-Affinity"))
}

// countingMockRouter counts how many providers are passed to Select.
type countingMockRouter struct {
	name           string
	providerCounts *[]int
	callCount      *int
	fallbackResult router.ProviderInfo
}

func (r *countingMockRouter) Select(
	_ context.Context, providerInfos []router.ProviderInfo,
) (router.ProviderInfo, error) {
	*r.callCount++
	*r.providerCounts = append(*r.providerCounts, len(providerInfos))
	if len(providerInfos) == 0 {
		return router.ProviderInfo{}, router.ErrNoProviders
	}
	return providerInfos[0], nil
}

func (r *countingMockRouter) Name() string {
	return r.name
}

// TestHandler_GetOrCreateProxy_KeyMatching tests the key-matching logic in getOrCreateProxy
// without requiring network listeners. Validates behavior for nil key maps and single-provider mode.
func TestHandlerGetOrCreateProxyKeyMatching(t *testing.T) {
	t.Parallel()

	t.Run("nil_keys_map_preserves_existing_proxy", func(t *testing.T) {
		t.Parallel()

		// Create provider with valid URL (required for proxy creation)
		prov := &mockProvider{baseURL: localBaseURL}

		// Create handler with a key but nil maps (single-provider mode)
		handler, err := NewHandler(&HandlerOptions{
			Provider:     prov,
			APIKey:       initialKey,
			DebugOptions: config.DebugOptions{},
		})
		require.NoError(t, err)

		// Get proxy - should have the initial key
		pp1, err := handler.getOrCreateProxy(prov)
		require.NoError(t, err)
		assert.Equal(t, initialKey, pp1.APIKey)

		// Get proxy again - with nil keys map, should return same proxy
		pp2, err := handler.getOrCreateProxy(prov)
		require.NoError(t, err)
		assert.Same(t, pp1, pp2, "should return same proxy instance")
		assert.Equal(t, initialKey, pp2.APIKey)
	})

	t.Run("nil_pools_map_preserves_existing_proxy", func(t *testing.T) {
		t.Parallel()

		prov := &mockProvider{baseURL: localBaseURL}
		pool, err := keypool.NewKeyPool(testProviderName, keypool.PoolConfig{
			Strategy: "least_loaded",
			Keys:     []keypool.KeyConfig{{APIKey: poolKey1}},
		})
		require.NoError(t, err)

		// Create handler with a pool but nil pools map (single-provider mode)
		handler, err := NewHandler(&HandlerOptions{
			Provider:     prov,
			Pool:         pool,
			DebugOptions: config.DebugOptions{},
		})
		require.NoError(t, err)

		pp1, err := handler.getOrCreateProxy(prov)
		require.NoError(t, err)
		assert.Same(t, pool, pp1.KeyPool)

		// Get proxy again - should return same proxy
		pp2, err := handler.getOrCreateProxy(prov)
		require.NoError(t, err)
		assert.Same(t, pp1, pp2)
		assert.Same(t, pool, pp2.KeyPool)
	})

	t.Run("multi_provider_mode_detects_key_change", func(t *testing.T) {
		t.Parallel()

		prov := &mockProvider{baseURL: localBaseURL}
		provName := prov.Name()

		// Create handler with keys map (multi-provider mode)
		keysMap := map[string]string{provName: "key-v1"}
		handler, err := NewHandler(&HandlerOptions{
			Provider:     prov,
			ProviderKeys: keysMap,
			DebugOptions: config.DebugOptions{},
		})
		require.NoError(t, err)

		pp1, err := handler.getOrCreateProxy(prov)
		require.NoError(t, err)
		assert.Equal(t, "key-v1", pp1.APIKey)

		// Simulate hot-reload: change key in map
		keysMap[provName] = "key-v2"

		// Get proxy again - should create new proxy with new key
		pp2, err := handler.getOrCreateProxy(prov)
		require.NoError(t, err)
		assert.NotSame(t, pp1, pp2, "should create new proxy after key change")
		assert.Equal(t, "key-v2", pp2.APIKey)
	})

	t.Run("multi_provider_mode_detects_pool_change", func(t *testing.T) {
		t.Parallel()

		prov := &mockProvider{baseURL: localBaseURL}
		provName := prov.Name()

		pool1, err := keypool.NewKeyPool(testProviderName, keypool.PoolConfig{
			Strategy: "least_loaded",
			Keys:     []keypool.KeyConfig{{APIKey: poolKey1}},
		})
		require.NoError(t, err)
		pool2, err := keypool.NewKeyPool(testProviderName, keypool.PoolConfig{
			Strategy: "least_loaded",
			Keys:     []keypool.KeyConfig{{APIKey: "pool-key-2"}},
		})
		require.NoError(t, err)

		// Create handler with pools map (multi-provider mode)
		poolsMap := map[string]*keypool.KeyPool{provName: pool1}
		handler, err := NewHandler(&HandlerOptions{
			Provider:      prov,
			ProviderPools: poolsMap,
			DebugOptions:  config.DebugOptions{},
		})
		require.NoError(t, err)

		pp1, err := handler.getOrCreateProxy(prov)
		require.NoError(t, err)
		assert.Same(t, pool1, pp1.KeyPool)

		// Simulate hot-reload: change pool in map
		poolsMap[provName] = pool2

		// Get proxy again - should create new proxy with new pool
		pp2, err := handler.getOrCreateProxy(prov)
		require.NoError(t, err)
		assert.NotSame(t, pp1, pp2, "should create new proxy after pool change")
		assert.Same(t, pool2, pp2.KeyPool)
	})

	t.Run("provider_baseurl_change_creates_new_proxy", func(t *testing.T) {
		t.Parallel()

		prov1 := &mockProvider{baseURL: localBaseURL}
		prov2 := &mockProvider{baseURL: "http://localhost:8888"} // Different URL, same name

		handler, err := NewHandler(&HandlerOptions{
			Provider:     prov1,
			APIKey:       testKey,
			DebugOptions: config.DebugOptions{},
		})
		require.NoError(t, err)

		pp1, err := handler.getOrCreateProxy(prov1)
		require.NoError(t, err)
		assert.Equal(t, localBaseURL, pp1.Provider.BaseURL())

		// Request proxy with different baseURL provider
		pp2, err := handler.getOrCreateProxy(prov2)
		require.NoError(t, err)
		assert.NotSame(t, pp1, pp2, "should create new proxy for different baseURL")
		assert.Equal(t, "http://localhost:8888", pp2.Provider.BaseURL())
	})

	t.Run("live_keys_func_used_over_static_map", func(t *testing.T) {
		t.Parallel()

		prov := &mockProvider{baseURL: localBaseURL}
		provName := prov.Name()

		// Static map has old key
		staticKeys := map[string]string{provName: "static-key"}

		// Live func returns new key (using a channel to allow updates)
		liveKey := make(chan string, 1)
		liveKey <- "live-key-v1"
		liveKeysFunc := func() map[string]string {
			select {
			case k := <-liveKey:
				liveKey <- k // Put it back for next read
				return map[string]string{provName: k}
			default:
				return map[string]string{provName: "fallback"}
			}
		}

		handler, err := NewHandlerWithLiveProviders(&HandlerOptions{
			Provider:     prov,
			ProviderKeys: staticKeys,
			DebugOptions: config.DebugOptions{},
			RoutingDebug: false,
		})
		require.NoError(t, err)

		// Set the live keys func
		handler.getProviderKeys = liveKeysFunc

		pp1, err := handler.getOrCreateProxy(prov)
		require.NoError(t, err)
		assert.Equal(t, "live-key-v1", pp1.APIKey, "should use live func, not static map")

		// Update live key
		<-liveKey // Drain
		liveKey <- "live-key-v2"

		// Get proxy again - should detect change from live func
		pp2, err := handler.getOrCreateProxy(prov)
		require.NoError(t, err)
		assert.NotSame(t, pp1, pp2)
		assert.Equal(t, "live-key-v2", pp2.APIKey)
	})
}
