package proxy_test

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
	"github.com/omarluq/cc-relay/internal/proxy"
	"github.com/omarluq/cc-relay/internal/router"
)

const (
	poolKey1               = "pool-key-1"
	localBaseURL           = "http://localhost:9999"
	initialKey             = "initial-key"
	testKey                = "test-key"
	testSingleKey          = "test-single-key"
	testProviderName       = "test-provider"
	providerAName          = "provider-a"
	providerBName          = "provider-b"
	anthropicVersionHeader = "Anthropic-Version"
	fallbackKey            = "fallback-key"
	test500ProviderName    = "test-500"
	test429ProviderName    = "test-429"
	test400ProviderName    = "test-400"
)

// newTestHandler is a helper that creates a handler with common test defaults.
func newTestHandler(
	t *testing.T,
	provider providers.Provider,
	providerInfos []router.ProviderInfo,
	providerRouter router.ProviderRouter,
	apiKey string,
	routingDebug bool,
) *proxy.Handler {
	t.Helper()
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderInfos:     providerInfos,
		ProviderRouter:    providerRouter,
		APIKey:            apiKey,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: routingDebug,
	})
	require.NoError(t, err)
	return handler
}

func newHandlerWithAPIKey(t *testing.T, provider providers.Provider) *proxy.Handler {
	t.Helper()
	return newTestHandler(t, provider, nil, nil, testKey, false)
}

func serveJSONMessages(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	return serveJSONMessagesBody(t, handler, `{}`)
}

func newJSONMessagesRequest(body string) *http.Request {
	return proxy.NewMessagesRequestWithHeaders(body,
		proxy.HeaderPair{Key: proxy.ContentTypeHeader, Value: proxy.JSONContentType},
	)
}

func serveJSONMessagesBody(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	return proxy.ServeRequest(t, handler, newJSONMessagesRequest(body))
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

func newHandlerWithPool(t *testing.T, provider providers.Provider, pool *keypool.KeyPool) *proxy.Handler {
	t.Helper()
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              pool,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            "",
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	require.NoError(t, err)
	return handler
}

func serveMessages(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := proxy.NewMessagesRequest(bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func newTrackedHandler(
	t *testing.T,
	providerName, backendURL, routerName string,
	failureThreshold uint32,
) (*proxy.Handler, *health.Tracker) {
	t.Helper()

	provider := proxy.NewNamedProvider(providerName, backendURL)
	logger := zerolog.Nop()
	tracker := health.NewTracker(health.CircuitBreakerConfig{
		FailureThreshold: failureThreshold,
		OpenDurationMS:   0,
		HalfOpenProbes:   0,
	}, &logger)

	providerInfos := []router.ProviderInfo{
		{
			Provider:  provider,
			IsHealthy: tracker.IsHealthyFunc(providerName),
			Weight:    0,
			Priority:  0,
		},
	}

	mockR := &mockRouter{
		err:  nil,
		name: routerName,
		selected: router.ProviderInfo{
			Provider:  provider,
			IsHealthy: tracker.IsHealthyFunc(providerName),
			Weight:    0,
			Priority:  0,
		},
	}

	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderInfos:     providerInfos,
		ProviderRouter:    mockR,
		APIKey:            testKey,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		SignatureCache:    nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug:  true,
		HealthTracker: tracker,
	})
	require.NoError(t, err)
	return handler, tracker
}

func TestNewHandlerValidProvider(t *testing.T) {
	t.Parallel()

	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)
	handler := newHandlerWithAPIKey(t, provider)

	if handler == nil {
		t.Error("Expected non-nil handler")
	}
}

func TestNewHandlerInvalidURL(t *testing.T) {
	t.Parallel()
	// Create a mock provider with invalid URL
	provider := &mockProvider{baseURL: "://invalid-url"}

	_, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            testKey,
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	if err == nil {
		t.Error("Expected error for invalid base URL, got nil")
	}
}

func TestNewHandlerWithLiveProvidersNilProviderInfosFunc(t *testing.T) {
	t.Parallel()

	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)

	// Should not panic with nil providerInfosFunc
	handler, err := proxy.NewHandlerWithLiveProviders(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            testKey,
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
		// nil ProviderInfosFunc - should be guarded
	})
	require.NoError(t, err)
	require.NotNil(t, handler)

	// Verify the handler works (single provider mode)
	assert.NotNil(t, proxy.GetHandlerDefaultProvider(handler))
	assert.Len(t, proxy.GetHandlerProviderProxies(handler), 1)
}

