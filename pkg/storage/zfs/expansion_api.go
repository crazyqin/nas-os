// Package zfs RAIDZ Expansion API 接口定义
// Version: v1.0.0
// 基于 OpenZFS 2.3 RAIDZ Expansion 特性设计
// 参考 TrueNAS 24.10 Electric Eel 实现

package zfs

import (
	"context"
	"time"
)

// ========== RAIDZ Expansion Manager 接口 ==========

// RAIDZExpansionManagerInterface RAIDZ 扩展管理器接口
type RAIDZExpansionManagerInterface interface {
	// IsAvailable 检查管理器是否可用
	IsAvailable() bool

	// CheckExpansionSupport 检查系统是否支持 RAIDZ 扩展
	CheckExpansionSupport() (supported bool, reason string)

	// GetExpansionStatus 获取当前扩展状态
	GetExpansionStatus() *ExpansionStatus

	// GetExpansionHistory 获取扩展历史记录
	GetExpansionHistory() []ExpansionStatus

	// GetPoolExpansionInfo 获取池扩展信息
	GetPoolExpansionInfo(ctx context.Context, poolName string) (*PoolExpansionInfo, error)

	// StartExpansion 开始 RAIDZ 扩展
	StartExpansion(ctx context.Context, config ExpansionConfig) (*ExpansionStatus, error)

	// PauseExpansion 暂停扩展
	PauseExpansion() error

	// ResumeExpansion 恢复扩展
	ResumeExpansion() error

	// CancelExpansion 取消扩展
	CancelExpansion() error

	// EstimateExpansionTime 估算扩展时间
	EstimateExpansionTime(ctx context.Context, poolName string) (time.Duration, error)

	// ValidateDiskSimple 简单验证磁盘是否可用于扩展
	ValidateDiskSimple(ctx context.Context, diskPath string) error

	// ListAvailableDisks 列出可用磁盘
	ListAvailableDisks(ctx context.Context) ([]string, error)

	// SetStateChangeCallback 设置状态变更回调
	SetStateChangeCallback(callback func(status *ExpansionStatus))

	// Close 关闭管理器
	Close() error
}

// ========== 扩展任务管理接口 ==========

// ExpansionTaskManager 扩展任务管理接口
type ExpansionTaskManager interface {
	// CreateExpansionTask 创建扩展任务
	CreateExpansionTask(ctx context.Context, config ExpansionConfig) (*ExpansionTask, error)

	// GetExpansionTask 获取扩展任务
	GetExpansionTask(ctx context.Context, taskID string) (*ExpansionTask, error)

	// ListExpansionTasks 列出扩展任务
	ListExpansionTasks(ctx context.Context) ([]ExpansionTask, error)

	// CancelExpansionTask 取消扩展任务
	CancelExpansionTask(ctx context.Context, taskID string) error

	// PauseExpansionTask 暂停扩展任务
	PauseExpansionTask(ctx context.Context, taskID string) error

	// ResumeExpansionTask 恢复扩展任务
	ResumeExpansionTask(ctx context.Context, taskID string) error

	// GetTaskProgress 获取任务进度
	GetTaskProgress(ctx context.Context, taskID string) (*ExpansionProgress, error)

	// CleanupCompletedTasks 清理已完成任务
	CleanupCompletedTasks(ctx context.Context, olderThan time.Duration) error
}

