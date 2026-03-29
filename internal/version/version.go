package version

// Version information
const (
	Version   = "2.310.0"
	BuildDate = "2026-03-29"
	GitCommit = "v2.310.0"
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