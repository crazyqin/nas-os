// Package filelifecycle 提供智能文件生命周期管理功能
// 基于文件访问频率、类型、大小自动进行数据分层
// 支持热/温/冷/归档四级策略，自动迁移冷数据到低成本存储
// 包含合规保留策略和完整的审计日志
package filelifecycle

import (
	"time"
)

// ==================== 存储层级 ====================

// FileTier 文件存储层级.
type FileTier string

const (
	// TierHot 热存储：高频访问，高性能介质.
	TierHot FileTier = "hot"
	// TierWarm 温存储：中频访问，中等性能介质.
	TierWarm FileTier = "warm"
	// TierCold 冷存储：低频访问，低成本介质.
	TierCold FileTier = "cold"
	// TierArchive 归档存储：极少访问，最低成本介质.
	TierArchive FileTier = "archive"
)

// TierOrder 层级顺序，用于验证迁移方向.
var TierOrder = map[FileTier]int{
	TierHot:     1,
	TierWarm:    2,
	TierCold:    3,
	TierArchive: 4,
}

// ==================== 生命周期阶段 ====================

// LifecycleStage 生命周期阶段.
type LifecycleStage string

const (
	// StageActive 活跃阶段：频繁读写.
	StageActive LifecycleStage = "active"
	// StageIdle 空闲阶段：偶尔访问.
	StageIdle LifecycleStage = "idle"
	// StageDormant 休眠阶段：极少访问.
	StageDormant LifecycleStage = "dormant"
	// StageArchived 已归档阶段：长期未访问.
	StageArchived LifecycleStage = "archived"
	// StageRetained 保留阶段：受合规保留策略保护.
	StageRetained LifecycleStage = "retained"
	// StageExpired 已过期阶段：超过保留期限.
	StageExpired LifecycleStage = "expired"
	// StagePendingDestroy 待销毁阶段：等待销毁确认.
	StagePendingDestroy LifecycleStage = "pending_destroy"
	// StageDestroyed 已销毁阶段：数据已被安全删除.
	StageDestroyed LifecycleStage = "destroyed"
)

// StageOrder 阶段顺序.
var StageOrder = map[LifecycleStage]int{
	StageActive:         1,
	StageIdle:           2,
	StageDormant:        3,
	StageArchived:       4,
	StageRetained:       5,
	StageExpired:        6,
	StagePendingDestroy: 7,
	StageDestroyed:      8,
}

// ==================== 文件分类 ====================

// FileCategory 文件分类.
type FileCategory string

const (
	// CategoryDocument 文档类文件.
	CategoryDocument FileCategory = "document"
	// CategoryMedia 媒体类文件（图片、视频、音频）.
	CategoryMedia FileCategory = "media"
	// CategoryCode 代码/开发文件.
	CategoryCode FileCategory = "code"
	// CategoryArchive 归档/压缩包文件.
	CategoryArchive FileCategory = "archive"
	// CategoryDatabase 数据库文件.
	CategoryDatabase FileCategory = "database"
	// CategoryLog 日志文件.
	CategoryLog FileCategory = "log"
	// CategoryBackup 备份文件.
	CategoryBackup FileCategory = "backup"
	// CategoryOther 其他类型文件.
	CategoryOther FileCategory = "other"
)

// ==================== 迁移状态 ====================

// MigrationState 迁移任务状态.
type MigrationState string

const (
	// MigrationPending 待执行.
	MigrationPending MigrationState = "pending"
	// MigrationInProgress 执行中.
	MigrationInProgress MigrationState = "in_progress"
	// MigrationDone 已完成.
	MigrationDone MigrationState = "done"
	// MigrationFailed 已失败.
	MigrationFailed MigrationState = "failed"
	// MigrationCancelled 已取消.
	MigrationCancelled MigrationState = "cancelled"
)

// ==================== 保留类型 ====================

// RetentionKind 保留策略类型.
type RetentionKind string

