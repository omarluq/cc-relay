package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	wrongKey            = "wrong-key"
	expectedStatusOKMsg = "expected status 200"
	keyV1               = "key-v1"
	keyV2               = "key-v2"
	keyAlpha            = "key-alpha"
	keyBeta             = "key-beta"
	concurrentKey       = "concurrent-key"
)

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, expected int, msg string) {
	t.Helper()
	if rec.Code != expected {
		t.Errorf("%s: got %d", msg, rec.Code)
	}
}

func assertStatusCode(t *testing.T, got, expected int, msg string) {
	t.Helper()
	if got != expected {
		t.Errorf("%s: got %d", msg, got)
	}
}

func doAPIKeyRequest(t *testing.T, handler http.Handler, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := newMessagesRequest(http.NoBody)
	req.Header.Set(apiKeyHeader, key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func assertKeyWithHandler(t *testing.T, handler http.Handler, key string, expected int, msg string) {
	t.Helper()
	rec := doAPIKeyRequest(t, handler, key)
	assertStatus(t, rec, expected, msg)
}

func runConcurrentRequests(
	t *testing.T,
	handler http.Handler,
	workers, iterations int,
	keyFn func(workerID, iter int) (string, int),
) {
	t.Helper()

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key, expected := keyFn(id, j)
				rec := doAPIKeyRequest(t, handler, key)
				if rec.Code != expected {
					t.Errorf("goroutine %d request %d: expected %d, got %d", id, j, expected, rec.Code)
				}
			}
		}(i)
	}

	wg.Wait()
}

func startConfigSwitcher(
	wg *sync.WaitGroup,
	runtime *config.Runtime,
	cfg1, cfg2 *config.Config,
	iterations int,
) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			if i%2 == 0 {
				runtime.Store(cfg1)
			} else {
				runtime.Store(cfg2)
			}
			time.Sleep(time.Microsecond)
		}
	}()
}

func TestAuthMiddlewareValidKey(t *testing.T) {
	t.Parallel()

	handler := okHandler()
	middleware := AuthMiddleware("secret-key")
	wrappedHandler := middleware(handler)

	req := newMessagesRequest(http.NoBody)
	req.Header.Set(apiKeyHeader, "secret-key")

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK, expectedStatusOKMsg)
}

func TestAuthMiddlewareInvalidKey(t *testing.T) {
	t.Parallel()

	handler := okHandler()
	middleware := AuthMiddleware("secret-key")
	wrappedHandler := middleware(handler)

	req := newMessagesRequest(http.NoBody)
	req.Header.Set(apiKeyHeader, wrongKey)

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusUnauthorized, "expected status 401")
}

func TestAuthMiddlewareMissingKey(t *testing.T) {
	t.Parallel()

	handler := okHandler()
	middleware := AuthMiddleware("secret-key")
	wrappedHandler := middleware(handler)

	req := newMessagesRequest(http.NoBody)
	// No x-api-key header

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusUnauthorized, "expected status 401")

	// Verify error message
	if !strings.Contains(rec.Body.String(), "missing x-api-key header") {
		t.Errorf("Expected error about missing header, got: %s", rec.Body.String())
	}
}

func TestMultiAuthMiddlewareNoAuthConfigured(t *testing.T) {
	t.Parallel()

	handler := okHandler()
	authConfig := &config.AuthConfig{}
	middleware := MultiAuthMiddleware(authConfig)
	wrappedHandler := middleware(handler)

	req := newMessagesRequest(http.NoBody)
	// No auth headers

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	// Should pass through when no auth configured
	assertStatus(t, rec, http.StatusOK, "expected status 200 when no auth configured")
}

func TestMultiAuthMiddlewareBearerOnly(t *testing.T) {
	t.Parallel()

	handler := okHandler()
	authConfig := &config.AuthConfig{
		AllowBearer:  true,
		BearerSecret: "test-bearer-secret",
	}
	middleware := MultiAuthMiddleware(authConfig)
	wrappedHandler := middleware(handler)

	// Valid bearer token
	req := newMessagesRequest(http.NoBody)
	req.Header.Set("Authorization", "Bearer test-bearer-secret")

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK, "expected status 200 with valid bearer")
}

