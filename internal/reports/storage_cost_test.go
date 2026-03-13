// Package reports 提供报表生成和管理功能
package reports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== 存储成本计算测试 ==========

func TestStorageCostCalculator_Calculate(t *testing.T) {
	config := StorageCostConfig{
		CostPerGBMonthly:        0.5,   // 0.5元/GB/月
		CostPerIOPSMonthly:      0.01,  // 0.01元/1000 IOPS/月
		CostPerBandwidthMonthly: 1.0,   // 1元/Mbps/月
		ElectricityCostPerKWh:   0.6,   // 0.6元/kWh
		DevicePowerWatts:        100,   // 100W
		OpsCostMonthly:          500,   // 500元/月
		DepreciationYears:       5,     // 5年
		HardwareCost:            50000, // 50000元
	}

	calculator := NewStorageCostCalculator(config)

	metrics := StorageMetrics{
		VolumeName:             "test-volume",
		TotalCapacityBytes:     1 * 1024 * 1024 * 1024 * 1024, // 1TB
		UsedCapacityBytes:      500 * 1024 * 1024 * 1024,      // 500GB
		AvailableCapacityBytes: 524 * 1024 * 1024 * 1024,      // 524GB
		IOPS:                   1000,
		ReadBandwidthBytes:     10 * 1024 * 1024, // 10MB/s
		WriteBandwidthBytes:    5 * 1024 * 1024,  // 5MB/s
		FileCount:              100000,
		Timestamp:              time.Now(),
	}

	result := calculator.Calculate(metrics)

	// 验证基本字段
	assert.Equal(t, "test-volume", result.VolumeName)
	assert.InDelta(t, 50.0, result.UsagePercent, 2.0) // 500GB / 1TB ≈ 48.8%

	// 验证容量成本（1TB * 0.5元/GB = 512元）
	assert.Equal(t, 512.0, result.CapacityCostMonthly)

	// 验证IOPS成本（1000/1000 * 0.01 = 0.01元）
	assert.Equal(t, 0.01, result.IOPSCostMonthly)

	// 验证总成本大于0
	assert.Greater(t, result.TotalCostMonthly, 0.0)

	// 验证单位成本
	assert.Greater(t, result.CostPerGBMonthly, 0.0)

	// 验证计算时间
	assert.NotZero(t, result.CalculatedAt)
}

func TestStorageCostCalculator_CalculateAll(t *testing.T) {
	config := StorageCostConfig{
		CostPerGBMonthly:  0.5,
		OpsCostMonthly:    100,
		DepreciationYears: 5,
		HardwareCost:      10000,
	}

	calculator := NewStorageCostCalculator(config)

	metrics := []StorageMetrics{
		{
			VolumeName:         "vol1",
			TotalCapacityBytes: 500 * 1024 * 1024 * 1024, // 500GB
			UsedCapacityBytes:  250 * 1024 * 1024 * 1024, // 250GB
			IOPS:               500,
			Timestamp:          time.Now(),
		},
		{
			VolumeName:         "vol2",
			TotalCapacityBytes: 1 * 1024 * 1024 * 1024 * 1024, // 1TB
			UsedCapacityBytes:  800 * 1024 * 1024 * 1024,      // 800GB
			IOPS:               1000,
			Timestamp:          time.Now(),
		},
	}

	results := calculator.CalculateAll(metrics)

	assert.Len(t, results, 2)
	assert.Equal(t, "vol1", results[0].VolumeName)
	assert.Equal(t, "vol2", results[1].VolumeName)
	assert.InDelta(t, 50.0, results[0].UsagePercent, 1.0)
	assert.InDelta(t, 78.0, results[1].UsagePercent, 1.0) // 约 800/1024
}

