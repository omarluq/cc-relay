// Package ratelimit provides rate limiting interfaces and implementations for cc-relay.
// This file provides reactive rate limiting using samber/ro.
//
// Reactive rate limiting functions are an ALTERNATIVE to TokenBucket, not a replacement.
// Use reactive functions for stream processing scenarios.
// Use TokenBucket for synchronous request/response scenarios.
//
// When to use reactive rate limiting:
//   - Processing event streams (SSE, websockets)
//   - Batching rate-limited operations
//   - Complex async workflows with backpressure
//
// When to use TokenBucket:
//   - Simple request/response handlers
//   - Synchronous API calls
//   - Per-request rate checking
package ratelimit

import (
	"time"

	"github.com/samber/ro"
	roratelimit "github.com/samber/ro/plugins/ratelimit/native"
)

// ROLimiterConfig holds configuration for reactive rate limiting.
type ROLimiterConfig struct {
	// Count is the maximum number of items allowed per interval.
	Count int64

	// Interval is the time window for rate limiting.
	// Defaults to time.Minute if zero.
	Interval time.Duration
}

// DefaultInterval is the default rate limit interval (1 minute).
const DefaultInterval = time.Minute

// normalizeInterval returns the interval, defaulting to DefaultInterval if zero.
func normalizeInterval(interval time.Duration) time.Duration {
	if interval == 0 {
		return DefaultInterval
	}
	return interval
}

// Limit applies rate limiting to an observable stream using the ro native plugin.
// Items exceeding the rate limit will be delayed (backpressure).
//
// The keyGetter function extracts a key from each item for rate limiting.
// Items with the same key share a rate limit bucket.
// Use an empty string key for global rate limiting.
//
// Parameters:
//   - source: the observable stream to rate limit
//   - count: maximum items allowed per interval
//   - interval: time window for rate limiting (use DefaultInterval for 1 minute)
//   - keyGetter: extracts a key from each item for per-key rate limiting
//
// Example:
//
//	// Rate limit by API key
//	limited := ratelimit.Limit(requests, 100, time.Minute, func(r Request) string {
//	    return r.APIKey
//	})
//
//	// Global rate limit (all items share one bucket)
//	limited := ratelimit.Limit(events, 100, time.Minute, func(_ Event) string {
//	    return ""
//	})
func Limit[T any](
	source ro.Observable[T],
	count int64,
	interval time.Duration,
	keyGetter func(T) string,
) ro.Observable[T] {
	return ro.Pipe1(
		source,
		roratelimit.NewRateLimiter[T](count, normalizeInterval(interval), keyGetter),
	)
}

// LimitGlobal applies a global rate limit to all items in the stream.
// All items share a single rate limit bucket.
//
// This is a convenience function equivalent to:
//
//	Limit(source, count, interval, func(_ T) string { return "" })
//
// Example:
//
//	// Limit all events to 100 per minute
//	limited := ratelimit.LimitGlobal(events, 100, time.Minute)
func LimitGlobal[T any](
	source ro.Observable[T],
	count int64,
	interval time.Duration,
) ro.Observable[T] {
	return Limit(source, count, interval, func(_ T) string { return "" })
}

// LimitWithConfig applies rate limiting using an ROLimiterConfig.
//
// Example:
//
//	cfg := ratelimit.ROLimiterConfig{Count: 100, Interval: time.Minute}
//	limited := ratelimit.LimitWithConfig(events, cfg, func(e Event) string {
//	    return e.UserID
//	})
func LimitWithConfig[T any](
	source ro.Observable[T],
	cfg ROLimiterConfig,
	keyGetter func(T) string,
) ro.Observable[T] {
	return Limit(source, cfg.Count, cfg.Interval, keyGetter)
}

// NewLimitOperator creates a reusable rate limiter operator.
// This is useful when you need to apply the same rate limit to multiple streams.
//
// Example:
//
//	// Create a reusable operator
//	op := ratelimit.NewLimitOperator[Request](50, time.Minute, func(r Request) string {
//	    return r.UserID
//	})
//
//	// Apply to multiple streams
//	limited1 := ro.Pipe1(stream1, op)
//	limited2 := ro.Pipe1(stream2, op)
func NewLimitOperator[T any](
	count int64,
	interval time.Duration,
	keyGetter func(T) string,
) func(ro.Observable[T]) ro.Observable[T] {
	return roratelimit.NewRateLimiter[T](count, normalizeInterval(interval), keyGetter)
}

// NewGlobalLimitOperator creates a reusable global rate limiter operator.
// All items share a single rate limit bucket.
//
// Example:
//
//	op := ratelimit.NewGlobalLimitOperator[Event](100, time.Minute)
//	limited := ro.Pipe1(events, op)
func NewGlobalLimitOperator[T any](
	count int64,
	interval time.Duration,
) func(ro.Observable[T]) ro.Observable[T] {
	return NewLimitOperator[T](count, interval, func(_ T) string { return "" })
}
