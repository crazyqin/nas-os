package calendarservice

import (
	"fmt"
	"sync"
	"time"
)

// EventStatus 事件状态
type EventStatus string

const (
	EventTentative EventStatus = "tentative"
	EventConfirmed EventStatus = "confirmed"
	EventCancelled EventStatus = "cancelled"
)

// EventRecurrence 重复规则
type EventRecurrence string

const (
	RecurrenceNone    EventRecurrence = "none"
	RecurrenceDaily   EventRecurrence = "daily"
	RecurrenceWeekly  EventRecurrence = "weekly"
	RecurrenceMonthly EventRecurrence = "monthly"
	RecurrenceYearly  EventRecurrence = "yearly"
)

// Reminder 提醒
type Reminder struct {
	ID        string `json:"id"`
	Minutes   int    `json:"minutes"` // 提前多少分钟提醒
	Method    string `json:"method"`  // email, notification, sms
	Triggered bool   `json:"triggered"`
}

// Attendee 参与者
type Attendee struct {
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Status      string `json:"status"` // accepted, declined, tentative, pending
	IsOrganizer bool   `json:"is_organizer"`
}

// CalendarEvent 日历事件
type CalendarEvent struct {
	ID          string          `json:"id"`
	CalendarID  string          `json:"calendar_id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Location    string          `json:"location"`
	StartTime   time.Time       `json:"start_time"`
	EndTime     time.Time       `json:"end_time"`
	AllDay      bool            `json:"all_day"`
	Status      EventStatus     `json:"status"`
	Recurrence  EventRecurrence `json:"recurrence"`
	Reminders   []Reminder      `json:"reminders"`
	Attendees   []Attendee      `json:"attendees"`
	Tags        []string        `json:"tags"`
	Color       string          `json:"color"`
	URL         string          `json:"url"`
	CreatedBy   string          `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// Calendar 日历
type Calendar struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	OwnerID     string    `json:"owner_id"`
	IsDefault   bool      `json:"is_default"`
	IsShared    bool      `json:"is_shared"`
	SharedWith  []string  `json:"shared_with"`
	Visibility  string    `json:"visibility"` // private, public, shared
	Timezone    string    `json:"timezone"`
	CreatedAt   time.Time `json:"created_at"`
}

// CalendarService 日历服务 (类似群晖 Calendar)
type CalendarService struct {
	mu        sync.RWMutex
	calendars map[string]*Calendar
	events    map[string]map[string]*CalendarEvent // calendarID -> eventID -> event
	users     map[string][]string                  // userID -> calendarIDs
}

// NewCalendarService 创建日历服务
func NewCalendarService() *CalendarService {
	return &CalendarService{
		calendars: make(map[string]*Calendar),
		events:    make(map[string]map[string]*CalendarEvent),
		users:     make(map[string][]string),
	}
}

// CreateCalendar 创建日历
func (s *CalendarService) CreateCalendar(cal *Calendar) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cal.ID == "" {
		return fmt.Errorf("日历ID不能为空")
	}
	if _, exists := s.calendars[cal.ID]; exists {
		return fmt.Errorf("日历 %s 已存在", cal.ID)
	}

	cal.CreatedAt = time.Now()
	s.calendars[cal.ID] = cal
	s.events[cal.ID] = make(map[string]*CalendarEvent)
	s.users[cal.OwnerID] = append(s.users[cal.OwnerID], cal.ID)
	return nil
}

// DeleteCalendar 删除日历
func (s *CalendarService) DeleteCalendar(calID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cal, exists := s.calendars[calID]
	if !exists {
		return fmt.Errorf("日历 %s 不存在", calID)
	}

	// 从用户列表中移除
	userCals := s.users[cal.OwnerID]
	for i, id := range userCals {
		if id == calID {
			s.users[cal.OwnerID] = append(userCals[:i], userCals[i+1:]...)
			break
		}
	}

	delete(s.calendars, calID)
	delete(s.events, calID)
	return nil
}

