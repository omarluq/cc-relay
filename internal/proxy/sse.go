// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"errors"
	"net/http"

	"github.com/omarluq/cc-relay/internal/providers"
)

// ErrStreamClosed is returned when attempting to read from a closed stream.
var ErrStreamClosed = errors.New("sse: stream is closed")

// SetSSEHeaders sets required headers for SSE streaming.
// These headers MUST be set for proper streaming through nginx/CDN:
//   - Content-Type: text/event-stream - SSE format
//   - Cache-Control: no-cache, no-transform - prevent caching
//   - X-Accel-Buffering: no - CRITICAL: disable nginx/Cloudflare buffering
//   - Connection: keep-alive - maintain streaming connection
func SetSSEHeaders(h http.Header) {
	h.Set("Content-Type", providers.ContentTypeSSE)
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("X-Accel-Buffering", "no")
	h.Set("Connection", "keep-alive")
}
