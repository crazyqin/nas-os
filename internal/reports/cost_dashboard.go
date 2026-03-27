// Package reports 提供报表生成和管理功能
package reports

import (
	"encoding/json"
	"time"
)

// ========== 存储成本分析仪表板 v2.60.0 ==========

// CostDashboard 成本分析仪表板.
type CostDashboard struct {
	// 仪表板ID
	ID string `json:"id"`

	// 名称
	Name string `json:"name"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 时间范围
	Period ReportPeriod `json:"period"`

	// 存储概览
	StorageSummary StorageCostSummary `json:"storage_summary"`

	// 成本概览
	CostSummary CostSummarySection `json:"cost_summary"`

	// 成本趋势
	CostTrend CostTrendSection `json:"cost_trend"`

	// 成本预测
	CostForecast CostForecastSection `json:"cost_forecast"`

	// 云存储对比
	CloudComparison CloudComparisonSection `json:"cloud_comparison"`

	// 成本分解
	CostBreakdown CostBreakdownSection `json:"cost_breakdown"`

	// 优化建议
	Recommendations []CostOptimizationRecommendation `json:"recommendations"`

	// 告警
	Alerts []CostAlert `json:"alerts"`

	// 图表数据
	Charts []CostChart `json:"charts"`
}

// StorageCostSummary 存储成本摘要.
type StorageCostSummary struct {
	// 总容量（TB）
	TotalCapacityTB float64 `json:"total_capacity_tb"`

	// 已用容量（TB）
	UsedCapacityTB float64 `json:"used_capacity_tb"`

	// 可用容量（TB）
	AvailableCapacityTB float64 `json:"available_capacity_tb"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 月增长率（%）
	MonthlyGrowthPercent float64 `json:"monthly_growth_percent"`

	// 预计满容量天数
	DaysToFullCapacity int `json:"days_to_full_capacity"`

	// 存储效率
	Efficiency StorageEfficiencyScore `json:"efficiency"`
}

// StorageEfficiencyScore 存储效率评分.
type StorageEfficiencyScore struct {
	// 总评分（0-100）
	OverallScore float64 `json:"overall_score"`

	// 空间利用率评分
	SpaceUtilization float64 `json:"space_utilization"`

	// 压缩率
	CompressionRatio float64 `json:"compression_ratio"`

	// 去重率
	DeduplicationRatio float64 `json:"deduplication_ratio"`

	// 等级
	Grade string `json:"grade"` // A, B, C, D, F
}

// CostSummarySection 成本摘要部分.
type CostSummarySection struct {
	// 本月总成本
	MonthlyTotal float64 `json:"monthly_total"`

	// 上月总成本
	LastMonthTotal float64 `json:"last_month_total"`

	// 环比变化（%）
	MonthOverMonthPercent float64 `json:"month_over_month_percent"`

	// 年度累计成本
	YearToDateTotal float64 `json:"year_to_date_total"`

	// 预计年度成本
	ProjectedYearTotal float64 `json:"projected_year_total"`

	// 单位成本（元/TB/月）
	CostPerTBPerMonth float64 `json:"cost_per_tb_per_month"`

	// 与预算对比
	BudgetComparison BudgetComparisonSummary `json:"budget_comparison"`
}

// BudgetComparisonSummary 预算对比摘要.
type BudgetComparisonSummary struct {
	// 月预算
	MonthlyBudget float64 `json:"monthly_budget"`

	// 已使用预算
	UsedBudget float64 `json:"used_budget"`

	// 剩余预算
	RemainingBudget float64 `json:"remaining_budget"`

	// 预算使用率（%）
	BudgetUsagePercent float64 `json:"budget_usage_percent"`

	// 预计超支
	ProjectedOverage float64 `json:"projected_overage"`

	// 状态
	Status string `json:"status"` // under_budget, on_track, over_budget
}

