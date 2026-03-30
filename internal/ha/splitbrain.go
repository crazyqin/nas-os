// Package ha 脑裂防护模块
// 实现基于法定人数的脑裂检测和恢复
// 参考 Synology High Availability 的脑裂防护机制
package ha

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SplitBrainGuard 脑裂防护器
type SplitBrainGuard struct {
	config      *HAConfig
	quorum      *QuorumManager
	detected    bool
	lastCheck   time.Time
	resolveLock sync.Mutex
	logger      *zap.Logger
}

// QuorumManager 法定人数管理器
type QuorumManager struct {
	quorumSize   int
	voteCount    map[string]int
	voteHistory  []VoteRecord
	votingActive bool
	mu           sync.RWMutex
}

// VoteRecord 投票记录
type VoteRecord struct {
	Timestamp time.Time `json:"timestamp"`
	VoterID   string    `json:"voter_id"`
	TargetID  string    `json:"target_id"`
	VoteType  VoteType  `json:"vote_type"`
	Result    bool      `json:"result"`
}

// VoteType 投票类型
type VoteType string

const (
	VoteTypePrimary    VoteType = "primary"    // 主节点投票
	VoteTypeSplitBrain VoteType = "splitbrain" // 脑裂投票
	VoteTypeFence      VoteType = "fence"      // 防护投票
	VoteTypeResume     VoteType = "resume"     // 恢复投票
)

// SplitBrainResolution 脑裂解决策略
type SplitBrainResolution string

const (
	ResolutionHighestPriority SplitBrainResolution = "highest_priority" // 最高优先级
	ResolutionLatestPrimary   SplitBrainResolution = "latest_primary"   // 最新主节点
	ResolutionManual          SplitBrainResolution = "manual"           // 手动解决
	ResolutionShutdownBoth    SplitBrainResolution = "shutdown_both"    // 关闭双方
)

// NewSplitBrainGuard 创建脑裂防护器
func NewSplitBrainGuard(config *HAConfig, logger *zap.Logger) *SplitBrainGuard {
	return &SplitBrainGuard{
		config: config,
		quorum: NewQuorumManager(config.QuorumRequired),
		logger: logger,
	}
}

// NewQuorumManager 创建法定人数管理器
func NewQuorumManager(quorumSize int) *QuorumManager {
	return &QuorumManager{
		quorumSize:  quorumSize,
		voteCount:   make(map[string]int),
		voteHistory: make([]VoteRecord, 0, 100),
	}
}

// CheckSplitBrain 检查脑裂
func (sb *SplitBrainGuard) CheckSplitBrain(nodes map[string]*NodeHAInfo) *SplitBrainResult {
	sb.resolveLock.Lock()
	defer sb.resolveLock.Unlock()

	result := &SplitBrainResult{
		Timestamp: time.Now(),
	}

	// 统计活跃节点和主节点
	activeNodes := make([]*NodeHAInfo, 0)
	primaryNodes := make([]*NodeHAInfo, 0)

	for _, node := range nodes {
		if node.State == HAStateActive || node.State == HAStatePassive {
			activeNodes = append(activeNodes, node)
		}
		if node.Role == HARolePrimary {
			primaryNodes = append(primaryNodes, node)
		}
	}

	// 检查法定人数
	result.ActiveNodeCount = len(activeNodes)
	result.QuorumRequired = sb.config.QuorumRequired
	result.QuorumMet = len(activeNodes) >= sb.config.QuorumRequired

	// 检查是否有多个主节点（脑裂）
	result.PrimaryNodeCount = len(primaryNodes)
	result.SplitBrainDetected = len(primaryNodes) > 1

	if result.SplitBrainDetected {
		result.PrimaryNodes = primaryNodes
		sb.detected = true
		sb.lastCheck = time.Now()

		sb.logger.Error("Split brain detected",
			zap.Int("primary_count", len(primaryNodes)),
			zap.Strings("primary_ids", extractNodeIDs(primaryNodes)),
		)
	} else if !result.QuorumMet {
		sb.logger.Warn("Quorum not met",
			zap.Int("active_nodes", len(activeNodes)),
			zap.Int("quorum_required", sb.config.QuorumRequired),
		)
	}

	// 检查节点是否隔离
	if len(activeNodes) == 1 {
		for _, node := range activeNodes {
			if node.ID == sb.config.NodeID {
				result.Isolated = true
				result.IsolatedNodeID = node.ID
			}
		}
	}

	return result
}

