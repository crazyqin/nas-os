// Package filetimemachine 提供文件时光机功能，支持文件版本历史管理、任意时间点恢复、
// 变更差异对比、批量回滚、自动快照策略、存储空间分析、回收站增强和文件锁定。
// 对标 macOS Time Machine + 群晖 Snapshot Replication。
package filetimemachine

import "time"

// ==================== 快照相关 ====================

// SnapshotStatus 快照状态.
type SnapshotStatus string

const (
	SnapshotStatusPending  SnapshotStatus = "pending"  // 待创建
	SnapshotStatusCreating SnapshotStatus = "creating" // 创建中
	SnapshotStatusActive   SnapshotStatus = "active"   // 活跃
	SnapshotStatusArchived SnapshotStatus = "archived" // 已归档
	SnapshotStatusDeleting SnapshotStatus = "deleting" // 删除中
	SnapshotStatusFailed   SnapshotStatus = "failed"   // 失败
)

// SnapshotTrigger 触发方式.
type SnapshotTrigger string

const (
	TriggerManual    SnapshotTrigger = "manual"    // 手动触发
	TriggerScheduled SnapshotTrigger = "scheduled" // 定时触发
	TriggerEvent     SnapshotTrigger = "event"     // 事件触发（文件变更）
	TriggerPolicy    SnapshotTrigger = "policy"    // 策略触发
)

// Snapshot 快照记录.
type Snapshot struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Status      SnapshotStatus  `json:"status"`
	Trigger     SnapshotTrigger `json:"trigger"`
	Path        string          `json:"path"`        // 快照根路径
	TotalFiles  int64           `json:"total_files"` // 文件总数
	TotalSize   int64           `json:"total_size"`  // 总大小（字节）
	DeltaSize   int64           `json:"delta_size"`  // 增量大小（字节）
	Tags        []string        `json:"tags,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	ExpiresAt   *time.Time      `json:"expires_at,omitempty"` // 过期时间，nil 表示永不过期
}

// SnapshotSummary 快照摘要（列表用）.
type SnapshotSummary struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Status     SnapshotStatus  `json:"status"`
	Trigger    SnapshotTrigger `json:"trigger"`
	TotalFiles int64           `json:"total_files"`
	TotalSize  int64           `json:"total_size"`
	DeltaSize  int64           `json:"delta_size"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ==================== 快照策略相关 ====================

// ScheduleUnit 调度单位.
type ScheduleUnit string

const (
	ScheduleMinutely ScheduleUnit = "minutely" // 分钟
	ScheduleHourly   ScheduleUnit = "hourly"   // 小时
	ScheduleDaily    ScheduleUnit = "daily"    // 每天
	ScheduleWeekly   ScheduleUnit = "weekly"   // 每周
	ScheduleMonthly  ScheduleUnit = "monthly"  // 每月
)

// RetentionAction 保留策略动作.
type RetentionAction string

const (
	RetentionDelete  RetentionAction = "delete"  // 删除过期快照
	RetentionArchive RetentionAction = "archive" // 归档过期快照
	RetentionKeep    RetentionAction = "keep"    // 保留（忽略过期）
)

