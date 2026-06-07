// Package costoptimizer 提供资源使用优化功能
package costoptimizer

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrUserNotFound 用户不存在.
	ErrUserNotFound = errors.New("用户不存在")
	// ErrResourceNotFound 资源不存在.
	ErrResourceNotFound = errors.New("资源不存在")
	// ErrInsufficientHistory 历史数据不足.
	ErrInsufficientHistory = errors.New("历史数据不足")
	// ErrQuotaExceeded 配额超限.
	ErrQuotaExceeded = errors.New("配额超限")
)

// ========== 资源类型定义 ==========

// ResourceType 资源类型.
type ResourceType string

const (
	// ResourceStorage 存储资源.
	ResourceStorage ResourceType = "storage"
	// ResourceCPU CPU资源.
	ResourceCPU ResourceType = "cpu"
	// ResourceMemory 内存资源.
	ResourceMemory ResourceType = "memory"
	// ResourceBandwidth 带宽资源.
	ResourceBandwidth ResourceType = "bandwidth"
)

// DataType 数据类型.
type DataType string

const (
	// DataTypeDocuments 文档类.
	DataTypeDocuments DataType = "documents"
	// DataTypeMedia 媒体文件（图片、视频）.
	DataTypeMedia DataType = "media"
	// DataTypeBackup 备份数据.
	DataTypeBackup DataType = "backup"
	// DataTypeArchive 归档数据.
	DataTypeArchive DataType = "archive"
	// DataTypeSystem 系统数据.
	DataTypeSystem DataType = "system"
	// DataTypeCache 缓存数据.
	DataTypeCache DataType = "cache"
)

// ========== 存储成本优化类型 ==========

// StorageTier 存储层级.
type StorageTier string

const (
	// TierNVMe NVMe SSD 层级.
	TierNVMe StorageTier = "nvme"
	// TierSSD SATA SSD 层级.
	TierSSD StorageTier = "ssd"
	// TierHDD HDD 层级.
	TierHDD StorageTier = "hdd"
	// TierCloud 云存储层级.
	TierCloud StorageTier = "cloud"
)

// CostProfile 成本画像.
type CostProfile struct {
	Tier           StorageTier `json:"tier"`
	Name           string      `json:"name"`
	CostPerTBMonth float64     `json:"cost_per_tb_month"` // 元/TB/月
	ReadSpeedMBps  int         `json:"read_speed_mbps"`
	WriteSpeedMBps int         `json:"write_speed_mbps"`
	LatencyMs      float64     `json:"latency_ms"`
	Reliability    string      `json:"reliability"` // 如 "99.999%"
}

// DefaultCostProfiles 默认成本画像.
var DefaultCostProfiles = map[StorageTier]CostProfile{
	TierNVMe: {
		Tier:           TierNVMe,
		Name:           "NVMe SSD",
		CostPerTBMonth: 500.0,
		ReadSpeedMBps:  3500,
		WriteSpeedMBps: 3000,
		LatencyMs:      0.1,
		Reliability:    "99.999%",
	},
	TierSSD: {
		Tier:           TierSSD,
		Name:           "SATA SSD",
		CostPerTBMonth: 300.0,
		ReadSpeedMBps:  550,
		WriteSpeedMBps: 520,
		LatencyMs:      0.5,
		Reliability:    "99.999%",
	},
	TierHDD: {
		Tier:           TierHDD,
		Name:           "HDD",
		CostPerTBMonth: 100.0,
		ReadSpeedMBps:  200,
		WriteSpeedMBps: 180,
		LatencyMs:      5.0,
		Reliability:    "99.99%",
	},
	TierCloud: {
		Tier:           TierCloud,
		Name:           "云存储",
		CostPerTBMonth: 50.0,
		ReadSpeedMBps:  100,
		WriteSpeedMBps: 50,
		LatencyMs:      50.0,
		Reliability:    "99.999999999%",
	},
}

// StorageAllocation 存储分配.
type StorageAllocation struct {
	Path        string      `json:"path"`
	Tier        StorageTier `json:"tier"`
	UsedBytes   int64       `json:"used_bytes"`
	SizeBytes   int64       `json:"size_bytes"`
	AccessCount int64       `json:"access_count"` // 月访问次数
	DataType    DataType    `json:"data_type"`
}

