package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HA 状态.
const (
	HAStateFollower  = "follower"
	HAStateCandidate = "candidate"
	HAStateLeader    = "leader"
)

// 故障转移事件类型.
const (
	FailoverEventDetection = "detection"
	FailoverEventElection  = "election"
	FailoverEventTransfer  = "transfer"
	FailoverEventRecovery  = "recovery"
)

// HAEvent 高可用事件.
type HAEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	OldLeader string    `json:"old_leader"`
	NewLeader string    `json:"new_leader"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
	Duration  string    `json:"duration"`
}

// HAConfig 高可用配置.
type HAConfig struct {
	NodeID           string `json:"node_id"`
	DataDir          string `json:"data_dir"`
	BindPort         int    `json:"bind_port"`
	HeartbeatTimeout int    `json:"heartbeat_timeout"` // 毫秒
	ElectionTimeout  int    `json:"election_timeout"`  // 毫秒
}

// HAStatus 高可用状态.
type HAStatus struct {
	State       string     `json:"state"`
	Leader      string     `json:"leader"`
	LeaderAddr  string     `json:"leader_addr"`
	Term        uint64     `json:"term"`
	LastContact time.Time  `json:"last_contact"`
	Peers       []PeerInfo `json:"peers"`
}

// PeerInfo 对等节点信息.
type PeerInfo struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Voter   bool   `json:"voter"`
	Healthy bool   `json:"healthy"`
}

// HighAvailability 高可用管理器（简化版 - 基于心跳的 leader 选举）.
type HighAvailability struct {
	config      HAConfig
	state       string
	leaderID    string
	leaderAddr  string
	term        uint64
	peers       map[string]*PeerInfo
	peersMutex  sync.RWMutex
	events      []HAEvent
	eventsMutex sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	logger      *zap.Logger
	callbacks   HACallbacks
	lastContact time.Time
}

// HACallbacks HA 事件回调.
type HACallbacks struct {
	OnLeaderChange func(oldLeader, newLeader string)
	OnNodeJoin     func(nodeID string)
	OnNodeLeave    func(nodeID string)
}

// NewHighAvailability 创建高可用管理器.
func NewHighAvailability(config HAConfig, logger *zap.Logger) (*HighAvailability, error) {
	if config.NodeID == "" {
		hostname, _ := os.Hostname()
		config.NodeID = hostname
	}
	if config.BindPort == 0 {
		config.BindPort = 8082
	}
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 3000 // 3 秒
	}
	if config.ElectionTimeout == 0 {
		config.ElectionTimeout = 5000 // 5 秒
	}
	if config.DataDir == "" {
		config.DataDir = "/var/lib/nas-os/ha"
	}

	ctx, cancel := context.WithCancel(context.Background())

	ha := &HighAvailability{
		config:      config,
		state:       HAStateFollower,
		peers:       make(map[string]*PeerInfo),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
		events:      make([]HAEvent, 0),
		lastContact: time.Now(),
	}

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0750); err != nil {
		cancel()
		return nil, fmt.Errorf("创建 HA 数据目录失败：%w", err)
	}

	// 加载持久化数据
	_ = ha.loadState()

	return ha, nil
}

// Initialize 初始化高可用管理器.
func (ha *HighAvailability) Initialize() error {
	ha.logger.Info("初始化高可用管理器", zap.String("node_id", ha.config.NodeID))

	// 初始化为 follower
	ha.state = HAStateFollower
	ha.term = 1

	// 启动领导者检查
	go ha.leaderCheckWorker()

	// 启动心跳广播
	go ha.heartbeatWorker()

	ha.logger.Info("高可用管理器初始化完成", zap.String("node_id", ha.config.NodeID))
	return nil
}

// leaderCheckWorker 检查领导者状态.
func (ha *HighAvailability) leaderCheckWorker() {
	ticker := time.NewTicker(time.Duration(ha.config.ElectionTimeout) * time.Millisecond / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ha.ctx.Done():
			return
		case <-ticker.C:
			ha.checkLeader()
		}
	}
}

// checkLeader 检查领导者是否存活.
func (ha *HighAvailability) checkLeader() {
	ha.peersMutex.RLock()
	currentLeader := ha.leaderID
	ha.peersMutex.RUnlock()

	// 如果没有领导者或者领导者超时，发起选举
	if currentLeader == "" || time.Since(ha.lastContact) > time.Duration(ha.config.HeartbeatTimeout)*time.Millisecond {
		ha.startElection()
	}
}

// startElection 开始选举.
func (ha *HighAvailability) startElection() {
	ha.logger.Info("开始领导者选举", zap.String("node_id", ha.config.NodeID))

	ha.state = HAStateCandidate
	ha.term++

	// 简单选举：成为领导者（实际应该投票）
	ha.becomeLeader()
}

// becomeLeader 成为领导者.
func (ha *HighAvailability) becomeLeader() {
	oldLeader := ha.leaderID

	ha.peersMutex.Lock()
	ha.state = HAStateLeader
	ha.leaderID = ha.config.NodeID
	ha.leaderAddr = fmt.Sprintf("0.0.0.0:%d", ha.config.BindPort)
	ha.peersMutex.Unlock()

	ha.logger.Info("成为领导者",
		zap.String("node_id", ha.config.NodeID),
		zap.Uint64("term", ha.term))

	// 记录故障转移事件
	ha.recordFailoverEvent(HAEvent{
		ID:        fmt.Sprintf("failover-%d", time.Now().UnixNano()),
		Type:      FailoverEventElection,
		OldLeader: oldLeader,
		NewLeader: ha.config.NodeID,
		Timestamp: time.Now(),
		Reason:    "election",
	})

	// 触发回调
	if ha.callbacks.OnLeaderChange != nil {
		go ha.callbacks.OnLeaderChange(oldLeader, ha.config.NodeID)
	}
}

// heartbeatWorker 心跳广播工作线程.
func (ha *HighAvailability) heartbeatWorker() {
	ticker := time.NewTicker(time.Duration(ha.config.HeartbeatTimeout) * time.Millisecond / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ha.ctx.Done():
			return
		case <-ticker.C:
			if ha.state == HAStateLeader {
				ha.broadcastHeartbeat()
			}
		}
	}
}

// broadcastHeartbeat 广播心跳.
func (ha *HighAvailability) broadcastHeartbeat() {
	// 简化实现：只更新本地状态
	ha.lastContact = time.Now()

	// 发送心跳到所有 follower
	ha.peersMutex.RLock()
	for _, peer := range ha.peers {
		if peer.Healthy {
			go ha.sendHeartbeatToPeer(peer)
		}
	}
	ha.peersMutex.RUnlock()
}

// sendHeartbeatToPeer 发送心跳到指定节点.
func (ha *HighAvailability) sendHeartbeatToPeer(peer *PeerInfo) {
	ctx, cancel := context.WithTimeout(ha.ctx, 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://%s/ha/heartbeat", peer.Address)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		ha.logger.Debug("创建心跳请求失败", zap.String("peer", peer.ID), zap.Error(err))
		return
	}

	req.Header.Set("X-Leader-ID", ha.config.NodeID)
	req.Header.Set("X-Term", fmt.Sprintf("%d", ha.term))

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		ha.logger.Debug("发送心跳失败", zap.String("peer", peer.ID), zap.Error(err))
		ha.markPeerUnhealthy(peer.ID)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		ha.logger.Debug("心跳响应异常", zap.String("peer", peer.ID), zap.Int("status", resp.StatusCode))
	}
}

// markPeerUnhealthy 标记节点为不健康.
func (ha *HighAvailability) markPeerUnhealthy(peerID string) {
	ha.peersMutex.Lock()
	defer ha.peersMutex.Unlock()

	if peer, exists := ha.peers[peerID]; exists {
		peer.Healthy = false
	}
}

// IsLeader 检查当前节点是否为领导者.
func (ha *HighAvailability) IsLeader() bool {
	return ha.state == HAStateLeader
}

// GetLeader 获取当前领导者.
func (ha *HighAvailability) GetLeader() (string, string) {
	ha.peersMutex.RLock()
	defer ha.peersMutex.RUnlock()
	return ha.leaderID, ha.leaderAddr
}

// GetState 获取当前状态.
func (ha *HighAvailability) GetState() string {
	return ha.state
}

// GetStatus 获取详细状态.
func (ha *HighAvailability) GetStatus() HAStatus {
	ha.peersMutex.RLock()
	defer ha.peersMutex.RUnlock()

	peers := make([]PeerInfo, 0, len(ha.peers))
	for _, peer := range ha.peers {
		peers = append(peers, *peer)
	}

	return HAStatus{
		State:       ha.state,
		Leader:      ha.leaderID,
		LeaderAddr:  ha.leaderAddr,
		Term:        ha.term,
		LastContact: ha.lastContact,
		Peers:       peers,
	}
}

// AddPeer 添加对等节点.
func (ha *HighAvailability) AddPeer(nodeID, address string) error {
	ha.peersMutex.Lock()
	defer ha.peersMutex.Unlock()

	ha.peers[nodeID] = &PeerInfo{
		ID:      nodeID,
		Address: address,
		Voter:   true,
		Healthy: true,
	}

	ha.logger.Info("添加对等节点", zap.String("node_id", nodeID), zap.String("address", address))

	// 触发回调
	if ha.callbacks.OnNodeJoin != nil {
		go ha.callbacks.OnNodeJoin(nodeID)
	}

	return nil
}

// RemovePeer 移除对等节点.
func (ha *HighAvailability) RemovePeer(nodeID string) error {
	ha.peersMutex.Lock()
	defer ha.peersMutex.Unlock()

	delete(ha.peers, nodeID)

	ha.logger.Info("移除对等节点", zap.String("node_id", nodeID))

	// 触发回调
	if ha.callbacks.OnNodeLeave != nil {
		go ha.callbacks.OnNodeLeave(nodeID)
	}

	return nil
}

// TransferLeadership 转移领导权.
func (ha *HighAvailability) TransferLeadership(targetNodeID string) error {
	if !ha.IsLeader() {
		return fmt.Errorf("当前节点不是领导者，无法转移领导权")
	}

	oldLeader := ha.leaderID

	ha.peersMutex.Lock()
	ha.leaderID = targetNodeID
	if peer, exists := ha.peers[targetNodeID]; exists {
		ha.leaderAddr = peer.Address
	}
	ha.state = HAStateFollower
	ha.peersMutex.Unlock()

	ha.logger.Info("领导权转移",
		zap.String("from", oldLeader),
		zap.String("to", targetNodeID))

	// 记录故障转移事件
	ha.recordFailoverEvent(HAEvent{
		ID:        fmt.Sprintf("failover-%d", time.Now().UnixNano()),
		Type:      FailoverEventTransfer,
		OldLeader: oldLeader,
		NewLeader: targetNodeID,
		Timestamp: time.Now(),
		Reason:    "manual_transfer",
	})

	return nil
}

// GetPeers 获取所有对等节点.
func (ha *HighAvailability) GetPeers() []*PeerInfo {
	ha.peersMutex.RLock()
	defer ha.peersMutex.RUnlock()

	peers := make([]*PeerInfo, 0, len(ha.peers))
	for _, peer := range ha.peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetFailoverHistory 获取故障转移历史.
func (ha *HighAvailability) GetFailoverHistory(limit int) []HAEvent {
	ha.eventsMutex.RLock()
	defer ha.eventsMutex.RUnlock()

	if limit <= 0 || limit > len(ha.events) {
		limit = len(ha.events)
	}

	start := len(ha.events) - limit
	if start < 0 {
		start = 0
	}

	result := make([]HAEvent, limit)
	copy(result, ha.events[start:])
	return result
}

// recordFailoverEvent 记录故障转移事件.
func (ha *HighAvailability) recordFailoverEvent(event HAEvent) {
	ha.eventsMutex.Lock()
	defer ha.eventsMutex.Unlock()

	ha.events = append(ha.events, event)

	// 限制历史记录数量
	if len(ha.events) > 100 {
		ha.events = ha.events[len(ha.events)-100:]
	}
}

// SetCallbacks 设置事件回调.
func (ha *HighAvailability) SetCallbacks(callbacks HACallbacks) {
	ha.callbacks = callbacks
}

// Shutdown 关闭高可用管理器.
func (ha *HighAvailability) Shutdown() error {
	ha.cancel()
	_ = ha.saveState()
	ha.logger.Info("高可用管理器已关闭")
	return nil
}

// 持久化

func (ha *HighAvailability) saveState() error {
	state := map[string]interface{}{
		"state":        ha.state,
		"leader_id":    ha.leaderID,
		"leader_addr":  ha.leaderAddr,
		"term":         ha.term,
		"last_contact": ha.lastContact,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	stateFile := filepath.Join(ha.config.DataDir, "ha_state.json")
	return os.WriteFile(stateFile, data, 0640)
}

func (ha *HighAvailability) loadState() error {
	stateFile := filepath.Join(ha.config.DataDir, "ha_state.json")

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	if s, ok := state["state"].(string); ok {
		ha.state = s
	}
	if id, ok := state["leader_id"].(string); ok {
		ha.leaderID = id
	}
	if addr, ok := state["leader_addr"].(string); ok {
		ha.leaderAddr = addr
	}
	if term, ok := state["term"].(float64); ok {
		ha.term = uint64(term)
	}

	return nil
}