// SnapshotPolicy 自动快照策略.
type SnapshotPolicy struct {
	ID              string          `json:"id"`
	Name            string          `json:"name" binding:"required"`
	Description     string          `json:"description,omitempty"`
	Enabled         bool            `json:"enabled"`
	TargetPaths     []string        `json:"target_paths" binding:"required,min=1"` // 监控路径列表
	Schedule        ScheduleUnit    `json:"schedule" binding:"required"`           // 调度频率
	ScheduleValue   int             `json:"schedule_value"`                        // 调度值（如每 N 小时）
	TimeOfDay       string          `json:"time_of_day,omitempty"`                 // 执行时间（HH:MM 格式）
	DayOfWeek       int             `json:"day_of_week,omitempty"`                 // 周几（0=周日）
	DayOfMonth      int             `json:"day_of_month,omitempty"`                // 每月第几天
	MaxSnapshots    int             `json:"max_snapshots"`                         // 最大快照数量
	RetentionDays   int             `json:"retention_days"`                        // 保留天数
	RetentionAction RetentionAction `json:"retention_action"`                      // 过期动作
	ExcludePatterns []string        `json:"exclude_patterns,omitempty"`            // 排除模式（glob）
	IncludePatterns []string        `json:"include_patterns,omitempty"`            // 包含模式（glob）
	Tags            []string        `json:"tags,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	LastRunAt       *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt       *time.Time      `json:"next_run_at,omitempty"`
}

// ==================== 文件版本相关 ====================

// FileType 文件类型.
type FileType string

const (
	FileTypeText   FileType = "text"   // 文本文件
	FileTypeImage  FileType = "image"  // 图片文件
	FileTypeCode   FileType = "code"   // 代码文件
	FileTypeBinary FileType = "binary" // 二进制文件
	FileTypeDir    FileType = "dir"    // 目录
	FileTypeLink   FileType = "link"   // 符号链接
)

// FileVersion 文件版本.
type FileVersion struct {
	ID          string    `json:"id"`
	SnapshotID  string    `json:"snapshot_id"` // 所属快照
	FilePath    string    `json:"file_path"`   // 文件相对路径
	FullPath    string    `json:"full_path"`   // 完整路径
	FileType    FileType  `json:"file_type"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
	Permissions string    `json:"permissions"`
	Owner       string    `json:"owner,omitempty"`
	Group       string    `json:"group,omitempty"`
	MD5         string    `json:"md5,omitempty"`
	SHA256      string    `json:"sha256,omitempty"`
	IsDeleted   bool      `json:"is_deleted"` // 在该快照中是否已删除
	IsLocked    bool      `json:"is_locked"`  // 是否被锁定
	CreatedAt   time.Time `json:"created_at"`
}

// ==================== 差异对比相关 ====================

// DiffType 差异类型.
type DiffType string

const (
	DiffAdded     DiffType = "added"     // 新增
	DiffDeleted   DiffType = "deleted"   // 删除
	DiffModified  DiffType = "modified"  // 修改
	DiffRenamed   DiffType = "renamed"   // 重命名
	DiffMoved     DiffType = "moved"     // 移动
	DiffUnchanged DiffType = "unchanged" // 未变更
)

// DiffHunk 差异片段.
type DiffHunk struct {
	OldStart int        `json:"old_start"` // 旧文件起始行
	OldLines int        `json:"old_lines"` // 旧文件行数
	NewStart int        `json:"new_start"` // 新文件起始行
	NewLines int        `json:"new_lines"` // 新文件行数
	Content  string     `json:"content"`   // 差异内容
	Lines    []DiffLine `json:"lines"`     // 逐行差异
}

// DiffLine 差异行.
type DiffLine struct {
	Type    DiffType `json:"type"`     // added/deleted/unchanged
	OldLine int      `json:"old_line"` // 旧文件行号（0 表示新增行）
	NewLine int      `json:"new_line"` // 新文件行号（0 表示删除行）
	Content string   `json:"content"`  // 行内容
}

// FileDiff 文件差异.
type FileDiff struct {
	FilePath   string     `json:"file_path"`
	OldPath    string     `json:"old_path,omitempty"` // 重命名时的旧路径
	NewPath    string     `json:"new_path,omitempty"` // 重命名时的新路径
	Type       DiffType   `json:"type"`
	FileType   FileType   `json:"file_type"`
	OldVersion string     `json:"old_version,omitempty"` // 旧版本ID
	NewVersion string     `json:"new_version,omitempty"` // 新版本ID
	OldSize    int64      `json:"old_size"`
	NewSize    int64      `json:"new_size"`
	Hunks      []DiffHunk `json:"hunks,omitempty"`      // 文本差异
	ImageDiff  *ImageDiff `json:"image_diff,omitempty"` // 图片差异
	BinaryDiff bool       `json:"binary_diff"`          // 是否二进制差异
	Additions  int        `json:"additions"`            // 新增行数
	Deletions  int        `json:"deletions"`            // 删除行数
}

