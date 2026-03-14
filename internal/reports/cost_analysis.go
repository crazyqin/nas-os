// Package reports 提供报表生成和管理功能
package reports

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// ========== 成本分析报告 ==========

// CostAnalysisReport 成本分析报告
type CostAnalysisReport struct {
	ID              string                    `json:"id"`
	Name            string                    `json:"name"`
	GeneratedAt     time.Time                 `json:"generated_at"`
	Period          ReportPeriod              `json:"period"`
	TotalCost       CostBreakdown             `json:"total_cost"`
	VolumeCosts     []VolumeCostAnalysis      `json:"volume_costs"`
	UserCosts       []UserCostAnalysis        `json:"user_costs"`
	TrendAnalysis   CostTrendAnalysis         `json:"trend_analysis"`
	Forecast        *CostForecast             `json:"forecast,omitempty"`
	Optimization    []CostOptimizationItem    `json:"optimization"`
	Recommendations []CostRecommendation      `json:"recommendations"`
	Summary         CostAnalysisSummary       `json:"summary"`
}

// CostBreakdown 成本细分
type CostBreakdown struct {
	StorageCost       float64 `json:"storage_cost"`        // 存储成本
	ComputeCost       float64 `json:"compute_cost"`        // 计算成本
	NetworkCost       float64 `json:"network_cost"`        // 网络成本
	OperationsCost    float64 `json:"operations_cost"`     // 运维成本
	ElectricityCost   float64 `json:"electricity_cost"`    // 电费成本
	DepreciationCost  float64 `json:"depreciation_cost"`   // 折旧成本
	TotalMonthlyCost  float64 `json:"total_monthly_cost"`  // 月度总成本
	CostPerGB         float64 `json:"cost_per_gb"`         // 每GB成本
	CostPerUser       float64 `json:"cost_per_user"`       // 每用户成本
}

// VolumeCostAnalysis 卷成本分析
type VolumeCostAnalysis struct {
	VolumeID          string        `json:"volume_id"`
	VolumeName        string        `json:"volume_name"`
	VolumeType        string        `json:"volume_type"`        // ssd, hdd, nvme
	CapacityGB        float64       `json:"capacity_gb"`
	UsedGB            float64       `json:"used_gb"`
	UsagePercent      float64       `json:"usage_percent"`
	CostBreakdown     CostBreakdown `json:"cost_breakdown"`
	EfficiencyScore   float64       `json:"efficiency_score"`   // 成本效率评分 0-100
	CostTrend         string        `json:"cost_trend"`         // increasing, stable, decreasing
	MonthlyGrowthRate float64       `json:"monthly_growth_rate"`
	PeakUsagePercent  float64       `json:"peak_usage_percent"`
	AvgUsagePercent   float64       `json:"avg_usage_percent"`
}

// UserCostAnalysis 用户成本分析
type UserCostAnalysis struct {
	UserID          string        `json:"user_id"`
	Username        string        `json:"username"`
	QuotaBytes      uint64        `json:"quota_bytes"`
	UsedBytes       uint64        `json:"used_bytes"`
	UsagePercent    float64       `json:"usage_percent"`
	CostBreakdown   CostBreakdown `json:"cost_breakdown"`
	FileCount       uint64        `json:"file_count"`
	AvgFileSize     float64       `json:"avg_file_size"`
	CostEfficiency  float64       `json:"cost_efficiency"`  // 成本效率
	LastAccessTime  *time.Time    `json:"last_access_time,omitempty"`
	TopFileTypes    []FileTypeCost `json:"top_file_types"`
}

// FileTypeCost 文件类型成本
type FileTypeCost struct {
	Type        string  `json:"type"`         // 文件扩展名
	Count       uint64  `json:"count"`        // 文件数量
	TotalBytes  uint64  `json:"total_bytes"`  // 总字节数
	Cost        float64 `json:"cost"`         // 成本
	Percent     float64 `json:"percent"`      // 占比
}