// CostTrendSection 成本趋势部分.
type CostTrendSection struct {
	// 趋势数据点
	DataPoints []CostTrendDataPoint `json:"data_points"`

	// 趋势方向
	Trend string `json:"trend"` // increasing, decreasing, stable

	// 平均月成本
	AvgMonthlyCost float64 `json:"avg_monthly_cost"`

	// 最高月成本
	MaxMonthlyCost float64 `json:"max_monthly_cost"`

	// 最低月成本
	MinMonthlyCost float64 `json:"min_monthly_cost"`

	// 成本波动率
	Volatility float64 `json:"volatility"`

	// 增长率（%/月）
	GrowthRate float64 `json:"growth_rate"`

	// 预测模型
	Model string `json:"model"`
}

// CostForecastSection 成本预测部分.
type CostForecastSection struct {
	// 预测数据点
	DataPoints []CostForecastDataPoint `json:"data_points"`

	// 预测模型
	Model string `json:"model"`

	// 置信度
	Confidence float64 `json:"confidence"`

	// 预测下月成本
	NextMonthCost float64 `json:"next_month_cost"`

	// 预测下季度成本
	NextQuarterCost float64 `json:"next_quarter_cost"`

	// 预测年度成本
	ForecastYearCost float64 `json:"forecast_year_cost"`
}

// CostForecastDataPoint 成本预测数据点.
type CostForecastDataPoint struct {
	// 时间
	Timestamp time.Time `json:"timestamp"`

	// 预测成本
	ForecastCost float64 `json:"forecast_cost"`

	// 置信区间下限
	ConfidenceLower float64 `json:"confidence_lower"`

	// 置信区间上限
	ConfidenceUpper float64 `json:"confidence_upper"`

	// 预测单位成本
	ForecastCostPerTB float64 `json:"forecast_cost_per_tb"`
}

// CloudComparisonSection 云存储对比部分.
type CloudComparisonSection struct {
	// 对比报告
	Comparison *CloudComparisonReport `json:"comparison,omitempty"`

	// 推荐方案
	RecommendedOption string `json:"recommended_option"`

	// 节省潜力（元/月）
	SavingsPotential float64 `json:"savings_potential"`

	// 最佳云服务商
	BestCloudProvider CloudProvider `json:"best_cloud_provider,omitempty"`

	// 最佳存储层级
	BestCloudTier CloudStorageTier `json:"best_cloud_tier,omitempty"`
}

// CostBreakdownSection 成本分解部分.
type CostBreakdownSection struct {
	// 分解项
	Items []CostBreakdownItem `json:"items"`

	// 图表类型
	ChartType string `json:"chart_type"` // pie, bar, treemap

	// 总成本
	TotalCost float64 `json:"total_cost"`
}

// CostBreakdownItem 成本分解项.
type CostBreakdownItem struct {
	// 分类
	Category string `json:"category"`

	// 名称
	Name string `json:"name"`

	// 金额
	Amount float64 `json:"amount"`

	// 占比（%）
	Percent float64 `json:"percent"`

	// 趋势（与上期对比）
	Trend float64 `json:"trend"`

	// 颜色
	Color string `json:"color"`

	// 子项
	Children []CostBreakdownItem `json:"children,omitempty"`
}

// CostOptimizationRecommendation 成本优化建议.
type CostOptimizationRecommendation struct {
	// ID
	ID string `json:"id"`

	// 类型
	Type string `json:"type"` // cost_reduction, efficiency, migration, consolidation

	// 优先级
	Priority string `json:"priority"` // high, medium, low

	// 标题
	Title string `json:"title"`

	// 描述
	Description string `json:"description"`

	// 当前成本
	CurrentCost float64 `json:"current_cost"`

	// 优化后成本
	OptimizedCost float64 `json:"optimized_cost"`

	// 节省金额（元/月）
	SavingsAmount float64 `json:"savings_amount"`

	// 节省比例（%）
	SavingsPercent float64 `json:"savings_percent"`

	// 实施难度
	Effort string `json:"effort"` // low, medium, high

	// 预计实施时间
	EstimatedTime string `json:"estimated_time"`

	// 实施步骤
	Steps []string `json:"steps"`

	// 风险评估
	Risks []string `json:"risks"`

	// 状态
	Status string `json:"status"` // pending, in_progress, completed, rejected
}

