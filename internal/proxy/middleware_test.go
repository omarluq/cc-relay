package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/config"
)

func TestAuthMiddleware_ValidKey(t *testing.T) {
	t.Parallel()

	handler := okHandler()
	middleware := AuthMiddleware("secret-key")
	wrappedHandler := middleware(handler)

	req := newMessagesRequest(http.NoBody)
	req.Header.Set("x-api-key", "secret-key")

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_InvalidKey(t *testing.T) {
	t.Parallel()

	handler := okHandler()
	middleware := AuthMiddleware("secret-key")
	wrappedHandler := middleware(handler)

	req := newMessagesRequest(http.NoBody)
	req.Header.Set("x-api-key", "wrong-key")

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MissingKey(t *testing.T) {
	t.Parallel()

	handler := okHandler()
	middleware := AuthMiddleware("secret-key")
	wrappedHandler := middleware(handler)

	req := newMessagesRequest(http.NoBody)
	// No x-api-key header

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}

	// Verify error message
	if !strings.Contains(rec.Body.String(), "missing x-api-key header") {
		t.Errorf("Expected error about missing header, got: %s", rec.Body.String())
	}
}

func TestMultiAuthMiddleware_NoAuthConfigured(t *testing.T) {
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
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 when no auth configured, got %d", rec.Code)
	}
}

func TestMultiAuthMiddleware_BearerOnly(t *testing.T) {
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

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid bearer, got %d", rec.Code)
	}
}

func TestMultiAuthMiddleware_APIKeyOnly(t *testing.T) {
	t.Parallel()

	handler := okHandler()
	authConfig := &config.AuthConfig{
		APIKey: "test-api-key",
	}
	middleware := MultiAuthMiddleware(authConfig)
	wrappedHandler := middleware(handler)

	// Valid API key
	req := newMessagesRequest(http.NoBody)
	req.Header.Set("x-api-key", "test-api-key")

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid API key, got %d", rec.Code)
	}
}

func TestMultiAuthMiddleware_BothMethods(t *testing.T) {
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

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 with bearer, got %d", rec.Code)
		}
	})

	// Test with API key - should work
	t.Run("api key works", func(t *testing.T) {
		t.Parallel()
		req := newMessagesRequest(http.NoBody)
		req.Header.Set("x-api-key", "test-api-key")

		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 with API key, got %d", rec.Code)
		}
	})
}

func TestMultiAuthMiddleware_SubscriptionAlias(t *testing.T) {
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

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with subscription token, got %d", rec.Code)
	}
}

