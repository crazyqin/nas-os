package wanreplanner

import (
	"fmt"
	"time"
)

// NewWANPlanner 创建 WAN 链路规划器
func NewWANPlanner(config PlannerConfig) *WANPlanner {
	return &WANPlanner{
		config:  config,
		links:   make(map[string]*WANLink),
		rules:   make(map[string]*QoSRule),
		history: make([]BandwidthSample, 0),
		failoverLog: make([]FailoverEvent, 0),
		stopCh:  make(chan struct{}),
	}
}

// Start 启动规划器
func (p *WANPlanner) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return nil
	}
	p.running = true
	p.stopCh = make(chan struct{})
	go p.healthCheckLoop()
	return nil
}

// Stop 停止规划器
func (p *WANPlanner) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return nil
	}
	close(p.stopCh)
	p.running = false
	return nil
}

// IsRunning 是否运行中
func (p *WANPlanner) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// AddLink 添加 WAN 链路
func (p *WANPlanner) AddLink(link *WANLink) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.links[link.ID]; exists {
		return ErrLinkAlreadyExists
	}
	now := time.Now()
	link.CreatedAt = now
	link.UpdatedAt = now
	if link.Status == "" {
		link.Status = LinkStatusUnknown
	}
	if link.Weight <= 0 {
		link.Weight = 1
	}
	p.links[link.ID] = link
	return nil
}

// RemoveLink 移除 WAN 链路
func (p *WANPlanner) RemoveLink(linkID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.links[linkID]; !exists {
		return ErrLinkNotFound
	}
	delete(p.links, linkID)
	return nil
}

// GetLink 获取链路信息
func (p *WANPlanner) GetLink(linkID string) (*WANLink, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	link, exists := p.links[linkID]
	if !exists {
		return nil, ErrLinkNotFound
	}
	copy := *link
	return &copy, nil
}

// ListLinks 列出所有链路
func (p *WANPlanner) ListLinks() []*WANLink {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*WANLink, 0, len(p.links))
	for _, l := range p.links {
		copy := *l
		result = append(result, &copy)
	}
	return result
}

// GetActiveLinks 获取活跃链路
func (p *WANPlanner) GetActiveLinks() []*WANLink {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*WANLink, 0)
	for _, l := range p.links {
		if l.Status == LinkStatusUp {
			copy := *l
			result = append(result, &copy)
		}
	}
	return result
}

// SelectLink 根据策略选择链路
func (p *WANPlanner) SelectLink(srcIP string) (*WANLink, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	active := p.activeLinks()
	if len(active) == 0 {
		return nil, ErrNoAvailableLinks
	}

	switch p.config.Strategy {
	case StrategyRoundRobin:
		return p.selectRoundRobin(active), nil
	case StrategyWeighted:
		return p.selectWeighted(active), nil
	case StrategyLeastConn:
		return p.selectLeastConn(active), nil
	case StrategySourceHash:
		return p.selectSourceHash(active, srcIP), nil
	default:
		return nil, ErrInvalidStrategy
	}
}

// SetStrategy 设置负载均衡策略
func (p *WANPlanner) SetStrategy(strategy LoadBalanceStrategy) error {
	switch strategy {
	case StrategyRoundRobin, StrategyWeighted, StrategyLeastConn, StrategySourceHash:
	default:
		return ErrInvalidStrategy
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.Strategy = strategy
	p.rrIndex = 0
	return nil
}

// GetConfig 获取当前配置
func (p *WANPlanner) GetConfig() PlannerConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// RecordSample 记录带宽采样
func (p *WANPlanner) RecordSample(sample BandwidthSample) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.history = append(p.history, sample)
	// 裁剪过期数据
	cutoff := time.Now().Add(-p.config.HistoryRetention)
	filtered := make([]BandwidthSample, 0, len(p.history))
	for _, s := range p.history {
		if s.Timestamp.After(cutoff) {
			filtered = append(filtered, s)
		}
	}
	p.history = filtered
}

// GetFailoverLog 获取故障切换日志
func (p *WANPlanner) GetFailoverLog() []FailoverEvent {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]FailoverEvent, len(p.failoverLog))
	copy(result, p.failoverLog)
	return result
}

// UpdateLinkScore 更新链路质量评分
func (p *WANPlanner) UpdateLinkScore(linkID string, latency time.Duration, packetLoss, jitter float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	link, exists := p.links[linkID]
	if !exists {
		return ErrLinkNotFound
	}
	link.Latency = latency
	link.PacketLoss = packetLoss
	link.Jitter = time.Duration(jitter * float64(time.Millisecond))
	link.Score = calculateScore(latency, packetLoss, jitter)
	link.LastCheck = time.Now()
	link.UpdatedAt = time.Now()
	return nil
}

