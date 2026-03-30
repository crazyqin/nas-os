// Package billing 提供成本分析功能
package billing

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrInvalidCostConfig 无效的成本配置错误.
	ErrInvalidCostConfig = errors.New("无效的成本配置")
	// ErrInsufficientData 数据不足无法计算成本错误.
	ErrInsufficientData = errors.New("数据不足，无法计算成本")
	// ErrStoragePoolNotFound 存储池不存在错误.
	ErrStoragePoolNotFound = errors.New("存储池不存在")
)

// ========== 成本分析配置 ==========

// CostAnalyzerConfig 成本分析器配置.
type CostAnalyzerConfig struct {
	// 存储成本配置
	Storage StorageCostConfig `json:"storage"`

	// 带宽成本配置
	Bandwidth BandwidthCostConfig `json:"bandwidth"`

	// AI服务成本配置 (v2.308.0新增)
	AIService AIServiceCostConfig `json:"ai_service"`

	// 勒索检测成本配置 (v2.308.0新增)
	RansomwareDetection RansomwareCostConfig `json:"ransomware_detection"`

	// 分析周期
	AnalysisPeriodDays int `json:"analysis_period_days"`

	// 访问频率分层配置
	AccessFrequencyTiers []AccessFrequencyTier `json:"access_frequency_tiers"`

	// 货币单位
	DefaultCurrency string `json:"default_currency"`
}

// AIServiceCostConfig AI服务成本配置 (v2.308.0新增).
type AIServiceCostConfig struct {
	// 是否启用AI成本追踪
	Enabled bool `json:"enabled"`

	// 本地推理成本
	LocalInference LocalInferenceCostConfig `json:"local_inference"`

	// 云端API成本
	CloudAPI CloudAPICostConfig `json:"cloud_api"`

	// 月预算限制
	MonthlyBudgetLimit float64 `json:"monthly_budget_limit"`

	// 单次请求成本目标
	PerRequestCostTarget float64 `json:"per_request_cost_target"`
}

// LocalInferenceCostConfig 本地推理成本配置.
type LocalInferenceCostConfig struct {
	// 是否启用本地推理
	Enabled bool `json:"enabled"`

	// GPU型号
	GPUModel string `json:"gpu_model"` // rtx3060, rtx4090, a100, etc.

	// 电费单价(元/kWh)
	ElectricityPricePerKWh float64 `json:"electricity_price_per_kwh"`

	// GPU功耗(W)
	GPUPowerConsumptionW float64 `json:"gpu_power_consumption_w"`

	// 日均推理时长(小时)
	DailyInferenceHours float64 `json:"daily_inference_hours"`

	// 硬件折旧(元/月)
	HardwareDepreciation float64 `json:"hardware_depreciation"`
}

// CloudAPICostConfig 云端API成本配置.
type CloudAPICostConfig struct {
	// 是否启用云端API
	Enabled bool `json:"enabled"`

	// 提供商价格表
	ProviderPrices []AIProviderPrice `json:"provider_prices"`

	// 日调用量限额
	DailyRequestLimit int `json:"daily_request_limit"`

	// 月Token限额
	MonthlyTokenLimit int64 `json:"monthly_token_limit"`

	// 免费额度
	FreeTokensPerDay int64 `json:"free_tokens_per_day"`
}

// AIProviderPrice AI提供商价格.
type AIProviderPrice struct {
	Provider     string  `json:"provider"`             // openai, baidu, aliyun, etc.
	Model        string  `json:"model"`                // 模型名称
	InputPrice   float64 `json:"input_price"`          // 输入价格(CNY/万tokens)
	OutputPrice  float64 `json:"output_price"`         // 输出价格(CNY/万tokens)
	IsDefault    bool    `json:"is_default"`           // 是否默认提供商
	Priority     int     `json:"priority"`             // 优先级(越小越优先)
}

// RansomwareCostConfig 勒索检测成本配置 (v2.308.0新增).
type RansomwareCostConfig struct {
	// 是否启用勒索检测成本追踪
	Enabled bool `json:"enabled"`

	// 特征库存储配置
	SignatureDB SignatureDBCostConfig `json:"signature_db"`

	// 隔离区存储配置
	Quarantine QuarantineCostConfig `json:"quarantine"`

	// 事件日志存储配置
	EventLog EventLogCostConfig `json:"event_log"`

	// 自动快照配置
	AutoSnapshot AutoSnapshotCostConfig `json:"auto_snapshot"`
}

// SignatureDBCostConfig 特征库存储成本配置.
type SignatureDBCostConfig struct {
	// 固定大小(MB)
	FixedSizeMB float64 `json:"fixed_size_mb"`

	// 更新频率(天)
	UpdateFrequencyDays int `json:"update_frequency_days"`

	// 成本估算(元/月) - 固定低成本
	MonthlyCostEstimate float64 `json:"monthly_cost_estimate"`
}

// QuarantineCostConfig 隔离区存储成本配置.
type QuarantineCostConfig struct {
	// 最大容量(GB)
	MaxCapacityGB float64 `json:"max_capacity_gb"`

	// 当前使用量(GB)
	CurrentUsageGB float64 `json:"current_usage_gb"`

	// 存储价格(元/GB/月)
	PricePerGB float64 `json:"price_per_gb"`

	// 自动清理天数
	AutoCleanupDays int `json:"auto_cleanup_days"`

	// 预估月成本
	MonthlyCostEstimate float64 `json:"monthly_cost_estimate"`
}

// EventLogCostConfig 事件日志存储成本配置.
type EventLogCostConfig struct {
	// 日均事件数
	DailyEventCount int64 `json:"daily_event_count"`

	// 单事件大小(KB)
	EventSizeKB float64 `json:"event_size_kb"`

	// 保留天数
	RetentionDays int `json:"retention_days"`

	// 存储价格(元/GB/月)
	PricePerGB float64 `json:"price_per_gb"`

	// 压缩率
	CompressionRatio float64 `json:"compression_ratio"`

	// 预估月成本
	MonthlyCostEstimate float64 `json:"monthly_cost_estimate"`
}

// AutoSnapshotCostConfig 自动快照成本配置.
type AutoSnapshotCostConfig struct {
	// 是否启用自动快照
	Enabled bool `json:"enabled"`

	// 快照触发次数(月)
	SnapshotTriggerCount int `json:"snapshot_trigger_count"`

	// 单快照大小(GB)
	SnapshotSizeGB float64 `json:"snapshot_size_gb"`

	// 快照价格(元/GB/月)
	SnapshotPricePerGB float64 `json:"snapshot_price_per_gb"`

	// 快照保留天数
	RetentionDays int `json:"retention_days"`

	// 预估月成本
	MonthlyCostEstimate float64 `json:"monthly_cost_estimate"`
}

// StorageCostConfig 存储成本配置.
type StorageCostConfig struct {
	// 按容量计费价格（元/GB/月）
	CapacityPricing CapacityPricingConfig `json:"capacity_pricing"`

	// 按访问频率计费价格
	AccessFrequencyPricing AccessFrequencyPricingConfig `json:"access_frequency_pricing"`

	// 存储类型价格
	SSDPricePerGB     float64 `json:"ssd_price_per_gb"`
	HDDPricePerGB     float64 `json:"hdd_price_per_gb"`
	ArchivePricePerGB float64 `json:"archive_price_per_gb"`

	// 免费额度
	FreeStorageGB float64 `json:"free_storage_gb"`
}

// CapacityPricingConfig 容量计费配置.
type CapacityPricingConfig struct {
	// 基础价格
	BasePricePerGB float64 `json:"base_price_per_gb"`

	// 阶梯定价
	TieredPricing []CapacityTier `json:"tiered_pricing"`

	// 是否启用阶梯定价
	EnableTieredPricing bool `json:"enable_tiered_pricing"`
}

// CapacityTier 容量阶梯.
type CapacityTier struct {
	MinGB      float64 `json:"min_gb"`
	MaxGB      float64 `json:"max_gb"` // -1 表示无限
	PricePerGB float64 `json:"price_per_gb"`
}

// AccessFrequencyPricingConfig 访问频率计费配置.
type AccessFrequencyPricingConfig struct {
	Enabled bool `json:"enabled"`

	// 热数据价格（经常访问）
	HotDataPricePerGB float64 `json:"hot_data_price_per_gb"`

	// 温数据价格（偶尔访问）
	WarmDataPricePerGB float64 `json:"warm_data_price_per_gb"`

	// 冷数据价格（很少访问）
	ColdDataPricePerGB float64 `json:"cold_data_price_per_gb"`

	// 访问频率阈值（次数/天）
	HotAccessThreshold  float64 `json:"hot_access_threshold"`  // 高于此值为热数据
	WarmAccessThreshold float64 `json:"warm_access_threshold"` // 高于此值为温数据
}

// AccessFrequencyTier 访问频率分层.
type AccessFrequencyTier struct {
	Name            string  `json:"name"`
	MinAccessPerDay float64 `json:"min_access_per_day"`
	MaxAccessPerDay float64 `json:"max_access_per_day"` // -1 表示无限
	PriceMultiplier float64 `json:"price_multiplier"`   // 价格乘数
}

// BandwidthBillingModel 带宽计费模式（扩展类型）.
type BandwidthBillingModel string

// 带宽计费模式常量.
const (
	BandwidthBillingTraffic BandwidthBillingModel = "traffic" // 按流量计费
	BandwidthBillingPeak    BandwidthBillingModel = "peak"    // 按峰值带宽计费
	BandwidthBillingPeak95  BandwidthBillingModel = "peak_95" // 按95峰值计费
	BandwidthBillingMonthly BandwidthBillingModel = "monthly" // 按月固定带宽
	BandwidthBillingHybrid  BandwidthBillingModel = "hybrid"  // 混合计费
)

// BandwidthCostConfig 带宽成本配置.
type BandwidthCostConfig struct {
	// 计费模式
	Model BandwidthBillingModel `json:"model"`

	// 按流量计费
	TrafficPricePerGB float64 `json:"traffic_price_per_gb"`

	// 按带宽峰值计费
	BandwidthPricePerMbps float64 `json:"bandwidth_price_per_mbps"`

	// 95峰值计费
	Peak95PricePerMbps float64 `json:"peak_95_price_per_mbps"`

	// 免费额度
	FreeTrafficGB     float64 `json:"free_traffic_gb"`
	FreeBandwidthMbps float64 `json:"free_bandwidth_mbps"`

	// 入站流量是否计费
	ChargeInbound bool `json:"charge_inbound"`

	// 出站流量是否计费
	ChargeOutbound bool `json:"charge_outbound"`
}

// ========== 成本分析数据结构 ==========

// StorageCostAnalysis 存储成本分析结果.
type StorageCostAnalysis struct {
	// 分析时间
	AnalysisTime time.Time `json:"analysis_time"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`

	// 总体成本
	TotalCost         float64 `json:"total_cost"`
	TotalCapacityCost float64 `json:"total_capacity_cost"`
	TotalAccessCost   float64 `json:"total_access_cost"`
	Currency          string  `json:"currency"`

	// 按容量计费详情
	CapacityAnalysis CapacityCostAnalysis `json:"capacity_analysis"`

	// 按访问频率计费详情
	AccessFrequencyAnalysis AccessFrequencyCostAnalysis `json:"access_frequency_analysis"`

	// 按存储池分析
	PoolCosts []PoolStorageCost `json:"pool_costs"`

	// 按用户分析
	UserCosts []UserStorageCost `json:"user_costs"`

	// 成本趋势
	TrendData []CostTrendPoint `json:"trend_data"`

	// 优化建议
	Recommendations []CostRecommendation `json:"recommendations"`
}

