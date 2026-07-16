// Package hybridshare provides cloud-local hybrid storage management for NAS-OS.
// Implements Synology Hybrid Share-like functionality with local caching,
// cloud storage backend, intelligent tiering, bandwidth control, and encryption.
package hybridshare

import (
	"fmt"
	"time"
)

// ============================================================
// 云存储后端类型
// ============================================================

// CloudBackend 云存储后端类型.
type CloudBackend string

const (
	BackendAWSS3      CloudBackend = "aws_s3"      // Amazon S3
	BackendAliyunOSS  CloudBackend = "aliyun_oss"  // 阿里云 OSS
	BackendTencentCOS CloudBackend = "tencent_cos" // 腾讯云 COS
	BackendMinIO      CloudBackend = "minio"       // MinIO
)

// IsValid 检查后端类型是否有效.
func (b CloudBackend) IsValid() bool {
	switch b {
	case BackendAWSS3, BackendAliyunOSS, BackendTencentCOS, BackendMinIO:
		return true
	}
	return false
}

// BackendName 返回后端显示名称.
func (b CloudBackend) BackendName() string {
	switch b {
	case BackendAWSS3:
		return "Amazon S3"
	case BackendAliyunOSS:
		return "阿里云 OSS"
	case BackendTencentCOS:
		return "腾讯云 COS"
	case BackendMinIO:
		return "MinIO"
	default:
		return string(b)
	}
}

// ============================================================
// 同步策略
// ============================================================

// SyncStrategy 同步策略类型.
type SyncStrategy string

const (
	SyncRealtime  SyncStrategy = "realtime"  // 实时同步
	SyncScheduled SyncStrategy = "scheduled" // 定时同步
	SyncManual    SyncStrategy = "manual"    // 手动同步
)

// IsValid 检查同步策略是否有效.
func (s SyncStrategy) IsValid() bool {
	switch s {
	case SyncRealtime, SyncScheduled, SyncManual:
		return true
	}
	return false
}

// ============================================================
// 缓存策略
// ============================================================

// CachePolicy 缓存策略类型.
type CachePolicy string

const (
	CachePolicyLRU  CachePolicy = "lru"  // 最近最少使用
	CachePolicyLFU  CachePolicy = "lfu"  // 最不经常使用
	CachePolicyFIFO CachePolicy = "fifo" // 先进先出
	CachePolicyTTL  CachePolicy = "ttl"  // 基于过期时间
)

// IsValid 检查缓存策略是否有效.
func (p CachePolicy) IsValid() bool {
	switch p {
	case CachePolicyLRU, CachePolicyLFU, CachePolicyFIFO, CachePolicyTTL:
		return true
	}
	return false
}

// ============================================================
// 文件状态
// ============================================================

// FileStatus 文件状态.
type FileStatus string

const (
	FileStatusLocal    FileStatus = "local"    // 仅本地
	FileStatusCloud    FileStatus = "cloud"    // 仅云端
	FileStatusSynced   FileStatus = "synced"   // 已同步
	FileStatusSyncing  FileStatus = "syncing"  // 同步中
	FileStatusPending  FileStatus = "pending"  // 待同步
	FileStatusConflict FileStatus = "conflict" // 冲突
	FileStatusError    FileStatus = "error"    // 错误
)

// ============================================================
// 混合共享配置
// ============================================================

