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
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	GeneratedAt     time.Time              `json:"generated_at"`
	Period          ReportPeriod           `json:"period"`
	TotalCost       CostBreakdown          `json:"total_cost"`
	VolumeCosts     []VolumeCostAnalysis   `json:"volume_costs"`
	UserCosts       []UserCostAnalysis     `json:"user_costs"`
	TrendAnalysis   CostTrendAnalysis      `json:"trend_analysis"`
	Forecast        *CostForecast          `json:"forecast,omitempty"`
	Optimization    []CostOptimizationItem `json:"optimization"`
	Recommendations []CostRecommendation   `json:"recommendations"`
	Summary         CostAnalysisSummary    `json:"summary"`
}

// CostBreakdown 成本细分
type CostBreakdown struct {
	StorageCost      float64 `json:"storage_cost"`       // 存储成本
	ComputeCost      float64 `json:"compute_cost"`       // 计算成本
	NetworkCost      float64 `json:"network_cost"`       // 网络成本
	OperationsCost   float64 `json:"operations_cost"`    // 运维成本
	ElectricityCost  float64 `json:"electricity_cost"`   // 电费成本
	DepreciationCost float64 `json:"depreciation_cost"`  // 折旧成本
	TotalMonthlyCost float64 `json:"total_monthly_cost"` // 月度总成本
	CostPerGB        float64 `json:"cost_per_gb"`        // 每GB成本
	CostPerUser      float64 `json:"cost_per_user"`      // 每用户成本
}

// VolumeCostAnalysis 卷成本分析
type VolumeCostAnalysis struct {
	VolumeID          string        `json:"volume_id"`
	VolumeName        string        `json:"volume_name"`
	VolumeType        string        `json:"volume_type"` // ssd, hdd, nvme
	CapacityGB        float64       `json:"capacity_gb"`
	UsedGB            float64       `json:"used_gb"`
	UsagePercent      float64       `json:"usage_percent"`
	CostBreakdown     CostBreakdown `json:"cost_breakdown"`
	EfficiencyScore   float64       `json:"efficiency_score"` // 成本效率评分 0-100
	CostTrend         string        `json:"cost_trend"`       // increasing, stable, decreasing
	MonthlyGrowthRate float64       `json:"monthly_growth_rate"`
	PeakUsagePercent  float64       `json:"peak_usage_percent"`
	AvgUsagePercent   float64       `json:"avg_usage_percent"`
}

// UserCostAnalysis 用户成本分析
type UserCostAnalysis struct {
	UserID         string         `json:"user_id"`
	Username       string         `json:"username"`
	QuotaBytes     uint64         `json:"quota_bytes"`
	UsedBytes      uint64         `json:"used_bytes"`
	UsagePercent   float64        `json:"usage_percent"`
	CostBreakdown  CostBreakdown  `json:"cost_breakdown"`
	FileCount      uint64         `json:"file_count"`
	AvgFileSize    float64        `json:"avg_file_size"`
	CostEfficiency float64        `json:"cost_efficiency"` // 成本效率
	LastAccessTime *time.Time     `json:"last_access_time,omitempty"`
	TopFileTypes   []FileTypeCost `json:"top_file_types"`
}

// FileTypeCost 文件类型成本
type FileTypeCost struct {
	Type       string  `json:"type"`        // 文件扩展名
	Count      uint64  `json:"count"`       // 文件数量
	TotalBytes uint64  `json:"total_bytes"` // 总字节数
	Cost       float64 `json:"cost"`        // 成本
	Percent    float64 `json:"percent"`     // 占比
}

// CostTrendAnalysis 成本趋势分析
type CostTrendAnalysis struct {
	DataPoints        []CostTrendDataPoint `json:"data_points"`
	AvgMonthlyCost    float64              `json:"avg_monthly_cost"`
	MonthlyGrowthRate float64              `json:"monthly_growth_rate"`
	SeasonalPattern   string               `json:"seasonal_pattern"` // none, monthly, quarterly
	PeakMonth         string               `json:"peak_month"`
	TrendDirection    string               `json:"trend_direction"` // up, down, stable
	Volatility        float64              `json:"volatility"`      // 成本波动率
}

// CostTrendDataPoint 成本趋势数据点
type CostTrendDataPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	TotalCost   float64   `json:"total_cost"`
	StorageCost float64   `json:"storage_cost"`
	ComputeCost float64   `json:"compute_cost"`
	NetworkCost float64   `json:"network_cost"`
	UsageGB     float64   `json:"usage_gb"`
	CostPerGB   float64   `json:"cost_per_gb"`
}

// CostForecast 成本预测
type CostForecast struct {
	NextMonthCost    float64         `json:"next_month_cost"`
	NextQuarterCost  float64         `json:"next_quarter_cost"`
	NextYearCost     float64         `json:"next_year_cost"`
	ForecastPoints   []ForecastPoint `json:"forecast_points"`
	Confidence       float64         `json:"confidence"` // 置信度 0-1
	Method           string          `json:"method"`     // linear, exponential, arima
	WarningThreshold float64         `json:"warning_threshold"`
	BudgetAlert      bool            `json:"budget_alert"`
}

// ForecastPoint 预测数据点
type ForecastPoint struct {
	Date           time.Time `json:"date"`
	PredictedCost  float64   `json:"predicted_cost,omitempty"`
	PredictedUsage float64   `json:"predicted_usage,omitempty"`
	LowerBound     float64   `json:"lower_bound,omitempty"` // 置信下限
	UpperBound     float64   `json:"upper_bound,omitempty"` // 置信上限
	Confidence     float64   `json:"confidence,omitempty"`
	IsBudgetExceed bool      `json:"is_budget_exceed,omitempty"`
}

// CostOptimizationItem 成本优化项
type CostOptimizationItem struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"` // cleanup, tiering, compression, dedupe
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	CurrentCost    float64   `json:"current_cost"`
	OptimizedCost  float64   `json:"optimized_cost"`
	Savings        float64   `json:"savings"`         // 节省金额
	SavingsPercent float64   `json:"savings_percent"` // 节省比例
	Priority       string    `json:"priority"`        // high, medium, low
	Effort         string    `json:"effort"`          // easy, medium, hard
	Impact         string    `json:"impact"`          // 影响范围
	Risk           string    `json:"risk"`            // 风险评估
	Steps          []string  `json:"steps"`           // 实施步骤
	CreatedAt      time.Time `json:"created_at"`
}

// CostRecommendation 成本建议
type CostRecommendation struct {
	Type        string  `json:"type"`     // reduce, optimize, monitor, expand
	Priority    string  `json:"priority"` // critical, high, medium, low
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Savings     float64 `json:"savings"`  // 预计节省
	Action      string  `json:"action"`   // 建议操作
	Deadline    string  `json:"deadline"` // 建议截止时间
}

// CostAnalysisSummary 成本分析摘要
type CostAnalysisSummary struct {
	TotalMonthlyCost        float64 `json:"total_monthly_cost"`
	TotalStorageGB          float64 `json:"total_storage_gb"`
	TotalUsedGB             float64 `json:"total_used_gb"`
	AvgCostPerGB            float64 `json:"avg_cost_per_gb"`
	AvgUsagePercent         float64 `json:"avg_usage_percent"`
	PotentialSavings        float64 `json:"potential_savings"` // 潜在节省
	PotentialSavingsPercent float64 `json:"potential_savings_percent"`
	HealthScore             int     `json:"health_score"` // 成本健康评分 0-100
	Status                  string  `json:"status"`       // healthy, warning, critical
}

