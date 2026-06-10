package smb

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// StateSyncConfig 状态同步配置
type StateSyncConfig struct {
	Enabled            bool   `json:"enabled"`
	SyncIntervalMs     int    `json:"sync_interval_ms"`      // 同步间隔(毫秒)
	BatchSize          int    `json:"batch_size"`             // 批量同步大小
	MaxConcurrentSyncs int    `json:"max_concurrent_syncs"`   // 最大并发同步数
	CompressionEnabled bool   `json:"compression_enabled"`    // 是否启用压缩
	CompressionLevel   int    `json:"compression_level"`      // 压缩级别 (1-9)
	RetryAttempts      int    `json:"retry_attempts"`         // 重试次数
	RetryDelayMs       int    `json:"retry_delay_ms"`         // 重试延迟(毫秒)
	TimeoutMs          int    `json:"timeout_ms"`             // 同步超时(毫秒)
	FullSyncIntervalMs int    `json:"full_sync_interval_ms"`  // 全量同步间隔(毫秒)
	Endpoint           string `json:"endpoint"`               // 同步端点URL
}

// DefaultStateSyncConfig 返回默认状态同步配置
func DefaultStateSyncConfig() *StateSyncConfig {
	return &StateSyncConfig{
		Enabled:            true,
		SyncIntervalMs:     5000,
		BatchSize:          100,
		MaxConcurrentSyncs: 5,
		CompressionEnabled: true,
		CompressionLevel:   gzip.DefaultCompression,
		RetryAttempts:      3,
		RetryDelayMs:       1000,
		TimeoutMs:          30000,
		FullSyncIntervalMs: 60000,
	}
}

// SyncType 同步类型
type SyncType string

const (
	SyncTypeFull        SyncType = "full"        // 全量同步
	SyncTypeIncremental SyncType = "incremental" // 增量同步
	SyncTypeDelta       SyncType = "delta"       // 差异同步
)

// SyncState 同步状态
type SyncState string

const (
	SyncStatePending    SyncState = "pending"
	SyncStateInProgress SyncState = "in_progress"
	SyncStateCompleted  SyncState = "completed"
	SyncStateFailed     SyncState = "failed"
)

// SyncRequest 同步请求
type SyncRequest struct {
	ID          string            `json:"id"`
	Type        SyncType          `json:"type"`
	SourceNode  string            `json:"source_node"`
	TargetNode  string            `json:"target_node"`
	Sessions    map[string][]byte `json:"sessions,omitempty"`
	SequenceNum uint64            `json:"sequence_num"`
	Timestamp   time.Time         `json:"timestamp"`
}

// SyncResponse 同步响应
type SyncResponse struct {
	ID           string    `json:"id"`
	Success      bool      `json:"success"`
	Message      string    `json:"message,omitempty"`
	SessionsSync int       `json:"sessions_synced"`
	Timestamp    time.Time `json:"timestamp"`
	SequenceNum  uint64    `json:"sequence_num"`
}

// SyncEndpoint 同步端点信息
type SyncEndpoint struct {
	mu          sync.RWMutex
	NodeID      string        `json:"node_id"`
	Hostname    string        `json:"hostname"`
	Address     string        `json:"address"`
	Port        int           `json:"port"`
	Connected   bool          `json:"connected"`
	LastSync    time.Time     `json:"last_sync"`
	SyncLag     time.Duration `json:"sync_lag"`
	SequenceNum uint64        `json:"sequence_num"`
	Failures    int           `json:"failures"`
}

// SyncOperation 同步操作
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

// SyncMetrics 同步指标
type SyncMetrics struct {
	mu               sync.RWMutex
	TotalSyncs       int64         `json:"total_syncs"`
	SuccessfulSyncs  int64         `json:"successful_syncs"`
	FailedSyncs      int64         `json:"failed_syncs"`
	TotalBytes       int64         `json:"total_bytes"`
	AverageDuration  time.Duration `json:"average_duration"`
	LastSyncTime     time.Time     `json:"last_sync_time"`
	CompressionRatio float64       `json:"compression_ratio"`
}

