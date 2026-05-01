// Package drivesync 提供增强版文件同步功能，对标群晖 Synology Drive
package drivesync

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

// 同步相关错误.
var (
	// ErrSyncTaskNotFound 同步任务不存在.
	ErrSyncTaskNotFound = errors.New("同步任务不存在")
	// ErrSyncTaskExists 同步任务已存在.
	ErrSyncTaskExists = errors.New("同步任务已存在")
	// ErrSyncTaskRunning 同步任务正在运行.
	ErrSyncTaskRunning = errors.New("同步任务正在运行")
	// ErrSyncTaskNotRunning 同步任务未在运行.
	ErrSyncTaskNotRunning = errors.New("同步任务未在运行")
	// ErrFileVersionNotFound 文件版本不存在.
	ErrFileVersionNotFound = errors.New("文件版本不存在")
	// ErrConflictNotFound 冲突不存在.
	ErrConflictNotFound = errors.New("冲突不存在")
	// ErrFileLocked 文件已被锁定.
	ErrFileLocked = errors.New("文件已被锁定")
	// ErrFileNotLocked 文件未被锁定.
	ErrFileNotLocked = errors.New("文件未被锁定")
	// ErrInvalidPath 无效路径.
	ErrInvalidPath = errors.New("无效路径")
	// ErrInvalidSyncDirection 无效的同步方向.
	ErrInvalidSyncDirection = errors.New("无效的同步方向")
	// ErrLockExpired 锁已过期.
	ErrLockExpired = errors.New("锁已过期")
)

// ========== 同步方向 ==========

// SyncDirection 同步方向.
type SyncDirection string

const (
	// SyncBidirectional 双向同步.
	SyncBidirectional SyncDirection = "bidirectional"
	// SyncUploadOnly 仅上传（本地到远程）.
	SyncUploadOnly SyncDirection = "upload_only"
	// SyncDownloadOnly 仅下载（远程到本地）.
	SyncDownloadOnly SyncDirection = "download_only"
	// SyncMirror 镜像同步（远程完全镜像本地）.
	SyncMirror SyncDirection = "mirror"
)

// ========== 同步状态 ==========

// SyncTaskStatus 同步任务状态.
type SyncTaskStatus string

const (
	// TaskStatusIdle 空闲.
	TaskStatusIdle SyncTaskStatus = "idle"
	// TaskStatusSyncing 同步中.
	TaskStatusSyncing SyncTaskStatus = "syncing"
	// TaskStatusPaused 已暂停.
	TaskStatusPaused SyncTaskStatus = "paused"
	// TaskStatusError 错误.
	TaskStatusError SyncTaskStatus = "error"
	// TaskStatusConflict 有冲突.
	TaskStatusConflict SyncTaskStatus = "conflict"
	// TaskStatusCompleted 已完成.
	TaskStatusCompleted SyncTaskStatus = "completed"
)

// FileSyncStatus 文件同步状态.
type FileSyncStatus string

const (
	// FileStatusSynced 已同步.
	FileStatusSynced FileSyncStatus = "synced"
	// FileStatusPending 待同步.
	FileStatusPending FileSyncStatus = "pending"
	// FileStatusSyncing 同步中.
	FileStatusSyncing FileSyncStatus = "syncing"
	// FileStatusConflict 冲突.
	FileStatusConflict FileSyncStatus = "conflict"
	// FileStatusError 错误.
	FileStatusError FileSyncStatus = "error"
	// FileStatusExcluded 已排除.
	FileStatusExcluded FileSyncStatus = "excluded"
)

// ========== 冲突解决策略 ==========

// ConflictResolution 冲突解决策略.
type ConflictResolution string

