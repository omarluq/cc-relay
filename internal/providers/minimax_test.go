package providers_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/omarluq/cc-relay/internal/providers"
)

const (
	testMiniMaxName  = "test-minimax"
	modelMiniMaxM25  = "MiniMax-M2.5"
	modelMiniMaxM21  = "MiniMax-M2.1"
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

	provider := providers.NewMiniMaxProvider(testMiniMaxName, "")

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

	provider := providers.NewMiniMaxProvider(testMiniMaxName, "")

	assertForwardHeaders(t, provider)
}

func TestMiniMaxSupportsStreaming(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProvider(testMiniMaxName, "")

	if !provider.SupportsStreaming() {
		t.Error("Expected MiniMaxProvider to support streaming")
	}
}

func TestMiniMaxForwardHeadersEdgeCases(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProvider(testMiniMaxName, "")

	assertForwardHeadersEdgeCases(t, provider)
}

func TestMiniMaxProviderInterface(t *testing.T) {
	t.Parallel()

	// Verify MiniMaxProvider implements Provider interface
	var _ providers.Provider = (*providers.MiniMaxProvider)(nil)
}

func TestMiniMaxOwner(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProvider(testMiniMaxName, "")

	if provider.Owner() != "minimax" {
		t.Errorf("Expected owner=minimax, got %s", provider.Owner())
	}
}

func TestMiniMaxSupportsTransparentAuth(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProvider(testMiniMaxName, "")

	if provider.SupportsTransparentAuth() {
		t.Error("Expected MiniMaxProvider to NOT support transparent auth")
	}
}

func TestMiniMaxListModelsWithConfiguredModels(t *testing.T) {
	t.Parallel()

	models := []string{modelMiniMaxM25, modelMiniMaxM21}
	provider := providers.NewMiniMaxProviderWithModels(
		"minimax-primary", "", models,
	)

	result := provider.ListModels()

	assertListModelsWithConfiguredModels(
		t, result, "minimax", "minimax-primary",
	)

	// Verify specific model IDs
	if result[0].ID != modelMiniMaxM25 {
		t.Errorf("Expected model ID=%s, got %s", modelMiniMaxM25, result[0].ID)
	}

	if result[1].ID != modelMiniMaxM21 {
		t.Errorf(
			"Expected model ID=%s, got %s",
			modelMiniMaxM21, result[1].ID,
		)
	}
}

func TestMiniMaxListModelsDefaults(t *testing.T) {
	t.Parallel()

	provider := providers.NewMiniMaxProvider(testMiniMaxName, "")

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

	provider := providers.NewMiniMaxProviderWithModels(testMiniMaxName, "", nil)

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
		testMiniMaxName, "", nil, mapping,
	)

	// Mapped model should resolve
	if provider.MapModel("claude-sonnet-4-5-20250514") != modelMiniMaxM25 {
		t.Error("Expected model mapping to resolve")
	}

	// Unmapped model should pass through
	if provider.MapModel("unknown-model") != "unknown-model" {
		t.Error("Expected unmapped model to pass through")
	}
}
