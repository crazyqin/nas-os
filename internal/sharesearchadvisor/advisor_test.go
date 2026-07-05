package sharesearchadvisor

import (
	"strings"
	"testing"
)

func TestGenerateRecommendsWebShareSearchAndSecurity(t *testing.T) {
	report := New().Generate(Signal{
		TotalFiles:          5000,
		IndexedFiles:        1200,
		SharedLinks:         4,
		ExternalShares:      2,
		PhotoFiles:          500,
		VideoFiles:          120,
		MobileAccessEnabled: true,
		SearchEnabled:       true,
	})

	if report.CoveragePercent != 24 {
		t.Fatalf("coverage = %d, want 24", report.CoveragePercent)
	}
	wantIDs := map[string]bool{
		"enable-webshare":         false,
		"expand-search-index":     false,
		"secure-external-share":   false,
		"mobile-media-experience": false,
		"snapshot-before-share":   false,
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
	if report.Recommendations[0].Priority != "high" {
		t.Fatalf("first priority = %s, want high", report.Recommendations[0].Priority)
	}
}

func TestGenerateHighReadinessWhenProtectedAndIndexed(t *testing.T) {
	report := New().Generate(Signal{
		TotalFiles:          10000,
		IndexedFiles:        9500,
		WebShareEnabled:     true,
		SearchEnabled:       true,
		MobileAccessEnabled: true,
		PasskeyEnabled:      true,
		SnapshotCount:       24,
	})
	if report.CoveragePercent != 95 {
		t.Fatalf("coverage = %d, want 95", report.CoveragePercent)
	}
	if report.ReadinessScore < 90 {
		t.Fatalf("readiness = %d, want >= 90", report.ReadinessScore)
	}
	if len(report.Recommendations) != 0 {
		t.Fatalf("recommendations = %#v, want none", report.Recommendations)
	}
}

func TestSummarizeActions(t *testing.T) {
	summary := SummarizeActions([]Recommendation{{Title: "开启搜索", Actions: []string{"建立索引"}}})
	if !strings.Contains(summary, "开启搜索: 建立索引") {
		t.Fatalf("summary = %q", summary)
	}
}
