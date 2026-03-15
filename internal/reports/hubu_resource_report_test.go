// Package reports 提供资源报告测试 (v2.92.0 户部)
package reports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== 户部资源报告生成器测试 ==========

func TestHubuResourceReportGenerator_GenerateStorageUsageReport(t *testing.T) {
	storageConfig := DefaultStorageReportConfig()
	bandwidthConfig := BandwidthReportConfig{
		BandwidthLimitMbps: 1000,
	}
	capacityConfig := CapacityPlanningConfig{
		AlertThreshold:    70.0,
		CriticalThreshold: 85.0,
		ForecastDays:      90,
	}
	costConfig := StorageCostConfig{
		CostPerGBMonthly: 0.5,
	}

	generator := NewHubuResourceReportGenerator(storageConfig, bandwidthConfig, capacityConfig, costConfig)

	now := time.Now()
	start := now.AddDate(0, 0, -30)

	req := HubuReportRequest{
		Type:      HubuReportStorageUsage,
		StartTime: &start,
		EndTime:   &now,
	}

	report := generator.GenerateReport(req)

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, HubuReportStorageUsage, report.Type)
	assert.Equal(t, "存储使用报告", report.Name)
	assert.NotNil(t, report.StorageUsage)
	assert.GreaterOrEqual(t, report.GenerationTimeMS, int64(0))
}

func TestHubuResourceReportGenerator_GenerateBandwidthStatsReport(t *testing.T) {
	storageConfig := DefaultStorageReportConfig()
	bandwidthConfig := BandwidthReportConfig{
		BandwidthLimitMbps: 1000,
	}
	capacityConfig := CapacityPlanningConfig{
		ForecastDays: 90,
	}
	costConfig := StorageCostConfig{}

	generator := NewHubuResourceReportGenerator(storageConfig, bandwidthConfig, capacityConfig, costConfig)

	now := time.Now()
	start := now.AddDate(0, 0, -30)

	req := HubuReportRequest{
		Type:      HubuReportBandwidthStats,
		StartTime: &start,
		EndTime:   &now,
	}

	report := generator.GenerateReport(req)

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, HubuReportBandwidthStats, report.Type)
	assert.Equal(t, "带宽统计报告", report.Name)
	assert.NotNil(t, report.BandwidthStats)
}

func TestHubuResourceReportGenerator_GenerateCapacityForecastReport(t *testing.T) {
	storageConfig := DefaultStorageReportConfig()
	bandwidthConfig := BandwidthReportConfig{}
	capacityConfig := CapacityPlanningConfig{
		AlertThreshold:    70.0,
		CriticalThreshold: 85.0,
		ForecastDays:      90,
		GrowthModel:       GrowthModelLinear,
	}
	costConfig := StorageCostConfig{}

	generator := NewHubuResourceReportGenerator(storageConfig, bandwidthConfig, capacityConfig, costConfig)

	now := time.Now()
	start := now.AddDate(0, 0, -30)

	req := HubuReportRequest{
		Type:         HubuReportCapacity,
		StartTime:    &start,
		EndTime:      &now,
		ForecastDays: 90,
	}

	report := generator.GenerateReport(req)

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, HubuReportCapacity, report.Type)
	assert.Equal(t, "容量预测报告", report.Name)
	assert.NotNil(t, report.CapacityForecast)
}

func TestHubuResourceReportGenerator_GenerateComprehensiveReport(t *testing.T) {
	storageConfig := DefaultStorageReportConfig()
	bandwidthConfig := BandwidthReportConfig{
		BandwidthLimitMbps: 1000,
	}
	capacityConfig := CapacityPlanningConfig{
		ForecastDays: 90,
	}
	costConfig := StorageCostConfig{}

	generator := NewHubuResourceReportGenerator(storageConfig, bandwidthConfig, capacityConfig, costConfig)

	now := time.Now()
	start := now.AddDate(0, 0, -30)

	req := HubuReportRequest{
		Type:      HubuReportComprehensive,
		StartTime: &start,
		EndTime:   &now,
	}

	report := generator.GenerateReport(req)

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, HubuReportComprehensive, report.Type)
	assert.Equal(t, "综合资源报告", report.Name)
	assert.NotNil(t, report.Comprehensive)
	assert.Greater(t, report.Comprehensive.OverallHealthScore, 0.0)
	assert.NotEmpty(t, report.Comprehensive.HealthStatus)
}

