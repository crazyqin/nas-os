package clustermgr

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Service 测试 ==========

func TestService_CreateCluster(t *testing.T) {
	svc := NewService()

	t.Run("成功创建集群", func(t *testing.T) {
		req := &CreateClusterRequest{
			Name:           "test-cluster",
			LeaderNodeName: "node-1",
			LeaderAddress:  "192.168.1.100",
			LeaderPort:     8080,
		}

		result, err := svc.CreateCluster(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.ID)
		assert.Equal(t, "test-cluster", result.Name)
		assert.Equal(t, ClusterHealthy, result.Status)
		assert.Equal(t, 1, result.NodeCount)
		assert.Equal(t, 1, result.HealthyNodes)
		assert.NotEmpty(t, result.LeaderID)
	})

	t.Run("空集群名称", func(t *testing.T) {
		req := &CreateClusterRequest{
			Name:           "",
			LeaderNodeName: "node-1",
			LeaderAddress:  "192.168.1.100",
		}

		_, err := svc.CreateCluster(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "集群名称不能为空")
	})

	t.Run("空主节点地址", func(t *testing.T) {
		req := &CreateClusterRequest{
			Name:           "test-cluster-2",
			LeaderNodeName: "node-1",
			LeaderAddress:  "",
		}

		_, err := svc.CreateCluster(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "主节点地址不能为空")
	})

	t.Run("默认端口", func(t *testing.T) {
		req := &CreateClusterRequest{
			Name:           "auto-port-cluster",
			LeaderNodeName: "leader",
			LeaderAddress:  "10.0.0.1",
		}

		result, err := svc.CreateCluster(context.Background(), req)
		require.NoError(t, err)

		nodes, err := svc.GetNodes(result.ID)
		require.NoError(t, err)
		assert.Equal(t, 8080, nodes.Nodes[0].Port)
	})
}

func TestService_GetCluster(t *testing.T) {
	svc := NewService()

	// 先创建集群
	cluster, err := svc.CreateCluster(context.Background(), &CreateClusterRequest{
		Name:           "get-test",
		LeaderNodeName: "leader",
		LeaderAddress:  "192.168.1.1",
	})
	require.NoError(t, err)

	t.Run("成功获取", func(t *testing.T) {
		result, err := svc.GetCluster(cluster.ID)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, cluster.ID, result.ID)
		assert.Equal(t, "get-test", result.Name)
	})

	t.Run("集群不存在", func(t *testing.T) {
		_, err := svc.GetCluster("non-existent")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrClusterNotFound)
	})
}

func TestService_AddNode(t *testing.T) {
	svc := NewService()

	cluster, err := svc.CreateCluster(context.Background(), &CreateClusterRequest{
		Name:           "add-node-test",
		LeaderNodeName: "leader",
		LeaderAddress:  "192.168.1.1",
	})
	require.NoError(t, err)

	t.Run("成功添加从节点", func(t *testing.T) {
		req := &AddNodeRequest{
			ClusterID: cluster.ID,
			Name:      "follower-1",
			Address:   "192.168.1.2",
			Role:      RoleFollower,
		}

		result, err := svc.AddNode(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.ID)
		assert.Equal(t, RoleFollower, result.Role)
		assert.Equal(t, NodeOnline, result.Status)
	})

	t.Run("成功添加见证节点", func(t *testing.T) {
		req := &AddNodeRequest{
			ClusterID: cluster.ID,
			Name:      "witness-1",
			Address:   "192.168.1.3",
			Role:      RoleWitness,
		}

		result, err := svc.AddNode(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, RoleWitness, result.Role)
	})

	t.Run("重复节点名称", func(t *testing.T) {
		req := &AddNodeRequest{
			ClusterID: cluster.ID,
			Name:      "follower-1", // 已存在
			Address:   "192.168.1.4",
			Role:      RoleFollower,
		}

		_, err := svc.AddNode(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNodeAlreadyExists)
	})

	t.Run("无效节点角色", func(t *testing.T) {
		req := &AddNodeRequest{
			ClusterID: cluster.ID,
			Name:      "bad-node",
			Address:   "192.168.1.5",
			Role:      "invalid-role",
		}

		_, err := svc.AddNode(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidNodeRole)
	})

	t.Run("集群不存在", func(t *testing.T) {
		req := &AddNodeRequest{
			ClusterID: "non-existent",
			Name:      "node-x",
			Address:   "192.168.1.6",
			Role:      RoleFollower,
		}

		_, err := svc.AddNode(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrClusterNotFound)
	})
}

