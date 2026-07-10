package spotlightbridge

import (
	"testing"
	"time"
)

func TestEnableSpotlight(t *testing.T) {
	s := Signal{SpotlightEnabled: false, SMBShareCount: 3, MacosClients: 2}
	recs := Analyze(s)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].ID != "enable-spotlight" {
		t.Errorf("expected enable-spotlight, got %s", recs[0].ID)
	}
	if recs[0].Priority != "critical" {
		t.Errorf("expected critical priority, got %s", recs[0].Priority)
	}
}

func TestExtendSpotlight(t *testing.T) {
	s := Signal{SpotlightEnabled: true, SMBShareCount: 4, SharesWithSpotlight: 2}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "extend-spotlight" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected extend-spotlight recommendation")
	}
}

func TestEnableContentIndexing(t *testing.T) {
	s := Signal{SpotlightEnabled: true, SMBShareCount: 1, SharesWithSpotlight: 1, ContentIndexingEnabled: false}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "enable-content-indexing" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected enable-content-indexing recommendation")
	}
}

func TestExcludeEncryptedShares(t *testing.T) {
	s := Signal{SpotlightEnabled: true, SMBShareCount: 1, SharesWithSpotlight: 1, ContentIndexingEnabled: true, EncryptedSharesExcluded: false}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "exclude-encrypted-shares" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected exclude-encrypted-shares recommendation")
	}
}

func TestOptimizeIndex(t *testing.T) {
	s := Signal{
		SpotlightEnabled:        true,
		SMBShareCount:          1,
		SharesWithSpotlight:    1,
		ContentIndexingEnabled: true,
		EncryptedSharesExcluded: true,
		SearchLatencyMs:        800,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "optimize-index" {
			found = true
			if r.Priority != "medium" {
				t.Errorf("expected medium priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected optimize-index recommendation")
	}
}

func TestLimitIndexSize(t *testing.T) {
	s := Signal{
		SpotlightEnabled:        true,
		SMBShareCount:          1,
		SharesWithSpotlight:    1,
		ContentIndexingEnabled: true,
		EncryptedSharesExcluded: true,
		IndexSizeGB:            150,
		MaxIndexSizeGB:         100,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "limit-index-size" {
			found = true
			if r.Priority != "medium" {
				t.Errorf("expected medium priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected limit-index-size recommendation")
	}
}

func TestIndexCorruption(t *testing.T) {
	s := Signal{
		SpotlightEnabled:         true,
		SMBShareCount:           1,
		SharesWithSpotlight:     1,
		ContentIndexingEnabled:  true,
		EncryptedSharesExcluded: true,
		IndexCorruptionDetected: true,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "rebuild-corrupt-index" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected critical priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected rebuild-corrupt-index recommendation")
	}
}

func TestStaleIndex(t *testing.T) {
	s := Signal{
		SpotlightEnabled:        true,
		SMBShareCount:          1,
		SharesWithSpotlight:    1,
		ContentIndexingEnabled: true,
		EncryptedSharesExcluded: true,
		IndexAge:               48 * time.Hour,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "refresh-stale-index" {
			found = true
			if r.Priority != "medium" {
				t.Errorf("expected medium priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected refresh-stale-index recommendation")
	}
}

func TestPeriodicReindex(t *testing.T) {
	s := Signal{
		SpotlightEnabled:        true,
		SMBShareCount:          1,
		SharesWithSpotlight:    1,
		ContentIndexingEnabled: true,
		EncryptedSharesExcluded: true,
		LastReindexAge:         10 * 24 * time.Hour,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "schedule-periodic-reindex" {
			found = true
			if r.Priority != "low" {
				t.Errorf("expected low priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected schedule-periodic-reindex recommendation")
	}
}

func TestEmptySignal(t *testing.T) {
	s := Signal{}
	recs := Analyze(s)
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations for empty signal, got %d", len(recs))
	}
}

func TestPriorityOrdering(t *testing.T) {
	s := Signal{
		SpotlightEnabled:         true,
		SMBShareCount:           4,
		SharesWithSpotlight:     2,
		MacosClients:            3,
		ContentIndexingEnabled:  false,
		EncryptedSharesExcluded: false,
		SearchLatencyMs:         600,
		IndexSizeGB:             150,
		MaxIndexSizeGB:          100,
		IndexCorruptionDetected: true,
		IndexAge:                48 * time.Hour,
		LastReindexAge:          8 * 24 * time.Hour,
	}
	recs := Analyze(s)
	if len(recs) < 2 {
		t.Fatalf("expected multiple recommendations, got %d", len(recs))
	}
	for i := 1; i < len(recs); i++ {
		if priorityRank(recs[i-1].Priority) > priorityRank(recs[i].Priority) {
			t.Errorf("recommendations not sorted by priority at index %d: %s before %s",
				i, recs[i-1].Priority, recs[i].Priority)
		}
	}
}