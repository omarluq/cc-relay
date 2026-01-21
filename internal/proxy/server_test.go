package proxy

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNewServer_CreatesValidServer(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := NewServer("127.0.0.1:0", handler, false)

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.addr != "127.0.0.1:0" {
		t.Errorf("Expected addr '127.0.0.1:0', got %s", server.addr)
	}

	if server.httpServer == nil {
		t.Fatal("Expected non-nil httpServer")
	}
}

func TestNewServer_HasCorrectTimeouts(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := NewServer("127.0.0.1:0", handler, false)

	// Verify timeouts match documented values
	if server.httpServer.ReadTimeout != 10*time.Second {
		t.Errorf("Expected ReadTimeout 10s, got %v", server.httpServer.ReadTimeout)
	}

	if server.httpServer.WriteTimeout != 600*time.Second {
		t.Errorf("Expected WriteTimeout 600s, got %v", server.httpServer.WriteTimeout)
	}

	if server.httpServer.IdleTimeout != 120*time.Second {
		t.Errorf("Expected IdleTimeout 120s, got %v", server.httpServer.IdleTimeout)
	}
}

func TestNewServer_HasCorrectHandler(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := NewServer("127.0.0.1:0", handler, false)

	if server.httpServer.Handler == nil {
		t.Error("Expected non-nil handler")
	}
}

func TestServer_ListenAndServe_InvalidAddress(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Use an invalid address that will fail to bind
	server := NewServer("invalid-address:99999", handler, false)

	err := server.ListenAndServe()
	if err == nil {
		t.Error("Expected error for invalid address")
	}
}

func TestServer_Shutdown(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := NewServer("127.0.0.1:0", handler, false)

	// Start server in goroutine
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		_ = server.ListenAndServe()
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

func TestNewServer_HTTP2Enabled(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create server with HTTP/2 enabled
	server := NewServer("127.0.0.1:0", handler, true)

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.httpServer == nil {
		t.Fatal("Expected non-nil httpServer")
	}

	if server.httpServer.Handler == nil {
		t.Error("Expected non-nil handler (should be wrapped with h2c)")
	}
}

func TestNewServer_HTTP2Disabled(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create server with HTTP/2 disabled
	server := NewServer("127.0.0.1:0", handler, false)

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.httpServer == nil {
		t.Fatal("Expected non-nil httpServer")
	}

	if server.httpServer.Handler == nil {
		t.Error("Expected non-nil handler")
	}
}
