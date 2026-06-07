package smbfailover

import (
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FailureDetector detects node failures using multiple detection methods
type FailureDetector struct {
	mu               sync.RWMutex
	config           DetectorConfig
	logger           *zap.Logger
	nodes            map[string]*NodeHealth
	localNodeID      string
	quorumConfig     QuorumConfig
	failureCallback  func(nodeID string, reason string)
	recoveryCallback func(nodeID string)
	running          bool
	stopCh           chan struct{}
}

// DetectorConfig configures failure detection
type DetectorConfig struct {
	HeartbeatInterval   time.Duration `json:"heartbeat_interval"`
	HeartbeatTimeout    time.Duration `json:"heartbeat_timeout"`
	TCPCheckInterval    time.Duration `json:"tcp_check_interval"`
	TCPCheckTimeout     time.Duration `json:"tcp_check_timeout"`
	TCPCheckPorts       []int         `json:"tcp_check_ports"`
	QuorumCheckInterval time.Duration `json:"quorum_check_interval"`
	MaxMissedHeartbeats int           `json:"max_missed_heartbeats"`
	AggressiveMode      bool          `json:"aggressive_mode"`
	NetworkPartition    bool          `json:"detect_network_partition"`
}

// DefaultDetectorConfig returns sensible defaults
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		HeartbeatInterval:   1 * time.Second,
		HeartbeatTimeout:    3 * time.Second,
		TCPCheckInterval:    2 * time.Second,
		TCPCheckTimeout:     1 * time.Second,
		TCPCheckPorts:       []int{445, 139}, // SMB ports
		QuorumCheckInterval: 5 * time.Second,
		MaxMissedHeartbeats: 3,
		AggressiveMode:      false,
		NetworkPartition:    true,
	}
}

// NodeHealth tracks the health of a cluster node
type NodeHealth struct {
	mu                sync.RWMutex
	NodeID            string        `json:"node_id"`
	Hostname          string        `json:"hostname"`
	IP                net.IP        `json:"ip"`
	State             NodeState     `json:"state"`
	LastHeartbeat     time.Time     `json:"last_heartbeat"`
	MissedHeartbeats  int           `json:"missed_heartbeats"`
	LastTCPCheck      time.Time     `json:"last_tcp_check"`
	TCPCheckOK        bool          `json:"tcp_check_ok"`
	LastQuorumCheck   time.Time     `json:"last_quorum_check"`
	QuorumMember      bool          `json:"quorum_member"`
	NetworkReachable  bool          `json:"network_reachable"`
	Latency           time.Duration `json:"latency"`
	Failures          int           `json:"failures"`
	LastFailure       time.Time     `json:"last_failure,omitempty"`
	LastFailureReason string        `json:"last_failure_reason,omitempty"`
	RecoveryTime      time.Time     `json:"recovery_time,omitempty"`
	TotalFailures     int64         `json:"total_failures"`
	TotalRecoveries   int64         `json:"total_recoveries"`
}

// QuorumConfig configures quorum behavior
type QuorumConfig struct {
	Enabled       bool    `json:"enabled"`
	MinNodes      int     `json:"min_nodes"`      // Minimum nodes for quorum
	QuorumPercent float64 `json:"quorum_percent"` // Percentage of nodes required
	ForceQuorum   bool    `json:"force_quorum"`   // Force quorum even with single node
}

// DefaultQuorumConfig returns default quorum configuration
func DefaultQuorumConfig() QuorumConfig {
	return QuorumConfig{
		Enabled:       true,
		MinNodes:      2,
		QuorumPercent: 0.5,
		ForceQuorum:   false,
	}
}

// HeartbeatMessage is sent between nodes
type HeartbeatMessage struct {
	NodeID       string    `json:"node_id"`
	Timestamp    time.Time `json:"timestamp"`
	Sequence     uint64    `json:"sequence"`
	SessionCount int       `json:"session_count"`
	CPULoad      float64   `json:"cpu_load"`
	MemUsage     float64   `json:"mem_usage"`
	State        NodeState `json:"state"`
}

