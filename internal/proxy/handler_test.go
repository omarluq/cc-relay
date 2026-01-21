package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
)

func TestNewHandler_ValidProvider(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	handler, err := NewHandler(provider, "test-key", config.DebugOptions{})
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

	_, err := NewHandler(provider, "test-key", config.DebugOptions{})
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

	handler, err := NewHandler(provider, "test-key", config.DebugOptions{})
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

	handler, err := NewHandler(provider, "test-key", config.DebugOptions{})
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

	handler, err := NewHandler(provider, "test-key", config.DebugOptions{})
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

	handler, err := NewHandler(provider, "test-key", config.DebugOptions{})
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
