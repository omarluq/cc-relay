package di

import (
	"context"
	"fmt"
	"net/http"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
)

// ErrUnknownProviderType is returned when the provider type is not recognized.
var ErrUnknownProviderType = fmt.Errorf("unknown provider type")

// Provider type constants.
const (
	ProviderTypeAnthropic = "anthropic"
	ProviderTypeZAI       = "zai"
	ProviderTypeMiniMax   = "minimax"
	ProviderTypeOllama    = "ollama"
	ProviderTypeBedrock   = "bedrock"
	ProviderTypeVertex    = "vertex"
	ProviderTypeAzure     = "azure"
)

// supportedProviderTypes is the list of supported provider types for error messages.
const supportedProviderTypes = "anthropic, zai, minimax, ollama, bedrock, vertex, azure"

// outboundTransportSetter is implemented by providers embedding providers.BaseProvider.
type outboundTransportSetter interface {
	SetOutboundTransport(transport http.RoundTripper)
}

// createCloudProvider creates a cloud provider (bedrock, vertex, azure) with validation.
// A nil transport keeps the SDK and http.DefaultTransport defaults.
func createCloudProvider(
	ctx context.Context,
	providerConfig *config.ProviderConfig,
	transport http.RoundTripper,
) (providers.Provider, error) {
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
			Transport:    transport,
		})
	case ProviderTypeVertex:
		return providers.NewVertexProvider(ctx, &providers.VertexConfig{
			Name:         providerConfig.Name,
			ProjectID:    providerConfig.GCPProjectID,
			Region:       providerConfig.GCPRegion,
			Models:       providerConfig.Models,
			ModelMapping: providerConfig.ModelMapping,
			Transport:    transport,
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
// The transport carries the provider's outbound proxy; nil means http.DefaultTransport.
// Returns ErrUnknownProviderType for unknown provider types.
func createProvider(
	ctx context.Context,
	providerConfig *config.ProviderConfig,
	transport http.RoundTripper,
) (providers.Provider, error) {
	prov, err := newProvider(ctx, providerConfig, transport)
	if err != nil {
		return nil, err
	}
	if setter, ok := prov.(outboundTransportSetter); ok {
		setter.SetOutboundTransport(transport)
	}
	return prov, nil
}

func newProvider(
	ctx context.Context,
	providerConfig *config.ProviderConfig,
	transport http.RoundTripper,
) (providers.Provider, error) {
	switch providerConfig.Type {
	case ProviderTypeAnthropic:
		return providers.NewAnthropicProvider(
			providerConfig.Name, providerConfig.BaseURL, providerConfig.Models, providerConfig.ModelMapping,
		), nil
	case ProviderTypeZAI:
		return providers.NewZAIProvider(
			providerConfig.Name, providerConfig.BaseURL, providerConfig.Models, providerConfig.ModelMapping,
		), nil
	case ProviderTypeMiniMax:
		return providers.NewMiniMaxProvider(
			providerConfig.Name, providerConfig.BaseURL, providerConfig.Models, providerConfig.ModelMapping,
		), nil
	case ProviderTypeOllama:
		return providers.NewOllamaProvider(
			providerConfig.Name, providerConfig.BaseURL, providerConfig.Models, providerConfig.ModelMapping,
		), nil
	case ProviderTypeBedrock, ProviderTypeVertex, ProviderTypeAzure:
		return createCloudProvider(ctx, providerConfig, transport)
	default:
		return nil, ErrUnknownProviderType
	}
}