// SyncNodeStatus 节点同步状态
type SyncNodeStatus struct {
	NodeID      string        `json:"node_id"`
	Hostname    string        `json:"hostname"`
	Address     string        `json:"address"`
	Port        int           `json:"port"`
	Connected   bool          `json:"connected"`
	LastSync    time.Time     `json:"last_sync"`
	SyncLag     time.Duration `json:"sync_lag"`
	SequenceNum uint64        `json:"sequence_num"`
	Failures    int           `json:"failures"`
}

// StateSynchronizer 状态同步器
type StateSynchronizer struct {
	mu              sync.RWMutex
	config          *StateSyncConfig
	nodes           map[string]*SyncEndpoint
	localNodeID     string
	syncQueue       chan SyncRequest
	activeSyncs     map[string]*SyncOperation
	lastSync        time.Time
	syncMetrics     SyncMetrics
	compressionPool sync.Pool
	sessionRegistry *SessionRegistry
	running         bool
	stopChan        chan struct{}
}

// NewStateSynchronizer 创建状态同步器
func NewStateSynchronizer(config *StateSyncConfig, registry *SessionRegistry) *StateSynchronizer {
	if config == nil {
		config = DefaultStateSyncConfig()
	}

	ss := &StateSynchronizer{
		config:          config,
		nodes:           make(map[string]*SyncEndpoint),
		syncQueue:       make(chan SyncRequest, 1000),
		activeSyncs:     make(map[string]*SyncOperation),
		sessionRegistry: registry,
		stopChan:        make(chan struct{}),
		compressionPool: sync.Pool{
			New: func() interface{} {
				w, _ := gzip.NewWriterLevel(nil, config.CompressionLevel)
				return w
			},
		},
	}

	return ss
}

// SetLocalNode 设置本地节点ID
func (ss *StateSynchronizer) SetLocalNode(nodeID string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.localNodeID = nodeID
}

// AddNode 添加同步节点
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

	logInfo("同步节点已添加", "node_id", nodeID, "address", fmt.Sprintf("%s:%d", address, port))
}

// RemoveNode 移除同步节点
func (ss *StateSynchronizer) RemoveNode(nodeID string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.nodes, nodeID)
	logInfo("同步节点已移除", "node_id", nodeID)
}

// Start 启动状态同步器
func (ss *StateSynchronizer) Start() error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.running {
		return fmt.Errorf("状态同步器已在运行")
	}

	// 启动同步工作线程
	for i := 0; i < ss.config.MaxConcurrentSyncs; i++ {
		go ss.syncWorker()
	}

	// 启动周期性同步
	go ss.periodicSync()

	ss.running = true
	logInfo("状态同步器已启动", "workers", ss.config.MaxConcurrentSyncs, "sync_interval_ms", ss.config.SyncIntervalMs)

	return nil
}

// Stop 停止状态同步器
func (ss *StateSynchronizer) Stop() {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if !ss.running {
		return
	}

	close(ss.stopChan)
	ss.running = false
	logInfo("状态同步器已停止")
}

// SyncSessions 同步会话到目标节点
func (ss *StateSynchronizer) SyncSessions(ctx context.Context, targetNodeID string, sessions map[string][]byte) error {
	ss.mu.RLock()
	endpoint, ok := ss.nodes[targetNodeID]
	ss.mu.RUnlock()

	if !ok {
		return fmt.Errorf("目标节点 %s 未注册", targetNodeID)
	}

	// 准备同步请求
	request := SyncRequest{
		ID:         fmt.Sprintf("sync-%d", time.Now().UnixNano()),
		Type:       SyncTypeFull,
		SourceNode: ss.localNodeID,
		TargetNode: targetNodeID,
		Sessions:   sessions,
		Timestamp:  time.Now(),
	}

	// 压缩数据
	if ss.config.CompressionEnabled {
		compressed, err := ss.compressSessions(sessions)
		if err != nil {
			return fmt.Errorf("压缩失败: %w", err)
		}
		request.Sessions = compressed
	}

	// 执行同步
	return ss.executeSync(ctx, endpoint, request)
}

