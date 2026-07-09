package datasovereigntyaudit

import (
	"testing"
)

func TestAnalyze_EncryptPII(t *testing.T) {
	recs := Analyze(Signal{
		PIIShares:      5,
		UnencryptedPII:  2,
	})
	found := false
	for _, r := range recs {
		if r.ID == "sovereign-encrypt-pii" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected sovereign-encrypt-pii recommendation")
	}
}

func TestAnalyze_CrossBorderRep(t *testing.T) {
	recs := Analyze(Signal{
		CrossBorderRepCount: 3,
		ActiveRegulations:    []Regulation{RegGDPR},
	})
	found := false
	for _, r := range recs {
		if r.ID == "sovereign-cross-border" {
			found = true
		}
	}
	if !found {
		t.Error("expected sovereign-cross-border recommendation")
	}
}

func TestAnalyze_CrossBorderPIPL(t *testing.T) {
	recs := Analyze(Signal{
		CrossBorderRepCount: 1,
		ActiveRegulations:    []Regulation{RegPIPL},
	})
	found := false
	for _, r := range recs {
		if r.ID == "sovereign-cross-border" {
			found = true
		}
	}
	if !found {
		t.Error("expected sovereign-cross-border recommendation for PIPL")
	}
}

func TestAnalyze_AccessLogs(t *testing.T) {
	recs := Analyze(Signal{
		NoAccessLogShares: 3,
	})
	found := false
	for _, r := range recs {
		if r.ID == "sovereign-access-logs" {
			found = true
		}
	}
	if !found {
		t.Error("expected sovereign-access-logs recommendation")
	}
}

func TestAnalyze_RetentionPII(t *testing.T) {
	recs := Analyze(Signal{
		PIIShares:        5,
		NoRetentionShares: 2,
	})
	found := false
	for _, r := range recs {
		if r.ID == "sovereign-retention-pii" {
			found = true
		}
	}
	if !found {
		t.Error("expected sovereign-retention-pii recommendation")
	}
}

func TestAnalyze_NoDataInventory(t *testing.T) {
	recs := Analyze(Signal{
		HasDataInventory: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "sovereign-data-inventory" {
			found = true
		}
	}
	if !found {
		t.Error("expected sovereign-data-inventory recommendation")
	}
}

func TestAnalyze_NoDPA(t *testing.T) {
	recs := Analyze(Signal{
		HasDPAProcessors: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "sovereign-dpa-processors" {
			found = true
		}
	}
	if !found {
		t.Error("expected sovereign-dpa-processors recommendation")
	}
}

func TestAnalyze_StaleAudit(t *testing.T) {
	recs := Analyze(Signal{
		StaleAuditShares: 4,
	})
	found := false
	for _, r := range recs {
		if r.ID == "sovereign-stale-audit" {
			found = true
		}
	}
	if !found {
		t.Error("expected sovereign-stale-audit recommendation")
	}
}

func TestAnalyze_UnknownRegionPII(t *testing.T) {
	recs := Analyze(Signal{
		Shares: []ShareSignal{
			{Name: "share1", HasPII: true, Region: RegionUnknown},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "sovereign-region-unknown-share1" {
			found = true
		}
	}
	if !found {
		t.Error("expected sovereign-region-unknown-share1 recommendation")
	}
}

func TestAnalyze_RestrictedUnencrypted(t *testing.T) {
	recs := Analyze(Signal{
		Shares: []ShareSignal{
			{Name: "secret", DataClass: ClassRestricted, HasEncryption: false},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "sovereign-restricted-encrypt-secret" {
			found = true
		}
	}
	if !found {
		t.Error("expected sovereign-restricted-encrypt-secret recommendation")
	}
}

func TestAnalyze_HIPAAInventory(t *testing.T) {
	recs := Analyze(Signal{
		ActiveRegulations: []Regulation{RegHIPAA},
		HasDataInventory:   false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "sovereign-hipaa-inventory" {
			found = true
		}
	}
	if !found {
		t.Error("expected sovereign-hipaa-inventory recommendation")
	}
}

func TestAnalyze_EmptySignal(t *testing.T) {
	recs := Analyze(Signal{})
	// Empty signal still flags missing data inventory and DPAs as baseline compliance gaps
	if len(recs) != 2 {
		t.Fatalf("expected 2 baseline compliance recommendations for empty signal, got %d", len(recs))
	}
}

func TestAnalyze_PriorityOrdering(t *testing.T) {
	recs := Analyze(Signal{
		PIIShares:           5,
		UnencryptedPII:      2,
		NoAccessLogShares:   3,
		CrossBorderRepCount: 1,
		ActiveRegulations:   []Regulation{RegGDPR},
	})
	if len(recs) < 2 {
		t.Fatal("expected multiple recommendations")
	}
	for i := 0; i < len(recs)-1; i++ {
		if priorityRank(recs[i].Priority) > priorityRank(recs[i+1].Priority) {
			t.Errorf("recommendations not sorted by priority at index %d", i)
		}
	}
}