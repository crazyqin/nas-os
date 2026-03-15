package storage

import (
	"testing"
	"time"
)

// ========== 分布式管理器创建测试 ==========

func TestNewDistributedManager(t *testing.T) {
	dm := NewDistributedManager(nil)
	if dm == nil {
		t.Fatal("NewDistributedManager returned nil")
	}

	if dm.nodes == nil {
		t.Error("nodes map should be initialized")
	}
	if dm.pools == nil {
		t.Error("pools map should be initialized")
	}
	if dm.shardPolicies == nil {
		t.Error("shardPolicies map should be initialized")
	}
	if dm.replicaPolicies == nil {
		t.Error("replicaPolicies map should be initialized")
	}
}

func TestNewDistributedManager_WithConfig(t *testing.T) {
	config := &DistributedConfig{
		HealthCheckInterval: 10 * time.Second,
		HealthCheckTimeout:  3 * time.Second,
	}

	dm := NewDistributedManager(config)
	if dm == nil {
		t.Fatal("NewDistributedManager returned nil")
	}

	if dm.healthCheckInterval != 10*time.Second {
		t.Errorf("Expected HealthCheckInterval=10s, got %v", dm.healthCheckInterval)
	}
}

func TestDistributedManager_StartStop(t *testing.T) {
	dm := NewDistributedManager(nil)

	// 启动
	if err := dm.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 停止
	dm.Stop()
}

// ========== 节点管理测试 ==========

func TestRegisterNode(t *testing.T) {
	dm := NewDistributedManager(nil)

	node := &StorageNode{
		ID:        "node-1",
		Name:      "storage-node-1",
		Address:   "192.168.1.100:9000",
		Status:    NodeStatusOnline,
		Capacity:  1000000000000,
		Available: 800000000000,
		Zone:      "zone-a",
		Region:    "us-east-1",
	}

	err := dm.RegisterNode(node)
	if err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	// 验证节点已注册
	registered, _ := dm.GetNode("node-1")
	if registered == nil {
		t.Fatal("Node not found after registration")
	}
	if registered.Name != "storage-node-1" {
		t.Errorf("Expected Name=storage-node-1, got %s", registered.Name)
	}
}

func TestRegisterNode_Duplicate(t *testing.T) {
	dm := NewDistributedManager(nil)

	node := &StorageNode{
		ID:      "node-1",
		Name:    "storage-node-1",
		Address: "192.168.1.100:9000",
	}

	// 第一次注册
	_ = dm.RegisterNode(node)

	// 重复注册应该失败
	err := dm.RegisterNode(node)
	if err == nil {
		t.Error("Expected error for duplicate node registration")
	}
}

func TestUnregisterNode(t *testing.T) {
	dm := NewDistributedManager(nil)

	node := &StorageNode{
		ID:      "node-1",
		Name:    "storage-node-1",
		Address: "192.168.1.100:9000",
	}
	_ = dm.RegisterNode(node)

	// 取消注册
	err := dm.UnregisterNode("node-1")
	if err != nil {
		t.Fatalf("UnregisterNode failed: %v", err)
	}

	// 验证节点已删除
	registered, _ := dm.GetNode("node-1")
	if registered != nil {
		t.Error("Node should be unregistered")
	}
}

func TestUnregisterNode_NotFound(t *testing.T) {
	dm := NewDistributedManager(nil)

	err := dm.UnregisterNode("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent node")
	}
}

func TestGetNode(t *testing.T) {
	dm := NewDistributedManager(nil)

	node := &StorageNode{
		ID:      "node-1",
		Name:    "storage-node-1",
		Address: "192.168.1.100:9000",
	}
	_ = dm.RegisterNode(node)

	// 获取存在的节点
	found, _ := dm.GetNode("node-1")
	if found == nil {
		t.Fatal("Node not found")
	}

	// 获取不存在的节点
	notFound, _ := dm.GetNode("nonexistent")
	if notFound != nil {
		t.Error("Expected nil for nonexistent node")
	}
}