func TestMultiAuthMiddlewareAPIKeyOnly(t *testing.T) {
	t.Parallel()

	handler := okHandler()
	authConfig := &config.AuthConfig{
		APIKey: "test-api-key",
	}
	middleware := MultiAuthMiddleware(authConfig)
	wrappedHandler := middleware(handler)

	// Valid API key
	req := newMessagesRequest(http.NoBody)
	req.Header.Set(apiKeyHeader, "test-api-key")

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK, "expected status 200 with valid API key")
}

func TestMultiAuthMiddlewareBothMethods(t *testing.T) {
	t.Parallel()

	handler := okHandler()
	authConfig := &config.AuthConfig{
		APIKey:       "test-api-key",
		AllowBearer:  true,
		BearerSecret: "test-bearer-secret",
	}
	middleware := MultiAuthMiddleware(authConfig)
	wrappedHandler := middleware(handler)

	// Test with bearer - should work
	t.Run("bearer works", func(t *testing.T) {
		t.Parallel()
		req := newMessagesRequest(http.NoBody)
		req.Header.Set("Authorization", "Bearer test-bearer-secret")

		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK, "expected status 200 with bearer")
	})

	// Test with API key - should work
	t.Run("api key works", func(t *testing.T) {
		t.Parallel()
		req := newMessagesRequest(http.NoBody)
		req.Header.Set(apiKeyHeader, "test-api-key")

		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK, "expected status 200 with API key")
	})
}

func TestMultiAuthMiddlewareSubscriptionAlias(t *testing.T) {
	t.Parallel()

	handler := okHandler()
	authConfig := &config.AuthConfig{
		AllowSubscription: true, // Alias for AllowBearer
	}
	middleware := MultiAuthMiddleware(authConfig)
	wrappedHandler := middleware(handler)

	// Any bearer token should work (passthrough mode)
	req := newMessagesRequest(http.NoBody)
	req.Header.Set("Authorization", "Bearer any-subscription-token")

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK, "expected status 200 with subscription token")
}

func TestLiveAuthMiddlewareToggleAPIKey(t *testing.T) {
	t.Parallel()

	runtimeCfg := config.NewRuntime(&config.Config{
		Server: config.ServerConfig{APIKey: "test-key"},
	})

	handler := okHandler()

	wrapped := LiveAuthMiddleware(runtimeCfg)(handler)

	req := newMessagesRequest(http.NoBody)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when API key required, got %d", rec.Code)
	}

	runtimeCfg.Store(&config.Config{
		Server: config.ServerConfig{APIKey: ""},
	})

	req2 := newMessagesRequest(http.NoBody)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 when API key disabled, got %d", rec2.Code)
	}
}

func TestRequestIDMiddlewareGeneratesID(t *testing.T) {
	t.Parallel()

	var capturedRequestID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestIDMiddleware()
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	// Should have generated a UUID
	if capturedRequestID == "" {
		t.Error("Expected generated request ID, got empty")
	}

	// Should be in response header
	responseID := rec.Header().Get("X-Request-ID")
	if responseID == "" {
		t.Error("Expected X-Request-ID in response header")
	}
}

func TestRequestIDMiddlewareUsesProvidedID(t *testing.T) {
	t.Parallel()

	providedID := "custom-request-id-123"
	var capturedRequestID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestIDMiddleware()
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Request-ID", providedID)

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	// Should use provided ID
	if capturedRequestID != providedID {
		t.Errorf("Expected request ID %s, got %s", providedID, capturedRequestID)
	}

	// Should echo in response
	responseID := rec.Header().Get("X-Request-ID")
	if responseID != providedID {
		t.Errorf("Expected response ID %s, got %s", providedID, responseID)
	}
}

