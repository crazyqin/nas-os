// Package netflowanalyzer 提供网络流量分析功能
// 对标群晖 Traffic Control + pfSense 流量分析
package netflowanalyzer

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== 常量定义 ==========

// 协议类型
const (
	ProtocolTCP   = "TCP"
	ProtocolUDP   = "UDP"
	ProtocolICMP  = "ICMP"
	ProtocolHTTP  = "HTTP"
	ProtocolHTTPS = "HTTPS"
	ProtocolDNS   = "DNS"
	ProtocolSSH   = "SSH"
	ProtocolFTP   = "FTP"
	ProtocolSMTP  = "SMTP"
	ProtocolOther = "OTHER"
)

// 告警级别
const (
	AlertLevelInfo     = "info"
	AlertLevelWarning  = "warning"
	AlertLevelCritical = "critical"
)

// 时间粒度
const (
	GranularityHourly  = "hourly"
	GranularityDaily   = "daily"
	GranularityWeekly  = "weekly"
	GranularityMonthly = "monthly"
)

// 默认配置
const (
	DefaultMaxRecords      = 100000
	DefaultAlertCooldown   = 5 * time.Minute
	DefaultDDoSThreshold   = 1000 // 每秒连接数
	DefaultSurgeMultiplier = 3.0  // 流量突增倍数
	DefaultSampleInterval  = 10 * time.Second
)

// ========== 错误定义 ==========

var (
	ErrInterfaceNotFound  = errors.New("接口不存在")
	ErrPolicyNotFound     = errors.New("策略不存在")
	ErrInvalidIP          = errors.New("无效的IP地址")
	ErrInvalidPort        = errors.New("无效的端口号")
	ErrInvalidGranularity = errors.New("无效的时间粒度")
	ErrAnalyzerStopped    = errors.New("分析器已停止")
	ErrPolicyExists       = errors.New("策略已存在")
	ErrInvalidPolicy      = errors.New("无效的策略配置")
)

// ========== 数据结构 ==========

// FlowRecord 流量记录
type FlowRecord struct {
	ID         string        `json:"id"`
	Timestamp  time.Time     `json:"timestamp"`
	Interface  string        `json:"interface"` // 网络接口名
	SrcIP      string        `json:"src_ip"`
	DstIP      string        `json:"dst_ip"`
	SrcPort    uint16        `json:"src_port"`
	DstPort    uint16        `json:"dst_port"`
	Protocol   string        `json:"protocol"`
	BytesIn    uint64        `json:"bytes_in"`    // 入站字节数
	BytesOut   uint64        `json:"bytes_out"`   // 出站字节数
	PacketsIn  uint64        `json:"packets_in"`  // 入站包数
	PacketsOut uint64        `json:"packets_out"` // 出站包数
	Duration   time.Duration `json:"duration"`    // 连接持续时间
}

// InterfaceStats 接口统计
type InterfaceStats struct {
	Name         string    `json:"name"`
	BytesIn      uint64    `json:"bytes_in"`
	BytesOut     uint64    `json:"bytes_out"`
	PacketsIn    uint64    `json:"packets_in"`
	PacketsOut   uint64    `json:"packets_out"`
	BandwidthIn  float64   `json:"bandwidth_in_bps"`  // 当前入站带宽 (bits/s)
	BandwidthOut float64   `json:"bandwidth_out_bps"` // 当前出站带宽 (bits/s)
	Connections  int64     `json:"connections"`        // 当前连接数
	LastUpdated  time.Time `json:"last_updated"`
}

// ConnectionInfo 连接信息
type ConnectionInfo struct {
	SrcIP     string    `json:"src_ip"`
	DstIP     string    `json:"dst_ip"`
	SrcPort   uint16    `json:"src_port"`
	DstPort   uint16    `json:"dst_port"`
	Protocol  string    `json:"protocol"`
	Interface string    `json:"interface"`
	StartTime time.Time `json:"start_time"`
	BytesIn   uint64    `json:"bytes_in"`
	BytesOut  uint64    `json:"bytes_out"`
}

// ProtocolDistribution 协议分布
type ProtocolDistribution struct {
	Protocol string  `json:"protocol"`
	Bytes    uint64  `json:"bytes"`
	Packets  uint64  `json:"packets"`
	Percent  float64 `json:"percent"`
}

// TrafficSnapshot 实时流量快照
type TrafficSnapshot struct {
	Timestamp         time.Time                `json:"timestamp"`
	Interfaces        map[string]InterfaceStats `json:"interfaces"`
	TotalBandwidthIn  float64                  `json:"total_bandwidth_in_bps"`
	TotalBandwidthOut float64                  `json:"total_bandwidth_out_bps"`
	TotalConnections  int64                    `json:"total_connections"`
	Protocols         []ProtocolDistribution   `json:"protocols"`
}

// TrafficStats 流量统计
type TrafficStats struct {
	StartTime     time.Time    `json:"start_time"`
	EndTime       time.Time    `json:"end_time"`
	Granularity   string       `json:"granularity"`
	TotalBytesIn  uint64       `json:"total_bytes_in"`
	TotalBytesOut uint64       `json:"total_bytes_out"`
	TotalPackets  uint64       `json:"total_packets"`
	AvgBandwidth  float64      `json:"avg_bandwidth_bps"`
	PeakBandwidth float64      `json:"peak_bandwidth_bps"`
	Entries       []StatsEntry `json:"entries"`
}

// StatsEntry 统计条目
type StatsEntry struct {
	Timestamp time.Time `json:"timestamp"`
	BytesIn   uint64    `json:"bytes_in"`
	BytesOut  uint64    `json:"bytes_out"`
	Packets   uint64    `json:"packets"`
	Bandwidth float64   `json:"bandwidth_bps"`
}

// GroupedStats 分组统计
type GroupedStats struct {
	GroupKey string `json:"group_key"`
	BytesIn  uint64 `json:"bytes_in"`
	BytesOut uint64 `json:"bytes_out"`
	Packets  uint64 `json:"packets"`
	Flows    int    `json:"flows"`
}

