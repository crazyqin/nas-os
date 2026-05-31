// Package remotebackup 远程备份引擎模块
package remotebackup

import (
	"context"
	"sync"
	"time"
)

// BackupTargetType 远程备份目标类型
type BackupTargetType string

const (
	// TargetTypeS3 AWS S3 或兼容 S3 的存储
	TargetTypeS3 BackupTargetType = "s3"
	// TargetTypeFTP FTP 协议
	TargetTypeFTP BackupTargetType = "ftp"
	// TargetTypeSFTP SFTP 协议
	TargetTypeSFTP BackupTargetType = "sftp"
	// TargetTypeWebDAV WebDAV 协议
	TargetTypeWebDAV BackupTargetType = "webdav"
	// TargetTypeRsync Rsync 协议
	TargetTypeRsync BackupTargetType = "rsync"
)

// BackupStatus 备份任务状态
type BackupStatus string

const (
	// StatusPending 等待执行
	StatusPending BackupStatus = "pending"
	// StatusRunning 执行中
	StatusRunning BackupStatus = "running"
	// StatusCompleted 已完成
	StatusCompleted BackupStatus = "completed"
	// StatusFailed 失败
	StatusFailed BackupStatus = "failed"
	// StatusCancelled 已取消
	StatusCancelled BackupStatus = "cancelled"
	// StatusPaused 已暂停
	StatusPaused BackupStatus = "paused"
)

// BackupStrategy 备份策略
type BackupStrategy string

const (
	// StrategyFull 全量备份
	StrategyFull BackupStrategy = "full"
	// StrategyIncremental 增量备份
	StrategyIncremental BackupStrategy = "incremental"
	// StrategyDifferential 差异备份
	StrategyDifferential BackupStrategy = "differential"
)

// RetentionUnit 保留策略单位
type RetentionUnit string

const (
	// RetentionDays 按天保留
	RetentionDays RetentionUnit = "days"
	// RetentionVersions 按版本数保留
	RetentionVersions RetentionUnit = "versions"
	// RetentionForever 永久保留
	RetentionForever RetentionUnit = "forever"
)

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	// Enabled 是否启用加密
	Enabled bool `json:"enabled"`
	// Algorithm 加密算法（默认 aes-256-gcm）
	Algorithm string `json:"algorithm,omitempty"`
	// KeyID 加密密钥标识
	KeyID string `json:"key_id,omitempty"`
	// Passphrase 加密口令（用于派生密钥）
	Passphrase string `json:"passphrase,omitempty"`
}

// RetentionPolicy 备份保留策略
type RetentionPolicy struct {
	// Unit 保留单位
	Unit RetentionUnit `json:"unit"`
	// Value 保留值（天数或版本数）
	Value int `json:"value"`
}

