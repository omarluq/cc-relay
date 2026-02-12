package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/omarluq/cc-relay/internal/vinfo"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the cc-relay version",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("%s %s\n", rootCmd.Name(), vinfo.String())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
