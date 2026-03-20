package cloudsync

import (
	"time"
)

// ProviderType 云存储提供商类型
type ProviderType string

// 云存储提供商类型常量
const (
	// ProviderAliyunOSS 阿里云 OSS
	ProviderAliyunOSS ProviderType = "aliyun_oss"
	// ProviderTencentCOS 腾讯云 COS
	ProviderTencentCOS ProviderType = "tencent_cos"
	ProviderAWSS3        ProviderType = "aws_s3"
	ProviderGoogleDrive  ProviderType = "google_drive"
	ProviderOneDrive     ProviderType = "onedrive"
	ProviderBackblazeB2  ProviderType = "backblaze_b2"
	ProviderWebDAV       ProviderType = "webdav"
	ProviderS3Compatible ProviderType = "s3_compatible" // 通用 S3 兼容存储
)

// SyncDirection 同步方向
type SyncDirection string

// 同步方向常量
const (
	// SyncDirectionUpload 本地 → 云端
	SyncDirectionUpload SyncDirection = "upload"
	// SyncDirectionDownload 云端 → 本地
	SyncDirectionDownload SyncDirection = "download"
	// SyncDirectionBidirect 双向同步
	SyncDirectionBidirect SyncDirection = "bidirect"
)

// SyncMode 同步模式
type SyncMode string

// 同步模式常量
const (
	// SyncModeMirror 镜像模式（本地为主）
	SyncModeMirror SyncMode = "mirror"
	// SyncModeBackup 备份模式（保留历史）
	SyncModeBackup SyncMode = "backup"
	// SyncModeSync 同步模式（双向）
	SyncModeSync SyncMode = "sync"
	// SyncModeIncrement 增量同步
	SyncModeIncrement SyncMode = "increment"
)

// ScheduleType 调度类型
type ScheduleType string

// 调度类型常量
const (
	// ScheduleTypeManual 手动触发
	ScheduleTypeManual ScheduleType = "manual"
	// ScheduleTypeRealtime 实时监控
	ScheduleTypeRealtime ScheduleType = "realtime"
	// ScheduleTypeInterval 定时执行
	ScheduleTypeInterval ScheduleType = "interval"
	// ScheduleTypeCron Cron 表达式
	ScheduleTypeCron ScheduleType = "cron"
)

// ConflictStrategy 冲突解决策略
type ConflictStrategy string

// 冲突解决策略常量
const (
	// ConflictStrategySkip 跳过冲突文件
	ConflictStrategySkip ConflictStrategy = "skip"
	// ConflictStrategyLocal 本地优先
	ConflictStrategyLocal ConflictStrategy = "local"
	// ConflictStrategyRemote 远程优先
	ConflictStrategyRemote ConflictStrategy = "remote"
	// ConflictStrategyNewer 较新文件优先
	ConflictStrategyNewer ConflictStrategy = "newer"
	// ConflictStrategyRename 重命名冲突文件
	ConflictStrategyRename ConflictStrategy = "rename"
	// ConflictStrategyAsk 询问用户
	ConflictStrategyAsk ConflictStrategy = "ask"
)

// TaskStatus 任务状态
type TaskStatus string

// 任务状态常量
const (
	// TaskStatusIdle 空闲状态
	TaskStatusIdle TaskStatus = "idle"
	// TaskStatusRunning 运行中
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusPaused 已暂停
	TaskStatusPaused TaskStatus = "paused"
	// TaskStatusCompleted 已完成
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusFailed 已失败
	TaskStatusFailed TaskStatus = "failed"
	// TaskStatusCancelled 已取消
	TaskStatusCancelled TaskStatus = "cancelled"
)

