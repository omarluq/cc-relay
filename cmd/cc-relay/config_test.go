package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omarluq/cc-relay/internal/cache"
	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/health"
)

const (
	defaultListenAddr = "localhost:8787"
	localListenAddr   = "127.0.0.1:8787"
	defaultAPIKey     = "test-key"
	configFileName    = "config.yaml"
	unexpectedErrFmt  = "Unexpected error: %v"
	providerAnthropic = "anthropic"
)

// Helper functions to create zero-filled config structs for testing.
func emptyRoutingConfig() config.RoutingConfig {
	return config.RoutingConfig{
		ModelMapping: nil, Strategy: "",
		DefaultProvider: "", FailoverTimeout: 0, Debug: false,
	}
}

func emptyLoggingConfig() config.LoggingConfig {
	return config.LoggingConfig{
		Level: "", Format: "", Output: "", Pretty: false,
		DebugOptions: config.DebugOptions{
			LogRequestBody: false, LogResponseHeaders: false,
			LogTLSMetrics: false, MaxBodyLogSize: 0,
		},
	}
}

func emptyHealthConfig() health.Config {
	return health.Config{
		CircuitBreaker: health.CircuitBreakerConfig{
			OpenDurationMS: 0, FailureThreshold: 0, HalfOpenProbes: 0,
		},
	}
}

func emptyCacheConfig() cache.Config {
	return cache.Config{
		Mode: "",
		Olric: cache.OlricConfig{
			DMapName: "", BindAddr: "", Environment: "",
			Addresses: nil, Peers: nil, ReplicaCount: 0,
			ReadQuorum: 0, WriteQuorum: 0,
			LeaveTimeout: 0, MemberCountQuorum: 0, Embedded: false,
		},
		Ristretto: cache.RistrettoConfig{
			NumCounters: 0, MaxCost: 0, BufferItems: 0,
		},
	}
}

func emptyAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		APIKey: "", BearerSecret: "",
		AllowBearer: false, AllowSubscription: false,
	}
}

func emptyKeyConfig(key string) config.KeyConfig {
	return config.KeyConfig{
		Key: key, RPMLimit: 0, ITPMLimit: 0,
		OTPMLimit: 0, Priority: 0, Weight: 0,
		TPMLimit: 0, //nolint:staticcheck // exhaustruct requires the deprecated field
	}
}

func emptyProviderConfig() config.ProviderConfig {
	return config.ProviderConfig{
		OutboundProxy: "",
		ModelMapping:  nil, AWSRegion: "",
		GCPProjectID: "", AzureAPIVersion: "",
		Name: "", Type: "", BaseURL: "",
		AzureDeploymentID: "", AWSAccessKeyID: "",
		AzureResourceName: "", AWSSecretAccessKey: "",
		GCPRegion: "", Keys: nil, Models: nil,
		Pooling: config.PoolingConfig{Strategy: "", Enabled: false},
		Enabled: false,
	}
}