// CostAlert 成本告警.
type CostAlert struct {
	// ID
	ID string `json:"id"`

	// 类型
	Type string `json:"type"` // budget_overrun, cost_spike, efficiency_drop, capacity_warning

	// 严重级别
	Severity string `json:"severity"` // info, warning, critical

	// 标题
	Title string `json:"title"`

	// 消息
	Message string `json:"message"`

	// 当前值
	CurrentValue float64 `json:"current_value"`

	// 阈值
	Threshold float64 `json:"threshold"`

	// 触发时间
	TriggeredAt time.Time `json:"triggered_at"`

	// 是否已确认
	Acknowledged bool `json:"acknowledged"`

	// 建议操作
	SuggestedAction string `json:"suggested_action"`
}

// CostChart 成本图表.
type CostChart struct {
	// 图表ID
	ID string `json:"id"`

	// 类型
	Type string `json:"type"` // line, bar, pie, gauge, area, treemap

	// 标题
	Title string `json:"title"`

	// 数据
	Data interface{} `json:"data"`

	// 配置
	Config CostChartConfig `json:"config"`
}

// CostChartConfig 图表配置.
type CostChartConfig struct {
	// X轴标签
	XAxisLabel string `json:"x_axis_label,omitempty"`

	// Y轴标签
	YAxisLabel string `json:"y_axis_label,omitempty"`

	// 是否显示图例
	ShowLegend bool `json:"show_legend"`

	// 是否显示网格
	ShowGrid bool `json:"show_grid"`

	// 颜色方案
	ColorScheme string `json:"color_scheme,omitempty"`

	// 单位
	Unit string `json:"unit,omitempty"`

	// 格式化
	Format string `json:"format,omitempty"`
}

// CostDashboardGenerator 成本仪表板生成器.
type CostDashboardGenerator struct {
	costAnalyzer       *CostAnalyzer
	cloudCalculator    *CloudCostCalculator
	storageBytes       uint64
	usedBytes          uint64
	monthlyBudget      float64
	trendData          []CostTrendDataPoint
}

// NewCostDashboardGenerator 创建仪表板生成器.
func NewCostDashboardGenerator(costConfig CostConfig) *CostDashboardGenerator {
	return &CostDashboardGenerator{
		costAnalyzer:    NewCostAnalyzer(costConfig),
		cloudCalculator: NewCloudCostCalculator(),
	}
}

// SetStorageInfo 设置存储信息.
func (g *CostDashboardGenerator) SetStorageInfo(totalBytes, usedBytes uint64) {
	g.storageBytes = totalBytes
	g.usedBytes = usedBytes
	g.costAnalyzer.SetStorageCapacity(totalBytes)
}

// SetMonthlyBudget 设置月预算.
func (g *CostDashboardGenerator) SetMonthlyBudget(budget float64) {
	g.monthlyBudget = budget
}

// SetTrendData 设置趋势数据.
func (g *CostDashboardGenerator) SetTrendData(data []CostTrendDataPoint) {
	g.trendData = data
}

// Generate 生成仪表板.
func (g *CostDashboardGenerator) Generate(period ReportPeriod) *CostDashboard {
	now := time.Now()

	// 生成各部分
	storageSummary := g.generateStorageSummary()
	costSummary := g.generateCostSummary(period)
	costTrend := g.generateCostTrend()
	costForecast := g.generateCostForecast()
	cloudComparison := g.generateCloudComparison()
	costBreakdown := g.generateCostBreakdown(period)
	recommendations := g.generateRecommendations()
	alerts := g.generateAlerts(costSummary)
	charts := g.generateCharts(costSummary, costTrend, costBreakdown)

	return &CostDashboard{
		ID:              "cost_dashboard_" + now.Format("20060102150405"),
		Name:            "存储成本分析仪表板",
		GeneratedAt:     now,
		Period:          period,
		StorageSummary:  storageSummary,
		CostSummary:     costSummary,
		CostTrend:       costTrend,
		CostForecast:    costForecast,
		CloudComparison: cloudComparison,
		CostBreakdown:   costBreakdown,
		Recommendations: recommendations,
		Alerts:          alerts,
		Charts:          charts,
	}
}

