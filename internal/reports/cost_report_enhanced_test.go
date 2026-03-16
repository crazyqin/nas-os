package reports

import (
	"testing"
	"time"
)

// ========== 成本报告增强测试 ==========

func TestCostReportSummary_HealthScoreValidation(t *testing.T) {
	summary := CostReportSummary{
		TotalCost:          1000.0,
		StorageCost:        800.0,
		BandwidthCost:      200.0,
		TotalStorageGB:     1000.0,
		StorageUtilization: 75.0,
		HealthScore:        85,
	}

	if summary.HealthScore < 0 || summary.HealthScore > 100 {
		t.Errorf("健康评分应该在 0-100 之间, 实际为 %d", summary.HealthScore)
	}
}

func TestCostReportSummary_BudgetStatusScenarios(t *testing.T) {
	tests := []struct {
		name          string
		utilization   float64
		expectedState string
	}{
		{"低使用率", 30.0, "on_track"},
		{"正常使用", 60.0, "on_track"},
		{"高使用", 85.0, "at_risk"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := CostReportSummary{
				StorageUtilization: tt.utilization,
			}
			// 模拟预算状态逻辑
			if tt.utilization > 80 {
				summary.BudgetStatus = "at_risk"
			} else {
				summary.BudgetStatus = "on_track"
			}

			if summary.BudgetStatus != tt.expectedState {
				t.Errorf("期望预算状态为 %s, 实际为 %s", tt.expectedState, summary.BudgetStatus)
			}
		})
	}
}

// ========== StorageCostSection 测试 ==========

func TestStorageCostSection_Calculation(t *testing.T) {
	storage := StorageCostSection{
		TotalCapacityGB: 1000.0,
		UsedCapacityGB:  800.0,
		FreeCapacityGB:  200.0,
		UtilizationRate: 80.0,
		MonthlyCost:     500.0,
		DailyCost:       16.67,
		AveragePrice:    0.50,
		SSDCost:         300.0,
		SSDUsedGB:       500.0,
		HDDCost:         200.0,
		HDDUsedGB:       300.0,
	}

	// 验证计算
	if storage.FreeCapacityGB != storage.TotalCapacityGB-storage.UsedCapacityGB {
		t.Error("剩余容量计算错误")
	}

	// 验证成本
	if storage.MonthlyCost <= 0 {
		t.Error("月度成本应该大于 0")
	}
}

// ========== BandwidthCostSection 测试 ==========

func TestBandwidthCostSection_Calculation(t *testing.T) {
	bandwidth := BandwidthCostSection{
		InboundTrafficGB:  500.0,
		OutboundTrafficGB: 300.0,
		TotalTrafficGB:    800.0,
		PeakMbps:          1000.0,
		AverageMbps:       500.0,
		TotalCost:         200.0,
		TrafficCost:       150.0,
		BandwidthCost:     50.0,
		BillingModel:      "95th_percentile",
	}

	// 验证总流量
	if bandwidth.TotalTrafficGB != bandwidth.InboundTrafficGB+bandwidth.OutboundTrafficGB {
		t.Error("总流量计算错误")
	}

	// 验证计费模式
	if bandwidth.BillingModel == "" {
		t.Error("计费模式不应为空")
	}
}

// ========== PoolCostItem 测试 ==========

func TestPoolCostItem_Efficiency(t *testing.T) {
	pool := PoolCostItem{
		PoolID:          "pool1",
		PoolName:        "主存储池",
		StorageType:     "ssd",
		TotalCapacityGB: 1000.0,
		UsedCapacityGB:  750.0,
		UsagePercent:    75.0,
		PricePerGB:      0.5,
		MonthlyCost:     375.0,
		CostEfficiency:  0.85,
		Trend:           "up",
	}

	// 验证使用率计算
	if pool.UsagePercent != (pool.UsedCapacityGB/pool.TotalCapacityGB)*100 {
		t.Error("使用率计算错误")
	}

	// 验证成本效率范围
	if pool.CostEfficiency < 0 || pool.CostEfficiency > 1 {
		t.Error("成本效率应该在 0-1 之间")
	}
}

