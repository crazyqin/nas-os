package notification

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// RuleEngine 规则引擎
type RuleEngine struct {
	rules     []*Rule
	ruleMap   map[string]*Rule
	rateCache map[string]*rateLimitEntry
	mu        sync.RWMutex
	storePath string
}

type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

// NewRuleEngine 创建规则引擎
func NewRuleEngine(storePath string) (*RuleEngine, error) {
	engine := &RuleEngine{
		rules:     make([]*Rule, 0),
		ruleMap:   make(map[string]*Rule),
		rateCache: make(map[string]*rateLimitEntry),
		storePath: storePath,
	}

	if err := engine.load(); err != nil {
		return nil, fmt.Errorf("加载规则失败: %w", err)
	}

	return engine, nil
}

// load 加载规则
func (e *RuleEngine) load() error {
	if e.storePath == "" {
		return nil
	}

	data, err := os.ReadFile(e.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var rules []*Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.rules = rules
	e.ruleMap = make(map[string]*Rule)
	for _, r := range rules {
		e.ruleMap[r.ID] = r
	}

	// 按优先级排序
	e.sortRules()

	return nil
}

// save 保存规则
func (e *RuleEngine) save() error {
	if e.storePath == "" {
		return nil
	}

	data, err := json.MarshalIndent(e.rules, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(e.storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(e.storePath, data, 0640)
}

// sortRules 按优先级排序规则
func (e *RuleEngine) sortRules() {
	// 简单冒泡排序，按优先级降序
	for i := 0; i < len(e.rules); i++ {
		for j := i + 1; j < len(e.rules); j++ {
			if e.rules[i].Priority < e.rules[j].Priority {
				e.rules[i], e.rules[j] = e.rules[j], e.rules[i]
			}
		}
	}
}

// CreateRule 创建规则
func (e *RuleEngine) CreateRule(rule *Rule) error {
	if rule.ID == "" {
		return fmt.Errorf("规则 ID 不能为空")
	}

	if rule.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.ruleMap[rule.ID]; exists {
		return fmt.Errorf("规则已存在: %s", rule.ID)
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	e.rules = append(e.rules, rule)
	e.ruleMap[rule.ID] = rule
	e.sortRules()

	return e.save()
}

// UpdateRule 更新规则
func (e *RuleEngine) UpdateRule(rule *Rule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, exists := e.ruleMap[rule.ID]
	if !exists {
		return fmt.Errorf("规则不存在: %s", rule.ID)
	}

	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	e.ruleMap[rule.ID] = rule

	// 更新列表中的规则
	for i, r := range e.rules {
		if r.ID == rule.ID {
			e.rules[i] = rule
			break
		}
	}
	e.sortRules()

	return e.save()
}

// DeleteRule 删除规则
func (e *RuleEngine) DeleteRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.ruleMap[id]; !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	delete(e.ruleMap, id)

	// 从列表中删除
	for i, r := range e.rules {
		if r.ID == id {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			break
		}
	}

	return e.save()
}

// GetRule 获取规则
func (e *RuleEngine) GetRule(id string) (*Rule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rule, exists := e.ruleMap[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}

	return rule, nil
}

// ListRules 列出规则
func (e *RuleEngine) ListRules() []*Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Rule, len(e.rules))
	copy(result, e.rules)
	return result
}

// MatchRules 匹配规则
func (e *RuleEngine) MatchRules(notification *Notification) []*Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	matched := make([]*Rule, 0)
	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		// 检查静默时段
		if e.isQuietHours(rule) {
			continue
		}

		// 检查频率限制
		if !e.checkRateLimit(rule, notification) {
			continue
		}

		// 评估条件
		if e.evaluateConditions(rule.Conditions, notification) {
			matched = append(matched, rule)
		}
	}

	return matched
}

