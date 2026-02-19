package proxy_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/proxy"
	"github.com/rs/zerolog"
)

// rewriteTestCase defines a model rewrite test scenario.
type rewriteTestCase struct {
	name          string
	mapping       map[string]string
	requestBody   string
	expectedModel string
	shouldRewrite bool
	expectError   bool
}

// singleMapping is the mapping used in most rewrite test cases.
var singleMapping = map[string]string{"claude-opus-4-5-20251101": "qwen3:8b"}

func TestModelRewriterRewriteRequest(t *testing.T) {
	t.Parallel()
	tests := []rewriteTestCase{
		{"rewrites model when mapping exists", singleMapping,
			`{"model":"claude-opus-4-5-20251101","messages":[{"role":"user","content":"hi"}]}`,
			"qwen3:8b", true, false},
		{"passes through when model not in mapping", singleMapping,
			`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hi"}]}`,
			"claude-sonnet-4-20250514", false, false},
		{"passes through when no mapping configured", nil,
			`{"model":"claude-opus-4-5-20251101","messages":[{"role":"user","content":"hi"}]}`,
			"claude-opus-4-5-20251101", false, false},
		{"passes through when mapping is empty", map[string]string{},
			`{"model":"claude-opus-4-5-20251101","messages":[{"role":"user","content":"hi"}]}`,
			"claude-opus-4-5-20251101", false, false},
		{"handles missing model field gracefully", singleMapping,
			`{"messages":[{"role":"user","content":"hi"}]}`, "", false, false},
		{"handles invalid JSON gracefully", singleMapping,
			`not valid json`, "", false, false},
		{"handles non-string model field gracefully", singleMapping,
			`{"model":123,"messages":[]}`, "", false, false},
		{"rewrites with multiple mappings", map[string]string{
			"claude-opus-4-5-20251101": "qwen3:8b", "claude-sonnet-4-20250514": "qwen3:4b",
			"claude-haiku-3-5-20241022": "qwen3:1b"},
			`{"model":"claude-sonnet-4-20250514","messages":[]}`, "qwen3:4b", true, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			RunModelRewriteCase(t, testCase)
		})
	}
}

// assertModelField checks the model field in the parsed JSON body.
func assertModelField(t *testing.T, bodyBytes []byte, expectedModel string) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return
	}
	model, ok := body["model"].(string)
	if !ok {
		assert.Empty(t, expectedModel, "expected model %q, but model field missing or not string", expectedModel)
		return
	}
	assert.Equal(t, expectedModel, model)
}

// testModelRewriteCase executes a single test case for model rewriting.
func RunModelRewriteCase(t *testing.T, testCase rewriteTestCase) {
	t.Helper()
	rewriter := proxy.NewModelRewriter(testCase.mapping)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader([]byte(testCase.requestBody)))
	req.Header.Set("Content-Type", "application/json")

	logger := zerolog.Nop()
	err := rewriter.RewriteRequest(req, &logger)

	if testCase.expectError {
		assert.Error(t, err)
	} else {
		assert.NoError(t, err)
	}

	bodyBytes, readErr := io.ReadAll(req.Body)
	require.NoError(t, readErr, "failed to read request body")

	if testCase.shouldRewrite || testCase.expectedModel != "" {
		assertModelField(t, bodyBytes, testCase.expectedModel)
	}

	assert.Equal(t, int64(len(bodyBytes)), req.ContentLength, "ContentLength mismatch")

	if testCase.requestBody == "not valid json" {
		assert.Equal(t, testCase.requestBody, string(bodyBytes), "body should be preserved")
	}
}

func TestModelRewriterRewriteModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		mapping  map[string]string
		input    string
		expected string
	}{
		{
			name: "maps model when found",
			mapping: map[string]string{
				"claude-opus-4-5-20251101": "qwen3:8b",
			},
			input:    "claude-opus-4-5-20251101",
			expected: "qwen3:8b",
		},
		{
			name: "returns original when not found",
			mapping: map[string]string{
				"claude-opus-4-5-20251101": "qwen3:8b",
			},
			input:    "some-other-model",
			expected: "some-other-model",
		},
		{
			name:     "returns original when mapping is nil",
			mapping:  nil,
			input:    "claude-opus-4-5-20251101",
			expected: "claude-opus-4-5-20251101",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			rewriter := proxy.NewModelRewriter(testCase.mapping)
			result := rewriter.RewriteModel(testCase.input)
			if result != testCase.expected {
				t.Errorf("expected %q, got %q", testCase.expected, result)
			}
		})
	}
}

func TestModelRewriterHasMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mapping  map[string]string
		name     string
		expected bool
	}{
		{
			name:     "returns true when mapping has entries",
			mapping:  map[string]string{"a": "b"},
			expected: true,
		},
		{
			name:     "returns false when mapping is empty",
			mapping:  map[string]string{},
			expected: false,
		},
		{
			name:     "returns false when mapping is nil",
			mapping:  nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rewriter := proxy.NewModelRewriter(tt.mapping)
			if rewriter.HasMapping() != tt.expected {
				t.Errorf("expected HasMapping() = %v, got %v", tt.expected, rewriter.HasMapping())
			}
		})
	}
}

func TestModelRewriterNilBody(t *testing.T) {
	t.Parallel()
	rewriter := proxy.NewModelRewriter(map[string]string{"a": "b"})

	// Create request with nil body
	req := httptest.NewRequest(http.MethodGet, "/v1/messages", http.NoBody)

	logger := zerolog.Nop()
	err := rewriter.RewriteRequest(req, &logger)

	if err != nil {
		t.Errorf("unexpected error for nil body: %v", err)
	}
}

func TestModelRewriterPreservesOtherFields(t *testing.T) {
	t.Parallel()
	mapping := map[string]string{
		"claude-opus-4-5-20251101": "qwen3:8b",
	}
	rewriter := proxy.NewModelRewriter(mapping)

	originalBody := `{"model":"claude-opus-4-5-20251101","messages":[{"role":"user",` +
		`"content":"hello"}],"max_tokens":1000,"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader([]byte(originalBody)))

	logger := zerolog.Nop()
	err := rewriter.RewriteRequest(req, &logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bodyBytes, readErr := io.ReadAll(req.Body)
	if readErr != nil {
		t.Fatalf("failed to read request body: %v", readErr)
	}
	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("failed to parse result body: %v", err)
	}

	// Check model was rewritten
	if body["model"] != "qwen3:8b" {
		t.Errorf("expected model qwen3:8b, got %v", body["model"])
	}

	// Check other fields preserved
	if body["max_tokens"] != float64(1000) {
		t.Errorf("expected max_tokens 1000, got %v", body["max_tokens"])
	}
	if body["stream"] != true {
		t.Errorf("expected stream true, got %v", body["stream"])
	}

	messages, ok := body["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Errorf("expected messages array with 1 element")
	}
}
