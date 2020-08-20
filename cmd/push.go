package cmd

import (
	"github.com/github/codeql-action-sync/internal/cachedirectory"
	"github.com/github/codeql-action-sync/internal/push"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push the CodeQL Action from the local cache to a GitHub Enterprise Server installation.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cacheDirectory := cachedirectory.NewCacheDirectory(rootFlags.cacheDir)
		return push.Push(cmd.Context(), cacheDirectory, pushFlags.destinationURL, pushFlags.destinationToken, pushFlags.destinationRepository)
	},
}

type pushFlagFields struct {
	destinationURL        string
	destinationToken      string
	destinationRepository string
}

var pushFlags = pushFlagFields{}

func (f *pushFlagFields) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.destinationURL, "destination-url", "", "The URL of the GitHub Enterprise instance to push to.")
	cmd.MarkFlagRequired("destination-url")
	cmd.Flags().StringVar(&f.destinationToken, "destination-token", "", "A token to access the API on the GitHub Enterprise instance.")
	cmd.MarkFlagRequired("destination-token")
	cmd.Flags().StringVar(&f.destinationRepository, "destination-repository", "github/codeql-action", "The name of the repository to create on GitHub Enterprise.")
}
