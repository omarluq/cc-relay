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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
)

func TestNewHandler_ValidProvider(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := NewHandler(provider, "test-key", nil, config.DebugOptions{})
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

	_, err := NewHandler(provider, "test-key", nil, config.DebugOptions{})
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

	handler, err := NewHandler(provider, "test-key", nil, config.DebugOptions{})
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

	handler, err := NewHandler(provider, "test-key", nil, config.DebugOptions{})
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	// Verify ErrorHandler is configured
	if handler.proxy.ErrorHandler == nil {
		t.Error("ErrorHandler should be configured")
	}
}

func TestHandler_StructureCorrect(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := NewHandler(provider, "test-key", nil, config.DebugOptions{})
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	// Verify handler has non-nil proxy
	if handler.proxy == nil {
		t.Error("handler.proxy is nil")
	}

	// Verify FlushInterval is set to -1
	if handler.proxy.FlushInterval != -1 {
		t.Errorf("FlushInterval = %v, want -1", handler.proxy.FlushInterval)
	}

	// Verify provider is set
	if handler.provider == nil {
		t.Error("handler.provider is nil")
	}

	// Verify apiKey is set
	if handler.apiKey != "test-key" {
		t.Errorf("handler.apiKey = %q, want %q", handler.apiKey, "test-key")
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

	handler, err := NewHandler(provider, "test-key", nil, config.DebugOptions{})
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

func (m *mockProvider) Owner() string {
	return "mock"
}

func (m *mockProvider) ListModels() []providers.Model {
	return nil
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
	handler, err := NewHandler(provider, "", pool, config.DebugOptions{})
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
	handler, err := NewHandler(provider, "", pool, config.DebugOptions{})
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
	handler, err := NewHandler(provider, "", pool, config.DebugOptions{})
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
	handler, err := NewHandler(provider, "", pool, config.DebugOptions{})
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
	handler, err := NewHandler(provider, "test-single-key", nil, config.DebugOptions{})
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
