package securityadvisor

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAnalyzeHealthySystem(t *testing.T) {
	report := Analyze(CheckInput{MFAEnabled: true, FirewallEnabled: true, AuditLogEnabled: true, BackupAgeHours: 24, LastScan: time.Unix(1, 0).UTC()})
	if report.Score != 100 || report.Grade != "A" {
		t.Fatalf("expected perfect score, got %d %s", report.Score, report.Grade)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(report.Findings))
	}
}

func TestAnalyzeRiskySystemSortsCriticalFirst(t *testing.T) {
	report := Analyze(CheckInput{SSHPasswordLogin: true, PublicShareCount: 2, PendingSecurityPatches: 6, BackupAgeHours: 120})
	if report.Score >= 70 {
		t.Fatalf("expected poor score, got %d", report.Score)
	}
	if len(report.Findings) < 4 {
		t.Fatalf("expected multiple findings, got %d", len(report.Findings))
	}
	if report.Findings[0].Severity != SeverityCritical {
		t.Fatalf("expected critical finding first, got %s", report.Findings[0].Severity)
	}
	if !strings.Contains(report.Summary(), "pending-security-patches") {
		t.Fatalf("summary missing finding: %s", report.Summary())
	}
}

func TestReportJSONUsesStableFieldNames(t *testing.T) {
	report := Analyze(CheckInput{SSHPasswordLogin: true})
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	out := string(data)
	for _, want := range []string{"\"score\"", "\"grade\"", "\"findings\"", "\"scannedAt\"", "\"scorePenalty\""} {
		if !strings.Contains(out, want) {
			t.Fatalf("json output missing %s: %s", want, out)
		}
	}
	if strings.Contains(out, "ScorePenalty") || strings.Contains(out, "ScannedAt") {
		t.Fatalf("json output should not use Go field names: %s", out)
	}
}
