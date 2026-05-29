package proxy_test

// Shared test constants for tests in package proxy_test.
// Centralized here so each value lives in exactly one place and to keep
// individual test files free of repeated string literals (goconst).
const (
	// Routing strategy.
	testStrategyLeastLoaded = "least_loaded"

	// HTTP header names.
	testHeaderAuth = "Authorization"

	// Provider names used across test fixtures.
	testProvider1         = "provider1"
	testProvider2         = "provider2"
	testKey1              = "key1"
	testKey2              = "key2"
	testProviderClaude    = "claude"
	testProviderGPT       = "gpt"
	testProviderAnthropic = "anthropic"
	testProviderOllama    = "ollama"
	testProviderZAI       = "zai"

	// Model prefix used by mapping/filter tests.
	testModelPrefixClaudeOpus = "claude-opus"

	// Additional model names used across tests.
	testModelGLM4Plus = "glm-4-plus"

	// Model identifiers used across tests.
	testModelGLM4           = "glm-4"
	testModelClaudeOpus4    = "claude-opus-4"
	testModelClaudeSonnet4  = "claude-sonnet-4-20250514"
	testModelClaudeOpus45   = "claude-opus-4-5-20251101"
	testModelClaudeSonnet45 = "claude-sonnet-4-5-20250514"
	testModelClaudeOpus45p  = "claude-opus-4-5-20250514"
	testModelQwen3_8B       = "qwen3:8b"
	testModelUnknown        = "unknown-model"

	// Request bodies / payloads.
	testRequestBodyOpus45 = `{"model":"claude-opus-4-5-20251101","messages":[{"role":"user","content":"hi"}]}`

	// Logger / log-level strings.
	testLogLevelInfo    = "info"
	testLogOutputStdout = "stdout"
	testLogLevelUnknown = "unknown"

	// Misc.
	testMockProvider = "mock"
	testValueGeneric = "test"
	testCommitSHA    = "abc123"
)
