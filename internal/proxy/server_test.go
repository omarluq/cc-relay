package proxy_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/proxy"
)

// Expected default timeouts. Mirrors the defaults in proxy/server.go so that
// changes there require updating exactly one place in this test file.
const (
	expectedDefaultReadTimeout  = 10 * time.Second
	expectedDefaultWriteTimeout = 600 * time.Second
	expectedDefaultIdleTimeout  = 120 * time.Second
)

func newServerTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// defaultServerOptions returns a ServerOptions with zero-valued timeouts and
// HTTP2 disabled. Tests that only care about a subset of fields can use this
// to satisfy the exhaustruct linter without repeating boilerplate.
func defaultServerOptions(addr string, handler http.Handler) proxy.ServerOptions {
	return proxy.ServerOptions{
		Addr:         addr,
		Handler:      handler,
		WriteTimeout: 0,
		ReadTimeout:  0,
		IdleTimeout:  0,
		EnableHTTP2:  false,
	}
}

func TestNewServerCreatesValidServer(t *testing.T) {
	t.Parallel()

	server := proxy.NewServer(defaultServerOptions("127.0.0.1:0", newServerTestHandler()))

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if proxy.GetServerAddr(server) != "127.0.0.1:0" {
		t.Errorf("Expected addr '127.0.0.1:0', got %s", proxy.GetServerAddr(server))
	}

	if proxy.GetHTTPServer(server) == nil {
		t.Fatal("Expected non-nil httpServer")
	}
}

func TestNewServerHasDefaultTimeouts(t *testing.T) {
	t.Parallel()

	server := proxy.NewServer(defaultServerOptions("127.0.0.1:0", newServerTestHandler()))

	if got := proxy.GetHTTPServer(server).ReadTimeout; got != expectedDefaultReadTimeout {
		t.Errorf("ReadTimeout = %v, want %v", got, expectedDefaultReadTimeout)
	}
	if got := proxy.GetHTTPServer(server).WriteTimeout; got != expectedDefaultWriteTimeout {
		t.Errorf("WriteTimeout = %v, want %v", got, expectedDefaultWriteTimeout)
	}
	if got := proxy.GetHTTPServer(server).IdleTimeout; got != expectedDefaultIdleTimeout {
		t.Errorf("IdleTimeout = %v, want %v", got, expectedDefaultIdleTimeout)
	}
}

// Regression test: server.timeout_ms must drive WriteTimeout. Previously
// hardcoded — see memory/known_issues.md (resolved).
func TestNewServerWriteTimeoutFromOptions(t *testing.T) {
	t.Parallel()

	custom := 90 * time.Second
	opts := defaultServerOptions("127.0.0.1:0", newServerTestHandler())
	opts.WriteTimeout = custom
	server := proxy.NewServer(opts)

	if got := proxy.GetHTTPServer(server).WriteTimeout; got != custom {
		t.Errorf("WriteTimeout = %v, want %v (server.timeout_ms must wire through)", got, custom)
	}
	if got := proxy.GetHTTPServer(server).ReadTimeout; got != expectedDefaultReadTimeout {
		t.Errorf("ReadTimeout = %v, want %v default", got, expectedDefaultReadTimeout)
	}
	if got := proxy.GetHTTPServer(server).IdleTimeout; got != expectedDefaultIdleTimeout {
		t.Errorf("IdleTimeout = %v, want %v default", got, expectedDefaultIdleTimeout)
	}
}

// Regression test: explicit overrides for ReadTimeout/IdleTimeout work too.
func TestNewServerAllTimeoutsConfigurable(t *testing.T) {
	t.Parallel()

	opts := defaultServerOptions("127.0.0.1:0", newServerTestHandler())
	opts.WriteTimeout = 30 * time.Second
	opts.ReadTimeout = 5 * time.Second
	opts.IdleTimeout = 60 * time.Second
	server := proxy.NewServer(opts)

	httpSrv := proxy.GetHTTPServer(server)
	if httpSrv.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", httpSrv.ReadTimeout)
	}
	if httpSrv.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v, want 30s", httpSrv.WriteTimeout)
	}
	if httpSrv.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want 60s", httpSrv.IdleTimeout)
	}
}

func TestNewServerHasCorrectHandler(t *testing.T) {
	t.Parallel()

	server := proxy.NewServer(defaultServerOptions("127.0.0.1:0", newServerTestHandler()))

	if proxy.GetHTTPServer(server).Handler == nil {
		t.Error("Expected non-nil handler")
	}
}

func TestServerListenAndServeInvalidAddress(t *testing.T) {
	t.Parallel()

	server := proxy.NewServer(defaultServerOptions("invalid-address:99999", newServerTestHandler()))

	err := server.ListenAndServe()
	if err == nil {
		t.Error("Expected error for invalid address")
	}
}

func TestServerShutdown(t *testing.T) {
	t.Parallel()

	server := proxy.NewServer(defaultServerOptions("127.0.0.1:0", newServerTestHandler()))

	// Start server in goroutine
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		if listenErr := server.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			t.Logf("server listen error: %v", listenErr)
		}
	}()

	time.Sleep(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	select {
	case <-serverDone:
		// OK
	case <-time.After(5 * time.Second):
		t.Error("Server did not shutdown in time")
	}
}

func TestNewServerHTTP2Enabled(t *testing.T) {
	t.Parallel()

	opts := defaultServerOptions("127.0.0.1:0", newServerTestHandler())
	opts.EnableHTTP2 = true
	server := proxy.NewServer(opts)

	if server == nil {
		t.Fatal("Expected non-nil server")
	}
	if proxy.GetHTTPServer(server) == nil {
		t.Fatal("Expected non-nil httpServer")
	}
	if proxy.GetHTTPServer(server).Handler == nil {
		t.Error("Expected non-nil handler (should be wrapped with h2c)")
	}
}

func TestNewServerHTTP2Disabled(t *testing.T) {
	t.Parallel()

	server := proxy.NewServer(defaultServerOptions("127.0.0.1:0", newServerTestHandler()))

	if server == nil {
		t.Fatal("Expected non-nil server")
	}
	if proxy.GetHTTPServer(server) == nil {
		t.Fatal("Expected non-nil httpServer")
	}
	if proxy.GetHTTPServer(server).Handler == nil {
		t.Error("Expected non-nil handler")
	}
}
