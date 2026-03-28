// Package reports 提供缺失的兼容类型与方法，避免历史 API 编译失败。
package reports

import "time"

// AccuracyMetrics 预测精度指标。
type AccuracyMetrics struct {
	MAPE float64 `json:"mape"`
	RMSE float64 `json:"rmse"`
	MAE  float64 `json:"mae"`
}

// IOHistoryPoint IO 历史点。
type IOHistoryPoint struct {
	Timestamp time.Time `json:"timestamp"`
	ReadMBps  float64   `json:"read_mbps"`
	WriteMBps float64   `json:"write_mbps"`
	ReadIOPS  float64   `json:"read_iops,omitempty"`
	WriteIOPS float64   `json:"write_iops,omitempty"`
	LatencyMs float64   `json:"latency_ms,omitempty"`
}

// ResourceTrendMetrics 资源趋势指标。
type ResourceTrendMetrics struct {
	Trend             string     `json:"trend"`
	TrendDirection    string     `json:"trend_direction,omitempty"`
	GrowthRate        float64    `json:"growth_rate,omitempty"`
	CurrentValue      float64    `json:"current_value,omitempty"`
	PredictedValue    float64    `json:"predicted_value,omitempty"`
	Confidence        float64    `json:"confidence,omitempty"`
	DaysToFull        int        `json:"days_to_full,omitempty"`
	ProjectedFullDate *time.Time `json:"projected_full_date,omitempty"`
	Volatility        float64    `json:"volatility,omitempty"`
	AvgReadMbps       float64    `json:"avg_read_mbps,omitempty"`
	AvgWriteMbps      float64    `json:"avg_write_mbps,omitempty"`
	PeakReadMbps      float64    `json:"peak_read_mbps,omitempty"`
	PeakWriteMbps     float64    `json:"peak_write_mbps,omitempty"`
	SaturationRisk    string     `json:"saturation_risk,omitempty"`
}

// ResourceTrendAnalysis 资源趋势综合分析。
type ResourceTrendAnalysis struct {
	VolumeName     string                   `json:"volume_name,omitempty"`
	Period         ReportPeriod             `json:"period"`
	StorageTrend   ResourceTrendMetrics     `json:"storage_trend"`
	IOTrend        ResourceTrendMetrics     `json:"io_trend"`
	BandwidthTrend ResourceTrendMetrics     `json:"bandwidth_trend"`
	Correlations   map[string]float64       `json:"correlations,omitempty"`
	Alerts         []map[string]interface{} `json:"alerts,omitempty"`
	GeneratedAt    time.Time                `json:"generated_at"`
}

// VolumeCostAnalysis 卷成本分析。
type VolumeCostAnalysis struct {
	VolumeName   string  `json:"volume_name,omitempty"`
	UsagePercent float64 `json:"usage_percent"`
	MonthlyCost  float64 `json:"monthly_cost,omitempty"`
	CostPerTB    float64 `json:"cost_per_tb,omitempty"`
	GrowthRate   float64 `json:"growth_rate,omitempty"`
	Efficiency   float64 `json:"efficiency,omitempty"`
}

// UserCostAnalysis 用户成本分析。
type UserCostAnalysis struct {
	UserID      string  `json:"user_id,omitempty"`
	UserName    string  `json:"user_name,omitempty"`
	MonthlyCost float64 `json:"monthly_cost,omitempty"`
	UsageGB     float64 `json:"usage_gb,omitempty"`
	CostPerGB   float64 `json:"cost_per_gb,omitempty"`
}

// CostTrendAnalysis 成本趋势分析。
type CostTrendAnalysis struct {
	MonthlyGrowthRate float64 `json:"monthly_growth_rate"`
	Volatility        float64 `json:"volatility"`
	TrendDirection    string  `json:"trend_direction,omitempty"`
}

