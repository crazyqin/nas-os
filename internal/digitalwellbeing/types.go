// Package digitalwellbeing provides digital wellness features for NAS-OS
// Features: Screen time tracking, usage statistics, parental controls, focus mode
// Competitor benchmark: 对标Apple Screen Time, Google Digital Wellbeing
package digitalwellbeing

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AppCategory represents application category
type AppCategory string

const (
	CategoryProductivity AppCategory = "productivity"
	CategoryEntertainment AppCategory = "entertainment"
	CategorySocial       AppCategory = "social"
	CategoryGaming       AppCategory = "gaming"
	CategoryEducation    AppCategory = "education"
	CategoryHealth       AppCategory = "health"
	CategoryOther        AppCategory = "other"
)

// UsageRecord represents app usage record
type UsageRecord struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	AppName   string      `json:"app_name"`
	Category  AppCategory `json:"category"`
	Duration  time.Duration `json:"duration"`
	StartTime time.Time   `json:"start_time"`
	EndTime   time.Time   `json:"end_time"`
	Device    string      `json:"device"`
}

// DailyUsage represents daily usage summary
type DailyUsage struct {
	Date          time.Time                `json:"date"`
	UserID        string                   `json:"user_id"`
	TotalTime     time.Duration            `json:"total_time"`
	ByCategory    map[AppCategory]time.Duration `json:"by_category"`
	ByApp         map[string]time.Duration `json:"by_app"`
	ScreenUnlocks int                      `json:"screen_unlocks"`
	FirstPickup   time.Time                `json:"first_pickup"`
	LastPutDown   time.Time                `json:"last_put_down"`
}

// UsageGoal represents a usage goal
type UsageGoal struct {
	ID        string        `json:"id"`
	UserID    string        `json:"user_id"`
	Category  AppCategory   `json:"category,omitempty"`
	AppName   string        `json:"app_name,omitempty"`
	DailyMax  time.Duration `json:"daily_max"`
	Enabled   bool          `json:"enabled"`
	CreatedAt time.Time     `json:"created_at"`
}

// FocusMode represents focus mode configuration
type FocusMode struct {
	ID          string        `json:"id"`
	UserID      string        `json:"user_id"`
	Name        string        `json:"name"`
	Enabled     bool          `json:"enabled"`
	BlockedApps []string      `json:"blocked_apps"`
	AllowedApps []string      `json:"allowed_apps"`
	Schedule    *Schedule     `json:"schedule,omitempty"`
	DNDMode     bool          `json:"dnd_mode"`
	CreatedAt   time.Time     `json:"created_at"`
}

// Schedule represents a time schedule
type Schedule struct {
	StartTime string   `json:"start_time"` // HH:MM
	EndTime   string   `json:"end_time"`   // HH:MM
	Days      []string `json:"days"`       // mon, tue, wed, thu, fri, sat, sun
}

// ParentalControl represents parental control settings
type ParentalControl struct {
	UserID         string        `json:"user_id"`
	DailyLimit     time.Duration `json:"daily_limit"`
	BedtimeStart   string        `json:"bedtime_start"` // HH:MM
	BedtimeEnd     string        `json:"bedtime_end"`   // HH:MM
	BlockedContent []string      `json:"blocked_content"`
	RequireApproval bool         `json:"require_approval"`
	AllowanceType  string        `json:"allowance_type"` // fixed, earn
	WeeklyBudget   time.Duration `json:"weekly_budget"`
}

// WellnessReport represents a wellness report
type WellnessReport struct {
	UserID      string        `json:"user_id"`
	Period      string        `json:"period"` // daily, weekly, monthly
	TotalTime   time.Duration `json:"total_time"`
	AverageDaily time.Duration `json:"average_daily"`
	TopApps     []AppUsage    `json:"top_apps"`
	GoalsMet    int           `json:"goals_met"`
	GoalsTotal  int           `json:"goals_total"`
	Suggestions []string      `json:"suggestions"`
	GeneratedAt time.Time     `json:"generated_at"`
}

