package di

import (
	"fmt"

	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/health"
)

// HealthTrackerService wraps the health tracker for DI.
type HealthTrackerService struct {
	Tracker *health.Tracker
}

// CheckerService wraps the health checker for DI.
type CheckerService struct {
	Checker *health.Checker
}

// NewHealthTracker creates the health tracker from configuration.
func NewHealthTracker(i do.Injector) (*HealthTrackerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	loggerSvc := do.MustInvoke[*LoggerService](i)

	tracker := health.NewTracker(
		cfgSvc.Config.Health.CircuitBreaker,
		loggerSvc.Logger,
	)
	return &HealthTrackerService{Tracker: tracker}, nil
}

// NewChecker creates the health checker from configuration.
func NewChecker(i do.Injector) (*CheckerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	trackerSvc := do.MustInvoke[*HealthTrackerService](i)
	loggerSvc := do.MustInvoke[*LoggerService](i)

	checker := health.NewChecker(
		trackerSvc.Tracker,
		cfgSvc.Config.Health.HealthCheck,
		loggerSvc.Logger,
	)

	// Register health checks for all enabled providers
	for idx := range cfgSvc.Config.Providers {
		pc := &cfgSvc.Config.Providers[idx]
		if !pc.Enabled {
			continue
		}

		// Construct base URL based on provider type
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

		// NewProviderHealthCheck handles empty BaseURL (returns NoOpHealthCheck)
		healthCheck := health.NewProviderHealthCheck(pc.Name, baseURL, nil)
		checker.RegisterProvider(healthCheck)
		loggerSvc.Logger.Debug().
			Str("provider", pc.Name).
			Str("base_url", baseURL).
			Msg("registered health check")
	}

	return &CheckerService{Checker: checker}, nil
}

// Shutdown implements do.Shutdowner for graceful checker cleanup.
func (h *CheckerService) Shutdown() error {
	if h.Checker != nil {
		h.Checker.Stop()
	}
	return nil
}
