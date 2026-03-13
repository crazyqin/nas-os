package cluster

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	ErrNodeNotFound      = errors.New("node not found")
	ErrNodeAlreadyExists = errors.New("node already exists")
	ErrClusterNotReady   = errors.New("cluster not ready")
	ErrNoLeader          = errors.New("no leader available")
	ErrSplitBrain        = errors.New("split brain detected")
)

// NodeState 节点状态
type NodeState string

const (
	NodeStateActive   NodeState = "active"
	NodeStateInactive NodeState = "inactive"
	NodeStateSuspect  NodeState = "suspect"
	NodeStateFailed   NodeState = "failed"
)

// NodeRole 节点角色
type NodeRole string

const (
	NodeRoleLeader   NodeRole = "leader"
	NodeRoleFollower NodeRole = "follower"
	NodeRoleCandidate NodeRole = "candidate"
)

// Node 集群节点
type Node struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Address       string            `json:"address"`
	Port          int               `json:"port"`
	Role          NodeRole          `json:"role"`
	State         NodeState         `json:"state"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	LastSeen      time.Time         `json:"last_seen"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Priority      int               `json:"priority"` // 用于领导者选举优先级
}

// ClusterConfig 集群配置
type ClusterConfig struct {
	NodeID              string        `json:"node_id"`
	NodeName            string        `json:"node_name"`
	Address             string        `json:"address"`
	Port                int           `json:"port"`
	Peers               []PeerConfig  `json:"peers"`
	HeartbeatInterval   time.Duration `json:"heartbeat_interval"`
	HeartbeatTimeout    time.Duration `json:"heartbeat_timeout"`
	FailureThreshold    int           `json:"failure_threshold"`    // 心跳失败次数阈值
	ElectionTimeout     time.Duration `json:"election_timeout"`     // 选举超时
	SplitBrainThreshold int           `json:"split_brain_threshold"` // 脑裂检测阈值（需要多少节点确认）
}

// PeerConfig 对等节点配置
type PeerConfig struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// Cluster 集群管理器
type Cluster struct {
	config    *ClusterConfig
	localNode *Node
	nodes     map[string]*Node
	leader    *Node
	
	// 故障检测
	heartbeatFailures map[string]int
	detector          *FailureDetector
	
	// 脑裂防护
	splitBrainGuard *SplitBrainGuard
	
	// 选举
	electionState *ElectionState
	
	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
	
	logger *zap.Logger
}

// NewCluster 创建集群管理器
func NewCluster(config *ClusterConfig, logger *zap.Logger) *Cluster {
	ctx, cancel := context.WithCancel(context.Background())
	
	c := &Cluster{
		config: config,
		localNode: &Node{
			ID:       config.NodeID,
			Name:     config.NodeName,
			Address:  config.Address,
			Port:     config.Port,
			Role:     NodeRoleFollower,
			State:    NodeStateActive,
			Priority: 100,
		},
		nodes:             make(map[string]*Node),
		heartbeatFailures: make(map[string]int),
		ctx:               ctx,
		cancel:            cancel,
		logger:            logger,
	}
	
	// 初始化故障检测器
	c.detector = NewFailureDetector(config.HeartbeatTimeout, config.FailureThreshold, logger)
	
	// 初始化脑裂防护
	c.splitBrainGuard = NewSplitBrainGuard(config.SplitBrainThreshold, logger)
	
	// 初始化选举状态
	c.electionState = NewElectionState(config.ElectionTimeout)
	
	// 添加本地节点
	c.nodes[config.NodeID] = c.localNode
	
	// 添加对等节点
	for _, peer := range config.Peers {
		c.nodes[peer.ID] = &Node{
			ID:       peer.ID,
			Name:     peer.Name,
			Address:  peer.Address,
			Port:     peer.Port,
			Role:     NodeRoleFollower,
			State:    NodeStateInactive,
			Priority: 50, // 默认优先级
		}
	}
	
	return c
}

// Start 启动集群
func (c *Cluster) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 启动心跳
	c.wg.Add(1)
	go c.heartbeatLoop()
	
	// 启动故障检测
	c.wg.Add(1)
	go c.failureDetectionLoop()
	
	// 启动选举
	c.wg.Add(1)
	go c.electionLoop()
	
	// 启动脑裂检测
	c.wg.Add(1)
	go c.splitBrainDetectionLoop()
	
	c.logger.Info("Cluster started",
		zap.String("node_id", c.config.NodeID),
		zap.Int("peers", len(c.config.Peers)),
	)
	
	return nil
}

// Stop 停止集群
func (c *Cluster) Stop() {
	c.cancel()
	c.wg.Wait()
	c.logger.Info("Cluster stopped")
}

// heartbeatLoop 心跳循环
func (c *Cluster) heartbeatLoop() {
	defer c.wg.Done()
	
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.sendHeartbeats()
		}
	}
}

// sendHeartbeats 发送心跳
func (c *Cluster) sendHeartbeats() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	for id, node := range c.nodes {
		if id == c.localNode.ID {
			continue
		}
		
		// 发送心跳（这里用模拟实现）
		if err := c.sendHeartbeat(node); err != nil {
			c.logger.Debug("Heartbeat failed",
				zap.String("node_id", id),
				zap.Error(err),
			)
			c.heartbeatFailures[id]++
		} else {
			c.heartbeatFailures[id] = 0
			node.LastHeartbeat = time.Now()
			node.State = NodeStateActive
		}
	}
}