// BackupTarget 远程备份目标配置
type BackupTarget struct {
	// ID 目标唯一标识
	ID string `json:"id"`
	// Name 目标名称
	Name string `json:"name"`
	// Type 目标类型（s3/ftp/sftp/webdav/rsync）
	Type BackupTargetType `json:"type"`
	// Endpoint 连接端点
	Endpoint string `json:"endpoint"`
	// Port 端口号
	Port int `json:"port,omitempty"`
	// Bucket 存储桶名称（S3/WebDAV使用）
	Bucket string `json:"bucket,omitempty"`
	// Path 远程路径
	Path string `json:"path,omitempty"`
	// AccessKey 访问密钥
	AccessKey string `json:"access_key,omitempty"`
	// SecretKey 秘密密钥
	SecretKey string `json:"secret_key,omitempty"`
	// Username 用户名（FTP/SFTP/WebDAV使用）
	Username string `json:"username,omitempty"`
	// Password 密码
	Password string `json:"password,omitempty"`
	// Region 区域（S3使用）
	Region string `json:"region,omitempty"`
	// UseSSL 是否使用SSL/TLS
	UseSSL bool `json:"use_ssl,omitempty"`
	// Encryption 加密配置
	Encryption EncryptionConfig `json:"encryption,omitempty"`
	// BandwidthLimit 带宽限制（字节/秒，0表示不限制）
	BandwidthLimit int64 `json:"bandwidth_limit,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// BackupSchedule 备份调度配置
type BackupSchedule struct {
	// Enabled 是否启用调度
	Enabled bool `json:"enabled"`
	// Cron cron 表达式
	Cron string `json:"cron,omitempty"`
	// Interval 间隔时间（秒）
	Interval int `json:"interval,omitempty"`
	// NextRun 下次运行时间
	NextRun *time.Time `json:"next_run,omitempty"`
}

// BackupJob 备份任务
type BackupJob struct {
	// ID 任务唯一标识
	ID string `json:"id"`
	// Name 任务名称
	Name string `json:"name"`
	// SourcePaths 源路径列表
	SourcePaths []string `json:"source_paths"`
	// TargetID 目标ID
	TargetID string `json:"target_id"`
	// Strategy 备份策略
	Strategy BackupStrategy `json:"strategy"`
	// RetentionPolicy 保留策略
	RetentionPolicy RetentionPolicy `json:"retention_policy"`
	// Schedule 调度配置
	Schedule BackupSchedule `json:"schedule"`
	// ExcludePatterns 排除模式列表
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`
	// Status 当前状态
	Status BackupStatus `json:"status"`
	// Progress 进度百分比（0-100）
	Progress float64 `json:"progress"`
	// TransferStats 传输统计
	TransferStats TransferStats `json:"transfer_stats"`
	// LastError 最后错误信息
	LastError string `json:"last_error,omitempty"`
	// LastRun 上次运行时间
	LastRun *time.Time `json:"last_run,omitempty"`
	// VersionCount 版本数量
	VersionCount int `json:"version_count"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// BackupVersion 备份版本
type BackupVersion struct {
	// ID 版本唯一标识
	ID string `json:"id"`
	// JobID 关联的任务ID
	JobID string `json:"job_id"`
	// VersionNumber 版本号
	VersionNumber int `json:"version_number"`
	// Type 备份类型
	Type BackupStrategy `json:"type"`
	// StartedAt 开始时间
	StartedAt time.Time `json:"started_at"`
	// CompletedAt 完成时间
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// TotalSize 总大小（字节）
	TotalSize int64 `json:"total_size"`
	// TransferSize 传输大小（字节，去重/压缩后）
	TransferSize int64 `json:"transfer_size"`
	// FileCount 文件数量
	FileCount int `json:"file_count"`
	// Checksum 整体校验和（SHA-256）
	Checksum string `json:"checksum"`
	// Status 状态
	Status BackupStatus `json:"status"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
}

// RestorePoint 恢复点
type RestorePoint struct {
	// Version 版本信息
	Version BackupVersion `json:"version"`
	// RecoverableFiles 可恢复文件列表
	RecoverableFiles []string `json:"recoverable_files"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
}

// TransferStats 传输统计
type TransferStats struct {
	// SpeedBytesPerSec 传输速度（字节/秒）
	SpeedBytesPerSec int64 `json:"speed_bytes_per_sec"`
	// TransferredBytes 已传输字节数
	TransferredBytes int64 `json:"transferred_bytes"`
	// TotalBytes 总字节数
	TotalBytes int64 `json:"total_bytes"`
	// RemainingTimeSec 预计剩余时间（秒）
	RemainingTimeSec int64 `json:"remaining_time_sec"`
	// StartTime 开始时间
	StartTime time.Time `json:"start_time"`
}

// TransferProgress 传输进度（用于更新）
type TransferProgress struct {
	// TransferredBytes 已传输字节数
	TransferredBytes int64
	// TotalBytes 总字节数
	TotalBytes int64
	// SpeedBytesPerSec 传输速度
	SpeedBytesPerSec int64
}

// RestoreRequest 恢复请求
type RestoreRequest struct {
	// JobID 任务ID
	JobID string `json:"job_id"`
	// VersionID 版本ID（为空则恢复最新版本）
	VersionID string `json:"version_id,omitempty"`
	// RestorePath 恢复目标路径
	RestorePath string `json:"restore_path"`
	// Files 要恢复的文件列表（为空则恢复全部）
	Files []string `json:"files,omitempty"`
	// Overwrite 是否覆盖已有文件
	Overwrite bool `json:"overwrite"`
}

// RestoreResult 恢复结果
type RestoreResult struct {
	// JobID 任务ID
	JobID string `json:"job_id"`
	// VersionID 版本ID
	VersionID string `json:"version_id"`
	// RestoredFiles 已恢复文件数
	RestoredFiles int `json:"restored_files"`
	// TotalBytes 恢复总字节数
	TotalBytes int64 `json:"total_bytes"`
	// Errors 错误列表
	Errors []string `json:"errors,omitempty"`
}

// Manager 远程备份管理器
type Manager struct {
	mu           sync.RWMutex
	targets      map[string]*BackupTarget
	jobs         map[string]*BackupJob
	versions     map[string][]*BackupVersion
	configPath   string
	cancelFuncs  map[string]context.CancelFunc
}
