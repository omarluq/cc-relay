package router_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/router"
)

func TestFailoverRouterName(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(0)
	if rtr.Name() != router.StrategyFailover {
		t.Errorf("Name() = %q, want %q", rtr.Name(), router.StrategyFailover)
	}
}

func TestFailoverRouterDefaultTimeout(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(0)
	if rtr.Timeout() != 5*time.Second {
		t.Errorf("Timeout() = %v, want %v", rtr.Timeout(), 5*time.Second)
	}
}

func TestFailoverRouterCustomTimeout(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(10 * time.Second)
	if rtr.Timeout() != 10*time.Second {
		t.Errorf("Timeout() = %v, want %v", rtr.Timeout(), 10*time.Second)
	}
}

func TestFailoverRouterDefaultTriggers(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(0)
	triggers := rtr.Triggers()
	if len(triggers) != 3 {
		t.Errorf("Triggers() count = %d, want 3 (status, timeout, connection)", len(triggers))
	}
}

func TestFailoverRouterCustomTriggers(t *testing.T) {
	t.Parallel()

	customTrigger := router.NewStatusCodeTrigger(500)
	rtr := router.NewFailoverRouter(0, customTrigger)
	triggers := rtr.Triggers()
	if len(triggers) != 1 {
		t.Errorf("Triggers() count = %d, want 1", len(triggers))
	}
	if triggers[0].Name() != router.TriggerStatusCode {
		t.Errorf("Triggers()[0].Name() = %q, want %q", triggers[0].Name(), router.TriggerStatusCode)
	}
}

func TestFailoverRouterSelectEmptyProviders(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(0)
	_, err := rtr.Select(context.Background(), []router.ProviderInfo{})
	if !errors.Is(err, router.ErrNoProviders) {
		t.Errorf("Select() error = %v, want %v", err, router.ErrNoProviders)
	}
}

func TestFailoverRouterSelectAllUnhealthy(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(0)
	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Priority: 1, Weight: 0, IsHealthy: func() bool { return false }},
		{Provider: router.NewTestProvider("p2"), Priority: 2, Weight: 0, IsHealthy: func() bool { return false }},
	}
	_, err := rtr.Select(context.Background(), providers)
	if !errors.Is(err, router.ErrAllProvidersUnhealthy) {
		t.Errorf("Select() error = %v, want %v", err, router.ErrAllProvidersUnhealthy)
	}
}

func TestFailoverRouterSelectReturnsHighestPriority(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(0)
	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Priority: 1, Weight: 1, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p2"), Priority: 3, Weight: 3, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p3"), Priority: 2, Weight: 2, IsHealthy: func() bool { return true }},
	}
	result, err := rtr.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if result.Priority != 3 {
		t.Errorf("Select() returned Priority = %d, want 3", result.Priority)
	}
	if result.Weight != 3 {
		t.Errorf("Select() returned Weight = %d, want 3", result.Weight)
	}
}

func TestFailoverRouterSelectSkipsUnhealthyHighPriority(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(0)
	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Priority: 1, Weight: 1, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p2"), Priority: 3, Weight: 3, IsHealthy: func() bool { return false }},
		{Provider: router.NewTestProvider("p3"), Priority: 2, Weight: 2, IsHealthy: func() bool { return true }},
	}
	result, err := rtr.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if result.Priority != 2 {
		t.Errorf("Select() returned Priority = %d, want 2 (highest healthy)", result.Priority)
	}
}

func TestFailoverRouterSelectWithRetryEmptyProviders(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(0)
	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		return 200, nil
	}
	_, err := rtr.SelectWithRetry(context.Background(), []router.ProviderInfo{}, tryProvider)
	if !errors.Is(err, router.ErrNoProviders) {
		t.Errorf("SelectWithRetry() error = %v, want %v", err, router.ErrNoProviders)
	}
}

func TestFailoverRouterSelectWithRetryAllUnhealthy(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(0)
	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		return 200, nil
	}
	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Priority: 1, Weight: 0, IsHealthy: func() bool { return false }},
	}
	_, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
	if !errors.Is(err, router.ErrAllProvidersUnhealthy) {
		t.Errorf("SelectWithRetry() error = %v, want %v", err, router.ErrAllProvidersUnhealthy)
	}
}