const (
	// ConflictKeepLocal 保留本地版本.
	ConflictKeepLocal ConflictResolution = "keep_local"
	// ConflictKeepRemote 保留远程版本.
	ConflictKeepRemote ConflictResolution = "keep_remote"
	// ConflictKeepBoth 保留双方版本.
	ConflictKeepBoth ConflictResolution = "keep_both"
	// ConflictManualMerge 手动合并.
	ConflictManualMerge ConflictResolution = "manual_merge"
	// ConflictNewerWins 保留较新版本.
	ConflictNewerWins ConflictResolution = "newer_wins"
)

// ConflictStatus 冲突状态.
type ConflictStatus string

const (
	// ConflictStatusPending 待解决.
	ConflictStatusPending ConflictStatus = "pending"
	// ConflictStatusResolved 已解决.
	ConflictStatusResolved ConflictStatus = "resolved"
	// ConflictStatusIgnored 已忽略.
	ConflictStatusIgnored ConflictStatus = "ignored"
)

// ========== 活动类型 ==========

// ActivityType 活动类型.
type ActivityType string

const (
	// ActivityFileCreated 文件创建.
	ActivityFileCreated ActivityType = "file_created"
	// ActivityFileModified 文件修改.
	ActivityFileModified ActivityType = "file_modified"
	// ActivityFileDeleted 文件删除.
	ActivityFileDeleted ActivityType = "file_deleted"
	// ActivityFileRenamed 文件重命名.
	ActivityFileRenamed ActivityType = "file_renamed"
	// ActivityFileRestored 文件恢复.
	ActivityFileRestored ActivityType = "file_restored"
	// ActivityVersionCreated 创建版本.
	ActivityVersionCreated ActivityType = "version_created"
	// ActivityConflictDetected 冲突检测.
	ActivityConflictDetected ActivityType = "conflict_detected"
	// ActivityConflictResolved 冲突解决.
	ActivityConflictResolved ActivityType = "conflict_resolved"
	// ActivityLockAcquired 获取锁.
	ActivityLockAcquired ActivityType = "lock_acquired"
	// ActivityLockReleased 释放锁.
	ActivityLockReleased ActivityType = "lock_released"
	// ActivityCommentAdded 添加评论.
	ActivityCommentAdded ActivityType = "comment_added"
	// ActivityMentionAdded 添加提及.
	ActivityMentionAdded ActivityType = "mention_added"
	// ActivitySyncStarted 同步开始.
	ActivitySyncStarted ActivityType = "sync_started"
	// ActivitySyncCompleted 同步完成.
	ActivitySyncCompleted ActivityType = "sync_completed"
	// ActivitySyncFailed 同步失败.
	ActivitySyncFailed ActivityType = "sync_failed"
)

// ========== 同步任务 ==========

// SyncTask 同步任务.
type SyncTask struct {
	ID              string           `json:"id"`                          // 任务唯一标识
	Name            string           `json:"name"`                        // 任务名称
	LocalPath       string           `json:"local_path"`                  // 本地路径
	RemotePath      string           `json:"remote_path"`                 // 远程路径
	DeviceID        string           `json:"device_id"`                   // 设备标识
	Direction       SyncDirection    `json:"direction"`                   // 同步方向
	ConflictPolicy  ConflictResolution `json:"conflict_policy"`          // 冲突解决策略
	Status          SyncTaskStatus   `json:"status"`                      // 当前状态
	Enabled         bool             `json:"enabled"`                     // 是否启用
	Interval        time.Duration    `json:"interval"`                    // 自动同步间隔
	ExcludePatterns []string         `json:"exclude_patterns,omitempty"`  // 排除模式
	IncludePatterns []string         `json:"include_patterns,omitempty"`  // 包含模式
	BandwidthLimit  int64            `json:"bandwidth_limit,omitempty"`   // 带宽限制（字节/秒）
	LastSyncAt      *time.Time       `json:"last_sync_at,omitempty"`      // 上次同步时间
	NextSyncAt      *time.Time       `json:"next_sync_at,omitempty"`      // 下次同步时间
	FileCount       int              `json:"file_count"`                  // 同步文件数
	SyncedBytes     int64            `json:"synced_bytes"`                // 已同步字节数
	ErrorCount      int              `json:"error_count"`                 // 错误数
	LastError       string           `json:"last_error,omitempty"`        // 最近错误
	CreatedAt       time.Time        `json:"created_at"`                  // 创建时间
	UpdatedAt       time.Time        `json:"updated_at"`                  // 更新时间
}

