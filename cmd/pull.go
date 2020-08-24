package cmd

import (
	"github.com/github/codeql-action-sync/internal/cachedirectory"
	"github.com/github/codeql-action-sync/internal/pull"
	"github.com/github/codeql-action-sync/internal/version"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull the CodeQL Action from GitHub to a local cache.",
	RunE: func(cmd *cobra.Command, args []string) error {
		version.LogVersion()
		cacheDirectory := cachedirectory.NewCacheDirectory(rootFlags.cacheDir)
		return pull.Pull(cmd.Context(), cacheDirectory)
	},
}

type pullFlagFields struct{}

var pullFlags = pullFlagFields{}

func (f *pullFlagFields) Init(cmd *cobra.Command) {}