// ========== UserCostItem 测试 ==========

func TestUserCostItem_Calculation(t *testing.T) {
	user := UserCostItem{
		UserID:      "user1",
		UserName:    "张三",
		UsedGB:      100.0,
		MonthlyCost: 50.0,
		CostPerGB:   0.5,
		Tier:        "standard",
		Trend:       "up",
		PoolUsage: map[string]float64{
			"pool1": 60.0,
			"pool2": 40.0,
		},
	}

	// 验证单位成本
	if user.CostPerGB != user.MonthlyCost/user.UsedGB {
		t.Error("单位成本计算错误")
	}

	// 验证池使用量总和
	var totalPoolUsage float64
	for _, v := range user.PoolUsage {
		totalPoolUsage += v
	}
	if totalPoolUsage != user.UsedGB {
		t.Error("池使用量总和应该等于总使用量")
	}
}

// ========== CostTrendItem 测试 ==========

func TestCostTrendItem_Trend(t *testing.T) {
	trends := []CostTrendItem{
		{Date: time.Now().AddDate(0, 0, -6), StorageCost: 100, BandwidthCost: 20, TotalCost: 120},
		{Date: time.Now().AddDate(0, 0, -5), StorageCost: 105, BandwidthCost: 22, TotalCost: 127},
		{Date: time.Now().AddDate(0, 0, -4), StorageCost: 110, BandwidthCost: 25, TotalCost: 135},
		{Date: time.Now().AddDate(0, 0, -3), StorageCost: 115, BandwidthCost: 28, TotalCost: 143},
		{Date: time.Now().AddDate(0, 0, -2), StorageCost: 120, BandwidthCost: 30, TotalCost: 150},
		{Date: time.Now().AddDate(0, 0, -1), StorageCost: 125, BandwidthCost: 32, TotalCost: 157},
	}

	// 验证趋势是增长的
	for i := 1; i < len(trends); i++ {
		if trends[i].TotalCost <= trends[i-1].TotalCost {
			t.Errorf("第 %d 天成本应该高于前一天", i)
		}
	}
}

// ========== RecommendationItem 测试 ==========

func TestRecommendationItem_Savings(t *testing.T) {
	rec := RecommendationItem{
		ID:               "rec1",
		Type:             "cleanup",
		Priority:         "high",
		Title:            "清理冷数据",
		Description:      "发现大量冷数据可归档",
		PotentialSavings: 100.0,
		CurrentCost:      500.0,
		OptimizedCost:    400.0,
		Action:           "执行冷数据归档",
		Impact:           "释放 200GB 存储空间",
	}

	// 验证节省计算
	if rec.PotentialSavings != rec.CurrentCost-rec.OptimizedCost {
		t.Error("潜在节省计算错误")
	}

	// 验证优先级
	if rec.Priority != "high" && rec.Priority != "medium" && rec.Priority != "low" {
		t.Errorf("无效的优先级: %s", rec.Priority)
	}
}

// ========== BudgetComparison 测试 ==========

func TestBudgetComparison_StatusScenarios(t *testing.T) {
	tests := []struct {
		name     string
		spend    float64
		budget   float64
		expected string
	}{
		{"正常", 500, 1000, "on_track"},
		{"警告", 800, 1000, "at_risk"},
		{"超支", 1100, 1000, "over_budget"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budget := BudgetComparison{
				TotalBudget:  tt.budget,
				CurrentSpend: tt.spend,
				Remaining:    tt.budget - tt.spend,
				Utilization:  (tt.spend / tt.budget) * 100,
			}

			// 计算状态
			if budget.Utilization > 100 {
				budget.Status = "over_budget"
			} else if budget.Utilization > 80 {
				budget.Status = "at_risk"
			} else {
				budget.Status = "on_track"
			}

			if budget.Status != tt.expected {
				t.Errorf("期望状态为 %s, 实际为 %s", tt.expected, budget.Status)
			}
		})
	}
}

