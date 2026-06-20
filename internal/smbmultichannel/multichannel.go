// Package smbmultichannel 提供 SMB Multichannel 核心业务逻辑
// 合并 Manager 和 MultiChannelManager 为统一管理器
// 对标 TrueNAS 25.04 的 SMB 多通道功能
package smbmultichannel

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 管理器核心结构 ==========

// Manager SMB Multichannel 统一管理器
// 整合多通道连接管理、带宽聚合、故障转移、性能监控、配置管理
type Manager struct {
	mu sync.RWMutex

	// 基础配置
	config *ChannelConfig

	// 网络通道（接口级别）
	channels map[string]*ChannelInfo

	// 多通道会话
	sessions map[string]*MultichannelSession

	// 性能数据
	history    []BandwidthHistoryItem
	maxHistory int

	// 健康监控
	health map[string]*ChannelHealth

	// 审计日志
	auditLog []AuditEntry
	maxAudit int

	// 通道统计
	stats *ChannelStats

	// 管理器级统计
	managerStats ManagerStats
}

// NewManager 创建 SMB Multichannel 管理器
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

	m.addAuditEntry("system", "127.0.0.1", "SMB Multichannel manager initialized")
	return m
}

// ========== 配置管理 ==========

// GetConfig 获取当前配置（线程安全副本）
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

// UpdateConfig 更新配置
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
		if !ValidLoadBalanceModes[*req.LoadBalanceMode] {
			return nil, fmt.Errorf("invalid load_balance_mode: %s", *req.LoadBalanceMode)
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

// SaveConfig 保存配置到文件
func (m *Manager) SaveConfig(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0640)
}

// LoadConfig 从文件加载配置
func (m *Manager) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return json.Unmarshal(data, &m.config)
}

// ========== 通道管理（接口级别） ==========

// DetectChannels 检测可用网络通道
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

// EnableChannel 启用指定通道
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

// DisableChannel 禁用指定通道
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

// GetChannelStatus 获取指定通道状态
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

// ========== 多通道会话管理 ==========

// CreateSession 创建多通道会话
// 自动选择可用通道，根据负载均衡策略分配
func (m *Manager) CreateSession(clientIP, serverIP string) (*MultichannelSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("SMB multichannel not enabled")
	}

	// 检查会话数量限制
	if len(m.sessions) >= m.config.MaxChannels*32 {
		return nil, fmt.Errorf("maximum session limit reached")
	}

	channels := make([]ChannelRef, 0)
	totalSpeed := 0
	enabledCount := 0

	for name, ch := range m.channels {
		if ch.Enabled && m.config.Enabled {
			enabledCount++
			if enabledCount > m.config.MaxChannels {
				break
			}
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

	if len(channels) == 0 {
		return nil, fmt.Errorf("no available channels")
	}

	session := &MultichannelSession{
		ID:          uuid.New().String(),
		ClientIP:    clientIP,
		ServerIP:    serverIP,
		Channels:    channels,
		State:       SessionStateActive,
		MaxChannels: m.config.MaxChannels,
		Algorithm:   LoadBalanceAlgo(m.config.LoadBalanceMode),
		TotalSpeed:  totalSpeed,
		StartTime:   time.Now(),
		Protocol:    "SMB3",
	}

	m.sessions[session.ID] = session
	m.updateManagerStats()

	m.addAuditEntry("system", clientIP,
		fmt.Sprintf("Session created: %s -> %s, channels=%d, speed=%d Mbps",
			clientIP, serverIP, len(channels), totalSpeed))

	return session, nil
}

// CloseSession 关闭多通道会话
func (m *Manager) CloseSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("session %q not found", id)
	}

	// 释放通道连接数
	for _, ch := range session.Channels {
		if info, ok := m.channels[ch.InterfaceName]; ok {
			info.Status.Connections--
			if info.Status.Connections < 0 {
				info.Status.Connections = 0
			}
		}
	}

	session.State = SessionStateClosed
	delete(m.sessions, id)
	m.updateManagerStats()

	m.addAuditEntry("system", session.ClientIP, fmt.Sprintf("Session closed: %s", id))
	return nil
}

