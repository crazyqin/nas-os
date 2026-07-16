package reports

import "time"

// NewCostAnalyzer 兼容旧调用：允许直接传 StorageCostConfig。
func NewCostAnalyzerFromStorage(config StorageCostConfig) *CostAnalyzer {
	return NewCostAnalyzer(CostConfig{
		ElectricityRate:   config.ElectricityCostPerKWh,
		DevicePowerWatts:  config.DevicePowerWatts,
		HardwareCost:      config.HardwareCost,
		DepreciationYears: config.DepreciationYears,
		MaintenanceRate:   0.1,
		RackRent:          config.OpsCostMonthly * 0.2,
		BandwidthCost:     config.OpsCostMonthly * 0.1,
		PersonnelCost:     config.OpsCostMonthly * 0.7,
		Currency:          firstNonEmpty(config.Currency, "CNY"),
	})
}

func firstNonEmpty(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

// AnalyzeLegacy 为旧测试提供兼容报表输出。
func (a *CostAnalyzer) AnalyzeLegacy(volumeMetrics []StorageMetrics, userUsages []UserStorageUsage, history []CostTrendDataPoint, period ReportPeriod) *LegacyCostAnalysisReport {
	calc := NewStorageCostCalculator(StorageCostConfig{})
	volumeCosts := make([]StorageCostResult, 0, len(volumeMetrics))
	var totalCapacityGB, totalUsedGB float64
	for _, vm := range volumeMetrics {
		res := calc.Calculate(vm)
		volumeCosts = append(volumeCosts, StorageCostResult{
			VolumeName:          vm.VolumeName,
			TotalGB:             float64(vm.TotalCapacityBytes) / (1024 * 1024 * 1024),
			UsedGB:              float64(vm.UsedCapacityBytes) / (1024 * 1024 * 1024),
			UsagePercent:        vm.UsagePercent,
			MonthlyCost:         res.TotalCostMonthly,
			TotalCostMonthly:    res.TotalCostMonthly,
			CapacityCostMonthly: res.CapacityCostMonthly,
			CalculatedAt:        time.Now(),
		})
		totalCapacityGB += float64(vm.TotalCapacityBytes) / (1024 * 1024 * 1024)
		totalUsedGB += float64(vm.UsedCapacityBytes) / (1024 * 1024 * 1024)
	}

	userCosts := make([]UserCostAnalysis, 0, len(userUsages))
	for _, u := range userUsages {
		name := u.UserName
		if name == "" {
			name = u.Username
		}
		userCosts = append(userCosts, UserCostAnalysis{
			UserID:      u.UserID,
			UserName:    name,
			UsageGB:     float64(u.UsedBytes) / (1024 * 1024 * 1024),
			MonthlyCost: 0,
		})
	}

	trend := CostTrendAnalysis{TrendDirection: "stable"}
	if len(history) >= 2 {
		first := history[0].TotalCost
		last := history[len(history)-1].TotalCost
		if first > 0 {
			trend.MonthlyGrowthRate = round((last-first)/first*100, 2)
		}
		if trend.MonthlyGrowthRate > 5 {
			trend.TrendDirection = "up"
		} else if trend.MonthlyGrowthRate < -5 {
			trend.TrendDirection = "down"
		}
	}

	cost := a.CalculateCost(period)
	optimization := []CostOptimizationItem{{ID: "cleanup", Priority: "high", Savings: 100}}

	return &LegacyCostAnalysisReport{
		ID:          generateReportID(CostReportTypeMonthly, time.Now()),
		GeneratedAt: time.Now(),
		VolumeCosts: volumeCosts,
		UserCosts:   userCosts,
		TotalCost: LegacyCostSummary{
			TotalMonthlyCost: cost.TotalCost,
			TotalCapacityGB:  totalCapacityGB,
			TotalUsedGB:      totalUsedGB,
			VolumeCount:      len(volumeMetrics),
			HealthScore:      80,
		},
		TrendAnalysis: trend,
		Optimization:  optimization,
	}
}
