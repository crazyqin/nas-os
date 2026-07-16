package smartqos

import (
	"testing"
)

func TestEngine_CreatePolicy(t *testing.T) {
	engine := NewEngine()

	policy := &QoSPolicy{
		ID:       "policy-1",
		Name:     "测试策略",
		AppType:  AppDatabase,
		Priority: PriorityHigh,
		MaxIOPS:  50000,
		MinIOPS:  5000,
		Enabled:  true,
	}

	err := engine.CreatePolicy(policy)
	if err != nil {
		t.Fatalf("创建策略失败: %v", err)
	}

	fetched, err := engine.GetPolicy("policy-1")
	if err != nil {
		t.Fatalf("获取策略失败: %v", err)
	}
	if fetched.Name != "测试策略" {
		t.Errorf("期望名称 测试策略, 实际 %s", fetched.Name)
	}
}

func TestEngine_CreatePolicy_EmptyID(t *testing.T) {
	engine := NewEngine()

	err := engine.CreatePolicy(&QoSPolicy{ID: ""})
	if err == nil {
		t.Error("空ID应返回错误")
	}
}

func TestEngine_CreatePolicy_Duplicate(t *testing.T) {
	engine := NewEngine()

	policy := &QoSPolicy{ID: "policy-1", Name: "测试"}
	engine.CreatePolicy(policy)

	err := engine.CreatePolicy(policy)
	if err == nil {
		t.Error("重复创建应返回错误")
	}
}

func TestEngine_UpdatePolicy(t *testing.T) {
	engine := NewEngine()

	engine.CreatePolicy(&QoSPolicy{ID: "policy-1", Name: "原始名称", MaxIOPS: 10000})

	updated := &QoSPolicy{ID: "policy-1", Name: "更新名称", MaxIOPS: 20000}
	err := engine.UpdatePolicy(updated)
	if err != nil {
		t.Fatalf("更新策略失败: %v", err)
	}

	fetched, _ := engine.GetPolicy("policy-1")
	if fetched.Name != "更新名称" {
		t.Errorf("期望名称 更新名称, 实际 %s", fetched.Name)
	}
	if fetched.MaxIOPS != 20000 {
		t.Errorf("期望MaxIOPS 20000, 实际 %d", fetched.MaxIOPS)
	}
}

func TestEngine_DeletePolicy(t *testing.T) {
	engine := NewEngine()

	engine.CreatePolicy(&QoSPolicy{ID: "policy-1", Name: "测试"})
	err := engine.DeletePolicy("policy-1")
	if err != nil {
		t.Fatalf("删除策略失败: %v", err)
	}

	_, err = engine.GetPolicy("policy-1")
	if err == nil {
		t.Error("已删除的策略不应存在")
	}
}

func TestEngine_DeletePolicy_InUse(t *testing.T) {
	engine := NewEngine()

	engine.CreatePolicy(&QoSPolicy{ID: "policy-1", Name: "测试"})
	engine.RegisterNode(&QoSNode{ID: "node-1", Name: "节点1", PolicyID: "policy-1"})

	err := engine.DeletePolicy("policy-1")
	if err == nil {
		t.Error("正在使用的策略不应能删除")
	}
}

func TestEngine_RegisterNode(t *testing.T) {
	engine := NewEngine()

	node := &QoSNode{
		ID:   "node-1",
		Name: "存储节点1",
		Type: "disk",
		Path: "/dev/sda",
	}

	err := engine.RegisterNode(node)
	if err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	fetched, err := engine.GetNode("node-1")
	if err != nil {
		t.Fatalf("获取节点失败: %v", err)
	}
	if fetched.Name != "存储节点1" {
		t.Errorf("期望名称 存储节点1, 实际 %s", fetched.Name)
	}
}

func TestEngine_RegisterNode_EmptyID(t *testing.T) {
	engine := NewEngine()

	err := engine.RegisterNode(&QoSNode{ID: ""})
	if err == nil {
		t.Error("空ID应返回错误")
	}
}

