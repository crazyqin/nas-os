// Package smartretention 实现智能数据保留策略管理
// 对标: 群晖 合规保留策略 + TrueNAS 数据保留管理
// 自动管理数据保留期限, 支持合规要求(GDPR/HIPAA/SOX)
package smartretention

import (
	"fmt"
	"sync"
	"time"
)

// RetentionStatus 保留状态
type RetentionStatus string

const (
	StatusActive    RetentionStatus = "active"    // 活跃
	StatusLocked    RetentionStatus = "locked"    // 锁定(WORM)
	StatusExpiring  RetentionStatus = "expiring"  // 即将过期
	StatusExpired   RetentionStatus = "expired"   // 已过期
	StatusProtected RetentionStatus = "protected" // 受保护(不可删除)
	StatusDeleted  RetentionStatus = "deleted"  // 已删除
)

// ComplianceType 合规类型
type ComplianceType string

const (
	ComplianceGDPR    ComplianceType = "GDPR"
	ComplianceHIPAA   ComplianceType = "HIPAA"
	ComplianceSOX     ComplianceType = "SOX"
	CompliancePCI     ComplianceType = "PCI_DSS"
	ComplianceISO27001 ComplianceType = "ISO_27001"
	ComplianceCustom  ComplianceType = "CUSTOM"
)

// RetentionRule 保留规则
type RetentionRule struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Compliance     ComplianceType  `json:"compliance"`
	RetentionDays  int             `json:"retention_days"`   // 保留天数
	WORMEnabled    bool            `json:"worm_enabled"`     // WORM(一次写入多次读取)
	AutoDelete     bool            `json:"auto_delete"`      // 自动删除
	WarningDays    int             `json:"warning_days"`     // 提前提醒天数
	Enabled        bool            `json:"enabled"`
	Description    string          `json:"description"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// RetainedData 保留数据
type RetainedData struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Path         string          `json:"path"`
	Size         int64           `json:"size"`
	RuleID       string          `json:"rule_id"`
	Status       RetentionStatus `json:"status"`
	RetainUntil  time.Time       `json:"retain_until"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Owner        string          `json:"owner"`
	LockedAt     *time.Time      `json:"locked_at,omitempty"`
	DeletedAt    *time.Time      `json:"deleted_at,omitempty"`
	Metadata     map[string]string `json:"metadata"`
}

// RetentionEvent 保留事件
type RetentionEvent struct {
	ID        string          `json:"id"`
	DataID    string          `json:"data_id"`
	EventType string          `json:"event_type"` // created, locked, warning, expired, deleted
	Message   string          `json:"message"`
	Timestamp time.Time       `json:"timestamp"`
}

// Manager 保留策略管理器
type Manager struct {
	mu           sync.RWMutex
	rules        map[string]*RetentionRule
	data         map[string]*RetainedData
	events       []RetentionEvent
	totalData    int
	totalExpired int
	totalDeleted int
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		rules:  make(map[string]*RetentionRule),
		data:   make(map[string]*RetainedData),
		events: make([]RetentionEvent, 0),
	}
	m.registerDefaults()
	return m
}

// registerDefaults 注册默认规则
func (m *Manager) registerDefaults() {
	defaults := []RetentionRule{
		{
			ID: "rule-gdpr-default", Name: "GDPR默认保留", Compliance: ComplianceGDPR,
			RetentionDays: 365 * 3, WORMEnabled: true, AutoDelete: false,
			WarningDays: 30, Enabled: true, Description: "GDPR合规数据保留3年",
		},
		{
			ID: "rule-hipaa-default", Name: "HIPAA默认保留", Compliance: ComplianceHIPAA,
			RetentionDays: 365 * 6, WORMEnabled: true, AutoDelete: false,
			WarningDays: 60, Enabled: true, Description: "HIPAA合规数据保留6年",
		},
		{
			ID: "rule-sox-default", Name: "SOX默认保留", Compliance: ComplianceSOX,
			RetentionDays: 365 * 7, WORMEnabled: true, AutoDelete: false,
			WarningDays: 90, Enabled: true, Description: "SOX合规数据保留7年",
		},
		{
			ID: "rule-general-default", Name: "通用保留", Compliance: ComplianceCustom,
			RetentionDays: 365, WORMEnabled: false, AutoDelete: true,
			WarningDays: 14, Enabled: true, Description: "通用数据保留1年",
		},
	}

	now := time.Now()
	for i := range defaults {
		defaults[i].CreatedAt = now
		defaults[i].UpdatedAt = now
		m.rules[defaults[i].ID] = &defaults[i]
	}
}

// CreateRule 创建规则
func (m *Manager) CreateRule(rule *RetentionRule) error {
	if rule.ID == "" {
		return fmt.Errorf("规则ID不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[rule.ID]; exists {
		return fmt.Errorf("规则 %s 已存在", rule.ID)
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	return nil
}

// GetRule 获取规则
func (m *Manager) GetRule(ruleID string) (*RetentionRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.rules[ruleID]
	if !ok {
		return nil, fmt.Errorf("规则 %s 不存在", ruleID)
	}
	return rule, nil
}

// ListRules 列出规则
func (m *Manager) ListRules() []*RetentionRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*RetentionRule, 0)
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	return rules
}

