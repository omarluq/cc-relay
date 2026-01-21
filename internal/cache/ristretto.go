package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/rs/zerolog"
)

// ristrettoCache implements Cache using Ristretto as the backend.
// It provides high-performance local in-memory caching with automatic
// admission policy based on access frequency.
type ristrettoCache struct {
	cache  *ristretto.Cache[string, []byte]
	log    zerolog.Logger
	closed atomic.Bool
	mu     sync.RWMutex
}

// Ensure ristrettoCache implements the required interfaces.
var (
	_ Cache         = (*ristrettoCache)(nil)
	_ StatsProvider = (*ristrettoCache)(nil)
)

// newRistrettoCache creates a new Ristretto cache with the given configuration.
func newRistrettoCache(cfg RistrettoConfig) (*ristrettoCache, error) {
	log := logger().With().Str("backend", "ristretto").Logger()

	bufferItems := cfg.BufferItems
	if bufferItems <= 0 {
		bufferItems = 64 // default buffer items
	}

	cache, err := ristretto.NewCache(&ristretto.Config[string, []byte]{
		NumCounters: cfg.NumCounters,
		MaxCost:     cfg.MaxCost,
		BufferItems: bufferItems,
		Metrics:     true, // enable stats
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to create ristretto cache")
		return nil, err
	}

	log.Info().
		Int64("num_counters", cfg.NumCounters).
		Int64("max_cost", cfg.MaxCost).
		Int64("buffer_items", bufferItems).
		Msg("ristretto cache created")

	return &ristrettoCache{
		cache: cache,
		log:   log,
	}, nil
}

// Get retrieves a value from the cache.
// Returns ErrNotFound if the key does not exist.
// Returns ErrClosed if the cache has been closed.
func (r *ristrettoCache) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if r.closed.Load() {
		return nil, ErrClosed
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed.Load() {
		return nil, ErrClosed
	}

	value, found := r.cache.Get(key)
	if !found {
		r.log.Debug().
			Str("key", key).
			Bool("hit", false).
			Msg("cache get")
		return nil, ErrNotFound
	}

	r.log.Debug().
		Str("key", key).
		Bool("hit", true).
		Int("size", len(value)).
		Msg("cache get")

	// Return a copy to prevent mutation of cached data
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

// Set stores a value in the cache with no expiration.
// Returns ErrClosed if the cache has been closed.
func (r *ristrettoCache) Set(ctx context.Context, key string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if r.closed.Load() {
		return ErrClosed
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed.Load() {
		return ErrClosed
	}

	// Make a copy to prevent caller from mutating cached data
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	// Cost is the byte length of the value
	r.cache.Set(key, valueCopy, int64(len(value)))

	r.log.Debug().
		Str("key", key).
		Int("size", len(value)).
		Msg("cache set")

	return nil
}

// SetWithTTL stores a value in the cache with a time-to-live.
// After the TTL expires, the key will no longer be retrievable.
// Returns ErrClosed if the cache has been closed.
func (r *ristrettoCache) SetWithTTL(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if r.closed.Load() {
		return ErrClosed
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed.Load() {
		return ErrClosed
	}

	// Make a copy to prevent caller from mutating cached data
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	// Cost is the byte length of the value
	r.cache.SetWithTTL(key, valueCopy, int64(len(value)), ttl)

	r.log.Debug().
		Str("key", key).
		Int("size", len(value)).
		Dur("ttl", ttl).
		Msg("cache set")

	return nil
}

// Delete removes a key from the cache.
// Returns nil if the key does not exist (idempotent).
// Returns ErrClosed if the cache has been closed.
func (r *ristrettoCache) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if r.closed.Load() {
		return ErrClosed
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed.Load() {
		return ErrClosed
	}

	r.cache.Del(key)

	r.log.Debug().
		Str("key", key).
		Msg("cache delete")

	return nil
}

// Exists checks if a key exists in the cache.
// Returns ErrClosed if the cache has been closed.
func (r *ristrettoCache) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	if r.closed.Load() {
		return false, ErrClosed
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed.Load() {
		return false, ErrClosed
	}

	_, found := r.cache.Get(key)
	return found, nil
}

// Close releases resources associated with the cache.
// After Close is called, all operations will return ErrClosed.
// Close is idempotent.
func (r *ristrettoCache) Close() error {
	if r.closed.Load() {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed.Load() {
		return nil
	}

	r.closed.Store(true)

	// Wait for all pending writes to complete
	r.cache.Wait()

	// Close the cache
	r.cache.Close()

	r.log.Info().Msg("ristretto cache closed")

	return nil
}

// Stats returns current cache statistics.
func (r *ristrettoCache) Stats() Stats {
	if r.closed.Load() {
		return Stats{}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed.Load() {
		return Stats{}
	}

	metrics := r.cache.Metrics

	stats := Stats{
		Hits:      metrics.Hits(),
		Misses:    metrics.Misses(),
		KeyCount:  metrics.KeysAdded() - metrics.KeysEvicted(),
		BytesUsed: metrics.CostAdded() - metrics.CostEvicted(),
		Evictions: metrics.KeysEvicted(),
	}

	r.log.Debug().
		Uint64("hits", stats.Hits).
		Uint64("misses", stats.Misses).
		Uint64("key_count", stats.KeyCount).
		Uint64("bytes_used", stats.BytesUsed).
		Uint64("evictions", stats.Evictions).
		Msg("cache stats")

	return stats
}
