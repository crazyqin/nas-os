// Package gpuscheduler 提供 GPU 容器调度与资源管理功能
package gpuscheduler

import (
	"fmt"
	"time"
)

// ========== GPU 设备核心类型 ==========

// GPUDevice GPU 设备信息
type GPUDevice struct {
	ID             string        `json:"id"`               // 设备唯一标识（如 GPU-xxxx）
	Index          int           `json:"index"`            // 设备索引（nvidia-smi 中的序号）
	Name           string        `json:"name"`             // 设备名称（如 NVIDIA GeForce RTX 4090）
	UUID           string        `json:"uuid"`             // 设备 UUID
	MemoryTotal    int64         `json:"memory_total"`     // 显存总量（MiB）
	MemoryUsed     int64         `json:"memory_used"`      // 显存已用（MiB）
	MemoryFree     int64         `json:"memory_free"`      // 显存空闲（MiB）
	Temperature    int           `json:"temperature"`      // 温度（℃）
	PowerDraw      float64       `json:"power_draw"`       // 当前功耗（W）
	PowerLimit     float64       `json:"power_limit"`      // 功耗上限（W）
	ComputeCap     string        `json:"compute_cap"`      // 计算能力（如 8.9）
	DriverVersion  string        `json:"driver_version"`   // 驱动版本
	UtilizationGPU int           `json:"utilization_gpu"`  // GPU 利用率（%）
	UtilizationMem int           `json:"utilization_mem"`  // 显存利用率（%）
	Status         DeviceStatus  `json:"status"`           // 设备状态
	Allocations    []*GPUAllocation `json:"allocations"`   // 当前分配列表
	ReservedMiB    int64         `json:"reserved_mib"`     // 预留显存（MiB）
	Labels         map[string]string `json:"labels,omitempty"` // 设备标签
	UpdatedAt      time.Time     `json:"updated_at"`       // 最后更新时间
}

// DeviceStatus GPU 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline      DeviceStatus = "online"       // 正常运行
	DeviceStatusOffline     DeviceStatus = "offline"      // 离线
	DeviceStatusError       DeviceStatus = "error"        // 错误状态
	DeviceStatusOverheat    DeviceStatus = "overheat"     // 过热
	DeviceStatusMaintenance DeviceStatus = "maintenance"  // 维护模式
)

// ========== GPU 分配记录 ==========

// GPUAllocation GPU 分配记录
type GPUAllocation struct {
	ID           string           `json:"id"`            // 分配记录 ID
	DeviceID     string           `json:"device_id"`     // GPU 设备 ID
	ContainerID  string           `json:"container_id"`  // 容器 ID
	ContainerName string          `json:"container_name"` // 容器名称
	VMID         string           `json:"vm_id,omitempty"` // VM ID（可选）
	MemoryMiB    int64            `json:"memory_mib"`    // 分配显存（MiB）
	Priority     Priority         `json:"priority"`      // 优先级
	Status       AllocationStatus `json:"status"`        // 分配状态
	Constraint   *AffinityConstraint `json:"constraint,omitempty"` // 亲和性约束
	CreatedAt    time.Time        `json:"created_at"`    // 创建时间
	ExpiresAt    *time.Time       `json:"expires_at,omitempty"` // 过期时间
	Labels       map[string]string `json:"labels,omitempty"` // 标签
}

// Priority 分配优先级
type Priority string

const (
	PriorityHigh   Priority = "high"   // 高优先级
	PriorityMedium Priority = "medium" // 中优先级
	PriorityLow    Priority = "low"    // 低优先级
)

// AllocationStatus 分配状态
type AllocationStatus string

const (
	AllocationStatusActive    AllocationStatus = "active"    // 活跃
	AllocationStatusPending   AllocationStatus = "pending"   // 等待中
	AllocationStatusReleased  AllocationStatus = "released"  // 已释放
	AllocationStatusExpired   AllocationStatus = "expired"   // 已过期
	AllocationStatusPreempted AllocationStatus = "preempted" // 被抢占
)