const (
	// RetentionTime 基于时间的保留.
	RetentionTime RetentionKind = "time"
	// RetentionLegal 法律保留.
	RetentionLegal RetentionKind = "legal"
	// RetentionAudit 审计保留.
	RetentionAudit RetentionKind = "audit"
	// RetentionIndefinite 无限期保留（手动解除）.
	RetentionIndefinite RetentionKind = "indefinite"
)

// ==================== 销毁状态 ====================

// DestroyState 销毁状态.
type DestroyState string

const (
	// DestroyPending 待确认.
	DestroyPending DestroyState = "pending"
	// DestroyApproved 已批准.
	DestroyApproved DestroyState = "approved"
	// DestroyExecuting 执行中.
	DestroyExecuting DestroyState = "executing"
	// DestroyDone 已完成.
	DestroyDone DestroyState = "done"
	// DestroyRejected 已拒绝.
	DestroyRejected DestroyState = "rejected"
)

// ==================== 核心配置 ====================

// FileLifecycleConfig 文件生命周期管理总配置.
type FileLifecycleConfig struct {
	// Enabled 是否启用生命周期管理.
	Enabled bool `json:"enabled"`
	// AutoMigrate 启用自动迁移.
	AutoMigrate bool `json:"autoMigrate"`
	// AutoCleanup 启用自动过期清理.
	AutoCleanup bool `json:"autoCleanup"`
	// ScanIntervalSec 扫描间隔（秒）.
	ScanIntervalSec int `json:"scanIntervalSec"`
	// HotAccessThreshold 热数据访问次数阈值（次/天）.
	HotAccessThreshold float64 `json:"hotAccessThreshold"`
	// WarmAccessThreshold 温数据访问次数阈值（次/天）.
	WarmAccessThreshold float64 `json:"warmAccessThreshold"`
	// ColdIdleDays 冷数据空闲天数.
	ColdIdleDays int `json:"coldIdleDays"`
	// ArchiveIdleDays 归档数据空闲天数.
	ArchiveIdleDays int `json:"archiveIdleDays"`
	// MaxConcurrentMigrations 最大并发迁移数.
	MaxConcurrentMigrations int `json:"maxConcurrentMigrations"`
	// DryRun 试运行模式.
	DryRun bool `json:"dryRun"`
}

// DefaultConfig 默认配置.
func DefaultConfig() FileLifecycleConfig {
	return FileLifecycleConfig{
		Enabled:                 true,
		AutoMigrate:             false,
		AutoCleanup:             false,
		ScanIntervalSec:         300,
		HotAccessThreshold:      10.0,
		WarmAccessThreshold:     2.0,
		ColdIdleDays:            30,
		ArchiveIdleDays:         90,
		MaxConcurrentMigrations: 4,
		DryRun:                  false,
	}
}

// ==================== 分层策略 ====================

// TieringPolicy 分层迁移策略.
type TieringPolicy struct {
	// ID 策略唯一标识.
	ID string `json:"id"`
	// Name 策略名称.
	Name string `json:"name"`
	// Description 策略描述.
	Description string `json:"description,omitempty"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// Priority 优先级（越大越优先）.
	Priority int `json:"priority"`
	// SourceTier 源存储层.
	SourceTier FileTier `json:"sourceTier"`
	// TargetTier 目标存储层.
	TargetTier FileTier `json:"targetTier"`
	// Conditions 触发条件列表（AND 关系）.
	Conditions []TieringCondition `json:"conditions"`
	// FilePattern 文件路径匹配模式（支持通配符）.
	FilePattern string `json:"filePattern,omitempty"`
	// Categories 适用的文件分类（空表示全部）.
	Categories []FileCategory `json:"categories,omitempty"`
	// ExcludePaths 排除路径.
	ExcludePaths []string `json:"excludePaths,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updatedAt"`
}

// TieringCondition 分层触发条件.
type TieringCondition struct {
	// Type 条件类型：access_count, idle_days, file_size, last_access.
	Type string `json:"type"`
	// Operator 操作符：gt, lt, gte, lte, eq.
	Operator string `json:"operator"`
	// Value 条件值（字符串，运行时解析）.
	Value string `json:"value"`
}

// ==================== 保留策略 ====================

// RetentionPolicy 文件保留策略.
type RetentionPolicy struct {
	// ID 策略唯一标识.
	ID string `json:"id"`
	// Name 策略名称.
	Name string `json:"name"`
	// Description 策略描述.
	Description string `json:"description,omitempty"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// Kind 保留类型.
	Kind RetentionKind `json:"kind"`
	// Duration 保留时长（Kind=time 时生效）.
	Duration time.Duration `json:"duration"`
	// Regulation 关联法规名称（Kind=legal/audit 时生效）.
	Regulation string `json:"regulation,omitempty"`
	// AutoDestroy 过期后自动销毁.
	AutoDestroy bool `json:"autoDestroy"`
	// FilePattern 文件路径匹配模式.
	FilePattern string `json:"filePattern,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updatedAt"`
}

