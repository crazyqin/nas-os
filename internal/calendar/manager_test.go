// Package calendar 测试
package calendar

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("管理器不应为nil")
	}
}

func TestCreateCalendar(t *testing.T) {
	m := NewManager()
	err := m.CreateCalendar(&Calendar{
		ID:    "cal-1",
		Name:  "工作",
		Color: "#3B82F6",
		Owner: "admin",
	})
	if err != nil {
		t.Fatalf("创建日历失败: %v", err)
	}
}

func TestGetCalendar(t *testing.T) {
	m := NewManager()
	m.CreateCalendar(&Calendar{ID: "cal-1", Name: "工作", Owner: "admin"})

	cal, err := m.GetCalendar("cal-1")
	if err != nil {
		t.Fatalf("获取日历失败: %v", err)
	}
	if cal.Name != "工作" {
		t.Errorf("名称不匹配: %s", cal.Name)
	}
}

func TestListCalendars(t *testing.T) {
	m := NewManager()
	m.CreateCalendar(&Calendar{ID: "cal-1", Name: "工作", Owner: "admin"})
	m.CreateCalendar(&Calendar{ID: "cal-2", Name: "个人", Owner: "admin"})

	cals := m.ListCalendars()
	if len(cals) != 2 {
		t.Errorf("期望2个日历，实际 %d", len(cals))
	}
}

func TestDeleteCalendar(t *testing.T) {
	m := NewManager()
	m.CreateCalendar(&Calendar{ID: "cal-1", Name: "temp", Owner: "admin"})

	err := m.DeleteCalendar("cal-1")
	if err != nil {
		t.Fatalf("删除日历失败: %v", err)
	}

	_, err = m.GetCalendar("cal-1")
	if err == nil {
		t.Error("已删除日历不应存在")
	}
}

func TestCreateEvent(t *testing.T) {
	m := NewManager()
	m.CreateCalendar(&Calendar{ID: "cal-1", Name: "工作", Owner: "admin"})

	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)
	err := m.CreateEvent(&Event{
		ID:         "evt-1",
		CalendarID: "cal-1",
		Title:      "周会",
		StartTime:  start,
		EndTime:    end,
		AllDay:     false,
		Status:     StatusConfirmed,
	})
	if err != nil {
		t.Fatalf("创建事件失败: %v", err)
	}
}

func TestGetEvent(t *testing.T) {
	m := NewManager()
	m.CreateCalendar(&Calendar{ID: "cal-1", Name: "工作", Owner: "admin"})

	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)
	m.CreateEvent(&Event{
		ID: "evt-1", CalendarID: "cal-1", Title: "周会",
		StartTime: start, EndTime: end,
	})

	evt, err := m.GetEvent("evt-1")
	if err != nil {
		t.Fatalf("获取事件失败: %v", err)
	}
	if evt.Title != "周会" {
		t.Errorf("标题不匹配: %s", evt.Title)
	}
}

func TestQueryEvents(t *testing.T) {
	m := NewManager()
	m.CreateCalendar(&Calendar{ID: "cal-1", Name: "工作", Owner: "admin"})

	now := time.Now()
	m.CreateEvent(&Event{
		ID: "evt-1", CalendarID: "cal-1", Title: "会议A",
		StartTime: now.Add(1 * time.Hour), EndTime: now.Add(2 * time.Hour),
	})
	m.CreateEvent(&Event{
		ID: "evt-2", CalendarID: "cal-1", Title: "会议B",
		StartTime: now.Add(25 * time.Hour), EndTime: now.Add(26 * time.Hour),
	})

	events := m.QueryEvents(EventQuery{
		CalendarID: "cal-1",
		Start:      now,
		End:        now.Add(24 * time.Hour),
	})
	if len(events) < 1 {
		t.Errorf("应查询到至少1个事件")
	}
}

func TestDeleteEvent(t *testing.T) {
	m := NewManager()
	m.CreateCalendar(&Calendar{ID: "cal-1", Name: "工作", Owner: "admin"})

	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)
	m.CreateEvent(&Event{
		ID: "evt-1", CalendarID: "cal-1", Title: "周会",
		StartTime: start, EndTime: end,
	})

	err := m.DeleteEvent("evt-1")
	if err != nil {
		t.Fatalf("删除事件失败: %v", err)
	}

	_, err = m.GetEvent("evt-1")
	if err == nil {
		t.Error("已删除事件不应存在")
	}
}

func TestImportICS(t *testing.T) {
	m := NewManager()
	m.CreateCalendar(&Calendar{ID: "cal-1", Name: "工作", Owner: "admin"})

	ics := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
DTSTART:20260501T100000Z
DTEND:20260501T110000Z
SUMMARY:测试会议
DESCRIPTION:这是测试
END:VEVENT
END:VCALENDAR`

	count, err := m.ImportICS("cal-1", ics)
	if err != nil {
		t.Fatalf("导入ICS失败: %v", err)
	}
	if count < 1 {
		t.Errorf("应导入至少1个事件")
	}
}

func TestExportCalendar(t *testing.T) {
	m := NewManager()
	m.CreateCalendar(&Calendar{ID: "cal-1", Name: "工作", Owner: "admin"})

	start := time.Now().Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)
	m.CreateEvent(&Event{
		ID: "evt-1", CalendarID: "cal-1", Title: "导出测试",
		StartTime: start, EndTime: end,
	})

	ics, err := m.ExportCalendar("cal-1")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if ics == "" {
		t.Error("导出内容不应为空")
	}
}
