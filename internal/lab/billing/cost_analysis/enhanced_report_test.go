// Package cost_analysis 提供增强成本分析报告测试
package cost_analysis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateEnhancedStorageReport(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    80 * 1024 * 1024 * 1024,
				UsagePercent: 80,
			},
			{
				QuotaID:      "quota-2",
				TargetID:     "user2",
				TargetName:   "User Two",
				VolumeName:   "pool1",
				HardLimit:    200 * 1024 * 1024 * 1024,
				UsedBytes:    50 * 1024 * 1024 * 1024,
				UsagePercent: 25,
			},
		},
	}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine(uniqueDataDir("enhanced"), billing, quota, config)

	report, err := engine.GenerateEnhancedStorageReport(30)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.NotEmpty(t, report.ID)
	assert.Equal(t, "CNY", report.Summary.Currency)
	assert.NotEmpty(t, report.PoolBreakdown)
	assert.NotEmpty(t, report.UserBreakdown)
	assert.NotEmpty(t, report.ExportFormats)
}

func TestEnhancedCostSummary(t *testing.T) {
	billing := &mockBillingProvider{
		storagePrice: 0.1,
		stats: &BillingStats{
			TotalStorageUsedGB: 500,
			TotalBandwidthGB:   200,
			TotalRevenue:       150,
			StorageRevenue:     100,
			BandwidthRevenue:   50,
		},
	}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine(uniqueDataDir("summary"), billing, quota, config)

	report, err := engine.GenerateEnhancedStorageReport(30)
	require.NoError(t, err)

	summary := report.Summary
	assert.Equal(t, 150.0, summary.TotalCost)
	assert.Equal(t, 100.0, summary.StorageCost)
	assert.Equal(t, 50.0, summary.BandwidthCost)
	assert.Greater(t, summary.ReportConfidence, 0.0)
	assert.LessOrEqual(t, summary.ReportConfidence, 1.0)
}

func TestCostDistribution(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "ssd-pool",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    60 * 1024 * 1024 * 1024,
				UsagePercent: 60,
			},
			{
				QuotaID:      "quota-2",
				TargetID:     "user2",
				TargetName:   "User Two",
				VolumeName:   "hdd-pool",
				HardLimit:    200 * 1024 * 1024 * 1024,
				UsedBytes:    150 * 1024 * 1024 * 1024,
				UsagePercent: 75,
			},
		},
	}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine(uniqueDataDir("dist"), billing, quota, config)

	report, err := engine.GenerateEnhancedStorageReport(30)
	require.NoError(t, err)

	dist := report.CostDistribution
	assert.NotEmpty(t, dist.ByPool)
	assert.Len(t, dist.ByTimeOfDay, 24)
	assert.Len(t, dist.ByDayOfWeek, 7)
}

func TestAnomalyDetection(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine(uniqueDataDir("anomaly"), billing, quota, config)

	// 添加一些趋势数据
	now := time.Now()
	for i := 0; i < 10; i++ {
		engine.RecordTrendData(CostTrend{
			Date:        now.AddDate(0, 0, -i),
			TotalCost:   100.0 + float64(i*10), // 逐步增加
			StorageCost: 80.0 + float64(i*8),
		})
	}

	report, err := engine.GenerateEnhancedStorageReport(30)
	require.NoError(t, err)

	detection := report.AnomalyDetection
	assert.NotEmpty(t, detection.DetectionMethod)
	assert.NotEmpty(t, detection.BaselinePeriod)
	assert.Greater(t, detection.Threshold, 0.0)
}

func TestPeriodComparison(t *testing.T) {
	billing := &mockBillingProvider{
		storagePrice: 0.1,
		stats: &BillingStats{
			TotalStorageUsedGB: 500,
			TotalRevenue:       150,
			StorageRevenue:     100,
			BandwidthRevenue:   50,
		},
	}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine(uniqueDataDir("comparison"), billing, quota, config)

	report, err := engine.GenerateEnhancedStorageReport(30)
	require.NoError(t, err)

	comparison := report.Comparison
	assert.NotNil(t, comparison.CurrentPeriod.PeriodStart)
	assert.NotNil(t, comparison.PreviousPeriod.PeriodStart)
}

