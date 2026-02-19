package proxy_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/omarluq/cc-relay/internal/proxy"
)

// Test gjson path constants to avoid duplication.
const (
	pathFirstSig = "messages.0.content.0.signature"
	pathContent  = "messages.0.content"
)

func TestHasThinkingBlocks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name: "has thinking with signature",
			body: `{
				"messages": [
					{"role": "assistant", "content": [
						{"type": "thinking", "thinking": "...", "signature": "abc123"}
					]}
				]
			}`,
			expected: true,
		},
		{
			name: "no thinking blocks",
			body: `{
				"messages": [
					{"role": "assistant", "content": [{"type": "text", "text": "Hello"}]}
				]
			}`,
			expected: false,
		},
		{
			name: "thinking without signature marker",
			body: `{
				"messages": [
					{"role": "assistant", "content": [{"type": "thinking", "thinking": "..."}]}
				]
			}`,
			expected: false,
		},
		{
			name: "signature but no thinking type",
			body: `{
				"messages": [
					{"role": "assistant", "content": [{"type": "text", "signature": "abc"}]}
				]
			}`,
			expected: false,
		},
		{
			name:     "empty body",
			body:     `{}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := proxy.HasThinkingBlocks([]byte(tt.body))
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestProcessRequestThinkingCachedSignature(t *testing.T) {
	t.Parallel()
	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()

	ctx := context.Background()

	// Pre-populate cache with a valid signature
	validSig := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz"
	thinkingText := "Let me think about this..."
	sigCache.Set(ctx, "claude-sonnet-4", thinkingText, validSig)
	time.Sleep(10 * time.Millisecond) // Wait for Ristretto async set

	// Request body with thinking block (no signature - will use cached)
	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Let me think about this...", "signature": ""}
			]}
		]
	}`

	modifiedBody, thinkingCtx, err := proxy.ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", sigCache)
	require.NoError(t, err)

	// Verify cached signature was used
	sig := gjson.GetBytes(modifiedBody, pathFirstSig).String()
	assert.Equal(t, validSig, sig, "should use cached signature")
	assert.Equal(t, 0, thinkingCtx.DroppedBlocks, "should not drop block with cached sig")
}

func TestProcessRequestThinkingClientSignatureTakesPriority(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()

	// Pre-populate cache with a DIFFERENT signature for the same thinking text
	thinkingText := "I need to consider this carefully..."
	wrongSig := "wrong_signature_from_cache"
	sigCache.Set(ctx, "claude-sonnet-4", thinkingText, wrongSig)

	// Request with thinking block that has a valid CLIENT signature
	// This should WIN over the cached signature
	clientSig := "claude#correct_client_sig"
	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{
					"type": "thinking",
					"thinking": "I need to consider this carefully...",
					"signature": "` + clientSig + `"
				}
			]}
		]
	}`

	modifiedBody, _, err := proxy.ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", sigCache)
	require.NoError(t, err)

	// Verify the CLIENT signature was used, not the cached one
	sig := gjson.GetBytes(modifiedBody, pathFirstSig).String()
	assert.Equal(t, "correct_client_sig", sig, "should prefer client signature over cache")
}

func TestProcessRequestThinkingDropsAndReordersCorrectly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Request with unsigned thinking (to drop), text, and thinking (signed)
	// After dropping unsigned block, the remaining blocks should be reordered:
	// thinking should come before text
	validSig := proxy.ValidTestSignature
	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "text", "text": "This should come after thinking"},
				{"type": "thinking", "thinking": "Let me think...", "signature": ""},
				{"type": "thinking", "thinking": "Now I decide", "signature": "` + validSig + `"}
			]}
		]
	}`

	modifiedBody, _, err := proxy.ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", nil)
	require.NoError(t, err)

	// Verify unsigned block was dropped
	content := gjson.GetBytes(modifiedBody, pathContent).Array()
	assert.Len(t, content, 2, "should have 2 blocks after dropping")

	// Verify thinking comes first
	assert.Equal(t, "thinking", content[0].Get("type").String(), "thinking block should be first")
	assert.Equal(t, "text", content[1].Get("type").String(), "text block should be second")
}

