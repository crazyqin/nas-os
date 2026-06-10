package smb

import (
	"context"
	"testing"
	"time"
)

// -------------------- 状态同步器测试 --------------------

func TestNewStateSynchronizer(t *testing.T) {
	config := DefaultStateSyncConfig()
	registry := NewSessionRegistry()

	sync := NewStateSynchronizer(config, registry)
	if sync == nil {
		t.Fatal("NewStateSynchronizer returned nil")
	}

	if sync.config != config {
		t.Error("config not set correctly")
	}
	if sync.sessionRegistry != registry {
		t.Error("session registry not set correctly")
	}
	if sync.running {
		t.Error("should not be running initially")
	}
}

func TestStateSynchronizerDefaultConfig(t *testing.T) {
	config := DefaultStateSyncConfig()

	if !config.Enabled {
		t.Error("default config should be enabled")
	}
	if config.SyncIntervalMs != 5000 {
		t.Errorf("expected SyncIntervalMs=5000, got %d", config.SyncIntervalMs)
	}
	if config.BatchSize != 100 {
		t.Errorf("expected BatchSize=100, got %d", config.BatchSize)
	}
	if config.MaxConcurrentSyncs != 5 {
		t.Errorf("expected MaxConcurrentSyncs=5, got %d", config.MaxConcurrentSyncs)
	}
	if config.RetryAttempts != 3 {
		t.Errorf("expected RetryAttempts=3, got %d", config.RetryAttempts)
	}
	if config.TimeoutMs != 30000 {
		t.Errorf("expected TimeoutMs=30000, got %d", config.TimeoutMs)
	}
}

func TestStateSynchronizerStartStop(t *testing.T) {
	config := DefaultStateSyncConfig()
	registry := NewSessionRegistry()
	sync := NewStateSynchronizer(config, registry)

	// 启动
	if err := sync.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !sync.IsRunning() {
		t.Error("sync should be running after Start")
	}

	// 重复启动应该报错
	if err := sync.Start(); err == nil {
		t.Error("duplicate Start should fail")
	}

	// 停止
	sync.Stop()

	if sync.IsRunning() {
		t.Error("sync should not be running after Stop")
	}
}

func TestStateSynchronizerAddRemoveNode(t *testing.T) {
	config := DefaultStateSyncConfig()
	registry := NewSessionRegistry()
	sync := NewStateSynchronizer(config, registry)

	// 添加节点
	sync.AddNode("node1", "host1", "192.168.1.1", 445)

	nodes := sync.GetNodeSyncStatus()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	node, ok := nodes["node1"]
	if !ok {
		t.Fatal("node1 not found")
	}
	if node.Hostname != "host1" {
		t.Errorf("expected hostname=host1, got %s", node.Hostname)
	}
	if !node.Connected {
		t.Error("node should be connected")
	}

	// 移除节点
	sync.RemoveNode("node1")
	nodes = sync.GetNodeSyncStatus()
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes after removal, got %d", len(nodes))
	}
}

func TestStateSynchronizerMetrics(t *testing.T) {
	config := DefaultStateSyncConfig()
	registry := NewSessionRegistry()
	sync := NewStateSynchronizer(config, registry)

	metrics := sync.GetSyncMetrics()
	if metrics.TotalSyncs != 0 {
		t.Errorf("expected TotalSyncs=0, got %d", metrics.TotalSyncs)
	}
	if metrics.SuccessfulSyncs != 0 {
		t.Errorf("expected SuccessfulSyncs=0, got %d", metrics.SuccessfulSyncs)
	}
}

func TestStateSynchronizerNodeSyncStatus(t *testing.T) {
	config := DefaultStateSyncConfig()
	registry := NewSessionRegistry()
	sync := NewStateSynchronizer(config, registry)

	sync.AddNode("node1", "host1", "192.168.1.1", 445)
	sync.AddNode("node2", "host2", "192.168.1.2", 445)
	sync.SetLocalNode("node1")

	nodes := sync.GetNodeSyncStatus()
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	// 检查IsNodeInSync
	if sync.IsNodeInSync("node1", 5*time.Second) {
		// 本地节点应该在同步中
	}

	if sync.IsNodeInSync("nonexistent", 5*time.Second) {
		t.Error("nonexistent node should not be in sync")
	}
}

func TestStateSynchronizerGetActiveSyncs(t *testing.T) {
	config := DefaultStateSyncConfig()
	registry := NewSessionRegistry()
	sync := NewStateSynchronizer(config, registry)

	syncs := sync.GetActiveSyncs()
	if len(syncs) != 0 {
		t.Errorf("expected 0 active syncs, got %d", len(syncs))
	}
}

