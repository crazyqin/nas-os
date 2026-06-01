// Package digitallegacy 提供数字遗产管理功能，支持遗产计划、受益人、资产分配、紧急访问等。
package digitallegacy

import (
	"time"
)

// LegacyStatus 遗产计划状态
type LegacyStatus string

const (
	LegacyStatusDraft     LegacyStatus = "draft"     // 草稿
	LegacyStatusActive    LegacyStatus = "active"    // 生效中
	LegacyStatusTriggered LegacyStatus = "triggered" // 已触发
	LegacyStatusCompleted LegacyStatus = "completed" // 已完成
	LegacyStatusRevoked   LegacyStatus = "revoked"   // 已撤销
)

// TriggerType 触发类型
type TriggerType string

const (
	TriggerManual     TriggerType = "manual"      // 手动触发
	TriggerInactivity TriggerType = "inactivity"  // 不活跃触发
	TriggerDeathCert  TriggerType = "death_cert"  // 死亡证明触发
	TriggerEmergency  TriggerType = "emergency"   // 紧急触发
	TriggerScheduled  TriggerType = "scheduled"   // 定时触发
)

// AssetType 数字资产类型
type AssetType string

const (
	AssetTypeAccount  AssetType = "account"  // 在线账号
	AssetTypeFile     AssetType = "file"     // 文件
	AssetTypePassword AssetType = "password" // 密码
	AssetTypeCrypto   AssetType = "crypto"   // 加密货币钱包
	AssetTypeDomain   AssetType = "domain"   // 域名
	AssetTypeSocial   AssetType = "social"   // 社交媒体
	AssetTypeEmail    AssetType = "email"    // 邮箱
	AssetTypeOther    AssetType = "other"    // 其他
)

// ContactRole 联系人角色
type ContactRole string

const (
	RoleBeneficiary ContactRole = "beneficiary" // 受益人
	RoleWitness     ContactRole = "witness"     // 见证人
	RoleExecutor    ContactRole = "executor"    // 执行人
	RoleEmergency   ContactRole = "emergency"   // 紧急联系人
)

// VerificationMethod 验证方式
type VerificationMethod string

const (
	VerifyEmail     VerificationMethod = "email"      // 邮箱验证
	VerifyPhone     VerificationMethod = "phone"      // 手机验证
	VerifyIDCard    VerificationMethod = "id_card"    // 身份证验证
	VerifyDeathCert VerificationMethod = "death_cert" // 死亡证明
	VerifyNotary    VerificationMethod = "notary"     // 公证处验证
)

// AccessLevel 访问级别
type AccessLevel string

const (
	AccessFull    AccessLevel = "full"    // 完全访问
	AccessRead    AccessLevel = "read"    // 只读访问
	AccessLimited AccessLevel = "limited" // 有限访问
	AccessNone    AccessLevel = "none"    // 无访问
)

// HeartbeatStatus 心跳状态
type HeartbeatStatus string

const (
	HeartbeatAlive     HeartbeatStatus = "alive"     // 存活
	HeartbeatMissing   HeartbeatStatus = "missing"   // 缺失
	HeartbeatConfirmed HeartbeatStatus = "confirmed" // 已确认死亡
)

// VerificationLevel 验证级别
type VerificationLevel int

const (
	VerifyLevelPrimary   VerificationLevel = 1 // 主要联系人验证
	VerifyLevelSecondary VerificationLevel = 2 // 次要联系人验证
	VerifyLevelTertiary  VerificationLevel = 3 // 第三方验证（如公证处）
)

