// Package lxcha 服务层
// 实现 LXC 容器状态监控、HA 节点管理、故障检测与自动故障转移
package lxcha

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 错误定义 ==========

var (
	// ErrContainerNotFound 容器不存在.
	ErrContainerNotFound = fmt.Errorf("容器不存在")
	// ErrNodeNotFound 节点不存在.
	ErrNodeNotFound = fmt.Errorf("HA 节点不存在")
	// ErrNodeOffline 节点离线.
	ErrNodeOffline = fmt.Errorf("HA 节点离线")
	// ErrFailoverInProgress 故障转移进行中.
	ErrFailoverInProgress = fmt.Errorf("故障转移正在进行中")
	// ErrAlreadyRegistered 容器已注册.
	ErrAlreadyRegistered = fmt.Errorf("容器已注册到 HA")
	// ErrPolicyDisabled 故障转移策略已禁用.
	ErrPolicyDisabled = fmt.Errorf("容器故障转移已禁用")
)

// ========== Service 定义 ==========

// Service LXC HA 故障转移管理服务.
type Service struct {
	mu             sync.RWMutex
	config         *Config
	containers     map[string]*LXCContainer   // 容器 ID -> 容器
	nodes          map[string]*HANode         // 节点 ID -> 节点
	policies       map[string]*FailoverPolicy // 容器 ID -> 故障转移策略
	failoverStates map[string]*FailoverState  // 容器 ID -> 故障转移状态
	ipReservations map[string]*IPReservation  // IP -> 预留记录
	history        []*FailoverHistoryEntry
	nodeHeartbeats map[string]time.Time // 节点 ID -> 最后心跳时间
}

// NewService 创建 HA 故障转移服务.
func NewService(cfg *Config) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Service{
		config:         cfg,
		containers:     make(map[string]*LXCContainer),
		nodes:          make(map[string]*HANode),
		policies:       make(map[string]*FailoverPolicy),
		failoverStates: make(map[string]*FailoverState),
		ipReservations: make(map[string]*IPReservation),
		nodeHeartbeats: make(map[string]time.Time),
	}
}

// ========== 节点管理 ==========

// RegisterNode 注册 HA 节点.
func (s *Service) RegisterNode(node *HANode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node.ID == "" {
		return fmt.Errorf("节点 ID 不能为空")
	}
	if _, exists := s.nodes[node.ID]; exists {
		return fmt.Errorf("节点 %s 已存在", node.ID)
	}

	node.LastSeen = time.Now()
	if node.State == "" {
		node.State = NodeStateOnline
	}
	s.nodes[node.ID] = node
	s.nodeHeartbeats[node.ID] = node.LastSeen
	return nil
}

// RemoveNode 移除 HA 节点.
func (s *Service) RemoveNode(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[nodeID]; !exists {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID)
	}

	// 检查是否有容器在该节点上
	for _, c := range s.containers {
		if c.NodeID == nodeID {
			return fmt.Errorf("节点 %s 上仍有容器 %s，无法移除", nodeID, c.ID)
		}
	}

	delete(s.nodes, nodeID)
	delete(s.nodeHeartbeats, nodeID)
	return nil
}

// GetNodes 获取所有 HA 节点列表.
func (s *Service) GetNodes() []*HANode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*HANode, 0, len(s.nodes))
	for _, n := range s.nodes {
		result = append(result, n)
	}
	return result
}

// GetNode 获取单个节点信息.
func (s *Service) GetNode(nodeID string) (*HANode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID)
	}
	return node, nil
}

// UpdateNodeHeartbeat 更新节点心跳.
func (s *Service) UpdateNodeHeartbeat(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[nodeID]; !exists {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID)
	}

	s.nodeHeartbeats[nodeID] = time.Now()
	s.nodes[nodeID].LastSeen = time.Now()
	s.nodes[nodeID].State = NodeStateOnline
	return nil
}

// ========== 容器注册与管理 ==========

