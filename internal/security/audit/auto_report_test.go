package audit

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Mock Data Sources ==========

type mockVulnScanner struct {
	items []VulnItem
	err   error
}

func (m *mockVulnScanner) Scan() ([]VulnItem, error) {
	return m.items, m.err
}

func (m *mockVulnScanner) LastScanTime() time.Time {
	return time.Now().Add(-1 * time.Hour)
}

type mockConfigAuditor struct {
	items []ConfigCheckItem
	err   error
}

func (m *mockConfigAuditor) AuditChecks() ([]ConfigCheckItem, error) {
	return m.items, m.err
}

type mockLoginSource struct {
	events []LoginEvent
	err    error
}

func (m *mockLoginSource) GetLoginEvents(start, end time.Time) ([]LoginEvent, error) {
	return m.events, m.err
}

type mockAuditSource struct {
	events []AuditEvent
	err    error
}

func (m *mockAuditSource) GetEvents(start, end time.Time, category string) ([]AuditEvent, error) {
	return m.events, m.err
}

// ========== Helper: Create test report generator ==========

func newTestReportGenerator(t *testing.T) *ReportGenerator {
	t.Helper()

	vulns := []VulnItem{
		{ID: "V-001", Severity: "critical", Title: "Remote Code Execution in SSH", Affected: "openssh 8.0", Remediation: "Update to latest", Status: "open"},
		{ID: "V-002", Severity: "high", Title: "OpenSSL vulnerability", CVE: "CVE-2024-1234", CVSS: 8.1, Affected: "openssl 1.1.1", Remediation: "Upgrade", Status: "open"},
		{ID: "V-003", Severity: "medium", Title: "Weak TLS configuration", Affected: "nginx", Remediation: "Disable TLS 1.0/1.1", Status: "open"},
		{ID: "V-004", Severity: "low", Title: "Information disclosure", Affected: "HTTP headers", Remediation: "Remove server header", Status: "open"},
	}

	configChecks := []ConfigCheckItem{
		{Category: "auth", Check: "Password policy enforced", Status: "pass", Expected: "min 8 chars", Actual: "min 8 chars"},
		{Category: "network", Check: "Firewall enabled", Status: "pass", Expected: "enabled", Actual: "enabled"},
		{Category: "auth", Check: "MFA enforced for admin", Status: "fail", Expected: "required", Actual: "optional", Remediation: "Enable mandatory MFA"},
		{Category: "system", Check: "Auto-update enabled", Status: "warning", Expected: "daily", Actual: "weekly", Remediation: "Switch to daily updates"},
	}

	loginEvents := []LoginEvent{
		{Timestamp: time.Now().Add(-2 * time.Hour), UserID: "u1", Username: "admin", IP: "192.168.1.10", Success: true},
		{Timestamp: time.Now().Add(-90 * time.Minute), UserID: "u2", Username: "user1", IP: "192.168.1.11", Success: true},
		{Timestamp: time.Now().Add(-80 * time.Minute), UserID: "u2", Username: "user1", IP: "10.0.0.1", Success: false},
		{Timestamp: time.Now().Add(-70 * time.Minute), UserID: "u2", Username: "user1", IP: "10.0.0.1", Success: false},
		{Timestamp: time.Now().Add(-60 * time.Minute), UserID: "u2", Username: "user1", IP: "10.0.0.1", Success: false},
		{Timestamp: time.Now().Add(-50 * time.Minute), UserID: "u2", Username: "user1", IP: "10.0.0.1", Success: false},
		{Timestamp: time.Now().Add(-40 * time.Minute), UserID: "u2", Username: "user1", IP: "10.0.0.1", Success: false},
		{Timestamp: time.Now().Add(-30 * time.Minute), UserID: "u2", Username: "user1", IP: "10.0.0.1", Success: false},
		{Timestamp: time.Now().Add(-3 * time.Hour), UserID: "u3", Username: "admin", IP: "203.0.113.50", Success: true, Country: "US"},
		{Timestamp: time.Now().Add(-10 * time.Hour), UserID: "u1", Username: "admin", IP: "192.168.1.10", Success: true},
	}

	rg := NewReportGenerator(
		DefaultReportGeneratorConfig(),
		WithVulnScanner(&mockVulnScanner{items: vulns}),
		WithConfigAuditor(&mockConfigAuditor{items: configChecks}),
		WithLoginDataSource(&mockLoginSource{events: loginEvents}),
	)

	return rg
}

