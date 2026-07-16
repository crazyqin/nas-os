package backuphealthadvisor

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateFindsProtectionGapsAndRecoveryRisks(t *testing.T) {
	advisor := New().WithNow(func() time.Time { return time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC) })
	report := advisor.Generate(Signal{
		ProtectedDevices: 1,
		TotalDevices:     4,
		LastBackupHours:  48,
		FailedBackups:    2,
		CriticalShares:   3,
		RansomwareAlerts: 1,
	})

	if report.ProtectionPercent != 25 {
		t.Fatalf("protection = %d, want 25", report.ProtectionPercent)
	}
	wantIDs := map[string]bool{
		"expand-device-protection":      false,
		"repair-backup-failures":        false,
		"optimize-backup-efficiency":    false,
		"enable-share-snapshots":        false,
		"add-immutable-recovery-points": false,
		"schedule-restore-drill":        false,
		"prepare-disaster-recovery":     false,
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

func TestGenerateHighReadinessWhenProtectedAndTested(t *testing.T) {
	report := New().Generate(Signal{
		ProtectedDevices:       6,
		TotalDevices:           6,
		LastBackupHours:        6,
		IncrementalEnabled:     true,
		DedupEnabled:           true,
		SnapshotCount:          48,
		ImmutableSnapshots:     12,
		OffsiteCopies:          1,
		RestoreTestsLast30Days: 2,
		RecoveryMediaCreated:   true,
		CriticalShares:         3,
	})
	if report.ProtectionPercent != 100 {
		t.Fatalf("protection = %d, want 100", report.ProtectionPercent)
	}
	if report.ReadinessScore < 90 {
		t.Fatalf("readiness = %d, want >= 90", report.ReadinessScore)
	}
	if len(report.Recommendations) != 0 {
		t.Fatalf("recommendations = %#v, want none", report.Recommendations)
	}
}

func TestSummarizeActions(t *testing.T) {
	summary := SummarizeActions([]Recommendation{{Title: "恢复演练", Actions: []string{"抽样试恢复"}}})
	if !strings.Contains(summary, "恢复演练: 抽样试恢复") {
		t.Fatalf("summary = %q", summary)
	}
}
