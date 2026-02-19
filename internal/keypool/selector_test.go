package keypool_test

import (
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewKeyMetadata verifies key metadata initialization.
func TestNewKeyMetadata(t *testing.T) {
	t.Parallel()
	sampleKeyInput := "sk-ant-test-key-12345" // #nosec G101 -- test data
	rpm := 50
	itpm := 30000
	otpm := 30000

	key := keypool.NewKeyMetadata(sampleKeyInput, rpm, itpm, otpm)

	assert.Equal(t, sampleKeyInput, key.APIKey)
	assert.Equal(t, rpm, key.RPMLimit)
	assert.Equal(t, itpm, key.ITPMLimit)
	assert.Equal(t, otpm, key.OTPMLimit)
	assert.Equal(t, rpm, key.RPMRemaining, "Should start at full capacity")
	assert.Equal(t, itpm, key.ITPMRemaining)
	assert.Equal(t, otpm, key.OTPMRemaining)
	assert.True(t, key.Healthy, "Should be healthy")
	assert.Equal(t, 1, key.Priority, "Should have normal priority")
	assert.Len(t, key.ID, 8, "ID should be 8 character hex string")
}

// capacityScoreTestCase defines a test case for GetCapacityScore.
var capacityScoreTestCases = []struct {
	name          string
	rpm           int
	rpmRemaining  int
	itpm          int
	itpmRemaining int
	otpm          int
	otpmRemaining int
	healthy       bool
	cooldown      bool
	wantScore     float64
}{
	{
		name:          "full capacity",
		rpm:           50,
		rpmRemaining:  50,
		itpm:          30000,
		itpmRemaining: 30000,
		otpm:          30000,
		otpmRemaining: 30000,
		healthy:       true,
		cooldown:      false,
		wantScore:     1.0,
	},
	{
		name:          "half capacity",
		rpm:           50,
		rpmRemaining:  25,
		itpm:          30000,
		itpmRemaining: 15000,
		otpm:          30000,
		otpmRemaining: 15000,
		healthy:       true,
		cooldown:      false,
		wantScore:     0.5,
	},
	{
		name:          "quarter capacity",
		rpm:           50,
		rpmRemaining:  12,
		itpm:          30000,
		itpmRemaining: 7500,
		otpm:          30000,
		otpmRemaining: 7500,
		healthy:       true,
		cooldown:      false,
		wantScore:     0.25,
	},
	{
		name:          "unhealthy key",
		rpm:           50,
		rpmRemaining:  50,
		itpm:          30000,
		itpmRemaining: 30000,
		otpm:          30000,
		otpmRemaining: 30000,
		healthy:       false,
		cooldown:      false,
		wantScore:     0.0,
	},
	{
		name:          "key in cooldown",
		rpm:           50,
		rpmRemaining:  50,
		itpm:          30000,
		itpmRemaining: 30000,
		otpm:          30000,
		otpmRemaining: 30000,
		healthy:       true,
		cooldown:      true,
		wantScore:     0.0,
	},
}

// TestGetCapacityScore verifies capacity score calculation.
func TestGetCapacityScore(t *testing.T) {
	t.Parallel()
	for _, testCase := range capacityScoreTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			key := keypool.NewKeyMetadata("test-key", testCase.rpm, testCase.itpm, testCase.otpm)
			key.RPMRemaining = testCase.rpmRemaining
			key.ITPMRemaining = testCase.itpmRemaining
			key.OTPMRemaining = testCase.otpmRemaining
			key.Healthy = testCase.healthy

			if testCase.cooldown {
				key.CooldownUntil = time.Now().Add(1 * time.Minute)
			}

			score := key.GetCapacityScore()
			// Allow small floating point error tolerance
			epsilon := 0.01
			if diff := score - testCase.wantScore; diff < -epsilon || diff > epsilon {
				t.Errorf("GetCapacityScore() = %f, want %f (diff=%f)", score, testCase.wantScore, diff)
			}
		})
	}
}

