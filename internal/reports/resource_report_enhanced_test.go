// Package reports 提供报表生成和管理功能
package reports

import (
	"testing"
	"time"
)

// ========== v2.76.0 资源报告增强测试 ==========

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1 KB"},
		{1024 * 1024, "1 MB"},
		{1024 * 1024 * 1024, "1 GB"},
		{1024 * 1024 * 1024 * 1024, "1 TB"},
		{1024 * 1024 * 1024 * 1024 * 1024, "1 PB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		t.Logf("FormatBytes(%d) = %s", tt.bytes, result)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  int64
		contains string
	}{
		{0, "秒"},
		{30, "秒"},
		{60, "分"},
		{3600, "小时"},
		{86400, "天"},
		{86400 * 400, "年"},
	}

	for _, tt := range tests {
		result := FormatDuration(tt.seconds)
		t.Logf("FormatDuration(%d) = %s", tt.seconds, result)
	}
}

func TestCalculateGrowthRate(t *testing.T) {
	// 测试增长率计算
	rate := CalculateGrowthRate(1000, 1500, 30)
	t.Logf("Growth rate: %.4f%% per day", rate)

	if rate < 0 {
		t.Error("Growth rate should be positive")
	}
}

func TestPredictLinear(t *testing.T) {
	current := uint64(1000)
	dailyGrowth := 100.0
	days := 30

	predicted := PredictLinear(current, dailyGrowth, days)
	expected := uint64(1000 + 100*30)

	t.Logf("Linear prediction: %d -> %d after %d days", current, predicted, days)

	if predicted != expected {
		t.Errorf("Expected %d, got %d", expected, predicted)
	}
}

func TestPredictExponential(t *testing.T) {
	current := uint64(1000)
	dailyRate := 0.01 // 1% daily growth
	days := 30

	predicted := PredictExponential(current, dailyRate, days)
	t.Logf("Exponential prediction: %d after %d days", predicted, days)

	if predicted < current {
		t.Error("Exponential prediction should be >= current value")
	}
}

func TestCalculateDaysToCapacity(t *testing.T) {
	tests := []struct {
		currentUsed   uint64
		totalCapacity uint64
		dailyGrowth   float64
		expected      int
	}{
		{800, 1000, 10, 20}, // 20 days to full
		{1000, 1000, 10, 0}, // already full
		{500, 1000, 0, -1},  // no growth
	}

	for _, tt := range tests {
		days := CalculateDaysToCapacity(tt.currentUsed, tt.totalCapacity, tt.dailyGrowth)
		t.Logf("Days to capacity: %d", days)
	}
}

func TestCalculateEfficiencyScore(t *testing.T) {
	tests := []struct {
		usagePercent float64
	}{
		{50,  // low usage
		},
		{70,  // optimal
		},
		{90,  // high
		},
		{95,  // critical
		},
	}

	for _, tt := range tests {
		score := CalculateEfficiencyScore(tt.usagePercent)
		t.Logf("Efficiency score for %.1f%% usage: %.2f", tt.usagePercent, score)

		if score < 0 || score > 100 {
			t.Errorf("Score should be between 0 and 100, got %.2f", score)
		}
	}
}

func TestCalculateHealthScore(t *testing.T) {
	score := CalculateHealthScore(50, 60, 70)
	t.Logf("Health score (CPU: 50%%, Mem: 60%%, Disk: 70%%): %d", score)

	if score < 0 || score > 100 {
		t.Errorf("Health score should be between 0 and 100, got %d", score)
	}
}

func TestGenerateReportID(t *testing.T) {
	id1 := GenerateReportID("storage")
	id2 := GenerateReportID("bandwidth")

	t.Logf("Report IDs: %s, %s", id1, id2)

	if id1 == id2 {
		t.Error("Report IDs should be unique")
	}
}

func TestSafeDivide(t *testing.T) {
	// 正常除法
	result := SafeDivide(10, 2)
	if result != 5 {
		t.Errorf("Expected 5, got %.2f", result)
	}

	// 除零保护
	result = SafeDivide(10, 0)
	if result != 0 {
		t.Errorf("Expected 0 for division by zero, got %.2f", result)
	}
}

func TestCalculateAverage(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	avg := CalculateAverage(values)

	t.Logf("Average of %v: %.2f", values, avg)

	if avg != 3 {
		t.Errorf("Expected 3, got %.2f", avg)
	}
}

