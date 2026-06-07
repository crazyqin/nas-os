// Package aidefrag 实现 AI 磁盘碎片整理模块
// 基于文件访问模式和热度分析，智能调度碎片整理，支持 btrfs/ext4/xfs
package aidefrag

import (
	"errors"
	"time"
)

var (
	ErrDiskNotFound     = errors.New("disk not found")
	ErrDefragRunning    = errors.New("defrag already running")
	ErrDefragNotRunning = errors.New("defrag not running")
	ErrInvalidPath      = errors.New("invalid path")
)

// FileSystemType 文件系统类型.
type FileSystemType string

const (
	FsBtrfs FileSystemType = "btrfs"
	FsExt4  FileSystemType = "ext4"
	FsXfs   FileSystemType = "xfs"
	FsZfs   FileSystemType = "zfs"
)

// DefragState 整理状态.
type DefragState string

const (
	StateIdle       DefragState = "idle"
	StateScanning   DefragState = "scanning"
	StateDefragging DefragState = "defragging"
	StatePaused     DefragState = "paused"
	StateCompleted  DefragState = "completed"
	StateFailed     DefragState = "failed"
)

// FileHeat 文件热度.
type FileHeat string

const (
	HeatHot    FileHeat = "hot"    // 频繁访问
	HeatWarm   FileHeat = "warm"   // 偶尔访问
	HeatCold   FileHeat = "cold"   // 很少访问
	HeatFrozen FileHeat = "frozen" // 从未访问
)

// DiskInfo 磁盘信息.
type DiskInfo struct {
	ID          string         `json:"id"`
	Device      string         `json:"device"`
	MountPoint  string         `json:"mount_point"`
	FileSystem  FileSystemType `json:"filesystem"`
	TotalBytes  int64          `json:"total_bytes"`
	UsedBytes   int64          `json:"used_bytes"`
	FreeBytes   int64          `json:"free_bytes"`
	FragPercent float64        `json:"frag_percent"` // 碎片率
	LastDefrag  time.Time      `json:"last_defrag"`
	NeedsDefrag bool           `json:"needs_defrag"`
}

// FileFragment 文件碎片信息.
type FileFragment struct {
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	Fragments   int       `json:"fragments"` // 碎片数
	Heat        FileHeat  `json:"heat"`
	AccessCount int64     `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
	Priority    int       `json:"priority"` // 整理优先级 0-100
}

// DefragJob 整理任务.
type DefragJob struct {
	ID               string      `json:"id"`
	DiskID           string      `json:"disk_id"`
	TargetPath       string      `json:"target_path"`
	State            DefragState `json:"state"`
	Progress         float64     `json:"progress"` // 0-100
	TotalFiles       int64       `json:"total_files"`
	ProcessedFiles   int64       `json:"processed_files"`
	TotalBytes       int64       `json:"total_bytes"`
	ProcessedBytes   int64       `json:"processed_bytes"`
	FragmentsReduced int64       `json:"fragments_reduced"`
	SpeedMBps        float64     `json:"speed_mbps"`
	ETA              string      `json:"eta"`
	StartedAt        time.Time   `json:"started_at"`
	FinishedAt       time.Time   `json:"finished_at"`
	ErrorMsg         string      `json:"error_msg"`
}

// DefragPolicy 整理策略.
type DefragPolicy struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Enabled        bool          `json:"enabled"`
	Schedule       string        `json:"schedule"`         // cron 表达式
	FragThreshold  float64       `json:"frag_threshold"`   // 碎片率阈值 %
	MinFreePercent float64       `json:"min_free_percent"` // 最低可用空间 %
	MaxDuration    time.Duration `json:"max_duration"`     // 最大持续时间
	PrioritizeHot  bool          `json:"prioritize_hot"`   // 优先整理热文件
	ExcludePaths   []string      `json:"exclude_paths"`
	CompressType   string        `json:"compress_type"`
	LastRun        time.Time     `json:"last_run"`
	NextRun        time.Time     `json:"next_run"`
	CreatedAt      time.Time     `json:"created_at"`
}

// DefragStats 统计.
type DefragStats struct {
	TotalDisks      int            `json:"total_disks"`
	NeedDefrag      int            `json:"need_defrag"`
	TotalJobs       int            `json:"total_jobs"`
	CompletedJobs   int            `json:"completed_jobs"`
	TotalFragments  int64          `json:"total_fragments_reduced"`
	TotalBytesSaved int64          `json:"total_bytes_processed"`
	AvgFragPercent  float64        `json:"avg_frag_percent"`
	FileSystemStats map[string]int `json:"filesystem_stats"`
}

// DefragConfig 配置.
type DefragConfig struct {
	ScanInterval   time.Duration `json:"scan_interval"`
	FragThreshold  float64       `json:"frag_threshold"`
	MaxConcurrent  int           `json:"max_concurrent"`
	IOLimitMBps    int           `json:"io_limit_mbps"`
	ScheduleWindow string        `json:"schedule_window"` // "02:00-06:00"
}