// CostReport 成本报告.
type CostReport struct {
	GeneratedAt      time.Time                `json:"generated_at"`
	TotalMonthlyCost float64                  `json:"total_monthly_cost"`
	OptimizedCost    float64                  `json:"optimized_cost"`
	TotalSavings     float64                  `json:"total_savings"`
	SavingsPercent   float64                  `json:"savings_percent"`
	CostByTier       map[StorageTier]float64  `json:"cost_by_tier"`
	Suggestions      []OptimizationSuggestion `json:"suggestions"`
	Allocations      []StorageAllocation      `json:"allocations"`
	WasteAnalysis    *WasteAnalysis           `json:"waste_analysis"`
}

// OptimizationSuggestion 优化建议.
type OptimizationSuggestion struct {
	ID              string      `json:"id"`
	Type            string      `json:"type"`     // migrate, compress, archive
	Priority        string      `json:"priority"` // high, medium, low
	Title           string      `json:"title"`
	Description     string      `json:"description"`
	SourcePath      string      `json:"source_path"`
	TargetTier      StorageTier `json:"target_tier,omitempty"`
	SavingsPerMonth float64     `json:"savings_per_month"`
	SavingsPercent  float64     `json:"savings_percent"`
	Effort          string      `json:"effort"` // 自动, 半自动, 手动
	Action          string      `json:"action"`
}

// WasteAnalysis 浪费分析.
type WasteAnalysis struct {
	TotalWastedBytes int64   `json:"total_wasted_bytes"`
	WastePercent     float64 `json:"waste_percent"`
}

// AccessPattern 访问模式.
type AccessPattern string

const (
	// AccessHot 热数据（频繁访问）.
	AccessHot AccessPattern = "hot"
	// AccessWarm 温数据（偶尔访问）.
	AccessWarm AccessPattern = "warm"
	// AccessCold 冷数据（很少访问）.
	AccessCold AccessPattern = "cold"
	// AccessFrozen 冻结数据（几乎不访问）.
	AccessFrozen AccessPattern = "frozen"
)

// ========== 核心数据结构 ==========

// ResourceUsage 资源使用记录.
type ResourceUsage struct {
	ID         string       `json:"id"`
	UserID     string       `json:"user_id"`
	Resource   ResourceType `json:"resource"`
	DataType   DataType     `json:"data_type"`
	Size       int64        `json:"size"` // 字节
	Cost       float64      `json:"cost"` // 元
	Timestamp  time.Time    `json:"timestamp"`
	AccessTime time.Time    `json:"access_time"`
	Path       string       `json:"path"`
}

// UserResourceProfile 用户资源画像.
type UserResourceProfile struct {
	UserID            string                   `json:"user_id"`
	Username          string                   `json:"username"`
	TotalUsage        map[ResourceType]int64   `json:"total_usage"`
	TotalCost         map[ResourceType]float64 `json:"total_cost"`
	QuotaLimit        map[ResourceType]int64   `json:"quota_limit"`
	UsageByType       map[DataType]int64       `json:"usage_by_type"`
	AccessPattern     map[AccessPattern]int64  `json:"access_pattern"`
	LastActive        time.Time                `json:"last_active"`
	GrowthRateGB      float64                  `json:"growth_rate_gb"`     // 月增长
	OptimizationScore float64                  `json:"optimization_score"` // 0-100
}

// StorageCostBreakdown 存储成本明细.
type StorageCostBreakdown struct {
	UserID       string                    `json:"user_id"`
	TotalCost    float64                   `json:"total_cost"`
	ByDataType   map[DataType]float64      `json:"by_data_type"`
	ByAccess     map[AccessPattern]float64 `json:"by_access_pattern"`
	MonthlyTrend []MonthlyCost             `json:"monthly_trend"`
	TopExpenses  []CostItem                `json:"top_expenses"`
	AnalyzedAt   time.Time                 `json:"analyzed_at"`
}

// MonthlyCost 月度成本.
type MonthlyCost struct {
	Month  string  `json:"month"`
	Cost   float64 `json:"cost"`
	SizeGB float64 `json:"size_gb"`
}

// CostItem 成本条目.
type CostItem struct {
	Path        string   `json:"path"`
	DataType    DataType `json:"data_type"`
	SizeGB      float64  `json:"size_gb"`
	Cost        float64  `json:"cost"`
	AccessCount int      `json:"access_count"`
}

// CapacityForecast 容量预测.
type CapacityForecast struct {
	Resource        ResourceType    `json:"resource"`
	CurrentUsage    int64           `json:"current_usage"`
	CurrentCapacity int64           `json:"current_capacity"`
	UsagePercent    float64         `json:"usage_percent"`
	GrowthRate      float64         `json:"growth_rate"` // 每月增长率（百分比）
	Predictions     []CapacityPoint `json:"predictions"`
	DaysUntilFull   float64         `json:"days_until_full"`
	UrgencyLevel    string          `json:"urgency_level"` // critical/warning/normal
	Recommendations []string        `json:"recommendations"`
	ForecastAt      time.Time       `json:"forecast_at"`
}

