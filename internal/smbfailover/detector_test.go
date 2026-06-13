package smbfailover

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestFailureDetector_RegisterNode(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	// Register node
	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))

	// Get node health
	health, ok := detector.GetNodeHealth("node-1")
	assert.True(t, ok)
	assert.NotNil(t, health)
	assert.Equal(t, "node-1", health.NodeID)
	assert.Equal(t, "server1", health.Hostname)
	assert.True(t, health.NetworkReachable)

	// Unregister node
	detector.UnregisterNode("node-1")
	_, ok = detector.GetNodeHealth("node-1")
	assert.False(t, ok)
}

func TestFailureDetector_UpdateHeartbeat(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))
	detector.SetLocalNode("node-local")

	// Initial heartbeat
	initialTime := time.Now()
	detector.UpdateHeartbeat(HeartbeatMessage{
		NodeID:    "node-1",
		Timestamp: initialTime,
		Sequence:  1,
		State:     NodeStateActive,
	})

	health, _ := detector.GetNodeHealth("node-1")
	assert.Equal(t, 0, health.MissedHeartbeats)

	// Second heartbeat
	time.Sleep(10 * time.Millisecond)
	detector.UpdateHeartbeat(HeartbeatMessage{
		NodeID:    "node-1",
		Timestamp: time.Now(),
		Sequence:  2,
		State:     NodeStateActive,
	})

	health, _ = detector.GetNodeHealth("node-1")
	assert.True(t, health.LastHeartbeat.After(initialTime))
}

func TestFailureDetector_StartStop(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	// Start
	err := detector.Start()
	require.NoError(t, err)

	// Try to start again
	err = detector.Start()
	assert.Error(t, err)

	// Stop
	detector.Stop()
}

func TestFailureDetector_NodeFailure(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	config.HeartbeatTimeout = 50 * time.Millisecond
	config.HeartbeatInterval = 10 * time.Millisecond
	config.MaxMissedHeartbeats = 3
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))
	detector.SetLocalNode("node-local")

	// Set failure callback
	var failedNode string
	detector.SetFailureCallback(func(nodeID string, reason string) {
		failedNode = nodeID
	})

	// Start detector
	err := detector.Start()
	require.NoError(t, err)
	defer detector.Stop()

	// Send initial heartbeat
	detector.UpdateHeartbeat(HeartbeatMessage{
		NodeID:    "node-1",
		Timestamp: time.Now(),
		Sequence:  1,
		State:     NodeStateActive,
	})

	// Wait for failure detection
	time.Sleep(200 * time.Millisecond)

	// Check if failure was detected
	assert.Equal(t, "node-1", failedNode)

	health, _ := detector.GetNodeHealth("node-1")
	assert.Equal(t, NodeStateFailed, health.State)
}

func TestFailureDetector_NodeRecovery(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	config.TCPCheckPorts = []int{80} // Use a port that will fail
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))
	detector.SetLocalNode("node-local")

	// Set callbacks
	var recoveredNode string
	detector.SetRecoveryCallback(func(nodeID string) {
		recoveredNode = nodeID
	})

	// Mark node as failed - directly access internal nodes map
	detector.mu.RLock()
	node := detector.nodes["node-1"]
	detector.mu.RUnlock()
	
	node.mu.Lock()
	node.State = NodeStateFailed
	node.LastFailure = time.Now()
	node.mu.Unlock()

	// Handle recovery
	detector.handleNodeRecovery("node-1")

	// Wait for callback
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, "node-1", recoveredNode)

	health, _ := detector.GetNodeHealth("node-1")
	assert.Equal(t, NodeStateRecovery, health.State)
}

func TestFailureDetector_Quorum(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	quorumConfig := DefaultQuorumConfig()
	quorumConfig.MinNodes = 2
	quorumConfig.QuorumPercent = 0.5
	detector := NewFailureDetector(config, quorumConfig, logger)

	detector.SetLocalNode("node-local")

	// Add nodes
	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))
	detector.RegisterNode("node-2", "server2", net.ParseIP("192.168.1.11"))
	detector.RegisterNode("node-3", "server3", net.ParseIP("192.168.1.12"))

	// All nodes healthy - should have quorum
	hasQuorum := detector.HasQuorum()
	assert.True(t, hasQuorum)

	// Mark one node as failed - directly access internal nodes map
	detector.mu.RLock()
	node2 := detector.nodes["node-2"]
	detector.mu.RUnlock()
	node2.mu.Lock()
	node2.State = NodeStateFailed
	node2.mu.Unlock()

	// Should still have quorum (2/3 > 50%)
	hasQuorum = detector.HasQuorum()
	assert.True(t, hasQuorum)

	// Mark another node as failed
	detector.mu.RLock()
	node3 := detector.nodes["node-3"]
	detector.mu.RUnlock()
	node3.mu.Lock()
	node3.State = NodeStateFailed
	node3.mu.Unlock()

	// Should lose quorum (1/3 < 50%)
	hasQuorum = detector.HasQuorum()
	assert.False(t, hasQuorum)
}

