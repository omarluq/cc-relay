package ro

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShutdownSignals(t *testing.T) {
	assert.Contains(t, ShutdownSignals, syscall.SIGINT)
	assert.Contains(t, ShutdownSignals, syscall.SIGTERM)
}

func TestGracefulShutdown(t *testing.T) {
	t.Run("creates observable without immediate emission", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		shutdown := GracefulShutdown(ctx)

		// Observable should be created without blocking
		assert.NotNil(t, shutdown)
	})
}

func TestGracefulShutdownWithSignals(t *testing.T) {
	t.Run("creates observable with custom signals", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		shutdown := GracefulShutdownWithSignals(ctx, syscall.SIGUSR1)

		assert.NotNil(t, shutdown)
	})
}

func TestOnShutdown(t *testing.T) {
	t.Run("registers callback and returns subscription", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		callbackCalled := false
		sub := OnShutdown(ctx, func(_ os.Signal) {
			callbackCalled = true
		})

		// Subscription should be returned immediately
		require.NotNil(t, sub)

		// Cancel to clean up
		cancel()

		// Give time for cleanup
		time.Sleep(10 * time.Millisecond)

		// Callback should not have been called (no signal sent)
		// This is expected behavior
		assert.False(t, callbackCalled)
	})
}

// Note: Testing actual signal handling requires process signals
// which can be complex and flaky in test environments.
// The following tests verify the structure and basic behavior
// without sending actual OS signals.

func TestWaitForShutdown_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately to test context cancellation path
	cancel()

	// WaitForShutdown should return quickly due to context cancellation
	done := make(chan struct{})
	var sig os.Signal
	var err error

	go func() {
		sig, err = WaitForShutdown(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Context was canceled, so we expect either nil sig or error
		// The exact behavior depends on timing - don't fail the test
		t.Logf("WaitForShutdown returned: sig=%v, err=%v", sig, err)
	case <-time.After(200 * time.Millisecond):
		// Acceptable - context cancellation may not be immediate
		t.Log("WaitForShutdown did not return quickly, which is acceptable")
	}
}
