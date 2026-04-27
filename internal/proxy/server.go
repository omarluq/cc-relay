// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// Default timeouts applied when ServerOptions fields are zero.
//
// ReadTimeout is intentionally short to defend against slow-client (slowloris)
// attacks. The proxy's request bodies are small (LLM prompts), so 10s is plenty.
//
// WriteTimeout is intentionally long because LLM streaming responses can run
// for many minutes. This is what the public `server.timeout_ms` config option
// controls — see ServerOptions.WriteTimeout below.
//
// IdleTimeout is a reasonable keep-alive window for connection reuse.
const (
	defaultReadTimeout  = 10 * time.Second
	defaultWriteTimeout = 600 * time.Second
	defaultIdleTimeout  = 120 * time.Second
)

// ServerOptions configures the HTTP server.
//
// Zero values for any timeout fall back to safe defaults (see defaultReadTimeout
// etc.), so callers only need to populate what they want to override.
//
// Field order: pointer-bearing fields first (Handler, then Addr), then int64
// timeouts, then bool. This satisfies govet fieldalignment by keeping the GC
// scan window to 24 pointer bytes.
type ServerOptions struct {
	// Handler is the root HTTP handler (mux + middleware chain).
	Handler http.Handler

	// Addr is the TCP listen address (e.g., "127.0.0.1:8787").
	Addr string

	// WriteTimeout caps how long a single response may take. This is the public
	// `server.timeout_ms` knob: it must be long enough for the slowest streaming
	// LLM response. Zero falls back to defaultWriteTimeout (600s).
	WriteTimeout time.Duration

	// ReadTimeout caps how long a request body+headers may take to read.
	// Kept small for slowloris defense. Zero falls back to defaultReadTimeout (10s).
	ReadTimeout time.Duration

	// IdleTimeout caps the keep-alive idle window. Zero falls back to
	// defaultIdleTimeout (120s).
	IdleTimeout time.Duration

	// EnableHTTP2 enables HTTP/2 cleartext (h2c). Recommended for Claude Code's
	// concurrent tool calls.
	EnableHTTP2 bool
}

// Server wraps http.Server with cc-relay configuration.
type Server struct {
	httpServer *http.Server
	addr       string
}

// NewServer creates a Server from ServerOptions.
//
// Timeout rationale:
//   - ReadTimeout: short (10s default) to protect against slowloris attacks.
//   - WriteTimeout: long (600s default, configurable via server.timeout_ms) —
//     LLM streaming responses can run for many minutes.
//   - IdleTimeout: 120s default for reasonable keep-alive reuse.
//
// When EnableHTTP2 is true, the handler is wrapped with h2c for cleartext
// HTTP/2 (better multiplexing for Claude Code's parallel tool calls).
func NewServer(opts ServerOptions) *Server {
	finalHandler := opts.Handler
	if opts.EnableHTTP2 {
		h2s := &http2.Server{}
		finalHandler = h2c.NewHandler(opts.Handler, h2s)
	}

	readTimeout := opts.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = defaultReadTimeout
	}
	writeTimeout := opts.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = defaultWriteTimeout
	}
	idleTimeout := opts.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = defaultIdleTimeout
	}

	return &Server{
		addr: opts.Addr,
		httpServer: &http.Server{
			Addr:         opts.Addr,
			Handler:      finalHandler,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
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
