package filetimeline

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// FileVersion represents a point-in-time version of a file
type FileVersion struct {
	ID          string            `json:"id"`
	FilePath    string            `json:"file_path"`
	Version     int               `json:"version"`
	Size        int64             `json:"size"`
	Checksum    string            `json:"checksum"`
	Timestamp   time.Time         `json:"timestamp"`
	Author      string            `json:"author"`
	Message     string            `json:"message"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	IsSnapshot  bool              `json:"is_snapshot"`
	SnapshotID  string            `json:"snapshot_id,omitempty"`
}

// TimelineEntry represents a node in the file timeline
type TimelineEntry struct {
	Version     *FileVersion     `json:"version"`
	Children    []*TimelineEntry `json:"children,omitempty"`
	BranchName  string           `json:"branch_name"`
	IsCurrent   bool             `json:"is_current"`
	Depth       int              `json:"depth"`
}

// DiffResult represents differences between two versions
type DiffResult struct {
	OldVersion  int          `json:"old_version"`
	NewVersion  int          `json:"new_version"`
	Changes     []Change     `json:"changes"`
	Summary     string       `json:"summary"`
	Timestamp   time.Time    `json:"timestamp"`
}

// Change represents a single change between versions
type Change struct {
	Type     string `json:"type"` // added, removed, modified
	Path     string `json:"path"`
	OldValue string `json:"old_value,omitempty"`
	NewValue string `json:"new_value,omitempty"`
	Line     int    `json:"line,omitempty"`
}

// RestoreResult represents the result of a file restore
type RestoreResult struct {
	FilePath    string    `json:"file_path"`
	Version     int       `json:"version"`
	RestoredAt  time.Time `json:"restored_at"`
	Size        int64     `json:"size"`
	Checksum    string    `json:"checksum"`
}

// TimelineStats aggregates timeline statistics
type TimelineStats struct {
	TotalVersions   int            `json:"total_versions"`
	TotalSize       int64          `json:"total_size"`
	OldestVersion   time.Time      `json:"oldest_version"`
	NewestVersion   time.Time      `json:"newest_version"`
	ByAuthor        map[string]int `json:"by_author"`
	ByTag           map[string]int `json:"by_tag"`
	SnapshotCount   int            `json:"snapshot_count"`
}

// FileTimeline manages file version history with visual timeline
type FileTimeline struct {
	mu       sync.RWMutex
	versions map[string][]*FileVersion // path -> versions
	current  map[string]int            // path -> current version
}

// NewFileTimeline creates a new file timeline manager
func NewFileTimeline() *FileTimeline {
	return &FileTimeline{
		versions: make(map[string][]*FileVersion),
		current:  make(map[string]int),
	}
}

// CommitVersion creates a new version of a file
func (ft *FileTimeline) CommitVersion(ctx context.Context, filePath string, size int64, data []byte, author string, message string) (*FileVersion, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	checksum := fmt.Sprintf("%x", sha256.Sum256(data))

	versionNum := ft.current[filePath] + 1

	version := &FileVersion{
		ID:        fmt.Sprintf("ver-%s-%d", filePath, versionNum),
		FilePath:  filePath,
		Version:   versionNum,
		Size:      size,
		Checksum:  checksum,
		Timestamp: time.Now(),
		Author:    author,
		Message:   message,
	}

	ft.versions[filePath] = append(ft.versions[filePath], version)
	ft.current[filePath] = versionNum

	return version, nil
}

// CreateSnapshot creates a snapshot of the current file state
func (ft *FileTimeline) CreateSnapshot(ctx context.Context, filePath string, message string) (*FileVersion, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	versions, ok := ft.versions[filePath]
	if !ok || len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for %s", filePath)
	}

	latest := versions[len(versions)-1]

	snapshot := &FileVersion{
		ID:         fmt.Sprintf("snap-%s-%d", filePath, time.Now().Unix()),
		FilePath:   filePath,
		Version:    latest.Version,
		Size:       latest.Size,
		Checksum:   latest.Checksum,
		Timestamp:  time.Now(),
		Author:     "system",
		Message:    message,
		IsSnapshot: true,
		SnapshotID: fmt.Sprintf("snap-%d", time.Now().Unix()),
	}

	ft.versions[filePath] = append(ft.versions[filePath], snapshot)

	return snapshot, nil
}

// GetTimeline returns the visual timeline for a file
func (ft *FileTimeline) GetTimeline(ctx context.Context, filePath string) (*TimelineEntry, error) {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	versions, ok := ft.versions[filePath]
	if !ok || len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for %s", filePath)
	}

	// Build tree structure
	root := &TimelineEntry{
		BranchName: "main",
		Depth:      0,
	}

	currentVersion := ft.current[filePath]
	var prev *TimelineEntry

	for i, v := range versions {
		entry := &TimelineEntry{
			Version:   v,
			IsCurrent: v.Version == currentVersion,
			Depth:     i,
		}

		if v.IsSnapshot {
			entry.BranchName = fmt.Sprintf("snapshot-%s", v.SnapshotID)
		}

		if prev != nil {
			prev.Children = []*TimelineEntry{entry}
		} else {
			root.Children = []*TimelineEntry{entry}
		}
		prev = entry
	}

	return root, nil
}

// DiffVersions compares two versions of a file
func (ft *FileTimeline) DiffVersions(ctx context.Context, filePath string, oldVersion, newVersion int) (*DiffResult, error) {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	versions, ok := ft.versions[filePath]
	if !ok {
		return nil, fmt.Errorf("no versions found for %s", filePath)
	}

	var oldVer, newVer *FileVersion
	for _, v := range versions {
		if v.Version == oldVersion {
			oldVer = v
		}
		if v.Version == newVersion {
			newVer = v
		}
	}

	if oldVer == nil || newVer == nil {
		return nil, fmt.Errorf("version not found")
	}

	changes := []Change{
		{
			Type:     "modified",
			Path:     filePath,
			OldValue: oldVer.Checksum,
			NewValue: newVer.Checksum,
		},
	}

	return &DiffResult{
		OldVersion: oldVersion,
		NewVersion: newVersion,
		Changes:    changes,
		Summary:    fmt.Sprintf("Version %d -> %d", oldVersion, newVersion),
		Timestamp:  time.Now(),
	}, nil
}

// RestoreVersion restores a file to a specific version
func (ft *FileTimeline) RestoreVersion(ctx context.Context, filePath string, version int) (*RestoreResult, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	versions, ok := ft.versions[filePath]
	if !ok {
		return nil, fmt.Errorf("no versions found for %s", filePath)
	}

	var target *FileVersion
	for _, v := range versions {
		if v.Version == version {
			target = v
			break
		}
	}

	if target == nil {
		return nil, fmt.Errorf("version %d not found", version)
	}

	// Create a new version for the restore
	newVersion := ft.current[filePath] + 1
	restore := &FileVersion{
		ID:       fmt.Sprintf("ver-%s-%d", filePath, newVersion),
		FilePath: filePath,
		Version:  newVersion,
		Size:     target.Size,
		Checksum: target.Checksum,
		Timestamp: time.Now(),
		Author:   "system",
		Message:  fmt.Sprintf("Restored from version %d", version),
	}

	ft.versions[filePath] = append(ft.versions[filePath], restore)
	ft.current[filePath] = newVersion

	return &RestoreResult{
		FilePath:   filePath,
		Version:    version,
		RestoredAt: time.Now(),
		Size:       target.Size,
		Checksum:   target.Checksum,
	}, nil
}

// GetStats returns timeline statistics
func (ft *FileTimeline) GetStats(ctx context.Context, filePath string) (*TimelineStats, error) {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	versions, ok := ft.versions[filePath]
	if !ok {
		return &TimelineStats{}, nil
	}

	stats := &TimelineStats{
		TotalVersions: len(versions),
		ByAuthor:      make(map[string]int),
		ByTag:         make(map[string]int),
	}

	for _, v := range versions {
		stats.TotalSize += v.Size
		stats.ByAuthor[v.Author]++

		if v.IsSnapshot {
			stats.SnapshotCount++
		}

		for _, tag := range v.Tags {
			stats.ByTag[tag]++
		}

		if stats.OldestVersion.IsZero() || v.Timestamp.Before(stats.OldestVersion) {
			stats.OldestVersion = v.Timestamp
		}
		if v.Timestamp.After(stats.NewestVersion) {
			stats.NewestVersion = v.Timestamp
		}
	}

	return stats, nil
}

// ListVersions returns all versions for a file
func (ft *FileTimeline) ListVersions(ctx context.Context, filePath string) []*FileVersion {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	return ft.versions[filePath]
}
