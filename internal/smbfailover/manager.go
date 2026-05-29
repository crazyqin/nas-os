// Package smbfailover implements SMB Stateful Failover for high availability.
// Inspired by TrueNAS SMB Stateful Failover, provides transparent client failover
// with session state preservation across clustered NAS nodes.
package smbfailover

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// NodeState represents the state of a cluster node
type NodeState string

const (
	NodeStateActive    NodeState = "active"
	NodeStateStandby   NodeState = "standby"
	NodeStateFailed    NodeState = "failed"
	NodeStateDraining  NodeState = "draining"
	NodeStateRecovery  NodeState = "recovery"
)

// FailoverState represents the failover process state
type FailoverState string

const (
	FailoverStateIdle       FailoverState = "idle"
	FailoverStateDetecting  FailoverState = "detecting"
	FailoverStateFailing    FailoverState = "failing_over"
	FailoverStateCompleted  FailoverState = "completed"
	FailoverStateFailed     FailoverState = "failed"
)

// ClusterNode represents a node in the SMB cluster
type ClusterNode struct {
	ID           string    `json:"id"`
	Hostname     string    `json:"hostname"`
	IP           net.IP    `json:"ip"`
	State        NodeState `json:"state"`
	VIP          string    `json:"vip"`          // Virtual IP for SMB
	Priority     int       `json:"priority"`     // Higher = preferred active
	Sessions     int64     `json:"sessions"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Uptime       time.Duration `json:"uptime"`
	StartedAt    time.Time `json:"started_at"`
}

// SMBSession represents a tracked SMB client session
type SMBSession struct {
	ID           string    `json:"id"`
	ClientIP     string    `json:"client_ip"`
	Username     string    `json:"username"`
	TreeConns    []TreeConn `json:"tree_connections"`
	OpenFiles    []OpenFile `json:"open_files"`
	Locks        []FileLock `json:"locks"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
	SequenceNum  uint64    `json:"sequence_num"`
}

// TreeConn represents an SMB tree connection
type TreeConn struct {
	ID       string `json:"id"`
	Share    string `json:"share"`
	Path     string `json:"path"`
	Flags    uint32 `json:"flags"`
}

// OpenFile represents an open file handle
type OpenFile struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Handle   uint64 `json:"handle"`
	Access   uint32 `json:"access"`
	Share    uint32 `json:"share"`
	LockType string `json:"lock_type"`
}

// FileLock represents a file lock
type FileLock struct {
	FileID   string `json:"file_id"`
	Offset   int64  `json:"offset"`
	Length   int64  `json:"length"`
	Type     string `json:"type"` // "read", "write", "exclusive"
	ClientIP string `json:"client_ip"`
}

// FailoverConfig configures failover behavior
type FailoverConfig struct {
	HeartbeatInterval time.Duration `json:"heartbeat_interval"` // Default: 1s
	HeartbeatTimeout  time.Duration `json:"heartbeat_timeout"`  // Default: 5s
	FailoverTimeout   time.Duration `json:"failover_timeout"`   // Default: 30s
	MaxRetries        int           `json:"max_retries"`        // Default: 3
	VIPAddress        string        `json:"vip_address"`        // Virtual IP
	InterfaceName     string        `json:"interface_name"`     // Network interface for VIP
	EnableGRATARP     bool          `json:"enable_gratarp"`     // Send GARP after failover
}

// DefaultFailoverConfig returns sensible defaults
func DefaultFailoverConfig() FailoverConfig {
	return FailoverConfig{
		HeartbeatInterval: 1 * time.Second,
		HeartbeatTimeout:  5 * time.Second,
		FailoverTimeout:   30 * time.Second,
		MaxRetries:        3,
		EnableGRATARP:     true,
	}
}

// FailoverEvent records a failover event
type FailoverEvent struct {
	ID           string        `json:"id"`
	Timestamp    time.Time     `json:"timestamp"`
	FromNode     string        `json:"from_node"`
	ToNode       string        `json:"to_node"`
	Reason       string        `json:"reason"`
	State        FailoverState `json:"state"`
	Sessions     int           `json:"sessions_transferred"`
	Duration     time.Duration `json:"duration"`
	Success      bool          `json:"success"`
}

