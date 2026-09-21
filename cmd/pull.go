package cmd

import (
	usererrors "errors"
	"strings"

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
		if err := pullFlags.Validate(); err != nil {
			return err
		}
		cacheDirectory := cachedirectory.NewCacheDirectory(rootFlags.cacheDir)
		return pull.Pull(cmd.Context(), cacheDirectory, pullFlags.sourceToken, pullFlags.sourceURL, pullFlags.assetOSIncludes(), pullFlags.assetOSExcludes(), pullFlags.compressionFormat)
	},
}

type pullFlagFields struct {
	sourceToken       string
	sourceURL         string
	osInclude         string
	osExclude         string
	compressionFormat string
}

var pullFlags = pullFlagFields{}

const errorOSIncludeAndExclude = "You cannot specify both --os-include and --os-exclude at the same time. Please use only one of these flags."
const errorInvalidCompressionFormat = "Invalid --compression-format value. Valid values are \"gz\" or \"zst\"."

func (f *pullFlagFields) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.sourceToken, "source-token", "", "A token to access the API of GitHub.com. This is normally not required, but can be provided if you have issues with API rate limiting.")
	cmd.Flags().StringVar(&f.sourceURL, "source-url", "", "Use a custom Git URL for fetching the Action repository contents from. The CodeQL bundles will still be fetched from GitHub.com.")
	cmd.Flags().MarkHidden("source-url")
	cmd.Flags().StringVar(&f.osInclude, "os-include", "", "A comma-separated list of operating systems (e.g. \"linux64,win64\") to include CodeQL bundle release assets for. Cannot be used together with --os-exclude. If neither is specified, assets for all operating systems are synced.")
	cmd.Flags().StringVar(&f.osExclude, "os-exclude", "", "A comma-separated list of operating systems (e.g. \"win64,osx64\") to exclude CodeQL bundle release assets for. Cannot be used together with --os-include.")
	cmd.Flags().StringVar(&f.compressionFormat, "compression-format", "", "The compression format of CodeQL bundle release assets to sync, either \"gz\" or \"zst\". If not specified, both compression formats are synced.")
}

func splitCommaSeparatedList(value string) []string {
	if value == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (f *pullFlagFields) assetOSIncludes() []string {
	return splitCommaSeparatedList(f.osInclude)
}

func (f *pullFlagFields) assetOSExcludes() []string {
	return splitCommaSeparatedList(f.osExclude)
}

func (f *pullFlagFields) Validate() error {
	if f.osInclude != "" && f.osExclude != "" {
		return usererrors.New(errorOSIncludeAndExclude)
	}
	if f.compressionFormat != "" && f.compressionFormat != "gz" && f.compressionFormat != "zst" {
		return usererrors.New(errorInvalidCompressionFormat)
	}
	return nil
}
