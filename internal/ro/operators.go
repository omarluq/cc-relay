package ro

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/samber/ro"
	rozerolog "github.com/samber/ro/plugins/observability/zerolog"
)

// Operator is a function that transforms an Observable[T] into an Observable[R].
// Operators can be chained together using ro.Pipe.
type Operator[T, R any] func(ro.Observable[T]) ro.Observable[R]

// LogEach logs each item that passes through the stream without modifying it.
// Uses the provided zerolog logger at Debug level.
//
// Example:
//
//	stream := ro.Pipe1(
//	    events,
//	    LogEach[Event](&logger, "sse-events"),
//	)
func LogEach[T any](logger *zerolog.Logger, name string) func(ro.Observable[T]) ro.Observable[T] {
	return ro.DoOnNext[T](func(item T) {
		logger.Debug().
			Interface("item", item).
			Str("stream", name).
			Msg("stream event")
	})
}

// LogWithLevel logs each item using the specified zerolog level.
// This is a thin wrapper around the ro zerolog plugin.
func LogWithLevel[T any](logger *zerolog.Logger, level zerolog.Level) func(ro.Observable[T]) ro.Observable[T] {
	return rozerolog.Log[T](logger, level)
}

// LogWithNotification logs all notifications (Next, Error, Complete) using the zerolog plugin.
func LogWithNotification[T any](logger *zerolog.Logger, level zerolog.Level) func(ro.Observable[T]) ro.Observable[T] {
	return rozerolog.LogWithNotification[T](logger, level)
}

// FatalOnError logs errors at Fatal level and exits.
// Use only for critical streams where errors should terminate the application.
func FatalOnError[T any](logger *zerolog.Logger) func(ro.Observable[T]) ro.Observable[T] {
	return rozerolog.FatalOnError[T](logger)
}

// WithTimeout adds a timeout to a stream. If no item is emitted within the
// specified duration, an error is raised.
//
// Example:
//
//	stream := ro.Pipe1(
//	    events,
//	    WithTimeout[Event](5*time.Second),
//	)
func WithTimeout[T any](timeout time.Duration) func(ro.Observable[T]) ro.Observable[T] {
	return ro.Timeout[T](timeout)
}

// WithRetry retries the source Observable if it errors.
// Useful for transient failures.
//
// Example:
//
//	stream := ro.Pipe1(
//	    fetchData,
//	    WithRetry[Data](),
//	)
func WithRetry[T any]() func(ro.Observable[T]) ro.Observable[T] {
	return ro.Retry[T]()
}

// WithRetryConfig retries with custom configuration.
func WithRetryConfig[T any](config ro.RetryConfig) func(ro.Observable[T]) ro.Observable[T] {
	return ro.RetryWithConfig[T](config)
}

// Catch handles errors in a stream by returning a fallback Observable.
//
// Example:
//
//	stream := ro.Pipe1(
//	    events,
//	    Catch(func(err error) ro.Observable[Event] {
//	        log.Error().Err(err).Msg("stream error")
//	        return ro.Just(fallbackEvent)
//	    }),
//	)
func Catch[T any](handler func(error) ro.Observable[T]) func(ro.Observable[T]) ro.Observable[T] {
	return ro.Catch(handler)
}

// DoOnNext performs a side effect for each item without modifying the stream.
func DoOnNext[T any](action func(T)) func(ro.Observable[T]) ro.Observable[T] {
	return ro.DoOnNext(action)
}

// DoOnError performs a side effect when an error occurs.
func DoOnError[T any](action func(error)) func(ro.Observable[T]) ro.Observable[T] {
	return ro.DoOnError[T](action)
}

// DoOnComplete performs a side effect when the stream completes.
func DoOnComplete[T any](action func()) func(ro.Observable[T]) ro.Observable[T] {
	return ro.DoOnComplete[T](action)
}

// DistinctValues removes duplicate items from a stream.
// Only works for comparable types.
func DistinctValues[T comparable]() func(ro.Observable[T]) ro.Observable[T] {
	return ro.Distinct[T]()
}

// DistinctBy removes duplicate items based on a key selector function.
func DistinctBy[T any, K comparable](keySelector func(T) K) func(ro.Observable[T]) ro.Observable[T] {
	return ro.DistinctBy(keySelector)
}

// SubscribeWithCallbacks creates an Observer with the provided callbacks and subscribes to the stream.
// Returns a Subscription that can be used to unsubscribe.
//
// Example:
//
//	sub := SubscribeWithCallbacks(
//	    stream,
//	    func(item Event) { process(item) },
//	    func(err error) { log.Error().Err(err).Msg("error") },
//	    func() { log.Info().Msg("complete") },
//	)
func SubscribeWithCallbacks[T any](
	source ro.Observable[T],
	onNext func(T),
	onError func(error),
	onComplete func(),
) ro.Subscription {
	observer := ro.NewObserver(onNext, onError, onComplete)
	return source.Subscribe(observer)
}

// SubscribeWithContext subscribes to a stream with context support.
func SubscribeWithContext[T any](
	ctx context.Context,
	source ro.Observable[T],
	onNext func(context.Context, T),
	onError func(context.Context, error),
	onComplete func(context.Context),
) ro.Subscription {
	observer := ro.NewObserverWithContext(onNext, onError, onComplete)
	return source.SubscribeWithContext(ctx, observer)
}
