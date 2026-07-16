package hdddedup

import (
	"sync"
	"time"
)

// DedupJobStatus 去重任务状态.
type DedupJobStatus string

const (
	JobStatusPending   DedupJobStatus = "pending"
	JobStatusRunning   DedupJobStatus = "running"
	JobStatusCompleted DedupJobStatus = "completed"
	JobStatusFailed    DedupJobStatus = "failed"
	JobStatusCancelled DedupJobStatus = "cancelled"
)

// CompressAlgorithm 压缩算法.
type CompressAlgorithm string

const (
	AlgoLZ4  CompressAlgorithm = "lz4"
	AlgoZSTD CompressAlgorithm = "zstd"
	AlgoGZIP CompressAlgorithm = "gzip"
)

// HDDDedupConfig 去重压缩配置.
type HDDDedupConfig struct {
	ChunkSize       int               `json:"chunkSize"`       // 数据块大小
	CompressAlgo    CompressAlgorithm `json:"compressAlgo"`    // 压缩算法
	CompressLevel   int               `json:"compressLevel"`   // 压缩级别
	MaxConcurrency  int               `json:"maxConcurrency"`  // 最大并发数
	ScheduleEnabled bool              `json:"scheduleEnabled"` // 启用调度
	RetentionDays   int               `json:"retentionDays"`   // 报告保留天数
}

// DefaultHDDDedupConfig 默认配置.
func DefaultHDDDedupConfig() *HDDDedupConfig {
	return &HDDDedupConfig{
		ChunkSize:       64 * 1024,
		CompressAlgo:    AlgoZSTD,
		CompressLevel:   3,
		MaxConcurrency:  4,
		ScheduleEnabled: true,
		RetentionDays:   90,
	}
}

// DedupJob 去重任务.
type DedupJob struct {
	ID         string         `json:"id"`
	TargetPath string         `json:"targetPath"`
	Status     DedupJobStatus `json:"status"`
	Progress   float64        `json:"progress"` // 0-100
	TotalFiles int64          `json:"totalFiles"`
	Processed  int64          `json:"processed"`
	DedupCount int64          `json:"dedupCount"` // 去重的块数
	SavedBytes int64          `json:"savedBytes"` // 节省的字节数
	StartTime  time.Time      `json:"startTime"`
	EndTime    *time.Time     `json:"endTime,omitempty"`
	ErrorMsg   string         `json:"errorMsg,omitempty"`
}

// CompressPolicy 压缩策略.
type CompressPolicy struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	PathPattern string            `json:"pathPattern"` // 路径匹配模式
	Algorithm   CompressAlgorithm `json:"algorithm"`
	Level       int               `json:"level"`
	MinSize     int64             `json:"minSize"`    // 最小文件大小
	Extensions  []string          `json:"extensions"` // 文件扩展名过滤
	Enabled     bool              `json:"enabled"`
}

// DedupSchedule 去重调度.
type DedupSchedule struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	TargetPath string     `json:"targetPath"`
	CronExpr   string     `json:"cronExpr"` // Cron 表达式
	Enabled    bool       `json:"enabled"`
	LastRun    *time.Time `json:"lastRun,omitempty"`
	NextRun    *time.Time `json:"nextRun,omitempty"`
}

// EfficiencyReport 效率报告.
type EfficiencyReport struct {
	ID             string    `json:"id"`
	GeneratedAt    time.Time `json:"generatedAt"`
	TotalData      int64     `json:"totalData"`      // 总数据量
	DedupedData    int64     `json:"dedupedData"`    // 去重后数据量
	CompressedData int64     `json:"compressedData"` // 压缩后数据量
	DedupRatio     float64   `json:"dedupRatio"`     // 去重率
	CompressRatio  float64   `json:"compressRatio"`  // 压缩率
	TotalSaved     int64     `json:"totalSaved"`     // 总节省空间
	ChunkCount     int64     `json:"chunkCount"`     // 数据块数
	UniqueChunks   int64     `json:"uniqueChunks"`   // 唯一块数
	FilePath       string    `json:"filePath"`       // 报告文件路径
}

// DedupChunk 去重数据块.
type DedupChunk struct {
	Hash       string `json:"hash"`       // 内容哈希
	Size       int    `json:"size"`       // 块大小
	RefCount   int    `json:"refCount"`   // 引用计数
	Compressed bool   `json:"compressed"` // 是否已压缩
}

// Engine 去重压缩引擎.
type Engine struct {
	mu         sync.RWMutex
	config     *HDDDedupConfig
	jobs       map[string]*DedupJob
	policies   map[string]*CompressPolicy
	schedules  map[string]*DedupSchedule
	reports    []*EfficiencyReport
	chunkIndex map[string]*DedupChunk // 哈希 -> 数据块
	running    bool
	stopCh     chan struct{}
}