// ==================== 合规保留 ====================

// ComplianceHold 合规保留记录.
type ComplianceHold struct {
	// ID 保留唯一标识.
	ID string `json:"id"`
	// Kind 保留类型.
	Kind RetentionKind `json:"kind"`
	// Name 保留名称.
	Name string `json:"name"`
	// Description 保留描述.
	Description string `json:"description,omitempty"`
	// FilePaths 被保留的文件路径（支持通配符）.
	FilePaths []string `json:"filePaths"`
	// CaseNumber 案件/审计编号.
	CaseNumber string `json:"caseNumber"`
	// IssuedBy 发起人.
	IssuedBy string `json:"issuedBy"`
	// Regulation 关联法规.
	Regulation string `json:"regulation,omitempty"`
	// ExpiresAt 过期时间（nil 表示手动解除）.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// Active 是否生效.
	Active bool `json:"active"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
	// ReleasedAt 解除时间.
	ReleasedAt *time.Time `json:"releasedAt,omitempty"`
	// ReleasedBy 解除人.
	ReleasedBy string `json:"releasedBy,omitempty"`
}

// ==================== 文件记录 ====================

// FileRecord 文件生命周期记录.
type FileRecord struct {
	// ID 记录唯一标识.
	ID string `json:"id"`
	// Path 文件路径.
	Path string `json:"path"`
	// Name 文件名.
	Name string `json:"name"`
	// Size 文件大小（字节）.
	Size int64 `json:"size"`
	// Category 文件分类.
	Category FileCategory `json:"category"`
	// CurrentTier 当前存储层.
	CurrentTier FileTier `json:"currentTier"`
	// CurrentStage 当前生命周期阶段.
	CurrentStage LifecycleStage `json:"currentStage"`
	// AccessCount 累计访问次数.
	AccessCount int64 `json:"accessCount"`
	// DailyAccessCount 日均访问次数.
	DailyAccessCount float64 `json:"dailyAccessCount"`
	// LastAccessedAt 最后访问时间.
	LastAccessedAt time.Time `json:"lastAccessedAt"`
	// CreatedAt 文件创建时间.
	CreatedAt time.Time `json:"createdAt"`
	// ModifiedAt 文件修改时间.
	ModifiedAt time.Time `json:"modifiedAt"`
	// PolicyID 关联的保留策略ID.
	PolicyID string `json:"policyId,omitempty"`
	// HoldIDs 关联的合规保留ID列表.
	HoldIDs []string `json:"holdIds,omitempty"`
	// Tags 文件标签.
	Tags []string `json:"tags,omitempty"`
	// TierHistory 层级变更历史.
	TierHistory []TierTransition `json:"tierHistory,omitempty"`
	// StageHistory 阶段变更历史.
	StageHistory []StageTransition `json:"stageHistory,omitempty"`
}

// TierTransition 层级转换记录.
type TierTransition struct {
	FromTier  FileTier  `json:"fromTier"`
	ToTier    FileTier  `json:"toTier"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
	PolicyID  string    `json:"policyId,omitempty"`
}

// StageTransition 阶段转换记录.
type StageTransition struct {
	FromStage LifecycleStage `json:"fromStage"`
	ToStage   LifecycleStage `json:"toStage"`
	Timestamp time.Time      `json:"timestamp"`
	Reason    string         `json:"reason"`
}

