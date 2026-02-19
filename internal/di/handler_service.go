package di

import (
	"fmt"
	"net/http"

	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/proxy"
	"github.com/omarluq/cc-relay/internal/router"
)

// HandlerService wraps the HTTP handler.
type HandlerService struct {
	Handler http.Handler
}

// NewProxyHandler creates the HTTP handler with all middleware.
func NewProxyHandler(injector do.Injector) (*HandlerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](injector)
	providerSvc := do.MustInvoke[*ProviderMapService](injector)
	poolSvc := do.MustInvoke[*KeyPoolService](injector)
	poolMapSvc := do.MustInvoke[*KeyPoolMapService](injector)
	routerSvc := do.MustInvoke[*RouterService](injector)
	providerInfoSvc := do.MustInvoke[*ProviderInfoService](injector)
	trackerSvc := do.MustInvoke[*HealthTrackerService](injector)
	sigCacheSvc := do.MustInvoke[*SignatureCacheService](injector)
	concurrencySvc := do.MustInvoke[*ConcurrencyService](injector)

	// Use SetupRoutesWithLiveKeyPools for full hot-reload support:
	// - Live provider info (enabled/disabled, weights, priorities)
	// - Live router (strategy/timeout changes without restart)
	// - Live key pools (newly enabled providers get keys immediately)
	// - Concurrency limiting with hot-reload
	liveRouter := router.NewLiveRouter(routerSvc.GetRouterAsFunc())
	handler, err := proxy.SetupRoutesWithLiveKeyPools(&proxy.RoutesOptions{
		ConfigProvider:     cfgSvc,
		Provider:           providerSvc.GetPrimaryProvider(),
		ProviderInfosFunc:  providerInfoSvc.Get, // Hot-reloadable provider info
		ProviderRouter:     liveRouter,          // Live router for strategy changes
		ProviderKey:        providerSvc.GetPrimaryKey(),
		Pool:               poolSvc.Get(),
		GetProviderPools:   poolMapSvc.GetPools, // Live key pools accessor
		GetProviderKeys:    poolMapSvc.GetKeys,  // Live fallback keys accessor
		GetAllProviders:    providerSvc.GetAllProviders,
		AllProviders:       providerSvc.GetAllProviders(),
		HealthTracker:      trackerSvc.Tracker,
		SignatureCache:     sigCacheSvc.Cache,
		ConcurrencyLimiter: concurrencySvc.Limiter, // Hot-reloadable concurrency limit
		ProviderPools:      nil,
		ProviderKeys:       nil,
		ProviderInfos:      nil,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup proxy handler: %w", err)
	}

	return &HandlerService{Handler: handler}, nil
}
