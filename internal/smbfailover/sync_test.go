package smbfailover

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestStateSynchronizer_AddNode(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	synchronizer := NewStateSynchronizer(config, logger)

	// Add node
	synchronizer.AddNode("node-1", "server1", "192.168.1.10", 8080)

	// Get node status
	nodes := synchronizer.GetNodeSyncStatus()
	assert.Len(t, nodes, 1)
	assert.Contains(t, nodes, "node-1")
	assert.Equal(t, "server1", nodes["node-1"].Hostname)
	assert.Equal(t, "192.168.1.10", nodes["node-1"].Address)
	assert.True(t, nodes["node-1"].Connected)

	// Remove node
	synchronizer.RemoveNode("node-1")
	nodes = synchronizer.GetNodeSyncStatus()
	assert.Len(t, nodes, 0)
}

func TestStateSynchronizer_SetLocalNode(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	synchronizer := NewStateSynchronizer(config, logger)

	synchronizer.SetLocalNode("node-local")
	assert.Equal(t, "node-local", synchronizer.localNodeID)
}

func TestStateSynchronizer_StartStop(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	synchronizer := NewStateSynchronizer(config, logger)

	// Start
	err := synchronizer.Start()
	require.NoError(t, err)

	// Stop
	synchronizer.Stop()
}

func TestStateSynchronizer_SyncSessions(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	synchronizer := NewStateSynchronizer(config, logger)

	synchronizer.SetLocalNode("node-1")
	synchronizer.AddNode("node-2", "server2", "192.168.1.11", 8080)

	// Start synchronizer
	err := synchronizer.Start()
	require.NoError(t, err)
	defer synchronizer.Stop()

	// Prepare sessions
	sessions := map[string][]byte{
		"session-1": []byte(`{"session_id":"session-1","client_ip":"192.168.1.100"}`),
		"session-2": []byte(`{"session_id":"session-2","client_ip":"192.168.1.101"}`),
	}

	// Sync sessions
	ctx := context.Background()
	err = synchronizer.SyncSessions(ctx, "node-2", sessions)
	require.NoError(t, err)

	// Check metrics
	metrics := synchronizer.GetSyncMetrics()
	assert.Equal(t, int64(1), metrics.TotalSyncs)
	assert.Equal(t, int64(1), metrics.SuccessfulSyncs)
}

func TestStateSynchronizer_SyncIncremental(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	synchronizer := NewStateSynchronizer(config, logger)

	synchronizer.SetLocalNode("node-1")
	synchronizer.AddNode("node-2", "server2", "192.168.1.11", 8080)

	err := synchronizer.Start()
	require.NoError(t, err)
	defer synchronizer.Stop()

	// Incremental changes
	changes := map[string][]byte{
		"session-1": []byte(`{"session_id":"session-1","client_ip":"192.168.1.100","updated":true}`),
	}

	ctx := context.Background()
	err = synchronizer.SyncIncremental(ctx, "node-2", changes)
	require.NoError(t, err)

	metrics := synchronizer.GetSyncMetrics()
	assert.Equal(t, int64(1), metrics.TotalSyncs)
}