// ProviderConfig 云存储提供商配置
type ProviderConfig struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Type      ProviderType `json:"type"`
	Enabled   bool         `json:"enabled"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
	LastUsed  time.Time    `json:"lastUsed,omitempty"`

	// S3 兼容存储配置（阿里云 OSS、腾讯云 COS、AWS S3、Backblaze B2）
	Endpoint  string `json:"endpoint,omitempty"`
	Region    string `json:"region,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"-"`                   // 安全：禁止序列化到 JSON
	PathStyle bool   `json:"pathStyle,omitempty"` // 路径风格访问

	// Google Drive 配置
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"-"` // 安全：禁止序列化到 JSON
	RefreshToken string `json:"-"` // 安全：禁止序列化到 JSON
	RootFolderID string `json:"rootFolderId,omitempty"`

	// OneDrive 配置
	TenantID string `json:"tenantId,omitempty"`

	// WebDAV 配置
	Insecure bool `json:"insecure,omitempty"` // 跳过 TLS 验证

	// 通用配置
	MaxConnections int `json:"maxConnections,omitempty"`
	Timeout        int `json:"timeout,omitempty"` // 秒
	RetryCount     int `json:"retryCount,omitempty"`
}

// SyncTask 同步任务配置
type SyncTask struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ProviderID string    `json:"providerId"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`

	// 同步配置
	LocalPath  string        `json:"localPath"`
	RemotePath string        `json:"remotePath"`
	Direction  SyncDirection `json:"direction"`
	Mode       SyncMode      `json:"mode"`

	// 调度配置
	ScheduleType ScheduleType `json:"scheduleType"`
	ScheduleExpr string       `json:"scheduleExpr,omitempty"` // Cron 表达式或间隔
	NextRun      time.Time    `json:"nextRun,omitempty"`

	// 过滤配置
	IncludePatterns []string `json:"includePatterns,omitempty"`
	ExcludePatterns []string `json:"excludePatterns,omitempty"`
	MaxFileSize     int64    `json:"maxFileSize,omitempty"` // 0 表示不限制

	// 冲突处理
	ConflictStrategy ConflictStrategy `json:"conflictStrategy"`

	// 高级选项
	DeleteRemote    bool   `json:"deleteRemote"`    // 删除本地时是否删除远程
	DeleteLocal     bool   `json:"deleteLocal"`     // 删除远程时是否删除本地
	PreserveModTime bool   `json:"preserveModTime"` // 保留修改时间
	ChecksumVerify  bool   `json:"checksumVerify"`  // 校验文件完整性
	Encrypt         bool   `json:"encrypt"`         // 加密传输
	EncryptKey      string `json:"-"`               // 安全：禁止序列化到 JSON

	// 带宽限制
	BandwidthLimit int64 `json:"bandwidthLimit,omitempty"` // KB/s, 0 表示不限制

	// 状态信息
	Status    TaskStatus `json:"status"`
	LastSync  time.Time  `json:"lastSync,omitempty"`
	LastError string     `json:"lastError,omitempty"`
}

// SyncStatus 同步状态
type SyncStatus struct {
	TaskID    string     `json:"taskId"`
	Status    TaskStatus `json:"status"`
	StartTime time.Time  `json:"startTime,omitempty"`
	EndTime   time.Time  `json:"endTime,omitempty"`

	// 进度信息
	TotalFiles       int64   `json:"totalFiles"`
	ProcessedFiles   int64   `json:"processedFiles"`
	TotalBytes       int64   `json:"totalBytes"`
	TransferredBytes int64   `json:"transferredBytes"`
	Speed            int64   `json:"speed"`    // KB/s
	Progress         float64 `json:"progress"` // 百分比

	// 当前操作
	CurrentFile   string `json:"currentFile,omitempty"`
	CurrentAction string `json:"currentAction,omitempty"`

	// 统计信息
	UploadedFiles   int64 `json:"uploadedFiles"`
	DownloadedFiles int64 `json:"downloadedFiles"`
	SkippedFiles    int64 `json:"skippedFiles"`
	FailedFiles     int64 `json:"failedFiles"`
	DeletedFiles    int64 `json:"deletedFiles"`

	// 冲突信息
	Conflicts []ConflictInfo `json:"conflicts,omitempty"`

	// 错误信息
	Errors []SyncError `json:"errors,omitempty"`
}

// ConflictInfo 冲突信息
type ConflictInfo struct {
	Path          string           `json:"path"`
	LocalModTime  time.Time        `json:"localModTime"`
	LocalSize     int64            `json:"localSize"`
	LocalHash     string           `json:"localHash,omitempty"`
	RemoteModTime time.Time        `json:"remoteModTime"`
	RemoteSize    int64            `json:"remoteSize"`
	RemoteHash    string           `json:"remoteHash,omitempty"`
	Resolution    ConflictStrategy `json:"resolution,omitempty"`
}

// SyncError 同步错误
type SyncError struct {
	Time   time.Time `json:"time"`
	Path   string    `json:"path"`
	Action string    `json:"action"`
	Error  string    `json:"error"`
}

// FileInfo 文件信息
type FileInfo struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	IsDir   bool      `json:"isDir"`
	Hash    string    `json:"hash,omitempty"`
	Version string    `json:"version,omitempty"`
}

// SyncOperation 同步操作
type SyncOperation struct {
	Type       SyncOpType `json:"type"`
	LocalPath  string     `json:"localPath"`
	RemotePath string     `json:"remotePath"`
	Size       int64      `json:"size"`
	ModTime    time.Time  `json:"modTime"`
	Hash       string     `json:"hash,omitempty"`
}

// SyncOpType 同步操作类型
type SyncOpType string

const (
	SyncOpUpload       SyncOpType = "upload"
	SyncOpDownload     SyncOpType = "download"
	SyncOpDeleteLocal  SyncOpType = "delete_local"
	SyncOpDeleteRemote SyncOpType = "delete_remote"
	SyncOpSkip         SyncOpType = "skip"
	SyncOpConflict     SyncOpType = "conflict"
)

// ProviderInfo 提供商信息
type ProviderInfo struct {
	Type        ProviderType `json:"type"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Features    []string     `json:"features"`
}