// SyncIncremental 同步增量变更
func (ss *StateSynchronizer) SyncIncremental(ctx context.Context, targetNodeID string, changes map[string][]byte) error {
	ss.mu.RLock()
	endpoint, ok := ss.nodes[targetNodeID]
	ss.mu.RUnlock()

	if !ok {
		return fmt.Errorf("目标节点 %s 未注册", targetNodeID)
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

// executeSync 执行同步请求
func (ss *StateSynchronizer) executeSync(ctx context.Context, endpoint *SyncEndpoint, request SyncRequest) error {
	// 创建同步操作
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

	// 重试逻辑
	var lastErr error
	for attempt := 0; attempt <= ss.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(ss.config.RetryDelayMs) * time.Millisecond)
			logInfo("重试同步", "operation_id", request.ID, "attempt", attempt)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 发送同步请求
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

			logInfo("同步完成", "operation_id", request.ID, "sessions_synced", response.SessionsSync)

			return nil
		}

		lastErr = fmt.Errorf("同步失败: %s", response.Message)
	}

	operation.State = SyncStateFailed
	operation.EndTime = time.Now()
	operation.Error = lastErr

	ss.updateMetrics(false, operation.EndTime.Sub(operation.StartTime), 0)

	return lastErr
}

// sendSyncRequest 发送同步请求到端点
func (ss *StateSynchronizer) sendSyncRequest(ctx context.Context, endpoint *SyncEndpoint, request SyncRequest) (*SyncResponse, error) {
	// 序列化请求
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 发送HTTP请求
	url := fmt.Sprintf("http://%s:%d/api/v1/smb/failover/sync", endpoint.Address, endpoint.Port)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: time.Duration(ss.config.TimeoutMs) * time.Millisecond,
	}

	resp, err := client.Do(req)
	if err != nil {
		endpoint.mu.Lock()
		endpoint.Failures++
		endpoint.mu.Unlock()
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("同步请求失败，状态码: %d", resp.StatusCode)
	}

	var syncResp SyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	endpoint.mu.Lock()
	endpoint.LastSync = time.Now()
	endpoint.SequenceNum = request.SequenceNum
	endpoint.Failures = 0
	endpoint.mu.Unlock()

	logInfo("同步请求已发送", "endpoint", endpoint.NodeID, "bytes", len(data), "sessions", len(request.Sessions))

	return &syncResp, nil
}

// HandleSyncRequest 处理接收到的同步请求
func (ss *StateSynchronizer) HandleSyncRequest(request SyncRequest) (*SyncResponse, error) {
	logInfo("处理同步请求", "operation_id", request.ID, "source", request.SourceNode, "type", string(request.Type))

	// 解压数据
	sessions := request.Sessions
	if ss.config.CompressionEnabled {
		decompressed, err := ss.decompressSessions(sessions)
		if err != nil {
			return nil, fmt.Errorf("解压失败: %w", err)
		}
		sessions = decompressed
	}

	// 恢复会话
	restored := 0
	for sessionID, sessionData := range sessions {
		var session SMBSession
		if err := json.Unmarshal(sessionData, &session); err != nil {
			logError("反序列化会话失败", err, "session_id", sessionID)
			continue
		}
		ss.sessionRegistry.Add(&session)
		restored++
	}

	return &SyncResponse{
		ID:           request.ID,
		Success:      true,
		SessionsSync: restored,
		Timestamp:    time.Now(),
	}, nil
}

// compressSessions 压缩会话数据
func (ss *StateSynchronizer) compressSessions(sessions map[string][]byte) (map[string][]byte, error) {
	result := make(map[string][]byte, len(sessions))

	for id, data := range sessions {
		var buf bytes.Buffer
		writer := ss.compressionPool.Get().(*gzip.Writer)
		writer.Reset(&buf)

		if _, err := writer.Write(data); err != nil {
			writer.Close()
			return nil, fmt.Errorf("压缩会话 %s 失败: %w", id, err)
		}

		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("压缩刷新失败: %w", err)
		}

		result[id] = buf.Bytes()
		ss.compressionPool.Put(writer)
	}

	return result, nil
}