// HybridShareConfig 混合共享配置.
type HybridShareConfig struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// 云存储后端配置
	Backend   CloudBackend `json:"backend"`
	Endpoint  string       `json:"endpoint,omitempty"` // 自定义端点
	Region    string       `json:"region,omitempty"`   // 区域
	Bucket    string       `json:"bucket"`             // 存储桶
	AccessKey string       `json:"access_key"`
	SecretKey string       `json:"secret_key"`
	BasePath  string       `json:"base_path,omitempty"` // 云端基础路径
	UseSSL    bool         `json:"use_ssl"`

	// 本地缓存配置
	LocalCachePath string      `json:"local_cache_path"`          // 本地缓存目录
	CacheSizeBytes int64       `json:"cache_size_bytes"`          // 缓存大小限制
	CachePolicy    CachePolicy `json:"cache_policy"`              // 缓存策略
	CacheTTLHours  int         `json:"cache_ttl_hours,omitempty"` // 缓存过期时间(小时)

	// 同步配置
	SyncStrategy    SyncStrategy `json:"sync_strategy"`
	SyncCronExpr    string       `json:"sync_cron_expr,omitempty"`    // 定时同步cron表达式
	SyncIntervalMin int          `json:"sync_interval_min,omitempty"` // 同步间隔(分钟)

	// 带宽控制
	UploadLimitKBps     int `json:"upload_limit_kbps,omitempty"`    // 上传限速(KB/s), 0=不限制
	DownloadLimitKBps   int `json:"download_limit_kbps,omitempty"`  // 下载限速(KB/s), 0=不限制
	ConcurrentTransfers int `json:"concurrent_transfers,omitempty"` // 并发传输数

	// 加密配置
	EncryptionEnabled bool   `json:"encryption_enabled"`
	EncryptionKey     string `json:"encryption_key,omitempty"` // 加密密钥(base64)

	// 状态
	Enabled   bool   `json:"enabled"`
	Status    string `json:"status"` // active, inactive, error, syncing
	LastError string `json:"last_error,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate 验证配置.
func (c *HybridShareConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !c.Backend.IsValid() {
		return fmt.Errorf("invalid backend: %s", c.Backend)
	}
	if c.Bucket == "" {
		return fmt.Errorf("bucket is required")
	}
	if c.AccessKey == "" {
		return fmt.Errorf("access_key is required")
	}
	if c.SecretKey == "" {
		return fmt.Errorf("secret_key is required")
	}
	if c.LocalCachePath == "" {
		return fmt.Errorf("local_cache_path is required")
	}
	if c.CacheSizeBytes <= 0 {
		return fmt.Errorf("cache_size_bytes must be positive")
	}
	if !c.CachePolicy.IsValid() {
		return fmt.Errorf("invalid cache_policy: %s", c.CachePolicy)
	}
	if !c.SyncStrategy.IsValid() {
		return fmt.Errorf("invalid sync_strategy: %s", c.SyncStrategy)
	}
	if c.SyncStrategy == SyncScheduled {
		if c.SyncCronExpr == "" && c.SyncIntervalMin <= 0 {
			return fmt.Errorf("sync_cron_expr or sync_interval_min required for scheduled sync")
		}
	}
	return nil
}

// DefaultHybridShareConfig 返回默认配置.
func DefaultHybridShareConfig() HybridShareConfig {
	return HybridShareConfig{
		CacheSizeBytes:      10 * 1024 * 1024 * 1024, // 10GB
		CachePolicy:         CachePolicyLRU,
		SyncStrategy:        SyncRealtime,
		ConcurrentTransfers: 4,
		UploadLimitKBps:     0,
		DownloadLimitKBps:   0,
		EncryptionEnabled:   false,
		UseSSL:              true,
		Enabled:             true,
		Status:              "inactive",
	}
}

// ============================================================
// 文件元数据
// ============================================================

// FileMetadata 文件元数据.
type FileMetadata struct {
	ID           string `json:"id"`
	ShareID      string `json:"share_id"`      // 关联的混合共享ID
	RelativePath string `json:"relative_path"` // 相对路径
	FileName     string `json:"file_name"`
	FileSize     int64  `json:"file_size"`
	ContentType  string `json:"content_type,omitempty"`
	MD5Hash      string `json:"md5_hash,omitempty"`
	SHA256Hash   string `json:"sha256_hash,omitempty"`

	// 状态信息
	Status       FileStatus `json:"status"`
	IsCached     bool       `json:"is_cached"` // 是否在本地缓存
	CachedAt     *time.Time `json:"cached_at,omitempty"`
	LastAccessAt *time.Time `json:"last_access_at,omitempty"`
	AccessCount  int64      `json:"access_count"`

	// 时间信息
	ModTime   time.Time `json:"mod_time"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 云端信息
	CloudPath    string `json:"cloud_path,omitempty"`
	CloudVersion string `json:"cloud_version,omitempty"`

	// 本地缓存信息
	LocalPath string `json:"local_path,omitempty"`
}

// ============================================================
// 同步任务
// ============================================================

// SyncTaskStatus 同步任务状态.
type SyncTaskStatus string

