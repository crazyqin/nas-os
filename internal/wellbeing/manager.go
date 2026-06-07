// Package wellbeing 提供数字健康助手管理器
package wellbeing

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 数字健康管理器
type Manager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	sessions  map[string]*UsageSession
	reminders map[string]*BreakReminder
	limits    map[string]*UsageLimit
	reports   map[string]*WellnessReport
	insights  map[string]*WellnessInsight
	stopChan  chan struct{}
	running   bool
}

// NewManager 创建数字健康管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:    logger,
		sessions:  make(map[string]*UsageSession),
		reminders: make(map[string]*BreakReminder),
		limits:    make(map[string]*UsageLimit),
		reports:   make(map[string]*WellnessReport),
		insights:  make(map[string]*WellnessInsight),
		stopChan:  make(chan struct{}),
	}

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("wb-%d", time.Now().UnixNano())
}

// TrackUsage 开始追踪使用会话
func (m *Manager) TrackUsage(userID string, sessionType SessionType, appName string) (*UsageSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &UsageSession{
		ID:        generateID(),
		UserID:    userID,
		Type:      sessionType,
		AppName:   appName,
		Status:    StatusActive,
		StartTime: time.Now(),
		CreatedAt: time.Now(),
	}

	m.sessions[session.ID] = session
	m.logger.Info("usage tracking started",
		zap.String("session_id", session.ID),
		zap.String("user_id", userID),
		zap.String("app", appName))

	return session, nil
}

// PauseTracking 暂停追踪
func (m *Manager) PauseTracking(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != StatusActive {
		return fmt.Errorf("session is not active")
	}

	session.Status = StatusPaused
	session.Duration += time.Since(session.StartTime)

	return nil
}

// ResumeTracking 恢复追踪
func (m *Manager) ResumeTracking(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != StatusPaused {
		return fmt.Errorf("session is not paused")
	}

	session.Status = StatusActive
	session.StartTime = time.Now()

	return nil
}

// EndTracking 结束追踪
func (m *Manager) EndTracking(sessionID string) (*UsageSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status == StatusEnded {
		return nil, fmt.Errorf("session already ended")
	}

	now := time.Now()
	session.EndTime = &now
	session.Status = StatusEnded
	if session.Status == StatusActive {
		session.Duration += time.Since(session.StartTime)
	}

	m.logger.Info("usage tracking ended",
		zap.String("session_id", sessionID),
		zap.Duration("duration", session.Duration))

	return session, nil
}

// GetActiveSessions 获取活跃会话
func (m *Manager) GetActiveSessions(userID string) []*UsageSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*UsageSession, 0)
	for _, s := range m.sessions {
		if s.UserID == userID && s.Status == StatusActive {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

// GetSessionHistory 获取会话历史
func (m *Manager) GetSessionHistory(userID string, limit int) []*UsageSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*UsageSession, 0)
	for _, s := range m.sessions {
		if s.UserID == userID && s.Status == StatusEnded {
			sessions = append(sessions, s)
		}
	}

	// 按时间倒序
	for i := 0; i < len(sessions)-1; i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[j].EndTime.After(*sessions[i].EndTime) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	if limit > 0 && limit < len(sessions) {
		sessions = sessions[:limit]
	}

	return sessions
}

// SetReminder 设置休息提醒
func (m *Manager) SetReminder(userID string, req *CreateReminderRequest) (*BreakReminder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	nextTrigger := now.Add(time.Duration(req.IntervalMinutes) * time.Minute)

	reminder := &BreakReminder{
		ID:              generateID(),
		UserID:          userID,
		Type:            req.Type,
		Title:           req.Title,
		Message:         req.Message,
		IntervalMinutes: req.IntervalMinutes,
		DurationMinutes: req.DurationMinutes,
		Status:          ReminderActive,
		Enabled:         true,
		SnoozeMinutes:   req.SnoozeMinutes,
		NextTrigger:     &nextTrigger,
		Sound:           req.Sound,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	m.reminders[reminder.ID] = reminder
	m.logger.Info("reminder set",
		zap.String("reminder_id", reminder.ID),
		zap.String("type", string(reminder.Type)),
		zap.Int("interval", reminder.IntervalMinutes))

	return reminder, nil
}

// GetReminder 获取提醒
func (m *Manager) GetReminder(reminderID string) (*BreakReminder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reminder, ok := m.reminders[reminderID]
	if !ok {
		return nil, fmt.Errorf("reminder not found: %s", reminderID)
	}
	return reminder, nil
}

// ListReminders 列出用户的所有提醒
func (m *Manager) ListReminders(userID string) []*BreakReminder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reminders := make([]*BreakReminder, 0)
	for _, r := range m.reminders {
		if r.UserID == userID {
			reminders = append(reminders, r)
		}
	}
	return reminders
}

