// Package reports 提供报表生成和管理功能
package reports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== 容量预测分析测试 ==========

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
	// 创建容量历史数据
	history := []CapacityHistory{
		{
			Timestamp:      now.AddDate(0, 0, -30),
			TotalBytes:     1 * 1024 * 1024 * 1024 * 1024, // 1TB
			UsedBytes:      400 * 1024 * 1024 * 1024,      // 400GB
			AvailableBytes: 624 * 1024 * 1024 * 1024,
			UsagePercent:   40.0,
		},
		{
			Timestamp:      now.AddDate(0, 0, -20),
			TotalBytes:     1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:      500 * 1024 * 1024 * 1024, // 500GB
			AvailableBytes: 524 * 1024 * 1024 * 1024,
			UsagePercent:   50.0,
		},
		{
			Timestamp:      now.AddDate(0, 0, -10),
			TotalBytes:     1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:      600 * 1024 * 1024 * 1024, // 600GB
			AvailableBytes: 424 * 1024 * 1024 * 1024,
			UsagePercent:   60.0,
		},
		{
			Timestamp:      now,
			TotalBytes:     1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:      700 * 1024 * 1024 * 1024, // 700GB
			AvailableBytes: 324 * 1024 * 1024 * 1024,
			UsagePercent:   70.0,
		},
	}

	report := planner.Analyze(history, "volume1")

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, "容量规划报告", report.Name)
	assert.Equal(t, "volume1", report.VolumeName)
	assert.NotNil(t, report.Current)
	assert.NotEmpty(t, report.Forecasts)
	assert.NotEmpty(t, report.Recommendations)

	// 验证当前状态
	assert.Equal(t, uint64(1*1024*1024*1024*1024), report.Current.TotalBytes)
	assert.Equal(t, uint64(700*1024*1024*1024), report.Current.UsedBytes)
	assert.Equal(t, "warning", report.Current.Status) // 70% 处于预警状态

	// 验证预测数据
	assert.Len(t, report.Forecasts, 30) // 30天预测
	assert.Greater(t, report.Summary.MonthlyGrowthRate, 0.0)
}

func TestCapacityPlanner_LinearForecast(t *testing.T) {
	config := CapacityPlanningConfig{
		GrowthModel:    GrowthModelLinear,
		ForecastDays:   10,
		AlertThreshold: 70.0,
	}

	planner := NewCapacityPlanner(config)

	now := time.Now()
	// 线性增长：每天增加 10GB
	history := []CapacityHistory{
		{
			Timestamp:    now.AddDate(0, 0, -10),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024, // 1TB
			UsedBytes:    100 * 1024 * 1024 * 1024,      // 100GB
			UsagePercent: 10.0,
		},
		{
			Timestamp:    now.AddDate(0, 0, -5),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    150 * 1024 * 1024 * 1024, // 150GB
			UsagePercent: 15.0,
		},
		{
			Timestamp:    now,
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    200 * 1024 * 1024 * 1024, // 200GB
			UsagePercent: 20.0,
		},
	}

	report := planner.Analyze(history, "volume1")

	assert.NotNil(t, report)
	assert.Len(t, report.Forecasts, 10)

	// 验证线性预测：每天增加约 10GB
	for i, forecast := range report.Forecasts {
		expectedUsed := uint64(200+10*(i+1)) * 1024 * 1024 * 1024
		// 允许一定误差
		assert.InDelta(t, float64(expectedUsed), float64(forecast.ForecastUsedBytes), float64(50*1024*1024*1024))
	}
}

func TestCapacityPlanner_MilestoneCalculation(t *testing.T) {
	config := CapacityPlanningConfig{
		AlertThreshold:    70.0,
		CriticalThreshold: 85.0,
		ForecastDays:      90,
		GrowthModel:       GrowthModelLinear,
	}

	planner := NewCapacityPlanner(config)

	now := time.Now()
	// 创建增长数据，会在预测期内达到各里程碑
	history := []CapacityHistory{
		{
			Timestamp:    now.AddDate(0, 0, -30),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    400 * 1024 * 1024 * 1024,
			UsagePercent: 40.0,
		},
		{
			Timestamp:    now.AddDate(0, 0, -20),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    500 * 1024 * 1024 * 1024,
			UsagePercent: 50.0,
		},
		{
			Timestamp:    now,
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    650 * 1024 * 1024 * 1024, // 65%
			UsagePercent: 65.0,
		},
	}

	report := planner.Analyze(history, "volume1")

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.Milestones)

	// 应该包含 70%, 80%, 90%, 95% 里程碑
	thresholds := make(map[float64]bool)
	for _, m := range report.Milestones {
		thresholds[m.Threshold] = true
	}

	assert.True(t, thresholds[70.0], "应包含 70% 里程碑")
	assert.True(t, thresholds[80.0], "应包含 80% 里程碑")
	assert.True(t, thresholds[90.0], "应包含 90% 里程碑")
}

