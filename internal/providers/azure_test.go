package providers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarluq/cc-relay/internal/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testAzureConfig creates a base AzureConfig with all fields set.
// Use opts to override specific fields for testing.
func testAzureConfig(opts func(*providers.AzureConfig)) *providers.AzureConfig {
	cfg := &providers.AzureConfig{
		ModelMapping: nil,
		Name:         "test-azure",
		ResourceName: "my-resource",
		DeploymentID: "",
		APIVersion:   "",
		AuthMethod:   "",
		Models:       nil,
	}
	if opts != nil {
		opts(cfg)
	}
	return cfg
}

// testAzureAuthenticate is a helper function to test authentication.
func testAzureAuthenticate(
	t *testing.T,
	authMethod string,
	key string,
	expectedHeader string,
	expectedValue string,
) {
	t.Helper()
	cfg := testAzureConfig(func(c *providers.AzureConfig) {
		c.AuthMethod = authMethod
	})
	provider, err := providers.NewAzureProvider(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
	err = provider.Authenticate(req, key)
	require.NoError(t, err)

	assert.Equal(t, expectedValue, req.Header.Get(expectedHeader))
}

func TestNewAzureProviderCreatesProvider(t *testing.T) {
	t.Parallel()
	t.Run("creates provider with required config", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(func(c *providers.AzureConfig) {
			c.DeploymentID = "claude-deployment"
		})
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)

		assert.Equal(t, "test-azure", provider.Name())
		assert.Equal(t, "https://my-resource.services.ai.azure.com", provider.BaseURL())
		assert.Equal(t, providers.AzureOwner, provider.Owner())
		assert.Equal(t, providers.DefaultAzureAPIVersion, provider.AzureAPIVersion())
		assert.Equal(t, "api_key", provider.AzureAuthMethod())
	})

	t.Run("returns error when resource_name is missing", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(func(c *providers.AzureConfig) {
			c.ResourceName = ""
		})
		_, err := providers.NewAzureProvider(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resource_name is required")
	})

	t.Run("uses custom API version", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(func(c *providers.AzureConfig) {
			c.APIVersion = "2024-12-01"
		})
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)
		assert.Equal(t, "2024-12-01", provider.AzureAPIVersion())
	})

	t.Run("uses default models when none specified", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(nil)
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)
		models := provider.ListModels()
		assert.Len(t, models, len(providers.DefaultAzureModels))
	})
}

func TestNewAzureProviderConfigOptions(t *testing.T) {
	t.Parallel()
	t.Run("uses custom models when specified", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(func(c *providers.AzureConfig) {
			c.Models = []string{"custom-model"}
		})
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)
		models := provider.ListModels()
		assert.Len(t, models, 1)
		assert.Equal(t, "custom-model", models[0].ID)
	})

	t.Run("stores resource name and deployment ID", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(func(c *providers.AzureConfig) {
			c.ResourceName = "my-special-resource"
			c.DeploymentID = "my-deployment-123"
		})
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)
		assert.Equal(t, "my-special-resource", provider.AzureResourceName())
		assert.Equal(t, "my-deployment-123", provider.AzureDeploymentID())
	})

	t.Run("uses api_key auth method by default", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(nil)
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)
		assert.Equal(t, "api_key", provider.AzureAuthMethod())
	})

	t.Run("accepts entra_id auth method", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(func(c *providers.AzureConfig) {
			c.AuthMethod = "entra_id"
		})
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)
		assert.Equal(t, "entra_id", provider.AzureAuthMethod())
	})
}

func TestAzureProviderAuthenticate(t *testing.T) {
	t.Parallel()
	t.Run("uses x-api-key for api_key auth", func(t *testing.T) {
		t.Parallel()
		testAzureAuthenticate(t, "api_key", "test-key-123", "x-api-key", "test-key-123")
	})

	t.Run("uses Bearer token for entra_id auth", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(func(c *providers.AzureConfig) {
			c.AuthMethod = "entra_id"
		})
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err = provider.Authenticate(req, "entra-token-xyz")

		require.NoError(t, err)
		assert.Equal(t, "Bearer entra-token-xyz", req.Header.Get("Authorization"))
		assert.Empty(t, req.Header.Get("x-api-key"))
	})

	t.Run("default auth method uses x-api-key", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(nil)
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err = provider.Authenticate(req, "default-key")

		require.NoError(t, err)
		assert.Equal(t, "default-key", req.Header.Get("x-api-key"))
	})
}

