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
		return pull.Pull(cmd.Context(), cacheDirectory, pullFlags.sourceToken, pullFlags.sourceURL)
	},
}

type pullFlagFields struct {
	sourceToken string
	sourceURL   string
}

var pullFlags = pullFlagFields{}

func (f *pullFlagFields) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.sourceToken, "source-token", "", "A token to access the API of GitHub.com. This is normally not required, but can be provided if you have issues with API rate limiting.")
	cmd.Flags().StringVar(&f.sourceURL, "source-url", "", "Use a custom Git URL for fetching the Action repository contents from. The CodeQL bundles will still be fetched from GitHub.com.")
	cmd.Flags().MarkHidden("source-url")
}
