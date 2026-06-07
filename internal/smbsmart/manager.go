// Package smbsmart SMB 多通道优化模块 - 业务逻辑层
package smbsmart

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 错误定义 ==========

var (
	// ErrChannelNotFound 通道未找到
	ErrChannelNotFound = fmt.Errorf("通道未找到")
	// ErrSessionNotFound 会话未找到
	ErrSessionNotFound = fmt.Errorf("会话未找到")
	// ErrBondNotFound 绑定未找到
	ErrBondNotFound = fmt.Errorf("绑定未找到")
	// ErrChannelAlreadyBonded 通道已被绑定
	ErrChannelAlreadyBonded = fmt.Errorf("通道已被绑定")
	// ErrInsufficientChannels 通道数不足
	ErrInsufficientChannels = fmt.Errorf("至少需要2个通道")
)

// ========== 管理器 ==========

// Manager SMB 多通道管理器
type Manager struct {
	mu           sync.RWMutex
	channels     map[string]*SMBChannel  // channelID -> Channel
	sessions     map[string]*SMBSession  // sessionID -> Session
	bonds        map[string]*ChannelBond // bondID -> Bond
	failover     FailoverConfig
	bwHistory    []BandwidthStats
	peakReadBps  int64
	peakWriteBps int64
}

// NewManager 创建 SMB 多通道管理器
func NewManager() *Manager {
	return &Manager{
		channels:  make(map[string]*SMBChannel),
		sessions:  make(map[string]*SMBSession),
		bonds:     make(map[string]*ChannelBond),
		failover:  DefaultFailoverConfig(),
		bwHistory: make([]BandwidthStats, 0),
	}
}

// ========== 通道管理 ==========

// ListChannels 列出所有 SMB 通道
func (m *Manager) ListChannels() []*SMBChannel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]*SMBChannel, 0, len(m.channels))
	for _, ch := range m.channels {
		cp := *ch
		channels = append(channels, &cp)
	}
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Name < channels[j].Name
	})
	return channels
}

// GetChannel 获取通道详情
func (m *Manager) GetChannel(id string) (*SMBChannel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ch, ok := m.channels[id]
	if !ok {
		return nil, ErrChannelNotFound
	}
	cp := *ch
	return &cp, nil
}

// DiscoverChannels 发现系统中的 SMB 通道
func (m *Manager) DiscoverChannels(ctx context.Context) ([]*SMBChannel, error) {
	// 获取 SMB 连接信息
	cmd := exec.CommandContext(ctx, "smbstatus", "--shares", "-b")
	output, _ := cmd.CombinedOutput()

	var discovered []*SMBChannel
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Pid") || strings.HasPrefix(line, "---") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			ch := &SMBChannel{
				ID:           uuid.New().String(),
				Name:         fmt.Sprintf("ch-%s-%s", fields[0], fields[3]),
				Type:         ChannelTypeTCP,
				Status:       ChannelStatusActive,
				RemoteIP:     fields[3],
				ConnectedAt:  time.Now(),
				LastActivity: time.Now(),
				MaxCredits:   128,
				Credits:      128,
			}
			discovered = append(discovered, ch)
		}
	}

	// 如果没有从 smbstatus 获取到，尝试从网络接口发现
	if len(discovered) == 0 {
		discovered = m.discoverFromInterfaces(ctx)
	}

	// 注册发现的通道
	m.mu.Lock()
	for _, ch := range discovered {
		if _, exists := m.channels[ch.ID]; !exists {
			m.channels[ch.ID] = ch
		}
	}
	m.mu.Unlock()

	return discovered, nil
}

// EnableChannel 启用通道
func (m *Manager) EnableChannel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, ok := m.channels[id]
	if !ok {
		return ErrChannelNotFound
	}
	ch.Status = ChannelStatusActive
	return nil
}

// DisableChannel 禁用通道
func (m *Manager) DisableChannel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, ok := m.channels[id]
	if !ok {
		return ErrChannelNotFound
	}
	ch.Status = ChannelStatusDisabled
	return nil
}

// ========== 会话管理 ==========

// GetSession 获取 SMB 会话详情
func (m *Manager) GetSession(sessionID string) (*SMBSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	cp := *session
	return &cp, nil
}

