package router_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/router"
)

// mockNetError implements net.Error for testing.
type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return "mock network error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

// Ensure mockNetError implements net.Error at compile time.
var _ net.Error = &mockNetError{timeout: false, temporary: false}

func TestStatusCodeTriggerShouldFailover(t *testing.T) {
	t.Parallel()

	trigger := router.NewStatusCodeTrigger(429, 500, 502, 503, 504)

	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		// Should trigger
		{name: "429 rate limit", statusCode: 429, want: true},
		{name: "500 internal error", statusCode: 500, want: true},
		{name: "502 bad gateway", statusCode: 502, want: true},
		{name: "503 service unavailable", statusCode: 503, want: true},
		{name: "504 gateway timeout", statusCode: 504, want: true},
		// Should NOT trigger
		{name: "200 OK", statusCode: 200, want: false},
		{name: "201 Created", statusCode: 201, want: false},
		{name: "400 Bad Request", statusCode: 400, want: false},
		{name: "401 Unauthorized", statusCode: 401, want: false},
		{name: "403 Forbidden", statusCode: 403, want: false},
		{name: "404 Not Found", statusCode: 404, want: false},
		{name: "501 Not Implemented", statusCode: 501, want: false},
		{name: "0 no status", statusCode: 0, want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := trigger.ShouldFailover(nil, testCase.statusCode)
			if got != testCase.want {
				t.Errorf("router.ShouldFailover(nil, %d) = %v, want %v",
					testCase.statusCode, got, testCase.want)
			}
		})
	}
}

func TestStatusCodeTriggerName(t *testing.T) {
	t.Parallel()

	trigger := router.NewStatusCodeTrigger(429)
	if got := trigger.Name(); got != "status_code" {
		t.Errorf("Name() = %q, want %q", got, "status_code")
	}
}

func TestStatusCodeTriggerEmpty(t *testing.T) {
	t.Parallel()

	trigger := router.NewStatusCodeTrigger() // No codes configured
	if trigger.ShouldFailover(nil, 500) {
		t.Error("Empty trigger should not fire on any status code")
	}
}

func TestTimeoutTriggerShouldFailover(t *testing.T) {
	t.Parallel()

	trigger := router.NewTimeoutTrigger()

	wrappedDeadline := errors.Join(errors.New("request failed"), context.DeadlineExceeded)
	tests := []struct {
		err  error
		name string
		want bool
	}{
		{name: "context.DeadlineExceeded", err: context.DeadlineExceeded, want: true},
		{name: "wrapped DeadlineExceeded", err: wrappedDeadline, want: true},
		{name: "nil error", err: nil, want: false},
		{name: "generic error", err: errors.New("something went wrong"), want: false},
		{name: "context.Canceled", err: context.Canceled, want: false},
		{name: "io.EOF", err: errors.New("EOF"), want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := trigger.ShouldFailover(testCase.err, 0)
			if got != testCase.want {
				t.Errorf("router.ShouldFailover(%v, 0) = %v, want %v",
					testCase.err, got, testCase.want)
			}
		})
	}
}

func TestTimeoutTriggerName(t *testing.T) {
	t.Parallel()

	trigger := router.NewTimeoutTrigger()
	if got := trigger.Name(); got != "timeout" {
		t.Errorf("Name() = %q, want %q", got, "timeout")
	}
}

func TestTimeoutTriggerIgnoresStatusCode(t *testing.T) {
	t.Parallel()

	trigger := router.NewTimeoutTrigger()
	// Status code should be ignored
	if trigger.ShouldFailover(nil, 504) {
		t.Error("router.TimeoutTrigger should not fire on 504 status code alone")
	}
	if !trigger.ShouldFailover(context.DeadlineExceeded, 200) {
		t.Error("router.TimeoutTrigger should fire on DeadlineExceeded regardless of status code")
	}
}

