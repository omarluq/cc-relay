package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

const (
	ccRelayProxyURL         = "http://127.0.0.1:8787"
	ccRelayProxyURLDesc     = "cc-relay proxy URL"
	ccRelayProxyURLFlag     = "proxy-url"
	claudeDirName           = ".claude"
	settingsFileName        = "settings.json"
	runConfigCCInitErrFmt   = "runConfigCCInit failed: %v"
	readSettingsErrFmt      = "Failed to read settings.json: %v"
	parseSettingsErrFmt     = "Failed to parse settings.json: %v"
	expectedEnvKeyMsg       = "Expected env key in settings"
	managedByCCRelayToken   = "managed-by-cc-relay"
	otherEnvValue           = "other-value"
	themePreservedErrFmt    = "Expected theme to be preserved, got %v"
	otherVarPreservedErrFmt = "Expected OTHER_VAR to be preserved, got %v"
)

func withTempHome(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	return tmpDir
}

func settingsPathForHome(home string) string {
	return filepath.Join(home, claudeDirName, settingsFileName)
}

func newConfigCCCommand(proxyURL string, setFlag bool) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String(ccRelayProxyURLFlag, proxyURL, ccRelayProxyURLDesc)
	if setFlag {
		_ = cmd.Flags().Set(ccRelayProxyURLFlag, proxyURL)
	}
	return cmd
}