func TestHandlerForwardsAnthropicHeaders(t *testing.T) {
	t.Parallel()

	// Create mock backend that echoes headers
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// Check for anthropic headers
		if request.Header.Get(anthropicVersionHeader) != proxy.AnthropicVersion {
			t.Errorf("Expected Anthropic-Version header, got %q", request.Header.Get(anthropicVersionHeader))
		}

		if request.Header.Get("Anthropic-Beta") != "test-feature" {
			t.Errorf("Expected Anthropic-Beta header, got %q", request.Header.Get("Anthropic-Beta"))
		}

		writer.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Create provider pointing to mock backend
	provider := proxy.NewTestProvider(backend.URL)
	handler := newHandlerWithAPIKey(t, provider)

	// Create request with anthropic headers
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: anthropicVersionHeader, Value: proxy.AnthropicVersion},
		proxy.HeaderPair{Key: "Anthropic-Beta", Value: "test-feature"},
	)
	w := proxy.ServeRequest(t, handler, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlerHasErrorHandler(t *testing.T) {
	t.Parallel()

	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)
	handler := newHandlerWithAPIKey(t, provider)

	// Verify ProviderProxy exists and has ErrorHandler configured
	pp, ok := proxy.GetHandlerProviderProxies(handler)[provider.Name()]
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

	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)
	handler := newHandlerWithAPIKey(t, provider)

	// Verify handler has providerProxies map
	providerProxies := proxy.GetHandlerProviderProxies(handler)
	if providerProxies == nil {
		t.Error("handler.providerProxies is nil")
	}

	// Verify provider proxy exists
	providerProxy, ok := providerProxies[provider.Name()]
	if !ok {
		t.Error("expected provider proxy to be configured")
		return
	}

	// Verify FlushInterval is set to -1
	if providerProxy.Proxy.FlushInterval != -1 {
		t.Errorf("FlushInterval = %v, want -1", providerProxy.Proxy.FlushInterval)
	}

	// Verify provider is set
	if providerProxy.Provider == nil {
		t.Error("provider proxy's Provider is nil")
	}

	// Verify apiKey is set
	if providerProxy.APIKey != testKey {
		t.Errorf("provider proxy APIKey = %q, want %q", providerProxy.APIKey, testKey)
	}
}

func TestHandlerPreservesToolUseId(t *testing.T) {
	t.Parallel()

	// Create mock backend that echoes request body
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		// Echo the body back
		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write(body); err != nil {
			return
		}
	}))
	defer backend.Close()

	// Create provider pointing to mock backend
	provider := proxy.NewTestProvider(backend.URL)

	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            testKey,
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	if err != nil {
		t.Fatalf("proxy.NewHandler failed: %v", err)
	}

	// Request body with tool_use_id
	requestBody := `{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"test"}],` +
		`"tools":[{"name":"test","input_schema":{}}],` +
		`"tool_choice":{"type":"tool","name":"test","tool_use_id":"toolu_123"}}`

	// Create request
	req := proxy.NewMessagesRequestWithHeaders(requestBody,
		proxy.HeaderPair{Key: proxy.ContentTypeHeader, Value: proxy.JSONContentType},
	)
	responseRecorder := proxy.ServeRequest(t, handler, req)

	// Verify response contains tool_use_id
	responseBody := responseRecorder.Body.String()
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

	headers.Set(proxy.ContentTypeHeader, proxy.JSONContentType)

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
	backend := proxy.NewJSONBackend(t, `{"id":"test","type":"message"}`)

	// Create key pool with test keys
	pool := newKeyPool(t, []keypool.KeyConfig{
		{APIKey: "test-key-1", RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000, Priority: 0, Weight: 0},
		{APIKey: "test-key-2", RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000, Priority: 0, Weight: 0},
	})

	// Create handler with key pool
	provider := proxy.NewTestProvider(backend.URL)
	handler := newHandlerWithPool(t, provider, pool)
	recorder := serveMessages(t, handler)

	// Verify response
	assert.Equal(t, http.StatusOK, recorder.Code)

	// Verify x-cc-relay-* headers are set
	assert.NotEmpty(t, recorder.Header().Get(proxy.HeaderRelayKeyID))
	assert.Equal(t, "2", recorder.Header().Get(proxy.HeaderRelayKeysTotal))
	assert.Equal(t, "2", recorder.Header().Get(proxy.HeaderRelayKeysAvail))
}

// TestHandler_AllKeysExhausted tests 429 response when all keys exhausted.
func TestHandlerAllKeysExhausted(t *testing.T) {
	t.Parallel()

	// Create key pool with single key and very low limit
	pool := newKeyPool(t, []keypool.KeyConfig{
		{APIKey: testKey, RPMLimit: 1, ITPMLimit: 1, OTPMLimit: 1, Priority: 0, Weight: 0},
	})

	// Exhaust the key by making a request
	_, _, err := pool.GetKey(context.Background())
	require.NoError(t, err)

	// Create handler
	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)
	handler := newHandlerWithPool(t, provider, pool)
	recorder := serveMessages(t, handler)

	// Verify 429 response
	assert.Equal(t, http.StatusTooManyRequests, recorder.Code)

	// Verify Retry-After header exists
	assert.NotEmpty(t, recorder.Header().Get("Retry-After"))

	// Verify response body matches Anthropic error format
	var errResp proxy.ErrorResponse
	err = json.NewDecoder(recorder.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "error", errResp.Type)
	assert.Equal(t, "rate_limit_error", errResp.Error.Type)
	assert.Contains(t, errResp.Error.Message, "rate limit")
}

// TestHandler_KeyPoolUpdate tests that handler updates key state from response headers.
func TestHandlerKeyPoolUpdate(t *testing.T) {
	t.Parallel()

	// Create mock backend that returns rate limit headers
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("anthropic-ratelimit-requests-limit", "100")
		writer.Header().Set("anthropic-ratelimit-requests-remaining", "99")
		writer.Header().Set("anthropic-ratelimit-input-tokens-limit", "50000")
		writer.Header().Set("anthropic-ratelimit-input-tokens-remaining", "49000")
		writer.Header().Set("anthropic-ratelimit-output-tokens-limit", "20000")
		writer.Header().Set("anthropic-ratelimit-output-tokens-remaining", "19000")
		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	// Create key pool
	pool := newKeyPool(t, []keypool.KeyConfig{
		{APIKey: testKey, RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000, Priority: 0, Weight: 0},
	})

	// Create handler
	provider := proxy.NewTestProvider(backend.URL)
	handler := newHandlerWithPool(t, provider, pool)
	responseRecorder := serveMessages(t, handler)

	// Verify response OK
	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	// Verify key state was updated (check via stats)
	stats := pool.GetStats()
	assert.Equal(t, 1, stats.TotalKeys)
	assert.Equal(t, 1, stats.AvailableKeys)
}

