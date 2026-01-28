package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunConfigCCInitNewSettings(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var

	// Create a temp directory to use as HOME
	tmpDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer func() { os.Setenv("HOME", origHome) }()

	os.Setenv("HOME", tmpDir)

	// Create a mock command with the proxy-url flag
	cmd := &cobra.Command{}
	cmd.Flags().String("proxy-url", "http://127.0.0.1:8787", "cc-relay proxy URL")

	// runConfigCCInit should create settings file
	err := runConfigCCInit(cmd, nil)
	if err != nil {
		t.Fatalf("runConfigCCInit failed: %v", err)
	}

	// Verify settings file was created
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Error("Expected settings.json to be created")
	}

	// Verify content
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	env, ok := settings["env"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected env key in settings")
	}

	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:8787" {
		t.Errorf("Expected ANTHROPIC_BASE_URL to be http://127.0.0.1:8787, got %v", env["ANTHROPIC_BASE_URL"])
	}

	if env["ANTHROPIC_AUTH_TOKEN"] != "managed-by-cc-relay" {
		t.Errorf("Expected ANTHROPIC_AUTH_TOKEN to be managed-by-cc-relay, got %v", env["ANTHROPIC_AUTH_TOKEN"])
	}
}

func TestRunConfigCCInitExistingSettings(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var

	// Create a temp directory to use as HOME
	tmpDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer func() { os.Setenv("HOME", origHome) }()

	os.Setenv("HOME", tmpDir)

	// Create existing settings file with other settings
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatal(err)
	}

	existingSettings := map[string]interface{}{
		"theme": "dark",
		"env": map[string]interface{}{
			"OTHER_VAR": "other-value",
		},
	}
	existingData, err := json.MarshalIndent(existingSettings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, existingData, 0o600); err != nil {
		t.Fatal(err)
	}

	// Create a mock command with the proxy-url flag and set it
	cmd := &cobra.Command{}
	cmd.Flags().String("proxy-url", "http://127.0.0.1:8787", "cc-relay proxy URL")
	_ = cmd.Flags().Set("proxy-url", "http://127.0.0.1:8787")

	// runConfigCCInit should update settings file
	err = runConfigCCInit(cmd, nil)
	if err != nil {
		t.Fatalf("runConfigCCInit failed: %v", err)
	}

	// Verify content preserves existing settings
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	// Check theme is preserved
	if settings["theme"] != "dark" {
		t.Errorf("Expected theme to be preserved, got %v", settings["theme"])
	}

	env, ok := settings["env"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected env key in settings")
	}

	// Check existing env var is preserved
	if env["OTHER_VAR"] != "other-value" {
		t.Errorf("Expected OTHER_VAR to be preserved, got %v", env["OTHER_VAR"])
	}

	// Check new env vars are added
	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:8787" {
		t.Errorf("Expected ANTHROPIC_BASE_URL to be set, got %v", env["ANTHROPIC_BASE_URL"])
	}
}

func TestRunConfigCCInitCustomProxyURL(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var

	// Create a temp directory to use as HOME
	tmpDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer func() { os.Setenv("HOME", origHome) }()

	os.Setenv("HOME", tmpDir)

	// Create a mock command with a custom proxy-url
	cmd := &cobra.Command{}
	cmd.Flags().String("proxy-url", "http://custom.host:9999", "cc-relay proxy URL")

	err := runConfigCCInit(cmd, nil)
	if err != nil {
		t.Fatalf("runConfigCCInit failed: %v", err)
	}

	// Verify custom URL was used
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	env := settings["env"].(map[string]interface{})
	if env["ANTHROPIC_BASE_URL"] != "http://custom.host:9999" {
		t.Errorf("Expected custom ANTHROPIC_BASE_URL, got %v", env["ANTHROPIC_BASE_URL"])
	}
}

