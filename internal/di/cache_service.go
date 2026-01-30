package di

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/cache"
)

// CacheService wraps the cache implementation.
type CacheService struct {
	Cache cache.Cache
}

// NewCache creates the cache based on configuration.
func NewCache(i do.Injector) (*CacheService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)

	// Use a background context with timeout for cache initialization
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := cache.New(ctx, &cfgSvc.Config.Cache)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	return &CacheService{Cache: c}, nil
}

// Shutdown implements do.Shutdowner for graceful cache cleanup.
func (c *CacheService) Shutdown() error {
	if c.Cache != nil {
		return c.Cache.Close()
	}
	return nil
}
