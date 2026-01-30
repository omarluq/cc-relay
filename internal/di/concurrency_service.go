package di

import (
	"github.com/rs/zerolog/log"
	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/proxy"
)

// ConcurrencyService wraps the concurrency limiter for DI.
type ConcurrencyService struct {
	Limiter *proxy.ConcurrencyLimiter
}

// NewConcurrencyService creates the concurrency limiter service.
// The limiter is initialized with the current config value and updated on hot-reload.
func NewConcurrencyService(i do.Injector) (*ConcurrencyService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	cfg := cfgSvc.Get()

	// Initialize with current config value
	maxConcurrent := int64(0)
	if cfg != nil {
		maxConcurrent = int64(cfg.Server.MaxConcurrent)
	}

	limiter := proxy.NewConcurrencyLimiter(maxConcurrent)

	svc := &ConcurrencyService{Limiter: limiter}

	// Register for hot-reload updates if watcher is available
	svc.startWatching(cfgSvc)

	return svc, nil
}

// startWatching registers for config hot-reload to update the concurrency limit.
func (s *ConcurrencyService) startWatching(cfgSvc *ConfigService) {
	if cfgSvc.watcher == nil {
		return
	}

	cfgSvc.watcher.OnReload(func(newCfg *config.Config) error {
		if newCfg != nil {
			newLimit := int64(newCfg.Server.MaxConcurrent)
			oldLimit := s.Limiter.GetLimit()
			if newLimit != oldLimit {
				s.Limiter.SetLimit(newLimit)
				log.Info().
					Int64("old_limit", oldLimit).
					Int64("new_limit", newLimit).
					Msg("concurrency limit updated via hot-reload")
			}
		}
		return nil
	})
}
