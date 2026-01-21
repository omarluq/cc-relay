package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/omarluq/cc-relay/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version, git commit, and build date.`,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("cc-relay %s\n", version.String())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
