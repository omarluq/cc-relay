// Package providers defines the interface for LLM backend providers.
package providers

import "net/http"

// Model represents an available model from a provider.
// This matches the Anthropic/OpenAI model list response format.
type Model struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	OwnedBy  string `json:"owned_by"`
	Provider string `json:"provider"`
	Created  int64  `json:"created"`
}

// Provider defines the interface for LLM backend providers.
// All provider implementations must implement this interface to be compatible with cc-relay.
type Provider interface {
	// Name returns the provider identifier.
	Name() string

	// BaseURL returns the backend API base URL.
	BaseURL() string

	// Owner returns the owner identifier (e.g., "anthropic", "zhipu").
	Owner() string

	// Authenticate adds provider-specific authentication to the request.
	// The key parameter is the API key to use for authentication.
	Authenticate(req *http.Request, key string) error

	// ForwardHeaders returns headers to add when forwarding the request.
	// This includes provider-specific headers and any anthropic-* headers from the original request.
	ForwardHeaders(originalHeaders http.Header) http.Header

	// SupportsStreaming indicates if the provider supports SSE streaming.
	SupportsStreaming() bool

	// SupportsTransparentAuth indicates if the provider accepts forwarded client auth.
	// When true, client's Authorization/x-api-key headers are passed through unchanged.
	// When false, the proxy uses configured API keys instead of client credentials.
	// Only Anthropic provider returns true since client tokens are valid for Anthropic API.
	SupportsTransparentAuth() bool

	// ListModels returns the list of available models for this provider.
	ListModels() []Model
}