func TestProcessRequestThinkingClientSignature(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Valid client signature
	clientSig := "client_signature_that_is_definitely_long_enough_for_validation"

	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Some thinking...", "signature": "` + clientSig + `"}
			]}
		]
	}`

	modifiedBody, thinkingCtx, err := proxy.ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", nil)
	require.NoError(t, err)

	// Verify client signature was preserved
	sig := gjson.GetBytes(modifiedBody, pathFirstSig).String()
	assert.Equal(t, clientSig, sig, "should preserve valid client signature")
	assert.Equal(t, 0, thinkingCtx.DroppedBlocks)
}

func TestProcessRequestThinkingUnsignedBlockDropped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Some thinking...", "signature": ""},
				{"type": "text", "text": "Hello!"}
			]}
		]
	}`

	modifiedBody, thinkingCtx, err := proxy.ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", nil)
	require.NoError(t, err)

	// Verify thinking block was dropped
	content := gjson.GetBytes(modifiedBody, pathContent)
	assert.Equal(t, 1, len(content.Array()), "should have only 1 block (text)")
	assert.Equal(t, "text", content.Array()[0].Get("type").String())
	assert.Equal(t, 1, thinkingCtx.DroppedBlocks, "should record dropped block")
}

func TestProcessRequestThinkingToolUseInheritance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Valid signature for thinking block
	thinkingSig := "thinking_signature_that_is_definitely_long_enough_for_validation"

	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Analyzing...", "signature": "` + thinkingSig + `"},
				{"type": "tool_use", "id": "tool_1", "name": "search", "input": {}}
			]}
		]
	}`

	modifiedBody, thinkingCtx, err := proxy.ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", nil)
	require.NoError(t, err)

	// Verify tool_use does not include signature
	toolBlock := gjson.GetBytes(modifiedBody, "messages.0.content.1")
	assert.Equal(t, "tool_use", toolBlock.Get("type").String())
	assert.False(t, toolBlock.Get("signature").Exists(), "tool_use should not include signature")
	assert.Equal(t, thinkingSig, thinkingCtx.CurrentSignature)
}

func TestProcessRequestThinkingBlockReordering(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Valid signature
	sig := proxy.ValidTestSignature

	// Text block before thinking block (wrong order)
	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "text", "text": "Hello"},
				{"type": "thinking", "thinking": "...", "signature": "` + sig + `"}
			]}
		]
	}`

	modifiedBody, thinkingCtx, err := proxy.ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", nil)
	require.NoError(t, err)

	// Verify blocks were reordered
	content := gjson.GetBytes(modifiedBody, pathContent)
	assert.Equal(t, 2, len(content.Array()))
	assert.Equal(t, "thinking", content.Array()[0].Get("type").String(), "thinking should be first")
	assert.Equal(t, "text", content.Array()[1].Get("type").String(), "text should be second")
	assert.True(t, thinkingCtx.ReorderedBlocks, "should record reordering")
}

func TestFormatSignature(t *testing.T) {
	t.Parallel()
	tests := []struct {
		modelName string
		signature string
		expected  string
	}{
		{"claude-sonnet-4", "abc123", "claude#abc123"},
		{"gpt-4-turbo", "xyz789", "gpt#xyz789"},
		{"gemini-pro", "sig", "gemini#sig"},
		{"unknown-model", "sig", "unknown-model#sig"},
	}

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			t.Parallel()
			got := proxy.FormatSignature(tt.modelName, tt.signature)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseSignature(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		prefixed  string
		wantGroup string
		wantSig   string
		wantOK    bool
	}{
		{"valid claude", "claude#abc123", "claude", "abc123", true},
		{"valid gpt", "gpt#xyz789", "gpt", "xyz789", true},
		{"no prefix", "abc123", "", "", false},
		{"empty", "", "", "", false},
		{"multiple hashes", "claude#sig#extra", "claude", "sig#extra", true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			group, sig, ok := proxy.ParseSignature(testCase.prefixed)
			assert.Equal(t, testCase.wantGroup, group)
			assert.Equal(t, testCase.wantSig, sig)
			assert.Equal(t, testCase.wantOK, ok)
		})
	}
}

