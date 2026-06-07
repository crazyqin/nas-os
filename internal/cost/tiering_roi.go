// Package cost - Tiering ROI分析模块
// 存储分层成本效益分析增强
package cost

import (
	"fmt"
	"math"
	"time"
)

// TieringROIAnalyzer 分层存储ROI分析器
type TieringROIAnalyzer struct {
	ssdConfig   TierConfig
	hddConfig   TierConfig
	cloudConfig TierConfig
}

// TierConfig 存储层配置
type TierConfig struct {
	MediaType         string  `json:"media_type"`           // SSD/HDD/Cloud
	CapacityGB        float64 `json:"capacity_gb"`          // 容量(GB)
	CostPerGB         float64 `json:"cost_per_gb"`          // 每GB成本($)
	ReadIOPS          float64 `json:"read_iops"`            // 读IOPS
	WriteIOPS         float64 `json:"write_iops"`           // 写IOPS
	LatencyMs         float64 `json:"latency_ms"`           // 延迟(ms)
	LifespanYears     float64 `json:"lifespan_years"`       // 寿命(年)
	EnergyCostPerYear float64 `json:"energy_cost_per_year"` // 年能耗成本($)
}

// TieringROIResult ROI分析结果
type TieringROIResult struct {
	TotalCost           float64   `json:"total_cost"`              // 总成本
	TotalCostOver5Years float64   `json:"total_cost_over_5_years"` // 5年总成本
	PerformanceGain     float64   `json:"performance_gain"`        // 性能提升(%)
	CostSavings         float64   `json:"cost_savings"`            // 成本节省(%)
	ROIPercent          float64   `json:"roi_percent"`             // ROI百分比
	BreakEvenMonths     int       `json:"break_even_months"`       // 收益平衡点(月)
	Recommendation      string    `json:"recommendation"`          // 建议
	Timestamp           time.Time `json:"timestamp"`
}

// NewTieringROIAnalyzer 创建ROI分析器
func NewTieringROIAnalyzer(ssd, hdd, cloud TierConfig) *TieringROIAnalyzer {
	return &TieringROIAnalyzer{
		ssdConfig:   ssd,
		hddConfig:   hdd,
		cloudConfig: cloud,
	}
}

// AnalyzeROI 分析分层存储ROI
func (t *TieringROIAnalyzer) AnalyzeROI(workload WorkloadProfile) *TieringROIResult {
	// 计算各层成本
	ssdTotalCost := t.ssdConfig.CapacityGB * t.ssdConfig.CostPerGB
	hddTotalCost := t.hddConfig.CapacityGB * t.hddConfig.CostPerGB

	// 5年能耗成本
	ssdEnergy5Y := t.ssdConfig.EnergyCostPerYear * 5
	hddEnergy5Y := t.hddConfig.EnergyCostPerYear * 5

	// 纯HDD方案成本
	pureHDDCost := hddTotalCost * (workload.HotDataPercent + workload.ColdDataPercent) / workload.ColdDataPercent
	pureHDD5Y := pureHDDCost + hddEnergy5Y*(workload.TotalDataGB/t.hddConfig.CapacityGB)

	// 分层方案成本
	tieringCost := ssdTotalCost + hddTotalCost
	tiering5Y := tieringCost + ssdEnergy5Y + hddEnergy5Y

	// 性能提升计算（基于热数据访问比例）
	perfGain := workload.HotDataPercent * (t.ssdConfig.ReadIOPS/t.hddConfig.ReadIOPS - 1) * 100

	// 成本节省
	costSavings := (pureHDD5Y - tiering5Y) / pureHDD5Y * 100

	// ROI计算
	roi := ((pureHDD5Y - tiering5Y) / tiering5Y) * 100

	// 收益平衡点
	breakEven := int(math.Ceil(tieringCost / ((pureHDD5Y - tiering5Y) / 60)))

	// 生成建议
	recommendation := t.generateRecommendation(costSavings, perfGain, breakEven)

	return &TieringROIResult{
		TotalCost:           tieringCost,
		TotalCostOver5Years: tiering5Y,
		PerformanceGain:     perfGain,
		CostSavings:         costSavings,
		ROIPercent:          roi,
		BreakEvenMonths:     breakEven,
		Recommendation:      recommendation,
		Timestamp:           time.Now(),
	}
}

