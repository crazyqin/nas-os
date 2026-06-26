// Package wormcomply 提供 WORM（Write Once Read Many）合规引擎，
// 支持数据保留策略管理（SOX/HIPAA/GDPR）、合规审计日志和策略执行。
// 对标群晖 WriteOnce 合规功能，提供更细粒度的法规级保留策略。
package wormcomply

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Regulation 法规类型
type Regulation string

const (
	RegSOX    Regulation = "SOX"    // Sarbanes-Oxley Act
	RegHIPAA  Regulation = "HIPAA"  // Health Insurance Portability and Accountability Act
	RegGDPR   Regulation = "GDPR"   // General Data Protection Regulation
	RegFINRA  Regulation = "FINRA"  // Financial Industry Regulatory Authority
	RegSEC17a Regulation = "SEC17a" // SEC Rule 17a-4
	RegCustom Regulation = "CUSTOM"
)

// RetentionAction 保留策略动作
type RetentionAction string

const (
	ActionRetain     RetentionAction = "retain"  // 保留，不可删除
	ActionDelete     RetentionAction = "delete"  // 到期后自动删除
	ActionArchive    RetentionAction = "archive" // 到期后归档
	ActionReview     RetentionAction = "review"  // 到期后人工审核
	ActionNotifyOnly RetentionAction = "notify"  // 仅通知，不执行动作
)

// PolicyState 策略状态
type PolicyState string

const (
	PolicyStateActive    PolicyState = "active"
	PolicyStateSuspended PolicyState = "suspended"
	PolicyStateExpired   PolicyState = "expired"
)

// ComplianceLevel 合规等级
type ComplianceLevel string

const (
	LevelStrict   ComplianceLevel = "strict"   // 严格模式，任何违规阻止操作
	LevelModerate ComplianceLevel = "moderate" // 中等模式，违规记录但允许
	LevelAdvisory ComplianceLevel = "advisory" // 建议模式，仅告警
)

// RetentionPolicy 数据保留策略
type RetentionPolicy struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	Regulation       Regulation      `json:"regulation"`
	RetentionDays    int             `json:"retention_days"`     // 保留天数（0 = 永久）
	MaxRetentionDays int             `json:"max_retention_days"` // 最大保留天数
	Action           RetentionAction `json:"action"`             // 到期动作
	Level            ComplianceLevel `json:"level"`              // 合规等级
	State            PolicyState     `json:"state"`
	SharePaths       []string        `json:"share_paths"`   // 适用的共享路径
	FilePatterns     []string        `json:"file_patterns"` // 文件匹配模式（glob）
	ExcludePaths     []string        `json:"exclude_paths"` // 排除路径
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	CreatedBy        string          `json:"created_by"`
	Version          int             `json:"version"`
}

// ComplianceAuditEntry 合规审计日志条目
type ComplianceAuditEntry struct {
	ID           string     `json:"id"`
	PolicyID     string     `json:"policy_id"`
	PolicyName   string     `json:"policy_name"`
	Regulation   Regulation `json:"regulation"`
	Action       string     `json:"action"` // create, update, delete, enforce, violate, expire
	FilePath     string     `json:"file_path,omitempty"`
	UserID       string     `json:"user_id"`
	UserName     string     `json:"user_name,omitempty"`
	Details      string     `json:"details"`
	Severity     string     `json:"severity"` // info, warning, critical
	Success      bool       `json:"success"`
	ErrorMessage string     `json:"error_msg,omitempty"`
	Timestamp    time.Time  `json:"timestamp"`
	ClientIP     string     `json:"client_ip,omitempty"`
}

// ComplianceViolation 合规违规记录
type ComplianceViolation struct {
	ID              string     `json:"id"`
	PolicyID        string     `json:"policy_id"`
	FilePath        string     `json:"file_path"`
	ViolationType   string     `json:"violation_type"` // delete_blocked, modify_blocked, retention_breach
	Description     string     `json:"description"`
	UserID          string     `json:"user_id"`
	AttemptedAction string     `json:"attempted_action"`
	Timestamp       time.Time  `json:"timestamp"`
	Resolved        bool       `json:"resolved"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy      string     `json:"resolved_by,omitempty"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID             string                 `json:"id"`
	GeneratedAt    time.Time              `json:"generated_at"`
	Period         string                 `json:"period"` // daily, weekly, monthly
	TotalPolicies  int                    `json:"total_policies"`
	ActivePolicies int                    `json:"active_policies"`
	TotalFiles     int64                  `json:"total_files"`
	ProtectedFiles int64                  `json:"protected_files"`
	Violations     int                    `json:"violations"`
	UnresolvedVios int                    `json:"unresolved_violations"`
	AuditEntries   int                    `json:"audit_entries"`
	ByRegulation   map[Regulation]int     `json:"by_regulation"`
	TopViolations  []*ComplianceViolation `json:"top_violations,omitempty"`
	Score          float64                `json:"score"` // 0-100
}

