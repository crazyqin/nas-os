// Package haconfig 提供高可用配置管理
// 支持主备切换、健康检查、故障转移配置
package haconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// 节点角色.
const (
	RolePrimary   = "primary"
	RoleSecondary = "secondary"
	RoleStandby   = "standby"
)

// 健康状态.
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusDegraded  = "degraded"
	HealthStatusUnhealthy = "unhealthy"
	HealthStatusUnknown   = "unknown"
)

// 错误定义.
var (
	ErrNotPrimary         = errors.New("current node is not primary")
	ErrNoHealthyStandby   = errors.New("no healthy standby available")
	ErrFailoverInProgress = errors.New("failover already in progress")
	ErrConfigNotFound     = errors.New("HA configuration not found")
)

// HAConfig 高可用配置.
type HAConfig struct {
	// 集群配置
	ClusterName string `json:"cluster_name" yaml:"cluster_name"`
	NodeID      string `json:"node_id" yaml:"node_id"`
	NodeRole    string `json:"node_role" yaml:"node_role"`

	// 优先级（数值越大优先级越高）
	Priority int `json:"priority" yaml:"priority"`

	// 心跳配置
	HeartbeatEnabled  bool          `json:"heartbeat_enabled" yaml:"heartbeat_enabled"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`
	HeartbeatTimeout  time.Duration `json:"heartbeat_timeout" yaml:"heartbeat_timeout"`

	// 故障转移配置
	FailoverEnabled       bool          `json:"failover_enabled" yaml:"failover_enabled"`
	FailoverTimeout       time.Duration `json:"failover_timeout" yaml:"failover_timeout"`
	FailbackEnabled       bool          `json:"failback_enabled" yaml:"failback_enabled"`
	FailbackDelay         time.Duration `json:"failback_delay" yaml:"failback_delay"`
	AutoFailbackThreshold float64       `json:"auto_failback_threshold" yaml:"auto_failback_threshold"`

	// 健康检查配置
	HealthCheckEnabled  bool          `json:"health_check_enabled" yaml:"health_check_enabled"`
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout" yaml:"health_check_timeout"`
	HealthCheckRetries  int           `json:"health_check_retries" yaml:"health_check_retries"`

	// 数据同步配置
	SyncEnabled   bool          `json:"sync_enabled" yaml:"sync_enabled"`
	SyncInterval  time.Duration `json:"sync_interval" yaml:"sync_interval"`
	SyncTimeout   time.Duration `json:"sync_timeout" yaml:"sync_timeout"`
	SyncBatchSize int           `json:"sync_batch_size" yaml:"sync_batch_size"`

	// 网络配置
	BindAddress string   `json:"bind_address" yaml:"bind_address"`
	PeerNodes   []string `json:"peer_nodes" yaml:"peer_nodes"`

	// 存储配置
	DataDir string `json:"data_dir" yaml:"data_dir"`

	// 通知配置
	NotifyOnFailover bool     `json:"notify_on_failover" yaml:"notify_on_failover"`
	NotifyEmails     []string `json:"notify_emails" yaml:"notify_emails"`
	NotifyWebhooks   []string `json:"notify_webhooks" yaml:"notify_webhooks"`
}

