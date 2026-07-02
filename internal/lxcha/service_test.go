package lxcha

import (
	"testing"
	"time"
)

// ========== 类型与常量测试 ==========

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() 返回 nil")
	}
	if cfg.ClusterName == "" {
		t.Error("默认 ClusterName 不应为空")
	}
	if cfg.HealthCheckSeconds <= 0 {
		t.Error("HealthCheckSeconds 应为正数")
	}
	if cfg.NodeID == "" {
		t.Error("默认 NodeID 不应为空")
	}
}

func TestContainerStateConstants(t *testing.T) {
	if StateRunning != "running" {
		t.Error("StateRunning 应为 'running'")
	}
	if StateStopped != "stopped" {
		t.Error("StateStopped 应为 'stopped'")
	}
	if StateMigrating != "migrating" {
		t.Error("StateMigrating 应为 'migrating'")
	}
}

func TestFailoverPolicyTypeConstants(t *testing.T) {
	if PolicyAuto != "auto" {
		t.Error("PolicyAuto 应为 'auto'")
	}
	if PolicyManual != "manual" {
		t.Error("PolicyManual 应为 'manual'")
	}
	if PPolicyNone != "none" {
		t.Error("PPolicyNone 应为 'none'")
	}
}

func TestFailoverStateTypeConstants(t *testing.T) {
	if FStateHealthy != "healthy" {
		t.Error("FStateHealthy 应为 'healthy'")
	}
	if FStateDegraded != "degraded" {
		t.Error("FStateDegraded 应为 'degraded'")
	}
	if FStateFailed != "failed" {
		t.Error("FStateFailed 应为 'failed'")
	}
	if FStateFailover != "failover" {
		t.Error("FStateFailover 应为 'failover'")
	}
}

func TestNodeTypeConstants(t *testing.T) {
	if NodeRolePrimary != "primary" {
		t.Error("NodeRolePrimary 应为 'primary'")
	}
	if NodeRoleBackup != "backup" {
		t.Error("NodeRoleBackup 应为 'backup'")
	}
	if NodeRoleWitness != "witness" {
		t.Error("NodeRoleWitness 应为 'witness'")
	}
}

// ========== Service 基础测试 ==========

func TestNewService(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("NewService(nil) 返回 nil")
	}
	if svc.config == nil {
		t.Error("config 未初始化")
	}
	if svc.containers == nil {
		t.Error("containers map 未初始化")
	}
	if svc.nodes == nil {
		t.Error("nodes map 未初始化")
	}
	if svc.policies == nil {
		t.Error("policies map 未初始化")
	}
	if svc.failoverStates == nil {
		t.Error("failoverStates map 未初始化")
	}
	if svc.ipReservations == nil {
		t.Error("ipReservations map 未初始化")
	}
}

func TestNewServiceWithConfig(t *testing.T) {
	cfg := &Config{
		ClusterName:        "test-cluster",
		HealthCheckSeconds: 5,
		NodeID:             "test-node-1",
	}
	svc := NewService(cfg)
	if svc.config != cfg {
		t.Error("config 应为传入的配置")
	}
	if svc.config.ClusterName != "test-cluster" {
		t.Error("ClusterName 不匹配")
	}
}

// ========== 节点管理测试 ==========

func TestRegisterNode(t *testing.T) {
	svc := NewService(nil)
	node := &HANode{
		ID:    "node-1",
		Name:  "node-1",
		Role:  NodeRolePrimary,
		State: NodeStateOnline,
	}
	err := svc.RegisterNode(node)
	if err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	// 验证节点存在
	got, err := svc.GetNode("node-1")
	if err != nil {
		t.Fatalf("获取节点失败: %v", err)
	}
	if got.Name != "node-1" {
		t.Errorf("名称期望 node-1，得到 %s", got.Name)
	}
	if got.LastSeen.IsZero() {
		t.Error("LastSeen 应被设置")
	}
}

func TestRegisterNodeDuplicate(t *testing.T) {
	svc := NewService(nil)
	node := &HANode{ID: "node-1", Name: "node-1"}
	_ = svc.RegisterNode(node)
	err := svc.RegisterNode(node)
	if err == nil {
		t.Error("重复注册应返回错误")
	}
}

