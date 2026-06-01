package containerscanpro

import (
	"context"
	"testing"
	"time"
)

func TestNewCVEDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := NewCVEDatabase(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create CVE database: %v", err)
	}
	if db == nil {
		t.Fatal("Database is nil")
	}

	stats := db.GetStats()
	if stats["total"] == 0 {
		t.Error("Expected builtin vulnerabilities to be loaded")
	}
}

func TestCVEDatabaseLookup(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := NewCVEDatabase(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create CVE database: %v", err)
	}

	// 查找存在的 CVE
	vuln, ok := db.Lookup("CVE-2024-3094")
	if !ok {
		t.Error("Expected to find CVE-2024-3094")
	}
	if vuln != nil && vuln.Severity != SeverityCritical {
		t.Errorf("Expected critical severity, got %s", vuln.Severity)
	}

	// 查找不存在的 CVE
	_, ok = db.Lookup("CVE-9999-9999")
	if ok {
		t.Error("Expected not to find CVE-9999-9999")
	}
}

func TestCVEDatabaseSearch(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := NewCVEDatabase(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create CVE database: %v", err)
	}

	// 按关键词搜索
	results := db.SearchByKeyword("runc")
	if len(results) == 0 {
		t.Error("Expected to find results for 'runc'")
	}

	// 按严重程度搜索
	results = db.SearchBySeverity(SeverityCritical)
	if len(results) == 0 {
		t.Error("Expected to find critical vulnerabilities")
	}
}

func TestCVEDatabaseAddVulnerability(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := NewCVEDatabase(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create CVE database: %v", err)
	}

	newVuln := &CVEInfo{
		ID:          "CVE-TEST-001",
		Title:       "Test Vulnerability",
		Description: "This is a test vulnerability",
		Severity:    SeverityHigh,
		Score:       7.5,
		PublishedAt: time.Now(),
	}

	err = db.AddVulnerability(newVuln)
	if err != nil {
		t.Fatalf("Failed to add vulnerability: %v", err)
	}

	// 验证已添加
	vuln, ok := db.Lookup("CVE-TEST-001")
	if !ok {
		t.Error("Expected to find CVE-TEST-001")
	}
	if vuln != nil && vuln.Title != "Test Vulnerability" {
		t.Errorf("Expected title 'Test Vulnerability', got '%s'", vuln.Title)
	}
}

