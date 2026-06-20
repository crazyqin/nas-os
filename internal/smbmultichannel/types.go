// Package smbmultichannel 提供 SMB Multichannel 支持，对标 TrueNAS 25.04 的多通道功能
// 统一类型定义，支持多通道连接管理、带宽聚合、故障转移、性能监控
package smbmultichannel

import (
	"time"
)

// ========== 会话与通道状态枚举 ==========

// SessionState 会话状态
type SessionState string

const (
	SessionStateActive   SessionState = "active"
	SessionStateInactive SessionState = "inactive"
	SessionStateDegraded SessionState = "degraded"
	SessionStateClosed   SessionState = "closed"
)

// ChannelState 通道状态
type ChannelState string

const (
	ChannelStateActive  ChannelState = "active"
	ChannelStateStandby ChannelState = "standby"
	ChannelStateFailed  ChannelState = "failed"
	ChannelStateClosed  ChannelState = "closed"
)

// LoadBalanceAlgo 负载均衡算法
type LoadBalanceAlgo string

const (
	LoadBalanceRoundRobin LoadBalanceAlgo = "round_robin"
	LoadBalanceLeastConn  LoadBalanceAlgo = "least_conn"
	LoadBalanceBandwidth  LoadBalanceAlgo = "bandwidth"
	LoadBalanceLatency    LoadBalanceAlgo = "latency"
	LoadBalanceAdaptive   LoadBalanceAlgo = "adaptive"
)

// ValidLoadBalanceModes 有效的负载均衡模式集合
var ValidLoadBalanceModes = map[string]bool{
	"round-robin": true,
	"weighted":    true,
	"hash":        true,
	string(LoadBalanceRoundRobin): true,
	string(LoadBalanceLeastConn):  true,
	string(LoadBalanceBandwidth):  true,
	string(LoadBalanceLatency):    true,
	string(LoadBalanceAdaptive):   true,
}

// ========== 配置类型 ==========

// ChannelConfig SMB Multichannel 配置
type ChannelConfig struct {
	Enabled         bool     `json:"enabled"`
	MaxChannels     int      `json:"max_channels"`
	InterfaceNames  []string `json:"interface_names"`
	MinSpeed        int      `json:"min_speed"`         // Mbps
	MinBandwidth    int      `json:"min_bandwidth"`     // Mbps, 最小带宽阈值
	LoadBalanceMode string   `json:"load_balance_mode"` // round-robin, weighted, hash, round_robin, least_conn, bandwidth, latency, adaptive
	JumboFrames     bool     `json:"jumbo_frames"`
	RDMAEnabled     bool     `json:"rdma_enabled"`
}

// ManagerConfig 管理器扩展配置
type ManagerConfig struct {
	Enabled              bool            `json:"enabled"`
	MaxChannelsPerClient int             `json:"max_channels_per_client"`
	MaxTotalChannels     int             `json:"max_total_channels"`
	DefaultAlgorithm     LoadBalanceAlgo `json:"default_algorithm"`
	HealthCheckInterval  int             `json:"health_check_interval"` // 秒
	FailoverEnabled      bool            `json:"failover_enabled"`
	AutoRebalance        bool            `json:"auto_rebalance"`
	RebalanceThreshold   float64         `json:"rebalance_threshold"` // 负载不均衡阈值 0.0-1.0
	EncryptionEnabled    bool            `json:"encryption_enabled"`
	CompressionEnabled   bool            `json:"compression_enabled"`
	SigningEnabled       bool            `json:"signing_enabled"`
	MaxSMBVersion        string          `json:"max_smb_version"` // 2.0, 2.1, 3.0, 3.1.1
	MinSMBVersion        string          `json:"min_smb_version"`
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Enabled         *bool    `json:"enabled,omitempty"`
	MaxChannels     *int     `json:"max_channels,omitempty"`
	InterfaceNames  []string `json:"interface_names,omitempty"`
	MinSpeed        *int     `json:"min_speed,omitempty"`
	MinBandwidth    *int     `json:"min_bandwidth,omitempty"`
	LoadBalanceMode *string  `json:"load_balance_mode,omitempty"`
	JumboFrames     *bool    `json:"jumbo_frames,omitempty"`
	RDMAEnabled     *bool    `json:"rdma_enabled,omitempty"`
}

// SetLoadBalanceModeRequest 设置负载均衡模式请求
type SetLoadBalanceModeRequest struct {
	Mode string `json:"mode"` // round-robin, weighted, hash, round_robin, least_conn, bandwidth, latency, adaptive
}

// ========== 通道类型 ==========

// ChannelStatus 通道状态信息（面向外部API）
type ChannelStatus struct {
	InterfaceName    string    `json:"interface_name"`
	Speed            int       `json:"speed"` // Mbps
	Active           bool      `json:"active"`
	Connections      int       `json:"connections"`
	BytesTransferred int64     `json:"bytes_transferred"`
	LastActive       time.Time `json:"last_active"`
}

// ChannelInfo 通道信息（内部使用）
type ChannelInfo struct {
	Status     ChannelStatus
	Enabled    bool
	TotalBytes int64
}

// Channel 详细通道信息（会话内使用）
type Channel struct {
	ID            string       `json:"id"`
	InterfaceName string       `json:"interface_name"`
	LocalAddr     string       `json:"local_addr"`
	RemoteAddr    string       `json:"remote_addr"`
	State         ChannelState `json:"state"`
	Speed         int64        `json:"speed"` // Mbps
	Stats         ChannelStats `json:"stats"`
	LastActive    time.Time    `json:"last_active"`
}

