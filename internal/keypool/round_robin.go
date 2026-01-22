package keypool

import "sync/atomic"

// RoundRobinSelector cycles through keys in order.
// Uses atomic counter for thread-safe operation without mutex overhead.
type RoundRobinSelector struct {
	index uint64 // Atomic counter for current position
}

// NewRoundRobinSelector creates a new round-robin selector.
func NewRoundRobinSelector() *RoundRobinSelector {
	return &RoundRobinSelector{}
}

// Select picks the next available key in round-robin order.
// Skips unavailable keys (unhealthy or in cooldown).
// Returns ErrAllKeysExhausted if no keys are available after a full loop.
func (s *RoundRobinSelector) Select(keys []*KeyMetadata) (*KeyMetadata, error) {
	if len(keys) == 0 {
		return nil, ErrNoKeys
	}

	// Get next index atomically
	nextIndex := atomic.AddUint64(&s.index, 1) - 1
	keysLen := uint64(len(keys))
	//nolint:gosec // Safe: modulo ensures result is within int range (< len(keys))
	startIdx := int(nextIndex % keysLen)

	// Try each key starting from startIdx, wrapping around
	for i := 0; i < len(keys); i++ {
		idx := (startIdx + i) % len(keys)
		key := keys[idx]

		if key.IsAvailable() {
			return key, nil
		}
	}

	// Full loop with no available keys
	return nil, ErrAllKeysExhausted
}

// Name returns the strategy name.
func (s *RoundRobinSelector) Name() string {
	return StrategyRoundRobin
}
