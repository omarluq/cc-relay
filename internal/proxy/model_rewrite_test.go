package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

func TestModelRewriter_RewriteRequest(t *testing.T) {
	tests := []struct {
		name          string
		mapping       map[string]string
		requestBody   string
		expectedModel string
		shouldRewrite bool
		expectError   bool
	}{
		{
			name: "rewrites model when mapping exists",
			mapping: map[string]string{
				"claude-opus-4-5-20251101": "qwen3:8b",
			},
			requestBody:   `{"model":"claude-opus-4-5-20251101","messages":[{"role":"user","content":"hi"}]}`,
			expectedModel: "qwen3:8b",
			shouldRewrite: true,
		},
		{
			name: "passes through when model not in mapping",
			mapping: map[string]string{
				"claude-opus-4-5-20251101": "qwen3:8b",
			},
			requestBody:   `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hi"}]}`,
			expectedModel: "claude-sonnet-4-20250514",
			shouldRewrite: false,
		},
		{
			name:          "passes through when no mapping configured",
			mapping:       nil,
			requestBody:   `{"model":"claude-opus-4-5-20251101","messages":[{"role":"user","content":"hi"}]}`,
			expectedModel: "claude-opus-4-5-20251101",
			shouldRewrite: false,
		},
		{
			name:          "passes through when mapping is empty",
			mapping:       map[string]string{},
			requestBody:   `{"model":"claude-opus-4-5-20251101","messages":[{"role":"user","content":"hi"}]}`,
			expectedModel: "claude-opus-4-5-20251101",
			shouldRewrite: false,
		},
		{
			name: "handles missing model field gracefully",
			mapping: map[string]string{
				"claude-opus-4-5-20251101": "qwen3:8b",
			},
			requestBody:   `{"messages":[{"role":"user","content":"hi"}]}`,
			expectedModel: "",
			shouldRewrite: false,
		},
		{
			name: "handles invalid JSON gracefully",
			mapping: map[string]string{
				"claude-opus-4-5-20251101": "qwen3:8b",
			},
			requestBody:   `not valid json`,
			expectedModel: "",
			shouldRewrite: false,
		},
		{
			name: "handles non-string model field gracefully",
			mapping: map[string]string{
				"claude-opus-4-5-20251101": "qwen3:8b",
			},
			requestBody:   `{"model":123,"messages":[]}`,
			expectedModel: "",
			shouldRewrite: false,
		},
		{
			name: "rewrites with multiple mappings",
			mapping: map[string]string{
				"claude-opus-4-5-20251101":  "qwen3:8b",
				"claude-sonnet-4-20250514":  "qwen3:4b",
				"claude-haiku-3-5-20241022": "qwen3:1b",
			},
			requestBody:   `{"model":"claude-sonnet-4-20250514","messages":[]}`,
			expectedModel: "qwen3:4b",
			shouldRewrite: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rewriter := NewModelRewriter(tt.mapping)

			// Create request with body
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")

			// Create a logger
			logger := zerolog.Nop()

			// Rewrite the request
			err := rewriter.RewriteRequest(req, &logger)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Read the resulting body
			bodyBytes, _ := io.ReadAll(req.Body)
			bodyStr := string(bodyBytes)

			if tt.shouldRewrite || tt.expectedModel != "" {
				// Parse JSON and check model field
				var body map[string]any
				if err := json.Unmarshal(bodyBytes, &body); err == nil {
					if model, ok := body["model"].(string); ok {
						if model != tt.expectedModel {
							t.Errorf("expected model %q, got %q", tt.expectedModel, model)
						}
					} else if tt.expectedModel != "" {
						t.Errorf("expected model %q, but model field missing or not string", tt.expectedModel)
					}
				}
			}

			// Verify body is still readable (Content-Length should match)
			if int64(len(bodyBytes)) != req.ContentLength {
				t.Errorf("ContentLength mismatch: body=%d, ContentLength=%d", len(bodyBytes), req.ContentLength)
			}

			// For non-JSON or invalid cases, verify body is preserved
			if tt.requestBody == "not valid json" {
				if bodyStr != tt.requestBody {
					t.Errorf("expected body preserved as %q, got %q", tt.requestBody, bodyStr)
				}
			}
		})
	}
}

func TestModelRewriter_RewriteModel(t *testing.T) {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rewriter := NewModelRewriter(tt.mapping)
			result := rewriter.RewriteModel(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestModelRewriter_HasMapping(t *testing.T) {
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
			rewriter := NewModelRewriter(tt.mapping)
			if rewriter.HasMapping() != tt.expected {
				t.Errorf("expected HasMapping() = %v, got %v", tt.expected, rewriter.HasMapping())
			}
		})
	}
}

func TestModelRewriter_NilBody(t *testing.T) {
	rewriter := NewModelRewriter(map[string]string{"a": "b"})

	// Create request with nil body
	req := httptest.NewRequest(http.MethodGet, "/v1/messages", http.NoBody)

	logger := zerolog.Nop()
	err := rewriter.RewriteRequest(req, &logger)

	if err != nil {
		t.Errorf("unexpected error for nil body: %v", err)
	}
}

func TestModelRewriter_PreservesOtherFields(t *testing.T) {
	mapping := map[string]string{
		"claude-opus-4-5-20251101": "qwen3:8b",
	}
	rewriter := NewModelRewriter(mapping)

	originalBody := `{"model":"claude-opus-4-5-20251101","messages":[{"role":"user",` +
		`"content":"hello"}],"max_tokens":1000,"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader([]byte(originalBody)))

	logger := zerolog.Nop()
	err := rewriter.RewriteRequest(req, &logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bodyBytes, _ := io.ReadAll(req.Body)
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
