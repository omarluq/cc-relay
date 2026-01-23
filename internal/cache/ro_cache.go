// Package cache provides a unified caching interface for cc-relay.
// This file provides reactive cache operations using samber/ro.
//
// ROCache wraps the existing Cache interface with reactive stream support.
// It is an ALTERNATIVE to direct Cache usage, not a replacement.
// Use ROCache for stream-based workflows where reactive patterns fit.
// Use direct Cache methods for simple synchronous operations.
//
// When to use ROCache:
//   - Caching results of stream processing
//   - Reactive get-or-fetch patterns
//   - Observable-based data pipelines
//   - Event-driven cache invalidation
//
// When to use Cache directly:
//   - Simple key-value operations
//   - Synchronous request handlers
//   - Direct get/set without streaming
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/samber/ro"
)

// ErrCacheFetchFailed is returned when a fetch operation fails.
var ErrCacheFetchFailed = errors.New("cache: fetch operation failed")

// ROCache provides reactive cache operations wrapping an existing Cache.
// It enables observable-based caching patterns for stream processing.
//
// Note: Since the ro hot cache plugin doesn't exist in v0.2.0, this
// implementation wraps the existing Cache interface using pure ro.
type ROCache struct {
	cache Cache
	ttl   time.Duration
}

// NewROCache creates a reactive cache wrapper around an existing Cache.
//
// Parameters:
//   - cache: the underlying cache implementation
//   - defaultTTL: default time-to-live for cached items (used by GetOrFetch)
//
// Example:
//
//	underlying, _ := cache.New(ctx, cfg)
//	roCache := cache.NewROCache(underlying, 5*time.Minute)
func NewROCache(cache Cache, defaultTTL time.Duration) *ROCache {
	return &ROCache{
		cache: cache,
		ttl:   defaultTTL,
	}
}

// GetOrFetch returns a cached value or fetches from the source observable.
// If the key exists in cache, returns immediately with the cached value.
// Otherwise, subscribes to the fetch observable, caches the result, and returns it.
//
// The fetched value is serialized to JSON for storage. Types must be JSON-serializable.
//
// Example:
//
//	result := roCache.GetOrFetch(
//	    ctx,
//	    "user:123",
//	    func() ro.Observable[User] {
//	        return fetchUserFromAPI(123)
//	    },
//	)
//	// Will use cache if available, otherwise fetch and cache
func (c *ROCache) GetOrFetch(
	ctx context.Context,
	key string,
	fetch func() ro.Observable[[]byte],
) ro.Observable[[]byte] {
	return ro.NewObservable(func(observer ro.Observer[[]byte]) ro.Teardown {
		// Try cache first
		data, err := c.cache.Get(ctx, key)
		if err == nil {
			observer.Next(data)
			observer.Complete()
			return nil
		}

		// Cache miss - fetch and cache
		if !errors.Is(err, ErrNotFound) {
			// Unexpected error
			observer.Error(err)
			return nil
		}

		// Subscribe to fetch observable
		var fetchErr error
		var result []byte

		fetch().Subscribe(ro.NewObserver(
			func(data []byte) {
				result = data
			},
			func(err error) {
				fetchErr = err
			},
			func() {},
		))

		if fetchErr != nil {
			observer.Error(fetchErr)
			return nil
		}

		// Cache the result (ignore errors - fetched data is still valid)
		//nolint:errcheck // Non-critical: caching failure doesn't affect returned data
		c.cache.SetWithTTL(ctx, key, result, c.ttl)

		observer.Next(result)
		observer.Complete()
		return nil
	})
}

// GetOrFetchTyped is like GetOrFetch but handles JSON serialization automatically.
// The type T must be JSON-serializable.
//
// Example:
//
//	result := GetOrFetchTyped[User](
//	    ctx,
//	    roCache,
//	    "user:123",
//	    func() ro.Observable[User] {
//	        return fetchUserFromAPI(123)
//	    },
//	)
func GetOrFetchTyped[T any](
	ctx context.Context,
	c *ROCache,
	key string,
	fetch func() ro.Observable[T],
) ro.Observable[T] {
	return ro.NewObservable(func(observer ro.Observer[T]) ro.Teardown {
		// Try cache first
		data, err := c.cache.Get(ctx, key)
		if err == nil {
			var result T
			if unmarshalErr := json.Unmarshal(data, &result); unmarshalErr == nil {
				observer.Next(result)
				observer.Complete()
				return nil
			}
			// Unmarshal failed - treat as cache miss and refetch
		}

		// Cache miss or unmarshal error - fetch and cache
		if err != nil && !errors.Is(err, ErrNotFound) {
			// Unexpected cache error
			observer.Error(err)
			return nil
		}

		// Subscribe to fetch observable
		var fetchErr error
		var result T
		var hasResult bool

		fetch().Subscribe(ro.NewObserver(
			func(val T) {
				result = val
				hasResult = true
			},
			func(err error) {
				fetchErr = err
			},
			func() {},
		))

		if fetchErr != nil {
			observer.Error(fetchErr)
			return nil
		}

		if !hasResult {
			observer.Error(ErrCacheFetchFailed)
			return nil
		}

		// Cache the result
		if data, marshalErr := json.Marshal(result); marshalErr == nil {
			//nolint:errcheck // Non-critical: caching failure doesn't affect returned data
			c.cache.SetWithTTL(ctx, key, data, c.ttl)
		}

		observer.Next(result)
		observer.Complete()
		return nil
	})
}