func TestRegisterNodeEmptyID(t *testing.T) {
	svc := NewService(nil)
	node := &HANode{ID: "", Name: "empty"}
	err := svc.RegisterNode(node)
	if err == nil {
		t.Error("空 ID 应返回错误")
	}
}

func TestRemoveNode(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-1", Name: "node-1"})
	err := svc.RemoveNode("node-1")
	if err != nil {
		t.Fatalf("移除节点失败: %v", err)
	}

	_, err = svc.GetNode("node-1")
	if err == nil {
		t.Error("移除后应无法获取节点")
	}
}

func TestRemoveNodeWithContainers(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-1", Name: "node-1"})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{
		ContainerID: "lxc-100",
		Policy:      PolicyAuto,
	})
	// 容器默认在 config.NodeID 上，手动设置
	svc.mu.Lock()
	svc.containers["lxc-100"].NodeID = "node-1"
	svc.mu.Unlock()

	err := svc.RemoveNode("node-1")
	if err == nil {
		t.Error("节点上有容器时应返回错误")
	}
}

func TestRemoveNodeNotFound(t *testing.T) {
	svc := NewService(nil)
	err := svc.RemoveNode("nonexistent")
	if err == nil {
		t.Error("不存在的节点应返回错误")
	}
}

func TestGetNodesEmpty(t *testing.T) {
	svc := NewService(nil)
	nodes := svc.GetNodes()
	if nodes == nil {
		t.Fatal("GetNodes() 返回 nil")
	}
	if len(nodes) != 0 {
		t.Errorf("期望 0 个节点，得到 %d", len(nodes))
	}
}

func TestUpdateNodeHeartbeat(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-1", Name: "node-1"})
	oldTime := svc.nodes["node-1"].LastSeen
	time.Sleep(10 * time.Millisecond)
	err := svc.UpdateNodeHeartbeat("node-1")
	if err != nil {
		t.Fatalf("心跳更新失败: %v", err)
	}
	if !svc.nodes["node-1"].LastSeen.After(oldTime) {
		t.Error("LastSeen 应更新")
	}
}

func TestUpdateNodeHeartbeatNotFound(t *testing.T) {
	svc := NewService(nil)
	err := svc.UpdateNodeHeartbeat("nonexistent")
	if err == nil {
		t.Error("不存在的节点心跳应返回错误")
	}
}

// ========== 容器注册测试 ==========

func TestRegisterContainer(t *testing.T) {
	svc := NewService(nil)
	req := &RegisterContainerRequest{
		ContainerID: "lxc-100",
		Policy:      PolicyAuto,
		Priority:    1,
	}
	container, err := svc.RegisterContainer(req)
	if err != nil {
		t.Fatalf("注册容器失败: %v", err)
	}
	if container.ID != "lxc-100" {
		t.Errorf("ID 期望 lxc-100，得到 %s", container.ID)
	}
	if !container.HAEnabled {
		t.Error("HAEnabled 应为 true")
	}
	if container.Policy != PolicyAuto {
		t.Errorf("Policy 期望 auto，得到 %s", container.Policy)
	}

	// 验证策略已创建
	policy, err := svc.GetPolicy("lxc-100")
	if err != nil {
		t.Fatalf("获取策略失败: %v", err)
	}
	if policy.Type != PolicyAuto {
		t.Error("策略类型应为 auto")
	}

	// 验证故障转移状态已初始化
	state, err := svc.GetFailoverState("lxc-100")
	if err != nil {
		t.Fatalf("获取故障转移状态失败: %v", err)
	}
	if state.State != FStateHealthy {
		t.Errorf("初始状态应为 healthy，得到 %s", state.State)
	}
}

func TestRegisterContainerDuplicate(t *testing.T) {
	svc := NewService(nil)
	req := &RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto}
	_, _ = svc.RegisterContainer(req)
	_, err := svc.RegisterContainer(req)
	if err == nil {
		t.Error("重复注册应返回错误")
	}
}

