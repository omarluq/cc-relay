package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const (
	ccRelayProxyURL          = "http://127.0.0.1:8787"
	claudeDirName            = ".claude"
	settingsFileName         = "settings.json"
	applyCCRelayConfigErrFmt = "applyCCRelayConfig failed: %v"
	readSettingsErrFmt       = "Failed to read settings.json: %v"
	parseSettingsErrFmt      = "Failed to parse settings.json: %v"
	expectedEnvKeyMsg        = "Expected env key in settings"
	managedByCCRelayValue    = "managed-by-cc-relay"
	otherEnvValue            = "other-value"
	themePreservedErrFmt     = "Expected theme to be preserved, got %v"
	otherVarPreservedErrFmt  = "Expected OTHER_VAR to be preserved, got %v"
	themeDark                = "dark"
)

func settingsPathForHome(home string) string {
	return filepath.Join(home, claudeDirName, settingsFileName)
}

func readSettings(t *testing.T, home string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(settingsPathForHome(home))
	if err != nil {
		t.Fatalf(readSettingsErrFmt, err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf(parseSettingsErrFmt, err)
	}

	return settings
}

func writeSettings(t *testing.T, home string, settings map[string]any) {
	t.Helper()
	claudeDir := filepath.Join(home, claudeDirName)
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatal(err)
	}

	existingData, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPathForHome(home), existingData, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestApplyCCRelayConfigNewSettings(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	settingsPath, err := applyCCRelayConfig(tmpDir, ccRelayProxyURL)
	if err != nil {
		t.Fatalf(applyCCRelayConfigErrFmt, err)
	}

	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("Expected settings.json to be created: %v", err)
	}

	settings := readSettings(t, tmpDir)

	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatal(expectedEnvKeyMsg)
	}

	if env["ANTHROPIC_BASE_URL"] != ccRelayProxyURL {
		t.Errorf("Expected ANTHROPIC_BASE_URL to be %s, got %v", ccRelayProxyURL, env["ANTHROPIC_BASE_URL"])
	}

	if env["ANTHROPIC_AUTH_TOKEN"] != managedByCCRelayValue {
		t.Errorf("Expected ANTHROPIC_AUTH_TOKEN to be %s, got %v", managedByCCRelayValue, env["ANTHROPIC_AUTH_TOKEN"])
	}
}

func TestApplyCCRelayConfigExistingSettings(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	existingSettings := map[string]any{
		"theme": themeDark,
		"env": map[string]any{
			"OTHER_VAR": otherEnvValue,
		},
	}
	writeSettings(t, tmpDir, existingSettings)

	_, err := applyCCRelayConfig(tmpDir, ccRelayProxyURL)
	if err != nil {
		t.Fatalf(applyCCRelayConfigErrFmt, err)
	}

	settings := readSettings(t, tmpDir)

	if settings["theme"] != themeDark {
		t.Errorf(themePreservedErrFmt, settings["theme"])
	}

	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatal(expectedEnvKeyMsg)
	}

	if env["OTHER_VAR"] != otherEnvValue {
		t.Errorf(otherVarPreservedErrFmt, env["OTHER_VAR"])
	}

	if env["ANTHROPIC_BASE_URL"] != ccRelayProxyURL {
		t.Errorf("Expected ANTHROPIC_BASE_URL to be set, got %v", env["ANTHROPIC_BASE_URL"])
	}
}

func TestApplyCCRelayConfigCustomProxyURL(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	customURL := "http://custom.host:9999"
	_, err := applyCCRelayConfig(tmpDir, customURL)
	if err != nil {
		t.Fatalf(applyCCRelayConfigErrFmt, err)
	}

	settings := readSettings(t, tmpDir)

	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatal(expectedEnvKeyMsg)
	}
	if env["ANTHROPIC_BASE_URL"] != customURL {
		t.Errorf("Expected custom ANTHROPIC_BASE_URL, got %v", env["ANTHROPIC_BASE_URL"])
	}
}

