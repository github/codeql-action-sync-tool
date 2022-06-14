package push

import (
	"context"
	"encoding/json"
	usererrors "errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/github/codeql-action-sync/internal/githubapiutil"

	log "github.com/sirupsen/logrus"

	"github.com/github/codeql-action-sync/internal/cachedirectory"
	"github.com/github/codeql-action-sync/internal/version"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v32/github"
	"github.com/mitchellh/ioprogress"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const repositoryHomepage = "https://github.com/github/codeql-action-sync-tool/"

const errorAlreadyExists = "The destination repository already exists, but it was not created with the CodeQL Action sync tool. If you are sure you want to push the CodeQL Action to it, re-run this command with the `--force` flag."
const errorInvalidDestinationToken = "The destination token you've provided is not valid."

const enterpriseAPIPath = "/api/v3"
const enterpriseUploadsPath = "/api/uploads"
const enterpriseVersionHeaderKey = "X-GitHub-Enterprise-Version"
const enterpriseAegisVersionHeaderValue = "GitHub AE"

type pushService struct {
	ctx                        context.Context
	cacheDirectory             cachedirectory.CacheDirectory
	githubEnterpriseClient     *github.Client
	destinationRepositoryName  string
	destinationRepositoryOwner string
	destinationToken           *oauth2.Token
	actionsAdminUser           string
	aegis                      bool
	force                      bool
	pushSSH                    bool
	gitURL                     string
}

func (pushService *pushService) createRepository() (*github.Repository, error) {
	minimumRepositoryScope := "public_repo"
	acceptableRepositoryScopes := []string{"public_repo", "repo"}
	desiredVisibility := "public"
	if pushService.aegis {
		minimumRepositoryScope = "repo"
		acceptableRepositoryScopes = []string{"repo"}
		desiredVisibility = "internal"
	}

	log.Debug("Ensuring repository exists...")
	user, response, err := pushService.githubEnterpriseClient.Users.Get(pushService.ctx, "")
	if err != nil {
		if response != nil && response.StatusCode == http.StatusUnauthorized {
			return nil, usererrors.New(errorInvalidDestinationToken)
		}
		return nil, githubapiutil.EnrichResponseError(response, err, "Error getting current user.")
	}

	// When creating a repository we can either create it in a named organization or under the current user (represented in go-github by an empty string).
	destinationOrganization := ""
	if pushService.destinationRepositoryOwner != user.GetLogin() {
		destinationOrganization = pushService.destinationRepositoryOwner
	}

	if destinationOrganization != "" {
		_, response, err := pushService.githubEnterpriseClient.Organizations.Get(pushService.ctx, pushService.destinationRepositoryOwner)
		if err != nil && (response == nil || response.StatusCode != http.StatusNotFound) {
			return nil, githubapiutil.EnrichResponseError(response, err, "Error checking if destination organization exists.")
		}
		if response != nil && response.StatusCode == http.StatusNotFound {
			log.Debugf("The organization %s does not exist. Creating it...", pushService.destinationRepositoryOwner)
			_, response, err := pushService.githubEnterpriseClient.Admin.CreateOrg(pushService.ctx, &github.Organization{
				Login: github.String(pushService.destinationRepositoryOwner),
				Name:  github.String(pushService.destinationRepositoryOwner),
			}, user.GetLogin())
			if err != nil {
				if response != nil && response.StatusCode == http.StatusNotFound && !githubapiutil.HasAnyScope(response, "site_admin") {
					return nil, usererrors.New("The destination token you have provided does not have the `site_admin` scope, so the destination organization cannot be created.")
				}
				return nil, githubapiutil.EnrichResponseError(response, err, "Error creating organization.")
			}
		}

		_, response, err = pushService.githubEnterpriseClient.Organizations.IsMember(pushService.ctx, pushService.destinationRepositoryOwner, user.GetLogin())
		if err != nil {
			return nil, githubapiutil.EnrichResponseError(response, err, "Failed to check membership of destination organization.")
		}
		if (response.StatusCode == http.StatusFound || response.StatusCode == http.StatusNotFound) && githubapiutil.HasAnyScope(response, "site_admin") {
			log.Debugf("No access to destination organization (status code %d). Switching to impersonation token for %s...", response.StatusCode, pushService.actionsAdminUser)
			impersonationToken, response, err := pushService.githubEnterpriseClient.Admin.CreateUserImpersonation(pushService.ctx, pushService.actionsAdminUser, &github.ImpersonateUserOptions{Scopes: []string{minimumRepositoryScope, "workflow"}})
			if err != nil {
				return nil, githubapiutil.EnrichResponseError(response, err, "Failed to impersonate Actions admin user.")
			}
			pushService.destinationToken.AccessToken = impersonationToken.GetToken()
		}
	}

	repository, response, err := pushService.githubEnterpriseClient.Repositories.Get(pushService.ctx, pushService.destinationRepositoryOwner, pushService.destinationRepositoryName)
	if err != nil && (response == nil || response.StatusCode != http.StatusNotFound) {
		return nil, githubapiutil.EnrichResponseError(response, err, "Error checking if destination repository exists.")
	}
	if response.StatusCode != http.StatusNotFound && repositoryHomepage != repository.GetHomepage() && !pushService.force {
		return nil, errors.Errorf(errorAlreadyExists)
	}
	desiredRepositoryProperties := github.Repository{
		Name:         github.String(pushService.destinationRepositoryName),
		Homepage:     github.String(repositoryHomepage),
		HasIssues:    github.Bool(false),
		HasProjects:  github.Bool(false),
		HasPages:     github.Bool(false),
		HasWiki:      github.Bool(false),
		HasDownloads: github.Bool(false),
		Archived:     github.Bool(false),
	}
	if repository.GetVisibility() != desiredVisibility {
		// For some reason if you provide a visibility it must be different than the current visibility.
		// It seems to be the only property that behaves this way, so we have to treat is specially...
		desiredRepositoryProperties.Visibility = github.String(desiredVisibility)
	}
	if response.StatusCode == http.StatusNotFound {
		log.Debug("Repository does not exist. Creating it...")
		repository, response, err = pushService.githubEnterpriseClient.Repositories.Create(pushService.ctx, destinationOrganization, &desiredRepositoryProperties)
		if err != nil {
			if response.StatusCode == http.StatusNotFound && !githubapiutil.HasAnyScope(response, acceptableRepositoryScopes...) {
				return nil, fmt.Errorf("The destination token you have provided does not have the `%s` scope.", minimumRepositoryScope)
			}
			return nil, githubapiutil.EnrichResponseError(response, err, "Error creating destination repository.")
		}
	} else {
		log.Debug("Repository already exists. Updating its metadata...")
		repository, response, err = pushService.githubEnterpriseClient.Repositories.Edit(pushService.ctx, pushService.destinationRepositoryOwner, pushService.destinationRepositoryName, &desiredRepositoryProperties)
		if err != nil {
			if response.StatusCode == http.StatusNotFound {
				if !githubapiutil.HasAnyScope(response, acceptableRepositoryScopes...) {
					return nil, fmt.Errorf("The destination token you have provided does not have the `%s` scope.", minimumRepositoryScope)
				} else {
					return nil, fmt.Errorf("You don't have permission to update the repository at %s/%s. If you wish to update the bundled CodeQL Action please provide a token with the `site_admin` scope.", pushService.destinationRepositoryOwner, pushService.destinationRepositoryName)
				}
			}
			return nil, githubapiutil.EnrichResponseError(response, err, "Error updating destination repository.")
		}
	}

	return repository, nil
}

func (pushService *pushService) pushGit(repository *github.Repository, initialPush bool) error {
	remoteURL := pushService.gitURL
	if remoteURL == "" {
		remoteURL = repository.GetCloneURL()
		if pushService.pushSSH {
			remoteURL = repository.GetSSHURL()
		}
	}
	if initialPush {
		log.Debugf("Pushing Git releases to %s...", remoteURL)
	} else {
		log.Debugf("Pushing Git references to %s...", remoteURL)
	}
	gitRepository, err := git.PlainOpen(pushService.cacheDirectory.GitPath())
	if err != nil {
		return errors.Wrap(err, "Error reading Git repository from cache.")
	}

	remote := git.NewRemote(gitRepository.Storer, &config.RemoteConfig{
		Name: git.DefaultRemoteName,
		URLs: []string{remoteURL},
	})

	credentials := &githttp.BasicAuth{
		Username: "x-access-token",
		Password: pushService.destinationToken.AccessToken,
	}
	if pushService.pushSSH {
		// Use the SSH key from the environment.
		credentials = nil
	}

	refSpecBatches := [][]config.RefSpec{}
	remoteReferences, err := remote.List(&git.ListOptions{Auth: credentials})
	if err != nil && err != transport.ErrEmptyRemoteRepository {
		return errors.Wrap(err, "Error listing remote references.")
	}
	deleteRefSpecs := []config.RefSpec{}
	for _, remoteReference := range remoteReferences {
		_, err := gitRepository.Reference(remoteReference.Name(), false)
		if err != nil && err != plumbing.ErrReferenceNotFound {
			return errors.Wrapf(err, "Error finding local reference %s.", remoteReference.Name())
		}
		if err == plumbing.ErrReferenceNotFound {
			deleteRefSpecs = append(deleteRefSpecs, config.RefSpec(":"+remoteReference.Name().String()))
		}
	}
	refSpecBatches = append(refSpecBatches, deleteRefSpecs)

	defaultBranchRefSpec := "+refs/heads/main:refs/heads/main"
	if initialPush {
		releasePathStats, err := ioutil.ReadDir(pushService.cacheDirectory.ReleasesPath())
		if err != nil {
			return errors.Wrap(err, "Error reading releases.")
		}
		initialRefSpecs := []config.RefSpec{}
		for _, releasePathStat := range releasePathStats {
			tagReferenceName := plumbing.NewTagReferenceName(releasePathStat.Name())
			_, err := gitRepository.Reference(tagReferenceName, true)
			if err != nil {
				return errors.Wrapf(err, "Error finding local tag reference %s.", tagReferenceName)
			}
			initialRefSpecs = append(initialRefSpecs, config.RefSpec("+"+tagReferenceName.String()+":"+tagReferenceName.String()))
		}
		refSpecBatches = append(refSpecBatches, initialRefSpecs)
	} else {
		// We've got to push the default branch on its own, so that it will be made the default branch if the repository has just been created. We then push everything else afterwards.
		refSpecBatches = append(refSpecBatches,
			[]config.RefSpec{
				config.RefSpec(defaultBranchRefSpec),
			},
			[]config.RefSpec{
				config.RefSpec("+refs/*:refs/*"),
			},
		)
	}
	for _, refSpecs := range refSpecBatches {
		if len(refSpecs) != 0 {
			err = remote.PushContext(pushService.ctx, &git.PushOptions{
				RefSpecs: refSpecs,
				Auth:     credentials,
				Progress: os.Stderr,
			})
			if err != nil && errors.Cause(err) != git.NoErrAlreadyUpToDate {
				return errors.Wrap(err, "Error pushing Action to GitHub Enterprise Server.")
			}
		}
	}

	return nil
}

func (pushService *pushService) createOrUpdateRelease(releaseName string) (*github.RepositoryRelease, error) {
	releaseMetadata := github.RepositoryRelease{}
	releaseMetadataPath := pushService.cacheDirectory.MetadataPath(releaseName)
	releaseMetadataFile, err := ioutil.ReadFile(releaseMetadataPath)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading release metadata.")
	}
	err = json.Unmarshal([]byte(releaseMetadataFile), &releaseMetadata)
	if err != nil {
		return nil, errors.Wrap(err, "Error converting release from JSON.")
	}
	// Some of our target commitishes are invalid as they point to `main` which we've not pushed yet.
	releaseMetadata.TargetCommitish = nil

	release, response, err := pushService.githubEnterpriseClient.Repositories.GetReleaseByTag(pushService.ctx, pushService.destinationRepositoryOwner, pushService.destinationRepositoryName, releaseMetadata.GetTagName())
	if err != nil && response.StatusCode != http.StatusNotFound {
		return nil, githubapiutil.EnrichResponseError(response, err, "Error checking for existing CodeQL release.")
	}
	if release == nil {
		log.Debugf("Creating release %s...", releaseMetadata.GetTagName())
		release, response, err := pushService.githubEnterpriseClient.Repositories.CreateRelease(pushService.ctx, pushService.destinationRepositoryOwner, pushService.destinationRepositoryName, &releaseMetadata)
		if err != nil {
			return nil, githubapiutil.EnrichResponseError(response, err, "Error creating release.")
		}
		return release, nil
	}
	release, response, err = pushService.githubEnterpriseClient.Repositories.EditRelease(pushService.ctx, pushService.destinationRepositoryOwner, pushService.destinationRepositoryName, release.GetID(), &releaseMetadata)
	if err != nil {
		log.Debugf("Updating release %s...", releaseMetadata.GetTagName())
		return nil, githubapiutil.EnrichResponseError(response, err, "Error updating release.")
	}
	return release, nil
}

