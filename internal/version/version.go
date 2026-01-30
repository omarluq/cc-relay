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

func init() {
	applyBuildInfoFallback()
}

// String returns formatted version information.
func String() string {
	return formatDisplayVersion(Version, Commit)
}

func applyBuildInfoFallback() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	applyMainVersionFallback(info)
	applySettingsFallback(info)
}

func applyMainVersionFallback(info *debug.BuildInfo) {
	if Version != "dev" && Version != "" {
		return
	}
	if info.Main.Version == "" || info.Main.Version == "(devel)" {
		return
	}
	Version = info.Main.Version
}

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

func shortCommit(commit string) string {
	if len(commit) <= 7 {
		return commit
	}
	return commit[:7]
}
