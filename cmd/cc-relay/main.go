// Package main is the entry point for cc-relay.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/omarluq/cc-relay/internal/version"
)

const (
	defaultConfigFile = "config.yaml"
)

var cfgFile string

// rootCmd is the root Cobra command for cc-relay.
var rootCmd = &cobra.Command{
	Use:   "cc-relay",
	Short: "LLMs API Gateway",
	Long:  `⚡️ Blazing fast LLMs API Gateway written in Go.`,
	Example: `  # Start the proxy server with default config:
  cc-relay serve

  # Start with a custom config file:
  cc-relay serve --config /path/to/config.yaml

  # Check if the server is running:
  cc-relay status

  # Generate shell completions:
  cc-relay completion bash`,
}

// init sets up global flags for the root command.
func init() {
	// Global flags available to all subcommands
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file path (default: ./"+defaultConfigFile+" or ~/.config/cc-relay/"+defaultConfigFile+")")
}

// main is the entry point of the application.
// It sets up signal handling and executes the CLI with Fang styling.
func main() {
	// Create signal-aware context for graceful shutdown on SIGINT/SIGTERM
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	rootCmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")

	// Configure Fang with version info and styling
	fangOpts := []fang.Option{
		fang.WithVersion(version.String()),
	}

	if err := fang.Execute(ctx, rootCmd, fangOpts...); err != nil {
		os.Exit(1)
	}
}