// NewFailureDetector creates a new failure detector
func NewFailureDetector(config DetectorConfig, quorumConfig QuorumConfig, logger *zap.Logger) *FailureDetector {
	return &FailureDetector{
		config:       config,
		logger:       logger,
		nodes:        make(map[string]*NodeHealth),
		quorumConfig: quorumConfig,
		stopCh:       make(chan struct{}),
	}
}

// SetLocalNode sets the local node ID
func (fd *FailureDetector) SetLocalNode(nodeID string) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	fd.localNodeID = nodeID
}

// RegisterNode registers a node for health monitoring
func (fd *FailureDetector) RegisterNode(nodeID, hostname string, ip net.IP) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	fd.nodes[nodeID] = &NodeHealth{
		NodeID:           nodeID,
		Hostname:         hostname,
		IP:               ip,
		State:            NodeStateStandby,
		LastHeartbeat:    time.Now(),
		NetworkReachable: true,
		QuorumMember:     true,
	}

	fd.logger.Info("node registered for monitoring",
		zap.String("node_id", nodeID),
		zap.String("hostname", hostname),
		zap.String("ip", ip.String()))
}

// UnregisterNode removes a node from monitoring
func (fd *FailureDetector) UnregisterNode(nodeID string) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	delete(fd.nodes, nodeID)
	fd.logger.Info("node unregistered", zap.String("node_id", nodeID))
}

// Start starts the failure detector
func (fd *FailureDetector) Start() error {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	if fd.running {
		return fmt.Errorf("failure detector already running")
	}

	fd.running = true

	go fd.heartbeatMonitor()
	go fd.tcpCheckMonitor()
	if fd.quorumConfig.Enabled {
		go fd.quorumMonitor()
	}

	fd.logger.Info("failure detector started",
		zap.Duration("heartbeat_interval", fd.config.HeartbeatInterval),
		zap.Duration("heartbeat_timeout", fd.config.HeartbeatTimeout),
		zap.Bool("quorum_enabled", fd.quorumConfig.Enabled))

	return nil
}

// Stop stops the failure detector
func (fd *FailureDetector) Stop() {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	if !fd.running {
		return
	}

	close(fd.stopCh)
	fd.running = false
	fd.logger.Info("failure detector stopped")
}

// UpdateHeartbeat updates the heartbeat for a node
func (fd *FailureDetector) UpdateHeartbeat(msg HeartbeatMessage) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	node, ok := fd.nodes[msg.NodeID]
	if !ok {
		fd.logger.Warn("heartbeat from unknown node", zap.String("node_id", msg.NodeID))
		return
	}

	node.mu.Lock()
	node.LastHeartbeat = time.Now()
	node.MissedHeartbeats = 0
	node.State = msg.State
	node.mu.Unlock()

	fd.logger.Debug("heartbeat received",
		zap.String("node_id", msg.NodeID),
		zap.Uint64("sequence", msg.Sequence),
		zap.Int("sessions", msg.SessionCount))
}

// SetFailureCallback sets the callback for node failure
func (fd *FailureDetector) SetFailureCallback(fn func(nodeID string, reason string)) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	fd.failureCallback = fn
}

// SetRecoveryCallback sets the callback for node recovery
func (fd *FailureDetector) SetRecoveryCallback(fn func(nodeID string)) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	fd.recoveryCallback = fn
}

// heartbeatMonitor monitors heartbeats from nodes
func (fd *FailureDetector) heartbeatMonitor() {
	ticker := time.NewTicker(fd.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fd.stopCh:
			return
		case <-ticker.C:
			fd.checkHeartbeats()
		}
	}
}

