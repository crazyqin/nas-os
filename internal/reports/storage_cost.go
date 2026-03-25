// Package reports 提供报表生成和管理功能
// v2.45.0 存储成本优化报告增强版
package reports

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== 基础类型定义 ==========

// StorageCostConfig 存储成本配置.
type StorageCostConfig struct {
	CostPerGB               float64 `json:"cost_per_gb"`
	CostPerGBMonthly        float64 `json:"cost_per_gb_monthly"`
	CostPerIOPSMonthly      float64 `json:"cost_per_iops_monthly"`
	CostPerBandwidthMonthly float64 `json:"cost_per_bandwidth_monthly"`
	ElectricityCostPerKWh   float64 `json:"electricity_cost_per_kwh"`
	DevicePowerWatts        float64 `json:"device_power_watts"`
	OpsCostMonthly          float64 `json:"ops_cost_monthly"`
	DepreciationYears       int     `json:"depreciation_years"`
	HardwareCost            float64 `json:"hardware_cost"`
	Currency                string  `json:"currency"`
	BudgetLimit             float64 `json:"budget_limit"`
	AlertThreshold          float64 `json:"alert_threshold"`
	EnableOptimization      bool    `json:"enable_optimization"`
}

// StorageCostCalculator 存储成本计算器.
type StorageCostCalculator struct {
	config StorageCostConfig
}

// CostCalculationResult 成本计算结果.
type CostCalculationResult struct {
	UsagePercent        float64 `json:"usage_percent"`
	CapacityCostMonthly float64 `json:"capacity_cost_monthly"`
	TotalCostMonthly    float64 `json:"total_cost_monthly"`
	CostPerGBMonthly    float64 `json:"cost_per_gb_monthly"`
	ProjectedAnnualCost float64 `json:"projected_annual_cost"`
	PotentialSavings    float64 `json:"potential_savings"`
}

// NewStorageCostCalculator 创建存储成本计算器.
func NewStorageCostCalculator(config StorageCostConfig) *StorageCostCalculator {
	return &StorageCostCalculator{config: config}
}

// Calculate 计算存储成本（v2.86.0 增强：完整成本模型）.
func (c *StorageCostCalculator) Calculate(metrics StorageMetrics) CostCalculationResult {
	totalGB := float64(metrics.TotalCapacityBytes) / (1024 * 1024 * 1024)
	usedGB := float64(metrics.UsedCapacityBytes) / (1024 * 1024 * 1024)

	usagePercent := 0.0
	if totalGB > 0 {
		usagePercent = (usedGB / totalGB) * 100
	}

	// 1. 容量成本：按实际使用量计费
	capacityCostMonthly := usedGB * c.config.CostPerGBMonthly

	// 2. IOPS 成本：按 IOPS 使用量计费
	iopsCostMonthly := 0.0
	if c.config.CostPerIOPSMonthly > 0 {
		totalIOPS := metrics.IOPS
		if totalIOPS == 0 {
			totalIOPS = metrics.IOPSRead + metrics.IOPSWrite
		}
		// 每1000 IOPS 计费
		iopsCostMonthly = float64(totalIOPS) / 1000.0 * c.config.CostPerIOPSMonthly
	}

	// 3. 带宽成本：按带宽使用量计费
	bandwidthCostMonthly := 0.0
	if c.config.CostPerBandwidthMonthly > 0 {
		totalBandwidthBytes := metrics.ReadBandwidthBytes + metrics.WriteBandwidthBytes
		if totalBandwidthBytes == 0 {
			totalBandwidthBytes = metrics.ThroughputReadBytes + metrics.ThroughputWriteBytes
		}
		// 转换为 Mbps（字节/秒 -> Mbps）
		bandwidthMbps := float64(totalBandwidthBytes) * 8.0 / (1024 * 1024)
		bandwidthCostMonthly = bandwidthMbps * c.config.CostPerBandwidthMonthly
	}

	// 4. 电力成本：按设备功耗计算
	electricityCostMonthly := 0.0
	if c.config.ElectricityCostPerKWh > 0 && c.config.DevicePowerWatts > 0 {
		// 每月电力成本 = 功率(kW) * 24小时 * 30天 * 电费单价
		powerKW := c.config.DevicePowerWatts / 1000.0
		electricityCostMonthly = powerKW * 24.0 * 30.0 * c.config.ElectricityCostPerKWh
	}

	// 5. 运维成本：分摊到每个卷
	opsCostMonthly := c.config.OpsCostMonthly
	if opsCostMonthly == 0 {
		opsCostMonthly = 0 // 默认无运维成本
	}

	// 6. 折旧成本：按硬件成本和使用年限计算
	depreciationCostMonthly := 0.0
	if c.config.HardwareCost > 0 && c.config.DepreciationYears > 0 {
		// 月折旧 = 总硬件成本 / 折旧年数 / 12个月
		depreciationCostMonthly = c.config.HardwareCost / float64(c.config.DepreciationYears) / 12.0
	}

	// 总月成本
	totalCostMonthly := capacityCostMonthly + iopsCostMonthly + bandwidthCostMonthly +
		electricityCostMonthly + opsCostMonthly + depreciationCostMonthly

	// 计算单位成本（按实际使用量）
	costPerGBMonthly := 0.0
	if usedGB > 0 {
		costPerGBMonthly = totalCostMonthly / usedGB
	}

	// 计算潜在节省（低使用率场景）
	potentialSavings := 0.0
	if usagePercent < 50 && totalGB > 0 {
		// 使用率低于50%，存在资源浪费
		wastedGB := totalGB*0.5 - usedGB
		if wastedGB > 0 {
			potentialSavings = wastedGB * c.config.CostPerGBMonthly
		}
	}

	return CostCalculationResult{
		UsagePercent:        round(usagePercent, 2),
		CapacityCostMonthly: round(capacityCostMonthly, 2),
		TotalCostMonthly:    round(totalCostMonthly, 2),
		CostPerGBMonthly:    round(costPerGBMonthly, 4),
		ProjectedAnnualCost: round(totalCostMonthly*12, 2),
		PotentialSavings:    round(potentialSavings, 2),
	}
}

// GetConfig 获取配置.
func (c *StorageCostCalculator) GetConfig() StorageCostConfig {
	return c.config
}

// UpdateConfig 更新配置.
func (c *StorageCostCalculator) UpdateConfig(config StorageCostConfig) {
	c.config = config
}

// CalculateAll 计算所有存储成本.
func (c *StorageCostCalculator) CalculateAll(metrics []StorageMetrics) []CostCalculationResult {
	results := make([]CostCalculationResult, len(metrics))
	for i, m := range metrics {
		results[i] = c.Calculate(m)
	}
	return results
}

// CostTrendPoint 成本趋势点.
type CostTrendPoint struct {
	Date   time.Time `json:"date"`
	Cost   float64   `json:"cost"`
	UsedGB float64   `json:"used_gb"`
	Trend  string    `json:"trend"` // up, down, stable
}

// GenerateReport 生成成本报告（v2.86.0 增强：完整成本分析）.
func (c *StorageCostCalculator) GenerateReport(metrics []StorageMetrics, period ReportPeriod) *StorageCostReport {
	now := time.Now()
	report := &StorageCostReport{
		ID:          "cost_report_" + now.Format("20060102150405"),
		Name:        "存储成本报告",
		GeneratedAt: now,
		Period:      period,
		VolumeCosts: make([]StorageCostResult, len(metrics)),
	}

	var totalCost, totalCapacity, totalUsed float64
	var totalIOPSCost, totalBandwidthCost, totalElecCost, totalOpsCost, totalDeprecCost float64

	for i, m := range metrics {
		result := c.Calculate(m)
		totalGB := float64(m.TotalCapacityBytes) / (1024 * 1024 * 1024)
		usedGB := float64(m.UsedCapacityBytes) / (1024 * 1024 * 1024)

		// 计算各项成本细分
		capacityCost := usedGB * c.config.CostPerGBMonthly

		// IOPS 成本
		iopsCost := 0.0
		if c.config.CostPerIOPSMonthly > 0 {
			totalIOPS := m.IOPS
			if totalIOPS == 0 {
				totalIOPS = m.IOPSRead + m.IOPSWrite
			}
			iopsCost = float64(totalIOPS) / 1000.0 * c.config.CostPerIOPSMonthly
		}

		// 带宽成本
		bandwidthCost := 0.0
		if c.config.CostPerBandwidthMonthly > 0 {
			totalBandwidth := m.ReadBandwidthBytes + m.WriteBandwidthBytes
			if totalBandwidth == 0 {
				totalBandwidth = m.ThroughputReadBytes + m.ThroughputWriteBytes
			}
			bandwidthMbps := float64(totalBandwidth) * 8.0 / (1024 * 1024)
			bandwidthCost = bandwidthMbps * c.config.CostPerBandwidthMonthly
		}

		// 电力成本（按卷容量比例分摊）
		elecCost := 0.0
		if c.config.ElectricityCostPerKWh > 0 && c.config.DevicePowerWatts > 0 {
			powerKW := c.config.DevicePowerWatts / 1000.0
			totalElec := powerKW * 24.0 * 30.0 * c.config.ElectricityCostPerKWh
			// 按容量比例分摊
			if totalCapacity > 0 {
				elecCost = totalElec * (totalGB / totalCapacity)
			} else {
				elecCost = totalElec / float64(len(metrics))
			}
		}

		// 运维成本（平均分摊）
		opsCost := c.config.OpsCostMonthly / float64(len(metrics))

		// 折旧成本（按容量比例分摊）
		deprecCost := 0.0
		if c.config.HardwareCost > 0 && c.config.DepreciationYears > 0 {
			totalDeprec := c.config.HardwareCost / float64(c.config.DepreciationYears) / 12.0
			if totalCapacity > 0 {
				deprecCost = totalDeprec * (totalGB / totalCapacity)
			} else {
				deprecCost = totalDeprec / float64(len(metrics))
			}
		}

		report.VolumeCosts[i] = StorageCostResult{
			VolumeName:              m.VolumeName,
			TotalGB:                 round(totalGB, 2),
			UsedGB:                  round(usedGB, 2),
			UsagePercent:            result.UsagePercent,
			CostPerGB:               round(c.config.CostPerGBMonthly, 4),
			CostPerGBMonthly:        result.CostPerGBMonthly,
			MonthlyCost:             result.TotalCostMonthly,
			TotalCostMonthly:        result.TotalCostMonthly,
			CapacityCostMonthly:     round(capacityCost, 2),
			IOPSCostMonthly:         round(iopsCost, 2),
			BandwidthCostMonthly:    round(bandwidthCost, 2),
			ElectricityCostMonthly:  round(elecCost, 2),
			OpsCostMonthly:          round(opsCost, 2),
			DepreciationCostMonthly: round(deprecCost, 2),
			AnnualCost:              round(result.TotalCostMonthly*12, 2),
			CalculatedAt:            now,
		}

		totalCost += result.TotalCostMonthly
		totalCapacity += totalGB
		totalUsed += usedGB
		totalIOPSCost += iopsCost
		totalBandwidthCost += bandwidthCost
		totalElecCost += elecCost
		totalOpsCost += opsCost
		totalDeprecCost += deprecCost
	}
	report.TotalCost = round(totalCost, 2)

	// 计算汇总
	report.Summary = StorageCostReportSummary{
		TotalCostMonthly: round(totalCost, 2),
		TotalCapacityGB:  round(totalCapacity, 2),
		TotalUsedGB:      round(totalUsed, 2),
		AvgUsagePercent:  0,
		PotentialSavings: 0,
		HealthScore:      0,
		VolumeCount:      len(metrics),
	}

	if totalCapacity > 0 {
		report.Summary.AvgUsagePercent = round(totalUsed/totalCapacity*100, 2)
	}

	// 计算健康评分
	report.Summary.HealthScore = c.calculateHealthScore(report)

	// 计算潜在节省
	report.Summary.PotentialSavings = round(totalCost*0.15, 2) // 估计15%的优化空间

	// 生成建议
	report.Recommendations = c.generateRecommendations(report)

	return report
}