func TestLiveAuthMiddleware_ToggleAPIKey(t *testing.T) {
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

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
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

func TestRequestIDMiddleware_UsesProvidedID(t *testing.T) {
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

func TestLoggingMiddleware_LogsRequest(t *testing.T) {
	t.Parallel()

	handler := okHandler()

	debugOpts := config.DebugOptions{}
	middleware := LoggingMiddleware(debugOpts)

	// Wrap with RequestIDMiddleware first (as in production)
	wrappedHandler := RequestIDMiddleware()(middleware(handler))

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"test"}`))
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct { //nolint:govet // test table struct alignment
		name     string
		ms       int
		expected string
	}{
		{"fast request", 100, "100ms"},
		{"medium request", 500, "500ms"},
		{"slow request", 1500, "1.50s"},
		{"very slow request", 5000, "5.00s"},
		{"zero", 0, "0ms"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Convert ms to duration
			d := tt.ms * 1000000 // nanoseconds
			got := formatDuration(time.Duration(d))
			if got != tt.expected {
				t.Errorf("formatDuration(%d ms) = %s, want %s", tt.ms, got, tt.expected)
			}
		})
	}
}

func TestFormatCompletionMessage(t *testing.T) {
	t.Parallel()

	tests := []struct { //nolint:govet // test table struct alignment
		status   int
		symbol   string
		duration string
		expected string
	}{
		{200, "!", "100ms", "! OK (100ms)"},
		{404, "?", "50ms", "? Not Found (50ms)"},
		{500, "x", "1.5s", "x Internal Server Error (1.5s)"},
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

func TestResponseWriter_CapturesStatusCode(t *testing.T) {
	t.Parallel()

	rw := newTestResponseWriter()

	rw.WriteHeader(http.StatusNotFound)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rw.statusCode)
	}
}

func TestResponseWriter_DetectsStreaming(t *testing.T) {
	t.Parallel()

	rw := newTestResponseWriter()

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.WriteHeader(http.StatusOK)

	if !rw.isStreaming {
		t.Error("Expected isStreaming to be true for text/event-stream")
	}
}

func TestResponseWriter_CountsSSEEvents(t *testing.T) {
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
			if fp == "" {
				t.Error("fingerprint should not be empty")
			}

			// Same inputs produce same fingerprint
			fp2 := authFingerprint(tt.bearerEnabled, tt.bearerSecret, tt.apiKey)
			if fp != fp2 {
				t.Errorf("fingerprint not deterministic: %q != %q", fp, fp2)
			}
		})
	}

	// Different inputs produce different fingerprints
	t.Run("different inputs differ", func(t *testing.T) {
		t.Parallel()
		fp1 := authFingerprint(true, "secret1", "key1")
		fp2 := authFingerprint(true, "secret2", "key1")
		fp3 := authFingerprint(false, "secret1", "key1")
		fp4 := authFingerprint(true, "secret1", "key2")

		if fp1 == fp2 {
			t.Error("different bearer secrets should produce different fingerprints")
		}
		if fp1 == fp3 {
			t.Error("different bearer enabled should produce different fingerprints")
		}
		if fp1 == fp4 {
			t.Error("different api keys should produce different fingerprints")
		}
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

func TestLiveAuthMiddleware_NilProvider(t *testing.T) {
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
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestLiveAuthMiddleware_NilConfig(t *testing.T) {
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
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestLiveAuthMiddleware_NoAuthConfigured(t *testing.T) {
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
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

//nolint:tparallel // subtests share wrappedHandler to test cache behavior
func TestLiveAuthMiddleware_APIKeyAuth(t *testing.T) {
	t.Parallel()

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
	wrappedHandler := middleware(handler)

	t.Run("valid key", func(t *testing.T) {
		req := newMessagesRequest(http.NoBody)
		req.Header.Set("x-api-key", "test-api-key")
		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("invalid key", func(t *testing.T) {
		req := newMessagesRequest(http.NoBody)
		req.Header.Set("x-api-key", "wrong-key")
		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		req := newMessagesRequest(http.NoBody)
		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})
}

func TestLiveAuthMiddleware_ConfigSwitching(t *testing.T) {
	t.Parallel()

	handler := okHandler()

	// Start with API key auth
	cfg1 := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey: "key-v1",
			},
		},
	}
	runtime := config.NewRuntime(cfg1)
	middleware := LiveAuthMiddleware(runtime)
	wrappedHandler := middleware(handler)

	// Request with key-v1 should succeed
	req1 := newMessagesRequest(http.NoBody)
	req1.Header.Set("x-api-key", "key-v1")
	rec1 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("key-v1 should work with cfg1: got %d", rec1.Code)
	}

	// Switch to new API key
	cfg2 := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey: "key-v2",
			},
		},
	}
	runtime.Store(cfg2)

	// Old key should now fail
	req2 := newMessagesRequest(http.NoBody)
	req2.Header.Set("x-api-key", "key-v1")
	rec2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("key-v1 should fail after switch: got %d", rec2.Code)
	}

	// New key should work
	req3 := newMessagesRequest(http.NoBody)
	req3.Header.Set("x-api-key", "key-v2")
	rec3 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Errorf("key-v2 should work after switch: got %d", rec3.Code)
	}
}

func TestLiveAuthMiddleware_SwitchAuthMethods(t *testing.T) {
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
	req3.Header.Set("x-api-key", "my-api-key")
	rec3 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusUnauthorized {
		t.Errorf("api key should fail after switch to bearer only: got %d", rec3.Code)
	}
}

func TestLiveAuthMiddleware_SwitchToNoAuth(t *testing.T) {
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

func TestLiveAuthMiddleware_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	handler := okHandler()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey: "concurrent-key",
			},
		},
	}
	runtime := config.NewRuntime(cfg)
	middleware := LiveAuthMiddleware(runtime)
	wrappedHandler := middleware(handler)

	const goroutines = 50
	const requestsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range requestsPerGoroutine {
				// Mix of valid and invalid requests
				req := newMessagesRequest(http.NoBody)
				if j%2 == 0 {
					req.Header.Set("x-api-key", "concurrent-key")
				} else {
					req.Header.Set("x-api-key", "wrong-key")
				}
				rec := httptest.NewRecorder()
				wrappedHandler.ServeHTTP(rec, req)

				expected := http.StatusOK
				if j%2 != 0 {
					expected = http.StatusUnauthorized
				}
				if rec.Code != expected {
					t.Errorf("goroutine %d request %d: expected %d, got %d", id, j, expected, rec.Code)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestLiveAuthMiddleware_ConcurrentConfigSwitch(t *testing.T) {
	t.Parallel()

	handler := okHandler()

	cfg1 := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey: "key-alpha",
			},
		},
	}
	cfg2 := &config.Config{
		Server: config.ServerConfig{
			Auth: config.AuthConfig{
				APIKey: "key-beta",
			},
		},
	}

	runtime := config.NewRuntime(cfg1)
	middleware := LiveAuthMiddleware(runtime)
	wrappedHandler := middleware(handler)

	const goroutines = 20
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines + 1) // +1 for config switcher

	// Goroutine that switches config rapidly
	go func() {
		defer wg.Done()
		for i := range iterations {
			if i%2 == 0 {
				runtime.Store(cfg1)
			} else {
				runtime.Store(cfg2)
			}
			time.Sleep(time.Microsecond)
		}
	}()

	// Request goroutines
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for range iterations {
				// Try both keys - one should work depending on current config
				for _, key := range []string{"key-alpha", "key-beta"} {
					req := newMessagesRequest(http.NoBody)
					req.Header.Set("x-api-key", key)
					rec := httptest.NewRecorder()
					wrappedHandler.ServeHTTP(rec, req)

					// Should get either 200 (correct key) or 401 (wrong key)
					// Never panic, never deadlock
					if rec.Code != http.StatusOK && rec.Code != http.StatusUnauthorized {
						t.Errorf("goroutine %d: unexpected status %d", id, rec.Code)
					}
				}
			}
		}(i)
	}

	wg.Wait()
}
