// Package datalifecycle 提供数据生命周期管理功能
// 支持数据从创建→使用→归档→销毁的全生命周期管理
// 包含自动数据迁移、保留策略、合规保留和数据销毁认证
package datalifecycle

import (
	"time"
)

// ==================== 生命周期阶段 ====================

// LifecyclePhase 数据生命周期阶段.
type LifecyclePhase string

const (
	// PhaseActive 活跃阶段：频繁访问.
	PhaseActive LifecyclePhase = "active"
	// PhaseReference 参考阶段：偶尔访问.
	PhaseReference LifecyclePhase = "reference"
	// PhaseArchive 归档阶段：极少访问.
	PhaseArchive LifecyclePhase = "archive"
	// PhaseRetained 保留阶段：受保留策略保护.
	PhaseRetained LifecyclePhase = "retained"
	// PhaseExpired 过期阶段：已超过保留期限.
	PhaseExpired LifecyclePhase = "expired"
	// PhasePendingDestruction 待销毁阶段：等待销毁确认.
	PhasePendingDestruction LifecyclePhase = "pending_destruction"
	// PhaseDestroyed 已销毁阶段：数据已被安全销毁.
	PhaseDestroyed LifecyclePhase = "destroyed"
)

// PhaseOrder 阶段顺序，用于验证转换合法性.
var PhaseOrder = map[LifecyclePhase]int{
	PhaseActive:             1,
	PhaseReference:          2,
	PhaseArchive:            3,
	PhaseRetained:           4,
	PhaseExpired:            5,
	PhasePendingDestruction: 6,
	PhaseDestroyed:          7,
}

// ==================== 数据分类 ====================

// DataClassification 数据分类.
type DataClassification string

const (
	// ClassificationPublic 公开数据.
	ClassificationPublic DataClassification = "public"
	// ClassificationInternal 内部数据.
	ClassificationInternal DataClassification = "internal"
	// ClassificationConfidential 机密数据.
	ClassificationConfidential DataClassification = "confidential"
	// ClassificationRestricted 受限数据（最高保密级别）.
	ClassificationRestricted DataClassification = "restricted"
)

// ==================== 策略类型 ====================

// PolicyType 策略类型.
type PolicyType string

const (
	// PolicyTypeRetention 保留策略：控制数据保留时长.
	PolicyTypeRetention PolicyType = "retention"
	// PolicyTypeTiering 分层策略：控制数据迁移.
	PolicyTypeTiering PolicyType = "tiering"
	// PolicyTypeArchival 归档策略：控制归档行为.
	PolicyTypeArchival PolicyType = "archival"
	// PolicyTypeDestruction 销毁策略：控制数据销毁.
	PolicyTypeDestruction PolicyType = "destruction"
	// PolicyTypeCompliance 合规策略：满足法规要求.
	PolicyTypeCompliance PolicyType = "compliance"
)

// ==================== 保留类型 ====================

// RetentionType 保留类型.
type RetentionType string

const (
	// RetentionTypeTime 基于时间的保留.
	RetentionTypeTime RetentionType = "time"
	// RetentionTypeVersion 基于版本数量的保留.
	RetentionTypeVersion RetentionType = "version"
	// RetentionTypeSpace 基于空间限制的保留.
	RetentionTypeSpace RetentionType = "space"
	// RetentionTypeLegal 法律保留.
	RetentionTypeLegal RetentionType = "legal"
	// RetentionTypeAudit 审计保留.
	RetentionTypeAudit RetentionType = "audit"
)

// ==================== 存储层 ====================

// StorageTier 存储层.
type StorageTier string

const (
	// TierHot 热存储：高性能，高成本.
	TierHot StorageTier = "hot"
	// TierWarm 温存储：中等性能，中等成本.
	TierWarm StorageTier = "warm"
	// TierCold 冷存储：低性能，低成本.
	TierCold StorageTier = "cold"
	// TierArchive 归档存储：最低成本，检索延迟高.
	TierArchive StorageTier = "archive"
)

// ==================== 迁移和销毁状态 ====================

// MigrationStatus 迁移状态.
type MigrationStatus string