// TestHandler_Backend429 tests that handler marks key exhausted on backend 429.
func TestHandlerBackend429(t *testing.T) {
	t.Parallel()

	// Create mock backend that returns 429
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "60")
		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusTooManyRequests)
		if _, err := writer.Write([]byte(
			`{"type":"error","error":{"type":"rate_limit_error","message":"rate limit"}}`),
		); err != nil {
			return
		}
	}))
	defer backend.Close()

	// Create key pool
	pool := newKeyPool(t, []keypool.KeyConfig{
		{APIKey: testKey, RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000, Priority: 0, Weight: 0},
	})

	// Create handler
	provider := proxy.NewTestProvider(backend.URL)
	handler := newHandlerWithPool(t, provider, pool)
	responseRecorder := serveMessages(t, handler)

	// Verify 429 is passed through
	assert.Equal(t, http.StatusTooManyRequests, responseRecorder.Code)

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
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// Verify auth header uses single key
		assert.Equal(t, testSingleKey, request.Header.Get("X-Api-Key"))
		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	// Create handler without key pool (nil)
	provider := proxy.NewTestProvider(backend.URL)
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            testSingleKey,
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	require.NoError(t, err)

	// Make request
	req := proxy.NewMessagesRequestWithHeaders("{}")
	responseRecorder := proxy.ServeRequest(t, handler, req)

	// Verify response OK
	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	// Verify no x-cc-relay-* headers (single key mode)
	assert.Empty(t, responseRecorder.Header().Get(proxy.HeaderRelayKeyID))
	assert.Empty(t, responseRecorder.Header().Get(proxy.HeaderRelayKeysTotal))
}

// TestHandler_UsesFallbackKeyWhenNoClientAuth tests that configured provider keys
// are used when client provides no auth headers.
func TestHandlerUsesFallbackKeyWhenNoClientAuth(t *testing.T) {
	t.Parallel()

	var receivedAuthHeader string
	var receivedAPIKeyHeader string

	// Create mock backend that captures headers
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedAuthHeader = request.Header.Get("Authorization")
		receivedAPIKeyHeader = request.Header.Get("x-api-key")

		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	provider := proxy.NewTestProvider(backend.URL)
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            "our-fallback-key",
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	require.NoError(t, err)

	// Create request WITHOUT any auth headers
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: anthropicVersionHeader, Value: proxy.AnthropicVersion2024},
	)
	// NO Authorization, NO x-api-key

	responseRecorder := proxy.ServeRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

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
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedAuthHeader = request.Header.Get("Authorization")
		receivedAPIKeyHeader = request.Header.Get("x-api-key")

		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	// Create handler with a configured fallback key
	provider := proxy.NewTestProvider(backend.URL)
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            fallbackKey,
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	require.NoError(t, err)

	// Create request WITH client Authorization header
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: "Authorization", Value: "Bearer sub_12345"},
		proxy.HeaderPair{Key: anthropicVersionHeader, Value: proxy.AnthropicVersion2024},
	)
	responseRecorder := proxy.ServeRequest(t, handler, req)

	// Verify response OK
	assert.Equal(t, http.StatusOK, responseRecorder.Code)

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

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedAuthHeader = request.Header.Get("Authorization")
		receivedAPIKeyHeader = request.Header.Get("x-api-key")

		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	provider := proxy.NewTestProvider(backend.URL)
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            fallbackKey,
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	require.NoError(t, err)

	// Create request WITH client x-api-key header
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: "x-api-key", Value: "sk-ant-client-key"},
		proxy.HeaderPair{Key: anthropicVersionHeader, Value: proxy.AnthropicVersion2024},
	)
	responseRecorder := proxy.ServeRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	// Client x-api-key should be forwarded UNCHANGED
	assert.Equal(t, "sk-ant-client-key", receivedAPIKeyHeader)

	// No Authorization header should be added
	assert.Empty(t, receivedAuthHeader)
}

// TestHandler_TransparentModeSkipsKeyPool tests that key pool is skipped
// when client provides auth (rate limiting is their problem).
func TestHandlerTransparentModeSkipsKeyPool(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	// Create key pool with test keys
	pool, err := keypool.NewKeyPool(testProviderName, keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			{APIKey: poolKey1, RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000, Priority: 0, Weight: 0},
		},
	})
	require.NoError(t, err)

	provider := proxy.NewTestProvider(backend.URL)
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              pool,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            "",
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	require.NoError(t, err)

	// Create request WITH client auth
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: "Authorization", Value: "Bearer client-token"},
	)
	responseRecorder := proxy.ServeRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	// x-cc-relay-* headers should NOT be set (key pool was skipped)
	assert.Empty(t, responseRecorder.Header().Get(proxy.HeaderRelayKeyID),
		"key pool should be skipped in transparent mode")
	assert.Empty(t, responseRecorder.Header().Get(proxy.HeaderRelayKeysTotal))
}

