// Package complianceautomation 测试
package complianceautomation

import (
	"testing"
)

func TestNewComplianceEngine(t *testing.T) {
	e := NewComplianceEngine()
	if e == nil {
		t.Fatal("NewComplianceEngine returned nil")
	}
	// 应该有默认检查项
	checks := e.ListChecks("")
	if len(checks) == 0 {
		t.Fatal("no default checks initialized")
	}
}

func TestListChecksByStandard(t *testing.T) {
	e := NewComplianceEngine()

	gdprChecks := e.ListChecks(StandardGDPR)
	if len(gdprChecks) == 0 {
		t.Fatal("no GDPR checks")
	}
	for _, c := range gdprChecks {
		if c.Standard != StandardGDPR {
			t.Fatalf("expected GDPR standard, got %s", c.Standard)
		}
	}

	isoChecks := e.ListChecks(StandardISO27001)
	if len(isoChecks) == 0 {
		t.Fatal("no ISO27001 checks")
	}
}

func TestUpdateCheckResult(t *testing.T) {
	e := NewComplianceEngine()

	if err := e.UpdateCheckResult("gdpr-001", CheckStatusPass, "AES-256 enabled", ""); err != nil {
		t.Fatalf("UpdateCheckResult failed: %v", err)
	}

	checks := e.ListChecks(StandardGDPR)
	for _, c := range checks {
		if c.ID == "gdpr-001" {
			if c.Status != CheckStatusPass {
				t.Fatalf("expected pass, got %s", c.Status)
			}
			return
		}
	}
	t.Fatal("gdpr-001 not found")
}

func TestUpdateCheckResultNotFound(t *testing.T) {
	e := NewComplianceEngine()
	if err := e.UpdateCheckResult("nonexistent", CheckStatusPass, "", ""); err != ErrCheckNotFound {
		t.Fatalf("expected ErrCheckNotFound, got %v", err)
	}
}

func TestRunAudit(t *testing.T) {
	e := NewComplianceEngine()

	// 设置一些检查结果
	e.UpdateCheckResult("gdpr-001", CheckStatusPass, "encrypted", "")
	e.UpdateCheckResult("gdpr-002", CheckStatusPass, "audit enabled", "")
	e.UpdateCheckResult("gdpr-003", CheckStatusFail, "", "implement data deletion")
	e.UpdateCheckResult("gdpr-004", CheckStatusWarn, "partial", "")
	e.UpdateCheckResult("gdpr-005", CheckStatusPass, "RBAC enabled", "")

	task, err := e.RunAudit(StandardGDPR)
	if err != nil {
		t.Fatalf("RunAudit failed: %v", err)
	}

	if task.Status != "completed" {
		t.Fatalf("expected completed, got %s", task.Status)
	}
	if task.Passed != 3 {
		t.Fatalf("expected 3 passed, got %d", task.Passed)
	}
	if task.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", task.Failed)
	}
	if task.Warnings != 1 {
		t.Fatalf("expected 1 warning, got %d", task.Warnings)
	}
	if task.Score <= 0 || task.Score >= 100 {
		t.Fatalf("expected score between 0-100, got %f", task.Score)
	}
}

func TestRunAuditStandardNotFound(t *testing.T) {
	e := NewComplianceEngine()
	if _, err := e.RunAudit("NONEXISTENT"); err != ErrStandardNotFound {
		t.Fatalf("expected ErrStandardNotFound, got %v", err)
	}
}

func TestGenerateReport(t *testing.T) {
	e := NewComplianceEngine()
	e.UpdateCheckResult("gdpr-001", CheckStatusPass, "ok", "")
	e.UpdateCheckResult("gdpr-002", CheckStatusFail, "", "fix this")
	e.RunAudit(StandardGDPR)

	report, err := e.GenerateReport(StandardGDPR)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if report.Standard != StandardGDPR {
		t.Fatalf("expected GDPR, got %s", report.Standard)
	}
	if len(report.Gaps) == 0 {
		t.Fatal("expected gaps to be identified")
	}
	if report.Score <= 0 {
		t.Fatal("expected positive score")
	}
}

func TestExportReport(t *testing.T) {
	e := NewComplianceEngine()
	report, _ := e.GenerateReport(StandardGDPR)

	data, err := e.ExportReport(report)
	if err != nil {
		t.Fatalf("ExportReport failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("exported data is empty")
	}
}
