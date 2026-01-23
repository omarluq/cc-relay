package router

// This file contains failover trigger definitions.

import (
	"context"
	"errors"
	"net"
)

// Trigger name constants for logging.
const (
	TriggerStatusCode  = "status_code"
	TriggerTimeout     = "timeout"
	TriggerConnection  = "connection"
)

// FailoverTrigger defines conditions that trigger failover to alternate providers.
// Implementations check specific failure conditions and return true if failover should occur.
//
// The trigger system is pluggable, allowing new conditions to be added without
// modifying core failover logic. Common triggers include:
//   - Status code triggers (429 rate limit, 5xx server errors)
//   - Timeout triggers (context deadline exceeded)
//   - Connection triggers (network errors)
type FailoverTrigger interface {
	// ShouldFailover returns true if the error/status warrants trying another provider.
	// statusCode is the HTTP status code (0 if not applicable).
	ShouldFailover(err error, statusCode int) bool

	// Name returns the trigger name for logging.
	Name() string
}

// StatusCodeTrigger triggers failover on specific HTTP status codes.
// Commonly used for rate limit (429) and server error (5xx) responses.
type StatusCodeTrigger struct {
	codes []int
}

// NewStatusCodeTrigger creates a trigger that fires on the specified status codes.
// Common usage: NewStatusCodeTrigger(429, 500, 502, 503, 504).
func NewStatusCodeTrigger(codes ...int) *StatusCodeTrigger {
	return &StatusCodeTrigger{codes: codes}
}

// ShouldFailover returns true if statusCode matches any configured code.
func (t *StatusCodeTrigger) ShouldFailover(_ error, statusCode int) bool {
	for _, code := range t.codes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// Name returns TriggerStatusCode for logging.
func (t *StatusCodeTrigger) Name() string {
	return TriggerStatusCode
}

// TimeoutTrigger triggers failover on context deadline exceeded errors.
// This handles both request timeouts and upstream response timeouts.
type TimeoutTrigger struct{}

// NewTimeoutTrigger creates a trigger that fires on context.DeadlineExceeded.
func NewTimeoutTrigger() *TimeoutTrigger {
	return &TimeoutTrigger{}
}

// ShouldFailover returns true if err wraps context.DeadlineExceeded.
func (t *TimeoutTrigger) ShouldFailover(err error, _ int) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

// Name returns TriggerTimeout for logging.
func (t *TimeoutTrigger) Name() string {
	return TriggerTimeout
}

// ConnectionTrigger triggers failover on network connection errors.
// This catches connection refused, DNS failures, network unreachable, etc.
type ConnectionTrigger struct{}

// NewConnectionTrigger creates a trigger that fires on net.Error.
func NewConnectionTrigger() *ConnectionTrigger {
	return &ConnectionTrigger{}
}

// ShouldFailover returns true if err implements net.Error.
// This includes connection refused, DNS failures, timeouts from net package.
func (t *ConnectionTrigger) ShouldFailover(err error, _ int) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// Name returns TriggerConnection for logging.
func (t *ConnectionTrigger) Name() string {
	return TriggerConnection
}

// DefaultTriggers returns the standard set of failover triggers:
//   - 429 (rate limit), 500, 502, 503, 504 status codes
//   - Timeout errors (context deadline exceeded)
//   - Network connection errors
//
// This provides sensible defaults for most use cases.
func DefaultTriggers() []FailoverTrigger {
	return []FailoverTrigger{
		NewStatusCodeTrigger(429, 500, 502, 503, 504),
		NewTimeoutTrigger(),
		NewConnectionTrigger(),
	}
}

// ShouldFailover checks if any trigger fires for the given error/status.
// Returns true on first matching trigger (short-circuit evaluation).
// Returns false if triggers slice is empty or nil.
func ShouldFailover(triggers []FailoverTrigger, err error, statusCode int) bool {
	for _, trigger := range triggers {
		if trigger.ShouldFailover(err, statusCode) {
			return true
		}
	}
	return false
}

// FindMatchingTrigger returns the first trigger that fires for the given error/status.
// Returns nil if no trigger matches. Useful for logging which trigger caused failover.
func FindMatchingTrigger(triggers []FailoverTrigger, err error, statusCode int) FailoverTrigger {
	for _, trigger := range triggers {
		if trigger.ShouldFailover(err, statusCode) {
			return trigger
		}
	}
	return nil
}
