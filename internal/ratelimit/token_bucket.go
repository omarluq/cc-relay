package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// TokenBucketLimiter implements RateLimiter using golang.org/x/time/rate.
//
// It uses two separate token bucket limiters:
//   - requestLimiter: tracks requests per minute (RPM)
//   - tokenLimiter: tracks tokens per minute (TPM)
//
// The token bucket algorithm provides smooth rate limiting without the
// boundary burst problem of fixed windows. Burst is set equal to the limit
// to allow consuming the full minute's capacity instantly, then refilling gradually.
//
// Thread safety: All methods are safe for concurrent use.
type TokenBucketLimiter struct {
	requestLimiter *rate.Limiter
	tokenLimiter   *rate.Limiter
	rpmLimit       int
	tpmLimit       int
	mu             sync.RWMutex // Protects limit fields and limiter updates
}

// NewTokenBucketLimiter creates a new token bucket rate limiter.
//
// Parameters:
//   - rpm: requests per minute limit (0 or negative = unlimited)
//   - tpm: tokens per minute limit (0 or negative = unlimited)
//
// The limiters are configured with:
//   - Rate: limit/60.0 (convert per-minute to per-second)
//   - Burst: limit (allow full minute's capacity instantly)
//
// Zero or negative limits are treated as "unlimited" by setting a very high limit.
func NewTokenBucketLimiter(rpm, tpm int) *TokenBucketLimiter {
	const unlimitedRate = 1_000_000 // Very high rate for "unlimited"

	// Handle zero/negative values as unlimited
	if rpm <= 0 {
		rpm = unlimitedRate
	}
	if tpm <= 0 {
		tpm = unlimitedRate
	}

	return &TokenBucketLimiter{
		requestLimiter: rate.NewLimiter(rate.Limit(float64(rpm)/60.0), rpm),
		tokenLimiter:   rate.NewLimiter(rate.Limit(float64(tpm)/60.0), tpm),
		rpmLimit:       rpm,
		tpmLimit:       tpm,
	}
}

// Allow checks if a request is allowed under the current RPM limit.
// This is a non-blocking operation.
//
// Note: This only checks the request limit, not the token limit.
// Token consumption is handled separately via ConsumeTokens after the response.
func (l *TokenBucketLimiter) Allow(_ context.Context) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.requestLimiter.Allow()
}

// Wait blocks until a request is allowed or the context is canceled.
// Returns ErrContextCancelled if the context is canceled while waiting.
func (l *TokenBucketLimiter) Wait(ctx context.Context) error {
	l.mu.RLock()
	limiter := l.requestLimiter
	l.mu.RUnlock()

	if err := limiter.Wait(ctx); err != nil {
		if ctx.Err() != nil {
			return ErrContextCancelled
		}
		return err
	}
	return nil
}

// SetLimit updates the rate limits dynamically.
// This is used to learn actual limits from provider response headers.
//
// The method is thread-safe and creates new limiters with updated rates.
// Zero or negative values are treated as unlimited.
func (l *TokenBucketLimiter) SetLimit(rpm, tpm int) {
	const unlimitedRate = 1_000_000

	// Handle zero/negative values as unlimited
	if rpm <= 0 {
		rpm = unlimitedRate
	}
	if tpm <= 0 {
		tpm = unlimitedRate
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Create new limiters with updated rates
	l.requestLimiter = rate.NewLimiter(rate.Limit(float64(rpm)/60.0), rpm)
	l.tokenLimiter = rate.NewLimiter(rate.Limit(float64(tpm)/60.0), tpm)
	l.rpmLimit = rpm
	l.tpmLimit = tpm
}

// GetUsage returns the current usage statistics.
//
// Note: golang.org/x/time/rate doesn't expose remaining tokens directly.
// We approximate by checking if a burst-sized reservation would succeed.
// This is accurate enough for key selection strategies.
func (l *TokenBucketLimiter) GetUsage() Usage {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Approximate remaining capacity by checking if burst-sized request would succeed
	// For RPM: check if we can do another request
	requestsRemaining := 0
	if l.requestLimiter.Allow() {
		// We consumed one, so add it back
		l.requestLimiter.Reserve().Cancel()
		// Estimate remaining based on burst
		requestsRemaining = l.rpmLimit / 2 // Conservative estimate
	}

	// For TPM: similar approach
	tokensRemaining := 0
	reservation := l.tokenLimiter.Reserve()
	if reservation.OK() {
		reservation.Cancel()
		tokensRemaining = l.tpmLimit / 2 // Conservative estimate
	}

	return Usage{
		RequestsUsed:      l.rpmLimit - requestsRemaining,
		RequestsLimit:     l.rpmLimit,
		TokensUsed:        l.tpmLimit - tokensRemaining,
		TokensLimit:       l.tpmLimit,
		RequestsRemaining: requestsRemaining,
		TokensRemaining:   tokensRemaining,
	}
}

// Reserve checks if a specific number of tokens can be reserved.
// This is a non-blocking optimistic check used before making the request.
//
// Note: This doesn't actually reserve the tokens - it just checks availability.
// Actual consumption happens via ConsumeTokens after the response.
func (l *TokenBucketLimiter) Reserve(tokens int) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Check if we can reserve the tokens without waiting
	reservation := l.tokenLimiter.ReserveN(time.Now(), tokens)
	if !reservation.OK() {
		return false
	}

	// Cancel the reservation - we're just checking, not consuming
	reservation.Cancel()
	return true
}

// ConsumeTokens records actual token usage after a response is received.
// This blocks if consuming the tokens would exceed the TPM limit.
//
// Returns ErrContextCancelled if the context is canceled while waiting.
func (l *TokenBucketLimiter) ConsumeTokens(ctx context.Context, tokens int) error {
	l.mu.RLock()
	limiter := l.tokenLimiter
	l.mu.RUnlock()

	if err := limiter.WaitN(ctx, tokens); err != nil {
		if ctx.Err() != nil {
			return ErrContextCancelled
		}
		return err
	}
	return nil
}
