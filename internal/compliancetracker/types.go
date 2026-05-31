// Package compliancetracker 提供合规审计追踪功能
package compliancetracker

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrRuleNotFound 合规规则不存在.
	ErrRuleNotFound = errors.New("合规规则不存在")
	// ErrCheckNotFound 合规检查不存在.
	ErrCheckNotFound = errors.New("合规检查不存在")
	// ErrInvalidTimeRange 无效的时间范围.
	ErrInvalidTimeRange = errors.New("无效的时间范围")
	// ErrInvalidRule 无效的合规规则.
	ErrInvalidRule = errors.New("无效的合规规则")
	// ErrDuplicateRule 重复的合规规则.
	ErrDuplicateRule = errors.New("重复的合规规则")
)

// ========== 合规状态 ==========

// ComplianceStatus 合规状态.
type ComplianceStatus string

const (
	// StatusCompliant 合规.
	StatusCompliant ComplianceStatus = "compliant"
	// StatusNonCompliant 不合规.
	StatusNonCompliant ComplianceStatus = "non_compliant"
	// StatusPartial 部分合规.
	StatusPartial ComplianceStatus = "partial"
	// StatusPending 待检查.
	StatusPending ComplianceStatus = "pending"
	// StatusError 检查错误.
	StatusError ComplianceStatus = "error"
)

// ========== 风险等级 ==========

// SeverityLevel 严重程度.
type SeverityLevel string

const (
	// SeverityLow 低严重度.
	SeverityLow SeverityLevel = "low"
	// SeverityMedium 中严重度.
	SeverityMedium SeverityLevel = "medium"
	// SeverityHigh 高严重度.
	SeverityHigh SeverityLevel = "high"
	// SeverityCritical 严重.
	SeverityCritical SeverityLevel = "critical"
)

// ========== 规则类型 ==========

// RuleType 规则类型.
type RuleType string

const (
	// RuleTypeAccess 访问控制规则.
	RuleTypeAccess RuleType = "access"
	// RuleTypeEncryption 加密规则.
	RuleTypeEncryption RuleType = "encryption"
	// RuleTypeRetention 数据保留规则.
	RuleTypeRetention RuleType = "retention"
	// RuleTypeAudit 审计规则.
	RuleTypeAudit RuleType = "audit"
	// RuleTypePrivacy 隐私保护规则.
	RuleTypePrivacy RuleType = "privacy"
	// RuleTypeCustom 自定义规则.
	RuleTypeCustom RuleType = "custom"
)

// ========== 核心数据结构 ==========

// ComplianceRule 合规规则.
type ComplianceRule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	RuleType    RuleType     `json:"rule_type"`
	Category    string       `json:"category,omitempty"`
	Severity    SeverityLevel `json:"severity"`
	Enabled     bool         `json:"enabled"`
	Conditions  []Condition  `json:"conditions"`
	Remediation string       `json:"remediation,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	CreatedBy   string       `json:"created_by,omitempty"`
}

// Condition 合规条件.
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"` // eq, ne, gt, lt, gte, lte, contains, regex
	Value    string `json:"value"`
	Logical  string `json:"logical,omitempty"` // and, or
}

// ComplianceCheck 合规检查结果.
type ComplianceCheck struct {
	ID          string           `json:"id"`
	RuleID      string           `json:"rule_id"`
	RuleName    string           `json:"rule_name"`
	Timestamp   time.Time        `json:"timestamp"`
	Status      ComplianceStatus `json:"status"`
	Target      string           `json:"target"`
	TargetType  string           `json:"target_type,omitempty"`
	Details     string           `json:"details,omitempty"`
	Violations  []Violation      `json:"violations,omitempty"`
	CheckDuration int64          `json:"check_duration_ms,omitempty"`
	CheckedBy   string           `json:"checked_by,omitempty"`
}

// Violation 合规违规.
type Violation struct {
	Field      string       `json:"field"`
	Expected   string       `json:"expected"`
	Actual     string       `json:"actual"`
	Severity   SeverityLevel `json:"severity"`
	Message    string       `json:"message"`
}

