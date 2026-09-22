package pull

import (
	"context"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/github/codeql-action-sync/internal/cachedirectory"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"

	"github.com/github/codeql-action-sync/test"
	"github.com/google/go-github/v32/github"
)

const initialActionRepository = "./pull_test/codeql-action-initial.git"
const modifiedActionRepository = "./pull_test/codeql-action-modified.git"

const releaseSomeCodeQLVersionOnMainContent = "This isn't really a CodeQL bundle!"

var releaseSomeCodeQLVersionOnMain = github.RepositoryRelease{
	TagName: github.String("some-codeql-version-on-main"),
	Name:    github.String("some-codeql-version-on-main"),
	Assets: []*github.ReleaseAsset{
		&github.ReleaseAsset{
			ID:   github.Int64(1),
			Name: github.String("codeql-bundle.tar.gz"),
			Size: github.Int(len(releaseSomeCodeQLVersionOnMainContent)),
		},
	},
}

const releaseSomeCodeQLVersionOnV1AndV2Content = "This isn't a CodeQL bundle either, but it's a different not-bundle."

var releaseSomeCodeQLVersionOnV1AndV2 = github.RepositoryRelease{
	TagName: github.String("some-codeql-version-on-v1-and-v2"),
	Name:    github.String("some-codeql-version-on-v1-and-v2"),
	Assets: []*github.ReleaseAsset{
		&github.ReleaseAsset{
			ID:   github.Int64(2),
			Name: github.String("codeql-bundle.tar.gz"),
			Size: github.Int(len(releaseSomeCodeQLVersionOnV1AndV2Content)),
		},
	},
}

const releaseWithMultipleOSAssetsLinux64GzContent = "linux64 gz content"
const releaseWithMultipleOSAssetsLinux64ZstContent = "linux64 zst content"
const releaseWithMultipleOSAssetsWin64GzContent = "win64 gz content"
const releaseWithMultipleOSAssetsCliVersionContent = "1.2.3"

var releaseWithMultipleOSAssets = github.RepositoryRelease{
	TagName: github.String("some-codeql-version-on-main"),
	Name:    github.String("some-codeql-version-on-main"),
	Assets: []*github.ReleaseAsset{
		&github.ReleaseAsset{
			ID:   github.Int64(10),
			Name: github.String("codeql-bundle-linux64.tar.gz"),
			Size: github.Int(len(releaseWithMultipleOSAssetsLinux64GzContent)),
		},
		&github.ReleaseAsset{
			ID:   github.Int64(11),
			Name: github.String("codeql-bundle-linux64.tar.zst"),
			Size: github.Int(len(releaseWithMultipleOSAssetsLinux64ZstContent)),
		},
		&github.ReleaseAsset{
			ID:   github.Int64(12),
			Name: github.String("codeql-bundle-win64.tar.gz"),
			Size: github.Int(len(releaseWithMultipleOSAssetsWin64GzContent)),
		},
		&github.ReleaseAsset{
			ID:   github.Int64(13),
			Name: github.String("cli-version-1.2.3.txt"),
			Size: github.Int(len(releaseWithMultipleOSAssetsCliVersionContent)),
		},
	},
}

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
		"b9f01aa2c50f49898d4c7845a66be8824499fe9d refs/heads/main",
		"26936381e619a01122ea33993e3cebc474496805 refs/heads/v1",
		"e529a54fad10a936308b2220e05f7f00757f8e7c refs/heads/v3",
		"26936381e619a01122ea33993e3cebc474496805 refs/tags/v2",
		// It is expected that we still pull these even though they don't match the expected pattern. We just ignore them later on.
		"bd82b85707bc13904e3526517677039d4da4a9bb refs/heads/very-ignored-branch",
		"bd82b85707bc13904e3526517677039d4da4a9bb refs/tags/an-ignored-tag-too",
		"26936381e619a01122ea33993e3cebc474496805 refs/heads/a-ref-that-will-need-pruning",
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
		"b9f01aa2c50f49898d4c7845a66be8824499fe9d refs/heads/main",
		"26936381e619a01122ea33993e3cebc474496805 refs/heads/v1",
		"33d42021633d74bcd0bf9c95e3d3159131a5faa7 refs/heads/v3", // v3 was force-pushed, and should have been force-pulled too.
		"42d077b4730d1ba413f7bb7e0fa7c98653fb0c78 refs/heads/v4", // v4 is a new branch.
		"bd82b85707bc13904e3526517677039d4da4a9bb refs/tags/an-ignored-tag-too",
		"26936381e619a01122ea33993e3cebc474496805 refs/heads/a-ref-that-will-need-pruning/because-it-now-has-this-extra-bit",
	})
}

