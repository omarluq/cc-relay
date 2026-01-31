package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a default config file",
	Long:  `Generate a default cc-relay configuration file at ~/.config/cc-relay/config.yaml`,
	RunE:  runConfigInit,
}

// init registers the config "init" subcommand and its CLI flags.
// It adds the command to configCmd and defines the "output" (-o) flag for the target
// config file path (default: ~/.config/cc-relay/config.yaml) and the "force" flag
// to allow overwriting an existing config file.
func init() {
	configCmd.AddCommand(configInitCmd)
	configInitCmd.Flags().StringP("output", "o", "", "output path (default: ~/.config/cc-relay/config.yaml)")
	configInitCmd.Flags().Bool("force", false, "overwrite existing config file")
}

// runConfigInit creates a default configuration file for cc-relay at the provided
// output path or, if none is provided, at ~/.config/cc-relay/config.yaml.
// It creates parent directories as needed (permissions 0750), writes the config
// file with permissions 0600, and prints post-creation next steps to stdout.
// It returns an error if flag retrieval fails, if the target file already exists
// and --force is not set, or if any filesystem operation (directory creation or
// file write) fails.
func runConfigInit(cmd *cobra.Command, _ []string) error {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return fmt.Errorf("failed to get output flag: %w", err)
	}
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
	}

	if output == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		output = filepath.Join(home, ".config", "cc-relay", "config.yaml")
	}

	// Check if file exists
	if _, err := os.Stat(output); err == nil && !force {
		return fmt.Errorf("config file already exists at %s (use --force to overwrite)", output)
	}

	// Create directory if needed
	dir := filepath.Dir(output)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write default config
	defaultConfig := defaultConfigTemplate

	if err := os.WriteFile(output, []byte(defaultConfig), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✓ Config file created at %s\n", output)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Set ANTHROPIC_API_KEY environment variable")
	fmt.Println("  2. Edit the config file to customize providers and routing")
	fmt.Println("  3. Validate with: cc-relay config validate")
	fmt.Println("  4. Start the proxy: cc-relay serve")
	fmt.Println("  5. Configure Claude Code: cc-relay config cc init")

	return nil
}