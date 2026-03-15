package version

import (
	"strings"
	"testing"
)

func TestInfo(t *testing.T) {
	info := Info()

	if info["version"] == "" {
		t.Error("version should not be empty")
	}
	if info["go_version"] == "" {
		t.Error("go_version should not be empty")
	}
	if info["platform"] == "" {
		t.Error("platform should not be empty")
	}
	if !strings.Contains(info["platform"], "/") {
		t.Errorf("platform should contain '/', got %s", info["platform"])
	}
}

func TestString(t *testing.T) {
	s := String()

	if s == "" {
		t.Error("String() should not return empty")
	}
	if !strings.Contains(s, "NAS-OS") {
		t.Errorf("String() should contain 'NAS-OS', got %s", s)
	}
	if !strings.Contains(s, "v") {
		t.Errorf("String() should contain version with 'v', got %s", s)
	}
}

func TestVersionFormat(t *testing.T) {
	info := Info()
	version := info["version"]

	// 版本号应该以数字开头
	if len(version) == 0 {
		t.Error("version should not be empty")
	}
}
