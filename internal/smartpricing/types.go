// Package smartpricing - 智能定价分析模块
// 多云存储成本对比、存储方案推荐、成本优化建议
package smartpricing

import (
	"fmt"
	"time"
)

// ============================================================
// 存储方案类型
// ============================================================

// StorageProvider 云存储提供商.
type StorageProvider string

const (
	ProviderAWSS3      StorageProvider = "aws_s3"
	ProviderAliyunOSS  StorageProvider = "aliyun_oss"
	ProviderTencentCOS StorageProvider = "tencent_cos"
	ProviderMinIO      StorageProvider = "minio"
)

// StorageTier 存储层级.
type StorageTier string

const (
	TierStandard   StorageTier = "standard"   // 标准存储
	TierInfrequent StorageTier = "infrequent" // 低频访问
	TierArchive    StorageTier = "archive"    // 归档存储
	TierCold       StorageTier = "cold"       // 冷存储
)

// AccessFrequency 访问频率.
type AccessFrequency string

const (
	FreqHigh   AccessFrequency = "high"   // 高频访问
	FreqMedium AccessFrequency = "medium" // 中频访问
	FreqLow    AccessFrequency = "low"    // 低频访问
	FreqRare   AccessFrequency = "rare"   // 极少访问
)

// StoragePlan 存储方案.
type StoragePlan struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Provider        StorageProvider `json:"provider"`
	Tier            StorageTier     `json:"tier"`
	Region          string          `json:"region"`
	StoragePriceGB  float64         `json:"storage_price_gb"`  // 每GB月存储价格
	RequestPrice1K  float64         `json:"request_price_1k"`  // 每1000次请求价格
	TransferPriceGB float64         `json:"transfer_price_gb"` // 每GB出站流量价格
	MinStorageGB    float64         `json:"min_storage_gb"`    // 最小存储量
	MaxStorageGB    float64         `json:"max_storage_gb"`    // 最大存储量
	Description     string          `json:"description"`
	CreatedAt       time.Time       `json:"created_at"`
}

// ============================================================
// 成本对比类型
// ============================================================

// CostCompareRequest 成本对比请求.
type CostCompareRequest struct {
	StorageGB     float64           `json:"storage_gb"`     // 存储量(GB)
	MonthlyReads  int64             `json:"monthly_reads"`  // 月读取次数
	MonthlyWrites int64             `json:"monthly_writes"` // 月写入次数
	TransferGB    float64           `json:"transfer_gb"`    // 出站流量(GB)
	Providers     []StorageProvider `json:"providers"`      // 对比的提供商
	Tiers         []StorageTier     `json:"tiers"`          // 对比的存储层级
	Region        string            `json:"region"`         // 区域
}

// CostCompareResult 成本对比结果.
type CostCompareResult struct {
	Request     CostCompareRequest `json:"request"`
	Comparisons []ProviderCost     `json:"comparisons"`
	BestOption  *ProviderCost      `json:"best_option"`
	GeneratedAt time.Time          `json:"generated_at"`
}

// ProviderCost 提供商成本.
type ProviderCost struct {
	Provider        StorageProvider `json:"provider"`
	Tier            StorageTier     `json:"tier"`
	Region          string          `json:"region"`
	StorageCost     float64         `json:"storage_cost"`     // 月存储成本
	RequestCost     float64         `json:"request_cost"`     // 请求成本
	TransferCost    float64         `json:"transfer_cost"`    // 流量成本
	TotalMonthly    float64         `json:"total_monthly"`    // 月总成本
	TotalYearly     float64         `json:"total_yearly"`     // 年总成本
	StoragePriceGB  float64         `json:"storage_price_gb"` // 每GB存储价格
	ConfidenceLevel string          `json:"confidence_level"` // "high", "medium", "low"
}

// ============================================================
// 优化建议类型
// ============================================================

// OptimizationType 优化类型.
type OptimizationType string