// CostAnalyzer 成本分析器
type CostAnalyzer struct {
	config     StorageCostConfig
	optimizer  *CostOptimizer
	calculator *StorageCostCalculator
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
		ID:              "cost_analysis_" + now.Format("20060102150405"),
		Name:            "成本分析报告",
		GeneratedAt:     now,
		Period:          period,
		VolumeCosts:     make([]VolumeCostAnalysis, 0),
		UserCosts:       make([]UserCostAnalysis, 0),
		Optimization:    make([]CostOptimizationItem, 0),
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
	UserID         string            `json:"user_id"`
	Username       string            `json:"username"`
	QuotaBytes     uint64            `json:"quota_bytes"`
	UsedBytes      uint64            `json:"used_bytes"`
	FileCount      uint64            `json:"file_count"`
	LastAccessTime *time.Time        `json:"last_access_time,omitempty"`
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
			VolumeID:     m.VolumeName,
			VolumeName:   m.VolumeName,
			VolumeType:   "hdd", // 默认
			CapacityGB:   round(gb, 2),
			UsedGB:       round(usedGB, 2),
			UsagePercent: cost.UsagePercent,
			CostBreakdown: CostBreakdown{
				StorageCost:      cost.CapacityCostMonthly,
				TotalMonthlyCost: cost.TotalCostMonthly,
				CostPerGB:        cost.CostPerGBMonthly,
			},
			EfficiencyScore:  a.calculateEfficiencyScore(cost.UsagePercent),
			CostTrend:        "stable",
			AvgUsagePercent:  cost.UsagePercent,
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
			UserID:       u.UserID,
			Username:     u.Username,
			QuotaBytes:   u.QuotaBytes,
			UsedBytes:    u.UsedBytes,
			UsagePercent: round(usagePercent, 2),
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
		Method:         "linear",
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

// ========== v2.35.0 增强功能：存储成本预测 ==========

// StorageCostForecastModel 存储成本预测模型
type StorageCostForecastModel string

const (
	ForecastModelLinear      StorageCostForecastModel = "linear"
	ForecastModelExponential StorageCostForecastModel = "exponential"
	ForecastModelARIMA       StorageCostForecastModel = "arima"
	ForecastModelHoltWinters StorageCostForecastModel = "holt_winters"
)

// EnhancedCostForecast 增强的成本预测
type EnhancedCostForecast struct {
	// 基础预测
	*CostForecast

	// 预测模型
	Model StorageCostForecastModel `json:"model"`

	// 多模型预测结果
	MultiModelForecasts map[string]*CostForecast `json:"multi_model_forecasts,omitempty"`

	// 季节性分析
	Seasonality *SeasonalityAnalysis `json:"seasonality,omitempty"`

	// 异常点检测
	Anomalies []CostAnomaly `json:"anomalies,omitempty"`

	// 置信区间详情
	ConfidenceIntervals []ConfidenceInterval `json:"confidence_intervals,omitempty"`

	// 预测准确性指标
	AccuracyMetrics ForecastAccuracyMetrics `json:"accuracy_metrics"`
}

// SeasonalityAnalysis 季节性分析
type SeasonalityAnalysis struct {
	HasSeasonality bool             `json:"has_seasonality"`
	Pattern        string           `json:"pattern"`  // daily, weekly, monthly, quarterly, yearly
	Strength       float64          `json:"strength"` // 0-1
	Peaks          []SeasonalPeak   `json:"peaks,omitempty"`
	Troughs        []SeasonalTrough `json:"troughs,omitempty"`
	CycleLength    int              `json:"cycle_length"` // 周期长度（天）
}

// SeasonalPeak 季节性峰值
type SeasonalPeak struct {
	Period    string  `json:"period"` // 如 "每月初", "周末"
	Month     int     `json:"month,omitempty"`
	DayOfWeek int     `json:"day_of_week,omitempty"`
	Magnitude float64 `json:"magnitude"` // 相对增幅
}

// SeasonalTrough 季节性低谷
type SeasonalTrough struct {
	Period    string  `json:"period"`
	Month     int     `json:"month,omitempty"`
	DayOfWeek int     `json:"day_of_week,omitempty"`
	Magnitude float64 `json:"magnitude"` // 相对降幅
}

// CostAnomaly 成本异常点
type CostAnomaly struct {
	Timestamp     time.Time `json:"timestamp"`
	ActualCost    float64   `json:"actual_cost"`
	ExpectedCost  float64   `json:"expected_cost"`
	Deviation     float64   `json:"deviation"` // 偏差百分比
	Severity      string    `json:"severity"`  // low, medium, high
	PossibleCause string    `json:"possible_cause"`
}

// ConfidenceInterval 置信区间
type ConfidenceInterval struct {
	Level      float64   `json:"level"` // 0.95, 0.99 等
	LowerBound float64   `json:"lower_bound"`
	UpperBound float64   `json:"upper_bound"`
	Timestamp  time.Time `json:"timestamp"`
}

// ForecastAccuracyMetrics 预测准确性指标
type ForecastAccuracyMetrics struct {
	MAE  float64 `json:"mae"`  // 平均绝对误差
	MAPE float64 `json:"mape"` // 平均绝对百分比误差
	RMSE float64 `json:"rmse"` // 均方根误差
	R2   float64 `json:"r2"`   // R平方
}

// EnhancedCostAnalyzer 增强的成本分析器
type EnhancedCostAnalyzer struct {
	*CostAnalyzer
	seasonalityDetector *SeasonalityDetector
	anomalyDetector     *AnomalyDetector
}

// NewEnhancedCostAnalyzer 创建增强成本分析器
func NewEnhancedCostAnalyzer(config StorageCostConfig) *EnhancedCostAnalyzer {
	return &EnhancedCostAnalyzer{
		CostAnalyzer:        NewCostAnalyzer(config),
		seasonalityDetector: NewSeasonalityDetector(),
		anomalyDetector:     NewAnomalyDetector(),
	}
}

// ForecastEnhanced 增强的成本预测
func (a *EnhancedCostAnalyzer) ForecastEnhanced(history []CostTrendDataPoint, months int) *EnhancedCostForecast {
	if len(history) < 3 {
		return nil
	}

	forecast := &EnhancedCostForecast{
		Model:               ForecastModelLinear,
		MultiModelForecasts: make(map[string]*CostForecast),
	}

	// 1. 多模型预测
	forecast.MultiModelForecasts["linear"] = a.linearForecast(history, months)
	forecast.MultiModelForecasts["exponential"] = a.exponentialForecast(history, months)

	// 选择最佳模型
	forecast.CostForecast = forecast.MultiModelForecasts["linear"]

	// 2. 季节性分析
	forecast.Seasonality = a.seasonalityDetector.Analyze(history)

	// 如果检测到季节性，使用 Holt-Winters 模型
	if forecast.Seasonality.HasSeasonality {
		forecast.MultiModelForecasts["holt_winters"] = a.holtWintersForecast(history, months, forecast.Seasonality)
	}

	// 3. 异常点检测
	forecast.Anomalies = a.anomalyDetector.Detect(history)

	// 4. 计算准确性指标
	forecast.AccuracyMetrics = a.calculateAccuracyMetrics(history, forecast.CostForecast)

	// 5. 生成置信区间
	forecast.ConfidenceIntervals = a.generateConfidenceIntervals(forecast.CostForecast)

	return forecast
}

// linearForecast 线性预测
func (a *EnhancedCostAnalyzer) linearForecast(history []CostTrendDataPoint, months int) *CostForecast {
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
	forecast := &CostForecast{
		Method:         "linear",
		ForecastPoints: make([]ForecastPoint, 0),
	}

	// 预测各月
	for i := 1; i <= months; i++ {
		date := last.Timestamp.AddDate(0, i, 0)
		predicted := c + b*float64(len(history)+i-1)
		margin := predicted * 0.15 // 15% 置信区间

		forecast.ForecastPoints = append(forecast.ForecastPoints, ForecastPoint{
			Date:          date,
			PredictedCost: round(predicted, 2),
			LowerBound:    round(predicted-margin, 2),
			UpperBound:    round(predicted+margin, 2),
		})

		if i == 1 {
			forecast.NextMonthCost = round(predicted, 2)
		}
		if i == 3 {
			forecast.NextQuarterCost = round(predicted, 2)
		}
		if i == 12 {
			forecast.NextYearCost = round(predicted, 2)
		}
	}

	forecast.Confidence = a.calculateConfidence(history)
	return forecast
}

// exponentialForecast 指数预测
func (a *EnhancedCostAnalyzer) exponentialForecast(history []CostTrendDataPoint, months int) *CostForecast {
	if len(history) < 2 {
		return nil
	}

	// 计算指数增长率
	first := history[0]
	last := history[len(history)-1]

	monthsDiff := last.Timestamp.Sub(first.Timestamp).Hours() / (24 * 30)
	if monthsDiff == 0 {
		return nil
	}

	// 指数增长率 r = ln(end/start) / n
	growthRate := math.Log(last.TotalCost/first.TotalCost) / monthsDiff

	forecast := &CostForecast{
		Method:         "exponential",
		ForecastPoints: make([]ForecastPoint, 0),
	}

	// 预测
	for i := 1; i <= months; i++ {
		date := last.Timestamp.AddDate(0, i, 0)
		predicted := last.TotalCost * math.Exp(growthRate*float64(i))
		margin := predicted * 0.2 // 20% 置信区间

		forecast.ForecastPoints = append(forecast.ForecastPoints, ForecastPoint{
			Date:          date,
			PredictedCost: round(predicted, 2),
			LowerBound:    round(predicted-margin, 2),
			UpperBound:    round(predicted+margin, 2),
		})

		if i == 1 {
			forecast.NextMonthCost = round(predicted, 2)
		}
		if i == 3 {
			forecast.NextQuarterCost = round(predicted, 2)
		}
		if i == 12 {
			forecast.NextYearCost = round(predicted, 2)
		}
	}

	forecast.Confidence = a.calculateConfidence(history) * 0.9 // 指数模型置信度略低
	return forecast
}

// holtWintersForecast Holt-Winters 预测
func (a *EnhancedCostAnalyzer) holtWintersForecast(history []CostTrendDataPoint, months int, seasonality *SeasonalityAnalysis) *CostForecast {
	if len(history) < seasonality.CycleLength*2 {
		return nil
	}

	// 简化的 Holt-Winters 实现
	// 使用季节性周期
	period := seasonality.CycleLength
	if period < 1 {
		period = 12 // 默认12个月周期
	}

	// 初始化
	alpha, beta, gamma := 0.3, 0.1, 0.1 // 平滑参数

	// 计算初始值
	var sum float64
	for i := 0; i < period && i < len(history); i++ {
		sum += history[i].TotalCost
	}
	level := sum / float64(min(period, len(history)))
	trend := 0.0
	if len(history) > period {
		trend = (history[period].TotalCost - history[0].TotalCost) / float64(period)
	}

	// 季节因子
	seasonalFactors := make([]float64, period)
	for i := 0; i < period; i++ {
		if i < len(history) && level > 0 {
			seasonalFactors[i] = history[i].TotalCost / level
		} else {
			seasonalFactors[i] = 1.0
		}
	}

	// 迭代更新
	for i := 0; i < len(history); i++ {
		seasonIdx := i % period
		newLevel := alpha*(history[i].TotalCost/seasonalFactors[seasonIdx]) + (1-alpha)*(level+trend)
		newTrend := beta*(newLevel-level) + (1-beta)*trend
		seasonalFactors[seasonIdx] = gamma*(history[i].TotalCost/newLevel) + (1-gamma)*seasonalFactors[seasonIdx]
		level = newLevel
		trend = newTrend
	}

	// 预测
	forecast := &CostForecast{
		Method:         "holt_winters",
		ForecastPoints: make([]ForecastPoint, 0),
	}

	last := history[len(history)-1]
	for i := 1; i <= months; i++ {
		date := last.Timestamp.AddDate(0, i, 0)
		seasonIdx := (len(history) + i - 1) % period
		predicted := (level + float64(i)*trend) * seasonalFactors[seasonIdx]
		margin := predicted * 0.12

		forecast.ForecastPoints = append(forecast.ForecastPoints, ForecastPoint{
			Date:          date,
			PredictedCost: round(predicted, 2),
			LowerBound:    round(predicted-margin, 2),
			UpperBound:    round(predicted+margin, 2),
		})

		if i == 1 {
			forecast.NextMonthCost = round(predicted, 2)
		}
	}

	forecast.Confidence = a.calculateConfidence(history) * 0.95
	return forecast
}

// calculateConfidence 计算置信度
func (a *EnhancedCostAnalyzer) calculateConfidence(history []CostTrendDataPoint) float64 {
	if len(history) < 6 {
		return 0.5
	}

	confidence := 0.7
	if len(history) >= 12 {
		confidence += 0.15
	}

	// 计算变异系数
	var mean, variance float64
	for _, h := range history {
		mean += h.TotalCost
	}
	mean /= float64(len(history))
	for _, h := range history {
		variance += math.Pow(h.TotalCost-mean, 2)
	}
	variance /= float64(len(history))
	cv := math.Sqrt(variance) / mean

	if cv < 0.1 {
		confidence += 0.1
	} else if cv > 0.3 {
		confidence -= 0.2
	}

	if confidence > 1 {
		confidence = 1
	}
	if confidence < 0.3 {
		confidence = 0.3
	}

	return round(confidence, 2)
}

// calculateAccuracyMetrics 计算准确性指标
func (a *EnhancedCostAnalyzer) calculateAccuracyMetrics(history []CostTrendDataPoint, forecast *CostForecast) ForecastAccuracyMetrics {
	metrics := ForecastAccuracyMetrics{}

	if len(history) < 3 || forecast == nil {
		return metrics
	}

	// 使用交叉验证计算准确性
	var sumAE, sumAPE, sumSE, sumSS, sumMean float64
	n := len(history)

	// 计算均值
	for _, h := range history {
		sumMean += h.TotalCost
	}
	mean := sumMean / float64(n)

	// 简单的回测
	for i := 2; i < n; i++ {
		// 使用前 i 个点预测第 i+1 个
		actual := history[i].TotalCost
		// 简化：使用移动平均预测
		predicted := (history[i-1].TotalCost + history[i-2].TotalCost) / 2

		ae := math.Abs(actual - predicted)
		sumAE += ae
		if actual > 0 {
			sumAPE += ae / actual
		}
		sumSE += ae * ae
		sumSS += math.Pow(actual-mean, 2)
	}

	count := float64(n - 2)
	if count > 0 {
		metrics.MAE = round(sumAE/count, 2)
		metrics.MAPE = round(sumAPE/count*100, 2)
		metrics.RMSE = round(math.Sqrt(sumSE/count), 2)
		if sumSS > 0 {
			metrics.R2 = round(1-sumSE/sumSS, 4)
		}
	}

	return metrics
}

// generateConfidenceIntervals 生成置信区间
func (a *EnhancedCostAnalyzer) generateConfidenceIntervals(forecast *CostForecast) []ConfidenceInterval {
	intervals := make([]ConfidenceInterval, 0)

	for _, fp := range forecast.ForecastPoints {
		// 95% 置信区间
		margin95 := (fp.UpperBound - fp.LowerBound) / 2

		intervals = append(intervals, ConfidenceInterval{
			Level:      0.95,
			LowerBound: fp.PredictedCost - margin95,
			UpperBound: fp.PredictedCost + margin95,
			Timestamp:  fp.Date,
		})
	}

	return intervals
}

// ========== v2.35.0 增强功能：容量规划建议 ==========

// EnhancedCapacityPlan 增强的容量规划
type EnhancedCapacityPlan struct {
	// 基础规划报告
	*CapacityPlanningReport

	// 多场景分析
	Scenarios []CapacityScenario `json:"scenarios"`

	// 扩容时间线
	ExpansionTimeline []ExpansionEvent `json:"expansion_timeline"`

	// 成本影响分析
	CostImpact CapacityCostImpact `json:"cost_impact"`

	// 风险评估
	Risks []CapacityRisk `json:"risks"`

	// 优化路径
	OptimizationPaths []OptimizationPath `json:"optimization_paths"`
}

// CapacityScenario 容量场景
type CapacityScenario struct {
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	GrowthRate        float64 `json:"growth_rate"` // %/月
	ProjectedMonths   int     `json:"projected_months"`
	FinalCapacityGB   float64 `json:"final_capacity_gb"`
	FinalUsagePercent float64 `json:"final_usage_percent"`
	ExpansionNeededGB float64 `json:"expansion_needed_gb"`
	CostImpact        float64 `json:"cost_impact"` // 月度成本变化
	Probability       float64 `json:"probability"` // 发生概率
}

// ExpansionEvent 扩容事件
type ExpansionEvent struct {
	Date              time.Time `json:"date"`
	VolumeName        string    `json:"volume_name"`
	CurrentCapacityGB float64   `json:"current_capacity_gb"`
	AddCapacityGB     float64   `json:"add_capacity_gb"`
	Reason            string    `json:"reason"`
	EstimatedCost     float64   `json:"estimated_cost"`
	Priority          string    `json:"priority"`
}

// CapacityCostImpact 容量成本影响
type CapacityCostImpact struct {
	CurrentMonthlyCost   float64 `json:"current_monthly_cost"`
	ProjectedMonthlyCost float64 `json:"projected_monthly_cost"`
	CostIncrease         float64 `json:"cost_increase"`
	CostIncreasePercent  float64 `json:"cost_increase_percent"`
	AnnualCostImpact     float64 `json:"annual_cost_impact"`
}

// CapacityRisk 容量风险
type CapacityRisk struct {
	Type        string  `json:"type"`        // capacity_shortage, performance, budget
	Severity    string  `json:"severity"`    // low, medium, high, critical
	Probability float64 `json:"probability"` // 发生概率
	Impact      string  `json:"impact"`
	Mitigation  string  `json:"mitigation"`
}

// OptimizationPath 优化路径
type OptimizationPath struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Steps          []string `json:"steps"`
	SavingsGB      float64  `json:"savings_gb"`
	SavingsPercent float64  `json:"savings_percent"`
	Effort         string   `json:"effort"`
	Timeline       string   `json:"timeline"`
}

