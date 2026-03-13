package cluster

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewCluster(t *testing.T) {
	logger := zap.NewNop()

	config := &ClusterConfig{
		NodeID:              "node1",
		NodeName:            "test-node-1",
		Address:             "192.168.1.1",
		Port:                7946,
		HeartbeatInterval:   1 * time.Second,
		HeartbeatTimeout:    5 * time.Second,
		FailureThreshold:    3,
		ElectionTimeout:     10 * time.Second,
		SplitBrainThreshold: 2,
		Peers: []PeerConfig{
			{ID: "node2", Name: "test-node-2", Address: "192.168.1.2", Port: 7946},
		},
	}

	cluster := NewCluster(config, logger)

	assert.NotNil(t, cluster)
	assert.Equal(t, "node1", cluster.localNode.ID)
	assert.Equal(t, NodeRoleFollower, cluster.localNode.Role)
	assert.Len(t, cluster.nodes, 2) // local + 1 peer
}

func TestCluster_GetLeader(t *testing.T) {
	logger := zap.NewNop()

	config := &ClusterConfig{
		NodeID:              "node1",
		NodeName:            "test-node-1",
		Address:             "192.168.1.1",
		Port:                7946,
		HeartbeatInterval:   1 * time.Second,
		HeartbeatTimeout:    5 * time.Second,
		FailureThreshold:    3,
		ElectionTimeout:     10 * time.Second,
		SplitBrainThreshold: 2,
	}

	cluster := NewCluster(config, logger)

	// 初始状态没有领导者
	_, err := cluster.GetLeader()
	assert.Error(t, err)
	assert.Equal(t, ErrNoLeader, err)

	// 设置领导者
	cluster.mu.Lock()
	cluster.leader = cluster.localNode
	cluster.mu.Unlock()

	leader, err := cluster.GetLeader()
	assert.NoError(t, err)
	assert.Equal(t, "node1", leader.ID)
}

func TestCluster_GetNodes(t *testing.T) {
	logger := zap.NewNop()

	config := &ClusterConfig{
		NodeID:              "node1",
		NodeName:            "test-node-1",
		Address:             "192.168.1.1",
		Port:                7946,
		HeartbeatInterval:   1 * time.Second,
		HeartbeatTimeout:    5 * time.Second,
		FailureThreshold:    3,
		ElectionTimeout:     10 * time.Second,
		SplitBrainThreshold: 2,
		Peers: []PeerConfig{
			{ID: "node2", Name: "test-node-2", Address: "192.168.1.2", Port: 7946},
			{ID: "node3", Name: "test-node-3", Address: "192.168.1.3", Port: 7946},
		},
	}

	cluster := NewCluster(config, logger)

	nodes := cluster.GetNodes()
	assert.Len(t, nodes, 3)
}

func TestCluster_IsLeader(t *testing.T) {
	logger := zap.NewNop()

	config := &ClusterConfig{
		NodeID:              "node1",
		NodeName:            "test-node-1",
		Address:             "192.168.1.1",
		Port:                7946,
		HeartbeatInterval:   1 * time.Second,
		HeartbeatTimeout:    5 * time.Second,
		FailureThreshold:    3,
		ElectionTimeout:     10 * time.Second,
		SplitBrainThreshold: 2,
	}

	cluster := NewCluster(config, logger)

	// 初始状态不是领导者
	assert.False(t, cluster.IsLeader())

	// 设置为领导者
	cluster.mu.Lock()
	cluster.localNode.Role = NodeRoleLeader
	cluster.mu.Unlock()

	assert.True(t, cluster.IsLeader())
}

func TestCluster_GetStats(t *testing.T) {
	logger := zap.NewNop()

	config := &ClusterConfig{
		NodeID:              "node1",
		NodeName:            "test-node-1",
		Address:             "192.168.1.1",
		Port:                7946,
		HeartbeatInterval:   1 * time.Second,
		HeartbeatTimeout:    5 * time.Second,
		FailureThreshold:    3,
		ElectionTimeout:     10 * time.Second,
		SplitBrainThreshold: 2,
		Peers: []PeerConfig{
			{ID: "node2", Name: "test-node-2", Address: "192.168.1.2", Port: 7946},
		},
	}

	cluster := NewCluster(config, logger)

	// 设置节点状态
	cluster.mu.Lock()
	cluster.nodes["node2"].State = NodeStateActive
	cluster.mu.Unlock()

	stats := cluster.GetStats()

	assert.Equal(t, 2, stats.TotalNodes)
	assert.Equal(t, 2, stats.ActiveNodes)
	assert.Equal(t, 0, stats.FailedNodes)
}

func TestFailureDetector_Phi(t *testing.T) {
	logger := zap.NewNop()
	fd := NewFailureDetector(5*time.Second, 3, logger)

	// 记录一些心跳
	for i := 0; i < 10; i++ {
		fd.RecordHeartbeat("node1")
		time.Sleep(10 * time.Millisecond)
	}

	// 测试 Phi 值
	phi := fd.Phi("node1", 100*time.Millisecond)
	assert.LessOrEqual(t, phi, fd.Threshold())

	// 测试长时间未收到心跳
	phi = fd.Phi("node1", 10*time.Second)
	assert.Greater(t, phi, fd.Threshold())
}

func TestFailureDetector_IsFailed(t *testing.T) {
	logger := zap.NewNop()
	fd := NewFailureDetector(5*time.Second, 3, logger)

	// 记录心跳
	for i := 0; i < 10; i++ {
		fd.RecordHeartbeat("node1")
		time.Sleep(10 * time.Millisecond)
	}

	// 短时间不算故障
	assert.False(t, fd.IsFailed("node1", 100*time.Millisecond))

	// 长时间算故障
	assert.True(t, fd.IsFailed("node1", 10*time.Second))
}