// ListSessions 列出所有 SMB 会话
func (m *Manager) ListSessions() []*SMBSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*SMBSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		cp := *s
		sessions = append(sessions, &cp)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ConnectedAt.After(sessions[j].ConnectedAt)
	})
	return sessions
}

// RefreshSessions 从系统刷新会话信息
func (m *Manager) RefreshSessions(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "smbstatus", "-S")
	output, _ := cmd.CombinedOutput()

	lines := strings.Split(string(output), "\n")
	var sessions []*SMBSession

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Pid") || strings.HasPrefix(line, "---") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			session := &SMBSession{
				ID:           uuid.New().String(),
				Username:     fields[0],
				ClientIP:     fields[3],
				Status:       SessionStatusActive,
				ConnectedAt:  time.Now(),
				LastActivity: time.Now(),
				Dialect:      "3.1.1",
			}
			sessions = append(sessions, session)
		}
	}

	m.mu.Lock()
	for _, s := range sessions {
		m.sessions[s.ID] = s
	}
	m.mu.Unlock()

	return nil
}

// ========== 通道绑定 ==========

// BondChannels 绑定多个通道
func (m *Manager) BondChannels(req BondChannelsRequest) (*ChannelBond, error) {
	if len(req.ChannelIDs) < 2 {
		return nil, ErrInsufficientChannels
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证所有通道存在且未被绑定
	for _, chID := range req.ChannelIDs {
		ch, ok := m.channels[chID]
		if !ok {
			return nil, fmt.Errorf("通道 %s: %w", chID, ErrChannelNotFound)
		}
		// 检查是否已被其他绑定使用
		for _, bond := range m.bonds {
			for _, bid := range bond.ChannelIDs {
				if bid == chID {
					return nil, fmt.Errorf("通道 %s: %w", chID, ErrChannelAlreadyBonded)
				}
			}
		}
		_ = ch
	}

	// 计算聚合带宽
	var totalSpeed int64
	for _, chID := range req.ChannelIDs {
		if ch, ok := m.channels[chID]; ok {
			totalSpeed += ch.Speed
		}
	}

	bond := &ChannelBond{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Mode:       req.Mode,
		ChannelIDs: req.ChannelIDs,
		Enabled:    true,
		TotalSpeed: totalSpeed,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// 主备模式下设置活跃通道
	if req.Mode == BondModeActiveBackup && len(req.ChannelIDs) > 0 {
		bond.ActiveChannelID = req.ChannelIDs[0]
	}

	m.bonds[bond.ID] = bond

	// 更新通道状态
	for _, chID := range req.ChannelIDs {
		if ch, ok := m.channels[chID]; ok {
			if req.Mode == BondModeActiveBackup && chID != bond.ActiveChannelID {
				ch.Status = ChannelStatusStandby
			} else {
				ch.Status = ChannelStatusActive
			}
		}
	}

	return bond, nil
}

// UnbondChannels 解除通道绑定
func (m *Manager) UnbondChannels(bondID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bond, ok := m.bonds[bondID]
	if !ok {
		return ErrBondNotFound
	}

	// 恢复通道状态
	for _, chID := range bond.ChannelIDs {
		if ch, ok := m.channels[chID]; ok {
			ch.Status = ChannelStatusActive
		}
	}

	delete(m.bonds, bondID)
	return nil
}

// ListBonds 列出所有绑定
func (m *Manager) ListBonds() []*ChannelBond {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bonds := make([]*ChannelBond, 0, len(m.bonds))
	for _, b := range m.bonds {
		cp := *b
		bonds = append(bonds, &cp)
	}
	return bonds
}

// GetBond 获取绑定详情
func (m *Manager) GetBond(bondID string) (*ChannelBond, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bond, ok := m.bonds[bondID]
	if !ok {
		return nil, ErrBondNotFound
	}
	cp := *bond
	return &cp, nil
}

// ========== 带宽监控 ==========

// GetBandwidth 获取当前带宽统计
func (m *Manager) GetBandwidth(ctx context.Context) (*BandwidthStats, error) {
	m.mu.RLock()
	channels := make([]*SMBChannel, 0, len(m.channels))
	for _, ch := range m.channels {
		cp := *ch
		channels = append(channels, &cp)
	}
	m.mu.RUnlock()

	stats := &BandwidthStats{
		Timestamp:    time.Now(),
		ChannelStats: make(map[string]ChannelBwStats),
	}

	activeCount := 0
	for _, ch := range channels {
		if ch.Status == ChannelStatusActive {
			activeCount++
		}

		chStats := ChannelBwStats{
			ChannelID: ch.ID,
			ReadBps:   ch.ReadBytes,
			WriteBps:  ch.WriteBytes,
			TotalBps:  ch.ReadBytes + ch.WriteBytes,
			ReadIOPS:  ch.ReadIOPS,
			WriteIOPS: ch.WriteIOPS,
			LatencyMs: ch.LatencyMs,
		}
		if ch.Speed > 0 {
			chStats.Utilization = float64(ch.ReadBytes+ch.WriteBytes) / float64(ch.Speed) * 100
		}

		stats.ChannelStats[ch.ID] = chStats
		stats.TotalReadBps += ch.ReadBytes
		stats.TotalWriteBps += ch.WriteBytes
		stats.TotalReadIOPS += ch.ReadIOPS
		stats.TotalWriteIOPS += ch.WriteIOPS
	}
	stats.TotalBps = stats.TotalReadBps + stats.TotalWriteBps
	stats.ActiveChannels = activeCount
	stats.TotalChannels = len(channels)

	if len(channels) > 0 {
		var totalLatency float64
		for _, ch := range channels {
			totalLatency += ch.LatencyMs
		}
		stats.AvgLatencyMs = totalLatency / float64(len(channels))
	}

	// 更新峰值
	m.mu.Lock()
	if stats.TotalReadBps > m.peakReadBps {
		m.peakReadBps = stats.TotalReadBps
	}
	if stats.TotalWriteBps > m.peakWriteBps {
		m.peakWriteBps = stats.TotalWriteBps
	}
	stats.PeakReadBps = m.peakReadBps
	stats.PeakWriteBps = m.peakWriteBps

	// 保存历史
	m.bwHistory = append(m.bwHistory, *stats)
	if len(m.bwHistory) > 1000 {
		m.bwHistory = m.bwHistory[len(m.bwHistory)-1000:]
	}
	m.mu.Unlock()

	return stats, nil
}

// GetBandwidthHistory 获取带宽历史
func (m *Manager) GetBandwidthHistory(limit int) []BandwidthStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.bwHistory) {
		limit = len(m.bwHistory)
	}
	start := len(m.bwHistory) - limit
	result := make([]BandwidthStats, limit)
	copy(result, m.bwHistory[start:])
	return result
}

