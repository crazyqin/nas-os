package powersched

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}

	holidays := m.ListHolidays()
	if len(holidays) == 0 {
		t.Error("expected default holidays")
	}
}

func TestCreateTask(t *testing.T) {
	m := NewManager()

	task := &ScheduleTask{
		Name:    "每日开机",
		Type:    TaskTypePowerOn,
		Rule:    ScheduleRule{Type: "daily", Time: "08:00"},
		Enabled: true,
	}

	err := m.CreateTask(task)
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	if task.ID == "" {
		t.Error("expected task ID to be set")
	}
	if task.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	// 测试空名称
	err = m.CreateTask(&ScheduleTask{Type: TaskTypePowerOff, Rule: ScheduleRule{Type: "daily", Time: "22:00"}})
	if err == nil {
		t.Error("expected error for empty name")
	}

	// 测试空时间
	err = m.CreateTask(&ScheduleTask{Name: "test", Type: TaskTypePowerOff, Rule: ScheduleRule{Type: "daily"}})
	if err == nil {
		t.Error("expected error for empty time")
	}
}

func TestListTasks(t *testing.T) {
	m := NewManager()

	m.CreateTask(&ScheduleTask{
		Name: "任务1", Type: TaskTypePowerOn, Rule: ScheduleRule{Type: "daily", Time: "08:00"}, Enabled: true,
	})
	m.CreateTask(&ScheduleTask{
		Name: "任务2", Type: TaskTypePowerOff, Rule: ScheduleRule{Type: "daily", Time: "22:00"}, Enabled: true,
	})

	tasks := m.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestEnableDisableTask(t *testing.T) {
	m := NewManager()

	task := &ScheduleTask{
		Name:    "测试任务",
		Type:    TaskTypePowerOn,
		Rule:    ScheduleRule{Type: "daily", Time: "08:00"},
		Enabled: true,
	}
	m.CreateTask(task)

	err := m.DisableTask(task.ID)
	if err != nil {
		t.Fatalf("disable task failed: %v", err)
	}

	got := m.GetTask(task.ID)
	if got.Enabled {
		t.Error("expected task to be disabled")
	}

	err = m.EnableTask(task.ID)
	if err != nil {
		t.Fatalf("enable task failed: %v", err)
	}

	got = m.GetTask(task.ID)
	if !got.Enabled {
		t.Error("expected task to be enabled")
	}

	// 测试不存在的任务
	err = m.EnableTask("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestDeleteTask(t *testing.T) {
	m := NewManager()

	task := &ScheduleTask{
		Name:    "待删除",
		Type:    TaskTypePowerOff,
		Rule:    ScheduleRule{Type: "daily", Time: "22:00"},
		Enabled: true,
	}
	m.CreateTask(task)

	err := m.DeleteTask(task.ID)
	if err != nil {
		t.Fatalf("delete task failed: %v", err)
	}

	if m.GetTask(task.ID) != nil {
		t.Error("expected task to be deleted")
	}

	// 测试不存在的任务
	err = m.DeleteTask("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestUpdateTask(t *testing.T) {
	m := NewManager()

	task := &ScheduleTask{
		Name:    "原始任务",
		Type:    TaskTypePowerOn,
		Rule:    ScheduleRule{Type: "daily", Time: "08:00"},
		Enabled: true,
	}
	m.CreateTask(task)

	task.Name = "更新后的任务"
	task.Rule.Time = "09:00"

	err := m.UpdateTask(task)
	if err != nil {
		t.Fatalf("update task failed: %v", err)
	}

	got := m.GetTask(task.ID)
	if got.Name != "更新后的任务" {
		t.Errorf("expected '更新后的任务', got '%s'", got.Name)
	}

	// 测试不存在的任务
	err = m.UpdateTask(&ScheduleTask{ID: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestHolidayManagement(t *testing.T) {
	m := NewManager()

	// 添加节假日
	holiday := &Holiday{Date: "2026-12-25", Name: "圣诞节"}
	err := m.AddHoliday(holiday)
	if err != nil {
		t.Fatalf("add holiday failed: %v", err)
	}

	holidays := m.ListHolidays()
	found := false
	for _, h := range holidays {
		if h.Date == "2026-12-25" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Christmas holiday to be listed")
	}

	// 移除节假日
	err = m.RemoveHoliday("2026-12-25")
	if err != nil {
		t.Fatalf("remove holiday failed: %v", err)
	}

	holidays = m.ListHolidays()
	for _, h := range holidays {
		if h.Date == "2026-12-25" {
			t.Error("expected Christmas holiday to be removed")
		}
	}

	// 测试空日期
	err = m.AddHoliday(&Holiday{Name: "test"})
	if err == nil {
		t.Error("expected error for empty date")
	}

	// 测试移除不存在的节假日
	err = m.RemoveHoliday("2099-01-01")
	if err == nil {
		t.Error("expected error for nonexistent holiday")
	}
}

func TestGetCalendar(t *testing.T) {
	m := NewManager()

	m.CreateTask(&ScheduleTask{
		Name:    "每日开机",
		Type:    TaskTypePowerOn,
		Rule:    ScheduleRule{Type: "daily", Time: "08:00"},
		Enabled: true,
	})

	calendar := m.GetCalendar(2026, time.May)
	if calendar.Year != 2026 {
		t.Errorf("expected 2026, got %d", calendar.Year)
	}
	if calendar.Month != time.May {
		t.Errorf("expected May, got %v", calendar.Month)
	}
	if len(calendar.Days) != 31 {
		t.Errorf("expected 31 days for May, got %d", len(calendar.Days))
	}

	// 检查每天都有任务
	for _, day := range calendar.Days {
		if len(day.Tasks) == 0 {
			t.Errorf("expected tasks on day %d", day.Day)
		}
	}
}

func TestCheckConflicts(t *testing.T) {
	m := NewManager()

	m.CreateTask(&ScheduleTask{
		Name: "开机任务", Type: TaskTypePowerOn,
		Rule: ScheduleRule{Type: "daily", Time: "08:00"}, Enabled: true,
	})

	// 同类型同时间应该冲突
	conflicts := m.CheckConflicts(&ScheduleTask{
		ID: "new", Name: "另一个开机", Type: TaskTypePowerOn,
		Rule: ScheduleRule{Type: "daily", Time: "08:00"},
	})
	if len(conflicts) == 0 {
		t.Error("expected conflict with same type and time")
	}

	// 开关机时间冲突
	conflicts = m.CheckConflicts(&ScheduleTask{
		ID: "new", Name: "关机任务", Type: TaskTypePowerOff,
		Rule: ScheduleRule{Type: "daily", Time: "08:00"},
	})
	if len(conflicts) == 0 {
		t.Error("expected conflict with power off at same time")
	}

	// 不同时间不应该冲突
	conflicts = m.CheckConflicts(&ScheduleTask{
		ID: "new", Name: "另一个开机", Type: TaskTypePowerOn,
		Rule: ScheduleRule{Type: "daily", Time: "12:00"},
	})
	if len(conflicts) != 0 {
		t.Error("expected no conflict with different time")
	}
}

func TestPowerEvents(t *testing.T) {
	m := NewManager()

	m.AddPowerEvent(&PowerEvent{
		TaskID: "task-1", TaskName: "开机", TaskType: TaskTypePowerOn,
		ExecTime: time.Now(), Success: true,
	})

	m.AddPowerEvent(&PowerEvent{
		TaskID: "task-2", TaskName: "关机", TaskType: TaskTypePowerOff,
		ExecTime: time.Now().Add(-1 * time.Hour), Success: true,
	})

	since := time.Now().Add(-30 * time.Minute)
	events := m.GetPowerEvents(since)
	if len(events) != 1 {
		t.Errorf("expected 1 event since 30min ago, got %d", len(events))
	}
}

func TestUPSStatus(t *testing.T) {
	m := NewManager()

	ups, err := m.GetUPSStatus()
	if err != nil {
		t.Fatalf("get UPS status failed: %v", err)
	}
	if ups.Battery < 0 || ups.Battery > 100 {
		t.Errorf("expected battery 0-100, got %d", ups.Battery)
	}
}