func TestListNodes(t *testing.T) {
	dm := NewDistributedManager(nil)

	// 空列表
	nodes := dm.ListNodes()
	if len(nodes) != 0 {
		t.Errorf("Expected empty list, got %d nodes", len(nodes))
	}

	// 添加节点
	for i := 0; i < 3; i++ {
		_ = dm.RegisterNode(&StorageNode{
			ID:      string(rune('A' + i)),
			Name:    "node-" + string(rune('A'+i)),
			Address: "192.168.1.100:9000",
		})
	}

	nodes = dm.ListNodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}
}

func TestListNodesByStatus(t *testing.T) {
	dm := NewDistributedManager(nil)

	// 添加不同状态的节点
	_ = dm.RegisterNode(&StorageNode{ID: "1", Status: NodeStatusOnline})
	_ = dm.RegisterNode(&StorageNode{ID: "2", Status: NodeStatusOnline})
	_ = dm.RegisterNode(&StorageNode{ID: "3", Status: NodeStatusOffline})
	_ = dm.RegisterNode(&StorageNode{ID: "4", Status: NodeStatusDegraded})

	onlineNodes := dm.ListNodesByStatus(NodeStatusOnline)
	if len(onlineNodes) != 2 {
		t.Errorf("Expected 2 online nodes, got %d", len(onlineNodes))
	}

	offlineNodes := dm.ListNodesByStatus(NodeStatusOffline)
	if len(offlineNodes) != 1 {
		t.Errorf("Expected 1 offline node, got %d", len(offlineNodes))
	}
}

func TestUpdateNode(t *testing.T) {
	dm := NewDistributedManager(nil)

	node := &StorageNode{
		ID:        "node-1",
		Name:      "storage-node-1",
		Capacity:  1000000000000,
		Available: 800000000000,
	}
	_ = dm.RegisterNode(node)

	// 更新节点
	updates := map[string]interface{}{
		"name":     "updated-node",
		"capacity": uint64(2000000000000),
		"available": uint64(1500000000000),
	}

	err := dm.UpdateNode("node-1", updates)
	if err != nil {
		t.Fatalf("UpdateNode failed: %v", err)
	}

	// 验证更新
	found, _ := dm.GetNode("node-1")
	if found.Name != "updated-node" {
		t.Errorf("Expected Name=updated-node, got %s", found.Name)
	}
	if found.Capacity != 2000000000000 {
		t.Errorf("Expected Capacity=2000000000000, got %d", found.Capacity)
	}
}

// ========== 分片策略测试 ==========

func TestCreateShardingPolicy(t *testing.T) {
	dm := NewDistributedManager(nil)

	policy := &ShardingPolicy{
		ID:          "policy-1",
		Name:        "hash-policy",
		Strategy:    ShardingHash,
		ShardCount:  16,
		HashAlgorithm: "md5",
	}

	err := dm.CreateShardingPolicy(policy)
	if err != nil {
		t.Fatalf("CreateShardingPolicy failed: %v", err)
	}

	// 验证策略已创建
	found, _ := dm.GetShardingPolicy("policy-1")
	if found == nil {
		t.Fatal("Policy not found after creation")
	}
}

func TestGetShardingPolicy_NotFound(t *testing.T) {
	dm := NewDistributedManager(nil)

	found, _ := dm.GetShardingPolicy("nonexistent")
	if found != nil {
		t.Error("Expected nil for nonexistent policy")
	}
}

func TestListShardingPolicies(t *testing.T) {
	dm := NewDistributedManager(nil)

	// 空列表
	policies := dm.ListShardingPolicies()
	if len(policies) != 0 {
		t.Errorf("Expected empty list, got %d", len(policies))
	}

	// 添加策略
	dm.CreateShardingPolicy(&ShardingPolicy{ID: "1", Name: "policy-1"})
	dm.CreateShardingPolicy(&ShardingPolicy{ID: "2", Name: "policy-2"})

	policies = dm.ListShardingPolicies()
	if len(policies) != 2 {
		t.Errorf("Expected 2 policies, got %d", len(policies))
	}
}

