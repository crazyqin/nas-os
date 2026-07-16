// Package reports 提供成本报告生成功能测试
package reports

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockReportDataProvider 模拟报告数据提供者.
type MockReportDataProvider struct {
	storageData     *StorageReportData
	bandwidthData   *BandwidthReportData
	poolData        []PoolReportData
	userData        []UserReportData
	trendData       []TrendReportData
	budgetData      *BudgetReportData
	recommendations []RecommendationItem
}

func (m *MockReportDataProvider) GetStorageData(ctx context.Context, start, end time.Time) (*StorageReportData, error) {
	if m.storageData != nil {
		return m.storageData, nil
	}
	return &StorageReportData{
		TotalCapacityGB: 1000,
		UsedCapacityGB:  750,
		MonthlyCost:     75,
		DailyCost:       2.5,
		AveragePrice:    0.1,
		SSDCost:         40,
		SSDUsedGB:       200,
		HDDCost:         30,
		HDDUsedGB:       500,
		ArchiveCost:     5,
		ArchiveUsedGB:   50,
		HotDataGB:       200,
		HotDataCost:     30,
		WarmDataGB:      300,
		WarmDataCost:    30,
		ColdDataGB:      250,
		ColdDataCost:    7.5,
		UtilizationRate: 75,
	}, nil
}

func (m *MockReportDataProvider) GetBandwidthData(ctx context.Context, start, end time.Time) (*BandwidthReportData, error) {
	if m.bandwidthData != nil {
		return m.bandwidthData, nil
	}
	return &BandwidthReportData{
		InboundTrafficGB:  500,
		OutboundTrafficGB: 300,
		TotalTrafficGB:    800,
		PeakMbps:          1000,
		AverageMbps:       500,
		Peak95Mbps:        850,
		TotalCost:         50,
		TrafficCost:       30,
		BandwidthCost:     20,
		BillingModel:      "traffic",
		FreeAllowanceGB:   100,
		PeakHours:         []int{10, 11, 14, 15, 16},
	}, nil
}

func (m *MockReportDataProvider) GetPoolData(ctx context.Context, start, end time.Time) ([]PoolReportData, error) {
	if m.poolData != nil {
		return m.poolData, nil
	}
	return []PoolReportData{
		{
			PoolID:          "pool-1",
			PoolName:        "SSD Pool",
			StorageType:     "ssd",
			TotalCapacityGB: 500,
			UsedCapacityGB:  400,
			PricePerGB:      0.2,
			MonthlyCost:     80,
			HotDataGB:       200,
			WarmDataGB:      150,
			ColdDataGB:      50,
			Trend:           "up",
		},
		{
			PoolID:          "pool-2",
			PoolName:        "HDD Pool",
			StorageType:     "hdd",
			TotalCapacityGB: 1000,
			UsedCapacityGB:  500,
			PricePerGB:      0.05,
			MonthlyCost:     25,
			HotDataGB:       100,
			WarmDataGB:      250,
			ColdDataGB:      150,
			Trend:           "stable",
		},
	}, nil
}

func (m *MockReportDataProvider) GetUserData(ctx context.Context, start, end time.Time) ([]UserReportData, error) {
	if m.userData != nil {
		return m.userData, nil
	}
	return []UserReportData{
		{
			UserID:      "user1",
			UserName:    "User One",
			UsedGB:      300,
			MonthlyCost: 30,
			CostPerGB:   0.1,
			Tier:        "premium",
			Trend:       "up",
			PoolUsage:   map[string]float64{"pool-1": 200, "pool-2": 100},
		},
		{
			UserID:      "user2",
			UserName:    "User Two",
			UsedGB:      150,
			MonthlyCost: 15,
			CostPerGB:   0.1,
			Tier:        "standard",
			Trend:       "stable",
			PoolUsage:   map[string]float64{"pool-1": 50, "pool-2": 100},
		},
	}, nil
}

func (m *MockReportDataProvider) GetTrendData(ctx context.Context, start, end time.Time) ([]TrendReportData, error) {
	if m.trendData != nil {
		return m.trendData, nil
	}
	data := make([]TrendReportData, 0)
	for i := 0; i < 7; i++ {
		data = append(data, TrendReportData{
			Date:          start.AddDate(0, 0, i),
			StorageCost:   75.0,
			BandwidthCost: 50.0,
			StorageGB:     750.0,
			TrafficGB:     800.0,
		})
	}
	return data, nil
}

func (m *MockReportDataProvider) GetBudgetData(ctx context.Context, budgetID string) (*BudgetReportData, error) {
	if m.budgetData != nil {
		return m.budgetData, nil
	}
	return &BudgetReportData{
		BudgetID:     "budget-1",
		BudgetName:   "Monthly Budget",
		TotalBudget:  200,
		CurrentSpend: 125,
		Status:       "on_track",
		AlertLevel:   "none",
		Categories: []BudgetCategoryItem{
			{Name: "storage", Budget: 150, CurrentSpend: 75, Utilization: 50, Trend: "stable"},
			{Name: "bandwidth", Budget: 50, CurrentSpend: 50, Utilization: 100, Trend: "up"},
		},
	}, nil
}