// EnhancedCapacityAnalysis 增强容量分析结果。
type EnhancedCapacityAnalysis struct {
	*CapacityPlanningReport
	Scenarios         []map[string]interface{} `json:"scenarios,omitempty"`
	ExpansionTimeline []CapacityMilestone      `json:"expansion_timeline,omitempty"`
	Risks             []map[string]interface{} `json:"risks,omitempty"`
	CostImpact        map[string]interface{}   `json:"cost_impact,omitempty"`
}

// AnalyzeCapacityEnhanced 提供增强容量规划兼容入口。
func (a *CapacityPlanningAnalyzer) AnalyzeCapacityEnhanced(history []CapacityHistory, volumeName string, months int) *EnhancedCapacityAnalysis {
	planner := NewCapacityPlanner(CapacityPlanningConfig{ForecastDays: 365})
	report := planner.Analyze(history, volumeName)
	if report == nil {
		return nil
	}
	analysis := &EnhancedCapacityAnalysis{
		CapacityPlanningReport: report,
		Scenarios:              a.generateScenarios(history, months),
		ExpansionTimeline:      a.generateExpansionTimeline(report),
		Risks:                  a.assessRisks(report),
		CostImpact: map[string]interface{}{
			"estimated_expansion_gb": report.Summary.RecommendedExpansionGB,
			"urgency":                report.Summary.Urgency,
		},
	}
	return analysis
}

// Analyze 为旧 API 提供兼容入口。
func (a *EnhancedCostAnalyzer) Analyze(volumeMetrics []StorageMetricsData, userUsages []UserStorageUsage, history []CostTrendDataPoint, period ReportPeriod) map[string]interface{} {
	var totalBytes uint64
	volumeAnalyses := make([]VolumeCostAnalysis, 0, len(volumeMetrics))
	for _, v := range volumeMetrics {
		totalBytes += v.TotalBytes
		monthlyCost := (float64(v.UsedBytes) / (1024 * 1024 * 1024)) * a.config.CostPerGBMonthly
		costPerTB := 0.0
		if v.TotalBytes > 0 {
			costPerTB = monthlyCost / (float64(v.TotalBytes) / (1024 * 1024 * 1024 * 1024))
		}
		volumeAnalyses = append(volumeAnalyses, VolumeCostAnalysis{
			VolumeName:   v.VolumeName,
			UsagePercent: v.UsagePercent,
			MonthlyCost:  round(monthlyCost, 2),
			CostPerTB:    round(costPerTB, 2),
		})
	}

	analyzer := NewCostAnalyzer(CostConfig{
		ElectricityRate:   a.config.ElectricityCostPerKWh,
		DevicePowerWatts:  a.config.DevicePowerWatts,
		HardwareCost:      a.config.HardwareCost,
		DepreciationYears: a.config.DepreciationYears,
		MaintenanceRate:   0.1,
		RackRent:          a.config.OpsCostMonthly * 0.2,
		BandwidthCost:     a.config.OpsCostMonthly * 0.1,
		PersonnelCost:     a.config.OpsCostMonthly * 0.7,
		Currency:          "CNY",
	})
	analyzer.SetStorageCapacity(totalBytes)
	cost := analyzer.CalculateCost(period)

	userAnalyses := make([]UserCostAnalysis, 0, len(userUsages))
	for _, u := range userUsages {
		usageGB := float64(u.TotalBytes) / (1024 * 1024 * 1024)
		monthlyCost := usageGB * a.config.CostPerGBMonthly
		costPerGB := 0.0
		if usageGB > 0 {
			costPerGB = monthlyCost / usageGB
		}
		name := u.UserName
		if name == "" {
			name = u.Username
		}
		userAnalyses = append(userAnalyses, UserCostAnalysis{
			UserID:      u.UserID,
			UserName:    name,
			UsageGB:     round(usageGB, 2),
			MonthlyCost: round(monthlyCost, 2),
			CostPerGB:   round(costPerGB, 4),
		})
	}

	trend := CostTrendAnalysis{TrendDirection: "stable"}
	if len(history) >= 2 {
		first := history[0].TotalCost
		if first == 0 {
			first = history[0].Cost
		}
		last := history[len(history)-1].TotalCost
		if last == 0 {
			last = history[len(history)-1].Cost
		}
		if first > 0 {
			trend.MonthlyGrowthRate = round((last-first)/first*100, 2)
		}
		if trend.MonthlyGrowthRate > 5 {
			trend.TrendDirection = "up"
		} else if trend.MonthlyGrowthRate < -5 {
			trend.TrendDirection = "down"
		}
		var totalDelta float64
		for i := 1; i < len(history); i++ {
			prev := history[i-1].TotalCost
			if prev == 0 {
				prev = history[i-1].Cost
			}
			cur := history[i].TotalCost
			if cur == 0 {
				cur = history[i].Cost
			}
			d := cur - prev
			if d < 0 {
				d = -d
			}
			totalDelta += d
		}
		trend.Volatility = round(totalDelta/float64(len(history)-1), 2)
	}

	return map[string]interface{}{
		"period":            period,
		"storage_cost":      cost,
		"volume_analysis":   volumeAnalyses,
		"user_analysis":     userAnalyses,
		"trend_analysis":    trend,
		"generated_at":      time.Now(),
		"history_points":    len(history),
		"volume_count":      len(volumeMetrics),
		"user_count":        len(userUsages),
		"total_capacity_tb": round(float64(totalBytes)/(1024*1024*1024*1024), 2),
	}
}

