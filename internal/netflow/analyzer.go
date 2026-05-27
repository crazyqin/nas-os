// Package netflow - 流量分析器
// 异常流量检测、TopN分析、协议解析
// 对标群晖Traffic Control的智能分析功能
package netflow

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Analyzer 流量分析器
// 负责异常检测、TopN统计、协议深度分析
type Analyzer struct {
	mu          sync.RWMutex
	collector   *Collector
	alerts      []AnomalyAlert
	alertCounts map[AnomalyType]int
	logger      *zap.Logger

	// 异常检测配置
	spikeThresholdMBPS   float64 // 流量突增阈值 (MB/s)
	portScanThreshold    int     // 端口扫描检测：不同端口数阈值
	dnsFloodThreshold    int     // DNS洪泛检测：每秒DNS请求阈值
	highConnRateThreshold int    // 高连接速率阈值 (连接/秒)

	// 窗口数据
	windowFlows   []FlowRecord
	windowStart   time.Time
	windowSizeSec int
}

// NewAnalyzer 创建流量分析器
func NewAnalyzer(collector *Collector, logger *zap.Logger) *Analyzer {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Analyzer{
		collector:             collector,
		alerts:                make([]AnomalyAlert, 0),
		alertCounts:           make(map[AnomalyType]int),
		logger:                logger,
		spikeThresholdMBPS:    100,  // 100 MB/s
		portScanThreshold:     20,   // 20个不同端口
		dnsFloodThreshold:     1000, // 1000次/秒
		highConnRateThreshold: 500,  // 500连接/秒
		windowFlows:           make([]FlowRecord, 0),
		windowStart:           time.Now(),
		windowSizeSec:         60, // 1分钟窗口
	}
}

// Analyze 执行一轮完整分析
func (a *Analyzer) Analyze() []AnomalyAlert {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 获取最近记录
	records := a.collector.GetRecentRecords(a.collector.config.BufferSize)
	if len(records) == 0 {
		return nil
	}

	newAlerts := make([]AnomalyAlert, 0)

	// 1. 流量突增检测
	if alert := a.detectTrafficSpike(records); alert != nil {
		newAlerts = append(newAlerts, *alert)
	}

	// 2. 端口扫描检测
	if alert := a.detectPortScan(records); alert != nil {
		newAlerts = append(newAlerts, *alert)
	}

	// 3. DNS洪泛检测
	if alert := a.detectDNSFlood(records); alert != nil {
		newAlerts = append(newAlerts, *alert)
	}

	// 4. 高连接速率检测
	if alert := a.detectHighConnectionRate(records); alert != nil {
		newAlerts = append(newAlerts, *alert)
	}

	// 保存告警
	for _, alert := range newAlerts {
		a.alerts = append(a.alerts, alert)
		a.alertCounts[alert.Type]++
	}

	// 最多保留10000条告警
	if len(a.alerts) > 10000 {
		a.alerts = a.alerts[len(a.alerts)-10000:]
	}

	return newAlerts
}

// detectTrafficSpike 检测流量突增
func (a *Analyzer) detectTrafficSpike(records []FlowRecord) *AnomalyAlert {
	now := time.Now()
	windowStart := now.Add(-time.Duration(a.windowSizeSec) * time.Second)

	var windowBytes int64
	for _, r := range records {
		if r.Timestamp.After(windowStart) {
			windowBytes += r.Bytes
		}
	}

	// 转换为 MB/s
	windowSec := float64(a.windowSizeSec)
	if windowSec <= 0 {
		windowSec = 1
	}
	mbps := float64(windowBytes) / (1024 * 1024) / windowSec

	if mbps > a.spikeThresholdMBPS {
		severity := "warning"
		if mbps > a.spikeThresholdMBPS*2 {
			severity = "critical"
		}

		return &AnomalyAlert{
			ID:          uuid.New().String(),
			Type:        AnomalyTrafficSpike,
			Severity:    severity,
			Description: fmt.Sprintf("流量突增: %.2f MB/s (阈值: %.2f MB/s)", mbps, a.spikeThresholdMBPS),
			DetectedAt:  now,
		}
	}
	return nil
}

