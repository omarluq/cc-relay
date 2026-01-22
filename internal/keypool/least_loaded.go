package keypool

import "github.com/samber/lo"

// LeastLoadedSelector picks the key with the most remaining capacity.
// Capacity is determined by GetCapacityScore() which considers both
// RPM and TPM (input + output tokens) remaining.
type LeastLoadedSelector struct{}

// NewLeastLoadedSelector creates a new least-loaded selector.
func NewLeastLoadedSelector() *LeastLoadedSelector {
	return &LeastLoadedSelector{}
}

// Select picks the key with the highest capacity score.
// Skips unavailable keys (unhealthy or in cooldown).
// Returns ErrAllKeysExhausted if no keys are available.
func (s *LeastLoadedSelector) Select(keys []*KeyMetadata) (*KeyMetadata, error) {
	if len(keys) == 0 {
		return nil, ErrNoKeys
	}

	// Filter to available keys only
	availableKeys := lo.Filter(keys, func(k *KeyMetadata, _ int) bool {
		return k.IsAvailable()
	})

	if len(availableKeys) == 0 {
		return nil, ErrAllKeysExhausted
	}

	// Find key with highest capacity score using MaxBy
	// MaxBy comparison: returns true if 'a' should replace 'b' as max
	bestKey := lo.MaxBy(availableKeys, func(a, b *KeyMetadata) bool {
		return a.GetCapacityScore() > b.GetCapacityScore()
	})

	return bestKey, nil
}

// Name returns the strategy name.
func (s *LeastLoadedSelector) Name() string {
	return StrategyLeastLoaded
}
