package keypool

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property-based tests for KeySelector implementations

func TestLeastLoadedSelector_Properties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: LeastLoaded always selects highest capacity key
	properties.Property("selects highest capacity key", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			keys := createTestKeysWithVaryingCapacity(keyCount)
			selector := NewLeastLoadedSelector()

			selected, err := selector.Select(keys)
			if err != nil {
				return false // Should always find a key when keys are available
			}

			// Find the actual max capacity key
			maxCapacity := 0.0
			for _, key := range keys {
				if key.IsAvailable() && key.GetCapacityScore() > maxCapacity {
					maxCapacity = key.GetCapacityScore()
				}
			}

			// Selected key should have the highest capacity (with tolerance for float comparison)
			return selected.GetCapacityScore() >= maxCapacity-0.01
		},
		gen.IntRange(1, 20),
	))

	// Property 2: LeastLoaded returns error for empty slice
	properties.Property("returns error for empty keys", prop.ForAll(
		func(_ bool) bool {
			selector := NewLeastLoadedSelector()
			_, err := selector.Select([]*KeyMetadata{})
			return errors.Is(err, ErrNoKeys)
		},
		gen.Bool(),
	))

	// Property 3: LeastLoaded returns error when all keys unavailable
	properties.Property("returns error when all keys unavailable", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			keys := createUnavailableKeys(keyCount)
			selector := NewLeastLoadedSelector()

			_, err := selector.Select(keys)
			return errors.Is(err, ErrAllKeysExhausted)
		},
		gen.IntRange(1, 10),
	))

	// Property 4: LeastLoaded skips unavailable keys
	properties.Property("skips unavailable keys", prop.ForAll(
		func(totalKeys int, unavailableCount int) bool {
			if totalKeys <= 0 || unavailableCount < 0 || unavailableCount >= totalKeys {
				return true
			}

			keys := createMixedAvailabilityKeys(totalKeys, unavailableCount)
			selector := NewLeastLoadedSelector()

			selected, err := selector.Select(keys)
			if err != nil {
				return false
			}

			return selected.IsAvailable()
		},
		gen.IntRange(1, 20),
		gen.IntRange(0, 10),
	))

	// Property 5: Strategy name is correct
	properties.Property("name returns least_loaded", prop.ForAll(
		func(_ bool) bool {
			selector := NewLeastLoadedSelector()
			return selector.Name() == StrategyLeastLoaded
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

func TestRoundRobinSelector_Properties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: RoundRobin distributes selections over all available keys
	properties.Property("distributes across all available keys", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 || keyCount > 10 {
				return true
			}

			keys := createHealthyKeys(keyCount)
			selector := NewRoundRobinSelector()

			// Track which keys are selected
			selections := make(map[string]int)
			iterations := keyCount * 3 // Iterate enough times to cover all keys

			for i := 0; i < iterations; i++ {
				selected, err := selector.Select(keys)
				if err != nil {
					return false
				}
				selections[selected.ID]++
			}

			// All keys should be selected at least once
			return len(selections) == keyCount
		},
		gen.IntRange(1, 10),
	))

	// Property 2: RoundRobin returns error for empty slice
	properties.Property("returns error for empty keys", prop.ForAll(
		func(_ bool) bool {
			selector := NewRoundRobinSelector()
			_, err := selector.Select([]*KeyMetadata{})
			return errors.Is(err, ErrNoKeys)
		},
		gen.Bool(),
	))

	// Property 3: RoundRobin skips unavailable keys
	properties.Property("skips unavailable keys", prop.ForAll(
		func(totalKeys int, unavailableCount int) bool {
			if totalKeys <= 0 || unavailableCount < 0 || unavailableCount >= totalKeys {
				return true
			}

			keys := createMixedAvailabilityKeys(totalKeys, unavailableCount)
			selector := NewRoundRobinSelector()

			selected, err := selector.Select(keys)
			if err != nil {
				return false
			}

			return selected.IsAvailable()
		},
		gen.IntRange(1, 20),
		gen.IntRange(0, 10),
	))

	// Property 4: RoundRobin returns error when all unavailable
	properties.Property("returns error when all unavailable", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			keys := createUnavailableKeys(keyCount)
			selector := NewRoundRobinSelector()

			_, err := selector.Select(keys)
			return errors.Is(err, ErrAllKeysExhausted)
		},
		gen.IntRange(1, 10),
	))

	// Property 5: Strategy name is correct
	properties.Property("name returns round_robin", prop.ForAll(
		func(_ bool) bool {
			selector := NewRoundRobinSelector()
			return selector.Name() == StrategyRoundRobin
		},
		gen.Bool(),
	))

	// Property 6: Selection is deterministic for same index state
	properties.Property("selection advances index", prop.ForAll(
		func(keyCount int, iterations int) bool {
			if keyCount <= 1 || iterations <= 0 || iterations > 100 {
				return true
			}

			keys := createHealthyKeys(keyCount)
			selector := NewRoundRobinSelector()

			// Get first selection
			first, err := selector.Select(keys)
			if err != nil {
				return false
			}

			// Get second selection
			second, err := selector.Select(keys)
			if err != nil {
				return false
			}

			// They should be different (rotating through keys)
			return first.ID != second.ID
		},
		gen.IntRange(2, 10),
		gen.IntRange(1, 50),
	))

	properties.TestingRun(t)
}

