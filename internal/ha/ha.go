// Package ha 提供高可用管理核心功能
// 实现企业级 Active-Passive 集群、心跳检测、自动故障转移
// 参考: TrueNAS 26 SMB Stateful Failover, Synology High Availability
package ha

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

// 错误定义
var (
	ErrNotLeader          = errors.New("current node is not leader")
	ErrNodeNotFound       = errors.New("node not found")
	ErrFailoverInProgress = errors.New("failover already in progress")
	ErrClusterNotReady    = errors.New("cluster not ready")
	ErrSyncFailed         = errors.New("state sync failed")
	ErrQuorumLost         = errors.New("quorum lost")
	ErrSplitBrain         = errors.New("split brain detected")
	ErrInvalidState       = errors.New("invalid state for operation")
)

// HAState 高可用状态
type HAState string

const (
	HAStateActive   HAState = "active"   // 活跃节点（主节点）
	HAStatePassive  HAState = "passive"  // 被动节点（备节点）
	HAStateStandby  HAState = "standby"  // 待机状态
	HAStateFailed   HAState = "failed"   // 故障状态
	HAStateSyncing  HAState = "syncing"  // 同步状态
	HAStateTakeover HAState = "takeover" // 接管状态
	HAStateUnknown  HAState = "unknown"  // 未知状态
)

// HARole 高可用角色
type HARole string

const (
	HARolePrimary   HARole = "primary"   // 主节点
	HARoleSecondary HARole = "secondary" // 备节点
	HARoleNone      HARole = "none"      // 无角色
)

// NodeHAInfo 节点HA信息
type NodeHAInfo struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Address       string            `json:"address"`
	Port          int               `json:"port"`
	Role          HARole            `json:"role"`
	State         HAState           `json:"state"`
	Priority      int               `json:"priority"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	LastSync      time.Time         `json:"last_sync"`
	HealthScore   float64           `json:"health_score"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// HAConfig 高可用配置
type HAConfig struct {
	// 集群配置
	ClusterName string     `json:"cluster_name"`
	NodeID      string     `json:"node_id"`
	NodeName    string     `json:"node_name"`
	Address     string     `json:"address"`
	Port        int        `json:"port"`
	Peers       []PeerNode `json:"peers"`

	// 心跳配置
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	HeartbeatTimeout  time.Duration `json:"heartbeat_timeout"`
	HeartbeatMissMax  int           `json:"heartbeat_miss_max"`

	// 故障转移配置
	FailoverEnabled      bool          `json:"failover_enabled"`
	FailoverDelay        time.Duration `json:"failover_delay"`
	FailoverConfirmation time.Duration `json:"failover_confirmation"`
	AutoFallback         bool          `json:"auto_fallback"`
	FallbackDelay        time.Duration `json:"fallback_delay"`

	// 同步配置
	SyncInterval time.Duration `json:"sync_interval"`
	SyncTimeout  time.Duration `json:"sync_timeout"`
	SyncRetryMax int           `json:"sync_retry_max"`

	// 选举配置
	ElectionTimeout time.Duration `json:"election_timeout"`
	PriorityPolicy  string        `json:"priority_policy"` // priority, round-robin, random

	// 脑裂防护
	QuorumRequired  int  `json:"quorum_required"`
	SplitBrainCheck bool `json:"split_brain_check"`

	// 存储路径
	DataDir string `json:"data_dir"`
}

// PeerNode 对等节点配置
type PeerNode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Priority int    `json:"priority"`
}

// HAStatus 高可用状态
type HAStatus struct {
	LocalNode      *NodeHAInfo   `json:"local_node"`
	PrimaryNode    *NodeHAInfo   `json:"primary_node"`
	SecondaryNodes []*NodeHAInfo `json:"secondary_nodes"`
	ClusterState   string        `json:"cluster_state"`
	LastFailover   time.Time     `json:"last_failover"`
	FailoverCount  int           `json:"failover_count"`
	SyncProgress   float64       `json:"sync_progress"`
	Uptime         time.Duration `json:"uptime"`
	IsHealthy      bool          `json:"is_healthy"`
	QuorumStatus   string        `json:"quorum_status"`
}

// HAEvent 高可用事件
type HAEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	NodeID    string    `json:"node_id"`
	OldState  HAState   `json:"old_state"`
	NewState  HAState   `json:"new_state"`
	OldRole   HARole    `json:"old_role"`
	NewRole   HARole    `json:"new_role"`
	Reason    string    `json:"reason"`
	Message   string    `json:"message"`
	Duration  string    `json:"duration,omitempty"`
}

