// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarluq/cc-relay/internal/providers"
)

func TestModelsHandlerReturnsCorrectFormat(t *testing.T) {
	t.Parallel()

	// Create providers with models
	anthropicProvider := providers.NewAnthropicProviderWithModels(
		"anthropic-primary",
		"https://api.anthropic.com",
		[]string{"claude-sonnet-4-5-20250514", "claude-opus-4-5-20250514"},
	)

	handler := NewModelsHandler([]providers.Provider{anthropicProvider})

	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
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
	var response ModelsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if response.Object != "list" {
		t.Errorf("Expected object=list, got %s", response.Object)
	}

	if len(response.Data) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(response.Data))
	}

	// Verify first model
	if response.Data[0].ID != "claude-sonnet-4-5-20250514" {
		t.Errorf("Expected first model ID=claude-sonnet-4-5-20250514, got %s", response.Data[0].ID)
	}
	if response.Data[0].Object != "model" {
		t.Errorf("Expected object=model, got %s", response.Data[0].Object)
	}
	if response.Data[0].OwnedBy != "anthropic" {
		t.Errorf("Expected owned_by=anthropic, got %s", response.Data[0].OwnedBy)
	}
	if response.Data[0].Provider != "anthropic-primary" {
		t.Errorf("Expected provider=anthropic-primary, got %s", response.Data[0].Provider)
	}
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

	handler := NewModelsHandler([]providers.Provider{anthropicProvider, zaiProvider})

	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var response ModelsResponse
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

	handler := NewModelsHandler([]providers.Provider{})

	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var response ModelsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Object != "list" {
		t.Errorf("Expected object=list, got %s", response.Object)
	}

	if len(response.Data) != 0 {
		t.Errorf("Expected 0 models, got %d", len(response.Data))
	}
}

func TestModelsHandlerProviderWithDefaultModels(t *testing.T) {
	t.Parallel()

	// Provider without configured models gets default models
	provider := providers.NewAnthropicProvider("anthropic", "https://api.anthropic.com")

	handler := NewModelsHandler([]providers.Provider{provider})

	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var response ModelsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Provider should return default models when none are explicitly configured
	expectedCount := len(providers.DefaultAnthropicModels)
	if len(response.Data) != expectedCount {
		t.Errorf("Expected %d default models, got %d", expectedCount, len(response.Data))
	}
}

func TestModelsHandlerNilProviders(t *testing.T) {
	t.Parallel()

	handler := NewModelsHandler(nil)

	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var response ModelsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Object != "list" {
		t.Errorf("Expected object=list, got %s", response.Object)
	}

	if len(response.Data) != 0 {
		t.Errorf("Expected 0 models, got %d", len(response.Data))
	}
}
