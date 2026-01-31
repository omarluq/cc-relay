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
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// KeyMetadata tracks rate limit state and health for a single API key.
// All methods are safe for concurrent use.
type KeyMetadata struct {
	RPMResetAt    time.Time
	ITPMResetAt   time.Time
	OTPMResetAt   time.Time
	LastErrorAt   time.Time
	CooldownUntil time.Time
	LastError     error
	APIKey        string
	ID            string
	RPMLimit      int
	ITPMLimit     int
	OTPMLimit     int
	RPMRemaining  int
	ITPMRemaining int
	OTPMRemaining int
	Priority      int
	Weight        int
	mu            sync.RWMutex
	Healthy       bool
}

// NewKeyMetadata creates a new KeyMetadata with the given API key and rate limits.
// The ID is generated from the first 8 characters of the FNV-1a hash of the key.
// Initial state: full capacity, healthy, normal priority.
//
// Note: The hash is for identification/logging only, NOT for security comparison.
// Remaining capacities are initialized to their respective limits. The key is marked healthy and given default Priority and Weight of 1.
func NewKeyMetadata(apiKey string, rpm, itpm, otpm int) *KeyMetadata {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(apiKey))
	id := hex.EncodeToString(hasher.Sum(nil))[:8]

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

	k.parseLimits(
		headers,
		"anthropic-ratelimit-requests-limit",
		"anthropic-ratelimit-requests-remaining",
		"anthropic-ratelimit-requests-reset",
		&k.RPMLimit,
		&k.RPMRemaining,
		&k.RPMResetAt,
	)
	k.parseLimits(
		headers,
		"anthropic-ratelimit-input-tokens-limit",
		"anthropic-ratelimit-input-tokens-remaining",
		"anthropic-ratelimit-input-tokens-reset",
		&k.ITPMLimit,
		&k.ITPMRemaining,
		&k.ITPMResetAt,
	)
	k.parseLimits(
		headers,
		"anthropic-ratelimit-output-tokens-limit",
		"anthropic-ratelimit-output-tokens-remaining",
		"anthropic-ratelimit-output-tokens-reset",
		&k.OTPMLimit,
		&k.OTPMRemaining,
		&k.OTPMResetAt,
	)

	return nil
}

func (k *KeyMetadata) parseLimits(
	headers http.Header,
	limitKey string,
	remainingKey string,
	resetKey string,
	limit *int,
	remaining *int,
	resetAt *time.Time,
) {
	if val := headers.Get(limitKey); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			*limit = parsed
		}
	}

	if val := headers.Get(remainingKey); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed >= 0 {
			*remaining = parsed
		}
	}

	if val := headers.Get(resetKey); val != "" {
		if parsed, err := time.Parse(time.RFC3339, val); err == nil {
			*resetAt = parsed
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