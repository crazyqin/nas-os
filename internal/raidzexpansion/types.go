// Package raidzexpansion 提供 RAID-Z 在线扩展功能
package raidzexpansion

import (
	"fmt"
	"time"
)

// ExpansionStatus 扩展任务状态
type ExpansionStatus string

const (
	// StatusPending 等待执行
	StatusPending ExpansionStatus = "pending"
	// StatusValidating 验证中
	StatusValidating ExpansionStatus = "validating"
	// StatusExpanding 扩展中
	StatusExpanding ExpansionStatus = "expanding"
	// StatusCompleted 已完成
	StatusCompleted ExpansionStatus = "completed"
	// StatusFailed 失败
	StatusFailed ExpansionStatus = "failed"
	// StatusRollingBack 回滚中
	StatusRollingBack ExpansionStatus = "rolling_back"
	// StatusRolledBack 已回滚
	StatusRolledBack ExpansionStatus = "rolled_back"
)

// DiskRole 磁盘角色
type DiskRole string

const (
	// RoleData 数据盘
	RoleData DiskRole = "data"
	// RoleParity 校验盘
	RoleParity DiskRole = "parity"
	// RoleSpare 热备盘
	RoleSpare DiskRole = "spare"
	// RoleLog 日志盘
	RoleLog DiskRole = "log"
	// RoleCache 缓存盘
	RoleCache DiskRole = "cache"
)

// RaidzLevel RAID-Z 等级
type RaidzLevel int

const (
	// Raidz1 RAID-Z1 (单校验)
	Raidz1 RaidzLevel = 1
	// Raidz2 RAID-Z2 (双校验)
	Raidz2 RaidzLevel = 2
	// Raidz3 RAID-Z3 (三校验)
	Raidz3 RaidzLevel = 3
)

// String 返回 RAID-Z 等级字符串
func (l RaidzLevel) String() string {
	return fmt.Sprintf("raidz%d", l)
}

// ExpansionRequest 扩展请求
type ExpansionRequest struct {
	// PoolName 存储池名称
	PoolName string `json:"poolName" binding:"required"`
	// NewDisks 新增磁盘列表
	NewDisks []string `json:"newDisks" binding:"required,min=1"`
	// Force 强制扩展（跳过部分检查）
	Force bool `json:"force,omitempty"`
	// DryRun 试运行模式
	DryRun bool `json:"dryRun,omitempty"`
}

// ExpansionResult 扩展结果
type ExpansionResult struct {
	// TaskID 任务ID
	TaskID string `json:"taskId"`
	// PoolName 存储池名称
	PoolName string `json:"poolName"`
	// Status 任务状态
	Status ExpansionStatus `json:"status"`
	// Progress 扩展进度 (0-100)
	Progress float64 `json:"progress"`
	// OriginalDiskCount 原始磁盘数
	OriginalDiskCount int `json:"originalDiskCount"`
	// NewDiskCount 扩展后磁盘数
	NewDiskCount int `json:"newDiskCount"`
	// AddedDisks 新增磁盘列表
	AddedDisks []string `json:"addedDisks"`
	// EstimatedCapacity 扩展后预估容量 (bytes)
	EstimatedCapacity uint64 `json:"estimatedCapacity"`
	// StartTime 开始时间
	StartTime time.Time `json:"startTime"`
	// EndTime 结束时间
	EndTime *time.Time `json:"endTime,omitempty"`
	// ErrorMessage 错误信息
	ErrorMessage string `json:"errorMessage,omitempty"`
	// BytesReshuffled 已重分布数据量 (bytes)
	BytesReshuffled uint64 `json:"bytesReshuffled"`
	// TotalBytesToReshuffle 需重分布总数据量 (bytes)
	TotalBytesToReshuffle uint64 `json:"totalBytesToReshuffle"`
	// ETA 预计剩余时间
	ETA *time.Duration `json:"eta,omitempty"`
}

