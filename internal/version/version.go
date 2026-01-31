// Package version provides version information for cc-relay.
package version

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
)

var (
	// Version is the semantic version (injected at build time via ldflags).
	Version = "dev"
	// Commit is the git commit hash (injected at build time via ldflags).
	Commit = "none"
	// BuildDate is the build timestamp (injected at build time via ldflags).
	BuildDate = "unknown"
)

// init populates package version metadata (Version, Commit, BuildDate) from
// runtime build information when those values were not provided at build time.
func init() {
	applyBuildInfoFallback()
}

// String returns formatted version information.
func String() string {
	return formatDisplayVersion(Version, Commit)
}

// applyBuildInfoFallback populates package version metadata from runtime build information when available.
// 
// If build information can be read, it updates Version, Commit, and BuildDate with values derived from the
// binary's build metadata. If no build information is available, it leaves the existing values unchanged.
func applyBuildInfoFallback() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	applyMainVersionFallback(info)
	applySettingsFallback(info)
}

// applyMainVersionFallback sets the package-level Version from the provided
// build info when the current Version is the default ("dev") or empty.
//
// If info.Main.Version is empty or equals "(devel)", the function leaves
// Version unchanged.
func applyMainVersionFallback(info *debug.BuildInfo) {
	if Version != "dev" && Version != "" {
		return
	}
	if info.Main.Version == "" || info.Main.Version == "(devel)" {
		return
	}
	Version = info.Main.Version
}

// applySettingsFallback updates package-level Commit and BuildDate from the
// provided build info settings when those variables are not already set.
// It looks for settings with keys "vcs.revision" and "vcs.time" and assigns
// their values to Commit and BuildDate respectively only if the current
// values are the defaults ("none"/"unknown") or empty.
func applySettingsFallback(info *debug.BuildInfo) {
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if Commit == "none" || Commit == "" {
				Commit = setting.Value
			}
		case "vcs.time":
			if BuildDate == "unknown" || BuildDate == "" {
				BuildDate = setting.Value
			}
		}
	}
}

var describePattern = regexp.MustCompile(`^(?P<base>.+?)(?:-(?P<count>\d+)-g(?P<sha>[0-9a-f]+))?(?:-dirty)?$`)

// formatDisplayVersion formats a user-facing version string from the given
// version and commit information.
//
// It uses the parsed describe-style parts of version and the commit fallback to
// produce either the base version (defaults to "dev" if empty) or a
// "base-sha-count" string when commit/count information is present. If a short
// commit SHA cannot be determined, the base version is returned.
func formatDisplayVersion(version, commit string) string {
	base, count, sha, dirty := parseDescribe(version)
	if base == "" {
		base = "dev"
	}
	if !dirty && count == "" {
		return base
	}
	if sha == "" {
		sha = shortCommit(commit)
	}
	if sha == "" || sha == "none" {
		return base
	}
	if count == "" {
		count = "0"
	}
	return fmt.Sprintf("%s-%s-%s", base, sha, count)
}

// parseDescribe parses a version string into its components: base, count, sha, and a dirty flag.
// If version is empty it returns empty strings and false. If the string does not match the expected
// describePattern it returns the entire input as base, empty count and sha, and sets dirty to true
// when the version ends with the "-dirty" suffix.
func parseDescribe(version string) (base, count, sha string, dirty bool) {
	if version == "" {
		return "", "", "", false
	}
	match := describePattern.FindStringSubmatch(version)
	if match == nil {
		return version, "", "", strings.HasSuffix(version, "-dirty")
	}
	base = match[describePattern.SubexpIndex("base")]
	count = match[describePattern.SubexpIndex("count")]
	sha = match[describePattern.SubexpIndex("sha")]
	dirty = strings.HasSuffix(version, "-dirty")
	return base, count, sha, dirty
}

// shortCommit returns the first seven characters of commit if commit is longer than seven characters; otherwise it returns commit unchanged.
func shortCommit(commit string) string {
	if len(commit) <= 7 {
		return commit
	}
	return commit[:7]
}