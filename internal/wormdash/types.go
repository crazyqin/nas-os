// Package wormdash 提供 WORM 合规仪表盘功能
// 合规概览、策略管理、报告生成、异常检测、审计追踪
package wormdash

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// PolicyScope 策略作用域
type PolicyScope string

const (
	ScopeDirectory PolicyScope = "directory" // 按目录
	ScopeFileType  PolicyScope = "filetype"  // 按文件类型
	ScopeGlobal    PolicyScope = "global"    // 全局
)

// PolicyStatus 策略状态
type PolicyStatus string

const (
	PolicyActive   PolicyStatus = "active"
	PolicyInactive PolicyStatus = "inactive"
	PolicyDraft    PolicyStatus = "draft"
)

// AlertSeverity 告警级别
type AlertSeverity string

const (
	AlertCritical AlertSeverity = "critical"
	AlertHigh     AlertSeverity = "high"
	AlertMedium   AlertSeverity = "medium"
	AlertLow      AlertSeverity = "low"
)

// AuditAction 审计操作类型
type AuditAction string

const (
	ActionLock       AuditAction = "lock"
	ActionUnlock     AuditAction = "unlock"
	ActionVerify     AuditAction = "verify"
	ActionExpire     AuditAction = "expire"
	ActionPolicyAdd  AuditAction = "policy_add"
	ActionPolicyDel  AuditAction = "policy_del"
	ActionReport     AuditAction = "report_gen"
	ActionBypass     AuditAction = "bypass_attempt"
	ActionRetention  AuditAction = "retention_change"
)

// Overview 仪表盘概览
type Overview struct {
	ProtectedFiles int64   `json:"protectedFiles"`
	TotalSizeBytes int64   `json:"totalSizeBytes"`
	ComplianceRate float64 `json:"complianceRate"` // 0-100
	TotalPolicies  int     `json:"totalPolicies"`
	ActivePolicies int     `json:"activePolicies"`
	OpenAlerts     int     `json:"openAlerts"`
	ExpiredFiles   int     `json:"expiredFiles"`
	BrokenFiles    int     `json:"brokenFiles"`
	LastAuditAt    string  `json:"lastAuditAt,omitempty"`
}

// WORMPolicy WORM合规策略
type WORMPolicy struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Scope         PolicyScope  `json:"scope"`
	Target        string       `json:"target"`            // 目录路径或文件扩展名
	RetentionDays int          `json:"retentionDays"`     // 0 = 永不过期
	Status        PolicyStatus `json:"status"`
	Description   string       `json:"description,omitempty"`
	CreatedBy     string       `json:"createdBy"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID              string            `json:"id"`
	ReportType      string            `json:"reportType"` // monthly|quarterly
	PeriodStart     time.Time         `json:"periodStart"`
	PeriodEnd       time.Time         `json:"periodEnd"`
	GeneratedAt     time.Time         `json:"generatedAt"`
	GeneratedBy     string            `json:"generatedBy"`
	TotalFiles      int               `json:"totalFiles"`
	ProtectedFiles  int               `json:"protectedFiles"`
	ComplianceRate  float64           `json:"complianceRate"`
	Violations      int               `json:"violations"`
	RetentionStats  map[string]int    `json:"retentionStats,omitempty"`
	PolicyStats     map[string]int    `json:"policyStats,omitempty"`
	Summary         string            `json:"summary"`
}

// AnomalyAlert 异常告警
type AnomalyAlert struct {
	ID          string        `json:"id"`
	Severity    AlertSeverity `json:"severity"`
	Type        string        `json:"type"` // bypass_attempt|integrity_violation|unauthorized_access
	Description string        `json:"description"`
	SourcePath  string        `json:"sourcePath,omitempty"`
	SourceIP    string        `json:"sourceIp,omitempty"`
	UserID      string        `json:"userId,omitempty"`
	DetectedAt  time.Time     `json:"detectedAt"`
	Resolved    bool          `json:"resolved"`
	ResolvedAt  *time.Time    `json:"resolvedAt,omitempty"`
}

// RetentionEntry 保留期记录
type RetentionEntry struct {
	FileID        string     `json:"fileId"`
	FilePath      string     `json:"filePath"`
	RetentionDays int        `json:"retentionDays"`
	LockedAt      time.Time  `json:"lockedAt"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	Extended      int        `json:"extendedCount"` // 延期次数
}

// AuditEntry 审计日志条目
type AuditEntry struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Action    AuditAction `json:"action"`
	Actor     string      `json:"actor"`
	Target    string      `json:"target,omitempty"`
	Details   string      `json:"details,omitempty"`
	SourceIP  string      `json:"sourceIp,omitempty"`
	Success   bool        `json:"success"`
}

// ReportRequest 报告生成请求
type ReportRequest struct {
	ReportType  string `json:"reportType" binding:"required"` // monthly|quarterly
	Year        int    `json:"year" binding:"required"`
	Quarter     int    `json:"quarter,omitempty"`  // 1-4, quarterly时必需
	Month       int    `json:"month,omitempty"`    // 1-12, monthly时必需
	GeneratedBy string `json:"generatedBy"`
}

// Dashboard 合规仪表盘引擎
type Dashboard struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	policies map[string]*WORMPolicy
	alerts   map[string]*AnomalyAlert
	retention map[string]*RetentionEntry
	reports  []*ComplianceReport
	auditLog []*AuditEntry
	nextID   int64
}

// NewDashboard 创建仪表盘引擎
func NewDashboard() *Dashboard {
	return &Dashboard{
		policies:  make(map[string]*WORMPolicy),
		alerts:    make(map[string]*AnomalyAlert),
		retention: make(map[string]*RetentionEntry),
		reports:   make([]*ComplianceReport, 0),
		auditLog:  make([]*AuditEntry, 0),
	}
}
