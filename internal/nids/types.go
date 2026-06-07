package nids

import (
	"net"
	"sync"
	"time"
)

// Severity 告警严重级别.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// DetectionType 检测类型.
type DetectionType string

const (
	DetectionSignature DetectionType = "signature"
	DetectionAnomaly   DetectionType = "anomaly"
	DetectionBoth      DetectionType = "both"
)

// RuleAction 规则动作.
type RuleAction string

const (
	ActionAlert  RuleAction = "alert"
	ActionBlock  RuleAction = "block"
	ActionDrop   RuleAction = "drop"
	ActionLog    RuleAction = "log"
	ActionIgnore RuleAction = "ignore"
)

// Protocol 网络协议.
type Protocol string

const (
	ProtoTCP  Protocol = "tcp"
	ProtoUDP  Protocol = "udp"
	ProtoICMP Protocol = "icmp"
	ProtoAny  Protocol = "any"
)

// AlertStatus 告警状态.
type AlertStatus string

const (
	AlertOpen     AlertStatus = "open"
	AlertAcked    AlertStatus = "acknowledged"
	AlertResolved AlertStatus = "resolved"
	AlertFalsePos AlertStatus = "false_positive"
)

// Rule 检测规则.
type Rule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Enabled     bool          `json:"enabled"`
	Priority    int           `json:"priority"`
	Severity    Severity      `json:"severity"`
	Action      RuleAction    `json:"action"`
	Type        DetectionType `json:"type"`
	Protocol    Protocol      `json:"protocol"`
	// SrcIP 源地址（CIDR 或 IP）.
	SrcIP string `json:"src_ip,omitempty"`
	// DstIP 目标地址.
	DstIP string `json:"dst_ip,omitempty"`
	// SrcPort 源端口 "80" 或 "8000-9000".
	SrcPort string `json:"src_port,omitempty"`
	// DstPort 目标端口.
	DstPort string `json:"dst_port,omitempty"`
	// Pattern 签名匹配模式（正则或关键字）.
	Pattern string `json:"pattern,omitempty"`
	// Content 包内容匹配.
	Content string `json:"content,omitempty"`
	// Threshold 阈值检测：单位时间内触发次数.
	Threshold *ThresholdConfig `json:"threshold,omitempty"`
	// Tags 规则标签.
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	HitCount  int64     `json:"hit_count"`
}

// ThresholdConfig 阈值配置.
type ThresholdConfig struct {
	// Count 触发次数.
	Count int `json:"count"`
	// Seconds 时间窗口（秒）.
	Seconds int `json:"seconds"`
	// TrackBy 跟踪方式: "src" / "dst" / "both".
	TrackBy string `json:"track_by"`
}