// Invalidate removes a key from the cache.
// Returns an observable that completes when invalidation is done.
//
// Example:
//
//	roCache.Invalidate(ctx, "user:123").Subscribe(...)
func (c *ROCache) Invalidate(ctx context.Context, key string) ro.Observable[struct{}] {
	return ro.NewObservable(func(observer ro.Observer[struct{}]) ro.Teardown {
		if err := c.cache.Delete(ctx, key); err != nil {
			observer.Error(err)
			return nil
		}
		observer.Complete()
		return nil
	})
}

// InvalidateMany removes multiple keys from the cache.
// Returns an observable that emits each successfully invalidated key.
//
// Example:
//
//	keys := []string{"user:1", "user:2", "user:3"}
//	roCache.InvalidateMany(ctx, keys).Subscribe(ro.OnNext(func(key string) {
//	    log.Info().Str("key", key).Msg("invalidated")
//	}))
func (c *ROCache) InvalidateMany(ctx context.Context, keys []string) ro.Observable[string] {
	return ro.NewObservable(func(observer ro.Observer[string]) ro.Teardown {
		for _, key := range keys {
			if err := c.cache.Delete(ctx, key); err != nil {
				// Non-existent key is not an error
				if !errors.Is(err, ErrNotFound) {
					observer.Error(err)
					return nil
				}
			}
			observer.Next(key)
		}
		observer.Complete()
		return nil
	})
}

// Set stores a value in the cache as an observable operation.
// Returns an observable that completes when the value is stored.
//
// Example:
//
//	roCache.Set(ctx, "key", []byte("value")).Subscribe(...)
func (c *ROCache) Set(ctx context.Context, key string, value []byte) ro.Observable[struct{}] {
	return ro.NewObservable(func(observer ro.Observer[struct{}]) ro.Teardown {
		if err := c.cache.SetWithTTL(ctx, key, value, c.ttl); err != nil {
			observer.Error(err)
			return nil
		}
		observer.Complete()
		return nil
	})
}

// SetWithTTL stores a value with a custom TTL as an observable operation.
func (c *ROCache) SetWithTTL(
	ctx context.Context,
	key string,
	value []byte,
	ttl time.Duration,
) ro.Observable[struct{}] {
	return ro.NewObservable(func(observer ro.Observer[struct{}]) ro.Teardown {
		if err := c.cache.SetWithTTL(ctx, key, value, ttl); err != nil {
			observer.Error(err)
			return nil
		}
		observer.Complete()
		return nil
	})
}

// Get retrieves a value from the cache as an observable.
// Emits the value and completes, or errors if not found.
//
// Example:
//
//	roCache.Get(ctx, "key").Subscribe(
//	    ro.OnNext(func(data []byte) { process(data) }),
//	    ro.OnError(func(err error) { handleError(err) }),
//	)
func (c *ROCache) Get(ctx context.Context, key string) ro.Observable[[]byte] {
	return ro.NewObservable(func(observer ro.Observer[[]byte]) ro.Teardown {
		data, err := c.cache.Get(ctx, key)
		if err != nil {
			observer.Error(err)
			return nil
		}
		observer.Next(data)
		observer.Complete()
		return nil
	})
}

// Exists checks if a key exists in the cache as an observable.
// Emits true/false and completes.
func (c *ROCache) Exists(ctx context.Context, key string) ro.Observable[bool] {
	return ro.NewObservable(func(observer ro.Observer[bool]) ro.Teardown {
		exists, err := c.cache.Exists(ctx, key)
		if err != nil {
			observer.Error(err)
			return nil
		}
		observer.Next(exists)
		observer.Complete()
		return nil
	})
}

// GetTTL returns the default TTL for this cache.
func (c *ROCache) GetTTL() time.Duration {
	return c.ttl
}

// Underlying returns the wrapped Cache interface.
// Use this when you need direct cache access for operations
// not available through the reactive interface.
func (c *ROCache) Underlying() Cache {
	return c.cache
}

// Stream caches all items from a source observable.
// Each item is stored with a key generated by the keyFunc.
// The stream passes through items unchanged.
//
// Example:
//
//	// Cache all users as they flow through
//	users := fetchAllUsers()
//	cached := Stream(ctx, roCache, users, func(u User) string {
//	    return fmt.Sprintf("user:%d", u.ID)
//	})
func Stream[T any](
	ctx context.Context,
	c *ROCache,
	source ro.Observable[T],
	keyFunc func(T) string,
) ro.Observable[T] {
	return ro.Pipe1(
		source,
		ro.DoOnNext(func(item T) {
			data, err := json.Marshal(item)
			if err == nil {
				//nolint:errcheck // Non-critical: caching failure doesn't affect stream
				c.cache.SetWithTTL(ctx, keyFunc(item), data, c.ttl)
			}
		}),
	)
}