// TestHandler_FallbackModeUsesKeyPool tests that key pool is used
// when client provides no auth.
func TestHandlerFallbackModeUsesKeyPool(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	pool, err := keypool.NewKeyPool(testProviderName, keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			{APIKey: poolKey1, RPMLimit: 50, ITPMLimit: 10000, OTPMLimit: 5000, Priority: 0, Weight: 0},
		},
	})
	require.NoError(t, err)

	provider := proxy.NewTestProvider(backend.URL)
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              pool,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            "",
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	require.NoError(t, err)

	// Create request WITHOUT client auth
	req := proxy.NewMessagesRequestWithHeaders("{}")
	// NO Authorization, NO x-api-key
	responseRecorder := proxy.ServeRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	// x-cc-relay-* headers SHOULD be set (key pool was used)
	assert.NotEmpty(t, responseRecorder.Header().Get(proxy.HeaderRelayKeyID),
		"key pool should be used in fallback mode")
	assert.Equal(t, "1", responseRecorder.Header().Get(proxy.HeaderRelayKeysTotal))
}

// TestHandler_TransparentModeForwardsAnthropicHeaders tests that anthropic-* headers
// are forwarded in transparent mode.
func TestHandlerTransparentModeForwardsAnthropicHeaders(t *testing.T) {
	t.Parallel()

	var receivedVersion string
	var receivedBeta string

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedVersion = request.Header.Get(anthropicVersionHeader)
		receivedBeta = request.Header.Get("Anthropic-Beta")

		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	provider := proxy.NewTestProvider(backend.URL)
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            fallbackKey,
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	require.NoError(t, err)

	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: "Authorization", Value: "Bearer client-token"},
		proxy.HeaderPair{Key: anthropicVersionHeader, Value: proxy.AnthropicVersion2024},
		proxy.HeaderPair{Key: "Anthropic-Beta", Value: proxy.AnthropicBetaExtendedThinking},
	)
	responseRecorder := proxy.ServeRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	assert.Equal(t, proxy.AnthropicVersion2024, receivedVersion)
	assert.Equal(t, proxy.AnthropicBetaExtendedThinking, receivedBeta)
}

// TestHandler_NonTransparentProviderUsesConfiguredKeys tests that providers
// that don't support transparent auth (like Z.AI) use configured keys even
// when client sends Authorization header.
func TestHandlerNonTransparentProviderUsesConfiguredKeys(t *testing.T) {
	t.Parallel()

	var receivedAPIKey string
	var receivedAuthHeader string

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedAPIKey = request.Header.Get("x-api-key")
		receivedAuthHeader = request.Header.Get("Authorization")

		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	// Z.AI provider does NOT support transparent auth
	provider := providers.NewZAIProvider("test-zai", backend.URL)
	handler := newTestHandler(t, provider, nil, nil, "zai-configured-key", false)

	// Client sends Authorization header (like Claude Code does)
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: "Authorization", Value: "Bearer client-anthropic-token"},
		proxy.HeaderPair{Key: anthropicVersionHeader, Value: proxy.AnthropicVersion2024},
	)
	responseRecorder := proxy.ServeRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

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

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedAPIKey = request.Header.Get("x-api-key")

		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	// Create key pool
	pool, err := keypool.NewKeyPool("test-zai", keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			{ //nolint:gosec // G101: test-only credential
				APIKey:    "zai-test-pool-key-one",
				RPMLimit:  50,
				ITPMLimit: 10000,
				OTPMLimit: 5000,
				Priority:  0,
				Weight:    0,
			},
		},
	})
	require.NoError(t, err)

	// Z.AI provider does NOT support transparent auth
	provider := providers.NewZAIProvider("test-zai", backend.URL)
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              pool,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            "",
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	require.NoError(t, err)

	// Client sends Authorization header
	req := proxy.NewMessagesRequestWithHeaders("{}",
		proxy.HeaderPair{Key: "Authorization", Value: "Bearer client-anthropic-token"},
	)
	responseRecorder := proxy.ServeRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	// Key pool should be used (relay headers present)
	assert.NotEmpty(t,
		responseRecorder.Header().Get(proxy.HeaderRelayKeyID),
		"key pool should be used for non-transparent provider",
	)

	// Configured pool key should be sent, not client auth
	assert.Equal(t, "zai-test-pool-key-one", receivedAPIKey)
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			headers := make(http.Header)
			if testCase.header != "" {
				headers.Set("Retry-After", testCase.header)
			}

			result := proxy.ParseRetryAfter(headers)
			assert.Equal(t, testCase.expected, result)
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

type captureRouter struct {
	name       string
	lastSeen   []router.ProviderInfo
	lastCalled int
}

func (c *captureRouter) Select(_ context.Context, infos []router.ProviderInfo) (router.ProviderInfo, error) {
	c.lastCalled++
	c.lastSeen = append([]router.ProviderInfo(nil), infos...)
	if len(infos) == 0 {
		return router.ProviderInfo{}, router.ErrNoProviders
	}
	return infos[0], nil
}

func (c *captureRouter) Name() string {
	return c.name
}

// TestHandler_SingleProviderMode tests that handler works without router (backwards compat).
func TestHandlerSingleProviderMode(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	provider := proxy.NewTestProvider(backend.URL)
	// No router (nil), no providers list (nil) - single provider mode
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            testKey,
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	require.NoError(t, err)

	req := proxy.NewMessagesRequestWithHeaders("{}")
	responseRecorder := proxy.ServeRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	// No routing debug headers in single provider mode
	assert.Empty(t, responseRecorder.Header().Get("X-CC-Relay-Strategy"))
	assert.Empty(t, responseRecorder.Header().Get("X-CC-Relay-Provider"))
}