func TestDeleteShardingPolicy(t *testing.T) {
	dm := NewDistributedManager(nil)

	policy := &ShardingPolicy{ID: "policy-1", Name: "test-policy"}
	_ = dm.CreateShardingPolicy(policy)

	err := dm.DeleteShardingPolicy("policy-1")
	if err != nil {
		t.Fatalf("DeleteShardingPolicy failed: %v", err)
	}

	// 验证策略已删除
	found, _ := dm.GetShardingPolicy("policy-1")
	if found != nil {
		t.Error("Policy should be deleted")
	}
}

// ========== 副本策略测试 ==========

func TestCreateReplicaPolicy(t *testing.T) {
	dm := NewDistributedManager(nil)

	policy := &ReplicaPolicy{
		ID:           "replica-1",
		Name:         "three-replica",
		ReplicaCount: 3,
		Strategy:     ReplicaSync,
		WriteQuorum:  2,
		ReadQuorum:   2,
	}

	err := dm.CreateReplicaPolicy(policy)
	if err != nil {
		t.Fatalf("CreateReplicaPolicy failed: %v", err)
	}

	// 验证策略已创建
	found, _ := dm.GetReplicaPolicy("replica-1")
	if found == nil {
		t.Fatal("Policy not found after creation")
	}
	if found.ReplicaCount != 3 {
		t.Errorf("Expected ReplicaCount=3, got %d", found.ReplicaCount)
	}
}

func TestGetReplicaPolicy_NotFound(t *testing.T) {
	dm := NewDistributedManager(nil)

	found, _ := dm.GetReplicaPolicy("nonexistent")
	if found != nil {
		t.Error("Expected nil for nonexistent policy")
	}
}

func TestListReplicaPolicies(t *testing.T) {
	dm := NewDistributedManager(nil)

	dm.CreateReplicaPolicy(&ReplicaPolicy{ID: "1", Name: "policy-1"})
	dm.CreateReplicaPolicy(&ReplicaPolicy{ID: "2", Name: "policy-2"})

	policies := dm.ListReplicaPolicies()
	if len(policies) != 2 {
		t.Errorf("Expected 2 policies, got %d", len(policies))
	}
}

func TestDeleteReplicaPolicy(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.CreateReplicaPolicy(&ReplicaPolicy{ID: "replica-1", Name: "test"})

	err := dm.DeleteReplicaPolicy("replica-1")
	if err != nil {
		t.Fatalf("DeleteReplicaPolicy failed: %v", err)
	}

	found, _ := dm.GetReplicaPolicy("replica-1")
	if found != nil {
		t.Error("Policy should be deleted")
	}
}

// ========== 存储池测试 ==========

func TestCreatePool(t *testing.T) {
	dm := NewDistributedManager(nil)

	// 先注册节点
	_ = dm.RegisterNode(&StorageNode{ID: "node-1", Capacity: 1000000000000})
	_ = dm.RegisterNode(&StorageNode{ID: "node-2", Capacity: 1000000000000})

	pool := &StoragePool{
		ID:          "pool-1",
		Name:        "main-pool",
		Description: "Primary storage pool",
		Nodes:       []string{"node-1", "node-2"},
	}

	err := dm.CreatePool(pool)
	if err != nil {
		t.Fatalf("CreatePool failed: %v", err)
	}

	// 验证池已创建
	found, _ := dm.GetPool("pool-1")
	if found == nil {
		t.Fatal("Pool not found after creation")
	}
}

func TestGetPool_NotFound(t *testing.T) {
	dm := NewDistributedManager(nil)

	found, _ := dm.GetPool("nonexistent")
	if found != nil {
		t.Error("Expected nil for nonexistent pool")
	}
}

func TestListPools(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{ID: "node-1"})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1"}})
	_ = dm.CreatePool(&StoragePool{ID: "pool-2", Name: "pool-2", Nodes: []string{"node-1"}})

	pools := dm.ListPools()
	if len(pools) != 2 {
		t.Errorf("Expected 2 pools, got %d", len(pools))
	}
}

func TestDeletePool(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{ID: "node-1"})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1"}})

	err := dm.DeletePool("pool-1")
	if err != nil {
		t.Fatalf("DeletePool failed: %v", err)
	}

	found, _ := dm.GetPool("pool-1")
	if found != nil {
		t.Error("Pool should be deleted")
	}
}

