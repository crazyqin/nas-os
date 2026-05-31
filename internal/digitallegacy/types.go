// Package digitallegacy 提供数字遗产管理功能，支持遗产计划、受益人、资产分配、紧急访问等。
// 参考群晖数字遗产功能设计，确保数字资产在用户意外情况下能够安全传递。
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
	TriggerManual        TriggerType = "manual"         // 手动触发
	TriggerInactivity    TriggerType = "inactivity"     // 不活跃触发
	TriggerDeathCert     TriggerType = "death_cert"     // 死亡证明触发
	TriggerEmergency     TriggerType = "emergency"      // 紧急触发
	TriggerScheduled     TriggerType = "scheduled"      // 定时触发
)

// AssetType 数字资产类型
type AssetType string

const (
	AssetTypeAccount   AssetType = "account"   // 在线账号
	AssetTypeFile      AssetType = "file"      // 文件
	AssetTypePassword  AssetType = "password"  // 密码
	AssetTypeCrypto    AssetType = "crypto"    // 加密货币
	AssetTypeDomain    AssetType = "domain"    // 域名
	AssetTypeSocial    AssetType = "social"    // 社交媒体
	AssetTypeEmail     AssetType = "email"     // 邮箱
	AssetTypeOther     AssetType = "other"     // 其他
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
	VerifyEmail     VerificationMethod = "email"     // 邮箱验证
	VerifyPhone     VerificationMethod = "phone"     // 手机验证
	VerifyIDCard    VerificationMethod = "id_card"   // 身份证验证
	VerifyDeathCert VerificationMethod = "death_cert" // 死亡证明
	VerifyNotary    VerificationMethod = "notary"    // 公证处验证
)

// AccessLevel 访问级别
type AccessLevel string

const (
	AccessFull    AccessLevel = "full"    // 完全访问
	AccessRead    AccessLevel = "read"    // 只读访问
	AccessLimited AccessLevel = "limited" // 有限访问
	AccessNone    AccessLevel = "none"    // 无访问
)

