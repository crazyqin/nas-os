package snapscheduler

import (
	"testing"
	"time"
)

func TestAnalyze_NoPolicy(t *testing.T) {
	s := Signal{TotalShares: 5, SharesWithoutPolicy: 5}
	recs := Analyze(s)
	if len(recs) == 0 {
		t.Fatal("expected recommendations for unprotected shares")
	}
	found := false
	for _, r := range recs {
		if r.ID == "snap-add-policy" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Fatal("expected snap-add-policy recommendation")
	}
}

func TestAnalyze_StaleSnapshot(t *testing.T) {
	s := Signal{
		TotalShares:      3,
		HasDailySnapshot: true,
		LastSnapshotAge:  100 * time.Hour,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "snap-stale" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected snap-stale recommendation")
	}
}

func TestAnalyze_ImmutableMissing(t *testing.T) {
	s := Signal{TotalShares: 2, HasDailySnapshot: true}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "snap-immutable" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected snap-immutable recommendation")
	}
}

func TestSuggestPolicy_Critical(t *testing.T) {
	p := SuggestPolicy("important-share", true)
	if p.Frequency != "hourly" {
		t.Errorf("expected hourly for critical share, got %s", p.Frequency)
	}
	if !p.Immutable {
		t.Error("expected immutable for critical share")
	}
}

func TestSuggestPolicy_Normal(t *testing.T) {
	p := SuggestPolicy("normal-share", false)
	if p.Frequency != "daily" {
		t.Errorf("expected daily for normal share, got %s", p.Frequency)
	}
	if p.Immutable {
		t.Error("expected non-immutable for normal share")
	}
}

func TestAnalyze_ReplicationGap(t *testing.T) {
	s := Signal{
		TotalShares:        4,
		HasDailySnapshot:   true,
		ReplicationEnabled: false,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "snap-replicate" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected snap-replicate recommendation")
	}
}

func TestPriorityRank(t *testing.T) {
	if priorityRank("critical") != 0 {
		t.Error("critical should rank 0")
	}
	if priorityRank("low") != 3 {
		t.Error("low should rank 3")
	}
}