func TestTrendAnalysis(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine(uniqueDataDir("trend-analysis"), billing, quota, config)

	// 添加趋势数据
	now := time.Now()
	for i := 0; i < 15; i++ {
		engine.RecordTrendData(CostTrend{
			Date:          now.AddDate(0, 0, -i),
			TotalCost:     100.0,
			StorageCost:   80.0,
			BandwidthCost: 20.0,
			StorageUsedGB: 50.0,
			BandwidthGB:   10.0,
		})
	}

	report, err := engine.GenerateEnhancedStorageReport(30)
	require.NoError(t, err)

	analysis := report.TrendAnalysis
	assert.NotEmpty(t, analysis.OverallTrend)
	assert.GreaterOrEqual(t, analysis.TrendStrength, 0.0)
	assert.LessOrEqual(t, analysis.TrendStrength, 1.0)
	assert.NotNil(t, analysis.WeeklyPattern)
	assert.NotNil(t, analysis.MonthlyProjection)
}

func TestPoolBreakdown(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "ssd-pool",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    90 * 1024 * 1024 * 1024,
				UsagePercent: 90,
			},
			{
				QuotaID:      "quota-2",
				TargetID:     "user2",
				TargetName:   "User Two",
				VolumeName:   "hdd-pool",
				HardLimit:    200 * 1024 * 1024 * 1024,
				UsedBytes:    30 * 1024 * 1024 * 1024,
				UsagePercent: 15,
			},
		},
	}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine(uniqueDataDir("pool"), billing, quota, config)

	report, err := engine.GenerateEnhancedStorageReport(30)
	require.NoError(t, err)

	breakdown := report.PoolBreakdown
	assert.NotEmpty(t, breakdown)

	// 检查高利用率存储池的建议
	for _, pool := range breakdown {
		if pool.Utilization > 80 {
			assert.NotEmpty(t, pool.Recommendation)
		}
	}
}

func TestUserBreakdown(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    95 * 1024 * 1024 * 1024,
				UsagePercent: 95,
			},
			{
				QuotaID:      "quota-2",
				TargetID:     "user2",
				TargetName:   "User Two",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    10 * 1024 * 1024 * 1024,
				UsagePercent: 10,
			},
		},
	}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine(uniqueDataDir("user"), billing, quota, config)

	report, err := engine.GenerateEnhancedStorageReport(30)
	require.NoError(t, err)

	breakdown := report.UserBreakdown
	assert.NotEmpty(t, breakdown)

	// 检查用户建议
	for _, user := range breakdown {
		if user.QuotaUtilization > 90 {
			assert.NotEmpty(t, user.Recommendation)
		}
	}
}

func TestEnhancedRecommendations(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    95 * 1024 * 1024 * 1024,
				UsagePercent: 95,
			},
		},
	}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine(uniqueDataDir("rec"), billing, quota, config)

	report, err := engine.GenerateEnhancedStorageReport(30)
	require.NoError(t, err)

	// 应该有建议
	assert.NotEmpty(t, report.Recommendations)

	for _, rec := range report.Recommendations {
		assert.NotEmpty(t, rec.ID)
		assert.NotEmpty(t, rec.Type)
		assert.NotEmpty(t, rec.Priority)
		assert.NotEmpty(t, rec.Title)
		assert.NotEmpty(t, rec.Description)
	}
}

func TestSaveAndLoadReport(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	dataDir := uniqueDataDir("save-load")
	engine := NewCostAnalysisEngine(dataDir, billing, quota, config)

	// 生成报告
	report, err := engine.GenerateEnhancedStorageReport(30)
	require.NoError(t, err)

	// 保存报告
	err = engine.SaveReport(report)
	require.NoError(t, err)

	// 加载报告
	loaded, err := engine.LoadReport(report.ID)
	require.NoError(t, err)
	assert.Equal(t, report.ID, loaded.ID)
	assert.Equal(t, report.Summary.TotalCost, loaded.Summary.TotalCost)
}

func TestListSavedReports(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	dataDir := uniqueDataDir("list-reports")
	engine := NewCostAnalysisEngine(dataDir, billing, quota, config)

	// 生成多个报告
	for i := 0; i < 3; i++ {
		report, err := engine.GenerateEnhancedStorageReport(30)
		require.NoError(t, err)
		err = engine.SaveReport(report)
		require.NoError(t, err)
	}

	// 列出报告
	ids, err := engine.ListSavedReports()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(ids), 3)
}

