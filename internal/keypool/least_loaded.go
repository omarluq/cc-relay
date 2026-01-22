package keypool

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

	var bestKey *KeyMetadata
	var bestScore float64

	for _, key := range keys {
		if !key.IsAvailable() {
			continue // Skip unhealthy/cooldown keys
		}

		score := key.GetCapacityScore()
		if bestKey == nil || score > bestScore {
			bestKey = key
			bestScore = score
		}
	}

	if bestKey == nil {
		return nil, ErrAllKeysExhausted
	}

	return bestKey, nil
}

// Name returns the strategy name.
func (s *LeastLoadedSelector) Name() string {
	return StrategyLeastLoaded
}
