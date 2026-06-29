// Package containerfailover 容器 HA 故障转移模块
package containerfailover

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 测试辅助函数 ==========

func setupTestManager(t *testing.T) (*Manager, *EtcdSimBackend) {
	t.Helper()
	backend := NewEtcdSimBackend()
	mgr := NewManager("node-a", backend)
	return mgr, backend
}

func setupCluster(t *testing.T, mgr *Manager) {
	t.Helper()
	_, err := mgr.RegisterNode(&ClusterNode{ID: "node-b", Name: "node-b", IP: "192.168.1.20"})
	require.NoError(t, err)
	nodeA, err := mgr.GetNode("node-a")
	require.NoError(t, err)
	nodeA.IP = "192.168.1.10"
}

func createTestContainer(id, name, image, ip, node string) *Container {
	return &Container{
		ID: id, Name: name, Image: image, IP: ip, Node: node, Status: ContainerRunning,
		Ports: []PortMapping{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}},
		Volumes: []VolumeMount{{HostPath: "/data/app", ContainerPath: "/app", ReadOnly: false}},
		Labels: map[string]string{"app": "web"}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
}

// ========== 数据模型测试 ==========

func TestContainerStatus(t *testing.T) {
	assert.Equal(t, "running", string(ContainerRunning))
	assert.Equal(t, "stopped", string(ContainerStopped))
	assert.Equal(t, "failed", string(ContainerFailed))
	assert.Equal(t, "failing-over", string(ContainerFailingOver))
	assert.Equal(t, "pending", string(ContainerPending))
}

func TestFailoverMode(t *testing.T) {
	assert.Equal(t, "active-passive", string(ModeActivePassive))
	assert.Equal(t, "active-active", string(ModeActiveActive))
}

func TestDefaultFailoverPolicy(t *testing.T) {
	p := DefaultFailoverPolicy()
	assert.NotNil(t, p)
	assert.Equal(t, ModeActivePassive, p.Mode)
	assert.Equal(t, 5, p.HealthCheckInterval)
	assert.Equal(t, 3, p.FailoverDelay)
	assert.Equal(t, 3, p.MaxRetryAttempts)
	assert.True(t, p.AutoFailover)
	assert.False(t, p.SMBHA)
}

// ========== IP 管理器测试 ==========

func TestIPManager_Allocate(t *testing.T) {
	ipMgr := NewIPManager()
	alloc, err := ipMgr.Allocate("192.168.1.100", "c1", "node-a", "eth0")
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.100", alloc.IP)
	assert.Equal(t, "c1", alloc.ContainerID)
	assert.True(t, alloc.Active)
}

func TestIPManager_Allocate_DuplicateIP(t *testing.T) {
	ipMgr := NewIPManager()
	_, _ = ipMgr.Allocate("192.168.1.100", "c1", "node-a", "eth0")
	_, err := ipMgr.Allocate("192.168.1.100", "c2", "node-b", "eth0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已被容器")
}

func TestIPManager_Release(t *testing.T) {
	ipMgr := NewIPManager()
	_, _ = ipMgr.Allocate("192.168.1.100", "c1", "node-a", "eth0")
	err := ipMgr.Release("c1")
	require.NoError(t, err)
	_, err = ipMgr.Allocate("192.168.1.100", "c2", "node-b", "eth0")
	require.NoError(t, err)
}

func TestIPManager_Release_NotAllocated(t *testing.T) {
	ipMgr := NewIPManager()
	err := ipMgr.Release("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未分配 IP")
}

func TestIPManager_Migrate(t *testing.T) {
	ipMgr := NewIPManager()
	_, _ = ipMgr.Allocate("192.168.1.100", "c1", "node-a", "eth0")
	alloc, err := ipMgr.Migrate("192.168.1.100", "node-b")
	require.NoError(t, err)
	assert.Equal(t, "node-b", alloc.Node)
	assert.True(t, alloc.Active)
}

