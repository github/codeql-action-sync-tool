package cmd

import (
	"os"

	"github.com/github/codeql-action-sync/internal/cachedirectory"
	"github.com/github/codeql-action-sync/internal/environment"
	"github.com/github/codeql-action-sync/internal/push"
	"github.com/github/codeql-action-sync/internal/version"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push the CodeQL Action from the local cache to a GitHub Enterprise Server installation.",
	RunE: func(cmd *cobra.Command, args []string) error {
		version.LogVersion()
		cacheDirectory := cachedirectory.NewCacheDirectory(rootFlags.cacheDir)
		return push.Push(cmd.Context(), cacheDirectory, pushFlags.destinationURL, pushFlags.destinationToken, pushFlags.destinationRepository, pushFlags.actionsAdminUser, pushFlags.force, pushFlags.pushSSH, pushFlags.gitURL)
	},
}

type pushFlagFields struct {
	destinationURL        string
	destinationToken      string
	destinationRepository string
	actionsAdminUser      string
	force                 bool
	pushSSH               bool
	gitURL                string
}

var pushFlags = pushFlagFields{}

func (f *pushFlagFields) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.destinationURL, "destination-url", "", "The URL of the GitHub Enterprise instance to push to.")
	cmd.MarkFlagRequired("destination-url")
	cmd.Flags().StringVar(&f.destinationToken, "destination-token", "", "A token to access the API on the GitHub Enterprise instance (can also be provided by setting the "+environment.DestinationToken+" environment variable).")
	if f.destinationToken == "" {
		f.destinationToken = os.Getenv(environment.DestinationToken)
		if f.destinationToken == "" {
			cmd.MarkFlagRequired("destination-token")
		}
	}
	cmd.Flags().StringVar(&f.destinationRepository, "destination-repository", "github/codeql-action", "The name of the repository to create on GitHub Enterprise.")
	cmd.Flags().StringVar(&f.actionsAdminUser, "actions-admin-user", "actions-admin", "The name of the Actions admin user.")
	cmd.Flags().BoolVar(&f.force, "force", false, "Replace the existing repository even if it was not created by the sync tool.")
	cmd.Flags().BoolVar(&f.pushSSH, "push-ssh", false, "Push Git contents over SSH rather than HTTPS. To use this option you must have SSH access to your GitHub Enterprise instance configured.")
	cmd.Flags().StringVar(&f.gitURL, "git-url", "", "Use a custom Git URL for pushing the Action repository contents to.")
	cmd.Flags().MarkHidden("git-url")
}
