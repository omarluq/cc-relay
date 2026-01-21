package version_test

import (
	"testing"

	"github.com/omarluq/cc-relay/internal/version"
)

func TestVersion(t *testing.T) {
	t.Parallel()

	got := version.Version()
	if got != "0.0.1" {
		t.Errorf("Version() = %q, want %q", got, "0.0.1")
	}
}
