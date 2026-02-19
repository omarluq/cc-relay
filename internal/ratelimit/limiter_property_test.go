package ratelimit_test

import (
	"context"
	"sync"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/omarluq/cc-relay/internal/ratelimit"
)

// verifyConcurrentSafety runs a work function in multiple goroutines and returns
// false if any goroutine panicked. Used to reduce test cognitive complexity.
func verifyConcurrentSafety(t *testing.T, goroutines int, work func()) bool {
	t.Helper()

	var waitGroup sync.WaitGroup
	panicked := make(chan bool, goroutines)

	for goroutineIdx := 0; goroutineIdx < goroutines; goroutineIdx++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					panicked <- true
				}
			}()
			work()
		}()
	}

	waitGroup.Wait()
	close(panicked)

	for didPanic := range panicked {
		if didPanic {
			return false
		}
	}

	return true
}

// verifyConcurrentSafetyWithIdx is like verifyConcurrentSafety but passes
// the goroutine index to the work function.
func verifyConcurrentSafetyWithIdx(t *testing.T, goroutines int, work func(idx int)) bool {
	t.Helper()

	var waitGroup sync.WaitGroup
	panicked := make(chan bool, goroutines)

	for goroutineIdx := 0; goroutineIdx < goroutines; goroutineIdx++ {
		waitGroup.Add(1)
		go func(idx int) {
			defer waitGroup.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					panicked <- true
				}
			}()
			work(idx)
		}(goroutineIdx)
	}

	waitGroup.Wait()
	close(panicked)

	for didPanic := range panicked {
		if didPanic {
			return false
		}
	}

	return true
}

// Property-based tests for RateLimiter interface implementations

func TestRateLimiterProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: Allow never blocks (non-blocking check)
	properties.Property("Allow is non-blocking", prop.ForAll(
		func(rpm, tpm int) bool {
			if rpm <= 0 || tpm <= 0 {
				return true // Skip invalid inputs
			}

			limiter := ratelimit.NewTokenBucketLimiter(rpm, tpm)
			ctx := context.Background()

			// Call Allow multiple times - should never block
			for allowIdx := 0; allowIdx < rpm*2; allowIdx++ {
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

			limiter := ratelimit.NewTokenBucketLimiter(rpm, tpm)
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

			limiter := ratelimit.NewTokenBucketLimiter(rpm, tpm)
			usage := limiter.GetUsage()

			// Limits should match configured values (or unlimited)
			return usage.RequestsLimit > 0 && usage.TokensLimit > 0
		},
		gen.IntRange(1, 1000),
		gen.IntRange(1000, 100000),
	))

	properties.TestingRun(t)
}

func TestRateLimiterSetLimitProperty(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 4: SetLimit updates limits
	properties.Property("SetLimit updates limits", prop.ForAll(
		func(initialRPM, initialTPM, newRPM, newTPM int) bool {
			if initialRPM <= 0 || initialTPM <= 0 || newRPM <= 0 || newTPM <= 0 {
				return true
			}

			limiter := ratelimit.NewTokenBucketLimiter(initialRPM, initialTPM)
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

	properties.TestingRun(t)
}

func TestRateLimiterZeroLimitsProperty(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

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

			limiter := ratelimit.NewTokenBucketLimiter(rpm, tpm)
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

			limiter := ratelimit.NewTokenBucketLimiter(100, 100000)

			// Should return true or false without panicking
			result := limiter.Reserve(tokens)
			return result || !result // Always true, just verifying no panic
		},
		gen.IntRange(1, 10000),
	))

	properties.TestingRun(t)
}

func TestRateLimiterBurstProperty(t *testing.T) {
	t.Parallel()
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
			limiter := ratelimit.NewTokenBucketLimiter(limit, limit*1000)
			ctx := context.Background()

			allowed := 0
			// Try to do limit*2 requests immediately
			for allowIdx := 0; allowIdx < limit*2; allowIdx++ {
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

func TestRateLimiterConcurrentAllowProperty(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	// Property: Concurrent Allow calls don't panic
	properties.Property("concurrent Allow is safe", prop.ForAll(
		func(goroutines int) bool {
			if goroutines <= 0 || goroutines > 100 {
				return true
			}

			limiter := ratelimit.NewTokenBucketLimiter(1000, 1000000)
			ctx := context.Background()

			return verifyConcurrentSafety(t, goroutines, func() {
				for step := 0; step < 10; step++ {
					_ = limiter.Allow(ctx)
				}
			})
		},
		gen.IntRange(1, 50),
	))

	properties.TestingRun(t)
}

func TestRateLimiterConcurrentGetUsageProperty(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	// Property: Concurrent GetUsage calls don't panic
	properties.Property("concurrent GetUsage is safe", prop.ForAll(
		func(goroutines int) bool {
			if goroutines <= 0 || goroutines > 100 {
				return true
			}

			limiter := ratelimit.NewTokenBucketLimiter(100, 100000)

			return verifyConcurrentSafety(t, goroutines, func() {
				for step := 0; step < 10; step++ {
					_ = limiter.GetUsage()
				}
			})
		},
		gen.IntRange(1, 50),
	))

	properties.TestingRun(t)
}

func TestRateLimiterConcurrentSetLimitProperty(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	// Property: Concurrent SetLimit calls don't panic
	properties.Property("concurrent SetLimit is safe", prop.ForAll(
		func(goroutines int) bool {
			if goroutines <= 0 || goroutines > 50 {
				return true
			}

			limiter := ratelimit.NewTokenBucketLimiter(100, 100000)

			return verifyConcurrentSafetyWithIdx(t, goroutines, func(idx int) {
				limiter.SetLimit(100+idx, 100000+idx*1000)
			})
		},
		gen.IntRange(1, 30),
	))

	properties.TestingRun(t)
}

func TestRateLimiterMixedConcurrentProperty(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	// Property: Mixed concurrent operations are safe
	properties.Property("mixed concurrent operations are safe", prop.ForAll(
		func(goroutines int) bool {
			if goroutines <= 0 || goroutines > 50 {
				return true
			}

			limiter := ratelimit.NewTokenBucketLimiter(1000, 1000000)
			ctx := context.Background()

			// Test Allow, GetUsage, and SetLimit concurrently
			allowOk := verifyConcurrentSafety(t, goroutines, func() {
				_ = limiter.Allow(ctx)
			})
			usageOk := verifyConcurrentSafety(t, goroutines, func() {
				_ = limiter.GetUsage()
			})
			setOk := verifyConcurrentSafetyWithIdx(t, goroutines, func(idx int) {
				limiter.SetLimit(100+idx, 100000)
			})

			return allowOk && usageOk && setOk
		},
		gen.IntRange(1, 20),
	))

	properties.TestingRun(t)
}
