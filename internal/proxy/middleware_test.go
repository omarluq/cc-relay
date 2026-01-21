package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/config"
)

func TestAuthMiddleware_ValidKey(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := AuthMiddleware("secret-key")
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
	req.Header.Set("x-api-key", "secret-key")

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_InvalidKey(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := AuthMiddleware("secret-key")
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
	req.Header.Set("x-api-key", "wrong-key")

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MissingKey(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := AuthMiddleware("secret-key")
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
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

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authConfig := &config.AuthConfig{}
	middleware := MultiAuthMiddleware(authConfig)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
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

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authConfig := &config.AuthConfig{
		AllowBearer:  true,
		BearerSecret: "test-bearer-secret",
	}
	middleware := MultiAuthMiddleware(authConfig)
	wrappedHandler := middleware(handler)

	// Valid bearer token
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
	req.Header.Set("Authorization", "Bearer test-bearer-secret")

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid bearer, got %d", rec.Code)
	}
}

func TestMultiAuthMiddleware_APIKeyOnly(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authConfig := &config.AuthConfig{
		APIKey: "test-api-key",
	}
	middleware := MultiAuthMiddleware(authConfig)
	wrappedHandler := middleware(handler)

	// Valid API key
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
	req.Header.Set("x-api-key", "test-api-key")

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid API key, got %d", rec.Code)
	}
}

func TestMultiAuthMiddleware_BothMethods(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

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
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
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
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
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

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authConfig := &config.AuthConfig{
		AllowSubscription: true, // Alias for AllowBearer
	}
	middleware := MultiAuthMiddleware(authConfig)
	wrappedHandler := middleware(handler)

	// Any bearer token should work (passthrough mode)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
	req.Header.Set("Authorization", "Bearer any-subscription-token")

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with subscription token, got %d", rec.Code)
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

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

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

	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	rw.WriteHeader(http.StatusNotFound)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rw.statusCode)
	}
}

func TestResponseWriter_DetectsStreaming(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.WriteHeader(http.StatusOK)

	if !rw.isStreaming {
		t.Error("Expected isStreaming to be true for text/event-stream")
	}
}

func TestResponseWriter_CountsSSEEvents(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

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
