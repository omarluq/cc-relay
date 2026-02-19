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

func init() {
	configCmd.AddCommand(configInitCmd)
	configInitCmd.Flags().StringP("output", "o", "", "output path (default: ~/.config/cc-relay/config.yaml)")
	configInitCmd.Flags().Bool("force", false, "overwrite existing config file")
}

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
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write default config
	defaultConfig := defaultConfigTemplate

	if err := os.WriteFile(output, []byte(defaultConfig), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	cmd.Printf("✓ Config file created at %s\n", output)
	cmd.Println("\nNext steps:")
	cmd.Println("  1. Set ANTHROPIC_API_KEY environment variable")
	cmd.Println("  2. Edit the config file to customize providers and routing")
	cmd.Println("  3. Validate with: cc-relay config validate")
	cmd.Println("  4. Start the proxy: cc-relay serve")
	cmd.Println("  5. Configure Claude Code: cc-relay config cc init")

	return nil
}