func TestRegisterContainerWithIP(t *testing.T) {
	svc := NewService(nil)
	req := &RegisterContainerRequest{
		ContainerID: "lxc-100",
		Policy:      PolicyAuto,
		IPConfigs: []*StaticIPConfig{
			{Interface: "eth0", Address: "192.168.1.100/24", Gateway: "192.168.1.1"},
		},
	}
	_, err := svc.RegisterContainer(req)
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	// 验证 IP 预留
	reservations := svc.GetIPReservations()
	if len(reservations) != 1 {
		t.Fatalf("期望 1 条 IP 预留，得到 %d", len(reservations))
	}
	if reservations[0].IP != "192.168.1.100/24" {
		t.Errorf("IP 期望 192.168.1.100/24，得到 %s", reservations[0].IP)
	}
}

func TestUnregisterContainer(t *testing.T) {
	svc := NewService(nil)
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{
		ContainerID: "lxc-100",
		Policy:      PolicyAuto,
		IPConfigs: []*StaticIPConfig{
			{Address: "192.168.1.100/24"},
		},
	})
	err := svc.UnregisterContainer("lxc-100")
	if err != nil {
		t.Fatalf("取消注册失败: %v", err)
	}

	// 验证容器已移除
	_, err = svc.GetContainer("lxc-100")
	if err == nil {
		t.Error("取消注册后应无法获取容器")
	}

	// 验证 IP 预留已移除
	reservations := svc.GetIPReservations()
	if len(reservations) != 0 {
		t.Errorf("期望 0 条 IP 预留，得到 %d", len(reservations))
	}
}

func TestUnregisterContainerNotFound(t *testing.T) {
	svc := NewService(nil)
	err := svc.UnregisterContainer("nonexistent")
	if err == nil {
		t.Error("不存在的容器应返回错误")
	}
}

func TestGetContainers(t *testing.T) {
	svc := NewService(nil)
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-200", Policy: PolicyManual})

	containers := svc.GetContainers()
	if len(containers) != 2 {
		t.Errorf("期望 2 个容器，得到 %d", len(containers))
	}
}

func TestUpdateContainerState(t *testing.T) {
	svc := NewService(nil)
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})
	err := svc.UpdateContainerState("lxc-100", StateRunning)
	if err != nil {
		t.Fatalf("更新状态失败: %v", err)
	}
	c, _ := svc.GetContainer("lxc-100")
	if c.State != StateRunning {
		t.Errorf("状态期望 running，得到 %s", c.State)
	}
}

// ========== 策略管理测试 ==========

func TestGetPolicyNotFound(t *testing.T) {
	svc := NewService(nil)
	_, err := svc.GetPolicy("nonexistent")
	if err == nil {
		t.Error("不存在的容器策略应返回错误")
	}
}

func TestUpdatePolicy(t *testing.T) {
	svc := NewService(nil)
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})

	req := &UpdatePolicyRequest{
		ContainerID:    "lxc-100",
		Type:           PolicyManual,
		PreferredNode:  "node-2",
		MaxRetries:     5,
		HealthCheckInt: 15,
		FailoverDelay:  10,
	}
	policy, err := svc.UpdatePolicy(req)
	if err != nil {
		t.Fatalf("更新策略失败: %v", err)
	}
	if policy.Type != PolicyManual {
		t.Error("策略类型应为 manual")
	}
	if policy.PreferredNode != "node-2" {
		t.Error("PreferredNode 应为 node-2")
	}
	if policy.MaxRetries != 5 {
		t.Error("MaxRetries 应为 5")
	}

	// 验证容器 HA 状态同步
	c, _ := svc.GetContainer("lxc-100")
	if c.Policy != PolicyManual {
		t.Error("容器策略应同步为 manual")
	}
}

func TestUpdatePolicyDisableHA(t *testing.T) {
	svc := NewService(nil)
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})

	_, err := svc.UpdatePolicy(&UpdatePolicyRequest{
		ContainerID: "lxc-100",
		Type:        PPolicyNone,
	})
	if err != nil {
		t.Fatalf("更新策略失败: %v", err)
	}
	c, _ := svc.GetContainer("lxc-100")
	if c.HAEnabled {
		t.Error("HAEnabled 应为 false")
	}
}

