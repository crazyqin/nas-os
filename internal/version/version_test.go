package version

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	v := GetVersion()

	if v == "" {
		t.Error("GetVersion() should not return empty")
	}
	if !strings.Contains(v, ".") {
		t.Errorf("GetVersion() should contain '.', got %s", v)
	}
}

func TestGetBuildInfo(t *testing.T) {
	info := GetBuildInfo()

	if info["version"] == "" {
		t.Error("version should not be empty")
	}
	if info["build_date"] == "" {
		t.Error("build_date should not be empty")
	}
	if info["git_commit"] == "" {
		t.Error("git_commit should not be empty")
	}
	if info["version"] != GetVersion() {
		t.Errorf("GetBuildInfo version %q != GetVersion %q", info["version"], GetVersion())
	}
}

func TestVersionFormat(t *testing.T) {
	v := GetVersion()

	if len(v) == 0 {
		t.Error("version should not be empty")
	}
	// Must be semver-like digits.digits... without a leading "v".
	if strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V") {
		t.Errorf("GetVersion() must not include leading v, got %q", v)
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		t.Errorf("GetVersion() should be at least major.minor, got %q", v)
	}
	for _, p := range parts {
		if p == "" {
			t.Errorf("GetVersion() has empty segment: %q", v)
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				// allow pre-release suffixes only after first non-digit via hyphen in a segment
				if r == '-' {
					break
				}
				t.Errorf("GetVersion() segment %q has non-digit %q in %q", p, string(r), v)
				break
			}
		}
	}
}

// TestVersionMatchesVERSIONFile asserts the shipped constant tracks the repo VERSION file.
// This is the real user-facing version source of truth for releases.
func TestVersionMatchesVERSIONFile(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/version -> repo root
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	data, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	want := strings.TrimSpace(string(data))
	want = strings.TrimPrefix(want, "v")
	want = strings.TrimPrefix(want, "V")
	if want == "" {
		t.Fatal("VERSION file is empty")
	}
	got := GetVersion()
	if got != want {
		t.Fatalf("GetVersion() = %q, want %q (from VERSION file)", got, want)
	}
}
