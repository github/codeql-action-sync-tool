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

type pushService struct {
	ctx                        context.Context
	cacheDirectory             cachedirectory.CacheDirectory
	githubEnterpriseClient     *github.Client
	destinationRepositoryName  string
	destinationRepositoryOwner string
	destinationToken           string
	force                      bool
}

func (pushService *pushService) createRepository() (*github.Repository, error) {
	log.Debug("Ensuring repository exists...")
	user, response, err := pushService.githubEnterpriseClient.Users.Get(pushService.ctx, "")
	if err != nil {
		if response != nil && response.StatusCode == http.StatusUnauthorized {
			return nil, usererrors.New(errorInvalidDestinationToken)
		}
		return nil, errors.Wrap(err, "Error getting current user.")
	}

	// When creating a repository we can either create it in a named organization or under the current user (represented in go-github by an empty string).
	destinationOrganization := ""
	if pushService.destinationRepositoryOwner != user.GetLogin() {
		destinationOrganization = pushService.destinationRepositoryOwner
	}

	if destinationOrganization != "" {
		_, response, err := pushService.githubEnterpriseClient.Organizations.Get(pushService.ctx, pushService.destinationRepositoryOwner)
		if err != nil && (response == nil || response.StatusCode != http.StatusNotFound) {
			return nil, errors.Wrap(err, "Error checking if destination organization exists.")
		}
		if response != nil && response.StatusCode == http.StatusNotFound {
			log.Debugf("The organization %s does not exist. Creating it...", pushService.destinationRepositoryOwner)
			_, _, err := pushService.githubEnterpriseClient.Admin.CreateOrg(pushService.ctx, &github.Organization{
				Login: github.String(pushService.destinationRepositoryOwner),
				Name:  github.String(pushService.destinationRepositoryOwner),
			}, user.GetLogin())
			if err != nil {
				if response != nil && response.StatusCode == http.StatusNotFound && githubapiutil.MissingAllScopes(response, "site_admin") {
					return nil, usererrors.New("The destination token you have provided does not have the `site_admin` scope, so the destination organization cannot be created.")
				}
				return nil, errors.Wrap(err, "Error creating organization.")
			}
		}
	}

	repository, response, err := pushService.githubEnterpriseClient.Repositories.Get(pushService.ctx, pushService.destinationRepositoryOwner, pushService.destinationRepositoryName)
	if err != nil && (response == nil || response.StatusCode != http.StatusNotFound) {
		return nil, errors.Wrap(err, "Error checking if destination repository exists.")
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
		Private:      github.Bool(false),
	}
	if response.StatusCode == http.StatusNotFound {
		repository, response, err = pushService.githubEnterpriseClient.Repositories.Create(pushService.ctx, destinationOrganization, &desiredRepositoryProperties)
		if err != nil {
			if response.StatusCode == http.StatusNotFound && githubapiutil.MissingAllScopes(response, "public_repo", "repo") {
				return nil, usererrors.New("The destination token you have provided does not have the `public_repo` scope.")
			}
			return nil, errors.Wrap(err, "Error creating destination repository.")
		}
	} else {
		repository, response, err = pushService.githubEnterpriseClient.Repositories.Edit(pushService.ctx, pushService.destinationRepositoryOwner, pushService.destinationRepositoryName, &desiredRepositoryProperties)
		if err != nil {
			if response.StatusCode == http.StatusNotFound && githubapiutil.MissingAllScopes(response, "public_repo", "repo") {
				return nil, usererrors.New("The destination token you have provided does not have the `public_repo` scope.")
			}
			return nil, errors.Wrap(err, "Error updating destination repository.")
		}
	}

	return repository, nil
}

