package cluster

import (
	"context"
	"sync"
	"time"
)

// ElectionState 选举状态
type ElectionState struct {
	state          ElectionPhase
	term           uint64
	votedFor       string
	electionTimer  *time.Timer
	electionChan   chan struct{}
	timeout        time.Duration
	lastElection   time.Time
	leaderChangeCh chan string
	mu             sync.RWMutex
}

// ElectionPhase 选举阶段
type ElectionPhase string

const (
	ElectionPhaseFollower  ElectionPhase = "follower"
	ElectionPhaseCandidate ElectionPhase = "candidate"
	ElectionPhaseLeader    ElectionPhase = "leader"
)

// NewElectionState 创建选举状态
func NewElectionState(timeout time.Duration) *ElectionState {
	return &ElectionState{
		state:          ElectionPhaseFollower,
		timeout:        timeout,
		electionChan:   make(chan struct{}, 1),
		leaderChangeCh: make(chan string, 10),
	}
}

// StartElection 触发选举
func (es *ElectionState) StartElection() {
	es.mu.Lock()
	defer es.mu.Unlock()
	
	select {
	case es.electionChan <- struct{}{}:
	default:
		// 已经有待处理的选举请求
	}
}

// ElectionChan 获取选举通道
func (es *ElectionState) ElectionChan() <-chan struct{} {
	return es.electionChan
}

// LeaderChangeChan 获取领导者变更通道
func (es *ElectionState) LeaderChangeChan() <-chan string {
	return es.leaderChangeCh
}

// GetCurrentTerm 获取当前任期
func (es *ElectionState) GetCurrentTerm() uint64 {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.term
}

// IncrementTerm 增加任期
func (es *ElectionState) IncrementTerm() uint64 {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.term++
	return es.term
}

// SetTerm 设置任期
func (es *ElectionState) SetTerm(term uint64) {
	es.mu.Lock()
	defer es.mu.Unlock()
	if term > es.term {
		es.term = term
		es.votedFor = ""
	}
}

// GetVotedFor 获取投票目标
func (es *ElectionState) GetVotedFor() string {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.votedFor
}

// SetVotedFor 设置投票目标
func (es *ElectionState) SetVotedFor(candidateID string) bool {
	es.mu.Lock()
	defer es.mu.Unlock()
	
	if es.votedFor == "" || es.votedFor == candidateID {
		es.votedFor = candidateID
		return true
	}
	return false
}

// ClearVote 清除投票
func (es *ElectionState) ClearVote() {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.votedFor = ""
}

// GetPhase 获取当前阶段
func (es *ElectionState) GetPhase() ElectionPhase {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.state
}

// SetPhase 设置阶段
func (es *ElectionState) SetPhase(phase ElectionPhase) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.state = phase
	
	if phase == ElectionPhaseCandidate {
		es.lastElection = time.Now()
	}
}

// BecomeLeader 成为领导者
func (es *ElectionState) BecomeLeader() {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.state = ElectionPhaseLeader
	es.lastElection = time.Now()
}

// BecomeFollower 成为跟随者
func (es *ElectionState) BecomeFollower(newLeaderID string) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.state = ElectionPhaseFollower
	
	select {
	case es.leaderChangeCh <- newLeaderID:
	default:
	}
}

// BecomeCandidate 成为候选人
func (es *ElectionState) BecomeCandidate() {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.state = ElectionPhaseCandidate
	es.term++
	es.lastElection = time.Now()
}

// GetLastElection 获取上次选举时间
func (es *ElectionState) GetLastElection() time.Time {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.lastElection
}

// ElectionStats 选举统计
type ElectionStats struct {
	CurrentTerm   uint64        `json:"current_term"`
	CurrentPhase  ElectionPhase `json:"current_phase"`
	VotedFor      string        `json:"voted_for,omitempty"`
	LastElection  time.Time     `json:"last_election"`
	TimeSinceLast time.Duration `json:"time_since_last"`
	Timeout       time.Duration `json:"timeout"`
}

// GetStats 获取选举统计
func (es *ElectionState) GetStats() ElectionStats {
	es.mu.RLock()
	defer es.mu.RUnlock()
	
	var since time.Duration
	if !es.lastElection.IsZero() {
		since = time.Since(es.lastElection)
	}
	
	return ElectionStats{
		CurrentTerm:   es.term,
		CurrentPhase:  es.state,
		VotedFor:      es.votedFor,
		LastElection:  es.lastElection,
		TimeSinceLast: since,
		Timeout:       es.timeout,
	}
}

// VoteRequest 投票请求
type VoteRequest struct {
	Term        uint64 `json:"term"`
	CandidateID string `json:"candidate_id"`
	LastLogIndex uint64 `json:"last_log_index"`
	LastLogTerm  uint64 `json:"last_log_term"`
}

// VoteResponse 投票响应
type VoteResponse struct {
	Term        uint64 `json:"term"`
	VoteGranted bool   `json:"vote_granted"`
	Reason      string `json:"reason,omitempty"`
}

