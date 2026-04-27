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
//
// Wires cfg.Server.TimeoutMS into proxy.ServerOptions.WriteTimeout. ReadTimeout
// and IdleTimeout are intentionally not exposed in config — they protect against
// slowloris and tune keep-alive, and the defaults from proxy.NewServer are sane.
func NewHTTPServer(i do.Injector) (*ServerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	handlerSvc := do.MustInvoke[*HandlerService](i)

	srvCfg := cfgSvc.Config.Server

	// ReadTimeout / IdleTimeout intentionally left zero so proxy.NewServer
	// applies its defaults (slowloris guard / keep-alive). They are not
	// user-configurable. WriteTimeout is the public `server.timeout_ms` knob.
	server := proxy.NewServer(proxy.ServerOptions{
		Addr:         srvCfg.Listen,
		Handler:      handlerSvc.Handler,
		WriteTimeout: time.Duration(srvCfg.TimeoutMS) * time.Millisecond,
		ReadTimeout:  0,
		IdleTimeout:  0,
		EnableHTTP2:  srvCfg.EnableHTTP2,
	})

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
