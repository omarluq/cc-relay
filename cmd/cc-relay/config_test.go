package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omarluq/cc-relay/internal/config"
)

const (
	defaultListenAddr = "localhost:8787"
	localListenAddr   = "127.0.0.1:8787"
	defaultAPIKey     = "test-key"
	configFileName    = "config.yaml"
	unexpectedErrFmt  = "Unexpected error: %v"
	restoreWdErrFmt   = "failed to restore working directory: %v"
)

func TestValidateConfigValid(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: defaultListenAddr,
			APIKey: defaultAPIKey,
		},
		Providers: []config.ProviderConfig{
			{
				Name:    "anthropic",
				Type:    "anthropic",
				Enabled: true,
				Keys: []config.KeyConfig{
					{Key: "test-api-key"},
				},
			},
		},
	}

	if err := validateConfig(cfg); err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidateConfigNoListen(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: defaultAPIKey,
		},
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Error("Expected error for missing listen address")
	}

	if err != nil && err.Error() != "server.listen is required" {
		t.Errorf(unexpectedErrFmt, err)
	}
}

func TestValidateConfigNoAPIKey(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: defaultListenAddr,
		},
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Error("Expected error for missing API key")
	}

	if err != nil && err.Error() != "server.api_key is required" {
		t.Errorf(unexpectedErrFmt, err)
	}
}

func TestValidateConfigNoEnabledProvider(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: defaultListenAddr,
			APIKey: defaultAPIKey,
		},
		Providers: []config.ProviderConfig{
			{
				Name:    "anthropic",
				Enabled: false,
			},
		},
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Error("Expected error for no enabled providers")
	}

	if err != nil && err.Error() != "no enabled providers configured" {
		t.Errorf(unexpectedErrFmt, err)
	}
}

func TestValidateConfigProviderNoKeys(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: defaultListenAddr,
			APIKey: defaultAPIKey,
		},
		Providers: []config.ProviderConfig{
			{
				Name:    "anthropic",
				Enabled: true,
				Keys:    []config.KeyConfig{},
			},
		},
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Error("Expected error for provider with no keys")
	}
}

func TestValidateConfigMultipleProviders(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: defaultListenAddr,
			APIKey: defaultAPIKey,
		},
		Providers: []config.ProviderConfig{
			{
				Name:    "anthropic",
				Type:    "anthropic",
				Enabled: true,
				Keys:    []config.KeyConfig{{Key: "key1"}},
			},
			{
				Name:    "zai",
				Type:    "zai",
				Enabled: true,
				Keys:    []config.KeyConfig{{Key: "key2"}},
			},
		},
	}

	if err := validateConfig(cfg); err != nil {
		t.Errorf("Expected valid config with multiple providers, got error: %v", err)
	}
}

func TestValidateConfigEmptyProviders(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: defaultListenAddr,
			APIKey: defaultAPIKey,
		},
		Providers: []config.ProviderConfig{},
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Error("Expected error for empty providers")
	}
}

func TestFindConfigFileForValidate(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Save original working directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Logf(restoreWdErrFmt, err)
		}
	}()

	// Create temp directory with config.yaml
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, configFileName)
	if err := os.WriteFile(configPath, []byte("server:\n  listen: "+defaultListenAddr+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Test finding config in current directory
	found := findConfigFileForValidate()
	if found != configFileName {
		t.Errorf("Expected %q, got %q", configFileName, found)
	}
}

func TestFindConfigFileForValidateNotFound(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Save original working directory and HOME
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")

	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Logf(restoreWdErrFmt, err)
		}
		os.Setenv("HOME", origHome)
	}()

	// Change to temp directory without config.yaml
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Set HOME to temp dir so it won't find user's config
	os.Setenv("HOME", tmpDir)

	// Should return default even if not found
	found := findConfigFileForValidate()
	if found != configFileName {
		t.Errorf("Expected %q default, got %q", configFileName, found)
	}
}

func TestRunConfigValidateValidConfig(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Create a valid config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, configFileName)
	configContent := `
server:
  listen: "` + localListenAddr + `"
  api_key: "` + defaultAPIKey + `"
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"
    keys:
      - key: "test-api-key"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	cfgFile = configPath

	// runConfigValidate should succeed
	err := runConfigValidate(nil, nil)
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestRunConfigValidateInvalidYAML(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Create a config file with invalid YAML
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, configFileName)
	if err := os.WriteFile(configPath, []byte("invalid: yaml: : content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	cfgFile = configPath

	// runConfigValidate should fail
	err := runConfigValidate(nil, nil)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestRunConfigValidateMissingServer(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Create a config file missing server section
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, configFileName)
	configContent := `
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "test-api-key"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	cfgFile = configPath

	// runConfigValidate should fail
	err := runConfigValidate(nil, nil)
	if err == nil {
		t.Error("Expected error for missing server section")
	}
	if err != nil && !strings.Contains(err.Error(), "server.listen is required") {
		t.Errorf(unexpectedErrFmt, err)
	}
}

func TestRunConfigValidateNonexistentFile(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies global cfgFile)

	// Save original cfgFile
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()

	cfgFile = "/nonexistent/path/" + configFileName

	// runConfigValidate should fail
	err := runConfigValidate(nil, nil)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}