// PolicyViolationError 策略违规错误
type PolicyViolationError struct {
	PolicyID   string
	PolicyName string
	FilePath   string
	Reason     string
}

func (e *PolicyViolationError) Error() string {
	return fmt.Sprintf("policy violation: policy=%s file=%s reason=%s", e.PolicyName, e.FilePath, e.Reason)
}

// 预定义错误
var (
	ErrPolicyNotFound  = errors.New("retention policy not found")
	ErrPolicyExists    = errors.New("retention policy already exists")
	ErrPolicySuspended = errors.New("retention policy is suspended")
	ErrFileProtected   = errors.New("file is protected by retention policy")
	ErrInvalidPolicy   = errors.New("invalid policy configuration")
	ErrAuditLogFull    = errors.New("audit log is full")
)

// Engine WORM 合规引擎
type Engine struct {
	mu          sync.RWMutex
	policies    map[string]*RetentionPolicy
	auditLog    []ComplianceAuditEntry
	violations  map[string]*ComplianceViolation
	maxAuditLog int
	config      EngineConfig
}

// EngineConfig 引擎配置
type EngineConfig struct {
	MaxAuditEntries   int             `json:"max_audit_entries"`
	DefaultLevel      ComplianceLevel `json:"default_level"`
	AutoDeleteExpired bool            `json:"auto_delete_expired"`
	NotifyOnViolation bool            `json:"notify_on_violation"`
	ReportDir         string          `json:"report_dir"`
}

// DefaultEngineConfig 默认引擎配置
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxAuditEntries:   100000,
		DefaultLevel:      LevelStrict,
		AutoDeleteExpired: false,
		NotifyOnViolation: true,
		ReportDir:         "/var/reports/wormcomply",
	}
}

// NewEngine 创建 WORM 合规引擎
func NewEngine(config EngineConfig) *Engine {
	if config.MaxAuditEntries <= 0 {
		config = DefaultEngineConfig()
	}
	return &Engine{
		policies:    make(map[string]*RetentionPolicy),
		auditLog:    make([]ComplianceAuditEntry, 0, 1024),
		violations:  make(map[string]*ComplianceViolation),
		maxAuditLog: config.MaxAuditEntries,
		config:      config,
	}
}

// CreatePolicy 创建保留策略
func (e *Engine) CreatePolicy(p *RetentionPolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if p.ID == "" {
		return ErrInvalidPolicy
	}
	if _, exists := e.policies[p.ID]; exists {
		return ErrPolicyExists
	}
	if p.RetentionDays < 0 {
		return ErrInvalidPolicy
	}
	if p.Name == "" {
		return ErrInvalidPolicy
	}

	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	p.Version = 1
	if p.State == "" {
		p.State = PolicyStateActive
	}
	if p.Level == "" {
		p.Level = e.config.DefaultLevel
	}

	e.policies[p.ID] = p

	e.appendAudit(&ComplianceAuditEntry{
		PolicyID:   p.ID,
		PolicyName: p.Name,
		Regulation: p.Regulation,
		Action:     "create",
		UserID:     p.CreatedBy,
		Details:    fmt.Sprintf("Policy created with retention %d days, action=%s", p.RetentionDays, p.Action),
		Severity:   "info",
		Success:    true,
		Timestamp:  now,
	})

	return nil
}

// UpdatePolicy 更新保留策略
func (e *Engine) UpdatePolicy(policyID string, updated *RetentionPolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.policies[policyID]
	if !ok {
		return ErrPolicyNotFound
	}

	updated.ID = existing.ID
	updated.CreatedAt = existing.CreatedAt
	updated.CreatedBy = existing.CreatedBy
	updated.Version = existing.Version + 1
	updated.UpdatedAt = time.Now()

	e.policies[policyID] = updated

	e.appendAudit(&ComplianceAuditEntry{
		PolicyID:   policyID,
		PolicyName: updated.Name,
		Regulation: updated.Regulation,
		Action:     "update",
		UserID:     updated.CreatedBy,
		Details:    fmt.Sprintf("Policy updated to v%d", updated.Version),
		Severity:   "info",
		Success:    true,
		Timestamp:  time.Now(),
	})

	return nil
}