// TestHandler_MultiProviderModeUsesRouter tests that handler uses router for selection.
func TestHandlerMultiProviderModeUsesRouter(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	provider1 := proxy.NewNamedProvider("provider1", backend.URL)
	provider2 := proxy.NewNamedProvider("provider2", backend.URL)

	providerInfos := []router.ProviderInfo{
		{Provider: provider1, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
		{Provider: provider2, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
	}

	// Mock router that always selects provider2
	mockR := &mockRouter{
		err:  nil,
		name: "test_strategy",
		selected: router.ProviderInfo{
			Provider:  provider2,
			IsHealthy: func() bool { return true },
			Weight:    0,
			Priority:  0,
		},
	}

	// routingDebug=true to get debug headers
	handler := newTestHandler(t, provider1, providerInfos, mockR, testKey, true)

	req := proxy.NewMessagesRequestWithHeaders("{}")
	responseRecorder := proxy.ServeRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	// Debug headers should be present
	assert.Equal(t, "test_strategy", responseRecorder.Header().Get("X-CC-Relay-Strategy"))
	assert.Equal(t, "provider2", responseRecorder.Header().Get("X-CC-Relay-Provider"))
}

func TestHandlerModelBasedRoutingHotReload(t *testing.T) {
	t.Parallel()

	backend := proxy.NewJSONBackend(t, `{"id":"test"}`)
	providerA := proxy.NewNamedProvider(providerAName, backend.URL)
	providerB := proxy.NewNamedProvider(providerBName, backend.URL)
	providerInfos := []router.ProviderInfo{
		proxy.TestProviderInfo(providerA),
		proxy.TestProviderInfo(providerB),
	}

	initialCfg := proxy.TestConfig("")
	initialCfg.Routing.Strategy = router.StrategyRoundRobin
	runtimeCfg := config.NewRuntime(initialCfg)

	capture := &captureRouter{name: "capture", lastSeen: nil, lastCalled: 0}
	handler := setupModelBasedHandler(t, providerA, providerInfos, capture, runtimeCfg)

	req := newJSONMessagesRequest(`{"model":"glm-4","messages":[]}`)
	resp := proxy.ServeRequest(t, handler, req)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Len(t, capture.lastSeen, 2)

	updatedCfg := proxy.TestConfig("")
	updatedCfg.Routing.Strategy = router.StrategyModelBased
	updatedCfg.Routing.ModelMapping = map[string]string{"glm": providerBName}
	updatedCfg.Routing.DefaultProvider = providerAName
	runtimeCfg.Store(updatedCfg)

	req = newJSONMessagesRequest(`{"model":"glm-4","messages":[]}`)
	resp = proxy.ServeRequest(t, handler, req)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Len(t, capture.lastSeen, 1)
	assert.Equal(t, providerBName, capture.lastSeen[0].Provider.Name())
}

// setupModelBasedHandler creates a handler with live providers for model-based routing tests.
func setupModelBasedHandler(
	t *testing.T,
	provider providers.Provider,
	providerInfos []router.ProviderInfo,
	providerRouter router.ProviderRouter,
	runtimeCfg config.RuntimeConfigGetter,
) *proxy.Handler {
	t.Helper()
	handler, err := proxy.NewHandlerWithLiveProviders(proxy.TestHandlerOptions(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderInfosFunc: func() []router.ProviderInfo { return providerInfos },
		ProviderRouter:    providerRouter,
		APIKey:            testKey,
		ProviderPools:     nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		ProviderInfos:     nil,
		DebugOptions:      proxy.TestDebugOptions(),
		RoutingDebug:      false,
	}))
	require.NoError(t, err)
	handler.SetRuntimeConfigGetter(runtimeCfg)
	return handler
}

func TestHandlerLazyProxyForNewProvider(t *testing.T) {
	t.Parallel()

	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"provider":"a"}`)); err != nil {
			return
		}
	}))
	defer backendA.Close()

	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"provider":"b"}`)); err != nil {
			return
		}
	}))
	defer backendB.Close()

	providerA := proxy.NewNamedProvider(providerAName, backendA.URL)
	providerB := proxy.NewNamedProvider(providerBName, backendB.URL)

	infos := []router.ProviderInfo{
		{Provider: providerA, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
	}
	providerInfosFunc := func() []router.ProviderInfo { return infos }

	mockR := &mockRouter{
		name: "mock",
		err:  nil,
		selected: router.ProviderInfo{
			Provider:  providerB,
			IsHealthy: func() bool { return true },
			Weight:    0,
			Priority:  0,
		},
	}

	handler, err := proxy.NewHandlerWithLiveProviders(&proxy.HandlerOptions{
		Provider:          providerA,
		ProviderInfosFunc: providerInfosFunc,
		ProviderRouter:    mockR,
		APIKey:            testKey,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug:     false,
		ProviderPools:    nil,
		Pool:             nil,
		ProviderKeys:     nil,
		GetProviderPools: nil,
		GetProviderKeys:  nil,
		RoutingConfig:    nil,
		HealthTracker:    nil,
		SignatureCache:   nil,
		ProviderInfos:    nil,
	})
	require.NoError(t, err)

	// Simulate reload: provider B becomes enabled
	infos = []router.ProviderInfo{
		{Provider: providerA, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
		{Provider: providerB, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
	}

	req := proxy.NewMessagesRequestWithHeaders(`{"model":"test","messages":[]}`)
	rec := proxy.ServeRequest(t, handler, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"provider":"b"`)
}

// TestHandler_DebugHeadersDisabledByDefault tests that debug headers are not added when disabled.
func TestHandlerDebugHeadersDisabledByDefault(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	provider := proxy.NewTestProvider(backend.URL)
	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
	}

	mockR := &mockRouter{
		name: "failover",
		err:  nil,
		selected: router.ProviderInfo{
			Provider:  provider,
			IsHealthy: func() bool { return true },
			Weight:    0,
			Priority:  0,
		},
	}

	// routingDebug=false (default)
	handler := newTestHandler(t, provider, providerInfos, mockR, testKey, false)

	req := proxy.NewMessagesRequestWithHeaders("{}")
	responseRecorder := proxy.ServeRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	// No debug headers
	assert.Empty(t, responseRecorder.Header().Get("X-CC-Relay-Strategy"))
	assert.Empty(t, responseRecorder.Header().Get("X-CC-Relay-Provider"))
}

// TestHandler_DebugHeadersWhenEnabled tests debug headers are added when routing.debug=true.
func TestHandlerDebugHeadersWhenEnabled(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"id":"test"}`)); err != nil {
			return
		}
	}))
	defer backend.Close()

	provider := proxy.NewNamedProvider(testProviderName, backend.URL)
	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
	}

	mockR := &mockRouter{
		name: "round_robin",
		err:  nil,
		selected: router.ProviderInfo{
			Provider:  provider,
			IsHealthy: func() bool { return true },
			Weight:    0,
			Priority:  0,
		},
	}

	handler := newTestHandler(t, provider, providerInfos, mockR, testKey, true)

	req := proxy.NewMessagesRequestWithHeaders("{}")
	responseRecorder := proxy.ServeRequest(t, handler, req)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	// Debug headers present
	assert.Equal(t, "round_robin", responseRecorder.Header().Get("X-CC-Relay-Strategy"))
	assert.Equal(t, testProviderName, responseRecorder.Header().Get("X-CC-Relay-Provider"))
}