const (
	// MigrationPending 迁移待执行.
	MigrationPending MigrationStatus = "pending"
	// MigrationRunning 迁移执行中.
	MigrationRunning MigrationStatus = "running"
	// MigrationCompleted 迁移已完成.
	MigrationCompleted MigrationStatus = "completed"
	// MigrationFailed 迁移失败.
	MigrationFailed MigrationStatus = "failed"
	// MigrationCancelled 迁移已取消.
	MigrationCancelled MigrationStatus = "cancelled"
)

// DestructionStatus 销毁状态.
type DestructionStatus string

const (
	// DestructionPending 销毁待确认.
	DestructionPending DestructionStatus = "pending"
	// DestructionApproved 销毁已批准.
	DestructionApproved DestructionStatus = "approved"
	// DestructionInProgress 销毁进行中.
	DestructionInProgress DestructionStatus = "in_progress"
	// DestructionCompleted 销毁已完成.
	DestructionCompleted DestructionStatus = "completed"
	// DestructionRejected 销毁被拒绝.
	DestructionRejected DestructionStatus = "rejected"
)

// ==================== 核心数据结构 ====================

// LifecyclePolicy 生命周期策略定义.
type LifecyclePolicy struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Description     string               `json:"description,omitempty"`
	Enabled         bool                 `json:"enabled"`
	Priority        int                  `json:"priority"`        // 越大优先级越高
	Type            PolicyType           `json:"type"`            // 策略类型
	Classifications []DataClassification `json:"classifications"` // 适用的数据分类
	Tags            []string             `json:"tags,omitempty"`  // 策略标签
	CreatedAt       time.Time            `json:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
	CreatedBy       string               `json:"createdBy,omitempty"`

	// 生命周期阶段定义
	Phases []PhaseDefinition `json:"phases"` // 生命周期各阶段定义

	// 保留规则
	Retention RetentionPolicy `json:"retention"` // 保留策略

	// 分层迁移规则
	TieringRules []TieringRule `json:"tieringRules,omitempty"` // 分层迁移规则

	// 销毁规则
	Destruction DestructionPolicy `json:"destruction"` // 销毁策略

	// 合规要求
	ComplianceRequirements []ComplianceRequirement `json:"complianceRequirements,omitempty"` // 合规要求

	// 适用范围
	PathPatterns    []string `json:"pathPatterns,omitempty"`    // 路径匹配模式
	ExcludePatterns []string `json:"excludePatterns,omitempty"` // 排除模式
}

// PhaseDefinition 生命周期阶段定义.
type PhaseDefinition struct {
	Phase      LifecyclePhase `json:"phase"`      // 阶段名称
	Duration   time.Duration  `json:"duration"`   // 阶段持续时间（0表示无限）
	Conditions []Condition    `json:"conditions"` // 进入该阶段的条件
	Actions    []Action       `json:"actions"`    // 进入该阶段时执行的动作
}

// Condition 触发条件.
type Condition struct {
	Type     string `json:"type"`     // access_count, age, size, last_access
	Operator string `json:"operator"` // gt, lt, gte, lte, eq
	Value    string `json:"value"`    // 条件值
}

// Action 自动动作.
type Action struct {
	Type       string            `json:"type"`       // move, archive, compress, encrypt, notify, delete
	TargetTier StorageTier       `json:"targetTier"` // 目标存储层（移动/归档时使用）
	Params     map[string]string `json:"params"`     // 动作参数
}

// RetentionPolicy 保留策略.
type RetentionPolicy struct {
	Type        RetentionType `json:"type"`        // 保留类型
	Duration    time.Duration `json:"duration"`    // 保留时长（时间保留）
	MaxVersions int           `json:"maxVersions"` // 最大版本数（版本保留）
	MaxSize     int64         `json:"maxSize"`     // 最大空间（空间保留，字节）
	AutoDelete  bool          `json:"autoDelete"`  // 过期后自动删除
}

// TieringRule 分层迁移规则.
type TieringRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	SourceTier  StorageTier   `json:"sourceTier"`  // 源存储层
	TargetTier  StorageTier   `json:"targetTier"`  // 目标存储层
	Conditions  []Condition   `json:"conditions"`  // 触发条件
	MinAge      time.Duration `json:"minAge"`      // 最小文件年龄
	MaxAccess   int64         `json:"maxAccess"`   // 最大访问次数
	FilePattern string        `json:"filePattern"` // 文件匹配模式
	Enabled     bool          `json:"enabled"`
}

// DestructionPolicy 销毁策略.
type DestructionPolicy struct {
	Method            DestructionMethod `json:"method"`            // 销毁方法
	RequireApproval   bool              `json:"requireApproval"`   // 需要审批
	Approvers         []string          `json:"approvers"`         // 审批人列表
	ApprovalTimeout   time.Duration     `json:"approvalTimeout"`   // 审批超时
	CertificationPath string            `json:"certificationPath"` // 销毁证书存储路径
	RetainAuditLog    bool              `json:"retainAuditLog"`    // 保留审计日志
}

// DestructionMethod 销毁方法.
type DestructionMethod string

const (
	// MethodSecureDelete 安全删除（多次覆写）.
	MethodSecureDelete DestructionMethod = "secure_delete"
	// MethodCryptoErase 密码学擦除（销毁加密密钥）.
	MethodCryptoErase DestructionMethod = "crypto_erase"
	// MethodPhysicalDestroy 物理销毁.
	MethodPhysicalDestroy DestructionMethod = "physical_destroy"
	// MethodSoftDelete 软删除（移至回收站）.
	MethodSoftDelete DestructionMethod = "soft_delete"
)

// ComplianceRequirement 合规要求.
type ComplianceRequirement struct {
	Name          string        `json:"name"`          // 要求名称
	Regulation    string        `json:"regulation"`    // 法规名称（如 GDPR, HIPAA, SOX）
	MinRetention  time.Duration `json:"minRetention"`  // 最短保留期
	MaxRetention  time.Duration `json:"maxRetention"`  // 最长保留期
	Immutable     bool          `json:"immutable"`     // 不可变（禁止删除/修改）
	AuditRequired bool          `json:"auditRequired"` // 需要审计日志
}

// ==================== 数据记录 ====================

// DataRecord 数据记录，跟踪数据的完整生命周期.
type DataRecord struct {
	ID             string             `json:"id"`
	Path           string             `json:"path"`
	Name           string             `json:"name"`
	Size           int64              `json:"size"`
	CreatedAt      time.Time          `json:"createdAt"`
	ModifiedAt     time.Time          `json:"modifiedAt"`
	LastAccessedAt time.Time          `json:"lastAccessedAt"`
	AccessCount    int64              `json:"accessCount"`
	CurrentPhase   LifecyclePhase     `json:"currentPhase"`
	CurrentTier    StorageTier        `json:"currentTier"`
	Classification DataClassification `json:"classification"`
	Tags           []string           `json:"tags,omitempty"`
	PolicyID       string             `json:"policyId"`          // 关联的策略ID
	HoldIDs        []string           `json:"holdIds,omitempty"` // 关联的保留ID
	Version        int                `json:"version"`
	TotalVersions  int                `json:"totalVersions"`
	IsEncrypted    bool               `json:"isEncrypted"`
	PhaseHistory   []PhaseTransition  `json:"phaseHistory,omitempty"` // 阶段变更历史
}

// PhaseTransition 阶段转换记录.
type PhaseTransition struct {
	FromPhase LifecyclePhase `json:"fromPhase"`
	ToPhase   LifecyclePhase `json:"toPhase"`
	Timestamp time.Time      `json:"timestamp"`
	Reason    string         `json:"reason"`
	PolicyID  string         `json:"policyId,omitempty"`
}

// ==================== 保留和合规 ====================

// ComplianceHold 合规保留（法律保留/审计保留）.
type ComplianceHold struct {
	ID          string        `json:"id"`
	Type        RetentionType `json:"type"` // legal 或 audit
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	FilePaths   []string      `json:"filePaths"`  // 被保留的文件路径（支持通配符）
	CaseNumber  string        `json:"caseNumber"` // 案件/审计编号
	IssuedBy    string        `json:"issuedBy"`   // 发起人
	Regulation  string        `json:"regulation"` // 关联法规
	ExpiresAt   *time.Time    `json:"expiresAt"`  // 过期时间，nil表示手动解除
	Active      bool          `json:"active"`
	CreatedAt   time.Time     `json:"createdAt"`
	ReleasedAt  *time.Time    `json:"releasedAt"` // 解除时间
	ReleasedBy  string        `json:"releasedBy,omitempty"`
}

// ==================== 迁移 ====================

// DataMigration 数据迁移任务.
type DataMigration struct {
	ID          string          `json:"id"`
	PolicyID    string          `json:"policyId,omitempty"`
	RuleID      string          `json:"ruleId,omitempty"`
	Status      MigrationStatus `json:"status"`
	CreatedAt   time.Time       `json:"createdAt"`
	StartedAt   time.Time       `json:"startedAt,omitempty"`
	CompletedAt time.Time       `json:"completedAt,omitempty"`

	// 迁移配置
	SourceTier StorageTier `json:"sourceTier"`
	TargetTier StorageTier `json:"targetTier"`
	SourcePath string      `json:"sourcePath"`
	TargetPath string      `json:"targetPath"`

	// 文件列表
	Files []MigrationFile `json:"files,omitempty"`

	// 进度
	TotalFiles     int64 `json:"totalFiles"`
	TotalBytes     int64 `json:"totalBytes"`
	ProcessedFiles int64 `json:"processedFiles"`
	ProcessedBytes int64 `json:"processedBytes"`
	FailedFiles    int64 `json:"failedFiles"`

	// 错误信息
	Errors []MigrationError `json:"errors,omitempty"`

	// 试运行模式
	DryRun bool `json:"dryRun"`
}

// MigrationFile 迁移文件.
type MigrationFile struct {
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
	Size       int64  `json:"size"`
	Status     string `json:"status"` // pending, completed, failed
	Error      string `json:"error,omitempty"`
}

// MigrationError 迁移错误.
type MigrationError struct {
	Path    string    `json:"path"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// ==================== 销毁 ====================

