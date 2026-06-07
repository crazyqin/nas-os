// Package trafficclassifier 提供 AI 驱动的网络流量分类功能，支持流量特征提取、
// 实时分析、异常检测和 QoS 策略联动。
package trafficclassifier

import "time"

// TrafficType 流量类型
type TrafficType string

const (
	TrafficTypeVideo    TrafficType = "video"    // 视频流量
	TrafficTypeAudio    TrafficType = "audio"    // 音频流量
	TrafficTypeGame     TrafficType = "game"     // 游戏流量
	TrafficTypeDownload TrafficType = "download" // 下载流量
	TrafficTypeOffice   TrafficType = "office"   // 办公流量
	TrafficTypeIoT      TrafficType = "iot"      // IoT 流量
	TrafficTypeUnknown  TrafficType = "unknown"  // 未知流量
)

// AnomalyType 异常类型
type AnomalyType string

const (
	AnomalyTypeDDoS       AnomalyType = "ddos"       // DDoS 攻击
	AnomalyTypeMining     AnomalyType = "mining"     // 挖矿行为
	AnomalyTypeDataLeak   AnomalyType = "data_leak"  // 数据泄露
	AnomalyTypeScan       AnomalyType = "scan"       // 端口扫描
	AnomalyTypeFlood      AnomalyType = "flood"      // 流量泛洪
	AnomalyTypeSuspicious AnomalyType = "suspicious" // 可疑行为
)

// AlertSeverity 告警严重程度
type AlertSeverity string