const (
	SyncTaskPending   SyncTaskStatus = "pending"
	SyncTaskRunning   SyncTaskStatus = "running"
	SyncTaskCompleted SyncTaskStatus = "completed"
	SyncTaskFailed    SyncTaskStatus = "failed"
	SyncTaskCancelled SyncTaskStatus = "cancelled"
)

// SyncDirection 同步方向.
type SyncDirection string

const (
	SyncDirectionUpload   SyncDirection = "upload"   // 上传到云端
	SyncDirectionDownload SyncDirection = "download" // 从云端下载
	SyncDirectionDelete   SyncDirection = "delete"   // 删除
)

// SyncTask 同步任务.
type SyncTask struct {
	ID          string         `json:"id"`
	ShareID     string         `json:"share_id"`
	Direction   SyncDirection  `json:"direction"`
	FilePath    string         `json:"file_path"`
	FileSize    int64          `json:"file_size"`
	Status      SyncTaskStatus `json:"status"`
	Progress    float64        `json:"progress"` // 0-100
	BytesSynced int64          `json:"bytes_synced"`
	SpeedBps    int64          `json:"speed_bps"` // 传输速度(bytes/sec)
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// ============================================================
// 容量统计
// ============================================================

// CapacityStats 容量统计.
type CapacityStats struct {
	ShareID string `json:"share_id"`

	// 本地缓存统计
	LocalCacheTotal int64   `json:"local_cache_total"` // 缓存总容量
	LocalCacheUsed  int64   `json:"local_cache_used"`  // 缓存已用
	LocalCacheFree  int64   `json:"local_cache_free"`  // 缓存可用
	CacheHitRate    float64 `json:"cache_hit_rate"`    // 缓存命中率

	// 云端存储统计
	CloudTotal       int64 `json:"cloud_total"`        // 云端总容量(0=无限)
	CloudUsed        int64 `json:"cloud_used"`         // 云端已用
	CloudObjectCount int64 `json:"cloud_object_count"` // 云端对象数

	// 文件统计
	TotalFiles     int64 `json:"total_files"`
	CachedFiles    int64 `json:"cached_files"`     // 本地缓存文件数
	CloudOnlyFiles int64 `json:"cloud_only_files"` // 仅云端文件数
	SyncedFiles    int64 `json:"synced_files"`     // 已同步文件数
	PendingFiles   int64 `json:"pending_files"`    // 待同步文件数

	// 传输统计
	TotalUploaded   int64 `json:"total_uploaded"`   // 总上传量
	TotalDownloaded int64 `json:"total_downloaded"` // 总下载量

	UpdatedAt time.Time `json:"updated_at"`
}

// ============================================================
// 带宽统计
// ============================================================

// BandwidthStats 带宽统计.
type BandwidthStats struct {
	ShareID            string    `json:"share_id"`
	CurrentUploadBps   int64     `json:"current_upload_bps"`   // 当前上传速度
	CurrentDownloadBps int64     `json:"current_download_bps"` // 当前下载速度
	AvgUploadBps       int64     `json:"avg_upload_bps"`       // 平均上传速度
	AvgDownloadBps     int64     `json:"avg_download_bps"`     // 平均下载速度
	PeakUploadBps      int64     `json:"peak_upload_bps"`      // 峰值上传速度
	PeakDownloadBps    int64     `json:"peak_download_bps"`    // 峰值下载速度
	UpdatedAt          time.Time `json:"updated_at"`
}

// ============================================================
// 同步日志
// ============================================================

// SyncLogLevel 日志级别.
type SyncLogLevel string

const (
	LogLevelInfo  SyncLogLevel = "info"
	LogLevelWarn  SyncLogLevel = "warn"
	LogLevelError SyncLogLevel = "error"
)

// SyncLog 同步日志.
type SyncLog struct {
	ID        string       `json:"id"`
	ShareID   string       `json:"share_id"`
	TaskID    string       `json:"task_id,omitempty"`
	Level     SyncLogLevel `json:"level"`
	Message   string       `json:"message"`
	FilePath  string       `json:"file_path,omitempty"`
	Error     string       `json:"error,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// ============================================================
// 请求/响应类型
// ============================================================

// CreateShareRequest 创建混合共享请求.
type CreateShareRequest struct {
	Name              string       `json:"name" binding:"required"`
	Description       string       `json:"description,omitempty"`
	Backend           CloudBackend `json:"backend" binding:"required"`
	Endpoint          string       `json:"endpoint,omitempty"`
	Region            string       `json:"region,omitempty"`
	Bucket            string       `json:"bucket" binding:"required"`
	AccessKey         string       `json:"access_key" binding:"required"`
	SecretKey         string       `json:"secret_key" binding:"required"`
	BasePath          string       `json:"base_path,omitempty"`
	LocalCachePath    string       `json:"local_cache_path" binding:"required"`
	CacheSizeBytes    int64        `json:"cache_size_bytes"`
	CachePolicy       CachePolicy  `json:"cache_policy"`
	SyncStrategy      SyncStrategy `json:"sync_strategy"`
	UploadLimitKBps   int          `json:"upload_limit_kbps,omitempty"`
	DownloadLimitKBps int          `json:"download_limit_kbps,omitempty"`
	EncryptionEnabled bool         `json:"encryption_enabled,omitempty"`
	EncryptionKey     string       `json:"encryption_key,omitempty"`
	UseSSL            bool         `json:"use_ssl,omitempty"`
}

// UpdateShareRequest 更新混合共享请求.
type UpdateShareRequest struct {
	Name              *string       `json:"name,omitempty"`
	Description       *string       `json:"description,omitempty"`
	Endpoint          *string       `json:"endpoint,omitempty"`
	Region            *string       `json:"region,omitempty"`
	Bucket            *string       `json:"bucket,omitempty"`
	AccessKey         *string       `json:"access_key,omitempty"`
	SecretKey         *string       `json:"secret_key,omitempty"`
	BasePath          *string       `json:"base_path,omitempty"`
	LocalCachePath    *string       `json:"local_cache_path,omitempty"`
	CacheSizeBytes    *int64        `json:"cache_size_bytes,omitempty"`
	CachePolicy       *CachePolicy  `json:"cache_policy,omitempty"`
	SyncStrategy      *SyncStrategy `json:"sync_strategy,omitempty"`
	SyncCronExpr      *string       `json:"sync_cron_expr,omitempty"`
	UploadLimitKBps   *int          `json:"upload_limit_kbps,omitempty"`
	DownloadLimitKBps *int          `json:"download_limit_kbps,omitempty"`
	EncryptionEnabled *bool         `json:"encryption_enabled,omitempty"`
	EncryptionKey     *string       `json:"encryption_key,omitempty"`
	Enabled           *bool         `json:"enabled,omitempty"`
}

// SyncRequest 同步请求.
type SyncRequest struct {
	ShareID   string        `json:"share_id" binding:"required"`
	FilePath  string        `json:"file_path,omitempty"` // 指定文件, 空=全部
	Direction SyncDirection `json:"direction,omitempty"` // 同步方向
	Force     bool          `json:"force,omitempty"`     // 强制同步
}

// CacheRequest 缓存操作请求.
type CacheRequest struct {
	ShareID  string `json:"share_id" binding:"required"`
	FilePath string `json:"file_path" binding:"required"` // 文件路径
	Action   string `json:"action" binding:"required"`    // pin(固定), unpin(取消固定), evict(驱逐)
}

// ============================================================
// 混合共享摘要
// ============================================================

// ShareSummary 混合共享摘要.
type ShareSummary struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Backend CloudBackend `json:"backend"`
	Bucket  string       `json:"bucket"`
	Status  string       `json:"status"`
	Enabled bool         `json:"enabled"`

	// 容量
	LocalCacheUsed  int64 `json:"local_cache_used"`
	LocalCacheTotal int64 `json:"local_cache_total"`
	CloudUsed       int64 `json:"cloud_used"`
	TotalFiles      int64 `json:"total_files"`
	CachedFiles     int64 `json:"cached_files"`
	PendingSync     int64 `json:"pending_sync"`

	// 性能
	CacheHitRate float64 `json:"cache_hit_rate"`

	LastSyncTime *time.Time `json:"last_sync_time,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// EventLog 事件日志.
type EventLog struct {
	ID        string    `json:"id"`
	ShareID   string    `json:"share_id"`
	EventType string    `json:"event_type"` // sync_start, sync_complete, cache_evict, error, etc.
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
