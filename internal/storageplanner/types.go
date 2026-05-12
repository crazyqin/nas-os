// Package storageplanner 提供存储容量规划功能，参考群晖 Storage Manager
package storageplanner

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrVolumeNotFound 卷不存在.
	ErrVolumeNotFound = errors.New("卷不存在")
	// ErrPathNotFound 路径不存在.
	ErrPathNotFound = errors.New("路径不存在")
	// ErrScanInProgress 扫描正在进行中.
	ErrScanInProgress = errors.New("扫描正在进行中")
	// ErrNoScanData 没有扫描数据.
	ErrNoScanData = errors.New("没有扫描数据")
	// ErrInvalidForecastDays 无效的预测天数.
	ErrInvalidForecastDays = errors.New("无效的预测天数")
	// ErrInsufficientData 数据不足，无法进行趋势分析.
	ErrInsufficientData = errors.New("数据不足，无法进行趋势分析")
)

// ========== 存储使用趋势 ==========

// UsageSnapshot 存储使用快照（一个时间点的数据）.
type UsageSnapshot struct {
	Timestamp    time.Time `json:"timestamp"`
	TotalBytes   uint64    `json:"total_bytes"`   // 总容量
	UsedBytes    uint64    `json:"used_bytes"`     // 已用
	FreeBytes    uint64    `json:"free_bytes"`     // 可用
	UsagePercent float64   `json:"usage_percent"`  // 使用率
	VolumeName   string    `json:"volume_name"`    // 卷名
}

// UsageTrend 存储使用趋势.
type UsageTrend struct {
	VolumeName       string           `json:"volume_name"`
	Period           TrendPeriod      `json:"period"`
	Snapshots        []UsageSnapshot  `json:"snapshots"`
	GrowthRate       float64          `json:"growth_rate"`       // 字节/天
	GrowthPercent    float64          `json:"growth_percent"`    // %/天
	AvgDailyGrowth   uint64           `json:"avg_daily_growth"`  // 日均增长（字节）
	TotalGrowth      uint64           `json:"total_growth"`      // 周期内总增长
	TotalGrowthPct   float64          `json:"total_growth_pct"`  // 周期内增长百分比
	MaxUsedBytes     uint64           `json:"max_used_bytes"`
	MinUsedBytes     uint64           `json:"min_used_bytes"`
}

// TrendPeriod 趋势统计周期.
type TrendPeriod struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Days      int       `json:"days"`
}

// ========== 容量预测 ==========

// CapacityForecast 容量预测结果.
type CapacityForecast struct {
	VolumeName       string           `json:"volume_name"`
	CurrentUsed      uint64           `json:"current_used"`
	CurrentTotal     uint64           `json:"current_total"`
	CurrentFree      uint64           `json:"current_free"`
	CurrentPct       float64          `json:"current_pct"`
	GrowthRateDaily  float64          `json:"growth_rate_daily"`  // 字节/天
	ForecastDate     time.Time        `json:"forecast_date"`      // 预测生成时间
	Predictions      []Prediction     `json:"predictions"`        // 按天/周/月预测
	DaysUntilFull    int              `json:"days_until_full"`    // 预计多少天后填满 (-1=不会满)
	EstimatedFullDate *time.Time      `json:"estimated_full_date"` // 预计满容量日期
	Confidence       float64          `json:"confidence"`         // 预测置信度 (0-1)
	WarningLevel     WarningLevel     `json:"warning_level"`      // 警告级别
	Recommendations  []string         `json:"recommendations"`    // 建议
}

// Prediction 单个预测点.
type Prediction struct {
	Date         time.Time `json:"date"`
	DaysFromNow  int       `json:"days_from_now"`
	PredictedUsed uint64   `json:"predicted_used"`
	PredictedFree uint64   `json:"predicted_free"`
	PredictedPct  float64  `json:"predicted_pct"`
}

// WarningLevel 告警级别.
type WarningLevel string

const (
	// WarningLevelNormal 正常.
	WarningLevelNormal WarningLevel = "normal"
	// WarningLevelLow 低风险.
	WarningLevelLow WarningLevel = "low"
	// WarningLevelMedium 中等风险.
	WarningLevelMedium WarningLevel = "medium"
	// WarningLevelHigh 高风险.
	WarningLevelHigh WarningLevel = "high"
	// WarningLevelCritical 临界.
	WarningLevelCritical WarningLevel = "critical"
)

// ========== 空间回收建议 ==========

