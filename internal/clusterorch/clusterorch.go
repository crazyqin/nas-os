// Package clusterorch 提供分布式集群编排功能，对标 TrueNAS SCALE 集群 + Proxmox VE
package clusterorch

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"
)

// ==================== 错误定义 ====================
var (
	ErrNodeNotFound         = errors.New("node not found")
	ErrNodeAlreadyExists    = errors.New("node already exists")
	ErrServiceNotFound      = errors.New("service not found")
	ErrServiceAlreadyExists = errors.New("service already exists")
	ErrNoAvailableNode      = errors.New("no available node")
	ErrResourceInsufficient = errors.New("insufficient resources")
	ErrConfigNotFound       = errors.New("config not found")
	ErrClusterFull          = errors.New("cluster is full")
	ErrAutoScaleDisabled    = errors.New("auto-scaling is disabled")
)

// ==================== 类型定义 ====================
type NodeState string

const (
	NodeStateOnline      NodeState = "online"
	NodeStateOffline     NodeState = "offline"
	NodeStateJoining     NodeState = "joining"
	NodeStateLeaving     NodeState = "leaving"
	NodeStateMaintenance NodeState = "maintenance"
)

type ServiceState string

const (
	ServiceStateRunning   ServiceState = "running"
	ServiceStateStopped   ServiceState = "stopped"
	ServiceStateFailed    ServiceState = "failed"
	ServiceStateMigrating ServiceState = "migrating"
)

type LoadBalanceStrategy string

const (
	LBStrategyRoundRobin       LoadBalanceStrategy = "round_robin"
	LBStrategyLeastConnections LoadBalanceStrategy = "least_connections"
	LBStrategyRandom           LoadBalanceStrategy = "random"
	LBStrategyHash             LoadBalanceStrategy = "hash"
)

type ResourceAllocationStrategy string

const (
	ResourceStrategyBinpack ResourceAllocationStrategy = "binpack"
	ResourceStrategySpread  ResourceAllocationStrategy = "spread"
	ResourceStrategyRandom  ResourceAllocationStrategy = "random"
)

// ==================== 核心结构体 ====================
type Node struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Address       string            `json:"address"`
	State         NodeState         `json:"state"`
	Resources     *NodeResources    `json:"resources"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	Weight        int               `json:"weight"`
	ActiveConns   int64             `json:"active_connections"`
}

type NodeResources struct {
	CPU     ResourcePool `json:"cpu"`
	Memory  ResourcePool `json:"memory"`
	Storage ResourcePool `json:"storage"`
}

type ResourcePool struct {
	Total     int64 `json:"total"`
	Used      int64 `json:"used"`
	Available int64 `json:"available"`
}

type Service struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	NodeID       string            `json:"node_id"`
	State        ServiceState      `json:"state"`
	Ports        []int             `json:"ports"`
	HealthCheck  *HealthCheck      `json:"health_check"`
	Resources    *ResourceRequest  `json:"resources"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	FailoverNode string            `json:"failover_node"`
}

type ResourceRequest struct {
	CPU     int64 `json:"cpu"`
	Memory  int64 `json:"memory"`
	Storage int64 `json:"storage"`
}

type HealthCheck struct {
	Interval         time.Duration `json:"interval"`
	Timeout          time.Duration `json:"timeout"`
	FailureThreshold int           `json:"failure_threshold"`
	SuccessThreshold int           `json:"success_threshold"`
	LastCheck        time.Time     `json:"last_check"`
	ConsecutiveFail  int           `json:"consecutive_fail"`
	ConsecutiveOK    int           `json:"consecutive_ok"`
	Healthy          bool          `json:"healthy"`
}

type ClusterConfig struct {
	Version   int               `json:"version"`
	Data      map[string]string `json:"data"`
	UpdatedAt time.Time         `json:"updated_at"`
	UpdatedBy string            `json:"updated_by"`
	Comment   string            `json:"comment"`
}

type ClusterLog struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"node_id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Service   string    `json:"service,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type LogFilter struct {
	NodeID    string
	Level     string
	Service   string
	Keyword   string
	StartTime time.Time
	EndTime   time.Time
	Limit     int
}

// ClusterOrch 集群编排器
type ClusterOrch struct {
	mu                 sync.RWMutex
	nodes              map[string]*Node
	localID            string
	services           map[string]*Service
	resourceStrategy   ResourceAllocationStrategy
	lbStrategy         LoadBalanceStrategy
	roundRobinIdx      uint64
	configs            map[string]*ClusterConfig
	configHistory      map[string][]*ClusterConfig
	logs               []*ClusterLog
	maxLogSize         int
	autoScale          bool
	scaleUpThreshold   float64
	scaleDownThreshold float64
	minNodes           int
	maxNodes           int
}