// TestHandler_RouterSelectionError tests error handling when router fails.
func TestHandlerRouterSelectionError(t *testing.T) {
	t.Parallel()

	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)
	providerInfos := []router.ProviderInfo{
		{Provider: provider, IsHealthy: func() bool { return false }, Weight: 0, Priority: 0},
	}

	// Mock router that returns error
	mockR := &mockRouter{
		name:     "failover",
		err:      router.ErrAllProvidersUnhealthy,
		selected: router.ProviderInfo{Provider: nil, IsHealthy: nil, Weight: 0, Priority: 0},
	}

	handler := newTestHandler(t, provider, providerInfos, mockR, testKey, false)

	req := proxy.NewMessagesRequestWithHeaders("{}")
	recorder := proxy.ServeRequest(t, handler, req)

	// Should return 503 Service Unavailable
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	var errResp proxy.ErrorResponse
	decodeErr := json.NewDecoder(recorder.Body).Decode(&errResp)
	require.NoError(t, decodeErr)
	assert.Equal(t, "error", errResp.Type)
	assert.Equal(t, "api_error", errResp.Error.Type)
	assert.Contains(t, errResp.Error.Message, "failed to select provider")
}

// TestHandler_SelectProviderSingleMode tests selectProvider in single provider mode.
func TestHandlerSelectProviderSingleMode(t *testing.T) {
	t.Parallel()

	provider := proxy.NewTestProvider(proxy.AnthropicBaseURL)

	// No router, no providers - single provider mode
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            testKey,
		ProviderInfos:     nil,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		RoutingDebug: false,
	})
	require.NoError(t, err)

	info, err := proxy.HandlerSelectProvider(context.Background(), handler, "", false)
	require.NoError(t, err)
	assert.Equal(t, "test", info.Provider.Name())
	assert.True(t, info.Healthy()) // Always healthy in single mode
}

