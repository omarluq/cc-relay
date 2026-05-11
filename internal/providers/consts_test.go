package providers_test

// Shared test constants to avoid goconst lint warnings for repeated string
// literals across the providers_test package.
const (
	// Common test case names.
	testNameCustomBaseURL = "with custom base URL"
	testNameEmptyBaseURL  = "with empty base URL uses default"

	// Model identifiers used across tests.
	modelClaudeSonnet45   = "claude-sonnet-4-5-20250514"
	modelClaude4          = "claude-4"
	modelClaudeOpus45Test = "claude-opus-4-5-20251101"

	// Cloud regions / project identifiers.
	awsRegionUSEast1    = "us-east-1"
	gcpRegionUSCentral1 = "us-central1"
	gcpProjectMyProject = "my-project"
	testVertexName      = "test-vertex"

	// Provider-specific URLs.
	ollamaCustomURL = "http://10.0.0.50:11434"

	// Transform / payload constants.
	transformEmptyBody  = "empty body"
	bedrockAnthropicVer = "bedrock-2023-05-31"

	// Event stream header names.
	eventTypeHeader     = ":event-type"
	contentTypeHeader   = ":content-type"
	messageTypeHeader   = ":message-type"
	exceptionTypeHeader = ":exception-type"

	// Event types.
	contentBlockDelta = "content_block_delta"
	eventTypeStart    = "message_start"
	eventTypeEvent    = "event"

	// Ollama model placeholder used across mappings.
	ollamaModelQwen8b = "qwen3:8b"

	// Generic test string used as placeholder data.
	testStringValue = "test"
)