// ========== Tests ==========

func TestNewReportGenerator(t *testing.T) {
	cfg := DefaultReportGeneratorConfig()
	rg := NewReportGenerator(cfg)
	assert.NotNil(t, rg)
	assert.True(t, rg.config.Enabled)
	assert.True(t, rg.config.WeeklyEnabled)
	assert.True(t, rg.config.MonthlyEnabled)
	assert.Equal(t, "monday", rg.config.ReportDay)
	assert.Equal(t, 8, rg.config.ReportHour)
}

func TestDefaultReportGeneratorConfig(t *testing.T) {
	cfg := DefaultReportGeneratorConfig()
	assert.Equal(t, 5, cfg.AnomalyThreshold)
	assert.Equal(t, 22, cfg.OffHourStart)
	assert.Equal(t, 6, cfg.OffHourEnd)
}

func TestGenerateReport(t *testing.T) {
	rg := newTestReportGenerator(t)

	now := time.Now()
	start := now.AddDate(0, 0, -7)
	end := now

	report, err := rg.GenerateReport(ReportTypeWeekly, start, end)
	require.NoError(t, err)
	assert.NotNil(t, report)

	// Verify structure
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, ReportTypeWeekly, report.Type)
	assert.Equal(t, start, report.PeriodStart)
	assert.Equal(t, end, report.PeriodEnd)

	// Verify vulnerability section
	assert.Equal(t, 4, report.VulnSection.VulnFound)
	assert.Equal(t, 1, report.VulnSection.Critical)
	assert.Equal(t, 1, report.VulnSection.High)
	assert.Equal(t, 1, report.VulnSection.Medium)
	assert.Equal(t, 1, report.VulnSection.Low)
	assert.Len(t, report.VulnSection.Items, 4)

	// Verify config section
	assert.Equal(t, 4, report.ConfigSection.TotalChecks)
	assert.Equal(t, 2, report.ConfigSection.Passed)
	assert.Equal(t, 1, report.ConfigSection.Failed)
	assert.Equal(t, 1, report.ConfigSection.Warning)
	assert.Equal(t, 50.0, report.ConfigSection.CompliancePct)

	// Verify access section
	assert.Equal(t, 10, report.AccessSection.TotalLogins)
	assert.Equal(t, 4, report.AccessSection.SuccessfulLogins)
	assert.Equal(t, 6, report.AccessSection.FailedLogins) // 6 failed from u2
	assert.Equal(t, 4, report.AccessSection.UniqueIPs)
	assert.Len(t, report.AccessSection.TopIPs, 4) // All unique IPs
	assert.NotNil(t, report.AccessSection.TopUsers)

	// Verify summary
	assert.True(t, report.Summary.OverallScore >= 0 && report.Summary.OverallScore <= 100)
	assert.Equal(t, 1, report.Summary.CriticalIssues)
	assert.Equal(t, 1, report.Summary.HighIssues)
	assert.NotNil(t, report.Recommendations)
	assert.True(t, len(report.Recommendations) > 0)

	// Verify trends
	assert.NotNil(t, report.Trends)
}

func TestGenerateReportDisabled(t *testing.T) {
	cfg := DefaultReportGeneratorConfig()
	cfg.Enabled = false
	rg := NewReportGenerator(cfg)

	now := time.Now()
	_, err := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestGetReport(t *testing.T) {
	rg := newTestReportGenerator(t)

	now := time.Now()
	report, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)

	found := rg.GetReport(report.ID)
	assert.NotNil(t, found)
	assert.Equal(t, report.ID, found.ID)

	// Not found
	assert.Nil(t, rg.GetReport("nonexistent"))
}

func TestListReports(t *testing.T) {
	rg := newTestReportGenerator(t)

	now := time.Now()
	_, _ = rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -14), now.AddDate(0, 0, -7))
	_, _ = rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)

	reports := rg.ListReports()
	assert.Len(t, reports, 2)
}

func TestGetLatestReport(t *testing.T) {
	rg := newTestReportGenerator(t)

	// No reports yet
	assert.Nil(t, rg.GetLatestReport())

	now := time.Now()
	report1, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -14), now.AddDate(0, 0, -7))
	report2, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)

	latest := rg.GetLatestReport()
	assert.Equal(t, report2.ID, latest.ID)
	assert.NotEqual(t, report1.ID, latest.ID)
}

