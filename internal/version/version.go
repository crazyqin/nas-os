package version

// Version information
const (
	Version   = "2.388.0"
	BuildDate = "2026-04-04"
	GitCommit = "v2.367.0"
)

// GetVersion returns the current version
func GetVersion() string {
	return Version
}

// GetBuildInfo returns build information
func GetBuildInfo() map[string]string {
	return map[string]string{
		"version":    Version,
		"build_date": BuildDate,
		"git_commit": GitCommit,
	}
}
