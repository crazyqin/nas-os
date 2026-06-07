// Package digitalwellbeing 提供数字健康核心逻辑
package digitalwellbeing

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 数字健康管理器
type Manager struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	members       map[string]*FamilyMember
	screenTimes   map[string][]*ScreenTime // userID -> screen times
	focusSessions map[string]*FocusSession
	schedules     map[string]*DowntimeSchedule
	limits        map[string]*AppLimit
}

// NewManager 创建数字健康管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		logger:        logger,
		members:       make(map[string]*FamilyMember),
		screenTimes:   make(map[string][]*ScreenTime),
		focusSessions: make(map[string]*FocusSession),
		schedules:     make(map[string]*DowntimeSchedule),
		limits:        make(map[string]*AppLimit),
	}
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetScreenTime 获取屏幕时间
func (m *Manager) GetScreenTime(userID, date string) (*ScreenTime, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	times, ok := m.screenTimes[userID]
	if !ok {
		return nil, fmt.Errorf("no screen time data for user %s", userID)
	}

	for _, st := range times {
		if st.Date == date {
			return st, nil
		}
	}

	return nil, fmt.Errorf("no screen time data for date %s", date)
}

// GetScreenTimeRange 获取时间段内的屏幕时间
func (m *Manager) GetScreenTimeRange(userID, startDate, endDate string) ([]*ScreenTime, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	times, ok := m.screenTimes[userID]
	if !ok {
		return nil, fmt.Errorf("no screen time data for user %s", userID)
	}

	var result []*ScreenTime
	for _, st := range times {
		if st.Date >= startDate && st.Date <= endDate {
			result = append(result, st)
		}
	}
	return result, nil
}

// AnalyzePatterns 分析使用模式
func (m *Manager) AnalyzePatterns(userID, period string) (*UsagePattern, error) {
	m.mu.RLock()
	times, ok := m.screenTimes[userID]
	m.mu.RUnlock()

	if !ok || len(times) == 0 {
		return nil, fmt.Errorf("insufficient data for user %s", userID)
	}

	// 计算平均使用时间
	totalMinutes := 0
	for _, st := range times {
		totalMinutes += st.TotalMinutes
	}
	avgMinutes := totalMinutes / len(times)

	// 分析趋势
	var trend string
	var trendPercent float64
	if len(times) >= 7 {
		recentAvg := 0
		oldAvg := 0
		half := len(times) / 2
		for i, st := range times {
			if i < half {
				oldAvg += st.TotalMinutes
			} else {
				recentAvg += st.TotalMinutes
			}
		}
		oldAvg /= half
		recentAvg /= (len(times) - half)

		if recentAvg > oldAvg {
			trend = "increasing"
			trendPercent = float64(recentAvg-oldAvg) / float64(oldAvg) * 100
		} else if recentAvg < oldAvg {
			trend = "decreasing"
			trendPercent = float64(oldAvg-recentAvg) / float64(oldAvg) * 100
		} else {
			trend = "stable"
		}
	}

	// 生成洞察
	var insights []Insight
	if avgMinutes > 240 {
		insights = append(insights, Insight{
			Type:    "warning",
			Title:   "使用时间较长",
			Message: fmt.Sprintf("您的日均屏幕时间为 %d 分钟，建议适当休息", avgMinutes),
		})
	}

	pattern := &UsagePattern{
		ID:             generateID(),
		UserID:         userID,
		Period:         period,
		AverageMinutes: avgMinutes,
		PeakHour:       20, // 模拟数据
		PeakDay:        "saturday",
		Trend:          trend,
		TrendPercent:   trendPercent,
		TopApps: []AppUsage{
			{AppName: "微信", Category: "social", Minutes: 60},
			{AppName: "抖音", Category: "entertainment", Minutes: 45},
			{AppName: "Safari", Category: "productivity", Minutes: 30},
		},
		Insights: insights,
	}

	return pattern, nil
}

// StartFocus 开始专注会话
func (m *Manager) StartFocus(userID, name string, durationMin int, blockedApps []string) (*FocusSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已有活跃的专注会话
	for _, session := range m.focusSessions {
		if session.UserID == userID && session.Status == FocusStatusActive {
			return nil, fmt.Errorf("user %s already has an active focus session", userID)
		}
	}

	session := &FocusSession{
		ID:          generateID(),
		UserID:      userID,
		Name:        name,
		StartAt:     time.Now(),
		DurationMin: durationMin,
		Status:      FocusStatusActive,
		BlockedApps: blockedApps,
		AllowCalls:  false,
		AllowNotifs: false,
	}

	m.focusSessions[session.ID] = session

	m.logger.Info("focus session started",
		zap.String("user_id", userID),
		zap.String("session_id", session.ID),
		zap.Int("duration", durationMin),
	)

	return session, nil
}

// StopFocus 停止专注会话
func (m *Manager) StopFocus(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.focusSessions[sessionID]
	if !ok {
		return fmt.Errorf("focus session %s not found", sessionID)
	}

	if session.Status != FocusStatusActive {
		return fmt.Errorf("focus session %s is not active", sessionID)
	}

	session.Status = FocusStatusCompleted
	session.EndAt = time.Now()
	session.ActualMin = int(time.Since(session.StartAt).Minutes())

	return nil
}

