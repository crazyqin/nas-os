// Package monitor 提供告警规则引擎
// 支持灵活的规则配置和多种触发条件
package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AlertRuleEngine 告警规则引擎
type AlertRuleEngine struct {
	mu           sync.RWMutex
	rules        map[string]*AlertRuleConfig
	alertManager *AlertingManager
	configPath   string
}

// AlertRuleConfig 告警规则配置
type AlertRuleConfig struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	Enabled      bool              `json:"enabled"`
	Type         AlertRuleType     `json:"type"`
	Conditions   []RuleCondition   `json:"conditions"`
	Logic        LogicOperator     `json:"logic"`    // and, or
	Level        string            `json:"level"`    // warning, critical
	Duration     time.Duration     `json:"duration"` // 持续时间
	Cooldown     time.Duration     `json:"cooldown"` // 冷却时间
	Channels     []string          `json:"channels"` // 通知渠道
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
	LastTrigger  *time.Time        `json:"lastTrigger,omitempty"`
	TriggerCount int               `json:"triggerCount"`
}

// AlertRuleType 告警规则类型
type AlertRuleType string

const (
	RuleTypeCPU         AlertRuleType = "cpu"
	RuleTypeMemory      AlertRuleType = "memory"
	RuleTypeDisk        AlertRuleType = "disk"
	RuleTypeDiskHealth  AlertRuleType = "disk_health"
	RuleTypeNetwork     AlertRuleType = "network"
	RuleTypeTemperature AlertRuleType = "temperature"
	RuleTypeService     AlertRuleType = "service"
	RuleTypeBackup      AlertRuleType = "backup"
	RuleTypeCustom      AlertRuleType = "custom"
)

// LogicOperator 逻辑运算符
type LogicOperator string

const (
	LogicAnd LogicOperator = "and"
	LogicOr  LogicOperator = "or"
)

// RuleCondition 规则条件
type RuleCondition struct {
	Field       string        `json:"field"`
	Operator    CompareOp     `json:"operator"`
	Value       interface{}   `json:"value"`
	Duration    time.Duration `json:"duration,omitempty"`    // 持续时间阈值
	Aggregation *Aggregation  `json:"aggregation,omitempty"` // 聚合方式
}

// CompareOp 比较运算符
type CompareOp string

const (
	OpEqual        CompareOp = "eq"
	OpNotEqual     CompareOp = "ne"
	OpGreaterThan  CompareOp = "gt"
	OpGreaterEqual CompareOp = "gte"
	OpLessThan     CompareOp = "lt"
	OpLessEqual    CompareOp = "lte"
	OpContains     CompareOp = "contains"
	OpMatches      CompareOp = "matches"
	OpExists       CompareOp = "exists"
	OpChangeRate   CompareOp = "rate" // 变化率
)

// Aggregation 聚合方式
type Aggregation struct {
	Type   AggregationType `json:"type"`
	Window time.Duration   `json:"window"`
}

// AggregationType 聚合类型
type AggregationType string

const (
	AggAvg   AggregationType = "avg"
	AggMax   AggregationType = "max"
	AggMin   AggregationType = "min"
	AggSum   AggregationType = "sum"
	AggCount AggregationType = "count"
)