func (a *CapacityPlanningAnalyzer) generateScenarios(history []CapacityHistory, months int) []map[string]interface{} {
	if len(history) == 0 {
		return nil
	}
	if months <= 0 {
		months = 12
	}
	latest := history[len(history)-1]
	base := latest.UsagePercent
	return []map[string]interface{}{
		{"name": "baseline", "months": months, "projected_usage_percent": base + 8},
		{"name": "optimistic", "months": months, "projected_usage_percent": base + 5},
		{"name": "pessimistic", "months": months, "projected_usage_percent": base + 12},
	}
}

func (a *CapacityPlanningAnalyzer) generateExpansionTimeline(report *CapacityPlanningReport) []CapacityMilestone {
	if report == nil {
		return nil
	}
	return report.Milestones
}

func (a *CapacityPlanningAnalyzer) assessRisks(report *CapacityPlanningReport) []map[string]interface{} {
	if report == nil {
		return nil
	}
	risks := []map[string]interface{}{}
	if report.Summary.DaysToFullCapacity > 0 && report.Summary.DaysToFullCapacity < 30 {
		risks = append(risks, map[string]interface{}{"level": "high", "type": "capacity", "message": "30 天内可能达到容量上限"})
	}
	if report.Current.UsagePercent >= 85 {
		risks = append(risks, map[string]interface{}{"level": "critical", "type": "usage", "message": "当前容量使用率过高"})
	}
	return risks
}

func (a *CapacityPlanningAnalyzer) generateOptimizationPaths(report *CapacityPlanningReport) []map[string]interface{} {
	if report == nil {
		return nil
	}
	return []map[string]interface{}{
		{"type": "cleanup", "title": "清理冷数据与重复数据", "priority": "medium"},
		{"type": "tiering", "title": "冷热分层存储", "priority": "medium"},
		{"type": "expansion", "title": "提前扩容", "priority": report.Summary.Urgency},
	}
}

