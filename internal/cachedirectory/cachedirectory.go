package cachedirectory

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
)

const errorCacheWrongVersion = "The cache you are trying to push was created with an old version of the CodeQL Action Sync tool. Please re-pull it with this version of the tool."
const errorNotACacheOrEmpty = "The cache directory you have selected is not empty, but was not created by the CodeQL Action Sync tool. If you are sure you want to use this directory, please delete it and run the sync tool again."
const errorCacheParentDoesNotExist = "Cannot create cache directory because its parent, does not exist."
const errorPushNonCache = "The directory you have provided does not appear to be valid. Please check it exists and that you have run the `pull` command to populate it."

const CacheReferencePrefix = "refs/remotes/" + git.DefaultRemoteName + "/"

type CacheDirectory struct {
	path string
}

func NewCacheDirectory(path string) CacheDirectory {
	return CacheDirectory{
		path: filepath.Clean(path),
	}
}

func isEmptyOrNonExistentDirectory(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, errors.Wrap(err, fmt.Sprintf("Could not access directory %s.", path))
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err != nil {
		if err == io.EOF {
			return true, nil
		}
		return false, errors.Wrap(err, fmt.Sprintf("Could not read contents of directory %s.", path))
	}
	return false, nil
}

func (cacheDirectory *CacheDirectory) CheckOrCreateVersionFile(pull bool, version string) error {
	cacheVersionFilePath := cacheDirectory.versionFilePath()
	cacheVersionBytes, err := ioutil.ReadFile(cacheVersionFilePath)
	cacheVersionFileExists := !os.IsNotExist(err)
	if err != nil && cacheVersionFileExists {
		return errors.Wrap(err, "Could not read version file from cache directory.")
	}
	cacheVersion := string(cacheVersionBytes)

	if cacheVersion == version {
		return nil
	}

	if pull {
		cacheParentPath := filepath.Dir(cacheDirectory.path)
		_, err := os.Stat(cacheParentPath)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New(errorCacheParentDoesNotExist)
			}
			return errors.Wrap(err, "Could not access parent path of cache directory.")
		}

		if cacheVersionFileExists {
			err := os.RemoveAll(cacheDirectory.path)
			if err != nil {
				return errors.Wrap(err, "Error removing outdated cache directory.")
			}
		}

		isEmptyOrNonExistent, err := isEmptyOrNonExistentDirectory(cacheDirectory.path)
		if err != nil {
			return err
		}
		if isEmptyOrNonExistent {
			err := os.Mkdir(cacheDirectory.path, 0755)
			if err != nil {
				return errors.Wrap(err, "Could not create cache directory.")
			}
			err = ioutil.WriteFile(cacheVersionFilePath, []byte(version), 0644)
			if err != nil {
				return errors.Wrap(err, "Could not create cache version file.")
			}
			return nil
		}
		return errors.New(errorNotACacheOrEmpty)
	}

	if cacheVersionFileExists {
		return errors.New(errorCacheWrongVersion)
	}
	return errors.New(errorPushNonCache)
}

func (cacheDirectory *CacheDirectory) versionFilePath() string {
	return path.Join(cacheDirectory.path, ".codeql-actions-sync-version")
}

func (cacheDirectory *CacheDirectory) GitPath() string {
	return path.Join(cacheDirectory.path, "git")
}

func (cacheDirectory *CacheDirectory) ReleasesPath() string {
	return path.Join(cacheDirectory.path, "releases")
}

func (cacheDirectory *CacheDirectory) ReleasePath(release string) string {
	return path.Join(cacheDirectory.ReleasesPath(), release)
}

func (cacheDirectory *CacheDirectory) AssetsPath(release string) string {
	return path.Join(cacheDirectory.ReleasePath(release), "assets")
}

func (cacheDirectory *CacheDirectory) AssetPath(release string, assetName string) string {
	return path.Join(cacheDirectory.AssetsPath(release), assetName)
}

func (cacheDirectory *CacheDirectory) MetadataPath(release string) string {
	return path.Join(cacheDirectory.ReleasePath(release), "metadata.json")
}