// checkHeartbeats checks for missed heartbeats
func (fd *FailureDetector) checkHeartbeats() {
	fd.mu.RLock()
	var failedNodes []string

	for nodeID, node := range fd.nodes {
		if nodeID == fd.localNodeID {
			continue
		}

		node.mu.RLock()
		elapsed := time.Since(node.LastHeartbeat)
		missed := node.MissedHeartbeats
		node.mu.RUnlock()

		if elapsed > fd.config.HeartbeatTimeout {
			missed++
			node.mu.Lock()
			node.MissedHeartbeats = missed
			node.mu.Unlock()

			if missed >= fd.config.MaxMissedHeartbeats {
				if node.State != NodeStateFailed {
					failedNodes = append(failedNodes, nodeID)
				}
			}
		}
	}
	fd.mu.RUnlock()

	for _, nodeID := range failedNodes {
		fd.handleNodeFailure(nodeID, "heartbeat_timeout")
	}
}

// handleNodeFailure handles a node failure
func (fd *FailureDetector) handleNodeFailure(nodeID, reason string) {
	fd.mu.Lock()

	node, ok := fd.nodes[nodeID]
	if !ok {
		fd.mu.Unlock()
		return
	}

	node.mu.Lock()
	if node.State == NodeStateFailed {
		node.mu.Unlock()
		fd.mu.Unlock()
		return
	}

	node.State = NodeStateFailed
	node.LastFailure = time.Now()
	node.LastFailureReason = reason
	node.Failures++
	node.TotalFailures++
	node.mu.Unlock()

	fd.mu.Unlock()

	fd.logger.Error("node failure detected",
		zap.String("node_id", nodeID),
		zap.String("reason", reason),
		zap.Time("last_heartbeat", node.LastHeartbeat))

	fd.mu.RLock()
	callback := fd.failureCallback
	fd.mu.RUnlock()

	if callback != nil {
		go callback(nodeID, reason)
	}
}

// tcpCheckMonitor performs TCP connectivity checks
func (fd *FailureDetector) tcpCheckMonitor() {
	ticker := time.NewTicker(fd.config.TCPCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fd.stopCh:
			return
		case <-ticker.C:
			fd.runTCPChecks()
		}
	}
}

// runTCPChecks performs TCP checks on all nodes
func (fd *FailureDetector) runTCPChecks() {
	fd.mu.RLock()
	nodes := make([]*NodeHealth, 0, len(fd.nodes))
	for _, node := range fd.nodes {
		if node.NodeID != fd.localNodeID {
			nodes = append(nodes, node)
		}
	}
	fd.mu.RUnlock()

	for _, node := range nodes {
		node.mu.RLock()
		ip := node.IP
		nodeID := node.NodeID
		node.mu.RUnlock()

		reachable := fd.checkTCPConnectivity(ip)
		fd.updateNodeReachability(nodeID, reachable)
	}
}

