package vinfo_test

import (
	"testing"

	"github.com/omarluq/cc-relay/internal/vinfo"
)

func TestVersion(t *testing.T) {
	t.Parallel()
	// Version should always be non-empty.
	if vinfo.Version == "" {
		t.Error("Version is empty")
	}
}

func TestCommit(t *testing.T) {
	t.Parallel()
	// Commit should always be non-empty.
	if vinfo.Commit == "" {
		t.Error("Commit is empty")
	}
}

func TestBuildDate(t *testing.T) {
	t.Parallel()
	// BuildDate should always be non-empty.
	if vinfo.BuildDate == "" {
		t.Error("BuildDate is empty")
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	got := vinfo.FormatDisplayVersion("v0.0.11-20-ga961617-dirty", "a961617")
	want := "v0.0.11-a961617-20"
	if got != want {
		t.Errorf("FormatDisplayVersion() = %q, want %q", got, want)
	}
}
