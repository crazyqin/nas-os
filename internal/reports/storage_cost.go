// Package reports 提供报表生成和管理功能
package reports

import (
	"math"
	"time"
)

// ========== 存储成本计算 ==========

// StorageCostConfig 存储成本配置
type StorageCostConfig struct {
	// 容量成本（元/GB/月）
	CostPerGBMonthly float64 `json:"cost_per_gb_monthly"`

	// IOPS成本（元/1000 IOPS/月）
	CostPerIOPSMonthly float64 `json:"cost_per_iops_monthly"`

	// 带宽成本（元/Mbps/月）
	CostPerBandwidthMonthly float64 `json:"cost_per_bandwidth_monthly"`

	// 电费成本（元/kWh）
	ElectricityCostPerKWh float64 `json:"electricity_cost_per_kwh"`

	// 设备功耗（瓦）
	DevicePowerWatts float64 `json:"device_power_watts"`

	// 运维人力成本（元/月）
	OpsCostMonthly float64 `json:"ops_cost_monthly"`

	// 折旧年限（年）
	DepreciationYears int `json:"depreciation_years"`

	// 设备采购成本（元）
	HardwareCost float64 `json:"hardware_cost"`
}

// StorageMetrics 存储指标
type StorageMetrics struct {
	// 卷名
	VolumeName string `json:"volume_name"`

	// 总容量（字节）
	TotalCapacityBytes uint64 `json:"total_capacity_bytes"`

	// 已使用容量（字节）
	UsedCapacityBytes uint64 `json:"used_capacity_bytes"`

	// 可用容量（字节）
	AvailableCapacityBytes uint64 `json:"available_capacity_bytes"`

	// IOPS（读写总和）
	IOPS uint64 `json:"iops"`

	// 读带宽（字节/秒）
	ReadBandwidthBytes uint64 `json:"read_bandwidth_bytes"`

	// 写带宽（字节/秒）
	WriteBandwidthBytes uint64 `json:"write_bandwidth_bytes"`

	// 文件数量
	FileCount uint64 `json:"file_count"`

	// 目录数量
	DirCount uint64 `json:"dir_count"`

	// 采集时间
	Timestamp time.Time `json:"timestamp"`
}

// StorageCostResult 存储成本计算结果
type StorageCostResult struct {
	// 卷名
	VolumeName string `json:"volume_name"`

	// 容量成本（元/月）
	CapacityCostMonthly float64 `json:"capacity_cost_monthly"`

	// IOPS成本（元/月）
	IOPSCostMonthly float64 `json:"iops_cost_monthly"`

	// 带宽成本（元/月）
	BandwidthCostMonthly float64 `json:"bandwidth_cost_monthly"`

	// 电费成本（元/月）
	ElectricityCostMonthly float64 `json:"electricity_cost_monthly"`

	// 运维成本（元/月）
	OpsCostMonthly float64 `json:"ops_cost_monthly"`

	// 折旧成本（元/月）
	DepreciationCostMonthly float64 `json:"depreciation_cost_monthly"`

	// 总成本（元/月）
	TotalCostMonthly float64 `json:"total_cost_monthly"`

	// 单位成本（元/GB/月）
	CostPerGBMonthly float64 `json:"cost_per_gb_monthly"`

	// 使用率
	UsagePercent float64 `json:"usage_percent"`

	// 计算时间
	CalculatedAt time.Time `json:"calculated_at"`
}

// StorageCostReport 存储成本报表
type StorageCostReport struct {
	// 报表ID
	ID string `json:"id"`

	// 报表名称
	Name string `json:"name"`

	// 报告时间范围
	Period ReportPeriod `json:"period"`

	// 配置
	Config StorageCostConfig `json:"config"`

	// 各卷成本明细
	VolumeCosts []StorageCostResult `json:"volume_costs"`

	// 汇总
	Summary StorageCostSummary `json:"summary"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`
}

// StorageCostSummary 存储成本汇总
type StorageCostSummary struct {
	// 总容量成本
	TotalCapacityCostMonthly float64 `json:"total_capacity_cost_monthly"`

	// 总IOPS成本
	TotalIOPSCostMonthly float64 `json:"total_iops_cost_monthly"`

	// 总带宽成本
	TotalBandwidthCostMonthly float64 `json:"total_bandwidth_cost_monthly"`

	// 总电费成本
	TotalElectricityCostMonthly float64 `json:"total_electricity_cost_monthly"`

	// 总运维成本
	TotalOpsCostMonthly float64 `json:"total_ops_cost_monthly"`

	// 总折旧成本
	TotalDepreciationCostMonthly float64 `json:"total_depreciation_cost_monthly"`

	// 总成本
	TotalCostMonthly float64 `json:"total_cost_monthly"`

	// 平均单位成本
	AvgCostPerGBMonthly float64 `json:"avg_cost_per_gb_monthly"`

	// 总容量（GB）
	TotalCapacityGB float64 `json:"total_capacity_gb"`

	// 总使用量（GB）
	TotalUsedGB float64 `json:"total_used_gb"`

	// 平均使用率
	AvgUsagePercent float64 `json:"avg_usage_percent"`

	// 卷数量
	VolumeCount int `json:"volume_count"`
}