// CapacityPoint 容量预测点.
type CapacityPoint struct {
	Date     time.Time `json:"date"`
	Usage    int64     `json:"usage"`
	UsagePct float64   `json:"usage_pct"`
}

// ResourceReport 资源使用报告.
type ResourceReport struct {
	ReportID                  string                    `json:"report_id"`
	Period                    ReportPeriod              `json:"period"`
	Summary                   ResourceSummary           `json:"summary"`
	UserBreakdown             []UserResourceSummary     `json:"user_breakdown"`
	ResourceTrends            map[ResourceType]Trend    `json:"resource_trends"`
	TopConsumers              []ResourceConsumer        `json:"top_consumers"`
	OptimizationOpportunities []OptimizationOpportunity `json:"optimization_opportunities"`
	GeneratedAt               time.Time                 `json:"generated_at"`
}

// ReportPeriod 报告周期.
type ReportPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ResourceSummary 资源摘要.
type ResourceSummary struct {
	TotalStorageGB float64                  `json:"total_storage_gb"`
	TotalCost      float64                  `json:"total_cost"`
	ActiveUsers    int                      `json:"active_users"`
	ResourceUsage  map[ResourceType]float64 `json:"resource_usage"`
	Growth         GrowthSummary            `json:"growth"`
}

// GrowthSummary 增长摘要.
type GrowthSummary struct {
	MonthlyGrowthGB  float64 `json:"monthly_growth_gb"`
	MonthlyGrowthPct float64 `json:"monthly_growth_pct"`
	ProjectedYearEnd float64 `json:"projected_year_end_gb"`
}

// UserResourceSummary 用户资源摘要.
type UserResourceSummary struct {
	UserID       string  `json:"user_id"`
	Username     string  `json:"username"`
	StorageGB    float64 `json:"storage_gb"`
	Cost         float64 `json:"cost"`
	UsagePercent float64 `json:"usage_percent"` // 相对于配额
	GrowthRateGB float64 `json:"growth_rate_gb"`
}

// Trend 趋势.
type Trend struct {
	Current   float64 `json:"current"`
	Previous  float64 `json:"previous"`
	ChangePct float64 `json:"change_pct"`
	Direction string  `json:"direction"` // up/down/stable
}

// ResourceConsumer 资源消耗者.
type ResourceConsumer struct {
	Path        string       `json:"path"`
	UserID      string       `json:"user_id"`
	Resource    ResourceType `json:"resource"`
	SizeGB      float64      `json:"size_gb"`
	Cost        float64      `json:"cost"`
	AccessCount int          `json:"access_count"`
	LastAccess  time.Time    `json:"last_access"`
}

// OptimizationOpportunity 优化机会.
type OptimizationOpportunity struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"` // tiering/dedup/compression/archive/cleanup
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	EstimatedSaving  float64 `json:"estimated_saving"` // 元
	EstimatedSpaceGB float64 `json:"estimated_space_gb"`
	Priority         int     `json:"priority"` // 1-3
	AffectedUsers    int     `json:"affected_users"`
	AffectedFiles    int     `json:"affected_files"`
}

// QuotaAllocation 配额分配.
type QuotaAllocation struct {
	UserID       string                   `json:"user_id"`
	Username     string                   `json:"username"`
	Allocations  map[ResourceType]int64   `json:"allocations"`
	Usage        map[ResourceType]int64   `json:"usage"`
	UsagePercent map[ResourceType]float64 `json:"usage_percent"`
	QuotaLevel   string                   `json:"quota_level"` // basic/standard/premium/custom
	LastAdjusted time.Time                `json:"last_adjusted"`
	NextReview   time.Time                `json:"next_review"`
}

// QuotaRecommendation 配额建议.
type QuotaRecommendation struct {
	UserID           string                 `json:"user_id"`
	CurrentQuota     map[ResourceType]int64 `json:"current_quota"`
	RecommendedQuota map[ResourceType]int64 `json:"recommended_quota"`
	Reason           string                 `json:"reason"`
	ExpectedImpact   string                 `json:"expected_impact"`
	Priority         int                    `json:"priority"`
}

// ReclaimableResource 可回收资源.
type ReclaimableResource struct {
	ID          string       `json:"id"`
	UserID      string       `json:"user_id"`
	Path        string       `json:"path"`
	Resource    ResourceType `json:"resource"`
	SizeGB      float64      `json:"size_gb"`
	LastAccess  time.Time    `json:"last_access"`
	DaysIdle    int          `json:"days_idle"`
	ReclaimType string       `json:"reclaim_type"` // archive/delete/compress
	Priority    int          `json:"priority"`
	Reason      string       `json:"reason"`
}