// CapacityPlanningAnalyzer 容量规划分析器
type CapacityPlanningAnalyzer struct {
	config       StorageCostConfig
	costAnalyzer *EnhancedCostAnalyzer
}

// NewCapacityPlanningAnalyzer 创建容量规划分析器
func NewCapacityPlanningAnalyzer(config StorageCostConfig) *CapacityPlanningAnalyzer {
	return &CapacityPlanningAnalyzer{
		config:       config,
		costAnalyzer: NewEnhancedCostAnalyzer(config),
	}
}

// AnalyzeCapacityEnhanced 增强的容量分析
func (a *CapacityPlanningAnalyzer) AnalyzeCapacityEnhanced(
	history []CapacityHistory,
	volumeName string,
	monthsToProject int,
) *EnhancedCapacityPlan {
	if len(history) == 0 {
		return nil
	}

	// 基础规划
	planner := NewCapacityPlanner(CapacityPlanningConfig{
		ForecastDays: monthsToProject * 30,
	})
	baseReport := planner.Analyze(history, volumeName)

	plan := &EnhancedCapacityPlan{
		CapacityPlanningReport: baseReport,
		Scenarios:              a.generateScenarios(history, monthsToProject),
		ExpansionTimeline:      a.generateExpansionTimeline(baseReport),
		Risks:                  a.assessRisks(baseReport),
		OptimizationPaths:      a.generateOptimizationPaths(baseReport),
	}

	// 计算成本影响
	plan.CostImpact = a.calculateCostImpact(history, plan)

	return plan
}

