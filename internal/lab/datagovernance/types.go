// Package datagovernance 实现数据主权管理引擎
// 支持数据分类标签、数据驻留合规、保留策略执行、审计追踪、合规报告生成、数据血缘追踪
// 参考群晖数据合规和 TrueNAS 审计能力
package datagovernance

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNotRunning 引擎未运行.
	ErrNotRunning = errors.New("data governance engine not running")
	// ErrAlreadyRunning 引擎已在运行.
	ErrAlreadyRunning = errors.New("data governance engine already running")
	// ErrPolicyNotFound 保留策略不存在.
	ErrPolicyNotFound = errors.New("retention policy not found")
	// ErrRecordNotFound 记录不存在.
	ErrRecordNotFound = errors.New("record not found")
	// ErrLineageNotFound 血缘记录不存在.
	ErrLineageNotFound = errors.New("lineage record not found")
	// ErrInvalidRegion 无效地理区域.
	ErrInvalidRegion = errors.New("invalid geographic region")
	// ErrResidencyViolation 数据驻留违规.
	ErrResidencyViolation = errors.New("data residency violation")
)

// ========== 数据敏感度分级 ==========

// SensitivityLevel 数据敏感度等级.
type SensitivityLevel string

const (
	// LevelPublic 公开数据.
	LevelPublic SensitivityLevel = "public"
	// LevelInternal 内部数据.
	LevelInternal SensitivityLevel = "internal"
	// LevelConfidential 机密数据.
	LevelConfidential SensitivityLevel = "confidential"
	// LevelTopSecret 绝密数据.
	LevelTopSecret SensitivityLevel = "top_secret"
)

// ========== 合规框架 ==========

// ComplianceFramework 合规框架.
type ComplianceFramework string

const (
	// FrameworkGDPR 欧盟通用数据保护条例.
	FrameworkGDPR ComplianceFramework = "GDPR"
	// FrameworkCCPA 加州消费者隐私法案.
	FrameworkCCPA ComplianceFramework = "CCPA"
	// FrameworkHIPAA 健康保险可携性和责任法案.
	FrameworkHIPAA ComplianceFramework = "HIPAA"
	// FrameworkPIPL 中国个人信息保护法.
	FrameworkPIPL ComplianceFramework = "PIPL"
	// FrameworkSOC2 服务组织控制报告.
	FrameworkSOC2 ComplianceFramework = "SOC2"
)

// ========== 审计操作类型 ==========

// AuditAction 审计操作类型.
type AuditAction string

const (
	// ActionCreate 创建.
	ActionCreate AuditAction = "create"
	// ActionRead 读取.
	ActionRead AuditAction = "read"
	// ActionUpdate 更新.
	ActionUpdate AuditAction = "update"
	// ActionDelete 删除.
	ActionDelete AuditAction = "delete"
	// ActionExport 导出.
	ActionExport AuditAction = "export"
	// ActionShare 分享.
	ActionShare AuditAction = "share"
	// ActionClassify 分类.
	ActionClassify AuditAction = "classify"
	// ActionDestroy 销毁.
	ActionDestroy AuditAction = "destroy"
)

// ========== 血缘关系类型 ==========

// LineageRelation 血缘关系类型.
type LineageRelation string

const (
	// RelationDerivedFrom 派生自.
	RelationDerivedFrom LineageRelation = "derived_from"
	// RelationCopiedFrom 复制自.
	RelationCopiedFrom LineageRelation = "copied_from"
	// RelationMergedFrom 合并自.
	RelationMergedFrom LineageRelation = "merged_from"
	// RelationTransformedFrom 转换自.
	RelationTransformedFrom LineageRelation = "transformed_from"
)

// ========== 保留策略动作 ==========

// RetentionAction 保留策略到期动作.
type RetentionAction string

const (
	// ActionArchive 归档.
	ActionArchive RetentionAction = "archive"
	// RetentionActionDestroy 永久销毁.
	RetentionActionDestroy RetentionAction = "destroy"
	// ActionNotify 通知管理员.
	ActionNotify RetentionAction = "notify"
	// ActionAnonymize 匿名化.
	ActionAnonymize RetentionAction = "anonymize"
)

// ========== 地理区域 ==========

// GeoRegion 地理区域.
type GeoRegion string