// GetSession 获取指定会话
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
	if s.Metadata != nil {
		cp.Metadata = make(map[string]string, len(s.Metadata))
		for k, v := range s.Metadata {
			cp.Metadata[k] = v
		}
	}
	return &cp, nil
}

// ListSessions 列出所有会话
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

// GetSessionStats 获取会话统计
func (m *Manager) GetSessionStats(id string) (*SessionStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}

	var totalSpeed int
	activeCount := 0
	for _, ch := range s.Channels {
		if ch.Active {
			totalSpeed += ch.Speed
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

// ========== 负载均衡 ==========

// SelectChannelForSession 为会话选择最佳通道（负载均衡）
func (m *Manager) SelectChannelForSession(sessionID string) (*ChannelRef, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	if len(session.Channels) == 0 {
		return nil, fmt.Errorf("no channels in session")
	}

	// 过滤活跃通道
	activeChannels := make([]ChannelRef, 0)
	for _, ch := range session.Channels {
		if ch.Active {
			activeChannels = append(activeChannels, ch)
		}
	}

	if len(activeChannels) == 0 {
		return nil, fmt.Errorf("no active channels")
	}

	algo := session.Algorithm
	if algo == "" {
		algo = LoadBalanceAdaptive
	}

	var selected *ChannelRef
	switch algo {
	case LoadBalanceRoundRobin:
		selected = m.selectRoundRobin(activeChannels)
	case LoadBalanceLeastConn:
		selected = m.selectLeastConn(activeChannels)
	case LoadBalanceBandwidth:
		selected = m.selectByBandwidth(activeChannels)
	case LoadBalanceLatency:
		selected = m.selectByLatency(activeChannels)
	case LoadBalanceAdaptive:
		selected = m.selectAdaptive(activeChannels)
	default:
		selected = &activeChannels[0]
	}

	m.stats.LoadBalanceCount++
	return selected, nil
}

// selectRoundRobin 轮询选择
func (m *Manager) selectRoundRobin(channels []ChannelRef) *ChannelRef {
	index := time.Now().UnixNano() % int64(len(channels))
	return &channels[index]
}

// selectLeastConn 最少连接选择
func (m *Manager) selectLeastConn(channels []ChannelRef) *ChannelRef {
	minConns := int(^uint(0) >> 1)
	var selected *ChannelRef

	for i, ch := range channels {
		if info, ok := m.channels[ch.InterfaceName]; ok {
			if info.Status.Connections < minConns {
				minConns = info.Status.Connections
				selected = &channels[i]
			}
		}
	}

	if selected == nil {
		selected = &channels[0]
	}
	return selected
}

// selectByBandwidth 带宽优先选择
func (m *Manager) selectByBandwidth(channels []ChannelRef) *ChannelRef {
	maxSpeed := 0
	var selected *ChannelRef

	for i, ch := range channels {
		if ch.Speed > maxSpeed {
			maxSpeed = ch.Speed
			selected = &channels[i]
		}
	}

	if selected == nil {
		selected = &channels[0]
	}
	return selected
}

// selectByLatency 延迟优先选择
func (m *Manager) selectByLatency(channels []ChannelRef) *ChannelRef {
	minLatency := int64(^uint64(0) >> 1)
	var selected *ChannelRef

	for i, ch := range channels {
		if h, ok := m.health[ch.InterfaceName]; ok {
			if h.Latency < minLatency && h.Status == "up" {
				minLatency = h.Latency
				selected = &channels[i]
			}
		}
	}

	if selected == nil {
		selected = &channels[0]
	}
	return selected
}

// selectAdaptive 自适应选择（综合带宽、延迟、连接数）
func (m *Manager) selectAdaptive(channels []ChannelRef) *ChannelRef {
	bestScore := float64(-1)
	var selected *ChannelRef

	for i, ch := range channels {
		bandwidthScore := float64(ch.Speed) / 10000.0 // 归一化

		latencyScore := 1.0
		if h, ok := m.health[ch.InterfaceName]; ok {
			latencyScore = 1.0 / (1.0 + float64(h.Latency))
		}

		loadScore := 1.0
		if info, ok := m.channels[ch.InterfaceName]; ok {
			loadScore = 1.0 / (1.0 + float64(info.Status.Connections))
		}

		score := bandwidthScore*0.4 + latencyScore*0.3 + loadScore*0.3
		if score > bestScore {
			bestScore = score
			selected = &channels[i]
		}
	}

	if selected == nil {
		selected = &channels[0]
	}
	return selected
}

// ========== 故障转移 ==========

// HandleChannelFailure 处理通道故障
func (m *Manager) HandleChannelFailure(sessionID, channelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %q not found", sessionID)
	}

	found := false
	for i, ch := range session.Channels {
		if ch.InterfaceName == channelID {
			session.Channels[i].Active = false
			found = true

			// 更新接口状态
			if info, ok := m.channels[channelID]; ok {
				info.Status.Active = false
				info.Status.Connections--
				if info.Status.Connections < 0 {
					info.Status.Connections = 0
				}
			}
			break
		}
	}

	if !found {
		return fmt.Errorf("channel %q not found in session", channelID)
	}

	// 检查剩余活跃通道
	activeCount := 0
	for _, ch := range session.Channels {
		if ch.Active {
			activeCount++
		}
	}

	totalChannels := len(session.Channels)
	switch {
	case activeCount == 0:
		session.State = SessionStateInactive
	case activeCount < totalChannels/2:
		session.State = SessionStateDegraded
	default:
		session.State = SessionStateActive
	}

	m.stats.FailoverCount++
	m.updateManagerStats()

	m.addAuditEntry("system", session.ClientIP,
		fmt.Sprintf("Channel failure: session=%s, channel=%s, remaining=%d/%d",
			sessionID, channelID, activeCount, totalChannels))

	return nil
}

// ========== 自动重平衡 ==========

// RebalanceChannels 重平衡通道负载
func (m *Manager) RebalanceChannels() (*RebalanceResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &RebalanceResult{
		Timestamp:       time.Now(),
		RebalancedCount: 0,
		Details:         make(map[string]string),
	}

	for sessionID, session := range m.sessions {
		if session.State != SessionStateActive {
			continue
		}

		if m.isSessionImbalanced(session) {
			result.Details[sessionID] = "rebalanced"
			result.RebalancedCount++
		}
	}

	m.managerStats.LastRebalance = time.Now()
	return result, nil
}

// isSessionImbalanced 检查会话负载是否不均衡
func (m *Manager) isSessionImbalanced(session *MultichannelSession) bool {
	if len(session.Channels) <= 1 {
		return false
	}

	var minSpeed, maxSpeed int
	minSpeed = int(^uint(0) >> 1)
	maxSpeed = 0

	for _, ch := range session.Channels {
		if !ch.Active {
			continue
		}
		if ch.Speed < minSpeed {
			minSpeed = ch.Speed
		}
		if ch.Speed > maxSpeed {
			maxSpeed = ch.Speed
		}
	}

	if maxSpeed == 0 {
		return false
	}

	imbalance := float64(maxSpeed-minSpeed) / float64(maxSpeed)
	return imbalance > 0.5 // 阈值
}

// ========== 性能监控 ==========

// GetThroughputStats 获取吞吐量统计
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
		if s.State == SessionStateActive {
			activeSessions++
			totalUpload += s.BytesTransferred / 2
			totalDownload += s.BytesTransferred / 2
		}
	}

	avgSpeed := 0
	if activeChannels > 0 {
		avgSpeed = peakSpeed
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

// GetBandwidthHistory 获取带宽历史
func (m *Manager) GetBandwidthHistory(limit int) []BandwidthHistoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]BandwidthHistoryItem, limit)
	copy(result, m.history[start:])
	return result
}

