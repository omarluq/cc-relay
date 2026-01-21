// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarluq/cc-relay/internal/providers"
)

func TestProvidersHandler_ReturnsCorrectFormat(t *testing.T) {
	t.Parallel()

	// Create provider with models
	anthropicProvider := providers.NewAnthropicProviderWithModels(
		"anthropic-primary",
		"https://api.anthropic.com",
		[]string{"claude-sonnet-4-5-20250514", "claude-opus-4-5-20250514"},
	)

	handler := NewProvidersHandler([]providers.Provider{anthropicProvider})

	req := httptest.NewRequest("GET", "/v1/providers", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	// Verify Content-Type
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type=application/json, got %s", rec.Header().Get("Content-Type"))
	}

	// Parse response
	var response ProvidersResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if response.Object != "list" {
		t.Errorf("Expected object=list, got %s", response.Object)
	}

	if len(response.Data) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(response.Data))
	}

	// Verify provider info
	provider := response.Data[0]
	if provider.Name != "anthropic-primary" {
		t.Errorf("Expected name=anthropic-primary, got %s", provider.Name)
	}
	if provider.Type != "anthropic" {
		t.Errorf("Expected type=anthropic, got %s", provider.Type)
	}
	if provider.BaseURL != "https://api.anthropic.com" {
		t.Errorf("Expected base_url=https://api.anthropic.com, got %s", provider.BaseURL)
	}
	if !provider.Active {
		t.Error("Expected active=true, got false")
	}
	if len(provider.Models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(provider.Models))
	}

	// Verify model IDs
	expectedModels := map[string]bool{
		"claude-sonnet-4-5-20250514": true,
		"claude-opus-4-5-20250514":   true,
	}
	for _, modelID := range provider.Models {
		if !expectedModels[modelID] {
			t.Errorf("Unexpected model ID: %s", modelID)
		}
	}
}

func TestProvidersHandler_MultipleProviders(t *testing.T) {
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

	handler := NewProvidersHandler([]providers.Provider{anthropicProvider, zaiProvider})

	req := httptest.NewRequest("GET", "/v1/providers", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var response ProvidersResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have 2 providers
	if len(response.Data) != 2 {
		t.Fatalf("Expected 2 providers, got %d", len(response.Data))
	}

	// Verify both providers are present
	providerNames := make(map[string]bool)
	for _, p := range response.Data {
		providerNames[p.Name] = true

		// All should be active
		if !p.Active {
			t.Errorf("Expected provider %s to be active", p.Name)
		}
	}

	expectedProviders := []string{"anthropic-primary", "zai-primary"}
	for _, expected := range expectedProviders {
		if !providerNames[expected] {
			t.Errorf("Expected provider %s to be present", expected)
		}
	}

	// Verify ZAI provider has correct number of models
	for _, p := range response.Data {
		if p.Name == "zai-primary" {
			if len(p.Models) != 2 {
				t.Errorf("Expected zai-primary to have 2 models, got %d", len(p.Models))
			}
		}
	}
}

func TestProvidersHandler_EmptyProviders(t *testing.T) {
	t.Parallel()

	handler := NewProvidersHandler([]providers.Provider{})

	req := httptest.NewRequest("GET", "/v1/providers", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var response ProvidersResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Object != "list" {
		t.Errorf("Expected object=list, got %s", response.Object)
	}

	if len(response.Data) != 0 {
		t.Errorf("Expected 0 providers, got %d", len(response.Data))
	}
}

func TestProvidersHandler_ProviderWithDefaultModels(t *testing.T) {
	t.Parallel()

	// Provider without explicitly configured models gets default models
	provider := providers.NewAnthropicProvider("anthropic", "https://api.anthropic.com")

	handler := NewProvidersHandler([]providers.Provider{provider})

	req := httptest.NewRequest("GET", "/v1/providers", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var response ProvidersResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Data) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(response.Data))
	}

	// Provider should have default models when none are explicitly configured
	expectedCount := len(providers.DefaultAnthropicModels)
	if len(response.Data[0].Models) != expectedCount {
		t.Errorf("Expected %d default models for provider, got %d", expectedCount, len(response.Data[0].Models))
	}
}

func TestProvidersHandler_NilProviders(t *testing.T) {
	t.Parallel()

	handler := NewProvidersHandler(nil)

	req := httptest.NewRequest("GET", "/v1/providers", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var response ProvidersResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Object != "list" {
		t.Errorf("Expected object=list, got %s", response.Object)
	}

	if len(response.Data) != 0 {
		t.Errorf("Expected 0 providers, got %d", len(response.Data))
	}
}
