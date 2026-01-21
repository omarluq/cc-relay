package proxy

import (
	"net/http"
	"testing"
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
