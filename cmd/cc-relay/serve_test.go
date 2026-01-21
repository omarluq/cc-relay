package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindConfigFile(t *testing.T) {
	t.Parallel()

	// Save original working directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Logf("failed to restore working directory: %v", err)
		}
	}()

	// Create temp directory with config.yaml
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  listen: localhost:8787\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Test finding config in current directory
	found := findConfigFile()
	if found != "config.yaml" {
		t.Errorf("Expected 'config.yaml', got %q", found)
	}
}

func TestFindConfigFile_NotFound(t *testing.T) {
	// Note: Cannot use t.Parallel() because we modify HOME env var

	// Save original working directory and HOME
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")

	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Logf("failed to restore working directory: %v", err)
		}
		os.Setenv("HOME", origHome)
	}()

	// Change to temp directory without config.yaml
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Set HOME to temp dir so it won't find user's config
	os.Setenv("HOME", tmpDir)

	// Should return default even if not found
	found := findConfigFile()
	if found != "config.yaml" {
		t.Errorf("Expected 'config.yaml' default, got %q", found)
	}
}
