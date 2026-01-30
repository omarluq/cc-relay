package router

import (
	"context"
	"time"
)

// WeightedFailoverRouter attempts providers in a weighted order.
// On retryable errors it fails over to the next provider in the order.
type WeightedFailoverRouter struct {
	triggers []FailoverTrigger
	timeout  time.Duration
}

// NewWeightedFailoverRouter creates a weighted failover router with the given timeout.
// If timeout is 0, defaults to 5 seconds. If triggers is empty, uses DefaultTriggers().
func NewWeightedFailoverRouter(timeout time.Duration, triggers ...FailoverTrigger) *WeightedFailoverRouter {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	if len(triggers) == 0 {
		triggers = DefaultTriggers()
	}
	return &WeightedFailoverRouter{
		triggers: triggers,
		timeout:  timeout,
	}
}

// Select returns the first provider in the weighted order.
func (r *WeightedFailoverRouter) Select(_ context.Context, providers []ProviderInfo) (ProviderInfo, error) {
	if len(providers) == 0 {
		return ProviderInfo{}, ErrNoProviders
	}

	healthy := FilterHealthy(providers)
	if len(healthy) == 0 {
		return ProviderInfo{}, ErrAllProvidersUnhealthy
	}

	order := r.weightedOrder(healthy)
	return order[0], nil
}

// Name returns the strategy name.
func (r *WeightedFailoverRouter) Name() string {
	return StrategyWeightedFailover
}

// Timeout returns the configured timeout.
func (r *WeightedFailoverRouter) Timeout() time.Duration {
	return r.timeout
}

// Triggers returns the configured failover triggers.
func (r *WeightedFailoverRouter) Triggers() []FailoverTrigger {
	return r.triggers
}

// SelectWithRetry attempts providers in weighted order and fails over on trigger errors.
func (r *WeightedFailoverRouter) SelectWithRetry(
	ctx context.Context,
	providers []ProviderInfo,
	tryProvider func(context.Context, ProviderInfo) (statusCode int, err error),
) (ProviderInfo, error) {
	if len(providers) == 0 {
		return ProviderInfo{}, ErrNoProviders
	}

	healthy := FilterHealthy(providers)
	if len(healthy) == 0 {
		return ProviderInfo{}, ErrAllProvidersUnhealthy
	}

	order := r.weightedOrder(healthy)
	var lastErr error
	for _, provider := range order {
		statusCode, err := tryProvider(ctx, provider)
		if err == nil {
			return provider, nil
		}
		lastErr = err
		if !ShouldFailover(r.triggers, err, statusCode) {
			return provider, err
		}
	}

	if lastErr == nil {
		lastErr = context.DeadlineExceeded
	}
	return ProviderInfo{}, lastErr
}

func (r *WeightedFailoverRouter) weightedOrder(providers []ProviderInfo) []ProviderInfo {
	remaining := make([]ProviderInfo, len(providers))
	copy(remaining, providers)

	order := make([]ProviderInfo, 0, len(remaining))

	for len(remaining) > 0 {
		idx := weightedIndex(remaining)
		order = append(order, remaining[idx])
		remaining = append(remaining[:idx], remaining[idx+1:]...)
	}
	return order
}

func weightedIndex(providers []ProviderInfo) int {
	total := 0
	for _, p := range providers {
		total += effectiveWeight(p.Weight)
	}
	if total <= 0 {
		return 0
	}

	roll := randIntn(total)
	for i, p := range providers {
		w := effectiveWeight(p.Weight)
		if roll < w {
			return i
		}
		roll -= w
	}
	return len(providers) - 1
}

func effectiveWeight(weight int) int {
	if weight <= 0 {
		return 1
	}
	return weight
}