// calculateHealthScore 计算健康评分.
func (c *StorageCostCalculator) calculateHealthScore(report *StorageCostReport) int {
	score := 100.0

	// 使用率评分
	avgUsage := report.Summary.AvgUsagePercent
	if avgUsage < 30 {
		score -= (30 - avgUsage) // 低利用率扣分
	} else if avgUsage > 85 {
		score -= (avgUsage - 85) // 高利用率扣分
	}

	// 成本效率评分（每GB成本）
	if report.Summary.TotalUsedGB > 0 {
		avgCostPerGB := report.TotalCost / report.Summary.TotalUsedGB
		if avgCostPerGB > c.config.CostPerGBMonthly*1.5 {
			score -= 10 // 成本过高扣分
		}
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return int(score)
}

// generateRecommendations 生成建议.
func (c *StorageCostCalculator) generateRecommendations(report *StorageCostReport) []string {
	recs := make([]string, 0)

	// 基于使用率
	if report.Summary.AvgUsagePercent > 80 {
		recs = append(recs, "存储使用率较高，建议规划扩容或数据清理")
	} else if report.Summary.AvgUsagePercent < 30 {
		recs = append(recs, "存储使用率较低，存在资源浪费，建议收缩容量或重新分配")
	}

	// 基于健康评分
	if report.Summary.HealthScore < 60 {
		recs = append(recs, "存储健康状态不佳，建议进行全面检查")
	}

	// 基于潜在节省
	if report.Summary.PotentialSavings > 0 {
		recs = append(recs, fmt.Sprintf("存在 %.2f 元/月的潜在节省空间", report.Summary.PotentialSavings))
	}

	return recs
}

// AnalyzeTrend 分析成本趋势.
func (c *StorageCostCalculator) AnalyzeTrend(trendData []CostTrendPoint) []CostTrendPoint {
	return trendData
}

// round 辅助函数 (定义在 bandwidth_report.go 中)

// StorageMetrics 存储指标（用于报告）.
type StorageMetrics struct {
	VolumeName             string    `json:"volume_name"`
	TotalCapacityBytes     uint64    `json:"total_capacity_bytes"`
	UsedCapacityBytes      uint64    `json:"used_capacity_bytes"`
	AvailableCapacityBytes uint64    `json:"available_capacity_bytes"`
	UsagePercent           float64   `json:"usage_percent"`
	IOPS                   uint64    `json:"iops"`
	IOPSRead               uint64    `json:"iops_read"`
	IOPSWrite              uint64    `json:"iops_write"`
	LatencyReadMs          float64   `json:"latency_read_ms"`
	LatencyWriteMs         float64   `json:"latency_write_ms"`
	ThroughputReadBytes    uint64    `json:"throughput_read_bytes"`
	ThroughputWriteBytes   uint64    `json:"throughput_write_bytes"`
	ReadBandwidthBytes     uint64    `json:"read_bandwidth_bytes"`
	WriteBandwidthBytes    uint64    `json:"write_bandwidth_bytes"`
	FileCount              uint64    `json:"file_count"`
	DirCount               uint64    `json:"dir_count"`
	Timestamp              time.Time `json:"timestamp"`
}

// StorageCostResult 存储成本结果.
type StorageCostResult struct {
	VolumeName              string    `json:"volume_name"`
	TotalGB                 float64   `json:"total_gb"`
	UsedGB                  float64   `json:"used_gb"`
	UsagePercent            float64   `json:"usage_percent"`
	CostPerGB               float64   `json:"cost_per_gb"`
	CostPerGBMonthly        float64   `json:"cost_per_gb_monthly"`
	MonthlyCost             float64   `json:"monthly_cost"`
	TotalCostMonthly        float64   `json:"total_cost_monthly"`
	CapacityCostMonthly     float64   `json:"capacity_cost_monthly"`
	IOPSCostMonthly         float64   `json:"iops_cost_monthly"`
	BandwidthCostMonthly    float64   `json:"bandwidth_cost_monthly"`
	ElectricityCostMonthly  float64   `json:"electricity_cost_monthly"`
	OpsCostMonthly          float64   `json:"ops_cost_monthly"`
	DepreciationCostMonthly float64   `json:"depreciation_cost_monthly"`
	AnnualCost              float64   `json:"annual_cost"`
	CalculatedAt            time.Time `json:"calculated_at"`
}

// StorageCostReportSummary 存储成本报告摘要.
type StorageCostReportSummary struct {
	TotalCostMonthly float64 `json:"total_cost_monthly"`
	TotalCapacityGB  float64 `json:"total_capacity_gb"`
	TotalUsedGB      float64 `json:"total_used_gb"`
	AvgUsagePercent  float64 `json:"avg_usage_percent"`
	PotentialSavings float64 `json:"potential_savings"`
	HealthScore      int     `json:"health_score"`
	VolumeCount      int     `json:"volume_count"`
}

// StorageCostReport 存储成本报告.
type StorageCostReport struct {
	ID              string                   `json:"id"`
	Name            string                   `json:"name"`
	GeneratedAt     time.Time                `json:"generated_at"`
	Period          ReportPeriod             `json:"period"`
	TotalCost       float64                  `json:"total_cost"`
	VolumeCosts     []StorageCostResult      `json:"volume_costs"`
	Summary         StorageCostReportSummary `json:"summary"`
	Recommendations []string                 `json:"recommendations"`
}

// ========== 存储空间利用率分析 v2.45.0 ==========

// StorageUtilizationAnalyzer 存储空间利用率分析器.
type StorageUtilizationAnalyzer struct {
	config StorageCostConfig
}

// NewStorageUtilizationAnalyzer 创建存储利用率分析器.
func NewStorageUtilizationAnalyzer(config StorageCostConfig) *StorageUtilizationAnalyzer {
	return &StorageUtilizationAnalyzer{config: config}
}

// UtilizationAnalysis 利用率分析结果.
type UtilizationAnalysis struct {
	// 分析ID
	ID string `json:"id"`

	// 分析时间
	AnalyzedAt time.Time `json:"analyzed_at"`

	// 分析周期
	Period ReportPeriod `json:"period"`

	// 卷利用率详情
	VolumeUtilizations []VolumeUtilization `json:"volume_utilizations"`

	// 汇总统计
	Summary UtilizationSummary `json:"summary"`

	// 告警列表
	Alerts []UtilizationAlert `json:"alerts,omitempty"`

	// 优化建议
	Recommendations []UtilizationRecommendation `json:"recommendations"`
}

// VolumeUtilization 卷利用率详情.
type VolumeUtilization struct {
	// 卷名
	VolumeName string `json:"volume_name"`

	// 总容量（字节）
	TotalCapacityBytes uint64 `json:"total_capacity_bytes"`

	// 已使用（字节）
	UsedBytes uint64 `json:"used_bytes"`

	// 可用（字节）
	AvailableBytes uint64 `json:"available_bytes"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 利用率评级（excellent/good/warning/critical）
	Rating string `json:"rating"`

	// IOPS利用率
	IOPSUtilization float64 `json:"iops_utilization"`

	// 带宽利用率
	BandwidthUtilization float64 `json:"bandwidth_utilization"`

	// 文件数量
	FileCount uint64 `json:"file_count"`

	// 平均文件大小（字节）
	AvgFileSize float64 `json:"avg_file_size"`

	// 小文件占比（<1KB）
	SmallFilePercent float64 `json:"small_file_percent"`

	// 大文件占比（>100MB）
	LargeFilePercent float64 `json:"large_file_percent"`

	// 预估天数到满（基于当前增长）
	DaysToFull int `json:"days_to_full"`

	// 成本效率（元/有效GB）
	CostEfficiency float64 `json:"cost_efficiency"`

	// 采集时间
	Timestamp time.Time `json:"timestamp"`
}

// UtilizationSummary 利用率汇总.
type UtilizationSummary struct {
	// 总容量（GB）
	TotalCapacityGB float64 `json:"total_capacity_gb"`

	// 总使用量（GB）
	TotalUsedGB float64 `json:"total_used_gb"`

	// 总可用量（GB）
	TotalAvailableGB float64 `json:"total_available_gb"`

	// 平均使用率
	AvgUsagePercent float64 `json:"avg_usage_percent"`

	// 最高使用率
	MaxUsagePercent float64 `json:"max_usage_percent"`

	// 最低使用率
	MinUsagePercent float64 `json:"min_usage_percent"`

	// 使用率分布
	UsageDistribution UsageDistribution `json:"usage_distribution"`

	// 低利用率卷数（<30%）
	LowUtilizationCount int `json:"low_utilization_count"`

	// 高利用率卷数（>80%）
	HighUtilizationCount int `json:"high_utilization_count"`

	// 潜在浪费空间（GB）
	WastedSpaceGB float64 `json:"wasted_space_gb"`

	// 潜在节省成本（元/月）
	PotentialSavingsMonthly float64 `json:"potential_savings_monthly"`

	// 整体健康评分（0-100）
	HealthScore float64 `json:"health_score"`

	// 卷数量
	VolumeCount int `json:"volume_count"`
}

// UsageDistribution 使用率分布.
type UsageDistribution struct {
	// 极低（0-20%）
	VeryLow int `json:"very_low"`

	// 低（20-40%）
	Low int `json:"low"`

	// 中等（40-60%）
	Medium int `json:"medium"`

	// 高（60-80%）
	High int `json:"high"`

	// 极高（80-100%）
	VeryHigh int `json:"very_high"`
}

// UtilizationAlert 利用率告警.
type UtilizationAlert struct {
	// 告警ID
	ID string `json:"id"`

	// 卷名
	VolumeName string `json:"volume_name"`

	// 告警类型
	Type string `json:"type"` // high_usage, low_usage, capacity_prediction

	// 严重级别
	Severity string `json:"severity"` // info, warning, critical

	// 当前值
	CurrentValue float64 `json:"current_value"`

	// 阈值
	Threshold float64 `json:"threshold"`

	// 告警消息
	Message string `json:"message"`

	// 建议操作
	SuggestedAction string `json:"suggested_action"`

	// 触发时间
	TriggeredAt time.Time `json:"triggered_at"`
}

// UtilizationRecommendation 利用率优化建议.
type UtilizationRecommendation struct {
	// 建议ID
	ID string `json:"id"`

	// 目标卷
	VolumeName string `json:"volume_name,omitempty"`

	// 建议类型
	Type string `json:"type"` // rebalance, expand, shrink, cleanup, archive

	// 优先级
	Priority int `json:"priority"`

	// 标题
	Title string `json:"title"`

	// 描述
	Description string `json:"description"`

	// 预计节省空间（GB）
	SavingsGB float64 `json:"savings_gb"`

	// 预计节省成本（元/月）
	SavingsMonthly float64 `json:"savings_monthly"`

	// 实施难度
	Implementation string `json:"implementation"` // easy, medium, hard

	// 实施步骤
	Steps []string `json:"steps"`
}

// AnalyzeUtilization 分析存储利用率.
func (a *StorageUtilizationAnalyzer) AnalyzeUtilization(metrics []StorageMetrics, period ReportPeriod) *UtilizationAnalysis {
	now := time.Now()
	analysis := &UtilizationAnalysis{
		ID:              "util_" + now.Format("20060102150405"),
		AnalyzedAt:      now,
		Period:          period,
		Alerts:          make([]UtilizationAlert, 0),
		Recommendations: make([]UtilizationRecommendation, 0),
	}

	// 分析各卷利用率
	for _, m := range metrics {
		volUtil := a.analyzeVolumeUtilization(m)
		analysis.VolumeUtilizations = append(analysis.VolumeUtilizations, volUtil)

		// 生成告警
		alerts := a.generateUtilizationAlerts(volUtil)
		analysis.Alerts = append(analysis.Alerts, alerts...)

		// 生成建议
		recs := a.generateUtilizationRecommendations(volUtil)
		analysis.Recommendations = append(analysis.Recommendations, recs...)
	}

	// 计算汇总
	analysis.Summary = a.calculateUtilizationSummary(analysis.VolumeUtilizations)

	// 添加跨卷优化建议
	crossRecs := a.generateCrossVolumeRecommendations(analysis)
	analysis.Recommendations = append(analysis.Recommendations, crossRecs...)

	// 按优先级排序建议
	sort.Slice(analysis.Recommendations, func(i, j int) bool {
		return analysis.Recommendations[i].Priority > analysis.Recommendations[j].Priority
	})

	return analysis
}

// analyzeVolumeUtilization 分析单个卷利用率.
func (a *StorageUtilizationAnalyzer) analyzeVolumeUtilization(m StorageMetrics) VolumeUtilization {
	vol := VolumeUtilization{
		VolumeName:         m.VolumeName,
		TotalCapacityBytes: m.TotalCapacityBytes,
		UsedBytes:          m.UsedCapacityBytes,
		AvailableBytes:     m.AvailableCapacityBytes,
		FileCount:          m.FileCount,
		Timestamp:          m.Timestamp,
	}

	// 计算使用率
	if m.TotalCapacityBytes > 0 {
		vol.UsagePercent = round(float64(m.UsedCapacityBytes)/float64(m.TotalCapacityBytes)*100, 2)
	}

	// 评级
	vol.Rating = a.getUtilizationRating(vol.UsagePercent)

	// 计算平均文件大小
	if m.FileCount > 0 {
		vol.AvgFileSize = float64(m.UsedCapacityBytes) / float64(m.FileCount)
	}

	// IOPS利用率（假设最大10000 IOPS）
	maxIOPS := uint64(10000)
	if maxIOPS > 0 {
		vol.IOPSUtilization = round(float64(m.IOPS)/float64(maxIOPS)*100, 2)
	}

	// 带宽利用率（假设最大1000MB/s）
	maxBandwidth := uint64(1000 * 1024 * 1024)
	totalBandwidth := m.ReadBandwidthBytes + m.WriteBandwidthBytes
	if maxBandwidth > 0 {
		vol.BandwidthUtilization = round(float64(totalBandwidth)/float64(maxBandwidth)*100, 2)
	}

	// 成本效率（有效使用量/总成本）
	usedGB := float64(m.UsedCapacityBytes) / (1024 * 1024 * 1024)
	totalGB := float64(m.TotalCapacityBytes) / (1024 * 1024 * 1024)
	if totalGB > 0 {
		vol.CostEfficiency = round(a.config.CostPerGBMonthly*totalGB/usedGB, 2)
	}

	return vol
}

// getUtilizationRating 获取利用率评级.
func (a *StorageUtilizationAnalyzer) getUtilizationRating(usagePercent float64) string {
	switch {
	case usagePercent >= 90:
		return "critical"
	case usagePercent >= 75:
		return "warning"
	case usagePercent >= 50:
		return "good"
	default:
		return "excellent"
	}
}

// generateUtilizationAlerts 生成利用率告警.
func (a *StorageUtilizationAnalyzer) generateUtilizationAlerts(vol VolumeUtilization) []UtilizationAlert {
	alerts := make([]UtilizationAlert, 0)
	now := time.Now()

	// 高使用率告警
	if vol.UsagePercent >= 90 {
		alerts = append(alerts, UtilizationAlert{
			ID:              fmt.Sprintf("alert_%s_%d", vol.VolumeName, now.Unix()),
			VolumeName:      vol.VolumeName,
			Type:            "high_usage",
			Severity:        "critical",
			CurrentValue:    vol.UsagePercent,
			Threshold:       90,
			Message:         fmt.Sprintf("卷 %s 使用率已达 %.1f%%，需要立即处理", vol.VolumeName, vol.UsagePercent),
			SuggestedAction: "清理无用数据或扩容",
			TriggeredAt:     now,
		})
	} else if vol.UsagePercent >= 80 {
		alerts = append(alerts, UtilizationAlert{
			ID:              fmt.Sprintf("alert_%s_%d", vol.VolumeName, now.Unix()),
			VolumeName:      vol.VolumeName,
			Type:            "high_usage",
			Severity:        "warning",
			CurrentValue:    vol.UsagePercent,
			Threshold:       80,
			Message:         fmt.Sprintf("卷 %s 使用率已达 %.1f%%，建议关注", vol.VolumeName, vol.UsagePercent),
			SuggestedAction: "规划扩容或清理",
			TriggeredAt:     now,
		})
	}

	// 低使用率告警
	if vol.UsagePercent < 20 {
		alerts = append(alerts, UtilizationAlert{
			ID:              fmt.Sprintf("alert_%s_low_%d", vol.VolumeName, now.Unix()),
			VolumeName:      vol.VolumeName,
			Type:            "low_usage",
			Severity:        "info",
			CurrentValue:    vol.UsagePercent,
			Threshold:       20,
			Message:         fmt.Sprintf("卷 %s 使用率仅 %.1f%%，可能存在资源浪费", vol.VolumeName, vol.UsagePercent),
			SuggestedAction: "考虑收缩卷大小或重新分配资源",
			TriggeredAt:     now,
		})
	}

	return alerts
}

// generateUtilizationRecommendations 生成利用率优化建议.
func (a *StorageUtilizationAnalyzer) generateUtilizationRecommendations(vol VolumeUtilization) []UtilizationRecommendation {
	recs := make([]UtilizationRecommendation, 0)
	now := time.Now()

	totalGB := float64(vol.TotalCapacityBytes) / (1024 * 1024 * 1024)
	usedGB := float64(vol.UsedBytes) / (1024 * 1024 * 1024)
	availableGB := float64(vol.AvailableBytes) / (1024 * 1024 * 1024)

	// 高使用率建议
	if vol.UsagePercent >= 80 {
		recs = append(recs, UtilizationRecommendation{
			ID:             fmt.Sprintf("rec_%s_expand_%d", vol.VolumeName, now.Unix()),
			VolumeName:     vol.VolumeName,
			Type:           "expand",
			Priority:       9,
			Title:          fmt.Sprintf("扩展卷 %s 容量", vol.VolumeName),
			Description:    fmt.Sprintf("当前使用率 %.1f%%，建议扩容以避免空间不足", vol.UsagePercent),
			SavingsGB:      0,
			SavingsMonthly: 0,
			Implementation: "medium",
			Steps: []string{
				"1. 分析数据增长趋势",
				"2. 确定扩容目标大小",
				"3. 规划扩容时间窗口",
				"4. 执行卷扩容",
				"5. 验证扩容结果",
			},
		})
	}

	// 低使用率建议
	if vol.UsagePercent < 30 && totalGB > 100 {
		wastedGB := availableGB * 0.5 // 假设回收一半空闲空间
		recs = append(recs, UtilizationRecommendation{
			ID:             fmt.Sprintf("rec_%s_shrink_%d", vol.VolumeName, now.Unix()),
			VolumeName:     vol.VolumeName,
			Type:           "shrink",
			Priority:       5,
			Title:          fmt.Sprintf("收缩卷 %s 容量", vol.VolumeName),
			Description:    fmt.Sprintf("当前使用率仅 %.1f%%，建议收缩以释放资源", vol.UsagePercent),
			SavingsGB:      wastedGB,
			SavingsMonthly: round(wastedGB*a.config.CostPerGBMonthly, 2),
			Implementation: "medium",
			Steps: []string{
				"1. 分析实际存储需求",
				"2. 确定合理容量目标",
				"3. 预留安全余量",
				"4. 执行卷收缩",
				"5. 验证数据完整性",
			},
		})
	}

	// 清理建议
	if vol.UsagePercent >= 60 {
		cleanupGB := usedGB * 0.1 // 假设可清理10%
		recs = append(recs, UtilizationRecommendation{
			ID:             fmt.Sprintf("rec_%s_cleanup_%d", vol.VolumeName, now.Unix()),
			VolumeName:     vol.VolumeName,
			Type:           "cleanup",
			Priority:       7,
			Title:          fmt.Sprintf("清理卷 %s 无用数据", vol.VolumeName),
			Description:    "识别并清理过期、重复、临时文件以释放空间",
			SavingsGB:      cleanupGB,
			SavingsMonthly: round(cleanupGB*a.config.CostPerGBMonthly, 2),
			Implementation: "easy",
			Steps: []string{
				"1. 扫描过期文件",
				"2. 识别重复数据",
				"3. 清理临时文件",
				"4. 归档冷数据",
				"5. 验证清理效果",
			},
		})
	}

	return recs
}

// generateCrossVolumeRecommendations 生成跨卷优化建议.
func (a *StorageUtilizationAnalyzer) generateCrossVolumeRecommendations(analysis *UtilizationAnalysis) []UtilizationRecommendation {
	recs := make([]UtilizationRecommendation, 0)
	now := time.Now()

	// 检查是否需要负载均衡
	var lowUsageVols, highUsageVols []VolumeUtilization
	for _, vol := range analysis.VolumeUtilizations {
		if vol.UsagePercent < 40 {
			lowUsageVols = append(lowUsageVols, vol)
		} else if vol.UsagePercent > 75 {
			highUsageVols = append(highUsageVols, vol)
		}
	}

	if len(lowUsageVols) > 0 && len(highUsageVols) > 0 {
		recs = append(recs, UtilizationRecommendation{
			ID:             fmt.Sprintf("rec_rebalance_%d", now.Unix()),
			Type:           "rebalance",
			Priority:       6,
			Title:          "存储负载均衡",
			Description:    "发现使用率不均衡的卷，建议迁移数据以优化资源利用",
			SavingsGB:      0,
			SavingsMonthly: 0,
			Implementation: "hard",
			Steps: []string{
				"1. 分析各卷数据类型",
				"2. 制定迁移计划",
				"3. 选择迁移时机",
				"4. 执行数据迁移",
				"5. 更新访问路径",
			},
		})
	}

	return recs
}

// calculateUtilizationSummary 计算利用率汇总.
func (a *StorageUtilizationAnalyzer) calculateUtilizationSummary(vols []VolumeUtilization) UtilizationSummary {
	summary := UtilizationSummary{
		VolumeCount: len(vols),
	}

	var totalUsage, maxUsage, minUsage float64
	minUsage = 100.0

	for _, vol := range vols {
		totalGB := float64(vol.TotalCapacityBytes) / (1024 * 1024 * 1024)
		usedGB := float64(vol.UsedBytes) / (1024 * 1024 * 1024)
		availableGB := float64(vol.AvailableBytes) / (1024 * 1024 * 1024)

		summary.TotalCapacityGB += totalGB
		summary.TotalUsedGB += usedGB
		summary.TotalAvailableGB += availableGB

		totalUsage += vol.UsagePercent
		if vol.UsagePercent > maxUsage {
			maxUsage = vol.UsagePercent
		}
		if vol.UsagePercent < minUsage {
			minUsage = vol.UsagePercent
		}

		// 统计使用率分布
		switch {
		case vol.UsagePercent < 20:
			summary.UsageDistribution.VeryLow++
		case vol.UsagePercent < 40:
			summary.UsageDistribution.Low++
		case vol.UsagePercent < 60:
			summary.UsageDistribution.Medium++
		case vol.UsagePercent < 80:
			summary.UsageDistribution.High++
		default:
			summary.UsageDistribution.VeryHigh++
		}

		// 统计高低利用率
		if vol.UsagePercent < 30 {
			summary.LowUtilizationCount++
			// 计算潜在浪费
			wastedGB := availableGB * 0.5
			summary.WastedSpaceGB += wastedGB
		}
		if vol.UsagePercent > 80 {
			summary.HighUtilizationCount++
		}
	}

	if len(vols) > 0 {
		summary.AvgUsagePercent = round(totalUsage/float64(len(vols)), 2)
	}

	summary.MaxUsagePercent = round(maxUsage, 2)
	summary.MinUsagePercent = round(minUsage, 2)

	// 四舍五入
	summary.TotalCapacityGB = round(summary.TotalCapacityGB, 2)
	summary.TotalUsedGB = round(summary.TotalUsedGB, 2)
	summary.TotalAvailableGB = round(summary.TotalAvailableGB, 2)
	summary.WastedSpaceGB = round(summary.WastedSpaceGB, 2)

	// 计算潜在节省
	summary.PotentialSavingsMonthly = round(summary.WastedSpaceGB*a.config.CostPerGBMonthly, 2)

	// 计算健康评分
	summary.HealthScore = a.calculateHealthScore(&summary)

	return summary
}

// calculateHealthScore 计算健康评分.
func (a *StorageUtilizationAnalyzer) calculateHealthScore(summary *UtilizationSummary) float64 {
	score := 100.0

	// 使用率过高扣分
	score -= float64(summary.HighUtilizationCount) * 10

	// 使用率过低扣分
	score -= float64(summary.LowUtilizationCount) * 3

	// 平均使用率偏离扣分
	avgDiff := math.Abs(summary.AvgUsagePercent - 60) // 理想使用率60%
	if avgDiff > 20 {
		score -= avgDiff - 20
	}

	// 浪费空间扣分
	if summary.TotalCapacityGB > 0 {
		wastePercent := summary.WastedSpaceGB / summary.TotalCapacityGB * 100
		score -= wastePercent
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return round(score, 1)
}

// ========== 冗余数据识别 v2.45.0 ==========

// RedundantDataScanner 冗余数据扫描器.
type RedundantDataScanner struct {
	config StorageCostConfig
}

// NewRedundantDataScanner 创建冗余数据扫描器.
func NewRedundantDataScanner(config StorageCostConfig) *RedundantDataScanner {
	return &RedundantDataScanner{config: config}
}

// RedundantDataScanResult 冗余数据扫描结果.
type RedundantDataScanResult struct {
	// 扫描ID
	ID string `json:"id"`

	// 扫描时间
	ScannedAt time.Time `json:"scanned_at"`

	// 扫描范围
	Scope string `json:"scope"` // volume, directory, user

	// 目标名称
	TargetName string `json:"target_name"`

	// 冗余数据列表
	RedundantItems []RedundantDataItem `json:"redundant_items"`

	// 汇总
	Summary RedundantDataSummary `json:"summary"`

	// 扫描耗时
	Duration time.Duration `json:"duration"`
}

// RedundantDataItem 冗余数据项.
type RedundantDataItem struct {
	// 类型
	Type string `json:"type"` // duplicate, old_version, orphan, temp, expired

	// 路径
	Path string `json:"path"`

	// 文件名
	Name string `json:"name"`

	// 大小（字节）
	SizeBytes uint64 `json:"size_bytes"`

	// 创建时间
	CreatedAt time.Time `json:"created_at"`

	// 最后访问时间
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`

	// 最后修改时间
	LastModifiedAt time.Time `json:"last_modified_at"`

	// 冗余原因
	Reason string `json:"reason"`

	// 是否可安全删除
	SafeToDelete bool `json:"safe_to_delete"`

	// 删除风险
	DeleteRisk string `json:"delete_risk"` // low, medium, high

	// 关联文件（如重复文件的原文件）
	RelatedPath string `json:"related_path,omitempty"`
}

// RedundantDataSummary 冗余数据汇总.
type RedundantDataSummary struct {
	// 总冗余数据量（字节）
	TotalRedundantBytes uint64 `json:"total_redundant_bytes"`

	// 总冗余数据量（GB）
	TotalRedundantGB float64 `json:"total_redundant_gb"`

	// 按类型统计大小
	ByTypeBytes map[string]uint64 `json:"by_type_bytes"`

	// 按类型统计数量
	ByTypeCount map[string]int `json:"by_type_count"`

	// 冗余率（%）
	RedundantPercent float64 `json:"redundant_percent"`

	// 可安全删除量（字节）
	SafeToDeleteBytes uint64 `json:"safe_to_delete_bytes"`

	// 预计节省成本（元/月）
	PotentialSavingsMonthly float64 `json:"potential_savings_monthly"`

	// 总文件数
	TotalFiles int `json:"total_files"`

	// 扫描的总空间（字节）
	ScannedBytes uint64 `json:"scanned_bytes"`
}

// ScanRedundantData 扫描冗余数据.
func (s *RedundantDataScanner) ScanRedundantData(
	duplicates []DuplicateFileInfo,
	orphanFiles []OrphanFileInfo,
	tempFiles []TempFileInfo,
	expiredFiles []ExpiredFileInfo,
	oldVersions []OldVersionFileInfo,
	totalScannedBytes uint64,
) *RedundantDataScanResult {
	now := time.Now()
	startTime := now

	result := &RedundantDataScanResult{
		ID:             "scan_" + now.Format("20060102150405"),
		ScannedAt:      now,
		RedundantItems: make([]RedundantDataItem, 0),
		Summary: RedundantDataSummary{
			ByTypeBytes:  make(map[string]uint64),
			ByTypeCount:  make(map[string]int),
			ScannedBytes: totalScannedBytes,
		},
	}

	// 处理重复文件
	for _, dup := range duplicates {
		item := RedundantDataItem{
			Type:           "duplicate",
			Path:           dup.Path,
			Name:           dup.Name,
			SizeBytes:      dup.SizeBytes,
			CreatedAt:      dup.CreatedAt,
			LastModifiedAt: dup.ModifiedAt,
			Reason:         "与 " + dup.OriginalPath + " 内容相同",
			SafeToDelete:   true,
			DeleteRisk:     "low",
			RelatedPath:    dup.OriginalPath,
		}
		result.RedundantItems = append(result.RedundantItems, item)
		result.Summary.TotalRedundantBytes += dup.SizeBytes
		result.Summary.ByTypeBytes["duplicate"] += dup.SizeBytes
		result.Summary.ByTypeCount["duplicate"]++
		result.Summary.SafeToDeleteBytes += dup.SizeBytes
	}

	// 处理孤立文件
	for _, orphan := range orphanFiles {
		item := RedundantDataItem{
			Type:           "orphan",
			Path:           orphan.Path,
			Name:           orphan.Name,
			SizeBytes:      orphan.SizeBytes,
			CreatedAt:      orphan.CreatedAt,
			LastAccessedAt: orphan.LastAccessedAt,
			LastModifiedAt: orphan.ModifiedAt,
			Reason:         "所有者不存在或无引用",
			SafeToDelete:   orphan.SafeToDelete,
			DeleteRisk:     orphan.Risk,
		}
		result.RedundantItems = append(result.RedundantItems, item)
		result.Summary.TotalRedundantBytes += orphan.SizeBytes
		result.Summary.ByTypeBytes["orphan"] += orphan.SizeBytes
		result.Summary.ByTypeCount["orphan"]++
		if orphan.SafeToDelete {
			result.Summary.SafeToDeleteBytes += orphan.SizeBytes
		}
	}

	// 处理临时文件
	for _, temp := range tempFiles {
		item := RedundantDataItem{
			Type:           "temp",
			Path:           temp.Path,
			Name:           temp.Name,
			SizeBytes:      temp.SizeBytes,
			CreatedAt:      temp.CreatedAt,
			LastModifiedAt: temp.ModifiedAt,
			Reason:         "临时文件已过期（超过 " + fmt.Sprintf("%d", temp.MaxAgeDays) + " 天）",
			SafeToDelete:   true,
			DeleteRisk:     "low",
		}
		result.RedundantItems = append(result.RedundantItems, item)
		result.Summary.TotalRedundantBytes += temp.SizeBytes
		result.Summary.ByTypeBytes["temp"] += temp.SizeBytes
		result.Summary.ByTypeCount["temp"]++
		result.Summary.SafeToDeleteBytes += temp.SizeBytes
	}

	// 处理过期文件
	for _, expired := range expiredFiles {
		item := RedundantDataItem{
			Type:           "expired",
			Path:           expired.Path,
			Name:           expired.Name,
			SizeBytes:      expired.SizeBytes,
			CreatedAt:      expired.CreatedAt,
			LastAccessedAt: expired.LastAccessedAt,
			LastModifiedAt: expired.ModifiedAt,
			Reason:         "超过保留期限（" + expired.RetentionPolicy + "）",
			SafeToDelete:   expired.SafeToDelete,
			DeleteRisk:     expired.Risk,
		}
		result.RedundantItems = append(result.RedundantItems, item)
		result.Summary.TotalRedundantBytes += expired.SizeBytes
		result.Summary.ByTypeBytes["expired"] += expired.SizeBytes
		result.Summary.ByTypeCount["expired"]++
		if expired.SafeToDelete {
			result.Summary.SafeToDeleteBytes += expired.SizeBytes
		}
	}

	// 处理旧版本文件
	for _, ver := range oldVersions {
		item := RedundantDataItem{
			Type:           "old_version",
			Path:           ver.Path,
			Name:           ver.Name,
			SizeBytes:      ver.SizeBytes,
			CreatedAt:      ver.CreatedAt,
			LastModifiedAt: ver.ModifiedAt,
			Reason:         "已有 " + fmt.Sprintf("%d", ver.NewerVersions) + " 个更新版本",
			SafeToDelete:   ver.SafeToDelete,
			DeleteRisk:     ver.Risk,
		}
		result.RedundantItems = append(result.RedundantItems, item)
		result.Summary.TotalRedundantBytes += ver.SizeBytes
		result.Summary.ByTypeBytes["old_version"] += ver.SizeBytes
		result.Summary.ByTypeCount["old_version"]++
		if ver.SafeToDelete {
			result.Summary.SafeToDeleteBytes += ver.SizeBytes
		}
	}

	// 计算汇总统计
	result.Summary.TotalFiles = len(result.RedundantItems)
	result.Summary.TotalRedundantGB = round(float64(result.Summary.TotalRedundantBytes)/(1024*1024*1024), 2)

	if totalScannedBytes > 0 {
		result.Summary.RedundantPercent = round(float64(result.Summary.TotalRedundantBytes)/float64(totalScannedBytes)*100, 2)
	}

	result.Summary.PotentialSavingsMonthly = round(result.Summary.TotalRedundantGB*s.config.CostPerGBMonthly, 2)

	// 计算扫描耗时
	result.Duration = time.Since(startTime)

	return result
}

// DuplicateFileInfo 重复文件信息.
type DuplicateFileInfo struct {
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	OriginalPath string    `json:"original_path"`
	SizeBytes    uint64    `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
	ModifiedAt   time.Time `json:"modified_at"`
	Hash         string    `json:"hash"`
}

// OrphanFileInfo 孤立文件信息.
type OrphanFileInfo struct {
	Path           string     `json:"path"`
	Name           string     `json:"name"`
	SizeBytes      uint64     `json:"size_bytes"`
	CreatedAt      time.Time  `json:"created_at"`
	ModifiedAt     time.Time  `json:"modified_at"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	Owner          string     `json:"owner,omitempty"`
	SafeToDelete   bool       `json:"safe_to_delete"`
	Risk           string     `json:"risk"`
}

// TempFileInfo 临时文件信息.
type TempFileInfo struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	SizeBytes  uint64    `json:"size_bytes"`
	CreatedAt  time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
	MaxAgeDays int       `json:"max_age_days"`
}

