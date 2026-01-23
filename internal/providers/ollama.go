package providers

const (
	// DefaultOllamaBaseURL is the default Ollama API base URL.
	// Ollama v0.14+ provides an Anthropic-compatible API endpoint.
	DefaultOllamaBaseURL = "http://localhost:11434"

	// OllamaOwner is the owner identifier for Ollama provider.
	OllamaOwner = "ollama"
)

// OllamaProvider implements the Provider interface for Ollama's Anthropic-compatible API.
// Ollama (v0.14+) offers local LLM models through an API that is compatible with Anthropic's
// Messages API format, enabling local inference as a drop-in replacement.
// It embeds BaseProvider for common Anthropic-compatible functionality.
type OllamaProvider struct {
	BaseProvider
}

// NewOllamaProvider creates a new Ollama provider instance.
// If baseURL is empty, DefaultOllamaBaseURL is used.
func NewOllamaProvider(name, baseURL string) *OllamaProvider {
	return NewOllamaProviderWithModels(name, baseURL, nil)
}

// NewOllamaProviderWithModels creates a new Ollama provider with configured models.
// If baseURL is empty, DefaultOllamaBaseURL is used.
// If models is empty/nil, an empty slice is used (Ollama models are user-installed).
func NewOllamaProviderWithModels(name, baseURL string, models []string) *OllamaProvider {
	if baseURL == "" {
		baseURL = DefaultOllamaBaseURL
	}

	// Use empty slice if no models configured (unlike Z.AI, Ollama has no standard model list)
	if models == nil {
		models = []string{}
	}

	return &OllamaProvider{
		BaseProvider: NewBaseProvider(name, baseURL, OllamaOwner, models),
	}
}
