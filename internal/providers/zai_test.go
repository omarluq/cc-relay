package providers

import (
	"net/http"
	"testing"
)

func TestNewZAIProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerName string
		baseURL      string
		wantBaseURL  string
	}{
		{
			name:         "with custom base URL",
			providerName: "zai-custom",
			baseURL:      "https://custom.zhipuai.cn/api/anthropic",
			wantBaseURL:  "https://custom.zhipuai.cn/api/anthropic",
		},
		{
			name:         "with empty base URL uses default",
			providerName: "zai-default",
			baseURL:      "",
			wantBaseURL:  DefaultZAIBaseURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := NewZAIProvider(tt.providerName, tt.baseURL)

			if provider.Name() != tt.providerName {
				t.Errorf("Expected name=%s, got %s", tt.providerName, provider.Name())
			}

			if provider.BaseURL() != tt.wantBaseURL {
				t.Errorf("Expected baseURL=%s, got %s", tt.wantBaseURL, provider.BaseURL())
			}
		})
	}
}

func TestZAIAuthenticate(t *testing.T) {
	t.Parallel()

	provider := NewZAIProvider("test-zai", "")

	req, err := http.NewRequest("POST", "https://api.z.ai/api/anthropic/v1/messages", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	apiKey := "zai-test-key-123"

	err = provider.Authenticate(req, apiKey)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	// Verify x-api-key header is set correctly (Z.AI uses same auth as Anthropic)
	gotKey := req.Header.Get("x-api-key")
	if gotKey != apiKey {
		t.Errorf("Expected x-api-key=%s, got %s", apiKey, gotKey)
	}
}

func TestZAIForwardHeaders(t *testing.T) {
	t.Parallel()

	provider := NewZAIProvider("test-zai", "")

	// Create original headers with mix of anthropic-* and other headers
	originalHeaders := http.Header{
		"anthropic-version":                         []string{"2023-06-01"},
		"anthropic-dangerous-direct-browser-access": []string{"true"},
		"Authorization":                             []string{"Bearer token"},
		"User-Agent":                                []string{"test-agent"},
		"X-Custom-Header":                           []string{"custom-value"},
	}

	forwardedHeaders := provider.ForwardHeaders(originalHeaders)

	// Verify anthropic-* headers are forwarded (Z.AI is Anthropic-compatible)
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

func TestZAISupportsStreaming(t *testing.T) {
	t.Parallel()

	provider := NewZAIProvider("test-zai", "")

	if !provider.SupportsStreaming() {
		t.Error("Expected ZAIProvider to support streaming")
	}
}

func TestZAIForwardHeaders_EdgeCases(t *testing.T) {
	t.Parallel()

	provider := NewZAIProvider("test-zai", "")

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

func TestZAIProviderInterface(t *testing.T) {
	t.Parallel()

	// Verify ZAIProvider implements Provider interface
	var _ Provider = (*ZAIProvider)(nil)
}

func TestZAIOwner(t *testing.T) {
	t.Parallel()

	provider := NewZAIProvider("test-zai", "")

	if provider.Owner() != "zhipu" {
		t.Errorf("Expected owner=zhipu, got %s", provider.Owner())
	}
}

func TestZAIListModels_WithConfiguredModels(t *testing.T) {
	t.Parallel()

	models := []string{"GLM-4.7", "GLM-4.5-Air"}
	provider := NewZAIProviderWithModels("zai-primary", "", models)

	result := provider.ListModels()

	if len(result) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(result))
	}

	// First model
	if result[0].ID != "GLM-4.7" {
		t.Errorf("Expected model ID=GLM-4.7, got %s", result[0].ID)
	}
	if result[0].Object != "model" {
		t.Errorf("Expected object=model, got %s", result[0].Object)
	}
	if result[0].OwnedBy != "zhipu" {
		t.Errorf("Expected owned_by=zhipu, got %s", result[0].OwnedBy)
	}
	if result[0].Provider != "zai-primary" {
		t.Errorf("Expected provider=zai-primary, got %s", result[0].Provider)
	}
	if result[0].Created == 0 {
		t.Error("Expected created timestamp to be set")
	}

	// Second model
	if result[1].ID != "GLM-4.5-Air" {
		t.Errorf("Expected model ID=GLM-4.5-Air, got %s", result[1].ID)
	}
}

func TestZAIListModels_Defaults(t *testing.T) {
	t.Parallel()

	provider := NewZAIProvider("test-zai", "")

	result := provider.ListModels()

	// Should return default models when none configured
	if len(result) != len(DefaultZAIModels) {
		t.Errorf("Expected %d default models, got %d", len(DefaultZAIModels), len(result))
	}
}

func TestZAIListModels_NilModels(t *testing.T) {
	t.Parallel()

	provider := NewZAIProviderWithModels("test-zai", "", nil)

	result := provider.ListModels()

	// nil models should use defaults
	if len(result) != len(DefaultZAIModels) {
		t.Errorf("Expected %d default models when nil, got %d", len(DefaultZAIModels), len(result))
	}
}
