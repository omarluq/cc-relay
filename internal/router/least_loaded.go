package router

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/omarluq/cc-relay/internal/providers"
)

// LeastLoadedRouter selects the provider with the fewest in-flight requests.
// Tie-breaker: highest priority, then random among remaining ties.
type LeastLoadedRouter struct {
	inFlight map[string]*atomic.Int64
	mu       sync.Mutex
}

// NewLeastLoadedRouter creates a new least-loaded router.
func NewLeastLoadedRouter() *LeastLoadedRouter {
	return &LeastLoadedRouter{
		inFlight: make(map[string]*atomic.Int64),
		mu:       sync.Mutex{},
	}
}

// Select chooses the least loaded healthy provider.
func (r *LeastLoadedRouter) Select(_ context.Context, infos []ProviderInfo) (ProviderInfo, error) {
	if len(infos) == 0 {
		return ProviderInfo{}, ErrNoProviders
	}

	healthy := FilterHealthy(infos)
	if len(healthy) == 0 {
		return ProviderInfo{}, ErrAllProvidersUnhealthy
	}

	minLoad := int64(-1)
	var candidates []ProviderInfo
	for _, providerInfo := range healthy {
		load := r.getCounter(providerInfo.Provider.Name()).Load()
		if minLoad == -1 || load < minLoad {
			minLoad = load
			candidates = []ProviderInfo{providerInfo}
			continue
		}
		if load == minLoad {
			candidates = append(candidates, providerInfo)
		}
	}

	// Tie-breaker: highest priority.
	candidates = sortByPriority(candidates)
	highestPriority := candidates[0].Priority
	var priorityTies []ProviderInfo
	for _, providerInfo := range candidates {
		if providerInfo.Priority != highestPriority {
			break
		}
		priorityTies = append(priorityTies, providerInfo)
	}

	if len(priorityTies) == 1 {
		return priorityTies[0], nil
	}

	// Random tie-break among equal priority.
	idx := randIntn(len(priorityTies))
	return priorityTies[idx], nil
}

// Name returns the strategy name.
func (r *LeastLoadedRouter) Name() string {
	return StrategyLeastLoaded
}

// Acquire increments the in-flight count for a provider.
func (r *LeastLoadedRouter) Acquire(provider providers.Provider) {
	r.getCounter(provider.Name()).Add(1)
}

// Release decrements the in-flight count for a provider.
func (r *LeastLoadedRouter) Release(provider providers.Provider) {
	r.getCounter(provider.Name()).Add(-1)
}

func (r *LeastLoadedRouter) getCounter(name string) *atomic.Int64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	if counter, ok := r.inFlight[name]; ok {
		return counter
	}
	counter := &atomic.Int64{}
	r.inFlight[name] = counter
	return counter
}