// TestIsAvailable verifies availability checks.
func TestIsAvailable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cooldownUntil time.Time
		name          string
		healthy       bool
		want          bool
	}{
		{
			name:          "healthy and no cooldown",
			healthy:       true,
			cooldownUntil: time.Time{},
			want:          true,
		},
		{
			name:          "unhealthy",
			healthy:       false,
			cooldownUntil: time.Time{},
			want:          false,
		},
		{
			name:          "in cooldown",
			healthy:       true,
			cooldownUntil: time.Now().Add(1 * time.Minute),
			want:          false,
		},
		{
			name:          "cooldown expired",
			healthy:       true,
			cooldownUntil: time.Now().Add(-1 * time.Minute),
			want:          true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			key := keypool.NewKeyMetadata("test-key", 50, 30000, 30000)
			key.Healthy = testCase.healthy
			key.CooldownUntil = testCase.cooldownUntil

			got := key.IsAvailable()
			if got != testCase.want {
				t.Errorf("IsAvailable() = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestUpdateFromHeaders verifies response header parsing.
func TestUpdateFromHeaders(t *testing.T) {
	t.Parallel()
	key := keypool.NewKeyMetadata("test-key", 50, 30000, 30000)

	headers := http.Header{}
	headers.Set("anthropic-ratelimit-requests-limit", "100")
	headers.Set("anthropic-ratelimit-requests-remaining", "75")
	headers.Set("anthropic-ratelimit-requests-reset", "2026-01-21T19:42:00Z")
	headers.Set("anthropic-ratelimit-input-tokens-limit", "40000")
	headers.Set("anthropic-ratelimit-input-tokens-remaining", "35000")
	headers.Set("anthropic-ratelimit-input-tokens-reset", "2026-01-21T19:42:00Z")
	headers.Set("anthropic-ratelimit-output-tokens-limit", "40000")
	headers.Set("anthropic-ratelimit-output-tokens-remaining", "35000")
	headers.Set("anthropic-ratelimit-output-tokens-reset", "2026-01-21T19:42:00Z")

	err := key.UpdateFromHeaders(headers)
	require.NoError(t, err)

	assert.Equal(t, 100, key.RPMLimit)
	assert.Equal(t, 75, key.RPMRemaining)
	assert.Equal(t, 40000, key.ITPMLimit)
	assert.Equal(t, 35000, key.ITPMRemaining)
	assert.Equal(t, 40000, key.OTPMLimit)
	assert.Equal(t, 35000, key.OTPMRemaining)

	expectedReset, parseErr := time.Parse(time.RFC3339, "2026-01-21T19:42:00Z")
	require.NoError(t, parseErr)
	assert.True(t, key.RPMResetAt.Equal(expectedReset))
}

// TestUpdateFromHeadersPartial verifies handling of missing headers.
func TestUpdateFromHeadersPartial(t *testing.T) {
	t.Parallel()
	key := keypool.NewKeyMetadata("test-key", 50, 30000, 30000)

	// Set only some headers
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-requests-remaining", "42")

	err := key.UpdateFromHeaders(headers)
	require.NoError(t, err)

	assert.Equal(t, 50, key.RPMLimit, "Original limit should be unchanged")
	assert.Equal(t, 42, key.RPMRemaining, "Remaining should be updated")
}

// TestUpdateFromHeadersInvalid verifies handling of invalid header values.
func TestUpdateFromHeadersInvalid(t *testing.T) {
	t.Parallel()
	key := keypool.NewKeyMetadata("test-key", 50, 30000, 30000)

	headers := http.Header{}
	headers.Set("anthropic-ratelimit-requests-limit", "invalid")
	headers.Set("anthropic-ratelimit-requests-remaining", "-10")
	headers.Set("anthropic-ratelimit-requests-reset", "not-a-time")

	err := key.UpdateFromHeaders(headers)
	require.NoError(t, err)

	assert.Equal(t, 50, key.RPMLimit, "Invalid values should be ignored")
	assert.Equal(t, 50, key.RPMRemaining, "Negative values should be ignored")
}

// TestSetCooldown verifies cooldown setting.
func TestSetCooldown(t *testing.T) {
	t.Parallel()
	key := keypool.NewKeyMetadata("test-key", 50, 30000, 30000)

	cooldownUntil := time.Now().Add(5 * time.Minute)
	key.SetCooldown(cooldownUntil)

	assert.False(t, key.IsAvailable(), "Should not be available during cooldown")

	key.TestRLock()
	assert.True(t, key.CooldownUntil.Equal(cooldownUntil))
	key.TestRUnlock()
}

// TestMarkUnhealthy verifies unhealthy marking.
func TestMarkUnhealthy(t *testing.T) {
	t.Parallel()
	key := keypool.NewKeyMetadata("test-key", 50, 30000, 30000)

	testErr := http.ErrServerClosed
	key.MarkUnhealthy(testErr)

	assert.False(t, key.Healthy, "Should be unhealthy after MarkUnhealthy")
	assert.True(t, errors.Is(key.LastError, testErr))
	assert.False(t, key.LastErrorAt.IsZero(), "LastErrorAt should be set")
	assert.False(t, key.IsAvailable(), "Should not be available when unhealthy")
}

// TestMarkHealthy verifies health recovery.
func TestMarkHealthy(t *testing.T) {
	t.Parallel()
	key := keypool.NewKeyMetadata("test-key", 50, 30000, 30000)

	key.MarkUnhealthy(http.ErrServerClosed)
	key.MarkHealthy()

	assert.True(t, key.Healthy, "Should be healthy after MarkHealthy")
	assert.Nil(t, key.LastError, "LastError should be nil after MarkHealthy")
	assert.True(t, key.IsAvailable(), "Should be available after MarkHealthy")
}

// TestLeastLoadedSelector verifies least-loaded selection strategy.
func TestLeastLoadedSelector(t *testing.T) {
	t.Parallel()
	selector := keypool.NewLeastLoadedSelector()

	assert.Equal(t, keypool.StrategyLeastLoaded, selector.Name())

	// Create keys with different capacity levels
	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000), // Full capacity
		keypool.NewKeyMetadata("key2", 50, 30000, 30000), // Will set to half
		keypool.NewKeyMetadata("key3", 50, 30000, 30000), // Will set to quarter
	}

	// Set different remaining capacities
	keys[1].RPMRemaining = 25
	keys[1].ITPMRemaining = 15000
	keys[1].OTPMRemaining = 15000

	keys[2].RPMRemaining = 12
	keys[2].ITPMRemaining = 7500
	keys[2].OTPMRemaining = 7500

	// Should select key with most capacity (key1)
	selected, err := selector.Select(keys)
	require.NoError(t, err)
	assert.Equal(t, "key1", selected.APIKey)
}

// TestLeastLoadedSelectorSkipsUnhealthy verifies unhealthy keys are skipped.
func TestLeastLoadedSelectorSkipsUnhealthy(t *testing.T) {
	t.Parallel()
	selector := keypool.NewLeastLoadedSelector()

	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
	}

	keys[0].MarkUnhealthy(http.ErrServerClosed)
	keys[1].RPMRemaining = 10
	keys[1].ITPMRemaining = 5000
	keys[1].OTPMRemaining = 5000

	selected, err := selector.Select(keys)
	require.NoError(t, err)
	assert.Equal(t, "key2", selected.APIKey)
}

// TestLeastLoadedSelectorAllExhausted verifies error when all keys unavailable.
func TestLeastLoadedSelectorAllExhausted(t *testing.T) {
	t.Parallel()
	selector := keypool.NewLeastLoadedSelector()

	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
	}

	keys[0].MarkUnhealthy(http.ErrServerClosed)
	keys[1].MarkUnhealthy(http.ErrServerClosed)

	_, err := selector.Select(keys)
	assert.ErrorIs(t, err, keypool.ErrAllKeysExhausted)
}