// ========== 故障转移配置 ==========

// GetFailoverConfig 获取故障转移配置
func (m *Manager) GetFailoverConfig() FailoverConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.failover
}

// ConfigureFailover 配置故障转移
func (m *Manager) ConfigureFailover(req UpdateFailoverConfigRequest) (*FailoverConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Enabled != nil {
		m.failover.Enabled = *req.Enabled
	}
	if req.HealthCheckInterval != "" {
		d, err := time.ParseDuration(req.HealthCheckInterval)
		if err != nil {
			return nil, fmt.Errorf("无效的健康检查间隔: %w", err)
		}
		m.failover.HealthCheckInterval = d
	}
	if req.HealthCheckTimeout != "" {
		d, err := time.ParseDuration(req.HealthCheckTimeout)
		if err != nil {
			return nil, fmt.Errorf("无效的健康检查超时: %w", err)
		}
		m.failover.HealthCheckTimeout = d
	}
	if req.FailureThreshold != nil {
		m.failover.FailureThreshold = *req.FailureThreshold
	}
	if req.RecoveryThreshold != nil {
		m.failover.RecoveryThreshold = *req.RecoveryThreshold
	}
	if req.AutoRebalance != nil {
		m.failover.AutoRebalance = *req.AutoRebalance
	}
	if req.RebalanceThreshold != nil {
		m.failover.RebalanceThreshold = *req.RebalanceThreshold
	}
	if req.NotificationEnabled != nil {
		m.failover.NotificationEnabled = *req.NotificationEnabled
	}
	if req.WebhookURL != nil {
		m.failover.WebhookURL = *req.WebhookURL
	}

	m.failover.UpdatedAt = time.Now()
	result := m.failover
	return &result, nil
}

// ========== 故障转移执行 ==========