type ClusterOrchConfig struct {
	LocalID            string
	ResourceStrategy   ResourceAllocationStrategy
	LBStrategy         LoadBalanceStrategy
	MaxLogSize         int
	AutoScale          bool
	ScaleUpThreshold   float64
	ScaleDownThreshold float64
	MinNodes           int
	MaxNodes           int
}

// ==================== 构造函数 ====================
func New(cfg ClusterOrchConfig) *ClusterOrch {
	if cfg.ResourceStrategy == "" {
		cfg.ResourceStrategy = ResourceStrategySpread
	}
	if cfg.LBStrategy == "" {
		cfg.LBStrategy = LBStrategyRoundRobin
	}
	if cfg.MaxLogSize <= 0 {
		cfg.MaxLogSize = 10000
	}
	if cfg.ScaleUpThreshold <= 0 {
		cfg.ScaleUpThreshold = 0.8
	}
	if cfg.ScaleDownThreshold <= 0 {
		cfg.ScaleDownThreshold = 0.3
	}
	if cfg.MinNodes <= 0 {
		cfg.MinNodes = 1
	}
	if cfg.MaxNodes <= 0 {
		cfg.MaxNodes = 100
	}
	return &ClusterOrch{
		nodes: make(map[string]*Node), localID: cfg.LocalID,
		services: make(map[string]*Service), resourceStrategy: cfg.ResourceStrategy,
		lbStrategy: cfg.LBStrategy, configs: make(map[string]*ClusterConfig),
		configHistory: make(map[string][]*ClusterConfig),
		logs:          make([]*ClusterLog, 0, cfg.MaxLogSize), maxLogSize: cfg.MaxLogSize,
		autoScale: cfg.AutoScale, scaleUpThreshold: cfg.ScaleUpThreshold,
		scaleDownThreshold: cfg.ScaleDownThreshold, minNodes: cfg.MinNodes, maxNodes: cfg.MaxNodes,
	}
}

// ==================== 集群管理 ====================
func (co *ClusterOrch) AddNode(node *Node) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	if _, exists := co.nodes[node.ID]; exists {
		return ErrNodeAlreadyExists
	}
	if len(co.nodes) >= co.maxNodes {
		return ErrClusterFull
	}
	node.State = NodeStateJoining
	node.LastHeartbeat = time.Now()
	co.nodes[node.ID] = node
	co.addLog(node.ID, "info", fmt.Sprintf("节点 %s (%s) 正在加入集群", node.Name, node.Address))
	node.State = NodeStateOnline
	co.addLog(node.ID, "info", fmt.Sprintf("节点 %s 已上线", node.Name))
	return nil
}

func (co *ClusterOrch) RemoveNode(nodeID string) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	node, exists := co.nodes[nodeID]
	if !exists {
		return ErrNodeNotFound
	}
	if len(co.nodes) <= co.minNodes {
		return fmt.Errorf("cannot remove node: minimum node count reached (%d)", co.minNodes)
	}
	for _, svc := range co.services {
		if svc.NodeID == nodeID {
			target := co.findFailoverTargetLocked(nodeID)
			if target != nil {
				svc.NodeID = target.ID
				svc.State = ServiceStateRunning
				co.addLog(svc.ID, "info", fmt.Sprintf("服务 %s 从节点 %s 迁移到 %s", svc.Name, nodeID, target.ID))
			} else {
				svc.State = ServiceStateStopped
				co.addLog(svc.ID, "warn", fmt.Sprintf("服务 %s 无法迁移，已停止", svc.Name))
			}
		}
	}
	node.State = NodeStateLeaving
	co.addLog(nodeID, "info", fmt.Sprintf("节点 %s 正在离开集群", node.Name))
	delete(co.nodes, nodeID)
	co.addLog(nodeID, "info", fmt.Sprintf("节点 %s 已离开集群", node.Name))
	return nil
}

func (co *ClusterOrch) GetNode(nodeID string) (*Node, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()
	node, exists := co.nodes[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}
	nodeCopy := *node
	return &nodeCopy, nil
}

func (co *ClusterOrch) ListNodes() []*Node {
	co.mu.RLock()
	defer co.mu.RUnlock()
	nodes := make([]*Node, 0, len(co.nodes))
	for _, node := range co.nodes {
		nodeCopy := *node
		nodes = append(nodes, &nodeCopy)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return nodes
}

func (co *ClusterOrch) UpdateNodeHeartbeat(nodeID string) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	node, exists := co.nodes[nodeID]
	if !exists {
		return ErrNodeNotFound
	}
	node.LastHeartbeat = time.Now()
	if node.State == NodeStateOffline {
		node.State = NodeStateOnline
		co.addLog(nodeID, "info", fmt.Sprintf("节点 %s 重新上线", node.Name))
	}
	return nil
}

