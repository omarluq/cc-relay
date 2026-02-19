// Package proxy_test implements tests for the HTTP proxy server.
package proxy_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/proxy"
)

// serveModels creates a GET /v1/models request and records the response.
func serveModels(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestModelsHandlerReturnsCorrectFormat(t *testing.T) {
	t.Parallel()

	anthropicProvider := providers.NewAnthropicProviderWithModels(
		"anthropic-primary",
		"https://api.anthropic.com",
		[]string{"claude-sonnet-4-5-20250514", "claude-opus-4-5-20250514"},
	)

	handler := proxy.NewModelsHandler([]providers.Provider{anthropicProvider})
	rec := serveModels(t, handler)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, proxy.JSONContentType, rec.Header().Get("Content-Type"))

	var response proxy.ModelsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))

	assert.Equal(t, proxy.ListObject, response.Object)
	require.Len(t, response.Data, 2)
	assert.Equal(t, "claude-sonnet-4-5-20250514", response.Data[0].ID)
	assert.Equal(t, "model", response.Data[0].Object)
	assert.Equal(t, "anthropic", response.Data[0].OwnedBy)
	assert.Equal(t, "anthropic-primary", response.Data[0].Provider)
}

func TestModelsHandlerMultipleProviders(t *testing.T) {
	t.Parallel()

	anthropicProvider := providers.NewAnthropicProviderWithModels(
		"anthropic-primary",
		"https://api.anthropic.com",
		[]string{"claude-sonnet-4-5-20250514"},
	)

	zaiProvider := providers.NewZAIProviderWithModels(
		"zai-primary",
		"",
		[]string{"glm-4", "glm-4-plus"},
	)

	handler := proxy.NewModelsHandler([]providers.Provider{anthropicProvider, zaiProvider})

	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var response proxy.ModelsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have 3 models total (1 from anthropic + 2 from zai)
	if len(response.Data) != 3 {
		t.Fatalf("Expected 3 models, got %d", len(response.Data))
	}

	// Verify models from both providers are present
	modelIDs := make(map[string]bool)
	for _, m := range response.Data {
		modelIDs[m.ID] = true
	}

	expectedModels := []string{"claude-sonnet-4-5-20250514", "glm-4", "glm-4-plus"}
	for _, expected := range expectedModels {
		if !modelIDs[expected] {
			t.Errorf("Expected model %s to be present", expected)
		}
	}
}

func TestModelsHandlerEmptyProviders(t *testing.T) {
	t.Parallel()

	handler := proxy.NewModelsHandler([]providers.Provider{})
	rec := serveModels(t, handler)

	require.Equal(t, http.StatusOK, rec.Code)

	var response proxy.ModelsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, proxy.ListObject, response.Object)
	assert.Empty(t, response.Data)
}

func TestModelsHandlerProviderWithDefaultModels(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("anthropic", "https://api.anthropic.com")
	handler := proxy.NewModelsHandler([]providers.Provider{provider})
	rec := serveModels(t, handler)

	require.Equal(t, http.StatusOK, rec.Code)

	var response proxy.ModelsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Len(t, response.Data, len(providers.DefaultAnthropicModels))
}

func TestModelsHandlerNilProviders(t *testing.T) {
	t.Parallel()

	handler := proxy.NewModelsHandler(nil)
	rec := serveModels(t, handler)

	require.Equal(t, http.StatusOK, rec.Code)

	var response proxy.ModelsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, proxy.ListObject, response.Object)
	assert.Empty(t, response.Data)
}
