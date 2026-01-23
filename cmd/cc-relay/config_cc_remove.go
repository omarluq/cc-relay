package main

import (
	"encoding/json"
	"errors"
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

// proxyEnvVars are the environment variables that cc-relay sets in Claude Code settings.
var proxyEnvVars = []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN"}

func runConfigCCRemove(_ *cobra.Command, _ []string) error {
	settingsPath, err := getClaudeSettingsPath()
	if err != nil {
		return err
	}

	settings, err := readClaudeSettings(settingsPath)
	if errors.Is(err, ErrSettingsNotFound) {
		fmt.Println("No Claude Code settings found. Nothing to remove.")
		return nil
	}
	if err != nil {
		return err
	}

	env, ok := settings["env"].(map[string]interface{})
	if !ok {
		fmt.Println("No environment variables found in settings. Nothing to remove.")
		return nil
	}

	removed := removeProxyEnvVars(env)
	if len(removed) == 0 {
		fmt.Println("No cc-relay configuration found in Claude Code settings.")
		return nil
	}

	updateSettingsEnv(settings, env)

	if err := writeClaudeSettings(settingsPath, settings); err != nil {
		return err
	}

	printRemovalSummary(settingsPath, removed)
	return nil
}

// getClaudeSettingsPath returns the path to Claude Code settings.json.
func getClaudeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// ErrSettingsNotFound indicates the Claude Code settings file doesn't exist.
var ErrSettingsNotFound = errors.New("settings file not found")

// readClaudeSettings reads and parses the Claude Code settings.json.
// Returns ErrSettingsNotFound if the file doesn't exist.
func readClaudeSettings(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSettingsNotFound
		}
		return nil, fmt.Errorf("failed to read settings.json: %w", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings.json: %w", err)
	}
	return settings, nil
}

// removeProxyEnvVars removes cc-relay environment variables and returns the list of removed keys.
func removeProxyEnvVars(env map[string]interface{}) []string {
	var removed []string
	for _, key := range proxyEnvVars {
		if _, exists := env[key]; exists {
			delete(env, key)
			removed = append(removed, key)
		}
	}
	return removed
}

// updateSettingsEnv updates the settings map with the modified env.
func updateSettingsEnv(settings, env map[string]interface{}) {
	if len(env) == 0 {
		delete(settings, "env")
	} else {
		settings["env"] = env
	}
}

// writeClaudeSettings writes the settings back to disk.
func writeClaudeSettings(path string, settings map[string]interface{}) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write settings.json: %w", err)
	}
	return nil
}

// printRemovalSummary prints a summary of the removed configuration.
func printRemovalSummary(settingsPath string, removed []string) {
	fmt.Println("Removed cc-relay configuration from Claude Code:")
	for _, key := range removed {
		fmt.Printf("  - %s\n", key)
	}
	fmt.Printf("\nSettings file: %s\n", settingsPath)
	fmt.Println("Restart Claude Code for changes to take effect.")
}
