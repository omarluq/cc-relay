package proxy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/omarluq/cc-relay/internal/proxy"
	"github.com/omarluq/cc-relay/internal/router"
)

// filterTestProviders returns test providers and model mapping for filter tests.
func filterTestProviders() (infos []router.ProviderInfo, mapping map[string]string) {
	anthropic := proxy.NewMockProvider(testProviderAnthropic)
	zai := proxy.NewMockProvider(testProviderZAI)
	ollama := proxy.NewMockProvider(testProviderOllama)

	infos = []router.ProviderInfo{
		proxy.TestProviderInfoWithHealth(anthropic, func() bool { return true }),
		proxy.TestProviderInfoWithHealth(zai, func() bool { return true }),
		proxy.TestProviderInfoWithHealth(ollama, func() bool { return true }),
	}

	mapping = map[string]string{
		testModelPrefixClaudeOpus: testProviderAnthropic,
		"claude-sonnet":           testProviderAnthropic,
		"claude-haiku":            testProviderAnthropic,
		testModelGLM4:             testProviderZAI,
		"glm-3":                   testProviderZAI,
		"qwen":                    testProviderOllama,
		"llama":                   testProviderOllama,
	}
	return
}

func TestFilterProvidersByModel(t *testing.T) {
	t.Parallel()

	providerInfos, modelMapping := filterTestProviders()

	tests := []struct {
		name            string
		model           string
		defaultProvider string
		expectedNames   []string
	}{
		{
			name:            "claude model routes to anthropic",
			model:           testModelClaudeOpus4,
			defaultProvider: "",
			expectedNames:   []string{testProviderAnthropic},
		},
		{
			name:            "claude-sonnet routes to anthropic",
			model:           testModelClaudeSonnet4,
			defaultProvider: "",
			expectedNames:   []string{testProviderAnthropic},
		},
		{
			name:            "glm model routes to zai",
			model:           "glm-4.7",
			defaultProvider: "",
			expectedNames:   []string{testProviderZAI},
		},
		{
			name:            "qwen routes to ollama",
			model:           testModelQwen3_8B,
			defaultProvider: "",
			expectedNames:   []string{testProviderOllama},
		},
		{
			name:            "llama routes to ollama",
			model:           "llama-3.3-70b",
			defaultProvider: "",
			expectedNames:   []string{testProviderOllama},
		},
		{
			name:            "unknown model uses default",
			model:           testModelUnknown,
			defaultProvider: testProviderAnthropic,
			expectedNames:   []string{testProviderAnthropic},
		},
		{
			name:            "unknown model without default returns all",
			model:           testModelUnknown,
			defaultProvider: "",
			expectedNames:   []string{testProviderAnthropic, testProviderZAI, testProviderOllama},
		},
		{
			name:            "empty model returns all",
			model:           "",
			defaultProvider: "",
			expectedNames:   []string{testProviderAnthropic, testProviderZAI, testProviderOllama},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			RunFilterTestCase(t, testCase, providerInfos, modelMapping)
		})
	}
}

// runFilterTestCase executes a single filter test case.
// This helper reduces the line count of TestFilterProvidersByModel.
func RunFilterTestCase(t *testing.T, testCase struct {
	name            string
	model           string
	defaultProvider string
	expectedNames   []string
}, providerInfos []router.ProviderInfo, modelMapping map[string]string) {
	t.Helper()
	result := proxy.FilterProvidersByModel(testCase.model, providerInfos, modelMapping, testCase.defaultProvider)

	resultNames := make([]string, 0, len(result))
	for _, p := range result {
		resultNames = append(resultNames, p.Provider.Name())
	}

	assert.ElementsMatch(t, testCase.expectedNames, resultNames)
}

func TestFilterProvidersByModelEmptyMapping(t *testing.T) {
	t.Parallel()

	anthropic := proxy.NewMockProvider(testProviderAnthropic)
	providerInfos := []router.ProviderInfo{
		proxy.TestProviderInfo(anthropic),
	}

	// Empty mapping should return all providers
	result := proxy.FilterProvidersByModel(testModelClaudeOpus4, providerInfos, nil, "")
	assert.Len(t, result, 1)
	assert.Equal(t, testProviderAnthropic, result[0].Provider.Name())

	result = proxy.FilterProvidersByModel(testModelClaudeOpus4, providerInfos, map[string]string{}, "")
	assert.Len(t, result, 1)
}

func TestFilterProvidersByModelLongestPrefixMatch(t *testing.T) {
	t.Parallel()

	anthropic := proxy.NewMockProvider(testProviderAnthropic)
	anthropicSpecial := proxy.NewMockProvider("anthropic-special")

	providerInfos := []router.ProviderInfo{
		proxy.TestProviderInfo(anthropic),
		proxy.TestProviderInfo(anthropicSpecial),
	}

	// "claude-opus-special" should match "claude-opus-special" (longer) not testModelPrefixClaudeOpus
	modelMapping := map[string]string{
		testModelPrefixClaudeOpus:         testProviderAnthropic,
		"claude-opus-special": "anthropic-special",
	}

	result := proxy.FilterProvidersByModel("claude-opus-special-4", providerInfos, modelMapping, "")
	assert.Len(t, result, 1)
	assert.Equal(t, "anthropic-special", result[0].Provider.Name())

	// testModelClaudeOpus4 should match testModelPrefixClaudeOpus (only match)
	result = proxy.FilterProvidersByModel(testModelClaudeOpus4, providerInfos, modelMapping, "")
	assert.Len(t, result, 1)
	assert.Equal(t, testProviderAnthropic, result[0].Provider.Name())
}

func TestFilterProvidersByModelGracefulDegradation(t *testing.T) {
	t.Parallel()

	anthropic := proxy.NewMockProvider(testProviderAnthropic)
	providerInfos := []router.ProviderInfo{
		proxy.TestProviderInfo(anthropic),
	}

	// Model maps to non-existent provider - should return all providers
	modelMapping := map[string]string{
		testModelPrefixClaudeOpus: "nonexistent",
	}

	result := proxy.FilterProvidersByModel(testModelClaudeOpus4, providerInfos, modelMapping, "")
	assert.Len(t, result, 1)
	assert.Equal(t, testProviderAnthropic, result[0].Provider.Name())
}

func TestFindProviderForModel(t *testing.T) {
	t.Parallel()

	modelMapping := map[string]string{
		testProviderClaude:      testProviderAnthropic,
		testModelPrefixClaudeOpus: "anthropic-premium",
		"glm":         testProviderZAI,
	}

	tests := []struct {
		name      string
		model     string
		expected  string
		isPresent bool
	}{
		{"exact match", testProviderClaude, testProviderAnthropic, true},
		{"prefix match", "claude-sonnet-4", testProviderAnthropic, true},
		{"longer prefix wins", testModelClaudeOpus4, "anthropic-premium", true},
		{"no match", "gpt-4", "", false},
		{"empty model", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := proxy.FindProviderForModel(tt.model, modelMapping)
			assert.Equal(t, tt.isPresent, result.IsPresent())
			assert.Equal(t, tt.expected, result.OrEmpty())
		})
	}
}