func TestIPManager_Migrate_NotFound(t *testing.T) {
	ipMgr := NewIPManager()
	_, err := ipMgr.Migrate("192.168.1.200", "node-b")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未分配")
}

func TestIPManager_GetContainerIP(t *testing.T) {
	ipMgr := NewIPManager()
	_, _ = ipMgr.Allocate("192.168.1.100", "c1", "node-a", "eth0")
	ip, err := ipMgr.GetContainerIP("c1")
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.100", ip)
}

func TestIPManager_ListAllocations(t *testing.T) {
	ipMgr := NewIPManager()
	_, _ = ipMgr.Allocate("192.168.1.100", "c1", "node-a", "eth0")
	_, _ = ipMgr.Allocate("192.168.1.101", "c2", "node-b", "eth0")
	assert.Len(t, ipMgr.ListAllocations(), 2)
}

func TestIPManager_ReleaseAll(t *testing.T) {
	ipMgr := NewIPManager()
	_, _ = ipMgr.Allocate("192.168.1.100", "c1", "node-a", "eth0")
	_, _ = ipMgr.Allocate("192.168.1.101", "c2", "node-a", "eth0")
	_, _ = ipMgr.Allocate("192.168.1.102", "c3", "node-b", "eth0")
	count := ipMgr.ReleaseAll("node-a")
	assert.Equal(t, 2, count)
}

func TestIPManager_GetAllocation_NotFound(t *testing.T) {
	ipMgr := NewIPManager()
	_, err := ipMgr.GetAllocation("192.168.1.200")
	assert.Error(t, err)
}

// ========== etcd 模拟后端测试 ==========

func TestEtcdSimBackend_SaveLoad(t *testing.T) {
	backend := NewEtcdSimBackend()
	_ = backend.Save("/test/key", []byte("value"))
	val, err := backend.Load("/test/key")
	require.NoError(t, err)
	assert.Equal(t, "value", string(val))
}

func TestEtcdSimBackend_LoadNotFound(t *testing.T) {
	backend := NewEtcdSimBackend()
	_, err := backend.Load("/nonexistent")
	assert.Error(t, err)
}

func TestEtcdSimBackend_Delete(t *testing.T) {
	backend := NewEtcdSimBackend()
	_ = backend.Save("/test/key", []byte("value"))
	_ = backend.Delete("/test/key")
	_, err := backend.Load("/test/key")
	assert.Error(t, err)
}

func TestEtcdSimBackend_List(t *testing.T) {
	backend := NewEtcdSimBackend()
	_ = backend.Save("/prefix/a", []byte("1"))
	_ = backend.Save("/prefix/b", []byte("2"))
	_ = backend.Save("/other/c", []byte("3"))
	keys, err := backend.List("/prefix/")
	require.NoError(t, err)
	assert.Len(t, keys, 2)
}

// ========== 状态同步测试 ==========

func TestStateSync_SyncContainer(t *testing.T) {
	backend := NewEtcdSimBackend()
	sync := NewStateSync(backend, "node-a")
	c := createTestContainer("c1", "web", "nginx:latest", "192.168.1.100", "node-a")
	_ = sync.SyncContainer(c)
	loaded, err := sync.LoadContainer("c1")
	require.NoError(t, err)
	assert.Equal(t, "c1", loaded.ID)
	assert.Equal(t, "nginx:latest", loaded.Image)
}

func TestStateSync_DeleteContainer(t *testing.T) {
	backend := NewEtcdSimBackend()
	sync := NewStateSync(backend, "node-a")
	c := createTestContainer("c1", "web", "nginx:latest", "", "node-a")
	_ = sync.SyncContainer(c)
	_ = sync.DeleteContainer("c1")
	_, err := sync.LoadContainer("c1")
	assert.Error(t, err)
}

func TestStateSync_ListContainers(t *testing.T) {
	backend := NewEtcdSimBackend()
	sync := NewStateSync(backend, "node-a")
	_ = sync.SyncContainer(createTestContainer("c1", "web", "nginx", "", "node-a"))
	_ = sync.SyncContainer(createTestContainer("c2", "db", "postgres", "", "node-a"))
	ids, err := sync.ListContainers()
	require.NoError(t, err)
	assert.Len(t, ids, 2)
}

