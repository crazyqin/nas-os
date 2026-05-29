// Package storageplanner 智能存储规划 - 容量趋势分析、扩容建议、成本优化
package storageplanner

import (
	"errors"
	"sync"
	"time"
)

// DataCategory 数据热度分类
type DataCategory string

const (
	HotData  DataCategory = "hot"  // 热数据：频繁访问
	WarmData DataCategory = "warm" // 温数据：偶尔访问
	ColdData DataCategory = "cold" // 冷数据：很少访问
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
)

// StoragePool 存储池
type StoragePool struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	TotalBytes  int64     `json:"total_bytes"`
	UsedBytes   int64     `json:"used_bytes"`
	FreeBytes   int64     `json:"free_bytes"`
	RAIDType    string    `json:"raid_type,omitempty"`
	Disks       []Disk    `json:"disks"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Disk 磁盘
type Disk struct {
	ID         string `json:"id"`
	Device     string `json:"device"`
	Model      string `json:"model"`
	Serial     string `json:"serial"`
	TotalBytes int64  `json:"total_bytes"`
	UsedBytes  int64  `json:"used_bytes"`
	TempC      int    `json:"temp_c"`
	Health     string `json:"health"`
}

// UsageSnapshot 使用量快照
type UsageSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	TotalBytes int64    `json:"total_bytes"`
	UsedBytes  int64    `json:"used_bytes"`
	FreeBytes  int64    `json:"free_bytes"`
	FileCount  int64    `json:"file_count"`
}

// TrendPoint 趋势点
type TrendPoint struct {
	Date      time.Time `json:"date"`
	UsedBytes int64     `json:"used_bytes"`
}

// TrendPrediction 趋势预测
type TrendPrediction struct {
	Method       string       `json:"method"`
	Slope        float64      `json:"slope"`
	Intercept    float64      `json:"intercept"`
	R2           float64      `json:"r2"`
	Predictions  []TrendPoint `json:"predictions"`
	FullDate     *time.Time   `json:"full_date,omitempty"`
	DaysUntilFull int         `json:"days_until_full"`
}

// ExpansionPlan 扩容计划
type ExpansionPlan struct {
	RecommendedAt   time.Time `json:"recommended_at"`
	CurrentUsage    float64   `json:"current_usage"`
	EstimatedFullAt time.Time `json:"estimated_full_at"`
	RecommendedGB   int64     `json:"recommended_gb"`
	EstimatedCost   float64   `json:"estimated_cost"`
	Currency        string    `json:"currency"`
	Options         []ExpansionOption `json:"options"`
	Reason          string    `json:"reason"`
}

// ExpansionOption 扩容选项
type ExpansionOption struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	CapacityGB  int64   `json:"capacity_gb"`
	UnitPrice   float64 `json:"unit_price"`
	TotalPrice  float64 `json:"total_price"`
	DiskCount   int     `json:"disk_count"`
	RAIDType    string  `json:"raid_type"`
	EffectiveGB int64   `json:"effective_gb"`
}

// DataItem 数据项
type DataItem struct {
	Path         string       `json:"path"`
	SizeBytes    int64        `json:"size_bytes"`
	LastAccess   time.Time    `json:"last_access"`
	LastModified time.Time    `json:"last_modified"`
	AccessCount  int64        `json:"access_count"`
	Category     DataCategory `json:"category"`
}

// DataHeatMap 数据热度图
type DataHeatMap struct {
	TotalFiles     int64   `json:"total_files"`
	TotalBytes     int64   `json:"total_bytes"`
	HotFiles       int64   `json:"hot_files"`
	HotBytes       int64   `json:"hot_bytes"`
	WarmFiles      int64   `json:"warm_files"`
	WarmBytes      int64   `json:"warm_bytes"`
	ColdFiles      int64   `json:"cold_files"`
	ColdBytes      int64   `json:"cold_bytes"`
	HotPercentage  float64 `json:"hot_percentage"`
	WarmPercentage float64 `json:"warm_percentage"`
	ColdPercentage float64 `json:"cold_percentage"`
	TopHotPaths    []DataItem `json:"top_hot_paths"`
	TopColdPaths   []DataItem `json:"top_cold_paths"`
}

// CompressionOpportunity 压缩机会
type CompressionOpportunity struct {
	Path           string  `json:"path"`
	CurrentSize    int64   `json:"current_size"`
	EstimatedSize  int64   `json:"estimated_size"`
	SavingsBytes   int64   `json:"savings_bytes"`
	SavingsPercent float64 `json:"savings_percent"`
	FileType       string  `json:"file_type"`
}

// DedupOpportunity 去重机会
type DedupOpportunity struct {
	Hash         string   `json:"hash"`
	Count        int      `json:"count"`
	TotalSize    int64    `json:"total_size"`
	WastedSize   int64    `json:"wasted_size"`
	Paths        []string `json:"paths"`
}

// TieringSuggestion 分层建议
type TieringSuggestion struct {
	Category        DataCategory `json:"category"`
	FileCount       int64        `json:"file_count"`
	TotalBytes      int64        `json:"total_bytes"`
	CurrentTier     string       `json:"current_tier"`
	RecommendedTier string       `json:"recommended_tier"`
	Reason          string       `json:"reason"`
	EstimatedSaving float64      `json:"estimated_saving"`
}

// CostOptimizationReport 成本优化报告
type CostOptimizationReport struct {
	GeneratedAt       time.Time              `json:"generated_at"`
	CurrentCostMonthly float64               `json:"current_cost_monthly"`
	Compression       []CompressionOpportunity `json:"compression"`
	Deduplication     []DedupOpportunity       `json:"deduplication"`
	Tiering           []TieringSuggestion      `json:"tiering"`
	TotalSavings      float64                  `json:"total_savings"`
	SavingsPercent    float64                  `json:"savings_percent"`
	Recommendations   []string                 `json:"recommendations"`
}

// AlertThreshold 告警阈值
type AlertThreshold struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Level       AlertLevel `json:"level"`
	Percentage  float64    `json:"percentage"`
	Enabled     bool       `json:"enabled"`
	NotifyEmail string     `json:"notify_email,omitempty"`
	NotifyWebhook string   `json:"notify_webhook,omitempty"`
	LastTriggered *time.Time `json:"last_triggered,omitempty"`
}

// Alert 告警
type Alert struct {
	ID          string     `json:"id"`
	ThresholdID string     `json:"threshold_id"`
	Level       AlertLevel `json:"level"`
	Message     string     `json:"message"`
	UsagePercent float64   `json:"usage_percent"`
	TriggeredAt  time.Time `json:"triggered_at"`
	Acknowledged bool      `json:"acknowledged"`
}

// PlanningReport 存储规划报告
type PlanningReport struct {
	GeneratedAt    time.Time          `json:"generated_at"`
	Summary        ReportSummary      `json:"summary"`
	Trend          TrendPrediction    `json:"trend"`
	Expansion      *ExpansionPlan     `json:"expansion,omitempty"`
	HeatMap        DataHeatMap        `json:"heat_map"`
	Optimization   CostOptimizationReport `json:"optimization"`
	Alerts         []Alert            `json:"alerts"`
	Recommendations []string          `json:"recommendations"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalCapacity  int64   `json:"total_capacity"`
	UsedCapacity   int64   `json:"used_capacity"`
	UsagePercent   float64 `json:"usage_percent"`
	GrowthRate7d   float64 `json:"growth_rate_7d"`
	GrowthRate30d  float64 `json:"growth_rate_30d"`
	DaysUntilFull  int     `json:"days_until_full"`
	HealthScore    int     `json:"health_score"`
	ActiveAlerts   int     `json:"active_alerts"`
}

