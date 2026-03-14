package perf

import (
	"testing"
	"time"
)

// ========== Manager Tests ==========

func TestNewManager_NilConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableBaseline = false
	cfg.SlowLogPath = t.TempDir() + "/slow.log"
	m, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("创建管理器失败：%v", err)
	}

	if m == nil {
		t.Fatal("管理器不应为 nil")
	}

	m.Stop()
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.SlowThreshold != 500*time.Millisecond {
		t.Errorf("期望 SlowThreshold 为 500ms，得到 %v", cfg.SlowThreshold)
	}

	if cfg.SlowLogMax != 1000 {
		t.Errorf("期望 SlowLogMax 为 1000，得到 %d", cfg.SlowLogMax)
	}
}

// ========== MetricsStore Tests ==========

func TestMetricsStore_Endpoints(t *testing.T) {
	m := &MetricsStore{
		Endpoints: make(map[string]*EndpointMetrics),
	}

	key := "GET:/api/test"
	m.Endpoints[key] = &EndpointMetrics{
		Path:         "/api/test",
		Method:       "GET",
		RequestCount: 100,
	}

	if m.Endpoints[key] == nil {
		t.Error("端点指标不应为 nil")
	}

	if m.Endpoints[key].RequestCount != 100 {
		t.Errorf("期望 RequestCount 为 100，得到 %d", m.Endpoints[key].RequestCount)
	}
}

func TestMetricsStore_GlobalStats(t *testing.T) {
	m := &MetricsStore{
		TotalRequests:   1000,
		TotalErrors:     10,
		TotalDuration:   5 * time.Second,
		AvgResponseTime: 5 * time.Millisecond,
	}

	if m.TotalRequests != 1000 {
		t.Errorf("期望 TotalRequests 为 1000，得到 %d", m.TotalRequests)
	}

	if m.TotalErrors != 10 {
		t.Errorf("期望 TotalErrors 为 10，得到 %d", m.TotalErrors)
	}

	// 错误率应为 1%
	errorRate := float64(m.TotalErrors) / float64(m.TotalRequests) * 100
	if errorRate != 1.0 {
		t.Errorf("期望错误率为 1%%，得到 %.2f%%", errorRate)
	}
}

// ========== EndpointMetrics Tests ==========

func TestEndpointMetrics_Percentiles(t *testing.T) {
	em := &EndpointMetrics{
		Path:          "/api/test",
		Method:        "GET",
		RequestCount:  100,
		TotalDuration: 10 * time.Second,
		durations: []time.Duration{
			10 * time.Millisecond,
			20 * time.Millisecond,
			30 * time.Millisecond,
			40 * time.Millisecond,
			50 * time.Millisecond,
			60 * time.Millisecond,
			70 * time.Millisecond,
			80 * time.Millisecond,
			90 * time.Millisecond,
			100 * time.Millisecond,
		},
		maxHistory: 1000,
	}

	em.calculatePercentiles()

	// P50 应该是第 5 个值（50%）
	if em.P50Duration == 0 {
		t.Error("P50Duration 不应为 0")
	}

	// P95 应该是第 9 个值（95%）
	if em.P95Duration == 0 {
		t.Error("P95Duration 不应为 0")
	}

	// P99 应该接近最大值
	if em.P99Duration == 0 {
		t.Error("P99Duration 不应为 0")
	}
}

func TestEndpointMetrics_DurationTracking(t *testing.T) {
	em := &EndpointMetrics{
		Path:        "/api/test",
		Method:      "GET",
		MinDuration: 10 * time.Millisecond,
		MaxDuration: 100 * time.Millisecond,
		durations:   make([]time.Duration, 0),
		maxHistory:  100,
	}

	// 添加一些持续时间
	em.durations = append(em.durations, 10*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond)

	if len(em.durations) != 3 {
		t.Errorf("期望 3 个持续时间，得到 %d", len(em.durations))
	}

	if em.MinDuration != 10*time.Millisecond {
		t.Errorf("期望 MinDuration 为 10ms，得到 %v", em.MinDuration)
	}

	if em.MaxDuration != 100*time.Millisecond {
		t.Errorf("期望 MaxDuration 为 100ms，得到 %v", em.MaxDuration)
	}
}

