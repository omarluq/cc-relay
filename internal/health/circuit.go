// Package health provides circuit breaker and health tracking for cc-relay providers.
package health

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"github.com/sony/gobreaker/v2"
)

// State represents the circuit breaker state.
type State = gobreaker.State

// Circuit breaker state constants.
const (
	StateClosed   = gobreaker.StateClosed
	StateOpen     = gobreaker.StateOpen
	StateHalfOpen = gobreaker.StateHalfOpen
)

// CircuitBreaker wraps sony/gobreaker TwoStepCircuitBreaker for provider health tracking.
type CircuitBreaker struct {
	cb   *gobreaker.TwoStepCircuitBreaker[struct{}]
	name string
}

// NewCircuitBreaker creates a new CircuitBreaker with the given configuration.
func NewCircuitBreaker(name string, cfg CircuitBreakerConfig, logger *zerolog.Logger) *CircuitBreaker {
	// Get config values with safe uint32 conversion
	halfOpenProbes := cfg.GetHalfOpenProbes()
	if halfOpenProbes < 0 {
		halfOpenProbes = DefaultHalfOpenProbes
	}
	failureThreshold := cfg.GetFailureThreshold()
	if failureThreshold < 0 {
		failureThreshold = DefaultFailureThreshold
	}

	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: uint32(halfOpenProbes), //nolint:gosec // validated non-negative above
		Timeout:     cfg.GetOpenDuration(),
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= uint32(failureThreshold) //nolint:gosec // validated non-negative above
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			if logger == nil {
				return
			}
			event := logger.Info()
			if to == gobreaker.StateOpen {
				event = logger.Warn()
			}
			event.
				Str("provider", name).
				Str("from", from.String()).
				Str("to", to.String()).
				Msg("circuit breaker state change")
		},
		IsSuccessful: func(err error) bool {
			return err == nil || errors.Is(err, context.Canceled)
		},
	}

	return &CircuitBreaker{
		cb:   gobreaker.NewTwoStepCircuitBreaker[struct{}](settings),
		name: name,
	}
}

// Allow checks if a request is allowed through the circuit breaker.
func (c *CircuitBreaker) Allow() (done func(err error), err error) {
	d, err := c.cb.Allow()
	if err != nil {
		return nil, ErrCircuitOpen
	}
	return d, nil
}

// State returns the current circuit breaker state.
func (c *CircuitBreaker) State() State {
	return c.cb.State()
}

// Name returns the circuit breaker's name.
func (c *CircuitBreaker) Name() string {
	return c.name
}

// ReportSuccess reports a successful operation to the circuit breaker.
// Returns true if the success was recorded, false if skipped.
//
// IMPORTANT: When the circuit is OPEN, gobreaker blocks all requests via Allow(),
// so successes cannot be recorded. The circuit will only transition to HALF-OPEN
// after the configured OpenDuration timeout expires. Health check successes during
// OPEN state verify provider recovery but do NOT accelerate the transition.
func (c *CircuitBreaker) ReportSuccess() bool {
	done, err := c.Allow()
	if err != nil {
		// Circuit is OPEN - cannot record success until timeout expires
		return false
	}
	done(nil)
	return true
}

// ReportFailure reports a failed operation to the circuit breaker.
// Returns true if the failure was recorded, false if skipped.
//
// When the circuit is OPEN, failures are not recorded (circuit already open).
func (c *CircuitBreaker) ReportFailure(err error) bool {
	done, allowErr := c.Allow()
	if allowErr != nil {
		// Circuit is OPEN - already tracking failures
		return false
	}
	done(err)
	return true
}

// ShouldCountAsFailure determines if a response should count as a circuit breaker failure.
func ShouldCountAsFailure(statusCode int, err error) bool {
	if err != nil {
		return !errors.Is(err, context.Canceled)
	}
	return statusCode >= 500 || statusCode == 429
}
