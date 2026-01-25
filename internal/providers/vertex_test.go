package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	}
}

func TestNewVertexProviderWithTokenSource(t *testing.T) {
	t.Run("creates provider with required config", func(t *testing.T) {
		cfg := &VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-project",
			Region:    "us-central1",
		}
		ts := newMockTokenSource("test-token")
		p := NewVertexProviderWithTokenSource(cfg, ts)

		assert.Equal(t, "test-vertex", p.Name())
		assert.Equal(t, "https://us-central1-aiplatform.googleapis.com", p.BaseURL())
		assert.Equal(t, VertexOwner, p.Owner())
		assert.Equal(t, "my-project", p.GetProjectID())
		assert.Equal(t, "us-central1", p.GetRegion())
	})

	t.Run("uses default models when none specified", func(t *testing.T) {
		cfg := &VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-project",
			Region:    "us-central1",
		}
		ts := newMockTokenSource("test-token")
		p := NewVertexProviderWithTokenSource(cfg, ts)

		models := p.ListModels()
		assert.Len(t, models, len(DefaultVertexModels))
	})

	t.Run("uses custom models when specified", func(t *testing.T) {
		cfg := &VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-project",
			Region:    "us-central1",
			Models:    []string{"custom-model"},
		}
		ts := newMockTokenSource("test-token")
		p := NewVertexProviderWithTokenSource(cfg, ts)

		models := p.ListModels()
		assert.Len(t, models, 1)
		assert.Equal(t, "custom-model", models[0].ID)
	})

	t.Run("constructs correct base URL for different regions", func(t *testing.T) {
		regions := []struct {
			region      string
			expectedURL string
		}{
			{"us-central1", "https://us-central1-aiplatform.googleapis.com"},
			{"europe-west4", "https://europe-west4-aiplatform.googleapis.com"},
			{"asia-northeast1", "https://asia-northeast1-aiplatform.googleapis.com"},
		}

		for _, tc := range regions {
			t.Run(tc.region, func(t *testing.T) {
				cfg := &VertexConfig{
					Name:      "test-vertex",
					ProjectID: "my-project",
					Region:    tc.region,
				}
				ts := newMockTokenSource("test-token")
				p := NewVertexProviderWithTokenSource(cfg, ts)

				assert.Equal(t, tc.expectedURL, p.BaseURL())
			})
		}
	})
}

func TestVertexProvider_Authenticate(t *testing.T) {
	t.Run("adds Bearer token from TokenSource", func(t *testing.T) {
		cfg := &VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-project",
			Region:    "us-central1",
		}
		ts := newMockTokenSource("gcp-oauth-token-xyz")
		p := NewVertexProviderWithTokenSource(cfg, ts)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err := p.Authenticate(req, "") // key param ignored

		require.NoError(t, err)
		assert.Equal(t, "Bearer gcp-oauth-token-xyz", req.Header.Get("Authorization"))
	})

	t.Run("ignores API key parameter", func(t *testing.T) {
		cfg := &VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-project",
			Region:    "us-central1",
		}
		ts := newMockTokenSource("oauth-token")
		p := NewVertexProviderWithTokenSource(cfg, ts)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err := p.Authenticate(req, "ignored-api-key")

		require.NoError(t, err)
		// Should use OAuth token, not the API key
		assert.Equal(t, "Bearer oauth-token", req.Header.Get("Authorization"))
		assert.Empty(t, req.Header.Get("x-api-key"))
	})

	t.Run("returns error when token source fails", func(t *testing.T) {
		cfg := &VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-project",
			Region:    "us-central1",
		}
		ts := &mockTokenSource{err: errors.New("token refresh failed")}
		p := NewVertexProviderWithTokenSource(cfg, ts)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err := p.Authenticate(req, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get token")
	})

	t.Run("returns error when no token source configured", func(t *testing.T) {
		cfg := &VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-project",
			Region:    "us-central1",
		}
		p := NewVertexProviderWithTokenSource(cfg, nil)

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		err := p.Authenticate(req, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no token source")
	})
}

