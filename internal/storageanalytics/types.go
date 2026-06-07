// Package storageanalytics 提供存储分析报告引擎功能。
package storageanalytics

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrPathRequired 路径参数必填.
	ErrPathRequired = errors.New("分析路径不能为空")
	// ErrAnalysisRunning 分析任务正在进行中.
	ErrAnalysisRunning = errors.New("分析任务正在执行中，请稍后再试")
	// ErrNoAnalysisData 尚未执行过分析.
	ErrNoAnalysisData = errors.New("尚未执行分析，请先调用 POST /analyze")
)

// ========== 文件分类 ==========

// FileType 文件类型.
type FileType string

const (
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeDocument FileType = "document"
	FileTypeArchive  FileType = "archive"
	FileTypeCode     FileType = "code"
	FileTypeOther    FileType = "other"
)

// ========== 文件大小分布 ==========

// SizeBracket 文件大小区间.
type SizeBracket string

const (
	SizeLT1MB     SizeBracket = "<1MB"
	Size1MBTo100  SizeBracket = "1MB-100MB"
	Size100MBTo1G SizeBracket = "100MB-1GB"
	SizeGT1GB     SizeBracket = ">1GB"
)

// ========== 文件年龄分布 ==========

// AgeBracket 文件年龄区间.
type AgeBracket string

const (
	AgeLT7Days    AgeBracket = "<7天"
	Age7To30Days  AgeBracket = "7-30天"
	Age30To90Days AgeBracket = "30-90天"
	Age90To365    AgeBracket = "90-365天"
	AgeGT1Year    AgeBracket = ">1年"
)

// ========== 访问频率 ==========

// AccessFrequency 访问频率.
type AccessFrequency string

const (
	AccessFrequent   AccessFrequency = "frequent"   // 频繁
	AccessOccasional AccessFrequency = "occasional" // 偶尔
	AccessRare       AccessFrequency = "rare"       // 很少
	AccessNever      AccessFrequency = "never"      // 从未访问
)

// ========== 分析请求 ==========

// AnalyzeRequest 启动分析请求.
type AnalyzeRequest struct {
	// Path 要分析的目录路径.
	Path string `json:"path" binding:"required"`
	// MaxDepth 最大扫描深度，0表示不限制.
	MaxDepth int `json:"max_depth"`
	// TopN 返回Top N大目录数量，默认10.
	TopN int `json:"top_n"`
}

// ========== 采集结果 ==========

// FileInfo 文件信息.
type FileInfo struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"` // 字节
	ModTime    time.Time `json:"mod_time"`
	AccessTime time.Time `json:"access_time"`
	IsDir      bool      `json:"is_dir"`
	FileType   FileType  `json:"file_type"`
}

// DirectoryInfo 目录统计信息.
type DirectoryInfo struct {
	Path      string `json:"path"`
	TotalSize int64  `json:"total_size"` // 字节
	FileCount int    `json:"file_count"`
	DirCount  int    `json:"dir_count"`
}

// CollectResult 采集结果.
type CollectResult struct {
	ScanPath    string          `json:"scan_path"`
	ScanTime    time.Time       `json:"scan_time"`
	Files       []FileInfo      `json:"files"`
	Directories []DirectoryInfo `json:"directories"`
	TotalSize   int64           `json:"total_size"`
	TotalFiles  int             `json:"total_files"`
	TotalDirs   int             `json:"total_dirs"`
}

// ========== 分析结果 ==========

// CategoryStat 分类统计.
type CategoryStat struct {
	Category   string  `json:"category"`
	FileCount  int     `json:"file_count"`
	TotalSize  int64   `json:"total_size"` // 字节
	Percentage float64 `json:"percentage"` // 占比百分比
}

// SizeDistribution 大小分布.
type SizeDistribution struct {
	Bracket    SizeBracket `json:"bracket"`
	FileCount  int         `json:"file_count"`
	TotalSize  int64       `json:"total_size"`
	Percentage float64     `json:"percentage"`
}

// AgeDistribution 年龄分布.
type AgeDistribution struct {
	Bracket    AgeBracket `json:"bracket"`
	FileCount  int        `json:"file_count"`
	TotalSize  int64      `json:"total_size"`
	Percentage float64    `json:"percentage"`
}

// AccessDistribution 访问频率分布.
type AccessDistribution struct {
	Frequency  AccessFrequency `json:"frequency"`
	FileCount  int             `json:"file_count"`
	TotalSize  int64           `json:"total_size"`
	Percentage float64         `json:"percentage"`
}

