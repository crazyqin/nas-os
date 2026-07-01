// Package clustermgr 提供集群管理器功能
// 参考群晖 DSM Cluster Manager，实现多节点集群管理、工作负载迁移、
// QoS 控管、集中化保护及节点健康监控
package clustermgr

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 错误定义 ==========

var (
	// ErrClusterNotFound 集群不存在.
	ErrClusterNotFound = errors.New("集群不存在")
	// ErrNodeNotFound 节点不存在.
	ErrNodeNotFound = errors.New("节点不存在")
	// ErrMigrationNotFound 迁移任务不存在.
	ErrMigrationNotFound = errors.New("迁移任务不存在")
	// ErrQoSRuleNotFound QoS 规则不存在.
	ErrQoSRuleNotFound = errors.New("QoS 规则不存在")
	// ErrProtectionNotFound 保护策略不存在.
	ErrProtectionNotFound = errors.New("保护策略不存在")
	// ErrNodeAlreadyExists 节点已存在.
	ErrNodeAlreadyExists = errors.New("节点已存在")
	// ErrInvalidNodeRole 无效节点角色.
	ErrInvalidNodeRole = errors.New("无效节点角色")
	// ErrMigrationFailed 迁移失败.
	ErrMigrationFailed = errors.New("工作负载迁移失败")
	// ErrNodeNotReady 节点未就绪.
	ErrNodeNotReady = errors.New("节点未就绪")
	// ErrClusterDegraded 集群已降级.
	ErrClusterDegraded = errors.New("集群已降级")
)

// ========== Service 定义 ==========

// Service 集群管理服务.
type Service struct {
	mu       sync.RWMutex
	clusters map[string]*clusterState
}

// NewService 创建集群管理服务.
func NewService() *Service {
	return &Service{
		clusters: make(map[string]*clusterState),
	}
}

// CreateCluster 创建集群.
func (s *Service) CreateCluster(ctx context.Context, req *CreateClusterRequest) (*Cluster, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("集群名称不能为空")
	}
	if req.LeaderAddress == "" {
		return nil, fmt.Errorf("主节点地址不能为空")
	}

	clusterID := uuid.New().String()
	leaderID := uuid.New().String()
	now := time.Now()

	port := req.LeaderPort
	if port == 0 {
		port = 8080
	}

	leaderNode := &ClusterNode{
		ID:            leaderID,
		Name:          req.LeaderNodeName,
		Role:          RoleLeader,
		Status:        NodeOnline,
		Address:       req.LeaderAddress,
		Port:          port,
		CPUCores:      0,
		MemoryBytes:   0,
		StorageBytes:  0,
		UsedStorage:   0,
		WorkloadCount: 0,
		JoinedAt:      now,
		LastHeartbeat: now,
	}

	cs := &clusterState{
		id:            clusterID,
		name:          req.Name,
		status:        ClusterHealthy,
		leaderID:      leaderID,
		nodes:         map[string]*ClusterNode{leaderID: leaderNode},
		migrations:    make(map[string]*WorkloadMigration),
		qosRules:      make(map[string]*QoSRule),
		protections:   make(map[string]*CentralizedProtection),
		healthRecords: make(map[string]*NodeHealth),
		createdAt:     now,
		updatedAt:     now,
		faultTolerance: 0,
	}

	s.mu.Lock()
	s.clusters[clusterID] = cs
	s.mu.Unlock()

	return s.toCluster(cs), nil
}

// GetCluster 获取集群信息.
func (s *Service) GetCluster(clusterID string) (*Cluster, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cs, ok := s.clusters[clusterID]
	if !ok {
		return nil, ErrClusterNotFound
	}
	return s.toCluster(cs), nil
}