func (pushService *pushService) uploadReleaseAsset(release *github.RepositoryRelease, assetPathStat os.FileInfo, reader io.Reader) (*github.ReleaseAsset, *github.Response, error) {
	// This is technically already part of the go-github library, but we re-implement it here since otherwise we can't get a progress bar.
	url := fmt.Sprintf("repos/%s/%s/releases/%d/assets?name=%s", pushService.destinationRepositoryOwner, pushService.destinationRepositoryName, release.GetID(), url.QueryEscape(assetPathStat.Name()))

	mediaType := mime.TypeByExtension(filepath.Ext(assetPathStat.Name()))
	request, err := pushService.githubEnterpriseClient.NewUploadRequest(url, reader, assetPathStat.Size(), mediaType)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error constructing upload request.")
	}

	asset := &github.ReleaseAsset{}
	response, err := pushService.githubEnterpriseClient.Do(pushService.ctx, request, asset)
	if err != nil {
		return nil, response, githubapiutil.EnrichResponseError(response, err, "Error uploading release asset.")
	}
	return asset, response, nil
}

func (pushService *pushService) uploadAsset(release *github.RepositoryRelease, assetPathStat os.FileInfo) (*github.Response, error) {
	assetFile, err := os.Open(pushService.cacheDirectory.AssetPath(release.GetTagName(), assetPathStat.Name()))
	if err != nil {
		return nil, errors.Wrap(err, "Error opening release asset.")
	}
	defer assetFile.Close()
	progressReader := &ioprogress.Reader{
		Reader:   assetFile,
		Size:     assetPathStat.Size(),
		DrawFunc: ioprogress.DrawTerminalf(os.Stderr, ioprogress.DrawTextFormatBytes),
	}
	if err != nil {
		return nil, errors.Wrap(err, "Error opening release asset.")
	}
	_, response, err := pushService.uploadReleaseAsset(release, assetPathStat, progressReader)
	return response, err
}

