package scrubadvisor

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateFindsOverdueScrubAndIntegrityRisks(t *testing.T) {
	advisor := New().WithNow(func() time.Time { return time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC) })
	report := advisor.Generate(Signal{
		PoolName:              "tank",
		DaysSinceLastScrub:    120,
		ParityOrMirrorEnabled: false,
		SMARTWarnings:         1,
		ChecksumErrors:        2,
		SnapshotCount:         0,
		FreePercent:           9,
		RecentPowerLosses:     1,
		CriticalShares:        2,
	})

	if report.ScrubFreshness != "overdue" {
		t.Fatalf("freshness = %s, want overdue", report.ScrubFreshness)
	}
	wantIDs := map[string]bool{
		"investigate-checksum-errors":        false,
		"replace-risky-drives":               false,
		"schedule-pool-scrub":                false,
		"add-redundancy-for-critical-shares": false,
		"enable-integrity-snapshots":         false,
		"reserve-scrub-headroom":             false,
		"stabilize-power-before-scrub":       false,
	}
	for _, rec := range report.Recommendations {
		if _, ok := wantIDs[rec.ID]; ok {
			wantIDs[rec.ID] = true
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Fatalf("missing recommendation %s in %#v", id, report.Recommendations)
		}
	}
	if report.Recommendations[0].Priority != "critical" {
		t.Fatalf("first priority = %s, want critical", report.Recommendations[0].Priority)
	}
	if report.IntegrityScore >= 60 {
		t.Fatalf("score = %d, want < 60 for severe risks", report.IntegrityScore)
	}
}

func TestGenerateHealthyPoolHasHighScore(t *testing.T) {
	report := New().Generate(Signal{
		PoolName:              "tank",
		DaysSinceLastScrub:    7,
		ParityOrMirrorEnabled: true,
		SnapshotCount:         48,
		ImmutableSnapshots:    4,
		FreePercent:           42,
		CriticalShares:        3,
	})
	if report.ScrubFreshness != "fresh" {
		t.Fatalf("freshness = %s, want fresh", report.ScrubFreshness)
	}
	if report.IntegrityScore < 95 {
		t.Fatalf("score = %d, want >= 95", report.IntegrityScore)
	}
	if len(report.Recommendations) != 0 {
		t.Fatalf("recommendations = %#v, want none", report.Recommendations)
	}
}

func TestSummarizeActions(t *testing.T) {
	summary := SummarizeActions([]Recommendation{{Title: "安排巡检", Actions: []string{"创建计划"}}})
	if !strings.Contains(summary, "安排巡检: 创建计划") {
		t.Fatalf("summary = %q", summary)
	}
}