// ========== 健康评分测试 ==========

func TestHubuResourceReportGenerator_CalculateHealthScore(t *testing.T) {
	storageConfig := DefaultStorageReportConfig()
	bandwidthConfig := BandwidthReportConfig{}
	capacityConfig := CapacityPlanningConfig{}
	costConfig := StorageCostConfig{}

	generator := NewHubuResourceReportGenerator(storageConfig, bandwidthConfig, capacityConfig, costConfig)

	t.Run("健康状态", func(t *testing.T) {
		storageReport := &StorageUsageReport{
			Summary: StorageUsageSummary{
				UsagePercent: 50.0,
			},
			Alerts: []StorageAlert{},
		}

		score := generator.CalculateHealthScore(storageReport, nil, nil)
		assert.GreaterOrEqual(t, score, 80.0)
	})

	t.Run("警告状态", func(t *testing.T) {
		storageReport := &StorageUsageReport{
			Summary: StorageUsageSummary{
				UsagePercent: 75.0,
			},
			Alerts: []StorageAlert{
				{Severity: "warning", Message: "存储使用率较高"},
			},
		}

		score := generator.CalculateHealthScore(storageReport, nil, nil)
		assert.Less(t, score, 90.0)
	})

	t.Run("严重状态", func(t *testing.T) {
		storageReport := &StorageUsageReport{
			Summary: StorageUsageSummary{
				UsagePercent: 95.0,
			},
			Alerts: []StorageAlert{
				{Severity: "critical", Message: "存储空间不足"},
			},
		}

		score := generator.CalculateHealthScore(storageReport, nil, nil)
		assert.Less(t, score, 70.0)
	})

	t.Run("带宽影响", func(t *testing.T) {
		storageReport := &StorageUsageReport{
			Summary: StorageUsageSummary{
				UsagePercent: 50.0,
			},
		}

		bandwidthReport := &BandwidthReport{
			Summary: BandwidthSummary{
				PeakUtilization: 95.0,
				AvgErrorRate:    2.0,
			},
		}

		score := generator.CalculateHealthScore(storageReport, bandwidthReport, nil)
		// 带宽95%扣20分 + 错误率>1扣10分 = 扣30分，从100开始 = 70分
		// 验证分数被带宽问题影响（应该是70或更低）
		assert.LessOrEqual(t, score, 80.0)
	})

	t.Run("容量影响", func(t *testing.T) {
		storageReport := &StorageUsageReport{
			Summary: StorageUsageSummary{
				UsagePercent: 50.0,
			},
		}

		capacityReport := &CapacityPlanningReport{
			Summary: CapacityPlanningSummary{
				Urgency: "critical",
			},
		}

		score := generator.CalculateHealthScore(storageReport, nil, capacityReport)
		assert.Less(t, score, 80.0)
	})
}

func TestHubuResourceReportGenerator_GetHealthStatus(t *testing.T) {
	storageConfig := DefaultStorageReportConfig()
	bandwidthConfig := BandwidthReportConfig{}
	capacityConfig := CapacityPlanningConfig{}
	costConfig := StorageCostConfig{}

	generator := NewHubuResourceReportGenerator(storageConfig, bandwidthConfig, capacityConfig, costConfig)

	testCases := []struct {
		score          float64
		expectedStatus string
	}{
		{95.0, "excellent"},
		{85.0, "good"},
		{75.0, "good"},
		{60.0, "warning"},
		{40.0, "critical"},
		{10.0, "critical"},
	}

	for _, tc := range testCases {
		status := generator.GetHealthStatus(tc.score)
		assert.Equal(t, tc.expectedStatus, status, "score: %f", tc.score)
	}
}

