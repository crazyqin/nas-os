// Package containerha 提供容器高可用故障转移功能
package containerha

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// 测试配置
func createTestConfig() *ContainerHAConfig {
	return &ContainerHAConfig{
		ClusterName:         "test-cluster",
		HealthCheckInterval: 10,
		FailureThreshold:    3,
		AutoFailback:        true,
		FailbackDelay:       60,
		HeartbeatTimeout:    30,
		SyncMode:            "checkpoint",
		SyncInterval:        60,
		EnableStaticIP:      true,
		EnableResourceCheck: true,
		PrimaryNode: NodeConfig{
			ID:      "node-1",
			Address: "192.168.1.100",
			Port:    8080,
			Role:    "master",
			Weight:  100,
		},
		SecondaryNodes: []NodeConfig{
			{
				ID:      "node-2",
				Address: "192.168.1.101",
				Port:    8080,
				Role:    "slave",
				Weight:  90,
			},
			{
				ID:      "node-3",
				Address: "192.168.1.102",
				Port:    8080,
				Role:    "slave",
				Weight:  80,
			},
		},
		ProtectedContainers: []ContainerConfig{
			{
				ContainerID:     "web-app-1",
				Type:            "docker",
				EnableFailover:  true,
				Priority:        1,
				StaticIP:        "192.168.1.200",
				HealthCheckPort: 8080,
				HealthCheckPath: "/health",
			},
			{
				ContainerID:     "db-app-1",
				Type:            "lxc",
				EnableFailover:  true,
				Priority:        2,
				StaticIP:        "192.168.1.201",
			},
		},
		VirtualIPs: []VirtualIPConfig{
			{
				IP:        "192.168.1.200",
				Interface: "eth0",
				SubnetMask: "255.255.255.0",
				Gateway:    "192.168.1.1",
			},
		},
		ResourceThresholds: ResourceThresholds{
			CPUThreshold:    90.0,
			MemoryThreshold: 85.0,
			DiskThreshold:   95.0,
		},
	}
}

// TestNewFailoverManager 测试创建故障转移管理器
func TestNewFailoverManager(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	if manager == nil {
		t.Fatal("管理器创建失败")
	}

	if manager.GetLocalNodeID() != "node-1" {
		t.Errorf("本地节点ID不匹配，期望: node-1, 实际: %s", manager.GetLocalNodeID())
	}

	if !manager.IsMaster() {
		t.Error("节点应该是主节点")
	}
}

// TestFailoverManager_Start 测试启动管理器
func TestFailoverManager_Start(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("启动管理器失败: %v", err)
	}

	// 等待一下让管理器完全启动
	time.Sleep(100 * time.Millisecond)

	// 检查状态
	status := manager.GetStatus()
	if status == nil {
		t.Fatal("状态为空")
	}

	if status.ClusterName != "test-cluster" {
		t.Errorf("集群名称不匹配，期望: test-cluster, 实际: %s", status.ClusterName)
	}

	// 停止管理器
	manager.Stop()
}

// TestFailoverManager_GetNodes 测试获取节点
func TestFailoverManager_GetNodes(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	nodes := manager.GetNodes()
	if len(nodes) != 3 {
		t.Fatalf("节点数量不匹配，期望: 3, 实际: %d", len(nodes))
	}

	// 查找主节点
	masterFound := false
	for _, node := range nodes {
		if node.Role == "master" {
			masterFound = true
			if node.ID != "node-1" {
				t.Errorf("主节点ID不匹配，期望: node-1, 实际: %s", node.ID)
			}
		}
	}

	if !masterFound {
		t.Error("未找到主节点")
	}
}

// TestFailoverManager_GetNode 测试获取单个节点
func TestFailoverManager_GetNode(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	// 获取存在的节点
	node, err := manager.GetNode("node-1")
	if err != nil {
		t.Fatalf("获取节点失败: %v", err)
	}

	if node.ID != "node-1" {
		t.Errorf("节点ID不匹配，期望: node-1, 实际: %s", node.ID)
	}

	// 获取不存在的节点
	_, err = manager.GetNode("node-999")
	if err == nil {
		t.Error("应该返回错误")
	}
}