// LegacyPlan 遗产计划
type LegacyPlan struct {
	ID                string              `json:"id"`
	Name              string              `json:"name" binding:"required"`
	Description       string              `json:"description,omitempty"`
	OwnerID           string              `json:"owner_id"`
	Status            LegacyStatus        `json:"status"`
	TriggerType       TriggerType         `json:"trigger_type"`
	TriggerConditions *TriggerConditions  `json:"trigger_conditions,omitempty"`
	Beneficiaries     []*Beneficiary      `json:"beneficiaries,omitempty"`
	Assets            []*DigitalAsset     `json:"assets,omitempty"`
	EmergencyContacts []*EmergencyContact `json:"emergency_contacts,omitempty"`
	WillDocument      *WillDocument       `json:"will_document,omitempty"`
	IsEncrypted       bool                `json:"is_encrypted"`
	EncryptionKeyHash string              `json:"encryption_key_hash,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
	TriggeredAt       *time.Time          `json:"triggered_at,omitempty"`
	CompletedAt       *time.Time          `json:"completed_at,omitempty"`
}

// TriggerConditions 触发条件
type TriggerConditions struct {
	InactivityDays     int        `json:"inactivity_days,omitempty"`     // 不活跃天数
	LastActiveAt       *time.Time `json:"last_active_at,omitempty"`      // 最后活跃时间
	RequiredWitnesses  int        `json:"required_witnesses,omitempty"`  // 所需见证人数
	RequiredVerifications int     `json:"required_verifications,omitempty"` // 所需验证数
	ScheduledDate      *time.Time `json:"scheduled_date,omitempty"`      // 定时触发日期
	EmergencyCode      string     `json:"emergency_code,omitempty"`      // 紧急代码
	GracePeriodDays    int        `json:"grace_period_days,omitempty"`   // 宽限期天数
}

// Beneficiary 受益人
type Beneficiary struct {
	ID               string          `json:"id"`
	PlanID           string          `json:"plan_id"`
	ContactID        string          `json:"contact_id"`
	Role             ContactRole     `json:"role"`
	Name             string          `json:"name" binding:"required"`
	Email            string          `json:"email,omitempty"`
	Phone            string          `json:"phone,omitempty"`
	Relationship     string          `json:"relationship,omitempty"`
	AllocationPercent int            `json:"allocation_percent"` // 分配比例 0-100
	AccessLevel      AccessLevel     `json:"access_level"`
	IsVerified       bool            `json:"is_verified"`
	VerifiedAt       *time.Time      `json:"verified_at,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// DigitalAsset 数字资产
type DigitalAsset struct {
	ID              string      `json:"id"`
	PlanID          string      `json:"plan_id"`
	Name            string      `json:"name" binding:"required"`
	Type            AssetType   `json:"type" binding:"required"`
	Description     string      `json:"description,omitempty"`
	Value           string      `json:"value,omitempty"`           // 资产价值描述
	EncryptedData   string      `json:"encrypted_data,omitempty"`  // 加密的敏感数据
	DataHash        string      `json:"data_hash,omitempty"`       // 数据哈希
	StoragePath     string      `json:"storage_path,omitempty"`    // 文件存储路径
	AccessURL       string      `json:"access_url,omitempty"`      // 访问链接
	Username        string      `json:"username,omitempty"`        // 账号用户名
	Notes           string      `json:"notes,omitempty"`           // 备注
	IsEncrypted     bool        `json:"is_encrypted"`
	AssignedTo      []string    `json:"assigned_to,omitempty"`     // 分配给的受益人ID列表
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

// EmergencyContact 紧急联系人
type EmergencyContact struct {
	ID              string              `json:"id"`
	PlanID          string              `json:"plan_id"`
	Name            string              `json:"name" binding:"required"`
	Email           string              `json:"email,omitempty"`
	Phone           string              `json:"phone,omitempty"`
	Relationship    string              `json:"relationship,omitempty"`
	Role            ContactRole         `json:"role"`
	IsPrimary       bool                `json:"is_primary"`
	CanTriggerPlan  bool                `json:"can_trigger_plan"`  // 是否可以触发计划
	NotifyOnTrigger bool                `json:"notify_on_trigger"` // 触发时是否通知
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

// WillDocument 遗嘱文档
type WillDocument struct {
	ID              string     `json:"id"`
	PlanID          string     `json:"plan_id"`
	Title           string     `json:"title" binding:"required"`
	Content         string     `json:"content"`                    // 遗嘱内容
	EncryptedContent string    `json:"encrypted_content,omitempty"` // 加密内容
	IsEncrypted     bool       `json:"is_encrypted"`
	FileHash        string     `json:"file_hash,omitempty"`       // 文件哈希
	StoragePath     string     `json:"storage_path,omitempty"`    // 存储路径
	NotarizedAt     *time.Time `json:"notarized_at,omitempty"`    // 公证时间
	NotaryInfo      string     `json:"notary_info,omitempty"`     // 公证信息
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TrustContact 信任联系人
type TrustContact struct {
	ID              string          `json:"id"`
	OwnerID         string          `json:"owner_id"`
	Name            string          `json:"name" binding:"required"`
	Email           string          `json:"email,omitempty"`
	Phone           string          `json:"phone,omitempty"`
	Relationship    string          `json:"relationship,omitempty"`
	Role            ContactRole     `json:"role"`
	VerificationMethod VerificationMethod `json:"verification_method"`
	IsVerified      bool            `json:"is_verified"`
	VerifiedAt      *time.Time      `json:"verified_at,omitempty"`
	LastContactAt   *time.Time      `json:"last_contact_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// VerificationRequest 验证请求
type VerificationRequest struct {
	ID              string              `json:"id"`
	PlanID          string              `json:"plan_id"`
	ContactID       string              `json:"contact_id"`
	Method          VerificationMethod  `json:"method"`
	Status          string              `json:"status"` // pending, verified, failed
	Code            string              `json:"code,omitempty"`
	ExpiresAt       time.Time           `json:"expires_at"`
	VerifiedAt      *time.Time          `json:"verified_at,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
}

// AccessGrant 访问授权
type AccessGrant struct {
	ID              string        `json:"id"`
	PlanID          string        `json:"plan_id"`
	BeneficiaryID   string        `json:"beneficiary_id"`
	AssetID         string        `json:"asset_id"`
	AccessLevel     AccessLevel   `json:"access_level"`
	GrantedAt       time.Time     `json:"granted_at"`
	ExpiresAt       *time.Time    `json:"expires_at,omitempty"`
	RevokedAt       *time.Time    `json:"revoked_at,omitempty"`
	IsActive        bool          `json:"is_active"`
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
	ID          string    `json:"id"`
	PlanID      string    `json:"plan_id"`
	OwnerID     string    `json:"owner_id"`
	LastActive  time.Time `json:"last_active"`
	DaysInactive int      `json:"days_inactive"`
	IsTriggered bool      `json:"is_triggered"`
	CheckedAt   time.Time `json:"checked_at"`
}

// LegacyPlanRequest 遗产计划请求
type LegacyPlanRequest struct {
	Name              string              `json:"name" binding:"required"`
	Description       string              `json:"description,omitempty"`
	TriggerType       TriggerType         `json:"trigger_type" binding:"required"`
	TriggerConditions *TriggerConditions  `json:"trigger_conditions,omitempty"`
	IsEncrypted       bool                `json:"is_encrypted"`
}

// BeneficiaryRequest 受益人请求
type BeneficiaryRequest struct {
	Name              string      `json:"name" binding:"required"`
	Email             string      `json:"email,omitempty"`
	Phone             string      `json:"phone,omitempty"`
	Relationship      string      `json:"relationship,omitempty"`
	Role              ContactRole `json:"role" binding:"required"`
	AllocationPercent int         `json:"allocation_percent"`
	AccessLevel       AccessLevel `json:"access_level"`
}

// AssetRequest 资产请求
type AssetRequest struct {
	Name        string    `json:"name" binding:"required"`
	Type        AssetType `json:"type" binding:"required"`
	Description string    `json:"description,omitempty"`
	Value       string    `json:"value,omitempty"`
	Data        string    `json:"data,omitempty"` // 敏感数据，将被加密
	Notes       string    `json:"notes,omitempty"`
	AssignedTo  []string  `json:"assigned_to,omitempty"`
}

// EmergencyContactRequest 紧急联系人请求
type EmergencyContactRequest struct {
	Name            string      `json:"name" binding:"required"`
	Email           string      `json:"email,omitempty"`
	Phone           string      `json:"phone,omitempty"`
	Relationship    string      `json:"relationship,omitempty"`
	Role            ContactRole `json:"role" binding:"required"`
	IsPrimary       bool        `json:"is_primary"`
	CanTriggerPlan  bool        `json:"can_trigger_plan"`
	NotifyOnTrigger bool        `json:"notify_on_trigger"`
}

// WillDocumentRequest 遗嘱文档请求
type WillDocumentRequest struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// TriggerRequest 触发请求
type TriggerRequest struct {
	PlanID        string `json:"plan_id" binding:"required"`
	EmergencyCode string `json:"emergency_code,omitempty"`
	VerificationCode string `json:"verification_code,omitempty"`
}

// DefaultLegacyConfig 默认配置
type DefaultLegacyConfig struct {
	InactivityDays      int  `json:"inactivity_days"`       // 默认不活跃天数
	GracePeriodDays     int  `json:"grace_period_days"`     // 默认宽限期
	RequiredWitnesses   int  `json:"required_witnesses"`    // 默认所需见证人数
	EnableEncryption    bool `json:"enable_encryption"`     // 默认启用加密
	EnableAuditLog      bool `json:"enable_audit_log"`      // 启用审计日志
	MaxBeneficiaries    int  `json:"max_beneficiaries"`     // 最大受益人数
	MaxAssets           int  `json:"max_assets"`            // 最大资产数
	NotifyBeforeTrigger int  `json:"notify_before_trigger"` // 触发前通知天数
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
