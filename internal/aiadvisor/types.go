// Package aiadvisor 提供AI存储优化顾问功能
package aiadvisor

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNoScanData 没有扫描数据.
	ErrNoScanData = errors.New("尚未执行存储扫描，请先调用 POST /scan")
	// ErrRecommendationNotFound 建议不存在.
	ErrRecommendationNotFound = errors.New("优化建议不存在")
	// ErrAlreadyScanning 扫描正在进行中.
	ErrAlreadyScanning = errors.New("存储扫描正在进行中")
	// ErrInsufficientHistory 历史数据不足.
	ErrInsufficientHistory = errors.New("历史数据不足，至少需要2个数据点")
	// ErrInvalidPath 无效扫描路径.
	ErrInvalidPath = errors.New("无效的扫描路径")
)

// ========== 扫描结果类型 ==========

// ScanConfig 扫描配置.
type ScanConfig struct {
	// RootPath 扫描根路径.
	RootPath string `json:"root_path"`
	// LargeFileThresholdMB 大文件阈值（MB）.
	LargeFileThresholdMB int `json:"large_file_threshold_mb"`
	// StaleDays 未访问天数阈值.
	StaleDays int `json:"stale_days"`
	// MaxDepth 最大扫描深度.
	MaxDepth int `json:"max_depth"`
	// EnableDedupCheck 是否启用重复文件检测.
	EnableDedupCheck bool `json:"enable_dedup_check"`
}

// DefaultScanConfig 返回默认扫描配置.
func DefaultScanConfig() *ScanConfig {
	return &ScanConfig{
		RootPath:             "/",
		LargeFileThresholdMB: 100,
		StaleDays:            90,
		MaxDepth:             10,
		EnableDedupCheck:     true,
	}
}

// FileInfo 文件信息.
type FileInfo struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"` // 字节
	ModTime      time.Time `json:"mod_time"`
	AccessTime   time.Time `json:"access_time"`
	IsDir        bool      `json:"is_dir"`
	Extension    string    `json:"extension"`
	Hash         string    `json:"hash,omitempty"` // 文件哈希（去重用）
	DaysSinceUse int       `json:"days_since_use"`
}

// DuplicateGroup 重复文件组.
type DuplicateGroup struct {
	Hash  string     `json:"hash"`
	Size  int64      `json:"size"` // 单文件大小
	Count int        `json:"count"`
	Files []FileInfo `json:"files"`
	// WastedBytes 浪费的空间（(count-1) * size）.
	WastedBytes int64 `json:"wasted_bytes"`
}

// ScanResult 存储扫描结果.
type ScanResult struct {
	RootPath          string           `json:"root_path"`
	ScanStartedAt     time.Time        `json:"scan_started_at"`
	ScanFinishedAt    time.Time        `json:"scan_finished_at"`
	DurationSeconds   float64          `json:"duration_seconds"`
	TotalFiles        int              `json:"total_files"`
	TotalDirs         int              `json:"total_dirs"`
	TotalSizeBytes    int64            `json:"total_size_bytes"`
	LargeFiles        []FileInfo       `json:"large_files"`
	StaleFiles        []FileInfo       `json:"stale_files"`
	DuplicateGroups   []DuplicateGroup `json:"duplicate_groups"`
	DuplicateWaste    int64            `json:"duplicate_waste"` // 去重可节省的总字节
	ExtensionSummary  map[string]ExtStat `json:"extension_summary"`
	TopDirsBySize     []DirSizeStat    `json:"top_dirs_by_size"`
}

// ExtStat 文件扩展名统计.
type ExtStat struct {
	Count    int   `json:"count"`
	TotalBytes int64 `json:"total_bytes"`
}

// DirSizeStat 目录大小统计.
type DirSizeStat struct {
	Path      string `json:"path"`
	TotalSize int64  `json:"total_size"`
	FileCount int    `json:"file_count"`
}

// ========== 建议类型 ==========

// RecommendationType 建议类型.
type RecommendationType string

const (
	// RecTypeLargeFile 大文件清理.
	RecTypeLargeFile RecommendationType = "large_file"
	// RecTypeDedup 去重优化.
	RecTypeDedup RecommendationType = "dedup"
	// RecTypeStaleArchive 冷数据归档.
	RecTypeStaleArchive RecommendationType = "stale_archive"
	// RecTypeCompress 压缩优化.
	RecTypeCompress RecommendationType = "compress"
	// RecTypeTierMigration 分层迁移.
	RecTypeTierMigration RecommendationType = "tier_migration"
	// RecTypeCleanup 清理临时文件.
	RecTypeCleanup RecommendationType = "cleanup"
)

// Recommendation 优化建议.
type Recommendation struct {
	ID               string             `json:"id"`
	Type             RecommendationType `json:"type"`
	Title            string             `json:"title"`
	Description      string             `json:"description"`
	Priority         int                `json:"priority"` // 1=高 2=中 3=低
	EstimatedSaving  int64              `json:"estimated_saving_bytes"`
	TargetFiles      []string           `json:"target_files,omitempty"`
	TargetPath       string             `json:"target_path,omitempty"`
	Applied          bool               `json:"applied"`
	AppliedAt        *time.Time         `json:"applied_at,omitempty"`
}

// ========== 容量预测类型 ==========

// CapacityDataPoint 容量历史数据点.
type CapacityDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	UsedBytes int64     `json:"used_bytes"`
	TotalBytes int64    `json:"total_bytes"`
}

// CapacityForecast 容量预测结果.
type CapacityForecast struct {
	CurrentUsedBytes    int64              `json:"current_used_bytes"`
	CurrentTotalBytes   int64              `json:"current_total_bytes"`
	UsagePercent        float64            `json:"usage_percent"`
	MonthlyGrowthGB     float64            `json:"monthly_growth_gb"`
	MonthlyGrowthPct    float64            `json:"monthly_growth_pct"`
	DaysUntilFull       float64            `json:"days_until_full"`
	Predictions         []PredictionPoint  `json:"predictions"`
	UrgencyLevel        string             `json:"urgency_level"` // critical/warning/normal
	GeneratedAt         time.Time          `json:"generated_at"`
}

// PredictionPoint 预测数据点.
type PredictionPoint struct {
	Date        time.Time `json:"date"`
	PredictedTB float64   `json:"predicted_tb"`
	UsagePct    float64   `json:"usage_pct"`
}

// ========== 优化报告类型 ==========

// OptimizationReport 优化报告.
type OptimizationReport struct {
	ScanSummary          ScanResultSummary `json:"scan_summary"`
	Recommendations      []Recommendation  `json:"recommendations"`
	TotalEstimatedSaving int64             `json:"total_estimated_saving_bytes"`
	SavingPercent        float64           `json:"saving_percent"`
	GeneratedAt          time.Time         `json:"generated_at"`
}

// ScanResultSummary 扫描结果摘要.
type ScanResultSummary struct {
	TotalFiles      int   `json:"total_files"`
	TotalSizeBytes  int64 `json:"total_size_bytes"`
	LargeFileCount  int   `json:"large_file_count"`
	StaleFileCount  int   `json:"stale_file_count"`
	DuplicateCount  int   `json:"duplicate_count"`
	DuplicateWaste  int64 `json:"duplicate_waste"`
}
