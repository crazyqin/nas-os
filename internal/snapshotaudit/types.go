// Package snapshotaudit 提供合规快照审计功能
// 支持快照完整性验证、合规性检查、审计日志、篡改检测
package snapshotaudit

import (
	"time"
)

// AuditStatus 审计状态
type AuditStatus string

const (
	StatusPending AuditStatus = "pending"
	StatusRunning AuditStatus = "running"
	StatusPassed  AuditStatus = "passed"
	StatusFailed  AuditStatus = "failed"
	StatusWarning AuditStatus = "warning"
)

// SnapshotStatus 快照状态
type SnapshotStatus string

const (
	SnapshotValid     SnapshotStatus = "valid"
	SnapshotCorrupted SnapshotStatus = "corrupted"
	SnapshotMissing   SnapshotStatus = "missing"
	SnapshotTampered  SnapshotStatus = "tampered"
	SnapshotExpired   SnapshotStatus = "expired"
)

// AuditResult 审计结果
type AuditResult string

const (
	ResultCompliant    AuditResult = "compliant"
	ResultNonCompliant AuditResult = "non_compliant"
	ResultUnknown      AuditResult = "unknown"
)

// SnapshotRecord 快照记录
type SnapshotRecord struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Volume        string         `json:"volume"`
	Size          int64          `json:"size"`
	Hash          string         `json:"hash"` // 完整性校验哈希
	Status        SnapshotStatus `json:"status"`
	CreatedAt     time.Time      `json:"createdAt"`
	ExpiresAt     *time.Time     `json:"expiresAt,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
	RetentionDays int            `json:"retentionDays"`
	Verified      bool           `json:"verified"`
	VerifiedAt    *time.Time     `json:"verifiedAt,omitempty"`
}

// AuditPolicy 审计策略
type AuditPolicy struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	// 检查项
	CheckIntegrity  bool `json:"checkIntegrity"`  // 完整性检查
	CheckRetention  bool `json:"checkRetention"`  // 保留期检查
	CheckCompliance bool `json:"checkCompliance"` // 合规性检查
	CheckAccess     bool `json:"checkAccess"`     // 访问控制检查
	// 合规标准
	Standard         string `json:"standard,omitempty"` // GDPR, HIPAA, 等保2.0
	MinRetentionDays int    `json:"minRetentionDays"`
	MaxRetentionDays int    `json:"maxRetentionDays"`
	// 调度
	ScheduleCron string    `json:"scheduleCron"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// AuditReport 审计报告
type AuditReport struct {
	ID             string        `json:"id"`
	PolicyID       string        `json:"policyId"`
	PolicyName     string        `json:"policyName"`
	Status         AuditStatus   `json:"status"`
	Result         AuditResult   `json:"result"`
	TotalSnapshots int           `json:"totalSnapshots"`
	ValidCount     int           `json:"validCount"`
	FailedCount    int           `json:"failedCount"`
	WarningCount   int           `json:"warningCount"`
	Issues         []*AuditIssue `json:"issues,omitempty"`
	StartTime      time.Time     `json:"startTime"`
	EndTime        time.Time     `json:"endTime"`
	Duration       time.Duration `json:"duration"`
}

// AuditIssue 审计问题
type AuditIssue struct {
	SnapshotID   string `json:"snapshotId"`
	SnapshotName string `json:"snapshotName"`
	Severity     string `json:"severity"` // info, warning, critical
	Code         string `json:"code"`     // 问题代码
	Message      string `json:"message"`
	Suggestion   string `json:"suggestion,omitempty"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	User      string    `json:"user,omitempty"`
	Details   string    `json:"details,omitempty"`
	IPAddress string    `json:"ipAddress,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// AuditStats 审计统计
type AuditStats struct {
	TotalAudits       int            `json:"totalAudits"`
	PassedAudits      int            `json:"passedAudits"`
	FailedAudits      int            `json:"failedAudits"`
	TotalSnapshots    int            `json:"totalSnapshots"`
	VerifiedSnapshots int            `json:"verifiedSnapshots"`
	TotalIssues       int            `json:"totalIssues"`
	CriticalIssues    int            `json:"criticalIssues"`
	ComplianceRate    float64        `json:"complianceRate"`
	LastAuditTime     *time.Time     `json:"lastAuditTime,omitempty"`
	StatusBreakdown   map[string]int `json:"statusBreakdown"`
}