// RegisterContainer 注册容器到 HA 管理.
func (s *Service) RegisterContainer(req *RegisterContainerRequest) (*LXCContainer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.containers[req.ContainerID]; exists {
		return nil, fmt.Errorf("%w: %s", ErrAlreadyRegistered, req.ContainerID)
	}

	now := time.Now()
	container := &LXCContainer{
		ID:        req.ContainerID,
		Name:      req.ContainerID,
		State:     StateStopped,
		NodeID:    s.config.NodeID,
		IPConfigs: req.IPConfigs,
		HAEnabled: req.Policy != PPolicyNone,
		Policy:    req.Policy,
		Priority:  req.Priority,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.containers[req.ContainerID] = container

	// 创建故障转移策略
	policy := &FailoverPolicy{
		ContainerID:    req.ContainerID,
		Type:           req.Policy,
		MaxRetries:     3,
		HealthCheckInt: s.config.HealthCheckSeconds,
		FailoverDelay:  5,
	}
	if policy.Type == "" {
		policy.Type = PolicyAuto
		container.Policy = PolicyAuto
		container.HAEnabled = true
	}
	s.policies[req.ContainerID] = policy

	// 初始化为健康状态
	s.failoverStates[req.ContainerID] = &FailoverState{
		ContainerID: req.ContainerID,
		State:       FStateHealthy,
		SourceNode:  s.config.NodeID,
	}

	// 预留静态 IP
	for _, ipCfg := range req.IPConfigs {
		if ipCfg.Address != "" {
			s.ipReservations[ipCfg.Address] = &IPReservation{
				IP:          ipCfg.Address,
				ContainerID: req.ContainerID,
				NodeID:      s.config.NodeID,
				ReservedAt:  now,
			}
		}
	}

	return container, nil
}

// UnregisterContainer 取消容器 HA 注册.
func (s *Service) UnregisterContainer(containerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.containers[containerID]; !exists {
		return fmt.Errorf("%w: %s", ErrContainerNotFound, containerID)
	}

	// 移除 IP 预留
	for ip, res := range s.ipReservations {
		if res.ContainerID == containerID {
			delete(s.ipReservations, ip)
		}
	}

	delete(s.containers, containerID)
	delete(s.policies, containerID)
	delete(s.failoverStates, containerID)
	return nil
}

// GetContainers 获取所有 HA 容器列表.
func (s *Service) GetContainers() []*LXCContainer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*LXCContainer, 0, len(s.containers))
	for _, c := range s.containers {
		result = append(result, c)
	}
	return result
}

// GetContainer 获取单个容器信息.
func (s *Service) GetContainer(containerID string) (*LXCContainer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.containers[containerID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrContainerNotFound, containerID)
	}
	return c, nil
}

// UpdateContainerState 更新容器运行状态.
func (s *Service) UpdateContainerState(containerID string, state ContainerState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.containers[containerID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrContainerNotFound, containerID)
	}

	c.State = state
	c.UpdatedAt = time.Now()
	if state == StateRunning {
		c.Uptime = 0 // 实际应由监控填充
	}
	return nil
}

// ========== 策略管理 ==========

// GetPolicy 获取容器故障转移策略.
func (s *Service) GetPolicy(containerID string) (*FailoverPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.policies[containerID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrContainerNotFound, containerID)
	}
	return p, nil
}

// UpdatePolicy 更新故障转移策略.
func (s *Service) UpdatePolicy(req *UpdatePolicyRequest) (*FailoverPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.policies[req.ContainerID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrContainerNotFound, req.ContainerID)
	}

	if req.Type != "" {
		p.Type = req.Type
	}
	if req.PreferredNode != "" {
		p.PreferredNode = req.PreferredNode
	}
	if req.MaxRetries > 0 {
		p.MaxRetries = req.MaxRetries
	}
	if req.HealthCheckInt > 0 {
		p.HealthCheckInt = req.HealthCheckInt
	}
	if req.FailoverDelay > 0 {
		p.FailoverDelay = req.FailoverDelay
	}

	// 同步更新容器 HA 状态
	if c, ok := s.containers[req.ContainerID]; ok {
		c.Policy = p.Type
		c.HAEnabled = p.Type != PPolicyNone
		c.UpdatedAt = time.Now()
	}

	return p, nil
}

// ========== 容器迁移 ==========