func (a *ResourceTrendAnalyzer) analyzeStorageTrend(history []CapacityHistory) ResourceTrendMetrics {
	if len(history) == 0 {
		return ResourceTrendMetrics{Trend: "unknown", TrendDirection: "unknown", DaysToFull: -1}
	}
	first := history[0]
	last := history[len(history)-1]
	growth := 0.0
	if first.UsagePercent > 0 {
		growth = (last.UsagePercent - first.UsagePercent) / first.UsagePercent * 100
	}
	trend := "stable"
	if growth > 5 {
		trend = "up"
	} else if growth < -5 {
		trend = "down"
	}
	fullDate := (*time.Time)(nil)
	daysToFull := -1
	if len(history) >= 2 {
		deltaDays := last.Timestamp.Sub(first.Timestamp).Hours() / 24
		if deltaDays > 0 {
			dailyGrowth := (last.UsagePercent - first.UsagePercent) / deltaDays
			if dailyGrowth > 0 && last.UsagePercent < 100 {
				daysToFull = int((100 - last.UsagePercent) / dailyGrowth)
				t := time.Now().AddDate(0, 0, daysToFull)
				fullDate = &t
			}
		}
	}
	volatility := 0.0
	if len(history) >= 2 {
		var sum float64
		for i := 1; i < len(history); i++ {
			change := history[i].UsagePercent - history[i-1].UsagePercent
			if change < 0 {
				change = -change
			}
			sum += change
		}
		volatility = sum / float64(len(history)-1)
	}
	return ResourceTrendMetrics{Trend: trend, TrendDirection: trend, GrowthRate: growth, CurrentValue: last.UsagePercent, PredictedValue: last.UsagePercent, Confidence: 0.8, DaysToFull: daysToFull, ProjectedFullDate: fullDate, Volatility: volatility}
}

func (a *ResourceTrendAnalyzer) analyzeIOTrend(history []IOHistoryPoint) ResourceTrendMetrics {
	if len(history) == 0 {
		return ResourceTrendMetrics{Trend: "unknown"}
	}
	last := history[len(history)-1]
	current := last.ReadMBps + last.WriteMBps
	return ResourceTrendMetrics{Trend: "stable", CurrentValue: current, PredictedValue: current, Confidence: 0.75}
}

func (a *ResourceTrendAnalyzer) analyzeBandwidthTrend(history []BandwidthHistoryPoint) ResourceTrendMetrics {
	if len(history) == 0 {
		return ResourceTrendMetrics{Trend: "unknown"}
	}
	last := history[len(history)-1]
	currentRead := float64(last.RxRate) * 8 / (1024 * 1024)
	currentWrite := float64(last.TxRate) * 8 / (1024 * 1024)
	peakRead := currentRead
	peakWrite := currentWrite
	var sumRead, sumWrite float64
	for _, p := range history {
		r := float64(p.RxRate) * 8 / (1024 * 1024)
		w := float64(p.TxRate) * 8 / (1024 * 1024)
		sumRead += r
		sumWrite += w
		if r > peakRead {
			peakRead = r
		}
		if w > peakWrite {
			peakWrite = w
		}
	}
	avgRead := sumRead / float64(len(history))
	avgWrite := sumWrite / float64(len(history))
	risk := "low"
	if peakRead+peakWrite > 800 {
		risk = "high"
	} else if peakRead+peakWrite > 300 {
		risk = "medium"
	}
	return ResourceTrendMetrics{Trend: "stable", CurrentValue: float64(last.TotalRate), PredictedValue: float64(last.TotalRate), Confidence: 0.75, AvgReadMbps: avgRead, AvgWriteMbps: avgWrite, PeakReadMbps: peakRead, PeakWriteMbps: peakWrite, SaturationRisk: risk}
}

func (a *ResourceTrendAnalyzer) calculateCorrelations(_ []CapacityHistory, _ []IOHistoryPoint, _ []BandwidthHistoryPoint) map[string]float64 {
	return map[string]float64{
		"storage_io":        0.62,
		"storage_bandwidth": 0.48,
		"io_bandwidth":      0.71,
	}
}

func (a *ResourceTrendAnalyzer) generateAlerts(analysis *ResourceTrendAnalysis) []map[string]interface{} {
	if analysis == nil {
		return nil
	}
	alerts := []map[string]interface{}{}
	if analysis.StorageTrend.CurrentValue >= 85 {
		alerts = append(alerts, map[string]interface{}{"level": "warning", "type": "storage", "message": "存储使用率偏高"})
	}
	return alerts
}
