// Package audittrail 合规审计追踪
// 操作日志、合规报告、异常检测
package audittrail

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RiskLevel 风险级别
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// AuditEvent 审计事件
type AuditEvent struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	UserName   string    `json:"user_name"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id"`
	Details    string    `json:"details"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	Timestamp  time.Time `json:"timestamp"`
	RiskLevel  RiskLevel `json:"risk_level"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID           string        `json:"id"`
	Period       string        `json:"period"`
	TotalEvents  int           `json:"total_events"`
	RiskSummary  map[RiskLevel]int `json:"risk_summary"`
	TopUsers     []UserCount   `json:"top_users"`
	TopActions   []ActionCount `json:"top_actions"`
	GeneratedAt  time.Time     `json:"generated_at"`
}

// UserCount 用户操作计数
type UserCount struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	Count    int    `json:"count"`
}

// ActionCount 操作计数
type ActionCount struct {
	Action string `json:"action"`
	Count  int    `json:"count"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Condition string `json:"condition"`
	Action    string `json:"action"`
	Enabled   bool   `json:"enabled"`
}

// EventFilter 事件过滤器
type EventFilter struct {
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	RiskLevel RiskLevel `json:"risk_level"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Limit     int       `json:"limit"`
}

// ExportFormat 导出格式
type ExportFormat string

const (
	FormatJSON ExportFormat = "json"
	FormatCSV  ExportFormat = "csv"
)

// Manager 审计追踪管理器
type Manager struct {
	mu        sync.RWMutex
	events    []AuditEvent
	reports   map[string]*ComplianceReport
	alerts    map[string]*AlertRule
	maxEvents int
}

// NewManager 创建审计追踪管理器
func NewManager() *Manager {
	return &Manager{
		events:    make([]AuditEvent, 0),
		reports:   make(map[string]*ComplianceReport),
		alerts:    make(map[string]*AlertRule),
		maxEvents: 100000,
	}
}

// LogEvent 记录审计事件
func (m *Manager) LogEvent(event AuditEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.ID == "" {
		event.ID = fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	m.events = append([]AuditEvent{event}, m.events...)
	if len(m.events) > m.maxEvents {
		m.events = m.events[:m.maxEvents]
	}
}

// GetEvent 获取单个事件
func (m *Manager) GetEvent(id string) (*AuditEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, e := range m.events {
		if e.ID == id {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("事件不存在: %s", id)
}

// QueryEvents 查询事件
func (m *Manager) QueryEvents(filter EventFilter) []AuditEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AuditEvent, 0)
	for _, e := range m.events {
		if filter.UserID != "" && e.UserID != filter.UserID {
			continue
		}
		if filter.Action != "" && e.Action != filter.Action {
			continue
		}
		if filter.RiskLevel != "" && e.RiskLevel != filter.RiskLevel {
			continue
		}
		if !filter.StartTime.IsZero() && e.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && e.Timestamp.After(filter.EndTime) {
			continue
		}
		result = append(result, e)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result
}

// GenerateReport 生成合规报告
func (m *Manager) GenerateReport(period string) *ComplianceReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	report := &ComplianceReport{
		ID:          fmt.Sprintf("rpt_%d", time.Now().UnixNano()),
		Period:      period,
		RiskSummary: make(map[RiskLevel]int),
		TopUsers:    make([]UserCount, 0),
		TopActions:  make([]ActionCount, 0),
		GeneratedAt: time.Now(),
	}

	userCount := make(map[string]*UserCount)
	actionCount := make(map[string]int)

	for _, e := range m.events {
		report.TotalEvents++
		report.RiskSummary[e.RiskLevel]++
		actionCount[e.Action]++

		uc, ok := userCount[e.UserID]
		if !ok {
			uc = &UserCount{UserID: e.UserID, UserName: e.UserName}
			userCount[e.UserID] = uc
		}
		uc.Count++
	}

	// Top 10 users
	topUsers := make([]*UserCount, 0, len(userCount))
	for _, uc := range userCount {
		topUsers = append(topUsers, uc)
	}
	sortByCount := func(a, b *UserCount) bool { return a.Count > b.Count }
	for i := 0; i < len(topUsers) && i < 10; i++ {
		for j := i + 1; j < len(topUsers); j++ {
			if sortByCount(topUsers[j], topUsers[i]) {
				topUsers[i], topUsers[j] = topUsers[j], topUsers[i]
			}
		}
		report.TopUsers = append(report.TopUsers, *topUsers[i])
	}

	// Top 10 actions
	type kv struct {
		k string
		v int
	}
	sorted := make([]kv, 0, len(actionCount))
	for k, v := range actionCount {
		sorted = append(sorted, kv{k, v})
	}
	for i := 0; i < len(sorted) && i < 10; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].v > sorted[i].v {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
		report.TopActions = append(report.TopActions, ActionCount{
			Action: sorted[i].k,
			Count:  sorted[i].v,
		})
	}

	m.reports[report.ID] = report
	return report
}

