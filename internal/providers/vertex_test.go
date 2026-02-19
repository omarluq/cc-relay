package providers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// mockTokenSource provides a controllable token source for testing.
type mockTokenSource struct {
	token *oauth2.Token
	err   error
}

func (m *mockTokenSource) Token() (*oauth2.Token, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.token, nil
}

func newMockTokenSource(accessToken string) *mockTokenSource {
	return &mockTokenSource{
		token: &oauth2.Token{
			AccessToken: accessToken,
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(1 * time.Hour),
		},
		err: nil,
	}
}

// newTestVertexConfig creates a default VertexConfig for testing.
func newTestVertexConfig() *providers.VertexConfig {
	return &providers.VertexConfig{
		ModelMapping: nil,
		Name:         "test-vertex",
		ProjectID:    "my-project",
		Region:       "us-central1",
		Models:       nil,
	}
}

func TestNewVertexProviderWithTokenSource(t *testing.T) {
	t.Parallel()

	t.Run("creates provider with required config", func(t *testing.T) {
		t.Parallel()

		cfg := newTestVertexConfig()
		tokenSource := newMockTokenSource("test-token")
		provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

		assert.Equal(t, "test-vertex", provider.Name())
		assert.Equal(t, "https://us-central1-aiplatform.googleapis.com", provider.BaseURL())
		assert.Equal(t, providers.VertexOwner, provider.Owner())
		assert.Equal(t, "my-project", provider.GetProjectID())
		assert.Equal(t, "us-central1", provider.GetRegion())
	})

	t.Run("uses default models when none specified", func(t *testing.T) {
		t.Parallel()

		cfg := newTestVertexConfig()
		tokenSource := newMockTokenSource("test-token")
		provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

		models := provider.ListModels()
		assert.Len(t, models, len(providers.DefaultVertexModels))
	})

	t.Run("uses custom models when specified", func(t *testing.T) {
		t.Parallel()

		cfg := newTestVertexConfig()
		cfg.Models = []string{"custom-model"}
		tokenSource := newMockTokenSource("test-token")
		provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

		models := provider.ListModels()
		assert.Len(t, models, 1)
		assert.Equal(t, "custom-model", models[0].ID)
	})
}

func TestNewVertexProviderRegionURLs(t *testing.T) {
	t.Parallel()

	regions := []struct {
		region      string
		expectedURL string
	}{
		{"us-central1", "https://us-central1-aiplatform.googleapis.com"},
		{"europe-west4", "https://europe-west4-aiplatform.googleapis.com"},
		{"asia-northeast1", "https://asia-northeast1-aiplatform.googleapis.com"},
	}

	for _, testCase := range regions {
		t.Run(testCase.region, func(t *testing.T) {
			t.Parallel()

			cfg := newTestVertexConfig()
			cfg.Region = testCase.region
			tokenSource := newMockTokenSource("test-token")
			provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

			assert.Equal(t, testCase.expectedURL, provider.BaseURL())
		})
	}
}

func TestVertexProviderAuthenticateSuccess(t *testing.T) {
	t.Parallel()

	t.Run("adds Bearer token from TokenSource", func(t *testing.T) {
		t.Parallel()

		cfg := newTestVertexConfig()
		tokenSource := newMockTokenSource("gcp-oauth-token-xyz")
		provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err := provider.Authenticate(req, "") // key param ignored

		require.NoError(t, err)
		assert.Equal(t, "Bearer gcp-oauth-token-xyz", req.Header.Get("Authorization"))
	})

	t.Run("ignores API key parameter", func(t *testing.T) {
		t.Parallel()

		cfg := newTestVertexConfig()
		tokenSource := newMockTokenSource("oauth-token")
		provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err := provider.Authenticate(req, "ignored-api-key")

		require.NoError(t, err)
		// Should use OAuth token, not the API key
		assert.Equal(t, "Bearer oauth-token", req.Header.Get("Authorization"))
		assert.Empty(t, req.Header.Get("x-api-key"))
	})
}