// evaluateConditions 评估条件组
func (e *RuleEngine) evaluateConditions(group RuleGroup, notification *Notification) bool {
	switch group.Operator {
	case OperatorAnd:
		// 所有条件都满足
		for _, condition := range group.Rules {
			if !e.evaluateCondition(condition, notification) {
				return false
			}
		}
		for _, subGroup := range group.Groups {
			if !e.evaluateConditions(subGroup, notification) {
				return false
			}
		}
		return true

	case OperatorOr:
		// 任一条件满足
		for _, condition := range group.Rules {
			if e.evaluateCondition(condition, notification) {
				return true
			}
		}
		for _, subGroup := range group.Groups {
			if e.evaluateConditions(subGroup, notification) {
				return true
			}
		}
		return false

	case OperatorNot:
		// 条件取反
		if len(group.Rules) > 0 {
			return !e.evaluateCondition(group.Rules[0], notification)
		}
		if len(group.Groups) > 0 {
			return !e.evaluateConditions(group.Groups[0], notification)
		}
		return false

	default:
		return false
	}
}

// evaluateCondition 评估单个条件
func (e *RuleEngine) evaluateCondition(condition RuleConditionItem, notification *Notification) bool {
	value := e.getFieldValue(notification, condition.Field)

	switch condition.Condition {
	case ConditionEquals:
		return e.compareEquals(value, condition.Value)

	case ConditionNotEquals:
		return !e.compareEquals(value, condition.Value)

	case ConditionContains:
		return e.checkContains(value, condition.Value)

	case ConditionNotContains:
		return !e.checkContains(value, condition.Value)

	case ConditionGreaterThan:
		result, _ := e.compareNumbers(value, condition.Value, ">")
		return result

	case ConditionLessThan:
		result, _ := e.compareNumbers(value, condition.Value, "<")
		return result

	case ConditionMatches:
		result, _ := e.checkRegex(value, condition.Value)
		return result

	case ConditionExists:
		return value != nil

	default:
		return false
	}
}

// getFieldValue 获取字段值
func (e *RuleEngine) getFieldValue(notification *Notification, field string) interface{} {
	parts := strings.Split(field, ".")

	var current interface{} = notification

	for _, part := range parts {
		switch v := current.(type) {
		case *Notification:
			switch part {
			case "title":
				current = v.Title
			case "message":
				current = v.Message
			case "level":
				current = string(v.Level)
			case "category":
				current = v.Category
			case "source":
				current = v.Source
			case "data":
				current = v.Data
			default:
				return nil
			}
		case map[string]interface{}:
			if val, ok := v[part]; ok {
				current = val
			} else {
				return nil
			}
		default:
			return nil
		}
	}

	return current
}

// compareEquals 比较相等
func (e *RuleEngine) compareEquals(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// checkContains 检查包含
func (e *RuleEngine) checkContains(a, b interface{}) bool {
	if a == nil {
		return false
	}

	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)

	return strings.Contains(aStr, bStr)
}

// compareNumbers 比较数字
func (e *RuleEngine) compareNumbers(a, b interface{}, op string) (bool, error) {
	aFloat, err := toFloat64Value(a)
	if err != nil {
		return false, err
	}

	bFloat, err := toFloat64Value(b)
	if err != nil {
		return false, err
	}

	switch op {
	case ">":
		return aFloat > bFloat, nil
	case "<":
		return aFloat < bFloat, nil
	default:
		return false, fmt.Errorf("未知比较运算符: %s", op)
	}
}

// checkRegex 正则匹配
func (e *RuleEngine) checkRegex(value, pattern interface{}) (bool, error) {
	if value == nil {
		return false, nil
	}

	valueStr := fmt.Sprintf("%v", value)
	patternStr := fmt.Sprintf("%v", pattern)

	matched, err := regexp.MatchString(patternStr, valueStr)
	if err != nil {
		return false, fmt.Errorf("无效的正则表达式: %w", err)
	}

	return matched, nil
}

