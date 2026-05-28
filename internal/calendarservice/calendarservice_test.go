package calendarservice

import (
	"testing"
	"time"
)

func TestCreateCalendar(t *testing.T) {
	s := NewCalendarService()

	cal := &Calendar{
		ID:      "cal1",
		Name:    "Work",
		OwnerID: "user1",
	}

	err := s.CreateCalendar(cal)
	if err != nil {
		t.Fatalf("CreateCalendar failed: %v", err)
	}

	// 重复创建
	err = s.CreateCalendar(cal)
	if err == nil {
		t.Error("expected error for duplicate calendar")
	}
}

func TestCreateEvent(t *testing.T) {
	s := NewCalendarService()
	s.CreateCalendar(&Calendar{ID: "cal1", Name: "Work", OwnerID: "user1"})

	event := &CalendarEvent{
		ID:         "evt1",
		CalendarID: "cal1",
		Title:      "Meeting",
		StartTime:  time.Now(),
		EndTime:    time.Now().Add(time.Hour),
	}

	err := s.CreateEvent(event)
	if err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}

	// 结束时间早于开始时间
	event2 := &CalendarEvent{
		ID:         "evt2",
		CalendarID: "cal1",
		Title:      "Bad",
		StartTime:  time.Now(),
		EndTime:    time.Now().Add(-time.Hour),
	}
	err = s.CreateEvent(event2)
	if err == nil {
		t.Error("expected error for end before start")
	}
}

func TestListEvents(t *testing.T) {
	s := NewCalendarService()
	s.CreateCalendar(&Calendar{ID: "cal1", Name: "Work", OwnerID: "user1"})

	now := time.Now()
	s.CreateEvent(&CalendarEvent{
		ID: "evt1", CalendarID: "cal1", Title: "Morning",
		StartTime: now, EndTime: now.Add(time.Hour),
	})
	s.CreateEvent(&CalendarEvent{
		ID: "evt2", CalendarID: "cal1", Title: "Afternoon",
		StartTime: now.Add(5 * time.Hour), EndTime: now.Add(6 * time.Hour),
	})

	events := s.ListEvents("cal1", now.Add(-time.Hour), now.Add(2*time.Hour))
	if len(events) != 1 {
		t.Errorf("expected 1 event in range, got %d", len(events))
	}
}

func TestShareCalendar(t *testing.T) {
	s := NewCalendarService()
	s.CreateCalendar(&Calendar{ID: "cal1", Name: "Work", OwnerID: "user1"})

	err := s.ShareCalendar("cal1", "user2")
	if err != nil {
		t.Fatalf("ShareCalendar failed: %v", err)
	}

	cal, _ := s.GetCalendar("cal1")
	if !cal.IsShared {
		t.Error("expected calendar to be shared")
	}
	if len(cal.SharedWith) != 1 {
		t.Errorf("expected 1 shared user, got %d", len(cal.SharedWith))
	}
}

func TestDeleteEvent(t *testing.T) {
	s := NewCalendarService()
	s.CreateCalendar(&Calendar{ID: "cal1", Name: "Work", OwnerID: "user1"})

	s.CreateEvent(&CalendarEvent{
		ID: "evt1", CalendarID: "cal1", Title: "Meeting",
		StartTime: time.Now(), EndTime: time.Now().Add(time.Hour),
	})

	err := s.DeleteEvent("cal1", "evt1")
	if err != nil {
		t.Fatalf("DeleteEvent failed: %v", err)
	}

	_, err = s.GetEvent("cal1", "evt1")
	if err == nil {
		t.Error("expected error for deleted event")
	}
}

func TestUserCalendars(t *testing.T) {
	s := NewCalendarService()
	s.CreateCalendar(&Calendar{ID: "cal1", Name: "Work", OwnerID: "user1"})
	s.CreateCalendar(&Calendar{ID: "cal2", Name: "Personal", OwnerID: "user1"})

	cals := s.ListUserCalendars("user1")
	if len(cals) != 2 {
		t.Errorf("expected 2 calendars, got %d", len(cals))
	}
}
