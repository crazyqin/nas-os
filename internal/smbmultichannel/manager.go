// Package smbmultichannel 提供 SMB Multichannel 管理核心业务逻辑
package smbmultichannel

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager SMB Multichannel 管理器.
type Manager struct {
	config       *ChannelConfig
	channels     map[string]*ChannelInfo
	sessions     map[string]*MultichannelSession
	history      []BandwidthHistoryItem
	mu           sync.RWMutex
	maxHistory   int
}

// NewManager 创建 SMB Multichannel 管理器.
func NewManager() *Manager {
	return &Manager{
		config: &ChannelConfig{
			Enabled:        false,
			MaxChannels:    4,
			InterfaceNames: []string{},
			MinSpeed:       1000, // 1 Gbps
		},
		channels:   make(map[string]*ChannelInfo),
		sessions:   make(map[string]*MultichannelSession),
		history:    make([]BandwidthHistoryItem, 0),
		maxHistory: 1440, // 24h at 1-min intervals
	}
}

// ========== 配置管理 ==========

// GetConfig 获取当前配置.
func (m *Manager) GetConfig() *ChannelConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg := *m.config
	if m.config.InterfaceNames != nil {
		cfg.InterfaceNames = make([]string, len(m.config.InterfaceNames))
		copy(cfg.InterfaceNames, m.config.InterfaceNames)
	}
	return &cfg
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(req UpdateConfigRequest) (*ChannelConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Enabled != nil {
		m.config.Enabled = *req.Enabled
	}
	if req.MaxChannels != nil {
		if *req.MaxChannels < 1 || *req.MaxChannels > 32 {
			return nil, fmt.Errorf("max_channels must be between 1 and 32")
		}
		m.config.MaxChannels = *req.MaxChannels
	}
	if req.InterfaceNames != nil {
		m.config.InterfaceNames = make([]string, len(req.InterfaceNames))
		copy(m.config.InterfaceNames, req.InterfaceNames)
	}
	if req.MinSpeed != nil {
		if *req.MinSpeed < 0 {
			return nil, fmt.Errorf("min_speed must be non-negative")
		}
		m.config.MinSpeed = *req.MinSpeed
	}

	cfg := *m.config
	cfg.InterfaceNames = make([]string, len(m.config.InterfaceNames))
	copy(cfg.InterfaceNames, m.config.InterfaceNames)
	return &cfg, nil
}

// ========== 通道管理 ==========

// DetectChannels 检测可用网络通道.
func (m *Manager) DetectChannels() []ChannelStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟检测系统网络接口
	interfaces := []struct {
		name  string
		speed int
	}{
		{"eth0", 10000}, // 10GbE
		{"eth1", 10000}, // 10GbE
		{"eth2", 2500},  // 2.5GbE
		{"bond0", 20000}, // bonded
	}

	for _, iface := range interfaces {
		if _, exists := m.channels[iface.name]; !exists {
			m.channels[iface.name] = &ChannelInfo{
				Status: ChannelStatus{
					InterfaceName: iface.name,
					Speed:         iface.speed,
					Active:        false,
					Connections:   0,
					LastActive:    time.Time{},
				},
				Enabled: false,
			}
		}
	}

	result := make([]ChannelStatus, 0, len(m.channels))
	for _, ch := range m.channels {
		status := ch.Status
		if ch.Enabled && m.config.Enabled {
			status.Active = true
		}
		result = append(result, status)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].InterfaceName < result[j].InterfaceName
	})
	return result
}

// EnableChannel 启用指定通道.
func (m *Manager) EnableChannel(name string) (*ChannelStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, ok := m.channels[name]
	if !ok {
		return nil, fmt.Errorf("channel %q not found", name)
	}

	if m.config.MinSpeed > 0 && ch.Status.Speed < m.config.MinSpeed {
		return nil, fmt.Errorf("channel speed %d Mbps below minimum %d Mbps", ch.Status.Speed, m.config.MinSpeed)
	}

	ch.Enabled = true
	ch.Status.Active = m.config.Enabled
	ch.Status.LastActive = time.Now()

	status := ch.Status
	return &status, nil
}

// DisableChannel 禁用指定通道.
func (m *Manager) DisableChannel(name string) (*ChannelStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, ok := m.channels[name]
	if !ok {
		return nil, fmt.Errorf("channel %q not found", name)
	}

	ch.Enabled = false
	ch.Status.Active = false
	ch.Status.Connections = 0

	status := ch.Status
	return &status, nil
}

// GetChannelStatus 获取指定通道状态.
func (m *Manager) GetChannelStatus(name string) (*ChannelStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ch, ok := m.channels[name]
	if !ok {
		return nil, fmt.Errorf("channel %q not found", name)
	}

	status := ch.Status
	return &status, nil
}

// ========== 会话监控 ==========

// ListSessions 列出所有会话.
func (m *Manager) ListSessions() []*MultichannelSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*MultichannelSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		cp := *s
		if s.Channels != nil {
			cp.Channels = make([]ChannelRef, len(s.Channels))
			copy(cp.Channels, s.Channels)
		}
		sessions = append(sessions, &cp)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})
	return sessions
}