// PlannerConfig 规划器配置
type PlannerConfig struct {
	HotDataDays     int     `json:"hot_data_days"`
	WarmDataDays    int     `json:"warm_data_days"`
	UnitPriceGB     float64 `json:"unit_price_gb"`
	Currency        string  `json:"currency"`
	AlertWebhook    string  `json:"alert_webhook"`
	ReportSchedule  string  `json:"report_schedule"`
}

// Planner 存储规划器
type Planner struct {
	mu           sync.RWMutex
	pools        map[string]*StoragePool
	history      []UsageSnapshot
	dataItems    []DataItem
	thresholds   map[string]*AlertThreshold
	alerts       []Alert
	config       *PlannerConfig
	dataFile     string
}

var (
	ErrPoolNotFound     = errors.New("storage pool not found")
	ErrThresholdNotFound = errors.New("alert threshold not found")
	ErrInsufficientData  = errors.New("insufficient data for analysis")
	ErrInvalidThreshold  = errors.New("invalid threshold percentage")
)

// NewPlanner 创建存储规划器
func NewPlanner(dataFile string) *Planner {
	return &Planner{
		pools:      make(map[string]*StoragePool),
		thresholds: make(map[string]*AlertThreshold),
		config: &PlannerConfig{
			HotDataDays:  7,
			WarmDataDays: 30,
			UnitPriceGB:  0.5,
			Currency:     "CNY",
		},
		dataFile: dataFile,
	}
}
