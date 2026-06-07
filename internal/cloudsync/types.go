// Package cloudsync provides cloud synchronization management for NAS-OS.
// Supports multiple cloud storage backends with bidirectional/unidirectional sync,
// file filtering, conflict resolution, encryption, and bandwidth limiting.
// Reference: Synology Cloud Sync / TrueNAS CloudSync
package cloudsync

import (
	"fmt"
	"time"
)

// ============================================================
// 文件信息
// ============================================================

// FileInfo 云存储文件信息
// 用于 Provider 接口的 List 和 Stat 返回值
type FileInfo struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	IsDir   bool      `json:"is_dir"`
	Hash    string    `json:"hash,omitempty"`    // 文件哈希值
	Version string    `json:"version,omitempty"` // 版本号
}

// ============================================================
// Provider 类型
// ============================================================

// ProviderType 云存储提供商类型
type ProviderType string

const (
	ProviderS3           ProviderType = "s3"
	ProviderS3Compatible ProviderType = "s3_compatible"
	ProviderAliyunOSS    ProviderType = "aliyun_oss"
	ProviderTencentCOS   ProviderType = "tencent_cos"
	ProviderAWSS3        ProviderType = "aws_s3"
	ProviderBackblazeB2  ProviderType = "backblaze_b2"
	ProviderWebDAV       ProviderType = "webdav"
	ProviderGoogleDrive  ProviderType = "google_drive"
	ProviderOneDrive     ProviderType = "onedrive"
	ProviderDropbox      ProviderType = "dropbox"
	Provider115          ProviderType = "115"
	ProviderQuark        ProviderType = "quark"
	ProviderBaidu        ProviderType = "baidu"
	ProviderBaiduPan     ProviderType = "baidu_pan"
	ProviderAliyunPan    ProviderType = "aliyun_pan"
)

