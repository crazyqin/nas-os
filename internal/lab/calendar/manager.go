// Package calendar 提供日历与事件管理功能.
package calendar

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 日历管理器.
type Manager struct {
	mu        sync.RWMutex
	calendars map[string]*Calendar
	events    map[string]*Event
}

// NewManager 创建日历管理器.
func NewManager() *Manager {
	return &Manager{
		calendars: make(map[string]*Calendar),
		events:    make(map[string]*Event),
	}
}

// ========== 日历 CRUD ==========

// CreateCalendar 创建日历.
func (m *Manager) CreateCalendar(cal *Calendar) error {
	if cal.Name == "" {
		return ErrInvalidInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if cal.ID == "" {
		cal.ID = uuid.New().String()
	}
	if cal.CreatedAt.IsZero() {
		cal.CreatedAt = time.Now()
	}
	m.calendars[cal.ID] = cal
	return nil
}

// GetCalendar 获取日历.
func (m *Manager) GetCalendar(id string) (*Calendar, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cal, ok := m.calendars[id]
	if !ok {
		return nil, ErrCalendarNotFound
	}
	return cal, nil
}

// ListCalendars 列出所有日历.
func (m *Manager) ListCalendars() []*Calendar {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cals := make([]*Calendar, 0, len(m.calendars))
	for _, c := range m.calendars {
		cals = append(cals, c)
	}
	return cals
}

// UpdateCalendar 更新日历.
func (m *Manager) UpdateCalendar(cal *Calendar) error {
	if cal.ID == "" {
		return ErrInvalidInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.calendars[cal.ID]
	if !ok {
		return ErrCalendarNotFound
	}
	if cal.Name != "" {
		existing.Name = cal.Name
	}
	if cal.Color != "" {
		existing.Color = cal.Color
	}
	if cal.Description != "" {
		existing.Description = cal.Description
	}
	if cal.Owner != "" {
		existing.Owner = cal.Owner
	}
	existing.IsDefault = cal.IsDefault
	return nil
}

// DeleteCalendar 删除日历（日历下不允许有事件）.
func (m *Manager) DeleteCalendar(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.calendars[id]; !ok {
		return ErrCalendarNotFound
	}
	// 检查是否有关联事件
	for _, e := range m.events {
		if e.CalendarID == id {
			return ErrCalendarHasEvents
		}
	}
	delete(m.calendars, id)
	return nil
}

// ========== 事件 CRUD ==========

// CreateEvent 创建事件.
func (m *Manager) CreateEvent(evt *Event) error {
	if evt.Title == "" || evt.CalendarID == "" {
		return ErrInvalidInput
	}
	if evt.EndTime.Before(evt.StartTime) {
		return ErrEndTimeBeforeStart
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.calendars[evt.CalendarID]; !ok {
		return ErrCalendarNotFound
	}

	if evt.ID == "" {
		evt.ID = uuid.New().String()
	}
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = time.Now()
	}
	if evt.Status == "" {
		evt.Status = StatusConfirmed
	}
	m.events[evt.ID] = evt
	return nil
}

// GetEvent 获取事件.
func (m *Manager) GetEvent(id string) (*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	evt, ok := m.events[id]
	if !ok {
		return nil, ErrEventNotFound
	}
	return evt, nil
}

// UpdateEvent 更新事件.
func (m *Manager) UpdateEvent(evt *Event) error {
	if evt.ID == "" {
		return ErrInvalidInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.events[evt.ID]
	if !ok {
		return ErrEventNotFound
	}
	if evt.CalendarID != "" {
		if _, calOK := m.calendars[evt.CalendarID]; !calOK {
			return ErrCalendarNotFound
		}
		existing.CalendarID = evt.CalendarID
	}
	if evt.Title != "" {
		existing.Title = evt.Title
	}
	if evt.Description != "" {
		existing.Description = evt.Description
	}
	if evt.Location != "" {
		existing.Location = evt.Location
	}
	if !evt.StartTime.IsZero() {
		existing.StartTime = evt.StartTime
	}
	if !evt.EndTime.IsZero() {
		existing.EndTime = evt.EndTime
	}
	if evt.EndTime.Before(evt.StartTime) && !evt.StartTime.IsZero() && !evt.EndTime.IsZero() {
		return ErrEndTimeBeforeStart
	}
	existing.AllDay = evt.AllDay
	if evt.Recurrence != nil {
		existing.Recurrence = evt.Recurrence
	}
	if evt.Reminders != nil {
		existing.Reminders = evt.Reminders
	}
	if evt.Attendees != nil {
		existing.Attendees = evt.Attendees
	}
	if evt.Status != "" {
		existing.Status = evt.Status
	}
	return nil
}

// DeleteEvent 删除事件.
func (m *Manager) DeleteEvent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.events[id]; !ok {
		return ErrEventNotFound
	}
	delete(m.events, id)
	return nil
}

// ========== 查询 ==========

// QueryEvents 按条件查询事件.
func (m *Manager) QueryEvents(q EventQuery) []*Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Event
	for _, evt := range m.events {
		// 日历过滤
		if q.CalendarID != "" && evt.CalendarID != q.CalendarID {
			continue
		}
		// 状态过滤
		if q.Status != "" && string(evt.Status) != q.Status {
			continue
		}
		// 日期范围过滤
		if !q.Start.IsZero() && evt.EndTime.Before(q.Start) {
			continue
		}
		if !q.End.IsZero() && evt.StartTime.After(q.End) {
			continue
		}
		// 关键词搜索
		if q.Keyword != "" {
			kw := strings.ToLower(q.Keyword)
			if !strings.Contains(strings.ToLower(evt.Title), kw) &&
				!strings.Contains(strings.ToLower(evt.Description), kw) &&
				!strings.Contains(strings.ToLower(evt.Location), kw) {
				continue
			}
		}
		// 展开重复事件
		if evt.Recurrence != nil {
			expanded := expandRecurrence(evt, q.Start, q.End)
			result = append(result, expanded...)
		} else {
			result = append(result, evt)
		}
	}
	return result
}

// ========== ICS 导入/导出 ==========

// ExportCalendar 导出日历为 ICS 格式.
func (m *Manager) ExportCalendar(calendarID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cal, ok := m.calendars[calendarID]
	if !ok {
		return "", ErrCalendarNotFound
	}

	var buf bytes.Buffer
	buf.WriteString("BEGIN:VCALENDAR\r\n")
	buf.WriteString("VERSION:2.0\r\n")
	buf.WriteString("PRODID:-//NAS-OS//Calendar//CN\r\n")
	buf.WriteString("CALSCALE:GREGORIAN\r\n")
	buf.WriteString("X-WR-CALNAME:" + escapeICS(cal.Name) + "\r\n")

	for _, evt := range m.events {
		if evt.CalendarID != calendarID {
			continue
		}
		writeEventToICS(&buf, evt)
	}

	buf.WriteString("END:VCALENDAR\r\n")
	return buf.String(), nil
}

// ExportAll 导出所有日历为 ICS.
func (m *Manager) ExportAll() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var buf bytes.Buffer
	buf.WriteString("BEGIN:VCALENDAR\r\n")
	buf.WriteString("VERSION:2.0\r\n")
	buf.WriteString("PRODID:-//NAS-OS//Calendar//CN\r\n")
	buf.WriteString("CALSCALE:GREGORIAN\r\n")

	for _, evt := range m.events {
		writeEventToICS(&buf, evt)
	}

	buf.WriteString("END:VCALENDAR\r\n")
	return buf.String(), nil
}