func TestFailureDetector_GetHealthyNodes(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))
	detector.RegisterNode("node-2", "server2", net.ParseIP("192.168.1.11"))
	detector.RegisterNode("node-3", "server3", net.ParseIP("192.168.1.12"))

	// Mark one node as failed - directly access internal nodes map
	detector.mu.RLock()
	node2 := detector.nodes["node-2"]
	detector.mu.RUnlock()
	node2.mu.Lock()
	node2.State = NodeStateFailed
	node2.mu.Unlock()

	healthy := detector.GetHealthyNodes()
	assert.Len(t, healthy, 2)
	assert.Contains(t, healthy, "node-1")
	assert.Contains(t, healthy, "node-3")
	assert.NotContains(t, healthy, "node-2")
}

func TestFailureDetector_GetFailedNodes(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))
	detector.RegisterNode("node-2", "server2", net.ParseIP("192.168.1.11"))

	// Mark node as failed using ForceNodeFailed
	detector.ForceNodeFailed("node-2", "test failure")

	failed := detector.GetFailedNodes()
	assert.Len(t, failed, 1)
	assert.Contains(t, failed, "node-2")
}

func TestFailureDetector_IsNodeHealthy(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))
	detector.RegisterNode("node-2", "server2", net.ParseIP("192.168.1.11"))

	// Node 1 should be healthy
	assert.True(t, detector.IsNodeHealthy("node-1"))

	// Mark node 2 as failed using ForceNodeFailed
	detector.ForceNodeFailed("node-2", "test failure")

	assert.False(t, detector.IsNodeHealthy("node-2"))

	// Non-existent node
	assert.False(t, detector.IsNodeHealthy("node-99"))
}

func TestFailureDetector_ForceNodeFailed(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))

	var failedNode string
	var failReason string
	detector.SetFailureCallback(func(nodeID string, reason string) {
		failedNode = nodeID
		failReason = reason
	})

	detector.ForceNodeFailed("node-1", "manual intervention")

	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, "node-1", failedNode)
	assert.Equal(t, "manual intervention", failReason)

	health, _ := detector.GetNodeHealth("node-1")
	assert.Equal(t, NodeStateFailed, health.State)
}

func TestFailureDetector_ForceNodeRecovery(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))

	// Mark as failed
	health, _ := detector.GetNodeHealth("node-1")
	// Mark as failed using ForceNodeFailed
	detector.ForceNodeFailed("node-1", "test failure")

	var recoveredNode string
	detector.SetRecoveryCallback(func(nodeID string) {
		recoveredNode = nodeID
	})

	detector.ForceNodeRecovery("node-1")

	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, "node-1", recoveredNode)

	health, _ = detector.GetNodeHealth("node-1")
	assert.Equal(t, NodeStateRecovery, health.State)
}

func TestFailureDetector_GetClusterHealth(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))
	detector.RegisterNode("node-2", "server2", net.ParseIP("192.168.1.11"))
	detector.RegisterNode("node-3", "server3", net.ParseIP("192.168.1.12"))

	// All healthy
	health := detector.GetClusterHealth()
	assert.Equal(t, 3, health["total_nodes"])
	assert.Equal(t, 3, health["healthy_nodes"])
	assert.Equal(t, 0, health["failed_nodes"])
	assert.True(t, health["cluster_healthy"].(bool))

	// Mark one failed using ForceNodeFailed
	detector.ForceNodeFailed("node-2", "test failure")

	health = detector.GetClusterHealth()
	assert.Equal(t, 2, health["healthy_nodes"])
	assert.Equal(t, 1, health["failed_nodes"])
	assert.False(t, health["cluster_healthy"].(bool))
}

func TestFailureDetector_GetAllNodeHealth(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))
	detector.RegisterNode("node-2", "server2", net.ParseIP("192.168.1.11"))

	allHealth := detector.GetAllNodeHealth()
	assert.Len(t, allHealth, 2)
	assert.Contains(t, allHealth, "node-1")
	assert.Contains(t, allHealth, "node-2")
}

func TestFailureDetector_TCPCheck(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	config.TCPCheckPorts = []int{80, 443}
	config.TCPCheckTimeout = 100 * time.Millisecond
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	// Test unreachable IP
	reachable := detector.checkTCPConnectivity(net.ParseIP("192.0.2.1")) // RFC 5737 test net
	assert.False(t, reachable)
}

func TestFailureDetector_UpdateNodeReachability(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultDetectorConfig()
	config.AggressiveMode = true
	quorumConfig := DefaultQuorumConfig()
	detector := NewFailureDetector(config, quorumConfig, logger)

	detector.RegisterNode("node-1", "server1", net.ParseIP("192.168.1.10"))

	var failedNode string
	detector.SetFailureCallback(func(nodeID string, reason string) {
		failedNode = nodeID
	})

	// Initially reachable
	health, _ := detector.GetNodeHealth("node-1")
	assert.True(t, health.NetworkReachable)

	// Make unreachable
	detector.updateNodeReachability("node-1", false)

	health, _ = detector.GetNodeHealth("node-1")
	assert.False(t, health.NetworkReachable)

	// Wait for failure callback
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, "node-1", failedNode)
}