func TestStateSynchronizer_SyncToInvalidNode(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	synchronizer := NewStateSynchronizer(config, logger)

	synchronizer.SetLocalNode("node-1")

	sessions := map[string][]byte{
		"session-1": []byte(`{"session_id":"session-1"}`),
	}

	ctx := context.Background()
	err := synchronizer.SyncSessions(ctx, "node-99", sessions)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestStateSynchronizer_Compression(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	config.CompressionEnabled = true
	synchronizer := NewStateSynchronizer(config, logger)

	// Prepare sessions
	sessions := map[string][]byte{
		"session-1": []byte(`{"session_id":"session-1","client_ip":"192.168.1.100","data":"some data to compress"}`),
	}

	// Compress
	compressed, err := synchronizer.compressSessions(sessions)
	require.NoError(t, err)
	assert.NotEmpty(t, compressed)
	assert.True(t, len(compressed["session-1"]) < len(sessions["session-1"]))

	// Decompress
	decompressed, err := synchronizer.decompressSessions(compressed)
	require.NoError(t, err)
	assert.Equal(t, sessions["session-1"], decompressed["session-1"])
}

func TestStateSynchronizer_HandleSyncRequest(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	synchronizer := NewStateSynchronizer(config, logger)

	synchronizer.SetLocalNode("node-2")

	// Create request
	request := SyncRequest{
		ID:         "sync-1",
		Type:       SyncTypeFull,
		SourceNode: "node-1",
		TargetNode: "node-2",
		Sessions: map[string][]byte{
			"session-1": []byte(`{"session_id":"session-1"}`),
		},
		Timestamp: time.Now(),
	}

	// Handle request
	response, err := synchronizer.HandleSyncRequest(request)
	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.SessionsSync)
}

func TestStateSynchronizer_QueueSync(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	synchronizer := NewStateSynchronizer(config, logger)

	synchronizer.AddNode("node-2", "server2", "192.168.1.11", 8080)

	err := synchronizer.Start()
	require.NoError(t, err)
	defer synchronizer.Stop()

	// Queue sync request
	request := SyncRequest{
		ID:         "sync-1",
		Type:       SyncTypeIncremental,
		SourceNode: "node-1",
		TargetNode: "node-2",
		Sessions: map[string][]byte{
			"session-1": []byte(`{"session_id":"session-1"}`),
		},
		Timestamp: time.Now(),
	}

	synchronizer.QueueSync(request)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)
}

func TestStateSynchronizer_GetActiveSyncs(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	synchronizer := NewStateSynchronizer(config, logger)

	// Initially no active syncs
	syncs := synchronizer.GetActiveSyncs()
	assert.Len(t, syncs, 0)
}

func TestStateSynchronizer_GetSyncMetrics(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	synchronizer := NewStateSynchronizer(config, logger)

	metrics := synchronizer.GetSyncMetrics()
	assert.Equal(t, int64(0), metrics.TotalSyncs)
	assert.Equal(t, int64(0), metrics.SuccessfulSyncs)
	assert.Equal(t, int64(0), metrics.FailedSyncs)
}

func TestStateSynchronizer_IsNodeInSync(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	synchronizer := NewStateSynchronizer(config, logger)

	synchronizer.AddNode("node-1", "server1", "192.168.1.10", 8080)

	// Update last sync time
	nodes := synchronizer.GetNodeSyncStatus()
	node := nodes["node-1"]
	node.mu.Lock()
	node.LastSync = time.Now()
	node.SyncLag = 1 * time.Second
	node.mu.Unlock()

	// Should be in sync with 5 second max lag
	assert.True(t, synchronizer.IsNodeInSync("node-1", 5*time.Second))

	// Should not be in sync with 500ms max lag
	assert.False(t, synchronizer.IsNodeInSync("node-1", 500*time.Millisecond))
}

func TestStateSynchronizer_GetNodeSyncStatus(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	synchronizer := NewStateSynchronizer(config, logger)

	synchronizer.AddNode("node-1", "server1", "192.168.1.10", 8080)
	synchronizer.AddNode("node-2", "server2", "192.168.1.11", 8081)

	nodes := synchronizer.GetNodeSyncStatus()
	assert.Len(t, nodes, 2)
	assert.Contains(t, nodes, "node-1")
	assert.Contains(t, nodes, "node-2")
}

