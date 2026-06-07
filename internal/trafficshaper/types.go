// Package trafficshaper 提供网络流量整形功能，支持带宽管理、QoS 策略、流量控制等。
// 提供流量规则管理、流量类别管理、带宽分配、流量监控与事件记录等功能。
package trafficshaper

import "time"

// TrafficDirection 流量方向
type TrafficDirection string

const (
	DirectionInbound  TrafficDirection = "inbound"
	DirectionOutbound TrafficDirection = "outbound"
	DirectionBoth     TrafficDirection = "both"
)

// Protocol 网络协议
type Protocol string

const (
	ProtocolTCP Protocol = "tcp"
	ProtocolUDP Protocol = "udp"
	ProtocolAny Protocol = "any"
)

// TrafficAction 流量动作
type TrafficAction string

const (
	ActionShape TrafficAction = "shape"
	ActionBlock TrafficAction = "block"
	ActionAllow TrafficAction = "allow"
)

// EventType 事件类型
type EventType string

const (
	EventThrottle EventType = "throttle"
	EventBlock    EventType = "block"
	EventAllow    EventType = "allow"
	EventOverflow EventType = "overflow"
)

// TrafficRule 流量规则
type TrafficRule struct {
	ID                  string           `json:"id"`
	Name                string           `json:"name" binding:"required"`
	Direction           TrafficDirection `json:"direction" binding:"required"`
	Priority            int              `json:"priority" binding:"required,min=1,max=10"`
	Protocol            Protocol         `json:"protocol"`
	SourceIP            string           `json:"source_ip,omitempty"`
	DestIP              string           `json:"dest_ip,omitempty"`
	PortRange           string           `json:"port_range,omitempty"`
	MaxBandwidth        int64            `json:"max_bandwidth"`
	GuaranteedBandwidth int64            `json:"guaranteed_bandwidth"`
	BurstSize           int64            `json:"burst_size"`
	Action              TrafficAction    `json:"action" binding:"required"`
	Enabled             bool             `json:"enabled"`
	CreatedAt           time.Time        `json:"created_at"`
}

// TrafficClass 流量类别
type TrafficClass struct {
	ID                  string `json:"id"`
	Name                string `json:"name" binding:"required"`
	Priority            int    `json:"priority"`
	MaxBandwidth        int64  `json:"max_bandwidth"`
	GuaranteedBandwidth int64  `json:"guaranteed_bandwidth"`
	CurrentUsage        int64  `json:"current_usage"`
	RuleCount           int    `json:"rule_count"`
	Description         string `json:"description,omitempty"`
}

// TrafficStats 流量统计
type TrafficStats struct {
	RuleID        string    `json:"rule_id"`
	BytesIn       int64     `json:"bytes_in"`
	BytesOut      int64     `json:"bytes_out"`
	PacketsIn     int64     `json:"packets_in"`
	PacketsOut    int64     `json:"packets_out"`
	DropsIn       int64     `json:"drops_in"`
	DropsOut      int64     `json:"drops_out"`
	CurrentBpsIn  int64     `json:"current_bps_in"`
	CurrentBpsOut int64     `json:"current_bps_out"`
	PeakBps       int64     `json:"peak_bps"`
	LastReset     time.Time `json:"last_reset"`
}

// BandwidthAllocation 带宽分配
type BandwidthAllocation struct {
	TotalBandwidth     int64             `json:"total_bandwidth"`
	AllocatedBandwidth int64             `json:"allocated_bandwidth"`
	FreeBandwidth      int64             `json:"free_bandwidth"`
	Classes            []ClassAllocation `json:"classes"`
}

// ClassAllocation 类别带宽分配
type ClassAllocation struct {
	ClassID    string  `json:"class_id"`
	ClassName  string  `json:"class_name"`
	Allocated  int64   `json:"allocated"`
	Used       int64   `json:"used"`
	Percentage float64 `json:"percentage"`
}

// TrafficEvent 流量事件
type TrafficEvent struct {
	ID            string    `json:"id"`
	RuleID        string    `json:"rule_id"`
	EventType     EventType `json:"event_type"`
	BytesAffected int64     `json:"bytes_affected"`
	Timestamp     time.Time `json:"timestamp"`
	Details       string    `json:"details,omitempty"`
}

// TrafficShaperConfig 流量整形配置
type TrafficShaperConfig struct {
	Enabled        bool  `json:"enabled"`
	TotalBandwidth int64 `json:"total_bandwidth"`
	DefaultMaxBps  int64 `json:"default_max_bps"`
	StatsInterval  int   `json:"stats_interval"`
	MaxEvents      int   `json:"max_events"`
}

// DefaultTrafficShaperConfig 默认配置
func DefaultTrafficShaperConfig() *TrafficShaperConfig {
	return &TrafficShaperConfig{
		Enabled:        true,
		TotalBandwidth: 1000000000, // 1 Gbps
		DefaultMaxBps:  100000000,  // 100 Mbps
		StatsInterval:  60,
		MaxEvents:      10000,
	}
}

// IsValidDirection 检查流量方向是否有效
func IsValidDirection(d TrafficDirection) bool {
	return d == DirectionInbound || d == DirectionOutbound || d == DirectionBoth
}

// IsValidProtocol 检查协议是否有效
func IsValidProtocol(p Protocol) bool {
	return p == ProtocolTCP || p == ProtocolUDP || p == ProtocolAny
}

// IsValidAction 检查流量动作是否有效
func IsValidAction(a TrafficAction) bool {
	return a == ActionShape || a == ActionBlock || a == ActionAllow
}

// IsValidPriority 检查优先级是否有效
func IsValidPriority(p int) bool {
	return p >= 1 && p <= 10
}