// sendHeartbeat 发送单个心跳
func (c *Cluster) sendHeartbeat(node *Node) error {
	// 实际实现中这里应该是网络调用
	// 这里使用模拟实现，假设心跳成功
	return nil
}

// failureDetectionLoop 故障检测循环
func (c *Cluster) failureDetectionLoop() {
	defer c.wg.Done()
	
	ticker := time.NewTicker(c.config.HeartbeatTimeout / 2)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.detectFailures()
		}
	}
}

// detectFailures 检测故障
func (c *Cluster) detectFailures() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	
	for id, node := range c.nodes {
		if id == c.localNode.ID {
			continue
		}
		
		// 使用 Phi Accrual 故障检测器
		phi := c.detector.Phi(id, now.Sub(node.LastHeartbeat))

		if phi > c.detector.Threshold() {
			// 故障确认
			if node.State != NodeStateFailed {
				c.logger.Warn("Node failure detected",
					zap.String("node_id", id),
					zap.Float64("phi", phi),
				)
				node.State = NodeStateFailed

				// 如果是领导者故障，触发选举
				if node.Role == NodeRoleLeader {
					c.logger.Info("Leader failed, triggering election")
					c.electionState.StartElection()
				}
			}
		} else if phi > c.detector.Threshold()/2 {
			// 可疑状态
			if node.State == NodeStateActive {
				node.State = NodeStateSuspect
				c.logger.Debug("Node suspect",
					zap.String("node_id", id),
					zap.Float64("phi", phi),
				)
			}
		}
	}
}

// electionLoop 选举循环
func (c *Cluster) electionLoop() {
	defer c.wg.Done()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.electionState.ElectionChan():
			c.performElection()
		}
	}
}

// performElection 执行选举
func (c *Cluster) performElection() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.logger.Info("Starting leader election")
	
	// 收集活跃节点
	var activeNodes []*Node
	for _, node := range c.nodes {
		if node.State == NodeStateActive || node.ID == c.localNode.ID {
			activeNodes = append(activeNodes, node)
		}
	}
	
	if len(activeNodes) == 0 {
		c.logger.Error("No active nodes for election")
		return
	}
	
	// 按优先级选择领导者
	var bestCandidate *Node
	for _, node := range activeNodes {
		if bestCandidate == nil || node.Priority > bestCandidate.Priority {
			bestCandidate = node
		}
	}
	
	if bestCandidate != nil {
		// 更新所有节点角色
		for _, node := range c.nodes {
			if node.ID == bestCandidate.ID {
				node.Role = NodeRoleLeader
			} else {
				node.Role = NodeRoleFollower
			}
		}
		
		c.leader = bestCandidate
		c.logger.Info("Leader elected",
			zap.String("leader_id", bestCandidate.ID),
			zap.String("leader_name", bestCandidate.Name),
		)
	}
}

// splitBrainDetectionLoop 脑裂检测循环
func (c *Cluster) splitBrainDetectionLoop() {
	defer c.wg.Done()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.checkSplitBrain()
		}
	}
}

// checkSplitBrain 检查脑裂
func (c *Cluster) checkSplitBrain() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// 收集活跃节点
	var activeNodes []*Node
	for _, node := range c.nodes {
		if node.State == NodeStateActive {
			activeNodes = append(activeNodes, node)
		}
	}
	
	// 检查是否存在多个领导者
	leaderCount := 0
	for _, node := range activeNodes {
		if node.Role == NodeRoleLeader {
			leaderCount++
		}
	}
	
	if leaderCount > 1 {
		c.logger.Error("Split brain detected",
			zap.Int("leaders", leaderCount),
		)
		c.splitBrainGuard.HandleSplitBrain(activeNodes)
	}
	
	// 检查网络分区
	if len(activeNodes) < c.config.SplitBrainThreshold {
		c.logger.Warn("Network partition suspected",
			zap.Int("active_nodes", len(activeNodes)),
			zap.Int("threshold", c.config.SplitBrainThreshold),
		)
		c.splitBrainGuard.HandlePartition(activeNodes)
	}
}

// GetLeader 获取当前领导者
func (c *Cluster) GetLeader() (*Node, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.leader == nil {
		return nil, ErrNoLeader
	}
	return c.leader, nil
}

// GetNodes 获取所有节点
func (c *Cluster) GetNodes() []*Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	nodes := make([]*Node, 0, len(c.nodes))
	for _, node := range c.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetNode 获取指定节点
func (c *Cluster) GetNode(id string) (*Node, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	node, ok := c.nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return node, nil
}

// IsLeader 当前节点是否是领导者
func (c *Cluster) IsLeader() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.localNode.Role == NodeRoleLeader
}

// GetLocalNode 获取本地节点
func (c *Cluster) GetLocalNode() *Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.localNode
}

// ClusterStats 集群统计
type ClusterStats struct {
	TotalNodes   int `json:"total_nodes"`
	ActiveNodes  int `json:"active_nodes"`
	FailedNodes  int `json:"failed_nodes"`
	LeaderID     string `json:"leader_id"`
	IsLeader     bool  `json:"is_leader"`
}

// GetStats 获取集群统计
func (c *Cluster) GetStats() ClusterStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	stats := ClusterStats{
		TotalNodes: len(c.nodes),
		IsLeader:   c.localNode.Role == NodeRoleLeader,
	}
	
	if c.leader != nil {
		stats.LeaderID = c.leader.ID
	}
	
	for _, node := range c.nodes {
		switch node.State {
		case NodeStateActive:
			stats.ActiveNodes++
		case NodeStateFailed:
			stats.FailedNodes++
		}
	}
	
	return stats
}