func TestScanStats(t *testing.T) {
	stats := &ScanStats{}

	stats.IncrementScans()
	stats.IncrementCompleted()
	stats.IncrementFailed()
	stats.AddVulns(1, 2, 3, 4)
	stats.AddAnomalies(5)
	stats.UpdateLastScan()

	s := stats.GetStats()
	if s.TotalScans != 1 {
		t.Errorf("Expected 1 total scan, got %d", s.TotalScans)
	}
	if s.CompletedScans != 1 {
		t.Errorf("Expected 1 completed scan, got %d", s.CompletedScans)
	}
	if s.FailedScans != 1 {
		t.Errorf("Expected 1 failed scan, got %d", s.FailedScans)
	}
	if s.CriticalVulns != 1 {
		t.Errorf("Expected 1 critical vuln, got %d", s.CriticalVulns)
	}
	if s.HighVulns != 2 {
		t.Errorf("Expected 2 high vulns, got %d", s.HighVulns)
	}
	if s.MediumVulns != 3 {
		t.Errorf("Expected 3 medium vulns, got %d", s.MediumVulns)
	}
	if s.LowVulns != 4 {
		t.Errorf("Expected 4 low vulns, got %d", s.LowVulns)
	}
	if s.TotalAnomalies != 5 {
		t.Errorf("Expected 5 anomalies, got %d", s.TotalAnomalies)
	}
	if s.LastScanTime == nil {
		t.Error("Expected last scan time to be set")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config == nil {
		t.Fatal("Default config is nil")
	}

	if !config.Scan.EnableCVEScan {
		t.Error("Expected CVE scan to be enabled by default")
	}
	if !config.Scan.EnableRuntimeMonitor {
		t.Error("Expected runtime monitor to be enabled by default")
	}
	if config.Scan.MaxConcurrent != 5 {
		t.Errorf("Expected max concurrent 5, got %d", config.Scan.MaxConcurrent)
	}
	if config.Alert.MinLevel != AlertLevelWarning {
		t.Errorf("Expected min alert level 'warning', got '%s'", config.Alert.MinLevel)
	}
}

func TestAlerter(t *testing.T) {
	config := &AlertConfig{
		Enabled:     true,
		MinLevel:    AlertLevelWarning,
		CooldownSec: 1,
	}

	alerter := NewAlerter(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := alerter.Start(ctx); err != nil {
		t.Fatalf("Failed to start alerter: %v", err)
	}

	// 发送告警
	alert := Alert{
		Level:   AlertLevelCritical,
		Title:   "Test Alert",
		Message: "This is a test alert",
		Source:  "test",
	}

	alerter.SendAlert(alert)
	time.Sleep(100 * time.Millisecond)

	alerts := alerter.GetAlerts()
	if len(alerts) == 0 {
		t.Error("Expected at least one alert")
	}

	// 测试统计
	stats := alerter.GetAlertStats()
	if stats["critical"] != 1 {
		t.Errorf("Expected 1 critical alert, got %d", stats["critical"])
	}
}

func TestContainerScanProVersion(t *testing.T) {
	version := Version()
	if version == "" {
		t.Error("Expected non-empty version")
	}
	if version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", version)
	}
}

func TestSeverityWeight(t *testing.T) {
	tests := []struct {
		severity Severity
		expected int
	}{
		{SeverityCritical, 5},
		{SeverityHigh, 4},
		{SeverityMedium, 3},
		{SeverityLow, 2},
		{SeverityInfo, 1},
		{"unknown", 0},
	}

	for _, test := range tests {
		weight := severityWeight(test.severity)
		if weight != test.expected {
			t.Errorf("severityWeight(%s): expected %d, got %d", test.severity, test.expected, weight)
		}
	}
}

func TestCountVulnsBySeverity(t *testing.T) {
	vulns := []VulnerabilityCVE{
		{Severity: SeverityCritical},
		{Severity: SeverityCritical},
		{Severity: SeverityHigh},
		{Severity: SeverityMedium},
		{Severity: SeverityMedium},
		{Severity: SeverityMedium},
		{Severity: SeverityLow},
	}

	critical, high, medium, low := countVulnsBySeverity(vulns)
	if critical != 2 {
		t.Errorf("Expected 2 critical, got %d", critical)
	}
	if high != 1 {
		t.Errorf("Expected 1 high, got %d", high)
	}
	if medium != 3 {
		t.Errorf("Expected 3 medium, got %d", medium)
	}
	if low != 1 {
		t.Errorf("Expected 1 low, got %d", low)
	}
}

func TestRuntimeScannerAnomalies(t *testing.T) {
	config := &ScanConfig{
		ExcludedContainers: []string{"excluded-container"},
	}

	scanner := NewRuntimeScanner(config)

	// 模拟异常
	anomaly := RuntimeAnomaly{
		Type:        AnomalySuspiciousProcess,
		ContainerID: "container-123",
		Description: "Test anomaly",
		Severity:    SeverityMedium,
	}

	scanner.anomalies = append(scanner.anomalies, anomaly)

	// 获取所有异常
	anomalies := scanner.GetAnomalies()
	if len(anomalies) != 1 {
		t.Errorf("Expected 1 anomaly, got %d", len(anomalies))
	}

	// 按容器获取异常
	containerAnomalies := scanner.GetAnomaliesByContainer("container-123")
	if len(containerAnomalies) != 1 {
		t.Errorf("Expected 1 anomaly for container, got %d", len(containerAnomalies))
	}

	// 不存在的容器
	containerAnomalies = scanner.GetAnomaliesByContainer("non-existent")
	if len(containerAnomalies) != 0 {
		t.Errorf("Expected 0 anomalies for non-existent container, got %d", len(containerAnomalies))
	}

	// 清除异常
	scanner.ClearAnomalies()
	anomalies = scanner.GetAnomalies()
	if len(anomalies) != 0 {
		t.Errorf("Expected 0 anomalies after clear, got %d", len(anomalies))
	}
}

func TestAlerterCooldown(t *testing.T) {
	config := &AlertConfig{
		Enabled:     true,
		MinLevel:    AlertLevelInfo,
		CooldownSec: 2,
	}

	alerter := NewAlerter(config)

	// 第一次告警应该通过
	alert := Alert{
		Title:   "Test Alert",
		Level:   AlertLevelWarning,
		Message: "Test",
	}

	if alerter.isInCooldown(alert) {
		t.Error("First alert should not be in cooldown")
	}

	// 记录告警时间
	alerter.mu.Lock()
	alerter.lastAlerts[alert.Title] = time.Now()
	alerter.mu.Unlock()

	// 立即再次告警应该在冷却期
	if !alerter.isInCooldown(alert) {
		t.Error("Second alert should be in cooldown")
	}
}

func TestScannerCalculateScore(t *testing.T) {
	config := &ScanConfig{}
	cveDB := &CVEDatabase{}
	runtime := &RuntimeScanner{}
	alerter := &Alerter{}
	scanner := NewScanner(config, cveDB, runtime, alerter)

	// 无漏洞和异常 - 满分
	score := scanner.calculateScore(nil, nil)
	if score.Overall != 100 {
		t.Errorf("Expected score 100, got %.1f", score.Overall)
	}
	if score.Grade != "A" {
		t.Errorf("Expected grade A, got %s", score.Grade)
	}

	// 有漏洞
	vulns := []VulnerabilityCVE{
		{Severity: SeverityCritical},
		{Severity: SeverityHigh},
	}
	score = scanner.calculateScore(vulns, nil)
	if score.Overall >= 100 {
		t.Errorf("Expected score < 100 with vulnerabilities, got %.1f", score.Overall)
	}

	// 有异常
	anomalies := []RuntimeAnomaly{
		{Severity: SeverityCritical},
	}
	score = scanner.calculateScore(nil, anomalies)
	if score.Overall >= 100 {
		t.Errorf("Expected score < 100 with anomalies, got %.1f", score.Overall)
	}
}

func TestScannerGenerateRecommendations(t *testing.T) {
	config := &ScanConfig{}
	cveDB := &CVEDatabase{}
	runtime := &RuntimeScanner{}
	alerter := &Alerter{}
	scanner := NewScanner(config, cveDB, runtime, alerter)

	// 无漏洞和异常
	recs := scanner.generateRecommendations(nil, nil)
	if len(recs) == 0 {
		t.Error("Expected at least general recommendations")
	}

	// 有漏洞
	vulns := []VulnerabilityCVE{
		{
			CVEID:      "CVE-TEST-001",
			FixVersion: "1.2.3",
			Package:    "test-pkg",
		},
	}
	recs = scanner.generateRecommendations(vulns, nil)
	found := false
	for _, r := range recs {
		if contains(r, "test-pkg") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected recommendation mentioning test-pkg")
	}

	// 有异常
	anomalies := []RuntimeAnomaly{
		{Type: AnomalyPrivilegeEscalation},
	}
	recs = scanner.generateRecommendations(nil, anomalies)
	found = false
	for _, r := range recs {
		if contains(r, "SUID") || contains(r, "privilege") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected recommendation about SUID/privileges")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
