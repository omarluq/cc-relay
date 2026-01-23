// Package router provides provider-level routing strategies for cc-relay.
//
// This package handles provider selection (choosing which LLM backend to use),
// which is distinct from key selection within a provider (handled by keypool).
//
// The routing layer sits above the key pool:
//
//	Request -> Router (select provider) -> KeyPool (select key) -> Provider (execute)
//
// Available routing strategies:
//   - round_robin: Rotate through providers sequentially
//   - weighted_round_robin: Rotate with weights (higher weight = more requests)
//   - shuffle: Random selection for load distribution
//   - failover: Try providers in priority order until one succeeds (default)
package router

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/samber/lo"
)

// Strategy constants for configuration.
const (
	StrategyRoundRobin         = "round_robin"
	StrategyWeightedRoundRobin = "weighted_round_robin"
	StrategyShuffle            = "shuffle"
	StrategyFailover           = "failover"
)

// Common errors returned by routers.
var (
	// ErrNoProviders is returned when no providers are configured.
	ErrNoProviders = errors.New("router: no providers configured")

	// ErrAllProvidersUnhealthy is returned when all providers fail health checks.
	ErrAllProvidersUnhealthy = errors.New("router: all providers unhealthy")
)

// ProviderRouter defines the interface for provider selection strategies.
// Implementations choose which provider to use from the available pool.
//
// This mirrors the keypool.KeySelector pattern but operates at the provider level.
type ProviderRouter interface {
	// Select chooses a provider from the pool based on the strategy.
	// Returns ErrNoProviders if providers slice is empty.
	// Returns ErrAllProvidersUnhealthy if no providers pass health checks.
	Select(ctx context.Context, providers []ProviderInfo) (ProviderInfo, error)

	// Name returns the strategy name for logging and configuration.
	Name() string
}

// ProviderInfo wraps a provider with routing metadata.
// This contains all information needed for routing decisions.
type ProviderInfo struct {
	Provider  providers.Provider
	IsHealthy func() bool
	Weight    int
	Priority  int
}

// Healthy returns true if the provider is currently healthy.
// Returns true if no health check function is configured.
func (p ProviderInfo) Healthy() bool {
	if p.IsHealthy == nil {
		return true
	}
	return p.IsHealthy()
}

// FilterHealthy returns only healthy providers from the input slice.
// Uses lo.Filter for functional-style filtering.
func FilterHealthy(providerInfos []ProviderInfo) []ProviderInfo {
	return lo.Filter(providerInfos, func(p ProviderInfo, _ int) bool {
		return p.Healthy()
	})
}

// NewRouter creates a ProviderRouter based on the strategy name.
// Returns an error if the strategy is unknown or not yet implemented.
//
// Default strategy is "failover" when strategy is empty string.
//
// Supported strategies (implementations added in later plans):
//   - round_robin: Sequential rotation
//   - weighted_round_robin: Weighted sequential rotation
//   - shuffle: Random selection
//   - failover: Priority-based with fallback (default)
func NewRouter(strategy string, timeout time.Duration) (ProviderRouter, error) {
	// Normalize empty to default strategy
	if strategy == "" {
		strategy = StrategyFailover
	}

	switch strategy {
	case StrategyRoundRobin:
		return NewRoundRobinRouter(), nil
	case StrategyShuffle:
		return NewShuffleRouter(), nil
	case StrategyWeightedRoundRobin:
		return NewWeightedRoundRobinRouter(), nil
	case StrategyFailover:
		return NewFailoverRouter(timeout), nil
	default:
		return nil, fmt.Errorf("router: unknown strategy %q", strategy)
	}
}
