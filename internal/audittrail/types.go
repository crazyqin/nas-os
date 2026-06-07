// Package audittrail 提供审计追踪功能，包括操作日志全记录、可疑行为检测、审计报告导出等。
package audittrail

import (
	"time"
)

// ==================== 审计事件相关 ====================

// AuditEvent 审计事件
type AuditEvent struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	EventType string            `json:"event_type"` // login, logout, access, modify, delete, admin
	Actor     Actor             `json:"actor"`
	Resource  Resource          `json:"resource"`
	Action    string            `json:"action"`
	Result    string            `json:"result"` // success, failure, denied
	Details   string            `json:"details"`
	IPAddress string            `json:"ip_address"`
	UserAgent string            `json:"user_agent"`
	Location  string            `json:"location,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Severity  string            `json:"severity"` // info, warning, critical
}

// Actor 操作者
type Actor struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"` // user, system, service
	Role  string `json:"role"`
	Email string `json:"email,omitempty"`
}

// Resource 资源
type Resource struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"` // file, database, api, system
	Path  string `json:"path,omitempty"`
	Owner string `json:"owner,omitempty"`
}

// EventFilter 事件过滤器
type EventFilter struct {
	StartTime  *time.Time `json:"start_time,omitempty"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	EventTypes []string   `json:"event_types,omitempty"`
	Actors     []string   `json:"actors,omitempty"`
	Resources  []string   `json:"resources,omitempty"`
	Severities []string   `json:"severities,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
}

// ==================== 可疑行为相关 ====================

// SuspiciousActivity 可疑行为
type SuspiciousActivity struct {
	ID            string     `json:"id"`
	Timestamp     time.Time  `json:"timestamp"`
	Type          string     `json:"type"` // brute-force, data-exfiltration, privilege-escalation, anomaly
	Actor         Actor      `json:"actor"`
	Description   string     `json:"description"`
	Indicators    []string   `json:"indicators"`
	RiskScore     float64    `json:"risk_score"` // 0-100
	Status        string     `json:"status"`     // detected, investigating, confirmed, false-positive
	RelatedEvents []string   `json:"related_events,omitempty"`
	AssignedTo    string     `json:"assigned_to,omitempty"`
	Notes         string     `json:"notes,omitempty"`
	DetectedAt    time.Time  `json:"detected_at"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
}

// SuspiciousFilter 可疑行为过滤器
type SuspiciousFilter struct {
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Types     []string   `json:"types,omitempty"`
	Statuses  []string   `json:"statuses,omitempty"`
	MinScore  *float64   `json:"min_score,omitempty"`
	Limit     int        `json:"limit,omitempty"`
}

// ==================== 审计报告相关 ====================

// AuditReport 审计报告
type AuditReport struct {
	ID          string               `json:"id"`
	Title       string               `json:"title"`
	Type        string               `json:"type"` // summary, detailed, compliance
	Period      ReportPeriod         `json:"period"`
	Summary     ReportSummary        `json:"summary"`
	Events      []AuditEvent         `json:"events,omitempty"`
	Activities  []SuspiciousActivity `json:"activities,omitempty"`
	GeneratedAt time.Time            `json:"generated_at"`
	GeneratedBy string               `json:"generated_by"`
	Format      string               `json:"format"` // json, csv, pdf
}

// ReportPeriod 报告周期
type ReportPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalEvents      int            `json:"total_events"`
	EventsByType     map[string]int `json:"events_by_type"`
	EventsBySeverity map[string]int `json:"events_by_severity"`
	TopActors        []ActorStat    `json:"top_actors"`
	TopResources     []ResourceStat `json:"top_resources"`
	SuspiciousCount  int            `json:"suspicious_count"`
	FailureRate      float64        `json:"failure_rate"`
}

// ActorStat 操作者统计
type ActorStat struct {
	ActorID   string `json:"actor_id"`
	ActorName string `json:"actor_name"`
	Count     int    `json:"count"`
}

// ResourceStat 资源统计
type ResourceStat struct {
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	Count        int    `json:"count"`
}

// ==================== 保留策略相关 ====================

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	EventTypes  []string  `json:"event_types"`
	Severity    []string  `json:"severity,omitempty"`
	Duration    string    `json:"duration"` // 30d, 90d, 1y, forever
	Action      string    `json:"action"`   // archive, delete, anonymize
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RetentionStats 保留统计
type RetentionStats struct {
	PolicyID    string    `json:"policy_id"`
	TotalEvents int       `json:"total_events"`
	Archived    int       `json:"archived"`
	Deleted     int       `json:"deleted"`
	Anonymized  int       `json:"anonymized"`
	OldestEvent time.Time `json:"oldest_event"`
	NewestEvent time.Time `json:"newest_event"`
	LastRunAt   time.Time `json:"last_run_at"`
}

// ==================== 导出相关 ====================

// AuditExport 审计导出
type AuditExport struct {
	ID          string      `json:"id"`
	Format      string      `json:"format"` // json, csv, pdf
	Filter      EventFilter `json:"filter"`
	Status      string      `json:"status"` // pending, processing, completed, failed
	FileSize    int64       `json:"file_size,omitempty"`
	FilePath    string      `json:"file_path,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	Error       string      `json:"error,omitempty"`
	ExpiresAt   time.Time   `json:"expires_at"`
}

// ExportRequest 导出请求
type ExportRequest struct {
	Format string      `json:"format"` // json, csv, pdf
	Filter EventFilter `json:"filter"`
	Async  bool        `json:"async,omitempty"`
}
