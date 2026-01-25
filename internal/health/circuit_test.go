package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewCircuitBreaker_DefaultSettings(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{}

	cb := NewCircuitBreaker("test-provider", cfg, &logger)

	if cb == nil {
		t.Fatal("expected non-nil CircuitBreaker")
	}
	if cb.Name() != "test-provider" {
		t.Errorf("expected name 'test-provider', got %q", cb.Name())
	}
	if cb.State() != StateClosed {
		t.Errorf("expected initial state CLOSED, got %s", cb.State().String())
	}
}

func TestCircuitBreaker_AllowWhenClosed(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenDurationMS:   1000,
		HalfOpenProbes:   3,
	}

	cb := NewCircuitBreaker("test-provider", cfg, &logger)

	done, err := cb.Allow()
	if err != nil {
		t.Fatalf("expected Allow to succeed when closed, got error: %v", err)
	}
	if done == nil {
		t.Fatal("expected non-nil done function")
	}

	done(nil)

	if cb.State() != StateClosed {
		t.Errorf("expected state CLOSED after success, got %s", cb.State().String())
	}
}

func TestCircuitBreaker_OpensAfterThresholdFailures(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 3,
		OpenDurationMS:   1000,
		HalfOpenProbes:   1,
	}

	cb := NewCircuitBreaker("test-provider", cfg, &logger)
	testErr := errors.New("test error")

	for i := 0; i < 3; i++ {
		done, err := cb.Allow()
		if err != nil {
			t.Fatalf("iteration %d: Allow failed before threshold: %v", i, err)
		}
		done(testErr)
	}

	if cb.State() != StateOpen {
		t.Errorf("expected state OPEN after %d failures, got %s", 3, cb.State().String())
	}

	_, err := cb.Allow()
	if err == nil {
		t.Error("expected Allow to fail when circuit is open")
	}
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_TransitionsToHalfOpenAfterTimeout(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   100,
		HalfOpenProbes:   1,
	}

	cb := NewCircuitBreaker("test-provider", cfg, &logger)
	testErr := errors.New("test error")

	for i := 0; i < 2; i++ {
		done, _ := cb.Allow()
		done(testErr)
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state OPEN, got %s", cb.State().String())
	}

	time.Sleep(150 * time.Millisecond)

	done, err := cb.Allow()
	if err != nil {
		t.Fatalf("expected Allow to succeed in half-open state, got error: %v", err)
	}

	if cb.State() != StateHalfOpen {
		t.Errorf("expected state HALF-OPEN after timeout, got %s", cb.State().String())
	}

	done(nil)
}

func TestCircuitBreaker_ClosesAfterSuccessfulProbes(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   50,
		HalfOpenProbes:   2,
	}

	cb := NewCircuitBreaker("test-provider", cfg, &logger)
	testErr := errors.New("test error")

	for i := 0; i < 2; i++ {
		done, _ := cb.Allow()
		done(testErr)
	}

	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 2; i++ {
		done, err := cb.Allow()
		if err != nil {
			t.Fatalf("probe %d: expected Allow to succeed, got error: %v", i, err)
		}
		done(nil)
	}

	if cb.State() != StateClosed {
		t.Errorf("expected state CLOSED after successful probes, got %s", cb.State().String())
	}
}

func TestCircuitBreaker_ContextCanceledNotFailure(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   1000,
		HalfOpenProbes:   1,
	}

	cb := NewCircuitBreaker("test-provider", cfg, &logger)

	for i := 0; i < 5; i++ {
		done, err := cb.Allow()
		if err != nil {
			t.Fatalf("iteration %d: Allow failed unexpectedly: %v", i, err)
		}
		done(context.Canceled)
	}

	if cb.State() != StateClosed {
		t.Errorf("expected state CLOSED after context.Canceled errors, got %s", cb.State().String())
	}
}

func TestCircuitBreaker_ReportSuccess(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenDurationMS:   1000,
		HalfOpenProbes:   3,
	}

	cb := NewCircuitBreaker("test-provider", cfg, &logger)

	recorded := cb.ReportSuccess()

	if !recorded {
		t.Error("expected ReportSuccess to return true when circuit is CLOSED")
	}

	if cb.State() != StateClosed {
		t.Errorf("expected state CLOSED, got %s", cb.State().String())
	}
}

func TestCircuitBreaker_ReportFailure(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   1000,
		HalfOpenProbes:   1,
	}

	cb := NewCircuitBreaker("test-provider", cfg, &logger)
	testErr := errors.New("test error")

	recorded := cb.ReportFailure(testErr)
	if !recorded {
		t.Error("expected ReportFailure to return true when circuit is CLOSED")
	}

	recorded = cb.ReportFailure(testErr)
	if !recorded {
		t.Error("expected ReportFailure to return true when circuit is CLOSED (second call)")
	}

	if cb.State() != StateOpen {
		t.Errorf("expected state OPEN after ReportFailure calls, got %s", cb.State().String())
	}
}

