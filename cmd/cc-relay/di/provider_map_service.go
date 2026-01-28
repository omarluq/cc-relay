package di

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
)

// providerMapData holds the provider map data for atomic swap.
type providerMapData struct {
	PrimaryProvider providers.Provider
	Providers       map[string]providers.Provider
	PrimaryKey      string
	AllProviders    []providers.Provider
}

// ProviderMapService wraps the map of providers with hot-reload support.
// Providers are rebuilt on config reload to support enabling/disabling providers dynamically.
type ProviderMapService struct {
	data   atomic.Pointer[providerMapData]
	cfgSvc *ConfigService

	// For backward compatibility
	PrimaryProvider providers.Provider
	Providers       map[string]providers.Provider
	PrimaryKey      string
	AllProviders    []providers.Provider
}

// GetPrimaryProvider returns the current primary provider (live, hot-reload aware).
func (s *ProviderMapService) GetPrimaryProvider() providers.Provider {
	d := s.data.Load()
	if d == nil {
		return s.PrimaryProvider
	}
	return d.PrimaryProvider
}

// GetPrimaryKey returns the current primary provider key (live, hot-reload aware).
func (s *ProviderMapService) GetPrimaryKey() string {
	d := s.data.Load()
	if d == nil {
		return s.PrimaryKey
	}
	return d.PrimaryKey
}

// GetProviders returns the current provider map (live, hot-reload aware).
func (s *ProviderMapService) GetProviders() map[string]providers.Provider {
	d := s.data.Load()
	if d == nil {
		return s.Providers // Fallback to legacy field
	}
	return d.Providers
}

// GetAllProviders returns the current all providers slice (live, hot-reload aware).
func (s *ProviderMapService) GetAllProviders() []providers.Provider {
	d := s.data.Load()
	if d == nil {
		return s.AllProviders // Fallback to legacy field
	}
	return d.AllProviders
}

// GetProvider returns a provider by name (live, hot-reload aware).
func (s *ProviderMapService) GetProvider(name string) (providers.Provider, bool) {
	providersMap := s.GetProviders()
	if providersMap == nil {
		return nil, false
	}
	prov, ok := providersMap[name]
	return prov, ok
}

// RebuildFrom rebuilds the provider map from the given config.
// Called from reload callbacks to create providers for newly enabled ones.
// Reuses existing providers when possible to preserve state.
func (s *ProviderMapService) RebuildFrom(cfg *config.Config) error {
	ctx := context.Background()

	providerMap := make(map[string]providers.Provider)
	var allProviders []providers.Provider
	var primaryProvider providers.Provider
	var primaryKey string

	for idx := range cfg.Providers {
		p := &cfg.Providers[idx]
		if !p.Enabled {
			continue
		}

		prov, err := createProvider(ctx, p)
		if errors.Is(err, ErrUnknownProviderType) {
			log.Warn().Str("provider", p.Name).Str("type", p.Type).Msg("skipping unknown provider type on reload")
			continue // Skip unknown provider types
		}
		if err != nil {
			log.Error().Err(err).Str("provider", p.Name).Msg("failed to create provider on reload")
			continue // Log and skip, don't fail the entire reload
		}

		providerMap[p.Name] = prov
		allProviders = append(allProviders, prov)

		if primaryProvider == nil {
			primaryProvider = prov
			if len(p.Keys) > 0 {
				primaryKey = p.Keys[0].Key
			}
		}
	}

	if primaryProvider == nil {
		// Keep using current providers if no enabled providers in new config
		log.Warn().Msg("no enabled providers in new config, keeping current providers")
		return nil
	}

	s.data.Store(&providerMapData{
		PrimaryProvider: primaryProvider,
		Providers:       providerMap,
		PrimaryKey:      primaryKey,
		AllProviders:    allProviders,
	})
	// Also update legacy fields for backward compatibility
	s.PrimaryProvider = primaryProvider
	s.Providers = providerMap
	s.PrimaryKey = primaryKey
	s.AllProviders = allProviders

	return nil
}

// StartWatching begins watching config changes for provider map updates.
func (s *ProviderMapService) StartWatching() {
	if s.cfgSvc == nil || s.cfgSvc.watcher == nil {
		return
	}

	s.cfgSvc.watcher.OnReload(func(newCfg *config.Config) error {
		if err := s.RebuildFrom(newCfg); err != nil {
			log.Error().Err(err).Msg("failed to rebuild provider map after config reload")
		}
		log.Info().Msg("provider map rebuilt after config reload")
		return nil
	})
}

// NewProviderMap creates the map of enabled providers with hot-reload support.
// Supports hot-reload: call StartWatching() is invoked automatically.
func NewProviderMap(i do.Injector) (*ProviderMapService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	cfg := cfgSvc.Config

	svc := &ProviderMapService{
		cfgSvc:       cfgSvc,
		Providers:    make(map[string]providers.Provider),
		AllProviders: nil,
	}

	var primaryProvider providers.Provider
	var primaryKey string

	ctx := context.Background()

	for idx := range cfg.Providers {
		p := &cfg.Providers[idx]
		if !p.Enabled {
			continue
		}

		prov, err := createProvider(ctx, p)
		if errors.Is(err, ErrUnknownProviderType) {
			continue // Skip unknown provider types
		}
		if err != nil {
			return nil, err
		}

		svc.Providers[p.Name] = prov
		svc.AllProviders = append(svc.AllProviders, prov)

		// First enabled provider becomes the primary
		if primaryProvider == nil {
			primaryProvider = prov
			if len(p.Keys) > 0 {
				primaryKey = p.Keys[0].Key
			}
		}
	}

	if primaryProvider == nil {
		return nil, fmt.Errorf("no enabled provider found (supported: %s)", supportedProviderTypes)
	}

	svc.PrimaryProvider = primaryProvider
	svc.PrimaryKey = primaryKey

	// Store initial data in atomic pointer
	svc.data.Store(&providerMapData{
		PrimaryProvider: primaryProvider,
		Providers:       svc.Providers,
		PrimaryKey:      primaryKey,
		AllProviders:    svc.AllProviders,
	})

	// Start watching for config changes
	svc.StartWatching()

	return svc, nil
}
