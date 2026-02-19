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

func TestProvidersHandlerReturnsCorrectFormat(t *testing.T) {
	t.Parallel()

	anthropicProvider := providers.NewAnthropicProviderWithModels(
		"anthropic-primary",
		"https://api.anthropic.com",
		[]string{"claude-sonnet-4-5-20250514", "claude-opus-4-5-20250514"},
	)

	handler := proxy.NewProvidersHandler([]providers.Provider{anthropicProvider})
	rec := serveProviders(t, handler)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, proxy.JSONContentType, rec.Header().Get("Content-Type"))

	var response proxy.ProvidersResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))

	assert.Equal(t, proxy.ListObject, response.Object)
	require.Len(t, response.Data, 1)

	provider := response.Data[0]
	assert.Equal(t, "anthropic-primary", provider.Name)
	assert.Equal(t, "anthropic", provider.Type)
	assert.Equal(t, "https://api.anthropic.com", provider.BaseURL)
	assert.True(t, provider.Active)
	require.Len(t, provider.Models, 2)
	assert.ElementsMatch(t, []string{"claude-sonnet-4-5-20250514", "claude-opus-4-5-20250514"}, provider.Models)
}

// serveProviders creates a GET /v1/providers request and records the response.
func serveProviders(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/v1/providers", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestProvidersHandlerMultipleProviders(t *testing.T) {
	t.Parallel()

	anthropicProvider := providers.NewAnthropicProviderWithModels(
		"anthropic-primary",
		"https://api.anthropic.com",
		[]string{"claude-sonnet-4-5-20250514"},
	)

	zaiProvider := providers.NewZAIProviderWithModels(
		"zai-primary",
		"https://open.bigmodel.cn/api/paas/v4",
		[]string{"glm-4", "glm-4-plus"},
	)

	handler := proxy.NewProvidersHandler([]providers.Provider{anthropicProvider, zaiProvider})
	rec := serveProviders(t, handler)

	require.Equal(t, http.StatusOK, rec.Code)

	var response proxy.ProvidersResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	require.Len(t, response.Data, 2)

	providerNames := make(map[string]bool)
	for _, p := range response.Data {
		providerNames[p.Name] = true
		assert.True(t, p.Active, "Provider %s should be active", p.Name)
	}

	assert.Contains(t, providerNames, "anthropic-primary")
	assert.Contains(t, providerNames, "zai-primary")

	for _, p := range response.Data {
		if p.Name == "zai-primary" {
			assert.Len(t, p.Models, 2, "zai-primary should have 2 models")
		}
	}
}

func TestProvidersHandlerEmptyProviders(t *testing.T) {
	t.Parallel()

	handler := proxy.NewProvidersHandler([]providers.Provider{})
	rec := serveProviders(t, handler)

	require.Equal(t, http.StatusOK, rec.Code)

	var response proxy.ProvidersResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, proxy.ListObject, response.Object)
	assert.Empty(t, response.Data)
}

func TestProvidersHandlerProviderWithDefaultModels(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("anthropic", "https://api.anthropic.com")
	handler := proxy.NewProvidersHandler([]providers.Provider{provider})
	rec := serveProviders(t, handler)

	require.Equal(t, http.StatusOK, rec.Code)

	var response proxy.ProvidersResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	require.Len(t, response.Data, 1)

	expectedCount := len(providers.DefaultAnthropicModels)
	assert.Len(t, response.Data[0].Models, expectedCount)
}

func TestProvidersHandlerNilProviders(t *testing.T) {
	t.Parallel()

	handler := proxy.NewProvidersHandler(nil)
	rec := serveProviders(t, handler)

	require.Equal(t, http.StatusOK, rec.Code)

	var response proxy.ProvidersResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, proxy.ListObject, response.Object)
	assert.Empty(t, response.Data)
}