func TestFailoverRouterSelectWithRetryPrimarySucceeds(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(0)
	var callCount atomic.Int32

	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		callCount.Add(1)
		return 200, nil
	}

	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("primary"), Priority: 2, Weight: 2, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("fallback"), Priority: 1, Weight: 1, IsHealthy: func() bool { return true }},
	}

	result, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
	if err != nil {
		t.Fatalf("SelectWithRetry() unexpected error: %v", err)
	}
	if result.Priority != 2 {
		t.Errorf("SelectWithRetry() returned Priority = %d, want 2 (primary)", result.Priority)
	}
	if callCount.Load() != 1 {
		t.Errorf("tryProvider called %d times, want 1 (primary only)", callCount.Load())
	}
}

func TestFailoverRouterSelectWithRetrySingleProvider(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(0)
	errFailed := errors.New("provider failed")

	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		return 500, errFailed
	}

	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("single"), Priority: 1, Weight: 0, IsHealthy: func() bool { return true }},
	}

	result, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
	if !errors.Is(err, errFailed) {
		t.Errorf("SelectWithRetry() error = %v, want %v", err, errFailed)
	}
	if result.Priority != 1 {
		t.Errorf("SelectWithRetry() should return single provider even on error")
	}
}

func TestFailoverRouterSelectWithRetryFailoverOnTrigger(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(100*time.Millisecond, router.NewStatusCodeTrigger(429))
	var callCount atomic.Int32

	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		count := callCount.Add(1)
		if count == 1 {
			// Primary fails with trigger condition
			return 429, errors.New("rate limited")
		}
		// Fallbacks succeed
		return 200, nil
	}

	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("primary"), Priority: 2, Weight: 2, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("fallback"), Priority: 1, Weight: 1, IsHealthy: func() bool { return true }},
	}

	result, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
	if err != nil {
		t.Fatalf("SelectWithRetry() unexpected error: %v", err)
	}

	// In parallel race, either primary or fallback can win
	// Wait a bit for all goroutines to complete
	time.Sleep(50 * time.Millisecond)

	// Should have tried at least primary, and maybe fallbacks
	if callCount.Load() < 1 {
		t.Errorf("tryProvider called %d times, want at least 1", callCount.Load())
	}

	// Result should be a valid provider
	if result.Priority != 1 && result.Priority != 2 {
		t.Errorf("SelectWithRetry() returned invalid Priority = %d", result.Priority)
	}
}

func TestFailoverRouterSelectWithRetryNoFailoverOnNonTrigger(t *testing.T) {
	t.Parallel()

	// Only 429 triggers failover
	rtr := router.NewFailoverRouter(100*time.Millisecond, router.NewStatusCodeTrigger(429))
	var callCount atomic.Int32

	errBadRequest := errors.New("bad request")
	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		callCount.Add(1)
		return 400, errBadRequest // 400 doesn't trigger failover
	}

	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("primary"), Priority: 2, Weight: 0, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("fallback"), Priority: 1, Weight: 0, IsHealthy: func() bool { return true }},
	}

	_, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
	if !errors.Is(err, errBadRequest) {
		t.Errorf("SelectWithRetry() error = %v, want %v", err, errBadRequest)
	}
	if callCount.Load() != 1 {
		t.Errorf("tryProvider called %d times, want 1 (no failover for 400)", callCount.Load())
	}
}