// TrafficAlert 流量告警
type TrafficAlert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`      // surge / connection_anomaly / ddos
	Level     string    `json:"level"`     // info / warning / critical
	Interface string    `json:"interface"`
	Message   string    `json:"message"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
	Resolved  bool      `json:"resolved"`
}

// BandwidthPolicy 带宽策略
type BandwidthPolicy struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	TargetIP   string    `json:"target_ip"`   // 目标IP（空表示所有）
	TargetPort uint16    `json:"target_port"` // 目标端口（0表示所有）
	Protocol   string    `json:"protocol"`    // 协议（空表示所有）
	MaxInBps   uint64    `json:"max_in_bps"`  // 最大入站带宽 (bits/s)
	MaxOutBps  uint64    `json:"max_out_bps"` // 最大出站带宽 (bits/s)
	Enabled    bool      `json:"enabled"`
	Priority   int       `json:"priority"`    // 优先级（数字越大优先级越高）
	CreatedAt  time.Time `json:"created_at"`
}

// PolicyViolation 策略违规记录
type PolicyViolation struct {
	PolicyID   string    `json:"policy_id"`
	PolicyName string    `json:"policy_name"`
	IP         string    `json:"ip"`
	Port       uint16    `json:"port"`
	ActualBps  uint64    `json:"actual_bps"`
	LimitBps   uint64    `json:"limit_bps"`
	Direction  string    `json:"direction"` // in / out
	Timestamp  time.Time `json:"timestamp"`
}

// TopNEntry Top N 条目
type TopNEntry struct {
	Rank   int    `json:"rank"`
	Key    string `json:"key"`
	Value  uint64 `json:"value"`
	Unit   string `json:"unit"`
	Detail string `json:"detail,omitempty"`
}

// TrafficReport 流量报表
type TrafficReport struct {
	Title        string         `json:"title"`
	GeneratedAt  time.Time      `json:"generated_at"`
	StartTime    time.Time      `json:"start_time"`
	EndTime      time.Time      `json:"end_time"`
	Summary      ReportSummary  `json:"summary"`
	TopTalkers   []TopNEntry    `json:"top_talkers"`
	TopPorts     []TopNEntry    `json:"top_ports"`
	TopProtocols []TopNEntry    `json:"top_protocols"`
	Trends       []TrendEntry   `json:"trends"`
	Alerts       []TrafficAlert `json:"alerts"`
}

// ReportSummary 报表摘要
type ReportSummary struct {
	TotalBytesIn  uint64  `json:"total_bytes_in"`
	TotalBytesOut uint64  `json:"total_bytes_out"`
	TotalPackets  uint64  `json:"total_packets"`
	TotalFlows    int     `json:"total_flows"`
	AvgBandwidth  float64 `json:"avg_bandwidth_bps"`
	PeakBandwidth float64 `json:"peak_bandwidth_bps"`
	UniqueSrcIPs  int     `json:"unique_src_ips"`
	UniqueDstIPs  int     `json:"unique_dst_ips"`
	AlertCount    int     `json:"alert_count"`
}

// TrendEntry 趋势条目
type TrendEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	BytesIn     uint64    `json:"bytes_in"`
	BytesOut    uint64    `json:"bytes_out"`
	Connections int64     `json:"connections"`
	Bandwidth   float64   `json:"bandwidth_bps"`
}

// ExportFormat 导出格式
type ExportFormat string

const (
	ExportJSON ExportFormat = "json"
	ExportCSV  ExportFormat = "csv"
)

// ========== 分析器主结构 ==========

// NetflowAnalyzer 网络流量分析器
type NetflowAnalyzer struct {
	mu sync.RWMutex

	// 配置
	maxRecords      int
	alertCooldown   time.Duration
	ddosThreshold   int
	surgeMultiplier float64
	sampleInterval  time.Duration

	// 数据存储
	records     []FlowRecord
	interfaces  map[string]*InterfaceStats
	connections map[string]*ConnectionInfo

	// 策略和告警
	policies    map[string]*BandwidthPolicy
	alerts      []TrafficAlert
	violations  []PolicyViolation
	alertTimers map[string]time.Time

	// 历史统计
	hourlyStats  map[string][]StatsEntry
	dailyStats   map[string][]StatsEntry
	weeklyStats  map[string][]StatsEntry
	monthlyStats map[string][]StatsEntry

	// 状态
	running   bool
	stopCh    chan struct{}
	startTime time.Time
}

// Option 配置选项
type Option func(*NetflowAnalyzer)

// WithMaxRecords 设置最大记录数
func WithMaxRecords(n int) Option {
	return func(a *NetflowAnalyzer) {
		if n > 0 {
			a.maxRecords = n
		}
	}
}

// WithDDoSThreshold 设置DDoS检测阈值
func WithDDoSThreshold(n int) Option {
	return func(a *NetflowAnalyzer) {
		if n > 0 {
			a.ddosThreshold = n
		}
	}
}

// WithSurgeMultiplier 设置流量突增倍数
func WithSurgeMultiplier(m float64) Option {
	return func(a *NetflowAnalyzer) {
		if m > 1.0 {
			a.surgeMultiplier = m
		}
	}
}

// WithSampleInterval 设置采样间隔
func WithSampleInterval(d time.Duration) Option {
	return func(a *NetflowAnalyzer) {
		if d > 0 {
			a.sampleInterval = d
		}
	}
}

// WithAlertCooldown 设置告警冷却时间
func WithAlertCooldown(d time.Duration) Option {
	return func(a *NetflowAnalyzer) {
		if d > 0 {
			a.alertCooldown = d
		}
	}
}

// ========== 构造函数 ==========

