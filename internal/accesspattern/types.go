// Package accesspattern - 访问模式分析模块
// 分析文件访问模式，识别冷热数据，为智能分层提供依据
// 参考群晖的智能分层和 TrueNAS 的数据管理
package accesspattern

import (
	"time"
)

// ============================================================
// 配置类型
// ============================================================

// AccessPatternConfig 访问模式分析配置
type AccessPatternConfig struct {
	// 数据分层阈值
	HotThresholdDays  int `json:"hot_threshold_days"`  // 热数据阈值（天），默认 7
	WarmThresholdDays int `json:"warm_threshold_days"` // 温数据阈值（天），默认 30
	ColdThresholdDays int `json:"cold_threshold_days"` // 冷数据阈值（天），默认 90

	// 访问频率阈值
	HotAccessCount  int `json:"hot_access_count"`  // 热数据访问次数阈值，默认 10
	WarmAccessCount int `json:"warm_access_count"` // 温数据访问次数阈值，默认 3

	// 分析配置
	AnalysisWindowDays int `json:"analysis_window_days"` // 分析窗口天数，默认 90
	MinAccessCount     int `json:"min_access_count"`     // 最小访问次数，默认 1

	// 采样配置
	SampleRate float64 `json:"sample_rate"` // 采样率 (0-1)，默认 1.0

	// 保留策略
	RetentionDays int `json:"retention_days"` // 记录保留天数，默认 365
}

// DefaultAccessPatternConfig 默认配置
func DefaultAccessPatternConfig() AccessPatternConfig {
	return AccessPatternConfig{
		HotThresholdDays:   7,
		WarmThresholdDays:  30,
		ColdThresholdDays:  90,
		HotAccessCount:     10,
		WarmAccessCount:    3,
		AnalysisWindowDays: 90,
		MinAccessCount:     1,
		SampleRate:         1.0,
		RetentionDays:      365,
	}
}

// ============================================================
// 数据温度枚举
// ============================================================

// DataTemperature 数据温度
type DataTemperature string

const (
	TemperatureHot  DataTemperature = "hot"  // 热数据
	TemperatureWarm DataTemperature = "warm" // 温数据
	TemperatureCold DataTemperature = "cold" // 冷数据
)

// ============================================================
// 访问记录类型
// ============================================================

// AccessRecord 访问记录
type AccessRecord struct {
	ID         string    `json:"id"`          // 记录ID
	FilePath   string    `json:"file_path"`   // 文件路径
	FileSize   int64     `json:"file_size"`   // 文件大小（字节）
	AccessTime time.Time `json:"access_time"` // 访问时间
	AccessMode string    `json:"access_mode"` // 访问模式: "read", "write", "execute"
	UserID     string    `json:"user_id"`     // 用户ID
	UserAgent  string    `json:"user_agent"`  // 用户代理
	ClientIP   string    `json:"client_ip"`   // 客户端IP
}

// ============================================================
// 模式分析类型
// ============================================================

// PatternAnalysis 文件访问模式分析结果
type PatternAnalysis struct {
	FilePath       string          `json:"file_path"`        // 文件路径
	FileSize       int64           `json:"file_size"`        // 文件大小
	TotalAccesses  int             `json:"total_accesses"`   // 总访问次数
	FirstAccess    time.Time       `json:"first_access"`     // 首次访问时间
	LastAccess     time.Time       `json:"last_access"`      // 最后访问时间
	AccessInterval time.Duration   `json:"access_interval"`  // 平均访问间隔
	Temperature    DataTemperature `json:"temperature"`      // 数据温度
	HeatScore      float64         `json:"heat_score"`       // 热度评分 (0-100)
	AccessPattern  string          `json:"access_pattern"`   // 访问模式: "sequential", "random", "burst"
	AccessHours    []int           `json:"access_hours"`     // 访问时间分布（24小时）
	AccessDays     []int           `json:"access_days"`      // 访问日期分布（7天）
	ReadWriteRatio float64         `json:"read_write_ratio"` // 读写比
	SuggestedTier  string          `json:"suggested_tier"`   // 建议存储层级
	AnalyzedAt     time.Time       `json:"analyzed_at"`      // 分析时间
}

// ============================================================
// 热力图类型
// ============================================================

// HeatMap 热力图数据
type HeatMap struct {
	GeneratedAt time.Time      `json:"generated_at"` // 生成时间
	TimeRange   TimeRange      `json:"time_range"`   // 时间范围
	Entries     []HeatMapEntry `json:"entries"`      // 热力图条目
	Summary     HeatMapSummary `json:"summary"`      // 汇总信息
}

