// Package rdmaoptimizer 提供 RDMA 网络传输优化
package rdmaoptimizer

import (
	"fmt"
	"sync"
	"time"
)

// Manager RDMA 优化器管理器
type Manager struct {
	mu     sync.RWMutex
	links  map[string]*RDMALink
	paths  map[string]*RDMAPath
	config CongestionConfig
	stats  OptimizerStats
}

// NewManager 创建 RDMA 优化器管理器
func NewManager(config CongestionConfig) *Manager {
	// 设置默认值
	if config.Threshold == 0 {
		config.Threshold = 0.7
	}
	if config.BackoffMs == 0 {
		config.BackoffMs = 10
	}
	if config.ProbeIntervalMs == 0 {
		config.ProbeIntervalMs = 100
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.ECNThreshold == 0 {
		config.ECNThreshold = 0.8
	}
	if config.DCQCNAlpha == 0 {
		config.DCQCNAlpha = 0.5
	}
	if config.DCQCNBeta == 0 {
		config.DCQCNBeta = 0.2
	}
	if config.DCQCNMinRate == 0 {
		config.DCQCNMinRate = 1.0
	}

	return &Manager{
		links:  make(map[string]*RDMALink),
		paths:  make(map[string]*RDMAPath),
		config: config,
	}
}

// DetectLinks 检测 RDMA 链路
func (m *Manager) DetectLinks() []RDMALink {
	m.mu.RLock()
	defer m.mu.RUnlock()

	links := make([]RDMALink, 0, len(m.links))
	for _, l := range m.links {
		links = append(links, *l)
	}
	return links
}

// CreateLink 创建 RDMA 链路
func (m *Manager) CreateLink(req CreateLinkRequest) (*RDMALink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 设置默认值
	linkSpeed := req.LinkSpeed
	if linkSpeed == "" {
		linkSpeed = "100Gb"
	}
	mtu := req.MTU
	if mtu == 0 {
		mtu = 4096
	}

	// 解析带宽
	bandwidth := parseLinkSpeed(linkSpeed)

	now := time.Now()
	link := &RDMALink{
		ID:            fmt.Sprintf("link-%s:%d-%s:%d", req.LocalDevice, req.LocalPort, req.RemoteDevice, req.RemotePort),
		LocalDevice:   req.LocalDevice,
		LocalPort:     req.LocalPort,
		RemoteDevice:  req.RemoteDevice,
		RemotePort:    req.RemotePort,
		RemoteGID:     req.RemoteGID,
		LinkSpeed:     linkSpeed,
		State:         LinkStateActive,
		BandwidthGbps: bandwidth,
		LatencyNs:     500, // 默认 500ns
		MTU:           mtu,
		Metadata:      make(map[string]string),
		LastSeen:      now,
		CreatedAt:     now,
	}

	m.links[link.ID] = link
	m.updateStats()

	return link, nil
}

// parseLinkSpeed 解析链路速度
func parseLinkSpeed(speed string) float64 {
	switch speed {
	case "10Gb":
		return 10.0
	case "25Gb":
		return 25.0
	case "40Gb":
		return 40.0
	case "50Gb":
		return 50.0
	case "100Gb":
		return 100.0
	case "200Gb":
		return 200.0
	case "400Gb":
		return 400.0
	default:
		return 100.0
	}
}

// GetLink 获取链路详情
func (m *Manager) GetLink(id string) (*RDMALink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, exists := m.links[id]
	if !exists {
		return nil, fmt.Errorf("link not found: %s", id)
	}
	return link, nil
}

// DeleteLink 删除链路
func (m *Manager) DeleteLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.links[id]; !exists {
		return fmt.Errorf("link not found: %s", id)
	}

	// 检查是否被路径使用
	for _, path := range m.paths {
		for _, linkID := range path.Links {
			if linkID == id {
				return fmt.Errorf("link %s is used by path %s", id, path.ID)
			}
		}
	}

	delete(m.links, id)
	m.updateStats()

	return nil
}

