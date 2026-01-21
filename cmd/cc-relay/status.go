package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/omarluq/cc-relay/internal/config"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if cc-relay server is running",
	Long: `Check the health status of a running cc-relay server by querying
its /health endpoint.`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(_ *cobra.Command, _ []string) error {
	// Load config to get server listen address
	configPath := cfgFile
	if configPath == "" {
		configPath = findConfigFileForStatus()
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Build health endpoint URL
	healthURL := fmt.Sprintf("http://%s/health", cfg.Server.Listen)

	// Query health endpoint with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	//nolint:noctx // Simple health check doesn't need context propagation
	resp, err := client.Get(healthURL)
	if err != nil {
		fmt.Printf("✗ cc-relay is not running (%s)\n", cfg.Server.Listen)
		return fmt.Errorf("server not reachable: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Logger.Warn().Err(closeErr).Msg("Failed to close response body")
		}
	}()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("✓ cc-relay is running (%s)\n", cfg.Server.Listen)
		return nil
	}

	fmt.Printf("✗ cc-relay returned unexpected status: %d\n", resp.StatusCode)

	return fmt.Errorf("health check failed with status %d", resp.StatusCode)
}

// findConfigFileForStatus is a copy of findConfigFile from serve.go.
// Duplicated to avoid shared state between subcommands.
//

func findConfigFileForStatus() string {
	// Check current directory
	if _, err := os.Stat(defaultConfigFile); err == nil {
		return defaultConfigFile
	}
	// Check ~/.config/cc-relay/
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		p := filepath.Join(home, ".config", "cc-relay", defaultConfigFile)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return defaultConfigFile
}