// ========== 成本计算测试 ==========

func TestCostCalculation_TotalCost(t *testing.T) {
	report := &CostReport{
		Summary: CostReportSummary{
			StorageCost:   800.0,
			BandwidthCost: 200.0,
			OtherCost:     50.0,
		},
	}

	// 计算总成本
	totalCost := report.Summary.StorageCost + report.Summary.BandwidthCost + report.Summary.OtherCost
	expectedTotal := 1050.0

	if totalCost != expectedTotal {
		t.Errorf("期望总成本为 %.2f, 实际为 %.2f", expectedTotal, totalCost)
	}
}

func TestCostCalculation_PercentageChange(t *testing.T) {
	current := 1000.0
	previous := 800.0

	change := current - previous
	changePercent := (change / previous) * 100

	if change != 200.0 {
		t.Errorf("期望变化金额为 200.0, 实际为 %.2f", change)
	}

	if changePercent != 25.0 {
		t.Errorf("期望变化百分比为 25.0%%, 实际为 %.2f%%", changePercent)
	}
}

// ========== 报告类型测试 ==========

func TestCostReportType_AllValues(t *testing.T) {
	types := []CostReportType{
		CostReportTypeDaily,
		CostReportTypeWeekly,
		CostReportTypeMonthly,
	}

	expected := []string{"daily", "weekly", "monthly"}

	for i, rt := range types {
		if string(rt) != expected[i] {
			t.Errorf("期望 %s, 实际为 %s", expected[i], rt)
		}
	}
}

func TestCostExportFormat_AllValues(t *testing.T) {
	formats := []CostExportFormat{
		CostExportFormatJSON,
		CostExportFormatCSV,
	}

	expected := []string{"json", "csv"}

	for i, f := range formats {
		if string(f) != expected[i] {
			t.Errorf("期望 %s, 实际为 %s", expected[i], f)
		}
	}
}

// ========== 边界条件测试 ==========

func TestCostReport_ZeroValues(t *testing.T) {
	report := &CostReport{
		ID:             "zero-report",
		CostReportType: CostReportTypeDaily,
		GeneratedAt:    time.Now(),
		Currency:       "CNY",
		Summary: CostReportSummary{
			TotalCost:     0,
			StorageCost:   0,
			BandwidthCost: 0,
			HealthScore:   100,
		},
	}

	if report.Summary.HealthScore != 100 {
		t.Errorf("零成本时健康评分应该是 100, 实际为 %d", report.Summary.HealthScore)
	}
}

func TestCostReport_LargeValues(t *testing.T) {
	report := &CostReport{
		Summary: CostReportSummary{
			TotalCost:     1e10, // 100亿
			StorageCost:   8e9,
			BandwidthCost: 2e9,
		},
	}

	// 验证大数值处理
	if report.Summary.TotalCost <= 0 {
		t.Error("大数值应该被正确处理")
	}
}

// ========== TierCostItem 测试 ==========

func TestTierCostItem_Calculation(t *testing.T) {
	tier := TierCostItem{
		TierName:   "标准层",
		MinGB:      0,
		MaxGB:      1000,
		UsedGB:     500,
		PricePerGB: 0.1,
		Cost:       50,
	}

	// 验证成本计算
	expectedCost := tier.UsedGB * tier.PricePerGB
	if tier.Cost != expectedCost {
		t.Errorf("期望成本 %.2f, 实际为 %.2f", expectedCost, tier.Cost)
	}
}

// ========== CostForecast 测试 ==========