// generateStorageSummary 生成存储摘要.
func (g *CostDashboardGenerator) generateStorageSummary() StorageCostSummary {
	totalTB := float64(g.storageBytes) / (1024 * 1024 * 1024 * 1024)
	usedTB := float64(g.usedBytes) / (1024 * 1024 * 1024 * 1024)
	availableTB := totalTB - usedTB

	usagePercent := 0.0
	if totalTB > 0 {
		usagePercent = usedTB / totalTB * 100
	}

	// 计算效率评分
	efficiency := StorageEfficiencyScore{
		OverallScore:        75.0,
		SpaceUtilization:    usagePercent,
		CompressionRatio:    1.5,
		DeduplicationRatio:  1.2,
		Grade:               "B",
	}
	if usagePercent >= 70 && usagePercent <= 85 {
		efficiency.OverallScore = 85.0
		efficiency.Grade = "A"
	}

	// 计算月增长率（如果有趋势数据）
	monthlyGrowth := 0.0
	daysToFull := 0
	if len(g.trendData) >= 2 {
		last := g.trendData[len(g.trendData)-1]
		first := g.trendData[0]
		months := last.Timestamp.Sub(first.Timestamp).Hours() / (24 * 30)
		if months > 0 && first.StorageTB > 0 {
			growth := (last.StorageTB - first.StorageTB) / first.StorageTB * 100
			monthlyGrowth = growth / months

			// 预测满容量天数
			if monthlyGrowth > 0 {
				remainingTB := availableTB
				growthPerMonth := usedTB * (monthlyGrowth / 100)
				if growthPerMonth > 0 {
					monthsToFull := remainingTB / growthPerMonth
					daysToFull = int(monthsToFull * 30)
				}
			}
		}
	}

	return StorageCostSummary{
		TotalCapacityTB:       round(totalTB, 2),
		UsedCapacityTB:        round(usedTB, 2),
		AvailableCapacityTB:   round(availableTB, 2),
		UsagePercent:          round(usagePercent, 1),
		MonthlyGrowthPercent:  round(monthlyGrowth, 1),
		DaysToFullCapacity:    daysToFull,
		Efficiency:            efficiency,
	}
}

// generateCostSummary 生成成本摘要.
func (g *CostDashboardGenerator) generateCostSummary(period ReportPeriod) CostSummarySection {
	cost := g.costAnalyzer.CalculateCost(period)

	var lastMonthTotal float64
	if len(g.trendData) >= 1 {
		lastMonthTotal = g.trendData[len(g.trendData)-1].TotalCost
	}

	monthOverMonth := 0.0
	if lastMonthTotal > 0 {
		monthOverMonth = (cost.TotalCost - lastMonthTotal) / lastMonthTotal * 100
	}

	// 计算年度累计
	yearToDate := 0.0
	for _, dp := range g.trendData {
		if dp.Timestamp.Year() == time.Now().Year() {
			yearToDate += dp.TotalCost
		}
	}

	// 预算对比
	budgetComparison := BudgetComparisonSummary{
		MonthlyBudget:    g.monthlyBudget,
		UsedBudget:       cost.TotalCost,
		RemainingBudget:  g.monthlyBudget - cost.TotalCost,
		BudgetUsagePercent: round(cost.TotalCost/g.monthlyBudget*100, 1),
		Status:           "on_track",
	}
	if budgetComparison.BudgetUsagePercent > 100 {
		budgetComparison.Status = "over_budget"
		budgetComparison.ProjectedOverage = cost.TotalCost - g.monthlyBudget
	} else if budgetComparison.BudgetUsagePercent > 90 {
		budgetComparison.Status = "on_track"
	} else {
		budgetComparison.Status = "under_budget"
	}

	return CostSummarySection{
		MonthlyTotal:          cost.TotalCost,
		LastMonthTotal:        lastMonthTotal,
		MonthOverMonthPercent: round(monthOverMonth, 1),
		YearToDateTotal:       round(yearToDate, 2),
		ProjectedYearTotal:    round(cost.TotalCost*12, 2),
		CostPerTBPerMonth:     cost.CostPerTB,
		BudgetComparison:      budgetComparison,
	}
}