func TestService_RemoveNode(t *testing.T) {
	svc := NewService()

	cluster, err := svc.CreateCluster(context.Background(), &CreateClusterRequest{
		Name:           "remove-test",
		LeaderNodeName: "leader",
		LeaderAddress:  "192.168.1.1",
	})
	require.NoError(t, err)

	// 添加一个从节点
	addReq := &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "follower-1",
		Address:   "192.168.1.2",
		Role:      RoleFollower,
	}
	follower, err := svc.AddNode(context.Background(), addReq)
	require.NoError(t, err)

	t.Run("成功移除从节点", func(t *testing.T) {
		req := &RemoveNodeRequest{
			ClusterID: cluster.ID,
			NodeID:    follower.ID,
		}

		result, err := svc.RemoveNode(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Contains(t, result.Message, "已移除")

		// 验证节点已被删除
		nodes, err := svc.GetNodes(cluster.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, len(nodes.Nodes))
	})

	t.Run("不能移除主节点", func(t *testing.T) {
		req := &RemoveNodeRequest{
			ClusterID: cluster.ID,
			NodeID:    cluster.LeaderID,
		}

		_, err := svc.RemoveNode(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "不能移除主节点")
	})

	t.Run("强制移除主节点", func(t *testing.T) {
		req := &RemoveNodeRequest{
			ClusterID: cluster.ID,
			NodeID:    cluster.LeaderID,
			Force:     true,
		}

		result, err := svc.RemoveNode(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("节点不存在", func(t *testing.T) {
		req := &RemoveNodeRequest{
			ClusterID: cluster.ID,
			NodeID:    "non-existent",
		}

		_, err := svc.RemoveNode(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNodeNotFound)
	})
}

func TestService_MigrateWorkload(t *testing.T) {
	svc := NewService()

	cluster, err := svc.CreateCluster(context.Background(), &CreateClusterRequest{
		Name:           "migrate-test",
		LeaderNodeName: "leader",
		LeaderAddress:  "192.168.1.1",
	})
	require.NoError(t, err)

	// 添加两个从节点
	node1, err := svc.AddNode(context.Background(), &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "worker-1",
		Address:   "192.168.1.2",
		Role:      RoleFollower,
	})
	require.NoError(t, err)

	node2, err := svc.AddNode(context.Background(), &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "worker-2",
		Address:   "192.168.1.3",
		Role:      RoleFollower,
	})
	require.NoError(t, err)

	// 给 node1 添加工作负载
	node1.WorkloadCount = 3

	t.Run("成功迁移工作负载", func(t *testing.T) {
		req := &MigrateWorkloadRequest{
			ClusterID:    cluster.ID,
			WorkloadID:   "wl-001",
			TargetNodeID: node2.ID,
			Reason:       "负载均衡",
		}

		result, err := svc.MigrateWorkload(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.MigrationID)
		assert.Equal(t, MigrationCompleted, result.Status)
		assert.Equal(t, float64(100), result.Progress)
		assert.Contains(t, result.Message, "迁移")
	})

	t.Run("目标节点不存在", func(t *testing.T) {
		req := &MigrateWorkloadRequest{
			ClusterID:    cluster.ID,
			WorkloadID:   "wl-002",
			TargetNodeID: "non-existent",
		}

		_, err := svc.MigrateWorkload(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNodeNotFound)
	})

	t.Run("集群不存在", func(t *testing.T) {
		req := &MigrateWorkloadRequest{
			ClusterID:    "non-existent",
			WorkloadID:   "wl-003",
			TargetNodeID: node2.ID,
		}

		_, err := svc.MigrateWorkload(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrClusterNotFound)
	})
}

func TestService_GetMigration(t *testing.T) {
	svc := NewService()

	cluster, err := svc.CreateCluster(context.Background(), &CreateClusterRequest{
		Name:           "get-mig-test",
		LeaderNodeName: "leader",
		LeaderAddress:  "192.168.1.1",
	})
	require.NoError(t, err)

	node1, err := svc.AddNode(context.Background(), &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "worker-1",
		Address:   "192.168.1.2",
		Role:      RoleFollower,
	})
	require.NoError(t, err)

	node2, err := svc.AddNode(context.Background(), &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "worker-2",
		Address:   "192.168.1.3",
		Role:      RoleFollower,
	})
	require.NoError(t, err)

	node1.WorkloadCount = 1

	migResult, err := svc.MigrateWorkload(context.Background(), &MigrateWorkloadRequest{
		ClusterID:    cluster.ID,
		WorkloadID:   "wl-test",
		TargetNodeID: node2.ID,
	})
	require.NoError(t, err)

	t.Run("成功获取迁移状态", func(t *testing.T) {
		result, err := svc.GetMigration(cluster.ID, migResult.MigrationID)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, MigrationCompleted, result.Status)
		assert.Equal(t, "wl-test", result.WorkloadID)
	})

	t.Run("迁移不存在", func(t *testing.T) {
		_, err := svc.GetMigration(cluster.ID, "non-existent")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrMigrationNotFound)
	})
}

