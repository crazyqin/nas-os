// Package hafailover 高可用故障转移模块
package hafailover

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// NewManager 创建HA管理器
func NewManager(configPath string) *Manager {
	m := &Manager{
		configPath:    configPath,
		heartbeatStop: make(map[HeartbeatLevel]chan struct{}),
		syncState:     SyncStateIdle,
		events:        make([]FailoverEvent, 0),
	}
	_ = m.loadConfig()
	return m
}

// ========== 配置管理 ==========

// GetConfig 获取HA配置
func (m *Manager) GetConfig() *HAConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config == nil {
		return &HAConfig{
			ClusterName:   "nas-os-ha",
			AutoFailover:  true,
			FailoverDelay: 5,
			Heartbeats: map[HeartbeatLevel]HeartbeatConfig{
				HeartbeatNetwork:  {Interval: 5, Timeout: 15, MaxRetries: 3},
				HeartbeatStorage:  {Interval: 10, Timeout: 30, MaxRetries: 3},
				HeartbeatService:  {Interval: 15, Timeout: 45, MaxRetries: 3},
			},
			VIP: VIPConfig{
				Enabled:   false,
				Interface: "eth0",
				Netmask:   "255.255.255.0",
			},
			Sync: SyncConfig{
				StorageSync:  true,
				ServiceSync:  true,
				SyncInterval: 60,
			},
		}
	}
	configCopy := *m.config
	return &configCopy
}

// UpdateConfig 更新HA配置
func (m *Manager) UpdateConfig(req *HAConfig) (*HAConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.ClusterName == "" {
		return nil, fmt.Errorf("集群名称不能为空")
	}
	if req.LocalNodeID == "" {
		return nil, fmt.Errorf("本节点ID不能为空")
	}

	if m.config == nil {
		m.config = &HAConfig{}
	}

	m.config.ClusterName = req.ClusterName
	m.config.LocalNodeID = req.LocalNodeID
	m.config.PeerNodeID = req.PeerNodeID
	m.config.AutoFailover = req.AutoFailover
	m.config.FailoverDelay = req.FailoverDelay
	m.config.Heartbeats = req.Heartbeats
	m.config.VIP = req.VIP
	m.config.Sync = req.Sync

	if err := m.saveConfig(); err != nil {
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	configCopy := *m.config
	return &configCopy, nil
}

// ========== 节点管理 ==========

// ListNodes 列出所有节点
func (m *Manager) ListNodes() []*NodeInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*NodeInfo, 0)
	if m.localNode != nil {
		nodeCopy := *m.localNode
		nodes = append(nodes, &nodeCopy)
	}
	if m.peerNode != nil {
		nodeCopy := *m.peerNode
		nodes = append(nodes, &nodeCopy)
	}
	return nodes
}

// GetNode 获取节点信息
func (m *Manager) GetNode(id string) (*NodeInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.localNode != nil && m.localNode.ID == id {
		nodeCopy := *m.localNode
		return &nodeCopy, nil
	}
	if m.peerNode != nil && m.peerNode.ID == id {
		nodeCopy := *m.peerNode
		return &nodeCopy, nil
	}
	return nil, fmt.Errorf("节点 %s 不存在", id)
}

// RegisterNode 注册节点
func (m *Manager) RegisterNode(req *NodeInfo) (*NodeInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("节点名称不能为空")
	}
	if req.IP == "" {
		return nil, fmt.Errorf("节点IP不能为空")
	}

	node := &NodeInfo{
		ID:              uuid.New().String(),
		Name:            req.Name,
		Hostname:        req.Hostname,
		IP:              req.IP,
		Role:            req.Role,
		Status:          StatusOnline,
		HeartbeatStatus: make(map[HeartbeatLevel]bool),
		Services:        req.Services,
		SystemInfo:      req.SystemInfo,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if m.localNode == nil {
		m.localNode = node
	} else if m.peerNode == nil {
		m.peerNode = node
	} else {
		return nil, fmt.Errorf("集群已满（最多2个节点）")
	}

	return node, nil
}

// GetHAStatus 获取HA集群状态
func (m *Manager) GetHAStatus() *HAStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &HAStatus{
		ClusterName:   "",
		VIPStatus:     "inactive",
		SyncState:     m.syncState,
		LastSyncAt:    m.lastSyncAt,
		FailoverCount: len(m.events),
		UpdatedAt:     time.Now(),
	}

	if m.config != nil {
		status.ClusterName = m.config.ClusterName
		status.VIPIP = m.config.VIP.VIP
		if m.vipActive {
			status.VIPStatus = "active"
		}
	}

	if m.localNode != nil {
		nodeCopy := *m.localNode
		if nodeCopy.Role == RoleActive {
			status.ActiveNode = &nodeCopy
		} else {
			status.StandbyNode = &nodeCopy
		}
	}
	if m.peerNode != nil {
		nodeCopy := *m.peerNode
		if nodeCopy.Role == RoleActive {
			status.ActiveNode = &nodeCopy
		} else {
			status.StandbyNode = &nodeCopy
		}
	}

	// 计算健康分数
	status.HealthScore = m.calculateHealthScore()

	if len(m.events) > 0 {
		lastEvent := m.events[len(m.events)-1]
		status.LastFailover = &lastEvent
	}

	return status
}

