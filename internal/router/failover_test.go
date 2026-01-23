package router

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFailoverRouter_Name(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(0)
	if router.Name() != StrategyFailover {
		t.Errorf("Name() = %q, want %q", router.Name(), StrategyFailover)
	}
}

func TestFailoverRouter_DefaultTimeout(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(0)
	if router.Timeout() != 5*time.Second {
		t.Errorf("Timeout() = %v, want %v", router.Timeout(), 5*time.Second)
	}
}

func TestFailoverRouter_CustomTimeout(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(10 * time.Second)
	if router.Timeout() != 10*time.Second {
		t.Errorf("Timeout() = %v, want %v", router.Timeout(), 10*time.Second)
	}
}

func TestFailoverRouter_DefaultTriggers(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(0)
	triggers := router.Triggers()
	if len(triggers) != 3 {
		t.Errorf("Triggers() count = %d, want 3 (status, timeout, connection)", len(triggers))
	}
}

func TestFailoverRouter_CustomTriggers(t *testing.T) {
	t.Parallel()

	customTrigger := NewStatusCodeTrigger(500)
	router := NewFailoverRouter(0, customTrigger)
	triggers := router.Triggers()
	if len(triggers) != 1 {
		t.Errorf("Triggers() count = %d, want 1", len(triggers))
	}
	if triggers[0].Name() != TriggerStatusCode {
		t.Errorf("Triggers()[0].Name() = %q, want %q", triggers[0].Name(), TriggerStatusCode)
	}
}

func TestFailoverRouter_Select_EmptyProviders(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(0)
	_, err := router.Select(context.Background(), []ProviderInfo{})
	if !errors.Is(err, ErrNoProviders) {
		t.Errorf("Select() error = %v, want %v", err, ErrNoProviders)
	}
}

func TestFailoverRouter_Select_AllUnhealthy(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(0)
	providers := []ProviderInfo{
		{Priority: 1, IsHealthy: func() bool { return false }},
		{Priority: 2, IsHealthy: func() bool { return false }},
	}
	_, err := router.Select(context.Background(), providers)
	if !errors.Is(err, ErrAllProvidersUnhealthy) {
		t.Errorf("Select() error = %v, want %v", err, ErrAllProvidersUnhealthy)
	}
}

