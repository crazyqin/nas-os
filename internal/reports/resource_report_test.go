// Package reports 提供资源报告测试 (v2.92.0)
package reports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== ResourceReportGenerator 测试 ==========

func TestNewResourceReportGenerator(t *testing.T) {
	// 测试创建生成器
	generator := NewResourceReportGenerator(func(period string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"resource_usage": map[string]interface{}{
				"cpu":    50.0,
				"memory": 60.0,
				"disk":   70.0,
			},
		}, nil
	})

	assert.NotNil(t, generator)
}

func TestResourceReportGenerator_GenerateDailyReport(t *testing.T) {
	generator := NewResourceReportGenerator(func(period string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"resource_usage": map[string]interface{}{
				"cpu_usage":    45.5,
				"memory_usage": 62.3,
				"disk_usage":   78.9,
			},
			"period": period,
		}, nil
	})

	report, err := generator.GenerateDailyReport()

	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Contains(t, report.Name, "日报")
	// 验证时间范围是24小时
	duration := report.Period.EndTime.Sub(report.Period.StartTime)
	assert.Equal(t, 24*time.Hour, duration)
}

func TestResourceReportGenerator_GenerateWeeklyReport(t *testing.T) {
	generator := NewResourceReportGenerator(func(period string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"resource_usage": map[string]interface{}{
				"cpu_avg":    42.0,
				"memory_avg": 58.0,
				"disk_peak":  85.0,
			},
			"period": period,
		}, nil
	})

	report, err := generator.GenerateWeeklyReport()

	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Contains(t, report.Name, "周报")
}

func TestResourceReportGenerator_GenerateHourlyReport(t *testing.T) {
	generator := NewResourceReportGenerator(func(period string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"resource_usage": map[string]interface{}{
				"current_cpu":    55.0,
				"current_memory": 65.0,
			},
			"period": period,
		}, nil
	})

	report, err := generator.GenerateHourlyReport()

	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Contains(t, report.Name, "时报")
}

func TestResourceReportGenerator_NilFunction(t *testing.T) {
	generator := NewResourceReportGenerator(nil)

	report, err := generator.GenerateDailyReport()

	assert.Error(t, err)
	assert.Nil(t, report)
	assert.Contains(t, err.Error(), "未配置")
}

func TestResourceReportGenerator_ErrorFunction(t *testing.T) {
	generator := NewResourceReportGenerator(func(period string) (map[string]interface{}, error) {
		return nil, assert.AnError
	})

	report, err := generator.GenerateDailyReport()

	assert.Error(t, err)
	assert.Nil(t, report)
}

// ========== ResourceReporter 测试 ==========

func TestNewResourceReporter(t *testing.T) {
	reporter := NewResourceReporter(DefaultResourceReportConfig())

	assert.NotNil(t, reporter)
	assert.Equal(t, 70.0, reporter.config.StorageWarningThreshold)
	assert.Equal(t, 85.0, reporter.config.StorageCriticalThreshold)
}

func TestResourceReporter_GenerateOverviewReport(t *testing.T) {
	reporter := NewResourceReporter(DefaultResourceReportConfig())

	storageMetrics := []StorageMetrics{
		{
			VolumeName:             "vol1",
			TotalCapacityBytes:     1 * 1024 * 1024 * 1024 * 1024,
			UsedCapacityBytes:      500 * 1024 * 1024 * 1024,
			AvailableCapacityBytes: 524 * 1024 * 1024 * 1024,
			FileCount:              10000,
			DirCount:               500,
			IOPS:                   1000,
		},
	}

	bandwidthHistory := map[string][]BandwidthHistoryPoint{
		"eth0": {
			{
				Timestamp: time.Now().Add(-time.Hour),
				RxBytes:   100 * 1024 * 1024,
				TxBytes:   50 * 1024 * 1024,
				RxRate:    10 * 1024 * 1024,
				TxRate:    5 * 1024 * 1024,
			},
		},
	}

	userMetrics := []UserResourceInfo{
		{
			Username:     "user1",
			UsedBytes:    100 * 1024 * 1024 * 1024,
			QuotaBytes:   200 * 1024 * 1024 * 1024,
			UsagePercent: 50.0,
		},
	}

	systemMetrics := &SystemResourceOverview{
		CPUUsage:     45.5,
		MemoryUsage:  62.3,
		Uptime:       86400 * 7,
		SystemStatus: "healthy",
	}

	report := reporter.GenerateOverviewReport(storageMetrics, bandwidthHistory, userMetrics, systemMetrics)

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, ResourceReportOverview, report.Type)
	assert.Equal(t, "资源可视化总览报告", report.Name)
	assert.NotNil(t, report.StorageOverview)
	assert.NotNil(t, report.BandwidthOverview)
	assert.NotNil(t, report.UserOverview)
	assert.NotNil(t, report.SystemOverview)
	assert.NotEmpty(t, report.Charts)
}

