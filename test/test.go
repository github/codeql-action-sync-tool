package test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func CreateTemporaryDirectory(t *testing.T) string {
	directory, err := ioutil.TempDir("", "codeql-action-sync-tests")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(directory)
	})
	return directory
}
