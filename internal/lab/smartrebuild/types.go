// Package smartrebuild 智能RAID重建引擎
// 参考 TrueNAS dRAID 和快速重建特性
package smartrebuild

import (
	"sync"
	"time"
)

// ========== 基础类型 ==========

// DiskStatus 磁盘状态.
type DiskStatus string

const (
	DiskStatusOnline     DiskStatus = "ONLINE"
	DiskStatusDegraded   DiskStatus = "DEGRADED"
	DiskStatusFaulted    DiskStatus = "FAULTED"
	DiskStatusRebuilding DiskStatus = "REBUILDING"
	DiskStatusSpare      DiskStatus = "SPARE"
)

// RebuildPriority 重建优先级.
type RebuildPriority int

const (
	PriorityCritical RebuildPriority = 1 // 关键数据，立即重建
	PriorityHigh     RebuildPriority = 2 // 高热度数据
	PriorityNormal   RebuildPriority = 3 // 普通数据
	PriorityLow      RebuildPriority = 4 // 低热度数据，可延迟
)

// RebuildState 重建状态.
type RebuildState string

const (
	StatePending   RebuildState = "PENDING"
	StateRunning   RebuildState = "RUNNING"
	StatePaused    RebuildState = "PAUSED"
	StateCompleted RebuildState = "COMPLETED"
	StateFailed    RebuildState = "FAILED"
	StateCancelled RebuildState = "CANCELLED"
)

// ========== 数据结构 ==========

// DiskInfo 磁盘信息.
type DiskInfo struct {
	ID         string     `json:"id"`
	Path       string     `json:"path"`
	SizeBytes  int64      `json:"size_bytes"`
	Status     DiskStatus `json:"status"`
	ReadSpeed  int64      `json:"read_speed"`  // bytes/sec
	WriteSpeed int64      `json:"write_speed"` // bytes/sec
	TempC      int        `json:"temp_c"`
	PoolName   string     `json:"pool_name"`
}

// DataSegment 数据段.
type DataSegment struct {
	ID         string          `json:"id"`
	Offset     int64           `json:"offset"`
	SizeBytes  int64           `json:"size_bytes"`
	HotScore   float64         `json:"hot_score"`  // 热度评分 0-1
	Importance float64         `json:"importance"` // 重要性评分 0-1
	Priority   RebuildPriority `json:"priority"`
}

// RebuildJob 重建任务.
type RebuildJob struct {
	ID           string        `json:"id"`
	PoolName     string        `json:"pool_name"`
	SourceDisks  []DiskInfo    `json:"source_disks"`
	TargetDisk   DiskInfo      `json:"target_disk"`
	State        RebuildState  `json:"state"`
	Progress     float64       `json:"progress"` // 0-100
	TotalBytes   int64         `json:"total_bytes"`
	RebuiltBytes int64         `json:"rebuilt_bytes"`
	CurrentSpeed int64         `json:"current_speed"` // bytes/sec
	AvgSpeed     int64         `json:"avg_speed"`     // bytes/sec
	ETA          time.Duration `json:"eta"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      *time.Time    `json:"end_time,omitempty"`
	Segments     []DataSegment `json:"segments"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// RebuildConfig 重建配置.
type RebuildConfig struct {
	MaxParallelJobs  int     `json:"max_parallel_jobs"`   // 最大并行重建数
	MaxDiskSpeedMBps int     `json:"max_disk_speed_mbps"` // 单盘最大速度限制(MB/s)
	BusinessIOWeight float64 `json:"business_io_weight"`  // 业务IO权重 (0-1)
	RebuildIOWeight  float64 `json:"rebuild_io_weight"`   // 重建IO权重 (0-1)
	SegmentSizeBytes int64   `json:"segment_size_bytes"`  // 数据段大小
	ProgressInterval int     `json:"progress_interval"`   // 进度更新间隔(秒)
	TempThreshold    int     `json:"temp_threshold"`      // 温度阈值(°C)
	PriorityBoost    bool    `json:"priority_boost"`      // 启用优先级加速
}

// ProgressSnapshot 进度快照.
type ProgressSnapshot struct {
	Timestamp      time.Time     `json:"timestamp"`
	Progress       float64       `json:"progress"`
	Speed          int64         `json:"speed"`
	ETA            time.Duration `json:"eta"`
	ActiveJobs     int           `json:"active_jobs"`
	IOUtil         float64       `json:"io_util"` // IO利用率
	RebuildIOMBps  int64         `json:"rebuild_io_mbps"`
	BusinessIOMBps int64         `json:"business_io_mbps"`
}

// IOMetrics IO指标.
type IOMetrics struct {
	ReadIOPS     int64   `json:"read_iops"`
	WriteIOPS    int64   `json:"write_iops"`
	ReadMBps     int64   `json:"read_mbps"`
	WriteMBps    int64   `json:"write_mbps"`
	Ioutil       float64 `json:"ioutil"` // 0-1
	AvgQueueSize float64 `json:"avg_queue_size"`
}

// RebuildSchedule 重建调度计划.
type RebuildSchedule struct {
	ID           string    `json:"id"`
	PoolName     string    `json:"pool_name"`
	Disks        []string  `json:"disks"`
	Strategy     string    `json:"strategy"` // "priority", "parallel", "sequential"
	MaxParallel  int       `json:"max_parallel"`
	ThrottleMBps int       `json:"throttle_mbps"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ========== Manager ==========

// Manager 智能重建管理器.
type Manager struct {
	mu          sync.RWMutex
	config      RebuildConfig
	jobs        map[string]*RebuildJob
	schedules   map[string]*RebuildSchedule
	ioMetrics   *IOMetrics
	progressLog []ProgressSnapshot
	hotDataMap  map[string]float64 // segment_id -> hot_score
}

// NewManager 创建新的智能重建管理器.
func NewManager(cfg RebuildConfig) *Manager {
	// 设置默认值
	if cfg.MaxParallelJobs <= 0 {
		cfg.MaxParallelJobs = 2
	}
	if cfg.MaxDiskSpeedMBps <= 0 {
		cfg.MaxDiskSpeedMBps = 200 // 默认200MB/s
	}
	if cfg.BusinessIOWeight <= 0 {
		cfg.BusinessIOWeight = 0.7 // 业务IO占70%
	}
	if cfg.RebuildIOWeight <= 0 {
		cfg.RebuildIOWeight = 0.3 // 重建IO占30%
	}
	if cfg.SegmentSizeBytes <= 0 {
		cfg.SegmentSizeBytes = 4 * 1024 * 1024 // 4MB
	}
	if cfg.ProgressInterval <= 0 {
		cfg.ProgressInterval = 5 // 5秒
	}
	if cfg.TempThreshold <= 0 {
		cfg.TempThreshold = 60
	}

	return &Manager{
		config:      cfg,
		jobs:        make(map[string]*RebuildJob),
		schedules:   make(map[string]*RebuildSchedule),
		ioMetrics:   &IOMetrics{},
		progressLog: make([]ProgressSnapshot, 0),
		hotDataMap:  make(map[string]float64),
	}
}

// DefaultConfig 返回默认配置.
func DefaultConfig() RebuildConfig {
	return RebuildConfig{
		MaxParallelJobs:  2,
		MaxDiskSpeedMBps: 200,
		BusinessIOWeight: 0.7,
		RebuildIOWeight:  0.3,
		SegmentSizeBytes: 4 * 1024 * 1024,
		ProgressInterval: 5,
		TempThreshold:    60,
		PriorityBoost:    true,
	}
}
