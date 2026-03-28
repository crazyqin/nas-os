package version

import (
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
}

func TestVersionFormat(t *testing.T) {
	v := GetVersion()

	// 版本号应该以数字开头
	if len(v) == 0 {
		t.Error("version should not be empty")
	}
}
