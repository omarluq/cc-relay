package keypool

import (
	"github.com/samber/lo"
)

// RandomSelector picks a random available key.
type RandomSelector struct {
}

// NewRandomSelector creates a new random selector.
func NewRandomSelector() *RandomSelector {
	return &RandomSelector{
	}
}

// Select picks a random available key.
// Returns ErrAllKeysExhausted if no keys are available.
func (s *RandomSelector) Select(keys []*KeyMetadata) (*KeyMetadata, error) {
	if len(keys) == 0 {
		return nil, ErrNoKeys
	}

	availableKeys := lo.Filter(keys, func(k *KeyMetadata, _ int) bool {
		return k.IsAvailable()
	})
	if len(availableKeys) == 0 {
		return nil, ErrAllKeysExhausted
	}

	idx := randIntn(len(availableKeys))

	return availableKeys[idx], nil
}

// Name returns the strategy name.
func (s *RandomSelector) Name() string {
	return StrategyRandom
}
