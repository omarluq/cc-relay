package di

import (
	"fmt"
	"sync"

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

// CheckerService wraps the health checker for DI.
type CheckerService struct {
	Checker   *health.Checker
	cfgSvc    *ConfigService
	tracker   *HealthTrackerService
	logger    *LoggerService
	started   bool
	startedMu sync.Mutex
}

// NewHealthTracker creates the health tracker from configuration.
func NewHealthTracker(i do.Injector) (*HealthTrackerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	loggerSvc := do.MustInvoke[*LoggerService](i)

	tracker := health.NewTracker(
		cfgSvc.Config.Health.CircuitBreaker,
		loggerSvc.Logger,
	)
	return &HealthTrackerService{
		Tracker: tracker,
		cfgSvc:  cfgSvc,
		logger:  loggerSvc,
	}, nil
}

// NewChecker creates the health checker from configuration.
func NewChecker(i do.Injector) (*CheckerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	trackerSvc := do.MustInvoke[*HealthTrackerService](i)
	loggerSvc := do.MustInvoke[*LoggerService](i)

	checkerSvc := &CheckerService{
		cfgSvc:  cfgSvc,
		tracker: trackerSvc,
		logger:  loggerSvc,
	}

	if err := checkerSvc.rebuildFrom(cfgSvc.Config); err != nil {
		return nil, err
	}
	checkerSvc.startWatching()

	return checkerSvc, nil
}

// Shutdown implements do.Shutdowner for graceful checker cleanup.
func (h *CheckerService) Shutdown() error {
	h.startedMu.Lock()
	defer h.startedMu.Unlock()
	if h.Checker != nil && h.started {
		h.Checker.Stop()
		h.started = false
	}
	return nil
}

// Start starts the health checker and records that it is running.
func (h *CheckerService) Start() {
	h.startedMu.Lock()
	h.started = true
	checker := h.Checker
	h.startedMu.Unlock()

	if checker != nil {
		checker.Start()
	}
}

func (h *CheckerService) startWatching() {
	if h.cfgSvc == nil || h.cfgSvc.watcher == nil {
		return
	}

	h.cfgSvc.watcher.OnReload(func(newCfg *config.Config) error {
		return h.rebuildFrom(newCfg)
	})
}

func (h *CheckerService) rebuildFrom(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}

	// Reset tracker with updated config (preserves pointer for handlers)
	h.tracker.Tracker.Reset(cfg.Health.CircuitBreaker, h.logger.Logger)

	checker := health.NewChecker(
		h.tracker.Tracker,
		cfg.Health.HealthCheck,
		h.logger.Logger,
	)

	h.registerProviders(checker, cfg)
	h.swapChecker(checker)
	return nil
}

func (h *CheckerService) registerProviders(checker *health.Checker, cfg *config.Config) {
	for idx := range cfg.Providers {
		pc := &cfg.Providers[idx]
		if !pc.Enabled {
			continue
		}

		baseURL := providerHealthBaseURL(pc)
		healthCheck := health.NewProviderHealthCheck(pc.Name, baseURL, nil)
		checker.RegisterProvider(healthCheck)
		h.logger.Logger.Debug().
			Str("provider", pc.Name).
			Str("base_url", baseURL).
			Msg("registered health check")
	}
}

func (h *CheckerService) swapChecker(checker *health.Checker) {
	h.startedMu.Lock()
	wasRunning := h.started
	oldChecker := h.Checker
	h.Checker = checker
	h.startedMu.Unlock()

	if oldChecker != nil && wasRunning {
		oldChecker.Stop()
		checker.Start()
	}
}

func providerHealthBaseURL(pc *config.ProviderConfig) string {
	baseURL := pc.BaseURL
	switch pc.Type {
	case "bedrock":
		// Bedrock base URL: https://bedrock-runtime.{region}.amazonaws.com
		if pc.AWSRegion != "" {
			baseURL = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", pc.AWSRegion)
		}
	case "vertex":
		// Vertex base URL: https://{region}-aiplatform.googleapis.com
		if pc.GCPRegion != "" {
			baseURL = fmt.Sprintf("https://%s-aiplatform.googleapis.com", pc.GCPRegion)
		}
	case "azure":
		// Azure base URL: https://{resource}.services.ai.azure.com
		if pc.AzureResourceName != "" {
			baseURL = fmt.Sprintf("https://%s.services.ai.azure.com", pc.AzureResourceName)
		}
	}
	return baseURL
}
