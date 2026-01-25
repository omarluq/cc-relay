package providers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAzureProvider(t *testing.T) {
	t.Run("creates provider with required config", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			DeploymentID: "claude-deployment",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)

		assert.Equal(t, "test-azure", p.Name())
		assert.Equal(t, "https://my-resource.services.ai.azure.com", p.BaseURL())
		assert.Equal(t, AzureOwner, p.Owner())
		assert.Equal(t, DefaultAzureAPIVersion, p.apiVersion)
		assert.Equal(t, "api_key", p.authMethod)
	})

	t.Run("returns error when resource_name is missing", func(t *testing.T) {
		cfg := &AzureConfig{
			Name: "test-azure",
			// ResourceName intentionally missing
		}
		_, err := NewAzureProvider(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resource_name is required")
	})

	t.Run("uses custom API version", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			APIVersion:   "2024-12-01",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)
		assert.Equal(t, "2024-12-01", p.apiVersion)
	})

	t.Run("uses default models when none specified", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)
		models := p.ListModels()
		assert.Len(t, models, len(DefaultAzureModels))
	})

	t.Run("uses custom models when specified", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			Models:       []string{"custom-model"},
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)
		models := p.ListModels()
		assert.Len(t, models, 1)
		assert.Equal(t, "custom-model", models[0].ID)
	})

	t.Run("stores resource name and deployment ID", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-special-resource",
			DeploymentID: "my-deployment-123",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)
		assert.Equal(t, "my-special-resource", p.resourceName)
		assert.Equal(t, "my-deployment-123", p.deploymentID)
	})

	t.Run("uses api_key auth method by default", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)
		assert.Equal(t, "api_key", p.authMethod)
	})

	t.Run("accepts entra_id auth method", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			AuthMethod:   "entra_id",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)
		assert.Equal(t, "entra_id", p.authMethod)
	})
}

func TestAzureProvider_Authenticate(t *testing.T) {
	t.Run("uses x-api-key for api_key auth", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			AuthMethod:   "api_key",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err = p.Authenticate(req, "test-key-123")

		require.NoError(t, err)
		assert.Equal(t, "test-key-123", req.Header.Get("x-api-key"))
		assert.Empty(t, req.Header.Get("Authorization"))
	})

	t.Run("uses Bearer token for entra_id auth", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			AuthMethod:   "entra_id",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err = p.Authenticate(req, "entra-token-xyz")

		require.NoError(t, err)
		assert.Equal(t, "Bearer entra-token-xyz", req.Header.Get("Authorization"))
		assert.Empty(t, req.Header.Get("x-api-key"))
	})

	t.Run("default auth method uses x-api-key", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			// AuthMethod not specified, defaults to "api_key"
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err = p.Authenticate(req, "default-key")

		require.NoError(t, err)
		assert.Equal(t, "default-key", req.Header.Get("x-api-key"))
	})
}

func TestAzureProvider_ForwardHeaders(t *testing.T) {
	cfg := &AzureConfig{
		Name:         "test-azure",
		ResourceName: "my-resource",
	}
	p, err := NewAzureProvider(cfg)
	require.NoError(t, err)

	t.Run("adds anthropic-version header if missing", func(t *testing.T) {
		origHeaders := http.Header{}
		headers := p.ForwardHeaders(origHeaders)

		assert.Equal(t, "2023-06-01", headers.Get("Anthropic-Version"))
		assert.Equal(t, "application/json", headers.Get("Content-Type"))
	})

	t.Run("preserves existing anthropic-version", func(t *testing.T) {
		origHeaders := http.Header{}
		origHeaders.Set("Anthropic-Version", "2024-01-01")
		headers := p.ForwardHeaders(origHeaders)

		assert.Equal(t, "2024-01-01", headers.Get("Anthropic-Version"))
	})

	t.Run("forwards other anthropic headers", func(t *testing.T) {
		origHeaders := http.Header{}
		origHeaders.Set("Anthropic-Beta", "tools-2024-04-04")
		headers := p.ForwardHeaders(origHeaders)

		assert.Equal(t, "tools-2024-04-04", headers.Get("Anthropic-Beta"))
	})

	t.Run("sets content-type to application/json", func(t *testing.T) {
		origHeaders := http.Header{}
		headers := p.ForwardHeaders(origHeaders)

		assert.Equal(t, "application/json", headers.Get("Content-Type"))
	})
}