// CostTrendAnalysis 成本趋势分析
type CostTrendAnalysis struct {
	DataPoints        []CostTrendDataPoint `json:"data_points"`
	AvgMonthlyCost    float64              `json:"avg_monthly_cost"`
	MonthlyGrowthRate float64              `json:"monthly_growth_rate"`
	SeasonalPattern   string               `json:"seasonal_pattern"`   // none, monthly, quarterly
	PeakMonth         string               `json:"peak_month"`
	TrendDirection    string               `json:"trend_direction"`    // up, down, stable
	Volatility        float64              `json:"volatility"`         // 成本波动率
}

// CostTrendDataPoint 成本趋势数据点
type CostTrendDataPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	TotalCost      float64   `json:"total_cost"`
	StorageCost    float64   `json:"storage_cost"`
	ComputeCost    float64   `json:"compute_cost"`
	NetworkCost    float64   `json:"network_cost"`
	UsageGB        float64   `json:"usage_gb"`
	CostPerGB      float64   `json:"cost_per_gb"`
}

// CostForecast 成本预测
type CostForecast struct {
	NextMonthCost     float64             `json:"next_month_cost"`
	NextQuarterCost   float64             `json:"next_quarter_cost"`
	NextYearCost      float64             `json:"next_year_cost"`
	ForecastPoints    []ForecastPoint     `json:"forecast_points"`
	Confidence        float64             `json:"confidence"`     // 置信度 0-1
	Method            string              `json:"method"`         // linear, exponential, arima
	WarningThreshold  float64             `json:"warning_threshold"`
	BudgetAlert       bool                `json:"budget_alert"`
}

// ForecastPoint 预测数据点
type ForecastPoint struct {
	Date            time.Time `json:"date"`
	PredictedCost   float64   `json:"predicted_cost"`
	LowerBound      float64   `json:"lower_bound"`     // 置信下限
	UpperBound      float64   `json:"upper_bound"`     // 置信上限
	IsBudgetExceed  bool      `json:"is_budget_exceed"`
}

