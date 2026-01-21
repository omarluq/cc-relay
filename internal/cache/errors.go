package cache

import "errors"

// Standard errors for cache operations.
//
// Use errors.Is to check for these errors:
//
//	data, err := c.Get(ctx, key)
//	if errors.Is(err, cache.ErrNotFound) {
//		// handle cache miss
//	}
var (
	// ErrNotFound is returned when a key does not exist in the cache.
	ErrNotFound = errors.New("cache: key not found")

	// ErrClosed is returned when operations are attempted on a closed cache.
	ErrClosed = errors.New("cache: cache is closed")

	// ErrSerializationFailed is returned when value serialization fails.
	// This typically occurs when encoding or decoding cached values.
	ErrSerializationFailed = errors.New("cache: serialization failed")
)
