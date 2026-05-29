// Package powermgmt 电源管理 - 测试
package powermgmt

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.GetState() != PowerOn {
		t.Errorf("expected PowerOn, got %s", m.GetState())
	}
}

func TestPowerStates(t *testing.T) {
	states := map[PowerState]string{
		PowerOn:        "on",
		PowerOff:       "off",
		PowerSleep:     "sleep",
		PowerHibernate: "hibernate",
		PowerWaking:    "waking",
	}
	for state, expected := range states {
		if string(state) != expected {
			t.Errorf("state %v != %q", state, expected)
		}
	}
}

func TestSetState(t *testing.T) {
	m := NewManager()

	var callbackFrom, callbackTo PowerState
	m.SetStateChangeCallback(func(from, to PowerState) {
		callbackFrom = from
		callbackTo = to
	})

	if err := m.SetState(PowerSleep); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}
	if m.GetState() != PowerSleep {
		t.Errorf("expected PowerSleep, got %s", m.GetState())
	}

	time.Sleep(10 * time.Millisecond)
	if callbackFrom != PowerOn || callbackTo != PowerSleep {
		t.Errorf("callback not called correctly: %s -> %s", callbackFrom, callbackTo)
	}

	// Same state should be no-op
	if err := m.SetState(PowerSleep); err != nil {
		t.Errorf("same state should not error: %v", err)
	}
}

func TestSchedules(t *testing.T) {
	m := NewManager()

	schedule := PowerSchedule{
		ID:       "test-schedule",
		Name:     "Daily shutdown",
		Type:     ScheduleDaily,
		Action:   ActionPowerOff,
		Enabled:  true,
		Hour:     23,
		Minute:   0,
		Weekdays: []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
	}

	if err := m.AddSchedule(schedule); err != nil {
		t.Fatalf("AddSchedule failed: %v", err)
	}

	// Duplicate should fail
	if err := m.AddSchedule(schedule); err == nil {
		t.Error("expected error on duplicate schedule")
	}

	got, err := m.GetSchedule("test-schedule")
	if err != nil {
		t.Fatalf("GetSchedule failed: %v", err)
	}
	if got.Name != "Daily shutdown" {
		t.Errorf("expected 'Daily shutdown', got %q", got.Name)
	}
	if got.NextRun.IsZero() {
		t.Error("NextRun should be calculated")
	}

	schedules := m.ListSchedules()
	if len(schedules) != 1 {
		t.Errorf("expected 1 schedule, got %d", len(schedules))
	}

	// Enable/Disable
	if err := m.DisableSchedule("test-schedule"); err != nil {
		t.Fatalf("DisableSchedule failed: %v", err)
	}
	got2, _ := m.GetSchedule("test-schedule")
	if got2.Enabled {
		t.Error("schedule should be disabled")
	}

	if err := m.EnableSchedule("test-schedule"); err != nil {
		t.Fatalf("EnableSchedule failed: %v", err)
	}
	got3, _ := m.GetSchedule("test-schedule")
	if !got3.Enabled {
		t.Error("schedule should be enabled")
	}

	// Update
	if err := m.UpdateSchedule("test-schedule", func(s *PowerSchedule) {
		s.Hour = 22
		s.Minute = 30
	}); err != nil {
		t.Fatalf("UpdateSchedule failed: %v", err)
	}
	got4, _ := m.GetSchedule("test-schedule")
	if got4.Hour != 22 || got4.Minute != 30 {
		t.Errorf("expected 22:30, got %d:%d", got4.Hour, got4.Minute)
	}

	// Remove
	if err := m.RemoveSchedule("test-schedule"); err != nil {
		t.Fatalf("RemoveSchedule failed: %v", err)
	}
	if err := m.RemoveSchedule("nonexistent"); err == nil {
		t.Error("expected error on removing nonexistent schedule")
	}
}

