package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/omarluq/cc-relay/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long: `Validate the configuration file without starting the server.
Checks YAML syntax, required fields, and provider configurations.`,
	RunE: runConfigValidate,
}

func init() {
	configCmd.AddCommand(configValidateCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigValidate(_ *cobra.Command, _ []string) error {
	// Determine config path
	configPath := cfgFile
	if configPath == "" {
		configPath = findConfigFileForValidate()
	}

	// Load and validate config
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("✗ Config validation failed: %s\n", err)
		return err
	}

	// Additional validation checks
	if err := validateConfig(cfg); err != nil {
		fmt.Printf("✗ Config validation failed: %s\n", err)
		return err
	}

	fmt.Printf("✓ %s is valid\n", configPath)

	return nil
}

// validateConfig performs semantic validation beyond YAML parsing.
func validateConfig(cfg *config.Config) error {
	// Check server config
	if cfg.Server.Listen == "" {
		return fmt.Errorf("server.listen is required")
	}

	if cfg.Server.APIKey == "" {
		return fmt.Errorf("server.api_key is required")
	}

	// Check at least one enabled provider
	hasEnabledProvider := false

	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		if p.Enabled {
			hasEnabledProvider = true
			// Check provider has at least one key
			if len(p.Keys) == 0 {
				return fmt.Errorf("provider %s has no API keys configured", p.Name)
			}
		}
	}

	if !hasEnabledProvider {
		return fmt.Errorf("no enabled providers configured")
	}

	return nil
}

// findConfigFileForValidate searches for config file in default locations.
func findConfigFileForValidate() string {
	// Check current directory
	if _, err := os.Stat(defaultConfigFile); err == nil {
		return defaultConfigFile
	}
	// Check ~/.config/cc-relay/
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		p := filepath.Join(home, ".config", "cc-relay", defaultConfigFile)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return defaultConfigFile
}