func TestCapacityPlanner_Recommendations(t *testing.T) {
	config := CapacityPlanningConfig{
		AlertThreshold:    70.0,
		CriticalThreshold: 85.0,
		ForecastDays:      30,
		GrowthModel:       GrowthModelLinear,
	}

	planner := NewCapacityPlanner(config)

	now := time.Now()

	// 测试严重状态
	t.Run("CriticalStatus", func(t *testing.T) {
		history := []CapacityHistory{
			{
				Timestamp:    now.AddDate(0, 0, -10),
				TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
				UsedBytes:    800 * 1024 * 1024 * 1024,
				UsagePercent: 80.0,
			},
			{
				Timestamp:    now,
				TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
				UsedBytes:    900 * 1024 * 1024 * 1024, // 90% - 严重状态
				UsagePercent: 90.0,
			},
		}

		report := planner.Analyze(history, "volume1")

		assert.NotNil(t, report)
		assert.Equal(t, "critical", report.Current.Status)

		// 应该有紧急扩容建议
		hasUrgentExpansion := false
		for _, rec := range report.Recommendations {
			if rec.Type == "expansion" && rec.Priority == "critical" {
				hasUrgentExpansion = true
			}
		}
		assert.True(t, hasUrgentExpansion, "应有紧急扩容建议")
	})

	// 测试警告状态
	t.Run("WarningStatus", func(t *testing.T) {
		history := []CapacityHistory{
			{
				Timestamp:    now.AddDate(0, 0, -10),
				TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
				UsedBytes:    600 * 1024 * 1024 * 1024,
				UsagePercent: 60.0,
			},
			{
				Timestamp:    now,
				TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
				UsedBytes:    750 * 1024 * 1024 * 1024, // 75% - 警告状态
				UsagePercent: 75.0,
			},
		}

		report := planner.Analyze(history, "volume1")

		assert.NotNil(t, report)
		assert.Equal(t, "warning", report.Current.Status)

		// 应该有计划扩容建议
		hasPlannedExpansion := false
		for _, rec := range report.Recommendations {
			if rec.Type == "expansion" && rec.Priority == "high" {
				hasPlannedExpansion = true
			}
		}
		assert.True(t, hasPlannedExpansion, "应有计划扩容建议")
	})
}

func TestCapacityPlanner_Summary(t *testing.T) {
	config := CapacityPlanningConfig{
		AlertThreshold:    70.0,
		CriticalThreshold: 85.0,
		ForecastDays:      60,
		GrowthModel:       GrowthModelLinear,
		ExpansionLeadTime: 30,
	}

	planner := NewCapacityPlanner(config)

	now := time.Now()
	history := []CapacityHistory{
		{
			Timestamp:    now.AddDate(0, 0, -30),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    400 * 1024 * 1024 * 1024,
			UsagePercent: 40.0,
		},
		{
			Timestamp:    now,
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    600 * 1024 * 1024 * 1024,
			UsagePercent: 60.0,
		},
	}

	report := planner.Analyze(history, "volume1")

	assert.NotNil(t, report)

	// 验证汇总信息
	assert.Equal(t, 60.0, report.Summary.CurrentUsagePercent)
	assert.Greater(t, report.Summary.MonthlyGrowthRate, 0.0)
	assert.Equal(t, "growing", report.Summary.Trend)

	// 验证建议扩容量
	assert.Greater(t, report.Summary.RecommendedExpansionGB, uint64(0))
}

func TestCapacityPlanner_PredictCapacityNeeds(t *testing.T) {
	config := CapacityPlanningConfig{
		GrowthModel:    GrowthModelLinear,
		SafetyBuffer:   20.0,
		AlertThreshold: 70.0,
	}

	planner := NewCapacityPlanner(config)

	now := time.Now()
	history := []CapacityHistory{
		{
			Timestamp:    now.AddDate(0, 0, -30),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    400 * 1024 * 1024 * 1024,
			UsagePercent: 40.0,
		},
		{
			Timestamp:    now,
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    600 * 1024 * 1024 * 1024,
			UsagePercent: 60.0,
		},
	}

	// 预测 3 个月后的容量需求
	predicted, err := planner.PredictCapacityNeeds(history, 3)

	assert.NoError(t, err)
	assert.Greater(t, predicted, uint64(600*1024*1024*1024)) // 应该大于当前使用量

	// 预测 6 个月后的容量需求
	predicted6, err := planner.PredictCapacityNeeds(history, 6)
	assert.NoError(t, err)
	assert.Greater(t, predicted6, predicted) // 6个月应该大于3个月
}

// ========== 边界条件测试 ==========

func TestCapacityPlanner_EmptyHistory(t *testing.T) {
	config := CapacityPlanningConfig{}
	planner := NewCapacityPlanner(config)

	report := planner.Analyze([]CapacityHistory{}, "volume1")
	assert.Nil(t, report)
}

func TestCapacityPlanner_SingleDataPoint(t *testing.T) {
	config := CapacityPlanningConfig{
		ForecastDays: 30,
	}

	planner := NewCapacityPlanner(config)

	now := time.Now()
	history := []CapacityHistory{
		{
			Timestamp:    now,
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    500 * 1024 * 1024 * 1024,
			UsagePercent: 50.0,
		},
	}

	report := planner.Analyze(history, "volume1")
	assert.NotNil(t, report)
	assert.Equal(t, uint64(500*1024*1024*1024), report.Current.UsedBytes)
	// 单点数据无法预测趋势，预测结果应为空
	assert.Len(t, report.Forecasts, 0)
}

