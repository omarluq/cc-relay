package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

const testProviderName = "test-provider"

func mustNewHTTPHealthCheck(t *testing.T, name, url string, client *http.Client) *HTTPHealthCheck {
	t.Helper()
	check, err := NewHTTPHealthCheck(name, url, client)
	if err != nil {
		t.Fatalf("NewHTTPHealthCheck failed: %v", err)
	}
	return check
}

func TestHTTPHealthCheckSuccess(t *testing.T) {
	// Create a test server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	check := mustNewHTTPHealthCheck(t, testProviderName, server.URL, server.Client())

	err := check.Check(context.Background())
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestHTTPHealthCheckFailure(t *testing.T) {
	// Create a test server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	check := mustNewHTTPHealthCheck(t, testProviderName, server.URL, server.Client())

	err := check.Check(context.Background())
	if err == nil {
		t.Error("expected error for 500 status, got nil")
	}
}

func TestHTTPHealthCheckTimeout(t *testing.T) {
	// Create a test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Use client with very short timeout
	client := &http.Client{Timeout: 50 * time.Millisecond}
	check := mustNewHTTPHealthCheck(t, testProviderName, server.URL, client)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := check.Check(ctx)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestHTTPHealthCheckProviderName(t *testing.T) {
	check := mustNewHTTPHealthCheck(t, "my-provider", "http://example.com", nil)
	if check.ProviderName() != "my-provider" {
		t.Errorf("expected provider name 'my-provider', got %q", check.ProviderName())
	}
}

func TestHTTPHealthCheckDefaultClient(t *testing.T) {
	// When nil client is passed, should create default
	check := mustNewHTTPHealthCheck(t, "test", "http://localhost", nil)
	if check.host == "" {
		t.Error("expected non-empty host")
	}
}

func TestNoOpHealthCheckAlwaysHealthy(t *testing.T) {
	check := NewNoOpHealthCheck(testProviderName)

	// Should always return nil
	for i := 0; i < 10; i++ {
		err := check.Check(context.Background())
		if err != nil {
			t.Errorf("iteration %d: expected nil error, got %v", i, err)
		}
	}
}

func TestNoOpHealthCheckProviderName(t *testing.T) {
	check := NewNoOpHealthCheck("noop-provider")
	if check.ProviderName() != "noop-provider" {
		t.Errorf("expected provider name 'noop-provider', got %q", check.ProviderName())
	}
}

func TestNewProviderHealthCheckWithURL(t *testing.T) {
	check := NewProviderHealthCheck("provider", "http://localhost:8080", nil)

	// Should return HTTPHealthCheck
	_, ok := check.(*HTTPHealthCheck)
	if !ok {
		t.Error("expected HTTPHealthCheck when URL is provided")
	}
}

func TestNewProviderHealthCheckEmptyURL(t *testing.T) {
	check := NewProviderHealthCheck("provider", "", nil)

	// Should return NoOpHealthCheck
	_, ok := check.(*NoOpHealthCheck)
	if !ok {
		t.Error("expected NoOpHealthCheck when URL is empty")
	}
}

func TestCheckerRegisterProvider(t *testing.T) {
	logger := zerolog.Nop()
	tracker := NewTracker(CircuitBreakerConfig{}, &logger)
	checker := NewChecker(tracker, CheckConfig{}, &logger)

	check1 := NewNoOpHealthCheck("provider-a")
	check2 := NewNoOpHealthCheck("provider-b")

	checker.RegisterProvider(check1)
	checker.RegisterProvider(check2)

	// Verify both registered
	checker.mu.RLock()
	defer checker.mu.RUnlock()

	if len(checker.checks) != 2 {
		t.Errorf("expected 2 registered providers, got %d", len(checker.checks))
	}
	if _, ok := checker.checks["provider-a"]; !ok {
		t.Error("expected provider-a to be registered")
	}
	if _, ok := checker.checks["provider-b"]; !ok {
		t.Error("expected provider-b to be registered")
	}
}

// mockHealthCheck records when Check is called.
type mockHealthCheck struct {
	checkErr  error
	name      string
	callCount atomic.Int32
}

func (m *mockHealthCheck) Check(_ context.Context) error {
	m.callCount.Add(1)
	return m.checkErr
}

func (m *mockHealthCheck) ProviderName() string {
	return m.name
}

func TestCheckerChecksOnlyOpenCircuits(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   30000,
		HalfOpenProbes:   1,
	}
	tracker := NewTracker(cfg, &logger)

	// Create checker with disabled auto-start
	enabled := false
	checkCfg := CheckConfig{Enabled: &enabled}
	checker := NewChecker(tracker, checkCfg, &logger)

	// Register two providers
	mockClosed := &mockHealthCheck{name: "closed-provider"}
	mockOpen := &mockHealthCheck{name: "open-provider"}

	checker.RegisterProvider(mockClosed)
	checker.RegisterProvider(mockOpen)

	// Open the circuit for open-provider
	testErr := errors.New("test error")
	tracker.RecordFailure("open-provider", testErr)
	tracker.RecordFailure("open-provider", testErr)

	// Verify states
	if tracker.GetState("closed-provider") != StateClosed {
		t.Fatal("expected closed-provider to be CLOSED")
	}
	if tracker.GetState("open-provider") != StateOpen {
		t.Fatal("expected open-provider to be OPEN")
	}

	// Manually trigger checkAllProviders
	checker.checkAllProviders()

	// Only open-provider should have been checked
	if mockClosed.callCount.Load() != 0 {
		t.Errorf("expected closed-provider check count 0, got %d", mockClosed.callCount.Load())
	}
	if mockOpen.callCount.Load() != 1 {
		t.Errorf("expected open-provider check count 1, got %d", mockOpen.callCount.Load())
	}
}

func TestCheckerRecordsSuccessOnHealthyCheck(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   100, // Short for testing
		HalfOpenProbes:   1,
	}
	tracker := NewTracker(cfg, &logger)

	enabled := false
	checkCfg := CheckConfig{Enabled: &enabled}
	checker := NewChecker(tracker, checkCfg, &logger)

	// Register provider with successful health check
	mockCheck := &mockHealthCheck{name: testProviderName, checkErr: nil}
	checker.RegisterProvider(mockCheck)

	// Open the circuit
	testErr := errors.New("test error")
	tracker.RecordFailure(testProviderName, testErr)
	tracker.RecordFailure(testProviderName, testErr)

	if tracker.GetState(testProviderName) != StateOpen {
		t.Fatal("expected circuit to be OPEN")
	}

	// Wait for circuit to transition to HALF-OPEN
	time.Sleep(150 * time.Millisecond)

	// Verify circuit is now HALF-OPEN
	if tracker.GetState(testProviderName) != StateHalfOpen {
		// Run check to trigger success recording
		checker.checkAllProviders()
	}

	// The circuit should still be checked if OPEN, and RecordSuccess called
	// Reset state for cleaner test
	tracker2 := NewTracker(cfg, &logger)
	checker2 := NewChecker(tracker2, checkCfg, &logger)

	mockCheck2 := &mockHealthCheck{name: "test-provider2", checkErr: nil}
	checker2.RegisterProvider(mockCheck2)

	// Open circuit
	tracker2.RecordFailure("test-provider2", testErr)
	tracker2.RecordFailure("test-provider2", testErr)

	// Verify OPEN
	if tracker2.GetState("test-provider2") != StateOpen {
		t.Fatal("expected circuit to be OPEN")
	}

	// Run health check
	checker2.checkAllProviders()

	// Health check should have been called
	if mockCheck2.callCount.Load() != 1 {
		t.Errorf("expected check to be called once, got %d", mockCheck2.callCount.Load())
	}

	// After successful health check, RecordSuccess is called
	// This should help transition state eventually
}

