package cmd

import (
	"context"
	"crypto/tls"
	"net/http"
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
	insecure bool
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
	cmd.PersistentFlags().BoolVar(&f.insecure, "insecure", false, "Allow insecure server connections when using TLS")
	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if f.insecure {
			http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
	}

	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.PrintErrln(err)
		cmd.PrintErrln()
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
	rootCmd.AddCommand(licensesCmd)

	rootCmd.AddCommand(pullCmd)
	pullFlags.Init(pullCmd)

	rootCmd.AddCommand(pushCmd)
	pushFlags.Init(pushCmd)

	rootCmd.AddCommand(syncCmd)
	pullFlags.Init(syncCmd)
	pushFlags.Init(syncCmd)

	return rootCmd.ExecuteContext(ctx)
}
