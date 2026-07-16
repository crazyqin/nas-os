// Package incidentmgr 提供事件管理功能，支持事件创建、状态流转、时间线记录、根因分析等。
package incidentmgr

import (
	"fmt"
	"sync"
	"time"
)

// IncidentSeverity 事件严重程度.
type IncidentSeverity string

const (
	Sev1Critical IncidentSeverity = "critical"
	Sev2High     IncidentSeverity = "high"
	Sev3Medium   IncidentSeverity = "medium"
	Sev4Low      IncidentSeverity = "low"
)

// IncidentStatus 事件状态.
type IncidentStatus string

const (
	StatusOpen          IncidentStatus = "open"
	StatusAcknowledged  IncidentStatus = "acknowledged"
	StatusInvestigating IncidentStatus = "investigating"
	StatusResolved      IncidentStatus = "resolved"
	StatusClosed        IncidentStatus = "closed"
)

// TimelineEvent 时间线事件类型.
type TimelineEventType string

const (
	TimelineEventCreated      TimelineEventType = "created"
	TimelineEventUpdated      TimelineEventType = "updated"
	TimelineEventStatusChange TimelineEventType = "status_change"
	TimelineEventAssigned     TimelineEventType = "assigned"
	TimelineEventComment      TimelineEventType = "comment"
	TimelineEventEscalated    TimelineEventType = "escalated"
	TimelineEventResolved     TimelineEventType = "resolved"
)

// Incident 事件.
type Incident struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Severity    IncidentSeverity `json:"severity"`
	Status      IncidentStatus   `json:"status"`
	Assignee    string           `json:"assignee,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// TimelineEvent 时间线事件.
type TimelineEvent struct {
	ID          string            `json:"id"`
	IncidentID  string            `json:"incident_id"`
	Type        TimelineEventType `json:"type"`
	Description string            `json:"description"`
	Operator    string            `json:"operator,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// RCAReport 根因分析报告.
type RCAReport struct {
	ID             string    `json:"id"`
	IncidentID     string    `json:"incident_id"`
	RootCause      string    `json:"root_cause"`
	ImpactScope    string    `json:"impact_scope"`
	FixActions     []string  `json:"fix_actions"`
	PreventActions []string  `json:"prevent_actions"`
	CreatedAt      time.Time `json:"created_at"`
}

// Manager 事件管理器.
type Manager struct {
	mu        sync.RWMutex
	incidents map[string]*Incident
	timeline  map[string][]*TimelineEvent
	rca       map[string]*RCAReport
	counter   int
}

// NewManager 创建事件管理器.
func NewManager() *Manager {
	return &Manager{
		incidents: make(map[string]*Incident),
		timeline:  make(map[string][]*TimelineEvent),
		rca:       make(map[string]*RCAReport),
	}
}

// generateIncidentID 生成事件编号: INC-YYYYMMDD-XXXX.
func (m *Manager) generateIncidentID() string {
	m.counter++
	return fmt.Sprintf("INC-%s-%04d", time.Now().Format("20060102"), m.counter)
}

// generateID 生成唯一 ID.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// CreateIncident 创建事件.
func (m *Manager) CreateIncident(inc Incident) (*Incident, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if inc.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if inc.Severity == "" {
		inc.Severity = Sev3Medium
	}

	id := m.generateIncidentID()
	now := time.Now()

	incident := &Incident{
		ID:          id,
		Title:       inc.Title,
		Description: inc.Description,
		Severity:    inc.Severity,
		Status:      StatusOpen,
		Assignee:    inc.Assignee,
		Tags:        inc.Tags,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.incidents[id] = incident

	// 添加创建事件到时间线
	event := &TimelineEvent{
		ID:          generateID(),
		IncidentID:  id,
		Type:        TimelineEventCreated,
		Description: fmt.Sprintf("事件创建: %s", inc.Title),
		CreatedAt:   now,
	}
	m.timeline[id] = []*TimelineEvent{event}

	return incident, nil
}

// GetIncident 获取事件.
func (m *Manager) GetIncident(id string) (*Incident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inc, ok := m.incidents[id]
	if !ok {
		return nil, fmt.Errorf("incident not found: %s", id)
	}
	return inc, nil
}

// ListIncidents 列出事件（按状态过滤）.
func (m *Manager) ListIncidents(status IncidentStatus) ([]Incident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Incident, 0)
	for _, inc := range m.incidents {
		if status == "" || inc.Status == status {
			result = append(result, *inc)
		}
	}
	return result, nil
}

// UpdateIncidentStatus 更新事件状态.
func (m *Manager) UpdateIncidentStatus(id string, status IncidentStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inc, ok := m.incidents[id]
	if !ok {
		return fmt.Errorf("incident not found: %s", id)
	}

	// 验证状态流转
	if !isValidStatusTransition(inc.Status, status) {
		return fmt.Errorf("invalid status transition: %s -> %s", inc.Status, status)
	}

	now := time.Now()
	inc.Status = status
	inc.UpdatedAt = now

	// 添加状态变更事件到时间线
	event := &TimelineEvent{
		ID:          generateID(),
		IncidentID:  id,
		Type:        TimelineEventStatusChange,
		Description: fmt.Sprintf("状态变更: %s -> %s", inc.Status, status),
		CreatedAt:   now,
	}
	m.timeline[id] = append(m.timeline[id], event)

	return nil
}