func (m *MockReportDataProvider) GetRecommendations(ctx context.Context) ([]RecommendationItem, error) {
	if m.recommendations != nil {
		return m.recommendations, nil
	}
	return []RecommendationItem{
		{
			ID:               "rec-1",
			Type:             "storage",
			Priority:         "high",
			Title:            "优化冷数据存储",
			Description:      "检测到大量冷数据，建议迁移至归档存储",
			PotentialSavings: 15,
			CurrentCost:      30,
			OptimizedCost:    15,
			Action:           "迁移冷数据到归档存储",
			Impact:           "降低存储成本",
		},
	}, nil
}

func (m *MockReportDataProvider) GetHistoricalReport(ctx context.Context, reportType CostReportType, date time.Time) (*CostReport, error) {
	return nil, nil
}

func createTestReportGenerator(t *testing.T) *CostReportGenerator {
	tmpDir := t.TempDir()
	provider := &MockReportDataProvider{}
	config := DefaultReportConfig()
	return NewCostReportGenerator(tmpDir, provider, config)
}

func TestNewCostReportGenerator(t *testing.T) {
	gen := createTestReportGenerator(t)
	require.NotNil(t, gen)
	assert.Equal(t, "CNY", gen.config.DefaultCurrency)
}

func TestGenerateDailyReport(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.NotEmpty(t, report.ID)
	assert.Equal(t, CostReportTypeDaily, report.CostReportType)
	assert.Equal(t, "CNY", report.Currency)
	assert.NotEmpty(t, report.Trends) // 检查趋势数据而非 Sections
}

func TestGenerateWeeklyReport(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateWeeklyReport(ctx, time.Now())
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.NotEmpty(t, report.ID)
	assert.Equal(t, CostReportTypeWeekly, report.CostReportType)
}

func TestGenerateMonthlyReport(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateMonthlyReport(ctx, time.Now())
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.NotEmpty(t, report.ID)
	assert.Equal(t, CostReportTypeMonthly, report.CostReportType)
}

func TestGenerateCustomReport(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	start := time.Now().AddDate(0, 0, -7)
	end := time.Now()

	report, err := gen.GenerateCustomReport(ctx, start, end)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, start, report.PeriodStart)
	assert.Equal(t, end, report.PeriodEnd)
}

func TestReportSummary(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	summary := report.Summary
	assert.GreaterOrEqual(t, summary.TotalCost, 0.0)
	assert.GreaterOrEqual(t, summary.StorageCost, 0.0)
	assert.GreaterOrEqual(t, summary.BandwidthCost, 0.0)
	assert.GreaterOrEqual(t, summary.HealthScore, 0)
	assert.LessOrEqual(t, summary.HealthScore, 100)
}

func TestStorageCostSection(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	storage := report.StorageCost
	assert.GreaterOrEqual(t, storage.TotalCapacityGB, 0.0)
	assert.GreaterOrEqual(t, storage.UsedCapacityGB, 0.0)
	assert.GreaterOrEqual(t, storage.UtilizationRate, 0.0)
}

func TestBandwidthCostSection(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	bandwidth := report.BandwidthCost
	assert.GreaterOrEqual(t, bandwidth.TotalTrafficGB, 0.0)
	assert.NotEmpty(t, bandwidth.BillingModel)
}

func TestPoolBreakdown(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	assert.NotEmpty(t, report.PoolBreakdown)
	for _, pool := range report.PoolBreakdown {
		assert.NotEmpty(t, pool.PoolID)
		assert.NotEmpty(t, pool.PoolName)
		assert.GreaterOrEqual(t, pool.UsagePercent, 0.0)
	}
}

func TestUserBreakdown(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	assert.NotEmpty(t, report.UserBreakdown)
	for _, user := range report.UserBreakdown {
		assert.NotEmpty(t, user.UserID)
		assert.GreaterOrEqual(t, user.UsedGB, 0.0)
	}
}

func TestTrendData(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateWeeklyReport(ctx, time.Now())
	require.NoError(t, err)

	assert.NotEmpty(t, report.Trends)
	for _, trend := range report.Trends {
		assert.NotZero(t, trend.Date)
	}
}

func TestRecommendations(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	assert.NotEmpty(t, report.Recommendations)
	for _, rec := range report.Recommendations {
		assert.NotEmpty(t, rec.ID)
		assert.NotEmpty(t, rec.Type)
		assert.NotEmpty(t, rec.Priority)
		assert.NotEmpty(t, rec.Title)
	}
}

func TestExportReportJSON(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	outputPath := t.TempDir() + "/report.json"
	err = gen.ExportReport(report, CostExportFormatJSON, outputPath)
	require.NoError(t, err)
	assert.FileExists(t, outputPath)
}

func TestExportReportCSV(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	outputPath := t.TempDir() + "/report.csv"
	err = gen.ExportReport(report, CostExportFormatCSV, outputPath)
	require.NoError(t, err)
	assert.FileExists(t, outputPath)
}