func TestService_QoSRule(t *testing.T) {
	svc := NewService()

	cluster, err := svc.CreateCluster(context.Background(), &CreateClusterRequest{
		Name:           "qos-test",
		LeaderNodeName: "leader",
		LeaderAddress:  "192.168.1.1",
	})
	require.NoError(t, err)

	node, err := svc.AddNode(context.Background(), &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "worker-1",
		Address:   "192.168.1.2",
		Role:      RoleFollower,
	})
	require.NoError(t, err)

	t.Run("成功创建 QoS 规则", func(t *testing.T) {
		req := &CreateQoSRuleRequest{
			ClusterID: cluster.ID,
			Name:      "cpu-limit",
			Category:  QoSCPU,
			NodeID:    node.ID,
			Limit:     80,
			Action:    QoSActionThrottle,
			Priority:  75,
		}

		result, err := svc.CreateQoSRule(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.ID)
		assert.Equal(t, "cpu-limit", result.Name)
		assert.True(t, result.Enabled)
		assert.Equal(t, 75, result.Priority)
	})

	t.Run("创建全局 QoS 规则（不指定节点）", func(t *testing.T) {
		req := &CreateQoSRuleRequest{
			ClusterID: cluster.ID,
			Name:      "net-limit-global",
			Category:  QoSNetwork,
			Limit:     1000,
			Action:    QoSActionQueue,
		}

		result, err := svc.CreateQoSRule(context.Background(), req)
		require.NoError(t, err)
		assert.Empty(t, result.NodeID)
		assert.Equal(t, 50, result.Priority) // 默认优先级
	})

	t.Run("指定不存在节点", func(t *testing.T) {
		req := &CreateQoSRuleRequest{
			ClusterID: cluster.ID,
			Name:      "bad-qos",
			Category:  QoSCPU,
			NodeID:    "non-existent",
			Limit:     50,
			Action:    QoSActionReject,
		}

		_, err := svc.CreateQoSRule(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNodeNotFound)
	})

	t.Run("获取 QoS 规则列表", func(t *testing.T) {
		result, err := svc.GetQoSRules(cluster.ID)
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("删除 QoS 规则", func(t *testing.T) {
		rules, err := svc.GetQoSRules(cluster.ID)
		require.NoError(t, err)
		require.Len(t, rules, 2)

		err = svc.DeleteQoSRule(cluster.ID, rules[0].ID)
		require.NoError(t, err)

		rules, err = svc.GetQoSRules(cluster.ID)
		require.NoError(t, err)
		assert.Len(t, rules, 1)
	})

	t.Run("删除不存在的 QoS 规则", func(t *testing.T) {
		err := svc.DeleteQoSRule(cluster.ID, "non-existent")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrQoSRuleNotFound)
	})
}

