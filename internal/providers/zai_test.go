package providers_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/omarluq/cc-relay/internal/providers"
)

func TestNewZAIProvider(t *testing.T) {
	t.Parallel()

	assertNewProvider(t,
		func(name, baseURL string) providers.Provider {
			return providers.NewZAIProvider(name, baseURL)
		},
		[]providerTestCase{
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
				wantBaseURL:  providers.DefaultZAIBaseURL,
			},
		},
	)
}

func TestZAIAuthenticate(t *testing.T) {
	t.Parallel()

	provider := providers.NewZAIProvider("test-zai", "")

	testURL := "https://api.z.ai/api/anthropic/v1/messages"
	req, err := http.NewRequestWithContext(
		context.Background(), "POST", testURL, http.NoBody,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	assertAuthenticateSetsKey(t, provider, req)
}

func TestZAIForwardHeaders(t *testing.T) {
	t.Parallel()

	provider := providers.NewZAIProvider("test-zai", "")

	assertForwardHeaders(t, provider)
}

func TestZAISupportsStreaming(t *testing.T) {
	t.Parallel()

	provider := providers.NewZAIProvider("test-zai", "")

	if !provider.SupportsStreaming() {
		t.Error("Expected ZAIProvider to support streaming")
	}
}

func TestZAIForwardHeadersEdgeCases(t *testing.T) {
	t.Parallel()

	provider := providers.NewZAIProvider("test-zai", "")

	assertForwardHeadersEdgeCases(t, provider)
}

func TestZAIProviderInterface(t *testing.T) {
	t.Parallel()

	// Verify ZAIProvider implements Provider interface
	var _ providers.Provider = (*providers.ZAIProvider)(nil)
}

func TestZAIOwner(t *testing.T) {
	t.Parallel()

	provider := providers.NewZAIProvider("test-zai", "")

	if provider.Owner() != "zhipu" {
		t.Errorf("Expected owner=zhipu, got %s", provider.Owner())
	}
}

func TestZAIListModelsWithConfiguredModels(t *testing.T) {
	t.Parallel()

	models := []string{"GLM-4.7", "GLM-4.5-Air"}
	provider := providers.NewZAIProviderWithModels(
		"zai-primary", "", models,
	)

	result := provider.ListModels()

	assertListModelsWithConfiguredModels(
		t, result, "zhipu", "zai-primary",
	)

	// Verify specific model IDs
	if result[0].ID != "GLM-4.7" {
		t.Errorf("Expected model ID=GLM-4.7, got %s", result[0].ID)
	}

	if result[1].ID != "GLM-4.5-Air" {
		t.Errorf(
			"Expected model ID=GLM-4.5-Air, got %s",
			result[1].ID,
		)
	}
}

func TestZAIListModelsDefaults(t *testing.T) {
	t.Parallel()

	provider := providers.NewZAIProvider("test-zai", "")

	result := provider.ListModels()

	// Should return default models when none configured
	if len(result) != len(providers.DefaultZAIModels) {
		t.Errorf(
			"Expected %d default models, got %d",
			len(providers.DefaultZAIModels), len(result),
		)
	}
}

func TestZAIListModelsNilModels(t *testing.T) {
	t.Parallel()

	provider := providers.NewZAIProviderWithModels("test-zai", "", nil)

	result := provider.ListModels()

	// nil models should use defaults
	if len(result) != len(providers.DefaultZAIModels) {
		t.Errorf(
			"Expected %d default models when nil, got %d",
			len(providers.DefaultZAIModels), len(result),
		)
	}
}
