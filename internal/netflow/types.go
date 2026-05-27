// Package netflow - 网络流量分析
// 对标群晖流量控制功能，支持sFlow/NetFlow协议、实时流量统计、协议分析
package netflow

import (
	"time"
)

// ============================================================
// 协议定义
// ============================================================

// Protocol 网络协议类型
type Protocol string

const (
	ProtocolTCP     Protocol = "TCP"
	ProtocolUDP     Protocol = "UDP"
	ProtocolHTTP    Protocol = "HTTP"
	ProtocolHTTPS   Protocol = "HTTPS"
	ProtocolDNS     Protocol = "DNS"
	ProtocolICMP    Protocol = "ICMP"
	ProtocolSSH     Protocol = "SSH"
	ProtocolSMB     Protocol = "SMB"
	ProtocolNFS     Protocol = "NFS"
	ProtocolFTP     Protocol = "FTP"
	ProtocolSMTP    Protocol = "SMTP"
	ProtocolOther   Protocol = "OTHER"
)

// FlowDirection 流量方向
type FlowDirection string

const (
	DirectionInbound  FlowDirection = "inbound"
	DirectionOutbound FlowDirection = "outbound"
)

// ============================================================
// 流量记录类型
// ============================================================

// FlowRecord 单条流量记录
// 对标sFlow/NetFlow v5/v9采样记录
type FlowRecord struct {
	// SrcIP 源IP
	SrcIP string `json:"src_ip"`
	// DstIP 目标IP
	DstIP string `json:"dst_ip"`
	// SrcPort 源端口
	SrcPort uint16 `json:"src_port"`
	// DstPort 目标端口
	DstPort uint16 `json:"dst_port"`
	// Protocol 协议
	Protocol Protocol `json:"protocol"`
	// Bytes 传输字节数
	Bytes int64 `json:"bytes"`
	// Packets 传输包数
	Packets int64 `json:"packets"`
	// Direction 流量方向
	Direction FlowDirection `json:"direction"`
	// Interface 网络接口名
	Interface string `json:"interface"`
	// Timestamp 记录时间
	Timestamp time.Time `json:"timestamp"`
	// Duration 持续时间 (秒)
	Duration float64 `json:"duration"`
}

// ============================================================
// 统计类型
// ============================================================

// TrafficStats 流量统计汇总
// 对标群晖"资源监控"中的网络流量面板
type TrafficStats struct {
	// TotalBytesIn 总入站字节
	TotalBytesIn int64 `json:"total_bytes_in"`
	// TotalBytesOut 总出站字节
	TotalBytesOut int64 `json:"total_bytes_out"`
	// TotalPacketsIn 总入站包数
	TotalPacketsIn int64 `json:"total_packets_in"`
	// TotalPacketsOut 总出站包数
	TotalPacketsOut int64 `json:"total_packets_out"`
	// CurrentBPSIn 当前入站速率 (bytes/s)
	CurrentBPSIn float64 `json:"current_bps_in"`
	// CurrentBPSOut 当前出站速率 (bytes/s)
	CurrentBPSOut float64 `json:"current_bps_out"`
	// PeakBPSIn 峰值入站速率 (bytes/s)
	PeakBPSIn float64 `json:"peak_bps_in"`
	// PeakBPSOut 峰值出站速率 (bytes/s)
	PeakBPSOut float64 `json:"peak_bps_out"`
	// ActiveConnections 当前活跃连接数
	ActiveConnections int `json:"active_connections"`
	// Timestamp 统计时间
	Timestamp time.Time `json:"timestamp"`
}

// ProtocolStats 协议流量统计
type ProtocolStats struct {
	// Protocol 协议名称
	Protocol Protocol `json:"protocol"`
	// Bytes 该协议总字节数
	Bytes int64 `json:"bytes"`
	// Packets 该协议总包数
	Packets int64 `json:"packets"`
	// Percentage 占总流量百分比
	Percentage float64 `json:"percentage"`
	// Connections 该协议连接数
	Connections int `json:"connections"`
}

// HostTraffic 主机流量统计
// 对标群晖"谁在连接"功能
type HostTraffic struct {
	// IP 主机IP
	IP string `json:"ip"`
	// Hostname 主机名（如果已知）
	Hostname string `json:"hostname,omitempty"`
	// BytesIn 入站字节
	BytesIn int64 `json:"bytes_in"`
	// BytesOut 出站字节
	BytesOut int64 `json:"bytes_out"`
	// TotalBytes 总字节
	TotalBytes int64 `json:"total_bytes"`
	// Connections 连接数
	Connections int `json:"connections"`
	// LastSeen 最后活跃时间
	LastSeen time.Time `json:"last_seen"`
}

