package storagecostanalyzer

import (
	"testing"
	"time"
)

func TestNewReportGenerator(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	generator := NewReportGenerator(manager)

	if generator == nil {
		t.Fatal("NewReportGenerator returned nil")
	}
	if generator.manager != manager {
		t.Error("Generator manager mismatch")
	}
}

func TestGenerateMultiDimensionReport(t *testing.T) {
	config := &Config{
		Enabled:        true,
		AlertThreshold: 80.0,
	}
	manager := NewManager(config)

	now := time.Now()
	manager.nowFunc = func() time.Time { return now }

	// 注册层级并添加记录
	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD存储",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         8,
	})

	manager.RegisterTier(TierHDD, TierConfig{
		Tier:           TierHDD,
		Name:           "HDD存储",
		CostPerTBMonth: 30,
		CapacityTB:     50,
		UsedTB:         40,
	})

	// 添加成本记录
	manager.RecordCost(TierSSD, CategoryHardware, 5000)
	manager.RecordCost(TierSSD, CategoryPower, 500)
	manager.RecordCost(TierHDD, CategoryHardware, 3000)
	manager.RecordCost(TierHDD, CategoryPower, 300)

	generator := NewReportGenerator(manager)

	// 测试月度报表
	report, err := generator.GenerateMultiDimensionReport("monthly")
	if err != nil {
		t.Fatalf("GenerateMultiDimensionReport failed: %v", err)
	}

	if report == nil {
		t.Fatal("Report should not be nil")
	}

	// 验证基本字段
	if report.ID == "" {
		t.Error("Report ID should not be empty")
	}
	if report.Title == "" {
		t.Error("Report title should not be empty")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should not be zero")
	}
	if report.PeriodStart.IsZero() {
		t.Error("PeriodStart should not be zero")
	}
	if report.PeriodEnd.IsZero() {
		t.Error("PeriodEnd should not be zero")
	}

	// 验证摘要
	if report.Summary.TotalCost <= 0 {
		t.Error("Total cost should be positive")
	}
	if report.Summary.TotalCapacityTB <= 0 {
		t.Error("Total capacity should be positive")
	}
	if report.Summary.TotalUsedTB <= 0 {
		t.Error("Total used should be positive")
	}
	if report.Summary.OverallUtilization <= 0 {
		t.Error("Overall utilization should be positive")
	}

	// 验证层级分解
	if len(report.TierBreakdown) != 2 {
		t.Errorf("Expected 2 tier breakdowns, got %d", len(report.TierBreakdown))
	}

	// 验证成本类别分解
	if len(report.CategoryBreakdown) == 0 {
		t.Error("Category breakdown should not be empty")
	}

	// 验证供应商分解
	if len(report.ProviderBreakdown) == 0 {
		t.Error("Provider breakdown should not be empty")
	}

	// 验证时间序列
	if len(report.TimeSeries) == 0 {
		t.Error("Time series should not be empty")
	}
}

func TestGenerateMultiDimensionReportPeriods(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	manager.nowFunc = time.Now

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         5,
	})

	generator := NewReportGenerator(manager)

	// 测试不同周期
	periods := []string{"monthly", "quarterly", "yearly"}
	for _, period := range periods {
		report, err := generator.GenerateMultiDimensionReport(period)
		if err != nil {
			t.Errorf("GenerateMultiDimensionReport(%s) failed: %v", period, err)
			continue
		}
		if report == nil {
			t.Errorf("Report for %s should not be nil", period)
		}
	}
}

func TestGenerateExecutiveSummary(t *testing.T) {
	config := &Config{
		Enabled:        true,
		AlertThreshold: 80.0,
	}
	manager := NewManager(config)
	manager.nowFunc = time.Now

	// 注册高压层级
	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD存储",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         9.5, // 95% 利用率
	})

	generator := NewReportGenerator(manager)

	summary, err := generator.GenerateExecutiveSummary("monthly")
	if err != nil {
		t.Fatalf("GenerateExecutiveSummary failed: %v", err)
	}

	if summary == nil {
		t.Fatal("Summary should not be nil")
	}

	if summary.Period != "monthly" {
		t.Errorf("Expected period 'monthly', got %s", summary.Period)
	}
	if summary.TotalCost <= 0 {
		t.Error("Total cost should be positive")
	}
	if summary.OverallUtilization <= 0 {
		t.Error("Overall utilization should be positive")
	}

	// 高利用率应该生成洞察
	if len(summary.KeyInsights) == 0 {
		t.Error("High utilization should generate insights")
	}

	// 高利用率应该生成建议
	if len(summary.Recommendations) == 0 {
		t.Error("High utilization should generate recommendations")
	}

	// 高利用率应该生成风险
	if len(summary.Risks) == 0 {
		t.Error("High utilization should generate risks")
	}

	// 验证风险级别
	for _, risk := range summary.Risks {
		if risk.Level == "" {
			t.Error("Risk level should not be empty")
		}
		if risk.Description == "" {
			t.Error("Risk description should not be empty")
		}
		if risk.Mitigation == "" {
			t.Error("Risk mitigation should not be empty")
		}
	}
}

