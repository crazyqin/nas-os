package energyaware

import (
	"testing"
	"time"
)

func TestDeviceRegistration(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterDevice(&DevicePower{
		DeviceID: "disk1",
		Name:     "HDD-1",
		Type:     "disk",
		State:    StateActive,
		Watts:    8.0,
	})
	
	devices := mgr.GetDevices()
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].DeviceID != "disk1" {
		t.Errorf("expected device disk1, got %s", devices[0].DeviceID)
	}
}

func TestStateUpdate(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterDevice(&DevicePower{
		DeviceID: "disk1",
		Name:     "HDD-1",
		Type:     "disk",
		State:    StateActive,
		Watts:    8.0,
	})
	
	mgr.UpdateDeviceState("disk1", StateIdle, 2.0)
	devices := mgr.GetDevices()
	if devices[0].State != StateIdle {
		t.Errorf("expected state idle, got %s", devices[0].State)
	}
	if devices[0].Watts != 2.0 {
		t.Errorf("expected watts 2.0, got %.1f", devices[0].Watts)
	}
}

func TestEnergyStats(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterDevice(&DevicePower{DeviceID: "d1", Watts: 10.0, State: StateActive})
	mgr.RegisterDevice(&DevicePower{DeviceID: "d2", Watts: 5.0, State: StateActive})
	
	stats := mgr.GetStats()
	if stats.TotalWatts != 15.0 {
		t.Errorf("total watts = %.1f, want 15.0", stats.TotalWatts)
	}
	if stats.DailyKWh <= 0 {
		t.Error("daily kWh should be positive")
	}
	if stats.MonthlyKWh <= 0 {
		t.Error("monthly kWh should be positive")
	}
}

func TestScheduleRules(t *testing.T) {
	mgr := NewManager(nil)
	
	rule := &ScheduleRule{
		ID:          "rule1",
		Name:        "夜间节能",
		Enabled:     true,
		StartTime:   "23:00",
		EndTime:     "07:00",
		Days:        []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
		TargetState: StateStandby,
		Devices:     []string{"disk1", "disk2"},
		Priority:    1,
	}
	
	mgr.AddRule(rule)
	rules := mgr.GetRules()
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ID != "rule1" {
		t.Errorf("expected rule1, got %s", rules[0].ID)
	}
}

func TestRemoveRule(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.AddRule(&ScheduleRule{ID: "rule1", Name: "test"})
	mgr.AddRule(&ScheduleRule{ID: "rule2", Name: "test2"})
	
	removed := mgr.RemoveRule("rule1")
	if !removed {
		t.Error("expected rule to be removed")
	}
	
	rules := mgr.GetRules()
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ID != "rule2" {
		t.Errorf("expected rule2, got %s", rules[0].ID)
	}
}

func TestSavingTips(t *testing.T) {
	config := DefaultManagerConfig()
	config.IdleTimeout = 1 * time.Millisecond
	mgr := NewManager(config)
	
	mgr.RegisterDevice(&DevicePower{
		DeviceID:   "disk1",
		State:      StateIdle,
		Watts:      8.0,
		LastActive: time.Now().Add(-1 * time.Hour),
	})
	
	stats := mgr.GetStats()
	if len(stats.SavingTips) == 0 {
		t.Error("expected saving tips")
	}
}

func TestCO2Calculation(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterDevice(&DevicePower{DeviceID: "d1", Watts: 100.0, State: StateActive})
	
	stats := mgr.GetStats()
	if stats.CO2Kg <= 0 {
		t.Error("CO2 should be positive")
	}
}

func TestSetDeviceActive(t *testing.T) {
	mgr := NewManager(nil)
	
	mgr.RegisterDevice(&DevicePower{
		DeviceID:   "disk1",
		State:      StateStandby,
		Watts:      2.0,
		LastActive: time.Now().Add(-1 * time.Hour),
	})
	
	mgr.SetDeviceActive("disk1")
	devices := mgr.GetDevices()
	if devices[0].State != StateActive {
		t.Errorf("expected active state, got %s", devices[0].State)
	}
}

func TestMonitorStartStop(t *testing.T) {
	mgr := NewManager(&ManagerConfig{
		IdleTimeout:     100 * time.Millisecond,
		StandbyTimeout:  200 * time.Millisecond,
		ElectricityRate: 0.55,
		CO2Factor:       0.5703,
	})
	
	mgr.Start()
	time.Sleep(50 * time.Millisecond)
	mgr.Stop()
}
