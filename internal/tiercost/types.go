// Package tiercost 提供存储分层成本分析功能
// 学习群晖Tiering的成本优化策略，分析各存储层的使用率和成本
package tiercost

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrTierNotFound 存储层不存在.
	ErrTierNotFound = errors.New("存储层不存在")
	// ErrInvalidPricing 无效的存储单价.
	ErrInvalidPricing = errors.New("无效的存储单价")
	// ErrInvalidInput 无效输入参数.
	ErrInvalidInput = errors.New("无效输入参数")
	// ErrInsufficientData 历史数据不足.
	ErrInsufficientData = errors.New("历史数据不足")
)

// ========== 存储层类型 ==========

// TierType 存储层类型.
type TierType string

const (
	// TierNVMe NVMe固态硬盘层.
	TierNVMe TierType = "nvme"
	// TierSSD SATA固态硬盘层.
	TierSSD TierType = "ssd"
	// TierHDD 机械硬盘层.
	TierHDD TierType = "hdd"
)

// ========== 核心数据结构 ==========

// TierInfo 存储层信息.
type TierInfo struct {
	// Name 存储层名称（NVMe/SSD/HDD）.
	Name TierType `json:"name"`
	// Capacity 总容量（字节）.
	Capacity int64 `json:"capacity"`
	// Used 已使用容量（字节）.
	Used int64 `json:"used"`
	// UnitPrice 每TB每年单价（元）.
	UnitPrice float64 `json:"unit_price"`
	// Utilization 使用率（0-1）.
	Utilization float64 `json:"utilization"`
	// DisplayName 显示名称.
	DisplayName string `json:"display_name"`
}

// DatasetInfo 数据集信息.
type DatasetInfo struct {
	// Name 数据集名称.
	Name string `json:"name"`
	// Size 数据集大小（字节）.
	Size int64 `json:"size"`
	// CurrentTier 当前所在存储层.
	CurrentTier TierType `json:"current_tier"`
	// AccessFrequency 访问频率: hot/warm/cold.
	AccessFrequency string `json:"access_frequency"`
	// LastAccessTime 最后访问时间.
	LastAccessTime time.Time `json:"last_access_time"`
}

// CostReport 成本分析报告.
type CostReport struct {
	// TotalCost 总年度成本（元）.
	TotalCost float64 `json:"total_cost"`
	// TierBreakdown 各层成本明细.
	TierBreakdown []TierCostDetail `json:"tier_breakdown"`
	// SavingsPotential 潜在节省金额（元）.
	SavingsPotential float64 `json:"savings_potential"`
	// Recommendations 分层建议列表.
	Recommendations []TierRecommendation `json:"recommendations"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generated_at"`
}

// TierCostDetail 单层成本明细.
type TierCostDetail struct {
	TierName       TierType `json:"tier_name"`
	DisplayName    string   `json:"display_name"`
	CapacityTB     float64  `json:"capacity_tb"`
	UsedTB         float64  `json:"used_tb"`
	Utilization    float64  `json:"utilization"`
	UnitPrice      float64  `json:"unit_price"`
	AnnualCost     float64  `json:"annual_cost"`
	CostPercentage float64  `json:"cost_percentage"`
}

// TierRecommendation 分层建议.
type TierRecommendation struct {
	// DatasetName 数据集名称.
	DatasetName string `json:"dataset_name"`
	// CurrentTier 当前存储层.
	CurrentTier TierType `json:"current_tier"`
	// RecommendedTier 推荐存储层.
	RecommendedTier TierType `json:"recommended_tier"`
	// EstSavings 预估节省金额（元/年）.
	EstSavings float64 `json:"est_savings"`
	// Reason 推荐原因.
	Reason string `json:"reason"`
}

// CostTrend 成本趋势数据点.
type CostTrend struct {
	// Date 日期.
	Date time.Time `json:"date"`
	// Cost 当月成本（元）.
	Cost float64 `json:"cost"`
	// ProjectedCost 预测成本（元）.
	ProjectedCost float64 `json:"projected_cost"`
	// IsProjected 是否为预测值.
	IsProjected bool `json:"is_projected"`
}

// SimulateRequest 模拟分层方案请求.
type SimulateRequest struct {
	// Datasets 数据集列表.
	Datasets []DatasetInfo `json:"datasets"`
	// TierAssignments 自定义分层方案: 数据集名 -> 目标存储层.
	TierAssignments map[string]TierType `json:"tier_assignments"`
}

// SimulateResponse 模拟分层方案响应.
type SimulateResponse struct {
	// CurrentCost 当前方案成本（元/年）.
	CurrentCost float64 `json:"current_cost"`
	// SimulatedCost 模拟方案成本（元/年）.
	SimulatedCost float64 `json:"simulated_cost"`
	// Savings 节省金额（元/年）.
	Savings float64 `json:"savings"`
	// SavingsPercent 节省百分比.
	SavingsPercent float64 `json:"savings_percent"`
	// Details 各层成本对比.
	Details []SimulateTierDetail `json:"details"`
}

// SimulateTierDetail 模拟方案各层详情.
type SimulateTierDetail struct {
	TierName        TierType `json:"tier_name"`
	DisplayName     string   `json:"display_name"`
	CurrentUsedTB   float64  `json:"current_used_tb"`
	SimulatedUsedTB float64  `json:"simulated_used_tb"`
	UnitPrice       float64  `json:"unit_price"`
	CurrentCost     float64  `json:"current_cost"`
	SimulatedCost   float64  `json:"simulated_cost"`
}

// PricingUpdateRequest 更新存储单价请求.
type PricingUpdateRequest struct {
	NVMePricePerTBYear *float64 `json:"nvme_price_per_tb_year,omitempty"`
	SSDPricePerTBYear  *float64 `json:"ssd_price_per_tb_year,omitempty"`
	HDDPricePerTBYear  *float64 `json:"hdd_price_per_tb_year,omitempty"`
}

// DefaultPricing 默认存储单价（元/TB/年）.
type DefaultPricing struct {
	NVMePricePerTBYear float64
	SSDPricePerTBYear  float64
	HDDPricePerTBYear  float64
}

// DefaultPricingConfig 返回默认存储单价配置.
func DefaultPricingConfig() DefaultPricing {
	return DefaultPricing{
		NVMePricePerTBYear: 800, // NVMe: ¥800/TB/年
		SSDPricePerTBYear:  500, // SSD: ¥500/TB/年
		HDDPricePerTBYear:  120, // HDD: ¥120/TB/年
	}
}