// ReclaimSuggestion 空间回收建议.
type ReclaimSuggestion struct {
	ID           string         `json:"id"`
	Category     ReclaimCategory `json:"category"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Path         string         `json:"path"`
	EstimatedSize uint64        `json:"estimated_size"` // 预估可回收字节数
	Severity     SuggestionSeverity `json:"severity"`
	AutoCleanable bool          `json:"auto_cleanable"` // 是否可以自动清理
	Risk         string         `json:"risk"`           // 风险说明
	CreatedAt    time.Time      `json:"created_at"`
}

// ReclaimCategory 回收建议类别.
type ReclaimCategory string

const (
	// ReclaimCategoryTempFiles 临时文件.
	ReclaimCategoryTempFiles ReclaimCategory = "temp_files"
	// ReclaimCategoryOldLogs 旧日志文件.
	ReclaimCategoryOldLogs ReclaimCategory = "old_logs"
	// ReclaimCategoryCache 缓存文件.
	ReclaimCategoryCache ReclaimCategory = "cache"
	// ReclaimCategoryTrash 回收站.
	ReclaimCategoryTrash ReclaimCategory = "trash"
	// ReclaimCategoryDuplicates 重复文件.
	ReclaimCategoryDuplicates ReclaimCategory = "duplicates"
	// ReclaimCategoryOldSnapshots 旧快照.
	ReclaimCategoryOldSnapshots ReclaimCategory = "old_snapshots"
	// ReclaimCategoryLargeFiles 大文件.
	ReclaimCategoryLargeFiles ReclaimCategory = "large_files"
	// ReclaimCategoryUnusedPackages 未使用的软件包.
	ReclaimCategoryUnusedPackages ReclaimCategory = "unused_packages"
)

// SuggestionSeverity 建议严重程度.
type SuggestionSeverity string

const (
	// SuggestionSeverityInfo 信息.
	SuggestionSeverityInfo SuggestionSeverity = "info"
	// SuggestionSeveritySuggestion 建议.
	SuggestionSeveritySuggestion SuggestionSeverity = "suggestion"
	// SuggestionSeverityWarning 警告.
	SuggestionSeverityWarning SuggestionSeverity = "warning"
	// SuggestionSeverityUrgent 紧急.
	SuggestionSeverityUrgent SuggestionSeverity = "urgent"
)

// ReclaimSummary 回收建议汇总.
type ReclaimSummary struct {
	TotalSuggestions   int    `json:"total_suggestions"`
	TotalReclaimable   uint64 `json:"total_reclaimable"`    // 总可回收字节数
	AutoCleanable      uint64 `json:"auto_cleanable"`       // 可自动清理字节数
	ManualReviewNeeded uint64 `json:"manual_review_needed"` // 需手动审核字节数
	CategoryBreakdown  map[ReclaimCategory]uint64 `json:"category_breakdown"`
}

// ========== 重复文件检测 ==========

// DuplicateGroup 重复文件组（相同内容的一组文件）.
type DuplicateGroup struct {
	ID           string        `json:"id"`
	Hash         string        `json:"hash"`          // 内容哈希 (SHA-256)
	FileSize     uint64        `json:"file_size"`     // 单个文件大小
	TotalWasted  uint64        `json:"total_wasted"`  // 浪费总空间
	FileCount    int           `json:"file_count"`    // 重复文件数
	Files        []DuplicateFile `json:"files"`
}

// DuplicateFile 重复文件信息.
type DuplicateFile struct {
	Path      string    `json:"path"`
	Size      uint64    `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	IsOriginal bool     `json:"is_original"` // 建议保留的文件
}

// DuplicateScanConfig 重复文件扫描配置.
type DuplicateScanConfig struct {
	Paths         []string `json:"paths"`           // 扫描路径
	MinFileSize   uint64   `json:"min_file_size"`   // 最小文件大小 (字节)
	MaxFileSize   uint64   `json:"max_file_size"`   // 最大文件大小 (字节)
	ExcludePatterns []string `json:"exclude_patterns"` // 排除模式
	Recursive     bool     `json:"recursive"`       // 递归子目录
	MaxDepth      int      `json:"max_depth"`       // 最大递归深度
}

// DuplicateScanResult 重复文件扫描结果.
type DuplicateScanResult struct {
	ID             string           `json:"id"`
	ScanTime       time.Time        `json:"scan_time"`
	Paths          []string         `json:"paths"`
	TotalFiles     int              `json:"total_files"`
	TotalSize      uint64           `json:"total_size"`
	DuplicateCount int              `json:"duplicate_count"`
	WastedBytes    uint64           `json:"wasted_bytes"`
	Groups         []DuplicateGroup `json:"groups"`
	Status         ScanStatus       `json:"status"`
	Duration       time.Duration    `json:"duration"`
}

// ScanStatus 扫描状态.
type ScanStatus string

const (
	// ScanStatusPending 等待中.
	ScanStatusPending ScanStatus = "pending"
	// ScanStatusRunning 运行中.
	ScanStatusRunning ScanStatus = "running"
	// ScanStatusCompleted 已完成.
	ScanStatusCompleted ScanStatus = "completed"
	// ScanStatusFailed 失败.
	ScanStatusFailed ScanStatus = "failed"
	// ScanStatusCancelled 已取消.
	ScanStatusCancelled ScanStatus = "cancelled"
)

// ========== 存储分析概览 ==========