// GetReports 获取所有报告
func (m *Manager) GetReports() []*ComplianceReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports := make([]*ComplianceReport, 0, len(m.reports))
	for _, r := range m.reports {
		reports = append(reports, r)
	}
	return reports
}

// AddAlertRule 添加告警规则
func (m *Manager) AddAlertRule(rule AlertRule) AlertRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("alert_%d", time.Now().UnixNano())
	}
	m.alerts[rule.ID] = &rule
	return rule
}

// UpdateAlertRule 更新告警规则
func (m *Manager) UpdateAlertRule(id string, rule AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.alerts[id]
	if !ok {
		return fmt.Errorf("告警规则不存在: %s", id)
	}

	if rule.Name != "" {
		existing.Name = rule.Name
	}
	if rule.Condition != "" {
		existing.Condition = rule.Condition
	}
	if rule.Action != "" {
		existing.Action = rule.Action
	}
	existing.Enabled = rule.Enabled

	return nil
}

// DeleteAlertRule 删除告警规则
func (m *Manager) DeleteAlertRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.alerts[id]; !ok {
		return fmt.Errorf("告警规则不存在: %s", id)
	}
	delete(m.alerts, id)
	return nil
}

// GetAlertRules 获取所有告警规则
func (m *Manager) GetAlertRules() []AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]AlertRule, 0, len(m.alerts))
	for _, r := range m.alerts {
		rules = append(rules, *r)
	}
	return rules
}

// CheckAlerts 检查告警
func (m *Manager) CheckAlerts() []AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	triggered := make([]AlertRule, 0)
	for _, rule := range m.alerts {
		if !rule.Enabled {
			continue
		}
		// 简单的条件检查示例
		if rule.Condition == "high_risk_count > 10" {
			count := 0
			for _, e := range m.events {
				if e.RiskLevel == RiskHigh || e.RiskLevel == RiskCritical {
					count++
				}
			}
			if count > 10 {
				triggered = append(triggered, *rule)
			}
		}
	}
	return triggered
}

// ExportEvents 导出事件
func (m *Manager) ExportEvents(format ExportFormat) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch format {
	case FormatJSON:
		return json.MarshalIndent(m.events, "", "  ")
	case FormatCSV:
		var buf strings.Builder
		w := csv.NewWriter(&buf)

		// Header
		w.Write([]string{"ID", "UserID", "UserName", "Action", "Resource", "ResourceID", "Details", "IP", "UserAgent", "Timestamp", "RiskLevel"})

		for _, e := range m.events {
			w.Write([]string{
				e.ID,
				e.UserID,
				e.UserName,
				e.Action,
				e.Resource,
				e.ResourceID,
				e.Details,
				e.IP,
				e.UserAgent,
				e.Timestamp.Format(time.RFC3339),
				string(e.RiskLevel),
			})
		}
		w.Flush()
		return []byte(buf.String()), nil
	default:
		return nil, fmt.Errorf("不支持的格式: %s", format)
	}
}
