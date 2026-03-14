// Package reports 提供报表生成和管理功能
package reports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== 带宽使用统计测试 ==========

func TestBandwidthReporter_CalculateStats(t *testing.T) {
	config := BandwidthReportConfig{
		BandwidthLimitMbps:       1000, // 1Gbps
		HighUtilizationThreshold: 70.0,
		CriticalUtilizationThreshold: 90.0,
	}

	reporter := NewBandwidthReporter(config)

	now := time.Now()
	history := []BandwidthHistoryPoint{
		{
			Timestamp:  now.Add(-30 * time.Minute),
			RxBytes:    100 * 1024 * 1024, // 100MB
			TxBytes:    50 * 1024 * 1024,  // 50MB
			RxRate:     10 * 1024 * 1024,  // 10MB/s
			TxRate:     5 * 1024 * 1024,   // 5MB/s
			RxPackets:  100000,
			TxPackets:  50000,
			ErrorCount: 10,
			DropCount:  5,
		},
		{
			Timestamp:  now.Add(-15 * time.Minute),
			RxBytes:    200 * 1024 * 1024,
			TxBytes:    100 * 1024 * 1024,
			RxRate:     20 * 1024 * 1024, // 20MB/s
			TxRate:     10 * 1024 * 1024, // 10MB/s
			RxPackets:  200000,
			TxPackets:  100000,
			ErrorCount: 15,
			DropCount:  8,
		},
		{
			Timestamp:  now,
			RxBytes:    300 * 1024 * 1024,
			TxBytes:    150 * 1024 * 1024,
			RxRate:     30 * 1024 * 1024, // 30MB/s
			TxRate:     15 * 1024 * 1024, // 15MB/s
			RxPackets:  300000,
			TxPackets:  150000,
			ErrorCount: 20,
			DropCount:  10,
		},
	}

	stats := reporter.CalculateStats(history, "eth0")

	assert.Equal(t, "eth0", stats.Interface)
	assert.Equal(t, uint64(600*1024*1024), stats.TotalRxBytes)
	assert.Equal(t, uint64(300*1024*1024), stats.TotalTxBytes)
	assert.Equal(t, uint64(900*1024*1024), stats.TotalBytes)
	assert.Greater(t, stats.PeakRxRate, uint64(0))
	assert.Greater(t, stats.PeakTxRate, uint64(0))
	assert.NotZero(t, stats.CalculatedAt)
}

func TestBandwidthReporter_GenerateTrends(t *testing.T) {
	config := BandwidthReportConfig{
		BandwidthLimitMbps: 1000,
	}

	reporter := NewBandwidthReporter(config)

	now := time.Now()
	history := []BandwidthHistoryPoint{
		{
			Timestamp: now.Add(-1 * time.Hour),
			RxRate:    50 * 1024 * 1024,  // 50MB/s
			TxRate:    25 * 1024 * 1024,  // 25MB/s
		},
		{
			Timestamp: now.Add(-30 * time.Minute),
			RxRate:    100 * 1024 * 1024, // 100MB/s
			TxRate:    50 * 1024 * 1024,  // 50MB/s
		},
		{
			Timestamp: now,
			RxRate:    80 * 1024 * 1024,  // 80MB/s
			TxRate:    40 * 1024 * 1024,  // 40MB/s
		},
	}

	trends := reporter.GenerateTrends(history)

	assert.Len(t, trends, 3)
	assert.Greater(t, trends[0].RxMbps, 0.0)
	assert.Greater(t, trends[0].TxMbps, 0.0)
	assert.Greater(t, trends[0].TotalMbps, 0.0)
}

func TestBandwidthReporter_DetectAlerts(t *testing.T) {
	config := BandwidthReportConfig{
		BandwidthLimitMbps:           100, // 100Mbps
		HighUtilizationThreshold:     70.0,
		CriticalUtilizationThreshold: 90.0,
		ErrorRateThreshold:           1.0,
		DropRateThreshold:            0.5,
	}

	reporter := NewBandwidthReporter(config)

	now := time.Now()
	// 创建会触发告警的数据：速率接近带宽限制
	history := []BandwidthHistoryPoint{
		{
			Timestamp:  now.Add(-1 * time.Hour),
			RxRate:     10 * 1024 * 1024,  // 10MB/s = ~80Mbps
			TxRate:     5 * 1024 * 1024,   // 5MB/s = ~40Mbps
			RxPackets:  10000,
			TxPackets:  5000,
			ErrorCount: 200, // 高错误率
			DropCount:  100, // 高丢包率
		},
	}

	alerts := reporter.DetectAlerts(history, "eth0")

	// 应该检测到高利用率告警
	assert.NotEmpty(t, alerts)

	// 检查告警类型
	hasHighUtilization := false
	hasErrorSpike := false
	hasDropSpike := false

	for _, alert := range alerts {
		if alert.Type == "high_utilization" {
			hasHighUtilization = true
		}
		if alert.Type == "error_spike" {
			hasErrorSpike = true
		}
		if alert.Type == "drop_spike" {
			hasDropSpike = true
		}
	}

	assert.True(t, hasHighUtilization, "应该检测到高利用率告警")
	assert.True(t, hasErrorSpike, "应该检测到错误率告警")
	assert.True(t, hasDropSpike, "应该检测到丢包率告警")
}