// HAEventType 事件类型
type HAEventType string

const (
	HAEventHeartbeatMissed  HAEventType = "heartbeat_missed"
	HAEventHeartbeatResume  HAEventType = "heartbeat_resume"
	HAEventNodeFailed       HAEventType = "node_failed"
	HAEventNodeRecovered    HAEventType = "node_recovered"
	HAEventFailoverStarted  HAEventType = "failover_started"
	HAEventFailoverComplete HAEventType = "failover_complete"
	HAEventFailoverFailed   HAEventType = "failover_failed"
	HAEventRoleChange       HAEventType = "role_change"
	HAEventStateChange      HAEventType = "state_change"
	HAEventSyncComplete     HAEventType = "sync_complete"
	HAEventSyncFailed       HAEventType = "sync_failed"
	HAEventSplitBrain       HAEventType = "split_brain"
	HAEventQuorumLost       HAEventType = "quorum_lost"
	HAEventQuorumRestore    HAEventType = "quorum_restore"
)

// HAManager 高可用管理器
type HAManager struct {
	config    *HAConfig
	localNode *NodeHAInfo
	nodes     map[string]*NodeHAInfo
	primary   *NodeHAInfo

	// 心跳管理
	heartbeatMgr *HeartbeatManager

	// 状态同步
	stateSyncer *StateSyncer

	// 故障转移
	failoverMgr *FailoverController

	// 脑裂防护
	splitBrainGuard *SplitBrainGuard

	// 事件管理
	events        []HAEvent
	eventChan     chan HAEvent
	eventHandlers []HAEventHandler

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
	logger *zap.Logger
}

// HAEventHandler 事件处理器接口
type HAEventHandler interface {
	OnHAEvent(event HAEvent)
}

// NewHAManager 创建高可用管理器
func NewHAManager(config *HAConfig, logger *zap.Logger) (*HAManager, error) {
	if err := ValidateHAConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 设置默认值
	config = ApplyHADefaults(config)

	mgr := &HAManager{
		config:    config,
		nodes:     make(map[string]*NodeHAInfo),
		events:    make([]HAEvent, 0, 100),
		eventChan: make(chan HAEvent, 50),
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger,
	}

	// 初始化本地节点
	mgr.localNode = &NodeHAInfo{
		ID:            config.NodeID,
		Name:          config.NodeName,
		Address:       config.Address,
		Port:          config.Port,
		Role:          HARoleNone,
		State:         HAStateUnknown,
		Priority:      100,
		HealthScore:   100.0,
		LastHeartbeat: time.Now(),
	}
	mgr.nodes[config.NodeID] = mgr.localNode

	// 初始化对等节点
	for _, peer := range config.Peers {
		mgr.nodes[peer.ID] = &NodeHAInfo{
			ID:          peer.ID,
			Name:        peer.Name,
			Address:     peer.Address,
			Port:        peer.Port,
			Role:        HARoleNone,
			State:       HAStateStandby,
			Priority:    peer.Priority,
			HealthScore: 100.0,
		}
	}

	// 初始化子组件
	mgr.heartbeatMgr = NewHeartbeatManager(config, logger)
	mgr.stateSyncer = NewStateSyncer(config, logger)
	mgr.failoverMgr = NewFailoverController(mgr, config, logger)
	mgr.splitBrainGuard = NewSplitBrainGuard(config, logger)

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0750); err != nil {
		cancel()
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// 加载持久化状态
	_ = mgr.loadState()

	return mgr, nil
}

// ValidateHAConfig 验证配置
func ValidateHAConfig(config *HAConfig) error {
	if config.NodeID == "" {
		return errors.New("node_id required")
	}
	if config.Address == "" {
		return errors.New("address required")
	}
	if len(config.Peers) == 0 {
		return errors.New("peers required for HA cluster")
	}
	return nil
}

