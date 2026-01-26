// Package keypool provides key pooling and rate limit tracking for multi-key API management.
//
// The keypool package enables intelligent selection of API keys from a pool based on
// remaining capacity and health status. It tracks rate limits (RPM, ITPM, OTPM) and
// cooldown periods, learning dynamically from provider response headers.
//
// Example usage:
//
//	key := keypool.NewKeyMetadata("sk-...", 50, 30000, 30000)
//	if key.IsAvailable() {
//	    // Use key for request
//	}
//	// After response
//	key.UpdateFromHeaders(resp.Header)
package keypool

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// KeyMetadata tracks rate limit state and health for a single API key.
// All methods are safe for concurrent use.
//
//nolint:govet // fieldalignment: struct ordered for clarity over memory optimization
type KeyMetadata struct {
	// Identity
	ID     string // Unique identifier (first 8 chars of key hash)
	APIKey string // The actual API key

	// Reset times - grouped for better alignment
	RPMResetAt    time.Time // When RPM resets
	ITPMResetAt   time.Time // When ITPM resets
	OTPMResetAt   time.Time // When OTPM resets
	LastErrorAt   time.Time // When last error occurred
	CooldownUntil time.Time // Don't use until this time (429 retry-after)

	// Health state
	LastError error // Last error encountered

	mu sync.RWMutex

	// Configured limits (from config or learned from headers)
	RPMLimit  int // Requests per minute limit
	ITPMLimit int // Input tokens per minute limit
	OTPMLimit int // Output tokens per minute limit

	// Current state (updated from response headers)
	RPMRemaining  int // Remaining requests this window
	ITPMRemaining int // Remaining input tokens
	OTPMRemaining int // Remaining output tokens

	// Priority (from config)
	Priority int // 0=low, 1=normal (default), 2=high
	Weight   int // For weighted selection strategy

	// Health state
	Healthy bool // Whether key is usable
}

// NewKeyMetadata creates a new KeyMetadata with the given API key and rate limits.
// The ID is generated from the first 8 characters of the SHA-256 hash of the key.
// Initial state: full capacity, healthy, normal priority.
//
// Note: The hash is for identification/logging only, NOT for security comparison.
// The key ID appears in logs for debugging purposes. It's not used for authentication.
// SHA-256 provides a stable, reproducible identifier from the key material.
func NewKeyMetadata(apiKey string, rpm, itpm, otpm int) *KeyMetadata {
	// Generate ID from hash of API key (for logging/identification, not security)
	// codeql[go/weak-sensitive-data-hashing] SHA-256 used for stable key identification, not security comparison
	// #nosec G401 -- SHA-256 used for stable key identification, not security comparison
	hash := sha256.Sum256([]byte(apiKey))
	id := hex.EncodeToString(hash[:])[:8]

	return &KeyMetadata{
		ID:            id,
		APIKey:        apiKey,
		RPMLimit:      rpm,
		ITPMLimit:     itpm,
		OTPMLimit:     otpm,
		RPMRemaining:  rpm,
		ITPMRemaining: itpm,
		OTPMRemaining: otpm,
		Healthy:       true,
		Priority:      1, // Normal priority
		Weight:        1, // Default weight
	}
}

// GetCapacityScore returns a 0-1 score representing remaining capacity.
// Higher score means more capacity available.
// Returns 0 if unhealthy or in cooldown.
func (k *KeyMetadata) GetCapacityScore() float64 {
	k.mu.RLock()
	defer k.mu.RUnlock()

	// Unavailable keys have 0 capacity
	if !k.Healthy || time.Now().Before(k.CooldownUntil) {
		return 0.0
	}

	// Calculate RPM score
	var rpmScore float64
	if k.RPMLimit > 0 {
		rpmScore = float64(k.RPMRemaining) / float64(k.RPMLimit)
	} else {
		rpmScore = 1.0 // Unlimited
	}

	// Calculate TPM score (combined input + output)
	var tpmScore float64
	totalTPMLimit := k.ITPMLimit + k.OTPMLimit
	totalTPMRemaining := k.ITPMRemaining + k.OTPMRemaining
	if totalTPMLimit > 0 {
		tpmScore = float64(totalTPMRemaining) / float64(totalTPMLimit)
	} else {
		tpmScore = 1.0 // Unlimited
	}

	// Average of both scores
	return (rpmScore + tpmScore) / 2.0
}

// IsAvailable returns true if the key is healthy and not in cooldown.
func (k *KeyMetadata) IsAvailable() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()

	return k.Healthy && time.Now().After(k.CooldownUntil)
}

