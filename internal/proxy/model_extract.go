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
func ExtractModelFromRequest(r *http.Request) mo.Option[string] {
	if r.Body == nil {
		return mo.None[string]()
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return mo.None[string]()
	}
	//nolint:errcheck // Best effort close
	r.Body.Close()

	// Always restore body for downstream use
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	r.ContentLength = int64(len(bodyBytes))

	// Parse JSON to get the model field
	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return mo.None[string]()
	}

	model, ok := body["model"].(string)
	if !ok || model == "" {
		return mo.None[string]()
	}

	return mo.Some(model)
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