func TestRemoveCCRelayConfigExistingSettings(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	existingSettings := map[string]any{
		"theme": themeDark,
		"env": map[string]any{
			"ANTHROPIC_BASE_URL":   ccRelayProxyURL,
			"ANTHROPIC_AUTH_TOKEN": managedByCCRelayValue,
			"OTHER_VAR":            otherEnvValue,
		},
	}
	writeSettings(t, tmpDir, existingSettings)

	removed, _, err := removeCCRelayConfig(tmpDir)
	if err != nil {
		t.Fatalf("removeCCRelayConfig failed: %v", err)
	}

	if removed == nil {
		t.Fatal("Expected cc-relay config to be removed, got nil")
	}

	settings := readSettings(t, tmpDir)

	if settings["theme"] != themeDark {
		t.Errorf(themePreservedErrFmt, settings["theme"])
	}

	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatal(expectedEnvKeyMsg)
	}

	if _, exists := env["ANTHROPIC_BASE_URL"]; exists {
		t.Error("Expected ANTHROPIC_BASE_URL to be removed")
	}
	if _, exists := env["ANTHROPIC_AUTH_TOKEN"]; exists {
		t.Error("Expected ANTHROPIC_AUTH_TOKEN to be removed")
	}

	if env["OTHER_VAR"] != otherEnvValue {
		t.Errorf(otherVarPreservedErrFmt, env["OTHER_VAR"])
	}
}

func TestRemoveCCRelayConfigNoSettings(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	removed, _, err := removeCCRelayConfig(tmpDir)
	if err != nil {
		t.Errorf("Expected success when no settings file exists, got error: %v", err)
	}

	if removed != nil {
		t.Errorf("Expected nil removed when no settings, got %v", removed)
	}
}

func TestRemoveCCRelayConfigNoEnvSection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	existingSettings := map[string]any{
		"theme": themeDark,
	}
	writeSettings(t, tmpDir, existingSettings)

	removed, _, err := removeCCRelayConfig(tmpDir)
	if err != nil {
		t.Errorf("Expected success when no env section exists, got error: %v", err)
	}

	if removed != nil {
		t.Errorf("Expected nil removed when no env section, got %v", removed)
	}
}

func TestRemoveCCRelayConfigNoCCRelayConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	existingSettings := map[string]any{
		"env": map[string]any{
			"OTHER_VAR": otherEnvValue,
		},
	}
	writeSettings(t, tmpDir, existingSettings)

	removed, _, err := removeCCRelayConfig(tmpDir)
	if err != nil {
		t.Errorf("Expected success when no cc-relay config exists, got error: %v", err)
	}

	if removed != nil {
		t.Errorf("Expected nil removed when no cc-relay config, got %v", removed)
	}

	settings := readSettings(t, tmpDir)

	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatal(expectedEnvKeyMsg)
	}
	if env["OTHER_VAR"] != otherEnvValue {
		t.Errorf(otherVarPreservedErrFmt, env["OTHER_VAR"])
	}
}

func TestRemoveCCRelayConfigRemovesEmptyEnv(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	existingSettings := map[string]any{
		"theme": themeDark,
		"env": map[string]any{
			"ANTHROPIC_BASE_URL":   ccRelayProxyURL,
			"ANTHROPIC_AUTH_TOKEN": managedByCCRelayValue,
		},
	}
	writeSettings(t, tmpDir, existingSettings)

	removed, _, err := removeCCRelayConfig(tmpDir)
	if err != nil {
		t.Fatalf("removeCCRelayConfig failed: %v", err)
	}

	if removed == nil {
		t.Fatal("Expected cc-relay config to be removed, got nil")
	}

	settings := readSettings(t, tmpDir)

	if env, exists := settings["env"]; exists {
		if envMap, ok := env.(map[string]any); ok && len(envMap) > 0 {
			t.Errorf("Expected env section to be removed or empty, got %v", envMap)
		}
	}

	if settings["theme"] != themeDark {
		t.Errorf(themePreservedErrFmt, settings["theme"])
	}
}