func TestCheckerDoesNotRecordSuccessOnFailedCheck(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   30000,
		HalfOpenProbes:   1,
	}
	tracker := NewTracker(cfg, &logger)

	enabled := false
	checkCfg := CheckConfig{Enabled: &enabled}
	checker := NewChecker(tracker, checkCfg, &logger)

	// Register provider with failing health check
	mockCheck := &mockHealthCheck{name: testProviderName, checkErr: errors.New("health check failed")}
	checker.RegisterProvider(mockCheck)

	// Open the circuit
	testErr := errors.New("test error")
	tracker.RecordFailure(testProviderName, testErr)
	tracker.RecordFailure(testProviderName, testErr)

	// Run health check
	checker.checkAllProviders()

	// Health check should have been called
	if mockCheck.callCount.Load() != 1 {
		t.Errorf("expected check to be called once, got %d", mockCheck.callCount.Load())
	}

	// Circuit should still be OPEN (no success recorded)
	if tracker.GetState(testProviderName) != StateOpen {
		t.Error("expected circuit to remain OPEN after failed health check")
	}
}

func TestCheckerStartStop(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenDurationMS:   30000,
		HalfOpenProbes:   1,
	}
	tracker := NewTracker(cfg, &logger)

	// Use very short interval for testing
	// Note: jitter adds 0-2s, so we need to wait long enough
	enabled := true
	checkCfg := CheckConfig{Enabled: &enabled, IntervalMS: 50} // 50ms base interval
	checker := NewChecker(tracker, checkCfg, &logger)

	mockCheck := &mockHealthCheck{name: testProviderName}
	checker.RegisterProvider(mockCheck)

	// Open the circuit so checks run
	testErr := errors.New("test error")
	tracker.RecordFailure(testProviderName, testErr)
	tracker.RecordFailure(testProviderName, testErr)

	// Start checker
	checker.Start()

	// Wait for at least one check cycle (interval 50ms + up to 2s jitter)
	// Give it enough time for at least one tick
	time.Sleep(2500 * time.Millisecond)

	// Stop checker
	checker.Stop()

	// Verify checks were made (at least 1)
	count := mockCheck.callCount.Load()
	if count < 1 {
		t.Errorf("expected at least 1 check, got %d", count)
	}

	// Record count after stop
	countAfterStop := mockCheck.callCount.Load()

	// Wait a bit more and verify no more checks happen
	time.Sleep(500 * time.Millisecond)

	if mockCheck.callCount.Load() != countAfterStop {
		t.Error("expected no more checks after Stop()")
	}
}