// BandwidthUsage 带宽使用情况
// 按时间窗口统计的带宽使用
type BandwidthUsage struct {
	// Timestamp 时间点
	Timestamp time.Time `json:"timestamp"`
	// InterfaceName 网络接口名
	InterfaceName string `json:"interface_name"`
	// BytesIn 入站字节
	BytesIn int64 `json:"bytes_in"`
	// BytesOut 出站字节
	BytesOut int64 `json:"bytes_out"`
	// Utilization 带宽利用率 (0-100%)
	Utilization float64 `json:"utilization"`
}

// ============================================================
// 异常检测类型
// ============================================================

// AnomalyType 异常类型
type AnomalyType string

const (
	// AnomalyTrafficSpike 流量突增
	AnomalyTrafficSpike AnomalyType = "traffic_spike"
	// AnomalyPortScan 端口扫描
	AnomalyPortScan AnomalyType = "port_scan"
	// AnomalyDNSFlood DNS洪泛
	AnomalyDNSFlood AnomalyType = "dns_flood"
	// AnomalyUnusualProtocol 异常协议
	AnomalyUnusualProtocol AnomalyType = "unusual_protocol"
	// AnomalyHighConnectionRate 高连接速率
	AnomalyHighConnectionRate AnomalyType = "high_connection_rate"
)

// AnomalyAlert 流量异常告警
type AnomalyAlert struct {
	// ID 告警ID
	ID string `json:"id"`
	// Type 异常类型
	Type AnomalyType `json:"type"`
	// Severity 严重程度: "info", "warning", "critical"
	Severity string `json:"severity"`
	// SourceIP 相关源IP
	SourceIP string `json:"source_ip"`
	// TargetIP 相关目标IP（如果有）
	TargetIP string `json:"target_ip,omitempty"`
	// Description 描述
	Description string `json:"description"`
	// DetectedAt 检测时间
	DetectedAt time.Time `json:"detected_at"`
	// Resolved 是否已解决
	Resolved bool `json:"resolved"`
}

// ============================================================
// TopN分析
// ============================================================

// TopNEntry TopN条目
type TopNEntry struct {
	// Key 排名键（IP、端口、协议等）
	Key string `json:"key"`
	// Value 排名值（字节数、连接数等）
	Value int64 `json:"value"`
	// Label 显示标签
	Label string `json:"label,omitempty"`
}

// TopNResult TopN分析结果
type TopNResult struct {
	// Category 分类: "hosts", "ports", "protocols", "conversations"
	Category string `json:"category"`
	// Metric 排名指标: "bytes", "packets", "connections"
	Metric string `json:"metric"`
	// Entries 排名条目（按value降序）
	Entries []TopNEntry `json:"entries"`
	// Timestamp 分析时间
	Timestamp time.Time `json:"timestamp"`
}

// ============================================================
// 采集器配置
// ============================================================

// CollectorConfig 流量采集器配置
type CollectorConfig struct {
	// ListenAddress 监听地址
	ListenAddress string `json:"listen_address"`
	// SFlowPort sFlow接收端口
	SFlowPort int `json:"sflow_port"`
	// NetFlowPort NetFlow接收端口
	NetFlowPort int `json:"netflow_port"`
	// SampleRate 采样率 (1/N)
	SampleRate int `json:"sample_rate"`
	// BufferSize 缓冲区大小（最大记录数）
	BufferSize int `json:"buffer_size"`
	// FlushIntervalSec 刷新间隔 (秒)
	FlushIntervalSec int `json:"flush_interval_sec"`
}

// DefaultCollectorConfig 默认采集器配置
func DefaultCollectorConfig() CollectorConfig {
	return CollectorConfig{
		ListenAddress:  "0.0.0.0",
		SFlowPort:      6343,
		NetFlowPort:    2055,
		SampleRate:     100,
		BufferSize:     100000,
		FlushIntervalSec: 60,
	}
}

// ============================================================
// 请求/响应类型
// ============================================================

// TrafficStatsResponse 流量统计响应
type TrafficStatsResponse struct {
	Stats      TrafficStats      `json:"stats"`
	Protocols  []ProtocolStats   `json:"protocols"`
	TopHosts   []HostTraffic     `json:"top_hosts"`
}

// BandwidthHistoryRequest 带宽历史查询请求
type BandwidthHistoryRequest struct {
	Interface string `form:"interface"`
	From      string `form:"from"`
	To        string `form:"to"`
	Interval  string `form:"interval"` // "1m", "5m", "1h", "1d"
}

// AlertListResponse 告警列表响应
type AlertListResponse struct {
	Alerts []AnomalyAlert `json:"alerts"`
	Total  int            `json:"total"`
}
