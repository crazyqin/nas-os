// Package cdp 持续数据保护 - 实时捕获文件变更，支持任意时间点恢复，RPO趋近于零
package cdp

import (
	"errors"
	"time"
)

// EventType 变更事件类型
type EventType string

const (
	EventCreate EventType = "create" // 文件创建
	EventModify EventType = "modify" // 文件修改
	EventDelete EventType = "delete" // 文件删除
	EventRename EventType = "rename" // 文件重命名
)

// StorageType 存储后端类型
type StorageType string

const (
	StorageLocal  StorageType = "local"  // 本地存储
	StorageRemote StorageType = "remote" // 远程存储
	StorageS3     StorageType = "s3"     // S3对象存储
	StorageOSS    StorageType = "oss"    // 阿里云OSS
	StorageMinIO  StorageType = "minio"  // MinIO对象存储
)

// CompressionType 压缩类型
type CompressionType string

const (
	CompressNone   CompressionType = "none"   // 不压缩
	CompressGzip   CompressionType = "gzip"   // Gzip压缩
	CompressLZ4    CompressionType = "lz4"    // LZ4压缩
	CompressZstd   CompressionType = "zstd"   // Zstandard压缩
	CompressSnappy CompressionType = "snappy" // Snappy压缩
)

// ReplicationMode 复制模式
type ReplicationMode string

const (
	ReplicationSync  ReplicationMode = "sync"  // 同步复制
	ReplicationAsync ReplicationMode = "async" // 异步复制
	ReplicationSemi  ReplicationMode = "semi"  // 半同步复制
)

// RetentionMode 保留策略模式
type RetentionMode string

const (
	RetentionByTime  RetentionMode = "time"  // 按时间保留
	RetentionByCount RetentionMode = "count" // 按数量保留
	RetentionBySize  RetentionMode = "size"  // 按空间保留
	RetentionSmart   RetentionMode = "smart" // 智能保留（综合策略）
)

// ChangeEvent 变更事件
type ChangeEvent struct {
	ID           string            `json:"id"`                  // 事件唯一标识
	EventType    EventType         `json:"event_type"`          // 事件类型
	FilePath     string            `json:"file_path"`           // 文件路径
	OldPath      string            `json:"old_path,omitempty"`  // 旧路径（重命名时使用）
	NewPath      string            `json:"new_path,omitempty"`  // 新路径（重命名时使用）
	Size         int64             `json:"size"`                // 文件大小
	ModTime      time.Time         `json:"mod_time"`            // 修改时间
	Checksum     string            `json:"checksum"`            // 内容校验和
	BlockRef     string            `json:"block_ref,omitempty"` // 数据块引用
	Compressed   bool              `json:"compressed"`          // 是否已压缩
	Deduplicated bool              `json:"deduplicated"`        // 是否已去重
	Metadata     map[string]string `json:"metadata,omitempty"`  // 扩展元数据
	Timestamp    time.Time         `json:"timestamp"`           // 事件发生时间
}

// RecoveryPoint 恢复点（精确到秒级）
type RecoveryPoint struct {
	ID             string            `json:"id"`                  // 恢复点唯一标识
	Timestamp      time.Time         `json:"timestamp"`           // 恢复点时间
	SequenceNum    uint64            `json:"sequence_num"`        // 序列号
	Events         []*ChangeEvent    `json:"events"`              // 包含的变更事件
	TotalSize      int64             `json:"total_size"`          // 总数据大小
	CompressedSize int64             `json:"compressed_size"`     // 压缩后大小
	ParentID       string            `json:"parent_id,omitempty"` // 父恢复点ID
	Checksum       string            `json:"checksum"`            // 完整性校验和
	Consistent     bool              `json:"consistent"`          // 是否一致
	Metadata       map[string]string `json:"metadata,omitempty"`  // 扩展元数据
}

