package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ========== 高可用集成测试 ==========

// NodeState 节点状态
type NodeState string

const (
	NodeStateFollower  NodeState = "follower"
	NodeStateCandidate NodeState = "candidate"
	NodeStateLeader    NodeState = "leader"
)

// HANode 高可用节点
type HANode struct {
	mu          sync.RWMutex
	ID          string
	Address     string
	State       NodeState
	Term        uint64
	LeaderID    string
	LastContact time.Time
	IsHealthy   bool
}

func NewHANode(id, address string) *HANode {
	return &HANode{
		ID:          id,
		Address:     address,
		State:       NodeStateFollower,
		Term:        0,
		LastContact: time.Now(),
		IsHealthy:   true,
	}
}

func (n *HANode) RequestVote(term uint64, candidateID string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if term > n.Term {
		n.Term = term
		return true
	}
	return false
}

func (n *HANode) Heartbeat(term uint64, leaderID string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if term >= n.Term {
		n.Term = term
		n.LeaderID = leaderID
		n.LastContact = time.Now()
		n.State = NodeStateFollower
		return true
	}
	return false
}

func (n *HANode) SetState(state NodeState) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.State = state
}

func (n *HANode) GetState() NodeState {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.State
}

// HACluster 高可用集群
type HACluster struct {
	mu     sync.RWMutex
	nodes  map[string]*HANode
	leader string
	term   uint64
}

func NewHACluster() *HACluster {
	return &HACluster{
		nodes: make(map[string]*HANode),
	}
}

func (c *HACluster) AddNode(node *HANode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nodes[node.ID] = node
}

func (c *HACluster) RemoveNode(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.nodes, nodeID)
}

func (c *HACluster) GetNodes() []*HANode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	nodes := make([]*HANode, 0, len(c.nodes))
	for _, n := range c.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

func (c *HACluster) SetLeader(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.leader = nodeID
	if node := c.nodes[nodeID]; node != nil {
		node.State = NodeStateLeader
	}
}

func (c *HACluster) GetLeader() *HANode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.leader == "" {
		return nil
	}
	return c.nodes[c.leader]
}

func (c *HACluster) IncrementTerm() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.term++
	return c.term
}

// TestLeaderElection 测试 Leader 选举
func TestLeaderElection(t *testing.T) {
	cluster := NewHACluster()

	// 创建 3 节点集群
	nodes := make([]*HANode, 3)
	for i := 0; i < 3; i++ {
		node := NewHANode(fmt.Sprintf("node-%d", i), fmt.Sprintf("192.168.1.%d:8080", i+1))
		cluster.AddNode(node)
		nodes[i] = node
	}

	// 初始状态：所有节点都是 follower
	for _, node := range nodes {
		if node.GetState() != NodeStateFollower {
			t.Errorf("expected initial state to be follower, got %s", node.GetState())
		}
	}

	// 模拟选举
	term := cluster.IncrementTerm()
	nodes[0].SetState(NodeStateCandidate)

	// 请求投票
	granted1 := nodes[1].RequestVote(term, nodes[0].ID)
	granted2 := nodes[2].RequestVote(term, nodes[0].ID)

	if !granted1 || !granted2 {
		t.Error("expected votes to be granted")
	}

	// node-0 成为 leader
	cluster.SetLeader(nodes[0].ID)
	if cluster.GetLeader() == nil || cluster.GetLeader().ID != "node-0" {
		t.Error("expected node-0 to become leader")
	}
}

// TestHeartbeat 测试心跳
func TestHeartbeat(t *testing.T) {
	cluster := NewHACluster()

	leader := NewHANode("leader", "192.168.1.1:8080")
	follower := NewHANode("follower", "192.168.1.2:8080")

	cluster.AddNode(leader)
	cluster.AddNode(follower)
	cluster.SetLeader(leader.ID)

	// Leader 发送心跳
	term := cluster.IncrementTerm()
	success := follower.Heartbeat(term, leader.ID)

	if !success {
		t.Error("heartbeat should succeed")
	}

	// 验证 follower 状态
	if follower.LeaderID != leader.ID {
		t.Errorf("expected leader ID to be '%s', got '%s'", leader.ID, follower.LeaderID)
	}

	if follower.GetState() != NodeStateFollower {
		t.Errorf("expected state to be follower, got %s", follower.GetState())
	}
}

// TestFailover 测试故障转移
func TestFailover(t *testing.T) {
	cluster := NewHACluster()

	// 创建 3 节点集群
	leader := NewHANode("leader", "192.168.1.1:8080")
	follower1 := NewHANode("follower1", "192.168.1.2:8080")
	follower2 := NewHANode("follower2", "192.168.1.3:8080")

	cluster.AddNode(leader)
	cluster.AddNode(follower1)
	cluster.AddNode(follower2)
	cluster.SetLeader(leader.ID)

	// 模拟 leader 故障
	cluster.RemoveNode(leader.ID)

	// 新选举
	term := cluster.IncrementTerm()
	follower1.SetState(NodeStateCandidate)
	follower2.RequestVote(term, follower1.ID)
	cluster.SetLeader(follower1.ID)

	// 验证新 leader
	if cluster.GetLeader() == nil || cluster.GetLeader().ID != "follower1" {
		t.Error("expected follower1 to become new leader")
	}
}

