package di

import (
	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/health"
)

// HealthTrackerService wraps the health tracker for DI.
type HealthTrackerService struct {
	Tracker *health.Tracker
	cfgSvc  *ConfigService
	logger  *LoggerService
}

// NewHealthTracker creates the health tracker from configuration.
// The tracker picks up circuit breaker changes on config reload.
func NewHealthTracker(i do.Injector) (*HealthTrackerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	loggerSvc := do.MustInvoke[*LoggerService](i)

	tracker := health.NewTracker(
		cfgSvc.Config.Health.CircuitBreaker,
		loggerSvc.Logger,
	)
	svc := &HealthTrackerService{
		Tracker: tracker,
		cfgSvc:  cfgSvc,
		logger:  loggerSvc,
	}
	svc.startWatching()
	return svc, nil
}

// RebuildFrom applies the circuit breaker settings from cfg.
// The tracker pointer is preserved so handlers keep their reference.
func (h *HealthTrackerService) RebuildFrom(cfg *config.Config) {
	if cfg == nil {
		return
	}
	h.Tracker.Reset(cfg.Health.CircuitBreaker, h.logger.Logger)
}

func (h *HealthTrackerService) startWatching() {
	if h.cfgSvc == nil || h.cfgSvc.watcher == nil {
		return
	}

	h.cfgSvc.watcher.OnReload(func(newCfg *config.Config) error {
		h.RebuildFrom(newCfg)
		return nil
	})
}