// ApplyHADefaults 应用默认配置
func ApplyHADefaults(config *HAConfig) *HAConfig {
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 3 * time.Second
	}
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 10 * time.Second
	}
	if config.HeartbeatMissMax == 0 {
		config.HeartbeatMissMax = 3
	}
	if config.FailoverDelay == 0 {
		config.FailoverDelay = 5 * time.Second
	}
	if config.FailoverConfirmation == 0 {
		config.FailoverConfirmation = 10 * time.Second
	}
	if config.FallbackDelay == 0 {
		config.FallbackDelay = 30 * time.Second
	}
	if config.SyncInterval == 0 {
		config.SyncInterval = 5 * time.Second
	}
	if config.SyncTimeout == 0 {
		config.SyncTimeout = 30 * time.Second
	}
	if config.ElectionTimeout == 0 {
		config.ElectionTimeout = 10 * time.Second
	}
	if config.QuorumRequired == 0 {
		config.QuorumRequired = len(config.Peers)/2 + 1
	}
	if config.DataDir == "" {
		config.DataDir = "/var/lib/nas-os/ha"
	}
	return config
}

// Start 启动高可用管理器
func (mgr *HAManager) Start() error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.logger.Info("HA manager starting",
		zap.String("node_id", mgr.config.NodeID),
		zap.String("cluster", mgr.config.ClusterName),
	)

	// 启动心跳管理
	if err := mgr.heartbeatMgr.Start(mgr.ctx); err != nil {
		return fmt.Errorf("start heartbeat: %w", err)
	}

	// 启动状态同步
	if err := mgr.stateSyncer.Start(mgr.ctx); err != nil {
		return fmt.Errorf("start state syncer: %w", err)
	}

	// 启动故障转移控制器
	if err := mgr.failoverMgr.Start(mgr.ctx); err != nil {
		return fmt.Errorf("start failover controller: %w", err)
	}

	// 启动事件处理器
	mgr.wg.Add(1)
	go mgr.eventLoop()

	// 启动状态监控
	mgr.wg.Add(1)
	go mgr.stateMonitorLoop()

	// 启动脑裂检测
	if mgr.config.SplitBrainCheck {
		mgr.wg.Add(1)
		go mgr.splitBrainCheckLoop()
	}

	// 初始角色选举
	mgr.performInitialElection()

	mgr.logger.Info("HA manager started successfully")
	return nil
}

// Stop 停止高可用管理器
func (mgr *HAManager) Stop() error {
	mgr.logger.Info("HA manager stopping")

	mgr.cancel()
	mgr.wg.Wait()

	// 停止子组件
	mgr.heartbeatMgr.Stop()
	mgr.stateSyncer.Stop()
	mgr.failoverMgr.Stop()

	// 保存状态
	_ = mgr.saveState()

	mgr.logger.Info("HA manager stopped")
	return nil
}

// performInitialElection 执行初始选举
func (mgr *HAManager) performInitialElection() {
	// 收集所有活跃节点
	var activeNodes []*NodeHAInfo
	for _, node := range mgr.nodes {
		activeNodes = append(activeNodes, node)
	}

	// 选择优先级最高的作为主节点
	var primary *NodeHAInfo
	for _, node := range activeNodes {
		if primary == nil || node.Priority > primary.Priority {
			primary = node
		}
	}

	if primary == nil {
		mgr.logger.Error("No nodes available for election")
		return
	}

	// 设置角色
	for id, node := range mgr.nodes {
		if id == primary.ID {
			node.Role = HARolePrimary
			node.State = HAStateActive
		} else {
			node.Role = HARoleSecondary
			node.State = HAStatePassive
		}
	}

	mgr.primary = primary

	mgr.logger.Info("Initial election completed",
		zap.String("primary", primary.ID),
	)

	// 记录事件
	mgr.recordEvent(HAEvent{
		ID:        fmt.Sprintf("election-%d", time.Now().UnixNano()),
		Type:      string(HAEventRoleChange),
		Timestamp: time.Now(),
		NodeID:    primary.ID,
		NewRole:   HARolePrimary,
		Reason:    "initial election",
	})
}

// eventLoop 事件处理循环
func (mgr *HAManager) eventLoop() {
	defer mgr.wg.Done()

	for {
		select {
		case <-mgr.ctx.Done():
			return
		case event := <-mgr.eventChan:
			mgr.handleEvent(event)
		}
	}
}

// handleEvent 处理事件
func (mgr *HAManager) handleEvent(event HAEvent) {
	// 保存事件
	mgr.mu.Lock()
	mgr.events = append(mgr.events, event)
	if len(mgr.events) > 100 {
		mgr.events = mgr.events[len(mgr.events)-100:]
	}
	mgr.mu.Unlock()

	// 通知处理器
	for _, handler := range mgr.eventHandlers {
		go handler.OnHAEvent(event)
	}

	mgr.logger.Info("HA event",
		zap.String("type", event.Type),
		zap.String("node", event.NodeID),
		zap.String("reason", event.Reason),
	)
}