// StorageCostCalculator 存储成本计算器
type StorageCostCalculator struct {
	config StorageCostConfig
}

// NewStorageCostCalculator 创建存储成本计算器
func NewStorageCostCalculator(config StorageCostConfig) *StorageCostCalculator {
	return &StorageCostCalculator{config: config}
}

// Calculate 计算单个存储的成本
func (c *StorageCostCalculator) Calculate(metrics StorageMetrics) StorageCostResult {
	// 转换为GB
	totalGB := float64(metrics.TotalCapacityBytes) / (1024 * 1024 * 1024)

	// 计算容量成本
	capacityCost := totalGB * c.config.CostPerGBMonthly

	// 计算IOPS成本（每1000 IOPS）
	iopsCost := float64(metrics.IOPS) / 1000.0 * c.config.CostPerIOPSMonthly

	// 计算带宽成本（转换为Mbps）
	totalBandwidthMbps := float64(metrics.ReadBandwidthBytes+metrics.WriteBandwidthBytes) * 8 / (1024 * 1024)
	bandwidthCost := totalBandwidthMbps * c.config.CostPerBandwidthMonthly

	// 计算电费成本（24小时/天，30天/月）
	hoursPerMonth := 24.0 * 30
	electricityCost := c.config.DevicePowerWatts / 1000.0 * hoursPerMonth * c.config.ElectricityCostPerKWh

	// 计算折旧成本
	monthsPerYear := 12.0
	depreciationMonths := float64(c.config.DepreciationYears) * monthsPerYear
	depreciationCost := c.config.HardwareCost / depreciationMonths

	// 计算总成本
	totalCost := capacityCost + iopsCost + bandwidthCost + electricityCost + c.config.OpsCostMonthly + depreciationCost

	// 计算单位成本
	costPerGB := 0.0
	if totalGB > 0 {
		costPerGB = totalCost / totalGB
	}

	// 计算使用率
	usagePercent := 0.0
	if metrics.TotalCapacityBytes > 0 {
		usagePercent = float64(metrics.UsedCapacityBytes) / float64(metrics.TotalCapacityBytes) * 100
	}

	return StorageCostResult{
		VolumeName:              metrics.VolumeName,
		CapacityCostMonthly:     round(capacityCost, 2),
		IOPSCostMonthly:         round(iopsCost, 2),
		BandwidthCostMonthly:    round(bandwidthCost, 2),
		ElectricityCostMonthly:  round(electricityCost, 2),
		OpsCostMonthly:          round(c.config.OpsCostMonthly, 2),
		DepreciationCostMonthly: round(depreciationCost, 2),
		TotalCostMonthly:        round(totalCost, 2),
		CostPerGBMonthly:        round(costPerGB, 2),
		UsagePercent:            round(usagePercent, 2),
		CalculatedAt:            time.Now(),
	}
}

// CalculateAll 计算所有存储的成本
func (c *StorageCostCalculator) CalculateAll(metrics []StorageMetrics) []StorageCostResult {
	results := make([]StorageCostResult, 0, len(metrics))
	for _, m := range metrics {
		results = append(results, c.Calculate(m))
	}
	return results
}

// GenerateReport 生成存储成本报表
func (c *StorageCostCalculator) GenerateReport(metrics []StorageMetrics, period ReportPeriod) *StorageCostReport {
	volumeCosts := c.CalculateAll(metrics)
	summary := c.calculateSummary(volumeCosts, metrics)

	return &StorageCostReport{
		ID:          generateReportID(),
		Name:        "存储成本报表",
		Period:      period,
		Config:      c.config,
		VolumeCosts: volumeCosts,
		Summary:     summary,
		GeneratedAt: time.Now(),
	}
}

// calculateSummary 计算汇总
func (c *StorageCostCalculator) calculateSummary(costs []StorageCostResult, metrics []StorageMetrics) StorageCostSummary {
	summary := StorageCostSummary{
		VolumeCount: len(costs),
	}

	var totalCapacityGB, totalUsedGB float64

	for _, cost := range costs {
		summary.TotalCapacityCostMonthly += cost.CapacityCostMonthly
		summary.TotalIOPSCostMonthly += cost.IOPSCostMonthly
		summary.TotalBandwidthCostMonthly += cost.BandwidthCostMonthly
		summary.TotalElectricityCostMonthly += cost.ElectricityCostMonthly
		summary.TotalOpsCostMonthly += cost.OpsCostMonthly
		summary.TotalDepreciationCostMonthly += cost.DepreciationCostMonthly
		summary.TotalCostMonthly += cost.TotalCostMonthly
		summary.AvgUsagePercent += cost.UsagePercent
	}

	for _, m := range metrics {
		totalCapacityGB += float64(m.TotalCapacityBytes) / (1024 * 1024 * 1024)
		totalUsedGB += float64(m.UsedCapacityBytes) / (1024 * 1024 * 1024)
	}

	summary.TotalCapacityGB = round(totalCapacityGB, 2)
	summary.TotalUsedGB = round(totalUsedGB, 2)

	if summary.VolumeCount > 0 {
		summary.AvgUsagePercent = round(summary.AvgUsagePercent/float64(summary.VolumeCount), 2)
	}

	if summary.TotalCapacityGB > 0 {
		summary.AvgCostPerGBMonthly = round(summary.TotalCostMonthly/summary.TotalCapacityGB, 2)
	}

	// 四舍五入
	summary.TotalCapacityCostMonthly = round(summary.TotalCapacityCostMonthly, 2)
	summary.TotalIOPSCostMonthly = round(summary.TotalIOPSCostMonthly, 2)
	summary.TotalBandwidthCostMonthly = round(summary.TotalBandwidthCostMonthly, 2)
	summary.TotalElectricityCostMonthly = round(summary.TotalElectricityCostMonthly, 2)
	summary.TotalOpsCostMonthly = round(summary.TotalOpsCostMonthly, 2)
	summary.TotalDepreciationCostMonthly = round(summary.TotalDepreciationCostMonthly, 2)
	summary.TotalCostMonthly = round(summary.TotalCostMonthly, 2)

	return summary
}

