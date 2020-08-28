package pull

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/github/codeql-action-sync/internal/actionconfiguration"
	"github.com/mitchellh/ioprogress"
	"golang.org/x/oauth2"

	"github.com/github/codeql-action-sync/internal/cachedirectory"
	"github.com/github/codeql-action-sync/internal/version"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v32/github"
	"github.com/pkg/errors"
)

const sourceOwner = "github"
const sourceRepository = "codeql-action"
const sourceURL = "https://github.com/" + sourceOwner + "/" + sourceRepository + ".git"

var relevantReferences = regexp.MustCompile("^refs/(heads|tags)/(main|v\\d+)$")

const defaultConfigurationPath = "src/defaults.json"

type pullService struct {
	ctx                context.Context
	cacheDirectory     cachedirectory.CacheDirectory
	gitCloneURL        string
	githubDotComClient *github.Client
	sourceToken        string
}

func (pullService *pullService) pullGit(fresh bool) error {
	if fresh {
		log.Debug("Pulling Git contents fresh...")
	} else {
		log.Debug("Updating Git contents...")
	}
	gitPath := pullService.cacheDirectory.GitPath()

	var localRepository *git.Repository
	if fresh {
		err := os.RemoveAll(gitPath)
		if err != nil {
			return errors.Wrap(err, "Error removing existing Git repository cache.")
		}
		localRepository, err = git.PlainInit(gitPath, true)
		if err != nil {
			return errors.Wrap(err, "Error initializing Git repository cache.")
		}
	} else {
		var err error
		localRepository, err = git.PlainOpen(gitPath)
		if err != nil {
			return errors.Wrap(err, "Error opening Git repository cache.")
		}
	}

	err := localRepository.DeleteRemote(git.DefaultRemoteName)
	if err != nil && err != git.ErrRemoteNotFound {
		return errors.Wrap(err, "Error removing existing Git remote.")
	}

	remote := git.NewRemote(localRepository.Storer, &config.RemoteConfig{
		Name: git.DefaultRemoteName,
		URLs: []string{pullService.gitCloneURL},
	})

	var credentials *githttp.BasicAuth
	if pullService.sourceToken != "" {
		credentials = &githttp.BasicAuth{
			Username: "x-access-token",
			Password: pullService.sourceToken,
		}
	}

	remoteReferences, err := remote.List(&git.ListOptions{Auth: credentials})
	if err != nil {
		return errors.Wrap(err, "Error listing remote references.")
	}
	localReferences, err := localRepository.References()
	if err != nil {
		return errors.Wrap(err, "Error listing local references.")
	}
	localReferences.ForEach(func(localReference *plumbing.Reference) error {
		if !strings.HasPrefix(localReference.Name().String(), "refs/") {
			return nil
		}
		for _, remoteReference := range remoteReferences {
			if remoteReference.Name().String() == localReference.Name().String() {
				return nil
			}
		}
		err := localRepository.Storer.RemoveReference(localReference.Name())
		if err != nil {
			return errors.Wrap(err, "Error pruning reference.")
		}
		return nil
	})

	err = remote.FetchContext(pullService.ctx, &git.FetchOptions{
		RemoteName: git.DefaultRemoteName,
		RefSpecs: []config.RefSpec{
			config.RefSpec("+refs/heads/*:refs/heads/*"),
			config.RefSpec("+refs/tags/*:refs/tags/*"),
		},
		Progress: os.Stderr,
		Tags:     git.NoTags,
		Force:    true,
		Auth:     credentials,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return errors.Wrap(err, "Error doing Git fetch.")
	}
	return nil
}

func (pullService *pullService) findRelevantReleases() ([]string, error) {
	log.Debug("Finding release references...")
	localRepository, err := git.PlainOpen(pullService.cacheDirectory.GitPath())
	if err != nil {
		return []string{}, errors.Wrap(err, "Error opening Git repository cache.")
	}
	references, err := localRepository.References()
	if err != nil {
		return []string{}, errors.Wrap(err, "Error reading references from Git repository cache.")
	}
	defer references.Close()
	releasesMap := map[string]bool{}
	releases := []string{}
	err = references.ForEach(func(reference *plumbing.Reference) error {
		if relevantReferences.MatchString(reference.Name().String()) {
			log.Debugf("Found %s.", reference.Name().String())
			commit, err := localRepository.CommitObject(reference.Hash())
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("Error loading commit %s for reference %s.", reference.Hash(), reference.Name().String()))
			}
			file, err := commit.File(defaultConfigurationPath)
			if err != nil {
				if err == object.ErrFileNotFound {
					log.Debugf("Ignoring reference %s as it does not have a default configuration.", reference.Name().String())
					return nil
				}
				return errors.Wrap(err, fmt.Sprintf("Error loading default configuration file from commit %s for reference %s.", reference.Hash(), reference.Name().String()))
			}
			content, err := file.Contents()
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("Error reading default configuration file content from commit %s for reference %s.", reference.Hash(), reference.Name().String()))
			}
			configuration, err := actionconfiguration.Parse(content)
			if err != nil {
				return err
			}
			if _, exists := releasesMap[configuration.BundleVersion]; !exists {
				releasesMap[configuration.BundleVersion] = true
				releases = append(releases, configuration.BundleVersion)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return releases, nil
}

func (pullService *pullService) pullReleases() error {
	log.Debug("Pulling CodeQL bundles...")
	relevantReleases, err := pullService.findRelevantReleases()
	if err != nil {
		return err
	}

	for index, releaseTag := range relevantReleases {
		log.Debugf("Pulling CodeQL bundle %s (%d/%d)...", releaseTag, index+1, len(relevantReleases))
		release, _, err := pullService.githubDotComClient.Repositories.GetReleaseByTag(pullService.ctx, sourceOwner, sourceRepository, releaseTag)
		if err != nil {
			return errors.Wrap(err, "Error loading CodeQL release information.")
		}
		err = os.MkdirAll(pullService.cacheDirectory.ReleasePath(releaseTag), 0755)
		if err != nil {
			return errors.Wrap(err, "Error creating releases directory.")
		}
		releaseMetadataPath := pullService.cacheDirectory.MetadataPath(releaseTag)
		releaseJSON, err := json.Marshal(release)
		if err != nil {
			return errors.Wrap(err, "Error converting release to JSON.")
		}
		err = ioutil.WriteFile(releaseMetadataPath, releaseJSON, 0644)
		if err != nil {
			return errors.Wrap(err, "Error writing release metadata.")
		}
		assetsPath := pullService.cacheDirectory.AssetsPath(releaseTag)
		err = os.MkdirAll(assetsPath, 0755)
		if err != nil {
			return errors.Wrap(err, "Error creating assets directory.")
		}
		for _, asset := range release.Assets {
			log.Debugf("Downloading asset %s...", asset.GetName())
			downloadPath := pullService.cacheDirectory.AssetPath(releaseTag, asset.GetName())
			downloadPathStat, err := os.Stat(downloadPath)
			if err == nil && downloadPathStat.Size() == int64(asset.GetSize()) {
				log.Debug("Asset is already in cache.")
				continue
			}
			err = os.RemoveAll(downloadPath)
			if err != nil {
				return errors.Wrap(err, "Error removing existing cached asset.")
			}
			reader, redirectURL, err := pullService.githubDotComClient.Repositories.DownloadReleaseAsset(pullService.ctx, sourceOwner, sourceRepository, asset.GetID(), http.DefaultClient)
			if err != nil {
				return errors.Wrap(err, "Error downloading asset.")
			}
			if reader == nil {
				response, err := http.Get(redirectURL)
				if err != nil {
					return errors.Wrap(err, "Error downloading asset.")
				}
				if response.StatusCode >= 300 {
					return errors.Wrapf(err, "Status code %d while downloading asset.", response.StatusCode)
				}
				reader = response.Body
			}
			defer reader.Close()
			downloadFile, err := os.Create(downloadPath)
			if err != nil {
				return errors.Wrap(err, "Error creating cached asset file.")
			}
			defer downloadFile.Close()
			progressReader := &ioprogress.Reader{
				Reader:   reader,
				Size:     int64(asset.GetSize()),
				DrawFunc: ioprogress.DrawTerminalf(os.Stderr, ioprogress.DrawTextFormatBytes),
			}
			_, err = io.Copy(downloadFile, progressReader)
			if err != nil {
				return errors.Wrap(err, "Error downloading asset.")
			}
		}
	}
	return nil
}

func Pull(ctx context.Context, cacheDirectory cachedirectory.CacheDirectory, sourceToken string) error {
	err := cacheDirectory.CheckOrCreateVersionFile(true, version.Version())
	if err != nil {
		return err
	}
	err = cacheDirectory.Lock()
	if err != nil {
		return err
	}

	var tokenClient *http.Client
	if sourceToken != "" {
		tokenSource := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: sourceToken},
		)
		tokenClient = oauth2.NewClient(ctx, tokenSource)
	}

	pullService := pullService{
		ctx:                ctx,
		cacheDirectory:     cacheDirectory,
		gitCloneURL:        sourceURL,
		githubDotComClient: github.NewClient(tokenClient),
		sourceToken:        sourceToken,
	}

	err = pullService.pullGit(false)
	if err != nil {
		// If an error occurred updating the existing copy then try cloning fresh instead. An error is expected if the local cache does not yet exist, but even if it is corrupt in some way we can safely delete it and start again.
		err := pullService.pullGit(true)
		if err != nil {
			return err
		}
	}
	err = pullService.pullReleases()
	if err != nil {
		return err
	}

	err = cacheDirectory.Unlock()
	if err != nil {
		return err
	}
	log.Info("Finished pulling the CodeQL Action repository and bundles!")
	return nil
}
