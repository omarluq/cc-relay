package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/providers"

	"github.com/omarluq/cc-relay/internal/proxy"
)

// TestNewProviderProxyValidProvider tests creating a ProviderProxy with valid provider.
func TestNewProviderProxyValidProvider(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	providerProxy, err := proxy.NewProviderProxy(provider, "test-key", nil, proxy.TestDebugOptions(), nil)
	require.NoError(t, err)
	require.NotNil(t, providerProxy)

	assert.Equal(t, provider, providerProxy.Provider)
	assert.Equal(t, "test-key", providerProxy.APIKey)
	assert.Nil(t, providerProxy.KeyPool)
	assert.NotNil(t, providerProxy.Proxy)
}

// TestNewProviderProxyInvalidURL tests that invalid URL returns error.
func TestNewProviderProxyInvalidURL(t *testing.T) {
	t.Parallel()

	provider := &mockProvider{baseURL: "://invalid-url"}

	_, err := proxy.NewProviderProxy(provider, "test-key", nil, proxy.TestDebugOptions(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid provider base URL")
}

// TestProviderProxySetsCorrectTargetURL tests that proxy routes to correct URL.
func TestProviderProxySetsCorrectTargetURL(t *testing.T) {
	t.Parallel()

	var receivedHost string
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedHost = request.Host
		writer.WriteHeader(http.StatusOK)
		if _, writeErr := writer.Write([]byte(`{"id":"test"}`)); writeErr != nil {
			return
		}
	}))
	defer backend.Close()

	backendURL := proxy.ParseTestURL(t, backend.URL)
	provider := providers.NewAnthropicProvider("test", backendURL)
	providerProxy, err := proxy.NewProviderProxy(provider, "test-key", nil, proxy.TestDebugOptions(), nil)
	require.NoError(t, err)

	// Verify target URL is set correctly
	assert.Equal(t, backendURL, providerProxy.GetTargetURL().String())

	// Make a request through the proxy
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	recorder := httptest.NewRecorder()
	providerProxy.Proxy.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
	// Host should be the backend server
	assert.NotEmpty(t, receivedHost)
}

// testProviderProxyAuthBehaviorTestCase is a test case for TestProviderProxyAuthBehavior.
type testProviderProxyAuthBehaviorTestCase struct {
	name            string
	providerName    string
	providerFactory func(string) providers.Provider
	configuredKey   string
	requestHeaders  map[string]string
	expectedAPIKey  string
	expectedAuth    string
	description     string
}

// testProviderProxyAuthBehaviorTestCases returns test cases for auth behavior tests.
func testProviderProxyAuthBehaviorTestCases() []testProviderProxyAuthBehaviorTestCase {
	const (
		testConfiguredKey = "my-configured-key"
		testFallbackKey   = "fallback-key"
		testClientToken   = "Bearer client-token"
	)
	return []testProviderProxyAuthBehaviorTestCase{
		{
			name:         "configured_key_used_when_set",
			providerName: "test",
			providerFactory: func(backendURL string) providers.Provider {
				return providers.NewAnthropicProvider("test", backendURL)
			},
			configuredKey: testConfiguredKey,
			requestHeaders: map[string]string{
				"X-Selected-Key": testConfiguredKey,
			},
			expectedAPIKey: testConfiguredKey,
			expectedAuth:   "",
			description:    "provider uses configured key when X-Selected-Key is set",
		},
		{
			name:         "transparent_auth_forwards_client_auth",
			providerName: "test",
			providerFactory: func(backendURL string) providers.Provider {
				return providers.NewAnthropicProvider("test", backendURL)
			},
			configuredKey: testFallbackKey,
			requestHeaders: map[string]string{
				"Authorization": testClientToken,
			},
			expectedAPIKey: "",
			expectedAuth:   testClientToken,
			description:    "transparent provider forwards client auth unchanged",
		},
	}
}

// TestProviderProxyAuthBehavior tests authentication forwarding behavior.
// It covers both configured key usage and transparent auth forwarding.
func TestProviderProxyAuthBehavior(t *testing.T) {
	t.Parallel()
	tests := testProviderProxyAuthBehaviorTestCases()
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			backend, capture := proxy.NewHeaderCaptureBackend(t)
			backendURL := proxy.ParseTestURL(t, backend.URL)
			provider := testCase.providerFactory(backendURL)
			providerProxy, err := proxy.NewProviderProxy(
				provider, testCase.configuredKey, nil, proxy.TestDebugOptions(), nil,
			)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
			for headerKey, headerValue := range testCase.requestHeaders {
				req.Header.Set(headerKey, headerValue)
			}
			recorder := httptest.NewRecorder()
			providerProxy.Proxy.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusOK, recorder.Code, testCase.description)
			if testCase.expectedAPIKey != "" {
				assert.Equal(t, testCase.expectedAPIKey, capture.Get("x-api-key"), testCase.description)
			}
			if testCase.expectedAuth != "" {
				assert.Equal(t, testCase.expectedAuth, capture.Get("Authorization"), testCase.description)
			}
		})
	}
}

