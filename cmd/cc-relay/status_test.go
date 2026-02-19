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

func TestFindConfigFileForStatus(t *testing.T) {
	t.Parallel()

	// Create temp directory with config.yaml
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, statusConfigFileName)
	if err := os.WriteFile(configPath, []byte("server:\n  listen: localhost:8787\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Test: file exists in given directory
	found := findConfigIn(tmpDir)
	if found != filepath.Join(tmpDir, defaultConfigFile) {
		t.Errorf("Expected config in tmpDir, got %q", found)
	}
}

func TestFindConfigFileForStatusNotFound(t *testing.T) {
	t.Parallel()

	// Empty temp directory - no config file
	tmpDir := t.TempDir()

	// Should return default when not found
	found := findConfigIn(tmpDir)
	if found != defaultConfigFile {
		t.Errorf("Expected %q default, got %q", defaultConfigFile, found)
	}
}

func TestFindConfigFileForStatusInHomeDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create config in HOME/.config/cc-relay/
	configDir := filepath.Join(tmpDir, ".config", "cc-relay")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, statusConfigFileName)
	if err := os.WriteFile(configPath, []byte("server:\n  listen: localhost:8787\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Use a work directory that does NOT have config.yaml
	workDir := t.TempDir()

	// Should find config in home/.config/cc-relay/
	found := findConfigInWithHome(workDir, tmpDir)
	if found != configPath {
		t.Errorf("Expected %q, got %q", configPath, found)
	}
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