func TestStateSync_SyncNode(t *testing.T) {
	backend := NewEtcdSimBackend()
	sync := NewStateSync(backend, "node-a")
	node := &ClusterNode{ID: "node-a", Name: "node-a", IP: "192.168.1.10", Status: NodeOnline, Containers: []string{"c1"}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = sync.SyncNode(node)
	loaded, err := sync.LoadNode("node-a")
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.10", loaded.IP)
	assert.Len(t, loaded.Containers, 1)
}

func TestStateSync_SyncIPAllocation(t *testing.T) {
	backend := NewEtcdSimBackend()
	sync := NewStateSync(backend, "node-a")
	alloc := &IPAllocation{IP: "192.168.1.100", ContainerID: "c1", Node: "node-a", Interface: "eth0", AllocatedAt: time.Now(), Active: true}
	_ = sync.SyncIPAllocation(alloc)
	loaded, err := sync.LoadIPAllocation("192.168.1.100")
	require.NoError(t, err)
	assert.Equal(t, "c1", loaded.ContainerID)
	assert.True(t, loaded.Active)
}

func TestStateSync_SyncFailoverEvent(t *testing.T) {
	backend := NewEtcdSimBackend()
	sync := NewStateSync(backend, "node-a")
	event := &FailoverEvent{ID: "event-1", ContainerID: "c1", ContainerName: "web", TriggeredAt: time.Now(), Trigger: TriggerManual, FromNode: "node-a", ToNode: "node-b", Reason: "测试", Success: true}
	_ = sync.SyncFailoverEvent(event)
	events, err := sync.ListFailoverEvents()
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "event-1", events[0].ID)
	assert.True(t, events[0].Success)
}

func TestStateSync_StartStop(t *testing.T) {
	backend := NewEtcdSimBackend()
	sync := NewStateSync(backend, "node-a")
	sync.SetSyncInterval(100 * time.Millisecond)
	require.NoError(t, sync.Start())
	assert.True(t, sync.IsRunning())
	time.Sleep(300 * time.Millisecond)
	require.NoError(t, sync.Stop())
	assert.False(t, sync.IsRunning())
}

func TestStateSync_Start_AlreadyRunning(t *testing.T) {
	backend := NewEtcdSimBackend()
	sync := NewStateSync(backend, "node-a")
	sync.SetSyncInterval(1 * time.Second)
	_ = sync.Start()
	defer sync.Stop()
	err := sync.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已在运行")
}

func TestStateSync_Stop_NotRunning(t *testing.T) {
	backend := NewEtcdSimBackend()
	sync := NewStateSync(backend, "node-a")
	err := sync.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未运行")
}

// ========== 管理器节点管理测试 ==========

func TestManager_RegisterNode(t *testing.T) {
	mgr, _ := setupTestManager(t)
	node, err := mgr.RegisterNode(&ClusterNode{ID: "node-b", Name: "node-b", IP: "192.168.1.20"})
	require.NoError(t, err)
	assert.Equal(t, "node-b", node.ID)
	assert.Equal(t, NodeOnline, node.Status)
}

func TestManager_RegisterNode_EmptyID(t *testing.T) {
	mgr, _ := setupTestManager(t)
	_, err := mgr.RegisterNode(&ClusterNode{Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID")
}

func TestManager_RegisterNode_EmptyName(t *testing.T) {
	mgr, _ := setupTestManager(t)
	_, err := mgr.RegisterNode(&ClusterNode{ID: "node-c"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "名称")
}

func TestManager_GetNode_NotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)
	_, err := mgr.GetNode("nonexistent")
	assert.Error(t, err)
}

func TestManager_ListNodes(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	assert.Len(t, mgr.ListNodes(), 2)
}

func TestManager_UpdateNodeStatus(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	require.NoError(t, mgr.UpdateNodeStatus("node-b", NodeOffline))
	node, _ := mgr.GetNode("node-b")
	assert.Equal(t, NodeOffline, node.Status)
}

func TestManager_UpdateNodeStatus_NotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)
	err := mgr.UpdateNodeStatus("nonexistent", NodeOffline)
	assert.Error(t, err)
}