func TestShouldDownloadAsset(t *testing.T) {
	cases := []struct {
		name                   string
		assetName              string
		assetOSIncludes        []string
		assetOSExcludes        []string
		assetCompressionFormat string
		expected               bool
	}{
		{"no filters, gz asset", "codeql-bundle-linux64.tar.gz", nil, nil, "", true},
		{"no filters, zst asset", "codeql-bundle-linux64.tar.zst", nil, nil, "", true},
		{"no filters, non-OS asset", "cli-version-1.2.3.txt", nil, nil, "", true},
		{"os-include matches", "codeql-bundle-linux64.tar.gz", []string{"linux64", "win64"}, nil, "", true},
		{"os-include does not match", "codeql-bundle-osx64.tar.gz", []string{"linux64", "win64"}, nil, "", false},
		{"os-include ignores non-OS asset", "cli-version-1.2.3.txt", []string{"linux64"}, nil, "", true},
		{"os-exclude matches", "codeql-bundle-win64.tar.gz", nil, []string{"win64"}, "", false},
		{"os-exclude does not match", "codeql-bundle-linux64.tar.gz", nil, []string{"win64"}, "", true},
		{"os-exclude ignores non-OS asset", "cli-version-1.2.3.txt", nil, []string{"linux64"}, "", true},
		{"compression format matches", "codeql-bundle-linux64.tar.zst", nil, nil, "zst", true},
		{"compression format does not match", "codeql-bundle-linux64.tar.gz", nil, nil, "zst", false},
		{"compression format ignores non-OS asset", "cli-version-1.2.3.txt", nil, nil, "zst", true},
		{"checksum file follows same rules as its asset", "codeql-bundle-linux64.tar.gz.checksum.txt", nil, nil, "zst", false},
		{"os-include and compression format combined", "codeql-bundle-linux-arm64.tar.gz", []string{"linux-arm64"}, nil, "gz", true},
		{"os-include and compression format combined, format mismatch", "codeql-bundle-linux-arm64.tar.zst", []string{"linux-arm64"}, nil, "gz", false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			pullService := pullService{
				assetOSIncludes:        testCase.assetOSIncludes,
				assetOSExcludes:        testCase.assetOSExcludes,
				assetCompressionFormat: testCase.assetCompressionFormat,
			}
			require.Equal(t, testCase.expected, pullService.shouldDownloadAsset(testCase.assetName))
		})
	}
}

func TestWarnOnUnmatchedOSFilters(t *testing.T) {
	hook := logtest.NewGlobal()
	defer hook.Reset()

	pullService := pullService{
		assetOSIncludes: []string{"linux", "win64"},
		seenAssetOSs:    map[string]bool{"win64": true, "osx64": true},
	}
	pullService.warnOnUnmatchedOSFilters()

	require.Len(t, hook.Entries, 1)
	require.Equal(t, log.WarnLevel, hook.Entries[0].Level)
	require.Contains(t, hook.Entries[0].Message, `The --os-include value "linux" did not match any release asset`)
	require.Contains(t, hook.Entries[0].Message, "osx64, win64")
}

