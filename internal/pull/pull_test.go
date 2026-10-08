package pull

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"testing"

	"github.com/github/codeql-action-sync/internal/cachedirectory"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
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

func getTestPullServiceWithAssets(t *testing.T, names ...string) pullService {
	t.Helper()
	router, githubURL := test.GetTestHTTPServer(t)
	assets := releaseAssets(names...)
	contents := map[int]string{}
	for index, asset := range assets {
		id := index + 1
		contents[id] = asset.GetName()
		asset.ID = github.Int64(int64(id))
		asset.Size = github.Int(len(contents[id]))
	}
	for _, tag := range []string{"some-codeql-version-on-main", "some-codeql-version-on-v1-and-v2"} {
		release := github.RepositoryRelease{TagName: github.String(tag), Assets: assets}
		router.HandleFunc("/api/v3/repos/github/codeql-action/releases/tags/"+tag, func(response http.ResponseWriter, request *http.Request) {
			test.ServeHTTPResponseFromObject(t, release, response)
		}).Methods("GET")
	}
	router.HandleFunc("/api/v3/repos/github/codeql-action/releases/assets/{id:[0-9]+}", func(response http.ResponseWriter, request *http.Request) {
		id, err := strconv.Atoi(mux.Vars(request)["id"])
		require.NoError(t, err)
		content, exists := contents[id]
		require.True(t, exists)
		test.ServeHTTPResponseFromString(t, content, response)
	}).Methods("GET").Headers("accept", "application/octet-stream")

	service := getTestPullService(t, test.CreateTemporaryDirectory(t), initialActionRepository, githubURL)
	require.NoError(t, service.pullGit(true))
	return service
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

func TestPullReleasesFiltersAndPrunesCache(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	githubTestServer, githubURL := test.GetTestHTTPServer(t)
	contents := map[int]string{
		10: "linux zstd",
		13: "version metadata",
		14: "linux proxy",
		20: "other linux zstd",
	}
	mainRelease := github.RepositoryRelease{
		TagName: github.String("some-codeql-version-on-main"),
		Assets: []*github.ReleaseAsset{
			{ID: github.Int64(10), Name: github.String("codeql-bundle-linux64.tar.zst"), Size: github.Int(len(contents[10]))},
			{ID: github.Int64(11), Name: github.String("codeql-bundle-linux64.tar.gz"), Size: github.Int(1)},
			{ID: github.Int64(12), Name: github.String("codeql-bundle.tar.zst"), Size: github.Int(1)},
			{ID: github.Int64(13), Name: github.String("cli-version-2.27.2.txt"), Size: github.Int(len(contents[13]))},
			{ID: github.Int64(14), Name: github.String("update-job-proxy-linux64.tar.gz"), Size: github.Int(len(contents[14]))},
			{ID: github.Int64(15), Name: github.String("codeql-bundle-win64.tar.zst"), Size: github.Int(1)},
		},
	}
	otherRelease := github.RepositoryRelease{
		TagName: github.String("some-codeql-version-on-v1-and-v2"),
		Assets: []*github.ReleaseAsset{
			{ID: github.Int64(20), Name: github.String("codeql-bundle-linux64.tar.zst"), Size: github.Int(len(contents[20]))},
		},
	}
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/tags/some-codeql-version-on-main", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, mainRelease, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/tags/some-codeql-version-on-v1-and-v2", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, otherRelease, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/github/codeql-action/releases/assets/{id:[0-9]+}", func(response http.ResponseWriter, request *http.Request) {
		id, err := strconv.Atoi(mux.Vars(request)["id"])
		require.NoError(t, err)
		content, expected := contents[id]
		require.True(t, expected, "asset %d should have been filtered", id)
		test.ServeHTTPResponseFromString(t, content, response)
	}).Methods("GET").Headers("accept", "application/octet-stream")

	pullService := getTestPullService(t, temporaryDirectory, initialActionRepository, githubURL)
	pullService.assetFilter, _ = newReleaseAssetFilter([]string{"linux64"}, nil, "tar.zst")
	err := pullService.pullGit(true)
	require.NoError(t, err)
	staleAssetsPath := pullService.cacheDirectory.AssetsPath("some-codeql-version-on-main")
	require.NoError(t, os.MkdirAll(staleAssetsPath, 0755))
	staleAssetPath := path.Join(staleAssetsPath, "codeql-bundle-osx64.tar.gz")
	require.NoError(t, ioutil.WriteFile(staleAssetPath, []byte("stale"), 0644))

	err = pullService.pullReleases()
	require.NoError(t, err)

	require.NoFileExists(t, staleAssetPath)
	test.RequireFileHasContent(t, contents[10], pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "codeql-bundle-linux64.tar.zst"))
	test.RequireFileHasContent(t, contents[13], pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "cli-version-2.27.2.txt"))
	test.RequireFileHasContent(t, contents[14], pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "update-job-proxy-linux64.tar.gz"))
	require.NoFileExists(t, pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "codeql-bundle-linux64.tar.gz"))
	require.NoFileExists(t, pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "codeql-bundle.tar.zst"))
	require.NoFileExists(t, pullService.cacheDirectory.AssetPath("some-codeql-version-on-main", "codeql-bundle-win64.tar.zst"))
	test.RequireFileHasContent(t, contents[20], pullService.cacheDirectory.AssetPath("some-codeql-version-on-v1-and-v2", "codeql-bundle-linux64.tar.zst"))
}