// TestFailoverManager_GetConfig 测试获取配置
func TestFailoverManager_GetConfig(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	retrievedConfig := manager.GetConfig()
	if retrievedConfig == nil {
		t.Fatal("配置为空")
	}

	if retrievedConfig.ClusterName != "test-cluster" {
		t.Errorf("集群名称不匹配，期望: test-cluster, 实际: %s", retrievedConfig.ClusterName)
	}
}

// TestFailoverManager_UpdateConfig 测试更新配置
func TestFailoverManager_UpdateConfig(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	// 创建新配置
	newConfig := createTestConfig()
	newConfig.ClusterName = "updated-cluster"
	newConfig.HealthCheckInterval = 20

	// 更新配置
	err := manager.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("更新配置失败: %v", err)
	}

	// 验证更新
	retrievedConfig := manager.GetConfig()
	if retrievedConfig.ClusterName != "updated-cluster" {
		t.Errorf("集群名称未更新，期望: updated-cluster, 实际: %s", retrievedConfig.ClusterName)
	}
}

// TestFailoverManager_GetProtectedContainers 测试获取受保护容器
func TestFailoverManager_GetProtectedContainers(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	containers := manager.GetAllProtectedContainers()
	if len(containers) != 2 {
		t.Fatalf("容器数量不匹配，期望: 2, 实际: %d", len(containers))
	}

	// 验证容器信息
	for _, container := range containers {
		if container.ContainerID == "" {
			t.Error("容器ID为空")
		}
	}
}

// TestFailoverManager_GetProtectedContainer 测试获取单个受保护容器
func TestFailoverManager_GetProtectedContainer(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	// 获取存在的容器
	container, err := manager.GetProtectedContainer("web-app-1")
	if err != nil {
		t.Fatalf("获取容器失败: %v", err)
	}

	if container.ContainerID != "web-app-1" {
		t.Errorf("容器ID不匹配，期望: web-app-1, 实际: %s", container.ContainerID)
	}

	// 获取不存在的容器
	_, err = manager.GetProtectedContainer("non-existent")
	if err == nil {
		t.Error("应该返回错误")
	}
}

// TestFailoverManager_ProcessHeartbeat 测试处理心跳
func TestFailoverManager_ProcessHeartbeat(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	heartbeat := &HeartbeatMessage{
		NodeID:    "node-2",
		Timestamp: time.Now(),
		Status:    "online",
		ResourceUsage: ResourceUsage{
			CPUUsage:    50.0,
			MemoryUsage: 60.0,
			DiskUsage:   70.0,
		},
		SequenceNumber: 1,
	}

	err := manager.ProcessHeartbeat(heartbeat)
	if err != nil {
		t.Fatalf("处理心跳失败: %v", err)
	}

	// 验证节点状态更新
	node, err := manager.GetNode("node-2")
	if err != nil {
		t.Fatalf("获取节点失败: %v", err)
	}

	if node.Status != "online" {
		t.Errorf("节点状态不匹配，期望: online, 实际: %s", node.Status)
	}
}

// TestFailoverManager_ExecuteFailover 测试执行故障转移
func TestFailoverManager_ExecuteFailover(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	// 模拟节点在线
	node2, _ := manager.GetNode("node-2")
	node2.Status = "online"
	node2.HealthScore = 80

	// 设置容器在node-1上运行
	manager.containerMu.Lock()
	for _, container := range manager.containers {
		container.CurrentNode = "node-1"
		container.Status = "running"
	}
	manager.containerMu.Unlock()

	// 执行故障转移
	request := &FailoverRequest{
		TargetNode: "node-2",
		Containers: []string{"web-app-1"},
		Reason:     "测试故障转移",
	}

	response, err := manager.ExecuteFailover(request)
	if err != nil {
		t.Fatalf("执行故障转移失败: %v", err)
	}

	if !response.Success {
		t.Error("故障转移应该成功")
	}

	if len(response.AffectedContainers) != 1 {
		t.Errorf("受影响容器数量不匹配，期望: 1, 实际: %d", len(response.AffectedContainers))
	}
}