// CostOptimizationItem 成本优化项
type CostOptimizationItem struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`            // cleanup, tiering, compression, dedupe
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	CurrentCost     float64   `json:"current_cost"`
	OptimizedCost   float64   `json:"optimized_cost"`
	Savings         float64   `json:"savings"`         // 节省金额
	SavingsPercent  float64   `json:"savings_percent"` // 节省比例
	Priority        string    `json:"priority"`        // high, medium, low
	Effort          string    `json:"effort"`          // easy, medium, hard
	Impact          string    `json:"impact"`          // 影响范围
	Risk            string    `json:"risk"`            // 风险评估
	Steps           []string  `json:"steps"`           // 实施步骤
	CreatedAt       time.Time `json:"created_at"`
}

// CostRecommendation 成本建议
type CostRecommendation struct {
	Type        string `json:"type"`        // reduce, optimize, monitor, expand
	Priority    string `json:"priority"`    // critical, high, medium, low
	Title       string `json:"title"`
	Description string `json:"description"`
	Savings     float64 `json:"savings"`     // 预计节省
	Action      string `json:"action"`      // 建议操作
	Deadline    string `json:"deadline"`    // 建议截止时间
}

// CostAnalysisSummary 成本分析摘要
type CostAnalysisSummary struct {
	TotalMonthlyCost      float64 `json:"total_monthly_cost"`
	TotalStorageGB        float64 `json:"total_storage_gb"`
	TotalUsedGB           float64 `json:"total_used_gb"`
	AvgCostPerGB          float64 `json:"avg_cost_per_gb"`
	AvgUsagePercent       float64 `json:"avg_usage_percent"`
	PotentialSavings      float64 `json:"potential_savings"`     // 潜在节省
	PotentialSavingsPercent float64 `json:"potential_savings_percent"`
	HealthScore           int     `json:"health_score"`          // 成本健康评分 0-100
	Status                string  `json:"status"`                // healthy, warning, critical
}

// CostAnalyzer 成本分析器
type CostAnalyzer struct {
	config      StorageCostConfig
	optimizer   *CostOptimizer
	calculator  *StorageCostCalculator
}

// NewCostAnalyzer 创建成本分析器
func NewCostAnalyzer(config StorageCostConfig) *CostAnalyzer {
	return &CostAnalyzer{
		config:     config,
		optimizer:  NewCostOptimizer(config),
		calculator: NewStorageCostCalculator(config),
	}
}

// Analyze 执行成本分析
func (a *CostAnalyzer) Analyze(
	volumeMetrics []StorageMetrics,
	userUsages []UserStorageUsage,
	history []CostTrendDataPoint,
	period ReportPeriod,
) *CostAnalysisReport {
	now := time.Now()
	report := &CostAnalysisReport{
		ID:           "cost_analysis_" + now.Format("20060102150405"),
		Name:         "成本分析报告",
		GeneratedAt:  now,
		Period:       period,
		VolumeCosts:  make([]VolumeCostAnalysis, 0),
		UserCosts:    make([]UserCostAnalysis, 0),
		Optimization: make([]CostOptimizationItem, 0),
		Recommendations: make([]CostRecommendation, 0),
	}

	// 分析卷成本
	report.VolumeCosts = a.analyzeVolumeCosts(volumeMetrics)

	// 分析用户成本
	report.UserCosts = a.analyzeUserCosts(userUsages)

	// 计算总成本
	report.TotalCost = a.calculateTotalCost(report.VolumeCosts, report.UserCosts)

	// 趋势分析
	report.TrendAnalysis = a.analyzeTrend(history)

	// 成本预测
	if len(history) >= 3 {
		report.Forecast = a.forecastCost(history)
	}

	// 优化建议
	report.Optimization = a.identifyOptimizations(report)

	// 生成建议
	report.Recommendations = a.generateRecommendations(report)

	// 计算摘要
	report.Summary = a.calculateSummary(report)

	return report
}

// UserStorageUsage 用户存储使用情况
type UserStorageUsage struct {
	UserID         string    `json:"user_id"`
	Username       string    `json:"username"`
	QuotaBytes     uint64    `json:"quota_bytes"`
	UsedBytes      uint64    `json:"used_bytes"`
	FileCount      uint64    `json:"file_count"`
	LastAccessTime *time.Time `json:"last_access_time,omitempty"`
	FileTypes      map[string]uint64 `json:"file_types"` // 扩展名 -> 字节数
}

// analyzeVolumeCosts 分析卷成本
func (a *CostAnalyzer) analyzeVolumeCosts(metrics []StorageMetrics) []VolumeCostAnalysis {
	analyses := make([]VolumeCostAnalysis, 0, len(metrics))

	for _, m := range metrics {
		cost := a.calculator.Calculate(m)
		gb := float64(m.TotalCapacityBytes) / (1024 * 1024 * 1024)
		usedGB := float64(m.UsedCapacityBytes) / (1024 * 1024 * 1024)

		analysis := VolumeCostAnalysis{
			VolumeID:       m.VolumeName,
			VolumeName:     m.VolumeName,
			VolumeType:     "hdd", // 默认
			CapacityGB:     round(gb, 2),
			UsedGB:         round(usedGB, 2),
			UsagePercent:   cost.UsagePercent,
			CostBreakdown: CostBreakdown{
				StorageCost:      cost.CapacityCostMonthly,
				TotalMonthlyCost: cost.TotalCostMonthly,
				CostPerGB:        cost.CostPerGBMonthly,
			},
			EfficiencyScore: a.calculateEfficiencyScore(cost.UsagePercent),
			CostTrend:       "stable",
			AvgUsagePercent: cost.UsagePercent,
			PeakUsagePercent: cost.UsagePercent,
		}

		analyses = append(analyses, analysis)
	}

	return analyses
}

// analyzeUserCosts 分析用户成本
func (a *CostAnalyzer) analyzeUserCosts(usages []UserStorageUsage) []UserCostAnalysis {
	analyses := make([]UserCostAnalysis, 0, len(usages))

	for _, u := range usages {
		usagePercent := 0.0
		if u.QuotaBytes > 0 {
			usagePercent = float64(u.UsedBytes) / float64(u.QuotaBytes) * 100
		}

		usedGB := float64(u.UsedBytes) / (1024 * 1024 * 1024)
		cost := usedGB * a.config.CostPerGBMonthly

		avgFileSize := 0.0
		if u.FileCount > 0 {
			avgFileSize = float64(u.UsedBytes) / float64(u.FileCount)
		}

		// 分析文件类型
		topTypes := make([]FileTypeCost, 0)
		for ext, bytes := range u.FileTypes {
			typeCost := float64(bytes) / (1024 * 1024 * 1024) * a.config.CostPerGBMonthly
			typePercent := 0.0
			if u.UsedBytes > 0 {
				typePercent = float64(bytes) / float64(u.UsedBytes) * 100
			}
			topTypes = append(topTypes, FileTypeCost{
				Type:       ext,
				TotalBytes: bytes,
				Cost:       round(typeCost, 2),
				Percent:    round(typePercent, 2),
			})
		}

		// 按成本排序
		sort.Slice(topTypes, func(i, j int) bool {
			return topTypes[i].Cost > topTypes[j].Cost
		})

		// 只保留前5个
		if len(topTypes) > 5 {
			topTypes = topTypes[:5]
		}

		analysis := UserCostAnalysis{
			UserID:         u.UserID,
			Username:       u.Username,
			QuotaBytes:     u.QuotaBytes,
			UsedBytes:      u.UsedBytes,
			UsagePercent:   round(usagePercent, 2),
			CostBreakdown: CostBreakdown{
				StorageCost:      round(cost, 2),
				TotalMonthlyCost: round(cost, 2),
			},
			FileCount:      u.FileCount,
			AvgFileSize:    avgFileSize,
			CostEfficiency: a.calculateCostEfficiency(usagePercent),
			LastAccessTime: u.LastAccessTime,
			TopFileTypes:   topTypes,
		}

		analyses = append(analyses, analysis)
	}

	return analyses
}

// calculateTotalCost 计算总成本
func (a *CostAnalyzer) calculateTotalCost(volumeCosts []VolumeCostAnalysis, userCosts []UserCostAnalysis) CostBreakdown {
	total := CostBreakdown{}

	for _, vc := range volumeCosts {
		total.StorageCost += vc.CostBreakdown.StorageCost
		total.TotalMonthlyCost += vc.CostBreakdown.TotalMonthlyCost
	}

	total.StorageCost = round(total.StorageCost, 2)
	total.TotalMonthlyCost = round(total.TotalMonthlyCost, 2)

	// 计算平均每GB成本
	var totalGB float64
	for _, vc := range volumeCosts {
		totalGB += vc.CapacityGB
	}
	if totalGB > 0 {
		total.CostPerGB = round(total.TotalMonthlyCost/totalGB, 2)
	}

	// 计算平均每用户成本
	if len(userCosts) > 0 {
		total.CostPerUser = round(total.TotalMonthlyCost/float64(len(userCosts)), 2)
	}

	return total
}

// analyzeTrend 分析趋势
func (a *CostAnalyzer) analyzeTrend(history []CostTrendDataPoint) CostTrendAnalysis {
	analysis := CostTrendAnalysis{
		DataPoints: history,
	}

	if len(history) < 2 {
		return analysis
	}

	// 计算平均月成本
	var totalCost float64
	for _, h := range history {
		totalCost += h.TotalCost
	}
	analysis.AvgMonthlyCost = round(totalCost/float64(len(history)), 2)

	// 计算增长率
	first := history[0]
	last := history[len(history)-1]
	if first.TotalCost > 0 {
		growthRate := (last.TotalCost - first.TotalCost) / first.TotalCost * 100
		months := last.Timestamp.Sub(first.Timestamp).Hours() / (24 * 30)
		if months > 0 {
			analysis.MonthlyGrowthRate = round(growthRate/months, 2)
		}
	}

	// 判断趋势方向
	if analysis.MonthlyGrowthRate > 5 {
		analysis.TrendDirection = "up"
	} else if analysis.MonthlyGrowthRate < -5 {
		analysis.TrendDirection = "down"
	} else {
		analysis.TrendDirection = "stable"
	}

	// 计算波动率
	var variance, mean float64
	mean = analysis.AvgMonthlyCost
	for _, h := range history {
		diff := h.TotalCost - mean
		variance += diff * diff
	}
	if len(history) > 1 {
		analysis.Volatility = round(math.Sqrt(variance/float64(len(history)-1))/mean*100, 2)
	}

	// 找出峰值月份
	maxCost := 0.0
	for _, h := range history {
		if h.TotalCost > maxCost {
			maxCost = h.TotalCost
			analysis.PeakMonth = h.Timestamp.Format("2006-01")
		}
	}

	return analysis
}

// forecastCost 预测成本
func (a *CostAnalyzer) forecastCost(history []CostTrendDataPoint) *CostForecast {
	if len(history) < 3 {
		return nil
	}

	forecast := &CostForecast{
		Method:   "linear",
		ForecastPoints: make([]ForecastPoint, 0),
	}

	// 线性回归预测
	n := float64(len(history))
	var sumX, sumY, sumXY, sumX2 float64

	for i, h := range history {
		x := float64(i)
		y := h.TotalCost
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return nil
	}

	b := (n*sumXY - sumX*sumY) / denominator
	c := (sumY - b*sumX) / n

	last := history[len(history)-1]

	// 预测下个月
	nextMonthIdx := float64(len(history))
	forecast.NextMonthCost = round(c+b*nextMonthIdx, 2)

	// 预测下季度
	forecast.NextQuarterCost = round(c+b*(nextMonthIdx+2), 2)

	// 预测下年
	forecast.NextYearCost = round(c+b*(nextMonthIdx+11), 2)

	// 生成预测点
	for i := 1; i <= 12; i++ {
		date := last.Timestamp.AddDate(0, i, 0)
		predicted := c + b*(nextMonthIdx+float64(i-1))
		margin := predicted * 0.1 // 10% 置信区间

		forecast.ForecastPoints = append(forecast.ForecastPoints, ForecastPoint{
			Date:           date,
			PredictedCost:  round(predicted, 2),
			LowerBound:     round(predicted-margin, 2),
			UpperBound:     round(predicted+margin, 2),
			IsBudgetExceed: false,
		})
	}

	// 计算置信度
	forecast.Confidence = a.calculateForecastConfidence(history)

	return forecast
}

// calculateForecastConfidence 计算预测置信度
func (a *CostAnalyzer) calculateForecastConfidence(history []CostTrendDataPoint) float64 {
	if len(history) < 6 {
		return 0.5
	}

	// 基于数据点数量和波动率计算置信度
	confidence := 0.7

	// 数据点越多，置信度越高
	if len(history) >= 12 {
		confidence += 0.15
	} else if len(history) >= 6 {
		confidence += 0.1
	}

	// 波动率越低，置信度越高
	analysis := a.analyzeTrend(history)
	if analysis.Volatility < 5 {
		confidence += 0.1
	} else if analysis.Volatility > 20 {
		confidence -= 0.2
	}

	// 确保在 0-1 范围内
	if confidence > 1 {
		confidence = 1
	}
	if confidence < 0.3 {
		confidence = 0.3
	}

	return round(confidence, 2)
}

// identifyOptimizations 识别优化项
func (a *CostAnalyzer) identifyOptimizations(report *CostAnalysisReport) []CostOptimizationItem {
	items := make([]CostOptimizationItem, 0)
	now := time.Now()

	// 基于低使用率卷
	for _, vc := range report.VolumeCosts {
		if vc.UsagePercent < 30 {
			savings := vc.CostBreakdown.TotalMonthlyCost * 0.3
			items = append(items, CostOptimizationItem{
				ID:             "opt_low_usage_" + vc.VolumeID,
				Type:           "tiering",
				Title:          "收缩低使用率卷",
				Description:    fmt.Sprintf("卷 %s 使用率仅 %.1f%%，建议收缩释放资源", vc.VolumeName, vc.UsagePercent),
				CurrentCost:    vc.CostBreakdown.TotalMonthlyCost,
				OptimizedCost:  vc.CostBreakdown.TotalMonthlyCost * 0.7,
				Savings:        round(savings, 2),
				SavingsPercent: 30,
				Priority:       "medium",
				Effort:         "medium",
				Impact:         "单个卷",
				Risk:           "低",
				Steps: []string{
					"1. 评估卷使用趋势",
					"2. 规划收缩方案",
					"3. 执行卷收缩",
					"4. 验证服务正常",
				},
				CreatedAt: now,
			})
		}
	}

	// 基于用户配额
	for _, uc := range report.UserCosts {
		if uc.UsagePercent < 20 && uc.UsedBytes > 0 {
			savings := uc.CostBreakdown.TotalMonthlyCost * 0.5
			items = append(items, CostOptimizationItem{
				ID:             "opt_quota_" + uc.UserID,
				Type:           "quota",
				Title:          "调整用户配额",
				Description:    fmt.Sprintf("用户 %s 配额使用率仅 %.1f%%，可回收配额", uc.Username, uc.UsagePercent),
				CurrentCost:    uc.CostBreakdown.TotalMonthlyCost,
				OptimizedCost:  uc.CostBreakdown.TotalMonthlyCost * 0.5,
				Savings:        round(savings, 2),
				SavingsPercent: 50,
				Priority:       "low",
				Effort:         "easy",
				Impact:         "单个用户",
				Risk:           "低",
				Steps: []string{
					"1. 通知用户",
					"2. 调整配额限制",
					"3. 更新配额策略",
				},
				CreatedAt: now,
			})
		}
	}

	// 通用优化建议
	items = append(items, CostOptimizationItem{
		ID:             "opt_compress",
		Type:           "compression",
		Title:          "启用数据压缩",
		Description:    "对适合的数据类型启用压缩，减少存储占用",
		CurrentCost:    report.TotalCost.StorageCost,
		OptimizedCost:  report.TotalCost.StorageCost * 0.6,
		Savings:        round(report.TotalCost.StorageCost*0.4, 2),
		SavingsPercent: 40,
		Priority:       "medium",
		Effort:         "medium",
		Impact:         "系统级",
		Risk:           "低",
		Steps: []string{
			"1. 评估可压缩数据",
			"2. 选择压缩算法",
			"3. 分批启用压缩",
			"4. 监控压缩效果",
		},
		CreatedAt: now,
	})

	items = append(items, CostOptimizationItem{
		ID:             "opt_dedupe",
		Type:           "dedupe",
		Title:          "启用数据去重",
		Description:    "识别并删除重复数据，释放存储空间",
		CurrentCost:    report.TotalCost.StorageCost,
		OptimizedCost:  report.TotalCost.StorageCost * 0.7,
		Savings:        round(report.TotalCost.StorageCost*0.3, 2),
		SavingsPercent: 30,
		Priority:       "medium",
		Effort:         "medium",
		Impact:         "系统级",
		Risk:           "中",
		Steps: []string{
			"1. 扫描重复文件",
			"2. 生成去重报告",
			"3. 审核去重列表",
			"4. 执行去重操作",
		},
		CreatedAt: now,
	})

	// 按节省金额排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].Savings > items[j].Savings
	})

	return items
}

// generateRecommendations 生成建议
func (a *CostAnalyzer) generateRecommendations(report *CostAnalysisReport) []CostRecommendation {
	recs := make([]CostRecommendation, 0)

	// 基于总成本趋势
	if report.TrendAnalysis.TrendDirection == "up" && report.TrendAnalysis.MonthlyGrowthRate > 10 {
		recs = append(recs, CostRecommendation{
			Type:        "reduce",
			Priority:    "high",
			Title:       "成本增长过快",
			Description: "月度成本增长超过10%，建议审查存储使用情况",
			Savings:     report.TotalCost.TotalMonthlyCost * 0.1,
			Action:      "审查高成本用户和卷，实施清理或优化",
			Deadline:    "1个月内",
		})
	}

	// 基于优化项
	var potentialSavings float64
	for _, opt := range report.Optimization {
		potentialSavings += opt.Savings
	}

	if potentialSavings > report.TotalCost.TotalMonthlyCost*0.2 {
		recs = append(recs, CostRecommendation{
			Type:        "optimize",
			Priority:    "high",
			Title:       "存在显著优化空间",
			Description: "通过优化可节省超过20%的月度成本",
			Savings:     potentialSavings,
			Action:      "实施成本优化方案",
			Deadline:    "2个月内",
		})
	}

	// 基于预测
	if report.Forecast != nil && report.Forecast.NextMonthCost > report.TotalCost.TotalMonthlyCost*1.2 {
		recs = append(recs, CostRecommendation{
			Type:        "monitor",
			Priority:    "medium",
			Title:       "预计成本将持续上升",
			Description: "下月成本预计将超过当前20%以上",
			Savings:     0,
			Action:      "加强成本监控，准备扩容预算",
			Deadline:    "持续关注",
		})
	}

	return recs
}

// calculateSummary 计算摘要
func (a *CostAnalyzer) calculateSummary(report *CostAnalysisReport) CostAnalysisSummary {
	summary := CostAnalysisSummary{
		TotalMonthlyCost: report.TotalCost.TotalMonthlyCost,
		AvgCostPerGB:     report.TotalCost.CostPerGB,
	}

	// 计算总存储和使用量
	for _, vc := range report.VolumeCosts {
		summary.TotalStorageGB += vc.CapacityGB
		summary.TotalUsedGB += vc.UsedGB
		summary.AvgUsagePercent += vc.UsagePercent
	}

	if len(report.VolumeCosts) > 0 {
		summary.AvgUsagePercent = round(summary.AvgUsagePercent/float64(len(report.VolumeCosts)), 2)
	}

	// 计算潜在节省
	for _, opt := range report.Optimization {
		summary.PotentialSavings += opt.Savings
	}

	if summary.TotalMonthlyCost > 0 {
		summary.PotentialSavingsPercent = round(summary.PotentialSavings/summary.TotalMonthlyCost*100, 2)
	}

	// 计算健康评分
	summary.HealthScore = a.calculateHealthScore(report)

	// 确定状态
	if summary.HealthScore >= 80 {
		summary.Status = "healthy"
	} else if summary.HealthScore >= 60 {
		summary.Status = "warning"
	} else {
		summary.Status = "critical"
	}

	return summary
}

// calculateEfficiencyScore 计算效率评分
func (a *CostAnalyzer) calculateEfficiencyScore(usagePercent float64) float64 {
	// 使用率在 60-80% 之间效率最高
	if usagePercent >= 60 && usagePercent <= 80 {
		return 100
	}

	// 过低或过高都会降低效率评分
	if usagePercent < 60 {
		return round(usagePercent/60*80, 2)
	}

	// 过高
	return round((100-usagePercent)/20*50+50, 2)
}

// calculateCostEfficiency 计算成本效率
func (a *CostAnalyzer) calculateCostEfficiency(usagePercent float64) float64 {
	if usagePercent >= 50 && usagePercent <= 85 {
		return 100
	}

	if usagePercent < 50 {
		return round(usagePercent/50*70, 2)
	}

	return round((100-usagePercent)/15*50+50, 2)
}

// calculateHealthScore 计算健康评分
func (a *CostAnalyzer) calculateHealthScore(report *CostAnalysisReport) int {
	score := 100.0

	// 使用率过低扣分
	if report.Summary.AvgUsagePercent < 30 {
		score -= 20
	} else if report.Summary.AvgUsagePercent < 50 {
		score -= 10
	}

	// 使用率过高扣分
	if report.Summary.AvgUsagePercent > 90 {
		score -= 30
	} else if report.Summary.AvgUsagePercent > 80 {
		score -= 15
	}

	// 成本增长过快扣分
	if report.TrendAnalysis.MonthlyGrowthRate > 20 {
		score -= 20
	} else if report.TrendAnalysis.MonthlyGrowthRate > 10 {
		score -= 10
	}

	// 波动过大扣分
	if report.TrendAnalysis.Volatility > 30 {
		score -= 15
	} else if report.TrendAnalysis.Volatility > 20 {
		score -= 8
	}

	if score < 0 {
		score = 0
	}

	return int(score)
}