func TestCircuitBreaker_ReportSuccessWhenOpen(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   1000,
		HalfOpenProbes:   1,
	}

	cb := NewCircuitBreaker("test-provider", cfg, &logger)
	testErr := errors.New("test error")

	// Trip the circuit breaker to OPEN state
	for i := 0; i < 2; i++ {
		done, _ := cb.Allow()
		done(testErr)
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state OPEN, got %s", cb.State().String())
	}

	// Now try to report success when circuit is OPEN
	recorded := cb.ReportSuccess()
	if recorded {
		t.Error("expected ReportSuccess to return false when circuit is OPEN")
	}

	// Circuit should remain OPEN
	if cb.State() != StateOpen {
		t.Errorf("expected state to remain OPEN, got %s", cb.State().String())
	}
}

func TestCircuitBreaker_ReportFailureWhenOpen(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   1000,
		HalfOpenProbes:   1,
	}

	cb := NewCircuitBreaker("test-provider", cfg, &logger)
	testErr := errors.New("test error")

	// Trip the circuit breaker to OPEN state
	for i := 0; i < 2; i++ {
		done, _ := cb.Allow()
		done(testErr)
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state OPEN, got %s", cb.State().String())
	}

	// Now try to report failure when circuit is OPEN
	recorded := cb.ReportFailure(testErr)
	if recorded {
		t.Error("expected ReportFailure to return false when circuit is OPEN")
	}

	// Circuit should remain OPEN
	if cb.State() != StateOpen {
		t.Errorf("expected state to remain OPEN, got %s", cb.State().String())
	}
}

func TestCircuitBreaker_ReportSuccessWhenHalfOpen(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   50,
		HalfOpenProbes:   2,
	}

	cb := NewCircuitBreaker("test-provider", cfg, &logger)
	testErr := errors.New("test error")

	// Trip the circuit breaker to OPEN state
	for i := 0; i < 2; i++ {
		done, _ := cb.Allow()
		done(testErr)
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state OPEN, got %s", cb.State().String())
	}

	// Wait for circuit to transition to HALF-OPEN
	time.Sleep(100 * time.Millisecond)

	// First probe should succeed and return true
	recorded := cb.ReportSuccess()
	if !recorded {
		t.Error("expected ReportSuccess to return true when circuit is HALF-OPEN")
	}

	if cb.State() != StateHalfOpen {
		t.Errorf("expected state HALF-OPEN, got %s", cb.State().String())
	}
}

func TestCircuitBreaker_ReportFailureWhenHalfOpen(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   50,
		HalfOpenProbes:   2,
	}

	cb := NewCircuitBreaker("test-provider", cfg, &logger)
	testErr := errors.New("test error")

	// Trip the circuit breaker to OPEN state
	for i := 0; i < 2; i++ {
		done, _ := cb.Allow()
		done(testErr)
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state OPEN, got %s", cb.State().String())
	}

	// Wait for circuit to transition to HALF-OPEN
	time.Sleep(100 * time.Millisecond)

	// First probe should be allowed and return true
	recorded := cb.ReportFailure(testErr)
	if !recorded {
		t.Error("expected ReportFailure to return true when circuit is HALF-OPEN")
	}

	// After failure in HALF-OPEN, circuit should go back to OPEN
	if cb.State() != StateOpen {
		t.Errorf("expected state OPEN after failure in HALF-OPEN, got %s", cb.State().String())
	}
}

func TestShouldCountAsFailure(t *testing.T) {
	tests := []struct {
		err        error
		name       string
		statusCode int
		want       bool
	}{
		{name: "200 OK", statusCode: 200, err: nil, want: false},
		{name: "201 Created", statusCode: 201, err: nil, want: false},
		{name: "400 Bad Request", statusCode: 400, err: nil, want: false},
		{name: "401 Unauthorized", statusCode: 401, err: nil, want: false},
		{name: "403 Forbidden", statusCode: 403, err: nil, want: false},
		{name: "404 Not Found", statusCode: 404, err: nil, want: false},
		{name: "422 Unprocessable", statusCode: 422, err: nil, want: false},
		{name: "context canceled", statusCode: 0, err: context.Canceled, want: false},
		{name: "429 Rate Limited", statusCode: 429, err: nil, want: true},
		{name: "500 Internal Server Error", statusCode: 500, err: nil, want: true},
		{name: "502 Bad Gateway", statusCode: 502, err: nil, want: true},
		{name: "503 Service Unavailable", statusCode: 503, err: nil, want: true},
		{name: "504 Gateway Timeout", statusCode: 504, err: nil, want: true},
		{name: "network error", statusCode: 0, err: errors.New("connection refused"), want: true},
		{name: "timeout error", statusCode: 0, err: errors.New("timeout"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldCountAsFailure(tt.statusCode, tt.err)
			if got != tt.want {
				t.Errorf("ShouldCountAsFailure(%d, %v) = %v, want %v", tt.statusCode, tt.err, got, tt.want)
			}
		})
	}
}

func TestShouldCountAsFailure_WrappedContextCanceled(t *testing.T) {
	wrappedErr := errors.Join(errors.New("request failed"), context.Canceled)

	if ShouldCountAsFailure(0, wrappedErr) {
		t.Error("expected wrapped context.Canceled to NOT count as failure")
	}
}
