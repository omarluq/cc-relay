package vinfo

import "runtime/debug"

// FormatDisplayVersion exports formatDisplayVersion for testing.
var FormatDisplayVersion = formatDisplayVersion

// ShortCommit exports shortCommit for testing.
var ShortCommit = shortCommit

// ParseDescribe exports parseDescribe for testing.
var ParseDescribe = parseDescribe

// ApplySettingsFallback exports applySettingsFallback for testing.
func ApplySettingsFallback(info *debug.BuildInfo) {
	applySettingsFallback(info)
}

// ApplyMainVersionFallback exports applyMainVersionFallback for testing.
func ApplyMainVersionFallback(info *debug.BuildInfo) {
	applyMainVersionFallback(info)
}

// SetVersionVars lets tests mutate the package-level version vars and get a
// restore function to reset them after the test. Tests that touch these globals
// MUST avoid t.Parallel() since the variables are package-scoped.
func SetVersionVars(version, commit, buildDate string) (restore func()) {
	prevVersion, prevCommit, prevBuildDate := Version, Commit, BuildDate
	Version, Commit, BuildDate = version, commit, buildDate
	return func() {
		Version, Commit, BuildDate = prevVersion, prevCommit, prevBuildDate
	}
}
