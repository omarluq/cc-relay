package ro

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/samber/ro"
)

// ShutdownSignals are the OS signals that trigger graceful shutdown.
var ShutdownSignals = []os.Signal{
	syscall.SIGINT,
	syscall.SIGTERM,
}

// GracefulShutdown creates an Observable that emits when a shutdown signal is received.
// The Observable emits the received signal and then completes.
//
// This is useful for coordinating graceful shutdown across multiple goroutines
// using reactive streams.
//
// Example:
//
//	shutdown := GracefulShutdown(ctx)
//	shutdown.Subscribe(ro.NewObserver(
//	    func(sig os.Signal) { log.Info().Msgf("received %v", sig) },
//	    func(err error) { log.Error().Err(err).Msg("shutdown error") },
//	    func() { log.Info().Msg("shutdown complete") },
//	))
func GracefulShutdown(ctx context.Context) ro.Observable[os.Signal] {
	return GracefulShutdownWithSignals(ctx, ShutdownSignals...)
}

// GracefulShutdownWithSignals creates an Observable that emits when any of the
// specified signals is received.
//
// Example:
//
//	// Only handle SIGTERM
//	shutdown := GracefulShutdownWithSignals(ctx, syscall.SIGTERM)
func GracefulShutdownWithSignals(parentCtx context.Context, signals ...os.Signal) ro.Observable[os.Signal] {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)

	return ro.NewObservableWithContext(func(ctx context.Context, observer ro.Observer[os.Signal]) ro.Teardown {
		// Use parent context for initial setup, then subscriber context for lifecycle
		_ = parentCtx // parentCtx used for Observable creation, ctx is subscriber context
		go func() {
			select {
			case sig := <-ch:
				observer.NextWithContext(ctx, sig)
				observer.CompleteWithContext(ctx)
			case <-ctx.Done():
				observer.ErrorWithContext(ctx, ctx.Err())
			}
		}()

		return func() {
			signal.Stop(ch)
			close(ch)
		}
	})
}

// WaitForShutdown blocks until a shutdown signal is received or context is canceled.
// Returns the received signal or an error if context was canceled.
//
// Example:
//
//	sig, err := WaitForShutdown(ctx)
//	if err != nil {
//	    return err
//	}
//	log.Info().Msgf("received %v, shutting down", sig)
func WaitForShutdown(ctx context.Context) (os.Signal, error) {
	results, _, err := ro.CollectWithContext(ctx, GracefulShutdown(ctx))
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, ctx.Err()
	}
	return results[0], nil
}

// OnShutdown registers a callback to be executed when a shutdown signal is received.
// Returns a Subscription that can be used to cancel the registration.
//
// Example:
//
//	sub := OnShutdown(ctx, func(sig os.Signal) {
//	    log.Info().Msgf("received %v, cleaning up...", sig)
//	    cleanup()
//	})
//	// Later: sub.Unsubscribe()
func OnShutdown(ctx context.Context, callback func(os.Signal)) ro.Subscription {
	return GracefulShutdown(ctx).SubscribeWithContext(ctx, ro.OnNextWithContext(func(_ context.Context, sig os.Signal) {
		callback(sig)
	}))
}
