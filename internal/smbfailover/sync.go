package smbfailover

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StateSynchronizer synchronizes session state between nodes
type StateSynchronizer struct {
	mu              sync.RWMutex
	config          SyncConfig
	logger          *zap.Logger
	nodes           map[string]*SyncEndpoint
	localNodeID     string
	syncQueue       chan SyncRequest
	activeSyncs     map[string]*SyncOperation
	lastSync        time.Time
	syncMetrics     SyncMetrics
	compressionPool sync.Pool
}

// SyncConfig configures state synchronization
type SyncConfig struct {
	SyncInterval       time.Duration `json:"sync_interval"`
	BatchSize          int           `json:"batch_size"`
	MaxConcurrentSyncs int           `json:"max_concurrent_syncs"`
	CompressionEnabled bool          `json:"compression_enabled"`
	CompressionLevel   int           `json:"compression_level"`
	RetryAttempts      int           `json:"retry_attempts"`
	RetryDelay         time.Duration `json:"retry_delay"`
	Timeout            time.Duration `json:"timeout"`
	FullSyncInterval   time.Duration `json:"full_sync_interval"`
}

// DefaultSyncConfig returns sensible defaults
func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		SyncInterval:       5 * time.Second,
		BatchSize:          100,
		MaxConcurrentSyncs: 5,
		CompressionEnabled: true,
		CompressionLevel:   gzip.DefaultCompression,
		RetryAttempts:      3,
		RetryDelay:         1 * time.Second,
		Timeout:            30 * time.Second,
		FullSyncInterval:   1 * time.Minute,
	}
}

// SyncEndpoint represents a sync endpoint
type SyncEndpoint struct {
	mu           sync.RWMutex
	NodeID       string    `json:"node_id"`
	Hostname     string    `json:"hostname"`
	Address      string    `json:"address"`
	Port         int       `json:"port"`
	Connected    bool      `json:"connected"`
	LastSync     time.Time `json:"last_sync"`
	SyncLag      time.Duration `json:"sync_lag"`
	SequenceNum  uint64    `json:"sequence_num"`
	Failures     int       `json:"failures"`
}

// SyncRequest represents a sync request
type SyncRequest struct {
	ID          string            `json:"id"`
	Type        SyncType          `json:"type"`
	SourceNode  string            `json:"source_node"`
	TargetNode  string            `json:"target_node"`
	Sessions    map[string][]byte `json:"sessions,omitempty"`
	SequenceNum uint64            `json:"sequence_num"`
	Timestamp   time.Time         `json:"timestamp"`
}

// SyncType defines the type of sync
type SyncType string

const (
	SyncTypeFull        SyncType = "full"
	SyncTypeIncremental SyncType = "incremental"
	SyncTypeDelta       SyncType = "delta"
)

// SyncOperation represents an ongoing sync operation
type SyncOperation struct {
	mu        sync.RWMutex
	ID        string      `json:"id"`
	Request   SyncRequest `json:"request"`
	State     SyncState   `json:"state"`
	StartTime time.Time   `json:"start_time"`
	EndTime   time.Time   `json:"end_time,omitempty"`
	Progress  float64     `json:"progress"`
	Error     error       `json:"error,omitempty"`
	BytesSent int64       `json:"bytes_sent"`
}

// SyncState represents sync operation state
type SyncState string

const (
	SyncStatePending    SyncState = "pending"
	SyncStateInProgress SyncState = "in_progress"
	SyncStateCompleted  SyncState = "completed"
	SyncStateFailed     SyncState = "failed"
)

// SyncResponse represents a sync response
type SyncResponse struct {
	ID          string    `json:"id"`
	Success     bool      `json:"success"`
	Message     string    `json:"message,omitempty"`
	SessionsSync int     `json:"sessions_synced"`
	Timestamp   time.Time `json:"timestamp"`
	SequenceNum uint64    `json:"sequence_num"`
}

// SyncMetrics tracks synchronization metrics
type SyncMetrics struct {
	mu              sync.RWMutex
	TotalSyncs      int64         `json:"total_syncs"`
	SuccessfulSyncs int64         `json:"successful_syncs"`
	FailedSyncs     int64         `json:"failed_syncs"`
	TotalBytes      int64         `json:"total_bytes"`
	AverageDuration time.Duration `json:"average_duration"`
	LastSyncTime    time.Time     `json:"last_sync_time"`
	CompressionRatio float64      `json:"compression_ratio"`
}