// AddNode 添加节点到集群.
func (s *Service) AddNode(ctx context.Context, req *AddNodeRequest) (*ClusterNode, error) {
	// 验证角色
	switch req.Role {
	case RoleLeader, RoleFollower, RoleWitness, RoleStandby:
	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidNodeRole, req.Role)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cs, ok := s.clusters[req.ClusterID]
	if !ok {
		return nil, ErrClusterNotFound
	}

	// 检查同名节点
	for _, n := range cs.nodes {
		if n.Name == req.Name {
			return nil, fmt.Errorf("%w: 节点名称 %s 已存在", ErrNodeAlreadyExists, req.Name)
		}
	}

	nodeID := uuid.New().String()
	now := time.Now()
	port := req.Port
	if port == 0 {
		port = 8080
	}

	node := &ClusterNode{
		ID:            nodeID,
		Name:          req.Name,
		Role:          req.Role,
		Status:        NodeJoining,
		Address:       req.Address,
		Port:          port,
		WorkloadCount: 0,
		JoinedAt:      now,
		LastHeartbeat: now,
	}

	cs.nodes[nodeID] = node
	cs.updatedAt = now

	// 模拟加入过程完成后状态变为 online
	node.Status = NodeOnline

	// 更新故障容忍度
	cs.faultTolerance = computeFaultTolerance(cs)

	return node, nil
}

// RemoveNode 从集群中移除节点.
func (s *Service) RemoveNode(ctx context.Context, req *RemoveNodeRequest) (*ClusterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cs, ok := s.clusters[req.ClusterID]
	if !ok {
		return nil, ErrClusterNotFound
	}

	node, ok := cs.nodes[req.NodeID]
	if !ok {
		return nil, ErrNodeNotFound
	}

	// 不允许移除主节点（除非强制）
	if node.Role == RoleLeader && !req.Force {
		return nil, fmt.Errorf("不能移除主节点，请先切换主节点")
	}

	// 如果需要迁移工作负载
	if req.MigrateWorkloads && node.WorkloadCount > 0 {
		// 模拟工作负载迁移到其他节点
		for _, n := range cs.nodes {
			if n.ID != node.ID && n.Status == NodeOnline {
				n.WorkloadCount += node.WorkloadCount
				break
			}
		}
	}

	node.Status = NodeLeaving
	delete(cs.nodes, req.NodeID)
	cs.updatedAt = time.Now()
	cs.faultTolerance = computeFaultTolerance(cs)

	return &ClusterResponse{
		ClusterID: req.ClusterID,
		Success:   true,
		Message:   fmt.Sprintf("节点 %s 已移除", node.Name),
	}, nil
}

// GetNodes 获取集群所有节点.
func (s *Service) GetNodes(clusterID string) (*NodeListResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cs, ok := s.clusters[clusterID]
	if !ok {
		return nil, ErrClusterNotFound
	}

	nodes := make([]ClusterNode, 0, len(cs.nodes))
	for _, n := range cs.nodes {
		nodes = append(nodes, *n)
	}

	return &NodeListResponse{
		ClusterID: clusterID,
		Nodes:     nodes,
	}, nil
}

// MigrateWorkload 迁移工作负载.
func (s *Service) MigrateWorkload(ctx context.Context, req *MigrateWorkloadRequest) (*MigrationResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cs, ok := s.clusters[req.ClusterID]
	if !ok {
		return nil, ErrClusterNotFound
	}

	targetNode, ok := cs.nodes[req.TargetNodeID]
	if !ok {
		return nil, fmt.Errorf("%w: 目标节点 %s 不存在", ErrNodeNotFound, req.TargetNodeID)
	}

	if targetNode.Status != NodeOnline {
		return nil, fmt.Errorf("%w: 目标节点 %s 状态为 %s", ErrNodeNotReady, targetNode.Name, targetNode.Status)
	}

	// 查找源节点
	var sourceNode *ClusterNode
	for _, n := range cs.nodes {
		// 模拟：通过 workloadID 关联节点
		if n.ID != req.TargetNodeID && n.WorkloadCount > 0 {
			sourceNode = n
			break
		}
	}
	if sourceNode == nil {
		return nil, fmt.Errorf("找不到工作负载 %s 所在的源节点", req.WorkloadID)
	}

	migrationID := uuid.New().String()
	now := time.Now()

	migration := &WorkloadMigration{
		ID:           migrationID,
		SourceNodeID: sourceNode.ID,
		TargetNodeID: req.TargetNodeID,
		WorkloadID:   req.WorkloadID,
		WorkloadName: req.WorkloadID, // 简化
		Status:       MigrationRunning,
		Progress:     0,
		Reason:       req.Reason,
		StartedAt:    now,
	}

	cs.migrations[migrationID] = migration

	// 模拟迁移完成
	sourceNode.WorkloadCount--
	targetNode.WorkloadCount++
	migration.Status = MigrationCompleted
	migration.Progress = 100
	migration.FinishedAt = time.Now()

	return &MigrationResponse{
		MigrationID: migrationID,
		Status:       migration.Status,
		Progress:     migration.Progress,
		Message:      fmt.Sprintf("工作负载 %s 已从 %s 迁移到 %s", req.WorkloadID, sourceNode.Name, targetNode.Name),
	}, nil
}