func (co *ClusterOrch) MarkNodeOffline(nodeID string) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	node, exists := co.nodes[nodeID]
	if !exists {
		return ErrNodeNotFound
	}
	node.State = NodeStateOffline
	co.addLog(nodeID, "warn", fmt.Sprintf("节点 %s 已离线", node.Name))
	for _, svc := range co.services {
		if svc.NodeID == nodeID && svc.State == ServiceStateRunning {
			target := co.findFailoverTargetLocked(nodeID)
			if target != nil {
				svc.NodeID = target.ID
				svc.State = ServiceStateRunning
				co.addLog(svc.ID, "info", fmt.Sprintf("服务 %s 自动故障转移到节点 %s", svc.Name, target.ID))
			} else {
				svc.State = ServiceStateFailed
				co.addLog(svc.ID, "error", fmt.Sprintf("服务 %s 故障转移失败，无可用节点", svc.Name))
			}
		}
	}
	return nil
}

func (co *ClusterOrch) GetClusterTopology() map[string]interface{} {
	co.mu.RLock()
	defer co.mu.RUnlock()
	nodesList := make([]map[string]interface{}, 0, len(co.nodes))
	for _, node := range co.nodes {
		nodesList = append(nodesList, map[string]interface{}{
			"id": node.ID, "name": node.Name, "state": node.State, "weight": node.Weight,
		})
	}
	svcByNode := make(map[string]int)
	for _, svc := range co.services {
		svcByNode[svc.NodeID]++
	}
	return map[string]interface{}{
		"node_count": len(co.nodes), "service_count": len(co.services),
		"nodes": nodesList, "services_by_node": svcByNode,
	}
}

func (co *ClusterOrch) SetNodeMaintenance(nodeID string, maintenance bool) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	node, exists := co.nodes[nodeID]
	if !exists {
		return ErrNodeNotFound
	}
	if maintenance {
		node.State = NodeStateMaintenance
		co.addLog(nodeID, "info", fmt.Sprintf("节点 %s 进入维护模式", node.Name))
	} else {
		node.State = NodeStateOnline
		co.addLog(nodeID, "info", fmt.Sprintf("节点 %s 退出维护模式", node.Name))
	}
	return nil
}

// ==================== 服务编排 ====================
func (co *ClusterOrch) RegisterService(svc *Service) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	if _, exists := co.services[svc.ID]; exists {
		return ErrServiceAlreadyExists
	}
	node, exists := co.nodes[svc.NodeID]
	if !exists {
		return ErrNodeNotFound
	}
	if node.State != NodeStateOnline {
		return fmt.Errorf("target node %s is not online", svc.NodeID)
	}
	if svc.Resources != nil {
		if !co.checkResourceLocked(svc.NodeID, svc.Resources) {
			return ErrResourceInsufficient
		}
		co.allocateResourceLocked(svc.NodeID, svc.Resources)
	}
	now := time.Now()
	svc.State = ServiceStateRunning
	svc.CreatedAt = now
	svc.UpdatedAt = now
	if svc.HealthCheck == nil {
		svc.HealthCheck = &HealthCheck{
			Interval: 30 * time.Second, Timeout: 5 * time.Second,
			FailureThreshold: 3, SuccessThreshold: 2, Healthy: true,
		}
	}
	co.services[svc.ID] = svc
	co.addLog(svc.ID, "info", fmt.Sprintf("服务 %s 已注册到节点 %s", svc.Name, svc.NodeID))
	return nil
}

func (co *ClusterOrch) DeregisterService(serviceID string) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	svc, exists := co.services[serviceID]
	if !exists {
		return ErrServiceNotFound
	}
	if svc.Resources != nil {
		co.releaseResourceLocked(svc.NodeID, svc.Resources)
	}
	delete(co.services, serviceID)
	co.addLog(serviceID, "info", fmt.Sprintf("服务 %s 已注销", svc.Name))
	return nil
}

func (co *ClusterOrch) GetService(serviceID string) (*Service, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()
	svc, exists := co.services[serviceID]
	if !exists {
		return nil, ErrServiceNotFound
	}
	svcCopy := *svc
	return &svcCopy, nil
}

func (co *ClusterOrch) ListServices() []*Service {
	co.mu.RLock()
	defer co.mu.RUnlock()
	services := make([]*Service, 0, len(co.services))
	for _, svc := range co.services {
		svcCopy := *svc
		services = append(services, &svcCopy)
	}
	sort.Slice(services, func(i, j int) bool { return services[i].ID < services[j].ID })
	return services
}