// MigrateContainer 执行容器迁移.
func (s *Service) MigrateContainer(req *MigrateRequest) (*MigrateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.containers[req.ContainerID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrContainerNotFound, req.ContainerID)
	}

	targetNode, ok := s.nodes[req.TargetNode]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, req.TargetNode)
	}
	if targetNode.State != NodeStateOnline {
		return nil, fmt.Errorf("%w: %s", ErrNodeOffline, req.TargetNode)
	}

	if c.State == StateMigrating {
		return nil, fmt.Errorf("%w: %s", ErrFailoverInProgress, req.ContainerID)
	}

	sourceNode := c.NodeID
	startTime := time.Now()

	// 标记迁移中
	oldState := c.State
	c.State = StateMigrating
	c.TargetNodeID = req.TargetNode
	c.UpdatedAt = startTime

	// 更新故障转移状态
	fs := s.failoverStates[req.ContainerID]
	if fs == nil {
		fs = &FailoverState{ContainerID: req.ContainerID}
		s.failoverStates[req.ContainerID] = fs
	}
	fs.State = FStateFailover
	fs.SourceNode = sourceNode
	fs.TargetNode = req.TargetNode
	fs.StartedAt = startTime

	// 执行迁移（模拟）
	result := &MigrateResult{
		ContainerID: req.ContainerID,
		SourceNode:  sourceNode,
		TargetNode:  req.TargetNode,
	}

	// 检查目标节点是否有 IP 冲突
	ipConflict := false
	if req.KeepIP {
		for _, ipCfg := range c.IPConfigs {
			if res, exists := s.ipReservations[ipCfg.Address]; exists {
				if res.NodeID != req.TargetNode && res.ContainerID != req.ContainerID {
					ipConflict = true
					break
				}
			}
		}
	}

	if ipConflict && !req.Online {
		// IP 冲突且非在线迁移，先释放原节点 IP
		for _, ipCfg := range c.IPConfigs {
			if res, exists := s.ipReservations[ipCfg.Address]; exists && res.ContainerID == req.ContainerID {
				res.NodeID = req.TargetNode
				res.ReservedAt = time.Now()
			}
		}
	}

	// 完成迁移
	c.NodeID = req.TargetNode
	c.TargetNodeID = ""
	if req.Online {
		c.State = oldState // 在线迁移保持原状态
	} else {
		c.State = StateStopped // 离线迁移后容器停止
		c.AutoStart = true     // 标记需要自动启动
	}
	c.UpdatedAt = time.Now()

	// 更新 IP 预留
	if req.KeepIP {
		for _, ipCfg := range c.IPConfigs {
			if ipCfg.Address != "" {
				s.ipReservations[ipCfg.Address] = &IPReservation{
					IP:          ipCfg.Address,
					ContainerID: req.ContainerID,
					NodeID:      req.TargetNode,
					ReservedAt:  time.Now(),
				}
			}
		}
	}

	// 更新故障转移状态
	fs.State = FStateHealthy
	fs.TargetNode = req.TargetNode
	fs.SourceNode = sourceNode
	fs.CompletedAt = time.Now()

	result.Success = true
	result.Duration = time.Since(startTime).Seconds()
	result.CompletedAt = time.Now()

	return result, nil
}

// ========== 故障检测与自动故障转移 ==========

// CheckNodeHealth 检查节点健康状态
// 比较节点最后心跳时间与超时阈值，判断是否故障.
func (s *Service) CheckNodeHealth() []*HANode {
	s.mu.Lock()
	defer s.mu.Unlock()

	var failedNodes []*HANode
	threshold := time.Duration(s.config.HealthCheckSeconds*3) * time.Second

	for nodeID, lastSeen := range s.nodeHeartbeats {
		if time.Since(lastSeen) > threshold {
			node, ok := s.nodes[nodeID]
			if !ok || node.State == NodeStateOffline {
				continue
			}
			node.State = NodeStateOffline
			failedNodes = append(failedNodes, node)
		}
	}

	return failedNodes
}