// GetSession 获取指定会话.
func (m *Manager) GetSession(id string) (*MultichannelSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	cp := *s
	if s.Channels != nil {
		cp.Channels = make([]ChannelRef, len(s.Channels))
		copy(cp.Channels, s.Channels)
	}
	return &cp, nil
}

// GetSessionStats 获取会话统计.
func (m *Manager) GetSessionStats(id string) (*SessionStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}

	var totalSpeed int
	for _, ch := range s.Channels {
		if ch.Active {
			totalSpeed += ch.Speed
		}
	}

	activeCount := 0
	for _, ch := range s.Channels {
		if ch.Active {
			activeCount++
		}
	}

	avgSpeed := 0
	if activeCount > 0 {
		avgSpeed = totalSpeed / activeCount
	}

	return &SessionStats{
		SessionID:       s.ID,
		ClientIP:        s.ClientIP,
		TotalBytes:      s.BytesTransferred,
		ChannelCount:    activeCount,
		AvgChannelSpeed: avgSpeed,
		Duration:        int64(time.Since(s.StartTime).Seconds()),
	}, nil
}

// ========== 内部方法 ==========

// createSession 创建新的多通道会话（内部方法）.
func (m *Manager) createSession(clientIP, serverIP string) *MultichannelSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	channels := make([]ChannelRef, 0)
	totalSpeed := 0
	for name, ch := range m.channels {
		if ch.Enabled && m.config.Enabled {
			channels = append(channels, ChannelRef{
				InterfaceName: name,
				Speed:         ch.Status.Speed,
				Active:        true,
			})
			totalSpeed += ch.Status.Speed
			ch.Status.Connections++
			ch.Status.Active = true
			ch.Status.LastActive = time.Now()
		}
	}

	session := &MultichannelSession{
		ID:       uuid.New().String(),
		ClientIP: clientIP,
		ServerIP: serverIP,
		Channels: channels,
		TotalSpeed: totalSpeed,
		StartTime:  time.Now(),
		Protocol:   "SMB3",
	}

	m.sessions[session.ID] = session
	return session
}

// ========== 性能统计 ==========

// GetThroughputStats 获取吞吐量统计.
func (m *Manager) GetThroughputStats() *ThroughputStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalDownload, totalUpload int64
	var peakSpeed int
	activeChannels := 0

	for _, ch := range m.channels {
		if ch.Status.Active {
			activeChannels++
			totalDownload += ch.Status.BytesTransferred
			if ch.Status.Speed > peakSpeed {
				peakSpeed = ch.Status.Speed
			}
		}
	}

	activeSessions := 0
	for _, s := range m.sessions {
		if time.Since(s.StartTime) < 24*time.Hour {
			activeSessions++
			totalUpload += s.BytesTransferred / 2
			totalDownload += s.BytesTransferred / 2
		}
	}

	avgSpeed := 0
	if activeChannels > 0 {
		avgSpeed = peakSpeed // simplified
	}

	return &ThroughputStats{
		TotalDownload:  totalDownload,
		TotalUpload:    totalUpload,
		AvgSpeed:       avgSpeed,
		PeakSpeed:      peakSpeed,
		ActiveSessions: activeSessions,
		ActiveChannels: activeChannels,
		LastUpdated:    time.Now(),
	}
}

// GetBandwidthHistory 获取带宽历史.
func (m *Manager) GetBandwidthHistory(limit int) []BandwidthHistoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	// 返回最近的记录
	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]BandwidthHistoryItem, limit)
	copy(result, m.history[start:])
	return result
}

// RecordBandwidth 记录带宽样本（内部方法）.
func (m *Manager) RecordBandwidth(download, upload int64, speed int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history = append(m.history, BandwidthHistoryItem{
		Timestamp: time.Now(),
		Download:  download,
		Upload:    upload,
		Speed:     speed,
		Sessions:  len(m.sessions),
	})

	// 裁剪历史
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
}

// SimulateTraffic 模拟流量（用于测试和演示）.
func (m *Manager) SimulateTraffic() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return
	}

	for _, ch := range m.channels {
		if ch.Enabled {
			bytesTransferred := int64(rand.Intn(100) + 10) * 1024 * 1024 // 10-110 MB
			ch.Status.BytesTransferred += bytesTransferred
			ch.TotalBytes += bytesTransferred
		}
	}

	// 生成带宽历史样本
	var totalSpeed int
	for _, ch := range m.channels {
		if ch.Enabled {
			totalSpeed += ch.Status.Speed
		}
	}

	m.history = append(m.history, BandwidthHistoryItem{
		Timestamp: time.Now(),
		Download:  int64(rand.Intn(500)+100) * 1024 * 1024,
		Upload:    int64(rand.Intn(200)+50) * 1024 * 1024,
		Speed:     totalSpeed,
		Sessions:  len(m.sessions),
	})

	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
}