func TestBandwidthReporter_GenerateRecommendations(t *testing.T) {
	config := BandwidthReportConfig{
		BandwidthLimitMbps:           100,
		HighUtilizationThreshold:     70.0,
		CriticalUtilizationThreshold: 90.0,
		ErrorRateThreshold:           1.0,
		DropRateThreshold:            0.5,
	}

	reporter := NewBandwidthReporter(config)

	// 创建高利用率的统计数据
	stats := []BandwidthUsageStats{
		{
			Interface:           "eth0",
			UtilizationPercent:  95.0, // 超过严重阈值
			ErrorRate:           2.0,  // 超过错误率阈值
			DropRate:            1.0,  // 超过丢包率阈值
		},
	}

	recommendations := reporter.GenerateRecommendations(stats)

	assert.NotEmpty(t, recommendations)

	// 应该包含升级带宽的建议
	hasUpgradeRecommendation := false
	for _, rec := range recommendations {
		if rec.Type == "upgrade" {
			hasUpgradeRecommendation = true
			assert.Equal(t, "high", rec.Priority)
		}
	}
	assert.True(t, hasUpgradeRecommendation, "应该包含升级带宽的建议")
}

func TestBandwidthReporter_GenerateReport(t *testing.T) {
	config := BandwidthReportConfig{
		BandwidthLimitMbps:       1000,
		HighUtilizationThreshold: 70.0,
	}

	reporter := NewBandwidthReporter(config)

	now := time.Now()
	historyByInterface := map[string][]BandwidthHistoryPoint{
		"eth0": {
			{
				Timestamp: now.Add(-1 * time.Hour),
				RxBytes:   100 * 1024 * 1024,
				TxBytes:   50 * 1024 * 1024,
				RxRate:    10 * 1024 * 1024,
				TxRate:    5 * 1024 * 1024,
			},
			{
				Timestamp: now,
				RxBytes:   200 * 1024 * 1024,
				TxBytes:   100 * 1024 * 1024,
				RxRate:    20 * 1024 * 1024,
				TxRate:    10 * 1024 * 1024,
			},
		},
	}

	period := ReportPeriod{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now,
	}

	report := reporter.GenerateReport(historyByInterface, period)

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, "带宽使用报告", report.Name)
	assert.Len(t, report.InterfaceStats, 1)
	assert.Equal(t, 1, report.Summary.InterfaceCount)
	assert.NotEmpty(t, report.Trends)
}

func TestBandwidthReporter_AnalyzeBandwidthTrend(t *testing.T) {
	config := BandwidthReportConfig{}
	reporter := NewBandwidthReporter(config)

	now := time.Now()
	history := []BandwidthHistoryPoint{
		{
			Timestamp: now.Add(-3 * time.Hour),
			RxBytes:   100 * 1024 * 1024 * 1024, // 100GB
			TxBytes:   50 * 1024 * 1024 * 1024,  // 50GB
			RxRate:    10 * 1024 * 1024,
			TxRate:    5 * 1024 * 1024,
		},
		{
			Timestamp: now.Add(-2 * time.Hour),
			RxBytes:   150 * 1024 * 1024 * 1024, // 150GB
			TxBytes:   75 * 1024 * 1024 * 1024,  // 75GB
			RxRate:    15 * 1024 * 1024,
			TxRate:    7 * 1024 * 1024,
		},
		{
			Timestamp: now.Add(-1 * time.Hour),
			RxBytes:   200 * 1024 * 1024 * 1024, // 200GB
			TxBytes:   100 * 1024 * 1024 * 1024, // 100GB
			RxRate:    20 * 1024 * 1024,
			TxRate:    10 * 1024 * 1024,
		},
	}

	analysis := reporter.AnalyzeBandwidthTrend(history)

	assert.NotNil(t, analysis)
	assert.NotZero(t, analysis.AvgRxMbps)
	assert.NotZero(t, analysis.AvgTxMbps)
	assert.NotZero(t, analysis.RxGrowthRateGBPerHour)
	assert.NotZero(t, analysis.TxGrowthRateGBPerHour)
	assert.NotZero(t, analysis.PredictedRxGB24h)
	assert.NotZero(t, analysis.PredictedTxGB24h)
	assert.Equal(t, "increasing", analysis.Trend)
}

// ========== 边界条件测试 ==========

func TestBandwidthReporter_EmptyHistory(t *testing.T) {
	config := BandwidthReportConfig{}
	reporter := NewBandwidthReporter(config)

	stats := reporter.CalculateStats([]BandwidthHistoryPoint{}, "eth0")
	assert.Equal(t, "eth0", stats.Interface)
	assert.Equal(t, uint64(0), stats.TotalRxBytes)
	assert.Equal(t, uint64(0), stats.TotalTxBytes)
}