const (
	AlertSeverityLow      AlertSeverity = "low"
	AlertSeverityMedium   AlertSeverity = "medium"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AnalysisStatus 分析状态
type AnalysisStatus string

const (
	AnalysisStatusPending   AnalysisStatus = "pending"
	AnalysisStatusRunning   AnalysisStatus = "running"
	AnalysisStatusCompleted AnalysisStatus = "completed"
	AnalysisStatusFailed    AnalysisStatus = "failed"
)

// TrafficFlow 流量流记录
type TrafficFlow struct {
	ID          string      `json:"id"`
	SrcIP       string      `json:"src_ip"`
	DstIP       string      `json:"dst_ip"`
	SrcPort     int         `json:"src_port"`
	DstPort     int         `json:"dst_port"`
	Protocol    string      `json:"protocol"`
	BytesIn     int64       `json:"bytes_in"`
	BytesOut    int64       `json:"bytes_out"`
	PacketsIn   int64       `json:"packets_in"`
	PacketsOut  int64       `json:"packets_out"`
	StartTime   time.Time   `json:"start_time"`
	EndTime     time.Time   `json:"end_time"`
	TrafficType TrafficType `json:"traffic_type"`
	Confidence  float64     `json:"confidence"`
	IsAnomaly   bool        `json:"is_anomaly"`
	AnomalyType AnomalyType `json:"anomaly_type,omitempty"`
	Application string      `json:"application,omitempty"`
	Labels      []string    `json:"labels,omitempty"`
}

// TrafficStats 流量统计
type TrafficStats struct {
	TotalBytes        int64                 `json:"total_bytes"`
	TotalPackets      int64                 `json:"total_packets"`
	ActiveFlows       int                   `json:"active_flows"`
	FlowsByType       map[TrafficType]int64 `json:"flows_by_type"`
	BytesByType       map[TrafficType]int64 `json:"bytes_by_type"`
	TopTalkers        []EndpointStats       `json:"top_talkers"`
	ProtocolBreakdown map[string]int64      `json:"protocol_breakdown"`
	AnomalyCount      int                   `json:"anomaly_count"`
	Timestamp         time.Time             `json:"timestamp"`
}

// EndpointStats 端点统计
type EndpointStats struct {
	IP        string `json:"ip"`
	BytesIn   int64  `json:"bytes_in"`
	BytesOut  int64  `json:"bytes_out"`
	FlowCount int    `json:"flow_count"`
}

// TrafficFeature 流量特征
type TrafficFeature struct {
	AvgPacketSize   float64 `json:"avg_packet_size"`
	StdPacketSize   float64 `json:"std_packet_size"`
	PacketRate      float64 `json:"packet_rate"` // 包/秒
	ByteRate        float64 `json:"byte_rate"`   // 字节/秒
	Protocol        string  `json:"protocol"`
	DstPort         int     `json:"dst_port"`
	FlowDuration    float64 `json:"flow_duration"` // 秒
	PushFlagRatio   float64 `json:"push_flag_ratio"`
	SynFlagRatio    float64 `json:"syn_flag_ratio"`
	BurstCount      int     `json:"burst_count"`
	InterArrivalAvg float64 `json:"inter_arrival_avg"` // 包间隔均值(ms)
	InterArrivalStd float64 `json:"inter_arrival_std"` // 包间隔标准差(ms)
}

// ClassificationResult 分类结果
type ClassificationResult struct {
	FlowID      string          `json:"flow_id"`
	TrafficType TrafficType     `json:"traffic_type"`
	Confidence  float64         `json:"confidence"`
	Features    *TrafficFeature `json:"features"`
	RuleName    string          `json:"rule_name,omitempty"`  // 命中的规则名
	ModelName   string          `json:"model_name,omitempty"` // 使用的模型名
	Timestamp   time.Time       `json:"timestamp"`
}

// AnomalyAlert 异常告警
type AnomalyAlert struct {
	ID          string        `json:"id"`
	AnomalyType AnomalyType   `json:"anomaly_type"`
	Severity    AlertSeverity `json:"severity"`
	SourceIP    string        `json:"source_ip"`
	DestIP      string        `json:"dest_ip,omitempty"`
	Description string        `json:"description"`
	FlowIDs     []string      `json:"flow_ids"`
	BytesTotal  int64         `json:"bytes_total"`
	FirstSeen   time.Time     `json:"first_seen"`
	LastSeen    time.Time     `json:"last_seen"`
	IsResolved  bool          `json:"is_resolved"`
	ResolvedAt  *time.Time    `json:"resolved_at,omitempty"`
}

// BandwidthPolicy 带宽分配策略
type BandwidthPolicy struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	TrafficType TrafficType `json:"traffic_type"`
	MinMbps     float64     `json:"min_mbps"` // 最小带宽(Mbps)
	MaxMbps     float64     `json:"max_mbps"` // 最大带宽(Mbps)
	Priority    int         `json:"priority"` // 优先级(1-10, 1最高)
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// ClassificationRule 自定义分类规则
type ClassificationRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	TrafficType TrafficType `json:"traffic_type"`
	// 规则条件
	SrcIPPattern  string `json:"src_ip_pattern,omitempty"`
	DstIPPattern  string `json:"dst_ip_pattern,omitempty"`
	Ports         []int  `json:"ports,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	PayloadRegex  string `json:"payload_regex,omitempty"`
	MinPacketSize int    `json:"min_packet_size,omitempty"`
	MaxPacketSize int    `json:"max_packet_size,omitempty"`
	// 元数据
	Priority  int       `json:"priority"` // 优先级，越高越先匹配
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MirrorConfig 流量镜像配置
type MirrorConfig struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	SourceIface string    `json:"source_iface"`
	TargetIface string    `json:"target_iface"`
	Filter      string    `json:"filter,omitempty"` // BPF 过滤表达式
	SampleRate  int       `json:"sample_rate"`      // 采样率(1/N)
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// QoSRule QoS 规则
type QoSRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	TrafficType TrafficType `json:"traffic_type"`
	DSCP        int         `json:"dscp"`      // DSCP 标记
	QueueID     int         `json:"queue_id"`  // 队列 ID
	RateMbps    float64     `json:"rate_mbps"` // 限速(Mbps)
	CeilMbps    float64     `json:"ceil_mbps"` // 突发上限(Mbps)
	Priority    int         `json:"priority"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// TrafficReport 流量报告
type TrafficReport struct {
	ID             string                  `json:"id"`
	Title          string                  `json:"title"`
	StartTime      time.Time               `json:"start_time"`
	EndTime        time.Time               `json:"end_time"`
	Stats          *TrafficStats           `json:"stats"`
	TopAnomalies   []AnomalyAlert          `json:"top_anomalies,omitempty"`
	BandwidthUsage map[TrafficType]float64 `json:"bandwidth_usage"` // Mbps by type
	Summary        string                  `json:"summary"`
	GeneratedAt    time.Time               `json:"generated_at"`
}

// AnalyzeRequest 分析请求
type AnalyzeRequest struct {
	Flows    []TrafficFlow `json:"flows" binding:"required,dive"`
	RealTime bool          `json:"real_time"`
	WithDPI  bool          `json:"with_dpi"` // 是否深度包检测
}

// AnalyzeResponse 分析响应
type AnalyzeResponse struct {
	Results   []ClassificationResult `json:"results"`
	Stats     *TrafficStats          `json:"stats"`
	Alerts    []AnomalyAlert         `json:"alerts,omitempty"`
	Status    AnalysisStatus         `json:"status"`
	ProcessMs int64                  `json:"process_ms"`
}

// ReportRequest 报告请求
type ReportRequest struct {
	StartTime time.Time `json:"start_time" binding:"required"`
	EndTime   time.Time `json:"end_time" binding:"required"`
	Title     string    `json:"title,omitempty"`
}

// ClassifierConfig 分类器配置
type ClassifierConfig struct {
	Enabled          bool    `json:"enabled"`
	MaxFlows         int     `json:"max_flows"`          // 最大追踪流数
	FlowTimeoutSec   int     `json:"flow_timeout_sec"`   // 流超时(秒)
	AnomalyThreshold float64 `json:"anomaly_threshold"`  // 异常检测阈值
	DPIDepth         int     `json:"dpi_depth"`          // DPI 检测深度(字节)
	SampleRate       int     `json:"sample_rate"`        // 采样率
	StatsIntervalSec int     `json:"stats_interval_sec"` // 统计间隔(秒)
	EnableMirror     bool    `json:"enable_mirror"`
	EnableQoS        bool    `json:"enable_qos"`
	ReportRetention  int     `json:"report_retention"` // 报告保留天数
}

// DefaultClassifierConfig 默认配置
func DefaultClassifierConfig() *ClassifierConfig {
	return &ClassifierConfig{
		Enabled:          true,
		MaxFlows:         100000,
		FlowTimeoutSec:   300,
		AnomalyThreshold: 0.8,
		DPIDepth:         128,
		SampleRate:       1,
		StatsIntervalSec: 60,
		EnableMirror:     false,
		EnableQoS:        true,
		ReportRetention:  30,
	}
}

// DPISignature DPI 签名
type DPISignature struct {
	Name        string      `json:"name"`
	Pattern     string      `json:"pattern"`
	TrafficType TrafficType `json:"traffic_type"`
	Port        int         `json:"port,omitempty"`
	Protocol    string      `json:"protocol,omitempty"`
}