// generateCostTrend 生成成本趋势.
func (g *CostDashboardGenerator) generateCostTrend() CostTrendSection {
	if len(g.trendData) == 0 {
		return CostTrendSection{}
	}

	// 计算统计数据
	var totalCost, maxCost, minCost float64
	maxCost = g.trendData[0].TotalCost
	minCost = g.trendData[0].TotalCost

	for _, dp := range g.trendData {
		totalCost += dp.TotalCost
		if dp.TotalCost > maxCost {
			maxCost = dp.TotalCost
		}
		if dp.TotalCost < minCost {
			minCost = dp.TotalCost
		}
	}

	avgCost := totalCost / float64(len(g.trendData))

	// 计算波动率（标准差/均值）
	variance := 0.0
	for _, dp := range g.trendData {
		variance += (dp.TotalCost - avgCost) * (dp.TotalCost - avgCost)
	}
	variance /= float64(len(g.trendData))
	volatility := 0.0
	if avgCost > 0 {
		volatility = round(float64(100)/avgCost, 2)
	}

	// 判断趋势
	trend := "stable"
	if len(g.trendData) >= 3 {
		recent := g.trendData[len(g.trendData)-3:]
		if recent[2].TotalCost > recent[0].TotalCost*1.05 {
			trend = "increasing"
		} else if recent[2].TotalCost < recent[0].TotalCost*0.95 {
			trend = "decreasing"
		}
	}

	return CostTrendSection{
		DataPoints:     g.trendData,
		Trend:          trend,
		AvgMonthlyCost: round(avgCost, 2),
		MaxMonthlyCost: round(maxCost, 2),
		MinMonthlyCost: round(minCost, 2),
		Volatility:     round(volatility, 2),
	}
}

// generateCostForecast 生成成本预测.
func (g *CostDashboardGenerator) generateCostForecast() CostForecastSection {
	if len(g.trendData) < 2 {
		return CostForecastSection{}
	}

	// 将趋势数据转换为预测器可用格式
	trends := make([]CostTrend, len(g.trendData))
	for i, dp := range g.trendData {
		trends[i] = CostTrend{
			Timestamp:        dp.Timestamp,
			TotalCost:        dp.TotalCost,
			StorageCapacityTB: dp.StorageTB,
		}
	}

	// 预测未来6个月
	forecasts := g.costAnalyzer.ForecastCost(trends, 6)

	// 转换为仪表板格式
	dataPoints := make([]CostForecastDataPoint, len(forecasts))
	for i, f := range forecasts {
		dataPoints[i] = CostForecastDataPoint{
			Timestamp:         f.Timestamp,
			ForecastCost:      f.ForecastCost,
			ConfidenceLower:   f.ConfidenceLower,
			ConfidenceUpper:   f.ConfidenceUpper,
			ForecastCostPerTB: f.ForecastCostPerTB,
		}
	}

	var nextMonth, nextQuarter, forecastYear float64
	if len(forecasts) > 0 {
		nextMonth = forecasts[0].ForecastCost
	}
	if len(forecasts) >= 3 {
		nextQuarter = forecasts[0].ForecastCost + forecasts[1].ForecastCost + forecasts[2].ForecastCost
	}
	for _, f := range forecasts {
		forecastYear += f.ForecastCost
	}

	return CostForecastSection{
		DataPoints:      dataPoints,
		Model:           "linear_growth",
		Confidence:      0.85,
		NextMonthCost:   round(nextMonth, 2),
		NextQuarterCost: round(nextQuarter, 2),
		ForecastYearCost: round(forecastYear, 2),
	}
}

