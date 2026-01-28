package di

import (
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/router"
)

// ProviderInfoService holds live provider routing information with atomic swap support.
// Provider info (enabled/disabled, weights, priorities) is rebuilt on config reload
// and atomically swapped for thread-safe access without mutex overhead.
type ProviderInfoService struct {
	// infos holds the current provider info slice via atomic pointer
	infos atomic.Pointer[[]router.ProviderInfo]

	// cfgSvc provides access to current config for rebuilding
	cfgSvc *ConfigService

	// providerSvc gives access to provider instances
	providerSvc *ProviderMapService

	// trackerSvc provides health check functions
	trackerSvc *HealthTrackerService
}

// Get returns the current provider info slice (lock-free read).
// Returns a shallow copy to prevent callers from mutating the internal slice.
func (s *ProviderInfoService) Get() []router.ProviderInfo {
	ptr := s.infos.Load()
	if ptr == nil {
		return nil
	}
	// Return shallow copy (append to nil) to prevent mutation of internal slice
	return append([]router.ProviderInfo(nil), (*ptr)...)
}

// Rebuild rebuilds the provider info slice from current config.
// This should be called on config reload to update provider routing inputs.
func (s *ProviderInfoService) Rebuild() {
	cfg := s.cfgSvc.Get()
	s.RebuildFrom(cfg)
}

// RebuildFrom rebuilds the provider info slice from the given config.
// This is called from reload callbacks to ensure we use the fresh config
// rather than racing with the atomic config swap.
// Uses the live provider map to pick up newly enabled providers.
func (s *ProviderInfoService) RebuildFrom(cfg *config.Config) {
	var providerInfos []router.ProviderInfo

	// Use live provider map to pick up newly enabled providers
	providerMap := s.providerSvc.GetProviders()

	for idx := range cfg.Providers {
		pc := &cfg.Providers[idx]
		if !pc.Enabled {
			continue
		}

		prov, ok := providerMap[pc.Name]
		if !ok {
			continue
		}

		// Get weight and priority from first key (provider-level defaults)
		var weight, priority int
		if len(pc.Keys) > 0 {
			weight = pc.Keys[0].Weight
			priority = pc.Keys[0].Priority
		}

		// Wire IsHealthy from tracker
		providerName := pc.Name
		providerInfos = append(providerInfos, router.ProviderInfo{
			Provider:  prov,
			Weight:    weight,
			Priority:  priority,
			IsHealthy: s.trackerSvc.Tracker.IsHealthyFunc(providerName),
		})
	}

	s.infos.Store(&providerInfos)
}

// StartWatching begins watching config changes for provider info updates.
// Registers a callback with the config watcher to rebuild provider info on reload.
func (s *ProviderInfoService) StartWatching() {
	if s.cfgSvc.watcher == nil {
		return
	}

	// Register callback to rebuild provider info on config reload.
	// Important: We rebuild from the newCfg passed to the callback, not from
	// cfgSvc.Get(), to ensure we use the freshly loaded config regardless of
	// callback registration order.
	s.cfgSvc.watcher.OnReload(func(newCfg *config.Config) error {
		s.RebuildFrom(newCfg)
		log.Info().Msg("provider info rebuilt after config reload")
		return nil
	})
}

// NewProviderInfo creates the provider info service with hot-reload support.
// Provider info (enabled/disabled, weights, priorities) is rebuilt on config reload.
func NewProviderInfo(i do.Injector) (*ProviderInfoService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	providerSvc := do.MustInvoke[*ProviderMapService](i)
	trackerSvc := do.MustInvoke[*HealthTrackerService](i)

	svc := &ProviderInfoService{
		cfgSvc:      cfgSvc,
		providerSvc: providerSvc,
		trackerSvc:  trackerSvc,
	}

	// Build initial provider info
	svc.Rebuild()

	// Start watching for config changes
	svc.StartWatching()

	return svc, nil
}
