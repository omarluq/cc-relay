package keypool

import (
	"errors"
	"fmt"
)

// KeySelector defines the interface for key selection strategies.
// Implementations choose which key to use from the available pool.
type KeySelector interface {
	// Select chooses a key from the pool based on the strategy.
	// Returns ErrAllKeysExhausted if no keys are available.
	Select(keys []*KeyMetadata) (*KeyMetadata, error)

	// Name returns the strategy name for logging and configuration.
	Name() string
}

// Common errors returned by key selectors.
var (
	ErrAllKeysExhausted = errors.New("keypool: all keys exhausted")
	ErrNoKeys           = errors.New("keypool: no keys configured")
)

// Strategy constants for configuration.
const (
	StrategyLeastLoaded = "least_loaded"
	StrategyRoundRobin  = "round_robin"
	StrategyRandom      = "random"
	StrategyWeighted    = "weighted"
)

// NewSelector creates a KeySelector based on the strategy name.
// Returns an error if the strategy is unknown.
// Empty string defaults to least_loaded.
func NewSelector(strategy string) (KeySelector, error) {
	switch strategy {
	case StrategyLeastLoaded, "":
		return NewLeastLoadedSelector(), nil
	case StrategyRoundRobin:
		return NewRoundRobinSelector(), nil
	default:
		return nil, fmt.Errorf("keypool: unknown strategy %q", strategy)
	}
}
