package keypool

import (
	"github.com/samber/lo"
)

// WeightedSelector picks an available key using weighted random selection.
type WeightedSelector struct {
}

// NewWeightedSelector creates a new weighted selector.
func NewWeightedSelector() *WeightedSelector {
	return &WeightedSelector{
	}
}

// Select picks a key using weighted random selection.
// Returns ErrAllKeysExhausted if no keys are available.
func (s *WeightedSelector) Select(keys []*KeyMetadata) (*KeyMetadata, error) {
	if len(keys) == 0 {
		return nil, ErrNoKeys
	}

	availableKeys := lo.Filter(keys, func(k *KeyMetadata, _ int) bool {
		return k.IsAvailable()
	})
	if len(availableKeys) == 0 {
		return nil, ErrAllKeysExhausted
	}

	totalWeight := 0
	for _, key := range availableKeys {
		totalWeight += effectiveKeyWeight(key.Weight)
	}
	if totalWeight <= 0 {
		return availableKeys[0], nil
	}

	roll := randIntn(totalWeight)

	for _, key := range availableKeys {
		w := effectiveKeyWeight(key.Weight)
		if roll < w {
			return key, nil
		}
		roll -= w
	}

	return availableKeys[len(availableKeys)-1], nil
}

// Name returns the strategy name.
func (s *WeightedSelector) Name() string {
	return StrategyWeighted
}

func effectiveKeyWeight(weight int) int {
	if weight <= 0 {
		return 1
	}
	return weight
}
