package version

// Version is the current version of the application
const Version = "3.7.0"

// BuildTime is the time when the application was built
var BuildTime = "unknown"

// Commit is the git commit hash
var Commit = "unknown"

// GetVersion returns the current version string
func GetVersion() string {
	return Version
}

// GetBuildInfo returns build information as a map
func GetBuildInfo() map[string]string {
	return map[string]string{
		"version":    Version,
		"build_date": BuildTime,
		"git_commit": Commit,
	}
}
