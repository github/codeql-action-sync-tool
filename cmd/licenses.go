package cmd

import (
	"github.com/github/codeql-action-sync/internal/licenses"
	"github.com/spf13/cobra"
)

var licensesCmd = &cobra.Command{
	Use:   "licenses",
	Short: "Display the licenses of all the dependencies of this tool.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return licenses.PrintLicenses()
	},
}
