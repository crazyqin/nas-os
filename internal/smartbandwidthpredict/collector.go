package smartbandwidthpredict

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Collector 流量采集器
type Collector struct {
	mu       sync.RWMutex
	samples  []*TrafficSample
	config   *Config
	logger   *zap.Logger
	running  bool
	stopCh   chan struct{}
	lastTick time.Time
}

// NewCollector 创建流量采集器
func NewCollector(config *Config, logger *zap.Logger) *Collector {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Collector{
		samples: make([]*TrafficSample, 0, config.MaxSamples),
		config:  config,
		logger:  logger,
		stopCh:  make(chan struct{}),
	}
}

// Start 启动采集器
func (c *Collector) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("采集器已经在运行")
	}

	c.logger.Info("启动流量采集器",
		zap.Strings("interfaces", c.config.Interfaces),
		zap.Duration("interval", c.config.CollectInterval),
	)

	c.running = true
	c.lastTick = time.Now()

	// 启动采集循环
	go c.collectLoop()

	return nil
}

// Stop 停止采集器
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	c.logger.Info("停止流量采集器")
	close(c.stopCh)
	c.running = false
}

// IsRunning 检查采集器是否运行中
func (c *Collector) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// Record 记录流量采样
func (c *Collector) Record(sample *TrafficSample) error {
	if sample == nil {
		return fmt.Errorf("采样数据不能为空")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 添加采样
	c.samples = append(c.samples, sample)
	c.lastTick = time.Now()

	// 清理旧数据
	if len(c.samples) > c.config.MaxSamples {
		c.samples = c.samples[len(c.samples)-c.config.MaxSamples:]
	}

	c.logger.Debug("流量采样记录成功",
		zap.Time("timestamp", sample.Timestamp),
		zap.Float64("inbound_mbps", sample.InboundMbps),
		zap.Float64("outbound_mbps", sample.OutboundMbps),
	)

	return nil
}

// GetSamples 获取所有采样数据
func (c *Collector) GetSamples() []*TrafficSample {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*TrafficSample, len(c.samples))
	copy(result, c.samples)
	return result
}

// GetRecentSamples 获取最近N个采样
func (c *Collector) GetRecentSamples(n int) []*TrafficSample {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if n <= 0 {
		return nil
	}

	if n > len(c.samples) {
		n = len(c.samples)
	}

	result := make([]*TrafficSample, n)
	copy(result, c.samples[len(c.samples)-n:])
	return result
}

// GetSampleCount 获取采样数量
func (c *Collector) GetSampleCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.samples)
}

// ClearSamples 清空采样数据
func (c *Collector) ClearSamples() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.samples = make([]*TrafficSample, 0, c.config.MaxSamples)
	c.logger.Info("采样数据已清空")
}

// GetLatestSample 获取最新采样
func (c *Collector) GetLatestSample() *TrafficSample {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.samples) == 0 {
		return nil
	}

	return c.samples[len(c.samples)-1]
}

// GetSamplesByTimeRange 获取时间范围内的采样
func (c *Collector) GetSamplesByTimeRange(start, end time.Time) []*TrafficSample {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*TrafficSample
	for _, sample := range c.samples {
		if sample.Timestamp.After(start) && sample.Timestamp.Before(end) {
			result = append(result, sample)
		}
	}
	return result
}

// GetSamplesByInterface 获取指定接口的采样
func (c *Collector) GetSamplesByInterface(iface string) []*TrafficSample {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*TrafficSample
	for _, sample := range c.samples {
		if sample.Interface == iface {
			result = append(result, sample)
		}
	}
	return result
}