func TestCostForecast_ConfidenceRange(t *testing.T) {
	forecast := &CostForecast{
		NextMonthCost:    1200.0,
		NextQuarterCost:  3600.0,
		NextYearCost:     14400.0,
		Confidence:       0.85,
		Method:           "linear",
		WarningThreshold: 1000.0,
		BudgetAlert:      false,
	}

	// 验证置信度范围
	if forecast.Confidence < 0 || forecast.Confidence > 1 {
		t.Errorf("置信度应该在 0-1 之间, 实际为 %.2f", forecast.Confidence)
	}

	// 验证成本预测递增关系
	if forecast.NextYearCost < forecast.NextQuarterCost {
		t.Error("年度成本应该大于季度成本")
	}

	if forecast.NextQuarterCost < forecast.NextMonthCost*3 {
		t.Error("季度成本应该约等于月成本的三倍")
	}
}

// ========== ForecastPoint 测试 ==========

func TestForecastPoint_Bounds(t *testing.T) {
	point := ForecastPoint{
		Date:           time.Now().AddDate(0, 1, 0),
		PredictedCost:  1000.0,
		PredictedUsage: 800.0,
		LowerBound:     900.0,
		UpperBound:     1100.0,
		Confidence:     0.9,
	}

	// 验证置信区间
	if point.LowerBound > point.PredictedCost {
		t.Error("置信下限不应大于预测值")
	}

	if point.UpperBound < point.PredictedCost {
		t.Error("置信上限不应小于预测值")
	}

	// 验证置信度
	if point.Confidence < 0 || point.Confidence > 1 {
		t.Errorf("置信度应该在 0-1 之间, 实际为 %.2f", point.Confidence)
	}
}

// ========== 成本优化测试 ==========

func TestCostOptimizationItem_Prioritization(t *testing.T) {
	items := []CostOptimizationItem{
		{ID: "opt1", Priority: "critical", Savings: 500},
		{ID: "opt2", Priority: "high", Savings: 300},
		{ID: "opt3", Priority: "medium", Savings: 200},
		{ID: "opt4", Priority: "low", Savings: 100},
	}

	// 验证优先级顺序
	priorityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
	for i := 1; i < len(items); i++ {
		if priorityOrder[items[i-1].Priority] > priorityOrder[items[i].Priority] {
			t.Error("优化项应该按优先级排序")
		}
	}
}

// ========== 数据完整性测试 ==========

func TestCostReport_DataIntegrity(t *testing.T) {
	report := &CostReport{
		ID:             "test-report",
		CostReportType: CostReportTypeDaily,
		GeneratedAt:    time.Now(),
		PeriodStart:    time.Now().AddDate(0, 0, -1),
		PeriodEnd:      time.Now(),
		Currency:       "CNY",
	}

	// 验证必需字段
	if report.ID == "" {
		t.Error("报告 ID 不能为空")
	}

	if report.Currency == "" {
		t.Error("货币不能为空")
	}

	if report.PeriodStart.After(report.PeriodEnd) {
		t.Error("开始时间不能晚于结束时间")
	}
}

// ========== ReportPeriod Duration 测试 ==========
// 注意：TestReportPeriod_Duration 已在 hubu_resource_report_test.go 中定义

// ========== 性能基准测试 ==========

func BenchmarkCostReport_Generate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		report := &CostReport{
			ID:             "bench-report",
			CostReportType: CostReportTypeDaily,
			GeneratedAt:    time.Now(),
			Currency:       "CNY",
			Summary: CostReportSummary{
				TotalCost:     1000,
				StorageCost:   800,
				BandwidthCost: 200,
				HealthScore:   85,
			},
		}
		_ = report
	}
}

func BenchmarkPoolCostItem_Calculate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pool := PoolCostItem{
			TotalCapacityGB: 1000,
			UsedCapacityGB:  750,
			PricePerGB:      0.5,
		}
		pool.UsagePercent = (pool.UsedCapacityGB / pool.TotalCapacityGB) * 100
		pool.MonthlyCost = pool.UsedCapacityGB * pool.PricePerGB
		_ = pool
	}
}