// generateCloudComparison 生成云存储对比.
func (g *CostDashboardGenerator) generateCloudComparison() CloudComparisonSection {
	cost := g.costAnalyzer.CalculateCost(ReportPeriod{
		StartTime: time.Now().AddDate(0, -1, 0),
		EndTime:   time.Now(),
	})

	storageGB := float64(g.usedBytes) / (1024 * 1024 * 1024)

	comparison := g.cloudCalculator.CompareWithCloud(cost, storageGB, 100, 50)

	recommended := "self_hosted"
	var savings float64
	var provider CloudProvider
	var tier CloudStorageTier

	if comparison != nil {
		recommended = comparison.Recommendation.RecommendedType
		savings = comparison.Recommendation.SavingsPerMonth
		provider = comparison.Recommendation.RecommendedProvider
		tier = comparison.Recommendation.RecommendedTier
	}

	return CloudComparisonSection{
		Comparison:         comparison,
		RecommendedOption:  recommended,
		SavingsPotential:   round(savings, 2),
		BestCloudProvider:  provider,
		BestCloudTier:      tier,
	}
}

// generateCostBreakdown 生成成本分解.
func (g *CostDashboardGenerator) generateCostBreakdown(period ReportPeriod) CostBreakdownSection {
	cost := g.costAnalyzer.CalculateCost(period)
	breakdown := g.costAnalyzer.GetCostBreakdown(cost)

	items := make([]CostBreakdownItem, len(breakdown))
	colors := []string{"#667eea", "#48bb78", "#ed8936", "#f56565", "#9f7aea", "#38b2ac"}

	for i, b := range breakdown {
		color := colors[i%len(colors)]
		items[i] = CostBreakdownItem{
			Category: b.Category,
			Name:     b.Description,
			Amount:   b.Amount,
			Percent:  b.Percent,
			Color:    color,
		}
	}

	return CostBreakdownSection{
		Items:      items,
		ChartType:  "pie",
		TotalCost:  cost.TotalCost,
	}
}

// generateRecommendations 生成优化建议.
func (g *CostDashboardGenerator) generateRecommendations() []CostOptimizationRecommendation {
	recommendations := make([]CostOptimizationRecommendation, 0)

	// 基于存储使用率生成建议
	usagePercent := 0.0
	if g.storageBytes > 0 {
		usagePercent = float64(g.usedBytes) / float64(g.storageBytes) * 100
	}

	if usagePercent > 80 {
		recommendations = append(recommendations, CostOptimizationRecommendation{
			ID:            "rec_1",
			Type:          "cost_reduction",
			Priority:      "high",
			Title:         "清理冗余数据释放存储空间",
			Description:   "当前存储使用率超过80%，建议清理过期数据、重复文件和临时文件",
			SavingsAmount: round(float64(g.usedBytes)*0.1/(1024*1024*1024)*100, 2), // 假设可清理10%
			SavingsPercent: 10,
			Effort:         "low",
			EstimatedTime:  "1-2周",
			Steps: []string{
				"扫描并识别重复文件",
				"清理过期备份文件",
				"归档不常用数据到冷存储",
			},
			Status: "pending",
		})
	}

	// 压缩优化建议
	recommendations = append(recommendations, CostOptimizationRecommendation{
		ID:            "rec_2",
		Type:          "efficiency",
		Priority:      "medium",
		Title:         "启用数据压缩降低存储成本",
		Description:   "对支持压缩的数据类型启用压缩功能，预计可节省20-40%存储空间",
		SavingsAmount: round(float64(g.usedBytes)*0.3/(1024*1024*1024)*100, 2),
		SavingsPercent: 30,
		Effort:        "medium",
		EstimatedTime: "1个月",
		Steps: []string{
			"评估可压缩数据类型",
			"配置压缩策略",
			"逐步启用压缩并监控性能影响",
		},
		Status: "pending",
	})

	// 云存储迁移建议
	recommendations = append(recommendations, CostOptimizationRecommendation{
		ID:            "rec_3",
		Type:          "migration",
		Priority:      "low",
		Title:         "冷数据迁移到云归档存储",
		Description:   "将超过90天未访问的冷数据迁移到云归档存储，降低本地存储压力",
		SavingsAmount: round(float64(g.usedBytes)*0.2/(1024*1024*1024)*50, 2), // 假设20%冷数据
		SavingsPercent: 15,
		Effort:        "high",
		EstimatedTime: "2-3个月",
		Steps: []string{
			"识别冷数据（超过90天未访问）",
			"选择合适的云存储服务商",
			"制定迁移计划并执行",
			"验证数据完整性",
		},
		Risks: []string{
			"数据迁移期间可能影响访问",
			"需要确保网络带宽充足",
			"取回数据可能有延迟",
		},
		Status: "pending",
	})

	return recommendations
}

