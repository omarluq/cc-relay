package version_test

import (
	"testing"

	"github.com/omarluq/cc-relay/internal/version"
)

func TestVersion(t *testing.T) {
	t.Parallel()

	// Version should always be non-empty.
	if version.Version == "" {
		t.Error("Version is empty")
	}
}

func TestCommit(t *testing.T) {
	t.Parallel()

	// Commit should always be non-empty.
	if version.Commit == "" {
		t.Error("Commit is empty")
	}
}

func TestBuildDate(t *testing.T) {
	t.Parallel()

	// BuildDate should always be non-empty.
	if version.BuildDate == "" {
		t.Error("BuildDate is empty")
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	origVersion := version.Version
	origCommit := version.Commit
	t.Cleanup(func() {
		version.Version = origVersion
		version.Commit = origCommit
	})

	version.Version = "v0.0.11-20-ga961617-dirty"
	version.Commit = "a961617"

	got := version.String()
	want := "v0.0.11-a961617-20"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