// NewNetflowAnalyzer 创建新的流量分析器
func NewNetflowAnalyzer(opts ...Option) *NetflowAnalyzer {
	a := &NetflowAnalyzer{
		maxRecords:      DefaultMaxRecords,
		alertCooldown:   DefaultAlertCooldown,
		ddosThreshold:   DefaultDDoSThreshold,
		surgeMultiplier: DefaultSurgeMultiplier,
		sampleInterval:  DefaultSampleInterval,
		records:         make([]FlowRecord, 0, 1000),
		interfaces:      make(map[string]*InterfaceStats),
		connections:     make(map[string]*ConnectionInfo),
		policies:        make(map[string]*BandwidthPolicy),
		alerts:          make([]TrafficAlert, 0),
		violations:      make([]PolicyViolation, 0),
		alertTimers:     make(map[string]time.Time),
		hourlyStats:     make(map[string][]StatsEntry),
		dailyStats:      make(map[string][]StatsEntry),
		weeklyStats:     make(map[string][]StatsEntry),
		monthlyStats:    make(map[string][]StatsEntry),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// ========== 生命周期 ==========

// Start 启动分析器
func (a *NetflowAnalyzer) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return nil
	}
	a.running = true
	a.startTime = time.Now()
	a.stopCh = make(chan struct{})

	// 启动采样协程
	go a.sampleLoop()

	return nil
}

// Stop 停止分析器
func (a *NetflowAnalyzer) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}
	a.running = false
	close(a.stopCh)
	return nil
}

// IsRunning 是否正在运行
func (a *NetflowAnalyzer) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// ========== 内部采样 ==========

// sampleLoop 采样循环
func (a *NetflowAnalyzer) sampleLoop() {
	ticker := time.NewTicker(a.sampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCh:
			return
		case <-ticker.C:
			a.collectSample()
		}
	}
}

// collectSample 采集一次样本
func (a *NetflowAnalyzer) collectSample() {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()

	// 更新各接口的带宽计算
	for _, iface := range a.interfaces {
		elapsed := now.Sub(iface.LastUpdated).Seconds()
		if elapsed > 0 {
			iface.BandwidthIn = float64(iface.BytesIn*8) / elapsed
			iface.BandwidthOut = float64(iface.BytesOut*8) / elapsed
			iface.LastUpdated = now
		}
	}

	// 检查告警
	a.checkAlerts(now)
}

// ========== 流量记录 ==========

// RecordFlow 记录一条流量
func (a *NetflowAnalyzer) RecordFlow(record FlowRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 设置时间戳
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}
	if record.ID == "" {
		record.ID = fmt.Sprintf("flow_%d_%d", record.Timestamp.UnixNano(), len(a.records))
	}

	// 存储记录
	a.records = append(a.records, record)

	// 限制记录数
	if len(a.records) > a.maxRecords {
		a.records = a.records[len(a.records)-a.maxRecords:]
	}

	// 更新接口统计
	a.updateInterfaceStats(record)

	// 更新连接追踪
	a.updateConnections(record)

	// 检查带宽策略
	a.checkBandwidthPolicy(record)

	// 检查异常
	a.checkAnomalies(record)
}

// RecordFlows 批量记录流量
func (a *NetflowAnalyzer) RecordFlows(records []FlowRecord) {
	for i := range records {
		a.RecordFlow(records[i])
	}
}

// ========== 内部方法 ==========

// updateInterfaceStats 更新接口统计（调用前需持有写锁）
func (a *NetflowAnalyzer) updateInterfaceStats(record FlowRecord) {
	iface, exists := a.interfaces[record.Interface]
	if !exists {
		iface = &InterfaceStats{
			Name:        record.Interface,
			LastUpdated: record.Timestamp,
		}
		a.interfaces[record.Interface] = iface
	}

	iface.BytesIn += record.BytesIn
	iface.BytesOut += record.BytesOut
	iface.PacketsIn += record.PacketsIn
	iface.PacketsOut += record.PacketsOut
	iface.LastUpdated = record.Timestamp
}

// updateConnections 更新连接追踪（调用前需持有写锁）
func (a *NetflowAnalyzer) updateConnections(record FlowRecord) {
	key := fmt.Sprintf("%s:%d->%s:%d:%s",
		record.SrcIP, record.SrcPort,
		record.DstIP, record.DstPort,
		record.Protocol)

	conn, exists := a.connections[key]
	if !exists {
		conn = &ConnectionInfo{
			SrcIP:     record.SrcIP,
			DstIP:     record.DstIP,
			SrcPort:   record.SrcPort,
			DstPort:   record.DstPort,
			Protocol:  record.Protocol,
			Interface: record.Interface,
			StartTime: record.Timestamp,
		}
		a.connections[key] = conn
	}

	conn.BytesIn += record.BytesIn
	conn.BytesOut += record.BytesOut
}

// checkBandwidthPolicy 检查带宽策略（调用前需持有写锁）
func (a *NetflowAnalyzer) checkBandwidthPolicy(record FlowRecord) {
	for _, policy := range a.policies {
		if !policy.Enabled {
			continue
		}
		if !matchPolicy(policy, record) {
			continue
		}

		// 检查入站限制
		if policy.MaxInBps > 0 {
			currentBps := record.BytesIn * 8
			if currentBps > policy.MaxInBps {
				a.violations = append(a.violations, PolicyViolation{
					PolicyID:   policy.ID,
					PolicyName: policy.Name,
					IP:         record.SrcIP,
					Port:       record.SrcPort,
					ActualBps:  currentBps,
					LimitBps:   policy.MaxInBps,
					Direction:  "in",
					Timestamp:  record.Timestamp,
				})
			}
		}

		// 检查出站限制
		if policy.MaxOutBps > 0 {
			currentBps := record.BytesOut * 8
			if currentBps > policy.MaxOutBps {
				a.violations = append(a.violations, PolicyViolation{
					PolicyID:   policy.ID,
					PolicyName: policy.Name,
					IP:         record.DstIP,
					Port:       record.DstPort,
					ActualBps:  currentBps,
					LimitBps:   policy.MaxOutBps,
					Direction:  "out",
					Timestamp:  record.Timestamp,
				})
			}
		}
	}

	// 限制违规记录数
	if len(a.violations) > a.maxRecords {
		a.violations = a.violations[len(a.violations)-a.maxRecords:]
	}
}

