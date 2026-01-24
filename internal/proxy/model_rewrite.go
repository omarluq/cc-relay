// Package proxy implements the HTTP reverse proxy for Claude Code.
package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/samber/mo"
)

// Sentinel errors for rewrite pipeline.
var (
	errNoModelField   = errors.New("no model field in body")
	errModelNotString = errors.New("model field is not a string")
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

// rewriteResult holds the outcome of a rewrite attempt.
type rewriteResult struct {
	originalModel string
	mappedModel   string
	bodyBytes     []byte
	wasRewritten  bool
}

// RewriteRequest rewrites the model field in the request body if a mapping exists.
// Returns the modified request with updated body if rewriting occurred.
// The original model name is logged for debugging purposes.
// Uses mo.Result for railway-oriented error handling with centralized body restoration.
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

	// Try to rewrite using railway-oriented pipeline
	result := r.tryRewrite(bodyBytes)

	// Handle result using OrElse for graceful degradation
	res := result.OrElse(rewriteResult{
		bodyBytes:    bodyBytes,
		wasRewritten: false,
	})

	// Log successful rewrite
	if res.wasRewritten && logger != nil {
		logger.Debug().
			Str("original_model", res.originalModel).
			Str("mapped_model", res.mappedModel).
			Msg("rewriting model name")
	}

	// Restore the (possibly modified) body - single restoration point
	req.Body = io.NopCloser(bytes.NewReader(res.bodyBytes))
	req.ContentLength = int64(len(res.bodyBytes))

	return nil
}

// tryRewrite attempts the rewrite pipeline using mo.Result for clean error chaining.
func (r *ModelRewriter) tryRewrite(bodyBytes []byte) mo.Result[rewriteResult] {
	// Step 1: Parse JSON
	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return mo.Err[rewriteResult](err)
	}

	// Step 2: Get model field
	modelField, ok := body["model"]
	if !ok {
		return mo.Err[rewriteResult](errNoModelField)
	}

	// Step 3: Ensure model is a string
	originalModel, ok := modelField.(string)
	if !ok {
		return mo.Err[rewriteResult](errModelNotString)
	}

	// Step 4: Check for mapping
	mappedModel, found := r.mapping[originalModel]
	if !found {
		// No mapping - return original body unchanged (not an error, just no rewrite)
		return mo.Ok(rewriteResult{
			bodyBytes:    bodyBytes,
			wasRewritten: false,
		})
	}

	// Step 5: Apply mapping and re-encode
	body["model"] = mappedModel
	newBodyBytes, err := json.Marshal(body)
	if err != nil {
		return mo.Err[rewriteResult](err)
	}

	return mo.Ok(rewriteResult{
		bodyBytes:     newBodyBytes,
		wasRewritten:  true,
		originalModel: originalModel,
		mappedModel:   mappedModel,
	})
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
