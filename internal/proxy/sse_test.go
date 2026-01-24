package proxy

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/cache"
)

func TestIsStreamingRequest_True(t *testing.T) {
	t.Parallel()

	body := []byte(`{"stream": true}`)
	if !IsStreamingRequest(body) {
		t.Error("Expected IsStreamingRequest to return true for stream: true")
	}
}

func TestIsStreamingRequest_False(t *testing.T) {
	t.Parallel()

	body := []byte(`{"stream": false}`)
	if IsStreamingRequest(body) {
		t.Error("Expected IsStreamingRequest to return false for stream: false")
	}
}

func TestIsStreamingRequest_Missing(t *testing.T) {
	t.Parallel()

	body := []byte(`{}`)
	if IsStreamingRequest(body) {
		t.Error("Expected IsStreamingRequest to return false when stream field is missing")
	}
}

func TestIsStreamingRequest_InvalidJSON(t *testing.T) {
	t.Parallel()

	body := []byte(`{invalid json}`)
	if IsStreamingRequest(body) {
		t.Error("Expected IsStreamingRequest to return false for invalid JSON")
	}
}

func TestSetSSEHeaders(t *testing.T) {
	t.Parallel()

	h := make(http.Header)
	SetSSEHeaders(h)

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"Content-Type", "Content-Type", "text/event-stream"},
		{"Cache-Control", "Cache-Control", "no-cache, no-transform"},
		{"X-Accel-Buffering", "X-Accel-Buffering", "no"},
		{"Connection", "Connection", "keep-alive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := h.Get(tt.key)
			if got != tt.expected {
				t.Errorf("Expected %s header to be %q, got %q", tt.key, tt.expected, got)
			}
		})
	}
}

func TestSSESignatureProcessor_AccumulatesThinking(t *testing.T) {
	t.Parallel()

	processor := NewSSESignatureProcessor(nil, "claude-sonnet-4")

	// Simulate thinking_delta events
	thinkingEvent1 := []byte(
		`data: {"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"Hello "}}`,
	)
	thinkingEvent2 := []byte(
		`data: {"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"world!"}}`,
	)

	ctx := context.Background()
	processor.ProcessEvent(ctx, thinkingEvent1)
	processor.ProcessEvent(ctx, thinkingEvent2)

	// The processor should have accumulated "Hello world!"
	// This is internal state, but we can verify via signature processing
	assert.Empty(t, processor.GetCurrentSignature(), "no signature yet")
}

func TestSSESignatureProcessor_CachesSignature(t *testing.T) {
	t.Parallel()

	cfg := cache.Config{
		Mode: cache.ModeSingle,
		Ristretto: cache.RistrettoConfig{
			NumCounters: 1e4,
			MaxCost:     1 << 20,
			BufferItems: 64,
		},
	}
	c, err := cache.New(context.Background(), &cfg)
	require.NoError(t, err)
	defer c.Close()

	sigCache := NewSignatureCache(c)
	processor := NewSSESignatureProcessor(sigCache, "claude-sonnet-4")
	ctx := context.Background()

	// Simulate thinking followed by signature
	thinkingEvent := []byte(
		`data: {"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"Deep thought"}}`,
	)
	processor.ProcessEvent(ctx, thinkingEvent)

	sig := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz"
	sigEvent := []byte(
		`data: {"type":"content_block_delta","delta":{"type":"signature_delta","signature":"` + sig + `"}}`,
	)
	processor.ProcessEvent(ctx, sigEvent)

	// Wait for Ristretto async set using Eventually instead of fixed sleep
	require.Eventually(t, func() bool {
		return sigCache.Get(ctx, "claude-sonnet-4", "Deep thought") == sig
	}, 250*time.Millisecond, 5*time.Millisecond, "signature should be cached")
	assert.Equal(t, sig, processor.GetCurrentSignature())
}

func TestSSESignatureProcessor_PassesThroughNonThinking(t *testing.T) {
	t.Parallel()

	processor := NewSSESignatureProcessor(nil, "claude-sonnet-4")
	ctx := context.Background()

	// Regular text event should pass through unchanged
	textEvent := []byte(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`)
	result := processor.ProcessEvent(ctx, textEvent)
	assert.Equal(t, textEvent, result, "non-thinking event should pass through unchanged")
}

func TestExtractSSEData(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "simple data line",
			input:    []byte(`data: {"type":"test"}`),
			expected: []byte(`{"type":"test"}`),
		},
		{
			name:     "data with trailing newline",
			input:    []byte("data: {\"type\":\"test\"}\n"),
			expected: []byte(`{"type":"test"}`),
		},
		{
			name:     "no data prefix",
			input:    []byte(`event: message`),
			expected: nil,
		},
		{
			name:     "empty data",
			input:    []byte(`data:`),
			expected: nil, // TrimSpace returns nil for empty input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSSEData(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