const (
	OptTierMigration  OptimizationType = "tier_migration"  // 存储层级迁移
	OptProviderSwitch OptimizationType = "provider_switch" // 提供商切换
	OptLifecycle      OptimizationType = "lifecycle"       // 生命周期策略
	OptCompression    OptimizationType = "compression"     // 压缩优化
	OptDeduplication  OptimizationType = "deduplication"   // 去重优化
)

// OptimizationPriority 优化优先级.
type OptimizationPriority string

const (
	PriorityHigh   OptimizationPriority = "high"
	PriorityMedium OptimizationPriority = "medium"
	PriorityLow    OptimizationPriority = "low"
)

// OptimizationRecommendation 优化建议.
type OptimizationRecommendation struct {
	ID          string               `json:"id"`
	Type        OptimizationType     `json:"type"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Priority    OptimizationPriority `json:"priority"`
	// 预估节省
	EstimatedSavingsMonthly float64 `json:"estimated_savings_monthly"`
	EstimatedSavingsYearly  float64 `json:"estimated_savings_yearly"`
	SavingsPercent          float64 `json:"savings_percent"` // 节省百分比
	// 实施难度
	Difficulty string `json:"difficulty"` // "easy", "medium", "hard"
	// 当前配置
	CurrentProvider StorageProvider `json:"current_provider,omitempty"`
	CurrentTier     StorageTier     `json:"current_tier,omitempty"`
	// 推荐配置
	RecommendedProvider StorageProvider `json:"recommended_provider,omitempty"`
	RecommendedTier     StorageTier     `json:"recommended_tier,omitempty"`
	// 前置条件
	Prerequisites []string  `json:"prerequisites,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// RecommendationsResponse 优化建议响应.
type RecommendationsResponse struct {
	Recommendations []OptimizationRecommendation `json:"recommendations"`
	TotalSavings    float64                      `json:"total_savings_monthly"`
	GeneratedAt     time.Time                    `json:"generated_at"`
}

// ============================================================
// 成本趋势类型
// ============================================================

// CostTrendPoint 成本趋势点.
type CostTrendPoint struct {
	Date      time.Time `json:"date"`
	Cost      float64   `json:"cost"`
	StorageGB float64   `json:"storage_gb"`
	Provider  string    `json:"provider"`
	Tier      string    `json:"tier"`
}

// CostTrendRequest 成本趋势请求.
type CostTrendRequest struct {
	Provider  StorageProvider `json:"provider,omitempty"`
	Tier      StorageTier     `json:"tier,omitempty"`
	StartDate time.Time       `json:"start_date"`
	EndDate   time.Time       `json:"end_date"`
	Interval  string          `json:"interval"` // "daily", "weekly", "monthly"
}

// CostTrendResponse 成本趋势响应.
type CostTrendResponse struct {
	Request     CostTrendRequest `json:"request"`
	Trends      []CostTrendPoint `json:"trends"`
	Summary     TrendSummary     `json:"summary"`
	GeneratedAt time.Time        `json:"generated_at"`
}

// TrendSummary 趋势摘要.
type TrendSummary struct {
	TotalCost      float64 `json:"total_cost"`
	AvgMonthlyCost float64 `json:"avg_monthly_cost"`
	MaxCost        float64 `json:"max_cost"`
	MinCost        float64 `json:"min_cost"`
	CostChange     float64 `json:"cost_change"` // 成本变化百分比
	GrowthRate     float64 `json:"growth_rate"` // 存储增长率
}

// ============================================================
// 配置类型
// ============================================================

// SmartPricingConfig 智能定价分析配置.
type SmartPricingConfig struct {
	Enabled          bool    `json:"enabled"`
	DefaultRegion    string  `json:"default_region"`
	BudgetThreshold  float64 `json:"budget_threshold"`  // 预算阈值
	AlertThreshold   float64 `json:"alert_threshold"`   // 告警阈值(百分比)
	AnalysisInterval int     `json:"analysis_interval"` // 分析间隔(小时)
	MaxHistoryDays   int     `json:"max_history_days"`  // 最大历史天数
}

// DefaultSmartPricingConfig 默认配置.
func DefaultSmartPricingConfig() SmartPricingConfig {
	return SmartPricingConfig{
		Enabled:          true,
		DefaultRegion:    "cn-north-1",
		BudgetThreshold:  1000.0,
		AlertThreshold:   80.0,
		AnalysisInterval: 24,
		MaxHistoryDays:   90,
	}
}

// ============================================================
// 本地存储方案类型 (用于 Analyzer)
// ============================================================

// PricingPlan 本地存储定价方案
// 用于 Analyzer 模块的本地存储成本分析
// 与 StoragePlan (云存储) 不同，PricingPlan 专注于本地存储
// 保留 PricingPlan 用于 Analyzer 模块，避免与云存储 StoragePlan 冲突.
type PricingPlan struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Tier           StorageTier   `json:"tier"`
	Replica        ReplicaPolicy `json:"replica"`
	UnitPrice      float64       `json:"unit_price"`       // 每GB月价格
	MinCapacity    int64         `json:"min_capacity"`     // 最小容量(GB)
	MaxCapacity    int64         `json:"max_capacity"`     // 最大容量(GB)，0表示无限制
	IOPSLimit      int           `json:"iops_limit"`       // IOPS上限
	ThroughputMB   int           `json:"throughput_mb"`    // 吞吐量(MB/s)
	ReadLatencyMs  float64       `json:"read_latency_ms"`  // 读延迟(ms)
	WriteLatencyMs float64       `json:"write_latency_ms"` // 写延迟(ms)
	MonthlyBaseFee float64       `json:"monthly_base_fee"` // 月基础费用
	TransferFee    float64       `json:"transfer_fee"`     // 传输费用(每GB)
	Description    string        `json:"description"`
}

