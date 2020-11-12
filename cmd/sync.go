package cmd

import (
	"github.com/github/codeql-action-sync/internal/cachedirectory"
	"github.com/github/codeql-action-sync/internal/pull"
	"github.com/github/codeql-action-sync/internal/push"
	"github.com/github/codeql-action-sync/internal/version"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync the CodeQL Action from GitHub to a GitHub Enterprise Server installation.",
	RunE: func(cmd *cobra.Command, args []string) error {
		version.LogVersion()
		cacheDirectory := cachedirectory.NewCacheDirectory(rootFlags.cacheDir)
		err := pull.Pull(cmd.Context(), cacheDirectory, pullFlags.sourceToken)
		if err != nil {
			return err
		}
		err = push.Push(cmd.Context(), cacheDirectory, pushFlags.destinationURL, pushFlags.destinationToken, pushFlags.destinationRepository, pushFlags.actionsAdminUser, pushFlags.force, pushFlags.pushSSH)
		if err != nil {
			return err
		}
		return nil
	},
}