// ProviderConfig 云存储提供商配置
type ProviderConfig struct {
	ID           string       `json:"id,omitempty"`   // 配置 ID
	Name         string       `json:"name,omitempty"` // 配置名称
	Provider     ProviderType `json:"provider"`
	Type         ProviderType `json:"type,omitempty"` // 别名
	Endpoint     string       `json:"endpoint,omitempty"`
	Region       string       `json:"region,omitempty"`
	Bucket       string       `json:"bucket,omitempty"`
	AccessKey    string       `json:"access_key"`
	SecretKey    string       `json:"secret_key"`
	PathStyle    bool         `json:"path_style,omitempty"`
	Insecure     bool         `json:"insecure,omitempty"` // 跳过 TLS 验证
	Timeout      int          `json:"timeout,omitempty"`  // 超时秒数
	AccessToken  string       `json:"access_token,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	UserID       string       `json:"user_id,omitempty"`
	DriveID      string       `json:"drive_id,omitempty"`
	ClientID     string       `json:"client_id,omitempty"`      // OAuth2 Client ID
	ClientSecret string       `json:"client_secret,omitempty"`  // OAuth2 Client Secret
	RootFolderID string       `json:"root_folder_id,omitempty"` // 根文件夹 ID
	TenantID     string       `json:"tenant_id,omitempty"`      // 租户 ID (Azure等)
	Enabled      bool         `json:"enabled,omitempty"`        // 是否启用
}

// SyncOpType 同步操作类型
type SyncOpType string

const (
	SyncOpSkip         SyncOpType = "skip"
	SyncOpUpload       SyncOpType = "upload"
	SyncOpDownload     SyncOpType = "download"
	SyncOpDelete       SyncOpType = "delete"
	SyncOpConflict     SyncOpType = "conflict"
	SyncOpDeleteLocal  SyncOpType = "delete_local"
	SyncOpDeleteRemote SyncOpType = "delete_remote"
)

// SyncOperation 同步操作
type SyncOperation struct {
	Type       SyncOpType `json:"type"`
	LocalPath  string     `json:"local_path"`
	RemotePath string     `json:"remote_path"`
	Size       int64      `json:"size"`
	ModTime    time.Time  `json:"mod_time"`
	Hash       string     `json:"hash,omitempty"`
}

// ConflictStrategy 冲突解决策略
type ConflictStrategy string

const (
	ConflictStrategySkip   ConflictStrategy = "skip"
	ConflictStrategyLocal  ConflictStrategy = "local"
	ConflictStrategyRemote ConflictStrategy = "remote"
	ConflictStrategyNewer  ConflictStrategy = "newer"
	ConflictStrategyRename ConflictStrategy = "rename"
	ConflictStrategyAsk    ConflictStrategy = "ask"
)

// ConflictInfo 冲突信息
type ConflictInfo struct {
	Path          string    `json:"path"`
	LocalModTime  time.Time `json:"local_mod_time"`
	RemoteModTime time.Time `json:"remote_mod_time"`
	LocalSize     int64     `json:"local_size"`
	RemoteSize    int64     `json:"remote_size"`
	LocalHash     string    `json:"local_hash,omitempty"`
	RemoteHash    string    `json:"remote_hash,omitempty"`
}

// ============================================================
// 云存储后端类型
// ============================================================

// CloudBackend 云存储后端类型
type CloudBackend string

const (
	BackendS3    CloudBackend = "s3"
	BackendAzure CloudBackend = "azure_blob"
	BackendGCS   CloudBackend = "gcs"
	BackendOSS   CloudBackend = "aliyun_oss"
	BackendMinIO CloudBackend = "minio"
)

// IsValid 检查后端类型是否有效
func (b CloudBackend) IsValid() bool {
	switch b {
	case BackendS3, BackendAzure, BackendGCS, BackendOSS, BackendMinIO:
		return true
	}
	return false
}

// ============================================================
// 同步方向常量（兼容 sync_engine.go）
// ============================================================

// SyncDirection 同步方向
type SyncDirection = SyncMode

const (
	SyncDirectionUpload   = SyncModeUploadOnly
	SyncDirectionDownload = SyncModeDownloadOnly
	SyncDirectionBidirect = SyncModeBidirectional
)

// ============================================================
// 同步模式
// ============================================================

// SyncMode 同步模式
type SyncMode string

const (
	SyncModeBidirectional SyncMode = "bidirect" // 双向同步
	SyncModeUploadOnly    SyncMode = "upload"   // 单向上传
	SyncModeDownloadOnly  SyncMode = "download" // 单向下载
	SyncModeSync          SyncMode = "sync"     // 同步（兼容旧接口）
)

// IsValid 检查同步模式是否有效
func (m SyncMode) IsValid() bool {
	switch m {
	case SyncModeBidirectional, SyncModeUploadOnly, SyncModeDownloadOnly:
		return true
	}
	return false
}

// ============================================================
// 冲突处理策略
// ============================================================

// ConflictPolicy 冲突处理策略
type ConflictPolicy string

const (
	ConflictLocalFirst  ConflictPolicy = "local_first"  // 本地优先
	ConflictRemoteFirst ConflictPolicy = "remote_first" // 远程优先
	ConflictKeepBoth    ConflictPolicy = "keep_both"    // 保留两者
)

// IsValid 检查冲突策略是否有效
func (p ConflictPolicy) IsValid() bool {
	switch p {
	case ConflictLocalFirst, ConflictRemoteFirst, ConflictKeepBoth:
		return true
	}
	return false
}

// ============================================================
// 同步任务状态
// ============================================================

// TaskStatus 任务状态
type TaskStatus string

const (
	StatusIdle      TaskStatus = "idle"
	StatusSyncing   TaskStatus = "running"
	StatusPaused    TaskStatus = "paused"
	StatusError     TaskStatus = "error"
	StatusDisabled  TaskStatus = "disabled"
	StatusCompleted TaskStatus = "completed"
	StatusCancelled TaskStatus = "cancelled"
	StatusFailed    TaskStatus = "failed"

	// 兼容 sync_engine.go 使用的常量
	TaskStatusIdle      = StatusIdle
	TaskStatusRunning   = StatusSyncing
	TaskStatusPaused    = StatusPaused
	TaskStatusCompleted = StatusCompleted
	TaskStatusCancelled = StatusCancelled
	TaskStatusFailed    = StatusFailed
)

// ============================================================
// 同步调度类型
// ============================================================

// ScheduleType 调度类型
type ScheduleType string

const (
	ScheduleManual   ScheduleType = "manual"   // 手动触发
	ScheduleCron     ScheduleType = "cron"     // 定时同步
	ScheduleRealtime ScheduleType = "realtime" // 实时同步

	// 兼容旧接口
	ScheduleTypeManual   = ScheduleManual
	ScheduleTypeCron     = ScheduleCron
	ScheduleTypeRealtime = ScheduleRealtime
)

// ============================================================
// 云存储连接配置
// ============================================================

// ConnectionConfig 云存储连接配置
type ConnectionConfig struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Backend     CloudBackend `json:"backend"`
	Endpoint    string       `json:"endpoint,omitempty"` // S3/MinIO 端点
	Region      string       `json:"region,omitempty"`   // S3/Azure 区域
	Bucket      string       `json:"bucket"`             // 存储桶/容器名称
	AccessKey   string       `json:"access_key"`
	SecretKey   string       `json:"secret_key"`
	BasePath    string       `json:"base_path,omitempty"`    // 远端基础路径
	AccountName string       `json:"account_name,omitempty"` // Azure 账户名
	ProjectID   string       `json:"project_id,omitempty"`   // GCS 项目ID
	UseSSL      bool         `json:"use_ssl"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Validate 验证连接配置
func (c *ConnectionConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("connection name is required")
	}
	if !c.Backend.IsValid() {
		return fmt.Errorf("invalid backend type: %s", c.Backend)
	}
	if c.Bucket == "" {
		return fmt.Errorf("bucket/container name is required")
	}
	if c.AccessKey == "" {
		return fmt.Errorf("access key is required")
	}
	if c.SecretKey == "" {
		return fmt.Errorf("secret key is required")
	}

	// 后端特定验证
	switch c.Backend {
	case BackendS3, BackendMinIO:
		if c.Endpoint == "" && c.Backend == BackendMinIO {
			return fmt.Errorf("endpoint is required for MinIO")
		}
	case BackendAzure:
		if c.AccountName == "" {
			return fmt.Errorf("account name is required for Azure Blob")
		}
	case BackendGCS:
		if c.ProjectID == "" {
			return fmt.Errorf("project ID is required for Google Cloud Storage")
		}
	}

	return nil
}

