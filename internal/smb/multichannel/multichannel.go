package multichannel

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// SMBMultichannelManager SMB多通道管理器
// 对标 TrueNAS SCALE 的 Multichannel SMB 功能
// 通过多网卡/多连接聚合提升 SMB 传输带宽
type SMBMultichannelManager struct {
	mu        sync.RWMutex
	config    *MultichannelConfig
	channels  map[string]*ChannelGroup
	stats     *MultichannelStats
	healthMon *ChannelHealthMonitor
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// MultichannelConfig 多通道配置
type MultichannelConfig struct {
	Enabled            bool          `json:"enabled"`
	MaxChannelsPerClient int         `json:"max_channels_per_client"`
	MinChannels        int           `json:"min_channels"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	FailoverTimeout    time.Duration `json:"failover_timeout"`
	BalanceMode        BalanceMode   `json:"balance_mode"`
	Interfaces         []string      `json:"interfaces"`
	MTU                int           `json:"mtu"`
	RSSEnabled         bool          `json:"rss_enabled"` // Receive Side Scaling
}

// BalanceMode 负载均衡模式
type BalanceMode string

const (
	BalanceRoundRobin BalanceMode = "round_robin"
	BalanceLeastLoad  BalanceMode = "least_load"
	BalanceHash       BalanceMode = "hash"       // 基于源/目标IP哈希
	BalanceAdaptive   BalanceMode = "adaptive"   // 自适应，根据延迟和带宽动态调整
)

// ChannelGroup 客户端通道组
type ChannelGroup struct {
	ClientIP   string              `json:"client_ip"`
	Channels   []*NetworkChannel   `json:"channels"`
	TotalBW    int64               `json:"total_bandwidth_mbps"`
	ActiveBW   int64               `json:"active_bandwidth_mbps"`
	CreatedAt  time.Time           `json:"created_at"`
	LastActive time.Time           `json:"last_active"`
	State      ChannelGroupState   `json:"state"`
}

// ChannelGroupState 通道组状态
type ChannelGroupState string

const (
	GroupStateActive   ChannelGroupState = "active"
	GroupStateDegraded ChannelGroupState = "degraded"
	GroupStateFailed   ChannelGroupState = "failed"
)

// NetworkChannel 单个网络通道
type NetworkChannel struct {
	ID           string        `json:"id"`
	LocalAddr    string        `json:"local_addr"`
	RemoteAddr   string        `json:"remote_addr"`
	Interface    string        `json:"interface"`
	Bandwidth    int64         `json:"bandwidth_mbps"`
	Latency      time.Duration `json:"latency"`
	PacketLoss   float64       `json:"packet_loss"`
	State        ChannelState  `json:"state"`
	BytesSent    int64         `json:"bytes_sent"`
	BytesRecv    int64         `json:"bytes_recv"`
	LastHealthAt time.Time     `json:"last_health_at"`
}

// ChannelState 通道状态
type ChannelState string

const (
	ChannelStateUp      ChannelState = "up"
	ChannelStateDown    ChannelState = "down"
	ChannelStateDegraded ChannelState = "degraded"
)

// MultichannelStats 多通道统计
type MultichannelStats struct {
	mu               sync.Mutex
	TotalConnections  int64   `json:"total_connections"`
	ActiveGroups      int     `json:"active_groups"`
	TotalBandwidth    int64   `json:"total_bandwidth_mbps"`
	UtilizedBandwidth int64   `json:"utilized_bandwidth_mbps"`
	FailoverCount     int64   `json:"failover_count"`
	AvgLatency        float64 `json:"avg_latency_ms"`
}

// ChannelHealthMonitor 通道健康监控
type ChannelHealthMonitor struct {
	mu       sync.RWMutex
	results  map[string]*HealthResult
	interval time.Duration
}

// HealthResult 健康检查结果
type HealthResult struct {
	ChannelID  string        `json:"channel_id"`
	Healthy    bool          `json:"healthy"`
	Latency    time.Duration `json:"latency"`
	CheckedAt  time.Time     `json:"checked_at"`
	ErrorMsg   string        `json:"error_msg,omitempty"`
}

// NewSMBMultichannelManager 创建多通道管理器
func NewSMBMultichannelManager(cfg *MultichannelConfig) *SMBMultichannelManager {
	if cfg == nil {
		cfg = &MultichannelConfig{
			Enabled:              true,
			MaxChannelsPerClient: 4,
			MinChannels:          1,
			HealthCheckInterval:  10 * time.Second,
			FailoverTimeout:      5 * time.Second,
			BalanceMode:          BalanceAdaptive,
			MTU:                  9000,
			RSSEnabled:           true,
		}
	}
	if cfg.MaxChannelsPerClient == 0 {
		cfg.MaxChannelsPerClient = 4
	}
	if cfg.HealthCheckInterval == 0 {
		cfg.HealthCheckInterval = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &SMBMultichannelManager{
		config:   cfg,
		channels: make(map[string]*ChannelGroup),
		stats:    &MultichannelStats{},
		healthMon: &ChannelHealthMonitor{
			results:  make(map[string]*HealthResult),
			interval: cfg.HealthCheckInterval,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动多通道管理
func (m *SMBMultichannelManager) Start() error {
	if !m.config.Enabled {
		return nil
	}

	// 检测可用网卡
	if err := m.detectInterfaces(); err != nil {
		return fmt.Errorf("检测网卡失败: %w", err)
	}

	m.wg.Add(1)
	go m.healthCheckLoop()

	return nil
}

// Stop 停止管理
func (m *SMBMultichannelManager) Stop() error {
	m.cancel()
	m.wg.Wait()
	return nil
}

// EstablishChannels 为客户端建立多通道
func (m *SMBMultichannelManager) EstablishChannels(clientIP string) (*ChannelGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if group, exists := m.channels[clientIP]; exists {
		return group, nil
	}

	ifaces, err := m.getAvailableInterfaces()
	if err != nil {
		return nil, err
	}

	channels := make([]*NetworkChannel, 0)
	for i, iface := range ifaces {
		if i >= m.config.MaxChannelsPerClient {
			break
		}

		ch := &NetworkChannel{
			ID:         fmt.Sprintf("%s-%s-%d", clientIP, iface.Name, i),
			LocalAddr:  getInterfaceAddr(iface),
			RemoteAddr: clientIP,
			Interface:  iface.Name,
			Bandwidth:  estimateBandwidth(iface),
			State:      ChannelStateUp,
			LastHealthAt: time.Now(),
		}
		channels = append(channels, ch)
	}

	if len(channels) < m.config.MinChannels {
		return nil, fmt.Errorf("可用通道数 %d 低于最低要求 %d", len(channels), m.config.MinChannels)
	}

	var totalBW int64
	for _, ch := range channels {
		totalBW += ch.Bandwidth
	}

	group := &ChannelGroup{
		ClientIP:   clientIP,
		Channels:   channels,
		TotalBW:    totalBW,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		State:      GroupStateActive,
	}

	m.channels[clientIP] = group
	m.stats.mu.Lock()
	m.stats.ActiveGroups++
	m.stats.TotalBandwidth += totalBW
	m.stats.mu.Unlock()

	return group, nil
}

// SelectChannel 为请求选择最佳通道
func (m *SMBMultichannelManager) SelectChannel(clientIP string) (*NetworkChannel, error) {
	m.mu.RLock()
	group, exists := m.channels[clientIP]
	m.mu.RUnlock()

	if !exists || group.State == GroupStateFailed {
		return nil, fmt.Errorf("客户端 %s 无可用通道", clientIP)
	}

	// 过滤活跃通道
	active := make([]*NetworkChannel, 0)
	for _, ch := range group.Channels {
		if ch.State == ChannelStateUp {
			active = append(active, ch)
		}
	}

	if len(active) == 0 {
		return nil, fmt.Errorf("客户端 %s 所有通道已断开", clientIP)
	}

	switch m.config.BalanceMode {
	case BalanceRoundRobin:
		return m.selectRoundRobin(clientIP, active), nil
	case BalanceLeastLoad:
		return m.selectLeastLoad(active), nil
	case BalanceHash:
		return m.selectByHash(clientIP, active), nil
	default: // adaptive
		return m.selectAdaptive(active), nil
	}
}

func (m *SMBMultichannelManager) selectRoundRobin(clientIP string, channels []*NetworkChannel) *NetworkChannel {
	idx := int(time.Now().UnixNano()) % len(channels)
	return channels[idx]
}

func (m *SMBMultichannelManager) selectLeastLoad(channels []*NetworkChannel) *NetworkChannel {
	best := channels[0]
	bestLoad := best.BytesSent + best.BytesRecv
	for _, ch := range channels[1:] {
		load := ch.BytesSent + ch.BytesRecv
		if load < bestLoad {
			best = ch
			bestLoad = load
		}
	}
	return best
}

func (m *SMBMultichannelManager) selectByHash(clientIP string, channels []*NetworkChannel) *NetworkChannel {
	hash := 0
	for _, b := range clientIP {
		hash += int(b)
	}
	return channels[hash%len(channels)]
}

func (m *SMBMultichannelManager) selectAdaptive(channels []*NetworkChannel) *NetworkChannel {
	best := channels[0]
	bestScore := m.channelScore(channels[0])
	for _, ch := range channels[1:] {
		score := m.channelScore(ch)
		if score > bestScore {
			best = ch
			bestScore = score
		}
	}
	return best
}

func (m *SMBMultichannelManager) channelScore(ch *NetworkChannel) float64 {
	bwScore := float64(ch.Bandwidth) / 1000.0
	latScore := 1.0 / (1.0 + ch.Latency.Seconds()*100)
	lossScore := 1.0 - ch.PacketLoss
	return bwScore*0.4 + latScore*0.3 + lossScore*0.3
}

// ReleaseChannels 释放客户端通道
func (m *SMBMultichannelManager) ReleaseChannels(clientIP string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if group, exists := m.channels[clientIP]; exists {
		m.stats.mu.Lock()
		m.stats.ActiveGroups--
		m.stats.TotalBandwidth -= group.TotalBW
		m.stats.mu.Unlock()
		delete(m.channels, clientIP)
	}
}

// GetStats 获取统计信息
func (m *SMBMultichannelManager) GetStats() *MultichannelStats {
	m.stats.mu.Lock()
	defer m.stats.mu.Unlock()
	stats := *m.stats
	return &stats
}

func (m *SMBMultichannelManager) healthCheckLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.runHealthChecks()
		}
	}
}

func (m *SMBMultichannelManager) runHealthChecks() {
	m.mu.RLock()
	groups := make([]*ChannelGroup, 0, len(m.channels))
	for _, g := range m.channels {
		groups = append(groups, g)
	}
	m.mu.RUnlock()

	for _, group := range groups {
		downCount := 0
		for _, ch := range group.Channels {
			healthy := m.checkChannelHealth(ch)
			ch.LastHealthAt = time.Now()
			if !healthy {
				ch.State = ChannelStateDown
				downCount++
			} else {
				ch.State = ChannelStateUp
			}
		}

		total := len(group.Channels)
		if downCount == total {
			group.State = GroupStateFailed
		} else if downCount > 0 {
			group.State = GroupStateDegraded
			m.stats.mu.Lock()
			m.stats.FailoverCount++
			m.stats.mu.Unlock()
		} else {
			group.State = GroupStateActive
		}
	}
}

func (m *SMBMultichannelManager) checkChannelHealth(ch *NetworkChannel) bool {
	ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
	defer cancel()

	d := net.Dialer{Timeout: 2 * time.Second}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", ch.RemoteAddr+":445")
	if err != nil {
		ch.PacketLoss = 1.0
		return false
	}
	ch.Latency = time.Since(start)
	conn.Close()
	ch.PacketLoss = 0
	return true
}

func (m *SMBMultichannelManager) detectInterfaces() error {
	if len(m.config.Interfaces) > 0 {
		return nil
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		if len(addrs) > 0 {
			m.config.Interfaces = append(m.config.Interfaces, iface.Name)
		}
	}
	return nil
}

func (m *SMBMultichannelManager) getAvailableInterfaces() ([]net.Interface, error) {
	result := make([]net.Interface, 0)
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		for _, name := range m.config.Interfaces {
			if iface.Name == name {
				result = append(result, iface)
				break
			}
		}
	}
	return result, nil
}

func getInterfaceAddr(iface net.Interface) string {
	addrs, _ := iface.Addrs()
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return ""
}

func estimateBandwidth(iface net.Interface) int64 {
	if iface.MTU >= 9000 {
		return 10000 // 10Gbps for jumbo frame capable
	}
	return 1000 // 1Gbps default
}
