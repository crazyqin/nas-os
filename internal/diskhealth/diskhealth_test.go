package diskhealth

import (
	"testing"
	"time"
)

func TestDefaultPredictorConfig(t *testing.T) {
	config := DefaultPredictorConfig()

	if config.CheckInterval != 1*time.Hour {
		t.Errorf("expected CheckInterval 1h, got %v", config.CheckInterval)
	}
	if !config.EnableAI {
		t.Error("expected EnableAI true")
	}
	if config.AlertThreshold != 70 {
		t.Errorf("expected AlertThreshold 70, got %d", config.AlertThreshold)
	}
	if config.MaxHistoryDays != 90 {
		t.Errorf("expected MaxHistoryDays 90, got %d", config.MaxHistoryDays)
	}
	if config.TemperatureWarn != 55 {
		t.Errorf("expected TemperatureWarn 55, got %d", config.TemperatureWarn)
	}
	if config.TemperatureCrit != 65 {
		t.Errorf("expected TemperatureCrit 65, got %d", config.TemperatureCrit)
	}
}

func TestDiskStatus_Constants(t *testing.T) {
	tests := []struct {
		status   DiskStatus
		expected string
	}{
		{DiskStatusHealthy, "healthy"},
		{DiskStatusWarning, "warning"},
		{DiskStatusCritical, "critical"},
		{DiskStatusFailed, "failed"},
		{DiskStatusUnknown, "unknown"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.status))
		}
	}
}

func TestHealthScore_Constants(t *testing.T) {
	if HealthScoreExcellent != 90 {
		t.Errorf("expected HealthScoreExcellent 90, got %d", HealthScoreExcellent)
	}
	if HealthScoreCritical != 10 {
		t.Errorf("expected HealthScoreCritical 10, got %d", HealthScoreCritical)
	}
}

func TestAlertLevel_Constants(t *testing.T) {
	tests := []struct {
		level    AlertLevel
		expected string
	}{
		{AlertLevelInfo, "info"},
		{AlertLevelWarning, "warning"},
		{AlertLevelCritical, "critical"},
		{AlertLevelEmergency, "emergency"},
	}

	for _, tt := range tests {
		if string(tt.level) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.level))
		}
	}
}

func TestPredictiveAnalyzer_AddSnapshot(t *testing.T) {
	analyzer := NewPredictiveAnalyzer()

	// 添加快照
	analyzer.AddSnapshot("/dev/sda", &HealthSnapshot{
		Timestamp:   time.Now(),
		Score:       90,
		Temperature: 45,
	})

	// 验证历史记录
	history := analyzer.GetHistory("/dev/sda", 30)
	if len(history) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(history))
	}
}

func TestPredictiveAnalyzer_AnalyzeTrend(t *testing.T) {
	analyzer := NewPredictiveAnalyzer()

	// 添加多个快照，模拟下降趋势
	baseTime := time.Now().AddDate(0, 0, -10)
	for i := 0; i < 10; i++ {
		analyzer.AddSnapshot("/dev/sda", &HealthSnapshot{
			Timestamp: baseTime.AddDate(0, 0, i),
			Score:     HealthScore(90 - i*2),
		})
	}

	trend, err := analyzer.AnalyzeTrend("/dev/sda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trend.CurrentScore != HealthScore(72) {
		t.Errorf("expected current score 72, got %d", trend.CurrentScore)
	}

	if trend.TrendDirection != "down" {
		t.Errorf("expected trend down, got %s", trend.TrendDirection)
	}
}

func TestPredictiveAnalyzer_InsufficientData(t *testing.T) {
	analyzer := NewPredictiveAnalyzer()

	// 只添加一个快照
	analyzer.AddSnapshot("/dev/sda", &HealthSnapshot{
		Timestamp: time.Now(),
		Score:     90,
	})

	_, err := analyzer.AnalyzeTrend("/dev/sda")
	if err == nil {
		t.Error("expected error for insufficient data")
	}
}

func TestAlertManager_CheckAndAlert(t *testing.T) {
	manager := NewAlertManager()

	// 创建低健康评分的评估
	disk := &DiskInfo{
		Device:      "/dev/sda",
		Temperature: 45,
	}
	assessment := &HealthAssessment{
		Device: "/dev/sda",
		Score:  25, // 低于阈值
		Status: DiskStatusCritical,
	}

	alerts := manager.CheckAndAlert(disk, assessment)
	if len(alerts) == 0 {
		t.Error("expected at least one alert")
	}

	// 验证告警
	found := false
	for _, a := range alerts {
		if a.Level == AlertLevelCritical {
			found = true
		}
	}
	if !found {
		t.Error("expected critical level alert")
	}
}

