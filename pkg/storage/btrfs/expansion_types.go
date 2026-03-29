// Package btrfs Btrfs RAID扩容类型定义
// 参考 ZFS RAIDZ Expansion 的设计思路，为 Btrfs 实现类似功能
// Btrfs扩容核心步骤: device add -> balance
package btrfs

import (
	"time"
)

// ========== 扩展状态定义 ==========

// ExpansionState 扩展状态
type ExpansionState string

const (
	// ExpansionStateIdle 空闲，无扩展进行
	ExpansionStateIdle ExpansionState = "idle"
	// ExpansionStatePreparing 准备中
	ExpansionStatePreparing ExpansionState = "preparing"
	// ExpansionStateAddingDevice 添加设备阶段
	ExpansionStateAddingDevice ExpansionState = "addingDevice"
	// ExpansionStateBalancing 数据平衡阶段
	ExpansionStateBalancing ExpansionState = "balancing"
	// ExpansionStateVerifying 验证阶段
	ExpansionStateVerifying ExpansionState = "verifying"
	// ExpansionStateCompleted 已完成
	ExpansionStateCompleted ExpansionState = "completed"
	// ExpansionStatePaused 已暂停
	ExpansionStatePaused ExpansionState = "paused"
	// ExpansionStateFailed 失败
	ExpansionStateFailed ExpansionState = "failed"
	// ExpansionStateCancelled 已取消
	ExpansionStateCancelled ExpansionState = "cancelled"
)

// ExpansionPhase 扩展阶段
type ExpansionPhase string

const (
	// PhaseValidation 验证阶段
	PhaseValidation ExpansionPhase = "validation"
	// PhasePreparation 准备阶段
	PhasePreparation ExpansionPhase = "preparation"
	// PhaseDeviceAdd 设备添加阶段
	PhaseDeviceAdd ExpansionPhase = "deviceAdd"
	// PhaseBalance 数据平衡阶段
	PhaseBalance ExpansionPhase = "balance"
	// PhaseVerification 验证阶段
	PhaseVerification ExpansionPhase = "verification"
	// PhaseCompletion 完成阶段
	PhaseCompletion ExpansionPhase = "completion"
)

// ========== 扩展配置 ==========

// ExpansionConfig 扩展配置
type ExpansionConfig struct {
	// VolumeName 卷名称
	VolumeName string `json:"volumeName"`

	// MountPoint 挂载点
	MountPoint string `json:"mountPoint"`

	// NewDevice 新增磁盘路径
	NewDevice string `json:"newDevice"`

	// TargetProfile 目标RAID配置（可选，如果不指定则保持原配置）
	// 支持: single, raid0, raid1, raid5, raid6, raid10
	TargetProfile string `json:"targetProfile,omitempty"`

	// Force 强制执行
	Force bool `json:"force"`

	// DryRun 仅模拟运行，不实际执行
	DryRun bool `json:"dryRun"`

	// BalanceOptions 平衡选项
	BalanceOptions BalanceOptions `json:"balanceOptions"`

	// AutoBalance 添加设备后自动执行平衡
	AutoBalance bool `json:"autoBalance"`

	// ProgressCallback 进度回调函数（可选）
	ProgressCallback func(phase ExpansionPhase, progress float64) `json:"-"`
}

// BalanceOptions 平衡选项
type BalanceOptions struct {
	// DataProfile 数据配置转换目标
	DataProfile string `json:"dataProfile"`

	// MetadataProfile 元数据配置转换目标
	MetadataProfile string `json:"metadataProfile"`

	// Start 开始平衡（不等待完成）
	Start bool `json:"start"`

	// Force 强制执行
	Force bool `json:"force"`

	// Full 全量平衡（包括已分配和未分配）
	Full bool `json:"full"`
}

// ========== 扩展状态 ==========

