// Package storage 提供存储管理功能
package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"nas-os/pkg/safeguards"
)

// ========== 分布式存储节点管理 ==========

// NodeStatus represents the operational status of a storage node.
type NodeStatus string

// NodeStatus constants define the possible states of a storage node.
const (
	// NodeStatusOnline indicates the node is fully operational and accepting requests.
	NodeStatusOnline NodeStatus = "online"
	// NodeStatusOffline indicates the node is not reachable or responding.
	NodeStatusOffline NodeStatus = "offline"
	// NodeStatusDegraded indicates the node is operational but with reduced capacity or performance.
	NodeStatusDegraded NodeStatus = "degraded"
	// NodeStatusRecovery indicates the node is currently recovering from a failure or maintenance.
	NodeStatusRecovery NodeStatus = "recovery"
	// NodeStatusMaintain indicates the node is under maintenance and not accepting new requests.
	NodeStatusMaintain NodeStatus = "maintain"
)

// Node represents a storage node in the distributed system.
type Node struct {
	ID           string            `json:"id"`           // 节点唯一标识
	Name         string            `json:"name"`         // 节点名称
	Address      string            `json:"address"`      // 节点地址 (IP:Port)
	Status       NodeStatus        `json:"status"`       // 节点状态
	Capacity     uint64            `json:"capacity"`     // 总容量（字节）
	Used         uint64            `json:"used"`         // 已用容量（字节）
	Available    uint64            `json:"available"`    // 可用容量（字节）
	Zone         string            `json:"zone"`         // 可用区/机架
	Region       string            `json:"region"`       // 地域
	Labels       map[string]string `json:"labels"`       // 自定义标签
	LastCheck    time.Time         `json:"lastCheck"`    // 最后检查时间
	LastOnline   time.Time         `json:"lastOnline"`   // 最后在线时间
	HealthScore  int               `json:"healthScore"`  // 健康评分 (0-100)
	Latency      time.Duration     `json:"latency"`      // 网络延迟
	Version      string            `json:"version"`      // 节点软件版本
	ReplicaCount int               `json:"replicaCount"` // 当前副本数量
	CreatedAt    time.Time         `json:"createdAt"`    // 加入时间
}

// NodeHealth 健康检查结果.
type NodeHealth struct {
	NodeID       string        `json:"nodeId"`
	Status       NodeStatus    `json:"status"`
	Healthy      bool          `json:"healthy"`
	ResponseTime time.Duration `json:"responseTime"`
	ErrorCount   int           `json:"errorCount"`
	LastError    string        `json:"lastError,omitempty"`
	CheckTime    time.Time     `json:"checkTime"`
	Details      HealthDetails `json:"details"`
}

// HealthDetails 健康检查详情.
type HealthDetails struct {
	DiskHealth     bool      `json:"diskHealth"`     // 磁盘健康
	MemoryUsage    float64   `json:"memoryUsage"`    // 内存使用率
	CPUUsage       float64   `json:"cpuUsage"`       // CPU 使用率
	NetworkHealthy bool      `json:"networkHealthy"` // 网络健康
	ReplicationOK  bool      `json:"replicationOk"`  // 副本同步正常
	LastSyncTime   time.Time `json:"lastSyncTime"`   // 最后同步时间
}

// ========== 分片策略 ==========

// ShardingStrategy defines the algorithm used to distribute data across shards.
type ShardingStrategy string

// ShardingStrategy constants define the available sharding algorithms.
const (
	// ShardingHash uses a hash function to distribute keys uniformly across shards.
	ShardingHash ShardingStrategy = "hash"
	// ShardingRange distributes keys based on key ranges, suitable for ordered data.
	ShardingRange ShardingStrategy = "range"
	// ShardingConsistent uses consistent hashing to minimize reshuffling when nodes change.
	ShardingConsistent ShardingStrategy = "consistent"
)

// ShardingPolicy 分片策略配置.
type ShardingPolicy struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	Strategy           ShardingStrategy `json:"strategy"`           // 分片策略
	ShardCount         int              `json:"shardCount"`         // 分片数量
	VirtualNodes       int              `json:"virtualNodes"`       // 虚拟节点数（一致性哈希）
	KeyPattern         string           `json:"keyPattern"`         // 键模式（范围分片）
	HashAlgorithm      string           `json:"hashAlgorithm"`      // 哈希算法 (md5, sha256, crc32)
	RebalanceThreshold float64          `json:"rebalanceThreshold"` // 重平衡阈值
	CreatedAt          time.Time        `json:"createdAt"`
	UpdatedAt          time.Time        `json:"updatedAt"`
}

// Shard 分片信息.
type Shard struct {
	ID           string    `json:"id"`
	PoolID       string    `json:"poolId"`
	ShardIndex   int       `json:"shardIndex"`   // 分片索引
	PrimaryNode  string    `json:"primaryNode"`  // 主节点 ID
	ReplicaNodes []string  `json:"replicaNodes"` // 副本节点 ID 列表
	KeyRange     KeyRange  `json:"keyRange"`     // 键范围（范围分片）
	Size         uint64    `json:"size"`         // 数据大小
	ObjectCount  int64     `json:"objectCount"`  // 对象数量
	Status       string    `json:"status"`       // 状态
	CreatedAt    time.Time `json:"createdAt"`
}

