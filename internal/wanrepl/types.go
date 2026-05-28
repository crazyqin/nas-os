// Package wanrepl provides WAN remote replication for multi-site data synchronization.
// Implements WAN-optimized transfer, incremental replication, and conflict resolution.
package wanrepl

import (
	"sync"
	"time"
)

// ============================================================
// 配置类型
// ============================================================

// ReplConfig 复制引擎配置
type ReplConfig struct {
	DataDir        string        `json:"data_dir"`         // 数据目录
	MaxConcurrent  int           `json:"max_concurrent"`   // 最大并发任务数，默认 4
	DefaultCompress string       `json:"default_compress"` // 默认压缩算法: "none", "zstd", "lz4", "gzip"
	TransferBufSize int          `json:"transfer_buf_size"` // 传输缓冲区大小 (bytes), 默认 4MB
	RetryAttempts   int          `json:"retry_attempts"`   // 重试次数, 默认 3
	RetryDelay      time.Duration `json:"retry_delay"`     // 重试间隔, 默认 5s
	HealthCheckSec  int          `json:"health_check_sec"` // 站点健康检查间隔 (秒), 默认 30
	LogLevel        string       `json:"log_level"`
}

// DefaultReplConfig 默认配置
func DefaultReplConfig() ReplConfig {
	return ReplConfig{
		DataDir:         "/var/lib/nas-os/wanrepl",
		MaxConcurrent:   4,
		DefaultCompress: "zstd",
		TransferBufSize: 4 * 1024 * 1024, // 4MB
		RetryAttempts:   3,
		RetryDelay:      5 * time.Second,
		HealthCheckSec:  30,
		LogLevel:        "info",
	}
}

// ============================================================
// 站点类型
// ============================================================

// SiteStatus 远程站点状态
type SiteStatus string

const (
	SiteStatusOnline  SiteStatus = "online"
	SiteStatusOffline SiteStatus = "offline"
	SiteStatusDegraded SiteStatus = "degraded"
	SiteStatusUnknown SiteStatus = "unknown"
)

// RemoteSite 远程站点
type RemoteSite struct {
	ID        string     `json:"id"`         // 站点唯一标识
	Name      string     `json:"name"`       // 站点名称
	Endpoint  string     `json:"endpoint"`   // 连接地址 (host:port)
	Status    SiteStatus `json:"status"`     // 站点状态
	LastSync  time.Time  `json:"last_sync"`  // 最后同步时间
	Bandwidth int64      `json:"bandwidth"`  // 带宽 (bytes/s)
	Latency   int64      `json:"latency"`    // 延迟 (ms)
	Tags      []string   `json:"tags,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ============================================================
// 复制任务类型
// ============================================================

// SyncStrategy 同步策略
type SyncStrategy string

const (
	StrategyFull        SyncStrategy = "full"        // 全量同步
	StrategyIncremental SyncStrategy = "incremental" // 增量同步
	StrategyMirror      SyncStrategy = "mirror"      // 镜像同步
)

// JobStatus 任务状态
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusPaused    JobStatus = "paused"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// CompressionType 压缩类型
type CompressionType string

const (
	CompressionNone CompressionType = "none"
	CompressionZstd CompressionType = "zstd"
	CompressionLz4  CompressionType = "lz4"
	CompressionGzip CompressionType = "gzip"
)

// EncryptionType 加密类型
type EncryptionType string

const (
	EncryptionNone EncryptionType = "none"
	EncryptionTLS  EncryptionType = "tls" // TLS 1.3
)

// ReplicationJob 复制任务
type ReplicationJob struct {
	ID            string          `json:"id"`             // 任务唯一标识
	Source        string          `json:"source"`         // 源路径
	Destination   string          `json:"destination"`    // 目标路径
	TargetSiteID  string          `json:"target_site_id"` // 目标站点ID
	Strategy      SyncStrategy    `json:"strategy"`       // 同步策略
	Compression   CompressionType `json:"compression"`    // 压缩算法
	Encryption    EncryptionType  `json:"encryption"`     // 加密方式
	BandwidthLimit int64          `json:"bandwidth_limit"` // 带宽限制 (bytes/s), 0=不限
	Status        JobStatus       `json:"status"`         // 任务状态
	Schedule      string          `json:"schedule,omitempty"` // cron 表达式，空=手动触发
	ExcludePatterns []string      `json:"exclude_patterns,omitempty"` // 排除模式
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	StartedAt     *time.Time      `json:"started_at,omitempty"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
}

// ============================================================
// 同步状态类型
// ============================================================