// ========== 心跳管理 ==========

// StartHeartbeat 启动心跳检测
func (m *Manager) StartHeartbeat(level HeartbeatLevel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config == nil {
		return fmt.Errorf("HA未配置")
	}

	if _, exists := m.heartbeatStop[level]; exists {
		return fmt.Errorf("心跳 %s 已在运行", level)
	}

	stopCh := make(chan struct{})
	m.heartbeatStop[level] = stopCh

	go m.runHeartbeat(level, stopCh)
	return nil
}

// StopHeartbeat 停止心跳检测
func (m *Manager) StopHeartbeat(level HeartbeatLevel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stopCh, exists := m.heartbeatStop[level]
	if !exists {
		return fmt.Errorf("心跳 %s 未在运行", level)
	}

	close(stopCh)
	delete(m.heartbeatStop, level)
	return nil
}

// GetHeartbeatStatus 获取心跳状态
func (m *Manager) GetHeartbeatStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]interface{})
	for level, stopCh := range m.heartbeatStop {
		running := true
		select {
		case <-stopCh:
			running = false
		default:
		}
		status[string(level)] = running
	}
	return status
}

func (m *Manager) runHeartbeat(level HeartbeatLevel, stopCh chan struct{}) {
	config := m.config.Heartbeats[level]
	ticker := time.NewTicker(time.Duration(config.Interval) * time.Second)
	defer ticker.Stop()

	failCount := 0

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			healthy := m.checkHeartbeat(level)
			m.mu.Lock()
			if m.peerNode != nil {
				m.peerNode.HeartbeatStatus[level] = healthy
				m.peerNode.UpdatedAt = time.Now()
			}
			m.mu.Unlock()

			if !healthy {
				failCount++
				if failCount >= config.MaxRetries {
					m.mu.RLock()
					autoFailover := m.config != nil && m.config.AutoFailover
					m.mu.RUnlock()
					if autoFailover {
						m.triggerAutoFailover(fmt.Sprintf("心跳 %s 连续失败 %d 次", level, failCount))
					}
					failCount = 0
				}
			} else {
				failCount = 0
			}
		}
	}
}

func (m *Manager) checkHeartbeat(level HeartbeatLevel) bool {
	m.mu.RLock()
	peerIP := ""
	if m.peerNode != nil {
		peerIP = m.peerNode.IP
	}
	m.mu.RUnlock()

	if peerIP == "" {
		return false
	}

	switch level {
	case HeartbeatNetwork:
		return m.checkNetworkHeartbeat(peerIP)
	case HeartbeatStorage:
		return m.checkStorageHeartbeat(peerIP)
	case HeartbeatService:
		return m.checkServiceHeartbeat(peerIP)
	default:
		return false
	}
}

