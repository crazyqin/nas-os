package smartcompliance

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// ComplianceStandard 合规标准.
type ComplianceStandard string

// 合规标准常量.
const (
	StandardGDPR     ComplianceStandard = "gdpr"
	StandardHIPAA    ComplianceStandard = "hipaa"
	StandardSOC2     ComplianceStandard = "soc2"
	StandardISO27001 ComplianceStandard = "iso27001"
	StandardPCI      ComplianceStandard = "pci"
	StandardCustom   ComplianceStandard = "custom"
)

// AuditStatus 审计状态.
type AuditStatus string

// 审计状态常量.
// 合规标准常量.
const (
	AuditStatusPending  AuditStatus = "pending"
	AuditStatusRunning  AuditStatus = "running"
	AuditStatusComplete AuditStatus = "complete"
	AuditStatusFailed   AuditStatus = "failed"
)

// Severity 严重程度.
type Severity string

// 严重程度常量.
// 合规标准常量.
const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// ComplianceRule 合规规则.
type ComplianceRule struct {
	ID          string             `json:"id"`
	Standard    ComplianceStandard `json:"standard"`
	Code        string             `json:"code"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Category    string             `json:"category"`
	Severity    Severity           `json:"severity"`
	Enabled     bool               `json:"enabled"`
	Metadata    map[string]string  `json:"metadata"`
}

// AuditResult 审计结果.
type AuditResult struct {
	ID          string             `json:"id"`
	Standard    ComplianceStandard `json:"standard"`
	Status      AuditStatus        `json:"status"`
	StartTime   time.Time          `json:"start_time"`
	EndTime     time.Time          `json:"end_time"`
	TotalChecks int                `json:"total_checks"`
	Passed      int                `json:"passed"`
	Failed      int                `json:"failed"`
	Warnings    int                `json:"warnings"`
	Score       float64            `json:"score"` // 0-100
	Findings    []Finding          `json:"findings"`
}

// Finding 发现项.
type Finding struct {
	RuleID      string   `json:"rule_id"`
	RuleName    string   `json:"rule_name"`
	Severity    Severity `json:"severity"`
	Status      string   `json:"status"` // pass/fail/warning
	Message     string   `json:"message"`
	Resource    string   `json:"resource"`
	Remediation string   `json:"remediation"`
}

// AccessPolicy 访问策略.
type AccessPolicy struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Subject    string    `json:"subject"`    // user/group/role
	Resource   string    `json:"resource"`   // path/pattern
	Actions    []string  `json:"actions"`    // read/write/delete/admin
	Conditions []string  `json:"conditions"` // time/ip/mfa
	Effect     string    `json:"effect"`     // allow/deny
	Priority   int       `json:"priority"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
}

