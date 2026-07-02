// Package nasopsintel NAS 运维智能分析引擎
// 对标群晖 Active Insight、TrueNAS TrueCommand 的运维分析能力
// 关联分析多子系统事件，AI 异常检测，自动化修复建议
package nasopsintel

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Severity 事件严重程度.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// severityRank 严重程度数值映射，用于正确比较.
func severityRank(s Severity) int {
	switch s {
	case SeverityInfo:
		return 0
	case SeverityWarning:
		return 1
	case SeverityError:
		return 2
	case SeverityCritical:
		return 3
	default:
		return -1
	}
}

// IsHigherThan 返回 a 是否比 b 更严重.
func (s Severity) IsHigherThan(other Severity) bool {
	return severityRank(s) > severityRank(other)
}

// EventSource 事件来源.
type EventSource string

const (
	SourceStorage   EventSource = "storage"
	SourceNetwork   EventSource = "network"
	SourceCompute   EventSource = "compute"
	SourceSecurity  EventSource = "security"
	SourceBackup    EventSource = "backup"
	SourceContainer EventSource = "container"
	SourceSystem    EventSource = "system"
)

// IncidentStatus 事件状态.
type IncidentStatus string

const (
	IncidentOpen          IncidentStatus = "open"
	IncidentInvestigating IncidentStatus = "investigating"
	IncidentResolved      IncidentStatus = "resolved"
	IncidentClosed        IncidentStatus = "closed"
)

// OpsEvent 运维事件.
type OpsEvent struct {
	ID          string                 `json:"id"`
	Source      EventSource            `json:"source"`
	Severity    Severity               `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Host        string                 `json:"host,omitempty"`
	Service     string                 `json:"service,omitempty"`
}

// Incident 运维事件（聚合后）.
type Incident struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	Severity         Severity       `json:"severity"`
	Status           IncidentStatus `json:"status"`
	Events           []OpsEvent     `json:"events"`
	RootCause        string         `json:"root_cause,omitempty"`
	Remediation      string         `json:"remediation,omitempty"`
	AffectedServices []string       `json:"affected_services"`
	FirstSeen        time.Time      `json:"first_seen"`
	LastSeen         time.Time      `json:"last_seen"`
	ResolvedAt       *time.Time     `json:"resolved_at,omitempty"`
	Assignee         string         `json:"assignee,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
}

// Anomaly 异常检测结果.
type Anomaly struct {
	ID          string      `json:"id"`
	Source      EventSource `json:"source"`
	Metric      string      `json:"metric"`
	Value       float64     `json:"value"`
	Expected    float64     `json:"expected"`
	Deviation   float64     `json:"deviation"`
	Severity    Severity    `json:"severity"`
	Description string      `json:"description"`
	DetectedAt  time.Time   `json:"detected_at"`
	Window      string      `json:"window"` // 检测窗口
}

// HealthScore 健康评分.
type HealthScore struct {
	Overall     float64            `json:"overall"` // 0-100
	Storage     float64            `json:"storage"`
	Network     float64            `json:"network"`
	Compute     float64            `json:"compute"`
	Security    float64            `json:"security"`
	Backup      float64            `json:"backup"`
	Trend       string             `json:"trend"` // improving, stable, degrading
	UpdatedAt   time.Time          `json:"updated_at"`
	Suggestions []HealthSuggestion `json:"suggestions"`
}

// HealthSuggestion 健康建议.
type HealthSuggestion struct {
	Category    string `json:"category"`
	Priority    int    `json:"priority"` // 1-5
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action,omitempty"`
}

// OpsMetrics 运维指标.
type OpsMetrics struct {
	TotalEvents       int64         `json:"total_events"`
	OpenIncidents     int           `json:"open_incidents"`
	ResolvedToday     int           `json:"resolved_today"`
	AvgResolution     time.Duration `json:"avg_resolution"`
	AnomaliesDetected int           `json:"anomalies_detected"`
	HealthScore       float64       `json:"health_score"`
	Uptime            time.Duration `json:"uptime"`
	LastUpdated       time.Time     `json:"last_updated"`
}

// Rule 关联规则.
type Rule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Source      EventSource `json:"source"`
	Condition   string      `json:"condition"` // 简单条件表达式
	Severity    Severity    `json:"severity"`
	Enabled     bool        `json:"enabled"`
	Actions     []string    `json:"actions"` // 触发的动作
	CreatedAt   time.Time   `json:"created_at"`
}

