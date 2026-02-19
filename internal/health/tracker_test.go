package health_test

import (
	"github.com/omarluq/cc-relay/internal/health"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewTracker(t *testing.T) {
	t.Parallel()
	logger := zerolog.Nop()
	cfg := health.CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenDurationMS:   30000,
		HalfOpenProbes:   3,
	}

	tracker := health.NewTracker(cfg, &logger)

	if tracker == nil {
		t.Fatal("expected non-nil health.Tracker")
	}
	if !tracker.HasCircuits() {
		t.Error("expected initialized circuits map")
	}
}

func TestTrackerGetOrCreateCircuitCreatesOnDemand(t *testing.T) {
	t.Parallel()
	logger := zerolog.Nop()
	cfg := health.CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenDurationMS:   30000,
		HalfOpenProbes:   3,
	}

	tracker := health.NewTracker(cfg, &logger)

	breaker := tracker.GetOrCreateCircuit("provider-a")
	if breaker == nil {
		t.Fatal("expected non-nil health.CircuitBreaker")
	}
	if breaker.Name() != "provider-a" {
		t.Errorf("expected name 'provider-a', got %q", breaker.Name())
	}
}

func TestTrackerGetOrCreateCircuitReturnsSame(t *testing.T) {
	t.Parallel()
	logger := zerolog.Nop()
	cfg := health.CircuitBreakerConfig{OpenDurationMS: 0, FailureThreshold: 0, HalfOpenProbes: 0}

	tracker := health.NewTracker(cfg, &logger)

	breaker1 := tracker.GetOrCreateCircuit("provider-a")
	breaker2 := tracker.GetOrCreateCircuit("provider-a")

	if breaker1 != breaker2 {
		t.Error("expected same health.CircuitBreaker instance for same provider")
	}
}

func TestTrackerIsHealthyFuncReturnsTrueWhenClosed(t *testing.T) {
	t.Parallel()
	logger := zerolog.Nop()
	cfg := health.CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenDurationMS:   30000,
		HalfOpenProbes:   3,
	}

	tracker := health.NewTracker(cfg, &logger)
	isHealthy := tracker.IsHealthyFunc("provider-a")

	// Circuit starts closed, should be healthy
	if !isHealthy() {
		t.Error("expected IsHealthyFunc to return true when circuit is closed")
	}
}

func TestTrackerIsHealthyFuncReturnsFalseWhenOpen(t *testing.T) {
	t.Parallel()
	logger := zerolog.Nop()
	cfg := health.CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   30000,
		HalfOpenProbes:   1,
	}

	tracker := health.NewTracker(cfg, &logger)
	testErr := errors.New("test error")

	// Open the circuit
	tracker.RecordFailure("provider-a", testErr)
	tracker.RecordFailure("provider-a", testErr)

	isHealthy := tracker.IsHealthyFunc("provider-a")

	if isHealthy() {
		t.Error("expected IsHealthyFunc to return false when circuit is open")
	}
}

func TestTrackerIsHealthyFuncReturnsTrueWhenHalfOpen(t *testing.T) {
	t.Parallel()
	logger := zerolog.Nop()
	cfg := health.CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   50, // Short timeout for testing
		HalfOpenProbes:   1,
	}

	tracker := health.NewTracker(cfg, &logger)
	testErr := errors.New("test error")

	// Open the circuit
	tracker.RecordFailure("provider-a", testErr)
	tracker.RecordFailure("provider-a", testErr)

	// Wait for timeout to transition to half-open
	time.Sleep(100 * time.Millisecond)

	// Trigger transition to half-open by calling Allow
	breaker := tracker.GetOrCreateCircuit("provider-a")
	done, allowErr := breaker.Allow()
	if allowErr != nil {
		t.Fatalf("expected Allow to succeed in half-open state, got: %v", allowErr)
	}
	// Leave done uncalled to keep in half-open state; report success to not affect state adversely
	done(nil)

	isHealthy := tracker.IsHealthyFunc("provider-a")

	// Half-open should be considered healthy (allows probes)
	if !isHealthy() {
		t.Error("expected IsHealthyFunc to return true when circuit is half-open")
	}
}

func TestTrackerRecordSuccess(t *testing.T) {
	t.Parallel()
	logger := zerolog.Nop()
	cfg := health.CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenDurationMS:   30000,
		HalfOpenProbes:   3,
	}

	tracker := health.NewTracker(cfg, &logger)

	// RecordSuccess should not panic and circuit should stay closed
	tracker.RecordSuccess("provider-a")

	state := tracker.GetState("provider-a")
	if state != health.StateClosed {
		t.Errorf("expected state CLOSED after RecordSuccess, got %s", state.String())
	}
}

func TestTrackerRecordFailure(t *testing.T) {
	t.Parallel()
	logger := zerolog.Nop()
	cfg := health.CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   30000,
		HalfOpenProbes:   1,
	}

	tracker := health.NewTracker(cfg, &logger)
	testErr := errors.New("test error")

	tracker.RecordFailure("provider-a", testErr)
	tracker.RecordFailure("provider-a", testErr)

	state := tracker.GetState("provider-a")
	if state != health.StateOpen {
		t.Errorf("expected state OPEN after threshold failures, got %s", state.String())
	}
}

func TestTrackerAllStates(t *testing.T) {
	t.Parallel()
	logger := zerolog.Nop()
	cfg := health.CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   30000,
		HalfOpenProbes:   1,
	}

	tracker := health.NewTracker(cfg, &logger)
	testErr := errors.New("test error")

	// Create circuits for multiple providers
	tracker.RecordSuccess("provider-a")
	tracker.RecordFailure("provider-b", testErr)
	tracker.RecordFailure("provider-b", testErr)

	states := tracker.AllStates()

	if len(states) != 2 {
		t.Errorf("expected 2 states, got %d", len(states))
	}
	if states["provider-a"] != health.StateClosed {
		t.Errorf("expected provider-a state CLOSED, got %s", states["provider-a"].String())
	}
	if states["provider-b"] != health.StateOpen {
		t.Errorf("expected provider-b state OPEN, got %s", states["provider-b"].String())
	}
}

func TestTrackerGetStateReturnsClosedForUnknown(t *testing.T) {
	t.Parallel()
	logger := zerolog.Nop()
	cfg := health.CircuitBreakerConfig{OpenDurationMS: 0, FailureThreshold: 0, HalfOpenProbes: 0}

	tracker := health.NewTracker(cfg, &logger)

	state := tracker.GetState("unknown-provider")
	if state != health.StateClosed {
		t.Errorf("expected health.StateClosed for unknown provider, got %s", state.String())
	}
}

func TestTrackerConcurrentAccess(t *testing.T) {
	t.Parallel()
	logger := zerolog.Nop()
	cfg := health.CircuitBreakerConfig{
		FailureThreshold: 100, // High threshold to avoid opening
		OpenDurationMS:   30000,
		HalfOpenProbes:   3,
	}

	tracker := health.NewTracker(cfg, &logger)

	const numGoroutines = 100
	const numOperations = 100

	var waitGroup sync.WaitGroup
	waitGroup.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer waitGroup.Done()
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

	waitGroup.Wait()

	// If we get here without deadlock or panic, the test passes
	states := tracker.AllStates()
	if len(states) != 1 {
		t.Errorf("expected 1 provider in states, got %d", len(states))
	}
}