const (
	// RegionChina 中国大陆.
	RegionChina GeoRegion = "CN"
	// RegionUSEast 美国东部.
	RegionUSEast GeoRegion = "US_EAST"
	// RegionUSWest 美国西部.
	RegionUSWest GeoRegion = "US_WEST"
	// RegionEU 欧盟.
	RegionEU GeoRegion = "EU"
	// RegionAPAC 亚太.
	RegionAPAC GeoRegion = "APAC"
)

// ========== 核心配置 ==========

// Config 数据治理引擎配置.
type Config struct {
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// DefaultRegion 默认数据驻留区域.
	DefaultRegion GeoRegion `json:"defaultRegion"`
	// AllowedRegions 允许的存储区域.
	AllowedRegions []GeoRegion `json:"allowedRegions"`
	// AutoClassify 是否自动分类.
	AutoClassify bool `json:"autoClassify"`
	// ClassifyIntervalHours 分类扫描间隔（小时）.
	ClassifyIntervalHours int `json:"classifyIntervalHours"`
	// RetentionCheckIntervalHours 保留策略检查间隔（小时）.
	RetentionCheckIntervalHours int `json:"retentionCheckIntervalHours"`
	// AuditRetentionDays 审计日志保留天数.
	AuditRetentionDays int `json:"auditRetentionDays"`
	// ScanPaths 数据扫描路径.
	ScanPaths []string `json:"scanPaths"`
	// NotifyEmail 通知邮箱.
	NotifyEmail string `json:"notifyEmail"`
}

// ========== 数据资产 ==========

// DataAsset 数据资产记录.
type DataAsset struct {
	// ID 资产ID.
	ID string `json:"id"`
	// Name 名称.
	Name string `json:"name"`
	// FilePath 文件路径.
	FilePath string `json:"filePath"`
	// FileType 文件类型.
	FileType string `json:"fileType"`
	// SizeBytes 文件大小（字节）.
	SizeBytes int64 `json:"sizeBytes"`
	// Sensitivity 敏感度等级.
	Sensitivity SensitivityLevel `json:"sensitivity"`
	// Region 当前存储区域.
	Region GeoRegion `json:"region"`
	// OwnerID 所有者ID.
	OwnerID string `json:"ownerId"`
	// OwnerName 所有者名称.
	OwnerName string `json:"ownerName"`
	// Tags 标签.
	Tags []string `json:"tags"`
	// PolicyID 关联的保留策略ID.
	PolicyID string `json:"policyId,omitempty"`
	// RetentionDeadline 保留截止日期.
	RetentionDeadline *time.Time `json:"retentionDeadline,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updatedAt"`
	// ClassifiedBy 分类方式 (ai/manual/rule).
	ClassifiedBy string `json:"classifiedBy"`
}

// ========== 保留策略 ==========