// TestFailoverManager_GetFailoverHistory 测试获取故障转移历史
func TestFailoverManager_GetFailoverHistory(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	// 添加一些历史事件
	manager.historyMu.Lock()
	manager.failoverHistory = append(manager.failoverHistory, FailoverEvent{
		EventID:   "event-1",
		Timestamp: time.Now(),
		Type:      "failover",
		Status:    "success",
	})
	manager.historyMu.Unlock()

	history := manager.GetFailoverHistory()
	if len(history) != 1 {
		t.Fatalf("历史数量不匹配，期望: 1, 实际: %d", len(history))
	}

	if history[0].EventID != "event-1" {
		t.Errorf("事件ID不匹配，期望: event-1, 实际: %s", history[0].EventID)
	}
}

// TestFailoverManager_SyncNow 测试立即同步
func TestFailoverManager_SyncNow(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	// 设置容器状态
	manager.containerMu.Lock()
	for _, container := range manager.containers {
		container.Status = "running"
		container.CurrentNode = "node-1"
	}
	manager.containerMu.Unlock()

	// 触发同步
	err := manager.SyncNow()
	if err != nil {
		t.Fatalf("触发同步失败: %v", err)
	}

	// 等待同步完成
	time.Sleep(200 * time.Millisecond)

	// 验证同步状态
	status := manager.GetSyncStatus()
	if status.State != "idle" && status.State != "syncing" {
		t.Errorf("同步状态异常: %s", status.State)
	}
}

// TestHealthChecker 测试健康检查器
func TestHealthChecker(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动健康检查器
	go manager.healthChecker.Start(ctx)

	// 等待检查完成
	time.Sleep(500 * time.Millisecond)

	// 获取检查结果
	_ = manager.healthChecker.GetAllCheckResults()
	// 注意：在测试环境中，节点可能无法连接，所以结果可能为空或失败

	// 停止健康检查器
	manager.healthChecker.Stop()
}

// TestContainerHAHandler 测试HTTP处理器
func TestContainerHAHandler(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")
	handler := NewContainerHAHandler(manager)

	// 测试状态端点
	t.Run("Status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/containerha/status", nil)
		w := httptest.NewRecorder()

		handler.handleStatus(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不匹配，期望: 200, 实际: %d", w.Code)
		}

		var response APIResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if !response.Success {
			t.Error("响应应该成功")
		}
	})

	// 测试节点端点
	t.Run("Nodes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/containerha/nodes", nil)
		w := httptest.NewRecorder()

		handler.handleNodes(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不匹配，期望: 200, 实际: %d", w.Code)
		}

		var response APIResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}

		if !response.Success {
			t.Error("响应应该成功")
		}
	})

	// 测试配置端点
	t.Run("Config", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/containerha/config", nil)
		w := httptest.NewRecorder()

		handler.handleConfig(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不匹配，期望: 200, 实际: %d", w.Code)
		}
	})

	// 测试容器端点
	t.Run("Containers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/containerha/containers", nil)
		w := httptest.NewRecorder()

		handler.handleContainers(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不匹配，期望: 200, 实际: %d", w.Code)
		}
	})

	// 测试心跳端点
	t.Run("Heartbeat", func(t *testing.T) {
		heartbeat := HeartbeatMessage{
			NodeID:    "node-2",
			Timestamp: time.Now(),
			Status:    "online",
			ResourceUsage: ResourceUsage{
				CPUUsage: 50.0,
			},
		}

		body, _ := json.Marshal(heartbeat)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/containerha/heartbeat", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		handler.handleHeartbeat(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("状态码不匹配，期望: 200, 实际: %d", w.Code)
		}
	})
}