func TestPullReleasesFiltersHistoricalCache(t *testing.T) {
	names := []string{
		"codeql-bundle-linux64.tar.gz",
		"codeql-bundle-linux64.tar.gz.checksum.txt",
		"codeql-bundle-linux64.tar.zst",
		"codeql-bundle-linux64.tar.zst.checksum.txt",
		"codeql-bundle-win64.tar.gz",
		"codeql-bundle-win64.tar.zst",
		"codeql-bundle.tar.gz",
		"codeql-bundle.tar.zst",
		"update-job-proxy-linux64.tar.gz",
		"update-job-proxy-win64.tar.gz",
		"cli-version.txt",
		"future-release.tar.gz",
	}
	cases := []struct {
		name                string
		includePlatforms    []string
		excludePlatforms    []string
		bundleArchiveFormat string
		expectedNames       []string
	}{
		{
			name:                "included platform and gzip",
			includePlatforms:    []string{"linux64"},
			bundleArchiveFormat: "tar.gz",
			expectedNames: []string{
				"codeql-bundle-linux64.tar.gz",
				"codeql-bundle-linux64.tar.gz.checksum.txt",
				"update-job-proxy-linux64.tar.gz",
				"cli-version.txt",
				"future-release.tar.gz",
			},
		},
		{
			name:                "excluded platform and zstd",
			excludePlatforms:    []string{"win64"},
			bundleArchiveFormat: "tar.zst",
			expectedNames: []string{
				"codeql-bundle-linux64.tar.zst",
				"codeql-bundle-linux64.tar.zst.checksum.txt",
				"update-job-proxy-linux64.tar.gz",
				"cli-version.txt",
				"future-release.tar.gz",
			},
		},
		{
			name:                "format only",
			bundleArchiveFormat: "tar.gz",
			expectedNames: []string{
				"codeql-bundle-linux64.tar.gz",
				"codeql-bundle-linux64.tar.gz.checksum.txt",
				"codeql-bundle-win64.tar.gz",
				"codeql-bundle.tar.gz",
				"update-job-proxy-linux64.tar.gz",
				"update-job-proxy-win64.tar.gz",
				"cli-version.txt",
				"future-release.tar.gz",
			},
		},
		{
			name:          "no filters preserve history",
			expectedNames: names,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := getTestPullServiceWithAssets(t, names...)
			var err error
			service.assetFilter, err = newReleaseAssetFilter(testCase.includePlatforms, testCase.excludePlatforms, testCase.bundleArchiveFormat)
			require.NoError(t, err)

			historicalTag := "an-ignored-tag-too"
			require.NoError(t, os.MkdirAll(service.cacheDirectory.AssetsPath(historicalTag), 0755))
			metadata, err := json.Marshal(github.RepositoryRelease{TagName: github.String(historicalTag)})
			require.NoError(t, err)
			require.NoError(t, ioutil.WriteFile(service.cacheDirectory.MetadataPath(historicalTag), metadata, 0644))
			for _, name := range names {
				require.NoError(t, ioutil.WriteFile(service.cacheDirectory.AssetPath(historicalTag, name), []byte("historical "+name), 0644))
			}

			require.NoError(t, service.pullReleases())
			cachedAssets, err := ioutil.ReadDir(service.cacheDirectory.AssetsPath(historicalTag))
			require.NoError(t, err)
			actualNames := []string{}
			for _, asset := range cachedAssets {
				actualNames = append(actualNames, asset.Name())
				test.RequireFileHasContent(t, "historical "+asset.Name(), service.cacheDirectory.AssetPath(historicalTag, asset.Name()))
			}
			require.ElementsMatch(t, testCase.expectedNames, actualNames)
			test.RequireFileHasContent(t, string(metadata), service.cacheDirectory.MetadataPath(historicalTag))
		})
	}
}

func TestPullReleasesWarnsWhenRestrictingBundleFormats(t *testing.T) {
	for _, format := range []string{"tar.gz", ""} {
		t.Run("format="+format, func(t *testing.T) {
			gzipName := "codeql-bundle-linux64.tar.gz"
			zstdName := "codeql-bundle-linux64.tar.zst"
			service := getTestPullServiceWithAssets(t, gzipName, zstdName)
			var err error
			service.assetFilter, err = newReleaseAssetFilter([]string{"linux64"}, nil, format)
			require.NoError(t, err)

			var logged bytes.Buffer
			previousOutput := log.StandardLogger().Out
			log.SetOutput(&logged)
			t.Cleanup(func() { log.SetOutput(previousOutput) })

			require.NoError(t, service.pullReleases())
			test.RequireFileHasContent(t, gzipName, service.cacheDirectory.AssetPath("some-codeql-version-on-main", gzipName))
			if format != "" {
				require.NoFileExists(t, service.cacheDirectory.AssetPath("some-codeql-version-on-main", zstdName))
				require.Contains(t, logged.String(), "--bundle-archive-format")
				require.Contains(t, logged.String(), "tools input with an explicit URL")
				require.Contains(t, logged.String(), "GitHub.com")
			} else {
				test.RequireFileHasContent(t, zstdName, service.cacheDirectory.AssetPath("some-codeql-version-on-main", zstdName))
				require.NotContains(t, logged.String(), "--bundle-archive-format")
			}
		})
	}
}
