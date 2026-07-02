package anomalydetect

import (
	"fmt"
	"sync"
)

// ==================== 规则引擎 ====================

// RulePriority 规则优先级.
type RulePriority string

const (
	PriorityCritical RulePriority = "critical" // 严重
	PriorityWarning  RulePriority = "warning"  // 警告
	PriorityInfo     RulePriority = "info"     // 信息
)

// LogicalOp 逻辑运算符.
type LogicalOp string

const (
	LogicalAND LogicalOp = "AND" // 与
	LogicalOR  LogicalOp = "OR"  // 或
)

// ComparisonOp 比较运算符.
type ComparisonOp string

const (
	OpGT  ComparisonOp = ">"  // 大于
	OpGTE ComparisonOp = ">=" // 大于等于
	OpLT  ComparisonOp = "<"  // 小于
	OpLTE ComparisonOp = "<=" // 小于等于
	OpEQ  ComparisonOp = "==" // 等于
	OpNEQ ComparisonOp = "!=" // 不等于
)

// RuleCondition 单个规则条件.
type RuleCondition struct {
	MetricType MetricType   `json:"metric_type"` // 指标类型
	Operator   ComparisonOp `json:"operator"`    // 比较运算符
	Threshold  float64      `json:"threshold"`   // 阈值
}

// Rule 自定义检测规则.
type Rule struct {
	ID          string          `json:"id"`          // 规则 ID
	Name        string          `json:"name"`        // 规则名称
	Description string          `json:"description"` // 规则描述
	Priority    RulePriority    `json:"priority"`    // 优先级
	Conditions  []RuleCondition `json:"conditions"`  // 条件列表
	Logic       LogicalOp       `json:"logic"`       // 条件间逻辑
	Enabled     bool            `json:"enabled"`     // 是否启用
	Message     string          `json:"message"`     // 告警消息模板
}

// RuleMatchResult 规则匹配结果.
type RuleMatchResult struct {
	Rule      *Rule  `json:"rule"`      // 匹配的规则
	Triggered bool   `json:"triggered"` // 是否触发
	Message   string `json:"message"`   // 描述
}

// RuleEngine 规则引擎.
type RuleEngine struct {
	mu    sync.RWMutex
	rules map[string]*Rule // 规则集（ID -> Rule）
}

// NewRuleEngine 创建规则引擎并加载默认规则.
func NewRuleEngine() *RuleEngine {
	engine := &RuleEngine{
		rules: make(map[string]*Rule),
	}
	// 加载预定义规则
	for _, rule := range DefaultRules() {
		engine.rules[rule.ID] = rule
	}
	return engine
}

// DefaultRules 返回 NAS 系统预定义的检测规则集.
func DefaultRules() []*Rule {
	return []*Rule{
		{
			ID:          "cpu-high-continuous",
			Name:        "CPU 持续高负载",
			Description: "CPU 使用率持续超过 90%",
			Priority:    PriorityCritical,
			Conditions: []RuleCondition{
				{MetricType: MetricCPU, Operator: OpGT, Threshold: 90},
			},
			Logic:   LogicalAND,
			Enabled: true,
			Message: "CPU 使用率持续超过 90%，当前值 %.2f%%",
		},
		{
			ID:          "memory-high",
			Name:        "内存使用率过高",
			Description: "内存使用率超过 85%",
			Priority:    PriorityWarning,
			Conditions: []RuleCondition{
				{MetricType: MetricMemory, Operator: OpGT, Threshold: 85},
			},
			Logic:   LogicalAND,
			Enabled: true,
			Message: "内存使用率超过 85%%，当前值 %.2f%%",
		},
		{
			ID:          "disk-space-low",
			Name:        "磁盘空间不足",
			Description: "磁盘可用空间低于 10%",
			Priority:    PriorityCritical,
			Conditions: []RuleCondition{
				{MetricType: MetricDisk, Operator: OpGT, Threshold: 90}, // 使用率 > 90% = 剩余 < 10%
			},
			Logic:   LogicalAND,
			Enabled: true,
			Message: "磁盘空间不足，使用率已达 %.2f%%",
		},
		{
			ID:          "temperature-high",
			Name:        "温度过高",
			Description: "设备温度超过 80°C",
			Priority:    PriorityCritical,
			Conditions: []RuleCondition{
				{MetricType: MetricTemp, Operator: OpGT, Threshold: 80},
			},
			Logic:   LogicalAND,
			Enabled: true,
			Message: "设备温度过高 %.1f°C，超过安全阈值 80°C",
		},
		{
			ID:          "temperature-warn",
			Name:        "温度偏高",
			Description: "设备温度超过 65°C",
			Priority:    PriorityWarning,
			Conditions: []RuleCondition{
				{MetricType: MetricTemp, Operator: OpGT, Threshold: 65},
			},
			Logic:   LogicalAND,
			Enabled: true,
			Message: "设备温度偏高 %.1f°C，建议检查散热",
		},
		{
			ID:          "network-spike",
			Name:        "网络流量突增",
			Description: "网络流量超过 800 MB/s",
			Priority:    PriorityWarning,
			Conditions: []RuleCondition{
				{MetricType: MetricNet, Operator: OpGT, Threshold: 800},
			},
			Logic:   LogicalAND,
			Enabled: true,
			Message: "网络流量突增，当前 %.2f MB/s",
		},
		{
			ID:          "cpu-memory-both-high",
			Name:        "CPU 与内存同时过高",
			Description: "CPU > 80% 且内存 > 80%，系统负载过重",
			Priority:    PriorityCritical,
			Conditions: []RuleCondition{
				{MetricType: MetricCPU, Operator: OpGT, Threshold: 80},
				{MetricType: MetricMemory, Operator: OpGT, Threshold: 80},
			},
			Logic:   LogicalAND,
			Enabled: true,
			Message: "系统负载过重: CPU %.2f%%, 内存 %.2f%%",
		},
		{
			ID:          "disk-critical-or-temp-high",
			Name:        "磁盘满或温度高",
			Description: "磁盘使用率 > 95% 或温度 > 75°C",
			Priority:    PriorityCritical,
			Conditions: []RuleCondition{
				{MetricType: MetricDisk, Operator: OpGT, Threshold: 95},
				{MetricType: MetricTemp, Operator: OpGT, Threshold: 75},
			},
			Logic:   LogicalOR,
			Enabled: true,
			Message: "紧急告警: 磁盘 %.2f%%, 温度 %.1f°C",
		},
	}
}

