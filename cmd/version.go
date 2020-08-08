package cmd

import (
	"fmt"

	"github.com/github/codeql-action-sync/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version of the sync tool.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Version())
		fmt.Println(version.Commit())
	},
}
