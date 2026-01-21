// Package cache provides a unified caching interface for cc-relay.
//
// The cache package abstracts over three cache backends:
//   - Single mode (Ristretto): High-performance local in-memory cache
//   - HA mode (Olric): Distributed cache for high-availability deployments
//   - Disabled mode (Noop): Passthrough when caching is disabled
//
// All implementations are safe for concurrent use.
//
// Basic usage:
//
//	cfg := cache.Config{
//		Mode: cache.ModeSingle,
//		Ristretto: cache.RistrettoConfig{
//			NumCounters: 1e6,
//			MaxCost:     100 << 20, // 100 MB
//			BufferItems: 64,
//		},
//	}
//
//	c, err := cache.New(context.Background(), cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer c.Close()
//
//	// Store a value with TTL
//	err = c.SetWithTTL(ctx, "key", []byte("value"), 5*time.Minute)
//
//	// Retrieve a value
//	data, err := c.Get(ctx, "key")
//	if errors.Is(err, cache.ErrNotFound) {
//		// Cache miss
//	}
package cache

import (
	"context"
	"time"
)

// Cache defines the interface for cache operations.
// All implementations must be safe for concurrent use.
type Cache interface {
	// Get retrieves a value from the cache.
	// Returns ErrNotFound if the key does not exist.
	// Returns ErrClosed if the cache has been closed.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the cache with no expiration.
	// Returns ErrClosed if the cache has been closed.
	Set(ctx context.Context, key string, value []byte) error

	// SetWithTTL stores a value in the cache with a time-to-live.
	// After the TTL expires, the key will no longer be retrievable.
	// Returns ErrClosed if the cache has been closed.
	SetWithTTL(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a key from the cache.
	// Returns nil if the key does not exist (idempotent).
	// Returns ErrClosed if the cache has been closed.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache.
	// Returns ErrClosed if the cache has been closed.
	Exists(ctx context.Context, key string) (bool, error)

	// Close releases resources associated with the cache.
	// After Close is called, all operations will return ErrClosed.
	// Close is idempotent.
	Close() error
}

// Stats provides cache statistics for observability.
type Stats struct {
	// Hits is the number of cache hits.
	Hits uint64 `json:"hits"`

	// Misses is the number of cache misses.
	Misses uint64 `json:"misses"`

	// KeyCount is the current number of keys in the cache.
	KeyCount uint64 `json:"key_count"`

	// BytesUsed is the approximate memory used by cached values.
	BytesUsed uint64 `json:"bytes_used"`

	// Evictions is the number of keys evicted due to capacity limits.
	Evictions uint64 `json:"evictions"`
}

// StatsProvider is an optional interface for caches that support statistics.
// Use type assertion to check if a cache implements this interface:
//
//	if sp, ok := c.(cache.StatsProvider); ok {
//		stats := sp.Stats()
//		// use stats
//	}
type StatsProvider interface {
	// Stats returns current cache statistics.
	Stats() Stats
}

// Pinger is an optional interface for caches that support health checks.
// For local caches, Ping always returns nil.
// For distributed caches, Ping validates cluster connectivity.
//
// Use type assertion to check if a cache implements this interface:
//
//	if p, ok := c.(cache.Pinger); ok {
//		if err := p.Ping(ctx); err != nil {
//			// handle unhealthy cache
//		}
//	}
type Pinger interface {
	// Ping verifies the cache connection is alive.
	// For local caches, this always returns nil.
	// For distributed caches, this validates cluster connectivity.
	Ping(ctx context.Context) error
}

// MultiGetter is an optional interface for batch get operations.
// Implementations that support efficient batch retrieval should implement this.
//
// Use type assertion to check if a cache implements this interface:
//
//	if mg, ok := c.(cache.MultiGetter); ok {
//		results, err := mg.GetMulti(ctx, keys)
//		// use batch results
//	}
type MultiGetter interface {
	// GetMulti retrieves multiple values from the cache.
	// Missing keys are not included in the result map (no error).
	// Returns a map of key -> value for found keys.
	GetMulti(ctx context.Context, keys []string) (map[string][]byte, error)
}

// MultiSetter is an optional interface for batch set operations.
// Implementations that support efficient batch writes should implement this.
//
// Use type assertion to check if a cache implements this interface:
//
//	if ms, ok := c.(cache.MultiSetter); ok {
//		err := ms.SetMulti(ctx, items)
//		// handle batch write
//	}
type MultiSetter interface {
	// SetMulti stores multiple values in the cache.
	// If any key fails, returns error but may have set some keys.
	SetMulti(ctx context.Context, items map[string][]byte) error

	// SetMultiWithTTL stores multiple values with a common TTL.
	SetMultiWithTTL(ctx context.Context, items map[string][]byte, ttl time.Duration) error
}