// TestRoundRobinSelector verifies round-robin selection strategy.
func TestRoundRobinSelector(t *testing.T) {
	t.Parallel()
	selector := keypool.NewRoundRobinSelector()

	assert.Equal(t, keypool.StrategyRoundRobin, selector.Name())

	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
		keypool.NewKeyMetadata("key3", 50, 30000, 30000),
	}

	selected1, err := selector.Select(keys)
	require.NoError(t, err)

	selected2, err := selector.Select(keys)
	require.NoError(t, err)

	selected3, err := selector.Select(keys)
	require.NoError(t, err)

	selected4, err := selector.Select(keys)
	require.NoError(t, err)

	selectedKeys := []string{
		selected1.APIKey,
		selected2.APIKey,
		selected3.APIKey,
		selected4.APIKey,
	}

	// First three should be unique
	seen := make(map[string]bool)
	for idx := range 3 {
		assert.False(t, seen[selectedKeys[idx]], "Selected duplicate key %q in first 3 selections", selectedKeys[idx])
		seen[selectedKeys[idx]] = true
	}

	// Fourth should match first (wrap around)
	assert.Equal(t, selectedKeys[0], selectedKeys[3], "Fourth selection should wrap around to first")
}

// TestRoundRobinSelectorSkipsUnavailable verifies skipping unavailable keys.
func TestRoundRobinSelectorSkipsUnavailable(t *testing.T) {
	t.Parallel()
	selector := keypool.NewRoundRobinSelector()

	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
		keypool.NewKeyMetadata("key3", 50, 30000, 30000),
	}

	keys[1].SetCooldown(time.Now().Add(5 * time.Minute))

	for iteration := range 4 {
		selected, err := selector.Select(keys)
		require.NoError(t, err)
		assert.NotEqual(t, "key2", selected.APIKey, "Selected unavailable key2 on iteration %d", iteration)
	}
}

