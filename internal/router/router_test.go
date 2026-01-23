package router

import (
	"context"
	"testing"
	"time"
)

func TestStrategyConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "round robin",
			constant: StrategyRoundRobin,
			expected: "round_robin",
		},
		{
			name:     "weighted round robin",
			constant: StrategyWeightedRoundRobin,
			expected: "weighted_round_robin",
		},
		{
			name:     "shuffle",
			constant: StrategyShuffle,
			expected: "shuffle",
		},
		{
			name:     "failover",
			constant: StrategyFailover,
			expected: "failover",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.constant != tt.expected {
				t.Errorf("Strategy constant = %q, want %q", tt.constant, tt.expected)
			}
		})
	}
}

func TestNewRouter_UnknownStrategy(t *testing.T) {
	t.Parallel()

	_, err := NewRouter("unknown_strategy", 5*time.Second)
	if err == nil {
		t.Error("NewRouter() expected error for unknown strategy, got nil")
	}

	// Error should mention "unknown strategy"
	if err.Error() != `router: unknown strategy "unknown_strategy"` {
		t.Errorf("NewRouter() error = %q, want mention of unknown strategy", err.Error())
	}
}

func TestNewRouter_Failover(t *testing.T) {
	t.Parallel()

	router, err := NewRouter(StrategyFailover, 10*time.Second)
	if err != nil {
		t.Fatalf("NewRouter(%q) unexpected error: %v", StrategyFailover, err)
	}
	if router == nil {
		t.Fatal("NewRouter() returned nil router")
	}

	// Verify correct type
	failover, ok := router.(*FailoverRouter)
	if !ok {
		t.Errorf("NewRouter() returned %T, want *FailoverRouter", router)
	}

	// Verify Name()
	if failover.Name() != StrategyFailover {
		t.Errorf("Name() = %q, want %q", failover.Name(), StrategyFailover)
	}

	// Verify timeout is passed correctly
	if failover.Timeout() != 10*time.Second {
		t.Errorf("Timeout() = %v, want %v", failover.Timeout(), 10*time.Second)
	}
}

func TestNewRouter_EmptyDefaultsToFailover(t *testing.T) {
	t.Parallel()

	router, err := NewRouter("", 0)
	if err != nil {
		t.Fatalf("NewRouter(\"\") unexpected error: %v", err)
	}
	if router == nil {
		t.Fatal("NewRouter() returned nil router")
	}

	// Verify correct type
	failover, ok := router.(*FailoverRouter)
	if !ok {
		t.Errorf("NewRouter(\"\") returned %T, want *FailoverRouter", router)
	}

	// Verify Name()
	if failover.Name() != StrategyFailover {
		t.Errorf("Name() = %q, want %q", failover.Name(), StrategyFailover)
	}

	// Verify default timeout (5 seconds when 0 is passed)
	if failover.Timeout() != 5*time.Second {
		t.Errorf("Timeout() = %v, want %v (default)", failover.Timeout(), 5*time.Second)
	}
}

func TestNewRouter_RoundRobin(t *testing.T) {
	t.Parallel()

	router, err := NewRouter(StrategyRoundRobin, 5*time.Second)
	if err != nil {
		t.Fatalf("NewRouter(%q) unexpected error: %v", StrategyRoundRobin, err)
	}
	if router == nil {
		t.Fatal("NewRouter() returned nil router")
	}

	// Verify correct type
	if router.Name() != StrategyRoundRobin {
		t.Errorf("NewRouter() returned router with name %q, want %q", router.Name(), StrategyRoundRobin)
	}

	// Type assertion to verify it's the right implementation
	if _, ok := router.(*RoundRobinRouter); !ok {
		t.Errorf("NewRouter(%q) returned wrong type: got %T", StrategyRoundRobin, router)
	}
}

func TestNewRouter_Shuffle(t *testing.T) {
	t.Parallel()

	router, err := NewRouter(StrategyShuffle, 5*time.Second)
	if err != nil {
		t.Fatalf("NewRouter(%q) unexpected error: %v", StrategyShuffle, err)
	}
	if router == nil {
		t.Fatal("NewRouter() returned nil router")
	}

	// Verify correct type
	if router.Name() != StrategyShuffle {
		t.Errorf("NewRouter() returned router with name %q, want %q", router.Name(), StrategyShuffle)
	}

	// Type assertion to verify it's the right implementation
	if _, ok := router.(*ShuffleRouter); !ok {
		t.Errorf("NewRouter(%q) returned wrong type: got %T", StrategyShuffle, router)
	}
}

