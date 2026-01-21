package providers

import (
	"net/http"
	"testing"
)

func TestNewAnthropicProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerName string
		baseURL      string
		wantBaseURL  string
	}{
		{
			name:         "with custom base URL",
			providerName: "test-provider",
			baseURL:      "https://custom.api.example.com",
			wantBaseURL:  "https://custom.api.example.com",
		},
		{
			name:         "with empty base URL uses default",
			providerName: "default-provider",
			baseURL:      "",
			wantBaseURL:  DefaultAnthropicBaseURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := NewAnthropicProvider(tt.providerName, tt.baseURL)

			if provider.Name() != tt.providerName {
				t.Errorf("Expected name=%s, got %s", tt.providerName, provider.Name())
			}

			if provider.BaseURL() != tt.wantBaseURL {
				t.Errorf("Expected baseURL=%s, got %s", tt.wantBaseURL, provider.BaseURL())
			}
		})
	}
}

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	provider := NewAnthropicProvider("test", "")

	req, err := http.NewRequest("POST", "https://api.example.com/v1/messages", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	apiKey := "sk-ant-test-key-123"

	err = provider.Authenticate(req, apiKey)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	// Verify x-api-key header is set correctly
	gotKey := req.Header.Get("x-api-key")
	if gotKey != apiKey {
		t.Errorf("Expected x-api-key=%s, got %s", apiKey, gotKey)
	}
}

func TestForwardHeaders(t *testing.T) {
	t.Parallel()

	provider := NewAnthropicProvider("test", "")

	// Create original headers with mix of anthropic-* and other headers
	originalHeaders := http.Header{
		"anthropic-version":                         []string{"2023-06-01"},
		"anthropic-dangerous-direct-browser-access": []string{"true"},
		"Authorization":                             []string{"Bearer token"},
		"User-Agent":                                []string{"test-agent"},
		"X-Custom-Header":                           []string{"custom-value"},
	}

	forwardedHeaders := provider.ForwardHeaders(originalHeaders)

	// Verify anthropic-* headers are forwarded
	if forwardedHeaders.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("Expected anthropic-version header to be forwarded")
	}

	if forwardedHeaders.Get("anthropic-dangerous-direct-browser-access") != "true" {
		t.Errorf("Expected anthropic-dangerous-direct-browser-access header to be forwarded")
	}

	// Verify Content-Type is set
	if forwardedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type=application/json, got %s", forwardedHeaders.Get("Content-Type"))
	}

	// Verify non-anthropic headers are NOT forwarded
	if forwardedHeaders.Get("Authorization") != "" {
		t.Error("Expected Authorization header to not be forwarded")
	}

	if forwardedHeaders.Get("User-Agent") != "" {
		t.Error("Expected User-Agent header to not be forwarded")
	}

	if forwardedHeaders.Get("X-Custom-Header") != "" {
		t.Error("Expected X-Custom-Header to not be forwarded")
	}
}

func TestSupportsStreaming(t *testing.T) {
	t.Parallel()

	provider := NewAnthropicProvider("test", "")

	if !provider.SupportsStreaming() {
		t.Error("Expected AnthropicProvider to support streaming")
	}
}

func TestForwardHeaders_EdgeCases(t *testing.T) {
	t.Parallel()

	provider := NewAnthropicProvider("test", "")

	tests := []struct {
		originalHeaders http.Header
		checkFunc       func(*testing.T, http.Header)
		name            string
	}{
		{
			name:            "empty headers",
			originalHeaders: http.Header{},
			checkFunc: func(t *testing.T, h http.Header) {
				t.Helper()
				if h.Get("Content-Type") != "application/json" {
					t.Error("Expected Content-Type to be set even with empty original headers")
				}
			},
		},
		{
			name: "multiple anthropic headers",
			originalHeaders: http.Header{
				"anthropic-version": []string{"2023-06-01"},
				"anthropic-beta":    []string{"feature-1", "feature-2"},
			},
			checkFunc: func(t *testing.T, h http.Header) {
				t.Helper()
				if h.Get("anthropic-version") != "2023-06-01" {
					t.Error("Expected anthropic-version to be forwarded")
				}
				beta := h["Anthropic-Beta"]
				if len(beta) != 2 || beta[0] != "feature-1" || beta[1] != "feature-2" {
					t.Errorf("Expected anthropic-beta to have both values, got %v", beta)
				}
			},
		},
		{
			name: "short header name starting with 'a'",
			originalHeaders: http.Header{
				"accept": []string{"application/json"},
			},
			checkFunc: func(t *testing.T, h http.Header) {
				t.Helper()
				if h.Get("accept") != "" {
					t.Error("Expected short header starting with 'a' to not be forwarded")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			forwardedHeaders := provider.ForwardHeaders(tt.originalHeaders)
			tt.checkFunc(t, forwardedHeaders)
		})
	}
}

func TestListModels_WithConfiguredModels(t *testing.T) {
	t.Parallel()

	models := []string{"claude-sonnet-4-5-20250514", "claude-opus-4-5-20250514"}
	provider := NewAnthropicProviderWithModels("anthropic-primary", "", models)

	result := provider.ListModels()

	if len(result) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(result))
	}

	// First model
	if result[0].ID != "claude-sonnet-4-5-20250514" {
		t.Errorf("Expected model ID=claude-sonnet-4-5-20250514, got %s", result[0].ID)
	}
	if result[0].Object != "model" {
		t.Errorf("Expected object=model, got %s", result[0].Object)
	}
	if result[0].OwnedBy != "anthropic" {
		t.Errorf("Expected owned_by=anthropic, got %s", result[0].OwnedBy)
	}
	if result[0].Provider != "anthropic-primary" {
		t.Errorf("Expected provider=anthropic-primary, got %s", result[0].Provider)
	}
	if result[0].Created == 0 {
		t.Error("Expected created timestamp to be set")
	}

	// Second model
	if result[1].ID != "claude-opus-4-5-20250514" {
		t.Errorf("Expected model ID=claude-opus-4-5-20250514, got %s", result[1].ID)
	}
}

func TestListModels_Defaults(t *testing.T) {
	t.Parallel()

	provider := NewAnthropicProvider("test", "")

	result := provider.ListModels()

	// Should return default models when none configured
	if len(result) != len(DefaultAnthropicModels) {
		t.Errorf("Expected %d default models, got %d", len(DefaultAnthropicModels), len(result))
	}
}

func TestListModels_NilModels(t *testing.T) {
	t.Parallel()

	provider := NewAnthropicProviderWithModels("test", "", nil)

	result := provider.ListModels()

	// nil models should use defaults
	if len(result) != len(DefaultAnthropicModels) {
		t.Errorf("Expected %d default models when nil, got %d", len(DefaultAnthropicModels), len(result))
	}
}

func TestProviderOwner(t *testing.T) {
	t.Parallel()

	provider := NewAnthropicProvider("test", "")

	if provider.Owner() != "anthropic" {
		t.Errorf("Expected owner=anthropic, got %s", provider.Owner())
	}
}