func TestScoreCalculation(t *testing.T) {
	calc := &DefaultScoreCalculator{}

	// No issues = 100
	score := calc.CalculateScore(nil, nil, nil)
	assert.Equal(t, 100, score)

	// With critical vuln = 85
	vulns := []VulnItem{{Severity: "critical"}}
	score = calc.CalculateScore(vulns, nil, nil)
	assert.Equal(t, 85, score)

	// With multiple issues
	vulns = []VulnItem{
		{Severity: "critical"},
		{Severity: "high"},
		{Severity: "medium"},
	}
	checks := []ConfigCheckItem{
		{Status: "fail"},
		{Status: "fail"},
	}
	anomalies := []AccessAnomaly{{}, {}, {}}

	score = calc.CalculateScore(vulns, checks, anomalies)
	// 100 - 15 - 8 - 3 - 10 - 9 = 55
	assert.Equal(t, 55, score)
}

func TestScoreFloor(t *testing.T) {
	calc := &DefaultScoreCalculator{}

	// Many critical vulns should floor at 0
	vulns := make([]VulnItem, 20)
	for i := range vulns {
		vulns[i] = VulnItem{Severity: "critical"}
	}
	score := calc.CalculateScore(vulns, nil, nil)
	assert.Equal(t, 0, score)
}

func TestCalculateWeeklyPeriod(t *testing.T) {
	// A Thursday
	thursday := time.Date(2024, 1, 18, 10, 0, 0, 0, time.UTC)
	start, end := CalculateWeeklyPeriod(thursday)

	assert.True(t, start.Before(end))
	assert.Equal(t, time.Monday, start.Weekday())
}

func TestCalculateMonthlyPeriod(t *testing.T) {
	// Feb 15, 2024
	midFeb := time.Date(2024, 2, 15, 10, 0, 0, 0, time.UTC)
	start, end := CalculateMonthlyPeriod(midFeb)

	assert.Equal(t, time.January, start.Month())
	assert.Equal(t, 1, start.Day())
	assert.Equal(t, time.January, end.Month())
	assert.Equal(t, 31, end.Day())
}

func TestShouldGenerateReportWeekly(t *testing.T) {
	cfg := DefaultReportGeneratorConfig()
	cfg.ReportDay = "thursday"
	cfg.ReportHour = 8
	rg := NewReportGenerator(cfg)

	// Thursday at 8am
	thursday8am := time.Date(2024, 1, 18, 8, 0, 0, 0, time.UTC)
	weekly, monthly := rg.ShouldGenerateReport(thursday8am)
	assert.True(t, weekly)
	assert.False(t, monthly)

	// Thursday at 9am
	thursday9am := time.Date(2024, 1, 18, 9, 0, 0, 0, time.UTC)
	weekly, _ = rg.ShouldGenerateReport(thursday9am)
	assert.False(t, weekly)

	// Monday at 8am (not configured day)
	monday8am := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	weekly, _ = rg.ShouldGenerateReport(monday8am)
	assert.False(t, weekly)
}

func TestShouldGenerateReportMonthly(t *testing.T) {
	cfg := DefaultReportGeneratorConfig()
	cfg.ReportHour = 8
	rg := NewReportGenerator(cfg)

	// 1st of month at 8am
	first := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	_, monthly := rg.ShouldGenerateReport(first)
	assert.True(t, monthly)

	// 2nd of month at 8am
	second := time.Date(2024, 1, 2, 8, 0, 0, 0, time.UTC)
	_, monthly = rg.ShouldGenerateReport(second)
	assert.False(t, monthly)
}

func TestDetectBruteForceAnomaly(t *testing.T) {
	cfg := DefaultReportGeneratorConfig()
	cfg.AnomalyThreshold = 3
	rg := NewReportGenerator(cfg)

	events := []LoginEvent{
		{Timestamp: time.Now(), UserID: "u1", Username: "admin", IP: "10.0.0.1", Success: false},
		{Timestamp: time.Now().Add(1 * time.Minute), UserID: "u1", Username: "admin", IP: "10.0.0.1", Success: false},
		{Timestamp: time.Now().Add(2 * time.Minute), UserID: "u1", Username: "admin", IP: "10.0.0.1", Success: false},
		{Timestamp: time.Now().Add(3 * time.Minute), UserID: "u1", Username: "admin", IP: "10.0.0.1", Success: false},
	}

	anomalies := rg.detectAnomalies(events)
	bruteForceCount := 0
	for _, a := range anomalies {
		if a.Type == "brute_force" {
			bruteForceCount++
		}
	}
	assert.True(t, bruteForceCount > 0, "should detect brute force anomaly")
}

