package monitor

import (
	"sync"
	"testing"
	"time"
)

func TestNewDistributedMonitor(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	collector := NewMetricsCollector(mgr, nil)
	alertMgr := NewAlertingManager()

	dm := NewDistributedMonitor(mgr, collector, alertMgr)
	if dm == nil {
		t.Fatal("创建 DistributedMonitor 失败")
	}

	if dm.aggregationRule.Interval != time.Minute {
		t.Errorf("默认聚合间隔应为 1 分钟，实际为 %v", dm.aggregationRule.Interval)
	}

	if len(dm.alertRules) == 0 {
		t.Error("默认告警规则不应为空")
	}
}

func TestDefaultStoragePoolAlertRules(t *testing.T) {
	rules := DefaultStoragePoolAlertRules()

	if len(rules) < 3 {
		t.Errorf("默认告警规则应至少有 3 条，实际有 %d 条", len(rules))
	}

	// 检查是否包含必要的规则类型
	hasUsageWarning := false
	hasDeviceFailure := false
	for _, rule := range rules {
		if rule.Name == "pool_usage_warning" {
			hasUsageWarning = true
		}
		if rule.Name == "pool_device_failure" {
			hasDeviceFailure = true
		}
	}

	if !hasUsageWarning {
		t.Error("缺少存储池使用率警告规则")
	}
	if !hasDeviceFailure {
		t.Error("缺少存储池设备故障规则")
	}
}

func TestDistributedMonitor_RegisterNode(t *testing.T) {
	mgr, _ := NewManager()
	dm := NewDistributedMonitor(mgr, nil, nil)

	node := ClusterNodeInfo{
		ID:       "node-1",
		Name:     "test-node",
		Address:  "192.168.1.100",
		Port:     8080,
		IsActive: true,
		IsLeader: false,
	}

	dm.RegisterNode(node)

	dm.mu.RLock()
	found := false
	for _, n := range dm.clusterNodes {
		if n.ID == "node-1" {
			found = true
			if n.Name != "test-node" {
				t.Errorf("节点名称不匹配: 期望 test-node, 实际 %s", n.Name)
			}
		}
	}
	dm.mu.RUnlock()

	if !found {
		t.Error("节点注册失败")
	}

	// 测试更新
	updatedNode := ClusterNodeInfo{
		ID:       "node-1",
		Name:     "updated-node",
		Address:  "192.168.1.101",
		Port:     8081,
		IsActive: true,
		IsLeader: true,
	}

	dm.RegisterNode(updatedNode)

	dm.mu.RLock()
	for _, n := range dm.clusterNodes {
		if n.ID == "node-1" {
			if n.Name != "updated-node" {
				t.Errorf("节点名称未更新: 期望 updated-node, 实际 %s", n.Name)
			}
			if !n.IsLeader {
				t.Error("节点 IsLeader 标志未更新")
			}
		}
	}
	dm.mu.RUnlock()
}

func TestDistributedMonitor_UnregisterNode(t *testing.T) {
	mgr, _ := NewManager()
	dm := NewDistributedMonitor(mgr, nil, nil)

	node := ClusterNodeInfo{
		ID:       "node-1",
		Name:     "test-node",
		Address:  "192.168.1.100",
		Port:     8080,
		IsActive: true,
	}

	dm.RegisterNode(node)
	dm.UnregisterNode("node-1")

	dm.mu.RLock()
	for _, n := range dm.clusterNodes {
		if n.ID == "node-1" {
			t.Error("节点未正确注销")
		}
	}
	dm.mu.RUnlock()
}

func TestDistributedMonitor_UpdateNodeMetrics(t *testing.T) {
	mgr, _ := NewManager()
	dm := NewDistributedMonitor(mgr, nil, nil)

	metrics := &NodeMetrics{
		NodeID:    "node-1",
		NodeName:  "test-node",
		Timestamp: time.Now(),
		Status:    "healthy",
		SystemMetrics: &SystemMetricData{
			CPUUsage:    45.5,
			MemoryUsage: 60.2,
		},
	}

	dm.UpdateNodeMetrics(metrics)

	retrieved := dm.GetNodeMetrics("node-1")
	if retrieved == nil {
		t.Fatal("获取节点指标失败")
	}

	if retrieved.SystemMetrics.CPUUsage != 45.5 {
		t.Errorf("CPU 使用率不匹配: 期望 45.5, 实际 %f", retrieved.SystemMetrics.CPUUsage)
	}
}