// DestructionRecord 数据销毁记录.
type DestructionRecord struct {
	ID          string            `json:"id"`
	Status      DestructionStatus `json:"status"`
	CreatedAt   time.Time         `json:"createdAt"`
	ApprovedAt  *time.Time        `json:"approvedAt,omitempty"`
	CompletedAt *time.Time        `json:"completedAt,omitempty"`
	ApprovedBy  string            `json:"approvedBy,omitempty"`

	// 销毁详情
	FilePaths     []string          `json:"filePaths"`     // 待销毁文件
	Method        DestructionMethod `json:"method"`        // 销毁方法
	TotalSize     int64             `json:"totalSize"`     // 总大小
	DestroyedSize int64             `json:"destroyedSize"` // 已销毁大小

	// 合规保留
	HoldID string `json:"holdId,omitempty"` // 关联的合规保留ID

	// 审批
	RequiresApproval bool          `json:"requiresApproval"`
	Approvers        []string      `json:"approvers,omitempty"`
	ApprovalTimeout  time.Duration `json:"approvalTimeout"`

	// 销毁证书
	Certification *DestructionCertification `json:"certification,omitempty"`
}

// DestructionCertification 销毁证书.
type DestructionCertification struct {
	ID            string            `json:"id"`
	DestructionID string            `json:"destructionId"`
	IssuedAt      time.Time         `json:"issuedAt"`
	Method        DestructionMethod `json:"method"`
	FileCount     int               `json:"fileCount"`
	TotalSize     int64             `json:"totalSize"`
	VerifiedBy    string            `json:"verifiedBy"`
	Signature     string            `json:"signature"`  // 数字签名
	HashBefore    string            `json:"hashBefore"` // 销毁前哈希
	HashAfter     string            `json:"hashAfter"`  // 销毁后验证哈希
}

