// Package complianceengine 提供合规审计引擎功能
package complianceengine

import "time"

// ============================================================
// 标准类型定义
// ============================================================

// ComplianceStandard 合规标准
type ComplianceStandard string

const (
	StandardGDPR  ComplianceStandard = "gdpr"
	StandardHIPAA ComplianceStandard = "hipaa"
	StandardCIS   ComplianceStandard = "cis"
	StandardSOC2  ComplianceStandard = "soc2"
	StandardISO   ComplianceStandard = "iso27001"
)

// RuleCategory 规则类别
type RuleCategory string

const (
	CategoryAccessControl    RuleCategory = "access_control"
	CategoryAuditLogging     RuleCategory = "audit_logging"
	CategoryDataProtection   RuleCategory = "data_protection"
	CategoryNetworkSecurity  RuleCategory = "network_security"
	CategoryIncidentResponse RuleCategory = "incident_response"
)

// Severity 严重程度
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// ScanStatus 扫描状态
type ScanStatus string

const (
	StatusPending   ScanStatus = "pending"
	StatusRunning   ScanStatus = "running"
	StatusCompleted ScanStatus = "completed"
	StatusFailed    ScanStatus = "failed"
)

// CheckResult 检查结果
type CheckResult string

const (
	ResultPass    CheckResult = "pass"
	ResultFail    CheckResult = "fail"
	ResultWarning CheckResult = "warning"
	ResultSkip    CheckResult = "skip"
	ResultError   CheckResult = "error"
)

// AlertSeverity 告警严重程度
type AlertSeverity string

const (
	AlertCritical AlertSeverity = "critical"
	AlertHigh     AlertSeverity = "high"
	AlertMedium   AlertSeverity = "medium"
	AlertLow      AlertSeverity = "low"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
)

// ReportFormat 报告格式
type ReportFormat string

const (
	FormatJSON ReportFormat = "json"
	FormatPDF  ReportFormat = "pdf"
	FormatHTML ReportFormat = "html"
)

// ============================================================
// 核心类型
// ============================================================

// EngineConfig 引擎配置
type EngineConfig struct {
	Enabled       bool `json:"enabled"`
	AutoScan      bool `json:"auto_scan"`
	MaxConcurrent int  `json:"max_concurrent"`
}

