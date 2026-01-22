// Package keypool provides key pooling and rate limit tracking for multi-key API management.
package keypool

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/omarluq/cc-relay/internal/ratelimit"
	"github.com/rs/zerolog/log"
)

// Common errors returned by KeyPool.
var (
	ErrKeyNotFound = errors.New("keypool: key not found")
)

// PoolConfig defines the configuration for a KeyPool.
type PoolConfig struct {
	// Strategy is the selection strategy name (least_loaded, round_robin, etc.)
	Strategy string `json:"strategy" yaml:"strategy"`

	// Keys are the API keys to pool
	Keys []KeyConfig `json:"keys" yaml:"keys"`
}

// KeyConfig defines the configuration for a single API key.
type KeyConfig struct {
	// APIKey is the actual API key value
	APIKey string `json:"api_key" yaml:"api_key"`

	// RPMLimit is the requests per minute limit (0 = unlimited, learn from headers)
	RPMLimit int `json:"rpm_limit" yaml:"rpm_limit"`

	// ITPMLimit is the input tokens per minute limit (0 = unlimited, learn from headers)
	ITPMLimit int `json:"itpm_limit" yaml:"itpm_limit"`

	// OTPMLimit is the output tokens per minute limit (0 = unlimited, learn from headers)
	OTPMLimit int `json:"otpm_limit" yaml:"otpm_limit"`

	// Priority is the key priority (0=low, 1=normal, 2=high)
	Priority int `json:"priority" yaml:"priority"`

	// Weight is the key weight for weighted selection strategies
	Weight int `json:"weight" yaml:"weight"`
}

// KeyPool manages multiple API keys with rate limiting and intelligent selection.
// All methods are safe for concurrent use.
type KeyPool struct {
	selector KeySelector
	keyMap   map[string]*KeyMetadata
	limiters map[string]ratelimit.RateLimiter
	provider string
	keys     []*KeyMetadata
	mu       sync.RWMutex
}

// NewKeyPool creates a new KeyPool with the given configuration.
// Returns an error if no keys are configured or the strategy is unknown.
func NewKeyPool(provider string, cfg PoolConfig) (*KeyPool, error) {
	if len(cfg.Keys) == 0 {
		return nil, fmt.Errorf("keypool: no keys configured for provider %s", provider)
	}

	// Create selector
	selector, err := NewSelector(cfg.Strategy)
	if err != nil {
		return nil, fmt.Errorf("keypool: failed to create selector: %w", err)
	}

	pool := &KeyPool{
		provider: provider,
		keys:     make([]*KeyMetadata, 0, len(cfg.Keys)),
		keyMap:   make(map[string]*KeyMetadata, len(cfg.Keys)),
		limiters: make(map[string]ratelimit.RateLimiter, len(cfg.Keys)),
		selector: selector,
	}

	// Initialize keys and limiters
	for i, keyCfg := range cfg.Keys {
		// Create key metadata
		key := NewKeyMetadata(keyCfg.APIKey, keyCfg.RPMLimit, keyCfg.ITPMLimit, keyCfg.OTPMLimit)

		// Set priority and weight
		if keyCfg.Priority > 0 {
			key.Priority = keyCfg.Priority
		}
		if keyCfg.Weight > 0 {
			key.Weight = keyCfg.Weight
		}

		// Create rate limiter
		limiter := ratelimit.NewTokenBucketLimiter(keyCfg.RPMLimit, keyCfg.ITPMLimit+keyCfg.OTPMLimit)

		// Store
		pool.keys = append(pool.keys, key)
		pool.keyMap[key.ID] = key
		pool.limiters[key.ID] = limiter

		log.Debug().
			Str("provider", provider).
			Str("key_id", key.ID).
			Int("index", i).
			Int("rpm_limit", keyCfg.RPMLimit).
			Int("itpm_limit", keyCfg.ITPMLimit).
			Int("otpm_limit", keyCfg.OTPMLimit).
			Int("priority", key.Priority).
			Int("weight", key.Weight).
			Msg("Initialized key in pool")
	}

	log.Info().
		Str("provider", provider).
		Int("num_keys", len(pool.keys)).
		Str("strategy", selector.Name()).
		Msg("Created key pool")

	return pool, nil
}

