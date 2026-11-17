package providers_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/providers"
)

func TestNewMoonshotProvider(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name   string
		url    string
		models []string
	}{
		{name: "defaults", url: "", models: nil},
		{name: "overrides", url: "https://moonshot.example/anthropic", models: []string{"custom-kimi"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			mapping := map[string]string{"claude-opus-4-6": "kimi-k3.1-pro"}
			provider := providers.NewMoonshotProvider("kimi", testCase.url, testCase.models, mapping)
			wantURL, wantModels := testCase.url, testCase.models
			if wantURL == "" {
				wantURL = providers.DefaultMoonshotBaseURL
				wantModels = providers.DefaultMoonshotModels
			}
			assert.Equal(t, "kimi", provider.Name())
			assert.Equal(t, wantURL, provider.BaseURL())
			assert.Equal(t, "kimi-k3.1-pro", provider.MapModel("claude-opus-4-6"))
			assert.Equal(t, "unmapped", provider.MapModel("unmapped"))
			assert.Equal(t, mapping, provider.GetModelMapping())
			assert.True(t, provider.SupportsStreaming())
			assert.False(t, provider.SupportsTransparentAuth())
			assert.False(t, provider.RequiresBodyTransform())

			models := provider.ListModels()
			require.Len(t, models, len(wantModels))
			for index, model := range models {
				assert.Equal(t, wantModels[index], model.ID)
				assert.Equal(t, providers.MoonshotOwner, model.OwnedBy)
			}
		})
	}
}

func TestMoonshotAuthenticateReplacesClientCredentials(t *testing.T) {
	t.Parallel()

	provider := providers.NewMoonshotProvider("kimi", "", nil, nil)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		providers.DefaultMoonshotBaseURL+"/v1/messages", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer client-token")
	req.Header.Set("x-api-key", "client-key")

	require.NoError(t, provider.Authenticate(req, "moonshot-key"))
	assert.Equal(t, "Bearer moonshot-key", req.Header.Get("Authorization"))
	assert.Empty(t, req.Header.Get("x-api-key"))
}

func TestMoonshotTransformRequestPassesThrough(t *testing.T) {
	t.Parallel()

	provider := providers.NewMoonshotProvider("kimi", "", nil, nil)
	body := []byte(`{"model":"kimi-k3.1-pro","messages":[],"stream":true}`)
	gotBody, target, err := provider.TransformRequest(body, "/v1/messages")
	require.NoError(t, err)
	assert.Equal(t, body, gotBody)
	assert.Equal(t, "https://api.moonshot.ai/anthropic/v1/messages", target)
}