// ============================================================
// 文件过滤规则
// ============================================================

// FileFilter 文件过滤规则
type FileFilter struct {
	// 按扩展名过滤（白名单）
	IncludeExtensions []string `json:"include_extensions,omitempty"`
	// 按扩展名过滤（黑名单）
	ExcludeExtensions []string `json:"exclude_extensions,omitempty"`
	// 按路径模式过滤（glob模式）
	IncludePatterns []string `json:"include_patterns,omitempty"`
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`
	// 按文件大小过滤
	MaxFileSize int64 `json:"max_file_size,omitempty"` // 最大文件大小 (bytes), 0=不限制
	MinFileSize int64 `json:"min_file_size,omitempty"` // 最小文件大小 (bytes), 0=不限制
	// 排除隐藏文件
	ExcludeHidden bool `json:"exclude_hidden"`
}

// ============================================================
// 同步调度配置
// ============================================================

// SyncSchedule 同步调度配置
type SyncSchedule struct {
	Type     ScheduleType `json:"type"`
	CronExpr string       `json:"cron_expr,omitempty"` // cron 表达式, 仅 Type=cron 时有效
	Interval int          `json:"interval,omitempty"`  // 间隔秒数, 仅 Type=cron 时有效
}

// ============================================================
// 传输配置
// ============================================================

// TransferConfig 传输配置
type TransferConfig struct {
	// 带宽限制 (KB/s), 0=不限制
	BandwidthLimit int `json:"bandwidth_limit,omitempty"`
	// 并发传输数
	ConcurrentTransfers int `json:"concurrent_transfers,omitempty"`
	// 传输加密
	EncryptionEnabled bool   `json:"encryption_enabled"`
	EncryptionKey     string `json:"encryption_key,omitempty"` // 加密密钥 (base64)
	// 块大小 (MB), 用于分片上传
	BlockSizeMB int `json:"block_size_mb,omitempty"`
	// 重试配置
	MaxRetries    int `json:"max_retries,omitempty"`
	RetryDelaySec int `json:"retry_delay_sec,omitempty"`
}

// DefaultTransferConfig 默认传输配置
func DefaultTransferConfig() TransferConfig {
	return TransferConfig{
		BandwidthLimit:      0,
		ConcurrentTransfers: 4,
		EncryptionEnabled:   false,
		BlockSizeMB:         8,
		MaxRetries:          3,
		RetryDelaySec:       5,
	}
}

// ============================================================
// 同步任务
// ============================================================

// SyncTask 同步任务
type SyncTask struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	ConnectionID     string           `json:"connection_id"`
	ProviderID       string           `json:"provider_id"` // 提供商 ID（兼容旧接口）
	LocalPath        string           `json:"local_path"`
	RemotePath       string           `json:"remote_path"`
	Mode             SyncMode         `json:"mode"`
	Direction        SyncMode         `json:"direction"` // 同步方向，用于 sync_engine
	ConflictPolicy   ConflictPolicy   `json:"conflict_policy"`
	ConflictStrategy ConflictStrategy `json:"conflict_strategy"` // 用于冲突解决器
	Filter           FileFilter       `json:"filter"`
	Schedule         SyncSchedule     `json:"schedule"`
	Transfer         TransferConfig   `json:"transfer"`
	Status           TaskStatus       `json:"status"`
	Enabled          bool             `json:"enabled"`
	ScheduleType     ScheduleType     `json:"schedule_type"`           // 调度类型（兼容旧接口）
	ScheduleExpr     string           `json:"schedule_expr,omitempty"` // cron 表达式（兼容旧接口）

	// 同步引擎字段
	IncludePatterns []string `json:"include_patterns,omitempty"`  // 包含的 glob 模式
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`  // 排除的 glob 模式
	MaxFileSize     int64    `json:"max_file_size,omitempty"`     // 最大文件大小
	ChecksumVerify  bool     `json:"checksum_verify,omitempty"`   // 校验和验证
	PreserveModTime bool     `json:"preserve_mod_time,omitempty"` // 保留修改时间
	DeleteLocal     bool     `json:"delete_local,omitempty"`      // 删除本地多余文件
	DeleteRemote    bool     `json:"delete_remote,omitempty"`     // 删除远程多余文件

	// 统计信息
	LastSyncTime   *time.Time `json:"last_sync_time,omitempty"`
	LastSyncResult string     `json:"last_sync_result,omitempty"`
	TotalFiles     int64      `json:"total_files"`
	TotalSize      int64      `json:"total_size"`
	SyncedFiles    int64      `json:"synced_files"`
	SkippedFiles   int64      `json:"skipped_files"`
	FailedFiles    int64      `json:"failed_files"`
	LastError      string     `json:"last_error,omitempty"`
	ErrorCount     int        `json:"error_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate 验证同步任务
func (t *SyncTask) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if t.ConnectionID == "" {
		return fmt.Errorf("connection ID is required")
	}
	if t.LocalPath == "" {
		return fmt.Errorf("local path is required")
	}
	if t.RemotePath == "" {
		return fmt.Errorf("remote path is required")
	}
	if !t.Mode.IsValid() {
		return fmt.Errorf("invalid sync mode: %s", t.Mode)
	}
	if !t.ConflictPolicy.IsValid() {
		return fmt.Errorf("invalid conflict policy: %s", t.ConflictPolicy)
	}
	if t.Schedule.Type == ScheduleCron && t.Schedule.CronExpr == "" && t.Schedule.Interval <= 0 {
		return fmt.Errorf("cron expression or interval is required for cron schedule")
	}
	return nil
}

// ============================================================
// 同步状态和统计
// ============================================================

// SyncStatus 同步状态详情
type SyncStatus struct {
	TaskID           string         `json:"task_id"`
	TaskName         string         `json:"task_name"`
	Status           TaskStatus     `json:"status"`
	Progress         float64        `json:"progress"` // 0-100
	CurrentFile      string         `json:"current_file"`
	CurrentAction    string         `json:"current_action"` // upload/download/delete/skip
	TotalBytes       int64          `json:"total_bytes"`
	BytesTotal       int64          `json:"bytes_total"`
	BytesSynced      int64          `json:"bytes_synced"`
	SpeedBps         int64          `json:"speed_bps"`   // 当前速度 (bytes/sec)
	ETASeconds       int            `json:"eta_seconds"` // 预计剩余秒数
	StartedAt        *time.Time     `json:"started_at,omitempty"`
	EstimatedEnd     *time.Time     `json:"estimated_end,omitempty"`
	StartTime        time.Time      `json:"start_time"`
	EndTime          time.Time      `json:"end_time"`
	TotalFiles       int64          `json:"total_files"`
	ProcessedFiles   int64          `json:"processed_files"`
	UploadedFiles    int64          `json:"uploaded_files"`
	DownloadedFiles  int64          `json:"downloaded_files"`
	DeletedFiles     int64          `json:"deleted_files"`
	SkippedFiles     int64          `json:"skipped_files"`
	FailedFiles      int64          `json:"failed_files"`
	TransferredBytes int64          `json:"transferred_bytes"`
	Speed            int64          `json:"speed"` // KB/s
	Conflicts        []ConflictInfo `json:"conflicts,omitempty"`
	Errors           []SyncError    `json:"errors,omitempty"`
}

// SyncError 同步错误
type SyncError struct {
	Path      string    `json:"path"`
	Action    string    `json:"action"` // upload/download/delete_remote/delete_local
	Error     string    `json:"error"`
	Time      time.Time `json:"time"`
	Timestamp time.Time `json:"timestamp"`
}

// ProviderInfo 提供商信息
type ProviderInfo struct {
	Type        ProviderType `json:"type"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Features    []string     `json:"features"`
}