// ExpansionStatus 扩展状态
type ExpansionStatus struct {
	// ID 扩展任务 ID
	ID string `json:"id"`

	// VolumeName 卷名称
	VolumeName string `json:"volumeName"`

	// MountPoint 挂载点
	MountPoint string `json:"mountPoint"`

	// NewDevice 新增磁盘
	NewDevice string `json:"newDevice"`

	// State 当前状态
	State ExpansionState `json:"state"`

	// Phase 当前阶段
	Phase ExpansionPhase `json:"phase"`

	// Progress 总体进度百分比 (0-100)
	Progress float64 `json:"progress"`

	// PhaseProgress 阶段进度
	PhaseProgress map[string]float64 `json:"phaseProgress"`

	// OriginalDevices 原始设备列表
	OriginalDevices []string `json:"originalDevices"`

	// NewDevices 新设备列表（扩展后）
	NewDevices []string `json:"newDevices"`

	// OriginalProfile 原始RAID配置
	OriginalProfile string `json:"originalProfile"`

	// TargetProfile 目标RAID配置
	TargetProfile string `json:"targetProfile"`

	// OriginalCapacity 原始容量（字节）
	OriginalCapacity uint64 `json:"originalCapacity"`

	// NewCapacity 新容量（字节）
	NewCapacity uint64 `json:"newCapacity"`

	// CapacityGain 容量增益（字节）
	CapacityGain uint64 `json:"capacityGain"`

	// StartTime 开始时间
	StartTime time.Time `json:"startTime"`

	// EndTime 结束时间
	EndTime time.Time `json:"endTime,omitempty"`

	// EstimatedTimeRemaining 预计剩余时间
	EstimatedTimeRemaining time.Duration `json:"estimatedTimeRemaining"`

	// Speed 当前速度 (MB/s)
	Speed float64 `json:"speed"`

	// BytesProcessed 已处理字节数
	BytesProcessed uint64 `json:"bytesProcessed"`

	// TotalBytes 总字节数
	TotalBytes uint64 `json:"totalBytes"`

	// Errors 错误信息
	Errors []string `json:"errors,omitempty"`

	// Warnings 警告信息
	Warnings []string `json:"warnings,omitempty"`

	// CanPause 是否可暂停
	CanPause bool `json:"canPause"`

	// CanResume 是否可恢复
	CanResume bool `json:"canResume"`

	// CanCancel 是否可取消
	CanCancel bool `json:"canCancel"`

	// PauseCount 暂停次数
	PauseCount int `json:"pauseCount"`

	// LastUpdateTime 最后更新时间
	LastUpdateTime time.Time `json:"lastUpdateTime"`
}

// ========== 扩展历史 ==========

// ExpansionHistory 扩展历史记录
type ExpansionHistory struct {
	// Expansions 扩展记录列表
	Expansions []ExpansionStatus `json:"expansions"`

	// LastUpdated 最后更新时间
	LastUpdated time.Time `json:"lastUpdated"`
}

// ========== 设备验证 ==========

// DeviceValidationResult 设备验证结果
type DeviceValidationResult struct {
	// Valid 是否有效
	Valid bool `json:"valid"`

	// DevicePath 设备路径
	DevicePath string `json:"devicePath"`

	// DeviceSize 设备大小
	DeviceSize uint64 `json:"deviceSize"`

	// IsAvailable 是否可用
	IsAvailable bool `json:"isAvailable"`

	// HasPartitions 是否有分区
	HasPartitions bool `json:"hasPartitions"`

	// IsInUse 是否在使用
	IsInUse bool `json:"isInUse"`

	// IsBtrfsMember 是否已是Btrfs成员
	IsBtrfsMember bool `json:"isBtrfsMember"`

	// SizeMatch 设备大小是否匹配（与现有设备对比）
	SizeMatch bool `json:"sizeMatch"`

	// Issues 问题列表
	Issues []ValidationIssue `json:"issues,omitempty"`
}

