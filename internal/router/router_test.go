package router_test

import (
	"context"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/router"
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
			constant: router.StrategyRoundRobin,
			expected: "round_robin",
		},
		{
			name:     "weighted round robin",
			constant: router.StrategyWeightedRoundRobin,
			expected: "weighted_round_robin",
		},
		{
			name:     "shuffle",
			constant: router.StrategyShuffle,
			expected: "shuffle",
		},
		{
			name:     "failover",
			constant: router.StrategyFailover,
			expected: "failover",
		},
		{
			name:     "least loaded",
			constant: router.StrategyLeastLoaded,
			expected: "least_loaded",
		},
		{
			name:     "weighted failover",
			constant: router.StrategyWeightedFailover,
			expected: "weighted_failover",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if testCase.constant != testCase.expected {
				t.Errorf("Strategy constant = %q, want %q", testCase.constant, testCase.expected)
			}
		})
	}
}

func TestNewRouterUnknownStrategy(t *testing.T) {
	t.Parallel()

	_, err := router.NewRouter("unknown_strategy", 5*time.Second)
	if err == nil {
		t.Error("router.NewRouter() expected error for unknown strategy, got nil")
	}

	// Error should mention "unknown strategy"
	if err.Error() != `router: unknown strategy "unknown_strategy"` {
		t.Errorf("router.NewRouter() error = %q, want mention of unknown strategy", err.Error())
	}
}

func TestNewRouterFailover(t *testing.T) {
	t.Parallel()

	rtr, err := router.NewRouter(router.StrategyFailover, 10*time.Second)
	if err != nil {
		t.Fatalf("router.NewRouter(%q) unexpected error: %v", router.StrategyFailover, err)
	}
	if rtr == nil {
		t.Fatal("router.NewRouter() returned nil router")
	}

	// Verify correct type
	failover, ok := rtr.(*router.FailoverRouter)
	if !ok {
		t.Errorf("router.NewRouter() returned %T, want *router.FailoverRouter", rtr)
	}

	// Verify Name()
	if failover.Name() != router.StrategyFailover {
		t.Errorf("Name() = %q, want %q", failover.Name(), router.StrategyFailover)
	}

	// Verify timeout is passed correctly
	if failover.Timeout() != 10*time.Second {
		t.Errorf("Timeout() = %v, want %v", failover.Timeout(), 10*time.Second)
	}
}

func TestNewRouterLeastLoaded(t *testing.T) {
	t.Parallel()

	rtr, err := router.NewRouter(router.StrategyLeastLoaded, 0)
	if err != nil {
		t.Fatalf("router.NewRouter(%q) unexpected error: %v", router.StrategyLeastLoaded, err)
	}

	if _, ok := rtr.(*router.LeastLoadedRouter); !ok {
		t.Errorf("router.NewRouter() returned %T, want *router.LeastLoadedRouter", rtr)
	}

	if rtr.Name() != router.StrategyLeastLoaded {
		t.Errorf("Name() = %q, want %q", rtr.Name(), router.StrategyLeastLoaded)
	}
}

func TestNewRouterWeightedFailover(t *testing.T) {
	t.Parallel()

	rtr, err := router.NewRouter(router.StrategyWeightedFailover, 0)
	if err != nil {
		t.Fatalf("router.NewRouter(%q) unexpected error: %v", router.StrategyWeightedFailover, err)
	}

	if _, ok := rtr.(*router.WeightedFailoverRouter); !ok {
		t.Errorf("router.NewRouter() returned %T, want *router.WeightedFailoverRouter", rtr)
	}

	if rtr.Name() != router.StrategyWeightedFailover {
		t.Errorf("Name() = %q, want %q", rtr.Name(), router.StrategyWeightedFailover)
	}
}

func TestNewRouterEmptyDefaultsToFailover(t *testing.T) {
	t.Parallel()

	rtr, err := router.NewRouter("", 0)
	if err != nil {
		t.Fatalf("router.NewRouter(\"\") unexpected error: %v", err)
	}
	if rtr == nil {
		t.Fatal("router.NewRouter() returned nil router")
	}

	// Verify correct type
	failover, ok := rtr.(*router.FailoverRouter)
	if !ok {
		t.Errorf("router.NewRouter(\"\") returned %T, want *router.FailoverRouter", rtr)
	}

	// Verify Name()
	if failover.Name() != router.StrategyFailover {
		t.Errorf("Name() = %q, want %q", failover.Name(), router.StrategyFailover)
	}

	// Verify default timeout (5 seconds when 0 is passed)
	if failover.Timeout() != 5*time.Second {
		t.Errorf("Timeout() = %v, want %v (default)", failover.Timeout(), 5*time.Second)
	}
}