func TestAddNodeToPool(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{ID: "node-1"})
	_ = dm.RegisterNode(&StorageNode{ID: "node-2"})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1"}})

	err := dm.AddNodeToPool("pool-1", "node-2")
	if err != nil {
		t.Fatalf("AddNodeToPool failed: %v", err)
	}

	pool, _ := dm.GetPool("pool-1")
	if len(pool.Nodes) != 2 {
		t.Errorf("Expected 2 nodes in pool, got %d", len(pool.Nodes))
	}
}

func TestRemoveNodeFromPool(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{ID: "node-1"})
	_ = dm.RegisterNode(&StorageNode{ID: "node-2"})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1", "node-2"}})

	err := dm.RemoveNodeFromPool("pool-1", "node-2")
	if err != nil {
		t.Fatalf("RemoveNodeFromPool failed: %v", err)
	}

	pool, _ := dm.GetPool("pool-1")
	if len(pool.Nodes) != 1 {
		t.Errorf("Expected 1 node in pool, got %d", len(pool.Nodes))
	}
}

func TestGetPoolStats(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{
		ID:        "node-1",
		Capacity:  1000000000000,
		Available: 800000000000,
	})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1"}})

	stats, err := dm.GetPoolStats("pool-1")
	if err != nil {
		t.Fatalf("GetPoolStats failed: %v", err)
	}

	if stats.TotalNodes != 1 {
		t.Errorf("Expected TotalNodes=1, got %d", stats.TotalNodes)
	}
}

// ========== 分片测试 ==========

func TestAllocateShards(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{ID: "node-1", Status: NodeStatusOnline})
	_ = dm.RegisterNode(&StorageNode{ID: "node-2", Status: NodeStatusOnline})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1", "node-2"}})

	shards, err := dm.AllocateShards("pool-1", 4)
	if err != nil {
		t.Fatalf("AllocateShards failed: %v", err)
	}

	if len(shards) != 4 {
		t.Errorf("Expected 4 shards, got %d", len(shards))
	}
}

func TestGetShard(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{ID: "node-1", Status: NodeStatusOnline})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1"}})
	_ = dm.CreateShardingPolicy(&ShardingPolicy{ID: "policy-1"})
	_ = dm.SetPoolShardingPolicy("pool-1", "policy-1")
	shards, _ := dm.AllocateShards("pool-1", 2)

	if len(shards) > 0 {
		found, _ := dm.GetShard("pool-1", shards[0].ID)
		if found == nil {
			t.Error("Shard not found")
		}
	}
}

func TestListShards(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{ID: "node-1", Status: NodeStatusOnline})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1"}})

	// 空池
	shards := dm.ListShards("pool-1")
	if len(shards) != 0 {
		t.Errorf("Expected empty list, got %d", len(shards))
	}

	// 分配分片
	_, _ = dm.AllocateShards("pool-1", 3)
	shards = dm.ListShards("pool-1")
	if len(shards) != 3 {
		t.Errorf("Expected 3 shards, got %d", len(shards))
	}
}

// ========== 健康检查测试 ==========

func TestCheckNode(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{ID: "node-1", Status: NodeStatusOnline})

	health := dm.CheckNode("node-1")
	if health == nil {
		t.Fatal("CheckNode returned nil")
	}
}

func TestCheckPool(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{ID: "node-1"})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1"}})

	health := dm.CheckPool("pool-1")
	if health == nil {
		t.Fatal("CheckPool returned nil")
	}
}

func TestGetClusterHealth(t *testing.T) {
	dm := NewDistributedManager(nil)

	// 空集群
	health := dm.GetClusterHealth()
	if health == nil {
		t.Fatal("GetClusterHealth returned nil")
	}

	// 添加节点和池
	_ = dm.RegisterNode(&StorageNode{ID: "node-1", Status: NodeStatusOnline})
	_ = dm.RegisterNode(&StorageNode{ID: "node-2", Status: NodeStatusOffline})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1"}})

	health = dm.GetClusterHealth()
	if health.TotalNodes != 2 {
		t.Errorf("Expected TotalNodes=2, got %d", health.TotalNodes)
	}
}

// ========== 节点选择测试 ==========

