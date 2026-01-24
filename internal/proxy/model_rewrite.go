// Package proxy implements the HTTP reverse proxy for Claude Code.
package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/rs/zerolog"
)

// ModelRewriter handles model name rewriting in request bodies.
type ModelRewriter struct {
	mapping map[string]string
}

// NewModelRewriter creates a new model rewriter with the given mapping.
// If mapping is nil or empty, the rewriter will pass through all models unchanged.
func NewModelRewriter(mapping map[string]string) *ModelRewriter {
	return &ModelRewriter{mapping: mapping}
}

// RewriteRequest rewrites the model field in the request body if a mapping exists.
// Returns the modified request with updated body if rewriting occurred.
// The original model name is logged for debugging purposes.
func (r *ModelRewriter) RewriteRequest(req *http.Request, logger *zerolog.Logger) error {
	// Skip if no mapping configured
	if len(r.mapping) == 0 {
		return nil
	}

	// Read the request body
	if req.Body == nil {
		return nil
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	//nolint:errcheck // Best effort close
	req.Body.Close()

	// Parse JSON to get the model field
	var body map[string]any
	if unmarshalErr := json.Unmarshal(bodyBytes, &body); unmarshalErr != nil {
		// Not valid JSON, restore body and return without modification (intentional)
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))
		return nil //nolint:nilerr // Returning nil is intentional - we gracefully degrade
	}

	// Get the model field
	modelField, ok := body["model"]
	if !ok {
		// No model field, restore body and return
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))
		return nil
	}

	originalModel, ok := modelField.(string)
	if !ok {
		// Model field is not a string, restore body and return
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))
		return nil
	}

	// Check if we have a mapping for this model
	mappedModel, found := r.mapping[originalModel]
	if !found {
		// No mapping for this model, restore body and return
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))
		return nil
	}

	// Log the rewrite
	if logger != nil {
		logger.Debug().
			Str("original_model", originalModel).
			Str("mapped_model", mappedModel).
			Msg("rewriting model name")
	}

	// Update the model field
	body["model"] = mappedModel

	// Re-encode the body
	newBodyBytes, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		// Failed to re-encode, restore original body (intentional graceful degradation)
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))
		return nil //nolint:nilerr // Returning nil is intentional - we gracefully degrade
	}

	// Replace request body with modified version
	req.Body = io.NopCloser(bytes.NewReader(newBodyBytes))
	req.ContentLength = int64(len(newBodyBytes))

	return nil
}

// RewriteModel maps a model name using the configured mapping.
// Returns the mapped name if found, otherwise returns the original unchanged.
func (r *ModelRewriter) RewriteModel(model string) string {
	if r.mapping == nil {
		return model
	}
	if mapped, ok := r.mapping[model]; ok {
		return mapped
	}
	return model
}

// HasMapping returns true if the rewriter has any mappings configured.
func (r *ModelRewriter) HasMapping() bool {
	return len(r.mapping) > 0
}