// ==================== 访问分析 ====================

// AccessPattern 访问模式分析.
type AccessPattern struct {
	Path              string         `json:"path"`
	AccessCount       int64          `json:"accessCount"`
	LastAccess        time.Time      `json:"lastAccess"`
	AvgAccessInterval time.Duration  `json:"avgAccessInterval"` // 平均访问间隔
	CurrentTier       StorageTier    `json:"currentTier"`
	RecommendedTier   StorageTier    `json:"recommendedTier"` // 建议的存储层
	Phase             LifecyclePhase `json:"phase"`
	Score             float64        `json:"score"` // 活跃度评分 (0-100)
}

// TierSuggestion 分层建议.
type TierSuggestion struct {
	Path            string      `json:"path"`
	CurrentTier     StorageTier `json:"currentTier"`
	RecommendedTier StorageTier `json:"recommendedTier"`
	Reason          string      `json:"reason"`
	EstimatedSaving int64       `json:"estimatedSaving"` // 预估节省空间/成本
	Priority        int         `json:"priority"`        // 建议优先级
}

// AccessAnalysisReport 访问分析报告.
type AccessAnalysisReport struct {
	GeneratedAt time.Time                       `json:"generatedAt"`
	TotalFiles  int                             `json:"totalFiles"`
	TotalSize   int64                           `json:"totalSize"`
	Analysis    []AccessPattern                 `json:"analysis"`
	Suggestions []TierSuggestion                `json:"suggestions"`
	TierStats   map[StorageTier]*TierStatistics `json:"tierStats"`
	PhaseStats  map[LifecyclePhase]int          `json:"phaseStats"`
}

