// Package storageefficiency 提供 HDD 存储效率增强功能
// 支持数据去重、压缩、存储优化，对标群晖 DSM 7.4 Storage Efficiency
package storageefficiency

import (
	"fmt"
	"time"
)

// ========== 效率策略 ==========

// EfficiencyStrategy 效率策略
type EfficiencyStrategy string

const (
	// StrategyDedup 去重
	StrategyDedup EfficiencyStrategy = "dedup"
	// StrategyCompress 压缩
	StrategyCompress EfficiencyStrategy = "compress"
	// StrategyBoth 去重+压缩
	StrategyBoth EfficiencyStrategy = "both"
	// StrategyNone 无
	StrategyNone EfficiencyStrategy = "none"
)

// CompressionAlgorithm 压缩算法
type CompressionAlgorithm string

const (
	AlgoLZ4    CompressionAlgorithm = "lz4"
	AlgoZSTD   CompressionAlgorithm = "zstd"
	AlgoGZIP   CompressionAlgorithm = "gzip"
	AlgoBZIP2  CompressionAlgorithm = "bzip2"
	AlgoSnappy CompressionAlgorithm = "snappy"
)

// DedupMode 去重模式
type DedupMode string

const (
	DedupInline  DedupMode = "inline"  // 实时去重
	DedupPost    DedupMode = "post"    // 后处理去重
	DedupHybrid  DedupMode = "hybrid"  // 混合模式
)

// ========== 配置类型 ==========

// EfficiencyConfig 效率配置
type EfficiencyConfig struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Strategy       EfficiencyStrategy   `json:"strategy"`
	Compression    CompressionAlgorithm `json:"compression"`
	CompressionLevel int                `json:"compressionLevel"`
	DedupMode      DedupMode            `json:"dedupMode"`
	ChunkSizeKB    int                  `json:"chunkSizeKB"`
	MinFileSizeKB  int                  `json:"minFileSizeKB"`
	MaxFileSizeGB  int                  `json:"maxFileSizeGB"`
	Enabled        bool                 `json:"enabled"`
	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
}

// CreateConfigRequest 创建配置请求
type CreateConfigRequest struct {
	Name             string               `json:"name" binding:"required"`
	Strategy         EfficiencyStrategy   `json:"strategy" binding:"required"`
	Compression      CompressionAlgorithm `json:"compression"`
	CompressionLevel int                  `json:"compressionLevel"`
	DedupMode        DedupMode            `json:"dedupMode"`
	ChunkSizeKB      int                  `json:"chunkSizeKB"`
	MinFileSizeKB    int                  `json:"minFileSizeKB"`
	MaxFileSizeGB    int                  `json:"maxFileSizeGB"`
}

// ========== 效率统计 ==========

// EfficiencyStats 效率统计
type EfficiencyStats struct {
	ConfigID           string    `json:"configId"`
	TotalFiles         int64     `json:"totalFiles"`
	ProcessedFiles     int64     `json:"processedFiles"`
	SkippedFiles       int64     `json:"skippedFiles"`
	FailedFiles        int64     `json:"failedFiles"`
	OriginalSizeBytes  int64     `json:"originalSizeBytes"`
	StoredSizeBytes    int64     `json:"storedSizeBytes"`
	DedupedSizeBytes   int64     `json:"dedupedSizeBytes"`
	CompressedSizeBytes int64    `json:"compressedSizeBytes"`
	SpaceSavedBytes    int64     `json:"spaceSavedBytes"`
	SpaceSavedPercent  float64   `json:"spaceSavedPercent"`
	DedupRatio         float64   `json:"dedupRatio"`
	CompressionRatio   float64   `json:"compressionRatio"`
	ProcessingTimeMs   int64     `json:"processingTimeMs"`
	LastRunTime        time.Time `json:"lastRunTime"`
	NextRunTime        time.Time `json:"nextRunTime"`
	Status             string    `json:"status"`
}

// DedupEntry 去重条目
type DedupEntry struct {
	Hash      string   `json:"hash"`
	Size      int64     `json:"size"`
	RefCount  int      `json:"refCount"`
	Files     []string `json:"files"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
}

// CompressionResult 压缩结果
type CompressionResult struct {
	Algorithm    CompressionAlgorithm `json:"algorithm"`
	OriginalSize int64                `json:"originalSize"`
	CompressedSize int64              `json:"compressedSize"`
	Ratio        float64              `json:"ratio"`
	TimeMs       int64                `json:"timeMs"`
}

// ========== 存储分析 ==========

// StorageAnalysis 存储分析
type StorageAnalysis struct {
	TotalCapacity    int64            `json:"totalCapacity"`
	UsedCapacity     int64            `json:"usedCapacity"`
	FreeCapacity     int64            `json:"freeCapacity"`
	UniqueData       int64            `json:"uniqueData"`
	DuplicateData    int64            `json:"duplicateData"`
	CompressibleData int64            `json:"compressibleData"`
	AlreadyCompressed int64           `json:"alreadyCompressed"`
	EstimatedSaving  int64            `json:"estimatedSaving"`
	FileTypeBreakdown map[string]int64 `json:"fileTypeBreakdown"`
	TopDuplicates    []DedupEntry     `json:"topDuplicates"`
	Recommendations  []Recommendation `json:"recommendations"`
}

// Recommendation 优化建议
type Recommendation struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // dedup, compress, archive, delete
	Title       string `json:"title"`
	Description string `json:"description"`
	SavingBytes int64  `json:"savingBytes"`
	Priority    int    `json:"priority"`
	Confidence  float64 `json:"confidence"`
}

// ========== 调度配置 ==========

// ScheduleConfig 调度配置
type ScheduleConfig struct {
	ID        string    `json:"id"`
	ConfigID  string    `json:"configId"`
	Enabled   bool      `json:"enabled"`
	Frequency string    `json:"frequency"` // daily, weekly, monthly
	DayOfWeek int       `json:"dayOfWeek"`
	Hour      int       `json:"hour"`
	Minute    int       `json:"minute"`
	LastRun   time.Time `json:"lastRun"`
	NextRun   time.Time `json:"nextRun"`
}

// ========== 任务状态 ==========

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

// EfficiencyTask 效率任务
type EfficiencyTask struct {
	ID          string           `json:"id"`
	ConfigID    string           `json:"configId"`
	Status      TaskStatus       `json:"status"`
	Progress    float64          `json:"progress"`
	CurrentFile string           `json:"currentFile,omitempty"`
	Stats       *EfficiencyStats `json:"stats,omitempty"`
	ErrorMsg    string           `json:"errorMsg,omitempty"`
	StartTime   time.Time        `json:"startTime"`
	EndTime     *time.Time       `json:"endTime,omitempty"`
}

// ========== 错误定义 ==========

var (
	ErrConfigNotFound  = fmt.Errorf("config not found")
	ErrTaskNotFound    = fmt.Errorf("task not found")
	ErrTaskRunning     = fmt.Errorf("task already running")
	ErrInvalidStrategy = fmt.Errorf("invalid strategy")
	ErrInvalidAlgo     = fmt.Errorf("invalid compression algorithm")
	ErrInsufficientSpace = fmt.Errorf("insufficient disk space")
)