func TestWakeTargets(t *testing.T) {
	m := NewManager()

	target := WakeTarget{
		Name:       "Desktop PC",
		MACAddress: "AA:BB:CC:DD:EE:FF",
		IPAddress:  "192.168.1.100",
		Enabled:    true,
	}

	if err := m.AddWakeTarget(target); err != nil {
		t.Fatalf("AddWakeTarget failed: %v", err)
	}

	// Duplicate should fail
	if err := m.AddWakeTarget(target); err == nil {
		t.Error("expected error on duplicate target")
	}

	targets := m.ListWakeTargets()
	if len(targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Port != 9 {
		t.Errorf("default port should be 9, got %d", targets[0].Port)
	}
	if targets[0].Broadcast != "255.255.255.255" {
		t.Errorf("default broadcast should be 255.255.255.255, got %q", targets[0].Broadcast)
	}

	// Wake
	if err := m.WakeDevice("AA:BB:CC:DD:EE:FF"); err != nil {
		t.Fatalf("WakeDevice failed: %v", err)
	}

	if err := m.WakeDevice("nonexistent"); err == nil {
		t.Error("expected error on nonexistent device")
	}

	// Disable and try to wake
	if err := m.AddWakeTarget(WakeTarget{
		Name:       "Disabled PC",
		MACAddress: "11:22:33:44:55:66",
		Enabled:    false,
	}); err != nil {
		t.Fatalf("AddWakeTarget failed: %v", err)
	}
	if err := m.WakeDevice("11:22:33:44:55:66"); err == nil {
		t.Error("expected error on disabled target")
	}

	// Remove
	if err := m.RemoveWakeTarget("AA:BB:CC:DD:EE:FF"); err != nil {
		t.Fatalf("RemoveWakeTarget failed: %v", err)
	}
}

func TestIdleConfig(t *testing.T) {
	m := NewManager()

	config := IdleConfig{
		Enabled:        true,
		IdleTimeout:    30 * time.Minute,
		IdleAction:     ActionSleep,
		MonitorCPU:     true,
		MonitorDisk:    true,
		MonitorNetwork: true,
		CPUThreshold:   10.0,
		DiskIOThreshold: 1024,
		NetIOThreshold:  512,
	}

	m.SetIdleConfig(config)
	got := m.GetIdleConfig()
	if !got.Enabled {
		t.Error("idle config should be enabled")
	}
	if got.IdleTimeout != 30*time.Minute {
		t.Errorf("expected 30m timeout, got %v", got.IdleTimeout)
	}
}

func TestEvents(t *testing.T) {
	m := NewManager()

	// Events are generated by state changes and schedule runs
	m.SetState(PowerSleep)
	m.SetState(PowerOn)

	events := m.GetEvents(10)
	if len(events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(events))
	}

	// Clear
	m.ClearEvents()
	events = m.GetEvents(10)
	if len(events) != 0 {
		t.Errorf("expected 0 events after clear, got %d", len(events))
	}
}

func TestUptime(t *testing.T) {
	m := NewManager()
	time.Sleep(10 * time.Millisecond)
	if m.GetUptime() < 10*time.Millisecond {
		t.Error("uptime should be >= 10ms")
	}

	m.state = PowerOff
	if m.GetUptime() != 0 {
		t.Error("uptime should be 0 when off")
	}
}

func TestExportImportConfig(t *testing.T) {
	m := NewManager()

	m.AddSchedule(PowerSchedule{
		ID:   "s1",
		Name: "Test",
		Type: ScheduleDaily,
		Action: ActionPowerOff,
		Hour:   22,
		Minute: 0,
	})
	m.AddWakeTarget(WakeTarget{
		Name:       "PC",
		MACAddress: "AA:BB:CC:DD:EE:FF",
		Enabled:    true,
	})

	data, err := m.ExportConfig()
	if err != nil {
		t.Fatalf("ExportConfig failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty config")
	}

	// Import to new manager
	m2 := NewManager()
	if err := m2.ImportConfig(data); err != nil {
		t.Fatalf("ImportConfig failed: %v", err)
	}

	schedules := m2.ListSchedules()
	if len(schedules) != 1 {
		t.Errorf("expected 1 schedule, got %d", len(schedules))
	}

	targets := m2.ListWakeTargets()
	if len(targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(targets))
	}
}

func TestScheduleTypes(t *testing.T) {
	types := map[ScheduleType]string{
		ScheduleOnce:    "once",
		ScheduleDaily:   "daily",
		ScheduleWeekly:  "weekly",
		ScheduleMonthly: "monthly",
	}
	for st, expected := range types {
		if string(st) != expected {
			t.Errorf("schedule type %v != %q", st, expected)
		}
	}
}

func TestActionTypes(t *testing.T) {
	actions := map[ActionType]string{
		ActionPowerOn:   "power_on",
		ActionPowerOff:  "power_off",
		ActionSleep:     "sleep",
		ActionHibernate: "hibernate",
		ActionReboot:    "reboot",
		ActionWakeOnLan: "wake_on_lan",
	}
	for action, expected := range actions {
		if string(action) != expected {
			t.Errorf("action %v != %q", action, expected)
		}
	}
}

func TestEmptyIDSchedule(t *testing.T) {
	m := NewManager()
	if err := m.AddSchedule(PowerSchedule{}); err == nil {
		t.Error("expected error on empty ID")
	}
}

func TestEmptyMACTarget(t *testing.T) {
	m := NewManager()
	if err := m.AddWakeTarget(WakeTarget{}); err == nil {
		t.Error("expected error on empty MAC")
	}
}