func TestGetNodeForKey(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{ID: "node-1", Status: NodeStatusOnline})
	_ = dm.RegisterNode(&StorageNode{ID: "node-2", Status: NodeStatusOnline})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1", "node-2"}})

	node := dm.GetNodeForKey("pool-1", "test-key")
	if node == "" {
		t.Error("GetNodeForKey should return a node")
	}
}

func TestGetReplicaNodes(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{ID: "node-1", Status: NodeStatusOnline, Zone: "zone-a"})
	_ = dm.RegisterNode(&StorageNode{ID: "node-2", Status: NodeStatusOnline, Zone: "zone-b"})
	_ = dm.RegisterNode(&StorageNode{ID: "node-3", Status: NodeStatusOnline, Zone: "zone-c"})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1", "node-2", "node-3"}})

	nodes, err := dm.GetReplicaNodes("pool-1", "test-key", 2)
	if err != nil {
		t.Fatalf("GetReplicaNodes failed: %v", err)
	}

	if len(nodes) != 2 {
		t.Errorf("Expected 2 replica nodes, got %d", len(nodes))
	}
}

// ========== 数据类型测试 ==========

func TestNodeStatus_Values(t *testing.T) {
	statuses := []NodeStatus{
		NodeStatusOnline,
		NodeStatusOffline,
		NodeStatusDegraded,
		NodeStatusRecovery,
		NodeStatusMaintain,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("NodeStatus should not be empty")
		}
	}
}

func TestShardingStrategy_Values(t *testing.T) {
	strategies := []ShardingStrategy{
		ShardingHash,
		ShardingRange,
		ShardingConsistent,
	}

	for _, strategy := range strategies {
		if strategy == "" {
			t.Error("ShardingStrategy should not be empty")
		}
	}
}

func TestReplicaStrategy_Values(t *testing.T) {
	strategies := []ReplicaStrategy{
		ReplicaSync,
		ReplicaAsync,
		ReplicaSemiSync,
	}

	for _, strategy := range strategies {
		if strategy == "" {
			t.Error("ReplicaStrategy should not be empty")
		}
	}
}

func TestPoolStatus_Values(t *testing.T) {
	statuses := []PoolStatus{
		PoolStatusActive,
		PoolStatusDegraded,
		PoolStatusRecovery,
		PoolStatusReadOnly,
		PoolStatusOffline,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("PoolStatus should not be empty")
		}
	}
}

// ========== 存储节点结构测试 ==========

func TestStorageNode_Struct(t *testing.T) {
	node := &StorageNode{
		ID:          "node-1",
		Name:        "test-node",
		Address:     "192.168.1.100:9000",
		Status:      NodeStatusOnline,
		Capacity:    1000000000000,
		Used:        200000000000,
		Available:   800000000000,
		Zone:        "zone-a",
		Region:      "us-east-1",
		HealthScore: 95,
		Version:     "1.0.0",
		Labels: map[string]string{
			"env": "production",
		},
	}

	if node.ID != "node-1" {
		t.Errorf("Expected ID=node-1, got %s", node.ID)
	}
	if node.HealthScore != 95 {
		t.Errorf("Expected HealthScore=95, got %d", node.HealthScore)
	}
}

func TestShardingPolicy_Struct(t *testing.T) {
	policy := &ShardingPolicy{
		ID:                 "policy-1",
		Name:               "hash-policy",
		Strategy:           ShardingHash,
		ShardCount:         16,
		VirtualNodes:       150,
		HashAlgorithm:      "md5",
		RebalanceThreshold: 0.8,
	}

	if policy.ShardCount != 16 {
		t.Errorf("Expected ShardCount=16, got %d", policy.ShardCount)
	}
}

func TestReplicaPolicy_Struct(t *testing.T) {
	policy := &ReplicaPolicy{
		ID:               "replica-1",
		Name:             "three-replica",
		ReplicaCount:     3,
		Strategy:         ReplicaSync,
		ConsistencyLevel: "quorum",
		WriteQuorum:      2,
		ReadQuorum:       2,
		RepairInterval:   1 * time.Hour,
		MaxLatency:       100 * time.Millisecond,
		Constraints: PlacementConstraint{
			ZoneAware: true,
			MinZones:  2,
		},
	}

	if policy.ReplicaCount != 3 {
		t.Errorf("Expected ReplicaCount=3, got %d", policy.ReplicaCount)
	}
	if !policy.Constraints.ZoneAware {
		t.Error("Expected ZoneAware=true")
	}
}

