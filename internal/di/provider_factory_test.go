package di_test

import (
	"context"
	"testing"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/di"
	"github.com/stretchr/testify/assert"
)

// baseProviderConfig returns a fully zero-initialized ProviderConfig for the
// given name and type. Specialized helpers layer provider-specific fields on
// top of this base to avoid repeating the full struct literal.
func baseProviderConfig(name, pType string) config.ProviderConfig {
	return config.ProviderConfig{
		ModelMapping:       map[string]string{},
		AWSRegion:          "",
		GCPProjectID:       "",
		AzureAPIVersion:    "",
		Name:               name,
		Type:               pType,
		BaseURL:            "",
		AzureDeploymentID:  "",
		AWSAccessKeyID:     "",
		AzureResourceName:  "",
		AWSSecretAccessKey: "",
		GCPRegion:          "",
		Models:             nil,
		Pooling:            config.PoolingConfig{Enabled: false, Strategy: ""},
		Keys:               nil,
		Enabled:            true,
	}
}

// baseBedrockConfig returns a fully initialized ProviderConfig with bedrock type.
func baseBedrockConfig(name, region string) config.ProviderConfig {
	cfg := baseProviderConfig(name, di.ProviderTypeBedrock)
	cfg.AWSRegion = region
	return cfg
}

// baseVertexConfig returns a fully initialized ProviderConfig with vertex type.
//nolint:unparam // name receives "test-vertex" in tests but parameter kept for API consistency
func baseVertexConfig(name, project, region string) config.ProviderConfig {
	cfg := baseProviderConfig(name, di.ProviderTypeVertex)
	cfg.GCPProjectID = project
	cfg.GCPRegion = region
	return cfg
}

// baseAzureConfig returns a fully initialized ProviderConfig with azure type.
func baseAzureConfig(name, resource, deployment, apiVersion string) config.ProviderConfig {
	cfg := baseProviderConfig(name, di.ProviderTypeAzure)
	cfg.AzureResourceName = resource
	cfg.AzureDeploymentID = deployment
	cfg.AzureAPIVersion = apiVersion
	return cfg
}

// newEmptyProviderMapData returns a fully initialized empty providerMapData.
func newEmptyProviderMapData() *di.TestProviderMapData {
	return &di.TestProviderMapData{
		PrimaryProvider: nil,
		Providers:       map[string]di.Provider{},
		PrimaryKey:      "",
		AllProviders:    nil,
	}
}

// newMockProvider returns a fully initialized MockProvider.
func newMockProvider() *di.MockProvider {
	return &di.MockProvider{
		ModelMappingVal:    map[string]string{},
		NameVal:            "",
		BaseURLVal:         "",
		OwnerVal:           "",
		StreamingTypeVal:   "",
		StreamingVal:       false,
		TransparentAuthVal: false,
		BodyTransformVal:   false,
	}
}

func TestCreateCloudProviderBedrockValidation(t *testing.T) {
	t.Parallel()

	validCfg := baseBedrockConfig("test-bedrock", "us-east-1")
	validCfg.Models = []string{"claude-3-5-sonnet"}
	validCfg.ModelMapping = map[string]string{"anthropic": "claude-3-5-sonnet"}

	tests := []struct {
		name    string
		wantErr string
		cfg     config.ProviderConfig
	}{
		{
			name:    "missing AWS region",
			cfg:     baseBedrockConfig("test-bedrock", ""),
			wantErr: "bedrock provider test-bedrock: config: aws_region required for bedrock provider",
		},
		{
			name:    "valid bedrock config",
			cfg:     validCfg,
			wantErr: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			prov, err := di.CreateCloudProvider(ctx, &testCase.cfg)

			if testCase.wantErr != "" {
				assert.Error(t, err)
				assert.Equal(t, testCase.wantErr, err.Error())
				assert.Nil(t, prov)
				return
			}
			// Valid config may still fail without AWS credentials but shouldn't fail
			// validation. Either the provider is returned, or the error is NOT a
			// validation error (ErrUnknownProviderType / required-field error).
			if err == nil {
				assert.NotNil(t, prov, "expected provider when no error returned")
			} else {
				assert.NotErrorIs(t, err, di.ErrUnknownProviderType)
				assert.NotContains(t, err.Error(), "region is required")
				assert.NotContains(t, err.Error(), "required for bedrock provider")
			}
		})
	}
}

