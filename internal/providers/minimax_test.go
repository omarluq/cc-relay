package providers_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/omarluq/cc-relay/internal/providers"
)

func TestNewMiniMaxProvider(t *testing.T) {
	t.Parallel()

	assertNewProvider(t,
		func(name, baseURL string) providers.Provider {
			return providers.NewMiniMaxProvider(name, baseURL)
		},
		[]providerTestCase{
			{
				name:         "with custom base URL",
				providerName: "minimax-custom",
				baseURL:      "https://custom.minimax.io/anthropic",
				wantBaseURL:  "https://custom.minimax.io/anthropic",
			},
			{
				name:         "with empty base URL uses default",
				providerName: "minimax-default",
				baseURL:      "",
				wantBaseURL:  providers.DefaultMiniMaxBaseURL,
			},
		},
	)
}

func TestMiniMaxAuthenticate(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProvider("test-minimax", "")

	testURL := "https://api.minimax.io/anthropic/v1/messages"
	req, err := http.NewRequestWithContext(
		context.Background(), "POST", testURL, http.NoBody,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	testAuthKey := "test-auth-key-for-testing-only"

	err = provider.Authenticate(req, testAuthKey)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	// MiniMax uses Bearer token authentication, NOT x-api-key
	gotAuth := req.Header.Get("Authorization")
	wantAuth := "Bearer " + testAuthKey

	if gotAuth != wantAuth {
		t.Errorf("Expected Authorization=%s, got %s", wantAuth, gotAuth)
	}

	// Verify x-api-key is NOT set
	if req.Header.Get("x-api-key") != "" {
		t.Error("Expected x-api-key header to not be set for MiniMax")
	}
}

func TestMiniMaxForwardHeaders(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProvider("test-minimax", "")

	assertForwardHeaders(t, provider)
}

func TestMiniMaxSupportsStreaming(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProvider("test-minimax", "")

	if !provider.SupportsStreaming() {
		t.Error("Expected MiniMaxProvider to support streaming")
	}
}

func TestMiniMaxForwardHeadersEdgeCases(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProvider("test-minimax", "")

	assertForwardHeadersEdgeCases(t, provider)
}

func TestMiniMaxProviderInterface(t *testing.T) {
	t.Parallel()

	// Verify MiniMaxProvider implements Provider interface
	var _ providers.Provider = (*providers.MiniMaxProvider)(nil)
}

func TestMiniMaxOwner(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProvider("test-minimax", "")

	if provider.Owner() != "minimax" {
		t.Errorf("Expected owner=minimax, got %s", provider.Owner())
	}
}

func TestMiniMaxSupportsTransparentAuth(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProvider("test-minimax", "")

	if provider.SupportsTransparentAuth() {
		t.Error("Expected MiniMaxProvider to NOT support transparent auth")
	}
}

func TestMiniMaxListModelsWithConfiguredModels(t *testing.T) {
	t.Parallel()

	models := []string{"MiniMax-M2.5", "MiniMax-M2.1"}
	provider := providers.NewMiniMaxProviderWithModels(
		"minimax-primary", "", models,
	)

	result := provider.ListModels()

	assertListModelsWithConfiguredModels(
		t, result, "minimax", "minimax-primary",
	)

	// Verify specific model IDs
	if result[0].ID != "MiniMax-M2.5" {
		t.Errorf("Expected model ID=MiniMax-M2.5, got %s", result[0].ID)
	}

	if result[1].ID != "MiniMax-M2.1" {
		t.Errorf(
			"Expected model ID=MiniMax-M2.1, got %s",
			result[1].ID,
		)
	}
}

func TestMiniMaxListModelsDefaults(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProvider("test-minimax", "")

	result := provider.ListModels()

	// Should return default models when none configured
	if len(result) != len(providers.DefaultMiniMaxModels) {
		t.Errorf(
			"Expected %d default models, got %d",
			len(providers.DefaultMiniMaxModels), len(result),
		)
	}
}

func TestMiniMaxListModelsNilModels(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProviderWithModels("test-minimax", "", nil)

	result := provider.ListModels()

	// nil models should use defaults
	if len(result) != len(providers.DefaultMiniMaxModels) {
		t.Errorf(
			"Expected %d default models when nil, got %d",
			len(providers.DefaultMiniMaxModels), len(result),
		)
	}
}

func TestMiniMaxModelMapping(t *testing.T) {
	t.Parallel()

	mapping := map[string]string{
		"claude-sonnet-4-5-20250514": "MiniMax-M2.5",
	}
	provider := providers.NewMiniMaxProviderWithMapping(
		"test-minimax", "", nil, mapping,
	)

	// Mapped model should resolve
	if provider.MapModel("claude-sonnet-4-5-20250514") != "MiniMax-M2.5" {
		t.Error("Expected model mapping to resolve")
	}

	// Unmapped model should pass through
	if provider.MapModel("unknown-model") != "unknown-model" {
		t.Error("Expected unmapped model to pass through")
	}
}
