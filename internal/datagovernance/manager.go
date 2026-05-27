package datagovernance

import (
	"fmt"
	"sync"
	"time"
)

// DataGovernanceManager 数据治理管理器
type DataGovernanceManager struct {
	mu        sync.RWMutex
	policies  map[string]*Policy
	classifications map[string]*Classification
	retentions map[string]*RetentionRule
	audits    []*AuditRecord
	config    *GovernanceConfig
}

// GovernanceConfig 治理配置
type GovernanceConfig struct {
	AutoClassify  bool `json:"auto_classify"`
	RetentionDays int  `json:"default_retention_days"`
	AuditEnabled  bool `json:"audit_enabled"`
}

// Policy 数据治理策略
type Policy struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        PolicyType `json:"type"`
	Rules       []Rule    `json:"rules"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PolicyType 策略类型
type PolicyType string

const (
	PolicyTypeRetention    PolicyType = "retention"
	PolicyTypeClassification PolicyType = "classification"
	PolicyTypeAccess       PolicyType = "access"
	PolicyTypeEncryption   PolicyType = "encryption"
)

// Rule 规则
type Rule struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Condition string     `json:"condition"`
	Action   string      `json:"action"`
	Value    interface{} `json:"value"`
}

// Classification 数据分类
type Classification struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Level     int       `json:"level"`
	Color     string    `json:"color"`
	Count     int       `json:"count"`
	CreatedAt time.Time `json:"created_at"`
}

// RetentionRule 保留规则
type RetentionRule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Days      int       `json:"days"`
	Action    string    `json:"action"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditRecord 审计记录
type AuditRecord struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	User      string    `json:"user"`
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
}

// NewDataGovernanceManager 创建管理器
func NewDataGovernanceManager(config *GovernanceConfig) *DataGovernanceManager {
	if config == nil {
		config = &GovernanceConfig{
			AutoClassify:  true,
			RetentionDays: 365,
			AuditEnabled:  true,
		}
	}
	return &DataGovernanceManager{
		policies:        make(map[string]*Policy),
		classifications: make(map[string]*Classification),
		retentions:      make(map[string]*RetentionRule),
		audits:          make([]*AuditRecord, 0),
		config:          config,
	}
}

// CreatePolicy 创建策略
func (m *DataGovernanceManager) CreatePolicy(policy *Policy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.Name == "" {
		return fmt.Errorf("策略名称不能为空")
	}

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("policy_%d", time.Now().UnixNano())
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy

	m.addAudit("create_policy", policy.ID, "system", fmt.Sprintf("创建策略: %s", policy.Name))
	return nil
}

// GetPolicy 获取策略
func (m *DataGovernanceManager) GetPolicy(id string) (*Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, exists := m.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略不存在: %s", id)
	}
	return policy, nil
}

// ListPolicies 列出所有策略
func (m *DataGovernanceManager) ListPolicies() []*Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*Policy, 0, len(m.policies))
	for _, policy := range m.policies {
		policies = append(policies, policy)
	}
	return policies
}

// UpdatePolicy 更新策略
func (m *DataGovernanceManager) UpdatePolicy(id string, update *Policy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[id]
	if !exists {
		return fmt.Errorf("策略不存在: %s", id)
	}

	if update.Name != "" {
		policy.Name = update.Name
	}
	if update.Description != "" {
		policy.Description = update.Description
	}
	if update.Rules != nil {
		policy.Rules = update.Rules
	}
	policy.UpdatedAt = time.Now()

	m.addAudit("update_policy", id, "system", fmt.Sprintf("更新策略: %s", policy.Name))
	return nil
}

// DeletePolicy 删除策略
func (m *DataGovernanceManager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return fmt.Errorf("策略不存在: %s", id)
	}

	delete(m.policies, id)
	m.addAudit("delete_policy", id, "system", "删除策略")
	return nil
}

// CreateClassification 创建分类
func (m *DataGovernanceManager) CreateClassification(class *Classification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if class.Name == "" {
		return fmt.Errorf("分类名称不能为空")
	}

	if class.ID == "" {
		class.ID = fmt.Sprintf("class_%d", time.Now().UnixNano())
	}

	class.CreatedAt = time.Now()
	m.classifications[class.ID] = class
	return nil
}

// ListClassifications 列出所有分类
func (m *DataGovernanceManager) ListClassifications() []*Classification {
	m.mu.RLock()
	defer m.mu.RUnlock()

	classes := make([]*Classification, 0, len(m.classifications))
	for _, class := range m.classifications {
		classes = append(classes, class)
	}
	return classes
}

// CreateRetentionRule 创建保留规则
func (m *DataGovernanceManager) CreateRetentionRule(rule *RetentionRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("retention_%d", time.Now().UnixNano())
	}

	rule.CreatedAt = time.Now()
	m.retentions[rule.ID] = rule
	return nil
}

// ListRetentionRules 列出所有保留规则
func (m *DataGovernanceManager) ListRetentionRules() []*RetentionRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*RetentionRule, 0, len(m.retentions))
	for _, rule := range m.retentions {
		rules = append(rules, rule)
	}
	return rules
}

// GetAuditRecords 获取审计记录
func (m *DataGovernanceManager) GetAuditRecords(limit int) []*AuditRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.audits) {
		limit = len(m.audits)
	}

	// 返回最新的记录
	start := len(m.audits) - limit
	if start < 0 {
		start = 0
	}
	return m.audits[start:]
}

// GetStats 获取统计信息
func (m *DataGovernanceManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_policies":        len(m.policies),
		"total_classifications": len(m.classifications),
		"total_retentions":      len(m.retentions),
		"total_audits":          len(m.audits),
	}
}

// addAudit 添加审计记录
func (m *DataGovernanceManager) addAudit(action, resource, user, details string) {
	if !m.config.AuditEnabled {
		return
	}

	record := &AuditRecord{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		Action:    action,
		Resource:  resource,
		User:      user,
		Details:   details,
		Timestamp: time.Now(),
	}

	m.audits = append(m.audits, record)
}
