package test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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

func GetTestHTTPServer(t *testing.T) (*mux.Router, string) {
	mux := mux.NewRouter()
	mux.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		require.Failf(t, "Unexpected HTTP request: %s %s", request.Method, request.URL.Path)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(func() {
		server.Close()
	})
	return mux, server.URL
}

func ServeHTTPResponseFromString(t *testing.T, content string, response http.ResponseWriter) {
	_, err := response.Write([]byte(content))
	require.NoError(t, err)
}

func ServeHTTPResponseFromObject(t *testing.T, object interface{}, response http.ResponseWriter) {
	bytes, err := json.Marshal(object)
	require.NoError(t, err)
	_, err = response.Write(bytes)
	require.NoError(t, err)
}

func RequireFileHasContent(t *testing.T, expectedContent string, actualPath string) {
	actualData, err := ioutil.ReadFile(actualPath)
	require.NoError(t, err)
	require.Equal(t, []byte(expectedContent), actualData)
}

func CheckExpectedReferencesInRepository(t *testing.T, repositoryPath string, expectedReferences []string) {
	localRepository, err := git.PlainOpen(repositoryPath)
	require.NoError(t, err)
	referenceIterator, err := localRepository.References()
	require.NoError(t, err)
	actualReferences := []string{}
	err = referenceIterator.ForEach(func(reference *plumbing.Reference) error {
		referenceString := reference.String()
		if referenceString != "ref: refs/heads/master HEAD" {
			actualReferences = append(actualReferences, referenceString)
		}
		return nil
	})
	require.NoError(t, err)
	require.ElementsMatch(t, expectedReferences, actualReferences)
}