// NewStateSynchronizer creates a new state synchronizer
func NewStateSynchronizer(config SyncConfig, logger *zap.Logger) *StateSynchronizer {
	ss := &StateSynchronizer{
		config:      config,
		logger:      logger,
		nodes:       make(map[string]*SyncEndpoint),
		syncQueue:   make(chan SyncRequest, 1000),
		activeSyncs: make(map[string]*SyncOperation),
		compressionPool: sync.Pool{
			New: func() interface{} {
				w, _ := gzip.NewWriterLevel(nil, config.CompressionLevel)
				return w
			},
		},
	}

	return ss
}

// SetLocalNode sets the local node ID
func (ss *StateSynchronizer) SetLocalNode(nodeID string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.localNodeID = nodeID
}

// AddNode adds a node for synchronization
func (ss *StateSynchronizer) AddNode(nodeID, hostname, address string, port int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.nodes[nodeID] = &SyncEndpoint{
		NodeID:    nodeID,
		Hostname:  hostname,
		Address:   address,
		Port:      port,
		Connected: true,
	}

	ss.logger.Info("sync node added",
		zap.String("node_id", nodeID),
		zap.String("address", fmt.Sprintf("%s:%d", address, port)))
}

// RemoveNode removes a node from synchronization
func (ss *StateSynchronizer) RemoveNode(nodeID string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.nodes, nodeID)
	ss.logger.Info("sync node removed", zap.String("node_id", nodeID))
}

// Start starts the state synchronizer
func (ss *StateSynchronizer) Start() error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Start sync workers
	for i := 0; i < ss.config.MaxConcurrentSyncs; i++ {
		go ss.syncWorker()
	}

	// Start periodic sync
	go ss.periodicSync()

	ss.logger.Info("state synchronizer started",
		zap.Int("workers", ss.config.MaxConcurrentSyncs),
		zap.Duration("sync_interval", ss.config.SyncInterval))

	return nil
}

// Stop stops the state synchronizer
func (ss *StateSynchronizer) Stop() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	close(ss.syncQueue)
	ss.logger.Info("state synchronizer stopped")
}

// SyncSessions synchronizes sessions to a target node
func (ss *StateSynchronizer) SyncSessions(ctx context.Context, targetNodeID string, sessions map[string][]byte) error {
	ss.mu.RLock()
	endpoint, ok := ss.nodes[targetNodeID]
	ss.mu.RUnlock()

	if !ok {
		return fmt.Errorf("target node %s not registered", targetNodeID)
	}

	// Prepare sync request
	request := SyncRequest{
		ID:         fmt.Sprintf("sync-%d", time.Now().UnixNano()),
		Type:       SyncTypeFull,
		SourceNode: ss.localNodeID,
		TargetNode: targetNodeID,
		Sessions:   sessions,
		Timestamp:  time.Now(),
	}

	// Compress if enabled
	if ss.config.CompressionEnabled {
		compressed, err := ss.compressSessions(sessions)
		if err != nil {
			return fmt.Errorf("compression failed: %w", err)
		}
		request.Sessions = compressed
	}

	// Execute sync
	return ss.executeSync(ctx, endpoint, request)
}

// SyncIncremental synchronizes incremental changes
func (ss *StateSynchronizer) SyncIncremental(ctx context.Context, targetNodeID string, changes map[string][]byte) error {
	ss.mu.RLock()
	endpoint, ok := ss.nodes[targetNodeID]
	ss.mu.RUnlock()

	if !ok {
		return fmt.Errorf("target node %s not registered", targetNodeID)
	}

	request := SyncRequest{
		ID:         fmt.Sprintf("sync-inc-%d", time.Now().UnixNano()),
		Type:       SyncTypeIncremental,
		SourceNode: ss.localNodeID,
		TargetNode: targetNodeID,
		Sessions:   changes,
		Timestamp:  time.Now(),
	}

	return ss.executeSync(ctx, endpoint, request)
}

// executeSync executes a sync request
func (ss *StateSynchronizer) executeSync(ctx context.Context, endpoint *SyncEndpoint, request SyncRequest) error {
	// Create sync operation
	operation := &SyncOperation{
		ID:        request.ID,
		Request:   request,
		State:     SyncStateInProgress,
		StartTime: time.Now(),
	}

	ss.mu.Lock()
	ss.activeSyncs[request.ID] = operation
	ss.mu.Unlock()

	defer func() {
		ss.mu.Lock()
		delete(ss.activeSyncs, request.ID)
		ss.mu.Unlock()
	}()

	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= ss.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(ss.config.RetryDelay)
			ss.logger.Info("retrying sync",
				zap.String("operation_id", request.ID),
				zap.Int("attempt", attempt))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Send sync request
		response, err := ss.sendSyncRequest(ctx, endpoint, request)
		if err != nil {
			lastErr = err
			continue
		}

		if response.Success {
			operation.State = SyncStateCompleted
			operation.EndTime = time.Now()
			operation.Progress = 1.0

			ss.updateMetrics(true, operation.EndTime.Sub(operation.StartTime), int64(len(request.Sessions)))

			ss.logger.Info("sync completed",
				zap.String("operation_id", request.ID),
				zap.Int("sessions_synced", response.SessionsSync))

			return nil
		}

		lastErr = fmt.Errorf("sync failed: %s", response.Message)
	}

	operation.State = SyncStateFailed
	operation.EndTime = time.Now()
	operation.Error = lastErr

	ss.updateMetrics(false, operation.EndTime.Sub(operation.StartTime), 0)

	return lastErr
}

