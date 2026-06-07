// Package aistorageoptim 提供AI驱动的存储分层优化引擎
// 基于文件访问频率、大小、类型自动在HDD/SSD/NVMe间迁移数据
// 使用加权评分算法（访问频率40%、文件大小30%、IO模式20%、时间衰减10%）
package aistorageoptim

import (
	"time"
)

// StorageTier 存储层级
type StorageTier string

const (
	TierHDD  StorageTier = "hdd"  // HDD层 - 大容量低成本
	TierSSD  StorageTier = "ssd"  // SSD层 - 平衡性能和容量
	TierNVMe StorageTier = "nvme" // NVMe层 - 高性能低延迟
)

// AccessPattern 访问模式类型
type AccessPattern string

const (
	PatternHot     AccessPattern = "hot"     // 热数据 - 频繁访问
	PatternWarm    AccessPattern = "warm"    // 温数据 - 偶尔访问
	PatternCold    AccessPattern = "cold"    // 冷数据 - 很少访问
	PatternArchive AccessPattern = "archive" // 归档数据 - 几乎不访问
)

// IOPattern I/O访问模式
type IOPattern string

const (
	IOPatternSequential IOPattern = "sequential" // 顺序读写
	IOPatternRandom     IOPattern = "random"     // 随机读写
	IOPatternBurst      IOPattern = "burst"      // 突发访问
	IOPatternStreaming  IOPattern = "streaming"  // 流式访问
)

// TieringPolicy 分层策略配置
type TieringPolicy struct {
	// 权重配置
	AccessFrequencyWeight float64 `json:"accessFrequencyWeight"` // 访问频率权重 (默认0.4)
	FileSizeWeight        float64 `json:"fileSizeWeight"`        // 文件大小权重 (默认0.3)
	IOPatternWeight       float64 `json:"ioPatternWeight"`       // IO模式权重 (默认0.2)
	TimeDecayWeight       float64 `json:"timeDecayWeight"`       // 时间衰减权重 (默认0.1)

	// 阈值配置
	NVMePromoteThreshold float64 `json:"nvmePromoteThreshold"` // NVMe提升阈值
	SSDPromoteThreshold  float64 `json:"ssdPromoteThreshold"`  // SSD提升阈值
	HDDDemoteThreshold   float64 `json:"hddDemoteThreshold"`   // HDD降级阈值

	// 分析配置
	AnalysisInterval time.Duration `json:"analysisInterval"` // 分析间隔
	BatchSize        int           `json:"batchSize"`        // 批量迁移大小

	// 文件大小阈值
	SmallFileThreshold int64 `json:"smallFileThreshold"` // 小文件阈值 (bytes)
	LargeFileThreshold int64 `json:"largeFileThreshold"` // 大文件阈值 (bytes)
}

// DefaultTieringPolicy 返回默认分层策略
func DefaultTieringPolicy() TieringPolicy {
	return TieringPolicy{
		AccessFrequencyWeight: 0.4,
		FileSizeWeight:        0.3,
		IOPatternWeight:       0.2,
		TimeDecayWeight:       0.1,
		NVMePromoteThreshold:  80.0,
		SSDPromoteThreshold:   50.0,
		HDDDemoteThreshold:    20.0,
		AnalysisInterval:      15 * time.Minute,
		BatchSize:             100,
		SmallFileThreshold:    1024 * 1024,        // 1MB
		LargeFileThreshold:    1024 * 1024 * 1024, // 1GB
	}
}

// FileAccessStats 文件访问统计
type FileAccessStats struct {
	FilePath        string         `json:"filePath"`
	FileSize        int64          `json:"fileSize"`
	FileType        string         `json:"fileType"`
	CurrentTier     StorageTier    `json:"currentTier"`
	AccessCount     int64          `json:"accessCount"`
	LastAccessTime  time.Time      `json:"lastAccessTime"`
	FirstAccessTime time.Time      `json:"firstAccessTime"`
	TotalBytesRead  int64          `json:"totalBytesRead"`
	TotalBytesWrite int64          `json:"totalBytesWrite"`
	AccessFrequency float64        `json:"accessFrequency"` // 每小时访问次数
	IOPattern       IOPattern      `json:"ioPattern"`
	AccessPattern   AccessPattern  `json:"accessPattern"`
	Windows         []AccessWindow `json:"-"` // 访问窗口
}