func TestStateSynchronizer_SyncWorker(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultSyncConfig()
	config.MaxConcurrentSyncs = 2
	synchronizer := NewStateSynchronizer(config, logger)

	synchronizer.AddNode("node-2", "server2", "192.168.1.11", 8080)

	err := synchronizer.Start()
	require.NoError(t, err)
	defer synchronizer.Stop()

	// Queue multiple sync requests
	for i := 0; i < 5; i++ {
		request := SyncRequest{
			ID:         "sync-" + string(rune('0'+i)),
			Type:       SyncTypeIncremental,
			SourceNode: "node-1",
			TargetNode: "node-2",
			Sessions: map[string][]byte{
				"session-1": []byte(`{"session_id":"session-1"}`),
			},
			Timestamp: time.Now(),
		}
		synchronizer.QueueSync(request)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	metrics := synchronizer.GetSyncMetrics()
	assert.True(t, metrics.TotalSyncs > 0)
}

func TestSyncConfig_Defaults(t *testing.T) {
	config := DefaultSyncConfig()

	assert.Equal(t, 5*time.Second, config.SyncInterval)
	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 5, config.MaxConcurrentSyncs)
	assert.True(t, config.CompressionEnabled)
	assert.Equal(t, 3, config.RetryAttempts)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
	assert.Equal(t, 30*time.Second, config.Timeout)
}

func TestSyncEndpoint_Fields(t *testing.T) {
	endpoint := &SyncEndpoint{
		NodeID:    "node-1",
		Hostname:  "server1",
		Address:   "192.168.1.10",
		Port:      8080,
		Connected: true,
	}

	assert.Equal(t, "node-1", endpoint.NodeID)
	assert.Equal(t, "server1", endpoint.Hostname)
	assert.Equal(t, "192.168.1.10", endpoint.Address)
	assert.Equal(t, 8080, endpoint.Port)
	assert.True(t, endpoint.Connected)
}

func TestSyncOperation_States(t *testing.T) {
	op := &SyncOperation{
		ID:    "sync-1",
		State: SyncStatePending,
	}

	assert.Equal(t, SyncStatePending, op.State)

	op.State = SyncStateInProgress
	assert.Equal(t, SyncStateInProgress, op.State)

	op.State = SyncStateCompleted
	assert.Equal(t, SyncStateCompleted, op.State)

	op.State = SyncStateFailed
	assert.Equal(t, SyncStateFailed, op.State)
}

func TestSyncRequest_Types(t *testing.T) {
	tests := []struct {
		name     string
		syncType SyncType
	}{
		{"Full", SyncTypeFull},
		{"Incremental", SyncTypeIncremental},
		{"Delta", SyncTypeDelta},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := SyncRequest{
				ID:   "sync-1",
				Type: tt.syncType,
			}
			assert.Equal(t, tt.syncType, request.Type)
		})
	}
}

func TestSyncResponse_Fields(t *testing.T) {
	response := &SyncResponse{
		ID:           "sync-1",
		Success:      true,
		Message:      "sync completed",
		SessionsSync: 5,
		Timestamp:    time.Now(),
		SequenceNum:  123,
	}

	assert.Equal(t, "sync-1", response.ID)
	assert.True(t, response.Success)
	assert.Equal(t, "sync completed", response.Message)
	assert.Equal(t, 5, response.SessionsSync)
	assert.Equal(t, uint64(123), response.SequenceNum)
}

func TestSyncMetrics_Fields(t *testing.T) {
	metrics := &SyncMetrics{
		TotalSyncs:       10,
		SuccessfulSyncs:  8,
		FailedSyncs:      2,
		TotalBytes:       1024 * 1024,
		AverageDuration:  100 * time.Millisecond,
		LastSyncTime:     time.Now(),
		CompressionRatio: 0.75,
	}

	assert.Equal(t, int64(10), metrics.TotalSyncs)
	assert.Equal(t, int64(8), metrics.SuccessfulSyncs)
	assert.Equal(t, int64(2), metrics.FailedSyncs)
	assert.Equal(t, int64(1024*1024), metrics.TotalBytes)
	assert.Equal(t, 100*time.Millisecond, metrics.AverageDuration)
	assert.Equal(t, 0.75, metrics.CompressionRatio)
}