func TestVertexProvider_ForwardHeaders(t *testing.T) {
	cfg := &VertexConfig{
		Name:      "test-vertex",
		ProjectID: "my-project",
		Region:    "us-central1",
	}
	ts := newMockTokenSource("test-token")
	p := NewVertexProviderWithTokenSource(cfg, ts)

	t.Run("removes anthropic-version header", func(t *testing.T) {
		origHeaders := http.Header{}
		origHeaders.Set("Anthropic-Version", "2023-06-01")
		headers := p.ForwardHeaders(origHeaders)

		// anthropic_version goes in body for Vertex, not header
		assert.Empty(t, headers.Get("Anthropic-Version"))
	})

	t.Run("preserves other anthropic headers", func(t *testing.T) {
		origHeaders := http.Header{}
		origHeaders.Set("Anthropic-Beta", "tools-2024-04-04")
		headers := p.ForwardHeaders(origHeaders)

		assert.Equal(t, "tools-2024-04-04", headers.Get("Anthropic-Beta"))
	})

	t.Run("sets Content-Type", func(t *testing.T) {
		origHeaders := http.Header{}
		headers := p.ForwardHeaders(origHeaders)

		assert.Equal(t, "application/json", headers.Get("Content-Type"))
	})
}

func TestVertexProvider_TransformRequest(t *testing.T) {
	cfg := &VertexConfig{
		Name:      "test-vertex",
		ProjectID: "my-project",
		Region:    "us-central1",
	}
	ts := newMockTokenSource("test-token")
	p := NewVertexProviderWithTokenSource(cfg, ts)

	t.Run("removes model from body and adds anthropic_version", func(t *testing.T) {
		body := []byte(`{"model":"claude-sonnet-4-5@20250514","messages":[{"role":"user","content":"Hello"}]}`)

		newBody, _, err := p.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)

		// Parse result to verify
		var result map[string]interface{}
		err = json.Unmarshal(newBody, &result)
		require.NoError(t, err)

		// Model should be removed
		_, hasModel := result["model"]
		assert.False(t, hasModel, "model should be removed from body")

		// anthropic_version should be added
		assert.Equal(t, VertexAnthropicVersion, result["anthropic_version"])

		// messages should be preserved
		assert.NotNil(t, result["messages"])
	})

	t.Run("constructs correct streaming URL with model in path", func(t *testing.T) {
		// stream: true in request body triggers streamRawPredict endpoint
		body := []byte(`{"model":"claude-sonnet-4-5@20250514","messages":[],"stream":true}`)

		_, targetURL, err := p.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		// Note: @ is valid in URL paths, so url.PathEscape doesn't escape it
		expected := "https://us-central1-aiplatform.googleapis.com" +
			"/v1/projects/my-project/locations/us-central1" +
			"/publishers/anthropic/models/claude-sonnet-4-5@20250514:streamRawPredict"
		assert.Equal(t, expected, targetURL)
	})

	t.Run("constructs correct non-streaming URL with model in path", func(t *testing.T) {
		// stream: false or missing triggers rawPredict endpoint
		body := []byte(`{"model":"claude-sonnet-4-5@20250514","messages":[]}`)

		_, targetURL, err := p.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		expected := "https://us-central1-aiplatform.googleapis.com" +
			"/v1/projects/my-project/locations/us-central1" +
			"/publishers/anthropic/models/claude-sonnet-4-5@20250514:rawPredict"
		assert.Equal(t, expected, targetURL)
	})

	t.Run("applies model mapping", func(t *testing.T) {
		cfgWithMapping := &VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-project",
			Region:    "us-central1",
			ModelMapping: map[string]string{
				"claude-4": "claude-sonnet-4-5@20250514",
			},
		}
		pWithMapping := NewVertexProviderWithTokenSource(cfgWithMapping, ts)

		body := []byte(`{"model":"claude-4","messages":[]}`)
		_, targetURL, err := pWithMapping.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		assert.Contains(t, targetURL, "claude-sonnet-4-5@20250514")
	})

	t.Run("handles special characters in project ID", func(t *testing.T) {
		cfgSpecial := &VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-project-123",
			Region:    "us-central1",
		}
		pSpecial := NewVertexProviderWithTokenSource(cfgSpecial, ts)

		body := []byte(`{"model":"claude-sonnet-4-5@20250514","messages":[]}`)
		_, targetURL, err := pSpecial.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		assert.Contains(t, targetURL, "my-project-123")
	})

	t.Run("preserves all other request body fields", func(t *testing.T) {
		body := []byte(`{
			"model": "claude-sonnet-4-5@20250514",
			"messages": [{"role": "user", "content": "Hello"}],
			"max_tokens": 1024,
			"temperature": 0.7,
			"stream": true,
			"system": "You are helpful"
		}`)

		newBody, _, err := p.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(newBody, &result)
		require.NoError(t, err)

		// All fields should be preserved except model
		assert.Equal(t, float64(1024), result["max_tokens"])
		assert.Equal(t, 0.7, result["temperature"])
		assert.Equal(t, true, result["stream"])
		assert.Equal(t, "You are helpful", result["system"])
		assert.NotNil(t, result["messages"])
		assert.Equal(t, VertexAnthropicVersion, result["anthropic_version"])
	})
}

