// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

// IsStreamingRequest checks if request body contains "stream": true.
// Returns false if the body is invalid JSON or stream field is missing/false.
func IsStreamingRequest(body []byte) bool {
	// Parse as map to check stream field
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}

	stream, ok := req["stream"].(bool)

	return ok && stream
}

// SetSSEHeaders sets required headers for SSE streaming.
// These headers MUST be set for proper streaming through nginx/CDN:
//   - Content-Type: text/event-stream - SSE format
//   - Cache-Control: no-cache, no-transform - prevent caching
//   - X-Accel-Buffering: no - CRITICAL: disable nginx/Cloudflare buffering
//   - Connection: keep-alive - maintain streaming connection
func SetSSEHeaders(h http.Header) {
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("X-Accel-Buffering", "no")
	h.Set("Connection", "keep-alive")
}

// SSE event type markers for thinking signature processing.
var (
	thinkingDeltaMarker  = []byte(`"type":"thinking_delta"`)
	signatureDeltaMarker = []byte(`"type":"signature_delta"`)
	eventDataPrefix      = []byte("data:")
)

// SSESignatureProcessor handles signature processing for SSE events.
// Accumulates thinking text and caches signatures as they stream.
type SSESignatureProcessor struct {
	cache            *SignatureCache
	modelName        string
	currentSignature string
	thinkingText     strings.Builder
}

// NewSSESignatureProcessor creates a new SSE signature processor.
func NewSSESignatureProcessor(cache *SignatureCache, modelName string) *SSESignatureProcessor {
	return &SSESignatureProcessor{
		cache:     cache,
		modelName: modelName,
	}
}

// ProcessEvent processes a single SSE event line.
// Accumulates thinking text from thinking_delta events.
// Caches and transforms signatures from signature_delta events.
// Returns the potentially modified event data.
func (p *SSESignatureProcessor) ProcessEvent(ctx context.Context, eventData []byte) []byte {
	// Fast path: check if this is a content_block_delta event
	if !bytes.Contains(eventData, eventDataPrefix) {
		return eventData
	}

	// Extract data field from SSE event
	data := extractSSEData(eventData)
	if data == nil {
		return eventData
	}

	// Check for thinking_delta to accumulate text
	if bytes.Contains(data, thinkingDeltaMarker) {
		thinking := gjson.GetBytes(data, "delta.thinking").String()
		p.thinkingText.WriteString(thinking)
		return eventData
	}

	// Check for signature_delta to cache and transform
	if bytes.Contains(data, signatureDeltaMarker) {
		return p.processSignatureDelta(ctx, eventData, data)
	}

	return eventData
}

// processSignatureDelta handles signature_delta events.
func (p *SSESignatureProcessor) processSignatureDelta(
	ctx context.Context, eventData, data []byte,
) []byte {
	// Extract signature from delta
	signature := gjson.GetBytes(data, "delta.signature").String()
	if signature == "" {
		return eventData
	}

	// Cache the signature with accumulated thinking text
	thinkingText := p.thinkingText.String()
	if p.cache != nil && thinkingText != "" {
		p.cache.Set(ctx, p.modelName, thinkingText, signature)
	}
	p.currentSignature = signature
	p.thinkingText.Reset()

	// Transform signature to include model group prefix
	// Re-wrap in SSE format since ProcessResponseSignature returns raw JSON
	modifiedData := ProcessResponseSignature(ctx, data, thinkingText, p.modelName, nil)
	return append([]byte("data: "), modifiedData...)
}

// GetCurrentSignature returns the last processed signature.
func (p *SSESignatureProcessor) GetCurrentSignature() string {
	return p.currentSignature
}

// extractSSEData extracts the data field from an SSE event line.
// Returns nil if no data field is found.
func extractSSEData(eventLine []byte) []byte {
	// Find "data:" prefix
	idx := bytes.Index(eventLine, eventDataPrefix)
	if idx == -1 {
		return nil
	}

	// Extract data after "data:" prefix, trim spaces
	data := bytes.TrimSpace(eventLine[idx+len(eventDataPrefix):])

	// Handle multi-line data (lines joined by \n)
	if bytes.HasSuffix(data, []byte("\n")) {
		data = bytes.TrimSuffix(data, []byte("\n"))
	}

	return data
}