// ImageDiff 图片差异信息.
type ImageDiff struct {
	OldWidth     int     `json:"old_width"`
	OldHeight    int     `json:"old_height"`
	NewWidth     int     `json:"new_width"`
	NewHeight    int     `json:"new_height"`
	OldFormat    string  `json:"old_format"`
	NewFormat    string  `json:"new_format"`
	DiffImageURL string  `json:"diff_image_url,omitempty"` // 差异热力图URL
	Similarity   float64 `json:"similarity"`               // 相似度（0-1）
}

// SnapshotDiff 快照间差异.
type SnapshotDiff struct {
	OldSnapshotID  string     `json:"old_snapshot_id"`
	NewSnapshotID  string     `json:"new_snapshot_id"`
	TotalFiles     int        `json:"total_files"`
	Added          int        `json:"added"`
	Deleted        int        `json:"deleted"`
	Modified       int        `json:"modified"`
	Renamed        int        `json:"renamed"`
	Unchanged      int        `json:"unchanged"`
	TotalAdditions int        `json:"total_additions"`
	TotalDeletions int        `json:"total_deletions"`
	Files          []FileDiff `json:"files"`
	GeneratedAt    time.Time  `json:"generated_at"`
}

// ==================== 回滚相关 ====================

// RollbackStatus 回滚状态.
type RollbackStatus string

const (
	RollbackPending   RollbackStatus = "pending"
	RollbackRunning   RollbackStatus = "running"
	RollbackCompleted RollbackStatus = "completed"
	RollbackFailed    RollbackStatus = "failed"
	RollbackCancelled RollbackStatus = "cancelled"
	RollbackPartial   RollbackStatus = "partial" // 部分成功
)

// RollbackMode 回滚模式.
type RollbackMode string

const (
	RollbackOverwrite RollbackMode = "overwrite" // 覆盖现有文件
	RollbackMerge     RollbackMode = "merge"     // 合并（保留新文件）
	RollbackSelective RollbackMode = "selective" // 选择性回滚
	RollbackDryRun    RollbackMode = "dry_run"   // 试运行（不实际修改）
)

// RollbackRequest 回滚请求.
type RollbackRequest struct {
	SnapshotID  string       `json:"snapshot_id" binding:"required"` // 目标快照
	TargetPath  string       `json:"target_path"`                    // 恢复目标路径（默认原路径）
	FilePaths   []string     `json:"file_paths,omitempty"`           // 指定文件（空则全部）
	Mode        RollbackMode `json:"mode"`                           // 回滚模式
	BackupFirst bool         `json:"backup_first"`                   // 回滚前先备份
	DryRun      bool         `json:"dry_run"`                        // 试运行
	Force       bool         `json:"force"`                          // 强制（跳过确认）
}