// Manager 运维智能管理器.
type Manager struct {
	mu         sync.RWMutex
	events     []OpsEvent
	incidents  map[string]*Incident
	anomalies  []Anomaly
	rules      map[string]*Rule
	health     *HealthScore
	metrics    *OpsMetrics
	startTime  time.Time
	cancelFunc context.CancelFunc
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		events:    make([]OpsEvent, 0, 1000),
		incidents: make(map[string]*Incident),
		anomalies: make([]Anomaly, 0, 100),
		rules:     make(map[string]*Rule),
		health:    &HealthScore{},
		metrics:   &OpsMetrics{},
		startTime: time.Now(),
	}
}

// Start 启动管理器.
func (m *Manager) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel

	go m.correlateLoop(ctx)
	go m.anomalyDetectLoop(ctx)
	go m.healthCalcLoop(ctx)

	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
}

// IngestEvent 接收事件.
func (m *Manager) IngestEvent(event OpsEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}

	m.events = append(m.events, event)
	m.metrics.TotalEvents++

	// 检查是否匹配关联规则
	m.checkRules(event)

	// 检查是否需要创建/更新事件
	m.correlateEvent(event)
}

// ListEvents 列出事件.
func (m *Manager) ListEvents(limit int, source EventSource, severity Severity) []OpsEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]OpsEvent, 0)
	for i := len(m.events) - 1; i >= 0; i-- {
		e := m.events[i]
		if source != "" && e.Source != source {
			continue
		}
		if severity != "" && e.Severity != severity {
			continue
		}
		result = append(result, e)
		if len(result) >= limit {
			break
		}
	}
	return result
}

// GetIncident 获取事件.
func (m *Manager) GetIncident(id string) (*Incident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inc, ok := m.incidents[id]
	if !ok {
		return nil, fmt.Errorf("incident %s not found", id)
	}
	return inc, nil
}

// ListIncidents 列出事件.
func (m *Manager) ListIncidents(status IncidentStatus) []*Incident {
	m.mu.RLock()
	defer m.mu.RUnlock()
	incidents := make([]*Incident, 0)
	for _, inc := range m.incidents {
		if status != "" && inc.Status != status {
			continue
		}
		incidents = append(incidents, inc)
	}
	return incidents
}

// ResolveIncident 解决事件.
func (m *Manager) ResolveIncident(id, rootCause, remediation string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inc, ok := m.incidents[id]
	if !ok {
		return fmt.Errorf("incident %s not found", id)
	}
	inc.Status = IncidentResolved
	inc.RootCause = rootCause
	inc.Remediation = remediation
	now := time.Now()
	inc.ResolvedAt = &now
	m.metrics.ResolvedToday++
	return nil
}

// ListAnomalies 列出异常.
func (m *Manager) ListAnomalies(limit int) []Anomaly {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.anomalies) {
		limit = len(m.anomalies)
	}
	return m.anomalies[len(m.anomalies)-limit:]
}

// GetHealthScore 获取健康评分.
func (m *Manager) GetHealthScore() *HealthScore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.health
}

// GetMetrics 获取运维指标.
func (m *Manager) GetMetrics() *OpsMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.metrics.Uptime = time.Since(m.startTime)
	m.metrics.LastUpdated = time.Now()
	return m.metrics
}

// AddRule 添加关联规则.
func (m *Manager) AddRule(rule *Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}
	rule.CreatedAt = time.Now()
	m.rules[rule.ID] = rule
	return nil
}

// ListRules 列出规则.
func (m *Manager) ListRules() []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rules := make([]*Rule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	return rules
}

// correlateEvent 关联事件.
func (m *Manager) correlateEvent(event OpsEvent) {
	// 检查是否有相关的开放事件
	for _, inc := range m.incidents {
		if inc.Status == IncidentClosed {
			continue
		}
		// 简单关联：同源、同服务
		if inc.Events[0].Source == event.Source ||
			(event.Service != "" && containsStr(inc.AffectedServices, event.Service)) {
			inc.Events = append(inc.Events, event)
			inc.LastSeen = event.Timestamp
			if event.Severity.IsHigherThan(inc.Severity) {
				inc.Severity = event.Severity
			}
			if event.Service != "" && !containsStr(inc.AffectedServices, event.Service) {
				inc.AffectedServices = append(inc.AffectedServices, event.Service)
			}
			return
		}
	}

	// 创建新事件
	if event.Severity == SeverityWarning || event.Severity == SeverityError || event.Severity == SeverityCritical {
		inc := &Incident{
			ID:          fmt.Sprintf("inc-%d", time.Now().UnixNano()),
			Title:       event.Title,
			Description: event.Description,
			Severity:    event.Severity,
			Status:      IncidentOpen,
			Events:      []OpsEvent{event},
			FirstSeen:   event.Timestamp,
			LastSeen:    event.Timestamp,
		}
		if event.Service != "" {
			inc.AffectedServices = []string{event.Service}
		}
		m.incidents[inc.ID] = inc
		m.metrics.OpenIncidents++
	}
}

