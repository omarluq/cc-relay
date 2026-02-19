package router

import (
	"context"
	"sync"

	"github.com/samber/lo"
)

// WeightedRoundRobinRouter distributes requests proportionally to provider weights.
// Uses the Nginx smooth weighted round-robin algorithm for even distribution.
//
// The algorithm works by:
// 1. Adding each provider's weight to its current weight
// 2. Selecting the provider with the highest current weight
// 3. Subtracting the total weight from the selected provider's current weight
//
// This ensures smooth distribution: a provider with weight 3 among providers
// totaling weight 6 will be selected 50% of the time, but selections are
// spread evenly rather than clustered.
type WeightedRoundRobinRouter struct {
	currentWeights  []int
	lastProviderIDs []string
	mu              sync.Mutex
}

// NewWeightedRoundRobinRouter creates a new weighted round-robin router.
func NewWeightedRoundRobinRouter() *WeightedRoundRobinRouter {
	return &WeightedRoundRobinRouter{
		currentWeights:  nil,
		lastProviderIDs: nil,
		mu:              sync.Mutex{},
	}
}

// Select chooses a provider using the Nginx smooth weighted round-robin algorithm.
// Providers with higher weights receive proportionally more traffic.
// Default weight is 1 when Weight <= 0.
func (r *WeightedRoundRobinRouter) Select(_ context.Context, providers []ProviderInfo) (ProviderInfo, error) {
	if len(providers) == 0 {
		return ProviderInfo{}, ErrNoProviders
	}

	healthy := FilterHealthy(providers)
	if len(healthy) == 0 {
		return ProviderInfo{}, ErrAllProvidersUnhealthy
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if provider list changed
	currentIDs := getProviderIDs(healthy)
	if !stringSliceEqual(r.lastProviderIDs, currentIDs) {
		// Reinitialize state for new provider list
		r.currentWeights = make([]int, len(healthy))
		r.lastProviderIDs = currentIDs
	}

	// Calculate total weight
	totalWeight := lo.SumBy(healthy, getEffectiveWeight)

	// Smooth weighted round-robin algorithm
	// Step 1: Add configured weight to each current weight
	for i, p := range healthy {
		r.currentWeights[i] += getEffectiveWeight(p)
	}

	// Step 2: Find provider with highest current weight
	maxIdx := 0
	maxWeight := r.currentWeights[0]
	for i := 1; i < len(r.currentWeights); i++ {
		if r.currentWeights[i] > maxWeight {
			maxIdx = i
			maxWeight = r.currentWeights[i]
		}
	}

	// Step 3: Subtract total weight from winner's current weight
	r.currentWeights[maxIdx] -= totalWeight

	return healthy[maxIdx], nil
}

// Name returns the strategy name.
func (r *WeightedRoundRobinRouter) Name() string {
	return StrategyWeightedRoundRobin
}

// getEffectiveWeight returns the weight to use for a provider.
// Returns 1 if weight is <= 0 (default weight).
func getEffectiveWeight(p ProviderInfo) int {
	if p.Weight <= 0 {
		return 1
	}
	return p.Weight
}

// getProviderIDs extracts unique identifiers for providers.
// Uses Provider.Name() if available, otherwise falls back to index-based ID.
func getProviderIDs(providers []ProviderInfo) []string {
	ids := make([]string, len(providers))
	for i, p := range providers {
		if p.Provider != nil {
			ids[i] = p.Provider.Name()
		} else {
			// Fallback for test cases without actual providers
			ids[i] = ""
		}
	}
	return ids
}

// stringSliceEqual checks if two string slices are equal.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