// WorkloadProfile 工作负载特征
type WorkloadProfile struct {
	TotalDataGB       float64 `json:"total_data_gb"`        // 总数据量(GB)
	HotDataPercent    float64 `json:"hot_data_percent"`     // 热数据占比(%)
	ColdDataPercent   float64 `json:"cold_data_percent"`    // 冷数据占比(%)
	ReadIntensity     float64 `json:"read_intensity"`       // 读强度(ops/s)
	WriteIntensity    float64 `json:"write_intensity"`      // 写强度(ops/s)
	GrowthRatePerYear float64 `json:"growth_rate_per_year"` // 年增长率(%)
}

// generateRecommendation 生成ROI建议
func (t *TieringROIAnalyzer) generateRecommendation(costSavings, perfGain float64, breakEven int) string {
	if costSavings > 20 && perfGain > 50 && breakEven < 24 {
		return fmt.Sprintf("强烈推荐: 成本节省%.1f%%，性能提升%.1f%%，%d个月收回投资",
			costSavings, perfGain, breakEven)
	}
	if costSavings > 10 && perfGain > 30 {
		return fmt.Sprintf("推荐: 成本节省%.1f%%，性能提升%.1f%%，投资回报周期合理",
			costSavings, perfGain)
	}
	if perfGain > 50 {
		return fmt.Sprintf("可选: 性能提升显著(%.1f%%)，但成本节省有限，适合性能优先场景", perfGain)
	}
	return "不推荐: ROI不足，建议维持现有架构"
}

// CompareWithCloud 对比云存储方案
func (t *TieringROIAnalyzer) CompareWithCloud(workload WorkloadProfile) *CloudComparisonResult {
	// 本地分层方案5年成本
	localCost := t.AnalyzeROI(workload).TotalCostOver5Years

	// 云存储方案成本计算
	cloudStorageCost := workload.TotalDataGB * t.cloudConfig.CostPerGB * 60 // 5年=60个月
	cloudTransferCost := workload.ReadIntensity * 0.01 * 60                 // 数据传输成本估算

	return &CloudComparisonResult{
		LocalCost5Y:    localCost,
		CloudCost5Y:    cloudStorageCost + cloudTransferCost,
		LocalLatencyMs: t.ssdConfig.LatencyMs,
		CloudLatencyMs: t.cloudConfig.LatencyMs,
		Recommendation: t.compareLocalVsCloud(localCost, cloudStorageCost+cloudTransferCost),
	}
}

// CloudComparisonResult 云存储对比结果
type CloudComparisonResult struct {
	LocalCost5Y    float64 `json:"local_cost_5y"`
	CloudCost5Y    float64 `json:"cloud_cost_5y"`
	LocalLatencyMs float64 `json:"local_latency_ms"`
	CloudLatencyMs float64 `json:"cloud_latency_ms"`
	Recommendation string  `json:"recommendation"`
}

// compareLocalVsCloud 本地vs云端对比建议
func (t *TieringROIAnalyzer) compareLocalVsCloud(local, cloud float64) string {
	diff := (cloud - local) / local * 100
	if diff > 30 {
		return fmt.Sprintf("本地存储更优: 云端成本高出%.1f%%", diff)
	}
	if diff < -30 {
		return fmt.Sprintf("云存储更优: 成本节省%.1f%%，适合低频访问数据", -diff)
	}
	return "成本相近: 混合方案最佳，热数据本地+冷数据云端"
}
