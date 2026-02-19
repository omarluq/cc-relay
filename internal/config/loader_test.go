package config_test

import (
	"github.com/omarluq/cc-relay/internal/config"
	"errors"
	"os"
	"strings"
	"testing"
)

// Helper to verify server config.
func assertServerConfig(t *testing.T, listen string, timeoutMS, maxConcurrent int, apiKey string, cfg *config.Config) {
	t.Helper()
	if cfg.Server.Listen != listen {
		t.Errorf("Expected listen=%s, got %s", listen, cfg.Server.Listen)
	}
	if cfg.Server.TimeoutMS != timeoutMS {
		t.Errorf("Expected timeout_ms=%d, got %d", timeoutMS, cfg.Server.TimeoutMS)
	}
	if cfg.Server.MaxConcurrent != maxConcurrent {
		t.Errorf("Expected max_concurrent=%d, got %d", maxConcurrent, cfg.Server.MaxConcurrent)
	}
	if cfg.Server.APIKey != apiKey {
		t.Errorf("Expected api_key=%s, got %s", apiKey, cfg.Server.APIKey)
	}
}

// Helper to verify provider config.
func assertProviderConfig(
	t *testing.T,
	name, pType string,
	enabled bool,
	cfg *config.Config,
	providerIdx int,
) *config.KeyConfig {
	t.Helper()
	if len(cfg.Providers) <= providerIdx {
		t.Fatalf("Expected at least %d provider(s), got %d", providerIdx+1, len(cfg.Providers))
	}
	provider := cfg.Providers[providerIdx]
	if provider.Name != name {
		t.Errorf("Expected provider name=%s, got %s", name, provider.Name)
	}
	if provider.Type != pType {
		t.Errorf("Expected provider type=%s, got %s", pType, provider.Type)
	}
	if provider.Enabled != enabled {
		t.Errorf("Expected provider enabled=%v, got %v", enabled, provider.Enabled)
	}
	if len(provider.Keys) != 1 {
		t.Fatalf("Expected 1 key, got %d", len(provider.Keys))
	}
	return &provider.Keys[0]
}

// Helper to verify key config.
func assertKeyConfig(t *testing.T, key string, rpm, tpm int, actual *config.KeyConfig) {
	t.Helper()
	if actual.Key != key {
		t.Errorf("Expected key=%s, got %s", key, actual.Key)
	}
	if actual.RPMLimit != rpm {
		t.Errorf("Expected rpm_limit=%d, got %d", rpm, actual.RPMLimit)
	}
	if actual.TPMLimit != tpm {
		t.Errorf("Expected tpm_limit=%d, got %d", tpm, actual.TPMLimit)
	}
}