// ==================== 迁移任务 ====================

// FileMigration 文件迁移任务.
type FileMigration struct {
	// ID 任务唯一标识.
	ID string `json:"id"`
	// State 任务状态.
	State MigrationState `json:"state"`
	// SourceTier 源存储层.
	SourceTier FileTier `json:"sourceTier"`
	// TargetTier 目标存储层.
	TargetTier FileTier `json:"targetTier"`
	// Files 待迁移文件列表.
	Files []MigrationFileEntry `json:"files,omitempty"`
	// TotalFiles 文件总数.
	TotalFiles int `json:"totalFiles"`
	// TotalBytes 总字节数.
	TotalBytes int64 `json:"totalBytes"`
	// MigratedFiles 已迁移文件数.
	MigratedFiles int `json:"migratedFiles"`
	// MigratedBytes 已迁移字节数.
	MigratedBytes int64 `json:"migratedBytes"`
	// FailedFiles 失败文件数.
	FailedFiles int `json:"failedFiles"`
	// Errors 错误记录.
	Errors []MigrationError `json:"errors,omitempty"`
	// DryRun 试运行模式.
	DryRun bool `json:"dryRun"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
	// StartedAt 开始时间.
	StartedAt time.Time `json:"startedAt,omitempty"`
	// CompletedAt 完成时间.
	CompletedAt time.Time `json:"completedAt,omitempty"`
}

// MigrationFileEntry 迁移文件条目.
type MigrationFileEntry struct {
	// FileID 文件记录ID.
	FileID string `json:"fileId"`
	// Path 文件路径.
	Path string `json:"path"`
	// Size 文件大小.
	Size int64 `json:"size"`
	// State 条目状态.
	State string `json:"state"` // pending, done, failed
	// Error 错误信息.
	Error string `json:"error,omitempty"`
}

// MigrationError 迁移错误记录.
type MigrationError struct {
	// Path 文件路径.
	Path string `json:"path"`
	// Message 错误消息.
	Message string `json:"message"`
	// Time 错误时间.
	Time time.Time `json:"time"`
}

// ==================== 销毁记录 ====================

// DestructionRecord 数据销毁记录.
type DestructionRecord struct {
	// ID 记录唯一标识.
	ID string `json:"id"`
	// State 销毁状态.
	State DestroyState `json:"state"`
	// FilePaths 待销毁文件路径.
	FilePaths []string `json:"filePaths"`
	// Reason 销毁原因.
	Reason string `json:"reason"`
	// TotalSize 待销毁数据总大小.
	TotalSize int64 `json:"totalSize"`
	// DestroyedSize 已销毁数据大小.
	DestroyedSize int64 `json:"destroyedSize"`
	// HoldID 关联的合规保留ID（如有）.
	HoldID string `json:"holdId,omitempty"`
	// RequiresApproval 是否需要审批.
	RequiresApproval bool `json:"requiresApproval"`
	// Approvers 审批人列表.
	Approvers []string `json:"approvers,omitempty"`
	// ApprovedBy 审批人.
	ApprovedBy string `json:"approvedBy,omitempty"`
	// ApprovedAt 审批时间.
	ApprovedAt *time.Time `json:"approvedAt,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
	// CompletedAt 完成时间.
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// ==================== 审计日志 ====================

// AuditEntry 审计日志条目.
type AuditEntry struct {
	// ID 日志唯一标识.
	ID string `json:"id"`
	// Timestamp 时间戳.
	Timestamp time.Time `json:"timestamp"`
	// Action 操作类型：create_policy, apply_policy, migrate, tier_change, stage_change, retain, release_hold, destroy.
	Action string `json:"action"`
	// Target 操作目标（文件路径或策略ID）.
	Target string `json:"target"`
	// Details 详细信息.
	Details string `json:"details"`
	// Operator 操作人.
	Operator string `json:"operator"`
	// Success 是否成功.
	Success bool `json:"success"`
	// FileTier 相关存储层.
	FileTier FileTier `json:"fileTier,omitempty"`
	// Stage 相关阶段.
	Stage LifecycleStage `json:"stage,omitempty"`
	// PolicyID 相关策略ID.
	PolicyID string `json:"policyId,omitempty"`
}

// ==================== 分析报告 ====================

// TierDistribution 层级分布统计.
type TierDistribution struct {
	Tier      FileTier `json:"tier"`
	FileCount int      `json:"fileCount"`
	TotalSize int64    `json:"totalSize"`
	Percent   float64  `json:"percent"`
}

// StageDistribution 阶段分布统计.
type StageDistribution struct {
	Stage     LifecycleStage `json:"stage"`
	FileCount int            `json:"fileCount"`
	TotalSize int64          `json:"totalSize"`
}

// LifecycleReport 生命周期分析报告.
type LifecycleReport struct {
	// GeneratedAt 报告生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// TotalFiles 文件总数.
	TotalFiles int `json:"totalFiles"`
	// TotalSize 数据总大小（字节）.
	TotalSize int64 `json:"totalSize"`
	// TierDistributions 各层级分布.
	TierDistributions []TierDistribution `json:"tierDistributions"`
	// StageDistributions 各阶段分布.
	StageDistributions []StageDistribution `json:"stageDistributions"`
	// PendingMigrations 待迁移文件数.
	PendingMigrations int `json:"pendingMigrations"`
	// ExpiredFiles 已过期文件数.
	ExpiredFiles int `json:"expiredFiles"`
	// ActiveHolds 生效中的合规保留数.
	ActiveHolds int `json:"activeHolds"`
}

// ==================== 模块状态 ====================

// ModuleStatus 模块运行状态.
type ModuleStatus struct {
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// AutoMigrate 自动迁移是否开启.
	AutoMigrate bool `json:"autoMigrate"`
	// AutoCleanup 自动清理是否开启.
	AutoCleanup bool `json:"autoCleanup"`
	// TotalPolicies 总策略数.
	TotalPolicies int `json:"totalPolicies"`
	// ActivePolicies 活跃策略数.
	ActivePolicies int `json:"activePolicies"`
	// TotalRecords 文件记录数.
	TotalRecords int `json:"totalRecords"`
	// ActiveHolds 生效中的合规保留数.
	ActiveHolds int `json:"activeHolds"`
	// RunningMigrations 执行中的迁移任务数.
	RunningMigrations int `json:"runningMigrations"`
	// PendingDestructions 待销毁任务数.
	PendingDestructions int `json:"pendingDestructions"`
	// TierDistribution 层级分布.
	TierDistribution map[FileTier]int `json:"tierDistribution"`
	// StageDistribution 阶段分布.
	StageDistribution map[LifecycleStage]int `json:"stageDistribution"`
}

// ==================== API 请求/响应 ====================

// APIResponse 通用API响应.
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// BatchMigrateRequest 批量迁移请求.
type BatchMigrateRequest struct {
	// FileIDs 文件记录ID列表（与 Paths 二选一）.
	FileIDs []string `json:"fileIds,omitempty"`
	// Paths 文件路径列表.
	Paths []string `json:"paths,omitempty"`
	// TargetTier 目标存储层.
	TargetTier FileTier `json:"targetTier" binding:"required"`
	// DryRun 试运行.
	DryRun bool `json:"dryRun"`
}

// BatchMigrateResult 批量迁移结果.
type BatchMigrateResult struct {
	// TotalFiles 总文件数.
	TotalFiles int `json:"totalFiles"`
	// AcceptedFiles 已接受的文件数.
	AcceptedFiles int `json:"acceptedFiles"`
	// SkippedFiles 跳过的文件数（已在目标层）.
	SkippedFiles int `json:"skippedFiles"`
	// FailedFiles 失败文件数.
	FailedFiles int `json:"failedFiles"`
	// MigrationID 创建的迁移任务ID.
	MigrationID string `json:"migrationId,omitempty"`
	// Errors 错误列表.
	Errors []string `json:"errors,omitempty"`
}
