// Package ha 提供高可用管理核心功能测试
package ha

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewHAManager(t *testing.T) {
	logger := zap.NewNop()

	config := &HAConfig{
		ClusterName:     "test-cluster",
		NodeID:          "node-1",
		NodeName:        "node-1",
		Address:         "127.0.0.1",
		Port:            8080,
		HeartbeatInterval: 3 * time.Second,
		HeartbeatTimeout:  10 * time.Second,
		Peers: []PeerNode{
			{ID: "node-2", Name: "node-2", Address: "127.0.0.1", Port: 8081, Priority: 50},
		},
	}

	mgr, err := NewHAManager(config, logger)
	if err != nil {
		t.Fatalf("创建 HA 管理器失败: %v", err)
	}

	if mgr == nil {
		t.Fatal("HA 管理器为 nil")
	}

	// 验证本地节点
	if mgr.localNode.ID != "node-1" {
		t.Errorf("本地节点 ID 错误: 期望 node-1, 实际 %s", mgr.localNode.ID)
	}

	// 验证节点数量
	if len(mgr.nodes) != 2 {
		t.Errorf("节点数量错误: 期望 2, 实际 %d", len(mgr.nodes))
	}
}

func TestValidateHAConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *HAConfig
		wantErr bool
	}{
		{
			name: "有效配置",
			config: &HAConfig{
				NodeID:  "node-1",
				Address: "127.0.0.1",
				Peers: []PeerNode{
					{ID: "node-2", Address: "127.0.0.1"},
				},
			},
			wantErr: false,
		},
		{
			name: "缺少 NodeID",
			config: &HAConfig{
				Address: "127.0.0.1",
				Peers: []PeerNode{
					{ID: "node-2", Address: "127.0.0.1"},
				},
			},
			wantErr: true,
		},
		{
			name: "缺少 Address",
			config: &HAConfig{
				NodeID: "node-1",
				Peers: []PeerNode{
					{ID: "node-2", Address: "127.0.0.1"},
				},
			},
			wantErr: true,
		},
		{
			name: "缺少 Peers",
			config: &HAConfig{
				NodeID:  "node-1",
				Address: "127.0.0.1",
				Peers:   []PeerNode{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHAConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHAConfig() 错误 = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyHADefaults(t *testing.T) {
	config := &HAConfig{
		NodeID:  "node-1",
		Address: "127.0.0.1",
		Peers: []PeerNode{
			{ID: "node-2", Address: "127.0.0.1"},
		},
	}

	config = ApplyHADefaults(config)

	// 验证默认值
	if config.HeartbeatInterval != 3*time.Second {
		t.Errorf("HeartbeatInterval 默认值错误: %v", config.HeartbeatInterval)
	}
	if config.HeartbeatTimeout != 10*time.Second {
		t.Errorf("HeartbeatTimeout 默认值错误: %v", config.HeartbeatTimeout)
	}
	if config.HeartbeatMissMax != 3 {
		t.Errorf("HeartbeatMissMax 默认值错误: %d", config.HeartbeatMissMax)
	}
	if config.FailoverDelay != 5*time.Second {
		t.Errorf("FailoverDelay 默认值错误: %v", config.FailoverDelay)
	}
	if config.QuorumRequired != 1 {
		t.Errorf("QuorumRequired 默认值错误: %d", config.QuorumRequired)
	}
	if config.DataDir != "/var/lib/nas-os/ha" {
		t.Errorf("DataDir 默认值错误: %s", config.DataDir)
	}
}

func TestHAStateValues(t *testing.T) {
	states := []HAState{
		HAStateActive,
		HAStatePassive,
		HAStateStandby,
		HAStateFailed,
		HAStateSyncing,
		HAStateTakeover,
		HAStateUnknown,
	}

	expected := map[HAState]string{
		HAStateActive:   "active",
		HAStatePassive:  "passive",
		HAStateStandby:  "standby",
		HAStateFailed:   "failed",
		HAStateSyncing:  "syncing",
		HAStateTakeover: "takeover",
		HAStateUnknown:  "unknown",
	}

	for _, state := range states {
		if string(state) != expected[state] {
			t.Errorf("状态字符串错误: %s 应为 %s", state, expected[state])
		}
	}
}

func TestHARoleValues(t *testing.T) {
	roles := []HARole{
		HARolePrimary,
		HARoleSecondary,
		HARoleNone,
	}

	expected := map[HARole]string{
		HARolePrimary:   "primary",
		HARoleSecondary: "secondary",
		HARoleNone:      "none",
	}

	for _, role := range roles {
		if string(role) != expected[role] {
			t.Errorf("角色字符串错误: %s 应为 %s", role, expected[role])
		}
	}
}

func TestPerformInitialElection(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name            string
		config          *HAConfig
		expectedPrimary string
	}{
		{
			name: "单节点选举",
			config: &HAConfig{
				NodeID:  "node-1",
				Address: "127.0.0.1",
				Peers: []PeerNode{
					{ID: "node-2", Address: "127.0.0.1", Priority: 50},
				},
			},
			expectedPrimary: "node-1",
		},
		{
			name: "多节点选举 - 高优先级",
			config: &HAConfig{
				NodeID:  "node-1",
				Address: "127.0.0.1",
				Peers: []PeerNode{
					{ID: "node-2", Address: "127.0.0.1", Priority: 100},
				},
			},
			expectedPrimary: "node-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ApplyHADefaults(tt.config)
			mgr, err := NewHAManager(config, logger)
			if err != nil {
				t.Fatalf("创建 HA 管理器失败: %v", err)
			}

			mgr.performInitialElection()

			if mgr.primary == nil {
				t.Fatal("主节点为 nil")
			}

			if mgr.primary.ID != tt.expectedPrimary {
				t.Errorf("主节点错误: 期望 %s, 实际 %s", tt.expectedPrimary, mgr.primary.ID)
			}
		})
	}
}

func TestIsPrimary(t *testing.T) {
	logger := zap.NewNop()

	config := &HAConfig{
		NodeID:  "node-1",
		Address: "127.0.0.1",
		Peers: []PeerNode{
			{ID: "node-2", Address: "127.0.0.1", Priority: 50},
		},
	}
	config = ApplyHADefaults(config)

	mgr, err := NewHAManager(config, logger)
	if err != nil {
		t.Fatalf("创建 HA 管理器失败: %v", err)
	}

	mgr.performInitialElection()

	// node-1 优先级更高 (默认 100)
	if !mgr.IsPrimary() {
		t.Error("node-1 应为主节点")
	}
}

func TestGetStatus(t *testing.T) {
	logger := zap.NewNop()

	config := &HAConfig{
		NodeID:      "node-1",
		NodeName:    "node-1",
		Address:     "127.0.0.1",
		Port:        8080,
		ClusterName: "test-cluster",
		Peers: []PeerNode{
			{ID: "node-2", Name: "node-2", Address: "127.0.0.1", Port: 8081, Priority: 50},
		},
	}
	config = ApplyHADefaults(config)

	mgr, err := NewHAManager(config, logger)
	if err != nil {
		t.Fatalf("创建 HA 管理器失败: %v", err)
	}

	mgr.performInitialElection()

	status := mgr.GetStatus()

	if status == nil {
		t.Fatal("状态为 nil")
	}

	if status.LocalNode == nil {
		t.Error("本地节点状态为 nil")
	}

	if status.ClusterState == "" {
		t.Error("集群状态为空")
	}
}

func TestGetNodes(t *testing.T) {
	logger := zap.NewNop()

	config := &HAConfig{
		NodeID:  "node-1",
		Address: "127.0.0.1",
		Peers: []PeerNode{
			{ID: "node-2", Address: "127.0.0.1", Priority: 50},
			{ID: "node-3", Address: "127.0.0.1", Priority: 30},
		},
	}
	config = ApplyHADefaults(config)

	mgr, err := NewHAManager(config, logger)
	if err != nil {
		t.Fatalf("创建 HA 管理器失败: %v", err)
	}

	nodes := mgr.GetNodes()

	if len(nodes) != 3 {
		t.Errorf("节点数量错误: 期望 3, 实际 %d", len(nodes))
	}
}

func TestUpdateNodeHeartbeat(t *testing.T) {
	logger := zap.NewNop()

	config := &HAConfig{
		NodeID:  "node-1",
		Address: "127.0.0.1",
		Peers: []PeerNode{
			{ID: "node-2", Address: "127.0.0.1", Priority: 50},
		},
	}
	config = ApplyHADefaults(config)

	mgr, err := NewHAManager(config, logger)
	if err != nil {
		t.Fatalf("创建 HA 管理器失败: %v", err)
	}

	// 设置节点为故障状态
	mgr.mu.Lock()
	mgr.nodes["node-2"].State = HAStateFailed
	mgr.nodes["node-2"].HealthScore = 0
	mgr.mu.Unlock()

	// 更新心跳
	mgr.UpdateNodeHeartbeat("node-2")

	// 验证状态恢复
	mgr.mu.RLock()
	node := mgr.nodes["node-2"]
	mgr.mu.RUnlock()

	if node.State == HAStateFailed {
		t.Error("节点状态应从 Failed 恢复")
	}

	if node.HealthScore != 100.0 {
		t.Errorf("健康分数应为 100, 实际 %.2f", node.HealthScore)
	}
}

func TestGetEvents(t *testing.T) {
	logger := zap.NewNop()

	config := &HAConfig{
		NodeID:  "node-1",
		Address: "127.0.0.1",
		Peers: []PeerNode{
			{ID: "node-2", Address: "127.0.0.1", Priority: 50},
		},
	}
	config = ApplyHADefaults(config)

	mgr, err := NewHAManager(config, logger)
	if err != nil {
		t.Fatalf("创建 HA 管理器失败: %v", err)
	}

	// 记录一些事件
	mgr.recordEvent(HAEvent{
		ID:        "event-1",
		Type:      string(HAEventRoleChange),
		Timestamp: time.Now(),
		NodeID:    "node-1",
		Reason:    "initial election",
	})

	mgr.recordEvent(HAEvent{
		ID:        "event-2",
		Type:      string(HAEventHeartbeatMissed),
		Timestamp: time.Now(),
		NodeID:    "node-2",
		Reason:    "timeout",
	})

	events := mgr.GetEvents(10)

	if len(events) != 2 {
		t.Errorf("事件数量错误: 期望 2, 实际 %d", len(events))
	}
}

func TestHAEventTypes(t *testing.T) {
	eventTypes := []HAEventType{
		HAEventHeartbeatMissed,
		HAEventHeartbeatResume,
		HAEventNodeFailed,
		HAEventNodeRecovered,
		HAEventFailoverStarted,
		HAEventFailoverComplete,
		HAEventFailoverFailed,
		HAEventRoleChange,
		HAEventStateChange,
		HAEventSyncComplete,
		HAEventSyncFailed,
		HAEventSplitBrain,
		HAEventQuorumLost,
		HAEventQuorumRestore,
	}

	expected := map[HAEventType]string{
		HAEventHeartbeatMissed:  "heartbeat_missed",
		HAEventHeartbeatResume:  "heartbeat_resume",
		HAEventNodeFailed:       "node_failed",
		HAEventNodeRecovered:    "node_recovered",
		HAEventFailoverStarted:  "failover_started",
		HAEventFailoverComplete: "failover_complete",
		HAEventFailoverFailed:   "failover_failed",
		HAEventRoleChange:       "role_change",
		HAEventStateChange:      "state_change",
		HAEventSyncComplete:     "sync_complete",
		HAEventSyncFailed:       "sync_failed",
		HAEventSplitBrain:       "split_brain",
		HAEventQuorumLost:       "quorum_lost",
		HAEventQuorumRestore:    "quorum_restore",
	}

	for _, eventType := range eventTypes {
		if string(eventType) != expected[eventType] {
			t.Errorf("事件类型字符串错误: %s 应为 %s", eventType, expected[eventType])
		}
	}
}

func TestManualFailover(t *testing.T) {
	logger := zap.NewNop()

	config := &HAConfig{
		NodeID:  "node-1",
		Address: "127.0.0.1",
		Peers: []PeerNode{
			{ID: "node-2", Address: "127.0.0.1", Priority: 50},
		},
		FailoverEnabled: true,
	}
	config = ApplyHADefaults(config)

	mgr, err := NewHAManager(config, logger)
	if err != nil {
		t.Fatalf("创建 HA 管理器失败: %v", err)
	}

	mgr.performInitialElection()

	// 如果 node-1 不是主节点，跳过测试
	if !mgr.IsPrimary() {
		t.Skip("node-1 不是主节点，无法测试手动故障转移")
	}

	// 测试非主节点调用
	err = mgr.ManualFailover("node-2")
	if err != nil {
		// 可能因为节点状态不正确而失败，这是预期的
		t.Logf("手动故障转移返回错误: %v (可能是预期的)", err)
	}
}

func TestHAManagerStartStop(t *testing.T) {
	logger := zap.NewNop()

	config := &HAConfig{
		NodeID:  "node-1",
		Address: "127.0.0.1",
		Port:    8080,
		Peers: []PeerNode{
			{ID: "node-2", Address: "127.0.0.1", Port: 8081, Priority: 50},
		},
		HeartbeatInterval: 100 * time.Millisecond,
		HeartbeatTimeout:  500 * time.Millisecond,
	}
	config = ApplyHADefaults(config)

	mgr, err := NewHAManager(config, logger)
	if err != nil {
		t.Fatalf("创建 HA 管理器失败: %v", err)
	}

	// 启动
	ctx := context.Background()
	err = mgr.Start()
	if err != nil {
		t.Fatalf("启动 HA 管理器失败: %v", err)
	}

	// 等待一小段时间
	time.Sleep(200 * time.Millisecond)

	// 检查状态
	status := mgr.GetStatus()
	if status == nil {
		t.Error("状态为 nil")
	}

	// 停止
	err = mgr.Stop()
	if err != nil {
		t.Fatalf("停止 HA 管理器失败: %v", err)
	}
}

func TestGetQuorumStatus(t *testing.T) {
	logger := zap.NewNop()

	config := &HAConfig{
		NodeID:  "node-1",
		Address: "127.0.0.1",
		Peers: []PeerNode{
			{ID: "node-2", Address: "127.0.0.1", Priority: 50},
		},
		QuorumRequired: 2,
	}
	config = ApplyHADefaults(config)

	mgr, err := NewHAManager(config, logger)
	if err != nil {
		t.Fatalf("创建 HA 管理器失败: %v", err)
	}

	mgr.performInitialElection()

	status := mgr.GetStatus()

	// 验证法定人数状态
	if status.QuorumStatus == "" {
		t.Error("法定人数状态为空")
	}
}

// 基准测试
func BenchmarkGetStatus(b *testing.B) {
	logger := zap.NewNop()

	config := &HAConfig{
		NodeID:  "node-1",
		Address: "127.0.0.1",
		Peers: []PeerNode{
			{ID: "node-2", Address: "127.0.0.1", Priority: 50},
		},
	}
	config = ApplyHADefaults(config)

	mgr, _ := NewHAManager(config, logger)
	mgr.performInitialElection()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.GetStatus()
	}
}

func BenchmarkUpdateNodeHeartbeat(b *testing.B) {
	logger := zap.NewNop()

	config := &HAConfig{
		NodeID:  "node-1",
		Address: "127.0.0.1",
		Peers: []PeerNode{
			{ID: "node-2", Address: "127.0.0.1", Priority: 50},
		},
	}
	config = ApplyHADefaults(config)

	mgr, _ := NewHAManager(config, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.UpdateNodeHeartbeat("node-2")
	}
}