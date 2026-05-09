// Package auditexport 提供合规审计日志导出功能。
// 支持将系统审计日志导出为 CSV/JSON 格式，满足企业合规需求。
// 参考群晖 Log Center 的导出功能，但增加合规报告能力。
package auditexport

import "time"

// AuditEntry 审计日志条目
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	Action    string    `json:"action"`   // login / logout / read / write / delete / config / admin
	Resource  string    `json:"resource"` // 操作对象路径
	Result    string    `json:"result"`   // success / denied / failed
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Details   string    `json:"details"`
	Severity  string    `json:"severity"` // info / warning / critical
}

// ExportFilter 导出过滤条件
type ExportFilter struct {
	StartTime  *time.Time `json:"start_time"`
	EndTime    *time.Time `json:"end_time"`
	UserIDs    []string   `json:"user_ids"`
	Actions    []string   `json:"actions"`
	Results    []string   `json:"results"`
	Severities []string   `json:"severities"`
	Resource   string     `json:"resource"`
	Limit      int        `json:"limit"`
}

// ExportFormat 导出格式
type ExportFormat string

const (
	FormatCSV  ExportFormat = "csv"
	FormatJSON ExportFormat = "json"
)

// ComplianceReport 合规报告摘要
type ComplianceReport struct {
	GeneratedAt    time.Time        `json:"generated_at"`
	PeriodStart    time.Time        `json:"period_start"`
	PeriodEnd      time.Time        `json:"period_end"`
	TotalEvents    int              `json:"total_events"`
	ActionStats    map[string]int   `json:"action_stats"`
	ResultStats    map[string]int   `json:"result_stats"`
	SecurityEvents []AuditEntry     `json:"security_events"`
	TopUsers       []UserActivity   `json:"top_users"`
	AnomalyLogins  []AuditEntry     `json:"anomaly_logins"`
}

// UserActivity 用户活跃度
type UserActivity struct {
	UserID      string    `json:"user_id"`
	UserName    string    `json:"user_name"`
	ActionCount int       `json:"action_count"`
	LastActive  time.Time `json:"last_active"`
}
