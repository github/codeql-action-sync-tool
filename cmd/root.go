package cmd

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "codeql-action-sync",
	Short:         "A tool for syncing the CodeQL Action from GitHub.com to GitHub Enterprise Server.",
	SilenceErrors: true,
	SilenceUsage:  true,
}

type rootFlagFields struct {
}

var rootFlags = rootFlagFields{}

var SilentErr = errors.New("SilentErr")

func (f *rootFlagFields) Init(cmd *cobra.Command) {
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.PrintErr(err)
		cmd.PrintErr(cmd.UsageString())
		return SilentErr
	})
}

func Execute(ctx context.Context) error {
	rootFlags.Init(rootCmd)

	rootCmd.AddCommand(versionCmd)

	return rootCmd.ExecuteContext(ctx)
}