// ExpiredFileInfo 过期文件信息.
type ExpiredFileInfo struct {
	Path            string     `json:"path"`
	Name            string     `json:"name"`
	SizeBytes       uint64     `json:"size_bytes"`
	CreatedAt       time.Time  `json:"created_at"`
	ModifiedAt      time.Time  `json:"modified_at"`
	LastAccessedAt  *time.Time `json:"last_accessed_at,omitempty"`
	RetentionPolicy string     `json:"retention_policy"`
	SafeToDelete    bool       `json:"safe_to_delete"`
	Risk            string     `json:"risk"`
}

// OldVersionFileInfo 旧版本文件信息.
type OldVersionFileInfo struct {
	Path          string    `json:"path"`
	Name          string    `json:"name"`
	SizeBytes     uint64    `json:"size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
	ModifiedAt    time.Time `json:"modified_at"`
	NewerVersions int       `json:"newer_versions"`
	SafeToDelete  bool      `json:"safe_to_delete"`
	Risk          string    `json:"risk"`
}

// ========== 成本节省建议生成 v2.45.0 ==========

// CostSavingsGenerator 成本节省建议生成器.
type CostSavingsGenerator struct {
	config StorageCostConfig
}

// NewCostSavingsGenerator 创建成本节省建议生成器.
func NewCostSavingsGenerator(config StorageCostConfig) *CostSavingsGenerator {
	return &CostSavingsGenerator{config: config}
}

// CostSavingsReport 成本节省报告.
type CostSavingsReport struct {
	// 报告ID
	ID string `json:"id"`

	// 报告名称
	Name string `json:"name"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 分析周期
	Period ReportPeriod `json:"period"`

	// 节省建议列表
	SavingsOpportunities []SavingsOpportunity `json:"savings_opportunities"`

	// 汇总
	Summary CostSavingsSummary `json:"summary"`

	// 快速见效项目
	QuickWins []SavingsOpportunity `json:"quick_wins"`

	// 长期优化项目
	LongTermProjects []SavingsOpportunity `json:"long_term_projects"`
}

