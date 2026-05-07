package auditreport

import (
	"testing"
	"time"
)

func TestGenerateReport(t *testing.T) {
	m := NewManager()
	req := GenerateReportRequest{
		Title:  "月度安全审计报告",
		Period: "2024-01",
	}

	report := m.GenerateReport(req)
	if report.ID == "" {
		t.Error("expected report ID to be set")
	}
	if report.Title != req.Title {
		t.Errorf("expected title %q, got %q", req.Title, report.Title)
	}
	if report.Period != req.Period {
		t.Errorf("expected period %q, got %q", req.Period, report.Period)
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected GeneratedAt to be set")
	}
}

func TestReportCRUD(t *testing.T) {
	m := NewManager()

	// 生成报告
	report := m.GenerateReport(GenerateReportRequest{Title: "测试报告", Period: "2024-01"})
	if report == nil {
		t.Fatal("expected report to be created")
	}

	// 获取报告
	got, err := m.GetReport(report.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != report.ID {
		t.Errorf("expected ID %q, got %q", report.ID, got.ID)
	}

	// 列出报告
	reports := m.ListReports()
	if len(reports) != 1 {
		t.Errorf("expected 1 report, got %d", len(reports))
	}

	// 删除报告
	if err := m.DeleteReport(report.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证删除
	_, err = m.GetReport(report.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}

	// 列出空
	reports = m.ListReports()
	if len(reports) != 0 {
		t.Errorf("expected 0 reports, got %d", len(reports))
	}
}

func TestGetReportNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetReport("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent report")
	}
}

func TestDeleteReportNotFound(t *testing.T) {
	m := NewManager()
	err := m.DeleteReport("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent report")
	}
}

func TestFindingManagement(t *testing.T) {
	m := NewManager()

	// 添加发现
	f := m.AddFinding(Finding{
		Severity:       SeverityCritical,
		Category:       "访问控制",
		Description:    "未授权访问风险",
		Recommendation: "实施最小权限原则",
	})
	if f.ID == "" {
		t.Error("expected finding ID")
	}
	if f.Status != StatusOpen {
		t.Errorf("expected status open, got %q", f.Status)
	}

	// 列出发现
	findings := m.ListFindings()
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}

	// 更新发现
	newStatus := StatusAcknowledged
	updated, err := m.UpdateFinding(f.ID, UpdateFindingRequest{Status: &newStatus})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != StatusAcknowledged {
		t.Errorf("expected status acknowledged, got %q", updated.Status)
	}

	// 解决发现
	resolved, err := m.ResolveFinding(f.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Status != StatusResolved {
		t.Errorf("expected status resolved, got %q", resolved.Status)
	}
}

func TestUpdateFindingNotFound(t *testing.T) {
	m := NewManager()
	newStatus := StatusResolved
	_, err := m.UpdateFinding("nonexistent", UpdateFindingRequest{Status: &newStatus})
	if err == nil {
		t.Error("expected error for nonexistent finding")
	}
}

func TestResolveFindingNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.ResolveFinding("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent finding")
	}
}

func TestListFindingsSorting(t *testing.T) {
	m := NewManager()

	m.AddFinding(Finding{Severity: SeverityLow, Category: "test", Description: "low"})
	m.AddFinding(Finding{Severity: SeverityCritical, Category: "test", Description: "critical"})
	m.AddFinding(Finding{Severity: SeverityMedium, Category: "test", Description: "medium"})

	findings := m.ListFindings()
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(findings))
	}
	if findings[0].Severity != SeverityCritical {
		t.Errorf("expected first finding to be critical, got %q", findings[0].Severity)
	}
	if findings[2].Severity != SeverityLow {
		t.Errorf("expected last finding to be low, got %q", findings[2].Severity)
	}
}

func TestComplianceCheck(t *testing.T) {
	m := NewManager()

	// SOC2
	check := m.RunComplianceCheck(RunComplianceCheckRequest{Standard: "SOC2"})
	if check.ID == "" {
		t.Error("expected check ID")
	}
	if check.Standard != "SOC2" {
		t.Errorf("expected standard SOC2, got %q", check.Standard)
	}
	if check.Passed+check.Failed != len(check.Items) {
		t.Error("passed + failed should equal total items")
	}

	// GDPR
	gdpr := m.RunComplianceCheck(RunComplianceCheckRequest{Standard: "GDPR"})
	if gdpr.Standard != "GDPR" {
		t.Errorf("expected standard GDPR, got %q", gdpr.Standard)
	}

	// HIPAA
	hipaa := m.RunComplianceCheck(RunComplianceCheckRequest{Standard: "HIPAA"})
	if hipaa.Standard != "HIPAA" {
		t.Errorf("expected standard HIPAA, got %q", hipaa.Standard)
	}
}

