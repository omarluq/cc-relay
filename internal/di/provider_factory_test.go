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
func baseVertexConfig(project, region string) config.ProviderConfig {
	cfg := baseProviderConfig("test-vertex", di.ProviderTypeVertex)
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

// cloudProviderValidationCase is the common table-row type shared across all
// cloud-provider validation tests. Using a named struct (rather than an
// anonymous one per test) lets the runner accept the slice directly without
// per-test getter callbacks.
type cloudProviderValidationCase struct {
	name    string
	wantErr string
	cfg     config.ProviderConfig
}

// runCloudProviderValidation runs a table of cloud-provider validation cases
// against di.CreateCloudProvider. For an expected error, it asserts the exact
// error message. For a valid config (wantErr == ""), it tolerates credential
// failures but asserts the error is NOT a validation error by checking that
// none of forbiddenSubstrings appear in the error message.
func runCloudProviderValidation(
	t *testing.T,
	tests []cloudProviderValidationCase,
	forbiddenSubstrings []string,
) {
	t.Helper()
	for i := range tests {
		testCase := &tests[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			cfg := testCase.cfg
			prov, err := di.CreateCloudProvider(ctx, &cfg)

			if testCase.wantErr != "" {
				assert.Error(t, err)
				assert.Equal(t, testCase.wantErr, err.Error())
				assert.Nil(t, prov)
				return
			}
			// Valid config may still fail without cloud credentials but shouldn't fail
			// validation. Either the provider is returned, or the error is NOT a
			// validation error (ErrUnknownProviderType / required-field error).
			if err == nil {
				assert.NotNil(t, prov, "expected provider when no error returned")
				return
			}
			assert.NotErrorIs(t, err, di.ErrUnknownProviderType)
			for _, forbidden := range forbiddenSubstrings {
				assert.NotContains(t, err.Error(), forbidden)
			}
		})
	}
}

// withValidModels attaches a sample model and mapping to cfg in place.
// Used to promote a base provider config into a "valid" one for validation tests.
func withValidModels(cfg *config.ProviderConfig) {
	cfg.Models = []string{"claude-3-5-sonnet"}
	cfg.ModelMapping = map[string]string{"anthropic": "claude-3-5-sonnet"}
}

func TestCreateCloudProviderBedrockValidation(t *testing.T) {
	t.Parallel()

	tests := []cloudProviderValidationCase{
		{
			name:    "missing AWS region",
			cfg:     baseBedrockConfig("test-bedrock", ""),
			wantErr: "bedrock provider test-bedrock: config: aws_region required for bedrock provider",
		},
		{
			name: "valid bedrock config",
			cfg: func() config.ProviderConfig {
				c := baseBedrockConfig("test-bedrock", "us-east-1")
				withValidModels(&c)
				return c
			}(),
			wantErr: "",
		},
	}

	runCloudProviderValidation(t, tests, []string{"region is required", "required for bedrock provider"})
}

func TestCreateCloudProviderVertexValidation(t *testing.T) {
	t.Parallel()

	tests := []cloudProviderValidationCase{
		{
			name:    "missing GCP project ID",
			cfg:     baseVertexConfig("", "us-central1"),
			wantErr: "vertex provider test-vertex: config: gcp_project_id required for vertex provider",
		},
		{
			name:    "missing GCP region",
			cfg:     baseVertexConfig("test-project", ""),
			wantErr: "vertex provider test-vertex: config: gcp_region required for vertex provider",
		},
		{
			name:    "missing both GCP project ID and region",
			cfg:     baseVertexConfig("", ""),
			wantErr: "vertex provider test-vertex: config: gcp_project_id required for vertex provider",
		},
		{
			name: "valid vertex config",
			cfg: func() config.ProviderConfig {
				c := baseVertexConfig("test-project", "us-central1")
				withValidModels(&c)
				return c
			}(),
			wantErr: "",
		},
	}

	runCloudProviderValidation(t, tests, []string{
		"project ID is required", "region is required", "required for vertex provider",
	})
}

func TestCreateCloudProviderAzureValidation(t *testing.T) {
	t.Parallel()

	tests := []cloudProviderValidationCase{
		{
			name:    "missing Azure resource name",
			cfg:     baseAzureConfig("test-azure", "", "test-deployment", "2024-02-01"),
			wantErr: "azure provider test-azure: config: azure_resource_name required for azure provider",
		},
		{
			name: "valid azure config",
			cfg: func() config.ProviderConfig {
				c := baseAzureConfig("test-azure", "test-resource", "test-deployment", "2024-02-01")
				withValidModels(&c)
				return c
			}(),
			wantErr: "",
		},
	}

	runCloudProviderValidation(t, tests, []string{"resource name is required", "required for azure provider"})
}

// assertUnknownProviderType asserts that CreateCloudProvider returns
// ErrUnknownProviderType for the given config. Shared between the
// Unknown/NonCloud tests to keep assertion shape identical.
func assertUnknownProviderType(t *testing.T, cfg *config.ProviderConfig) {
	t.Helper()
	prov, err := di.CreateCloudProvider(context.Background(), cfg)
	assert.ErrorIs(t, err, di.ErrUnknownProviderType)
	assert.Nil(t, prov)
}

func TestCreateCloudProviderUnknownType(t *testing.T) {
	t.Parallel()
	cfg := baseProviderConfig("test-unknown", "unknown-type")
	assertUnknownProviderType(t, &cfg)
}

func TestCreateCloudProviderNonCloudType(t *testing.T) {
	t.Parallel()

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
			cfg := baseProviderConfig("test-"+pType, pType)
			cfg.BaseURL = "https://api.example.com"
			assertUnknownProviderType(t, &cfg)
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