func (pushService *pushService) createOrUpdateReleaseAsset(release *github.RepositoryRelease, existingAssets []*github.ReleaseAsset, assetPathStat os.FileInfo) error {
	attempt := 0
	for {
		attempt++
		for _, existingAsset := range existingAssets {
			if existingAsset.GetName() == assetPathStat.Name() {
				actualSize := int64(existingAsset.GetSize())
				expectedSize := assetPathStat.Size()
				if actualSize == expectedSize {
					return nil
				} else {
					log.Warnf("Removing existing release asset %s because it was only partially-uploaded (had size %d, but should have been %d)...", existingAsset.GetName(), actualSize, expectedSize)
					response, err := pushService.githubEnterpriseClient.Repositories.DeleteReleaseAsset(pushService.ctx, pushService.destinationRepositoryOwner, pushService.destinationRepositoryName, existingAsset.GetID())
					if err != nil {
						return githubapiutil.EnrichResponseError(response, err, "Error deleting existing release asset.")
					}
				}
			}
		}
		log.Debugf("Uploading release asset %s...", assetPathStat.Name())
		response, err := pushService.uploadAsset(release, assetPathStat)
		if err == nil {
			return nil
		} else {
			if githubErrorResponse := new(github.ErrorResponse); errors.As(err, &githubErrorResponse) {
				for _, innerError := range githubErrorResponse.Errors {
					if innerError.Code == "already_exists" {
						log.Warn("Asset already existed.")
						return nil
					}
				}
			}
			if response == nil || response.StatusCode < 500 || attempt >= 5 {
				return err
			}
			log.Warnf("Attempt %d failed to upload release asset (%s), retrying...", attempt, err.Error())
		}
	}
}