// SyncStats 同步统计
type SyncStats struct {
	TotalTasks      int64     `json:"totalTasks"`
	ActiveTasks     int64     `json:"activeTasks"`
	TotalProviders  int64     `json:"totalProviders"`
	TotalSynced     int64     `json:"totalSynced"`
	TotalBytes      int64     `json:"totalBytes"`
	TotalBytesHuman string    `json:"totalBytesHuman"`
	LastSyncTime    time.Time `json:"lastSyncTime,omitempty"`
}

// SupportedProviders 返回支持的提供商列表
func SupportedProviders() []ProviderInfo {
	return []ProviderInfo{
		{
			Type:        ProviderAliyunOSS,
			Name:        "阿里云 OSS",
			Description: "阿里云对象存储服务",
			Features:    []string{"upload", "download", "delete", "list", "multipart"},
		},
		{
			Type:        ProviderTencentCOS,
			Name:        "腾讯云 COS",
			Description: "腾讯云对象存储服务",
			Features:    []string{"upload", "download", "delete", "list", "multipart"},
		},
		{
			Type:        ProviderAWSS3,
			Name:        "AWS S3",
			Description: "Amazon Simple Storage Service",
			Features:    []string{"upload", "download", "delete", "list", "multipart"},
		},
		{
			Type:        ProviderGoogleDrive,
			Name:        "Google Drive",
			Description: "Google 云端硬盘",
			Features:    []string{"upload", "download", "delete", "list", "share"},
		},
		{
			Type:        ProviderOneDrive,
			Name:        "OneDrive",
			Description: "Microsoft OneDrive",
			Features:    []string{"upload", "download", "delete", "list", "share"},
		},
		{
			Type:        ProviderBackblazeB2,
			Name:        "Backblaze B2",
			Description: "Backblaze B2 云存储",
			Features:    []string{"upload", "download", "delete", "list", "multipart"},
		},
		{
			Type:        ProviderWebDAV,
			Name:        "WebDAV",
			Description: "通用 WebDAV 协议",
			Features:    []string{"upload", "download", "delete", "list"},
		},
		{
			Type:        ProviderS3Compatible,
			Name:        "S3 兼容存储",
			Description: "兼容 S3 协议的存储服务",
			Features:    []string{"upload", "download", "delete", "list", "multipart"},
		},
	}
}
