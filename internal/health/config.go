// Package health provides circuit breaker and health tracking for cc-relay providers.
//
// The package implements:
//   - Circuit breaker state machine (CLOSED -> OPEN -> HALF-OPEN -> CLOSED)
//   - Provider health checks with configurable intervals
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
	DefaultHealthCheckMS    = 10000 // 10 seconds between health checks
	DefaultHealthEnabled    = true  // health checks enabled by default
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

// CheckConfig defines health check behavior.
type CheckConfig struct {
	Enabled    *bool `yaml:"enabled" toml:"enabled"`
	IntervalMS int   `yaml:"interval_ms" toml:"interval_ms"`
}

// GetInterval returns the health check interval as time.Duration.
// Returns default 10s if not set or negative.
func (c *CheckConfig) GetInterval() time.Duration {
	if c.IntervalMS <= 0 {
		return time.Duration(DefaultHealthCheckMS) * time.Millisecond
	}
	return time.Duration(c.IntervalMS) * time.Millisecond
}

// IsEnabled returns whether health checks are enabled.
// Returns true by default if not explicitly set.
func (c *CheckConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return DefaultHealthEnabled
	}
	return *c.Enabled
}

// Config combines circuit breaker and health check configuration.
type Config struct {
	HealthCheck    CheckConfig          `yaml:"health_check" toml:"health_check"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker" toml:"circuit_breaker"`
}
