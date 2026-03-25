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

	// 分析周期
	AnalysisPeriodDays int `json:"analysis_period_days"`

	// 访问频率分层配置
	AccessFrequencyTiers []AccessFrequencyTier `json:"access_frequency_tiers"`

	// 货币单位
	DefaultCurrency string `json:"default_currency"`
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
	Type             string  `json:"type"`     // storage, bandwidth, access_pattern
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
