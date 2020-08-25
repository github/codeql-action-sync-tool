package push

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net/http"
	"path"
	"strconv"
	"testing"

	"github.com/github/codeql-action-sync/internal/cachedirectory"
	"github.com/github/codeql-action-sync/test"
	"github.com/go-git/go-git/v5"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/google/go-github/v32/github"
)

func getTestPushService(t *testing.T, cacheDirectoryString string, githubEnterpriseURL string) pushService {
	cacheDirectory := cachedirectory.NewCacheDirectory(cacheDirectoryString)
	var githubEnterpriseClient *github.Client
	if githubEnterpriseURL != "" {
		client, err := github.NewEnterpriseClient(githubEnterpriseURL+"/api/v3", githubEnterpriseURL+"/api/uploads", &http.Client{})
		githubEnterpriseClient = client
		require.NoError(t, err)
	} else {
		githubEnterpriseClient = nil
	}
	return pushService{
		ctx:                        context.Background(),
		cacheDirectory:             cacheDirectory,
		githubEnterpriseClient:     githubEnterpriseClient,
		destinationRepositoryOwner: "destination-repository-owner",
		destinationRepositoryName:  "destination-repository-name",
		destinationToken:           "token",
	}
}

func TestCreateRepositoryWhenUserIsOwner(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	githubTestServer, githubEnterpriseURL := test.GetTestHTTPServer(t)
	pushService := getTestPushService(t, temporaryDirectory, githubEnterpriseURL)
	githubTestServer.HandleFunc("/api/v3/user", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, github.User{Login: github.String("destination-repository-owner")}, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/destination-repository-owner/destination-repository-name", func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusNotFound)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/user/repos", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, github.Repository{}, response)
	}).Methods("POST")
	_, err := pushService.createRepository()
	require.NoError(t, err)
}

func TestUpdateRepositoryWhenUserIsOwner(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	githubTestServer, githubEnterpriseURL := test.GetTestHTTPServer(t)
	pushService := getTestPushService(t, temporaryDirectory, githubEnterpriseURL)
	githubTestServer.HandleFunc("/api/v3/user", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, github.User{Login: github.String("destination-repository-owner")}, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/destination-repository-owner/destination-repository-name", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, github.Repository{Homepage: github.String(repositoryHomepage)}, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/destination-repository-owner/destination-repository-name", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, github.Repository{}, response)
	}).Methods("PATCH")
	_, err := pushService.createRepository()
	require.NoError(t, err)
}

func TestUpdateRepositoryWhenUserIsOwnerForced(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	githubTestServer, githubEnterpriseURL := test.GetTestHTTPServer(t)
	pushService := getTestPushService(t, temporaryDirectory, githubEnterpriseURL)
	githubTestServer.HandleFunc("/api/v3/user", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, github.User{Login: github.String("destination-repository-owner")}, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/destination-repository-owner/destination-repository-name", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, github.Repository{}, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/destination-repository-owner/destination-repository-name", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, github.Repository{}, response)
	}).Methods("PATCH")
	_, err := pushService.createRepository()
	require.EqualError(t, err, errorAlreadyExists)
	pushService.force = true
	_, err = pushService.createRepository()
	require.NoError(t, err)
}

func TestCreateOrganizationAndRepositoryWhenOrganizationIsOwner(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	githubTestServer, githubEnterpriseURL := test.GetTestHTTPServer(t)
	pushService := getTestPushService(t, temporaryDirectory, githubEnterpriseURL)
	organizationCreated := false
	githubTestServer.HandleFunc("/api/v3/user", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, github.User{Login: github.String("user")}, response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/orgs/destination-repository-owner", func(response http.ResponseWriter, request *http.Request) {
		if organizationCreated {
			test.ServeHTTPResponseFromObject(t, github.Organization{}, response)
		} else {
			response.WriteHeader(http.StatusNotFound)
		}
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/admin/organizations", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, github.Organization{}, response)
		organizationCreated = true
	}).Methods("POST")
	githubTestServer.HandleFunc("/api/v3/repos/destination-repository-owner/destination-repository-name", func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusNotFound)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/orgs/destination-repository-owner/repos", func(response http.ResponseWriter, request *http.Request) {
		test.ServeHTTPResponseFromObject(t, github.Repository{}, response)
	}).Methods("POST")
	_, err := pushService.createRepository()
	require.NoError(t, err)
}