// ReplicaPolicy 副本策略.
type ReplicaPolicy string

const (
	ReplicaNone   ReplicaPolicy = "none"   // 无副本
	ReplicaMirror ReplicaPolicy = "mirror" // 镜像
	ReplicaRAID5  ReplicaPolicy = "raid5"  // RAID5
	ReplicaRAID6  ReplicaPolicy = "raid6"  // RAID6
	ReplicaTriple ReplicaPolicy = "triple" // 三副本
)

// WorkloadType 工作负载类型.
type WorkloadType string

const (
	WorkloadCold  WorkloadType = "cold"  // 冷数据
	WorkloadWarm  WorkloadType = "warm"  // 温数据
	WorkloadHot   WorkloadType = "hot"   // 热数据
	WorkloadMixed WorkloadType = "mixed" // 混合
)

// TierSSD SSD 存储层级.
const TierSSD StorageTier = "ssd"

// TierHDD HDD 存储层级.
const TierHDD StorageTier = "hdd"

// TierHybrid 混合存储层级.
const TierHybrid StorageTier = "hybrid"

// GetReplicaOverhead 获取副本开销系数.
func GetReplicaOverhead(replica ReplicaPolicy) float64 {
	switch replica {
	case ReplicaNone:
		return 1.0
	case ReplicaMirror:
		return 2.0
	case ReplicaRAID5:
		return 1.33 // 4盘RAID5
	case ReplicaRAID6:
		return 1.5 // 4盘RAID6
	case ReplicaTriple:
		return 3.0
	default:
		return 1.0
	}
}

// GetUsableRatio 获取可用容量比例.
func GetUsableRatio(replica ReplicaPolicy) float64 {
	overhead := GetReplicaOverhead(replica)
	if overhead > 0 {
		return 1.0 / overhead
	}
	return 1.0
}

// CostBreakdown 成本明细.
type CostBreakdown struct {
	StorageCost    float64 `json:"storage_cost"`     // 存储费用
	ReplicaCost    float64 `json:"replica_cost"`     // 副本费用
	TransferCost   float64 `json:"transfer_cost"`    // 传输费用
	BaseFee        float64 `json:"base_fee"`         // 基础费用
	TotalCost      float64 `json:"total_cost"`       // 总成本
	EffectivePerGB float64 `json:"effective_per_gb"` // 有效单价(每GB)
}