func TestRunConfigCCRemoveExistingSettings(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var

	// Create a temp directory to use as HOME
	tmpDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer func() { os.Setenv("HOME", origHome) }()

	os.Setenv("HOME", tmpDir)

	// Create existing settings file with cc-relay config
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatal(err)
	}

	existingSettings := map[string]interface{}{
		"theme": "dark",
		"env": map[string]interface{}{
			"ANTHROPIC_BASE_URL":   "http://127.0.0.1:8787",
			"ANTHROPIC_AUTH_TOKEN": "managed-by-cc-relay",
			"OTHER_VAR":            "other-value",
		},
	}
	existingData, err := json.MarshalIndent(existingSettings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, existingData, 0o600); err != nil {
		t.Fatal(err)
	}

	// runConfigCCRemove should remove cc-relay env vars
	err = runConfigCCRemove(nil, nil)
	if err != nil {
		t.Fatalf("runConfigCCRemove failed: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	// Check theme is preserved
	if settings["theme"] != "dark" {
		t.Errorf("Expected theme to be preserved, got %v", settings["theme"])
	}

	env, ok := settings["env"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected env key in settings")
	}

	// Check cc-relay env vars are removed
	if _, exists := env["ANTHROPIC_BASE_URL"]; exists {
		t.Error("Expected ANTHROPIC_BASE_URL to be removed")
	}
	if _, exists := env["ANTHROPIC_AUTH_TOKEN"]; exists {
		t.Error("Expected ANTHROPIC_AUTH_TOKEN to be removed")
	}

	// Check other env var is preserved
	if env["OTHER_VAR"] != "other-value" {
		t.Errorf("Expected OTHER_VAR to be preserved, got %v", env["OTHER_VAR"])
	}
}

func TestRunConfigCCRemoveNoSettings(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var

	// Create a temp directory to use as HOME
	tmpDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer func() { os.Setenv("HOME", origHome) }()

	os.Setenv("HOME", tmpDir)

	// runConfigCCRemove should succeed (nothing to remove)
	err := runConfigCCRemove(nil, nil)
	if err != nil {
		t.Errorf("Expected success when no settings file exists, got error: %v", err)
	}
}

func TestRunConfigCCRemoveNoEnvSection(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var

	// Create a temp directory to use as HOME
	tmpDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer func() { os.Setenv("HOME", origHome) }()

	os.Setenv("HOME", tmpDir)

	// Create settings file without env section
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatal(err)
	}

	existingSettings := map[string]interface{}{
		"theme": "dark",
	}
	existingData, err := json.MarshalIndent(existingSettings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, existingData, 0o600); err != nil {
		t.Fatal(err)
	}

	// runConfigCCRemove should succeed (nothing to remove)
	err = runConfigCCRemove(nil, nil)
	if err != nil {
		t.Errorf("Expected success when no env section exists, got error: %v", err)
	}
}

func TestRunConfigCCRemoveNoCCRelayConfig(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var

	// Create a temp directory to use as HOME
	tmpDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer func() { os.Setenv("HOME", origHome) }()

	os.Setenv("HOME", tmpDir)

	// Create settings file with env but no cc-relay vars
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatal(err)
	}

	existingSettings := map[string]interface{}{
		"env": map[string]interface{}{
			"OTHER_VAR": "other-value",
		},
	}
	existingData, err := json.MarshalIndent(existingSettings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, existingData, 0o600); err != nil {
		t.Fatal(err)
	}

	// runConfigCCRemove should succeed (nothing cc-relay specific to remove)
	err = runConfigCCRemove(nil, nil)
	if err != nil {
		t.Errorf("Expected success when no cc-relay config exists, got error: %v", err)
	}

	// Verify other env vars are preserved
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	env := settings["env"].(map[string]interface{})
	if env["OTHER_VAR"] != "other-value" {
		t.Errorf("Expected OTHER_VAR to be preserved, got %v", env["OTHER_VAR"])
	}
}

func TestRunConfigCCRemoveRemovesEmptyEnv(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var

	// Create a temp directory to use as HOME
	tmpDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer func() { os.Setenv("HOME", origHome) }()

	os.Setenv("HOME", tmpDir)

	// Create settings file with only cc-relay env vars
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatal(err)
	}

	existingSettings := map[string]interface{}{
		"theme": "dark",
		"env": map[string]interface{}{
			"ANTHROPIC_BASE_URL":   "http://127.0.0.1:8787",
			"ANTHROPIC_AUTH_TOKEN": "managed-by-cc-relay",
		},
	}
	existingData, err := json.MarshalIndent(existingSettings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, existingData, 0o600); err != nil {
		t.Fatal(err)
	}

	// runConfigCCRemove should remove cc-relay vars and empty env section
	err = runConfigCCRemove(nil, nil)
	if err != nil {
		t.Fatalf("runConfigCCRemove failed: %v", err)
	}

	// Verify env section is removed when empty
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	// After removal, the env section should not exist (was empty)
	// or if it does, it should be empty
	if env, exists := settings["env"]; exists {
		if envMap, ok := env.(map[string]interface{}); ok && len(envMap) > 0 {
			t.Errorf("Expected env section to be removed or empty, got %v", envMap)
		}
	}

	// Check theme is still there
	if settings["theme"] != "dark" {
		t.Errorf("Expected theme to be preserved, got %v", settings["theme"])
	}
}