func TestPushGit(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	destinationPath := path.Join(temporaryDirectory, "target")
	_, err := git.PlainInit(destinationPath, true)
	require.NoError(t, err)
	pushService := getTestPushService(t, "./push_test/action-cache-initial/", "")
	repository := github.Repository{
		CloneURL: github.String(destinationPath),
	}

	err = pushService.pushGit(&repository, true)
	require.NoError(t, err)
	test.CheckExpectedReferencesInRepository(t, destinationPath, []string{
		"26936381e619a01122ea33993e3cebc474496805 refs/tags/codeql-bundle-20200101",
		"26936381e619a01122ea33993e3cebc474496805 refs/tags/codeql-bundle-20200630",
	})

	err = pushService.pushGit(&repository, false)
	require.NoError(t, err)
	test.CheckExpectedReferencesInRepository(t, destinationPath, []string{
		"26936381e619a01122ea33993e3cebc474496805 refs/tags/codeql-bundle-20200101",
		"26936381e619a01122ea33993e3cebc474496805 refs/tags/codeql-bundle-20200630",
		"b9f01aa2c50f49898d4c7845a66be8824499fe9d refs/heads/main",
		"26936381e619a01122ea33993e3cebc474496805 refs/heads/v1",
		"e529a54fad10a936308b2220e05f7f00757f8e7c refs/heads/v3",
		"bd82b85707bc13904e3526517677039d4da4a9bb refs/heads/very-ignored-branch",
		"bd82b85707bc13904e3526517677039d4da4a9bb refs/tags/an-ignored-tag-too",
		"26936381e619a01122ea33993e3cebc474496805 refs/tags/v2",
	})
}

func TestPushReleases(t *testing.T) {
	githubTestServer, githubEnterpriseURL := test.GetTestHTTPServer(t)
	pushService := getTestPushService(t, "./push_test/action-cache-initial/", githubEnterpriseURL)
	existingReleases := map[string]github.RepositoryRelease{}
	existingAssets := map[int][]github.ReleaseAsset{}
	existingAssetBodys := map[int]map[string][]byte{}
	githubTestServer.HandleFunc("/api/v3/repos/destination-repository-owner/destination-repository-name/releases/tags/{tag}", func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		if value, ok := existingReleases[vars["tag"]]; ok {
			test.ServeHTTPResponseFromObject(t, value, response)
		} else {
			response.WriteHeader(http.StatusNotFound)
		}
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/v3/repos/destination-repository-owner/destination-repository-name/releases", func(response http.ResponseWriter, request *http.Request) {
		body, err := ioutil.ReadAll(request.Body)
		require.NoError(t, err)
		var release *github.RepositoryRelease
		err = json.Unmarshal(body, &release)
		require.NoError(t, err)
		release.ID = github.Int64(rand.Int63())
		existingReleases[release.GetTagName()] = *release
		test.ServeHTTPResponseFromObject(t, release, response)
	}).Methods("POST")
	githubTestServer.HandleFunc("/api/v3/repos/destination-repository-owner/destination-repository-name/releases/{id:[0-9]+}/assets", func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		releaseID, err := strconv.Atoi(vars["id"])
		require.NoError(t, err)
		test.ServeHTTPResponseFromObject(t, existingAssets[releaseID], response)
	}).Methods("GET")
	githubTestServer.HandleFunc("/api/uploads/repos/destination-repository-owner/destination-repository-name/releases/{id:[0-9]+}/assets", func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		releaseID, err := strconv.Atoi(vars["id"])
		require.NoError(t, err)
		assetName := request.URL.Query().Get("name")
		asset := github.ReleaseAsset{
			Name: github.String(assetName),
		}
		existingAssets[releaseID] = append(existingAssets[releaseID], asset)
		if existingAssetBodys[releaseID] == nil {
			existingAssetBodys[releaseID] = map[string][]byte{}
		}
		existingAssetBodys[releaseID][assetName], err = ioutil.ReadAll(request.Body)
		require.NoError(t, err)
		test.ServeHTTPResponseFromObject(t, asset, response)
	}).Methods("POST")
	err := pushService.pushReleases()
	require.NoError(t, err)
}