func TestDetectOffHoursAnomaly(t *testing.T) {
	cfg := DefaultReportGeneratorConfig()
	cfg.OffHourStart = 22
	cfg.OffHourEnd = 6
	rg := NewReportGenerator(cfg)

	events := []LoginEvent{
		{Timestamp: time.Date(2024, 1, 15, 3, 0, 0, 0, time.UTC), UserID: "u1", Username: "admin", IP: "1.2.3.4", Success: true},
		{Timestamp: time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC), UserID: "u1", Username: "admin", IP: "1.2.3.4", Success: true},
	}

	anomalies := rg.detectAnomalies(events)
	offHourCount := 0
	for _, a := range anomalies {
		if a.Type == "off_hours" {
			offHourCount++
		}
	}
	assert.Equal(t, 1, offHourCount) // Only the 3am login
}

func TestNoAnomaliesNormalAccess(t *testing.T) {
	cfg := DefaultReportGeneratorConfig()
	cfg.AnomalyThreshold = 5
	rg := NewReportGenerator(cfg)

	events := []LoginEvent{
		{Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), UserID: "u1", Username: "user", IP: "192.168.1.1", Success: true},
		{Timestamp: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC), UserID: "u2", Username: "user2", IP: "192.168.1.2", Success: true},
	}

	anomalies := rg.detectAnomalies(events)
	assert.Len(t, anomalies, 0)
}

func TestRecommendationsWithCriticalVulns(t *testing.T) {
	rg := newTestReportGenerator(t)

	now := time.Now()
	report, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)

	found := false
	for _, rec := range report.Recommendations {
		if contains(rec, "critical vulnerabilities") || contains(rec, "URGENT") {
			found = true
			break
		}
	}
	assert.True(t, found, "should have URGENT recommendation for critical vulns")
}

func TestRecommendationsWithBruteForce(t *testing.T) {
	// Create source with many failed logins
	loginEvents := make([]LoginEvent, 150)
	for i := range loginEvents {
		loginEvents[i] = LoginEvent{
			Timestamp: time.Now().Add(-time.Duration(i) * time.Minute),
			UserID:    "u1",
			Username:  "user",
			IP:        "10.0.0.1",
			Success:   false,
		}
	}

	rg := NewReportGenerator(
		DefaultReportGeneratorConfig(),
		WithLoginDataSource(&mockLoginSource{events: loginEvents}),
	)

	now := time.Now()
	report, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)

	found := false
	for _, rec := range report.Recommendations {
		if contains(rec, "failed logins") {
			found = true
			break
		}
	}
	assert.True(t, found, "should recommend rate limiting for high failed logins")
}

func TestReportScoreDelta(t *testing.T) {
	rg := newTestReportGenerator(t)

	now := time.Now()

	// First report
	report1, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -14), now.AddDate(0, 0, -7))
	// First report has no delta (no previous)
	assert.Equal(t, 0, report1.Summary.ScoreDelta)

	// Second report
	report2, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)
	// Delta should be 0 if scores are the same (same data sources)
	assert.Equal(t, 0, report2.Summary.ScoreDelta)
}

func TestReportStatus(t *testing.T) {
	rg := newTestReportGenerator(t)

	now := time.Now()
	report, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)

	// With critical vulns, status should be "critical"
	assert.Equal(t, "critical", report.Summary.Status)
}

func TestStatsAndConfig(t *testing.T) {
	cfg := DefaultReportGeneratorConfig()
	cfg.ReportDay = "friday"
	cfg.ReportHour = 9
	rg := NewReportGenerator(cfg)

	stats := rg.Stats()
	assert.Equal(t, 0, stats["totalReports"])
	assert.True(t, stats["enabled"].(bool))
	assert.Equal(t, "friday", stats["reportDay"])
	assert.Equal(t, 9, stats["reportHour"])

	gotCfg := rg.GetConfig()
	assert.Equal(t, "friday", gotCfg.ReportDay)

	rg.UpdateConfig(ReportGeneratorConfig{ReportDay: "wednesday"})
	assert.Equal(t, "wednesday", rg.GetConfig().ReportDay)
}

