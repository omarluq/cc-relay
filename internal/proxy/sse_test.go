package proxy_test

import (
	"net/http"
	"testing"

	"github.com/omarluq/cc-relay/internal/proxy"
)

func TestProxy_SetSSEHeaders(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	proxy.SetSSEHeaders(headers)

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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := headers.Get(testCase.key)
			if got != testCase.expected {
				t.Errorf("Expected %s header to be %q, got %q", testCase.key, testCase.expected, got)
			}
		})
	}
}
