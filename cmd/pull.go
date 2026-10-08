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
		return pull.Pull(cmd.Context(), cacheDirectory, pullFlags.sourceToken, pullFlags.sourceURL, pullFlags.includePlatforms, pullFlags.excludePlatforms, pullFlags.bundleArchiveFormat)
	},
}

type pullFlagFields struct {
	sourceToken         string
	sourceURL           string
	includePlatforms    []string
	excludePlatforms    []string
	bundleArchiveFormat string
}

var pullFlags = pullFlagFields{}

func (f *pullFlagFields) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.sourceToken, "source-token", "", "A token to access the API of GitHub.com. This is normally not required, but can be provided if you have issues with API rate limiting.")
	cmd.Flags().StringVar(&f.sourceURL, "source-url", "", "Use a custom Git URL for fetching the Action repository contents from. The CodeQL bundles will still be fetched from GitHub.com.")
	cmd.Flags().MarkHidden("source-url")
	cmd.Flags().StringSliceVar(&f.includePlatforms, "include-platforms", nil, "Only download release assets for these platforms: linux64, linux-arm64, osx64, win64.")
	cmd.Flags().StringSliceVar(&f.excludePlatforms, "exclude-platforms", nil, "Download release assets for every platform except these: linux64, linux-arm64, osx64, win64.")
	cmd.Flags().StringVar(&f.bundleArchiveFormat, "bundle-archive-format", "", "Only download CodeQL bundles in this archive format: tar.gz or tar.zst. Requires an explicit tools URL in CodeQL workflows.")
}