func TestResourceReporter_GenerateStorageReport(t *testing.T) {
	reporter := NewResourceReporter(DefaultResourceReportConfig())

	metrics := []StorageMetrics{
		{
			VolumeName:             "vol1",
			TotalCapacityBytes:     2 * 1024 * 1024 * 1024 * 1024,
			UsedCapacityBytes:      1600 * 1024 * 1024 * 1024,
			AvailableCapacityBytes: 448 * 1024 * 1024 * 1024,
			FileCount:              25000,
			DirCount:               1200,
		},
	}

	report := reporter.GenerateStorageReport(metrics)

	assert.NotNil(t, report)
	assert.Equal(t, ResourceReportStorage, report.Type)
	assert.Equal(t, "存储使用报告", report.Name)
	assert.NotNil(t, report.StorageOverview)
	assert.Equal(t, 1, report.StorageOverview.VolumeCount)
	assert.NotEmpty(t, report.Charts)

	// 验证使用率计算
	assert.InDelta(t, 78.125, report.StorageOverview.UsagePercent, 1.0)
}

func TestResourceReporter_GenerateBandwidthReport(t *testing.T) {
	reporter := NewResourceReporter(DefaultResourceReportConfig())

	history := map[string][]BandwidthHistoryPoint{
		"eth0": {
			{
				Timestamp: time.Now().Add(-2 * time.Hour),
				RxBytes:   500 * 1024 * 1024,
				TxBytes:   200 * 1024 * 1024,
				RxRate:    20 * 1024 * 1024,
				TxRate:    10 * 1024 * 1024,
			},
			{
				Timestamp: time.Now().Add(-time.Hour),
				RxBytes:   800 * 1024 * 1024,
				TxBytes:   400 * 1024 * 1024,
				RxRate:    30 * 1024 * 1024,
				TxRate:    15 * 1024 * 1024,
			},
		},
		"eth1": {
			{
				Timestamp: time.Now().Add(-time.Hour),
				RxBytes:   300 * 1024 * 1024,
				TxBytes:   150 * 1024 * 1024,
				RxRate:    15 * 1024 * 1024,
				TxRate:    8 * 1024 * 1024,
			},
		},
	}

	report := reporter.GenerateBandwidthReport(history)

	assert.NotNil(t, report)
	assert.Equal(t, ResourceReportBandwidth, report.Type)
	assert.Equal(t, "带宽使用报告", report.Name)
	assert.NotNil(t, report.BandwidthOverview)
	assert.Equal(t, 2, report.BandwidthOverview.InterfaceCount)
	assert.NotEmpty(t, report.BandwidthOverview.Interfaces)
	assert.NotEmpty(t, report.Charts)

	// 验证流量模式判断
	assert.Contains(t, []string{"balanced", "download_heavy", "upload_heavy"}, report.BandwidthOverview.TrafficPattern)
}

func TestResourceReporter_GenerateUserReport(t *testing.T) {
	reporter := NewResourceReporter(DefaultResourceReportConfig())

	metrics := []UserResourceInfo{
		{Username: "user1", UsedBytes: 500 * 1024 * 1024 * 1024, QuotaBytes: 1000 * 1024 * 1024 * 1024, UsagePercent: 50.0},
		{Username: "user2", UsedBytes: 800 * 1024 * 1024 * 1024, QuotaBytes: 1000 * 1024 * 1024 * 1024, UsagePercent: 80.0},
		{Username: "user3", UsedBytes: 1200 * 1024 * 1024 * 1024, QuotaBytes: 1000 * 1024 * 1024 * 1024, UsagePercent: 120.0},
	}

	report := reporter.GenerateUserReport(metrics)

	assert.NotNil(t, report)
	assert.Equal(t, ResourceReportUser, report.Type)
	assert.Equal(t, "用户资源报告", report.Name)
	assert.NotNil(t, report.UserOverview)
	assert.Equal(t, 3, report.UserOverview.TotalUsers)
	assert.NotEmpty(t, report.UserOverview.TopUsers)

	// 验证超限用户检测
	assert.Equal(t, 1, report.UserOverview.OverHardLimit)
	assert.Equal(t, 1, report.UserOverview.OverSoftLimit)
}

