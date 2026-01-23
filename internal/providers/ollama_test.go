package providers

import (
	"net/http"
	"testing"
)

func TestNewOllamaProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerName string
		baseURL      string
		wantBaseURL  string
	}{
		{
			name:         "with custom base URL",
			providerName: "ollama-custom",
			baseURL:      "http://192.168.1.100:11434",
			wantBaseURL:  "http://192.168.1.100:11434",
		},
		{
			name:         "with empty base URL uses default",
			providerName: "ollama-default",
			baseURL:      "",
			wantBaseURL:  DefaultOllamaBaseURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := NewOllamaProvider(tt.providerName, tt.baseURL)

			if provider.Name() != tt.providerName {
				t.Errorf("Expected name=%s, got %s", tt.providerName, provider.Name())
			}

			if provider.BaseURL() != tt.wantBaseURL {
				t.Errorf("Expected baseURL=%s, got %s", tt.wantBaseURL, provider.BaseURL())
			}
		})
	}
}

func TestOllamaAuthenticate(t *testing.T) {
	t.Parallel()

	provider := NewOllamaProvider("test-ollama", "")

	req, err := http.NewRequest("POST", "http://localhost:11434/v1/messages", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	apiKey := "ollama-test-key-123"

	err = provider.Authenticate(req, apiKey)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	// Verify x-api-key header is set correctly (Ollama accepts but ignores auth)
	gotKey := req.Header.Get("x-api-key")
	if gotKey != apiKey {
		t.Errorf("Expected x-api-key=%s, got %s", apiKey, gotKey)
	}
}

func TestOllamaForwardHeaders(t *testing.T) {
	t.Parallel()

	provider := NewOllamaProvider("test-ollama", "")

	// Create original headers with mix of anthropic-* and other headers
	originalHeaders := http.Header{
		"anthropic-version":                         []string{"2023-06-01"},
		"anthropic-dangerous-direct-browser-access": []string{"true"},
		"Authorization":                             []string{"Bearer token"},
		"User-Agent":                                []string{"test-agent"},
		"X-Custom-Header":                           []string{"custom-value"},
	}

	forwardedHeaders := provider.ForwardHeaders(originalHeaders)

	// Verify anthropic-* headers are forwarded (Ollama is Anthropic-compatible)
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

func TestOllamaSupportsStreaming(t *testing.T) {
	t.Parallel()

	provider := NewOllamaProvider("test-ollama", "")

	if !provider.SupportsStreaming() {
		t.Error("Expected OllamaProvider to support streaming")
	}
}

func TestOllamaForwardHeaders_EdgeCases(t *testing.T) {
	t.Parallel()

	provider := NewOllamaProvider("test-ollama", "")

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

func TestOllamaProviderInterface(t *testing.T) {
	t.Parallel()

	// Verify OllamaProvider implements Provider interface
	var _ Provider = (*OllamaProvider)(nil)
}

func TestOllamaOwner(t *testing.T) {
	t.Parallel()

	provider := NewOllamaProvider("test-ollama", "")

	if provider.Owner() != "ollama" {
		t.Errorf("Expected owner=ollama, got %s", provider.Owner())
	}
}

func TestOllamaListModels_WithConfiguredModels(t *testing.T) {
	t.Parallel()

	models := []string{"llama3.2:3b", "qwen2.5:7b"}
	provider := NewOllamaProviderWithModels("ollama-primary", "", models)

	result := provider.ListModels()

	if len(result) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(result))
	}

	// First model
	if result[0].ID != "llama3.2:3b" {
		t.Errorf("Expected model ID=llama3.2:3b, got %s", result[0].ID)
	}
	if result[0].Object != "model" {
		t.Errorf("Expected object=model, got %s", result[0].Object)
	}
	if result[0].OwnedBy != "ollama" {
		t.Errorf("Expected owned_by=ollama, got %s", result[0].OwnedBy)
	}
	if result[0].Provider != "ollama-primary" {
		t.Errorf("Expected provider=ollama-primary, got %s", result[0].Provider)
	}
	if result[0].Created == 0 {
		t.Error("Expected created timestamp to be set")
	}

	// Second model
	if result[1].ID != "qwen2.5:7b" {
		t.Errorf("Expected model ID=qwen2.5:7b, got %s", result[1].ID)
	}
}

func TestOllamaListModels_Empty(t *testing.T) {
	t.Parallel()

	// Unlike Z.AI, Ollama has no default models (models are user-installed)
	provider := NewOllamaProvider("test-ollama", "")

	result := provider.ListModels()

	// Should return empty slice when no models configured
	if len(result) != 0 {
		t.Errorf("Expected 0 models (no defaults), got %d", len(result))
	}
}

func TestOllamaListModels_NilModels(t *testing.T) {
	t.Parallel()

	provider := NewOllamaProviderWithModels("test-ollama", "", nil)

	result := provider.ListModels()

	// nil models should result in empty slice (not defaults like Z.AI)
	if len(result) != 0 {
		t.Errorf("Expected 0 models when nil, got %d", len(result))
	}
}

func TestOllamaSupportsTransparentAuth(t *testing.T) {
	t.Parallel()

	provider := NewOllamaProvider("test-ollama", "")

	// Ollama cannot validate Anthropic tokens
	if provider.SupportsTransparentAuth() {
		t.Error("Expected SupportsTransparentAuth to return false for Ollama")
	}
}
