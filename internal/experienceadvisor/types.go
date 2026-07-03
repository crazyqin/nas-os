// Package experienceadvisor provides a competitor-inspired NAS experience advisor.
// It turns local usage signals into privacy-preserving feature recommendations
// for photos, media, backup, remote access, snapshots, and app discovery.
package experienceadvisor

import "time"

// WorkloadType describes a high-level NAS usage pattern.
type WorkloadType string

const (
	WorkloadPhotos  WorkloadType = "photos"
	WorkloadMedia   WorkloadType = "media"
	WorkloadBackup  WorkloadType = "backup"
	WorkloadRemote  WorkloadType = "remote_access"
	WorkloadApps    WorkloadType = "apps"
	WorkloadStorage WorkloadType = "storage"
)

// Signal is a normalized local usage signal. It intentionally contains no
// filenames, user names, tokens, or other private data.
type Signal struct {
	Workload      WorkloadType `json:"workload"`
	ItemCount     int          `json:"item_count"`
	SizeBytes     int64        `json:"size_bytes"`
	ActiveDevices int          `json:"active_devices"`
	ErrorCount    int          `json:"error_count"`
	LastActivity  time.Time    `json:"last_activity"`
	Enabled       bool         `json:"enabled"`
}

// AdvisorConfig controls recommendation thresholds.
type AdvisorConfig struct {
	LargePhotoLibraryCount int
	LargeMediaLibraryGB    int64
	BackupSizeGB           int64
	MinActiveDevices       int
	HighErrorCount         int
	StaleDays              int
}

// DefaultConfig returns conservative thresholds suited for home NAS systems.
func DefaultConfig() AdvisorConfig {
	return AdvisorConfig{
		LargePhotoLibraryCount: 5000,
		LargeMediaLibraryGB:    500,
		BackupSizeGB:           100,
		MinActiveDevices:       2,
		HighErrorCount:         3,
		StaleDays:              14,
	}
}

// Recommendation is a user-facing action suggested by the advisor.
type Recommendation struct {
	ID          string       `json:"id"`
	Workload    WorkloadType `json:"workload"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Priority    int          `json:"priority"` // 100 is highest
	Actions     []string     `json:"actions"`
	Benchmarks  []string     `json:"benchmarks,omitempty"`
}