func TestVertexProvider_RequiresBodyTransform(t *testing.T) {
	cfg := &VertexConfig{
		Name:      "test-vertex",
		ProjectID: "my-project",
		Region:    "us-central1",
	}
	ts := newMockTokenSource("test-token")
	p := NewVertexProviderWithTokenSource(cfg, ts)

	assert.True(t, p.RequiresBodyTransform())
}

func TestVertexProvider_SupportsStreaming(t *testing.T) {
	cfg := &VertexConfig{
		Name:      "test-vertex",
		ProjectID: "my-project",
		Region:    "us-central1",
	}
	ts := newMockTokenSource("test-token")
	p := NewVertexProviderWithTokenSource(cfg, ts)

	assert.True(t, p.SupportsStreaming())
}

func TestVertexProvider_StreamingContentType(t *testing.T) {
	cfg := &VertexConfig{
		Name:      "test-vertex",
		ProjectID: "my-project",
		Region:    "us-central1",
	}
	ts := newMockTokenSource("test-token")
	p := NewVertexProviderWithTokenSource(cfg, ts)

	// Vertex uses standard SSE (unlike Bedrock)
	assert.Equal(t, "text/event-stream", p.StreamingContentType())
}

func TestVertexProvider_RefreshToken(t *testing.T) {
	t.Run("successfully refreshes token", func(t *testing.T) {
		cfg := &VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-project",
			Region:    "us-central1",
		}
		ts := newMockTokenSource("refreshed-token")
		p := NewVertexProviderWithTokenSource(cfg, ts)

		err := p.RefreshToken(context.Background())
		require.NoError(t, err)
	})

	t.Run("returns error when no token source", func(t *testing.T) {
		cfg := &VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-project",
			Region:    "us-central1",
		}
		p := NewVertexProviderWithTokenSource(cfg, nil)

		err := p.RefreshToken(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no token source")
	})

	t.Run("returns error when refresh fails", func(t *testing.T) {
		cfg := &VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-project",
			Region:    "us-central1",
		}
		ts := &mockTokenSource{err: errors.New("network error")}
		p := NewVertexProviderWithTokenSource(cfg, ts)

		err := p.RefreshToken(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "refresh failed")
	})
}

func TestVertexProvider_ModelMapping(t *testing.T) {
	cfg := &VertexConfig{
		Name:      "test-vertex",
		ProjectID: "my-project",
		Region:    "us-central1",
		ModelMapping: map[string]string{
			"claude-4":    "claude-sonnet-4-5@20250514",
			"claude-opus": "claude-opus-4-5@20250514",
		},
	}
	ts := newMockTokenSource("test-token")
	p := NewVertexProviderWithTokenSource(cfg, ts)

	t.Run("maps known model", func(t *testing.T) {
		assert.Equal(t, "claude-sonnet-4-5@20250514", p.MapModel("claude-4"))
	})

	t.Run("returns original for unknown model", func(t *testing.T) {
		assert.Equal(t, "unknown-model", p.MapModel("unknown-model"))
	})
}

func TestVertexProvider_InterfaceCompliance(_ *testing.T) {
	// Compile-time check that VertexProvider implements Provider
	var _ Provider = (*VertexProvider)(nil)
}

func TestVertexProvider_Owner(t *testing.T) {
	cfg := &VertexConfig{
		Name:      "test-vertex",
		ProjectID: "my-project",
		Region:    "us-central1",
	}
	ts := newMockTokenSource("test-token")
	p := NewVertexProviderWithTokenSource(cfg, ts)

	assert.Equal(t, "google", p.Owner())
}

func TestVertexProvider_SupportsTransparentAuth(t *testing.T) {
	cfg := &VertexConfig{
		Name:      "test-vertex",
		ProjectID: "my-project",
		Region:    "us-central1",
	}
	ts := newMockTokenSource("test-token")
	p := NewVertexProviderWithTokenSource(cfg, ts)

	// Vertex uses OAuth, not client API keys
	assert.False(t, p.SupportsTransparentAuth())
}
