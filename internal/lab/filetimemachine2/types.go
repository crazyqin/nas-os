// Package filetimemachine2 提供文件系统时间机器功能，支持快照管理、版本浏览、差异对比和文件恢复。
package filetimemachine2

import "time"

// ========== 快照相关类型 ==========

// SnapshotStatus 快照状态.
type SnapshotStatus string

const (
	SnapshotCreating  SnapshotStatus = "creating"
	SnapshotCompleted SnapshotStatus = "completed"
	SnapshotFailed    SnapshotStatus = "failed"
	SnapshotDeleting  SnapshotStatus = "deleting"
	SnapshotExpired   SnapshotStatus = "expired"
)

// Snapshot 快照信息.
type Snapshot struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Status      SnapshotStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	Size        int64          `json:"size"`
	FileCount   int            `json:"file_count"`
	DirCount    int            `json:"dir_count"`
	Tags        []string       `json:"tags,omitempty"`
	IsAuto      bool           `json:"is_auto"` // 是否自动生成
	RootPath    string         `json:"root_path"`
	ErrorMsg    string         `json:"error_msg,omitempty"`
}

// SnapshotListItem 快照列表项（轻量）.
type SnapshotListItem struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Status    SnapshotStatus `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	Size      int64          `json:"size"`
	FileCount int            `json:"file_count"`
	Tags      []string       `json:"tags,omitempty"`
}

// CreateSnapshotRequest 创建快照请求.
type CreateSnapshotRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description,omitempty"`
	RootPath    string   `json:"root_path" binding:"required"`
	Tags        []string `json:"tags,omitempty"`
	Force       bool     `json:"force"` // 即使无变更也强制创建
}

// RestoreRequest 恢复请求.
type RestoreRequest struct {
	TargetPath    string   `json:"target_path" binding:"required"` // 恢复目标路径
	SourcePaths   []string `json:"source_paths"`                   // 指定要恢复的文件/目录，空则全量恢复
	OverwriteMode string   `json:"overwrite_mode"`                 // overwrite | coexist
	DryRun        bool     `json:"dry_run"`                        // 仅预览不执行
}

// RestoreResult 恢复结果.
type RestoreResult struct {
	RestoredFiles int      `json:"restored_files"`
	RestoredDirs  int      `json:"restored_dirs"`
	SkippedFiles  int      `json:"skipped_files"`
	FailedFiles   int      `json:"failed_files"`
	FailedPaths   []string `json:"failed_paths,omitempty"`
	TotalBytes    int64    `json:"total_bytes"`
	IsDryRun      bool     `json:"is_dry_run"`
}

// SnapshotContent 快照浏览内容.
type SnapshotContent struct {
	SnapshotID string      `json:"snapshot_id"`
	Path       string      `json:"path"`
	Entries    []FileEntry `json:"entries"`
	TotalCount int         `json:"total_count"`
}

// FileEntry 文件条目.
type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
	Hash    string    `json:"hash,omitempty"` // 文件内容 hash
}

// TagRequest 标签请求.
type TagRequest struct {
	Tags []string `json:"tags" binding:"required"`
}

// ========== Diff 相关类型 ==========

// DiffType 差异类型.
type DiffType string

const (
	DiffAdded    DiffType = "added"
	DiffModified DiffType = "modified"
	DiffDeleted  DiffType = "deleted"
	DiffRenamed  DiffType = "renamed"
)

// FileDiff 文件差异.
type FileDiff struct {
	Path      string     `json:"path"`
	Type      DiffType   `json:"type"`
	OldPath   string     `json:"old_path,omitempty"` // 重命名时的旧路径
	OldSize   int64      `json:"old_size,omitempty"`
	NewSize   int64      `json:"new_size,omitempty"`
	OldHash   string     `json:"old_hash,omitempty"`
	NewHash   string     `json:"new_hash,omitempty"`
	IsBinary  bool       `json:"is_binary"`
	LineDiffs []LineDiff `json:"line_diffs,omitempty"` // 文本文件行级差异
}

// LineDiff 行级差异.
type LineDiff struct {
	OldLine int    `json:"old_line"` // 0 表示新增行
	NewLine int    `json:"new_line"` // 0 表示删除行
	Type    string `json:"type"`     // "equal", "added", "deleted"
	Content string `json:"content"`
}

// DiffResult 差异对比结果.
type DiffResult struct {
	SnapshotA  string     `json:"snapshot_a"`
	SnapshotB  string     `json:"snapshot_b"`
	Changes    []FileDiff `json:"changes"`
	Stats      DiffStats  `json:"stats"`
	ComparedAt time.Time  `json:"compared_at"`
}

// DiffStats 变更统计.
type DiffStats struct {
	Added        int   `json:"added"`
	Modified     int   `json:"modified"`
	Deleted      int   `json:"deleted"`
	Renamed      int   `json:"renamed"`
	Total        int   `json:"total"`
	BytesAdded   int64 `json:"bytes_added"`
	BytesDeleted int64 `json:"bytes_deleted"`
}