func TestStorageCostCalculator_GenerateReport(t *testing.T) {
	config := StorageCostConfig{
		CostPerGBMonthly:  0.5,
		OpsCostMonthly:    100,
		DepreciationYears: 5,
		HardwareCost:      10000,
	}

	calculator := NewStorageCostCalculator(config)

	metrics := []StorageMetrics{
		{
			VolumeName:         "vol1",
			TotalCapacityBytes: 500 * 1024 * 1024 * 1024,
			UsedCapacityBytes:  250 * 1024 * 1024 * 1024,
			IOPS:               500,
			Timestamp:          time.Now(),
		},
	}

	period := ReportPeriod{
		StartTime: time.Now().AddDate(0, -1, 0),
		EndTime:   time.Now(),
	}

	report := calculator.GenerateReport(metrics, period)

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, "存储成本报表", report.Name)
	assert.Len(t, report.VolumeCosts, 1)
	assert.Equal(t, 1, report.Summary.VolumeCount)
	assert.Greater(t, report.Summary.TotalCostMonthly, 0.0)
}

func TestStorageCostCalculator_AnalyzeTrend(t *testing.T) {
	config := StorageCostConfig{
		CostPerGBMonthly: 0.5,
	}

	calculator := NewStorageCostCalculator(config)

	now := time.Now()
	history := []CostTrendPoint{
		{Timestamp: now.AddDate(0, -3, 0), TotalCostMonthly: 1000, UsagePercent: 40},
		{Timestamp: now.AddDate(0, -2, 0), TotalCostMonthly: 1100, UsagePercent: 45},
		{Timestamp: now.AddDate(0, -1, 0), TotalCostMonthly: 1200, UsagePercent: 50},
		{Timestamp: now, TotalCostMonthly: 1300, UsagePercent: 55},
	}

	report := calculator.AnalyzeTrend(history)

	assert.NotNil(t, report)
	assert.Equal(t, "成本趋势分析", report.Name)
	assert.Equal(t, 1150.0, report.Summary.AvgMonthlyCost)
	assert.Equal(t, 1300.0, report.Summary.MaxMonthlyCost)
	assert.Equal(t, 1000.0, report.Summary.MinMonthlyCost)
	assert.Greater(t, report.Summary.CostGrowthRate, 0.0)
}

// ========== 容量规划测试 ==========

func TestCapacityPlanner_Analyze(t *testing.T) {
	config := CapacityPlanningConfig{
		AlertThreshold:    70.0,
		CriticalThreshold: 85.0,
		ForecastDays:      30,
		GrowthModel:       GrowthModelLinear,
		ExpansionLeadTime: 14,
		SafetyBuffer:      20.0,
	}

	planner := NewCapacityPlanner(config)

	now := time.Now()
	history := []CapacityHistory{
		{
			Timestamp:    now.AddDate(0, 0, -30),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024, // 1TB
			UsedBytes:    500 * 1024 * 1024 * 1024,      // 500GB
			UsagePercent: 50.0,
		},
		{
			Timestamp:    now.AddDate(0, 0, -15),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    600 * 1024 * 1024 * 1024,
			UsagePercent: 60.0,
		},
		{
			Timestamp:    now,
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    700 * 1024 * 1024 * 1024, // 700GB
			UsagePercent: 70.0,
		},
	}

	report := planner.Analyze(history, "test-volume")

	assert.NotNil(t, report)
	assert.Equal(t, "test-volume", report.VolumeName)
	assert.Equal(t, "warning", report.Current.Status) // 70% = warning
	assert.Equal(t, 70.0, report.Current.UsagePercent)
	assert.NotEmpty(t, report.Forecasts)
	assert.NotEmpty(t, report.Milestones)
	assert.NotEmpty(t, report.Recommendations)
	assert.NotEmpty(t, report.Summary.Trend)
}

