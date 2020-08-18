package pull

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/github/codeql-action-sync/internal/actionconfiguration"
	"github.com/mitchellh/ioprogress"

	"github.com/github/codeql-action-sync/internal/cachedirectory"
	"github.com/github/codeql-action-sync/internal/version"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
)

const sourceOwner = "github"
const sourceRepository = "codeql-action"
const sourceURL = "https://github.com/" + sourceOwner + "/" + sourceRepository + ".git"

var relevantReferences = regexp.MustCompile("^refs/remotes/" + git.DefaultRemoteName + "/(heads|tags)/(main|v\\d+)$")

const defaultConfigurationPath = "src/defaults.json"

type pullService struct {
	ctx                context.Context
	cacheDirectory     cachedirectory.CacheDirectory
	gitCloneURL        string
	githubDotComClient *github.Client
}

func (pullService *pullService) pullGit(fresh bool) error {
	if fresh {
		log.Print("Pulling Git contents fresh...")
	} else {
		log.Print("Updating Git contents...")
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

	_, err = localRepository.CreateRemote(&config.RemoteConfig{
		Name: git.DefaultRemoteName,
		URLs: []string{pullService.gitCloneURL},
	})
	if err != nil {
		return errors.Wrap(err, "Error setting Git remote.")
	}

	err = localRepository.FetchContext(pullService.ctx, &git.FetchOptions{
		RemoteName: git.DefaultRemoteName,
		RefSpecs: []config.RefSpec{
			config.RefSpec("+refs/heads/*:refs/remotes/" + git.DefaultRemoteName + "/heads/*"),
			config.RefSpec("+refs/tags/*:refs/remotes/" + git.DefaultRemoteName + "/tags/*"),
		},
		Progress: os.Stderr,
		Tags:     git.NoTags,
		Force:    true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return errors.Wrap(err, "Error doing Git fetch.")
	}
	return nil
}

func (pullService *pullService) findRelevantReleases() ([]string, error) {
	log.Print("Finding release references...")
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
			log.Printf("Found %s.", reference.Name().String())
			commit, err := localRepository.CommitObject(reference.Hash())
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("Error loading commit %s for reference %s.", reference.Hash(), reference.Name().String()))
			}
			file, err := commit.File(defaultConfigurationPath)
			if err != nil {
				if err == object.ErrFileNotFound {
					log.Printf("Ignoring reference %s as it does not have a default configuration.", reference.Name().String())
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
	log.Print("Pulling CodeQL bundles...")
	relevantReleases, err := pullService.findRelevantReleases()
	if err != nil {
		return err
	}

	for index, releaseTag := range relevantReleases {
		log.Printf("Pulling CodeQL bundle %s (%d/%d)...", releaseTag, index+1, len(relevantReleases))
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
			log.Printf("Downloading asset %s...", asset.GetName())
			downloadPath := pullService.cacheDirectory.AssetPath(releaseTag, asset.GetName())
			downloadPathStat, err := os.Stat(downloadPath)
			if err == nil && downloadPathStat.Size() == int64(asset.GetSize()) {
				log.Println("Asset is already in cache.")
				continue
			}
			err = os.RemoveAll(downloadPath)
			if err != nil {
				return errors.Wrap(err, "Error removing existing cached asset.")
			}
			reader, redirectURL, err := pullService.githubDotComClient.Repositories.DownloadReleaseAsset(pullService.ctx, sourceOwner, sourceRepository, asset.GetID())
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

func Pull(ctx context.Context, cacheDirectory cachedirectory.CacheDirectory) error {
	err := cacheDirectory.CheckOrCreateVersionFile(true, version.Version())
	if err != nil {
		return err
	}

	pullService := pullService{
		ctx:                ctx,
		cacheDirectory:     cacheDirectory,
		gitCloneURL:        sourceURL,
		githubDotComClient: github.NewClient(nil),
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
	log.Print("Finished pulling the CodeQL Action repository and bundles!")
	return nil
}