// checkRules 检查规则.
func (m *Manager) checkRules(event OpsEvent) {
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		if rule.Source != "" && rule.Source != event.Source {
			continue
		}
		// 简单条件匹配
		if matchCondition(rule.Condition, event) {
			// 规则匹配，可以触发动作
			_ = rule
		}
	}
}

// correlateLoop 关联分析循环.
func (m *Manager) correlateLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.correlateEvents()
		}
	}
}

// anomalyDetectLoop 异常检测循环.
func (m *Manager) anomalyDetectLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.detectAnomalies()
		}
	}
}

// healthCalcLoop 健康评分计算循环.
func (m *Manager) healthCalcLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.calculateHealth()
		}
	}
}

// correlateEvents 关联分析.
func (m *Manager) correlateEvents() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清理过期事件（超过 24 小时的已解决事件）
	cutoff := time.Now().Add(-24 * time.Hour)
	for id, inc := range m.incidents {
		if inc.Status == IncidentResolved && inc.ResolvedAt != nil && inc.ResolvedAt.Before(cutoff) {
			inc.Status = IncidentClosed
		}
		_ = id
	}
}

// detectAnomalies 异常检测.
func (m *Manager) detectAnomalies() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 基于事件频率的简单异常检测
	recentEvents := 0
	cutoff := time.Now().Add(-5 * time.Minute)
	for _, e := range m.events {
		if e.Timestamp.After(cutoff) {
			recentEvents++
		}
	}

	// 如果最近5分钟事件数超过阈值，标记为异常
	if recentEvents > 50 {
		anomaly := Anomaly{
			ID:          fmt.Sprintf("anom-%d", time.Now().UnixNano()),
			Source:      SourceSystem,
			Metric:      "event_rate",
			Value:       float64(recentEvents),
			Expected:    10,
			Deviation:   float64(recentEvents) / 10,
			Severity:    SeverityWarning,
			Description: fmt.Sprintf("事件频率异常：5分钟内 %d 个事件（正常 < 10）", recentEvents),
			DetectedAt:  time.Now(),
			Window:      "5m",
		}
		m.anomalies = append(m.anomalies, anomaly)
		m.metrics.AnomaliesDetected++
	}
}

// calculateHealth 计算健康评分.
func (m *Manager) calculateHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 基于开放事件数计算健康评分
	baseScore := 100.0
	for _, inc := range m.incidents {
		if inc.Status == IncidentOpen || inc.Status == IncidentInvestigating {
			switch inc.Severity {
			case SeverityCritical:
				baseScore -= 20
			case SeverityError:
				baseScore -= 10
			case SeverityWarning:
				baseScore -= 5
			case SeverityInfo:
				baseScore -= 1
			}
		}
	}

	if baseScore < 0 {
		baseScore = 0
	}

	prevOverall := m.health.Overall
	m.health.Overall = baseScore
	m.health.Storage = baseScore
	m.health.Network = baseScore
	m.health.Compute = baseScore
	m.health.Security = baseScore
	m.health.Backup = baseScore
	m.health.UpdatedAt = time.Now()

	// 计算趋势
	if baseScore > prevOverall+5 {
		m.health.Trend = "improving"
	} else if baseScore < prevOverall-5 {
		m.health.Trend = "degrading"
	} else {
		m.health.Trend = "stable"
	}

	// 生成建议
	m.health.Suggestions = m.generateSuggestions(baseScore)
}

// generateSuggestions 生成建议.
func (m *Manager) generateSuggestions(score float64) []HealthSuggestion {
	suggestions := make([]HealthSuggestion, 0)

	if score < 50 {
		suggestions = append(suggestions, HealthSuggestion{
			Category:    "critical",
			Priority:    1,
			Title:       "系统健康评分较低",
			Description: "存在多个严重问题需要立即处理",
			Action:      "检查所有开放事件并优先处理严重级别事件",
		})
	}

	criticalCount := 0
	for _, inc := range m.incidents {
		if inc.Status == IncidentOpen && inc.Severity == SeverityCritical {
			criticalCount++
		}
	}

	if criticalCount > 0 {
		suggestions = append(suggestions, HealthSuggestion{
			Category:    "incidents",
			Priority:    1,
			Title:       fmt.Sprintf("有 %d 个严重事件待处理", criticalCount),
			Description: "严重事件可能影响系统可用性",
			Action:      "立即处理严重事件",
		})
	}

	return suggestions
}

func containsStr(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func matchCondition(condition string, event OpsEvent) bool {
	// 简单条件匹配实现
	return condition == "" || condition == "*"
}
