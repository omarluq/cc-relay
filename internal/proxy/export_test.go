package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/cache"
	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/health"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
)

// ---------------------------------------------------------------------------
// Internal test constants (lowercase, used by tests in package proxy)
// ---------------------------------------------------------------------------

const (
	anthropicVersion              = "2023-06-01"
	anthropicBaseURL              = "https://api.anthropic.com"
	anthropicVersion2024          = "2024-01-01"
	anthropicBetaExtendedThinking = "extended-thinking-2024-01-01"
	messagesPath                  = "/v1/messages"
	apiKeyHeader                  = "x-api-key"
	contentTypeHeader             = "Content-Type"
	jsonContentType               = "application/json"
	validTestSignature            = "valid_signature_that_is_definitely_long_enough_for_validation"
	listObject                    = "list"
)

// ---------------------------------------------------------------------------
// Exported constants (uppercase, used by tests in package proxy_test)
// These are the same constants with the first letter capitalized for export.
// ---------------------------------------------------------------------------

const (
	// AnthropicVersion is the default Anthropic API version for tests.
	AnthropicVersion = anthropicVersion
	// AnthropicBaseURL is the default Anthropic API base URL for tests.
	AnthropicBaseURL = anthropicBaseURL
	// AnthropicVersion2024 is the 2024 Anthropic API version for tests.
	AnthropicVersion2024 = anthropicVersion2024
	// AnthropicBetaExtendedThinking is the extended thinking beta header value.
	AnthropicBetaExtendedThinking = anthropicBetaExtendedThinking
	// MessagesPath is the messages API endpoint path.
	MessagesPath = messagesPath
	// APIKeyHeader is the API key header name.
	APIKeyHeader = apiKeyHeader
	// ContentTypeHeader is the content type header name.
	ContentTypeHeader = contentTypeHeader
	// JSONContentType is the JSON content type value.
	JSONContentType = jsonContentType
	// ListObject is the "list" object value for API responses.
	ListObject = listObject
	// ValidTestSignature is a valid test signature long enough for validation.
	ValidTestSignature = validTestSignature
)

// ---------------------------------------------------------------------------
// Internal test types (lowercase, used by tests in package proxy)
// ---------------------------------------------------------------------------

type recordingBackend struct {
	body []byte
	mu   sync.Mutex
}

func (rec *recordingBackend) Body() []byte {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.body == nil {
		return nil
	}
	return append([]byte(nil), rec.body...)
}

type headerPair struct {
	Key   string
	Value string
}

// ---------------------------------------------------------------------------
// Exported types (uppercase aliases, used by tests in package proxy_test)
// ---------------------------------------------------------------------------

// RecordingBackend exports the recordingBackend type for proxy_test package.
type RecordingBackend = recordingBackend

// HeaderPair exports the headerPair type for proxy_test package.
type HeaderPair = headerPair

// ---------------------------------------------------------------------------
// Internal test helper functions (lowercase, used by tests in package proxy)
// ---------------------------------------------------------------------------

func newRecordingBackend(t *testing.T) (*httptest.Server, *recordingBackend) {
	t.Helper()

	recorder := &recordingBackend{
		body: nil,
		mu:   sync.Mutex{},
	}
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				return
			}
			recorder.mu.Lock()
			recorder.body = body
			recorder.mu.Unlock()

			writer.Header().Set(contentTypeHeader, jsonContentType)
			writer.WriteHeader(http.StatusOK)
			if _, writeErr := writer.Write([]byte(`{"content": []}`)); writeErr != nil {
				return
			}
		},
	))
	t.Cleanup(server.Close)
	return server, recorder
}

func newBackendServer(t *testing.T, bodyStr string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
			if bodyStr != "" {
				if _, err := writer.Write([]byte(bodyStr)); err != nil {
					return
				}
			}
		},
	))
	t.Cleanup(server.Close)
	return server
}

func newJSONBackend(t *testing.T, bodyStr string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set(contentTypeHeader, jsonContentType)
			writer.WriteHeader(http.StatusOK)
			if bodyStr != "" {
				if _, err := writer.Write([]byte(bodyStr)); err != nil {
					return
				}
			}
		},
	))
	t.Cleanup(server.Close)
	return server
}

func newStatusBackend(
	t *testing.T,
	status int,
	bodyStr string,
	headers map[string]string,
) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			for key, value := range headers {
				writer.Header().Set(key, value)
			}
			writer.WriteHeader(status)
			if bodyStr != "" {
				if _, err := writer.Write([]byte(bodyStr)); err != nil {
					return
				}
			}
		},
	))
	t.Cleanup(server.Close)
	return server
}

