package version_test

import (
	"testing"

	"github.com/omarluq/cc-relay/internal/version"
)

func TestVersion(t *testing.T) {
	t.Parallel()

	// Test that Version is set to default value
	if version.Version != "dev" {
		t.Errorf("Version = %q, want %q", version.Version, "dev")
	}
}

func TestCommit(t *testing.T) {
	t.Parallel()

	// Test that Commit is set to default value
	if version.Commit != "none" {
		t.Errorf("Commit = %q, want %q", version.Commit, "none")
	}
}

func TestBuildDate(t *testing.T) {
	t.Parallel()

	// Test that BuildDate is set to default value
	if version.BuildDate != "unknown" {
		t.Errorf("BuildDate = %q, want %q", version.BuildDate, "unknown")
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	// Test String() formatting
	got := version.String()
	want := "dev (commit: none, built: unknown)"

	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