// CDPolicy 保护策略（文件类型过滤、目录白名单、大小限制）
type CDPolicy struct {
	ID                 string          `json:"id"`                    // 策略ID
	Name               string          `json:"name"`                  // 策略名称
	Description        string          `json:"description,omitempty"` // 策略描述
	Enabled            bool            `json:"enabled"`               // 是否启用
	FilePatterns       []string        `json:"file_patterns"`         // 文件匹配模式（glob）
	ExcludePatterns    []string        `json:"exclude_patterns"`      // 排除模式
	DirectoryWhitelist []string        `json:"directory_whitelist"`   // 目录白名单
	DirectoryBlacklist []string        `json:"directory_blacklist"`   // 目录黑名单
	MaxFileSize        int64           `json:"max_file_size"`         // 最大文件大小限制
	MinFileSize        int64           `json:"min_file_size"`         // 最小文件大小限制
	TrackCreate        bool            `json:"track_create"`          // 跟踪创建事件
	TrackModify        bool            `json:"track_modify"`          // 跟踪修改事件
	TrackDelete        bool            `json:"track_delete"`          // 跟踪删除事件
	TrackRename        bool            `json:"track_rename"`          // 跟踪重命名事件
	Compression        CompressionType `json:"compression"`           // 压缩类型
	Deduplication      bool            `json:"deduplication"`         // 是否启用去重
	Priority           int             `json:"priority"`              // 策略优先级
	CreatedAt          time.Time       `json:"created_at"`            // 创建时间
	UpdatedAt          time.Time       `json:"updated_at"`            // 更新时间
}

// StorageBackend 存储后端（本地/远程/对象存储）
type StorageBackend struct {
	ID          string            `json:"id"`                   // 后端ID
	Name        string            `json:"name"`                 // 后端名称
	Type        StorageType       `json:"type"`                 // 存储类型
	Endpoint    string            `json:"endpoint,omitempty"`   // 端点地址
	Bucket      string            `json:"bucket,omitempty"`     // 存储桶
	AccessKey   string            `json:"access_key,omitempty"` // 访问密钥ID
	SecretKey   string            `json:"secret_key,omitempty"` // 访问密钥
	Region      string            `json:"region,omitempty"`     // 区域
	BasePath    string            `json:"base_path"`            // 基础路径
	MaxSize     int64             `json:"max_size"`             // 最大容量
	CurrentSize int64             `json:"current_size"`         // 当前使用量
	Enabled     bool              `json:"enabled"`              // 是否启用
	TLS         bool              `json:"tls"`                  // 是否使用TLS
	Timeout     int               `json:"timeout"`              // 超时时间（秒）
	RetryCount  int               `json:"retry_count"`          // 重试次数
	Metadata    map[string]string `json:"metadata,omitempty"`   // 扩展配置
}

// RetentionManager 保留策略（按时间/数量/空间）
type RetentionManager struct {
	ID              string        `json:"id"`               // 保留策略ID
	Name            string        `json:"name"`             // 策略名称
	Mode            RetentionMode `json:"mode"`             // 保留模式
	MaxAge          time.Duration `json:"max_age"`          // 最大保留时间
	MaxCount        int           `json:"max_count"`        // 最大保留数量
	MaxSize         int64         `json:"max_size"`         // 最大保留空间（字节）
	MinRetain       int           `json:"min_retain"`       // 最少保留数量
	CleanupInterval time.Duration `json:"cleanup_interval"` // 清理间隔
	LastCleanup     time.Time     `json:"last_cleanup"`     // 上次清理时间
	Enabled         bool          `json:"enabled"`          // 是否启用
}

// PointInTimeRecovery 时间点恢复
type PointInTimeRecovery struct {
	ID            string           `json:"id"`                       // 恢复任务ID
	TargetTime    time.Time        `json:"target_time"`              // 目标恢复时间点
	SourcePath    string           `json:"source_path"`              // 源路径
	TargetPath    string           `json:"target_path"`              // 目标恢复路径
	RecoveryPoint *RecoveryPoint   `json:"recovery_point,omitempty"` // 关联的恢复点
	Status        RecoveryStatus   `json:"status"`                   // 恢复状态
	Progress      float64          `json:"progress"`                 // 恢复进度（0-100）
	TotalFiles    int              `json:"total_files"`              // 总文件数
	RestoredFiles int              `json:"restored_files"`           // 已恢复文件数
	TotalBytes    int64            `json:"total_bytes"`              // 总字节数
	RestoredBytes int64            `json:"restored_bytes"`           // 已恢复字节数
	StartTime     time.Time        `json:"start_time"`               // 开始时间
	EndTime       time.Time        `json:"end_time,omitempty"`       // 结束时间
	Error         string           `json:"error,omitempty"`          // 错误信息
	Options       *RecoveryOptions `json:"options,omitempty"`        // 恢复选项
}

