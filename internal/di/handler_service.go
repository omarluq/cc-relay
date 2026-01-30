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
func NewProxyHandler(i do.Injector) (*HandlerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	providerSvc := do.MustInvoke[*ProviderMapService](i)
	poolSvc := do.MustInvoke[*KeyPoolService](i)
	poolMapSvc := do.MustInvoke[*KeyPoolMapService](i)
	routerSvc := do.MustInvoke[*RouterService](i)
	providerInfoSvc := do.MustInvoke[*ProviderInfoService](i)
	trackerSvc := do.MustInvoke[*HealthTrackerService](i)
	sigCacheSvc := do.MustInvoke[*SignatureCacheService](i)

	// Use SetupRoutesWithLiveKeyPools for full hot-reload support:
	// - Live provider info (enabled/disabled, weights, priorities)
	// - Live router (strategy/timeout changes without restart)
	// - Live key pools (newly enabled providers get keys immediately)
	liveRouter := router.NewLiveRouter(routerSvc.GetRouterAsFunc())
	handler, err := proxy.SetupRoutesWithLiveKeyPools(&proxy.RoutesOptions{
		ConfigProvider:    cfgSvc,
		Provider:          providerSvc.GetPrimaryProvider(),
		ProviderInfosFunc: providerInfoSvc.Get, // Hot-reloadable provider info
		ProviderRouter:    liveRouter,          // Live router for strategy changes
		ProviderKey:       providerSvc.GetPrimaryKey(),
		Pool:              poolSvc.Get(),
		GetProviderPools:  poolMapSvc.GetPools, // Live key pools accessor
		GetProviderKeys:   poolMapSvc.GetKeys,  // Live fallback keys accessor
		AllProviders:      providerSvc.GetAllProviders(),
		HealthTracker:     trackerSvc.Tracker,
		SignatureCache:    sigCacheSvc.Cache,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup proxy handler: %w", err)
	}

	return &HandlerService{Handler: handler}, nil
}