func TestConnectionTriggerShouldFailover(t *testing.T) {
	t.Parallel()

	trigger := router.NewConnectionTrigger()

	tests := []struct {
		err  error
		name string
		want bool
	}{
		{name: "net.Error timeout", err: &mockNetError{timeout: true, temporary: false}, want: true},
		{name: "net.Error temporary", err: &mockNetError{timeout: false, temporary: true}, want: true},
		{name: "net.Error basic", err: &mockNetError{timeout: false, temporary: false}, want: true},
		{name: "nil error", err: nil, want: false},
		{name: "generic error", err: errors.New("not a network error"), want: false},
		// Note: context.DeadlineExceeded satisfies net.Error interface in Go stdlib
		{name: "context.DeadlineExceeded", err: context.DeadlineExceeded, want: true},
		{name: "context.Canceled", err: context.Canceled, want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := trigger.ShouldFailover(testCase.err, 0)
			if got != testCase.want {
				t.Errorf("router.ShouldFailover(%v, 0) = %v, want %v",
					testCase.err, got, testCase.want)
			}
		})
	}
}

func TestConnectionTriggerName(t *testing.T) {
	t.Parallel()

	trigger := router.NewConnectionTrigger()
	if got := trigger.Name(); got != "connection" {
		t.Errorf("Name() = %q, want %q", got, "connection")
	}
}

func TestConnectionTriggerWrappedNetError(t *testing.T) {
	t.Parallel()

	trigger := router.NewConnectionTrigger()
	wrappedErr := errors.Join(
		errors.New("connection failed"),
		&mockNetError{timeout: true, temporary: false},
	)

	if !trigger.ShouldFailover(wrappedErr, 0) {
		t.Error("router.ConnectionTrigger should fire on wrapped net.Error")
	}
}

func TestDefaultTriggers(t *testing.T) {
	t.Parallel()

	triggers := router.DefaultTriggers()

	if len(triggers) != 3 {
		t.Fatalf("router.DefaultTriggers() returned %d triggers, want 3", len(triggers))
	}

	// Verify each trigger type is present
	names := make(map[string]bool)
	for _, trigger := range triggers {
		names[trigger.Name()] = true
	}

	expectedNames := []string{"status_code", "timeout", "connection"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("router.DefaultTriggers() missing %q trigger", name)
		}
	}
}

func TestDefaultTriggersStatusCodes(t *testing.T) {
	t.Parallel()

	triggers := router.DefaultTriggers()

	// Find the status code trigger
	var statusTrigger router.FailoverTrigger
	for _, trigger := range triggers {
		if trigger.Name() == "status_code" {
			statusTrigger = trigger
			break
		}
	}

	if statusTrigger == nil {
		t.Fatal("router.DefaultTriggers() missing status_code trigger")
	}

	// Verify expected status codes trigger failover
	expectedCodes := []int{429, 500, 502, 503, 504}
	for _, code := range expectedCodes {
		if !statusTrigger.ShouldFailover(nil, code) {
			t.Errorf("router.DefaultTriggers status_code should fire on %d", code)
		}
	}
}

func TestShouldFailoverStatusCode(t *testing.T) {
	t.Parallel()

	triggers := router.DefaultTriggers()

	tests := []struct {
		err        error
		name       string
		statusCode int
		want       bool
	}{
		{name: "429 rate limit", err: nil, statusCode: 429, want: true},
		{name: "500 server error", err: nil, statusCode: 500, want: true},
		{name: "200 OK", err: nil, statusCode: 200, want: false},
		{name: "404 not found", err: nil, statusCode: 404, want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := router.ShouldFailover(triggers, testCase.err, testCase.statusCode)
			if got != testCase.want {
				t.Errorf("router.ShouldFailover(triggers, %v, %d) = %v, want %v",
					testCase.err, testCase.statusCode, got, testCase.want)
			}
		})
	}
}

func TestShouldFailoverTimeout(t *testing.T) {
	t.Parallel()

	triggers := router.DefaultTriggers()

	if !router.ShouldFailover(triggers, context.DeadlineExceeded, 0) {
		t.Error("router.ShouldFailover should return true for DeadlineExceeded")
	}

	if router.ShouldFailover(triggers, context.Canceled, 0) {
		t.Error("router.ShouldFailover should return false for Canceled")
	}
}

