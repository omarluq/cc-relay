package router

import (
	"context"
	"slices"
	"sync"
	"time"
)

// RoutingResult contains the result of a routing attempt.
// Used for parallel race results where multiple providers compete.
type RoutingResult struct {
	Err      error
	Provider ProviderInfo
}

// FailoverRouter implements priority-based routing with smart parallel retry.
// It tries the primary (highest priority) provider first, and on failure
// (per trigger conditions), starts a parallel race with all providers.
// First success wins and cancels other attempts.
type FailoverRouter struct {
	triggers []FailoverTrigger
	timeout  time.Duration
}

// NewFailoverRouter creates a failover router with the given timeout.
// If timeout is 0, defaults to 5 seconds.
// If triggers is empty, uses DefaultTriggers().
func NewFailoverRouter(timeout time.Duration, triggers ...FailoverTrigger) *FailoverRouter {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	if len(triggers) == 0 {
		triggers = DefaultTriggers()
	}
	return &FailoverRouter{
		triggers: triggers,
		timeout:  timeout,
	}
}

// Select returns the highest priority healthy provider.
// For retry with parallel racing, use SelectWithRetry instead.
func (r *FailoverRouter) Select(_ context.Context, providers []ProviderInfo) (ProviderInfo, error) {
	if len(providers) == 0 {
		return ProviderInfo{}, ErrNoProviders
	}

	healthy := FilterHealthy(providers)
	if len(healthy) == 0 {
		return ProviderInfo{}, ErrAllProvidersUnhealthy
	}

	// Sort by priority descending, return highest
	sorted := sortByPriority(healthy)
	return sorted[0], nil
}

// Name returns the strategy name for logging and configuration.
func (r *FailoverRouter) Name() string {
	return StrategyFailover
}

// Timeout returns the configured parallel race timeout.
func (r *FailoverRouter) Timeout() time.Duration {
	return r.timeout
}

// Triggers returns the configured failover triggers.
func (r *FailoverRouter) Triggers() []FailoverTrigger {
	return r.triggers
}

// sortByPriority returns providers sorted by priority descending (highest first).
// Makes a copy to avoid mutating the input slice.
func sortByPriority(providers []ProviderInfo) []ProviderInfo {
	sorted := make([]ProviderInfo, len(providers))
	copy(sorted, providers)
	slices.SortStableFunc(sorted, func(a, b ProviderInfo) int {
		return b.Priority - a.Priority // Descending
	})
	return sorted
}

// SelectWithRetry implements smart parallel retry:
//  1. Try primary (highest priority healthy) provider
//  2. If fails with trigger condition, start parallel race
//  3. All providers race - first success wins
//  4. Cancel others on success or timeout
//
// tryProvider is called to actually attempt the request.
// Returns the winning provider or the last error.
func (r *FailoverRouter) SelectWithRetry(
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

	sorted := sortByPriority(healthy)

	// Simple case: only one provider
	if len(sorted) == 1 {
		_, err := tryProvider(ctx, sorted[0])
		return sorted[0], err
	}

	// Try primary first
	primary := sorted[0]
	statusCode, err := tryProvider(ctx, primary)

	if err == nil {
		return primary, nil // Primary succeeded
	}

	// Check if we should failover
	if !ShouldFailover(r.triggers, err, statusCode) {
		return primary, err // Don't failover for this error type
	}

	// Start parallel race
	return r.parallelRace(ctx, sorted, tryProvider)
}

// parallelRace launches all provider attempts in parallel and returns the first success.
// Creates a race context with timeout, starts goroutines for each provider,
// and waits for either a success or all failures.
func (r *FailoverRouter) parallelRace(
	ctx context.Context,
	providers []ProviderInfo,
	tryProvider func(context.Context, ProviderInfo) (int, error),
) (ProviderInfo, error) {
	raceCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	resultCh := make(chan RoutingResult, len(providers))

	// Launch all attempts in parallel
	var wg sync.WaitGroup
	for _, p := range providers {
		wg.Add(1)
		go func(provider ProviderInfo) {
			defer wg.Done()

			_, err := tryProvider(raceCtx, provider)
			select {
			case resultCh <- RoutingResult{Provider: provider, Err: err}:
			case <-raceCtx.Done():
			}
		}(p)
	}

	// Close result channel when all done
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Wait for first success or all failures
	var lastErr error
	for result := range resultCh {
		if result.Err == nil {
			cancel() // Cancel other attempts
			return result.Provider, nil
		}
		lastErr = result.Err
	}

	return ProviderInfo{}, lastErr
}
