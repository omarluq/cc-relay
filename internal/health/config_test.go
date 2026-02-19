package health_test

import (
	"github.com/omarluq/cc-relay/internal/health"
	"testing"
	"time"
)

func TestCircuitBreakerConfigUint32Getters(t *testing.T) {
	t.Parallel()

	type uint32GetterTestCase struct {
		getter     func(health.CircuitBreakerConfig) uint32
		name       string
		getterName string
		config     health.CircuitBreakerConfig
		expected   uint32
	}

	getFailureThreshold := func(cfg health.CircuitBreakerConfig) uint32 {
		return cfg.GetFailureThreshold()
	}
	getHalfOpenProbes := func(cfg health.CircuitBreakerConfig) uint32 {
		return cfg.GetHalfOpenProbes()
	}

	tests := []uint32GetterTestCase{
		// FailureThreshold tests
		{
			getter:     getFailureThreshold,
			name:       "FailureThreshold zero value returns default 5",
			getterName: "GetFailureThreshold",
			config:     health.CircuitBreakerConfig{OpenDurationMS: 0, FailureThreshold: 0, HalfOpenProbes: 0},
			expected:   5,
		},
		{
			getter:     getFailureThreshold,
			name:       "FailureThreshold custom value 10",
			getterName: "GetFailureThreshold",
			config:     health.CircuitBreakerConfig{OpenDurationMS: 0, FailureThreshold: 10, HalfOpenProbes: 0},
			expected:   10,
		},
		{
			getter:     getFailureThreshold,
			name:       "FailureThreshold custom value 1",
			getterName: "GetFailureThreshold",
			config:     health.CircuitBreakerConfig{OpenDurationMS: 0, FailureThreshold: 1, HalfOpenProbes: 0},
			expected:   1,
		},
		// HalfOpenProbes tests
		{
			getter:     getHalfOpenProbes,
			name:       "HalfOpenProbes zero value returns default 3",
			getterName: "GetHalfOpenProbes",
			config:     health.CircuitBreakerConfig{OpenDurationMS: 0, FailureThreshold: 0, HalfOpenProbes: 0},
			expected:   3,
		},
		{
			getter:     getHalfOpenProbes,
			name:       "HalfOpenProbes custom value 5",
			getterName: "GetHalfOpenProbes",
			config:     health.CircuitBreakerConfig{OpenDurationMS: 0, FailureThreshold: 0, HalfOpenProbes: 5},
			expected:   5,
		},
		{
			getter:     getHalfOpenProbes,
			name:       "HalfOpenProbes custom value 1",
			getterName: "GetHalfOpenProbes",
			config:     health.CircuitBreakerConfig{OpenDurationMS: 0, FailureThreshold: 0, HalfOpenProbes: 1},
			expected:   1,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := testCase.getter(testCase.config)
			if got != testCase.expected {
				t.Errorf("%s() = %v, want %v", testCase.getterName, got, testCase.expected)
			}
		})
	}
}

func TestCircuitBreakerConfigGetOpenDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   health.CircuitBreakerConfig
		expected time.Duration
	}{
		{
			name: "zero value returns default 30s",
			config: health.CircuitBreakerConfig{
				OpenDurationMS: 0, FailureThreshold: 0, HalfOpenProbes: 0,
			},
			expected: 30 * time.Second,
		},
		{
			name: "custom value 60000ms returns 60s",
			config: health.CircuitBreakerConfig{
				OpenDurationMS: 60000, FailureThreshold: 0, HalfOpenProbes: 0,
			},
			expected: 60 * time.Second,
		},
		{
			name: "custom value 5000ms returns 5s",
			config: health.CircuitBreakerConfig{
				OpenDurationMS: 5000, FailureThreshold: 0, HalfOpenProbes: 0,
			},
			expected: 5 * time.Second,
		},
		{
			name: "negative value returns default 30s",
			config: health.CircuitBreakerConfig{
				OpenDurationMS: -100, FailureThreshold: 0, HalfOpenProbes: 0,
			},
			expected: 30 * time.Second,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := testCase.config.GetOpenDuration()
			if got != testCase.expected {
				t.Errorf("GetOpenDuration() = %v, want %v", got, testCase.expected)
			}
		})
	}
}

func TestCheckConfigGetInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   health.CheckConfig
		expected time.Duration
	}{
		{
			name:     "zero value returns default 10s",
			config:   health.CheckConfig{Enabled: nil, IntervalMS: 0},
			expected: 10 * time.Second,
		},
		{
			name:     "custom value 5000ms returns 5s",
			config:   health.CheckConfig{Enabled: nil, IntervalMS: 5000},
			expected: 5 * time.Second,
		},
		{
			name:     "custom value 30000ms returns 30s",
			config:   health.CheckConfig{Enabled: nil, IntervalMS: 30000},
			expected: 30 * time.Second,
		},
		{
			name:     "negative value returns default 10s",
			config:   health.CheckConfig{Enabled: nil, IntervalMS: -500},
			expected: 10 * time.Second,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := testCase.config.GetInterval()
			if got != testCase.expected {
				t.Errorf("GetInterval() = %v, want %v", got, testCase.expected)
			}
		})
	}
}

func TestCheckConfigIsEnabled(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name     string
		config   health.CheckConfig
		expected bool
	}{
		{
			name:     "default (nil) returns true",
			config:   health.CheckConfig{Enabled: nil, IntervalMS: 0},
			expected: true,
		},
		{
			name:     "explicit true returns true",
			config:   health.CheckConfig{Enabled: boolPtr(true), IntervalMS: 0},
			expected: true,
		},
		{
			name:     "explicit false returns false",
			config:   health.CheckConfig{Enabled: boolPtr(false), IntervalMS: 0},
			expected: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := testCase.config.IsEnabled()
			if got != testCase.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, testCase.expected)
			}
		})
	}
}

func TestConfigStructComposition(t *testing.T) {
	t.Parallel()

	// Test that Config properly composes CircuitBreakerConfig and CheckConfig
	cfg := health.Config{
		CircuitBreaker: health.CircuitBreakerConfig{
			FailureThreshold: 10,
			OpenDurationMS:   60000,
			HalfOpenProbes:   5,
		},
		HealthCheck: health.CheckConfig{
			Enabled:    nil,
			IntervalMS: 15000,
		},
	}

	// Verify nested configs work correctly
	if got := cfg.CircuitBreaker.GetFailureThreshold(); got != 10 {
		t.Errorf("CircuitBreaker.GetFailureThreshold() = %v, want 10", got)
	}
	if got := cfg.CircuitBreaker.GetOpenDuration(); got != 60*time.Second {
		t.Errorf("CircuitBreaker.GetOpenDuration() = %v, want 60s", got)
	}
	if got := cfg.CircuitBreaker.GetHalfOpenProbes(); got != 5 {
		t.Errorf("CircuitBreaker.GetHalfOpenProbes() = %v, want 5", got)
	}
	if got := cfg.HealthCheck.GetInterval(); got != 15*time.Second {
		t.Errorf("HealthCheck.GetInterval() = %v, want 15s", got)
	}
}

func TestDefaults(t *testing.T) {
	t.Parallel()

	// Verify default constants are set correctly
	tests := []struct {
		got      any
		expected any
		name     string
	}{
		{got: health.DefaultFailureThreshold, expected: 5, name: "health.DefaultFailureThreshold"},
		{got: health.DefaultOpenDurationMS, expected: 30000, name: "health.DefaultOpenDurationMS"},
		{got: health.DefaultHalfOpenProbes, expected: 3, name: "health.DefaultHalfOpenProbes"},
		{got: health.DefaultHealthCheckMS, expected: 10000, name: "health.DefaultHealthCheckMS"},
		{got: health.DefaultHealthEnabled, expected: true, name: "health.DefaultHealthEnabled"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if testCase.got != testCase.expected {
				t.Errorf("%s = %v, want %v", testCase.name, testCase.got, testCase.expected)
			}
		})
	}
}