// AuditLog 审计日志.
type AuditLog struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Actor     string    `json:"actor"`
	Target    string    `json:"target"`
	Details   string    `json:"details,omitempty"`
	Status    string    `json:"status"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	ID                string                  `json:"id"`
	GeneratedAt       time.Time               `json:"generated_at"`
	StartTime         time.Time               `json:"start_time"`
	EndTime           time.Time               `json:"end_time"`
	TotalChecks       int                     `json:"total_checks"`
	CompliantCount    int                     `json:"compliant_count"`
	NonCompliantCount int                     `json:"non_compliant_count"`
	PartialCount      int                     `json:"partial_count"`
	PendingCount      int                     `json:"pending_count"`
	ErrorCount        int                     `json:"error_count"`
	ComplianceRate    float64                 `json:"compliance_rate"`
	RuleSummary       []RuleSummary           `json:"rule_summary"`
	TopViolations     []ViolationSummary      `json:"top_violations"`
	TrendData         []TrendPoint            `json:"trend_data"`
	Recommendations   []string                `json:"recommendations"`
	StatusDistribution map[ComplianceStatus]int `json:"status_distribution"`
}

// RuleSummary 规则摘要.
type RuleSummary struct {
	RuleID         string           `json:"rule_id"`
	RuleName       string           `json:"rule_name"`
	TotalChecks    int              `json:"total_checks"`
	CompliantCount int              `json:"compliant_count"`
	ComplianceRate float64          `json:"compliance_rate"`
	LastCheckTime  time.Time        `json:"last_check_time"`
	Status         ComplianceStatus `json:"status"`
}

// ViolationSummary 违规摘要.
type ViolationSummary struct {
	RuleName    string       `json:"rule_name"`
	Field       string       `json:"field"`
	Count       int          `json:"count"`
	Severity    SeverityLevel `json:"severity"`
	LastSeen    time.Time    `json:"last_seen"`
}

// TrendPoint 趋势数据点.
type TrendPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	ComplianceRate float64   `json:"compliance_rate"`
	TotalChecks    int       `json:"total_checks"`
	Violations     int       `json:"violations"`
}

// QueryFilter 查询过滤器.
type QueryFilter struct {
	StartTime    *time.Time        `json:"start_time,omitempty"`
	EndTime      *time.Time        `json:"end_time,omitempty"`
	RuleID       string            `json:"rule_id,omitempty"`
	RuleType     RuleType          `json:"rule_type,omitempty"`
	Status       ComplianceStatus  `json:"status,omitempty"`
	Target       string            `json:"target,omitempty"`
	TargetType   string            `json:"target_type,omitempty"`
	Severity     SeverityLevel     `json:"severity,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Limit        int               `json:"limit,omitempty"`
	Offset       int               `json:"offset,omitempty"`
}

// ComplianceConfig 合规配置.
type ComplianceConfig struct {
	// AutoCheckEnabled 启用自动检查.
	AutoCheckEnabled bool `json:"auto_check_enabled"`
	// AutoCheckInterval 自动检查间隔（分钟）.
	AutoCheckInterval int `json:"auto_check_interval"`
	// AlertOnViolation 违规时告警.
	AlertOnViolation bool `json:"alert_on_violation"`
	// AlertThreshold 告警阈值（合规率低于此值触发告警）.
	AlertThreshold float64 `json:"alert_threshold"`
	// RetentionDays 审计日志保留天数.
	RetentionDays int `json:"retention_days"`
	// MaxChecksPerRule 每个规则最大检查数.
	MaxChecksPerRule int `json:"max_checks_per_rule"`
}

// DefaultComplianceConfig 返回默认合规配置.
func DefaultComplianceConfig() *ComplianceConfig {
	return &ComplianceConfig{
		AutoCheckEnabled:  true,
		AutoCheckInterval: 60,
		AlertOnViolation:  true,
		AlertThreshold:    80.0,
		RetentionDays:     90,
		MaxChecksPerRule:  1000,
	}
}