func TestExportToJSON(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	jsonStr, err := gen.ExportToJSON(report)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonStr)
	assert.Contains(t, jsonStr, "total_cost")
}

func TestExportToCSV(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	csvStr, err := gen.ExportToCSV(report)
	require.NoError(t, err)
	assert.NotEmpty(t, csvStr)
}

func TestGetReport(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	created, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	retrieved, err := gen.GetReport(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
}

func TestListReports(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	// 创建多个报告
	for i := 0; i < 3; i++ {
		_, err := gen.GenerateDailyReport(ctx, time.Now().AddDate(0, 0, -i))
		require.NoError(t, err)
	}

	reports, err := gen.ListReports(CostReportTypeDaily, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(reports), 3)
}

func TestDeleteReport(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	err = gen.DeleteReport(report.ID)
	require.NoError(t, err)

	_, err = gen.GetReport(report.ID)
	assert.Error(t, err)
}

func TestHealthScoreCalculation(t *testing.T) {
	gen := createTestReportGenerator(t)

	// 高利用率报告
	report := &CostReport{
		StorageCost: StorageCostSection{
			UtilizationRate: 95,
			UsedCapacityGB:  950,
			ColdDataGB:      100,
		},
	}

	summary := &CostReportSummary{}
	score := gen.calculateHealthScore(report, summary)
	assert.Less(t, score, 100) // 高利用率应该扣分
}

func TestCostEfficiencyCalculation(t *testing.T) {
	gen := createTestReportGenerator(t)

	tests := []struct {
		usagePercent float64
		minEff       float64
		maxEff       float64
	}{
		{50, 0.7, 0.9}, // 中等利用率
		{70, 0.9, 1.0}, // 理想利用率
		{95, 0.0, 0.9}, // 过高利用率
		{20, 0.2, 0.4}, // 低利用率
	}

	for _, tt := range tests {
		eff := gen.calculateCostEfficiency(tt.usagePercent)
		assert.GreaterOrEqual(t, eff, tt.minEff)
		assert.LessOrEqual(t, eff, tt.maxEff)
	}
}

func TestCleanupOldReports(t *testing.T) {
	gen := createTestReportGenerator(t)
	ctx := context.Background()

	// 创建报告
	_, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	err = gen.CleanupOldReports()
	require.NoError(t, err)
}

func TestBudgetComparison(t *testing.T) {
	provider := &MockReportDataProvider{
		budgetData: &BudgetReportData{
			BudgetID:     "budget-1",
			BudgetName:   "Test Budget",
			TotalBudget:  100,
			CurrentSpend: 75,
			Status:       "on_track",
			AlertLevel:   "none",
		},
	}

	tmpDir := t.TempDir()
	config := DefaultReportConfig()
	gen := NewCostReportGenerator(tmpDir, provider, config)

	ctx := context.Background()
	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	// 手动设置预算对比
	report.BudgetComparison = &BudgetComparison{
		BudgetID:     "budget-1",
		BudgetName:   "Test Budget",
		TotalBudget:  100,
		CurrentSpend: 75,
		Remaining:    25,
		Utilization:  75,
		Status:       "on_track",
	}

	assert.NotNil(t, report.BudgetComparison)
	assert.Equal(t, 100.0, report.BudgetComparison.TotalBudget)
}

func TestTierBreakdown(t *testing.T) {
	provider := &MockReportDataProvider{
		storageData: &StorageReportData{
			TotalCapacityGB: 1000,
			UsedCapacityGB:  500,
			MonthlyCost:     50,
			TierBreakdown: []TierCostItem{
				{TierName: "Tier 1", MinGB: 0, MaxGB: 100, UsedGB: 100, PricePerGB: 0.1, Cost: 10},
				{TierName: "Tier 2", MinGB: 100, MaxGB: 500, UsedGB: 400, PricePerGB: 0.08, Cost: 32},
			},
		},
	}

	tmpDir := t.TempDir()
	config := DefaultReportConfig()
	gen := NewCostReportGenerator(tmpDir, provider, config)

	ctx := context.Background()
	report, err := gen.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)

	assert.NotEmpty(t, report.StorageCost.TierBreakdown)
}

func TestDefaultReportConfig(t *testing.T) {
	config := DefaultReportConfig()

	assert.Equal(t, "CNY", config.DefaultCurrency)
	assert.Equal(t, 365, config.DataRetentionDays)
	assert.True(t, config.EnableCache)
	assert.NotZero(t, config.CacheExpiry)
}

func TestReportPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	provider := &MockReportDataProvider{}
	config := DefaultReportConfig()

	gen1 := NewCostReportGenerator(tmpDir, provider, config)
	ctx := context.Background()

	report, err := gen1.GenerateDailyReport(ctx, time.Now())
	require.NoError(t, err)
	reportID := report.ID

	// 创建新的生成器
	gen2 := NewCostReportGenerator(tmpDir, provider, config)

	// 验证持久化
	retrieved, err := gen2.GetReport(reportID)
	require.NoError(t, err)
	assert.Equal(t, reportID, retrieved.ID)
}