// matchPolicy 检查流量是否匹配策略
func matchPolicy(policy *BandwidthPolicy, record FlowRecord) bool {
	// 检查IP
	if policy.TargetIP != "" {
		if record.SrcIP != policy.TargetIP && record.DstIP != policy.TargetIP {
			return false
		}
	}

	// 检查端口
	if policy.TargetPort != 0 {
		if record.SrcPort != policy.TargetPort && record.DstPort != policy.TargetPort {
			return false
		}
	}

	// 检查协议
	if policy.Protocol != "" {
		if !strings.EqualFold(record.Protocol, policy.Protocol) {
			return false
		}
	}

	return true
}

// checkAnomalies 检查流量异常（调用前需持有写锁）
func (a *NetflowAnalyzer) checkAnomalies(record FlowRecord) {
	now := record.Timestamp

	// 检查流量突增
	a.checkTrafficSurge(record, now)

	// 检查连接数异常
	a.checkConnectionAnomaly(now)

	// 检查DDoS
	a.checkDDoS(record, now)
}

// checkTrafficSurge 检查流量突增（调用前需持有写锁）
func (a *NetflowAnalyzer) checkTrafficSurge(record FlowRecord, now time.Time) {
	iface, exists := a.interfaces[record.Interface]
	if !exists {
		return
	}

	// 计算历史平均流量
	totalBytes := iface.BytesIn + iface.BytesOut
	elapsed := now.Sub(iface.LastUpdated).Seconds()
	if elapsed < 1 {
		elapsed = 1
	}
	avgBps := float64(totalBytes*8) / elapsed

	// 当前流量
	currentBps := float64((record.BytesIn+record.BytesOut)*8) / elapsed

	// 如果当前流量超过历史平均的N倍，触发告警
	if avgBps > 0 && currentBps > avgBps*a.surgeMultiplier {
		alertKey := fmt.Sprintf("surge_%s", record.Interface)
		if a.canAlert(alertKey, now) {
			a.addAlert(TrafficAlert{
				ID:        fmt.Sprintf("alert_%d", now.UnixNano()),
				Type:      "surge",
				Level:     AlertLevelWarning,
				Interface: record.Interface,
				Message:   fmt.Sprintf("接口 %s 流量突增: %.2f Mbps (平均: %.2f Mbps)", record.Interface, currentBps/1e6, avgBps/1e6),
				Value:     currentBps,
				Threshold: avgBps * a.surgeMultiplier,
				Timestamp: now,
			})
		}
	}
}

// checkConnectionAnomaly 检查连接数异常（调用前需持有写锁）
func (a *NetflowAnalyzer) checkConnectionAnomaly(now time.Time) {
	totalConns := int64(len(a.connections))

	// 超过阈值视为异常
	if totalConns > int64(a.ddosThreshold) {
		alertKey := "connection_anomaly"
		if a.canAlert(alertKey, now) {
			a.addAlert(TrafficAlert{
				ID:        fmt.Sprintf("alert_%d", now.UnixNano()),
				Type:      "connection_anomaly",
				Level:     AlertLevelWarning,
				Message:   fmt.Sprintf("连接数异常: %d 个活跃连接", totalConns),
				Value:     float64(totalConns),
				Threshold: float64(a.ddosThreshold),
				Timestamp: now,
			})
		}
	}
}

// checkDDoS 检查DDoS攻击（调用前需持有写锁）
func (a *NetflowAnalyzer) checkDDoS(record FlowRecord, now time.Time) {
	// 统计同一目标IP的连接数
	targetConns := int64(0)
	for _, conn := range a.connections {
		if conn.DstIP == record.DstIP {
			targetConns++
		}
	}

	if targetConns > int64(a.ddosThreshold) {
		alertKey := fmt.Sprintf("ddos_%s", record.DstIP)
		if a.canAlert(alertKey, now) {
			a.addAlert(TrafficAlert{
				ID:        fmt.Sprintf("alert_%d", now.UnixNano()),
				Type:      "ddos",
				Level:     AlertLevelCritical,
				Interface: record.Interface,
				Message:   fmt.Sprintf("疑似DDoS攻击: 目标IP %s 连接数 %d", record.DstIP, targetConns),
				Value:     float64(targetConns),
				Threshold: float64(a.ddosThreshold),
				Timestamp: now,
			})
		}
	}
}

// canAlert 检查是否可以发送告警（冷却检查）
func (a *NetflowAnalyzer) canAlert(key string, now time.Time) bool {
	lastAlert, exists := a.alertTimers[key]
	if !exists {
		return true
	}
	return now.Sub(lastAlert) > a.alertCooldown
}

// addAlert 添加告警（调用前需持有写锁）
func (a *NetflowAnalyzer) addAlert(alert TrafficAlert) {
	a.alerts = append(a.alerts, alert)
	a.alertTimers[alert.Type+"_"+alert.Interface] = alert.Timestamp

	// 限制告警记录数
	if len(a.alerts) > a.maxRecords {
		a.alerts = a.alerts[len(a.alerts)-a.maxRecords:]
	}
}

// checkAlerts 定期检查告警状态（调用前需持有写锁）
func (a *NetflowAnalyzer) checkAlerts(now time.Time) {
	// 检查连接数
	totalConns := int64(len(a.connections))
	if totalConns > int64(a.ddosThreshold) {
		alertKey := "ddos_check"
		if a.canAlert(alertKey, now) {
			a.addAlert(TrafficAlert{
				ID:        fmt.Sprintf("alert_%d", now.UnixNano()),
				Type:      "ddos",
				Level:     AlertLevelCritical,
				Message:   fmt.Sprintf("持续DDoS攻击: 连接数 %d", totalConns),
				Value:     float64(totalConns),
				Threshold: float64(a.ddosThreshold),
				Timestamp: now,
			})
		}
	}
}