// checkTCPConnectivity checks if a node is reachable via TCP
func (fd *FailureDetector) checkTCPConnectivity(ip net.IP) bool {
	for _, port := range fd.config.TCPCheckPorts {
		var addr string
		if ip.To4() != nil {
			addr = fmt.Sprintf("%s:%d", ip.String(), port)
		} else {
			addr = fmt.Sprintf("[%s]:%d", ip.String(), port)
		}
		conn, err := net.DialTimeout("tcp", addr, fd.config.TCPCheckTimeout)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

// updateNodeReachability updates node reachability status
func (fd *FailureDetector) updateNodeReachability(nodeID string, reachable bool) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	node, ok := fd.nodes[nodeID]
	if !ok {
		return
	}

	node.mu.Lock()
	wasReachable := node.NetworkReachable
	node.NetworkReachable = reachable
	node.LastTCPCheck = time.Now()
	node.TCPCheckOK = reachable
	node.mu.Unlock()

	// If node became unreachable
	if wasReachable && !reachable {
		fd.logger.Warn("node became unreachable",
			zap.String("node_id", nodeID))

		if fd.config.AggressiveMode {
			go fd.handleNodeFailure(nodeID, "tcp_unreachable")
		}
	}

	// If node became reachable again
	if !wasReachable && reachable {
		fd.logger.Info("node became reachable",
			zap.String("node_id", nodeID))
		go fd.handleNodeRecovery(nodeID)
	}
}

// handleNodeRecovery handles node recovery
func (fd *FailureDetector) handleNodeRecovery(nodeID string) {
	fd.mu.Lock()

	node, ok := fd.nodes[nodeID]
	if !ok {
		fd.mu.Unlock()
		return
	}

	node.mu.Lock()
	if node.State != NodeStateFailed {
		node.mu.Unlock()
		fd.mu.Unlock()
		return
	}

	node.State = NodeStateRecovery
	node.RecoveryTime = time.Now()
	node.MissedHeartbeats = 0
	node.TotalRecoveries++
	node.mu.Unlock()

	fd.mu.Unlock()

	fd.logger.Info("node recovery detected",
		zap.String("node_id", nodeID),
		zap.Duration("downtime", time.Since(node.LastFailure)))

	fd.mu.RLock()
	callback := fd.recoveryCallback
	fd.mu.RUnlock()

	if callback != nil {
		go callback(nodeID)
	}
}

// quorumMonitor monitors quorum status
func (fd *FailureDetector) quorumMonitor() {
	ticker := time.NewTicker(fd.config.QuorumCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fd.stopCh:
			return
		case <-ticker.C:
			fd.checkQuorum()
		}
	}
}

// checkQuorum checks if quorum is maintained
func (fd *FailureDetector) checkQuorum() bool {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	totalNodes := len(fd.nodes)
	activeNodes := 0

	for _, node := range fd.nodes {
		node.mu.RLock()
		if node.State == NodeStateActive || node.State == NodeStateStandby {
			activeNodes++
		}
		node.mu.RUnlock()
	}

	if totalNodes == 0 {
		return fd.quorumConfig.ForceQuorum
	}

	hasQuorum := false
	if fd.quorumConfig.ForceQuorum && totalNodes == 1 {
		hasQuorum = true
	} else if activeNodes >= fd.quorumConfig.MinNodes {
		quorumCount := int(float64(totalNodes) * fd.quorumConfig.QuorumPercent)
		if quorumCount < fd.quorumConfig.MinNodes {
			quorumCount = fd.quorumConfig.MinNodes
		}
		hasQuorum = activeNodes >= quorumCount
	}

	fd.logger.Debug("quorum check",
		zap.Int("total_nodes", totalNodes),
		zap.Int("active_nodes", activeNodes),
		zap.Bool("has_quorum", hasQuorum))

	return hasQuorum
}

// HasQuorum returns true if quorum is maintained
func (fd *FailureDetector) HasQuorum() bool {
	return fd.checkQuorum()
}

// GetNodeHealth returns the health status of a node
func (fd *FailureDetector) GetNodeHealth(nodeID string) (*NodeHealth, bool) {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	node, ok := fd.nodes[nodeID]
	if !ok {
		return nil, false
	}

	// Create a copy
	node.mu.RLock()
	copy := &NodeHealth{
		NodeID:            node.NodeID,
		Hostname:          node.Hostname,
		IP:                node.IP,
		State:             node.State,
		LastHeartbeat:     node.LastHeartbeat,
		MissedHeartbeats:  node.MissedHeartbeats,
		LastTCPCheck:      node.LastTCPCheck,
		TCPCheckOK:        node.TCPCheckOK,
		LastQuorumCheck:   node.LastQuorumCheck,
		QuorumMember:      node.QuorumMember,
		NetworkReachable:  node.NetworkReachable,
		Latency:           node.Latency,
		Failures:          node.Failures,
		LastFailure:       node.LastFailure,
		LastFailureReason: node.LastFailureReason,
		RecoveryTime:      node.RecoveryTime,
		TotalFailures:     node.TotalFailures,
		TotalRecoveries:   node.TotalRecoveries,
	}
	node.mu.RUnlock()

	return copy, true
}

