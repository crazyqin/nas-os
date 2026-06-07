// Package filesync 提供文件同步服务功能，对标群晖 Synology Drive，包括实时同步、冲突解决、选择性同步、版本追踪等。
package filesync

import (
	"time"

	"go.uber.org/zap"
)

// SyncStatus 同步状态
type SyncStatus string

const (
	SyncStatusIdle     SyncStatus = "idle"
	SyncStatusSyncing  SyncStatus = "syncing"
	SyncStatusPaused   SyncStatus = "paused"
	SyncStatusError    SyncStatus = "error"
	SyncStatusConflict SyncStatus = "conflict"
)

// SyncDirection 同步方向
type SyncDirection string

const (
	DirectionUpload   SyncDirection = "upload"
	DirectionDownload SyncDirection = "download"
	DirectionBoth     SyncDirection = "bidirectional"
)

// ConflictResolution 冲突解决策略
type ConflictResolution string

const (
	ConflictKeepLocal     ConflictResolution = "keep_local"
	ConflictKeepRemote    ConflictResolution = "keep_remote"
	ConflictKeepBoth      ConflictResolution = "keep_both"
	ConflictManualResolve ConflictResolution = "manual"
	ConflictNewerWins     ConflictResolution = "newer_wins"
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceOnline  DeviceStatus = "online"
	DeviceOffline DeviceStatus = "offline"
	DeviceSyncing DeviceStatus = "syncing"
)

// FileAction 文件操作类型
type FileAction string

const (
	ActionCreate FileAction = "create"
	ActionModify FileAction = "modify"
	ActionDelete FileAction = "delete"
	ActionMove   FileAction = "move"
	ActionRename FileAction = "rename"
)

// TransferStatus 传输状态
type TransferStatus string

const (
	TransferPending   TransferStatus = "pending"
	TransferActive    TransferStatus = "active"
	TransferPaused    TransferStatus = "paused"
	TransferCompleted TransferStatus = "completed"
	TransferFailed    TransferStatus = "failed"
)

// Device 同步设备
type Device struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Platform   string       `json:"platform"`
	IP         string       `json:"ip,omitempty"`
	Status     DeviceStatus `json:"status"`
	Version    string       `json:"version,omitempty"`
	LastSeen   time.Time    `json:"last_seen"`
	SyncCount  int          `json:"sync_count"`
	TotalSize  int64        `json:"total_size"`
	QuotaUsed  int64        `json:"quota_used"`
	QuotaLimit int64        `json:"quota_limit"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// SyncFolder 同步文件夹配置
type SyncFolder struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	LocalPath       string             `json:"local_path"`
	RemotePath      string             `json:"remote_path"`
	Direction       SyncDirection      `json:"direction"`
	Enabled         bool               `json:"enabled"`
	ConflictPolicy  ConflictResolution `json:"conflict_policy"`
	FilterPatterns  []string           `json:"filter_patterns,omitempty"`
	ExcludePatterns []string           `json:"exclude_patterns,omitempty"`
	FileCount       int                `json:"file_count"`
	TotalSize       int64              `json:"total_size"`
	LastSyncAt      *time.Time         `json:"last_sync_at,omitempty"`
	SyncedCount     int                `json:"synced_count"`
	DeviceIDs       []string           `json:"device_ids,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

// SyncTask 同步任务
type SyncTask struct {
	ID          string         `json:"id"`
	FolderID    string         `json:"folder_id"`
	DeviceID    string         `json:"device_id"`
	Status      TransferStatus `json:"status"`
	Direction   SyncDirection  `json:"direction"`
	FilePath    string         `json:"file_path"`
	FileSize    int64          `json:"file_size"`
	Transferred int64          `json:"transferred"`
	Speed       int64          `json:"speed"`
	Error       string         `json:"error,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// SyncConflict 同步冲突
type SyncConflict struct {
	ID            string             `json:"id"`
	FolderID      string             `json:"folder_id"`
	DeviceID      string             `json:"device_id"`
	FilePath      string             `json:"file_path"`
	LocalVersion  FileVersion        `json:"local_version"`
	RemoteVersion FileVersion        `json:"remote_version"`
	Resolution    ConflictResolution `json:"resolution,omitempty"`
	Resolved      bool               `json:"resolved"`
	ResolvedBy    string             `json:"resolved_by,omitempty"`
	ResolvedAt    *time.Time         `json:"resolved_at,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
}

// FileVersion 文件版本
type FileVersion struct {
	Version  int        `json:"version"`
	Size     int64      `json:"size"`
	Checksum string     `json:"checksum,omitempty"`
	Modified time.Time  `json:"modified"`
	DeviceID string     `json:"device_id"`
	Action   FileAction `json:"action"`
}