func TestNewSelector_Properties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20
	properties := gopter.NewProperties(parameters)

	// Property: Known strategies always succeed
	properties.Property("known strategies create selector", prop.ForAll(
		func(strategyIdx int) bool {
			strategies := []string{StrategyLeastLoaded, StrategyRoundRobin, ""}
			strategy := strategies[strategyIdx%len(strategies)]

			selector, err := NewSelector(strategy)
			return selector != nil && err == nil
		},
		gen.IntRange(0, 2),
	))

	// Property: Unknown strategies return error
	properties.Property("unknown strategies return error", prop.ForAll(
		func(name string) bool {
			// Skip known strategies
			if name == StrategyLeastLoaded || name == StrategyRoundRobin || name == "" {
				return true
			}

			selector, err := NewSelector(name)
			return selector == nil && err != nil
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

// Helper functions for selector property tests

func createTestKeysWithVaryingCapacity(n int) []*KeyMetadata {
	keys := make([]*KeyMetadata, n)
	for i := 0; i < n; i++ {
		// Give each key different remaining capacity
		rpm := 100
		remaining := (i + 1) * 10 // Increasing remaining capacity
		keys[i] = NewKeyMetadata(fmt.Sprintf("sk-test-varied-%d", i), rpm, 10000, 10000)
		keys[i].mu.Lock()
		keys[i].RPMRemaining = remaining
		keys[i].ITPMRemaining = (i + 1) * 1000
		keys[i].OTPMRemaining = (i + 1) * 1000
		keys[i].mu.Unlock()
	}
	return keys
}

func createHealthyKeys(n int) []*KeyMetadata {
	keys := make([]*KeyMetadata, n)
	for i := 0; i < n; i++ {
		keys[i] = NewKeyMetadata(fmt.Sprintf("sk-test-healthy-%d", i), 100, 10000, 10000)
	}
	return keys
}

func createUnavailableKeys(n int) []*KeyMetadata {
	keys := make([]*KeyMetadata, n)
	for i := 0; i < n; i++ {
		keys[i] = NewKeyMetadata(fmt.Sprintf("sk-test-unavail-%d", i), 100, 10000, 10000)
		keys[i].MarkUnhealthy(fmt.Errorf("test error"))
	}
	return keys
}

func createMixedAvailabilityKeys(total, unavailable int) []*KeyMetadata {
	keys := make([]*KeyMetadata, total)
	for i := 0; i < total; i++ {
		keys[i] = NewKeyMetadata(fmt.Sprintf("sk-test-mixed-%d", i), 100, 10000, 10000)
		if i < unavailable {
			keys[i].MarkUnhealthy(fmt.Errorf("test error"))
		}
	}
	return keys
}
