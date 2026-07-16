package version

// Version is the current application version (no "v" prefix).
// Keep in sync with the root VERSION file (which uses a "v" prefix).
// Overridable at link time via -X nas-os/internal/version.Version=...
var Version = "3.23.1"

// BuildTime is the time when the application was built.
// Overridable at link time via -X nas-os/internal/version.BuildTime=...
var BuildTime = "unknown"

// Commit is the git commit hash.
// Overridable at link time via -X nas-os/internal/version.Commit=...
var Commit = "unknown"

// GetVersion returns the current version string.
func GetVersion() string {
	return Version
}

// GetBuildInfo returns build information as a map.
func GetBuildInfo() map[string]string {
	return map[string]string{
		"version":    Version,
		"build_date": BuildTime,
		"git_commit": Commit,
	}
}
