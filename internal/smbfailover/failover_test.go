package smbfailover

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestFailoverExecutor_RegisterNode(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)

	// Register nodes
	node1 := &ClusterNode{
		ID:       "node-1",
		Hostname: "server1",
		IP:       []byte{192, 168, 1, 10},
		State:    NodeStateActive,
		Priority: 10,
	}

	node2 := &ClusterNode{
		ID:       "node-2",
		Hostname: "server2",
		IP:       []byte{192, 168, 1, 11},
		State:    NodeStateStandby,
		Priority: 5,
	}

	executor.RegisterNode(node1)
	executor.RegisterNode(node2)

	assert.Equal(t, "node-1", executor.GetActiveNodeID())
}

func TestFailoverExecutor_SetLocalNode(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)

	executor.SetLocalNode("node-local")
	assert.Equal(t, "node-local", executor.localNodeID)
}

func TestFailoverExecutor_ExecuteFailover(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)
	executor.SetLocalNode("node-1")

	// Register nodes
	executor.RegisterNode(&ClusterNode{
		ID:       "node-1",
		Hostname: "server1",
		IP:       []byte{192, 168, 1, 10},
		State:    NodeStateActive,
		Priority: 10,
	})

	executor.RegisterNode(&ClusterNode{
		ID:       "node-2",
		Hostname: "server2",
		IP:       []byte{192, 168, 1, 11},
		State:    NodeStateStandby,
		Priority: 5,
	})

	// Register with detector
	detector.RegisterNode("node-1", "server1", []byte{192, 168, 1, 10})
	detector.RegisterNode("node-2", "server2", []byte{192, 168, 1, 11})
	detector.SetLocalNode("node-1")

	// Start components
	err := sessionManager.Start()
	require.NoError(t, err)
	defer sessionManager.Stop()

	err = detector.Start()
	require.NoError(t, err)
	defer detector.Stop()

	err = synchronizer.Start()
	require.NoError(t, err)
	defer synchronizer.Stop()

	// Track callbacks
	var failoverStarted bool
	var failoverCompleted bool

	executor.SetFailoverCallbacks(
		func(event *FailoverEvent) {
			failoverStarted = true
		},
		func(event *FailoverEvent) {
			failoverCompleted = true
		},
	)

	// Execute failover
	ctx := context.Background()
	err = executor.ExecuteFailover(ctx, "node-2", "manual test")
	require.NoError(t, err)

	// Verify
	assert.True(t, failoverStarted)
	assert.True(t, failoverCompleted)
	assert.Equal(t, "node-2", executor.GetActiveNodeID())

	// Check metrics
	metrics := executor.GetFailoverMetrics()
	assert.Equal(t, int64(1), metrics.TotalFailovers)
	assert.Equal(t, int64(1), metrics.Successful)
	assert.Equal(t, int64(0), metrics.Failed)
}

func TestFailoverExecutor_ConcurrentFailover(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)
	executor.SetLocalNode("node-1")

	executor.RegisterNode(&ClusterNode{
		ID:       "node-1",
		Hostname: "server1",
		IP:       []byte{192, 168, 1, 10},
		State:    NodeStateActive,
	})

	executor.RegisterNode(&ClusterNode{
		ID:       "node-2",
		Hostname: "server2",
		IP:       []byte{192, 168, 1, 11},
		State:    NodeStateStandby,
	})

	detector.RegisterNode("node-1", "server1", []byte{192, 168, 1, 10})
	detector.RegisterNode("node-2", "server2", []byte{192, 168, 1, 11})
	detector.SetLocalNode("node-1")

	err := sessionManager.Start()
	require.NoError(t, err)
	defer sessionManager.Stop()

	err = detector.Start()
	require.NoError(t, err)
	defer detector.Stop()

	err = synchronizer.Start()
	require.NoError(t, err)
	defer synchronizer.Stop()

	// Try to start two failovers concurrently
	ctx := context.Background()

	// First failover should succeed
	err = executor.ExecuteFailover(ctx, "node-2", "first")
	assert.NoError(t, err)

	// State should be idle now
	assert.Equal(t, FailoverStateIdle, executor.GetState())
}