func (pushService *pushService) pushReleases() error {
	log.Debugf("Pushing CodeQL bundles...")
	releasesPath := pushService.cacheDirectory.ReleasesPath()

	releasePathStats, err := ioutil.ReadDir(releasesPath)
	if err != nil {
		return errors.Wrap(err, "Error reading releases.")
	}
	for index, releasePathStat := range releasePathStats {
		releaseName := releasePathStat.Name()
		log.Debugf("Pushing CodeQL bundle %s (%d/%d)...", releaseName, index+1, len(releasePathStats))
		release, err := pushService.createOrUpdateRelease(releaseName)
		if err != nil {
			return err
		}

		existingAssets := []*github.ReleaseAsset{}
		for page := 1; ; page++ {
			assets, response, err := pushService.githubEnterpriseClient.Repositories.ListReleaseAssets(pushService.ctx, pushService.destinationRepositoryOwner, pushService.destinationRepositoryName, release.GetID(), &github.ListOptions{Page: page})
			if err != nil {
				return githubapiutil.EnrichResponseError(response, err, "Error fetching existing release assets.")
			}
			if len(assets) == 0 {
				break
			}
			existingAssets = append(existingAssets, assets...)
		}

		assetsPath := pushService.cacheDirectory.AssetsPath(releaseName)
		assetPathStats, err := ioutil.ReadDir(assetsPath)
		if err != nil {
			return errors.Wrap(err, "Error reading release assets.")
		}
		for _, assetPathStat := range assetPathStats {
			err := pushService.createOrUpdateReleaseAsset(release, existingAssets, assetPathStat)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func Push(ctx context.Context, cacheDirectory cachedirectory.CacheDirectory, destinationURL string, destinationToken string, destinationRepository string, actionsAdminUser string, force bool, pushSSH bool, gitURL string) error {
	err := cacheDirectory.CheckOrCreateVersionFile(false, version.Version())
	if err != nil {
		return err
	}
	err = cacheDirectory.CheckLock()
	if err != nil {
		return err
	}

	destinationURL = strings.TrimRight(destinationURL, "/")
	token := oauth2.Token{AccessToken: destinationToken}
	tokenSource := oauth2.StaticTokenSource(
		&token,
	)
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	client, err := github.NewEnterpriseClient(destinationURL+enterpriseAPIPath, destinationURL+enterpriseUploadsPath, tokenClient)
	if err != nil {
		return errors.Wrap(err, "Error creating GitHub Enterprise client.")
	}
	rootRequest, err := client.NewRequest("GET", enterpriseAPIPath, nil)
	if err != nil {
		return errors.Wrap(err, "Error constructing request for GitHub Enterprise client.")
	}
	rootResponse, err := client.Do(ctx, rootRequest, nil)
	if err != nil {
		return githubapiutil.EnrichResponseError(rootResponse, err, "Error checking connectivity for GitHub Enterprise client.")
	}
	if rootRequest.URL != rootResponse.Request.URL {
		updatedBaseURL, _ := url.Parse(client.BaseURL.String())
		updatedBaseURL.Scheme = rootResponse.Request.URL.Scheme
		updatedBaseURL.Host = rootResponse.Request.URL.Host
		log.Warnf("%s redirected to %s. The URL %s will be used for all API requests.", rootRequest.URL, rootResponse.Request.URL, updatedBaseURL)
		updatedUploadsURL, _ := url.Parse(client.UploadURL.String())
		updatedUploadsURL.Scheme = rootResponse.Request.URL.Scheme
		updatedUploadsURL.Host = rootResponse.Request.URL.Host
		client, err = github.NewEnterpriseClient(updatedBaseURL.String(), updatedUploadsURL.String(), tokenClient)
		if err != nil {
			return errors.Wrap(err, "Error creating GitHub Enterprise client.")
		}
	}
	aegis := rootResponse.Header.Get(enterpriseVersionHeaderKey) == enterpriseAegisVersionHeaderValue

	destinationRepositorySplit := strings.Split(destinationRepository, "/")
	destinationRepositoryOwner := destinationRepositorySplit[0]
	destinationRepositoryName := destinationRepositorySplit[1]

	pushService := pushService{
		ctx:                        ctx,
		cacheDirectory:             cacheDirectory,
		githubEnterpriseClient:     client,
		destinationRepositoryOwner: destinationRepositoryOwner,
		destinationRepositoryName:  destinationRepositoryName,
		destinationToken:           &token,
		actionsAdminUser:           actionsAdminUser,
		aegis:                      aegis,
		force:                      force,
		pushSSH:                    pushSSH,
		gitURL:                     gitURL,
	}

	repository, err := pushService.createRepository()
	if err != nil {
		return err
	}

	// "He was going to live forever, or die in the attempt." - Catch-22, Joseph Heller
	// We can't push the releases first because you can't create tags in an empty Git repository.
	// We can't push the Git content first because then we'd have Git content that references releases that don't exist yet.
	// In this compromise solution we push only the tags that are referenced by releases, we then push the releases, and then finally we push the rest of the Git content.
	// This should work so long as no one uses a tag both to reference a specific version of the CodeQL Action and as a storage mechanism for a CodeQL bundle.
	err = pushService.pushGit(repository, true)
	if err != nil {
		return err
	}
	err = pushService.pushReleases()
	if err != nil {
		return err
	}
	err = pushService.pushGit(repository, false)
	if err != nil {
		return err
	}
	log.Infof("Finished pushing CodeQL Action to %s!", destinationRepository)
	return nil
}