func TestReportWithoutDataSources(t *testing.T) {
	rg := NewReportGenerator(DefaultReportGeneratorConfig())

	now := time.Now()
	report, err := rg.GenerateReport(ReportTypeMonthly, now.AddDate(0, -1, 0), now)
	require.NoError(t, err)

	// Should have empty sections but not crash
	assert.Equal(t, 0, report.VulnSection.VulnFound)
	assert.Equal(t, 0, report.ConfigSection.TotalChecks)
	assert.Equal(t, 0, report.AccessSection.TotalLogins)
	assert.Equal(t, 100, report.Summary.OverallScore) // No issues = 100
}

func TestMultipleReportTypes(t *testing.T) {
	rg := newTestReportGenerator(t)

	now := time.Now()

	_, _ = rg.GenerateReport(ReportTypeDaily, now.AddDate(0, 0, -1), now)
	_, _ = rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)
	_, _ = rg.GenerateReport(ReportTypeMonthly, now.AddDate(0, -1, 0), now)

	reports := rg.ListReports()
	assert.Len(t, reports, 3)

	types := make(map[ReportType]bool)
	for _, r := range reports {
		types[r.Type] = true
	}
	assert.True(t, types[ReportTypeDaily])
	assert.True(t, types[ReportTypeWeekly])
	assert.True(t, types[ReportTypeMonthly])
}

func TestReportMaxRetained(t *testing.T) {
	cfg := DefaultReportGeneratorConfig()
	rg := NewReportGenerator(cfg)

	// Override max reports for testing
	rg.maxReports = 5

	now := time.Now()
	for i := 0; i < 10; i++ {
		_, _ = rg.GenerateReport(ReportTypeDaily, now.AddDate(0, 0, -10+i), now.AddDate(0, 0, -9+i))
	}

	reports := rg.ListReports()
	assert.Len(t, reports, 5) // Only last 5 kept
}

func TestReportIDFormat(t *testing.T) {
	rg := newTestReportGenerator(t)

	now := time.Now()
	report, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)

	assert.Regexp(t, `^RPT-\d{8}-[0-9a-f]+$`, report.ID)
}

func TestReportTimestampOrdering(t *testing.T) {
	rg := newTestReportGenerator(t)

	now := time.Now()
	report, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)

	assert.True(t, report.GeneratedAt.Before(time.Now().Add(1*time.Second)))
	assert.True(t, report.GeneratedAt.After(time.Now().Add(-1*time.Second)))
}

func TestTrendsAccumulateAcrossReports(t *testing.T) {
	cfg := DefaultReportGeneratorConfig()
	rg := NewReportGenerator(cfg,
		WithVulnScanner(&mockVulnScanner{items: []VulnItem{{Severity: "low"}}}),
		WithConfigAuditor(&mockConfigAuditor{items: []ConfigCheckItem{{Status: "pass"}}}),
		WithLoginDataSource(&mockLoginSource{events: []LoginEvent{{UserID: "u1", Success: true}}}),
	)

	// Generate 3 reports with distinct timestamps by manipulating the reports slice
	now := time.Now()
	for i := 0; i < 3; i++ {
		r, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7*(3-i)), now.AddDate(0, 0, -7*(2-i)))
		// Backdate the GeneratedAt to simulate different days
		rg.mu.Lock()
		r.GeneratedAt = now.AddDate(0, 0, -2+i)
		rg.mu.Unlock()
	}

	latest := rg.GetLatestReport()
	assert.Len(t, latest.Trends.ScoreHistory, 3)
	assert.Len(t, latest.Trends.VulnHistory, 3)
	assert.Len(t, latest.Trends.LoginFailureTrend, 3)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestFunctionalOptions(t *testing.T) {
	vs := &mockVulnScanner{items: []VulnItem{{ID: "V1", Severity: "low"}}}
	ca := &mockConfigAuditor{items: []ConfigCheckItem{{Category: "auth", Check: "test", Status: "pass"}}}
	ls := &mockLoginSource{events: []LoginEvent{{UserID: "u1", Success: true}}}
	sc := &DefaultScoreCalculator{}

	rg := NewReportGenerator(
		DefaultReportGeneratorConfig(),
		WithVulnScanner(vs),
		WithConfigAuditor(ca),
		WithLoginDataSource(ls),
		WithScoreCalculator(sc),
	)

	now := time.Now()
	report, err := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)
	require.NoError(t, err)

	assert.Equal(t, 1, report.VulnSection.VulnFound)
	assert.Equal(t, 1, report.ConfigSection.TotalChecks)
	assert.Equal(t, 1, report.AccessSection.TotalLogins)
}