// CapacityCostAnalysis 容量成本分析.
type CapacityCostAnalysis struct {
	// 总容量
	TotalCapacityGB float64 `json:"total_capacity_gb"`
	UsedCapacityGB  float64 `json:"used_capacity_gb"`
	FreeCapacityGB  float64 `json:"free_capacity_gb"`

	// 利用率
	UtilizationRate float64 `json:"utilization_rate"`

	// 成本
	MonthlyCost      float64 `json:"monthly_cost"`
	DailyCost        float64 `json:"daily_cost"`
	CostPerGB        float64 `json:"cost_per_gb"`
	AveragePricePaid float64 `json:"average_price_paid"`

	// 阶梯详情
	TierBreakdown []CapacityTierCost `json:"tier_breakdown"`
}

// CapacityTierCost 容量阶梯成本.
type CapacityTierCost struct {
	TierName   string  `json:"tier_name"`
	MinGB      float64 `json:"min_gb"`
	MaxGB      float64 `json:"max_gb"`
	UsedGB     float64 `json:"used_gb"`
	PricePerGB float64 `json:"price_per_gb"`
	Cost       float64 `json:"cost"`
}

// AccessFrequencyCostAnalysis 访问频率成本分析.
type AccessFrequencyCostAnalysis struct {
	Enabled bool `json:"enabled"`

	// 热数据
	HotDataGB   float64 `json:"hot_data_gb"`
	HotDataCost float64 `json:"hot_data_cost"`
	HotPercent  float64 `json:"hot_percent"`

	// 温数据
	WarmDataGB   float64 `json:"warm_data_gb"`
	WarmDataCost float64 `json:"warm_data_cost"`
	WarmPercent  float64 `json:"warm_percent"`

	// 冷数据
	ColdDataGB   float64 `json:"cold_data_gb"`
	ColdDataCost float64 `json:"cold_data_cost"`
	ColdPercent  float64 `json:"cold_percent"`

	// 总计
	TotalDataGB float64 `json:"total_data_gb"`
	TotalCost   float64 `json:"total_cost"`

	// 优化建议
	OptimizationPotential float64 `json:"optimization_potential"` // 潜在节省
}

// PoolStorageCost 存储池成本.
type PoolStorageCost struct {
	PoolID      string `json:"pool_id"`
	PoolName    string `json:"pool_name"`
	StorageType string `json:"storage_type"` // ssd, hdd, archive

	// 容量
	TotalCapacityGB float64 `json:"total_capacity_gb"`
	UsedCapacityGB  float64 `json:"used_capacity_gb"`
	UsagePercent    float64 `json:"usage_percent"`

	// 访问频率分布
	HotDataGB  float64 `json:"hot_data_gb"`
	WarmDataGB float64 `json:"warm_data_gb"`
	ColdDataGB float64 `json:"cold_data_gb"`

	// 成本
	PricePerGB        float64 `json:"price_per_gb"`
	MonthlyCost       float64 `json:"monthly_cost"`
	CostEfficiency    float64 `json:"cost_efficiency"` // 成本效率评分
	OptimizationScore float64 `json:"optimization_score"`
}

// UserStorageCost 用户存储成本.
type UserStorageCost struct {
	UserID        string             `json:"user_id"`
	UserName      string             `json:"user_name"`
	UsedGB        float64            `json:"used_gb"`
	MonthlyCost   float64            `json:"monthly_cost"`
	CostPerGB     float64            `json:"cost_per_gb"`
	AccessScore   float64            `json:"access_score"`   // 访问活跃度评分
	Tier          string             `json:"tier"`           // 用户层级
	PoolBreakdown map[string]float64 `json:"pool_breakdown"` // 各存储池用量
}

// CostTrendPoint 成本趋势数据点.
type CostTrendPoint struct {
	Date          time.Time `json:"date"`
	StorageCost   float64   `json:"storage_cost"`
	BandwidthCost float64   `json:"bandwidth_cost"`
	TotalCost     float64   `json:"total_cost"`
	UsedGB        float64   `json:"used_gb"`
	TrafficGB     float64   `json:"traffic_gb"`
}

// BandwidthCostAnalysis 带宽成本分析结果.
type BandwidthCostAnalysis struct {
	// 分析时间
	AnalysisTime time.Time `json:"analysis_time"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`

	// 总体成本
	TotalCost    float64 `json:"total_cost"`
	Currency     string  `json:"currency"`
	BillingModel string  `json:"billing_model"`

	// 流量统计
	InboundTrafficGB  float64 `json:"inbound_traffic_gb"`
	OutboundTrafficGB float64 `json:"outbound_traffic_gb"`
	TotalTrafficGB    float64 `json:"total_traffic_gb"`
	PeakBandwidthMbps float64 `json:"peak_bandwidth_mbps"`
	AverageMbps       float64 `json:"average_mbps"`
	Peak95Mbps        float64 `json:"peak_95_mbps"`

	// 成本明细
	TrafficCost      float64 `json:"traffic_cost"`
	BandwidthCost    float64 `json:"bandwidth_cost"`
	OverageCost      float64 `json:"overage_cost"` // 超额费用
	FreeAllowanceGB  float64 `json:"free_allowance_gb"`
	ChargedTrafficGB float64 `json:"charged_traffic_gb"`

	// 按时间分布
	HourlyDistribution []HourlyBandwidthCost `json:"hourly_distribution"`
	DailyDistribution  []DailyBandwidthCost  `json:"daily_distribution"`

	// 趋势
	TrendData []CostTrendPoint `json:"trend_data"`

	// 建议
	Recommendations []CostRecommendation `json:"recommendations"`
}

// HourlyBandwidthCost 小时带宽成本.
type HourlyBandwidthCost struct {
	Hour       int     `json:"hour"`
	InboundGB  float64 `json:"inbound_gb"`
	OutboundGB float64 `json:"outbound_gb"`
	TotalGB    float64 `json:"total_gb"`
	PeakMbps   float64 `json:"peak_mbps"`
	Cost       float64 `json:"cost"`
}

// DailyBandwidthCost 日带宽成本.
type DailyBandwidthCost struct {
	Date        time.Time `json:"date"`
	InboundGB   float64   `json:"inbound_gb"`
	OutboundGB  float64   `json:"outbound_gb"`
	TotalGB     float64   `json:"total_gb"`
	PeakMbps    float64   `json:"peak_mbps"`
	AverageMbps float64   `json:"average_mbps"`
	Cost        float64   `json:"cost"`
}

// CostRecommendation 成本优化建议.
type CostRecommendation struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"`     // storage, bandwidth, access_pattern, ai_service, ransomware
	Priority         string  `json:"priority"` // high, medium, low
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	PotentialSavings float64 `json:"potential_savings"` // 预计节省（元/月）
	CurrentCost      float64 `json:"current_cost"`
	OptimizedCost    float64 `json:"optimized_cost"`
	Action           string  `json:"action"`
	Impact           string  `json:"impact"`
	Implemented      bool    `json:"implemented"`
}

// ========== AI服务成本分析结构体 (v2.308.0新增) ==========

// AIServiceCostAnalysis AI服务成本分析结果.
type AIServiceCostAnalysis struct {
	AnalysisTime       time.Time `json:"analysis_time"`
	PeriodStart        time.Time `json:"period_start"`
	PeriodEnd          time.Time `json:"period_end"`
	TotalCost          float64   `json:"total_cost"`
	LocalInferenceCost float64   `json:"local_inference_cost"`
	CloudAPICost       float64   `json:"cloud_api_cost"`
	Currency           string    `json:"currency"`
	LocalAnalysis      LocalInferenceAnalysis      `json:"local_analysis"`
	CloudAnalysis      CloudAPIAnalysis           `json:"cloud_analysis"`
	UserAICosts        []UserAIServiceCost        `json:"user_ai_costs"`
	ModelCosts         []ModelCostBreakdown       `json:"model_costs"`
	TokenConsumption   TokenConsumptionStats      `json:"token_consumption"`
	TrendData          []AICostTrendPoint         `json:"trend_data"`
	Recommendations    []CostRecommendation       `json:"recommendations"`
	BudgetStatus       BudgetStatus               `json:"budget_status"`
}

// LocalInferenceAnalysis 本地推理分析.
type LocalInferenceAnalysis struct {
	DailyHours          float64   `json:"daily_hours"`
	MonthlyHours        float64   `json:"monthly_hours"`
	GPUModel            string    `json:"gpu_model"`
	GPUPowerW           float64   `json:"gpu_power_w"`
	GPUUtilization      float64   `json:"gpu_utilization"`
	ElectricityCost     float64   `json:"electricity_cost"`
	ElectricityPrice    float64   `json:"electricity_price"`
	HardwareDepreciation float64  `json:"hardware_depreciation"`
	MonthlyCost         float64   `json:"monthly_cost"`
	RequestCount        int64     `json:"request_count"`
	CostPerRequest      float64   `json:"cost_per_request"`
}

// CloudAPIAnalysis 云端API分析.
type CloudAPIAnalysis struct {
	DailyRequestCount   int64     `json:"daily_request_count"`
	MonthlyRequestCount int64     `json:"monthly_request_count"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	TotalTokens         int64     `json:"total_tokens"`
	InputTokenCost      float64   `json:"input_token_cost"`
	OutputTokenCost     float64   `json:"output_token_cost"`
	TotalTokenCost      float64   `json:"total_token_cost"`
	ProviderBreakdown   []ProviderCostBreakdown `json:"provider_breakdown"`
	FreeTokensUsed      int64     `json:"free_tokens_used"`
	FreeTokensLimit     int64     `json:"free_tokens_limit"`
	CostPerRequest      float64   `json:"cost_per_request"`
	CostPerToken        float64   `json:"cost_per_token"`
}

// ProviderCostBreakdown 提供商成本分布.
type ProviderCostBreakdown struct {
	Provider       string  `json:"provider"`
	RequestCount   int64   `json:"request_count"`
	InputTokens    int64   `json:"input_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
	InputCost      float64 `json:"input_cost"`
	OutputCost     float64 `json:"output_cost"`
	TotalCost      float64 `json:"total_cost"`
	PercentOfTotal float64 `json:"percent_of_total"`
}

// UserAIServiceCost 用户AI服务成本.
type UserAIServiceCost struct {
	UserID            string  `json:"user_id"`
	UserName          string  `json:"user_name"`
	RequestCount      int64   `json:"request_count"`
	InputTokens       int64   `json:"input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
	TotalCost         float64 `json:"total_cost"`
	CostPerRequest    float64 `json:"cost_per_request"`
	PreferredProvider string  `json:"preferred_provider"`
	PreferredModel    string  `json:"preferred_model"`
}

// ModelCostBreakdown 模型成本分布.
type ModelCostBreakdown struct {
	ModelName      string  `json:"model_name"`
	Provider       string  `json:"provider"`
	RequestCount   int64   `json:"request_count"`
	InputTokens    int64   `json:"input_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
	InputPrice     float64 `json:"input_price"`
	OutputPrice    float64 `json:"output_price"`
	TotalCost      float64 `json:"total_cost"`
	PercentOfTotal float64 `json:"percent_of_total"`
}

