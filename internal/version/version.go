// Package version provides version information for cc-relay.
package version

var (
	// Version is the semantic version (injected at build time via ldflags).
	Version = "dev"
	// Commit is the git commit hash (injected at build time via ldflags).
	Commit = "none"
	// BuildDate is the build timestamp (injected at build time via ldflags).
	BuildDate = "unknown"
)

// String returns formatted version information.
func String() string {
	return Version + " (commit: " + Commit + ", built: " + BuildDate + ")"
}
