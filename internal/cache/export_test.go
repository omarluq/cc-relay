package cache

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// Exported for testing in external test package (cache_test).

// NewOlricCacheForTest exports the internal cache constructor for testing.
var NewOlricCacheForTest = newOlricCache

// NewRistrettoCacheForTest exports the ristretto cache constructor for testing.
var NewRistrettoCacheForTest = newRistrettoCache

// NewNoopCacheForTest exports the noop cache constructor for testing.
var NewNoopCacheForTest = newNoopCache

// ParseBindAddrForTest exports parseBindAddr for testing.
var ParseBindAddrForTest = parseBindAddr

// OlricCacheT exports the internal cache type for testing.
type OlricCacheT = olricCache

// RistrettoCacheT exports the internal cache type for testing.
type RistrettoCacheT = ristrettoCache

// NoopCacheT exports the internal noop cache type for testing.
type NoopCacheT = noopCache

// ContainsString checks if a string contains a substring (for testing).
func ContainsString(str, substr string) bool {
	return len(str) >= len(substr) && containsStr(str, substr)
}

// containsStr searches for substr in str.
func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// IgnoreCacheErr exports ignoreCacheErr for testing.
var IgnoreCacheErr = ignoreCacheErr

// NewOlricCacheCtx is a wrapper that creates an olric cache for testing.
func NewOlricCacheCtx(ctx context.Context, cfg *OlricConfig) (Cache, error) {
	return newOlricCache(ctx, cfg)
}

// RistrettoWait calls Wait() on the underlying ristretto cache for test synchronization.
func RistrettoWait(cache *ristrettoCache) {
	cache.cache.Wait()
}

// NewRistrettoCacheWithLogger creates a ristretto cache using a specific logger,
// avoiding the global cache.Logger for test isolation.
func NewRistrettoCacheWithLogger(cfg RistrettoConfig, l *zerolog.Logger) (*ristrettoCache, error) {
	return newRistrettoCacheWithLog(cfg, l)
}

// NewNoopCacheWithLogger creates a noop cache using a specific logger,
// avoiding the global cache.Logger for test isolation.
func NewNoopCacheWithLogger(l *zerolog.Logger) *noopCache {
	return newNoopCacheWithLog(l)
}

// NewForTest creates a cache via the factory using a specific logger,
// avoiding the global cache.Logger for test isolation.
func NewForTest(ctx context.Context, cfg *Config, testLog *zerolog.Logger) (Cache, error) {
	log := testLog.With().Str("component", "cache_factory").Logger()
	start := time.Now()

	if err := cfg.Validate(); err != nil {
		log.Debug().Err(err).Str("mode", string(cfg.Mode)).Msg("cache factory: validation failed")
		return nil, err
	}

	log.Info().
		Str("mode", string(cfg.Mode)).
		Msg("cache factory: initializing backend")

	var result Cache
	var err error

	switch cfg.Mode {
	case ModeSingle:
		result, err = newRistrettoCacheWithLog(cfg.Ristretto, testLog)
	case ModeHA:
		// For Olric, we need to use the global logger approach
		// since newOlricCache uses logger() internally
		loggerMu.Lock()
		prev := Logger
		Logger = *testLog
		loggerMu.Unlock()
		result, err = newOlricCache(ctx, &cfg.Olric)
		loggerMu.Lock()
		Logger = prev
		loggerMu.Unlock()
	case ModeDisabled:
		result = newNoopCacheWithLog(testLog)
	default:
		return nil, fmt.Errorf("cache: unknown mode %q", cfg.Mode)
	}

	if err != nil {
		log.Error().Err(err).Str("mode", string(cfg.Mode)).Msg("cache factory: backend initialization failed")
		return nil, err
	}

	log.Info().
		Str("mode", string(cfg.Mode)).
		Dur("init_time", time.Since(start)).
		Msg("cache factory: backend initialized")

	return result, nil
}

// NewTestLogger creates a test logger at the given level, returning
// the buffer (for inspecting output) and the logger pointer.
func NewTestLogger(level zerolog.Level) (*bytes.Buffer, *zerolog.Logger) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(level)
	return &buf, &logger
}

// DefaultTestRistrettoConfig returns the standard test Ristretto configuration
// used across most tests, reducing duplication.
func DefaultTestRistrettoConfig() RistrettoConfig {
	return RistrettoConfig{
		NumCounters: 100_000,
		MaxCost:     10 << 20,
		BufferItems: 64,
	}
}

// SmallTestRistrettoConfig returns a smaller test Ristretto configuration
// for lightweight tests.
func SmallTestRistrettoConfig() RistrettoConfig {
	return RistrettoConfig{
		NumCounters: 1000,
		MaxCost:     1 << 20,
		BufferItems: 64,
	}
}

// ZeroOlricConfig returns a zero-value OlricConfig for use in factory tests
// that only exercise non-HA paths.
func ZeroOlricConfig() OlricConfig {
	return OlricConfig{
		DMapName:          "",
		BindAddr:          "",
		Environment:       "",
		Addresses:         nil,
		Peers:             nil,
		ReplicaCount:      0,
		ReadQuorum:        0,
		WriteQuorum:       0,
		LeaveTimeout:      0,
		MemberCountQuorum: 0,
		Embedded:          false,
	}
}

// DefaultTestOlricConfig returns the standard embedded Olric configuration
// used across most integration tests, reducing duplication.
// Only DMapName and BindAddr need to be parameterized for test isolation.
func DefaultTestOlricConfig(dmapName, bindAddr string) OlricConfig {
	return OlricConfig{
		DMapName:          dmapName,
		BindAddr:          bindAddr,
		Environment:       "",
		Addresses:         nil,
		Peers:             nil,
		ReplicaCount:      0,
		ReadQuorum:        0,
		WriteQuorum:       0,
		LeaveTimeout:      0,
		MemberCountQuorum: 0,
		Embedded:          true,
	}
}

// ZeroRistrettoConfig returns a zero-value RistrettoConfig for factory tests.
func ZeroRistrettoConfig() RistrettoConfig {
	return RistrettoConfig{
		NumCounters: 0,
		MaxCost:     0,
		BufferItems: 0,
	}
}

// NewTestRistrettoCacheWithCleanup creates a ristretto cache with the default
// test config and registers cleanup with t.Cleanup.
func NewTestRistrettoCacheWithCleanup(t *testing.T, testLogger *zerolog.Logger) *ristrettoCache {
	t.Helper()
	cache, err := newRistrettoCacheWithLog(DefaultTestRistrettoConfig(), testLogger)
	if err != nil {
		t.Fatalf("NewRistrettoCacheWithLogger failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := cache.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})
	return cache
}

// NewTestNoopCacheWithCleanup creates a noop cache with the given logger
// and registers cleanup with t.Cleanup.
func NewTestNoopCacheWithCleanup(t *testing.T, testLogger *zerolog.Logger) *noopCache {
	t.Helper()
	cache := newNoopCacheWithLog(testLogger)
	t.Cleanup(func() {
		if closeErr := cache.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})
	return cache
}