// TokenConsumptionStats Token消耗统计.
type TokenConsumptionStats struct {
	TotalInputTokens   int64             `json:"total_input_tokens"`
	TotalOutputTokens  int64             `json:"total_output_tokens"`
	TotalTokens        int64             `json:"total_tokens"`
	DailyInputTokens   float64           `json:"daily_input_tokens"`
	DailyOutputTokens  float64           `json:"daily_output_tokens"`
	DailyTotalTokens   float64           `json:"daily_total_tokens"`
	FunctionBreakdown  map[string]int64  `json:"function_breakdown"`
	GrowthRate         float64           `json:"growth_rate"`
}

// AICostTrendPoint AI成本趋势数据点.
type AICostTrendPoint struct {
	Date               time.Time `json:"date"`
	LocalInferenceCost float64   `json:"local_inference_cost"`
	CloudAPICost       float64   `json:"cloud_api_cost"`
	TotalCost          float64   `json:"total_cost"`
	RequestCount       int64     `json:"request_count"`
	InputTokens        int64     `json:"input_tokens"`
	OutputTokens       int64     `json:"output_tokens"`
}

// BudgetStatus 预算状态.
type BudgetStatus struct {
	MonthlyBudget     float64 `json:"monthly_budget"`
	CurrentSpending   float64 `json:"current_spending"`
	RemainingBudget   float64 `json:"remaining_budget"`
	PercentUsed       float64 `json:"percent_used"`
	DaysRemaining     int     `json:"days_remaining"`
	ProjectedSpending float64 `json:"projected_spending"`
	AlertLevel        string  `json:"alert_level"`
}

// ========== 勒索检测成本分析结构体 (v2.308.0新增) ==========

// RansomwareCostAnalysis 勒索检测成本分析结果.
type RansomwareCostAnalysis struct {
	AnalysisTime        time.Time `json:"analysis_time"`
	PeriodStart         time.Time `json:"period_start"`
	PeriodEnd           time.Time `json:"period_end"`
	TotalCost           float64   `json:"total_cost"`
	Currency            string    `json:"currency"`
	SignatureDBCost     float64   `json:"signature_db_cost"`
	QuarantineCost      float64   `json:"quarantine_cost"`
	EventLogCost        float64   `json:"event_log_cost"`
	AutoSnapshotCost    float64   `json:"auto_snapshot_cost"`
	SignatureDBAnalysis SignatureDBCostBreakdown  `json:"signature_db_analysis"`
	QuarantineAnalysis  QuarantineCostBreakdown   `json:"quarantine_analysis"`
	EventLogAnalysis    EventLogCostBreakdown     `json:"event_log_analysis"`
	SnapshotAnalysis    SnapshotCostBreakdown     `json:"snapshot_analysis"`
	AIServiceConsumption AIServiceRansomwareConsumption `json:"ai_service_consumption"`
	TrendData           []RansomwareCostTrendPoint `json:"trend_data"`
	Recommendations     []CostRecommendation       `json:"recommendations"`
}

// SignatureDBCostBreakdown 特征库成本明细.
type SignatureDBCostBreakdown struct {
	SizeMB             float64   `json:"size_mb"`
	UpdateFrequency    int       `json:"update_frequency_days"`
	LastUpdate         time.Time `json:"last_update"`
	SignatureCount     int64     `json:"signature_count"`
	FixedMonthlyCost   float64   `json:"fixed_monthly_cost"`
	NetworkTransferCost float64  `json:"network_transfer_cost"`
}

// QuarantineCostBreakdown 隔离区成本明细.
type QuarantineCostBreakdown struct {
	MaxCapacityGB    float64 `json:"max_capacity_gb"`
	CurrentUsageGB   float64 `json:"current_usage_gb"`
	UsagePercent     float64 `json:"usage_percent"`
	FileCount        int64   `json:"file_count"`
	AutoCleanupDays  int     `json:"auto_cleanup_days"`
	FilesCleaned     int64   `json:"files_cleaned"`
	SpaceReclaimedGB float64 `json:"space_reclaimed_gb"`
	PricePerGB       float64 `json:"price_per_gb"`
	MonthlyCost      float64 `json:"monthly_cost"`
	OptimizedCost    float64 `json:"optimized_cost"`
	SavingsPotential float64 `json:"savings_potential"`
}

// EventLogCostBreakdown 事件日志成本明细.
type EventLogCostBreakdown struct {
	DailyEventCount   int64   `json:"daily_event_count"`
	MonthlyEventCount int64   `json:"monthly_event_count"`
	EventSizeKB       float64 `json:"event_size_kb"`
	TotalSizeGB       float64 `json:"total_size_gb"`
	RetentionDays     int     `json:"retention_days"`
	CompressionRatio  float64 `json:"compression_ratio"`
	CompressedSizeGB  float64 `json:"compressed_size_gb"`
	PricePerGB        float64 `json:"price_per_gb"`
	MonthlyCost       float64 `json:"monthly_cost"`
	AfterCleanupCost  float64 `json:"after_cleanup_cost"`
}

// SnapshotCostBreakdown 快照成本明细.
type SnapshotCostBreakdown struct {
	Enabled              bool    `json:"enabled"`
	SnapshotTriggerCount int     `json:"snapshot_trigger_count"`
	SnapshotSizeGB       float64 `json:"snapshot_size_gb"`
	TotalSnapshotGB      float64 `json:"total_snapshot_gb"`
	RetentionDays        int     `json:"retention_days"`
	ActiveSnapshots      int     `json:"active_snapshots"`
	SnapshotPricePerGB   float64 `json:"snapshot_price_per_gb"`
	MonthlyCost          float64 `json:"monthly_cost"`
	OptimizedCost        float64 `json:"optimized_cost"`
}

// AIServiceRansomwareConsumption AI服务用于勒索检测的资源消耗.
type AIServiceRansomwareConsumption struct {
	Enabled             bool    `json:"enabled"`
	BehaviorAnalysisCount int64  `json:"behavior_analysis_count"`
	PatternMatchCount     int64  `json:"pattern_match_count"`
	InputTokens           int64  `json:"input_tokens"`
	OutputTokens          int64  `json:"output_tokens"`
	TokenCost             float64 `json:"token_cost"`
	PercentOfAIBudget     float64 `json:"percent_of_ai_budget"`
}

// RansomwareCostTrendPoint 勒索检测成本趋势数据点.
type RansomwareCostTrendPoint struct {
	Date            time.Time `json:"date"`
	SignatureDBCost float64   `json:"signature_db_cost"`
	QuarantineCost  float64   `json:"quarantine_cost"`
	EventLogCost    float64   `json:"event_log_cost"`
	SnapshotCost    float64   `json:"snapshot_cost"`
	TotalCost       float64   `json:"total_cost"`
	QuarantineGB    float64   `json:"quarantine_gb"`
	EventCount      int64     `json:"event_count"`
}

// ========== 成本分析器 ==========

// CostAnalyzer 成本分析器.
type CostAnalyzer struct {
	config *CostAnalyzerConfig
	mu     sync.RWMutex

	// 数据提供者
	storageProvider   StorageDataProvider
	bandwidthProvider BandwidthDataProvider
	accessTracker     AccessTracker

	// 缓存
	costCache      map[string]*StorageCostAnalysis
	bandwidthCache map[string]*BandwidthCostAnalysis
	cacheExpiry    time.Duration
}

// StorageDataProvider 存储数据提供者接口.
type StorageDataProvider interface {
	// 获取存储池列表
	GetPools(ctx context.Context) ([]StoragePoolInfo, error)

	// 获取存储池使用情况
	GetPoolUsage(ctx context.Context, poolID string) (*StoragePoolUsage, error)

	// 获取所有存储使用情况
	GetAllUsage(ctx context.Context) ([]*StorageUsageInfo, error)

	// 获取用户存储使用情况
	GetUserUsage(ctx context.Context, userID string) (*UserStorageUsage, error)
}

// StoragePoolInfo 存储池信息.
type StoragePoolInfo struct {
	PoolID         string `json:"pool_id"`
	PoolName       string `json:"pool_name"`
	StorageType    string `json:"storage_type"` // ssd, hdd, archive
	TotalBytes     uint64 `json:"total_bytes"`
	UsedBytes      uint64 `json:"used_bytes"`
	AvailableBytes uint64 `json:"available_bytes"`
}