// TestNewRouterStrategies tests that NewRouter returns the correct router type
// for simple strategies (round_robin, shuffle, weighted_round_robin).
func TestNewRouterStrategies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		strategy     string
		wantName     string
		wantTypeName string
	}{
		{
			name:         "round robin",
			strategy:     router.StrategyRoundRobin,
			wantName:     router.StrategyRoundRobin,
			wantTypeName: "*router.RoundRobinRouter",
		},
		{
			name:         "shuffle",
			strategy:     router.StrategyShuffle,
			wantName:     router.StrategyShuffle,
			wantTypeName: "*router.ShuffleRouter",
		},
		{
			name:         "weighted round robin",
			strategy:     router.StrategyWeightedRoundRobin,
			wantName:     router.StrategyWeightedRoundRobin,
			wantTypeName: "*router.WeightedRoundRobinRouter",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			rtr, err := router.NewRouter(testCase.strategy, 5*time.Second)
			if err != nil {
				t.Fatalf("router.NewRouter(%q) unexpected error: %v", testCase.strategy, err)
			}
			if rtr == nil {
				t.Fatal("router.NewRouter() returned nil router")
			}

			if rtr.Name() != testCase.wantName {
				t.Errorf("Name() = %q, want %q", rtr.Name(), testCase.wantName)
			}
		})
	}
}

func TestFilterHealthy(t *testing.T) {
	t.Parallel()

	type filterHealthyTestCase struct {
		name           string
		providers      []router.ProviderInfo
		expectedNames  []string
		healthyResults []bool
		expectedCount  int
	}

	tests := []filterHealthyTestCase{
		{
			name:           "all healthy",
			providers:      nil,
			expectedNames:  nil,
			healthyResults: []bool{true, true, true},
			expectedCount:  3,
		},
		{
			name:           "all unhealthy",
			providers:      nil,
			expectedNames:  nil,
			healthyResults: []bool{false, false, false},
			expectedCount:  0,
		},
		{
			name:           "mixed health status",
			providers:      nil,
			expectedNames:  nil,
			healthyResults: []bool{true, false, true, false, true},
			expectedCount:  3,
		},
		{
			name:           "empty slice",
			providers:      nil,
			expectedNames:  nil,
			healthyResults: []bool{},
			expectedCount:  0,
		},
		{
			name:           "single healthy",
			providers:      nil,
			expectedNames:  nil,
			healthyResults: []bool{true},
			expectedCount:  1,
		},
		{
			name:           "single unhealthy",
			providers:      nil,
			expectedNames:  nil,
			healthyResults: []bool{false},
			expectedCount:  0,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Build providers with IsHealthy closures
			providers := make([]router.ProviderInfo, len(testCase.healthyResults))
			for idx, healthy := range testCase.healthyResults {
				isHealthy := healthy // Capture for closure
				providers[idx] = router.ProviderInfo{
					Provider:  router.NewTestProvider(string(rune('a' + idx))),
					Weight:    0,
					Priority:  0,
					IsHealthy: func() bool { return isHealthy },
				}
			}

			result := router.FilterHealthy(providers)
			if len(result) != testCase.expectedCount {
				t.Errorf("router.FilterHealthy() returned %d providers, want %d",
					len(result), testCase.expectedCount)
			}
		})
	}
}

func TestFilterHealthyNilIsHealthyTreatedAsHealthy(t *testing.T) {
	t.Parallel()

	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Weight: 0, Priority: 0, IsHealthy: nil},
		{Provider: router.NewTestProvider("p2"), Weight: 0, Priority: 0, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p3"), Weight: 0, Priority: 0, IsHealthy: func() bool { return false }},
		{Provider: router.NewTestProvider("p4"), Weight: 0, Priority: 0, IsHealthy: nil},
	}

	result := router.FilterHealthy(providers)
	if len(result) != 3 {
		t.Errorf("router.FilterHealthy() returned %d providers, want 3 (nil IsHealthy should be healthy)",
			len(result))
	}
}

func TestProviderInfoHealthy(t *testing.T) {
	t.Parallel()

	t.Run("nil IsHealthy returns true", func(t *testing.T) {
		t.Parallel()

		prov := router.ProviderInfo{
			Provider: router.NewTestProvider("test"), Weight: 0, Priority: 0, IsHealthy: nil,
		}
		if !prov.Healthy() {
			t.Error("Healthy() should return true when IsHealthy is nil")
		}
	})

	t.Run("IsHealthy returns true", func(t *testing.T) {
		t.Parallel()

		prov := router.ProviderInfo{
			Provider:  router.NewTestProvider("test"),
			Weight:    0,
			Priority:  0,
			IsHealthy: func() bool { return true },
		}
		if !prov.Healthy() {
			t.Error("Healthy() should return true when IsHealthy returns true")
		}
	})

	t.Run("IsHealthy returns false", func(t *testing.T) {
		t.Parallel()

		prov := router.ProviderInfo{
			Provider:  router.NewTestProvider("test"),
			Weight:    0,
			Priority:  0,
			IsHealthy: func() bool { return false },
		}
		if prov.Healthy() {
			t.Error("Healthy() should return false when IsHealthy returns false")
		}
	})
}

func TestProviderRouterInterface(t *testing.T) {
	t.Parallel()

	// Compile-time interface compliance check
	var _ router.ProviderRouter = (*mockRouter)(nil)
}

// mockRouter is a test implementation of ProviderRouter.
type mockRouter struct {
	name string
}

func (m *mockRouter) Select(_ context.Context, _ []router.ProviderInfo) (router.ProviderInfo, error) {
	return router.ProviderInfo{
		Provider:  router.NewTestProvider("mock"),
		Weight:    0,
		Priority:  0,
		IsHealthy: nil,
	}, nil
}

func (m *mockRouter) Name() string {
	return m.name
}
