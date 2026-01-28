package di

import (
	"context"
	"time"

	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/proxy"
)

// ServerService wraps the HTTP server.
type ServerService struct {
	Server *proxy.Server
}

// NewHTTPServer creates the HTTP server.
func NewHTTPServer(i do.Injector) (*ServerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	handlerSvc := do.MustInvoke[*HandlerService](i)

	server := proxy.NewServer(
		cfgSvc.Config.Server.Listen,
		handlerSvc.Handler,
		cfgSvc.Config.Server.EnableHTTP2,
	)

	return &ServerService{Server: server}, nil
}

// Shutdown implements do.Shutdowner for graceful server shutdown.
func (s *ServerService) Shutdown() error {
	if s.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return s.Server.Shutdown(ctx)
	}
	return nil
}
