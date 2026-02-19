package ratelimit_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/ratelimit"
)

func TestNewTokenBucketLimiter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		rpm     int
		tpm     int
		wantRPM int
		wantTPM int
		isUnlim bool
	}{
		{
			name:    "valid limits",
			rpm:     50,
			tpm:     30000,
			wantRPM: 50,
			wantTPM: 30000,
			isUnlim: false,
		},
		{
			name:    "zero rpm treated as unlimited",
			rpm:     0,
			tpm:     30000,
			wantRPM: 1_000_000,
			wantTPM: 30000,
			isUnlim: true,
		},
		{
			name:    "zero tpm treated as unlimited",
			rpm:     50,
			tpm:     0,
			wantRPM: 50,
			wantTPM: 1_000_000,
			isUnlim: true,
		},
		{
			name:    "negative values treated as unlimited",
			rpm:     -1,
			tpm:     -1,
			wantRPM: 1_000_000,
			wantTPM: 1_000_000,
			isUnlim: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			limiter := ratelimit.NewTokenBucketLimiter(testCase.rpm, testCase.tpm)
			if limiter == nil {
				t.Fatal("ratelimit.NewTokenBucketLimiter returned nil")
			}

			if limiter.GetRPMLimit() != testCase.wantRPM {
				t.Errorf("rpmLimit = %d, want %d", limiter.GetRPMLimit(), testCase.wantRPM)
			}
			if limiter.GetTPMLimit() != testCase.wantTPM {
				t.Errorf("tpmLimit = %d, want %d", limiter.GetTPMLimit(), testCase.wantTPM)
			}
		})
	}
}

func TestAllow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		rpm         int
		tpm         int
		numRequests int
		wantAllowed int
	}{
		{
			name:        "under limit",
			rpm:         10,
			tpm:         10000,
			numRequests: 5,
			wantAllowed: 5,
		},
		{
			name:        "at capacity",
			rpm:         5,
			tpm:         10000,
			numRequests: 10,
			wantAllowed: 5, // Burst allows 5 instantly
		},
		{
			name:        "unlimited rpm",
			rpm:         0,
			tpm:         10000,
			numRequests: 100,
			wantAllowed: 100,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			limiter := ratelimit.NewTokenBucketLimiter(testCase.rpm, testCase.tpm)
			ctx := context.Background()

			allowed := 0
			for reqIdx := 0; reqIdx < testCase.numRequests; reqIdx++ {
				if limiter.Allow(ctx) {
					allowed++
				}
			}

			if allowed != testCase.wantAllowed {
				t.Errorf("Allow() allowed %d requests, want %d", allowed, testCase.wantAllowed)
			}
		})
	}
}

func TestWait(t *testing.T) {
	t.Parallel()
	t.Run("blocks until capacity available", func(t *testing.T) {
		t.Parallel(
		// Use very low limit for fast test
		)

		limiter := ratelimit.NewTokenBucketLimiter(60, 10000) // 1 per second
		ctx := context.Background()

		// Exhaust the burst capacity (60 requests available immediately)
		for burstIdx := 0; burstIdx < 60; burstIdx++ {
			if err := limiter.Wait(ctx); err != nil {
				t.Fatalf("Wait() %d failed: %v", burstIdx, err)
			}
		}

		// Next request should block briefly then succeed
		start := time.Now()
		if err := limiter.Wait(ctx); err != nil {
			t.Fatalf("Wait() after burst failed: %v", err)
		}
		elapsed := time.Since(start)

		// Should have waited at least 500ms (conservative check)
		if elapsed < 500*time.Millisecond {
			t.Errorf("Wait() did not block long enough: %v", elapsed)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewTokenBucketLimiter(1, 10000) // Very low limit
		ctx, cancel := context.WithCancel(context.Background())

		// Exhaust capacity
		_ = limiter.Allow(ctx)

		// Cancel context and try to wait
		cancel()
		err := limiter.Wait(ctx)
		if !errors.Is(err, ratelimit.ErrContextCancelled) {
			t.Errorf("Wait() error = %v, want ratelimit.ErrContextCancelled", err)
		}
	})

	t.Run("respects context deadline", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewTokenBucketLimiter(1, 10000) // Very low limit
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Exhaust capacity
		_ = limiter.Allow(ctx)

		// Try to wait with short deadline
		err := limiter.Wait(ctx)
		if err == nil {
			t.Error("Wait() succeeded, want error")
		}
		// Either ErrContextCancelled or context deadline error is acceptable
	})
}

