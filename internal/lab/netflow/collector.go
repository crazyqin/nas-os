// Package netflow - 流量收集器
// 支持sFlow/NetFlow协议采集，实时流量统计
// 对标群晖Traffic Control的底层数据采集
package netflow

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// Collector 流量收集器
// 负责接收sFlow/NetFlow数据包，解析并缓存流量记录.
type Collector struct {
	mu      sync.RWMutex
	config  CollectorConfig
	records []FlowRecord
	stats   TrafficStats
	logger  *zap.Logger

	// 协议统计
	protocolStats map[Protocol]*ProtocolStats
	// 主机流量统计
	hostStats map[string]*HostTraffic
	// 带宽使用历史
	bandwidthHistory []BandwidthUsage

	// 运行状态
	running   bool
	stopCh    chan struct{}
	startTime time.Time
}

// NewCollector 创建流量收集器.
func NewCollector(config CollectorConfig, logger *zap.Logger) *Collector {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Collector{
		config:           config,
		records:          make([]FlowRecord, 0, config.BufferSize),
		protocolStats:    make(map[Protocol]*ProtocolStats),
		hostStats:        make(map[string]*HostTraffic),
		bandwidthHistory: make([]BandwidthUsage, 0),
		logger:           logger,
		stopCh:           make(chan struct{}),
	}
}

// Start 启动流量收集.
func (c *Collector) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	c.running = true
	c.startTime = time.Now()

	// 启动后台刷新协程
	go c.flushLoop()

	c.logger.Info("流量收集器已启动",
		zap.String("listen", c.config.ListenAddress),
		zap.Int("sflow_port", c.config.SFlowPort),
		zap.Int("netflow_port", c.config.NetFlowPort),
		zap.Int("sample_rate", c.config.SampleRate),
	)

	return nil
}

// Stop 停止流量收集.
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	c.running = false
	close(c.stopCh)

	c.logger.Info("流量收集器已停止")
}

// IsRunning 检查收集器是否运行中.
func (c *Collector) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// IngestFlow 注入一条流量记录（供sFlow/NetFlow解析器调用）.
func (c *Collector) IngestFlow(record FlowRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 缓冲区已满时丢弃最旧记录
	if len(c.records) >= c.config.BufferSize {
		c.records = c.records[1:]
	}
	c.records = append(c.records, record)

	// 更新统计
	c.updateStats(record)
}

// IngestBatch 批量注入流量记录.
func (c *Collector) IngestBatch(records []FlowRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, record := range records {
		if len(c.records) >= c.config.BufferSize {
			c.records = c.records[1:]
		}
		c.records = append(c.records, record)
		c.updateStats(record)
	}
}

// updateStats 更新统计数据（需持有写锁）.
func (c *Collector) updateStats(record FlowRecord) {
	// 更新总流量统计
	switch record.Direction {
	case DirectionInbound:
		c.stats.TotalBytesIn += record.Bytes
		c.stats.TotalPacketsIn += record.Packets
	case DirectionOutbound:
		c.stats.TotalBytesOut += record.Bytes
		c.stats.TotalPacketsOut += record.Packets
	}

	// 更新协议统计
	protoStats, ok := c.protocolStats[record.Protocol]
	if !ok {
		protoStats = &ProtocolStats{
			Protocol: record.Protocol,
		}
		c.protocolStats[record.Protocol] = protoStats
	}
	protoStats.Bytes += record.Bytes
	protoStats.Packets += record.Packets

	// 更新主机流量统计
	hostIP := record.SrcIP
	if record.Direction == DirectionInbound {
		hostIP = record.DstIP
	}
	hostStats, ok := c.hostStats[hostIP]
	if !ok {
		hostStats = &HostTraffic{
			IP: hostIP,
		}
		c.hostStats[hostIP] = hostStats
	}
	switch record.Direction {
	case DirectionInbound:
		hostStats.BytesIn += record.Bytes
	case DirectionOutbound:
		hostStats.BytesOut += record.Bytes
	}
	hostStats.TotalBytes += record.Bytes
	hostStats.LastSeen = record.Timestamp
}

// GetRecentRecords 获取最近N条记录.
func (c *Collector) GetRecentRecords(n int) []FlowRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := len(c.records)
	if n > total {
		n = total
	}
	result := make([]FlowRecord, n)
	copy(result, c.records[total-n:])
	return result
}