// ========== 容器迁移测试 ==========

func TestMigrateContainer(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-1", Name: "node-1", Role: NodeRolePrimary, State: NodeStateOnline})
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", Role: NodeRoleBackup, State: NodeStateOnline})

	_, _ = svc.RegisterContainer(&RegisterContainerRequest{
		ContainerID: "lxc-100",
		Policy:      PolicyAuto,
		IPConfigs:   []*StaticIPConfig{{Address: "192.168.1.100/24"}},
	})
	// 设置容器在 node-1 上
	svc.mu.Lock()
	svc.containers["lxc-100"].NodeID = "node-1"
	svc.containers["lxc-100"].State = StateRunning
	svc.mu.Unlock()

	req := &MigrateRequest{
		ContainerID: "lxc-100",
		TargetNode:  "node-2",
		Online:      true,
		KeepIP:      true,
	}
	result, err := svc.MigrateContainer(req)
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if !result.Success {
		t.Error("迁移应成功")
	}
	if result.SourceNode != "node-1" {
		t.Errorf("源节点期望 node-1，得到 %s", result.SourceNode)
	}
	if result.TargetNode != "node-2" {
		t.Errorf("目标节点期望 node-2，得到 %s", result.TargetNode)
	}

	// 验证容器已迁移
	c, _ := svc.GetContainer("lxc-100")
	if c.NodeID != "node-2" {
		t.Errorf("容器 NodeID 期望 node-2，得到 %s", c.NodeID)
	}
	if c.TargetNodeID != "" {
		t.Error("迁移完成后 TargetNodeID 应为空")
	}
}

func TestMigrateContainerNodeNotFound(t *testing.T) {
	svc := NewService(nil)
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})

	req := &MigrateRequest{ContainerID: "lxc-100", TargetNode: "nonexistent"}
	_, err := svc.MigrateContainer(req)
	if err == nil {
		t.Error("目标节点不存在应返回错误")
	}
}

func TestMigrateContainerNodeOffline(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOffline})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})

	req := &MigrateRequest{ContainerID: "lxc-100", TargetNode: "node-2"}
	_, err := svc.MigrateContainer(req)
	if err == nil {
		t.Error("离线节点应返回错误")
	}
}

func TestMigrateContainerAlreadyMigrating(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOnline})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})
	svc.mu.Lock()
	svc.containers["lxc-100"].State = StateMigrating
	svc.mu.Unlock()

	req := &MigrateRequest{ContainerID: "lxc-100", TargetNode: "node-2"}
	_, err := svc.MigrateContainer(req)
	if err == nil {
		t.Error("迁移中的容器应返回错误")
	}
}

func TestMigrateContainerNotFound(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOnline})
	req := &MigrateRequest{ContainerID: "nonexistent", TargetNode: "node-2"}
	_, err := svc.MigrateContainer(req)
	if err == nil {
		t.Error("不存在的容器应返回错误")
	}
}

// ========== 故障检测测试 ==========

func TestCheckNodeHealth(t *testing.T) {
	svc := NewService(&Config{
		ClusterName:        "test",
		HealthCheckSeconds: 1,
		NodeID:             "node-1",
	})
	_ = svc.RegisterNode(&HANode{ID: "node-1", Name: "node-1", State: NodeStateOnline})
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOnline})

	// 模拟 node-2 心跳超时
	svc.mu.Lock()
	svc.nodeHeartbeats["node-2"] = time.Now().Add(-10 * time.Second)
	svc.mu.Unlock()

	failed := svc.CheckNodeHealth()
	if len(failed) != 1 {
		t.Fatalf("期望 1 个故障节点，得到 %d", len(failed))
	}
	if failed[0].ID != "node-2" {
		t.Errorf("故障节点应为 node-2，得到 %s", failed[0].ID)
	}
	if failed[0].State != NodeStateOffline {
		t.Error("故障节点状态应为 offline")
	}
}