func TestVertexProviderAuthenticateErrors(t *testing.T) {
	t.Parallel()

	t.Run("returns error when token source fails", func(t *testing.T) {
		t.Parallel()

		cfg := newTestVertexConfig()
		tokenSource := &mockTokenSource{
			token: nil,
			err:   errors.New("token refresh failed"),
		}
		provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err := provider.Authenticate(req, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get token")
	})

	t.Run("returns error when no token source configured", func(t *testing.T) {
		t.Parallel()

		cfg := newTestVertexConfig()
		provider := providers.NewVertexProviderWithTokenSource(cfg, nil)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err := provider.Authenticate(req, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no token source")
	})
}

func TestVertexProviderForwardHeaders(t *testing.T) {
	t.Parallel()

	cfg := newTestVertexConfig()
	tokenSource := newMockTokenSource("test-token")
	provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

	t.Run("removes anthropic-version header", func(t *testing.T) {
		t.Parallel()

		origHeaders := http.Header{}
		origHeaders.Set("Anthropic-Version", "2023-06-01")
		headers := provider.ForwardHeaders(origHeaders)

		// anthropic_version goes in body for Vertex, not header
		assert.Empty(t, headers.Get("Anthropic-Version"))
	})

	t.Run("preserves other anthropic headers", func(t *testing.T) {
		t.Parallel()

		origHeaders := http.Header{}
		origHeaders.Set("Anthropic-Beta", "tools-2024-04-04")
		headers := provider.ForwardHeaders(origHeaders)

		assert.Equal(t, "tools-2024-04-04", headers.Get("Anthropic-Beta"))
	})

	t.Run("sets Content-Type", func(t *testing.T) {
		t.Parallel()

		origHeaders := http.Header{}
		headers := provider.ForwardHeaders(origHeaders)

		assert.Equal(t, "application/json", headers.Get("Content-Type"))
	})
}

func TestVertexTransformRequestBasicFields(t *testing.T) {
	t.Parallel()

	cfg := newTestVertexConfig()
	tokenSource := newMockTokenSource("test-token")
	provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

	t.Run("removes model from body and adds anthropic_version", func(t *testing.T) {
		t.Parallel()

		body := []byte(`{"model":"claude-sonnet-4-5@20250514","messages":[{"role":"user","content":"Hello"}]}`)

		newBody, _, err := provider.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)

		// Parse result to verify
		var result map[string]interface{}
		err = json.Unmarshal(newBody, &result)
		require.NoError(t, err)

		// Model should be removed
		_, hasModel := result["model"]
		assert.False(t, hasModel, "model should be removed from body")

		// anthropic_version should be added
		assert.Equal(t, providers.VertexAnthropicVersion, result["anthropic_version"])

		// messages should be preserved
		assert.NotNil(t, result["messages"])
	})

	t.Run("preserves all other request body fields", func(t *testing.T) {
		t.Parallel()

		body := []byte(`{
			"model": "claude-sonnet-4-5@20250514",
			"messages": [{"role": "user", "content": "Hello"}],
			"max_tokens": 1024,
			"temperature": 0.7,
			"stream": true,
			"system": "You are helpful"
		}`)

		newBody, _, err := provider.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		providers.AssertPreservesBodyFields(t, newBody, providers.VertexAnthropicVersion)
	})
}

func TestVertexTransformRequestURLConstruction(t *testing.T) {
	t.Parallel()

	cfg := newTestVertexConfig()
	tokenSource := newMockTokenSource("test-token")
	provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

	t.Run("constructs correct streaming URL with model in path", func(t *testing.T) {
		t.Parallel()
		// stream: true in request body triggers streamRawPredict endpoint

		body := []byte(`{"model":"claude-sonnet-4-5@20250514","messages":[],"stream":true}`)

		_, targetURL, err := provider.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		// Note: @ is valid in URL paths, so url.PathEscape doesn't escape it
		expected := "https://us-central1-aiplatform.googleapis.com" +
			"/v1/projects/my-project/locations/us-central1" +
			"/publishers/anthropic/models/claude-sonnet-4-5@20250514:streamRawPredict"
		assert.Equal(t, expected, targetURL)
	})

	t.Run("constructs correct non-streaming URL with model in path", func(t *testing.T) {
		t.Parallel()
		// stream: false or missing triggers rawPredict endpoint

		body := []byte(`{"model":"claude-sonnet-4-5@20250514","messages":[]}`)

		_, targetURL, err := provider.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		expected := "https://us-central1-aiplatform.googleapis.com" +
			"/v1/projects/my-project/locations/us-central1" +
			"/publishers/anthropic/models/claude-sonnet-4-5@20250514:rawPredict"
		assert.Equal(t, expected, targetURL)
	})

	t.Run("handles special characters in project ID", func(t *testing.T) {
		t.Parallel()

		cfgSpecial := &providers.VertexConfig{
			ModelMapping: nil,
			Name:         "test-vertex",
			ProjectID:    "my-project-123",
			Region:       "us-central1",
			Models:       nil,
		}
		providerSpecial := providers.NewVertexProviderWithTokenSource(cfgSpecial, tokenSource)

		body := []byte(`{"model":"claude-sonnet-4-5@20250514","messages":[]}`)
		_, targetURL, err := providerSpecial.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		assert.Contains(t, targetURL, "my-project-123")
	})
}

func TestVertexTransformRequestModelMapping(t *testing.T) {
	t.Parallel()

	tokenSource := newMockTokenSource("test-token")

	t.Run("applies model mapping", func(t *testing.T) {
		t.Parallel()

		cfgWithMapping := &providers.VertexConfig{
			ModelMapping: map[string]string{
				"claude-4": "claude-sonnet-4-5@20250514",
			},
			Name:      "test-vertex",
			ProjectID: "my-project",
			Region:    "us-central1",
			Models:    nil,
		}
		providerWithMapping := providers.NewVertexProviderWithTokenSource(cfgWithMapping, tokenSource)

		body := []byte(`{"model":"claude-4","messages":[]}`)
		_, targetURL, err := providerWithMapping.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		assert.Contains(t, targetURL, "claude-sonnet-4-5@20250514")
	})
}

func TestVertexProviderRequiresBodyTransform(t *testing.T) {
	t.Parallel()

	cfg := newTestVertexConfig()
	tokenSource := newMockTokenSource("test-token")
	provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

	assert.True(t, provider.RequiresBodyTransform())
}

func TestVertexProviderSupportsStreaming(t *testing.T) {
	t.Parallel()

	cfg := newTestVertexConfig()
	tokenSource := newMockTokenSource("test-token")
	provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

	assert.True(t, provider.SupportsStreaming())
}

func TestVertexProviderStreamingContentType(t *testing.T) {
	t.Parallel()

	cfg := newTestVertexConfig()
	tokenSource := newMockTokenSource("test-token")
	provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

	// Vertex uses standard SSE (unlike Bedrock)
	assert.Equal(t, "text/event-stream", provider.StreamingContentType())
}

func TestVertexProviderRefreshToken(t *testing.T) {
	t.Parallel()

	t.Run("successfully refreshes token", func(t *testing.T) {
		t.Parallel()

		cfg := newTestVertexConfig()
		tokenSource := newMockTokenSource("refreshed-token")
		provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

		err := provider.RefreshToken(context.Background())
		require.NoError(t, err)
	})

	t.Run("returns error when no token source", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.VertexConfig{
			ModelMapping: nil,
			Name:         "test-vertex",
			ProjectID:    "my-project",
			Region:       "us-central1",
			Models:       nil,
		}
		provider := providers.NewVertexProviderWithTokenSource(cfg, nil)

		err := provider.RefreshToken(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no token source")
	})

	t.Run("returns error when refresh fails", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.VertexConfig{
			ModelMapping: nil,
			Name:         "test-vertex",
			ProjectID:    "my-project",
			Region:       "us-central1",
			Models:       nil,
		}
		tokenSource := &mockTokenSource{
			token: nil,
			err:   errors.New("network error"),
		}
		provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

		err := provider.RefreshToken(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "refresh failed")
	})
}

