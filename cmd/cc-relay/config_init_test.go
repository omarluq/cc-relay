package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunConfigInitDefaultPath(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies HOME env var)

	// Create a temp directory to use as HOME
	tmpDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer func() { os.Setenv("HOME", origHome) }()

	os.Setenv("HOME", tmpDir)

	// Create a mock command with the output and force flags
	cmd := &cobra.Command{}
	cmd.Flags().StringP("output", "o", "", "output path")
	cmd.Flags().Bool("force", false, "overwrite existing")

	// runConfigInit should create config file
	err := runConfigInit(cmd, nil)
	if err != nil {
		t.Fatalf("runConfigInit failed: %v", err)
	}

	// Verify config file was created
	configPath := filepath.Join(tmpDir, ".config", "cc-relay", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config.yaml to be created")
	}

	// Verify content has expected structure
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config.yaml: %v", err)
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
	// Note: Cannot use t.Parallel() (modifies HOME env var)

	// Create a temp directory
	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "custom", "config.yaml")

	// Create a mock command with custom output path
	cmd := &cobra.Command{}
	cmd.Flags().StringP("output", "o", "", "output path")
	cmd.Flags().Bool("force", false, "overwrite existing")
	_ = cmd.Flags().Set("output", customPath)

	// runConfigInit should create config file at custom path
	err := runConfigInit(cmd, nil)
	if err != nil {
		t.Fatalf("runConfigInit failed: %v", err)
	}

	// Verify config file was created at custom path
	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Errorf("Expected config.yaml to be created at %s", customPath)
	}
}

func TestRunConfigInitExistingFileWithoutForce(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies HOME env var)

	// Create a temp directory with an existing config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("existing: content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a mock command without force flag
	cmd := &cobra.Command{}
	cmd.Flags().StringP("output", "o", "", "output path")
	cmd.Flags().Bool("force", false, "overwrite existing")
	_ = cmd.Flags().Set("output", configPath)

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
	// Note: Cannot use t.Parallel() (modifies HOME env var)

	// Create a temp directory with an existing config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("existing: content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a mock command with force flag
	cmd := &cobra.Command{}
	cmd.Flags().StringP("output", "o", "", "output path")
	cmd.Flags().Bool("force", false, "overwrite existing")
	_ = cmd.Flags().Set("output", configPath)
	_ = cmd.Flags().Set("force", "true")

	// runConfigInit should succeed and overwrite
	err := runConfigInit(cmd, nil)
	if err != nil {
		t.Fatalf("runConfigInit with force failed: %v", err)
	}

	// Verify content was overwritten (not "existing: content")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config.yaml: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "existing: content") {
		t.Error("Expected config to be overwritten")
	}
	if !strings.Contains(content, "server:") {
		t.Error("Expected new config to contain 'server:' section")
	}
}

func TestRunConfigInitCreatesDirectory(t *testing.T) {
	// Note: Cannot use t.Parallel() (modifies HOME env var)

	// Create a temp directory
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "a", "b", "c", "config.yaml")

	// Create a mock command with nested path
	cmd := &cobra.Command{}
	cmd.Flags().StringP("output", "o", "", "output path")
	cmd.Flags().Bool("force", false, "overwrite existing")
	_ = cmd.Flags().Set("output", nestedPath)

	// runConfigInit should create nested directories
	err := runConfigInit(cmd, nil)
	if err != nil {
		t.Fatalf("runConfigInit failed: %v", err)
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