// DeletePolicy 删除保留策略
func (e *Engine) DeletePolicy(policyID string, userID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, ok := e.policies[policyID]
	if !ok {
		return ErrPolicyNotFound
	}

	delete(e.policies, policyID)

	e.appendAudit(&ComplianceAuditEntry{
		PolicyID:   policyID,
		PolicyName: p.Name,
		Regulation: p.Regulation,
		Action:     "delete",
		UserID:     userID,
		Details:    "Policy deleted",
		Severity:   "warning",
		Success:    true,
		Timestamp:  time.Now(),
	})

	return nil
}

// GetPolicy 获取策略
func (e *Engine) GetPolicy(policyID string) (*RetentionPolicy, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	p, ok := e.policies[policyID]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return p, nil
}

// ListPolicies 列出所有策略
func (e *Engine) ListPolicies() []*RetentionPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*RetentionPolicy, 0, len(e.policies))
	for _, p := range e.policies {
		result = append(result, p)
	}
	return result
}

// ListPoliciesByRegulation 按法规列出策略
func (e *Engine) ListPoliciesByRegulation(reg Regulation) []*RetentionPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*RetentionPolicy, 0)
	for _, p := range e.policies {
		if p.Regulation == reg {
			result = append(result, p)
		}
	}
	return result
}

// CheckFileAccess 检查文件操作是否合规
// operation: delete, modify, rename, read
func (e *Engine) CheckFileAccess(filePath string, operation string, userID string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, p := range e.policies {
		if p.State != PolicyStateActive {
			continue
		}
		if !e.matchesPolicy(filePath, p) {
			continue
		}

		// WORM 策略下，受保护文件不可删除或修改
		if operation == "delete" || operation == "modify" || operation == "rename" {
			// 违规记录由上层调用 RecordViolation 完成
			return &PolicyViolationError{
				PolicyID:   p.ID,
				PolicyName: p.Name,
				FilePath:   filePath,
				Reason:     fmt.Sprintf("File is protected by %s retention policy (%s)", p.Regulation, p.Name),
			}
		}
	}

	return nil
}

// RecordViolation 记录合规违规
func (e *Engine) RecordViolation(v *ComplianceViolation) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.violations[v.ID] = v

	e.appendAudit(&ComplianceAuditEntry{
		PolicyID:   v.PolicyID,
		Regulation: "",
		Action:     "violate",
		FilePath:   v.FilePath,
		UserID:     v.UserID,
		Details:    v.Description,
		Severity:   "critical",
		Success:    false,
		Timestamp:  time.Now(),
	})
}

// ResolveViolation 解决违规
func (e *Engine) ResolveViolation(violationID string, resolvedBy string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	v, ok := e.violations[violationID]
	if !ok {
		return errors.New("violation not found")
	}

	now := time.Now()
	v.Resolved = true
	v.ResolvedAt = &now
	v.ResolvedBy = resolvedBy

	e.appendAudit(&ComplianceAuditEntry{
		PolicyID:  v.PolicyID,
		Action:    "violation_resolved",
		FilePath:  v.FilePath,
		UserID:    resolvedBy,
		Details:   fmt.Sprintf("Violation %s resolved", violationID),
		Severity:  "info",
		Success:   true,
		Timestamp: now,
	})

	return nil
}

// GetViolations 获取未解决违规
func (e *Engine) GetViolations(unresolvedOnly bool) []*ComplianceViolation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*ComplianceViolation, 0, len(e.violations))
	for _, v := range e.violations {
		if unresolvedOnly && v.Resolved {
			continue
		}
		result = append(result, v)
	}
	return result
}

// GetAuditLog 获取审计日志
func (e *Engine) GetAuditLog(limit int, policyID string) []ComplianceAuditEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.auditLog) {
		limit = len(e.auditLog)
	}

	start := len(e.auditLog) - limit
	if start < 0 {
		start = 0
	}

	result := make([]ComplianceAuditEntry, 0, limit)
	for i := len(e.auditLog) - 1; i >= start; i-- {
		if policyID != "" && e.auditLog[i].PolicyID != policyID {
			continue
		}
		result = append(result, e.auditLog[i])
	}
	return result
}