func (co *ClusterOrch) DiscoverService(name string) []*Service {
	co.mu.RLock()
	defer co.mu.RUnlock()
	var result []*Service
	for _, svc := range co.services {
		if svc.Name == name && svc.State == ServiceStateRunning {
			svcCopy := *svc
			result = append(result, &svcCopy)
		}
	}
	return result
}

func (co *ClusterOrch) HealthCheckService(serviceID string, healthy bool) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	svc, exists := co.services[serviceID]
	if !exists {
		return ErrServiceNotFound
	}
	hc := svc.HealthCheck
	hc.LastCheck = time.Now()
	if healthy {
		hc.ConsecutiveFail = 0
		hc.ConsecutiveOK++
		if hc.ConsecutiveOK >= hc.SuccessThreshold {
			hc.Healthy = true
			if svc.State == ServiceStateFailed {
				svc.State = ServiceStateRunning
				co.addLog(serviceID, "info", fmt.Sprintf("服务 %s 恢复健康", svc.Name))
			}
		}
	} else {
		hc.ConsecutiveOK = 0
		hc.ConsecutiveFail++
		if hc.ConsecutiveFail >= hc.FailureThreshold {
			hc.Healthy = false
			svc.State = ServiceStateFailed
			co.addLog(serviceID, "warn", fmt.Sprintf("服务 %s 健康检查失败", svc.Name))
			if svc.FailoverNode != "" {
				target, exists := co.nodes[svc.FailoverNode]
				if exists && target.State == NodeStateOnline {
					svc.NodeID = target.ID
					svc.State = ServiceStateRunning
					hc.ConsecutiveFail = 0
					hc.ConsecutiveOK = 0
					co.addLog(serviceID, "info", fmt.Sprintf("服务 %s 故障转移到节点 %s", svc.Name, target.ID))
				}
			}
		}
	}
	svc.UpdatedAt = time.Now()
	return nil
}

func (co *ClusterOrch) MigrateService(serviceID, targetNodeID string) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	svc, exists := co.services[serviceID]
	if !exists {
		return ErrServiceNotFound
	}
	targetNode, exists := co.nodes[targetNodeID]
	if !exists {
		return ErrNodeNotFound
	}
	if targetNode.State != NodeStateOnline {
		return fmt.Errorf("target node %s is not online", targetNodeID)
	}
	if svc.Resources != nil {
		if !co.checkResourceLocked(targetNodeID, svc.Resources) {
			return ErrResourceInsufficient
		}
		co.releaseResourceLocked(svc.NodeID, svc.Resources)
		co.allocateResourceLocked(targetNodeID, svc.Resources)
	}
	oldNodeID := svc.NodeID
	svc.NodeID = targetNodeID
	svc.State = ServiceStateRunning
	svc.UpdatedAt = time.Now()
	co.addLog(serviceID, "info", fmt.Sprintf("服务 %s 从节点 %s 迁移到 %s", svc.Name, oldNodeID, targetNodeID))
	return nil
}

// ==================== 资源调度 ====================
func (co *ClusterOrch) UpdateNodeResources(nodeID string, resources *NodeResources) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	node, exists := co.nodes[nodeID]
	if !exists {
		return ErrNodeNotFound
	}
	node.Resources = resources
	return nil
}

func (co *ClusterOrch) AllocateResources(req *ResourceRequest) (string, error) {
	co.mu.Lock()
	defer co.mu.Unlock()
	node := co.selectNodeForAllocationLocked(req)
	if node == nil {
		return "", ErrNoAvailableNode
	}
	co.allocateResourceLocked(node.ID, req)
	return node.ID, nil
}

func (co *ClusterOrch) ReleaseResources(nodeID string, req *ResourceRequest) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	if _, exists := co.nodes[nodeID]; !exists {
		return ErrNodeNotFound
	}
	co.releaseResourceLocked(nodeID, req)
	return nil
}

func (co *ClusterOrch) GetNodeResourceUsage(nodeID string) (map[string]float64, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()
	node, exists := co.nodes[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}
	if node.Resources == nil {
		return map[string]float64{"cpu": 0, "memory": 0, "storage": 0}, nil
	}
	return map[string]float64{
		"cpu":     safeDivide(node.Resources.CPU.Used, node.Resources.CPU.Total),
		"memory":  safeDivide(node.Resources.Memory.Used, node.Resources.Memory.Total),
		"storage": safeDivide(node.Resources.Storage.Used, node.Resources.Storage.Total),
	}, nil
}

func (co *ClusterOrch) SetResourceStrategy(strategy ResourceAllocationStrategy) {
	co.mu.Lock()
	defer co.mu.Unlock()
	co.resourceStrategy = strategy
}