// HealthMetrics 存储健康指标.
type HealthMetrics struct {
	// FragmentationScore 碎片化程度评分 0-100（越高越健康）.
	FragmentationScore float64 `json:"fragmentation_score"`
	// EfficiencyScore 存储效率评分 0-100.
	EfficiencyScore float64 `json:"efficiency_score"`
	// RedundancyRate 数据冗余率 0-1.
	RedundancyRate float64 `json:"redundancy_rate"`
	// BackupCoverage 备份覆盖率 0-1.
	BackupCoverage float64 `json:"backup_coverage"`
	// OverallScore 综合健康评分 0-100.
	OverallScore float64 `json:"overall_score"`
}

// TrendPoint 趋势数据点.
type TrendPoint struct {
	Date      time.Time `json:"date"`
	TotalSize int64     `json:"total_size"`
	FileCount int       `json:"file_count"`
	Growth    int64     `json:"growth"` // 相比上期增长（字节）
}

// TrendAnalysis 趋势分析结果.
type TrendAnalysis struct {
	Daily   []TrendPoint `json:"daily"`
	Weekly  []TrendPoint `json:"weekly"`
	Monthly []TrendPoint `json:"monthly"`
	// DailyGrowthRate 日均增长率（字节/天）.
	DailyGrowthRate int64 `json:"daily_growth_rate"`
	// DaysUntilFull 预计几天后用满，-1表示无法预测.
	DaysUntilFull int `json:"days_until_full"`
	// CategoryGrowth 各类数据增长对比.
	CategoryGrowth []CategoryGrowthInfo `json:"category_growth"`
}

// CategoryGrowthInfo 各类数据增长信息.
type CategoryGrowthInfo struct {
	Category      string  `json:"category"`
	GrowthBytes   int64   `json:"growth_bytes"`
	GrowthPercent float64 `json:"growth_percent"`
	CurrentSize   int64   `json:"current_size"`
}

// Insight 智能洞察.
type Insight struct {
	Type     string `json:"type"`     // anomaly, waste, optimization
	Severity string `json:"severity"` // high, medium, low
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Saving   int64  `json:"saving"` // 预估可节省空间（字节）
	Action   string `json:"action"` // 建议操作
}

// InsightAnalysis 智能洞察分析结果.
type InsightAnalysis struct {
	Insights []Insight `json:"insights"`
	// WastedSpace 浪费空间总量（字节）.
	WastedSpace int64 `json:"wasted_space"`
	// TotalPotentialSaving 总潜在节省空间（字节）.
	TotalPotentialSaving int64 `json:"total_potential_saving"`
}

// ========== 完整报告 ==========

// StorageReport 完整存储分析报告.
type StorageReport struct {
	ScanPath       string               `json:"scan_path"`
	GeneratedAt    time.Time            `json:"generated_at"`
	Summary        Summary              `json:"summary"`
	FileTypeStats  []CategoryStat       `json:"file_type_stats"`
	TopDirectories []DirectoryInfo      `json:"top_directories"`
	SizeDist       []SizeDistribution   `json:"size_distribution"`
	AgeDist        []AgeDistribution    `json:"age_distribution"`
	AccessDist     []AccessDistribution `json:"access_distribution"`
	Health         HealthMetrics        `json:"health"`
	Trends         TrendAnalysis        `json:"trends"`
	Insights       InsightAnalysis      `json:"insights"`
}

// Summary 存储概览摘要.
type Summary struct {
	TotalSize      int64  `json:"total_size"`
	TotalFiles     int    `json:"total_files"`
	TotalDirs      int    `json:"total_dirs"`
	LargestFile    string `json:"largest_file"`
	LargestSize    int64  `json:"largest_size"`
	OldestFile     string `json:"oldest_file"`
	OldestAge      string `json:"oldest_age"`
	AvgFileSize    int64  `json:"avg_file_size"`
	MedianFileSize int64  `json:"median_file_size"`
}

// ========== 分析配置 ==========

// Config 分析配置.
type Config struct {
	// DefaultTopN 默认返回Top N大目录.
	DefaultTopN int
	// WastePatterns 临时文件/缓存匹配模式.
	WastePatterns []string
	// MaxFileSizeForAnalysis 单文件最大扫描大小，超过跳过.
	MaxFileSizeForAnalysis int64
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		DefaultTopN:            10,
		MaxFileSizeForAnalysis: 10 * 1024 * 1024 * 1024, // 10GB
		WastePatterns: []string{
			"*.tmp", "*.temp", "*.cache", "*.log",
			".DS_Store", "Thumbs.db", "*.swp",
			"__pycache__", ".npm", "node_modules/.cache",
			"*.bak", "*.old",
		},
	}
}