// ========== 查询接口 ==========

// GetRealtimeSnapshot 获取实时流量快照
func (a *NetflowAnalyzer) GetRealtimeSnapshot() TrafficSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.getRealtimeSnapshotUnsafe()
}

// getRealtimeSnapshotUnsafe 内部方法（调用前需持有读锁）
func (a *NetflowAnalyzer) getRealtimeSnapshotUnsafe() TrafficSnapshot {
	snapshot := TrafficSnapshot{
		Timestamp:  time.Now(),
		Interfaces: make(map[string]InterfaceStats),
	}

	totalBandwidthIn := 0.0
	totalBandwidthOut := 0.0

	for name, iface := range a.interfaces {
		snapshot.Interfaces[name] = InterfaceStats{
			Name:         iface.Name,
			BytesIn:      iface.BytesIn,
			BytesOut:     iface.BytesOut,
			PacketsIn:    iface.PacketsIn,
			PacketsOut:   iface.PacketsOut,
			BandwidthIn:  iface.BandwidthIn,
			BandwidthOut: iface.BandwidthOut,
			Connections:  iface.Connections,
			LastUpdated:  iface.LastUpdated,
		}
		totalBandwidthIn += iface.BandwidthIn
		totalBandwidthOut += iface.BandwidthOut
	}

	snapshot.TotalBandwidthIn = totalBandwidthIn
	snapshot.TotalBandwidthOut = totalBandwidthOut
	snapshot.TotalConnections = int64(len(a.connections))
	snapshot.Protocols = a.getProtocolDistribution()

	return snapshot
}

// GetInterfaceStats 获取指定接口统计
func (a *NetflowAnalyzer) GetInterfaceStats(name string) (InterfaceStats, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	iface, exists := a.interfaces[name]
	if !exists {
		return InterfaceStats{}, ErrInterfaceNotFound
	}

	return InterfaceStats{
		Name:         iface.Name,
		BytesIn:      iface.BytesIn,
		BytesOut:     iface.BytesOut,
		PacketsIn:    iface.PacketsIn,
		PacketsOut:   iface.PacketsOut,
		BandwidthIn:  iface.BandwidthIn,
		BandwidthOut: iface.BandwidthOut,
		Connections:  iface.Connections,
		LastUpdated:  iface.LastUpdated,
	}, nil
}

// GetAllInterfaceStats 获取所有接口统计
func (a *NetflowAnalyzer) GetAllInterfaceStats() map[string]InterfaceStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]InterfaceStats, len(a.interfaces))
	for name, iface := range a.interfaces {
		result[name] = InterfaceStats{
			Name:         iface.Name,
			BytesIn:      iface.BytesIn,
			BytesOut:     iface.BytesOut,
			PacketsIn:    iface.PacketsIn,
			PacketsOut:   iface.PacketsOut,
			BandwidthIn:  iface.BandwidthIn,
			BandwidthOut: iface.BandwidthOut,
			Connections:  iface.Connections,
			LastUpdated:  iface.LastUpdated,
		}
	}
	return result
}

// GetActiveConnections 获取活跃连接数
func (a *NetflowAnalyzer) GetActiveConnections() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return int64(len(a.connections))
}

// GetConnectionList 获取连接列表
func (a *NetflowAnalyzer) GetConnectionList() []ConnectionInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]ConnectionInfo, 0, len(a.connections))
	for _, conn := range a.connections {
		result = append(result, ConnectionInfo{
			SrcIP:     conn.SrcIP,
			DstIP:     conn.DstIP,
			SrcPort:   conn.SrcPort,
			DstPort:   conn.DstPort,
			Protocol:  conn.Protocol,
			Interface: conn.Interface,
			StartTime: conn.StartTime,
			BytesIn:   conn.BytesIn,
			BytesOut:  conn.BytesOut,
		})
	}
	return result
}

// GetProtocolDistribution 获取协议分布
func (a *NetflowAnalyzer) GetProtocolDistribution() []ProtocolDistribution {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.getProtocolDistribution()
}

