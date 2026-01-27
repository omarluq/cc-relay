// Package main is the entry point for cc-relay.
package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/omarluq/cc-relay/internal/version"
)

const (
	defaultConfigFile = "config.yaml"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "cc-relay",
	Short: "Blazing fast LLMs API Gateway written in Go",
	Long: `⚡️ CC-Relay is a Blazing fast LLMs API Gateway written in Go.`,
	Example: `  # Start the proxy server with default config:
  cc-relay serve

  # Start with a custom config file:
  cc-relay serve --config /path/to/config.yaml

  # Check if the server is running:
  cc-relay status

  # Generate shell completions:
  cc-relay completion bash`,
}

func init() {
	// Global flags available to all subcommands
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file path (default: ./"+defaultConfigFile+" or ~/.config/cc-relay/"+defaultConfigFile+")")
}

func main() {
	// Configure Fang with version info and styling
	fangOpts := []fang.Option{
		fang.WithVersion(version.Version),
		fang.WithCommit(version.Commit),
	}

	if err := fang.Execute(context.Background(), rootCmd, fangOpts...); err != nil {
		os.Exit(1)
	}
}
