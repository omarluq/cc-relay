package cache

import (
	"context"
	"fmt"
	"time"
)

// New creates a new Cache based on the configuration.
// It returns an error if the configuration is invalid or if the cache
// backend fails to initialize.
//
// The context is used for initialization of distributed caches (ModeHA).
// For local caches (ModeSingle, ModeDisabled), the context is not used
// but is included for API consistency.
//
// Example:
//
//	cfg := cache.Config{
//		Mode: cache.ModeSingle,
//		Ristretto: cache.RistrettoConfig{
//			NumCounters: 1e6,
//			MaxCost:     100 << 20, // 100 MB
//			BufferItems: 64,
//		},
//	}
//	c, err := cache.New(ctx, cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer c.Close()
func New(ctx context.Context, cfg *Config) (Cache, error) {
	log := logger().With().Str("component", "cache_factory").Logger()
	start := time.Now()

	if err := cfg.Validate(); err != nil {
		log.Debug().Err(err).Str("mode", string(cfg.Mode)).Msg("cache factory: validation failed")
		return nil, err
	}

	log.Info().
		Str("mode", string(cfg.Mode)).
		Msg("cache factory: initializing backend")

	var cache Cache
	var err error

	switch cfg.Mode {
	case ModeSingle:
		cache, err = newRistrettoCache(cfg.Ristretto)
	case ModeHA:
		cache, err = newOlricCache(ctx, &cfg.Olric)
	case ModeDisabled:
		cache = newNoopCache()
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

	return cache, nil
}
