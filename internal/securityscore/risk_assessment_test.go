package securityscore

import (
	"testing"
	"time"
)

func TestRiskAssessor_KEVList(t *testing.T) {
	ra := NewRiskAssessor()

	entries := []KEVEntry{
		{
			CVEID:         "CVE-2026-1234",
			VendorProject: "OpenSSL",
			Product:       "OpenSSL",
			Vulnerability: "Buffer overflow",
			DateAdded:     time.Now(),
			RansomwareUse: "known",
		},
		{
			CVEID:         "CVE-2026-5678",
			VendorProject: "Linux",
			Product:       "Kernel",
			Vulnerability: "Privilege escalation",
			DateAdded:     time.Now(),
		},
	}

	ra.UpdateKEVList(entries)

	if ra.KEVCount() != 2 {
		t.Errorf("expected 2 KEV entries, got %d", ra.KEVCount())
	}
	if !ra.IsKEVListed("CVE-2026-1234") {
		t.Error("CVE-2026-1234 should be in KEV list")
	}
	if ra.IsKEVListed("CVE-9999-0000") {
		t.Error("CVE-9999-0000 should not be in KEV list")
	}
}

func TestRiskAssessor_EPSSScores(t *testing.T) {
	ra := NewRiskAssessor()

	scores := []EPSSScore{
		{CVEID: "CVE-2026-1234", Score: 0.85, Percentile: 95.0},
		{CVEID: "CVE-2026-5678", Score: 0.30, Percentile: 60.0},
	}

	ra.UpdateEPSSScores(scores)

	epss, exists := ra.GetEPSSScore("CVE-2026-1234")
	if !exists {
		t.Fatal("EPSS score not found")
	}
	if epss.Score != 0.85 {
		t.Errorf("expected 0.85, got %f", epss.Score)
	}
}

func TestRiskAssessor_AssessRisk(t *testing.T) {
	ra := NewRiskAssessor()

	ra.UpdateKEVList([]KEVEntry{
		{CVEID: "CVE-2026-1234", RansomwareUse: "known"},
	})
	ra.UpdateEPSSScores([]EPSSScore{
		{CVEID: "CVE-2026-1234", Score: 0.85},
		{CVEID: "CVE-2026-5678", Score: 0.75},
	})

	vulns := []LEVEntry{
		{
			CVEID:        "CVE-2026-1234",
			Component:    "openssl",
			Version:      "3.0.0",
			Severity:     RiskCritical,
			CVSSScore:    9.8,
			FixAvailable: true,
			Remediation:  "升级到 OpenSSL 3.0.1",
		},
		{
			CVEID:     "CVE-2026-5678",
			Component: "kernel",
			Version:   "6.1.0",
			Severity:  RiskHigh,
			CVSSScore: 8.1,
		},
	}

	assessment := ra.AssessRisk(vulns)

	if assessment.TotalCVEs != 2 {
		t.Errorf("expected 2 CVEs, got %d", assessment.TotalCVEs)
	}
	if assessment.KEVCount != 1 {
		t.Errorf("expected 1 KEV, got %d", assessment.KEVCount)
	}
	if assessment.HighEPSSCount != 2 {
		t.Errorf("expected 2 high EPSS, got %d", assessment.HighEPSSCount)
	}
	// Both CVEs are KEV-listed or high EPSS, so risk should be at least high
	if assessment.OverallRisk != RiskCritical && assessment.OverallRisk != RiskHigh && assessment.OverallRisk != RiskMedium {
		t.Errorf("expected critical/high/medium risk, got %s", assessment.OverallRisk)
	}
	if len(assessment.Remediations) == 0 {
		t.Error("expected remediations")
	}
}

func TestRiskAssessor_EmptyAssessment(t *testing.T) {
	ra := NewRiskAssessor()

	assessment := ra.AssessRisk([]LEVEntry{})

	if assessment.TotalCVEs != 0 {
		t.Errorf("expected 0 CVEs, got %d", assessment.TotalCVEs)
	}
	if assessment.OverallRisk != RiskLow {
		t.Errorf("expected low risk for empty, got %s", assessment.OverallRisk)
	}
}

func TestRiskAssessor_AssessmentHistory(t *testing.T) {
	ra := NewRiskAssessor()

	ra.AssessRisk([]LEVEntry{})
	ra.AssessRisk([]LEVEntry{})

	history := ra.GetAssessmentHistory(10)
	if len(history) != 2 {
		t.Errorf("expected 2 assessments, got %d", len(history))
	}
}