// VolumeValidationResult 卷验证结果
type VolumeValidationResult struct {
	// Valid 是否有效
	Valid bool `json:"valid"`

	// VolumeName 卷名称
	VolumeName string `json:"volumeName"`

	// MountPoint 挂载点
	MountPoint string `json:"mountPoint"`

	// VolumeState 卷状态
	VolumeState string `json:"volumeState"`

	// IsHealthy 是否健康
	IsHealthy bool `json:"isHealthy"`

	// CurrentProfile 当前RAID配置
	CurrentProfile string `json:"currentProfile"`

	// DeviceCount 当前设备数
	DeviceCount int `json:"deviceCount"`

	// Issues 问题列表
	Issues []ValidationIssue `json:"issues,omitempty"`

	// Warnings 警告列表
	Warnings []string `json:"warnings,omitempty"`
}

// ValidationIssue 验证问题
type ValidationIssue struct {
	// Code 问题代码
	Code string `json:"code"`

	// Severity 严重程度 (error, warning, info)
	Severity string `json:"severity"`

	// Message 消息
	Message string `json:"message"`

	// Field 相关字段
	Field string `json:"field,omitempty"`

	// Resolution 解决方法
	Resolution string `json:"resolution,omitempty"`
}

// ========== 容量估算 ==========

// CapacityEstimate 容量估算
type CapacityEstimate struct {
	// VolumeName 卷名称
	VolumeName string `json:"volumeName"`

	// OriginalCapacity 原始容量
	OriginalCapacity uint64 `json:"originalCapacity"`

	// NewCapacity 新容量
	NewCapacity uint64 `json:"newCapacity"`

	// CapacityGain 容量增益
	CapacityGain uint64 `json:"capacityGain"`

	// EffectiveGain 有效增益（考虑RAID开销）
	EffectiveGain uint64 `json:"effectiveGain"`

	// EfficiencyRatio 效率比
	EfficiencyRatio float64 `json:"efficiencyRatio"`

	// OriginalDataRatio 原始数据比
	OriginalDataRatio float64 `json:"originalDataRatio"`

	// NewDataRatio 新数据比
	NewDataRatio float64 `json:"newDataRatio"`

	// RAIDLevel RAID级别
	RAIDLevel string `json:"raidLevel"`

	// OriginalWidth 原始宽度（磁盘数）
	OriginalWidth int `json:"originalWidth"`

	// NewWidth 新宽度
	NewWidth int `json:"newWidth"`

	// DiskSize 磁盘大小
	DiskSize uint64 `json:"diskSize"`
}

// ========== Btrfs RAID配置 ==========

// BtrfsRAIDConfig Btrfs RAID配置
type BtrfsRAIDConfig struct {
	// ProfileName 配置名称
	ProfileName string `json:"profileName"`

	// MinDevices 最少设备数
	MinDevices int `json:"minDevices"`

	// RecommendedDevices 推荐设备数
	RecommendedDevices int `json:"recommendedDevices"`

	// FaultTolerance 容错磁盘数
	FaultTolerance int `json:"faultTolerance"`

	// Efficiency 效率百分比（实际可用容量比例）
	Efficiency float64 `json:"efficiency"`

	// Description 描述
	Description string `json:"description"`

	// CanExpandTo 是否可以扩展到此配置
	CanExpandTo bool `json:"canExpandTo"`

	// ExpansionNote 扩展说明
	ExpansionNote string `json:"expansionNote"`
}