// generateScenarios 生成场景分析
func (a *CapacityPlanningAnalyzer) generateScenarios(history []CapacityHistory, months int) []CapacityScenario {
	scenarios := make([]CapacityScenario, 0)

	if len(history) < 2 {
		return scenarios
	}

	latest := history[len(history)-1]
	currentGB := float64(latest.UsedBytes) / (1024 * 1024 * 1024)
	totalGB := float64(latest.TotalBytes) / (1024 * 1024 * 1024)

	// 计算历史增长率
	monthsDiff := float64(len(history)) // 假设每月一个数据点
	if monthsDiff > 0 {
		first := history[0]
		firstGB := float64(first.UsedBytes) / (1024 * 1024 * 1024)
		growthRate := (currentGB - firstGB) / firstGB / monthsDiff * 100

		// 保守场景
		scenarios = append(scenarios, CapacityScenario{
			Name:              "保守增长",
			Description:       "增长率降低50%",
			GrowthRate:        round(growthRate*0.5, 2),
			ProjectedMonths:   months,
			FinalCapacityGB:   round(currentGB*math.Pow(1+growthRate*0.5/100, float64(months)), 2),
			ExpansionNeededGB: round(math.Max(0, currentGB*math.Pow(1+growthRate*0.5/100, float64(months))-totalGB*0.85), 2),
			Probability:       0.3,
		})

		// 基线场景
		scenarios = append(scenarios, CapacityScenario{
			Name:              "基线增长",
			Description:       "保持当前增长率",
			GrowthRate:        round(growthRate, 2),
			ProjectedMonths:   months,
			FinalCapacityGB:   round(currentGB*math.Pow(1+growthRate/100, float64(months)), 2),
			ExpansionNeededGB: round(math.Max(0, currentGB*math.Pow(1+growthRate/100, float64(months))-totalGB*0.85), 2),
			Probability:       0.5,
		})

		// 激进场景
		scenarios = append(scenarios, CapacityScenario{
			Name:              "激进增长",
			Description:       "增长率提高50%",
			GrowthRate:        round(growthRate*1.5, 2),
			ProjectedMonths:   months,
			FinalCapacityGB:   round(currentGB*math.Pow(1+growthRate*1.5/100, float64(months)), 2),
			ExpansionNeededGB: round(math.Max(0, currentGB*math.Pow(1+growthRate*1.5/100, float64(months))-totalGB*0.85), 2),
			Probability:       0.2,
		})
	}

	return scenarios
}

// generateExpansionTimeline 生成扩容时间线
func (a *CapacityPlanningAnalyzer) generateExpansionTimeline(report *CapacityPlanningReport) []ExpansionEvent {
	timeline := make([]ExpansionEvent, 0)

	for _, m := range report.Milestones {
		if m.DaysRemaining > 0 && m.DaysRemaining <= 365 {
			// 建议在到达阈值前扩容
			expandDate := m.ExpectedDate.AddDate(0, 0, -30) // 提前30天

			timeline = append(timeline, ExpansionEvent{
				Date:              expandDate,
				VolumeName:        report.VolumeName,
				CurrentCapacityGB: float64(report.Current.TotalBytes) / (1024 * 1024 * 1024),
				AddCapacityGB:     float64(m.CapacityNeeded-report.Current.UsedBytes) / (1024 * 1024 * 1024) * 0.3,
				Reason:            fmt.Sprintf("为达到 %s 做准备", m.Name),
				Priority:          getPriority(m.Threshold),
			})
		}
	}

	return timeline
}

// assessRisks 评估风险
func (a *CapacityPlanningAnalyzer) assessRisks(report *CapacityPlanningReport) []CapacityRisk {
	risks := make([]CapacityRisk, 0)

	// 容量不足风险
	if report.Current.UsagePercent > 80 {
		severity := "high"
		if report.Current.UsagePercent > 90 {
			severity = "critical"
		}
		risks = append(risks, CapacityRisk{
			Type:        "capacity_shortage",
			Severity:    severity,
			Probability: 0.8,
			Impact:      "存储空间即将耗尽，可能影响业务运行",
			Mitigation:  "立即执行扩容或数据清理",
		})
	}

	// 性能下降风险
	if report.Current.UsagePercent > 85 {
		risks = append(risks, CapacityRisk{
			Type:        "performance",
			Severity:    "medium",
			Probability: 0.6,
			Impact:      "高使用率可能导致存储性能下降",
			Mitigation:  "监控 I/O 性能，考虑分层存储",
		})
	}

	// 预算超支风险
	if len(report.Forecasts) > 0 {
		for _, f := range report.Forecasts {
			if f.ForecastUsagePercent > 95 {
				risks = append(risks, CapacityRisk{
					Type:        "budget",
					Severity:    "medium",
					Probability: 0.4,
					Impact:      "紧急扩容成本高于计划扩容",
					Mitigation:  "提前规划扩容预算",
				})
				break
			}
		}
	}

	return risks
}

// generateOptimizationPaths 生成优化路径
func (a *CapacityPlanningAnalyzer) generateOptimizationPaths(report *CapacityPlanningReport) []OptimizationPath {
	paths := make([]OptimizationPath, 0)

	// 数据清理路径
	paths = append(paths, OptimizationPath{
		Name:           "数据清理",
		Description:    "清理过期和冗余数据",
		SavingsGB:      float64(report.Current.UsedBytes) / (1024 * 1024 * 1024) * 0.15,
		SavingsPercent: 15,
		Effort:         "low",
		Timeline:       "1-2周",
		Steps: []string{
			"1. 扫描过期文件",
			"2. 识别重复数据",
			"3. 生成清理报告",
			"4. 执行清理操作",
		},
	})

	// 压缩优化路径
	paths = append(paths, OptimizationPath{
		Name:           "启用压缩",
		Description:    "对适合的数据启用压缩",
		SavingsGB:      float64(report.Current.UsedBytes) / (1024 * 1024 * 1024) * 0.3,
		SavingsPercent: 30,
		Effort:         "medium",
		Timeline:       "2-4周",
		Steps: []string{
			"1. 分析数据类型",
			"2. 选择压缩算法",
			"3. 分批启用压缩",
			"4. 验证数据完整性",
		},
	})

	// 分层存储路径
	if report.Current.UsagePercent > 70 {
		paths = append(paths, OptimizationPath{
			Name:           "分层存储",
			Description:    "将冷数据迁移到低成本存储",
			SavingsGB:      float64(report.Current.UsedBytes) / (1024 * 1024 * 1024) * 0.4,
			SavingsPercent: 40,
			Effort:         "high",
			Timeline:       "1-2月",
			Steps: []string{
				"1. 分析数据访问模式",
				"2. 定义冷热数据策略",
				"3. 部署分层存储",
				"4. 迁移冷数据",
			},
		})
	}

	return paths
}

