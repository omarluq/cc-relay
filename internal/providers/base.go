// Package providers defines the interface for LLM backend providers.
package providers

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

// BaseProvider provides common functionality for Anthropic-compatible providers.
// It implements the shared methods that are identical across Anthropic and Z.AI.
type BaseProvider struct {
	name    string
	baseURL string
	owner   string
	models  []string
}

// NewBaseProvider creates a new base provider with the given parameters.
func NewBaseProvider(name, baseURL, owner string, models []string) BaseProvider {
	return BaseProvider{
		name:    name,
		baseURL: baseURL,
		models:  models,
		owner:   owner,
	}
}

// Name returns the provider identifier.
func (p *BaseProvider) Name() string {
	return p.name
}

// BaseURL returns the backend API base URL.
func (p *BaseProvider) BaseURL() string {
	return p.baseURL
}

// Owner returns the owner identifier.
func (p *BaseProvider) Owner() string {
	return p.owner
}

// Authenticate adds Anthropic-style authentication to the request.
// Sets the x-api-key header with the provided API key.
func (p *BaseProvider) Authenticate(req *http.Request, key string) error {
	req.Header.Set("x-api-key", key)

	// Log authentication (key is redacted for security)
	log.Ctx(req.Context()).Debug().
		Str("provider", p.name).
		Msg("added authentication header")

	return nil
}

// ForwardHeaders returns headers to forward to the backend.
// Copies all anthropic-* headers from the original request and adds Content-Type.
func (p *BaseProvider) ForwardHeaders(originalHeaders http.Header) http.Header {
	headers := make(http.Header)

	// Copy all anthropic-* headers from the original request using lo.ForEach
	// http.Header stores keys in canonical form (Title-Case)
	lo.ForEach(lo.Entries(originalHeaders), func(entry lo.Entry[string, []string], _ int) {
		canonicalKey := http.CanonicalHeaderKey(entry.Key)
		if len(canonicalKey) >= 10 && canonicalKey[:10] == "Anthropic-" {
			headers[canonicalKey] = append(headers[canonicalKey], entry.Value...)
		}
	})

	// Always set Content-Type for JSON requests
	headers.Set("Content-Type", "application/json")

	return headers
}

// SupportsStreaming indicates that the provider supports SSE streaming.
func (p *BaseProvider) SupportsStreaming() bool {
	return true
}

// SupportsTransparentAuth returns false by default.
// Non-Anthropic providers cannot accept Anthropic client tokens.
func (p *BaseProvider) SupportsTransparentAuth() bool {
	return false
}

// ListModels returns the list of available models for this provider.
func (p *BaseProvider) ListModels() []Model {
	if len(p.models) == 0 {
		return []Model{}
	}

	now := time.Now().Unix()

	// Use lo.Map to transform model IDs into Model structs
	return lo.Map(p.models, func(modelID string, _ int) Model {
		return Model{
			ID:       modelID,
			Object:   "model",
			OwnedBy:  p.owner,
			Provider: p.name,
			Created:  now,
		}
	})
}
