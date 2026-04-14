// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
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
	getProviders ProvidersGetter
}

// ProvidersGetter returns the current provider list for live updates.
type ProvidersGetter func() []providers.Provider

// NewModelsHandler creates a new models handler with the given providers.
// If getProviders is nil, a safe default returning an empty slice is used.
func NewModelsHandler(getProviders ProvidersGetter) *ModelsHandler {
	if getProviders == nil {
		getProviders = func() []providers.Provider { return nil }
	}
	return &ModelsHandler{
		getProviders: getProviders,
	}
}

func (h *ModelsHandler) providerList() []providers.Provider {
	return h.getProviders()
}

// ServeHTTP handles GET /v1/models requests.
func (h *ModelsHandler) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	// Collect all models from all providers using lo.FlatMap
	allModels := lo.FlatMap(h.providerList(), func(provider providers.Provider, _ int) []providers.Model {
		return provider.ListModels()
	})

	response := ModelsResponse{
		Object: "list",
		Data:   allModels,
	}

	writeJSON(writer, http.StatusOK, response)
}