// ImportICS 导入 ICS 内容到指定日历.
func (m *Manager) ImportICS(calendarID string, icsContent string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.calendars[calendarID]; !ok {
		return 0, ErrCalendarNotFound
	}

	events := parseICS(icsContent, calendarID)
	count := 0
	for _, evt := range events {
		if evt.ID == "" {
			evt.ID = uuid.New().String()
		}
		evt.CreatedAt = time.Now()
		m.events[evt.ID] = evt
		count++
	}
	return count, nil
}

// ========== 重复事件展开 ==========

// expandRecurrence 将重复事件展开为指定范围内的多个实例.
func expandRecurrence(evt *Event, start, end time.Time) []*Event {
	rule := evt.Recurrence
	if rule == nil {
		return []*Event{evt}
	}

	interval := rule.Interval
	if interval <= 0 {
		interval = 1
	}

	// 查询范围默认值
	queryStart := start
	if queryStart.IsZero() {
		queryStart = time.Now().AddDate(0, -1, 0)
	}
	queryEnd := end
	if queryEnd.IsZero() {
		queryEnd = time.Now().AddDate(1, 0, 0)
	}

	// 构建例外日期集合
	exceptionSet := make(map[string]bool)
	for _, ex := range rule.Exceptions {
		exceptionSet[ex.Format("2006-01-02")] = true
	}

	var result []*Event
	duration := evt.EndTime.Sub(evt.StartTime)
	current := evt.StartTime
	count := 0

	for current.Before(queryEnd) {
		if rule.Count > 0 && count >= rule.Count {
			break
		}
		if rule.Until != nil && current.After(*rule.Until) {
			break
		}

		// 检查星期过滤
		if len(rule.ByDay) > 0 && !matchesByDay(current, rule.ByDay) {
			current = advanceOneDay(current, rule.Frequency)
			continue
		}

		// 检查例外日期
		if exceptionSet[current.Format("2006-01-02")] {
			current = advanceOneDay(current, rule.Frequency)
			count++
			continue
		}

		evtEnd := current.Add(duration)

		// 在查询范围内
		if !evtEnd.Before(queryStart) && !current.After(queryEnd) {
			instance := &Event{
				ID:          evt.ID + "_" + strconv.Itoa(count),
				CalendarID:  evt.CalendarID,
				Title:       evt.Title,
				Description: evt.Description,
				Location:    evt.Location,
				StartTime:   current,
				EndTime:     evtEnd,
				AllDay:      evt.AllDay,
				Reminders:   evt.Reminders,
				Attendees:   evt.Attendees,
				Status:      evt.Status,
				CreatedAt:   evt.CreatedAt,
			}
			result = append(result, instance)
		}

		count++
		// 根据频率推进日期
		switch rule.Frequency {
		case FreqDaily:
			current = current.AddDate(0, 0, interval)
		case FreqWeekly:
			current = current.AddDate(0, 0, 7*interval)
		case FreqMonthly:
			current = current.AddDate(0, interval, 0)
		case FreqYearly:
			current = current.AddDate(interval, 0, 0)
		default:
			current = current.AddDate(0, 0, 1)
		}
	}

	return result
}