func TestCheckNodeHealthAllOnline(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-1", Name: "node-1", State: NodeStateOnline})
	failed := svc.CheckNodeHealth()
	if len(failed) != 0 {
		t.Errorf("期望 0 个故障节点，得到 %d", len(failed))
	}
}

// ========== 手动故障转移测试 ==========

func TestTriggerFailover(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-1", Name: "node-1", State: NodeStateOnline})
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOnline})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})
	svc.mu.Lock()
	svc.containers["lxc-100"].NodeID = "node-1"
	svc.containers["lxc-100"].State = StateRunning
	svc.mu.Unlock()

	req := &TriggerFailoverRequest{
		ContainerID: "lxc-100",
		TargetNode:  "node-2",
	}
	result, err := svc.TriggerFailover(req)
	if err != nil {
		t.Fatalf("故障转移失败: %v", err)
	}
	if !result.Success {
		t.Error("故障转移应成功")
	}

	// 验证容器已迁移
	c, _ := svc.GetContainer("lxc-100")
	if c.NodeID != "node-2" {
		t.Errorf("容器应在 node-2 上，得到 %s", c.NodeID)
	}

	// 验证历史记录
	history := svc.GetHistory()
	if len(history) != 1 {
		t.Errorf("期望 1 条历史记录，得到 %d", len(history))
	}
	if !history[0].Success {
		t.Error("历史记录应显示成功")
	}
}

func TestTriggerFailoverNoTarget(t *testing.T) {
	svc := NewService(nil)
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})
	req := &TriggerFailoverRequest{ContainerID: "lxc-100"}
	_, err := svc.TriggerFailover(req)
	if err == nil {
		t.Error("无可用目标节点应返回错误")
	}
}

func TestTriggerFailoverDisabled(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOnline})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PPolicyNone})
	svc.mu.Lock()
	svc.containers["lxc-100"].NodeID = "node-1"
	svc.mu.Unlock()

	req := &TriggerFailoverRequest{ContainerID: "lxc-100"}
	_, err := svc.TriggerFailover(req)
	if err == nil {
		t.Error("禁用策略应返回错误")
	}
}

func TestTriggerFailoverForce(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOnline})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PPolicyNone})
	svc.mu.Lock()
	svc.containers["lxc-100"].NodeID = "node-1"
	svc.mu.Unlock()

	req := &TriggerFailoverRequest{ContainerID: "lxc-100", TargetNode: "node-2", Force: true}
	result, err := svc.TriggerFailover(req)
	if err != nil {
		t.Fatalf("强制故障转移应成功: %v", err)
	}
	if !result.Success {
		t.Error("强制故障转移应成功")
	}
}

// ========== 自动故障转移测试 ==========

func TestAutoFailover(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-1", Name: "node-1", State: NodeStateOnline})
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOnline, Role: NodeRoleBackup})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})
	svc.mu.Lock()
	svc.containers["lxc-100"].NodeID = "node-1"
	svc.containers["lxc-100"].State = StateRunning
	svc.mu.Unlock()

	result, err := svc.AutoFailover("lxc-100", "node-1")
	if err != nil {
		t.Fatalf("自动故障转移失败: %v", err)
	}
	if !result.Success {
		t.Error("自动故障转移应成功")
	}
	if result.TargetNode != "node-2" {
		t.Errorf("目标节点期望 node-2，得到 %s", result.TargetNode)
	}
}

func TestAutoFailoverPolicyManual(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOnline})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyManual})
	svc.mu.Lock()
	svc.containers["lxc-100"].NodeID = "node-1"
	svc.mu.Unlock()

	_, err := svc.AutoFailover("lxc-100", "node-1")
	if err == nil {
		t.Error("手动策略不应自动故障转移")
	}
}

func TestAutoFailoverNoTargetNode(t *testing.T) {
	svc := NewService(nil)
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})
	svc.mu.Lock()
	svc.containers["lxc-100"].NodeID = "node-1"
	svc.mu.Unlock()

	_, err := svc.AutoFailover("lxc-100", "node-1")
	if err == nil {
		t.Error("无可用目标节点应返回错误")
	}
}