func TestSetLimitUpdates(t *testing.T) {
	t.Parallel()

	t.Run("updates limits dynamically", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewTokenBucketLimiter(10, 1000)

		// Update limits
		limiter.SetLimit(50, 5000)

		if limiter.GetRPMLimit() != 50 {
			t.Errorf("rpmLimit = %d, want 50", limiter.GetRPMLimit())
		}
		if limiter.GetTPMLimit() != 5000 {
			t.Errorf("tpmLimit = %d, want 5000", limiter.GetTPMLimit())
		}
	})

	t.Run("new limit takes effect immediately", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewTokenBucketLimiter(5, 1000)
		ctx := context.Background()

		// Exhaust initial limit
		for exhaustIdx := 0; exhaustIdx < 5; exhaustIdx++ {
			limiter.Allow(ctx)
		}

		// Should be rate limited now
		if limiter.Allow(ctx) {
			t.Error("Allow() succeeded after exhausting limit")
		}

		// Increase limit
		limiter.SetLimit(100, 10000)

		// Should now allow requests
		if !limiter.Allow(ctx) {
			t.Error("Allow() failed after increasing limit")
		}
	})
}

func TestSetLimitThreadSafety(t *testing.T) {
	t.Parallel()

	limiter := ratelimit.NewTokenBucketLimiter(100, 10000)
	ctx := context.Background()

	var waitGroup sync.WaitGroup
	errorsChan := make(chan error, 100)

	// Spawn multiple goroutines updating limits
	for goroutineIdx := 0; goroutineIdx < 10; goroutineIdx++ {
		waitGroup.Add(1)
		go func(iteration int) {
			defer waitGroup.Done()
			for step := 0; step < 10; step++ {
				limiter.SetLimit(50+iteration, 5000+iteration*1000)
				_ = limiter.Allow(ctx)
				usage := limiter.GetUsage()
				if usage.RequestsLimit <= 0 {
					errorsChan <- ratelimit.ErrRateLimitExceeded
					return
				}
			}
		}(goroutineIdx)
	}

	waitGroup.Wait()
	close(errorsChan)

	// Check for errors
	for err := range errorsChan {
		if err != nil {
			t.Errorf("concurrent SetLimit/Allow failed: %v", err)
		}
	}
}

func TestGetUsage(t *testing.T) {
	t.Parallel()
	t.Run("returns correct limits", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewTokenBucketLimiter(50, 30000)
		usage := limiter.GetUsage()

		if usage.RequestsLimit != 50 {
			t.Errorf("RequestsLimit = %d, want 50", usage.RequestsLimit)
		}
		if usage.TokensLimit != 30000 {
			t.Errorf("TokensLimit = %d, want 30000", usage.TokensLimit)
		}
	})

	t.Run("updates after Allow calls", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewTokenBucketLimiter(10, 10000)
		ctx := context.Background()

		// Exhaust all capacity
		for exhaustIdx := 0; exhaustIdx < 10; exhaustIdx++ {
			limiter.Allow(ctx)
		}

		// Get usage after exhaustion
		usage := limiter.GetUsage()

		// Should report that we're at or near capacity
		// Note: GetUsage is approximate due to token bucket refill
		if usage.RequestsRemaining > 5 {
			t.Errorf("RequestsRemaining = %d after exhausting capacity, want <= 5", usage.RequestsRemaining)
		}
	})
}

func TestConsumeTokens(t *testing.T) {
	t.Parallel()
	t.Run("records token usage correctly", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewTokenBucketLimiter(100, 1000) // 1000 TPM
		ctx := context.Background()

		// Consume some tokens (should succeed immediately with burst)
		err := limiter.ConsumeTokens(ctx, 500)
		if err != nil {
			t.Fatalf("ConsumeTokens(500) failed: %v", err)
		}

		// Consume more tokens
		err = limiter.ConsumeTokens(ctx, 300)
		if err != nil {
			t.Fatalf("ConsumeTokens(300) failed: %v", err)
		}
	})

	t.Run("blocks if over TPM limit", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewTokenBucketLimiter(100, 60) // 1 token per second
		ctx := context.Background()

		// Exhaust the burst capacity (60 tokens available immediately)
		err := limiter.ConsumeTokens(ctx, 60)
		if err != nil {
			t.Fatalf("ConsumeTokens(60) failed: %v", err)
		}

		// Next token should block
		start := time.Now()
		err = limiter.ConsumeTokens(ctx, 1)
		if err != nil {
			t.Fatalf("ConsumeTokens(1) after burst failed: %v", err)
		}
		elapsed := time.Since(start)

		// Should have waited at least 500ms
		if elapsed < 500*time.Millisecond {
			t.Errorf("ConsumeTokens() did not block long enough: %v", elapsed)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewTokenBucketLimiter(100, 1) // Very low TPM
		ctx, cancel := context.WithCancel(context.Background())

		// Exhaust token capacity
		if err := limiter.ConsumeTokens(context.Background(), 1); err != nil {
			t.Fatalf("ConsumeTokens() setup failed: %v", err)
		}

		// Cancel context and try to consume
		cancel()
		err := limiter.ConsumeTokens(ctx, 1)
		if !errors.Is(err, ratelimit.ErrContextCancelled) {
			t.Errorf("ConsumeTokens() error = %v, want ratelimit.ErrContextCancelled", err)
		}
	})
}