func TestDistributedMonitor_AggregateMetrics(t *testing.T) {
	mgr, _ := NewManager()
	dm := NewDistributedMonitor(mgr, nil, nil)

	// 注册节点
	dm.RegisterNode(ClusterNodeInfo{ID: "node-1", IsActive: true})
	dm.RegisterNode(ClusterNodeInfo{ID: "node-2", IsActive: true})

	// 更新节点指标
	dm.UpdateNodeMetrics(&NodeMetrics{
		NodeID:   "node-1",
		NodeName: "node-1",
		SystemMetrics: &SystemMetricData{
			CPUUsage:    50.0,
			MemoryUsage: 60.0,
			MemoryTotal: 16 * 1024 * 1024 * 1024,
			MemoryUsed:  10 * 1024 * 1024 * 1024,
		},
		HealthScore: 85.0,
	})

	dm.UpdateNodeMetrics(&NodeMetrics{
		NodeID:   "node-2",
		NodeName: "node-2",
		SystemMetrics: &SystemMetricData{
			CPUUsage:    30.0,
			MemoryUsage: 40.0,
			MemoryTotal: 8 * 1024 * 1024 * 1024,
			MemoryUsed:  3 * 1024 * 1024 * 1024,
		},
		HealthScore: 90.0,
	})

	agg := dm.GetAggregatedMetrics()

	if agg.NodeCount != 2 {
		t.Errorf("节点数量不匹配: 期望 2, 实际 %d", agg.NodeCount)
	}

	// 平均 CPU 应该是 (50 + 30) / 2 = 40
	if agg.TotalCPU != 40.0 {
		t.Errorf("聚合 CPU 不匹配: 期望 40.0, 实际 %f", agg.TotalCPU)
	}

	// 平均健康评分应该是 (85 + 90) / 2 = 87.5
	if agg.ClusterHealth != 87.5 {
		t.Errorf("集群健康评分不匹配: 期望 87.5, 实际 %f", agg.ClusterHealth)
	}
}

func TestDistributedMonitor_AlertRules(t *testing.T) {
	mgr, _ := NewManager()
	dm := NewDistributedMonitor(mgr, nil, nil)

	initialCount := len(dm.GetAlertRules())

	newRule := StoragePoolAlertRule{
		Name:            "custom-pool-alert",
		PoolPattern:     "*",
		MetricType:      "usage",
		Threshold:       75,
		Duration:        5 * time.Minute,
		Level:           "warning",
		Enabled:         true,
		MessageTemplate: "自定义告警",
	}

	dm.AddAlertRule(newRule)

	rules := dm.GetAlertRules()
	if len(rules) != initialCount+1 {
		t.Errorf("添加规则后数量不匹配: 期望 %d, 实际 %d", initialCount+1, len(rules))
	}

	dm.RemoveAlertRule("custom-pool-alert")

	rules = dm.GetAlertRules()
	if len(rules) != initialCount {
		t.Errorf("删除规则后数量不匹配: 期望 %d, 实际 %d", initialCount, len(rules))
	}
}

