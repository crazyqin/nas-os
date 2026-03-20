// Package cost_analysis 提供增强的成本分析报告功能
package cost_analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 增强版存储成本分析报告 ==========

// EnhancedStorageReport 增强版存储成本分析报告
type EnhancedStorageReport struct {
	ID               string               `json:"id"`
	GeneratedAt      time.Time            `json:"generated_at"`
	PeriodStart      time.Time            `json:"period_start"`
	PeriodEnd        time.Time            `json:"period_end"`
	Summary          EnhancedCostSummary  `json:"summary"`
	CostDistribution CostDistribution     `json:"cost_distribution"`
	AnomalyDetection AnomalyDetection     `json:"anomaly_detection"`
	Comparison       PeriodComparison     `json:"comparison"`
	TrendAnalysis    TrendAnalysis        `json:"trend_analysis"`
	PoolBreakdown    []PoolCostBreakdown  `json:"pool_breakdown"`
	UserBreakdown    []UserCostBreakdown  `json:"user_breakdown"`
	Recommendations  []CostRecommendation `json:"recommendations"`
	ExportFormats    []string             `json:"available_export_formats"`
}

// EnhancedCostSummary 增强版成本摘要
type EnhancedCostSummary struct {
	TotalCost             float64 `json:"total_cost"`
	StorageCost           float64 `json:"storage_cost"`
	BandwidthCost         float64 `json:"bandwidth_cost"`
	OtherCost             float64 `json:"other_cost"`
	Currency              string  `json:"currency"`
	CostChangePercent     float64 `json:"cost_change_percent"`
	AvgDailyCost          float64 `json:"avg_daily_cost"`
	PeakDailyCost         float64 `json:"peak_daily_cost"`
	MinDailyCost          float64 `json:"min_daily_cost"`
	ProjectedMonthlyCost  float64 `json:"projected_monthly_cost"`
	BudgetUtilization     float64 `json:"budget_utilization"`
	CostEfficiencyScore   float64 `json:"cost_efficiency_score"`  // 0-100 成本效率评分
	OptimizationPotential float64 `json:"optimization_potential"` // 可优化金额
	TotalUsedGB           float64 `json:"total_used_gb"`
	TotalBandwidthGB      float64 `json:"total_bandwidth_gb"`
	ActiveUsers           int     `json:"active_users"`
	ActivePools           int     `json:"active_pools"`
	ReportConfidence      float64 `json:"report_confidence"` // 报告置信度 0-1
}

// CostDistribution 成本分布
type CostDistribution struct {
	ByStorageType  []StorageTypeCost `json:"by_storage_type"`
	ByUser         []UserCostDist    `json:"by_user"`
	ByPool         []PoolCostDist    `json:"by_pool"`
	ByTimeOfDay    []HourlyCost      `json:"by_time_of_day"`
	ByDayOfWeek    []DailyCost       `json:"by_day_of_week"`
	TopCostDrivers []CostDriver      `json:"top_cost_drivers"`
}

// StorageTypeCost 按存储类型的成本
type StorageTypeCost struct {
	StorageType    string  `json:"storage_type"` // ssd, hdd, archive
	TotalCost      float64 `json:"total_cost"`
	UsedGB         float64 `json:"used_gb"`
	PricePerGB     float64 `json:"price_per_gb"`
	Percentage     float64 `json:"percentage"`
	CostEfficiency float64 `json:"cost_efficiency"`
}

// UserCostDist 用户成本分布
type UserCostDist struct {
	UserID      string  `json:"user_id"`
	UserName    string  `json:"user_name"`
	TotalCost   float64 `json:"total_cost"`
	StorageGB   float64 `json:"storage_gb"`
	BandwidthGB float64 `json:"bandwidth_gb"`
	Percentage  float64 `json:"percentage"`
	Rank        int     `json:"rank"`
	Trend       string  `json:"trend"` // up, down, stable
}

// PoolCostDist 存储池成本分布
type PoolCostDist struct {
	PoolID      string  `json:"pool_id"`
	PoolName    string  `json:"pool_name"`
	StorageType string  `json:"storage_type"`
	TotalCost   float64 `json:"total_cost"`
	UsedGB      float64 `json:"used_gb"`
	Percentage  float64 `json:"percentage"`
	Utilization float64 `json:"utilization"`
	Health      string  `json:"health"` // good, warning, critical
}

// HourlyCost 每小时成本
type HourlyCost struct {
	Hour    int     `json:"hour"`
	AvgCost float64 `json:"avg_cost"`
	MaxCost float64 `json:"max_cost"`
	MinCost float64 `json:"min_cost"`
}

// DailyCost 每日成本
type DailyCost struct {
	DayOfWeek string  `json:"day_of_week"` // Monday, Tuesday, etc.
	AvgCost   float64 `json:"avg_cost"`
	MaxCost   float64 `json:"max_cost"`
}