func TestAzureProvider_TransformRequest(t *testing.T) {
	t.Run("constructs correct URL with api-version", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			APIVersion:   "2024-06-01",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)

		body := []byte(`{"model":"claude-sonnet-4-5-20250514","messages":[]}`)

		newBody, targetURL, err := p.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		assert.Equal(t, body, newBody) // Body unchanged
		assert.Equal(t,
			"https://my-resource.services.ai.azure.com/models/chat/completions?api-version=2024-06-01",
			targetURL)
	})

	t.Run("body is unchanged", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)

		body := []byte(`{"model":"claude-sonnet-4-5-20250514","messages":[{"role":"user","content":"Hello"}]}`)

		newBody, _, err := p.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		assert.JSONEq(t, string(body), string(newBody))
	})

	t.Run("uses custom api-version in URL", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "custom-res",
			APIVersion:   "2025-01-15",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)

		body := []byte(`{"model":"test"}`)
		_, targetURL, err := p.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		assert.Contains(t, targetURL, "api-version=2025-01-15")
		assert.Contains(t, targetURL, "custom-res.services.ai.azure.com")
	})

	t.Run("ignores endpoint parameter", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			APIVersion:   "2024-06-01",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)

		body := []byte(`{"model":"test"}`)
		_, targetURL1, _ := p.TransformRequest(body, "/v1/messages")
		_, targetURL2, _ := p.TransformRequest(body, "/different/endpoint")

		// Both should produce the same URL
		assert.Equal(t, targetURL1, targetURL2)
	})
}

func TestAzureProvider_RequiresBodyTransform(t *testing.T) {
	cfg := &AzureConfig{
		Name:         "test-azure",
		ResourceName: "my-resource",
	}
	p, err := NewAzureProvider(cfg)
	require.NoError(t, err)

	assert.False(t, p.RequiresBodyTransform())
}

func TestAzureProvider_SupportsStreaming(t *testing.T) {
	cfg := &AzureConfig{
		Name:         "test-azure",
		ResourceName: "my-resource",
	}
	p, err := NewAzureProvider(cfg)
	require.NoError(t, err)

	assert.True(t, p.SupportsStreaming())
}

func TestAzureProvider_StreamingContentType(t *testing.T) {
	cfg := &AzureConfig{
		Name:         "test-azure",
		ResourceName: "my-resource",
	}
	p, err := NewAzureProvider(cfg)
	require.NoError(t, err)

	assert.Equal(t, "text/event-stream", p.StreamingContentType())
}

func TestAzureProvider_SupportsTransparentAuth(t *testing.T) {
	cfg := &AzureConfig{
		Name:         "test-azure",
		ResourceName: "my-resource",
	}
	p, err := NewAzureProvider(cfg)
	require.NoError(t, err)

	// Azure does NOT support transparent auth (Anthropic tokens not valid)
	assert.False(t, p.SupportsTransparentAuth())
}

func TestAzureProvider_ModelMapping(t *testing.T) {
	cfg := &AzureConfig{
		Name:         "test-azure",
		ResourceName: "my-resource",
		ModelMapping: map[string]string{
			"claude-4": "claude-sonnet-4-5-20250514",
		},
	}
	p, err := NewAzureProvider(cfg)
	require.NoError(t, err)

	t.Run("maps known model", func(t *testing.T) {
		assert.Equal(t, "claude-sonnet-4-5-20250514", p.MapModel("claude-4"))
	})

	t.Run("returns original for unknown model", func(t *testing.T) {
		assert.Equal(t, "unknown-model", p.MapModel("unknown-model"))
	})
}

func TestAzureProvider_GetModelMapping(t *testing.T) {
	t.Run("returns model mapping when configured", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			ModelMapping: map[string]string{
				"alias": "real-model",
			},
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)

		mapping := p.GetModelMapping()
		assert.NotNil(t, mapping)
		assert.Equal(t, "real-model", mapping["alias"])
	})

	t.Run("returns nil when no mapping configured", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)

		mapping := p.GetModelMapping()
		assert.Nil(t, mapping)
	})
}

func TestAzureProvider_ListModels(t *testing.T) {
	t.Run("returns default models with correct metadata", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)

		models := p.ListModels()
		assert.Len(t, models, len(DefaultAzureModels))

		for _, model := range models {
			assert.Equal(t, "model", model.Object)
			assert.Equal(t, AzureOwner, model.OwnedBy)
			assert.Equal(t, "test-azure", model.Provider)
			assert.Greater(t, model.Created, int64(0))
		}
	})

	t.Run("returns custom models when specified", func(t *testing.T) {
		cfg := &AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			Models:       []string{"custom-model-1", "custom-model-2"},
		}
		p, err := NewAzureProvider(cfg)
		require.NoError(t, err)

		models := p.ListModels()
		assert.Len(t, models, 2)
		assert.Equal(t, "custom-model-1", models[0].ID)
		assert.Equal(t, "custom-model-2", models[1].ID)
	})
}

func TestDefaultAzureModels(t *testing.T) {
	// Ensure default models list is reasonable
	assert.NotEmpty(t, DefaultAzureModels)
	assert.Contains(t, DefaultAzureModels, "claude-sonnet-4-5-20250514")
	assert.Contains(t, DefaultAzureModels, "claude-opus-4-5-20250514")
	assert.Contains(t, DefaultAzureModels, "claude-haiku-3-5-20241022")
}

func TestAzureOwner(t *testing.T) {
	assert.Equal(t, "azure", AzureOwner)
}

func TestDefaultAzureAPIVersion(t *testing.T) {
	assert.Equal(t, "2024-06-01", DefaultAzureAPIVersion)
}
