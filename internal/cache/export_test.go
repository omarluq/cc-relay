package cache

import (
	"context"
	"fmt"
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
