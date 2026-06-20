// Package security 提供数据安全功能
// 文件加密、密钥管理、审计日志、合规报告
package security

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrKeyNotFound 密钥未找到
	ErrKeyNotFound = errors.New("密钥未找到")
	// ErrEncryptionFailed 加密失败
	ErrEncryptionFailed = errors.New("加密失败")
	// ErrDecryptionFailed 解密失败
	ErrDecryptionFailed = errors.New("解密失败")
	// ErrInvalidPassword 无效密码
	ErrInvalidPassword = errors.New("无效密码")
	// ErrAuditNotFound 审计记录未找到
	ErrAuditNotFound = errors.New("审计记录未找到")
)

// ========== 加密算法 ==========

// EncryptionAlgorithm 加密算法
type EncryptionAlgorithm string

const (
	AlgoAES256GCM EncryptionAlgorithm = "aes-256-gcm"
	AlgoAES256CBC EncryptionAlgorithm = "aes-256-cbc"
	AlgoChaCha20  EncryptionAlgorithm = "chacha20-poly1305"
	AlgoXChaCha20 EncryptionAlgorithm = "xchacha20-poly1305"
)

// ========== 密钥管理 ==========

// KeyType 密钥类型
type KeyType string

const (
	KeyTypeMaster  KeyType = "master"  // 主密钥
	KeyTypeData    KeyType = "data"    // 数据密钥
	KeyTypeBackup  KeyType = "backup"  // 备份密钥
	KeyTypeRecovery KeyType = "recovery" // 恢复密钥
)

// Key 密钥信息
type Key struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Type        KeyType             `json:"type"`
	Algorithm   EncryptionAlgorithm `json:"algorithm"`
	KeySize     int                 `json:"key_size"` // bits
	Version     int                 `json:"version"`
	Status      KeyStatus           `json:"status"`
	CreatedAt   time.Time           `json:"created_at"`
	ExpiresAt   *time.Time          `json:"expires_at,omitempty"`
	RotatedAt   *time.Time          `json:"rotated_at,omitempty"`
	LastUsedAt  *time.Time          `json:"last_used_at,omitempty"`
	Description string              `json:"description,omitempty"`
}

// KeyStatus 密钥状态
type KeyStatus string

const (
	KeyStatusActive   KeyStatus = "active"
	KeyStatusRotating KeyStatus = "rotating"
	KeyStatusExpired  KeyStatus = "expired"
	KeyStatusRevoked  KeyStatus = "revoked"
)

// KeyCreateRequest 创建密钥请求
type KeyCreateRequest struct {
	Name        string              `json:"name"`
	Type        KeyType             `json:"type"`
	Algorithm   EncryptionAlgorithm `json:"algorithm"`
	KeySize     int                 `json:"key_size"`
	ExpiresIn   int                 `json:"expires_in"` // days, 0=永不过期
	Description string              `json:"description,omitempty"`
}

// KeyRotateRequest 密钥轮换请求
type KeyRotateRequest struct {
	KeyID       string `json:"key_id"`
	ReEncrypt   bool   `json:"re_encrypt"` // 是否重新加密现有数据
}

// ========== 文件加密 ==========