// ========== TimeWindow Tests ==========

func TestTimeWindow_Requests(t *testing.T) {
	window := &TimeWindow{
		RequestsPerSecond: make(map[int64]uint64),
		ErrorsPerSecond:   make(map[int64]uint64),
		WindowSize:        60,
	}

	now := time.Now().Unix()
	window.RequestsPerSecond[now] = 100
	window.RequestsPerSecond[now-1] = 80
	window.RequestsPerSecond[now-2] = 60

	// 计算总请求数
	var total uint64
	for _, count := range window.RequestsPerSecond {
		total += count
	}

	if total != 240 {
		t.Errorf("期望总请求数为 240，得到 %d", total)
	}
}

func TestTimeWindow_WindowSize(t *testing.T) {
	window := &TimeWindow{
		WindowSize: 60,
	}

	if window.WindowSize != 60 {
		t.Errorf("期望窗口大小为 60 秒，得到 %d", window.WindowSize)
	}
}

// ========== SlowLogEntry Tests ==========

func TestSlowLogEntry_Struct(t *testing.T) {
	entry := &SlowLogEntry{
		Timestamp:   time.Now(),
		RequestID:   "req-123",
		Method:      "GET",
		Path:        "/api/slow",
		Query:       "id=1",
		ClientIP:    "127.0.0.1",
		Duration:    1 * time.Second,
		StatusCode:  200,
		UserAgent:   "Mozilla/5.0",
		RequestSize: 1024,
	}

	if entry.Duration < 500*time.Millisecond {
		t.Error("慢请求持续时间应超过阈值")
	}

	if entry.StatusCode != 200 {
		t.Errorf("期望 StatusCode 为 200，得到 %d", entry.StatusCode)
	}
}

// ========== ThroughputTracker Tests ==========

func TestThroughputTracker_MinuteStats(t *testing.T) {
	tracker := &ThroughputTracker{
		MinuteStats: make(map[int64]*MinuteStat),
		HourlyStats: make(map[int64]*HourlyStat),
		DailyStats:  make(map[int64]*DailyStat),
	}

	now := time.Now().Unix() / 60
	tracker.MinuteStats[now] = &MinuteStat{
		Timestamp:    now * 60,
		RequestCount: 100,
		ErrorCount:   5,
		TotalBytes:   1024000,
		AvgLatencyMs: 15.5,
		PeakRPS:      2.0,
	}

	stat := tracker.MinuteStats[now]
	if stat.RequestCount != 100 {
		t.Errorf("期望 RequestCount 为 100，得到 %d", stat.RequestCount)
	}
}

func TestThroughputTracker_HourlyStats(t *testing.T) {
	tracker := &ThroughputTracker{
		HourlyStats: make(map[int64]*HourlyStat),
	}

	now := time.Now().Unix() / 3600
	tracker.HourlyStats[now] = &HourlyStat{
		Timestamp:    now * 3600,
		RequestCount: 6000,
		ErrorCount:   30,
		AvgLatencyMs: 20.0,
		PeakRPS:      2.5,
	}

	stat := tracker.HourlyStats[now]
	if stat.RequestCount != 6000 {
		t.Errorf("期望 RequestCount 为 6000，得到 %d", stat.RequestCount)
	}
}

func TestThroughputTracker_DailyStats(t *testing.T) {
	tracker := &ThroughputTracker{
		DailyStats: make(map[int64]*DailyStat),
	}

	now := time.Now().Unix() / 86400
	tracker.DailyStats[now] = &DailyStat{
		Timestamp:    now * 86400,
		RequestCount: 144000,
		ErrorCount:   720,
		AvgLatencyMs: 18.0,
		PeakRPS:      3.0,
	}

	stat := tracker.DailyStats[now]
	if stat.RequestCount != 144000 {
		t.Errorf("期望 RequestCount 为 144000，得到 %d", stat.RequestCount)
	}
}

