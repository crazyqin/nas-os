// Package nvmeof ACL访问控制
// 实现NVMe-oF安全增强，支持ACL和认证机制
package nvmeof

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"sync"
	"time"
)

// ========== ACL 规则定义 ==========

// ACLRule ACL规则
type ACLRule struct {
	ID          string       `json:"id"`           // 规则ID
	Name        string       `json:"name"`         // 规则名称
	Priority    int          `json:"priority"`     // 优先级（数值越小优先级越高）
	Action      ACLAction    `json:"action"`       // 动作（允许/拒绝）
	Subjects    []ACLSubject `json:"subjects"`     // 主体（主机/用户）
	Resources   []ACLResource `json:"resources"`   // 资源（子系统/命名空间）
	Operations  []ACLOperation `json:"operations"` // 操作类型
	Conditions  []ACLCondition `json:"conditions"`  // 条件
	Enabled     bool         `json:"enabled"`      // 是否启用
	CreatedAt   time.Time    `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time    `json:"updated_at"`   // 更新时间
	Description string       `json:"description"`  // 描述
	mu          sync.RWMutex `json:"-"`            // 保护内部状态
}

// ACLAction ACL动作
type ACLAction string

const (
	ACLAllow ACLAction = "allow"
	ACLDeny  ACLAction = "deny"
	ACLLog   ACLAction = "log"  // 仅记录，不拦截
)

// ACLSubject ACL主体
type ACLSubject struct {
	Type     SubjectType `json:"type"`     // 主体类型
	Value    string      `json:"value"`    // 值
	Match    MatchType   `json:"match"`    // 匹配方式
	NQN      string      `json:"nqn"`      // NQN标识（主机类型）
}

// SubjectType 主体类型
type SubjectType string

const (
	SubjectHost    SubjectType = "host"     // NVMe主机
	SubjectIP      SubjectType = "ip"       // IP地址
	SubjectIPNet   SubjectType = "ipnet"    // IP网段
	SubjectUser    SubjectType = "user"     // 用户
	SubjectGroup   SubjectType = "group"    // 用户组
	SubjectAny     SubjectType = "any"      // 任意主体
)

// MatchType 匹配类型
type MatchType string

const (
	MatchExact  MatchType = "exact"  // 精确匹配
	MatchPrefix MatchType = "prefix" // 前缀匹配
	MatchRegex  MatchType = "regex"  // 正则匹配
	MatchCIDR   MatchType = "cidr"   // CIDR匹配（IP网段）
)

// ACLResource ACL资源
type ACLResource struct {
	Type       ResourceType `json:"type"`        // 资源类型
	Subsystem  string       `json:"subsystem"`   // 子系统名称
	Namespace  uint32       `json:"namespace"`   // 命名空间ID
	Path       string       `json:"path"`        // 资源路径
}

// ResourceType 资源类型
type ResourceType string

const (
	ResourceSubsystem  ResourceType = "subsystem"  // NVMe子系统
	ResourceNamespace  ResourceType = "namespace"  // 命名空间
	ResourceListener   ResourceType = "listener"   // 监听器
	ResourceAll        ResourceType = "all"        // 所有资源
)

// ACLOperation ACL操作
type ACLOperation string

const (
	OpConnect    ACLOperation = "connect"    // 连接
	OpDisconnect ACLOperation = "disconnect" // 断开
	OpRead       ACLOperation = "read"       // 读
	OpWrite      ACLOperation = "write"      // 写
	OpAdmin      ACLOperation = "admin"      // 管理操作
	OpAll        ACLOperation = "all"        // 所有操作
)

// ACLCondition ACL条件
type ACLCondition struct {
	Type      ConditionType `json:"type"`       // 条件类型
	Value     string        `json:"value"`      // 条件值
	Operator  ConditionOp   `json:"operator"`  // 条件操作符
}

// ConditionType 条件类型
type ConditionType string

const (
	CondTimeRange   ConditionType = "time_range"   // 时间范围
	CondRateLimit   ConditionType = "rate_limit"  // 速率限制
	CondQuota       ConditionType = "quota"       // 配额限制
	CondAuthMethod  ConditionType = "auth_method" // 认证方法
)

// ConditionOp 条件操作符
type ConditionOp string

const (
	CondOpEQ   ConditionOp = "eq"   // 等于
	CondOpNE   ConditionOp = "ne"   // 不等于
	CondOpLT   ConditionOp = "lt"   // 小于
	CondOpGT   ConditionOp = "gt"   // 大于
	CondOpIn   ConditionOp = "in"   // 在范围内
	CondOpNotIn ConditionOp = "notin" // 不在范围内
)

// ========== ACL 管理器 ==========

// ACLManager ACL管理器
type ACLManager struct {
	rules     map[string]*ACLRule    // 规则映射
	ruleIndex []*ACLRule             // 按优先级排序的规则索引
	config    *ACLConfig             // ACL配置
	auditLog  *AuditLogger           // 审计日志
	ctx       context.Context        // 上下文
	cancel    context.CancelFunc     // 取消函数
	mu        sync.RWMutex           // 保护状态
	logger    Logger                 // 日志接口
}

// ACLConfig ACL配置
type ACLConfig struct {
	DefaultAction     ACLAction    `json:"default_action"`      // 默认动作
	EnableAudit       bool         `json:"enable_audit"`        // 启用审计
	AuditLogPath      string       `json:"audit_log_path"`      // 审计日志路径
	MaxRules          int          `json:"max_rules"`           // 最大规则数
	RuleCheckInterval time.Duration `json:"rule_check_interval"` // 规则检查间隔
	EnableRateLimit   bool         `json:"enable_rate_limit"`   // 启用速率限制
	DefaultRateLimit  uint64       `json:"default_rate_limit"`  // 默认速率限制
}

// DefaultACLConfig 默认ACL配置
func DefaultACLConfig() *ACLConfig {
	return &ACLConfig{
		DefaultAction:     ACLDeny,               // 默认拒绝
		EnableAudit:       true,
		AuditLogPath:      "/var/log/nas-os/nvmeof-acl.log",
		MaxRules:          1000,
		RuleCheckInterval: 1 * time.Minute,
		EnableRateLimit:   true,
		DefaultRateLimit:  10000, // 10K IOPS
	}
}

// ========== 错误定义 ==========

var (
	ErrACLRuleNotFound    = errors.New("acl rule not found")
	ErrACLRuleExists      = errors.New("acl rule already exists")
	ErrACLRuleDisabled    = errors.New("acl rule is disabled")
	ErrACLAccessDenied    = errors.New("nvme-of access denied by acl")
	ErrACLInvalidRule     = errors.New("invalid acl rule")
	ErrACLMaxRulesExceeded = errors.New("max acl rules exceeded")
	ErrACLConditionFailed = errors.New("acl condition check failed")
)

// ========== ACL 管理器方法 ==========

// NewACLManager 创建ACL管理器
func NewACLManager(config *ACLConfig, logger Logger) *ACLManager {
	if config == nil {
		config = DefaultACLConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ACLManager{
		rules:    make(map[string]*ACLRule),
		ruleIndex: make([]*ACLRule, 0),
		config:   config,
		auditLog: NewAuditLogger(config.AuditLogPath),
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
	}
}

// Start 启动ACL管理器
func (m *ACLManager) Start() error {
	m.logger.Infof("ACL manager started")
	return nil
}

// Stop 停止ACL管理器
func (m *ACLManager) Stop() {
	m.cancel()
	m.logger.Infof("ACL manager stopped")
}

// AddRule 添加ACL规则
func (m *ACLManager) AddRule(rule *ACLRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if len(m.rules) >= m.config.MaxRules {
		return ErrACLMaxRulesExceeded
	}
	
	if _, exists := m.rules[rule.ID]; exists {
		return ErrACLRuleExists
	}
	
	// 验证规则
	if err := m.validateRule(rule); err != nil {
		return err
	}
	
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	rule.Enabled = true
	
	m.rules[rule.ID] = rule
	
	// 更新规则索引
	m.rebuildIndex()
	
	m.logger.Infof("ACL rule added: id=%s, name=%s, action=%s", 
		rule.ID, rule.Name, rule.Action)
	return nil
}

// RemoveRule 移除ACL规则
func (m *ACLManager) RemoveRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.rules[ruleID]; !exists {
		return ErrACLRuleNotFound
	}
	
	delete(m.rules, ruleID)
	m.rebuildIndex()
	
	m.logger.Infof("ACL rule removed: id=%s", ruleID)
	return nil
}

// EnableRule 启用规则
func (m *ACLManager) EnableRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	rule, exists := m.rules[ruleID]
	if !exists {
		return ErrACLRuleNotFound
	}
	
	rule.mu.Lock()
	rule.Enabled = true
	rule.UpdatedAt = time.Now()
	rule.mu.Unlock()
	
	m.logger.Infof("ACL rule enabled: id=%s", ruleID)
	return nil
}

// DisableRule 禁用规则
func (m *ACLManager) DisableRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	rule, exists := m.rules[ruleID]
	if !exists {
		return ErrACLRuleNotFound
	}
	
	rule.mu.Lock()
	rule.Enabled = false
	rule.UpdatedAt = time.Now()
	rule.mu.Unlock()
	
	m.logger.Infof("ACL rule disabled: id=%s", ruleID)
	return nil
}

// CheckAccess 检查访问权限
func (m *ACLManager) CheckAccess(subject *ACLSubject, resource *ACLResource, operation ACLOperation) (ACLAction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// 按优先级检查规则
	for _, rule := range m.ruleIndex {
		rule.mu.RLock()
		if !rule.Enabled {
			rule.mu.RUnlock()
			continue
		}
		
		// 检查主体匹配
		if !m.matchSubject(rule.Subjects, subject) {
			rule.mu.RUnlock()
			continue
		}
		
		// 检查资源匹配
		if !m.matchResource(rule.Resources, resource) {
			rule.mu.RUnlock()
			continue
		}
		
		// 检查操作匹配
		if !m.matchOperation(rule.Operations, operation) {
			rule.mu.RUnlock()
			continue
		}
		
		// 检查条件
		condResult, err := m.checkConditions(rule.Conditions, subject, resource, operation)
		if err != nil {
			rule.mu.RUnlock()
			continue
		}
		if !condResult {
			rule.mu.RUnlock()
			continue
		}
		
		// 记录审计日志
		if m.config.EnableAudit {
			m.auditLog.Log(subject, resource, operation, rule.Action, rule.ID)
		}
		
		action := rule.Action
		rule.mu.RUnlock()
		
		return action, nil
	}
	
	// 无匹配规则，返回默认动作
	if m.config.EnableAudit {
		m.auditLog.Log(subject, resource, operation, m.config.DefaultAction, "default")
	}
	
	return m.config.DefaultAction, nil
}

// ========== 匹配函数 ==========

func (m *ACLManager) matchSubject(subjects []ACLSubject, target *ACLSubject) bool {
	for _, s := range subjects {
		if s.Type == SubjectAny {
			return true
		}
		
		if s.Type != target.Type {
			continue
		}
		
		switch s.Match {
		case MatchExact:
			if s.Value == target.Value {
				return true
			}
		case MatchPrefix:
			if len(target.Value) >= len(s.Value) && target.Value[:len(s.Value)] == s.Value {
				return true
			}
		case MatchCIDR:
			// IP网段匹配
			if s.Type == SubjectIPNet || s.Type == SubjectIP {
				_, ipNet, err := net.ParseCIDR(s.Value)
				if err == nil {
					ip := net.ParseIP(target.Value)
					if ip != nil && ipNet.Contains(ip) {
						return true
					}
				}
			}
		}
	}
	return false
}

func (m *ACLManager) matchResource(resources []ACLResource, target *ACLResource) bool {
	for _, r := range resources {
		if r.Type == ResourceAll {
			return true
		}
		
		if r.Type != target.Type {
			continue
		}
		
		if r.Type == ResourceSubsystem && r.Subsystem == target.Subsystem {
			return true
		}
		
		if r.Type == ResourceNamespace {
			if r.Subsystem == target.Subsystem && r.Namespace == target.Namespace {
				return true
			}
		}
	}
	return false
}

func (m *ACLManager) matchOperation(operations []ACLOperation, target ACLOperation) bool {
	for _, op := range operations {
		if op == OpAll {
			return true
		}
		if op == target {
			return true
		}
	}
	return false
}

func (m *ACLManager) checkConditions(conditions []ACLCondition, subject *ACLSubject, resource *ACLResource, operation ACLOperation) (bool, error) {
	for _, cond := range conditions {
		switch cond.Type {
		case CondTimeRange:
			// 时间范围检查
			// 解析条件值，例如 "09:00-18:00"
			// 简化实现：假设条件值是允许的时间范围
			if cond.Value != "" {
				// 这里需要实现时间范围解析
				// 简化：返回true
				return true, nil
			}
			
		case CondRateLimit:
			// 速率限制检查
			// 需要结合速率限制器实现
			return true, nil
			
		case CondQuota:
			// 配额检查
			return true, nil
			
		case CondAuthMethod:
			// 认证方法检查
			return true, nil
		}
	}
	return true, nil
}

// ========== 规则验证 ==========

func (m *ACLManager) validateRule(rule *ACLRule) error {
	if rule.ID == "" {
		return ErrACLInvalidRule
	}
	
	if rule.Action != ACLAllow && rule.Action != ACLDeny && rule.Action != ACLLog {
		return ErrACLInvalidRule
	}
	
	if len(rule.Subjects) == 0 {
		return ErrACLInvalidRule
	}
	
	if len(rule.Resources) == 0 {
		return ErrACLInvalidRule
	}
	
	return nil
}

// ========== 索引重建 ==========

func (m *ACLManager) rebuildIndex() {
	// 按优先级排序
	m.ruleIndex = make([]*ACLRule, 0, len(m.rules))
	for _, rule := range m.rules {
		m.ruleIndex = append(m.ruleIndex, rule)
	}
	
	// 按优先级排序（数值越小优先级越高）
	for i := 0; i < len(m.ruleIndex)-1; i++ {
		for j := i + 1; j < len(m.ruleIndex); j++ {
			if m.ruleIndex[i].Priority > m.ruleIndex[j].Priority {
				m.ruleIndex[i], m.ruleIndex[j] = m.ruleIndex[j], m.ruleIndex[i]
			}
		}
	}
}

// ========== 审计日志 ==========

// AuditLogger 审计日志记录器
type AuditLogger struct {
	path    string
	entries []*AuditEntry
	mu      sync.RWMutex
}

// AuditEntry 审计条目
type AuditEntry struct {
	Timestamp   time.Time     `json:"timestamp"`
	Subject     *ACLSubject   `json:"subject"`
	Resource    *ACLResource  `json:"resource"`
	Operation   ACLOperation  `json:"operation"`
	Action      ACLAction     `json:"action"`
	RuleID      string        `json:"rule_id"`
	Result      string        `json:"result"`     // success/denied
	ClientIP    string        `json:"client_ip"`
	SessionID   string        `json:"session_id"`
}

// NewAuditLogger 创建审计日志记录器
func NewAuditLogger(path string) *AuditLogger {
	return &AuditLogger{
		path:    path,
		entries: make([]*AuditEntry, 0),
	}
}

// Log 记录审计日志
func (l *AuditLogger) Log(subject *ACLSubject, resource *ACLResource, operation ACLOperation, action ACLAction, ruleID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	entry := &AuditEntry{
		Timestamp: time.Now(),
		Subject:   subject,
		Resource:  resource,
		Operation: operation,
		Action:    action,
		RuleID:    ruleID,
		Result:    string(action),
	}
	
	l.entries = append(l.entries, entry)
}

// GetEntries 获取审计日志
func (l *AuditLogger) GetEntries(limit int) []*AuditEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if limit <= 0 || limit > len(l.entries) {
		return l.entries
	}
	
	return l.entries[len(l.entries)-limit:]
}

// ========== 认证支持 ==========

// AuthMethod 认证方法
type AuthMethod string

const (
	AuthNone    AuthMethod = "none"     // 无认证
	AuthPSK     AuthMethod = "psk"      // 预共享密钥
	AuthDHCHAP  AuthMethod = "dhchap"   // DH-HMAC-CHAP
	AuthTLS     AuthMethod = "tls"      // TLS证书
)

// AuthConfig 认证配置
type AuthConfig struct {
	Method          AuthMethod `json:"method"`           // 认证方法
	PSK             string     `json:"psk"`              // 预共享密钥
	PSKHash         string     `json:"psk_hash"`         // PSK哈希
	DHCHAPKey       string     `json:"dhchap_key"`       // DH-CHAP密钥
	DHCHAPGroup     string     `json:"dhchap_group"`     // DH-CHAP组
	TLSCertPath     string     `json:"tls_cert_path"`    // TLS证书路径
	TLSKeyPath      string     `json:"tls_key_path"`     // TLS私钥路径
	TLSCACertPath   string     `json:"tls_ca_cert_path"` // CA证书路径
}

// ComputePSKHash 计算PSK哈希
func ComputePSKHash(psk string) string {
	hash := sha256.Sum256([]byte(psk))
	return hex.EncodeToString(hash[:])
}

// ========== 统计信息 ==========

// GetStats 获取ACL统计
func (m *ACLManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	ruleStats := make([]map[string]interface{}, 0)
	for _, rule := range m.rules {
		rule.mu.RLock()
		ruleStats = append(ruleStats, map[string]interface{}{
			"id":       rule.ID,
			"name":     rule.Name,
			"action":   rule.Action,
			"priority": rule.Priority,
			"enabled":  rule.Enabled,
		})
		rule.mu.RUnlock()
	}
	
	return map[string]interface{}{
		"total_rules":   len(m.rules),
		"default_action": m.config.DefaultAction,
		"audit_enabled":  m.config.EnableAudit,
		"rules":          ruleStats,
	}
}