// Helper to verify logging config.
func assertLoggingConfig(t *testing.T, cfg *config.Config) {
	t.Helper()
	if cfg.Logging.Level != logLevelInfo {
		t.Errorf("Expected logging level=info, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != logFormatJSON {
		t.Errorf("Expected logging format=json, got %s", cfg.Logging.Format)
	}
}

func TestLoadValidYAML(t *testing.T) {
	t.Parallel()

	yamlContent := `server:
  listen: "` + defaultListenAddr + `"
  timeout_ms: 60000
  max_concurrent: 10
  api_key: "test-key"

providers:
  - name: "` + testProviderType + `"
    type: "` + testProviderType + `"
    enabled: true
    keys:
      - key: "sk-ant-test"
        rpm_limit: 60
        tpm_limit: 100000

logging:
  level: "info"
  format: "json"
`

	cfg, err := config.LoadFromReader(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("config.LoadFromReader failed: %v", err)
	}

	assertServerConfig(t, defaultListenAddr, 60000, 10, "test-key", cfg)
	key := assertProviderConfig(t, testProviderType, testProviderType, true, cfg, 0)
	assertKeyConfig(t, "sk-ant-test", 60, 100000, key)
	assertLoggingConfig(t, cfg)
}

func TestLoadEnvironmentExpansion(t *testing.T) {
	t.Parallel()

	// Set a test environment variable
	testKey := "TEST_API_KEY_12345"
	testValue := "sk-test-value"
	if err := os.Setenv(testKey, testValue); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	defer func() {
		if err := os.Unsetenv(testKey); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	yamlContent := `server:
  listen: "` + defaultListenAddr + `"
  api_key: "${` + testKey + `}"

providers:
  - name: "test"
    type: "` + testProviderType + `"
    enabled: true
    keys:
      - key: "${` + testKey + `}"

logging:
  level: "info"
  format: "text"
`

	cfg, err := config.LoadFromReader(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("config.LoadFromReader failed: %v", err)
	}

	// Verify environment variable was expanded in server config
	if cfg.Server.APIKey != testValue {
		t.Errorf("Expected api_key=%s, got %s", testValue, cfg.Server.APIKey)
	}

	// Verify environment variable was expanded in provider key
	if len(cfg.Providers) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(cfg.Providers))
	}

	if len(cfg.Providers[0].Keys) != 1 {
		t.Fatalf("Expected 1 key, got %d", len(cfg.Providers[0].Keys))
	}

	if cfg.Providers[0].Keys[0].Key != testValue {
		t.Errorf("Expected provider key=%s, got %s", testValue, cfg.Providers[0].Keys[0].Key)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	t.Parallel()

	yamlContent := `
server:
  listen: "127.0.0.1:8787
  # Missing closing quote above
  timeout_ms: not_a_number
`

	_, err := config.LoadFromReader(strings.NewReader(yamlContent))
	if err == nil {
		t.Fatal("Expected error for invalid YAML, got nil")
	}

	if !strings.Contains(err.Error(), "failed to parse config YAML") {
		t.Errorf("Expected parse error message, got: %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()

	_, err := config.Load("/nonexistent/path/to/config.yaml")
	if err == nil {
		t.Fatal("Expected error for missing file, got nil")
	}

	if !strings.Contains(err.Error(), "failed to open config file") {
		t.Errorf("Expected open error message, got: %v", err)
	}
}

func TestLoadServerAPIKey(t *testing.T) {
	t.Parallel()

	yamlContent := `server:
  listen: "` + defaultListenAddr + `"
  api_key: "my-secret-key"

providers: []

logging:
  level: "info"
  format: "text"
`

	cfg, err := config.LoadFromReader(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("config.LoadFromReader failed: %v", err)
	}

	if cfg.Server.APIKey != "my-secret-key" {
		t.Errorf("Expected api_key=my-secret-key, got %s", cfg.Server.APIKey)
	}
}

func TestLoadProviderModels(t *testing.T) {
	t.Parallel()

	yamlContent := `server:
  listen: "` + defaultListenAddr + `"

providers:
  - name: "anthropic-primary"
    type: "` + testProviderType + `"
    enabled: true
    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"
    keys:
      - key: "sk-ant-test"

logging:
  level: "info"
`

	cfg, err := config.LoadFromReader(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("config.LoadFromReader failed: %v", err)
	}

	if len(cfg.Providers) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(cfg.Providers))
	}

	provider := cfg.Providers[0]
	if len(provider.Models) != 3 {
		t.Fatalf("Expected 3 models, got %d", len(provider.Models))
	}

	expectedModels := []string{
		"claude-sonnet-4-5-20250514",
		"claude-opus-4-5-20250514",
		"claude-haiku-3-5-20241022",
	}

	for i, expected := range expectedModels {
		if provider.Models[i] != expected {
			t.Errorf("Expected model[%d]=%s, got %s", i, expected, provider.Models[i])
		}
	}
}

func TestLoadProviderModelsEmpty(t *testing.T) {
	t.Parallel()

	yamlContent := `server:
  listen: "` + defaultListenAddr + `"

providers:
  - name: "` + testProviderType + `"
    type: "` + testProviderType + `"
    enabled: true
    keys:
      - key: "sk-ant-test"

logging:
  level: "info"
`

	cfg, err := config.LoadFromReader(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("config.LoadFromReader failed: %v", err)
	}

	if len(cfg.Providers) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(cfg.Providers))
	}

	// Models should be empty (nil or empty slice)
	if len(cfg.Providers[0].Models) != 0 {
		t.Errorf("Expected empty models, got %d", len(cfg.Providers[0].Models))
	}
}