// GenerateReport 生成合规报告
func (e *Engine) GenerateReport(period string) *ComplianceReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	report := &ComplianceReport{
		ID:           fmt.Sprintf("report-%d", time.Now().UnixNano()),
		GeneratedAt:  time.Now(),
		Period:       period,
		ByRegulation: make(map[Regulation]int),
	}

	report.TotalPolicies = len(e.policies)
	for _, p := range e.policies {
		if p.State == PolicyStateActive {
			report.ActivePolicies++
		}
		report.ByRegulation[p.Regulation]++
	}

	report.AuditEntries = len(e.auditLog)
	report.Violations = len(e.violations)
	for _, v := range e.violations {
		if !v.Resolved {
			report.UnresolvedVios++
		}
	}

	// 计算合规分数
	if report.Violations > 0 {
		report.Score = float64(report.Violations-report.UnresolvedVios) / float64(report.Violations) * 100
	} else {
		report.Score = 100
	}

	// Top 10 违规
	violations := e.GetViolations(true)
	if len(violations) > 10 {
		report.TopViolations = violations[:10]
	} else {
		report.TopViolations = violations
	}

	return report
}

// SuspendPolicy 暂停策略
func (e *Engine) SuspendPolicy(policyID string, userID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, ok := e.policies[policyID]
	if !ok {
		return ErrPolicyNotFound
	}

	p.State = PolicyStateSuspended
	p.UpdatedAt = time.Now()

	e.appendAudit(&ComplianceAuditEntry{
		PolicyID:   policyID,
		PolicyName: p.Name,
		Regulation: p.Regulation,
		Action:     "suspend",
		UserID:     userID,
		Details:    "Policy suspended",
		Severity:   "warning",
		Success:    true,
		Timestamp:  time.Now(),
	})

	return nil
}

// ActivatePolicy 激活策略
func (e *Engine) ActivatePolicy(policyID string, userID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, ok := e.policies[policyID]
	if !ok {
		return ErrPolicyNotFound
	}

	p.State = PolicyStateActive
	p.UpdatedAt = time.Now()

	e.appendAudit(&ComplianceAuditEntry{
		PolicyID:   policyID,
		PolicyName: p.Name,
		Regulation: p.Regulation,
		Action:     "activate",
		UserID:     userID,
		Details:    "Policy activated",
		Severity:   "info",
		Success:    true,
		Timestamp:  time.Now(),
	})

	return nil
}

// GetExpiredPolicies 获取已过期的策略（保留期已过但未处理）
func (e *Engine) GetExpiredPolicies() []*RetentionPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now()
	result := make([]*RetentionPolicy, 0)
	for _, p := range e.policies {
		if p.RetentionDays > 0 && p.UpdatedAt.Add(time.Duration(p.RetentionDays)*24*time.Hour).Before(now) {
			result = append(result, p)
		}
	}
	return result
}

// matchesPolicy 检查文件路径是否匹配策略
func (e *Engine) matchesPolicy(filePath string, policy *RetentionPolicy) bool {
	// 检查排除路径
	for _, exclude := range policy.ExcludePaths {
		if matchPath(exclude, filePath) {
			return false
		}
	}

	// 检查共享路径
	if len(policy.SharePaths) == 0 {
		return true // 没有指定路径则匹配所有
	}

	for _, sharePath := range policy.SharePaths {
		if matchPath(sharePath, filePath) {
			// 如果有文件模式过滤，进一步检查
			if len(policy.FilePatterns) == 0 {
				return true
			}
			for _, pattern := range policy.FilePatterns {
				if matchPattern(pattern, filePath) {
					return true
				}
			}
		}
	}

	return false
}

// matchPath 检查路径前缀匹配
func matchPath(prefix, path string) bool {
	if len(prefix) > len(path) {
		return false
	}
	return path[:len(prefix)] == prefix
}

// matchPattern 简单 glob 匹配
// TODO: 实现完整的 glob 匹配逻辑，当前仅支持后缀匹配
func matchPattern(pattern, path string) bool {
	if pattern == "" {
		return true
	}
	if pattern[0] == '*' {
		suffix := pattern[1:]
		return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
	}
	return path == pattern
}

// appendAudit 追加审计日志（调用方需持有写锁）
func (e *Engine) appendAudit(entry *ComplianceAuditEntry) {
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("audit-%d-%d", time.Now().UnixNano(), len(e.auditLog))
	}
	e.auditLog = append(e.auditLog, *entry)
	if len(e.auditLog) > e.maxAuditLog {
		e.auditLog = e.auditLog[len(e.auditLog)-e.maxAuditLog:]
	}
}