func TestVertexProviderModelMapping(t *testing.T) {
	t.Parallel()

	cfg := &providers.VertexConfig{
		ModelMapping: map[string]string{
			"claude-4":    "claude-sonnet-4-5@20250514",
			"claude-opus": "claude-opus-4-5@20250514",
		},
		Name:      "test-vertex",
		ProjectID: "my-project",
		Region:    "us-central1",
		Models:    nil,
	}
	tokenSource := newMockTokenSource("test-token")
	provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

	t.Run("maps known model", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "claude-sonnet-4-5@20250514", provider.MapModel("claude-4"))
	})

	t.Run("returns original for unknown model", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "unknown-model", provider.MapModel("unknown-model"))
	})
}

func TestVertexProviderInterfaceCompliance(t *testing.T) {
	t.Parallel()
	// Compile-time check that VertexProvider implements Provider
	var _ providers.Provider = (*providers.VertexProvider)(nil)
}

func TestVertexProviderOwner(t *testing.T) {
	t.Parallel()

	cfg := newTestVertexConfig()
	tokenSource := newMockTokenSource("test-token")
	provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

	assert.Equal(t, "google", provider.Owner())
}

func TestVertexProviderSupportsTransparentAuth(t *testing.T) {
	t.Parallel()

	cfg := newTestVertexConfig()
	tokenSource := newMockTokenSource("test-token")
	provider := providers.NewVertexProviderWithTokenSource(cfg, tokenSource)

	// Vertex uses OAuth, not client API keys
	assert.False(t, provider.SupportsTransparentAuth())
}
