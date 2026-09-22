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
		if err := pullFlags.Validate(); err != nil {
			return err
		}
		cacheDirectory := cachedirectory.NewCacheDirectory(rootFlags.cacheDir)
		err := pull.Pull(cmd.Context(), cacheDirectory, pullFlags.sourceToken, pullFlags.sourceURL, pullFlags.assetOSIncludes(), pullFlags.assetOSExcludes(), pullFlags.compressionFormat)
		if err != nil {
			return err
		}
		err = push.Push(cmd.Context(), cacheDirectory, pushFlags.destinationURL, pushFlags.destinationToken, pushFlags.destinationRepository, pushFlags.actionsAdminUser, pushFlags.force, pushFlags.pushSSH, pushFlags.gitURL)
		if err != nil {
			return err
		}
		return nil
	},
}