// ========== IP 管理测试 ==========

func TestReserveIP(t *testing.T) {
	svc := NewService(nil)
	req := &ReserveIPRequest{IP: "192.168.1.100/24", ContainerID: "lxc-100", NodeID: "node-1"}
	reservation, err := svc.ReserveIP(req)
	if err != nil {
		t.Fatalf("预留失败: %v", err)
	}
	if reservation.IP != "192.168.1.100/24" {
		t.Error("IP 不匹配")
	}

	// 重复预留同 IP 同容器应更新节点
	req.NodeID = "node-2"
	reservation, err = svc.ReserveIP(req)
	if err != nil {
		t.Fatalf("更新预留失败: %v", err)
	}
	if reservation.NodeID != "node-2" {
		t.Error("NodeID 应更新为 node-2")
	}
}

func TestReserveIPConflict(t *testing.T) {
	svc := NewService(nil)
	_, _ = svc.ReserveIP(&ReserveIPRequest{IP: "192.168.1.100/24", ContainerID: "lxc-100", NodeID: "node-1"})

	_, err := svc.ReserveIP(&ReserveIPRequest{IP: "192.168.1.100/24", ContainerID: "lxc-200", NodeID: "node-1"})
	if err == nil {
		t.Error("不同容器预留同 IP 应返回错误")
	}
}

func TestReleaseIP(t *testing.T) {
	svc := NewService(nil)
	_, _ = svc.ReserveIP(&ReserveIPRequest{IP: "192.168.1.100/24", ContainerID: "lxc-100", NodeID: "node-1"})
	err := svc.ReleaseIP("192.168.1.100/24")
	if err != nil {
		t.Fatalf("释放失败: %v", err)
	}
	reservations := svc.GetIPReservations()
	if len(reservations) != 0 {
		t.Errorf("期望 0 条预留，得到 %d", len(reservations))
	}
}

func TestReleaseIPNotFound(t *testing.T) {
	svc := NewService(nil)
	err := svc.ReleaseIP("10.0.0.1")
	if err == nil {
		t.Error("未预留的 IP 应返回错误")
	}
}

func TestCheckIPConflict(t *testing.T) {
	svc := NewService(nil)
	_, _ = svc.ReserveIP(&ReserveIPRequest{IP: "192.168.1.100/24", ContainerID: "lxc-100", NodeID: "node-1"})

	// 同容器同节点不冲突
	if svc.CheckIPConflict("192.168.1.100/24", "lxc-100", "node-1") {
		t.Error("同容器同节点不应报告冲突")
	}

	// 不同容器冲突
	if !svc.CheckIPConflict("192.168.1.100/24", "lxc-200", "node-1") {
		t.Error("不同容器应报告冲突")
	}

	// 不存在的 IP 不冲突
	if svc.CheckIPConflict("10.0.0.1", "lxc-100", "node-1") {
		t.Error("不存在的 IP 不应报告冲突")
	}
}

// ========== 故障转移状态与事件测试 ==========

func TestGetFailoverStateNotFound(t *testing.T) {
	svc := NewService(nil)
	_, err := svc.GetFailoverState("nonexistent")
	if err == nil {
		t.Error("不存在的容器应返回错误")
	}
}

func TestGetFailoverEvents(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOnline})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})
	svc.mu.Lock()
	svc.containers["lxc-100"].NodeID = "node-1"
	svc.mu.Unlock()

	_, _ = svc.TriggerFailover(&TriggerFailoverRequest{ContainerID: "lxc-100", TargetNode: "node-2"})

	// 获取所有事件
	events := svc.GetFailoverEvents("")
	if len(events) != 1 {
		t.Errorf("期望 1 个事件，得到 %d", len(events))
	}

	// 获取特定容器事件
	events = svc.GetFailoverEvents("lxc-100")
	if len(events) != 1 {
		t.Errorf("期望 1 个事件，得到 %d", len(events))
	}

	// 获取不存在容器的事件
	events = svc.GetFailoverEvents("nonexistent")
	if len(events) != 0 {
		t.Errorf("期望 0 个事件，得到 %d", len(events))
	}
}

