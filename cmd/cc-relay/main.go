// Package main is the entry point for cc-relay.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "cc-relay",
	Short: "Multi-provider proxy for Claude Code",
	Long: `cc-relay is a multi-provider proxy that sits between Claude Code and multiple
LLM providers (Anthropic, Z.AI, Ollama), enabling seamless model switching,
rate limit pooling, and intelligent routing.`,
}

func init() {
	// Global flags available to all subcommands
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file path (default: ./config.yaml or ~/.config/cc-relay/config.yaml)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