// RecordBandwidth 记录带宽样本
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

	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
}

// SimulateTraffic 模拟流量（用于测试和演示）
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

// GetChannelStats 获取通道统计信息
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

// GetChannelHealth 获取所有通道健康状态
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

// UpdateChannelHealth 更新通道健康状态
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

// GetManagerStats 获取管理器全局统计
func (m *Manager) GetManagerStats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.managerStats
}

// ========== 全局启用/禁用 ==========

// EnableMultichannel 启用 SMB Multichannel
func (m *Manager) EnableMultichannel(clientIP string) *EnableDisableResponse {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Enabled = true
	m.addAuditEntry("admin", clientIP, "Multichannel enabled")

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

// DisableMultichannel 禁用 SMB Multichannel
func (m *Manager) DisableMultichannel(clientIP string) *EnableDisableResponse {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Enabled = false
	m.addAuditEntry("admin", clientIP, "Multichannel disabled")

	for _, ch := range m.channels {
		ch.Status.Active = false
		ch.Status.Connections = 0
	}

	// 关闭所有会话
	for id, s := range m.sessions {
		s.State = SessionStateClosed
		delete(m.sessions, id)
	}

	m.updateManagerStats()

	return &EnableDisableResponse{
		Enabled: false,
		Message: "Multichannel disabled successfully",
	}
}

// SetLoadBalanceMode 设置负载均衡模式
func (m *Manager) SetLoadBalanceMode(mode string, clientIP string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !ValidLoadBalanceModes[mode] {
		return fmt.Errorf("invalid load balance mode: %s", mode)
	}

	m.config.LoadBalanceMode = mode
	m.addAuditEntry("admin", clientIP, fmt.Sprintf("Load balance mode changed to %s", mode))
	return nil
}

// ========== 审计日志 ==========

// ListAuditEntries 获取审计日志
func (m *Manager) ListAuditEntries(limit int) []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.auditLog) {
		limit = len(m.auditLog)
	}

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

