package ratelimit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewTokenBucketLimiter(t *testing.T) {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewTokenBucketLimiter(tt.rpm, tt.tpm)
			if limiter == nil {
				t.Fatal("NewTokenBucketLimiter returned nil")
			}

			if limiter.rpmLimit != tt.wantRPM {
				t.Errorf("rpmLimit = %d, want %d", limiter.rpmLimit, tt.wantRPM)
			}
			if limiter.tpmLimit != tt.wantTPM {
				t.Errorf("tpmLimit = %d, want %d", limiter.tpmLimit, tt.wantTPM)
			}
		})
	}
}

func TestAllow(t *testing.T) {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewTokenBucketLimiter(tt.rpm, tt.tpm)
			ctx := context.Background()

			allowed := 0
			for i := 0; i < tt.numRequests; i++ {
				if limiter.Allow(ctx) {
					allowed++
				}
			}

			if allowed != tt.wantAllowed {
				t.Errorf("Allow() allowed %d requests, want %d", allowed, tt.wantAllowed)
			}
		})
	}
}

func TestWait(t *testing.T) {
	t.Run("blocks until capacity available", func(t *testing.T) {
		// Use very low limit for fast test
		limiter := NewTokenBucketLimiter(60, 10000) // 1 per second
		ctx := context.Background()

		// Exhaust the burst capacity (60 requests available immediately)
		for i := 0; i < 60; i++ {
			if err := limiter.Wait(ctx); err != nil {
				t.Fatalf("Wait() %d failed: %v", i, err)
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
		limiter := NewTokenBucketLimiter(1, 10000) // Very low limit
		ctx, cancel := context.WithCancel(context.Background())

		// Exhaust capacity
		_ = limiter.Allow(ctx)

		// Cancel context and try to wait
		cancel()
		err := limiter.Wait(ctx)
		if !errors.Is(err, ErrContextCancelled) {
			t.Errorf("Wait() error = %v, want ErrContextCancelled", err)
		}
	})

	t.Run("respects context deadline", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(1, 10000) // Very low limit
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

func TestSetLimit(t *testing.T) {
	t.Run("updates limits dynamically", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(10, 1000)

		// Update limits
		limiter.SetLimit(50, 5000)

		if limiter.rpmLimit != 50 {
			t.Errorf("rpmLimit = %d, want 50", limiter.rpmLimit)
		}
		if limiter.tpmLimit != 5000 {
			t.Errorf("tpmLimit = %d, want 5000", limiter.tpmLimit)
		}
	})

	t.Run("new limit takes effect immediately", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(5, 1000)
		ctx := context.Background()

		// Exhaust initial limit
		for i := 0; i < 5; i++ {
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

	t.Run("thread safe", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(100, 10000)
		ctx := context.Background()

		var wg sync.WaitGroup
		errorsChan := make(chan error, 100)

		// Spawn multiple goroutines updating limits
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					limiter.SetLimit(50+n, 5000+n*1000)
					_ = limiter.Allow(ctx)
					usage := limiter.GetUsage()
					if usage.RequestsLimit <= 0 {
						errorsChan <- ErrRateLimitExceeded
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(errorsChan)

		// Check for errors
		for err := range errorsChan {
			if err != nil {
				t.Errorf("concurrent SetLimit/Allow failed: %v", err)
			}
		}
	})
}

func TestGetUsage(t *testing.T) {
	t.Run("returns correct limits", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(50, 30000)
		usage := limiter.GetUsage()

		if usage.RequestsLimit != 50 {
			t.Errorf("RequestsLimit = %d, want 50", usage.RequestsLimit)
		}
		if usage.TokensLimit != 30000 {
			t.Errorf("TokensLimit = %d, want 30000", usage.TokensLimit)
		}
	})

	t.Run("updates after Allow calls", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(10, 10000)
		ctx := context.Background()

		// Exhaust all capacity
		for i := 0; i < 10; i++ {
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
	t.Run("records token usage correctly", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(100, 1000) // 1000 TPM
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
		limiter := NewTokenBucketLimiter(100, 60) // 1 token per second
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
		limiter := NewTokenBucketLimiter(100, 1) // Very low TPM
		ctx, cancel := context.WithCancel(context.Background())

		// Exhaust token capacity
		_ = limiter.ConsumeTokens(context.Background(), 1)

		// Cancel context and try to consume
		cancel()
		err := limiter.ConsumeTokens(ctx, 1)
		if !errors.Is(err, ErrContextCancelled) {
			t.Errorf("ConsumeTokens() error = %v, want ErrContextCancelled", err)
		}
	})
}

func TestReserve(t *testing.T) {
	t.Run("returns true when tokens available", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(100, 10000)

		if !limiter.Reserve(1000) {
			t.Error("Reserve(1000) = false, want true")
		}
	})

	t.Run("returns false when tokens unavailable", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(100, 100) // Low TPM
		ctx := context.Background()

		// Exhaust capacity
		_ = limiter.ConsumeTokens(ctx, 100)

		// Try to reserve more than remaining
		// Note: Token bucket refills over time, so check for large amount
		if limiter.Reserve(1000) {
			t.Error("Reserve(1000) = true after exhausting capacity, want false")
		}
	})

	t.Run("does not actually consume tokens", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(100, 10000)

		// Reserve tokens multiple times
		for i := 0; i < 5; i++ {
			if !limiter.Reserve(1000) {
				t.Errorf("Reserve(1000) call %d failed", i+1)
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

func TestConcurrency(t *testing.T) {
	t.Run("multiple goroutines calling Allow/Wait", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(100, 100000)
		ctx := context.Background()

		var wg sync.WaitGroup
		successCount := int32(0)
		var mu sync.Mutex

		// Spawn 50 goroutines trying to use the limiter
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					if limiter.Allow(ctx) {
						mu.Lock()
						successCount++
						mu.Unlock()
					}
					// Also test Wait occasionally
					if j%3 == 0 {
						_ = limiter.Wait(ctx)
					}
				}
			}()
		}

		wg.Wait()

		// At least some requests should have succeeded
		if successCount == 0 {
			t.Error("No requests succeeded under concurrent load")
		}
	})

	t.Run("concurrent GetUsage calls", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(100, 100000)

		var wg sync.WaitGroup
		errorsChan := make(chan error, 100)

		// Spawn many goroutines reading usage
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				usage := limiter.GetUsage()
				if usage.RequestsLimit != 100 {
					errorsChan <- ErrRateLimitExceeded
				}
			}()
		}

		wg.Wait()
		close(errorsChan)

		// Check for errors
		for err := range errorsChan {
			if err != nil {
				t.Error("GetUsage() failed under concurrent load")
			}
		}
	})

	t.Run("concurrent ConsumeTokens calls", func(t *testing.T) {
		limiter := NewTokenBucketLimiter(1000, 100000) // High limits
		ctx := context.Background()

		var wg sync.WaitGroup
		errorsChan := make(chan error, 100)

		// Spawn goroutines consuming tokens
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					if err := limiter.ConsumeTokens(ctx, 100); err != nil {
						errorsChan <- err
						return
					}
				}
			}()
		}

		wg.Wait()
		close(errorsChan)

		// Check for errors (other than rate limit errors which are expected)
		for err := range errorsChan {
			if err != nil && !errors.Is(err, ErrRateLimitExceeded) {
				t.Errorf("ConsumeTokens() failed: %v", err)
			}
		}
	})
}
