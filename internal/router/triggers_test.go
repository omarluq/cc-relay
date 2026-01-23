package router

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// mockNetError implements net.Error for testing.
type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return "mock network error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

// Ensure mockNetError implements net.Error.
var _ net.Error = (*mockNetError)(nil)

func TestStatusCodeTrigger_ShouldFailover(t *testing.T) {
	t.Parallel()

	trigger := NewStatusCodeTrigger(429, 500, 502, 503, 504)

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := trigger.ShouldFailover(nil, tt.statusCode)
			if got != tt.want {
				t.Errorf("ShouldFailover(nil, %d) = %v, want %v", tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestStatusCodeTrigger_Name(t *testing.T) {
	t.Parallel()

	trigger := NewStatusCodeTrigger(429)
	if got := trigger.Name(); got != "status_code" {
		t.Errorf("Name() = %q, want %q", got, "status_code")
	}
}

func TestStatusCodeTrigger_Empty(t *testing.T) {
	t.Parallel()

	trigger := NewStatusCodeTrigger() // No codes configured
	if trigger.ShouldFailover(nil, 500) {
		t.Error("Empty trigger should not fire on any status code")
	}
}

func TestTimeoutTrigger_ShouldFailover(t *testing.T) {
	t.Parallel()

	trigger := NewTimeoutTrigger()

	wrappedDeadline := errors.Join(errors.New("request failed"), context.DeadlineExceeded)
	//nolint:govet // fieldalignment: struct ordered for clarity over memory optimization
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "context.DeadlineExceeded", err: context.DeadlineExceeded, want: true},
		{name: "wrapped DeadlineExceeded", err: wrappedDeadline, want: true},
		{name: "nil error", err: nil, want: false},
		{name: "generic error", err: errors.New("something went wrong"), want: false},
		{name: "context.Canceled", err: context.Canceled, want: false},
		{name: "io.EOF", err: errors.New("EOF"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := trigger.ShouldFailover(tt.err, 0)
			if got != tt.want {
				t.Errorf("ShouldFailover(%v, 0) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestTimeoutTrigger_Name(t *testing.T) {
	t.Parallel()

	trigger := NewTimeoutTrigger()
	if got := trigger.Name(); got != "timeout" {
		t.Errorf("Name() = %q, want %q", got, "timeout")
	}
}

func TestTimeoutTrigger_IgnoresStatusCode(t *testing.T) {
	t.Parallel()

	trigger := NewTimeoutTrigger()
	// Status code should be ignored
	if trigger.ShouldFailover(nil, 504) {
		t.Error("TimeoutTrigger should not fire on 504 status code alone")
	}
	if !trigger.ShouldFailover(context.DeadlineExceeded, 200) {
		t.Error("TimeoutTrigger should fire on DeadlineExceeded regardless of status code")
	}
}

func TestConnectionTrigger_ShouldFailover(t *testing.T) {
	t.Parallel()

	trigger := NewConnectionTrigger()

	//nolint:govet // fieldalignment: struct ordered for clarity over memory optimization
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "net.Error timeout", err: &mockNetError{timeout: true}, want: true},
		{name: "net.Error temporary", err: &mockNetError{temporary: true}, want: true},
		{name: "net.Error basic", err: &mockNetError{}, want: true},
		{name: "nil error", err: nil, want: false},
		{name: "generic error", err: errors.New("not a network error"), want: false},
		// Note: context.DeadlineExceeded satisfies net.Error interface in Go stdlib
		{name: "context.DeadlineExceeded", err: context.DeadlineExceeded, want: true},
		{name: "context.Canceled", err: context.Canceled, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := trigger.ShouldFailover(tt.err, 0)
			if got != tt.want {
				t.Errorf("ShouldFailover(%v, 0) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestConnectionTrigger_Name(t *testing.T) {
	t.Parallel()

	trigger := NewConnectionTrigger()
	if got := trigger.Name(); got != "connection" {
		t.Errorf("Name() = %q, want %q", got, "connection")
	}
}

func TestConnectionTrigger_WrappedNetError(t *testing.T) {
	t.Parallel()

	trigger := NewConnectionTrigger()
	wrappedErr := errors.Join(errors.New("connection failed"), &mockNetError{timeout: true})

	if !trigger.ShouldFailover(wrappedErr, 0) {
		t.Error("ConnectionTrigger should fire on wrapped net.Error")
	}
}

func TestDefaultTriggers(t *testing.T) {
	t.Parallel()

	triggers := DefaultTriggers()

	if len(triggers) != 3 {
		t.Fatalf("DefaultTriggers() returned %d triggers, want 3", len(triggers))
	}

	// Verify each trigger type is present
	names := make(map[string]bool)
	for _, tr := range triggers {
		names[tr.Name()] = true
	}

	expectedNames := []string{"status_code", "timeout", "connection"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("DefaultTriggers() missing %q trigger", name)
		}
	}
}

func TestDefaultTriggers_StatusCodes(t *testing.T) {
	t.Parallel()

	triggers := DefaultTriggers()

	// Find the status code trigger
	var statusTrigger FailoverTrigger
	for _, tr := range triggers {
		if tr.Name() == "status_code" {
			statusTrigger = tr
			break
		}
	}

	if statusTrigger == nil {
		t.Fatal("DefaultTriggers() missing status_code trigger")
	}

	// Verify expected status codes trigger failover
	expectedCodes := []int{429, 500, 502, 503, 504}
	for _, code := range expectedCodes {
		if !statusTrigger.ShouldFailover(nil, code) {
			t.Errorf("DefaultTriggers status_code should fire on %d", code)
		}
	}
}

func TestShouldFailover_StatusCode(t *testing.T) {
	t.Parallel()

	triggers := DefaultTriggers()

	//nolint:govet // fieldalignment: struct ordered for clarity over memory optimization
	tests := []struct {
		name       string
		err        error
		statusCode int
		want       bool
	}{
		{name: "429 rate limit", err: nil, statusCode: 429, want: true},
		{name: "500 server error", err: nil, statusCode: 500, want: true},
		{name: "200 OK", err: nil, statusCode: 200, want: false},
		{name: "404 not found", err: nil, statusCode: 404, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ShouldFailover(triggers, tt.err, tt.statusCode)
			if got != tt.want {
				t.Errorf("ShouldFailover(triggers, %v, %d) = %v, want %v",
					tt.err, tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestShouldFailover_Timeout(t *testing.T) {
	t.Parallel()

	triggers := DefaultTriggers()

	if !ShouldFailover(triggers, context.DeadlineExceeded, 0) {
		t.Error("ShouldFailover should return true for DeadlineExceeded")
	}

	if ShouldFailover(triggers, context.Canceled, 0) {
		t.Error("ShouldFailover should return false for Canceled")
	}
}

func TestShouldFailover_Connection(t *testing.T) {
	t.Parallel()

	triggers := DefaultTriggers()

	netErr := &mockNetError{timeout: true}
	if !ShouldFailover(triggers, netErr, 0) {
		t.Error("ShouldFailover should return true for net.Error")
	}

	genericErr := errors.New("generic error")
	if ShouldFailover(triggers, genericErr, 200) {
		t.Error("ShouldFailover should return false for generic error with 200 status")
	}
}

func TestShouldFailover_EmptyTriggers(t *testing.T) {
	t.Parallel()

	if ShouldFailover(nil, context.DeadlineExceeded, 500) {
		t.Error("ShouldFailover should return false for nil triggers")
	}

	if ShouldFailover([]FailoverTrigger{}, context.DeadlineExceeded, 500) {
		t.Error("ShouldFailover should return false for empty triggers")
	}
}

func TestShouldFailover_ShortCircuit(t *testing.T) {
	t.Parallel()

	// Create triggers where first one fires
	triggers := []FailoverTrigger{
		NewStatusCodeTrigger(429),
		NewTimeoutTrigger(),
	}

	// 429 should trigger immediately (first trigger)
	if !ShouldFailover(triggers, nil, 429) {
		t.Error("ShouldFailover should return true on first matching trigger")
	}
}

func TestFindMatchingTrigger(t *testing.T) {
	t.Parallel()

	triggers := DefaultTriggers()

	//nolint:govet // fieldalignment: struct ordered for clarity over memory optimization
	tests := []struct {
		name          string
		err           error
		statusCode    int
		wantName      string
		wantNilResult bool
	}{
		{
			name: "429 finds status_code", err: nil,
			statusCode: 429, wantName: TriggerStatusCode, wantNilResult: false,
		},
		{
			name: "DeadlineExceeded finds timeout", err: context.DeadlineExceeded,
			statusCode: 0, wantName: TriggerTimeout, wantNilResult: false,
		},
		{
			name: "net.Error finds connection", err: &mockNetError{},
			statusCode: 0, wantName: TriggerConnection, wantNilResult: false,
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FindMatchingTrigger(triggers, tt.err, tt.statusCode)

			if tt.wantNilResult {
				if got != nil {
					t.Errorf("FindMatchingTrigger() = %v, want nil", got.Name())
				}
				return
			}

			if got == nil {
				t.Fatal("FindMatchingTrigger() = nil, want non-nil")
			}
			if got.Name() != tt.wantName {
				t.Errorf("FindMatchingTrigger().Name() = %q, want %q", got.Name(), tt.wantName)
			}
		})
	}
}

func TestFindMatchingTrigger_Empty(t *testing.T) {
	t.Parallel()

	if got := FindMatchingTrigger(nil, context.DeadlineExceeded, 500); got != nil {
		t.Errorf("FindMatchingTrigger(nil, ...) = %v, want nil", got)
	}

	if got := FindMatchingTrigger([]FailoverTrigger{}, context.DeadlineExceeded, 500); got != nil {
		t.Errorf("FindMatchingTrigger([], ...) = %v, want nil", got)
	}
}

// TestRealNetworkError tests with a real network error scenario.
func TestRealNetworkError(t *testing.T) {
	t.Parallel()

	trigger := NewConnectionTrigger()

	// Create a real dial error by trying to connect to an invalid address
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	var d net.Dialer
	_, err := d.DialContext(ctx, "tcp", "192.0.2.1:1") // TEST-NET-1, guaranteed unreachable

	if err == nil {
		t.Skip("Connection unexpectedly succeeded")
	}

	// The error should be a net.Error (either timeout or connection refused)
	if !trigger.ShouldFailover(err, 0) {
		t.Errorf("ConnectionTrigger should fire on real dial error: %v", err)
	}
}
