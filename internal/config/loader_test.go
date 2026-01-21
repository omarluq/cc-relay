package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad_ValidYAML(t *testing.T) {
	yamlContent := `
server:
  listen: "127.0.0.1:8787"
  timeout_ms: 60000
  max_concurrent: 10
  api_key: "test-key"

providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "sk-ant-test"
        rpm_limit: 60
        tpm_limit: 100000

logging:
  level: "info"
  format: "json"
`

	cfg, err := LoadFromReader(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	// Verify server config
	if cfg.Server.Listen != "127.0.0.1:8787" {
		t.Errorf("Expected listen=127.0.0.1:8787, got %s", cfg.Server.Listen)
	}

	if cfg.Server.TimeoutMS != 60000 {
		t.Errorf("Expected timeout_ms=60000, got %d", cfg.Server.TimeoutMS)
	}

	if cfg.Server.MaxConcurrent != 10 {
		t.Errorf("Expected max_concurrent=10, got %d", cfg.Server.MaxConcurrent)
	}

	if cfg.Server.APIKey != "test-key" {
		t.Errorf("Expected api_key=test-key, got %s", cfg.Server.APIKey)
	}

	// Verify providers
	if len(cfg.Providers) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(cfg.Providers))
	}

	provider := cfg.Providers[0]
	if provider.Name != "anthropic" {
		t.Errorf("Expected provider name=anthropic, got %s", provider.Name)
	}

	if provider.Type != "anthropic" {
		t.Errorf("Expected provider type=anthropic, got %s", provider.Type)
	}

	if !provider.Enabled {
		t.Error("Expected provider enabled=true, got false")
	}

	// Verify keys
	if len(provider.Keys) != 1 {
		t.Fatalf("Expected 1 key, got %d", len(provider.Keys))
	}

	key := provider.Keys[0]
	if key.Key != "sk-ant-test" {
		t.Errorf("Expected key=sk-ant-test, got %s", key.Key)
	}

	if key.RPMLimit != 60 {
		t.Errorf("Expected rpm_limit=60, got %d", key.RPMLimit)
	}

	if key.TPMLimit != 100000 {
		t.Errorf("Expected tpm_limit=100000, got %d", key.TPMLimit)
	}

	// Verify logging
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected logging level=info, got %s", cfg.Logging.Level)
	}

	if cfg.Logging.Format != "json" {
		t.Errorf("Expected logging format=json, got %s", cfg.Logging.Format)
	}
}

func TestLoad_EnvironmentExpansion(t *testing.T) {
	// Set a test environment variable
	testKey := "TEST_API_KEY_12345"
	testValue := "sk-test-value"
	os.Setenv(testKey, testValue)

	defer os.Unsetenv(testKey)

	yamlContent := `
server:
  listen: "127.0.0.1:8787"
  api_key: "${` + testKey + `}"

providers:
  - name: "test"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${` + testKey + `}"

logging:
  level: "info"
  format: "text"
`

	cfg, err := LoadFromReader(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
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

func TestLoad_InvalidYAML(t *testing.T) {
	yamlContent := `
server:
  listen: "127.0.0.1:8787
  # Missing closing quote above
  timeout_ms: not_a_number
`

	_, err := LoadFromReader(strings.NewReader(yamlContent))
	if err == nil {
		t.Fatal("Expected error for invalid YAML, got nil")
	}

	if !strings.Contains(err.Error(), "failed to parse config YAML") {
		t.Errorf("Expected parse error message, got: %v", err)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/to/config.yaml")
	if err == nil {
		t.Fatal("Expected error for missing file, got nil")
	}

	if !strings.Contains(err.Error(), "failed to open config file") {
		t.Errorf("Expected open error message, got: %v", err)
	}
}

func TestLoad_ServerAPIKey(t *testing.T) {
	yamlContent := `
server:
  listen: "127.0.0.1:8787"
  api_key: "my-secret-key"

providers: []

logging:
  level: "info"
  format: "text"
`

	cfg, err := LoadFromReader(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	if cfg.Server.APIKey != "my-secret-key" {
		t.Errorf("Expected api_key=my-secret-key, got %s", cfg.Server.APIKey)
	}
}