func (co *ClusterOrch) GetClusterResourceSummary() map[string]int64 {
	co.mu.RLock()
	defer co.mu.RUnlock()
	summary := map[string]int64{
		"total_cpu": 0, "used_cpu": 0, "total_memory": 0, "used_memory": 0,
		"total_storage": 0, "used_storage": 0, "online_nodes": 0, "offline_nodes": 0,
		"total_services": int64(len(co.services)),
	}
	for _, node := range co.nodes {
		if node.State == NodeStateOnline {
			summary["online_nodes"]++
		} else if node.State == NodeStateOffline {
			summary["offline_nodes"]++
		}
		if node.Resources != nil {
			summary["total_cpu"] += node.Resources.CPU.Total
			summary["used_cpu"] += node.Resources.CPU.Used
			summary["total_memory"] += node.Resources.Memory.Total
			summary["used_memory"] += node.Resources.Memory.Used
			summary["total_storage"] += node.Resources.Storage.Total
			summary["used_storage"] += node.Resources.Storage.Used
		}
	}
	return summary
}

// ==================== 负载均衡 ====================
func (co *ClusterOrch) SelectNode(key string) (*Node, error) {
	co.mu.Lock()
	defer co.mu.Unlock()
	var onlineNodes []*Node
	for _, node := range co.nodes {
		if node.State == NodeStateOnline {
			nodeCopy := *node
			onlineNodes = append(onlineNodes, &nodeCopy)
		}
	}
	if len(onlineNodes) == 0 {
		return nil, ErrNoAvailableNode
	}
	// 按 ID 排序保证 map 迭代顺序不影响哈希策略
	sort.Slice(onlineNodes, func(i, j int) bool {
		return onlineNodes[i].ID < onlineNodes[j].ID
	})
	switch co.lbStrategy {
	case LBStrategyRoundRobin:
		return co.selectRoundRobinLocked(onlineNodes), nil
	case LBStrategyLeastConnections:
		return co.selectLeastConnectionsLocked(onlineNodes), nil
	case LBStrategyRandom:
		return co.selectRandomLocked(onlineNodes), nil
	case LBStrategyHash:
		return co.selectHashLocked(onlineNodes, key), nil
	default:
		return co.selectRoundRobinLocked(onlineNodes), nil
	}
}

func (co *ClusterOrch) UpdateNodeConnections(nodeID string, delta int64) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	node, exists := co.nodes[nodeID]
	if !exists {
		return ErrNodeNotFound
	}
	node.ActiveConns += delta
	if node.ActiveConns < 0 {
		node.ActiveConns = 0
	}
	return nil
}

func (co *ClusterOrch) SetLoadBalanceStrategy(strategy LoadBalanceStrategy) {
	co.mu.Lock()
	defer co.mu.Unlock()
	co.lbStrategy = strategy
}

// ==================== 配置同步 ====================
func (co *ClusterOrch) SetConfig(key, value, updatedBy, comment string) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	now := time.Now()
	var version int
	if existing, exists := co.configs[key]; exists {
		version = existing.Version + 1
		co.configHistory[key] = append(co.configHistory[key], existing)
	} else {
		version = 1
	}
	co.configs[key] = &ClusterConfig{
		Version: version, Data: map[string]string{"value": value},
		UpdatedAt: now, UpdatedBy: updatedBy, Comment: comment,
	}
	co.addLog("", "info", fmt.Sprintf("配置 %s 已更新到版本 %d", key, version))
	return nil
}

func (co *ClusterOrch) GetConfig(key string) (*ClusterConfig, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()
	cfg, exists := co.configs[key]
	if !exists {
		return nil, ErrConfigNotFound
	}
	cfgCopy := *cfg
	return &cfgCopy, nil
}

func (co *ClusterOrch) DeleteConfig(key string) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	if _, exists := co.configs[key]; !exists {
		return ErrConfigNotFound
	}
	co.configHistory[key] = append(co.configHistory[key], co.configs[key])
	delete(co.configs, key)
	co.addLog("", "info", fmt.Sprintf("配置 %s 已删除", key))
	return nil
}

func (co *ClusterOrch) RollbackConfig(key string, version int) error {
	co.mu.Lock()
	defer co.mu.Unlock()
	history, exists := co.configHistory[key]
	if !exists {
		return ErrConfigNotFound
	}
	for _, cfg := range history {
		if cfg.Version == version {
			if current, ok := co.configs[key]; ok {
				co.configHistory[key] = append(co.configHistory[key], current)
			}
			rollbackCfg := *cfg
			if current, ok := co.configs[key]; ok {
				rollbackCfg.Version = current.Version + 1
			} else {
				rollbackCfg.Version = 1
			}
			rollbackCfg.UpdatedAt = time.Now()
			rollbackCfg.Comment = fmt.Sprintf("回滚到版本 %d", version)
			co.configs[key] = &rollbackCfg
			co.addLog("", "info", fmt.Sprintf("配置 %s 已回滚到版本 %d", key, version))
			return nil
		}
	}
	return fmt.Errorf("config version %d not found for key %s", version, key)
}

