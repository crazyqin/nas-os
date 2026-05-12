// Package auditreport 提供安全审计报告功能
package auditreport

import (
	"time"
)

// Severity 严重级别.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// FindingStatus 发现状态.
type FindingStatus string

const (
	StatusOpen         FindingStatus = "open"
	StatusAcknowledged FindingStatus = "acknowledged"
	StatusResolved     FindingStatus = "resolved"
)

// AuditReport 审计报告.
type AuditReport struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Period      string      `json:"period"`
	GeneratedAt time.Time   `json:"generated_at"`
	Score       float64     `json:"score"`
	Findings    []Finding   `json:"findings"`
	Summary     string      `json:"summary"`
}

// Finding 安全发现.
type Finding struct {
	ID             string        `json:"id"`
	Severity       Severity      `json:"severity"`
	Category       string        `json:"category"`
	Description    string        `json:"description"`
	Recommendation string        `json:"recommendation"`
	Status         FindingStatus `json:"status"`
}

// ComplianceCheck 合规检查.
type ComplianceCheck struct {
	ID       string            `json:"id"`
	Standard string            `json:"standard"`
	Score    float64           `json:"score"`
	Passed   int               `json:"passed"`
	Failed   int               `json:"failed"`
	Items    []ComplianceItem  `json:"items"`
}

// ComplianceItem 合规检查项.
type ComplianceItem struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Passed      bool   `json:"passed"`
	Detail      string `json:"detail"`
}

// AuditEvent 审计事件.
type AuditEvent struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	IP        string    `json:"ip"`
	Result    string    `json:"result"`
}

// SecurityScanResult 安全扫描结果.
type SecurityScanResult struct {
	ID          string    `json:"id"`
	ScanType    string    `json:"scan_type"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Total       int       `json:"total"`
	Critical    int       `json:"critical"`
	High        int       `json:"high"`
	Medium      int       `json:"medium"`
	Low         int       `json:"low"`
	Info        int       `json:"info"`
	Findings    []Finding `json:"findings"`
}

// ========== Request/Response ==========

// GenerateReportRequest 生成报告请求.
type GenerateReportRequest struct {
	Title  string `json:"title" binding:"required"`
	Period string `json:"period" binding:"required"`
}

// UpdateFindingRequest 更新发现请求.
type UpdateFindingRequest struct {
	Status         *FindingStatus `json:"status,omitempty"`
	Recommendation *string        `json:"recommendation,omitempty"`
}

// RunComplianceCheckRequest 运行合规检查请求.
type RunComplianceCheckRequest struct {
	Standard string `json:"standard" binding:"required"`
}

// RunSecurityScanRequest 运行安全扫描请求.
type RunSecurityScanRequest struct {
	ScanType string `json:"scan_type" binding:"required"`
}

// ExportEventsRequest 导出事件请求.
type ExportEventsRequest struct {
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Format    string     `json:"format"` // json or csv
}

// QueryEventsRequest 查询事件请求.
type QueryEventsRequest struct {
	UserID   string `form:"user_id"`
	Action   string `form:"action"`
	Resource string `form:"resource"`
	Result   string `form:"result"`
	Limit    int    `form:"limit"`
}