func TestResourceReporter_DetectStorageAlerts(t *testing.T) {
	config := DefaultResourceReportConfig()
	config.StorageWarningThreshold = 70.0
	config.StorageCriticalThreshold = 85.0
	reporter := NewResourceReporter(config)

	overview := &StorageOverview{
		UsagePercent: 90.0,
		Volumes: []VolumeStorageInfo{
			{Name: "vol1", UsagePercent: 92.0},
			{Name: "vol2", UsagePercent: 75.0},
		},
	}

	alerts := reporter.detectStorageAlerts(overview)

	assert.NotEmpty(t, alerts)

	// 验证有严重告警
	hasCritical := false
	for _, alert := range alerts {
		if alert.Severity == "critical" {
			hasCritical = true
			break
		}
	}
	assert.True(t, hasCritical)
}

func TestResourceReporter_DetectUserAlerts(t *testing.T) {
	reporter := NewResourceReporter(DefaultResourceReportConfig())

	overview := &UserResourceOverview{
		TopUsers: []UserResourceInfo{
			{Username: "over_quota", UsagePercent: 110.0},
			{Username: "normal", UsagePercent: 50.0},
		},
	}

	alerts := reporter.detectUserAlerts(overview)

	assert.NotEmpty(t, alerts)
	assert.Equal(t, "over_quota", alerts[0].Resource)
	assert.Equal(t, "critical", alerts[0].Severity)
}

func TestResourceReporter_GenerateStorageRecommendations(t *testing.T) {
	config := DefaultResourceReportConfig()
	config.StorageCriticalThreshold = 85.0
	reporter := NewResourceReporter(config)

	overview := &StorageOverview{
		UsagePercent:  90.0,
		TotalCapacity: 1 * 1024 * 1024 * 1024 * 1024,
	}

	recommendations := reporter.generateStorageRecommendations(overview)

	assert.NotEmpty(t, recommendations)
	assert.Equal(t, "high", recommendations[0].Priority)
	assert.Equal(t, "storage", recommendations[0].Type)
}

// ========== 辅助函数测试 ==========

func TestFormatBytesForResource(t *testing.T) {
	tests := []struct {
		bytes    uint64
		contains string
	}{
		{500, "500 B"},
		{1024, "1.0 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TiB"},
	}

	for _, tt := range tests {
		result := formatBytesForResource(tt.bytes)
		assert.Contains(t, result, tt.contains[:len(tt.contains)-3])
	}
}

func TestGetPeriodDuration(t *testing.T) {
	tests := []struct {
		period   string
		expected time.Duration
	}{
		{"hourly", time.Hour},
		{"daily", 24 * time.Hour},
		{"weekly", 7 * 24 * time.Hour},
		{"unknown", 24 * time.Hour}, // default
	}

	for _, tt := range tests {
		result := getPeriodDuration(tt.period)
		assert.Equal(t, tt.expected, result)
	}
}

// ========== MonitorDataSource 测试 ==========

func TestNewMonitorDataSource(t *testing.T) {
	config := MonitorDataSourceConfig{
		Name: "test-monitor",
		GetSystemStats: func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"cpu_usage":    45.0,
				"memory_usage": 60.0,
			}, nil
		},
	}

	ds := NewMonitorDataSource(config)

	assert.NotNil(t, ds)
	assert.Equal(t, "test-monitor", ds.Name())
}

func TestMonitorDataSource_Query(t *testing.T) {
	ds := NewMonitorDataSource(MonitorDataSourceConfig{
		Name: "test",
		GetSystemStats: func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"cpu_usage":    45.0,
				"memory_usage": 60.0,
			}, nil
		},
		GetHealthScore: func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"total_score": 85.0,
				"grade":       "good",
			}, nil
		},
	})

	// 测试系统查询
	results, err := ds.Query(map[string]interface{}{"type": "system"}, nil, nil, nil, nil, nil, 0, 0)
	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Contains(t, results[0], "cpu_usage")
}

func TestMonitorDataSource_GetSummary(t *testing.T) {
	ds := NewMonitorDataSource(MonitorDataSourceConfig{
		Name: "test",
		GetSystemStats: func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"cpu_usage":    45.0,
				"memory_usage": 60.0,
			}, nil
		},
		GetHealthScore: func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"total_score": 85.0,
				"grade":       "good",
			}, nil
		},
	})

	summary, err := ds.GetSummary(nil)
	assert.NoError(t, err)
	assert.NotNil(t, summary)
	assert.Contains(t, summary, "health_score")
	assert.Contains(t, summary, "health_grade")
}

func TestMonitorDataSource_GetAvailableFields(t *testing.T) {
	ds := NewMonitorDataSource(MonitorDataSourceConfig{Name: "test"})

	fields := ds.GetAvailableFields()

	assert.NotEmpty(t, fields)
	// 验证关键字段存在
	fieldNames := make(map[string]bool)
	for _, f := range fields {
		fieldNames[f.Name] = true
	}
	assert.True(t, fieldNames["cpu_usage"])
	assert.True(t, fieldNames["memory_usage"])
	assert.True(t, fieldNames["health_score"])
}