// ProviderItem 已配置的提供商实例
type ProviderItem struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      ProviderType   `json:"type"`
	Enabled   bool           `json:"enabled"`
	Bucket    string         `json:"bucket,omitempty"`
	Config    ProviderConfig `json:"config,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// SupportedProviders 返回支持的云存储提供商列表
func SupportedProviders() []ProviderInfo {
	return []ProviderInfo{
		{Type: ProviderS3, Name: "S3", Description: "Amazon S3 / S3 兼容存储", Features: []string{"upload", "download", "sync", "multipart"}},
		{Type: ProviderAWSS3, Name: "AWS S3", Description: "Amazon Web Services S3", Features: []string{"upload", "download", "sync", "multipart", "versioning"}},
		{Type: ProviderAliyunOSS, Name: "阿里云 OSS", Description: "阿里云对象存储", Features: []string{"upload", "download", "sync", "multipart"}},
		{Type: ProviderTencentCOS, Name: "腾讯云 COS", Description: "腾讯云对象存储", Features: []string{"upload", "download", "sync", "multipart"}},
		{Type: ProviderBackblazeB2, Name: "Backblaze B2", Description: "Backblaze 对象存储", Features: []string{"upload", "download", "sync"}},
		{Type: ProviderWebDAV, Name: "WebDAV", Description: "WebDAV 协议存储", Features: []string{"upload", "download"}},
		{Type: ProviderGoogleDrive, Name: "Google Drive", Description: "Google 云端硬盘", Features: []string{"upload", "download", "sync", "oauth"}},
		{Type: ProviderOneDrive, Name: "OneDrive", Description: "Microsoft OneDrive", Features: []string{"upload", "download", "sync", "oauth"}},
		{Type: ProviderDropbox, Name: "Dropbox", Description: "Dropbox 云存储", Features: []string{"upload", "download", "sync", "oauth"}},
		{Type: Provider115, Name: "115 网盘", Description: "115 网盘", Features: []string{"upload", "download"}},
		{Type: ProviderQuark, Name: "夸克网盘", Description: "夸克网盘", Features: []string{"upload", "download"}},
		{Type: ProviderBaiduPan, Name: "百度网盘", Description: "百度网盘", Features: []string{"upload", "download"}},
		{Type: ProviderAliyunPan, Name: "阿里云盘", Description: "阿里云盘", Features: []string{"upload", "download"}},
	}
}

// SyncLog 同步日志条目
type SyncLog struct {
	TaskID    string    `json:"task_id"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // "info", "warn", "error"
	Message   string    `json:"message"`
	FilePath  string    `json:"file_path,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// StorageUsage 存储空间用量
type StorageUsage struct {
	ConnectionID string       `json:"connection_id"`
	Backend      CloudBackend `json:"backend"`
	Bucket       string       `json:"bucket"`
	TotalBytes   int64        `json:"total_bytes"`
	UsedBytes    int64        `json:"used_bytes"`
	FreeBytes    int64        `json:"free_bytes"`
	ObjectCount  int64        `json:"object_count"`
	QuotaBytes   int64        `json:"quota_bytes,omitempty"` // 配额, 0=无限
}

// SyncStats 同步统计汇总
type SyncStats struct {
	TotalProviders  int64     `json:"total_providers"`
	TotalTasks      int64     `json:"total_tasks"`
	ActiveTasks     int64     `json:"active_tasks"`
	PausedTasks     int64     `json:"paused_tasks"`
	ErrorTasks      int64     `json:"error_tasks"`
	TotalFiles      int64     `json:"total_files"`
	TotalSize       int64     `json:"total_size"`
	TotalSynced     int64     `json:"total_synced"`
	TotalBytes      int64     `json:"total_bytes"`
	TotalBytesHuman string    `json:"total_bytes_human,omitempty"`
	LastSyncTime    time.Time `json:"last_sync_time,omitempty"`
	SyncedFiles     int64     `json:"synced_files"`
	FailedFiles     int64     `json:"failed_files"`
	TotalBandwidth  int       `json:"total_bandwidth"` // KB/s
}

// ============================================================
// 创建/更新请求
// ============================================================

// CreateConnectionRequest 创建连接请求
type CreateConnectionRequest struct {
	Name        string       `json:"name" binding:"required"`
	Backend     CloudBackend `json:"backend" binding:"required"`
	Endpoint    string       `json:"endpoint,omitempty"`
	Region      string       `json:"region,omitempty"`
	Bucket      string       `json:"bucket" binding:"required"`
	AccessKey   string       `json:"access_key" binding:"required"`
	SecretKey   string       `json:"secret_key" binding:"required"`
	BasePath    string       `json:"base_path,omitempty"`
	AccountName string       `json:"account_name,omitempty"`
	ProjectID   string       `json:"project_id,omitempty"`
	UseSSL      bool         `json:"use_ssl"`
}

// UpdateConnectionRequest 更新连接请求
type UpdateConnectionRequest struct {
	Name        string `json:"name,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	Region      string `json:"region,omitempty"`
	Bucket      string `json:"bucket,omitempty"`
	AccessKey   string `json:"access_key,omitempty"`
	SecretKey   string `json:"secret_key,omitempty"`
	BasePath    string `json:"base_path,omitempty"`
	AccountName string `json:"account_name,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`
	UseSSL      *bool  `json:"use_ssl,omitempty"`
}

