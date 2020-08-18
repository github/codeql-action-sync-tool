package test

import (
	"io/ioutil"
	"net/http"
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

func ServeHTTPResponseFromFile(t *testing.T, statusCode int, path string, response http.ResponseWriter) {
	data, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	response.WriteHeader(statusCode)
	_, err = response.Write(data)
	require.NoError(t, err)
}

func RequireFilesAreEqual(t *testing.T, expectedPath string, actualPath string) {
	expectedData, err := ioutil.ReadFile(expectedPath)
	require.NoError(t, err)
	actualData, err := ioutil.ReadFile(actualPath)
	require.NoError(t, err)
	require.Equal(t, expectedData, actualData)
}