// CreateEvent 创建事件
func (s *CalendarService) CreateEvent(event *CalendarEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.calendars[event.CalendarID]; !exists {
		return fmt.Errorf("日历 %s 不存在", event.CalendarID)
	}

	if event.EndTime.Before(event.StartTime) {
		return fmt.Errorf("结束时间不能早于开始时间")
	}

	event.CreatedAt = time.Now()
	event.UpdatedAt = time.Now()
	if event.Status == "" {
		event.Status = EventConfirmed
	}

	s.events[event.CalendarID][event.ID] = event
	return nil
}

// UpdateEvent 更新事件
func (s *CalendarService) UpdateEvent(event *CalendarEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	calEvents, exists := s.events[event.CalendarID]
	if !exists {
		return fmt.Errorf("日历 %s 不存在", event.CalendarID)
	}
	if _, exists := calEvents[event.ID]; !exists {
		return fmt.Errorf("事件 %s 不存在", event.ID)
	}

	event.UpdatedAt = time.Now()
	calEvents[event.ID] = event
	return nil
}

// DeleteEvent 删除事件
func (s *CalendarService) DeleteEvent(calID, eventID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	calEvents, exists := s.events[calID]
	if !exists {
		return fmt.Errorf("日历 %s 不存在", calID)
	}
	if _, exists := calEvents[eventID]; !exists {
		return fmt.Errorf("事件 %s 不存在", eventID)
	}

	delete(calEvents, eventID)
	return nil
}

// GetEvent 获取事件
func (s *CalendarService) GetEvent(calID, eventID string) (*CalendarEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	calEvents, exists := s.events[calID]
	if !exists {
		return nil, fmt.Errorf("日历 %s 不存在", calID)
	}
	event, exists := calEvents[eventID]
	if !exists {
		return nil, fmt.Errorf("事件 %s 不存在", eventID)
	}
	return event, nil
}

// ListEvents 列出日历事件
func (s *CalendarService) ListEvents(calID string, start, end time.Time) []*CalendarEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*CalendarEvent, 0)
	calEvents, exists := s.events[calID]
	if !exists {
		return result
	}

	for _, event := range calEvents {
		if event.Status == EventCancelled {
			continue
		}
		// 时间范围过滤
		if !event.EndTime.Before(start) && !event.StartTime.After(end) {
			result = append(result, event)
		}
	}
	return result
}

// ListUserEvents 列出用户所有日历的事件
func (s *CalendarService) ListUserEvents(userID string, start, end time.Time) []*CalendarEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*CalendarEvent, 0)

	for _, calID := range s.users[userID] {
		if events, ok := s.events[calID]; ok {
			for _, event := range events {
				if event.Status == EventCancelled {
					continue
				}
				if !event.EndTime.Before(start) && !event.StartTime.After(end) {
					result = append(result, event)
				}
			}
		}
	}

	return result
}

// ShareCalendar 分享日历
func (s *CalendarService) ShareCalendar(calID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cal, exists := s.calendars[calID]
	if !exists {
		return fmt.Errorf("日历 %s 不存在", calID)
	}

	for _, id := range cal.SharedWith {
		if id == userID {
			return nil // 已经分享过了
		}
	}

	cal.SharedWith = append(cal.SharedWith, userID)
	cal.IsShared = true
	return nil
}

// GetCalendar 获取日历
func (s *CalendarService) GetCalendar(calID string) (*Calendar, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cal, exists := s.calendars[calID]
	if !exists {
		return nil, fmt.Errorf("日历 %s 不存在", calID)
	}
	return cal, nil
}

// ListUserCalendars 列出用户的日历
func (s *CalendarService) ListUserCalendars(userID string) []*Calendar {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Calendar, 0)
	for _, calID := range s.users[userID] {
		if cal, exists := s.calendars[calID]; exists {
			result = append(result, cal)
		}
	}
	return result
}