// ========== 调度策略 ==========

// SchedulerPolicy 调度策略
type SchedulerPolicy struct {
	Strategy         SchedulingStrategy `json:"strategy"`           // 调度策略
	PreemptionEnabled bool              `json:"preemption_enabled"` // 是否启用抢占
	ReservedPercent  float64            `json:"reserved_percent"`   // 预留资源百分比
	OvercommitRatio  float64            `json:"overcommit_ratio"`   // 超分配比率
	MaxTemperature   int                `json:"max_temperature"`    // 最大允许温度
	UpdatedAt        time.Time          `json:"updated_at"`         // 更新时间
}

// SchedulingStrategy 调度策略类型
type SchedulingStrategy string

const (
	StrategyRoundRobin    SchedulingStrategy = "round_robin"    // 轮询
	StrategyLeastUsed     SchedulingStrategy = "least_used"     // 最少使用
	StrategyPriority      SchedulingStrategy = "priority"       // 优先级调度
	StrategyAffinity      SchedulingStrategy = "affinity"       // 亲和性调度
	StrategyBinPacking    SchedulingStrategy = "bin_packing"    // 装箱策略
)

// ========== 亲和性约束 ==========

// AffinityConstraint GPU 亲和性约束
type AffinityConstraint struct {
	PreferredDeviceIDs []string `json:"preferred_device_ids,omitempty"` // 偏好设备列表
	ExcludedDeviceIDs  []string `json:"excluded_device_ids,omitempty"`  // 排除设备列表
	DeviceLabelSelector map[string]string `json:"device_label_selector,omitempty"` // 标签选择器
}

// ========== GPU 资源池 ==========

// GPUPool GPU 资源池
type GPUPool struct {
	ID              string             `json:"id"`               // 资源池 ID
	Name            string             `json:"name"`             // 资源池名称
	Description     string             `json:"description,omitempty"` // 描述
	DeviceIDs       []string           `json:"device_ids"`       // 包含的 GPU 设备 ID 列表
	TotalMemoryMiB  int64              `json:"total_memory_mib"` // 总显存（MiB）
	UsedMemoryMiB   int64              `json:"used_memory_mib"`  // 已用显存（MiB）
	FreeMemoryMiB   int64              `json:"free_memory_mib"`  // 空闲显存（MiB）
	AllocationCount int                `json:"allocation_count"` // 分配数量
	Policy          SchedulerPolicy    `json:"policy"`           // 调度策略
	Labels          map[string]string  `json:"labels,omitempty"` // 标签
	CreatedAt       time.Time          `json:"created_at"`       // 创建时间
	UpdatedAt       time.Time          `json:"updated_at"`       // 更新时间
}

// ========== API 请求/响应类型 ==========

// AllocateRequest GPU 分配请求
type AllocateRequest struct {
	ContainerID   string               `json:"container_id" binding:"required"`   // 容器 ID
	ContainerName string               `json:"container_name"`                    // 容器名称
	VMID          string               `json:"vm_id,omitempty"`                   // VM ID
	MemoryMiB     int64                `json:"memory_mib" binding:"required,min=1"` // 请求显存（MiB）
	Priority      Priority             `json:"priority"`                          // 优先级
	Constraint    *AffinityConstraint  `json:"constraint,omitempty"`              // 亲和性约束
	Labels        map[string]string    `json:"labels,omitempty"`                  // 标签
}

// UpdatePolicyRequest 更新调度策略请求
type UpdatePolicyRequest struct {
	Strategy          SchedulingStrategy `json:"strategy"`           // 调度策略
	PreemptionEnabled *bool              `json:"preemption_enabled"` // 是否启用抢占
	ReservedPercent   *float64           `json:"reserved_percent"`   // 预留资源百分比
	OvercommitRatio   *float64           `json:"overcommit_ratio"`   // 超分配比率
	MaxTemperature    *int               `json:"max_temperature"`    // 最大允许温度
}