func (co *ClusterOrch) GetConfigHistory(key string) ([]*ClusterConfig, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()
	history, exists := co.configHistory[key]
	if !exists {
		return nil, ErrConfigNotFound
	}
	result := make([]*ClusterConfig, len(history))
	for i, cfg := range history {
		cfgCopy := *cfg
		result[i] = &cfgCopy
	}
	return result, nil
}

func (co *ClusterOrch) SyncConfig(key string) error {
	co.mu.RLock()
	defer co.mu.RUnlock()
	if _, exists := co.configs[key]; !exists {
		return ErrConfigNotFound
	}
	co.addLog("", "info", fmt.Sprintf("配置 %s 已同步到 %d 个节点", key, len(co.nodes)))
	return nil
}

func (co *ClusterOrch) ListConfigs() map[string]*ClusterConfig {
	co.mu.RLock()
	defer co.mu.RUnlock()
	result := make(map[string]*ClusterConfig, len(co.configs))
	for k, v := range co.configs {
		cfgCopy := *v
		result[k] = &cfgCopy
	}
	return result
}

// ==================== 集群日志 ====================
func (co *ClusterOrch) AddLog(nodeID, level, message, service string) {
	co.mu.Lock()
	defer co.mu.Unlock()
	co.addLogWithService(nodeID, level, message, service)
}