// TestHandler_SelectProviderMultiMode tests selectProvider uses router.
func TestHandlerSelectProviderMultiMode(t *testing.T) {
	t.Parallel()

	provider1 := proxy.NewNamedProvider("provider1", proxy.AnthropicBaseURL)
	provider2 := proxy.NewNamedProvider("provider2", proxy.AnthropicBaseURL)

	providerInfos := []router.ProviderInfo{
		{Provider: provider1, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
		{Provider: provider2, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
	}

	mockR := &mockRouter{
		name: "test",
		err:  nil,
		selected: router.ProviderInfo{
			Provider:  provider2,
			IsHealthy: func() bool { return true },
			Weight:    0,
			Priority:  0,
		},
	}

	handler := newTestHandler(t, provider1, providerInfos, mockR, testKey, false)

	info, err := proxy.HandlerSelectProvider(context.Background(), handler, "", false)
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
		if _, err := w.Write([]byte(`{"id":"msg_123","type":"message"}`)); err != nil {
			return
		}
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

	backend := proxy.NewStatusBackend(t, http.StatusOK, `{"id":"msg_123","type":"message"}`, nil)

	handler, tracker := newTrackedHandler(t, "test", backend.URL, "test", 2)
	rr := serveJSONMessages(t, handler)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Provider should still be healthy after successful request
	assert.True(t, tracker.IsHealthyFunc("test")())
}

// TestHandler_ReportOutcome_Failure5xx tests 5xx responses count as failures.
func TestHandlerReportOutcomeFailure5xx(t *testing.T) {
	t.Parallel()

	backend := proxy.NewStatusBackend(t, http.StatusInternalServerError, `{"error":"internal"}`, nil)

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

	backend := proxy.NewStatusBackend(t, http.StatusTooManyRequests, `{"error":"rate_limited"}`, map[string]string{
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

	backend := proxy.NewStatusBackend(t, http.StatusBadRequest, `{"error":"bad_request"}`, nil)

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
	backend1 := proxy.NewJSONBackend(t, `{"id":"test1"}`)
	backend2 := proxy.NewJSONBackend(t, `{"id":"test2"}`)

	provider1 := proxy.NewNamedProvider("provider1", backend1.URL)
	provider2 := proxy.NewNamedProvider("provider2", backend2.URL)

	providerInfos := []router.ProviderInfo{
		{Provider: provider1, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
		{Provider: provider2, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
	}

	tracker := &trackingRouter{name: "tracking", receivedProviders: nil}

	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider1,
		ProviderInfos:     providerInfos,
		ProviderRouter:    tracker,
		APIKey:            testKey,
		ProviderPools:     map[string]*keypool.KeyPool{"provider1": nil, "provider2": nil},
		ProviderKeys:      map[string]string{"provider1": "key1", "provider2": "key2"},
		DebugOptions:      proxy.TestDebugOptions(),
		RoutingDebug:      true,
		ProviderInfosFunc: nil,
		Pool:              nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
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

	backend1 := proxy.NewJSONBackend(t, `{"id":"test1"}`)
	backend2 := proxy.NewJSONBackend(t, `{"id":"test2"}`)

	provider1 := proxy.NewNamedProvider("provider1", backend1.URL)
	provider2 := proxy.NewNamedProvider("provider2", backend2.URL)

	// Provider1 is unhealthy, provider2 is healthy
	providerInfos := []router.ProviderInfo{
		{Provider: provider1, IsHealthy: func() bool { return false }, Weight: 0, Priority: 0}, // UNHEALTHY
		{Provider: provider2, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
	}

	tracker := &trackingRouter{name: "tracking", receivedProviders: nil}

	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider1,
		ProviderInfos:     providerInfos,
		ProviderRouter:    tracker,
		APIKey:            testKey,
		ProviderPools:     map[string]*keypool.KeyPool{"provider1": nil, "provider2": nil},
		ProviderKeys:      map[string]string{"provider1": "key1", "provider2": "key2"},
		DebugOptions:      proxy.TestDebugOptions(),
		RoutingDebug:      false,
		ProviderInfosFunc: nil,
		Pool:              nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
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

	backend1 := proxy.NewJSONBackend(t, `{"id":"test1"}`)
	backend2 := proxy.NewJSONBackend(t, `{"id":"test2"}`)

	provider1 := proxy.NewNamedProvider("provider1", backend1.URL)
	provider2 := proxy.NewNamedProvider("provider2", backend2.URL)

	providerInfos := []router.ProviderInfo{
		{Provider: provider1, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
		{Provider: provider2, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
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

	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider1,
		ProviderInfos:     providerInfos,
		ProviderRouter:    countingRouter,
		APIKey:            testKey,
		ProviderPools:     map[string]*keypool.KeyPool{"provider1": nil, "provider2": nil},
		ProviderKeys:      map[string]string{"provider1": "key1", "provider2": "key2"},
		DebugOptions:      proxy.TestDebugOptions(),
		RoutingDebug:      true,
		ProviderInfosFunc: nil,
		Pool:              nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
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
	responseRecorder := serveJSONMessagesBody(t, handler, noThinkingBody)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	// Should receive ALL providers (2), not just 1
	require.Len(t, providerCounts, 1)
	assert.Equal(t, 2, providerCounts[0], "non-thinking request should receive all 2 providers")

	// No thinking affinity header should be set
	assert.Empty(t, responseRecorder.Header().Get("X-CC-Relay-Thinking-Affinity"))
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

// TestHandler_GetOrCreateProxy_NilKeysMapPreservesExistingProxy tests that nil keys map
// preserves existing proxy in single-provider mode.
func TestHandlerGetOrCreateProxyNilKeysMapPreservesExistingProxy(t *testing.T) {
	t.Parallel()

	// Create provider with valid URL (required for proxy creation)
	prov := &mockProvider{baseURL: localBaseURL}

	// Create handler with a key but nil maps (single-provider mode)
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          prov,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            initialKey,
		ProviderInfos:     nil,
		DebugOptions:      proxy.TestDebugOptions(),
		RoutingDebug:      false,
	})
	require.NoError(t, err)

	// Get proxy - should have the initial key
	pp1, err := proxy.HandlerGetOrCreateProxy(handler, prov)
	require.NoError(t, err)
	assert.Equal(t, initialKey, pp1.APIKey)

	// Get proxy again - with nil keys map, should return same proxy
	pp2, err := proxy.HandlerGetOrCreateProxy(handler, prov)
	require.NoError(t, err)
	assert.Same(t, pp1, pp2, "should return same proxy instance")
	assert.Equal(t, initialKey, pp2.APIKey)
}

// TestHandler_GetOrCreateProxy_NilPoolsMapPreservesExistingProxy tests that nil pools map
// preserves existing proxy in single-provider mode.
func TestHandlerGetOrCreateProxyNilPoolsMapPreservesExistingProxy(t *testing.T) {
	t.Parallel()

	prov := &mockProvider{baseURL: localBaseURL}
	pool, err := keypool.NewKeyPool(testProviderName, keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys:     []keypool.KeyConfig{proxy.TestKeyConfig(poolKey1)},
	})
	require.NoError(t, err)

	// Create handler with a pool but nil pools map (single-provider mode)
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          prov,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              pool,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            "",
		ProviderInfos:     nil,
		DebugOptions:      proxy.TestDebugOptions(),
		RoutingDebug:      false,
	})
	require.NoError(t, err)

	pp1, err := proxy.HandlerGetOrCreateProxy(handler, prov)
	require.NoError(t, err)
	assert.Same(t, pool, pp1.KeyPool)

	// Get proxy again - should return same proxy
	pp2, err := proxy.HandlerGetOrCreateProxy(handler, prov)
	require.NoError(t, err)
	assert.Same(t, pp1, pp2)
	assert.Same(t, pool, pp2.KeyPool)
}

// TestHandler_GetOrCreateProxy_MultiProviderModeDetectsKeyChange tests that
// multi-provider mode detects key changes via hot-reload.
func TestHandlerGetOrCreateProxyMultiProviderModeDetectsKeyChange(t *testing.T) {
	t.Parallel()

	prov := &mockProvider{baseURL: localBaseURL}
	provName := prov.Name()

	// Create handler with keys map (multi-provider mode)
	keysMap := map[string]string{provName: "key-v1"}
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          prov,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      keysMap,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            "",
		ProviderInfos:     nil,
		DebugOptions:      proxy.TestDebugOptions(),
		RoutingDebug:      false,
	})
	require.NoError(t, err)

	pp1, err := proxy.HandlerGetOrCreateProxy(handler, prov)
	require.NoError(t, err)
	assert.Equal(t, "key-v1", pp1.APIKey)

	// Simulate hot-reload: change key in map
	keysMap[provName] = "key-v2"

	// Get proxy again - should create new proxy with new key
	pp2, err := proxy.HandlerGetOrCreateProxy(handler, prov)
	require.NoError(t, err)
	assert.NotSame(t, pp1, pp2, "should create new proxy after key change")
	assert.Equal(t, "key-v2", pp2.APIKey)
}

// TestHandler_GetOrCreateProxy_MultiProviderModeDetectsPoolChange tests that
// multi-provider mode detects pool changes via hot-reload.
func TestHandlerGetOrCreateProxyMultiProviderModeDetectsPoolChange(t *testing.T) {
	t.Parallel()

	prov := &mockProvider{baseURL: localBaseURL}
	provName := prov.Name()

	pool1, err := keypool.NewKeyPool(testProviderName, keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys:     []keypool.KeyConfig{proxy.TestKeyConfig(poolKey1)},
	})
	require.NoError(t, err)
	pool2, err := keypool.NewKeyPool(testProviderName, keypool.PoolConfig{
		Strategy: "least_loaded",
		Keys: []keypool.KeyConfig{
			proxy.TestKeyConfig("pool-key-2"),
		},
	})
	require.NoError(t, err)

	// Create handler with pools map (multi-provider mode)
	poolsMap := map[string]*keypool.KeyPool{provName: pool1}
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          prov,
		ProviderRouter:    nil,
		ProviderPools:     poolsMap,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            "",
		ProviderInfos:     nil,
		DebugOptions:      proxy.TestDebugOptions(),
		RoutingDebug:      false,
	})
	require.NoError(t, err)

	pp1, err := proxy.HandlerGetOrCreateProxy(handler, prov)
	require.NoError(t, err)
	assert.Same(t, pool1, pp1.KeyPool)

	// Simulate hot-reload: change pool in map
	poolsMap[provName] = pool2

	// Get proxy again - should create new proxy with new pool
	pp2, err := proxy.HandlerGetOrCreateProxy(handler, prov)
	require.NoError(t, err)
	assert.NotSame(t, pp1, pp2, "should create new proxy after pool change")
	assert.Same(t, pool2, pp2.KeyPool)
}

// TestHandler_GetOrCreateProxy_ProviderBaseURLChangeCreatesNewProxy tests that
// different base URLs create different proxy instances.
func TestHandlerGetOrCreateProxyProviderBaseURLChangeCreatesNewProxy(t *testing.T) {
	t.Parallel()

	prov1 := &mockProvider{baseURL: localBaseURL}
	prov2 := &mockProvider{baseURL: "http://localhost:8888"} // Different URL, same name

	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          prov1,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            testKey,
		ProviderInfos:     nil,
		DebugOptions:      proxy.TestDebugOptions(),
		RoutingDebug:      false,
	})
	require.NoError(t, err)

	pp1, err := proxy.HandlerGetOrCreateProxy(handler, prov1)
	require.NoError(t, err)
	assert.Equal(t, localBaseURL, pp1.Provider.BaseURL())

	// Request proxy with different baseURL provider
	pp2, err := proxy.HandlerGetOrCreateProxy(handler, prov2)
	require.NoError(t, err)
	assert.NotSame(t, pp1, pp2, "should create new proxy for different baseURL")
	assert.Equal(t, "http://localhost:8888", pp2.Provider.BaseURL())
}