// FileHistory 文件历史版本
type FileHistory struct {
	ID        string     `json:"id"`
	FolderID  string     `json:"folder_id"`
	FilePath  string     `json:"file_path"`
	Version   int        `json:"version"`
	Size      int64      `json:"size"`
	Checksum  string     `json:"checksum,omitempty"`
	Action    FileAction `json:"action"`
	DeviceID  string     `json:"device_id"`
	Message   string     `json:"message,omitempty"`
	Restored  bool       `json:"restored"`
	CreatedAt time.Time  `json:"created_at"`
}

// SyncEngine 同步引擎状态
type SyncEngine struct {
	Status        SyncStatus `json:"status"`
	ActiveTasks   int        `json:"active_tasks"`
	TotalSynced   int64      `json:"total_synced"`
	TotalErrors   int        `json:"total_errors"`
	BandwidthUp   int64      `json:"bandwidth_up"`   // bytes/sec
	BandwidthDown int64      `json:"bandwidth_down"` // bytes/sec
	LastSyncAt    *time.Time `json:"last_sync_at,omitempty"`
	Uptime        int64      `json:"uptime"` // seconds
}

// TransferInfo 断点续传信息
type TransferInfo struct {
	TaskID      string `json:"task_id"`
	FilePath    string `json:"file_path"`
	FileSize    int64  `json:"file_size"`
	Offset      int64  `json:"offset"`
	ChunkSize   int64  `json:"chunk_size"`
	TotalChunks int    `json:"total_chunks"`
	DoneChunks  int    `json:"done_chunks"`
	Resumable   bool   `json:"resumable"`
	ETag        string `json:"etag,omitempty"`
}

// BandwidthLimit 带宽限制配置
type BandwidthLimit struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	UploadLimit   int64     `json:"upload_limit"`   // bytes/sec, 0=unlimited
	DownloadLimit int64     `json:"download_limit"` // bytes/sec, 0=unlimited
	ScheduleStart string    `json:"schedule_start"` // HH:MM
	ScheduleEnd   string    `json:"schedule_end"`   // HH:MM
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SyncStats 同步统计
type SyncStats struct {
	TotalFolders        int            `json:"total_folders"`
	TotalDevices        int            `json:"total_devices"`
	TotalFiles          int            `json:"total_files"`
	TotalSize           int64          `json:"total_size"`
	ActiveTasks         int            `json:"active_tasks"`
	PendingTasks        int            `json:"pending_tasks"`
	FailedTasks         int            `json:"failed_tasks"`
	TotalConflicts      int            `json:"total_conflicts"`
	UnresolvedConflicts int            `json:"unresolved_conflicts"`
	TotalVersions       int            `json:"total_versions"`
	SyncSpeed           int64          `json:"sync_speed"`
	EngineStatus        SyncStatus     `json:"engine_status"`
	Uptime              int64          `json:"uptime"`
	RecentConflicts     []SyncConflict `json:"recent_conflicts,omitempty"`
	RecentHistory       []FileHistory  `json:"recent_history,omitempty"`
}

// SelectiveSyncRule 选择性同步规则
type SelectiveSyncRule struct {
	ID          string    `json:"id"`
	FolderID    string    `json:"folder_id"`
	DeviceID    string    `json:"device_id"`
	PathPattern string    `json:"path_pattern"`
	Enabled     bool      `json:"enabled"`
	Type        string    `json:"type"` // "include" or "exclude"
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SyncStartRequest 启动同步请求
type SyncStartRequest struct {
	FolderID string `json:"folder_id,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
	Force    bool   `json:"force"`
}

// SyncStopRequest 停止同步请求
type SyncStopRequest struct {
	FolderID string `json:"folder_id,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
}

// ConflictResolveRequest 冲突解决请求
type ConflictResolveRequest struct {
	ConflictID string             `json:"conflict_id" binding:"required"`
	Resolution ConflictResolution `json:"resolution" binding:"required"`
}

// DeviceRegisterRequest 设备注册请求
type DeviceRegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Platform string `json:"platform" binding:"required"`
	IP       string `json:"ip,omitempty"`
	Version  string `json:"version,omitempty"`
}

// FolderCreateRequest 创建同步文件夹请求
type FolderCreateRequest struct {
	Name            string             `json:"name" binding:"required"`
	LocalPath       string             `json:"local_path" binding:"required"`
	RemotePath      string             `json:"remote_path" binding:"required"`
	Direction       SyncDirection      `json:"direction"`
	ConflictPolicy  ConflictResolution `json:"conflict_policy"`
	FilterPatterns  []string           `json:"filter_patterns,omitempty"`
	ExcludePatterns []string           `json:"exclude_patterns,omitempty"`
	DeviceIDs       []string           `json:"device_ids,omitempty"`
}

// SyncManager 文件同步管理器（兼容 web/server.go 调用）
type SyncManager struct {
	*Manager
	path string
}

// NewSyncManager 创建文件同步管理器
func NewSyncManager(_ *zap.Logger, path string) *SyncManager {
	return &SyncManager{
		Manager: NewManager(),
		path:    path,
	}
}

// HistoryRequest 历史查询请求
type HistoryRequest struct {
	FolderID string `json:"folder_id,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}