// UpdateReminder 更新提醒
func (m *Manager) UpdateReminder(reminderID string, req *UpdateReminderRequest) (*BreakReminder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	reminder, ok := m.reminders[reminderID]
	if !ok {
		return nil, fmt.Errorf("reminder not found: %s", reminderID)
	}

	if req.Title != "" {
		reminder.Title = req.Title
	}
	if req.Message != "" {
		reminder.Message = req.Message
	}
	if req.IntervalMinutes > 0 {
		reminder.IntervalMinutes = req.IntervalMinutes
	}
	if req.DurationMinutes > 0 {
		reminder.DurationMinutes = req.DurationMinutes
	}
	if req.SnoozeMinutes > 0 {
		reminder.SnoozeMinutes = req.SnoozeMinutes
	}
	if req.Enabled != nil {
		reminder.Enabled = *req.Enabled
	}
	if req.Sound != "" {
		reminder.Sound = req.Sound
	}
	reminder.UpdatedAt = time.Now()

	return reminder, nil
}

// DeleteReminder 删除提醒
func (m *Manager) DeleteReminder(reminderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.reminders[reminderID]; !ok {
		return fmt.Errorf("reminder not found: %s", reminderID)
	}

	delete(m.reminders, reminderID)
	return nil
}

// TriggerReminder 触发提醒
func (m *Manager) TriggerReminder(reminderID string) (*BreakReminder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	reminder, ok := m.reminders[reminderID]
	if !ok {
		return nil, fmt.Errorf("reminder not found: %s", reminderID)
	}

	now := time.Now()
	reminder.Status = ReminderTriggered
	reminder.LastTriggered = &now
	nextTrigger := now.Add(time.Duration(reminder.IntervalMinutes) * time.Minute)
	reminder.NextTrigger = &nextTrigger
	reminder.UpdatedAt = now

	return reminder, nil
}

// SnoozeReminder 贪睡提醒
func (m *Manager) SnoozeReminder(reminderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	reminder, ok := m.reminders[reminderID]
	if !ok {
		return fmt.Errorf("reminder not found: %s", reminderID)
	}

	reminder.Status = ReminderSnoozed
	now := time.Now()
	nextTrigger := now.Add(time.Duration(reminder.SnoozeMinutes) * time.Minute)
	reminder.NextTrigger = &nextTrigger
	reminder.UpdatedAt = now

	return nil
}

// DismissReminder 忽略提醒
func (m *Manager) DismissReminder(reminderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	reminder, ok := m.reminders[reminderID]
	if !ok {
		return fmt.Errorf("reminder not found: %s", reminderID)
	}

	reminder.Status = ReminderDismissed
	now := time.Now()
	nextTrigger := now.Add(time.Duration(reminder.IntervalMinutes) * time.Minute)
	reminder.NextTrigger = &nextTrigger
	reminder.UpdatedAt = now

	return nil
}

// SetUsageLimit 设置使用限制
func (m *Manager) SetUsageLimit(userID string, req *CreateUsageLimitRequest) (*UsageLimit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	warningAt := req.WarningAt
	if warningAt == 0 {
		warningAt = 80
	}

	limit := &UsageLimit{
		ID:           generateID(),
		UserID:       userID,
		AppName:      req.AppName,
		DailyLimit:   req.DailyLimit,
		Enabled:      true,
		CurrentUsed:  0,
		WarningAt:    warningAt,
		BlockAtLimit: req.BlockAtLimit,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.limits[limit.ID] = limit
	m.logger.Info("usage limit set",
		zap.String("limit_id", limit.ID),
		zap.String("app", limit.AppName),
		zap.Int("daily_limit", limit.DailyLimit))

	return limit, nil
}

// GetUsageLimits 获取用户的使用限制
func (m *Manager) GetUsageLimits(userID string) []*UsageLimit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limits := make([]*UsageLimit, 0)
	for _, l := range m.limits {
		if l.UserID == userID {
			limits = append(limits, l)
		}
	}
	return limits
}