// SavingsOpportunity 节省机会.
type SavingsOpportunity struct {
	// ID
	ID string `json:"id"`

	// 类型
	Type string `json:"type"` // cleanup, dedupe, compress, archive, tier, resize

	// 标题
	Title string `json:"title"`

	// 描述
	Description string `json:"description"`

	// 目标范围
	Scope string `json:"scope"` // volume, user, system-wide

	// 目标名称
	TargetName string `json:"target_name,omitempty"`

	// 当前月成本（元）
	CurrentCostMonthly float64 `json:"current_cost_monthly"`

	// 优化后月成本（元）
	OptimizedCostMonthly float64 `json:"optimized_cost_monthly"`

	// 月节省金额（元）
	SavingsMonthly float64 `json:"savings_monthly"`

	// 年节省金额（元）
	SavingsYearly float64 `json:"savings_yearly"`

	// 节省空间（GB）
	SavingsGB float64 `json:"savings_gb"`

	// 节省比例（%）
	SavingsPercent float64 `json:"savings_percent"`

	// 实施难度
	Implementation string `json:"implementation"` // easy, medium, hard

	// 预计实施时间
	EstimatedTime string `json:"estimated_time"`

	// 投资回报周期（月）
	ROIMonths int `json:"roi_months"`

	// 优先级（1-10）
	Priority int `json:"priority"`

	// 风险等级
	Risk string `json:"risk"` // low, medium, high

	// 实施步骤
	Steps []string `json:"steps"`

	// 前提条件
	Prerequisites []string `json:"prerequisites,omitempty"`

	// 预期收益
	ExpectedBenefits []string `json:"expected_benefits"`

	// 潜在风险
	PotentialRisks []string `json:"potential_risks"`
}