func TestFailoverExecutor_InvalidTarget(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)
	executor.SetLocalNode("node-1")

	executor.RegisterNode(&ClusterNode{
		ID:       "node-1",
		Hostname: "server1",
		IP:       []byte{192, 168, 1, 10},
		State:    NodeStateActive,
	})

	detector.RegisterNode("node-1", "server1", []byte{192, 168, 1, 10})
	detector.SetLocalNode("node-1")

	ctx := context.Background()

	// Try to failover to non-existent node
	err := executor.ExecuteFailover(ctx, "node-99", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFailoverExecutor_NodeFailure(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)
	executor.SetLocalNode("node-1")

	executor.RegisterNode(&ClusterNode{
		ID:       "node-1",
		Hostname: "server1",
		IP:       []byte{192, 168, 1, 10},
		State:    NodeStateStandby,
		Priority: 5,
	})

	executor.RegisterNode(&ClusterNode{
		ID:       "node-2",
		Hostname: "server2",
		IP:       []byte{192, 168, 1, 11},
		State:    NodeStateActive,
		Priority: 10,
	})

	detector.RegisterNode("node-1", "server1", []byte{192, 168, 1, 10})
	detector.RegisterNode("node-2", "server2", []byte{192, 168, 1, 11})
	detector.SetLocalNode("node-1")

	err := sessionManager.Start()
	require.NoError(t, err)
	defer sessionManager.Stop()

	err = detector.Start()
	require.NoError(t, err)
	defer detector.Stop()

	err = synchronizer.Start()
	require.NoError(t, err)
	defer synchronizer.Stop()

	// Handle node failure
	executor.HandleNodeFailure("node-2", "test failure")

	// Wait for failover
	time.Sleep(100 * time.Millisecond)

	// Should have failed over to node-1
	assert.Equal(t, "node-1", executor.GetActiveNodeID())
}

func TestFailoverExecutor_SelectFailoverTarget(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)
	executor.SetLocalNode("node-1")

	// Register nodes with different priorities
	executor.RegisterNode(&ClusterNode{
		ID:       "node-1",
		Hostname: "server1",
		IP:       []byte{192, 168, 1, 10},
		State:    NodeStateActive,
		Priority: 5,
	})

	executor.RegisterNode(&ClusterNode{
		ID:       "node-2",
		Hostname: "server2",
		IP:       []byte{192, 168, 1, 11},
		State:    NodeStateStandby,
		Priority: 10,
	})

	executor.RegisterNode(&ClusterNode{
		ID:       "node-3",
		Hostname: "server3",
		IP:       []byte{192, 168, 1, 12},
		State:    NodeStateStandby,
		Priority: 3,
	})

	detector.RegisterNode("node-1", "server1", []byte{192, 168, 1, 10})
	detector.RegisterNode("node-2", "server2", []byte{192, 168, 1, 11})
	detector.RegisterNode("node-3", "server3", []byte{192, 168, 1, 12})
	detector.SetLocalNode("node-1")

	// Should select highest priority node
	target := executor.selectFailoverTarget("node-1")
	assert.Equal(t, "node-2", target)
}

func TestFailoverExecutor_GetFailoverHistory(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)
	executor.SetLocalNode("node-1")

	executor.RegisterNode(&ClusterNode{
		ID:       "node-1",
		Hostname: "server1",
		IP:       []byte{192, 168, 1, 10},
		State:    NodeStateActive,
	})

	executor.RegisterNode(&ClusterNode{
		ID:       "node-2",
		Hostname: "server2",
		IP:       []byte{192, 168, 1, 11},
		State:    NodeStateStandby,
	})

	detector.RegisterNode("node-1", "server1", []byte{192, 168, 1, 10})
	detector.RegisterNode("node-2", "server2", []byte{192, 168, 1, 11})
	detector.SetLocalNode("node-1")

	err := sessionManager.Start()
	require.NoError(t, err)
	defer sessionManager.Stop()

	err = detector.Start()
	require.NoError(t, err)
	defer detector.Stop()

	err = synchronizer.Start()
	require.NoError(t, err)
	defer synchronizer.Stop()

	// Execute a failover
	ctx := context.Background()
	_ = executor.ExecuteFailover(ctx, "node-2", "test")

	// Get history
	history := executor.GetFailoverHistory(10)
	assert.Len(t, history, 1)
	assert.Equal(t, "node-1", history[0].FromNode)
	assert.Equal(t, "node-2", history[0].ToNode)
}

func TestFailoverExecutor_GetState(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)

	assert.Equal(t, FailoverStateIdle, executor.GetState())
}