// stateMonitorLoop 状态监控循环
func (mgr *HAManager) stateMonitorLoop() {
	defer mgr.wg.Done()

	ticker := time.NewTicker(mgr.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mgr.ctx.Done():
			return
		case <-ticker.C:
			mgr.checkNodesState()
		}
	}
}

// checkNodesState 检查节点状态
func (mgr *HAManager) checkNodesState() {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	now := time.Now()

	for id, node := range mgr.nodes {
		if id == mgr.config.NodeID {
			continue // 跳过本地节点
		}

		elapsed := now.Sub(node.LastHeartbeat)

		// 检查心跳超时
		if elapsed > mgr.config.HeartbeatTimeout {
			missed := int(elapsed / mgr.config.HeartbeatInterval)

			if missed >= mgr.config.HeartbeatMissMax {
				// 标记节点故障
				if node.State != HAStateFailed {
					oldState := node.State
					node.State = HAStateFailed
					node.HealthScore = 0

					mgr.sendEvent(HAEvent{
						ID:        fmt.Sprintf("fail-%d", time.Now().UnixNano()),
						Type:      string(HAEventNodeFailed),
						Timestamp: now,
						NodeID:    id,
						OldState:  oldState,
						NewState:  HAStateFailed,
						Reason:    fmt.Sprintf("heartbeat timeout: %d misses", missed),
					})

					// 如果主节点故障，触发故障转移
					if node.Role == HARolePrimary {
						mgr.failoverMgr.TriggerFailover(node)
					}
				}
			} else {
				// 心跳丢失警告
				mgr.sendEvent(HAEvent{
					ID:        fmt.Sprintf("miss-%d", time.Now().UnixNano()),
					Type:      string(HAEventHeartbeatMissed),
					Timestamp: now,
					NodeID:    id,
					Reason:    fmt.Sprintf("heartbeat miss: %d", missed),
				})
			}
		}
	}
}

// splitBrainCheckLoop 脑裂检测循环
func (mgr *HAManager) splitBrainCheckLoop() {
	defer mgr.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-mgr.ctx.Done():
			return
		case <-ticker.C:
			mgr.checkSplitBrain()
		}
	}
}

// checkSplitBrain 检查脑裂
func (mgr *HAManager) checkSplitBrain() {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	// 统计活跃节点
	activeCount := 0
	primaryCount := 0

	for _, node := range mgr.nodes {
		if node.State == HAStateActive || node.State == HAStatePassive {
			activeCount++
		}
		if node.Role == HARolePrimary {
			primaryCount++
		}
	}

	// 检查是否满足法定人数
	if activeCount < mgr.config.QuorumRequired {
		mgr.sendEvent(HAEvent{
			ID:        fmt.Sprintf("quorum-%d", time.Now().UnixNano()),
			Type:      string(HAEventQuorumLost),
			Timestamp: time.Now(),
			Reason:    fmt.Sprintf("active nodes %d < quorum %d", activeCount, mgr.config.QuorumRequired),
		})
	}

	// 检查是否存在多个主节点（脑裂）
	if primaryCount > 1 {
		mgr.sendEvent(HAEvent{
			ID:        fmt.Sprintf("splitbrain-%d", time.Now().UnixNano()),
			Type:      string(HAEventSplitBrain),
			Timestamp: time.Now(),
			Reason:    fmt.Sprintf("multiple primaries: %d", primaryCount),
		})

		// 触发脑裂处理
		mgr.splitBrainGuard.HandleSplitBrain(mgr.nodes)
	}
}

// sendEvent 发送事件
func (mgr *HAManager) sendEvent(event HAEvent) {
	select {
	case mgr.eventChan <- event:
	default:
		mgr.logger.Warn("Event channel full, dropping event")
	}
}

// recordEvent 记录事件
func (mgr *HAManager) recordEvent(event HAEvent) {
	mgr.mu.Lock()
	mgr.events = append(mgr.events, event)
	if len(mgr.events) > 100 {
		mgr.events = mgr.events[len(mgr.events)-100:]
	}
	mgr.mu.Unlock()
}