// CostAnalysis 成本分析结果.
type CostAnalysis struct {
	TotalCapacityGB     int64         `json:"total_capacity_gb"`
	Tier                StorageTier   `json:"tier"`
	Replica             ReplicaPolicy `json:"replica"`
	Workload            WorkloadType  `json:"workload"`
	MonthlyCost         CostBreakdown `json:"monthly_cost"`
	AnnualCost          CostBreakdown `json:"annual_cost"`
	ThreeYearCost       CostBreakdown `json:"three_year_cost"`
	EffectiveIOPS       int           `json:"effective_iops"`
	EffectiveThroughput int           `json:"effective_throughput"`
	ReadLatencyMs       float64       `json:"read_latency_ms"`
	WriteLatencyMs      float64       `json:"write_latency_ms"`
	ReplicaOverhead     float64       `json:"replica_overhead"`
	UsableRatio         float64       `json:"usable_ratio"`
	AnalyzedAt          time.Time     `json:"analyzed_at"`
	PlanUsed            string        `json:"plan_used"`
}

// OptimizeRequest 优化请求.
type OptimizeRequest struct {
	CapacityGB    int64        `json:"capacity_gb" binding:"required,gt=0"`
	Workload      WorkloadType `json:"workload"`
	MaxBudget     float64      `json:"max_budget"`
	MinIOPS       int          `json:"min_iops"`
	MaxLatencyMs  float64      `json:"max_latency_ms"`
	PreferredTier StorageTier  `json:"preferred_tier"`
}

// Validate 验证请求.
func (r OptimizeRequest) Validate() error {
	if r.CapacityGB <= 0 {
		return fmt.Errorf("capacity must be positive")
	}
	return nil
}

// Recommendation 推荐方案.
type Recommendation struct {
	Plan         PricingPlan   `json:"plan"`
	Rank         int           `json:"rank"`
	Score        float64       `json:"score"`
	TotalCost    float64       `json:"total_cost"`
	CostPerGB    float64       `json:"cost_per_gb"`
	CostAnalysis *CostAnalysis `json:"cost_analysis,omitempty"`
	Reasons      []string      `json:"reasons"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// OptimizeResult 优化结果.
type OptimizeResult struct {
	Request         OptimizeRequest  `json:"request"`
	Recommendations []Recommendation `json:"recommendations"`
	BestOption      *Recommendation  `json:"best_option,omitempty"`
	GeneratedAt     time.Time        `json:"generated_at"`
}

// ============================================================
// 报告类型
// ============================================================

// ReportType 报告类型.
type ReportType string

const (
	ReportMonthly ReportType = "monthly" // 月度报告
	ReportAnnual  ReportType = "annual"  // 年度报告
)

// CostReport 成本报告.
type CostReport struct {
	ReportID        string            `json:"report_id"`
	ReportType      ReportType        `json:"report_type"`
	Title           string            `json:"title"`
	GeneratedAt     time.Time         `json:"generated_at"`
	PeriodStart     time.Time         `json:"period_start"`
	PeriodEnd       time.Time         `json:"period_end"`
	TotalCapacityGB int64             `json:"total_capacity_gb"`
	UsedCapacityGB  int64             `json:"used_capacity_gb"`
	UsageRatio      float64           `json:"usage_ratio"`
	TotalCost       float64           `json:"total_cost"`
	StorageCost     float64           `json:"storage_cost"`
	ReplicaCost     float64           `json:"replica_cost"`
	TransferCost    float64           `json:"transfer_cost"`
	TierBreakdown   []TierCostSummary `json:"tier_breakdown"`
	Suggestions     []string          `json:"suggestions"`
}

// TierCostSummary 层级成本摘要.
type TierCostSummary struct {
	Tier       StorageTier `json:"tier"`
	CapacityGB int64       `json:"capacity_gb"`
	Cost       float64     `json:"cost"`
	CostPerGB  float64     `json:"cost_per_gb"`
	UsageRatio float64     `json:"usage_ratio"`
}
