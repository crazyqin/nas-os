package ssdcacheschedule

import (
	"testing"
	"time"
)

func TestAnalyze_NoCacheLargePool(t *testing.T) {
	recs := Analyze(Signal{
		SSDCachePresent: false,
		TotalPoolGB:     1000,
	})
	if len(recs) == 0 {
		t.Fatal("expected recommendation for large pool without cache")
	}
	found := false
	for _, r := range recs {
		if r.ID == "ssd-add-cache" {
			found = true
			if r.Priority != "medium" {
				t.Errorf("expected medium priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected ssd-add-cache recommendation")
	}
}

func TestAnalyze_LowHitRateExpansion(t *testing.T) {
	recs := Analyze(Signal{
		SSDCachePresent: true,
		CacheHitRate:    0.15,
		WorkingSetGB:    500,
		SSDCapacityGB:   200,
	})
	found := false
	for _, r := range recs {
		if r.ID == "ssd-expand-cache" {
			found = true
		}
	}
	if !found {
		t.Error("expected ssd-expand-cache recommendation")
	}
}

func TestAnalyze_HighWearLevel(t *testing.T) {
	recs := Analyze(Signal{
		SSDCachePresent: true,
		SSDWearLevelPct: 85,
	})
	found := false
	for _, r := range recs {
		if r.ID == "ssd-wear-alert" {
			found = true
		}
	}
	if !found {
		t.Error("expected ssd-wear-alert recommendation")
	}
}

func TestAnalyze_HighTemperature(t *testing.T) {
	recs := Analyze(Signal{
		SSDCachePresent:  true,
		SSDTemperatureC:  75,
	})
	found := false
	for _, r := range recs {
		if r.ID == "ssd-temp-throttle" {
			found = true
		}
	}
	if !found {
		t.Error("expected ssd-temp-throttle recommendation")
	}
}

func TestAnalyze_ReadAmplification(t *testing.T) {
	recs := Analyze(Signal{
		SSDCachePresent:   true,
		ReadAmplification: 5.0,
	})
	found := false
	for _, r := range recs {
		if r.ID == "ssd-read-amp" {
			found = true
		}
	}
	if !found {
		t.Error("expected ssd-read-amp recommendation")
	}
}

func TestAnalyze_WriteAmplification(t *testing.T) {
	recs := Analyze(Signal{
		SSDCachePresent:    true,
		CacheTier:          TierReadWrite,
		WriteAmplification: 7.0,
	})
	found := false
	for _, r := range recs {
		if r.ID == "ssd-write-amp" {
			found = true
		}
	}
	if !found {
		t.Error("expected ssd-write-amp recommendation")
	}
}

func TestAnalyze_SmoothingSchedule(t *testing.T) {
	recs := Analyze(Signal{
		SSDCachePresent:       true,
		SmoothingSchedule:     false,
		BackupsRunningAtNight: true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "ssd-smooth-schedule" {
			found = true
			if r.ScheduleAt != "02:00" {
				t.Errorf("expected schedule at 02:00, got %s", r.ScheduleAt)
			}
		}
	}
	if !found {
		t.Error("expected ssd-smooth-schedule recommendation")
	}
}

func TestAnalyze_StaleCache(t *testing.T) {
	recs := Analyze(Signal{
		SSDCachePresent: true,
		LastRefreshAge:  48 * time.Hour,
	})
	found := false
	for _, r := range recs {
		if r.ID == "ssd-refresh-cache" {
			found = true
		}
	}
	if !found {
		t.Error("expected ssd-refresh-cache recommendation")
	}
}

func TestAnalyze_NVMePromoteRW(t *testing.T) {
	recs := Analyze(Signal{
		SSDCachePresent: true,
		HasNVMeSSD:      true,
		CacheTier:       TierReadOnly,
	})
	found := false
	for _, r := range recs {
		if r.ID == "ssd-promote-rw" {
			found = true
		}
	}
	if !found {
		t.Error("expected ssd-promote-rw recommendation")
	}
}

func TestAnalyze_EmptySignal(t *testing.T) {
	recs := Analyze(Signal{})
	if len(recs) != 0 {
		t.Fatalf("expected no recommendations for empty signal, got %d", len(recs))
	}
}

func TestAnalyze_PriorityOrdering(t *testing.T) {
	recs := Analyze(Signal{
		SSDCachePresent:  true,
		SSDWearLevelPct:  85,
		SSDTemperatureC:  75,
		ReadAmplification: 5.0,
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