func newMessagesRequest(body io.Reader) *http.Request {
	req := httptest.NewRequest("POST", messagesPath, body)
	req.Header.Set("anthropic-version", anthropicVersion)
	return req
}

func newMessagesRequestWithHeaders(bodyStr string, headers ...headerPair) *http.Request {
	req := newMessagesRequest(bytes.NewReader([]byte(bodyStr)))
	for _, header := range headers {
		req.Header.Set(header.Key, header.Value)
	}
	return req
}

func serveRequest(
	t *testing.T,
	handler http.Handler,
	req *http.Request,
) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func newTestProvider(providerURL string) providers.Provider {
	return providers.NewAnthropicProvider("test", providerURL)
}

func newNamedProvider(name, providerURL string) providers.Provider {
	return providers.NewAnthropicProvider(name, providerURL)
}

func newTestSignatureCache(t *testing.T) (sigCache *SignatureCache, cleanup func()) {
	t.Helper()
	cfg := cache.Config{
		Mode: cache.ModeSingle,
		Olric: cache.OlricConfig{
			DMapName:          "",
			BindAddr:          "",
			Environment:       "",
			Addresses:         nil,
			Peers:             nil,
			ReplicaCount:      0,
			ReadQuorum:        0,
			WriteQuorum:       0,
			LeaveTimeout:      0,
			MemberCountQuorum: 0,
			Embedded:          false,
		},
		Ristretto: cache.RistrettoConfig{
			NumCounters: 1e4,
			MaxCost:     1 << 20,
			BufferItems: 64,
		},
	}
	cacheInstance, err := cache.New(context.Background(), &cfg)
	require.NoError(t, err)

	sigCache = NewSignatureCache(cacheInstance)
	cleanup = func() {
		if closeErr := cacheInstance.Close(); closeErr != nil {
			// Cleanup errors in tests are non-fatal; log for debugging
			t.Logf("cache close error: %v", closeErr)
		}
	}
	return sigCache, cleanup
}

// testDebugOptions returns an empty DebugOptions struct for testing.
// All fields are explicitly initialized to satisfy exhaustruct linter.
func testDebugOptions() config.DebugOptions {
	return config.DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     0,
	}
}

// testLoggingConfig returns a minimal LoggingConfig for testing.
// All fields are explicitly initialized to satisfy exhaustruct linter.
func testLoggingConfig() config.LoggingConfig {
	return config.LoggingConfig{
		Level:        "",
		Format:       "",
		Output:       "",
		Pretty:       false,
		DebugOptions: testDebugOptions(),
	}
}

// testServerConfig returns a minimal ServerConfig for testing.
// All fields are explicitly initialized to satisfy exhaustruct linter.
func testServerConfig(apiKey string) config.ServerConfig {
	return config.ServerConfig{
		Listen:        "",
		APIKey:        apiKey,
		Auth:          testAuthConfig(),
		TimeoutMS:     0,
		MaxConcurrent: 0,
		MaxBodyBytes:  0,
		EnableHTTP2:   false,
	}
}

// testRoutingConfig returns a minimal RoutingConfig for testing.
// All fields are explicitly initialized to satisfy exhaustruct linter.
func testRoutingConfig() config.RoutingConfig {
	return config.RoutingConfig{
		ModelMapping:    nil,
		Strategy:        "",
		DefaultProvider: "",
		FailoverTimeout: 0,
		Debug:           false,
	}
}

// testAuthConfig returns an empty AuthConfig for testing.
// All fields are explicitly initialized to satisfy exhaustruct linter.
func testAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		APIKey:            "",
		BearerSecret:      "",
		AllowBearer:       false,
		AllowSubscription: false,
	}
}

// testHealthConfig returns a minimal health.Config for testing.
// All fields are explicitly initialized to satisfy exhaustruct linter.
func testHealthConfig() health.Config {
	return health.Config{
		HealthCheck: health.CheckConfig{
			Enabled:    nil,
			IntervalMS: 0,
		},
		CircuitBreaker: health.CircuitBreakerConfig{
			OpenDurationMS:   0,
			FailureThreshold: 0,
			HalfOpenProbes:   0,
		},
	}
}