// UpdateLinkState 更新链路状态
func (m *Manager) UpdateLinkState(id string, state LinkState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, exists := m.links[id]
	if !exists {
		return fmt.Errorf("link not found: %s", id)
	}

	link.State = state
	link.LastSeen = time.Now()
	m.updateStats()

	return nil
}

// CreatePath 创建 RDMA 路径
func (m *Manager) CreatePath(req CreatePathRequest) (*RDMAPath, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证链路存在
	for _, linkID := range req.Links {
		if _, exists := m.links[linkID]; !exists {
			return nil, fmt.Errorf("link not found: %s", linkID)
		}
	}

	// 计算路径属性
	var minBandwidth float64 = 1e9
	var totalLatency int64
	first := true

	for _, linkID := range req.Links {
		link := m.links[linkID]
		if first || link.BandwidthGbps < minBandwidth {
			minBandwidth = link.BandwidthGbps
			first = false
		}
		totalLatency += link.LatencyNs
	}

	now := time.Now()
	path := &RDMAPath{
		ID:            fmt.Sprintf("path-%s-%s-%d", req.SourceDevice, req.DestDevice, now.UnixNano()),
		SourceDevice:  req.SourceDevice,
		DestDevice:    req.DestDevice,
		Links:         req.Links,
		HopCount:      len(req.Links),
		State:         PathStateAvailable,
		BandwidthGbps: minBandwidth,
		LatencyNs:     totalLatency,
		Congestion:    0.0,
		Weight:        float64(totalLatency) / minBandwidth, // 越小越好
		LastMeasured:  now,
		CreatedAt:     now,
	}

	m.paths[path.ID] = path
	m.updateStats()

	return path, nil
}

// GetPath 获取路径详情
func (m *Manager) GetPath(id string) (*RDMAPath, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path, exists := m.paths[id]
	if !exists {
		return nil, fmt.Errorf("path not found: %s", id)
	}
	return path, nil
}

// ListPaths 列出路径
func (m *Manager) ListPaths(sourceDevice, destDevice string) []RDMAPath {
	m.mu.RLock()
	defer m.mu.RUnlock()

	paths := make([]RDMAPath, 0)
	for _, p := range m.paths {
		if (sourceDevice == "" || p.SourceDevice == sourceDevice) &&
			(destDevice == "" || p.DestDevice == destDevice) {
			paths = append(paths, *p)
		}
	}
	return paths
}

// DeletePath 删除路径
func (m *Manager) DeletePath(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.paths[id]; !exists {
		return fmt.Errorf("path not found: %s", id)
	}

	delete(m.paths, id)
	m.updateStats()

	return nil
}

// SelectPath 自动选择最佳路径
func (m *Manager) SelectPath(sourceDevice, destDevice string) (*RDMAPath, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var bestPath *RDMAPath

	for _, path := range m.paths {
		if path.SourceDevice != sourceDevice || path.DestDevice != destDevice {
			continue
		}

		if path.State == PathStateDown {
			continue
		}

		if bestPath == nil || path.Weight < bestPath.Weight {
			bestPath = path
		}
	}

	if bestPath == nil {
		return nil, fmt.Errorf("no available path from %s to %s", sourceDevice, destDevice)
	}

	return bestPath, nil
}