// EncryptedFile 加密文件信息
type EncryptedFile struct {
	ID            string              `json:"id"`
	OriginalPath  string              `json:"original_path"`
	EncryptedPath string              `json:"encrypted_path"`
	KeyID         string              `json:"key_id"`
	Algorithm     EncryptionAlgorithm `json:"algorithm"`
	OriginalSize  int64               `json:"original_size"`
	EncryptedSize int64               `json:"encrypted_size"`
	Checksum      string              `json:"checksum"`      // 原始文件校验和
	IV            string              `json:"iv"`            // 初始化向量
	Salt          string              `json:"salt,omitempty"` // 盐值
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

// EncryptRequest 加密请求
type EncryptRequest struct {
	FilePath  string              `json:"file_path"`
	KeyID     string              `json:"key_id"`
	Algorithm EncryptionAlgorithm `json:"algorithm"`
	DeleteOriginal bool           `json:"delete_original"`
}

// DecryptRequest 解密请求
type DecryptRequest struct {
	EncryptedFileID string `json:"encrypted_file_id"`
	OutputPath      string `json:"output_path,omitempty"`
	DeleteEncrypted bool   `json:"delete_encrypted"`
}

// ========== 审计日志 ==========

// AuditAction 审计动作
type AuditAction string

const (
	ActionLogin         AuditAction = "login"
	ActionLogout        AuditAction = "logout"
	ActionFileRead      AuditAction = "file_read"
	ActionFileWrite     AuditAction = "file_write"
	ActionFileDelete    AuditAction = "file_delete"
	ActionFileShare     AuditAction = "file_share"
	ActionEncrypt       AuditAction = "encrypt"
	ActionDecrypt       AuditAction = "decrypt"
	ActionKeyCreate     AuditAction = "key_create"
	ActionKeyRotate     AuditAction = "key_rotate"
	ActionKeyRevoke     AuditAction = "key_revoke"
	ActionConfigChange  AuditAction = "config_change"
	ActionUserCreate    AuditAction = "user_create"
	ActionUserDelete    AuditAction = "user_delete"
	ActionPermissionChange AuditAction = "permission_change"
)

// AuditLog 审计日志
type AuditLog struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	UserID    string      `json:"user_id"`
	Username  string      `json:"username"`
	IP        string      `json:"ip"`
	Action    AuditAction `json:"action"`
	Resource  string      `json:"resource"`
	Details   string      `json:"details,omitempty"`
	Status    string      `json:"status"` // success, failure
	Error     string      `json:"error,omitempty"`
	UserAgent string      `json:"user_agent,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
}

// AuditSearchRequest 审计搜索请求
type AuditSearchRequest struct {
	UserID    string       `json:"user_id,omitempty"`
	Action    AuditAction  `json:"action,omitempty"`
	Resource  string       `json:"resource,omitempty"`
	Status    string       `json:"status,omitempty"`
	StartTime *time.Time   `json:"start_time,omitempty"`
	EndTime   *time.Time   `json:"end_time,omitempty"`
	IP        string       `json:"ip,omitempty"`
	Page      int          `json:"page"`
	PageSize  int          `json:"page_size"`
}

// AuditSearchResult 审计搜索结果
type AuditSearchResult struct {
	Logs       []*AuditLog `json:"logs"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// ========== 合规报告 ==========

// ComplianceStandard 合规标准
type ComplianceStandard string

const (
	StandardGDPR   ComplianceStandard = "gdpr"
	StandardHIPAA  ComplianceStandard = "hipaa"
	StandardSOC2   ComplianceStandard = "soc2"
	StandardISO27001 ComplianceStandard = "iso27001"
	StandardCustom ComplianceStandard = "custom"
)

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID          string             `json:"id"`
	Standard    ComplianceStandard `json:"standard"`
	GeneratedAt time.Time          `json:"generated_at"`
	PeriodStart time.Time          `json:"period_start"`
	PeriodEnd   time.Time          `json:"period_end"`
	Score       float64            `json:"score"` // 0-100
	Status      string             `json:"status"` // compliant, non_compliant, partial
	Summary     string             `json:"summary"`
	Sections    []ReportSection    `json:"sections"`
	Issues      []ComplianceIssue  `json:"issues"`
	Recommendations []string       `json:"recommendations"`
}

// ReportSection 报告章节
type ReportSection struct {
	Title    string  `json:"title"`
	Score    float64 `json:"score"`
	Status   string  `json:"status"`
	Details  string  `json:"details"`
}

// ComplianceIssue 合规问题
type ComplianceIssue struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"` // critical, high, medium, low
	Title       string `json:"title"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}

// ========== 统计 ==========

// SecurityStats 安全统计
type SecurityStats struct {
	TotalKeys       int     `json:"total_keys"`
	ActiveKeys      int     `json:"active_keys"`
	ExpiredKeys     int     `json:"expired_keys"`
	EncryptedFiles  int     `json:"encrypted_files"`
	TotalEncryptedSize int64 `json:"total_encrypted_size"`
	TotalAuditLogs  int     `json:"total_audit_logs"`
	FailedLogins    int     `json:"failed_logins"`
	LastKeyRotation *time.Time `json:"last_key_rotation,omitempty"`
	ComplianceScore float64 `json:"compliance_score"`
}

// ========== 配置 ==========

// SecurityConfig 安全配置
type SecurityConfig struct {
	// 加密配置
	DefaultAlgorithm EncryptionAlgorithm `json:"default_algorithm"`
	DefaultKeySize   int                 `json:"default_key_size"`
	KeyRotationDays  int                 `json:"key_rotation_days"`

	// 审计配置
	AuditEnabled     bool `json:"audit_enabled"`
	AuditRetentionDays int `json:"audit_retention_days"`

	// 密码策略
	MinPasswordLength  int  `json:"min_password_length"`
	RequireUppercase   bool `json:"require_uppercase"`
	RequireLowercase   bool `json:"require_lowercase"`
	RequireNumbers     bool `json:"require_numbers"`
	RequireSpecial     bool `json:"require_special"`
	PasswordExpiration int  `json:"password_expiration"` // days

	// 会话配置
	SessionTimeout     int  `json:"session_timeout"` // minutes
	MaxLoginAttempts   int  `json:"max_login_attempts"`
	LockoutDuration    int  `json:"lockout_duration"` // minutes
}

// DefaultSecurityConfig 默认配置
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		DefaultAlgorithm:   AlgoAES256GCM,
		DefaultKeySize:     256,
		KeyRotationDays:    90,
		AuditEnabled:       true,
		AuditRetentionDays: 365,
		MinPasswordLength:  12,
		RequireUppercase:   true,
		RequireLowercase:   true,
		RequireNumbers:     true,
		RequireSpecial:     true,
		PasswordExpiration: 90,
		SessionTimeout:     30,
		MaxLoginAttempts:   5,
		LockoutDuration:    15,
	}
}