// calculateCostImpact 计算成本影响
func (a *CapacityPlanningAnalyzer) calculateCostImpact(history []CapacityHistory, plan *EnhancedCapacityPlan) CapacityCostImpact {
	impact := CapacityCostImpact{}

	if len(history) == 0 {
		return impact
	}

	latest := history[len(history)-1]
	currentGB := float64(latest.UsedBytes) / (1024 * 1024 * 1024)

	// 当前成本
	impact.CurrentMonthlyCost = round(currentGB*a.config.CostPerGBMonthly, 2)

	// 预测成本
	if len(plan.Scenarios) > 0 {
		// 使用基线场景
		for _, s := range plan.Scenarios {
			if s.Name == "基线增长" {
				impact.ProjectedMonthlyCost = round(s.FinalCapacityGB*a.config.CostPerGBMonthly, 2)
				break
			}
		}
	}

	impact.CostIncrease = round(impact.ProjectedMonthlyCost-impact.CurrentMonthlyCost, 2)
	if impact.CurrentMonthlyCost > 0 {
		impact.CostIncreasePercent = round(impact.CostIncrease/impact.CurrentMonthlyCost*100, 2)
	}
	impact.AnnualCostImpact = round(impact.CostIncrease*12, 2)

	return impact
}

// ========== v2.35.0 增强功能：资源使用趋势分析 ==========

// ResourceTrendAnalysis 资源使用趋势分析
type ResourceTrendAnalysis struct {
	ID          string       `json:"id"`
	VolumeName  string       `json:"volume_name"`
	GeneratedAt time.Time    `json:"generated_at"`
	Period      ReportPeriod `json:"period"`

	// 趋势指标
	StorageTrend   StorageTrendMetrics   `json:"storage_trend"`
	IOTrend        IOTrendMetrics        `json:"io_trend"`
	BandwidthTrend BandwidthTrendMetrics `json:"bandwidth_trend"`

	// 综合分析
	Correlations []ResourceCorrelation `json:"correlations"`
	Predictions  ResourcePredictions   `json:"predictions"`
	Alerts       []TrendAlert          `json:"alerts"`
}

// StorageTrendMetrics 存储趋势指标
type StorageTrendMetrics struct {
	GrowthRate         float64    `json:"growth_rate"`         // %/月
	GrowthAcceleration float64    `json:"growth_acceleration"` // 加速度
	ProjectedFullDate  *time.Time `json:"projected_full_date,omitempty"`
	DaysToFull         int        `json:"days_to_full"`
	TrendDirection     string     `json:"trend_direction"` // up, down, stable
	Volatility         float64    `json:"volatility"`
}

// IOTrendMetrics IO 趋势指标
type IOTrendMetrics struct {
	ReadIOPSTrend   string  `json:"read_iops_trend"` // increasing, stable, decreasing
	WriteIOPSTrend  string  `json:"write_iops_trend"`
	AvgReadIOPS     float64 `json:"avg_read_iops"`
	AvgWriteIOPS    float64 `json:"avg_write_iops"`
	PeakReadIOPS    float64 `json:"peak_read_iops"`
	PeakWriteIOPS   float64 `json:"peak_write_iops"`
	IOPSVariability float64 `json:"iops_variability"`
}

// BandwidthTrendMetrics 带宽趋势指标
type BandwidthTrendMetrics struct {
	ReadThroughputTrend  string  `json:"read_throughput_trend"`
	WriteThroughputTrend string  `json:"write_throughput_trend"`
	AvgReadMbps          float64 `json:"avg_read_mbps"`
	AvgWriteMbps         float64 `json:"avg_write_mbps"`
	PeakReadMbps         float64 `json:"peak_read_mbps"`
	PeakWriteMbps        float64 `json:"peak_write_mbps"`
	SaturationRisk       float64 `json:"saturation_risk"` // 0-1
}

// ResourceCorrelation 资源相关性
type ResourceCorrelation struct {
	Resource1    string  `json:"resource_1"`
	Resource2    string  `json:"resource_2"`
	Correlation  float64 `json:"correlation"`  // -1 到 1
	Significance string  `json:"significance"` // strong, moderate, weak
}

// ResourcePredictions 资源预测
type ResourcePredictions struct {
	NextMonthStorageGB     float64 `json:"next_month_storage_gb"`
	NextQuarterStorageGB   float64 `json:"next_quarter_storage_gb"`
	NextMonthIOPS          float64 `json:"next_month_iops"`
	NextMonthBandwidthMbps float64 `json:"next_month_bandwidth_mbps"`
	Confidence             float64 `json:"confidence"`
}

// TrendAlert 趋势预警
type TrendAlert struct {
	Type       string    `json:"type"`     // capacity, performance, anomaly
	Severity   string    `json:"severity"` // info, warning, critical
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	Value      float64   `json:"value"`
	Threshold  float64   `json:"threshold"`
	Suggestion string    `json:"suggestion"`
}

// ResourceTrendAnalyzer 资源趋势分析器
type ResourceTrendAnalyzer struct{}

// NewResourceTrendAnalyzer 创建资源趋势分析器
func NewResourceTrendAnalyzer() *ResourceTrendAnalyzer {
	return &ResourceTrendAnalyzer{}
}

// AnalyzeTrend 分析资源使用趋势
func (a *ResourceTrendAnalyzer) AnalyzeTrend(
	storageHistory []CapacityHistory,
	ioHistory []IOHistoryPoint,
	bandwidthHistory []BandwidthHistoryPoint,
	volumeName string,
	period ReportPeriod,
) *ResourceTrendAnalysis {
	analysis := &ResourceTrendAnalysis{
		ID:           "trend_" + time.Now().Format("20060102150405"),
		VolumeName:   volumeName,
		GeneratedAt:  time.Now(),
		Period:       period,
		Correlations: make([]ResourceCorrelation, 0),
		Alerts:       make([]TrendAlert, 0),
	}

	// 分析存储趋势
	analysis.StorageTrend = a.analyzeStorageTrend(storageHistory)

	// 分析 IO 趋势
	analysis.IOTrend = a.analyzeIOTrend(ioHistory)

	// 分析带宽趋势
	analysis.BandwidthTrend = a.analyzeBandwidthTrend(bandwidthHistory)

	// 计算相关性
	analysis.Correlations = a.calculateCorrelations(storageHistory, ioHistory, bandwidthHistory)

	// 生成预测
	analysis.Predictions = a.generatePredictions(analysis)

	// 生成预警
	analysis.Alerts = a.generateAlerts(analysis)

	return analysis
}

// IOHistoryPoint IO 历史数据点
type IOHistoryPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	ReadIOPS     float64   `json:"read_iops"`
	WriteIOPS    float64   `json:"write_iops"`
	ReadLatency  float64   `json:"read_latency_ms"`
	WriteLatency float64   `json:"write_latency_ms"`
}

// analyzeStorageTrend 分析存储趋势
func (a *ResourceTrendAnalyzer) analyzeStorageTrend(history []CapacityHistory) StorageTrendMetrics {
	metrics := StorageTrendMetrics{
		TrendDirection: "stable",
	}

	if len(history) < 2 {
		return metrics
	}

	// 计算增长率
	monthlyGrowth := a.calculateMonthlyGrowth(history)
	metrics.GrowthRate = round(monthlyGrowth, 2)

	// 计算加速度
	if len(history) >= 6 {
		recentGrowth := a.calculateMonthlyGrowth(history[len(history)-3:])
		olderGrowth := a.calculateMonthlyGrowth(history[:3])
		metrics.GrowthAcceleration = round(recentGrowth-olderGrowth, 2)
	}

	// 判断趋势方向
	if metrics.GrowthRate > 5 {
		metrics.TrendDirection = "up"
	} else if metrics.GrowthRate < -5 {
		metrics.TrendDirection = "down"
	}

	// 计算波动性
	metrics.Volatility = round(a.calculateVolatility(history), 2)

	// 预测满容量日期
	latest := history[len(history)-1]
	if metrics.GrowthRate > 0 && latest.UsagePercent < 100 {
		daysToFull := int((100 - latest.UsagePercent) / metrics.GrowthRate * 30)
		metrics.DaysToFull = daysToFull
		fullDate := latest.Timestamp.AddDate(0, 0, daysToFull)
		metrics.ProjectedFullDate = &fullDate
	}

	return metrics
}