// testCacheConfig returns a minimal cache.Config for testing.
// All fields are explicitly initialized to satisfy exhaustruct linter.
func testCacheConfig() cache.Config {
	return cache.Config{
		Mode: "",
		Olric: cache.OlricConfig{
			DMapName:          "",
			BindAddr:          "",
			Environment:       "",
			Addresses:         nil,
			Peers:             nil,
			ReplicaCount:      0,
			ReadQuorum:        0,
			WriteQuorum:       0,
			LeaveTimeout:      0,
			MemberCountQuorum: 0,
			Embedded:          false,
		},
		Ristretto: cache.RistrettoConfig{
			NumCounters: 0,
			MaxCost:     0,
			BufferItems: 0,
		},
	}
}

// testConfig returns a minimal config.Config for testing.
// All fields are explicitly initialized to satisfy exhaustruct linter.
func testConfig(apiKey string) *config.Config {
	return &config.Config{
		Providers: nil,
		Server:    testServerConfig(apiKey),
		Routing:   testRoutingConfig(),
		Logging:   testLoggingConfig(),
		Health:    testHealthConfig(),
		Cache:     testCacheConfig(),
	}
}

// testHandlerOptions returns a *HandlerOptions initialized with
// the provided non-nil values, and all other fields set to their zero values.
// This satisfies the exhaustruct linter by explicitly initializing all fields.
func testHandlerOptions(opts *HandlerOptions) *HandlerOptions {
	if opts == nil {
		opts = &HandlerOptions{
			ProviderRouter:    nil,
			Provider:          nil,
			ProviderPools:     nil,
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
			DebugOptions:      testDebugOptions(),
			RoutingDebug:      false,
		}
	}
	return &HandlerOptions{
		ProviderRouter:    opts.ProviderRouter,
		Provider:          opts.Provider,
		ProviderPools:     opts.ProviderPools,
		ProviderInfosFunc: opts.ProviderInfosFunc,
		Pool:              opts.Pool,
		ProviderKeys:      opts.ProviderKeys,
		GetProviderPools:  opts.GetProviderPools,
		GetProviderKeys:   opts.GetProviderKeys,
		RoutingConfig:     opts.RoutingConfig,
		HealthTracker:     opts.HealthTracker,
		SignatureCache:    opts.SignatureCache,
		APIKey:            opts.APIKey,
		ProviderInfos:     opts.ProviderInfos,
		DebugOptions:      testDebugOptions(),
		RoutingDebug:      opts.RoutingDebug,
	}
}

// mustTestHandlerOptions creates a *HandlerOptions for testing.
// This is a convenience function for tests that don't need to handle the error.
func mustTestHandlerOptions(provider providers.Provider, apiKey string) *HandlerOptions {
	return testHandlerOptions(&HandlerOptions{
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
		APIKey:            apiKey,
		ProviderInfos:     nil,
		DebugOptions:      testDebugOptions(),
		RoutingDebug:      false,
	})
}

// testKeyConfig returns a keypool.KeyConfig with the given API key.
// All fields are explicitly initialized to satisfy exhaustruct linter.
func testKeyConfig(apiKey string) keypool.KeyConfig {
	return keypool.KeyConfig{
		APIKey:    apiKey,
		RPMLimit:  0,
		ITPMLimit: 0,
		OTPMLimit: 0,
		Priority:  0,
		Weight:    0,
	}
}

// testProviderInfo returns a router.ProviderInfo with the given provider.
// All fields are explicitly initialized to satisfy exhaustruct linter.
func testProviderInfo(provider providers.Provider) router.ProviderInfo {
	return router.ProviderInfo{
		Provider:  provider,
		IsHealthy: func() bool { return true },
		Weight:    0,
		Priority:  0,
	}
}

// testProviderInfoWithHealth returns a router.ProviderInfo with custom health function.
// All fields are explicitly initialized to satisfy exhaustruct linter.
func testProviderInfoWithHealth(provider providers.Provider, isHealthy func() bool) router.ProviderInfo {
	return router.ProviderInfo{
		Provider:  provider,
		IsHealthy: isHealthy,
		Weight:    0,
		Priority:  0,
	}
}

