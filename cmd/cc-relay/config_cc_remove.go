package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var configCCRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove cc-relay configuration from Claude Code",
	Long:  `Remove cc-relay proxy environment variables from ~/.claude/settings.json`,
	RunE:  runConfigCCRemove,
}

func init() {
	configCCCmd.AddCommand(configCCRemoveCmd)
}

func runConfigCCRemove(_ *cobra.Command, _ []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")

	// Read existing settings
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No Claude Code settings found. Nothing to remove.")
			return nil
		}
		return fmt.Errorf("failed to read settings.json: %w", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("failed to parse settings.json: %w", err)
	}

	// Get env map
	env, ok := settings["env"].(map[string]interface{})
	if !ok {
		fmt.Println("No environment variables found in settings. Nothing to remove.")
		return nil
	}

	// Track what we removed
	removed := []string{}

	// Remove proxy env vars
	if _, exists := env["ANTHROPIC_BASE_URL"]; exists {
		delete(env, "ANTHROPIC_BASE_URL")
		removed = append(removed, "ANTHROPIC_BASE_URL")
	}

	if _, exists := env["ANTHROPIC_AUTH_TOKEN"]; exists {
		delete(env, "ANTHROPIC_AUTH_TOKEN")
		removed = append(removed, "ANTHROPIC_AUTH_TOKEN")
	}

	if len(removed) == 0 {
		fmt.Println("No cc-relay configuration found in Claude Code settings.")
		return nil
	}

	// Update settings
	if len(env) == 0 {
		delete(settings, "env")
	} else {
		settings["env"] = env
	}

	// Write settings back
	data, err = json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write settings.json: %w", err)
	}

	fmt.Println("Removed cc-relay configuration from Claude Code:")
	for _, key := range removed {
		fmt.Printf("  - %s\n", key)
	}
	fmt.Printf("\nSettings file: %s\n", settingsPath)
	fmt.Println("Restart Claude Code for changes to take effect.")

	return nil
}
