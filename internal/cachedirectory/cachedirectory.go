package cachedirectory

import (
	usererrors "errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
)

const errorCacheWrongVersion = "The cache you are trying to push was created with an old version of the CodeQL Action Sync tool. Please re-pull it with this version of the tool."
const errorNotACacheOrEmpty = "The cache directory you have selected is not empty, but was not created by the CodeQL Action Sync tool. If you are sure you want to use this directory, please delete it and run the sync tool again."
const errorCacheParentDoesNotExist = "Cannot create cache directory because its parent, does not exist."
const errorPushNonCache = "The cache directory you have provided does not appear to be valid. Please check it exists and that you have run the `pull` command to populate it."
const errorCacheLocked = "The cache directory is locked, likely due to a `pull` command being interrupted. Please run `pull` again to ensure all required data is downloaded."

type CacheDirectory struct {
	path string
}

func NewCacheDirectory(path string) CacheDirectory {
	return CacheDirectory{
		path: filepath.Clean(path),
	}
}

func isAccessibleDirectory(path string) (bool, error) {
	_, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "Could not access directory %s.", path)
	}
	
	return true, nil
}

func isEmptyDirectory(path string) (bool, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return false, errors.Wrapf(err, "Could not read contents of directory %s.", path)
	}

	return len(files) == 0, nil
}

func existsDirectory(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrapf(err, "Could not access directory %s.", path)
	}
	return true, nil
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
				return usererrors.New(errorCacheParentDoesNotExist)
			}
			return errors.Wrap(err, "Could not access parent path of cache directory.")
		}

		if cacheVersionFileExists {
			err := os.RemoveAll(cacheDirectory.path)
			if err != nil {
				return errors.Wrap(err, "Error removing outdated cache directory.")
			}
		}

		existsDirectory, err := existsDirectory(cacheDirectory.path)
		if err != nil {
			return err
		}
		if !existsDirectory {
			err := os.Mkdir(cacheDirectory.path, 0755)
			if err != nil {
				return errors.Wrap(err, "Could not create cache directory.")
			}

		} else {
			isAccessible, err := isAccessibleDirectory(cacheDirectory.path)
			if err != nil {
				return err
			}
			if !isAccessible {
				return errors.Wrap(err, "Cache dir exists, but the current user can't write to it.")
			}
		}
		isEmpty, err := isEmptyDirectory(cacheDirectory.path)
		if err != nil {
			return err
		}
		if isEmpty {
			err = ioutil.WriteFile(cacheVersionFilePath, []byte(version), 0644)
			if err != nil {
				return errors.Wrap(err, "Could not create cache version file.")
			}
			return nil
		}
		return usererrors.New(errorNotACacheOrEmpty)
	}

	if cacheVersionFileExists {
		return usererrors.New(errorCacheWrongVersion)
	}
	return usererrors.New(errorPushNonCache)
}

func (cacheDirectory *CacheDirectory) Lock() error {
	file, err := os.Create(cacheDirectory.lockFilePath())
	if err != nil {
		return errors.Wrap(err, "Error locking cache directory.")
	}
	defer file.Close()
	// If the cache directory is already locked, it's not really a huge issue since the purpose of the lock is mostly to check whether a `pull` operation was interrupted before pushing.
	return nil
}

func (cacheDirectory *CacheDirectory) Unlock() error {
	err := os.Remove(cacheDirectory.lockFilePath())
	if err != nil {
		return errors.Wrap(err, "Error unlocking cache directory.")
	}
	return nil
}

func (cacheDirectory *CacheDirectory) CheckLock() error {
	_, err := os.Stat(cacheDirectory.lockFilePath())
	if err == nil {
		return usererrors.New(errorCacheLocked)
	}
	if os.IsNotExist(err) {
		return nil
	}
	return errors.Wrap(err, "Error checking if cache directory is locked.")
}

func (cacheDirectory *CacheDirectory) versionFilePath() string {
	return path.Join(cacheDirectory.path, ".codeql-actions-sync-version")
}

func (cacheDirectory *CacheDirectory) lockFilePath() string {
	return path.Join(cacheDirectory.path, ".codeql-actions-sync-lock")
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
