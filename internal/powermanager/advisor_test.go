package powermanager

import (
	"testing"
	"time"
)

func TestAnalyze_EnterIdleLongIdle(t *testing.T) {
	recs := Analyze(Signal{
		CurrentMode:  ModeActive,
		IdleSince:    3 * time.Hour,
		ActiveUsers:  0,
		RunningTasks: 0,
	})
	found := false
	for _, r := range recs {
		if r.ID == "pm-enter-idle" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected pm-enter-idle recommendation")
	}
}

func TestAnalyze_EnterStandbyOvernight(t *testing.T) {
	recs := Analyze(Signal{
		IdleSince:           7 * time.Hour,
		ActiveUsers:         0,
		RunningTasks:        0,
		NextBackupScheduled: time.Now().Add(3 * time.Hour),
	})
	found := false
	for _, r := range recs {
		if r.ID == "pm-enter-standby" {
			found = true
		}
	}
	if !found {
		t.Error("expected pm-enter-standby recommendation")
	}
}

func TestAnalyze_NoStandbyCloseBackup(t *testing.T) {
	recs := Analyze(Signal{
		IdleSince:           7 * time.Hour,
		ActiveUsers:         0,
		RunningTasks:        0,
		NextBackupScheduled: time.Now().Add(20 * time.Minute),
	})
	found := false
	for _, r := range recs {
		if r.ID == "pm-enter-standby" {
			found = true
		}
	}
	if found {
		t.Error("should not recommend standby when backup is close")
	}
}

func TestAnalyze_DiskSpinDown(t *testing.T) {
	recs := Analyze(Signal{
		DiskSpinPolicy: SpinNever,
		HasSSDCache:   true,
		IdleSince:     45 * time.Minute,
	})
	found := false
	for _, r := range recs {
		if r.ID == "pm-disk-spin-down" {
			found = true
		}
	}
	if !found {
		t.Error("expected pm-disk-spin-down recommendation")
	}
}

func TestAnalyze_HighPowerIdle(t *testing.T) {
	recs := Analyze(Signal{
		CurrentMode:       ModeActive,
		IdleSince:         3 * time.Hour,
		ActiveUsers:       0,
		RunningTasks:      0,
		PowerConsumptionW: 60,
		IdlePowerW:        15,
	})
	found := false
	for _, r := range recs {
		if r.ID == "pm-high-power-idle" {
			found = true
		}
	}
	if !found {
		t.Error("expected pm-high-power-idle recommendation")
	}
}

func TestAnalyze_NightlySchedule(t *testing.T) {
	recs := Analyze(Signal{
		NightlySchedule:  false,
		DailyActiveHours: 22,
	})
	found := false
	for _, r := range recs {
		if r.ID == "pm-nightly-schedule" {
			found = true
		}
	}
	if !found {
		t.Error("expected pm-nightly-schedule recommendation")
	}
}

func TestAnalyze_SolarAlign(t *testing.T) {
	recs := Analyze(Signal{
		HasSolar:        true,
		SolarPeakHours:  "11:00-14:00",
	})
	found := false
	for _, r := range recs {
		if r.ID == "pm-solar-align" {
			found = true
		}
	}
	if !found {
		t.Error("expected pm-solar-align recommendation")
	}
}

func TestAnalyze_EnableWoL(t *testing.T) {
	recs := Analyze(Signal{
		CurrentMode: ModeStandby,
		WakeOnLAN:  false,
	})
	found := false
	for _, r := range recs {
		if r.ID == "pm-enable-wol" {
			found = true
		}
	}
	if !found {
		t.Error("expected pm-enable-wol recommendation")
	}
}

func TestAnalyze_WakeForTasks(t *testing.T) {
	recs := Analyze(Signal{
		CurrentMode:  ModeStandby,
		RunningTasks: 2,
	})
	found := false
	for _, r := range recs {
		if r.ID == "pm-wake-for-tasks" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high priority, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected pm-wake-for-tasks recommendation")
	}
}

func TestAnalyze_WakeForUsers(t *testing.T) {
	recs := Analyze(Signal{
		CurrentMode: ModeSuspend,
		ActiveUsers: 2,
	})
	found := false
	for _, r := range recs {
		if r.ID == "pm-wake-for-users" {
			found = true
		}
	}
	if !found {
		t.Error("expected pm-wake-for-users recommendation")
	}
}

func TestAnalyze_ShortSpinTimer(t *testing.T) {
	recs := Analyze(Signal{
		HasSSDCache:      true,
		DiskSpinPolicy:   SpinAfter10,
		IdleSince:        5 * time.Minute,
	})
	found := false
	for _, r := range recs {
		if r.ID == "pm-short-spin-timer" {
			found = true
		}
	}
	if !found {
		t.Error("expected pm-short-spin-timer recommendation")
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
		CurrentMode:       ModeActive,
		IdleSince:         3 * time.Hour,
		ActiveUsers:       0,
		RunningTasks:      0,
		PowerConsumptionW: 60,
		IdlePowerW:        15,
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