package di

import (
	"context"
	"fmt"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
)

// ErrUnknownProviderType is returned when the provider type is not recognized.
var ErrUnknownProviderType = fmt.Errorf("unknown provider type")

// Provider type constants.
const (
	ProviderTypeAnthropic = "anthropic"
	ProviderTypeZAI       = "zai"
	ProviderTypeOllama    = "ollama"
	ProviderTypeBedrock   = "bedrock"
	ProviderTypeVertex    = "vertex"
	ProviderTypeAzure     = "azure"
)

// supportedProviderTypes is the list of supported provider types for error messages.
const supportedProviderTypes = "anthropic, zai, ollama, bedrock, vertex, azure"

// createCloudProvider creates a cloud provider (bedrock, vertex, azure) with validation.
func createCloudProvider(ctx context.Context, providerConfig *config.ProviderConfig) (providers.Provider, error) {
	if err := providerConfig.ValidateCloudConfig(); err != nil {
		return nil, fmt.Errorf("%s provider %s: %w", providerConfig.Type, providerConfig.Name, err)
	}

	switch providerConfig.Type {
	case ProviderTypeBedrock:
		return providers.NewBedrockProvider(ctx, &providers.BedrockConfig{
			Name:         providerConfig.Name,
			Region:       providerConfig.AWSRegion,
			Models:       providerConfig.Models,
			ModelMapping: providerConfig.ModelMapping,
		})
	case ProviderTypeVertex:
		return providers.NewVertexProvider(ctx, &providers.VertexConfig{
			Name:         providerConfig.Name,
			ProjectID:    providerConfig.GCPProjectID,
			Region:       providerConfig.GCPRegion,
			Models:       providerConfig.Models,
			ModelMapping: providerConfig.ModelMapping,
		})
	case ProviderTypeAzure:
		return providers.NewAzureProvider(&providers.AzureConfig{
			Name:         providerConfig.Name,
			ResourceName: providerConfig.AzureResourceName,
			DeploymentID: providerConfig.AzureDeploymentID,
			APIVersion:   providerConfig.GetAzureAPIVersion(),
			Models:       providerConfig.Models,
			ModelMapping: providerConfig.ModelMapping,
			AuthMethod:   "",
		})
	default:
		return nil, ErrUnknownProviderType
	}
}

// createProvider creates a provider instance from configuration.
// Returns ErrUnknownProviderType for unknown provider types.
func createProvider(ctx context.Context, providerConfig *config.ProviderConfig) (providers.Provider, error) {
	switch providerConfig.Type {
	case ProviderTypeAnthropic:
		return providers.NewAnthropicProviderWithMapping(
			providerConfig.Name, providerConfig.BaseURL, providerConfig.Models, providerConfig.ModelMapping,
		), nil
	case ProviderTypeZAI:
		return providers.NewZAIProviderWithMapping(
			providerConfig.Name, providerConfig.BaseURL, providerConfig.Models, providerConfig.ModelMapping,
		), nil
	case ProviderTypeOllama:
		return providers.NewOllamaProviderWithMapping(
			providerConfig.Name, providerConfig.BaseURL, providerConfig.Models, providerConfig.ModelMapping,
		), nil
	case ProviderTypeBedrock, ProviderTypeVertex, ProviderTypeAzure:
		return createCloudProvider(ctx, providerConfig)
	default:
		return nil, ErrUnknownProviderType
	}
}