// PoolInfo 存储池信息（扩展相关）
type PoolInfo struct {
	// Name 池名称
	Name string `json:"name"`
	// RaidzLevel RAID-Z 等级
	RaidzLevel RaidzLevel `json:"raidzLevel"`
	// DiskCount 当前磁盘数
	DiskCount int `json:"diskCount"`
	// TotalCapacity 总容量 (bytes)
	TotalCapacity uint64 `json:"totalCapacity"`
	// UsedCapacity 已用容量 (bytes)
	UsedCapacity uint64 `json:"usedCapacity"`
	// FreeCapacity 可用容量 (bytes)
	FreeCapacity uint64 `json:"freeCapacity"`
	// IsExpanding 是否正在扩展
	IsExpanding bool `json:"isExpanding"`
	// ExpansionTaskID 当前扩展任务ID
	ExpansionTaskID string `json:"expansionTaskId,omitempty"`
	// Disks 磁盘列表
	Disks []*DiskInfo `json:"disks"`
	// Health 健康状态
	Health string `json:"health"`
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	// Device 设备路径
	Device string `json:"device"`
	// Serial 序列号
	Serial string `json:"serial"`
	// Model 型号
	Model string `json:"model"`
	// SizeBytes 容量 (bytes)
	SizeBytes uint64 `json:"sizeBytes"`
	// Role 角色
	Role DiskRole `json:"role"`
	// Healthy 是否健康
	Healthy bool `json:"healthy"`
	// Temperature 温度 (摄氏度)
	Temperature int `json:"temperature"`
	// Pool 所属池
	Pool string `json:"pool,omitempty"`
}

// ValidationResult 验证结果
type ValidationResult struct {
	// Valid 是否通过验证
	Valid bool `json:"valid"`
	// Errors 错误列表
	Errors []ValidationError `json:"errors,omitempty"`
	// Warnings 警告列表
	Warnings []ValidationWarning `json:"warnings,omitempty"`
	// DiskCompatibility 磁盘兼容性检查结果
	DiskCompatibility []*DiskCompatibilityResult `json:"diskCompatibility,omitempty"`
}

// ValidationError 验证错误
type ValidationError struct {
	// Code 错误代码
	Code string `json:"code"`
	// Message 错误消息
	Message string `json:"message"`
	// Disk 相关磁盘（可选）
	Disk string `json:"disk,omitempty"`
}

// ValidationWarning 验证警告
type ValidationWarning struct {
	// Code 警告代码
	Code string `json:"code"`
	// Message 警告消息
	Message string `json:"message"`
}

// DiskCompatibilityResult 磁盘兼容性结果
type DiskCompatibilityResult struct {
	// Device 设备路径
	Device string `json:"device"`
	// Compatible 是否兼容
	Compatible bool `json:"compatible"`
	// SizeBytes 磁盘容量
	SizeBytes uint64 `json:"sizeBytes"`
	// MinRequiredSize 最小要求容量
	MinRequiredSize uint64 `json:"minRequiredSize"`
	// Reason 不兼容原因
	Reason string `json:"reason,omitempty"`
}

// RollbackResult 回滚结果
type RollbackResult struct {
	// TaskID 原任务ID
	TaskID string `json:"taskId"`
	// Success 是否成功
	Success bool `json:"success"`
	// Message 消息
	Message string `json:"message"`
	// Timestamp 回滚时间
	Timestamp time.Time `json:"timestamp"`
}

// ExpansionConfig 扩展配置
type ExpansionConfig struct {
	// MaxConcurrentReshuffles 最大并发重分布数
	MaxConcurrentReshuffles int `json:"maxConcurrentReshuffles"`
	// ReshufflePriority 重分布优先级 (1-10, 10最高)
	ReshufflePriority int `json:"reshufflePriority"`
	// ChecksumVerification 校验和验证
	ChecksumVerification bool `json:"checksumVerification"`
	// AutoRollbackOnFailure 失败时自动回滚
	AutoRollbackOnFailure bool `json:"autoRollbackOnFailure"`
	// ProgressUpdateIntervalSec 进度更新间隔（秒）
	ProgressUpdateIntervalSec int `json:"progressUpdateIntervalSec"`
}

// DefaultExpansionConfig 默认扩展配置
func DefaultExpansionConfig() *ExpansionConfig {
	return &ExpansionConfig{
		MaxConcurrentReshuffles:   2,
		ReshufflePriority:         5,
		ChecksumVerification:      true,
		AutoRollbackOnFailure:     true,
		ProgressUpdateIntervalSec: 5,
	}
}