func TestCapacityPlanner_PredictCapacityNeeds(t *testing.T) {
	config := CapacityPlanningConfig{
		GrowthModel:  GrowthModelLinear,
		SafetyBuffer: 20.0,
	}

	planner := NewCapacityPlanner(config)

	now := time.Now()
	history := []CapacityHistory{
		{
			Timestamp:    now.AddDate(0, 0, -60),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    400 * 1024 * 1024 * 1024,
			UsagePercent: 40.0,
		},
		{
			Timestamp:    now.AddDate(0, 0, -30),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    500 * 1024 * 1024 * 1024,
			UsagePercent: 50.0,
		},
		{
			Timestamp:    now,
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    600 * 1024 * 1024 * 1024,
			UsagePercent: 60.0,
		},
	}

	// 预测3个月后
	predicted, err := planner.PredictCapacityNeeds(history, 3)

	assert.NoError(t, err)
	assert.Greater(t, predicted, uint64(600*1024*1024*1024)) // 应该大于当前使用量
}

func TestCapacityPlanner_GrowthModels(t *testing.T) {
	now := time.Now()
	history := []CapacityHistory{
		{Timestamp: now.AddDate(0, 0, -30), TotalBytes: 1e12, UsedBytes: 5e11, UsagePercent: 50},
		{Timestamp: now.AddDate(0, 0, -20), TotalBytes: 1e12, UsedBytes: 6e11, UsagePercent: 60},
		{Timestamp: now.AddDate(0, 0, -10), TotalBytes: 1e12, UsedBytes: 7e11, UsagePercent: 70},
		{Timestamp: now, TotalBytes: 1e12, UsedBytes: 8e11, UsagePercent: 80},
	}

	// 测试线性增长模型
	linearConfig := CapacityPlanningConfig{
		GrowthModel:    GrowthModelLinear,
		ForecastDays:   30,
		AlertThreshold: 70,
	}
	linearPlanner := NewCapacityPlanner(linearConfig)
	linearReport := linearPlanner.Analyze(history, "test")
	assert.NotNil(t, linearReport)

	// 测试指数增长模型
	expConfig := CapacityPlanningConfig{
		GrowthModel:    GrowthModelExponential,
		ForecastDays:   30,
		AlertThreshold: 70,
	}
	expPlanner := NewCapacityPlanner(expConfig)
	expReport := expPlanner.Analyze(history, "test")
	assert.NotNil(t, expReport)

	// 测试对数增长模型
	logConfig := CapacityPlanningConfig{
		GrowthModel:    GrowthModelLogarithmic,
		ForecastDays:   30,
		AlertThreshold: 70,
	}
	logPlanner := NewCapacityPlanner(logConfig)
	logReport := logPlanner.Analyze(history, "test")
	assert.NotNil(t, logReport)
}

func TestCapacityPlanner_Milestones(t *testing.T) {
	config := CapacityPlanningConfig{
		AlertThreshold:    70.0,
		CriticalThreshold: 85.0,
		ForecastDays:      90,
	}

	planner := NewCapacityPlanner(config)

	now := time.Now()
	history := []CapacityHistory{
		{
			Timestamp:    now.AddDate(0, 0, -30),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    500 * 1024 * 1024 * 1024,
			UsagePercent: 50.0,
		},
		{
			Timestamp:    now,
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    650 * 1024 * 1024 * 1024,
			UsagePercent: 65.0,
		},
	}

	report := planner.Analyze(history, "test")

	// 应该有里程碑（70%、80%、90%等）
	assert.NotEmpty(t, report.Milestones)

	// 验证里程碑结构
	for _, m := range report.Milestones {
		assert.NotEmpty(t, m.Name)
		assert.Greater(t, m.Threshold, 0.0)
		assert.NotEmpty(t, m.ActionRequired)
	}
}

// ========== 成本优化测试 ==========

