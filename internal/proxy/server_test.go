package proxy_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/proxy"
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

	if proxy.GetHTTPServer(server).ReadTimeout != 10*time.Second {
		t.Errorf("Expected ReadTimeout 10s, got %v", proxy.GetHTTPServer(server).ReadTimeout)
	}
	if proxy.GetHTTPServer(server).WriteTimeout != 600*time.Second {
		t.Errorf("Expected WriteTimeout 600s, got %v", proxy.GetHTTPServer(server).WriteTimeout)
	}
	if proxy.GetHTTPServer(server).IdleTimeout != 120*time.Second {
		t.Errorf("Expected IdleTimeout 120s, got %v", proxy.GetHTTPServer(server).IdleTimeout)
	}
}

// Regression test: server.timeout_ms must drive WriteTimeout. Previously
// hardcoded — see memory/known_issues.md (resolved).
func TestNewServerWriteTimeoutFromOptions(t *testing.T) {
	t.Parallel()

	custom := 90 * time.Second
	server := proxy.NewServer(proxy.ServerOptions{
		Addr:         "127.0.0.1:0",
		Handler:      newServerTestHandler(),
		WriteTimeout: custom,
		ReadTimeout:  0,
		IdleTimeout:  0,
		EnableHTTP2:  false,
	})

	if got := proxy.GetHTTPServer(server).WriteTimeout; got != custom {
		t.Errorf("WriteTimeout = %v, want %v (server.timeout_ms must wire through)", got, custom)
	}
	if got := proxy.GetHTTPServer(server).ReadTimeout; got != 10*time.Second {
		t.Errorf("ReadTimeout = %v, want 10s default", got)
	}
	if got := proxy.GetHTTPServer(server).IdleTimeout; got != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want 120s default", got)
	}
}

// Regression test: explicit overrides for ReadTimeout/IdleTimeout work too.
func TestNewServerAllTimeoutsConfigurable(t *testing.T) {
	t.Parallel()

	server := proxy.NewServer(proxy.ServerOptions{
		Addr:         "127.0.0.1:0",
		Handler:      newServerTestHandler(),
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  5 * time.Second,
		IdleTimeout:  60 * time.Second,
		EnableHTTP2:  false,
	})

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

	server := proxy.NewServer(proxy.ServerOptions{
		Addr:         "127.0.0.1:0",
		Handler:      newServerTestHandler(),
		WriteTimeout: 0,
		ReadTimeout:  0,
		IdleTimeout:  0,
		EnableHTTP2:  true,
	})

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