// sendSyncRequest sends a sync request to an endpoint
func (ss *StateSynchronizer) sendSyncRequest(ctx context.Context, endpoint *SyncEndpoint, request SyncRequest) (*SyncResponse, error) {
	// Serialize request
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// In production, this would send over gRPC or HTTP
	// For now, simulate the sync
	response := &SyncResponse{
		ID:           request.ID,
		Success:      true,
		SessionsSync: len(request.Sessions),
		Timestamp:    time.Now(),
	}

	endpoint.mu.Lock()
	endpoint.LastSync = time.Now()
	endpoint.SequenceNum = request.SequenceNum
	endpoint.mu.Unlock()

	ss.logger.Debug("sync request sent",
		zap.String("endpoint", endpoint.NodeID),
		zap.Int("bytes", len(data)),
		zap.Int("sessions", len(request.Sessions)))

	return response, nil
}

// compressSessions compresses session data
func (ss *StateSynchronizer) compressSessions(sessions map[string][]byte) (map[string][]byte, error) {
	result := make(map[string][]byte, len(sessions))

	for id, data := range sessions {
		var buf bytes.Buffer
		writer := ss.compressionPool.Get().(*gzip.Writer)
		writer.Reset(&buf)

		if _, err := writer.Write(data); err != nil {
			writer.Close()
			return nil, fmt.Errorf("compression failed for session %s: %w", id, err)
		}

		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("compression flush failed: %w", err)
		}

		result[id] = buf.Bytes()
		ss.compressionPool.Put(writer)
	}

	return result, nil
}

// decompressSessions decompresses session data
func (ss *StateSynchronizer) decompressSessions(sessions map[string][]byte) (map[string][]byte, error) {
	result := make(map[string][]byte, len(sessions))

	for id, data := range sessions {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decompression failed for session %s: %w", id, err)
		}
		defer reader.Close()

		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("decompression read failed: %w", err)
		}

		result[id] = decompressed
	}

	return result, nil
}

// HandleSyncRequest handles an incoming sync request
func (ss *StateSynchronizer) HandleSyncRequest(request SyncRequest) (*SyncResponse, error) {
	ss.logger.Info("handling sync request",
		zap.String("operation_id", request.ID),
		zap.String("source", request.SourceNode),
		zap.String("type", string(request.Type)))

	// Decompress if needed
	sessions := request.Sessions
	if ss.config.CompressionEnabled {
		decompressed, err := ss.decompressSessions(sessions)
		if err != nil {
			return nil, fmt.Errorf("decompression failed: %w", err)
		}
		sessions = decompressed
	}

	// In production, this would restore sessions
	// For now, acknowledge receipt
	return &SyncResponse{
		ID:           request.ID,
		Success:      true,
		SessionsSync: len(sessions),
		Timestamp:    time.Now(),
	}, nil
}

// periodicSync performs periodic synchronization
func (ss *StateSynchronizer) periodicSync() {
	ticker := time.NewTicker(ss.config.SyncInterval)
	defer ticker.Stop()

	for range ticker.C {
		ss.syncAllNodes()
	}
}

// syncAllNodes synchronizes with all nodes
func (ss *StateSynchronizer) syncAllNodes() {
	ss.mu.RLock()
	nodes := make([]*SyncEndpoint, 0, len(ss.nodes))
	for _, node := range ss.nodes {
		if node.NodeID != ss.localNodeID {
			nodes = append(nodes, node)
		}
	}
	ss.mu.RUnlock()

	for _, node := range nodes {
		if err := ss.syncWithNode(node); err != nil {
			ss.logger.Error("sync failed",
				zap.String("node", node.NodeID),
				zap.Error(err))
		}
	}
}

// syncWithNode synchronizes with a specific node
func (ss *StateSynchronizer) syncWithNode(endpoint *SyncEndpoint) error {
	// In production, this would fetch and sync delta changes
	ss.logger.Debug("periodic sync", zap.String("node", endpoint.NodeID))

	endpoint.mu.Lock()
	endpoint.LastSync = time.Now()
	endpoint.mu.Unlock()

	return nil
}