// GetMigration 获取迁移状态.
func (s *Service) GetMigration(clusterID, migrationID string) (*WorkloadMigration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cs, ok := s.clusters[clusterID]
	if !ok {
		return nil, ErrClusterNotFound
	}

	m, ok := cs.migrations[migrationID]
	if !ok {
		return nil, ErrMigrationNotFound
	}
	return m, nil
}

// CreateQoSRule 创建 QoS 规则.
func (s *Service) CreateQoSRule(ctx context.Context, req *CreateQoSRuleRequest) (*QoSRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cs, ok := s.clusters[req.ClusterID]
	if !ok {
		return nil, ErrClusterNotFound
	}

	// 验证节点（如果指定）
	if req.NodeID != "" {
		if _, ok := cs.nodes[req.NodeID]; !ok {
			return nil, fmt.Errorf("%w: 节点 %s 不存在", ErrNodeNotFound, req.NodeID)
		}
	}

	ruleID := uuid.New().String()
	now := time.Now()

	priority := req.Priority
	if priority == 0 {
		priority = 50
	}

	rule := &QoSRule{
		ID:         ruleID,
		Name:       req.Name,
		Category:   req.Category,
		NodeID:     req.NodeID,
		WorkloadID: req.WorkloadID,
		Limit:      req.Limit,
		Burst:      req.Burst,
		Action:     req.Action,
		Priority:   priority,
		Enabled:    true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	cs.qosRules[ruleID] = rule
	cs.updatedAt = now

	return rule, nil
}

// GetQoSRules 获取 QoS 规则列表.
func (s *Service) GetQoSRules(clusterID string) ([]QoSRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cs, ok := s.clusters[clusterID]
	if !ok {
		return nil, ErrClusterNotFound
	}

	rules := make([]QoSRule, 0, len(cs.qosRules))
	for _, r := range cs.qosRules {
		rules = append(rules, *r)
	}
	return rules, nil
}

// DeleteQoSRule 删除 QoS 规则.
func (s *Service) DeleteQoSRule(clusterID, ruleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cs, ok := s.clusters[clusterID]
	if !ok {
		return ErrClusterNotFound
	}

	if _, ok := cs.qosRules[ruleID]; !ok {
		return ErrQoSRuleNotFound
	}
	delete(cs.qosRules, ruleID)
	cs.updatedAt = time.Now()
	return nil
}

// CreateProtection 创建集中化保护策略.
func (s *Service) CreateProtection(ctx context.Context, req *CreateProtectionRequest) (*CentralizedProtection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cs, ok := s.clusters[req.ClusterID]
	if !ok {
		return nil, ErrClusterNotFound
	}

	// 验证节点存在
	for _, nodeID := range req.NodeIDs {
		if _, ok := cs.nodes[nodeID]; !ok {
			return nil, fmt.Errorf("%w: 节点 %s 不存在", ErrNodeNotFound, nodeID)
		}
	}

	protectionID := uuid.New().String()
	now := time.Now()

	replicaCount := req.ReplicaCount
	if replicaCount == 0 {
		replicaCount = 1
	}

	protection := &CentralizedProtection{
		ID:              protectionID,
		Name:            req.Name,
		Type:            req.Type,
		Level:           req.Level,
		NodeIDs:         req.NodeIDs,
		WorkloadIDs:     req.WorkloadIDs,
		AutoFailover:    req.AutoFailover,
		MaxFailoverTime: req.MaxFailoverTime,
		ReplicaCount:    replicaCount,
		CreatedAt:       now,
		UpdatedAt:       now,
		Enabled:         true,
	}

	cs.protections[protectionID] = protection
	cs.updatedAt = now

	return protection, nil
}

// GetProtections 获取保护策略列表.
func (s *Service) GetProtections(clusterID string) ([]CentralizedProtection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cs, ok := s.clusters[clusterID]
	if !ok {
		return nil, ErrClusterNotFound
	}

	protections := make([]CentralizedProtection, 0, len(cs.protections))
	for _, p := range cs.protections {
		protections = append(protections, *p)
	}
	return protections, nil
}

// CheckNodeHealth 检查节点健康状态.
func (s *Service) CheckNodeHealth(ctx context.Context, clusterID string) (*HealthResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cs, ok := s.clusters[clusterID]
	if !ok {
		return nil, ErrClusterNotFound
	}

	now := time.Now()
	var nodes []NodeHealth
	overallStatus := ClusterHealthy

	for _, n := range cs.nodes {
		// 模拟检查
		cpuUsage := 25.0 + float64(n.WorkloadCount*10)
		if cpuUsage > 100 {
			cpuUsage = 100
		}
		memUsage := 30.0 + float64(n.WorkloadCount*8)
		if memUsage > 100 {
			memUsage = 100
		}
		diskUsage := 40.0
		if n.StorageBytes > 0 {
			diskUsage = float64(n.UsedStorage) / float64(n.StorageBytes) * 100
		}
		temp := 45.0

		health := &NodeHealth{
			NodeID:            n.ID,
			Status:            n.Status,
			CPUUsage:          cpuUsage,
			MemoryUsage:       memUsage,
			DiskUsage:         diskUsage,
			NetworkThroughput: 100.0,
			Temperature:       temp,
			Uptime:            int64(now.Sub(n.JoinedAt).Seconds()),
			LoadAvg:           [3]float64{cpuUsage / 100, cpuUsage / 100, cpuUsage / 100},
			CheckedAt:         now,
		}

		// 根据指标判定健康问题
		if n.Status != NodeOnline {
			health.Errors = append(health.Errors, fmt.Sprintf("节点状态异常: %s", n.Status))
			if overallStatus == ClusterHealthy {
				overallStatus = ClusterWarning
			}
		}
		if cpuUsage > 80 {
			health.Errors = append(health.Errors, "CPU 使用率过高")
			overallStatus = ClusterCritical
		}
		if temp > 75 {
			health.Errors = append(health.Errors, "温度过高")
			overallStatus = ClusterCritical
		}

		cs.healthRecords[n.ID] = health
		nodes = append(nodes, *health)
	}

	cs.status = overallStatus
	cs.updatedAt = now

	return &HealthResponse{
		ClusterID:     clusterID,
		Nodes:         nodes,
		OverallStatus: overallStatus,
	}, nil
}

// ListClusters 列出所有集群.
func (s *Service) ListClusters() ([]Cluster, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Cluster, 0, len(s.clusters))
	for _, cs := range s.clusters {
		result = append(result, *s.toCluster(cs))
	}
	return result, nil
}

// ========== 内部辅助方法 ==========

// toCluster 将内部状态转为对外 Cluster 对象.
func (s *Service) toCluster(cs *clusterState) *Cluster {
	nodes := make([]ClusterNode, 0, len(cs.nodes))
	healthyCount := 0
	for _, n := range cs.nodes {
		nodes = append(nodes, *n)
		if n.Status == NodeOnline {
			healthyCount++
		}
	}

	return &Cluster{
		ID:             cs.id,
		Name:           cs.name,
		Status:         cs.status,
		LeaderID:       cs.leaderID,
		Nodes:          nodes,
		NodeCount:      len(nodes),
		HealthyNodes:   healthyCount,
		CreatedAt:      cs.createdAt,
		UpdatedAt:      cs.updatedAt,
		FaultTolerance: cs.faultTolerance,
	}
}

// computeFaultTolerance 计算故障容忍度.
// 公式：(在线节点数 - 1) / 2，最少 0.
func computeFaultTolerance(cs *clusterState) int {
	onlineCount := 0
	for _, n := range cs.nodes {
		if n.Status == NodeOnline {
			onlineCount++
		}
	}
	if onlineCount <= 1 {
		return 0
	}
	return (onlineCount - 1) / 2
}