func TestCostOptimizer_AnalyzeWaste(t *testing.T) {
	config := StorageCostConfig{
		CostPerGBMonthly: 0.5,
	}

	optimizer := NewCostOptimizer(config)

	items := []WasteItem{
		{Type: WasteTypeDuplicate, Name: "dup1.txt", WastedBytes: 100 * 1024 * 1024, Recoverable: true},
		{Type: WasteTypeDuplicate, Name: "dup2.txt", WastedBytes: 200 * 1024 * 1024, Recoverable: true},
		{Type: WasteTypeExpired, Name: "old_backup.zip", WastedBytes: 500 * 1024 * 1024, Recoverable: true},
		{Type: WasteTypeTemp, Name: "/tmp/cache", WastedBytes: 50 * 1024 * 1024, Recoverable: true},
	}

	totalCapacity := uint64(10 * 1024 * 1024 * 1024) // 10GB

	summary := optimizer.AnalyzeWaste(items, totalCapacity)

	assert.Equal(t, uint64(850*1024*1024), summary.TotalWastedBytes)
	assert.Equal(t, 2, summary.ItemCounts[WasteTypeDuplicate])
	assert.Equal(t, 1, summary.ItemCounts[WasteTypeExpired])
	assert.Equal(t, 1, summary.ItemCounts[WasteTypeTemp])
	assert.Greater(t, summary.WastePercent, 0.0)
	assert.Greater(t, summary.PotentialSavingsMonthly, 0.0)
}

func TestCostOptimizer_IdentifyOpportunities(t *testing.T) {
	config := StorageCostConfig{
		CostPerGBMonthly: 0.5,
	}

	optimizer := NewCostOptimizer(config)

	wasteItems := []WasteItem{
		{Type: WasteTypeDuplicate, Name: "dup1", WastedBytes: 1024 * 1024 * 1024, Recoverable: true},
		{Type: WasteTypeExpired, Name: "old", WastedBytes: 2 * 1024 * 1024 * 1024, Recoverable: true},
	}

	metrics := []StorageMetrics{
		{
			VolumeName:         "vol1",
			TotalCapacityBytes: 100 * 1024 * 1024 * 1024,
			UsedCapacityBytes:  30 * 1024 * 1024 * 1024,
		},
	}

	costs := []StorageCostResult{
		{VolumeName: "vol1", UsagePercent: 30.0},
	}

	opportunities := optimizer.IdentifyOpportunities(wasteItems, metrics, costs)

	assert.NotEmpty(t, opportunities)

	// 验证机会结构
	for _, opp := range opportunities {
		assert.NotEmpty(t, opp.ID)
		assert.NotEmpty(t, opp.Title)
		assert.NotEmpty(t, opp.Type)
		assert.Greater(t, opp.Priority, 0)
	}
}

func TestCostOptimizer_GenerateReport(t *testing.T) {
	config := StorageCostConfig{
		CostPerGBMonthly: 0.5,
	}

	optimizer := NewCostOptimizer(config)

	wasteItems := []WasteItem{
		{Type: WasteTypeDuplicate, Name: "dup1", WastedBytes: 1024 * 1024 * 1024, Recoverable: true},
		{Type: WasteTypeExpired, Name: "old", WastedBytes: 2 * 1024 * 1024 * 1024, Recoverable: true},
	}

	metrics := []StorageMetrics{
		{
			VolumeName:         "vol1",
			TotalCapacityBytes: 100 * 1024 * 1024 * 1024,
			UsedCapacityBytes:  50 * 1024 * 1024 * 1024,
		},
	}

	costs := []StorageCostResult{
		{VolumeName: "vol1", UsagePercent: 50.0, TotalCostMonthly: 100.0},
	}

	period := ReportPeriod{
		StartTime: time.Now().AddDate(0, -1, 0),
		EndTime:   time.Now(),
	}

	report := optimizer.GenerateReport(wasteItems, metrics, costs, 100*1024*1024*1024, period)

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.NotEmpty(t, report.WasteItems)
	assert.NotEmpty(t, report.Opportunities)
	assert.NotEmpty(t, report.ActionPlan)
	assert.Greater(t, report.OptimizationSummary.TotalSavingsMonthly, 0.0)
}

