// Package smbmultichannel 提供 SMB Multichannel 支持，对标 TrueNAS 的多通道功能
package smbmultichannel

import (
	"time"
)

// ChannelConfig SMB Multichannel 配置.
type ChannelConfig struct {
	Enabled        bool     `json:"enabled"`
	MaxChannels    int      `json:"max_channels"`
	InterfaceNames []string `json:"interface_names"`
	MinSpeed       int      `json:"min_speed"` // Mbps
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
	ID               string           `json:"id"`
	ClientIP         string           `json:"client_ip"`
	ServerIP         string           `json:"server_ip"`
	Channels         []ChannelRef     `json:"channels"`
	TotalSpeed       int              `json:"total_speed"` // Mbps
	StartTime        time.Time        `json:"start_time"`
	BytesTransferred int64            `json:"bytes_transferred"`
	Protocol         string           `json:"protocol"`
}

// ChannelRef 会话中的通道引用.
type ChannelRef struct {
	InterfaceName string `json:"interface_name"`
	Speed         int    `json:"speed"` // Mbps
	Active        bool   `json:"active"`
}

// ThroughputStats 吞吐量统计.
type ThroughputStats struct {
	TotalDownload   int64      `json:"total_download"` // bytes
	TotalUpload     int64      `json:"total_upload"`   // bytes
	AvgSpeed        int        `json:"avg_speed"`      // Mbps
	PeakSpeed       int        `json:"peak_speed"`     // Mbps
	ActiveSessions  int        `json:"active_sessions"`
	ActiveChannels  int        `json:"active_channels"`
	LastUpdated     time.Time  `json:"last_updated"`
}

// BandwidthHistoryItem 带宽历史记录.
type BandwidthHistoryItem struct {
	Timestamp   time.Time `json:"timestamp"`
	Download    int64     `json:"download"` // bytes
	Upload      int64     `json:"upload"`   // bytes
	Speed       int       `json:"speed"`    // Mbps
	Sessions    int       `json:"sessions"`
}

// SessionStats 会话统计.
type SessionStats struct {
	SessionID       string    `json:"session_id"`
	ClientIP        string    `json:"client_ip"`
	TotalBytes      int64     `json:"total_bytes"`
	ChannelCount    int       `json:"channel_count"`
	AvgChannelSpeed int       `json:"avg_channel_speed"` // Mbps
	Duration        int64     `json:"duration"`          // seconds
}

// UpdateConfigRequest 更新配置请求.
type UpdateConfigRequest struct {
	Enabled        *bool    `json:"enabled,omitempty"`
	MaxChannels    *int     `json:"max_channels,omitempty"`
	InterfaceNames []string `json:"interface_names,omitempty"`
	MinSpeed       *int     `json:"min_speed,omitempty"`
}
