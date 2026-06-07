// Package wanoptimize 提供 WAN 传输加速与优化
package wanoptimize

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrTunnelNotFound 隧道不存在.
	ErrTunnelNotFound = errors.New("隧道不存在")
	// ErrTunnelAlreadyExists 隧道已存在.
	ErrTunnelAlreadyExists = errors.New("隧道已存在")
	// ErrTunnelActive 隧道处于活跃状态.
	ErrTunnelActive = errors.New("隧道处于活跃状态")
	// ErrPeerNotFound 对端不存在.
	ErrPeerNotFound = errors.New("对端不存在")
)

// ========== 核心类型 ==========

// CompressMode 压缩模式.
type CompressMode string

const (
	// CompressNone 不压缩.
	CompressNone CompressMode = "none"
	// CompressLZ4 LZ4快速压缩.
	CompressLZ4 CompressMode = "lz4"
	// CompressZSTD Zstandard压缩.
	CompressZSTD CompressMode = "zstd"
	// CompressAdaptive 自适应压缩.
	CompressAdaptive CompressMode = "adaptive"
)

// TunnelStatus 隧道状态.
type TunnelStatus string

const (
	// TunnelStatusActive 活跃.
	TunnelStatusActive TunnelStatus = "active"
	// TunnelStatusInactive 未活跃.
	TunnelStatusInactive TunnelStatus = "inactive"
	// TunnelStatusConnecting 连接中.
	TunnelStatusConnecting TunnelStatus = "connecting"
	// TunnelStatusError 错误.
	TunnelStatusError TunnelStatus = "error"
)

// ========== 数据结构 ==========

// Tunnel WAN加速隧道.
type Tunnel struct {
	ID         string        `json:"id"`          // 隧道ID
	Name       string        `json:"name"`        // 隧道名称
	LocalAddr  string        `json:"local_addr"`  // 本地地址
	RemoteAddr string        `json:"remote_addr"` // 远端地址
	Port       int           `json:"port"`        // 端口
	Compress   CompressMode  `json:"compress"`    // 压缩模式
	Encrypt    bool          `json:"encrypt"`     // 是否加密
	Bandwidth  int64         `json:"bandwidth"`   // 带宽限制(bytes/s)
	Status     TunnelStatus  `json:"status"`      // 状态
	BytesSent  int64         `json:"bytes_sent"`  // 已发送字节
	BytesRecv  int64         `json:"bytes_recv"`  // 已接收字节
	Latency    time.Duration `json:"latency"`     // 延迟
	CreatedAt  time.Time     `json:"created_at"`  // 创建时间
	UpdatedAt  time.Time     `json:"updated_at"`  // 更新时间
}

// TransferStats 传输统计.
type TransferStats struct {
	TunnelID      string    `json:"tunnel_id"`      // 隧道ID
	BytesSent     int64     `json:"bytes_sent"`     // 发送字节
	BytesRecv     int64     `json:"bytes_recv"`     // 接收字节
	CompressRatio float64   `json:"compress_ratio"` // 压缩比
	AvgSpeed      int64     `json:"avg_speed"`      // 平均速度
	PeakSpeed     int64     `json:"peak_speed"`     // 峰值速度
	PacketLoss    float64   `json:"packet_loss"`    // 丢包率
	Timestamp     time.Time `json:"timestamp"`      // 时间戳
}

// WANStats 全局WAN统计.
type WANStats struct {
	TotalTunnels  int64         `json:"total_tunnels"`  // 总隧道数
	ActiveTunnels int64         `json:"active_tunnels"` // 活跃隧道数
	TotalSent     int64         `json:"total_sent"`     // 总发送
	TotalRecv     int64         `json:"total_recv"`     // 总接收
	AvgLatency    time.Duration `json:"avg_latency"`    // 平均延迟
}

// CreateTunnelRequest 创建隧道请求.
type CreateTunnelRequest struct {
	Name       string       `json:"name" binding:"required"`
	LocalAddr  string       `json:"local_addr" binding:"required"`
	RemoteAddr string       `json:"remote_addr" binding:"required"`
	Port       int          `json:"port"`
	Compress   CompressMode `json:"compress"`
	Encrypt    bool         `json:"encrypt"`
	Bandwidth  int64        `json:"bandwidth"`
}