func TestCreateCloudProviderVertexValidation(t *testing.T) {
	t.Parallel()

	validCfg := baseVertexConfig("test-vertex", "test-project", "us-central1")
	validCfg.Models = []string{"claude-3-5-sonnet"}
	validCfg.ModelMapping = map[string]string{"anthropic": "claude-3-5-sonnet"}

	tests := []struct {
		name    string
		wantErr string
		cfg     config.ProviderConfig
	}{
		{
			name:    "missing GCP project ID",
			cfg:     baseVertexConfig("test-vertex", "", "us-central1"),
			wantErr: "vertex provider test-vertex: config: gcp_project_id required for vertex provider",
		},
		{
			name:    "missing GCP region",
			cfg:     baseVertexConfig("test-vertex", "test-project", ""),
			wantErr: "vertex provider test-vertex: config: gcp_region required for vertex provider",
		},
		{
			name:    "missing both GCP project ID and region",
			cfg:     baseVertexConfig("test-vertex", "", ""),
			wantErr: "vertex provider test-vertex: config: gcp_project_id required for vertex provider",
		},
		{
			name:    "valid vertex config",
			cfg:     validCfg,
			wantErr: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			prov, err := di.CreateCloudProvider(ctx, &testCase.cfg)

			if testCase.wantErr != "" {
				assert.Error(t, err)
				assert.Equal(t, testCase.wantErr, err.Error())
				assert.Nil(t, prov)
				return
			}
			// Valid config may still fail without GCP credentials but shouldn't fail
			// validation. Either the provider is returned, or the error is NOT a
			// validation error (ErrUnknownProviderType / required-field error).
			if err == nil {
				assert.NotNil(t, prov, "expected provider when no error returned")
			} else {
				assert.NotErrorIs(t, err, di.ErrUnknownProviderType)
				assert.NotContains(t, err.Error(), "project ID is required")
				assert.NotContains(t, err.Error(), "region is required")
				assert.NotContains(t, err.Error(), "required for vertex provider")
			}
		})
	}
}

func TestCreateCloudProviderAzureValidation(t *testing.T) {
	t.Parallel()

	validCfg := baseAzureConfig("test-azure", "test-resource", "test-deployment", "2024-02-01")
	validCfg.Models = []string{"claude-3-5-sonnet"}
	validCfg.ModelMapping = map[string]string{"anthropic": "claude-3-5-sonnet"}

	tests := []struct {
		name    string
		wantErr string
		cfg     config.ProviderConfig
	}{
		{
			name:    "missing Azure resource name",
			cfg:     baseAzureConfig("test-azure", "", "test-deployment", "2024-02-01"),
			wantErr: "azure provider test-azure: config: azure_resource_name required for azure provider",
		},
		{
			name:    "valid azure config",
			cfg:     validCfg,
			wantErr: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			prov, err := di.CreateCloudProvider(ctx, &testCase.cfg)

			if testCase.wantErr != "" {
				assert.Error(t, err)
				assert.Equal(t, testCase.wantErr, err.Error())
				assert.Nil(t, prov)
				return
			}
			// Valid config may still fail without Azure credentials but shouldn't fail
			// validation. Either the provider is returned, or the error is NOT a
			// validation error (ErrUnknownProviderType / required-field error).
			if err == nil {
				assert.NotNil(t, prov, "expected provider when no error returned")
			} else {
				assert.NotErrorIs(t, err, di.ErrUnknownProviderType)
				assert.NotContains(t, err.Error(), "resource name is required")
				assert.NotContains(t, err.Error(), "required for azure provider")
			}
		})
	}
}

func TestCreateCloudProviderUnknownType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := config.ProviderConfig{
		ModelMapping:       map[string]string{},
		AWSRegion:          "",
		GCPProjectID:       "",
		AzureAPIVersion:    "",
		Name:               "test-unknown",
		Type:               "unknown-type",
		BaseURL:            "",
		AzureDeploymentID:  "",
		AWSAccessKeyID:     "",
		AzureResourceName:  "",
		AWSSecretAccessKey: "",
		GCPRegion:          "",
		Models:             nil,
		Pooling:            config.PoolingConfig{Enabled: false, Strategy: ""},
		Keys:               nil,
		Enabled:            true,
	}

	prov, err := di.CreateCloudProvider(ctx, &cfg)

	assert.ErrorIs(t, err, di.ErrUnknownProviderType)
	assert.Nil(t, prov)
}