// TierStatistics 存储层统计.
type TierStatistics struct {
	Tier      StorageTier   `json:"tier"`
	FileCount int           `json:"fileCount"`
	TotalSize int64         `json:"totalSize"`
	AvgAge    time.Duration `json:"avgAge"`
	AvgAccess float64       `json:"avgAccess"`
}

// ==================== 模板和批量操作 ====================

// PolicyTemplate 策略模板.
type PolicyTemplate struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"` // general, healthcare, finance, legal, govt
	Policy      LifecyclePolicy `json:"policy"`   // 模板中的策略定义
	IsSystem    bool            `json:"isSystem"` // 系统预置模板
	CreatedAt   time.Time       `json:"createdAt"`
}

// BatchApplyRequest 批量应用请求.
type BatchApplyRequest struct {
	PolicyID string   `json:"policyId"` // 策略ID
	Paths    []string `json:"paths"`    // 适用路径
	Tags     []string `json:"tags"`     // 按标签筛选
	Force    bool     `json:"force"`    // 强制覆盖已有策略
}

// BatchApplyResult 批量应用结果.
type BatchApplyResult struct {
	TotalFiles   int      `json:"totalFiles"`
	AppliedFiles int      `json:"appliedFiles"`
	SkippedFiles int      `json:"skippedFiles"`
	FailedFiles  int      `json:"failedFiles"`
	Errors       []string `json:"errors,omitempty"`
}

// ==================== 审计和状态 ====================

// LifecycleAuditEntry 生命周期审计日志条目.
type LifecycleAuditEntry struct {
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Action    string         `json:"action"` // create_policy, update_policy, apply_policy, phase_change, migrate, destroy, hold_create, hold_release
	Target    string         `json:"target"` // 受影响的文件或策略ID
	Details   string         `json:"details"`
	Operator  string         `json:"operator"` // 操作人
	Success   bool           `json:"success"`
	PolicyID  string         `json:"policyId,omitempty"`
	Phase     LifecyclePhase `json:"phase,omitempty"`
}

// LifecycleStatus 生命周期模块状态.
type LifecycleStatus struct {
	Enabled             bool                   `json:"enabled"`
	TotalPolicies       int                    `json:"totalPolicies"`
	ActivePolicies      int                    `json:"activePolicies"`
	TotalRecords        int                    `json:"totalRecords"`
	ActiveHolds         int                    `json:"activeHolds"`
	RunningMigrations   int                    `json:"runningMigrations"`
	PendingDestructions int                    `json:"pendingDestructions"`
	PhaseDistribution   map[LifecyclePhase]int `json:"phaseDistribution"`
	TierDistribution    map[StorageTier]int    `json:"tierDistribution"`
}

// ==================== 通用响应 ====================

// Response 通用API响应.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse 错误响应.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