// TestProviderProxyNonTransparentProviderUsesConfiguredKey tests that non-transparent
// providers use configured keys even when client sends auth.
func TestProviderProxyNonTransparentProviderUsesConfiguredKey(t *testing.T) {
	t.Parallel()

	backend, capture := proxy.NewHeaderCaptureBackend(t)
	backendURL := proxy.ParseTestURL(t, backend.URL)
	// Z.AI provider does NOT support transparent auth
	provider := providers.NewZAIProvider("test-zai", backendURL)
	providerProxy, err := proxy.NewProviderProxy(provider, "zai-key", nil, proxy.TestDebugOptions(), nil)
	require.NoError(t, err)

	// Make request with client auth (but provider doesn't support transparent)
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer client-token")
	// Handler sets X-Selected-Key since provider doesn't support transparent auth
	req.Header.Set("X-Selected-Key", "zai-key")
	recorder := httptest.NewRecorder()
	providerProxy.Proxy.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
	// Client auth should NOT be forwarded
	assert.Empty(t, capture.Get("Authorization"))
	// Our configured key should be used
	assert.Equal(t, "zai-key", capture.Get("x-api-key"))
}

// TestProviderProxyForwardsAnthropicHeaders tests anthropic-* header forwarding.
func TestProviderProxyForwardsAnthropicHeaders(t *testing.T) {
	t.Parallel()

	backend, capture := proxy.NewHeaderCaptureBackend(t)
	backendURL := proxy.ParseTestURL(t, backend.URL)
	provider := providers.NewAnthropicProvider("test", backendURL)
	providerProxy, err := proxy.NewProviderProxy(provider, "key", nil, proxy.TestDebugOptions(), nil)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Anthropic-Version", proxy.AnthropicVersion2024)
	req.Header.Set("Anthropic-Beta", "extended-thinking")
	recorder := httptest.NewRecorder()
	providerProxy.Proxy.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, proxy.AnthropicVersion2024, capture.Get("Anthropic-Version"))
	assert.Equal(t, "extended-thinking", capture.Get("Anthropic-Beta"))
}

// TestProviderProxySSEHeadersSet tests that SSE headers are set for streaming responses.
func TestProviderProxySSEHeadersSet(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)
		if _, writeErr := writer.Write([]byte("data: hello\n\n")); writeErr != nil {
			return
		}
	}))
	defer backend.Close()

	backendURL := proxy.ParseTestURL(t, backend.URL)
	provider := providers.NewAnthropicProvider("test", backendURL)
	providerProxy, err := proxy.NewProviderProxy(provider, "key", nil, proxy.TestDebugOptions(), nil)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("X-Selected-Key", "key")
	recorder := httptest.NewRecorder()
	providerProxy.Proxy.ServeHTTP(recorder, req)
	// Check SSE headers were added
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache, no-transform", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "no", recorder.Header().Get("X-Accel-Buffering"))
}

// TestProviderProxyModifyResponseHookCalled tests that the hook is called.
func TestProviderProxyModifyResponseHookCalled(t *testing.T) {
	t.Parallel()

	hookCalled := false
	hook := func(_ *http.Response) error {
		hookCalled = true
		return nil
	}

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		if _, writeErr := writer.Write([]byte(`{"id":"test"}`)); writeErr != nil {
			return
		}
	}))
	defer backend.Close()

	backendURL := proxy.ParseTestURL(t, backend.URL)
	provider := providers.NewAnthropicProvider("test", backendURL)
	providerProxy, err := proxy.NewProviderProxy(provider, "key", nil, proxy.TestDebugOptions(), hook)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("X-Selected-Key", "key")
	recorder := httptest.NewRecorder()
	providerProxy.Proxy.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, hookCalled, "modifyResponse hook should be called")
}

// TestProviderProxyErrorHandlerReturnsAnthropicFormat tests error response format.
func TestProviderProxyErrorHandlerReturnsAnthropicFormat(t *testing.T) {
	t.Parallel()

	// Create a provider with unreachable backend
	provider := providers.NewAnthropicProvider("test", "http://localhost:1")
	providerProxy, err := proxy.NewProviderProxy(provider, "key", nil, proxy.TestDebugOptions(), nil)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("X-Selected-Key", "key")
	recorder := httptest.NewRecorder()
	providerProxy.Proxy.ServeHTTP(recorder, req)
	// Should return 502 Bad Gateway
	assert.Equal(t, http.StatusBadGateway, recorder.Code)

	// Should be Anthropic-format error
	body, readErr := io.ReadAll(recorder.Body)
	require.NoError(t, readErr)
	assert.Contains(t, string(body), "upstream connection failed")
}