func TestCreateCloudProviderNonCloudType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// createCloudProvider should return ErrUnknownProviderType for non-cloud types
	nonCloudTypes := []string{
		di.ProviderTypeAnthropic,
		di.ProviderTypeZAI,
		di.ProviderTypeMiniMax,
		di.ProviderTypeOllama,
	}

	for _, pType := range nonCloudTypes {
		t.Run(pType, func(t *testing.T) {
			t.Parallel()
			cfg := config.ProviderConfig{
				ModelMapping:       map[string]string{},
				AWSRegion:          "",
				GCPProjectID:       "",
				AzureAPIVersion:    "",
				Name:               "test-" + pType,
				Type:               pType,
				BaseURL:            "https://api.example.com",
				AzureDeploymentID:  "",
				AWSAccessKeyID:     "",
				AzureResourceName:  "",
				AWSSecretAccessKey: "",
				GCPRegion:          "",
				Models:             nil,
				Pooling:            config.PoolingConfig{Enabled: false, Strategy: ""},
				Keys:               nil,
				Enabled:            true,
			}

			prov, err := di.CreateCloudProvider(ctx, &cfg)

			assert.ErrorIs(t, err, di.ErrUnknownProviderType)
			assert.Nil(t, prov)
		})
	}
}

func TestGetProvider(t *testing.T) {
	t.Parallel()

	mockProvider := newMockProvider()

	t.Run("found provider returns provider and true", func(t *testing.T) {
		t.Parallel()
		svc := di.NewProviderMapServiceWithConfigService(di.NewConfigServiceUninitialized())

		data := newEmptyProviderMapData()
		data.Providers = map[string]di.Provider{
			"test-provider": mockProvider,
		}
		svc.StoreProviderMapData(data)

		prov, ok := svc.GetProvider("test-provider")

		assert.Same(t, mockProvider, prov)
		assert.True(t, ok)
	})

	t.Run("not found provider returns nil and false", func(t *testing.T) {
		t.Parallel()
		svc := di.NewProviderMapServiceWithConfigService(di.NewConfigServiceUninitialized())

		data := newEmptyProviderMapData()
		data.Providers = map[string]di.Provider{
			"other-provider": mockProvider,
		}
		svc.StoreProviderMapData(data)

		prov, ok := svc.GetProvider("non-existent")

		assert.Nil(t, prov)
		assert.False(t, ok)
	})

	t.Run("nil providers map returns nil and false", func(t *testing.T) {
		t.Parallel()
		svc := di.NewProviderMapServiceWithConfigService(di.NewConfigServiceUninitialized())

		data := newEmptyProviderMapData()
		data.Providers = nil
		svc.StoreProviderMapData(data)

		prov, ok := svc.GetProvider("any-provider")

		assert.Nil(t, prov)
		assert.False(t, ok)
	})

	t.Run("empty providers map returns nil and false", func(t *testing.T) {
		t.Parallel()
		svc := di.NewProviderMapServiceWithConfigService(di.NewConfigServiceUninitialized())

		svc.StoreProviderMapData(newEmptyProviderMapData())

		prov, ok := svc.GetProvider("any-provider")

		assert.Nil(t, prov)
		assert.False(t, ok)
	})
}

func TestGetProviderLegacyFallback(t *testing.T) {
	t.Parallel()

	mockProvider := newMockProvider()

	t.Run("uses legacy field when atomic data is nil", func(t *testing.T) {
		t.Parallel()
		svc := di.NewProviderMapServiceWithConfigService(di.NewConfigServiceUninitialized())

		// Don't call StoreProviderMapData, so atomic data remains nil.
		svc.SetLegacyProviders(map[string]di.Provider{
			"legacy-provider": mockProvider,
		})

		prov, ok := svc.GetProvider("legacy-provider")

		assert.Equal(t, mockProvider, prov)
		assert.True(t, ok)
	})
}