// getProtocolDistribution 内部方法（调用前需持有读锁）
func (a *NetflowAnalyzer) getProtocolDistribution() []ProtocolDistribution {
	protocolBytes := make(map[string]uint64)
	protocolPackets := make(map[string]uint64)
	totalBytes := uint64(0)

	for _, record := range a.records {
		protocolBytes[record.Protocol] += record.BytesIn + record.BytesOut
		protocolPackets[record.Protocol] += record.PacketsIn + record.PacketsOut
		totalBytes += record.BytesIn + record.BytesOut
	}

	result := make([]ProtocolDistribution, 0, len(protocolBytes))
	for proto, bytes := range protocolBytes {
		percent := 0.0
		if totalBytes > 0 {
			percent = float64(bytes) / float64(totalBytes) * 100
		}
		result = append(result, ProtocolDistribution{
			Protocol: proto,
			Bytes:    bytes,
			Packets:  protocolPackets[proto],
			Percent:  percent,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Bytes > result[j].Bytes
	})

	return result
}

// ========== 统计查询 ==========

// GetTrafficStats 获取流量统计
func (a *NetflowAnalyzer) GetTrafficStats(granularity string, start, end time.Time) (TrafficStats, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	switch granularity {
	case GranularityHourly, GranularityDaily, GranularityWeekly, GranularityMonthly:
	default:
		return TrafficStats{}, ErrInvalidGranularity
	}

	// 过滤时间范围内的记录
	var filtered []FlowRecord
	for _, r := range a.records {
		if !r.Timestamp.Before(start) && !r.Timestamp.After(end) {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == 0 {
		return TrafficStats{
			StartTime:   start,
			EndTime:     end,
			Granularity: granularity,
		}, nil
	}

	// 按时间间隔分组
	interval := getInterval(granularity)
	groups := make(map[int64]*StatsEntry)

	totalBytesIn := uint64(0)
	totalBytesOut := uint64(0)
	totalPackets := uint64(0)

	for _, r := range filtered {
		totalBytesIn += r.BytesIn
		totalBytesOut += r.BytesOut
		totalPackets += r.PacketsIn + r.PacketsOut

		slot := r.Timestamp.Truncate(interval).Unix()
		entry, exists := groups[slot]
		if !exists {
			entry = &StatsEntry{
				Timestamp: r.Timestamp.Truncate(interval),
			}
			groups[slot] = entry
		}
		entry.BytesIn += r.BytesIn
		entry.BytesOut += r.BytesOut
		entry.Packets += r.PacketsIn + r.PacketsOut
	}

	// 转换为切片并排序
	entries := make([]StatsEntry, 0, len(groups))
	peakBandwidth := 0.0
	for _, entry := range groups {
		entry.Bandwidth = float64((entry.BytesIn+entry.BytesOut)*8) / interval.Seconds()
		if entry.Bandwidth > peakBandwidth {
			peakBandwidth = entry.Bandwidth
		}
		entries = append(entries, *entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	elapsed := end.Sub(start).Seconds()
	avgBandwidth := 0.0
	if elapsed > 0 {
		avgBandwidth = float64((totalBytesIn+totalBytesOut)*8) / elapsed
	}

	return TrafficStats{
		StartTime:     start,
		EndTime:       end,
		Granularity:   granularity,
		TotalBytesIn:  totalBytesIn,
		TotalBytesOut: totalBytesOut,
		TotalPackets:  totalPackets,
		AvgBandwidth:  avgBandwidth,
		PeakBandwidth: peakBandwidth,
		Entries:       entries,
	}, nil
}

// getInterval 获取时间间隔
func getInterval(granularity string) time.Duration {
	switch granularity {
	case GranularityHourly:
		return time.Hour
	case GranularityDaily:
		return 24 * time.Hour
	case GranularityWeekly:
		return 7 * 24 * time.Hour
	case GranularityMonthly:
		return 30 * 24 * time.Hour
	default:
		return time.Hour
	}
}

// GetStatsByIP 按IP分组统计
func (a *NetflowAnalyzer) GetStatsByIP(start, end time.Time, topN int) []GroupedStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	ipStats := make(map[string]*GroupedStats)

	for _, r := range a.records {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}

		// 源IP统计
		srcStat, exists := ipStats[r.SrcIP]
		if !exists {
			srcStat = &GroupedStats{GroupKey: r.SrcIP}
			ipStats[r.SrcIP] = srcStat
		}
		srcStat.BytesOut += r.BytesOut
		srcStat.Packets += r.PacketsOut
		srcStat.Flows++

		// 目标IP统计
		dstStat, exists := ipStats[r.DstIP]
		if !exists {
			dstStat = &GroupedStats{GroupKey: r.DstIP}
			ipStats[r.DstIP] = dstStat
		}
		dstStat.BytesIn += r.BytesIn
		dstStat.Packets += r.PacketsIn
		dstStat.Flows++
	}

	result := make([]GroupedStats, 0, len(ipStats))
	for _, stat := range ipStats {
		result = append(result, *stat)
	}

	sort.Slice(result, func(i, j int) bool {
		return (result[i].BytesIn + result[i].BytesOut) > (result[j].BytesIn + result[j].BytesOut)
	})

	if topN > 0 && topN < len(result) {
		result = result[:topN]
	}
	return result
}

// GetStatsByPort 按端口分组统计
func (a *NetflowAnalyzer) GetStatsByPort(start, end time.Time, topN int) []GroupedStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	portStats := make(map[uint16]*GroupedStats)

	for _, r := range a.records {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}

		// 目标端口统计
		stat, exists := portStats[r.DstPort]
		if !exists {
			stat = &GroupedStats{GroupKey: fmt.Sprintf("%d", r.DstPort)}
			portStats[r.DstPort] = stat
		}
		stat.BytesIn += r.BytesIn
		stat.BytesOut += r.BytesOut
		stat.Packets += r.PacketsIn + r.PacketsOut
		stat.Flows++
	}

	result := make([]GroupedStats, 0, len(portStats))
	for _, stat := range portStats {
		result = append(result, *stat)
	}

	sort.Slice(result, func(i, j int) bool {
		return (result[i].BytesIn + result[i].BytesOut) > (result[j].BytesIn + result[j].BytesOut)
	})

	if topN > 0 && topN < len(result) {
		result = result[:topN]
	}
	return result
}

// GetStatsByProtocol 按协议分组统计
func (a *NetflowAnalyzer) GetStatsByProtocol(start, end time.Time) []GroupedStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	protoStats := make(map[string]*GroupedStats)

	for _, r := range a.records {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}

		stat, exists := protoStats[r.Protocol]
		if !exists {
			stat = &GroupedStats{GroupKey: r.Protocol}
			protoStats[r.Protocol] = stat
		}
		stat.BytesIn += r.BytesIn
		stat.BytesOut += r.BytesOut
		stat.Packets += r.PacketsIn + r.PacketsOut
		stat.Flows++
	}

	result := make([]GroupedStats, 0, len(protoStats))
	for _, stat := range protoStats {
		result = append(result, *stat)
	}

	sort.Slice(result, func(i, j int) bool {
		return (result[i].BytesIn + result[i].BytesOut) > (result[j].BytesIn + result[j].BytesOut)
	})

	return result
}

// ========== 带宽策略管理 ==========

// AddBandwidthPolicy 添加带宽策略
func (a *NetflowAnalyzer) AddBandwidthPolicy(policy BandwidthPolicy) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if policy.ID == "" {
		return ErrInvalidPolicy
	}
	if _, exists := a.policies[policy.ID]; exists {
		return ErrPolicyExists
	}
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = time.Now()
	}
	a.policies[policy.ID] = &policy
	return nil
}

