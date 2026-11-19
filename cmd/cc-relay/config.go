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

func runConfigValidate(cmd *cobra.Command, _ []string) error {
	// Determine config path
	configPath := cfgFile
	if configPath == "" {
		configPath = findConfigFileForValidate()
	}

	if err := validateConfigAtPath(configPath); err != nil {
		cmd.Printf("✗ Config validation failed: %s\n", err)
		return err
	}

	cmd.Printf("✓ %s is valid\n", configPath)

	return nil
}

// validateConfigAtPath validates the config file at the given path.
func validateConfigAtPath(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	return validateConfig(cfg)
}

// validateConfig checks the CLI's enabled-provider requirement after config.Load
// has performed schema validation.
func validateConfig(cfg *config.Config) error {
	for i := range cfg.Providers {
		if cfg.Providers[i].Enabled {
			return nil
		}
	}

	return fmt.Errorf("no enabled providers configured")
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
