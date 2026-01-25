package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
)

// TestNewProviderProxy_ValidProvider tests creating a ProviderProxy with valid provider.
func TestNewProviderProxy_ValidProvider(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	pp, err := NewProviderProxy(provider, "test-key", nil, config.DebugOptions{}, nil)
	require.NoError(t, err)
	require.NotNil(t, pp)

	assert.Equal(t, provider, pp.Provider)
	assert.Equal(t, "test-key", pp.APIKey)
	assert.Nil(t, pp.KeyPool)
	assert.NotNil(t, pp.Proxy)
}

// TestNewProviderProxy_InvalidURL tests that invalid URL returns error.
func TestNewProviderProxy_InvalidURL(t *testing.T) {
	t.Parallel()

	provider := &mockProvider{baseURL: "://invalid-url"}

	_, err := NewProviderProxy(provider, "test-key", nil, config.DebugOptions{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid provider base URL")
}

// TestProviderProxy_SetsCorrectTargetURL tests that proxy routes to correct URL.
func TestProviderProxy_SetsCorrectTargetURL(t *testing.T) {
	t.Parallel()

	var receivedHost string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	pp, err := NewProviderProxy(provider, "test-key", nil, config.DebugOptions{}, nil)
	require.NoError(t, err)

	// Verify target URL is set correctly
	assert.Equal(t, backend.URL, pp.GetTargetURL().String())

	// Make a request through the proxy
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	pp.Proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Host should be the backend server
	assert.NotEmpty(t, receivedHost)
}

// TestProviderProxy_UsesCorrectAuth tests that provider's Authenticate is called.
func TestProviderProxy_UsesCorrectAuth(t *testing.T) {
	t.Parallel()

	var receivedAPIKey string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("x-api-key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	pp, err := NewProviderProxy(provider, "my-configured-key", nil, config.DebugOptions{}, nil)
	require.NoError(t, err)

	// Make request without client auth (should use configured key)
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	// Set the X-Selected-Key header (simulating handler setting it)
	req.Header.Set("X-Selected-Key", "my-configured-key")
	w := httptest.NewRecorder()
	pp.Proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "my-configured-key", receivedAPIKey)
}

// TestProviderProxy_TransparentModeForwardsClientAuth tests transparent auth mode.
func TestProviderProxy_TransparentModeForwardsClientAuth(t *testing.T) {
	t.Parallel()

	var receivedAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Anthropic provider supports transparent auth
	provider := providers.NewAnthropicProvider("test", backend.URL)
	pp, err := NewProviderProxy(provider, "fallback-key", nil, config.DebugOptions{}, nil)
	require.NoError(t, err)

	// Make request with client auth
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer client-token")
	w := httptest.NewRecorder()
	pp.Proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Client auth should be forwarded unchanged
	assert.Equal(t, "Bearer client-token", receivedAuth)
}

// TestProviderProxy_NonTransparentProviderUsesConfiguredKey tests that non-transparent
// providers use configured keys even when client sends auth.
func TestProviderProxy_NonTransparentProviderUsesConfiguredKey(t *testing.T) {
	t.Parallel()

	var receivedAPIKey string
	var receivedAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("x-api-key")
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Z.AI provider does NOT support transparent auth
	provider := providers.NewZAIProvider("test-zai", backend.URL)
	pp, err := NewProviderProxy(provider, "zai-key", nil, config.DebugOptions{}, nil)
	require.NoError(t, err)

	// Make request with client auth (but provider doesn't support transparent)
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer client-token")
	// Handler sets X-Selected-Key since provider doesn't support transparent auth
	req.Header.Set("X-Selected-Key", "zai-key")
	w := httptest.NewRecorder()
	pp.Proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Client auth should NOT be forwarded
	assert.Empty(t, receivedAuth)
	// Our configured key should be used
	assert.Equal(t, "zai-key", receivedAPIKey)
}

// TestProviderProxy_ForwardsAnthropicHeaders tests anthropic-* header forwarding.
func TestProviderProxy_ForwardsAnthropicHeaders(t *testing.T) {
	t.Parallel()

	var receivedVersion string
	var receivedBeta string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedVersion = r.Header.Get("Anthropic-Version")
		receivedBeta = r.Header.Get("Anthropic-Beta")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	pp, err := NewProviderProxy(provider, "key", nil, config.DebugOptions{}, nil)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Anthropic-Version", "2024-01-01")
	req.Header.Set("Anthropic-Beta", "extended-thinking")
	w := httptest.NewRecorder()
	pp.Proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "2024-01-01", receivedVersion)
	assert.Equal(t, "extended-thinking", receivedBeta)
}

// TestProviderProxy_SSEHeadersSet tests that SSE headers are set for streaming responses.
func TestProviderProxy_SSEHeadersSet(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: hello\n\n"))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	pp, err := NewProviderProxy(provider, "key", nil, config.DebugOptions{}, nil)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("X-Selected-Key", "key")
	w := httptest.NewRecorder()
	pp.Proxy.ServeHTTP(w, req)

	// Check SSE headers were added
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache, no-transform", w.Header().Get("Cache-Control"))
	assert.Equal(t, "no", w.Header().Get("X-Accel-Buffering"))
}

// TestProviderProxy_ModifyResponseHookCalled tests that the hook is called.
func TestProviderProxy_ModifyResponseHookCalled(t *testing.T) {
	t.Parallel()

	hookCalled := false
	hook := func(_ *http.Response) error {
		hookCalled = true
		return nil
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	provider := providers.NewAnthropicProvider("test", backend.URL)
	pp, err := NewProviderProxy(provider, "key", nil, config.DebugOptions{}, hook)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("X-Selected-Key", "key")
	w := httptest.NewRecorder()
	pp.Proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hookCalled, "modifyResponse hook should be called")
}

// TestProviderProxy_ErrorHandlerReturnsAnthropicFormat tests error response format.
func TestProviderProxy_ErrorHandlerReturnsAnthropicFormat(t *testing.T) {
	t.Parallel()

	// Create a provider with unreachable backend
	provider := providers.NewAnthropicProvider("test", "http://localhost:1")
	pp, err := NewProviderProxy(provider, "key", nil, config.DebugOptions{}, nil)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("X-Selected-Key", "key")
	w := httptest.NewRecorder()
	pp.Proxy.ServeHTTP(w, req)

	// Should return 502 Bad Gateway
	assert.Equal(t, http.StatusBadGateway, w.Code)

	// Should be Anthropic-format error
	body, _ := io.ReadAll(w.Body)
	assert.Contains(t, string(body), "upstream connection failed")
}

// TestProviderProxy_FlushIntervalSetForSSE tests that FlushInterval is -1.
func TestProviderProxy_FlushIntervalSetForSSE(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")
	pp, err := NewProviderProxy(provider, "key", nil, config.DebugOptions{}, nil)
	require.NoError(t, err)

	// FlushInterval -1 means flush after every write (important for SSE)
	assert.Equal(t, int64(-1), int64(pp.Proxy.FlushInterval))
}

// mockCloudProvider simulates a cloud provider that requires body transformation.
type mockCloudProvider struct {
	baseURL      string
	transformURL string
}

func (m *mockCloudProvider) Name() string                       { return "mock-cloud" }
func (m *mockCloudProvider) BaseURL() string                    { return m.baseURL }
func (m *mockCloudProvider) Owner() string                      { return "cloud" }
func (m *mockCloudProvider) SupportsStreaming() bool            { return true }
func (m *mockCloudProvider) SupportsTransparentAuth() bool      { return false }
func (m *mockCloudProvider) ListModels() []providers.Model      { return nil }
func (m *mockCloudProvider) GetModelMapping() map[string]string { return nil }
func (m *mockCloudProvider) MapModel(model string) string       { return model }
func (m *mockCloudProvider) StreamingContentType() string       { return "text/event-stream" }
func (m *mockCloudProvider) Authenticate(req *http.Request, _ string) error {
	req.Header.Set("X-Cloud-Auth", "signed")
	return nil
}
func (m *mockCloudProvider) ForwardHeaders(_ http.Header) http.Header {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	return headers
}
func (m *mockCloudProvider) TransformRequest(_ []byte, _ string) (newBody []byte, targetURL string, err error) {
	// Simulate transformation: remove model, return dynamic URL
	return []byte(`{"transformed":true}`), m.transformURL, nil
}
func (m *mockCloudProvider) TransformResponse(_ *http.Response, _ http.ResponseWriter) error {
	return nil
}
func (m *mockCloudProvider) RequiresBodyTransform() bool {
	return true // This is the key difference from standard providers
}

// TestProviderProxy_TransformRequest_CalledForCloudProviders tests that TransformRequest
// is called for providers that require body transformation.
func TestProviderProxy_TransformRequest_CalledForCloudProviders(t *testing.T) {
	t.Parallel()

	var receivedBody string
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Cloud provider transforms URL to include model in path
	provider := &mockCloudProvider{
		baseURL:      backend.URL,
		transformURL: backend.URL + "/model/claude-3/invoke",
	}

	pp, err := NewProviderProxy(provider, "key", nil, config.DebugOptions{}, nil)
	require.NoError(t, err)

	// Send request with original body
	req := httptest.NewRequest("POST", "/v1/messages",
		strings.NewReader(`{"model":"claude-3","max_tokens":100}`))
	req.Header.Set("X-Selected-Key", "key")
	w := httptest.NewRecorder()
	pp.Proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Body should be transformed
	assert.Equal(t, `{"transformed":true}`, receivedBody)
	// URL should include model in path
	assert.Equal(t, "/model/claude-3/invoke", receivedPath)
}

// TestProviderProxy_TransformRequest_NotCalledForStandardProviders tests that
// TransformRequest is NOT called for standard providers.
func TestProviderProxy_TransformRequest_NotCalledForStandardProviders(t *testing.T) {
	t.Parallel()

	var receivedBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer backend.Close()

	// Anthropic provider does NOT require body transform
	provider := providers.NewAnthropicProvider("test", backend.URL)
	assert.False(t, provider.RequiresBodyTransform())

	pp, err := NewProviderProxy(provider, "key", nil, config.DebugOptions{}, nil)
	require.NoError(t, err)

	originalBody := `{"model":"claude-3","max_tokens":100}`
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(originalBody))
	req.Header.Set("X-Selected-Key", "key")
	w := httptest.NewRecorder()
	pp.Proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Body should NOT be transformed - sent as-is
	assert.Equal(t, originalBody, receivedBody)
}

