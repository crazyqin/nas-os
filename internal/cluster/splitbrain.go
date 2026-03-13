package cluster

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// SplitBrainGuard 脑裂防护机制
type SplitBrainGuard struct {
	threshold    int
	partitionMap map[string]*PartitionInfo
	events       []SplitBrainEvent
	mu           sync.RWMutex
	logger       *zap.Logger
	
	// 仲裁相关
	quorumSize      int
	lastQuorumCheck time.Time
	quorumHealthy   bool
}

// PartitionInfo 分区信息
type PartitionInfo struct {
	NodeID        string    `json:"node_id"`
	DetectedAt    time.Time `json:"detected_at"`
	ResolvedAt    time.Time `json:"resolved_at,omitempty"`
	PartitionID   string    `json:"partition_id"`
	IsLeader      bool      `json:"is_leader"`
	ActiveNodes   []string  `json:"active_nodes"`
}

// SplitBrainEvent 脑裂事件
type SplitBrainEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	Type         string    `json:"type"` // "detected", "resolved", "partition"
	Description  string    `json:"description"`
	Leaders      []string  `json:"leaders,omitempty"`
	Partitions   []string  `json:"partitions,omitempty"`
	Resolution   string    `json:"resolution,omitempty"`
}

// QuorumStatus 仲裁状态
type QuorumStatus struct {
	Healthy       bool      `json:"healthy"`
	TotalNodes    int       `json:"total_nodes"`
	RequiredNodes int       `json:"required_nodes"`
	ActiveNodes   int       `json:"active_nodes"`
	LastCheck     time.Time `json:"last_check"`
	CanProceed    bool      `json:"can_proceed"`
}

// NewSplitBrainGuard 创建脑裂防护
func NewSplitBrainGuard(threshold int, logger *zap.Logger) *SplitBrainGuard {
	return &SplitBrainGuard{
		threshold:    threshold,
		partitionMap: make(map[string]*PartitionInfo),
		events:       make([]SplitBrainEvent, 0),
		quorumSize:   threshold,
		logger:       logger,
	}
}

// HandleSplitBrain 处理脑裂
func (sbg *SplitBrainGuard) HandleSplitBrain(activeNodes []*Node) {
	sbg.mu.Lock()
	defer sbg.mu.Unlock()
	
	// 收集领导者
	var leaders []string
	for _, node := range activeNodes {
		if node.Role == NodeRoleLeader {
			leaders = append(leaders, node.ID)
		}
	}
	
	if len(leaders) <= 1 {
		return // 没有脑裂
	}
	
	// 记录脑裂事件
	event := SplitBrainEvent{
		Timestamp:   time.Now(),
		Type:        "detected",
		Description: "Multiple leaders detected",
		Leaders:     leaders,
	}
	sbg.events = append(sbg.events, event)
	
	sbg.logger.Error("Split brain detected",
		zap.Strings("leaders", leaders),
		zap.Int("active_nodes", len(activeNodes)),
	)
	
	// 执行脑裂解决策略
	sbg.resolveSplitBrain(activeNodes, leaders)
}

// resolveSplitBrain 解决脑裂
func (sbg *SplitBrainGuard) resolveSplitBrain(activeNodes []*Node, leaders []string) {
	// 策略1: 选择优先级最高的领导者
	var bestLeader *Node
	for _, node := range activeNodes {
		if node.Role == NodeRoleLeader {
			if bestLeader == nil || node.Priority > bestLeader.Priority {
				bestLeader = node
			}
		}
	}
	
	if bestLeader == nil {
		sbg.logger.Error("No valid leader found during split brain resolution")
		return
	}
	
	// 降级其他领导者
	for _, node := range activeNodes {
		if node.Role == NodeRoleLeader && node.ID != bestLeader.ID {
			node.Role = NodeRoleFollower
			sbg.logger.Warn("Demoting leader due to split brain",
				zap.String("node_id", node.ID),
				zap.String("new_role", "follower"),
			)
		}
	}
	
	// 记录解决事件
	event := SplitBrainEvent{
		Timestamp:   time.Now(),
		Type:        "resolved",
		Description: "Split brain resolved by priority-based leader selection",
		Leaders:     []string{bestLeader.ID},
		Resolution:  "Selected highest priority leader",
	}
	sbg.events = append(sbg.events, event)
	
	sbg.logger.Info("Split brain resolved",
		zap.String("elected_leader", bestLeader.ID),
	)
}

// HandlePartition 处理网络分区
func (sbg *SplitBrainGuard) HandlePartition(activeNodes []*Node) {
	sbg.mu.Lock()
	defer sbg.mu.Unlock()
	
	// 检查仲裁
	quorum := sbg.checkQuorumInternal(len(activeNodes))
	
	if !quorum.CanProceed {
		event := SplitBrainEvent{
			Timestamp:    time.Now(),
			Type:         "partition",
			Description:  "Network partition detected, quorum lost",
			Partitions:   getNodeIDs(activeNodes),
		}
		sbg.events = append(sbg.events, event)
		
		sbg.logger.Warn("Network partition detected, entering read-only mode",
			zap.Int("active_nodes", len(activeNodes)),
			zap.Int("required_quorum", sbg.quorumSize),
		)
	}
}

// getNodeIDs 获取节点ID列表
func getNodeIDs(nodes []*Node) []string {
	ids := make([]string, len(nodes))
	for i, node := range nodes {
		ids[i] = node.ID
	}
	return ids
}