// StoragePoolUsage 存储池使用情况.
type StoragePoolUsage struct {
	PoolID       string  `json:"pool_id"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	UsagePercent float64 `json:"usage_percent"`
	UserCount    int     `json:"user_count"`
}

// StorageUsageInfo 存储使用信息.
type StorageUsageInfo struct {
	UserID      string    `json:"user_id"`
	UserName    string    `json:"user_name"`
	PoolID      string    `json:"pool_id"`
	PoolName    string    `json:"pool_name"`
	UsedBytes   uint64    `json:"used_bytes"`
	AccessCount int64     `json:"access_count"` // 访问次数
	LastAccess  time.Time `json:"last_access"`
}

// UserStorageUsage 用户存储使用情况.
type UserStorageUsage struct {
	UserID      string            `json:"user_id"`
	UserName    string            `json:"user_name"`
	TotalBytes  uint64            `json:"total_bytes"`
	PoolUsage   map[string]uint64 `json:"pool_usage"` // poolID -> bytes
	AccessStats UserAccessStats   `json:"access_stats"`
}

// UserAccessStats 用户访问统计.
type UserAccessStats struct {
	DailyAccessCount   float64 `json:"daily_access_count"`
	WeeklyAccessCount  float64 `json:"weekly_access_count"`
	MonthlyAccessCount float64 `json:"monthly_access_count"`
	HotDataGB          float64 `json:"hot_data_gb"`
	WarmDataGB         float64 `json:"warm_data_gb"`
	ColdDataGB         float64 `json:"cold_data_gb"`
}

// BandwidthDataProvider 带宽数据提供者接口.
type BandwidthDataProvider interface {
	// 获取带宽数据
	GetBandwidthData(ctx context.Context, start, end time.Time) (*BandwidthData, error)

	// 获取小时分布
	GetHourlyDistribution(ctx context.Context, date time.Time) ([]HourlyBandwidth, error)

	// 获取峰值带宽
	GetPeakBandwidth(ctx context.Context, start, end time.Time) (float64, error)

	// 获取95峰值
	GetPeak95Bandwidth(ctx context.Context, start, end time.Time) (float64, error)
}

// BandwidthData 带宽数据.
type BandwidthData struct {
	TotalInboundBytes  uint64  `json:"total_inbound_bytes"`
	TotalOutboundBytes uint64  `json:"total_outbound_bytes"`
	PeakMbps           float64 `json:"peak_mbps"`
	AverageMbps        float64 `json:"average_mbps"`
	Peak95Mbps         float64 `json:"peak_95_mbps"`
}

// HourlyBandwidth 小时带宽.
type HourlyBandwidth struct {
	Hour          int     `json:"hour"`
	InboundBytes  uint64  `json:"inbound_bytes"`
	OutboundBytes uint64  `json:"outbound_bytes"`
	PeakMbps      float64 `json:"peak_mbps"`
}

// AccessTracker 访问追踪器接口.
type AccessTracker interface {
	// 获取访问频率
	GetAccessFrequency(ctx context.Context, userID, poolID string) (float64, error)

	// 获取访问分布
	GetAccessDistribution(ctx context.Context, poolID string) (*AccessDistribution, error)

	// 标记访问
	RecordAccess(ctx context.Context, userID, poolID string, bytes uint64) error
}

// AccessDistribution 访问分布.
type AccessDistribution struct {
	PoolID     string  `json:"pool_id"`
	HotDataGB  float64 `json:"hot_data_gb"`
	WarmDataGB float64 `json:"warm_data_gb"`
	ColdDataGB float64 `json:"cold_data_gb"`
	TotalGB    float64 `json:"total_gb"`
}

// NewCostAnalyzer 创建成本分析器.
func NewCostAnalyzer(config *CostAnalyzerConfig, storage StorageDataProvider, bandwidth BandwidthDataProvider, access AccessTracker) *CostAnalyzer {
	if config == nil {
		config = DefaultCostAnalyzerConfig()
	}

	return &CostAnalyzer{
		config:            config,
		storageProvider:   storage,
		bandwidthProvider: bandwidth,
		accessTracker:     access,
		costCache:         make(map[string]*StorageCostAnalysis),
		bandwidthCache:    make(map[string]*BandwidthCostAnalysis),
		cacheExpiry:       5 * time.Minute,
	}
}

// DefaultCostAnalyzerConfig 默认成本分析器配置.
func DefaultCostAnalyzerConfig() *CostAnalyzerConfig {
	return &CostAnalyzerConfig{
		Storage: StorageCostConfig{
			CapacityPricing: CapacityPricingConfig{
				BasePricePerGB:      0.1,
				EnableTieredPricing: true,
				TieredPricing: []CapacityTier{
					{MinGB: 0, MaxGB: 100, PricePerGB: 0.1},
					{MinGB: 100, MaxGB: 1000, PricePerGB: 0.08},
					{MinGB: 1000, MaxGB: -1, PricePerGB: 0.05},
				},
			},
			AccessFrequencyPricing: AccessFrequencyPricingConfig{
				Enabled:             true,
				HotDataPricePerGB:   0.15,
				WarmDataPricePerGB:  0.10,
				ColdDataPricePerGB:  0.03,
				HotAccessThreshold:  10,
				WarmAccessThreshold: 1,
			},
			SSDPricePerGB:     0.2,
			HDDPricePerGB:     0.05,
			ArchivePricePerGB: 0.01,
			FreeStorageGB:     10,
		},
		Bandwidth: BandwidthCostConfig{
			Model:                 BandwidthBillingTraffic,
			TrafficPricePerGB:     0.5,
			BandwidthPricePerMbps: 20,
			Peak95PricePerMbps:    15,
			FreeTrafficGB:         100,
			ChargeInbound:         false,
			ChargeOutbound:        true,
		},
		// AI服务成本默认配置 (v2.308.0新增)
		AIService: AIServiceCostConfig{
			Enabled:             true,
			MonthlyBudgetLimit:  500,   // 月预算500元
			PerRequestCostTarget: 0.05, // 单次成本目标0.05元
			LocalInference: LocalInferenceCostConfig{
				Enabled:               true,
				GPUModel:              "rtx4090",
				ElectricityPricePerKWh: 0.5,     // 电费0.5元/kWh
				GPUPowerConsumptionW:   450,     // GPU功耗450W
				DailyInferenceHours:    4,       // 日均4小时推理
				HardwareDepreciation:   100,     // 硬件折旧100元/月
			},
			CloudAPI: CloudAPICostConfig{
				Enabled:           true,
				DailyRequestLimit: 100,
				MonthlyTokenLimit: 1000000, // 月Token限额100万
				FreeTokensPerDay:  10000,   // 日免费额度1万
				ProviderPrices: []AIProviderPrice{
					{Provider: "baidu", Model: "ernie-3.5", InputPrice: 12, OutputPrice: 12, IsDefault: true, Priority: 1},
					{Provider: "aliyun", Model: "qwen-turbo", InputPrice: 2, OutputPrice: 6, IsDefault: false, Priority: 2},
					{Provider: "openai", Model: "gpt-4o-mini", InputPrice: 1.2, OutputPrice: 4.8, IsDefault: false, Priority: 3},
				},
			},
		},
		// 勒索检测成本默认配置 (v2.308.0新增)
		RansomwareDetection: RansomwareCostConfig{
			Enabled: true,
			SignatureDB: SignatureDBCostConfig{
				FixedSizeMB:        100,
				UpdateFrequencyDays: 7,
				MonthlyCostEstimate: 1, // 固定成本1元/月
			},
			Quarantine: QuarantineCostConfig{
				MaxCapacityGB:     10,
				PricePerGB:        0.05,
				AutoCleanupDays:   30,
				MonthlyCostEstimate: 0.5,
			},
			EventLog: EventLogCostConfig{
				DailyEventCount:   1000,
				EventSizeKB:       2,
				RetentionDays:     30,
				PricePerGB:        0.05,
				CompressionRatio:  0.3,
				MonthlyCostEstimate: 3,
			},
			AutoSnapshot: AutoSnapshotCostConfig{
				Enabled:           true,
				SnapshotPricePerGB: 0.1,
				RetentionDays:     7,
				MonthlyCostEstimate: 10,
			},
		},
		AnalysisPeriodDays: 30,
		AccessFrequencyTiers: []AccessFrequencyTier{
			{Name: "hot", MinAccessPerDay: 10, MaxAccessPerDay: -1, PriceMultiplier: 1.5},
			{Name: "warm", MinAccessPerDay: 1, MaxAccessPerDay: 10, PriceMultiplier: 1.0},
			{Name: "cold", MinAccessPerDay: 0, MaxAccessPerDay: 1, PriceMultiplier: 0.3},
		},
		DefaultCurrency: "CNY",
	}
}

// ========== 存储成本计算 ==========

// AnalyzeStorageCost 分析存储成本.
func (a *CostAnalyzer) AnalyzeStorageCost(ctx context.Context) (*StorageCostAnalysis, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	analysis := &StorageCostAnalysis{
		AnalysisTime:    now,
		PeriodStart:     now.AddDate(0, 0, -a.config.AnalysisPeriodDays),
		PeriodEnd:       now,
		Currency:        a.config.DefaultCurrency,
		PoolCosts:       make([]PoolStorageCost, 0),
		UserCosts:       make([]UserStorageCost, 0),
		TrendData:       make([]CostTrendPoint, 0),
		Recommendations: make([]CostRecommendation, 0),
	}

	// 获取所有存储使用情况
	usages, err := a.storageProvider.GetAllUsage(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取存储使用数据失败: %w", err)
	}

	// 按存储池和用户聚合
	poolData := make(map[string]*PoolStorageCost)
	userData := make(map[string]*UserStorageCost)

	for _, usage := range usages {
		// 按存储池聚合
		if _, exists := poolData[usage.PoolID]; !exists {
			poolData[usage.PoolID] = &PoolStorageCost{
				PoolID:      usage.PoolID,
				PoolName:    usage.PoolName,
				StorageType: "hdd", // 默认
			}
		}
		poolData[usage.PoolID].UsedCapacityGB += float64(usage.UsedBytes) / (1024 * 1024 * 1024)

		// 按用户聚合
		if _, exists := userData[usage.UserID]; !exists {
			userData[usage.UserID] = &UserStorageCost{
				UserID:        usage.UserID,
				UserName:      usage.UserName,
				PoolBreakdown: make(map[string]float64),
			}
		}
		userData[usage.UserID].UsedGB += float64(usage.UsedBytes) / (1024 * 1024 * 1024)
		userData[usage.UserID].PoolBreakdown[usage.PoolID] += float64(usage.UsedBytes) / (1024 * 1024 * 1024)

		// 计算访问频率成本
		if a.config.Storage.AccessFrequencyPricing.Enabled && a.accessTracker != nil {
			accessFreq, err := a.accessTracker.GetAccessFrequency(ctx, usage.UserID, usage.PoolID)
			if err == nil {
				// 根据访问频率分类
				gb := float64(usage.UsedBytes) / (1024 * 1024 * 1024)
				if accessFreq >= a.config.Storage.AccessFrequencyPricing.HotAccessThreshold {
					poolData[usage.PoolID].HotDataGB += gb
					analysis.AccessFrequencyAnalysis.HotDataGB += gb
				} else if accessFreq >= a.config.Storage.AccessFrequencyPricing.WarmAccessThreshold {
					poolData[usage.PoolID].WarmDataGB += gb
					analysis.AccessFrequencyAnalysis.WarmDataGB += gb
				} else {
					poolData[usage.PoolID].ColdDataGB += gb
					analysis.AccessFrequencyAnalysis.ColdDataGB += gb
				}
			}
		}
	}

	// 获取存储池信息并计算成本
	pools, err := a.storageProvider.GetPools(ctx)
	if err == nil {
		for _, pool := range pools {
			if p, exists := poolData[pool.PoolID]; exists {
				p.TotalCapacityGB = float64(pool.TotalBytes) / (1024 * 1024 * 1024)
				if p.TotalCapacityGB > 0 {
					p.UsagePercent = p.UsedCapacityGB / p.TotalCapacityGB * 100
				}
				p.StorageType = pool.StorageType

				// 设置价格
				p.PricePerGB = a.getStoragePrice(pool.StorageType)

				// 计算成本
				p.MonthlyCost = a.calculatePoolCost(p)
				p.CostEfficiency = a.calculateCostEfficiency(p.UsagePercent)
				p.OptimizationScore = a.calculateOptimizationScore(p)

				analysis.PoolCosts = append(analysis.PoolCosts, *p)
			}
		}
	}

	// 计算用户成本
	for _, u := range userData {
		u.MonthlyCost = a.calculateCapacityCost(u.UsedGB)
		if u.UsedGB > 0 {
			u.CostPerGB = u.MonthlyCost / u.UsedGB
		}
		analysis.UserCosts = append(analysis.UserCosts, *u)
	}

	// 计算总体容量成本
	var totalUsedGB float64
	for _, p := range analysis.PoolCosts {
		totalUsedGB += p.UsedCapacityGB
	}

	analysis.CapacityAnalysis = CapacityCostAnalysis{
		TotalCapacityGB: totalUsedGB + a.getFreeCapacity(),
		UsedCapacityGB:  totalUsedGB,
		FreeCapacityGB:  a.getFreeCapacity(),
		UtilizationRate: 0,
		MonthlyCost:     a.calculateCapacityCost(totalUsedGB),
		DailyCost:       a.calculateCapacityCost(totalUsedGB) / 30,
		CostPerGB:       a.config.Storage.CapacityPricing.BasePricePerGB,
		TierBreakdown:   a.calculateTierBreakdown(totalUsedGB),
	}

	if analysis.CapacityAnalysis.TotalCapacityGB > 0 {
		analysis.CapacityAnalysis.UtilizationRate = totalUsedGB / analysis.CapacityAnalysis.TotalCapacityGB * 100
	}

	// 计算访问频率成本
	analysis.AccessFrequencyAnalysis.Enabled = a.config.Storage.AccessFrequencyPricing.Enabled
	analysis.AccessFrequencyAnalysis.TotalDataGB = analysis.AccessFrequencyAnalysis.HotDataGB +
		analysis.AccessFrequencyAnalysis.WarmDataGB + analysis.AccessFrequencyAnalysis.ColdDataGB

	if analysis.AccessFrequencyAnalysis.TotalDataGB > 0 {
		analysis.AccessFrequencyAnalysis.HotPercent = analysis.AccessFrequencyAnalysis.HotDataGB / analysis.AccessFrequencyAnalysis.TotalDataGB * 100
		analysis.AccessFrequencyAnalysis.WarmPercent = analysis.AccessFrequencyAnalysis.WarmDataGB / analysis.AccessFrequencyAnalysis.TotalDataGB * 100
		analysis.AccessFrequencyAnalysis.ColdPercent = analysis.AccessFrequencyAnalysis.ColdDataGB / analysis.AccessFrequencyAnalysis.TotalDataGB * 100
	}

	// 计算访问频率成本
	analysis.AccessFrequencyAnalysis.HotDataCost = analysis.AccessFrequencyAnalysis.HotDataGB * a.config.Storage.AccessFrequencyPricing.HotDataPricePerGB
	analysis.AccessFrequencyAnalysis.WarmDataCost = analysis.AccessFrequencyAnalysis.WarmDataGB * a.config.Storage.AccessFrequencyPricing.WarmDataPricePerGB
	analysis.AccessFrequencyAnalysis.ColdDataCost = analysis.AccessFrequencyAnalysis.ColdDataGB * a.config.Storage.AccessFrequencyPricing.ColdDataPricePerGB
	analysis.AccessFrequencyAnalysis.TotalCost = analysis.AccessFrequencyAnalysis.HotDataCost +
		analysis.AccessFrequencyAnalysis.WarmDataCost + analysis.AccessFrequencyAnalysis.ColdDataCost

	// 计算优化潜力
	analysis.AccessFrequencyAnalysis.OptimizationPotential = a.calculateAccessOptimizationPotential(&analysis.AccessFrequencyAnalysis)

	// 计算总成本
	analysis.TotalCapacityCost = analysis.CapacityAnalysis.MonthlyCost
	analysis.TotalAccessCost = analysis.AccessFrequencyAnalysis.TotalCost
	analysis.TotalCost = analysis.TotalCapacityCost + analysis.TotalAccessCost

	// 生成建议
	analysis.Recommendations = a.generateStorageRecommendations(analysis)

	return analysis, nil
}

// CalculateStorageCostByCapacity 按容量计算存储成本.
func (a *CostAnalyzer) CalculateStorageCostByCapacity(gb float64) (float64, error) {
	if gb < 0 {
		return 0, ErrInvalidCostConfig
	}

	// 扣除免费额度
	if gb <= a.config.Storage.FreeStorageGB {
		return 0, nil
	}
	gb -= a.config.Storage.FreeStorageGB

	return a.calculateCapacityCost(gb), nil
}

// CalculateStorageCostByAccessFrequency 按访问频率计算存储成本.
func (a *CostAnalyzer) CalculateStorageCostByAccessFrequency(hotGB, warmGB, coldGB float64) (float64, error) {
	if !a.config.Storage.AccessFrequencyPricing.Enabled {
		return 0, nil
	}

	cost := hotGB * a.config.Storage.AccessFrequencyPricing.HotDataPricePerGB
	cost += warmGB * a.config.Storage.AccessFrequencyPricing.WarmDataPricePerGB
	cost += coldGB * a.config.Storage.AccessFrequencyPricing.ColdDataPricePerGB

	return cost, nil
}

// CalculateStorageCost 混合计算存储成本.
func (a *CostAnalyzer) CalculateStorageCost(ctx context.Context, userID string) (*UserStorageCost, error) {
	usage, err := a.storageProvider.GetUserUsage(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户存储使用数据失败: %w", err)
	}

	cost := &UserStorageCost{
		UserID:        userID,
		UserName:      usage.UserName,
		UsedGB:        float64(usage.TotalBytes) / (1024 * 1024 * 1024),
		PoolBreakdown: make(map[string]float64),
	}

	// 计算各存储池用量
	for poolID, bytes := range usage.PoolUsage {
		cost.PoolBreakdown[poolID] = float64(bytes) / (1024 * 1024 * 1024)
	}

	// 计算基础容量成本
	cost.MonthlyCost = a.calculateCapacityCost(cost.UsedGB)

	// 计算访问频率附加成本
	if a.config.Storage.AccessFrequencyPricing.Enabled && a.accessTracker != nil {
		accessFreq, err := a.accessTracker.GetAccessFrequency(ctx, userID, "")
		if err == nil {
			if accessFreq >= a.config.Storage.AccessFrequencyPricing.HotAccessThreshold {
				cost.Tier = "hot"
				cost.MonthlyCost += cost.UsedGB * (a.config.Storage.AccessFrequencyPricing.HotDataPricePerGB - a.config.Storage.CapacityPricing.BasePricePerGB)
			} else if accessFreq >= a.config.Storage.AccessFrequencyPricing.WarmAccessThreshold {
				cost.Tier = "warm"
			} else {
				cost.Tier = "cold"
				cost.MonthlyCost = cost.UsedGB * a.config.Storage.AccessFrequencyPricing.ColdDataPricePerGB
			}
		}
	}

	if cost.UsedGB > 0 {
		cost.CostPerGB = cost.MonthlyCost / cost.UsedGB
	}

	return cost, nil
}

// ========== 带宽成本计算 ==========

// AnalyzeBandwidthCost 分析带宽成本.
func (a *CostAnalyzer) AnalyzeBandwidthCost(ctx context.Context, start, end time.Time) (*BandwidthCostAnalysis, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	analysis := &BandwidthCostAnalysis{
		AnalysisTime:       time.Now(),
		PeriodStart:        start,
		PeriodEnd:          end,
		Currency:           a.config.DefaultCurrency,
		BillingModel:       string(a.config.Bandwidth.Model),
		HourlyDistribution: make([]HourlyBandwidthCost, 24),
		DailyDistribution:  make([]DailyBandwidthCost, 0),
		TrendData:          make([]CostTrendPoint, 0),
		Recommendations:    make([]CostRecommendation, 0),
	}

	// 获取带宽数据
	data, err := a.bandwidthProvider.GetBandwidthData(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("获取带宽数据失败: %w", err)
	}

	// 转换为GB
	analysis.InboundTrafficGB = float64(data.TotalInboundBytes) / (1024 * 1024 * 1024)
	analysis.OutboundTrafficGB = float64(data.TotalOutboundBytes) / (1024 * 1024 * 1024)
	analysis.TotalTrafficGB = analysis.InboundTrafficGB + analysis.OutboundTrafficGB
	analysis.PeakBandwidthMbps = data.PeakMbps
	analysis.AverageMbps = data.AverageMbps
	analysis.Peak95Mbps = data.Peak95Mbps

	// 计算免费额度
	analysis.FreeAllowanceGB = a.config.Bandwidth.FreeTrafficGB

	// 根据计费模式计算成本
	switch a.config.Bandwidth.Model {
	case BandwidthBillingTraffic:
		analysis = a.calculateTrafficBasedCost(analysis)
	case BandwidthBillingPeak:
		analysis = a.calculatePeakBasedCost(analysis)
	case BandwidthBillingPeak95:
		analysis = a.calculatePeak95BasedCost(analysis)
	case BandwidthBillingMonthly:
		analysis = a.calculateMonthlyBasedCost(analysis)
	case BandwidthBillingHybrid:
		analysis = a.calculateHybridBasedCost(analysis)
	default:
		analysis = a.calculateTrafficBasedCost(analysis)
	}

	// 获取小时分布
	hourlyData, err := a.bandwidthProvider.GetHourlyDistribution(ctx, time.Now())
	if err == nil {
		for _, h := range hourlyData {
			if h.Hour >= 0 && h.Hour < 24 {
				analysis.HourlyDistribution[h.Hour] = HourlyBandwidthCost{
					Hour:       h.Hour,
					InboundGB:  float64(h.InboundBytes) / (1024 * 1024 * 1024),
					OutboundGB: float64(h.OutboundBytes) / (1024 * 1024 * 1024),
					PeakMbps:   h.PeakMbps,
				}
			}
		}
	}

	// 生成建议
	analysis.Recommendations = a.generateBandwidthRecommendations(analysis)

	return analysis, nil
}

// EstimateBandwidthCost 估算带宽成本.
func (a *CostAnalyzer) EstimateBandwidthCost(trafficGB, peakMbps float64) (float64, error) {
	if trafficGB < 0 || peakMbps < 0 {
		return 0, ErrInvalidCostConfig
	}

	var cost float64

	switch a.config.Bandwidth.Model {
	case BandwidthBillingTraffic:
		chargedTraffic := trafficGB - a.config.Bandwidth.FreeTrafficGB
		if chargedTraffic > 0 {
			cost = chargedTraffic * a.config.Bandwidth.TrafficPricePerGB
		}
	case BandwidthBillingPeak:
		chargedBandwidth := peakMbps - a.config.Bandwidth.FreeBandwidthMbps
		if chargedBandwidth > 0 {
			cost = chargedBandwidth * a.config.Bandwidth.BandwidthPricePerMbps
		}
	case BandwidthBillingPeak95:
		cost = peakMbps * a.config.Bandwidth.Peak95PricePerMbps
	case BandwidthBillingMonthly:
		cost = peakMbps * a.config.Bandwidth.BandwidthPricePerMbps
	case BandwidthBillingHybrid:
		// 混合计费：带宽基础费 + 流量费
		baseCost := peakMbps * a.config.Bandwidth.BandwidthPricePerMbps * 0.5
		trafficCost := trafficGB * a.config.Bandwidth.TrafficPricePerGB * 0.5
		cost = baseCost + trafficCost
	}

	return cost, nil
}

// ========== 辅助方法 ==========

// calculateCapacityCost 计算容量成本.
func (a *CostAnalyzer) calculateCapacityCost(gb float64) float64 {
	if gb <= 0 {
		return 0
	}

	// 扣除免费额度
	if gb <= a.config.Storage.FreeStorageGB {
		return 0
	}
	gb -= a.config.Storage.FreeStorageGB

	// 阶梯定价
	if a.config.Storage.CapacityPricing.EnableTieredPricing && len(a.config.Storage.CapacityPricing.TieredPricing) > 0 {
		return a.calculateTieredCapacityCost(gb, a.config.Storage.CapacityPricing.TieredPricing)
	}

	return gb * a.config.Storage.CapacityPricing.BasePricePerGB
}

// calculateTieredCapacityCost 阶梯容量成本计算.
func (a *CostAnalyzer) calculateTieredCapacityCost(gb float64, tiers []CapacityTier) float64 {
	var totalCost float64
	remaining := gb

	for _, tier := range tiers {
		if remaining <= 0 {
			break
		}

		tierSize := tier.MaxGB - tier.MinGB
		if tier.MaxGB < 0 {
			// 无限阶梯
			totalCost += remaining * tier.PricePerGB
			break
		}

		if remaining <= tierSize {
			totalCost += remaining * tier.PricePerGB
			break
		}

		totalCost += tierSize * tier.PricePerGB
		remaining -= tierSize
	}

	return totalCost
}

// calculatePoolCost 计算存储池成本.
func (a *CostAnalyzer) calculatePoolCost(p *PoolStorageCost) float64 {
	baseCost := p.UsedCapacityGB * p.PricePerGB

	// 如果启用访问频率计费，加上附加成本
	if a.config.Storage.AccessFrequencyPricing.Enabled {
		baseCost += p.HotDataGB * (a.config.Storage.AccessFrequencyPricing.HotDataPricePerGB - p.PricePerGB)
	}

	return baseCost
}

// getStoragePrice 获取存储价格.
func (a *CostAnalyzer) getStoragePrice(storageType string) float64 {
	switch storageType {
	case "ssd":
		return a.config.Storage.SSDPricePerGB
	case "archive":
		return a.config.Storage.ArchivePricePerGB
	default:
		return a.config.Storage.HDDPricePerGB
	}
}

// getFreeCapacity 获取免费容量.
func (a *CostAnalyzer) getFreeCapacity() float64 {
	return a.config.Storage.FreeStorageGB
}

// calculateTierBreakdown 计算阶梯成本明细.
func (a *CostAnalyzer) calculateTierBreakdown(totalGB float64) []CapacityTierCost {
	breakdown := make([]CapacityTierCost, 0)

	if !a.config.Storage.CapacityPricing.EnableTieredPricing {
		breakdown = append(breakdown, CapacityTierCost{
			TierName:   "基础",
			UsedGB:     totalGB,
			PricePerGB: a.config.Storage.CapacityPricing.BasePricePerGB,
			Cost:       totalGB * a.config.Storage.CapacityPricing.BasePricePerGB,
		})
		return breakdown
	}

	remaining := totalGB
	for i, tier := range a.config.Storage.CapacityPricing.TieredPricing {
		if remaining <= 0 {
			break
		}

		tierSize := tier.MaxGB - tier.MinGB
		var usedInTier float64

		if tier.MaxGB < 0 {
			usedInTier = remaining
		} else if remaining <= tierSize {
			usedInTier = remaining
		} else {
			usedInTier = tierSize
		}

		breakdown = append(breakdown, CapacityTierCost{
			TierName:   fmt.Sprintf("阶梯%d", i+1),
			MinGB:      tier.MinGB,
			MaxGB:      tier.MaxGB,
			UsedGB:     usedInTier,
			PricePerGB: tier.PricePerGB,
			Cost:       usedInTier * tier.PricePerGB,
		})

		remaining -= usedInTier
	}

	return breakdown
}

// calculateCostEfficiency 计算成本效率.
func (a *CostAnalyzer) calculateCostEfficiency(usagePercent float64) float64 {
	// 理想利用率 60-80%
	if usagePercent >= 60 && usagePercent <= 80 {
		return 1.0
	} else if usagePercent > 80 {
		return math.Max(0, 1-(usagePercent-80)/100)
	} else {
		return usagePercent / 60
	}
}

// calculateOptimizationScore 计算优化评分.
func (a *CostAnalyzer) calculateOptimizationScore(p *PoolStorageCost) float64 {
	score := 100.0

	// 利用率过低扣分
	if p.UsagePercent < 30 {
		score -= (30 - p.UsagePercent)
	}

	// 利用率过高扣分
	if p.UsagePercent > 90 {
		score -= (p.UsagePercent - 90)
	}

	// 冷数据过多扣分（应该归档）
	coldPercent := p.ColdDataGB / p.UsedCapacityGB * 100
	if coldPercent > 50 {
		score -= (coldPercent - 50) / 2
	}

	return math.Max(0, score)
}

// calculateAccessOptimizationPotential 计算访问优化潜力.
func (a *CostAnalyzer) calculateAccessOptimizationPotential(afa *AccessFrequencyCostAnalysis) float64 {
	// 冷数据如果使用热存储价格，则有优化空间
	// 假设冷数据使用热存储价格会有30%的浪费
	potential := afa.ColdDataGB * (a.config.Storage.AccessFrequencyPricing.HotDataPricePerGB - a.config.Storage.AccessFrequencyPricing.ColdDataPricePerGB) * 0.3
	return potential
}

// generateStorageRecommendations 生成存储建议.
func (a *CostAnalyzer) generateStorageRecommendations(analysis *StorageCostAnalysis) []CostRecommendation {
	recs := make([]CostRecommendation, 0)

	// 检查低利用率存储池
	for _, p := range analysis.PoolCosts {
		if p.UsagePercent < 30 {
			recs = append(recs, CostRecommendation{
				ID:          generateCostRecID(),
				Type:        "storage",
				Priority:    "medium",
				Title:       fmt.Sprintf("存储池 %s 利用率较低", p.PoolName),
				Description: fmt.Sprintf("存储池利用率仅 %.1f%%，建议考虑资源整合或降配", p.UsagePercent),
				Action:      "评估存储池使用情况，考虑整合或降配",
				Impact:      "可降低存储成本",
			})
		}

		// 检查冷数据归档机会
		coldPercent := p.ColdDataGB / p.UsedCapacityGB * 100
		if coldPercent > 30 && p.StorageType != "archive" {
			potentialSavings := p.ColdDataGB * (p.PricePerGB - a.config.Storage.ArchivePricePerGB)
			recs = append(recs, CostRecommendation{
				ID:               generateCostRecID(),
				Type:             "storage",
				Priority:         "high",
				Title:            fmt.Sprintf("存储池 %s 存在大量冷数据", p.PoolName),
				Description:      fmt.Sprintf("冷数据占比 %.1f%%，建议迁移至归档存储", coldPercent),
				PotentialSavings: potentialSavings,
				CurrentCost:      p.ColdDataGB * p.PricePerGB,
				OptimizedCost:    p.ColdDataGB * a.config.Storage.ArchivePricePerGB,
				Action:           "将冷数据迁移至归档存储",
				Impact:           "可显著降低存储成本",
			})
		}
	}

	// 检查容量成本优化
	if analysis.CapacityAnalysis.UtilizationRate < 50 {
		recs = append(recs, CostRecommendation{
			ID:          generateCostRecID(),
			Type:        "storage",
			Priority:    "low",
			Title:       "整体存储利用率偏低",
			Description: fmt.Sprintf("整体利用率仅 %.1f%%，资源可能过剩", analysis.CapacityAnalysis.UtilizationRate),
			Action:      "评估是否需要减少存储采购或优化资源分配",
			Impact:      "提高资源利用效率",
		})
	}

	return recs
}

// generateBandwidthRecommendations 生成带宽建议.
func (a *CostAnalyzer) generateBandwidthRecommendations(analysis *BandwidthCostAnalysis) []CostRecommendation {
	recs := make([]CostRecommendation, 0)

	// 检查是否可以优化计费模式
	if a.config.Bandwidth.Model == BandwidthBillingTraffic && analysis.TotalTrafficGB > 1000 {
		// 大流量场景建议切换到峰值计费
		trafficCost := analysis.TotalTrafficGB * a.config.Bandwidth.TrafficPricePerGB
		peakCost := analysis.PeakBandwidthMbps * a.config.Bandwidth.BandwidthPricePerMbps
		if peakCost < trafficCost*0.8 {
			recs = append(recs, CostRecommendation{
				ID:               generateCostRecID(),
				Type:             "bandwidth",
				Priority:         "medium",
				Title:            "建议切换带宽计费模式",
				Description:      "当前流量较大，峰值计费可能更经济",
				PotentialSavings: trafficCost - peakCost,
				CurrentCost:      trafficCost,
				OptimizedCost:    peakCost,
				Action:           "评估切换到峰值带宽计费模式",
				Impact:           "可降低带宽成本",
			})
		}
	}

	// 检查峰值带宽是否过高
	if analysis.PeakBandwidthMbps > analysis.AverageMbps*3 {
		recs = append(recs, CostRecommendation{
			ID:          generateCostRecID(),
			Type:        "bandwidth",
			Priority:    "high",
			Title:       "带宽峰值波动较大",
			Description: fmt.Sprintf("峰值 %.1f Mbps 远高于均值 %.1f Mbps，建议优化流量调度", analysis.PeakBandwidthMbps, analysis.AverageMbps),
			Action:      "分析流量高峰时段，考虑负载均衡或流量整形",
			Impact:      "可降低峰值带宽成本",
		})
	}

	return recs
}

// calculateTrafficBasedCost 按流量计费.
func (a *CostAnalyzer) calculateTrafficBasedCost(analysis *BandwidthCostAnalysis) *BandwidthCostAnalysis {
	// 计算需要计费的流量
	var chargedTraffic float64
	if a.config.Bandwidth.ChargeOutbound {
		chargedTraffic += analysis.OutboundTrafficGB
	}
	if a.config.Bandwidth.ChargeInbound {
		chargedTraffic += analysis.InboundTrafficGB
	}

	analysis.ChargedTrafficGB = chargedTraffic

	// 扣除免费额度
	if chargedTraffic > a.config.Bandwidth.FreeTrafficGB {
		chargedTraffic -= a.config.Bandwidth.FreeTrafficGB
		analysis.OverageCost = chargedTraffic * a.config.Bandwidth.TrafficPricePerGB
	}

	analysis.TrafficCost = analysis.OverageCost
	analysis.TotalCost = analysis.TrafficCost

	return analysis
}

// calculatePeakBasedCost 按峰值带宽计费.
func (a *CostAnalyzer) calculatePeakBasedCost(analysis *BandwidthCostAnalysis) *BandwidthCostAnalysis {
	chargedBandwidth := analysis.PeakBandwidthMbps - a.config.Bandwidth.FreeBandwidthMbps
	if chargedBandwidth < 0 {
		chargedBandwidth = 0
	}

	analysis.BandwidthCost = chargedBandwidth * a.config.Bandwidth.BandwidthPricePerMbps
	analysis.TotalCost = analysis.BandwidthCost

	return analysis
}

// calculatePeak95BasedCost 按95峰值计费.
func (a *CostAnalyzer) calculatePeak95BasedCost(analysis *BandwidthCostAnalysis) *BandwidthCostAnalysis {
	analysis.BandwidthCost = analysis.Peak95Mbps * a.config.Bandwidth.Peak95PricePerMbps
	analysis.TotalCost = analysis.BandwidthCost

	return analysis
}

// calculateMonthlyBasedCost 按月固定带宽计费.
func (a *CostAnalyzer) calculateMonthlyBasedCost(analysis *BandwidthCostAnalysis) *BandwidthCostAnalysis {
	analysis.BandwidthCost = analysis.PeakBandwidthMbps * a.config.Bandwidth.BandwidthPricePerMbps
	analysis.TotalCost = analysis.BandwidthCost

	return analysis
}

// calculateHybridBasedCost 混合计费.
func (a *CostAnalyzer) calculateHybridBasedCost(analysis *BandwidthCostAnalysis) *BandwidthCostAnalysis {
	// 50% 峰值 + 50% 流量
	peakCost := analysis.PeakBandwidthMbps * a.config.Bandwidth.BandwidthPricePerMbps * 0.5

	var trafficCost float64
	if analysis.TotalTrafficGB > a.config.Bandwidth.FreeTrafficGB {
		trafficCost = (analysis.TotalTrafficGB - a.config.Bandwidth.FreeTrafficGB) * a.config.Bandwidth.TrafficPricePerGB * 0.5
	}

	analysis.BandwidthCost = peakCost
	analysis.TrafficCost = trafficCost
	analysis.TotalCost = peakCost + trafficCost

	return analysis
}

// generateCostRecID 生成成本建议ID.
func generateCostRecID() string {
	return fmt.Sprintf("cost-rec-%d", time.Now().UnixNano())
}

// GetConfig 获取配置.
func (a *CostAnalyzer) GetConfig() *CostAnalyzerConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config
}

// UpdateConfig 更新配置.
func (a *CostAnalyzer) UpdateConfig(config *CostAnalyzerConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if config == nil {
		return ErrInvalidCostConfig
	}

	a.config = config
	return nil
}

// ========== AI服务成本分析方法 (v2.308.0新增) ==========

// AIServiceDataProvider AI服务数据提供者接口.
type AIServiceDataProvider interface {
	// 获取AI调用统计
	GetAIUsageStats(ctx context.Context, start, end time.Time) (*AIUsageData, error)

	// 获取用户AI使用情况
	GetUserAIUsage(ctx context.Context, userID string) (*UserAIUsageData, error)

	// 获取Token消耗统计
	GetTokenConsumption(ctx context.Context, start, end time.Time) (*TokenConsumptionData, error)
}

// AIUsageData AI使用数据.
type AIUsageData struct {
	// 本地推理
	LocalInferenceHours float64 `json:"local_inference_hours"`
	LocalRequestCount   int64   `json:"local_request_count"`

	// 云端API
	CloudRequestCount int64 `json:"cloud_request_count"`
	InputTokens       int64 `json:"input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`

	// 按提供商分布
	ProviderStats []ProviderUsageStats `json:"provider_stats"`

	// GPU信息
	GPUModel       string  `json:"gpu_model"`
	GPUPowerW      float64 `json:"gpu_power_w"`
	GPUUtilization float64 `json:"gpu_utilization"`
}