// TestProviderProxyFlushIntervalSetForSSE tests that FlushInterval is -1.
func TestProviderProxyFlushIntervalSetForSSE(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")
	providerProxy, err := proxy.NewProviderProxy(provider, "key", nil, proxy.TestDebugOptions(), nil)
	require.NoError(t, err)

	// FlushInterval -1 means flush after every write (important for SSE)
	assert.Equal(t, int64(-1), int64(providerProxy.Proxy.FlushInterval))
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

// TestProviderProxyTransformRequestCalledForCloudProviders tests that TransformRequest
// is called for providers that require body transformation.
func TestProviderProxyTransformRequestCalledForCloudProviders(t *testing.T) {
	t.Parallel()

	var receivedBody string
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		bodyBytes, readErr := io.ReadAll(request.Body)
		require.NoError(t, readErr)
		receivedBody = string(bodyBytes)
		receivedPath = request.URL.Path
		writer.WriteHeader(http.StatusOK)
		if _, writeErr := writer.Write([]byte(`{"id":"test"}`)); writeErr != nil {
			return
		}
	}))
	defer backend.Close()

	backendURL := proxy.ParseTestURL(t, backend.URL)
	// Cloud provider transforms URL to include model in path
	provider := &mockCloudProvider{
		baseURL:      backendURL,
		transformURL: backendURL + "/model/claude-3/invoke",
	}

	providerProxy, err := proxy.NewProviderProxy(provider, "key", nil, proxy.TestDebugOptions(), nil)
	require.NoError(t, err)

	// Send request with original body
	req := httptest.NewRequest("POST", "/v1/messages",
		strings.NewReader(`{"model":"claude-3","max_tokens":100}`))
	req.Header.Set("X-Selected-Key", "key")
	recorder := httptest.NewRecorder()
	providerProxy.Proxy.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
	// Body should be transformed
	assert.Equal(t, `{"transformed":true}`, receivedBody)
	// URL should include model in path
	assert.Equal(t, "/model/claude-3/invoke", receivedPath)
}

// TestProviderProxyTransformRequestNotCalledForStandardProviders tests that
// TransformRequest is NOT called for standard providers.
func TestProviderProxyTransformRequestNotCalledForStandardProviders(t *testing.T) {
	t.Parallel()

	var receivedBody string
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		bodyBytes, readErr := io.ReadAll(request.Body)
		require.NoError(t, readErr)
		receivedBody = string(bodyBytes)
		writer.WriteHeader(http.StatusOK)
		if _, writeErr := writer.Write([]byte(`{"id":"test"}`)); writeErr != nil {
			return
		}
	}))
	defer backend.Close()

	backendURL := proxy.ParseTestURL(t, backend.URL)
	// Anthropic provider does NOT require body transform
	provider := providers.NewAnthropicProvider("test", backendURL)
	assert.False(t, provider.RequiresBodyTransform())

	providerProxy, err := proxy.NewProviderProxy(provider, "key", nil, proxy.TestDebugOptions(), nil)
	require.NoError(t, err)

	originalBody := `{"model":"claude-3","max_tokens":100}`
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(originalBody))
	req.Header.Set("X-Selected-Key", "key")
	recorder := httptest.NewRecorder()
	providerProxy.Proxy.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
	// Body should NOT be transformed - sent as-is
	assert.Equal(t, originalBody, receivedBody)
}

// TestProviderProxyEventStreamConversion tests Bedrock Event Stream handling.
func TestProviderProxyEventStreamConversion(t *testing.T) {
	t.Parallel()

	// Mock Bedrock provider that returns Event Stream
	provider := &mockEventStreamProvider{
		baseURL: "https://bedrock.example.com",
	}

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		// Return Event Stream content type (like Bedrock)
		writer.Header().Set("Content-Type", providers.ContentTypeEventStream)
		writer.WriteHeader(http.StatusOK)
		// In real scenario, this would be Event Stream binary data
		if _, writeErr := writer.Write([]byte("event-stream-data")); writeErr != nil {
			return
		}
	}))
	defer backend.Close()

	backendURL := proxy.ParseTestURL(t, backend.URL)
	provider.baseURL = backendURL

	providerProxy, err := proxy.NewProviderProxy(provider, "key", nil, proxy.TestDebugOptions(), nil)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("X-Selected-Key", "key")
	recorder := httptest.NewRecorder()
	providerProxy.Proxy.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
	// Content-Type should be converted to SSE
	assert.Equal(t, providers.ContentTypeSSE, recorder.Header().Get("Content-Type"))
	// SSE headers should be set
	assert.Equal(t, "no-cache, no-transform", recorder.Header().Get("Cache-Control"))
}

func TestEventStreamToSSEBodyNoProgress(t *testing.T) {
	t.Parallel()

	body := proxy.NewEventStreamToSSEBody(&stallingReadCloser{})
	buf := make([]byte, 8)

	bytesRead, readErr := body.Read(buf)

	assert.Equal(t, 0, bytesRead)
	assert.ErrorIs(t, readErr, proxy.ErrStreamClosed)
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

type stallingReadCloser struct{}

func (s *stallingReadCloser) Read(_ []byte) (int, error) {
	return 0, nil
}

func (s *stallingReadCloser) Close() error {
	return nil
}
