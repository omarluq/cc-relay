package di

import (
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/router"
)

// RouterService wraps the provider router for DI.
// It provides hot-reloadable router access with caching to preserve router state.
//
// Router instances are cached and only rebuilt when strategy or timeout changes.
// This preserves state for stateful routers (round_robin, weighted_round_robin)
// while still allowing config changes to take effect without restart.
type RouterService struct {
	cfgSvc *ConfigService

	// Cached router with atomic swap support
	router atomic.Pointer[routerCacheEntry]
}

// routerCacheEntry holds a cached router with its configuration key.
type routerCacheEntry struct {
	router   router.ProviderRouter
	strategy string
	timeout  time.Duration
}

// GetRouter returns the current router, using cache when config unchanged.
// This method is safe for concurrent use and preserves router state.
func (s *RouterService) GetRouter() router.ProviderRouter {
	cfg := s.cfgSvc.Get()
	strategy := cfg.Routing.GetEffectiveStrategy()
	timeout := cfg.Routing.GetFailoverTimeoutOption().OrElse(5 * time.Second)

	// Check cache for existing router with same config
	cached := s.router.Load()
	if cached != nil && cached.strategy == strategy && cached.timeout == timeout {
		return cached.router
	}

	// Create new router for updated config
	r, err := router.NewRouter(strategy, timeout)
	if err != nil {
		// Fallback to failover if strategy is invalid
		var fallbackErr error
		r, fallbackErr = router.NewRouter(router.StrategyFailover, timeout)
		// If even failover fails, we have a configuration problem
		if fallbackErr != nil {
			// Return a failover router with default timeout as last resort
			// At this point we log the error but continue with a known-good fallback
			log.Error().Err(fallbackErr).Msg("failed to create failover router, using default")
			r, err = router.NewRouter(router.StrategyFailover, 5*time.Second)
			if err != nil {
				// This should never happen unless there's a code bug
				panic("router: failed to create default failover router")
			}
		}
	}

	// Atomically store new router (racing updates may overwrite, last wins)
	newEntry := &routerCacheEntry{
		router:   r,
		strategy: strategy,
		timeout:  timeout,
	}
	s.router.Store(newEntry)

	return r
}

// GetRouterFunc returns a function that fetches the current router.
// This is used with LiveRouter for per-request router access.
func (s *RouterService) GetRouterFunc() router.ProviderRouterFunc {
	return s.GetRouter
}

// GetRouterAsFunc returns the router getter as a ProviderRouterFunc directly.
// This is a convenience wrapper for passing to NewLiveRouter.
// Delegates to GetRouterFunc for deduplication.
func (s *RouterService) GetRouterAsFunc() router.ProviderRouterFunc {
	return s.GetRouterFunc()
}

// NewRouter creates the provider router service with hot-reload support.
// The router is created dynamically per-request based on current config.
func NewRouter(i do.Injector) (*RouterService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	return &RouterService{cfgSvc: cfgSvc}, nil
}
