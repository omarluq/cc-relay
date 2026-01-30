package providers

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

const (
	// AzureOwner is the owner identifier for Azure Foundry provider.
	AzureOwner = "azure"

	// DefaultAzureAPIVersion is the default Azure API version.
	DefaultAzureAPIVersion = "2024-06-01"
)

// DefaultAzureModels are the default models available from Azure Foundry.
// These match Anthropic models available through Azure.
var DefaultAzureModels = []string{
	"claude-sonnet-4-5-20250514",
	"claude-opus-4-5-20250514",
	"claude-haiku-3-5-20241022",
}

// AzureProvider implements the Provider interface for Azure Foundry.
// Azure Foundry uses standard Anthropic API format with x-api-key authentication.
// The key difference is the URL structure:
// https://{resource}.services.ai.azure.com/models/chat/completions?api-version={version}
type AzureProvider struct {
	resourceName string
	deploymentID string
	apiVersion   string
	authMethod   string
	BaseProvider
}

// AzureConfig holds Azure-specific configuration.
type AzureConfig struct {
	ModelMapping map[string]string
	Name         string
	ResourceName string
	DeploymentID string
	APIVersion   string
	AuthMethod   string
	Models       []string
}

// NewAzureProvider creates a new Azure Foundry provider instance.
// Returns an error if required configuration is missing.
func NewAzureProvider(cfg *AzureConfig) (*AzureProvider, error) {
	if cfg.ResourceName == "" {
		return nil, fmt.Errorf("azure: resource_name is required")
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = DefaultAzureAPIVersion
	}
	if cfg.AuthMethod == "" {
		cfg.AuthMethod = "api_key"
	}
	if len(cfg.Models) == 0 {
		cfg.Models = DefaultAzureModels
	}

	// Construct base URL from resource name
	// Format: https://{resource-name}.services.ai.azure.com
	baseURL := fmt.Sprintf("https://%s.services.ai.azure.com", cfg.ResourceName)

	return &AzureProvider{
		BaseProvider: NewBaseProviderWithMapping(
			cfg.Name,
			baseURL,
			AzureOwner,
			cfg.Models,
			cfg.ModelMapping,
		),
		resourceName: cfg.ResourceName,
		deploymentID: cfg.DeploymentID,
		apiVersion:   cfg.APIVersion,
		authMethod:   cfg.AuthMethod,
	}, nil
}

// Authenticate adds Azure-specific authentication to the request.
// Uses x-api-key header (same as Anthropic) for API key auth.
// Uses Bearer token for Entra ID auth.
func (p *AzureProvider) Authenticate(req *http.Request, key string) error {
	if p.authMethod == "entra_id" {
		// Entra ID uses Bearer token
		req.Header.Set("Authorization", "Bearer "+key)
	} else {
		// API key uses x-api-key header (Anthropic-compatible)
		req.Header.Set("x-api-key", key)
	}

	log.Ctx(req.Context()).Debug().
		Str("provider", p.name).
		Str("auth_method", p.authMethod).
		Msg("added Azure authentication")

	return nil
}

// ForwardHeaders returns headers to forward to Azure.
// Includes anthropic-version header (required by Azure Foundry).
func (p *AzureProvider) ForwardHeaders(originalHeaders http.Header) http.Header {
	headers := p.BaseProvider.ForwardHeaders(originalHeaders)

	// Azure requires anthropic-version in header (unlike Bedrock/Vertex which use body)
	if headers.Get("Anthropic-Version") == "" {
		headers.Set("Anthropic-Version", "2023-06-01")
	}

	return headers
}

// TransformRequest constructs the Azure endpoint URL.
// Azure uses standard Anthropic body format (no model removal).
// URL format: https://{resource}.services.ai.azure.com/models/chat/completions?api-version={version}
func (p *AzureProvider) TransformRequest(
	body []byte,
	_ string,
) (newBody []byte, targetURL string, err error) {
	// Azure keeps model in body (same as direct Anthropic)
	// Construct URL with api-version query parameter
	targetURL = fmt.Sprintf("%s/models/chat/completions?api-version=%s",
		p.baseURL, p.apiVersion)

	return body, targetURL, nil
}

// RequiresBodyTransform returns false for Azure.
// Azure uses standard Anthropic body format.
func (p *AzureProvider) RequiresBodyTransform() bool {
	return false
}