// RollbackResult 回滚结果.
type RollbackResult struct {
	ID             string          `json:"id"`
	SnapshotID     string          `json:"snapshot_id"`
	Status         RollbackStatus  `json:"status"`
	Mode           RollbackMode    `json:"mode"`
	TotalFiles     int             `json:"total_files"`
	RestoredFiles  int             `json:"restored_files"`
	FailedFiles    int             `json:"failed_files"`
	SkippedFiles   int             `json:"skipped_files"`
	BackupSnapshot string          `json:"backup_snapshot,omitempty"` // 备份快照ID
	Errors         []RollbackError `json:"errors,omitempty"`
	StartedAt      time.Time       `json:"started_at"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	Duration       time.Duration   `json:"duration"`
}

// RollbackError 回滚错误.
type RollbackError struct {
	FilePath string `json:"file_path"`
	Error    string `json:"error"`
}

// ==================== 回收站相关 ====================

// TrashStatus 回收站状态.
type TrashStatus string

const (
	TrashStatusActive   TrashStatus = "active"   // 在回收站中
	TrashStatusRestored TrashStatus = "restored" // 已恢复
	TrashStatusPurged   TrashStatus = "purged"   // 已彻底删除
)

// TrashItem 回收站项目.
type TrashItem struct {
	ID           string      `json:"id"`
	OriginalPath string      `json:"original_path"` // 原始路径
	TrashPath    string      `json:"trash_path"`    // 回收站路径
	FileName     string      `json:"file_name"`
	FileType     FileType    `json:"file_type"`
	Size         int64       `json:"size"`
	DeletedAt    time.Time   `json:"deleted_at"`
	DeletedBy    string      `json:"deleted_by,omitempty"`   // 删除者
	Source       string      `json:"source,omitempty"`       // 删除来源（命令行/web/API）
	RestorePath  string      `json:"restore_path,omitempty"` // 恢复路径（已恢复时）
	RestoredAt   *time.Time  `json:"restored_at,omitempty"`
	AutoPurgeAt  *time.Time  `json:"auto_purge_at,omitempty"` // 自动清除时间
	Status       TrashStatus `json:"status"`
	MD5          string      `json:"md5,omitempty"`
}

// ==================== 文件锁定相关 ====================

// LockType 锁类型.
type LockType string

const (
	LockTypeSoft LockType = "soft" // 软锁（可被管理员解锁）
	LockTypeHard LockType = "hard" // 硬锁（需特殊权限解锁）
	LockTypeAuto LockType = "auto" // 自动锁（快照自动锁定重要文件）
)

// FileLock 文件锁.
type FileLock struct {
	ID        string     `json:"id"`
	FilePath  string     `json:"file_path"`
	LockType  LockType   `json:"lock_type"`
	LockedBy  string     `json:"locked_by"` // 锁定者
	Reason    string     `json:"reason,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // 过期时间，nil 表示永不过期
	CreatedAt time.Time  `json:"created_at"`
}

// ==================== 存储分析相关 ====================

// StorageTier 存储层级.
type StorageTier string

const (
	TierHot  StorageTier = "hot"  // 热存储（频繁访问）
	TierWarm StorageTier = "warm" // 温存储（偶尔访问）
	TierCold StorageTier = "cold" // 冷存储（归档）
)

// StorageUsage 存储使用情况.
type StorageUsage struct {
	TotalSpace     int64                 `json:"total_space"`     // 总空间（字节）
	UsedSpace      int64                 `json:"used_space"`      // 已用空间
	AvailableSpace int64                 `json:"available_space"` // 可用空间
	SnapshotSpace  int64                 `json:"snapshot_space"`  // 快照占用
	TrashSpace     int64                 `json:"trash_space"`     // 回收站占用
	ActiveSpace    int64                 `json:"active_space"`    // 活跃文件占用
	UsagePercent   float64               `json:"usage_percent"`   // 使用率
	ByTier         map[StorageTier]int64 `json:"by_tier"`         // 按层级分布
	ByPath         map[string]int64      `json:"by_path"`         // 按路径分布
	ByPolicy       map[string]int64      `json:"by_policy"`       // 按策略分布
	UpdatedAt      time.Time             `json:"updated_at"`
}

// SnapshotStorageInfo 快照存储信息.
type SnapshotStorageInfo struct {
	SnapshotID    string      `json:"snapshot_id"`
	SnapshotName  string      `json:"snapshot_name"`
	TotalSize     int64       `json:"total_size"`
	DeltaSize     int64       `json:"delta_size"`     // 增量大小
	Deduplication int64       `json:"deduplication"`  // 去重节省空间
	Compression   int64       `json:"compression"`    // 压缩节省空间
	EffectiveSize int64       `json:"effective_size"` // 实际占用
	Tier          StorageTier `json:"tier"`
	CreatedAt     time.Time   `json:"created_at"`
}