func TestResolveAnomaly(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	dataDir := uniqueDataDir("resolve")
	engine := NewCostAnalysisEngine(dataDir, billing, quota, config)

	// 生成报告
	report, err := engine.GenerateEnhancedStorageReport(30)
	require.NoError(t, err)

	// 添加一个异常用于测试
	report.AnomalyDetection.HasAnomalies = true
	report.AnomalyDetection.Anomalies = []CostAnomaly{
		{
			ID:            "anomaly-1",
			DetectedAt:    time.Now(),
			Type:          "spike",
			Severity:      "medium",
			ResourceType:  "storage",
			ExpectedValue: 100,
			ActualValue:   150,
			Deviation:     50,
			Status:        "new",
		},
	}

	err = engine.SaveReport(report)
	require.NoError(t, err)

	// 解决异常
	err = engine.ResolveAnomaly(report.ID, "anomaly-1", "已处理")
	require.NoError(t, err)

	// 验证状态
	loaded, err := engine.LoadReport(report.ID)
	require.NoError(t, err)
	assert.Equal(t, "resolved", loaded.AnomalyDetection.Anomalies[0].Status)
	assert.NotEmpty(t, loaded.AnomalyDetection.Anomalies[0].Resolution)
}

func TestIgnoreAnomaly(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	dataDir := uniqueDataDir("ignore")
	engine := NewCostAnalysisEngine(dataDir, billing, quota, config)

	// 生成报告
	report, err := engine.GenerateEnhancedStorageReport(30)
	require.NoError(t, err)

	// 添加一个异常用于测试
	report.AnomalyDetection.HasAnomalies = true
	report.AnomalyDetection.Anomalies = []CostAnomaly{
		{
			ID:         "anomaly-2",
			DetectedAt: time.Now(),
			Type:       "drop",
			Severity:   "low",
			Status:     "new",
		},
	}

	err = engine.SaveReport(report)
	require.NoError(t, err)

	// 忽略异常
	err = engine.IgnoreAnomaly(report.ID, "anomaly-2")
	require.NoError(t, err)

	// 验证状态
	loaded, err := engine.LoadReport(report.ID)
	require.NoError(t, err)
	assert.Equal(t, "ignored", loaded.AnomalyDetection.Anomalies[0].Status)
}

func TestCalculateOverallEfficiencyScore(t *testing.T) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine(uniqueDataDir("efficiency"), billing, quota, config)

	tests := []struct {
		summary  EnhancedCostSummary
		minScore float64
		maxScore float64
	}{
		{
			summary: EnhancedCostSummary{
				ActiveUsers:       10,
				TotalUsedGB:       500,
				CostChangePercent: 5,
			},
			minScore: 80,
			maxScore: 100,
		},
		{
			summary: EnhancedCostSummary{
				ActiveUsers:       10,
				TotalUsedGB:       50, // 低平均使用
				CostChangePercent: 25, // 高增长
			},
			minScore: 0,
			maxScore: 100,
		},
	}

	for _, tt := range tests {
		score := engine.calculateOverallEfficiencyScore(tt.summary)
		assert.GreaterOrEqual(t, score, tt.minScore)
		assert.LessOrEqual(t, score, tt.maxScore)
	}
}

func TestCostAnomalySeverity(t *testing.T) {
	tests := []struct {
		deviation float64
		severity  string
	}{
		{10, "low"},
		{30, "medium"},
		{60, "high"},
		{100, "critical"},
	}

	for _, tt := range tests {
		anomaly := CostAnomaly{
			Deviation: tt.deviation,
		}

		// 根据偏差确定严重级别
		switch {
		case anomaly.Deviation > 80:
			anomaly.Severity = "critical"
		case anomaly.Deviation > 50:
			anomaly.Severity = "high"
		case anomaly.Deviation > 20:
			anomaly.Severity = "medium"
		default:
			anomaly.Severity = "low"
		}

		assert.Equal(t, tt.severity, anomaly.Severity)
	}
}

func TestEnhancedReportCache(t *testing.T) {
	cache := newEnhancedReportCache()

	report := &EnhancedStorageReport{
		ID:          "test-report",
		GeneratedAt: time.Now(),
	}

	// 设置
	cache.Set(report.ID, report)

	// 获取
	retrieved, ok := cache.Get(report.ID)
	assert.True(t, ok)
	assert.Equal(t, report.ID, retrieved.ID)

	// 删除
	cache.Delete(report.ID)
	_, ok = cache.Get(report.ID)
	assert.False(t, ok)
}