func TestWarnOnUnmatchedOSFiltersNoWarningWhenAllMatch(t *testing.T) {
	hook := logtest.NewGlobal()
	defer hook.Reset()

	pullService := pullService{
		assetOSIncludes: []string{"linux64"},
		assetOSExcludes: []string{"win64"},
		seenAssetOSs:    map[string]bool{"linux64": true, "win64": true, "osx64": true},
	}
	pullService.warnOnUnmatchedOSFilters()

	require.Empty(t, hook.Entries)
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
		test.ServeHTTPResponseFromObject(t, releaseSomeCodeQLVersionOnMain, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/assets/1", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromString(t, releaseSomeCodeQLVersionOnMainContent, response)
	}).Methods("GET").Headers("accept", "application/octet-stream")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/tags/some-codeql-version-on-v1-and-v2", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, releaseSomeCodeQLVersionOnV1AndV2, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/assets/2", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromString(t, releaseSomeCodeQLVersionOnV1AndV2Content, response)
	}).Methods("GET").Headers("accept", "application/octet-stream")
	pullService := getTestPullService(t, temporaryDirectory, initialActionRepository, githubURL)
	err := pullService.pullGit(true)
	require.NoError(t, err)
	err = pullService.pullReleases()
	require.NoError(t, err)

	test.RequireFileHasContent(t, releaseSomeCodeQLVersionOnMainContent, pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "codeql-bundle.tar.gz"))
	test.RequireFileHasContent(t, releaseSomeCodeQLVersionOnV1AndV2Content, pullService.cacheDirectory.AssetPath("some-codeql-version-on-v1-and-v2", "codeql-bundle.tar.gz"))

	// If we pull again, we should only download assets where the size mismatches.
	err = ioutil.WriteFile(pullService.cacheDirectory.AssetPath("some-codeql-version-on-v1-and-v2", "codeql-bundle.tar.gz"), []byte("Some nonsense."), 0644)
	require.NoError(t, err)
	githubTestServer, githubURL = test.GetTestHTTPServer(t)
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/tags/some-codeql-version-on-main", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, releaseSomeCodeQLVersionOnMain, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/tags/some-codeql-version-on-v1-and-v2", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, releaseSomeCodeQLVersionOnV1AndV2, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/assets/2", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromString(t, releaseSomeCodeQLVersionOnV1AndV2Content, response)
	}).Methods("GET").Headers("accept", "application/octet-stream")
	pullService = getTestPullService(t, temporaryDirectory, initialActionRepository, githubURL)
	err = pullService.pullReleases()
	require.NoError(t, err)

	test.RequireFileHasContent(t, releaseSomeCodeQLVersionOnMainContent, pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "codeql-bundle.tar.gz"))
	test.RequireFileHasContent(t, releaseSomeCodeQLVersionOnV1AndV2Content, pullService.cacheDirectory.AssetPath("some-codeql-version-on-v1-and-v2", "codeql-bundle.tar.gz"))
}

func TestPullReleasesWithOSAndCompressionFilters(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	githubTestServer, githubURL := test.GetTestHTTPServer(t)
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/tags/some-codeql-version-on-main", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, releaseWithMultipleOSAssets, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/assets/10", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromString(t, releaseWithMultipleOSAssetsLinux64GzContent, response)
	}).Methods("GET").Headers("accept", "application/octet-stream")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/assets/13", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromString(t, releaseWithMultipleOSAssetsCliVersionContent, response)
	}).Methods("GET").Headers("accept", "application/octet-stream")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/tags/some-codeql-version-on-v1-and-v2", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, releaseSomeCodeQLVersionOnV1AndV2, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/assets/2", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromString(t, releaseSomeCodeQLVersionOnV1AndV2Content, response)
	}).Methods("GET").Headers("accept", "application/octet-stream")

	pullService := getTestPullService(t, temporaryDirectory, initialActionRepository, githubURL)
	pullService.assetOSIncludes = []string{"linux64"}
	pullService.assetCompressionFormat = "gz"
	err := pullService.pullGit(true)
	require.NoError(t, err)
	err = pullService.pullReleases()
	require.NoError(t, err)

	// The included OS + compression format asset, and the non-OS-specific asset, should be downloaded.
	test.RequireFileHasContent(t, releaseWithMultipleOSAssetsLinux64GzContent, pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "codeql-bundle-linux64.tar.gz"))
	test.RequireFileHasContent(t, releaseWithMultipleOSAssetsCliVersionContent, pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "cli-version-1.2.3.txt"))

	// The other OS/compression format combinations should have been skipped entirely.
	require.NoFileExists(t, pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "codeql-bundle-linux64.tar.zst"))
	require.NoFileExists(t, pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "codeql-bundle-win64.tar.gz"))
}
