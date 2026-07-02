// Package fileactivitywatcher 文件活动监控
// 实时监控文件系统变更，支持活动统计与异常检测告警
package fileactivitywatcher

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// EventType 事件类型.
type EventType string

const (
	EventCreate EventType = "create" // 创建
	EventModify EventType = "modify" // 修改
	EventDelete EventType = "delete" // 删除
	EventMove   EventType = "move"   // 移动/重命名
)

// AlertSeverity 告警严重级别.
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// ActivityEvent 文件活动事件.
type ActivityEvent struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	EventType EventType `json:"event_type"`
	Size      int64     `json:"size"`
	UserID    string    `json:"user_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// WatchDir 监控目录.
type WatchDir struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Recursive bool      `json:"recursive"`
	FileTypes []string  `json:"file_types,omitempty"` // 过滤的文件扩展名，如 [".txt", ".go"]
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// AlertRule 告警规则.
type AlertRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Enabled     bool          `json:"enabled"`
	Threshold   int           `json:"threshold"`  // 阈值：触发告警的操作次数
	WindowSec   int           `json:"window_sec"` // 时间窗口（秒）
	Severity    AlertSeverity `json:"severity"`
	EventTypes  []EventType   `json:"event_types"`             // 监控的事件类型
	WatchDirIDs []string      `json:"watch_dir_ids,omitempty"` // 关联的监控目录，空则全部
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// Alert 告警实例.
type Alert struct {
	ID        string        `json:"id"`
	RuleID    string        `json:"rule_id"`
	RuleName  string        `json:"rule_name"`
	Severity  AlertSeverity `json:"severity"`
	Message   string        `json:"message"`
	Count     int           `json:"count"`
	WindowSec int           `json:"window_sec"`
	Timestamp time.Time     `json:"timestamp"`
	Resolved  bool          `json:"resolved"`
}

// ActivityStats 活动统计.
type ActivityStats struct {
	Hourly map[string]int `json:"hourly"` // "2006-01-02T15" -> count
	Daily  map[string]int `json:"daily"`  // "2006-01-02" -> count
	Total  int            `json:"total"`
}

// Manager 文件活动监控管理器.
type Manager struct {
	mu         sync.RWMutex
	events     []*ActivityEvent
	watchDirs  map[string]*WatchDir
	alertRules map[string]*AlertRule
	alerts     []*Alert
	maxEvents  int
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		events:     make([]*ActivityEvent, 0, 1000),
		watchDirs:  make(map[string]*WatchDir),
		alertRules: make(map[string]*AlertRule),
		alerts:     make([]*Alert, 0, 100),
		maxEvents:  10000,
	}
}

// AddWatchDir 添加监控目录.
func (m *Manager) AddWatchDir(dir *WatchDir) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if dir.Path == "" {
		return fmt.Errorf("监控路径不能为空")
	}
	if dir.ID == "" {
		dir.ID = fmt.Sprintf("wd_%d", time.Now().UnixNano())
	}
	dir.CreatedAt = time.Now()
	dir.Enabled = true
	m.watchDirs[dir.ID] = dir
	return nil
}

// RemoveWatchDir 移除监控目录.
func (m *Manager) RemoveWatchDir(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.watchDirs[id]; !exists {
		return false
	}
	delete(m.watchDirs, id)
	return true
}

// ListWatchDirs 列出监控目录.
func (m *Manager) ListWatchDirs() []WatchDir {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]WatchDir, 0, len(m.watchDirs))
	for _, d := range m.watchDirs {
		result = append(result, *d)
	}
	return result
}

// AddAlertRule 添加告警规则.
func (m *Manager) AddAlertRule(rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}
	if rule.Threshold <= 0 {
		rule.Threshold = 100
	}
	if rule.WindowSec <= 0 {
		rule.WindowSec = 60
	}
	if rule.Severity == "" {
		rule.Severity = SeverityWarning
	}
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule_%d", time.Now().UnixNano())
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	rule.Enabled = true
	m.alertRules[rule.ID] = rule
	return nil
}

// UpdateAlertRule 更新告警规则.
func (m *Manager) UpdateAlertRule(rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.alertRules[rule.ID]; !exists {
		return fmt.Errorf("规则不存在: %s", rule.ID)
	}
	rule.UpdatedAt = time.Now()
	m.alertRules[rule.ID] = rule
	return nil
}

// DeleteAlertRule 删除告警规则.
func (m *Manager) DeleteAlertRule(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.alertRules[id]; !exists {
		return false
	}
	delete(m.alertRules, id)
	return true
}

// ListAlertRules 列出告警规则.
func (m *Manager) ListAlertRules() []AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AlertRule, 0, len(m.alertRules))
	for _, r := range m.alertRules {
		result = append(result, *r)
	}
	return result
}

// RecordEvent 记录文件活动事件.
func (m *Manager) RecordEvent(event *ActivityEvent) *Alert {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.ID == "" {
		event.ID = fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// 检查是否匹配监控目录
	if !m.matchesWatchDir(event.Path) {
		return nil
	}

	// 存储事件
	m.events = append(m.events, event)
	if len(m.events) > m.maxEvents {
		m.events = m.events[len(m.events)-m.maxEvents:]
	}

	// 检查告警规则
	return m.checkAlertRules(event)
}

// matchesWatchDir 检查路径是否匹配监控目录.
func (m *Manager) matchesWatchDir(path string) bool {
	if len(m.watchDirs) == 0 {
		return true
	}

	for _, dir := range m.watchDirs {
		if !dir.Enabled {
			continue
		}
		if !strings.HasPrefix(path, dir.Path) {
			continue
		}
		// 检查文件类型过滤
		if len(dir.FileTypes) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			matched := false
			for _, ft := range dir.FileTypes {
				if strings.ToLower(ft) == ext {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		return true
	}
	return false
}

// checkAlertRules 检查告警规则.
func (m *Manager) checkAlertRules(event *ActivityEvent) *Alert {
	for _, rule := range m.alertRules {
		if !rule.Enabled {
			continue
		}

		// 检查事件类型匹配
		if len(rule.EventTypes) > 0 {
			matched := false
			for _, et := range rule.EventTypes {
				if et == event.EventType {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 检查监控目录匹配
		if len(rule.WatchDirIDs) > 0 {
			matched := false
			for _, wdID := range rule.WatchDirIDs {
				if wd, exists := m.watchDirs[wdID]; exists && wd.Enabled {
					if strings.HasPrefix(event.Path, wd.Path) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		// 统计时间窗口内的事件数量
		cutoff := time.Now().Add(-time.Duration(rule.WindowSec) * time.Second)
		count := 0
		for _, e := range m.events {
			if e.Timestamp.After(cutoff) || e.Timestamp.Equal(cutoff) {
				count++
			}
		}

		if count >= rule.Threshold {
			alert := &Alert{
				ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				Severity:  rule.Severity,
				Message:   fmt.Sprintf("规则 [%s] 触发: %d 秒内检测到 %d 次操作", rule.Name, rule.WindowSec, count),
				Count:     count,
				WindowSec: rule.WindowSec,
				Timestamp: time.Now(),
			}
			m.alerts = append(m.alerts, alert)
			return alert
		}
	}
	return nil
}

// GetEvents 查询活动事件.
func (m *Manager) GetEvents(eventType string, limit int) []ActivityEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	result := make([]ActivityEvent, 0)
	// 从最新开始遍历
	for i := len(m.events) - 1; i >= 0 && len(result) < limit; i-- {
		e := m.events[i]
		if eventType != "" && string(e.EventType) != eventType {
			continue
		}
		result = append(result, *e)
	}
	return result
}

// GetStats 获取活动统计.
func (m *Manager) GetStats() ActivityStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ActivityStats{
		Hourly: make(map[string]int),
		Daily:  make(map[string]int),
		Total:  len(m.events),
	}

	for _, e := range m.events {
		hourKey := e.Timestamp.Format("2006-01-02T15")
		dayKey := e.Timestamp.Format("2006-01-02")
		stats.Hourly[hourKey]++
		stats.Daily[dayKey]++
	}

	return stats
}

// GetAlerts 获取告警列表.
func (m *Manager) GetAlerts(resolved bool) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Alert, 0)
	for _, a := range m.alerts {
		if a.Resolved == resolved {
			result = append(result, *a)
		}
	}
	return result
}

// ResolveAlert 解决告警.
func (m *Manager) ResolveAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.alerts {
		if a.ID == alertID {
			a.Resolved = true
			return nil
		}
	}
	return fmt.Errorf("告警不存在: %s", alertID)
}
