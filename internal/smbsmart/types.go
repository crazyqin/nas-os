// Package smbsmart 提供 SMB 多通道优化功能
// 实现多通道聚合、自动故障转移、带宽监控
package smbsmart

import (
	"time"
)

// ========== 通道类型 ==========

// ChannelStatus 通道状态
type ChannelStatus string

const (
	// ChannelStatusActive 活跃通道
	ChannelStatusActive ChannelStatus = "active"
	// ChannelStatusStandby 待命通道
	ChannelStatusStandby ChannelStatus = "standby"
	// ChannelStatusFailed 故障通道
	ChannelStatusFailed ChannelStatus = "failed"
	// ChannelStatusDisabled 已禁用通道
	ChannelStatusDisabled ChannelStatus = "disabled"
)

// ChannelType 通道类型
type ChannelType string

const (
	// ChannelTypeTCP TCP通道
	ChannelTypeTCP ChannelType = "tcp"
	// ChannelTypeRDMA RDMA通道
	ChannelTypeRDMA ChannelType = "rdma"
	// ChannelTypeMultichannel SMB多通道
	ChannelTypeMultichannel ChannelType = "multichannel"
)

// SMBChannel SMB通道信息
type SMBChannel struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Type          ChannelType   `json:"type"`
	Status        ChannelStatus `json:"status"`
	LocalIP       string        `json:"local_ip"`
	RemoteIP      string        `json:"remote_ip"`
	LocalPort     int           `json:"local_port"`
	RemotePort    int           `json:"remote_port"`
	InterfaceName string        `json:"interface_name"` // 网卡名称
	Speed         int64         `json:"speed"`          // 链路速度 (bps)
	MTU           int           `json:"mtu"`
	RSS           bool          `json:"rss"`         // 接收端缩放
	RDMA          bool          `json:"rdma"`        // 是否RDMA通道
	Credits       int           `json:"credits"`     // SMB信用数
	MaxCredits    int           `json:"max_credits"` // 最大信用数
	ReadBytes     int64         `json:"read_bytes"`  // 累计读取字节
	WriteBytes    int64         `json:"write_bytes"` // 累计写入字节
	ReadIOPS      int64         `json:"read_iops"`   // 读IOPS
	WriteIOPS     int64         `json:"write_iops"`  // 写IOPS
	LatencyMs     float64       `json:"latency_ms"`  // 延迟
	ErrorCount    int64         `json:"error_count"` // 错误数
	LastError     string        `json:"last_error"`  // 最后错误
	ConnectedAt   time.Time     `json:"connected_at"`
	LastActivity  time.Time     `json:"last_activity"`
}

// ========== 会话类型 ==========

// SessionStatus 会话状态
type SessionStatus string

const (
	// SessionStatusActive 活跃会话
	SessionStatusActive SessionStatus = "active"
	// SessionStatusDisconnected 已断开
	SessionStatusDisconnected SessionStatus = "disconnected"
	// SessionStatusExpired 已过期
	SessionStatusExpired SessionStatus = "expired"
)

// SMBSession SMB会话信息
type SMBSession struct {
	ID             string        `json:"id"`
	Username       string        `json:"username"`
	ClientIP       string        `json:"client_ip"`
	ClientName     string        `json:"client_name"`
	ServerIP       string        `json:"server_ip"`
	Dialect        string        `json:"dialect"`         // SMB协议版本 (3.1.1等)
	Signing        bool          `json:"signing"`         // 是否签名
	Encryption     bool          `json:"encryption"`      // 是否加密
	Channels       []string      `json:"channels"`        // 关联的通道ID列表
	ChannelCount   int           `json:"channel_count"`   // 通道数
	TotalBandwidth int64         `json:"total_bandwidth"` // 总带宽 (bytes/s)
	OpenFiles      int           `json:"open_files"`      // 打开的文件数
	OpenShares     int           `json:"open_shares"`     // 打开的共享数
	Status         SessionStatus `json:"status"`
	ConnectedAt    time.Time     `json:"connected_at"`
	LastActivity   time.Time     `json:"last_activity"`
	BytesRead      int64         `json:"bytes_read"`
	BytesWritten   int64         `json:"bytes_written"`
}

// ========== 通道绑定 ==========

// BondMode 绑定模式
type BondMode string

const (
	// BondModeRoundRobin 轮询模式
	BondModeRoundRobin BondMode = "round_robin"
	// BondModeActiveBackup 主备模式
	BondModeActiveBackup BondMode = "active_backup"
	// BondModeBalanceXOR XOR均衡模式
	BondModeBalanceXOR BondMode = "balance_xor"
	// BondModeBalanceSLB SLB均衡模式
	BondModeBalanceSLB BondMode = "balance_slb"
	// BondModeAdaptive 自适应模式
	BondModeAdaptive BondMode = "adaptive"
)

