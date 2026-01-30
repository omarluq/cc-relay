package proxy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/omarluq/cc-relay/internal/cache"
)

func TestHasThinkingBlocks(t *testing.T) {
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
			got := HasThinkingBlocks([]byte(tt.body))
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestProcessRequestThinkingCachedSignature(t *testing.T) {
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

	sc := NewSignatureCache(c)
	ctx := context.Background()

	// Pre-populate cache with a valid signature
	validSig := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz"
	thinkingText := "Let me think about this..."
	sc.Set(ctx, "claude-sonnet-4", thinkingText, validSig)
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

	modifiedBody, thinkingCtx, err := ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", sc)
	require.NoError(t, err)

	// Verify cached signature was used
	sig := gjson.GetBytes(modifiedBody, "messages.0.content.0.signature").String()
	assert.Equal(t, validSig, sig, "should use cached signature")
	assert.Equal(t, 0, thinkingCtx.DroppedBlocks, "should not drop block with cached sig")
}

func TestProcessRequestThinkingClientSignature(t *testing.T) {
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

	modifiedBody, thinkingCtx, err := ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", nil)
	require.NoError(t, err)

	// Verify client signature was preserved
	sig := gjson.GetBytes(modifiedBody, "messages.0.content.0.signature").String()
	assert.Equal(t, clientSig, sig, "should preserve valid client signature")
	assert.Equal(t, 0, thinkingCtx.DroppedBlocks)
}

func TestProcessRequestThinkingUnsignedBlockDropped(t *testing.T) {
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

	modifiedBody, thinkingCtx, err := ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", nil)
	require.NoError(t, err)

	// Verify thinking block was dropped
	content := gjson.GetBytes(modifiedBody, "messages.0.content")
	assert.Equal(t, 1, len(content.Array()), "should have only 1 block (text)")
	assert.Equal(t, "text", content.Array()[0].Get("type").String())
	assert.Equal(t, 1, thinkingCtx.DroppedBlocks, "should record dropped block")
}

func TestProcessRequestThinkingToolUseInheritance(t *testing.T) {
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

	modifiedBody, thinkingCtx, err := ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", nil)
	require.NoError(t, err)

	// Verify tool_use does not include signature
	toolBlock := gjson.GetBytes(modifiedBody, "messages.0.content.1")
	assert.Equal(t, "tool_use", toolBlock.Get("type").String())
	assert.False(t, toolBlock.Get("signature").Exists(), "tool_use should not include signature")
	assert.Equal(t, thinkingSig, thinkingCtx.CurrentSignature)
}

func TestProcessRequestThinkingBlockReordering(t *testing.T) {
	ctx := context.Background()

	// Valid signature
	sig := "valid_signature_that_is_definitely_long_enough_for_validation"

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

	modifiedBody, thinkingCtx, err := ProcessRequestThinking(ctx, []byte(body), "claude-sonnet-4", nil)
	require.NoError(t, err)

	// Verify blocks were reordered
	content := gjson.GetBytes(modifiedBody, "messages.0.content")
	assert.Equal(t, 2, len(content.Array()))
	assert.Equal(t, "thinking", content.Array()[0].Get("type").String(), "thinking should be first")
	assert.Equal(t, "text", content.Array()[1].Get("type").String(), "text should be second")
	assert.True(t, thinkingCtx.ReorderedBlocks, "should record reordering")
}

func TestFormatSignature(t *testing.T) {
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
			got := FormatSignature(tt.modelName, tt.signature)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseSignature(t *testing.T) {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, sig, ok := ParseSignature(tt.prefixed)
			assert.Equal(t, tt.wantGroup, group)
			assert.Equal(t, tt.wantSig, sig)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestProcessResponseSignature(t *testing.T) {
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

	sc := NewSignatureCache(c)
	ctx := context.Background()

	thinkingText := "Let me analyze this..."
	signature := "original_signature_that_is_long_enough_for_caching_validation"

	eventData := `{"type": "content_block_delta", "delta": ` +
		`{"type": "signature_delta", "signature": "` + signature + `"}}`

	modifiedData := ProcessResponseSignature(ctx, []byte(eventData), thinkingText, "claude-sonnet-4", sc)

	// Verify signature was prefixed
	var result map[string]interface{}
	err = json.Unmarshal(modifiedData, &result)
	require.NoError(t, err)

	delta := result["delta"].(map[string]interface{})
	assert.Equal(t, "claude#"+signature, delta["signature"])

	// Wait for Ristretto async set
	time.Sleep(10 * time.Millisecond)

	// Verify signature was cached
	cached := sc.Get(ctx, "claude-sonnet-4", thinkingText)
	assert.Equal(t, signature, cached, "signature should be cached")
}

func TestProcessNonStreamingResponse(t *testing.T) {
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

	sc := NewSignatureCache(c)
	ctx := context.Background()

	signature := "response_signature_that_is_long_enough_for_validation"
	thinkingText := "Deep thinking here..."

	body := `{
		"content": [
			{"type": "thinking", "thinking": "` + thinkingText + `", "signature": "` + signature + `"},
			{"type": "text", "text": "Result"}
		]
	}`

	modifiedBody := ProcessNonStreamingResponse(ctx, []byte(body), "claude-sonnet-4", sc)

	// Verify signature was prefixed
	sig := gjson.GetBytes(modifiedBody, "content.0.signature").String()
	assert.Equal(t, "claude#"+signature, sig)

	// Wait for Ristretto async set
	time.Sleep(10 * time.Millisecond)

	// Verify signature was cached
	cached := sc.Get(ctx, "claude-sonnet-4", thinkingText)
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
			HasThinkingBlocks(bodyWithThinking)
		}
	})

	b.Run("JSONParse", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var body map[string]interface{}
			_ = json.Unmarshal(bodyWithThinking, &body)
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
			HasThinkingBlocks(bodyWithoutThinking)
		}
	})
}
