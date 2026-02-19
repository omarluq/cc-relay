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

// findConfigIn searches for defaultConfigFile in workDir, returning full path
// if found, or just the default name if not found. For testing without global state.
func findConfigIn(workDir string) string {
	p := filepath.Join(workDir, defaultConfigFile)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return defaultConfigFile
}

// findConfigInWithHome searches for defaultConfigFile in workDir then
// homeDir/.config/cc-relay/. For testing without global state.
func findConfigInWithHome(workDir, homeDir string) string {
	// Check current directory
	p := filepath.Join(workDir, defaultConfigFile)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	// Check homeDir/.config/cc-relay/
	homeConfig := filepath.Join(homeDir, ".config", "cc-relay", defaultConfigFile)
	if _, err := os.Stat(homeConfig); err == nil {
		return homeConfig
	}
	return defaultConfigFile
}
