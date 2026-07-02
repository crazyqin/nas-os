// Package audittrail 提供审计追踪核心逻辑
package audittrail

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 审计追踪管理器.
type Manager struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	events     map[string]*AuditEvent
	activities map[string]*SuspiciousActivity
	reports    map[string]*AuditReport
	policies   map[string]*RetentionPolicy
	exports    map[string]*AuditExport
	rules      []DetectionRule
}

// DetectionRule 检测规则.
type DetectionRule struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Conditions []string `json:"conditions"`
	Threshold  int      `json:"threshold"`
	TimeWindow string   `json:"time_window"`
	Enabled    bool     `json:"enabled"`
}

// NewManager 创建审计追踪管理器.
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:     logger,
		events:     make(map[string]*AuditEvent),
		activities: make(map[string]*SuspiciousActivity),
		reports:    make(map[string]*AuditReport),
		policies:   make(map[string]*RetentionPolicy),
		exports:    make(map[string]*AuditExport),
	}

	// 初始化检测规则
	m.initDetectionRules()
	// 初始化默认保留策略
	m.initDefaultPolicies()

	return m
}

// initDetectionRules 初始化检测规则.
func (m *Manager) initDetectionRules() {
	m.rules = []DetectionRule{
		{
			ID: "brute-force", Name: "暴力破解检测", Type: "brute-force",
			Conditions: []string{"event_type == 'login'", "result == 'failure'"},
			Threshold:  5, TimeWindow: "5m", Enabled: true,
		},
		{
			ID: "data-exfil", Name: "数据泄露检测", Type: "data-exfiltration",
			Conditions: []string{"event_type == 'download'", "resource.type == 'database'"},
			Threshold:  100, TimeWindow: "1h", Enabled: true,
		},
		{
			ID: "privilege-esc", Name: "权限提升检测", Type: "privilege-escalation",
			Conditions: []string{"event_type == 'admin'", "actor.role != 'admin'"},
			Threshold:  1, TimeWindow: "1m", Enabled: true,
		},
	}
}

// initDefaultPolicies 初始化默认保留策略.
func (m *Manager) initDefaultPolicies() {
	defaultPolicies := []*RetentionPolicy{
		{
			ID: "default-90d", Name: "默认90天保留", Description: "普通事件保留90天",
			EventTypes: []string{"login", "logout", "access"}, Duration: "90d", Action: "archive", Enabled: true,
		},
		{
			ID: "critical-1y", Name: "关键事件1年保留", Description: "关键安全事件保留1年",
			EventTypes: []string{"admin", "modify", "delete"}, Severity: []string{"critical"}, Duration: "1y", Action: "archive", Enabled: true,
		},
	}

	now := time.Now()
	for _, policy := range defaultPolicies {
		policy.CreatedAt = now
		policy.UpdatedAt = now
		m.policies[policy.ID] = policy
	}
}

// RecordEvent 记录审计事件.
func (m *Manager) RecordEvent(event *AuditEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	m.events[event.ID] = event
	m.logger.Debug("Recorded audit event",
		zap.String("id", event.ID),
		zap.String("type", event.EventType),
		zap.String("actor", event.Actor.Name),
	)

	// 检测可疑行为
	go m.detectSuspiciousActivity(event)
}

// ListEvents 获取事件列表.
func (m *Manager) ListEvents(filter *EventFilter) ([]AuditEvent, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var events []AuditEvent
	for _, event := range m.events {
		if m.matchesFilter(event, filter) {
			events = append(events, *event)
		}
	}

	total := len(events)

	// 应用分页
	if filter != nil {
		if filter.Offset > 0 && filter.Offset < len(events) {
			events = events[filter.Offset:]
		}
		if filter.Limit > 0 && filter.Limit < len(events) {
			events = events[:filter.Limit]
		}
	}

	return events, total
}

// GetEvent 获取单个事件.
func (m *Manager) GetEvent(id string) (*AuditEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	event, ok := m.events[id]
	if !ok {
		return nil, fmt.Errorf("event %s not found", id)
	}
	return event, nil
}