// CostSavingsSummary 成本节省汇总.
type CostSavingsSummary struct {
	// 总机会数
	TotalOpportunities int `json:"total_opportunities"`

	// 总月度节省（元）
	TotalSavingsMonthly float64 `json:"total_savings_monthly"`

	// 总年度节省（元）
	TotalSavingsYearly float64 `json:"total_savings_yearly"`

	// 总节省空间（GB）
	TotalSavingsGB float64 `json:"total_savings_gb"`

	// 平均节省比例
	AvgSavingsPercent float64 `json:"avg_savings_percent"`

	// 按类型统计
	ByType map[string]SavingsTypeStats `json:"by_type"`

	// 按难度统计
	ByDifficulty map[string]int `json:"by_difficulty"`

	// 快速见效项目数
	QuickWinCount int `json:"quick_win_count"`

	// 当前总月成本
	CurrentTotalCostMonthly float64 `json:"current_total_cost_monthly"`

	// 优化后总月成本
	OptimizedTotalCostMonthly float64 `json:"optimized_total_cost_monthly"`

	// 整体节省比例
	OverallSavingsPercent float64 `json:"overall_savings_percent"`
}

// SavingsTypeStats 节省类型统计.
type SavingsTypeStats struct {
	Count          int     `json:"count"`
	SavingsMonthly float64 `json:"savings_monthly"`
	SavingsGB      float64 `json:"savings_gb"`
}

// GenerateCostSavingsReport 生成成本节省报告.
func (g *CostSavingsGenerator) GenerateCostSavingsReport(
	utilization *UtilizationAnalysis,
	redundantScan *RedundantDataScanResult,
	costs []StorageCostResult,
	period ReportPeriod,
) *CostSavingsReport {
	now := time.Now()
	report := &CostSavingsReport{
		ID:                   "savings_" + now.Format("20060102150405"),
		Name:                 "成本节省建议报告",
		GeneratedAt:          now,
		Period:               period,
		SavingsOpportunities: make([]SavingsOpportunity, 0),
		QuickWins:            make([]SavingsOpportunity, 0),
		LongTermProjects:     make([]SavingsOpportunity, 0),
	}

	// 基于利用率分析生成建议
	for _, rec := range utilization.Recommendations {
		opp := g.convertRecommendationToOpportunity(rec)
		report.SavingsOpportunities = append(report.SavingsOpportunities, opp)
	}

	// 基于冗余数据生成建议
	if redundantScan != nil {
		opp := g.generateRedundancySavingsOpportunity(redundantScan)
		if opp != nil {
			report.SavingsOpportunities = append(report.SavingsOpportunities, *opp)
		}
	}

	// 基于成本分析生成建议
	for _, cost := range costs {
		if cost.UsagePercent > 80 {
			opp := g.generateHighUsageOpportunity(cost)
			report.SavingsOpportunities = append(report.SavingsOpportunities, opp)
		}
		if cost.UsagePercent < 30 {
			opp := g.generateLowUsageOpportunity(cost)
			report.SavingsOpportunities = append(report.SavingsOpportunities, opp)
		}
	}

	// 去重和排序
	report.SavingsOpportunities = g.deduplicateOpportunities(report.SavingsOpportunities)
	sort.Slice(report.SavingsOpportunities, func(i, j int) bool {
		return report.SavingsOpportunities[i].Priority > report.SavingsOpportunities[j].Priority
	})

	// 计算汇总
	report.Summary = g.calculateSavingsSummary(report.SavingsOpportunities)

	// 分类快速见效和长期项目
	for _, opp := range report.SavingsOpportunities {
		if opp.Implementation == "easy" && opp.ROIMonths <= 1 {
			report.QuickWins = append(report.QuickWins, opp)
		} else if opp.Implementation == "hard" || opp.ROIMonths > 3 {
			report.LongTermProjects = append(report.LongTermProjects, opp)
		}
	}

	return report
}

// convertRecommendationToOpportunity 转换建议为节省机会.
func (g *CostSavingsGenerator) convertRecommendationToOpportunity(rec UtilizationRecommendation) SavingsOpportunity {
	opp := SavingsOpportunity{
		ID:               rec.ID,
		Type:             rec.Type,
		Title:            rec.Title,
		Description:      rec.Description,
		TargetName:       rec.VolumeName,
		Scope:            "volume",
		SavingsGB:        rec.SavingsGB,
		SavingsMonthly:   rec.SavingsMonthly,
		SavingsYearly:    round(rec.SavingsMonthly*12, 2),
		Implementation:   rec.Implementation,
		Steps:            rec.Steps,
		ExpectedBenefits: []string{"降低存储成本", "提高资源利用率"},
		PotentialRisks:   []string{"需要短暂服务中断"},
	}

	// 计算优先级和ROI
	switch rec.Priority {
	case 9, 10:
		opp.Priority = 10
		opp.Risk = "high"
	case 7, 8:
		opp.Priority = 8
		opp.Risk = "medium"
	default:
		opp.Priority = 5
		opp.Risk = "low"
	}

	// 估算实施时间和ROI
	switch opp.Implementation {
	case "easy":
		opp.EstimatedTime = "1-3天"
		opp.ROIMonths = 1
	case "medium":
		opp.EstimatedTime = "1-2周"
		opp.ROIMonths = 2
	case "hard":
		opp.EstimatedTime = "2-4周"
		opp.ROIMonths = 3
	}

	// 计算节省比例
	if opp.SavingsMonthly > 0 {
		opp.SavingsPercent = round(opp.SavingsMonthly/g.config.CostPerGBMonthly/opp.SavingsGB*100, 2)
	}

	return opp
}

// generateRedundancySavingsOpportunity 生成冗余数据节省机会.
func (g *CostSavingsGenerator) generateRedundancySavingsOpportunity(scan *RedundantDataScanResult) *SavingsOpportunity {
	if scan.Summary.TotalRedundantBytes == 0 {
		return nil
	}

	now := time.Now()
	return &SavingsOpportunity{
		ID:               "savings_redundancy_" + now.Format("20060102"),
		Type:             "cleanup",
		Title:            "清理冗余数据",
		Description:      fmt.Sprintf("发现 %.2f GB 冗余数据，可节省 %.2f 元/月", scan.Summary.TotalRedundantGB, scan.Summary.PotentialSavingsMonthly),
		Scope:            "system-wide",
		SavingsGB:        scan.Summary.TotalRedundantGB,
		SavingsMonthly:   scan.Summary.PotentialSavingsMonthly,
		SavingsYearly:    round(scan.Summary.PotentialSavingsMonthly*12, 2),
		Implementation:   "easy",
		EstimatedTime:    "1-3天",
		ROIMonths:        1,
		Priority:         8,
		Risk:             "low",
		Steps:            []string{"审核冗余数据列表", "确认删除范围", "执行清理", "验证结果"},
		ExpectedBenefits: []string{"释放存储空间", "降低存储成本", "提高管理效率"},
		PotentialRisks:   []string{"误删有用数据"},
	}
}

