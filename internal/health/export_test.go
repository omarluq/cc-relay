package health

import "github.com/rs/zerolog"

// NewTestBreaker creates a CircuitBreaker for tests with the given config values.
// It uses a nop logger and the fixed name "test-provider".
func NewTestBreaker(threshold uint32, openDurationMS int, halfOpenProbes uint32) *CircuitBreaker {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: threshold,
		OpenDurationMS:   openDurationMS,
		HalfOpenProbes:   halfOpenProbes,
	}

	return NewCircuitBreaker("test-provider", cfg, &logger)
}

// GetBreakerName returns the name field for testing.
func (b *CircuitBreaker) GetBreakerName() string {
	return b.name
}

// GetErrCircuitOpen returns the errCircuitOpen sentinel for test assertions.
func GetErrCircuitOpen() error {
	return errCircuitOpen
}

// GetHost returns the host field for testing.
func (h *HTTPHealthCheck) GetHost() string {
	return h.host
}

// GetChecksCount returns the number of registered checks under lock (for testing).
func (c *Checker) GetChecksCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.checks)
}

// HasCheck returns whether a named check is registered under lock (for testing).
func (c *Checker) HasCheck(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.checks[name]
	return ok
}

// CryptoRandDurationExported exports cryptoRandDuration for testing.
var CryptoRandDurationExported = cryptoRandDuration

// CheckAllProviders exports checkAllProviders for testing.
func (c *Checker) CheckAllProviders() {
	c.checkAllProviders()
}

// HasCircuits returns whether the circuits map is initialized (for testing).
func (t *Tracker) HasCircuits() bool {
	return t.circuits != nil
}

// AllStates returns a snapshot of all provider circuit states.
// Useful for debugging and monitoring.
func (t *Tracker) AllStates() map[string]State {
	t.mu.RLock()
	defer t.mu.RUnlock()

	states := make(map[string]State, len(t.circuits))
	for name, cb := range t.circuits {
		states[name] = cb.State()
	}
	return states
}