// ExpansionTask 扩展任务
type ExpansionTask struct {
	// ID 任务 ID
	ID string `json:"id"`

	// PoolName 池名称
	PoolName string `json:"poolName"`

	// Config 扩展配置
	Config ExpansionConfig `json:"config"`

	// Status 任务状态
	Status ExpansionTaskStatus `json:"status"`

	// Progress 扩展进度
	Progress *ExpansionProgress `json:"progress"`

	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`

	// StartedAt 开始时间
	StartedAt *time.Time `json:"startedAt,omitempty"`

	// CompletedAt 完成时间
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// Error 错误信息
	Error string `json:"error,omitempty"`

	// Warnings 警告信息
	Warnings []string `json:"warnings,omitempty"`

	// Metadata 元数据
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ExpansionTaskStatus 扩展任务状态
type ExpansionTaskStatus string

const (
	// TaskStatusPending 待执行
	TaskStatusPending ExpansionTaskStatus = "pending"
	// TaskStatusPreparing 准备中
	TaskStatusPreparing ExpansionTaskStatus = "preparing"
	// TaskStatusRunning 执行中
	TaskStatusRunning ExpansionTaskStatus = "running"
	// TaskStatusPaused 已暂停
	TaskStatusPaused ExpansionTaskStatus = "paused"
	// TaskStatusCompleted 已完成
	TaskStatusCompleted ExpansionTaskStatus = "completed"
	// TaskStatusFailed 失败
	TaskStatusFailed ExpansionTaskStatus = "failed"
	// TaskStatusCancelled 已取消
	TaskStatusCancelled ExpansionTaskStatus = "cancelled"
)

// ExpansionProgress 扩展进度
type ExpansionProgress struct {
	// TaskID 任务 ID
	TaskID string `json:"taskId"`

	// Percentage 百分比 (0-100)
	Percentage float64 `json:"percentage"`

	// BytesProcessed 已处理字节
	BytesProcessed uint64 `json:"bytesProcessed"`

	// BytesTotal 总字节
	BytesTotal uint64 `json:"bytesTotal"`

	// Speed 当前速度 (MB/s)
	Speed float64 `json:"speed"`

	// AverageSpeed 平均速度 (MB/s)
	AverageSpeed float64 `json:"averageSpeed"`

	// ETA 预估剩余时间
	ETA time.Duration `json:"eta"`

	// Elapsed 已用时间
	Elapsed time.Duration `json:"elapsed"`

	// Phase 当前阶段
	Phase ExpansionPhase `json:"phase"`

	// PhaseProgress 阶段进度
	PhaseProgress map[string]float64 `json:"phaseProgress,omitempty"`

	// LastUpdate 最后更新时间
	LastUpdate time.Time `json:"lastUpdate"`
}

// ExpansionPhase 扩展阶段
type ExpansionPhase string

const (
	// PhaseValidation 验证阶段
	PhaseValidation ExpansionPhase = "validation"
	// PhasePreparation 准备阶段
	PhasePreparation ExpansionPhase = "preparation"
	// PhaseDataMigration 数据迁移阶段
	PhaseDataMigration ExpansionPhase = "dataMigration"
	// PhaseVerification 验证阶段
	PhaseVerification ExpansionPhase = "verification"
	// PhaseCompletion 完成阶段
	PhaseCompletion ExpansionPhase = "completion"
)

// ========== 扩展容量估算接口 ==========

// ExpansionCapacityEstimator 扩展容量估算接口
type ExpansionCapacityEstimator interface {
	// EstimateCapacityGain 估算容量增益
	EstimateCapacityGain(ctx context.Context, poolName string, newDiskSize uint64) (*CapacityEstimate, error)

	// EstimateHeadroomLoss 估算容量折损
	EstimateHeadroomLoss(ctx context.Context, poolName string, originalWidth, newWidth int) (*HeadroomLoss, error)

	// CalculateRecoveryPotential 计算恢复潜力
	CalculateRecoveryPotential(ctx context.Context, poolName string) (*RecoveryPotential, error)

	// CompareExpansionOptions 对比扩展选项
	CompareExpansionOptions(ctx context.Context, poolName string, options []ExpansionOption) ([]ExpansionComparison, error)
}

// CapacityEstimate 容量估算
type CapacityEstimate struct {
	// PoolName 池名称
	PoolName string `json:"poolName"`

	// OriginalCapacity 原始容量
	OriginalCapacity uint64 `json:"originalCapacity"`

	// NewCapacity 新容量
	NewCapacity uint64 `json:"newCapacity"`

	// CapacityGain 容量增益
	CapacityGain uint64 `json:"capacityGain"`

	// EffectiveGain 有效增益（考虑折损）
	EffectiveGain uint64 `json:"effectiveGain"`

	// EfficiencyRatio 效率比
	EfficiencyRatio float64 `json:"efficiencyRatio"`

	// OriginalDataRatio 原始数据比
	OriginalDataRatio float64 `json:"originalDataRatio"`

	// NewDataRatio 新数据比
	NewDataRatio float64 `json:"newDataRatio"`

	// RAIDZLevel RAIDZ 级别
	RAIDZLevel RAIDZLevel `json:"raidzLevel"`

	// OriginalWidth 原始宽度
	OriginalWidth int `json:"originalWidth"`

	// NewWidth 新宽度
	NewWidth int `json:"newWidth"`

	// DiskSize 磁盘大小
	DiskSize uint64 `json:"diskSize"`
}

// HeadroomLoss 容量折损
type HeadroomLoss struct {
	// PoolName 池名称
	PoolName string `json:"poolName"`

	// PercentageLoss 折损百分比
	PercentageLoss float64 `json:"percentageLoss"`

	// BytesLoss 折损字节
	BytesLoss uint64 `json:"bytesLoss"`

	// Reason 折损原因
	Reason string `json:"reason"`

	// RecoveryMethods 恢复方法
	RecoveryMethods []RecoveryMethod `json:"recoveryMethods"`

	// EstimatedRecoveryTime 预估恢复时间
	EstimatedRecoveryTime time.Duration `json:"estimatedRecoveryTime"`
}

// RecoveryMethod 恢复方法
type RecoveryMethod struct {
	// Method 方法名称
	Method string `json:"method"`

	// Description 描述
	Description string `json:"description"`

	// EffortLevel 工作量级别
	EffortLevel string `json:"effortLevel"` // low, medium, high

	// RecoveryPercentage 恢复百分比
	RecoveryPercentage float64 `json:"recoveryPercentage"`

	// EstimatedTime 预估时间
	EstimatedTime time.Duration `json:"estimatedTime"`

	// Recommended 是否推荐
	Recommended bool `json:"recommended"`
}

// RecoveryPotential 恢复潜力
type RecoveryPotential struct {
	// PoolName 池名称
	PoolName string `json:"poolName"`

	// TotalRecoverable 可恢复总量
	TotalRecoverable uint64 `json:"totalRecoverable"`

	// DataAgeDistribution 数据年龄分布
	DataAgeDistribution map[string]uint64 `json:"dataAgeDistribution"`

	// HotDataRatio 热数据比
	HotDataRatio float64 `json:"hotDataRatio"`

	// ColdDataRatio 冷数据比
	ColdDataRatio float64 `json:"coldDataRatio"`

	// NaturalRecoveryRate 自然恢复率（每月）
	NaturalRecoveryRate float64 `json:"naturalRecoveryRate"`

	// ActiveRecoveryOptions 主动恢复选项
	ActiveRecoveryOptions []RecoveryMethod `json:"activeRecoveryOptions"`

	// Recommendation 建议
	Recommendation string `json:"recommendation"`
}

// ExpansionOption 扩展选项
type ExpansionOption struct {
	// OptionID 选项 ID
	OptionID string `json:"optionId"`

	// DiskCount 磁盘数量
	DiskCount int `json:"diskCount"`

	// DiskSize 磁盘大小
	DiskSize uint64 `json:"diskSize"`

	// Method 扩展方法
	Method ExpansionMethod `json:"method"`

	// Description 描述
	Description string `json:"description"`
}

// ExpansionMethod 扩展方法
type ExpansionMethod string

const (
	// MethodSingleDisk 单盘扩展
	MethodSingleDisk ExpansionMethod = "singleDisk"
	// MethodAddVdev 添加 VDEV
	MethodAddVdev ExpansionMethod = "addVdev"
	// MethodReplaceDisk 替换磁盘
	MethodReplaceDisk ExpansionMethod = "replaceDisk"
)

// ExpansionComparison 扩展对比
type ExpansionComparison struct {
	// OptionID 选项 ID
	OptionID string `json:"optionId"`

	// CapacityGain 容量增益
	CapacityGain uint64 `json:"capacityGain"`

	// CostEstimate 成本估算
	CostEstimate float64 `json:"costEstimate"`

	// TimeEstimate 时间估算
	TimeEstimate time.Duration `json:"timeEstimate"`

	// EfficiencyScore 效率评分
	EfficiencyScore float64 `json:"efficiencyScore"`

	// RiskLevel 风险级别
	RiskLevel string `json:"riskLevel"` // low, medium, high

	// Pros 优点
	Pros []string `json:"pros"`

	// Cons 缺点
	Cons []string `json:"cons"`

	// Recommended 是否推荐
	Recommended bool `json:"recommended"`
}

// ========== 扩展验证接口 ==========

// ExpansionValidator 扩展验证接口
type ExpansionValidator interface {
	// ValidatePool 验证池是否可扩展
	ValidatePool(ctx context.Context, poolName string) (*PoolValidationResult, error)

	// ValidateDisk 验证磁盘是否可用于扩展
	ValidateDisk(ctx context.Context, diskPath string) (*DiskValidationResult, error)

	// ValidateExpansionConfig 验证扩展配置
	ValidateExpansionConfig(ctx context.Context, config ExpansionConfig) (*ConfigValidationResult, error)

	// CheckPrerequisites 检查前置条件
	CheckPrerequisites(ctx context.Context, poolName string) (*PrerequisiteCheck, error)
}

// PoolValidationResult 池验证结果
type PoolValidationResult struct {
	// Valid 是否有效
	Valid bool `json:"valid"`

	// PoolName 池名称
	PoolName string `json:"poolName"`

	// PoolState 池状态
	PoolState string `json:"poolState"`

	// IsHealthy 是否健康
	IsHealthy bool `json:"isHealthy"`

	// HasSingleRAIDZ 是否为单 RAIDZ
	HasSingleRAIDZ bool `json:"hasSingleRAIDZ"`

	// FeatureEnabled 特性是否启用
	FeatureEnabled bool `json:"featureEnabled"`

	// Issues 问题列表
	Issues []ValidationIssue `json:"issues,omitempty"`

	// Warnings 警告列表
	Warnings []string `json:"warnings,omitempty"`
}

// DiskValidationResult 磁盘验证结果
type DiskValidationResult struct {
	// Valid 是否有效
	Valid bool `json:"valid"`

	// DiskPath 磁盘路径
	DiskPath string `json:"diskPath"`

	// DiskSize 磁盘大小
	DiskSize uint64 `json:"diskSize"`

	// IsAvailable 是否可用
	IsAvailable bool `json:"isAvailable"`

	// HasPartitions 是否有分区
	HasPartitions bool `json:"hasPartitions"`

	// IsInUse 是否在使用
	IsInUse bool `json:"isInUse"`

	// SizeMatch 磁盘大小是否匹配
	SizeMatch bool `json:"sizeMatch"`

	// Issues 问题列表
	Issues []ValidationIssue `json:"issues,omitempty"`
}

// ConfigValidationResult 配置验证结果
type ConfigValidationResult struct {
	// Valid 是否有效
	Valid bool `json:"valid"`

	// PoolName 池名称
	PoolName string `json:"poolName"`

	// NewDisk 新磁盘
	NewDisk string `json:"newDisk"`

	// RAIDZLevelMatch RAIDZ 级别是否匹配
	RAIDZLevelMatch bool `json:"raidzLevelMatch"`

	// Issues 问题列表
	Issues []ValidationIssue `json:"issues,omitempty"`

	// EstimatedTime 预估时间
	EstimatedTime time.Duration `json:"estimatedTime"`

	// EstimatedCapacityGain 预估容量增益
	EstimatedCapacityGain uint64 `json:"estimatedCapacityGain"`
}

// ValidationIssue 验证问题
type ValidationIssue struct {
	// Code 问题代码
	Code string `json:"code"`

	// Severity 严重程度
	Severity string `json:"severity"` // error, warning, info

	// Message 消息
	Message string `json:"message"`

	// Field 相关字段
	Field string `json:"field,omitempty"`

	// Resolution 解决方法
	Resolution string `json:"resolution,omitempty"`
}

// PrerequisiteCheck 前置条件检查
type PrerequisiteCheck struct {
	// Passed 是否通过
	Passed bool `json:"passed"`

	// Checks 检查项
	Checks []PrerequisiteCheckItem `json:"checks"`

	// OverallStatus 总体状态
	OverallStatus string `json:"overallStatus"` // pass, fail, warning

	// Summary 总结
	Summary string `json:"summary"`
}

// PrerequisiteCheckItem 前置条件检查项
type PrerequisiteCheckItem struct {
	// Name 检查项名称
	Name string `json:"name"`

	// Status 状态
	Status string `json:"status"` // pass, fail, warning

	// Message 消息
	Message string `json:"message"`

	// Required 是否必需
	Required bool `json:"required"`

	// Action 建议操作
	Action string `json:"action,omitempty"`
}

// ========== 扩展通知接口 ==========

// ExpansionNotifier 扩展通知接口
type ExpansionNotifier interface {
	// NotifyExpansionStart 通知扩展开始
	NotifyExpansionStart(status *ExpansionStatus) error

	// NotifyExpansionProgress 通知扩展进度
	NotifyExpansionProgress(progress *ExpansionProgress) error

	// NotifyExpansionComplete 通知扩展完成
	NotifyExpansionComplete(status *ExpansionStatus) error

	// NotifyExpansionFailed 通知扩展失败
	NotifyExpansionFailed(status *ExpansionStatus, err error) error

	// NotifyExpansionPaused 通知扩展暂停
	NotifyExpansionPaused(status *ExpansionStatus) error

	// NotifyExpansionResumed 通知扩展恢复
	NotifyExpansionResumed(status *ExpansionStatus) error

	// NotifyDiskFailure 通知磁盘故障
	NotifyDiskFailure(status *ExpansionStatus, diskPath string) error

	// SubscribeProgress 订阅进度更新
	SubscribeProgress(taskID string, callback func(progress *ExpansionProgress)) error

	// UnsubscribeProgress 取消订阅
	UnsubscribeProgress(taskID string) error
}

// ExpansionNotification 扩展通知
type ExpansionNotification struct {
	// Type 通知类型
	Type NotificationType `json:"type"`

	// TaskID 任务 ID
	TaskID string `json:"taskId"`

	// PoolName 池名称
	PoolName string `json:"poolName"`

	// Message 消息
	Message string `json:"message"`

	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`

	// Data 数据
	Data map[string]interface{} `json:"data,omitempty"`

	// Priority 优先级
	Priority string `json:"priority"` // low, normal, high, critical
}

// NotificationType 通知类型
type NotificationType string

const (
	// NotificationStart 开始通知
	NotificationStart NotificationType = "start"
	// NotificationProgress 进度通知
	NotificationProgress NotificationType = "progress"
	// NotificationComplete 完成通知
	NotificationComplete NotificationType = "complete"
	// NotificationFailed 失败通知
	NotificationFailed NotificationType = "failed"
	// NotificationPaused 暂停通知
	NotificationPaused NotificationType = "paused"
	// NotificationResumed 恢复通知
	NotificationResumed NotificationType = "resumed"
	// NotificationWarning 警告通知
	NotificationWarning NotificationType = "warning"
	// NotificationError 错误通知
	NotificationError NotificationType = "error"
	// NotificationDiskFailure 磁盘故障通知
	NotificationDiskFailure NotificationType = "diskFailure"
)

// ========== 完整服务接口 ==========

// RAIDZExpansionService 完整的 RAIDZ 扩展服务接口
type RAIDZExpansionService interface {
	RAIDZExpansionManagerInterface
	ExpansionTaskManager
	ExpansionCapacityEstimator
	ExpansionValidator
	ExpansionNotifier
}