// HeatMapEntry 热力图条目
type HeatMapEntry struct {
	FilePath    string          `json:"file_path"`    // 文件路径
	HeatScore   float64         `json:"heat_score"`   // 热度评分
	Temperature DataTemperature `json:"temperature"`  // 数据温度
	AccessCount int             `json:"access_count"` // 访问次数
	Size        int64           `json:"size"`         // 文件大小
}

// HeatMapSummary 热力图汇总
type HeatMapSummary struct {
	TotalFiles   int     `json:"total_files"`    // 总文件数
	HotFiles     int     `json:"hot_files"`      // 热文件数
	WarmFiles    int     `json:"warm_files"`     // 温文件数
	ColdFiles    int     `json:"cold_files"`     // 冷文件数
	TotalSize    int64   `json:"total_size"`     // 总大小
	HotSize      int64   `json:"hot_size"`       // 热数据大小
	WarmSize     int64   `json:"warm_size"`      // 温数据大小
	ColdSize     int64   `json:"cold_size"`      // 冷数据大小
	AvgHeatScore float64 `json:"avg_heat_score"` // 平均热度
}

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"` // 开始时间
	End   time.Time `json:"end"`   // 结束时间
}

// ============================================================
// 统计类型
// ============================================================

// AccessStats 访问统计
type AccessStats struct {
	TotalRecords   int            `json:"total_records"`    // 总记录数
	UniqueFiles    int            `json:"unique_files"`     // 唯一文件数
	TotalAccesses  int            `json:"total_accesses"`   // 总访问次数
	ByTemperature  map[string]int `json:"by_temperature"`   // 按温度统计
	ByAccessMode   map[string]int `json:"by_access_mode"`   // 按访问模式统计
	TopFiles       []FileAccess   `json:"top_files"`        // 热门文件
	LastAnalyzedAt *time.Time     `json:"last_analyzed_at"` // 最后分析时间
}

// FileAccess 文件访问统计
type FileAccess struct {
	FilePath     string    `json:"file_path"`      // 文件路径
	AccessCount  int       `json:"access_count"`   // 访问次数
	TotalSize    int64     `json:"total_size"`     // 文件大小
	LastAccessAt time.Time `json:"last_access_at"` // 最后访问时间
}

// ============================================================
// 分层建议类型
// ============================================================

// TieringSuggestion 分层建议
type TieringSuggestion struct {
	FilePath      string          `json:"file_path"`      // 文件路径
	CurrentTier   string          `json:"current_tier"`   // 当前层级
	SuggestedTier string          `json:"suggested_tier"` // 建议层级
	Temperature   DataTemperature `json:"temperature"`    // 数据温度
	HeatScore     float64         `json:"heat_score"`     // 热度评分
	Reason        string          `json:"reason"`         // 建议原因
	Priority      int             `json:"priority"`       // 优先级 (1-10)
}

// TieringReport 分层报告
type TieringReport struct {
	GeneratedAt time.Time           `json:"generated_at"` // 生成时间
	Suggestions []TieringSuggestion `json:"suggestions"`  // 建议列表
	Summary     TieringSummary      `json:"summary"`      // 汇总信息
}

// TieringSummary 分层汇总
type TieringSummary struct {
	TotalFiles       int   `json:"total_files"`       // 总文件数
	NeedMigration    int   `json:"need_migration"`    // 需要迁移的文件数
	PotentialSavings int64 `json:"potential_savings"` // 潜在节省空间
}

// ============================================================
// HTTP 请求/响应类型
// ============================================================

// RecordAccessRequest 记录访问请求
type RecordAccessRequest struct {
	FilePath   string `json:"file_path" binding:"required"`
	FileSize   int64  `json:"file_size"`
	AccessMode string `json:"access_mode"`
	UserID     string `json:"user_id"`
	UserAgent  string `json:"user_agent"`
	ClientIP   string `json:"client_ip"`
}

// AnalysisRequest 分析请求
type AnalysisRequest struct {
	FilePath string `json:"file_path"` // 指定文件（可选，为空则分析全部）
	Force    bool   `json:"force"`     // 强制重新分析
}

// HeatMapRequest 热力图请求
type HeatMapRequest struct {
	StartTime string `json:"start_time"` // 开始时间 (RFC3339)
	EndTime   string `json:"end_time"`   // 结束时间 (RFC3339)
	Limit     int    `json:"limit"`      // 返回条目数限制
}

// AccessPatternResponse 访问模式响应
type AccessPatternResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// AnalysisListResponse 分析结果列表响应
type AnalysisListResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Data    []PatternAnalysis `json:"data,omitempty"`
}

// HeatMapResponse 热力图响应
type HeatMapResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Data    *HeatMap `json:"data,omitempty"`
}

// StatsResponse 统计响应
type StatsResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    *AccessStats `json:"data,omitempty"`
}

// TieringResponse 分层建议响应
type TieringResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    *TieringReport `json:"data,omitempty"`
}
