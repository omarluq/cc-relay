package health

import (
	"testing"
	"time"
)

func TestCircuitBreakerConfigGetFailureThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   CircuitBreakerConfig
		expected uint32
	}{
		{
			name:     "zero value returns default 5",
			config:   CircuitBreakerConfig{},
			expected: 5,
		},
		{
			name:     "custom value 10 returns 10",
			config:   CircuitBreakerConfig{FailureThreshold: 10},
			expected: 10,
		},
		{
			name:     "custom value 1 returns 1",
			config:   CircuitBreakerConfig{FailureThreshold: 1},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.config.GetFailureThreshold()
			if got != tt.expected {
				t.Errorf("GetFailureThreshold() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCircuitBreakerConfigGetOpenDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   CircuitBreakerConfig
		expected time.Duration
	}{
		{
			name:     "zero value returns default 30s",
			config:   CircuitBreakerConfig{},
			expected: 30 * time.Second,
		},
		{
			name:     "custom value 60000ms returns 60s",
			config:   CircuitBreakerConfig{OpenDurationMS: 60000},
			expected: 60 * time.Second,
		},
		{
			name:     "custom value 5000ms returns 5s",
			config:   CircuitBreakerConfig{OpenDurationMS: 5000},
			expected: 5 * time.Second,
		},
		{
			name:     "negative value returns default 30s",
			config:   CircuitBreakerConfig{OpenDurationMS: -100},
			expected: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.config.GetOpenDuration()
			if got != tt.expected {
				t.Errorf("GetOpenDuration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCircuitBreakerConfigGetHalfOpenProbes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   CircuitBreakerConfig
		expected uint32
	}{
		{
			name:     "zero value returns default 3",
			config:   CircuitBreakerConfig{},
			expected: 3,
		},
		{
			name:     "custom value 5 returns 5",
			config:   CircuitBreakerConfig{HalfOpenProbes: 5},
			expected: 5,
		},
		{
			name:     "custom value 1 returns 1",
			config:   CircuitBreakerConfig{HalfOpenProbes: 1},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.config.GetHalfOpenProbes()
			if got != tt.expected {
				t.Errorf("GetHalfOpenProbes() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCheckConfigGetInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   CheckConfig
		expected time.Duration
	}{
		{
			name:     "zero value returns default 10s",
			config:   CheckConfig{},
			expected: 10 * time.Second,
		},
		{
			name:     "custom value 5000ms returns 5s",
			config:   CheckConfig{IntervalMS: 5000},
			expected: 5 * time.Second,
		},
		{
			name:     "custom value 30000ms returns 30s",
			config:   CheckConfig{IntervalMS: 30000},
			expected: 30 * time.Second,
		},
		{
			name:     "negative value returns default 10s",
			config:   CheckConfig{IntervalMS: -500},
			expected: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.config.GetInterval()
			if got != tt.expected {
				t.Errorf("GetInterval() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCheckConfigIsEnabled(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name     string
		config   CheckConfig
		expected bool
	}{
		{
			name:     "default (nil) returns true",
			config:   CheckConfig{},
			expected: true,
		},
		{
			name:     "explicit true returns true",
			config:   CheckConfig{Enabled: boolPtr(true)},
			expected: true,
		},
		{
			name:     "explicit false returns false",
			config:   CheckConfig{Enabled: boolPtr(false)},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.config.IsEnabled()
			if got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfigStructComposition(t *testing.T) {
	t.Parallel()

	// Test that Config properly composes CircuitBreakerConfig and CheckConfig
	cfg := Config{
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold: 10,
			OpenDurationMS:   60000,
			HalfOpenProbes:   5,
		},
		HealthCheck: CheckConfig{
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
		{got: DefaultFailureThreshold, expected: 5, name: "DefaultFailureThreshold"},
		{got: DefaultOpenDurationMS, expected: 30000, name: "DefaultOpenDurationMS"},
		{got: DefaultHalfOpenProbes, expected: 3, name: "DefaultHalfOpenProbes"},
		{got: DefaultHealthCheckMS, expected: 10000, name: "DefaultHealthCheckMS"},
		{got: DefaultHealthEnabled, expected: true, name: "DefaultHealthEnabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}
