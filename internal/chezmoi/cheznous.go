package chezmoi

import (
	"strings"

	"github.com/coreos/go-semver/semver"
)

// cheznousPreReleasePrefix marks a cheznous release, which contains the chezmoi
// release with the same major, minor and patch version.
const cheznousPreReleasePrefix = "cheznous."

// SatisfiesMinimumVersion returns whether version is at least minimumVersion,
// counting a cheznous release as the chezmoi release it is built on.
func SatisfiesMinimumVersion(version, minimumVersion semver.Version) bool {
	if strings.HasPrefix(string(version.PreRelease), cheznousPreReleasePrefix) {
		version.PreRelease = ""
	}
	return !version.LessThan(minimumVersion)
}