// ProviderUsageStats 提供商使用统计.
type ProviderUsageStats struct {
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	RequestCount   int64  `json:"request_count"`
	InputTokens    int64  `json:"input_tokens"`
	OutputTokens   int64  `json:"output_tokens"`
}

// UserAIUsageData 用户AI使用数据.
type UserAIUsageData struct {
	UserID          string `json:"user_id"`
	UserName        string `json:"user_name"`
	RequestCount    int64  `json:"request_count"`
	InputTokens     int64  `json:"input_tokens"`
	OutputTokens    int64  `json:"output_tokens"`
	PreferredProvider string `json:"preferred_provider"`
	PreferredModel    string `json:"preferred_model"`
}

// TokenConsumptionData Token消耗数据.
type TokenConsumptionData struct {
	TotalInputTokens  int64              `json:"total_input_tokens"`
	TotalOutputTokens int64              `json:"total_output_tokens"`
	FunctionBreakdown map[string]int64   `json:"function_breakdown"`
	GrowthRate        float64            `json:"growth_rate"`
}

// AnalyzeAIServiceCost 分析AI服务成本.
func (a *CostAnalyzer) AnalyzeAIServiceCost(ctx context.Context, provider AIServiceDataProvider) (*AIServiceCostAnalysis, error) {
	if !a.config.AIService.Enabled {
		return nil, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	start := now.AddDate(0, 0, -a.config.AnalysisPeriodDays)

	analysis := &AIServiceCostAnalysis{
		AnalysisTime:  now,
		PeriodStart:   start,
		PeriodEnd:     now,
		Currency:      a.config.DefaultCurrency,
		UserAICosts:   make([]UserAIServiceCost, 0),
		ModelCosts:    make([]ModelCostBreakdown, 0),
		TrendData:     make([]AICostTrendPoint, 0),
		Recommendations: make([]CostRecommendation, 0),
	}

	// 获取AI使用数据
	usageData, err := provider.GetAIUsageStats(ctx, start, now)
	if err != nil {
		return nil, fmt.Errorf("获取AI使用数据失败: %w", err)
	}

	// 计算本地推理成本
	analysis.LocalAnalysis = a.calculateLocalInferenceCost(usageData)
	analysis.LocalInferenceCost = analysis.LocalAnalysis.MonthlyCost

	// 计算云端API成本
	analysis.CloudAnalysis = a.calculateCloudAPICost(usageData)
	analysis.CloudAPICost = analysis.CloudAnalysis.TotalTokenCost

	// 计算总成本
	analysis.TotalCost = analysis.LocalInferenceCost + analysis.CloudAPICost

	// 计算预算状态
	analysis.BudgetStatus = a.calculateBudgetStatus(analysis.TotalCost)

	// 生成建议
	analysis.Recommendations = a.generateAIRecommendations(analysis)

	return analysis, nil
}

// calculateLocalInferenceCost 计算本地推理成本.
func (a *CostAnalyzer) calculateLocalInferenceCost(data *AIUsageData) LocalInferenceAnalysis {
	config := a.config.AIService.LocalInference

	analysis := LocalInferenceAnalysis{
		GPUModel:        config.GPUModel,
		GPUPowerW:       config.GPUPowerConsumptionW,
		GPUUtilization:  data.GPUUtilization,
		DailyHours:      data.LocalInferenceHours / float64(a.config.AnalysisPeriodDays),
		MonthlyHours:    data.LocalInferenceHours * 30 / float64(a.config.AnalysisPeriodDays),
		RequestCount:    data.LocalRequestCount,
		ElectricityPrice: config.ElectricityPricePerKWh,
		HardwareDepreciation: config.HardwareDepreciation,
	}

	// 计算电费成本 (功率 * 小时 * 电价)
	// 功率转换为kW，小时按月计算
	analysis.ElectricityCost = (config.GPUPowerConsumptionW / 1000) * analysis.MonthlyHours * config.ElectricityPricePerKWh

	// 总成本 = 电费 + 硬件折旧
	analysis.MonthlyCost = analysis.ElectricityCost + config.HardwareDepreciation

	// 单次请求成本
	if analysis.RequestCount > 0 {
		analysis.CostPerRequest = analysis.MonthlyCost / float64(analysis.RequestCount)
	}

	return analysis
}

// calculateCloudAPICost 计算云端API成本.
func (a *CostAnalyzer) calculateCloudAPICost(data *AIUsageData) CloudAPIAnalysis {
	config := a.config.AIService.CloudAPI

	analysis := CloudAPIAnalysis{
		DailyRequestCount:   data.CloudRequestCount / int64(a.config.AnalysisPeriodDays),
		MonthlyRequestCount: data.CloudRequestCount * 30 / int64(a.config.AnalysisPeriodDays),
		InputTokens:         data.InputTokens,
		OutputTokens:        data.OutputTokens,
		TotalTokens:         data.InputTokens + data.OutputTokens,
		FreeTokensLimit:     config.MonthlyTokenLimit,
		ProviderBreakdown:   make([]ProviderCostBreakdown, 0),
	}

	// 计算Token成本
	totalInputCost := 0.0
	totalOutputCost := 0.0

	for _, stats := range data.ProviderStats {
		// 查找对应的价格配置
		var inputPrice, outputPrice float64
		for _, price := range config.ProviderPrices {
			if price.Provider == stats.Provider && price.Model == stats.Model {
				inputPrice = price.InputPrice
				outputPrice = price.OutputPrice
				break
			}
		}

		// 计算成本 (价格单位: CNY/万tokens, Token单位: 个)
		inputCost := float64(stats.InputTokens) * inputPrice / 10000
		outputCost := float64(stats.OutputTokens) * outputPrice / 10000

		totalInputCost += inputCost
		totalOutputCost += outputCost

		analysis.ProviderBreakdown = append(analysis.ProviderBreakdown, ProviderCostBreakdown{
			Provider:     stats.Provider,
			RequestCount: stats.RequestCount,
			InputTokens:  stats.InputTokens,
			OutputTokens: stats.OutputTokens,
			InputCost:    inputCost,
			OutputCost:   outputCost,
			TotalCost:    inputCost + outputCost,
		})
	}

	analysis.InputTokenCost = totalInputCost
	analysis.OutputTokenCost = totalOutputCost
	analysis.TotalTokenCost = totalInputCost + totalOutputCost

	// 计算单次请求成本和单Token成本
	if analysis.MonthlyRequestCount > 0 {
		analysis.CostPerRequest = analysis.TotalTokenCost / float64(analysis.MonthlyRequestCount)
	}
	if analysis.TotalTokens > 0 {
		analysis.CostPerToken = analysis.TotalTokenCost / float64(analysis.TotalTokens)
	}

	return analysis
}

// calculateBudgetStatus 计算预算状态.
func (a *CostAnalyzer) calculateBudgetStatus(currentCost float64) BudgetStatus {
	budget := a.config.AIService.MonthlyBudgetLimit

	// 计算月剩余天数
	now := time.Now()
	daysRemaining := 30 - now.Day()
	if daysRemaining < 0 {
		daysRemaining = 0
	}

	// 预测月总支出 (当前成本 * 30 / 已过天数)
	passedDays := now.Day()
	if passedDays == 0 {
		passedDays = 1
	}
	projectedSpending := currentCost * 30 / float64(passedDays)

	status := BudgetStatus{
		MonthlyBudget:     budget,
		CurrentSpending:    currentCost,
		RemainingBudget:   budget - currentCost,
		PercentUsed:       currentCost / budget * 100,
		DaysRemaining:     daysRemaining,
		ProjectedSpending: projectedSpending,
	}

	// 确定告警级别
	if projectedSpending > budget * 1.2 {
		status.AlertLevel = "critical"
	} else if projectedSpending > budget {
		status.AlertLevel = "warning"
	} else {
		status.AlertLevel = "normal"
	}

	return status
}

// generateAIRecommendations 生成AI成本建议.
func (a *CostAnalyzer) generateAIRecommendations(analysis *AIServiceCostAnalysis) []CostRecommendation {
	recs := make([]CostRecommendation, 0)

	// 检查预算状态
	if analysis.BudgetStatus.AlertLevel == "critical" {
		recs = append(recs, CostRecommendation{
			ID:          generateCostRecID(),
			Type:        "ai_service",
			Priority:    "high",
			Title:       "AI服务预算即将超限",
			Description: fmt.Sprintf("预计月支出 %.2f 元，超出预算 %.2f 元的 %.0f%%", 
				analysis.BudgetStatus.ProjectedSpending, 
				analysis.BudgetStatus.MonthlyBudget,
				analysis.BudgetStatus.ProjectedSpending / analysis.BudgetStatus.MonthlyBudget * 100),
			Action:      "考虑增加本地推理比例或切换更经济的模型",
			Impact:      "避免预算超支",
		})
	}

	// 比较本地和云端成本效率
	if analysis.LocalInferenceCost > 0 && analysis.CloudAPICost > 0 {
		localCostPerRequest := analysis.LocalAnalysis.CostPerRequest
		cloudCostPerRequest := analysis.CloudAnalysis.CostPerRequest

		if cloudCostPerRequest < localCostPerRequest * 0.5 {
			recs = append(recs, CostRecommendation{
				ID:          generateCostRecID(),
				Type:        "ai_service",
				Priority:    "medium",
				Title:       "云端API性价比更高",
				Description: fmt.Sprintf("云端单次成本 %.4f 元，本地 %.4f 元", cloudCostPerRequest, localCostPerRequest),
				PotentialSavings: analysis.LocalInferenceCost * 0.3,
				Action:      "增加云端API使用比例",
				Impact:      "降低整体AI成本",
			})
		} else if localCostPerRequest < cloudCostPerRequest * 0.5 {
			recs = append(recs, CostRecommendation{
				ID:          generateCostRecID(),
				Type:        "ai_service",
				Priority:    "medium",
				Title:       "本地推理性价比更高",
				Description: fmt.Sprintf("本地单次成本 %.4f 元，云端 %.4f 元", localCostPerRequest, cloudCostPerRequest),
				PotentialSavings: analysis.CloudAPICost * 0.3,
				Action:      "增加本地推理使用比例",
				Impact:      "降低整体AI成本",
			})
		}
	}

	// 检查Token消耗是否可以优化
	if analysis.TokenConsumption.GrowthRate > 20 {
		recs = append(recs, CostRecommendation{
			ID:          generateCostRecID(),
			Type:        "ai_service",
			Priority:    "low",
			Title:       "Token消耗增长较快",
			Description: fmt.Sprintf("月增长率 %.1f%%，建议优化提示词长度", analysis.TokenConsumption.GrowthRate),
			Action:      "优化提示词，减少不必要的Token消耗",
			Impact:      "降低云端API成本",
		})
	}

	return recs
}

// ========== 勒索检测成本分析方法 (v2.308.0新增) ==========

// RansomwareDataProvider 勒索检测数据提供者接口.
type RansomwareDataProvider interface {
	// 获取特征库信息
	GetSignatureDBInfo(ctx context.Context) (*SignatureDBInfo, error)

	// 获取隔离区使用情况
	GetQuarantineUsage(ctx context.Context) (*QuarantineUsageData, error)

	// 获取事件日志统计
	GetEventLogStats(ctx context.Context, start, end time.Time) (*EventLogStats, error)

	// 获取快照统计
	GetSnapshotStats(ctx context.Context, start, end time.Time) (*SnapshotStats, error)
}

// SignatureDBInfo 特征库信息.
type SignatureDBInfo struct {
	SizeMB         float64   `json:"size_mb"`
	SignatureCount int64     `json:"signature_count"`
	LastUpdate     time.Time `json:"last_update"`
}

// QuarantineUsageData 隔离区使用数据.
type QuarantineUsageData struct {
	MaxCapacityGB    float64 `json:"max_capacity_gb"`
	CurrentUsageGB   float64 `json:"current_usage_gb"`
	FileCount        int64   `json:"file_count"`
	FilesCleaned     int64   `json:"files_cleaned"`
	SpaceReclaimedGB float64 `json:"space_reclaimed_gb"`
}

// EventLogStats 事件日志统计.
type EventLogStats struct {
	DailyEventCount   int64   `json:"daily_event_count"`
	MonthlyEventCount int64   `json:"monthly_event_count"`
	EventSizeKB       float64 `json:"event_size_kb"`
	TotalSizeGB       float64 `json:"total_size_gb"`
}

// SnapshotStats 快照统计.
type SnapshotStats struct {
	TriggerCount       int     `json:"trigger_count"`
	SnapshotSizeGB     float64 `json:"snapshot_size_gb"`
	TotalSnapshotGB    float64 `json:"total_snapshot_gb"`
	ActiveSnapshots    int     `json:"active_snapshots"`
}

// AnalyzeRansomwareCost 分析勒索检测成本.
func (a *CostAnalyzer) AnalyzeRansomwareCost(ctx context.Context, provider RansomwareDataProvider) (*RansomwareCostAnalysis, error) {
	if !a.config.RansomwareDetection.Enabled {
		return nil, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	start := now.AddDate(0, 0, -a.config.AnalysisPeriodDays)

	analysis := &RansomwareCostAnalysis{
		AnalysisTime:  now,
		PeriodStart:   start,
		PeriodEnd:     now,
		Currency:      a.config.DefaultCurrency,
		TrendData:     make([]RansomwareCostTrendPoint, 0),
		Recommendations: make([]CostRecommendation, 0),
	}

	// 获取各组件数据
	sigDBInfo, err := provider.GetSignatureDBInfo(ctx)
	if err == nil && sigDBInfo != nil {
		analysis.SignatureDBAnalysis = a.calculateSignatureDBCost(sigDBInfo)
		analysis.SignatureDBCost = analysis.SignatureDBAnalysis.FixedMonthlyCost
	}

	quarantineData, err := provider.GetQuarantineUsage(ctx)
	if err == nil && quarantineData != nil {
		analysis.QuarantineAnalysis = a.calculateQuarantineCost(quarantineData)
		analysis.QuarantineCost = analysis.QuarantineAnalysis.MonthlyCost
	}

	eventLogStats, err := provider.GetEventLogStats(ctx, start, now)
	if err == nil && eventLogStats != nil {
		analysis.EventLogAnalysis = a.calculateEventLogCost(eventLogStats)
		analysis.EventLogCost = analysis.EventLogAnalysis.MonthlyCost
	}

	snapshotStats, err := provider.GetSnapshotStats(ctx, start, now)
	if err == nil && snapshotStats != nil {
		analysis.SnapshotAnalysis = a.calculateSnapshotCost(snapshotStats)
		analysis.AutoSnapshotCost = analysis.SnapshotAnalysis.MonthlyCost
	}

	// 计算总成本
	analysis.TotalCost = analysis.SignatureDBCost + analysis.QuarantineCost + 
		analysis.EventLogCost + analysis.AutoSnapshotCost

	// 生成建议
	analysis.Recommendations = a.generateRansomwareRecommendations(analysis)

	return analysis, nil
}

// calculateSignatureDBCost 计算特征库成本.
func (a *CostAnalyzer) calculateSignatureDBCost(info *SignatureDBInfo) SignatureDBCostBreakdown {
	config := a.config.RansomwareDetection.SignatureDB

	return SignatureDBCostBreakdown{
		SizeMB:           info.SizeMB,
		UpdateFrequency:  config.UpdateFrequencyDays,
		LastUpdate:       info.LastUpdate,
		SignatureCount:   info.SignatureCount,
		FixedMonthlyCost: config.MonthlyCostEstimate,
		NetworkTransferCost: info.SizeMB * 0.001 / 1024 * 30 / float64(config.UpdateFrequencyDays), // 假设流量成本0.001元/GB
	}
}

// calculateQuarantineCost 计算隔离区成本.
func (a *CostAnalyzer) calculateQuarantineCost(data *QuarantineUsageData) QuarantineCostBreakdown {
	config := a.config.RansomwareDetection.Quarantine

	usagePercent := 0.0
	if data.MaxCapacityGB > 0 {
		usagePercent = data.CurrentUsageGB / data.MaxCapacityGB * 100
	}

	monthlyCost := data.CurrentUsageGB * config.PricePerGB

	// 优化后预估成本 (假设清理后剩余30%)
	optimizedCost := data.CurrentUsageGB * 0.3 * config.PricePerGB

	return QuarantineCostBreakdown{
		MaxCapacityGB:    data.MaxCapacityGB,
		CurrentUsageGB:   data.CurrentUsageGB,
		UsagePercent:     usagePercent,
		FileCount:        data.FileCount,
		AutoCleanupDays:  config.AutoCleanupDays,
		FilesCleaned:     data.FilesCleaned,
		SpaceReclaimedGB: data.SpaceReclaimedGB,
		PricePerGB:       config.PricePerGB,
		MonthlyCost:      monthlyCost,
		OptimizedCost:    optimizedCost,
		SavingsPotential: monthlyCost - optimizedCost,
	}
}

// calculateEventLogCost 计算事件日志成本.
func (a *CostAnalyzer) calculateEventLogCost(stats *EventLogStats) EventLogCostBreakdown {
	config := a.config.RansomwareDetection.EventLog

	compressedSizeGB := stats.TotalSizeGB * config.CompressionRatio

	return EventLogCostBreakdown{
		DailyEventCount:   stats.DailyEventCount,
		MonthlyEventCount: stats.MonthlyEventCount,
		EventSizeKB:       stats.EventSizeKB,
		TotalSizeGB:       stats.TotalSizeGB,
		RetentionDays:     config.RetentionDays,
		CompressionRatio:  config.CompressionRatio,
		CompressedSizeGB:  compressedSizeGB,
		PricePerGB:        config.PricePerGB,
		MonthlyCost:       stats.TotalSizeGB * config.PricePerGB,
		AfterCleanupCost:  compressedSizeGB * config.PricePerGB,
	}
}

// calculateSnapshotCost 计算快照成本.
func (a *CostAnalyzer) calculateSnapshotCost(stats *SnapshotStats) SnapshotCostBreakdown {
	config := a.config.RansomwareDetection.AutoSnapshot

	return SnapshotCostBreakdown{
		Enabled:            config.Enabled,
		SnapshotTriggerCount: stats.TriggerCount,
		SnapshotSizeGB:     stats.SnapshotSizeGB,
		TotalSnapshotGB:    stats.TotalSnapshotGB,
		RetentionDays:      config.RetentionDays,
		ActiveSnapshots:    stats.ActiveSnapshots,
		SnapshotPricePerGB: config.SnapshotPricePerGB,
		MonthlyCost:        stats.TotalSnapshotGB * config.SnapshotPricePerGB,
		OptimizedCost:      float64(stats.TriggerCount) * stats.SnapshotSizeGB * config.SnapshotPricePerGB * 0.5, // 假设优化后减少50%
	}
}

// generateRansomwareRecommendations 生成勒索检测成本建议.
func (a *CostAnalyzer) generateRansomwareRecommendations(analysis *RansomwareCostAnalysis) []CostRecommendation {
	recs := make([]CostRecommendation, 0)

	// 检查隔离区利用率
	if analysis.QuarantineAnalysis.UsagePercent > 80 {
		recs = append(recs, CostRecommendation{
			ID:          generateCostRecID(),
			Type:        "ransomware",
			Priority:    "high",
			Title:       "隔离区存储接近上限",
			Description: fmt.Sprintf("隔离区利用率 %.1f%%，建议清理过期文件", analysis.QuarantineAnalysis.UsagePercent),
			PotentialSavings: analysis.QuarantineAnalysis.SavingsPotential,
			CurrentCost:      analysis.QuarantineAnalysis.MonthlyCost,
			OptimizedCost:    analysis.QuarantineAnalysis.OptimizedCost,
			Action:      "执行隔离区自动清理策略",
			Impact:      "释放存储空间，降低成本",
		})
	}

	// 检查快照成本
	if analysis.SnapshotAnalysis.Enabled && analysis.SnapshotAnalysis.MonthlyCost > 100 {
		recs = append(recs, CostRecommendation{
			ID:          generateCostRecID(),
			Type:        "ransomware",
			Priority:    "medium",
			Title:       "快照成本较高",
			Description: fmt.Sprintf("月快照成本 %.2f 元，触发次数 %d", 
				analysis.SnapshotAnalysis.MonthlyCost, analysis.SnapshotAnalysis.SnapshotTriggerCount),
			PotentialSavings: analysis.SnapshotAnalysis.MonthlyCost - analysis.SnapshotAnalysis.OptimizedCost,
			CurrentCost:      analysis.SnapshotAnalysis.MonthlyCost,
			OptimizedCost:    analysis.SnapshotAnalysis.OptimizedCost,
			Action:      "优化快照触发策略，仅在真正威胁时创建",
			Impact:      "降低快照存储成本",
		})
	}

	// 检查事件日志是否过大
	if analysis.EventLogAnalysis.TotalSizeGB > 10 {
		recs = append(recs, CostRecommendation{
			ID:          generateCostRecID(),
			Type:        "ransomware",
			Priority:    "low",
			Title:       "事件日志存储较大",
			Description: fmt.Sprintf("事件日志 %.1f GB，建议启用压缩归档", analysis.EventLogAnalysis.TotalSizeGB),
			PotentialSavings: analysis.EventLogAnalysis.MonthlyCost - analysis.EventLogAnalysis.AfterCleanupCost,
			CurrentCost:      analysis.EventLogAnalysis.MonthlyCost,
			OptimizedCost:    analysis.EventLogAnalysis.AfterCleanupCost,
			Action:      "启用日志压缩和定期清理策略",
			Impact:      "降低日志存储成本",
		})
	}

	return recs
}

// EstimateRansomwareStorageCost 估算勒索检测存储成本.
func (a *CostAnalyzer) EstimateRansomwareStorageCost(quarantineGB, eventLogGB, snapshotGB float64) float64 {
	config := a.config.RansomwareDetection

	cost := config.SignatureDB.MonthlyCostEstimate // 固定成本
	cost += quarantineGB * config.Quarantine.PricePerGB
	cost += eventLogGB * config.EventLog.PricePerGB * config.EventLog.CompressionRatio
	cost += snapshotGB * config.AutoSnapshot.SnapshotPricePerGB

	return cost
}
