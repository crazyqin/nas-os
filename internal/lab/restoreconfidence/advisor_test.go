package restoreconfidence

import (
	"testing"
	"time"
)

func TestAnalyze_NoDrill90Days(t *testing.T) {
	recs := Analyze(Signal{})
	found := false
	for _, r := range recs {
		if r.ID == "restore-drill-overdue" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected critical, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected restore-drill-overdue recommendation")
	}
}

func TestAnalyze_NoDrill30Days(t *testing.T) {
	recs := Analyze(Signal{
		HasDrillIn90Days: true,
		HasDrillIn30Days: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "restore-drill-monthly" {
			found = true
		}
	}
	if !found {
		t.Error("expected restore-drill-monthly recommendation")
	}
}

func TestAnalyze_LastDrillFailed(t *testing.T) {
	recs := Analyze(Signal{
		LastDrillDate:    time.Now().Add(-7 * 24 * time.Hour),
		LastDrillSuccess: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "restore-drill-failed" {
			found = true
		}
	}
	if !found {
		t.Error("expected restore-drill-failed recommendation")
	}
}

func TestAnalyze_RTOExceeded(t *testing.T) {
	recs := Analyze(Signal{
		HasDrillIn30Days: true,
		HasDrillIn90Days: true,
		LastDrillSuccess: true,
		HasImmutableBackup: true,
		HasOffsiteReplica: true,
		RecoveryTargets: []RecoveryTarget{
			{DatasetName: "critical", TargetRTOMinutes: 30, ActualRTOMinutes: 120},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "restore-rto-exceeded-critical" {
			found = true
		}
	}
	if !found {
		t.Error("expected restore-rto-exceeded-critical recommendation")
	}
}

func TestAnalyze_NoImmutable(t *testing.T) {
	recs := Analyze(Signal{
		HasDrillIn30Days: true,
		HasDrillIn90Days: true,
		LastDrillSuccess: true,
		HasImmutableBackup: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "restore-immutable-missing" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected critical, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected restore-immutable-missing recommendation")
	}
}

func TestAnalyze_NoOffsiteReplica(t *testing.T) {
	recs := Analyze(Signal{
		HasDrillIn30Days: true,
		HasDrillIn90Days: true,
		LastDrillSuccess: true,
		HasImmutableBackup: true,
		HasOffsiteReplica: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "restore-offsite-missing" {
			found = true
		}
	}
	if !found {
		t.Error("expected restore-offsite-missing recommendation")
	}
}

func TestAnalyze_NoTFA(t *testing.T) {
	recs := Analyze(Signal{
		HasDrillIn30Days: true,
		HasDrillIn90Days: true,
		LastDrillSuccess: true,
		HasImmutableBackup: true,
		HasOffsiteReplica: true,
		TFAEnabled: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "restore-no-tfa" {
			found = true
		}
	}
	if !found {
		t.Error("expected restore-no-tfa recommendation")
	}
}

func TestAnalyze_VeryLowConfidence(t *testing.T) {
	recs := Analyze(Signal{})
	found := false
	for _, r := range recs {
		if r.ID == "restore-confidence-very-low" {
			found = true
			if r.Confidence != ConfidenceVeryLow {
				t.Errorf("expected very_low, got %s", r.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected restore-confidence-very-low recommendation")
	}
}

func TestAnalyze_HighConfidence(t *testing.T) {
	recs := Analyze(Signal{
		HasDrillIn30Days:   true,
		HasDrillIn90Days:   true,
		LastDrillSuccess:   true,
		HasImmutableBackup:  true,
		HasOffsiteReplica:   true,
		HasRansomwareDetection: true,
		ScanEnabled:         true,
		TFAEnabled:          true,
	})
	for _, r := range recs {
		if r.ID == "restore-confidence-very-low" || r.ID == "restore-confidence-low" {
			t.Errorf("should not have low confidence with all checks passing: %s", r.ID)
		}
	}
}

func TestAnalyze_NoRollbackPlan(t *testing.T) {
	recs := Analyze(Signal{
		HasDrillIn30Days: true,
		HasDrillIn90Days: true,
		LastDrillSuccess: true,
		HasImmutableBackup: true,
		HasOffsiteReplica: true,
		HasRansomwareDetection: true,
		ScanEnabled: true,
		TFAEnabled: true,
		HasRollbackPlan: false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "restore-no-rollback-plan" {
			found = true
		}
	}
	if !found {
		t.Error("expected restore-no-rollback-plan recommendation")
	}
}