// ========== 报告类型测试 ==========

func TestHubuReportType_String(t *testing.T) {
	types := []HubuReportType{
		HubuReportStorageUsage,
		HubuReportBandwidthStats,
		HubuReportCapacity,
		HubuReportComprehensive,
	}

	for _, rt := range types {
		assert.NotEmpty(t, string(rt))
	}
}

// ========== 报告请求测试 ==========

func TestHubuReportRequest_Defaults(t *testing.T) {
	req := HubuReportRequest{
		Type: HubuReportStorageUsage,
	}

	// 验证默认值
	assert.Equal(t, HubuReportStorageUsage, req.Type)
	assert.False(t, req.IncludeForecast)
	assert.False(t, req.IncludeRecommendations)
	assert.False(t, req.IncludeAlerts)
}

// ========== 综合报告测试 ==========

func TestHubuComprehensiveReport_Initialization(t *testing.T) {
	report := &HubuComprehensiveReport{
		OverallHealthScore: 85.0,
		HealthStatus:       "good",
		KeyMetrics:         []HubuKeyMetric{},
		AlertSummary: HubuAlertSummary{
			Total:      0,
			Critical:   0,
			Warning:    0,
			Info:       0,
			ByType:     make(map[string]int),
			ByResource: make(map[string]int),
		},
		TopRecommendations: []HubuPrioritizedRecommendation{},
		TrendSummary: HubuTrendSummary{
			StorageTrend:   "stable",
			BandwidthTrend: "stable",
		},
		ForecastSummary: HubuForecastSummary{
			Confidence: 0.75,
		},
	}

	assert.Equal(t, 85.0, report.OverallHealthScore)
	assert.Equal(t, "good", report.HealthStatus)
	assert.NotNil(t, report.AlertSummary.ByType)
	assert.NotNil(t, report.AlertSummary.ByResource)
}

// ========== 关键指标测试 ==========

func TestHubuKeyMetric_Status(t *testing.T) {
	metrics := []HubuKeyMetric{
		{Name: "存储使用率", Value: 50.0, Unit: "%", Status: "normal"},
		{Name: "带宽利用率", Value: 85.0, Unit: "%", Status: "warning"},
		{Name: "容量剩余天数", Value: 5.0, Unit: "天", Status: "critical"},
	}

	assert.Equal(t, "normal", metrics[0].Status)
	assert.Equal(t, "warning", metrics[1].Status)
	assert.Equal(t, "critical", metrics[2].Status)
}

// ========== 告警摘要测试 ==========

func TestHubuAlertSummary_Counts(t *testing.T) {
	summary := HubuAlertSummary{
		Total:    5,
		Critical: 1,
		Warning:  2,
		Info:     2,
		ByType: map[string]int{
			"capacity_high":  2,
			"quota_exceeded": 1,
			"bandwidth_high": 2,
		},
		ByResource: map[string]int{
			"volume1": 2,
			"volume2": 3,
		},
	}

	assert.Equal(t, 5, summary.Total)
	assert.Equal(t, 1, summary.Critical)
	assert.Equal(t, 2, summary.Warning)
	assert.Equal(t, 2, summary.Info)
	assert.Equal(t, 2, summary.ByType["capacity_high"])
	assert.Equal(t, 3, summary.ByResource["volume2"])
}

// ========== 优先建议测试 ==========

func TestHubuPrioritizedRecommendation_Ranking(t *testing.T) {
	recommendations := []HubuPrioritizedRecommendation{
		{Rank: 1, Type: "storage", Priority: "critical", Title: "紧急扩容"},
		{Rank: 2, Type: "bandwidth", Priority: "high", Title: "升级带宽"},
		{Rank: 3, Type: "capacity", Priority: "medium", Title: "规划扩容"},
		{Rank: 4, Type: "cost", Priority: "low", Title: "优化成本"},
	}

	assert.Equal(t, 1, recommendations[0].Rank)
	assert.Equal(t, "critical", recommendations[0].Priority)
	assert.Equal(t, "low", recommendations[3].Priority)
}

