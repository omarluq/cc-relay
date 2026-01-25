package providers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		body     []byte
	}{
		{
			name:     "valid JSON with model",
			body:     []byte(`{"model":"claude-3-opus-20240229","max_tokens":1024}`),
			expected: "claude-3-opus-20240229",
		},
		{
			name:     "missing model field",
			body:     []byte(`{"max_tokens":1024}`),
			expected: "",
		},
		{
			name:     "empty body",
			body:     []byte{},
			expected: "",
		},
		{
			name:     "invalid JSON",
			body:     []byte(`{invalid`),
			expected: "",
		},
		{
			name:     "null model",
			body:     []byte(`{"model":null}`),
			expected: "",
		},
		{
			name:     "empty model string",
			body:     []byte(`{"model":""}`),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ExtractModel(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRemoveModelFromBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		checkFields map[string]interface{}
		name        string
		body        []byte
		wantModel   bool
		wantErr     bool
	}{
		{
			name:      "removes model preserves other fields",
			body:      []byte(`{"model":"claude-3-opus-20240229","max_tokens":1024,"stream":true}`),
			wantModel: false,
			wantErr:   false,
			checkFields: map[string]interface{}{
				"max_tokens": float64(1024),
				"stream":     true,
			},
		},
		{
			name:        "body without model unchanged",
			body:        []byte(`{"max_tokens":1024}`),
			wantModel:   false,
			wantErr:     false,
			checkFields: map[string]interface{}{"max_tokens": float64(1024)},
		},
		{
			name:        "empty object",
			body:        []byte(`{}`),
			wantModel:   false,
			wantErr:     false,
			checkFields: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := RemoveModelFromBody(tt.body)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Parse result to check fields
			var parsed map[string]interface{}
			err = json.Unmarshal(result, &parsed)
			require.NoError(t, err)

			// Model should be absent
			_, hasModel := parsed["model"]
			assert.Equal(t, tt.wantModel, hasModel, "model field presence")

			// Check expected fields
			for key, expectedVal := range tt.checkFields {
				assert.Equal(t, expectedVal, parsed[key], "field %s", key)
			}
		})
	}
}

func TestAddAnthropicVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		body    []byte
		wantErr bool
	}{
		{
			name:    "adds to body without existing field",
			body:    []byte(`{"max_tokens":1024}`),
			version: "bedrock-2023-05-31",
			wantErr: false,
		},
		{
			name:    "updates existing field",
			body:    []byte(`{"anthropic_version":"old","max_tokens":1024}`),
			version: "vertex-2023-10-16",
			wantErr: false,
		},
		{
			name:    "empty object",
			body:    []byte(`{}`),
			version: "bedrock-2023-05-31",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := AddAnthropicVersion(tt.body, tt.version)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Parse result to check version
			var parsed map[string]interface{}
			err = json.Unmarshal(result, &parsed)
			require.NoError(t, err)

			assert.Equal(t, tt.version, parsed["anthropic_version"])
		})
	}
}

func TestTransformBodyForCloudProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		version         string
		expectedModel   string
		body            []byte
		wantErr         bool
		wantModelInBody bool
	}{
		{
			name: "full transformation pipeline",
			body: []byte(`{
				"model": "claude-3-opus-20240229",
				"max_tokens": 1024,
				"messages": [{"role": "user", "content": "Hello"}]
			}`),
			version:         "bedrock-2023-05-31",
			expectedModel:   "claude-3-opus-20240229",
			wantErr:         false,
			wantModelInBody: false,
		},
		{
			name:            "no model in body",
			body:            []byte(`{"max_tokens": 1024}`),
			version:         "vertex-2023-10-16",
			expectedModel:   "",
			wantErr:         false,
			wantModelInBody: false,
		},
		{
			name:            "empty body",
			body:            []byte(`{}`),
			version:         "bedrock-2023-05-31",
			expectedModel:   "",
			wantErr:         false,
			wantModelInBody: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			newBody, model, err := TransformBodyForCloudProvider(tt.body, tt.version)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Check extracted model
			assert.Equal(t, tt.expectedModel, model)

			// Parse result to verify transformation
			var parsed map[string]interface{}
			err = json.Unmarshal(newBody, &parsed)
			require.NoError(t, err)

			// Model should be removed
			_, hasModel := parsed["model"]
			assert.Equal(t, tt.wantModelInBody, hasModel, "model should be removed from body")

			// Version should be added
			assert.Equal(t, tt.version, parsed["anthropic_version"])
		})
	}
}

func TestTransformBodyForCloudProvider_PreservesFields(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model": "claude-3-opus-20240229",
		"max_tokens": 4096,
		"messages": [{"role": "user", "content": "Test"}],
		"system": "You are helpful",
		"temperature": 0.7,
		"stream": true
	}`)

	newBody, model, err := TransformBodyForCloudProvider(body, "bedrock-2023-05-31")
	require.NoError(t, err)

	assert.Equal(t, "claude-3-opus-20240229", model)

	var parsed map[string]interface{}
	err = json.Unmarshal(newBody, &parsed)
	require.NoError(t, err)

	// Check all expected fields preserved
	assert.Equal(t, float64(4096), parsed["max_tokens"])
	assert.Equal(t, "You are helpful", parsed["system"])
	assert.Equal(t, 0.7, parsed["temperature"])
	assert.Equal(t, true, parsed["stream"])
	assert.Equal(t, "bedrock-2023-05-31", parsed["anthropic_version"])

	// Model should be removed
	_, hasModel := parsed["model"]
	assert.False(t, hasModel)

	// Messages should be preserved
	messages, ok := parsed["messages"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, messages, 1)
}