func TestShouldFailoverConnection(t *testing.T) {
	t.Parallel()

	triggers := router.DefaultTriggers()

	netErr := &mockNetError{timeout: true, temporary: false}
	if !router.ShouldFailover(triggers, netErr, 0) {
		t.Error("router.ShouldFailover should return true for net.Error")
	}

	genericErr := errors.New("generic error")
	if router.ShouldFailover(triggers, genericErr, 200) {
		t.Error("router.ShouldFailover should return false for generic error with 200 status")
	}
}

func TestShouldFailoverEmptyTriggers(t *testing.T) {
	t.Parallel()

	if router.ShouldFailover(nil, context.DeadlineExceeded, 500) {
		t.Error("router.ShouldFailover should return false for nil triggers")
	}

	if router.ShouldFailover([]router.FailoverTrigger{}, context.DeadlineExceeded, 500) {
		t.Error("router.ShouldFailover should return false for empty triggers")
	}
}

func TestShouldFailoverShortCircuit(t *testing.T) {
	t.Parallel()

	// Create triggers where first one fires
	triggers := []router.FailoverTrigger{
		router.NewStatusCodeTrigger(429),
		router.NewTimeoutTrigger(),
	}

	// 429 should trigger immediately (first trigger)
	if !router.ShouldFailover(triggers, nil, 429) {
		t.Error("router.ShouldFailover should return true on first matching trigger")
	}
}

func TestFindMatchingTrigger(t *testing.T) {
	t.Parallel()

	triggers := router.DefaultTriggers()

	tests := []struct {
		err           error
		name          string
		wantName      string
		statusCode    int
		wantNilResult bool
	}{
		{
			name: "429 finds status_code", err: nil,
			statusCode: 429, wantName: router.TriggerStatusCode, wantNilResult: false,
		},
		{
			name: "DeadlineExceeded finds timeout", err: context.DeadlineExceeded,
			statusCode: 0, wantName: router.TriggerTimeout, wantNilResult: false,
		},
		{
			name:       "net.Error finds connection",
			err:        &mockNetError{timeout: false, temporary: false},
			statusCode: 0, wantName: router.TriggerConnection, wantNilResult: false,
		},
		{
			name: "200 OK finds nothing", err: nil,
			statusCode: 200, wantName: "", wantNilResult: true,
		},
		{
			name: "generic error finds nothing", err: errors.New("error"),
			statusCode: 200, wantName: "", wantNilResult: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := router.FindMatchingTrigger(triggers, testCase.err, testCase.statusCode)

			if testCase.wantNilResult {
				if got != nil {
					t.Errorf("router.FindMatchingTrigger() = %v, want nil", got.Name())
				}
				return
			}

			if got == nil {
				t.Fatal("router.FindMatchingTrigger() = nil, want non-nil")
			}
			if got.Name() != testCase.wantName {
				t.Errorf("router.FindMatchingTrigger().Name() = %q, want %q",
					got.Name(), testCase.wantName)
			}
		})
	}
}

func TestFindMatchingTriggerEmpty(t *testing.T) {
	t.Parallel()

	if got := router.FindMatchingTrigger(nil, context.DeadlineExceeded, 500); got != nil {
		t.Errorf("router.FindMatchingTrigger(nil, ...) = %v, want nil", got)
	}

	if got := router.FindMatchingTrigger(
		[]router.FailoverTrigger{}, context.DeadlineExceeded, 500,
	); got != nil {
		t.Errorf("router.FindMatchingTrigger([], ...) = %v, want nil", got)
	}
}

// TestRealNetworkError tests with a real network error scenario.
func TestRealNetworkError(t *testing.T) {
	t.Parallel()

	trigger := router.NewConnectionTrigger()

	// Create a real dial error by trying to connect to an invalid address
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	var dialer net.Dialer
	_, err := dialer.DialContext(ctx, "tcp", "192.0.2.1:1") // TEST-NET-1, guaranteed unreachable

	if err == nil {
		t.Skip("Connection unexpectedly succeeded")
	}

	// The error should be a net.Error (either timeout or connection refused)
	if !trigger.ShouldFailover(err, 0) {
		t.Errorf("router.ConnectionTrigger should fire on real dial error: %v", err)
	}
}