// RegisterData 注册保留数据
func (m *Manager) RegisterData(data *RetainedData) error {
	if data.ID == "" {
		return fmt.Errorf("数据ID不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.data[data.ID]; exists {
		return fmt.Errorf("数据 %s 已存在", data.ID)
	}

	// 查找规则
	rule, ok := m.rules[data.RuleID]
	if !ok {
		return fmt.Errorf("规则 %s 不存在", data.RuleID)
	}

	now := time.Now()
	data.CreatedAt = now
	data.UpdatedAt = now
	data.Status = StatusActive
	data.RetainUntil = now.AddDate(0, 0, rule.RetentionDays)
	data.Metadata = make(map[string]string)

	m.data[data.ID] = data
	m.totalData++

	// 记录事件
	m.addEvent(data.ID, "created", fmt.Sprintf("数据 %s 注册保留, 保留至 %s", data.Name, data.RetainUntil.Format("2006-01-02")))
	return nil
}

// LockData 锁定数据(WORM)
func (m *Manager) LockData(dataID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, ok := m.data[dataID]
	if !ok {
		return fmt.Errorf("数据 %s 不存在", dataID)
	}

	if data.Status == StatusLocked {
		return fmt.Errorf("数据 %s 已被锁定", dataID)
	}

	now := time.Now()
	data.Status = StatusLocked
	data.LockedAt = &now
	data.UpdatedAt = now

	m.addEvent(dataID, "locked", fmt.Sprintf("数据 %s 已锁定(WORM)", data.Name))
	return nil
}

// GetData 获取数据
func (m *Manager) GetData(dataID string) (*RetainedData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, ok := m.data[dataID]
	if !ok {
		return nil, fmt.Errorf("数据 %s 不存在", dataID)
	}
	return data, nil
}

// ListData 列出数据
func (m *Manager) ListData(status RetentionStatus, limit int) []*RetainedData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]*RetainedData, 0)
	for _, d := range m.data {
		if status == "" || d.Status == status {
			items = append(items, d)
			if limit > 0 && len(items) >= limit {
				break
			}
		}
	}
	return items
}

// EvaluateRetention 评估保留状态
func (m *Manager) EvaluateRetention() []RetentionEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	newEvents := make([]RetentionEvent, 0)

	for _, data := range m.data {
		if data.Status == StatusLocked || data.Status == StatusDeleted {
			continue
		}

		rule, ok := m.rules[data.RuleID]
		if !ok {
			continue
		}

		daysUntilExpiry := int(data.RetainUntil.Sub(now).Hours() / 24)

		if daysUntilExpiry <= 0 {
			// 已过期
			data.Status = StatusExpired
			data.UpdatedAt = now
			m.totalExpired++
			event := m.addEvent(data.ID, "expired", fmt.Sprintf("数据 %s 保留期已过期", data.Name))
			newEvents = append(newEvents, event)

			// 自动删除
			if rule.AutoDelete && data.Status != StatusLocked {
				deletedAt := now
				data.DeletedAt = &deletedAt
				data.Status = StatusDeleted
				m.totalDeleted++
				event := m.addEvent(data.ID, "deleted", fmt.Sprintf("数据 %s 已自动删除", data.Name))
				newEvents = append(newEvents, event)
			}
		} else if rule.WarningDays > 0 && daysUntilExpiry <= rule.WarningDays {
			// 即将过期
			if data.Status != StatusExpiring {
				data.Status = StatusExpiring
				data.UpdatedAt = now
				event := m.addEvent(data.ID, "warning", fmt.Sprintf("数据 %s 将在 %d 天后过期", data.Name, daysUntilExpiry))
				newEvents = append(newEvents, event)
			}
		}
	}

	return newEvents
}

// addEvent 添加事件
func (m *Manager) addEvent(dataID, eventType, message string) RetentionEvent {
	event := RetentionEvent{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		DataID:    dataID,
		EventType: eventType,
		Message:   message,
		Timestamp: time.Now(),
	}
	m.events = append(m.events, event)
	return event
}

// GetEvents 获取事件
func (m *Manager) GetEvents(dataID string, limit int) []RetentionEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]RetentionEvent, 0)
	for _, e := range m.events {
		if dataID == "" || e.DataID == dataID {
			events = append(events, e)
		}
	}

	if limit > 0 && len(events) > limit {
		return events[len(events)-limit:]
	}
	return events
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statusCount := make(map[string]int)
	for _, d := range m.data {
		statusCount[string(d.Status)]++
	}

	return map[string]interface{}{
		"total_data":    m.totalData,
		"total_expired": m.totalExpired,
		"total_deleted": m.totalDeleted,
		"total_rules":   len(m.rules),
		"status_distribution": statusCount,
		"total_events":  len(m.events),
	}
}

// ExtendRetention 延长保留期
func (m *Manager) ExtendRetention(dataID string, additionalDays int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, ok := m.data[dataID]
	if !ok {
		return fmt.Errorf("数据 %s 不存在", dataID)
	}

	if data.Status == StatusLocked {
		return fmt.Errorf("数据 %s 已锁定, 无法修改保留期", dataID)
	}

	data.RetainUntil = data.RetainUntil.AddDate(0, 0, additionalDays)
	data.UpdatedAt = time.Now()
	data.Status = StatusActive

	m.addEvent(dataID, "extended", fmt.Sprintf("数据 %s 保留期延长 %d 天至 %s", data.Name, additionalDays, data.RetainUntil.Format("2006-01-02")))
	return nil
}

// DeleteData 删除数据
func (m *Manager) DeleteData(dataID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, ok := m.data[dataID]
	if !ok {
		return fmt.Errorf("数据 %s 不存在", dataID)
	}

	if data.Status == StatusLocked {
		return fmt.Errorf("数据 %s 已锁定(WORM), 无法删除", dataID)
	}

	now := time.Now()
	data.Status = StatusDeleted
	data.DeletedAt = &now
	data.UpdatedAt = now
	m.totalDeleted++

	m.addEvent(dataID, "deleted", fmt.Sprintf("数据 %s 已手动删除", data.Name))
	return nil
}
