// Package netmonitor 提供网络监控功能
package netmonitor

import (
	"time"
)

// InterfaceStatus 网络接口状态.
type InterfaceStatus string

const (
	InterfaceStatusUp      InterfaceStatus = "up"
	InterfaceStatusDown    InterfaceStatus = "down"
	InterfaceStatusUnknown InterfaceStatus = "unknown"
)

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// NetworkInterface 网络接口信息.
type NetworkInterface struct {
	Name        string          `json:"name"`
	Status      InterfaceStatus `json:"status"`
	MTU         int             `json:"mtu"`
	MACAddress  string          `json:"mac_address"`
	IPv4Addr    string          `json:"ipv4_addr"`
	IPv6Addr    string          `json:"ipv6_addr"`
	Speed       int             `json:"speed"` // Mbps
	RxBytes     int64           `json:"rx_bytes"`
	TxBytes     int64           `json:"tx_bytes"`
	RxPackets   int64           `json:"rx_packets"`
	TxPackets   int64           `json:"tx_packets"`
	RxErrors    int64           `json:"rx_errors"`
	TxErrors    int64           `json:"tx_errors"`
	RxDropped   int64           `json:"rx_dropped"`
	TxDropped   int64           `json:"tx_dropped"`
	LastUpdated time.Time       `json:"last_updated"`
}

// TrafficStats 流量统计.
type TrafficStats struct {
	Interface    string    `json:"interface"`
	Timestamp    time.Time `json:"timestamp"`
	RxBytesSec   int64     `json:"rx_bytes_sec"`   // 接收速率（字节/秒）
	TxBytesSec   int64     `json:"tx_bytes_sec"`   // 发送速率（字节/秒）
	RxPacketsSec int64     `json:"rx_packets_sec"` // 接收包速率
	TxPacketsSec int64     `json:"tx_packets_sec"` // 发送包速率
	TotalRxBytes int64     `json:"total_rx_bytes"`
	TotalTxBytes int64     `json:"total_tx_bytes"`
}

// ConnectionInfo 连接信息.
type ConnectionInfo struct {
	Protocol      string    `json:"protocol"` // tcp, udp
	LocalAddr     string    `json:"local_addr"`
	LocalPort     int       `json:"local_port"`
	RemoteAddr    string    `json:"remote_addr"`
	RemotePort    int       `json:"remote_port"`
	State         string    `json:"state"` // ESTABLISHED, LISTEN, etc.
	PID           int       `json:"pid"`
	ProcessName   string    `json:"process_name"`
	EstablishedAt time.Time `json:"established_at"`
}

// ConnectionStats 连接统计.
type ConnectionStats struct {
	Timestamp   time.Time         `json:"timestamp"`
	TotalConns  int               `json:"total_conns"`
	TCPConns    int               `json:"tcp_conns"`
	UDPConns    int               `json:"udp_conns"`
	Established int               `json:"established"`
	Listening   int               `json:"listening"`
	TimeWait    int               `json:"time_wait"`
	Connections []*ConnectionInfo `json:"connections,omitempty"`
}

// AlertRule 告警规则.
type AlertRule struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Interface   string     `json:"interface"` // 接口名，空表示所有接口
	Type        string     `json:"type"`      // bandwidth, packet_loss, errors, latency
	Threshold   float64    `json:"threshold"` // 阈值
	Level       AlertLevel `json:"level"`
	Enabled     bool       `json:"enabled"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
}

// AlertEvent 告警事件.
type AlertEvent struct {
	ID         string     `json:"id"`
	RuleID     string     `json:"rule_id"`
	RuleName   string     `json:"rule_name"`
	Interface  string     `json:"interface"`
	Level      AlertLevel `json:"level"`
	Message    string     `json:"message"`
	Value      float64    `json:"value"`
	Threshold  float64    `json:"threshold"`
	Timestamp  time.Time  `json:"timestamp"`
	Resolved   bool       `json:"resolved"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// NetworkNode 网络拓扑节点.
type NetworkNode struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"` // router, switch, nas, client, unknown
	IPAddr     string    `json:"ip_addr"`
	MACAddr    string    `json:"mac_addr"`
	Vendor     string    `json:"vendor"`
	Interfaces []string  `json:"interfaces"`
	IsOnline   bool      `json:"is_online"`
	LastSeen   time.Time `json:"last_seen"`
}

// NetworkLink 网络拓扑连接.
type NetworkLink struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Speed    int    `json:"speed"` // Mbps
	Active   bool   `json:"active"`
}

// NetworkTopology 网络拓扑.
type NetworkTopology struct {
	Nodes      []*NetworkNode `json:"nodes"`
	Links      []*NetworkLink `json:"links"`
	Discovered time.Time      `json:"discovered"`
}

// PortInfo 端口信息.
type PortInfo struct {
	Port        int       `json:"port"`
	Protocol    string    `json:"protocol"`
	State       string    `json:"state"` // open, closed, filtered
	Service     string    `json:"service"`
	Description string    `json:"description"`
	LastChecked time.Time `json:"last_checked"`
}

// PortMonitorConfig 端口监控配置.
type PortMonitorConfig struct {
	Host     string `json:"host"`
	Ports    []int  `json:"ports"`
	Interval int    `json:"interval"` // 检查间隔（秒）
}

// BandwidthTrend 带宽趋势.
type BandwidthTrend struct {
	Interface  string          `json:"interface"`
	StartTime  time.Time       `json:"start_time"`
	EndTime    time.Time       `json:"end_time"`
	Interval   string          `json:"interval"` // 1m, 5m, 1h, 1d
	DataPoints []*TrafficStats `json:"data_points"`
	AvgRx      int64           `json:"avg_rx"`
	AvgTx      int64           `json:"avg_tx"`
	PeakRx     int64           `json:"peak_rx"`
	PeakTx     int64           `json:"peak_tx"`
}