// KeyRange 键范围.
type KeyRange struct {
	Start string `json:"start"` // 起始键
	End   string `json:"end"`   // 结束键
}

// ========== 副本策略 ==========

// ReplicaStrategy defines the replication mode for data consistency.
type ReplicaStrategy string

// ReplicaStrategy constants define the available replication modes.
const (
	// ReplicaSync ensures writes are acknowledged only after all replicas confirm.
	ReplicaSync ReplicaStrategy = "sync"
	// ReplicaSync acknowledges writes immediately without waiting for replica confirmation.
	ReplicaAsync ReplicaStrategy = "async"
	// ReplicaSemiSync acknowledges writes after the primary and at least one replica confirm.
	ReplicaSemiSync ReplicaStrategy = "semiSync"
)

// PlacementConstraint 放置约束.
type PlacementConstraint struct {
	ZoneAware      bool              `json:"zoneAware"`      // 机架/可用区感知
	RegionAware    bool              `json:"regionAware"`    // 地域感知
	MinZones       int               `json:"minZones"`       // 最少跨可用区数
	ExcludeNodes   []string          `json:"excludeNodes"`   // 排除节点
	PreferredNodes []string          `json:"preferredNodes"` // 优先节点
	RequireLabels  map[string]string `json:"requireLabels"`  // 必需标签
}

// ReplicaPolicy 副本策略配置.
type ReplicaPolicy struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	ReplicaCount     int                 `json:"replicaCount"`     // 副本数量
	Strategy         ReplicaStrategy     `json:"strategy"`         // 复制策略
	ConsistencyLevel string              `json:"consistencyLevel"` // 一致性级别 (one, quorum, all)
	Constraints      PlacementConstraint `json:"constraints"`      // 放置约束
	WriteQuorum      int                 `json:"writeQuorum"`      // 写入仲裁
	ReadQuorum       int                 `json:"readQuorum"`       // 读取仲裁
	RepairInterval   time.Duration       `json:"repairInterval"`   // 副本修复间隔
	MaxLatency       time.Duration       `json:"maxLatency"`       // 最大允许延迟
	CreatedAt        time.Time           `json:"createdAt"`
	UpdatedAt        time.Time           `json:"updatedAt"`
}

// ReplicaStatus 副本状态.
type ReplicaStatus struct {
	ShardID      string        `json:"shardId"`
	NodeID       string        `json:"nodeId"`
	ReplicaIndex int           `json:"replicaIndex"`
	Status       string        `json:"status"`       // syncing, synced, error
	Progress     float64       `json:"progress"`     // 同步进度
	LastSyncTime time.Time     `json:"lastSyncTime"` // 最后同步时间
	BytesSynced  uint64        `json:"bytesSynced"`  // 已同步字节数
	BytesPending uint64        `json:"bytesPending"` // 待同步字节数
	Lag          time.Duration `json:"lag"`          // 副本延迟
}

// ========== 存储池管理 ==========

// PoolStatus represents the operational status of a storage pool.
type PoolStatus string

// PoolStatus constants define the possible states of a storage pool.
const (
	// PoolStatusActive indicates the pool is fully operational.
	PoolStatusActive PoolStatus = "active"
	// PoolStatusDegraded indicates the pool is operational with reduced capacity or replicas.
	PoolStatusDegraded PoolStatus = "degraded"
	// PoolStatusRecovery indicates the pool is recovering from a failure or rebalancing.
	PoolStatusRecovery PoolStatus = "recovery"
	// PoolStatusReadOnly indicates the pool accepts reads but not writes.
	PoolStatusReadOnly PoolStatus = "readOnly"
	// PoolStatusOffline indicates the pool is not available for any operations.
	PoolStatusOffline PoolStatus = "offline"
)

// Pool represents a storage pool that aggregates multiple nodes.
type Pool struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Status        PoolStatus      `json:"status"`
	Nodes         []string        `json:"nodes"`         // 节点 ID 列表
	ShardPolicy   *ShardingPolicy `json:"shardPolicy"`   // 分片策略
	ReplicaPolicy *ReplicaPolicy  `json:"replicaPolicy"` // 副本策略
	Shards        []*Shard        `json:"shards"`        // 分片列表
	Capacity      uint64          `json:"capacity"`      // 总容量
	Used          uint64          `json:"used"`          // 已用容量
	Available     uint64          `json:"available"`     // 可用容量
	ObjectCount   int64           `json:"objectCount"`   // 对象总数
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

// PoolStats 存储池统计.
type PoolStats struct {
	PoolID         string        `json:"poolId"`
	TotalNodes     int           `json:"totalNodes"`
	OnlineNodes    int           `json:"onlineNodes"`
	TotalShards    int           `json:"totalShards"`
	HealthyShards  int           `json:"healthyShards"`
	TotalReplicas  int           `json:"totalReplicas"`
	SyncedReplicas int           `json:"syncedReplicas"`
	Capacity       uint64        `json:"capacity"`
	Used           uint64        `json:"used"`
	Available      uint64        `json:"available"`
	ObjectCount    int64         `json:"objectCount"`
	AvgLatency     time.Duration `json:"avgLatency"`
	HealthScore    int           `json:"healthScore"` // 0-100
}

