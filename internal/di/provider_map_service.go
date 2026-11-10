package di

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/outbound"
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
	// transports holds the shared outbound transports, one per proxy setting.
	transports *outbound.Registry

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

// buildProvider creates a provider with the outbound transport its config resolves to.
// It also returns the resolved outbound proxy setting so unused transports can be released.
func (s *ProviderMapService) buildProvider(
	ctx context.Context,
	cfg *config.Config,
	providerCfg *config.ProviderConfig,
) (providers.Provider, string, error) {
	if s.transports == nil {
		s.transports = outbound.NewRegistry()
	}

	setting := providerCfg.GetEffectiveOutboundProxy(cfg.Server.OutboundProxy)

	// Unset keeps today's behavior: http.DefaultTransport and SDK defaults,
	// which honor HTTP_PROXY / HTTPS_PROXY / NO_PROXY.
	var transport http.RoundTripper
	if setting != "" {
		var err error
		transport, err = s.transports.Transport(setting)
		if err != nil {
			return nil, "", fmt.Errorf("provider %s: %w", providerCfg.Name, err)
		}
		log.Info().
			Str("provider", providerCfg.Name).
			Str("outbound_proxy", outbound.Redact(setting)).
			Msg("outbound proxy configured")
	}

	prov, err := createProvider(ctx, providerCfg, transport)
	if err != nil {
		return nil, "", err
	}
	return prov, setting, nil
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
	var outboundSettings []string

	for idx := range cfg.Providers {
		providerCfg := &cfg.Providers[idx]
		if !providerCfg.Enabled {
			continue
		}

		prov, setting, err := s.buildProvider(ctx, cfg, providerCfg)
		if errors.Is(err, ErrUnknownProviderType) {
			log.Warn().
				Str("provider", providerCfg.Name).
				Str("type", providerCfg.Type).
				Msg("skipping unknown provider type on reload")
			continue // Skip unknown provider types
		}
		if err != nil {
			log.Error().Err(err).Str("provider", providerCfg.Name).Msg("failed to create provider on reload")
			continue // Log and skip, don't fail the entire reload
		}

		outboundSettings = append(outboundSettings, setting)
		providerMap[providerCfg.Name] = prov
		allProviders = append(allProviders, prov)

		if primaryProvider == nil {
			primaryProvider = prov
			if len(providerCfg.Keys) > 0 {
				primaryKey = providerCfg.Keys[0].Key
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

	// Release transports for proxy settings no provider uses anymore
	// (e.g. a proxy URL or its credentials changed). Requests still in
	// flight on them finish normally; only idle connections are closed.
	s.transports.Retain(outboundSettings)

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
		data:            atomic.Pointer[providerMapData]{},
		cfgSvc:          cfgSvc,
		transports:      outbound.NewRegistry(),
		Providers:       make(map[string]providers.Provider),
		PrimaryProvider: nil,
		PrimaryKey:      "",
		AllProviders:    nil,
	}

	var primaryProvider providers.Provider
	var primaryKey string

	ctx := context.Background()

	for idx := range cfg.Providers {
		providerCfg := &cfg.Providers[idx]
		if !providerCfg.Enabled {
			continue
		}

		prov, _, err := svc.buildProvider(ctx, cfg, providerCfg)
		if errors.Is(err, ErrUnknownProviderType) {
			continue // Skip unknown provider types
		}
		if err != nil {
			return nil, err
		}

		svc.Providers[providerCfg.Name] = prov
		svc.AllProviders = append(svc.AllProviders, prov)

		// First enabled provider becomes the primary
		if primaryProvider == nil {
			primaryProvider = prov
			if len(providerCfg.Keys) > 0 {
				primaryKey = providerCfg.Keys[0].Key
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
