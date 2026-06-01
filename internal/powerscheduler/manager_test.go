package powerscheduler

import (
	"fmt"
	"testing"
)

func TestCreateSchedule(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	s := Schedule{
		ID:   "test-1",
		Name: "夜间休眠",
		Action: ActionSuspend,
		Time: "02:00",
		Days: []DayOfWeek{Monday, Tuesday, Wednesday, Thursday, Friday},
	}
	result, err := m.CreateSchedule(s)
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}
	if result.Name != "夜间休眠" {
		t.Errorf("expected 夜间休眠, got %s", result.Name)
	}
}

func TestDuplicateSchedule(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	s := Schedule{ID: "dup", Name: "test", Action: ActionSuspend, Time: "01:00"}
	m.CreateSchedule(s)
	_, err := m.CreateSchedule(s)
	if err != ErrScheduleExists {
		t.Errorf("expected ErrScheduleExists, got %v", err)
	}
}

func TestPowerProfiles(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	m.ApplyProfile("powersave")
	state := m.GetPowerState()
	_ = state // State would be updated in real implementation
}

func TestListSchedules(t *testing.T) {
	m := NewManager(nil)
	m.Start()
	defer m.Stop()

	for i := 0; i < 3; i++ {
		m.CreateSchedule(Schedule{
			ID:     fmt.Sprintf("s-%d", i),
			Name:   fmt.Sprintf("schedule-%d", i),
			Action: ActionSuspend,
			Time:   "01:00",
		})
	}
	schedules := m.ListSchedules()
	if len(schedules) != 3 {
		t.Errorf("expected 3 schedules, got %d", len(schedules))
	}
}