// ========== 配置测试 ==========

func TestDefaultResourceReportConfig(t *testing.T) {
	config := DefaultResourceReportConfig()

	assert.Equal(t, 70.0, config.StorageWarningThreshold)
	assert.Equal(t, 85.0, config.StorageCriticalThreshold)
	assert.Equal(t, 70.0, config.BandwidthHighThreshold)
	assert.Equal(t, 90.0, config.BandwidthCriticalThreshold)
	assert.Equal(t, 30, config.PredictionDays)
	assert.True(t, config.EnablePrediction)
	assert.True(t, config.EnableRecommendations)
	assert.Equal(t, 10, config.TopUsersCount)
}

// ========== 边界条件测试 ==========

func TestResourceReporter_EmptyMetrics(t *testing.T) {
	reporter := NewResourceReporter(DefaultResourceReportConfig())

	// 空存储指标
	report := reporter.GenerateStorageReport([]StorageMetrics{})
	assert.NotNil(t, report)
	assert.Equal(t, 0, report.StorageOverview.VolumeCount)

	// 空带宽历史
	report = reporter.GenerateBandwidthReport(map[string][]BandwidthHistoryPoint{})
	assert.NotNil(t, report)
	assert.Equal(t, 0, report.BandwidthOverview.InterfaceCount)

	// 空用户指标
	report = reporter.GenerateUserReport([]UserResourceInfo{})
	assert.NotNil(t, report)
	assert.Equal(t, 0, report.UserOverview.TotalUsers)
}

func TestResourceReporter_ZeroCapacity(t *testing.T) {
	reporter := NewResourceReporter(DefaultResourceReportConfig())

	metrics := []StorageMetrics{
		{VolumeName: "empty_vol"},
	}

	report := reporter.GenerateStorageReport(metrics)

	assert.NotNil(t, report)
	assert.Equal(t, uint64(0), report.StorageOverview.TotalCapacity)
	assert.Equal(t, uint64(0), report.StorageOverview.UsedCapacity)
	assert.Equal(t, 0.0, report.StorageOverview.UsagePercent)
}

// ========== 健康状态测试 ==========

func TestResourceReporter_HealthStatus(t *testing.T) {
	reporter := NewResourceReporter(DefaultResourceReportConfig())

	// 健康系统
	healthyMetrics := []StorageMetrics{
		{
			VolumeName:         "vol1",
			TotalCapacityBytes: 1000,
			UsedCapacityBytes:  500,
		},
	}

	report := reporter.GenerateStorageReport(healthyMetrics)
	alerts := reporter.detectStorageAlerts(report.StorageOverview)

	// 使用率 50%，不应该有告警
	assert.Empty(t, alerts)
}

// ========== 报告类型测试 ==========

func TestResourceReportType_Values(t *testing.T) {
	types := []ResourceReportType{
		ResourceReportOverview,
		ResourceReportStorage,
		ResourceReportBandwidth,
		ResourceReportUser,
		ResourceReportCapacity,
		ResourceReportPerformance,
	}

	for _, rt := range types {
		assert.NotEmpty(t, string(rt))
	}
}

// ========== 性能测试 ==========

func TestResourceReportGenerator_Performance(t *testing.T) {
	generator := NewResourceReportGenerator(func(period string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"resource_usage": map[string]interface{}{
				"cpu":    45.0,
				"memory": 60.0,
			},
		}, nil
	})

	start := time.Now()
	for i := 0; i < 100; i++ {
		_, _ = generator.GenerateDailyReport()
	}
	elapsed := time.Since(start)

	// 100次生成应该在1秒内完成
	assert.Less(t, elapsed.Milliseconds(), int64(1000))
}

func TestResourceReporter_Performance(t *testing.T) {
	reporter := NewResourceReporter(DefaultResourceReportConfig())

	// 创建大量测试数据
	metrics := make([]StorageMetrics, 100)
	for i := 0; i < 100; i++ {
		metrics[i] = StorageMetrics{
			VolumeName:         "vol" + string(rune(i)),
			TotalCapacityBytes: 1 * 1024 * 1024 * 1024 * 1024,
			UsedCapacityBytes:  uint64(i) * 10 * 1024 * 1024 * 1024,
		}
	}

	start := time.Now()
	for i := 0; i < 10; i++ {
		_ = reporter.GenerateStorageReport(metrics)
	}
	elapsed := time.Since(start)

	// 10次生成应该在500毫秒内完成
	assert.Less(t, elapsed.Milliseconds(), int64(500))
}