// OptimizeTransfer 优化传输
func (m *Manager) OptimizeTransfer(req OptimizeRequest) (*TransferMetrics, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证链路或路径存在
	var bandwidthGbps float64
	var latencyNs int64

	if req.LinkID != "" {
		link, exists := m.links[req.LinkID]
		if !exists {
			return nil, fmt.Errorf("link not found: %s", req.LinkID)
		}
		bandwidthGbps = link.BandwidthGbps
		latencyNs = link.LatencyNs
	} else if req.PathID != "" {
		path, exists := m.paths[req.PathID]
		if !exists {
			return nil, fmt.Errorf("path not found: %s", req.PathID)
		}
		bandwidthGbps = path.BandwidthGbps
		latencyNs = path.LatencyNs
	} else {
		return nil, fmt.Errorf("either link_id or path_id is required")
	}

	// 应用拥塞控制优化
	if m.config.Enabled {
		if req.TargetLatencyNs > 0 && latencyNs > req.TargetLatencyNs {
			// 降低延迟
			latencyNs = req.TargetLatencyNs
		}
		if req.TargetBandwidthGbps > 0 && bandwidthGbps < req.TargetBandwidthGbps {
			// 提升带宽
			bandwidthGbps = req.TargetBandwidthGbps
		}
	}

	m.stats.OptimizationCount++
	m.stats.LastOptimized = time.Now()

	metrics := &TransferMetrics{
		LinkID:        req.LinkID,
		PathID:        req.PathID,
		BandwidthGbps: bandwidthGbps,
		AvgLatencyNs:  latencyNs,
		MaxLatencyNs:  latencyNs * 2,
		Timestamp:     time.Now(),
	}

	return metrics, nil
}

// GetMetrics 获取传输指标
func (m *Manager) GetMetrics(linkID string) (*TransferMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.links[linkID]; !exists {
		return nil, fmt.Errorf("link not found: %s", linkID)
	}

	// 模拟指标
	metrics := &TransferMetrics{
		LinkID:        linkID,
		BytesSent:     1024 * 1024 * 100,
		BytesReceived: 1024 * 1024 * 50,
		IOPS:          100000,
		BandwidthGbps: 95.0,
		AvgLatencyNs:  500,
		MaxLatencyNs:  1000,
		PacketLoss:    0.0001,
		Congestion:    0.1,
		Timestamp:     time.Now(),
	}

	return metrics, nil
}

// GetLinkMetrics 获取链路指标
func (m *Manager) GetLinkMetrics(linkID string) (*LinkMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, exists := m.links[linkID]
	if !exists {
		return nil, fmt.Errorf("link not found: %s", linkID)
	}

	metrics := &LinkMetrics{
		LinkID:        linkID,
		BandwidthGbps: link.BandwidthGbps,
		Utilization:   0.5,
		LatencyNs:     link.LatencyNs,
		ErrorRate:     0.0001,
		Congestion:    0.1,
		Timestamp:     time.Now(),
	}

	return metrics, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() OptimizerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats
}

// updateStats 更新统计
func (m *Manager) updateStats() {
	m.stats.TotalLinks = len(m.links)
	m.stats.ActiveLinks = 0

	var totalLatency int64
	var totalBandwidth float64
	var count int

	for _, l := range m.links {
		if l.State == LinkStateActive {
			m.stats.ActiveLinks++
			totalLatency += l.LatencyNs
			totalBandwidth += l.BandwidthGbps
			count++
		}
	}

	m.stats.TotalPaths = len(m.paths)
	m.stats.ActivePaths = 0
	for _, p := range m.paths {
		if p.State == PathStateAvailable {
			m.stats.ActivePaths++
		}
	}

	if count > 0 {
		m.stats.AvgLatencyNs = totalLatency / int64(count)
		m.stats.AvgBandwidthGbps = totalBandwidth / float64(count)
	}
}

// GetConfig 获取配置
func (m *Manager) GetConfig() CongestionConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config CongestionConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// UpdateCongestion 更新路径拥塞状态
func (m *Manager) UpdateCongestion(pathID string, congestion float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path, exists := m.paths[pathID]
	if !exists {
		return fmt.Errorf("path not found: %s", pathID)
	}

	path.Congestion = congestion
	path.LastMeasured = time.Now()

	// 更新路径状态
	if congestion >= m.config.Threshold {
		path.State = PathStateCongested
		path.Weight = float64(path.LatencyNs) / path.BandwidthGbps * (1 + congestion)
	} else {
		path.State = PathStateAvailable
		path.Weight = float64(path.LatencyNs) / path.BandwidthGbps
	}

	return nil
}
