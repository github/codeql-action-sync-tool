package pull

import (
	"context"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/github/codeql-action-sync/internal/cachedirectory"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"

	"github.com/github/codeql-action-sync/test"
	"github.com/google/go-github/v32/github"
)

const initialActionRepository = "./pull_test/codeql-action-initial.git"
const modifiedActionRepository = "./pull_test/codeql-action-modified.git"

func getTestPullService(t *testing.T, temporaryDirectory string, gitCloneURL string, githubURL string) pullService {
	cacheDirectory := cachedirectory.NewCacheDirectory(temporaryDirectory)
	var githubDotComClient *github.Client
	if githubURL != "" {
		client, err := github.NewEnterpriseClient(githubURL+"/api/v3", githubURL+"/api/uploads", &http.Client{})
		githubDotComClient = client
		require.NoError(t, err)
	} else {
		githubDotComClient = nil
	}
	return pullService{
		ctx:                context.Background(),
		cacheDirectory:     cacheDirectory,
		gitCloneURL:        gitCloneURL,
		githubDotComClient: githubDotComClient,
	}
}

func checkExpectedReferencesInCache(t *testing.T, cacheDirectory cachedirectory.CacheDirectory, expectedReferences []string) {
	localRepository, err := git.PlainOpen(cacheDirectory.GitPath())
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

func TestPullGitFresh(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	pullService := getTestPullService(t, temporaryDirectory, initialActionRepository, "")
	err := pullService.pullGit(true)
	require.NoError(t, err)
	test.CheckExpectedReferencesInRepository(t, pullService.cacheDirectory.GitPath(), []string{
		"b9f01aa2c50f49898d4c7845a66be8824499fe9d refs/remotes/origin/heads/main",
		"26936381e619a01122ea33993e3cebc474496805 refs/remotes/origin/heads/v1",
		"e529a54fad10a936308b2220e05f7f00757f8e7c refs/remotes/origin/heads/v3",
		"26936381e619a01122ea33993e3cebc474496805 refs/remotes/origin/tags/v2",
		// It is expected that we still pull these even though they don't match the expected pattern. We just ignore them later on.
		"bd82b85707bc13904e3526517677039d4da4a9bb refs/remotes/origin/heads/very-ignored-branch",
		"bd82b85707bc13904e3526517677039d4da4a9bb refs/remotes/origin/tags/an-ignored-tag-too",
	})
}

func TestPullGitNotFreshReturnsErrorIfNoCache(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	pullService := getTestPullService(t, temporaryDirectory, initialActionRepository, "")
	err := pullService.pullGit(false)
	require.Error(t, err)
}

func TestPullGitNotFreshNoChanges(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	pullService := getTestPullService(t, temporaryDirectory, initialActionRepository, "")
	err := pullService.pullGit(true)
	require.NoError(t, err)
	err = pullService.pullGit(false)
	require.NoError(t, err)
}

func TestPullGitNotFreshWithChanges(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	pullService := getTestPullService(t, temporaryDirectory, initialActionRepository, "")
	err := pullService.pullGit(true)
	require.NoError(t, err)
	pullService = getTestPullService(t, temporaryDirectory, modifiedActionRepository, "")
	err = pullService.pullGit(false)
	require.NoError(t, err)
	test.CheckExpectedReferencesInRepository(t, pullService.cacheDirectory.GitPath(), []string{
		"b9f01aa2c50f49898d4c7845a66be8824499fe9d refs/remotes/origin/heads/main",
		"26936381e619a01122ea33993e3cebc474496805 refs/remotes/origin/heads/v1",
		"33d42021633d74bcd0bf9c95e3d3159131a5faa7 refs/remotes/origin/heads/v3", // v3 was force-pushed, and should have been force-pulled too.
		"26936381e619a01122ea33993e3cebc474496805 refs/remotes/origin/tags/v2",
		"42d077b4730d1ba413f7bb7e0fa7c98653fb0c78 refs/remotes/origin/heads/v4", // v4 is a new branch.
		// We deleted these, but we don't currently do any pruning.
		"bd82b85707bc13904e3526517677039d4da4a9bb refs/remotes/origin/heads/very-ignored-branch",
		"bd82b85707bc13904e3526517677039d4da4a9bb refs/remotes/origin/tags/an-ignored-tag-too",
	})
}

func TestFindRelevantReleases(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	pullService := getTestPullService(t, temporaryDirectory, initialActionRepository, "")
	err := pullService.pullGit(true)
	require.NoError(t, err)
	relevantReleases, err := pullService.findRelevantReleases()
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"some-codeql-version-on-main",
		"some-codeql-version-on-v1-and-v2",
		// v3 intentionally matches the patten for a release branch but has no configuration so it should be ignored with a warning.
	}, relevantReleases)
}

