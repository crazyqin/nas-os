// Package cost 提供成本分析和管理功能
package cost

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// ========== 基础类型定义 ==========

// CostType 成本类型
type CostType string

const (
	// CostTypeStorage 表示存储成本类型.
	CostTypeStorage CostType = "storage"
	// CostTypeElectricity 表示电力成本类型.
	CostTypeElectricity CostType = "electricity"
	// CostTypeNetwork 表示网络成本类型.
	CostTypeNetwork CostType = "network"
	// CostTypeOperations 表示运维成本类型.
	CostTypeOperations CostType = "operations"
	// CostTypeDepreciation 表示折旧成本类型.
	CostTypeDepreciation CostType = "depreciation"
)

// CostItem 单项成本条目
type CostItem struct {
	// 成本ID
	ID string `json:"id"`

	// 成本类型
	Type CostType `json:"type"`

	// 成本名称
	Name string `json:"name"`

	// 成本描述
	Description string `json:"description"`

	// 关联资源名称（如卷名、设备名）
	ResourceName string `json:"resource_name"`

	// 成本金额（元）
	Amount float64 `json:"amount"`

	// 成本单位（元/GB/月、元/kWh、元/月）
	Unit string `json:"unit"`

	// 计算依据（如容量大小、功率等）
	Basis float64 `json:"basis"`

	// 单价
	UnitPrice float64 `json:"unit_price"`

	// 成本占比
	Percent float64 `json:"percent"`

	// 计算时间
	CalculatedAt time.Time `json:"calculated_at"`

	// 时间周期（monthly/daily/hourly）
	Period string `json:"period"`
}

// CostSummary 成本汇总
type CostSummary struct {
	// 汇总ID
	ID string `json:"id"`

	// 汇总时间
	SummarizedAt time.Time `json:"summarized_at"`

	// 总成本（元/月）
	TotalCostMonthly float64 `json:"total_cost_monthly"`

	// 总成本（元/年）
	TotalCostYearly float64 `json:"total_cost_yearly"`

	// 按类型汇总
	CostByType map[CostType]float64 `json:"cost_by_type"`

	// 成本明细列表
	CostItems []CostItem `json:"cost_items"`

	// 资源数量
	ResourceCount int `json:"resource_count"`

	// 平均单位成本（元/GB）
	AvgCostPerGB float64 `json:"avg_cost_per_gb"`

	// 成本效率评分（0-100）
	EfficiencyScore float64 `json:"efficiency_score"`

	// 潜在节省空间（元/月）
	PotentialSavings float64 `json:"potential_savings"`

	// 预算使用比例（%）
	BudgetUsagePercent float64 `json:"budget_usage_percent"`

	// 预算余额
	BudgetRemaining float64 `json:"budget_remaining"`
}