func TestService_Protection(t *testing.T) {
	svc := NewService()

	cluster, err := svc.CreateCluster(context.Background(), &CreateClusterRequest{
		Name:           "protection-test",
		LeaderNodeName: "leader",
		LeaderAddress:  "192.168.1.1",
	})
	require.NoError(t, err)

	node1, err := svc.AddNode(context.Background(), &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "worker-1",
		Address:   "192.168.1.2",
		Role:      RoleFollower,
	})
	require.NoError(t, err)

	node2, err := svc.AddNode(context.Background(), &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "worker-2",
		Address:   "192.168.1.3",
		Role:      RoleFollower,
	})
	require.NoError(t, err)

	t.Run("成功创建保护策略", func(t *testing.T) {
		req := &CreateProtectionRequest{
			ClusterID:    cluster.ID,
			Name:         "failover-protection",
			Type:         ProtectionFailover,
			Level:        ProtectionFull,
			NodeIDs:      []string{node1.ID, node2.ID},
			AutoFailover: true,
			ReplicaCount: 2,
		}

		result, err := svc.CreateProtection(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.ID)
		assert.True(t, result.Enabled)
		assert.True(t, result.AutoFailover)
		assert.Equal(t, 2, result.ReplicaCount)
	})

	t.Run("不存在的节点", func(t *testing.T) {
		req := &CreateProtectionRequest{
			ClusterID: cluster.ID,
			Name:      "bad-protection",
			Type:      ProtectionReplication,
			Level:     ProtectionPartial,
			NodeIDs:   []string{"non-existent"},
		}

		_, err := svc.CreateProtection(context.Background(), req)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNodeNotFound)
	})

	t.Run("获取保护策略列表", func(t *testing.T) {
		result, err := svc.GetProtections(cluster.ID)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "failover-protection", result[0].Name)
	})
}

func TestService_CheckNodeHealth(t *testing.T) {
	svc := NewService()

	cluster, err := svc.CreateCluster(context.Background(), &CreateClusterRequest{
		Name:           "health-test",
		LeaderNodeName: "leader",
		LeaderAddress:  "192.168.1.1",
	})
	require.NoError(t, err)

	node, err := svc.AddNode(context.Background(), &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "worker-1",
		Address:   "192.168.1.2",
		Role:      RoleFollower,
	})
	require.NoError(t, err)

	// 给节点添加工作负载以测试 CPU 使用率
	node.WorkloadCount = 5

	t.Run("成功检查健康状态", func(t *testing.T) {
		result, err := svc.CheckNodeHealth(context.Background(), cluster.ID)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, cluster.ID, result.ClusterID)
		assert.NotEmpty(t, result.Nodes)
		assert.True(t, len(result.Nodes) >= 2) // leader + worker

		// 验证健康数据
		for _, h := range result.Nodes {
			assert.GreaterOrEqual(t, h.CPUUsage, float64(0))
			assert.LessOrEqual(t, h.CPUUsage, float64(100))
			assert.GreaterOrEqual(t, h.MemoryUsage, float64(0))
			assert.LessOrEqual(t, h.MemoryUsage, float64(100))
			assert.NotZero(t, h.CheckedAt)
		}
	})

	t.Run("集群不存在", func(t *testing.T) {
		_, err := svc.CheckNodeHealth(context.Background(), "non-existent")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrClusterNotFound)
	})

	t.Run("高负载节点触发警告", func(t *testing.T) {
		// 添加高负载节点
		highNode, err := svc.AddNode(context.Background(), &AddNodeRequest{
			ClusterID: cluster.ID,
			Name:      "high-load-node",
			Address:   "192.168.1.3",
			Role:      RoleFollower,
		})
		require.NoError(t, err)
		highNode.WorkloadCount = 10 // 25 + 10*10 = 125 -> 100

		result, err := svc.CheckNodeHealth(context.Background(), cluster.ID)
		require.NoError(t, err)

		// 应有节点 CPU 超过 80
		foundHighCPU := false
		for _, h := range result.Nodes {
			if h.CPUUsage > 80 {
				foundHighCPU = true
				assert.NotEmpty(t, h.Errors)
			}
		}
		assert.True(t, foundHighCPU, "应检测到高 CPU 使用率节点")
		assert.Equal(t, ClusterCritical, result.OverallStatus)
	})
}

func TestService_ListClusters(t *testing.T) {
	svc := NewService()

	// 初始应为空
	clusters, err := svc.ListClusters()
	require.NoError(t, err)
	assert.Empty(t, clusters)

	// 创建几个集群
	for i := 0; i < 3; i++ {
		_, err := svc.CreateCluster(context.Background(), &CreateClusterRequest{
			Name:           fmt.Sprintf("cluster-%d", i),
			LeaderNodeName: "leader",
			LeaderAddress:  fmt.Sprintf("192.168.1.%d", i+1),
		})
		require.NoError(t, err)
	}

	clusters, err = svc.ListClusters()
	require.NoError(t, err)
	assert.Len(t, clusters, 3)
}

