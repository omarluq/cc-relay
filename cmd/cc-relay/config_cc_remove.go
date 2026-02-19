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

func runConfigCCRemove(cmd *cobra.Command, _ []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	removed, settingsPath, err := removeCCRelayConfig(home)
	if err != nil {
		return err
	}

	if removed == nil {
		cmd.Println("No cc-relay configuration found in Claude Code settings.")
		return nil
	}

	printRemovalSummary(cmd, settingsPath, removed)
	return nil
}

// removeCCRelayConfig removes cc-relay env vars from Claude Code settings.
// Returns the list of removed keys and the settings path, or nil if nothing was removed.
func removeCCRelayConfig(home string) (removed []string, settingsPath string, err error) {
	settingsPath = filepath.Join(home, ".claude", "settings.json")

	var settings map[string]any
	settings, err = readClaudeSettings(settingsPath)
	if errors.Is(err, ErrSettingsNotFound) {
		return nil, settingsPath, nil
	}
	if err != nil {
		return nil, "", err
	}

	env, ok := settings["env"].(map[string]any)
	if !ok {
		return nil, settingsPath, nil
	}

	removed = removeProxyEnvVars(env)
	if len(removed) == 0 {
		return nil, settingsPath, nil
	}

	updateSettingsEnv(settings, env)

	writeErr := writeClaudeSettings(settingsPath, settings)
	if writeErr != nil {
		return nil, "", writeErr
	}

	return removed, settingsPath, nil
}

// ErrSettingsNotFound indicates the Claude Code settings file doesn't exist.
var ErrSettingsNotFound = errors.New("settings file not found")

// readClaudeSettings reads and parses the Claude Code settings.json.
// Returns ErrSettingsNotFound if the file doesn't exist.
func readClaudeSettings(path string) (map[string]any, error) {
	// Clean the path to avoid directory traversal issues
	path = filepath.Clean(path)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSettingsNotFound
		}
		return nil, fmt.Errorf("failed to read settings.json: %w", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings.json: %w", err)
	}
	return settings, nil
}

// removeProxyEnvVars removes cc-relay environment variables and returns the list of removed keys.
func removeProxyEnvVars(env map[string]any) []string {
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
func updateSettingsEnv(settings, env map[string]any) {
	if len(env) == 0 {
		delete(settings, "env")
	} else {
		settings["env"] = env
	}
}

// writeClaudeSettings writes the settings back to disk.
func writeClaudeSettings(path string, settings map[string]any) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write settings.json: %w", err)
	}
	return nil
}

// printRemovalSummary prints a summary of the removed configuration.
func printRemovalSummary(cmd *cobra.Command, settingsPath string, removed []string) {
	cmd.Println("Removed cc-relay configuration from Claude Code:")
	for _, key := range removed {
		cmd.Printf("  - %s\n", key)
	}
	cmd.Printf("\nSettings file: %s\n", settingsPath)
	cmd.Println("Restart Claude Code for changes to take effect.")
}