func TestLoadMultipleProvidersWithModels(t *testing.T) {
	t.Parallel()

	yamlContent := `server:
  listen: "` + defaultListenAddr + `"

providers:
  - name: "anthropic-primary"
    type: "` + testProviderType + `"
    enabled: true
    models:
      - "claude-sonnet-4-5-20250514"
    keys:
      - key: "sk-ant-test"
  - name: "zai-primary"
    type: "zai"
    enabled: true
    models:
      - "glm-4"
      - "glm-4-plus"
    keys:
      - key: "zai-key"

logging:
  level: "info"
`

	cfg, err := config.LoadFromReader(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("config.LoadFromReader failed: %v", err)
	}

	if len(cfg.Providers) != 2 {
		t.Fatalf("Expected 2 providers, got %d", len(cfg.Providers))
	}

	// First provider
	if len(cfg.Providers[0].Models) != 1 {
		t.Errorf("Expected 1 model for anthropic, got %d", len(cfg.Providers[0].Models))
	}
	if cfg.Providers[0].Models[0] != "claude-sonnet-4-5-20250514" {
		t.Errorf("Expected claude-sonnet-4-5-20250514, got %s", cfg.Providers[0].Models[0])
	}

	// Second provider
	if len(cfg.Providers[1].Models) != 2 {
		t.Errorf("Expected 2 models for zai, got %d", len(cfg.Providers[1].Models))
	}
	if cfg.Providers[1].Models[0] != "glm-4" {
		t.Errorf("Expected glm-4, got %s", cfg.Providers[1].Models[0])
	}
	if cfg.Providers[1].Models[1] != "glm-4-plus" {
		t.Errorf("Expected glm-4-plus, got %s", cfg.Providers[1].Models[1])
	}
}

func TestLoadTOMLFormat(t *testing.T) {
	t.Parallel()

	tomlContent := `[server]
listen = "` + defaultListenAddr + `"
timeout_ms = 60000
max_concurrent = 10
api_key = "test-key"

[[providers]]
name = "` + testProviderType + `"
type = "` + testProviderType + `"
enabled = true

[[providers.keys]]
key = "sk-ant-test"
rpm_limit = 60
tpm_limit = 100000

[logging]
level = "info"
format = "json"
`

	cfg, err := config.LoadFromReaderWithFormat(strings.NewReader(tomlContent), config.FormatTOML)
	if err != nil {
		t.Fatalf("config.LoadFromReaderWithFormat failed: %v", err)
	}

	assertServerConfig(t, defaultListenAddr, 60000, 10, "test-key", cfg)
	key := assertProviderConfig(t, testProviderType, testProviderType, true, cfg, 0)
	assertKeyConfig(t, "sk-ant-test", 60, 100000, key)
	assertLoggingConfig(t, cfg)
}

func TestLoadTOMLEnvironmentExpansion(t *testing.T) {
	t.Parallel()

	// Set a test environment variable
	testKey := "TEST_TOML_API_KEY_12345"
	testValue := "sk-toml-test-value"
	if err := os.Setenv(testKey, testValue); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	defer func() {
		if err := os.Unsetenv(testKey); err != nil {
			t.Fatalf("Failed to unset environment variable: %v", err)
		}
	}()

	tomlContent := `[server]
listen = "` + defaultListenAddr + `"
api_key = "${` + testKey + `}"

[[providers]]
name = "test"
type = "` + testProviderType + `"
enabled = true

[[providers.keys]]
key = "${` + testKey + `}"

[logging]
level = "info"
format = "text"
`

	cfg, err := config.LoadFromReaderWithFormat(strings.NewReader(tomlContent), config.FormatTOML)
	if err != nil {
		t.Fatalf("config.LoadFromReaderWithFormat failed: %v", err)
	}

	// Verify environment variable was expanded in server config
	if cfg.Server.APIKey != testValue {
		t.Errorf("Expected api_key=%s, got %s", testValue, cfg.Server.APIKey)
	}

	// Verify environment variable was expanded in provider key
	if len(cfg.Providers) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(cfg.Providers))
	}

	if len(cfg.Providers[0].Keys) != 1 {
		t.Fatalf("Expected 1 key, got %d", len(cfg.Providers[0].Keys))
	}

	if cfg.Providers[0].Keys[0].Key != testValue {
		t.Errorf("Expected provider key=%s, got %s", testValue, cfg.Providers[0].Keys[0].Key)
	}
}

