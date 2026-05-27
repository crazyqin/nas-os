// Package datagovernance 提供数据治理管理功能
package datagovernance

import (
	"fmt"
	"time"
)

// DataClassification 数据分类等级.
type DataClassification string

const (
	ClassPublic       DataClassification = "public"       // 公开
	ClassInternal     DataClassification = "internal"     // 内部
	ClassConfidential DataClassification = "confidential" // 机密
	ClassTopSecret    DataClassification = "top_secret"   // 绝密
)

// ComplianceStandard 合规标准.
type ComplianceStandard string

const (
	StandardGDPR        ComplianceStandard = "gdpr"         // 欧盟通用数据保护条例
	StandardDSL         ComplianceStandard = "dsl"          // 中华人民共和国数据安全法
	StandardMLPS        ComplianceStandard = "mlps"         // 等保 2.0
	StandardPIPL        ComplianceStandard = "pipl"         // 个人信息保护法
	StandardHIPAA       ComplianceStandard = "hipaa"        // 健康保险可携性与责任法案
)

// LifecycleStage 数据生命周期阶段.
type LifecycleStage string

const (
	StageCreated  LifecycleStage = "created"  // 创建
	StageActive   LifecycleStage = "active"   // 使用中
	StageArchived LifecycleStage = "archived" // 已归档
	StageDestroyed LifecycleStage = "destroyed" // 已销毁
)

// SensitiveDataType 敏感数据类型.
type SensitiveDataType string

const (
	SensitiveIDCard    SensitiveDataType = "id_card"    // 身份证号
	SensitivePhone     SensitiveDataType = "phone"      // 手机号
	SensitiveBankCard  SensitiveDataType = "bank_card"  // 银行卡号
	SensitiveEmail     SensitiveDataType = "email"      // 邮箱
	SensitivePassport  SensitiveDataType = "passport"   // 护照号
)

// RiskLevel 风险等级.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// ComplianceStatus 合规状态.
type ComplianceStatus string

const (
	StatusCompliant    ComplianceStatus = "compliant"     // 合规
	StatusNonCompliant ComplianceStatus = "non_compliant" // 不合规
	StatusPending      ComplianceStatus = "pending"       // 待审查
)

// EncryptionStatus 加密状态.
type EncryptionStatus string

const (
	EncryptedNone     EncryptionStatus = "none"      // 未加密
	EncryptedAtRest   EncryptionStatus = "at_rest"   // 静态加密
	EncryptedInTransit EncryptionStatus = "in_transit" // 传输加密
	EncryptedBoth     EncryptionStatus = "both"      // 全加密
)

// AuditAction 审计动作类型.
type AuditAction string

const (
	ActionCreate    AuditAction = "create"
	ActionRead      AuditAction = "read"
	ActionUpdate    AuditAction = "update"
	ActionDelete    AuditAction = "delete"
	ActionExport    AuditAction = "export"
	ActionShare     AuditAction = "share"
	ActionClassify  AuditAction = "classify"
	ActionEncrypt   AuditAction = "encrypt"
	ActionArchive   AuditAction = "archive"
	ActionDestroy   AuditAction = "destroy"
)

// DataTag 数据分类标签.
type DataTag struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Level       DataClassification `json:"level"`
	Description string             `json:"description"`
	Color       string             `json:"color"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// RetentionPolicy 数据保留策略.
type RetentionPolicy struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	DataType       string             `json:"data_type"`       // 适用数据类型
	Classification DataClassification `json:"classification"`  // 适用分类等级
	RetentionDays  int                `json:"retention_days"`  // 保留天数
	AutoArchive    bool               `json:"auto_archive"`    // 自动归档
	AutoDestroy    bool               `json:"auto_destroy"`    // 自动销毁
	IsActive       bool               `json:"is_active"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// ComplianceCheck 合规检查结果.
