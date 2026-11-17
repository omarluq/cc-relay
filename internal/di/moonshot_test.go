package di_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/di"
	"github.com/omarluq/cc-relay/internal/providers"
)

const moonshotToolReply = `{
  "id":"msg-kimi","type":"message","role":"assistant","model":"kimi-k3.1-pro",
  "content":[{"type":"tool_use","id":"tool-1","name":"echo","input":{"text":"hello"}}],
  "stop_reason":"tool_use","usage":{"input_tokens":5,"output_tokens":10}
}`

const moonshotStreamReply = "event: message_start\n" +
	"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg-kimi\",\"model\":\"kimi-k3.1-pro\"}}\n\n" +
	"event: content_block_delta\n" +
	"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
	"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

const moonshotRequest = `{
  "model":"claude-opus-4-6","max_tokens":4096,"stream":%t,
  "thinking":{"type":"enabled","budget_tokens":1024},
  "messages":[{"role":"user","content":"Say hello using echo"}],
  "tools":[{"name":"echo","input_schema":{"type":"object","properties":{"text":{"type":"string"}}}}]
}`

const moonshotProxyConfig = `
server:
  listen: ":8787"
  outbound_proxy: %q
cache:
  mode: disabled
providers:
  - name: kimi
    type: moonshot
    enabled: true
    base_url: "http://moonshot.example/anthropic"
    keys:
      - key: "moonshot-test-key"
    model_mapping:
      "claude-opus-4-6": "kimi-k3.1-pro"
`

func TestMoonshotRequestThroughOutboundProxy(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name        string
		contentType string
		reply       string
		stream      bool
	}{
		{name: "tool response", stream: false, contentType: "application/json", reply: moonshotToolReply},
		{name: "streaming", stream: true, contentType: "text/event-stream", reply: moonshotStreamReply},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			seen := make(chan *http.Request, 1)
			bodies := make(chan string, 1)
			forwardProxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Errorf("read upstream request: %v", err)
					http.Error(writer, "read failed", http.StatusBadRequest)
					return
				}
				seen <- req
				bodies <- string(body)
				writer.Header().Set("Content-Type", testCase.contentType)
				_, err = io.WriteString(writer, testCase.reply)
				assert.NoError(t, err)
			}))
			defer forwardProxy.Close()

			injector := createTestInjector(t, fmt.Sprintf(moonshotProxyConfig, forwardProxy.URL))
			defer shutdownInjector(t, injector)
			handlerSvc, err := do.Invoke[*di.HandlerService](injector)
			require.NoError(t, err)
			providerSvc := do.MustInvoke[*di.ProviderMapService](injector)
			provider, exists := providerSvc.GetProvider("kimi")
			require.True(t, exists)
			assert.IsType(t, (*providers.MoonshotProvider)(nil), provider)

			body := fmt.Sprintf(moonshotRequest, testCase.stream)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/messages", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Anthropic-Version", "2023-06-01")
			req.Header.Set("Authorization", "Bearer client-token")
			req.Header.Set("x-api-key", "client-key")
			recorder := httptest.NewRecorder()
			handlerSvc.Handler.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, testCase.contentType, recorder.Header().Get("Content-Type"))
			assert.Equal(t, testCase.reply, recorder.Body.String())
			if testCase.stream {
				assert.True(t, recorder.Flushed, "SSE responses should flush immediately")
			}

			select {
			case upstreamReq := <-seen:
				assert.Equal(t, "http://moonshot.example/anthropic/v1/messages", upstreamReq.URL.String())
				assert.Equal(t, http.MethodPost, upstreamReq.Method)
				assert.Equal(t, "Bearer moonshot-test-key", upstreamReq.Header.Get("Authorization"))
				assert.Empty(t, upstreamReq.Header.Get("x-api-key"))
				assert.Empty(t, upstreamReq.Header.Get("X-Selected-Key"))
				assert.Equal(t, "2023-06-01", upstreamReq.Header.Get("Anthropic-Version"))
				assert.JSONEq(t, strings.ReplaceAll(body, "claude-opus-4-6", "kimi-k3.1-pro"), <-bodies)
			default:
				t.Fatal("request did not reach the configured outbound proxy")
			}
		})
	}
}