// GetTrafficStats 获取流量统计.
func (c *Collector) GetTrafficStats() TrafficStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	stats.Timestamp = time.Now()
	stats.ActiveConnections = c.estimateActiveConnections()
	return stats
}

// GetProtocolStats 获取协议统计.
func (c *Collector) GetProtocolStats() []ProtocolStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalBytes := c.stats.TotalBytesIn + c.stats.TotalBytesOut
	stats := make([]ProtocolStats, 0, len(c.protocolStats))

	for _, ps := range c.protocolStats {
		entry := *ps
		if totalBytes > 0 {
			entry.Percentage = float64(ps.Bytes) / float64(totalBytes) * 100
		}
		stats = append(stats, entry)
	}

	return stats
}

// GetTopHosts 获取Top N主机流量.
func (c *Collector) GetTopHosts(n int) []HostTraffic {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hosts := make([]HostTraffic, 0, len(c.hostStats))
	for _, h := range c.hostStats {
		hosts = append(hosts, *h)
	}

	// 简单排序（按TotalBytes降序）
	for i := 0; i < len(hosts); i++ {
		for j := i + 1; j < len(hosts); j++ {
			if hosts[j].TotalBytes > hosts[i].TotalBytes {
				hosts[i], hosts[j] = hosts[j], hosts[i]
			}
		}
	}

	if n > len(hosts) {
		n = len(hosts)
	}
	return hosts[:n]
}

// GetBandwidthHistory 获取带宽历史.
func (c *Collector) GetBandwidthHistory() []BandwidthUsage {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]BandwidthUsage, len(c.bandwidthHistory))
	copy(result, c.bandwidthHistory)
	return result
}

// estimateActiveConnections 估算活跃连接数.
func (c *Collector) estimateActiveConnections() int {
	// 基于最近5分钟内的唯一(src,dst,port)组合估算
	cutoff := time.Now().Add(-5 * time.Minute)
	seen := make(map[string]struct{})

	for i := len(c.records) - 1; i >= 0; i-- {
		r := c.records[i]
		if r.Timestamp.Before(cutoff) {
			break
		}
		key := r.SrcIP + ":" + r.DstIP + ":" + string(r.Protocol)
		seen[key] = struct{}{}
	}

	return len(seen)
}

// Clear 清空所有数据.
func (c *Collector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.records = make([]FlowRecord, 0, c.config.BufferSize)
	c.protocolStats = make(map[Protocol]*ProtocolStats)
	c.hostStats = make(map[string]*HostTraffic)
	c.bandwidthHistory = make([]BandwidthUsage, 0)
	c.stats = TrafficStats{}
}

// flushLoop 定期刷新带宽历史.
func (c *Collector) flushLoop() {
	ticker := time.NewTicker(time.Duration(c.config.FlushIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.flushBandwidth()
		}
	}
}

// flushBandwidth 记录带宽使用快照.
func (c *Collector) flushBandwidth() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	usage := BandwidthUsage{
		Timestamp:     now,
		InterfaceName: "all",
		BytesIn:       c.stats.TotalBytesIn,
		BytesOut:      c.stats.TotalBytesOut,
	}

	// 计算利用率（假设1Gbps链路）
	interval := float64(c.config.FlushIntervalSec)
	if interval > 0 {
		bpsIn := float64(c.stats.CurrentBPSIn) * 8
		bpsOut := float64(c.stats.CurrentBPSOut) * 8
		maxBPS := 1e9 // 1 Gbps
		utilIn := bpsIn / maxBPS * 100
		utilOut := bpsOut / maxBPS * 100
		if utilIn > utilOut {
			usage.Utilization = utilIn
		} else {
			usage.Utilization = utilOut
		}
	}

	c.bandwidthHistory = append(c.bandwidthHistory, usage)

	// 最多保留1440条（24小时，每分钟一条）
	if len(c.bandwidthHistory) > 1440 {
		c.bandwidthHistory = c.bandwidthHistory[len(c.bandwidthHistory)-1440:]
	}

	// 更新峰值
	if c.stats.CurrentBPSIn > c.stats.PeakBPSIn {
		c.stats.PeakBPSIn = c.stats.CurrentBPSIn
	}
	if c.stats.CurrentBPSOut > c.stats.PeakBPSOut {
		c.stats.PeakBPSOut = c.stats.CurrentBPSOut
	}

	// 重置当前速率（下一个窗口重新计算）
	c.stats.CurrentBPSIn = 0
	c.stats.CurrentBPSOut = 0
}
