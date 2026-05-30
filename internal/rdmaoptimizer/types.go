// Package rdmaoptimizer 提供 RDMA 网络传输优化
// 支持自动路径选择和拥塞控制
package rdmaoptimizer

import (
	"time"
)

// LinkState 链路状态
type LinkState string

const (
	LinkStateActive    LinkState = "active"
	LinkStateDown      LinkState = "down"
	LinkStateDegraded  LinkState = "degraded"
	LinkStateError     LinkState = "error"
)

// PathState 路径状态
type PathState string

const (
	PathStateAvailable PathState = "available"
	PathStateCongested PathState = "congested"
	PathStateDown      PathState = "down"
)

// CongestionAlgorithm 拥塞控制算法
type CongestionAlgorithm string

const (
	CongestionECN  CongestionAlgorithm = "ecn"  // Explicit Congestion Notification
	CongestionDCTCP CongestionAlgorithm = "dctcp" // Data Center TCP
	CongestionDCQCN CongestionAlgorithm = "dcqcn" // Data Center QCN
	CongestionTIMELY CongestionAlgorithm = "timely"
)

// RDMALink RDMA 链路
type RDMALink struct {
	ID            string            `json:"id"`
	LocalDevice   string            `json:"local_device"`
	LocalPort     int               `json:"local_port"`
	RemoteDevice  string            `json:"remote_device"`
	RemotePort    int               `json:"remote_port"`
	RemoteGID     string            `json:"remote_gid"`
	LinkSpeed     string            `json:"link_speed"` // 100Gb, 200Gb, 400Gb
	State         LinkState         `json:"state"`
	BandwidthGbps float64           `json:"bandwidth_gbps"`
	LatencyNs     int64             `json:"latency_ns"`
	MTU           int               `json:"mtu"`
	ErrorCount    int64             `json:"error_count"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	LastSeen      time.Time         `json:"last_seen"`
	CreatedAt     time.Time         `json:"created_at"`
}

// RDMAPath RDMA 路径
type RDMAPath struct {
	ID            string     `json:"id"`
	SourceDevice  string     `json:"source_device"`
	DestDevice    string     `json:"dest_device"`
	Links         []string   `json:"links"`         // Link IDs in path
	HopCount      int        `json:"hop_count"`
	State         PathState  `json:"state"`
	BandwidthGbps float64    `json:"bandwidth_gbps"`
	LatencyNs     int64      `json:"latency_ns"`
	Congestion    float64    `json:"congestion"`     // 0.0 - 1.0
	Weight        float64    `json:"weight"`         // For path selection
	LastMeasured  time.Time  `json:"last_measured"`
	CreatedAt     time.Time  `json:"created_at"`
}

// CongestionConfig 拥塞控制配置
type CongestionConfig struct {
	Algorithm        CongestionAlgorithm `json:"algorithm"`
	Enabled          bool                `json:"enabled"`
	Threshold        float64             `json:"threshold"`         // Congestion threshold 0.0-1.0
	BackoffMs        int                 `json:"backoff_ms"`        // Backoff time in ms
	ProbeIntervalMs  int                 `json:"probe_interval_ms"` // Probe interval
	MaxRetries       int                 `json:"max_retries"`
	ECNEnabled       bool                `json:"ecn_enabled"`
	ECNThreshold     float64             `json:"ecn_threshold"`
	DCQCNAlpha       float64             `json:"dcqcn_alpha"`
	DCQCNBeta        float64             `json:"dcqcn_beta"`
	DCQCNMinRate     float64             `json:"dcqcn_min_rate"` // Gbps
}

// TransferMetrics 传输指标
type TransferMetrics struct {
	LinkID        string    `json:"link_id"`
	PathID        string    `json:"path_id"`
	BytesSent     int64     `json:"bytes_sent"`
	BytesReceived int64     `json:"bytes_received"`
	IOPS          int64     `json:"iops"`
	BandwidthGbps float64   `json:"bandwidth_gbps"`
	AvgLatencyNs  int64     `json:"avg_latency_ns"`
	MaxLatencyNs  int64     `json:"max_latency_ns"`
	PacketLoss     float64   `json:"packet_loss"`    // 0.0-1.0
	Congestion    float64   `json:"congestion"`     // 0.0-1.0
	Timestamp     time.Time `json:"timestamp"`
}

// LinkMetrics 链路指标
type LinkMetrics struct {
	LinkID        string    `json:"link_id"`
	BandwidthGbps float64   `json:"bandwidth_gbps"`
	Utilization   float64   `json:"utilization"`    // 0.0-1.0
	LatencyNs     int64     `json:"latency_ns"`
	ErrorRate     float64   `json:"error_rate"`
	Congestion    float64   `json:"congestion"`
	Timestamp     time.Time `json:"timestamp"`
}

// OptimizerStats 优化器统计
type OptimizerStats struct {
	TotalLinks      int       `json:"total_links"`
	ActiveLinks     int       `json:"active_links"`
	TotalPaths      int       `json:"total_paths"`
	ActivePaths     int       `json:"active_paths"`
	AvgLatencyNs    int64     `json:"avg_latency_ns"`
	AvgBandwidthGbps float64  `json:"avg_bandwidth_gbps"`
	TotalBytes      int64     `json:"total_bytes"`
	OptimizationCount int64   `json:"optimization_count"`
	LastOptimized   time.Time `json:"last_optimized"`
}

// CreateLinkRequest 创建链路请求
type CreateLinkRequest struct {
	LocalDevice   string `json:"local_device" binding:"required"`
	LocalPort     int    `json:"local_port" binding:"required"`
	RemoteDevice  string `json:"remote_device" binding:"required"`
	RemotePort    int    `json:"remote_port" binding:"required"`
	RemoteGID     string `json:"remote_gid"`
	LinkSpeed     string `json:"link_speed"`
	MTU           int    `json:"mtu"`
}

// CreatePathRequest 创建路径请求
type CreatePathRequest struct {
	SourceDevice string   `json:"source_device" binding:"required"`
	DestDevice   string   `json:"dest_device" binding:"required"`
	Links        []string `json:"links" binding:"required"`
}

// OptimizeRequest 优化请求
type OptimizeRequest struct {
	LinkID     string `json:"link_id"`
	PathID     string `json:"path_id"`
	TargetLatencyNs int64 `json:"target_latency_ns"`
	TargetBandwidthGbps float64 `json:"target_bandwidth_gbps"`
}