// RemoveBandwidthPolicy 删除带宽策略
func (a *NetflowAnalyzer) RemoveBandwidthPolicy(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.policies[id]; !exists {
		return ErrPolicyNotFound
	}
	delete(a.policies, id)
	return nil
}

// UpdateBandwidthPolicy 更新带宽策略
func (a *NetflowAnalyzer) UpdateBandwidthPolicy(policy BandwidthPolicy) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if policy.ID == "" {
		return ErrInvalidPolicy
	}
	if _, exists := a.policies[policy.ID]; !exists {
		return ErrPolicyNotFound
	}
	a.policies[policy.ID] = &policy
	return nil
}

// GetBandwidthPolicy 获取带宽策略
func (a *NetflowAnalyzer) GetBandwidthPolicy(id string) (BandwidthPolicy, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	policy, exists := a.policies[id]
	if !exists {
		return BandwidthPolicy{}, ErrPolicyNotFound
	}
	return *policy, nil
}

// GetAllBandwidthPolicies 获取所有带宽策略
func (a *NetflowAnalyzer) GetAllBandwidthPolicies() []BandwidthPolicy {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]BandwidthPolicy, 0, len(a.policies))
	for _, p := range a.policies {
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority
	})
	return result
}

// EnablePolicy 启用策略
func (a *NetflowAnalyzer) EnablePolicy(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	policy, exists := a.policies[id]
	if !exists {
		return ErrPolicyNotFound
	}
	policy.Enabled = true
	return nil
}

// DisablePolicy 禁用策略
func (a *NetflowAnalyzer) DisablePolicy(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	policy, exists := a.policies[id]
	if !exists {
		return ErrPolicyNotFound
	}
	policy.Enabled = false
	return nil
}

// GetPolicyViolations 获取策略违规记录
func (a *NetflowAnalyzer) GetPolicyViolations(limit int) []PolicyViolation {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit <= 0 || limit > len(a.violations) {
		limit = len(a.violations)
	}

	// 返回最近的违规记录
	start := len(a.violations) - limit
	result := make([]PolicyViolation, limit)
	copy(result, a.violations[start:])
	return result
}

// ========== 告警管理 ==========

// GetAlerts 获取告警列表
func (a *NetflowAnalyzer) GetAlerts(limit int) []TrafficAlert {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit <= 0 || limit > len(a.alerts) {
		limit = len(a.alerts)
	}

	start := len(a.alerts) - limit
	result := make([]TrafficAlert, limit)
	copy(result, a.alerts[start:])
	return result
}

// GetAlertsByType 按类型获取告警
func (a *NetflowAnalyzer) GetAlertsByType(alertType string) []TrafficAlert {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []TrafficAlert
	for _, alert := range a.alerts {
		if alert.Type == alertType {
			result = append(result, alert)
		}
	}
	return result
}

// GetAlertsByLevel 按级别获取告警
func (a *NetflowAnalyzer) GetAlertsByLevel(level string) []TrafficAlert {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []TrafficAlert
	for _, alert := range a.alerts {
		if alert.Level == level {
			result = append(result, alert)
		}
	}
	return result
}

// ResolveAlert 解决告警
func (a *NetflowAnalyzer) ResolveAlert(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.alerts {
		if a.alerts[i].ID == id {
			a.alerts[i].Resolved = true
			return nil
		}
	}
	return fmt.Errorf("告警不存在: %s", id)
}

// ClearAlerts 清除已解决的告警
func (a *NetflowAnalyzer) ClearAlerts() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	before := len(a.alerts)
	var active []TrafficAlert
	for _, alert := range a.alerts {
		if !alert.Resolved {
			active = append(active, alert)
		}
	}
	a.alerts = active
	return before - len(a.alerts)
}

// ========== 报表生成 ==========

// GenerateReport 生成流量报表
func (a *NetflowAnalyzer) GenerateReport(title string, start, end time.Time, topN int) TrafficReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	report := TrafficReport{
		Title:       title,
		GeneratedAt: time.Now(),
		StartTime:   start,
		EndTime:     end,
	}

	// 过滤记录
	var filtered []FlowRecord
	srcIPs := make(map[string]bool)
	dstIPs := make(map[string]bool)

	for _, r := range a.records {
		if !r.Timestamp.Before(start) && !r.Timestamp.After(end) {
			filtered = append(filtered, r)
			srcIPs[r.SrcIP] = true
			dstIPs[r.DstIP] = true
		}
	}

	// 摘要
	totalBytesIn := uint64(0)
	totalBytesOut := uint64(0)
	totalPackets := uint64(0)

	for _, r := range filtered {
		totalBytesIn += r.BytesIn
		totalBytesOut += r.BytesOut
		totalPackets += r.PacketsIn + r.PacketsOut
	}

	elapsed := end.Sub(start).Seconds()
	avgBw := 0.0
	if elapsed > 0 {
		avgBw = float64((totalBytesIn+totalBytesOut)*8) / elapsed
	}

	// 统计告警数
	alertCount := 0
	for _, alert := range a.alerts {
		if !alert.Timestamp.Before(start) && !alert.Timestamp.After(end) {
			alertCount++
		}
	}

	report.Summary = ReportSummary{
		TotalBytesIn:  totalBytesIn,
		TotalBytesOut: totalBytesOut,
		TotalPackets:  totalPackets,
		TotalFlows:    len(filtered),
		AvgBandwidth:  avgBw,
		UniqueSrcIPs:  len(srcIPs),
		UniqueDstIPs:  len(dstIPs),
		AlertCount:    alertCount,
	}

	// Top Talkers (按IP)
	ipStats := make(map[string]uint64)
	for _, r := range filtered {
		ipStats[r.SrcIP] += r.BytesOut
		ipStats[r.DstIP] += r.BytesIn
	}
	report.TopTalkers = buildTopN(ipStats, topN, "bytes")

	// Top Ports
	portStats := make(map[string]uint64)
	for _, r := range filtered {
		key := fmt.Sprintf("%d", r.DstPort)
		portStats[key] += r.BytesIn + r.BytesOut
	}
	report.TopPorts = buildTopN(portStats, topN, "bytes")

	// Top Protocols
	protoStats := make(map[string]uint64)
	for _, r := range filtered {
		protoStats[r.Protocol] += r.BytesIn + r.BytesOut
	}
	report.TopProtocols = buildTopN(protoStats, topN, "bytes")

	// 趋势（按小时）
	trendMap := make(map[int64]*TrendEntry)
	for _, r := range filtered {
		slot := r.Timestamp.Truncate(time.Hour).Unix()
		entry, exists := trendMap[slot]
		if !exists {
			entry = &TrendEntry{
				Timestamp: r.Timestamp.Truncate(time.Hour),
			}
			trendMap[slot] = entry
		}
		entry.BytesIn += r.BytesIn
		entry.BytesOut += r.BytesOut
	}

	for _, entry := range trendMap {
		entry.Bandwidth = float64((entry.BytesIn+entry.BytesOut)*8) / 3600
		report.Trends = append(report.Trends, *entry)
	}

	sort.Slice(report.Trends, func(i, j int) bool {
		return report.Trends[i].Timestamp.Before(report.Trends[j].Timestamp)
	})

	// 告警
	for _, alert := range a.alerts {
		if !alert.Timestamp.Before(start) && !alert.Timestamp.After(end) {
			report.Alerts = append(report.Alerts, alert)
		}
	}

	// 计算峰值带宽
	peakBw := 0.0
	for _, entry := range report.Trends {
		if entry.Bandwidth > peakBw {
			peakBw = entry.Bandwidth
		}
	}
	report.Summary.PeakBandwidth = peakBw

	return report
}