// calculateScore 计算链路质量评分 (0-100)
func calculateScore(latency time.Duration, packetLoss, jitterMs float64) float64 {
	latScore := 100.0 - float64(latency.Milliseconds())*0.1
	if latScore < 0 {
		latScore = 0
	}
	lossScore := 100.0 * (1.0 - packetLoss)
	jitterScore := 100.0 - jitterMs*0.5
	if jitterScore < 0 {
		jitterScore = 0
	}
	return (latScore*0.4 + lossScore*0.4 + jitterScore*0.2)
}

// activeLinks 返回所有状态为 UP 的链路（调用者需持锁）
func (p *WANPlanner) activeLinks() []*WANLink {
	result := make([]*WANLink, 0)
	for _, l := range p.links {
		if l.Status == LinkStatusUp {
			result = append(result, l)
		}
	}
	return result
}

// healthCheckLoop 健康检查循环
func (p *WANPlanner) healthCheckLoop() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.runHealthChecks()
		}
	}
}

// runHealthChecks 执行所有链路的健康检查
func (p *WANPlanner) runHealthChecks() {
	p.mu.RLock()
	links := make([]*WANLink, 0, len(p.links))
	for _, l := range p.links {
		links = append(links, l)
	}
	probes := make([]ProbeTarget, len(p.probes))
	copy(probes, p.probes)
	p.mu.RUnlock()

	for _, link := range links {
		result := p.probeLink(link, probes)
		p.mu.Lock()
		if l, exists := p.links[link.ID]; exists {
			l.LastCheck = time.Now()
			l.UpdatedAt = time.Now()
			if result {
				if l.Status != LinkStatusUp {
					l.Status = LinkStatusUp
				}
			} else {
				l.Status = LinkStatusDown
			}
		}
		p.mu.Unlock()
	}
}

// probeLink 对单个链路执行探测
func (p *WANPlanner) probeLink(link *WANLink, probes []ProbeTarget) bool {
	if len(probes) == 0 {
		// 无探测目标，默认 ping 网关
		return true
	}
	for _, probe := range probes {
		result := ExecuteProbe(link, probe, p.config.ProbeTimeout)
		if result.Success {
			p.mu.Lock()
			if l, exists := p.links[link.ID]; exists {
				l.Latency = result.Latency
			}
			p.mu.Unlock()
			return true
		}
	}
	return false
}

// AddProbe 添加探测目标
func (p *WANPlanner) AddProbe(target ProbeTarget) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.probes = append(p.probes, target)
}

// SetLinkStatus 手动设置链路状态
func (p *WANPlanner) SetLinkStatus(linkID string, status LinkStatus) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	link, exists := p.links[linkID]
	if !exists {
		return ErrLinkNotFound
	}
	link.Status = status
	link.UpdatedAt = time.Now()
	return nil
}

// IncrementConn 增加链路活跃连接数
func (p *WANPlanner) IncrementConn(linkID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	link, exists := p.links[linkID]
	if !exists {
		return ErrLinkNotFound
	}
	link.ActiveConns++
	return nil
}

// DecrementConn 减少链路活跃连接数
func (p *WANPlanner) DecrementConn(linkID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	link, exists := p.links[linkID]
	if !exists {
		return ErrLinkNotFound
	}
	if link.ActiveConns > 0 {
		link.ActiveConns--
	}
	return nil
}

// GetHistory 获取历史采样数据
func (p *WANPlanner) GetHistory() []BandwidthSample {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]BandwidthSample, len(p.history))
	copy(result, p.history)
	return result
}

// LinkStats 链路统计
type LinkStats struct {
	TotalLinks    int     `json:"total_links"`
	ActiveLinks   int     `json:"active_links"`
	DownLinks     int     `json:"down_links"`
	AvgLatency    time.Duration `json:"avg_latency"`
	AvgPacketLoss float64 `json:"avg_packet_loss"`
	AvgScore      float64 `json:"avg_score"`
}

// GetStats 获取整体统计
func (p *WANPlanner) GetStats() LinkStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	stats := LinkStats{TotalLinks: len(p.links)}
	var totalLat time.Duration
	var totalLoss, totalScore float64
	for _, l := range p.links {
		if l.Status == LinkStatusUp {
			stats.ActiveLinks++
		} else if l.Status == LinkStatusDown {
			stats.DownLinks++
		}
		totalLat += l.Latency
		totalLoss += l.PacketLoss
		totalScore += l.Score
	}
	if stats.TotalLinks > 0 {
		stats.AvgLatency = totalLat / time.Duration(stats.TotalLinks)
		stats.AvgPacketLoss = totalLoss / float64(stats.TotalLinks)
		stats.AvgScore = totalScore / float64(stats.TotalLinks)
	}
	return stats
}

// String 返回可读描述
func (s LinkStats) String() string {
	return fmt.Sprintf("Links: %d total, %d up, %d down | Avg Latency: %s | Avg Loss: %.2f%% | Avg Score: %.1f",
		s.TotalLinks, s.ActiveLinks, s.DownLinks, s.AvgLatency, s.AvgPacketLoss*100, s.AvgScore)
}