// ========== 时间线相关类型 ==========

// AggregationGranularity 聚合粒度.
type AggregationGranularity string

const (
	GranularityHour  AggregationGranularity = "hour"
	GranularityDay   AggregationGranularity = "day"
	GranularityWeek  AggregationGranularity = "week"
	GranularityMonth AggregationGranularity = "month"
)

// TimelineRequest 时间线请求.
type TimelineRequest struct {
	Granularity string `form:"granularity"` // hour, day, week, month
	StartTime   string `form:"start_time"`
	EndTime     string `form:"end_time"`
}

// TimelineData 时间线数据.
type TimelineData struct {
	Granularity AggregationGranularity `json:"granularity"`
	Buckets     []TimelineBucket       `json:"buckets"`
	Total       int                    `json:"total"`
}

// TimelineBucket 时间线桶.
type TimelineBucket struct {
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	SnapshotIDs   []string  `json:"snapshot_ids"`
	SnapshotCount int       `json:"snapshot_count"`
	TotalSize     int64     `json:"total_size"`
}

// ========== 保留策略相关类型 ==========

// RetentionRule 保留规则.
type RetentionRule struct {
	Name     string `json:"name"`     // 规则名称
	Interval string `json:"interval"` // 时间间隔: 1h, 1d, 1w, 1m
	Count    int    `json:"count"`    // 保留数量
	Priority int    `json:"priority"` // 优先级，数字越大越优先
}

// RetentionConfig 保留策略配置.
type RetentionConfig struct {
	Enabled       bool            `json:"enabled"`
	Rules         []RetentionRule `json:"rules"`
	MaxSnapshots  int             `json:"max_snapshots"`  // 最大快照总数
	MaxTotalSize  int64           `json:"max_total_size"` // 最大总存储（字节）
	AutoCleanup   bool            `json:"auto_cleanup"`   // 自动清理过期快照
	LastCleanupAt *time.Time      `json:"last_cleanup_at,omitempty"`
	NextCleanupAt *time.Time      `json:"next_cleanup_at,omitempty"`
}

// UpdateRetentionRequest 更新保留策略请求.
type UpdateRetentionRequest struct {
	Enabled      bool            `json:"enabled"`
	Rules        []RetentionRule `json:"rules"`
	MaxSnapshots int             `json:"max_snapshots"`
	MaxTotalSize int64           `json:"max_total_size"`
	AutoCleanup  bool            `json:"auto_cleanup"`
}

// CleanupResult 清理结果.
type CleanupResult struct {
	DeletedCount   int      `json:"deleted_count"`
	ReclaimedBytes int64    `json:"reclaimed_bytes"`
	DeletedIDs     []string `json:"deleted_ids,omitempty"`
}

// ========== 存储统计相关类型 ==========

// StorageStats 存储统计.
type StorageStats struct {
	TotalSnapshots  int        `json:"total_snapshots"`
	TotalSize       int64      `json:"total_size"`      // 总占用空间
	UniqueSize      int64      `json:"unique_size"`     // 去重后唯一数据大小
	DedupRate       float64    `json:"dedup_rate"`      // 去重率 (0-1)
	CompressedSize  int64      `json:"compressed_size"` // 压缩后大小
	CompressRate    float64    `json:"compress_rate"`   // 压缩率 (0-1)
	OldestSnapshot  *time.Time `json:"oldest_snapshot,omitempty"`
	NewestSnapshot  *time.Time `json:"newest_snapshot,omitempty"`
	AvgSnapshotSize int64      `json:"avg_snapshot_size"`
}

// ========== 搜索相关类型 ==========

// SearchRequest 搜索请求.
type SearchRequest struct {
	FileName  string `form:"file_name"`  // 文件名（支持通配符）
	StartTime string `form:"start_time"` // 搜索起始时间
	EndTime   string `form:"end_time"`   // 搜索结束时间
	Tag       string `form:"tag"`        // 标签过滤
	MinSize   int64  `form:"min_size"`   // 最小文件大小
	MaxSize   int64  `form:"max_size"`   // 最大文件大小
	Limit     int    `form:"limit"`      // 返回数量限制
}

// SearchResult 搜索结果.
type SearchResult struct {
	Total   int           `json:"total"`
	Entries []SearchEntry `json:"entries"`
}

// SearchEntry 搜索结果条目.
type SearchEntry struct {
	SnapshotID   string    `json:"snapshot_id"`
	SnapshotName string    `json:"snapshot_name"`
	FilePath     string    `json:"file_path"`
	FileName     string    `json:"file_name"`
	FileSize     int64     `json:"file_size"`
	ModTime      time.Time `json:"mod_time"`
	SnapshotTime time.Time `json:"snapshot_time"`
}
