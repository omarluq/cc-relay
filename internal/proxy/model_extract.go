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

// bodyTooLargeKey is used for storing the body too large error in request context.
type bodyTooLargeKey struct{}

// ExtractModelFromRequest reads the model field from the request body.
// Returns mo.None if body is missing, malformed, or has no model field.
// Returns mo.Some with the model name if extraction succeeds.
// The request body is restored for subsequent reads.
//
// If the body exceeds max_body_bytes limit (set via http.MaxBytesReader),
// ExtractModelFromRequest extracts the "model" field from the JSON request body and restores the body for downstream use.
// It returns mo.Some(model) when a non-empty "model" string is present, or mo.None when extraction fails or the field is absent.
// For callers that need to distinguish a body-too-large read error, use ExtractModelWithBodyCheck.
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
// ExtractModelWithBodyCheck extracts the "model" field from the request JSON body and restores the body so it remains usable by downstream handlers.
// It returns mo.Some(model) when a non-empty "model" string is present in the parsed JSON, otherwise mo.None[string]().
// The boolean return value is true only when reading the body failed due to a body-too-large error; on nil body, read errors other than size limits, JSON parse failures, or missing/empty "model", it returns false.
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

// MarkBodyTooLarge returns a copy of ctx with a boolean flag set indicating the request body was too large to read.
func MarkBodyTooLarge(ctx context.Context) context.Context {
	return context.WithValue(ctx, bodyTooLargeKey{}, true)
}

// IsBodyTooLargeFromContext reports whether the request context contains a flag indicating the request body was too large during extraction.
// It returns true if the flag is present and set to true, otherwise false.
func IsBodyTooLargeFromContext(ctx context.Context) bool {
	v, ok := ctx.Value(bodyTooLargeKey{}).(bool)
	return ok && v
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