func TestGenerateTrendReport(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	manager.nowFunc = time.Now

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         5,
	})

	manager.RegisterTier(TierHDD, TierConfig{
		Tier:           TierHDD,
		Name:           "HDD",
		CostPerTBMonth: 30,
		CapacityTB:     50,
		UsedTB:         30,
	})

	generator := NewReportGenerator(manager)

	report, err := generator.GenerateTrendReport(12)
	if err != nil {
		t.Fatalf("GenerateTrendReport failed: %v", err)
	}

	if report == nil {
		t.Fatal("Trend report should not be nil")
	}

	if report.AnalysisMonths != 12 {
		t.Errorf("Expected 12 months, got %d", report.AnalysisMonths)
	}
	if len(report.MonthlyTrends) != 13 { // 包括当前月
		t.Errorf("Expected 13 monthly trends (including current), got %d", len(report.MonthlyTrends))
	}
	if report.CostTrend == "" {
		t.Error("Cost trend should not be empty")
	}
	if report.UtilizationTrend == "" {
		t.Error("Utilization trend should not be empty")
	}

	// 验证月度趋势数据
	for i, trend := range report.MonthlyTrends {
		if trend.Month.IsZero() {
			t.Errorf("Month %d: month should not be zero", i)
		}
		if trend.TotalCost < 0 {
			t.Errorf("Month %d: cost should not be negative", i)
		}
		if trend.CapacityTB <= 0 {
			t.Errorf("Month %d: capacity should be positive", i)
		}
		if trend.Utilization < 0 || trend.Utilization > 100 {
			t.Errorf("Month %d: utilization should be 0-100%%, got %f", i, trend.Utilization)
		}
	}
}

func TestGenerateCostAllocationReport(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	manager.nowFunc = time.Now

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         8,
	})

	manager.RegisterTier(TierHDD, TierConfig{
		Tier:           TierHDD,
		Name:           "HDD",
		CostPerTBMonth: 30,
		CapacityTB:     50,
		UsedTB:         40,
	})

	generator := NewReportGenerator(manager)

	report, err := generator.GenerateCostAllocationReport()
	if err != nil {
		t.Fatalf("GenerateCostAllocationReport failed: %v", err)
	}

	if report == nil {
		t.Fatal("Cost allocation report should not be nil")
	}

	if report.TotalMonthlyCost <= 0 {
		t.Error("Total monthly cost should be positive")
	}
	if len(report.TierAllocations) != 2 {
		t.Errorf("Expected 2 tier allocations, got %d", len(report.TierAllocations))
	}
	if len(report.EfficiencyMetrics) != 2 {
		t.Errorf("Expected 2 efficiency metrics, got %d", len(report.EfficiencyMetrics))
	}

	// 验证层级分配
	totalShare := 0.0
	for _, alloc := range report.TierAllocations {
		if alloc.MonthlyCost <= 0 {
			t.Error("Tier monthly cost should be positive")
		}
		if alloc.CostPerTB <= 0 {
			t.Error("Cost per TB should be positive")
		}
		totalShare += alloc.CostShare
	}
	if totalShare < 99.9 || totalShare > 100.1 {
		t.Errorf("Cost shares should sum to ~100%%, got %f", totalShare)
	}

	// 验证效率指标
	for _, metric := range report.EfficiencyMetrics {
		if metric.Utilization < 0 || metric.Utilization > 100 {
			t.Errorf("Utilization should be 0-100%%, got %f", metric.Utilization)
		}
		if metric.CostEfficiency < 0 {
			t.Error("Cost efficiency should not be negative")
		}
	}
}