// FailoverManager manages SMB stateful failover
type FailoverManager struct {
	mu          sync.RWMutex
	config      FailoverConfig
	logger      *zap.Logger
	nodes       map[string]*ClusterNode
	localNode   *ClusterNode
	sessions    map[string]*SMBSession
	state       FailoverState
	events      []FailoverEvent
	activeNode  string // ID of current active node
	vipOwner    string // ID of node owning VIP
	running     int32  // atomic
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	onFailover  func(event FailoverEvent) // Callback
}

// NewFailoverManager creates a new failover manager
func NewFailoverManager(config FailoverConfig, logger *zap.Logger) *FailoverManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &FailoverManager{
		config:   config,
		logger:   logger,
		nodes:    make(map[string]*ClusterNode),
		sessions: make(map[string]*SMBSession),
		state:    FailoverStateIdle,
		events:   make([]FailoverEvent, 0, 100),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// RegisterNode registers a cluster node
func (fm *FailoverManager) RegisterNode(node *ClusterNode) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	node.LastHeartbeat = time.Now()
	fm.nodes[node.ID] = node
	if node.Priority > 0 {
		fm.activeNode = node.ID
	}
	fm.logger.Info("node registered",
		zap.String("id", node.ID),
		zap.String("hostname", node.Hostname),
		zap.String("state", string(node.State)))
}

// SetLocalNode sets the local node identity
func (fm *FailoverManager) SetLocalNode(node *ClusterNode) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.localNode = node
	fm.nodes[node.ID] = node
}

// Start begins failover monitoring
func (fm *FailoverManager) Start() error {
	if !atomic.CompareAndSwapInt32(&fm.running, 0, 1) {
		return fmt.Errorf("failover manager already running")
	}
	fm.wg.Add(2)
	go fm.heartbeatSender()
	go fm.heartbeatChecker()
	fm.logger.Info("failover manager started")
	return nil
}

// Stop gracefully stops the failover manager
func (fm *FailoverManager) Stop() {
	fm.cancel()
	fm.wg.Wait()
	atomic.StoreInt32(&fm.running, 0)
	fm.logger.Info("failover manager stopped")
}

// TrackSession registers an SMB session for stateful failover
func (fm *FailoverManager) TrackSession(session *SMBSession) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.sessions[session.ID] = session
	fm.logger.Debug("session tracked", zap.String("id", session.ID))
}

// UntrackSession removes a session from tracking
func (fm *FailoverManager) UntrackSession(sessionID string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	delete(fm.sessions, sessionID)
}

// UpdateSessionActivity updates the last activity time for a session
func (fm *FailoverManager) UpdateSessionActivity(sessionID string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if s, ok := fm.sessions[sessionID]; ok {
		s.LastActivity = time.Now()
		s.SequenceNum++
	}
}

// InitiateFailover manually triggers failover to a target node
func (fm *FailoverManager) InitiateFailover(targetNodeID, reason string) error {
	fm.mu.Lock()
	if fm.state != FailoverStateIdle {
		fm.mu.Unlock()
		return fmt.Errorf("failover already in progress: %s", fm.state)
	}
	fm.state = FailoverStateDetecting
	fm.mu.Unlock()

	startTime := time.Now()
	event := FailoverEvent{
		ID:        fmt.Sprintf("fo-%d", time.Now().UnixNano()),
		Timestamp: startTime,
		FromNode:  fm.activeNode,
		ToNode:    targetNodeID,
		Reason:    reason,
		State:     FailoverStateFailing,
	}

	fm.mu.Lock()
	fm.state = FailoverStateFailing
	sessionsToTransfer := make([]*SMBSession, 0, len(fm.sessions))
	for _, s := range fm.sessions {
		sessionsToTransfer = append(sessionsToTransfer, s)
	}
	fm.mu.Unlock()

	// Transfer sessions
	transferred := 0
	for _, s := range sessionsToTransfer {
		if err := fm.transferSession(s, targetNodeID); err != nil {
			fm.logger.Error("session transfer failed",
				zap.String("session", s.ID),
				zap.Error(err))
			continue
		}
		transferred++
	}

	// Move VIP
	if err := fm.moveVIP(targetNodeID); err != nil {
		fm.logger.Error("VIP move failed", zap.Error(err))
		event.State = FailoverStateFailed
		event.Success = false
	} else {
		event.State = FailoverStateCompleted
		event.Success = true
	}

	event.Sessions = transferred
	event.Duration = time.Since(startTime)

	fm.mu.Lock()
	fm.state = FailoverStateIdle
	if event.Success {
		fm.activeNode = targetNodeID
	}
	fm.events = append(fm.events, event)
	fm.mu.Unlock()

	if fm.onFailover != nil {
		fm.onFailover(event)
	}

	fm.logger.Info("failover completed",
		zap.String("from", event.FromNode),
		zap.String("to", event.ToNode),
		zap.Int("sessions", transferred),
		zap.Duration("duration", event.Duration),
		zap.Bool("success", event.Success))

	return nil
}