func TestService_FaultTolerance(t *testing.T) {
	svc := NewService()

	cluster, err := svc.CreateCluster(context.Background(), &CreateClusterRequest{
		Name:           "ft-test",
		LeaderNodeName: "leader",
		LeaderAddress:  "192.168.1.1",
	})
	require.NoError(t, err)

	// 1 个节点 -> 容忍度 0
	c, err := svc.GetCluster(cluster.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, c.FaultTolerance)

	// 添加 1 个节点 -> 2 个在线 -> (2-1)/2 = 0
	svc.AddNode(context.Background(), &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "n2",
		Address:   "192.168.1.2",
		Role:      RoleFollower,
	})
	c, err = svc.GetCluster(cluster.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, c.FaultTolerance)

	// 添加第 3 个节点 -> 3 个在线 -> (3-1)/2 = 1
	svc.AddNode(context.Background(), &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "n3",
		Address:   "192.168.1.3",
		Role:      RoleFollower,
	})
	c, err = svc.GetCluster(cluster.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, c.FaultTolerance)

	// 添加第 4、5 个节点 -> 5 个在线 -> (5-1)/2 = 2
	svc.AddNode(context.Background(), &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "n4",
		Address:   "192.168.1.4",
		Role:      RoleFollower,
	})
	svc.AddNode(context.Background(), &AddNodeRequest{
		ClusterID: cluster.ID,
		Name:      "n5",
		Address:   "192.168.1.5",
		Role:      RoleFollower,
	})
	c, err = svc.GetCluster(cluster.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, c.FaultTolerance)
}

// ========== Handler 测试 ==========

func TestHandler_RegisterRoutes(t *testing.T) {
	svc := NewService()
	h := NewHandler(svc)
	assert.NotNil(t, h)
}

// ========== 常量测试 ==========

func TestConstants(t *testing.T) {
	assert.Equal(t, NodeRole("leader"), RoleLeader)
	assert.Equal(t, NodeRole("follower"), RoleFollower)
	assert.Equal(t, NodeRole("witness"), RoleWitness)
	assert.Equal(t, NodeRole("standby"), RoleStandby)

	assert.Equal(t, NodeStatus("online"), NodeOnline)
	assert.Equal(t, NodeStatus("offline"), NodeOffline)
	assert.Equal(t, NodeStatus("degraded"), NodeDegraded)
	assert.Equal(t, NodeStatus("maintenance"), NodeMaintenance)

	assert.Equal(t, ClusterStatus("healthy"), ClusterHealthy)
	assert.Equal(t, ClusterStatus("warning"), ClusterWarning)
	assert.Equal(t, ClusterStatus("critical"), ClusterCritical)

	assert.Equal(t, MigrationStatus("pending"), MigrationPending)
	assert.Equal(t, MigrationStatus("running"), MigrationRunning)
	assert.Equal(t, MigrationStatus("completed"), MigrationCompleted)

	assert.Equal(t, QoSCategory("cpu"), QoSCPU)
	assert.Equal(t, QoSCategory("memory"), QoSMemory)
	assert.Equal(t, QoSCategory("network"), QoSNetwork)
	assert.Equal(t, QoSCategory("storage"), QoSStorage)

	assert.Equal(t, QoSAction("throttle"), QoSActionThrottle)
	assert.Equal(t, QoSAction("reject"), QoSActionReject)
	assert.Equal(t, QoSAction("queue"), QoSActionQueue)
	assert.Equal(t, QoSAction("migrate"), QoSActionMigrate)

	assert.Equal(t, ProtectionType("failover"), ProtectionFailover)
	assert.Equal(t, ProtectionType("replication"), ProtectionReplication)
	assert.Equal(t, ProtectionType("snapshot"), ProtectionSnapshot)
	assert.Equal(t, ProtectionType("backup"), ProtectionBackup)

	assert.Equal(t, ProtectionLevel("full"), ProtectionFull)
	assert.Equal(t, ProtectionLevel("partial"), ProtectionPartial)
	assert.Equal(t, ProtectionLevel("none"), ProtectionNone)
}