// NodeState 节点状态.
type NodeState struct {
	NodeID        string            `json:"node_id"`
	Role          string            `json:"role"`
	Status        string            `json:"status"`
	Priority      int               `json:"priority"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	LastSync      time.Time         `json:"last_sync"`
	IsHealthy     bool              `json:"is_healthy"`
	Address       string            `json:"address"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// FailoverRecord 故障转移记录.
type FailoverRecord struct {
	ID           string        `json:"id"`
	Timestamp    time.Time     `json:"timestamp"`
	FromNode     string        `json:"from_node"`
	ToNode       string        `json:"to_node"`
	Reason       string        `json:"reason"`
	Success      bool          `json:"success"`
	Duration     time.Duration `json:"duration"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// HAStatus 高可用状态.
type HAStatus struct {
	CurrentRole      string               `json:"current_role"`
	PrimaryNode      string               `json:"primary_node"`
	HealthyNodes     int                  `json:"healthy_nodes"`
	TotalNodes       int                  `json:"total_nodes"`
	LastFailover     time.Time            `json:"last_failover"`
	FailoverCount    int                  `json:"failover_count"`
	IsFailoverActive bool                 `json:"is_failover_active"`
	NodeStates       map[string]NodeState `json:"node_states"`
	Uptime           time.Duration        `json:"uptime"`
	LastHealthCheck  time.Time            `json:"last_health_check"`
}

// HAConfigManager 高可用配置管理器.
type HAConfigManager struct {
	config     *HAConfig
	status     *HAStatus
	nodeStates map[string]*NodeState
	failovers  []FailoverRecord

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *zap.Logger

	// 回调函数
	onRoleChange   func(oldRole, newRole string)
	onFailover     func(record FailoverRecord)
	onHealthChange func(nodeID string, healthy bool)
}

// NewHAConfigManager 创建高可用配置管理器.
func NewHAConfigManager(config *HAConfig, logger *zap.Logger) (*HAConfigManager, error) {
	if config.ClusterName == "" {
		config.ClusterName = "nas-os-cluster"
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 5 * time.Second
	}
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 15 * time.Second
	}
	if config.FailoverTimeout == 0 {
		config.FailoverTimeout = 30 * time.Second
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 10 * time.Second
	}
	if config.DataDir == "" {
		config.DataDir = "/var/lib/nas-os/ha"
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &HAConfigManager{
		config:     config,
		status:     &HAStatus{},
		nodeStates: make(map[string]*NodeState),
		failovers:  make([]FailoverRecord, 0),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
	}

	// 初始化节点状态
	m.initNodeStates()

	return m, nil
}

// initNodeStates 初始化节点状态.
func (m *HAConfigManager) initNodeStates() {
	// 本节点
	m.nodeStates[m.config.NodeID] = &NodeState{
		NodeID:    m.config.NodeID,
		Role:      m.config.NodeRole,
		Status:    HealthStatusUnknown,
		Priority:  m.config.Priority,
		IsHealthy: true,
		Address:   m.config.BindAddress,
		Metadata:  make(map[string]string),
	}

	// 对等节点
	for _, peerID := range m.config.PeerNodes {
		m.nodeStates[peerID] = &NodeState{
			NodeID:    peerID,
			Role:      RoleStandby,
			Status:    HealthStatusUnknown,
			Priority:  50,
			IsHealthy: false,
			Metadata:  make(map[string]string),
		}
	}

	m.status.NodeStates = make(map[string]NodeState)
	for k, v := range m.nodeStates {
		m.status.NodeStates[k] = *v
	}
	m.status.TotalNodes = len(m.nodeStates)
}

// Start 启动高可用管理.
func (m *HAConfigManager) Start() error {
	m.logger.Info("Starting HA config manager",
		zap.String("node_id", m.config.NodeID),
		zap.String("role", m.config.NodeRole),
	)

	// 启动心跳
	if m.config.HeartbeatEnabled {
		m.wg.Add(1)
		go m.heartbeatLoop()
	}

	// 启动健康检查
	if m.config.HealthCheckEnabled {
		m.wg.Add(1)
		go m.healthCheckLoop()
	}

	// 启动数据同步
	if m.config.SyncEnabled && m.config.NodeRole == RolePrimary {
		m.wg.Add(1)
		go m.syncLoop()
	}

	// 启动故障检测
	m.wg.Add(1)
	go m.failureDetectionLoop()

	return nil
}

// Stop 停止高可用管理.
func (m *HAConfigManager) Stop() {
	m.cancel()
	m.wg.Wait()
	m.logger.Info("HA config manager stopped")
}

// heartbeatLoop 心跳循环.
func (m *HAConfigManager) heartbeatLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.sendHeartbeats()
		}
	}
}

// sendHeartbeats 发送心跳.
func (m *HAConfigManager) sendHeartbeats() {
	m.mu.RLock()
	myID := m.config.NodeID
	m.mu.RUnlock()

	for nodeID, state := range m.nodeStates {
		if nodeID == myID {
			continue
		}

		// TODO: 实际发送心跳到对等节点
		_ = state
	}

	// 更新本地节点的心跳时间
	m.mu.Lock()
	if local, ok := m.nodeStates[m.config.NodeID]; ok {
		local.LastHeartbeat = time.Now()
	}
	m.mu.Unlock()
}

// healthCheckLoop 健康检查循环.
func (m *HAConfigManager) healthCheckLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performHealthChecks()
		}
	}
}

// performHealthChecks 执行健康检查.
func (m *HAConfigManager) performHealthChecks() {
	m.mu.RLock()
	myID := m.config.NodeID
	peers := make([]string, 0)
	for id := range m.nodeStates {
		if id != myID {
			peers = append(peers, id)
		}
	}
	m.mu.RUnlock()

	healthyCount := 0
	for _, peerID := range peers {
		healthy := m.checkNodeHealth(peerID)
		m.updateNodeHealth(peerID, healthy)
		if healthy {
			healthyCount++
		}
	}

	// 更新本地节点健康状态
	m.updateNodeHealth(myID, true)

	m.mu.Lock()
	m.status.HealthyNodes = healthyCount + 1 // +1 for local node
	m.status.LastHealthCheck = time.Now()
	m.mu.Unlock()
}

// checkNodeHealth 检查节点健康.
func (m *HAConfigManager) checkNodeHealth(nodeID string) bool {
	// TODO: 实际健康检查逻辑
	// 1. TCP 连接检查
	// 2. API 健康检查
	// 3. 数据同步状态检查
	_ = nodeID
	return true
}

// updateNodeHealth 更新节点健康状态.
func (m *HAConfigManager) updateNodeHealth(nodeID string, healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.nodeStates[nodeID]; ok {
		oldHealthy := state.IsHealthy
		state.IsHealthy = healthy
		state.LastHeartbeat = time.Now()

		if healthy {
			state.Status = HealthStatusHealthy
		} else {
			state.Status = HealthStatusUnhealthy
		}

		// 更新状态映射
		m.status.NodeStates[nodeID] = *state

		// 触发回调
		if oldHealthy != healthy && m.onHealthChange != nil {
			go m.onHealthChange(nodeID, healthy)
		}
	}
}

// syncLoop 数据同步循环.
func (m *HAConfigManager) syncLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.syncData()
		}
	}
}

// syncData 同步数据.
func (m *HAConfigManager) syncData() {
	// TODO: 实际数据同步逻辑
	// 1. 同步配置
	// 2. 同步元数据
	// 3. 同步状态
}

// failureDetectionLoop 故障检测循环.
func (m *HAConfigManager) failureDetectionLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.HeartbeatTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.detectFailures()
		}
	}
}

// detectFailures 检测故障.
func (m *HAConfigManager) detectFailures() {
	m.mu.RLock()
	myRole := m.config.NodeRole
	primaryNode := m.status.PrimaryNode
	m.mu.RUnlock()

	// 如果我是主节点，检查所有从节点
	if myRole == RolePrimary {
		for nodeID, state := range m.nodeStates {
			if nodeID == m.config.NodeID {
				continue
			}
			if time.Since(state.LastHeartbeat) > m.config.HeartbeatTimeout {
				m.logger.Warn("Node heartbeat timeout",
					zap.String("node_id", nodeID),
					zap.Duration("timeout", time.Since(state.LastHeartbeat)),
				)
				m.updateNodeHealth(nodeID, false)
			}
		}
	}

	// 如果我是从节点，检查主节点
	if myRole != RolePrimary {
		if primaryNode != "" {
			if state, ok := m.nodeStates[primaryNode]; ok {
				if time.Since(state.LastHeartbeat) > m.config.HeartbeatTimeout {
					m.logger.Warn("Primary node heartbeat timeout, triggering failover",
						zap.String("primary", primaryNode),
					)
					if m.config.FailoverEnabled {
						if err := m.triggerFailover(primaryNode); err != nil {
							m.logger.Error("触发故障转移失败",
								zap.String("primary", primaryNode),
								zap.Error(err),
							)
						}
					}
				}
			}
		}
	}
}

// triggerFailover 触发故障转移.
func (m *HAConfigManager) triggerFailover(failedNode string) error {
	m.mu.Lock()
	if m.status.IsFailoverActive {
		m.mu.Unlock()
		return ErrFailoverInProgress
	}
	m.status.IsFailoverActive = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.status.IsFailoverActive = false
		m.mu.Unlock()
	}()

	startTime := time.Now()
	record := FailoverRecord{
		ID:        fmt.Sprintf("failover-%d", startTime.UnixNano()),
		Timestamp: startTime,
		FromNode:  failedNode,
		Reason:    "primary node failure",
	}

	// 选择新主节点
	newPrimary := m.selectNewPrimary()
	if newPrimary == "" {
		record.Success = false
		record.ErrorMessage = "no healthy standby available"
		m.recordFailover(record)
		return ErrNoHealthyStandby
	}

	record.ToNode = newPrimary

	// 执行故障转移
	if err := m.executeFailover(failedNode, newPrimary); err != nil {
		record.Success = false
		record.ErrorMessage = err.Error()
		m.recordFailover(record)
		return err
	}

	record.Success = true
	record.Duration = time.Since(startTime)
	m.recordFailover(record)

	// 发送通知
	if m.config.NotifyOnFailover {
		m.sendFailoverNotification(record)
	}

	return nil
}

// selectNewPrimary 选择新主节点.
func (m *HAConfigManager) selectNewPrimary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var bestNode string
	var bestPriority int

	for nodeID, state := range m.nodeStates {
		if !state.IsHealthy {
			continue
		}
		if state.Role == RolePrimary {
			continue
		}
		if bestNode == "" || state.Priority > bestPriority {
			bestNode = nodeID
			bestPriority = state.Priority
		}
	}

	return bestNode
}

// executeFailover 执行故障转移.
func (m *HAConfigManager) executeFailover(fromNode, toNode string) error {
	m.logger.Info("Executing failover",
		zap.String("from", fromNode),
		zap.String("to", toNode),
	)

	// 更新节点角色
	m.mu.Lock()
	oldRole := m.config.NodeRole

	// 更新本地角色（如果我是新主节点）
	if toNode == m.config.NodeID {
		m.config.NodeRole = RolePrimary
		m.status.CurrentRole = RolePrimary
	}

	m.status.PrimaryNode = toNode

	// 更新节点状态
	if state, ok := m.nodeStates[fromNode]; ok {
		state.Role = RoleSecondary
	}
	if state, ok := m.nodeStates[toNode]; ok {
		state.Role = RolePrimary
	}
	m.mu.Unlock()

	// 触发角色变化回调
	if m.onRoleChange != nil {
		go m.onRoleChange(oldRole, m.config.NodeRole)
	}

	m.logger.Info("Failover completed",
		zap.String("new_primary", toNode),
	)

	return nil
}

// recordFailover 记录故障转移.
func (m *HAConfigManager) recordFailover(record FailoverRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.failovers = append(m.failovers, record)
	if len(m.failovers) > 100 {
		m.failovers = m.failovers[len(m.failovers)-100:]
	}

	m.status.LastFailover = record.Timestamp
	m.status.FailoverCount++
}

// sendFailoverNotification 发送故障转移通知.
func (m *HAConfigManager) sendFailoverNotification(record FailoverRecord) {
	// TODO: 发送邮件/ webhook 通知
	_ = record
}

// GetStatus 获取状态.
func (m *HAConfigManager) GetStatus() HAStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := *m.status
	status.CurrentRole = m.config.NodeRole
	status.Uptime = 0 // TODO: 计算运行时间

	return status
}

// GetConfig 获取配置.
func (m *HAConfigManager) GetConfig() HAConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.config
}

// UpdateConfig 更新配置.
func (m *HAConfigManager) UpdateConfig(config *HAConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	m.initNodeStates()

	return m.SaveConfig()
}

// SaveConfig 保存配置.
func (m *HAConfigManager) SaveConfig() error {
	if err := os.MkdirAll(m.config.DataDir, 0750); err != nil {
		return err
	}

	configFile := filepath.Join(m.config.DataDir, "ha_config.json")
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configFile, data, 0600)
}

// LoadConfig 加载配置.
func LoadConfig(path string) (*HAConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}

	var config HAConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// GetFailoverHistory 获取故障转移历史.
func (m *HAConfigManager) GetFailoverHistory(limit int) []FailoverRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.failovers) {
		limit = len(m.failovers)
	}

	start := len(m.failovers) - limit
	if start < 0 {
		start = 0
	}

	result := make([]FailoverRecord, limit)
	copy(result, m.failovers[start:])
	return result
}

// ManualFailover 手动故障转移.
func (m *HAConfigManager) ManualFailover(targetNode string) error {
	m.mu.RLock()
	if m.config.NodeRole != RolePrimary {
		m.mu.RUnlock()
		return ErrNotPrimary
	}
	m.mu.RUnlock()

	return m.triggerFailover(m.config.NodeID)
}

// SetOnRoleChange 设置角色变化回调.
func (m *HAConfigManager) SetOnRoleChange(fn func(oldRole, newRole string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRoleChange = fn
}

// SetOnFailover 设置故障转移回调.
func (m *HAConfigManager) SetOnFailover(fn func(record FailoverRecord)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onFailover = fn
}

// SetOnHealthChange 设置健康状态变化回调.
func (m *HAConfigManager) SetOnHealthChange(fn func(nodeID string, healthy bool)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onHealthChange = fn
}

// IsPrimary 是否是主节点.
func (m *HAConfigManager) IsPrimary() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.NodeRole == RolePrimary
}

// GetPrimaryNode 获取主节点.
func (m *HAConfigManager) GetPrimaryNode() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status.PrimaryNode
}

// GetNodeState 获取节点状态.
func (m *HAConfigManager) GetNodeState(nodeID string) (NodeState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.nodeStates[nodeID]
	if !exists {
		return NodeState{}, false
	}
	return *state, true
}

// GetAllNodeStates 获取所有节点状态.
func (m *HAConfigManager) GetAllNodeStates() map[string]NodeState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make(map[string]NodeState)
	for k, v := range m.nodeStates {
		states[k] = *v
	}
	return states
}