// detectPortScan 检测端口扫描
func (a *Analyzer) detectPortScan(records []FlowRecord) *AnomalyAlert {
	now := time.Now()
	windowStart := now.Add(-time.Duration(a.windowSizeSec) * time.Second)

	// 按源IP统计访问的不同目标端口
	srcPorts := make(map[string]map[uint16]struct{})
	for _, r := range records {
		if r.Timestamp.After(windowStart) {
			if _, ok := srcPorts[r.SrcIP]; !ok {
				srcPorts[r.SrcIP] = make(map[uint16]struct{})
			}
			srcPorts[r.SrcIP][r.DstPort] = struct{}{}
		}
	}

	for srcIP, ports := range srcPorts {
		if len(ports) >= a.portScanThreshold {
			severity := "warning"
			if len(ports) >= a.portScanThreshold*2 {
				severity = "critical"
			}

			return &AnomalyAlert{
				ID:          uuid.New().String(),
				Type:        AnomalyPortScan,
				Severity:    severity,
				SourceIP:    srcIP,
				Description: fmt.Sprintf("疑似端口扫描: %s 访问了 %d 个不同端口", srcIP, len(ports)),
				DetectedAt:  now,
			}
		}
	}
	return nil
}

// detectDNSFlood 检测DNS洪泛
func (a *Analyzer) detectDNSFlood(records []FlowRecord) *AnomalyAlert {
	now := time.Now()
	windowStart := now.Add(-time.Duration(a.windowSizeSec) * time.Second)

	dnsCount := 0
	for _, r := range records {
		if r.Timestamp.After(windowStart) && r.Protocol == ProtocolDNS {
			dnsCount++
		}
	}

	dnsPerSec := float64(dnsCount) / float64(a.windowSizeSec)
	if dnsPerSec > float64(a.dnsFloodThreshold) {
		severity := "warning"
		if dnsPerSec > float64(a.dnsFloodThreshold)*3 {
			severity = "critical"
		}

		return &AnomalyAlert{
			ID:          uuid.New().String(),
			Type:        AnomalyDNSFlood,
			Severity:    severity,
			Description: fmt.Sprintf("DNS洪泛攻击: %.0f 次/秒 (阈值: %d 次/秒)", dnsPerSec, a.dnsFloodThreshold),
			DetectedAt:  now,
		}
	}
	return nil
}

// detectHighConnectionRate 检测高连接速率
func (a *Analyzer) detectHighConnectionRate(records []FlowRecord) *AnomalyAlert {
	now := time.Now()
	windowStart := now.Add(-time.Duration(a.windowSizeSec) * time.Second)

	// 统计唯一(src,dst)对
	connections := make(map[string]struct{})
	for _, r := range records {
		if r.Timestamp.After(windowStart) {
			key := r.SrcIP + "->" + r.DstIP
			connections[key] = struct{}{}
		}
	}

	connPerSec := float64(len(connections)) / float64(a.windowSizeSec)
	if connPerSec > float64(a.highConnRateThreshold) {
		return &AnomalyAlert{
			ID:          uuid.New().String(),
			Type:        AnomalyHighConnectionRate,
			Severity:    "warning",
			Description: fmt.Sprintf("高连接速率: %.0f 连接/秒 (阈值: %d 连接/秒)", connPerSec, a.highConnRateThreshold),
			DetectedAt:  now,
		}
	}
	return nil
}

