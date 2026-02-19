package providers_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/omarluq/cc-relay/internal/providers"
)

func TestNewOllamaProvider(t *testing.T) {
	t.Parallel()

	assertNewProvider(t,
		func(name, baseURL string) providers.Provider {
			return providers.NewOllamaProvider(name, baseURL)
		},
		[]providerTestCase{
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
				wantBaseURL:  providers.DefaultOllamaBaseURL,
			},
		},
	)
}

func TestOllamaAuthenticate(t *testing.T) {
	t.Parallel()

	provider := providers.NewOllamaProvider("test-ollama", "")

	testURL := "http://localhost:11434/v1/messages"
	req, err := http.NewRequestWithContext(
		context.Background(), "POST", testURL, http.NoBody,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	assertAuthenticateSetsKey(t, provider, req)
}

func TestOllamaForwardHeaders(t *testing.T) {
	t.Parallel()

	provider := providers.NewOllamaProvider("test-ollama", "")

	assertForwardHeaders(t, provider)
}

func TestOllamaSupportsStreaming(t *testing.T) {
	t.Parallel()

	provider := providers.NewOllamaProvider("test-ollama", "")

	if !provider.SupportsStreaming() {
		t.Error("Expected OllamaProvider to support streaming")
	}
}

func TestOllamaForwardHeadersEdgeCases(t *testing.T) {
	t.Parallel()

	provider := providers.NewOllamaProvider("test-ollama", "")

	assertForwardHeadersEdgeCases(t, provider)
}

func TestOllamaProviderInterface(t *testing.T) {
	t.Parallel()

	// Verify OllamaProvider implements Provider interface
	var _ providers.Provider = (*providers.OllamaProvider)(nil)
}

func TestOllamaOwner(t *testing.T) {
	t.Parallel()

	provider := providers.NewOllamaProvider("test-ollama", "")

	if provider.Owner() != "ollama" {
		t.Errorf("Expected owner=ollama, got %s", provider.Owner())
	}
}

func TestOllamaListModelsWithConfiguredModels(t *testing.T) {
	t.Parallel()

	models := []string{"llama3.2:3b", "qwen2.5:7b"}
	provider := providers.NewOllamaProviderWithModels(
		"ollama-primary", "", models,
	)

	result := provider.ListModels()

	assertListModelsWithConfiguredModels(
		t, result, "ollama", "ollama-primary",
	)

	// Verify specific model IDs
	if result[0].ID != "llama3.2:3b" {
		t.Errorf("Expected model ID=llama3.2:3b, got %s", result[0].ID)
	}

	if result[1].ID != "qwen2.5:7b" {
		t.Errorf("Expected model ID=qwen2.5:7b, got %s", result[1].ID)
	}
}

func TestOllamaListModelsEmpty(t *testing.T) {
	t.Parallel()

	// Unlike Z.AI, Ollama has no default models (models are user-installed)
	provider := providers.NewOllamaProvider("test-ollama", "")

	result := provider.ListModels()

	// Should return empty slice when no models configured
	if len(result) != 0 {
		t.Errorf("Expected 0 models (no defaults), got %d", len(result))
	}
}

func TestOllamaListModelsNilModels(t *testing.T) {
	t.Parallel()

	provider := providers.NewOllamaProviderWithModels("test-ollama", "", nil)

	result := provider.ListModels()

	// nil models should result in empty slice (not defaults like Z.AI)
	if len(result) != 0 {
		t.Errorf("Expected 0 models when nil, got %d", len(result))
	}
}

func TestOllamaSupportsTransparentAuth(t *testing.T) {
	t.Parallel()

	provider := providers.NewOllamaProvider("test-ollama", "")

	// Ollama cannot validate Anthropic tokens
	if provider.SupportsTransparentAuth() {
		t.Error(
			"Expected SupportsTransparentAuth to return false for Ollama",
		)
	}
}

func TestOllamaGetModelMapping(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when no mapping configured", func(t *testing.T) {
		t.Parallel()
		provider := providers.NewOllamaProvider("test-ollama", "")
		if provider.GetModelMapping() != nil {
			t.Error("Expected nil model mapping when not configured")
		}
	})

	t.Run("returns mapping when configured", func(t *testing.T) {
		t.Parallel()
		mapping := map[string]string{
			"claude-opus-4-5-20251101": "qwen3:8b",
		}
		provider := providers.NewOllamaProviderWithMapping(
			"test-ollama", "", nil, mapping,
		)
		result := provider.GetModelMapping()
		if result == nil {
			t.Fatal("Expected non-nil model mapping")
		}
		if result["claude-opus-4-5-20251101"] != "qwen3:8b" {
			t.Errorf(
				"Expected mapping for claude-opus-4-5-20251101, got %v",
				result,
			)
		}
	})
}

func TestOllamaMapModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mapping  map[string]string
		input    string
		expected string
	}{
		{
			name:     "returns original when no mapping",
			mapping:  nil,
			input:    "claude-opus-4-5-20251101",
			expected: "claude-opus-4-5-20251101",
		},
		{
			name: "maps when found",
			mapping: map[string]string{
				"claude-opus-4-5-20251101": "qwen3:8b",
			},
			input:    "claude-opus-4-5-20251101",
			expected: "qwen3:8b",
		},
		{
			name: "returns original when not found",
			mapping: map[string]string{
				"claude-opus-4-5-20251101": "qwen3:8b",
			},
			input:    "some-other-model",
			expected: "some-other-model",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			provider := providers.NewOllamaProviderWithMapping(
				"test-ollama", "", nil, testCase.mapping,
			)
			result := provider.MapModel(testCase.input)
			if result != testCase.expected {
				t.Errorf("Expected %q, got %q", testCase.expected, result)
			}
		})
	}
}

func TestNewOllamaProviderWithMapping(t *testing.T) {
	t.Parallel()

	mapping := map[string]string{
		"claude-opus-4-5-20251101":  "qwen3:8b",
		"claude-sonnet-4-20250514":  "qwen3:4b",
		"claude-haiku-3-5-20241022": "qwen3:1b",
	}
	models := []string{"qwen3:8b", "qwen3:4b", "qwen3:1b"}

	provider := providers.NewOllamaProviderWithMapping(
		"ollama-primary",
		"http://192.168.1.100:11434",
		models,
		mapping,
	)

	if provider.Name() != "ollama-primary" {
		t.Errorf("Expected name=ollama-primary, got %s", provider.Name())
	}

	if provider.BaseURL() != "http://192.168.1.100:11434" {
		t.Errorf("Expected custom base URL, got %s", provider.BaseURL())
	}

	if len(provider.ListModels()) != 3 {
		t.Errorf(
			"Expected 3 models, got %d",
			len(provider.ListModels()),
		)
	}

	if len(provider.GetModelMapping()) != 3 {
		t.Errorf(
			"Expected 3 mappings, got %d",
			len(provider.GetModelMapping()),
		)
	}

	// Verify mapping works
	if provider.MapModel("claude-opus-4-5-20251101") != "qwen3:8b" {
		t.Error("Expected model mapping to work")
	}
}