// matchesFilter 检查事件是否匹配过滤器.
func (m *Manager) matchesFilter(event *AuditEvent, filter *EventFilter) bool {
	if filter == nil {
		return true
	}

	if filter.StartTime != nil && event.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && event.Timestamp.After(*filter.EndTime) {
		return false
	}
	if len(filter.EventTypes) > 0 {
		found := false
		for _, t := range filter.EventTypes {
			if event.EventType == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(filter.Actors) > 0 {
		found := false
		for _, a := range filter.Actors {
			if event.Actor.ID == a || event.Actor.Name == a {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(filter.Severities) > 0 {
		found := false
		for _, s := range filter.Severities {
			if event.Severity == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// detectSuspiciousActivity 检测可疑行为.
func (m *Manager) detectSuspiciousActivity(event *AuditEvent) {
	// 简化的检测逻辑
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}

		isSuspicious := false
		switch rule.Type {
		case "brute-force":
			if event.EventType == "login" && event.Result == "failure" {
				isSuspicious = true
			}
		case "privilege-escalation":
			if event.EventType == "admin" && event.Actor.Role != "admin" {
				isSuspicious = true
			}
		}

		if isSuspicious {
			activity := &SuspiciousActivity{
				ID:          fmt.Sprintf("susp-%d", time.Now().UnixNano()),
				Timestamp:   time.Now(),
				Type:        rule.Type,
				Actor:       event.Actor,
				Description: fmt.Sprintf("检测到可疑行为: %s", rule.Name),
				Indicators:  []string{event.ID},
				RiskScore:   75.0,
				Status:      "detected",
				DetectedAt:  time.Now(),
			}

			m.mu.Lock()
			m.activities[activity.ID] = activity
			m.mu.Unlock()

			m.logger.Warn("Suspicious activity detected",
				zap.String("id", activity.ID),
				zap.String("type", activity.Type),
				zap.String("actor", event.Actor.Name),
			)
		}
	}
}

// DetectSuspicious 获取可疑行为列表.
func (m *Manager) DetectSuspicious(filter *SuspiciousFilter) ([]SuspiciousActivity, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var activities []SuspiciousActivity
	for _, activity := range m.activities {
		if m.matchesSuspiciousFilter(activity, filter) {
			activities = append(activities, *activity)
		}
	}

	total := len(activities)

	if filter != nil && filter.Limit > 0 && filter.Limit < len(activities) {
		activities = activities[:filter.Limit]
	}

	return activities, total
}

// matchesSuspiciousFilter 检查可疑行为是否匹配过滤器.
func (m *Manager) matchesSuspiciousFilter(activity *SuspiciousActivity, filter *SuspiciousFilter) bool {
	if filter == nil {
		return true
	}

	if filter.StartTime != nil && activity.DetectedAt.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && activity.DetectedAt.After(*filter.EndTime) {
		return false
	}
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if activity.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(filter.Statuses) > 0 {
		found := false
		for _, s := range filter.Statuses {
			if activity.Status == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if filter.MinScore != nil && activity.RiskScore < *filter.MinScore {
		return false
	}

	return true
}

// UpdateSuspiciousStatus 更新可疑行为状态.
func (m *Manager) UpdateSuspiciousStatus(id, status, assignedTo, notes string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	activity, ok := m.activities[id]
	if !ok {
		return fmt.Errorf("activity %s not found", id)
	}

	activity.Status = status
	if assignedTo != "" {
		activity.AssignedTo = assignedTo
	}
	if notes != "" {
		activity.Notes = notes
	}
	if status == "confirmed" || status == "false-positive" {
		now := time.Now()
		activity.ResolvedAt = &now
	}

	return nil
}

// GenerateReport 生成审计报告.
func (m *Manager) GenerateReport(title, reportType string, period ReportPeriod, generatedBy string) (*AuditReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reportID := fmt.Sprintf("report-%d", time.Now().UnixNano())

	// 获取周期内的事件
	var events []AuditEvent
	eventsByType := make(map[string]int)
	eventsBySeverity := make(map[string]int)
	actorCounts := make(map[string]int)
	resourceCounts := make(map[string]int)
	failures := 0

	for _, event := range m.events {
		if event.Timestamp.After(period.Start) && event.Timestamp.Before(period.End) {
			events = append(events, *event)
			eventsByType[event.EventType]++
			eventsBySeverity[event.Severity]++
			actorCounts[event.Actor.Name]++
			resourceCounts[event.Resource.Name]++
			if event.Result == "failure" {
				failures++
			}
		}
	}

	// 获取可疑行为
	var activities []SuspiciousActivity
	for _, activity := range m.activities {
		if activity.DetectedAt.After(period.Start) && activity.DetectedAt.Before(period.End) {
			activities = append(activities, *activity)
		}
	}

	// 构建Top统计
	var topActors []ActorStat
	for name, count := range actorCounts {
		topActors = append(topActors, ActorStat{ActorName: name, Count: count})
	}
	var topResources []ResourceStat
	for name, count := range resourceCounts {
		topResources = append(topResources, ResourceStat{ResourceName: name, Count: count})
	}

	failureRate := 0.0
	if len(events) > 0 {
		failureRate = float64(failures) / float64(len(events)) * 100
	}

	report := &AuditReport{
		ID:     reportID,
		Title:  title,
		Type:   reportType,
		Period: period,
		Summary: ReportSummary{
			TotalEvents:      len(events),
			EventsByType:     eventsByType,
			EventsBySeverity: eventsBySeverity,
			TopActors:        topActors,
			TopResources:     topResources,
			SuspiciousCount:  len(activities),
			FailureRate:      failureRate,
		},
		Events:      events,
		Activities:  activities,
		GeneratedAt: time.Now(),
		GeneratedBy: generatedBy,
		Format:      "json",
	}

	m.reports[reportID] = report
	m.logger.Info("Generated audit report",
		zap.String("id", reportID),
		zap.String("title", title),
		zap.Int("events", len(events)),
	)

	return report, nil
}

// GetReport 获取报告.
func (m *Manager) GetReport(id string) (*AuditReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("report %s not found", id)
	}
	return report, nil
}

// ListReports 获取报告列表.
func (m *Manager) ListReports(reportType string) []*AuditReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var reports []*AuditReport
	for _, report := range m.reports {
		if reportType == "" || report.Type == reportType {
			reports = append(reports, report)
		}
	}
	return reports
}

// SetRetention 设置保留策略.
func (m *Manager) SetRetention(policy *RetentionPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("policy-%d", time.Now().UnixNano())
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	m.policies[policy.ID] = policy
	m.logger.Info("Set retention policy",
		zap.String("id", policy.ID),
		zap.String("name", policy.Name),
		zap.String("duration", policy.Duration),
	)
	return nil
}

// GetRetention 获取保留策略.
func (m *Manager) GetRetention(id string) (*RetentionPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	return policy, nil
}

// ListRetentions 获取保留策略列表.
func (m *Manager) ListRetentions() []*RetentionPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var policies []*RetentionPolicy
	for _, policy := range m.policies {
		policies = append(policies, policy)
	}
	return policies
}

// GetRetentionStats 获取保留统计.
func (m *Manager) GetRetentionStats(policyID string) (*RetentionStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[policyID]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", policyID)
	}

	stats := &RetentionStats{
		PolicyID:    policy.ID,
		TotalEvents: len(m.events),
		LastRunAt:   time.Now(),
	}

	return stats, nil
}

// ExportAudit 导出审计数据.
func (m *Manager) ExportAudit(req *ExportRequest) (*AuditExport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	exportID := fmt.Sprintf("export-%d", time.Now().UnixNano())

	export := &AuditExport{
		ID:        exportID,
		Format:    req.Format,
		Filter:    req.Filter,
		Status:    "completed",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	// 获取匹配的事件
	var events []AuditEvent
	for _, event := range m.events {
		if m.matchesFilter(event, &req.Filter) {
			events = append(events, *event)
		}
	}

	export.FileSize = int64(len(events) * 100) // 粗略估算
	m.exports[exportID] = export

	m.logger.Info("Exported audit data",
		zap.String("id", exportID),
		zap.String("format", req.Format),
		zap.Int("events", len(events)),
	)

	return export, nil
}

// GetExport 获取导出状态.
func (m *Manager) GetExport(id string) (*AuditExport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	export, ok := m.exports[id]
	if !ok {
		return nil, fmt.Errorf("export %s not found", id)
	}
	return export, nil
}

// ListExports 获取导出列表.
func (m *Manager) ListExports() []*AuditExport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var exports []*AuditExport
	for _, export := range m.exports {
		exports = append(exports, export)
	}
	return exports
}