func TestProcessRequestThinkingPreservesUnknownFields(t *testing.T) {
	t.Parallel(
	// Regression test for Anthropic API error:
	// "thinking blocks in the latest assistant message cannot be modified"
	// The proxy was reconstructing thinking blocks from scratch, losing fields like
	// 'data' and 'redacted_thinking' that Anthropic now requires to be preserved.
	)

	ctx := context.Background()

	sig := proxy.ValidTestSignature

	// Thinking block with additional fields that must be preserved
	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{
				"role": "assistant",
				"content": [
					{
						"type": "thinking",
						"thinking": "Some thinking...",
						"signature": "` + sig + `",
						"data": {"some": "metadata", "tokens": 1234}
					},
					{
						"type": "thinking",
						"thinking": "Redacted thinking...",
						"signature": "` + sig + `",
						"redacted_thinking": true
					}
				]
			}
		]
	}`

	modifiedBody, thinkingCtx, err := proxy.ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", nil)
	require.NoError(t, err)
	assert.Equal(t, 0, thinkingCtx.DroppedBlocks)

	// Verify first thinking block preserves 'data' field
	dataField := gjson.GetBytes(modifiedBody, "messages.0.content.0.data")
	assert.True(t, dataField.Exists(), "data field should be preserved")
	assert.Equal(t, "metadata", dataField.Get("some").String(), "data.some should be preserved")
	assert.Equal(t, float64(1234), dataField.Get("tokens").Float(), "data.tokens should be preserved")

	// Verify second thinking block preserves 'redacted_thinking' field
	redactedField := gjson.GetBytes(modifiedBody, "messages.0.content.1.redacted_thinking")
	assert.True(t, redactedField.Exists(), "redacted_thinking field should be preserved")
	assert.True(t, redactedField.Bool(), "redacted_thinking should be true")
}

func TestProcessResponseSignature(t *testing.T) {
	t.Parallel()
	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()

	ctx := context.Background()

	thinkingText := "Let me analyze this..."
	signature := "original_signature_that_is_long_enough_for_caching_validation"

	eventData := `{"type": "content_block_delta", "delta": ` +
		`{"type": "signature_delta", "signature": "` + signature + `"}}`

	modifiedData := proxy.ProcessResponseSignature(ctx, []byte(eventData), thinkingText, "claude-sonnet-4", sigCache)

	// Verify signature was prefixed
	var result map[string]any
	err := json.Unmarshal(modifiedData, &result)
	require.NoError(t, err)

	delta, ok := result["delta"].(map[string]any)
	require.True(t, ok, "delta should be a map")
	assert.Equal(t, "claude#"+signature, delta["signature"])

	// Wait for Ristretto async set
	time.Sleep(10 * time.Millisecond)

	// Verify signature was cached
	cached := sigCache.Get(ctx, "claude-sonnet-4", thinkingText)
	assert.Equal(t, signature, cached, "signature should be cached")
}

func TestProcessNonStreamingResponse(t *testing.T) {
	t.Parallel()
	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()

	ctx := context.Background()

	signature := "response_signature_that_is_long_enough_for_validation"
	thinkingText := "Deep thinking here..."

	body := `{
		"content": [
			{"type": "thinking", "thinking": "` + thinkingText + `", "signature": "` + signature + `"},
			{"type": "text", "text": "Result"}
		]
	}`

	modifiedBody := proxy.ProcessNonStreamingResponse(ctx, []byte(body), "claude-sonnet-4", sigCache)

	// Verify signature was prefixed
	sig := gjson.GetBytes(modifiedBody, "content.0.signature").String()
	assert.Equal(t, "claude#"+signature, sig)

	// Wait for Ristretto async set
	time.Sleep(10 * time.Millisecond)

	// Verify signature was cached
	cached := sigCache.Get(ctx, "claude-sonnet-4", thinkingText)
	assert.Equal(t, signature, cached)
}

func BenchmarkHasThinkingBlocks(b *testing.B) {
	bodyWithThinking := []byte(`{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Hello"}]},
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Let me think about this carefully...", "signature": "abc123xyz"},
				{"type": "text", "text": "Here is my response"}
			]}
		]
	}`)

	b.Run("HasThinkingBlocks", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			proxy.HasThinkingBlocks(bodyWithThinking)
		}
	})

	b.Run("JSONParse", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var body map[string]any
			// Intentionally ignoring errors in benchmark for fair comparison
			if unmarshalErr := json.Unmarshal(bodyWithThinking, &body); unmarshalErr != nil {
				b.Fatal(unmarshalErr)
			}
		}
	})
}

func BenchmarkHasThinkingBlocksNoThinking(b *testing.B) {
	bodyWithoutThinking := []byte(`{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Hello"}]},
			{"role": "assistant", "content": [{"type": "text", "text": "Hi there!"}]}
		]
	}`)

	b.Run("HasThinkingBlocks", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			proxy.HasThinkingBlocks(bodyWithoutThinking)
		}
	})
}