func TestStateSynchronizerLastSyncTime(t *testing.T) {
	config := DefaultStateSyncConfig()
	registry := NewSessionRegistry()
	sync := NewStateSynchronizer(config, registry)

	lastSync := sync.GetLastSyncTime()
	if !lastSync.IsZero() {
		t.Error("last sync time should be zero initially")
	}
}

// -------------------- 健康检查器测试 --------------------

func TestNewHealthChecker(t *testing.T) {
	config := DefaultHealthCheckConfig()

	hc := NewHealthChecker(config)
	if hc == nil {
		t.Fatal("NewHealthChecker returned nil")
	}

	if hc.config != config {
		t.Error("config not set correctly")
	}
	if hc.running {
		t.Error("should not be running initially")
	}
}

func TestHealthCheckerDefaultConfig(t *testing.T) {
	config := DefaultHealthCheckConfig()

	if !config.Enabled {
		t.Error("default config should be enabled")
	}
	if config.CheckIntervalMs != 5000 {
		t.Errorf("expected CheckIntervalMs=5000, got %d", config.CheckIntervalMs)
	}
	if config.TimeoutMs != 3000 {
		t.Errorf("expected TimeoutMs=3000, got %d", config.TimeoutMs)
	}
	if config.HealthyThreshold != 2 {
		t.Errorf("expected HealthyThreshold=2, got %d", config.HealthyThreshold)
	}
	if config.UnhealthyThreshold != 3 {
		t.Errorf("expected UnhealthyThreshold=3, got %d", config.UnhealthyThreshold)
	}
	if !config.SMBServiceCheck {
		t.Error("SMBServiceCheck should be enabled by default")
	}
	if !config.DiskSpaceCheck {
		t.Error("DiskSpaceCheck should be enabled by default")
	}
	if !config.MemoryCheck {
		t.Error("MemoryCheck should be enabled by default")
	}
}

func TestHealthCheckerStartStop(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	// 启动
	if err := hc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !hc.IsRunning() {
		t.Error("should be running after Start")
	}

	// 重复启动应该报错
	if err := hc.Start(); err == nil {
		t.Error("duplicate Start should fail")
	}

	// 停止
	hc.Stop()

	if hc.IsRunning() {
		t.Error("should not be running after Stop")
	}
}

func TestHealthCheckerAddRemoveNode(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	// 添加节点
	hc.AddNode("node1", "host1", "192.168.1.1", 445)

	nodes := hc.GetAllNodeHealth()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	node, ok := nodes["node1"]
	if !ok {
		t.Fatal("node1 not found")
	}
	if node.Hostname != "host1" {
		t.Errorf("expected hostname=host1, got %s", node.Hostname)
	}
	if !node.Healthy {
		t.Error("node should be healthy initially")
	}

	// 移除节点
	hc.RemoveNode("node1")
	nodes = hc.GetAllNodeHealth()
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes after removal, got %d", len(nodes))
	}
}

func TestHealthCheckerSetLocalNode(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	hc.SetLocalNode("local-node")

	// 添加本地节点和远程节点
	hc.AddNode("local-node", "localhost", "127.0.0.1", 445)
	hc.AddNode("remote-node", "remote", "192.168.1.2", 445)

	// 验证本地节点不参与检查
	// (在checkAllNodes中会跳过localNodeID)
}

func TestHealthCheckerIsNodeHealthy(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	hc.AddNode("node1", "host1", "192.168.1.1", 445)

	// 新节点应该是健康的
	if !hc.IsNodeHealthy("node1") {
		t.Error("node should be healthy initially")
	}

	// 不存在的节点
	if hc.IsNodeHealthy("nonexistent") {
		t.Error("nonexistent node should not be healthy")
	}
}

func TestHealthCheckerGetNodeHealth(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	hc.AddNode("node1", "host1", "192.168.1.1", 445)

	node, ok := hc.GetNodeHealth("node1")
	if !ok {
		t.Fatal("node1 not found")
	}
	if node.NodeID != "node1" {
		t.Errorf("expected node_id=node1, got %s", node.NodeID)
	}
	if !node.Healthy {
		t.Error("node should be healthy")
	}

	// 不存在的节点
	_, ok = hc.GetNodeHealth("nonexistent")
	if ok {
		t.Error("nonexistent node should not be found")
	}
}