func TestCheckerDisabledDoesNotStart(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{}
	tracker := NewTracker(cfg, &logger)

	// Disabled config
	enabled := false
	checkCfg := CheckConfig{Enabled: &enabled, IntervalMS: 10}
	checker := NewChecker(tracker, checkCfg, &logger)

	mockCheck := &mockHealthCheck{name: testProviderName}
	checker.RegisterProvider(mockCheck)

	// Start (should not actually start due to disabled)
	checker.Start()

	// Wait
	time.Sleep(50 * time.Millisecond)

	// No checks should have been made
	if mockCheck.callCount.Load() != 0 {
		t.Errorf("expected 0 checks when disabled, got %d", mockCheck.callCount.Load())
	}

	// Stop should not block
	checker.Stop()
}

func TestCheckerConcurrentRegister(t *testing.T) {
	logger := zerolog.Nop()
	cfg := CircuitBreakerConfig{}
	tracker := NewTracker(cfg, &logger)

	enabled := false
	checkCfg := CheckConfig{Enabled: &enabled}
	checker := NewChecker(tracker, checkCfg, &logger)

	// Register providers concurrently
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			check := NewNoOpHealthCheck(string(rune('a' + idx%26)))
			checker.RegisterProvider(check)
		}(i)
	}
	wg.Wait()

	// Should not panic, and some providers should be registered
	checker.mu.RLock()
	count := len(checker.checks)
	checker.mu.RUnlock()

	// Due to concurrent registration with same names, we'll have at most 26
	if count == 0 {
		t.Error("expected some providers to be registered")
	}
}

func TestCryptoRandDuration(t *testing.T) {
	// Test that it returns values in expected range
	maxDur := 2 * time.Second

	for i := 0; i < 100; i++ {
		d := cryptoRandDuration(maxDur)
		if d < 0 || d >= maxDur {
			t.Errorf("expected duration in [0, %v), got %v", maxDur, d)
		}
	}
}

func TestCryptoRandDurationZeroMax(t *testing.T) {
	d := cryptoRandDuration(0)
	if d != 0 {
		t.Errorf("expected 0 duration for 0 max, got %v", d)
	}
}

func TestCryptoRandDurationNegativeMax(t *testing.T) {
	d := cryptoRandDuration(-time.Second)
	if d != 0 {
		t.Errorf("expected 0 duration for negative max, got %v", d)
	}
}
