package wormdash

import (
	"testing"
	"time"
)

func TestNewDashboard(t *testing.T) {
	d := NewDashboard()
	if d == nil {
		t.Fatal("NewDashboard returned nil")
	}
	if d.policies == nil {
		t.Fatal("policies map is nil")
	}
	if d.alerts == nil {
		t.Fatal("alerts map is nil")
	}
}

func TestOverview(t *testing.T) {
	d := NewDashboard()
	overview := d.Overview()
	if overview.ComplianceRate != 100.0 {
		t.Errorf("expected 100%% compliance for empty dashboard, got %.1f", overview.ComplianceRate)
	}
	if overview.TotalPolicies != 0 {
		t.Errorf("expected 0 policies, got %d", overview.TotalPolicies)
	}
}

func TestAddAndListPolicies(t *testing.T) {
	d := NewDashboard()

	p1 := d.AddPolicy("test-policy-1", ScopeDirectory, "/data/docs", 365, "文档目录策略", "admin")
	if p1.ID == "" {
		t.Fatal("policy ID is empty")
	}
	if p1.Status != PolicyActive {
		t.Errorf("expected active status, got %s", p1.Status)
	}

	p2 := d.AddPolicy("test-policy-2", ScopeFileType, ".pdf", 730, "PDF文件策略", "admin")
	if p2.ID == "" {
		t.Fatal("policy ID is empty")
	}

	all := d.ListPolicies("")
	if len(all) != 2 {
		t.Errorf("expected 2 policies, got %d", len(all))
	}

	active := d.ListPolicies(PolicyActive)
	if len(active) != 2 {
		t.Errorf("expected 2 active policies, got %d", len(active))
	}
}

func TestGetAndUpdatePolicy(t *testing.T) {
	d := NewDashboard()
	p := d.AddPolicy("test", ScopeGlobal, "*", 0, "", "admin")

	got, ok := d.GetPolicy(p.ID)
	if !ok {
		t.Fatal("GetPolicy failed")
	}
	if got.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", got.Name)
	}

	newName := "updated-test"
	updated, err := d.UpdatePolicy(p.ID, &newName, nil, nil, "admin")
	if err != nil {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}
	if updated.Name != "updated-test" {
		t.Errorf("expected name 'updated-test', got '%s'", updated.Name)
	}

	_, err = d.UpdatePolicy("nonexistent", nil, nil, nil, "admin")
	if err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestDeletePolicy(t *testing.T) {
	d := NewDashboard()
	p := d.AddPolicy("to-delete", ScopeDirectory, "/tmp", 30, "", "admin")

	err := d.DeletePolicy(p.ID, "admin")
	if err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	if len(d.ListPolicies("")) != 0 {
		t.Error("expected 0 policies after delete")
	}

	err = d.DeletePolicy("nonexistent", "admin")
	if err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestGenerateReport(t *testing.T) {
	d := NewDashboard()

	// 添加一些保留记录
	d.AddRetention("f1", "/data/file1.txt", 365, "admin")
	d.AddRetention("f2", "/data/file2.txt", 0, "admin")

	req := &ReportRequest{
		ReportType:  "monthly",
		Year:        2026,
		Month:       5,
		GeneratedBy: "admin",
	}
	report, err := d.GenerateReport(req)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}
	if report.ReportType != "monthly" {
		t.Errorf("expected monthly, got %s", report.ReportType)
	}
	if report.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", report.TotalFiles)
	}

	// quarterly report
	req2 := &ReportRequest{
		ReportType:  "quarterly",
		Year:        2026,
		Quarter:     2,
		GeneratedBy: "admin",
	}
	report2, err := d.GenerateReport(req2)
	if err != nil {
		t.Fatalf("GenerateReport quarterly failed: %v", err)
	}
	if report2.ReportType != "quarterly" {
		t.Errorf("expected quarterly, got %s", report2.ReportType)
	}

	// invalid
	req3 := &ReportRequest{ReportType: "monthly", Year: 2026, Month: 13}
	_, err = d.GenerateReport(req3)
	if err == nil {
		t.Error("expected error for invalid month")
	}

	reports := d.ListReports()
	if len(reports) != 2 {
		t.Errorf("expected 2 reports, got %d", len(reports))
	}
}