// SyncState 同步状态
type SyncState struct {
	JobID           string        `json:"job_id"`
	Progress        float64       `json:"progress"`          // 0.0 - 1.0
	BytesTransferred int64        `json:"bytes_transferred"`  // 已传输字节
	BytesRemaining   int64        `json:"bytes_remaining"`    // 剩余字节
	TotalBytes       int64        `json:"total_bytes"`        // 总字节
	Speed            int64        `json:"speed"`              // 当前速度 (bytes/s)
	AvgSpeed         int64        `json:"avg_speed"`          // 平均速度 (bytes/s)
	ETA              time.Duration `json:"eta"`               // 预计剩余时间
	CurrentFile      string       `json:"current_file"`       // 当前处理文件
	FilesTotal       int          `json:"files_total"`        // 文件总数
	FilesCompleted   int          `json:"files_completed"`    // 已完成文件数
	FilesFailed      int          `json:"files_failed"`       // 失败文件数
	StartTime        time.Time    `json:"start_time"`
	LastUpdated      time.Time    `json:"last_updated"`
}

// ============================================================
// 冲突记录类型
// ============================================================

// ConflictResolution 冲突解决策略
type ConflictResolution string

const (
	ConflictLocalWins  ConflictResolution = "local"   // 本地优先
	ConflictRemoteWins ConflictResolution = "remote"  // 远程优先
	ConflictNewest     ConflictResolution = "newest"  // 最新优先
	ConflictManual     ConflictResolution = "manual"  // 手动解决
	ConflictRename     ConflictResolution = "rename"  // 重命名保留两者
)

// ConflictRecord 冲突记录
type ConflictRecord struct {
	ID             string             `json:"id"`
	JobID          string             `json:"job_id"`
	Path           string             `json:"path"`            // 冲突文件路径
	LocalModTime   time.Time          `json:"local_mod_time"`  // 本地修改时间
	RemoteModTime  time.Time          `json:"remote_mod_time"` // 远程修改时间
	LocalSize      int64              `json:"local_size"`
	RemoteSize     int64              `json:"remote_size"`
	Resolution     ConflictResolution `json:"resolution"`      // 解决策略
	Resolved       bool               `json:"resolved"`        // 是否已解决
	ResolvedAt     *time.Time         `json:"resolved_at,omitempty"`
	DetectedAt     time.Time          `json:"detected_at"`
}

// ============================================================
// 传输统计类型
// ============================================================

// TransferStats 传输统计
type TransferStats struct {
	JobID             string        `json:"job_id"`
	TotalBytes        int64         `json:"total_bytes"`         // 总传输字节
	CompressedBytes   int64         `json:"compressed_bytes"`    // 压缩后字节
	CompressionRatio  float64       `json:"compression_ratio"`   // 压缩率
	TotalDuration     time.Duration `json:"total_duration"`      // 总耗时
	AvgSpeedBps       int64         `json:"avg_speed_bps"`       // 平均速度 (bytes/s)
	PeakSpeedBps      int64         `json:"peak_speed_bps"`      // 峰值速度
	FilesTransferred  int           `json:"files_transferred"`   // 传输文件数
	FilesSkipped      int           `json:"files_skipped"`       // 跳过文件数
	BlocksTransferred int64         `json:"blocks_transferred"`  // 传输块数
	BlocksSkipped     int64         `json:"blocks_skipped"`      // 跳过块数(未变更)
	RetryCount        int           `json:"retry_count"`         // 重试次数
	ErrorCount        int           `json:"error_count"`         // 错误次数
	ConflictCount     int           `json:"conflict_count"`      // 冲突数
	UpdatedAt         time.Time     `json:"updated_at"`
}

// ============================================================
// 块级变更追踪类型
// ============================================================

// BlockChange 块级变更记录
type BlockChange struct {
	File       string    `json:"file"`        // 文件路径
	Offset     int64     `json:"offset"`      // 块偏移
	Size       int64     `json:"size"`        // 块大小
	Checksum   string    `json:"checksum"`    // 块校验和
	ModTime    time.Time `json:"mod_time"`    // 修改时间
}

// ChangeSet 变更集
type ChangeSet struct {
	JobID    string        `json:"job_id"`
	Changes  []BlockChange `json:"changes"`
	Snapshot string        `json:"snapshot"`   // 快照标识
	Created  time.Time     `json:"created"`
}

// ============================================================
// 内部引擎结构
// ============================================================

// replicationJobState 内部任务运行状态
type replicationJobState struct {
	mu        sync.Mutex
	state     SyncState
	cancel    chan struct{}
	paused    bool
	stats     TransferStats
	conflicts []ConflictRecord
}

// ReplicationEngine 复制引擎
type ReplicationEngine struct {
	mu       sync.RWMutex
	config   ReplConfig
	sites    map[string]*RemoteSite
	jobs     map[string]*ReplicationJob
	states   map[string]*replicationJobState
	running  bool
}