// TriggerFailover 手动触发故障转移.
func (s *Service) TriggerFailover(req *TriggerFailoverRequest) (*MigrateResult, error) {
	s.mu.Lock()

	c, ok := s.containers[req.ContainerID]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrContainerNotFound, req.ContainerID)
	}

	p, ok := s.policies[req.ContainerID]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrContainerNotFound, req.ContainerID)
	}

	// 检查策略
	if p.Type == PPolicyNone && !req.Force {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrPolicyDisabled, req.ContainerID)
	}

	// 如果已在进行中
	fs, ok := s.failoverStates[req.ContainerID]
	if ok && fs.State == FStateFailover {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrFailoverInProgress, req.ContainerID)
	}

	// 选择目标节点
	targetNode := req.TargetNode
	if targetNode == "" {
		targetNode = p.PreferredNode
	}
	if targetNode == "" {
		// 自动选择：找负载最低的在线备份节点
		targetNode = s.selectFailoverNode(c.NodeID)
		if targetNode == "" {
			s.mu.Unlock()
			return nil, fmt.Errorf("无可用的故障转移目标节点")
		}
	}

	sourceNode := c.NodeID
	startTime := time.Now()

	// 初始化故障转移状态
	if fs == nil {
		fs = &FailoverState{ContainerID: req.ContainerID}
		s.failoverStates[req.ContainerID] = fs
	}
	fs.State = FStateFailover
	fs.SourceNode = sourceNode
	fs.TargetNode = targetNode
	fs.StartedAt = startTime
	fs.RetryCount = 0
	fs.Error = ""

	s.mu.Unlock()

	// 执行迁移
	migrateReq := &MigrateRequest{
		ContainerID: req.ContainerID,
		TargetNode:  targetNode,
		Online:      false,
		Timeout:     300,
		KeepIP:      true,
	}

	result, err := s.MigrateContainer(migrateReq)

	// 记录历史
	s.mu.Lock()
	now := time.Now()
	entry := &FailoverHistoryEntry{
		ID:          uuid.New().String(),
		ContainerID: req.ContainerID,
		SourceNode:  sourceNode,
		TargetNode:  targetNode,
		Reason:      "manual_trigger",
		StartedAt:   startTime,
		FinishedAt:  now,
	}
	if err != nil {
		entry.Success = false
		entry.Error = err.Error()
		entry.State = FStateFailed
		fs.State = FStateFailed
		fs.Error = err.Error()
		fs.RetryCount++
	} else {
		entry.Success = true
		entry.State = FStateHealthy
	}
	s.history = append(s.history, entry)
	s.mu.Unlock()

	return result, err
}

// AutoFailover 自动故障转移（由健康检查触发）.
func (s *Service) AutoFailover(containerID string, failedNodeID string) (*MigrateResult, error) {
	s.mu.Lock()

	c, ok := s.containers[containerID]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrContainerNotFound, containerID)
	}

	p, ok := s.policies[containerID]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrContainerNotFound, containerID)
	}

	if p.Type != PolicyAuto {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrPolicyDisabled, containerID)
	}

	// 仅处理指定节点上的容器
	if c.NodeID != failedNodeID {
		s.mu.Unlock()
		return nil, fmt.Errorf("容器 %s 不在故障节点 %s 上", containerID, failedNodeID)
	}

	// 选择目标节点
	targetNode := p.PreferredNode
	if targetNode == "" || targetNode == failedNodeID {
		targetNode = s.selectFailoverNode(failedNodeID)
	}
	if targetNode == "" {
		// 更新状态为失败
		if fs, ok := s.failoverStates[containerID]; ok {
			fs.State = FStateFailed
			fs.Error = "无可用的故障转移目标节点"
		}
		s.mu.Unlock()
		return nil, fmt.Errorf("无可用的故障转移目标节点")
	}

	fs := s.failoverStates[containerID]
	if fs == nil {
		fs = &FailoverState{ContainerID: containerID}
		s.failoverStates[containerID] = fs
	}
	if fs.State == FStateFailover {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrFailoverInProgress, containerID)
	}

	fs.State = FStateFailover
	fs.SourceNode = failedNodeID
	fs.TargetNode = targetNode
	fs.StartedAt = time.Now()
	fs.RetryCount = 0

	s.mu.Unlock()

	// 执行迁移
	result, err := s.MigrateContainer(&MigrateRequest{
		ContainerID: containerID,
		TargetNode:  targetNode,
		Online:      false,
		Timeout:     300,
		KeepIP:      true,
	})

	// 记录历史
	s.mu.Lock()
	entry := &FailoverHistoryEntry{
		ID:          uuid.New().String(),
		ContainerID: containerID,
		SourceNode:  failedNodeID,
		TargetNode:  targetNode,
		Reason:      fmt.Sprintf("auto_failover:node_%s_failed", failedNodeID),
		StartedAt:   time.Now(),
		FinishedAt:  time.Now(),
	}
	if err != nil {
		entry.Success = false
		entry.Error = err.Error()
		entry.State = FStateFailed
	} else {
		entry.Success = true
		entry.State = FStateHealthy
	}
	s.history = append(s.history, entry)
	s.mu.Unlock()

	return result, err
}