func TestLoggingMiddlewareLogsRequest(t *testing.T) {
	t.Parallel()

	handler := okHandler()

	debugOpts := config.DebugOptions{}
	middleware := LoggingMiddleware(debugOpts)

	// Wrap with RequestIDMiddleware first (as in production)
	wrappedHandler := RequestIDMiddleware()(middleware(handler))

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"test"}`))
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK, expectedStatusOKMsg)
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		d        time.Duration
	}{
		{"micro request", "500µs", 500 * time.Microsecond},
		{"fast request", "100.00ms", 100 * time.Millisecond},
		{"medium request", "500.00ms", 500 * time.Millisecond},
		{"slow request", "1.50s", 1500 * time.Millisecond},
		{"very slow request", "5.00s", 5 * time.Second},
		{"minutes request", "2m3s", 2*time.Minute + 3*time.Second},
		{"zero", "0s", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatDuration(tt.d)
			if got != tt.expected {
				t.Errorf("formatDuration(%s) = %s, want %s", tt.d, got, tt.expected)
			}
		})
	}
}

func TestFormatCompletionMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		symbol   string
		duration string
		expected string
		status   int
	}{
		{"!", "100ms", "! OK (100ms)", 200},
		{"?", "50ms", "? Not Found (50ms)", 404},
		{"x", "1.5s", "x Internal Server Error (1.5s)", 500},
	}

	for _, tt := range tests {
		name := http.StatusText(tt.status)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := formatCompletionMessage(tt.status, tt.symbol, tt.duration)
			if got != tt.expected {
				t.Errorf("formatCompletionMessage(%d, %s, %s) = %s, want %s",
					tt.status, tt.symbol, tt.duration, got, tt.expected)
			}
		})
	}
}

func TestRedactSensitiveFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		contains    string
		notContains string
	}{
		{
			name:        "redacts api_key",
			input:       `{"api_key": "sk-secret-123", "model": "claude"}`,
			contains:    "REDACTED",
			notContains: "sk-secret-123",
		},
		{
			name:        "preserves non-sensitive fields",
			input:       `{"model": "claude", "max_tokens": 100}`,
			contains:    "claude",
			notContains: "REDACTED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := redactSensitiveFields(tt.input)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("Expected output to contain %q, got: %s", tt.contains, got)
			}
			if tt.notContains != "" && strings.Contains(got, tt.notContains) {
				t.Errorf("Expected output to NOT contain %q, got: %s", tt.notContains, got)
			}
		})
	}
}

func TestResponseWriterCapturesStatusCode(t *testing.T) {
	t.Parallel()

	rw := newTestResponseWriter()

	rw.WriteHeader(http.StatusNotFound)

	assertStatusCode(t, rw.statusCode, http.StatusNotFound, "expected status 404")
}

func TestResponseWriterDetectsStreaming(t *testing.T) {
	t.Parallel()

	rw := newTestResponseWriter()

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.WriteHeader(http.StatusOK)

	if !rw.isStreaming {
		t.Error("Expected isStreaming to be true for text/event-stream")
	}
}

func TestResponseWriterCountsSSEEvents(t *testing.T) {
	t.Parallel()

	rw := newTestResponseWriter()

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.WriteHeader(http.StatusOK)

	// Write SSE events
	sseData := "event: message_start\ndata: {}\n\nevent: content_block_start\ndata: {}\n\n"
	_, err := rw.Write([]byte(sseData))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if rw.sseEvents != 2 {
		t.Errorf("Expected 2 SSE events, got %d", rw.sseEvents)
	}
}

func TestAuthFingerprint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		bearerSecret  string
		apiKey        string
		bearerEnabled bool
	}{
		{"no auth", "", "", false},
		{"bearer only", "secret", "", true},
		{"api key only", "", "my-key", false},
		{"both enabled", "bearer-secret", "api-key", true},
		{"bearer no secret", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fp := authFingerprint(tt.bearerEnabled, tt.bearerSecret, tt.apiKey)
			require.NotEmpty(t, fp, "fingerprint should not be empty")

			// Same inputs produce same fingerprint
			fp2 := authFingerprint(tt.bearerEnabled, tt.bearerSecret, tt.apiKey)
			assert.Equalf(t, fp, fp2, "fingerprint not deterministic: %q != %q", fp, fp2)
		})
	}

	// Different inputs produce different fingerprints
	t.Run("different inputs differ", func(t *testing.T) {
		t.Parallel()
		fp1 := authFingerprint(true, "secret1", "key1")
		fp2 := authFingerprint(true, "secret2", "key1")
		fp3 := authFingerprint(false, "secret1", "key1")
		fp4 := authFingerprint(true, "secret1", "key2")

		assert.NotEqual(t, fp1, fp2, "different bearer secrets should produce different fingerprints")
		assert.NotEqual(t, fp1, fp3, "different bearer enabled should produce different fingerprints")
		assert.NotEqual(t, fp1, fp4, "different api keys should produce different fingerprints")
	})

	// Delimiter collision resistance - secrets containing delimiters must not collide
	t.Run("delimiter collision resistance", func(t *testing.T) {
		t.Parallel()
		// These would collide with naive delimiter-based format
		fp1 := authFingerprint(true, "secret|5:fake", "real")
		fp2 := authFingerprint(true, "secret", "fake|5:real")
		if fp1 == fp2 {
			t.Error("fingerprints should not collide when secrets contain delimiters")
		}

		// Additional edge case with length-like patterns
		fp3 := authFingerprint(true, "a|3:bcd", "ef")
		fp4 := authFingerprint(true, "a", "bcd|2:ef")
		if fp3 == fp4 {
			t.Error("fingerprints should not collide with length-like patterns")
		}
	})
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func handlerWithCalled(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func newTestResponseWriter() *responseWriter {
	rec := httptest.NewRecorder()
	return &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}
}

func TestLiveAuthMiddlewareNilProvider(t *testing.T) {
	t.Parallel()

	called := false
	handler := handlerWithCalled(&called)

	middleware := LiveAuthMiddleware(nil)
	wrappedHandler := middleware(handler)

	req := newMessagesRequest(http.NoBody)
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler should be called when provider is nil")
	}
	assertStatus(t, rec, http.StatusOK, expectedStatusOKMsg)
}

func TestLiveAuthMiddlewareNilConfig(t *testing.T) {
	t.Parallel()

	called := false
	handler := handlerWithCalled(&called)

	runtime := config.NewRuntime(nil)
	middleware := LiveAuthMiddleware(runtime)
	wrappedHandler := middleware(handler)

	req := newMessagesRequest(http.NoBody)
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler should be called when config is nil")
	}
	assertStatus(t, rec, http.StatusOK, expectedStatusOKMsg)
}

func TestLiveAuthMiddlewareNoAuthConfigured(t *testing.T) {
	t.Parallel()

	called := false
	handler := handlerWithCalled(&called)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{},
		},
	}
	runtime := config.NewRuntime(cfg)
	middleware := LiveAuthMiddleware(runtime)
	wrappedHandler := middleware(handler)

	req := newMessagesRequest(http.NoBody)
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler should be called when no auth configured")
	}
	assertStatus(t, rec, http.StatusOK, expectedStatusOKMsg)
}

func TestLiveAuthMiddlewareAPIKeyAuth(t *testing.T) {
	t.Parallel()

	newWrappedHandler := func() http.Handler {
		handler := okHandler()
		cfg := &config.Config{
			Server: config.ServerConfig{
				Auth: config.AuthConfig{
					APIKey: "test-api-key",
				},
			},
		}
		runtime := config.NewRuntime(cfg)
		middleware := LiveAuthMiddleware(runtime)
		return middleware(handler)
	}

	t.Run("valid key", func(t *testing.T) {
		t.Parallel()
		wrappedHandler := newWrappedHandler()
		req := newMessagesRequest(http.NoBody)
		req.Header.Set(apiKeyHeader, "test-api-key")
		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)

		assertStatus(t, rec, http.StatusOK, expectedStatusOKMsg)
	})

	t.Run("invalid key", func(t *testing.T) {
		t.Parallel()
		wrappedHandler := newWrappedHandler()
		req := newMessagesRequest(http.NoBody)
		req.Header.Set(apiKeyHeader, wrongKey)
		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		t.Parallel()
		wrappedHandler := newWrappedHandler()
		req := newMessagesRequest(http.NoBody)
		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})
}

func TestLiveAuthMiddlewareConfigSwitching(t *testing.T) {
	t.Parallel()

	handler := okHandler()

	// Start with API key auth
	cfg1 := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey: keyV1,
			},
		},
	}
	runtime := config.NewRuntime(cfg1)
	middleware := LiveAuthMiddleware(runtime)
	wrappedHandler := middleware(handler)

	// Request with key-v1 should succeed
	assertKeyWithHandler(t, wrappedHandler, keyV1, http.StatusOK, "key-v1 should work with cfg1")

	// Switch to new API key
	cfg2 := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey: keyV2,
			},
		},
	}
	runtime.Store(cfg2)

	// Old key should now fail
	assertKeyWithHandler(t, wrappedHandler, keyV1, http.StatusUnauthorized, "key-v1 should fail after switch")

	// New key should work
	assertKeyWithHandler(t, wrappedHandler, keyV2, http.StatusOK, "key-v2 should work after switch")
}

func TestLiveAuthMiddlewareSwitchAuthMethods(t *testing.T) {
	t.Parallel()

	handler := okHandler()

	// Start with API key auth
	cfg1 := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey: "my-api-key",
			},
		},
	}
	runtime := config.NewRuntime(cfg1)
	middleware := LiveAuthMiddleware(runtime)
	wrappedHandler := middleware(handler)

	// Bearer should fail, API key should work
	req1 := newMessagesRequest(http.NoBody)
	req1.Header.Set("Authorization", "Bearer my-bearer-token")
	rec1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusUnauthorized {
		t.Errorf("bearer should fail with API key only: got %d", rec1.Code)
	}

	// Switch to bearer auth
	cfg2 := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				AllowBearer:  true,
				BearerSecret: "my-bearer-token",
			},
		},
	}
	runtime.Store(cfg2)

	// Now bearer should work
	req2 := newMessagesRequest(http.NoBody)
	req2.Header.Set("Authorization", "Bearer my-bearer-token")
	rec2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("bearer should work after switch: got %d", rec2.Code)
	}

	// API key should now fail
	req3 := newMessagesRequest(http.NoBody)
	req3.Header.Set(apiKeyHeader, "my-api-key")
	rec3 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusUnauthorized {
		t.Errorf("api key should fail after switch to bearer only: got %d", rec3.Code)
	}
}

func TestLiveAuthMiddlewareSwitchToNoAuth(t *testing.T) {
	t.Parallel()

	handler := okHandler()

	// Start with API key auth
	cfg1 := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey: "required-key",
			},
		},
	}
	runtime := config.NewRuntime(cfg1)
	middleware := LiveAuthMiddleware(runtime)
	wrappedHandler := middleware(handler)

	// No key should fail
	req1 := newMessagesRequest(http.NoBody)
	rec1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusUnauthorized {
		t.Errorf("should require auth initially: got %d", rec1.Code)
	}

	// Switch to no auth
	cfg2 := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{},
		},
	}
	runtime.Store(cfg2)

	// Now no key should pass through
	req2 := newMessagesRequest(http.NoBody)
	rec2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("should pass through with no auth configured: got %d", rec2.Code)
	}
}

func TestLiveAuthMiddlewareConcurrentAccess(t *testing.T) {
	t.Parallel()

	handler := okHandler()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey: concurrentKey,
			},
		},
	}
	runtime := config.NewRuntime(cfg)
	middleware := LiveAuthMiddleware(runtime)
	wrappedHandler := middleware(handler)

	const goroutines = 50
	const requestsPerGoroutine = 20

	runConcurrentRequests(t, wrappedHandler, goroutines, requestsPerGoroutine, func(_, j int) (string, int) {
		if j%2 == 0 {
			return concurrentKey, http.StatusOK
		}
		return wrongKey, http.StatusUnauthorized
	})
}

func TestLiveAuthMiddlewareConcurrentConfigSwitch(t *testing.T) {
	t.Parallel()

	handler := okHandler()

	cfg1 := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey: keyAlpha,
			},
		},
	}
	cfg2 := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey: keyBeta,
			},
		},
	}

	runtime := config.NewRuntime(cfg1)
	middleware := LiveAuthMiddleware(runtime)
	wrappedHandler := middleware(handler)

	const goroutines = 20
	const iterations = 50

	var wg sync.WaitGroup
	startConfigSwitcher(&wg, runtime, cfg1, cfg2, iterations)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range iterations {
				for _, key := range []string{keyAlpha, keyBeta} {
					status := doAPIKeyRequest(t, wrappedHandler, key).Code
					if status != http.StatusOK && status != http.StatusUnauthorized {
						t.Errorf("goroutine %d: unexpected status %d", id, status)
					}
				}
			}
		}(i)
	}

	wg.Wait()
}

// --- ConcurrencyLimiter Tests ---

func TestConcurrencyLimiter_TryAcquire_WithinLimit(t *testing.T) {
	t.Parallel()
	limiter := NewConcurrencyLimiter(3)

	// Should acquire 3 times successfully
	require.True(t, limiter.TryAcquire())
	require.True(t, limiter.TryAcquire())
	require.True(t, limiter.TryAcquire())
	require.Equal(t, int64(3), limiter.CurrentInFlight())

	// 4th should fail
	require.False(t, limiter.TryAcquire())
	require.Equal(t, int64(3), limiter.CurrentInFlight())
}

func TestConcurrencyLimiter_TryAcquire_Release(t *testing.T) {
	t.Parallel()
	limiter := NewConcurrencyLimiter(2)

	// Acquire 2
	require.True(t, limiter.TryAcquire())
	require.True(t, limiter.TryAcquire())
	require.False(t, limiter.TryAcquire())

	// Release 1
	limiter.Release()
	require.Equal(t, int64(1), limiter.CurrentInFlight())

	// Should be able to acquire again
	require.True(t, limiter.TryAcquire())
	require.Equal(t, int64(2), limiter.CurrentInFlight())
}

func TestConcurrencyLimiter_Unlimited(t *testing.T) {
	t.Parallel()
	limiter := NewConcurrencyLimiter(0)

	// Should always succeed with limit 0
	for i := 0; i < 100; i++ {
		require.True(t, limiter.TryAcquire())
	}
	require.Equal(t, int64(100), limiter.CurrentInFlight())
}

func TestConcurrencyLimiter_SetLimit(t *testing.T) {
	t.Parallel()
	limiter := NewConcurrencyLimiter(5)
	require.Equal(t, int64(5), limiter.GetLimit())

	limiter.SetLimit(10)
	require.Equal(t, int64(10), limiter.GetLimit())

	limiter.SetLimit(0)
	require.Equal(t, int64(0), limiter.GetLimit())
}

func TestConcurrencyLimiter_HotReload(t *testing.T) {
	t.Parallel()
	limiter := NewConcurrencyLimiter(2)

	// Fill up limit
	require.True(t, limiter.TryAcquire())
	require.True(t, limiter.TryAcquire())
	require.False(t, limiter.TryAcquire())

	// Hot-reload to increase limit
	limiter.SetLimit(3)
	require.True(t, limiter.TryAcquire())
	require.Equal(t, int64(3), limiter.CurrentInFlight())

	// Hot-reload to decrease (doesn't kick out existing, but prevents new)
	limiter.SetLimit(1)
	require.False(t, limiter.TryAcquire())

	// Release all
	limiter.Release()
	limiter.Release()
	limiter.Release()
	require.Equal(t, int64(0), limiter.CurrentInFlight())

	// Now only 1 allowed
	require.True(t, limiter.TryAcquire())
	require.False(t, limiter.TryAcquire())
}

func TestConcurrencyLimiter_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	const limit = 10
	limiter := NewConcurrencyLimiter(limit)

	var wg sync.WaitGroup
	acquired := make(chan struct{}, 100)
	rejected := make(chan struct{}, 100)

	// Spawn many goroutines trying to acquire
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.TryAcquire() {
				acquired <- struct{}{}
				time.Sleep(10 * time.Millisecond)
				limiter.Release()
			} else {
				rejected <- struct{}{}
			}
		}()
	}

	wg.Wait()
	close(acquired)
	close(rejected)

	// Should have acquired at most 'limit' at any point
	acquiredCount := len(acquired)
	rejectedCount := len(rejected)
	require.Equal(t, 50, acquiredCount+rejectedCount)
	require.LessOrEqual(t, limiter.CurrentInFlight(), int64(limit))
}

func TestConcurrencyMiddleware_EnforcesLimit(t *testing.T) {
	t.Parallel()
	limiter := NewConcurrencyLimiter(1)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := ConcurrencyMiddleware(limiter)(handler)

	// First request should succeed
	req1 := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
	resp1 := httptest.NewRecorder()

	// Second request should fail with 503
	req2 := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
	resp2 := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		wrappedHandler.ServeHTTP(resp1, req1)
	}()

	// Give first request time to acquire
	time.Sleep(10 * time.Millisecond)

	go func() {
		defer wg.Done()
		wrappedHandler.ServeHTTP(resp2, req2)
	}()

	wg.Wait()

	require.Equal(t, http.StatusOK, resp1.Code)
	require.Equal(t, http.StatusServiceUnavailable, resp2.Code)
	require.Contains(t, resp2.Body.String(), "server_busy")
}

func TestConcurrencyMiddleware_ReleasesOnCompletion(t *testing.T) {
	t.Parallel()
	limiter := NewConcurrencyLimiter(1)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := ConcurrencyMiddleware(limiter)(handler)

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
	resp1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(resp1, req1)
	require.Equal(t, http.StatusOK, resp1.Code)

	// Limiter should have released
	require.Equal(t, int64(0), limiter.CurrentInFlight())

	// Second request should also succeed
	req2 := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
	resp2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(resp2, req2)
	require.Equal(t, http.StatusOK, resp2.Code)
}

// --- MaxBodyBytesMiddleware Tests ---

func TestMaxBodyBytesMiddleware_AllowsWithinLimit(t *testing.T) {
	t.Parallel()

	var receivedBody []byte
	handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
	})

	wrappedHandler := MaxBodyBytesMiddleware(func() int64 { return 100 })(handler)

	body := bytes.NewReader([]byte(`{"model": "claude-3"}`))
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	resp := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(resp, req)

	require.Equal(t, `{"model": "claude-3"}`, string(receivedBody))
}

func TestMaxBodyBytesMiddleware_ErrorOnOversized(t *testing.T) {
	t.Parallel()

	var readErr error
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
		if readErr != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := MaxBodyBytesMiddleware(func() int64 { return 10 })(handler)

	body := bytes.NewReader([]byte(`{"model": "claude-3-opus-20240229", "messages": []}`))
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	resp := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(resp, req)

	// Handler should have gotten an error
	require.Error(t, readErr)
	require.True(t, IsBodyTooLargeError(readErr))
	require.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)
}

func TestMaxBodyBytesMiddleware_UnlimitedWhenZero(t *testing.T) {
	t.Parallel()

	var receivedBody []byte
	handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
	})

	wrappedHandler := MaxBodyBytesMiddleware(func() int64 { return 0 })(handler)

	// Large body should work when limit is 0
	largeBody := bytes.Repeat([]byte("x"), 1000)
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(largeBody))
	resp := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(resp, req)

	require.Equal(t, largeBody, receivedBody)
}

func TestMaxBodyBytesMiddleware_HotReload(t *testing.T) {
	t.Parallel()

	limit := int64(100)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := MaxBodyBytesMiddleware(func() int64 { return limit })(handler)

	// First request with small body should succeed
	req1 := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte("small")))
	resp1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(resp1, req1)
	require.Equal(t, http.StatusOK, resp1.Code)

	// Hot-reload to smaller limit
	limit = 5

	// Now larger body should fail
	req2 := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte("this is too long")))
	resp2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(resp2, req2)
	require.Equal(t, http.StatusRequestEntityTooLarge, resp2.Code)
}

func TestMaxBodyBytesMiddleware_NilBody(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := MaxBodyBytesMiddleware(func() int64 { return 10 })(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
}