// testRoutesOptions returns a RoutesOptions struct initialized with
// the provided non-nil values, and all other fields set to their zero values.
// This satisfies the exhaustruct linter by explicitly initializing all fields.
func testRoutesOptions(opts *RoutesOptions) RoutesOptions {
	if opts == nil {
		opts = &RoutesOptions{
			ProviderRouter:     nil,
			Provider:           nil,
			ConfigProvider:     nil,
			Pool:               nil,
			ProviderInfosFunc:  nil,
			ProviderPools:      nil,
			ProviderKeys:       nil,
			GetProviderPools:   nil,
			GetProviderKeys:    nil,
			GetAllProviders:    nil,
			HealthTracker:      nil,
			SignatureCache:     nil,
			ConcurrencyLimiter: nil,
			ProviderKey:        "",
			ProviderInfos:      nil,
			AllProviders:       nil,
		}
	}
	return RoutesOptions{
		ProviderRouter:     opts.ProviderRouter,
		Provider:           opts.Provider,
		ConfigProvider:     opts.ConfigProvider,
		Pool:               opts.Pool,
		ProviderInfosFunc:  opts.ProviderInfosFunc,
		ProviderPools:      opts.ProviderPools,
		ProviderKeys:       opts.ProviderKeys,
		GetProviderPools:   opts.GetProviderPools,
		GetProviderKeys:    opts.GetProviderKeys,
		GetAllProviders:    opts.GetAllProviders,
		HealthTracker:      opts.HealthTracker,
		SignatureCache:     opts.SignatureCache,
		ConcurrencyLimiter: opts.ConcurrencyLimiter,
		ProviderKey:        opts.ProviderKey,
		ProviderInfos:      opts.ProviderInfos,
		AllProviders:       opts.AllProviders,
	}
}

