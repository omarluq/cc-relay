package main

import (
	"testing"

	"github.com/omarluq/cc-relay/internal/config"
)

func TestValidateConfig_Valid(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: "localhost:8787",
			APIKey: "test-key",
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

func TestValidateConfig_NoListen(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			APIKey: "test-key",
		},
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Error("Expected error for missing listen address")
	}

	if err != nil && err.Error() != "server.listen is required" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestValidateConfig_NoAPIKey(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: "localhost:8787",
		},
	}

	err := validateConfig(cfg)
	if err == nil {
		t.Error("Expected error for missing API key")
	}

	if err != nil && err.Error() != "server.api_key is required" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestValidateConfig_NoEnabledProvider(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: "localhost:8787",
			APIKey: "test-key",
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
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestValidateConfig_ProviderNoKeys(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: "localhost:8787",
			APIKey: "test-key",
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