// ========== 管理器容器管理测试 ==========

func TestManager_RegisterContainer(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	c := createTestContainer("c1", "web", "nginx:latest", "192.168.1.100", "node-a")
	container, err := mgr.RegisterContainer(c)
	require.NoError(t, err)
	assert.Equal(t, "c1", container.ID)
	assert.Equal(t, ContainerRunning, container.Status)
}

func TestManager_RegisterContainer_WithIP(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	c := createTestContainer("c1", "web", "nginx:latest", "192.168.1.100", "node-a")
	_, err := mgr.RegisterContainer(c)
	require.NoError(t, err)
	alloc, err := mgr.GetIPManager().GetAllocation("192.168.1.100")
	require.NoError(t, err)
	assert.Equal(t, "c1", alloc.ContainerID)
	assert.True(t, alloc.Active)
}

func TestManager_RegisterContainer_InvalidNode(t *testing.T) {
	mgr, _ := setupTestManager(t)
	c := createTestContainer("c1", "web", "nginx:latest", "", "nonexistent")
	_, err := mgr.RegisterContainer(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "节点")
}

func TestManager_RegisterContainer_EmptyID(t *testing.T) {
	mgr, _ := setupTestManager(t)
	c := createTestContainer("", "web", "nginx:latest", "", "")
	_, err := mgr.RegisterContainer(c)
	assert.Error(t, err)
}

func TestManager_GetContainer_NotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)
	_, err := mgr.GetContainer("nonexistent")
	assert.Error(t, err)
}

func TestManager_ListContainers(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_, _ = mgr.RegisterContainer(createTestContainer("c1", "web", "nginx", "", "node-a"))
	_, _ = mgr.RegisterContainer(createTestContainer("c2", "db", "postgres", "", "node-a"))
	assert.Len(t, mgr.ListContainers(), 2)
}

func TestManager_RemoveContainer(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	c := createTestContainer("c1", "web", "nginx:latest", "192.168.1.100", "node-a")
	_, _ = mgr.RegisterContainer(c)
	require.NoError(t, mgr.RemoveContainer("c1"))
	_, err := mgr.GetContainer("c1")
	assert.Error(t, err)
	_, err = mgr.GetIPManager().GetContainerIP("c1")
	assert.Error(t, err)
}

func TestManager_RemoveContainer_NotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)
	err := mgr.RemoveContainer("nonexistent")
	assert.Error(t, err)
}

// ========== 策略管理测试 ==========

func TestManager_GetPolicy(t *testing.T) {
	mgr, _ := setupTestManager(t)
	policy := mgr.GetPolicy()
	assert.NotNil(t, policy)
	assert.Equal(t, ModeActivePassive, policy.Mode)
}

func TestManager_SetPolicy(t *testing.T) {
	mgr, _ := setupTestManager(t)
	newPolicy := &FailoverPolicy{Mode: ModeActiveActive, HealthCheckInterval: 10, FailoverDelay: 5, MaxRetryAttempts: 5, AutoFailover: false, SMBHA: true}
	require.NoError(t, mgr.SetPolicy(newPolicy))
	policy := mgr.GetPolicy()
	assert.Equal(t, ModeActiveActive, policy.Mode)
	assert.Equal(t, 10, policy.HealthCheckInterval)
	assert.False(t, policy.AutoFailover)
	assert.True(t, policy.SMBHA)
}

func TestManager_SetPolicy_InvalidInterval(t *testing.T) {
	mgr, _ := setupTestManager(t)
	err := mgr.SetPolicy(&FailoverPolicy{Mode: ModeActivePassive, HealthCheckInterval: 0})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "间隔")
}