func TestFailoverRouter_Select_ReturnsHighestPriority(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(0)
	providers := []ProviderInfo{
		{Priority: 1, Weight: 1, IsHealthy: func() bool { return true }},
		{Priority: 3, Weight: 3, IsHealthy: func() bool { return true }}, // Highest priority
		{Priority: 2, Weight: 2, IsHealthy: func() bool { return true }},
	}
	result, err := router.Select(context.Background(), providers)
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

func TestFailoverRouter_Select_SkipsUnhealthyHighPriority(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(0)
	providers := []ProviderInfo{
		{Priority: 1, Weight: 1, IsHealthy: func() bool { return true }},
		{Priority: 3, Weight: 3, IsHealthy: func() bool { return false }}, // Highest but unhealthy
		{Priority: 2, Weight: 2, IsHealthy: func() bool { return true }},  // Next highest healthy
	}
	result, err := router.Select(context.Background(), providers)
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if result.Priority != 2 {
		t.Errorf("Select() returned Priority = %d, want 2 (highest healthy)", result.Priority)
	}
}

func TestFailoverRouter_SelectWithRetry_EmptyProviders(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(0)
	tryProvider := func(_ context.Context, _ ProviderInfo) (int, error) {
		return 200, nil
	}
	_, err := router.SelectWithRetry(context.Background(), []ProviderInfo{}, tryProvider)
	if !errors.Is(err, ErrNoProviders) {
		t.Errorf("SelectWithRetry() error = %v, want %v", err, ErrNoProviders)
	}
}

func TestFailoverRouter_SelectWithRetry_AllUnhealthy(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(0)
	tryProvider := func(_ context.Context, _ ProviderInfo) (int, error) {
		return 200, nil
	}
	providers := []ProviderInfo{
		{Priority: 1, IsHealthy: func() bool { return false }},
	}
	_, err := router.SelectWithRetry(context.Background(), providers, tryProvider)
	if !errors.Is(err, ErrAllProvidersUnhealthy) {
		t.Errorf("SelectWithRetry() error = %v, want %v", err, ErrAllProvidersUnhealthy)
	}
}

func TestFailoverRouter_SelectWithRetry_PrimarySucceeds(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(0)
	var callCount atomic.Int32

	tryProvider := func(_ context.Context, _ ProviderInfo) (int, error) {
		callCount.Add(1)
		return 200, nil
	}

	providers := []ProviderInfo{
		{Priority: 2, Weight: 2, IsHealthy: func() bool { return true }}, // Primary
		{Priority: 1, Weight: 1, IsHealthy: func() bool { return true }}, // Fallback
	}

	result, err := router.SelectWithRetry(context.Background(), providers, tryProvider)
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

func TestFailoverRouter_SelectWithRetry_SingleProvider(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(0)
	errFailed := errors.New("provider failed")

	tryProvider := func(_ context.Context, _ ProviderInfo) (int, error) {
		return 500, errFailed
	}

	providers := []ProviderInfo{
		{Priority: 1, IsHealthy: func() bool { return true }},
	}

	result, err := router.SelectWithRetry(context.Background(), providers, tryProvider)
	if !errors.Is(err, errFailed) {
		t.Errorf("SelectWithRetry() error = %v, want %v", err, errFailed)
	}
	if result.Priority != 1 {
		t.Errorf("SelectWithRetry() should return single provider even on error")
	}
}

func TestFailoverRouter_SelectWithRetry_FailoverOnTrigger(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(100*time.Millisecond, NewStatusCodeTrigger(429))
	var callCount atomic.Int32

	tryProvider := func(_ context.Context, _ ProviderInfo) (int, error) {
		count := callCount.Add(1)
		if count == 1 {
			// Primary fails with trigger condition
			return 429, errors.New("rate limited")
		}
		// Fallbacks succeed
		return 200, nil
	}

	providers := []ProviderInfo{
		{Priority: 2, Weight: 2, IsHealthy: func() bool { return true }}, // Primary - will fail
		{Priority: 1, Weight: 1, IsHealthy: func() bool { return true }}, // Fallback - will succeed
	}

	result, err := router.SelectWithRetry(context.Background(), providers, tryProvider)
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

func TestFailoverRouter_SelectWithRetry_NoFailoverOnNonTrigger(t *testing.T) {
	t.Parallel()

	// Only 429 triggers failover
	router := NewFailoverRouter(100*time.Millisecond, NewStatusCodeTrigger(429))
	var callCount atomic.Int32

	errBadRequest := errors.New("bad request")
	tryProvider := func(_ context.Context, _ ProviderInfo) (int, error) {
		callCount.Add(1)
		return 400, errBadRequest // 400 doesn't trigger failover
	}

	providers := []ProviderInfo{
		{Priority: 2, IsHealthy: func() bool { return true }},
		{Priority: 1, IsHealthy: func() bool { return true }},
	}

	_, err := router.SelectWithRetry(context.Background(), providers, tryProvider)
	if !errors.Is(err, errBadRequest) {
		t.Errorf("SelectWithRetry() error = %v, want %v", err, errBadRequest)
	}
	if callCount.Load() != 1 {
		t.Errorf("tryProvider called %d times, want 1 (no failover for 400)", callCount.Load())
	}
}

func TestFailoverRouter_SelectWithRetry_FirstSuccessWins(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	startedOrder := make([]int, 0)
	finishedOrder := make([]int, 0)

	tryProvider := func(ctx context.Context, p ProviderInfo) (int, error) {
		mu.Lock()
		startedOrder = append(startedOrder, p.Priority)
		mu.Unlock()

		// Priority 1 finishes fast, Priority 2 finishes slow
		if p.Priority == 1 {
			time.Sleep(10 * time.Millisecond)
		} else {
			time.Sleep(100 * time.Millisecond)
		}

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			mu.Lock()
			finishedOrder = append(finishedOrder, p.Priority)
			mu.Unlock()
			return 200, nil
		}
	}

	providers := []ProviderInfo{
		{Priority: 2, IsHealthy: func() bool { return true }}, // Primary - slow
		{Priority: 1, IsHealthy: func() bool { return true }}, // Fallback - fast
	}

	// Use status code trigger to ensure failover
	router := NewFailoverRouter(5*time.Second, NewStatusCodeTrigger(500))

	// First call primary (which we need to fail to trigger parallel race)
	var callCount atomic.Int32
	tryProviderWithFail := func(ctx context.Context, p ProviderInfo) (int, error) {
		count := callCount.Add(1)
		if count == 1 {
			// Primary fails immediately
			return 500, errors.New("primary failed")
		}
		return tryProvider(ctx, p)
	}

	result, err := router.SelectWithRetry(context.Background(), providers, tryProviderWithFail)
	if err != nil {
		t.Fatalf("SelectWithRetry() unexpected error: %v", err)
	}

	// The fast provider (Priority 1) should win
	if result.Priority != 1 {
		t.Errorf("SelectWithRetry() Priority = %d, want 1 (fast provider)", result.Priority)
	}
}

func TestFailoverRouter_SelectWithRetry_TimeoutRespected(t *testing.T) {
	t.Parallel()

	shortTimeout := 50 * time.Millisecond
	router := NewFailoverRouter(shortTimeout, NewStatusCodeTrigger(500))

	var callCount atomic.Int32

	tryProvider := func(ctx context.Context, _ ProviderInfo) (int, error) {
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

	providers := []ProviderInfo{
		{Priority: 3, IsHealthy: func() bool { return true }},
		{Priority: 2, IsHealthy: func() bool { return true }},
		{Priority: 1, IsHealthy: func() bool { return true }},
	}

	start := time.Now()
	_, err := router.SelectWithRetry(context.Background(), providers, tryProvider)
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

func TestFailoverRouter_SelectWithRetry_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(100 * time.Millisecond)
	var successCount atomic.Int32
	var errorCount atomic.Int32

	tryProvider := func(_ context.Context, p ProviderInfo) (int, error) {
		if p.Priority%2 == 0 {
			return 200, nil
		}
		return 500, errors.New("odd priority fails")
	}

	providers := []ProviderInfo{
		{Priority: 4, IsHealthy: func() bool { return true }},
		{Priority: 3, IsHealthy: func() bool { return true }},
		{Priority: 2, IsHealthy: func() bool { return true }},
		{Priority: 1, IsHealthy: func() bool { return true }},
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := router.SelectWithRetry(context.Background(), providers, tryProvider)
			if err != nil {
				errorCount.Add(1)
			} else {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	total := successCount.Load() + errorCount.Load()
	if total != 100 {
		t.Errorf("Total operations = %d, want 100", total)
	}

	// Primary (Priority 4) is even, so should succeed
	if successCount.Load() != 100 {
		t.Errorf("Success count = %d, want 100 (primary succeeds)", successCount.Load())
	}
}

func TestFailoverRouter_SortByPriority(t *testing.T) {
	t.Parallel()

	providers := []ProviderInfo{
		{Priority: 1, Weight: 1},
		{Priority: 3, Weight: 3},
		{Priority: 2, Weight: 2},
		{Priority: 5, Weight: 5},
		{Priority: 4, Weight: 4},
	}

	sorted := sortByPriority(providers)

	// Should be in descending order
	expected := []int{5, 4, 3, 2, 1}
	for i, p := range sorted {
		if p.Priority != expected[i] {
			t.Errorf("sortByPriority()[%d].Priority = %d, want %d", i, p.Priority, expected[i])
		}
	}

	// Original should be unmodified
	if providers[0].Priority != 1 {
		t.Error("sortByPriority() modified input slice")
	}
}

func TestFailoverRouter_SortByPriority_StableSort(t *testing.T) {
	t.Parallel()

	// Same priority, different weights - should preserve order
	providers := []ProviderInfo{
		{Priority: 1, Weight: 10},
		{Priority: 1, Weight: 20},
		{Priority: 1, Weight: 30},
	}

	sorted := sortByPriority(providers)

	// Should preserve original order for equal priorities
	expected := []int{10, 20, 30}
	for i, p := range sorted {
		if p.Weight != expected[i] {
			t.Errorf("sortByPriority()[%d].Weight = %d, want %d (stable sort)", i, p.Weight, expected[i])
		}
	}
}

func TestRoutingResult(t *testing.T) {
	t.Parallel()

	provider := ProviderInfo{Priority: 1}
	err := errors.New("test error")

	result := RoutingResult{
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
func TestFailoverRouter_ImplementsProviderRouter(t *testing.T) {
	t.Parallel()

	var _ ProviderRouter = (*FailoverRouter)(nil)
}

func TestFailoverRouter_ParallelRace_AllFail(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(100*time.Millisecond, NewStatusCodeTrigger(500))
	lastErr := errors.New("last error")
	var callCount atomic.Int32

	tryProvider := func(_ context.Context, _ ProviderInfo) (int, error) {
		count := callCount.Add(1)
		if count == 1 {
			return 500, errors.New("primary failed") // Trigger parallel race
		}
		// All parallel attempts fail
		time.Sleep(10 * time.Millisecond) // Small delay
		return 500, lastErr
	}

	providers := []ProviderInfo{
		{Priority: 2, IsHealthy: func() bool { return true }},
		{Priority: 1, IsHealthy: func() bool { return true }},
	}

	_, err := router.SelectWithRetry(context.Background(), providers, tryProvider)
	if err == nil {
		t.Error("SelectWithRetry() should have returned error when all fail")
	}
}

func TestFailoverRouter_ContextCancellation(t *testing.T) {
	t.Parallel()

	router := NewFailoverRouter(5*time.Second, NewStatusCodeTrigger(500))
	var callCount atomic.Int32

	tryProvider := func(ctx context.Context, _ ProviderInfo) (int, error) {
		count := callCount.Add(1)
		if count == 1 {
			return 500, errors.New("primary failed")
		}
		// Block until context canceled
		<-ctx.Done()
		return 0, ctx.Err()
	}

	providers := []ProviderInfo{
		{Priority: 2, IsHealthy: func() bool { return true }},
		{Priority: 1, IsHealthy: func() bool { return true }},
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, _ = router.SelectWithRetry(ctx, providers, tryProvider)
	elapsed := time.Since(start)

	// Should complete quickly after cancellation (regardless of error/success)
	// The key is that cancellation stops the parallel race promptly
	if elapsed > 200*time.Millisecond {
		t.Errorf("SelectWithRetry() took %v, should complete shortly after cancel", elapsed)
	}
}