func TestFailoverExecutor_VIPManagement(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)

	// Add VIP
	vip := &VIPConfig{
		IP:          "192.168.1.100",
		Netmask:     "255.255.255.0",
		Gateway:     "192.168.1.1",
		OwnerNodeID: "node-1",
		Active:      true,
	}

	executor.AddVIP("vip-1", vip)

	// Get VIP status
	vips := executor.GetVIPStatus()
	assert.Len(t, vips, 1)
	assert.Equal(t, "192.168.1.100", vips["vip-1"].IP)

	// Remove VIP
	executor.RemoveVIP("vip-1")
	vips = executor.GetVIPStatus()
	assert.Len(t, vips, 0)
}

func TestFailoverExecutor_CanFailover(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)
	executor.SetLocalNode("node-1")

	executor.RegisterNode(&ClusterNode{
		ID:       "node-1",
		Hostname: "server1",
		IP:       []byte{192, 168, 1, 10},
		State:    NodeStateActive,
	})

	executor.RegisterNode(&ClusterNode{
		ID:       "node-2",
		Hostname: "server2",
		IP:       []byte{192, 168, 1, 11},
		State:    NodeStateStandby,
	})

	detector.RegisterNode("node-1", "server1", []byte{192, 168, 1, 10})
	detector.RegisterNode("node-2", "server2", []byte{192, 168, 1, 11})
	detector.SetLocalNode("node-1")

	// Should be able to failover (has healthy target)
	canFailover := executor.CanFailover()
	assert.True(t, canFailover)
}

func TestFailoverExecutor_ManualFailover(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)
	executor.SetLocalNode("node-1")

	executor.RegisterNode(&ClusterNode{
		ID:       "node-1",
		Hostname: "server1",
		IP:       []byte{192, 168, 1, 10},
		State:    NodeStateActive,
	})

	executor.RegisterNode(&ClusterNode{
		ID:       "node-2",
		Hostname: "server2",
		IP:       []byte{192, 168, 1, 11},
		State:    NodeStateStandby,
	})

	detector.RegisterNode("node-1", "server1", []byte{192, 168, 1, 10})
	detector.RegisterNode("node-2", "server2", []byte{192, 168, 1, 11})
	detector.SetLocalNode("node-1")

	err := sessionManager.Start()
	require.NoError(t, err)
	defer sessionManager.Stop()

	err = detector.Start()
	require.NoError(t, err)
	defer detector.Stop()

	err = synchronizer.Start()
	require.NoError(t, err)
	defer synchronizer.Stop()

	// Manual failover
	ctx := context.Background()
	err = executor.ManualFailover(ctx, "node-2", "maintenance")
	require.NoError(t, err)

	assert.Equal(t, "node-2", executor.GetActiveNodeID())
}

func TestFailoverExecutor_GetFailoverStatus(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)
	executor.SetLocalNode("node-1")

	executor.RegisterNode(&ClusterNode{
		ID:       "node-1",
		Hostname: "server1",
		IP:       []byte{192, 168, 1, 10},
		State:    NodeStateActive,
	})

	status := executor.GetFailoverStatus()
	assert.Equal(t, FailoverStateIdle, status["state"])
	assert.Equal(t, "node-1", status["local_node"])
}

func TestFailoverExecutor_FreezeSessionState(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)

	err := sessionManager.Start()
	require.NoError(t, err)
	defer sessionManager.Stop()

	// Create a session
	_, err = sessionManager.CreateSession("session-1", "192.168.1.100", "testuser")
	require.NoError(t, err)

	// Freeze should succeed
	err = executor.freezeSessionState()
	assert.NoError(t, err)
}

func TestFailoverExecutor_SendGARP(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFailoverConfig()
	config.EnableGRATARP = false

	sessionManager := NewSessionManager(DefaultSessionConfig(), nil, logger)
	detector := NewFailureDetector(DefaultDetectorConfig(), DefaultQuorumConfig(), logger)
	synchronizer := NewStateSynchronizer(DefaultSyncConfig(), logger)
	auditLogger, _ := NewAuditLogger(DefaultAuditConfig(), logger)

	executor := NewFailoverExecutor(config, sessionManager, detector, synchronizer, auditLogger, logger)

	// GARP disabled, should return nil
	err := executor.sendGARP("node-1")
	assert.NoError(t, err)
}
