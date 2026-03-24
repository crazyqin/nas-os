// Package version provides version information for NAS-OS
package version

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Version information - can be overridden via ldflags
// Example: go build -ldflags "-X nas-os/internal/version.Version=1.0.0"
var (
	Version   = "2.253.287" // Default, may be overridden by ldflags or VERSION file
	BuildDate = "2026-03-24"
	GitCommit = ""
	GoVersion = runtime.Version()
	Platform  = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

// initialized tracks whether we've loaded from VERSION file
var initialized bool
var initOnce sync.Once

// initVersion loads version from VERSION file if not set via ldflags
func initVersion() {
	initOnce.Do(func() {
		initialized = true

		// Try to find VERSION file relative to executable
		exePath, err := os.Executable()
		if err != nil {
			return
		}

		// Look for VERSION file in several locations
		versionPaths := []string{
			filepath.Join(filepath.Dir(exePath), "VERSION"),
			"/etc/nas-os/VERSION",
			"/var/lib/nas-os/VERSION",
		}

		for _, path := range versionPaths {
			if data, err := os.ReadFile(path); err == nil {
				fileVersion := strings.TrimSpace(string(data))
				fileVersion = strings.TrimPrefix(fileVersion, "v")
				if fileVersion != "" {
					Version = fileVersion
					return
				}
			}
		}
	})
}

// GetVersion returns the current version, loading from VERSION file if needed
func GetVersion() string {
	if !initialized {
		initVersion()
	}
	return Version
}

// Info returns version information
func Info() map[string]string {
	if !initialized {
		initVersion()
	}
	return map[string]string{
		"version":    Version,
		"build_date": BuildDate,
		"git_commit": GitCommit,
		"go_version": GoVersion,
		"platform":   Platform,
	}
}

// String returns version string
func String() string {
	if !initialized {
		initVersion()
	}
	return fmt.Sprintf("NAS-OS v%s (%s)", Version, Platform)
}
