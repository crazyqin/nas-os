// Package storageqos - 存储QoS（Quality of Service）管理
// 对标TrueNAS QoS功能，提供I/O优先级控制、带宽限制、突发流量管理
package storageqos

import (
	"time"
)

// ============================================================
// I/O优先级 - 对标TrueNAS I/O调度
// ============================================================

// IOPriority I/O优先级
// 对标Linux ionice和TrueNAS I/O调度策略
type IOPriority string

const (
	// IOPriorityHigh 高优先级 - 关键业务、数据库
	IOPriorityHigh IOPriority = "high"
	// IOPriorityNormal 普通优先级 - 默认
	IOPriorityNormal IOPriority = "normal"
	// IOPriorityLow 低优先级 - 备份、归档
	IOPriorityLow IOPriority = "low"
)

// ============================================================
// QoS策略类型
// ============================================================

// BandwidthLimit 带宽限制配置
// 对标TrueNAS ZFS dataset quotas和share limits
type BandwidthLimit struct {
	// ReadBPSLimit 读带宽限制 (MB/s), 0表示不限制
	ReadBPSLimit float64 `json:"read_bps_limit"`
	// WriteBPSLimit 写带宽限制 (MB/s), 0表示不限制
	WriteBPSLimit float64 `json:"write_bps_limit"`
	// ReadIOPSLimit 读IOPS限制, 0表示不限制
	ReadIOPSLimit int64 `json:"read_iops_limit"`
	// WriteIOPSLimit 写IOPS限制, 0表示不限制
	WriteIOPSLimit int64 `json:"write_iops_limit"`
}

// BurstConfig 突发流量配置
// 允许短时间内超过限制，类似令牌桶算法
type BurstConfig struct {
	// BurstEnabled 是否启用突发流量
	BurstEnabled bool `json:"burst_enabled"`
	// BurstSizeMB 突发大小 (MB), 超过此值后限速生效
	BurstSizeMB float64 `json:"burst_size_mb"`
	// BurstDurationSec 突发持续时间 (秒)
	BurstDurationSec int `json:"burst_duration_sec"`
	// BurstReplenishRateMB 突发容量补充速率 (MB/s)
	BurstReplenishRateMB float64 `json:"burst_replenish_rate_mb"`
}

// QoSPolicy QoS策略定义
// 对标TrueNAS dataset QoS策略，支持多维度限速
type QoSPolicy struct {
	// ID 策略唯一标识
	ID string `json:"id"`
	// Name 策略名称
	Name string `json:"name"`
	// Description 策略描述
	Description string `json:"description,omitempty"`
	// Target 目标资源（dataset路径、share名称或IP范围）
	Target string `json:"target"`
	// TargetType 目标类型: "dataset", "share", "ip_range"
	TargetType string `json:"target_type"`
	// Priority I/O优先级
	Priority IOPriority `json:"priority"`
	// Bandwidth 带宽限制
	Bandwidth BandwidthLimit `json:"bandwidth"`
	// Burst 突发流量配置
	Burst BurstConfig `json:"burst"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// ============================================================
// QoS监控指标
// ============================================================

// QoSMetrics QoS实时监控指标
// 对标TrueNAS reporting功能中的I/O统计
type QoSMetrics struct {
	// PolicyID 关联的策略ID
	PolicyID string `json:"policy_id"`
	// Target 监控目标
	Target string `json:"target"`
	// CurrentReadBPS 当前读带宽 (MB/s)
	CurrentReadBPS float64 `json:"current_read_bps"`
	// CurrentWriteBPS 当前写带宽 (MB/s)
	CurrentWriteBPS float64 `json:"current_write_bps"`
	// CurrentReadIOPS 当前读IOPS
	CurrentReadIOPS int64 `json:"current_read_iops"`
	// CurrentWriteIOPS 当前写IOPS
	CurrentWriteIOPS int64 `json:"current_write_iops"`
	// ThrottledReadBytes 被限速的读字节数 (累计)
	ThrottledReadBytes int64 `json:"throttled_read_bytes"`
	// ThrottledWriteBytes 被限速的写字节数 (累计)
	ThrottledWriteBytes int64 `json:"throttled_write_bytes"`
	// ThrottleEvents 限速触发次数 (累计)
	ThrottleEvents int64 `json:"throttle_events"`
	// BurstUsedMB 当前已使用的突发容量 (MB)
	BurstUsedMB float64 `json:"burst_used_mb"`
	// LatencyMs 当前平均延迟 (ms)
	LatencyMs float64 `json:"latency_ms"`
	// Timestamp 指标采集时间
	Timestamp time.Time `json:"timestamp"`
}

// QoSMetricsHistory 历史指标查询结果
type QoSMetricsHistory struct {
	PolicyID string       `json:"policy_id"`
	Target   string       `json:"target"`
	From     time.Time    `json:"from"`
	To       time.Time    `json:"to"`
	Samples  []QoSMetrics `json:"samples"`
}

// ============================================================
// 请求/响应类型
// ============================================================

// CreatePolicyRequest 创建策略请求
type CreatePolicyRequest struct {
	Name        string      `json:"name" binding:"required"`
	Description string      `json:"description"`
	Target      string      `json:"target" binding:"required"`
	TargetType  string      `json:"target_type" binding:"required,oneof=dataset share ip_range"`
	Priority    IOPriority  `json:"priority" binding:"required,oneof=high normal low"`
	Bandwidth   BandwidthLimit `json:"bandwidth"`
	Burst       BurstConfig    `json:"burst"`
	Enabled     *bool       `json:"enabled"`
}

// UpdatePolicyRequest 更新策略请求
type UpdatePolicyRequest struct {
	Name        *string        `json:"name"`
	Description *string        `json:"description"`
	Priority    *IOPriority    `json:"priority"`
	Bandwidth   *BandwidthLimit `json:"bandwidth"`
	Burst       *BurstConfig    `json:"burst"`
	Enabled     *bool          `json:"enabled"`
}

// PolicyListResponse 策略列表响应
type PolicyListResponse struct {
	Policies []QoSPolicy `json:"policies"`
	Total    int         `json:"total"`
}

// MetricsResponse 指标响应
type MetricsResponse struct {
	PolicyID string     `json:"policy_id"`
	Target   string     `json:"target"`
	Metrics  QoSMetrics `json:"metrics"`
}
