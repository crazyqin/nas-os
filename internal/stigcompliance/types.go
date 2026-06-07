// Package stigcompliance STIG合规检查器
// 基于DISA STIG（Security Technical Implementation Guides）标准的安全合规检查。
// 对标TrueNAS GPOS STIG合规，支持自动化审计和修复建议。
package stigcompliance

import (
	"errors"
	"sync"
	"time"
)

// SeverityLevel 严重程度
type SeverityLevel string

const (
	SeverityCat1 SeverityLevel = "cat1" // 严重
	SeverityCat2 SeverityLevel = "cat2" // 中等
	SeverityCat3 SeverityLevel = "cat3" // 低
)

// CheckStatus 检查状态
type CheckStatus string

const (
	CheckPass     CheckStatus = "pass"
	CheckFail     CheckStatus = "fail"
	CheckSkip     CheckStatus = "skip"
	CheckNotApplicable CheckStatus = "not_applicable"
)

// STIGRule STIG规则
type STIGRule struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Severity    SeverityLevel `json:"severity"`
	Category    string        `json:"category"`
	Fix         string        `json:"fix"`
	Check       string        `json:"check"`
	Enabled     bool          `json:"enabled"`
}

// CheckResult 检查结果
type CheckResult struct {
	RuleID      string        `json:"rule_id"`
	RuleTitle   string        `json:"rule_title"`
	Status      CheckStatus   `json:"status"`
	Severity    SeverityLevel `json:"severity"`
	Message     string        `json:"message"`
	Remediation string        `json:"remediation,omitempty"`
	CheckedAt   time.Time     `json:"checked_at"`
}

// AuditReport 审计报告
type AuditReport struct {
	ID          string         `json:"id"`
	TotalRules  int            `json:"total_rules"`
	Passed      int            `json:"passed"`
	Failed      int            `json:"failed"`
	Skipped     int            `json:"skipped"`
	Score       float64        `json:"score"` // 0-100
	Results     []CheckResult  `json:"results"`
	GeneratedAt time.Time      `json:"generated_at"`
	Duration    int64          `json:"duration_ms"`
}

// STIGComplianceChecker STIG合规检查器
type STIGComplianceChecker struct {
	mu     sync.RWMutex
	rules  map[string]*STIGRule
	reports []*AuditReport
	config CheckerConfig
}

// CheckerConfig 检查器配置
type CheckerConfig struct {
	AutoFix        bool     `json:"auto_fix"`
	FailOnCat1     bool     `json:"fail_on_cat1"`
	FailOnCat2     bool     `json:"fail_on_cat2"`
	ExcludedRules  []string `json:"excluded_rules"`
	ReportDir      string   `json:"report_dir"`
}

// DefaultCheckerConfig 默认配置
func DefaultCheckerConfig() CheckerConfig {
	return CheckerConfig{
		AutoFix:    false,
		FailOnCat1: true,
		FailOnCat2: false,
		ReportDir:  "/var/reports/stig",
	}
}

// 预定义错误
var (
	ErrRuleNotFound    = errors.New("STIG rule not found")
	ErrRuleExists      = errors.New("STIG rule already exists")
	ErrReportNotFound  = errors.New("audit report not found")
	ErrCheckerRunning  = errors.New("checker is already running")
)