// UpdateFromHeaders parses Anthropic rate limit headers and updates the key's state.
// Headers format:
//   - anthropic-ratelimit-requests-limit: 50
//   - anthropic-ratelimit-requests-remaining: 42
//   - anthropic-ratelimit-requests-reset: 2026-01-21T19:42:00Z
//   - anthropic-ratelimit-input-tokens-limit: 30000
//   - anthropic-ratelimit-input-tokens-remaining: 27000
//   - anthropic-ratelimit-output-tokens-limit: 30000
//   - anthropic-ratelimit-output-tokens-remaining: 27000
func (k *KeyMetadata) UpdateFromHeaders(headers http.Header) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.parseRPMLimits(headers)
	k.parseInputTokenLimits(headers)
	k.parseOutputTokenLimits(headers)

	return nil
}

// parseRPMLimits parses request rate limit headers.
//
//nolint:dupl // Similar pattern repeated for each token type
func (k *KeyMetadata) parseRPMLimits(headers http.Header) {
	if val := headers.Get("anthropic-ratelimit-requests-limit"); val != "" {
		if limit, err := strconv.Atoi(val); err == nil && limit > 0 {
			k.RPMLimit = limit
		}
	}

	if val := headers.Get("anthropic-ratelimit-requests-remaining"); val != "" {
		if remaining, err := strconv.Atoi(val); err == nil && remaining >= 0 {
			k.RPMRemaining = remaining
		}
	}

	if val := headers.Get("anthropic-ratelimit-requests-reset"); val != "" {
		if resetTime, err := time.Parse(time.RFC3339, val); err == nil {
			k.RPMResetAt = resetTime
		}
	}
}

// parseInputTokenLimits parses input token rate limit headers.
//
//nolint:dupl // Similar pattern repeated for each token type
func (k *KeyMetadata) parseInputTokenLimits(headers http.Header) {
	if val := headers.Get("anthropic-ratelimit-input-tokens-limit"); val != "" {
		if limit, err := strconv.Atoi(val); err == nil && limit > 0 {
			k.ITPMLimit = limit
		}
	}

	if val := headers.Get("anthropic-ratelimit-input-tokens-remaining"); val != "" {
		if remaining, err := strconv.Atoi(val); err == nil && remaining >= 0 {
			k.ITPMRemaining = remaining
		}
	}

	if val := headers.Get("anthropic-ratelimit-input-tokens-reset"); val != "" {
		if resetTime, err := time.Parse(time.RFC3339, val); err == nil {
			k.ITPMResetAt = resetTime
		}
	}
}

// parseOutputTokenLimits parses output token rate limit headers.
//
//nolint:dupl // Similar pattern repeated for each token type
func (k *KeyMetadata) parseOutputTokenLimits(headers http.Header) {
	if val := headers.Get("anthropic-ratelimit-output-tokens-limit"); val != "" {
		if limit, err := strconv.Atoi(val); err == nil && limit > 0 {
			k.OTPMLimit = limit
		}
	}

	if val := headers.Get("anthropic-ratelimit-output-tokens-remaining"); val != "" {
		if remaining, err := strconv.Atoi(val); err == nil && remaining >= 0 {
			k.OTPMRemaining = remaining
		}
	}

	if val := headers.Get("anthropic-ratelimit-output-tokens-reset"); val != "" {
		if resetTime, err := time.Parse(time.RFC3339, val); err == nil {
			k.OTPMResetAt = resetTime
		}
	}
}

// SetCooldown marks the key as unavailable until the specified time.
// Used when a 429 response includes a retry-after header.
func (k *KeyMetadata) SetCooldown(until time.Time) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.CooldownUntil = until
}

// MarkUnhealthy marks the key as unhealthy with the given error.
// Unhealthy keys are skipped during selection.
func (k *KeyMetadata) MarkUnhealthy(err error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.Healthy = false
	k.LastError = err
	k.LastErrorAt = time.Now()
}

// MarkHealthy marks the key as healthy and clears any previous error.
func (k *KeyMetadata) MarkHealthy() {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.Healthy = true
	k.LastError = nil
}

// String returns a human-readable representation of the key metadata.
func (k *KeyMetadata) String() string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	return fmt.Sprintf("Key[%s] rpm=%d/%d itpm=%d/%d otpm=%d/%d healthy=%v",
		k.ID, k.RPMRemaining, k.RPMLimit,
		k.ITPMRemaining, k.ITPMLimit,
		k.OTPMRemaining, k.OTPMLimit,
		k.Healthy)
}
