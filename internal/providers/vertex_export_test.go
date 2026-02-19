package providers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertPreservesBodyFields verifies that a transformed request body preserves all
// expected fields (max_tokens, temperature, stream, system, messages) and has the
// correct anthropic_version. This helper is shared between Bedrock and Vertex tests.
// Exported for use by providers_test package.
func AssertPreservesBodyFields(
	t *testing.T,
	newBody []byte,
	expectedAnthropicVersion string,
) {
	t.Helper()

	var result map[string]any
	err := json.Unmarshal(newBody, &result)
	require.NoError(t, err)

	// All fields should be preserved except model
	assert.Equal(t, float64(1024), result["max_tokens"])
	assert.Equal(t, 0.7, result["temperature"])
	assert.Equal(t, true, result["stream"])
	assert.Equal(t, "You are helpful", result["system"])
	assert.NotNil(t, result["messages"])
	assert.Equal(t, expectedAnthropicVersion, result["anthropic_version"])
}
