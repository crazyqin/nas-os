package compliancedashboard

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	cfg := ComplianceConfig{Enabled: true, AutoScan: true}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestStartStop(t *testing.T) {
	m := NewManager(ComplianceConfig{})
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := m.Start(); err == nil {
		t.Error("double Start should fail")
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestInitDefaultChecks(t *testing.T) {
	m := NewManager(ComplianceConfig{})
	m.Start()
	checks := m.GetChecks("", "")
	if len(checks) < 5 {
		t.Errorf("expected at least 5 default checks, got %d", len(checks))
	}
}

func TestRunScan(t *testing.T) {
	m := NewManager(ComplianceConfig{})
	m.Start()

	report, err := m.RunScan(FrameworkGDPR)
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}
	if report.Framework != FrameworkGDPR {
		t.Errorf("expected GDPR, got %s", report.Framework)
	}
	if report.TotalChecks == 0 {
		t.Error("expected some checks")
	}
	if report.OverallScore <= 0 {
		t.Error("expected positive score")
	}
}

func TestRunScanAll(t *testing.T) {
	m := NewManager(ComplianceConfig{})
	m.Start()

	report, err := m.RunScan("")
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}
	if report.TotalChecks < 5 {
		t.Errorf("expected more checks for all frameworks, got %d", report.TotalChecks)
	}
}

func TestGetReport(t *testing.T) {
	m := NewManager(ComplianceConfig{})
	m.Start()
	report, _ := m.RunScan(FrameworkISO27001)

	got, err := m.GetReport(report.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if got.ID != report.ID {
		t.Error("report ID mismatch")
	}

	_, err = m.GetReport("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent report")
	}
}

func TestListReports(t *testing.T) {
	m := NewManager(ComplianceConfig{})
	m.Start()
	m.RunScan(FrameworkGDPR)
	m.RunScan(FrameworkISO27001)
	m.RunScan(FrameworkMLPS2)

	reports, total := m.ListReports("", 1, 10)
	if total != 3 {
		t.Errorf("expected 3 reports, got %d", total)
	}
	_ = reports

	gdprReports, total := m.ListReports(FrameworkGDPR, 1, 10)
	if total != 1 {
		t.Errorf("expected 1 GDPR report, got %d", total)
	}
	_ = gdprReports
}

func TestGetStats(t *testing.T) {
	m := NewManager(ComplianceConfig{})
	m.Start()

	stats := m.GetStats()
	if stats.TotalChecks == 0 {
		t.Error("expected some checks")
	}
	if stats.OverallScore <= 0 {
		t.Error("expected positive overall score")
	}
}

func TestLogAuditEvent(t *testing.T) {
	m := NewManager(ComplianceConfig{})
	m.LogAuditEvent(AuditEvent{
		UserID:    "user1",
		UserName:  "Admin",
		Action:    "login",
		Resource:  "system",
		Result:    "success",
		RiskLevel: "low",
	})

	events, total := m.GetAuditLog("", "", 1, 10)
	if total != 1 {
		t.Errorf("expected 1 event, got %d", total)
	}
	if events[0].Action != "login" {
		t.Errorf("expected login, got %s", events[0].Action)
	}
}

func TestGetAuditLogFilter(t *testing.T) {
	m := NewManager(ComplianceConfig{})
	m.LogAuditEvent(AuditEvent{UserID: "u1", Action: "login", Result: "success"})
	m.LogAuditEvent(AuditEvent{UserID: "u2", Action: "logout", Result: "success"})
	m.LogAuditEvent(AuditEvent{UserID: "u1", Action: "download", Result: "success"})

	u1Events, _ := m.GetAuditLog("u1", "", 1, 10)
	if len(u1Events) != 2 {
		t.Errorf("expected 2 u1 events, got %d", len(u1Events))
	}

	loginEvents, _ := m.GetAuditLog("", "login", 1, 10)
	if len(loginEvents) != 1 {
		t.Errorf("expected 1 login event, got %d", len(loginEvents))
	}
}

func TestGetChecks(t *testing.T) {
	m := NewManager(ComplianceConfig{})
	m.Start()

	gdprChecks := m.GetChecks(FrameworkGDPR, "")
	if len(gdprChecks) == 0 {
		t.Error("expected GDPR checks")
	}
	for _, c := range gdprChecks {
		if c.Framework != FrameworkGDPR {
			t.Errorf("expected GDPR framework, got %s", c.Framework)
		}
	}
}

func TestGetFindings(t *testing.T) {
	m := NewManager(ComplianceConfig{})
	findings := m.GetFindings("")
	_ = findings
}

func TestConfigCRUD(t *testing.T) {
	m := NewManager(ComplianceConfig{Enabled: false})
	cfg := m.GetConfig()
	if cfg.Enabled {
		t.Error("expected disabled")
	}
	m.UpdateConfig(ComplianceConfig{Enabled: true, AutoScan: true, ScanInterval: 24})
	cfg = m.GetConfig()
	if !cfg.Enabled {
		t.Error("expected enabled")
	}
	if cfg.ScanInterval != 24 {
		t.Errorf("expected 24, got %d", cfg.ScanInterval)
	}
}
