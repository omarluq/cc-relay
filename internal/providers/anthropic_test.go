package providers_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/omarluq/cc-relay/internal/providers"
)

func TestNewAnthropicProvider(t *testing.T) {
	t.Parallel()

	assertNewProvider(t,
		func(name, baseURL string) providers.Provider {
			return providers.NewAnthropicProvider(name, baseURL)
		},
		[]providerTestCase{
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
				wantBaseURL:  providers.DefaultAnthropicBaseURL,
			},
		},
	)
}

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "")

	testURL := "https://api.example.com/v1/messages"
	req, err := http.NewRequestWithContext(
		context.Background(), "POST", testURL, http.NoBody,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	assertAuthenticateSetsKey(t, provider, req)
}

func TestForwardHeaders(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "")

	assertForwardHeaders(t, provider)
}

func TestSupportsStreaming(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "")

	if !provider.SupportsStreaming() {
		t.Error("Expected AnthropicProvider to support streaming")
	}
}

func TestForwardHeadersEdgeCases(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "")

	assertForwardHeadersEdgeCases(t, provider)
}

func TestListModelsWithConfiguredModels(t *testing.T) {
	t.Parallel()

	models := []string{
		"claude-sonnet-4-5-20250514",
		"claude-opus-4-5-20250514",
	}
	provider := providers.NewAnthropicProviderWithModels(
		"anthropic-primary", "", models,
	)

	result := provider.ListModels()

	assertListModelsWithConfiguredModels(
		t, result, "anthropic", "anthropic-primary",
	)

	// Verify specific model IDs
	if result[0].ID != "claude-sonnet-4-5-20250514" {
		t.Errorf(
			"Expected model ID=claude-sonnet-4-5-20250514, got %s",
			result[0].ID,
		)
	}

	if result[1].ID != "claude-opus-4-5-20250514" {
		t.Errorf(
			"Expected model ID=claude-opus-4-5-20250514, got %s",
			result[1].ID,
		)
	}
}

func TestListModelsDefaults(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "")

	result := provider.ListModels()

	// Should return default models when none configured
	if len(result) != len(providers.DefaultAnthropicModels) {
		t.Errorf(
			"Expected %d default models, got %d",
			len(providers.DefaultAnthropicModels), len(result),
		)
	}
}

func TestListModelsNilModels(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProviderWithModels("test", "", nil)

	result := provider.ListModels()

	// nil models should use defaults
	if len(result) != len(providers.DefaultAnthropicModels) {
		t.Errorf(
			"Expected %d default models when nil, got %d",
			len(providers.DefaultAnthropicModels), len(result),
		)
	}
}

func TestProviderOwner(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("test", "")

	if provider.Owner() != "anthropic" {
		t.Errorf("Expected owner=anthropic, got %s", provider.Owner())
	}
}