func TestValidateConfigValid(t *testing.T) {
	t.Parallel()

	provider := emptyProviderConfig()
	provider.Name = providerAnthropic
	provider.Type = providerAnthropic
	provider.Enabled = true
	provider.Keys = []config.KeyConfig{emptyKeyConfig("test-api-key")}

	cfg := &config.Config{
		Routing: emptyRoutingConfig(),
		Logging: emptyLoggingConfig(),
		Health:  emptyHealthConfig(),
		Cache:   emptyCacheConfig(),
		Server: config.ServerConfig{
			OutboundProxy: "",
			Listen:        defaultListenAddr,
			APIKey:        defaultAPIKey,
			Auth:          emptyAuthConfig(),
			TimeoutMS:     0,
			MaxConcurrent: 0,
			MaxBodyBytes:  0,
			EnableHTTP2:   false,
		},
		Providers: []config.ProviderConfig{provider},
	}

	if err := validateConfig(cfg); err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidateConfigNoEnabledProvider(t *testing.T) {
	t.Parallel()

	provider := emptyProviderConfig()
	provider.Name = providerAnthropic
	provider.Enabled = false

	cfg := &config.Config{
		Routing: emptyRoutingConfig(),
		Logging: emptyLoggingConfig(),
		Health:  emptyHealthConfig(),
		Cache:   emptyCacheConfig(),
		Server: config.ServerConfig{
			OutboundProxy: "",
			Listen:        defaultListenAddr,
			APIKey:        defaultAPIKey,
			Auth:          emptyAuthConfig(),
			TimeoutMS:     0,
			MaxConcurrent: 0,
			MaxBodyBytes:  0,
			EnableHTTP2:   false,
		},
		Providers: []config.ProviderConfig{provider},
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
	t.Parallel()

	provider := emptyProviderConfig()
	provider.Name = providerAnthropic
	provider.Enabled = true
	provider.Keys = []config.KeyConfig{}

	cfg := &config.Config{
		Routing: emptyRoutingConfig(),
		Logging: emptyLoggingConfig(),
		Health:  emptyHealthConfig(),
		Cache:   emptyCacheConfig(),
		Server: config.ServerConfig{
			OutboundProxy: "",
			Listen:        defaultListenAddr,
			APIKey:        defaultAPIKey,
			Auth:          emptyAuthConfig(),
			TimeoutMS:     0,
			MaxConcurrent: 0,
			MaxBodyBytes:  0,
			EnableHTTP2:   false,
		},
		Providers: []config.ProviderConfig{provider},
	}

	err := validateConfig(cfg)
	if err != nil {
		t.Errorf(unexpectedErrFmt, err)
	}
}

func TestValidateConfigMultipleProviders(t *testing.T) {
	t.Parallel()

	provider1 := emptyProviderConfig()
	provider1.Name = providerAnthropic
	provider1.Type = providerAnthropic
	provider1.Enabled = true
	provider1.Keys = []config.KeyConfig{emptyKeyConfig("key1")}

	provider2 := emptyProviderConfig()
	provider2.Name = "zai"
	provider2.Type = "zai"
	provider2.Enabled = true
	provider2.Keys = []config.KeyConfig{emptyKeyConfig("key2")}

	cfg := &config.Config{
		Routing: emptyRoutingConfig(),
		Logging: emptyLoggingConfig(),
		Health:  emptyHealthConfig(),
		Cache:   emptyCacheConfig(),
		Server: config.ServerConfig{
			OutboundProxy: "",
			Listen:        defaultListenAddr,
			APIKey:        defaultAPIKey,
			Auth:          emptyAuthConfig(),
			TimeoutMS:     0,
			MaxConcurrent: 0,
			MaxBodyBytes:  0,
			EnableHTTP2:   false,
		},
		Providers: []config.ProviderConfig{provider1, provider2},
	}

	if err := validateConfig(cfg); err != nil {
		t.Errorf("Expected valid config with multiple providers, got error: %v", err)
	}
}

func TestValidateConfigEmptyProviders(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Routing: emptyRoutingConfig(),
		Logging: emptyLoggingConfig(),
		Health:  emptyHealthConfig(),
		Cache:   emptyCacheConfig(),
		Server: config.ServerConfig{
			OutboundProxy: "",
			Listen:        defaultListenAddr,
			APIKey:        defaultAPIKey,
			Auth:          emptyAuthConfig(),
			TimeoutMS:     0,
			MaxConcurrent: 0,
			MaxBodyBytes:  0,
			EnableHTTP2:   false,
		},
		Providers: []config.ProviderConfig{},
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Error("Expected error for empty providers")
	}
}

func TestRunConfigValidateValidConfig(t *testing.T) {
	t.Parallel()

	for name, auth := range map[string]string{
		"legacy API key": "  api_key: test-key\n",
		"nested API key": "  auth:\n    api_key: test-key\n",
		"no API key":     "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			configPath := filepath.Join(t.TempDir(), configFileName)
			content := "server:\n  listen: " + localListenAddr + "\n" + auth + `providers:
  - name: anthropic
    type: anthropic
    enabled: true
`
			if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := validateConfigAtPath(configPath); err != nil {
				t.Fatalf("validate config: %v", err)
			}
		})
	}
}

func TestRunConfigValidateInvalidYAML(t *testing.T) {
	t.Parallel()

	// Create a config file with invalid YAML
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, configFileName)
	if err := os.WriteFile(configPath, []byte("invalid: yaml: : content"), 0o600); err != nil {
		t.Fatal(err)
	}

	// validateConfigAtPath should fail
	err := validateConfigAtPath(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestRunConfigValidateMissingServer(t *testing.T) {
	t.Parallel()

	// Create a config file missing server section
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, configFileName)
	configContent := `
providers:
  - name: "` + providerAnthropic + `"
    type: "` + providerAnthropic + `"
    enabled: true
    keys:
      - key: "test-api-key"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatal(err)
	}

	// validateConfigAtPath should fail
	err := validateConfigAtPath(configPath)
	if err == nil {
		t.Error("Expected error for missing server section")
	}
	if err != nil && !strings.Contains(err.Error(), "server.listen is required") {
		t.Errorf(unexpectedErrFmt, err)
	}
}

func TestRunConfigValidateNonexistentFile(t *testing.T) {
	t.Parallel()

	// validateConfigAtPath should fail
	err := validateConfigAtPath("/nonexistent/path/" + configFileName)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}
