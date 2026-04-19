package vinfo_test

import (
	"runtime/debug"
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

func TestCommit(t *testing.T) { //nolint:paralleltest // mutates package-level vars
	restore := vinfo.SetVersionVars("", "test-commit", "")
	defer restore()

	// Commit should always be non-empty.
	if vinfo.Commit == "" {
		t.Error("Commit is empty")
	}
}

func TestBuildDate(t *testing.T) { //nolint:paralleltest // mutates package-level vars
	restore := vinfo.SetVersionVars("", "", "test-build-date")
	defer restore()

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

func TestApplySettingsFallback(t *testing.T) { //nolint:paralleltest // mutates package-level vars
	restore := vinfo.SetVersionVars("", "", "")
	defer restore()

	tests := []struct {
		name          string
		wantVersion   string
		wantCommit    string
		wantBuildDate string
		settings      []debug.BuildSetting
	}{
		{
			name: "vcs.revision and vcs.time from settings",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123def456"},
				{Key: "vcs.time", Value: "2026-04-18T12:00:00Z"},
			},
			wantVersion:   "",
			wantCommit:    "abc123def456",
			wantBuildDate: "2026-04-18T12:00:00Z",
		},
		{
			name: "only vcs.revision",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "xyz789"},
			},
			wantVersion:   "",
			wantCommit:    "xyz789",
			wantBuildDate: "",
		},
		{
			name:          "empty settings",
			settings:      []debug.BuildSetting{},
			wantVersion:   "",
			wantCommit:    "",
			wantBuildDate: "",
		},
		{
			name: "unrelated keys are ignored",
			settings: []debug.BuildSetting{
				{Key: "GOOS", Value: "linux"},
				{Key: "vcs.revision", Value: "abc123"},
			},
			wantVersion:   "",
			wantCommit:    "abc123",
			wantBuildDate: "",
		},
	}

	for _, testCase := range tests { //nolint:paralleltest // mutates package-level vars
		t.Run(testCase.name, func(t *testing.T) {
			restoreInner := vinfo.SetVersionVars("", "", "")
			defer restoreInner()

			info := &debug.BuildInfo{
				Settings: testCase.settings,
			}
			vinfo.ApplySettingsFallback(info)

			if vinfo.Commit != testCase.wantCommit {
				t.Errorf("ApplySettingsFallback() Commit = %q, want %q", vinfo.Commit, testCase.wantCommit)
			}
			if vinfo.BuildDate != testCase.wantBuildDate {
				t.Errorf("ApplySettingsFallback() BuildDate = %q, want %q", vinfo.BuildDate, testCase.wantBuildDate)
			}
		})
	}
}

func TestApplyMainVersionFallback(t *testing.T) { //nolint:paralleltest // mutates package-level vars
	restore := vinfo.SetVersionVars("", "", "")
	defer restore()

	tests := []struct {
		name        string
		mainVersion string
		wantVersion string
	}{
		{
			name:        "valid version",
			mainVersion: "v1.2.3",
			wantVersion: "v1.2.3",
		},
		{
			name:        "empty version is ignored",
			mainVersion: "",
			wantVersion: "",
		},
		{
			name:        "devel marker is ignored",
			mainVersion: "(devel)",
			wantVersion: "",
		},
		{
			name:        "version with build metadata",
			mainVersion: "v1.2.3+meta",
			wantVersion: "v1.2.3+meta",
		},
	}

	for _, testCase := range tests { //nolint:paralleltest // mutates package-level vars
		t.Run(testCase.name, func(t *testing.T) {
			restoreInner := vinfo.SetVersionVars("", "", "")
			defer restoreInner()

			info := &debug.BuildInfo{
				Main: debug.Module{Path: "test", Version: testCase.mainVersion},
			}
			vinfo.ApplyMainVersionFallback(info)

			if vinfo.Version != testCase.wantVersion {
				t.Errorf("ApplyMainVersionFallback() Version = %q, want %q", vinfo.Version, testCase.wantVersion)
			}
		})
	}
}
