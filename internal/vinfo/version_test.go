package vinfo_test

import (
	"testing"

	"github.com/omarluq/cc-relay/internal/vinfo"
)

func TestVersion(t *testing.T) {
	// Version should always be non-empty.
	if vinfo.Version == "" {
		t.Error("Version is empty")
	}
}

func TestCommit(t *testing.T) {
	// Commit should always be non-empty.
	if vinfo.Commit == "" {
		t.Error("Commit is empty")
	}
}

func TestBuildDate(t *testing.T) {
	// BuildDate should always be non-empty.
	if vinfo.BuildDate == "" {
		t.Error("BuildDate is empty")
	}
}

func TestString(t *testing.T) {
	origVersion := vinfo.Version
	origCommit := vinfo.Commit
	t.Cleanup(func() {
		vinfo.Version = origVersion
		vinfo.Commit = origCommit
	})

	vinfo.Version = "v0.0.11-20-ga961617-dirty"
	vinfo.Commit = "a961617"

	got := vinfo.String()
	want := "v0.0.11-a961617-20"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