// advanceOneDay 按频率类型推进一天（用于遍历 byDay 过滤）.
func advanceOneDay(t time.Time, freq RecurrenceFrequency) time.Time {
	return t.AddDate(0, 0, 1)
}

// matchesByDay 检查日期是否匹配 byDay 规则.
func matchesByDay(t time.Time, byDay []string) bool {
	dayMap := map[string]time.Weekday{
		"SU": time.Sunday,
		"MO": time.Monday,
		"TU": time.Tuesday,
		"WE": time.Wednesday,
		"TH": time.Thursday,
		"FR": time.Friday,
		"SA": time.Saturday,
	}
	weekday := t.Weekday()
	for _, d := range byDay {
		if wd, ok := dayMap[strings.ToUpper(d)]; ok && wd == weekday {
			return true
		}
	}
	return false
}

// ========== ICS 解析与生成 ==========

// writeEventToICS 将事件写入 ICS 缓冲区.
func writeEventToICS(buf *bytes.Buffer, evt *Event) {
	buf.WriteString("BEGIN:VEVENT\r\n")
	buf.WriteString("UID:" + evt.ID + "\r\n")
	buf.WriteString("SUMMARY:" + escapeICS(evt.Title) + "\r\n")

	if evt.Description != "" {
		buf.WriteString("DESCRIPTION:" + escapeICS(evt.Description) + "\r\n")
	}
	if evt.Location != "" {
		buf.WriteString("LOCATION:" + escapeICS(evt.Location) + "\r\n")
	}

	if evt.AllDay {
		buf.WriteString("DTSTART;VALUE=DATE:" + evt.StartTime.Format("20060102") + "\r\n")
		buf.WriteString("DTEND;VALUE=DATE:" + evt.EndTime.Format("20060102") + "\r\n")
	} else {
		buf.WriteString("DTSTART:" + evt.StartTime.Format("20060102T150405Z") + "\r\n")
		buf.WriteString("DTEND:" + evt.EndTime.Format("20060102T150405Z") + "\r\n")
	}

	buf.WriteString("STATUS:" + strings.ToUpper(string(evt.Status)) + "\r\n")
	buf.WriteString("DTSTAMP:" + evt.CreatedAt.Format("20060102T150405Z") + "\r\n")
	buf.WriteString("CREATED:" + evt.CreatedAt.Format("20060102T150405Z") + "\r\n")

	// RRULE
	if evt.Recurrence != nil {
		rr := evt.Recurrence
		freqMap := map[RecurrenceFrequency]string{
			FreqDaily:   "DAILY",
			FreqWeekly:  "WEEKLY",
			FreqMonthly: "MONTHLY",
			FreqYearly:  "YEARLY",
		}
		rrule := "RRULE:FREQ=" + freqMap[rr.Frequency]
		if rr.Interval > 1 {
			rrule += ";INTERVAL=" + strconv.Itoa(rr.Interval)
		}
		if rr.Count > 0 {
			rrule += ";COUNT=" + strconv.Itoa(rr.Count)
		}
		if rr.Until != nil {
			rrule += ";UNTIL=" + rr.Until.Format("20060102T150405Z")
		}
		if len(rr.ByDay) > 0 {
			rrule += ";BYDAY=" + strings.Join(rr.ByDay, ",")
		}
		buf.WriteString(rrule + "\r\n")
	}

	// Reminders
	for _, r := range evt.Reminders {
		buf.WriteString("BEGIN:VALARM\r\n")
		buf.WriteString("TRIGGER:-PT" + strconv.Itoa(r.Minutes) + "M\r\n")
		buf.WriteString("ACTION:DISPLAY\r\n")
		buf.WriteString("DESCRIPTION:Reminder\r\n")
		buf.WriteString("END:VALARM\r\n")
	}

	buf.WriteString("END:VEVENT\r\n")
}