// GetAlerts 获取告警列表
func (a *Analyzer) GetAlerts(limit int, severity string, anomalyType string) []AnomalyAlert {
	a.mu.RLock()
	defer a.mu.RUnlock()

	filtered := make([]AnomalyAlert, 0)
	for i := len(a.alerts) - 1; i >= 0; i-- {
		alert := a.alerts[i]

		if severity != "" && alert.Severity != severity {
			continue
		}
		if anomalyType != "" && string(alert.Type) != anomalyType {
			continue
		}

		filtered = append(filtered, alert)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	return filtered
}

// GetAlertStats 获取告警统计
func (a *Analyzer) GetAlertStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_alerts"] = len(a.alerts)
	stats["by_type"] = a.alertCounts

	severityCounts := make(map[string]int)
	for _, alert := range a.alerts {
		severityCounts[alert.Severity]++
	}
	stats["by_severity"] = severityCounts

	return stats
}

// ResolveAlert 标记告警为已解决
func (a *Analyzer) ResolveAlert(id string) error {
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

// ============================================================
// TopN分析
// ============================================================

// TopHosts Top N 主机分析
func (a *Analyzer) TopHosts(n int) TopNResult {
	hosts := a.collector.GetTopHosts(n)

	entries := make([]TopNEntry, len(hosts))
	for i, h := range hosts {
		entries[i] = TopNEntry{
			Key:   h.IP,
			Value: h.TotalBytes,
			Label: h.Hostname,
		}
	}

	return TopNResult{
		Category:  "hosts",
		Metric:    "bytes",
		Entries:   entries,
		Timestamp: time.Now(),
	}
}

// TopProtocols Top N 协议分析
func (a *Analyzer) TopProtocols(n int) TopNResult {
	protocols := a.collector.GetProtocolStats()

	// 按字节数排序
	sort.Slice(protocols, func(i, j int) bool {
		return protocols[i].Bytes > protocols[j].Bytes
	})

	if n > len(protocols) {
		n = len(protocols)
	}

	entries := make([]TopNEntry, n)
	for i := 0; i < n; i++ {
		entries[i] = TopNEntry{
			Key:   string(protocols[i].Protocol),
			Value: protocols[i].Bytes,
			Label: fmt.Sprintf("%.1f%%", protocols[i].Percentage),
		}
	}

	return TopNResult{
		Category:  "protocols",
		Metric:    "bytes",
		Entries:   entries,
		Timestamp: time.Now(),
	}
}

// TopConversations Top N 会话分析
// 基于最近记录分析(src, dst)对的流量
func (a *Analyzer) TopConversations(n int) TopNResult {
	records := a.collector.GetRecentRecords(10000)

	type conv struct {
		bytes int64
	}
	conversations := make(map[string]*conv)

	for _, r := range records {
		key := r.SrcIP + " <-> " + r.DstIP
		c, ok := conversations[key]
		if !ok {
			c = &conv{}
			conversations[key] = c
		}
		c.bytes += r.Bytes
	}

	// 转换为切片并排序
	type kv struct {
		key   string
		bytes int64
	}
	pairs := make([]kv, 0, len(conversations))
	for k, v := range conversations {
		pairs = append(pairs, kv{k, v.bytes})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].bytes > pairs[j].bytes
	})

	if n > len(pairs) {
		n = len(pairs)
	}

	entries := make([]TopNEntry, n)
	for i := 0; i < n; i++ {
		entries[i] = TopNEntry{
			Key:   pairs[i].key,
			Value: pairs[i].bytes,
		}
	}

	return TopNResult{
		Category:  "conversations",
		Metric:    "bytes",
		Entries:   entries,
		Timestamp: time.Now(),
	}
}

// SetSpikeThreshold 设置流量突增阈值
func (a *Analyzer) SetSpikeThreshold(mbps float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.spikeThresholdMBPS = mbps
}

// SetPortScanThreshold 设置端口扫描阈值
func (a *Analyzer) SetPortScanThreshold(ports int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.portScanThreshold = ports
}

// SetDNSFloodThreshold 设置DNS洪泛阈值
func (a *Analyzer) SetDNSFloodThreshold(perSec int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dnsFloodThreshold = perSec
}

// SetHighConnRateThreshold 设置高连接速率阈值
func (a *Analyzer) SetHighConnRateThreshold(perSec int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.highConnRateThreshold = perSec
}
