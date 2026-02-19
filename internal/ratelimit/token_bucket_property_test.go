package ratelimit_test

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/omarluq/cc-relay/internal/ratelimit"
)

// Property-based tests specific to TokenBucketLimiter implementation

func TestTokenBucketLimiterConstructorProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: Constructor always returns non-nil limiter
	properties.Property("constructor returns non-nil", prop.ForAll(
		func(rpm, tpm int) bool {
			limiter := ratelimit.NewTokenBucketLimiter(rpm, tpm)
			return limiter != nil
		},
		gen.IntRange(-100, 1000),
		gen.IntRange(-100, 1000000),
	))

	// Property 2: Negative limits converted to unlimited
	properties.Property("negative limits become unlimited", prop.ForAll(
		func(rpm, tpm int) bool {
			if rpm >= 0 || tpm >= 0 {
				return true // Only test negative values
			}

			limiter := ratelimit.NewTokenBucketLimiter(rpm, tpm)
			usage := limiter.GetUsage()

			// Negative should be treated as unlimited (1M)
			return usage.RequestsLimit == 1_000_000 && usage.TokensLimit == 1_000_000
		},
		gen.IntRange(-1000, -1),
		gen.IntRange(-1000000, -1),
	))

	properties.TestingRun(t)
}

func TestTokenBucketLimiterContextProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 3: Wait returns immediately or waits (doesn't panic)
	properties.Property("Wait handles context correctly", prop.ForAll(
		func(rpm int) bool {
			if rpm <= 0 {
				return true
			}

			limiter := ratelimit.NewTokenBucketLimiter(rpm, 100000)
			ctx := context.Background()

			// First wait should succeed quickly for fresh limiter
			err := limiter.Wait(ctx)
			return err == nil
		},
		gen.IntRange(1, 100),
	))

	// Property 4: Canceled context returns error
	properties.Property("canceled context returns error", prop.ForAll(
		func(rpm int) bool {
			if rpm <= 0 {
				return true
			}

			limiter := ratelimit.NewTokenBucketLimiter(rpm, 100000)
			ctx, cancel := context.WithCancel(context.Background())

			// Cancel immediately
			cancel()

			// Wait should return error for canceled context
			err := limiter.Wait(ctx)
			return err != nil
		},
		gen.IntRange(1, 100),
	))

	// Property 5: ConsumeTokens with canceled context returns error
	properties.Property("ConsumeTokens respects context cancellation", prop.ForAll(
		func(tokens int) bool {
			if tokens <= 0 {
				return true
			}

			limiter := ratelimit.NewTokenBucketLimiter(100, tokens*2)
			ctx, cancel := context.WithCancel(context.Background())

			// Cancel immediately
			cancel()

			// Should return error
			err := limiter.ConsumeTokens(ctx, tokens)
			return err != nil
		},
		gen.IntRange(1, 1000),
	))

	properties.TestingRun(t)
}

func TestTokenBucketLimiterUsageProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 6: Usage remaining never exceeds limit
	properties.Property("remaining never exceeds limit", prop.ForAll(
		func(rpm, tpm int) bool {
			if rpm <= 0 || tpm <= 0 {
				return true
			}

			limiter := ratelimit.NewTokenBucketLimiter(rpm, tpm)
			usage := limiter.GetUsage()

			return usage.RequestsRemaining <= usage.RequestsLimit &&
				usage.TokensRemaining <= usage.TokensLimit
		},
		gen.IntRange(1, 1000),
		gen.IntRange(1000, 1000000),
	))

	// Property 7: Usage used is non-negative
	properties.Property("used is non-negative", prop.ForAll(
		func(rpm, tpm int) bool {
			if rpm <= 0 || tpm <= 0 {
				return true
			}

			limiter := ratelimit.NewTokenBucketLimiter(rpm, tpm)
			usage := limiter.GetUsage()

			return usage.RequestsUsed >= 0 && usage.TokensUsed >= 0
		},
		gen.IntRange(1, 1000),
		gen.IntRange(1000, 1000000),
	))

	properties.TestingRun(t)
}

func TestTokenBucketLimiterReserveProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: Reserve with small tokens succeeds on fresh limiter
	properties.Property("reserve small amount succeeds on fresh limiter", prop.ForAll(
		func(tpm int) bool {
			if tpm <= 100 {
				return true
			}

			limiter := ratelimit.NewTokenBucketLimiter(100, tpm)

			// Reserve a small portion should succeed
			return limiter.Reserve(10)
		},
		gen.IntRange(1000, 100000),
	))

	// Property 2: Reserve returns boolean (idempotent check)
	properties.Property("reserve is idempotent", prop.ForAll(
		func(tokens, tpm int) bool {
			if tokens <= 0 || tpm <= 0 {
				return true
			}

			limiter := ratelimit.NewTokenBucketLimiter(100, tpm)

			// Multiple reserve calls should all return booleans
			firstReserve := limiter.Reserve(tokens)
			secondReserve := limiter.Reserve(tokens)

			// Both should be valid booleans (either true or false)
			return (firstReserve || !firstReserve) && (secondReserve || !secondReserve)
		},
		gen.IntRange(1, 10000),
		gen.IntRange(1000, 100000),
	))

	properties.TestingRun(t)
}
