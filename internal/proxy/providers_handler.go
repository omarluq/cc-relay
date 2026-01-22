// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"encoding/json"
	"net/http"

	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/samber/lo"
)

// ProviderInfo represents provider information in the API response.
type ProviderInfo struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	BaseURL string   `json:"base_url"`
	Models  []string `json:"models"`
	Active  bool     `json:"active"`
}

// ProvidersResponse represents the response format for /v1/providers endpoint.
type ProvidersResponse struct {
	Object string         `json:"object"`
	Data   []ProviderInfo `json:"data"`
}

// ProvidersHandler handles requests to /v1/providers endpoint.
type ProvidersHandler struct {
	providers []providers.Provider
}

// NewProvidersHandler creates a new providers handler with the given providers.
func NewProvidersHandler(providerList []providers.Provider) *ProvidersHandler {
	return &ProvidersHandler{
		providers: providerList,
	}
}

// ServeHTTP handles GET /v1/providers requests.
func (h *ProvidersHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	// Collect provider information using lo.Map
	data := lo.Map(h.providers, func(p providers.Provider, _ int) ProviderInfo {
		// Extract model IDs from provider's models using lo.Map
		modelIDs := lo.Map(p.ListModels(), func(m providers.Model, _ int) string {
			return m.ID
		})

		return ProviderInfo{
			Name:    p.Name(),
			Type:    p.Owner(),
			BaseURL: p.BaseURL(),
			Models:  modelIDs,
			Active:  true,
		}
	})

	response := ProvidersResponse{
		Object: "list",
		Data:   data,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	//nolint:errcheck // Response is already committed with status code
	json.NewEncoder(w).Encode(response)
}
