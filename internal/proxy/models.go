// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"encoding/json"
	"net/http"

	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/samber/lo"
)

// ModelsResponse represents the response format for /v1/models endpoint.
// This matches the Anthropic/OpenAI model list response format.
type ModelsResponse struct {
	Object string            `json:"object"`
	Data   []providers.Model `json:"data"`
}

// ModelsHandler handles requests to /v1/models endpoint.
type ModelsHandler struct {
	providers []providers.Provider
}

// NewModelsHandler creates a new models handler with the given providers.
func NewModelsHandler(providerList []providers.Provider) *ModelsHandler {
	return &ModelsHandler{
		providers: providerList,
	}
}

// ServeHTTP handles GET /v1/models requests.
func (h *ModelsHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	// Collect all models from all providers using lo.FlatMap
	allModels := lo.FlatMap(h.providers, func(provider providers.Provider, _ int) []providers.Model {
		return provider.ListModels()
	})

	response := ModelsResponse{
		Object: "list",
		Data:   allModels,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	//nolint:errcheck // Response is already committed with status code
	json.NewEncoder(w).Encode(response)
}
