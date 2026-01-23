package router

import (
	"context"
	"sync/atomic"
)

// RoundRobinRouter cycles through providers in order.
// Uses atomic counter for thread-safe operation without mutex overhead.
type RoundRobinRouter struct {
	index uint64 // Atomic counter for current position
}

// NewRoundRobinRouter creates a new round-robin router.
func NewRoundRobinRouter() *RoundRobinRouter {
	return &RoundRobinRouter{}
}

// Select picks the next healthy provider in round-robin order.
// Returns ErrNoProviders if providers slice is empty.
// Returns ErrAllProvidersUnhealthy if no providers pass health checks.
func (r *RoundRobinRouter) Select(_ context.Context, providers []ProviderInfo) (ProviderInfo, error) {
	if len(providers) == 0 {
		return ProviderInfo{}, ErrNoProviders
	}

	// Filter to healthy providers only
	healthy := FilterHealthy(providers)
	if len(healthy) == 0 {
		return ProviderInfo{}, ErrAllProvidersUnhealthy
	}

	// Get next index atomically
	nextIndex := atomic.AddUint64(&r.index, 1) - 1
	healthyLen := uint64(len(healthy))
	//nolint:gosec // Safe: modulo ensures result is within int range (< len(healthy))
	idx := int(nextIndex % healthyLen)

	return healthy[idx], nil
}

// Name returns the strategy name for logging and configuration.
func (r *RoundRobinRouter) Name() string {
	return StrategyRoundRobin
}
