// Package health provides circuit breaker and health tracking for cc-relay providers.
//
// The package implements:
//   - Circuit breaker state machine (CLOSED -> OPEN -> HALF-OPEN -> CLOSED)
//   - Failure tracking and automatic recovery probing
//
// Circuit breaker prevents cascading failures by temporarily blocking requests
// to unhealthy providers, allowing them time to recover before retrying.
package health

import "time"

// Default configuration values.
const (
	DefaultFailureThreshold = 5     // consecutive failures to open circuit
	DefaultOpenDurationMS   = 30000 // 30 seconds before half-open
	DefaultHalfOpenProbes   = 3     // probes allowed in half-open state
)

// CircuitBreakerConfig defines circuit breaker behavior.
type CircuitBreakerConfig struct {
	OpenDurationMS   int    `yaml:"open_duration_ms" toml:"open_duration_ms"`
	FailureThreshold uint32 `yaml:"failure_threshold" toml:"failure_threshold"`
	HalfOpenProbes   uint32 `yaml:"half_open_probes" toml:"half_open_probes"`
}

// GetFailureThreshold returns the configured failure threshold or default 5.
func (c *CircuitBreakerConfig) GetFailureThreshold() uint32 {
	if c.FailureThreshold == 0 {
		return DefaultFailureThreshold
	}
	return c.FailureThreshold
}

// GetOpenDuration returns the open duration as time.Duration.
// Returns default 30s if not set or negative.
func (c *CircuitBreakerConfig) GetOpenDuration() time.Duration {
	if c.OpenDurationMS <= 0 {
		return time.Duration(DefaultOpenDurationMS) * time.Millisecond
	}
	return time.Duration(c.OpenDurationMS) * time.Millisecond
}

// GetHalfOpenProbes returns the configured half-open probes or default 3.
func (c *CircuitBreakerConfig) GetHalfOpenProbes() uint32 {
	if c.HalfOpenProbes == 0 {
		return DefaultHalfOpenProbes
	}
	return c.HalfOpenProbes
}

// Config holds the health tracking configuration.
//
// The former health_check section is no longer read: its periodic checks
// could not change circuit state, so they were removed. Config files that
// still contain it load fine, because unknown keys are ignored.
type Config struct {
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker" toml:"circuit_breaker"`
}
