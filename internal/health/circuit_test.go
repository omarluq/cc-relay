package health_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/health"
)

const testProviderName = "test-provider"

func TestNewCircuitBreakerDefaultSettings(t *testing.T) {
	t.Parallel()

	breaker := health.NewTestBreaker(0, 0, 0)

	if breaker == nil {
		t.Fatal("expected non-nil health.CircuitBreaker")
	}
	if breaker.Name() != "test-provider" {
		t.Errorf("expected name 'test-provider', got %q", breaker.Name())
	}
	if breaker.State() != health.StateClosed {
		t.Errorf("expected initial state CLOSED, got %s", breaker.State().String())
	}
}

func TestCircuitBreakerAllowWhenClosed(t *testing.T) {
	t.Parallel()

	breaker := health.NewTestBreaker(5, 1000, 3)

	done, err := breaker.Allow()
	if err != nil {
		t.Fatalf("expected Allow to succeed when closed, got error: %v", err)
	}
	if done == nil {
		t.Fatal("expected non-nil done function")
	}

	done(nil)

	if breaker.State() != health.StateClosed {
		t.Errorf("expected state CLOSED after success, got %s", breaker.State().String())
	}
}

func TestCircuitBreakerOpensAfterThresholdFailures(t *testing.T) {
	t.Parallel()

	breaker := health.NewTestBreaker(3, 1000, 1)
	testErr := errors.New("test error")

	for i := 0; i < 3; i++ {
		done, allowErr := breaker.Allow()
		if allowErr != nil {
			t.Fatalf("iteration %d: Allow failed before threshold: %v", i, allowErr)
		}
		done(testErr)
	}

	if breaker.State() != health.StateOpen {
		t.Errorf("expected state OPEN after %d failures, got %s", 3, breaker.State().String())
	}

	_, err := breaker.Allow()
	if err == nil {
		t.Error("expected Allow to fail when circuit is open")
	}
	if !errors.Is(err, health.ErrCircuitOpen) {
		t.Errorf("expected health.ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreakerTransitionsToHalfOpenAfterTimeout(t *testing.T) {
	t.Parallel()

	breaker := health.NewTestBreaker(2, 100, 1)
	testErr := errors.New("test error")

	for i := 0; i < 2; i++ {
		done, allowErr := breaker.Allow()
		if allowErr != nil {
			t.Fatalf("Allow failed: %v", allowErr)
		}
		done(testErr)
	}

	if breaker.State() != health.StateOpen {
		t.Fatalf("expected state OPEN, got %s", breaker.State().String())
	}

	time.Sleep(150 * time.Millisecond)

	done, err := breaker.Allow()
	if err != nil {
		t.Fatalf("expected Allow to succeed in half-open state, got error: %v", err)
	}

	if breaker.State() != health.StateHalfOpen {
		t.Errorf("expected state HALF-OPEN after timeout, got %s", breaker.State().String())
	}

	done(nil)
}

func TestCircuitBreakerClosesAfterSuccessfulProbes(t *testing.T) {
	t.Parallel()

	breaker := health.NewTestBreaker(2, 50, 2)
	testErr := errors.New("test error")

	for i := 0; i < 2; i++ {
		done, allowErr := breaker.Allow()
		if allowErr != nil {
			t.Fatalf("iteration %d: Allow failed: %v", i, allowErr)
		}
		done(testErr)
	}

	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 2; i++ {
		done, allowErr := breaker.Allow()
		if allowErr != nil {
			t.Fatalf("probe %d: expected Allow to succeed, got error: %v", i, allowErr)
		}
		done(nil)
	}

	if breaker.State() != health.StateClosed {
		t.Errorf("expected state CLOSED after successful probes, got %s", breaker.State().String())
	}
}

func TestCircuitBreakerContextCanceledNotFailure(t *testing.T) {
	t.Parallel()

	breaker := health.NewTestBreaker(2, 1000, 1)

	for i := 0; i < 5; i++ {
		done, allowErr := breaker.Allow()
		if allowErr != nil {
			t.Fatalf("iteration %d: Allow failed unexpectedly: %v", i, allowErr)
		}
		done(context.Canceled)
	}

	if breaker.State() != health.StateClosed {
		t.Errorf("expected state CLOSED after context.Canceled errors, got %s", breaker.State().String())
	}
}

func TestCircuitBreakerReportSuccess(t *testing.T) {
	t.Parallel()

	breaker := health.NewTestBreaker(5, 1000, 3)

	recorded := breaker.ReportSuccess()

	if !recorded {
		t.Error("expected ReportSuccess to return true when circuit is CLOSED")
	}

	if breaker.State() != health.StateClosed {
		t.Errorf("expected state CLOSED, got %s", breaker.State().String())
	}
}

func TestCircuitBreakerReportFailure(t *testing.T) {
	t.Parallel()

	breaker := health.NewTestBreaker(2, 1000, 1)
	testErr := errors.New("test error")

	recorded := breaker.ReportFailure(testErr)
	if !recorded {
		t.Error("expected ReportFailure to return true when circuit is CLOSED")
	}

	recorded = breaker.ReportFailure(testErr)
	if !recorded {
		t.Error("expected ReportFailure to return true when circuit is CLOSED (second call)")
	}

	if breaker.State() != health.StateOpen {
		t.Errorf("expected state OPEN after ReportFailure calls, got %s", breaker.State().String())
	}
}

func TestCircuitBreakerReportSuccessWhenOpen(t *testing.T) {
	t.Parallel()

	breaker := health.NewTestBreaker(2, 1000, 1)
	testErr := errors.New("test error")

	// Trip the circuit breaker to OPEN state
	for i := 0; i < 2; i++ {
		done, allowErr := breaker.Allow()
		if allowErr != nil {
			t.Fatalf("Allow failed: %v", allowErr)
		}
		done(testErr)
	}

	if breaker.State() != health.StateOpen {
		t.Fatalf("expected state OPEN, got %s", breaker.State().String())
	}

	// Now try to report success when circuit is OPEN
	recorded := breaker.ReportSuccess()
	if recorded {
		t.Error("expected ReportSuccess to return false when circuit is OPEN")
	}

	// Circuit should remain OPEN
	if breaker.State() != health.StateOpen {
		t.Errorf("expected state to remain OPEN, got %s", breaker.State().String())
	}
}

func TestCircuitBreakerReportFailureWhenOpen(t *testing.T) {
	t.Parallel()

	breaker := health.NewTestBreaker(2, 1000, 1)
	testErr := errors.New("test error")

	// Trip the circuit breaker to OPEN state
	for i := 0; i < 2; i++ {
		done, allowErr := breaker.Allow()
		if allowErr != nil {
			t.Fatalf("Allow failed: %v", allowErr)
		}
		done(testErr)
	}

	if breaker.State() != health.StateOpen {
		t.Fatalf("expected state OPEN, got %s", breaker.State().String())
	}

	// Now try to report failure when circuit is OPEN
	recorded := breaker.ReportFailure(testErr)
	if recorded {
		t.Error("expected ReportFailure to return false when circuit is OPEN")
	}

	// Circuit should remain OPEN
	if breaker.State() != health.StateOpen {
		t.Errorf("expected state to remain OPEN, got %s", breaker.State().String())
	}
}

func TestCircuitBreakerReportSuccessWhenHalfOpen(t *testing.T) {
	t.Parallel()

	breaker := health.NewTestBreaker(2, 50, 2)
	testErr := errors.New("test error")

	// Trip the circuit breaker to OPEN state
	for i := 0; i < 2; i++ {
		done, allowErr := breaker.Allow()
		if allowErr != nil {
			t.Fatalf("Allow failed: %v", allowErr)
		}
		done(testErr)
	}

	if breaker.State() != health.StateOpen {
		t.Fatalf("expected state OPEN, got %s", breaker.State().String())
	}

	// Wait for circuit to transition to HALF-OPEN
	time.Sleep(100 * time.Millisecond)

	// First probe should succeed and return true
	recorded := breaker.ReportSuccess()
	if !recorded {
		t.Error("expected ReportSuccess to return true when circuit is HALF-OPEN")
	}

	if breaker.State() != health.StateHalfOpen {
		t.Errorf("expected state HALF-OPEN, got %s", breaker.State().String())
	}
}

func TestCircuitBreakerReportFailureWhenHalfOpen(t *testing.T) {
	t.Parallel()

	breaker := health.NewTestBreaker(2, 50, 2)
	testErr := errors.New("test error")

	// Trip the circuit breaker to OPEN state
	for i := 0; i < 2; i++ {
		done, allowErr := breaker.Allow()
		if allowErr != nil {
			t.Fatalf("Allow failed: %v", allowErr)
		}
		done(testErr)
	}

	if breaker.State() != health.StateOpen {
		t.Fatalf("expected state OPEN, got %s", breaker.State().String())
	}

	// Wait for circuit to transition to HALF-OPEN
	time.Sleep(100 * time.Millisecond)

	// First probe should be allowed and return true
	recorded := breaker.ReportFailure(testErr)
	if !recorded {
		t.Error("expected ReportFailure to return true when circuit is HALF-OPEN")
	}

	// After failure in HALF-OPEN, circuit should go back to OPEN
	if breaker.State() != health.StateOpen {
		t.Errorf("expected state OPEN after failure in HALF-OPEN, got %s", breaker.State().String())
	}
}

func TestShouldCountAsFailure(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			got := health.ShouldCountAsFailure(tt.statusCode, tt.err)
			if got != tt.want {
				t.Errorf("health.ShouldCountAsFailure(%d, %v) = %v, want %v", tt.statusCode, tt.err, got, tt.want)
			}
		})
	}
}

func TestShouldCountAsFailureWrappedContextCanceled(t *testing.T) {
	t.Parallel()
	wrappedErr := errors.Join(errors.New("request failed"), context.Canceled)

	if health.ShouldCountAsFailure(0, wrappedErr) {
		t.Error("expected wrapped context.Canceled to NOT count as failure")
	}
}