// ChannelRef 会话中的通道引用（轻量级）
type ChannelRef struct {
	InterfaceName string `json:"interface_name"`
	Speed         int    `json:"speed"` // Mbps
	Active        bool   `json:"active"`
}

// ChannelStats 通道统计信息
type ChannelStats struct {
	ActiveChannels      int            `json:"active_channels"`
	TotalBandwidth      int            `json:"total_bandwidth"` // Mbps
	PerChannelBandwidth map[string]int `json:"per_channel_bandwidth"`
	ErrorCount          int64          `json:"error_count"`
	ReconnectCount      int64          `json:"reconnect_count"`
	BytesSent           int64          `json:"bytes_sent"`
	BytesReceived       int64          `json:"bytes_received"`
	OpsSent             int64          `json:"ops_sent"`
	OpsReceived         int64          `json:"ops_received"`
	AvgLatencyMs        float64        `json:"avg_latency_ms"`
	ThroughputMBps      float64        `json:"throughput_mbps"`
	Errors              int64          `json:"errors"`
	LastActive          time.Time      `json:"last_active"`
	LoadBalanceCount    int64          `json:"load_balance_count"`   // 负载均衡次数
	FailoverCount       int64          `json:"failover_count"`       // 故障转移次数
}

// ChannelHealth 通道健康状态
type ChannelHealth struct {
	ChannelID  string    `json:"channel_id"`
	Status     string    `json:"status"`      // up, down, degraded
	Latency    int64     `json:"latency"`     // ms
	PacketLoss float64   `json:"packet_loss"` // 0.0-100.0
	LastCheck  time.Time `json:"last_check"`
}

// ========== 会话类型 ==========

// MultichannelSession Multichannel 会话
type MultichannelSession struct {
	ID               string            `json:"id"`
	ClientIP         string            `json:"client_ip"`
	ServerIP         string            `json:"server_ip"`
	Channels         []ChannelRef      `json:"channels"`
	State            SessionState      `json:"state"`
	MaxChannels      int               `json:"max_channels"`
	Algorithm        LoadBalanceAlgo   `json:"algorithm"`
	TotalSpeed       int               `json:"total_speed"` // Mbps
	StartTime        time.Time         `json:"start_time"`
	BytesTransferred int64             `json:"bytes_transferred"`
	Protocol         string            `json:"protocol"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// ========== 统计类型 ==========

// ThroughputStats 吞吐量统计
type ThroughputStats struct {
	TotalDownload  int64     `json:"total_download"` // bytes
	TotalUpload    int64     `json:"total_upload"`   // bytes
	AvgSpeed       int       `json:"avg_speed"`      // Mbps
	PeakSpeed      int       `json:"peak_speed"`     // Mbps
	ActiveSessions int       `json:"active_sessions"`
	ActiveChannels int       `json:"active_channels"`
	LastUpdated    time.Time `json:"last_updated"`
}

// BandwidthHistoryItem 带宽历史记录
type BandwidthHistoryItem struct {
	Timestamp time.Time `json:"timestamp"`
	Download  int64     `json:"download"` // bytes
	Upload    int64     `json:"upload"`   // bytes
	Speed     int       `json:"speed"`    // Mbps
	Sessions  int       `json:"sessions"`
}

// SessionStats 会话统计
type SessionStats struct {
	SessionID       string `json:"session_id"`
	ClientIP        string `json:"client_ip"`
	TotalBytes      int64  `json:"total_bytes"`
	ChannelCount    int    `json:"channel_count"`
	AvgChannelSpeed int    `json:"avg_channel_speed"` // Mbps
	Duration        int64  `json:"duration"`          // seconds
}

// ManagerStats 管理器全局统计
type ManagerStats struct {
	TotalSessions     int       `json:"total_sessions"`
	ActiveSessions    int       `json:"active_sessions"`
	TotalChannels     int       `json:"total_channels"`
	ActiveChannels    int       `json:"active_channels"`
	TotalThroughputGB float64   `json:"total_throughput_gb"`
	AvgLatencyMs      float64   `json:"avg_latency_ms"`
	LoadBalanceCount  int64     `json:"load_balance_count"`
	FailoverCount     int64     `json:"failover_count"`
	LastRebalance     time.Time `json:"last_rebalance"`
}

// ========== 网络与重平衡类型 ==========

// NetworkInterface 网络接口
type NetworkInterface struct {
	Name         string   `json:"name"`
	MTU          int      `json:"mtu"`
	HardwareAddr string   `json:"hardware_addr"`
	Addresses    []string `json:"addresses"`
	Speed        int64    `json:"speed"` // Mbps
}

// RebalanceResult 重平衡结果
type RebalanceResult struct {
	Timestamp       time.Time         `json:"timestamp"`
	RebalancedCount int               `json:"rebalanced_count"`
	Details         map[string]string `json:"details"`
}

// ========== 审计类型 ==========

// AuditEntry 审计日志条目
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	User      string    `json:"user"`
	ClientIP  string    `json:"client_ip"`
	Details   string    `json:"details"`
}

// ========== 响应类型 ==========

// ChannelStatsResponse 通道统计响应
type ChannelStatsResponse struct {
	Stats *ChannelStats `json:"stats"`
}

// ChannelHealthResponse 通道健康响应
type ChannelHealthResponse struct {
	Health []ChannelHealth `json:"health"`
}

// AuditLogResponse 审计日志响应
type AuditLogResponse struct {
	Total   int          `json:"total"`
	Entries []AuditEntry `json:"entries"`
}

// EnableDisableResponse 启用/禁用响应
type EnableDisableResponse struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
}