// UpdateUsageLimit 更新使用限制
func (m *Manager) UpdateUsageLimit(limitID string, req *UpdateUsageLimitRequest) (*UsageLimit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	limit, ok := m.limits[limitID]
	if !ok {
		return nil, fmt.Errorf("usage limit not found: %s", limitID)
	}

	if req.DailyLimit != nil {
		limit.DailyLimit = *req.DailyLimit
	}
	if req.Enabled != nil {
		limit.Enabled = *req.Enabled
	}
	if req.WarningAt != nil {
		limit.WarningAt = *req.WarningAt
	}
	if req.BlockAtLimit != nil {
		limit.BlockAtLimit = *req.BlockAtLimit
	}
	limit.UpdatedAt = time.Now()

	return limit, nil
}

// DeleteUsageLimit 删除使用限制
func (m *Manager) DeleteUsageLimit(limitID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.limits[limitID]; !ok {
		return fmt.Errorf("usage limit not found: %s", limitID)
	}

	delete(m.limits, limitID)
	return nil
}

// CheckUsageLimit 检查使用限制
func (m *Manager) CheckUsageLimit(userID, appName string) (bool, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, limit := range m.limits {
		if limit.UserID == userID && limit.AppName == appName && limit.Enabled {
			percentUsed := float64(limit.CurrentUsed) / float64(limit.DailyLimit) * 100

			if percentUsed >= 100 && limit.BlockAtLimit {
				return true, "blocked", nil
			}
			if percentUsed >= float64(limit.WarningAt) {
				return true, "warning", nil
			}
		}
	}

	return false, "", nil
}

// GenerateReport 生成健康报告
func (m *Manager) GenerateReport(req *ReportRequest) (*WellnessReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 计算日期范围
	endDate := time.Now()
	startDate := endDate

	switch req.Period {
	case "daily":
		startDate = endDate.AddDate(0, 0, -1)
	case "weekly":
		startDate = endDate.AddDate(0, 0, -7)
	case "monthly":
		startDate = endDate.AddDate(0, -1, 0)
	}

	// 收集使用数据
	totalMinutes := 0
	appUsageMap := make(map[string]int)

	for _, session := range m.sessions {
		if session.UserID == req.UserID &&
			session.Status == StatusEnded &&
			session.EndTime != nil &&
			session.EndTime.After(startDate) &&
			session.EndTime.Before(endDate) {

			minutes := int(session.Duration.Minutes())
			totalMinutes += minutes
			appUsageMap[session.AppName] += minutes
		}
	}

	// 生成应用使用统计
	topApps := make([]AppUsage, 0)
	for app, minutes := range appUsageMap {
		percentage := 0.0
		if totalMinutes > 0 {
			percentage = float64(minutes) / float64(totalMinutes) * 100
		}
		topApps = append(topApps, AppUsage{
			AppName:    app,
			Minutes:    minutes,
			Percentage: percentage,
		})
	}

	// 生成洞察
	insights := m.generateInsights(req.UserID, totalMinutes, topApps)

	// 计算健康分数 (0-100)
	score := calculateWellnessScore(totalMinutes, len(topApps))

	report := &WellnessReport{
		ID:              generateID(),
		UserID:          req.UserID,
		Period:          req.Period,
		StartDate:       startDate,
		EndDate:         endDate,
		TotalScreenTime: totalMinutes,
		AvgDailyMinutes: totalMinutes / max(1, daysBetween(startDate, endDate)),
		TopApps:         topApps,
		Insights:        insights,
		Score:           score,
		CreatedAt:       time.Now(),
	}

	m.reports[report.ID] = report
	m.logger.Info("wellness report generated",
		zap.String("report_id", report.ID),
		zap.String("user_id", req.UserID),
		zap.Int("score", score))

	return report, nil
}