func newHandlerWithSignatureCache(
	t *testing.T,
	provider providers.Provider,
	sigCache *SignatureCache,
) *Handler {
	t.Helper()
	handler, err := NewHandler(&HandlerOptions{
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
		SignatureCache:    sigCache,
		APIKey:            "test-key",
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

// ---------------------------------------------------------------------------
// Exported function variables (uppercase, used by tests in package proxy_test)
// ---------------------------------------------------------------------------

var (
	// NewRecordingBackend creates a test backend that records request bodies.
	NewRecordingBackend = newRecordingBackend
	// NewBackendServer creates a test HTTP server that returns a fixed body.
	NewBackendServer = newBackendServer
	// NewJSONBackend creates a test HTTP server that returns JSON.
	NewJSONBackend = newJSONBackend
	// NewStatusBackend creates a test HTTP server with specific status.
	NewStatusBackend = newStatusBackend
	// NewMessagesRequest creates a test messages request.
	NewMessagesRequest = newMessagesRequest
	// NewMessagesRequestWithHeaders creates a test request with headers.
	NewMessagesRequestWithHeaders = newMessagesRequestWithHeaders
	// ServeRequest serves an HTTP request for testing.
	ServeRequest = serveRequest
	// NewTestProvider creates a test Anthropic provider.
	NewTestProvider = newTestProvider
	// NewNamedProvider creates a test provider with a name.
	NewNamedProvider = newNamedProvider
	// NewTestSignatureCache creates a signature cache for testing.
	NewTestSignatureCache = newTestSignatureCache
	// NewHandlerWithSignatureCache creates a handler with signature cache.
	NewHandlerWithSignatureCache = newHandlerWithSignatureCache
	// TestDebugOptions returns an empty DebugOptions struct for testing.
	TestDebugOptions = testDebugOptions
	// TestLoggingConfig returns a minimal LoggingConfig for testing.
	TestLoggingConfig = testLoggingConfig
	// TestServerConfig returns a minimal ServerConfig for testing.
	TestServerConfig = testServerConfig
	// TestRoutingConfig returns a minimal RoutingConfig for testing.
	TestRoutingConfig = testRoutingConfig
	// TestHealthConfig returns a minimal health.Config for testing.
	TestHealthConfig = testHealthConfig
	// TestCacheConfig returns a minimal cache.Config for testing.
	TestCacheConfig = testCacheConfig
	// TestConfig returns a minimal config.Config for testing.
	TestConfig = testConfig
	// TestHandlerOptions returns a HandlerOptions struct for testing.
	TestHandlerOptions = testHandlerOptions
	// MustTestHandlerOptions creates a HandlerOptions for testing.
	MustTestHandlerOptions = mustTestHandlerOptions
	// TestKeyConfig returns a keypool.KeyConfig for testing.
	TestKeyConfig = testKeyConfig
	// TestProviderInfo returns a router.ProviderInfo for testing.
	TestProviderInfo = testProviderInfo
	// TestProviderInfoWithHealth returns a router.ProviderInfo with custom health.
	TestProviderInfoWithHealth = testProviderInfoWithHealth
	// TestRoutesOptions returns a RoutesOptions struct for testing.
	TestRoutesOptions = testRoutesOptions
	// TestAuthConfig returns an AuthConfig for testing.
	TestAuthConfig = testAuthConfig
)

// ---------------------------------------------------------------------------
// Server accessor functions (exported for proxy_test package)
// These provide access to unexported Server fields for testing.
// ---------------------------------------------------------------------------

// GetServerAddr returns the address of a Server.
func GetServerAddr(s *Server) string {
	return s.addr
}

// GetHTTPServer returns the underlying http.Server of a Server.
func GetHTTPServer(s *Server) *http.Server {
	return s.httpServer
}

// GetHTTPServerHandler returns the handler from the underlying http.Server.
func GetHTTPServerHandler(s *Server) http.Handler {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Handler
}

// TLSVersionString converts TLS version constant to string.
// Exported for testing in proxy_test package.
var TLSVersionString = tlsVersionString

// ExtractSSEData extracts data from SSE event lines.
// Exported for testing in proxy_test package.
var ExtractSSEData = extractSSEData

// FindProviderForModel finds the provider for a model using the model mapping.
// Exported for testing in proxy_test package.
var FindProviderForModel = findProviderForModel

// ---------------------------------------------------------------------------
// Handler accessor functions (exported for proxy_test package)
// These provide access to unexported Handler fields and methods for testing.
// ---------------------------------------------------------------------------

// GetHandlerDefaultProvider returns the default provider from a Handler.
func GetHandlerDefaultProvider(h *Handler) providers.Provider {
	return h.defaultProvider
}

// GetHandlerProviderProxies returns the provider proxies map from a Handler.
func GetHandlerProviderProxies(h *Handler) map[string]*ProviderProxy {
	return h.providerProxies
}

// HandlerSelectProvider calls the unexported selectProvider method on a Handler.
func HandlerSelectProvider(
	ctx context.Context, h *Handler, model string, hasThinking bool,
) (router.ProviderInfo, error) {
	return h.selectProvider(ctx, model, hasThinking)
}

// HandlerGetOrCreateProxy calls the unexported getOrCreateProxy method on a Handler.
func HandlerGetOrCreateProxy(h *Handler, provider providers.Provider) (*ProviderProxy, error) {
	return h.getOrCreateProxy(provider)
}

// SetHandlerGetProviderKeys sets the getProviderKeys function on a Handler.
func SetHandlerGetProviderKeys(h *Handler, fn func() map[string]string) {
	h.getProviderKeys = fn
}

// ParseRetryAfter parses the Retry-After header from an HTTP response.
// Exported for testing in proxy_test package.
var ParseRetryAfter = parseRetryAfter

// ResponseWriter exports the responseWriter type for proxy_test package.
type ResponseWriter = responseWriter

// NewTestResponseWriter creates a responseWriter for testing.
func NewTestResponseWriter() *ResponseWriter {
	rec := httptest.NewRecorder()
	return &responseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
		sseEvents:      0,
		isStreaming:     false,
	}
}

// FormatDuration exports formatDuration for proxy_test package.
var FormatDuration = formatDuration

// AuthFingerprint exports authFingerprint for proxy_test package.
var AuthFingerprint = authFingerprint

// FormatCompletionMessage exports formatCompletionMessage for proxy_test package.
var FormatCompletionMessage = formatCompletionMessage

// RedactSensitiveFields exports redactSensitiveFields for proxy_test package.
var RedactSensitiveFields = redactSensitiveFields

// EventStreamToSSEBody exports the eventStreamToSSEBody type for proxy_test package.
type EventStreamToSSEBody = eventStreamToSSEBody

// NewEventStreamToSSEBody exports newEventStreamToSSEBody for proxy_test package.
var NewEventStreamToSSEBody = newEventStreamToSSEBody

// CacheKey exports the cacheKey method on SignatureCache for proxy_test package.
func CacheKey(sc *SignatureCache, modelGroup, thinkingText string) string {
	return sc.cacheKey(modelGroup, thinkingText)
}

// GetResponseWriterStatusCode returns the statusCode field from a responseWriter.
func GetResponseWriterStatusCode(rw *ResponseWriter) int {
	return rw.statusCode
}

// GetResponseWriterIsStreaming returns the isStreaming field from a responseWriter.
func GetResponseWriterIsStreaming(rw *ResponseWriter) bool {
	return rw.isStreaming
}

// GetResponseWriterSSEEvents returns the sseEvents field from a responseWriter.
func GetResponseWriterSSEEvents(rw *ResponseWriter) int {
	return rw.sseEvents
}