// LegacyPlan 遗产计划
type LegacyPlan struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Description       string              `json:"description,omitempty"`
	OwnerID           string              `json:"owner_id"`
	Status            LegacyStatus        `json:"status"`
	TriggerType       TriggerType         `json:"trigger_type"`
	TriggerConditions *TriggerConditions  `json:"trigger_conditions,omitempty"`
	Beneficiaries     []*Beneficiary      `json:"beneficiaries,omitempty"`
	Assets            []*DigitalAsset     `json:"assets,omitempty"`
	EmergencyContacts []*EmergencyContact `json:"emergency_contacts,omitempty"`
	WillDocument      *WillDocument       `json:"will_document,omitempty"`
	TimeLock          *TimeLock           `json:"time_lock,omitempty"`
	IsEncrypted       bool                `json:"is_encrypted"`
	EncryptionKeyHash string              `json:"encryption_key_hash,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
	TriggeredAt       *time.Time          `json:"triggered_at,omitempty"`
	CompletedAt       *time.Time          `json:"completed_at,omitempty"`
}

// TriggerConditions 触发条件
type TriggerConditions struct {
	InactivityDays        int        `json:"inactivity_days,omitempty"`
	LastActiveAt          *time.Time `json:"last_active_at,omitempty"`
	RequiredWitnesses     int        `json:"required_witnesses,omitempty"`
	RequiredVerifications int        `json:"required_verifications,omitempty"`
	ScheduledDate         *time.Time `json:"scheduled_date,omitempty"`
	EmergencyCode         string     `json:"emergency_code,omitempty"`
	GracePeriodDays       int        `json:"grace_period_days,omitempty"`
}

// TimeLock 时间锁
type TimeLock struct {
	ID            string     `json:"id"`
	PlanID        string     `json:"plan_id"`
	UnlockAt      time.Time  `json:"unlock_at"`                // 解锁时间
	IsActive      bool       `json:"is_active"`
	UnlockedAt    *time.Time `json:"unlocked_at,omitempty"`
	RequiredLevel int        `json:"required_level"`           // 所需验证级别
	CreatedAt     time.Time  `json:"created_at"`
}

// Beneficiary 受益人
type Beneficiary struct {
	ID                string      `json:"id"`
	PlanID            string      `json:"plan_id"`
	ContactID         string      `json:"contact_id"`
	Role              ContactRole `json:"role"`
	Name              string      `json:"name"`
	Email             string      `json:"email,omitempty"`
	Phone             string      `json:"phone,omitempty"`
	Relationship      string      `json:"relationship,omitempty"`
	AllocationPercent int         `json:"allocation_percent"`
	AccessLevel       AccessLevel `json:"access_level"`
	IsVerified        bool        `json:"is_verified"`
	VerifiedAt        *time.Time  `json:"verified_at,omitempty"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