func TestStoragePool_Struct(t *testing.T) {
	pool := &StoragePool{
		ID:          "pool-1",
		Name:        "main-pool",
		Description: "Primary storage pool",
		Status:      PoolStatusActive,
		Nodes:       []string{"node-1", "node-2"},
		Capacity:    2000000000000,
		Used:        500000000000,
		Available:   1500000000000,
		ObjectCount: 1000000,
	}

	if len(pool.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(pool.Nodes))
	}
}

func TestShard_Struct(t *testing.T) {
	shard := &Shard{
		ID:           "shard-1",
		PoolID:       "pool-1",
		ShardIndex:   0,
		PrimaryNode:  "node-1",
		ReplicaNodes: []string{"node-2", "node-3"},
		Size:         1000000000,
		ObjectCount:  5000,
		Status:       "active",
	}

	if shard.ShardIndex != 0 {
		t.Errorf("Expected ShardIndex=0, got %d", shard.ShardIndex)
	}
	if len(shard.ReplicaNodes) != 2 {
		t.Errorf("Expected 2 replica nodes, got %d", len(shard.ReplicaNodes))
	}
}

func TestNodeHealth_Struct(t *testing.T) {
	health := &NodeHealth{
		NodeID:       "node-1",
		Status:       NodeStatusOnline,
		Healthy:      true,
		ResponseTime: 10 * time.Millisecond,
		ErrorCount:   0,
		Details: HealthDetails{
			DiskHealth:     true,
			MemoryUsage:    50.0,
			CPUUsage:       30.0,
			NetworkHealthy: true,
			ReplicationOK:  true,
		},
	}

	if !health.Healthy {
		t.Error("Expected Healthy=true")
	}
	if health.Details.MemoryUsage != 50.0 {
		t.Errorf("Expected MemoryUsage=50.0, got %f", health.Details.MemoryUsage)
	}
}

func TestPoolStats_Struct(t *testing.T) {
	stats := &PoolStats{
		PoolID:         "pool-1",
		TotalNodes:     3,
		OnlineNodes:    2,
		TotalShards:    16,
		HealthyShards:  15,
		TotalReplicas:  48,
		SyncedReplicas: 45,
		Capacity:       3000000000000,
		Used:           1000000000000,
		Available:      2000000000000,
		ObjectCount:    1000000,
		AvgLatency:     10 * time.Millisecond,
		HealthScore:    90,
	}

	if stats.HealthScore != 90 {
		t.Errorf("Expected HealthScore=90, got %d", stats.HealthScore)
	}
}

// ========== 并发访问测试 ==========

func TestDistributedManager_ConcurrentAccess(t *testing.T) {
	dm := NewDistributedManager(nil)

	done := make(chan bool)

	// 并发注册节点
	for i := 0; i < 10; i++ {
		go func(i int) {
			_ = dm.RegisterNode(&StorageNode{
				ID:   string(rune('A' + i)),
				Name: "node",
			})
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			_ = dm.ListNodes()
			done <- true
		}()
	}

	// 等待所有操作完成
	for i := 0; i < 20; i++ {
		<-done
	}
}

// ========== 重平衡测试 ==========

func TestRebalanceShards(t *testing.T) {
	dm := NewDistributedManager(nil)

	_ = dm.RegisterNode(&StorageNode{ID: "node-1", Status: NodeStatusOnline})
	_ = dm.RegisterNode(&StorageNode{ID: "node-2", Status: NodeStatusOnline})
	_ = dm.RegisterNode(&StorageNode{ID: "node-3", Status: NodeStatusOnline})
	_ = dm.CreatePool(&StoragePool{ID: "pool-1", Name: "pool-1", Nodes: []string{"node-1", "node-2", "node-3"}})
	_, _ = dm.AllocateShards("pool-1", 6)

	// 执行重平衡
	err := dm.RebalanceShards("pool-1")
	if err != nil {
		t.Logf("RebalanceShards: %v", err)
	}
}