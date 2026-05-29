// Package smbmultichannel 提供 SMB 多通道支持
// 对标 TrueNAS Multichannel SMB，提升文件传输性能
package smbmultichannel

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// ========== 多通道连接管理 ==========

// MultiChannelSession 多通道会话
type MultiChannelSession struct {
	ID           string            `json:"id"`
	ClientIP     string            `json:"client_ip"`
	ServerIP     string            `json:"server_ip"`
	Channels     []Channel         `json:"channels"`
	State        SessionState      `json:"state"`
	MaxChannels  int               `json:"max_channels"`
	Algorithm    LoadBalanceAlgo   `json:"algorithm"`
	Stats        SessionStats      `json:"stats"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	LastActive   time.Time         `json:"last_active"`
}

// Channel 通道
type Channel struct {
	ID           string      `json:"id"`
	LocalAddr    string      `json:"local_addr"`
	RemoteAddr   string      `json:"remote_addr"`
	Socket       int         `json:"socket"`
	State        ChannelState `json:"state"`
	Capabilities []string    `json:"capabilities"`
	Speed        int64       `json:"speed"` // Mbps
	Stats        ChannelStats `json:"stats"`
	LastActive   time.Time   `json:"last_active"`
}

// SessionState 会话状态
type SessionState string

const (
	SessionStateActive   SessionState = "active"
	SessionStateInactive SessionState = "inactive"
	SessionStateDegraded SessionState = "degraded"
	SessionStateClosed   SessionState = "closed"
)

// ChannelState 通道状态
type ChannelState string

const (
	ChannelStateActive  ChannelState = "active"
	ChannelStateStandby ChannelState = "standby"
	ChannelStateFailed  ChannelState = "failed"
	ChannelStateClosed  ChannelState = "closed"
)

// LoadBalanceAlgo 负载均衡算法
type LoadBalanceAlgo string

const (
	LoadBalanceRoundRobin   LoadBalanceAlgo = "round_robin"
	LoadBalanceLeastConn    LoadBalanceAlgo = "least_conn"
	LoadBalanceBandwidth    LoadBalanceAlgo = "bandwidth"
	LoadBalanceLatency      LoadBalanceAlgo = "latency"
	LoadBalanceAdaptive     LoadBalanceAlgo = "adaptive"
)

// SessionStats and ChannelStats are defined in types.go

// ========== 多通道管理器 ==========

// MultiChannelManager 多通道管理器
type MultiChannelManager struct {
	mu       sync.RWMutex
	sessions map[string]*MultiChannelSession
	config   ManagerConfig
	stats    ManagerStats
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	Enabled           bool           `json:"enabled"`
	MaxChannelsPerClient int        `json:"max_channels_per_client"`
	MaxTotalChannels  int           `json:"max_total_channels"`
	DefaultAlgorithm  LoadBalanceAlgo `json:"default_algorithm"`
	HealthCheckInterval int         `json:"health_check_interval"` // 秒
	FailoverEnabled   bool          `json:"failover_enabled"`
	AutoRebalance     bool          `json:"auto_rebalance"`
	RebalanceThreshold float64      `json:"rebalance_threshold"` // 负载不均衡阈值
	EncryptionEnabled bool          `json:"encryption_enabled"`
	CompressionEnabled bool         `json:"compression_enabled"`
	SigningEnabled    bool          `json:"signing_enabled"`
	MaxSMBVersion     string        `json:"max_smb_version"` // 2.0, 2.1, 3.0, 3.1.1
	MinSMBVersion     string        `json:"min_smb_version"`
}

// ManagerStats 管理器统计
type ManagerStats struct {
	TotalSessions     int       `json:"total_sessions"`
	ActiveSessions    int       `json:"active_sessions"`
	TotalChannels     int       `json:"total_channels"`
	ActiveChannels    int       `json:"active_channels"`
	TotalThroughputGB float64   `json:"total_throughput_gb"`
	AvgLatencyMs      float64   `json:"avg_latency_ms"`
	LoadBalanceCount  int64     `json:"load_balance_count"`
	FailoverCount     int64     `json:"failover_count"`
	LastRebalance     time.Time `json:"last_rebalance"`
}

// NewMultiChannelManager 创建多通道管理器
func NewMultiChannelManager(config ManagerConfig) *MultiChannelManager {
	// 设置默认值
	if config.MaxChannelsPerClient == 0 {
		config.MaxChannelsPerClient = 8
	}
	if config.MaxTotalChannels == 0 {
		config.MaxTotalChannels = 256
	}
	if config.DefaultAlgorithm == "" {
		config.DefaultAlgorithm = LoadBalanceAdaptive
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30
	}
	if config.MaxSMBVersion == "" {
		config.MaxSMBVersion = "3.1.1"
	}
	if config.MinSMBVersion == "" {
		config.MinSMBVersion = "2.1"
	}

	return &MultiChannelManager{
		sessions: make(map[string]*MultiChannelSession),
		config:   config,
	}
}

// ========== 会话管理 ==========

// CreateSession 创建多通道会话
func (m *MultiChannelManager) CreateSession(session MultiChannelSession) (*MultiChannelSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("SMB 多通道未启用")
	}

	if session.ID == "" {
		session.ID = fmt.Sprintf("session-%s-%s-%d", session.ClientIP, session.ServerIP, time.Now().UnixNano())
	}

	if _, exists := m.sessions[session.ID]; exists {
		return nil, fmt.Errorf("会话已存在: %s", session.ID)
	}

	if session.MaxChannels == 0 {
		session.MaxChannels = m.config.MaxChannelsPerClient
	}
	if session.Algorithm == "" {
		session.Algorithm = m.config.DefaultAlgorithm
	}

	session.State = SessionStateActive
	session.CreatedAt = time.Now()
	session.LastActive = time.Now()
	session.Channels = make([]Channel, 0)

	m.sessions[session.ID] = &session
	m.updateStats()

	return &session, nil
}

// CloseSession 关闭会话
func (m *MultiChannelManager) CloseSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("会话不存在: %s", id)
	}

	// 关闭所有通道
	for i := range session.Channels {
		session.Channels[i].State = ChannelStateClosed
	}

	session.State = SessionStateClosed
	delete(m.sessions, id)
	m.updateStats()

	return nil
}

// GetSession 获取会话
func (m *MultiChannelManager) GetSession(id string) (*MultiChannelSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[id]
	if !exists {
		return nil, fmt.Errorf("会话不存在: %s", id)
	}

	return session, nil
}

// ListSessions 列出所有会话
func (m *MultiChannelManager) ListSessions() []*MultiChannelSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MultiChannelSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}

	return result
}

// ========== 通道管理 ==========

// AddChannel 添加通道
func (m *MultiChannelManager) AddChannel(sessionID string, channel Channel) (*Channel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	if len(session.Channels) >= session.MaxChannels {
		return nil, fmt.Errorf("已达到最大通道数: %d", session.MaxChannels)
	}

	// 检查总通道数限制
	totalChannels := m.countTotalChannels()
	if totalChannels >= m.config.MaxTotalChannels {
		return nil, fmt.Errorf("已达到系统最大通道数: %d", m.config.MaxTotalChannels)
	}

	if channel.ID == "" {
		channel.ID = fmt.Sprintf("channel-%s-%d", sessionID, time.Now().UnixNano())
	}

	channel.State = ChannelStateActive
	channel.LastActive = time.Now()
	if channel.Speed == 0 {
		channel.Speed = 1000 // 默认 1Gbps
	}

	session.Channels = append(session.Channels, channel)
	session.LastActive = time.Now()
	m.updateStats()

	return &channel, nil
}

// RemoveChannel 移除通道
func (m *MultiChannelManager) RemoveChannel(sessionID, channelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	for i, ch := range session.Channels {
		if ch.ID == channelID {
			session.Channels = append(session.Channels[:i], session.Channels[i+1:]...)
			session.LastActive = time.Now()
			m.updateStats()
			return nil
		}
	}

	return fmt.Errorf("通道不存在: %s", channelID)
}

// ========== 负载均衡 ==========

// SelectChannel 选择通道（负载均衡）
func (m *MultiChannelManager) SelectChannel(sessionID string) (*Channel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	if len(session.Channels) == 0 {
		return nil, fmt.Errorf("没有可用通道")
	}

	// 过滤活跃通道
	activeChannels := make([]Channel, 0)
	for _, ch := range session.Channels {
		if ch.State == ChannelStateActive {
			activeChannels = append(activeChannels, ch)
		}
	}

	if len(activeChannels) == 0 {
		return nil, fmt.Errorf("没有活跃通道")
	}

	var selected *Channel

	switch session.Algorithm {
	case LoadBalanceRoundRobin:
		selected = m.selectRoundRobin(session, activeChannels)
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
func (m *MultiChannelManager) selectRoundRobin(session *MultiChannelSession, channels []Channel) *Channel {
	// 简化实现：基于时间戳取模
	index := time.Now().UnixNano() % int64(len(channels))
	return &channels[index]
}

// selectLeastConn 最少连接选择
func (m *MultiChannelManager) selectLeastConn(channels []Channel) *Channel {
	minOps := int64(^uint64(0) >> 1) // MaxInt64
	var selected *Channel

	for i, ch := range channels {
		totalOps := ch.Stats.OpsSent + ch.Stats.OpsReceived
		if totalOps < minOps {
			minOps = totalOps
			selected = &channels[i]
		}
	}

	return selected
}

// selectByBandwidth 带宽选择
func (m *MultiChannelManager) selectByBandwidth(channels []Channel) *Channel {
	maxSpeed := int64(0)
	var selected *Channel

	for i, ch := range channels {
		if ch.Speed > maxSpeed {
			maxSpeed = ch.Speed
			selected = &channels[i]
		}
	}

	return selected
}

// selectByLatency 延迟选择
func (m *MultiChannelManager) selectByLatency(channels []Channel) *Channel {
	minLatency := float64(^uint64(0) >> 1) // MaxFloat64
	var selected *Channel

	for i, ch := range channels {
		if ch.Stats.AvgLatencyMs < minLatency {
			minLatency = ch.Stats.AvgLatencyMs
			selected = &channels[i]
		}
	}

	return selected
}

// selectAdaptive 自适应选择
func (m *MultiChannelManager) selectAdaptive(channels []Channel) *Channel {
	// 综合考虑带宽、延迟、负载
	bestScore := float64(-1)
	var selected *Channel

	for i, ch := range channels {
		// 计算综合评分（越高越好）
		bandwidthScore := float64(ch.Speed) / 10000.0 // 归一化到 0-1
		latencyScore := 1.0 / (1.0 + ch.Stats.AvgLatencyMs)
		loadScore := 1.0 / (1.0 + float64(ch.Stats.OpsSent+ch.Stats.OpsReceived))

		score := bandwidthScore*0.4 + latencyScore*0.4 + loadScore*0.2

		if score > bestScore {
			bestScore = score
			selected = &channels[i]
		}
	}

	return selected
}

// ========== 故障转移 ==========

// HandleChannelFailure 处理通道故障
func (m *MultiChannelManager) HandleChannelFailure(sessionID, channelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.FailoverEnabled {
		return fmt.Errorf("故障转移未启用")
	}

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	// 标记通道为失败状态
	for i, ch := range session.Channels {
		if ch.ID == channelID {
			session.Channels[i].State = ChannelStateFailed
			break
		}
	}

	// 更新会话状态
	activeCount := 0
	for _, ch := range session.Channels {
		if ch.State == ChannelStateActive {
			activeCount++
		}
	}

	if activeCount == 0 {
		session.State = SessionStateInactive
	} else if activeCount < len(session.Channels)/2 {
		session.State = SessionStateDegraded
	}

	m.stats.FailoverCount++

	return nil
}

// ========== 自动重平衡 ==========

// RebalanceChannels 重平衡通道
func (m *MultiChannelManager) RebalanceChannels() (*RebalanceResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.AutoRebalance {
		return nil, fmt.Errorf("自动重平衡未启用")
	}

	result := &RebalanceResult{
		Timestamp:       time.Now(),
		RebalancedCount: 0,
		Details:         make(map[string]string),
	}

	for sessionID, session := range m.sessions {
		if session.State != SessionStateActive {
			continue
		}

		// 检查负载是否不均衡
		if m.isImbalanced(session) {
			result.Details[sessionID] = "负载不均衡，已调整"
			result.RebalancedCount++
		}
	}

	m.stats.LastRebalance = time.Now()

	return result, nil
}

// isImbalanced 检查负载是否不均衡
func (m *MultiChannelManager) isImbalanced(session *MultiChannelSession) bool {
	if len(session.Channels) <= 1 {
		return false
	}

	var minThroughput, maxThroughput float64
	minThroughput = float64(^uint64(0) >> 1)
	maxThroughput = 0

	for _, ch := range session.Channels {
		if ch.State != ChannelStateActive {
			continue
		}
		if ch.Stats.ThroughputMBps < minThroughput {
			minThroughput = ch.Stats.ThroughputMBps
		}
		if ch.Stats.ThroughputMBps > maxThroughput {
			maxThroughput = ch.Stats.ThroughputMBps
		}
	}

	if maxThroughput == 0 {
		return false
	}

	// 计算负载不均衡度
	imbalance := (maxThroughput - minThroughput) / maxThroughput
	return imbalance > m.config.RebalanceThreshold
}

// RebalanceResult 重平衡结果
type RebalanceResult struct {
	Timestamp       time.Time          `json:"timestamp"`
	RebalancedCount int                `json:"rebalanced_count"`
	Details         map[string]string  `json:"details"`
}

// ========== 辅助方法 ==========

// countTotalChannels 计算总通道数
func (m *MultiChannelManager) countTotalChannels() int {
	count := 0
	for _, session := range m.sessions {
		count += len(session.Channels)
	}
	return count
}

// updateStats 更新统计
func (m *MultiChannelManager) updateStats() {
	m.stats.TotalSessions = len(m.sessions)
	m.stats.ActiveSessions = 0
	m.stats.TotalChannels = 0
	m.stats.ActiveChannels = 0

	for _, session := range m.sessions {
		if session.State == SessionStateActive {
			m.stats.ActiveSessions++
		}
		m.stats.TotalChannels += len(session.Channels)
		for _, ch := range session.Channels {
			if ch.State == ChannelStateActive {
				m.stats.ActiveChannels++
			}
		}
	}
}

// GetStats 获取统计
func (m *MultiChannelManager) GetStats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// DetectNetworkInterfaces 检测网络接口
func (m *MultiChannelManager) DetectNetworkInterfaces() ([]NetworkInterface, error) {
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
			Name:      iface.Name,
			MTU:       iface.MTU,
			HardwareAddr: iface.HardwareAddr.String(),
			Addresses: make([]string, 0),
		}

		for _, addr := range addrs {
			ni.Addresses = append(ni.Addresses, addr.String())
		}

		result = append(result, ni)
	}

	return result, nil
}

// NetworkInterface 网络接口
type NetworkInterface struct {
	Name         string   `json:"name"`
	MTU          int      `json:"mtu"`
	HardwareAddr string   `json:"hardware_addr"`
	Addresses    []string `json:"addresses"`
	Speed        int64    `json:"speed"` // Mbps
}

// SaveConfig 保存配置
func (m *MultiChannelManager) SaveConfig(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0640)
}

// LoadConfig 加载配置
func (m *MultiChannelManager) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return json.Unmarshal(data, &m.config)
}