// UpdateConfig 更新配置
func (c *StorageCostCalculator) UpdateConfig(config StorageCostConfig) {
	c.config = config
}

// GetConfig 获取配置
func (c *StorageCostCalculator) GetConfig() StorageCostConfig {
	return c.config
}

// ========== 成本趋势分析 ==========

// CostTrendPoint 成本趋势点
type CostTrendPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	TotalCostMonthly float64   `json:"total_cost_monthly"`
	CapacityCost     float64   `json:"capacity_cost"`
	UsagePercent     float64   `json:"usage_percent"`
	CostPerGB        float64   `json:"cost_per_gb"`
}

// CostTrendReport 成本趋势报表
type CostTrendReport struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Period      ReportPeriod     `json:"period"`
	TrendPoints []CostTrendPoint `json:"trend_points"`
	Summary     TrendSummary     `json:"summary"`
	GeneratedAt time.Time        `json:"generated_at"`
}

// TrendSummary 趋势汇总
type TrendSummary struct {
	// 平均月成本
	AvgMonthlyCost float64 `json:"avg_monthly_cost"`

	// 成本增长率（%）
	CostGrowthRate float64 `json:"cost_growth_rate"`

	// 最高月成本
	MaxMonthlyCost float64 `json:"max_monthly_cost"`

	// 最低月成本
	MinMonthlyCost float64 `json:"min_month_cost"`

	// 预测下月成本
	NextMonthCostForecast float64 `json:"next_month_cost_forecast"`
}

// AnalyzeTrend 分析成本趋势
func (c *StorageCostCalculator) AnalyzeTrend(history []CostTrendPoint) *CostTrendReport {
	if len(history) < 2 {
		return nil
	}

	summary := TrendSummary{}
	var totalCost float64

	// 找出最大最小值
	summary.MaxMonthlyCost = history[0].TotalCostMonthly
	summary.MinMonthlyCost = history[0].TotalCostMonthly

	for _, point := range history {
		totalCost += point.TotalCostMonthly
		if point.TotalCostMonthly > summary.MaxMonthlyCost {
			summary.MaxMonthlyCost = point.TotalCostMonthly
		}
		if point.TotalCostMonthly < summary.MinMonthlyCost {
			summary.MinMonthlyCost = point.TotalCostMonthly
		}
	}

	summary.AvgMonthlyCost = round(totalCost/float64(len(history)), 2)

	// 计算增长率
	if len(history) >= 2 && history[0].TotalCostMonthly > 0 {
		first := history[0].TotalCostMonthly
		last := history[len(history)-1].TotalCostMonthly
		summary.CostGrowthRate = round((last-first)/first*100, 2)
	}

	// 简单线性预测
	summary.NextMonthCostForecast = c.forecastNextMonth(history)

	return &CostTrendReport{
		ID:          generateReportID(),
		Name:        "成本趋势分析",
		TrendPoints: history,
		Summary:     summary,
		GeneratedAt: time.Now(),
	}
}

// forecastNextMonth 预测下月成本（简单线性回归）
func (c *StorageCostCalculator) forecastNextMonth(history []CostTrendPoint) float64 {
	if len(history) < 2 {
		return 0
	}

	n := float64(len(history))
	var sumX, sumY, sumXY, sumX2 float64

	for i, point := range history {
		x := float64(i)
		y := point.TotalCostMonthly
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// 线性回归 y = a + bx
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return history[len(history)-1].TotalCostMonthly
	}

	b := (n*sumXY - sumX*sumY) / denominator
	a := (sumY - b*sumX) / n

	// 预测下一个点
	forecast := a + b*float64(len(history))

	// 确保不为负
	if forecast < 0 {
		return history[len(history)-1].TotalCostMonthly
	}

	return round(forecast, 2)
}

// ========== 辅助函数 ==========

func round(val float64, precision int) float64 {
	pow := math.Pow10(precision)
	return math.Round(val*pow) / pow
}

func generateReportID() string {
	return "cost_" + time.Now().Format("20060102150405")
}
