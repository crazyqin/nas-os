package cluster

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FailoverManager 故障转移管理器
type FailoverManager struct {
	cluster       *Cluster
	config        *FailoverConfig
	state         *FailoverState
	strategies    map[string]FailoverStrategy
	eventHandlers []FailoverEventHandler
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	logger        *zap.Logger
}

// FailoverConfig 故障转移配置
type FailoverConfig struct {
	// 自动故障转移开关
	AutoFailoverEnabled bool `json:"auto_failover_enabled"`

	// 故障确认时间（等待多久确认节点故障）
	FailoverConfirmationTime time.Duration `json:"failover_confirmation_time"`

	// 最大故障转移尝试次数
	MaxFailoverAttempts int `json:"max_failover_attempts"`

	// 故障转移间隔
	FailoverInterval time.Duration `json:"failover_interval"`

	// 优先级策略：priority（优先级）、random（随机）、round-robin（轮询）
	PriorityPolicy string `json:"priority_policy"`

	// 预热时间（新领导者预热期）
	WarmupTime time.Duration `json:"warmup_time"`

	// 回切开关
	AutomaticFallbackEnabled bool `json:"automatic_fallback_enabled"`

	// 回切等待时间
	FallbackDelay time.Duration `json:"fallback_delay"`
}

// FailoverState 故障转移状态
type FailoverState struct {
	InProgress       bool            `json:"in_progress"`
	CurrentAttempt   int             `json:"current_attempt"`
	LastFailover     time.Time       `json:"last_failover"`
	FailedNode       *Node           `json:"failed_node,omitempty"`
	NewLeader        *Node           `json:"new_leader,omitempty"`
	StatusHistory    []FailoverEvent `json:"status_history"`
	LastStatusChange time.Time       `json:"last_status_change"`
}

