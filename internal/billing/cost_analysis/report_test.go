// Package cost_analysis 提供成本分析报告功能测试
package cost_analysis

import (
	"testing"
	"time"
)

// ========== Mock 数据提供者 ==========

type MockBillingProvider struct{}

func (m *MockBillingProvider) GetUsageRecords(userID, poolID string, start, end time.Time) ([]*UsageRecord, error) {
	return []*UsageRecord{
		{
			ID:            "usage-1",
			UserID:        "user1",
			PoolID:        "pool1",
			StorageUsedGB: 100,
			BandwidthGB:   50,
			RecordedAt:    time.Now(),
		},
	}, nil
}

func (m *MockBillingProvider) GetUserUsageSummary(userID string, start, end time.Time) (*UsageSummary, error) {
	return &UsageSummary{
		UserID:             userID,
		TotalStorageUsedGB: 100,
		TotalBandwidthGB:   50,
	}, nil
}

func (m *MockBillingProvider) GetBillingStats(start, end time.Time) (*BillingStats, error) {
	return &BillingStats{
		TotalStorageUsedGB: 1000,
		TotalBandwidthGB:   500,
		TotalRevenue:       1000.0,
		StorageRevenue:     800.0,
		BandwidthRevenue:   200.0,
	}, nil
}

func (m *MockBillingProvider) GetStoragePrice(poolID string) float64 {
	return 0.1 // 0.1元/GB
}

func (m *MockBillingProvider) GetBandwidthPrice() float64 {
	return 0.5 // 0.5元/GB
}

type MockQuotaProvider struct{}

func (m *MockQuotaProvider) GetAllUsage() ([]*QuotaUsageInfo, error) {
	return []*QuotaUsageInfo{
		{
			QuotaID:     "quota-1",
			TargetID:    "user1",
			TargetName:  "用户1",
			VolumeName:  "pool1",
			HardLimit:   200 * 1024 * 1024 * 1024, // 200GB
			UsedBytes:   100 * 1024 * 1024 * 1024, // 100GB
			Available:   100 * 1024 * 1024 * 1024,
			UsagePercent: 50,
		},
		{
			QuotaID:     "quota-2",
			TargetID:    "user2",
			TargetName:  "用户2",
			VolumeName:  "pool1",
			HardLimit:   500 * 1024 * 1024 * 1024, // 500GB
			UsedBytes:   50 * 1024 * 1024 * 1024,  // 50GB
			Available:   450 * 1024 * 1024 * 1024,
			UsagePercent: 10,
		},
	}, nil
}

func (m *MockQuotaProvider) GetUserUsage(username string) ([]*QuotaUsageInfo, error) {
	return []*QuotaUsageInfo{
		{
			QuotaID:     "quota-1",
			TargetID:    username,
			TargetName:  username,
			VolumeName:  "pool1",
			HardLimit:   200 * 1024 * 1024 * 1024,
			UsedBytes:   100 * 1024 * 1024 * 1024,
			UsagePercent: 50,
		},
	}, nil
}

func (m *MockQuotaProvider) GetPoolUsage(poolID string) (*QuotaUsageInfo, error) {
	return &QuotaUsageInfo{
		QuotaID:     poolID,
		VolumeName:  poolID,
		HardLimit:   1000 * 1024 * 1024 * 1024,
		UsedBytes:   500 * 1024 * 1024 * 1024,
		UsagePercent: 50,
	}, nil
}

// ========== 测试用例 ==========

func TestNewCostAnalysisEngine(t *testing.T) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	if engine == nil {
		t.Fatal("创建引擎失败")
	}

	if engine.config.DefaultCurrency != "CNY" {
		t.Errorf("默认货币应该是 CNY，实际是 %s", engine.config.DefaultCurrency)
	}
}

func TestGenerateStorageTrendReport(t *testing.T) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	report, err := engine.GenerateStorageTrendReport(30)
	if err != nil {
		t.Fatalf("生成存储趋势报告失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告不应为空")
	}

	if report.Type != CostReportStorageTrend {
		t.Errorf("报告类型应该是 storage_trend，实际是 %s", report.Type)
	}

	if report.PeriodEnd.Before(report.PeriodStart) {
		t.Error("结束时间应该在开始时间之后")
	}
}