// ComplianceRule 合规规则
type ComplianceRule struct {
	ID          string             `json:"id"`
	Standard    ComplianceStandard `json:"standard"`
	Category    RuleCategory       `json:"category"`
	Severity    Severity           `json:"severity"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Requirement string             `json:"requirement"`
	Remediation string             `json:"remediation"`
	Enabled     bool               `json:"enabled"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// ComplianceScan 合规扫描
type ComplianceScan struct {
	ID          string               `json:"id"`
	Standards   []ComplianceStandard `json:"standards"`
	Status      ScanStatus           `json:"status"`
	Score       float64              `json:"score"`
	TotalRules  int                  `json:"total_rules"`
	PassedRules int                  `json:"passed_rules"`
	FailedRules int                  `json:"failed_rules"`
	WarnRules   int                  `json:"warn_rules"`
	SkipRules   int                  `json:"skip_rules"`
	ErrorRules  int                  `json:"error_rules"`
	Checks      []CheckDetail        `json:"checks"`
	StartTime   time.Time            `json:"start_time"`
	EndTime     time.Time            `json:"end_time"`
	Duration    time.Duration        `json:"duration"`
}

// CheckDetail 检查详情
type CheckDetail struct {
	RuleID    string        `json:"rule_id"`
	Result    CheckResult   `json:"result"`
	Message   string        `json:"message"`
	CheckedAt time.Time     `json:"checked_at"`
	Duration  time.Duration `json:"duration"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID          string               `json:"id"`
	Title       string               `json:"title"`
	Format      ReportFormat         `json:"format"`
	ScanID      string               `json:"scan_id"`
	Standards   []ComplianceStandard `json:"standards"`
	Summary     ReportSummary        `json:"summary"`
	GeneratedAt time.Time            `json:"generated_at"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	ComplianceScore  float64 `json:"compliance_score"`
	TotalChecks      int     `json:"total_checks"`
	PassedChecks     int     `json:"passed_checks"`
	FailedChecks     int     `json:"failed_checks"`
	WarningChecks    int     `json:"warning_checks"`
	CriticalFindings int     `json:"critical_findings"`
	HighFindings     int     `json:"high_findings"`
	MediumFindings   int     `json:"medium_findings"`
	LowFindings      int     `json:"low_findings"`
}

// ComplianceAlert 合规告警
type ComplianceAlert struct {
	ID        string        `json:"id"`
	RuleID    string        `json:"rule_id"`
	Severity  AlertSeverity `json:"severity"`
	Title     string        `json:"title"`
	Message   string        `json:"message"`
	Status    string        `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// RemediationTask 修复任务
type RemediationTask struct {
	ID          string     `json:"id"`
	RuleID      string     `json:"rule_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    Severity   `json:"priority"`
	Status      TaskStatus `json:"status"`
	Assignee    string     `json:"assignee"`
	Commands    []string   `json:"commands"`
	Result      string     `json:"result"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// ComplianceStats 合规统计
type ComplianceStats struct {
	TotalScans      int        `json:"total_scans"`
	SuccessfulScans int        `json:"successful_scans"`
	FailedScans     int        `json:"failed_scans"`
	LastScanTime    *time.Time `json:"last_scan_time"`
	LastScanStatus  ScanStatus `json:"last_scan_status"`
	AverageScore    float64    `json:"average_score"`
	TotalAlerts     int        `json:"total_alerts"`
	ActiveAlerts    int        `json:"active_alerts"`
	TotalTasks      int        `json:"total_tasks"`
	PendingTasks    int        `json:"pending_tasks"`
	CompletedTasks  int        `json:"completed_tasks"`
}

// GetSnapshot 获取统计快照
func (s *ComplianceStats) GetSnapshot() *ComplianceStats {
	return &ComplianceStats{
		TotalScans:      s.TotalScans,
		SuccessfulScans: s.SuccessfulScans,
		FailedScans:     s.FailedScans,
		LastScanTime:    s.LastScanTime,
		LastScanStatus:  s.LastScanStatus,
		AverageScore:    s.AverageScore,
		TotalAlerts:     s.TotalAlerts,
		ActiveAlerts:    s.ActiveAlerts,
		TotalTasks:      s.TotalTasks,
		PendingTasks:    s.PendingTasks,
		CompletedTasks:  s.CompletedTasks,
	}
}

// UpdateScore 更新平均分
func (s *ComplianceStats) UpdateScore(score float64) {
	if s.TotalScans == 0 {
		s.AverageScore = score
	} else {
		s.AverageScore = (s.AverageScore*float64(s.TotalScans-1) + score) / float64(s.TotalScans)
	}
}

// GapAnalysis 差距分析
type GapAnalysis struct {
	ID          string               `json:"id"`
	Standards   []ComplianceStandard `json:"standards"`
	Score       float64              `json:"score"`
	Categories  []CategoryGap        `json:"categories"`
	Gaps        []ComplianceGap      `json:"gaps"`
	Actions     []RecommendedAction  `json:"actions"`
	GeneratedAt time.Time            `json:"generated_at"`
}

// CategoryGap 分类差距
type CategoryGap struct {
	Category    RuleCategory `json:"category"`
	TotalChecks int          `json:"total_checks"`
	Passed      int          `json:"passed"`
	Failed      int          `json:"failed"`
	GapCount    int          `json:"gap_count"`
	Score       float64      `json:"score"`
}

// ComplianceGap 合规差距
type ComplianceGap struct {
	RuleID      string             `json:"rule_id"`
	Standard    ComplianceStandard `json:"standard"`
	Severity    Severity           `json:"severity"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Current     string             `json:"current"`
	Required    string             `json:"required"`
	Impact      string             `json:"impact"`
}

// RecommendedAction 推荐行动
type RecommendedAction struct {
	ID          string   `json:"id"`
	Priority    Severity `json:"priority"`
	RuleID      string   `json:"rule_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Action      string   `json:"action"`
	Effort      string   `json:"effort"`
	AutoFixable bool     `json:"auto_fixable"`
}

// Issue 合规问题 (API 兼容)
type Issue struct {
	RuleID      string `json:"rule_id"`
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	Resource    string `json:"resource"`
	Description string `json:"description"`
	Fix         string `json:"fix"`
	Status      string `json:"status"` // open, in_progress, resolved
	Assignee    string `json:"assignee"`
}

// ComplianceTrend 合规趋势数据 (API 兼容)
type ComplianceTrend struct {
	Date  time.Time `json:"date"`
	Score int       `json:"score"`
}

// ScanRequest 扫描请求
type ScanRequest struct {
	Standard string `json:"standard" binding:"required"`
}

// IssueUpdateRequest 问题更新请求
type IssueUpdateRequest struct {
	Status   string `json:"status"`
	Assignee string `json:"assignee"`
}