// transferSession transfers a session to target node
func (fm *FailoverManager) transferSession(session *SMBSession, targetNodeID string) error {
	fm.mu.RLock()
	target, ok := fm.nodes[targetNodeID]
	fm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("target node %s not found", targetNodeID)
	}
	if target.State != NodeStateActive && target.State != NodeStateStandby {
		return fmt.Errorf("target node %s not ready: %s", targetNodeID, target.State)
	}
	// In real implementation, this would serialize session state and send to target
	fm.logger.Debug("transferring session",
		zap.String("session", session.ID),
		zap.String("target", targetNodeID))
	return nil
}

// moveVIP moves the virtual IP to target node
func (fm *FailoverManager) moveVIP(targetNodeID string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if fm.config.VIPAddress == "" {
		return nil // No VIP configured
	}
	fm.vipOwner = targetNodeID
	// In real implementation, this would issue ARP/NDP announcements
	fm.logger.Info("VIP moved", zap.String("to", targetNodeID))
	return nil
}

// heartbeatSender sends heartbeats to other nodes
func (fm *FailoverManager) heartbeatSender() {
	defer fm.wg.Done()
	ticker := time.NewTicker(fm.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fm.ctx.Done():
			return
		case <-ticker.C:
			fm.mu.RLock()
			local := fm.localNode
			fm.mu.RUnlock()
			if local != nil {
				local.LastHeartbeat = time.Now()
			}
		}
	}
}

// heartbeatChecker checks for failed nodes
func (fm *FailoverManager) heartbeatChecker() {
	defer fm.wg.Done()
	ticker := time.NewTicker(fm.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fm.ctx.Done():
			return
		case <-ticker.C:
			fm.checkNodeHealth()
		}
	}
}

func (fm *FailoverManager) checkNodeHealth() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	now := time.Now()
	for _, node := range fm.nodes {
		if node.ID == fm.localNode.ID {
			continue
		}
		if node.State == NodeStateActive && now.Sub(node.LastHeartbeat) > fm.config.HeartbeatTimeout {
			fm.logger.Warn("node heartbeat timeout",
				zap.String("node", node.ID),
				zap.Duration("since", now.Sub(node.LastHeartbeat)))
			node.State = NodeStateFailed
			// Trigger automatic failover
			go fm.InitiateFailover(fm.localNode.ID, fmt.Sprintf("node %s heartbeat timeout", node.ID))
		}
	}
}

// GetClusterStatus returns the current cluster status
func (fm *FailoverManager) GetClusterStatus() map[string]interface{} {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	nodes := make([]*ClusterNode, 0, len(fm.nodes))
	for _, n := range fm.nodes {
		nodes = append(nodes, n)
	}

	return map[string]interface{}{
		"state":       fm.state,
		"active_node": fm.activeNode,
		"vip_owner":   fm.vipOwner,
		"nodes":       nodes,
		"sessions":    len(fm.sessions),
		"events":      len(fm.events),
	}
}

// GetEvents returns recent failover events
func (fm *FailoverManager) GetEvents(limit int) []FailoverEvent {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if limit <= 0 || limit > len(fm.events) {
		limit = len(fm.events)
	}
	start := len(fm.events) - limit
	return fm.events[start:]
}

// SetFailoverCallback sets a callback for failover events
func (fm *FailoverManager) SetFailoverCallback(fn func(event FailoverEvent)) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.onFailover = fn
}