// CreateTaskRequest 创建同步任务请求
type CreateTaskRequest struct {
	Name           string          `json:"name" binding:"required"`
	ConnectionID   string          `json:"connection_id" binding:"required"`
	LocalPath      string          `json:"local_path" binding:"required"`
	RemotePath     string          `json:"remote_path" binding:"required"`
	Mode           SyncMode        `json:"mode" binding:"required"`
	ConflictPolicy ConflictPolicy  `json:"conflict_policy"`
	Filter         *FileFilter     `json:"filter,omitempty"`
	Schedule       *SyncSchedule   `json:"schedule,omitempty"`
	Transfer       *TransferConfig `json:"transfer,omitempty"`
}

// UpdateTaskRequest 更新同步任务请求
type UpdateTaskRequest struct {
	Name           string          `json:"name,omitempty"`
	LocalPath      string          `json:"local_path,omitempty"`
	RemotePath     string          `json:"remote_path,omitempty"`
	Mode           *SyncMode       `json:"mode,omitempty"`
	ConflictPolicy *ConflictPolicy `json:"conflict_policy,omitempty"`
	Filter         *FileFilter     `json:"filter,omitempty"`
	Schedule       *SyncSchedule   `json:"schedule,omitempty"`
	Transfer       *TransferConfig `json:"transfer,omitempty"`
	Enabled        *bool           `json:"enabled,omitempty"`
}