func TestComplianceStatus(t *testing.T) {
	m := NewManager()
	m.RunComplianceCheck(RunComplianceCheckRequest{Standard: "SOC2"})
	m.RunComplianceCheck(RunComplianceCheckRequest{Standard: "GDPR"})

	status := m.GetComplianceStatus()
	if len(status) != 2 {
		t.Errorf("expected 2 standards, got %d", len(status))
	}
}

func TestListComplianceChecks(t *testing.T) {
	m := NewManager()
	m.RunComplianceCheck(RunComplianceCheckRequest{Standard: "SOC2"})
	m.RunComplianceCheck(RunComplianceCheckRequest{Standard: "GDPR"})

	checks := m.ListComplianceChecks()
	if len(checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(checks))
	}
}

func TestAuditEventLogging(t *testing.T) {
	m := NewManager()

	event := m.LogEvent(AuditEvent{
		UserID:   "user1",
		Action:   "login",
		Resource: "/api/v1/auth",
		IP:       "192.168.1.1",
		Result:   "success",
	})

	if event.ID == "" {
		t.Error("expected event ID")
	}
	if event.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestQueryEvents(t *testing.T) {
	m := NewManager()

	m.LogEvent(AuditEvent{UserID: "user1", Action: "login", Resource: "/auth", IP: "1.1.1.1", Result: "success"})
	m.LogEvent(AuditEvent{UserID: "user2", Action: "login", Resource: "/auth", IP: "2.2.2.2", Result: "failure"})
	m.LogEvent(AuditEvent{UserID: "user1", Action: "read", Resource: "/api/files", IP: "1.1.1.1", Result: "success"})

	// 查询所有
	events := m.QueryEvents(QueryEventsRequest{Limit: 100})
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// 按用户过滤
	events = m.QueryEvents(QueryEventsRequest{UserID: "user1", Limit: 100})
	if len(events) != 2 {
		t.Errorf("expected 2 events for user1, got %d", len(events))
	}

	// 按结果过滤
	events = m.QueryEvents(QueryEventsRequest{Result: "failure", Limit: 100})
	if len(events) != 1 {
		t.Errorf("expected 1 failure event, got %d", len(events))
	}

	// 限制数量
	events = m.QueryEvents(QueryEventsRequest{Limit: 1})
	if len(events) != 1 {
		t.Errorf("expected 1 event with limit, got %d", len(events))
	}
}

func TestExportEvents(t *testing.T) {
	m := NewManager()

	m.LogEvent(AuditEvent{UserID: "user1", Action: "login", Result: "success"})
	m.LogEvent(AuditEvent{UserID: "user2", Action: "logout", Result: "success"})

	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	events := m.ExportEvents(ExportEventsRequest{StartTime: &start, EndTime: &end})
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestSecurityScan(t *testing.T) {
	m := NewManager()

	result := m.RunSecurityScan(RunSecurityScanRequest{ScanType: "vulnerability"})
	if result.ID == "" {
		t.Error("expected scan ID")
	}
	if result.Total == 0 {
		t.Error("expected at least 1 finding")
	}
	if result.Critical+result.High+result.Medium+result.Low+result.Info != result.Total {
		t.Error("severity counts should sum to total")
	}

	// 获取扫描结果
	got, err := m.GetScanResults(result.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != result.ID {
		t.Errorf("expected ID %q, got %q", result.ID, got.ID)
	}
}

func TestGetScanResultsNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetScanResults("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent scan")
	}
}

func TestMultipleScanTypes(t *testing.T) {
	m := NewManager()

	types := []string{"vulnerability", "configuration", "network"}
	for _, scanType := range types {
		result := m.RunSecurityScan(RunSecurityScanRequest{ScanType: scanType})
		if result.Total == 0 {
			t.Errorf("expected findings for scan type %q", scanType)
		}
	}
}