func TestGetHistory(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOnline})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})
	svc.mu.Lock()
	svc.containers["lxc-100"].NodeID = "node-1"
	svc.mu.Unlock()

	_, _ = svc.TriggerFailover(&TriggerFailoverRequest{ContainerID: "lxc-100", TargetNode: "node-2"})

	history := svc.GetHistory()
	if len(history) != 1 {
		t.Errorf("期望 1 条历史记录，得到 %d", len(history))
	}
}

// ========== 状态总览测试 ==========

func TestGetStatusEmpty(t *testing.T) {
	svc := NewService(nil)
	status := svc.GetStatus()
	if status == nil {
		t.Fatal("GetStatus() 返回 nil")
	}
	if status.TotalNodes != 0 {
		t.Errorf("期望 0 个节点，得到 %d", status.TotalNodes)
	}
	if status.TotalContainers != 0 {
		t.Errorf("期望 0 个容器，得到 %d", status.TotalContainers)
	}
	if status.History == nil {
		t.Error("History 不应为 nil")
	}
}

func TestGetStatus(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-1", Name: "node-1", State: NodeStateOnline})
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOnline})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-100", Policy: PolicyAuto})
	_, _ = svc.RegisterContainer(&RegisterContainerRequest{ContainerID: "lxc-200", Policy: PPolicyNone})

	status := svc.GetStatus()
	if status.TotalNodes != 2 {
		t.Errorf("期望 2 个节点，得到 %d", status.TotalNodes)
	}
	if status.OnlineNodes != 2 {
		t.Errorf("期望 2 个在线节点，得到 %d", status.OnlineNodes)
	}
	if status.TotalContainers != 2 {
		t.Errorf("期望 2 个容器，得到 %d", status.TotalContainers)
	}
	if status.HAContainers != 1 {
		t.Errorf("期望 1 个 HA 容器，得到 %d", status.HAContainers)
	}
}

// ========== Handler 测试 ==========

func TestNewHandler(t *testing.T) {
	svc := NewService(nil)
	handler := NewHandler(svc)
	if handler == nil {
		t.Fatal("NewHandler() 返回 nil")
	}
	if handler.service != svc {
		t.Error("Handler 的 service 应为传入的 Service")
	}
}

// ========== selectFailoverNode 测试 ==========

func TestSelectFailoverNode(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-1", Name: "node-1", State: NodeStateOnline, Role: NodeRolePrimary, Containers: 5, CPUUsage: 50})
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOnline, Role: NodeRoleBackup, Containers: 2, CPUUsage: 20})
	_ = svc.RegisterNode(&HANode{ID: "node-3", Name: "node-3", State: NodeStateOffline, Role: NodeRoleBackup})

	// 排除 node-1，应在 node-2 (负载更低且在线)
	svc.mu.Lock()
	target := svc.selectFailoverNode("node-1")
	svc.mu.Unlock()
	if target != "node-2" {
		t.Errorf("期望 node-2，得到 %s", target)
	}
}

func TestSelectFailoverNodeAllOffline(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-1", Name: "node-1", State: NodeStateOffline})
	_ = svc.RegisterNode(&HANode{ID: "node-2", Name: "node-2", State: NodeStateOffline})
	svc.mu.Lock()
	target := svc.selectFailoverNode("node-1")
	svc.mu.Unlock()
	if target != "" {
		t.Error("所有节点离线应返回空字符串")
	}
}

func TestSelectFailoverNodeSkipWitness(t *testing.T) {
	svc := NewService(nil)
	_ = svc.RegisterNode(&HANode{ID: "node-1", Name: "node-1", State: NodeStateOnline, Role: NodeRolePrimary})
	_ = svc.RegisterNode(&HANode{ID: "witness", Name: "witness", State: NodeStateOnline, Role: NodeRoleWitness})
	svc.mu.Lock()
	target := svc.selectFailoverNode("node-1")
	svc.mu.Unlock()
	if target != "" {
		t.Error("见证节点不应被选为故障转移目标")
	}
}