func TestBuildTopIPsOrdering(t *testing.T) {
	counts := map[string]int{
		"192.168.1.1": 100,
		"10.0.0.1":    50,
		"172.16.0.1":  200,
		"8.8.8.8":     75,
	}
	countries := map[string]string{
		"172.16.0.1": "CN",
		"8.8.8.8":    "US",
	}

	top := buildTopIPs(counts, countries, 3)
	require.Len(t, top, 3)

	assert.Equal(t, "172.16.0.1", top[0].IP)
	assert.Equal(t, 200, top[0].Count)
	assert.Equal(t, "CN", top[0].Country)

	assert.Equal(t, "192.168.1.1", top[1].IP)
	assert.Equal(t, 100, top[1].Count)

	assert.Equal(t, "8.8.8.8", top[2].IP)
	assert.Equal(t, 75, top[2].Count)
}

func TestBuildTopUsersOrdering(t *testing.T) {
	success := map[string]int{"u1": 50, "u2": 10, "u3": 30}
	failure := map[string]int{"u1": 5, "u2": 20, "u4": 10}
	names := map[string]string{"u1": "admin", "u2": "user1", "u3": "user2", "u4": "guest"}

	top := buildTopUsers(success, failure, names, 3)
	require.Len(t, top, 3)

	// u1: 50+5=55, u2: 10+20=30, u3: 30+0=30, u4: 0+10=10
	assert.Equal(t, "u1", top[0].UserID)
	assert.Equal(t, 55, top[0].Logins+top[0].Failures)
}

func TestReportSummaryHealthy(t *testing.T) {
	rg := NewReportGenerator(DefaultReportGeneratorConfig(),
		WithConfigAuditor(&mockConfigAuditor{items: []ConfigCheckItem{
			{Category: "auth", Check: "test", Status: "pass"},
		}}),
		WithLoginDataSource(&mockLoginSource{events: []LoginEvent{
			{UserID: "u1", Success: true, Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		}}),
	)

	now := time.Now()
	report, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)

	assert.Equal(t, "healthy", report.Summary.Status)
	assert.Equal(t, 100, report.Summary.OverallScore)
}

func TestReportConfigCompliancePercentage(t *testing.T) {
	checks := []ConfigCheckItem{
		{Category: "auth", Check: "1", Status: "pass"},
		{Category: "auth", Check: "2", Status: "pass"},
		{Category: "auth", Check: "3", Status: "fail"},
		{Category: "auth", Check: "4", Status: "fail"},
	}

	rg := NewReportGenerator(DefaultReportGeneratorConfig(),
		WithConfigAuditor(&mockConfigAuditor{items: checks}),
	)

	now := time.Now()
	report, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)

	assert.Equal(t, 50.0, report.ConfigSection.CompliancePct)
}

func TestEmptyRecommendationsNoIssues(t *testing.T) {
	rg := NewReportGenerator(DefaultReportGeneratorConfig(),
		WithConfigAuditor(&mockConfigAuditor{items: []ConfigCheckItem{
			{Category: "auth", Check: "pw", Status: "pass"},
			{Category: "auth", Check: "mfa", Status: "pass"},
			{Category: "auth", Check: "lockout", Status: "pass"},
			{Category: "auth", Check: "session", Status: "pass"},
		}}),
		WithLoginDataSource(&mockLoginSource{events: []LoginEvent{
			{UserID: "u1", Success: true, Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		}}),
	)

	now := time.Now()
	report, _ := rg.GenerateReport(ReportTypeWeekly, now.AddDate(0, 0, -7), now)

	assert.Len(t, report.Recommendations, 1)
	assert.Contains(t, report.Recommendations[0], "No critical issues")
}

func TestReportTypeString(t *testing.T) {
	assert.Equal(t, "weekly", string(ReportTypeWeekly))
	assert.Equal(t, "monthly", string(ReportTypeMonthly))
	assert.Equal(t, "daily", string(ReportTypeDaily))
}

func TestReportIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateReportID()
		assert.False(t, ids[id], fmt.Sprintf("duplicate report ID: %s", id))
		ids[id] = true
	}
}