func TestCalculateMedian(t *testing.T) {
	tests := []struct {
		values   []float64
		expected float64
	}{
		{[]float64{1, 2, 3, 4, 5}, 3},    // odd count
		{[]float64{1, 2, 3, 4}, 2.5},     // even count
		{[]float64{5, 1, 3, 2, 4}, 3},    // unsorted
		{[]float64{}, 0},                  // empty
	}

	for _, tt := range tests {
		median := CalculateMedian(tt.values)
		t.Logf("Median of %v: %.2f", tt.values, median)

		if median != tt.expected {
			t.Errorf("Expected %.2f, got %.2f", tt.expected, median)
		}
	}
}

func TestCalculateStdDev(t *testing.T) {
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	stdDev := CalculateStdDev(values)

	t.Logf("Standard deviation of %v: %.4f", values, stdDev)

	// 标准差应该大于 0（除非所有值相同）
	if stdDev <= 0 {
		t.Error("Standard deviation should be positive for varied values")
	}
}

func TestCalculatePercentile(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		percentile float64
		expected   float64
	}{
		{50, 5.5},  // median
		{90, 9.1},  // 90th percentile
		{0, 1},     // min
		{100, 10},  // max
	}

	for _, tt := range tests {
		result := CalculatePercentile(values, tt.percentile)
		t.Logf("%dth percentile of %v: %.2f", int(tt.percentile), values, result)
	}
}