// PredefinedRAIDConfigs 预定义的Btrfs RAID配置
var PredefinedRAIDConfigs = map[string]BtrfsRAIDConfig{
	"single": {
		ProfileName:        "single",
		MinDevices:         1,
		RecommendedDevices: 1,
		FaultTolerance:     0,
		Efficiency:         100,
		Description:        "单盘模式，无冗余",
		CanExpandTo:        true,
		ExpansionNote:      "可扩展至任意RAID配置",
	},
	"raid0": {
		ProfileName:        "raid0",
		MinDevices:         2,
		RecommendedDevices: 2,
		FaultTolerance:     0,
		Efficiency:         100,
		Description:        "条带模式，性能最佳，无冗余",
		CanExpandTo:        true,
		ExpansionNote:      "可添加设备以扩展容量，但无法增加冗余",
	},
	"raid1": {
		ProfileName:        "raid1",
		MinDevices:         2,
		RecommendedDevices: 2,
		FaultTolerance:     1,
		Efficiency:         50,
		Description:        "镜像模式，允许1盘故障",
		CanExpandTo:        true,
		ExpansionNote:      "添加设备后需balance重新分布数据",
	},
	"raid10": {
		ProfileName:        "raid10",
		MinDevices:         4,
		RecommendedDevices: 4,
		FaultTolerance:     1,
		Efficiency:         50,
		Description:        "条带镜像，性能与冗余平衡",
		CanExpandTo:        true,
		ExpansionNote:      "扩展需保持偶数设备数",
	},
	"raid5": {
		ProfileName:        "raid5",
		MinDevices:         3,
		RecommendedDevices: 3,
		FaultTolerance:     1,
		Efficiency:         66.67,
		Description:        "分布式奇偶校验，允许1盘故障",
		CanExpandTo:        true,
		ExpansionNote:      "添加设备后balance重新分布数据",
	},
	"raid6": {
		ProfileName:        "raid6",
		MinDevices:         4,
		RecommendedDevices: 4,
		FaultTolerance:     2,
		Efficiency:         50,
		Description:        "双奇偶校验，允许2盘故障",
		CanExpandTo:        true,
		ExpansionNote:      "添加设备后balance重新分布数据",
	},
}

// ========== 扩展错误 ==========

// 扩展相关错误
var (
	// ErrExpansionInProgress 表示扩展正在进行
	ErrExpansionInProgress = NewExpansionError("expansion_in_progress", "RAID扩展正在进行中")

	// ErrNoExpansionInProgress 表示没有扩展正在进行
	ErrNoExpansionInProgress = NewExpansionError("no_expansion_in_progress", "没有RAID扩展正在进行")

	// ErrDeviceNotFound 表示设备未找到
	ErrDeviceNotFound = NewExpansionError("device_not_found", "设备未找到")

	// ErrDeviceInUse 表示设备已被使用
	ErrDeviceInUse = NewExpansionError("device_in_use", "设备已被使用")

	// ErrDeviceHasPartitions 表示设备有分区
	ErrDeviceHasPartitions = NewExpansionError("device_has_partitions", "设备存在分区，需要先清除")

	// ErrVolumeNotMounted 表示卷未挂载
	ErrVolumeNotMounted = NewExpansionError("volume_not_mounted", "卷未挂载")

	// ErrVolumeUnhealthy 表示卷不健康
	ErrVolumeUnhealthy = NewExpansionError("volume_unhealthy", "卷状态不健康，无法扩展")

	// ErrBalanceRunning 表示平衡正在运行
	ErrBalanceRunning = NewExpansionError("balance_running", "平衡任务正在运行")

	// ErrExpansionFailed 表示扩展失败
	ErrExpansionFailed = NewExpansionError("expansion_failed", "RAID扩展失败")

	// ErrExpansionCancelled 表示扩展已取消
	ErrExpansionCancelled = NewExpansionError("expansion_cancelled", "RAID扩展已取消")

	// ErrExpansionPaused 表示扩展已暂停
	ErrExpansionPaused = NewExpansionError("expansion_paused", "RAID扩展已暂停")

	// ErrInvalidProfile 表示无效的RAID配置
	ErrInvalidProfile = NewExpansionError("invalid_profile", "无效的RAID配置")
)

// ExpansionError 扩展错误类型
type ExpansionError struct {
	// Code 错误代码
	Code string `json:"code"`

	// Message 错误消息
	Message string `json:"message"`

	// Details 详细信息
	Details string `json:"details,omitempty"`
}

// NewExpansionError 创建扩展错误
func NewExpansionError(code, message string) *ExpansionError {
	return &ExpansionError{
		Code:    code,
		Message: message,
	}
}

// Error 实现error接口
func (e *ExpansionError) Error() string {
	if e.Details != "" {
		return e.Message + ": " + e.Details
	}
	return e.Message
}

// WithDetails 添加详细信息
func (e *ExpansionError) WithDetails(details string) *ExpansionError {
	e.Details = details
	return e
}