// ========== PerformanceBaseline Tests ==========

func TestPerformanceBaseline_Values(t *testing.T) {
	baseline := &PerformanceBaseline{
		AvgResponseTime: 50.0,
		P95ResponseTime: 150.0,
		P99ResponseTime: 300.0,
		AvgRPS:          100.0,
		PeakRPS:         500.0,
		AvgErrorRate:    1.0,
		LastUpdated:     time.Now(),
	}

	if baseline.AvgResponseTime != 50.0 {
		t.Errorf("期望 AvgResponseTime 为 50.0，得到 %f", baseline.AvgResponseTime)
	}

	if baseline.P95ResponseTime <= baseline.AvgResponseTime {
		t.Error("P95 应大于平均值")
	}

	if baseline.P99ResponseTime <= baseline.P95ResponseTime {
		t.Error("P99 应大于 P95")
	}
}

// ========== AlertRule Tests ==========

func TestAlertRule_Validation(t *testing.T) {
	rule := AlertRule{
		ID:          "alert-1",
		Name:        "高响应时间告警",
		Description: "响应时间超过 500ms",
		Enabled:     true,
		Metric:      "response_time",
		Operator:    ">",
		Threshold:   500,
		Duration:    5 * time.Minute,
		Severity:    "warning",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if !rule.Enabled {
		t.Error("告警规则应为启用状态")
	}

	if rule.Metric != "response_time" {
		t.Errorf("期望 Metric 为 response_time，得到 %s", rule.Metric)
	}

	if rule.Threshold != 500 {
		t.Errorf("期望 Threshold 为 500，得到 %f", rule.Threshold)
	}
}

func TestAlertRule_Severities(t *testing.T) {
	severities := []string{"info", "warning", "critical"}

	for _, severity := range severities {
		t.Run(severity, func(t *testing.T) {
			rule := AlertRule{
				Severity: severity,
			}
			if rule.Severity != severity {
				t.Errorf("期望 Severity 为 %s，得到 %s", severity, rule.Severity)
			}
		})
	}
}

func TestAlertRule_Metrics(t *testing.T) {
	validMetrics := map[string]bool{
		"response_time": true,
		"error_rate":    true,
		"rps":           true,
		"cpu":           true,
		"memory":        true,
	}

	for metric := range validMetrics {
		t.Run(metric, func(t *testing.T) {
			if !validMetrics[metric] {
				t.Errorf("无效的指标名: %s", metric)
			}
		})
	}
}

func TestAlertRule_Operators(t *testing.T) {
	validOperators := map[string]bool{
		">":  true,
		"<":  true,
		">=": true,
		"<=": true,
		"==": true,
	}

	for op := range validOperators {
		t.Run(op, func(t *testing.T) {
			if !validOperators[op] {
				t.Errorf("无效的操作符: %s", op)
			}
		})
	}
}

// ========== AlertInstance Tests ==========

func TestAlertInstance_Struct(t *testing.T) {
	alert := &AlertInstance{
		RuleID:      "alert-1",
		RuleName:    "高响应时间告警",
		Severity:    "warning",
		Message:     "响应时间超过阈值",
		Value:       600.0,
		Threshold:   500.0,
		TriggeredAt: time.Now(),
		Resolved:    false,
	}

	if alert.Resolved {
		t.Error("告警应未解决")
	}

	if alert.Value <= alert.Threshold {
		t.Error("告警值应超过阈值")
	}
}

// ========== PerformanceReport Tests ==========

func TestPerformanceReport_Struct(t *testing.T) {
	report := &PerformanceReport{
		GeneratedAt: time.Now(),
		TimeRange:   "24h",
		Summary: &ReportSummary{
			TotalRequests:   10000,
			TotalErrors:     50,
			ErrorRate:       0.5,
			AvgResponseTime: 45.0,
			P50ResponseTime: 30.0,
			P95ResponseTime: 120.0,
			P99ResponseTime: 250.0,
			PeakRPS:         500.0,
			AvgRPS:          100.0,
		},
		Endpoints: []*EndpointReport{
			{
				Path:         "/api/test",
				Method:       "GET",
				RequestCount: 5000,
				ErrorCount:   25,
				ErrorRate:    0.5,
				AvgDuration:  40.0,
			},
		},
	}

	if report.Summary.TotalRequests != 10000 {
		t.Errorf("期望 TotalRequests 为 10000，得到 %d", report.Summary.TotalRequests)
	}

	if len(report.Endpoints) != 1 {
		t.Errorf("期望 1 个端点报告，得到 %d", len(report.Endpoints))
	}
}

func TestReportSummary_ErrorRate(t *testing.T) {
	summary := &ReportSummary{
		TotalRequests: 1000,
		TotalErrors:   10,
	}

	expectedRate := float64(summary.TotalErrors) / float64(summary.TotalRequests) * 100
	if expectedRate != 1.0 {
		t.Errorf("期望错误率为 1%%，得到 %.2f%%", expectedRate)
	}
}

// ========== ExportFormat Tests ==========

func TestExportFormat_Values(t *testing.T) {
	formats := map[ExportFormat]bool{
		ExportFormatJSON:     true,
		ExportFormatCSV:      true,
		ExportFormatHTML:     true,
		ExportFormatMarkdown: true,
	}

	for format := range formats {
		t.Run(string(format), func(t *testing.T) {
			if !formats[format] {
				t.Errorf("无效的导出格式: %s", format)
			}
		})
	}
}

// ========== EndpointReport Tests ==========

func TestEndpointReport_Struct(t *testing.T) {
	report := &EndpointReport{
		Path:         "/api/test",
		Method:       "GET",
		RequestCount: 100,
		ErrorCount:   5,
		ErrorRate:    5.0,
		AvgDuration:  50.0,
		P50Duration:  40.0,
		P95Duration:  120.0,
		P99Duration:  200.0,
	}

	if report.ErrorRate != 5.0 {
		t.Errorf("期望 ErrorRate 为 5.0，得到 %f", report.ErrorRate)
	}
}

// ========== ThroughputReport Tests ==========

func TestThroughputReport_Struct(t *testing.T) {
	report := &ThroughputReport{
		CurrentRPS: 100.0,
		PeakRPS:    500.0,
		Hourly: []*HourlyStat{
			{Timestamp: time.Now().Unix(), RequestCount: 3600},
		},
	}

	if report.PeakRPS != 500.0 {
		t.Errorf("期望 PeakRPS 为 500.0，得到 %f", report.PeakRPS)
	}
}

// ========== MinuteStat Tests ==========

func TestMinuteStat_Struct(t *testing.T) {
	stat := &MinuteStat{
		Timestamp:    time.Now().Unix(),
		RequestCount: 100,
		ErrorCount:   5,
		TotalBytes:   1024000,
		AvgLatencyMs: 15.5,
		PeakRPS:      2.0,
	}

	if stat.RequestCount != 100 {
		t.Errorf("期望 RequestCount 为 100，得到 %d", stat.RequestCount)
	}
}

// ========== HourlyStat Tests ==========

func TestHourlyStat_Struct(t *testing.T) {
	stat := &HourlyStat{
		Timestamp:    time.Now().Unix(),
		RequestCount: 6000,
		ErrorCount:   30,
		AvgLatencyMs: 20.0,
		PeakRPS:      2.5,
	}

	if stat.RequestCount != 6000 {
		t.Errorf("期望 RequestCount 为 6000，得到 %d", stat.RequestCount)
	}
}

// ========== DailyStat Tests ==========

func TestDailyStat_Struct(t *testing.T) {
	stat := &DailyStat{
		Timestamp:    time.Now().Unix(),
		RequestCount: 144000,
		ErrorCount:   720,
		AvgLatencyMs: 18.0,
		PeakRPS:      3.0,
	}

	if stat.RequestCount != 144000 {
		t.Errorf("期望 RequestCount 为 144000，得到 %d", stat.RequestCount)
	}
}
