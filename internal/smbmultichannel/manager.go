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
	config     *ChannelConfig
	channels   map[string]*ChannelInfo
	sessions   map[string]*MultichannelSession
	history    []BandwidthHistoryItem
	health     map[string]*ChannelHealth
	auditLog   []AuditEntry
	stats      *ChannelStats
	mu         sync.RWMutex
	maxHistory int
	maxAudit   int
}

// NewManager 创建 SMB Multichannel 管理器.
func NewManager() *Manager {
	m := &Manager{
		config: &ChannelConfig{
			Enabled:         false,
			MaxChannels:     4,
			InterfaceNames:  []string{},
			MinSpeed:        1000, // 1 Gbps
			MinBandwidth:    100,  // 100 Mbps
			LoadBalanceMode: "round-robin",
			JumboFrames:     false,
			RDMAEnabled:     false,
		},
		channels: make(map[string]*ChannelInfo),
		sessions: make(map[string]*MultichannelSession),
		history:  make([]BandwidthHistoryItem, 0),
		health:   make(map[string]*ChannelHealth),
		auditLog: make([]AuditEntry, 0),
		stats: &ChannelStats{
			PerChannelBandwidth: make(map[string]int),
		},
		maxHistory: 1440, // 24h at 1-min intervals
		maxAudit:   10000,
	}

	// 记录初始化审计
	m.addAuditEntry("system", "127.0.0.1", "SMB Multichannel manager initialized")

	return m
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
	if req.MinBandwidth != nil {
		if *req.MinBandwidth < 0 {
			return nil, fmt.Errorf("min_bandwidth must be non-negative")
		}
		m.config.MinBandwidth = *req.MinBandwidth
	}
	if req.LoadBalanceMode != nil {
		validModes := map[string]bool{"round-robin": true, "weighted": true, "hash": true}
		if !validModes[*req.LoadBalanceMode] {
			return nil, fmt.Errorf("invalid load_balance_mode: %s (must be round-robin, weighted, or hash)", *req.LoadBalanceMode)
		}
		m.config.LoadBalanceMode = *req.LoadBalanceMode
	}
	if req.JumboFrames != nil {
		m.config.JumboFrames = *req.JumboFrames
	}
	if req.RDMAEnabled != nil {
		m.config.RDMAEnabled = *req.RDMAEnabled
	}

	cfg := *m.config
	cfg.InterfaceNames = make([]string, len(m.config.InterfaceNames))
	copy(cfg.InterfaceNames, m.config.InterfaceNames)

	m.addAuditEntry("system", "127.0.0.1", "Config updated")

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
		{"eth0", 10000},  // 10GbE
		{"eth1", 10000},  // 10GbE
		{"eth2", 2500},   // 2.5GbE
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
		ID:         uuid.New().String(),
		ClientIP:   clientIP,
		ServerIP:   serverIP,
		Channels:   channels,
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
			bytesTransferred := int64(rand.Intn(100)+10) * 1024 * 1024 // 10-110 MB
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

// ========== 增强功能 ==========

// GetChannelConfig 获取完整通道配置（增强版）.
func (m *Manager) GetChannelConfig() *ChannelConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg := *m.config
	if m.config.InterfaceNames != nil {
		cfg.InterfaceNames = make([]string, len(m.config.InterfaceNames))
		copy(cfg.InterfaceNames, m.config.InterfaceNames)
	}
	return &cfg
}

// UpdateChannelConfig 更新通道配置（增强版）.
func (m *Manager) UpdateChannelConfig(req UpdateConfigRequest) (*ChannelConfig, error) {
	return m.UpdateConfig(req)
}

// GetChannelStats 获取通道统计信息.
func (m *Manager) GetChannelStats() *ChannelStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeChannels := 0
	totalBandwidth := 0
	perChannelBandwidth := make(map[string]int)

	for name, ch := range m.channels {
		if ch.Enabled && ch.Status.Active {
			activeChannels++
			totalBandwidth += ch.Status.Speed
			perChannelBandwidth[name] = ch.Status.Speed
		}
	}

	return &ChannelStats{
		ActiveChannels:      activeChannels,
		TotalBandwidth:      totalBandwidth,
		PerChannelBandwidth: perChannelBandwidth,
		ErrorCount:          m.stats.ErrorCount,
		ReconnectCount:      m.stats.ReconnectCount,
	}
}