// RunHealthCheck 执行通道健康检查
func (m *Manager) RunHealthCheck(ctx context.Context) ([]string, []string) {
	m.mu.RLock()
	channels := make([]*SMBChannel, 0, len(m.channels))
	for _, ch := range m.channels {
		cp := *ch
		channels = append(channels, &cp)
	}
	threshold := m.failover.FailureThreshold
	m.mu.RUnlock()

	var failed, recovered []string

	for _, ch := range channels {
		alive := m.checkChannelHealth(ctx, ch)

		m.mu.Lock()
		stored, ok := m.channels[ch.ID]
		if !ok {
			m.mu.Unlock()
			continue
		}

		if alive {
			if stored.Status == ChannelStatusFailed {
				stored.ErrorCount = 0
				stored.Status = ChannelStatusActive
				recovered = append(recovered, ch.ID)
			}
		} else {
			stored.ErrorCount++
			stored.LastError = "健康检查失败"
			if int(stored.ErrorCount) >= threshold {
				stored.Status = ChannelStatusFailed
				failed = append(failed, ch.ID)
			}
		}
		m.mu.Unlock()
	}

	// 执行故障转移
	if len(failed) > 0 {
		m.performFailover(failed)
	}

	return failed, recovered
}

// ========== 内部辅助方法 ==========

func (m *Manager) discoverFromInterfaces(ctx context.Context) []*SMBChannel {
	cmd := exec.CommandContext(ctx, "ip", "-4", "addr", "show")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	var channels []*SMBChannel
	lines := strings.Split(string(output), "\n")
	var currentIface string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, ": ") && !strings.HasPrefix(line, "inet") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				currentIface = strings.TrimSpace(strings.Fields(parts[1])[0])
			}
			continue
		}
		if strings.HasPrefix(line, "inet ") && currentIface != "" && currentIface != "lo" {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ip := strings.Split(fields[1], "/")[0]
				ch := &SMBChannel{
					ID:            uuid.New().String(),
					Name:          fmt.Sprintf("ch-%s", currentIface),
					Type:          ChannelTypeTCP,
					Status:        ChannelStatusActive,
					LocalIP:       ip,
					InterfaceName: currentIface,
					Speed:         1000000000, // 默认 1Gbps
					MTU:           1500,
					MaxCredits:    128,
					Credits:       128,
					ConnectedAt:   time.Now(),
					LastActivity:  time.Now(),
				}
				channels = append(channels, ch)
			}
		}
	}
	return channels
}

func (m *Manager) checkChannelHealth(ctx context.Context, ch *SMBChannel) bool {
	if ch.RemoteIP == "" {
		return true
	}

	timeout := m.failover.HealthCheckTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", ch.RemoteIP)
	err := cmd.Run()
	return err == nil
}

func (m *Manager) performFailover(failedIDs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, bond := range m.bonds {
		if !bond.Enabled || bond.Mode != BondModeActiveBackup {
			continue
		}

		// 检查活跃通道是否故障
		activeIsFailed := false
		for _, fid := range failedIDs {
			if bond.ActiveChannelID == fid {
				activeIsFailed = true
				break
			}
		}

		if activeIsFailed {
			// 找到备用通道
			for _, chID := range bond.ChannelIDs {
				if chID == bond.ActiveChannelID {
					continue
				}
				if ch, ok := m.channels[chID]; ok && ch.Status != ChannelStatusFailed {
					bond.ActiveChannelID = chID
					ch.Status = ChannelStatusActive
					bond.FailoverCount++
					bond.LastFailover = time.Now()
					bond.UpdatedAt = time.Now()
					break
				}
			}
		}
	}
}

// GetChannelStats 获取通道统计（供外部使用）
func (m *Manager) GetChannelStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_channels":  len(m.channels),
		"active_channels": m.countByStatus(ChannelStatusActive),
		"failed_channels": m.countByStatus(ChannelStatusFailed),
		"total_sessions":  len(m.sessions),
		"total_bonds":     len(m.bonds),
	}
}

func (m *Manager) countByStatus(status ChannelStatus) int {
	count := 0
	for _, ch := range m.channels {
		if ch.Status == status {
			count++
		}
	}
	return count
}

// parseSpeed 解析速度字符串
func parseSpeed(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}