// ========== 分布式存储管理器 ==========

// DistributedManager 分布式存储管理器.
type DistributedManager struct {
	nodes               map[string]*Node           // 节点映射
	pools               map[string]*Pool           // 存储池映射
	shardPolicies       map[string]*ShardingPolicy // 分片策略
	replicaPolicies     map[string]*ReplicaPolicy  // 副本策略
	mu                  sync.RWMutex
	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
}

// DistributedConfig 分布式存储配置.
type DistributedConfig struct {
	HealthCheckInterval time.Duration // 健康检查间隔
	HealthCheckTimeout  time.Duration // 健康检查超时
}

// DefaultDistributedConfig 默认配置.
var DefaultDistributedConfig = DistributedConfig{
	HealthCheckInterval: 30 * time.Second,
	HealthCheckTimeout:  5 * time.Second,
}

// NewDistributedManager 创建分布式存储管理器.
func NewDistributedManager(config *DistributedConfig) *DistributedManager {
	if config == nil {
		config = &DefaultDistributedConfig
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &DistributedManager{
		nodes:               make(map[string]*Node),
		pools:               make(map[string]*Pool),
		shardPolicies:       make(map[string]*ShardingPolicy),
		replicaPolicies:     make(map[string]*ReplicaPolicy),
		healthCheckInterval: config.HealthCheckInterval,
		healthCheckTimeout:  config.HealthCheckTimeout,
		ctx:                 ctx,
		cancel:              cancel,
	}
}

// Start 启动管理器.
func (dm *DistributedManager) Start() error {
	dm.wg.Add(1)
	go dm.healthCheckLoop()
	return nil
}

// Stop 停止管理器.
func (dm *DistributedManager) Stop() {
	dm.cancel()
	dm.wg.Wait()
}

// healthCheckLoop 健康检查循环.
func (dm *DistributedManager) healthCheckLoop() {
	defer dm.wg.Done()

	ticker := time.NewTicker(dm.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-dm.ctx.Done():
			return
		case <-ticker.C:
			dm.runHealthChecks()
		}
	}
}

// runHealthChecks 执行健康检查.
func (dm *DistributedManager) runHealthChecks() {
	dm.mu.RLock()
	nodes := make([]*Node, 0, len(dm.nodes))
	for _, node := range dm.nodes {
		nodes = append(nodes, node)
	}
	dm.mu.RUnlock()

	for _, node := range nodes {
		health := dm.checkNodeHealth(node)
		dm.updateNodeHealth(node.ID, health)
	}
}

// checkNodeHealth 检查节点健康状态.
func (dm *DistributedManager) checkNodeHealth(node *Node) *NodeHealth {
	ctx, cancel := context.WithTimeout(dm.ctx, dm.healthCheckTimeout)
	defer cancel()

	start := time.Now()
	health := &NodeHealth{
		NodeID:    node.ID,
		CheckTime: start,
		Details:   HealthDetails{},
	}

	// 模拟健康检查（实际实现需要调用节点 API）
	select {
	case <-ctx.Done():
		health.Status = NodeStatusOffline
		health.Healthy = false
		health.LastError = "health check timeout"
		health.ErrorCount++
		health.ResponseTime = time.Since(start)
		return health
	default:
		// 实际实现：发送健康检查请求
		// 这里模拟检查结果
		health.ResponseTime = time.Since(start)
		health.Status = NodeStatusOnline
		health.Healthy = true
		health.Details.DiskHealth = true
		health.Details.MemoryUsage = 0.5
		health.Details.CPUUsage = 0.3
		health.Details.NetworkHealthy = true
		health.Details.ReplicationOK = true
		health.Details.LastSyncTime = time.Now()
		return health
	}
}

// updateNodeHealth 更新节点健康状态.
func (dm *DistributedManager) updateNodeHealth(nodeID string, health *NodeHealth) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	node, exists := dm.nodes[nodeID]
	if !exists {
		return
	}

	node.LastCheck = health.CheckTime
	node.Status = health.Status
	node.HealthScore = calculateHealthScore(health)
	node.Latency = health.ResponseTime

	if health.Healthy {
		node.LastOnline = health.CheckTime
	}
}

// calculateHealthScore 计算健康评分.
func calculateHealthScore(health *NodeHealth) int {
	if !health.Healthy {
		return 0
	}

	score := 100

	// 根据响应时间扣分
	if health.ResponseTime > 100*time.Millisecond {
		score -= 10
	}
	if health.ResponseTime > 500*time.Millisecond {
		score -= 20
	}

	// 根据资源使用扣分
	if health.Details.MemoryUsage > 0.8 {
		score -= 10
	}
	if health.Details.CPUUsage > 0.8 {
		score -= 10
	}

	// 根据错误次数扣分
	score -= health.ErrorCount * 5

	if score < 0 {
		score = 0
	}

	return score
}

// ========== 节点管理 ==========

