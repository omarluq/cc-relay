package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const (
	statusConfigFileName  = "config.yaml"
	statusRestoreWdErrFmt = "failed to restore working directory: %v"
)

func writeStatusConfig(t *testing.T, dir, listenAddr string) string {
	t.Helper()
	configPath := filepath.Join(dir, statusConfigFileName)
	configContent := "server:\n  listen: " + listenAddr + "\n  api_key: test\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func saveWdHome(t *testing.T) func() {
	t.Helper()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")

	return func() {
		if err := os.Chdir(origWd); err != nil {
			t.Logf(statusRestoreWdErrFmt, err)
		}
		os.Setenv("HOME", origHome)
	}
}

func TestFindConfigFileForStatus(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Save original working directory and HOME
	defer saveWdHome(t)()

	// Create temp directory with config.yaml
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, statusConfigFileName)
	if err := os.WriteFile(configPath, []byte("server:\n  listen: localhost:8787\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set HOME to empty temp dir to prevent finding real config
	os.Setenv("HOME", t.TempDir())

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Test finding config in current directory
	found := findConfigFileForStatus()
	if found != statusConfigFileName {
		t.Errorf("Expected %q, got %q", statusConfigFileName, found)
	}
}

func TestFindConfigFileForStatusNotFound(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Save original working directory and HOME
	defer saveWdHome(t)()

	// Change to temp directory without config.yaml
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Set HOME to temp dir so it won't find user's config
	os.Setenv("HOME", tmpDir)

	// Should return default even if not found
	found := findConfigFileForStatus()
	if found != statusConfigFileName {
		t.Errorf("Expected %q default, got %q", statusConfigFileName, found)
	}
}

func TestFindConfigFileForStatusInHomeDir(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Save original working directory and HOME
	defer saveWdHome(t)()

	// Create temp directories
	tmpDir := t.TempDir()
	workDir := t.TempDir()

	// Create config in HOME/.config/cc-relay/
	configDir := filepath.Join(tmpDir, ".config", "cc-relay")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, statusConfigFileName)
	if err := os.WriteFile(configPath, []byte("server:\n  listen: localhost:8787\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set HOME and change to work directory
	os.Setenv("HOME", tmpDir)
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}

	// Should find config in HOME/.config/cc-relay/
	found := findConfigFileForStatus()
	if found != configPath {
		t.Errorf("Expected %q, got %q", configPath, found)
	}
}

func TestRunStatusServerRunning(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

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

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	cfgFile = configPath

	// runStatus should succeed
	err := runStatus(nil, nil)
	if err != nil {
		t.Errorf("Expected success for running server, got error: %v", err)
	}
}

func TestRunStatusServerNotRunning(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Create temp config file pointing to a non-existent server
	tmpDir := t.TempDir()
	configPath := writeStatusConfig(t, tmpDir, "127.0.0.1:19999")

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	cfgFile = configPath

	// runStatus should fail
	err := runStatus(nil, nil)
	if err == nil {
		t.Error("Expected error for non-running server")
	}
}

func TestRunStatusServerUnhealthy(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

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

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	cfgFile = configPath

	// runStatus should fail
	err := runStatus(nil, nil)
	if err == nil {
		t.Error("Expected error for unhealthy server")
	}
}

func TestRunStatusInvalidConfig(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	cfgFile = "/nonexistent/path/config.yaml"

	// runStatus should fail
	err := runStatus(nil, nil)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}