// GetAllNodeHealth returns health status of all nodes
func (fd *FailureDetector) GetAllNodeHealth() map[string]*NodeHealth {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	result := make(map[string]*NodeHealth, len(fd.nodes))
	for id, node := range fd.nodes {
		node.mu.RLock()
		copy := &NodeHealth{
			NodeID:            node.NodeID,
			Hostname:          node.Hostname,
			IP:                node.IP,
			State:             node.State,
			LastHeartbeat:     node.LastHeartbeat,
			MissedHeartbeats:  node.MissedHeartbeats,
			LastTCPCheck:      node.LastTCPCheck,
			TCPCheckOK:        node.TCPCheckOK,
			LastQuorumCheck:   node.LastQuorumCheck,
			QuorumMember:      node.QuorumMember,
			NetworkReachable:  node.NetworkReachable,
			Latency:           node.Latency,
			Failures:          node.Failures,
			LastFailure:       node.LastFailure,
			LastFailureReason: node.LastFailureReason,
			RecoveryTime:      node.RecoveryTime,
			TotalFailures:     node.TotalFailures,
			TotalRecoveries:   node.TotalRecoveries,
		}
		node.mu.RUnlock()
		result[id] = copy
	}
	return result
}

// GetHealthyNodes returns IDs of healthy nodes
func (fd *FailureDetector) GetHealthyNodes() []string {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	var healthy []string
	for id, node := range fd.nodes {
		node.mu.RLock()
		if node.State == NodeStateActive || node.State == NodeStateStandby {
			healthy = append(healthy, id)
		}
		node.mu.RUnlock()
	}
	return healthy
}

// GetFailedNodes returns IDs of failed nodes
func (fd *FailureDetector) GetFailedNodes() []string {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	var failed []string
	for id, node := range fd.nodes {
		node.mu.RLock()
		if node.State == NodeStateFailed {
			failed = append(failed, id)
		}
		node.mu.RUnlock()
	}
	return failed
}

// IsNodeHealthy returns true if a node is healthy
func (fd *FailureDetector) IsNodeHealthy(nodeID string) bool {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	node, ok := fd.nodes[nodeID]
	if !ok {
		return false
	}

	node.mu.RLock()
	defer node.mu.RUnlock()

	return node.State == NodeStateActive || node.State == NodeStateStandby
}

// ForceNodeFailed forces a node to failed state (for testing/manual intervention)
func (fd *FailureDetector) ForceNodeFailed(nodeID, reason string) {
	fd.handleNodeFailure(nodeID, reason)
}

// ForceNodeRecovery forces a node to recovery state
func (fd *FailureDetector) ForceNodeRecovery(nodeID string) {
	fd.handleNodeRecovery(nodeID)
}

// GetClusterHealth returns overall cluster health
func (fd *FailureDetector) GetClusterHealth() map[string]interface{} {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	totalNodes := len(fd.nodes)
	healthyNodes := 0
	failedNodes := 0
	recoveringNodes := 0

	for _, node := range fd.nodes {
		node.mu.RLock()
		switch node.State {
		case NodeStateActive, NodeStateStandby:
			healthyNodes++
		case NodeStateFailed:
			failedNodes++
		case NodeStateRecovery:
			recoveringNodes++
		}
		node.mu.RUnlock()
	}

	hasQuorum := fd.checkQuorum()

	return map[string]interface{}{
		"total_nodes":      totalNodes,
		"healthy_nodes":    healthyNodes,
		"failed_nodes":     failedNodes,
		"recovering_nodes": recoveringNodes,
		"has_quorum":       hasQuorum,
		"cluster_healthy":  hasQuorum && failedNodes == 0,
	}
}
