package wormcompliance

import (
	"time"
)

// ComplianceMode 合规模式
type ComplianceMode string

const (
	// ModeGovernance 治理模式 - 允许特权用户删除
	ModeGovernance ComplianceMode = "governance"
	// ModeEnterprise 企业模式 - 仅允许到期后自动删除
	ModeEnterprise ComplianceMode = "enterprise"
	// ModeRegulatory 法规模式 - 完全不可删除，仅到期归档
	ModeRegulatory ComplianceMode = "regulatory"
)

// RetentionUnit 保留期单位
type RetentionUnit string

const (
	RetentionDays   RetentionUnit = "days"
	RetentionMonths RetentionUnit = "months"
	RetentionYears  RetentionUnit = "years"
	RetentionForever RetentionUnit = "forever"
)

// RegulationType 法规类型
type RegulationType string

const (
	RegulationGDPR RegulationType = "GDPR"
	RegulationSOX  RegulationType = "SOX"
	RegulationHIPAA RegulationType = "HIPAA"
)

// Policy WORM 合规策略
type Policy struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Mode            ComplianceMode `json:"mode"`
	RetentionPeriod RetentionPeriod `json:"retention_period"`
	Enabled         bool           `json:"enabled"`
	ApplyToPaths    []string       `json:"apply_to_paths"`
	Regulations     []RegulationType `json:"regulations,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// RetentionPeriod 数据保留期
type RetentionPeriod struct {
	Value int64        `json:"value"`
	Unit  RetentionUnit `json:"unit"`
}

// GetDuration 获取保留期时长
func (rp RetentionPeriod) GetDuration() time.Duration {
	switch rp.Unit {
	case RetentionDays:
		return time.Duration(rp.Value) * 24 * time.Hour
	case RetentionMonths:
		return time.Duration(rp.Value) * 30 * 24 * time.Hour
	case RetentionYears:
		return time.Duration(rp.Value) * 365 * 24 * time.Hour
	case RetentionForever:
		return time.Duration(100 * 365 * 24 * time.Hour) // 100年作为"永久"
	default:
		return 0
	}
}

// IsForever 是否永久保留
func (rp RetentionPeriod) IsForever() bool {
	return rp.Unit == RetentionForever
}

// ProtectedObject 受保护对象
type ProtectedObject struct {
	ID            string         `json:"id"`
	Path          string         `json:"path"`
	Hash          string         `json:"hash"` // SHA-256
	HashChainPrev string         `json:"hash_chain_prev,omitempty"`
	Size          int64          `json:"size"`
	PolicyID      string         `json:"policy_id"`
	Locked        bool           `json:"locked"`
	LockedAt      *time.Time     `json:"locked_at,omitempty"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	CreatedBy     string         `json:"created_by"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// IsExpired 检查对象是否已过保留期
func (po *ProtectedObject) IsExpired(now time.Time) bool {
	if po.ExpiresAt == nil {
		return false // 永久保留
	}
	return now.After(*po.ExpiresAt)
}

// AuditEntry 审计条目
type AuditEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	ObjectID  string    `json:"object_id"`
	Action    string    `json:"action"`
	Actor     string    `json:"actor"`
	Details   string    `json:"details"`
	PrevHash  string    `json:"prev_hash"` // 上一条审计记录的哈希
	Hash      string    `json:"hash"`      // 本条记录的哈希
	IPAddress string    `json:"ip_address,omitempty"`
	Success   bool      `json:"success"`
	Reason    string    `json:"reason,omitempty"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID             string                `json:"id"`
	GeneratedAt    time.Time             `json:"generated_at"`
	RegulationType RegulationType        `json:"regulation_type"`
	Status         ComplianceStatus      `json:"status"`
	Summary        ReportSummary         `json:"summary"`
	Violations     []ComplianceViolation `json:"violations,omitempty"`
	Recommendations []string            `json:"recommendations,omitempty"`
}

// ComplianceStatus 合规状态
type ComplianceStatus string

const (
	StatusCompliant    ComplianceStatus = "compliant"
	StatusNonCompliant ComplianceStatus = "non_compliant"
	StatusWarning      ComplianceStatus = "warning"
)

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalObjects     int   `json:"total_objects"`
	ProtectedObjects int   `json:"protected_objects"`
	ExpiredObjects   int   `json:"expired_objects"`
	TotalPolicies    int   `json:"total_policies"`
	ActivePolicies   int   `json:"active_policies"`
	TotalAuditLogs   int   `json:"total_audit_logs"`
	StorageUsedBytes int64 `json:"storage_used_bytes"`
}

// ComplianceViolation 合规违规
type ComplianceViolation struct {
	ObjectID    string         `json:"object_id"`
	Path        string         `json:"path"`
	ViolationType string       `json:"violation_type"`
	Severity    string         `json:"severity"`
	Description string         `json:"description"`
	DetectedAt  time.Time      `json:"detected_at"`
}

// WORMConfig WORM 配置
type WORMConfig struct {
	// HashChainSeed 哈希链种子
	HashChainSeed string `json:"hash_chain_seed"`
	// EnableAuditLog 启用审计日志
	EnableAuditLog bool `json:"enable_audit_log"`
	// MaxAuditRetentionDays 审计日志最大保留天数
	MaxAuditRetentionDays int `json:"max_audit_retention_days"`
	// DefaultMode 默认合规模式
	DefaultMode ComplianceMode `json:"default_mode"`
	// EnableTamperDetection 启用篡改检测
	EnableTamperDetection bool `json:"enable_tamper_detection"`
}

// DefaultWORMConfig 默认配置
func DefaultWORMConfig() WORMConfig {
	return WORMConfig{
		HashChainSeed:         "nas-os-worm-seed-2024",
		EnableAuditLog:        true,
		MaxAuditRetentionDays: 365,
		DefaultMode:           ModeGovernance,
		EnableTamperDetection: true,
	}
}
