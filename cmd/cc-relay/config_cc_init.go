package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var configCCInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Configure Claude Code to use cc-relay",
	Long:  `Add cc-relay proxy environment variables to ~/.claude/settings.json`,
	RunE:  runConfigCCInit,
}

func init() {
	configCCCmd.AddCommand(configCCInitCmd)
	configCCInitCmd.Flags().String("proxy-url", "http://127.0.0.1:8787", "cc-relay proxy URL")
}

func runConfigCCInit(cmd *cobra.Command, _ []string) error {
	proxyURL, err := cmd.Flags().GetString("proxy-url")
	if err != nil {
		return fmt.Errorf("failed to get proxy-url flag: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")

	// Read existing settings or create new
	var settings map[string]interface{}

	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("failed to parse settings.json: %w", err)
		}
	} else {
		settings = make(map[string]interface{})
	}

	// Get or create env map
	env, ok := settings["env"].(map[string]interface{})
	if !ok {
		env = make(map[string]interface{})
	}

	// Set proxy env vars
	env["ANTHROPIC_BASE_URL"] = proxyURL
	env["ANTHROPIC_AUTH_TOKEN"] = "managed-by-cc-relay"

	settings["env"] = env

	// Create directory if needed
	dir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	// Write settings with pretty formatting
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write settings.json: %w", err)
	}

	fmt.Printf("Claude Code configured to use cc-relay at %s\n", proxyURL)
	fmt.Printf("Settings file: %s\n", settingsPath)
	fmt.Println("\nEnvironment variables added:")
	fmt.Printf("  ANTHROPIC_BASE_URL=%s\n", proxyURL)
	fmt.Println("  ANTHROPIC_AUTH_TOKEN=managed-by-cc-relay")
	fmt.Println("\nRestart Claude Code for changes to take effect.")

	return nil
}