func TestEngine_UnregisterNode(t *testing.T) {
	engine := NewEngine()

	engine.RegisterNode(&QoSNode{ID: "node-1", Name: "节点1"})
	err := engine.UnregisterNode("node-1")
	if err != nil {
		t.Fatalf("注销节点失败: %v", err)
	}

	_, err = engine.GetNode("node-1")
	if err == nil {
		t.Error("已注销的节点不应存在")
	}
}

func TestEngine_AssignPolicy(t *testing.T) {
	engine := NewEngine()

	engine.CreatePolicy(&QoSPolicy{ID: "policy-1", Name: "策略1", Enabled: true})
	engine.RegisterNode(&QoSNode{ID: "node-1", Name: "节点1"})

	err := engine.AssignPolicy("node-1", "policy-1")
	if err != nil {
		t.Fatalf("分配策略失败: %v", err)
	}

	node, _ := engine.GetNode("node-1")
	if node.PolicyID != "policy-1" {
		t.Errorf("期望策略ID policy-1, 实际 %s", node.PolicyID)
	}
}

func TestEngine_AssignPolicy_NodeNotFound(t *testing.T) {
	engine := NewEngine()

	err := engine.AssignPolicy("non-existent", "policy-1")
	if err == nil {
		t.Error("不存在的节点应返回错误")
	}
}

func TestEngine_ReportMetric(t *testing.T) {
	engine := NewEngine()

	engine.RegisterNode(&QoSNode{ID: "node-1", Name: "节点1"})

	metric := &IOMetric{
		IOPS:      10000,
		Bandwidth: 500,
		Latency:   5,
	}

	err := engine.ReportMetric("node-1", metric)
	if err != nil {
		t.Fatalf("上报指标失败: %v", err)
	}

	node, _ := engine.GetNode("node-1")
	if node.Metric.IOPS != 10000 {
		t.Errorf("期望IOPS 10000, 实际 %d", node.Metric.IOPS)
	}
}

func TestEngine_EvaluateQoS_Allow(t *testing.T) {
	engine := NewEngine()

	engine.CreatePolicy(&QoSPolicy{
		ID: "policy-1", Name: "策略1", Enabled: true,
		MaxIOPS: 50000, MaxBandwidth: 1000, MaxLatency: 10,
	})
	engine.RegisterNode(&QoSNode{ID: "node-1", Name: "节点1", PolicyID: "policy-1"})
	engine.ReportMetric("node-1", &IOMetric{IOPS: 10000, Bandwidth: 500, Latency: 5})

	allowed, reason, err := engine.EvaluateQoS("node-1")
	if err != nil {
		t.Fatalf("评估QoS失败: %v", err)
	}
	if !allowed {
		t.Errorf("应允许IO, 原因: %s", reason)
	}
}

func TestEngine_EvaluateQoS_Throttle_IOPS(t *testing.T) {
	engine := NewEngine()

	engine.CreatePolicy(&QoSPolicy{
		ID: "policy-1", Name: "策略1", Enabled: true,
		MaxIOPS: 50000, MaxBandwidth: 1000, MaxLatency: 10,
	})
	engine.RegisterNode(&QoSNode{ID: "node-1", Name: "节点1", PolicyID: "policy-1"})
	engine.ReportMetric("node-1", &IOMetric{IOPS: 60000, Bandwidth: 500, Latency: 5})

	allowed, reason, err := engine.EvaluateQoS("node-1")
	if err != nil {
		t.Fatalf("评估QoS失败: %v", err)
	}
	if allowed {
		t.Error("应限流, IOPS超限")
	}
	if reason == "" {
		t.Error("限流原因不应为空")
	}
}