// TestProviderProxy_EventStreamConversion tests Bedrock Event Stream handling.
func TestProviderProxy_EventStreamConversion(t *testing.T) {
	t.Parallel()

	// Mock Bedrock provider that returns Event Stream
	provider := &mockEventStreamProvider{
		baseURL: "https://bedrock.example.com",
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Return Event Stream content type (like Bedrock)
		w.Header().Set("Content-Type", providers.ContentTypeEventStream)
		w.WriteHeader(http.StatusOK)
		// In real scenario, this would be Event Stream binary data
		_, _ = w.Write([]byte("event-stream-data"))
	}))
	defer backend.Close()

	provider.baseURL = backend.URL

	pp, err := NewProviderProxy(provider, "key", nil, config.DebugOptions{}, nil)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("X-Selected-Key", "key")
	w := httptest.NewRecorder()
	pp.Proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Content-Type should be converted to SSE
	assert.Equal(t, providers.ContentTypeSSE, w.Header().Get("Content-Type"))
	// SSE headers should be set
	assert.Equal(t, "no-cache, no-transform", w.Header().Get("Cache-Control"))
}

// mockEventStreamProvider simulates a Bedrock-like provider.
type mockEventStreamProvider struct {
	baseURL string
}

func (m *mockEventStreamProvider) Name() string                                 { return "mock-bedrock" }
func (m *mockEventStreamProvider) BaseURL() string                              { return m.baseURL }
func (m *mockEventStreamProvider) Owner() string                                { return "aws" }
func (m *mockEventStreamProvider) SupportsStreaming() bool                      { return true }
func (m *mockEventStreamProvider) SupportsTransparentAuth() bool                { return false }
func (m *mockEventStreamProvider) ListModels() []providers.Model                { return nil }
func (m *mockEventStreamProvider) GetModelMapping() map[string]string           { return nil }
func (m *mockEventStreamProvider) MapModel(model string) string                 { return model }
func (m *mockEventStreamProvider) RequiresBodyTransform() bool                  { return false }
func (m *mockEventStreamProvider) Authenticate(_ *http.Request, _ string) error { return nil }
func (m *mockEventStreamProvider) ForwardHeaders(_ http.Header) http.Header {
	return make(http.Header)
}
func (m *mockEventStreamProvider) TransformRequest(
	body []byte, endpoint string,
) (newBody []byte, targetURL string, err error) {
	return body, m.baseURL + endpoint, nil
}
func (m *mockEventStreamProvider) TransformResponse(_ *http.Response, _ http.ResponseWriter) error {
	return nil
}

// StreamingContentType returns Event Stream (like Bedrock).
func (m *mockEventStreamProvider) StreamingContentType() string {
	return providers.ContentTypeEventStream
}
