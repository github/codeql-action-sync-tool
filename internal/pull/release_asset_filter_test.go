package pull

import (
	"testing"

	"github.com/google/go-github/v32/github"
	"github.com/stretchr/testify/require"
)

func releaseAssets(names ...string) []*github.ReleaseAsset {
	assets := make([]*github.ReleaseAsset, len(names))
	for index, name := range names {
		assets[index] = &github.ReleaseAsset{Name: github.String(name)}
	}
	return assets
}

func releaseAssetNames(assets []*github.ReleaseAsset) []string {
	names := make([]string, len(assets))
	for index, asset := range assets {
		names[index] = asset.GetName()
	}
	return names
}

func TestNewReleaseAssetFilter(t *testing.T) {
	_, err := newReleaseAssetFilter([]string{"linux64"}, []string{"osx64"}, "")
	require.EqualError(t, err, "--include-platforms and --exclude-platforms cannot be used together")

	_, err = newReleaseAssetFilter([]string{"plan9"}, nil, "")
	require.EqualError(t, err, `invalid platform "plan9": expected one of linux64, linux-arm64, osx64, win64`)

	_, err = newReleaseAssetFilter(nil, nil, "zip")
	require.EqualError(t, err, `invalid bundle archive format "zip": expected tar.gz or tar.zst`)
}

func TestReleaseAssetFilterDefaultsToAllAssets(t *testing.T) {
	filter, err := newReleaseAssetFilter(nil, nil, "")
	require.NoError(t, err)
	assets := releaseAssets(
		"codeql-bundle.tar.gz",
		"codeql-bundle-linux64.tar.zst",
		"codeql-bundle-swift-osx64.tar.zst",
		"update-job-proxy-win64.tar.gz",
		"cli-version-2.27.2.txt",
	)

	selected, skipped, err := filter.selectAssets("release", assets)
	require.NoError(t, err)
	require.Equal(t, releaseAssetNames(assets), releaseAssetNames(selected))
	require.Empty(t, skipped)
}

func TestReleaseAssetFilterIncludesExactPlatforms(t *testing.T) {
	filter, err := newReleaseAssetFilter([]string{"linux-arm64"}, nil, "")
	require.NoError(t, err)
	assets := releaseAssets(
		"codeql-bundle.tar.gz",
		"codeql-bundle-linux64.tar.gz",
		"codeql-bundle-linux-arm64.tar.gz",
		"codeql-bundle-linux-arm64.tar.gz.checksum.txt",
		"update-job-proxy-linux-arm64.tar.gz",
		"codeql-bundle-swift-osx64.tar.zst",
		"cli-version-2.27.2.txt",
		"future-release-metadata.json",
	)

	selected, skipped, err := filter.selectAssets("release", assets)
	require.NoError(t, err)
	require.Equal(t, []string{
		"codeql-bundle-linux-arm64.tar.gz",
		"codeql-bundle-linux-arm64.tar.gz.checksum.txt",
		"update-job-proxy-linux-arm64.tar.gz",
		"cli-version-2.27.2.txt",
		"future-release-metadata.json",
	}, releaseAssetNames(selected))
	require.Len(t, skipped, 3)
}

func TestReleaseAssetFilterExcludesPlatforms(t *testing.T) {
	filter, err := newReleaseAssetFilter(nil, []string{"osx64"}, "")
	require.NoError(t, err)
	assets := releaseAssets(
		"codeql-bundle.tar.zst",
		"codeql-bundle-linux64.tar.zst",
		"codeql-bundle-osx64.tar.zst",
		"update-job-proxy-osx64.tar.gz",
		"codeql-bundle-win64.tar.zst",
	)

	selected, _, err := filter.selectAssets("release", assets)
	require.NoError(t, err)
	require.Equal(t, []string{
		"codeql-bundle-linux64.tar.zst",
		"codeql-bundle-win64.tar.zst",
	}, releaseAssetNames(selected))
}

func TestReleaseAssetFilterSelectsBundleFormat(t *testing.T) {
	filter, err := newReleaseAssetFilter(nil, nil, "tar.gz")
	require.NoError(t, err)
	assets := releaseAssets(
		"codeql-bundle.tar.gz",
		"codeql-bundle.tar.zst",
		"codeql-bundle-linux64.tar.gz",
		"codeql-bundle-linux64.tar.gz.checksum.txt",
		"codeql-bundle-linux64.tar.zst",
		"codeql-bundle-actions-linux64.tar.zst",
		"update-job-proxy-linux64.tar.gz",
		"cli-version-2.27.2.txt",
	)

	selected, _, err := filter.selectAssets("release", assets)
	require.NoError(t, err)
	require.Equal(t, []string{
		"codeql-bundle.tar.gz",
		"codeql-bundle-linux64.tar.gz",
		"codeql-bundle-linux64.tar.gz.checksum.txt",
		"update-job-proxy-linux64.tar.gz",
		"cli-version-2.27.2.txt",
	}, releaseAssetNames(selected))
}

func TestReleaseAssetFilterFailsWhenPrimaryFormatIsMissing(t *testing.T) {
	filter, err := newReleaseAssetFilter([]string{"win64"}, nil, "tar.zst")
	require.NoError(t, err)

	_, _, err = filter.selectAssets("codeql-bundle-v1.2.3", releaseAssets(
		"codeql-bundle-win64.tar.gz",
		"codeql-bundle-actions-linux64.tar.zst",
	))
	require.EqualError(t, err, "release codeql-bundle-v1.2.3 does not publish required asset codeql-bundle-win64.tar.zst")
}