type ComplianceCheck struct {
	ID             string             `json:"id"`
	Standard       ComplianceStandard `json:"standard"`
	CheckName      string             `json:"check_name"`
	Status         ComplianceStatus   `json:"status"`
	RiskLevel      RiskLevel          `json:"risk_level"`
	Description    string             `json:"description"`
	Findings       []string           `json:"findings,omitempty"`
	Recommendation string             `json:"recommendation,omitempty"`
	CheckedAt      time.Time          `json:"checked_at"`
}

// AuditLog 审计日志.
type AuditLog struct {
	ID         string      `json:"id"`
	UserID     string      `json:"user_id"`
	UserName   string      `json:"user_name"`
	Action     AuditAction `json:"action"`
	Resource   string      `json:"resource"`
	ResourceID string      `json:"resource_id"`
	Details    string      `json:"details,omitempty"`
	IPAddress  string      `json:"ip_address,omitempty"`
	Success    bool        `json:"success"`
	Timestamp  time.Time   `json:"timestamp"`
}

// SensitiveDataFinding 敏感数据发现.
type SensitiveDataFinding struct {
	ID        string           `json:"id"`
	FilePath  string           `json:"file_path"`
	Type      SensitiveDataType `json:"type"`
	Value     string           `json:"value"`     // 脱敏后的值
	Count     int              `json:"count"`     // 发现数量
	RiskLevel RiskLevel        `json:"risk_level"`
	ScannedAt time.Time        `json:"scanned_at"`
}

// AccessControlPolicy 数据访问控制策略.
type AccessControlPolicy struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Classification DataClassification `json:"classification"`
	AllowedRoles   []string           `json:"allowed_roles"`
	AllowedUsers   []string           `json:"allowed_users,omitempty"`
	DeniedUsers    []string           `json:"denied_users,omitempty"`
	RequireMFA     bool               `json:"require_mfa"`
	MaxAccessCount int                `json:"max_access_count"` // 0=不限制
	ExpireAt       *time.Time         `json:"expire_at,omitempty"`
	IsActive       bool               `json:"is_active"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// DataLifecycleRecord 数据生命周期记录.
type DataLifecycleRecord struct {
	ID             string          `json:"id"`
	ResourcePath   string          `json:"resource_path"`
	ResourceType   string          `json:"resource_type"`
	CurrentStage   LifecycleStage  `json:"current_stage"`
	Classification DataClassification `json:"classification"`
	OwnerID        string          `json:"owner_id"`
	CreatedAt      time.Time       `json:"created_at"`
	StageHistory   []StageEvent    `json:"stage_history"`
	ExpireAt       *time.Time      `json:"expire_at,omitempty"`
}

// StageEvent 生命周期阶段事件.
type StageEvent struct {
	Stage     LifecycleStage `json:"stage"`
	Timestamp time.Time      `json:"timestamp"`
	UserID    string         `json:"user_id,omitempty"`
	Reason    string         `json:"reason,omitempty"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	ID               string             `json:"id"`
	Standard         ComplianceStandard `json:"standard"`
	Status           ComplianceStatus   `json:"status"`
	Score            int                `json:"score"` // 0-100
	TotalChecks      int                `json:"total_checks"`
	Passed           int                `json:"passed"`
	Failed           int                `json:"failed"`
	Warnings         int                `json:"warnings"`
	Checks           []ComplianceCheck  `json:"checks"`
	Summary          string             `json:"summary"`
	Recommendations  []string           `json:"recommendations,omitempty"`
	GeneratedAt      time.Time          `json:"generated_at"`
}

// DataLeakRiskAssessment 数据泄露风险评估.
type DataLeakRiskAssessment struct {
	ID                  string                  `json:"id"`
	OverallRisk         RiskLevel               `json:"overall_risk"`
	RiskScore           int                     `json:"risk_score"` // 0-100
	Categories          []RiskCategory          `json:"categories"`
	SensitiveDataCount  int                     `json:"sensitive_data_count"`
	UnencryptedCount    int                     `json:"unencrypted_count"`
	ExpiredRetention    int                     `json:"expired_retention"`
	WeakAccessCount     int                     `json:"weak_access_count"`
	Recommendations     []string                `json:"recommendations"`
	AssessedAt          time.Time               `json:"assessed_at"`
}