// decompressSessions 解压会话数据
func (ss *StateSynchronizer) decompressSessions(sessions map[string][]byte) (map[string][]byte, error) {
	result := make(map[string][]byte, len(sessions))

	for id, data := range sessions {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("解压会话 %s 失败: %w", id, err)
		}

		decompressed, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return nil, fmt.Errorf("读取解压数据失败: %w", err)
		}

		result[id] = decompressed
	}

	return result, nil
}

// periodicSync 周期性同步
func (ss *StateSynchronizer) periodicSync() {
	interval := time.Duration(ss.config.SyncIntervalMs) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ss.stopChan:
			return
		case <-ticker.C:
			ss.syncAllNodes()
		}
	}
}

// syncAllNodes 同步到所有节点
func (ss *StateSynchronizer) syncAllNodes() {
	ss.mu.RLock()
	nodes := make([]*SyncEndpoint, 0, len(ss.nodes))
	for _, node := range ss.nodes {
		if node.NodeID != ss.localNodeID {
			nodes = append(nodes, node)
		}
	}
	ss.mu.RUnlock()

	// 获取当前会话
	sessions := ss.sessionRegistry.ListAll()
	if len(sessions) == 0 {
		return
	}

	// 序列化会话
	sessionData := make(map[string][]byte, len(sessions))
	for _, session := range sessions {
		data, err := json.Marshal(session)
		if err != nil {
			logError("序列化会话失败", err, "session_id", session.SessionID)
			continue
		}
		sessionData[session.SessionID] = data
	}

	// 同步到每个节点
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ss.config.TimeoutMs)*time.Millisecond)
	defer cancel()

	for _, node := range nodes {
		if err := ss.SyncSessions(ctx, node.NodeID, sessionData); err != nil {
			logError("同步到节点失败", err, "node_id", node.NodeID)
		}
	}

	ss.mu.Lock()
	ss.lastSync = time.Now()
	ss.mu.Unlock()
}

// syncWorker 同步工作线程
func (ss *StateSynchronizer) syncWorker() {
	for {
		select {
		case <-ss.stopChan:
			return
		case request := <-ss.syncQueue:
			ss.mu.RLock()
			endpoint, ok := ss.nodes[request.TargetNode]
			ss.mu.RUnlock()

			if !ok {
				logError("同步目标节点未找到", nil, "operation_id", request.ID, "target", request.TargetNode)
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ss.config.TimeoutMs)*time.Millisecond)
			if err := ss.executeSync(ctx, endpoint, request); err != nil {
				logError("异步同步失败", err, "operation_id", request.ID)
			}
			cancel()
		}
	}
}

// QueueSync 排队同步请求
func (ss *StateSynchronizer) QueueSync(request SyncRequest) {
	select {
	case ss.syncQueue <- request:
		logInfo("同步已排队", "operation_id", request.ID)
	default:
		logInfo("同步队列已满，丢弃请求", "operation_id", request.ID)
	}
}

// updateMetrics 更新同步指标
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
			int64(ss.syncMetrics.AverageDuration)*(ss.syncMetrics.TotalSyncs-1)+int64(duration),
		) / time.Duration(ss.syncMetrics.TotalSyncs)
	}
}

// GetSyncMetrics 返回同步指标
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

// GetActiveSyncs 返回当前活跃的同步操作
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

// GetNodeSyncStatus 返回所有节点的同步状态
func (ss *StateSynchronizer) GetNodeSyncStatus() map[string]*SyncNodeStatus {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	result := make(map[string]*SyncNodeStatus, len(ss.nodes))
	for id, node := range ss.nodes {
		node.mu.RLock()
		result[id] = &SyncNodeStatus{
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
	}
	return result
}

// IsNodeInSync 检查节点是否已同步
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

// GetLastSyncTime 返回最后同步时间
func (ss *StateSynchronizer) GetLastSyncTime() time.Time {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.lastSync
}

// IsRunning 检查同步器是否在运行
func (ss *StateSynchronizer) IsRunning() bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.running
}
