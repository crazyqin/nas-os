// Package compliancescan 提供数据合规扫描功能
package compliancescan

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrRuleNotFound 规则不存在.
	ErrRuleNotFound = errors.New("扫描规则不存在")
	// ErrTaskNotFound 任务不存在.
	ErrTaskNotFound = errors.New("扫描任务不存在")
	// ErrViolationNotFound 违规不存在.
	ErrViolationNotFound = errors.New("违规记录不存在")
	// ErrResultNotFound 结果不存在.
	ErrResultNotFound = errors.New("扫描结果不存在")
	// ErrTaskNotPending 任务状态不允许执行.
	ErrTaskNotPending = errors.New("任务状态不允许执行")
	// ErrInvalidRule 无效规则.
	ErrInvalidRule = errors.New("无效的扫描规则")
	// ErrViolationAlreadyResolved 违规已解决.
	ErrViolationAlreadyResolved = errors.New("违规已解决")
	// ErrFileNotQuarantinable 文件不可隔离.
	ErrFileNotQuarantinable = errors.New("文件不可隔离")
)

// ========== 规则分类 ==========

// RuleCategory 规则分类.
type RuleCategory string

const (
	// CategoryPII 个人身份信息.
	CategoryPII RuleCategory = "pii"
	// CategoryFinancial 金融数据.
	CategoryFinancial RuleCategory = "financial"
	// CategoryHealth 健康数据.
	CategoryHealth RuleCategory = "health"
	// CategoryCustom 自定义规则.
	CategoryCustom RuleCategory = "custom"
)

// ========== 严重程度 ==========

// Severity 严重程度.
type Severity string

const (
	// SeverityLow 低风险.
	SeverityLow Severity = "low"
	// SeverityMedium 中风险.
	SeverityMedium Severity = "medium"
	// SeverityHigh 高风险.
	SeverityHigh Severity = "high"
	// SeverityCritical 严重风险.
	SeverityCritical Severity = "critical"
)

// ========== 处置动作 ==========

// Action 处置动作.
type Action string

const (
	// ActionLog 仅记录日志.
	ActionLog Action = "log"
	// ActionQuarantine 隔离文件.
	ActionQuarantine Action = "quarantine"
	// ActionEncrypt 加密文件.
	ActionEncrypt Action = "encrypt"
	// ActionDelete 删除文件.
	ActionDelete Action = "delete"
	// ActionNotify 发送通知.
	ActionNotify Action = "notify"
)

// ========== 任务状态 ==========

// TaskStatus 任务状态.
type TaskStatus string

const (
	// StatusPending 等待执行.
	StatusPending TaskStatus = "pending"
	// StatusScanning 扫描中.
	StatusScanning TaskStatus = "scanning"
	// StatusCompleted 已完成.
	StatusCompleted TaskStatus = "completed"
	// StatusFailed 失败.
	StatusFailed TaskStatus = "failed"
)

// ========== 敏感级别 ==========

// SensitivityLevel 敏感级别.
type SensitivityLevel string

const (
	// SensitivityPublic 公开.
	SensitivityPublic SensitivityLevel = "public"
	// SensitivityInternal 内部.
	SensitivityInternal SensitivityLevel = "internal"
	// SensitivityConfidential 机密.
	SensitivityConfidential SensitivityLevel = "confidential"
	// SensitivityRestricted 受限.
	SensitivityRestricted SensitivityLevel = "restricted"
)

// ========== 核心数据结构 ==========

// ScanRule 扫描规则.
type ScanRule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Category    RuleCategory `json:"category"`
	Pattern     string       `json:"pattern"`
	Severity    Severity     `json:"severity"`
	Enabled     bool         `json:"enabled"`
	Action      Action       `json:"action"`
	Description string       `json:"description"`
	CreatedAt   time.Time    `json:"created_at"`
}

// ScanTask 扫描任务.
type ScanTask struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	TargetPath string     `json:"target_path"`
	RuleIDs    []string   `json:"rule_ids"`
	Status     TaskStatus `json:"status"`
	Progress   float64    `json:"progress"`
	StartTime  *time.Time `json:"start_time,omitempty"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ScanResult 扫描结果.
type ScanResult struct {
	ID             string    `json:"id"`
	TaskID         string    `json:"task_id"`
	TotalFiles     int       `json:"total_files"`
	ScannedFiles   int       `json:"scanned_files"`
	ViolationCount int       `json:"violation_count"`
	RiskScore      float64   `json:"risk_score"`
	Duration       string    `json:"duration"`
	CreatedAt      time.Time `json:"created_at"`
}

// Violation 违规记录.
type Violation struct {
	ID           string     `json:"id"`
	ResultID     string     `json:"result_id"`
	RuleID       string     `json:"rule_id"`
	RuleName     string     `json:"rule_name"`
	FilePath     string     `json:"file_path"`
	LineNumber   int        `json:"line_number"`
	MatchContent string     `json:"match_content"`
	Severity     Severity   `json:"severity"`
	Action       Action     `json:"action"`
	Resolved     bool       `json:"resolved"`
	ResolvedBy   string     `json:"resolved_by,omitempty"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// RuleSummary 规则摘要.
type RuleSummary struct {
	RuleID         string   `json:"rule_id"`
	RuleName       string   `json:"rule_name"`
	ViolationCount int      `json:"violation_count"`
	Severity       Severity `json:"severity"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	GeneratedAt          time.Time      `json:"generated_at"`
	TotalScans           int            `json:"total_scans"`
	ViolationsBySeverity map[string]int `json:"violations_by_severity"`
	ViolationsByCategory map[string]int `json:"violations_by_category"`
	TopViolatedRules     []RuleSummary  `json:"top_violated_rules"`
	RiskScore            float64        `json:"risk_score"`
	Recommendations      []string       `json:"recommendations"`
}

// ScanSchedule 扫描调度.
type ScanSchedule struct {
	ID       string    `json:"id"`
	TaskID   string    `json:"task_id"`
	CronExpr string    `json:"cron_expr"`
	Enabled  bool      `json:"enabled"`
	LastRun  time.Time `json:"last_run,omitempty"`
	NextRun  time.Time `json:"next_run,omitempty"`
}

// DataClassification 数据分类.
type DataClassification struct {
	FilePath         string           `json:"file_path"`
	Categories       []string         `json:"categories"`
	SensitivityLevel SensitivityLevel `json:"sensitivity_level"`
	LastScanned      time.Time        `json:"last_scanned"`
	ViolationCount   int              `json:"violation_count"`
}