func (co *ClusterOrch) QueryLogs(filter LogFilter) []*ClusterLog {
	co.mu.RLock()
	defer co.mu.RUnlock()
	var result []*ClusterLog
	for _, log := range co.logs {
		if filter.NodeID != "" && log.NodeID != filter.NodeID {
			continue
		}
		if filter.Level != "" && log.Level != filter.Level {
			continue
		}
		if filter.Service != "" && log.Service != filter.Service {
			continue
		}
		if !filter.StartTime.IsZero() && log.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && log.Timestamp.After(filter.EndTime) {
			continue
		}
		if filter.Keyword != "" && !strings.Contains(log.Message, filter.Keyword) {
			continue
		}
		logCopy := *log
		result = append(result, &logCopy)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	return result
}

func (co *ClusterOrch) GetLogStats() map[string]int {
	co.mu.RLock()
	defer co.mu.RUnlock()
	stats := map[string]int{"total": len(co.logs), "info": 0, "warn": 0, "error": 0}
	for _, log := range co.logs {
		stats[log.Level]++
	}
	return stats
}

func (co *ClusterOrch) ClearLogs() {
	co.mu.Lock()
	defer co.mu.Unlock()
	co.logs = make([]*ClusterLog, 0, co.maxLogSize)
}

// ==================== 扩缩容 ====================
func (co *ClusterOrch) ScaleOut(count int) ([]*Node, error) {
	co.mu.Lock()
	defer co.mu.Unlock()
	if !co.autoScale {
		return nil, ErrAutoScaleDisabled
	}
	currentCount := len(co.nodes)
	if currentCount+count > co.maxNodes {
		count = co.maxNodes - currentCount
		if count <= 0 {
			return nil, ErrClusterFull
		}
	}
	var newNodes []*Node
	for i := 0; i < count; i++ {
		nodeID := fmt.Sprintf("auto-node-%d-%d", time.Now().UnixNano(), i)
		node := &Node{
			ID: nodeID, Name: fmt.Sprintf("auto-node-%d", currentCount+i+1),
			Address: fmt.Sprintf("192.168.1.%d", 100+currentCount+i), State: NodeStateOnline,
			Resources: &NodeResources{
				CPU:     ResourcePool{Total: 8000, Used: 0, Available: 8000},
				Memory:  ResourcePool{Total: 16 * 1024 * 1024 * 1024, Used: 0, Available: 16 * 1024 * 1024 * 1024},
				Storage: ResourcePool{Total: 1024 * 1024 * 1024 * 1024, Used: 0, Available: 1024 * 1024 * 1024 * 1024},
			},
			Weight: 100, LastHeartbeat: time.Now(),
		}
		co.nodes[nodeID] = node
		newNodes = append(newNodes, node)
		co.addLog(nodeID, "info", fmt.Sprintf("自动扩容：节点 %s 已添加", node.Name))
	}
	return newNodes, nil
}

func (co *ClusterOrch) ScaleIn(count int) ([]*Node, error) {
	co.mu.Lock()
	defer co.mu.Unlock()
	if !co.autoScale {
		return nil, ErrAutoScaleDisabled
	}
	currentCount := len(co.nodes)
	if currentCount-count < co.minNodes {
		return nil, fmt.Errorf("cannot scale in: would go below minimum node count (%d)", co.minNodes)
	}
	type nodeLoad struct {
		id   string
		load float64
	}
	var nodeLoads []nodeLoad
	for id, node := range co.nodes {
		var load float64
		if node.Resources != nil && node.Resources.CPU.Total > 0 {
			load = float64(node.Resources.CPU.Used) / float64(node.Resources.CPU.Total)
		}
		nodeLoads = append(nodeLoads, nodeLoad{id: id, load: load})
	}
	sort.Slice(nodeLoads, func(i, j int) bool {
		return nodeLoads[i].load < nodeLoads[j].load
	})
	var removedNodes []*Node
	for i := 0; i < count && i < len(nodeLoads); i++ {
		node := co.nodes[nodeLoads[i].id]
		for _, svc := range co.services {
			if svc.NodeID == nodeLoads[i].id {
				target := co.findFailoverTargetLocked(nodeLoads[i].id)
				if target != nil {
					svc.NodeID = target.ID
					co.addLog(svc.ID, "info", fmt.Sprintf("缩容迁移：服务 %s 移到节点 %s", svc.Name, target.ID))
				}
			}
		}
		node.State = NodeStateLeaving
		removedNodes = append(removedNodes, node)
		delete(co.nodes, nodeLoads[i].id)
		co.addLog(nodeLoads[i].id, "info", fmt.Sprintf("自动缩容：节点 %s 已移除", node.Name))
	}
	return removedNodes, nil
}

func (co *ClusterOrch) SetAutoScale(enabled bool, scaleUpThreshold, scaleDownThreshold float64, minNodes, maxNodes int) {
	co.mu.Lock()
	defer co.mu.Unlock()
	co.autoScale = enabled
	if scaleUpThreshold > 0 {
		co.scaleUpThreshold = scaleUpThreshold
	}
	if scaleDownThreshold > 0 {
		co.scaleDownThreshold = scaleDownThreshold
	}
	if minNodes > 0 {
		co.minNodes = minNodes
	}
	if maxNodes > 0 {
		co.maxNodes = maxNodes
	}
}

func (co *ClusterOrch) CheckAutoScale() (string, int, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()
	if !co.autoScale {
		return "disabled", 0, nil
	}
	if len(co.nodes) == 0 {
		return "no_nodes", 0, nil
	}
	var totalUsage float64
	var nodeCount int
	for _, node := range co.nodes {
		if node.State == NodeStateOnline && node.Resources != nil && node.Resources.CPU.Total > 0 {
			totalUsage += float64(node.Resources.CPU.Used) / float64(node.Resources.CPU.Total)
			nodeCount++
		}
	}
	if nodeCount == 0 {
		return "no_data", 0, nil
	}
	avgUsage := totalUsage / float64(nodeCount)
	if avgUsage > co.scaleUpThreshold && len(co.nodes) < co.maxNodes {
		scaleCount := 1
		if len(co.nodes)+scaleCount > co.maxNodes {
			scaleCount = co.maxNodes - len(co.nodes)
		}
		return "scale_out", scaleCount, nil
	}
	if avgUsage < co.scaleDownThreshold && len(co.nodes) > co.minNodes {
		scaleCount := 1
		if len(co.nodes)-scaleCount < co.minNodes {
			scaleCount = len(co.nodes) - co.minNodes
		}
		return "scale_in", scaleCount, nil
	}
	return "stable", 0, nil
}

// ==================== 内部方法 ====================
func (co *ClusterOrch) addLog(nodeID, level, message string) {
	co.addLogWithService(nodeID, level, message, "")
}

func (co *ClusterOrch) addLogWithService(nodeID, level, message, service string) {
	log := &ClusterLog{
		ID: generateID(), NodeID: nodeID, Level: level,
		Message: message, Service: service, Timestamp: time.Now(),
	}
	co.logs = append(co.logs, log)
	if len(co.logs) > co.maxLogSize {
		co.logs = co.logs[len(co.logs)-co.maxLogSize:]
	}
}

func (co *ClusterOrch) findFailoverTargetLocked(excludeNodeID string) *Node {
	var bestNode *Node
	var bestScore float64 = -1
	for _, node := range co.nodes {
		if node.ID == excludeNodeID || node.State != NodeStateOnline {
			continue
		}
		var score float64
		if node.Resources != nil && node.Resources.CPU.Total > 0 {
			score = 1.0 - float64(node.Resources.CPU.Used)/float64(node.Resources.CPU.Total)
		} else {
			score = 1.0
		}
		if score > bestScore {
			bestScore = score
			bestNode = node
		}
	}
	return bestNode
}

func (co *ClusterOrch) checkResourceLocked(nodeID string, req *ResourceRequest) bool {
	node, exists := co.nodes[nodeID]
	if !exists || node.Resources == nil {
		return false
	}
	r := node.Resources
	return r.CPU.Available >= req.CPU && r.Memory.Available >= req.Memory && r.Storage.Available >= req.Storage
}

func (co *ClusterOrch) allocateResourceLocked(nodeID string, req *ResourceRequest) {
	node, exists := co.nodes[nodeID]
	if !exists || node.Resources == nil {
		return
	}
	r := node.Resources
	r.CPU.Used += req.CPU
	r.CPU.Available -= req.CPU
	r.Memory.Used += req.Memory
	r.Memory.Available -= req.Memory
	r.Storage.Used += req.Storage
	r.Storage.Available -= req.Storage
}

func (co *ClusterOrch) releaseResourceLocked(nodeID string, req *ResourceRequest) {
	node, exists := co.nodes[nodeID]
	if !exists || node.Resources == nil {
		return
	}
	r := node.Resources
	r.CPU.Used -= req.CPU
	if r.CPU.Used < 0 {
		r.CPU.Used = 0
	}
	r.CPU.Available = r.CPU.Total - r.CPU.Used
	r.Memory.Used -= req.Memory
	if r.Memory.Used < 0 {
		r.Memory.Used = 0
	}
	r.Memory.Available = r.Memory.Total - r.Memory.Used
	r.Storage.Used -= req.Storage
	if r.Storage.Used < 0 {
		r.Storage.Used = 0
	}
	r.Storage.Available = r.Storage.Total - r.Storage.Used
}

func (co *ClusterOrch) selectNodeForAllocationLocked(req *ResourceRequest) *Node {
	switch co.resourceStrategy {
	case ResourceStrategyBinpack:
		return co.selectBinpackLocked(req)
	case ResourceStrategySpread:
		return co.selectSpreadLocked(req)
	case ResourceStrategyRandom:
		return co.selectRandomAllocLocked(req)
	default:
		return co.selectSpreadLocked(req)
	}
}

func (co *ClusterOrch) selectBinpackLocked(req *ResourceRequest) *Node {
	var bestNode *Node
	var bestUsage float64 = -1
	for _, node := range co.nodes {
		if node.State != NodeStateOnline || !co.checkResourceLocked(node.ID, req) {
			continue
		}
		var usage float64
		if node.Resources.CPU.Total > 0 {
			usage = float64(node.Resources.CPU.Used) / float64(node.Resources.CPU.Total)
		}
		if usage > bestUsage {
			bestUsage = usage
			bestNode = node
		}
	}
	return bestNode
}

func (co *ClusterOrch) selectSpreadLocked(req *ResourceRequest) *Node {
	var bestNode *Node
	var bestUsage float64 = 2.0
	for _, node := range co.nodes {
		if node.State != NodeStateOnline || !co.checkResourceLocked(node.ID, req) {
			continue
		}
		var usage float64
		if node.Resources.CPU.Total > 0 {
			usage = float64(node.Resources.CPU.Used) / float64(node.Resources.CPU.Total)
		}
		if usage < bestUsage {
			bestUsage = usage
			bestNode = node
		}
	}
	return bestNode
}

func (co *ClusterOrch) selectRandomAllocLocked(req *ResourceRequest) *Node {
	var candidates []*Node
	for _, node := range co.nodes {
		if node.State == NodeStateOnline && co.checkResourceLocked(node.ID, req) {
			candidates = append(candidates, node)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates[rand.Intn(len(candidates))]
}

func (co *ClusterOrch) selectRoundRobinLocked(nodes []*Node) *Node {
	// 加权轮询
	totalWeight := 0
	for _, node := range nodes {
		w := node.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}
	if totalWeight == 0 {
		return nodes[0]
	}
	co.roundRobinIdx++
	target := int(co.roundRobinIdx % uint64(totalWeight))
	cumulative := 0
	for _, node := range nodes {
		w := node.Weight
		if w <= 0 {
			w = 1
		}
		cumulative += w
		if target < cumulative {
			return node
		}
	}
	return nodes[0]
}

func (co *ClusterOrch) selectLeastConnectionsLocked(nodes []*Node) *Node {
	var bestNode *Node
	var minConns int64 = 1<<63 - 1
	for _, node := range nodes {
		if node.ActiveConns < minConns {
			minConns = node.ActiveConns
			bestNode = node
		}
	}
	return bestNode
}

func (co *ClusterOrch) selectRandomLocked(nodes []*Node) *Node {
	return nodes[rand.Intn(len(nodes))]
}

func (co *ClusterOrch) selectHashLocked(nodes []*Node, key string) *Node {
	h := fnv.New32a()
	h.Write([]byte(key))
	hashVal := h.Sum32()
	return nodes[int(hashVal)%len(nodes)]
}

func safeDivide(a, b int64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Int63())
}