func readSettings(t *testing.T, home string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(settingsPathForHome(home))
	if err != nil {
		t.Fatalf(readSettingsErrFmt, err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf(parseSettingsErrFmt, err)
	}

	return settings
}

func writeSettings(t *testing.T, home string, settings map[string]interface{}) {
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

func TestRunConfigCCInitNewSettings(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var
	tmpDir := withTempHome(t)

	// Create a mock command with the proxy-url flag
	cmd := newConfigCCCommand(ccRelayProxyURL, false)

	// runConfigCCInit should create settings file
	err := runConfigCCInit(cmd, nil)
	if err != nil {
		t.Fatalf(runConfigCCInitErrFmt, err)
	}

	// Verify settings file was created
	if _, err := os.Stat(settingsPathForHome(tmpDir)); os.IsNotExist(err) {
		t.Error("Expected settings.json to be created")
	}

	// Verify content
	settings := readSettings(t, tmpDir)

	env, ok := settings["env"].(map[string]interface{})
	if !ok {
		t.Fatal(expectedEnvKeyMsg)
	}

	if env["ANTHROPIC_BASE_URL"] != ccRelayProxyURL {
		t.Errorf("Expected ANTHROPIC_BASE_URL to be %s, got %v", ccRelayProxyURL, env["ANTHROPIC_BASE_URL"])
	}

	if env["ANTHROPIC_AUTH_TOKEN"] != managedByCCRelayToken {
		t.Errorf("Expected ANTHROPIC_AUTH_TOKEN to be %s, got %v", managedByCCRelayToken, env["ANTHROPIC_AUTH_TOKEN"])
	}
}

func TestRunConfigCCInitExistingSettings(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var
	tmpDir := withTempHome(t)

	existingSettings := map[string]interface{}{
		"theme": "dark",
		"env": map[string]interface{}{
			"OTHER_VAR": otherEnvValue,
		},
	}
	writeSettings(t, tmpDir, existingSettings)

	// Create a mock command with the proxy-url flag and set it
	cmd := newConfigCCCommand(ccRelayProxyURL, true)

	// runConfigCCInit should update settings file
	err := runConfigCCInit(cmd, nil)
	if err != nil {
		t.Fatalf(runConfigCCInitErrFmt, err)
	}

	// Verify content preserves existing settings
	settings := readSettings(t, tmpDir)

	// Check theme is preserved
	if settings["theme"] != "dark" {
		t.Errorf(themePreservedErrFmt, settings["theme"])
	}

	env, ok := settings["env"].(map[string]interface{})
	if !ok {
		t.Fatal(expectedEnvKeyMsg)
	}

	// Check existing env var is preserved
	if env["OTHER_VAR"] != otherEnvValue {
		t.Errorf(otherVarPreservedErrFmt, env["OTHER_VAR"])
	}

	// Check new env vars are added
	if env["ANTHROPIC_BASE_URL"] != ccRelayProxyURL {
		t.Errorf("Expected ANTHROPIC_BASE_URL to be set, got %v", env["ANTHROPIC_BASE_URL"])
	}
}

func TestRunConfigCCInitCustomProxyURL(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var
	tmpDir := withTempHome(t)

	// Create a mock command with a custom proxy-url
	cmd := newConfigCCCommand("http://custom.host:9999", false)

	err := runConfigCCInit(cmd, nil)
	if err != nil {
		t.Fatalf(runConfigCCInitErrFmt, err)
	}

	// Verify custom URL was used
	settings := readSettings(t, tmpDir)

	env := settings["env"].(map[string]interface{})
	if env["ANTHROPIC_BASE_URL"] != "http://custom.host:9999" {
		t.Errorf("Expected custom ANTHROPIC_BASE_URL, got %v", env["ANTHROPIC_BASE_URL"])
	}
}

func TestRunConfigCCRemoveExistingSettings(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var
	tmpDir := withTempHome(t)

	existingSettings := map[string]interface{}{
		"theme": "dark",
		"env": map[string]interface{}{
			"ANTHROPIC_BASE_URL":   ccRelayProxyURL,
			"ANTHROPIC_AUTH_TOKEN": managedByCCRelayToken,
			"OTHER_VAR":            otherEnvValue,
		},
	}
	writeSettings(t, tmpDir, existingSettings)

	// runConfigCCRemove should remove cc-relay env vars
	err := runConfigCCRemove(nil, nil)
	if err != nil {
		t.Fatalf("runConfigCCRemove failed: %v", err)
	}

	// Verify content
	settings := readSettings(t, tmpDir)

	// Check theme is preserved
	if settings["theme"] != "dark" {
		t.Errorf(themePreservedErrFmt, settings["theme"])
	}

	env, ok := settings["env"].(map[string]interface{})
	if !ok {
		t.Fatal(expectedEnvKeyMsg)
	}

	// Check cc-relay env vars are removed
	if _, exists := env["ANTHROPIC_BASE_URL"]; exists {
		t.Error("Expected ANTHROPIC_BASE_URL to be removed")
	}
	if _, exists := env["ANTHROPIC_AUTH_TOKEN"]; exists {
		t.Error("Expected ANTHROPIC_AUTH_TOKEN to be removed")
	}

	// Check other env var is preserved
	if env["OTHER_VAR"] != otherEnvValue {
		t.Errorf(otherVarPreservedErrFmt, env["OTHER_VAR"])
	}
}

func TestRunConfigCCRemoveNoSettings(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var
	_ = withTempHome(t)

	// runConfigCCRemove should succeed (nothing to remove)
	err := runConfigCCRemove(nil, nil)
	if err != nil {
		t.Errorf("Expected success when no settings file exists, got error: %v", err)
	}
}

func TestRunConfigCCRemoveNoEnvSection(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var
	tmpDir := withTempHome(t)

	existingSettings := map[string]interface{}{
		"theme": "dark",
	}
	writeSettings(t, tmpDir, existingSettings)

	// runConfigCCRemove should succeed (nothing to remove)
	err := runConfigCCRemove(nil, nil)
	if err != nil {
		t.Errorf("Expected success when no env section exists, got error: %v", err)
	}
}

func TestRunConfigCCRemoveNoCCRelayConfig(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var
	tmpDir := withTempHome(t)

	existingSettings := map[string]interface{}{
		"env": map[string]interface{}{
			"OTHER_VAR": otherEnvValue,
		},
	}
	writeSettings(t, tmpDir, existingSettings)

	// runConfigCCRemove should succeed (nothing cc-relay specific to remove)
	err := runConfigCCRemove(nil, nil)
	if err != nil {
		t.Errorf("Expected success when no cc-relay config exists, got error: %v", err)
	}

	// Verify other env vars are preserved
	settings := readSettings(t, tmpDir)

	env := settings["env"].(map[string]interface{})
	if env["OTHER_VAR"] != otherEnvValue {
		t.Errorf(otherVarPreservedErrFmt, env["OTHER_VAR"])
	}
}

func TestRunConfigCCRemoveRemovesEmptyEnv(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var
	tmpDir := withTempHome(t)

	existingSettings := map[string]interface{}{
		"theme": "dark",
		"env": map[string]interface{}{
			"ANTHROPIC_BASE_URL":   ccRelayProxyURL,
			"ANTHROPIC_AUTH_TOKEN": managedByCCRelayToken,
		},
	}
	writeSettings(t, tmpDir, existingSettings)

	// runConfigCCRemove should remove cc-relay vars and empty env section
	err := runConfigCCRemove(nil, nil)
	if err != nil {
		t.Fatalf("runConfigCCRemove failed: %v", err)
	}

	// Verify env section is removed when empty
	settings := readSettings(t, tmpDir)

	// After removal, the env section should not exist (was empty)
	// or if it does, it should be empty
	if env, exists := settings["env"]; exists {
		if envMap, ok := env.(map[string]interface{}); ok && len(envMap) > 0 {
			t.Errorf("Expected env section to be removed or empty, got %v", envMap)
		}
	}

	// Check theme is still there
	if settings["theme"] != "dark" {
		t.Errorf(themePreservedErrFmt, settings["theme"])
	}
}