func TestDistributedMonitor_StoragePoolMetrics(t *testing.T) {
	mgr, _ := NewManager()
	dm := NewDistributedMonitor(mgr, nil, nil)

	poolMetrics := []StoragePoolMetric{
		{
			PoolName:       "data-pool",
			PoolType:       "btrfs",
			TotalBytes:     10 * 1024 * 1024 * 1024 * 1024, // 10TB
			UsedBytes:      7 * 1024 * 1024 * 1024 * 1024,  // 7TB
			UsagePercent:   70.0,
			HealthStatus:   "healthy",
			RAIDLevel:      "raid5",
			DeviceCount:    4,
			HealthyDevices: 4,
			FailedDevices:  0,
		},
		{
			PoolName:       "backup-pool",
			PoolType:       "btrfs",
			TotalBytes:     5 * 1024 * 1024 * 1024 * 1024, // 5TB
			UsedBytes:      4 * 1024 * 1024 * 1024 * 1024, // 4TB
			UsagePercent:   90.0,
			HealthStatus:   "degraded",
			RAIDLevel:      "raid1",
			DeviceCount:    2,
			HealthyDevices: 1,
			FailedDevices:  1,
		},
	}

	dm.UpdateStoragePoolMetrics(poolMetrics)

	// 验证指标已更新
	nodeMetrics := dm.GetNodeMetrics(mgr.GetHostname())
	if nodeMetrics == nil {
		t.Fatal("获取节点指标失败")
	}

	if len(nodeMetrics.StorageMetrics) != 2 {
		t.Errorf("存储池数量不匹配: 期望 2, 实际 %d", len(nodeMetrics.StorageMetrics))
	}

	// 验证第一个池的数据
	for _, pool := range nodeMetrics.StorageMetrics {
		if pool.PoolName == "backup-pool" {
			if pool.HealthStatus != "degraded" {
				t.Errorf("存储池健康状态不匹配: 期望 degraded, 实际 %s", pool.HealthStatus)
			}
			if pool.FailedDevices != 1 {
				t.Errorf("故障设备数不匹配: 期望 1, 实际 %d", pool.FailedDevices)
			}
		}
	}
}

func TestDistributedMonitor_GetClusterStats(t *testing.T) {
	mgr, _ := NewManager()
	dm := NewDistributedMonitor(mgr, nil, nil)

	dm.RegisterNode(ClusterNodeInfo{ID: "node-1", IsActive: true})
	dm.RegisterNode(ClusterNodeInfo{ID: "node-2", IsActive: false})

	dm.UpdateNodeMetrics(&NodeMetrics{
		NodeID:    "node-1",
		Timestamp: time.Now(),
	})

	// 使用一个较旧的时间戳来模拟离线节点
	oldTime := time.Now().Add(-5 * time.Minute)
	dm.mu.Lock()
	dm.nodes["node-2"] = &NodeMetrics{
		NodeID:    "node-2",
		Timestamp: oldTime,
	}
	dm.mu.Unlock()

	stats := dm.GetClusterStats()

	if stats["total_nodes"].(int) != 2 {
		t.Errorf("总节点数不匹配: 期望 2, 实际 %d", stats["total_nodes"])
	}

	if stats["active_nodes"].(int) != 1 {
		t.Errorf("活跃节点数不匹配: 期望 1, 实际 %d", stats["active_nodes"])
	}

	// 检查在线/离线统计
	online := stats["nodes_online"].(int)
	offline := stats["nodes_offline"].(int)

	if online != 1 {
		t.Errorf("在线节点数不匹配: 期望 1, 实际 %d", online)
	}

	if offline != 1 {
		t.Errorf("离线节点数不匹配: 期望 1, 实际 %d", offline)
	}
}

func TestDistributedMonitor_StartStop(t *testing.T) {
	mgr, _ := NewManager()
	collector := NewMetricsCollector(mgr, nil)
	dm := NewDistributedMonitor(mgr, collector, nil)

	if err := dm.Start(); err != nil {
		t.Fatalf("启动分布式监控失败: %v", err)
	}

	// 等待一轮收集
	time.Sleep(100 * time.Millisecond)

	dm.Stop()
}

func TestDistributedMonitor_ConcurrentAccess(t *testing.T) {
	mgr, _ := NewManager()
	dm := NewDistributedMonitor(mgr, nil, nil)

	var wg sync.WaitGroup

	// 并发注册节点
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			node := ClusterNodeInfo{
				ID:       string(rune('A' + idx)),
				Name:     "concurrent-node",
				IsActive: true,
			}
			dm.RegisterNode(node)
		}(i)
	}

	// 并发更新指标
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			metrics := &NodeMetrics{
				NodeID: string(rune('A' + idx)),
				Status: "healthy",
			}
			dm.UpdateNodeMetrics(metrics)
		}(i)
	}

	// 并发读取
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = dm.GetAggregatedMetrics()
			_ = dm.GetClusterStats()
			_ = dm.GetAllNodeMetrics()
		}()
	}

	wg.Wait()

	// 验证数据一致性
	stats := dm.GetClusterStats()
	if stats["total_nodes"].(int) != 10 {
		t.Errorf("并发注册后节点数不正确: %d", stats["total_nodes"])
	}
}