func TestManager_SetPolicy_NegativeDelay(t *testing.T) {
	mgr, _ := setupTestManager(t)
	err := mgr.SetPolicy(&FailoverPolicy{Mode: ModeActivePassive, HealthCheckInterval: 5, FailoverDelay: -1})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "延迟")
}

// ========== 健康检查测试 ==========

func TestManager_StartStopHealthCheck(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_ = mgr.SetPolicy(&FailoverPolicy{Mode: ModeActivePassive, HealthCheckInterval: 1, FailoverDelay: 0, MaxRetryAttempts: 3, AutoFailover: false})
	require.NoError(t, mgr.StartHealthCheck())
	time.Sleep(2 * time.Second)
	require.NoError(t, mgr.StopHealthCheck())
}

func TestManager_StartHealthCheck_AlreadyRunning(t *testing.T) {
	mgr, _ := setupTestManager(t)
	_ = mgr.SetPolicy(&FailoverPolicy{Mode: ModeActivePassive, HealthCheckInterval: 1, FailoverDelay: 0, MaxRetryAttempts: 3, AutoFailover: false})
	_ = mgr.StartHealthCheck()
	defer mgr.StopHealthCheck()
	err := mgr.StartHealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已在运行")
}

func TestManager_StopHealthCheck_NotRunning(t *testing.T) {
	mgr, _ := setupTestManager(t)
	err := mgr.StopHealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未运行")
}

func TestManager_CheckContainerHealth_OnlineNode(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	healthy := mgr.checkContainerHealth(&Container{ID: "c1", Status: ContainerRunning, Node: "node-a"})
	assert.True(t, healthy)
}

func TestManager_CheckContainerHealth_OfflineNode(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_ = mgr.UpdateNodeStatus("node-a", NodeOffline)
	healthy := mgr.checkContainerHealth(&Container{ID: "c1", Status: ContainerRunning, Node: "node-a"})
	assert.False(t, healthy)
}

func TestManager_CheckContainerHealth_StoppedContainer(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	healthy := mgr.checkContainerHealth(&Container{ID: "c1", Status: ContainerStopped, Node: "node-a"})
	assert.False(t, healthy)
}

// ========== 故障转移测试 ==========

func TestManager_ManualFailover(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_ = mgr.SetPolicy(&FailoverPolicy{Mode: ModeActivePassive, HealthCheckInterval: 5, FailoverDelay: 0, MaxRetryAttempts: 3, AutoFailover: true})
	c := createTestContainer("c1", "web", "nginx:latest", "192.168.1.100", "node-a")
	_, err := mgr.RegisterContainer(c)
	require.NoError(t, err)
	event, err := mgr.ManualFailover("c1", "node-b", "手动测试故障转移")
	require.NoError(t, err)
	assert.Equal(t, "c1", event.ContainerID)
	assert.Equal(t, "node-a", event.FromNode)
	assert.Equal(t, "node-b", event.ToNode)
	assert.Equal(t, TriggerManual, event.Trigger)
	assert.True(t, event.Success)
	assert.True(t, event.IPMigrated)
	assert.NotNil(t, event.CompletedAt)
	container, err := mgr.GetContainer("c1")
	require.NoError(t, err)
	assert.Equal(t, "node-b", container.Node)
	assert.Equal(t, ContainerRunning, container.Status)
	alloc, err := mgr.GetIPManager().GetAllocation("192.168.1.100")
	require.NoError(t, err)
	assert.Equal(t, "node-b", alloc.Node)
}

func TestManager_ManualFailover_ContainerNotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_, err := mgr.ManualFailover("nonexistent", "node-b", "测试")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestManager_ManualFailover_TargetNodeNotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_, _ = mgr.RegisterContainer(createTestContainer("c1", "web", "nginx:latest", "", "node-a"))
	_, err := mgr.ManualFailover("c1", "nonexistent", "测试")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestManager_ManualFailover_TargetNodeOffline(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_ = mgr.UpdateNodeStatus("node-b", NodeOffline)
	_, _ = mgr.RegisterContainer(createTestContainer("c1", "web", "nginx:latest", "", "node-a"))
	_, err := mgr.ManualFailover("c1", "node-b", "测试")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不在线")
}

