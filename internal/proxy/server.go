// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// Server wraps http.Server with cc-relay configuration.
type Server struct {
	httpServer *http.Server
	addr       string
}

// NewServer creates a new Server with proper timeouts for streaming.
// Timeout rationale:
//   - ReadTimeout: 10s - protect against slowloris attacks
//   - WriteTimeout: 600s - Claude Code operations can stream for 10+ minutes
//   - IdleTimeout: 120s - reasonable keep-alive
//
// If enableHTTP2 is true, enables HTTP/2 cleartext (h2c) support for non-TLS connections.
// HTTP/2 provides better multiplexing and performance for Claude Code's concurrent tool calls.
func NewServer(addr string, handler http.Handler, enableHTTP2 bool) *Server {
	// Wrap handler with HTTP/2 support if enabled
	finalHandler := handler
	if enableHTTP2 {
		h2s := &http2.Server{}
		finalHandler = h2c.NewHandler(handler, h2s)
	}

	return &Server{
		addr: addr,
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      finalHandler,
			ReadTimeout:  10 * time.Second,  // Prevent slow client attacks
			WriteTimeout: 600 * time.Second, // 10 min for long streaming responses
			IdleTimeout:  120 * time.Second, // Keep-alive connections
		},
	}
}

// ListenAndServe starts the server (blocks).
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
