// Test for the orphaned tool_result fix
package proxy_test

import (
	"testing"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/omarluq/cc-relay/internal/proxy"
)

const (
	roleUser       = "user"
	roleAssistant  = "assistant"
	typeText       = "text"
	typeToolUse    = "tool_use"
	typeToolResult = "tool_result"
)

// TestWouldOrphanToolResults_ToolResultInNextMessage tests detection when next message has tool_result.
func TestWouldOrphanToolResults_ToolResultInNextMessage(t *testing.T) {
	t.Parallel()

	body := `{
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "hello"}]},
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "...", "signature": "sig"}
			]},
			{"role": "user", "content": [
				{
					"type": "tool_result",
					"tool_use_id": "toolu_123",
					"content": "result"
				}
			]}
		]
	}`

	got := proxy.WouldOrphanToolResults([]byte(body), 1)
	if !got {
		t.Error("should detect tool_result in next message")
	}
}

// TestWouldOrphanToolResults_SafeToDrop tests detection when safe to drop (last message).
func TestWouldOrphanToolResults_SafeToDrop(t *testing.T) {
	t.Parallel()

	// An assistant at the end with no following user message is safe to drop.
	body := `{
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "hello"}]},
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "...", "signature": "sig"}
			]}
		]
	}`

	got := proxy.WouldOrphanToolResults([]byte(body), 1)
	if got {
		t.Error("should be safe to drop when assistant is last message")
	}
}

// TestWouldOrphanToolResults_ConsecutiveUsers tests that dropping between two user msgs is blocked.
func TestWouldOrphanToolResults_ConsecutiveUsers(t *testing.T) {
	t.Parallel()

	// Dropping this assistant would create consecutive user messages
	body := `{
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "hello"}]},
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "...", "signature": "sig"}
			]},
			{"role": "user", "content": [{"type": "text", "text": "continue"}]}
		]
	}`

	got := proxy.WouldOrphanToolResults([]byte(body), 1)
	if !got {
		t.Error("should prevent consecutive user messages")
	}
}

// TestWouldOrphanToolResults_EmptyMessages tests empty messages array.
func TestWouldOrphanToolResults_EmptyMessages(t *testing.T) {
	t.Parallel()

	body := `{"messages": []}`

	got := proxy.WouldOrphanToolResults([]byte(body), 0)
	if got {
		t.Error("empty messages should not orphan")
	}
}

// TestReplaceContentWithPlaceholder tests the placeholder replacement.
func TestReplaceContentWithPlaceholder(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "hello"}]},
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "...", "signature": "sig"}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "toolu_123", "content": "result"}
			]}
		]
	}`)

	modified := proxy.ReplaceContentWithPlaceholder(body, 1)

	// Check that the assistant message now has placeholder content
	content := gjson.GetBytes(modified, "messages.1.content")
	if !content.Exists() {
		t.Fatal("content should exist after replacement")
	}

	if content.Get("0.type").String() != typeText {
		t.Errorf("expected placeholder type 'text', got '%s'", content.Get("0.type").String())
	}

	if content.Get("0.text").String() != "" {
		t.Errorf("expected empty placeholder text, got '%s'", content.Get("0.text").String())
	}

	// Verify the tool_result is still there
	toolResult := gjson.GetBytes(modified, "messages.2.content.0.type")
	if toolResult.String() != typeToolResult {
		t.Errorf("tool_result should still exist, got type '%s'", toolResult.String())
	}
}

// TestWouldOrphanWithMixedContent tests edge case where user has both text and tool_result.
func TestWouldOrphanWithMixedContent(t *testing.T) {
	t.Parallel()

	body := `{
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "hello"}]},
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "...", "signature": "sig"}
			]},
			{"role": "user", "content": [
				{"type": "text", "text": "here's the result"},
				{"type": "tool_result", "tool_use_id": "toolu_123", "content": "result"}
			]}
		]
	}`

	got := proxy.WouldOrphanToolResults([]byte(body), 1)
	if !got {
		t.Error("should detect tool_result in mixed content")
	}
}

// TestReplaceContentPreservesStructure tests that placeholder replacement preserves JSON structure.
func TestReplaceContentPreservesStructure(t *testing.T) {
	t.Parallel()

	original := `{
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "first"}]},
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "...", "signature": "sig"}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "abc", "content": "output"}
			]}
		],
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 100
	}`

	modified := proxy.ReplaceContentWithPlaceholder([]byte(original), 1)

	// Check model is preserved
	model := gjson.GetBytes(modified, "model").String()
	if model != "claude-3-5-sonnet-20241022" {
		t.Errorf("model should be preserved, got '%s'", model)
	}

	// Check max_tokens is preserved
	maxTokens := gjson.GetBytes(modified, "max_tokens").Int()
	if maxTokens != 100 {
		t.Errorf("max_tokens should be preserved, got %d", maxTokens)
	}

	// Check message count is preserved
	msgCount := gjson.GetBytes(modified, "messages.#").Int()
	if msgCount != 3 {
		t.Errorf("should have 3 messages, got %d", msgCount)
	}

	// Check first and last messages are unchanged
	firstRole := gjson.GetBytes(modified, "messages.0.role").String()
	if firstRole != roleUser {
		t.Errorf("first message role should be 'user', got '%s'", firstRole)
	}

	lastRole := gjson.GetBytes(modified, "messages.2.role").String()
	if lastRole != roleUser {
		t.Errorf("last message role should be 'user', got '%s'", lastRole)
	}

	// Check assistant now has placeholder
	assistantContent := gjson.GetBytes(modified, "messages.1.content.0.type").String()
	if assistantContent != typeText {
		t.Errorf("assistant should have placeholder text block, got type '%s'", assistantContent)
	}
}

// TestSjsonSetBytesBehavior is a sanity check for sjson behavior.
func TestSjsonSetBytesBehavior(t *testing.T) {
	t.Parallel()

	// This test documents sjson.SetBytes behavior for reference
	original := `{"a": 1, "b": 2}`

	modified, err := sjson.SetBytes([]byte(original), "b", 99)
	if err != nil {
		t.Fatal(err)
	}

	result := gjson.GetBytes(modified, "b").Int()
	if result != 99 {
		t.Errorf("sjson.SetBytes should update value, got %d", result)
	}

	// Verify 'a' is unchanged
	a := gjson.GetBytes(modified, "a").Int()
	if a != 1 {
		t.Errorf("other values should be preserved, got %d", a)
	}
}
