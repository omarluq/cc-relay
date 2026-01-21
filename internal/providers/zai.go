package providers

const (
	// DefaultZAIBaseURL is the default Z.AI API base URL.
	// Z.AI provides an Anthropic-compatible API endpoint.
	DefaultZAIBaseURL = "https://api.z.ai/api/anthropic"

	// ZAIOwner is the owner identifier for Z.AI (Zhipu AI) provider.
	ZAIOwner = "zhipu"
)

// DefaultZAIModels are the default models available from Z.AI.
var DefaultZAIModels = []string{
	"GLM-4.7",
	"GLM-4.5-Air",
	"GLM-4-Plus",
}

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
// If models is empty, DefaultZAIModels are used.
func NewZAIProviderWithModels(name, baseURL string, models []string) *ZAIProvider {
	if baseURL == "" {
		baseURL = DefaultZAIBaseURL
	}

	// Use default models if none configured
	if len(models) == 0 {
		models = DefaultZAIModels
	}

	return &ZAIProvider{
		BaseProvider: NewBaseProvider(name, baseURL, ZAIOwner, models),
	}
}
