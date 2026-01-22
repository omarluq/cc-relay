// Package ratelimit provides rate limiting interfaces and implementations for cc-relay.
//
// The ratelimit package abstracts over different rate limiting strategies:
//   - Token bucket: Uses golang.org/x/time/rate for smooth traffic shaping
//   - Sliding window: Time-based window with precise limit enforcement
//
// All implementations track both RPM (requests per minute) and TPM (tokens per minute)
// to match Anthropic API rate limit semantics.
//
// Basic usage:
//
//	limiter := ratelimit.NewTokenBucketLimiter(50, 30000) // 50 RPM, 30K TPM
//
//	// Check if request is allowed (non-blocking)
//	if !limiter.Allow(ctx) {
//		return ErrRateLimitExceeded
//	}
//
//	// Record actual token usage after response
//	err := limiter.ConsumeTokens(ctx, 1234)
package ratelimit

import (
	"context"
	"errors"
)

// Common errors returned by rate limiters.
var (
	// ErrRateLimitExceeded is returned when a rate limit is exceeded.
	ErrRateLimitExceeded = errors.New("ratelimit: rate limit exceeded")

	// ErrContextCancelled is returned when the context is canceled during a blocking operation.
	ErrContextCancelled = errors.New("ratelimit: context canceled")
)

// Usage represents the current usage and limits for a rate limiter.
type Usage struct {
	// RequestsUsed is the number of requests consumed in the current window.
	RequestsUsed int `json:"requests_used"`

	// RequestsLimit is the maximum number of requests allowed per minute.
	RequestsLimit int `json:"requests_limit"`

	// TokensUsed is the number of tokens consumed in the current window.
	TokensUsed int `json:"tokens_used"`

	// TokensLimit is the maximum number of tokens allowed per minute.
	TokensLimit int `json:"tokens_limit"`

	// RequestsRemaining is the number of requests remaining in the current window.
	RequestsRemaining int `json:"requests_remaining"`

	// TokensRemaining is the number of tokens remaining in the current window.
	TokensRemaining int `json:"tokens_remaining"`
}

// RateLimiter defines the interface for rate limiting operations.
// All implementations must be safe for concurrent use.
//
// Rate limiters track two dimensions:
//   - Requests per minute (RPM): Number of requests allowed
//   - Tokens per minute (TPM): Total tokens (input + output) allowed
//
// Typical workflow:
//  1. Call Allow() to check if request can proceed (non-blocking)
//  2. If allowed, call Reserve() to reserve expected token count
//  3. After response, call ConsumeTokens() with actual token usage
//  4. Limits can be updated dynamically via SetLimit() (for header learning)
type RateLimiter interface {
	// Allow checks if a request is allowed under the current rate limits.
	// This is a non-blocking operation that returns immediately.
	// Returns true if the request can proceed, false if rate limited.
	Allow(ctx context.Context) bool

	// Wait blocks until a request is allowed or the context is canceled.
	// Returns nil when the request is allowed to proceed.
	// Returns ErrContextCancelled if the context is canceled before capacity is available.
	Wait(ctx context.Context) error

	// SetLimit updates the rate limits dynamically.
	// This is used to learn actual limits from provider response headers.
	// rpm: requests per minute limit (0 = unlimited)
	// tpm: tokens per minute limit (0 = unlimited)
	SetLimit(rpm, tpm int)

	// GetUsage returns the current usage statistics.
	// This can be used for key selection strategies (e.g., least-loaded).
	GetUsage() Usage

	// Reserve reserves a specific number of tokens for an upcoming request.
	// This is a non-blocking optimistic check used before making the request.
	// Returns true if the tokens can be reserved, false if it would exceed limits.
	// The actual consumption happens via ConsumeTokens after the response.
	Reserve(tokens int) bool

	// ConsumeTokens records actual token usage after a response is received.
	// This blocks if consuming the tokens would exceed the TPM limit.
	// tokens: actual token count from response (input + output tokens)
	// Returns ErrContextCancelled if the context is canceled while waiting.
	ConsumeTokens(ctx context.Context, tokens int) error
}