// TestRoundRobinSelectorConcurrent verifies thread-safety.
func TestRoundRobinSelectorConcurrent(t *testing.T) {
	t.Parallel()
	selector := keypool.NewRoundRobinSelector()

	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
		keypool.NewKeyMetadata("key3", 50, 30000, 30000),
	}

	var waitGroup sync.WaitGroup
	numGoroutines := 100
	selectionsPerGoroutine := 10

	waitGroup.Add(numGoroutines)
	for range numGoroutines {
		go func() {
			defer waitGroup.Done()
			for range selectionsPerGoroutine {
				_, err := selector.Select(keys)
				if err != nil {
					t.Errorf("Concurrent Select() error = %v", err)
				}
			}
		}()
	}

	waitGroup.Wait()
}

// TestRoundRobinSelectorAllExhausted verifies error when all keys unavailable.
func TestRoundRobinSelectorAllExhausted(t *testing.T) {
	t.Parallel()
	selector := keypool.NewRoundRobinSelector()

	keys := []*keypool.KeyMetadata{
		keypool.NewKeyMetadata("key1", 50, 30000, 30000),
		keypool.NewKeyMetadata("key2", 50, 30000, 30000),
	}

	keys[0].MarkUnhealthy(http.ErrServerClosed)
	keys[1].MarkUnhealthy(http.ErrServerClosed)

	_, err := selector.Select(keys)
	assert.ErrorIs(t, err, keypool.ErrAllKeysExhausted)
}

// TestNewSelector verifies selector factory.
func TestNewSelector(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		strategy string
		wantType string
		wantErr  bool
	}{
		{
			name:     "least_loaded",
			strategy: keypool.StrategyLeastLoaded,
			wantType: keypool.StrategyLeastLoaded,
			wantErr:  false,
		},
		{
			name:     "round_robin",
			strategy: keypool.StrategyRoundRobin,
			wantType: keypool.StrategyRoundRobin,
			wantErr:  false,
		},
		{
			name:     "random",
			strategy: keypool.StrategyRandom,
			wantType: keypool.StrategyRandom,
			wantErr:  false,
		},
		{
			name:     "weighted",
			strategy: keypool.StrategyWeighted,
			wantType: keypool.StrategyWeighted,
			wantErr:  false,
		},
		{
			name:     "empty defaults to least_loaded",
			strategy: "",
			wantType: keypool.StrategyLeastLoaded,
			wantErr:  false,
		},
		{
			name:     "unknown strategy",
			strategy: "unknown",
			wantType: "",
			wantErr:  true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			selector, err := keypool.NewSelector(testCase.strategy)
			if (err != nil) != testCase.wantErr {
				t.Errorf("keypool.NewSelector() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}

			if err == nil {
				assert.Equal(t, testCase.wantType, selector.Name())
			}
		})
	}
}

// createTestKeys creates test keys with specific capacities.
func createTestKeys(capacities []float64) []*keypool.KeyMetadata {
	keys := make([]*keypool.KeyMetadata, len(capacities))
	for keyIdx, capacity := range capacities {
		key := keypool.NewKeyMetadata("key"+string(rune('1'+keyIdx)), 100, 60000, 60000)
		// Set remaining based on capacity percentage
		key.RPMRemaining = int(float64(key.RPMLimit) * capacity)
		key.ITPMRemaining = int(float64(key.ITPMLimit) * capacity)
		key.OTPMRemaining = int(float64(key.OTPMLimit) * capacity)
		keys[keyIdx] = key
	}
	return keys
}

// TestLeastLoadedSelectorVariousCapacities tests selection with various capacity levels.
func TestLeastLoadedSelectorVariousCapacities(t *testing.T) {
	t.Parallel()
	selector := keypool.NewLeastLoadedSelector()

	// Create keys with 100%, 75%, 50%, 25% capacity
	keys := createTestKeys([]float64{1.0, 0.75, 0.5, 0.25})

	// Should always select the first key (100% capacity)
	for iteration := range 5 {
		selected, err := selector.Select(keys)
		require.NoError(t, err)
		assert.Equal(t, "key1", selected.APIKey,
			"Select() iteration %d selected %q, want key1", iteration, selected.APIKey)
	}
}
