package pull

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-github/v32/github"
	log "github.com/sirupsen/logrus"
)

var releaseAssetPlatforms = []string{"linux64", "linux-arm64", "osx64", "win64"}

type releaseAssetFilter struct {
	includePlatforms    map[string]bool
	excludePlatforms    map[string]bool
	bundleArchiveFormat string
}

type releaseAsset struct {
	name                string
	platform            string
	format              string
	bundle              bool
	primary             bool
	combined            bool
	unclassifiedArchive bool
}

type skippedReleaseAsset struct {
	name   string
	reason string
}

func newReleaseAssetFilter(includePlatforms []string, excludePlatforms []string, bundleArchiveFormat string) (releaseAssetFilter, error) {
	if len(includePlatforms) > 0 && len(excludePlatforms) > 0 {
		return releaseAssetFilter{}, fmt.Errorf("--include-platforms and --exclude-platforms cannot be used together")
	}
	if bundleArchiveFormat != "" && bundleArchiveFormat != "tar.gz" && bundleArchiveFormat != "tar.zst" {
		return releaseAssetFilter{}, fmt.Errorf("invalid bundle archive format %q: expected tar.gz or tar.zst", bundleArchiveFormat)
	}

	include, err := platformSet(includePlatforms)
	if err != nil {
		return releaseAssetFilter{}, err
	}
	exclude, err := platformSet(excludePlatforms)
	if err != nil {
		return releaseAssetFilter{}, err
	}
	return releaseAssetFilter{
		includePlatforms:    include,
		excludePlatforms:    exclude,
		bundleArchiveFormat: bundleArchiveFormat,
	}, nil
}

func platformSet(platforms []string) (map[string]bool, error) {
	result := map[string]bool{}
	for _, platform := range platforms {
		if !isReleaseAssetPlatform(platform) {
			return nil, fmt.Errorf("invalid platform %q: expected one of %s", platform, strings.Join(releaseAssetPlatforms, ", "))
		}
		result[platform] = true
	}
	return result, nil
}

func isReleaseAssetPlatform(platform string) bool {
	for _, candidate := range releaseAssetPlatforms {
		if platform == candidate {
			return true
		}
	}
	return false
}

func classifyReleaseAsset(name string) releaseAsset {
	result := releaseAsset{name: name}
	archiveName := name
	if strings.HasSuffix(archiveName, ".checksum.txt") {
		archiveName = strings.TrimSuffix(archiveName, ".checksum.txt")
	}
	switch {
	case strings.HasSuffix(archiveName, ".tar.gz"):
		result.format = "tar.gz"
		archiveName = strings.TrimSuffix(archiveName, ".tar.gz")
	case strings.HasSuffix(archiveName, ".tar.zst"):
		result.format = "tar.zst"
		archiveName = strings.TrimSuffix(archiveName, ".tar.zst")
	default:
		return result
	}

	result.bundle = strings.HasPrefix(archiveName, "codeql-bundle")
	result.combined = archiveName == "codeql-bundle"
	for _, platform := range releaseAssetPlatforms {
		if strings.HasSuffix(archiveName, "-"+platform) {
			result.platform = platform
			result.primary = archiveName == "codeql-bundle-"+platform
			return result
		}
	}
	result.unclassifiedArchive = !result.combined
	return result
}

func (filter releaseAssetFilter) filtersPlatforms() bool {
	return len(filter.includePlatforms) > 0 || len(filter.excludePlatforms) > 0
}

func (filter releaseAssetFilter) includesPlatform(platform string) bool {
	if len(filter.includePlatforms) > 0 {
		return filter.includePlatforms[platform]
	}
	return !filter.excludePlatforms[platform]
}

func (filter releaseAssetFilter) selectAssets(releaseTag string, assets []*github.ReleaseAsset) ([]*github.ReleaseAsset, []skippedReleaseAsset, error) {
	classified := make([]releaseAsset, len(assets))
	primaryFormats := map[string]map[string]bool{}
	for index, asset := range assets {
		classified[index] = classifyReleaseAsset(asset.GetName())
		item := classified[index]
		if item.bundle && (item.primary || item.combined) && !strings.HasSuffix(item.name, ".checksum.txt") {
			if primaryFormats[item.nameWithoutFormat()] == nil {
				primaryFormats[item.nameWithoutFormat()] = map[string]bool{}
			}
			primaryFormats[item.nameWithoutFormat()][item.format] = true
		}
	}

	if filter.bundleArchiveFormat != "" {
		for name, formats := range primaryFormats {
			item := classifyReleaseAsset(name + "." + firstFormat(formats))
			if item.combined && filter.filtersPlatforms() {
				continue
			}
			if item.platform != "" && !filter.includesPlatform(item.platform) {
				continue
			}
			if !formats[filter.bundleArchiveFormat] {
				return nil, nil, fmt.Errorf("release %s does not publish required asset %s.%s", releaseTag, name, filter.bundleArchiveFormat)
			}
		}
	}

	selected := []*github.ReleaseAsset{}
	skipped := []skippedReleaseAsset{}
	for index, asset := range assets {
		item := classified[index]
		reason := ""
		switch {
		case item.combined && filter.filtersPlatforms():
			reason = "combined bundle omitted by platform filter"
		case item.platform != "" && !filter.includesPlatform(item.platform):
			reason = "platform filtered"
		case item.bundle && item.format != "" && filter.bundleArchiveFormat != "" && item.format != filter.bundleArchiveFormat:
			reason = "bundle archive format filtered"
		}
		if reason == "" {
			if item.unclassifiedArchive {
				log.Debugf("Keeping unclassified release archive %s.", asset.GetName())
			}
			selected = append(selected, asset)
		} else {
			skipped = append(skipped, skippedReleaseAsset{name: asset.GetName(), reason: reason})
		}
	}
	return selected, skipped, nil
}

func (asset releaseAsset) nameWithoutFormat() string {
	name := strings.TrimSuffix(asset.name, ".checksum.txt")
	return strings.TrimSuffix(name, "."+asset.format)
}

func firstFormat(formats map[string]bool) string {
	for format := range formats {
		return format
	}
	return ""
}

func logAssetSelection(releaseTag string, total int, selected int, skipped []skippedReleaseAsset) {
	if len(skipped) == 0 {
		return
	}
	reasonCounts := map[string]int{}
	for _, asset := range skipped {
		reasonCounts[asset.reason]++
		log.Debugf("Skipping release asset %s: %s.", asset.name, asset.reason)
	}
	reasons := make([]string, 0, len(reasonCounts))
	for reason, count := range reasonCounts {
		reasons = append(reasons, fmt.Sprintf("%s=%d", reason, count))
	}
	sort.Strings(reasons)
	log.Infof("Selected %d of %d assets for release %s; skipped %d (%s).", selected, total, releaseTag, len(skipped), strings.Join(reasons, ", "))
}
