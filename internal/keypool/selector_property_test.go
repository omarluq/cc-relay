package keypool_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/omarluq/cc-relay/internal/keypool"
)

// Property-based tests for KeySelector implementations - split to reduce cognitive complexity.

func TestLeastLoadedSelectsHighestCapacity(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("selects highest capacity key", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			keys := createTestKeysWithVaryingCapacity(keyCount)
			selector := keypool.NewLeastLoadedSelector()

			selected, err := selector.Select(keys)
			if err != nil {
				return false
			}

			maxCapacity := 0.0
			for _, key := range keys {
				if key.IsAvailable() && key.GetCapacityScore() > maxCapacity {
					maxCapacity = key.GetCapacityScore()
				}
			}

			return selected.GetCapacityScore() >= maxCapacity-0.01
		},
		gen.IntRange(1, 20),
	))

	properties.TestingRun(t)
}

func TestLeastLoadedErrorProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("returns error for empty keys", prop.ForAll(
		func(_ bool) bool {
			selector := keypool.NewLeastLoadedSelector()
			_, err := selector.Select([]*keypool.KeyMetadata{})
			return errors.Is(err, keypool.ErrNoKeys)
		},
		gen.Bool(),
	))

	properties.Property("returns error when all keys unavailable", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			keys := createUnavailableKeys(keyCount)
			selector := keypool.NewLeastLoadedSelector()

			_, err := selector.Select(keys)
			return errors.Is(err, keypool.ErrAllKeysExhausted)
		},
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

func TestLeastLoadedAvailabilityProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("skips unavailable keys", prop.ForAll(
		func(totalKeys int, unavailableCount int) bool {
			if totalKeys <= 0 || unavailableCount < 0 || unavailableCount >= totalKeys {
				return true
			}

			keys := createMixedAvailabilityKeys(totalKeys, unavailableCount)
			selector := keypool.NewLeastLoadedSelector()

			selected, err := selector.Select(keys)
			if err != nil {
				return false
			}

			return selected.IsAvailable()
		},
		gen.IntRange(1, 20),
		gen.IntRange(0, 10),
	))

	properties.Property("name returns least_loaded", prop.ForAll(
		func(_ bool) bool {
			selector := keypool.NewLeastLoadedSelector()
			return selector.Name() == keypool.StrategyLeastLoaded
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

func TestRoundRobinDistributionProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("distributes across all available keys", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 || keyCount > 10 {
				return true
			}

			keys := createHealthyKeys(keyCount)
			selector := keypool.NewRoundRobinSelector()

			selections := make(map[string]int)
			iterations := keyCount * 3

			for iteration := 0; iteration < iterations; iteration++ {
				selected, err := selector.Select(keys)
				if err != nil {
					return false
				}
				selections[selected.ID]++
			}

			return len(selections) == keyCount
		},
		gen.IntRange(1, 10),
	))

	properties.Property("selection advances index", prop.ForAll(
		func(keyCount int, iterations int) bool {
			if keyCount <= 1 || iterations <= 0 || iterations > 100 {
				return true
			}

			keys := createHealthyKeys(keyCount)
			selector := keypool.NewRoundRobinSelector()

			first, err := selector.Select(keys)
			if err != nil {
				return false
			}

			second, err := selector.Select(keys)
			if err != nil {
				return false
			}

			return first.ID != second.ID
		},
		gen.IntRange(2, 10),
		gen.IntRange(1, 50),
	))

	properties.TestingRun(t)
}

func TestRoundRobinErrorProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("returns error for empty keys", prop.ForAll(
		func(_ bool) bool {
			selector := keypool.NewRoundRobinSelector()
			_, err := selector.Select([]*keypool.KeyMetadata{})
			return errors.Is(err, keypool.ErrNoKeys)
		},
		gen.Bool(),
	))

	properties.Property("skips unavailable keys", prop.ForAll(
		func(totalKeys int, unavailableCount int) bool {
			if totalKeys <= 0 || unavailableCount < 0 || unavailableCount >= totalKeys {
				return true
			}

			keys := createMixedAvailabilityKeys(totalKeys, unavailableCount)
			selector := keypool.NewRoundRobinSelector()

			selected, err := selector.Select(keys)
			if err != nil {
				return false
			}

			return selected.IsAvailable()
		},
		gen.IntRange(1, 20),
		gen.IntRange(0, 10),
	))

	properties.Property("returns error when all unavailable", prop.ForAll(
		func(keyCount int) bool {
			if keyCount <= 0 {
				return true
			}

			keys := createUnavailableKeys(keyCount)
			selector := keypool.NewRoundRobinSelector()

			_, err := selector.Select(keys)
			return errors.Is(err, keypool.ErrAllKeysExhausted)
		},
		gen.IntRange(1, 10),
	))

	properties.Property("name returns round_robin", prop.ForAll(
		func(_ bool) bool {
			selector := keypool.NewRoundRobinSelector()
			return selector.Name() == keypool.StrategyRoundRobin
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

func TestNewSelectorProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20
	properties := gopter.NewProperties(parameters)

	properties.Property("known strategies create selector", prop.ForAll(
		func(strategyIdx int) bool {
			strategies := []string{keypool.StrategyLeastLoaded, keypool.StrategyRoundRobin, ""}
			strategy := strategies[strategyIdx%len(strategies)]

			selector, err := keypool.NewSelector(strategy)
			return selector != nil && err == nil
		},
		gen.IntRange(0, 2),
	))

	properties.Property("unknown strategies return error", prop.ForAll(
		func(name string) bool {
			if name == keypool.StrategyLeastLoaded || name == keypool.StrategyRoundRobin || name == "" {
				return true
			}

			selector, err := keypool.NewSelector(name)
			return selector == nil && err != nil
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

// Helper functions for selector property tests

func createTestKeysWithVaryingCapacity(numKeys int) []*keypool.KeyMetadata {
	keys := make([]*keypool.KeyMetadata, numKeys)
	for keyIdx := 0; keyIdx < numKeys; keyIdx++ {
		rpm := 100
		remaining := (keyIdx + 1) * 10
		keys[keyIdx] = keypool.NewKeyMetadata(fmt.Sprintf("sk-test-varied-%d", keyIdx), rpm, 10000, 10000)
		keys[keyIdx].TestLock()
		keys[keyIdx].RPMRemaining = remaining
		keys[keyIdx].ITPMRemaining = (keyIdx + 1) * 1000
		keys[keyIdx].OTPMRemaining = (keyIdx + 1) * 1000
		keys[keyIdx].TestUnlock()
	}
	return keys
}

func createHealthyKeys(numKeys int) []*keypool.KeyMetadata {
	keys := make([]*keypool.KeyMetadata, numKeys)
	for keyIdx := 0; keyIdx < numKeys; keyIdx++ {
		keys[keyIdx] = keypool.NewKeyMetadata(fmt.Sprintf("sk-test-healthy-%d", keyIdx), 100, 10000, 10000)
	}
	return keys
}

func createUnavailableKeys(numKeys int) []*keypool.KeyMetadata {
	keys := make([]*keypool.KeyMetadata, numKeys)
	for keyIdx := 0; keyIdx < numKeys; keyIdx++ {
		keys[keyIdx] = keypool.NewKeyMetadata(fmt.Sprintf("sk-test-unavail-%d", keyIdx), 100, 10000, 10000)
		keys[keyIdx].MarkUnhealthy(fmt.Errorf("test error"))
	}
	return keys
}

func createMixedAvailabilityKeys(total, unavailable int) []*keypool.KeyMetadata {
	keys := make([]*keypool.KeyMetadata, total)
	for keyIdx := 0; keyIdx < total; keyIdx++ {
		keys[keyIdx] = keypool.NewKeyMetadata(fmt.Sprintf("sk-test-mixed-%d", keyIdx), 100, 10000, 10000)
		if keyIdx < unavailable {
			keys[keyIdx].MarkUnhealthy(fmt.Errorf("test error"))
		}
	}
	return keys
}
