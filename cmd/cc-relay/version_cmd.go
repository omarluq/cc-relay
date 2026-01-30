package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/omarluq/cc-relay/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the cc-relay version",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("%s %s\n", rootCmd.Name(), version.String())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
