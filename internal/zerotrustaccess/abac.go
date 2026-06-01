// Package zerotrustaccess 提供ABAC（基于属性的访问控制）支持
package zerotrustaccess

import (
	"fmt"
	"strings"
	"time"
)

// ========== ABAC相关类型 ==========

// AttributeType 属性类型
type AttributeType string

const (
	AttrTypeSubject  AttributeType = "subject"  // 主体属性
	AttrTypeResource AttributeType = "resource" // 资源属性
	AttrTypeAction   AttributeType = "action"   // 动作属性
	AttrTypeContext  AttributeType = "context"  // 环境属性
)

// Attribute 属性定义
type Attribute struct {
	Type  AttributeType `json:"type"`
	Name  string        `json:"name"`
	Value string        `json:"value"`
}

// ABACPolicy ABAC策略
type ABACPolicy struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Enabled     bool         `json:"enabled"`
	Priority    int          `json:"priority"`
	Target      *PolicyTarget `json:"target"`
	Rules       []Rule       `json:"rules"`
	Effect      PolicyEffect `json:"effect"`
	Obligations []Obligation `json:"obligations"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// PolicyTarget 策略目标
type PolicyTarget struct {
	Subjects  []AttributeMatch `json:"subjects,omitempty"`
	Resources []AttributeMatch `json:"resources,omitempty"`
	Actions   []AttributeMatch `json:"actions,omitempty"`
	Contexts  []AttributeMatch `json:"contexts,omitempty"`
}

// AttributeMatch 属性匹配
type AttributeMatch struct {
	Name     string `json:"name"`
	Operator string `json:"operator"` // "eq", "neq", "contains", "startswith", "endswith", "regex"
	Value    string `json:"value"`
}

// Rule 策略规则
type Rule struct {
	ID         string        `json:"id"`
	Condition  string        `json:"condition"` // 表达式，如 "subject.trust_level >= 3 && resource.sensitivity <= 2"
	Effect     PolicyEffect  `json:"effect"`
	Target     *PolicyTarget `json:"target,omitempty"`
}

// PolicyEffect 策略效果
type PolicyEffect string

const (
	EffectPermit PolicyEffect = "permit"
	EffectDeny   PolicyEffect = "deny"
)

// Obligation 义务（附加条件）
type Obligation struct {
	Type     string            `json:"type"` // "mfa", "logging", "notification", "encryption"
	Params   map[string]string `json:"params"`
	Required bool              `json:"required"`
}

// ABACRequest ABAC请求
type ABACRequest struct {
	Subject  map[string]string `json:"subject"`
	Resource map[string]string `json:"resource"`
	Action   string            `json:"action"`
	Context  map[string]string `json:"context"`
}

// ABACDecision ABAC决策
type ABACDecision struct {
	Allowed     bool         `json:"allowed"`
	Effect      PolicyEffect `json:"effect"`
	Reason      string       `json:"reason"`
	Obligations []Obligation `json:"obligations"`
	PolicyID    string       `json:"policy_id"`
	EvaluatedAt time.Time    `json:"evaluated_at"`
}

// ========== ABAC评估引擎 ==========

// ABACEngine ABAC评估引擎
type ABACEngine struct {
	policies map[string]*ABACPolicy
}

// NewABACEngine 创建ABAC引擎
func NewABACEngine() *ABACEngine {
	return &ABACEngine{
		policies: make(map[string]*ABACPolicy),
	}
}

// AddPolicy 添加ABAC策略
func (e *ABACEngine) AddPolicy(policy *ABACPolicy) error {
	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	e.policies[policy.ID] = policy
	return nil
}

// RemovePolicy 删除策略
func (e *ABACEngine) RemovePolicy(policyID string) error {
	if _, exists := e.policies[policyID]; !exists {
		return fmt.Errorf("policy %s not found", policyID)
	}
	delete(e.policies, policyID)
	return nil
}

// Evaluate 评估ABAC请求
func (e *ABACEngine) Evaluate(request *ABACRequest) *ABACDecision {
	decision := &ABACDecision{
		EvaluatedAt: time.Now(),
	}

	// 收集所有匹配策略
	var matchedPolicies []*ABACPolicy

	for _, policy := range e.policies {
		if !policy.Enabled {
			continue
		}

		if e.matchTarget(policy.Target, request) {
			matchedPolicies = append(matchedPolicies, policy)
		}
	}

	// 按优先级排序（数字越小优先级越高）
	sortPolicies(matchedPolicies)

	// 评估规则
	for _, policy := range matchedPolicies {
		if e.evaluateRules(policy.Rules, request) {
			decision.Allowed = policy.Effect == EffectPermit
			decision.Effect = policy.Effect
			decision.Reason = fmt.Sprintf("Policy '%s' matched", policy.Name)
			decision.Obligations = policy.Obligations
			decision.PolicyID = policy.ID
			return decision
		}
	}

	// 默认拒绝
	decision.Allowed = false
	decision.Effect = EffectDeny
	decision.Reason = "No matching policy, default deny"
	return decision
}

// matchTarget 匹配策略目标
func (e *ABACEngine) matchTarget(target *PolicyTarget, request *ABACRequest) bool {
	if target == nil {
		return true
	}

	// 匹配主体
	if len(target.Subjects) > 0 {
		if !matchAttributes(target.Subjects, request.Subject) {
			return false
		}
	}

	// 匹配资源
	if len(target.Resources) > 0 {
		if !matchAttributes(target.Resources, request.Resource) {
			return false
		}
	}

	// 匹配动作
	if len(target.Actions) > 0 {
		actionMatch := false
		for _, match := range target.Actions {
			if matchOperator(match.Operator, request.Action, match.Value) {
				actionMatch = true
				break
			}
		}
		if !actionMatch {
			return false
		}
	}

	// 匹配环境
	if len(target.Contexts) > 0 {
		if !matchAttributes(target.Contexts, request.Context) {
			return false
		}
	}

	return true
}

// matchAttributes 匹配属性列表
func matchAttributes(matches []AttributeMatch, attrs map[string]string) bool {
	for _, match := range matches {
		value, exists := attrs[match.Name]
		if !exists {
			return false
		}
		if !matchOperator(match.Operator, value, match.Value) {
			return false
		}
	}
	return true
}

// matchOperator 匹配操作
func matchOperator(operator, value, expected string) bool {
	switch operator {
	case "eq":
		return value == expected
	case "neq":
		return value != expected
	case "contains":
		return strings.Contains(value, expected)
	case "startswith":
		return strings.HasPrefix(value, expected)
	case "endswith":
		return strings.HasSuffix(value, expected)
	case "regex":
		// 简化实现，实际应该使用正则
		return strings.Contains(value, expected)
	case "in":
		values := strings.Split(expected, ",")
		for _, v := range values {
			if strings.TrimSpace(v) == value {
				return true
			}
		}
		return false
	case "not_in":
		values := strings.Split(expected, ",")
		for _, v := range values {
			if strings.TrimSpace(v) == value {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// evaluateRules 评估规则
func (e *ABACEngine) evaluateRules(rules []Rule, request *ABACRequest) bool {
	if len(rules) == 0 {
		return true
	}

	for _, rule := range rules {
		if e.evaluateCondition(rule.Condition, request) {
			return rule.Effect == EffectPermit
		}
	}

	return false
}

// evaluateCondition 评估条件表达式（简化版本）
func (e *ABACEngine) evaluateCondition(condition string, request *ABACRequest) bool {
	// 解析条件：subject.trust_level >= 3 && resource.sensitivity <= 2
	// 简化实现，支持基本比较
	parts := strings.Split(condition, "&&")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !e.evaluateSingleCondition(part, request) {
			return false
		}
	}
	return true
}

// evaluateSingleCondition 评估单个条件
func (e *ABACEngine) evaluateSingleCondition(condition string, request *ABACRequest) bool {
	// 解析 "subject.trust_level >= 3"
	tokens := strings.Fields(condition)
	if len(tokens) != 3 {
		return false
	}

	attrPath := tokens[0]
	operator := tokens[1]
	expected := tokens[2]

	// 获取属性值
	parts := strings.SplitN(attrPath, ".", 2)
	if len(parts) != 2 {
		return false
	}

	attrType := parts[0]
	attrName := parts[1]

	var value string
	switch attrType {
	case "subject":
		value = request.Subject[attrName]
	case "resource":
		value = request.Resource[attrName]
	case "action":
		value = request.Action
	case "context":
		value = request.Context[attrName]
	default:
		return false
	}

	// 比较
	switch operator {
	case "==", "eq":
		return value == expected
	case "!=", "neq":
		return value != expected
	case ">=", "gte":
		return compareVersions(value, expected) >= 0
	case "<=", "lte":
		return compareVersions(value, expected) <= 0
	case ">", "gt":
		return compareVersions(value, expected) > 0
	case "<", "lt":
		return compareVersions(value, expected) < 0
	case "contains":
		return strings.Contains(value, expected)
	default:
		return false
	}
}

// compareVersions 比较版本或数字
func compareVersions(a, b string) int {
	// 尝试数字比较
	var numA, numB int
	if _, err := fmt.Sscanf(a, "%d", &numA); err == nil {
		if _, err := fmt.Sscanf(b, "%d", &numB); err == nil {
			if numA < numB {
				return -1
			} else if numA > numB {
				return 1
			}
			return 0
		}
	}

	// 字符串比较
	return strings.Compare(a, b)
}

// sortPolicies 按优先级排序策略
func sortPolicies(policies []*ABACPolicy) {
	// 简单冒泡排序
	for i := 0; i < len(policies)-1; i++ {
		for j := 0; j < len(policies)-i-1; j++ {
			if policies[j].Priority > policies[j+1].Priority {
				policies[j], policies[j+1] = policies[j+1], policies[j]
			}
		}
	}
}

// GetPolicies 获取所有ABAC策略
func (e *ABACEngine) GetPolicies() []*ABACPolicy {
	policies := make([]*ABACPolicy, 0, len(e.policies))
	for _, p := range e.policies {
		policies = append(policies, p)
	}
	return policies
}
