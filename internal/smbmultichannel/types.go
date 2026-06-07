// Package smbmultichannel 提供 SMB Multichannel 支持，对标 TrueNAS 的多通道功能
package smbmultichannel

import (
	"time"
)

// ChannelConfig SMB Multichannel 配置.
type ChannelConfig struct {
	Enabled         bool     `json:"enabled"`
	MaxChannels     int      `json:"max_channels"`
	InterfaceNames  []string `json:"interface_names"`
	MinSpeed        int      `json:"min_speed"`         // Mbps
	MinBandwidth    int      `json:"min_bandwidth"`     // Mbps, 最小带宽阈值
	LoadBalanceMode string   `json:"load_balance_mode"` // round-robin, weighted, hash
	JumboFrames     bool     `json:"jumbo_frames"`
	RDMAEnabled     bool     `json:"rdma_enabled"`
}

// ChannelStatus 通道状态.
type ChannelStatus struct {
	InterfaceName    string    `json:"interface_name"`
	Speed            int       `json:"speed"` // Mbps
	Active           bool      `json:"active"`
	Connections      int       `json:"connections"`
	BytesTransferred int64     `json:"bytes_transferred"`
	LastActive       time.Time `json:"last_active"`
}

// ChannelInfo 通道信息（内部使用）.
type ChannelInfo struct {
	Status     ChannelStatus
	Enabled    bool
	TotalBytes int64
}

// MultichannelSession Multichannel 会话.
type MultichannelSession struct {
	ID               string       `json:"id"`
	ClientIP         string       `json:"client_ip"`
	ServerIP         string       `json:"server_ip"`
	Channels         []ChannelRef `json:"channels"`
	TotalSpeed       int          `json:"total_speed"` // Mbps
	StartTime        time.Time    `json:"start_time"`
	BytesTransferred int64        `json:"bytes_transferred"`
	Protocol         string       `json:"protocol"`
}

// ChannelRef 会话中的通道引用.
type ChannelRef struct {
	InterfaceName string `json:"interface_name"`
	Speed         int    `json:"speed"` // Mbps
	Active        bool   `json:"active"`
}

// ThroughputStats 吞吐量统计.
type ThroughputStats struct {
	TotalDownload  int64     `json:"total_download"` // bytes
	TotalUpload    int64     `json:"total_upload"`   // bytes
	AvgSpeed       int       `json:"avg_speed"`      // Mbps
	PeakSpeed      int       `json:"peak_speed"`     // Mbps
	ActiveSessions int       `json:"active_sessions"`
	ActiveChannels int       `json:"active_channels"`
	LastUpdated    time.Time `json:"last_updated"`
}

// BandwidthHistoryItem 带宽历史记录.
type BandwidthHistoryItem struct {
	Timestamp time.Time `json:"timestamp"`
	Download  int64     `json:"download"` // bytes
	Upload    int64     `json:"upload"`   // bytes
	Speed     int       `json:"speed"`    // Mbps
	Sessions  int       `json:"sessions"`
}

// SessionStats 会话统计.
type SessionStats struct {
	SessionID       string `json:"session_id"`
	ClientIP        string `json:"client_ip"`
	TotalBytes      int64  `json:"total_bytes"`
	ChannelCount    int    `json:"channel_count"`
	AvgChannelSpeed int    `json:"avg_channel_speed"` // Mbps
	Duration        int64  `json:"duration"`          // seconds
}

// UpdateConfigRequest 更新配置请求.
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

// ChannelStats 通道统计信息.
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
}

// ChannelHealth 通道健康状态.
type ChannelHealth struct {
	ChannelID  string    `json:"channel_id"`
	Status     string    `json:"status"`      // up, down, degraded
	Latency    int64     `json:"latency"`     // ms
	PacketLoss float64   `json:"packet_loss"` // 0.0-100.0
	LastCheck  time.Time `json:"last_check"`
}

// AuditEntry 审计日志条目.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	User      string    `json:"user"`
	ClientIP  string    `json:"client_ip"`
	Details   string    `json:"details"`
}

// ChannelStatsResponse 通道统计响应.
type ChannelStatsResponse struct {
	Stats *ChannelStats `json:"stats"`
}

// ChannelHealthResponse 通道健康响应.
type ChannelHealthResponse struct {
	Health []ChannelHealth `json:"health"`
}

// AuditLogResponse 审计日志响应.
type AuditLogResponse struct {
	Total   int          `json:"total"`
	Entries []AuditEntry `json:"entries"`
}

// EnableDisableResponse 启用/禁用响应.
type EnableDisableResponse struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
}

// SetLoadBalanceModeRequest 设置负载均衡模式请求.
type SetLoadBalanceModeRequest struct {
	Mode string `json:"mode"` // round-robin, weighted, hash
}