// RequestVote 处理投票请求
func (es *ElectionState) RequestVote(req VoteRequest) VoteResponse {
	es.mu.Lock()
	defer es.mu.Unlock()
	
	resp := VoteResponse{Term: es.term}
	
	// 任期过旧
	if req.Term < es.term {
		resp.VoteGranted = false
		resp.Reason = "stale term"
		return resp
	}
	
	// 更新任期
	if req.Term > es.term {
		es.term = req.Term
		es.votedFor = ""
		es.state = ElectionPhaseFollower
	}
	
	// 检查是否已投票
	if es.votedFor == "" || es.votedFor == req.CandidateID {
		es.votedFor = req.CandidateID
		resp.VoteGranted = true
	} else {
		resp.VoteGranted = false
		resp.Reason = "already voted for another candidate"
	}
	
	return resp
}

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct {
	Term     uint64 `json:"term"`
	LeaderID string `json:"leader_id"`
}

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	Term    uint64 `json:"term"`
	Success bool   `json:"success"`
}

// ProcessHeartbeat 处理心跳
func (es *ElectionState) ProcessHeartbeat(req HeartbeatRequest) HeartbeatResponse {
	es.mu.Lock()
	defer es.mu.Unlock()
	
	resp := HeartbeatResponse{Term: es.term}
	
	// 任期过旧
	if req.Term < es.term {
		resp.Success = false
		return resp
	}
	
	// 更新任期
	if req.Term > es.term {
		es.term = req.Term
	}
	
	// 成为跟随者
	es.state = ElectionPhaseFollower
	es.votedFor = req.LeaderID
	resp.Success = true
	
	return resp
}

// FollowerTracker 跟随者追踪器（用于领导者）
type FollowerTracker struct {
	followers map[string]*FollowerInfo
	mu        sync.RWMutex
}

// FollowerInfo 跟随者信息
type FollowerInfo struct {
	ID            string        `json:"id"`
	LastContact   time.Time     `json:"last_contact"`
	NextIndex     uint64        `json:"next_index"`
	MatchIndex    uint64        `json:"match_index"`
	ReplicationLag time.Duration `json:"replication_lag"`
}

// NewFollowerTracker 创建跟随者追踪器
func NewFollowerTracker() *FollowerTracker {
	return &FollowerTracker{
		followers: make(map[string]*FollowerInfo),
	}
}

// AddFollower 添加跟随者
func (ft *FollowerTracker) AddFollower(id string) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	
	ft.followers[id] = &FollowerInfo{
		ID:          id,
		LastContact: time.Now(),
		NextIndex:   1,
		MatchIndex:  0,
	}
}

// RemoveFollower 移除跟随者
func (ft *FollowerTracker) RemoveFollower(id string) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	delete(ft.followers, id)
}

// UpdateFollower 更新跟随者状态
func (ft *FollowerTracker) UpdateFollower(id string, matchIndex uint64) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	
	if info, exists := ft.followers[id]; exists {
		info.LastContact = time.Now()
		info.MatchIndex = matchIndex
		info.ReplicationLag = time.Since(info.LastContact)
	}
}

// GetFollower 获取跟随者信息
func (ft *FollowerTracker) GetFollower(id string) *FollowerInfo {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	return ft.followers[id]
}

// GetAllFollowers 获取所有跟随者
func (ft *FollowerTracker) GetAllFollowers() []*FollowerInfo {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	
	followers := make([]*FollowerInfo, 0, len(ft.followers))
	for _, info := range ft.followers {
		followers = append(followers, info)
	}
	return followers
}

// GetMatchIndices 获取所有匹配索引
func (ft *FollowerTracker) GetMatchIndices() []uint64 {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	
	indices := make([]uint64, 0, len(ft.followers))
	for _, info := range ft.followers {
		indices = append(indices, info.MatchIndex)
	}
	return indices
}

// ComputeCommitIndex 计算提交索引（基于多数）
func (ft *FollowerTracker) ComputeCommitIndex(leaderIndex uint64) uint64 {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	
	indices := make([]uint64, 0, len(ft.followers)+1)
	indices = append(indices, leaderIndex)
	for _, info := range ft.followers {
		indices = append(indices, info.MatchIndex)
	}
	
	// 排序并找中位数
	return computeMajorityIndex(indices)
}

// computeMajorityIndex 计算多数同意的索引
func computeMajorityIndex(indices []uint64) uint64 {
	if len(indices) == 0 {
		return 0
	}
	
	// 简单冒泡排序
	for i := 0; i < len(indices)-1; i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[i] > indices[j] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}
	
	// 返回中位数
	return indices[len(indices)/2]
}

// WaitForElection 等待选举完成
func (es *ElectionState) WaitForElection(ctx context.Context, timeout time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(timeout):
		return false
	case leaderID := <-es.leaderChangeCh:
		// 放回通道供其他监听者
		select {
		case es.leaderChangeCh <- leaderID:
		default:
		}
		return true
	}
}