package router

import (
	"context"
	"sync"

	lom "github.com/samber/lo/mutable"
)

// ShuffleRouter distributes requests like dealing cards - everyone gets one
// before anyone gets seconds. Uses Fisher-Yates shuffle for randomization.
//
// The algorithm works by:
// 1. Shuffling all healthy providers into a random order
// 2. Dealing providers one by one in that order
// 3. Reshuffling when the deck is exhausted
//
// This ensures fair distribution: each provider gets exactly one request
// before any provider receives a second request.
type ShuffleRouter struct {
	mu            sync.Mutex
	shuffledOrder []int // indices into provider list
	position      int   // current position in shuffled order
	lastLen       int   // track if provider list changed
}

// NewShuffleRouter creates a new shuffle router.
func NewShuffleRouter() *ShuffleRouter {
	return &ShuffleRouter{}
}

// Select picks the next healthy provider from the shuffled deck.
// Returns ErrNoProviders if providers slice is empty.
// Returns ErrAllProvidersUnhealthy if no providers pass health checks.
//
// Reshuffles when:
// - First call (no prior state)
// - Provider count has changed
// - All providers have been dealt (deck exhausted)
func (r *ShuffleRouter) Select(_ context.Context, providers []ProviderInfo) (ProviderInfo, error) {
	if len(providers) == 0 {
		return ProviderInfo{}, ErrNoProviders
	}

	// Filter to healthy providers only
	healthy := FilterHealthy(providers)
	if len(healthy) == 0 {
		return ProviderInfo{}, ErrAllProvidersUnhealthy
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if we need to reshuffle
	needsReshuffle := len(r.shuffledOrder) == 0 || // first time
		len(healthy) != r.lastLen || // provider count changed
		r.position >= len(r.shuffledOrder) // exhausted

	if needsReshuffle {
		r.reshuffle(len(healthy))
	}

	// Deal the next provider
	idx := r.shuffledOrder[r.position]
	r.position++

	return healthy[idx], nil
}

// reshuffle creates a new shuffled order for n providers.
// Must be called with lock held.
func (r *ShuffleRouter) reshuffle(n int) {
	// Create index slice [0, 1, 2, ..., n-1]
	r.shuffledOrder = make([]int, n)
	for i := 0; i < n; i++ {
		r.shuffledOrder[i] = i
	}

	// Fisher-Yates shuffle using lo/mutable
	lom.Shuffle(r.shuffledOrder)

	// Reset position and update length tracking
	r.position = 0
	r.lastLen = n
}

// Name returns the strategy name for logging and configuration.
func (r *ShuffleRouter) Name() string {
	return StrategyShuffle
}