// checkQuorumInternal 内部仲裁检查（需要已持有锁）
func (sbg *SplitBrainGuard) checkQuorumInternal(activeCount int) QuorumStatus {
	sbg.lastQuorumCheck = time.Now()
	required := sbg.quorumSize
	sbg.quorumHealthy = activeCount >= required
	
	return QuorumStatus{
		Healthy:       sbg.quorumHealthy,
		TotalNodes:    -1, // 需要从外部获取
		RequiredNodes: required,
		ActiveNodes:   activeCount,
		LastCheck:     sbg.lastQuorumCheck,
		CanProceed:    sbg.quorumHealthy,
	}
}

// CheckQuorum 检查仲裁状态
func (sbg *SplitBrainGuard) CheckQuorum(totalNodes, activeNodes int) QuorumStatus {
	sbg.mu.Lock()
	defer sbg.mu.Unlock()
	
	sbg.quorumSize = totalNodes/2 + 1
	sbg.lastQuorumCheck = time.Now()
	sbg.quorumHealthy = activeNodes >= sbg.quorumSize
	
	return QuorumStatus{
		Healthy:       sbg.quorumHealthy,
		TotalNodes:    totalNodes,
		RequiredNodes: sbg.quorumSize,
		ActiveNodes:   activeNodes,
		LastCheck:     sbg.lastQuorumCheck,
		CanProceed:    sbg.quorumHealthy,
	}
}

// CanCommit 检查是否可以提交操作（仲裁检查）
func (sbg *SplitBrainGuard) CanCommit(activeNodes int) bool {
	sbg.mu.RLock()
	defer sbg.mu.RUnlock()
	
	return activeNodes >= sbg.quorumSize
}

// GetEvents 获取脑裂事件历史
func (sbg *SplitBrainGuard) GetEvents(limit int) []SplitBrainEvent {
	sbg.mu.RLock()
	defer sbg.mu.RUnlock()
	
	if limit <= 0 || limit > len(sbg.events) {
		limit = len(sbg.events)
	}
	
	// 返回最近的事件
	start := len(sbg.events) - limit
	if start < 0 {
		start = 0
	}
	
	result := make([]SplitBrainEvent, limit)
	copy(result, sbg.events[start:])
	return result
}

// GetStatus 获取脑裂防护状态
func (sbg *SplitBrainGuard) GetStatus() map[string]interface{} {
	sbg.mu.RLock()
	defer sbg.mu.RUnlock()
	
	return map[string]interface{}{
		"threshold":        sbg.threshold,
		"quorum_size":      sbg.quorumSize,
		"quorum_healthy":   sbg.quorumHealthy,
		"last_quorum_check": sbg.lastQuorumCheck,
		"partition_count":  len(sbg.partitionMap),
		"event_count":      len(sbg.events),
	}
}

// ClearEvents 清除事件历史
func (sbg *SplitBrainGuard) ClearEvents() {
	sbg.mu.Lock()
	defer sbg.mu.Unlock()
	
	sbg.events = make([]SplitBrainEvent, 0)
}

// RecordPartition 记录分区
func (sbg *SplitBrainGuard) RecordPartition(nodeID string, partitionID string, isLeader bool, activeNodes []string) {
	sbg.mu.Lock()
	defer sbg.mu.Unlock()
	
	info := &PartitionInfo{
		NodeID:       nodeID,
		DetectedAt:   time.Now(),
		PartitionID:  partitionID,
		IsLeader:     isLeader,
		ActiveNodes:  activeNodes,
	}
	
	sbg.partitionMap[nodeID] = info
	
	sbg.logger.Info("Partition recorded",
		zap.String("node_id", nodeID),
		zap.String("partition_id", partitionID),
		zap.Bool("is_leader", isLeader),
	)
}

// ResolvePartition 解决分区
func (sbg *SplitBrainGuard) ResolvePartition(nodeID string) {
	sbg.mu.Lock()
	defer sbg.mu.Unlock()
	
	if info, exists := sbg.partitionMap[nodeID]; exists {
		info.ResolvedAt = time.Now()
		delete(sbg.partitionMap, nodeID)
		
		sbg.logger.Info("Partition resolved",
			zap.String("node_id", nodeID),
			zap.Duration("duration", time.Since(info.DetectedAt)),
		)
	}
}

// GetActivePartitions 获取活跃分区
func (sbg *SplitBrainGuard) GetActivePartitions() []*PartitionInfo {
	sbg.mu.RLock()
	defer sbg.mu.RUnlock()
	
	partitions := make([]*PartitionInfo, 0, len(sbg.partitionMap))
	for _, info := range sbg.partitionMap {
		if info.ResolvedAt.IsZero() {
			partitions = append(partitions, info)
		}
	}
	return partitions
}

// IsQuorumHealthy 检查仲裁是否健康
func (sbg *SplitBrainGuard) IsQuorumHealthy() bool {
	sbg.mu.RLock()
	defer sbg.mu.RUnlock()
	
	return sbg.quorumHealthy
}

// SetQuorumSize 设置仲裁大小
func (sbg *SplitBrainGuard) SetQuorumSize(size int) {
	sbg.mu.Lock()
	defer sbg.mu.Unlock()
	
	sbg.quorumSize = size
	sbg.threshold = size
}

// ValidateOperation 验证操作是否可以执行
// 用于在写入操作前检查仲裁状态
func (sbg *SplitBrainGuard) ValidateOperation(opType string, activeNodes int) error {
	sbg.mu.RLock()
	defer sbg.mu.RUnlock()
	
	// 读操作通常不需要仲裁检查
	if opType == "read" {
		return nil
	}
	
	// 写操作需要仲裁
	if !sbg.CanCommit(activeNodes) {
		sbg.logger.Warn("Operation rejected due to insufficient quorum",
			zap.String("op_type", opType),
			zap.Int("active_nodes", activeNodes),
			zap.Int("required", sbg.quorumSize),
		)
		return ErrClusterNotReady
	}
	
	return nil
}