// StorageOverview 存储规划概览（首页仪表盘）.
type StorageOverview struct {
	Volumes         []VolumeOverview     `json:"volumes"`
	TotalCapacity   uint64               `json:"total_capacity"`
	TotalUsed       uint64               `json:"total_used"`
	TotalFree       uint64               `json:"total_free"`
	TotalUsagePct   float64              `json:"total_usage_pct"`
	TopGrowthPaths  []PathGrowth         `json:"top_growth_paths"`
	Reclaimable     uint64               `json:"reclaimable"`
	DuplicateWaste  uint64               `json:"duplicate_waste"`
	NextFullWarning *NextFullWarning     `json:"next_full_warning,omitempty"`
	LastScanTime    *time.Time           `json:"last_scan_time,omitempty"`
	GeneratedAt     time.Time            `json:"generated_at"`
}

// VolumeOverview 单卷概览.
type VolumeOverview struct {
	Name        string  `json:"name"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsagePct    float64 `json:"usage_pct"`
	GrowthRate  float64 `json:"growth_rate"`  // 字节/天
	DaysToFull  int     `json:"days_to_full"` // -1=不会满
}

// PathGrowth 路径增长排名条目.
type PathGrowth struct {
	Path        string  `json:"path"`
	VolumeName  string  `json:"volume_name"`
	Size        uint64  `json:"size"`
	GrowthRate  float64 `json:"growth_rate"`  // 字节/天
	GrowthPct   float64 `json:"growth_pct"`   // %/天
}

// NextFullWarning 最先满容量的警告.
type NextFullWarning struct {
	VolumeName    string    `json:"volume_name"`
	DaysUntilFull int       `json:"days_until_full"`
	EstimatedDate time.Time `json:"estimated_date"`
	CurrentPct    float64   `json:"current_pct"`
}

// ========== 输入结构 ==========

// TrendRequest 趋势查询请求.
type TrendRequest struct {
	VolumeName string `json:"volume_name"`
	Days       int    `json:"days"` // 统计天数 (默认30)
}

// ForecastRequest 预测请求.
type ForecastRequest struct {
	VolumeName    string `json:"volume_name"`
	ForecastDays  int    `json:"forecast_days"` // 预测天数 (默认90)
}

// ReclaimRequest 回收建议请求.
type ReclaimRequest struct {
	VolumeName string `json:"volume_name"`
	MinSize    uint64 `json:"min_size"` // 最小回收价值（字节）
}

// DuplicateScanRequest 启动重复文件扫描请求.
type DuplicateScanRequest DuplicateScanConfig

// DuplicateDeleteRequest 删除重复文件请求.
type DuplicateDeleteRequest struct {
	GroupID      string   `json:"group_id" binding:"required"`
	KeepFilePath string   `json:"keep_file_path"` // 保留的文件路径，空则自动选择
	FilePaths    []string `json:"file_paths"`      // 指定要删除的文件
}

// ========== 持久化结构 ==========

// PlannerConfig 存储规划器配置.
type PlannerConfig struct {
	// 趋势采集配置
	TrendCollectionEnabled  bool          `json:"trend_collection_enabled"`
	TrendCollectionInterval time.Duration `json:"trend_collection_interval"` // 采集间隔
	TrendRetentionDays      int           `json:"trend_retention_days"`     // 数据保留天数
	Volumes                 []string      `json:"volumes"`                   // 监控的卷

	// 回收建议配置
	ReclaimMinSize uint64 `json:"reclaim_min_size"` // 最小回收价值（字节）

	// 重复文件检测配置
	DuplicateDefaultMinSize uint64   `json:"duplicate_default_min_size"` // 默认最小文件大小
	DuplicateExcludePatterns []string `json:"duplicate_exclude_patterns"` // 默认排除模式

	// 预测配置
	DefaultForecastDays int     `json:"default_forecast_days"` // 默认预测天数
	GrowthThresholdPct  float64 `json:"growth_threshold_pct"`  // 增长告警阈值（%）
}

// DefaultPlannerConfig 默认配置.
func DefaultPlannerConfig() PlannerConfig {
	return PlannerConfig{
		TrendCollectionEnabled:  true,
		TrendCollectionInterval: 1 * time.Hour,
		TrendRetentionDays:      90,
		Volumes:                 []string{},
		ReclaimMinSize:          100 * 1024 * 1024, // 100MB
		DuplicateDefaultMinSize: 1024 * 1024,        // 1MB
		DuplicateExcludePatterns: []string{
			"/proc", "/sys", "/dev", "/run",
			".git", "node_modules", ".cache",
		},
		DefaultForecastDays: 90,
		GrowthThresholdPct:  10.0,
	}
}

// persistentState 持久化状态.
type persistentState struct {
	Config   PlannerConfig        `json:"config"`
	Trends   map[string][]UsageSnapshot `json:"trends"` // volumeName -> snapshots
	LastScan *DuplicateScanResult `json:"last_scan,omitempty"`
}