// ==================== 通用请求/响应 ====================

// ListOptions 列表查询选项.
type ListOptions struct {
	Page      int    `json:"page" form:"page"`
	PageSize  int    `json:"page_size" form:"page_size"`
	SortBy    string `json:"sort_by" form:"sort_by"`
	SortOrder string `json:"sort_order" form:"sort_order"` // asc/desc
	Search    string `json:"search" form:"search"`
	Status    string `json:"status" form:"status"`
	StartDate string `json:"start_date" form:"start_date"`
	EndDate   string `json:"end_date" form:"end_date"`
}

// PageResult 分页结果.
type PageResult struct {
	Items      interface{} `json:"items"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// HealthStatus 健康状态.
type HealthStatus struct {
	Status          string    `json:"status"` // healthy/degraded/unhealthy
	ActiveSnapshots int       `json:"active_snapshots"`
	TotalSnapshots  int       `json:"total_snapshots"`
	TrashItems      int       `json:"trash_items"`
	LockedFiles     int       `json:"locked_files"`
	StorageUsage    int64     `json:"storage_usage"`
	LastSnapshot    time.Time `json:"last_snapshot"`
	NextScheduled   time.Time `json:"next_scheduled"`
	Errors          []string  `json:"errors,omitempty"`
	CheckedAt       time.Time `json:"checked_at"`
}

// ==================== 配置 ====================

// FileTimeMachineConfig 文件时光机配置.
type FileTimeMachineConfig struct {
	Enabled              bool     `json:"enabled"`
	StorageRoot          string   `json:"storage_root"`                 // 快照存储根目录
	TrashRoot            string   `json:"trash_root"`                   // 回收站根目录
	MaxSnapshots         int      `json:"max_snapshots"`                // 最大快照数
	MaxTrashDays         int      `json:"max_trash_days"`               // 回收站保留天数
	CompressionEnabled   bool     `json:"compression_enabled"`          // 启用压缩
	DeduplicationEnabled bool     `json:"deduplication_enabled"`        // 启用去重
	AutoLockImportant    bool     `json:"auto_lock_important"`          // 自动锁定重要文件
	ImportantPatterns    []string `json:"important_patterns"`           // 重要文件模式
	ExcludePatterns      []string `json:"exclude_patterns"`             // 全局排除模式
	TrashAutoPurgeDays   int      `json:"trash_auto_purge_days"`        // 回收站自动清除天数
	MaxConcurrentOps     int      `json:"max_concurrent_ops"`           // 最大并发操作数
	NotificationEmail    string   `json:"notification_email,omitempty"` // 通知邮箱
}

// DefaultConfig 默认配置.
func DefaultConfig() *FileTimeMachineConfig {
	return &FileTimeMachineConfig{
		Enabled:              true,
		StorageRoot:          "/data/snapshots",
		TrashRoot:            "/data/trash",
		MaxSnapshots:         100,
		MaxTrashDays:         30,
		CompressionEnabled:   true,
		DeduplicationEnabled: true,
		AutoLockImportant:    true,
		ImportantPatterns: []string{
			"*.doc", "*.docx", "*.pdf", "*.xls", "*.xlsx",
			"*.ppt", "*.pptx", "*.key", "*.numbers", "*.pages",
			"/etc/*", "/var/lib/*",
		},
		ExcludePatterns: []string{
			"*.tmp", "*.temp", "*.cache", "*.log",
			".git/objects/*", "node_modules/*", "__pycache__/*",
			"/proc/*", "/sys/*", "/dev/*",
		},
		TrashAutoPurgeDays: 30,
		MaxConcurrentOps:   5,
	}
}
