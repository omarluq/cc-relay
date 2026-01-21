package providers

const (
	// DefaultAnthropicBaseURL is the default Anthropic API base URL.
	DefaultAnthropicBaseURL = "https://api.anthropic.com"

	// AnthropicOwner is the owner identifier for Anthropic provider.
	AnthropicOwner = "anthropic"
)

// AnthropicProvider implements the Provider interface for Anthropic's API.
// It embeds BaseProvider for common Anthropic-compatible functionality.
type AnthropicProvider struct {
	BaseProvider
}

// NewAnthropicProvider creates a new Anthropic provider instance.
// If baseURL is empty, DefaultAnthropicBaseURL is used.
func NewAnthropicProvider(name, baseURL string) *AnthropicProvider {
	return NewAnthropicProviderWithModels(name, baseURL, nil)
}

// NewAnthropicProviderWithModels creates a new Anthropic provider with configured models.
// If baseURL is empty, DefaultAnthropicBaseURL is used.
func NewAnthropicProviderWithModels(name, baseURL string, models []string) *AnthropicProvider {
	if baseURL == "" {
		baseURL = DefaultAnthropicBaseURL
	}

	return &AnthropicProvider{
		BaseProvider: NewBaseProvider(name, baseURL, AnthropicOwner, models),
	}
}