func TestAlertManager_CheckTemperatureAlert(t *testing.T) {
	manager := NewAlertManager()

	disk := &DiskInfo{
		Device:      "/dev/sda",
		Temperature: 70, // 超过阈值
	}
	assessment := &HealthAssessment{
		Device: "/dev/sda",
		Score:  90,
		Status: DiskStatusHealthy,
	}

	alerts := manager.CheckAndAlert(disk, assessment)

	// 检查是否有温度告警
	found := false
	for _, a := range alerts {
		if a.Level == AlertLevelCritical && a.Device == "/dev/sda" {
			found = true
		}
	}
	if !found {
		t.Error("expected temperature alert")
	}
}

func TestAlertManager_AckAlert(t *testing.T) {
	manager := NewAlertManager()

	// 先创建一个告警
	disk := &DiskInfo{Device: "/dev/sda", Temperature: 45}
	assessment := &HealthAssessment{Device: "/dev/sda", Score: 20, Status: DiskStatusCritical}
	alerts := manager.CheckAndAlert(disk, assessment)

	if len(alerts) == 0 {
		t.Fatal("no alerts generated")
	}

	// 确认所有告警
	for _, alert := range alerts {
		err := manager.AckAlert(alert.ID)
		if err != nil {
			t.Fatalf("failed to ack alert %s: %v", alert.ID, err)
		}
	}

	// 验证告警已确认
	allAlerts := manager.GetAlerts("/dev/sda", false)
	if len(allAlerts) != 0 {
		t.Errorf("expected 0 unacked alerts, got %d", len(allAlerts))
	}

	allAlerts = manager.GetAlerts("/dev/sda", true)
	if len(allAlerts) == 0 {
		t.Error("expected at least one alert when including acked")
	}
}

func TestAlertManager_GetRules(t *testing.T) {
	manager := NewAlertManager()
	rules := manager.GetRules()

	if len(rules) < 5 {
		t.Errorf("expected at least 5 rules, got %d", len(rules))
	}
}

func TestAssessHealth_HealthyDisk(t *testing.T) {
	collector := NewSMARTCollector()

	info := &DiskInfo{
		Device:       "/dev/sda",
		Temperature:  40,
		PowerOnHours: 10000,
		SMARTAttrs:   []SMARTAttribute{},
	}

	assessment := collector.assessHealth(info)

	if assessment.Score < 80 {
		t.Errorf("expected high score for healthy disk, got %d", assessment.Score)
	}
	if assessment.Status != DiskStatusHealthy {
		t.Errorf("expected healthy status, got %s", assessment.Status)
	}
}

func TestAssessHealth_HighTemp(t *testing.T) {
	collector := NewSMARTCollector()

	info := &DiskInfo{
		Device:       "/dev/sda",
		Temperature:  70, // 过高
		PowerOnHours: 10000,
		SMARTAttrs:   []SMARTAttribute{},
	}

	assessment := collector.assessHealth(info)

	if assessment.Score >= 90 {
		t.Error("expected score reduction for high temperature")
	}
}

func TestAssessHealth_ReallocatedSectors(t *testing.T) {
	collector := NewSMARTCollector()

	info := &DiskInfo{
		Device:       "/dev/sda",
		Temperature:  40,
		PowerOnHours: 10000,
		SMARTAttrs: []SMARTAttribute{
			{ID: 5, Name: "Reallocated_Sector_Ct", RawValue: 500, Failed: false},
		},
	}

	assessment := collector.assessHealth(info)

	if assessment.Score >= 80 {
		t.Errorf("expected score reduction for reallocated sectors, got %d", assessment.Score)
	}

	if len(assessment.RiskFactors) == 0 {
		t.Error("expected risk factors for reallocated sectors")
	}
}

func TestAssessHealth_FailedSMART(t *testing.T) {
	collector := NewSMARTCollector()

	info := &DiskInfo{
		Device:       "/dev/sda",
		Temperature:  40,
		PowerOnHours: 10000,
		SMARTAttrs: []SMARTAttribute{
			{ID: 5, Name: "Reallocated_Sector_Ct", Value: 1, Threshold: 10, Failed: true},
		},
	}

	assessment := collector.assessHealth(info)

	if assessment.Score >= 80 {
		t.Error("expected score reduction for failed SMART attribute")
	}
}

func TestGetRiskLevel(t *testing.T) {
	tests := []struct {
		prob     float64
		expected string
	}{
		{0.6, "critical"},
		{0.3, "high"},
		{0.15, "medium"},
		{0.07, "low"},
		{0.01, "minimal"},
	}

	for _, tt := range tests {
		result := getRiskLevel(tt.prob)
		if result != tt.expected {
			t.Errorf("for prob %f: expected %s, got %s", tt.prob, tt.expected, result)
		}
	}
}

func TestGetRecommendations(t *testing.T) {
	// 高概率
	recs := getRecommendations(0.6)
	if len(recs) < 2 {
		t.Error("expected multiple recommendations for high probability")
	}

	// 低概率
	recs = getRecommendations(0.01)
	if len(recs) < 1 {
		t.Error("expected at least one recommendation")
	}
}
