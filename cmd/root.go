package cmd

import (
	"context"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "codeql-action-sync",
	Short:         "A tool for syncing the CodeQL Action from GitHub.com to GitHub Enterprise Server.",
	SilenceErrors: true,
	SilenceUsage:  true,
}

type rootFlagFields struct {
	cacheDir string
}

var rootFlags = rootFlagFields{}

var SilentErr = errors.New("SilentErr")

func (f *rootFlagFields) Init(cmd *cobra.Command) error {
	executablePath, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "Error finding own executable path.")
	}
	executableDirectoryPath := filepath.Dir(executablePath)
	defaultCacheDir := path.Join(executableDirectoryPath, "cache")

	cmd.PersistentFlags().StringVar(&f.cacheDir, "cache-dir", defaultCacheDir, "The path to a local directory to cache the Action in.")

	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.PrintErr(err)
		cmd.PrintErr(cmd.UsageString())
		return SilentErr
	})

	return nil
}

func Execute(ctx context.Context) error {
	err := rootFlags.Init(rootCmd)
	if err != nil {
		return err
	}

	rootCmd.AddCommand(versionCmd)

	rootCmd.AddCommand(pullCmd)
	pullFlags.Init(pullCmd)

	rootCmd.AddCommand(pushCmd)
	pushFlags.Init(pushCmd)

	rootCmd.AddCommand(syncCmd)
	pullFlags.Init(syncCmd)
	pushFlags.Init(syncCmd)

	return rootCmd.ExecuteContext(ctx)
}
