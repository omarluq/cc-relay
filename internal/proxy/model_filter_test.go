package proxy

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
)

// mockProviderForFilter creates a minimal provider for filter testing.
type mockProviderForFilter struct {
	name string
}

func (m *mockProviderForFilter) Name() string                                 { return m.name }
func (m *mockProviderForFilter) BaseURL() string                              { return "" }
func (m *mockProviderForFilter) Owner() string                                { return m.name }
func (m *mockProviderForFilter) Authenticate(_ *http.Request, _ string) error { return nil }
func (m *mockProviderForFilter) ForwardHeaders(_ http.Header) http.Header     { return nil }
func (m *mockProviderForFilter) SupportsStreaming() bool                      { return true }
func (m *mockProviderForFilter) GetModelMapping() map[string]string           { return nil }
func (m *mockProviderForFilter) MapModel(model string) string                 { return model }
func (m *mockProviderForFilter) ListModels() []providers.Model                { return nil }
func (m *mockProviderForFilter) SupportsTransparentAuth() bool                { return false }
func (m *mockProviderForFilter) GetTransparentAuthHeader() string             { return "" }
func (m *mockProviderForFilter) HasValidTransparentAuth(_ *http.Request) bool { return false }

func (m *mockProviderForFilter) TransformRequest(
	body []byte, endpoint string,
) (newBody []byte, targetURL string, err error) {
	return body, endpoint, nil
}
func (m *mockProviderForFilter) TransformResponse(_ *http.Response, _ http.ResponseWriter) error {
	return nil
}
func (m *mockProviderForFilter) RequiresBodyTransform() bool { return false }
func (m *mockProviderForFilter) StreamingContentType() string {
	return providers.ContentTypeSSE
}

func TestFilterProvidersByModel(t *testing.T) {
	t.Parallel()

	anthropic := &mockProviderForFilter{name: "anthropic"}
	zai := &mockProviderForFilter{name: "zai"}
	ollama := &mockProviderForFilter{name: "ollama"}

	providerInfos := []router.ProviderInfo{
		{Provider: anthropic, IsHealthy: func() bool { return true }},
		{Provider: zai, IsHealthy: func() bool { return true }},
		{Provider: ollama, IsHealthy: func() bool { return true }},
	}

	modelMapping := map[string]string{
		"claude-opus":   "anthropic",
		"claude-sonnet": "anthropic",
		"claude-haiku":  "anthropic",
		"glm-4":         "zai",
		"glm-3":         "zai",
		"qwen":          "ollama",
		"llama":         "ollama",
	}

	tests := []struct {
		name            string
		model           string
		defaultProvider string
		expectedNames   []string
	}{
		{
			name:          "claude model routes to anthropic",
			model:         "claude-opus-4",
			expectedNames: []string{"anthropic"},
		},
		{
			name:          "claude-sonnet routes to anthropic",
			model:         "claude-sonnet-4-20250514",
			expectedNames: []string{"anthropic"},
		},
		{
			name:          "glm model routes to zai",
			model:         "glm-4.7",
			expectedNames: []string{"zai"},
		},
		{
			name:          "qwen routes to ollama",
			model:         "qwen3:8b",
			expectedNames: []string{"ollama"},
		},
		{
			name:          "llama routes to ollama",
			model:         "llama-3.3-70b",
			expectedNames: []string{"ollama"},
		},
		{
			name:            "unknown model uses default",
			model:           "unknown-model",
			defaultProvider: "anthropic",
			expectedNames:   []string{"anthropic"},
		},
		{
			name:          "unknown model without default returns all",
			model:         "unknown-model",
			expectedNames: []string{"anthropic", "zai", "ollama"},
		},
		{
			name:          "empty model returns all",
			model:         "",
			expectedNames: []string{"anthropic", "zai", "ollama"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := FilterProvidersByModel(tt.model, providerInfos, modelMapping, tt.defaultProvider)

			resultNames := make([]string, 0, len(result))
			for _, p := range result {
				resultNames = append(resultNames, p.Provider.Name())
			}

			assert.ElementsMatch(t, tt.expectedNames, resultNames)
		})
	}
}

func TestFilterProvidersByModel_EmptyMapping(t *testing.T) {
	t.Parallel()

	anthropic := &mockProviderForFilter{name: "anthropic"}
	providerInfos := []router.ProviderInfo{
		{Provider: anthropic},
	}

	// Empty mapping should return all providers
	result := FilterProvidersByModel("claude-opus-4", providerInfos, nil, "")
	assert.Len(t, result, 1)
	assert.Equal(t, "anthropic", result[0].Provider.Name())

	result = FilterProvidersByModel("claude-opus-4", providerInfos, map[string]string{}, "")
	assert.Len(t, result, 1)
}

func TestFilterProvidersByModel_LongestPrefixMatch(t *testing.T) {
	t.Parallel()

	anthropic := &mockProviderForFilter{name: "anthropic"}
	anthropicSpecial := &mockProviderForFilter{name: "anthropic-special"}

	providerInfos := []router.ProviderInfo{
		{Provider: anthropic},
		{Provider: anthropicSpecial},
	}

	// "claude-opus-special" should match "claude-opus-special" (longer) not "claude-opus"
	modelMapping := map[string]string{
		"claude-opus":         "anthropic",
		"claude-opus-special": "anthropic-special",
	}

	result := FilterProvidersByModel("claude-opus-special-4", providerInfos, modelMapping, "")
	assert.Len(t, result, 1)
	assert.Equal(t, "anthropic-special", result[0].Provider.Name())

	// "claude-opus-4" should match "claude-opus" (only match)
	result = FilterProvidersByModel("claude-opus-4", providerInfos, modelMapping, "")
	assert.Len(t, result, 1)
	assert.Equal(t, "anthropic", result[0].Provider.Name())
}

func TestFilterProvidersByModel_GracefulDegradation(t *testing.T) {
	t.Parallel()

	anthropic := &mockProviderForFilter{name: "anthropic"}
	providerInfos := []router.ProviderInfo{
		{Provider: anthropic},
	}

	// Model maps to non-existent provider - should return all providers
	modelMapping := map[string]string{
		"claude-opus": "nonexistent",
	}

	result := FilterProvidersByModel("claude-opus-4", providerInfos, modelMapping, "")
	assert.Len(t, result, 1)
	assert.Equal(t, "anthropic", result[0].Provider.Name())
}

func TestFindProviderForModel(t *testing.T) {
	t.Parallel()

	modelMapping := map[string]string{
		"claude":      "anthropic",
		"claude-opus": "anthropic-premium",
		"glm":         "zai",
	}

	tests := []struct {
		name      string
		model     string
		expected  string
		isPresent bool
	}{
		{"exact match", "claude", "anthropic", true},
		{"prefix match", "claude-sonnet-4", "anthropic", true},
		{"longer prefix wins", "claude-opus-4", "anthropic-premium", true},
		{"no match", "gpt-4", "", false},
		{"empty model", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := findProviderForModel(tt.model, modelMapping)
			assert.Equal(t, tt.isPresent, result.IsPresent())
			assert.Equal(t, tt.expected, result.OrEmpty())
		})
	}
}
