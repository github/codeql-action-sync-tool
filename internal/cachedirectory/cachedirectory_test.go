package cachedirectory

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/github/codeql-action-sync/test"
	"github.com/stretchr/testify/require"
)

const aVersion = "1.0.0"
const aDifferentVersion = "1.0.1"

func TestCreateCacheDirectoryDuringPullThenReuse(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	cacheDirectory := NewCacheDirectory(path.Join(temporaryDirectory, "cache"))
	err := cacheDirectory.CheckOrCreateVersionFile(true, aVersion)
	require.NoError(t, err)
	flagFile := path.Join(cacheDirectory.path, "flag")
	ioutil.WriteFile(flagFile, []byte("test"), 0644)
	err = cacheDirectory.CheckOrCreateVersionFile(true, aVersion)
	require.NoError(t, err)
	err = cacheDirectory.CheckOrCreateVersionFile(false, aVersion)
	require.NoError(t, err)
	require.FileExists(t, flagFile)
}

func TestOverwriteCacheDirectoryIfVersionMismatchDuringPull(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	cacheDirectory := NewCacheDirectory(path.Join(temporaryDirectory, "cache"))
	err := cacheDirectory.CheckOrCreateVersionFile(true, aVersion)
	require.NoError(t, err)
	flagFile := path.Join(cacheDirectory.path, "flag")
	ioutil.WriteFile(flagFile, []byte("test"), 0644)
	err = cacheDirectory.CheckOrCreateVersionFile(true, aDifferentVersion)
	require.NoError(t, err)
	require.NoFileExists(t, flagFile)
}

func TestErrorIfVersionMismatchDuringPush(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	cacheDirectory := NewCacheDirectory(path.Join(temporaryDirectory, "cache"))
	err := cacheDirectory.CheckOrCreateVersionFile(true, aVersion)
	require.NoError(t, err)
	err = cacheDirectory.CheckOrCreateVersionFile(false, aDifferentVersion)
	require.EqualError(t, err, errorCacheWrongVersion)
}

func TestErrorIfCacheIsNonEmptyAndNotCache(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	cacheDirectoryPath := path.Join(temporaryDirectory, "cache")
	err := os.MkdirAll(cacheDirectoryPath, 0755)
	require.NoError(t, err)
	flagFile := path.Join(cacheDirectoryPath, "flag")
	ioutil.WriteFile(flagFile, []byte("test"), 0644)
	cacheDirectory := NewCacheDirectory(cacheDirectoryPath)
	err = cacheDirectory.CheckOrCreateVersionFile(true, aVersion)
	require.EqualError(t, err, errorNotACacheOrEmpty)
	require.FileExists(t, flagFile)
}

func TestErrorIfCacheParentDoesNotExist(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	cacheDirectory := NewCacheDirectory(path.Join(temporaryDirectory, "non-existent-parent", "cache"))
	err := cacheDirectory.CheckOrCreateVersionFile(true, aVersion)
	require.EqualError(t, err, errorCacheParentDoesNotExist)
}

func TestErrorIfPushNonCache(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	cacheDirectory := NewCacheDirectory(temporaryDirectory)
	err := cacheDirectory.CheckOrCreateVersionFile(false, aVersion)
	require.EqualError(t, err, errorPushNonCache)
}

func TestErrorIfPushNonExistent(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	cacheDirectory := NewCacheDirectory(path.Join(temporaryDirectory, "cache"))
	err := cacheDirectory.CheckOrCreateVersionFile(false, aVersion)
	require.EqualError(t, err, errorPushNonCache)
}

func TestCreateCacheDirectoryWithTrailingSlash(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	cacheDirectory := NewCacheDirectory(path.Join(temporaryDirectory, "cache") + string(os.PathSeparator))
	err := cacheDirectory.CheckOrCreateVersionFile(true, aVersion)
	require.NoError(t, err)
}

func TestUseProvidedEmptyCacheDirectory(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	cacheDirectoryPath := path.Join(temporaryDirectory, "cache")
	err := os.MkdirAll(cacheDirectoryPath, 0755)
	require.NoError(t, err)
	cacheDirectory := NewCacheDirectory(cacheDirectoryPath)
	err = cacheDirectory.CheckOrCreateVersionFile(true, aVersion)
	require.NoError(t, err)
	cacheVersionFilePath := cacheDirectory.versionFilePath()
	require.FileExists(t, cacheVersionFilePath)
}

func TestLocking(t *testing.T) {
	temporaryDirectory := test.CreateTemporaryDirectory(t)
	cacheDirectory := NewCacheDirectory(path.Join(temporaryDirectory, "cache"))
	require.NoError(t, cacheDirectory.CheckOrCreateVersionFile(true, aVersion))
	require.NoError(t, cacheDirectory.Lock())
	require.NoError(t, cacheDirectory.Lock())
	require.EqualError(t, cacheDirectory.CheckLock(), errorCacheLocked)
	require.NoError(t, cacheDirectory.Unlock())
	require.NoError(t, cacheDirectory.CheckLock())
}
