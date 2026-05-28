// Package sasmultipath 提供 SAS 多路径管理的数据结构定义
package sasmultipath

import (
	"time"
)

// PathStatus 路径状态
type PathStatus string

const (
	// PathActive 活跃路径
	PathActive PathStatus = "active"
	// PathStandby 备用路径
	PathStandby PathStatus = "standby"
	// PathFailed 故障路径
	PathFailed PathStatus = "failed"
	// PathRemoved 已移除
	PathRemoved PathStatus = "removed"
)

// LoadBalancePolicy 负载均衡策略
type LoadBalancePolicy string

const (
	// PolicyRoundRobin 轮询
	PolicyRoundRobin LoadBalancePolicy = "round-robin"
	// PolicyLeastPending 最少待处理
	PolicyLeastPending LoadBalancePolicy = "least-pending"
)

// SASDevice SAS 设备
type SASDevice struct {
	// WWN 设备全球唯一标识
	WWN string `json:"wwn"`
	// Model 型号
	Model string `json:"model"`
	// Serial 序列号
	Serial string `json:"serial"`
	// Paths 该设备的所有路径
	Paths []*Path `json:"paths"`
	// ActivePath 当前活跃路径
	ActivePath *Path `json:"activePath"`
	// Policy 负载均衡策略
	Policy LoadBalancePolicy `json:"policy"`
	// Status 设备整体状态
	Status DeviceStatus `json:"status"`
	// LastFailover 最后一次故障切换时间
	LastFailover *time.Time `json:"lastFailover,omitempty"`
}

// DeviceStatus 设备状态
type DeviceStatus string

const (
	// DeviceHealthy 设备健康
	DeviceHealthy DeviceStatus = "healthy"
	// DeviceDegraded 设备降级（部分路径故障）
	DeviceDegraded DeviceStatus = "degraded"
	// DeviceFailed 设备故障（所有路径不可用）
	DeviceFailed DeviceStatus = "failed"
)

// Path 单条路径
type Path struct {
	// ID 路径标识（host:channel:id:lun）
	ID string `json:"id"`
	// HostAdapter 主机适配器编号
	HostAdapter int `json:"hostAdapter"`
	// Channel 通道号
	Channel int `json:"channel"`
	// TargetID 目标 ID
	TargetID int `json:"targetId"`
	// LUN 逻辑单元号
	LUN int `json:"lun"`
	// DeviceNode 设备节点（如 /dev/sda）
	DeviceNode string `json:"deviceNode"`
	// SASAddress SAS 地址
	SASAddress string `json:"sasAddress"`
	// Controller 控制器标识
	Controller string `json:"controller"`
	// Status 路径状态
	Status PathStatus `json:"status"`
	// IOPs 当前 IOPS
	IOPs int64 `json:"iops"`
	// PendingIOs 待处理 IO 数
	PendingIOs int64 `json:"pendingIOs"`
	// LatencyMs 平均延迟（毫秒）
	LatencyMs float64 `json:"latencyMs"`
	// ErrorCount 错误计数
	ErrorCount int64 `json:"errorCount"`
	// LastHealthCheck 最后健康检查时间
	LastHealthCheck time.Time `json:"lastHealthCheck"`
}

// FailoverEvent 故障切换事件
type FailoverEvent struct {
	// Timestamp 事件时间
	Timestamp time.Time `json:"timestamp"`
	// DeviceWWN 设备 WWN
	DeviceWWN string `json:"deviceWwn"`
	// FromPath 源路径 ID
	FromPath string `json:"fromPath"`
	// ToPath 目标路径 ID
	ToPath string `json:"toPath"`
	// Reason 切换原因
	Reason string `json:"reason"`
	// Duration 切换耗时（毫秒）
	DurationMs int64 `json:"durationMs"`
}

// HealthCheckResult 路径健康检查结果
type HealthCheckResult struct {
	// PathID 路径 ID
	PathID string `json:"pathId"`
	// Healthy 是否健康
	Healthy bool `json:"healthy"`
	// LatencyMs 延迟（毫秒）
	LatencyMs float64 `json:"latencyMs"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
	// CheckedAt 检查时间
	CheckedAt time.Time `json:"checkedAt"`
}

// ManualFailoverRequest 手动故障切换请求
type ManualFailoverRequest struct {
	// DeviceWWN 设备 WWN
	DeviceWWN string `json:"deviceWwn"`
	// TargetPathID 目标路径 ID
	TargetPathID string `json:"targetPathId"`
}

// PolicyUpdateRequest 策略更新请求
type PolicyUpdateRequest struct {
	// DeviceWWN 设备 WWN
	DeviceWWN string `json:"deviceWwn"`
	// Policy 新策略
	Policy LoadBalancePolicy `json:"policy"`
}