func TestFailoverRouterSelectWithRetryFirstSuccessWins(t *testing.T) {
	t.Parallel()

	var mutex sync.Mutex
	startedOrder := make([]int, 0)
	finishedOrder := make([]int, 0)

	tryProvider := func(ctx context.Context, providerInfo router.ProviderInfo) (int, error) {
		mutex.Lock()
		startedOrder = append(startedOrder, providerInfo.Priority)
		mutex.Unlock()

		// Priority 1 finishes fast, Priority 2 finishes slow
		if providerInfo.Priority == 1 {
			time.Sleep(10 * time.Millisecond)
		} else {
			time.Sleep(100 * time.Millisecond)
		}

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			mutex.Lock()
			finishedOrder = append(finishedOrder, providerInfo.Priority)
			mutex.Unlock()
			return 200, nil
		}
	}

	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("slow"), Priority: 2, Weight: 0, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("fast"), Priority: 1, Weight: 0, IsHealthy: func() bool { return true }},
	}

	// Use status code trigger to ensure failover
	rtr := router.NewFailoverRouter(5*time.Second, router.NewStatusCodeTrigger(500))

	// First call primary (which we need to fail to trigger parallel race)
	var callCount atomic.Int32
	tryProviderWithFail := func(ctx context.Context, providerInfo router.ProviderInfo) (int, error) {
		count := callCount.Add(1)
		if count == 1 {
			// Primary fails immediately
			return 500, errors.New("primary failed")
		}
		return tryProvider(ctx, providerInfo)
	}

	result, err := rtr.SelectWithRetry(context.Background(), providers, tryProviderWithFail)
	if err != nil {
		t.Fatalf("SelectWithRetry() unexpected error: %v", err)
	}

	// The fast provider (Priority 1) should win
	if result.Priority != 1 {
		t.Errorf("SelectWithRetry() Priority = %d, want 1 (fast provider)", result.Priority)
	}
}

func TestFailoverRouterSelectWithRetryTimeoutRespected(t *testing.T) {
	t.Parallel()

	shortTimeout := 50 * time.Millisecond
	rtr := router.NewFailoverRouter(shortTimeout, router.NewStatusCodeTrigger(500))

	var callCount atomic.Int32

	tryProvider := func(ctx context.Context, _ router.ProviderInfo) (int, error) {
		count := callCount.Add(1)
		if count == 1 {
			// Primary fails immediately to trigger parallel race
			return 500, errors.New("primary failed")
		}

		// All fallbacks are slow - much longer than timeout
		select {
		case <-time.After(500 * time.Millisecond):
			return 200, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}

	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Priority: 3, Weight: 0, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p2"), Priority: 2, Weight: 0, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p3"), Priority: 1, Weight: 0, IsHealthy: func() bool { return true }},
	}

	start := time.Now()
	_, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
	elapsed := time.Since(start)

	// Should fail due to timeout - all providers are slow
	if err == nil {
		t.Error("SelectWithRetry() should have returned error when all providers timeout")
	}

	// The key assertion: it should complete around timeout, not wait for slow providers
	// Allow 3x margin for CI timing variance
	if elapsed > 3*shortTimeout {
		t.Errorf("SelectWithRetry() took %v, want around %v (timeout enforcement)", elapsed, shortTimeout)
	}
}

func TestFailoverRouterSelectWithRetryConcurrentSafety(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(100 * time.Millisecond)
	var successCount atomic.Int32
	var errorCount atomic.Int32

	tryProvider := func(_ context.Context, providerInfo router.ProviderInfo) (int, error) {
		if providerInfo.Priority%2 == 0 {
			return 200, nil
		}
		return 500, errors.New("odd priority fails")
	}

	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p4"), Priority: 4, Weight: 0, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p3"), Priority: 3, Weight: 0, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p2"), Priority: 2, Weight: 0, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p1"), Priority: 1, Weight: 0, IsHealthy: func() bool { return true }},
	}

	var waitGroup sync.WaitGroup
	for idx := 0; idx < 100; idx++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
			if err != nil {
				errorCount.Add(1)
			} else {
				successCount.Add(1)
			}
		}()
	}

	waitGroup.Wait()

	total := successCount.Load() + errorCount.Load()
	if total != 100 {
		t.Errorf("Total operations = %d, want 100", total)
	}

	// Primary (Priority 4) is even, so should succeed
	if successCount.Load() != 100 {
		t.Errorf("Success count = %d, want 100 (primary succeeds)", successCount.Load())
	}
}

func TestFailoverRouterSortByPriority(t *testing.T) {
	t.Parallel()

	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Priority: 1, Weight: 1, IsHealthy: nil},
		{Provider: router.NewTestProvider("p3"), Priority: 3, Weight: 3, IsHealthy: nil},
		{Provider: router.NewTestProvider("p2"), Priority: 2, Weight: 2, IsHealthy: nil},
		{Provider: router.NewTestProvider("p5"), Priority: 5, Weight: 5, IsHealthy: nil},
		{Provider: router.NewTestProvider("p4"), Priority: 4, Weight: 4, IsHealthy: nil},
	}

	sorted := router.SortByPriority(providers)

	// Should be in descending order
	expected := []int{5, 4, 3, 2, 1}
	for idx, prov := range sorted {
		if prov.Priority != expected[idx] {
			t.Errorf("router.SortByPriority()[%d].Priority = %d, want %d", idx, prov.Priority, expected[idx])
		}
	}

	// Original should be unmodified
	if providers[0].Priority != 1 {
		t.Error("router.SortByPriority() modified input slice")
	}
}