// generateInsights 生成洞察
func (m *Manager) generateInsights(userID string, totalMinutes int, topApps []AppUsage) []WellnessInsight {
	insights := make([]WellnessInsight, 0)

	// 使用时间过长提醒
	if totalMinutes > 480 { // 8小时
		insights = append(insights, WellnessInsight{
			ID:        generateID(),
			UserID:    userID,
			Type:      InsightUsageIncrease,
			Title:     "屏幕时间较长",
			Message:   fmt.Sprintf("您今天的屏幕时间为 %d 分钟，建议适当休息。", totalMinutes),
			Priority:  1,
			CreatedAt: time.Now(),
		})
	}

	// 休息提醒建议
	insights = append(insights, WellnessInsight{
		ID:        generateID(),
		UserID:    userID,
		Type:      InsightSuggestion,
		Title:     "定期休息",
		Message:   "建议每 45 分钟休息 5 分钟，保护眼睛和颈椎。",
		Priority:  2,
		CreatedAt: time.Now(),
	})

	return insights
}

// GetInsights 获取健康洞察
func (m *Manager) GetInsights(userID string) []WellnessInsight {
	m.mu.RLock()
	defer m.mu.RUnlock()

	insights := make([]WellnessInsight, 0)
	for _, i := range m.insights {
		if i.UserID == userID {
			insights = append(insights, *i)
		}
	}
	return insights
}

// MarkInsightRead 标记洞察已读
func (m *Manager) MarkInsightRead(insightID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	insight, ok := m.insights[insightID]
	if !ok {
		return fmt.Errorf("insight not found: %s", insightID)
	}

	insight.Read = true
	return nil
}

// GetScreenTime 获取屏幕时间统计
func (m *Manager) GetScreenTime(userID, date string) (*ScreenTime, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targetDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}

	appUsageMap := make(map[string]int)
	hourlyUsage := make([]HourlyUsage, 24)
	totalMinutes := 0
	breaksTaken := 0
	firstActive := time.Time{}
	lastActive := time.Time{}

	for _, session := range m.sessions {
		if session.UserID == userID && session.Status == StatusEnded && session.EndTime != nil {
			if session.EndTime.Format("2006-01-02") == date {
				minutes := int(session.Duration.Minutes())
				totalMinutes += minutes
				appUsageMap[session.AppName] += minutes

				// 统计每小时使用
				hour := session.StartTime.Hour()
				hourlyUsage[hour].Hour = hour
				hourlyUsage[hour].Minutes += minutes

				if firstActive.IsZero() || session.StartTime.Before(firstActive) {
					firstActive = session.StartTime
				}
				if lastActive.IsZero() || session.EndTime.After(lastActive) {
					lastActive = *session.EndTime
				}
			}
		}
	}

	appUsage := make([]AppUsage, 0)
	for app, minutes := range appUsageMap {
		appUsage = append(appUsage, AppUsage{
			AppName:    app,
			Minutes:    minutes,
			Percentage: float64(minutes) / float64(max(1, totalMinutes)) * 100,
		})
	}

	_ = targetDate // 用于未来扩展

	screenTime := &ScreenTime{
		UserID:       userID,
		Date:         date,
		TotalMinutes: totalMinutes,
		AppUsage:     appUsage,
		HourlyUsage:  hourlyUsage,
		BreaksTaken:  breaksTaken,
		BreaksMissed: 0,
		FirstActive:  firstActive,
		LastActive:   lastActive,
	}

	return screenTime, nil
}

// calculateWellnessScore 计算健康分数
func calculateWellnessScore(totalMinutes int, appCount int) int {
	// 基础分 100
	score := 100

	// 超过 4 小时开始扣分
	if totalMinutes > 240 {
		overtime := totalMinutes - 240
		score -= overtime / 30 * 5
	}

	// 应用种类过多扣分
	if appCount > 10 {
		score -= (appCount - 10) * 2
	}

	if score < 0 {
		score = 0
	}
	return score
}

// daysBetween 计算两个日期之间的天数
func daysBetween(a, b time.Time) int {
	if b.After(a) {
		a, b = b, a
	}
	return int(a.Sub(b).Hours() / 24)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