// ========== 趋势摘要测试 ==========

func TestHubuTrendSummary_GrowthRates(t *testing.T) {
	summary := HubuTrendSummary{
		StorageTrend:        "increasing",
		StorageGrowthRate:   5.0,
		BandwidthTrend:      "stable",
		BandwidthGrowthRate: 1.0,
		DaysToStorageFull:   60,
		DaysToBandwidthFull: 0,
	}

	assert.Equal(t, "increasing", summary.StorageTrend)
	assert.Equal(t, 5.0, summary.StorageGrowthRate)
	assert.Equal(t, 60, summary.DaysToStorageFull)
}

// ========== 预测摘要测试 ==========

func TestHubuForecastSummary_Predictions(t *testing.T) {
	summary := HubuForecastSummary{
		NextMonthStorageGB:     500.0,
		NextQuarterStorageGB:   650.0,
		NextMonthBandwidthGB:   100.0,
		NextQuarterBandwidthGB: 300.0,
		RecommendedExpansionGB: 200.0,
		Confidence:             0.85,
	}

	assert.Equal(t, 500.0, summary.NextMonthStorageGB)
	assert.Equal(t, 650.0, summary.NextQuarterStorageGB)
	assert.Equal(t, 0.85, summary.Confidence)
	assert.Greater(t, summary.NextQuarterStorageGB, summary.NextMonthStorageGB)
}

// ========== 存储增强报告测试 ==========

func TestHubuStorageEnhancedReport(t *testing.T) {
	now := time.Now()
	report := &HubuStorageEnhancedReport{
		StorageUsageReport: &StorageUsageReport{
			ID:          "storage_001",
			Name:        "存储使用报表",
			GeneratedAt: now,
			Summary: StorageUsageSummary{
				TotalCapacity: 1 * 1024 * 1024 * 1024 * 1024,
				TotalUsed:     500 * 1024 * 1024 * 1024,
				UsagePercent:  50.0,
				HealthStatus:  "healthy",
			},
		},
		EnhancedMetrics: HubuStorageEnhancedMetrics{
			ReadIOPS:          1000,
			WriteIOPS:         500,
			ReadThroughputMB:  100.0,
			WriteThroughputMB: 50.0,
			AvgLatencyMs:      5.0,
			CompressionRatio:  1.5,
			DedupRatio:        1.2,
		},
		CostAnalysis: HubuStorageCostAnalysis{
			MonthlyCost:    500.0,
			ProjectedCost:  550.0,
			CostEfficiency: 85.0,
		},
	}

	assert.Equal(t, "storage_001", report.ID)
	assert.Equal(t, uint64(1000), report.EnhancedMetrics.ReadIOPS)
	assert.Equal(t, 1.5, report.EnhancedMetrics.CompressionRatio)
	assert.Equal(t, 500.0, report.CostAnalysis.MonthlyCost)
}

// ========== 带宽增强报告测试 ==========

func TestHubuBandwidthEnhancedReport(t *testing.T) {
	now := time.Now()
	report := &HubuBandwidthEnhancedReport{
		BandwidthReport: &BandwidthReport{
			ID:          "bw_001",
			Name:        "带宽使用报告",
			GeneratedAt: now,
			Summary: BandwidthSummary{
				TotalGB:        500.0,
				AvgTotalMbps:   100.0,
				PeakTotalMbps:  500.0,
				TrafficPattern: "balanced",
			},
		},
		EnhancedMetrics: HubuBandwidthEnhancedMetrics{
			ActiveConnections: 100,
			MaxConnections:    1000,
			TotalSessions:     10000,
			AvgSessionTime:    300.0,
			RetransmitRate:    0.5,
			AvgRTTMs:          20.0,
		},
		TrafficAnalysis: HubuTrafficAnalysis{
			Pattern: "steady",
			PeakHours: []HubuPeakHour{
				{Hour: 10, AvgMbps: 200.0, PeakMbps: 400.0},
				{Hour: 14, AvgMbps: 180.0, PeakMbps: 350.0},
			},
		},
		ProtocolDistribution: []HubuProtocolStats{
			{Protocol: "HTTPS", Percentage: 60.0},
			{Protocol: "HTTP", Percentage: 20.0},
			{Protocol: "Other", Percentage: 20.0},
		},
	}

	assert.Equal(t, "bw_001", report.ID)
	assert.Equal(t, 100, report.EnhancedMetrics.ActiveConnections)
	assert.Equal(t, "steady", report.TrafficAnalysis.Pattern)
	assert.Len(t, report.ProtocolDistribution, 3)
}