// RegisterNode 注册节点.
func (dm *DistributedManager) RegisterNode(node *Node) error {
	if node.ID == "" {
		return fmt.Errorf("节点 ID 不能为空")
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.nodes[node.ID]; exists {
		return fmt.Errorf("节点 %s 已存在", node.ID)
	}

	node.CreatedAt = time.Now()
	node.LastCheck = time.Now()
	if node.Status == "" {
		node.Status = NodeStatusOnline
	}
	if node.HealthScore == 0 {
		node.HealthScore = 100
	}
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	dm.nodes[node.ID] = node
	return nil
}

// UnregisterNode 注销节点.
func (dm *DistributedManager) UnregisterNode(nodeID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.nodes[nodeID]; !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	// 检查节点是否属于某个存储池
	for _, pool := range dm.pools {
		for _, nid := range pool.Nodes {
			if nid == nodeID {
				return fmt.Errorf("节点 %s 正被存储池 %s 使用", nodeID, pool.Name)
			}
		}
	}

	delete(dm.nodes, nodeID)
	return nil
}

// GetNode 获取节点.
func (dm *DistributedManager) GetNode(nodeID string) (*Node, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	node, exists := dm.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("节点 %s 不存在", nodeID)
	}

	return node, nil
}

// ListNodes 列出所有节点.
func (dm *DistributedManager) ListNodes() []*Node {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	nodes := make([]*Node, 0, len(dm.nodes))
	for _, node := range dm.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// ListNodesByStatus 按状态列出节点.
func (dm *DistributedManager) ListNodesByStatus(status NodeStatus) []*Node {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	nodes := make([]*Node, 0)
	for _, node := range dm.nodes {
		if node.Status == status {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// UpdateNode 更新节点信息.
func (dm *DistributedManager) UpdateNode(nodeID string, updates map[string]interface{}) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	node, exists := dm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	for key, value := range updates {
		switch key {
		case "name":
			if v, ok := value.(string); ok {
				node.Name = v
			}
		case "address":
			if v, ok := value.(string); ok {
				node.Address = v
			}
		case "zone":
			if v, ok := value.(string); ok {
				node.Zone = v
			}
		case "region":
			if v, ok := value.(string); ok {
				node.Region = v
			}
		case "capacity":
			if v, ok := value.(uint64); ok {
				node.Capacity = v
			}
		case "used":
			if v, ok := value.(uint64); ok {
				node.Used = v
				node.Available = node.Capacity - node.Used
			}
		case "labels":
			if v, ok := value.(map[string]string); ok {
				node.Labels = v
			}
		}
	}

	return nil
}

// ========== 分片策略管理 ==========

// CreateShardingPolicy 创建分片策略.
func (dm *DistributedManager) CreateShardingPolicy(policy *ShardingPolicy) error {
	if policy.ID == "" {
		return fmt.Errorf("策略 ID 不能为空")
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.shardPolicies[policy.ID]; exists {
		return fmt.Errorf("分片策略 %s 已存在", policy.ID)
	}

	// 验证策略
	if policy.ShardCount <= 0 {
		return fmt.Errorf("分片数量必须大于 0")
	}

	if policy.Strategy == ShardingConsistent && policy.VirtualNodes <= 0 {
		policy.VirtualNodes = 150 // 默认虚拟节点数
	}

	if policy.HashAlgorithm == "" {
		policy.HashAlgorithm = "crc32"
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	dm.shardPolicies[policy.ID] = policy
	return nil
}

// GetShardingPolicy 获取分片策略.
func (dm *DistributedManager) GetShardingPolicy(policyID string) (*ShardingPolicy, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	policy, exists := dm.shardPolicies[policyID]
	if !exists {
		return nil, fmt.Errorf("分片策略 %s 不存在", policyID)
	}

	return policy, nil
}

// ListShardingPolicies 列出所有分片策略.
func (dm *DistributedManager) ListShardingPolicies() []*ShardingPolicy {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	policies := make([]*ShardingPolicy, 0, len(dm.shardPolicies))
	for _, p := range dm.shardPolicies {
		policies = append(policies, p)
	}
	return policies
}

// DeleteShardingPolicy 删除分片策略.
func (dm *DistributedManager) DeleteShardingPolicy(policyID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.shardPolicies[policyID]; !exists {
		return fmt.Errorf("分片策略 %s 不存在", policyID)
	}

	// 检查是否有存储池在使用
	for _, pool := range dm.pools {
		if pool.ShardPolicy != nil && pool.ShardPolicy.ID == policyID {
			return fmt.Errorf("分片策略正在被存储池 %s 使用", pool.Name)
		}
	}

	delete(dm.shardPolicies, policyID)
	return nil
}

// ========== 副本策略管理 ==========

// CreateReplicaPolicy 创建副本策略.
func (dm *DistributedManager) CreateReplicaPolicy(policy *ReplicaPolicy) error {
	if policy.ID == "" {
		return fmt.Errorf("策略 ID 不能为空")
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.replicaPolicies[policy.ID]; exists {
		return fmt.Errorf("副本策略 %s 已存在", policy.ID)
	}

	// 验证策略
	if policy.ReplicaCount < 1 {
		return fmt.Errorf("副本数量必须至少为 1")
	}

	if policy.ReplicaCount > 1 && policy.Strategy == "" {
		policy.Strategy = ReplicaSync
	}

	if policy.ConsistencyLevel == "" {
		policy.ConsistencyLevel = "quorum"
	}

	// 计算默认仲裁值
	if policy.WriteQuorum == 0 {
		policy.WriteQuorum = (policy.ReplicaCount + 1) / 2
	}
	if policy.ReadQuorum == 0 {
		policy.ReadQuorum = 1
	}

	if policy.RepairInterval == 0 {
		policy.RepairInterval = 1 * time.Hour
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	dm.replicaPolicies[policy.ID] = policy
	return nil
}

// GetReplicaPolicy 获取副本策略.
func (dm *DistributedManager) GetReplicaPolicy(policyID string) (*ReplicaPolicy, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	policy, exists := dm.replicaPolicies[policyID]
	if !exists {
		return nil, fmt.Errorf("副本策略 %s 不存在", policyID)
	}

	return policy, nil
}

// ListReplicaPolicies 列出所有副本策略.
func (dm *DistributedManager) ListReplicaPolicies() []*ReplicaPolicy {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	policies := make([]*ReplicaPolicy, 0, len(dm.replicaPolicies))
	for _, p := range dm.replicaPolicies {
		policies = append(policies, p)
	}
	return policies
}

// DeleteReplicaPolicy 删除副本策略.
func (dm *DistributedManager) DeleteReplicaPolicy(policyID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.replicaPolicies[policyID]; !exists {
		return fmt.Errorf("副本策略 %s 不存在", policyID)
	}

	// 检查是否有存储池在使用
	for _, pool := range dm.pools {
		if pool.ReplicaPolicy != nil && pool.ReplicaPolicy.ID == policyID {
			return fmt.Errorf("副本策略正在被存储池 %s 使用", pool.Name)
		}
	}

	delete(dm.replicaPolicies, policyID)
	return nil
}

// ========== 存储池管理 ==========

// CreatePool 创建存储池.
func (dm *DistributedManager) CreatePool(pool *Pool) error {
	if pool.ID == "" {
		return fmt.Errorf("存储池 ID 不能为空")
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.pools[pool.ID]; exists {
		return fmt.Errorf("存储池 %s 已存在", pool.ID)
	}

	// 验证节点
	for _, nodeID := range pool.Nodes {
		if _, exists := dm.nodes[nodeID]; !exists {
			return fmt.Errorf("节点 %s 不存在", nodeID)
		}
	}

	// 验证分片策略
	if pool.ShardPolicy != nil {
		if _, exists := dm.shardPolicies[pool.ShardPolicy.ID]; !exists {
			return fmt.Errorf("分片策略 %s 不存在", pool.ShardPolicy.ID)
		}
	}

	// 验证副本策略
	if pool.ReplicaPolicy != nil {
		if _, exists := dm.replicaPolicies[pool.ReplicaPolicy.ID]; !exists {
			return fmt.Errorf("副本策略 %s 不存在", pool.ReplicaPolicy.ID)
		}
	}

	now := time.Now()
	pool.CreatedAt = now
	pool.UpdatedAt = now
	pool.Status = PoolStatusActive

	if pool.Shards == nil {
		pool.Shards = make([]*Shard, 0)
	}

	// 计算容量
	dm.calculatePoolCapacity(pool)

	dm.pools[pool.ID] = pool
	return nil
}

// GetPool 获取存储池.
func (dm *DistributedManager) GetPool(poolID string) (*Pool, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	pool, exists := dm.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}

	return pool, nil
}

// ListPools 列出所有存储池.
func (dm *DistributedManager) ListPools() []*Pool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	pools := make([]*Pool, 0, len(dm.pools))
	for _, p := range dm.pools {
		pools = append(pools, p)
	}
	return pools
}

// DeletePool 删除存储池.
func (dm *DistributedManager) DeletePool(poolID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.pools[poolID]; !exists {
		return fmt.Errorf("存储池 %s 不存在", poolID)
	}

	delete(dm.pools, poolID)
	return nil
}

// AddNodeToPool 添加节点到存储池.
func (dm *DistributedManager) AddNodeToPool(poolID, nodeID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	pool, exists := dm.pools[poolID]
	if !exists {
		return fmt.Errorf("存储池 %s 不存在", poolID)
	}

	if _, exists := dm.nodes[nodeID]; !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	// 检查是否已在池中
	for _, nid := range pool.Nodes {
		if nid == nodeID {
			return fmt.Errorf("节点 %s 已在存储池中", nodeID)
		}
	}

	pool.Nodes = append(pool.Nodes, nodeID)
	pool.UpdatedAt = time.Now()
	dm.calculatePoolCapacity(pool)

	return nil
}

// RemoveNodeFromPool 从存储池移除节点.
func (dm *DistributedManager) RemoveNodeFromPool(poolID, nodeID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	pool, exists := dm.pools[poolID]
	if !exists {
		return fmt.Errorf("存储池 %s 不存在", poolID)
	}

	found := false
	newNodes := make([]string, 0, len(pool.Nodes)-1)
	for _, nid := range pool.Nodes {
		if nid == nodeID {
			found = true
			continue
		}
		newNodes = append(newNodes, nid)
	}

	if !found {
		return fmt.Errorf("节点 %s 不在存储池中", nodeID)
	}

	pool.Nodes = newNodes
	pool.UpdatedAt = time.Now()
	dm.calculatePoolCapacity(pool)

	return nil
}

// GetPoolStats 获取存储池统计.
func (dm *DistributedManager) GetPoolStats(poolID string) (*PoolStats, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	pool, exists := dm.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}

	stats := &PoolStats{
		PoolID:         poolID,
		TotalNodes:     len(pool.Nodes),
		TotalShards:    len(pool.Shards),
		TotalReplicas:  0,
		SyncedReplicas: 0,
		Capacity:       pool.Capacity,
		Used:           pool.Used,
		Available:      pool.Available,
		ObjectCount:    pool.ObjectCount,
		AvgLatency:     0,
		HealthScore:    100,
	}

	// 统计在线节点
	for _, nodeID := range pool.Nodes {
		if node, exists := dm.nodes[nodeID]; exists {
			if node.Status == NodeStatusOnline {
				stats.OnlineNodes++
				stats.AvgLatency += node.Latency
			}
			if node.HealthScore < stats.HealthScore {
				stats.HealthScore = node.HealthScore
			}
		}
	}

	if stats.OnlineNodes > 0 {
		stats.AvgLatency = stats.AvgLatency / time.Duration(stats.OnlineNodes)
	}

	// 统计分片和副本
	for _, shard := range pool.Shards {
		if shard.Status == "active" {
			stats.HealthyShards++
		}
		stats.TotalReplicas += len(shard.ReplicaNodes) + 1 // 包括主节点
	}

	// 根据节点健康度调整评分
	if stats.OnlineNodes < stats.TotalNodes {
		stats.HealthScore = stats.HealthScore * stats.OnlineNodes / stats.TotalNodes
	}

	return stats, nil
}

// calculatePoolCapacity 计算存储池容量.
func (dm *DistributedManager) calculatePoolCapacity(pool *Pool) {
	var totalCapacity, totalUsed uint64

	for _, nodeID := range pool.Nodes {
		if node, exists := dm.nodes[nodeID]; exists {
			totalCapacity += node.Capacity
			totalUsed += node.Used
		}
	}

	pool.Capacity = totalCapacity
	pool.Used = totalUsed
	pool.Available = totalCapacity - totalUsed
}

// SetPoolShardingPolicy 设置存储池分片策略.
func (dm *DistributedManager) SetPoolShardingPolicy(poolID, policyID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	pool, exists := dm.pools[poolID]
	if !exists {
		return fmt.Errorf("存储池 %s 不存在", poolID)
	}

	policy, exists := dm.shardPolicies[policyID]
	if !exists {
		return fmt.Errorf("分片策略 %s 不存在", policyID)
	}

	pool.ShardPolicy = policy
	pool.UpdatedAt = time.Now()

	return nil
}

// SetPoolReplicaPolicy 设置存储池副本策略.
func (dm *DistributedManager) SetPoolReplicaPolicy(poolID, policyID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	pool, exists := dm.pools[poolID]
	if !exists {
		return fmt.Errorf("存储池 %s 不存在", poolID)
	}

	policy, exists := dm.replicaPolicies[policyID]
	if !exists {
		return fmt.Errorf("副本策略 %s 不存在", policyID)
	}

	pool.ReplicaPolicy = policy
	pool.UpdatedAt = time.Now()

	return nil
}

// ========== 分片管理 ==========

// AllocateShards 为存储池分配分片.
func (dm *DistributedManager) AllocateShards(poolID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	pool, exists := dm.pools[poolID]
	if !exists {
		return fmt.Errorf("存储池 %s 不存在", poolID)
	}

	if pool.ShardPolicy == nil {
		return fmt.Errorf("存储池未配置分片策略")
	}

	if pool.ReplicaPolicy == nil {
		return fmt.Errorf("存储池未配置副本策略")
	}

	// 获取可用节点
	availableNodes := make([]*Node, 0)
	for _, nodeID := range pool.Nodes {
		if node, exists := dm.nodes[nodeID]; exists && node.Status == NodeStatusOnline {
			availableNodes = append(availableNodes, node)
		}
	}

	if len(availableNodes) < pool.ReplicaPolicy.ReplicaCount {
		return fmt.Errorf("可用节点不足，需要 %d 个节点，当前 %d 个",
			pool.ReplicaPolicy.ReplicaCount, len(availableNodes))
	}

	// 创建分片
	shardCount := pool.ShardPolicy.ShardCount
	pool.Shards = make([]*Shard, 0, shardCount)

	for i := 0; i < shardCount; i++ {
		shard := &Shard{
			ID:           fmt.Sprintf("%s-shard-%d", poolID, i),
			PoolID:       poolID,
			ShardIndex:   i,
			ReplicaNodes: make([]string, 0),
			Status:       "active",
			CreatedAt:    time.Now(),
		}

		// 选择主节点（根据分片策略）
		primaryIdx := i % len(availableNodes)
		shard.PrimaryNode = availableNodes[primaryIdx].ID

		// 选择副本节点
		replicaCount := pool.ReplicaPolicy.ReplicaCount - 1
		if replicaCount > 0 {
			for j := 0; j < replicaCount; j++ {
				idx := (primaryIdx + j + 1) % len(availableNodes)
				shard.ReplicaNodes = append(shard.ReplicaNodes, availableNodes[idx].ID)
			}
		}

		pool.Shards = append(pool.Shards, shard)
	}

	pool.UpdatedAt = time.Now()
	return nil
}

// GetShard 获取分片.
func (dm *DistributedManager) GetShard(poolID, shardID string) (*Shard, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	pool, exists := dm.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}

	for _, shard := range pool.Shards {
		if shard.ID == shardID {
			return shard, nil
		}
	}

	return nil, fmt.Errorf("分片 %s 不存在", shardID)
}

// ListShards 列出存储池的所有分片.
func (dm *DistributedManager) ListShards(poolID string) ([]*Shard, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	pool, exists := dm.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}

	return pool.Shards, nil
}

// RebalanceShards 重新平衡分片.
func (dm *DistributedManager) RebalanceShards(poolID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	pool, exists := dm.pools[poolID]
	if !exists {
		return fmt.Errorf("存储池 %s 不存在", poolID)
	}

	if pool.ShardPolicy == nil {
		return fmt.Errorf("存储池未配置分片策略")
	}

	// 获取当前可用节点
	availableNodes := make([]*Node, 0)
	for _, nodeID := range pool.Nodes {
		if node, exists := dm.nodes[nodeID]; exists && node.Status == NodeStatusOnline {
			availableNodes = append(availableNodes, node)
		}
	}

	if len(availableNodes) == 0 {
		return fmt.Errorf("没有可用节点进行重新平衡")
	}

	// 重新分配分片
	for i, shard := range pool.Shards {
		primaryIdx := i % len(availableNodes)
		newPrimary := availableNodes[primaryIdx].ID

		// 如果主节点变化，需要迁移
		if shard.PrimaryNode != newPrimary {
			oldPrimary := shard.PrimaryNode
			shard.Status = "migrating"

			// 执行数据迁移（异步）
			go dm.migrateShardData(poolID, shard.ID, oldPrimary, newPrimary)

			shard.PrimaryNode = newPrimary
		}

		// 重新选择副本节点
		replicaCount := pool.ReplicaPolicy.ReplicaCount - 1
		if replicaCount > 0 {
			newReplicas := make([]string, 0, replicaCount)
			for j := 0; j < replicaCount; j++ {
				idx := (primaryIdx + j + 1) % len(availableNodes)
				newReplicas = append(newReplicas, availableNodes[idx].ID)
			}
			shard.ReplicaNodes = newReplicas
		}
	}

	pool.UpdatedAt = time.Now()
	return nil
}

// migrateShardData 迁移分片数据.
func (dm *DistributedManager) migrateShardData(poolID, shardID, sourceNode, targetNode string) {
	// 记录迁移开始
	migration := &MigrationTask{
		PoolID:     poolID,
		ShardID:    shardID,
		SourceNode: sourceNode,
		TargetNode: targetNode,
		Status:     "running",
		StartTime:  time.Now(),
	}

	dm.mu.Lock()
	if pool, exists := dm.pools[poolID]; exists {
		for _, shard := range pool.Shards {
			if shard.ID == shardID {
				shard.Status = "migrating"
				break
			}
		}
	}
	dm.mu.Unlock()

	// 实际迁移逻辑（需要根据具体存储后端实现）
	// 1. 连接源节点和目标节点
	// 2. 传输数据
	// 3. 验证数据完整性
	// 4. 更新元数据
	// 这里模拟迁移完成
	migration.EndTime = time.Now()
	migration.Status = "completed"

	// 更新分片状态
	dm.mu.Lock()
	if pool, exists := dm.pools[poolID]; exists {
		for _, shard := range pool.Shards {
			if shard.ID == shardID {
				shard.Status = "active"
				break
			}
		}
	}
	dm.mu.Unlock()
}

// MigrationTask 迁移任务.
type MigrationTask struct {
	PoolID     string    `json:"poolId"`
	ShardID    string    `json:"shardId"`
	SourceNode string    `json:"sourceNode"`
	TargetNode string    `json:"targetNode"`
	Status     string    `json:"status"` // running, completed, failed
	StartTime  time.Time `json:"startTime"`
	EndTime    time.Time `json:"endTime,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// ========== 健康检查接口 ==========

// CheckNode 手动检查节点健康.
func (dm *DistributedManager) CheckNode(nodeID string) (*NodeHealth, error) {
	dm.mu.RLock()
	node, exists := dm.nodes[nodeID]
	dm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("节点 %s 不存在", nodeID)
	}

	health := dm.checkNodeHealth(node)
	dm.updateNodeHealth(nodeID, health)
	return health, nil
}

// CheckPool 检查存储池健康.
func (dm *DistributedManager) CheckPool(poolID string) (*PoolStats, error) {
	return dm.GetPoolStats(poolID)
}

// GetClusterHealth 获取集群整体健康状态.
func (dm *DistributedManager) GetClusterHealth() *ClusterHealth {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	health := &ClusterHealth{
		CheckTime: time.Now(),
	}

	totalNodes := len(dm.nodes)
	onlineNodes := 0
	totalCapacity := uint64(0)
	totalUsed := uint64(0)

	for _, node := range dm.nodes {
		if node.Status == NodeStatusOnline {
			onlineNodes++
		}
		totalCapacity += node.Capacity
		totalUsed += node.Used
	}

	health.TotalNodes = totalNodes
	health.OnlineNodes = onlineNodes
	health.OfflineNodes = totalNodes - onlineNodes
	health.TotalCapacity = totalCapacity
	health.UsedCapacity = totalUsed
	health.AvailableCapacity = totalCapacity - totalUsed
	health.TotalPools = len(dm.pools)

	healthyPools := 0
	for _, pool := range dm.pools {
		if pool.Status == PoolStatusActive {
			healthyPools++
		}
	}
	health.HealthyPools = healthyPools

	if totalNodes > 0 {
		health.HealthScore = onlineNodes * 100 / totalNodes
	}

	return health
}

// ClusterHealth 集群健康状态.
type ClusterHealth struct {
	CheckTime         time.Time `json:"checkTime"`
	TotalNodes        int       `json:"totalNodes"`
	OnlineNodes       int       `json:"onlineNodes"`
	OfflineNodes      int       `json:"offlineNodes"`
	TotalPools        int       `json:"totalPools"`
	HealthyPools      int       `json:"healthyPools"`
	TotalCapacity     uint64    `json:"totalCapacity"`
	UsedCapacity      uint64    `json:"usedCapacity"`
	AvailableCapacity uint64    `json:"availableCapacity"`
	HealthScore       int       `json:"healthScore"` // 0-100
}

// ========== 数据放置接口 ==========

// GetNodeForKey 根据键获取应该存放的节点（一致性哈希）.
func (dm *DistributedManager) GetNodeForKey(poolID, key string) (*Node, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	pool, exists := dm.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}

	if pool.ShardPolicy == nil {
		return nil, fmt.Errorf("存储池未配置分片策略")
	}

	if len(pool.Shards) == 0 {
		return nil, fmt.Errorf("存储池没有分片")
	}

	// 根据策略选择分片
	var shardIndex int
	switch pool.ShardPolicy.Strategy {
	case ShardingHash, ShardingConsistent:
		// 简单哈希取模
		hash := uint32(0)
		for _, c := range key {
			// 安全转换：rune 可能是负数，使用安全的转换方式
			if c >= 0 {
				hash = hash*31 + uint32(c)
			} else {
				hash = hash*31 + uint32(c&0x7FFFFFFF) // 取绝对值的低31位
			}
		}
		// 安全转换：hash % uint32(...) 结果不会超过 uint32 最大值
		shardIndexVal := hash % uint32(len(pool.Shards))
		if idx, err := safeguards.SafeUint64ToInt(uint64(shardIndexVal)); err == nil {
			shardIndex = idx
		} else {
			shardIndex = 0 // 溢出时使用默认分片
		}
	case ShardingRange:
		// 范围分片
		shardIndex = 0
		for i, shard := range pool.Shards {
			if key >= shard.KeyRange.Start && key < shard.KeyRange.End {
				shardIndex = i
				break
			}
		}
	default:
		shardIndex = 0
	}

	shard := pool.Shards[shardIndex]

	// 返回主节点
	node, exists := dm.nodes[shard.PrimaryNode]
	if !exists {
		return nil, fmt.Errorf("节点 %s 不存在", shard.PrimaryNode)
	}

	return node, nil
}

// GetReplicaNodes 获取副本节点列表.
func (dm *DistributedManager) GetReplicaNodes(poolID, key string) ([]*Node, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	pool, exists := dm.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}

	if pool.ShardPolicy == nil {
		return nil, fmt.Errorf("存储池未配置分片策略")
	}

	if len(pool.Shards) == 0 {
		return nil, fmt.Errorf("存储池没有分片")
	}

	// 根据策略选择分片
	var shardIndex int
	switch pool.ShardPolicy.Strategy {
	case ShardingHash, ShardingConsistent:
		hash := uint32(0)
		for _, c := range key {
			// 安全转换：rune 可能是负数，使用安全的转换方式
			if c >= 0 {
				hash = hash*31 + uint32(c)
			} else {
				hash = hash*31 + uint32(c&0x7FFFFFFF) // 取绝对值的低31位
			}
		}
		// 安全转换：hash % uint32(...) 结果不会超过 uint32 最大值
		shardIndexVal := hash % uint32(len(pool.Shards))
		if idx, err := safeguards.SafeUint64ToInt(uint64(shardIndexVal)); err == nil {
			shardIndex = idx
		} else {
			shardIndex = 0 // 溢出时使用默认分片
		}
	default:
		shardIndex = 0
	}

	shard := pool.Shards[shardIndex]

	// 收集所有副本节点
	nodes := make([]*Node, 0, len(shard.ReplicaNodes)+1)

	// 主节点
	if node, exists := dm.nodes[shard.PrimaryNode]; exists {
		nodes = append(nodes, node)
	}

	// 副本节点
	for _, nodeID := range shard.ReplicaNodes {
		if node, exists := dm.nodes[nodeID]; exists {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}