// AppUsage represents app usage statistics
type AppUsage struct {
	AppName  string        `json:"app_name"`
	Category AppCategory   `json:"category"`
	Duration time.Duration `json:"duration"`
	Percent  float64       `json:"percent"`
}

// AlertType represents wellness alert type
type AlertType string

const (
	AlertGoalExceeded  AlertType = "goal_exceeded"
	AlertBedtime       AlertType = "bedtime"
	AlertBreakReminder AlertType = "break_reminder"
	AlertWeeklyReport  AlertType = "weekly_report"
)

// WellnessAlert represents a wellness alert
type WellnessAlert struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      AlertType `json:"type"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// Config represents digital wellbeing configuration
type Config struct {
	Enabled          bool `json:"enabled"`
	TrackingEnabled  bool `json:"tracking_enabled"`
	AlertsEnabled    bool `json:"alerts_enabled"`
	BreakReminder    int  `json:"break_reminder"`    // minutes
	BreakDuration    int  `json:"break_duration"`    // minutes
	DataRetention    int  `json:"data_retention"`    // days
}

// Manager manages digital wellbeing features
type Manager struct {
	config    *Config
	records   []*UsageRecord
	goals     map[string]*UsageGoal
	focusModes map[string]*FocusMode
	parental  map[string]*ParentalControl
	alerts    []*WellnessAlert
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a new digital wellbeing manager
func NewManager(config *Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:     config,
		records:    make([]*UsageRecord, 0),
		goals:      make(map[string]*UsageGoal),
		focusModes: make(map[string]*FocusMode),
		parental:   make(map[string]*ParentalControl),
		alerts:     make([]*WellnessAlert, 0),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the digital wellbeing manager
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return nil
	}
	
	// Start usage tracking
	if m.config.TrackingEnabled {
		go m.trackUsage()
	}
	
	// Start alert checker
	if m.config.AlertsEnabled {
		go m.checkAlerts()
	}
	
	// Start data cleanup
	go m.cleanupOldData()
	
	return nil
}

// Stop stops the digital wellbeing manager
func (m *Manager) Stop() {
	m.cancel()
}

// trackUsage tracks app usage
func (m *Manager) trackUsage() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			// Track active app usage
		}
	}
}

// checkAlerts checks for wellness alerts
func (m *Manager) checkAlerts() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.evaluateAlerts()
		}
	}
}

// evaluateAlerts evaluates wellness alerts
func (m *Manager) evaluateAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check goals
	for _, goal := range m.goals {
		if !goal.Enabled {
			continue
		}
		usage := m.getAppUsageToday(goal.UserID, goal.AppName, goal.Category)
		if usage > goal.DailyMax {
			m.createAlert(goal.UserID, AlertGoalExceeded, 
				fmt.Sprintf("已超过 %s 的每日使用限制", goal.AppName))
		}
	}
}

// getAppUsageToday gets today's usage for an app/category
func (m *Manager) getAppUsageToday(userID, appName string, category AppCategory) time.Duration {
	total := time.Duration(0)
	today := time.Now().Truncate(24 * time.Hour)
	
	for _, record := range m.records {
		if record.UserID != userID {
			continue
		}
		if record.StartTime.Before(today) {
			continue
		}
		if appName != "" && record.AppName != appName {
			continue
		}
		if category != "" && record.Category != category {
			continue
		}
		total += record.Duration
	}
	
	return total
}

// createAlert creates a wellness alert
func (m *Manager) createAlert(userID string, alertType AlertType, message string) {
	alert := &WellnessAlert{
		ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		UserID:    userID,
		Type:      alertType,
		Message:   message,
		CreatedAt: time.Now(),
	}
	m.alerts = append(m.alerts, alert)
}

// cleanupOldData cleans up old usage data
func (m *Manager) cleanupOldData() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

// cleanup removes old records
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	cutoff := time.Now().AddDate(0, 0, -m.config.DataRetention)
	filtered := make([]*UsageRecord, 0)
	
	for _, record := range m.records {
		if record.StartTime.After(cutoff) {
			filtered = append(filtered, record)
		}
	}
	
	m.records = filtered
}

// RecordUsage records app usage
func (m *Manager) RecordUsage(record *UsageRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if record.ID == "" {
		record.ID = fmt.Sprintf("usage_%d", time.Now().UnixNano())
	}
	
	m.records = append(m.records, record)
	return nil
}

// SetGoal sets a usage goal
func (m *Manager) SetGoal(goal *UsageGoal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if goal.ID == "" {
		goal.ID = fmt.Sprintf("goal_%d", time.Now().UnixNano())
	}
	goal.CreatedAt = time.Now()
	
	m.goals[goal.ID] = goal
	return nil
}

// GetDailyUsage gets daily usage for a user
func (m *Manager) GetDailyUsage(userID string, date time.Time) *DailyUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	dayStart := date.Truncate(24 * time.Hour)
	dayEnd := dayStart.Add(24 * time.Hour)
	
	usage := &DailyUsage{
		Date:       dayStart,
		UserID:     userID,
		ByCategory: make(map[AppCategory]time.Duration),
		ByApp:      make(map[string]time.Duration),
	}
	
	for _, record := range m.records {
		if record.UserID != userID {
			continue
		}
		if record.StartTime.Before(dayStart) || record.StartTime.After(dayEnd) {
			continue
		}
		
		usage.TotalTime += record.Duration
		usage.ByCategory[record.Category] += record.Duration
		usage.ByApp[record.AppName] += record.Duration
	}
	
	return usage
}

// EnableFocusMode enables focus mode
func (m *Manager) EnableFocusMode(mode *FocusMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if mode.ID == "" {
		mode.ID = fmt.Sprintf("focus_%d", time.Now().UnixNano())
	}
	mode.CreatedAt = time.Now()
	
	m.focusModes[mode.ID] = mode
	return nil
}

// SetParentalControl sets parental control
func (m *Manager) SetParentalControl(control *ParentalControl) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.parental[control.UserID] = control
	return nil
}

// GenerateReport generates a wellness report
func (m *Manager) GenerateReport(userID string, period string) *WellnessReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	report := &WellnessReport{
		UserID:      userID,
		Period:      period,
		TopApps:     make([]AppUsage, 0),
		Suggestions: make([]string, 0),
		GeneratedAt: time.Now(),
	}
	
	// Calculate period duration
	var duration time.Duration
	switch period {
	case "daily":
		duration = 24 * time.Hour
	case "weekly":
		duration = 7 * 24 * time.Hour
	case "monthly":
		duration = 30 * 24 * time.Hour
	}
	
	cutoff := time.Now().Add(-duration)
	appUsage := make(map[string]time.Duration)
	totalTime := time.Duration(0)
	
	for _, record := range m.records {
		if record.UserID != userID || record.StartTime.Before(cutoff) {
			continue
		}
		totalTime += record.Duration
		appUsage[record.AppName] += record.Duration
	}
	
	report.TotalTime = totalTime
	if period == "daily" {
		report.AverageDaily = totalTime
	} else {
		report.AverageDaily = totalTime / time.Duration(int(duration.Hours()/24))
	}
	
	// Top apps
	for app, dur := range appUsage {
		report.TopApps = append(report.TopApps, AppUsage{
			AppName:  app,
			Duration: dur,
			Percent:  float64(dur) / float64(totalTime) * 100,
		})
	}
	
	// Check goals
	for _, goal := range m.goals {
		if goal.UserID == userID && goal.Enabled {
			report.GoalsTotal++
		}
	}
	
	// Generate suggestions
	if report.AverageDaily > 8*time.Hour {
		report.Suggestions = append(report.Suggestions, "建议减少屏幕时间，多进行户外活动")
	}
	
	return report
}

// GetStats returns wellbeing statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return map[string]interface{}{
		"total_records":    len(m.records),
		"total_goals":      len(m.goals),
		"total_focus_modes": len(m.focusModes),
		"total_alerts":     len(m.alerts),
		"tracking_enabled": m.config.TrackingEnabled,
	}
}
