// Package proxy implements the HTTP reverse proxy for Claude Code.
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/samber/mo"
)

// modelContextKey is used for storing the extracted model name in request context.
type modelContextKey struct{}

// ExtractModelFromRequest reads the model field from the request body.
// Returns mo.None if body is missing, malformed, or has no model field.
// Returns mo.Some with the model name if extraction succeeds.
// The request body is restored for subsequent reads.
//
// If the body exceeds max_body_bytes limit (set via http.MaxBytesReader),
// returns mo.None. Use ExtractModelWithBodyCheck for explicit error detection.
func ExtractModelFromRequest(r *http.Request) mo.Option[string] {
	model, _ := ExtractModelWithBodyCheck(r)
	return model
}

// ExtractModelWithBodyCheck reads the model field and reports body size errors.
// Returns:
//   - mo.Some[string] if model extraction succeeds
//   - mo.None[string] if body is missing, malformed, or has no model field
//   - bodyTooLarge=true if reading failed due to http.MaxBytesReader limit
//
// The request body is always restored for downstream use.
func ExtractModelWithBodyCheck(r *http.Request) (model mo.Option[string], bodyTooLarge bool) {
	if r.Body == nil {
		return mo.None[string](), false
	}

	bodyBytes, err := io.ReadAll(r.Body)
	closeBody(r.Body)

	// Always restore body for downstream use, even on partial read error
	// io.ReadAll may return partial bytes alongside an error
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	r.ContentLength = int64(len(bodyBytes))

	if err != nil {
		// Check if this is a body too large error
		if IsBodyTooLargeError(err) {
			return mo.None[string](), true
		}
		return mo.None[string](), false
	}

	// Parse JSON to get the model field
	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return mo.None[string](), false
	}

	modelStr, ok := body["model"].(string)
	if !ok || modelStr == "" {
		return mo.None[string](), false
	}

	return mo.Some(modelStr), false
}

// CacheModelInContext stores the extracted model name in the request context.
// This avoids re-reading the body when the model is needed again (e.g., for rewriting).
func CacheModelInContext(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, modelContextKey{}, model)
}

// GetModelFromContext retrieves the cached model name from the context.
// Returns the model and true if found, empty string and false otherwise.
func GetModelFromContext(ctx context.Context) (string, bool) {
	model, ok := ctx.Value(modelContextKey{}).(string)
	return model, ok
}