func TestPullReleases(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	githubTestServer, githubURL := test.GetTestHTTPServer(t)
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/tags/some-codeql-version-on-main", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromFile(t, http.StatusOK, "./pull_test/api/release-some-codeql-version-on-main.json", response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/assets/1", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromFile(t, http.StatusOK, "./pull_test/api/asset-some-codeql-version-on-main.bin", response)
	}).Methods("GET").Headers("accept", "application/octet-stream")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/tags/some-codeql-version-on-v1-and-v2", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromFile(t, http.StatusOK, "./pull_test/api/release-some-codeql-version-on-v1-and-v2.json", response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/assets/2", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromFile(t, http.StatusOK, "./pull_test/api/asset-some-codeql-version-on-v1-and-v2.bin", response)
	}).Methods("GET").Headers("accept", "application/octet-stream")
	pullService := getTestPullService(t, temporaryDirectory, initialActionRepository, githubURL)
	err := pullService.pullGit(true)
	require.NoError(t, err)
	err = pullService.pullReleases()
	require.NoError(t, err)

	test.RequireFilesAreEqual(t, "./pull_test/api/asset-some-codeql-version-on-main.bin", pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "codeql-bundle.tar.gz"))
	test.RequireFilesAreEqual(t, "./pull_test/api/asset-some-codeql-version-on-v1-and-v2.bin", pullService.cacheDirectory.AssetPath("some-codeql-version-on-v1-and-v2", "codeql-bundle.tar.gz"))

	// If we pull again, we should only download assets where the size mismatches.
	err = ioutil.WriteFile(pullService.cacheDirectory.AssetPath("some-codeql-version-on-v1-and-v2", "codeql-bundle.tar.gz"), []byte("Some nonsense."), 0644)
	require.NoError(t, err)
	githubTestServer, githubURL = test.GetTestHTTPServer(t)
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/tags/some-codeql-version-on-main", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromFile(t, http.StatusOK, "./pull_test/api/release-some-codeql-version-on-main.json", response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/tags/some-codeql-version-on-v1-and-v2", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromFile(t, http.StatusOK, "./pull_test/api/release-some-codeql-version-on-v1-and-v2.json", response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/assets/2", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromFile(t, http.StatusOK, "./pull_test/api/asset-some-codeql-version-on-v1-and-v2.bin", response)
	}).Methods("GET").Headers("accept", "application/octet-stream")
	pullService = getTestPullService(t, temporaryDirectory, initialActionRepository, githubURL)
	err = pullService.pullReleases()
	require.NoError(t, err)

	test.RequireFilesAreEqual(t, "./pull_test/api/asset-some-codeql-version-on-main.bin", pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "codeql-bundle.tar.gz"))
	test.RequireFilesAreEqual(t, "./pull_test/api/asset-some-codeql-version-on-v1-and-v2.bin", pullService.cacheDirectory.AssetPath("some-codeql-version-on-v1-and-v2", "codeql-bundle.tar.gz"))
}