// DigitalAsset 数字资产
type DigitalAsset struct {
	ID            string    `json:"id"`
	PlanID        string    `json:"plan_id"`
	Name          string    `json:"name"`
	Type          AssetType `json:"type"`
	Description   string    `json:"description,omitempty"`
	Value         string    `json:"value,omitempty"`
	EncryptedData string    `json:"encrypted_data,omitempty"`
	DataHash      string    `json:"data_hash,omitempty"`
	StoragePath   string    `json:"storage_path,omitempty"`
	AccessURL     string    `json:"access_url,omitempty"`
	Username      string    `json:"username,omitempty"`
	Notes         string    `json:"notes,omitempty"`
	IsEncrypted   bool      `json:"is_encrypted"`
	AssignedTo    []string  `json:"assigned_to,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// EmergencyContact 紧急联系人
type EmergencyContact struct {
	ID              string      `json:"id"`
	PlanID          string      `json:"plan_id"`
	Name            string      `json:"name"`
	Email           string      `json:"email,omitempty"`
	Phone           string      `json:"phone,omitempty"`
	Relationship    string      `json:"relationship,omitempty"`
	Role            ContactRole `json:"role"`
	Level           int         `json:"level"` // 验证级别 1-3
	IsPrimary       bool        `json:"is_primary"`
	CanTriggerPlan  bool        `json:"can_trigger_plan"`
	NotifyOnTrigger bool        `json:"notify_on_trigger"`
	IsVerified      bool        `json:"is_verified"`
	VerifiedAt      *time.Time  `json:"verified_at,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

// WillDocument 遗嘱文档
type WillDocument struct {
	ID               string     `json:"id"`
	PlanID           string     `json:"plan_id"`
	Title            string     `json:"title"`
	Content          string     `json:"content,omitempty"`
	EncryptedContent string     `json:"encrypted_content,omitempty"`
	IsEncrypted      bool       `json:"is_encrypted"`
	FileHash         string     `json:"file_hash,omitempty"`
	StoragePath      string     `json:"storage_path,omitempty"`
	NotarizedAt      *time.Time `json:"notarized_at,omitempty"`
	NotaryInfo       string     `json:"notary_info,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// TrustContact 信任联系人
type TrustContact struct {
	ID                 string             `json:"id"`
	OwnerID            string             `json:"owner_id"`
	Name               string             `json:"name"`
	Email              string             `json:"email,omitempty"`
	Phone              string             `json:"phone,omitempty"`
	Relationship       string             `json:"relationship,omitempty"`
	Role               ContactRole        `json:"role"`
	VerificationMethod VerificationMethod `json:"verification_method"`
	IsVerified         bool               `json:"is_verified"`
	VerifiedAt         *time.Time         `json:"verified_at,omitempty"`
	LastContactAt      *time.Time         `json:"last_contact_at,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

// VerificationRequest 验证请求
type VerificationRequest struct {
	ID         string             `json:"id"`
	PlanID     string             `json:"plan_id"`
	ContactID  string             `json:"contact_id"`
	Method     VerificationMethod `json:"method"`
	Level      VerificationLevel  `json:"level"`
	Status     string             `json:"status"` // pending, verified, failed
	Code       string             `json:"code,omitempty"`
	ExpiresAt  time.Time          `json:"expires_at"`
	VerifiedAt *time.Time         `json:"verified_at,omitempty"`
	CreatedAt  time.Time          `json:"created_at"`
}

// AccessGrant 访问授权
type AccessGrant struct {
	ID            string      `json:"id"`
	PlanID        string      `json:"plan_id"`
	BeneficiaryID string      `json:"beneficiary_id"`
	AssetID       string      `json:"asset_id"`
	AccessLevel   AccessLevel `json:"access_level"`
	GrantedAt     time.Time   `json:"granted_at"`
	ExpiresAt     *time.Time  `json:"expires_at,omitempty"`
	RevokedAt     *time.Time  `json:"revoked_at,omitempty"`
	IsActive      bool        `json:"is_active"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        string    `json:"id"`
	PlanID    string    `json:"plan_id,omitempty"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Details   string    `json:"details,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// InactivityCheck 不活跃检测记录
type InactivityCheck struct {
	ID           string    `json:"id"`
	PlanID       string    `json:"plan_id"`
	OwnerID      string    `json:"owner_id"`
	LastActive   time.Time `json:"last_active"`
	DaysInactive int       `json:"days_inactive"`
	IsTriggered  bool      `json:"is_triggered"`
	CheckedAt    time.Time `json:"checked_at"`
}

// HeartbeatRecord 心跳记录
type HeartbeatRecord struct {
	ID        string          `json:"id"`
	OwnerID   string          `json:"owner_id"`
	Status    HeartbeatStatus `json:"status"`
	CheckedAt time.Time       `json:"checked_at"`
	ExpiresAt time.Time       `json:"expires_at"`
	Note      string          `json:"note,omitempty"`
}

// DeathVerification 死亡验证记录
type DeathVerification struct {
	ID                string             `json:"id"`
	PlanID            string             `json:"plan_id"`
	OwnerID           string             `json:"owner_id"`
	Status            string             `json:"status"` // pending, in_progress, confirmed, rejected
	VerificationLevel VerificationLevel  `json:"verification_level"`
	ConfirmerID       string             `json:"confirmer_id"`
	ConfirmerName     string             `json:"confirmer_name"`
	ConfirmerRelation string             `json:"confirmer_relation"`
	Method            VerificationMethod `json:"method"`
	Evidence          string             `json:"evidence,omitempty"`
	Notes             string             `json:"notes,omitempty"`
	ConfirmedAt       *time.Time         `json:"confirmed_at,omitempty"`
	RejectedAt        *time.Time         `json:"rejected_at,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

// LegacyPlanRequest 遗产计划请求
type LegacyPlanRequest struct {
	Name              string             `json:"name"`
	Description       string             `json:"description,omitempty"`
	TriggerType       TriggerType        `json:"trigger_type"`
	TriggerConditions *TriggerConditions `json:"trigger_conditions,omitempty"`
	IsEncrypted       bool               `json:"is_encrypted"`
}

// BeneficiaryRequest 受益人请求
type BeneficiaryRequest struct {
	Name              string      `json:"name"`
	Email             string      `json:"email,omitempty"`
	Phone             string      `json:"phone,omitempty"`
	Relationship      string      `json:"relationship,omitempty"`
	Role              ContactRole `json:"role"`
	AllocationPercent int         `json:"allocation_percent"`
	AccessLevel       AccessLevel `json:"access_level"`
}

// AssetRequest 资产请求
type AssetRequest struct {
	Name        string    `json:"name"`
	Type        AssetType `json:"type"`
	Description string    `json:"description,omitempty"`
	Value       string    `json:"value,omitempty"`
	Data        string    `json:"data,omitempty"`
	Notes       string    `json:"notes,omitempty"`
	AssignedTo  []string  `json:"assigned_to,omitempty"`
}

// EmergencyContactRequest 紧急联系人请求
type EmergencyContactRequest struct {
	Name            string      `json:"name"`
	Email           string      `json:"email,omitempty"`
	Phone           string      `json:"phone,omitempty"`
	Relationship    string      `json:"relationship,omitempty"`
	Role            ContactRole `json:"role"`
	Level           int         `json:"level"`
	IsPrimary       bool        `json:"is_primary"`
	CanTriggerPlan  bool        `json:"can_trigger_plan"`
	NotifyOnTrigger bool        `json:"notify_on_trigger"`
}

// WillDocumentRequest 遗嘱文档请求
type WillDocumentRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// TriggerRequest 触发请求
type TriggerRequest struct {
	PlanID           string `json:"plan_id"`
	EmergencyCode    string `json:"emergency_code,omitempty"`
	VerificationCode string `json:"verification_code,omitempty"`
}

// TimeLockRequest 时间锁请求
type TimeLockRequest struct {
	UnlockAt      time.Time `json:"unlock_at"`
	RequiredLevel int       `json:"required_level"`
}

// DeathVerificationRequest 死亡验证请求
type DeathVerificationRequest struct {
	ConfirmerID       string             `json:"confirmer_id"`
	ConfirmerName     string             `json:"confirmer_name"`
	ConfirmerRelation string             `json:"confirmer_relation"`
	Method            VerificationMethod `json:"method"`
	Evidence          string             `json:"evidence,omitempty"`
	Notes             string             `json:"notes,omitempty"`
}

// DefaultLegacyConfig 默认配置
type DefaultLegacyConfig struct {
	InactivityDays      int  `json:"inactivity_days"`
	GracePeriodDays     int  `json:"grace_period_days"`
	RequiredWitnesses   int  `json:"required_witnesses"`
	EnableEncryption    bool `json:"enable_encryption"`
	EnableAuditLog      bool `json:"enable_audit_log"`
	MaxBeneficiaries    int  `json:"max_beneficiaries"`
	MaxAssets           int  `json:"max_assets"`
	NotifyBeforeTrigger int  `json:"notify_before_trigger"`
	HeartbeatInterval   int  `json:"heartbeat_interval_hours"`
	HeartbeatTimeout    int  `json:"heartbeat_timeout_hours"`
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() *DefaultLegacyConfig {
	return &DefaultLegacyConfig{
		InactivityDays:      365,
		GracePeriodDays:     30,
		RequiredWitnesses:   2,
		EnableEncryption:    true,
		EnableAuditLog:      true,
		MaxBeneficiaries:    10,
		MaxAssets:           100,
		NotifyBeforeTrigger: 7,
		HeartbeatInterval:   24,
		HeartbeatTimeout:    168, // 7 days
	}
}

// IsValidTriggerType 检查触发类型是否有效
func IsValidTriggerType(t TriggerType) bool {
	switch t {
	case TriggerManual, TriggerInactivity, TriggerDeathCert, TriggerEmergency, TriggerScheduled:
		return true
	default:
		return false
	}
}

// IsValidAssetType 检查资产类型是否有效
func IsValidAssetType(t AssetType) bool {
	switch t {
	case AssetTypeAccount, AssetTypeFile, AssetTypePassword, AssetTypeCrypto,
		AssetTypeDomain, AssetTypeSocial, AssetTypeEmail, AssetTypeOther:
		return true
	default:
		return false
	}
}

// IsValidContactRole 检查联系人角色是否有效
func IsValidContactRole(r ContactRole) bool {
	switch r {
	case RoleBeneficiary, RoleWitness, RoleExecutor, RoleEmergency:
		return true
	default:
		return false
	}
}

// IsValidAccessLevel 检查访问级别是否有效
func IsValidAccessLevel(l AccessLevel) bool {
	switch l {
	case AccessFull, AccessRead, AccessLimited, AccessNone:
		return true
	default:
		return false
	}
}

// IsValidVerificationMethod 检查验证方式是否有效
func IsValidVerificationMethod(m VerificationMethod) bool {
	switch m {
	case VerifyEmail, VerifyPhone, VerifyIDCard, VerifyDeathCert, VerifyNotary:
		return true
	default:
		return false
	}
}

// IsValidVerificationLevel 检查验证级别是否有效
func IsValidVerificationLevel(l VerificationLevel) bool {
	switch l {
	case VerifyLevelPrimary, VerifyLevelSecondary, VerifyLevelTertiary:
		return true
	default:
		return false
	}
}