// SyncTaskInput 创建/更新同步任务的输入.
type SyncTaskInput struct {
	Name            string           `json:"name" binding:"required"`
	LocalPath       string           `json:"local_path" binding:"required"`
	RemotePath      string           `json:"remote_path" binding:"required"`
	DeviceID        string           `json:"device_id"`
	Direction       SyncDirection    `json:"direction"`
	ConflictPolicy  ConflictResolution `json:"conflict_policy"`
	Enabled         bool             `json:"enabled"`
	Interval        time.Duration    `json:"interval"`
	ExcludePatterns []string         `json:"exclude_patterns"`
	IncludePatterns []string         `json:"include_patterns"`
	BandwidthLimit  int64            `json:"bandwidth_limit"`
}

// ========== 文件版本 ==========

// FileVersion 文件版本.
type FileVersion struct {
	ID          string    `json:"id"`                    // 版本唯一标识
	FilePath    string    `json:"file_path"`             // 文件路径
	VersionNum  int       `json:"version_num"`           // 版本号
	Size        int64     `json:"size"`                  // 文件大小
	Checksum    string    `json:"checksum"`              // SHA256校验和
	StoragePath string    `json:"storage_path"`          // 版本存储路径
	Label       string    `json:"label,omitempty"`       // 版本标签
	Comment     string    `json:"comment,omitempty"`     // 版本注释
	DeviceID    string    `json:"device_id,omitempty"`   // 来源设备
	CreatedBy   string    `json:"created_by,omitempty"`  // 创建者
	CreatedAt   time.Time `json:"created_at"`            // 创建时间
	ExpiresAt   *time.Time `json:"expires_at,omitempty"` // 过期时间
}

// VersionConfig 版本控制配置.
type VersionConfig struct {
	Enabled       bool `json:"enabled"`        // 是否启用版本历史
	MaxVersions   int  `json:"max_versions"`   // 最大保留版本数
	RetentionDays int  `json:"retention_days"` // 版本保留天数（默认30）
	MaxTotalSize  int64 `json:"max_total_size"` // 版本存储总量上限（字节）
}

// VersionDiff 版本差异.
type VersionDiff struct {
	FromVersion  string       `json:"from_version"`  // 源版本ID
	ToVersion    string       `json:"to_version"`    // 目标版本ID
	FromChecksum string       `json:"from_checksum"` // 源版本校验和
	ToChecksum   string       `json:"to_checksum"`   // 目标版本校验和
	Added        int          `json:"added"`         // 新增行数
	Removed      int          `json:"removed"`       // 删除行数
	Modified     int          `json:"modified"`      // 修改行数
	IsBinary     bool         `json:"is_binary"`     // 是否为二进制文件
	Similarity   float64      `json:"similarity"`    // 相似度（0-1）
	Details      []DiffDetail `json:"details,omitempty"` // 详细差异
}

// DiffDetail 差异详情.
type DiffDetail struct {
	Type     string `json:"type"`     // "added", "removed", "modified"
	LineFrom int    `json:"line_from,omitempty"` // 起始行号
	LineTo   int    `json:"line_to,omitempty"`   // 结束行号
	Content  string `json:"content"`  // 内容
}

// ========== 冲突 ==========