// parseICS 解析 ICS 文本为事件列表.
func parseICS(content string, calendarID string) []*Event {
	var events []*Event
	lines := strings.Split(content, "\n")

	var current *Event
	inEvent := false
	inAlarm := false
	triggerMinutes := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimRight(line, "\r")

		switch {
		case line == "BEGIN:VEVENT":
			inEvent = true
			current = &Event{
				CalendarID: calendarID,
				Status:     StatusConfirmed,
			}
		case line == "END:VEVENT":
			if current != nil {
				events = append(events, current)
			}
			current = nil
			inEvent = false
		case line == "BEGIN:VALARM":
			inAlarm = true
		case line == "END:VALARM":
			if inAlarm && current != nil && triggerMinutes > 0 {
				current.Reminders = append(current.Reminders, Reminder{
					ID:      uuid.New().String(),
					Minutes: triggerMinutes,
					Method:  "notification",
				})
			}
			inAlarm = false
			triggerMinutes = 0
		case inAlarm && strings.HasPrefix(line, "TRIGGER:"):
			triggerMinutes = parseTriggerMinutes(line[len("TRIGGER:"):])
		case inEvent && current != nil:
			parseICSField(current, line)
		}
	}

	return events
}

// parseICSField 解析单个 ICS 字段.
func parseICSField(evt *Event, line string) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return
	}
	key := parts[0]
	value := parts[1]

	// 处理带参数的 key，如 DTSTART;VALUE=DATE
	baseKey := strings.SplitN(key, ";", 2)[0]

	switch baseKey {
	case "UID":
		evt.ID = value
	case "SUMMARY":
		evt.Title = unescapeICS(value)
	case "DESCRIPTION":
		evt.Description = unescapeICS(value)
	case "LOCATION":
		evt.Location = unescapeICS(value)
	case "DTSTART":
		if strings.Contains(key, "VALUE=DATE") {
			evt.AllDay = true
			if t, err := time.Parse("20060102", value); err == nil {
				evt.StartTime = t
			}
		} else {
			evt.StartTime = parseICSDateTime(value)
		}
	case "DTEND":
		if strings.Contains(key, "VALUE=DATE") {
			if t, err := time.Parse("20060102", value); err == nil {
				evt.EndTime = t
			}
		} else {
			evt.EndTime = parseICSDateTime(value)
		}
	case "STATUS":
		switch strings.ToUpper(value) {
		case "CONFIRMED":
			evt.Status = StatusConfirmed
		case "TENTATIVE":
			evt.Status = StatusTentative
		case "CANCELLED":
			evt.Status = StatusCancelled
		}
	case "RRULE":
		evt.Recurrence = parseRRULE(value)
	}
}

