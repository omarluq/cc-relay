package health

import (
	"sync"

	"github.com/rs/zerolog"
)

// Tracker manages per-provider circuit breakers.
// It provides thread-safe access to circuit breakers and exposes
// IsHealthyFunc closures for integration with the router.
type Tracker struct {
	circuits map[string]*CircuitBreaker
	logger   *zerolog.Logger
	config   CircuitBreakerConfig
	mu       sync.RWMutex
}

// NewTracker creates a new Tracker with the given configuration.
func NewTracker(cfg CircuitBreakerConfig, logger *zerolog.Logger) *Tracker {
	return &Tracker{
		circuits: make(map[string]*CircuitBreaker),
		config:   cfg,
		logger:   logger,
	}
}

// Reset replaces the tracker configuration and clears existing circuits.
// This is used to apply hot-reload changes consistently across providers.
func (t *Tracker) Reset(cfg CircuitBreakerConfig, logger *zerolog.Logger) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.config = cfg
	t.logger = logger
	t.circuits = make(map[string]*CircuitBreaker)
}

// GetOrCreateCircuit returns the circuit breaker for a provider, creating it if necessary.
// This method is thread-safe and uses lazy initialization.
func (t *Tracker) GetOrCreateCircuit(providerName string) *CircuitBreaker {
	// Fast path: check if circuit exists with read lock
	t.mu.RLock()
	cb, exists := t.circuits[providerName]
	t.mu.RUnlock()

	if exists {
		return cb
	}

	// Slow path: create circuit with write lock
	t.mu.Lock()
	defer t.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists = t.circuits[providerName]; exists {
		return cb
	}

	// Create new circuit breaker
	cb = NewCircuitBreaker(providerName, t.config, t.logger)
	t.circuits[providerName] = cb

	if t.logger != nil {
		t.logger.Debug().
			Str("provider", providerName).
			Msg("created circuit breaker")
	}

	return cb
}

// IsHealthyFunc returns a closure that checks if a provider is healthy.
// This closure is designed to be wired into ProviderInfo.IsHealthy.
//
// A provider is considered healthy if its circuit is:
//   - CLOSED: Normal operation, requests flow through
//   - HALF-OPEN: Testing recovery, probe requests are allowed
//
// A provider is unhealthy only if the circuit is OPEN.
func (t *Tracker) IsHealthyFunc(providerName string) func() bool {
	return func() bool {
		cb := t.GetOrCreateCircuit(providerName)
		// OPEN = unhealthy, CLOSED/HALF-OPEN = healthy
		return cb.State() != StateOpen
	}
}

// GetState returns the current state of a provider's circuit breaker.
// Returns StateClosed if no circuit exists for the provider (healthy by default).
func (t *Tracker) GetState(providerName string) State {
	t.mu.RLock()
	cb, exists := t.circuits[providerName]
	t.mu.RUnlock()

	if !exists {
		return StateClosed
	}
	return cb.State()
}

// RecordSuccess records a successful operation for a provider.
// When the circuit is OPEN, the success is not recorded (gobreaker limitation)
// and a debug log is emitted to make this visible.
func (t *Tracker) RecordSuccess(providerName string) {
	cb := t.GetOrCreateCircuit(providerName)
	recorded := cb.ReportSuccess()

	if t.logger != nil {
		if recorded {
			t.logger.Debug().
				Str("provider", providerName).
				Str("state", cb.State().String()).
				Msg("recorded success")
		} else {
			// Circuit is OPEN - success not recorded, waiting for timeout
			t.logger.Debug().
				Str("provider", providerName).
				Str("state", cb.State().String()).
				Msg("success not recorded (circuit open, waiting for timeout)")
		}
	}
}

// RecordFailure records a failed operation for a provider.
// When the circuit is OPEN, the failure is not recorded (already open).
func (t *Tracker) RecordFailure(providerName string, err error) {
	cb := t.GetOrCreateCircuit(providerName)
	recorded := cb.ReportFailure(err)

	if t.logger != nil {
		if recorded {
			t.logger.Debug().
				Str("provider", providerName).
				Str("state", cb.State().String()).
				Err(err).
				Msg("recorded failure")
		} else {
			t.logger.Debug().
				Str("provider", providerName).
				Str("state", cb.State().String()).
				Err(err).
				Msg("failure not recorded (circuit already open)")
		}
	}
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