func (pushService *pushService) pushGit(repository *github.Repository, initialPush bool) error {
	remoteURL := repository.GetCloneURL()
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
		Password: pushService.destinationToken,
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

	if initialPush {
		releasePathStats, err := ioutil.ReadDir(pushService.cacheDirectory.ReleasesPath())
		if err != nil {
			return errors.Wrap(err, "Error reading releases.")
		}
		initialRefSpecs := []config.RefSpec{}
		for _, releasePathStat := range releasePathStats {
			initialRefSpecs = append(initialRefSpecs, config.RefSpec("+refs/tags/"+releasePathStat.Name()+":refs/tags/"+releasePathStat.Name()))
		}
		refSpecBatches = append(refSpecBatches, initialRefSpecs)
	} else {
		// We've got to push `main` on its own, so that it will be made the default branch if the repository has just been created. We then push everything else afterwards.
		refSpecBatches = append(refSpecBatches,
			[]config.RefSpec{
				config.RefSpec("+refs/heads/main:refs/heads/main"),
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
		return nil, errors.Wrap(err, "Error checking for existing CodeQL release.")
	}
	if release == nil {
		log.Debugf("Creating release %s...", releaseMetadata.GetTagName())
		release, _, err := pushService.githubEnterpriseClient.Repositories.CreateRelease(pushService.ctx, pushService.destinationRepositoryOwner, pushService.destinationRepositoryName, &releaseMetadata)
		if err != nil {
			return nil, errors.Wrap(err, "Error creating release.")
		}
		return release, nil
	}
	release, _, err = pushService.githubEnterpriseClient.Repositories.EditRelease(pushService.ctx, pushService.destinationRepositoryOwner, pushService.destinationRepositoryName, release.GetID(), &releaseMetadata)
	if err != nil {
		log.Debugf("Updating release %s...", releaseMetadata.GetTagName())
		return nil, errors.Wrap(err, "Error updating release.")
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
		return nil, response, errors.Wrap(err, "Error uploading release asset.")
	}
	return asset, response, nil
}

func (pushService *pushService) createOrUpdateReleaseAsset(release *github.RepositoryRelease, existingAssets []*github.ReleaseAsset, assetPathStat os.FileInfo) error {
	for _, existingAsset := range existingAssets {
		if existingAsset.GetName() == assetPathStat.Name() {
			if int64(existingAsset.GetSize()) == assetPathStat.Size() {
				return nil
			}
		}
	}
	log.Debugf("Uploading release asset %s...", assetPathStat.Name())
	assetFile, err := os.Open(pushService.cacheDirectory.AssetPath(release.GetTagName(), assetPathStat.Name()))
	defer assetFile.Close()
	progressReader := &ioprogress.Reader{
		Reader:   assetFile,
		Size:     assetPathStat.Size(),
		DrawFunc: ioprogress.DrawTerminalf(os.Stderr, ioprogress.DrawTextFormatBytes),
	}
	if err != nil {
		return errors.Wrap(err, "Error opening release asset.")
	}
	_, _, err = pushService.uploadReleaseAsset(release, assetPathStat, progressReader)
	if err != nil {
		return errors.Wrap(err, "Error uploading release asset.")
	}
	return nil
}

func (pushService *pushService) pushReleases() error {
	log.Debugf("Pushing CodeQL bundles...")
	releasesPath := pushService.cacheDirectory.ReleasesPath()

	releasePathStats, err := ioutil.ReadDir(releasesPath)
	if err != nil {
		return errors.Wrap(err, "Error reading releases.")
	}
	for _, releasePathStat := range releasePathStats {
		releaseName := releasePathStat.Name()
		release, err := pushService.createOrUpdateRelease(releaseName)
		if err != nil {
			return err
		}

		existingAssets := []*github.ReleaseAsset{}
		for page := 1; ; page++ {
			assets, _, err := pushService.githubEnterpriseClient.Repositories.ListReleaseAssets(pushService.ctx, pushService.destinationRepositoryOwner, pushService.destinationRepositoryName, release.GetID(), &github.ListOptions{Page: page})
			if err != nil {
				return errors.Wrap(err, "Error fetching existing release assets.")
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
				return errors.Wrap(err, "Error uploading release assets.")
			}
		}
	}

	return nil
}

func Push(ctx context.Context, cacheDirectory cachedirectory.CacheDirectory, destinationURL string, destinationToken string, destinationRepository string, force bool) error {
	err := cacheDirectory.CheckOrCreateVersionFile(false, version.Version())
	if err != nil {
		return err
	}
	err = cacheDirectory.CheckLock()
	if err != nil {
		return err
	}

	destinationURL = strings.TrimRight(destinationURL, "/")
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: destinationToken},
	)
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	client, err := github.NewEnterpriseClient(destinationURL+"/api/v3", destinationURL+"/api/uploads", tokenClient)
	if err != nil {
		return errors.Wrap(err, "Error creating GitHub Enterprise client.")
	}

	destinationRepositorySplit := strings.Split(destinationRepository, "/")
	destinationRepositoryOwner := destinationRepositorySplit[0]
	destinationRepositoryName := destinationRepositorySplit[1]

	pushService := pushService{
		ctx:                        ctx,
		cacheDirectory:             cacheDirectory,
		githubEnterpriseClient:     client,
		destinationRepositoryOwner: destinationRepositoryOwner,
		destinationRepositoryName:  destinationRepositoryName,
		destinationToken:           destinationToken,
		force:                      force,
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