// MetricValue 指标值
type MetricValue struct {
	Name      string                 `json:"name"`
	Value     float64                `json:"value"`
	Labels    map[string]string      `json:"labels,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewAlertRuleEngine 创建告警规则引擎
func NewAlertRuleEngine(configPath string, alertMgr *AlertingManager) *AlertRuleEngine {
	engine := &AlertRuleEngine{
		rules:        make(map[string]*AlertRuleConfig),
		alertManager: alertMgr,
		configPath:   configPath,
	}

	// 加载配置
	if configPath != "" {
		_ = engine.LoadRules()
	}

	// 添加默认规则
	engine.addDefaultRules()

	return engine
}

// addDefaultRules 添加默认规则
func (e *AlertRuleEngine) addDefaultRules() {
	defaultRules := []*AlertRuleConfig{
		// CPU 告警规则
		{
			ID:          "cpu-warning",
			Name:        "CPU 使用率警告",
			Description: "CPU 使用率超过 80%，持续 5 分钟",
			Enabled:     true,
			Type:        RuleTypeCPU,
			Conditions: []RuleCondition{
				{Field: "cpu_usage", Operator: OpGreaterThan, Value: 80},
			},
			Logic:    LogicAnd,
			Level:    "warning",
			Duration: 5 * time.Minute,
			Cooldown: 10 * time.Minute,
		},
		{
			ID:          "cpu-critical",
			Name:        "CPU 使用率严重",
			Description: "CPU 使用率超过 95%，持续 2 分钟",
			Enabled:     true,
			Type:        RuleTypeCPU,
			Conditions: []RuleCondition{
				{Field: "cpu_usage", Operator: OpGreaterThan, Value: 95},
			},
			Logic:    LogicAnd,
			Level:    "critical",
			Duration: 2 * time.Minute,
			Cooldown: 5 * time.Minute,
		},

		// 内存告警规则
		{
			ID:          "memory-warning",
			Name:        "内存使用率警告",
			Description: "内存使用率超过 85%",
			Enabled:     true,
			Type:        RuleTypeMemory,
			Conditions: []RuleCondition{
				{Field: "memory_usage", Operator: OpGreaterThan, Value: 85},
			},
			Logic:    LogicAnd,
			Level:    "warning",
			Duration: 3 * time.Minute,
			Cooldown: 10 * time.Minute,
		},
		{
			ID:          "memory-critical",
			Name:        "内存使用率严重",
			Description: "内存使用率超过 95%",
			Enabled:     true,
			Type:        RuleTypeMemory,
			Conditions: []RuleCondition{
				{Field: "memory_usage", Operator: OpGreaterThan, Value: 95},
			},
			Logic:    LogicAnd,
			Level:    "critical",
			Duration: 1 * time.Minute,
			Cooldown: 5 * time.Minute,
		},

		// 磁盘空间告警规则
		{
			ID:          "disk-space-warning",
			Name:        "磁盘空间不足警告",
			Description: "磁盘使用率超过 85%",
			Enabled:     true,
			Type:        RuleTypeDisk,
			Conditions: []RuleCondition{
				{Field: "disk_usage", Operator: OpGreaterThan, Value: 85},
			},
			Logic:    LogicAnd,
			Level:    "warning",
			Duration: 5 * time.Minute,
			Cooldown: 30 * time.Minute,
		},
		{
			ID:          "disk-space-critical",
			Name:        "磁盘空间严重不足",
			Description: "磁盘使用率超过 95%",
			Enabled:     true,
			Type:        RuleTypeDisk,
			Conditions: []RuleCondition{
				{Field: "disk_usage", Operator: OpGreaterThan, Value: 95},
			},
			Logic:    LogicAnd,
			Level:    "critical",
			Duration: 1 * time.Minute,
			Cooldown: 10 * time.Minute,
		},

		// 磁盘健康告警规则
		{
			ID:          "disk-health-warning",
			Name:        "磁盘健康警告",
			Description: "磁盘健康评分低于 80",
			Enabled:     true,
			Type:        RuleTypeDiskHealth,
			Conditions: []RuleCondition{
				{Field: "health_score", Operator: OpLessThan, Value: 80},
			},
			Logic:    LogicAnd,
			Level:    "warning",
			Duration: 0,
			Cooldown: 24 * time.Hour,
		},
		{
			ID:          "disk-health-critical",
			Name:        "磁盘健康严重",
			Description: "磁盘健康评分低于 50 或健康状态为失败",
			Enabled:     true,
			Type:        RuleTypeDiskHealth,
			Conditions: []RuleCondition{
				{Field: "health_score", Operator: OpLessThan, Value: 50},
			},
			Logic:    LogicOr,
			Level:    "critical",
			Duration: 0,
			Cooldown: 1 * time.Hour,
		},

		// 温度告警规则
		{
			ID:          "temperature-warning",
			Name:        "磁盘温度警告",
			Description: "磁盘温度超过 50°C",
			Enabled:     true,
			Type:        RuleTypeTemperature,
			Conditions: []RuleCondition{
				{Field: "temperature", Operator: OpGreaterThan, Value: 50},
			},
			Logic:    LogicAnd,
			Level:    "warning",
			Duration: 5 * time.Minute,
			Cooldown: 30 * time.Minute,
		},
		{
			ID:          "temperature-critical",
			Name:        "磁盘温度严重",
			Description: "磁盘温度超过 65°C",
			Enabled:     true,
			Type:        RuleTypeTemperature,
			Conditions: []RuleCondition{
				{Field: "temperature", Operator: OpGreaterThan, Value: 65},
			},
			Logic:    LogicAnd,
			Level:    "critical",
			Duration: 1 * time.Minute,
			Cooldown: 10 * time.Minute,
		},
	}

	for _, rule := range defaultRules {
		if _, exists := e.rules[rule.ID]; !exists {
			rule.CreatedAt = time.Now()
			rule.UpdatedAt = time.Now()
			e.rules[rule.ID] = rule
		}
	}
}

// AddRule 添加规则
func (e *AlertRuleEngine) AddRule(rule *AlertRuleConfig) error {
	if rule.ID == "" {
		return fmt.Errorf("规则 ID 不能为空")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	e.rules[rule.ID] = rule

	// 保存到文件
	if e.configPath != "" {
		_ = e.saveRules()
	}

	return nil
}

// UpdateRule 更新规则
func (e *AlertRuleEngine) UpdateRule(rule *AlertRuleConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.rules[rule.ID]; !exists {
		return fmt.Errorf("规则不存在: %s", rule.ID)
	}

	rule.UpdatedAt = time.Now()
	e.rules[rule.ID] = rule

	if e.configPath != "" {
		_ = e.saveRules()
	}

	return nil
}

// DeleteRule 删除规则
func (e *AlertRuleEngine) DeleteRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.rules[id]; !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	delete(e.rules, id)

	if e.configPath != "" {
		_ = e.saveRules()
	}

	return nil
}

// GetRule 获取规则
func (e *AlertRuleEngine) GetRule(id string) (*AlertRuleConfig, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rule, exists := e.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}

	return rule, nil
}

// GetRules 获取所有规则
func (e *AlertRuleEngine) GetRules() []*AlertRuleConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rules := make([]*AlertRuleConfig, 0, len(e.rules))
	for _, rule := range e.rules {
		rules = append(rules, rule)
	}

	return rules
}

// GetRulesByType 按类型获取规则
func (e *AlertRuleEngine) GetRulesByType(ruleType AlertRuleType) []*AlertRuleConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rules := make([]*AlertRuleConfig, 0)
	for _, rule := range e.rules {
		if rule.Type == ruleType {
			rules = append(rules, rule)
		}
	}

	return rules
}

// EnableRule 启用规则
func (e *AlertRuleEngine) EnableRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	rule, exists := e.rules[id]
	if !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	rule.Enabled = true
	rule.UpdatedAt = time.Now()

	if e.configPath != "" {
		_ = e.saveRules()
	}

	return nil
}

// DisableRule 禁用规则
func (e *AlertRuleEngine) DisableRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	rule, exists := e.rules[id]
	if !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	rule.Enabled = false
	rule.UpdatedAt = time.Now()

	if e.configPath != "" {
		_ = e.saveRules()
	}

	return nil
}

// Evaluate 评估指标
func (e *AlertRuleEngine) Evaluate(metrics map[string]float64, labels map[string]string) []*AlertRuleConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	triggered := make([]*AlertRuleConfig, 0)

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		// 检查冷却时间
		if rule.LastTrigger != nil && time.Since(*rule.LastTrigger) < rule.Cooldown {
			continue
		}

		// 评估条件
		if e.evaluateRule(rule, metrics, labels) {
			triggered = append(triggered, rule)

			// 更新触发时间
			now := time.Now()
			rule.LastTrigger = &now
			rule.TriggerCount++

			// 触发告警
			if e.alertManager != nil {
				message := e.generateAlertMessage(rule, metrics, labels)
				e.alertManager.triggerAlert(
					string(rule.Type),
					rule.Level,
					message,
					labels["source"],
					map[string]interface{}{
						"rule_id":   rule.ID,
						"rule_name": rule.Name,
						"metrics":   metrics,
						"labels":    labels,
					},
				)
			}
		}
	}

	return triggered
}

// evaluateRule 评估单个规则
func (e *AlertRuleEngine) evaluateRule(rule *AlertRuleConfig, metrics map[string]float64, labels map[string]string) bool {
	results := make([]bool, len(rule.Conditions))

	for i, condition := range rule.Conditions {
		value, exists := metrics[condition.Field]
		if !exists {
			results[i] = false
			continue
		}

		results[i] = e.evaluateCondition(condition, value, labels)
	}

	// 根据逻辑运算符计算结果
	if rule.Logic == LogicOr {
		for _, r := range results {
			if r {
				return true
			}
		}
		return false
	}

	// 默认 AND 逻辑
	for _, r := range results {
		if !r {
			return false
		}
	}
	return true
}

// evaluateCondition 评估条件
func (e *AlertRuleEngine) evaluateCondition(condition RuleCondition, value float64, labels map[string]string) bool {
	threshold, ok := condition.Value.(float64)
	if !ok {
		// 尝试 int 转换
		if intVal, ok := condition.Value.(int); ok {
			threshold = float64(intVal)
		} else {
			return false
		}
	}

	switch condition.Operator {
	case OpEqual:
		return value == threshold
	case OpNotEqual:
		return value != threshold
	case OpGreaterThan:
		return value > threshold
	case OpGreaterEqual:
		return value >= threshold
	case OpLessThan:
		return value < threshold
	case OpLessEqual:
		return value <= threshold
	default:
		return false
	}
}

// generateAlertMessage 生成告警消息
func (e *AlertRuleEngine) generateAlertMessage(rule *AlertRuleConfig, metrics map[string]float64, labels map[string]string) string {
	msg := rule.Name

	if rule.Description != "" {
		msg = rule.Description
	}

	// 添加指标详情
	for _, condition := range rule.Conditions {
		if value, exists := metrics[condition.Field]; exists {
			msg += fmt.Sprintf(" [%s=%.1f]", condition.Field, value)
		}
	}

	return msg
}

// LoadRules 从文件加载规则
func (e *AlertRuleEngine) LoadRules() error {
	if e.configPath == "" {
		return nil
	}

	data, err := os.ReadFile(e.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var rules map[string]*AlertRuleConfig
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for id, rule := range rules {
		e.rules[id] = rule
	}

	return nil
}

// saveRules 保存规则到文件
func (e *AlertRuleEngine) saveRules() error {
	if e.configPath == "" {
		return nil
	}

	// 确保目录存在
	dir := filepath.Dir(e.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(e.rules, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(e.configPath, data, 0644)
}

// GetRuleStats 获取规则统计
func (e *AlertRuleEngine) GetRuleStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := map[string]interface{}{
		"total_rules":    len(e.rules),
		"enabled_rules":  0,
		"disabled_rules": 0,
		"by_type":        make(map[AlertRuleType]int),
		"total_triggers": 0,
	}

	byType := stats["by_type"].(map[AlertRuleType]int)

	for _, rule := range e.rules {
		if rule.Enabled {
			stats["enabled_rules"] = stats["enabled_rules"].(int) + 1
		} else {
			stats["disabled_rules"] = stats["disabled_rules"].(int) + 1
		}

		byType[rule.Type]++
		stats["total_triggers"] = stats["total_triggers"].(int) + rule.TriggerCount
	}

	return stats
}

// AlertRuleTemplate 告警规则模板
type AlertRuleTemplate struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Type        AlertRuleType   `json:"type"`
	Conditions  []RuleCondition `json:"conditions"`
	Logic       LogicOperator   `json:"logic"`
	Level       string          `json:"level"`
	Duration    time.Duration   `json:"duration"`
	Cooldown    time.Duration   `json:"cooldown"`
}

// GetRuleTemplates 获取规则模板
func GetRuleTemplates() []AlertRuleTemplate {
	return []AlertRuleTemplate{
		{
			ID:          "template-cpu-usage",
			Name:        "CPU 使用率模板",
			Description: "监控 CPU 使用率",
			Type:        RuleTypeCPU,
			Conditions: []RuleCondition{
				{Field: "cpu_usage", Operator: OpGreaterThan, Value: 80},
			},
			Logic:    LogicAnd,
			Level:    "warning",
			Duration: 5 * time.Minute,
			Cooldown: 10 * time.Minute,
		},
		{
			ID:          "template-memory-usage",
			Name:        "内存使用率模板",
			Description: "监控内存使用率",
			Type:        RuleTypeMemory,
			Conditions: []RuleCondition{
				{Field: "memory_usage", Operator: OpGreaterThan, Value: 85},
			},
			Logic:    LogicAnd,
			Level:    "warning",
			Duration: 3 * time.Minute,
			Cooldown: 10 * time.Minute,
		},
		{
			ID:          "template-disk-usage",
			Name:        "磁盘使用率模板",
			Description: "监控磁盘使用率",
			Type:        RuleTypeDisk,
			Conditions: []RuleCondition{
				{Field: "disk_usage", Operator: OpGreaterThan, Value: 85},
			},
			Logic:    LogicAnd,
			Level:    "warning",
			Duration: 5 * time.Minute,
			Cooldown: 30 * time.Minute,
		},
		{
			ID:          "template-disk-health",
			Name:        "磁盘健康模板",
			Description: "监控磁盘健康状态",
			Type:        RuleTypeDiskHealth,
			Conditions: []RuleCondition{
				{Field: "health_score", Operator: OpLessThan, Value: 80},
			},
			Logic: LogicAnd,
			Level: "warning",
		},
		{
			ID:          "template-temperature",
			Name:        "温度监控模板",
			Description: "监控设备温度",
			Type:        RuleTypeTemperature,
			Conditions: []RuleCondition{
				{Field: "temperature", Operator: OpGreaterThan, Value: 50},
			},
			Logic:    LogicAnd,
			Level:    "warning",
			Duration: 5 * time.Minute,
			Cooldown: 30 * time.Minute,
		},
	}
}