// RiskCategory 风险类别.
type RiskCategory struct {
	Category    string    `json:"category"`
	RiskLevel   RiskLevel `json:"risk_level"`
	Score       int       `json:"score"`
	Description string    `json:"description"`
	Count       int       `json:"count"`
}

// EncryptionCheckResult 加密状态检查结果.
type EncryptionCheckResult struct {
	ResourcePath    string           `json:"resource_path"`
	Status          EncryptionStatus `json:"status"`
	Algorithm       string           `json:"algorithm,omitempty"`
	KeyID           string           `json:"key_id,omitempty"`
	Compliant       bool             `json:"compliant"`
	Issues          []string         `json:"issues,omitempty"`
	CheckedAt       time.Time        `json:"checked_at"`
}

// ========== 请求/响应类型 ==========

// CreateTagRequest 创建标签请求.
type CreateTagRequest struct {
	Name        string             `json:"name" binding:"required"`
	Level       DataClassification `json:"level" binding:"required"`
	Description string             `json:"description,omitempty"`
	Color       string             `json:"color,omitempty"`
}

// CreateRetentionPolicyRequest 创建保留策略请求.
type CreateRetentionPolicyRequest struct {
	Name           string             `json:"name" binding:"required"`
	DataType       string             `json:"data_type" binding:"required"`
	Classification DataClassification `json:"classification" binding:"required"`
	RetentionDays  int                `json:"retention_days" binding:"required,min=1"`
	AutoArchive    bool               `json:"auto_archive,omitempty"`
	AutoDestroy    bool               `json:"auto_destroy,omitempty"`
}

// ScanRequest 敏感数据扫描请求.
type ScanRequest struct {
	Paths []string           `json:"paths" binding:"required"`
	Types []SensitiveDataType `json:"types,omitempty"` // 为空则扫描所有类型
}

// ComplianceCheckRequest 合规检查请求.
type ComplianceCheckRequest struct {
	Standard ComplianceStandard `json:"standard" binding:"required"`
}

// AuditLogQuery 审计日志查询参数.
type AuditLogQuery struct {
	UserID    string     `form:"user_id,omitempty"`
	Action    AuditAction `form:"action,omitempty"`
	Resource  string     `form:"resource,omitempty"`
	StartTime *time.Time `form:"start_time,omitempty"`
	EndTime   *time.Time `form:"end_time,omitempty"`
	Limit     int        `form:"limit,omitempty"`
	Offset    int        `form:"offset,omitempty"`
}

// CreateAccessPolicyRequest 创建访问控制策略请求.
type CreateAccessPolicyRequest struct {
	Name           string             `json:"name" binding:"required"`
	Description    string             `json:"description,omitempty"`
	Classification DataClassification `json:"classification" binding:"required"`
	AllowedRoles   []string           `json:"allowed_roles" binding:"required"`
	AllowedUsers   []string           `json:"allowed_users,omitempty"`
	DeniedUsers    []string           `json:"denied_users,omitempty"`
	RequireMFA     bool               `json:"require_mfa,omitempty"`
	MaxAccessCount int                `json:"max_access_count,omitempty"`
	ExpireAt       *time.Time         `json:"expire_at,omitempty"`
}

// CreateLifecycleRequest 创建生命周期记录请求.
type CreateLifecycleRequest struct {
	ResourcePath   string             `json:"resource_path" binding:"required"`
	ResourceType   string             `json:"resource_type" binding:"required"`
	Classification DataClassification `json:"classification" binding:"required"`
	OwnerID        string             `json:"owner_id" binding:"required"`
}

// TransitionLifecycleRequest 生命周期阶段转换请求.
type TransitionLifecycleRequest struct {
	Stage  LifecycleStage `json:"stage" binding:"required"`
	UserID string         `json:"user_id" binding:"required"`
	Reason string         `json:"reason,omitempty"`
}

// GenerateID 生成唯一 ID.
func GenerateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// APIResponse 通用 API 响应.
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func okResp(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

func errResp(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}
