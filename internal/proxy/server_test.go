package proxy_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"


	"github.com/omarluq/cc-relay/internal/proxy"
)

func TestNewServerCreatesValidServer(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := proxy.NewServer("127.0.0.1:0", handler, false)

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

func TestNewServerHasCorrectTimeouts(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := proxy.NewServer("127.0.0.1:0", handler, false)

	// Verify timeouts match documented values
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

func TestNewServerHasCorrectHandler(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := proxy.NewServer("127.0.0.1:0", handler, false)

	if proxy.GetHTTPServer(server).Handler == nil {
		t.Error("Expected non-nil handler")
	}
}

func TestServerListenAndServeInvalidAddress(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Use an invalid address that will fail to bind
	server := proxy.NewServer("invalid-address:99999", handler, false)

	err := server.ListenAndServe()
	if err == nil {
		t.Error("Expected error for invalid address")
	}
}

func TestServerShutdown(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := proxy.NewServer("127.0.0.1:0", handler, false)

	// Start server in goroutine
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		if listenErr := server.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			t.Logf("server listen error: %v", listenErr)
		}
	}()

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Wait for server goroutine to finish
	select {
	case <-serverDone:
		// OK
	case <-time.After(5 * time.Second):
		t.Error("Server did not shutdown in time")
	}
}

func TestNewServerHTTP2Enabled(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create server with HTTP/2 enabled
	server := proxy.NewServer("127.0.0.1:0", handler, true)

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

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create server with HTTP/2 disabled
	server := proxy.NewServer("127.0.0.1:0", handler, false)

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
