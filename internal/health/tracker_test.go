package health

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewTracker(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenDurationMS:   30000,
		HalfOpenProbes:   3,
	}

	tracker := NewTracker(cfg, &logger)

	if tracker == nil {
		t.Fatal("expected non-nil Tracker")
	}
	if tracker.circuits == nil {
		t.Error("expected initialized circuits map")
	}
}

func TestTracker_GetOrCreateCircuit_CreatesOnDemand(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenDurationMS:   30000,
		HalfOpenProbes:   3,
	}

	tracker := NewTracker(cfg, &logger)

	cb := tracker.GetOrCreateCircuit("provider-a")
	if cb == nil {
		t.Fatal("expected non-nil CircuitBreaker")
	}
	if cb.Name() != "provider-a" {
		t.Errorf("expected name 'provider-a', got %q", cb.Name())
	}
}

func TestTracker_GetOrCreateCircuit_ReturnsSame(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{}

	tracker := NewTracker(cfg, &logger)

	cb1 := tracker.GetOrCreateCircuit("provider-a")
	cb2 := tracker.GetOrCreateCircuit("provider-a")

	if cb1 != cb2 {
		t.Error("expected same CircuitBreaker instance for same provider")
	}
}

func TestTracker_IsHealthyFunc_ReturnsTrueWhenClosed(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenDurationMS:   30000,
		HalfOpenProbes:   3,
	}

	tracker := NewTracker(cfg, &logger)
	isHealthy := tracker.IsHealthyFunc("provider-a")

	// Circuit starts closed, should be healthy
	if !isHealthy() {
		t.Error("expected IsHealthyFunc to return true when circuit is closed")
	}
}

func TestTracker_IsHealthyFunc_ReturnsFalseWhenOpen(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   30000,
		HalfOpenProbes:   1,
	}

	tracker := NewTracker(cfg, &logger)
	testErr := errors.New("test error")

	// Open the circuit
	tracker.RecordFailure("provider-a", testErr)
	tracker.RecordFailure("provider-a", testErr)

	isHealthy := tracker.IsHealthyFunc("provider-a")

	if isHealthy() {
		t.Error("expected IsHealthyFunc to return false when circuit is open")
	}
}

func TestTracker_IsHealthyFunc_ReturnsTrueWhenHalfOpen(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   50, // Short timeout for testing
		HalfOpenProbes:   1,
	}

	tracker := NewTracker(cfg, &logger)
	testErr := errors.New("test error")

	// Open the circuit
	tracker.RecordFailure("provider-a", testErr)
	tracker.RecordFailure("provider-a", testErr)

	// Wait for timeout to transition to half-open
	time.Sleep(100 * time.Millisecond)

	// Trigger transition to half-open by calling Allow
	cb := tracker.GetOrCreateCircuit("provider-a")
	_, _ = cb.Allow() // Discard done func - leave in half-open state

	isHealthy := tracker.IsHealthyFunc("provider-a")

	// Half-open should be considered healthy (allows probes)
	if !isHealthy() {
		t.Error("expected IsHealthyFunc to return true when circuit is half-open")
	}
}

func TestTracker_RecordSuccess(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenDurationMS:   30000,
		HalfOpenProbes:   3,
	}

	tracker := NewTracker(cfg, &logger)

	// RecordSuccess should not panic and circuit should stay closed
	tracker.RecordSuccess("provider-a")

	state := tracker.GetState("provider-a")
	if state != StateClosed {
		t.Errorf("expected state CLOSED after RecordSuccess, got %s", state.String())
	}
}

func TestTracker_RecordFailure(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   30000,
		HalfOpenProbes:   1,
	}

	tracker := NewTracker(cfg, &logger)
	testErr := errors.New("test error")

	tracker.RecordFailure("provider-a", testErr)
	tracker.RecordFailure("provider-a", testErr)

	state := tracker.GetState("provider-a")
	if state != StateOpen {
		t.Errorf("expected state OPEN after threshold failures, got %s", state.String())
	}
}

func TestTracker_AllStates(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   30000,
		HalfOpenProbes:   1,
	}

	tracker := NewTracker(cfg, &logger)
	testErr := errors.New("test error")

	// Create circuits for multiple providers
	tracker.RecordSuccess("provider-a")
	tracker.RecordFailure("provider-b", testErr)
	tracker.RecordFailure("provider-b", testErr)

	states := tracker.AllStates()

	if len(states) != 2 {
		t.Errorf("expected 2 states, got %d", len(states))
	}
	if states["provider-a"] != StateClosed {
		t.Errorf("expected provider-a state CLOSED, got %s", states["provider-a"].String())
	}
	if states["provider-b"] != StateOpen {
		t.Errorf("expected provider-b state OPEN, got %s", states["provider-b"].String())
	}
}

func TestTracker_GetState_ReturnsClosedForUnknown(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{}

	tracker := NewTracker(cfg, &logger)

	state := tracker.GetState("unknown-provider")
	if state != StateClosed {
		t.Errorf("expected StateClosed for unknown provider, got %s", state.String())
	}
}

func TestTracker_ConcurrentAccess(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 100, // High threshold to avoid opening
		OpenDurationMS:   30000,
		HalfOpenProbes:   3,
	}

	tracker := NewTracker(cfg, &logger)

	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			providerName := "provider"
			testErr := errors.New("test error")

			for j := 0; j < numOperations; j++ {
				// Mix of operations
				switch j % 5 {
				case 0:
					tracker.GetOrCreateCircuit(providerName)
				case 1:
					tracker.RecordSuccess(providerName)
				case 2:
					tracker.RecordFailure(providerName, testErr)
				case 3:
					tracker.GetState(providerName)
				case 4:
					tracker.AllStates()
				}
			}
		}()
	}

	wg.Wait()

	// If we get here without deadlock or panic, the test passes
	states := tracker.AllStates()
	if len(states) != 1 {
		t.Errorf("expected 1 provider in states, got %d", len(states))
	}
}
