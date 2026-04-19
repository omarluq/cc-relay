package proxy

import (
	"testing"

	"github.com/tidwall/gjson"

	"github.com/omarluq/cc-relay/internal/keypool"
)

func TestDropMessagesByIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		indexes []int
		want    int // expected message count after drop
		wantErr bool
	}{
		{
			name: "drop single message",
			body: `{"messages":[{"role":"user","content":"a"},` +
				`{"role":"assistant","content":"b"},{"role":"user","content":"c"}]}`,
			indexes: []int{1},
			want:    2,
			wantErr: false,
		},
		{
			name: "drop multiple messages",
			body: `{"messages":[{"role":"user","content":"a"},` +
				`{"role":"assistant","content":"b"},{"role":"user","content":"c"},` +
				`{"role":"assistant","content":"d"}]}`,
			indexes: []int{1, 3},
			want:    2,
			wantErr: false,
		},
		{
			name:    "drop first message",
			body:    `{"messages":[{"role":"user","content":"a"},{"role":"assistant","content":"b"}]}`,
			indexes: []int{0},
			want:    1,
			wantErr: false,
		},
		{
			name:    "empty indexes does nothing",
			body:    `{"messages":[{"role":"user","content":"a"}]}`,
			indexes: []int{},
			want:    1,
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()
			t.Parallel()

			result, err := dropMessagesByIndex([]byte(testCase.body), testCase.indexes)
			if testCase.wantErr {
				if err == nil {
					t.Fatal("dropMessagesByIndex() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("dropMessagesByIndex() unexpected error: %v", err)
			}

			msgCount := countMessages(result)
			if msgCount != int64(testCase.want) {
				t.Errorf("dropMessagesByIndex() message count = %d, want %d", msgCount, testCase.want)
			}
		})
	}
}

func TestNextMessageHasToolResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		nextIdx int64
		want    bool
		wantErr bool
	}{
		{
			name: "user message with tool_result",
			body: `{"messages":[{"role":"assistant","content":[` +
				`{"type":"tool_use","id":"t1"}]},{"role":"user","content":[` +
				`{"type":"tool_result","tool_use_id":"t1"}]}]}`,
			nextIdx: 1,
			want:    true,
			wantErr: false,
		},
		{
			name: "user message without tool_result",
			body: `{"messages":[{"role":"assistant","content":"hello"},` +
				`{"role":"user","content":[{"type":"text","text":"hi"}]}]}`,
			nextIdx: 1,
			want:    false,
			wantErr: false,
		},
		{
			name: "assistant message is not a tool result source",
			body: `{"messages":[{"role":"user","content":"hi"},{"role":"assistant",` +
				`"content":[{"type":"tool_result","tool_use_id":"t1"}]}]}`,
			nextIdx: 1,
			want:    false,
			wantErr: false,
		},
		{
			name: "user message with mixed content including tool_result",
			body: `{"messages":[{"role":"assistant","content":"x"},{"role":"user","content":[` +
				`{"type":"text","text":"intro"},{"type":"tool_result","tool_use_id":"t1"}]}]}`,
			nextIdx: 1,
			want:    true,
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()
			t.Parallel()

			got := nextMessageHasToolResult([]byte(testCase.body), testCase.nextIdx)
			if got != testCase.want {
				t.Errorf("nextMessageHasToolResult() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestProcessToolUseBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		blockJSON     string
		wantSignature bool
		wantErr       bool
	}{
		{
			name: "removes signature field",
			blockJSON: `{"type":"tool_use","id":"t1","name":"get_weather",` +
				`"input":{"city":"NYC"},"signature":"abc123"}`,
			wantSignature: false,
			wantErr:       false,
		},
		{
			name:          "preserves other fields when no signature",
			blockJSON:     `{"type":"tool_use","id":"t1","name":"get_weather","input":{"city":"NYC"}}`,
			wantSignature: false,
			wantErr:       false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()
			t.Parallel()

			block := parseJSON(testCase.blockJSON)
			result := processToolUseBlock(&block, "sig")

			resultMap, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("processToolUseBlock() returned %T, want map[string]any", result)
			}

			_, hasSig := resultMap["signature"]
			if hasSig != testCase.wantSignature {
				t.Errorf("processToolUseBlock() has signature = %v, want %v", hasSig, testCase.wantSignature)
			}

			// Verify other fields are preserved
			if resultMap["type"] != "tool_use" {
				t.Errorf("processToolUseBlock() type = %v, want \"tool_use\"", resultMap["type"])
			}
			if resultMap["id"] != "t1" {
				t.Errorf("processToolUseBlock() id = %v, want \"t1\"", resultMap["id"])
			}
		})
	}
}

func TestResolveInitialMaps_BothNil(t *testing.T) {
	t.Parallel()

	pools, keys := resolveInitialMaps(nil, nil)
	if pools != nil {
		t.Errorf("resolveInitialMaps() pools = %v, want nil", pools)
	}
	if keys != nil {
		t.Errorf("resolveInitialMaps() keys = %v, want nil", keys)
	}
}

func TestResolveInitialMaps_PoolsFuncProvided(t *testing.T) {
	t.Parallel()

	getPoolsFn := func() map[string]*keypool.KeyPool {
		return map[string]*keypool.KeyPool{"test": nil}
	}

	pools, keys := resolveInitialMaps(getPoolsFn, nil)
	if len(pools) != 1 {
		t.Errorf("resolveInitialMaps() pools count = %d, want 1", len(pools))
	}
	if keys != nil {
		t.Errorf("resolveInitialMaps() keys = %v, want nil", keys)
	}
}

func TestResolveInitialMaps_KeysFuncProvided(t *testing.T) {
	t.Parallel()

	getKeysFn := func() map[string]string {
		return map[string]string{"k": "v"}
	}

	pools, keys := resolveInitialMaps(nil, getKeysFn)
	if pools != nil {
		t.Errorf("resolveInitialMaps() pools = %v, want nil", pools)
	}
	if len(keys) != 1 {
		t.Errorf("resolveInitialMaps() keys count = %d, want 1", len(keys))
	}
}

func TestResolveInitialMaps_BothFuncsProvided(t *testing.T) {
	t.Parallel()

	getPoolsFn := func() map[string]*keypool.KeyPool {
		return map[string]*keypool.KeyPool{"p": nil}
	}
	getKeysFn := func() map[string]string {
		return map[string]string{"k": "v"}
	}

	pools, keys := resolveInitialMaps(getPoolsFn, getKeysFn)
	if len(pools) != 1 {
		t.Errorf("resolveInitialMaps() pools count = %d, want 1", len(pools))
	}
	if len(keys) != 1 {
		t.Errorf("resolveInitialMaps() keys count = %d, want 1", len(keys))
	}
}

// parseJSON is a helper to parse a JSON string into a gjson.Result.
func parseJSON(jsonStr string) gjson.Result {
	return gjson.Parse(jsonStr)
}