// TestNodeRecovery 测试节点恢复
func TestNodeRecovery(t *testing.T) {
	cluster := NewHACluster()

	leader := NewHANode("leader", "192.168.1.1:8080")
	recoveringNode := NewHANode("recovering", "192.168.1.2:8080")

	cluster.AddNode(leader)
	cluster.AddNode(recoveringNode)
	cluster.SetLeader(leader.ID)

	// 节点恢复后同步状态
	success := recoveringNode.Heartbeat(1, leader.ID)
	if !success {
		t.Error("recovery heartbeat should succeed")
	}

	// 验证节点已正确恢复
	if recoveringNode.LeaderID != leader.ID {
		t.Errorf("expected leader ID to be '%s', got '%s'", leader.ID, recoveringNode.LeaderID)
	}
}

// TestConcurrentElection 测试并发选举
func TestConcurrentElection(t *testing.T) {
	cluster := NewHACluster()

	nodes := make([]*HANode, 3)
	for i := 0; i < 3; i++ {
		nodes[i] = NewHANode(fmt.Sprintf("node-%d", i), fmt.Sprintf("192.168.1.%d:8080", i+1))
		cluster.AddNode(nodes[i])
	}

	// 多个节点同时成为 candidate
	term := cluster.IncrementTerm()
	nodes[0].SetState(NodeStateCandidate)
	nodes[1].SetState(NodeStateCandidate)

	// 分散投票
	grant0 := nodes[2].RequestVote(term, nodes[0].ID)
	grant1 := nodes[2].RequestVote(term, nodes[1].ID)

	// 只有第一个请求应该成功（同 term）
	if grant0 && grant1 {
		t.Error("vote should only be granted once per term")
	}
}

// TestHeartbeatTimeout 测试心跳超时检测
func TestHeartbeatTimeout(t *testing.T) {
	follower := NewHANode("follower", "192.168.1.1:8080")
	follower.LastContact = time.Now().Add(-500 * time.Millisecond)

	// 检测超时
	timeout := 300 * time.Millisecond
	if time.Since(follower.LastContact) < timeout {
		t.Error("expected heartbeat timeout to be detected")
	}
}

// TestClusterQuorum 测试集群仲裁
func TestClusterQuorum(t *testing.T) {
	testCases := []struct {
		nodeCount int
		majority  int
	}{
		{3, 2},
		{5, 3},
		{7, 4},
	}

	for _, tc := range testCases {
		cluster := NewHACluster()

		for i := 0; i < tc.nodeCount; i++ {
			node := NewHANode(fmt.Sprintf("node-%d", i), fmt.Sprintf("192.168.1.%d:8080", i+1))
			cluster.AddNode(node)
		}

		nodes := cluster.GetNodes()
		if len(nodes) != tc.nodeCount {
			t.Errorf("expected %d nodes, got %d", tc.nodeCount, len(nodes))
		}

		// 验证多数派
		majority := len(nodes)/2 + 1
		if majority != tc.majority {
			t.Errorf("expected majority %d, got %d", tc.majority, majority)
		}
	}
}

// TestHAWithContext 测试带上下文的操作
func TestHAWithContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	cluster := NewHACluster()
	leader := NewHANode("leader", "192.168.1.1:8080")
	cluster.AddNode(leader)
	cluster.SetLeader(leader.ID)

	select {
	case <-ctx.Done():
		t.Log("context cancelled as expected")
	case <-time.After(200 * time.Millisecond):
		t.Error("expected context to cancel")
	}
}

// BenchmarkLeaderElection Leader 选举基准测试
func BenchmarkLeaderElection(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cluster := NewHACluster()

		for j := 0; j < 3; j++ {
			node := NewHANode(fmt.Sprintf("node-%d-%d", i, j), fmt.Sprintf("192.168.1.%d:8080", j+1))
			cluster.AddNode(node)
		}

		cluster.SetLeader("node-0")
	}
}

// BenchmarkHeartbeat 心跳基准测试
func BenchmarkHeartbeat(b *testing.B) {
	leader := NewHANode("leader", "192.168.1.1:8080")
	follower := NewHANode("follower", "192.168.1.2:8080")

	cluster := NewHACluster()
	cluster.AddNode(leader)
	cluster.AddNode(follower)
	cluster.SetLeader(leader.ID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		follower.Heartbeat(uint64(i), leader.ID)
	}
}
