// Package main is the entry point for cc-relay.
package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

const (
	defaultConfigFile = "config.yaml"
)

var cfgFile string

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
		"config file path (default: ./"+defaultConfigFile+" or ~/.config/cc-relay/"+defaultConfigFile+")")
}

func main() {
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		os.Exit(1)
	}
}