// TrendData 成本趋势数据点
type TrendData struct {
	// 时间戳
	Timestamp time.Time `json:"timestamp"`

	// 总成本
	TotalCost float64 `json:"total_cost"`

	// 存储成本
	StorageCost float64 `json:"storage_cost"`

	// 电费成本
	ElectricityCost float64 `json:"electricity_cost"`

	// 网络成本
	NetworkCost float64 `json:"network_cost"`

	// 运维成本
	OperationsCost float64 `json:"operations_cost"`

	// 折旧成本
	DepreciationCost float64 `json:"depreciation_cost"`

	// 使用量（GB）
	UsedGB float64 `json:"used_gb"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 单位成本（元/GB）
	CostPerGB float64 `json:"cost_per_gb"`

	// 趋势方向（up/down/stable）
	Trend string `json:"trend"`

	// 变化率（%）
	ChangeRate float64 `json:"change_rate"`
}

// TrendAnalysisResult 趋势分析结果
type TrendAnalysisResult struct {
	// 分析ID
	ID string `json:"id"`

	// 分析时间
	AnalyzedAt time.Time `json:"analyzed_at"`

	// 时间范围
	TimeRange TimeRange `json:"time_range"`

	// 趋势数据点列表
	DataPoints []TrendData `json:"data_points"`

	// 趋势统计
	Statistics TrendStatistics `json:"statistics"`

	// 趋势预测
	Forecast TrendForecast `json:"forecast"`
}

// TimeRange 时间范围
type TimeRange struct {
	// 开始时间
	StartTime time.Time `json:"start_time"`

	// 结束时间
	EndTime time.Time `json:"end_time"`

	// 时间跨度（day/week/month/year）
	Granularity string `json:"granularity"`
}

// TrendStatistics 趋势统计
type TrendStatistics struct {
	// 平均成本
	AvgCost float64 `json:"avg_cost"`

	// 最大成本
	MaxCost float64 `json:"max_cost"`

	// 最小成本
	MinCost float64 `json:"min_cost"`

	// 成本标准差
	CostStdDev float64 `json:"cost_std_dev"`

	// 成本增长率（%）
	GrowthRate float64 `json:"growth_rate"`

	// 趋势方向（increasing/decreasing/stable）
	TrendDirection string `json:"trend_direction"`

	// 波动系数
	VolatilityCoeff float64 `json:"volatility_coeff"`

	// 预测置信度
	Confidence float64 `json:"confidence"`
}

// TrendForecast 趋势预测
type TrendForecast struct {
	// 预测下月成本
	NextMonthCost float64 `json:"next_month_cost"`

	// 预测下季度成本
	NextQuarterCost float64 `json:"next_quarter_cost"`

	// 预测下年度成本
	NextYearCost float64 `json:"next_year_cost"`

	// 预测准确度（%）
	Accuracy float64 `json:"accuracy"`

	// 预测模型类型
	ModelType string `json:"model_type"`

	// 预测数据点
	ForecastPoints []TrendData `json:"forecast_points"`
}

// ========== 配置定义 ==========

// DashboardConfig 成本分析配置
type DashboardConfig struct {
	// 存储单价（元/GB/月）
	StorageCostPerGB float64 `json:"storage_cost_per_gb"`

	// 电价（元/kWh）
	ElectricityCostPerKWh float64 `json:"electricity_cost_per_kwh"`

	// 默认设备功率（瓦）
	DefaultDevicePowerWatts float64 `json:"default_device_power_watts"`

	// 网络带宽单价（元/Mbps/月）
	NetworkCostPerMbps float64 `json:"network_cost_per_mbps"`

	// 月度运维成本
	OpsCostMonthly float64 `json:"ops_cost_monthly"`

	// 硬件总成本
	HardwareCost float64 `json:"hardware_cost"`

	// 折旧年限
	DepreciationYears int `json:"depreciation_years"`

	// 月度预算上限
	BudgetLimitMonthly float64 `json:"budget_limit_monthly"`

	// 货币单位
	Currency string `json:"currency"`

	// 低使用率阈值（%）
	LowUsageThreshold float64 `json:"low_usage_threshold"`

	// 高使用率阈值（%）
	HighUsageThreshold float64 `json:"high_usage_threshold"`

	// 趋势数据保留天数
	TrendRetentionDays int `json:"trend_retention_days"`
}

// ResourceInfo 资源信息
type ResourceInfo struct {
	// 资源名称
	Name string `json:"name"`

	// 资源类型
	Type string `json:"type"`

	// 总容量（字节）
	TotalCapacityBytes uint64 `json:"total_capacity_bytes"`

	// 已用容量（字节）
	UsedCapacityBytes uint64 `json:"used_capacity_bytes"`

	// IOPS
	IOPS uint64 `json:"iops"`

	// 读带宽（字节/秒）
	ReadBandwidthBytes uint64 `json:"read_bandwidth_bytes"`

	// 写带宽（字节/秒）
	WriteBandwidthBytes uint64 `json:"write_bandwidth_bytes"`

	// 功率（瓦）
	PowerWatts float64 `json:"power_watts"`

	// 硬件成本
	HardwareCost float64 `json:"hardware_cost"`
}

// ========== 成本分析服务 ==========

// DashboardService 成本分析服务
type DashboardService struct {
	config    DashboardConfig
	trendData []TrendData
	mu        sync.RWMutex
}

// NewDashboardService 创建成本分析服务
func NewDashboardService(config DashboardConfig) *DashboardService {
	return &DashboardService{
		config:    config,
		trendData: make([]TrendData, 0),
	}
}

// CalculateStorageCost 计算存储成本（容量 × 单价）
func (s *DashboardService) CalculateStorageCost(resource ResourceInfo) CostItem {
	now := time.Now()

	totalGB := float64(resource.TotalCapacityBytes) / (1024 * 1024 * 1024)
	usedGB := float64(resource.UsedCapacityBytes) / (1024 * 1024 * 1024)

	// 存储成本按实际使用量计算
	amount := usedGB * s.config.StorageCostPerGB

	return CostItem{
		ID:           fmt.Sprintf("storage_%s_%d", resource.Name, now.Unix()),
		Type:         CostTypeStorage,
		Name:         fmt.Sprintf("%s 存储成本", resource.Name),
		Description:  fmt.Sprintf("容量 %.2f GB，使用 %.2f GB", totalGB, usedGB),
		ResourceName: resource.Name,
		Amount:       round(amount, 2),
		Unit:         "元/月",
		Basis:        round(usedGB, 2),
		UnitPrice:    s.config.StorageCostPerGB,
		CalculatedAt: now,
		Period:       "monthly",
	}
}

// CalculateElectricityCost 计算电费成本（功率 × 时间 × 电价）
func (s *DashboardService) CalculateElectricityCost(resource ResourceInfo) CostItem {
	now := time.Now()

	// 使用设备功率或默认功率
	powerWatts := resource.PowerWatts
	if powerWatts == 0 {
		powerWatts = s.config.DefaultDevicePowerWatts
	}

	// 月度电费 = 功率(kW) × 24小时 × 30天 × 电价
	powerKW := powerWatts / 1000.0
	hoursPerMonth := 24.0 * 30.0
	amount := powerKW * hoursPerMonth * s.config.ElectricityCostPerKWh

	return CostItem{
		ID:           fmt.Sprintf("elec_%s_%d", resource.Name, now.Unix()),
		Type:         CostTypeElectricity,
		Name:         fmt.Sprintf("%s 电费成本", resource.Name),
		Description:  fmt.Sprintf("功率 %.1f W，月耗电 %.2f kWh", powerWatts, powerKW*hoursPerMonth),
		ResourceName: resource.Name,
		Amount:       round(amount, 2),
		Unit:         "元/月",
		Basis:        round(powerKW*hoursPerMonth, 2),
		UnitPrice:    s.config.ElectricityCostPerKWh,
		CalculatedAt: now,
		Period:       "monthly",
	}
}

// CalculateAllCosts 计算所有成本项
func (s *DashboardService) CalculateAllCosts(resources []ResourceInfo) []CostItem {
	items := make([]CostItem, 0)

	for _, r := range resources {
		// 存储成本
		items = append(items, s.CalculateStorageCost(r))

		// 电费成本
		items = append(items, s.CalculateElectricityCost(r))
	}

	// 运维成本（按资源数量分摊）
	if s.config.OpsCostMonthly > 0 && len(resources) > 0 {
		opsPerResource := s.config.OpsCostMonthly / float64(len(resources))
		for _, r := range resources {
			items = append(items, CostItem{
				ID:           fmt.Sprintf("ops_%s_%d", r.Name, time.Now().Unix()),
				Type:         CostTypeOperations,
				Name:         fmt.Sprintf("%s 运维成本", r.Name),
				Description:  "月度运维成本分摊",
				ResourceName: r.Name,
				Amount:       round(opsPerResource, 2),
				Unit:         "元/月",
				Basis:        1,
				UnitPrice:    opsPerResource,
				CalculatedAt: time.Now(),
				Period:       "monthly",
			})
		}
	}

	// 折旧成本
	if s.config.HardwareCost > 0 && s.config.DepreciationYears > 0 {
		depreciationMonthly := s.config.HardwareCost / float64(s.config.DepreciationYears) / 12.0
		for _, r := range resources {
			// 按容量比例分摊
			totalGB := float64(r.TotalCapacityBytes) / (1024 * 1024 * 1024)
			allTotalGB := s.getTotalCapacityGB(resources)
			ratio := 1.0 / float64(len(resources))
			if allTotalGB > 0 && totalGB > 0 {
				ratio = totalGB / allTotalGB
			}
			amount := depreciationMonthly * ratio

			items = append(items, CostItem{
				ID:           fmt.Sprintf("deprec_%s_%d", r.Name, time.Now().Unix()),
				Type:         CostTypeDepreciation,
				Name:         fmt.Sprintf("%s 折旧成本", r.Name),
				Description:  fmt.Sprintf("硬件折旧 %d 年分摊", s.config.DepreciationYears),
				ResourceName: r.Name,
				Amount:       round(amount, 2),
				Unit:         "元/月",
				Basis:        ratio,
				UnitPrice:    depreciationMonthly,
				CalculatedAt: time.Now(),
				Period:       "monthly",
			})
		}
	}

	return items
}

// GenerateCostSummary 生成成本汇总
func (s *DashboardService) GenerateCostSummary(resources []ResourceInfo) *CostSummary {
	now := time.Now()

	items := s.CalculateAllCosts(resources)

	summary := &CostSummary{
		ID:            fmt.Sprintf("summary_%d", now.Unix()),
		SummarizedAt:  now,
		CostItems:     items,
		CostByType:    make(map[CostType]float64),
		ResourceCount: len(resources),
	}

	// 计算总成本和按类型汇总
	for _, item := range items {
		summary.TotalCostMonthly += item.Amount
		summary.CostByType[item.Type] += item.Amount
	}

	summary.TotalCostYearly = round(summary.TotalCostMonthly*12, 2)
	summary.TotalCostMonthly = round(summary.TotalCostMonthly, 2)

	// 计算平均单位成本
	totalUsedGB := s.getTotalUsedGB(resources)
	if totalUsedGB > 0 {
		summary.AvgCostPerGB = round(summary.TotalCostMonthly/totalUsedGB, 4)
	}

	// 计算成本效率评分
	summary.EfficiencyScore = s.calculateEfficiencyScore(resources, summary.TotalCostMonthly)

	// 计算潜在节省
	summary.PotentialSavings = s.calculatePotentialSavings(resources)

	// 计算预算使用情况
	if s.config.BudgetLimitMonthly > 0 {
		summary.BudgetUsagePercent = round(summary.TotalCostMonthly/s.config.BudgetLimitMonthly*100, 2)
		summary.BudgetRemaining = round(s.config.BudgetLimitMonthly-summary.TotalCostMonthly, 2)
	}

	// 计算各项成本占比
	for i := range summary.CostItems {
		if summary.TotalCostMonthly > 0 {
			summary.CostItems[i].Percent = round(summary.CostItems[i].Amount/summary.TotalCostMonthly*100, 2)
		}
	}

	return summary
}

// AnalyzeTrend 成本趋势分析接口
func (s *DashboardService) AnalyzeTrend(ctx context.Context, resources []ResourceInfo, timeRange TimeRange) (*TrendAnalysisResult, error) {
	now := time.Now()

	result := &TrendAnalysisResult{
		ID:         fmt.Sprintf("trend_%d", now.Unix()),
		AnalyzedAt: now,
		TimeRange:  timeRange,
	}

	// 获取历史趋势数据
	s.mu.RLock()
	historicalData := s.getTrendDataInRange(timeRange)
	s.mu.RUnlock()

	// 计算当前成本点
	currentCost := s.GenerateCostSummary(resources)

	// 构建趋势数据点
	dataPoints := s.buildTrendDataPoints(historicalData, currentCost, resources)
	result.DataPoints = dataPoints

	// 计算统计指标
	result.Statistics = s.calculateTrendStatistics(dataPoints)

	// 生成预测
	result.Forecast = s.generateForecast(dataPoints)

	return result, nil
}

// RecordTrendPoint 记录趋势数据点
func (s *DashboardService) RecordTrendPoint(resources []ResourceInfo) {
	now := time.Now()

	summary := s.GenerateCostSummary(resources)

	point := TrendData{
		Timestamp:        now,
		TotalCost:        summary.TotalCostMonthly,
		StorageCost:      summary.CostByType[CostTypeStorage],
		ElectricityCost:  summary.CostByType[CostTypeElectricity],
		NetworkCost:      summary.CostByType[CostTypeNetwork],
		OperationsCost:   summary.CostByType[CostTypeOperations],
		DepreciationCost: summary.CostByType[CostTypeDepreciation],
		UsedGB:           s.getTotalUsedGB(resources),
		UsagePercent:     s.getAvgUsagePercent(resources),
		CostPerGB:        summary.AvgCostPerGB,
	}

	// 计算趋势方向和变化率
	s.mu.Lock()
	if len(s.trendData) > 0 {
		lastPoint := s.trendData[len(s.trendData)-1]
		if point.TotalCost > lastPoint.TotalCost {
			point.Trend = "up"
		} else if point.TotalCost < lastPoint.TotalCost {
			point.Trend = "down"
		} else {
			point.Trend = "stable"
		}

		if lastPoint.TotalCost > 0 {
			point.ChangeRate = round((point.TotalCost-lastPoint.TotalCost)/lastPoint.TotalCost*100, 2)
		}
	} else {
		point.Trend = "stable"
		point.ChangeRate = 0
	}

	s.trendData = append(s.trendData, point)

	// 清理过期数据
	s.cleanupOldTrendData()
	s.mu.Unlock()
}

// GetTrendData 获取趋势数据
func (s *DashboardService) GetTrendData() []TrendData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.trendData
}

// ========== 辅助方法 ==========

// getTotalCapacityGB 获取总容量（GB）
func (s *DashboardService) getTotalCapacityGB(resources []ResourceInfo) float64 {
	var total float64
	for _, r := range resources {
		total += float64(r.TotalCapacityBytes) / (1024 * 1024 * 1024)
	}
	return total
}

// getTotalUsedGB 获取总使用量（GB）
func (s *DashboardService) getTotalUsedGB(resources []ResourceInfo) float64 {
	var total float64
	for _, r := range resources {
		total += float64(r.UsedCapacityBytes) / (1024 * 1024 * 1024)
	}
	return total
}

// getAvgUsagePercent 获取平均使用率
func (s *DashboardService) getAvgUsagePercent(resources []ResourceInfo) float64 {
	if len(resources) == 0 {
		return 0
	}

	var totalUsage float64
	for _, r := range resources {
		if r.TotalCapacityBytes > 0 {
			usage := float64(r.UsedCapacityBytes) / float64(r.TotalCapacityBytes) * 100
			totalUsage += usage
		}
	}

	return round(totalUsage/float64(len(resources)), 2)
}

// calculateEfficiencyScore 计算成本效率评分
func (s *DashboardService) calculateEfficiencyScore(resources []ResourceInfo, totalCost float64) float64 {
	score := 100.0

	avgUsage := s.getAvgUsagePercent(resources)

	// 低使用率扣分（资源浪费）
	if avgUsage < s.config.LowUsageThreshold {
		score -= (s.config.LowUsageThreshold - avgUsage) * 0.5
	}

	// 高使用率扣分（风险）
	if avgUsage > s.config.HighUsageThreshold {
		score -= (avgUsage - s.config.HighUsageThreshold) * 0.5
	}

	// 预算超支扣分
	if s.config.BudgetLimitMonthly > 0 {
		budgetUsage := totalCost / s.config.BudgetLimitMonthly * 100
		if budgetUsage > 80 {
			score -= (budgetUsage - 80) * 0.3
		}
	}

	// 确保评分在有效范围内
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return round(score, 1)
}

// calculatePotentialSavings 计算潜在节省空间
func (s *DashboardService) calculatePotentialSavings(resources []ResourceInfo) float64 {
	var savings float64

	for _, r := range resources {
		if r.TotalCapacityBytes > 0 {
			usage := float64(r.UsedCapacityBytes) / float64(r.TotalCapacityBytes) * 100

			// 低使用率资源：释放闲置空间的成本
			if usage < s.config.LowUsageThreshold {
				totalGB := float64(r.TotalCapacityBytes) / (1024 * 1024 * 1024)
				usedGB := float64(r.UsedCapacityBytes) / (1024 * 1024 * 1024)
				// 假设可以释放到使用率50%
				targetGB := usedGB * 2 // 50% 使用率
				if totalGB > targetGB {
					wastedGB := totalGB - targetGB
					savings += wastedGB * s.config.StorageCostPerGB
				}
			}
		}
	}

	return round(savings, 2)
}

// getTrendDataInRange 获取指定时间范围内的趋势数据
func (s *DashboardService) getTrendDataInRange(timeRange TimeRange) []TrendData {
	var result []TrendData

	for _, point := range s.trendData {
		if point.Timestamp.After(timeRange.StartTime) && point.Timestamp.Before(timeRange.EndTime) {
			result = append(result, point)
		}
	}

	return result
}

// buildTrendDataPoints 构建趋势数据点
func (s *DashboardService) buildTrendDataPoints(historical []TrendData, current *CostSummary, resources []ResourceInfo) []TrendData {
	points := make([]TrendData, len(historical)+1)

	// 复制历史数据
	for i, h := range historical {
		points[i] = h
	}

	// 添加当前数据点
	points[len(historical)] = TrendData{
		Timestamp:        time.Now(),
		TotalCost:        current.TotalCostMonthly,
		StorageCost:      current.CostByType[CostTypeStorage],
		ElectricityCost:  current.CostByType[CostTypeElectricity],
		NetworkCost:      current.CostByType[CostTypeNetwork],
		OperationsCost:   current.CostByType[CostTypeOperations],
		DepreciationCost: current.CostByType[CostTypeDepreciation],
		UsedGB:           s.getTotalUsedGB(resources),
		UsagePercent:     s.getAvgUsagePercent(resources),
		CostPerGB:        current.AvgCostPerGB,
	}

	return points
}

// calculateTrendStatistics 计算趋势统计
func (s *DashboardService) calculateTrendStatistics(points []TrendData) TrendStatistics {
	if len(points) == 0 {
		return TrendStatistics{}
	}

	stats := TrendStatistics{}

	// 计算平均值、最大值、最小值
	var sum float64
	for _, p := range points {
		sum += p.TotalCost
		if p.TotalCost > stats.MaxCost || stats.MaxCost == 0 {
			stats.MaxCost = p.TotalCost
		}
		if p.TotalCost < stats.MinCost || stats.MinCost == 0 {
			stats.MinCost = p.TotalCost
		}
	}
	stats.AvgCost = round(sum/float64(len(points)), 2)

	// 计算标准差
	var variance float64
	for _, p := range points {
		diff := p.TotalCost - stats.AvgCost
		variance += diff * diff
	}
	if len(points) > 1 {
		stats.CostStdDev = round(math.Sqrt(variance/float64(len(points)-1)), 2)
	}

	// 计算增长率
	if len(points) >= 2 {
		first := points[0].TotalCost
		last := points[len(points)-1].TotalCost
		if first > 0 {
			stats.GrowthRate = round((last-first)/first*100, 2)
		}
	}

	// 判断趋势方向
	if stats.GrowthRate > 5 {
		stats.TrendDirection = "increasing"
	} else if stats.GrowthRate < -5 {
		stats.TrendDirection = "decreasing"
	} else {
		stats.TrendDirection = "stable"
	}

	// 计算波动系数
	if stats.AvgCost > 0 {
		stats.VolatilityCoeff = round(stats.CostStdDev/stats.AvgCost*100, 2)
	}

	// 预测置信度（基于数据点数量和波动性）
	confidence := 100.0
	if len(points) < 7 {
		confidence -= float64(7-len(points)) * 10
	}
	if stats.VolatilityCoeff > 20 {
		confidence -= stats.VolatilityCoeff * 0.5
	}
	if confidence < 0 {
		confidence = 0
	}
	stats.Confidence = round(confidence, 1)

	return stats
}

// generateForecast 生成趋势预测
func (s *DashboardService) generateForecast(points []TrendData) TrendForecast {
	forecast := TrendForecast{
		ModelType: "linear",
	}

	if len(points) == 0 {
		return forecast
	}

	// 简单线性预测
	stats := s.calculateTrendStatistics(points)

	// 下月预测
	current := points[len(points)-1].TotalCost
	monthlyGrowth := stats.GrowthRate / 100.0
	forecast.NextMonthCost = round(current*(1+monthlyGrowth), 2)

	// 下季度预测
	forecast.NextQuarterCost = round(current*(1+monthlyGrowth*3), 2)

	// 下年度预测
	forecast.NextYearCost = round(current*(1+monthlyGrowth*12), 2)

	// 预测准确度基于置信度
	forecast.Accuracy = stats.Confidence

	// 生成预测数据点
	forecast.ForecastPoints = s.generateForecastPoints(points, stats)

	return forecast
}

// generateForecastPoints 生成预测数据点
func (s *DashboardService) generateForecastPoints(points []TrendData, stats TrendStatistics) []TrendData {
	if len(points) == 0 {
		return nil
	}

	forecastPoints := make([]TrendData, 3)
	current := points[len(points)-1]
	growthRate := stats.GrowthRate / 100.0

	now := time.Now()

	// 下月预测
	forecastPoints[0] = TrendData{
		Timestamp:  now.AddDate(0, 1, 0),
		TotalCost:  round(current.TotalCost*(1+growthRate), 2),
		Trend:      stats.TrendDirection,
		ChangeRate: stats.GrowthRate,
		UsedGB:     round(current.UsedGB*(1+growthRate), 2),
		CostPerGB:  current.CostPerGB,
	}

	// 下季度预测（3个月后）
	forecastPoints[1] = TrendData{
		Timestamp:  now.AddDate(0, 3, 0),
		TotalCost:  round(current.TotalCost*(1+growthRate*3), 2),
		Trend:      stats.TrendDirection,
		ChangeRate: stats.GrowthRate * 3,
		UsedGB:     round(current.UsedGB*(1+growthRate*3), 2),
		CostPerGB:  current.CostPerGB,
	}

	// 下半年预测（6个月后）
	forecastPoints[2] = TrendData{
		Timestamp:  now.AddDate(0, 6, 0),
		TotalCost:  round(current.TotalCost*(1+growthRate*6), 2),
		Trend:      stats.TrendDirection,
		ChangeRate: stats.GrowthRate * 6,
		UsedGB:     round(current.UsedGB*(1+growthRate*6), 2),
		CostPerGB:  current.CostPerGB,
	}

	return forecastPoints
}

// cleanupOldTrendData 清理过期趋势数据
func (s *DashboardService) cleanupOldTrendData() {
	if s.config.TrendRetentionDays <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -s.config.TrendRetentionDays)
	var validData []TrendData

	for _, point := range s.trendData {
		if point.Timestamp.After(cutoff) {
			validData = append(validData, point)
		}
	}

	s.trendData = validData
}

// UpdateConfig 更新配置
func (s *DashboardService) UpdateConfig(config DashboardConfig) {
	s.mu.Lock()
	s.config = config
	s.mu.Unlock()
}

// GetConfig 获取配置
func (s *DashboardService) GetConfig() DashboardConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// round 辅助函数：四舍五入
func round(val float64, precision int) float64 {
	multiplier := math.Pow(10, float64(precision))
	return math.Round(val*multiplier) / multiplier
}