// PacketInfo 解析后的数据包信息.
type PacketInfo struct {
	Timestamp time.Time         `json:"timestamp"`
	SrcIP     net.IP            `json:"src_ip"`
	DstIP     net.IP            `json:"dst_ip"`
	SrcPort   int               `json:"src_port"`
	DstPort   int               `json:"dst_port"`
	Protocol  Protocol          `json:"protocol"`
	Size      int               `json:"size"`
	Payload   []byte            `json:"payload,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// Alert 告警记录.
type Alert struct {
	ID          string      `json:"id"`
	RuleID      string      `json:"rule_id"`
	RuleName    string      `json:"rule_name"`
	Severity    Severity    `json:"severity"`
	Status      AlertStatus `json:"status"`
	Action      RuleAction  `json:"action"`
	SrcIP       net.IP      `json:"src_ip"`
	DstIP       net.IP      `json:"dst_ip"`
	SrcPort     int         `json:"src_port"`
	DstPort     int         `json:"dst_port"`
	Protocol    Protocol    `json:"protocol"`
	Description string      `json:"description"`
	PacketInfo  *PacketInfo `json:"packet_info,omitempty"`
	Count       int         `json:"count"`
	FirstSeen   time.Time   `json:"first_seen"`
	LastSeen    time.Time   `json:"last_seen"`
	AckedAt     *time.Time  `json:"acked_at,omitempty"`
	ResolvedAt  *time.Time  `json:"resolved_at,omitempty"`
}

// TrafficBaseline 流量基线.
type TrafficBaseline struct {
	Protocol    Protocol  `json:"protocol"`
	AvgPPS      float64   `json:"avg_pps"` // 平均包/秒
	AvgBPS      float64   `json:"avg_bps"` // 平均字节/秒
	MaxPPS      float64   `json:"max_pps"`
	MaxBPS      float64   `json:"max_bps"`
	SampleCount int       `json:"sample_count"`
	LastUpdate  time.Time `json:"last_update"`
}

// TrafficStats 流量统计.
type TrafficStats struct {
	TotalPackets    int64                         `json:"total_packets"`
	TotalBytes      int64                         `json:"total_bytes"`
	TotalRules      int64                         `json:"total_rules"`
	PPS             float64                       `json:"pps"` // 当前包/秒
	BPS             float64                       `json:"bps"` // 当前字节/秒
	ProtocolStats   map[Protocol]int64            `json:"protocol_stats"`
	TopSources      []IPCount                     `json:"top_sources"`
	TopDestinations []IPCount                     `json:"top_destinations"`
	Baselines       map[Protocol]*TrafficBaseline `json:"baselines"`
	LastUpdate      time.Time                     `json:"last_update"`
}

// IPCount IP 计数.
type IPCount struct {
	IP    string `json:"ip"`
	Count int64  `json:"count"`
}

// NIDSConfig 全局配置.
type NIDSConfig struct {
	Enabled         bool    `json:"enabled"`
	Mode            string  `json:"mode"` // "inline" / "passive"
	AlertThreshold  int     `json:"alert_threshold"`
	BlockThreshold  int     `json:"block_threshold"`
	BaselineWindowS int     `json:"baseline_window_s"`
	AnomalyFactor   float64 `json:"anomaly_factor"` // 偏离倍数
	MaxAlerts       int     `json:"max_alerts"`
	MaxRules        int     `json:"max_rules"`
	FirewallSync    bool    `json:"firewall_sync"` // 与防火墙联动
}

// NIDSStats NIDS 总体统计.
type NIDSStats struct {
	TotalRules      int       `json:"total_rules"`
	EnabledRules    int       `json:"enabled_rules"`
	TotalAlerts     int       `json:"total_alerts"`
	OpenAlerts      int       `json:"open_alerts"`
	BlockedIPs      int       `json:"blocked_ips"`
	WhitelistedIPs  int       `json:"whitelisted_ips"`
	BlacklistedIPs  int       `json:"blacklisted_ips"`
	PacketsAnalyzed int64     `json:"packets_analyzed"`
	AttacksDetected int64     `json:"attacks_detected"`
	LastUpdate      time.Time `json:"last_update"`
}

// IPEntry IP 黑白名单条目.
type IPEntry struct {
	IP        string     `json:"ip"`
	Reason    string     `json:"reason,omitempty"`
	AddedAt   time.Time  `json:"added_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// ForensicRecord 取证记录.
type ForensicRecord struct {
	ID        string       `json:"id"`
	AlertID   string       `json:"alert_id"`
	SrcIP     net.IP       `json:"src_ip"`
	DstIP     net.IP       `json:"dst_ip"`
	Protocol  Protocol     `json:"protocol"`
	Packets   []PacketInfo `json:"packets"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time"`
	Notes     string       `json:"notes,omitempty"`
}

// Manager NIDS 管理器.
type Manager struct {
	mu           sync.RWMutex
	config       *NIDSConfig
	detector     *Detector
	rules        map[string]*Rule
	alerts       map[string]*Alert
	alertLog     []*Alert
	blacklist    map[string]*IPEntry
	whitelist    map[string]*IPEntry
	forensics    map[string]*ForensicRecord
	trafficStats *TrafficStats
	baselines    map[Protocol]*TrafficBaseline
	alertCounter int64
	forensicID   int64
	maxAlertLog  int
}

// NewManager 创建 NIDS 管理器.
func NewManager() *Manager {
	m := &Manager{
		config: &NIDSConfig{
			Enabled:         true,
			Mode:            "passive",
			AlertThreshold:  3,
			BlockThreshold:  5,
			BaselineWindowS: 3600,
			AnomalyFactor:   3.0,
			MaxAlerts:       10000,
			MaxRules:        500,
			FirewallSync:    false,
		},
		rules:     make(map[string]*Rule),
		alerts:    make(map[string]*Alert),
		alertLog:  make([]*Alert, 0, 1000),
		blacklist: make(map[string]*IPEntry),
		whitelist: make(map[string]*IPEntry),
		forensics: make(map[string]*ForensicRecord),
		trafficStats: &TrafficStats{
			ProtocolStats: make(map[Protocol]int64),
			Baselines:     make(map[Protocol]*TrafficBaseline),
		},
		baselines:   make(map[Protocol]*TrafficBaseline),
		maxAlertLog: 10000,
	}
	m.detector = NewDetector(m)
	return m
}
