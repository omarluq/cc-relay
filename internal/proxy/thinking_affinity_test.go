package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"testing"
)

func TestHasThinkingSignature_NoMessages(t *testing.T) {
	body := `{"model": "claude-3-5-sonnet-20241022", "messages": []}`
	r := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte(body)))

	got := HasThinkingSignature(r)
	if got {
		t.Error("expected false for empty messages, got true")
	}
}

func TestHasThinkingSignature_NoThinkingBlocks(t *testing.T) {
	body := `{
		"model": "claude-3-5-sonnet-20241022",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Hello"}]},
			{"role": "assistant", "content": [{"type": "text", "text": "Hi there!"}]}
		]
	}`
	r := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte(body)))

	got := HasThinkingSignature(r)
	if got {
		t.Error("expected false for messages without thinking blocks, got true")
	}
}

func TestHasThinkingSignature_HasThinkingBlock(t *testing.T) {
	body := `{
		"model": "claude-3-5-sonnet-20241022",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Think carefully"}]},
			{
				"role": "assistant",
				"content": [
					{"type": "thinking", "thinking": "Let me think...", "signature": "abc123xyz"},
					{"type": "text", "text": "Here's my response"}
				]
			}
		]
	}`
	r := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte(body)))

	got := HasThinkingSignature(r)
	if !got {
		t.Error("expected true for messages with thinking block, got false")
	}
}

func TestHasThinkingSignature_ThinkingWithoutSignature(t *testing.T) {
	// Edge case: thinking block exists but no signature (shouldn't happen in practice)
	body := `{
		"model": "claude-3-5-sonnet-20241022",
		"messages": [
			{"role": "assistant", "content": [{"type": "thinking", "thinking": "..."}]}
		]
	}`
	r := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte(body)))

	got := HasThinkingSignature(r)
	if got {
		t.Error("expected false for thinking block without signature, got true")
	}
}

func TestHasThinkingSignature_ThinkingInUserMessage(t *testing.T) {
	// Thinking blocks in user messages should not trigger affinity
	// (only assistant messages contain provider signatures)
	body := `{
		"model": "claude-3-5-sonnet-20241022",
		"messages": [
			{"role": "user", "content": [{"type": "thinking", "thinking": "...", "signature": "abc123"}]}
		]
	}`
	r := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte(body)))

	got := HasThinkingSignature(r)
	if got {
		t.Error("expected false for thinking block in user message, got true")
	}
}

func TestHasThinkingSignature_MalformedJSON(t *testing.T) {
	body := `{invalid json`
	r := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte(body)))

	got := HasThinkingSignature(r)
	if got {
		t.Error("expected false for malformed JSON, got true")
	}
}

func TestHasThinkingSignature_NilBody(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/messages", http.NoBody)
	r.Body = nil

	got := HasThinkingSignature(r)
	if got {
		t.Error("expected false for nil body, got true")
	}
}

func TestHasThinkingSignature_BodyRestored(t *testing.T) {
	originalBody := `{"model": "test", "messages": []}`
	r := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte(originalBody)))

	// Call the function
	_ = HasThinkingSignature(r)

	// Verify body can be read again
	restoredBody, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read restored body: %v", err)
	}

	if string(restoredBody) != originalBody {
		t.Errorf("body not properly restored: got %q, want %q", string(restoredBody), originalBody)
	}
}

func TestHasThinkingSignature_MultipleAssistantMessages(t *testing.T) {
	// Test conversation with multiple turns, only later message has thinking
	body := `{
		"model": "claude-3-5-sonnet-20241022",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Hello"}]},
			{"role": "assistant", "content": [{"type": "text", "text": "Hi!"}]},
			{"role": "user", "content": [{"type": "text", "text": "Think about this"}]},
			{
				"role": "assistant",
				"content": [
					{"type": "thinking", "thinking": "...", "signature": "sig123"}
				]
			}
		]
	}`
	r := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte(body)))

	got := HasThinkingSignature(r)
	if !got {
		t.Error("expected true for multi-turn conversation with thinking, got false")
	}
}

func TestCacheThinkingAffinityInContext(t *testing.T) {
	ctx := context.Background()

	// Test caching true
	ctx = CacheThinkingAffinityInContext(ctx, true)
	got := GetThinkingAffinityFromContext(ctx)
	if !got {
		t.Error("expected true from cached context, got false")
	}

	// Test caching false
	ctx2 := CacheThinkingAffinityInContext(context.Background(), false)
	got2 := GetThinkingAffinityFromContext(ctx2)
	if got2 {
		t.Error("expected false from cached context, got true")
	}
}

func TestGetThinkingAffinityFromContext_NotCached(t *testing.T) {
	ctx := context.Background()
	got := GetThinkingAffinityFromContext(ctx)
	if got {
		t.Error("expected false for uncached context, got true")
	}
}