// buildTopN 构建Top N列表
func buildTopN(stats map[string]uint64, topN int, unit string) []TopNEntry {
	type kv struct {
		Key   string
		Value uint64
	}
	var sorted []kv
	for k, v := range stats {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	if topN > 0 && topN < len(sorted) {
		sorted = sorted[:topN]
	}

	result := make([]TopNEntry, len(sorted))
	for i, item := range sorted {
		result[i] = TopNEntry{
			Rank:  i + 1,
			Key:   item.Key,
			Value: item.Value,
			Unit:  unit,
		}
	}
	return result
}

// ========== 导出功能 ==========

// ExportJSON 导出为JSON
func (a *NetflowAnalyzer) ExportJSON(start, end time.Time) ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var filtered []FlowRecord
	for _, r := range a.records {
		if !r.Timestamp.Before(start) && !r.Timestamp.After(end) {
			filtered = append(filtered, r)
		}
	}

	return json.MarshalIndent(filtered, "", "  ")
}

// ExportCSV 导出为CSV
func (a *NetflowAnalyzer) ExportCSV(start, end time.Time) ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// 写入表头
	header := []string{
		"timestamp", "interface", "src_ip", "dst_ip",
		"src_port", "dst_port", "protocol",
		"bytes_in", "bytes_out", "packets_in", "packets_out",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// 写入数据
	for _, r := range a.records {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}
		record := []string{
			r.Timestamp.Format(time.RFC3339),
			r.Interface,
			r.SrcIP,
			r.DstIP,
			fmt.Sprintf("%d", r.SrcPort),
			fmt.Sprintf("%d", r.DstPort),
			r.Protocol,
			fmt.Sprintf("%d", r.BytesIn),
			fmt.Sprintf("%d", r.BytesOut),
			fmt.Sprintf("%d", r.PacketsIn),
			fmt.Sprintf("%d", r.PacketsOut),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return []byte(buf.String()), writer.Error()
}

// ExportReportJSON 导出报表为JSON
func (a *NetflowAnalyzer) ExportReportJSON(report TrafficReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// ========== IP验证工具 ==========

// ValidateIP 验证IP地址
func ValidateIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// NormalizeIP 标准化IP地址
func NormalizeIP(ip string) (string, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", ErrInvalidIP
	}
	return parsed.String(), nil
}

// IsPrivateIP 判断是否为私有IP
func IsPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.IsPrivate() || parsed.IsLoopback()
}

// ========== 流量计算工具 ==========

// FormatBytes 格式化字节数
func FormatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatBandwidth 格式化带宽
func FormatBandwidth(bps float64) string {
	const (
		Kbps = 1000
		Mbps = Kbps * 1000
		Gbps = Mbps * 1000
	)

	switch {
	case bps >= Gbps:
		return fmt.Sprintf("%.2f Gbps", bps/Gbps)
	case bps >= Mbps:
		return fmt.Sprintf("%.2f Mbps", bps/Mbps)
	case bps >= Kbps:
		return fmt.Sprintf("%.2f Kbps", bps/Kbps)
	default:
		return fmt.Sprintf("%.0f bps", bps)
	}
}

// ========== 状态查询 ==========

// GetRecordCount 获取记录总数
func (a *NetflowAnalyzer) GetRecordCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.records)
}

// GetAlertCount 获取告警总数
func (a *NetflowAnalyzer) GetAlertCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.alerts)
}

// GetViolationCount 获取违规记录总数
func (a *NetflowAnalyzer) GetViolationCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.violations)
}

// GetPolicyCount 获取策略数
func (a *NetflowAnalyzer) GetPolicyCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.policies)
}

// Reset 重置分析器数据
func (a *NetflowAnalyzer) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.records = make([]FlowRecord, 0, 1000)
	a.interfaces = make(map[string]*InterfaceStats)
	a.connections = make(map[string]*ConnectionInfo)
	a.policies = make(map[string]*BandwidthPolicy)
	a.alerts = make([]TrafficAlert, 0)
	a.violations = make([]PolicyViolation, 0)
	a.alertTimers = make(map[string]time.Time)
}

// GetUptime 获取运行时间
func (a *NetflowAnalyzer) GetUptime() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.running {
		return 0
	}
	return time.Since(a.startTime)
}