func TestGenerateResourceUtilizationReport(t *testing.T) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	report, err := engine.GenerateResourceUtilizationReport()
	if err != nil {
		t.Fatalf("生成资源利用率报告失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告不应为空")
	}

	if report.Type != CostReportResourceUtil {
		t.Errorf("报告类型应该是 resource_util，实际是 %s", report.Type)
	}

	// 检查用户利用率统计
	if len(report.UserUtilization) == 0 {
		t.Error("用户利用率统计不应为空")
	}

	// 检查存储池利用率统计
	if len(report.PoolUtilization) == 0 {
		t.Error("存储池利用率统计不应为空")
	}
}

func TestGenerateOptimizationReport(t *testing.T) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	report, err := engine.GenerateOptimizationReport()
	if err != nil {
		t.Fatalf("生成优化建议报告失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告不应为空")
	}

	if report.Type != CostReportOptimization {
		t.Errorf("报告类型应该是 optimization，实际是 %s", report.Type)
	}

	// 检查建议
	if len(report.Recommendations) == 0 {
		t.Log("建议列表可能为空，取决于数据分析结果")
	}
}

func TestGenerateComprehensiveReport(t *testing.T) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	report, err := engine.GenerateComprehensiveReport()
	if err != nil {
		t.Fatalf("生成综合报告失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告不应为空")
	}

	if report.Type != CostReportComprehensive {
		t.Errorf("报告类型应该是 comprehensive，实际是 %s", report.Type)
	}
}

func TestBudgetCRUD(t *testing.T) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	// 创建预算
	budgetConfig := BudgetConfig{
		Name:         "测试预算",
		TotalBudget:  10000.0,
		Period:       "monthly",
		StartDate:    time.Now(),
		EndDate:      time.Now().AddDate(0, 1, 0),
		Enabled:      true,
		AlertThresholds: []float64{50, 75, 90, 100},
	}

	budget, err := engine.CreateBudget(budgetConfig)
	if err != nil {
		t.Fatalf("创建预算失败: %v", err)
	}

	if budget.ID == "" {
		t.Error("预算ID不应为空")
	}

	// 获取预算
	retrieved, err := engine.GetBudget(budget.ID)
	if err != nil {
		t.Fatalf("获取预算失败: %v", err)
	}

	if retrieved.Name != budgetConfig.Name {
		t.Errorf("预算名称应该是 %s，实际是 %s", budgetConfig.Name, retrieved.Name)
	}

	// 列出预算
	budgets := engine.ListBudgets()
	if len(budgets) != 1 {
		t.Errorf("预算数量应该是 1，实际是 %d", len(budgets))
	}

	// 更新预算
	updatedConfig := budgetConfig
	updatedConfig.Name = "更新后的预算"
	updatedConfig.TotalBudget = 20000.0

	updated, err := engine.UpdateBudget(budget.ID, updatedConfig)
	if err != nil {
		t.Fatalf("更新预算失败: %v", err)
	}

	if updated.Name != "更新后的预算" {
		t.Error("预算名称未更新")
	}

	if updated.TotalBudget != 20000.0 {
		t.Error("预算金额未更新")
	}

	// 删除预算
	err = engine.DeleteBudget(budget.ID)
	if err != nil {
		t.Fatalf("删除预算失败: %v", err)
	}

	// 确认删除
	budgets = engine.ListBudgets()
	if len(budgets) != 0 {
		t.Errorf("删除后预算数量应该是 0，实际是 %d", len(budgets))
	}
}