// HandleSplitBrain 处理脑裂
func (sb *SplitBrainGuard) HandleSplitBrain(nodes map[string]*NodeHAInfo) error {
	sb.resolveLock.Lock()
	defer sb.resolveLock.Unlock()

	if !sb.detected {
		return nil
	}

	sb.logger.Warn("Handling split brain situation")

	// 收集所有声称是主节点的节点
	claimingPrimaries := make([]*NodeHAInfo, 0)
	for _, node := range nodes {
		if node.Role == HARolePrimary {
			claimingPrimaries = append(claimingPrimaries, node)
		}
	}

	if len(claimingPrimaries) == 0 {
		sb.detected = false
		return nil
	}

	// 根据策略解决脑裂
	resolution := sb.selectResolutionStrategy()

	switch resolution {
	case ResolutionHighestPriority:
		// 选择优先级最高的作为主节点
		return sb.resolveByPriority(claimingPrimaries)

	case ResolutionLatestPrimary:
		// 选择最后成为主节点的
		return sb.resolveByLatest(claimingPrimaries)

	case ResolutionShutdownBoth:
		// 关闭双方，等待手动介入
		return sb.resolveByShutdown(claimingPrimaries)

	default:
		return sb.resolveByPriority(claimingPrimaries)
	}
}

// selectResolutionStrategy 选择解决策略
func (sb *SplitBrainGuard) selectResolutionStrategy() SplitBrainResolution {
	// 默认使用优先级策略
	// 在实际实现中可以根据配置或自动检测选择

	return ResolutionHighestPriority
}

// resolveByPriority 通过优先级解决
func (sb *SplitBrainGuard) resolveByPriority(nodes []*NodeHAInfo) error {
	if len(nodes) == 0 {
		return nil
	}

	// 找到优先级最高的节点
	var winner *NodeHAInfo
	for _, node := range nodes {
		if winner == nil || node.Priority > winner.Priority {
			winner = node
		}
	}

	sb.logger.Info("Split brain resolved by priority",
		zap.String("winner", winner.ID),
		zap.Int("priority", winner.Priority),
	)

	// 其他节点降级为备节点
	for _, node := range nodes {
		if node.ID != winner.ID {
			node.Role = HARoleSecondary
			node.State = HAStatePassive
			sb.logger.Warn("Node downgraded due to split brain resolution",
				zap.String("node_id", node.ID),
			)
		}
	}

	sb.detected = false
	return nil
}

// resolveByLatest 通过最新主节点解决
func (sb *SplitBrainGuard) resolveByLatest(nodes []*NodeHAInfo) error {
	// 选择最后心跳时间最新的
	var latest *NodeHAInfo
	for _, node := range nodes {
		if latest == nil || node.LastHeartbeat.After(latest.LastHeartbeat) {
			latest = node
		}
	}

	sb.logger.Info("Split brain resolved by latest",
		zap.String("winner", latest.ID),
	)

	// 其他节点降级
	for _, node := range nodes {
		if node.ID != latest.ID {
			node.Role = HARoleSecondary
			node.State = HAStatePassive
		}
	}

	sb.detected = false
	return nil
}

// resolveByShutdown 通过关闭解决
func (sb *SplitBrainGuard) resolveByShutdown(nodes []*NodeHAInfo) error {
	sb.logger.Error("Split brain unresolved - requiring manual intervention")

	// 所有节点进入安全模式
	for _, node := range nodes {
		node.State = HAStateStandby
	}

	// 不自动解决，等待手动介入
	return errors.New("split brain requires manual resolution")
}

// IsQuorumMet 检查法定人数是否满足
func (sb *SplitBrainGuard) IsQuorumMet(nodes map[string]*NodeHAInfo) bool {
	activeCount := 0
	for _, node := range nodes {
		if node.State == HAStateActive || node.State == HAStatePassive {
			activeCount++
		}
	}

	return activeCount >= sb.config.QuorumRequired
}