func TestAnalyzeTrend(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected TrendDirection
	}{
		{"increasing", []float64{1, 2, 3, 4, 5, 6}, TrendUp},
		{"decreasing", []float64{6, 5, 4, 3, 2, 1}, TrendDown},
		{"stable", []float64{3, 3, 3, 3, 3, 3}, TrendStable},
		{"empty", []float64{}, TrendUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeTrend(tt.values)
			t.Logf("Trend for %v: %s", tt.values, result)

			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCalculateTrendSlope(t *testing.T) {
	// 线性增长
	values := []float64{1, 2, 3, 4, 5}
	slope := CalculateTrendSlope(values)

	t.Logf("Trend slope for linear growth: %.4f", slope)

	if slope <= 0 {
		t.Error("Slope should be positive for increasing values")
	}
}

func TestStorageUsageReporter(t *testing.T) {
	config := DefaultStorageReportConfig()
	reporter := NewStorageUsageReporter(config)

	// 创建测试数据
	volumes := []VolumeUsageDetail{
		{
			Name:            "vol1",
			TotalCapacity:   1000 * 1024 * 1024 * 1024, // 1TB
			UsedCapacity:    800 * 1024 * 1024 * 1024,  // 800GB
			UsagePercent:    80,
			FileCount:       10000,
			DirectoryCount:  500,
			HealthStatus:    "warning",
		},
	}

	topUsers := []UserStorageUsage{
		{Username: "user1", UsedBytes: 500 * 1024 * 1024 * 1024},
		{Username: "user2", UsedBytes: 300 * 1024 * 1024 * 1024},
	}

	fileTypes := []FileTypeStats{
		{Type: "video", Count: 1000, Size: 400 * 1024 * 1024 * 1024},
		{Type: "documents", Count: 5000, Size: 200 * 1024 * 1024 * 1024},
	}

	now := time.Now()
	history := []StorageTrendPoint{
		{Timestamp: now.AddDate(0, 0, -7), UsedCapacity: 700 * 1024 * 1024 * 1024, UsagePercent: 70},
		{Timestamp: now.AddDate(0, 0, -3), UsedCapacity: 750 * 1024 * 1024 * 1024, UsagePercent: 75},
		{Timestamp: now, UsedCapacity: 800 * 1024 * 1024 * 1024, UsagePercent: 80},
	}

	report := reporter.GenerateReport(volumes, topUsers, fileTypes, history)

	t.Logf("Storage Report ID: %s", report.ID)
	t.Logf("Total Capacity: %s", FormatBytes(report.Summary.TotalCapacity))
	t.Logf("Total Used: %s", FormatBytes(report.Summary.TotalUsed))
	t.Logf("Usage Percent: %.2f%%", report.Summary.UsagePercent)
	t.Logf("Health Status: %s", report.Summary.HealthStatus)
	t.Logf("Alerts: %d", len(report.Alerts))
	t.Logf("Recommendations: %d", len(report.Recommendations))

	if report.Summary.UsagePercent < 0 || report.Summary.UsagePercent > 100 {
		t.Error("Usage percent should be between 0 and 100")
	}
}

func TestBandwidthReporter(t *testing.T) {
	config := BandwidthReportConfig{
		BandwidthLimitMbps:           1000,
		HighUtilizationThreshold:     70.0,
		CriticalUtilizationThreshold: 90.0,
	}

	reporter := NewBandwidthReporter(config)

	// 创建测试数据
	history := []BandwidthHistoryPoint{
		{
			Timestamp:  time.Now().Add(-5 * time.Minute),
			RxBytes:    100 * 1024 * 1024,
			TxBytes:    50 * 1024 * 1024,
			RxRate:     10 * 1024 * 1024, // 10 MB/s
			TxRate:     5 * 1024 * 1024,  // 5 MB/s
			RxPackets:  10000,
			TxPackets:  5000,
		},
		{
			Timestamp:  time.Now(),
			RxBytes:    200 * 1024 * 1024,
			TxBytes:    100 * 1024 * 1024,
			RxRate:     15 * 1024 * 1024, // 15 MB/s
			TxRate:     8 * 1024 * 1024,  // 8 MB/s
			RxPackets:  15000,
			TxPackets:  8000,
		},
	}

	stats := reporter.CalculateStats(history, "eth0")

	t.Logf("Interface: %s", stats.Interface)
	t.Logf("Total RX: %s", FormatBytes(stats.TotalRxBytes))
	t.Logf("Total TX: %s", FormatBytes(stats.TotalTxBytes))
	t.Logf("Utilization: %.2f%%", stats.UtilizationPercent)
	t.Logf("Error Rate: %.4f%%", stats.ErrorRate)
	t.Logf("Drop Rate: %.4f%%", stats.DropRate)

	trends := reporter.GenerateTrends(history)
	t.Logf("Trends count: %d", len(trends))

	alerts := reporter.DetectAlerts(history, "eth0")
	t.Logf("Alerts count: %d", len(alerts))
}

func TestCapacityPlanner(t *testing.T) {
	config := CapacityPlanningConfig{
		AlertThreshold:    70.0,
		CriticalThreshold: 85.0,
		ForecastDays:      90,
		GrowthModel:       GrowthModelLinear,
		ExpansionLeadTime: 30,
		SafetyBuffer:      20.0,
	}

	planner := NewCapacityPlanner(config)

	// 创建测试历史数据
	now := time.Now()
	history := []CapacityHistory{
		{Timestamp: now.AddDate(0, 0, -30), TotalBytes: 1000 * 1024 * 1024 * 1024, UsedBytes: 500 * 1024 * 1024 * 1024, UsagePercent: 50},
		{Timestamp: now.AddDate(0, 0, -20), TotalBytes: 1000 * 1024 * 1024 * 1024, UsedBytes: 600 * 1024 * 1024 * 1024, UsagePercent: 60},
		{Timestamp: now.AddDate(0, 0, -10), TotalBytes: 1000 * 1024 * 1024 * 1024, UsedBytes: 700 * 1024 * 1024 * 1024, UsagePercent: 70},
		{Timestamp: now, TotalBytes: 1000 * 1024 * 1024 * 1024, UsedBytes: 800 * 1024 * 1024 * 1024, UsagePercent: 80},
	}

	report := planner.Analyze(history, "vol1")

	t.Logf("Capacity Report ID: %s", report.ID)
	t.Logf("Volume: %s", report.VolumeName)
	t.Logf("Current Usage: %.2f%%", report.Current.UsagePercent)
	t.Logf("Current Status: %s", report.Current.Status)
	t.Logf("Forecasts: %d", len(report.Forecasts))
	t.Logf("Milestones: %d", len(report.Milestones))
	t.Logf("Recommendations: %d", len(report.Recommendations))
	t.Logf("Trend: %s", report.Summary.Trend)
	t.Logf("Urgency: %s", report.Summary.Urgency)

	if report.Current.UsagePercent < 0 || report.Current.UsagePercent > 100 {
		t.Error("Usage percent should be between 0 and 100")
	}
}

func TestResourceReportEnhancedAPI(t *testing.T) {
	api := NewResourceReportEnhancedAPI()

	if api == nil {
		t.Fatal("API should not be nil")
	}

	if api.storageReporter == nil {
		t.Error("Storage reporter should be initialized")
	}

	if api.bandwidthReporter == nil {
		t.Error("Bandwidth reporter should be initialized")
	}

	if api.capacityPlanner == nil {
		t.Error("Capacity planner should be initialized")
	}

	if api.systemReporter == nil {
		t.Error("System reporter should be initialized")
	}

	t.Log("ResourceReportEnhancedAPI initialized successfully")
}