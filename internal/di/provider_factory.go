package di

import (
	"context"
	"fmt"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
)

// ErrUnknownProviderType is returned when the provider type is not recognized.
var ErrUnknownProviderType = fmt.Errorf("unknown provider type")

// supportedProviderTypes is the list of supported provider types for error messages.
const supportedProviderTypes = "anthropic, zai, ollama, bedrock, vertex, azure"

// createProvider creates a provider instance from configuration.
// Returns ErrUnknownProviderType for unknown provider types.
func createProvider(ctx context.Context, p *config.ProviderConfig) (providers.Provider, error) {
	switch p.Type {
	case "anthropic":
		return providers.NewAnthropicProviderWithMapping(
			p.Name, p.BaseURL, p.Models, p.ModelMapping,
		), nil
	case "zai":
		return providers.NewZAIProviderWithMapping(
			p.Name, p.BaseURL, p.Models, p.ModelMapping,
		), nil
	case "ollama":
		return providers.NewOllamaProviderWithMapping(
			p.Name, p.BaseURL, p.Models, p.ModelMapping,
		), nil
	case "bedrock":
		if err := p.ValidateCloudConfig(); err != nil {
			return nil, fmt.Errorf("bedrock provider %s: %w", p.Name, err)
		}
		return providers.NewBedrockProvider(ctx, &providers.BedrockConfig{
			Name:         p.Name,
			Region:       p.AWSRegion,
			Models:       p.Models,
			ModelMapping: p.ModelMapping,
		})
	case "vertex":
		if err := p.ValidateCloudConfig(); err != nil {
			return nil, fmt.Errorf("vertex provider %s: %w", p.Name, err)
		}
		return providers.NewVertexProvider(ctx, &providers.VertexConfig{
			Name:         p.Name,
			ProjectID:    p.GCPProjectID,
			Region:       p.GCPRegion,
			Models:       p.Models,
			ModelMapping: p.ModelMapping,
		})
	case "azure":
		if err := p.ValidateCloudConfig(); err != nil {
			return nil, fmt.Errorf("azure provider %s: %w", p.Name, err)
		}
		return providers.NewAzureProvider(&providers.AzureConfig{
			Name:         p.Name,
			ResourceName: p.AzureResourceName,
			DeploymentID: p.AzureDeploymentID,
			APIVersion:   p.GetAzureAPIVersion(),
			Models:       p.Models,
			ModelMapping: p.ModelMapping,
		})
	default:
		return nil, ErrUnknownProviderType
	}
}
