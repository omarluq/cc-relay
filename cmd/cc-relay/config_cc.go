package main

import "github.com/spf13/cobra"

var configCCCmd = &cobra.Command{
	Use:   "cc",
	Short: "Configure Claude Code integration",
}

func init() {
	configCmd.AddCommand(configCCCmd)
}