// TestContainerHAHandler_Failover 测试故障转移API
func TestContainerHAHandler_Failover(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")
	handler := NewContainerHAHandler(manager)

	// 模拟节点在线
	node2, _ := manager.GetNode("node-2")
	node2.Status = "online"
	node2.HealthScore = 80

	// 设置容器在node-1上运行
	manager.containerMu.Lock()
	for _, container := range manager.containers {
		container.CurrentNode = "node-1"
		container.Status = "running"
	}
	manager.containerMu.Unlock()

	// 测试执行故障转移
	failoverReq := FailoverRequest{
		TargetNode: "node-2",
		Containers: []string{"web-app-1"},
		Reason:     "测试",
	}

	body, _ := json.Marshal(failoverReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/containerha/failover", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	handler.handleFailover(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不匹配，期望: 200, 实际: %d", w.Code)
	}

	var response APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if !response.Success {
		t.Error("故障转移应该成功")
	}
}

// TestContainerHAHandler_UpdateConfig 测试更新配置API
func TestContainerHAHandler_UpdateConfig(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")
	handler := NewContainerHAHandler(manager)

	// 创建新配置
	newConfig := createTestConfig()
	newConfig.ClusterName = "new-cluster"

	body, _ := json.Marshal(newConfig)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/containerha/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	handler.handleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不匹配，期望: 200, 实际: %d", w.Code)
	}

	// 验证配置已更新
	retrievedConfig := manager.GetConfig()
	if retrievedConfig.ClusterName != "new-cluster" {
		t.Errorf("配置未更新，期望: new-cluster, 实际: %s", retrievedConfig.ClusterName)
	}
}

// TestContainerHAHandler_MethodNotAllowed 测试方法不允许
func TestContainerHAHandler_MethodNotAllowed(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")
	handler := NewContainerHAHandler(manager)

	// 测试错误的方法
	req := httptest.NewRequest(http.MethodPost, "/api/v1/containerha/status", nil)
	w := httptest.NewRecorder()

	handler.handleStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("状态码不匹配，期望: 405, 实际: %d", w.Code)
	}
}

// TestExtractIDFromPath 测试从路径中提取ID
func TestExtractIDFromPath(t *testing.T) {
	tests := []struct {
		path     string
		prefix   string
		expected string
	}{
		{"/api/v1/containerha/nodes/node-1", "/api/v1/containerha/nodes/", "node-1"},
		{"/api/v1/containerha/nodes/node-1/", "/api/v1/containerha/nodes/", "node-1"},
		{"/api/v1/containerha/nodes/", "/api/v1/containerha/nodes/", ""},
		{"/api/v1/containerha/other", "/api/v1/containerha/nodes/", ""},
	}

	for _, test := range tests {
		result := extractIDFromPath(test.path, test.prefix)
		if result != test.expected {
			t.Errorf("路径 %s: 期望 %s, 实际 %s", test.path, test.expected, result)
		}
	}
}

// TestCountOnlineNodes 测试计算在线节点数
func TestCountOnlineNodes(t *testing.T) {
	nodes := []ContainerHANode{
		{ID: "node-1", Status: "online"},
		{ID: "node-2", Status: "offline"},
		{ID: "node-3", Status: "online"},
		{ID: "node-4", Status: "degraded"},
	}

	count := countOnlineNodes(nodes)
	if count != 2 {
		t.Errorf("在线节点数不匹配，期望: 2, 实际: %d", count)
	}
}