// isQuietHours 检查是否在静默时段
func (e *RuleEngine) isQuietHours(rule *Rule) bool {
	if rule.QuietHours == nil {
		return false
	}

	now := time.Now()

	// 检查星期几
	if len(rule.QuietHours.Days) > 0 {
		weekday := int(now.Weekday())
		found := false
		for _, d := range rule.QuietHours.Days {
			if d == weekday {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 解析开始和结束时间
	startTime, err := time.Parse("15:04", rule.QuietHours.Start)
	if err != nil {
		return false
	}

	endTime, err := time.Parse("15:04", rule.QuietHours.End)
	if err != nil {
		return false
	}

	// 获取当前时间（只保留小时和分钟）
	currentTime := time.Date(0, 1, 1, now.Hour(), now.Minute(), 0, 0, time.UTC)
	startDateTime := time.Date(0, 1, 1, startTime.Hour(), startTime.Minute(), 0, 0, time.UTC)
	endDateTime := time.Date(0, 1, 1, endTime.Hour(), endTime.Minute(), 0, 0, time.UTC)

	// 处理跨午夜的情况
	if endDateTime.Before(startDateTime) {
		// 例如：23:00 - 06:00
		return currentTime.After(startDateTime) || currentTime.Before(endDateTime)
	}

	// 正常情况
	return currentTime.After(startDateTime) && currentTime.Before(endDateTime)
}

// checkRateLimit 检查频率限制
func (e *RuleEngine) checkRateLimit(rule *Rule, notification *Notification) bool {
	if rule.RateLimit == nil {
		return true
	}

	key := fmt.Sprintf("%s:%s", rule.ID, notification.Source)

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	entry, exists := e.rateCache[key]

	if !exists || now.After(entry.resetTime) {
		// 重置计数器
		e.rateCache[key] = &rateLimitEntry{
			count:     1,
			resetTime: now.Add(rule.RateLimit.Duration),
		}
		return true
	}

	if entry.count >= rule.RateLimit.Count {
		return false
	}

	entry.count++
	return true
}

// EnableRule 启用规则
func (e *RuleEngine) EnableRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	rule, exists := e.ruleMap[id]
	if !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	rule.Enabled = true
	rule.UpdatedAt = time.Now()

	return e.save()
}

// DisableRule 禁用规则
func (e *RuleEngine) DisableRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	rule, exists := e.ruleMap[id]
	if !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	rule.Enabled = false
	rule.UpdatedAt = time.Now()

	return e.save()
}

// TestRule 测试规则
func (e *RuleEngine) TestRule(rule *Rule, notification *Notification) *RuleTestResult {
	result := &RuleTestResult{
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		Matched:     false,
		Errors:      make([]string, 0),
		Evaluations: make([]ConditionEvaluation, 0),
	}

	// 检查静默时段
	if e.isQuietHours(rule) {
		result.Errors = append(result.Errors, "当前处于静默时段")
		return result
	}

	// 评估条件
	result.Matched = e.evaluateConditions(rule.Conditions, notification)

	return result
}

// RuleTestResult 规则测试结果
type RuleTestResult struct {
	RuleID      string                `json:"ruleId"`
	RuleName    string                `json:"ruleName"`
	Matched     bool                  `json:"matched"`
	Errors      []string              `json:"errors,omitempty"`
	Evaluations []ConditionEvaluation `json:"evaluations,omitempty"`
}

// ConditionEvaluation 条件评估结果
type ConditionEvaluation struct {
	Field     string        `json:"field"`
	Condition RuleCondition `json:"condition"`
	Value     interface{}   `json:"value"`
	Result    bool          `json:"result"`
}

// 辅助函数

func toFloat64Value(v interface{}) (float64, error) {
	switch val := v.(type) {
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case float32:
		return float64(val), nil
	case float64:
		return val, nil
	case string:
		var f float64
		_, err := fmt.Sscanf(val, "%f", &f)
		return f, err
	default:
		return 0, fmt.Errorf("无法将 %T 转换为 float64", v)
	}
}