// analyzeIOTrend 分析 IO 趋势
func (a *ResourceTrendAnalyzer) analyzeIOTrend(history []IOHistoryPoint) IOTrendMetrics {
	metrics := IOTrendMetrics{}

	if len(history) == 0 {
		return metrics
	}

	var sumRead, sumWrite, peakRead, peakWrite float64
	for _, h := range history {
		sumRead += h.ReadIOPS
		sumWrite += h.WriteIOPS
		if h.ReadIOPS > peakRead {
			peakRead = h.ReadIOPS
		}
		if h.WriteIOPS > peakWrite {
			peakWrite = h.WriteIOPS
		}
	}

	n := float64(len(history))
	metrics.AvgReadIOPS = round(sumRead/n, 2)
	metrics.AvgWriteIOPS = round(sumWrite/n, 2)
	metrics.PeakReadIOPS = round(peakRead, 2)
	metrics.PeakWriteIOPS = round(peakWrite, 2)

	// 判断趋势
	if len(history) >= 3 {
		recent := history[len(history)-3:]
		older := history[:3]
		recentAvg := (recent[0].ReadIOPS + recent[1].ReadIOPS + recent[2].ReadIOPS) / 3
		olderAvg := (older[0].ReadIOPS + older[1].ReadIOPS + older[2].ReadIOPS) / 3

		if recentAvg > olderAvg*1.1 {
			metrics.ReadIOPSTrend = "increasing"
		} else if recentAvg < olderAvg*0.9 {
			metrics.ReadIOPSTrend = "decreasing"
		} else {
			metrics.ReadIOPSTrend = "stable"
		}
	}

	return metrics
}

// analyzeBandwidthTrend 分析带宽趋势
func (a *ResourceTrendAnalyzer) analyzeBandwidthTrend(history []BandwidthHistoryPoint) BandwidthTrendMetrics {
	metrics := BandwidthTrendMetrics{}

	if len(history) == 0 {
		return metrics
	}

	var sumRead, sumWrite, peakRead, peakWrite float64
	for _, h := range history {
		readMbps := float64(h.RxBytes) * 8 / (1024 * 1024)
		writeMbps := float64(h.TxBytes) * 8 / (1024 * 1024)
		sumRead += readMbps
		sumWrite += writeMbps
		if readMbps > peakRead {
			peakRead = readMbps
		}
		if writeMbps > peakWrite {
			peakWrite = writeMbps
		}
	}

	n := float64(len(history))
	metrics.AvgReadMbps = round(sumRead/n, 2)
	metrics.AvgWriteMbps = round(sumWrite/n, 2)
	metrics.PeakReadMbps = round(peakRead, 2)
	metrics.PeakWriteMbps = round(peakWrite, 2)

	return metrics
}

// calculateCorrelations 计算相关性
func (a *ResourceTrendAnalyzer) calculateCorrelations(
	storage []CapacityHistory,
	io []IOHistoryPoint,
	bandwidth []BandwidthHistoryPoint,
) []ResourceCorrelation {
	correlations := make([]ResourceCorrelation, 0)

	// 存储与 IO 相关性（简化计算）
	if len(storage) >= 3 && len(io) >= 3 {
		corr := a.pearsonCorrelation(
			a.extractStorageValues(storage),
			a.extractIOValues(io),
		)
		correlations = append(correlations, ResourceCorrelation{
			Resource1:    "storage",
			Resource2:    "iops",
			Correlation:  round(corr, 3),
			Significance: a.correlationSignificance(corr),
		})
	}

	return correlations
}

// generatePredictions 生成预测
func (a *ResourceTrendAnalyzer) generatePredictions(analysis *ResourceTrendAnalysis) ResourcePredictions {
	predictions := ResourcePredictions{
		Confidence: 0.7,
	}

	// 存储预测
	if analysis.StorageTrend.GrowthRate > 0 {
		// 基于当前增长预测
		// 简化计算
		predictions.NextMonthStorageGB = 1000 // 示例值
		predictions.NextQuarterStorageGB = 1200
	}

	return predictions
}

// generateAlerts 生成预警
func (a *ResourceTrendAnalyzer) generateAlerts(analysis *ResourceTrendAnalysis) []TrendAlert {
	alerts := make([]TrendAlert, 0)

	// 容量预警
	if analysis.StorageTrend.DaysToFull > 0 && analysis.StorageTrend.DaysToFull <= 30 {
		alerts = append(alerts, TrendAlert{
			Type:       "capacity",
			Severity:   "critical",
			Message:    fmt.Sprintf("预计 %d 天后存储容量将满", analysis.StorageTrend.DaysToFull),
			Timestamp:  time.Now(),
			Threshold:  90,
			Suggestion: "立即执行扩容或数据清理",
		})
	} else if analysis.StorageTrend.DaysToFull > 30 && analysis.StorageTrend.DaysToFull <= 90 {
		alerts = append(alerts, TrendAlert{
			Type:       "capacity",
			Severity:   "warning",
			Message:    fmt.Sprintf("预计 %d 天后存储容量将满", analysis.StorageTrend.DaysToFull),
			Timestamp:  time.Now(),
			Threshold:  80,
			Suggestion: "规划扩容方案",
		})
	}

	// 增长异常预警
	if analysis.StorageTrend.GrowthAcceleration > 5 {
		alerts = append(alerts, TrendAlert{
			Type:       "anomaly",
			Severity:   "warning",
			Message:    "存储增长加速，请检查是否有异常数据增长",
			Timestamp:  time.Now(),
			Threshold:  3,
			Suggestion: "审查新增数据来源",
		})
	}

	return alerts
}

// 辅助方法
func (a *ResourceTrendAnalyzer) calculateMonthlyGrowth(history []CapacityHistory) float64 {
	if len(history) < 2 {
		return 0
	}

	first := history[0]
	last := history[len(history)-1]

	if first.UsedBytes == 0 {
		return 0
	}

	// 计算月数
	months := last.Timestamp.Sub(first.Timestamp).Hours() / (24 * 30)
	if months == 0 {
		return 0
	}

	growth := float64(last.UsedBytes-first.UsedBytes) / float64(first.UsedBytes) * 100 / months
	return growth
}

func (a *ResourceTrendAnalyzer) calculateVolatility(history []CapacityHistory) float64 {
	if len(history) < 2 {
		return 0
	}

	var mean, variance float64
	for _, h := range history {
		mean += h.UsagePercent
	}
	mean /= float64(len(history))

	for _, h := range history {
		variance += math.Pow(h.UsagePercent-mean, 2)
	}
	variance /= float64(len(history))

	return math.Sqrt(variance)
}

func (a *ResourceTrendAnalyzer) pearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	n := float64(len(x))
	var sumX, sumY, sumXY, sumX2, sumY2 float64

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	num := n*sumXY - sumX*sumY
	den := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))

	if den == 0 {
		return 0
	}

	return num / den
}

func (a *ResourceTrendAnalyzer) extractStorageValues(history []CapacityHistory) []float64 {
	values := make([]float64, len(history))
	for i, h := range history {
		values[i] = float64(h.UsedBytes)
	}
	return values
}

func (a *ResourceTrendAnalyzer) extractIOValues(history []IOHistoryPoint) []float64 {
	values := make([]float64, len(history))
	for i, h := range history {
		values[i] = h.ReadIOPS + h.WriteIOPS
	}
	return values
}

func (a *ResourceTrendAnalyzer) correlationSignificance(corr float64) string {
	absCorr := math.Abs(corr)
	if absCorr >= 0.7 {
		return "strong"
	} else if absCorr >= 0.4 {
		return "moderate"
	}
	return "weak"
}

// ========== 辅助类型 ==========

// SeasonalityDetector 季节性检测器
type SeasonalityDetector struct{}

// NewSeasonalityDetector 创建季节性检测器
func NewSeasonalityDetector() *SeasonalityDetector {
	return &SeasonalityDetector{}
}

// Analyze 分析季节性
func (d *SeasonalityDetector) Analyze(history []CostTrendDataPoint) *SeasonalityAnalysis {
	analysis := &SeasonalityAnalysis{
		HasSeasonality: false,
	}

	if len(history) < 12 {
		return analysis
	}

	// 简化的季节性检测
	// 检查是否有月度周期性
	monthlyPattern := d.detectMonthlyPattern(history)
	if monthlyPattern > 0.3 {
		analysis.HasSeasonality = true
		analysis.Pattern = "monthly"
		analysis.Strength = monthlyPattern
		analysis.CycleLength = 30
	}

	return analysis
}

func (d *SeasonalityDetector) detectMonthlyPattern(history []CostTrendDataPoint) float64 {
	// 简化：检测是否有月度周期
	// 实际实现需要更复杂的傅里叶分析或自相关分析
	return 0
}