func TestCostOptimizer_ActionPlanPrioritization(t *testing.T) {
	config := StorageCostConfig{
		CostPerGBMonthly: 0.5,
	}

	optimizer := NewCostOptimizer(config)

	// 创建多个优化机会
	opportunities := []OptimizationOpportunity{
		{
			ID:             "opt1",
			Type:           OptimizationTypeCleanup,
			Title:          "Low priority",
			Priority:       3,
			Implementation: "hard",
			SavingsMonthly: 100,
		},
		{
			ID:             "opt2",
			Type:           OptimizationTypeCleanup,
			Title:          "High priority",
			Priority:       9,
			Implementation: "easy",
			SavingsMonthly: 500,
		},
		{
			ID:             "opt3",
			Type:           OptimizationTypeCleanup,
			Title:          "Medium priority",
			Priority:       6,
			Implementation: "medium",
			SavingsMonthly: 200,
		},
	}

	actionPlan := optimizer.generateActionPlan(opportunities)

	assert.Len(t, actionPlan, 3)
	// 第一个应该是高优先级的
	assert.Equal(t, "High priority", actionPlan[0].Title)
	assert.Equal(t, 1, actionPlan[0].Sequence)
}

// ========== 边界条件测试 ==========

func TestStorageCostCalculator_EmptyMetrics(t *testing.T) {
	config := StorageCostConfig{}
	calculator := NewStorageCostCalculator(config)

	metrics := StorageMetrics{
		VolumeName:         "empty",
		TotalCapacityBytes: 0,
		UsedCapacityBytes:  0,
	}

	result := calculator.Calculate(metrics)

	assert.Equal(t, "empty", result.VolumeName)
	assert.Equal(t, 0.0, result.UsagePercent)
	assert.Equal(t, 0.0, result.CostPerGBMonthly)
}

func TestCapacityPlanner_EmptyHistory(t *testing.T) {
	config := CapacityPlanningConfig{}
	planner := NewCapacityPlanner(config)

	report := planner.Analyze([]CapacityHistory{}, "test")
	assert.Nil(t, report)
}

func TestCapacityPlanner_SingleHistoryPoint(t *testing.T) {
	config := CapacityPlanningConfig{
		ForecastDays: 30,
	}
	planner := NewCapacityPlanner(config)

	history := []CapacityHistory{
		{
			Timestamp:    time.Now(),
			TotalBytes:   1e12,
			UsedBytes:    5e11,
			UsagePercent: 50.0,
		},
	}

	report := planner.Analyze(history, "test")
	// 单点历史也能生成报告，但预测有限
	assert.NotNil(t, report)
}

func TestCostOptimizer_EmptyWasteItems(t *testing.T) {
	config := StorageCostConfig{}
	optimizer := NewCostOptimizer(config)

	summary := optimizer.AnalyzeWaste([]WasteItem{}, 1e12)

	assert.Equal(t, uint64(0), summary.TotalWastedBytes)
	assert.Equal(t, 0.0, summary.WastePercent)
	assert.Equal(t, 0.0, summary.PotentialSavingsMonthly)
}

// ========== 配置更新测试 ==========

func TestStorageCostCalculator_UpdateConfig(t *testing.T) {
	config := StorageCostConfig{
		CostPerGBMonthly: 0.5,
	}
	calculator := NewStorageCostCalculator(config)

	newConfig := StorageCostConfig{
		CostPerGBMonthly: 0.8,
	}
	calculator.UpdateConfig(newConfig)

	assert.Equal(t, 0.8, calculator.GetConfig().CostPerGBMonthly)
}

func TestCapacityPlanner_UpdateConfig(t *testing.T) {
	config := CapacityPlanningConfig{
		AlertThreshold: 70.0,
	}
	planner := NewCapacityPlanner(config)

	newConfig := CapacityPlanningConfig{
		AlertThreshold: 80.0,
	}
	planner.UpdateConfig(newConfig)

	assert.Equal(t, 80.0, planner.GetConfig().AlertThreshold)
}

func TestCostOptimizer_UpdateConfig(t *testing.T) {
	config := StorageCostConfig{
		CostPerGBMonthly: 0.5,
	}
	optimizer := NewCostOptimizer(config)

	newConfig := StorageCostConfig{
		CostPerGBMonthly: 0.8,
	}
	optimizer.UpdateConfig(newConfig)

	assert.Equal(t, 0.8, optimizer.GetConfig().CostPerGBMonthly)
}