// RequestVote 请求投票
func (qm *QuorumManager) RequestVote(voterID, targetID string, voteType VoteType) bool {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	// 记录投票
	vote := VoteRecord{
		Timestamp: time.Now(),
		VoterID:   voterID,
		TargetID:  targetID,
		VoteType:  voteType,
	}

	// 统计投票
	key := fmt.Sprintf("%s:%s", voteType, targetID)
	qm.voteCount[key]++

	vote.Result = qm.voteCount[key] >= qm.quorumSize
	qm.voteHistory = append(qm.voteHistory, vote)

	// 限制历史记录
	if len(qm.voteHistory) > 100 {
		qm.voteHistory = qm.voteHistory[len(qm.voteHistory)-100:]
	}

	return vote.Result
}

// ClearVotes 清除投票
func (qm *QuorumManager) ClearVotes(voteType VoteType) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	for key := range qm.voteCount {
		if len(voteType) > 0 && key[:len(voteType)] == string(voteType) {
			delete(qm.voteCount, key)
		}
	}
}

// GetVoteStatus 获取投票状态
func (qm *QuorumManager) GetVoteStatus(targetID string, voteType VoteType) VoteStatus {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", voteType, targetID)
	count := qm.voteCount[key]

	return VoteStatus{
		VotesReceived: count,
		VotesRequired: qm.quorumSize,
		VoteMet:       count >= qm.quorumSize,
	}
}

// VoteStatus 投票状态
type VoteStatus struct {
	VotesReceived int  `json:"votes_received"`
	VotesRequired int  `json:"votes_required"`
	VoteMet       bool `json:"vote_met"`
}

// SplitBrainResult 脑裂检测结果
type SplitBrainResult struct {
	Timestamp          time.Time     `json:"timestamp"`
	SplitBrainDetected bool          `json:"split_brain_detected"`
	PrimaryNodeCount   int           `json:"primary_node_count"`
	PrimaryNodes       []*NodeHAInfo `json:"primary_nodes,omitempty"`
	QuorumMet          bool          `json:"quorum_met"`
	QuorumRequired     int           `json:"quorum_required"`
	ActiveNodeCount    int           `json:"active_node_count"`
	Isolated           bool          `json:"isolated"`
	IsolatedNodeID     string        `json:"isolated_node_id,omitempty"`
}

// IsSplitBrainDetected 是否检测到脑裂
func (sb *SplitBrainGuard) IsSplitBrainDetected() bool {
	sb.resolveLock.Lock()
	defer sb.resolveLock.Unlock()
	return sb.detected
}

// GetLastCheckTime 获取最后检查时间
func (sb *SplitBrainGuard) GetLastCheckTime() time.Time {
	sb.resolveLock.Lock()
	defer sb.resolveLock.Unlock()
	return sb.lastCheck
}

// FenceNode 防护节点（STONITH机制）
// 参考 Synology 的节点防护机制
func (sb *SplitBrainGuard) FenceNode(nodeID string) error {
	sb.logger.Warn("Fencing node",
		zap.String("node_id", nodeID),
	)

	// 在实际实现中，这里需要：
	// 1. 通过 IPMI/iLO 关闭节点电源
	// 2. 或通过 SSH 执行关机命令
	// 3. 或标记节点为不可用

	// 这里使用模拟实现
	sb.quorum.RequestVote(sb.config.NodeID, nodeID, VoteTypeFence)

	return nil
}

// NodeFencing 节点防护操作
type NodeFencing struct {
	NodeID         string        `json:"node_id"`
	Method         FencingMethod `json:"method"`
	Timestamp      time.Time     `json:"timestamp"`
	Success        bool          `json:"success"`
	ErrorMessage   string        `json:"error_message,omitempty"`
	RecoveryAction string        `json:"recovery_action,omitempty"`
}

// FencingMethod 防护方法
type FencingMethod string

const (
	FencingIPMI    FencingMethod = "ipmi"    // IPMI 电源控制
	FencingSSH     FencingMethod = "ssh"     // SSH 关机
	FencingStorage FencingMethod = "storage" // 存储隔离
	FencingManual  FencingMethod = "manual"  // 手动干预
)

// extractNodeIDs 提取节点ID列表
func extractNodeIDs(nodes []*NodeHAInfo) []string {
	ids := make([]string, len(nodes))
	for i, node := range nodes {
		ids[i] = node.ID
	}
	return ids
}