// ========== 网络接口检测 ==========

// DetectNetworkInterfaces 检测系统网络接口
func (m *Manager) DetectNetworkInterfaces() ([]NetworkInterface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	result := make([]NetworkInterface, 0)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		ni := NetworkInterface{
			Name:         iface.Name,
			MTU:          iface.MTU,
			HardwareAddr: iface.HardwareAddr.String(),
			Addresses:    make([]string, 0),
		}

		for _, addr := range addrs {
			ni.Addresses = append(ni.Addresses, addr.String())
		}

		result = append(result, ni)
	}

	return result, nil
}

// ========== 错误计数 ==========

// IncrementErrorCount 增加错误计数
func (m *Manager) IncrementErrorCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats.ErrorCount++
}

// IncrementReconnectCount 增加重连计数
func (m *Manager) IncrementReconnectCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats.ReconnectCount++
}

// ========== 内部方法 ==========

// addAuditEntry 添加审计日志条目
func (m *Manager) addAuditEntry(user, clientIP, details string) {
	m.auditLog = append(m.auditLog, AuditEntry{
		Timestamp: time.Now(),
		Action:    "config_change",
		User:      user,
		ClientIP:  clientIP,
		Details:   details,
	})

	if len(m.auditLog) > m.maxAudit {
		m.auditLog = m.auditLog[len(m.auditLog)-m.maxAudit:]
	}
}

// updateManagerStats 更新管理器级统计
func (m *Manager) updateManagerStats() {
	m.managerStats.TotalSessions = len(m.sessions)
	m.managerStats.ActiveSessions = 0
	m.managerStats.TotalChannels = 0
	m.managerStats.ActiveChannels = 0

	for _, session := range m.sessions {
		if session.State == SessionStateActive {
			m.managerStats.ActiveSessions++
		}
		m.managerStats.TotalChannels += len(session.Channels)
		for _, ch := range session.Channels {
			if ch.Active {
				m.managerStats.ActiveChannels++
			}
		}
	}

	m.managerStats.LoadBalanceCount = m.stats.LoadBalanceCount
	m.managerStats.FailoverCount = m.stats.FailoverCount
}