// FileConflict 文件冲突.
type FileConflict struct {
	ID             string           `json:"id"`               // 冲突唯一标识
	TaskID         string           `json:"task_id"`          // 关联的同步任务ID
	FilePath       string           `json:"file_path"`        // 文件路径
	LocalChecksum  string           `json:"local_checksum"`   // 本地文件校验和
	RemoteChecksum string           `json:"remote_checksum"`  // 远程文件校验和
	LocalModTime   time.Time        `json:"local_mod_time"`   // 本地修改时间
	RemoteModTime  time.Time        `json:"remote_mod_time"`  // 远程修改时间
	LocalSize      int64            `json:"local_size"`       // 本地文件大小
	RemoteSize     int64            `json:"remote_size"`      // 远程文件大小
	LocalDeviceID  string           `json:"local_device_id"`  // 本地设备ID
	RemoteDeviceID string           `json:"remote_device_id"` // 远程设备ID
	Resolution     ConflictResolution `json:"resolution"`     // 解决策略
	Status         ConflictStatus   `json:"status"`           // 冲突状态
	ResolvedBy     string           `json:"resolved_by,omitempty"` // 解决者
	ResolvedAt     *time.Time       `json:"resolved_at,omitempty"` // 解决时间
	RenamedPath    string           `json:"renamed_path,omitempty"` // 重命名后的路径
	CreatedAt      time.Time        `json:"created_at"`       // 创建时间
}

// ConflictResolveInput 冲突解决输入.
type ConflictResolveInput struct {
	Resolution ConflictResolution `json:"resolution" binding:"required"` // 解决策略
	Comment    string             `json:"comment"`                       // 备注
}

// ========== 文件锁 ==========

// FileLock 文件锁.
type FileLock struct {
	ID        string    `json:"id"`          // 锁唯一标识
	FilePath  string    `json:"file_path"`   // 文件路径
	LockedBy  string    `json:"locked_by"`   // 锁定者（用户ID或设备ID）
	LockType  string    `json:"lock_type"`   // 锁类型："exclusive"（独占）或 "shared"（共享）
	Reason    string    `json:"reason"`      // 锁定原因
	ExpiresAt time.Time `json:"expires_at"`  // 过期时间
	CreatedAt time.Time `json:"created_at"`  // 创建时间
}

// FileLockInput 锁定文件输入.
type FileLockInput struct {
	LockedBy string `json:"locked_by" binding:"required"` // 锁定者
	LockType string `json:"lock_type"`                    // 锁类型，默认独占
	Reason   string `json:"reason"`                       // 锁定原因
	Duration int    `json:"duration"`                     // 锁定时长（分钟），默认30
}

// ========== 协作 ==========

// Comment 文件评论.
type Comment struct {
	ID        string    `json:"id"`          // 评论唯一标识
	FilePath  string    `json:"file_path"`   // 文件路径
	UserID    string    `json:"user_id"`     // 评论者ID
	UserName  string    `json:"user_name"`   // 评论者名称
	Content   string    `json:"content"`     // 评论内容
	Mentions  []string  `json:"mentions"`    // @提及的用户ID列表
	VersionID string    `json:"version_id"`  // 关联的版本ID
	CreatedAt time.Time `json:"created_at"`  // 创建时间
	UpdatedAt time.Time `json:"updated_at"`  // 更新时间
}

// CommentInput 创建评论输入.
type CommentInput struct {
	UserID   string   `json:"user_id" binding:"required"` // 评论者ID
	UserName string   `json:"user_name" binding:"required"` // 评论者名称
	Content  string   `json:"content" binding:"required"`  // 评论内容
	Mentions []string `json:"mentions"`                    // @提及的用户ID列表
}

// Activity 活动记录.
type Activity struct {
	ID        string       `json:"id"`         // 活动唯一标识
	Type      ActivityType `json:"type"`       // 活动类型
	FilePath  string       `json:"file_path"`  // 关联文件路径
	UserID    string       `json:"user_id"`    // 操作者ID
	UserName  string       `json:"user_name"`  // 操作者名称
	TaskID    string       `json:"task_id"`    // 关联任务ID
	Details   string       `json:"details"`    // 详细信息
	DeviceID  string       `json:"device_id"`  // 设备标识
	CreatedAt time.Time    `json:"created_at"` // 创建时间
}

