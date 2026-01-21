package providers

const (
	// DefaultZAIBaseURL is the default Z.AI API base URL.
	// Z.AI provides an Anthropic-compatible API endpoint.
	DefaultZAIBaseURL = "https://api.z.ai/api/anthropic"

	// ZAIOwner is the owner identifier for Z.AI (Zhipu AI) provider.
	ZAIOwner = "zhipu"
)

// ZAIProvider implements the Provider interface for Z.AI's Anthropic-compatible API.
// Z.AI (Zhipu AI) offers GLM models through an API that is compatible with Anthropic's
// Messages API format, making it a drop-in replacement for cost optimization.
// It embeds BaseProvider for common Anthropic-compatible functionality.
type ZAIProvider struct {
	BaseProvider
}

// NewZAIProvider creates a new Z.AI provider instance.
// If baseURL is empty, DefaultZAIBaseURL is used.
func NewZAIProvider(name, baseURL string) *ZAIProvider {
	return NewZAIProviderWithModels(name, baseURL, nil)
}

// NewZAIProviderWithModels creates a new Z.AI provider with configured models.
// If baseURL is empty, DefaultZAIBaseURL is used.
func NewZAIProviderWithModels(name, baseURL string, models []string) *ZAIProvider {
	if baseURL == "" {
		baseURL = DefaultZAIBaseURL
	}

	return &ZAIProvider{
		BaseProvider: NewBaseProvider(name, baseURL, ZAIOwner, models),
	}
}