// GetChannelHealth 获取所有通道健康状态.
func (m *Manager) GetChannelHealth() []ChannelHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	healthList := make([]ChannelHealth, 0, len(m.health))
	for _, h := range m.health {
		cp := *h
		healthList = append(healthList, cp)
	}

	sort.Slice(healthList, func(i, j int) bool {
		return healthList[i].ChannelID < healthList[j].ChannelID
	})

	return healthList
}

// UpdateChannelHealth 更新通道健康状态.
func (m *Manager) UpdateChannelHealth(channelID, status string, latency int64, packetLoss float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.health[channelID] = &ChannelHealth{
		ChannelID:  channelID,
		Status:     status,
		Latency:    latency,
		PacketLoss: packetLoss,
		LastCheck:  time.Now(),
	}
}

// ListAuditEntries 获取审计日志.
func (m *Manager) ListAuditEntries(limit int) []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.auditLog) {
		limit = len(m.auditLog)
	}

	// 返回最近的记录（从末尾开始）
	start := len(m.auditLog) - limit
	if start < 0 {
		start = 0
	}

	result := make([]AuditEntry, limit)
	copy(result, m.auditLog[start:])

	// 反转顺序，最新的在前
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// addAuditEntry 添加审计日志条目（内部方法）.
func (m *Manager) addAuditEntry(user, clientIP, details string) {
	m.auditLog = append(m.auditLog, AuditEntry{
		Timestamp: time.Now(),
		Action:    "config_change",
		User:      user,
		ClientIP:  clientIP,
		Details:   details,
	})

	// 裁剪审计日志
	if len(m.auditLog) > m.maxAudit {
		m.auditLog = m.auditLog[len(m.auditLog)-m.maxAudit:]
	}
}

// EnableMultichannel 启用 SMB Multichannel.
func (m *Manager) EnableMultichannel(clientIP string) *EnableDisableResponse {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Enabled = true
	m.addAuditEntry("admin", clientIP, "Multichannel enabled")

	// 更新通道状态
	for _, ch := range m.channels {
		if ch.Enabled {
			ch.Status.Active = true
			ch.Status.LastActive = time.Now()
		}
	}

	return &EnableDisableResponse{
		Enabled: true,
		Message: "Multichannel enabled successfully",
	}
}

// DisableMultichannel 禁用 SMB Multichannel.
func (m *Manager) DisableMultichannel(clientIP string) *EnableDisableResponse {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Enabled = false
	m.addAuditEntry("admin", clientIP, "Multichannel disabled")

	// 更新通道状态
	for _, ch := range m.channels {
		ch.Status.Active = false
		ch.Status.Connections = 0
	}

	// 终止所有会话
	for id := range m.sessions {
		delete(m.sessions, id)
	}

	return &EnableDisableResponse{
		Enabled: false,
		Message: "Multichannel disabled successfully",
	}
}

// SetLoadBalanceMode 设置负载均衡模式.
func (m *Manager) SetLoadBalanceMode(mode string, clientIP string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	validModes := map[string]bool{"round-robin": true, "weighted": true, "hash": true}
	if !validModes[mode] {
		return fmt.Errorf("invalid load balance mode: %s (must be round-robin, weighted, or hash)", mode)
	}

	m.config.LoadBalanceMode = mode
	m.addAuditEntry("admin", clientIP, fmt.Sprintf("Load balance mode changed to %s", mode))

	return nil
}

// IncrementErrorCount 增加错误计数.
func (m *Manager) IncrementErrorCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats.ErrorCount++
}

// IncrementReconnectCount 增加重连计数.
func (m *Manager) IncrementReconnectCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats.ReconnectCount++
}