// AggregateSamples 聚合多个接口的采样
func (c *Collector) AggregateSamples() []*TrafficSample {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.samples) == 0 {
		return nil
	}

	// 按时间戳分组
	timeGroups := make(map[time.Time][]*TrafficSample)
	for _, sample := range c.samples {
		key := sample.Timestamp.Truncate(c.config.CollectInterval)
		timeGroups[key] = append(timeGroups[key], sample)
	}

	// 聚合每组数据
	result := make([]*TrafficSample, 0, len(timeGroups))
	for _, group := range timeGroups {
		aggregated := &TrafficSample{
			Timestamp: group[0].Timestamp,
			Interface: "aggregated",
		}

		for _, sample := range group {
			aggregated.InboundMbps += sample.InboundMbps
			aggregated.OutboundMbps += sample.OutboundMbps
			aggregated.LatencyMs += sample.LatencyMs
			aggregated.PacketLoss += sample.PacketLoss
		}

		// 计算平均值
		count := float64(len(group))
		aggregated.LatencyMs /= count
		aggregated.PacketLoss /= count

		result = append(result, aggregated)
	}

	return result
}

// GetBandwidthStats 获取带宽统计
func (c *Collector) GetBandwidthStats() *BandwidthStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.samples) == 0 {
		return &BandwidthStats{}
	}

	stats := &BandwidthStats{
		SampleCount: len(c.samples),
		StartTime:   c.samples[0].Timestamp,
		EndTime:     c.samples[len(c.samples)-1].Timestamp,
	}

	var totalInbound, totalOutbound, totalLatency, totalLoss float64
	var maxInbound, maxOutbound float64

	for _, sample := range c.samples {
		totalInbound += sample.InboundMbps
		totalOutbound += sample.OutboundMbps
		totalLatency += sample.LatencyMs
		totalLoss += sample.PacketLoss

		if sample.InboundMbps > maxInbound {
			maxInbound = sample.InboundMbps
		}
		if sample.OutboundMbps > maxOutbound {
			maxOutbound = sample.OutboundMbps
		}
	}

	count := float64(stats.SampleCount)
	stats.AvgInboundMbps = totalInbound / count
	stats.AvgOutboundMbps = totalOutbound / count
	stats.AvgLatencyMs = totalLatency / count
	stats.AvgPacketLoss = totalLoss / count
	stats.MaxInboundMbps = maxInbound
	stats.MaxOutboundMbps = maxOutbound

	return stats
}

// UpdateConfig 更新配置
func (c *Collector) UpdateConfig(config *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if config != nil {
		c.config = config
	}
}

// BandwidthStats 带宽统计
type BandwidthStats struct {
	SampleCount     int       `json:"sample_count"`
	AvgInboundMbps  float64   `json:"avg_inbound_mbps"`
	AvgOutboundMbps float64   `json:"avg_outbound_mbps"`
	AvgLatencyMs    float64   `json:"avg_latency_ms"`
	AvgPacketLoss   float64   `json:"avg_packet_loss"`
	MaxInboundMbps  float64   `json:"max_inbound_mbps"`
	MaxOutboundMbps float64   `json:"max_outbound_mbps"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
}

// collectLoop 采集循环
func (c *Collector) collectLoop() {
	ticker := time.NewTicker(c.config.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.collectTraffic()
		}
	}
}

// collectTraffic 采集流量数据
func (c *Collector) collectTraffic() {
	// 遍历所有接口采集流量
	for _, iface := range c.config.Interfaces {
		sample := c.collectFromInterface(iface)
		if sample != nil {
			if err := c.Record(sample); err != nil {
				c.logger.Error("记录流量采样失败",
					zap.String("interface", iface),
					zap.Error(err),
				)
			}
		}
	}
}

// collectFromInterface 从指定接口采集流量
func (c *Collector) collectFromInterface(iface string) *TrafficSample {
	// 这里模拟流量采集，实际实现需要读取系统网络统计
	// 在真实环境中，应该读取 /proc/net/dev 或使用 netlink
	return &TrafficSample{
		Timestamp:    time.Now(),
		InboundMbps:  0,
		OutboundMbps: 0,
		LatencyMs:    0,
		PacketLoss:   0,
		Interface:    iface,
	}
}