// syncWorker processes sync requests
func (ss *StateSynchronizer) syncWorker() {
	for request := range ss.syncQueue {
		ss.mu.RLock()
		endpoint, ok := ss.nodes[request.TargetNode]
		ss.mu.RUnlock()

		if !ok {
			ss.logger.Error("target node not found for sync",
				zap.String("operation_id", request.ID),
				zap.String("target", request.TargetNode))
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), ss.config.Timeout)
		if err := ss.executeSync(ctx, endpoint, request); err != nil {
			ss.logger.Error("async sync failed",
				zap.String("operation_id", request.ID),
				zap.Error(err))
		}
		cancel()
	}
}

// QueueSync queues a sync request
func (ss *StateSynchronizer) QueueSync(request SyncRequest) {
	select {
	case ss.syncQueue <- request:
		ss.logger.Debug("sync queued", zap.String("operation_id", request.ID))
	default:
		ss.logger.Warn("sync queue full, dropping request",
			zap.String("operation_id", request.ID))
	}
}

// updateMetrics updates sync metrics
func (ss *StateSynchronizer) updateMetrics(successful bool, duration time.Duration, bytes int64) {
	ss.syncMetrics.mu.Lock()
	defer ss.syncMetrics.mu.Unlock()

	ss.syncMetrics.TotalSyncs++
	if successful {
		ss.syncMetrics.SuccessfulSyncs++
	} else {
		ss.syncMetrics.FailedSyncs++
	}

	ss.syncMetrics.TotalBytes += bytes
	ss.syncMetrics.LastSyncTime = time.Now()

	if ss.syncMetrics.TotalSyncs > 0 {
		ss.syncMetrics.AverageDuration = time.Duration(
			int64(ss.syncMetrics.AverageDuration)*(ss.syncMetrics.TotalSyncs-1) + int64(duration),
		) / time.Duration(ss.syncMetrics.TotalSyncs)
	}
}

// GetSyncMetrics returns synchronization metrics
func (ss *StateSynchronizer) GetSyncMetrics() *SyncMetrics {
	ss.syncMetrics.mu.RLock()
	defer ss.syncMetrics.mu.RUnlock()
	metrics := &SyncMetrics{
		TotalSyncs:       ss.syncMetrics.TotalSyncs,
		SuccessfulSyncs:  ss.syncMetrics.SuccessfulSyncs,
		FailedSyncs:      ss.syncMetrics.FailedSyncs,
		TotalBytes:       ss.syncMetrics.TotalBytes,
		AverageDuration:  ss.syncMetrics.AverageDuration,
		LastSyncTime:     ss.syncMetrics.LastSyncTime,
		CompressionRatio: ss.syncMetrics.CompressionRatio,
	}
	return metrics
}

// GetActiveSyncs returns currently active sync operations
func (ss *StateSynchronizer) GetActiveSyncs() map[string]*SyncOperation {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	result := make(map[string]*SyncOperation, len(ss.activeSyncs))
	for id, op := range ss.activeSyncs {
		op.mu.RLock()
		copy := &SyncOperation{
			ID:        op.ID,
			Request:   op.Request,
			State:     op.State,
			StartTime: op.StartTime,
			EndTime:   op.EndTime,
			Progress:  op.Progress,
			Error:     op.Error,
			BytesSent: op.BytesSent,
		}
		op.mu.RUnlock()
		result[id] = copy
	}
	return result
}

// GetNodeSyncStatus returns sync status for all nodes
func (ss *StateSynchronizer) GetNodeSyncStatus() map[string]*SyncEndpoint {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	result := make(map[string]*SyncEndpoint, len(ss.nodes))
	for id, node := range ss.nodes {
		node.mu.RLock()
		copy := &SyncEndpoint{
			NodeID:      node.NodeID,
			Hostname:    node.Hostname,
			Address:     node.Address,
			Port:        node.Port,
			Connected:   node.Connected,
			LastSync:    node.LastSync,
			SyncLag:     node.SyncLag,
			SequenceNum: node.SequenceNum,
			Failures:    node.Failures,
		}
		node.mu.RUnlock()
		result[id] = copy
	}
	return result
}

// IsNodeInSync returns true if a node is in sync
func (ss *StateSynchronizer) IsNodeInSync(nodeID string, maxLag time.Duration) bool {
	ss.mu.RLock()
	node, ok := ss.nodes[nodeID]
	ss.mu.RUnlock()

	if !ok {
		return false
	}

	node.mu.RLock()
	defer node.mu.RUnlock()

	return node.Connected && node.SyncLag <= maxLag
}
