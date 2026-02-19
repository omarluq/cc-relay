package proxy_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/proxy"
)

func TestExtractModelFromRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		expected  string
		isPresent bool
	}{
		{
			name:      "valid model",
			body:      `{"model":"claude-opus-4","messages":[]}`,
			expected:  "claude-opus-4",
			isPresent: true,
		},
		{
			name:      "model with prefix",
			body:      `{"model":"claude-sonnet-4-20250514","messages":[]}`,
			expected:  "claude-sonnet-4-20250514",
			isPresent: true,
		},
		{
			name:      "no model field",
			body:      `{"messages":[]}`,
			expected:  "",
			isPresent: false,
		},
		{
			name:      "empty model",
			body:      `{"model":"","messages":[]}`,
			expected:  "",
			isPresent: false,
		},
		{
			name:      "invalid json",
			body:      `not json`,
			expected:  "",
			isPresent: false,
		},
		{
			name:      "model not string",
			body:      `{"model":123,"messages":[]}`,
			expected:  "",
			isPresent: false,
		},
		{
			name:      "empty body",
			body:      ``,
			expected:  "",
			isPresent: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("POST", "/v1/messages", bytes.NewBufferString(testCase.body))
			result := proxy.ExtractModelFromRequest(req)

			assert.Equal(t, testCase.isPresent, result.IsPresent())
			assert.Equal(t, testCase.expected, result.OrEmpty())

			// Verify body is restored for downstream use
			restored, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Equal(t, testCase.body, string(restored))
		})
	}
}

func TestExtractModelFromRequestNilBody(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/v1/messages", http.NoBody)
	req.Body = nil

	result := proxy.ExtractModelFromRequest(req)
	assert.False(t, result.IsPresent())
	assert.Equal(t, "", result.OrEmpty())
}

func TestCacheModelInContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	model := "claude-opus-4"

	newCtx := proxy.CacheModelInContext(ctx, model)

	// Verify model can be retrieved
	retrieved, ok := proxy.GetModelFromContext(newCtx)
	assert.True(t, ok)
	assert.Equal(t, model, retrieved)
}

func TestGetModelFromContextNotCached(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	retrieved, ok := proxy.GetModelFromContext(ctx)
	assert.False(t, ok)
	assert.Equal(t, "", retrieved)
}

func TestGetModelFromContextEmptyModel(t *testing.T) {
	t.Parallel()

	ctx := proxy.CacheModelInContext(context.Background(), "")

	retrieved, ok := proxy.GetModelFromContext(ctx)
	assert.True(t, ok) // Key exists
	assert.Equal(t, "", retrieved)
}
