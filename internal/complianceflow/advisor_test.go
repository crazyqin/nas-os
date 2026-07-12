package complianceflow

import (
	"testing"
	"time"
)

func TestAnalyze_PII_NoGDPR(t *testing.T) {
	recs := Analyze(Signal{
		PIIDataDetected: true,
		HasGDPR:        false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "compliance-start-gdpr" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected critical, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected compliance-start-gdpr recommendation")
	}
}

func TestAnalyze_PII_CrossBorder_NoPIPL(t *testing.T) {
	recs := Analyze(Signal{
		PIIDataDetected:  true,
		CrossBorderData: true,
		HasPIPL:         false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "compliance-start-pipl" {
			found = true
		}
	}
	if !found {
		t.Error("expected compliance-start-pipl recommendation")
	}
}

func TestAnalyze_PHI_NoHIPAA(t *testing.T) {
	recs := Analyze(Signal{
		PHIDataDetected: true,
		HasHIPAA:       false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "compliance-start-hipaa" {
			found = true
		}
	}
	if !found {
		t.Error("expected compliance-start-hipaa recommendation")
	}
}

func TestAnalyze_PaymentData_NoPCI(t *testing.T) {
	recs := Analyze(Signal{
		PaymentDataDetected: true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "compliance-start-pci" {
			found = true
		}
	}
	if !found {
		t.Error("expected compliance-start-pci recommendation")
	}
}

func TestAnalyze_FailedControls(t *testing.T) {
	recs := Analyze(Signal{
		FailedControls: 5,
	})
	found := false
	for _, r := range recs {
		if r.ID == "compliance-remediate-failed" {
			found = true
		}
	}
	if !found {
		t.Error("expected compliance-remediate-failed recommendation")
	}
}

func TestAnalyze_OverdueReviews(t *testing.T) {
	recs := Analyze(Signal{
		OverdueReviews: 3,
	})
	found := false
	for _, r := range recs {
		if r.ID == "compliance-overdue-reviews" {
			found = true
		}
	}
	if !found {
		t.Error("expected compliance-overdue-reviews recommendation")
	}
}

func TestAnalyze_EncryptionGaps(t *testing.T) {
	recs := Analyze(Signal{
		EncryptionAtRest:    false,
		EncryptionInTransit: false,
	})
	foundRest := false
	foundTransit := false
	for _, r := range recs {
		if r.ID == "compliance-encrypt-at-rest" {
			foundRest = true
		}
		if r.ID == "compliance-encrypt-transit" {
			foundTransit = true
		}
	}
	if !foundRest {
		t.Error("expected compliance-encrypt-at-rest recommendation")
	}
	if !foundTransit {
		t.Error("expected compliance-encrypt-transit recommendation")
	}
}

func TestAnalyze_NoAuditLogging(t *testing.T) {
	recs := Analyze(Signal{
		AuditLoggingEnabled: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "compliance-audit-logging" {
			found = true
		}
	}
	if !found {
		t.Error("expected compliance-audit-logging recommendation")
	}
}

func TestAnalyze_StalledWorkflow(t *testing.T) {
	recs := Analyze(Signal{
		Workflows: []Workflow{
			{
				Standard:     StandardGDPR,
				CurrentPhase: PhaseDiscovery,
				StartTime:    time.Now().Add(-10 * 24 * time.Hour),
			},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "compliance-stalled-gdpr" {
			found = true
		}
	}
	if !found {
		t.Error("expected compliance-stalled-gdpr recommendation")
	}
}

func TestAnalyze_SOC2Recommended(t *testing.T) {
	recs := Analyze(Signal{
		HasGDPR: true,
		HasSOC2: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "compliance-start-soc2" {
			found = true
		}
	}
	if !found {
		t.Error("expected compliance-start-soc2 recommendation")
	}
}

func TestAnalyze_ManyWarnings(t *testing.T) {
	recs := Analyze(Signal{
		WarningControls: 8,
	})
	found := false
	for _, r := range recs {
		if r.ID == "compliance-warnings-review" {
			found = true
		}
	}
	if !found {
		t.Error("expected compliance-warnings-review recommendation")
	}
}