func TestAzureProviderForwardHeaders(t *testing.T) {
	t.Parallel()
	cfg := testAzureConfig(nil)
	provider, err := providers.NewAzureProvider(cfg)
	require.NoError(t, err)

	t.Run("adds anthropic-version header if missing", func(t *testing.T) {
		t.Parallel()
		origHeaders := http.Header{}
		headers := provider.ForwardHeaders(origHeaders)

		assert.Equal(t, "2023-06-01", headers.Get("Anthropic-Version"))
		assert.Equal(t, "application/json", headers.Get("Content-Type"))
	})

	t.Run("preserves existing anthropic-version", func(t *testing.T) {
		t.Parallel()
		origHeaders := http.Header{}
		origHeaders.Set("Anthropic-Version", "2024-01-01")
		headers := provider.ForwardHeaders(origHeaders)

		assert.Equal(t, "2024-01-01", headers.Get("Anthropic-Version"))
	})

	t.Run("forwards other anthropic headers", func(t *testing.T) {
		t.Parallel()
		origHeaders := http.Header{}
		origHeaders.Set("Anthropic-Beta", "tools-2024-04-04")
		headers := provider.ForwardHeaders(origHeaders)

		assert.Equal(t, "tools-2024-04-04", headers.Get("Anthropic-Beta"))
	})

	t.Run("sets content-type to application/json", func(t *testing.T) {
		t.Parallel()
		origHeaders := http.Header{}
		headers := provider.ForwardHeaders(origHeaders)

		assert.Equal(t, "application/json", headers.Get("Content-Type"))
	})
}

func TestAzureProviderTransformRequest(t *testing.T) {
	t.Parallel()
	t.Run("constructs correct URL with api-version", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(func(c *providers.AzureConfig) {
			c.APIVersion = "2024-06-01"
		})
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)

		body := []byte(`{"model":"claude-sonnet-4-5-20250514","messages":[]}`)

		newBody, targetURL, err := provider.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		assert.Equal(t, body, newBody) // Body unchanged
		assert.Equal(t,
			"https://my-resource.services.ai.azure.com/models/chat/completions?api-version=2024-06-01",
			targetURL)
	})

	t.Run("body is unchanged", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(nil)
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)

		body := []byte(`{"model":"claude-sonnet-4-5-20250514","messages":[{"role":"user","content":"Hello"}]}`)

		newBody, _, err := provider.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		assert.JSONEq(t, string(body), string(newBody))
	})

	t.Run("uses custom api-version in URL", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(func(c *providers.AzureConfig) {
			c.ResourceName = "custom-res"
			c.APIVersion = "2025-01-15"
		})
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)

		body := []byte(`{"model":"test"}`)
		_, targetURL, err := provider.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		assert.Contains(t, targetURL, "api-version=2025-01-15")
		assert.Contains(t, targetURL, "custom-res.services.ai.azure.com")
	})
}

func TestAzureProviderTransformRequestEndpoint(t *testing.T) {
	t.Parallel()
	t.Run("ignores endpoint parameter", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(func(c *providers.AzureConfig) {
			c.APIVersion = "2024-06-01"
		})
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)

		body := []byte(`{"model":"test"}`)
		_, targetURL1, err1 := provider.TransformRequest(body, "/v1/messages")
		require.NoError(t, err1)
		_, targetURL2, err2 := provider.TransformRequest(body, "/different/endpoint")
		require.NoError(t, err2)

		// Both should produce the same URL
		assert.Equal(t, targetURL1, targetURL2)
	})
}

