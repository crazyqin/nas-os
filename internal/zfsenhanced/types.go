// Package zfsenhanced 提供 ZFS 增强功能，对标 TrueNAS 25.04 的 OpenZFS 2.4 特性。
// 包括：RAID-Z 在线扩展、快速去重、混合存储池优化、数据完整性校验、快照管理、压缩算法选择。
package zfsenhanced

import (
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// RAID-Z 扩展
// ============================================================================

// RAIDZLevel RAID-Z 等级
type RAIDZLevel int

const (
	RAIDZ1 RAIDZLevel = iota + 1 // RAID-Z1（单奇偶校验）
	RAIDZ2                       // RAID-Z2（双奇偶校验）
	RAIDZ3                       // RAID-Z3（三奇偶校验）
)

// String 返回 RAID-Z 等级名称
func (r RAIDZLevel) String() string {
	switch r {
	case RAIDZ1:
		return "raidz1"
	case RAIDZ2:
		return "raidz2"
	case RAIDZ3:
		return "raidz3"
	default:
		return "unknown"
	}
}

// VDevState 虚拟设备状态
type VDevState string

const (
	VDevStateOnline  VDevState = "online"
	VDevStateDegraded VDevState = "degraded"
	VDevStateFaulted VDevState = "faulted"
	VDevStateExpanding VDevState = "expanding"
	VDevStateRemoved VDevState = "removed"
)

// DiskState 磁盘状态
type DiskState string

const (
	DiskStateOnline   DiskState = "online"
	DiskStateOffline  DiskState = "offline"
	DiskStateFaulted  DiskState = "faulted"
	DiskStateSpare    DiskState = "spare"
	DiskStateRemoving DiskState = "removing"
)

// VDev 虚拟设备
type VDev struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Type       RAIDZLevel  `json:"type"`
	State      VDevState   `json:"state"`
	Disks      []Disk      `json:"disks"`
	TotalBytes int64       `json:"total_bytes"`
	UsedBytes  int64       `json:"used_bytes"`
	FreeBytes  int64       `json:"free_bytes"`
	ExpandPct  float64     `json:"expand_pct"` // 扩展进度 0-100
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// Disk 磁盘设备
type Disk struct {
	ID         string     `json:"id"`
	Path       string     `json:"path"`
	State      DiskState  `json:"state"`
	SizeBytes  int64      `json:"size_bytes"`
	Serial     string     `json:"serial,omitempty"`
	Model      string     `json:"model,omitempty"`
	SlotNumber int        `json:"slot_number"`
	InsertedAt *time.Time `json:"inserted_at,omitempty"`
}

// ExpandRequest RAID-Z 在线扩展请求
type ExpandRequest struct {
	VDevID  string `json:"vdev_id"`
	DiskIDs []string `json:"disk_ids"`
}

// ExpandResult 扩展结果
type ExpandResult struct {
	Success      bool       `json:"success"`
	VDevID       string     `json:"vdev_id"`
	DisksAdded   int        `json:"disks_added"`
	NewCapacity  int64      `json:"new_capacity"`
	ExpandStatus string     `json:"expand_status"`
	StartedAt    time.Time  `json:"started_at"`
	ETA          *time.Time `json:"eta,omitempty"`
}

// ============================================================================
// 快速去重 (Fast-Dedup)
// ============================================================================

// DedupPolicy 去重策略
type DedupPolicy string

const (
	DedupPolicyNone     DedupPolicy = "none"
	DedupPolicySHA256   DedupPolicy = "sha256"
	DedupPolicyXXHash   DedupPolicy = "xxhash"
	DedupPolicyMurmur3  DedupPolicy = "murmur3"
)

// DedupEntry 去重条目
type DedupEntry struct {
	Hash      string    `json:"hash"`
	Algorithm DedupPolicy `json:"algorithm"`
	Size      int64     `json:"size"`
	RefCount  int64     `json:"ref_count"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	BlockPath string    `json:"block_path"`
}

// DedupStats 去重统计
type DedupStats struct {
	TotalBlocks     int64         `json:"total_blocks"`
	UniqueBlocks    int64         `json:"unique_blocks"`
	DedupRatio      float64       `json:"dedup_ratio"`
	SavedBytes      int64         `json:"saved_bytes"`
	HashCollisions  int64         `json:"hash_collisions"`
	ActiveEntries   int64         `json:"active_entries"`
	DedupTableSize  int64         `json:"dedup_table_size"`
	MemoryUsage     int64         `json:"memory_usage"`
	LookupLatencyMs float64       `json:"lookup_latency_ms"`
	Enabled         bool          `json:"enabled"`
	Policy          DedupPolicy   `json:"policy"`
}

// DedupConfig 去重配置
type DedupConfig struct {
	Enabled        bool        `json:"enabled"`
	Policy         DedupPolicy `json:"policy"`
	MinBlockSize   int64       `json:"min_block_size"`
	MaxBlockSize   int64       `json:"max_block_size"`
	MemLimitMB     int64       `json:"mem_limit_mb"`
	VerifyHash     bool        `json:"verify_hash"`
	SyncIntervalMs int64       `json:"sync_interval_ms"`
}

// DefaultDedupConfig 返回默认去重配置
func DefaultDedupConfig() *DedupConfig {
	return &DedupConfig{
		Enabled:        false,
		Policy:         DedupPolicySHA256,
		MinBlockSize:   4096,
		MaxBlockSize:   128 * 1024, // 128KB
		MemLimitMB:     512,
		VerifyHash:     true,
		SyncIntervalMs: 5000,
	}
}

// ============================================================================
// 混合存储池
// ============================================================================

// DeviceClass 设备类型
type DeviceClass string

const (
	DeviceClassSpecial DeviceClass = "special"
	DeviceClassDedup   DeviceClass = "dedup"
	DeviceClassLog     DeviceClass = "log"
	DeviceClassCache   DeviceClass = "cache"
	DeviceClassData    DeviceClass = "data"
)

// HybridPool 混合存储池
type HybridPool struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	State       VDevState   `json:"state"`
	DataVDevs   []VDev      `json:"data_vdevs"`
	SpecialVDev *VDev       `json:"special_vdev,omitempty"`
	LogVDev     *VDev       `json:"log_vdev,omitempty"`
	CacheVDev   *VDev       `json:"cache_vdev,omitempty"`
	TotalBytes  int64       `json:"total_bytes"`
	UsedBytes   int64       `json:"used_bytes"`
	FreeBytes   int64       `json:"free_bytes"`
	DedupRatio  float64     `json:"dedup_ratio"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// TieringPolicy 智能分层策略
type TieringPolicy struct {
	Enabled       bool    `json:"enabled"`
	HotThreshold  float64 `json:"hot_threshold"`   // 热度分数阈值
	WarmThreshold float64 `json:"warm_threshold"`
	SSDTier       bool    `json:"ssd_tier"`         // 是否启用 SSD 分层
	HDDTier       bool    `json:"hdd_tier"`         // 是否启用 HDD 分层
	AutoPromote   bool    `json:"auto_promote"`     // 自动提升
	AutoDemote    bool    `json:"auto_demote"`      // 自动降级
	BlockSize     int64   `json:"block_size"`       // 分层块大小
}

// DefaultTieringPolicy 返回默认分层策略
func DefaultTieringPolicy() *TieringPolicy {
	return &TieringPolicy{
		Enabled:       true,
		HotThreshold:  70.0,
		WarmThreshold: 30.0,
		SSDTier:       true,
		HDDTier:       true,
		AutoPromote:   true,
		AutoDemote:    true,
		BlockSize:     128 * 1024, // 128KB
	}
}

// ============================================================================
// 数据完整性
// ============================================================================

// ScrubState Scrub 状态
type ScrubState string

const (
	ScrubStateIdle     ScrubState = "idle"
	ScrubStateRunning  ScrubState = "running"
	ScrubStatePaused   ScrubState = "paused"
	ScrubStateFinished ScrubState = "finished"
	ScrubStateFailed   ScrubState = "failed"
)

// ScrubConfig Scrub 配置
type ScrubConfig struct {
	Enabled          bool          `json:"enabled"`
	IntervalDays     int           `json:"interval_days"`
	ThrottleIOPS     int           `json:"throttle_iops"`
	ThrottleMBps     int           `json:"throttle_mbps"`
	AutoRepair       bool          `json:"auto_repair"`
	RepairRetries    int           `json:"repair_retries"`
	PriorityClass    string        `json:"priority_class"`
	ScheduleHour     int           `json:"schedule_hour"`
	ScheduleMinute   int           `json:"schedule_minute"`
}

// DefaultScrubConfig 返回默认 Scrub 配置
func DefaultScrubConfig() *ScrubConfig {
	return &ScrubConfig{
		Enabled:        true,
		IntervalDays:   14,
		ThrottleIOPS:   1000,
		ThrottleMBps:   200,
		AutoRepair:     true,
		RepairRetries:  3,
		PriorityClass:  "idle",
		ScheduleHour:   2,
		ScheduleMinute: 0,
	}
}

// ScrubStatus Scrub 状态
type ScrubStatus struct {
	State          ScrubState  `json:"state"`
	StartTime      *time.Time  `json:"start_time,omitempty"`
	EndTime        *time.Time  `json:"end_time,omitempty"`
	Progress       float64     `json:"progress"`        // 0-100
	BytesScanned   int64       `json:"bytes_scanned"`
	BytesTotal     int64       `json:"bytes_total"`
	ErrorsFound    int64       `json:"errors_found"`
	ErrorsRepaired int64       `json:"errors_repaired"`
	ErrorsUnrepaired int64     `json:"errors_unrepaired"`
	ScanRate       float64     `json:"scan_rate"`        // MB/s
	ETA            *time.Time  `json:"eta,omitempty"`
}

// Checksum 校验和
type Checksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
	Size      int64  `json:"size"`
}

// ============================================================================
// 快照管理
// ============================================================================

// Snapshot 快照
type Snapshot struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Pool       string    `json:"pool"`
	Dataset    string    `json:"dataset"`
	FullName   string    `json:"full_name"` // pool/dataset@name
	SizeBytes  int64     `json:"size_bytes"`
	UsedBytes  int64     `json:"used_bytes"`
	ReferBytes int64     `json:"refer_bytes"`
	Clones     []string  `json:"clones,omitempty"`
	IsClone    bool      `json:"is_clone"`
	Origin     string    `json:"origin,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// SnapshotCreateRequest 创建快照请求
type SnapshotCreateRequest struct {
	Pool      string `json:"pool"`
	Dataset   string `json:"dataset"`
	Name      string `json:"name"`
	Recursive bool   `json:"recursive"`
}

// CloneRequest 克隆请求
type CloneRequest struct {
	SnapshotID  string `json:"snapshot_id"`
	TargetPool  string `json:"target_pool"`
	TargetName  string `json:"target_name"`
}

// RollbackRequest 回滚请求
type RollbackRequest struct {
	SnapshotID string `json:"snapshot_id"`
	Force      bool   `json:"force"`
}

// SnapshotStats 快照统计
type SnapshotStats struct {
	TotalSnapshots  int64   `json:"total_snapshots"`
	TotalUsedBytes  int64   `json:"total_used_bytes"`
	TotalReferBytes int64   `json:"total_refer_bytes"`
	OldestSnapshot  string  `json:"oldest_snapshot"`
	NewestSnapshot  string  `json:"newest_snapshot"`
	CloneCount      int64   `json:"clone_count"`
}

// ============================================================================
// 压缩算法
// ============================================================================

// CompressionAlgorithm 压缩算法
type CompressionAlgorithm string

const (
	CompressionLZ4   CompressionAlgorithm = "lz4"
	CompressionZSTD  CompressionAlgorithm = "zstd"
	CompressionZLE   CompressionAlgorithm = "zle"
	CompressionOff   CompressionAlgorithm = "off"
)

// String 返回压缩算法名称
func (c CompressionAlgorithm) String() string {
	return string(c)
}

// CompressionLevel 压缩级别
type CompressionLevel int

const (
	CompressionLevelFast CompressionLevel = 1    // 快速压缩
	CompressionLevelBalanced CompressionLevel = 3 // 平衡
	CompressionLevelBest CompressionLevel = 9    // 最佳压缩
)

// CompressionStats 压缩统计
type CompressionStats struct {
	Algorithm      CompressionAlgorithm `json:"algorithm"`
	Level          CompressionLevel     `json:"level"`
	OriginalBytes  int64                `json:"original_bytes"`
	CompressedBytes int64               `json:"compressed_bytes"`
	Ratio          float64              `json:"ratio"`
	SpeedMBps      float64              `json:"speed_mbps"`
	DecompressMBps float64              `json:"decompress_mbps"`
}

// CompressionConfig 压缩配置
type CompressionConfig struct {
	DefaultAlgo    CompressionAlgorithm `json:"default_algo"`
	DefaultLevel   CompressionLevel     `json:"default_level"`
	AdaptiveMode   bool                 `json:"adaptive_mode"`  // 自适应模式
	MinRatio       float64              `json:"min_ratio"`      // 最小压缩比阈值
	MaxCPUUsage    int                  `json:"max_cpu_usage"`  // 最大 CPU 使用率
}

// DefaultCompressionConfig 返回默认压缩配置
func DefaultCompressionConfig() *CompressionConfig {
	return &CompressionConfig{
		DefaultAlgo:  CompressionLZ4,
		DefaultLevel: CompressionLevelBalanced,
		AdaptiveMode: true,
		MinRatio:     1.1,
		MaxCPUUsage:  50,
	}
}

// ============================================================================
// 池管理
// ============================================================================

// Pool ZFS 存储池
type Pool struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	State        VDevState       `json:"state"`
	VDevs        []VDev          `json:"vdevs"`
	HybridPool   *HybridPool     `json:"hybrid_pool,omitempty"`
	TotalBytes   int64           `json:"total_bytes"`
	UsedBytes    int64           `json:"used_bytes"`
	FreeBytes    int64           `json:"free_bytes"`
	FragmentPct  float64         `json:"fragment_pct"`
	DedupRatio   float64         `json:"dedup_ratio"`
	Compression  CompressionStats `json:"compression"`
	ScrubStatus  ScrubStatus     `json:"scrub_status"`
	SnapshotCount int64          `json:"snapshot_count"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// PoolConfig 池配置
type PoolConfig struct {
	Name           string               `json:"name"`
	Compression    CompressionAlgorithm `json:"compression"`
	CompressionLvl CompressionLevel     `json:"compression_level"`
	DedupPolicy    DedupPolicy          `json:"dedup_policy"`
	TieringPolicy  *TieringPolicy       `json:"tiering_policy,omitempty"`
	ScrubConfig    *ScrubConfig         `json:"scrub_config,omitempty"`
	AutoExpand     bool                 `json:"auto_expand"`
	Ashift         int                  `json:"ashift"` // 扇区大小指数 (9=512B, 12=4K)
}

// DefaultPoolConfig 返回默认池配置
func DefaultPoolConfig(name string) *PoolConfig {
	return &PoolConfig{
		Name:           name,
		Compression:    CompressionLZ4,
		CompressionLvl: CompressionLevelBalanced,
		DedupPolicy:    DedupPolicyNone,
		TieringPolicy:  DefaultTieringPolicy(),
		ScrubConfig:    DefaultScrubConfig(),
		AutoExpand:     true,
		Ashift:         12, // 4K 扇区
	}
}

// ============================================================================
// 引擎统计
// ============================================================================

// EngineStats 引擎总体统计
type EngineStats struct {
	Pools          int              `json:"pools"`
	VDevs          int              `json:"vdevs"`
	TotalCapacity  int64            `json:"total_capacity"`
	TotalUsed      int64            `json:"total_used"`
	TotalFree      int64            `json:"total_free"`
	ScrubStatus    ScrubStatus      `json:"scrub_status"`
	DedupStats     DedupStats       `json:"dedup_stats"`
	SnapshotStats  SnapshotStats    `json:"snapshot_stats"`
	Compression    CompressionStats `json:"compression"`
	LastScrubTime  *time.Time       `json:"last_scrub_time,omitempty"`
	NextScrubTime  *time.Time       `json:"next_scrub_time,omitempty"`
}

// ============================================================================
// 错误定义
// ============================================================================

var (
	ErrPoolNotFound       = fmt.Errorf("pool not found")
	ErrVDevNotFound       = fmt.Errorf("vdev not found")
	ErrDiskNotFound       = fmt.Errorf("disk not found")
	ErrSnapshotNotFound   = fmt.Errorf("snapshot not found")
	ErrCloneNotFound      = fmt.Errorf("clone not found")
	ErrDiskAlreadyInUse   = fmt.Errorf("disk already in use")
	ErrInvalidRAIDZLevel  = fmt.Errorf("invalid raidz level")
	ErrPoolExists         = fmt.Errorf("pool already exists")
	ErrScrubAlreadyRunning = fmt.Errorf("scrub already running")
	ErrNoDataDisks        = fmt.Errorf("no data disks specified")
	ErrInsufficientDisks  = fmt.Errorf("insufficient disks for raidz level")
	ErrSnapshotHasClones  = fmt.Errorf("snapshot has clones, force required")
	ErrReadOnlySnapshot   = fmt.Errorf("cannot modify read-only snapshot")
	ErrDedupDisabled      = fmt.Errorf("dedup is disabled")
	ErrCompressionFailed  = fmt.Errorf("compression failed")
	ErrDecompressFailed   = fmt.Errorf("decompression failed")
	ErrInvalidHash        = fmt.Errorf("invalid hash")
	ErrChecksumMismatch   = fmt.Errorf("checksum mismatch")
	ErrPoolReadOnly       = fmt.Errorf("pool is read-only")
)

// ============================================================================
// 后端接口
// ============================================================================

// StorageBackend 存储后端接口
type StorageBackend interface {
	// 池操作
	CreatePool(config *PoolConfig, disks []Disk) (*Pool, error)
	DestroyPool(poolID string) error
	GetPool(poolID string) (*Pool, error)
	ListPools() ([]*Pool, error)

	// VDev 操作
	AddVDev(poolID string, vdev *VDev) error
	RemoveVDev(poolID string, vdevID string) error
	AttachDisk(vdevID string, disk Disk) error
	DetachDisk(vdevID string, diskID string) error
	ExpandVDev(vdevID string, disks []Disk) error

	// 快照操作
	CreateSnapshot(req *SnapshotCreateRequest) (*Snapshot, error)
	DeleteSnapshot(snapshotID string) error
	RollbackSnapshot(req *RollbackRequest) error
	CloneSnapshot(req *CloneRequest) (*Snapshot, error)
	ListSnapshots(poolID string) ([]*Snapshot, error)

	// Scrub
	StartScrub(poolID string) error
	StopScrub(poolID string) error
	GetScrubStatus(poolID string) (*ScrubStatus, error)

	// 去重
	EnableDedup(poolID string, config *DedupConfig) error
	DisableDedup(poolID string) error
	GetDedupStats(poolID string) (*DedupStats, error)

	// 压缩
	SetCompression(poolID string, algo CompressionAlgorithm, level CompressionLevel) error
	GetCompressionStats(poolID string) (*CompressionStats, error)
}

// ============================================================================
// 引擎
// ============================================================================

// Engine ZFS 增强功能引擎
type Engine struct {
	mu     sync.RWMutex
	config *EngineConfig
	logger Logger

	backend StorageBackend

	pools    map[string]*Pool
	snapshots map[string]*Snapshot
	dedupTable map[string]*DedupEntry

	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// EngineConfig 引擎配置
type EngineConfig struct {
	Dedup        *DedupConfig        `json:"dedup,omitempty"`
	Compression  *CompressionConfig  `json:"compression,omitempty"`
	Scrub        *ScrubConfig        `json:"scrub,omitempty"`
	Tiering      *TieringPolicy      `json:"tiering,omitempty"`
	AutoExpand   bool                `json:"auto_expand"`
}

// DefaultEngineConfig 返回默认引擎配置
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		Dedup:       DefaultDedupConfig(),
		Compression: DefaultCompressionConfig(),
		Scrub:       DefaultScrubConfig(),
		Tiering:     DefaultTieringPolicy(),
		AutoExpand:  true,
	}
}

// Logger 日志接口
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// nopLogger 空日志实现
type nopLogger struct{}

func (n *nopLogger) Info(msg string, keysAndValues ...interface{})  {}
func (n *nopLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (n *nopLogger) Error(msg string, keysAndValues ...interface{}) {}
func (n *nopLogger) Debug(msg string, keysAndValues ...interface{}) {}