func TestEngine_EvaluateQoS_Throttle_Latency(t *testing.T) {
	engine := NewEngine()

	engine.CreatePolicy(&QoSPolicy{
		ID: "policy-1", Name: "策略1", Enabled: true,
		MaxIOPS: 100000, MaxBandwidth: 5000, MaxLatency: 10,
	})
	engine.RegisterNode(&QoSNode{ID: "node-1", Name: "节点1", PolicyID: "policy-1"})
	engine.ReportMetric("node-1", &IOMetric{IOPS: 10000, Bandwidth: 500, Latency: 50})

	allowed, _, _ := engine.EvaluateQoS("node-1")
	if allowed {
		t.Error("应限流, 延迟超限")
	}
}

func TestEngine_EvaluateQoS_Burst(t *testing.T) {
	engine := NewEngine()

	engine.CreatePolicy(&QoSPolicy{
		ID: "policy-1", Name: "策略1", Enabled: true,
		MaxIOPS: 50000, BurstIOPS: 80000, BurstDuration: 30,
	})
	engine.RegisterNode(&QoSNode{ID: "node-1", Name: "节点1", PolicyID: "policy-1"})
	engine.ReportMetric("node-1", &IOMetric{IOPS: 70000, Bandwidth: 500, Latency: 5})

	allowed, reason, _ := engine.EvaluateQoS("node-1")
	if !allowed {
		t.Errorf("突发应允许, 原因: %s", reason)
	}
}

func TestEngine_EvaluateQoS_NodeNotFound(t *testing.T) {
	engine := NewEngine()

	_, _, err := engine.EvaluateQoS("non-existent")
	if err == nil {
		t.Error("不存在的节点应返回错误")
	}
}

func TestEngine_ListNodes(t *testing.T) {
	engine := NewEngine()

	engine.RegisterNode(&QoSNode{ID: "node-1", Name: "节点1"})
	engine.RegisterNode(&QoSNode{ID: "node-2", Name: "节点2"})

	nodes := engine.ListNodes()
	if len(nodes) != 2 {
		t.Errorf("期望2个节点, 实际 %d", len(nodes))
	}
}

func TestEngine_ListPolicies(t *testing.T) {
	engine := NewEngine()

	engine.CreatePolicy(&QoSPolicy{ID: "policy-1", Name: "自定义策略"})

	policies := engine.ListPolicies()
	// 自定义 + 默认策略
	if len(policies) < 1 {
		t.Errorf("期望至少1个策略, 实际 %d", len(policies))
	}
}

func TestEngine_GetDefaultPolicyForApp(t *testing.T) {
	engine := NewEngine()

	policy := engine.GetDefaultPolicyForApp(AppDatabase)
	if policy == nil {
		t.Fatal("数据库默认策略不应为空")
	}
	if policy.Priority != PriorityCritical {
		t.Errorf("期望优先级 %d, 实际 %d", PriorityCritical, policy.Priority)
	}
}

func TestEngine_GetStats(t *testing.T) {
	engine := NewEngine()

	engine.RegisterNode(&QoSNode{ID: "node-1", Name: "节点1"})
	engine.CreatePolicy(&QoSPolicy{ID: "policy-1", Name: "策略1"})

	stats := engine.GetStats()
	if stats["total_nodes"] != 1 {
		t.Errorf("期望1个节点, 实际 %v", stats["total_nodes"])
	}
}

func TestEngine_GetNodeMetrics(t *testing.T) {
	engine := NewEngine()

	engine.RegisterNode(&QoSNode{ID: "node-1", Name: "节点1"})
	engine.ReportMetric("node-1", &IOMetric{IOPS: 1000})
	engine.ReportMetric("node-1", &IOMetric{IOPS: 2000})

	metrics := engine.GetNodeMetrics("node-1", 0)
	if len(metrics) != 2 {
		t.Errorf("期望2条指标, 实际 %d", len(metrics))
	}

	limited := engine.GetNodeMetrics("node-1", 1)
	if len(limited) != 1 {
		t.Errorf("期望1条指标, 实际 %d", len(limited))
	}
}