func TestManager_ManualFailover_WithSMBHA(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_ = mgr.SetPolicy(&FailoverPolicy{Mode: ModeActivePassive, HealthCheckInterval: 5, FailoverDelay: 0, MaxRetryAttempts: 3, AutoFailover: true, SMBHA: true})
	_, err := mgr.RegisterContainer(createTestContainer("c1", "web", "nginx:latest", "192.168.1.100", "node-a"))
	require.NoError(t, err)
	event, err := mgr.ManualFailover("c1", "node-b", "SMB HA 测试")
	require.NoError(t, err)
	assert.True(t, event.SMBFailover)
	assert.True(t, event.Success)
}

func TestManager_ManualFailover_NoIP(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_ = mgr.SetPolicy(&FailoverPolicy{Mode: ModeActivePassive, HealthCheckInterval: 5, FailoverDelay: 0, MaxRetryAttempts: 3, AutoFailover: true})
	_, err := mgr.RegisterContainer(createTestContainer("c1", "web", "nginx:latest", "", "node-a"))
	require.NoError(t, err)
	event, err := mgr.ManualFailover("c1", "node-b", "无 IP 容器迁移")
	require.NoError(t, err)
	assert.True(t, event.Success)
	assert.False(t, event.IPMigrated)
}

func TestManager_FailoverHistory(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_ = mgr.SetPolicy(&FailoverPolicy{Mode: ModeActivePassive, HealthCheckInterval: 5, FailoverDelay: 0, MaxRetryAttempts: 3, AutoFailover: true})
	_, _ = mgr.RegisterContainer(createTestContainer("c1", "web", "nginx:latest", "", "node-a"))
	_, err := mgr.ManualFailover("c1", "node-b", "第一次迁移")
	require.NoError(t, err)
	_, err = mgr.ManualFailover("c1", "node-a", "第二次迁移")
	require.NoError(t, err)
	assert.Len(t, mgr.GetFailoverHistory(0), 2)
	assert.Len(t, mgr.GetFailoverHistory(1), 1)
}

func TestManager_SelectFailoverTarget(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	c := createTestContainer("c1", "web", "nginx:latest", "", "node-a")
	c.PreferredNode = "node-b"
	_, _ = mgr.RegisterContainer(c)
	target := mgr.selectFailoverTarget("c1")
	assert.Equal(t, "node-b", target)
}

func TestManager_SelectFailoverTarget_NoPreferred(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_, _ = mgr.RegisterContainer(createTestContainer("c1", "web", "nginx:latest", "", "node-a"))
	target := mgr.selectFailoverTarget("c1")
	assert.Equal(t, "node-b", target)
}

func TestManager_SelectFailoverTarget_AllOffline(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_ = mgr.UpdateNodeStatus("node-b", NodeOffline)
	_, _ = mgr.RegisterContainer(createTestContainer("c1", "web", "nginx:latest", "", "node-a"))
	target := mgr.selectFailoverTarget("c1")
	assert.Equal(t, "", target)
}

// ========== 集群状态测试 ==========

func TestManager_GetClusterStatus(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_, _ = mgr.RegisterContainer(createTestContainer("c1", "web", "nginx:latest", "", "node-a"))
	_, _ = mgr.RegisterContainer(createTestContainer("c2", "db", "postgres:latest", "", "node-b"))
	status := mgr.GetClusterStatus()
	assert.Equal(t, 2, status.TotalNodes)
	assert.Equal(t, 2, status.OnlineNodes)
	assert.Equal(t, 2, status.TotalContainers)
	assert.Equal(t, 2, status.RunningContainers)
	assert.Equal(t, 0, status.FailoverCount)
}

