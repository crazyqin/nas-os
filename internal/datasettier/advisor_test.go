package datasettier

import (
	"fmt"
	"testing"
	"time"
)

func TestAnalyze_EnableAutoTier(t *testing.T) {
	recs := Analyze(Signal{
		Pool: Pool{
			HasOpenZFS24: true,
			Datasets:     make([]Dataset, 15),
		},
		AutoTierEnabled: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "tier-enable-auto" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected tier-enable-auto recommendation")
	}
}

func TestAnalyze_PromoteHotDatasets(t *testing.T) {
	recs := Analyze(Signal{
		HotDatasetsOnHDD: 5,
		Pool: Pool{
			FlashCapacityGB: 1000,
			FlashUsedGB:     200,
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "tier-promote-hot" {
			found = true
		}
	}
	if !found {
		t.Error("expected tier-promote-hot recommendation")
	}
}

func TestAnalyze_DemoteColdDatasets(t *testing.T) {
	recs := Analyze(Signal{
		ColdDatasetsOnFlash: 3,
	})
	found := false
	for _, r := range recs {
		if r.ID == "tier-demote-cold" {
			found = true
		}
	}
	if !found {
		t.Error("expected tier-demote-cold recommendation")
	}
}

func TestAnalyze_FlashNearFull(t *testing.T) {
	recs := Analyze(Signal{
		Pool: Pool{
			FlashCapacityGB: 1000,
			FlashUsedGB:     900,
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "tier-flash-near-full" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected tier-flash-near-full recommendation")
	}
}

func TestAnalyze_TierOverflowRisk(t *testing.T) {
	recs := Analyze(Signal{
		TierOverflowRisk: true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "tier-overflow-risk" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected critical priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected tier-overflow-risk recommendation")
	}
}

func TestAnalyze_EnablePredictive(t *testing.T) {
	recs := Analyze(Signal{
		Pool: Pool{
			HasOpenZFS24: true,
		},
		PredictiveEnabled: false,
		FlashHitRate:      0.3,
	})
	found := false
	for _, r := range recs {
		if r.ID == "tier-enable-predictive" {
			found = true
		}
	}
	if !found {
		t.Error("expected tier-enable-predictive recommendation")
	}
}

func TestAnalyze_HDDOverload(t *testing.T) {
	recs := Analyze(Signal{
		HDDBusyPct:    80,
		FlashHitRate:  0.3,
	})
	found := false
	for _, r := range recs {
		if r.ID == "tier-hdd-overload" {
			found = true
		}
	}
	if !found {
		t.Error("expected tier-hdd-overload recommendation")
	}
}

func TestAnalyze_StaleTieringRun(t *testing.T) {
	recs := Analyze(Signal{
		LastTieringRun: time.Now().Add(-48 * time.Hour),
	})
	found := false
	for _, r := range recs {
		if r.ID == "tier-stale-run" {
			found = true
		}
	}
	if !found {
		t.Error("expected tier-stale-run recommendation")
	}
}

func TestAnalyze_EnableArchive(t *testing.T) {
	datasets := []Dataset{
		{Name: "old1", LastAccessed: time.Now().Add(-120 * 24 * time.Hour)},
		{Name: "old2", LastAccessed: time.Now().Add(-100 * 24 * time.Hour)},
		{Name: "old3", LastAccessed: time.Now().Add(-95 * 24 * time.Hour)},
		{Name: "new1", LastAccessed: time.Now().Add(-1 * 24 * time.Hour)},
	}
	recs := Analyze(Signal{
		Pool: Pool{
			ArchiveEnabled: false,
			Datasets:       datasets,
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "tier-enable-archive" {
			found = true
		}
	}
	if !found {
		t.Error("expected tier-enable-archive recommendation")
	}
}

func TestAnalyze_PromoteIndividualDataset(t *testing.T) {
	datasets := []Dataset{
		{Name: "hot-db", CurrentTier: TierHDD, AccessFrequency: 150},
	}
	recs := Analyze(Signal{
		Pool: Pool{
			Datasets: datasets,
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "tier-promote-hot-db" {
			found = true
			if r.FromTier != TierHDD || r.ToTier != TierFlash {
				t.Errorf("expected HDD->Flash, got %s->%s", r.FromTier, r.ToTier)
			}
		}
	}
	if !found {
		t.Error("expected tier-promote-hot-db recommendation")
	}
}

func TestAnalyze_DemoteIndividualDataset(t *testing.T) {
	datasets := []Dataset{
		{Name: "cold-archive", CurrentTier: TierFlash, AccessFrequency: 0.1},
	}
	recs := Analyze(Signal{
		Pool: Pool{
			Datasets: datasets,
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "tier-demote-cold-archive" {
			found = true
		}
	}
	if !found {
		t.Error(fmt.Errorf("expected tier-demote-cold-archive recommendation"))
	}
}

func TestAnalyze_PinnedDatasetSkipped(t *testing.T) {
	datasets := []Dataset{
		{Name: "pinned-hot", CurrentTier: TierHDD, AccessFrequency: 200, Pinned: true},
	}
	recs := Analyze(Signal{
		Pool: Pool{
			Datasets: datasets,
		},
	})
	for _, r := range recs {
		if r.Dataset == "pinned-hot" {
			t.Error("pinned dataset should not generate recommendations")
		}
	}
}