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
	providers    []providers.Provider
}

// ProvidersGetter returns the current provider list for live updates.
type ProvidersGetter func() []providers.Provider

// NewModelsHandler creates a new models handler with the given providers.
func NewModelsHandler(providerList []providers.Provider) *ModelsHandler {
	return &ModelsHandler{
		providers: providerList,
	}
}

// NewModelsHandlerWithProviderFunc creates a models handler with a live provider accessor.
func NewModelsHandlerWithProviderFunc(getProviders ProvidersGetter) *ModelsHandler {
	return &ModelsHandler{
		getProviders: getProviders,
	}
}

func (h *ModelsHandler) providerList() []providers.Provider {
	if h.getProviders != nil {
		return h.getProviders()
	}
	return h.providers
}

// ServeHTTP handles GET /v1/models requests.
func (h *ModelsHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	// Collect all models from all providers using lo.FlatMap
	allModels := lo.FlatMap(h.providerList(), func(provider providers.Provider, _ int) []providers.Model {
		return provider.ListModels()
	})

	response := ModelsResponse{
		Object: "list",
		Data:   allModels,
	}

	writeJSON(w, http.StatusOK, response)
}
