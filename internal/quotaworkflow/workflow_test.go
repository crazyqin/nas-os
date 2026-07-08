package quotaworkflow

import (
	"testing"
)

func TestAnalyze_NoGlobalPolicy(t *testing.T) {
	s := Signal{TotalShares: 5, HasGlobalPolicy: false}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "quota-global-policy" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected quota-global-policy recommendation")
	}
}

func TestAnalyze_OverQuota(t *testing.T) {
	s := Signal{TotalShares: 3, OverQuotaShares: 2}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "quota-over-limit" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected quota-over-limit recommendation")
	}
}

func TestAnalyze_NearQuota(t *testing.T) {
	s := Signal{TotalShares: 3, NearQuotaShares: 1}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "quota-near-limit" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected quota-near-limit recommendation")
	}
}

func TestAnalyze_UnprotectedShares(t *testing.T) {
	s := Signal{TotalShares: 5, SharesWithQuota: 2}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "quota-unprotected" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected quota-unprotected recommendation")
	}
}

func TestAnalyze_CriticalQuota(t *testing.T) {
	s := Signal{
		TotalShares:     2,
		SharesWithQuota: 2,
		HasGlobalPolicy: true,
		QuotaList: []Quota{
			{ShareName: "important", LimitGB: 100, UsedGB: 97, CriticalPct: 95, WarningPct: 80},
		},
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "quota-critical-important" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected quota-critical-important recommendation")
	}
}

func TestAnalyze_PoolLow(t *testing.T) {
	s := Signal{
		TotalShares:     2,
		SharesWithQuota: 2,
		HasGlobalPolicy: true,
		PoolFreeGB:      50,
		PoolTotalGB:     1000,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "quota-pool-low" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected quota-pool-low recommendation")
	}
}

func TestSuggestQuota(t *testing.T) {
	q := SuggestQuota("test-share", 500, 2000)
	if q.ShareName != "test-share" {
		t.Error("wrong share name")
	}
	if q.LimitGB != 500 {
		t.Errorf("expected 500, got %f", q.LimitGB)
	}
	if q.State != StateSoft {
		t.Error("expected soft state")
	}
	if !q.NotifyUser {
		t.Error("expected notify user")
	}
}

func TestSuggestQuota_Capped(t *testing.T) {
	q := SuggestQuota("big-share", 2000, 1000)
	if q.LimitGB != 800 {
		t.Errorf("expected 800 (80%% of 1000), got %f", q.LimitGB)
	}
}