func TestBypassAlerts(t *testing.T) {
	d := NewDashboard()

	alert := d.ReportBypassAttempt("/data/secret", "192.168.1.100", "user1", "尝试删除WORM文件", AlertHigh)
	if alert.ID == "" {
		t.Fatal("alert ID is empty")
	}
	if alert.Severity != AlertHigh {
		t.Errorf("expected high severity, got %s", alert.Severity)
	}

	open := d.ListAlerts(nil)
	if len(open) != 1 {
		t.Errorf("expected 1 alert, got %d", len(open))
	}

	err := d.ResolveAlert(alert.ID, "admin")
	if err != nil {
		t.Fatalf("ResolveAlert failed: %v", err)
	}

	resolved := true
	resolvedList := d.ListAlerts(&resolved)
	if len(resolvedList) != 1 {
		t.Errorf("expected 1 resolved alert, got %d", len(resolvedList))
	}

	unresolved := false
	unresolvedList := d.ListAlerts(&unresolved)
	if len(unresolvedList) != 0 {
		t.Errorf("expected 0 unresolved alerts, got %d", len(unresolvedList))
	}
}

func TestRetentionManagement(t *testing.T) {
	d := NewDashboard()

	entry := d.AddRetention("file-1", "/data/important.doc", 365, "admin")
	if entry.RetentionDays != 365 {
		t.Errorf("expected 365 days, got %d", entry.RetentionDays)
	}
	if entry.ExpiresAt == nil {
		t.Error("expected non-nil expiresAt for 365-day retention")
	}

	// permanent retention
	perm := d.AddRetention("file-2", "/data/permanent.dat", 0, "admin")
	if perm.ExpiresAt != nil {
		t.Error("expected nil expiresAt for permanent retention")
	}

	entries := d.ListRetention()
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	// extend
	extended, err := d.ExtendRetention("file-1", 30, "admin")
	if err != nil {
		t.Fatalf("ExtendRetention failed: %v", err)
	}
	if extended.Extended != 1 {
		t.Errorf("expected extended count 1, got %d", extended.Extended)
	}
	if extended.RetentionDays != 395 {
		t.Errorf("expected 395 days, got %d", extended.RetentionDays)
	}

	_, err = d.ExtendRetention("nonexistent", 30, "admin")
	if err == nil {
		t.Error("expected error for nonexistent retention")
	}
}

func TestAuditLog(t *testing.T) {
	d := NewDashboard()

	// 触发一些操作生成审计日志
	d.AddPolicy("p1", ScopeDirectory, "/data", 100, "", "admin")
	d.AddRetention("f1", "/data/file.txt", 100, "admin")
	d.ReportBypassAttempt("/data/file.txt", "10.0.0.1", "evil", "bypass", AlertCritical)

	entries := d.ListAudit("", 0)
	if len(entries) < 3 {
		t.Errorf("expected at least 3 audit entries, got %d", len(entries))
	}

	// filter by action
	policyEntries := d.ListAudit(ActionPolicyAdd, 0)
	if len(policyEntries) < 1 {
		t.Error("expected at least 1 policy audit entry")
	}

	// limit
	limited := d.ListAudit("", 2)
	if len(limited) > 2 {
		t.Errorf("expected at most 2 entries with limit, got %d", len(limited))
	}
}

func TestComplianceRate(t *testing.T) {
	d := NewDashboard()

	// 无文件时合规率应为100%
	overview := d.Overview()
	if overview.ComplianceRate != 100.0 {
		t.Errorf("expected 100%% for empty, got %.1f", overview.ComplianceRate)
	}

	// 添加受保护文件
	d.AddRetention("f1", "/a", 365, "admin")
	d.AddRetention("f2", "/b", 365, "admin")
	d.AddRetention("f3", "/c", 365, "admin")

	overview = d.Overview()
	if overview.ComplianceRate != 100.0 {
		t.Errorf("expected 100%% with no violations, got %.1f", overview.ComplianceRate)
	}

	// 添加一个绕过告警（会降低合规率）
	d.ReportBypassAttempt("/a", "", "", "bypass", AlertHigh)

	overview = d.Overview()
	if overview.ComplianceRate >= 100.0 {
		t.Errorf("expected <100%% after violation, got %.1f", overview.ComplianceRate)
	}
}

func TestReportBypassTimestamp(t *testing.T) {
	d := NewDashboard()
	before := time.Now()
	alert := d.ReportBypassAttempt("/x", "", "", "test", AlertLow)
	after := time.Now()

	if alert.DetectedAt.Before(before) || alert.DetectedAt.After(after) {
		t.Error("alert timestamp out of expected range")
	}
}