func TestReserve(t *testing.T) {
	t.Parallel()
	t.Run("returns true when tokens available", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewTokenBucketLimiter(100, 10000)

		if !limiter.Reserve(1000) {
			t.Error("Reserve(1000) = false, want true")
		}
	})

	t.Run("returns false when tokens unavailable", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewTokenBucketLimiter(100, 100) // Low TPM
		ctx := context.Background()

		// Exhaust capacity
		if err := limiter.ConsumeTokens(ctx, 100); err != nil {
			t.Fatalf("ConsumeTokens() setup failed: %v", err)
		}

		// Try to reserve more than remaining
		// Note: Token bucket refills over time, so check for large amount
		if limiter.Reserve(1000) {
			t.Error("Reserve(1000) = true after exhausting capacity, want false")
		}
	})

	t.Run("does not actually consume tokens", func(t *testing.T) {
		t.Parallel()
		limiter := ratelimit.NewTokenBucketLimiter(100, 10000)

		// Reserve tokens multiple times
		for reserveIdx := 0; reserveIdx < 5; reserveIdx++ {
			if !limiter.Reserve(1000) {
				t.Errorf("Reserve(1000) call %d failed", reserveIdx+1)
			}
		}

		// Should still be able to actually consume
		ctx := context.Background()
		err := limiter.ConsumeTokens(ctx, 1000)
		if err != nil {
			t.Errorf("ConsumeTokens(1000) failed after Reserve: %v", err)
		}
	})
}

// concurrentAllowAndWaitWorker performs Allow and Wait calls on a limiter.
// Returns the number of successful Allow calls.
func concurrentAllowAndWaitWorker(ctx context.Context, limiter *ratelimit.TokenBucketLimiter) int32 {
	var count int32
	for requestIdx := 0; requestIdx < 10; requestIdx++ {
		if limiter.Allow(ctx) {
			count++
		}
		// Also test Wait occasionally
		if requestIdx%3 == 0 {
			if waitErr := limiter.Wait(ctx); waitErr != nil {
				// Context timeout is acceptable under concurrent load
				break
			}
		}
	}
	return count
}

func TestConcurrencyAllowAndWait(t *testing.T) {
	t.Parallel()

	limiter := ratelimit.NewTokenBucketLimiter(10000, 100000)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var waitGroup sync.WaitGroup
	var successCount atomic.Int32

	// Spawn 50 goroutines trying to use the limiter
	for goroutineIdx := 0; goroutineIdx < 50; goroutineIdx++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			successCount.Add(concurrentAllowAndWaitWorker(ctx, limiter))
		}()
	}

	waitGroup.Wait()

	// At least some requests should have succeeded
	if successCount.Load() == 0 {
		t.Error("No requests succeeded under concurrent load")
	}
}

func TestConcurrencyGetUsage(t *testing.T) {
	t.Parallel()

	limiter := ratelimit.NewTokenBucketLimiter(100, 100000)

	var waitGroup sync.WaitGroup
	errorsChan := make(chan error, 100)

	// Spawn many goroutines reading usage
	for goroutineIdx := 0; goroutineIdx < 100; goroutineIdx++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			usage := limiter.GetUsage()
			if usage.RequestsLimit != 100 {
				errorsChan <- ratelimit.ErrRateLimitExceeded
			}
		}()
	}

	waitGroup.Wait()
	close(errorsChan)

	// Check for errors
	for err := range errorsChan {
		if err != nil {
			t.Error("GetUsage() failed under concurrent load")
		}
	}
}

func TestConcurrencyConsumeTokens(t *testing.T) {
	t.Parallel()

	limiter := ratelimit.NewTokenBucketLimiter(1000, 100000) // High limits
	ctx := context.Background()

	var waitGroup sync.WaitGroup
	errorsChan := make(chan error, 100)

	// Spawn goroutines consuming tokens
	for goroutineIdx := 0; goroutineIdx < 50; goroutineIdx++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for consumeIdx := 0; consumeIdx < 10; consumeIdx++ {
				if err := limiter.ConsumeTokens(ctx, 100); err != nil {
					errorsChan <- err
					return
				}
			}
		}()
	}

	waitGroup.Wait()
	close(errorsChan)

	// Check for errors (other than rate limit errors which are expected)
	for err := range errorsChan {
		if err != nil && !errors.Is(err, ratelimit.ErrRateLimitExceeded) {
			t.Errorf("ConsumeTokens() failed: %v", err)
		}
	}
}