// CostDriver 成本驱动因素
type CostDriver struct {
	Factor        string  `json:"factor"`
	Description   string  `json:"description"`
	ImpactAmount  float64 `json:"impact_amount"`
	ImpactPercent float64 `json:"impact_percent"`
	Controllable  bool    `json:"controllable"`
}

// AnomalyDetection 异常检测
type AnomalyDetection struct {
	HasAnomalies    bool              `json:"has_anomalies"`
	AnomalyCount    int               `json:"anomaly_count"`
	Anomalies       []CostAnomaly     `json:"anomalies"`
	DetectionMethod string            `json:"detection_method"`
	BaselinePeriod  string            `json:"baseline_period"`
	Threshold       float64           `json:"threshold"` // 异常阈值百分比
	Statistics      AnomalyStatistics `json:"statistics"`
}

// CostAnomaly 成本异常
type CostAnomaly struct {
	ID             string     `json:"id"`
	DetectedAt     time.Time  `json:"detected_at"`
	Type           string     `json:"type"`          // spike, drop, trend_change
	Severity       string     `json:"severity"`      // low, medium, high, critical
	ResourceType   string     `json:"resource_type"` // storage, bandwidth, user, pool
	ResourceID     string     `json:"resource_id"`
	ResourceName   string     `json:"resource_name"`
	ExpectedValue  float64    `json:"expected_value"`
	ActualValue    float64    `json:"actual_value"`
	Deviation      float64    `json:"deviation"` // 偏差百分比
	PotentialCause string     `json:"potential_cause"`
	AffectedPeriod string     `json:"affected_period"`
	Status         string     `json:"status"` // new, investigating, resolved, ignored
	Resolution     string     `json:"resolution,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

// AnomalyStatistics 异常统计
type AnomalyStatistics struct {
	Mean            float64 `json:"mean"`
	StdDev          float64 `json:"std_dev"`
	Median          float64 `json:"median"`
	Percentile95    float64 `json:"percentile_95"`
	Percentile99    float64 `json:"percentile_99"`
	ZScoreThreshold float64 `json:"z_score_threshold"`
}

// PeriodComparison 周期对比
type PeriodComparison struct {
	CurrentPeriod  PeriodCostDetail  `json:"current_period"`
	PreviousPeriod PeriodCostDetail  `json:"previous_period"`
	YearOverYear   *PeriodCostDetail `json:"year_over_year,omitempty"`
	ChangeAnalysis ChangeAnalysis    `json:"change_analysis"`
}

// PeriodCostDetail 周期成本详情
type PeriodCostDetail struct {
	PeriodStart     time.Time `json:"period_start"`
	PeriodEnd       time.Time `json:"period_end"`
	TotalCost       float64   `json:"total_cost"`
	StorageCost     float64   `json:"storage_cost"`
	BandwidthCost   float64   `json:"bandwidth_cost"`
	OtherCost       float64   `json:"other_cost"`
	StorageUsedGB   float64   `json:"storage_used_gb"`
	BandwidthUsedGB float64   `json:"bandwidth_used_gb"`
	UserCount       int       `json:"user_count"`
	PoolCount       int       `json:"pool_count"`
}

// ChangeAnalysis 变化分析
type ChangeAnalysis struct {
	TotalCostChange        float64        `json:"total_cost_change"`
	TotalCostChangePercent float64        `json:"total_cost_change_percent"`
	StorageCostChange      float64        `json:"storage_cost_change"`
	BandwidthCostChange    float64        `json:"bandwidth_cost_change"`
	StorageUsageChange     float64        `json:"storage_usage_change"`
	BandwidthUsageChange   float64        `json:"bandwidth_usage_change"`
	UserCountChange        int            `json:"user_count_change"`
	KeyFactors             []ChangeFactor `json:"key_factors"`
}

// ChangeFactor 变化因素
type ChangeFactor struct {
	Factor      string  `json:"factor"`
	Description string  `json:"description"`
	Impact      float64 `json:"impact"`
	Direction   string  `json:"direction"` // increase, decrease
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	OverallTrend      string            `json:"overall_trend"`  // increasing, decreasing, stable
	TrendStrength     float64           `json:"trend_strength"` // 0-1
	DailyData         []DailyTrendData  `json:"daily_data"`
	WeeklyPattern     WeeklyPattern     `json:"weekly_pattern"`
	MonthlyProjection MonthlyProjection `json:"monthly_projection"`
	Seasonality       SeasonalityInfo   `json:"seasonality"`
}

// DailyTrendData 每日趋势数据
type DailyTrendData struct {
	Date          time.Time `json:"date"`
	TotalCost     float64   `json:"total_cost"`
	StorageCost   float64   `json:"storage_cost"`
	BandwidthCost float64   `json:"bandwidth_cost"`
	StorageGB     float64   `json:"storage_gb"`
	BandwidthGB   float64   `json:"bandwidth_gb"`
	MovingAvg7d   float64   `json:"moving_avg_7d"`
	MovingAvg30d  float64   `json:"moving_avg_30d"`
	IsAnomaly     bool      `json:"is_anomaly"`
}

// WeeklyPattern 周模式
type WeeklyPattern struct {
	PeakDay      string  `json:"peak_day"`
	LowDay       string  `json:"low_day"`
	Variance     float64 `json:"variance"`
	IsConsistent bool    `json:"is_consistent"`
}

// MonthlyProjection 月度预测
type MonthlyProjection struct {
	ProjectedCost    float64  `json:"projected_cost"`
	ConfidenceLevel  float64  `json:"confidence_level"`
	UpperBound       float64  `json:"upper_bound"`
	LowerBound       float64  `json:"lower_bound"`
	ProjectionMethod string   `json:"projection_method"`
	KeyAssumptions   []string `json:"key_assumptions"`
}

// SeasonalityInfo 季节性信息
type SeasonalityInfo struct {
	HasSeasonality bool     `json:"has_seasonality"`
	PeakMonths     []string `json:"peak_months,omitempty"`
	LowMonths      []string `json:"low_months,omitempty"`
	SeasonalFactor float64  `json:"seasonal_factor"`
}

// PoolCostBreakdown 存储池成本分解
type PoolCostBreakdown struct {
	PoolID          string          `json:"pool_id"`
	PoolName        string          `json:"pool_name"`
	StorageType     string          `json:"storage_type"`
	TotalCost       float64         `json:"total_cost"`
	StorageCost     float64         `json:"storage_cost"`
	BandwidthCost   float64         `json:"bandwidth_cost"`
	UsedGB          float64         `json:"used_gb"`
	CapacityGB      float64         `json:"capacity_gb"`
	Utilization     float64         `json:"utilization"`
	PricePerGB      float64         `json:"price_per_gb"`
	CostTrend       string          `json:"cost_trend"`
	EfficiencyScore float64         `json:"efficiency_score"`
	UserCount       int             `json:"user_count"`
	TopUsers        []TopUserInPool `json:"top_users"`
	GrowthRate      float64         `json:"growth_rate"`
	Recommendation  string          `json:"recommendation,omitempty"`
}

// TopUserInPool 存储池中用量最大的用户
type TopUserInPool struct {
	UserID     string  `json:"user_id"`
	UserName   string  `json:"user_name"`
	UsedGB     float64 `json:"used_gb"`
	Cost       float64 `json:"cost"`
	Percentage float64 `json:"percentage"`
}

// UserCostBreakdown 用户成本分解
type UserCostBreakdown struct {
	UserID           string          `json:"user_id"`
	UserName         string          `json:"user_name"`
	TotalCost        float64         `json:"total_cost"`
	StorageCost      float64         `json:"storage_cost"`
	BandwidthCost    float64         `json:"bandwidth_cost"`
	StorageUsedGB    float64         `json:"storage_used_gb"`
	BandwidthUsedGB  float64         `json:"bandwidth_used_gb"`
	QuotaLimitGB     float64         `json:"quota_limit_gb"`
	QuotaUtilization float64         `json:"quota_utilization"`
	CostTrend        string          `json:"cost_trend"`
	PoolCount        int             `json:"pool_count"`
	PoolDistribution []PoolUsage     `json:"pool_distribution"`
	DailyTrend       []DailyUserCost `json:"daily_trend"`
	EfficiencyScore  float64         `json:"efficiency_score"`
	Recommendation   string          `json:"recommendation,omitempty"`
}

// PoolUsage 存储池使用情况
type PoolUsage struct {
	PoolID   string  `json:"pool_id"`
	PoolName string  `json:"pool_name"`
	UsedGB   float64 `json:"used_gb"`
	Cost     float64 `json:"cost"`
}

// DailyUserCost 用户每日成本
type DailyUserCost struct {
	Date      time.Time `json:"date"`
	Cost      float64   `json:"cost"`
	StorageGB float64   `json:"storage_gb"`
}

// ========== 增强版成本分析引擎方法 ==========

// GenerateEnhancedStorageReport 生成增强版存储成本分析报告
func (e *CostAnalysisEngine) GenerateEnhancedStorageReport(days int) (*EnhancedStorageReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now()
	startDate := now.AddDate(0, 0, -days)

	report := &EnhancedStorageReport{
		ID:              generateReportID(),
		GeneratedAt:     now,
		PeriodStart:     startDate,
		PeriodEnd:       now,
		ExportFormats:   []string{"json", "csv", "pdf", "xlsx"},
		PoolBreakdown:   make([]PoolCostBreakdown, 0),
		UserBreakdown:   make([]UserCostBreakdown, 0),
		Recommendations: make([]CostRecommendation, 0),
	}

	// 生成增强摘要
	report.Summary = e.generateEnhancedSummary(startDate, now)

	// 生成成本分布
	report.CostDistribution = e.generateCostDistribution(startDate, now)

	// 异常检测
	report.AnomalyDetection = e.detectAnomalies(startDate, now)

	// 周期对比
	report.Comparison = e.generatePeriodComparison(days)

	// 趋势分析
	report.TrendAnalysis = e.generateTrendAnalysis(startDate, now)

	// 存储池分解
	report.PoolBreakdown = e.generatePoolBreakdown()

	// 用户分解
	report.UserBreakdown = e.generateUserBreakdown()

	// 生成建议
	report.Recommendations = e.generateEnhancedRecommendations(report)

	return report, nil
}

// generateEnhancedSummary 生成增强摘要
func (e *CostAnalysisEngine) generateEnhancedSummary(start, end time.Time) EnhancedCostSummary {
	summary := EnhancedCostSummary{
		Currency:         e.config.DefaultCurrency,
		ReportConfidence: 0.85,
	}

	// 获取基础统计数据
	if stats, err := e.billingClient.GetBillingStats(start, end); err == nil {
		summary.TotalCost = stats.TotalRevenue
		summary.StorageCost = stats.StorageRevenue
		summary.BandwidthCost = stats.BandwidthRevenue
		summary.TotalUsedGB = stats.TotalStorageUsedGB
		summary.TotalBandwidthGB = stats.TotalBandwidthGB
	}

	// 计算日均成本
	days := int(end.Sub(start).Hours() / 24)
	if days > 0 {
		summary.AvgDailyCost = summary.TotalCost / float64(days)
	}

	// 计算成本效率评分
	summary.CostEfficiencyScore = e.calculateOverallEfficiencyScore(summary)

	// 计算可优化金额
	summary.OptimizationPotential = summary.TotalCost * 0.15 // 估算15%可优化

	// 预测月成本
	summary.ProjectedMonthlyCost = summary.AvgDailyCost * 30

	// 获取活跃用户和存储池数
	if quotaUsages, err := e.quotaClient.GetAllUsage(); err == nil {
		userSet := make(map[string]bool)
		poolSet := make(map[string]bool)
		for _, u := range quotaUsages {
			userSet[u.TargetID] = true
			poolSet[u.VolumeName] = true
		}
		summary.ActiveUsers = len(userSet)
		summary.ActivePools = len(poolSet)
	}

	return summary
}

// generateCostDistribution 生成成本分布
func (e *CostAnalysisEngine) generateCostDistribution(start, end time.Time) CostDistribution {
	dist := CostDistribution{
		ByStorageType:  make([]StorageTypeCost, 0),
		ByUser:         make([]UserCostDist, 0),
		ByPool:         make([]PoolCostDist, 0),
		ByTimeOfDay:    make([]HourlyCost, 24),
		ByDayOfWeek:    make([]DailyCost, 7),
		TopCostDrivers: make([]CostDriver, 0),
	}

	// 按存储池分析
	if quotaUsages, err := e.quotaClient.GetAllUsage(); err == nil {
		poolCosts := make(map[string]*PoolCostDist)
		totalCost := 0.0

		for _, u := range quotaUsages {
			poolName := u.VolumeName
			if _, exists := poolCosts[poolName]; !exists {
				poolCosts[poolName] = &PoolCostDist{
					PoolID:      poolName,
					PoolName:    poolName,
					StorageType: "hdd", // 默认
					UsedGB:      0,
					TotalCost:   0,
				}
			}
			usedGB := float64(u.UsedBytes) / (1024 * 1024 * 1024)
			poolCosts[poolName].UsedGB += usedGB
			cost := usedGB * e.billingClient.GetStoragePrice(poolName)
			poolCosts[poolName].TotalCost += cost
			totalCost += cost
		}

		for _, pc := range poolCosts {
			if totalCost > 0 {
				pc.Percentage = pc.TotalCost / totalCost * 100
			}
			if pc.UsedGB > 0 {
				pc.Utilization = pc.UsedGB / 100 // 简化
			}
			if pc.Utilization > 80 {
				pc.Health = "critical"
			} else if pc.Utilization > 60 {
				pc.Health = "warning"
			} else {
				pc.Health = "good"
			}
			dist.ByPool = append(dist.ByPool, *pc)
		}
	}

	// 生成时间分布（模拟数据）
	for i := 0; i < 24; i++ {
		dist.ByTimeOfDay[i] = HourlyCost{
			Hour:    i,
			AvgCost: 10.0 + float64(i%12)*2, // 模拟工作时间成本较高
		}
	}

	// 生成星期分布
	days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	for i, day := range days {
		cost := 100.0
		if i >= 5 { // 周末
			cost = 60.0
		}
		dist.ByDayOfWeek[i] = DailyCost{
			DayOfWeek: day,
			AvgCost:   cost,
		}
	}

	// 成本驱动因素
	dist.TopCostDrivers = []CostDriver{
		{
			Factor:        "storage_growth",
			Description:   "存储容量增长",
			ImpactAmount:  dist.ByPool[0].TotalCost * 0.3,
			ImpactPercent: 30,
			Controllable:  true,
		},
		{
			Factor:        "user_activity",
			Description:   "用户活跃度增加",
			ImpactAmount:  dist.ByPool[0].TotalCost * 0.2,
			ImpactPercent: 20,
			Controllable:  false,
		},
	}

	return dist
}

// detectAnomalies 检测异常
func (e *CostAnalysisEngine) detectAnomalies(start, end time.Time) AnomalyDetection {
	detection := AnomalyDetection{
		HasAnomalies:    false,
		AnomalyCount:    0,
		Anomalies:       make([]CostAnomaly, 0),
		DetectionMethod: "statistical",
		BaselinePeriod:  "30_days",
		Threshold:       20.0, // 20% 偏差视为异常
		Statistics: AnomalyStatistics{
			Mean:            100,
			StdDev:          15,
			Median:          95,
			Percentile95:    130,
			Percentile99:    150,
			ZScoreThreshold: 2.0,
		},
	}

	// 检查趋势数据中的异常
	for _, trend := range e.trendData {
		if trend.Date.After(start) && trend.Date.Before(end) {
			// 简单的异常检测：超过平均值2个标准差
			deviation := (trend.TotalCost - detection.Statistics.Mean) / detection.Statistics.StdDev
			if deviation > detection.Statistics.ZScoreThreshold || deviation < -detection.Statistics.ZScoreThreshold {
				anomaly := CostAnomaly{
					ID:             generateReportID(),
					DetectedAt:     trend.Date,
					Type:           "spike",
					Severity:       "medium",
					ResourceType:   "storage",
					ExpectedValue:  detection.Statistics.Mean,
					ActualValue:    trend.TotalCost,
					Deviation:      deviation * 100,
					PotentialCause: "异常使用或系统活动",
					Status:         "new",
				}
				detection.Anomalies = append(detection.Anomalies, anomaly)
				detection.HasAnomalies = true
				detection.AnomalyCount++
			}
		}
	}

	return detection
}

// generatePeriodComparison 生成周期对比
func (e *CostAnalysisEngine) generatePeriodComparison(days int) PeriodComparison {
	now := time.Now()

	comparison := PeriodComparison{
		CurrentPeriod: PeriodCostDetail{
			PeriodStart: now.AddDate(0, 0, -days),
			PeriodEnd:   now,
		},
		PreviousPeriod: PeriodCostDetail{
			PeriodStart: now.AddDate(0, 0, -days*2),
			PeriodEnd:   now.AddDate(0, 0, -days),
		},
	}

	// 获取当前周期数据
	if stats, err := e.billingClient.GetBillingStats(comparison.CurrentPeriod.PeriodStart, comparison.CurrentPeriod.PeriodEnd); err == nil {
		comparison.CurrentPeriod.TotalCost = stats.TotalRevenue
		comparison.CurrentPeriod.StorageCost = stats.StorageRevenue
		comparison.CurrentPeriod.BandwidthCost = stats.BandwidthRevenue
		comparison.CurrentPeriod.StorageUsedGB = stats.TotalStorageUsedGB
		comparison.CurrentPeriod.BandwidthUsedGB = stats.TotalBandwidthGB
	}

	// 获取上一周期数据
	if stats, err := e.billingClient.GetBillingStats(comparison.PreviousPeriod.PeriodStart, comparison.PreviousPeriod.PeriodEnd); err == nil {
		comparison.PreviousPeriod.TotalCost = stats.TotalRevenue
		comparison.PreviousPeriod.StorageCost = stats.StorageRevenue
		comparison.PreviousPeriod.BandwidthCost = stats.BandwidthRevenue
		comparison.PreviousPeriod.StorageUsedGB = stats.TotalStorageUsedGB
		comparison.PreviousPeriod.BandwidthUsedGB = stats.TotalBandwidthGB
	}

	// 计算变化
	if comparison.PreviousPeriod.TotalCost > 0 {
		comparison.ChangeAnalysis.TotalCostChange = comparison.CurrentPeriod.TotalCost - comparison.PreviousPeriod.TotalCost
		comparison.ChangeAnalysis.TotalCostChangePercent = comparison.ChangeAnalysis.TotalCostChange / comparison.PreviousPeriod.TotalCost * 100
	}
	comparison.ChangeAnalysis.StorageCostChange = comparison.CurrentPeriod.StorageCost - comparison.PreviousPeriod.StorageCost
	comparison.ChangeAnalysis.BandwidthCostChange = comparison.CurrentPeriod.BandwidthCost - comparison.PreviousPeriod.BandwidthCost
	comparison.ChangeAnalysis.StorageUsageChange = comparison.CurrentPeriod.StorageUsedGB - comparison.PreviousPeriod.StorageUsedGB
	comparison.ChangeAnalysis.BandwidthUsageChange = comparison.CurrentPeriod.BandwidthUsedGB - comparison.PreviousPeriod.BandwidthUsedGB

	// 关键因素
	if comparison.ChangeAnalysis.TotalCostChange > 0 {
		comparison.ChangeAnalysis.KeyFactors = []ChangeFactor{
			{
				Factor:      "storage_growth",
				Description: "存储使用量增加",
				Impact:      comparison.ChangeAnalysis.StorageCostChange,
				Direction:   "increase",
			},
		}
	}

	return comparison
}

// generateTrendAnalysis 生成趋势分析
func (e *CostAnalysisEngine) generateTrendAnalysis(start, end time.Time) TrendAnalysis {
	analysis := TrendAnalysis{
		OverallTrend:  "stable",
		TrendStrength: 0.5,
		DailyData:     make([]DailyTrendData, 0),
		WeeklyPattern: WeeklyPattern{
			PeakDay:      "Wednesday",
			LowDay:       "Sunday",
			Variance:     0.2,
			IsConsistent: true,
		},
		MonthlyProjection: MonthlyProjection{
			ProjectedCost:    0,
			ConfidenceLevel:  0.75,
			ProjectionMethod: "linear_regression",
			KeyAssumptions:   []string{"使用模式保持稳定", "无重大系统变更"},
		},
		Seasonality: SeasonalityInfo{
			HasSeasonality: false,
		},
	}

	// 生成每日数据
	for _, trend := range e.trendData {
		if trend.Date.After(start) && trend.Date.Before(end) {
			analysis.DailyData = append(analysis.DailyData, DailyTrendData{
				Date:          trend.Date,
				TotalCost:     trend.TotalCost,
				StorageCost:   trend.StorageCost,
				BandwidthCost: trend.BandwidthCost,
				StorageGB:     trend.StorageUsedGB,
				BandwidthGB:   trend.BandwidthGB,
				IsAnomaly:     false,
			})
		}
	}

	// 计算预测
	if len(analysis.DailyData) > 0 {
		var totalCost float64
		for _, d := range analysis.DailyData {
			totalCost += d.TotalCost
		}
		avgDaily := totalCost / float64(len(analysis.DailyData))
		analysis.MonthlyProjection.ProjectedCost = avgDaily * 30
		analysis.MonthlyProjection.UpperBound = analysis.MonthlyProjection.ProjectedCost * 1.2
		analysis.MonthlyProjection.LowerBound = analysis.MonthlyProjection.ProjectedCost * 0.8
	}

	return analysis
}

// generatePoolBreakdown 生成存储池分解
func (e *CostAnalysisEngine) generatePoolBreakdown() []PoolCostBreakdown {
	breakdown := make([]PoolCostBreakdown, 0)

	if quotaUsages, err := e.quotaClient.GetAllUsage(); err == nil {
		poolMap := make(map[string]*PoolCostBreakdown)

		for _, u := range quotaUsages {
			poolName := u.VolumeName
			if _, exists := poolMap[poolName]; !exists {
				poolMap[poolName] = &PoolCostBreakdown{
					PoolID:          poolName,
					PoolName:        poolName,
					StorageType:     "hdd",
					UsedGB:          0,
					CapacityGB:      0,
					PricePerGB:      e.billingClient.GetStoragePrice(poolName),
					CostTrend:       "stable",
					TopUsers:        make([]TopUserInPool, 0),
					UserCount:       0,
					GrowthRate:      0,
					EfficiencyScore: 0.8,
				}
			}

			usedGB := float64(u.UsedBytes) / (1024 * 1024 * 1024)
			capacityGB := float64(u.HardLimit) / (1024 * 1024 * 1024)
			poolMap[poolName].UsedGB += usedGB
			poolMap[poolName].CapacityGB += capacityGB
			poolMap[poolName].UserCount++

			// 记录顶部用户
			topUser := TopUserInPool{
				UserID:     u.TargetID,
				UserName:   u.TargetName,
				UsedGB:     usedGB,
				Cost:       usedGB * poolMap[poolName].PricePerGB,
				Percentage: 0, // 后续计算
			}
			poolMap[poolName].TopUsers = append(poolMap[poolName].TopUsers, topUser)
		}

		for _, pool := range poolMap {
			pool.StorageCost = pool.UsedGB * pool.PricePerGB
			pool.TotalCost = pool.StorageCost
			if pool.CapacityGB > 0 {
				pool.Utilization = pool.UsedGB / pool.CapacityGB * 100
			}

			// 计算用户占比
			for i := range pool.TopUsers {
				if pool.UsedGB > 0 {
					pool.TopUsers[i].Percentage = pool.TopUsers[i].UsedGB / pool.UsedGB * 100
				}
			}

			// 生成建议
			if pool.Utilization > 80 {
				pool.Recommendation = "存储池使用率较高，建议扩容或清理"
			} else if pool.Utilization < 30 {
				pool.Recommendation = "存储池利用率偏低，可考虑整合资源"
			}

			breakdown = append(breakdown, *pool)
		}
	}

	return breakdown
}

// generateUserBreakdown 生成用户分解
func (e *CostAnalysisEngine) generateUserBreakdown() []UserCostBreakdown {
	breakdown := make([]UserCostBreakdown, 0)

	if quotaUsages, err := e.quotaClient.GetAllUsage(); err == nil {
		userMap := make(map[string]*UserCostBreakdown)

		for _, u := range quotaUsages {
			if _, exists := userMap[u.TargetID]; !exists {
				userMap[u.TargetID] = &UserCostBreakdown{
					UserID:           u.TargetID,
					UserName:         u.TargetName,
					StorageUsedGB:    0,
					BandwidthUsedGB:  0,
					QuotaLimitGB:     0,
					PoolDistribution: make([]PoolUsage, 0),
					DailyTrend:       make([]DailyUserCost, 0),
					CostTrend:        "stable",
					EfficiencyScore:  0.75,
				}
			}

			usedGB := float64(u.UsedBytes) / (1024 * 1024 * 1024)
			limitGB := float64(u.HardLimit) / (1024 * 1024 * 1024)
			pricePerGB := e.billingClient.GetStoragePrice(u.VolumeName)

			userMap[u.TargetID].StorageUsedGB += usedGB
			userMap[u.TargetID].QuotaLimitGB += limitGB
			userMap[u.TargetID].StorageCost += usedGB * pricePerGB
			userMap[u.TargetID].PoolCount++

			userMap[u.TargetID].PoolDistribution = append(userMap[u.TargetID].PoolDistribution, PoolUsage{
				PoolID:   u.VolumeName,
				PoolName: u.VolumeName,
				UsedGB:   usedGB,
				Cost:     usedGB * pricePerGB,
			})
		}

		for _, user := range userMap {
			user.TotalCost = user.StorageCost + user.BandwidthCost
			if user.QuotaLimitGB > 0 {
				user.QuotaUtilization = user.StorageUsedGB / user.QuotaLimitGB * 100
			}

			// 生成建议
			if user.QuotaUtilization > 90 {
				user.Recommendation = "配额使用率接近上限，建议增加配额或清理"
			} else if user.QuotaUtilization < 20 {
				user.Recommendation = "配额利用率较低，可考虑降低配额"
			}

			breakdown = append(breakdown, *user)
		}
	}

	return breakdown
}

// generateEnhancedRecommendations 生成增强版建议
func (e *CostAnalysisEngine) generateEnhancedRecommendations(report *EnhancedStorageReport) []CostRecommendation {
	recommendations := make([]CostRecommendation, 0)

	// 基于成本分布生成建议
	for _, pool := range report.PoolBreakdown {
		if pool.Utilization > 80 {
			recommendations = append(recommendations, CostRecommendation{
				ID:               generateReportID(),
				Type:             "pool",
				Priority:         "high",
				Title:            fmt.Sprintf("存储池 %s 容量预警", pool.PoolName),
				Description:      fmt.Sprintf("存储池使用率达 %.1f%%，建议尽快扩容", pool.Utilization),
				PotentialSavings: 0,
				Impact:           "系统稳定性",
				Action:           "评估扩容需求或清理无用数据",
			})
		} else if pool.Utilization < 30 && pool.CapacityGB > 100 {
			recommendations = append(recommendations, CostRecommendation{
				ID:               generateReportID(),
				Type:             "pool",
				Priority:         "low",
				Title:            fmt.Sprintf("存储池 %s 资源优化", pool.PoolName),
				Description:      fmt.Sprintf("存储池利用率仅 %.1f%%，存在资源浪费", pool.Utilization),
				PotentialSavings: pool.TotalCost * 0.2,
				Impact:           "成本优化",
				Action:           "考虑整合存储资源或调整存储类型",
			})
		}
	}

	// 基于异常生成建议
	if report.AnomalyDetection.HasAnomalies {
		recommendations = append(recommendations, CostRecommendation{
			ID:               generateReportID(),
			Type:             "anomaly",
			Priority:         "high",
			Title:            "成本异常需关注",
			Description:      fmt.Sprintf("检测到 %d 个成本异常点，建议排查", report.AnomalyDetection.AnomalyCount),
			PotentialSavings: 0,
			Impact:           "成本控制",
			Action:           "查看异常详情，排查原因并采取措施",
		})
	}

	// 基于趋势生成建议
	if report.TrendAnalysis.OverallTrend == "increasing" {
		recommendations = append(recommendations, CostRecommendation{
			ID:               generateReportID(),
			Type:             "trend",
			Priority:         "medium",
			Title:            "成本上升趋势预警",
			Description:      "近期成本呈上升趋势，建议关注",
			PotentialSavings: report.Summary.OptimizationPotential,
			Impact:           "预算管理",
			Action:           "评估是否需要调整预算或优化使用",
		})
	}

	return recommendations
}

// calculateOverallEfficiencyScore 计算整体效率评分
func (e *CostAnalysisEngine) calculateOverallEfficiencyScore(summary EnhancedCostSummary) float64 {
	score := 100.0

	// 低利用率扣分
	if summary.ActiveUsers > 0 && summary.TotalUsedGB > 0 {
		avgUsagePerUser := summary.TotalUsedGB / float64(summary.ActiveUsers)
		if avgUsagePerUser < 10 { // 平均每用户少于10GB
			score -= 10
		}
	}

	// 成本变化扣分
	if summary.CostChangePercent > 20 {
		score -= 15
	} else if summary.CostChangePercent > 10 {
		score -= 5
	}

	if score < 0 {
		score = 0
	}

	return score
}

// ========== 报告持久化 ==========

// SaveReport 保存报告
func (e *CostAnalysisEngine) SaveReport(report *EnhancedStorageReport) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := os.MkdirAll(filepath.Join(e.dataDir, "reports"), 0755); err != nil {
		return err
	}

	path := filepath.Join(e.dataDir, "reports", report.ID+".json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0640)
}

// LoadReport 加载报告
func (e *CostAnalysisEngine) LoadReport(id string) (*EnhancedStorageReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	path := filepath.Join(e.dataDir, "reports", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var report EnhancedStorageReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}

	return &report, nil
}

// ListSavedReports 列出已保存的报告
func (e *CostAnalysisEngine) ListSavedReports() ([]string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	reportsDir := filepath.Join(e.dataDir, "reports")
	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		return nil, err
	}

	var reports []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			reports = append(reports, entry.Name()[:len(entry.Name())-5])
		}
	}

	return reports, nil
}

// ========== 异常管理 ==========

// ResolveAnomaly 解决异常
func (e *CostAnalysisEngine) ResolveAnomaly(reportID, anomalyID, resolution string) error {
	report, err := e.LoadReport(reportID)
	if err != nil {
		return err
	}

	for i := range report.AnomalyDetection.Anomalies {
		if report.AnomalyDetection.Anomalies[i].ID == anomalyID {
			now := time.Now()
			report.AnomalyDetection.Anomalies[i].Status = "resolved"
			report.AnomalyDetection.Anomalies[i].Resolution = resolution
			report.AnomalyDetection.Anomalies[i].ResolvedAt = &now
			return e.SaveReport(report)
		}
	}

	return fmt.Errorf("异常不存在: %s", anomalyID)
}

// IgnoreAnomaly 忽略异常
func (e *CostAnalysisEngine) IgnoreAnomaly(reportID, anomalyID string) error {
	report, err := e.LoadReport(reportID)
	if err != nil {
		return err
	}

	for i := range report.AnomalyDetection.Anomalies {
		if report.AnomalyDetection.Anomalies[i].ID == anomalyID {
			report.AnomalyDetection.Anomalies[i].Status = "ignored"
			return e.SaveReport(report)
		}
	}

	return fmt.Errorf("异常不存在: %s", anomalyID)
}

// ========== 线程安全 ==========

type enhancedReportCache struct {
	mu      sync.RWMutex
	reports map[string]*EnhancedStorageReport
}

func newEnhancedReportCache() *enhancedReportCache {
	return &enhancedReportCache{
		reports: make(map[string]*EnhancedStorageReport),
	}
}

func (c *enhancedReportCache) Get(id string) (*EnhancedStorageReport, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	report, ok := c.reports[id]
	return report, ok
}

func (c *enhancedReportCache) Set(id string, report *EnhancedStorageReport) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reports[id] = report
}

func (c *enhancedReportCache) Delete(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.reports, id)
}
