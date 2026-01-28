package di

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/proxy"
)

// LoggerService wraps the zerolog logger for DI.
type LoggerService struct {
	Logger *zerolog.Logger
}

// NewLogger creates the zerolog logger from configuration.
func NewLogger(i do.Injector) (*LoggerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)

	logger, err := proxy.NewLogger(cfgSvc.Config.Logging)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &LoggerService{Logger: &logger}, nil
}