// ChannelBond 通道绑定配置
type ChannelBond struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Mode            BondMode  `json:"mode"`
	ChannelIDs      []string  `json:"channel_ids"`       // 绑定的通道ID列表
	ActiveChannelID string    `json:"active_channel_id"` // 当前活跃通道（主备模式）
	Enabled         bool      `json:"enabled"`
	TotalSpeed      int64     `json:"total_speed"`    // 聚合带宽 (bps)
	CurrentSpeed    int64     `json:"current_speed"`  // 当前实际带宽 (bps)
	Utilization     float64   `json:"utilization"`    // 带宽利用率 (0-100%)
	FailoverCount   int64     `json:"failover_count"` // 故障转移次数
	LastFailover    time.Time `json:"last_failover,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ========== 带宽统计 ==========

// BandwidthStats 带宽统计信息
type BandwidthStats struct {
	Timestamp      time.Time                 `json:"timestamp"`
	TotalReadBps   int64                     `json:"total_read_bps"`  // 总读带宽 (bytes/s)
	TotalWriteBps  int64                     `json:"total_write_bps"` // 总写带宽 (bytes/s)
	TotalBps       int64                     `json:"total_bps"`       // 总带宽
	TotalReadIOPS  int64                     `json:"total_read_iops"`
	TotalWriteIOPS int64                     `json:"total_write_iops"`
	AvgLatencyMs   float64                   `json:"avg_latency_ms"`
	ChannelStats   map[string]ChannelBwStats `json:"channel_stats"` // 每通道统计
	ActiveChannels int                       `json:"active_channels"`
	TotalChannels  int                       `json:"total_channels"`
	PeakReadBps    int64                     `json:"peak_read_bps"`
	PeakWriteBps   int64                     `json:"peak_write_bps"`
}

// ChannelBwStats 单通道带宽统计
type ChannelBwStats struct {
	ChannelID   string  `json:"channel_id"`
	ReadBps     int64   `json:"read_bps"`
	WriteBps    int64   `json:"write_bps"`
	TotalBps    int64   `json:"total_bps"`
	ReadIOPS    int64   `json:"read_iops"`
	WriteIOPS   int64   `json:"write_iops"`
	LatencyMs   float64 `json:"latency_ms"`
	Utilization float64 `json:"utilization"` // 通道利用率
}

// ========== 故障转移配置 ==========

// FailoverConfig 故障转移配置
type FailoverConfig struct {
	ID                  string        `json:"id"`
	Enabled             bool          `json:"enabled"`
	HealthCheckInterval time.Duration `json:"health_check_interval"` // 健康检查间隔
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`  // 健康检查超时
	FailureThreshold    int           `json:"failure_threshold"`     // 连续失败阈值
	RecoveryThreshold   int           `json:"recovery_threshold"`    // 恢复阈值
	AutoRebalance       bool          `json:"auto_rebalance"`        // 自动重平衡
	RebalanceThreshold  float64       `json:"rebalance_threshold"`   // 重平衡触发阈值 (利用率差异%)
	NotificationEnabled bool          `json:"notification_enabled"`  // 故障通知
	WebhookURL          string        `json:"webhook_url,omitempty"` // 通知webhook
	UpdatedAt           time.Time     `json:"updated_at"`
}

// DefaultFailoverConfig 默认故障转移配置
func DefaultFailoverConfig() FailoverConfig {
	return FailoverConfig{
		Enabled:             true,
		HealthCheckInterval: 10 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		FailureThreshold:    3,
		RecoveryThreshold:   2,
		AutoRebalance:       true,
		RebalanceThreshold:  30.0,
		NotificationEnabled: false,
	}
}

// ========== 请求/响应类型 ==========

// BondChannelsRequest 通道绑定请求
type BondChannelsRequest struct {
	Name       string   `json:"name" binding:"required"`
	Mode       BondMode `json:"mode" binding:"required"`
	ChannelIDs []string `json:"channel_ids" binding:"required,min=2"`
}

// UpdateFailoverConfigRequest 更新故障转移配置请求
type UpdateFailoverConfigRequest struct {
	Enabled             *bool    `json:"enabled,omitempty"`
	HealthCheckInterval string   `json:"health_check_interval,omitempty"` // "10s", "30s" 等
	HealthCheckTimeout  string   `json:"health_check_timeout,omitempty"`
	FailureThreshold    *int     `json:"failure_threshold,omitempty"`
	RecoveryThreshold   *int     `json:"recovery_threshold,omitempty"`
	AutoRebalance       *bool    `json:"auto_rebalance,omitempty"`
	RebalanceThreshold  *float64 `json:"rebalance_threshold,omitempty"`
	NotificationEnabled *bool    `json:"notification_enabled,omitempty"`
	WebhookURL          *string  `json:"webhook_url,omitempty"`
}