// generateAlerts 生成告警.
func (g *CostDashboardGenerator) generateAlerts(costSummary CostSummarySection) []CostAlert {
	alerts := make([]CostAlert, 0)

	// 预算告警
	if costSummary.BudgetComparison.Status == "over_budget" {
		alerts = append(alerts, CostAlert{
			ID:             "alert_budget_1",
			Type:           "budget_overrun",
			Severity:       "critical",
			Title:          "预算超支警告",
			Message:        "本月存储成本已超出预算",
			CurrentValue:   costSummary.MonthlyTotal,
			Threshold:      costSummary.BudgetComparison.MonthlyBudget,
			TriggeredAt:    time.Now(),
			SuggestedAction: "立即审查成本构成，采取节约措施",
		})
	} else if costSummary.BudgetComparison.BudgetUsagePercent > 90 {
		alerts = append(alerts, CostAlert{
			ID:             "alert_budget_2",
			Type:           "budget_overrun",
			Severity:       "warning",
			Title:          "预算接近上限",
			Message:        "本月存储成本已使用预算的90%以上",
			CurrentValue:   costSummary.BudgetComparison.BudgetUsagePercent,
			Threshold:      90,
			TriggeredAt:    time.Now(),
			SuggestedAction: "监控成本变化，考虑采取节约措施",
		})
	}

	// 成本突增告警
	if costSummary.MonthOverMonthPercent > 20 {
		alerts = append(alerts, CostAlert{
			ID:             "alert_spike_1",
			Type:           "cost_spike",
			Severity:       "warning",
			Title:          "成本突增警告",
			Message:        "本月成本环比增长超过20%",
			CurrentValue:   costSummary.MonthOverMonthPercent,
			Threshold:      20,
			TriggeredAt:    time.Now(),
			SuggestedAction: "分析成本增长原因，排查异常使用",
		})
	}

	return alerts
}

// generateCharts 生成图表数据.
func (g *CostDashboardGenerator) generateCharts(
	costSummary CostSummarySection,
	costTrend CostTrendSection,
	costBreakdown CostBreakdownSection,
) []CostChart {
	charts := make([]CostChart, 0)

	// 成本趋势图
	if len(g.trendData) > 0 {
		trendChartData, _ := json.Marshal(g.trendData)
		charts = append(charts, CostChart{
			ID:    "chart_trend",
			Type:  "line",
			Title: "成本趋势",
			Data:  string(trendChartData),
			Config: CostChartConfig{
				XAxisLabel:  "时间",
				YAxisLabel:  "成本（元）",
				ShowLegend:  true,
				ShowGrid:    true,
				ColorScheme: "blue",
				Unit:        "yuan",
			},
		})
	}

	// 成本分解饼图
	pieData, _ := json.Marshal(costBreakdown.Items)
	charts = append(charts, CostChart{
		ID:    "chart_breakdown",
		Type:  "pie",
		Title: "成本构成",
		Data:  string(pieData),
		Config: CostChartConfig{
			ShowLegend:  true,
			ColorScheme: "default",
		},
	})

	// 预算对比仪表盘
	gaugeData := map[string]interface{}{
		"value":    costSummary.BudgetComparison.BudgetUsagePercent,
		"max":      100,
		"thresholds": []float64{70, 90},
	}
	gaugeDataJSON, _ := json.Marshal(gaugeData)
	charts = append(charts, CostChart{
		ID:    "chart_budget",
		Type:  "gauge",
		Title: "预算使用率",
		Data:  string(gaugeDataJSON),
		Config: CostChartConfig{
			Unit:   "percent",
			Format: "0.0%",
		},
	})

	return charts
}