func TestCapacityPlanner_ExponentialGrowth(t *testing.T) {
	config := CapacityPlanningConfig{
		GrowthModel:    GrowthModelExponential,
		ForecastDays:   30,
		AlertThreshold: 70.0,
	}

	planner := NewCapacityPlanner(config)

	now := time.Now()
	history := []CapacityHistory{
		{
			Timestamp:    now.AddDate(0, 0, -30),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    200 * 1024 * 1024 * 1024,
			UsagePercent: 20.0,
		},
		{
			Timestamp:    now.AddDate(0, 0, -15),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    300 * 1024 * 1024 * 1024,
			UsagePercent: 30.0,
		},
		{
			Timestamp:    now,
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    450 * 1024 * 1024 * 1024,
			UsagePercent: 45.0,
		},
	}

	report := planner.Analyze(history, "volume1")

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.Forecasts)

	// 指数增长预测应该更快达到阈值
	for _, f := range report.Forecasts {
		assert.Equal(t, GrowthModelExponential, f.Model)
	}
}

func TestCapacityPlanner_LogarithmicGrowth(t *testing.T) {
	config := CapacityPlanningConfig{
		GrowthModel:    GrowthModelLogarithmic,
		ForecastDays:   30,
		AlertThreshold: 70.0,
	}

	planner := NewCapacityPlanner(config)

	now := time.Now()
	history := []CapacityHistory{
		{
			Timestamp:    now.AddDate(0, 0, -30),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    400 * 1024 * 1024 * 1024,
			UsagePercent: 40.0,
		},
		{
			Timestamp:    now.AddDate(0, 0, -15),
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    500 * 1024 * 1024 * 1024,
			UsagePercent: 50.0,
		},
		{
			Timestamp:    now,
			TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
			UsedBytes:    550 * 1024 * 1024 * 1024,
			UsagePercent: 55.0,
		},
	}

	report := planner.Analyze(history, "volume1")

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.Forecasts)

	// 对数增长预测应该更慢
	for _, f := range report.Forecasts {
		assert.Equal(t, GrowthModelLogarithmic, f.Model)
	}
}

// ========== 配置更新测试 ==========

func TestCapacityPlanner_UpdateConfig(t *testing.T) {
	config := CapacityPlanningConfig{
		AlertThreshold:    70.0,
		CriticalThreshold: 85.0,
		ForecastDays:      30,
	}

	planner := NewCapacityPlanner(config)
	assert.Equal(t, 70.0, planner.GetConfig().AlertThreshold)

	newConfig := CapacityPlanningConfig{
		AlertThreshold:    80.0,
		CriticalThreshold: 95.0,
		ForecastDays:      60,
	}

	planner.UpdateConfig(newConfig)
	assert.Equal(t, 80.0, planner.GetConfig().AlertThreshold)
	assert.Equal(t, 95.0, planner.GetConfig().CriticalThreshold)
	assert.Equal(t, 60, planner.GetConfig().ForecastDays)
}

func TestCapacityPlanner_DefaultConfig(t *testing.T) {
	config := CapacityPlanningConfig{}
	planner := NewCapacityPlanner(config)

	// 验证默认值被设置
	cfg := planner.GetConfig()
	assert.Equal(t, 70.0, cfg.AlertThreshold)
	assert.Equal(t, 85.0, cfg.CriticalThreshold)
	assert.Equal(t, 90, cfg.ForecastDays)
	assert.Equal(t, GrowthModelLinear, cfg.GrowthModel)
	assert.Equal(t, 30, cfg.ExpansionLeadTime)
	assert.Equal(t, 20.0, cfg.SafetyBuffer)
}

// ========== 紧急程度测试 ==========

func TestCapacityPlanner_UrgencyLevels(t *testing.T) {
	config := CapacityPlanningConfig{
		AlertThreshold:    70.0,
		CriticalThreshold: 85.0,
		ExpansionLeadTime: 30,
	}

	planner := NewCapacityPlanner(config)

	testCases := []struct {
		name          string
		usagePercent  float64
		expectedLevel string
	}{
		{"Low_50%", 50.0, "low"},
		{"Medium_75%", 75.0, "high"},
		{"Critical_90%", 90.0, "critical"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			history := []CapacityHistory{
				{
					Timestamp:    now.AddDate(0, 0, -1),
					TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
					UsedBytes:    uint64(float64(1*1024*1024*1024*1024) * tc.usagePercent / 100),
					UsagePercent: tc.usagePercent,
				},
				{
					Timestamp:    now,
					TotalBytes:   1 * 1024 * 1024 * 1024 * 1024,
					UsedBytes:    uint64(float64(1*1024*1024*1024*1024) * tc.usagePercent / 100),
					UsagePercent: tc.usagePercent,
				},
			}

			report := planner.Analyze(history, "volume1")
			assert.NotNil(t, report)
			assert.Equal(t, tc.expectedLevel, report.Summary.Urgency)
		})
	}
}