// generateHighUsageOpportunity 生成高使用率优化机会.
func (g *CostSavingsGenerator) generateHighUsageOpportunity(cost StorageCostResult) SavingsOpportunity {
	now := time.Now()
	return SavingsOpportunity{
		ID:                   fmt.Sprintf("savings_high_%s_%s", cost.VolumeName, now.Format("20060102")),
		Type:                 "resize",
		Title:                fmt.Sprintf("扩展卷 %s 避免空间耗尽", cost.VolumeName),
		Description:          fmt.Sprintf("当前使用率 %.1f%%，存在空间不足风险", cost.UsagePercent),
		Scope:                "volume",
		TargetName:           cost.VolumeName,
		CurrentCostMonthly:   cost.TotalCostMonthly,
		OptimizedCostMonthly: round(cost.TotalCostMonthly*1.2, 2),
		SavingsMonthly:       0, // 扩容不直接节省
		Implementation:       "medium",
		EstimatedTime:        "1-2周",
		ROIMonths:            0,
		Priority:             9,
		Risk:                 "medium",
		Steps: []string{
			"分析数据增长趋势",
			"规划扩容方案",
			"执行扩容操作",
			"验证扩容结果",
		},
		ExpectedBenefits: []string{"避免存储空间耗尽", "保障业务连续性"},
		PotentialRisks:   []string{"扩容期间服务中断"},
	}
}

// generateLowUsageOpportunity 生成低使用率优化机会.
func (g *CostSavingsGenerator) generateLowUsageOpportunity(cost StorageCostResult) SavingsOpportunity {
	now := time.Now()
	// 假设收缩50%的空间
	savingsGB := cost.TotalCostMonthly / g.config.CostPerGBMonthly * 0.5
	savingsMonthly := savingsGB * g.config.CostPerGBMonthly

	return SavingsOpportunity{
		ID:                   fmt.Sprintf("savings_low_%s_%s", cost.VolumeName, now.Format("20060102")),
		Type:                 "resize",
		Title:                fmt.Sprintf("收缩卷 %s 释放闲置资源", cost.VolumeName),
		Description:          fmt.Sprintf("当前使用率仅 %.1f%%，存在资源浪费", cost.UsagePercent),
		Scope:                "volume",
		TargetName:           cost.VolumeName,
		CurrentCostMonthly:   cost.TotalCostMonthly,
		OptimizedCostMonthly: round(cost.TotalCostMonthly-savingsMonthly, 2),
		SavingsMonthly:       round(savingsMonthly, 2),
		SavingsYearly:        round(savingsMonthly*12, 2),
		SavingsGB:            round(savingsGB, 2),
		Implementation:       "medium",
		EstimatedTime:        "1-2周",
		ROIMonths:            1,
		Priority:             5,
		Risk:                 "low",
		Steps: []string{
			"分析实际存储需求",
			"确定收缩目标",
			"预留安全余量",
			"执行收缩操作",
		},
		ExpectedBenefits: []string{"释放闲置资源", "降低存储成本"},
		PotentialRisks:   []string{"未来扩容需求"},
	}
}

// deduplicateOpportunities 去重机会列表.
func (g *CostSavingsGenerator) deduplicateOpportunities(opps []SavingsOpportunity) []SavingsOpportunity {
	seen := make(map[string]bool)
	result := make([]SavingsOpportunity, 0)

	for _, opp := range opps {
		key := opp.Type + "_" + opp.TargetName
		if !seen[key] {
			seen[key] = true
			result = append(result, opp)
		}
	}

	return result
}

// calculateSavingsSummary 计算节省汇总.
func (g *CostSavingsGenerator) calculateSavingsSummary(opps []SavingsOpportunity) CostSavingsSummary {
	summary := CostSavingsSummary{
		TotalOpportunities: len(opps),
		ByType:             make(map[string]SavingsTypeStats),
		ByDifficulty:       make(map[string]int),
	}

	for _, opp := range opps {
		summary.TotalSavingsMonthly += opp.SavingsMonthly
		summary.TotalSavingsYearly += opp.SavingsYearly
		summary.TotalSavingsGB += opp.SavingsGB
		summary.CurrentTotalCostMonthly += opp.CurrentCostMonthly

		// 按类型统计
		stats := summary.ByType[opp.Type]
		stats.Count++
		stats.SavingsMonthly += opp.SavingsMonthly
		stats.SavingsGB += opp.SavingsGB
		summary.ByType[opp.Type] = stats

		// 按难度统计
		summary.ByDifficulty[opp.Implementation]++

		// 统计快速见效
		if opp.Implementation == "easy" && opp.ROIMonths <= 1 {
			summary.QuickWinCount++
		}
	}

	summary.TotalSavingsMonthly = round(summary.TotalSavingsMonthly, 2)
	summary.TotalSavingsYearly = round(summary.TotalSavingsYearly, 2)
	summary.TotalSavingsGB = round(summary.TotalSavingsGB, 2)

	if len(opps) > 0 {
		summary.AvgSavingsPercent = round(summary.TotalSavingsMonthly/float64(len(opps)), 2)
	}

	summary.OptimizedTotalCostMonthly = round(summary.CurrentTotalCostMonthly-summary.TotalSavingsMonthly, 2)

	if summary.CurrentTotalCostMonthly > 0 {
		summary.OverallSavingsPercent = round(summary.TotalSavingsMonthly/summary.CurrentTotalCostMonthly*100, 2)
	}

	return summary
}

// ========== 导出功能 v2.45.0 ==========

// StorageCostReportExporter 存储成本报告导出器.
type StorageCostReportExporter struct {
	outputDir string
}

// NewStorageCostReportExporter 创建导出器.
func NewStorageCostReportExporter(outputDir string) *StorageCostReportExporter {
	_ = os.MkdirAll(outputDir, 0750)
	return &StorageCostReportExporter{outputDir: outputDir}
}

// ExportReport 导出报告.
func (e *StorageCostReportExporter) ExportReport(report interface{}, format string, filename string) (string, error) {
	outputPath := filepath.Join(e.outputDir, filename)

	switch strings.ToLower(format) {
	case "json":
		return e.exportJSON(report, outputPath)
	case "csv":
		return e.exportCSV(report, outputPath)
	default:
		return e.exportJSON(report, outputPath)
	}
}

// exportJSON 导出为JSON.
func (e *StorageCostReportExporter) exportJSON(data interface{}, outputPath string) (string, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON序列化失败: %w", err)
	}

	if err := os.WriteFile(outputPath, jsonData, 0640); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return outputPath, nil
}

