package snapshottimeline

import "time"

// SnapshotEntry 快照条目
type SnapshotEntry struct {
	ID          string            `json:"id"`
	PoolID      string            `json:"pool_id"`
	Dataset     string            `json:"dataset"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"created_at"`
	SizeBytes   int64             `json:"size_bytes"`
	State       SnapshotState     `json:"state"`
	Tags        []string          `json:"tags"`
	ParentID    string            `json:"parent_id,omitempty"`
	Children    []string          `json:"children,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SnapshotState 快照状态
type SnapshotState string

const (
	SnapshotStateActive   SnapshotState = "active"
	SnapshotStateCloned   SnapshotState = "cloned"
	SnapshotStateMounted  SnapshotState = "mounted"
	SnapshotStateRollback SnapshotState = "rollback"
	SnapshotStateDeleting SnapshotState = "deleting"
)

// TimelineFilter 时间线过滤器
type TimelineFilter struct {
	Dataset string        `json:"dataset,omitempty"`
	PoolID  string        `json:"pool_id,omitempty"`
	Since   time.Time     `json:"since,omitempty"`
	Until   time.Time     `json:"until,omitempty"`
	Tags    []string      `json:"tags,omitempty"`
	State   SnapshotState `json:"state,omitempty"`
	Limit   int           `json:"limit,omitempty"`
	Offset  int           `json:"offset,omitempty"`
}

// RestoreRequest 恢复请求
type RestoreRequest struct {
	SnapshotID  string `json:"snapshot_id"`
	TargetPath  string `json:"target_path,omitempty"`
	CreateClone bool   `json:"create_clone"`
	Force       bool   `json:"force"`
}

// RestoreResult 恢复结果
type RestoreResult struct {
	Success      bool   `json:"success"`
	RestoredPath string `json:"restored_path"`
	RestoreType  string `json:"restore_type"` // "rollback" or "clone"
	Duration     string `json:"duration"`
	Message      string `json:"message"`
}

// TimelineStats 时间线统计信息
type TimelineStats struct {
	TotalSnapshots  int            `json:"total_snapshots"`
	TotalSizeBytes  int64          `json:"total_size_bytes"`
	ByDataset       map[string]int `json:"by_dataset"`
	OldestSnapshot  time.Time      `json:"oldest_snapshot"`
	NewestSnapshot  time.Time      `json:"newest_snapshot"`
	AvgSnapshotSize int64          `json:"avg_snapshot_size"`
}

// SnapshotDiff 快照对比结果
type SnapshotDiff struct {
	Snapshot1    *SnapshotEntry `json:"snapshot1"`
	Snapshot2    *SnapshotEntry `json:"snapshot2"`
	SizeDelta    int64          `json:"size_delta"`
	TimeDelta    string         `json:"time_delta"`
	TagsAdded    []string       `json:"tags_added,omitempty"`
	TagsRemoved  []string       `json:"tags_removed,omitempty"`
	StateChanged bool           `json:"state_changed"`
}