// IsPrimary 检查是否是主节点
func (mgr *HAManager) IsPrimary() bool {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.localNode.Role == HARolePrimary
}

// GetPrimary 获取主节点
func (mgr *HAManager) GetPrimary() *NodeHAInfo {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.primary
}

// GetStatus 获取HA状态
func (mgr *HAManager) GetStatus() *HAStatus {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	var secondary []*NodeHAInfo
	for _, node := range mgr.nodes {
		if node.Role == HARoleSecondary {
			secondary = append(secondary, node)
		}
	}

	return &HAStatus{
		LocalNode:      mgr.localNode,
		PrimaryNode:    mgr.primary,
		SecondaryNodes: secondary,
		ClusterState:   string(mgr.localNode.State),
		LastFailover:   mgr.failoverMgr.LastFailover(),
		FailoverCount:  mgr.failoverMgr.FailoverCount(),
		SyncProgress:   mgr.stateSyncer.Progress(),
		IsHealthy:      mgr.localNode.HealthScore >= 50,
		QuorumStatus:   mgr.getQuorumStatus(),
	}
}

// getQuorumStatus 获取法定人数状态
func (mgr *HAManager) getQuorumStatus() string {
	activeCount := 0
	for _, node := range mgr.nodes {
		if node.State == HAStateActive || node.State == HAStatePassive {
			activeCount++
		}
	}

	if activeCount >= mgr.config.QuorumRequired {
		return "healthy"
	}
	return "degraded"
}

// GetNodes 获取所有节点
func (mgr *HAManager) GetNodes() []*NodeHAInfo {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	nodes := make([]*NodeHAInfo, 0, len(mgr.nodes))
	for _, node := range mgr.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetEvents 获取事件历史
func (mgr *HAManager) GetEvents(limit int) []HAEvent {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if limit <= 0 || limit > len(mgr.events) {
		limit = len(mgr.events)
	}

	start := len(mgr.events) - limit
	if start < 0 {
		start = 0
	}

	return mgr.events[start:]
}

// RegisterEventHandler 注册事件处理器
func (mgr *HAManager) RegisterEventHandler(handler HAEventHandler) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.eventHandlers = append(mgr.eventHandlers, handler)
}

// ManualFailover 手动故障转移
func (mgr *HAManager) ManualFailover(targetNodeID string) error {
	mgr.mu.RLock()
	if mgr.localNode.Role != HARolePrimary {
		mgr.mu.RUnlock()
		return ErrNotLeader
	}
	mgr.mu.RUnlock()

	target, exists := mgr.nodes[targetNodeID]
	if !exists {
		return ErrNodeNotFound
	}

	if target.State != HAStatePassive && target.State != HAStateActive {
		return ErrInvalidState
	}

	return mgr.failoverMgr.ExecuteManualFailover(target)
}

// UpdateNodeHeartbeat 更新节点心跳
func (mgr *HAManager) UpdateNodeHeartbeat(nodeID string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	node, exists := mgr.nodes[nodeID]
	if !exists {
		return
	}

	oldState := node.State
	node.LastHeartbeat = time.Now()
	node.HealthScore = 100.0

	if node.State == HAStateFailed || node.State == HAStateUnknown {
		node.State = HAStatePassive

		mgr.sendEvent(HAEvent{
			ID:        fmt.Sprintf("recover-%d", time.Now().UnixNano()),
			Type:      string(HAEventNodeRecovered),
			Timestamp: time.Now(),
			NodeID:    nodeID,
			OldState:  oldState,
			NewState:  node.State,
			Reason:    "heartbeat received",
		})
	}
}

// 持久化
func (mgr *HAManager) saveState() error {
	state := map[string]interface{}{
		"local_role":    mgr.localNode.Role,
		"local_state":   mgr.localNode.State,
		"primary_id":    mgr.primary.ID,
		"events":        mgr.events,
		"last_failover": mgr.failoverMgr.LastFailover(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	stateFile := filepath.Join(mgr.config.DataDir, "ha_state.json")
	return os.WriteFile(stateFile, data, 0600)
}

func (mgr *HAManager) loadState() error {
	stateFile := filepath.Join(mgr.config.DataDir, "ha_state.json")

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	if role, ok := state["local_role"].(string); ok {
		mgr.localNode.Role = HARole(role)
	}
	if st, ok := state["local_state"].(string); ok {
		mgr.localNode.State = HAState(st)
	}

	return nil
}
