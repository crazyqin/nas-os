package dashboard

import (
	"fmt"
	"sync"
	"time"
)

// AlertSeverity 告警严重级别.
type AlertSeverity string

const (
	AlertInfo     AlertSeverity = "info"
	AlertWarning  AlertSeverity = "warning"
	AlertCritical AlertSeverity = "critical"
)

// AlertOperator 比较运算符.
type AlertOperator string

const (
	OpGT  AlertOperator = ">"
	OpGTE AlertOperator = ">="
	OpLT  AlertOperator = "<"
	OpLTE AlertOperator = "<="
	OpEQ  AlertOperator = "=="
)

// AlertRule 告警规则.
type AlertRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Metric      string        `json:"metric"`       // cpu/memory/disk/network/process_count
	Operator    AlertOperator `json:"operator"`
	Threshold   float64       `json:"threshold"`
	Severity    AlertSeverity `json:"severity"`
	Duration    time.Duration `json:"duration"`     // 持续多久才触发
	Enabled     bool          `json:"enabled"`
	Description string        `json:"description"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

// AlertEvent 告警事件.
type AlertEvent struct {
	ID        string        `json:"id"`
	RuleID    string        `json:"ruleId"`
	RuleName  string        `json:"ruleName"`
	Severity  AlertSeverity `json:"severity"`
	Metric    string        `json:"metric"`
	Value     float64       `json:"value"`
	Threshold float64       `json:"threshold"`
	Message   string        `json:"message"`
	Timestamp time.Time     `json:"timestamp"`
	Resolved  bool          `json:"resolved"`
	ResolvedAt *time.Time   `json:"resolvedAt,omitempty"`
}

// AlertManager 告警规则管理器.
type AlertManager struct {
	mu     sync.RWMutex
	rules  map[string]*AlertRule
	events []AlertEvent
	maxEvents int
}

// NewAlertManager 创建告警管理器.
func NewAlertManager(maxEvents int) *AlertManager {
	if maxEvents <= 0 {
		maxEvents = 500
	}
	return &AlertManager{
		rules:     make(map[string]*AlertRule),
		events:    make([]AlertEvent, 0),
		maxEvents: maxEvents,
	}
}

// AddRule 添加告警规则.
func (am *AlertManager) AddRule(rule *AlertRule) error {
	if rule.ID == "" {
		return fmt.Errorf("规则 ID 不能为空")
	}
	if rule.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}
	if rule.Metric == "" {
		return fmt.Errorf("监控指标不能为空")
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.rules[rule.ID]; exists {
		return fmt.Errorf("规则 %s 已存在", rule.ID)
	}

	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	am.rules[rule.ID] = rule
	return nil
}

// UpdateRule 更新告警规则.
func (am *AlertManager) UpdateRule(rule *AlertRule) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	existing, ok := am.rules[rule.ID]
	if !ok {
		return fmt.Errorf("规则 %s 不存在", rule.ID)
	}
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	am.rules[rule.ID] = rule
	return nil
}

// DeleteRule 删除告警规则.
func (am *AlertManager) DeleteRule(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	if _, exists := am.rules[id]; !exists {
		return fmt.Errorf("规则 %s 不存在", id)
	}
	delete(am.rules, id)
	return nil
}

// GetRule 获取告警规则.
func (am *AlertManager) GetRule(id string) (*AlertRule, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	rule, ok := am.rules[id]
	if !ok {
		return nil, fmt.Errorf("规则 %s 不存在", id)
	}
	return rule, nil
}

// ListRules 列出所有规则.
func (am *AlertManager) ListRules() []*AlertRule {
	am.mu.RLock()
	defer am.mu.RUnlock()
	result := make([]*AlertRule, 0, len(am.rules))
	for _, r := range am.rules {
		result = append(result, r)
	}
	return result
}

// Evaluate 评估指标是否触发告警.
func (am *AlertManager) Evaluate(metric string, value float64) []AlertEvent {
	am.mu.Lock()
	defer am.mu.Unlock()

	var events []AlertEvent
	for _, rule := range am.rules {
		if !rule.Enabled || rule.Metric != metric {
			continue
		}

		triggered := false
		switch rule.Operator {
		case OpGT:
			triggered = value > rule.Threshold
		case OpGTE:
			triggered = value >= rule.Threshold
		case OpLT:
			triggered = value < rule.Threshold
		case OpLTE:
			triggered = value <= rule.Threshold
		case OpEQ:
			triggered = value == rule.Threshold
		}

		if triggered {
			event := AlertEvent{
				ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				Severity:  rule.Severity,
				Metric:    metric,
				Value:     value,
				Threshold: rule.Threshold,
				Message:   fmt.Sprintf("%s: %s %s %.2f (阈值 %.2f)", rule.Name, metric, rule.Operator, value, rule.Threshold),
				Timestamp: time.Now(),
			}
			events = append(events, event)
			am.events = append(am.events, event)
		}
	}

	// 截断旧事件
	if len(am.events) > am.maxEvents {
		am.events = am.events[len(am.events)-am.maxEvents:]
	}

	return events
}

// GetEvents 获取告警事件.
func (am *AlertManager) GetEvents(limit int) []AlertEvent {
	am.mu.RLock()
	defer am.mu.RUnlock()
	if limit <= 0 || limit > len(am.events) {
		limit = len(am.events)
	}
	start := len(am.events) - limit
	result := make([]AlertEvent, limit)
	copy(result, am.events[start:])
	return result
}

// GetEventsBySeverity 按严重级别获取事件.
func (am *AlertManager) GetEventsBySeverity(severity AlertSeverity, limit int) []AlertEvent {
	am.mu.RLock()
	defer am.mu.RUnlock()
	var filtered []AlertEvent
	for i := len(am.events) - 1; i >= 0; i-- {
		if am.events[i].Severity == severity {
			filtered = append(filtered, am.events[i])
			if limit > 0 && len(filtered) >= limit {
				break
			}
		}
	}
	return filtered
}

// ResolveEvent 标记事件为已解决.
func (am *AlertManager) ResolveEvent(eventID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	for i := range am.events {
		if am.events[i].ID == eventID {
			now := time.Now()
			am.events[i].Resolved = true
			am.events[i].ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("事件 %s 不存在", eventID)
}

// RuleCount 返回规则数.
func (am *AlertManager) RuleCount() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return len(am.rules)
}

// EventCount 返回事件数.
func (am *AlertManager) EventCount() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return len(am.events)
}
