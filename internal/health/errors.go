package health

import "errors"

// Sentinel errors for health tracking.
var (
	// ErrCircuitOpen is returned when the circuit breaker is open and rejecting requests.
	ErrCircuitOpen = errors.New("health: circuit breaker is open")

	// ErrHealthCheckFailed is returned when a synthetic health check fails.
	ErrHealthCheckFailed = errors.New("health: health check failed")

	// ErrProviderUnhealthy is returned when a provider is marked as unhealthy.
	ErrProviderUnhealthy = errors.New("health: provider is unhealthy")
)
