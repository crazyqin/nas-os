// Package version provides version information for NAS-OS
package version

import (
	"fmt"
	"runtime"
)

// Version information
var (
	Version   = "2.132.0"
	BuildDate = ""
	GitCommit = ""
	GoVersion = runtime.Version()
	Platform  = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

// Info returns version information
func Info() map[string]string {
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
	return fmt.Sprintf("NAS-OS v%s (%s)", Version, Platform)
}
