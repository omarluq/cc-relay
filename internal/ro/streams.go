// Package ro provides reactive stream utilities for cc-relay using samber/ro.
//
// IMPORTANT: samber/ro is v0.2.0 (pre-1.0 stability). Use cautiously.
// Monitor GitHub releases for breaking changes.
//
// Use this package when:
//   - Processing actual streams (SSE, websockets, file watching)
//   - Event-driven architectures
//   - Need operators like debounce, throttle, buffer
//   - Complex async coordination
//
// Do NOT use this package when:
//   - Simple request/response (use standard handlers)
//   - Synchronous operations (overhead not justified)
//   - Small, bounded data (use samber/lo instead)
//   - Critical hot paths (benchmark first)
package ro

import (
	"context"
	"time"

	"github.com/samber/ro"
)

// StreamFromChannel creates an Observable from a receive-only channel.
// When the channel is closed, the Observable completes.
//
// Example:
//
//	events := make(chan Event)
//	stream := StreamFromChannel(events)
//	stream.Subscribe(ro.OnNext(func(e Event) { process(e) }))
func StreamFromChannel[T any](ch <-chan T) ro.Observable[T] {
	return ro.FromChannel(ch)
}

// StreamFromSlice creates an Observable from a slice.
// Items are emitted in order, then the Observable completes.
//
// Note: For bounded data transformations, prefer samber/lo functions
// (lo.Filter, lo.Map) over creating streams.
//
// Example:
//
//	items := []Event{event1, event2, event3}
//	stream := StreamFromSlice(items)
func StreamFromSlice[T any](items []T) ro.Observable[T] {
	return ro.FromSlice(items)
}

// Just creates an Observable that emits the provided values and completes.
//
// Example:
//
//	single := Just(config)
//	multiple := Just(1, 2, 3)
func Just[T any](values ...T) ro.Observable[T] {
	return ro.Just(values...)
}

// Empty creates an Observable that emits no items and completes immediately.
func Empty[T any]() ro.Observable[T] {
	return ro.Empty[T]()
}

// Throw creates an Observable that emits an error immediately.
func Throw[T any](err error) ro.Observable[T] {
	return ro.Throw[T](err)
}

// ProcessStream applies a standard pipeline of map and filter operations.
// This is a convenience function for common stream processing patterns.
//
// Example:
//
//	// Double all values and keep only those > 4
//	result := ProcessStream(
//	    StreamFromSlice([]int{1, 2, 3, 4, 5}),
//	    func(i int) int { return i * 2 },      // mapper
//	    func(i int) bool { return i > 4 },     // filter
//	)
//	// Result: 6, 8, 10
func ProcessStream[T, R any](
	source ro.Observable[T],
	mapper func(T) R,
	filter func(R) bool,
) ro.Observable[R] {
	return ro.Pipe2(
		source,
		ro.Map(mapper),
		ro.Filter(filter),
	)
}

// FilterStream filters items from a source Observable based on a predicate.
func FilterStream[T any](source ro.Observable[T], predicate func(T) bool) ro.Observable[T] {
	return ro.Pipe1(source, ro.Filter(predicate))
}

// MapStream transforms items from a source Observable using a mapper function.
func MapStream[T, R any](source ro.Observable[T], mapper func(T) R) ro.Observable[R] {
	return ro.Pipe1(source, ro.Map(mapper))
}

// TakeFirst takes only the first n items from a stream.
func TakeFirst[T any](source ro.Observable[T], count int64) ro.Observable[T] {
	return ro.Pipe1(source, ro.Take[T](count))
}

// SkipFirst skips the first n items from a stream.
func SkipFirst[T any](source ro.Observable[T], count int64) ro.Observable[T] {
	return ro.Pipe1(source, ro.Skip[T](count))
}

// MergeStreams merges multiple Observables into a single Observable.
// Items from all sources are interleaved as they arrive.
func MergeStreams[T any](sources ...ro.Observable[T]) ro.Observable[T] {
	return ro.Merge(sources...)
}

// ConcatStreams concatenates multiple Observables into a single Observable.
// Each source is subscribed to only after the previous one completes.
func ConcatStreams[T any](sources ...ro.Observable[T]) ro.Observable[T] {
	return ro.Concat(sources...)
}

// Collect collects all items from a stream into a slice.
// Blocks until the stream completes or errors.
//
// Example:
//
//	items, err := Collect(StreamFromSlice([]int{1, 2, 3}))
//	// items: []int{1, 2, 3}
func Collect[T any](source ro.Observable[T]) ([]T, error) {
	return ro.Collect(source)
}

// CollectWithContext collects all items from a stream with context support.
// The context can be used for cancellation.
func CollectWithContext[T any](ctx context.Context, source ro.Observable[T]) ([]T, context.Context, error) {
	return ro.CollectWithContext(ctx, source)
}

// BufferWithTime buffers items for a specified duration, then emits them as a slice.
// Useful for batching events.
//
// Example:
//
//	// Batch events every 100ms
//	batched := BufferWithTime(events, 100*time.Millisecond)
func BufferWithTime[T any](source ro.Observable[T], duration time.Duration) ro.Observable[[]T] {
	return ro.Pipe1(source, ro.BufferWithTime[T](duration))
}

// BufferWithCount buffers items until a count is reached, then emits them as a slice.
//
// Example:
//
//	// Batch events every 10 items
//	batched := BufferWithCount(events, 10)
func BufferWithCount[T any](source ro.Observable[T], count int) ro.Observable[[]T] {
	return ro.Pipe1(source, ro.BufferWithCount[T](count))
}

// BufferWithTimeOrCount buffers items until either duration or count is reached.
// Whichever comes first triggers the batch emission.
func BufferWithTimeOrCount[T any](source ro.Observable[T], count int, duration time.Duration) ro.Observable[[]T] {
	return ro.Pipe1(source, ro.BufferWithTimeOrCount[T](count, duration))
}