// GetFocusSession 获取专注会话
func (m *Manager) GetFocusSession(sessionID string) (*FocusSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.focusSessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("focus session %s not found", sessionID)
	}
	return session, nil
}

// ListFocusSessions 获取用户的专注会话列表
func (m *Manager) ListFocusSessions(userID string) []*FocusSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*FocusSession
	for _, session := range m.focusSessions {
		if session.UserID == userID {
			result = append(result, session)
		}
	}
	return result
}

// ListMembers 获取家庭成员列表
func (m *Manager) ListMembers() []*FamilyMember {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*FamilyMember, 0, len(m.members))
	for _, member := range m.members {
		result = append(result, member)
	}
	return result
}

// AddMember 添加家庭成员
func (m *Manager) AddMember(name, role, ageGroup string) *FamilyMember {
	m.mu.Lock()
	defer m.mu.Unlock()

	member := &FamilyMember{
		ID:        generateID(),
		Name:      name,
		Role:      role,
		AgeGroup:  ageGroup,
		CreatedAt: time.Now(),
	}
	m.members[member.ID] = member
	return member
}

// UpdateMember 更新家庭成员
func (m *Manager) UpdateMember(id string, name, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	member, ok := m.members[id]
	if !ok {
		return fmt.Errorf("member %s not found", id)
	}

	if name != "" {
		member.Name = name
	}
	if role != "" {
		member.Role = role
	}
	return nil
}

// RemoveMember 移除家庭成员
func (m *Manager) RemoveMember(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.members[id]; !ok {
		return fmt.Errorf("member %s not found", id)
	}
	delete(m.members, id)
	return nil
}

// GetReport 获取健康报告
func (m *Manager) GetReport(userID, period, startDate, endDate string) (*WellbeingReport, error) {
	m.mu.RLock()
	times, ok := m.screenTimes[userID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no data for user %s", userID)
	}

	// 过滤日期范围
	var filteredPtrs []*ScreenTime
	totalMinutes := 0
	totalPickups := 0
	for _, st := range times {
		if st.Date >= startDate && st.Date <= endDate {
			filteredPtrs = append(filteredPtrs, st)
			totalMinutes += st.TotalMinutes
			totalPickups += st.PickupCount
		}
	}

	if len(filteredPtrs) == 0 {
		return nil, fmt.Errorf("no data in specified date range")
	}

	avgDaily := totalMinutes / len(filteredPtrs)
	avgPickups := totalPickups / len(filteredPtrs)

	// 转换为值类型切片
	filtered := make([]ScreenTime, len(filteredPtrs))
	for i, st := range filteredPtrs {
		filtered[i] = *st
	}

	// 生成建议
	var suggestions []string
	if avgDaily > 240 {
		suggestions = append(suggestions, "建议减少屏幕时间，每小时休息 5-10 分钟")
	}
	if avgPickups > 50 {
		suggestions = append(suggestions, "频繁查看手机可能影响专注力，建议开启专注模式")
	}

	report := &WellbeingReport{
		ID:        generateID(),
		UserID:    userID,
		Period:    period,
		StartDate: startDate,
		EndDate:   endDate,
		Summary: ReportSummary{
			TotalMinutes:     totalMinutes,
			AverageDaily:     avgDaily,
			MostUsedApp:      "微信",
			MostUsedCategory: "social",
			TotalPickups:     totalPickups,
			AveragePickups:   avgPickups,
			ComparedToLast:   -5.2,
		},
		DailyData: filtered,
		TopApps: []AppUsage{
			{AppName: "微信", Category: "social", Minutes: 60},
			{AppName: "抖音", Category: "entertainment", Minutes: 45},
		},
		Suggestions: suggestions,
	}

	return report, nil
}

// SetDowntimeSchedule 设置停机时间计划
func (m *Manager) SetDowntimeSchedule(userID string, schedule *DowntimeSchedule) {
	m.mu.Lock()
	defer m.mu.Unlock()

	schedule.ID = generateID()
	schedule.UserID = userID
	m.schedules[schedule.ID] = schedule
}

// GetDowntimeSchedule 获取停机时间计划
func (m *Manager) GetDowntimeSchedule(userID string) *DowntimeSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, schedule := range m.schedules {
		if schedule.UserID == userID {
			return schedule
		}
	}
	return nil
}

// SetAppLimit 设置应用限制
func (m *Manager) SetAppLimit(userID, appName, appID string, dailyMin int) *AppLimit {
	m.mu.Lock()
	defer m.mu.Unlock()

	limit := &AppLimit{
		ID:       generateID(),
		UserID:   userID,
		AppName:  appName,
		AppID:    appID,
		DailyMin: dailyMin,
		Enabled:  true,
	}
	m.limits[limit.ID] = limit
	return limit
}

// GetAppLimits 获取应用限制列表
func (m *Manager) GetAppLimits(userID string) []*AppLimit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*AppLimit
	for _, limit := range m.limits {
		if limit.UserID == userID {
			result = append(result, limit)
		}
	}
	return result
}