// GetKey selects the best available key from the pool using the configured strategy.
// Returns (keyID, apiKey, error).
// Returns ErrAllKeysExhausted if no keys have capacity.
func (p *KeyPool) GetKey(ctx context.Context) (keyID, apiKey string, err error) {
	p.mu.RLock()
	// Make a copy of keys slice for selector (avoid holding lock during selection)
	availableKeys := make([]*KeyMetadata, len(p.keys))
	copy(availableKeys, p.keys)
	p.mu.RUnlock()

	// Try to select a key, checking rate limiter for each attempt
	// Selector may return multiple candidates, try each until one has capacity
	maxAttempts := len(availableKeys)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Select key based on strategy
		key, err := p.selector.Select(availableKeys)
		if err != nil {
			// No keys available
			log.Warn().
				Str("provider", p.provider).
				Str("strategy", p.selector.Name()).
				Err(err).
				Msg("No keys available in pool")
			return "", "", err
		}

		// Check rate limiter
		p.mu.RLock()
		limiter := p.limiters[key.ID]
		p.mu.RUnlock()

		if limiter.Allow(ctx) {
			// Key has capacity
			log.Debug().
				Str("provider", p.provider).
				Str("key_id", key.ID).
				Int("attempt", attempt+1).
				Str("strategy", p.selector.Name()).
				Msg("Selected key from pool")
			return key.ID, key.APIKey, nil
		}

		// This key is rate limited, mark it and try next
		log.Debug().
			Str("provider", p.provider).
			Str("key_id", key.ID).
			Msg("Key rate limited, trying next")

		// Remove this key from available list and retry
		for i, k := range availableKeys {
			if k.ID == key.ID {
				availableKeys = append(availableKeys[:i], availableKeys[i+1:]...)
				break
			}
		}
	}

	// All keys exhausted
	log.Warn().
		Str("provider", p.provider).
		Int("num_keys", len(p.keys)).
		Msg("All keys exhausted in pool")
	return "", "", ErrAllKeysExhausted
}

// UpdateKeyFromHeaders updates a key's rate limit state from response headers.
// Returns ErrKeyNotFound if the key ID is not in the pool.
func (p *KeyPool) UpdateKeyFromHeaders(keyID string, headers http.Header) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	key, ok := p.keyMap[keyID]
	if !ok {
		return ErrKeyNotFound
	}

	// Update key metadata from headers
	err := key.UpdateFromHeaders(headers)
	if err != nil {
		log.Warn().
			Str("provider", p.provider).
			Str("key_id", keyID).
			Err(err).
			Msg("Failed to update key from headers")
		return err
	}

	// Update rate limiter if limits changed
	limiter := p.limiters[keyID]
	key.mu.RLock()
	rpm := key.RPMLimit
	tpm := key.ITPMLimit + key.OTPMLimit
	key.mu.RUnlock()

	limiter.SetLimit(rpm, tpm)

	log.Debug().
		Str("provider", p.provider).
		Str("key_id", keyID).
		Int("rpm_limit", rpm).
		Int("tpm_limit", tpm).
		Msg("Updated key from response headers")

	return nil
}

// MarkKeyExhausted marks a key as unavailable until the cooldown expires.
// Used when a 429 response includes a retry-after header.
func (p *KeyPool) MarkKeyExhausted(keyID string, retryAfter time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key, ok := p.keyMap[keyID]
	if !ok {
		log.Warn().
			Str("provider", p.provider).
			Str("key_id", keyID).
			Msg("Attempted to mark unknown key as exhausted")
		return
	}

	cooldownUntil := time.Now().Add(retryAfter)
	key.SetCooldown(cooldownUntil)

	log.Warn().
		Str("provider", p.provider).
		Str("key_id", keyID).
		Dur("retry_after", retryAfter).
		Time("cooldown_until", cooldownUntil).
		Msg("Key marked as exhausted with cooldown")
}

// GetEarliestResetTime returns the duration until the earliest rate limit reset.
// Used for setting retry-after headers when all keys are exhausted.
// Returns 60s default if no reset times are set.
func (p *KeyPool) GetEarliestResetTime() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var earliest time.Time
	for _, key := range p.keys {
		key.mu.RLock()
		resetAt := key.RPMResetAt
		key.mu.RUnlock()

		if resetAt.IsZero() {
			continue
		}

		if earliest.IsZero() || resetAt.Before(earliest) {
			earliest = resetAt
		}
	}

	if earliest.IsZero() {
		return 60 * time.Second // Default to 60s
	}

	duration := time.Until(earliest)
	if duration < 0 {
		return 0 // Already passed
	}

	return duration
}

// PoolStats contains statistics about the key pool.
type PoolStats struct {
	TotalKeys     int `json:"total_keys"`
	AvailableKeys int `json:"available_keys"`
	ExhaustedKeys int `json:"exhausted_keys"`
	TotalRPM      int `json:"total_rpm"`
	RemainingRPM  int `json:"remaining_rpm"`
}

// GetStats returns statistics about the current state of the pool.
func (p *KeyPool) GetStats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := PoolStats{
		TotalKeys: len(p.keys),
	}

	for _, key := range p.keys {
		key.mu.RLock()
		stats.TotalRPM += key.RPMLimit
		stats.RemainingRPM += key.RPMRemaining

		if key.IsAvailable() {
			stats.AvailableKeys++
		} else {
			stats.ExhaustedKeys++
		}
		key.mu.RUnlock()
	}

	return stats
}

// Keys returns a copy of the keys slice for external iteration.
// Callers can safely iterate over the returned slice.
func (p *KeyPool) Keys() []*KeyMetadata {
	p.mu.RLock()
	defer p.mu.RUnlock()

	keysCopy := make([]*KeyMetadata, len(p.keys))
	copy(keysCopy, p.keys)
	return keysCopy
}