// TestHandler_GetOrCreateProxy_LiveKeysFuncUsedOverStaticMap tests that
// the live keys function takes precedence over static map.
func TestHandlerGetOrCreateProxyLiveKeysFuncUsedOverStaticMap(t *testing.T) {
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

	handler, err := proxy.NewHandlerWithLiveProviders(&proxy.HandlerOptions{
		Provider:          prov,
		ProviderKeys:      staticKeys,
		DebugOptions:      proxy.TestDebugOptions(),
		RoutingDebug:      false,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            "",
		ProviderInfos:     nil,
	})
	require.NoError(t, err)

	// Set the live keys func
	proxy.SetHandlerGetProviderKeys(handler, liveKeysFunc)

	pp1, err := proxy.HandlerGetOrCreateProxy(handler, prov)
	require.NoError(t, err)
	assert.Equal(t, "live-key-v1", pp1.APIKey, "should use live func, not static map")

	// Update live key
	<-liveKey // Drain
	liveKey <- "live-key-v2"

	// Get proxy again - should detect change from live func
	pp2, err := proxy.HandlerGetOrCreateProxy(handler, prov)
	require.NoError(t, err)
	assert.NotSame(t, pp1, pp2)
	assert.Equal(t, "live-key-v2", pp2.APIKey)
}