func TestSplitBrainGuard_CheckQuorum(t *testing.T) {
	logger := zap.NewNop()
	sbg := NewSplitBrainGuard(2, logger)

	// 测试仲裁
	status := sbg.CheckQuorum(5, 3)
	assert.True(t, status.Healthy)
	assert.True(t, status.CanProceed)

	status = sbg.CheckQuorum(5, 2)
	assert.False(t, status.Healthy)
	assert.False(t, status.CanProceed)
}

func TestSplitBrainGuard_CanCommit(t *testing.T) {
	logger := zap.NewNop()
	sbg := NewSplitBrainGuard(2, logger)

	// 设置仲裁大小
	sbg.SetQuorumSize(3)

	// 测试提交权限
	assert.True(t, sbg.CanCommit(3))
	assert.True(t, sbg.CanCommit(4))
	assert.False(t, sbg.CanCommit(2))
}

func TestSplitBrainGuard_HandleSplitBrain(t *testing.T) {
	logger := zap.NewNop()
	sbg := NewSplitBrainGuard(2, logger)

	// 创建多个领导者的节点
	nodes := []*Node{
		{ID: "node1", Role: NodeRoleLeader, State: NodeStateActive, Priority: 100},
		{ID: "node2", Role: NodeRoleLeader, State: NodeStateActive, Priority: 50},
		{ID: "node3", Role: NodeRoleFollower, State: NodeStateActive, Priority: 30},
	}

	sbg.HandleSplitBrain(nodes)

	// 检查事件记录
	events := sbg.GetEvents(10)
	assert.Greater(t, len(events), 0)
}

func TestElectionState_VoteRequest(t *testing.T) {
	es := NewElectionState(10 * time.Second)

	// 测试投票请求
	req := VoteRequest{
		Term:        1,
		CandidateID: "node1",
	}

	resp := es.RequestVote(req)
	assert.True(t, resp.VoteGranted)
	assert.Equal(t, uint64(1), es.GetCurrentTerm()) // 任期应该在内部被更新

	// 再次投票给同一候选人
	resp = es.RequestVote(req)
	assert.True(t, resp.VoteGranted)

	// 投票给不同候选人
	req2 := VoteRequest{
		Term:        1,
		CandidateID: "node2",
	}
	resp = es.RequestVote(req2)
	assert.False(t, resp.VoteGranted)

	// 更高任期的投票
	req3 := VoteRequest{
		Term:        2,
		CandidateID: "node2",
	}
	resp = es.RequestVote(req3)
	assert.True(t, resp.VoteGranted)
}

func TestElectionState_Heartbeat(t *testing.T) {
	es := NewElectionState(10 * time.Second)

	// 设置任期
	es.SetTerm(1)

	// 测试心跳
	req := HeartbeatRequest{
		Term:     2,
		LeaderID: "node1",
	}

	resp := es.ProcessHeartbeat(req)
	assert.True(t, resp.Success)
	assert.Equal(t, uint64(2), es.GetCurrentTerm())
}

func TestFailoverManager_SelectNewLeader(t *testing.T) {
	strategy := NewDefaultFailoverStrategy("priority")

	nodes := []*Node{
		{ID: "node1", State: NodeStateActive, Priority: 50},
		{ID: "node2", State: NodeStateActive, Priority: 100},
		{ID: "node3", State: NodeStateActive, Priority: 75},
	}

	failedNode := &Node{ID: "node0"}

	leader, err := strategy.SelectNewLeader(nodes, failedNode)
	assert.NoError(t, err)
	assert.Equal(t, "node2", leader.ID) // 最高优先级
}

func TestFailoverManager_SelectNewLeader_NoCandidates(t *testing.T) {
	strategy := NewDefaultFailoverStrategy("priority")

	nodes := []*Node{
		{ID: "node1", State: NodeStateFailed, Priority: 50},
		{ID: "node2", State: NodeStateInactive, Priority: 100},
	}

	failedNode := &Node{ID: "node0"}

	_, err := strategy.SelectNewLeader(nodes, failedNode)
	assert.Error(t, err)
	assert.Equal(t, ErrClusterNotReady, err)
}

func TestFollowerTracker(t *testing.T) {
	ft := NewFollowerTracker()

	// 添加跟随者
	ft.AddFollower("node1")
	ft.AddFollower("node2")

	// 更新状态
	ft.UpdateFollower("node1", 10)
	ft.UpdateFollower("node2", 8)

	// 获取信息
	info := ft.GetFollower("node1")
	assert.NotNil(t, info)
	assert.Equal(t, uint64(10), info.MatchIndex)

	// 计算提交索引
	commitIdx := ft.ComputeCommitIndex(12)
	assert.GreaterOrEqual(t, commitIdx, uint64(8))
}

func TestAccrualLevel(t *testing.T) {
	logger := zap.NewNop()
	fd := NewFailureDetector(5*time.Second, 3, logger)

	// 记录心跳
	for i := 0; i < 10; i++ {
		fd.RecordHeartbeat("node1")
		time.Sleep(10 * time.Millisecond)
	}

	// 测试累积级别
	level := fd.GetAccrualLevel("node1", 100*time.Millisecond)
	assert.Equal(t, AccrualLevelHealthy, level)

	level = fd.GetAccrualLevel("node1", 10*time.Second)
	assert.Equal(t, AccrualLevelCritical, level)
}