// AccessWindow 访问时间窗口
type AccessWindow struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int64     `json:"count"`
	Bytes     int64     `json:"bytes"`
}

// OptimizationScore 优化评分
type OptimizationScore struct {
	FilePath             string      `json:"filePath"`
	CurrentTier          StorageTier `json:"currentTier"`
	RecommendedTier      StorageTier `json:"recommendedTier"`
	Score                float64     `json:"score"`
	AccessFrequencyScore float64     `json:"accessFrequencyScore"`
	FileSizeScore        float64     `json:"fileSizeScore"`
	IOPatternScore       float64     `json:"ioPatternScore"`
	TimeDecayScore       float64     `json:"timeDecayScore"`
	Priority             int         `json:"priority"` // 1-10, 10最高
	Reason               string      `json:"reason"`
}

// OptimizationDecision 优化决策
type OptimizationDecision struct {
	FilePath         string      `json:"filePath"`
	Action           string      `json:"action"` // promote/demote/keep
	FromTier         StorageTier `json:"fromTier"`
	ToTier           StorageTier `json:"toTier"`
	Score            float64     `json:"score"`
	Priority         int         `json:"priority"`
	Reason           string      `json:"reason"`
	EstimatedBenefit float64     `json:"estimatedBenefit"` // 预估性能提升百分比
}

// StorageMetrics 存储层指标
type StorageMetrics struct {
	Tier          StorageTier `json:"tier"`
	TotalCapacity int64       `json:"totalCapacity"` // 总容量 bytes
	UsedCapacity  int64       `json:"usedCapacity"`  // 已用容量 bytes
	UsagePercent  float64     `json:"usagePercent"`  // 使用率
	FileCount     int64       `json:"fileCount"`     // 文件数量
	AvgLatencyMs  float64     `json:"avgLatencyMs"`  // 平均延迟 ms
}

// OptimizationStats 优化统计
type OptimizationStats struct {
	TotalFiles       int64   `json:"totalFiles"`
	TotalDecisions   int64   `json:"totalDecisions"`
	PromoteCount     int64   `json:"promoteCount"`
	DemoteCount      int64   `json:"demoteCount"`
	KeepCount        int64   `json:"keepCount"`
	AvgScore         float64 `json:"avgScore"`
	LastAnalysisTime string  `json:"lastAnalysisTime"`
	NVMeUsage        float64 `json:"nvmeUsage"`
	SSDUsage         float64 `json:"ssdUsage"`
	HDDUsage         float64 `json:"hddUsage"`
}

// AnalysisRequest 分析请求
type AnalysisRequest struct {
	Path   string `json:"path"`   // 分析路径
	Force  bool   `json:"force"`  // 强制重新分析
	DryRun bool   `json:"dryRun"` // 只分析不执行
}

// OptimizationResponse 优化响应
type OptimizationResponse struct {
	Status    string                 `json:"status"`
	Decisions []OptimizationDecision `json:"decisions,omitempty"`
	Stats     OptimizationStats      `json:"stats"`
	Scores    []OptimizationScore    `json:"scores,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

// TierConfig 存储层配置
type TierConfig struct {
	Tier     StorageTier `json:"tier"`
	Path     string      `json:"path"`     // 挂载路径
	Capacity int64       `json:"capacity"` // 容量 bytes
	Type     string      `json:"type"`     // 设备类型
}

// MigrationRecord 迁移记录
type MigrationRecord struct {
	ID        string      `json:"id"`
	FilePath  string      `json:"filePath"`
	FromTier  StorageTier `json:"fromTier"`
	ToTier    StorageTier `json:"toTier"`
	Timestamp time.Time   `json:"timestamp"`
	Status    string      `json:"status"` // pending/running/completed/failed
	Score     float64     `json:"score"`
	Error     string      `json:"error,omitempty"`
}