// ========== 同步统计 ==========

// SyncStats 同步统计信息.
type SyncStats struct {
	TotalTasks      int       `json:"total_tasks"`       // 总任务数
	ActiveTasks     int       `json:"active_tasks"`      // 活跃任务数
	PausedTasks     int       `json:"paused_tasks"`      // 暂停任务数
	ErrorTasks      int       `json:"error_tasks"`       // 错误任务数
	TotalFiles      int       `json:"total_files"`       // 总文件数
	SyncedFiles     int       `json:"synced_files"`      // 已同步文件数
	ConflictFiles   int       `json:"conflict_files"`    // 冲突文件数
	PendingFiles    int       `json:"pending_files"`     // 待同步文件数
	TotalBytes      int64     `json:"total_bytes"`       // 总字节数
	SyncedBytes     int64     `json:"synced_bytes"`      // 已同步字节数
	TotalVersions   int       `json:"total_versions"`    // 版本总数
	ActiveLocks     int       `json:"active_locks"`      // 活跃锁数
	PendingConflicts int      `json:"pending_conflicts"` // 待解决冲突数
	LastSyncAt      *time.Time `json:"last_sync_at"`     // 最近同步时间
	Uptime          time.Duration `json:"uptime"`        // 运行时长
}

// FileInfo 文件信息（含版本历史）.
type FileInfo struct {
	Path         string        `json:"path"`          // 文件路径
	Size         int64         `json:"size"`          // 文件大小
	IsDir        bool          `json:"is_dir"`        // 是否为目录
	Checksum     string        `json:"checksum"`      // SHA256校验和
	ModTime      time.Time     `json:"mod_time"`      // 修改时间
	SyncStatus   FileSyncStatus `json:"sync_status"`  // 同步状态
	TaskID       string        `json:"task_id"`       // 关联任务ID
	Versions     []*FileVersion `json:"versions"`     // 版本历史
	Lock         *FileLock     `json:"lock,omitempty"` // 当前锁
	Comments     []*Comment    `json:"comments"`      // 评论列表
	IsConflicted bool          `json:"is_conflicted"` // 是否有冲突
}

// BlockInfo 块信息（用于增量同步）.
type BlockInfo struct {
	Index    int    `json:"index"`    // 块索引
	Offset   int64  `json:"offset"`   // 偏移量
	Size     int    `json:"size"`     // 块大小
	Checksum string `json:"checksum"` // 块校验和（弱校验+强校验）
}

// DeltaSyncRequest 增量同步请求.
type DeltaSyncRequest struct {
	FilePath     string       `json:"file_path"`     // 文件路径
	LocalSize    int64        `json:"local_size"`    // 本地文件大小
	LocalChecksum string      `json:"local_checksum"` // 本地文件校验和
	LocalBlocks  []BlockInfo  `json:"local_blocks"`  // 本地文件块信息
}

// DeltaSyncResponse 增量同步响应.
type DeltaSyncResponse struct {
	NeedFullSync bool         `json:"need_full_sync"` // 是否需要全量同步
	MatchedBlocks []int       `json:"matched_blocks"` // 匹配的块索引
	NewBlocks    []BlockInfo  `json:"new_blocks"`     // 需要传输的新块
	TotalBlocks  int          `json:"total_blocks"`   // 总块数
	SavedBytes   int64        `json:"saved_bytes"`    // 节省的传输字节数
}

// WebSocketMessage WebSocket 推送消息.
type WebSocketMessage struct {
	Type    string      `json:"type"`    // 消息类型："sync_update", "conflict", "file_change", "lock_change"
	Payload interface{} `json:"payload"` // 消息负载
	Time    time.Time   `json:"time"`    // 消息时间
}