func TestLoadTOMLFile(t *testing.T) {
	t.Parallel()

	// Create a temporary TOML file
	tmpDir := t.TempDir()
	tomlPath := tmpDir + "/config.toml"

	tomlContent := `[server]
listen = "` + defaultListenAddr + `"

[[providers]]
name = "` + testProviderType + `"
type = "` + testProviderType + `"
enabled = true

[[providers.keys]]
key = "sk-ant-test"

[logging]
level = "info"
`

	if err := os.WriteFile(tomlPath, []byte(tomlContent), 0o600); err != nil {
		t.Fatalf("Failed to write temp TOML file: %v", err)
	}

	cfg, err := config.Load(tomlPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}

	if cfg.Server.Listen != defaultListenAddr {
		t.Errorf("Expected listen=%s, got %s", defaultListenAddr, cfg.Server.Listen)
	}

	if len(cfg.Providers) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(cfg.Providers))
	}

	if cfg.Providers[0].Name != testProviderType {
		t.Errorf("Expected provider name=%s, got %s", testProviderType, cfg.Providers[0].Name)
	}
}

func TestLoadUnsupportedFormat(t *testing.T) {
	t.Parallel()

	_, err := config.Load("/path/to/config.json")
	if err == nil {
		t.Fatal("Expected error for unsupported format, got nil")
	}

	// Check it's an UnsupportedFormatError using errors.As
	var unsupportedErr *config.UnsupportedFormatError
	if !errors.As(err, &unsupportedErr) {
		t.Fatalf("Expected config.UnsupportedFormatError, got %T: %v", err, err)
	}

	if unsupportedErr.Extension != ".json" {
		t.Errorf("Expected extension=.json, got %s", unsupportedErr.Extension)
	}

	if !strings.Contains(err.Error(), "unsupported config format") {
		t.Errorf("Expected unsupported format error message, got: %v", err)
	}

	if !strings.Contains(err.Error(), ".yaml, .yml, .toml") {
		t.Errorf("Expected supported formats in error message, got: %v", err)
	}
}

func TestLoadUnsupportedFormatNoExtension(t *testing.T) {
	t.Parallel()

	_, err := config.Load("/path/to/config")
	if err == nil {
		t.Fatal("Expected error for file without extension, got nil")
	}

	var unsupportedErr *config.UnsupportedFormatError
	if !errors.As(err, &unsupportedErr) {
		t.Fatalf("Expected config.UnsupportedFormatError, got %T: %v", err, err)
	}

	if unsupportedErr.Extension != "" {
		t.Errorf("Expected empty extension, got %s", unsupportedErr.Extension)
	}
}

func TestDetectFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		expected config.Format
		wantErr  bool
	}{
		{"config.yaml", config.FormatYAML, false},
		{"config.yml", config.FormatYAML, false},
		{"config.YAML", config.FormatYAML, false},
		{"config.YML", config.FormatYAML, false},
		{"config.toml", config.FormatTOML, false},
		{"config.TOML", config.FormatTOML, false},
		{"/path/to/config.yaml", config.FormatYAML, false},
		{"/path/to/config.toml", config.FormatTOML, false},
		{"config.json", "", true},
		{"config.xml", "", true},
		{"config", "", true},
		{"", "", true},
	}

	for _, testCase := range tests {
		t.Run(testCase.path, func(t *testing.T) {
			t.Parallel()
			format, err := config.DetectFormat(testCase.path)
			if testCase.wantErr {
				if err == nil {
					t.Errorf("config.DetectFormat(%q) expected error, got nil", testCase.path)
				}
			} else {
				if err != nil {
					t.Errorf("config.DetectFormat(%q) unexpected error: %v", testCase.path, err)
				}
				if format != testCase.expected {
					t.Errorf("config.DetectFormat(%q) = %v, want %v", testCase.path, format, testCase.expected)
				}
			}
		})
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	t.Parallel()

	tomlContent := `
[server]
listen = "127.0.0.1:8787
# Missing closing quote above
`

	_, err := config.LoadFromReaderWithFormat(strings.NewReader(tomlContent), config.FormatTOML)
	if err == nil {
		t.Fatal("Expected error for invalid TOML, got nil")
	}

	if !strings.Contains(err.Error(), "failed to parse config TOML") {
		t.Errorf("Expected parse error message, got: %v", err)
	}
}
