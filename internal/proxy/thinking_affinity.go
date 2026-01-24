// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// thinkingAffinityContextKey is used for storing thinking affinity detection result in context.
type thinkingAffinityContextKey struct{}

// HasThinkingSignature checks if the request body contains thinking signatures
// in assistant messages. This indicates a conversation with extended thinking enabled,
// which requires sticky provider routing to avoid signature validation errors.
//
// When extended thinking is enabled, providers return a thinking content block
// with a provider-specific signature. On subsequent turns, this signature must
// be validated by the same provider. If requests are routed to a different
// provider (e.g., via round-robin), the signature validation fails.
//
// The request body is restored for subsequent reads.
func HasThinkingSignature(r *http.Request) bool {
	if r.Body == nil {
		return false
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	//nolint:errcheck // Best effort close
	r.Body.Close()

	// Always restore body for downstream use
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	r.ContentLength = int64(len(bodyBytes))

	// Parse JSON body
	var body struct {
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Type      string `json:"type"`
				Signature string `json:"signature,omitempty"`
			} `json:"content"`
		} `json:"messages"`
	}

	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return false
	}

	// Look for thinking blocks with signatures in assistant messages
	for _, msg := range body.Messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, content := range msg.Content {
			if content.Type == "thinking" && content.Signature != "" {
				return true
			}
		}
	}

	return false
}

// CacheThinkingAffinityInContext stores the thinking affinity detection result in context.
func CacheThinkingAffinityInContext(ctx context.Context, hasThinking bool) context.Context {
	return context.WithValue(ctx, thinkingAffinityContextKey{}, hasThinking)
}

// GetThinkingAffinityFromContext retrieves the cached thinking affinity result from context.
// Returns false if not cached.
func GetThinkingAffinityFromContext(ctx context.Context) bool {
	hasThinking, ok := ctx.Value(thinkingAffinityContextKey{}).(bool)
	if !ok {
		return false
	}
	return hasThinking
}
