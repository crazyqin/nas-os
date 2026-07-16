package retentionpolicy

import (
	"testing"
	"time"
)

func TestAnalyze_NoWORM(t *testing.T) {
	s := Signal{TotalShares: 3, SharesWithPolicy: 3, WORMEnabled: false}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "retention-worm-enable" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected retention-worm-enable recommendation")
	}
}

func TestAnalyze_MissingPolicy(t *testing.T) {
	s := Signal{TotalShares: 5, SharesWithPolicy: 2, WORMEnabled: true}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "retention-missing-policy" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected retention-missing-policy recommendation")
	}
}

func TestAnalyze_ExpiredData(t *testing.T) {
	s := Signal{TotalShares: 2, SharesWithPolicy: 2, WORMEnabled: true, ExpiredDataGB: 50}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "retention-expired" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected retention-expired recommendation")
	}
}

func TestAnalyze_LegalHoldNoWORM(t *testing.T) {
	s := Signal{TotalShares: 3, SharesWithPolicy: 3, WORMEnabled: false, LegalHoldShares: 1}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "retention-hold-no-worm" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected critical priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Fatal("expected retention-hold-no-worm recommendation")
	}
}

func TestAnalyze_AuditOverdue(t *testing.T) {
	s := Signal{
		TotalShares:        2,
		SharesWithPolicy:   2,
		WORMEnabled:         true,
		ComplianceAuditDue:  true,
		LastAuditDate:       time.Now().Add(-120 * 24 * time.Hour),
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "retention-audit" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected retention-audit recommendation")
	}
}

func TestAnalyze_LegalHold(t *testing.T) {
	s := Signal{
		TotalShares:      2,
		SharesWithPolicy: 2,
		WORMEnabled:      true,
		LegalHoldShares:  1,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "retention-legal-hold" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected retention-legal-hold recommendation")
	}
}

func TestSuggestPolicy_Financial(t *testing.T) {
	p := SuggestPolicy("financial")
	if p.Category != "financial" {
		t.Error("expected financial category")
	}
	if !p.Immutable {
		t.Error("expected immutable for financial")
	}
	if p.AfterExpiry != "archive" {
		t.Error("expected archive after expiry")
	}
}

func TestSuggestPolicy_Legal(t *testing.T) {
	p := SuggestPolicy("legal")
	if p.Category != "legal" {
		t.Error("expected legal category")
	}
	if !p.Immutable {
		t.Error("expected immutable for legal")
	}
}

func TestSuggestPolicy_General(t *testing.T) {
	p := SuggestPolicy("general")
	if p.Immutable {
		t.Error("expected non-immutable for general")
	}
}

func TestAnalyze_NoIssues(t *testing.T) {
	s := Signal{
		TotalShares:      2,
		SharesWithPolicy: 2,
		WORMEnabled:      true,
	}
	recs := Analyze(s)
	for _, r := range recs {
		if r.ID == "retention-worm-enable" || r.ID == "retention-missing-policy" {
			t.Fatal("should not have these recommendations when everything is ok")
		}
	}
}
