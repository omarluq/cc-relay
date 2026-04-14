package router_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/router"
)

func TestWeightedFailoverRouterName(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(0)
	if rtr.Name() != router.StrategyWeightedFailover {
		t.Errorf("Name() = %q, want %q", rtr.Name(), router.StrategyWeightedFailover)
	}
}

func TestWeightedFailoverRouterDefaultTimeout(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(0)
	if rtr.Timeout() != 5*time.Second {
		t.Errorf("Timeout() = %v, want %v", rtr.Timeout(), 5*time.Second)
	}
}

func TestWeightedFailoverRouterCustomTimeout(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(10 * time.Second)
	if rtr.Timeout() != 10*time.Second {
		t.Errorf("Timeout() = %v, want %v", rtr.Timeout(), 10*time.Second)
	}
}

func TestWeightedFailoverRouterDefaultTriggers(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(0)
	triggers := rtr.Triggers()
	if len(triggers) != 3 {
		t.Errorf("Triggers() count = %d, want 3", len(triggers))
	}
}

func TestWeightedFailoverRouterCustomTriggers(t *testing.T) {
	t.Parallel()

	customTrigger := router.NewStatusCodeTrigger(500)
	rtr := router.NewWeightedFailoverRouter(0, customTrigger)
	triggers := rtr.Triggers()
	if len(triggers) != 1 {
		t.Errorf("Triggers() count = %d, want 1", len(triggers))
	}
}

func TestWeightedFailoverRouterSelectNoProviders(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(0)
	_, err := rtr.Select(context.Background(), []router.ProviderInfo{})

	if !errors.Is(err, router.ErrNoProviders) {
		t.Errorf("Select() error = %v, want %v", err, router.ErrNoProviders)
	}
}

func TestWeightedFailoverRouterSelectAllUnhealthy(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(0)
	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, router.NeverHealthy()),
		router.NewTestProviderInfo("p2", 2, 2, router.NeverHealthy()),
	}

	_, err := rtr.Select(context.Background(), providers)
	if !errors.Is(err, router.ErrAllProvidersUnhealthy) {
		t.Errorf("Select() error = %v, want %v", err, router.ErrAllProvidersUnhealthy)
	}
}

func TestWeightedFailoverRouterSelectSingleProvider(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(0)
	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("only", 1, 5, router.AlwaysHealthy()),
	}

	result, err := rtr.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if result.Provider.Name() != "only" {
		t.Errorf("Select() = %q, want only", result.Provider.Name())
	}
}

func TestWeightedFailoverRouterSelectSkipsUnhealthy(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(0)
	// highWeight unhealthy, lowWeight healthy
	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("heavy", 1, 100, router.NeverHealthy()),
		router.NewTestProviderInfo("light", 1, 1, router.AlwaysHealthy()),
	}

	result, err := rtr.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if result.Provider.Name() != "light" {
		t.Errorf("Select() = %q, want light (only healthy)", result.Provider.Name())
	}
}

func TestWeightedFailoverRouterSelectWithRetryNoProviders(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(0)
	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		return 200, nil
	}
	_, err := rtr.SelectWithRetry(context.Background(), []router.ProviderInfo{}, tryProvider)
	if !errors.Is(err, router.ErrNoProviders) {
		t.Errorf("SelectWithRetry() error = %v, want %v", err, router.ErrNoProviders)
	}
}

func TestWeightedFailoverRouterSelectWithRetryAllUnhealthy(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(0)
	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		return 200, nil
	}
	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, router.NeverHealthy()),
	}
	_, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
	if !errors.Is(err, router.ErrAllProvidersUnhealthy) {
		t.Errorf("SelectWithRetry() error = %v, want %v", err, router.ErrAllProvidersUnhealthy)
	}
}

func TestWeightedFailoverRouterSelectWithRetrySucceeds(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(0)
	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		return 200, nil
	}
	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 5, router.AlwaysHealthy()),
		router.NewTestProviderInfo("p2", 2, 3, router.AlwaysHealthy()),
	}

	result, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
	if err != nil {
		t.Fatalf("SelectWithRetry() unexpected error: %v", err)
	}
	// Should succeed on first try
	if result.Provider == nil {
		t.Error("SelectWithRetry() returned nil provider")
	}
}

func TestWeightedFailoverRouterSelectWithRetryFailsOver(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(0, router.NewStatusCodeTrigger(500))
	callCount := 0
	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		callCount++
		if callCount == 1 {
			return 500, errors.New("first fails")
		}
		return 200, nil
	}
	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 5, router.AlwaysHealthy()),
		router.NewTestProviderInfo("p2", 2, 3, router.AlwaysHealthy()),
	}

	result, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
	if err != nil {
		t.Fatalf("SelectWithRetry() unexpected error: %v", err)
	}
	if callCount < 2 {
		t.Errorf("tryProvider called %d times, want >= 2 (failover)", callCount)
	}
	if result.Provider == nil {
		t.Error("SelectWithRetry() returned nil provider")
	}
}

func TestWeightedFailoverRouterSelectWithRetryNoFailoverOnNonTrigger(t *testing.T) {
	t.Parallel()

	// Only 429 triggers failover
	rtr := router.NewWeightedFailoverRouter(0, router.NewStatusCodeTrigger(429))
	errBadRequest := errors.New("bad request")
	callCount := 0
	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		callCount++
		return 400, errBadRequest // 400 should not trigger failover
	}
	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 5, router.AlwaysHealthy()),
		router.NewTestProviderInfo("p2", 2, 3, router.AlwaysHealthy()),
	}

	_, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
	if !errors.Is(err, errBadRequest) {
		t.Errorf("SelectWithRetry() error = %v, want %v", err, errBadRequest)
	}
	if callCount != 1 {
		t.Errorf("tryProvider called %d times, want 1 (no failover)", callCount)
	}
}

func TestWeightedFailoverRouterSelectWithRetryAllFail(t *testing.T) {
	t.Parallel()

	rtr := router.NewWeightedFailoverRouter(0, router.NewStatusCodeTrigger(500))
	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		return 500, errors.New("all fail")
	}
	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 5, router.AlwaysHealthy()),
		router.NewTestProviderInfo("p2", 2, 3, router.AlwaysHealthy()),
	}

	_, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
	if err == nil {
		t.Error("SelectWithRetry() should have returned error when all providers fail")
	}
}

// Verify interface compliance.
var _ router.ProviderRouter = (*router.WeightedFailoverRouter)(nil)