// RecoveryStatus 恢复状态
type RecoveryStatus string

const (
	RecoveryPending    RecoveryStatus = "pending"     // 待执行
	RecoveryInProgress RecoveryStatus = "in_progress" // 进行中
	RecoveryCompleted  RecoveryStatus = "completed"   // 已完成
	RecoveryFailed     RecoveryStatus = "failed"      // 失败
	RecoveryCancelled  RecoveryStatus = "cancelled"   // 已取消
)

// RecoveryOptions 恢复选项
type RecoveryOptions struct {
	Overwrite    bool     `json:"overwrite"`               // 是否覆盖已存在文件
	PreserveACL  bool     `json:"preserve_acl"`            // 保留ACL权限
	PreserveTime bool     `json:"preserve_time"`           // 保留时间戳
	TargetFiles  []string `json:"target_files,omitempty"`  // 指定恢复文件列表
	ExcludeFiles []string `json:"exclude_files,omitempty"` // 排除文件列表
	DryRun       bool     `json:"dry_run"`                 // 干跑模式（只验证不恢复）
	VerifyOnly   bool     `json:"verify_only"`             // 仅验证完整性
}

// ReplicationManager 异地复制管理
type ReplicationManager struct {
	ID             string            `json:"id"`              // 复制管理器ID
	Name           string            `json:"name"`            // 名称
	Mode           ReplicationMode   `json:"mode"`            // 复制模式
	Source         *StorageBackend   `json:"source"`          // 源存储后端
	Targets        []*StorageBackend `json:"targets"`         // 目标存储后端
	Enabled        bool              `json:"enabled"`         // 是否启用
	Status         ReplicationStatus `json:"status"`          // 复制状态
	LastSync       time.Time         `json:"last_sync"`       // 上次同步时间
	PendingBytes   int64             `json:"pending_bytes"`   // 待复制字节数
	SyncInterval   time.Duration     `json:"sync_interval"`   // 同步间隔
	BandwidthLimit int64             `json:"bandwidth_limit"` // 带宽限制（字节/秒）
	CompressData   bool              `json:"compress_data"`   // 传输时压缩
	EncryptData    bool              `json:"encrypt_data"`    // 传输时加密
}

// ReplicationStatus 复制状态
type ReplicationStatus string

const (
	ReplicationIdle    ReplicationStatus = "idle"    // 空闲
	ReplicationSyncing ReplicationStatus = "syncing" // 同步中
	ReplicationError   ReplicationStatus = "error"   // 错误
	ReplicationPaused  ReplicationStatus = "paused"  // 暂停
)

// DedupStats 去重统计
type DedupStats struct {
	TotalChunks     int64   `json:"total_chunks"`     // 总数据块数
	UniqueChunks    int64   `json:"unique_chunks"`    // 唯一数据块数
	DuplicateChunks int64   `json:"duplicate_chunks"` // 重复数据块数
	TotalSize       int64   `json:"total_size"`       // 总大小
	DedupSize       int64   `json:"dedup_size"`       // 去重后大小
	SpaceSaved      int64   `json:"space_saved"`      // 节省空间
	DedupRatio      float64 `json:"dedup_ratio"`      // 去重率
}

// PerformanceStats 性能统计
type PerformanceStats struct {
	EventsPerSecond float64       `json:"events_per_second"` // 每秒事件数
	BytesPerSecond  int64         `json:"bytes_per_second"`  // 每秒字节数
	AvgLatency      time.Duration `json:"avg_latency"`       // 平均延迟
	MaxLatency      time.Duration `json:"max_latency"`       // 最大延迟
	ActiveMonitors  int           `json:"active_monitors"`   // 活跃监控数
	TotalEvents     int64         `json:"total_events"`      // 总事件数
	TotalBytes      int64         `json:"total_bytes"`       // 总字节数
	QueueDepth      int           `json:"queue_depth"`       // 队列深度
	ErrorCount      int64         `json:"error_count"`       // 错误数
	LastEventTime   time.Time     `json:"last_event_time"`   // 最后事件时间
}