// RetentionPolicy 数据保留策略.
type RetentionPolicy struct {
	// ID 策略ID.
	ID string `json:"id"`
	// Name 策略名称.
	Name string `json:"name"`
	// Description 描述.
	Description string `json:"description"`
	// SensitivityLevels 适用的敏感度等级.
	SensitivityLevels []SensitivityLevel `json:"sensitivityLevels"`
	// RetentionDays 保留天数.
	RetentionDays int `json:"retentionDays"`
	// ExpirationAction 到期动作.
	ExpirationAction RetentionAction `json:"expirationAction"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// ApplicableRegions 适用区域（空表示全部）.
	ApplicableRegions []GeoRegion `json:"applicableRegions,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updatedAt"`
}

// ========== 审计记录 ==========

// AuditRecord 审计追踪记录.
type AuditRecord struct {
	// ID 记录ID.
	ID string `json:"id"`
	// Timestamp 时间戳.
	Timestamp time.Time `json:"timestamp"`
	// UserID 操作用户ID.
	UserID string `json:"userId"`
	// UserName 操作用户名.
	UserName string `json:"userName"`
	// Action 操作类型.
	Action AuditAction `json:"action"`
	// AssetID 关联资产ID.
	AssetID string `json:"assetId"`
	// AssetName 关联资产名.
	AssetName string `json:"assetName"`
	// Details 操作详情.
	Details string `json:"details"`
	// IPAddress 来源IP.
	IPAddress string `json:"ipAddress"`
	// Region 操作发生区域.
	Region GeoRegion `json:"region"`
	// Result 操作结果 (success/failure/denied).
	Result string `json:"result"`
	// RiskLevel 风险等级 (low/medium/high/critical).
	RiskLevel string `json:"riskLevel"`
}

// ========== 数据血缘 ==========

// LineageRecord 数据血缘记录.
type LineageRecord struct {
	// ID 记录ID.
	ID string `json:"id"`
	// AssetID 目标资产ID.
	AssetID string `json:"assetId"`
	// AssetName 目标资产名.
	AssetName string `json:"assetName"`
	// SourceAssetID 源资产ID.
	SourceAssetID string `json:"sourceAssetId"`
	// SourceAssetName 源资产名.
	SourceAssetName string `json:"sourceAssetName"`
	// Relation 关系类型.
	Relation LineageRelation `json:"relation"`
	// Description 描述.
	Description string `json:"description"`
	// OperatorID 操作者ID.
	OperatorID string `json:"operatorId"`
	// OperatorName 操作者名.
	OperatorName string `json:"operatorName"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
	// Metadata 额外元数据.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ========== 合规报告 ==========

// ComplianceReport 数据治理合规报告.
type ComplianceReport struct {
	// ID 报告ID.
	ID string `json:"id"`
	// Framework 合规框架.
	Framework ComplianceFramework `json:"framework"`
	// OverallScore 总分 (0-100).
	OverallScore float64 `json:"overallScore"`
	// MaxScore 满分.
	MaxScore float64 `json:"maxScore"`
	// Status 合规状态.
	Status string `json:"status"`
	// TotalChecks 总检查数.
	TotalChecks int `json:"totalChecks"`
	// PassedChecks 通过数.
	PassedChecks int `json:"passedChecks"`
	// FailedChecks 失败数.
	FailedChecks int `json:"failedChecks"`
	// Findings 发现问题.
	Findings []ReportFinding `json:"findings"`
	// RegionCompliance 区域合规情况.
	RegionCompliance []RegionComplianceStatus `json:"regionCompliance"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// ValidUntil 有效期至.
	ValidUntil time.Time `json:"validUntil"`
	// GeneratedBy 生成者.
	GeneratedBy string `json:"generatedBy"`
}

// ReportFinding 报告中的发现项.
type ReportFinding struct {
	// ID 发现ID.
	ID string `json:"id"`
	// Category 分类.
	Category string `json:"category"`
	// Title 标题.
	Title string `json:"title"`
	// Description 描述.
	Description string `json:"description"`
	// Severity 严重程度.
	Severity string `json:"severity"`
	// AffectedAssets 受影响资产数.
	AffectedAssets int `json:"affectedAssets"`
	// Remediation 整改建议.
	Remediation string `json:"remediation"`
	// Status 状态 (open/remediated/accepted).
	Status string `json:"status"`
}

// RegionComplianceStatus 区域合规状态.
type RegionComplianceStatus struct {
	// Region 区域.
	Region GeoRegion `json:"region"`
	// TotalAssets 资产总数.
	TotalAssets int `json:"totalAssets"`
	// CompliantAssets 合规资产数.
	CompliantAssets int `json:"compliantAssets"`
	// Violations 违规数.
	Violations int `json:"violations"`
	// Score 分数.
	Score float64 `json:"score"`
}

// ========== 统计概览 ==========

// Stats 数据治理统计.
type Stats struct {
	// TotalAssets 资产总数.
	TotalAssets int `json:"totalAssets"`
	// Classifications 分类统计.
	Classifications map[SensitivityLevel]int `json:"classifications"`
	// RegionDistribution 区域分布.
	RegionDistribution map[GeoRegion]int `json:"regionDistribution"`
	// TotalPolicies 策略总数.
	TotalPolicies int `json:"totalPolicies"`
	// ActivePolicies 活跃策略数.
	ActivePolicies int `json:"activePolicies"`
	// TotalAuditRecords 审计记录总数.
	TotalAuditRecords int `json:"totalAuditRecords"`
	// TotalLineageRecords 血缘记录总数.
	TotalLineageRecords int `json:"totalLineageRecords"`
	// PendingDestructions 待销毁资产数.
	PendingDestructions int `json:"pendingDestructions"`
	// ResidencyViolations 驻留违规数.
	ResidencyViolations int `json:"residencyViolations"`
	// RecentAuditRecords 最近审计记录.
	RecentAuditRecords []AuditRecord `json:"recentAuditRecords"`
}