func (m *Manager) checkNetworkHeartbeat(peerIP string) bool {
	// 简单 ping 检测
	conn, err := net.DialTimeout("tcp", peerIP+":22", 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (m *Manager) checkStorageHeartbeat(peerIP string) bool {
	// 检查存储可达性（通过 NFS/SMB 端口检测）
	ports := []string{"2049", "445"}
	for _, port := range ports {
		conn, err := net.DialTimeout("tcp", peerIP+":"+port, 3*time.Second)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

func (m *Manager) checkServiceHeartbeat(peerIP string) bool {
	// 检查服务健康（HTTP 健康端点）
	conn, err := net.DialTimeout("tcp", peerIP+":8080", 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// ========== 故障切换 ==========

// ManualFailover 手动故障切换
func (m *Manager) ManualFailover(req *FailoverRequest) (*FailoverEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failoverActive {
		return nil, fmt.Errorf("故障切换正在进行中")
	}

	if m.localNode == nil || m.peerNode == nil {
		return nil, fmt.Errorf("节点信息不完整，无法切换")
	}

	if req.Reason == "" {
		return nil, fmt.Errorf("请提供切换原因")
	}

	m.failoverActive = true
	defer func() { m.failoverActive = false }()

	event := FailoverEvent{
		ID:         uuid.New().String(),
		TriggeredAt: time.Now(),
		Trigger:    TriggerManual,
		FromNode:   m.localNode.ID,
		ToNode:     m.peerNode.ID,
		Reason:     req.Reason,
	}

	startTime := time.Now()
	err := m.executeFailover(req.Force)
	event.Duration = time.Since(startTime).Milliseconds()

	if err != nil {
		event.Success = false
		event.Error = err.Error()
	} else {
		event.Success = true
		now := time.Now()
		event.CompletedAt = &now
	}

	m.events = append(m.events, event)
	return &event, nil
}

// GetFailoverHistory 获取切换历史
func (m *Manager) GetFailoverHistory(limit int) []FailoverEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := m.events
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}

	result := make([]FailoverEvent, len(events))
	copy(result, events)
	return result
}

func (m *Manager) triggerAutoFailover(reason string) {
	m.mu.Lock()
	if m.failoverActive {
		m.mu.Unlock()
		return
	}
	m.failoverActive = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.failoverActive = false
		m.mu.Unlock()
	}()

	event := FailoverEvent{
		ID:         uuid.New().String(),
		TriggeredAt: time.Now(),
		Trigger:    TriggerAuto,
		FromNode:   m.localNode.ID,
		ToNode:     m.peerNode.ID,
		Reason:     reason,
	}

	startTime := time.Now()
	err := m.executeFailover(false)
	event.Duration = time.Since(startTime).Milliseconds()

	if err != nil {
		event.Success = false
		event.Error = err.Error()
	} else {
		event.Success = true
		now := time.Now()
		event.CompletedAt = &now
	}

	m.mu.Lock()
	m.events = append(m.events, event)
	m.mu.Unlock()
}

func (m *Manager) executeFailover(force bool) error {
	// 1. VIP 漂移
	if m.config != nil && m.config.VIP.Enabled {
		if err := m.vipDown(); err != nil && !force {
			return fmt.Errorf("VIP下线失败: %w", err)
		}
	}

	// 2. 角色切换
	m.localNode, m.peerNode = m.peerNode, m.localNode
	m.localNode.Role = RoleActive
	m.peerNode.Role = RoleStandby
	m.localNode.UpdatedAt = time.Now()
	m.peerNode.UpdatedAt = time.Now()

	// 3. VIP 上线到新活动节点
	if m.config != nil && m.config.VIP.Enabled {
		if err := m.vipUp(); err != nil && !force {
			return fmt.Errorf("VIP上线失败: %w", err)
		}
	}

	return nil
}

func (m *Manager) vipUp() error {
	if m.config == nil || m.config.VIP.VIP == "" {
		return fmt.Errorf("VIP未配置")
	}

	iface := m.config.VIP.Interface
	vip := m.config.VIP.VIP

	// 使用 ip addr add 添加 VIP
	cmd := exec.Command("ip", "addr", "add", vip+"/24", "dev", iface)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("添加VIP失败: %w", err)
	}

	m.vipActive = true
	return nil
}

func (m *Manager) vipDown() error {
	if m.config == nil || m.config.VIP.VIP == "" {
		return nil
	}

	iface := m.config.VIP.Interface
	vip := m.config.VIP.VIP

	// 使用 ip addr del 删除 VIP
	cmd := exec.Command("ip", "addr", "del", vip+"/24", "dev", iface)
	if err := cmd.Run(); err != nil {
		// VIP 可能不存在，忽略错误
		return nil
	}

	m.vipActive = false
	return nil
}

// ========== 数据同步 ==========

// TriggerSync 手动触发同步
func (m *Manager) TriggerSync() (*SyncStatus, error) {
	m.mu.Lock()
	if m.syncState == SyncStateSyncing {
		m.mu.Unlock()
		return nil, fmt.Errorf("同步正在进行中")
	}
	m.syncState = SyncStateSyncing
	m.mu.Unlock()

	go m.executeSync()

	return m.GetSyncStatus(), nil
}

// GetSyncStatus 获取同步状态
func (m *Manager) GetSyncStatus() *SyncStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &SyncStatus{
		State:     m.syncState,
		LastSyncAt: m.lastSyncAt,
	}
	return status
}

func (m *Manager) executeSync() {
	startTime := time.Now()

	// 模拟同步过程
	time.Sleep(2 * time.Second)

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.syncState = SyncStateCompleted
	m.lastSyncAt = &now
	_ = time.Since(startTime)
}

// ========== 内部工具 ==========

func (m *Manager) calculateHealthScore() int {
	// 如果没有配置任何节点，返回 100（未配置状态）
	if m.localNode == nil && m.peerNode == nil {
		return 100
	}

	score := 100

	if m.localNode == nil || m.localNode.Status != StatusOnline {
		score -= 40
	}
	if m.peerNode == nil || m.peerNode.Status != StatusOnline {
		score -= 30
	}
	if m.syncState == SyncStateFailed {
		score -= 20
	}
	if m.config != nil && m.config.VIP.Enabled && !m.vipActive {
		score -= 10
	}

	if score < 0 {
		score = 0
	}
	return score
}

func (m *Manager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var config HAConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	m.config = &config
	return nil
}

func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}