func TestManager_GetClusterStatus_WithFailures(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_ = mgr.SetPolicy(&FailoverPolicy{Mode: ModeActivePassive, HealthCheckInterval: 5, FailoverDelay: 0, MaxRetryAttempts: 3, AutoFailover: true})
	_, _ = mgr.RegisterContainer(createTestContainer("c1", "web", "nginx:latest", "", "node-a"))
	_, _ = mgr.ManualFailover("c1", "node-b", "测试")
	status := mgr.GetClusterStatus()
	assert.Equal(t, 1, status.FailoverCount)
	assert.NotNil(t, status.LastFailover)
}

// ========== 同步测试 ==========

func TestManager_SyncAll(t *testing.T) {
	mgr, _ := setupTestManager(t)
	setupCluster(t, mgr)
	_, _ = mgr.RegisterContainer(createTestContainer("c1", "web", "nginx:latest", "192.168.1.100", "node-a"))
	require.NoError(t, mgr.SyncAll())
	sync := mgr.GetStateSync()
	require.NotNil(t, sync)
	loaded, err := sync.LoadContainer("c1")
	require.NoError(t, err)
	assert.Equal(t, "c1", loaded.ID)
	ids, _ := sync.ListContainers()
	assert.Contains(t, ids, "c1")
}

func TestManager_SyncAll_NoBackend(t *testing.T) {
	mgr := NewManager("node-a", nil)
	err := mgr.SyncAll()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未配置")
}

// ========== 完整故障转移流程测试 ==========

func TestManager_FullFailoverFlow(t *testing.T) {
	mgr, backend := setupTestManager(t)
	setupCluster(t, mgr)
	_ = mgr.SetPolicy(&FailoverPolicy{Mode: ModeActivePassive, HealthCheckInterval: 5, FailoverDelay: 0, MaxRetryAttempts: 3, AutoFailover: true, SMBHA: true})

	// 1. 注册容器到 node-a，分配静态 IP
	c := createTestContainer("c1", "web", "nginx:latest", "192.168.1.100", "node-a")
	c.PreferredNode = "node-b"
	_, err := mgr.RegisterContainer(c)
	require.NoError(t, err)

	// 2. 验证初始状态
	container, err := mgr.GetContainer("c1")
	require.NoError(t, err)
	assert.Equal(t, "node-a", container.Node)
	assert.Equal(t, ContainerRunning, container.Status)

	// 3. 同步到后端
	require.NoError(t, mgr.SyncAll())
	sync := mgr.GetStateSync()
	loaded, err := sync.LoadContainer("c1")
	require.NoError(t, err)
	assert.Equal(t, "node-a", loaded.Node)

	// 4. 触发故障转移
	event, err := mgr.ManualFailover("c1", "node-b", "完整流程测试")
	require.NoError(t, err)
	assert.True(t, event.Success)
	assert.True(t, event.IPMigrated)
	assert.True(t, event.SMBFailover)

	// 5. 验证容器已迁移
	container, err = mgr.GetContainer("c1")
	require.NoError(t, err)
	assert.Equal(t, "node-b", container.Node)
	assert.Equal(t, ContainerRunning, container.Status)

	// 6. 验证 IP 已迁移
	alloc, err := mgr.GetIPManager().GetAllocation("192.168.1.100")
	require.NoError(t, err)
	assert.Equal(t, "node-b", alloc.Node)

	// 7. 验证后端已同步
	loaded, err = sync.LoadContainer("c1")
	require.NoError(t, err)
	assert.Equal(t, "node-b", loaded.Node)

	// 8. 验证故障转移事件已记录
	history := mgr.GetFailoverHistory(0)
	require.Len(t, history, 1)
	assert.True(t, history[0].Success)

	// 9. 验证节点容器列表已更新
	nodeA, _ := mgr.GetNode("node-a")
	assert.NotContains(t, nodeA.Containers, "c1")
	nodeB, _ := mgr.GetNode("node-b")
	assert.Contains(t, nodeB.Containers, "c1")

	// 10. 验证后端有故障转移事件
	events, err := sync.ListFailoverEvents()
	require.NoError(t, err)
	assert.Len(t, events, 1)

	// 避免未使用变量警告
	assert.NotNil(t, backend)
}