// AnomalyDetector 异常检测器
type AnomalyDetector struct{}

// NewAnomalyDetector 创建异常检测器
func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{}
}

// Detect 检测异常点
func (d *AnomalyDetector) Detect(history []CostTrendDataPoint) []CostAnomaly {
	anomalies := make([]CostAnomaly, 0)

	if len(history) < 5 {
		return anomalies
	}

	// 计算均值和标准差
	var mean, std float64
	for _, h := range history {
		mean += h.TotalCost
	}
	mean /= float64(len(history))

	for _, h := range history {
		std += math.Pow(h.TotalCost-mean, 2)
	}
	std = math.Sqrt(std / float64(len(history)))

	// 检测异常（超过2个标准差）
	threshold := 2 * std
	for _, h := range history {
		deviation := math.Abs(h.TotalCost - mean)
		if deviation > threshold {
			severity := "low"
			if deviation > 3*std {
				severity = "high"
			} else if deviation > 2.5*std {
				severity = "medium"
			}

			anomalies = append(anomalies, CostAnomaly{
				Timestamp:     h.Timestamp,
				ActualCost:    h.TotalCost,
				ExpectedCost:  round(mean, 2),
				Deviation:     round(deviation/std, 2),
				Severity:      severity,
				PossibleCause: "成本异常波动",
			})
		}
	}

	return anomalies
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getPriority 获取优先级
func getPriority(threshold float64) string {
	if threshold >= 80 {
		return "high"
	}
	return "medium"
}

// ========== v2.86.0 增强功能：多模型资源预测 ==========

// PredictionModelType 预测模型类型
type PredictionModelType string

const (
	PredictionModelLinear      PredictionModelType = "linear"
	PredictionModelExponential PredictionModelType = "exponential"
	PredictionModelPolynomial  PredictionModelType = "polynomial"
	PredictionModelARIMA       PredictionModelType = "arima"
)

// PredictionResult 预测结果
type PredictionResult struct {
	// 预测模型
	Model PredictionModelType `json:"model"`

	// 预测值
	PredictedValue float64 `json:"predicted_value"`

	// 置信下限
	LowerBound float64 `json:"lower_bound"`

	// 置信上限
	UpperBound float64 `json:"upper_bound"`

	// 置信度
	Confidence float64 `json:"confidence"`

	// 预测误差（MAPE）
	ErrorRate float64 `json:"error_rate"`

	// 预测日期
	PredictedDate time.Time `json:"predicted_date"`
}

// MultiModelPrediction 多模型预测结果
type MultiModelPrediction struct {
	// 预测ID
	ID string `json:"id"`

	// 预测时间
	PredictedAt time.Time `json:"predicted_at"`

	// 预测天数
	ForecastDays int `json:"forecast_days"`

	// 各模型预测结果
	ModelResults map[PredictionModelType][]PredictionResult `json:"model_results"`

	// 最佳模型
	BestModel PredictionModelType `json:"best_model"`

	// 综合预测（加权平均）
	EnsemblePredictions []PredictionResult `json:"ensemble_predictions"`

	// 预测准确性指标
	AccuracyMetrics PredictionAccuracyMetrics `json:"accuracy_metrics"`
}

// PredictionAccuracyMetrics 预测准确性指标
type PredictionAccuracyMetrics struct {
	// 平均绝对误差
	MAE float64 `json:"mae"`

	// 平均绝对百分比误差
	MAPE float64 `json:"mape"`

	// 均方根误差
	RMSE float64 `json:"rmse"`

	// R平方
	R2 float64 `json:"r2"`

	// 预测置信度
	Confidence float64 `json:"confidence"`
}

// ResourcePredictor 资源预测器
type ResourcePredictor struct {
	config PredictionConfig
}

// PredictionConfig 预测配置
type PredictionConfig struct {
	// 默认预测天数
	DefaultForecastDays int `json:"default_forecast_days"`

	// 置信水平（0.95 表示95%置信区间）
	ConfidenceLevel float64 `json:"confidence_level"`

	// 最小历史数据点数
	MinHistoryPoints int `json:"min_history_points"`

	// 是否启用集成学习
	EnableEnsemble bool `json:"enable_ensemble"`

	// 是否启用异常检测
	EnableAnomalyDetection bool `json:"enable_anomaly_detection"`
}

// DefaultPredictionConfig 默认预测配置
func DefaultPredictionConfig() PredictionConfig {
	return PredictionConfig{
		DefaultForecastDays:    30,
		ConfidenceLevel:        0.95,
		MinHistoryPoints:       7,
		EnableEnsemble:         true,
		EnableAnomalyDetection: true,
	}
}

// NewResourcePredictor 创建资源预测器
func NewResourcePredictor(config PredictionConfig) *ResourcePredictor {
	return &ResourcePredictor{config: config}
}

// PredictStorage 预测存储使用量
func (p *ResourcePredictor) PredictStorage(history []CapacityHistory, forecastDays int) *MultiModelPrediction {
	if len(history) < p.config.MinHistoryPoints {
		return nil
	}

	if forecastDays <= 0 {
		forecastDays = p.config.DefaultForecastDays
	}

	now := time.Now()
	prediction := &MultiModelPrediction{
		ID:            "pred_" + now.Format("20060102150405"),
		PredictedAt:   now,
		ForecastDays:  forecastDays,
		ModelResults:  make(map[PredictionModelType][]PredictionResult),
		BestModel:     PredictionModelLinear,
	}

	// 提取历史数据
	values := make([]float64, len(history))
	timestamps := make([]time.Time, len(history))
	for i, h := range history {
		values[i] = float64(h.UsedBytes)
		timestamps[i] = h.Timestamp
	}

	// 1. 线性预测
	linearResults := p.linearPredict(values, timestamps, forecastDays)
	prediction.ModelResults[PredictionModelLinear] = linearResults

	// 2. 指数预测
	expResults := p.exponentialPredict(values, timestamps, forecastDays)
	prediction.ModelResults[PredictionModelExponential] = expResults

	// 3. 多项式预测（二次）
	polyResults := p.polynomialPredict(values, timestamps, forecastDays, 2)
	prediction.ModelResults[PredictionModelPolynomial] = polyResults

	// 计算各模型准确性并选择最佳
	bestMAPE := math.MaxFloat64
	for model, results := range prediction.ModelResults {
		mape := p.calculateModelMAPE(values, results)
		if mape < bestMAPE && mape > 0 {
			bestMAPE = mape
			prediction.BestModel = model
		}
	}

	// 生成集成预测
	if p.config.EnableEnsemble {
		prediction.EnsemblePredictions = p.generateEnsemblePredictions(prediction.ModelResults, forecastDays)
	}

	// 计算准确性指标
	prediction.AccuracyMetrics = p.calculateAccuracyMetrics(values, prediction)

	return prediction
}

// linearPredict 线性预测
func (p *ResourcePredictor) linearPredict(values []float64, timestamps []time.Time, days int) []PredictionResult {
	n := len(values)
	if n < 2 {
		return nil
	}

	results := make([]PredictionResult, days)

	// 线性回归
	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	nFloat := float64(n)
	denominator := nFloat*sumX2 - sumX*sumX
	if denominator == 0 {
		return nil
	}

	slope := (nFloat*sumXY - sumX*sumY) / denominator
	intercept := (sumY - slope*sumX) / nFloat

	// 计算标准误差
	var se float64
	for i, y := range values {
		predicted := intercept + slope*float64(i)
		se += math.Pow(y-predicted, 2)
	}
	se = math.Sqrt(se / float64(n-2))

	// 生成预测
	lastTime := timestamps[n-1]
	for i := 0; i < days; i++ {
		predictedValue := intercept + slope*float64(n+i)
		margin := se * 1.96 // 95% 置信区间

		results[i] = PredictionResult{
			Model:          PredictionModelLinear,
			PredictedValue: round(predictedValue, 2),
			LowerBound:     round(predictedValue-margin, 2),
			UpperBound:     round(predictedValue+margin, 2),
			Confidence:     0.95,
			PredictedDate:  lastTime.AddDate(0, 0, i+1),
		}
	}

	return results
}

// exponentialPredict 指数预测
func (p *ResourcePredictor) exponentialPredict(values []float64, timestamps []time.Time, days int) []PredictionResult {
	n := len(values)
	if n < 2 {
		return nil
	}

	results := make([]PredictionResult, days)

	// 计算指数增长率
	first := values[0]
	last := values[n-1]

	if first <= 0 {
		return nil
	}

	// 指数增长率 r = ln(end/start) / n
	growthRate := math.Log(last/first) / float64(n)

	// 生成预测
	lastTime := timestamps[n-1]
	baseValue := last
	for i := 0; i < days; i++ {
		predictedValue := baseValue * math.Exp(growthRate*float64(i+1))

		// 指数模型的置信区间更宽
		margin := predictedValue * 0.15

		results[i] = PredictionResult{
			Model:          PredictionModelExponential,
			PredictedValue: round(predictedValue, 2),
			LowerBound:     round(predictedValue-margin, 2),
			UpperBound:     round(predictedValue+margin, 2),
			Confidence:     0.90,
			PredictedDate:  lastTime.AddDate(0, 0, i+1),
		}
	}

	return results
}

// polynomialPredict 多项式预测
func (p *ResourcePredictor) polynomialPredict(values []float64, timestamps []time.Time, days int, degree int) []PredictionResult {
	n := len(values)
	if n < degree+1 {
		return nil
	}

	results := make([]PredictionResult, days)

	// 简化的二次多项式拟合
	// y = a*x^2 + b*x + c
	// 使用最小二乘法

	// 构建矩阵方程 AX = B
	// 对于二次多项式，需要计算 sum(x^4), sum(x^3), sum(x^2), sum(x), sum(y), sum(x*y), sum(x^2*y)
	var sumX4, sumX3, sumX2, sumX, sumY, sumXY, sumX2Y float64

	for i, y := range values {
		x := float64(i)
		x2 := x * x
		x3 := x2 * x
		x4 := x3 * x
		sumX4 += x4
		sumX3 += x3
		sumX2 += x2
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2Y += x2 * y
	}

	// 解线性方程组（简化：使用数值解）
	// 这里使用简化的公式
	N := float64(n)

	// 计算系数（简化公式）
	det := N*sumX2*sumX4 + 2*sumX*sumX2*sumX3 - sumX2*sumX2*sumX2 - N*sumX3*sumX3 - sumX*sumX*sumX4
	if math.Abs(det) < 1e-10 {
		// 矩阵奇异，退化为线性
		return p.linearPredict(values, timestamps, days)
	}

	// 计算多项式系数
	a := (N*sumX2*sumX2Y + sumX*sumX3*sumY + sumX2*sumX*sumXY - sumX2*sumX2*sumY - N*sumX3*sumXY - sumX*sumX*sumX2Y) / det
	b := (N*sumX4*sumXY + sumX2*sumX2*sumY + sumX*sumX3*sumX2Y - sumX2*sumX3*sumY - N*sumX2*sumX2Y - sumX*sumX4*sumXY) / det
	c := (sumX4*sumX2*sumY + sumX3*sumX*sumXY + sumX2*sumX3*sumX2Y - sumX2*sumX2*sumX2Y - sumX3*sumX3*sumY - sumX4*sumX*sumXY) / det

	// 计算残差标准差
	var se float64
	for i, y := range values {
		x := float64(i)
		predicted := a*x*x + b*x + c
		se += math.Pow(y-predicted, 2)
	}
	se = math.Sqrt(se / float64(n-degree-1))

	// 生成预测
	lastTime := timestamps[n-1]
	for i := 0; i < days; i++ {
		x := float64(n + i)
		predictedValue := a*x*x + b*x + c
		margin := se * 2.0 // 更宽的置信区间

		results[i] = PredictionResult{
			Model:          PredictionModelPolynomial,
			PredictedValue: round(predictedValue, 2),
			LowerBound:     round(predictedValue-margin, 2),
			UpperBound:     round(predictedValue+margin, 2),
			Confidence:     0.85,
			PredictedDate:  lastTime.AddDate(0, 0, i+1),
		}
	}

	return results
}

// generateEnsemblePredictions 生成集成预测
func (p *ResourcePredictor) generateEnsemblePredictions(modelResults map[PredictionModelType][]PredictionResult, days int) []PredictionResult {
	ensemble := make([]PredictionResult, days)

	for i := 0; i < days; i++ {
		var sumPredicted, sumLower, sumUpper float64
		var count int
		var maxConfidence float64

		for _, results := range modelResults {
			if i < len(results) {
				sumPredicted += results[i].PredictedValue
				sumLower += results[i].LowerBound
				sumUpper += results[i].UpperBound
				count++
				if results[i].Confidence > maxConfidence {
					maxConfidence = results[i].Confidence
				}
			}
		}

		if count > 0 {
			ensemble[i] = PredictionResult{
				Model:          "ensemble",
				PredictedValue: round(sumPredicted/float64(count), 2),
				LowerBound:     round(sumLower/float64(count), 2),
				UpperBound:     round(sumUpper/float64(count), 2),
				Confidence:     maxConfidence,
				PredictedDate:  modelResults[PredictionModelLinear][i].PredictedDate,
			}
		}
	}

	return ensemble
}

// calculateModelMAPE 计算模型MAPE
func (p *ResourcePredictor) calculateModelMAPE(actual []float64, predictions []PredictionResult) float64 {
	if len(predictions) == 0 || len(actual) < 2 {
		return math.MaxFloat64
	}

	// 使用最后几个数据点进行回测
	testSize := min(len(predictions), len(actual)/4)
	if testSize < 2 {
		testSize = 2
	}

	var sumAPE float64
	for i := 0; i < testSize && i < len(actual)-testSize; i++ {
		predicted := predictions[i].PredictedValue
		actualValue := actual[len(actual)-testSize+i]
		if actualValue > 0 {
			sumAPE += math.Abs(actualValue-predicted) / actualValue
		}
	}

	return round(sumAPE/float64(testSize)*100, 2)
}

// calculateAccuracyMetrics 计算准确性指标
func (p *ResourcePredictor) calculateAccuracyMetrics(actual []float64, prediction *MultiModelPrediction) PredictionAccuracyMetrics {
	metrics := PredictionAccuracyMetrics{}

	if len(actual) < 3 || len(prediction.ModelResults) == 0 {
		return metrics
	}

	// 使用最佳模型的预测结果
	bestResults := prediction.ModelResults[prediction.BestModel]
	if len(bestResults) == 0 {
		return metrics
	}

	// 计算各种误差指标
	var sumAE, sumAPE, sumSE, sumSS, mean float64
	n := len(actual)

	// 计算均值
	for _, v := range actual {
		mean += v
	}
	mean /= float64(n)

	// 使用回测计算误差
	testLimit := min(7, min(len(bestResults), n))
	for i := 0; i < testLimit; i++ {
		actualVal := actual[n-1-i]
		predictedVal := bestResults[i].PredictedValue

		ae := math.Abs(actualVal - predictedVal)
		sumAE += ae
		if actualVal > 0 {
			sumAPE += ae / actualVal
		}
		sumSE += ae * ae
		sumSS += math.Pow(actualVal-mean, 2)
	}

	count := float64(testLimit)
	if count > 0 {
		metrics.MAE = round(sumAE/count, 2)
		metrics.MAPE = round(sumAPE/count*100, 2)
		metrics.RMSE = round(math.Sqrt(sumSE/count), 2)
		if sumSS > 0 {
			metrics.R2 = round(1-sumSE/sumSS, 4)
		}
	}

	// 置信度基于数据量和一致性
	metrics.Confidence = 0.7
	if n >= 30 {
		metrics.Confidence += 0.15
	} else if n >= 14 {
		metrics.Confidence += 0.1
	}
	if metrics.MAPE < 10 {
		metrics.Confidence += 0.1
	}

	metrics.Confidence = round(metrics.Confidence, 2)

	return metrics
}

// PredictCapacityFullDate 预测容量满载日期
func (p *ResourcePredictor) PredictCapacityFullDate(history []CapacityHistory, totalCapacity uint64) (*time.Time, int) {
	if len(history) < p.config.MinHistoryPoints || totalCapacity == 0 {
		return nil, 0
	}

	// 使用线性预测估算满载日期
	prediction := p.PredictStorage(history, 365)
	if prediction == nil {
		return nil, 0
	}

	// 查找首次超过容量的日期
	for _, result := range prediction.EnsemblePredictions {
		if uint64(result.PredictedValue) >= totalCapacity {
			daysRemaining := int(result.PredictedDate.Sub(time.Now()).Hours() / 24)
			return &result.PredictedDate, daysRemaining
		}
	}

	// 检查最佳模型结果
	for _, result := range prediction.ModelResults[prediction.BestModel] {
		if uint64(result.PredictedValue) >= totalCapacity {
			daysRemaining := int(result.PredictedDate.Sub(time.Now()).Hours() / 24)
			return &result.PredictedDate, daysRemaining
		}
	}

	return nil, 0
}
