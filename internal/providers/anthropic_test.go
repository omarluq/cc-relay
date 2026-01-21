package providers

import (
	"net/http"
	"testing"
)

func TestNewAnthropicProvider(t *testing.T) {
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
	provider := NewAnthropicProvider("test", "")

	if !provider.SupportsStreaming() {
		t.Error("Expected AnthropicProvider to support streaming")
	}
}

func TestForwardHeaders_EdgeCases(t *testing.T) {
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
				if h.Get("accept") != "" {
					t.Error("Expected short header starting with 'a' to not be forwarded")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forwardedHeaders := provider.ForwardHeaders(tt.originalHeaders)
			tt.checkFunc(t, forwardedHeaders)
		})
	}
}
