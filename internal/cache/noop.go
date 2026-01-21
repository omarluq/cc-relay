package cache

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// noopCache is a no-op cache implementation that stores nothing.
// It is used when caching is disabled.
// All write operations succeed but do nothing.
// All read operations return ErrNotFound.
type noopCache struct {
	log    zerolog.Logger
	closed atomic.Bool
}

// newNoopCache creates a new no-op cache instance.
func newNoopCache() *noopCache {
	log := logger().With().Str("backend", "noop").Logger()
	log.Debug().Str("note", "caching is disabled").Msg("noop cache created")
	return &noopCache{
		log: log,
	}
}

// Get always returns ErrNotFound since noopCache stores nothing.
// Returns ErrClosed if the cache has been closed.
func (c *noopCache) Get(_ context.Context, key string) ([]byte, error) {
	if c.closed.Load() {
		return nil, ErrClosed
	}
	c.log.Debug().
		Str("key", key).
		Bool("hit", false).
		Msg("cache get")
	return nil, ErrNotFound
}

// Set is a no-op that always returns nil.
// Returns ErrClosed if the cache has been closed.
func (c *noopCache) Set(_ context.Context, key string, value []byte) error {
	if c.closed.Load() {
		return ErrClosed
	}
	c.log.Debug().
		Str("key", key).
		Int("size", len(value)).
		Msg("cache set")
	return nil
}

// SetWithTTL is a no-op that always returns nil.
// Returns ErrClosed if the cache has been closed.
func (c *noopCache) SetWithTTL(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if c.closed.Load() {
		return ErrClosed
	}
	c.log.Debug().
		Str("key", key).
		Int("size", len(value)).
		Dur("ttl", ttl).
		Msg("cache set")
	return nil
}

// Delete is a no-op that always returns nil.
// Returns ErrClosed if the cache has been closed.
func (c *noopCache) Delete(_ context.Context, key string) error {
	if c.closed.Load() {
		return ErrClosed
	}
	c.log.Debug().
		Str("key", key).
		Msg("cache delete")
	return nil
}

// Exists always returns false since noopCache stores nothing.
// Returns ErrClosed if the cache has been closed.
func (c *noopCache) Exists(_ context.Context, _ string) (bool, error) {
	if c.closed.Load() {
		return false, ErrClosed
	}
	return false, nil
}

// Close marks the cache as closed. It is idempotent.
func (c *noopCache) Close() error {
	if c.closed.Load() {
		return nil
	}
	c.closed.Store(true)
	c.log.Info().Msg("noop cache closed")
	return nil
}

// Stats returns zeroed cache statistics.
// The noopCache never stores anything, so all stats are zero.
func (c *noopCache) Stats() Stats {
	return Stats{}
}

// Compile-time interface checks ensure noopCache implements required interfaces.
var (
	_ Cache         = (*noopCache)(nil)
	_ StatsProvider = (*noopCache)(nil)
)