// ReclaimPlan 回收计划.
type ReclaimPlan struct {
	PlanID             string                `json:"plan_id"`
	TotalReclaimableGB float64               `json:"total_reclaimable_gb"`
	TotalSaving        float64               `json:"total_saving"`
	Resources          []ReclaimableResource `json:"resources"`
	ByType             map[string]float64    `json:"by_type"`
	ByUser             map[string]float64    `json:"by_user"`
	EstimatedTimeHours float64               `json:"estimated_time_hours"`
	GeneratedAt        time.Time             `json:"generated_at"`
}

// ========== 配置 ==========

// OptimizerConfig 优化器配置.
type OptimizerConfig struct {
	// StorageCostPerGBMonth 存储成本（元/GB/月）.
	StorageCostPerGBMonth float64
	// HotDataCostMultiplier 热数据成本倍数.
	HotDataCostMultiplier float64
	// WarmDataCostMultiplier 温数据成本倍数.
	WarmDataCostMultiplier float64
	// ColdDataCostMultiplier 冷数据成本倍数.
	ColdDataCostMultiplier float64
	// IdleDaysThreshold 闲置天数阈值.
	IdleDaysThreshold int
	// GrowthAlertThresholdGB 增长告警阈值（GB）.
	GrowthAlertThresholdGB float64
	// UsageAlertPercent 使用率告警百分比.
	UsageAlertPercent float64
	// DefaultQuotaGB 默认配额（GB）.
	DefaultQuotaGB int64
}

// DefaultOptimizerConfig 返回默认配置.
func DefaultOptimizerConfig() *OptimizerConfig {
	return &OptimizerConfig{
		StorageCostPerGBMonth:  0.15,
		HotDataCostMultiplier:  1.5,
		WarmDataCostMultiplier: 1.0,
		ColdDataCostMultiplier: 0.5,
		IdleDaysThreshold:      90,
		GrowthAlertThresholdGB: 100,
		UsageAlertPercent:      80,
		DefaultQuotaGB:         1000,
	}
}

// QuotaLevel 配额级别.
type QuotaLevel string

const (
	// QuotaBasic 基础配额.
	QuotaBasic QuotaLevel = "basic"
	// QuotaStandard 标准配额.
	QuotaStandard QuotaLevel = "standard"
	// QuotaPremium 高级配额.
	QuotaPremium QuotaLevel = "premium"
	// QuotaCustom 自定义配额.
	QuotaCustom QuotaLevel = "custom"
)

// QuotaLevelConfig 配额级别配置.
type QuotaLevelConfig struct {
	Level      QuotaLevel             `json:"level"`
	Name       string                 `json:"name"`
	Quotas     map[ResourceType]int64 `json:"quotas"`
	MaxUsers   int                    `json:"max_users"`
	MonthlyFee float64                `json:"monthly_fee"`
}

// DefaultQuotaLevels 返回默认配额级别.
func DefaultQuotaLevels() []QuotaLevelConfig {
	return []QuotaLevelConfig{
		{
			Level: QuotaBasic,
			Name:  "基础版",
			Quotas: map[ResourceType]int64{
				ResourceStorage:   100 * 1024 * 1024 * 1024, // 100GB
				ResourceCPU:       2,                        // 2核
				ResourceMemory:    2 * 1024 * 1024 * 1024,   // 2GB
				ResourceBandwidth: 100 * 1024 * 1024 * 1024, // 100GB
			},
			MonthlyFee: 0,
		},
		{
			Level: QuotaStandard,
			Name:  "标准版",
			Quotas: map[ResourceType]int64{
				ResourceStorage:   500 * 1024 * 1024 * 1024, // 500GB
				ResourceCPU:       4,
				ResourceMemory:    4 * 1024 * 1024 * 1024,
				ResourceBandwidth: 500 * 1024 * 1024 * 1024,
			},
			MonthlyFee: 29,
		},
		{
			Level: QuotaPremium,
			Name:  "高级版",
			Quotas: map[ResourceType]int64{
				ResourceStorage:   2 * 1024 * 1024 * 1024 * 1024, // 2TB
				ResourceCPU:       8,
				ResourceMemory:    8 * 1024 * 1024 * 1024,
				ResourceBandwidth: 2 * 1024 * 1024 * 1024 * 1024,
			},
			MonthlyFee: 99,
		},
	}
}