func TestAzureProviderRequiresBodyTransform(t *testing.T) {
	t.Parallel()
	cfg := testAzureConfig(nil)
	provider, err := providers.NewAzureProvider(cfg)
	require.NoError(t, err)

	assert.False(t, provider.RequiresBodyTransform())
}

func TestAzureProviderSupportsStreaming(t *testing.T) {
	t.Parallel()
	cfg := testAzureConfig(nil)
	provider, err := providers.NewAzureProvider(cfg)
	require.NoError(t, err)

	assert.True(t, provider.SupportsStreaming())
}

func TestAzureProviderStreamingContentType(t *testing.T) {
	t.Parallel()
	cfg := testAzureConfig(nil)
	provider, err := providers.NewAzureProvider(cfg)
	require.NoError(t, err)

	assert.Equal(t, "text/event-stream", provider.StreamingContentType())
}

func TestAzureProviderSupportsTransparentAuth(t *testing.T) {
	t.Parallel()
	cfg := testAzureConfig(nil)
	provider, err := providers.NewAzureProvider(cfg)
	require.NoError(t, err)

	// Azure does NOT support transparent auth (Anthropic tokens not valid)
	assert.False(t, provider.SupportsTransparentAuth())
}

func TestAzureProviderModelMapping(t *testing.T) {
	t.Parallel()
	cfg := testAzureConfig(func(c *providers.AzureConfig) {
		c.ModelMapping = map[string]string{
			"claude-4": "claude-sonnet-4-5-20250514",
		}
	})
	provider, err := providers.NewAzureProvider(cfg)
	require.NoError(t, err)

	t.Run("maps known model", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "claude-sonnet-4-5-20250514", provider.MapModel("claude-4"))
	})

	t.Run("returns original for unknown model", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "unknown-model", provider.MapModel("unknown-model"))
	})
}

func TestAzureProviderGetModelMapping(t *testing.T) {
	t.Parallel()
	t.Run("returns model mapping when configured", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(func(c *providers.AzureConfig) {
			c.ModelMapping = map[string]string{
				"alias": "real-model",
			}
		})
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)

		mapping := provider.GetModelMapping()
		assert.NotNil(t, mapping)
		assert.Equal(t, "real-model", mapping["alias"])
	})

	t.Run("returns nil when no mapping configured", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(nil)
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)

		mapping := provider.GetModelMapping()
		assert.Nil(t, mapping)
	})
}

func TestAzureProviderListModels(t *testing.T) {
	t.Parallel()
	t.Run("returns default models with correct metadata", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(nil)
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)

		models := provider.ListModels()
		assert.Len(t, models, len(providers.DefaultAzureModels))

		for _, model := range models {
			assert.Equal(t, "model", model.Object)
			assert.Equal(t, providers.AzureOwner, model.OwnedBy)
			assert.Equal(t, "test-azure", model.Provider)
			assert.Greater(t, model.Created, int64(0))
		}
	})

	t.Run("returns custom models when specified", func(t *testing.T) {
		t.Parallel()
		cfg := testAzureConfig(func(c *providers.AzureConfig) {
			c.Models = []string{"custom-model-1", "custom-model-2"}
		})
		provider, err := providers.NewAzureProvider(cfg)
		require.NoError(t, err)

		models := provider.ListModels()
		assert.Len(t, models, 2)
		assert.Equal(t, "custom-model-1", models[0].ID)
		assert.Equal(t, "custom-model-2", models[1].ID)
	})
}

func TestDefaultAzureModels(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, providers.DefaultAzureModels)
	assert.Contains(t, providers.DefaultAzureModels, "claude-sonnet-4-5-20250514")
	assert.Contains(t, providers.DefaultAzureModels, "claude-opus-4-6")
	assert.Contains(t, providers.DefaultAzureModels, "claude-opus-4-5-20250514")
	assert.Contains(t, providers.DefaultAzureModels, "claude-haiku-3-5-20241022")
}

func TestAzureOwner(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "azure", providers.AzureOwner)
}

func TestDefaultAzureAPIVersion(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "2024-06-01", providers.DefaultAzureAPIVersion)
}