func TestGenerateBudgetTrackingReport(t *testing.T) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	// 创建预算
	budgetConfig := BudgetConfig{
		Name:         "测试预算",
		TotalBudget:  10000.0,
		Period:       "monthly",
		StartDate:    time.Now().AddDate(0, 0, -15), // 15天前开始
		EndDate:      time.Now().AddDate(0, 0, 15),  // 15天后结束
		Enabled:      true,
		AlertThresholds: []float64{50, 75, 90, 100},
	}

	budget, _ := engine.CreateBudget(budgetConfig)

	// 生成预算跟踪报告
	report, err := engine.GenerateBudgetTrackingReport(budget.ID)
	if err != nil {
		t.Fatalf("生成预算跟踪报告失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告不应为空")
	}

	if report.Type != CostReportBudgetTracking {
		t.Errorf("报告类型应该是 budget_tracking，实际是 %s", report.Type)
	}

	if report.TotalBudget != 10000.0 {
		t.Errorf("总预算应该是 10000，实际是 %.2f", report.TotalBudget)
	}

	// 检查剩余天数
	if report.DaysRemaining < 14 || report.DaysRemaining > 16 {
		t.Errorf("剩余天数应该约为 15 天，实际是 %d", report.DaysRemaining)
	}
}

func TestCalculateCostEfficiency(t *testing.T) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	tests := []struct {
		usagePercent float64
		expectedMin  float64
		expectedMax  float64
	}{
		{50, 0.8, 0.9},   // 低于理想范围
		{70, 0.99, 1.0},  // 理想范围
		{90, 0.6, 0.7},   // 高于理想范围
	}

	for _, tt := range tests {
		efficiency := engine.calculateCostEfficiency(tt.usagePercent)
		if efficiency < tt.expectedMin || efficiency > tt.expectedMax {
			t.Errorf("利用率 %.1f%% 的效率应该在 %.2f-%.2f 之间，实际是 %.2f",
				tt.usagePercent, tt.expectedMin, tt.expectedMax, efficiency)
		}
	}
}

func TestRecordTrendData(t *testing.T) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	// 记录趋势数据
	trend := CostTrend{
		Date:          time.Now(),
		StorageCost:   100.0,
		BandwidthCost: 50.0,
		TotalCost:     150.0,
		StorageUsedGB: 1000,
		BandwidthGB:   500,
	}

	engine.RecordTrendData(trend)

	// 验证趋势数据被记录
	if len(engine.trendData) == 0 {
		t.Error("趋势数据未被记录")
	}

	recorded := engine.trendData[len(engine.trendData)-1]
	if recorded.TotalCost != 150.0 {
		t.Errorf("记录的总成本应该是 150，实际是 %.2f", recorded.TotalCost)
	}
}

func TestAlertManagement(t *testing.T) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	// 手动添加一个告警
	alert := &CostAlert{
		ID:        "alert-1",
		Type:      "budget_exceeded",
		Severity:   "critical",
		Message:   "预算超支",
		Value:     11000,
		Threshold: 10000,
		CreatedAt: time.Now(),
	}
	engine.alerts = append(engine.alerts, alert)

	// 获取告警列表
	alerts := engine.GetAlerts()
	if len(alerts) != 1 {
		t.Errorf("告警数量应该是 1，实际是 %d", len(alerts))
	}

	// 确认告警
	err := engine.AcknowledgeAlert("alert-1")
	if err != nil {
		t.Fatalf("确认告警失败: %v", err)
	}

	// 验证确认状态
	alerts = engine.GetAlerts()
	if !alerts[0].Acknowledged {
		t.Error("告警应该已被确认")
	}

	// 确认不存在的告警
	err = engine.AcknowledgeAlert("non-existent")
	if err == nil {
		t.Error("确认不存在的告警应该返回错误")
	}
}

// ========== 基准测试 ==========

func BenchmarkGenerateStorageTrendReport(b *testing.B) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.GenerateStorageTrendReport(30)
	}
}

func BenchmarkGenerateResourceUtilizationReport(b *testing.B) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.GenerateResourceUtilizationReport()
	}
}

func BenchmarkGenerateComprehensiveReport(b *testing.B) {
	config := DefaultAnalysisConfig()
	engine := NewCostAnalysisEngine("/tmp/cost_test", &MockBillingProvider{}, &MockQuotaProvider{}, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.GenerateComprehensiveReport()
	}
}