package fworchestrator

import (
	"testing"
	"time"
)

func TestAnalyze_NoUpdate_Stale(t *testing.T) {
	s := Signal{
		UpdateAvailable: false,
		LastUpdateTime:  time.Now().Add(-40 * 24 * time.Hour),
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "fw-check-updates" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected fw-check-updates recommendation")
	}
}

func TestAnalyze_CriticalUpdate(t *testing.T) {
	s := Signal{
		UpdateAvailable:  true,
		IsCriticalUpdate: true,
		DiskHealthOK:     true,
		FreeSpaceMB:      5000,
		HasBackup:        true,
		MaintenanceWindow: true,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "fw-critical-apply" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected critical priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Fatal("expected fw-critical-apply recommendation")
	}
}

func TestAnalyze_DiskHealthFail(t *testing.T) {
	s := Signal{
		UpdateAvailable: true,
		DiskHealthOK:   false,
		FreeSpaceMB:    5000,
		HasBackup:      true,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "fw-disk-health" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected fw-disk-health recommendation")
	}
}

func TestAnalyze_NoBackup(t *testing.T) {
	s := Signal{
		UpdateAvailable: true,
		DiskHealthOK:   true,
		FreeSpaceMB:    5000,
		HasBackup:      false,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "fw-backup" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected fw-backup recommendation")
	}
}

func TestAnalyze_LowSpace(t *testing.T) {
	s := Signal{
		UpdateAvailable: true,
		DiskHealthOK:   true,
		FreeSpaceMB:    500,
		HasBackup:      true,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "fw-space" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected fw-space recommendation")
	}
}

func TestAnalyze_FailedUpdates(t *testing.T) {
	s := Signal{
		UpdateAvailable: true,
		DiskHealthOK:   true,
		FreeSpaceMB:    5000,
		HasBackup:      true,
		FailedUpdates:  2,
	}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "fw-investigate-failures" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected fw-investigate-failures recommendation")
	}
}

func TestPlanUpdate(t *testing.T) {
	plan := PlanUpdate("v3.14.0", "v3.15.0")
	if len(plan) != 8 {
		t.Fatalf("expected 8 phases, got %d", len(plan))
	}
	if plan[0].Phase != PhasePreCheck {
		t.Error("expected pre-check as first phase")
	}
	if plan[len(plan)-1].Phase != PhaseVerify {
		t.Error("expected verify as last phase")
	}
	if plan[0].CurrentVersion != "v3.14.0" {
		t.Error("expected current version in plan")
	}
	if plan[0].TargetVersion != "v3.15.0" {
		t.Error("expected target version in plan")
	}
}

func TestAnalyze_NoUpdateNoStale(t *testing.T) {
	s := Signal{
		UpdateAvailable: false,
		LastUpdateTime:  time.Now().Add(-5 * 24 * time.Hour),
	}
	recs := Analyze(s)
	if len(recs) != 0 {
		t.Fatalf("expected no recommendations, got %d", len(recs))
	}
}