// isValidStatusTransition 验证状态流转是否合法.
func isValidStatusTransition(from, to IncidentStatus) bool {
	validTransitions := map[IncidentStatus][]IncidentStatus{
		StatusOpen:          {StatusAcknowledged, StatusInvestigating, StatusClosed},
		StatusAcknowledged:  {StatusInvestigating, StatusClosed},
		StatusInvestigating: {StatusResolved, StatusClosed},
		StatusResolved:      {StatusClosed, StatusOpen},
		StatusClosed:        {StatusOpen},
	}

	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// AssignIncident 分配事件负责人.
func (m *Manager) AssignIncident(id string, assignee string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inc, ok := m.incidents[id]
	if !ok {
		return fmt.Errorf("incident not found: %s", id)
	}

	now := time.Now()
	oldAssignee := inc.Assignee
	inc.Assignee = assignee
	inc.UpdatedAt = now

	// 添加分配事件到时间线
	description := fmt.Sprintf("负责人分配给: %s", assignee)
	if oldAssignee != "" {
		description = fmt.Sprintf("负责人从 %s 变更为 %s", oldAssignee, assignee)
	}

	event := &TimelineEvent{
		ID:          generateID(),
		IncidentID:  id,
		Type:        TimelineEventAssigned,
		Description: description,
		CreatedAt:   now,
	}
	m.timeline[id] = append(m.timeline[id], event)

	return nil
}

// AddTimelineEvent 添加时间线事件.
func (m *Manager) AddTimelineEvent(incidentID string, event TimelineEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.incidents[incidentID]; !ok {
		return fmt.Errorf("incident not found: %s", incidentID)
	}

	event.ID = generateID()
	event.IncidentID = incidentID
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	m.timeline[incidentID] = append(m.timeline[incidentID], &event)
	return nil
}

// GetTimeline 获取事件时间线.
func (m *Manager) GetTimeline(incidentID string) ([]TimelineEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.incidents[incidentID]; !ok {
		return nil, fmt.Errorf("incident not found: %s", incidentID)
	}

	events := m.timeline[incidentID]
	result := make([]TimelineEvent, len(events))
	for i, e := range events {
		result[i] = *e
	}
	return result, nil
}

// CreateRCA 创建根因分析报告.
func (m *Manager) CreateRCA(incidentID string, rca RCAReport) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.incidents[incidentID]; !ok {
		return fmt.Errorf("incident not found: %s", incidentID)
	}

	if rca.RootCause == "" {
		return fmt.Errorf("root cause is required")
	}

	rca.ID = generateID()
	rca.IncidentID = incidentID
	rca.CreatedAt = time.Now()

	m.rca[incidentID] = &rca
	return nil
}

// GetRCA 获取根因分析报告.
func (m *Manager) GetRCA(incidentID string) (*RCAReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rca, ok := m.rca[incidentID]
	if !ok {
		return nil, fmt.Errorf("RCA not found for incident: %s", incidentID)
	}
	return rca, nil
}

// GetIncidentStats 获取事件统计.
func (m *Manager) GetIncidentStats() (map[string]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]int)

	// 按状态统计
	stats["total"] = len(m.incidents)
	stats["open"] = 0
	stats["acknowledged"] = 0
	stats["investigating"] = 0
	stats["resolved"] = 0
	stats["closed"] = 0

	// 按严重程度统计
	stats["critical"] = 0
	stats["high"] = 0
	stats["medium"] = 0
	stats["low"] = 0

	for _, inc := range m.incidents {
		// 状态统计
		switch inc.Status {
		case StatusOpen:
			stats["open"]++
		case StatusAcknowledged:
			stats["acknowledged"]++
		case StatusInvestigating:
			stats["investigating"]++
		case StatusResolved:
			stats["resolved"]++
		case StatusClosed:
			stats["closed"]++
		}

		// 严重程度统计
		switch inc.Severity {
		case Sev1Critical:
			stats["critical"]++
		case Sev2High:
			stats["high"]++
		case Sev3Medium:
			stats["medium"]++
		case Sev4Low:
			stats["low"]++
		}
	}

	return stats, nil
}

// EscalateIncident 升级事件（自动提升严重程度）.
func (m *Manager) EscalateIncident(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inc, ok := m.incidents[id]
	if !ok {
		return fmt.Errorf("incident not found: %s", id)
	}

	now := time.Now()
	oldSeverity := inc.Severity

	// 自动升级严重程度
	switch inc.Severity {
	case Sev4Low:
		inc.Severity = Sev3Medium
	case Sev3Medium:
		inc.Severity = Sev2High
	case Sev2High:
		inc.Severity = Sev1Critical
	case Sev1Critical:
		return fmt.Errorf("incident already at critical severity")
	}

	inc.UpdatedAt = now

	// 添加升级事件到时间线
	event := &TimelineEvent{
		ID:          generateID(),
		IncidentID:  id,
		Type:        TimelineEventEscalated,
		Description: fmt.Sprintf("事件升级: %s -> %s", oldSeverity, inc.Severity),
		CreatedAt:   now,
	}
	m.timeline[id] = append(m.timeline[id], event)

	return nil
}
