package main

import (
	"github.com/spf13/cobra"

	"github.com/omarluq/cc-relay/internal/vinfo"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the cc-relay version",
	Run: func(cmd *cobra.Command, _ []string) {
		cmd.Printf("%s %s\n", rootCmd.Name(), vinfo.String())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