// FailoverEvent 故障转移事件
type FailoverEvent struct {
	Timestamp time.Time     `json:"timestamp"`
	Type      string        `json:"type"` // "started", "completed", "failed", "fallback"
	FromNode  string        `json:"from_node,omitempty"`
	ToNode    string        `json:"to_node,omitempty"`
	Reason    string        `json:"reason,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// FailoverStrategy 故障转移策略接口
type FailoverStrategy interface {
	Name() string
	SelectNewLeader(nodes []*Node, failedNode *Node) (*Node, error)
	PreFailover(failedNode *Node) error
	PostFailover(newLeader *Node) error
}

// FailoverEventHandler 故障转移事件处理器
type FailoverEventHandler interface {
	OnFailoverStarted(event FailoverEvent)
	OnFailoverCompleted(event FailoverEvent)
	OnFailoverFailed(event FailoverEvent)
	OnFallback(event FailoverEvent)
}

// DefaultFailoverStrategy 默认故障转移策略
type DefaultFailoverStrategy struct {
	policy string
}

// NewDefaultFailoverStrategy 创建默认策略
func NewDefaultFailoverStrategy(policy string) *DefaultFailoverStrategy {
	return &DefaultFailoverStrategy{policy: policy}
}

// Name 策略名称
func (s *DefaultFailoverStrategy) Name() string {
	return "default"
}

// SelectNewLeader 选择新领导者
func (s *DefaultFailoverStrategy) SelectNewLeader(nodes []*Node, failedNode *Node) (*Node, error) {
	var candidates []*Node

	// 筛选可用节点
	for _, node := range nodes {
		if node.ID == failedNode.ID {
			continue
		}
		if node.State == NodeStateActive {
			candidates = append(candidates, node)
		}
	}

	if len(candidates) == 0 {
		return nil, ErrClusterNotReady
	}

	// 根据策略选择
	switch s.policy {
	case "priority":
		// 选择优先级最高的
		var best *Node
		for _, node := range candidates {
			if best == nil || node.Priority > best.Priority {
				best = node
			}
		}
		return best, nil

	case "random":
		// 随机选择
		return candidates[0], nil

	case "round-robin":
		// 轮询（简化实现，选择第一个）
		return candidates[0], nil

	default:
		return candidates[0], nil
	}
}

// PreFailover 故障转移前
func (s *DefaultFailoverStrategy) PreFailover(failedNode *Node) error {
	return nil
}

// PostFailover 故障转移后
func (s *DefaultFailoverStrategy) PostFailover(newLeader *Node) error {
	return nil
}

// NewFailoverManager 创建故障转移管理器
func NewFailoverManager(cluster *Cluster, config *FailoverConfig, logger *zap.Logger) *FailoverManager {
	ctx, cancel := context.WithCancel(context.Background())

	fm := &FailoverManager{
		cluster: cluster,
		config:  config,
		state: &FailoverState{
			StatusHistory: make([]FailoverEvent, 0),
		},
		strategies: make(map[string]FailoverStrategy),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
	}

	// 注册默认策略
	fm.strategies["default"] = NewDefaultFailoverStrategy(config.PriorityPolicy)

	return fm
}

// Start 启动故障转移管理器
func (fm *FailoverManager) Start() error {
	fm.wg.Add(1)
	go fm.monitorLoop()

	fm.logger.Info("Failover manager started",
		zap.Bool("auto_failover", fm.config.AutoFailoverEnabled),
	)
	return nil
}

// Stop 停止故障转移管理器
func (fm *FailoverManager) Stop() {
	fm.cancel()
	fm.wg.Wait()
	fm.logger.Info("Failover manager stopped")
}

// monitorLoop 监控循环
func (fm *FailoverManager) monitorLoop() {
	defer fm.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-fm.ctx.Done():
			return
		case <-ticker.C:
			fm.checkAndFailover()
		}
	}
}

// checkAndFailover 检查并执行故障转移
func (fm *FailoverManager) checkAndFailover() {
	if !fm.config.AutoFailoverEnabled {
		return
	}

	fm.mu.RLock()
	if fm.state.InProgress {
		fm.mu.RUnlock()
		return
	}
	fm.mu.RUnlock()

	// 检查领导者状态
	leader, err := fm.cluster.GetLeader()
	if err != nil {
		// 没有领导者，触发选举
		fm.logger.Info("No leader available, triggering election")
		fm.triggerElection()
		return
	}

	// 检查领导者是否故障
	if leader.State == NodeStateFailed || leader.State == NodeStateInactive {
		fm.logger.Warn("Leader failure detected",
			zap.String("leader_id", leader.ID),
			zap.String("state", string(leader.State)),
		)
		_ = fm.executeFailover(leader)
	}
}

// executeFailover 执行故障转移
func (fm *FailoverManager) executeFailover(failedNode *Node) error {
	fm.mu.Lock()

	// 检查是否已经在进行中
	if fm.state.InProgress {
		fm.mu.Unlock()
		return nil
	}

	// 标记开始
	fm.state.InProgress = true
	fm.state.FailedNode = failedNode
	fm.state.LastFailover = time.Now()
	fm.state.LastStatusChange = time.Now()

	event := FailoverEvent{
		Timestamp: time.Now(),
		Type:      "started",
		FromNode:  failedNode.ID,
		Reason:    "node failure detected",
	}
	fm.state.StatusHistory = append(fm.state.StatusHistory, event)

	fm.mu.Unlock()

	// 通知事件处理器
	for _, handler := range fm.eventHandlers {
		handler.OnFailoverStarted(event)
	}

	// 获取策略
	strategy := fm.strategies["default"]

	// 预处理
	if err := strategy.PreFailover(failedNode); err != nil {
		fm.handleFailoverFailure(err)
		return err
	}

	// 确认故障（等待一段时间）
	time.Sleep(fm.config.FailoverConfirmationTime)

	// 重新检查节点状态
	node, err := fm.cluster.GetNode(failedNode.ID)
	if err == nil && node.State == NodeStateActive {
		// 节点恢复，取消故障转移
		fm.logger.Info("Node recovered, canceling failover",
			zap.String("node_id", failedNode.ID),
		)
		fm.mu.Lock()
		fm.state.InProgress = false
		fm.state.FailedNode = nil
		fm.mu.Unlock()
		return nil
	}

	// 选择新领导者
	nodes := fm.cluster.GetNodes()
	newLeader, err := strategy.SelectNewLeader(nodes, failedNode)
	if err != nil {
		fm.handleFailoverFailure(err)
		return err
	}

	// 执行故障转移
	fm.mu.Lock()
	fm.state.NewLeader = newLeader
	fm.state.CurrentAttempt++
	fm.mu.Unlock()

	// 更新集群状态
	if err := fm.promoteToLeader(newLeader); err != nil {
		fm.handleFailoverFailure(err)
		return err
	}

	// 后处理
	if err := strategy.PostFailover(newLeader); err != nil {
		fm.logger.Warn("Post-failover hook failed", zap.Error(err))
	}

	// 标记完成
	fm.mu.Lock()
	fm.state.InProgress = false
	duration := time.Since(fm.state.LastFailover)

	completedEvent := FailoverEvent{
		Timestamp: time.Now(),
		Type:      "completed",
		FromNode:  failedNode.ID,
		ToNode:    newLeader.ID,
		Duration:  duration,
	}
	fm.state.StatusHistory = append(fm.state.StatusHistory, completedEvent)
	fm.mu.Unlock()

	// 通知事件处理器
	for _, handler := range fm.eventHandlers {
		handler.OnFailoverCompleted(completedEvent)
	}

	fm.logger.Info("Failover completed",
		zap.String("from", failedNode.ID),
		zap.String("to", newLeader.ID),
		zap.Duration("duration", duration),
	)

	return nil
}

// promoteToLeader 提升为领导者
func (fm *FailoverManager) promoteToLeader(node *Node) error {
	// 这里应该调用集群的选举逻辑
	node.Role = NodeRoleLeader
	fm.cluster.leader = node

	fm.logger.Info("Node promoted to leader",
		zap.String("node_id", node.ID),
	)

	return nil
}

// handleFailoverFailure 处理故障转移失败
func (fm *FailoverManager) handleFailoverFailure(err error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.state.InProgress = false
	fm.state.CurrentAttempt++

	event := FailoverEvent{
		Timestamp: time.Now(),
		Type:      "failed",
		Reason:    err.Error(),
	}
	fm.state.StatusHistory = append(fm.state.StatusHistory, event)

	// 通知事件处理器
	for _, handler := range fm.eventHandlers {
		handler.OnFailoverFailed(event)
	}

	fm.logger.Error("Failover failed",
		zap.Error(err),
		zap.Int("attempt", fm.state.CurrentAttempt),
	)
}

// triggerElection 触发选举
func (fm *FailoverManager) triggerElection() {
	// 获取活跃节点
	nodes := fm.cluster.GetNodes()
	var activeNodes []*Node
	for _, node := range nodes {
		if node.State == NodeStateActive {
			activeNodes = append(activeNodes, node)
		}
	}

	if len(activeNodes) == 0 {
		fm.logger.Error("No active nodes for election")
		return
	}

	// 使用策略选择领导者
	strategy := fm.strategies["default"]
	newLeader, err := strategy.SelectNewLeader(activeNodes, &Node{})
	if err != nil {
		fm.logger.Error("Failed to select leader", zap.Error(err))
		return
	}

	// 提升为领导者
	if err := fm.promoteToLeader(newLeader); err != nil {
		fm.logger.Error("Failed to promote leader", zap.Error(err))
		return
	}

	fm.logger.Info("New leader elected",
		zap.String("leader_id", newLeader.ID),
	)
}

// RegisterStrategy 注册策略
func (fm *FailoverManager) RegisterStrategy(name string, strategy FailoverStrategy) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.strategies[name] = strategy
}

// RegisterEventHandler 注册事件处理器
func (fm *FailoverManager) RegisterEventHandler(handler FailoverEventHandler) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.eventHandlers = append(fm.eventHandlers, handler)
}

// GetState 获取故障转移状态
func (fm *FailoverManager) GetState() FailoverState {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	return FailoverState{
		InProgress:       fm.state.InProgress,
		CurrentAttempt:   fm.state.CurrentAttempt,
		LastFailover:     fm.state.LastFailover,
		LastStatusChange: fm.state.LastStatusChange,
	}
}

// GetHistory 获取故障转移历史
func (fm *FailoverManager) GetHistory(limit int) []FailoverEvent {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if limit <= 0 || limit > len(fm.state.StatusHistory) {
		limit = len(fm.state.StatusHistory)
	}

	start := len(fm.state.StatusHistory) - limit
	if start < 0 {
		start = 0
	}

	result := make([]FailoverEvent, limit)
	copy(result, fm.state.StatusHistory[start:])
	return result
}

// ManualFailover 手动故障转移
func (fm *FailoverManager) ManualFailover(targetNodeID string) error {
	fm.mu.Lock()
	if fm.state.InProgress {
		fm.mu.Unlock()
		return errors.New("failover already in progress")
	}
	fm.mu.Unlock()

	// 获取当前领导者
	leader, err := fm.cluster.GetLeader()
	if err != nil {
		return err
	}

	// 获取目标节点
	targetNode, err := fm.cluster.GetNode(targetNodeID)
	if err != nil {
		return err
	}

	// 检查目标节点状态
	if targetNode.State != NodeStateActive {
		return errors.New("target node is not active")
	}

	fm.logger.Info("Manual failover initiated",
		zap.String("from", leader.ID),
		zap.String("to", targetNodeID),
	)

	// 执行故障转移
	return fm.executeFailover(leader)
}

// CancelFailover 取消故障转移
func (fm *FailoverManager) CancelFailover() error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if !fm.state.InProgress {
		return errors.New("no failover in progress")
	}

	fm.state.InProgress = false
	fm.state.FailedNode = nil
	fm.state.NewLeader = nil

	fm.logger.Info("Failover canceled")
	return nil
}

// FailoverStats 故障转移统计
type FailoverStats struct {
	AutoFailoverEnabled      bool          `json:"auto_failover_enabled"`
	InProgress               bool          `json:"in_progress"`
	CurrentAttempt           int           `json:"current_attempt"`
	TotalFailovers           int           `json:"total_failovers"`
	FailedAttempts           int           `json:"failed_attempts"`
	LastFailover             time.Time     `json:"last_failover"`
	AverageFailoverTime      time.Duration `json:"average_failover_time"`
	AutomaticFallbackEnabled bool          `json:"automatic_fallback_enabled"`
}

// GetStats 获取统计
func (fm *FailoverManager) GetStats() FailoverStats {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	var totalFailoverTime time.Duration
	var failoverCount int
	var failedCount int

	for _, event := range fm.state.StatusHistory {
		switch event.Type {
		case "completed":
			failoverCount++
			totalFailoverTime += event.Duration
		case "failed":
			failedCount++
		}
	}

	var avgTime time.Duration
	if failoverCount > 0 {
		avgTime = totalFailoverTime / time.Duration(failoverCount)
	}

	return FailoverStats{
		AutoFailoverEnabled:      fm.config.AutoFailoverEnabled,
		InProgress:               fm.state.InProgress,
		CurrentAttempt:           fm.state.CurrentAttempt,
		TotalFailovers:           failoverCount,
		FailedAttempts:           failedCount,
		LastFailover:             fm.state.LastFailover,
		AverageFailoverTime:      avgTime,
		AutomaticFallbackEnabled: fm.config.AutomaticFallbackEnabled,
	}
}