// TestFailoverManager_calculateHealthScore 测试健康分数计算
func TestFailoverManager_calculateHealthScore(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	tests := []struct {
		usage    ResourceUsage
		expected int
	}{
		{ResourceUsage{CPUUsage: 50, MemoryUsage: 60, DiskUsage: 70}, 100},
		{ResourceUsage{CPUUsage: 80, MemoryUsage: 80, DiskUsage: 85}, 60},
		{ResourceUsage{CPUUsage: 95, MemoryUsage: 95, DiskUsage: 98}, 20},
		{ResourceUsage{CPUUsage: 100, MemoryUsage: 100, DiskUsage: 100}, 20},
	}

	for _, test := range tests {
		score := manager.calculateHealthScore(test.usage)
		if score != test.expected {
			t.Errorf("CPU: %.0f, Memory: %.0f, Disk: %.0f: 期望 %d, 实际 %d",
				test.usage.CPUUsage, test.usage.MemoryUsage, test.usage.DiskUsage,
				test.expected, score)
		}
	}
}

// TestFailoverManager_isResourceExhausted 测试资源耗尽检查
func TestFailoverManager_isResourceExhausted(t *testing.T) {
	config := createTestConfig()
	config.ResourceThresholds = ResourceThresholds{
		CPUThreshold:    90.0,
		MemoryThreshold: 85.0,
		DiskThreshold:   95.0,
	}
	manager := NewFailoverManager(config, "node-1")

	tests := []struct {
		usage    ResourceUsage
		expected bool
	}{
		{ResourceUsage{CPUUsage: 50, MemoryUsage: 60, DiskUsage: 70}, false},
		{ResourceUsage{CPUUsage: 95, MemoryUsage: 60, DiskUsage: 70}, true},
		{ResourceUsage{CPUUsage: 50, MemoryUsage: 90, DiskUsage: 70}, true},
		{ResourceUsage{CPUUsage: 50, MemoryUsage: 60, DiskUsage: 98}, true},
	}

	for _, test := range tests {
		result := manager.isResourceExhausted(test.usage)
		if result != test.expected {
			t.Errorf("CPU: %.0f, Memory: %.0f, Disk: %.0f: 期望 %v, 实际 %v",
				test.usage.CPUUsage, test.usage.MemoryUsage, test.usage.DiskUsage,
				test.expected, result)
		}
	}
}

// TestFailoverManager_calculateClusterStatus 测试集群状态计算
func TestFailoverManager_calculateClusterStatus(t *testing.T) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	// 所有节点在线
	manager.nodeMu.Lock()
	manager.nodes["node-1"].Status = "online"
	manager.nodes["node-2"].Status = "online"
	manager.nodes["node-3"].Status = "online"
	manager.nodeMu.Unlock()

	status := manager.calculateClusterStatus()
	if status != "healthy" {
		t.Errorf("期望 healthy，实际 %s", status)
	}

	// 部分节点在线
	manager.nodeMu.Lock()
	manager.nodes["node-3"].Status = "offline"
	manager.nodeMu.Unlock()

	status = manager.calculateClusterStatus()
	if status != "degraded" {
		t.Errorf("期望 degraded，实际 %s", status)
	}

	// 大部分节点离线
	manager.nodeMu.Lock()
	manager.nodes["node-2"].Status = "offline"
	manager.nodeMu.Unlock()

	status = manager.calculateClusterStatus()
	if status != "critical" {
		t.Errorf("期望 critical，实际 %s", status)
	}
}

// BenchmarkFailoverManager_GetStatus 性能测试
func BenchmarkFailoverManager_GetStatus(b *testing.B) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetStatus()
	}
}

// BenchmarkFailoverManager_GetNodes 性能测试
func BenchmarkFailoverManager_GetNodes(b *testing.B) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetNodes()
	}
}

// BenchmarkContainerHAHandler_Status 性能测试
func BenchmarkContainerHAHandler_Status(b *testing.B) {
	config := createTestConfig()
	manager := NewFailoverManager(config, "node-1")
	handler := NewContainerHAHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/containerha/status", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.handleStatus(w, req)
	}
}
