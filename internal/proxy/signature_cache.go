// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/omarluq/cc-relay/internal/cache"
)

const (
	// SignatureCacheTTL is the TTL for cached signatures (3 hours, matching CLIProxyAPI).
	SignatureCacheTTL = 3 * time.Hour

	// SignatureHashLen is the number of hex characters to use from SHA256 hash.
	SignatureHashLen = 16

	// MinSignatureLen is the minimum length for a valid signature.
	MinSignatureLen = 50

	// GeminiSignatureSentinel is a special sentinel value for Gemini models.
	GeminiSignatureSentinel = "skip_thought_signature_validator"
)

// GetModelGroup returns the model group for signature sharing.
// Models in the same group share signatures (e.g., claude-sonnet-4, claude-3-opus → "claude").
func GetModelGroup(modelName string) string {
	modelLower := strings.ToLower(modelName)
	switch {
	case strings.Contains(modelLower, "claude"):
		return "claude"
	case strings.Contains(modelLower, "gpt"):
		return "gpt"
	case strings.Contains(modelLower, "gemini"):
		return "gemini"
	default:
		return modelName // Fallback to exact model name
	}
}

// SignatureCache provides thread-safe caching of thinking block signatures.
// Uses cc-relay's cache.Cache interface for storage.
type SignatureCache struct {
	cache cache.Cache
}

// NewSignatureCache creates a new signature cache using the provided cache backend.
// Returns nil if the cache is nil (no-op mode).
func NewSignatureCache(c cache.Cache) *SignatureCache {
	if c == nil {
		return nil
	}
	return &SignatureCache{cache: c}
}

// cacheKey builds the cache key: "sig:{modelGroup}:{textHash}".
func (sc *SignatureCache) cacheKey(modelGroup, text string) string {
	h := sha256.Sum256([]byte(text))
	textHash := hex.EncodeToString(h[:])[:SignatureHashLen]
	return fmt.Sprintf("sig:%s:%s", modelGroup, textHash)
}

// Get retrieves a cached signature for the given model and text.
// Returns empty string on cache miss or error.
func (sc *SignatureCache) Get(ctx context.Context, modelName, text string) string {
	if sc == nil || sc.cache == nil {
		return ""
	}

	modelGroup := GetModelGroup(modelName)
	key := sc.cacheKey(modelGroup, text)

	data, err := sc.cache.Get(ctx, key)
	if err != nil {
		return ""
	}

	return string(data)
}

// Set caches a signature for the given model and text.
// Skips caching if signature is too short or cache is nil.
func (sc *SignatureCache) Set(ctx context.Context, modelName, text, signature string) {
	if sc == nil || sc.cache == nil {
		return
	}

	if !IsValidSignature(modelName, signature) {
		return
	}

	modelGroup := GetModelGroup(modelName)
	key := sc.cacheKey(modelGroup, text)

	//nolint:errcheck // Best effort caching - errors don't affect correctness
	sc.cache.SetWithTTL(ctx, key, []byte(signature), SignatureCacheTTL)
}

// IsValidSignature checks if a signature is valid (non-empty and long enough).
// Special case: "skip_thought_signature_validator" is valid only for Gemini models.
func IsValidSignature(modelName, signature string) bool {
	if signature == "" {
		return false
	}

	// Gemini sentinel is only valid for Gemini models
	if signature == GeminiSignatureSentinel {
		return GetModelGroup(modelName) == "gemini"
	}

	// Check minimum length
	return len(signature) >= MinSignatureLen
}
