package providers

const (
	// DefaultAnthropicBaseURL is the default Anthropic API base URL.
	DefaultAnthropicBaseURL = "https://api.anthropic.com"

	// AnthropicOwner is the owner identifier for Anthropic provider.
	AnthropicOwner = "anthropic"
)

// DefaultAnthropicModels are the default models available from Anthropic.
var DefaultAnthropicModels = []string{
	"claude-sonnet-4-5-20250514",
	"claude-opus-4-5-20250514",
	"claude-haiku-3-5-20241022",
}

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
// If models is empty, DefaultAnthropicModels are used.
func NewAnthropicProviderWithModels(name, baseURL string, models []string) *AnthropicProvider {
	if baseURL == "" {
		baseURL = DefaultAnthropicBaseURL
	}

	// Use default models if none configured
	if len(models) == 0 {
		models = DefaultAnthropicModels
	}

	return &AnthropicProvider{
		BaseProvider: NewBaseProvider(name, baseURL, AnthropicOwner, models),
	}
}

// SupportsTransparentAuth returns true for Anthropic.
// Client tokens (from Claude Code subscriptions) are valid for direct Anthropic API calls.
func (p *AnthropicProvider) SupportsTransparentAuth() bool {
	return true
}