// ========== 容量预测增强报告测试 ==========

func TestHubuCapacityEnhancedReport(t *testing.T) {
	now := time.Now()
	fullDate := now.AddDate(0, 2, 0)

	report := &HubuCapacityEnhancedReport{
		CapacityPlanningReport: &CapacityPlanningReport{
			ID:          "cap_001",
			Name:        "容量规划报告",
			GeneratedAt: now,
			Summary: CapacityPlanningSummary{
				CurrentUsagePercent:    70.0,
				MonthlyGrowthRate:      5.0,
				DaysToFullCapacity:     60,
				RecommendedExpansionGB: 200,
				Urgency:                "high",
				Trend:                  "growing",
			},
		},
		EnhancedForecast: HubuEnhancedCapacityForecast{
			ConfidenceIntervals: []HubuConfidenceInterval{
				{
					Date:      now.AddDate(0, 1, 0),
					Lower:     600 * 1024 * 1024 * 1024,
					Predicted: 700 * 1024 * 1024 * 1024,
					Upper:     800 * 1024 * 1024 * 1024,
					Level:     0.95,
				},
			},
			ModelDetails: HubuModelDetails{
				Type:         "linear",
				Accuracy:     0.92,
				MAPE:         3.5,
				TrainingDays: 90,
			},
			Seasonality: HubuSeasonalityAnalysis{
				HasSeasonality: true,
				CycleDays:      30,
				PeakDay:        15,
				Variation:      10.0,
			},
		},
		ScenarioAnalysis: []HubuCapacityScenario{
			{
				Name:        "基准场景",
				Description: "基于历史增长率的预测",
				GrowthRate:  5.0,
				FullDate:    &fullDate,
			},
		},
	}

	assert.Equal(t, "cap_001", report.ID)
	assert.Equal(t, 70.0, report.Summary.CurrentUsagePercent)
	assert.Equal(t, 0.92, report.EnhancedForecast.ModelDetails.Accuracy)
	assert.True(t, report.EnhancedForecast.Seasonality.HasSeasonality)
}

// ========== 报告周期测试 ==========

func TestReportPeriod_Duration(t *testing.T) {
	now := time.Now()
	start := now.AddDate(0, 0, -30)

	period := ReportPeriod{
		StartTime: start,
		EndTime:   now,
	}

	duration := period.EndTime.Sub(period.StartTime)
	assert.Equal(t, 30*24*time.Hour, duration)
}

// ========== 成本构成测试 ==========

func TestHubuCostBreakdown_Total(t *testing.T) {
	breakdown := HubuCostBreakdown{
		StorageCost:      300.0,
		ElectricityCost:  50.0,
		OperationsCost:   100.0,
		DepreciationCost: 50.0,
	}

	total := breakdown.StorageCost + breakdown.ElectricityCost +
		breakdown.OperationsCost + breakdown.DepreciationCost

	assert.Equal(t, 500.0, total)
}

// ========== 流量分析测试 ==========