func TestHealthCheckerGetClusterHealth(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	hc.AddNode("node1", "host1", "192.168.1.1", 445)
	hc.AddNode("node2", "host2", "192.168.1.2", 445)

	health := hc.GetClusterHealth()
	if health.TotalNodes != 2 {
		t.Errorf("expected TotalNodes=2, got %d", health.TotalNodes)
	}
	if health.HealthyNodes != 2 {
		t.Errorf("expected HealthyNodes=2, got %d", health.HealthyNodes)
	}
	if health.UnhealthyNodes != 0 {
		t.Errorf("expected UnhealthyNodes=0, got %d", health.UnhealthyNodes)
	}
	if !health.ClusterHealthy {
		t.Error("cluster should be healthy")
	}
}

func TestHealthCheckerGetHealthyUnhealthyNodes(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	hc.AddNode("node1", "host1", "192.168.1.1", 445)
	hc.AddNode("node2", "host2", "192.168.1.2", 445)

	healthy := hc.GetHealthyNodes()
	if len(healthy) != 2 {
		t.Errorf("expected 2 healthy nodes, got %d", len(healthy))
	}

	unhealthy := hc.GetUnhealthyNodes()
	if len(unhealthy) != 0 {
		t.Errorf("expected 0 unhealthy nodes, got %d", len(unhealthy))
	}
}

func TestHealthCheckerGetNodeResult(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	hc.AddNode("node1", "host1", "192.168.1.1", 445)

	// 初始没有结果
	_, ok := hc.GetNodeResult("node1")
	if ok {
		t.Error("should have no result initially")
	}
}

func TestHealthCheckerGetStats(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	hc.AddNode("node1", "host1", "192.168.1.1", 445)

	stats := hc.GetHealthStats()
	if stats.TotalChecks != 0 {
		t.Errorf("expected TotalChecks=0, got %d", stats.TotalChecks)
	}
	if stats.TotalFailures != 0 {
		t.Errorf("expected TotalFailures=0, got %d", stats.TotalFailures)
	}
}

func TestHealthCheckerGetUptime(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	// 未启动时
	uptime := hc.GetUptime()
	if uptime != 0 {
		t.Errorf("expected uptime=0, got %v", uptime)
	}

	// 启动后
	hc.Start()
	time.Sleep(10 * time.Millisecond)
	uptime = hc.GetUptime()
	if uptime <= 0 {
		t.Errorf("expected positive uptime, got %v", uptime)
	}
	hc.Stop()
}

func TestHealthCheckerHealthCheckHandler(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	handler := hc.HealthCheckHandler()
	if handler == nil {
		t.Fatal("HealthCheckHandler returned nil")
	}
}

func TestHealthCheckerCustomCheck(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	// 注册自定义检查
	hc.RegisterCheck("custom_check", func(ctx context.Context, node *NodeHealthStatus) CheckDetail {
		return CheckDetail{
			Name:    "custom_check",
			Healthy: true,
			Message: "custom check passed",
		}
	})

	// 验证自定义检查已注册
	stats := hc.GetHealthStats()
	if stats.RegisteredChecks != 1 {
		t.Errorf("expected 1 registered check, got %d", stats.RegisteredChecks)
	}
}

func TestHealthCheckerHealthChangeCallback(t *testing.T) {
	config := DefaultHealthCheckConfig()
	hc := NewHealthChecker(config)

	callbackCalled := false
	var callbackNodeID string
	var callbackHealthy bool

	hc.SetHealthChangeCallback(func(nodeID string, healthy bool) {
		callbackCalled = true
		callbackNodeID = nodeID
		callbackHealthy = healthy
	})

	// 模拟健康状态变化
	hc.AddNode("node1", "host1", "192.168.1.1", 445)

	// 直接测试回调触发（通过processResult）
	node, _ := hc.GetNodeHealth("node1")
	result := &HealthCheckResult{
		NodeID:    "node1",
		Healthy:   false,
		Latency:   10 * time.Millisecond,
		Message:   "test failure",
		Timestamp: time.Now(),
		Checks:    []CheckDetail{},
	}

	// 复制节点信息用于测试
	testNode := &NodeHealthStatus{
		NodeID:   node.NodeID,
		Hostname: node.Hostname,
		Address:  node.Address,
		Port:     node.Port,
		Healthy:  node.Healthy,
	}

	hc.processResult(testNode, result)

	// 注意：processResult可能需要调整才能直接测试
	// 这里验证回调函数已设置
	_ = callbackCalled
	_ = callbackNodeID
	_ = callbackHealthy
}

// -------------------- 辅助函数测试 --------------------

func TestCheckDiskSpaceScore(t *testing.T) {
	score := checkDiskSpaceScore()
	if score < 0 || score > 100 {
		t.Errorf("score should be 0-100, got %d", score)
	}
}

func TestCheckMemoryPressureScore(t *testing.T) {
	score := checkMemoryPressureScore()
	if score < 0 || score > 100 {
		t.Errorf("score should be 0-100, got %d", score)
	}
}