func TestNewRouter_WeightedRoundRobin(t *testing.T) {
	t.Parallel()

	router, err := NewRouter(StrategyWeightedRoundRobin, 5*time.Second)
	if err != nil {
		t.Fatalf("NewRouter(%q) unexpected error: %v", StrategyWeightedRoundRobin, err)
	}
	if router == nil {
		t.Fatal("NewRouter() returned nil router")
	}

	// Verify correct type
	wrr, ok := router.(*WeightedRoundRobinRouter)
	if !ok {
		t.Errorf("NewRouter() returned %T, want *WeightedRoundRobinRouter", router)
	}

	// Verify Name()
	if wrr.Name() != StrategyWeightedRoundRobin {
		t.Errorf("Name() = %q, want %q", wrr.Name(), StrategyWeightedRoundRobin)
	}
}

func TestFilterHealthy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		providers      []ProviderInfo
		expectedNames  []string
		healthyResults []bool
		expectedCount  int
	}{
		{
			name:           "all healthy",
			healthyResults: []bool{true, true, true},
			expectedCount:  3,
		},
		{
			name:           "all unhealthy",
			healthyResults: []bool{false, false, false},
			expectedCount:  0,
		},
		{
			name:           "mixed health status",
			healthyResults: []bool{true, false, true, false, true},
			expectedCount:  3,
		},
		{
			name:           "empty slice",
			healthyResults: []bool{},
			expectedCount:  0,
		},
		{
			name:           "single healthy",
			healthyResults: []bool{true},
			expectedCount:  1,
		},
		{
			name:           "single unhealthy",
			healthyResults: []bool{false},
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Build providers with IsHealthy closures
			providers := make([]ProviderInfo, len(tt.healthyResults))
			for i, healthy := range tt.healthyResults {
				h := healthy // Capture for closure
				providers[i] = ProviderInfo{
					IsHealthy: func() bool { return h },
				}
			}

			result := FilterHealthy(providers)
			if len(result) != tt.expectedCount {
				t.Errorf("FilterHealthy() returned %d providers, want %d", len(result), tt.expectedCount)
			}
		})
	}
}

func TestFilterHealthy_NilIsHealthyTreatedAsHealthy(t *testing.T) {
	t.Parallel()

	providers := []ProviderInfo{
		{IsHealthy: nil}, // nil = healthy
		{IsHealthy: func() bool { return true }},
		{IsHealthy: func() bool { return false }},
		{IsHealthy: nil}, // nil = healthy
	}

	result := FilterHealthy(providers)
	if len(result) != 3 {
		t.Errorf("FilterHealthy() returned %d providers, want 3 (nil IsHealthy should be healthy)", len(result))
	}
}

func TestProviderInfo_Healthy(t *testing.T) {
	t.Parallel()

	t.Run("nil IsHealthy returns true", func(t *testing.T) {
		t.Parallel()

		p := ProviderInfo{IsHealthy: nil}
		if !p.Healthy() {
			t.Error("Healthy() should return true when IsHealthy is nil")
		}
	})

	t.Run("IsHealthy returns true", func(t *testing.T) {
		t.Parallel()

		p := ProviderInfo{IsHealthy: func() bool { return true }}
		if !p.Healthy() {
			t.Error("Healthy() should return true when IsHealthy returns true")
		}
	})

	t.Run("IsHealthy returns false", func(t *testing.T) {
		t.Parallel()

		p := ProviderInfo{IsHealthy: func() bool { return false }}
		if p.Healthy() {
			t.Error("Healthy() should return false when IsHealthy returns false")
		}
	})
}

func TestProviderRouterInterface(t *testing.T) {
	t.Parallel()

	// Compile-time interface compliance check
	// When implementations are added, this will verify they implement the interface
	var _ ProviderRouter = (*mockRouter)(nil)
}

// mockRouter is a test implementation of ProviderRouter.
type mockRouter struct {
	name string
}

func (m *mockRouter) Select(_ context.Context, _ []ProviderInfo) (ProviderInfo, error) {
	return ProviderInfo{}, nil
}

func (m *mockRouter) Name() string {
	return m.name
}