// AccessAuditLog 访问审计日志.
type AccessAuditLog struct {
	ID        string    `json:"id"`
	Subject   string    `json:"subject"`
	Resource  string    `json:"resource"`
	Action    string    `json:"action"`
	Effect    string    `json:"effect"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
}

// Engine 合规审计引擎.
type Engine struct {
	rules    map[string]*ComplianceRule
	audits   map[string]*AuditResult
	policies map[string]*AccessPolicy
	logs     []*AccessAuditLog
	logger   *zap.Logger
	mu       sync.RWMutex
}

// NewEngine 创建合规审计引擎.
func NewEngine(logger *zap.Logger) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	engine := &Engine{
		rules:    make(map[string]*ComplianceRule),
		audits:   make(map[string]*AuditResult),
		policies: make(map[string]*AccessPolicy),
		logger:   logger,
	}
	engine.initDefaultRules()
	return engine
}

// initDefaultRules 初始化默认规则.
func (e *Engine) initDefaultRules() {
	defaults := []*ComplianceRule{
		{ID: "gdpr-001", Standard: StandardGDPR, Code: "GDPR-17", Name: "数据删除权", Description: "用户有权要求删除个人数据", Category: "data_rights", Severity: SeverityHigh},
		{ID: "gdpr-002", Standard: StandardGDPR, Code: "GDPR-25", Name: "数据保护设计", Description: "系统应默认保护数据隐私", Category: "privacy", Severity: SeverityMedium},
		{ID: "hipaa-001", Standard: StandardHIPAA, Code: "HIPAA-164", Name: "访问控制", Description: "实施最小权限访问控制", Category: "access", Severity: SeverityCritical},
		{ID: "hipaa-002", Standard: StandardHIPAA, Code: "HIPAA-164", Name: "审计日志", Description: "维护完整的访问审计日志", Category: "audit", Severity: SeverityHigh},
		{ID: "soc2-001", Standard: StandardSOC2, Code: "SOC2-CC6", Name: "逻辑访问", Description: "实施逻辑访问控制", Category: "access", Severity: SeverityHigh},
	}

	for _, rule := range defaults {
		rule.Enabled = true
		if rule.Metadata == nil {
			rule.Metadata = make(map[string]string)
		}
		e.rules[rule.ID] = rule
	}
}

// AddRule 添加合规规则.
func (e *Engine) AddRule(rule *ComplianceRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule.ID == "" {
		return ErrInvalidRuleID
	}
	if rule.Metadata == nil {
		rule.Metadata = make(map[string]string)
	}
	e.rules[rule.ID] = rule
	return nil
}

// GetRule 获取规则.
func (e *Engine) GetRule(id string) (*ComplianceRule, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rule, ok := e.rules[id]
	return rule, ok
}

// ListRules 列出规则.
func (e *Engine) ListRules(standard ComplianceStandard) []*ComplianceRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var rules []*ComplianceRule
	for _, r := range e.rules {
		if standard == "" || r.Standard == standard {
			rules = append(rules, r)
		}
	}
	return rules
}

// RunAudit 运行审计.
func (e *Engine) RunAudit(standard ComplianceStandard) (*AuditResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	audit := &AuditResult{
		ID:        generateID(),
		Standard:  standard,
		Status:    AuditStatusRunning,
		StartTime: time.Now(),
	}

	// 执行审计检查
	var findings []Finding
	passed, failed, warnings := 0, 0, 0

	for _, rule := range e.rules {
		if !rule.Enabled || (standard != "" && rule.Standard != standard) {
			continue
		}

		// 模拟检查
		finding := Finding{
			RuleID:   rule.ID,
			RuleName: rule.Name,
			Severity: rule.Severity,
			Status:   "pass",
			Message:  "检查通过",
		}

		// 随机模拟一些失败（实际应检查真实配置）
		if rule.Severity == SeverityCritical {
			finding.Status = "warning"
			finding.Message = "建议加强控制"
			finding.Remediation = "请配置额外的安全措施"
			warnings++
		} else {
			passed++
		}

		findings = append(findings, finding)
	}

	audit.Status = AuditStatusComplete
	audit.EndTime = time.Now()
	audit.TotalChecks = passed + failed + warnings
	audit.Passed = passed
	audit.Failed = failed
	audit.Warnings = warnings
	audit.Findings = findings

	if audit.TotalChecks > 0 {
		audit.Score = float64(passed) / float64(audit.TotalChecks) * 100
	}

	e.audits[audit.ID] = audit
	e.logger.Info("审计完成",
		zap.String("id", audit.ID),
		zap.String("standard", string(standard)),
		zap.Float64("score", audit.Score),
	)

	return audit, nil
}

// GetAudit 获取审计结果.
func (e *Engine) GetAudit(id string) (*AuditResult, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	audit, ok := e.audits[id]
	return audit, ok
}

// AddAccessPolicy 添加访问策略.
func (e *Engine) AddAccessPolicy(policy *AccessPolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if policy.ID == "" {
		return ErrInvalidPolicyID
	}
	policy.CreatedAt = time.Now()
	e.policies[policy.ID] = policy
	return nil
}

// CheckAccess 检查访问权限.
func (e *Engine) CheckAccess(subject, resource, action string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 按优先级检查策略
	var matchedPolicy *AccessPolicy
	highestPriority := -1

	for _, policy := range e.policies {
		if !policy.Enabled {
			continue
		}
		if policy.Subject == subject || policy.Subject == "*" {
			if matchResource(policy.Resource, resource) {
				if containsAction(policy.Actions, action) {
					if policy.Priority > highestPriority {
						matchedPolicy = policy
						highestPriority = policy.Priority
					}
				}
			}
		}
	}

	if matchedPolicy == nil {
		return false // 默认拒绝
	}

	return matchedPolicy.Effect == "allow"
}

// LogAccess 记录访问日志.
func (e *Engine) LogAccess(log *AccessAuditLog) {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Timestamp = time.Now()
	e.logs = append(e.logs, log)
}

// GetComplianceStatus 获取合规状态.
func (e *Engine) GetComplianceStatus() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	totalRules := len(e.rules)
	enabledRules := 0
	for _, r := range e.rules {
		if r.Enabled {
			enabledRules++
		}
	}

	latestScore := 0.0
	for _, a := range e.audits {
		if a.Score > latestScore {
			latestScore = a.Score
		}
	}

	return map[string]interface{}{
		"total_rules":      totalRules,
		"enabled_rules":    enabledRules,
		"total_audits":     len(e.audits),
		"total_policies":   len(e.policies),
		"compliance_score": latestScore,
		"audit_logs":       len(e.logs),
	}
}

func matchResource(pattern, resource string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == resource {
		return true
	}
	// 简单前缀匹配
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(resource) >= len(prefix) && resource[:len(prefix)] == prefix
	}
	return false
}

func containsAction(actions []string, action string) bool {
	for _, a := range actions {
		if a == "*" || a == action {
			return true
		}
	}
	return false
}

func generateID() string {
	return time.Now().Format("20060102150405")
}