// CDPEngine CDP引擎
type CDPEngine struct {
	ID              string              `json:"id"`                    // 引擎ID
	Name            string              `json:"name"`                  // 引擎名称
	Config          *EngineConfig       `json:"config"`                // 引擎配置
	Policies        []*CDPolicy         `json:"policies"`              // 保护策略列表
	Backends        []*StorageBackend   `json:"backends"`              // 存储后端列表
	Retention       *RetentionManager   `json:"retention"`             // 保留策略
	Replication     *ReplicationManager `json:"replication,omitempty"` // 复制管理器
	Stats           *PerformanceStats   `json:"stats"`                 // 性能统计
	DedupStats      *DedupStats         `json:"dedup_stats"`           // 去重统计
	Status          EngineStatus        `json:"status"`                // 引擎状态
	StartTime       time.Time           `json:"start_time"`            // 启动时间
	LastHealthCheck time.Time           `json:"last_health_check"`     // 上次健康检查
}

// EngineConfig 引擎配置
type EngineConfig struct {
	WatchPaths          []string        `json:"watch_paths"`           // 监控路径列表
	ExcludePaths        []string        `json:"exclude_paths"`         // 排除路径列表
	BatchSize           int             `json:"batch_size"`            // 批处理大小
	FlushInterval       time.Duration   `json:"flush_interval"`        // 刷新间隔
	MaxQueueSize        int             `json:"max_queue_size"`        // 最大队列大小
	WorkerCount         int             `json:"worker_count"`          // 工作线程数
	EnableDedup         bool            `json:"enable_dedup"`          // 启用去重
	EnableCompress      bool            `json:"enable_compress"`       // 启用压缩
	CompressionType     CompressionType `json:"compression_type"`      // 压缩类型
	BlockSize           int             `json:"block_size"`            // 数据块大小
	IndexEnabled        bool            `json:"index_enabled"`         // 启用索引
	ChecksumAlgo        string          `json:"checksum_algo"`         // 校验算法
	HealthCheckInterval time.Duration   `json:"health_check_interval"` // 健康检查间隔
}

// EngineStatus 引擎状态
type EngineStatus string

const (
	EngineStatusRunning EngineStatus = "running" // 运行中
	EngineStatusStopped EngineStatus = "stopped" // 已停止
	EngineStatusError   EngineStatus = "error"   // 错误
	EngineStatusPaused  EngineStatus = "paused"  // 暂停
)

// 预定义错误
var (
	ErrEngineNotRunning      = errors.New("engine is not running")
	ErrEngineAlreadyRunning  = errors.New("engine is already running")
	ErrPolicyNotFound        = errors.New("policy not found")
	ErrPolicyExists          = errors.New("policy already exists")
	ErrBackendNotFound       = errors.New("storage backend not found")
	ErrBackendExists         = errors.New("storage backend already exists")
	ErrBackendFull           = errors.New("storage backend is full")
	ErrRecoveryPointNotFound = errors.New("recovery point not found")
	ErrRecoveryFailed        = errors.New("recovery failed")
	ErrRecoveryInProgress    = errors.New("recovery already in progress")
	ErrInvalidTimestamp      = errors.New("invalid timestamp")
	ErrInvalidPath           = errors.New("invalid path")
	ErrFileTooLarge          = errors.New("file exceeds size limit")
	ErrFileExcluded          = errors.New("file is excluded by policy")
	ErrDedupFailed           = errors.New("deduplication failed")
	ErrCompressionFailed     = errors.New("compression failed")
	ErrReplicationFailed     = errors.New("replication failed")
	ErrQueueFull             = errors.New("event queue is full")
	ErrTimeout               = errors.New("operation timed out")
)
