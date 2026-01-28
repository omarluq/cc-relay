package di

import (
	"fmt"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/keypool"
)

// keyPoolData holds the primary key pool for atomic swap.
type keyPoolData struct {
	Pool         *keypool.KeyPool
	ProviderName string
}

// KeyPoolService wraps the optional key pool for the primary provider.
// Supports hot-reload: primary key pool can be rebuilt on config reload.
type KeyPoolService struct {
	data   atomic.Pointer[keyPoolData]
	cfgSvc *ConfigService

	// For backward compatibility during transition
	Pool         *keypool.KeyPool
	ProviderName string
}

func buildPoolConfig(p *config.ProviderConfig) keypool.PoolConfig {
	poolCfg := keypool.PoolConfig{
		Strategy: p.GetEffectiveStrategy(),
		Keys:     make([]keypool.KeyConfig, len(p.Keys)),
	}

	for j, k := range p.Keys {
		itpm, otpm := k.GetEffectiveTPM()
		poolCfg.Keys[j] = keypool.KeyConfig{
			APIKey:    k.Key,
			RPMLimit:  k.RPMLimit,
			ITPMLimit: itpm,
			OTPMLimit: otpm,
			Priority:  k.Priority,
			Weight:    k.Weight,
		}
	}

	return poolCfg
}

// Get returns the current primary key pool (live, hot-reload aware).
func (s *KeyPoolService) Get() *keypool.KeyPool {
	d := s.data.Load()
	if d == nil {
		return s.Pool
	}
	return d.Pool
}

// RebuildFrom rebuilds the primary key pool from the given config.
// Uses the first enabled provider with pooling enabled.
func (s *KeyPoolService) RebuildFrom(cfg *config.Config) error {
	for idx := range cfg.Providers {
		p := &cfg.Providers[idx]
		if !p.Enabled {
			continue
		}

		if !p.IsPoolingEnabled() {
			s.data.Store(&keyPoolData{ProviderName: p.Name, Pool: nil})
			s.Pool = nil
			s.ProviderName = p.Name
			return nil
		}

		poolCfg := buildPoolConfig(p)

		pool, err := keypool.NewKeyPool(p.Name, poolCfg)
		if err != nil {
			return fmt.Errorf("failed to create key pool for provider %s: %w", p.Name, err)
		}

		s.data.Store(&keyPoolData{ProviderName: p.Name, Pool: pool})
		s.Pool = pool
		s.ProviderName = p.Name
		return nil
	}

	// No enabled providers found
	s.data.Store(&keyPoolData{ProviderName: "", Pool: nil})
	s.Pool = nil
	s.ProviderName = ""
	return nil
}

// StartWatching begins watching config changes for primary key pool updates.
func (s *KeyPoolService) StartWatching() {
	if s.cfgSvc == nil || s.cfgSvc.watcher == nil {
		return
	}

	s.cfgSvc.watcher.OnReload(func(newCfg *config.Config) error {
		if err := s.RebuildFrom(newCfg); err != nil {
			log.Error().Err(err).Msg("failed to rebuild primary key pool after config reload")
			return err
		}
		log.Info().Msg("primary key pool rebuilt after config reload")
		return nil
	})
}

// keyPoolMapData holds the key pools and keys for atomic swap.
type keyPoolMapData struct {
	Pools map[string]*keypool.KeyPool // Provider name -> KeyPool
	Keys  map[string]string           // Provider name -> API key (fallback)
}

// KeyPoolMapService wraps per-provider key pools for multi-provider routing.
// Supports hot-reload: key pools for newly enabled providers are created on reload.
type KeyPoolMapService struct {
	data   atomic.Pointer[keyPoolMapData]
	cfgSvc *ConfigService

	// For backward compatibility during transition
	Pools map[string]*keypool.KeyPool // Provider name -> KeyPool
	Keys  map[string]string           // Provider name -> API key (fallback)
}

// GetPools returns the current key pools (live, hot-reload aware).
func (s *KeyPoolMapService) GetPools() map[string]*keypool.KeyPool {
	d := s.data.Load()
	if d == nil {
		return s.Pools // Fallback to legacy field
	}
	return d.Pools
}

// GetKeys returns the current fallback keys (live, hot-reload aware).
func (s *KeyPoolMapService) GetKeys() map[string]string {
	d := s.data.Load()
	if d == nil {
		return s.Keys // Fallback to legacy field
	}
	return d.Keys
}

// RebuildFrom rebuilds key pools from the given config.
// Called from reload callbacks to create pools for newly enabled providers.
func (s *KeyPoolMapService) RebuildFrom(cfg *config.Config) error {
	pools := make(map[string]*keypool.KeyPool)
	keys := make(map[string]string)
	var rebuildErr error

	for idx := range cfg.Providers {
		p := &cfg.Providers[idx]
		if !p.Enabled {
			continue
		}

		// Store fallback key (first key in list)
		if len(p.Keys) > 0 {
			keys[p.Name] = p.Keys[0].Key
		}

		// Skip pool creation if pooling not enabled for this provider
		if !p.IsPoolingEnabled() {
			continue
		}

		poolCfg := buildPoolConfig(p)

		pool, err := keypool.NewKeyPool(p.Name, poolCfg)
		if err != nil {
			log.Error().Err(err).Str("provider", p.Name).Msg("failed to create key pool on reload")
			rebuildErr = err
			continue // Log and skip, don't fail the entire reload
		}

		pools[p.Name] = pool
	}

	s.data.Store(&keyPoolMapData{Pools: pools, Keys: keys})
	// Also update legacy fields for backward compatibility
	s.Pools = pools
	s.Keys = keys

	return rebuildErr
}

// StartWatching begins watching config changes for key pool updates.
func (s *KeyPoolMapService) StartWatching() {
	if s.cfgSvc == nil || s.cfgSvc.watcher == nil {
		return
	}

	s.cfgSvc.watcher.OnReload(func(newCfg *config.Config) error {
		if err := s.RebuildFrom(newCfg); err != nil {
			log.Error().Err(err).Msg("failed to rebuild key pools after config reload")
			// Don't return error to avoid blocking other callbacks
		}
		log.Info().Msg("key pools rebuilt after config reload")
		return nil
	})
}

// NewKeyPool creates the key pool for the primary provider if pooling is enabled.
func NewKeyPool(i do.Injector) (*KeyPoolService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	cfg := cfgSvc.Config

	svc := &KeyPoolService{cfgSvc: cfgSvc}
	if err := svc.RebuildFrom(cfg); err != nil {
		return nil, err
	}

	// Start watching for config changes
	svc.StartWatching()

	return svc, nil
}

// NewKeyPoolMap creates key pools for all enabled providers.
// This enables dynamic provider routing with per-provider rate limiting.
// Supports hot-reload: call StartWatching() after container init.
func NewKeyPoolMap(i do.Injector) (*KeyPoolMapService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	cfg := cfgSvc.Config

	svc := &KeyPoolMapService{cfgSvc: cfgSvc}
	if err := svc.RebuildFrom(cfg); err != nil {
		return nil, err
	}

	// Start watching for config changes
	svc.StartWatching()

	return svc, nil
}