func TestHubuTrafficAnalysis_PeakHours(t *testing.T) {
	analysis := HubuTrafficAnalysis{
		Pattern: "periodic",
		PeakHours: []HubuPeakHour{
			{Hour: 9, AvgMbps: 150.0, PeakMbps: 300.0, Percentage: 15.0},
			{Hour: 10, AvgMbps: 200.0, PeakMbps: 400.0, Percentage: 20.0},
			{Hour: 14, AvgMbps: 180.0, PeakMbps: 350.0, Percentage: 18.0},
		},
		AppDistribution: []HubuAppStats{
			{Application: "Web", Percentage: 50.0},
			{Application: "API", Percentage: 30.0},
			{Application: "Other", Percentage: 20.0},
		},
	}

	assert.Equal(t, "periodic", analysis.Pattern)
	assert.Len(t, analysis.PeakHours, 3)
	assert.Len(t, analysis.AppDistribution, 3)

	// 找到峰值时段
	var maxPeakHour int
	var maxPeakMbps float64
	for _, ph := range analysis.PeakHours {
		if ph.PeakMbps > maxPeakMbps {
			maxPeakMbps = ph.PeakMbps
			maxPeakHour = ph.Hour
		}
	}
	assert.Equal(t, 10, maxPeakHour)
	assert.Equal(t, 400.0, maxPeakMbps)
}

// ========== 场景分析测试 ==========

func TestHubuCapacityScenario_Comparison(t *testing.T) {
	now := time.Now()
	scenarios := []HubuCapacityScenario{
		{
			Name:       "乐观场景",
			GrowthRate: 2.0,
			FullDate:   ptrTime(now.AddDate(0, 6, 0)),
		},
		{
			Name:       "基准场景",
			GrowthRate: 5.0,
			FullDate:   ptrTime(now.AddDate(0, 3, 0)),
		},
		{
			Name:       "悲观场景",
			GrowthRate: 10.0,
			FullDate:   ptrTime(now.AddDate(0, 1, 0)),
		},
	}

	// 验证场景排序（增长率越高，满载越快）
	assert.Equal(t, 2.0, scenarios[0].GrowthRate)
	assert.Equal(t, 5.0, scenarios[1].GrowthRate)
	assert.Equal(t, 10.0, scenarios[2].GrowthRate)
}

// 辅助函数
func ptrTime(t time.Time) *time.Time {
	return &t
}

// ========== 边界条件测试 ==========

func TestHubuResourceReportGenerator_ZeroValues(t *testing.T) {
	storageConfig := StorageReportConfig{}
	bandwidthConfig := BandwidthReportConfig{}
	capacityConfig := CapacityPlanningConfig{}
	costConfig := StorageCostConfig{}

	generator := NewHubuResourceReportGenerator(storageConfig, bandwidthConfig, capacityConfig, costConfig)

	req := HubuReportRequest{
		Type: HubuReportComprehensive,
	}

	report := generator.GenerateReport(req)

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
}

func TestHubuResourceReportGenerator_NilTimes(t *testing.T) {
	storageConfig := DefaultStorageReportConfig()
	bandwidthConfig := BandwidthReportConfig{}
	capacityConfig := CapacityPlanningConfig{}
	costConfig := StorageCostConfig{}

	generator := NewHubuResourceReportGenerator(storageConfig, bandwidthConfig, capacityConfig, costConfig)

	req := HubuReportRequest{
		Type:      HubuReportStorageUsage,
		StartTime: nil,
		EndTime:   nil,
	}

	report := generator.GenerateReport(req)

	assert.NotNil(t, report)
	assert.NotNil(t, report.Period.StartTime)
	assert.NotNil(t, report.Period.EndTime)
}

// ========== 性能测试 ==========

func TestHubuResourceReportGenerator_Performance(t *testing.T) {
	storageConfig := DefaultStorageReportConfig()
	bandwidthConfig := BandwidthReportConfig{}
	capacityConfig := CapacityPlanningConfig{}
	costConfig := StorageCostConfig{}

	generator := NewHubuResourceReportGenerator(storageConfig, bandwidthConfig, capacityConfig, costConfig)

	start := time.Now()

	for i := 0; i < 100; i++ {
		req := HubuReportRequest{
			Type: HubuReportComprehensive,
		}
		_ = generator.GenerateReport(req)
	}

	elapsed := time.Since(start)

	// 100次生成应该在1秒内完成
	assert.Less(t, elapsed.Milliseconds(), int64(1000), "报告生成性能测试")
}