func TestBandwidthReporter_SingleHistoryPoint(t *testing.T) {
	config := BandwidthReportConfig{
		BandwidthLimitMbps: 1000,
	}

	reporter := NewBandwidthReporter(config)

	now := time.Now()
	history := []BandwidthHistoryPoint{
		{
			Timestamp: now,
			RxBytes:   100 * 1024 * 1024,
			TxBytes:   50 * 1024 * 1024,
			RxRate:    10 * 1024 * 1024,
			TxRate:    5 * 1024 * 1024,
		},
	}

	stats := reporter.CalculateStats(history, "eth0")
	assert.Equal(t, uint64(100*1024*1024), stats.TotalRxBytes)
	assert.Equal(t, uint64(50*1024*1024), stats.TotalTxBytes)

	analysis := reporter.AnalyzeBandwidthTrend(history)
	assert.Nil(t, analysis) // 单点数据无法分析趋势
}

func TestBandwidthReporter_NoAlerts(t *testing.T) {
	config := BandwidthReportConfig{
		BandwidthLimitMbps:           10000, // 10Gbps - 很大的带宽
		HighUtilizationThreshold:     70.0,
		ErrorRateThreshold:           10.0, // 高阈值
		DropRateThreshold:            10.0, // 高阈值
	}

	reporter := NewBandwidthReporter(config)

	now := time.Now()
	history := []BandwidthHistoryPoint{
		{
			Timestamp:  now,
			RxRate:     10 * 1024 * 1024, // 低速率
			TxRate:     5 * 1024 * 1024,
			RxPackets:  10000,
			TxPackets:  5000,
			ErrorCount: 0,
			DropCount:  0,
		},
	}

	alerts := reporter.DetectAlerts(history, "eth0")
	assert.Empty(t, alerts) // 不应该有告警
}

// ========== 配置更新测试 ==========

func TestBandwidthReporter_UpdateConfig(t *testing.T) {
	config := BandwidthReportConfig{
		BandwidthLimitMbps: 1000,
	}

	reporter := NewBandwidthReporter(config)
	assert.Equal(t, 1000.0, reporter.GetConfig().BandwidthLimitMbps)

	newConfig := BandwidthReportConfig{
		BandwidthLimitMbps: 10000,
	}

	reporter.UpdateConfig(newConfig)
	assert.Equal(t, 10000.0, reporter.GetConfig().BandwidthLimitMbps)
}

// ========== 工具函数测试 ==========

func TestBandwidthToMbps(t *testing.T) {
	// 1 MB/s = 8 Mbps
	mbps := BandwidthToMbps(1 * 1024 * 1024)
	assert.InDelta(t, 8.0, mbps, 0.1)

	// 100 MB/s = 800 Mbps
	mbps = BandwidthToMbps(100 * 1024 * 1024)
	assert.InDelta(t, 800.0, mbps, 0.1)
}

func TestMbpsToBytes(t *testing.T) {
	// 8 Mbps = 1 MB/s
	bytes := MbpsToBytes(8)
	assert.Equal(t, uint64(1*1024*1024), bytes)

	// 800 Mbps = 100 MB/s
	bytes = MbpsToBytes(800)
	assert.Equal(t, uint64(100*1024*1024), bytes)
}

// ========== 汇总计算测试 ==========

func TestBandwidthSummary_TrafficPattern(t *testing.T) {
	config := BandwidthReportConfig{}
	reporter := NewBandwidthReporter(config)

	// 测试下载为主
	stats1 := []BandwidthUsageStats{
		{
			TotalRxBytes: 1000 * 1024 * 1024 * 1024, // 1000GB 接收
			TotalTxBytes: 100 * 1024 * 1024 * 1024,  // 100GB 发送
		},
	}
	summary1 := reporter.calculateSummary(stats1)
	assert.Equal(t, "download_heavy", summary1.TrafficPattern)
	assert.Equal(t, BandwidthDirectionIn, summary1.PrimaryDirection)

	// 测试上传为主
	stats2 := []BandwidthUsageStats{
		{
			TotalRxBytes: 100 * 1024 * 1024 * 1024,  // 100GB 接收
			TotalTxBytes: 1000 * 1024 * 1024 * 1024, // 1000GB 发送
		},
	}
	summary2 := reporter.calculateSummary(stats2)
	assert.Equal(t, "upload_heavy", summary2.TrafficPattern)
	assert.Equal(t, BandwidthDirectionOut, summary2.PrimaryDirection)

	// 测试平衡
	stats3 := []BandwidthUsageStats{
		{
			TotalRxBytes: 500 * 1024 * 1024 * 1024, // 500GB 接收
			TotalTxBytes: 400 * 1024 * 1024 * 1024, // 400GB 发送
		},
	}
	summary3 := reporter.calculateSummary(stats3)
	assert.Equal(t, "balanced", summary3.TrafficPattern)
}