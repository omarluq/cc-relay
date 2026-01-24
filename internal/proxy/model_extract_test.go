package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("POST", "/v1/messages", bytes.NewBufferString(tt.body))
			result := ExtractModelFromRequest(req)

			assert.Equal(t, tt.isPresent, result.IsPresent())
			assert.Equal(t, tt.expected, result.OrEmpty())

			// Verify body is restored for downstream use
			restored, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.body, string(restored))
		})
	}
}

func TestExtractModelFromRequest_NilBody(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/v1/messages", http.NoBody)
	req.Body = nil

	result := ExtractModelFromRequest(req)
	assert.False(t, result.IsPresent())
	assert.Equal(t, "", result.OrEmpty())
}

func TestCacheModelInContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	model := "claude-opus-4"

	newCtx := CacheModelInContext(ctx, model)

	// Verify model can be retrieved
	retrieved, ok := GetModelFromContext(newCtx)
	assert.True(t, ok)
	assert.Equal(t, model, retrieved)
}

func TestGetModelFromContext_NotCached(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	retrieved, ok := GetModelFromContext(ctx)
	assert.False(t, ok)
	assert.Equal(t, "", retrieved)
}

func TestGetModelFromContext_EmptyModel(t *testing.T) {
	t.Parallel()

	ctx := CacheModelInContext(context.Background(), "")

	retrieved, ok := GetModelFromContext(ctx)
	assert.True(t, ok) // Key exists
	assert.Equal(t, "", retrieved)
}
