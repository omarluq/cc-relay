package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

const (
	statusConfigFileName = "config.yaml"
)

func writeStatusConfig(t *testing.T, dir, listenAddr string) string {
	t.Helper()
	configPath := filepath.Join(dir, statusConfigFileName)
	configContent := "server:\n  listen: " + listenAddr + "\n  api_key: test\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func TestRunStatusServerRunning(t *testing.T) {
	t.Parallel()

	// Create a mock server that returns 200 OK on /health
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Extract host:port from server URL (remove http://)
	serverAddr := server.URL[7:] // Remove "http://"

	// Create temp config file pointing to our mock server
	tmpDir := t.TempDir()
	configPath := writeStatusConfig(t, tmpDir, serverAddr)

	// checkStatusWithConfig should succeed
	err := checkStatusWithConfig(&cobra.Command{}, configPath)
	if err != nil {
		t.Errorf("Expected success for running server, got error: %v", err)
	}
}

func TestRunStatusServerNotRunning(t *testing.T) {
	t.Parallel()

	// Create temp config file pointing to a non-existent server
	tmpDir := t.TempDir()
	configPath := writeStatusConfig(t, tmpDir, "127.0.0.1:19999")

	// checkStatusWithConfig should fail
	err := checkStatusWithConfig(&cobra.Command{}, configPath)
	if err == nil {
		t.Error("Expected error for non-running server")
	}
}

func TestRunStatusServerUnhealthy(t *testing.T) {
	t.Parallel()

	// Create a mock server that returns 500 on /health
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Extract host:port from server URL
	serverAddr := server.URL[7:]

	// Create temp config file
	tmpDir := t.TempDir()
	configPath := writeStatusConfig(t, tmpDir, serverAddr)

	// checkStatusWithConfig should fail
	err := checkStatusWithConfig(&cobra.Command{}, configPath)
	if err == nil {
		t.Error("Expected error for unhealthy server")
	}
}

func TestRunStatusInvalidConfig(t *testing.T) {
	t.Parallel()

	// checkStatusWithConfig should fail
	err := checkStatusWithConfig(&cobra.Command{}, "/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}