// AddRule 添加自定义规则.
func (re *RuleEngine) AddRule(rule *Rule) error {
	if rule.ID == "" {
		return fmt.Errorf("规则 ID 不能为空")
	}
	if len(rule.Conditions) == 0 {
		return fmt.Errorf("规则 %s 至少需要一个条件", rule.ID)
	}

	re.mu.Lock()
	defer re.mu.Unlock()

	if _, exists := re.rules[rule.ID]; exists {
		return fmt.Errorf("规则 %s 已存在", rule.ID)
	}
	re.rules[rule.ID] = rule
	return nil
}

// UpdateRule 更新规则.
func (re *RuleEngine) UpdateRule(rule *Rule) error {
	if rule.ID == "" {
		return fmt.Errorf("规则 ID 不能为空")
	}

	re.mu.Lock()
	defer re.mu.Unlock()

	if _, exists := re.rules[rule.ID]; !exists {
		return fmt.Errorf("规则 %s 不存在", rule.ID)
	}
	re.rules[rule.ID] = rule
	return nil
}

// DeleteRule 删除规则.
func (re *RuleEngine) DeleteRule(ruleID string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	if _, exists := re.rules[ruleID]; !exists {
		return fmt.Errorf("规则 %s 不存在", ruleID)
	}
	delete(re.rules, ruleID)
	return nil
}

// GetRule 获取指定规则.
func (re *RuleEngine) GetRule(ruleID string) *Rule {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.rules[ruleID]
}

// GetAllRules 获取所有规则.
func (re *RuleEngine) GetAllRules() []*Rule {
	re.mu.RLock()
	defer re.mu.RUnlock()
	rules := make([]*Rule, 0, len(re.rules))
	for _, r := range re.rules {
		rules = append(rules, r)
	}
	return rules
}

// Evaluate 使用最新指标值评估所有启用的规则
// metrics: 当前各指标最新值
// 返回所有匹配结果.
func (re *RuleEngine) Evaluate(metrics map[MetricType]float64) []RuleMatchResult {
	re.mu.RLock()
	defer re.mu.RUnlock()

	var results []RuleMatchResult

	for _, rule := range re.rules {
		if !rule.Enabled {
			continue
		}

		match := re.evaluateRule(rule, metrics)
		if match != nil {
			results = append(results, *match)
		}
	}
	return results
}

// evaluateRule 评估单个规则（内部方法，调用者需持读锁）.
func (re *RuleEngine) evaluateRule(rule *Rule, metrics map[MetricType]float64) *RuleMatchResult {
	if len(rule.Conditions) == 0 {
		return nil
	}

	conditionResults := make([]bool, len(rule.Conditions))
	for i, cond := range rule.Conditions {
		value, ok := metrics[cond.MetricType]
		if !ok {
			conditionResults[i] = false
			continue
		}
		conditionResults[i] = evaluateCondition(value, cond.Operator, cond.Threshold)
	}

	// 根据逻辑运算符组合条件结果
	var triggered bool
	switch rule.Logic {
	case LogicalAND:
		triggered = true
		for _, r := range conditionResults {
			if !r {
				triggered = false
				break
			}
		}
	case LogicalOR:
		triggered = false
		for _, r := range conditionResults {
			if r {
				triggered = true
				break
			}
		}
	default:
		triggered = conditionResults[0]
	}

	if !triggered {
		return nil
	}

	// 构建消息
	values := make([]interface{}, len(rule.Conditions))
	for i, cond := range rule.Conditions {
		values[i] = metrics[cond.MetricType]
	}
	msg := rule.Message
	if len(values) > 0 {
		msg = fmt.Sprintf(rule.Message, values...)
	}

	return &RuleMatchResult{
		Rule:      rule,
		Triggered: true,
		Message:   msg,
	}
}

// evaluateCondition 评估单个比较条件.
func evaluateCondition(value float64, op ComparisonOp, threshold float64) bool {
	switch op {
	case OpGT:
		return value > threshold
	case OpGTE:
		return value >= threshold
	case OpLT:
		return value < threshold
	case OpLTE:
		return value <= threshold
	case OpEQ:
		return value == threshold
	case OpNEQ:
		return value != threshold
	default:
		return false
	}
}