func TestFailoverRouterSortByPriorityStableSort(t *testing.T) {
	t.Parallel()

	// Same priority, different weights - should preserve order
	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Priority: 1, Weight: 10, IsHealthy: nil},
		{Provider: router.NewTestProvider("p2"), Priority: 1, Weight: 20, IsHealthy: nil},
		{Provider: router.NewTestProvider("p3"), Priority: 1, Weight: 30, IsHealthy: nil},
	}

	sorted := router.SortByPriority(providers)

	// Should preserve original order for equal priorities
	expected := []int{10, 20, 30}
	for idx, prov := range sorted {
		if prov.Weight != expected[idx] {
			t.Errorf("router.SortByPriority()[%d].Weight = %d, want %d (stable sort)", idx, prov.Weight, expected[idx])
		}
	}
}

func TestRoutingResult(t *testing.T) {
	t.Parallel()

	provider := router.ProviderInfo{Provider: router.NewTestProvider("test"), Priority: 1, Weight: 0, IsHealthy: nil}
	err := errors.New("test error")

	result := router.RoutingResult{
		Provider: provider,
		Err:      err,
	}

	if result.Provider.Priority != 1 {
		t.Errorf("RoutingResult.Provider.Priority = %d, want 1", result.Provider.Priority)
	}
	if !errors.Is(result.Err, err) {
		t.Errorf("RoutingResult.Err = %v, want %v", result.Err, err)
	}
}

// Test that FailoverRouter implements ProviderRouter interface.
func TestFailoverRouterImplementsProviderRouter(t *testing.T) {
	t.Parallel()

	var _ router.ProviderRouter = (*router.FailoverRouter)(nil)
}

func TestFailoverRouterParallelRaceAllFail(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(100*time.Millisecond, router.NewStatusCodeTrigger(500))
	lastErr := errors.New("last error")
	var callCount atomic.Int32

	tryProvider := func(_ context.Context, _ router.ProviderInfo) (int, error) {
		count := callCount.Add(1)
		if count == 1 {
			return 500, errors.New("primary failed") // Trigger parallel race
		}
		// All parallel attempts fail
		time.Sleep(10 * time.Millisecond) // Small delay
		return 500, lastErr
	}

	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Priority: 2, Weight: 0, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p2"), Priority: 1, Weight: 0, IsHealthy: func() bool { return true }},
	}

	_, err := rtr.SelectWithRetry(context.Background(), providers, tryProvider)
	if err == nil {
		t.Error("SelectWithRetry() should have returned error when all fail")
	}
}

func TestFailoverRouterContextCancellation(t *testing.T) {
	t.Parallel()

	rtr := router.NewFailoverRouter(5*time.Second, router.NewStatusCodeTrigger(500))
	var callCount atomic.Int32

	tryProvider := func(ctx context.Context, _ router.ProviderInfo) (int, error) {
		count := callCount.Add(1)
		if count == 1 {
			return 500, errors.New("primary failed")
		}
		// Block until context canceled
		<-ctx.Done()
		return 0, ctx.Err()
	}

	providers := []router.ProviderInfo{
		{Provider: router.NewTestProvider("p1"), Priority: 2, Weight: 0, IsHealthy: func() bool { return true }},
		{Provider: router.NewTestProvider("p2"), Priority: 1, Weight: 0, IsHealthy: func() bool { return true }},
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := rtr.SelectWithRetry(ctx, providers, tryProvider)
	elapsed := time.Since(start)

	// Should complete quickly after cancellation (regardless of error/success)
	// The key is that cancellation stops the parallel race promptly
	if elapsed > 200*time.Millisecond {
		t.Errorf("SelectWithRetry() took %v, should complete shortly after cancel", elapsed)
	}

	// Ensure error is non-nil to validate context cancellation propagates
	if err == nil {
		t.Error("SelectWithRetry() should return error when context is canceled")
	}
}
