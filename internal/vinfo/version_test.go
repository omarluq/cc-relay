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

func TestFormatDisplayVersionDescribeWithDirty(t *testing.T) {
	t.Parallel()

	got := vinfo.FormatDisplayVersion("v0.0.11-20-ga961617-dirty", "a961617")
	want := "v0.0.11-a961617-20"
	if got != want {
		t.Errorf("FormatDisplayVersion() = %q, want %q", got, want)
	}
}

func TestShortCommit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		commit string
		want   string
	}{
		{"long commit", "a961617123456789", "a961617"},
		{"short commit", "a96161", "a96161"},
		{"empty commit", "", ""},
		{"exactly 7 chars", "a961617", "a961617"},
		{"3 chars", "abc", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := vinfo.ShortCommit(tt.commit)
			if got != tt.want {
				t.Errorf("ShortCommit(%q) = %q, want %q", tt.commit, got, tt.want)
			}
		})
	}
}

func TestParseDescribe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		version   string
		wantBase  string
		wantCount string
		wantSHA   string
		wantDirty bool
	}{
		{
			name: "full describe with dirty", version: "v0.0.11-20-ga961617-dirty",
			wantBase: "v0.0.11", wantCount: "20", wantSHA: "a961617", wantDirty: true,
		},
		{
			name: "full describe without dirty", version: "v0.0.11-20-ga961617",
			wantBase: "v0.0.11", wantCount: "20", wantSHA: "a961617", wantDirty: false,
		},
		{
			name: "simple tag", version: "v0.0.11",
			wantBase: "v0.0.11", wantCount: "", wantSHA: "", wantDirty: false,
		},
		{
			name: "empty version", version: "",
			wantBase: "", wantCount: "", wantSHA: "", wantDirty: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			base, count, sha, dirty := vinfo.ParseDescribe(testCase.version)
			if base != testCase.wantBase {
				t.Errorf("ParseDescribe(%q) base = %q, want %q", testCase.version, base, testCase.wantBase)
			}
			if count != testCase.wantCount {
				t.Errorf("ParseDescribe(%q) count = %q, want %q", testCase.version, count, testCase.wantCount)
			}
			if sha != testCase.wantSHA {
				t.Errorf("ParseDescribe(%q) sha = %q, want %q", testCase.version, sha, testCase.wantSHA)
			}
			if dirty != testCase.wantDirty {
				t.Errorf("ParseDescribe(%q) dirty = %v, want %v", testCase.version, dirty, testCase.wantDirty)
			}
		})
	}
}

func TestFormatDisplayVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		commit  string
		want    string
	}{
		{"clean tag", "v0.0.11", "a961617", "v0.0.11"},
		{"describe with commit", "v0.0.11-20-ga961617", "a961617", "v0.0.11-a961617-20"},
		{"describe with dirty", "v0.0.11-20-ga961617-dirty", "a961617", "v0.0.11-a961617-20"},
		{"empty version", "", "abc123", "dev"},
		{"dev with commit", "dev", "a961617123456789", "dev"},
		{"commit none returns base", "v1.0.0", "none", "v1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := vinfo.FormatDisplayVersion(tt.version, tt.commit)
			if got != tt.want {
				t.Errorf("FormatDisplayVersion(%q, %q) = %q, want %q", tt.version, tt.commit, got, tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	got := vinfo.String()
	if got == "" {
		t.Error("String() returned empty string")
	}
}
