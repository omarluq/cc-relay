package providers

import "net/http"

const (
	// DefaultMoonshotBaseURL is Moonshot's Anthropic-compatible API base URL.
	DefaultMoonshotBaseURL = "https://api.moonshot.ai/anthropic"

	// MoonshotOwner identifies models served by Moonshot AI.
	MoonshotOwner = "moonshot"
)

// DefaultMoonshotModels lists the models supported by Moonshot's Messages API.
var DefaultMoonshotModels = []string{
	"kimi-k3.1-pro",
	"kimi-k3.1-pro-highspeed",
	"kimi-k3",
}

// MoonshotProvider serves Kimi models through Moonshot's Anthropic-compatible API.
// It uses a Moonshot platform API key, not a Kimi Code subscription key.
type MoonshotProvider struct {
	BaseProvider
}

// NewMoonshotProvider creates a provider, using defaults for an empty URL or model list.
func NewMoonshotProvider(name, baseURL string, models []string, modelMapping map[string]string) *MoonshotProvider {
	if baseURL == "" {
		baseURL = DefaultMoonshotBaseURL
	}
	if len(models) == 0 {
		models = DefaultMoonshotModels
	}

	return &MoonshotProvider{
		BaseProvider: NewBaseProviderWithMapping(name, baseURL, MoonshotOwner, models, modelMapping),
	}
}

// Authenticate replaces client credentials with the configured Moonshot API key.
func (p *MoonshotProvider) Authenticate(req *http.Request, key string) error {
	req.Header.Del("x-api-key")
	req.Header.Set("Authorization", "Bearer "+key)
	return nil
}