// GPUStats GPU 使用统计
type GPUStats struct {
	TotalDevices      int                `json:"total_devices"`       // 设备总数
	OnlineDevices     int                `json:"online_devices"`      // 在线设备数
	TotalMemoryMiB    int64              `json:"total_memory_mib"`    // 总显存
	UsedMemoryMiB     int64              `json:"used_memory_mib"`     // 已用显存
	FreeMemoryMiB     int64              `json:"free_memory_mib"`     // 空闲显存
	MemoryUtilization float64            `json:"memory_utilization"`  // 显存利用率（%）
	TotalAllocations  int                `json:"total_allocations"`   // 总分配数
	ActiveAllocations int                `json:"active_allocations"`  // 活跃分配数
	DeviceStats       []DeviceStatsEntry `json:"device_stats"`        // 各设备统计
	Policy            SchedulerPolicy    `json:"policy"`              // 当前策略
	UpdatedAt         time.Time          `json:"updated_at"`         // 更新时间
}

// DeviceStatsEntry 设备统计条目
type DeviceStatsEntry struct {
	DeviceID          string  `json:"device_id"`          // 设备 ID
	Name              string  `json:"name"`               // 设备名称
	Temperature       int     `json:"temperature"`        // 温度
	PowerDraw         float64 `json:"power_draw"`         // 功耗
	MemoryUtilization float64 `json:"memory_utilization"` // 显存利用率
	GPUUtilization    int     `json:"gpu_utilization"`    // GPU 利用率
	AllocationCount   int     `json:"allocation_count"`   // 分配数
}

// ========== 标准响应 ==========

// Response 标准 API 响应
// NotFoundError 资源未找到
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Resource, e.ID)
}

// InsufficientResourceError 资源不足
type InsufficientResourceError struct {
	Message string
}

func (e *InsufficientResourceError) Error() string {
	return fmt.Sprintf("insufficient resource: %s", e.Message)
}

// DeviceOfflineError 设备离线
type DeviceOfflineError struct {
	DeviceID string
}

func (e *DeviceOfflineError) Error() string {
	return fmt.Sprintf("device %q is offline", e.DeviceID)
}

// PolicyViolationError 策略违规
type PolicyViolationError struct {
	Message string
}

func (e *PolicyViolationError) Error() string {
	return fmt.Sprintf("policy violation: %s", e.Message)
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// Response 标准 API 响应
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ========== 健康检查类型 ==========

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Timestamp        time.Time      `json:"timestamp"`         // 检查时间
	OverallStatus    string         `json:"overall_status"`    // 整体状态: healthy/warning/unhealthy
	TotalDevices     int            `json:"total_devices"`     // 设备总数
	HealthyDevices   int            `json:"healthy_devices"`   // 健康设备数
	WarningDevices   int            `json:"warning_devices"`   // 告警设备数
	UnhealthyDevices int            `json:"unhealthy_devices"` // 异常设备数
	Devices          []DeviceHealth `json:"devices"`           // 各设备健康详情
}

// DeviceHealth 设备健康状态
type DeviceHealth struct {
	DeviceID   string            `json:"device_id"`   // 设备 ID
	DeviceName string            `json:"device_name"` // 设备名称
	Status     string            `json:"status"`      // 状态: healthy/warning/unhealthy
	Checks     []HealthCheckItem `json:"checks"`      // 检查项列表
	Issues     []string          `json:"issues,omitempty"` // 问题列表
}

// HealthCheckItem 健康检查项
type HealthCheckItem struct {
	Name    string `json:"name"`    // 检查项名称
	Status  string `json:"status"`  // 状态: healthy/warning/unhealthy
	Value   string `json:"value"`   // 当前值
	Message string `json:"message"` // 描述信息
}
