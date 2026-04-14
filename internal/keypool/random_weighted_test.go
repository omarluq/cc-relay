package keypool_test

import (
	"sync"
	"testing"

	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandomSelectorName(t *testing.T) {
	t.Parallel()

	selector := keypool.NewRandomSelector()
	assert.Equal(t, keypool.StrategyRandom, selector.Name())
}

func TestRandomSelectorSelectNoKeys(t *testing.T) {
	t.Parallel()

	selector := keypool.NewRandomSelector()
	_, err := selector.Select([]*keypool.KeyMetadata{})
	assert.ErrorIs(t, err, keypool.ErrNoKeys)
}

func TestRandomSelectorSelectAllExhausted(t *testing.T) {
	t.Parallel()

	selector := keypool.NewRandomSelector()
	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
	}
	keys[0].MarkUnhealthy(assert.AnError)
	keys[1].MarkUnhealthy(assert.AnError)

	_, err := selector.Select(keys)
	assert.ErrorIs(t, err, keypool.ErrAllKeysExhausted)
}

func TestRandomSelectorSelectSingleKey(t *testing.T) {
	t.Parallel()

	selector := keypool.NewRandomSelector()
	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("only", 50, 30000, 30000),
	}

	selected, err := selector.Select(keys)
	require.NoError(t, err)
	assert.Equal(t, "only", selected.APIKey)
}

func TestRandomSelectorSelectMultipleKeys(t *testing.T) {
	t.Parallel()

	selector := keypool.NewRandomSelector()
	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
		keypool.NewKeyMetadata("key3", 50, 30000, 30000),
	}

	counts := map[string]int{"key1": 0, "key2": 0, "key3": 0}
	for range 300 {
		selected, err := selector.Select(keys)
		require.NoError(t, err)
		counts[selected.APIKey]++
	}

	// All keys should be selected at least once with random selection
	for name, count := range counts {
		assert.Greater(t, count, 0, "Key %s was never selected", name)
	}
}

func TestRandomSelectorSkipsUnavailable(t *testing.T) {
	t.Parallel()

	selector := keypool.NewRandomSelector()
	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
		keypool.NewKeyMetadata("key3", 50, 30000, 30000),
	}
	keys[1].MarkUnhealthy(assert.AnError)

	for range 100 {
		selected, err := selector.Select(keys)
		require.NoError(t, err)
		assert.NotEqual(t, "key2", selected.APIKey, "Should not select unhealthy key2")
	}
}

func TestRandomSelectorConcurrent(t *testing.T) {
	t.Parallel()

	selector := keypool.NewRandomSelector()
	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
	}

	var waitGroup sync.WaitGroup
	for range 50 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for range 10 {
				_, err := selector.Select(keys)
				if err != nil {
					t.Errorf("Select() unexpected error: %v", err)
				}
			}
		}()
	}
	waitGroup.Wait()
}

func TestWeightedSelectorName(t *testing.T) {
	t.Parallel()

	selector := keypool.NewWeightedSelector()
	assert.Equal(t, keypool.StrategyWeighted, selector.Name())
}

func TestWeightedSelectorSelectNoKeys(t *testing.T) {
	t.Parallel()

	selector := keypool.NewWeightedSelector()
	_, err := selector.Select([]*keypool.KeyMetadata{})
	assert.ErrorIs(t, err, keypool.ErrNoKeys)
}

func TestWeightedSelectorSelectAllExhausted(t *testing.T) {
	t.Parallel()

	selector := keypool.NewWeightedSelector()
	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
	}
	keys[0].MarkUnhealthy(assert.AnError)
	keys[1].MarkUnhealthy(assert.AnError)

	_, err := selector.Select(keys)
	assert.ErrorIs(t, err, keypool.ErrAllKeysExhausted)
}

func TestWeightedSelectorSelectSingleKey(t *testing.T) {
	t.Parallel()

	selector := keypool.NewWeightedSelector()
	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("only", 50, 30000, 30000),
	}
	keys[0].Weight = 5

	selected, err := selector.Select(keys)
	require.NoError(t, err)
	assert.Equal(t, "only", selected.APIKey)
}

func TestWeightedSelectorProportional(t *testing.T) {
	t.Parallel()

	selector := keypool.NewWeightedSelector()
	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("heavy", 50, 30000, 30000),
		keypool.NewKeyMetadata("light", 50, 30000, 30000),
	}
	keys[0].Weight = 9
	keys[1].Weight = 1

	counts := map[string]int{"heavy": 0, "light": 0}
	iterations := 1000
	for range iterations {
		selected, err := selector.Select(keys)
		require.NoError(t, err)
		counts[selected.APIKey]++
	}

	// "heavy" should get roughly 90% of selections
	heavyPct := float64(counts["heavy"]) / float64(iterations)
	if heavyPct < 0.75 || heavyPct > 0.98 {
		t.Errorf("Heavy key selected %.1f%%, want ~90%%", heavyPct*100)
	}
}

func TestWeightedSelectorDefaultWeight(t *testing.T) {
	t.Parallel()

	selector := keypool.NewWeightedSelector()
	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
	}
	// Both have Weight=0, which means default weight of 1
	keys[0].Weight = 0
	keys[1].Weight = 0

	counts := map[string]int{"key1": 0, "key2": 0}
	for range 200 {
		selected, err := selector.Select(keys)
		require.NoError(t, err)
		counts[selected.APIKey]++
	}

	// With equal weights, both should be selected
	assert.Greater(t, counts["key1"], 0, "key1 was never selected")
	assert.Greater(t, counts["key2"], 0, "key2 was never selected")
}

func TestWeightedSelectorSkipsUnavailable(t *testing.T) {
	t.Parallel()

	selector := keypool.NewWeightedSelector()
	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
	}
	keys[0].Weight = 10
	keys[1].Weight = 10
	keys[0].MarkUnhealthy(assert.AnError)

	for range 50 {
		selected, err := selector.Select(keys)
		require.NoError(t, err)
		assert.Equal(t, "key2", selected.APIKey, "Should only select healthy key2")
	}
}

func TestWeightedSelectorConcurrent(t *testing.T) {
	t.Parallel()

	selector := keypool.NewWeightedSelector()
	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
	}
	keys[0].Weight = 5
	keys[1].Weight = 5

	var waitGroup sync.WaitGroup
	for range 50 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for range 10 {
				_, err := selector.Select(keys)
				if err != nil {
					t.Errorf("Select() unexpected error: %v", err)
				}
			}
		}()
	}
	waitGroup.Wait()
}