// exportCSV 导出为CSV.
func (e *StorageCostReportExporter) exportCSV(data interface{}, outputPath string) (string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	switch v := data.(type) {
	case *StorageCostReport:
		if err := e.writeStorageCostReportCSV(writer, v); err != nil {
			return "", err
		}
	case *UtilizationAnalysis:
		if err := e.writeUtilizationAnalysisCSV(writer, v); err != nil {
			return "", err
		}
	case *RedundantDataScanResult:
		if err := e.writeRedundantDataScanCSV(writer, v); err != nil {
			return "", err
		}
	case *CostSavingsReport:
		if err := e.writeCostSavingsReportCSV(writer, v); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("不支持的报告类型")
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("CSV写入失败: %w", err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0640); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return outputPath, nil
}

// writeStorageCostReportCSV 写入存储成本报告CSV.
func (e *StorageCostReportExporter) writeStorageCostReportCSV(writer *csv.Writer, report *StorageCostReport) error {
	// 写入标题
	if err := writer.Write([]string{"存储成本报告"}); err != nil {
		return err
	}
	if err := writer.Write([]string{"报告ID", report.ID}); err != nil {
		return err
	}
	if err := writer.Write([]string{"生成时间", report.GeneratedAt.Format("2006-01-02 15:04:05")}); err != nil {
		return err
	}
	if err := writer.Write([]string{}); err != nil {
		return err
	}

	// 写入汇总
	if err := writer.Write([]string{"汇总统计"}); err != nil {
		return err
	}
	if err := writer.Write([]string{"总月成本(元)", fmt.Sprintf("%.2f", report.Summary.TotalCostMonthly)}); err != nil {
		return err
	}
	if err := writer.Write([]string{"总容量(GB)", fmt.Sprintf("%.2f", report.Summary.TotalCapacityGB)}); err != nil {
		return err
	}
	if err := writer.Write([]string{"总使用量(GB)", fmt.Sprintf("%.2f", report.Summary.TotalUsedGB)}); err != nil {
		return err
	}
	if err := writer.Write([]string{"平均使用率(%)", fmt.Sprintf("%.2f", report.Summary.AvgUsagePercent)}); err != nil {
		return err
	}
	if err := writer.Write([]string{}); err != nil {
		return err
	}

	// 写入明细表头
	if err := writer.Write([]string{"卷名", "容量成本", "IOPS成本", "带宽成本", "电费成本", "运维成本", "折旧成本", "总成本", "单位成本", "使用率(%)"}); err != nil {
		return err
	}

	for _, cost := range report.VolumeCosts {
		if err := writer.Write([]string{
			cost.VolumeName,
			fmt.Sprintf("%.2f", cost.CapacityCostMonthly),
			fmt.Sprintf("%.2f", cost.IOPSCostMonthly),
			fmt.Sprintf("%.2f", cost.BandwidthCostMonthly),
			fmt.Sprintf("%.2f", cost.ElectricityCostMonthly),
			fmt.Sprintf("%.2f", cost.OpsCostMonthly),
			fmt.Sprintf("%.2f", cost.DepreciationCostMonthly),
			fmt.Sprintf("%.2f", cost.TotalCostMonthly),
			fmt.Sprintf("%.2f", cost.CostPerGBMonthly),
			fmt.Sprintf("%.2f", cost.UsagePercent),
		}); err != nil {
			return err
		}
	}

	return nil
}

// writeUtilizationAnalysisCSV 写入利用率分析CSV.
func (e *StorageCostReportExporter) writeUtilizationAnalysisCSV(writer *csv.Writer, report *UtilizationAnalysis) error {
	// 写入标题
	if err := writer.Write([]string{"存储利用率分析报告"}); err != nil {
		return err
	}
	if err := writer.Write([]string{"分析ID", report.ID}); err != nil {
		return err
	}
	if err := writer.Write([]string{"分析时间", report.AnalyzedAt.Format("2006-01-02 15:04:05")}); err != nil {
		return err
	}
	if err := writer.Write([]string{}); err != nil {
		return err
	}

	// 写入汇总
	if err := writer.Write([]string{"汇总统计"}); err != nil {
		return err
	}
	if err := writer.Write([]string{"总容量(GB)", fmt.Sprintf("%.2f", report.Summary.TotalCapacityGB)}); err != nil {
		return err
	}
	if err := writer.Write([]string{"总使用量(GB)", fmt.Sprintf("%.2f", report.Summary.TotalUsedGB)}); err != nil {
		return err
	}
	if err := writer.Write([]string{"平均使用率(%)", fmt.Sprintf("%.2f", report.Summary.AvgUsagePercent)}); err != nil {
		return err
	}
	if err := writer.Write([]string{"健康评分", fmt.Sprintf("%.1f", report.Summary.HealthScore)}); err != nil {
		return err
	}
	if err := writer.Write([]string{}); err != nil {
		return err
	}

	// 写入明细表头
	if err := writer.Write([]string{"卷名", "总容量(GB)", "已用(GB)", "可用(GB)", "使用率(%)", "评级", "IOPS利用率", "带宽利用率"}); err != nil {
		return err
	}

	for _, vol := range report.VolumeUtilizations {
		if err := writer.Write([]string{
			vol.VolumeName,
			fmt.Sprintf("%.2f", float64(vol.TotalCapacityBytes)/(1024*1024*1024)),
			fmt.Sprintf("%.2f", float64(vol.UsedBytes)/(1024*1024*1024)),
			fmt.Sprintf("%.2f", float64(vol.AvailableBytes)/(1024*1024*1024)),
			fmt.Sprintf("%.2f", vol.UsagePercent),
			vol.Rating,
			fmt.Sprintf("%.2f", vol.IOPSUtilization),
			fmt.Sprintf("%.2f", vol.BandwidthUtilization),
		}); err != nil {
			return err
		}
	}

	return nil
}

// writeRedundantDataScanCSV 写入冗余数据扫描CSV.
func (e *StorageCostReportExporter) writeRedundantDataScanCSV(writer *csv.Writer, report *RedundantDataScanResult) error {
	// 写入标题
	if err := writer.Write([]string{"冗余数据扫描报告"}); err != nil {
		return err
	}
	if err := writer.Write([]string{"扫描ID", report.ID}); err != nil {
		return err
	}
	if err := writer.Write([]string{"扫描时间", report.ScannedAt.Format("2006-01-02 15:04:05")}); err != nil {
		return err
	}
	if err := writer.Write([]string{}); err != nil {
		return err
	}

	// 写入汇总
	if err := writer.Write([]string{"汇总统计"}); err != nil {
		return err
	}
	if err := writer.Write([]string{"总冗余数据(GB)", fmt.Sprintf("%.2f", report.Summary.TotalRedundantGB)}); err != nil {
		return err
	}
	if err := writer.Write([]string{"冗余率(%)", fmt.Sprintf("%.2f", report.Summary.RedundantPercent)}); err != nil {
		return err
	}
	if err := writer.Write([]string{"潜在月节省(元)", fmt.Sprintf("%.2f", report.Summary.PotentialSavingsMonthly)}); err != nil {
		return err
	}
	if err := writer.Write([]string{}); err != nil {
		return err
	}

	// 写入明细表头
	if err := writer.Write([]string{"类型", "路径", "文件名", "大小(字节)", "原因", "安全删除", "风险等级"}); err != nil {
		return err
	}

	for _, item := range report.RedundantItems {
		if err := writer.Write([]string{
			item.Type,
			item.Path,
			item.Name,
			fmt.Sprintf("%d", item.SizeBytes),
			item.Reason,
			fmt.Sprintf("%v", item.SafeToDelete),
			item.DeleteRisk,
		}); err != nil {
			return err
		}
	}

	return nil
}

// writeCostSavingsReportCSV 写入成本节省报告CSV.
func (e *StorageCostReportExporter) writeCostSavingsReportCSV(writer *csv.Writer, report *CostSavingsReport) error {
	// 写入标题
	_ = writer.Write([]string{"成本节省建议报告"})
	_ = writer.Write([]string{"报告ID", report.ID})
	_ = writer.Write([]string{"生成时间", report.GeneratedAt.Format("2006-01-02 15:04:05")})
	_ = writer.Write([]string{})

	// 写入汇总
	_ = writer.Write([]string{"汇总统计"})
	_ = writer.Write([]string{"总节省机会数", fmt.Sprintf("%d", report.Summary.TotalOpportunities)})
	_ = writer.Write([]string{"总月节省(元)", fmt.Sprintf("%.2f", report.Summary.TotalSavingsMonthly)})
	_ = writer.Write([]string{"总年节省(元)", fmt.Sprintf("%.2f", report.Summary.TotalSavingsYearly)})
	_ = writer.Write([]string{"快速见效项目数", fmt.Sprintf("%d", report.Summary.QuickWinCount)})
	_ = writer.Write([]string{})

	// 写入明细表头
	_ = writer.Write([]string{"类型", "标题", "描述", "范围", "目标", "月节省(元)", "年节省(元)", "节省(GB)", "难度", "优先级", "实施时间"})

	for _, opp := range report.SavingsOpportunities {
		_ = writer.Write([]string{
			opp.Type,
			opp.Title,
			opp.Description,
			opp.Scope,
			opp.TargetName,
			fmt.Sprintf("%.2f", opp.SavingsMonthly),
			fmt.Sprintf("%.2f", opp.SavingsYearly),
			fmt.Sprintf("%.2f", opp.SavingsGB),
			opp.Implementation,
			fmt.Sprintf("%d", opp.Priority),
			opp.EstimatedTime,
		})
	}

	return nil
}

// ========== 综合报告生成器 v2.45.0 ==========

// StorageCostReportGenerator 存储成本综合报告生成器.
type StorageCostReportGenerator struct {
	costCalculator      *StorageCostCalculator
	utilizationAnalyzer *StorageUtilizationAnalyzer
	redundantScanner    *RedundantDataScanner
	savingsGenerator    *CostSavingsGenerator
	exporter            *StorageCostReportExporter
}

// NewStorageCostReportGenerator 创建综合报告生成器.
func NewStorageCostReportGenerator(config StorageCostConfig, outputDir string) *StorageCostReportGenerator {
	return &StorageCostReportGenerator{
		costCalculator:      NewStorageCostCalculator(config),
		utilizationAnalyzer: NewStorageUtilizationAnalyzer(config),
		redundantScanner:    NewRedundantDataScanner(config),
		savingsGenerator:    NewCostSavingsGenerator(config),
		exporter:            NewStorageCostReportExporter(outputDir),
	}
}

// ComprehensiveStorageReport 综合存储成本报告.
type ComprehensiveStorageReport struct {
	// 报告ID
	ID string `json:"id"`

	// 报告名称
	Name string `json:"name"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 分析周期
	Period ReportPeriod `json:"period"`

	// 存储成本报告
	CostReport *StorageCostReport `json:"cost_report,omitempty"`

	// 利用率分析
	UtilizationAnalysis *UtilizationAnalysis `json:"utilization_analysis,omitempty"`

	// 冗余数据扫描
	RedundantDataScan *RedundantDataScanResult `json:"redundant_data_scan,omitempty"`

	// 成本节省建议
	SavingsReport *CostSavingsReport `json:"savings_report,omitempty"`

	// 执行摘要
	ExecutiveSummary ExecutiveSummary `json:"executive_summary"`
}

// ExecutiveSummary 执行摘要.
type ExecutiveSummary struct {
	// 总存储容量（GB）
	TotalCapacityGB float64 `json:"total_capacity_gb"`

	// 总使用量（GB）
	TotalUsedGB float64 `json:"total_used_gb"`

	// 平均使用率
	AvgUsagePercent float64 `json:"avg_usage_percent"`

	// 总月成本（元）
	TotalCostMonthly float64 `json:"total_cost_monthly"`

	// 潜在月节省（元）
	PotentialSavingsMonthly float64 `json:"potential_savings_monthly"`

	// 健康评分
	HealthScore float64 `json:"health_score"`

	// 关键发现
	KeyFindings []string `json:"key_findings"`

	// 优先建议
	TopRecommendations []string `json:"top_recommendations"`

	// 卷数量
	VolumeCount int `json:"volume_count"`

	// 告警数量
	AlertCount int `json:"alert_count"`
}

// GenerateComprehensiveReport 生成综合报告.
func (g *StorageCostReportGenerator) GenerateComprehensiveReport(
	metrics []StorageMetrics,
	duplicates []DuplicateFileInfo,
	orphanFiles []OrphanFileInfo,
	tempFiles []TempFileInfo,
	expiredFiles []ExpiredFileInfo,
	oldVersions []OldVersionFileInfo,
	period ReportPeriod,
) *ComprehensiveStorageReport {
	now := time.Now()

	report := &ComprehensiveStorageReport{
		ID:          "comprehensive_" + now.Format("20060102150405"),
		Name:        "存储成本优化综合报告",
		GeneratedAt: now,
		Period:      period,
	}

	// 生成成本报告
	report.CostReport = g.costCalculator.GenerateReport(metrics, period)

	// 生成利用率分析
	report.UtilizationAnalysis = g.utilizationAnalyzer.AnalyzeUtilization(metrics, period)

	// 计算总扫描字节数
	var totalScannedBytes uint64
	for _, m := range metrics {
		totalScannedBytes += m.UsedCapacityBytes
	}

	// 扫描冗余数据
	report.RedundantDataScan = g.redundantScanner.ScanRedundantData(
		duplicates, orphanFiles, tempFiles, expiredFiles, oldVersions, totalScannedBytes,
	)

	// 生成成本节省建议
	report.SavingsReport = g.savingsGenerator.GenerateCostSavingsReport(
		report.UtilizationAnalysis,
		report.RedundantDataScan,
		report.CostReport.VolumeCosts,
		period,
	)

	// 生成执行摘要
	report.ExecutiveSummary = g.generateExecutiveSummary(report)

	return report
}

// generateExecutiveSummary 生成执行摘要.
func (g *StorageCostReportGenerator) generateExecutiveSummary(report *ComprehensiveStorageReport) ExecutiveSummary {
	summary := ExecutiveSummary{
		KeyFindings:        make([]string, 0),
		TopRecommendations: make([]string, 0),
	}

	// 汇总数据
	if report.CostReport != nil {
		summary.TotalCapacityGB = report.CostReport.Summary.TotalCapacityGB
		summary.TotalUsedGB = report.CostReport.Summary.TotalUsedGB
		summary.AvgUsagePercent = report.CostReport.Summary.AvgUsagePercent
		summary.TotalCostMonthly = report.CostReport.Summary.TotalCostMonthly
		summary.VolumeCount = report.CostReport.Summary.VolumeCount
	}

	if report.UtilizationAnalysis != nil {
		summary.HealthScore = report.UtilizationAnalysis.Summary.HealthScore
		summary.AlertCount = len(report.UtilizationAnalysis.Alerts)
	}

	if report.SavingsReport != nil {
		summary.PotentialSavingsMonthly = report.SavingsReport.Summary.TotalSavingsMonthly
	}

	// 关键发现
	if report.UtilizationAnalysis != nil {
		if report.UtilizationAnalysis.Summary.HighUtilizationCount > 0 {
			summary.KeyFindings = append(summary.KeyFindings,
				fmt.Sprintf("发现 %d 个高使用率卷（>80%%）", report.UtilizationAnalysis.Summary.HighUtilizationCount))
		}
		if report.UtilizationAnalysis.Summary.LowUtilizationCount > 0 {
			summary.KeyFindings = append(summary.KeyFindings,
				fmt.Sprintf("发现 %d 个低使用率卷（<30%%），存在资源浪费", report.UtilizationAnalysis.Summary.LowUtilizationCount))
		}
	}

	if report.RedundantDataScan != nil && report.RedundantDataScan.Summary.TotalRedundantGB > 0 {
		summary.KeyFindings = append(summary.KeyFindings,
			fmt.Sprintf("发现 %.2f GB 冗余数据，可节省 %.2f 元/月",
				report.RedundantDataScan.Summary.TotalRedundantGB,
				report.RedundantDataScan.Summary.PotentialSavingsMonthly))
	}

	// 优先建议
	if report.SavingsReport != nil && len(report.SavingsReport.SavingsOpportunities) > 0 {
		for i, opp := range report.SavingsReport.SavingsOpportunities {
			if i >= 3 {
				break
			}
			summary.TopRecommendations = append(summary.TopRecommendations, opp.Title)
		}
	}

	return summary
}

// ExportComprehensiveReport 导出综合报告.
func (g *StorageCostReportGenerator) ExportComprehensiveReport(report *ComprehensiveStorageReport, format string) (string, error) {
	filename := fmt.Sprintf("storage_cost_report_%s.%s", report.ID, format)
	return g.exporter.ExportReport(report, format, filename)
}

// ExportToJSON 导出为JSON.
func (g *StorageCostReportGenerator) ExportToJSON(report *ComprehensiveStorageReport) (string, error) {
	return g.ExportComprehensiveReport(report, "json")
}

// ExportToCSV 导出为CSV.
func (g *StorageCostReportGenerator) ExportToCSV(report *ComprehensiveStorageReport) (string, error) {
	return g.ExportComprehensiveReport(report, "csv")
}

// ========== v2.86.0 增强功能：并发报告生成与缓存 ==========

// CachedReport 缓存的报告.
type CachedReport struct {
	Report      interface{}
	GeneratedAt time.Time
	ExpiresAt   time.Time
}

// ReportCache 报告缓存.
type ReportCache struct {
	mu      sync.RWMutex
	reports map[string]*CachedReport
	ttl     time.Duration
	maxSize int
}

// NewReportCache 创建报告缓存.
func NewReportCache(ttl time.Duration, maxSize int) *ReportCache {
	cache := &ReportCache{
		reports: make(map[string]*CachedReport),
		ttl:     ttl,
		maxSize: maxSize,
	}

	// 启动清理协程
	go cache.cleanupExpired()

	return cache
}

// Get 获取缓存的报告.
func (c *ReportCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, ok := c.reports[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(cached.ExpiresAt) {
		return nil, false
	}

	return cached.Report, true
}

// Set 设置缓存的报告.
func (c *ReportCache) Set(key string, report interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查容量，必要时清理
	if len(c.reports) >= c.maxSize {
		c.evictOldest()
	}

	now := time.Now()
	c.reports[key] = &CachedReport{
		Report:      report,
		GeneratedAt: now,
		ExpiresAt:   now.Add(c.ttl),
	}
}

// Delete 删除缓存的报告.
func (c *ReportCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.reports, key)
}

// Clear 清空缓存.
func (c *ReportCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reports = make(map[string]*CachedReport)
}

// evictOldest 清理最旧的缓存.
func (c *ReportCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for k, v := range c.reports {
		if oldestKey == "" || v.GeneratedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.GeneratedAt
		}
	}

	if oldestKey != "" {
		delete(c.reports, oldestKey)
	}
}

// cleanupExpired 定期清理过期缓存.
func (c *ReportCache) cleanupExpired() {
	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.reports {
			if now.After(v.ExpiresAt) {
				delete(c.reports, k)
			}
		}
		c.mu.Unlock()
	}
}

// ConcurrentReportGenerator 并发报告生成器.
type ConcurrentReportGenerator struct {
	costCalculator      *StorageCostCalculator
	utilizationAnalyzer *StorageUtilizationAnalyzer
	redundantScanner    *RedundantDataScanner
	savingsGenerator    *CostSavingsGenerator
	exporter            *StorageCostReportExporter
	cache               *ReportCache
	workerCount         int
}

// NewConcurrentReportGenerator 创建并发报告生成器.
func NewConcurrentReportGenerator(config StorageCostConfig, outputDir string, workerCount int) *ConcurrentReportGenerator {
	if workerCount <= 0 {
		workerCount = 4
	}

	return &ConcurrentReportGenerator{
		costCalculator:      NewStorageCostCalculator(config),
		utilizationAnalyzer: NewStorageUtilizationAnalyzer(config),
		redundantScanner:    NewRedundantDataScanner(config),
		savingsGenerator:    NewCostSavingsGenerator(config),
		exporter:            NewStorageCostReportExporter(outputDir),
		cache:               NewReportCache(time.Minute*30, 100),
		workerCount:         workerCount,
	}
}

// GenerateReportConcurrent 并发生成报告.
func (g *ConcurrentReportGenerator) GenerateReportConcurrent(
	ctx context.Context,
	metrics []StorageMetrics,
	duplicates []DuplicateFileInfo,
	orphanFiles []OrphanFileInfo,
	tempFiles []TempFileInfo,
	expiredFiles []ExpiredFileInfo,
	oldVersions []OldVersionFileInfo,
	period ReportPeriod,
) *ComprehensiveStorageReport {
	now := time.Now()

	// 检查缓存
	cacheKey := fmt.Sprintf("comprehensive_%s_%d", period.StartTime.Format("20060102"), len(metrics))
	if cached, ok := g.cache.Get(cacheKey); ok {
		if report, ok := cached.(*ComprehensiveStorageReport); ok {
			return report
		}
	}

	report := &ComprehensiveStorageReport{
		ID:          "comprehensive_" + now.Format("20060102150405"),
		Name:        "存储成本优化综合报告",
		GeneratedAt: now,
		Period:      period,
	}

	// 使用 WaitGroup 并发生成各部分报告
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 并发生成成本报告
	wg.Add(1)
	go func() {
		defer wg.Done()
		costReport := g.costCalculator.GenerateReport(metrics, period)
		mu.Lock()
		report.CostReport = costReport
		mu.Unlock()
	}()

	// 并发生成利用率分析
	wg.Add(1)
	go func() {
		defer wg.Done()
		utilAnalysis := g.utilizationAnalyzer.AnalyzeUtilization(metrics, period)
		mu.Lock()
		report.UtilizationAnalysis = utilAnalysis
		mu.Unlock()
	}()

	// 并发生成冗余数据扫描
	wg.Add(1)
	go func() {
		defer wg.Done()

		// 计算总扫描字节数
		var totalScannedBytes uint64
		for _, m := range metrics {
			totalScannedBytes += m.UsedCapacityBytes
		}

		redundantScan := g.redundantScanner.ScanRedundantData(
			duplicates, orphanFiles, tempFiles, expiredFiles, oldVersions, totalScannedBytes,
		)
		mu.Lock()
		report.RedundantDataScan = redundantScan
		mu.Unlock()
	}()

	// 等待基础报告生成完成
	wg.Wait()

	// 生成成本节省建议（依赖前面的结果）
	if report.UtilizationAnalysis != nil && report.CostReport != nil {
		report.SavingsReport = g.savingsGenerator.GenerateCostSavingsReport(
			report.UtilizationAnalysis,
			report.RedundantDataScan,
			report.CostReport.VolumeCosts,
			period,
		)
	}

	// 生成执行摘要
	report.ExecutiveSummary = g.generateExecutiveSummary(report)

	// 缓存结果
	g.cache.Set(cacheKey, report)

	return report
}

// GenerateReportsBatch 批量并发生成报告.
func (g *ConcurrentReportGenerator) GenerateReportsBatch(
	ctx context.Context,
	requests []ReportRequest,
) []*ComprehensiveStorageReport {
	results := make([]*ComprehensiveStorageReport, len(requests))

	// 使用信号量控制并发数
	sem := make(chan struct{}, g.workerCount)
	var wg sync.WaitGroup

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, request ReportRequest) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			// 检查上下文是否已取消
			select {
			case <-ctx.Done():
				return
			default:
			}

			// 生成报告
			report := g.GenerateReportConcurrent(
				ctx,
				request.Metrics,
				request.Duplicates,
				request.OrphanFiles,
				request.TempFiles,
				request.ExpiredFiles,
				request.OldVersions,
				request.Period,
			)
			results[idx] = report
		}(i, req)
	}

	wg.Wait()
	return results
}

// ReportRequest 报告请求.
type ReportRequest struct {
	Metrics      []StorageMetrics
	Duplicates   []DuplicateFileInfo
	OrphanFiles  []OrphanFileInfo
	TempFiles    []TempFileInfo
	ExpiredFiles []ExpiredFileInfo
	OldVersions  []OldVersionFileInfo
	Period       ReportPeriod
}

// GetCacheStats 获取缓存统计.
func (g *ConcurrentReportGenerator) GetCacheStats() map[string]interface{} {
	g.cache.mu.RLock()
	defer g.cache.mu.RUnlock()

	return map[string]interface{}{
		"cache_size":   len(g.cache.reports),
		"max_size":     g.cache.maxSize,
		"ttl_minutes":  g.cache.ttl.Minutes(),
		"worker_count": g.workerCount,
	}
}

// ClearCache 清空缓存.
func (g *ConcurrentReportGenerator) ClearCache() {
	g.cache.Clear()
}

// generateExecutiveSummary 生成执行摘要.
func (g *ConcurrentReportGenerator) generateExecutiveSummary(report *ComprehensiveStorageReport) ExecutiveSummary {
	summary := ExecutiveSummary{
		KeyFindings:        make([]string, 0),
		TopRecommendations: make([]string, 0),
	}

	// 汇总数据
	if report.CostReport != nil {
		summary.TotalCapacityGB = report.CostReport.Summary.TotalCapacityGB
		summary.TotalUsedGB = report.CostReport.Summary.TotalUsedGB
		summary.AvgUsagePercent = report.CostReport.Summary.AvgUsagePercent
		summary.TotalCostMonthly = report.CostReport.Summary.TotalCostMonthly
		summary.VolumeCount = report.CostReport.Summary.VolumeCount
	}

	if report.UtilizationAnalysis != nil {
		summary.HealthScore = report.UtilizationAnalysis.Summary.HealthScore
		summary.AlertCount = len(report.UtilizationAnalysis.Alerts)
	}

	if report.SavingsReport != nil {
		summary.PotentialSavingsMonthly = report.SavingsReport.Summary.TotalSavingsMonthly
	}

	// 关键发现
	if report.UtilizationAnalysis != nil {
		if report.UtilizationAnalysis.Summary.HighUtilizationCount > 0 {
			summary.KeyFindings = append(summary.KeyFindings,
				fmt.Sprintf("发现 %d 个高使用率卷（>80%%）", report.UtilizationAnalysis.Summary.HighUtilizationCount))
		}
		if report.UtilizationAnalysis.Summary.LowUtilizationCount > 0 {
			summary.KeyFindings = append(summary.KeyFindings,
				fmt.Sprintf("发现 %d 个低使用率卷（<30%%），存在资源浪费", report.UtilizationAnalysis.Summary.LowUtilizationCount))
		}
	}

	if report.RedundantDataScan != nil && report.RedundantDataScan.Summary.TotalRedundantGB > 0 {
		summary.KeyFindings = append(summary.KeyFindings,
			fmt.Sprintf("发现 %.2f GB 冗余数据，可节省 %.2f 元/月",
				report.RedundantDataScan.Summary.TotalRedundantGB,
				report.RedundantDataScan.Summary.PotentialSavingsMonthly))
	}

	// 优先建议
	if report.SavingsReport != nil && len(report.SavingsReport.SavingsOpportunities) > 0 {
		for i, opp := range report.SavingsReport.SavingsOpportunities {
			if i >= 3 {
				break
			}
			summary.TopRecommendations = append(summary.TopRecommendations, opp.Title)
		}
	}

	return summary
}