// selectFailoverNode 选择故障转移目标节点（调用方需持有锁）.
func (s *Service) selectFailoverNode(excludeNodeID string) string {
	var bestNode *HANode
	minLoad := 999999.0

	for _, node := range s.nodes {
		if node.ID == excludeNodeID {
			continue
		}
		if node.State != NodeStateOnline {
			continue
		}
		if node.Role == NodeRoleWitness {
			continue
		}
		// 负载 = 容器数 + CPU 使用率 / 10
		load := float64(node.Containers) + node.CPUUsage/10
		if load < minLoad {
			minLoad = load
			bestNode = node
		}
	}

	if bestNode == nil {
		return ""
	}
	return bestNode.ID
}

// ========== IP 管理 ==========

// ReserveIP 预留静态 IP.
func (s *Service) ReserveIP(req *ReserveIPRequest) (*IPReservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.ipReservations[req.IP]; exists {
		existing := s.ipReservations[req.IP]
		if existing.ContainerID != req.ContainerID {
			return nil, fmt.Errorf("IP %s 已被容器 %s 预留", req.IP, existing.ContainerID)
		}
		// 更新节点
		existing.NodeID = req.NodeID
		existing.ReservedAt = time.Now()
		return existing, nil
	}

	reservation := &IPReservation{
		IP:          req.IP,
		ContainerID: req.ContainerID,
		NodeID:      req.NodeID,
		ReservedAt:  time.Now(),
	}
	s.ipReservations[req.IP] = reservation
	return reservation, nil
}

// ReleaseIP 释放 IP 预留.
func (s *Service) ReleaseIP(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.ipReservations[ip]; !exists {
		return fmt.Errorf("IP %s 未被预留", ip)
	}
	delete(s.ipReservations, ip)
	return nil
}

// GetIPReservations 获取所有 IP 预留.
func (s *Service) GetIPReservations() []*IPReservation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*IPReservation, 0, len(s.ipReservations))
	for _, r := range s.ipReservations {
		result = append(result, r)
	}
	return result
}

// CheckIPConflict 检查 IP 冲突.
func (s *Service) CheckIPConflict(ip, containerID, nodeID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res, exists := s.ipReservations[ip]
	if !exists {
		return false
	}
	return res.ContainerID != containerID || res.NodeID != nodeID
}

// ========== 故障转移状态查询 ==========

// GetFailoverState 获取容器故障转移状态.
func (s *Service) GetFailoverState(containerID string) (*FailoverState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fs, ok := s.failoverStates[containerID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrContainerNotFound, containerID)
	}
	return fs, nil
}

// GetFailoverEvents 获取故障转移事件列表.
func (s *Service) GetFailoverEvents(containerID string) []*FailoverEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if containerID == "" {
		// 返回所有历史转换为事件
		result := make([]*FailoverEvent, 0, len(s.history))
		for _, h := range s.history {
			result = append(result, &FailoverEvent{
				ID:          h.ID,
				ContainerID: h.ContainerID,
				SourceNode:  h.SourceNode,
				TargetNode:  h.TargetNode,
				Reason:      h.Reason,
				Success:     h.Success,
				EndState:    h.State,
				Timestamp:   h.StartedAt,
				Error:       h.Error,
			})
		}
		return result
	}

	result := make([]*FailoverEvent, 0)
	for _, h := range s.history {
		if h.ContainerID == containerID {
			result = append(result, &FailoverEvent{
				ID:          h.ID,
				ContainerID: h.ContainerID,
				SourceNode:  h.SourceNode,
				TargetNode:  h.TargetNode,
				Reason:      h.Reason,
				Success:     h.Success,
				EndState:    h.State,
				Timestamp:   h.StartedAt,
				Error:       h.Error,
			})
		}
	}
	return result
}

// GetHistory 获取故障转移历史.
func (s *Service) GetHistory() []*FailoverHistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*FailoverHistoryEntry, len(s.history))
	copy(result, s.history)
	return result
}

// ========== 状态总览 ==========

// GetStatus 获取 HA 集群状态总览.
func (s *Service) GetStatus() *HAStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &HAStatus{
		History: make([]*FailoverHistoryEntry, len(s.history)),
	}

	for _, node := range s.nodes {
		status.TotalNodes++
		if node.State == NodeStateOnline {
			status.OnlineNodes++
		}
	}

	for _, c := range s.containers {
		status.TotalContainers++
		if c.HAEnabled {
			status.HAContainers++
		}
	}

	for _, fs := range s.failoverStates {
		if fs.State == FStateFailover {
			status.ActiveFailovers++
		}
	}

	status.IPReservations = len(s.ipReservations)
	status.FailoverEvents = len(s.history)
	copy(status.History, s.history)

	return status
}
