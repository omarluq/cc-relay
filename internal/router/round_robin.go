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
	return &RoundRobinRouter{
		index: 0,
	}
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
	healthyLen := len(healthy)
	idx64 := nextIndex % uint64(healthyLen)
	maxInt := uint64(int(^uint(0) >> 1))
	if idx64 > maxInt {
		return ProviderInfo{}, ErrNoProviders
	}
	idx := int(idx64)

	return healthy[idx], nil
}

// Name returns the strategy name for logging and configuration.
func (r *RoundRobinRouter) Name() string {
	return StrategyRoundRobin
}
