package providers

import "net/http"

const (
	// DefaultMiniMaxBaseURL is the default MiniMax API base URL.
	// MiniMax provides an Anthropic-compatible API endpoint.
	DefaultMiniMaxBaseURL = "https://api.minimax.io/anthropic"

	// MiniMaxOwner is the owner identifier for MiniMax provider.
	MiniMaxOwner = "minimax"
)

// DefaultMiniMaxModels are the default models available from MiniMax.
var DefaultMiniMaxModels = []string{
	"MiniMax-M2.5",
	"MiniMax-M2.5-highspeed",
	"MiniMax-M2.1",
	"MiniMax-M2.1-highspeed",
	"MiniMax-M2",
}

// MiniMaxProvider implements the Provider interface for MiniMax's Anthropic-compatible API.
// MiniMax offers models through an API that is compatible with Anthropic's
// Messages API format, making it a drop-in replacement for cost optimization.
// It embeds BaseProvider for common Anthropic-compatible functionality.
// The key difference is that MiniMax uses Bearer token authentication instead of x-api-key.
type MiniMaxProvider struct {
	BaseProvider
}

// NewMiniMaxProvider creates a new MiniMax provider instance.
// If baseURL is empty, DefaultMiniMaxBaseURL is used.
func NewMiniMaxProvider(name, baseURL string) *MiniMaxProvider {
	return NewMiniMaxProviderWithModels(name, baseURL, nil)
}

// NewMiniMaxProviderWithModels creates a new MiniMax provider with configured models.
// If baseURL is empty, DefaultMiniMaxBaseURL is used.
// If models is empty, DefaultMiniMaxModels are used.
func NewMiniMaxProviderWithModels(name, baseURL string, models []string) *MiniMaxProvider {
	return NewMiniMaxProviderWithMapping(name, baseURL, models, nil)
}

// NewMiniMaxProviderWithMapping creates a new MiniMax provider with model mapping.
// If baseURL is empty, DefaultMiniMaxBaseURL is used.
// If models is empty, DefaultMiniMaxModels are used.
func NewMiniMaxProviderWithMapping(
	name, baseURL string,
	models []string,
	modelMapping map[string]string,
) *MiniMaxProvider {
	if baseURL == "" {
		baseURL = DefaultMiniMaxBaseURL
	}

	// Use default models if none configured
	if len(models) == 0 {
		models = DefaultMiniMaxModels
	}

	return &MiniMaxProvider{
		BaseProvider: NewBaseProviderWithMapping(name, baseURL, MiniMaxOwner, models, modelMapping),
	}
}

// Authenticate adds MiniMax-style authentication to the request.
// MiniMax uses Bearer token authentication instead of x-api-key.
func (p *MiniMaxProvider) Authenticate(req *http.Request, key string) error {
	req.Header.Set("Authorization", "Bearer "+key)

	return nil
}
