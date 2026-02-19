package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const (
	initConfigFileName            = "config.yaml"
	initConfigOutputFlag          = "output"
	initConfigOutputFlagShorthand = "o"
	initConfigOutputDesc          = "output path"
	initConfigForceFlag           = "force"
	initConfigForceDesc           = "overwrite existing"
	runConfigInitErrFmt           = "runConfigInit failed: %v"
	existingConfigContent         = "existing: content"
)

// newMockInitCmd creates a mock cobra.Command with the output and force flags
// pre-registered, matching the flags used by the init command.
func newMockInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "init",
	}
	cmd.Flags().StringP(initConfigOutputFlag, initConfigOutputFlagShorthand, "", initConfigOutputDesc)
	cmd.Flags().Bool(initConfigForceFlag, false, initConfigForceDesc)
	return cmd
}

func TestRunConfigInitDefaultPath(t *testing.T) {
	t.Parallel(
	// Note: Cannot use t.Parallel() (modifies HOME env var)
	)

	// Create a temp directory to use as HOME
	tmpDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("HOME", origHome); err != nil {
			t.Logf("failed to restore HOME: %v", err)
		}
	}()

	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create a mock command with the output and force flags
	cmd := newMockInitCmd()

	// runConfigInit should create config file
	err := runConfigInit(cmd, nil)
	if err != nil {
		t.Fatalf(runConfigInitErrFmt, err)
	}

	// Verify config file was created
	configPath := filepath.Join(tmpDir, ".config", "cc-relay", initConfigFileName)
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		t.Error("Expected config.yaml to be created")
	}

	// Verify content has expected structure
	data, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		t.Fatalf("Failed to read %s: %v", initConfigFileName, err)
	}

	content := string(data)
	if !strings.Contains(content, "server:") {
		t.Error("Expected config to contain 'server:' section")
	}
	if !strings.Contains(content, "providers:") {
		t.Error("Expected config to contain 'providers:' section")
	}
}

func TestRunConfigInitCustomPath(t *testing.T) {
	t.Parallel(
	// Note: Cannot use t.Parallel() (modifies HOME env var)
	)

	// Create a temp directory
	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "custom", initConfigFileName)

	// Create a mock command with custom output path
	cmd := newMockInitCmd()
	if err := cmd.Flags().Set(initConfigOutputFlag, customPath); err != nil {
		t.Fatal(err)
	}

	// runConfigInit should create config file at custom path
	err := runConfigInit(cmd, nil)
	if err != nil {
		t.Fatalf(runConfigInitErrFmt, err)
	}

	// Verify config file was created at custom path
	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Errorf("Expected config.yaml to be created at %s", customPath)
	}
}

func TestRunConfigInitExistingFileWithoutForce(t *testing.T) {
	t.Parallel(
	// Note: Cannot use t.Parallel() (modifies HOME env var)
	)

	// Create a temp directory with an existing config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, initConfigFileName)
	if err := os.WriteFile(configPath, []byte(existingConfigContent), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create a mock command without force flag
	cmd := newMockInitCmd()
	if err := cmd.Flags().Set(initConfigOutputFlag, configPath); err != nil {
		t.Fatal(err)
	}

	// runConfigInit should fail
	err := runConfigInit(cmd, nil)
	if err == nil {
		t.Error("Expected error when config file exists and force is not set")
	}
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got: %v", err)
	}
}

func TestRunConfigInitExistingFileWithForce(t *testing.T) {
	t.Parallel(
	// Note: Cannot use t.Parallel() (modifies HOME env var)
	)

	// Create a temp directory with an existing config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, initConfigFileName)
	if err := os.WriteFile(configPath, []byte(existingConfigContent), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create a mock command with force flag
	cmd := newMockInitCmd()
	if err := cmd.Flags().Set(initConfigOutputFlag, configPath); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set(initConfigForceFlag, "true"); err != nil {
		t.Fatal(err)
	}

	// runConfigInit should succeed and overwrite
	err := runConfigInit(cmd, nil)
	if err != nil {
		t.Fatalf("runConfigInit with force failed: %v", err)
	}

	// Verify content was overwritten (not "existing: content")
	data, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		t.Fatalf("Failed to read %s: %v", initConfigFileName, err)
	}

	content := string(data)
	if strings.Contains(content, existingConfigContent) {
		t.Error("Expected config to be overwritten")
	}
	if !strings.Contains(content, "server:") {
		t.Error("Expected new config to contain 'server:' section")
	}
}

func TestRunConfigInitCreatesDirectory(t *testing.T) {
	t.Parallel(
	// Note: Cannot use t.Parallel() (modifies HOME env var)
	)

	// Create a temp directory
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "a", "b", "c", initConfigFileName)

	// Create a mock command with nested path
	cmd := newMockInitCmd()
	if err := cmd.Flags().Set(initConfigOutputFlag, nestedPath); err != nil {
		t.Fatal(err)
	}

	// runConfigInit should create nested directories
	err := runConfigInit(cmd, nil)
	if err != nil {
		t.Fatalf(runConfigInitErrFmt, err)
	}

	// Verify directories were created
	if _, err := os.Stat(filepath.Dir(nestedPath)); os.IsNotExist(err) {
		t.Error("Expected nested directories to be created")
	}

	// Verify config file was created
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("Expected config.yaml to be created")
	}
}