// parseICSDateTime 解析 ICS 日期时间.
func parseICSDateTime(s string) time.Time {
	s = strings.TrimRight(s, "Z")
	formats := []string{
		"20060102T150405",
		"20060102T150405Z",
		"20060102",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// parseRRULE 解析 RRULE 字符串.
func parseRRULE(s string) *RecurrenceRule {
	rule := &RecurrenceRule{Interval: 1}
	pairs := strings.Split(s, ";")

	freqMap := map[string]RecurrenceFrequency{
		"DAILY":   FreqDaily,
		"WEEKLY":  FreqWeekly,
		"MONTHLY": FreqMonthly,
		"YEARLY":  FreqYearly,
	}

	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.ToUpper(kv[0]) {
		case "FREQ":
			if f, ok := freqMap[strings.ToUpper(kv[1])]; ok {
				rule.Frequency = f
			}
		case "INTERVAL":
			if v, err := strconv.Atoi(kv[1]); err == nil {
				rule.Interval = v
			}
		case "COUNT":
			if v, err := strconv.Atoi(kv[1]); err == nil {
				rule.Count = v
			}
		case "UNTIL":
			if t, err := time.Parse("20060102T150405Z", kv[1]); err == nil {
				rule.Until = &t
			} else if t, err := time.Parse("20060102", kv[1]); err == nil {
				rule.Until = &t
			}
		case "BYDAY":
			rule.ByDay = strings.Split(kv[1], ",")
		}
	}

	return rule
}

// parseTriggerMinutes 解析 VALARM TRIGGER 字段为分钟数.
func parseTriggerMinutes(s string) int {
	s = strings.TrimSpace(s)
	// 格式: -PT5M, -PT15M, -PT1H, -P1D
	s = strings.TrimPrefix(s, "-")
	s = strings.TrimPrefix(s, "PT")
	s = strings.TrimPrefix(s, "P")

	if strings.HasSuffix(s, "M") {
		s = strings.TrimSuffix(s, "M")
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
	}
	if strings.HasSuffix(s, "H") {
		s = strings.TrimSuffix(s, "H")
		if v, err := strconv.Atoi(s); err == nil {
			return v * 60
		}
	}
	if strings.HasSuffix(s, "D") {
		s = strings.TrimSuffix(s, "D")
		if v, err := strconv.Atoi(s); err == nil {
			return v * 1440
		}
	}
	return 0
}

// escapeICS 转义 ICS 特殊字符.
func escapeICS(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// unescapeICS 还原 ICS 转义字符（逐字符解析，避免误匹配）.
func unescapeICS(s string) string {
	var buf bytes.Buffer
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			next := runes[i+1]
			switch next {
			case 'n', 'N':
				buf.WriteRune('\n')
				i++
				continue
			case ',':
				buf.WriteRune(',')
				i++
				continue
			case ';':
				buf.WriteRune(';')
				i++
				continue
			case '\\':
				buf.WriteRune('\\')
				i++
				continue
			}
		}
		buf.WriteRune(runes[i])
	}
	return buf.String()
}

// generateReminderID 生成提醒 ID.
func generateReminderID() string {
	return fmt.Sprintf("r-%s", uuid.New().String()[:8])
}
