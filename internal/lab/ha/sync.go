// Package ha 状态同步器
// 实现节点间状态同步，包括 SMB 配置、会话状态、共享状态等
// 参考 TrueNAS SMB Stateful Failover 的状态同步机制
package ha

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StateSyncer 状态同步器.
type StateSyncer struct {
	config     *HAConfig
	syncQueue  []SyncTask
	syncStatus map[string]*SyncStatus
	progress   float64
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	logger     *zap.Logger
}

// SyncTask 同步任务.
type SyncTask struct {
	ID          string    `json:"id"`
	Type        SyncType  `json:"type"`
	Source      string    `json:"source"`
	Target      string    `json:"target"`
	Data        []byte    `json:"data,omitempty"`
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	RetryCount  int       `json:"retry_count"`
}

// SyncType 同步类型.
type SyncType string

const (
	SyncTypeSMBConfig   SyncType = "smb_config"   // SMB 配置
	SyncTypeSMBSession  SyncType = "smb_session"  // SMB 会话状态
	SyncTypeSMBShare    SyncType = "smb_share"    // SMB 共享状态
	SyncTypeSMBLock     SyncType = "smb_lock"     // SMB 锁定状态
	SyncTypeUserDB      SyncType = "user_db"      // 用户数据库
	SyncTypeVolumeState SyncType = "volume_state" // 卷状态
	SyncTypeFullSync    SyncType = "full_sync"    // 全量同步
)

// SyncStatus 同步状态.
type SyncStatus struct {
	NodeID          string        `json:"node_id"`
	LastSync        time.Time     `json:"last_sync"`
	LastSyncType    SyncType      `json:"last_sync_type"`
	TotalSyncs      int           `json:"total_syncs"`
	FailedSyncs     int           `json:"failed_syncs"`
	SuccessRate     float64       `json:"success_rate"`
	AvgSyncDuration time.Duration `json:"avg_sync_duration"`
	PendingTasks    int           `json:"pending_tasks"`
	IsSyncing       bool          `json:"is_syncing"`
}

// SMBState SMB状态数据
// 参考 TrueNAS SMB Stateful Failover 的状态结构.
type SMBState struct {
	Config         *SMBConfigState  `json:"config"`
	Sessions       []*SMBSession    `json:"sessions"`
	Shares         []*SMBShareState `json:"shares"`
	Locks          []*SMBLockState  `json:"locks"`
	OpenFiles      []*SMBOpenFile   `json:"open_files"`
	ConnectedUsers []string         `json:"connected_users"`
	ServiceStatus  string           `json:"service_status"`
	LastUpdate     time.Time        `json:"last_update"`
}

// SMBConfigState SMB配置状态.
type SMBConfigState struct {
	Workgroup    string            `json:"workgroup"`
	ServerString string            `json:"server_string"`
	MinProtocol  string            `json:"min_protocol"`
	MaxProtocol  string            `json:"max_protocol"`
	GlobalConfig map[string]string `json:"global_config"`
	ShareConfigs map[string]string `json:"share_configs"`
}

// SMBSession SMB会话状态
// 参考 TrueNAS 的会话保持机制.
type SMBSession struct {
	SessionID       string    `json:"session_id"`
	Username        string    `json:"username"`
	ClientIP        string    `json:"client_ip"`
	ClientName      string    `json:"client_name,omitempty"`
	ConnectedAt     time.Time `json:"connected_at"`
	Protocol        string    `json:"protocol"`
	Encryption      string    `json:"encryption,omitempty"`
	Shares          []string  `json:"shares"`
	OpenFiles       int       `json:"open_files"`
	SessionKey      []byte    `json:"session_key,omitempty"`
	ServerChallenge []byte    `json:"server_challenge,omitempty"`
}

// SMBShareState SMB共享状态.
type SMBShareState struct {
	Name              string    `json:"name"`
	Path              string    `json:"path"`
	ActiveConnections int       `json:"active_connections"`
	OpenFiles         int       `json:"open_files"`
	LastAccess        time.Time `json:"last_access"`
	Status            string    `json:"status"`
}

// SMBLockState SMB锁定状态.
type SMBLockState struct {
	FileID       string    `json:"file_id"`
	ShareName    string    `json:"share_name"`
	Path         string    `json:"path"`
	OwnerPID     int       `json:"owner_pid"`
	OwnerSession string    `json:"owner_session"`
	LockType     string    `json:"lock_type"`
	StartTime    time.Time `json:"start_time"`
	ExpiryTime   time.Time `json:"expiry_time,omitempty"`
}

// SMBOpenFile SMB打开文件状态.
type SMBOpenFile struct {
	FileID       string    `json:"file_id"`
	ShareName    string    `json:"share_name"`
	Path         string    `json:"path"`
	OwnerPID     int       `json:"owner_pid"`
	OwnerSession string    `json:"owner_session"`
	OpenMode     string    `json:"open_mode"`
	OpenTime     time.Time `json:"open_time"`
	LockCount    int       `json:"lock_count"`
}

// NewStateSyncer 创建状态同步器.
func NewStateSyncer(config *HAConfig, logger *zap.Logger) *StateSyncer {
	ctx, cancel := context.WithCancel(context.Background())

	return &StateSyncer{
		config:     config,
		syncQueue:  make([]SyncTask, 0, 100),
		syncStatus: make(map[string]*SyncStatus),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
	}
}

// Start 启动状态同步器.
func (ss *StateSyncer) Start(ctx context.Context) error {
	ss.ctx, ss.cancel = context.WithCancel(ctx)

	// 初始化节点同步状态
	for _, peer := range ss.config.Peers {
		ss.syncStatus[peer.ID] = &SyncStatus{
			NodeID:      peer.ID,
			SuccessRate: 1.0,
			LastSync:    time.Now(),
		}
	}

	// 启动同步工作循环
	ss.wg.Add(1)
	go ss.syncWorkerLoop()

	// 启动状态收集循环
	ss.wg.Add(1)
	go ss.stateCollectLoop()

	ss.logger.Info("State syncer started",
		zap.Duration("interval", ss.config.SyncInterval),
	)

	return nil
}

// Stop 停止状态同步器.
func (ss *StateSyncer) Stop() {
	ss.cancel()
	ss.wg.Wait()
	ss.logger.Info("State syncer stopped")
}

// syncWorkerLoop 同步工作循环.
func (ss *StateSyncer) syncWorkerLoop() {
	defer ss.wg.Done()

	ticker := time.NewTicker(ss.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ss.ctx.Done():
			return
		case <-ticker.C:
			ss.processSyncQueue()
		}
	}
}

// stateCollectLoop 状态收集循环.
func (ss *StateSyncer) stateCollectLoop() {
	defer ss.wg.Done()

	ticker := time.NewTicker(ss.config.SyncInterval * 2)
	defer ticker.Stop()

	for {
		select {
		case <-ss.ctx.Done():
			return
		case <-ticker.C:
			// 收集本地 SMB 状态
			state, err := ss.collectLocalSMBState()
			if err != nil {
				ss.logger.Warn("Failed to collect SMB state", zap.Error(err))
				continue
			}

			// 添加同步任务
			ss.EnqueueSyncTask(SyncTask{
				ID:        fmt.Sprintf("sync-%d", time.Now().UnixNano()),
				Type:      SyncTypeSMBConfig,
				Source:    ss.config.NodeID,
				Priority:  1,
				CreatedAt: time.Now(),
				Status:    "pending",
			})

			// 保存状态到本地
			_ = ss.saveLocalState(state)
		}
	}
}

// processSyncQueue 处理同步队列.
func (ss *StateSyncer) processSyncQueue() {
	ss.mu.Lock()
	if len(ss.syncQueue) == 0 {
		ss.mu.Unlock()
		return
	}

	// 取出任务
	task := ss.syncQueue[0]
	ss.syncQueue = ss.syncQueue[1:]
	task.StartedAt = time.Now()
	task.Status = "running"
	ss.mu.Unlock()

	// 执行同步
	err := ss.executeSync(&task)

	// 更新状态
	ss.mu.Lock()
	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		task.RetryCount++

		// 重试逻辑
		if task.RetryCount < ss.config.SyncRetryMax {
			task.Status = "pending"
			ss.syncQueue = append(ss.syncQueue, task)
		}
	} else {
		task.Status = "completed"
		task.CompletedAt = time.Now()
	}

	// 更新进度
	ss.updateProgress()
	ss.mu.Unlock()

	ss.logger.Debug("Sync task completed",
		zap.String("task_id", task.ID),
		zap.String("type", string(task.Type)),
		zap.String("status", task.Status),
	)
}

// executeSync 执行同步.
func (ss *StateSyncer) executeSync(task *SyncTask) error {
	// 收集要同步的数据
	data, err := ss.collectSyncData(task.Type)
	if err != nil {
		return fmt.Errorf("collect data: %w", err)
	}

	task.Data = data

	// 发送到目标节点
	for _, peer := range ss.config.Peers {
		if err := ss.sendSyncData(peer, task); err != nil {
			ss.logger.Warn("Sync to peer failed",
				zap.String("peer", peer.ID),
				zap.Error(err),
			)

			// 更新节点同步状态
			ss.updateSyncStatus(peer.ID, false)

			continue
		}

		// 更新节点同步状态
		ss.updateSyncStatus(peer.ID, true)
	}

	return nil
}

// collectSyncData 收集同步数据.
func (ss *StateSyncer) collectSyncData(syncType SyncType) ([]byte, error) {
	switch syncType {
	case SyncTypeSMBConfig:
		return ss.collectSMBConfig()
	case SyncTypeSMBSession:
		return ss.collectSMBSessions()
	case SyncTypeSMBShare:
		return ss.collectSMBShares()
	case SyncTypeSMBLock:
		return ss.collectSMBLocks()
	case SyncTypeFullSync:
		return ss.collectFullState()
	default:
		return nil, errors.New("unknown sync type")
	}
}

// collectLocalSMBState 收集本地 SMB 状态.
func (ss *StateSyncer) collectLocalSMBState() (*SMBState, error) {
	state := &SMBState{
		LastUpdate: time.Now(),
	}

	// 收集配置
	config, err := ss.collectSMBConfig()
	if err == nil {
		var smbConfig SMBConfigState
		if json.Unmarshal(config, &smbConfig) == nil {
			state.Config = &smbConfig
		}
	}

	// 收集会话
	sessions, err := ss.collectSMBSessions()
	if err == nil {
		var smbSessions []*SMBSession
		if json.Unmarshal(sessions, &smbSessions) == nil {
			state.Sessions = smbSessions
		}
	}

	// 收集共享
	shares, err := ss.collectSMBShares()
	if err == nil {
		var smbShares []*SMBShareState
		if json.Unmarshal(shares, &smbShares) == nil {
			state.Shares = smbShares
		}
	}

	// 收集锁定
	locks, err := ss.collectSMBLocks()
	if err == nil {
		var smbLocks []*SMBLockState
		if json.Unmarshal(locks, &smbLocks) == nil {
			state.Locks = smbLocks
		}
	}

	return state, nil
}

// collectSMBConfig 收集 SMB 配置.
func (ss *StateSyncer) collectSMBConfig() ([]byte, error) {
	// 在实际实现中，这里需要读取 smb.conf 文件
	// 使用 smbstatus 获取当前状态

	config := SMBConfigState{
		Workgroup:    "WORKGROUP",
		ServerString: "NAS-OS HA",
		MinProtocol:  "SMB2",
		MaxProtocol:  "SMB3",
		GlobalConfig: make(map[string]string),
		ShareConfigs: make(map[string]string),
	}

	// 模拟读取配置文件
	configFile := "/etc/samba/smb.conf"
	if data, err := os.ReadFile(configFile); err == nil {
		// 解析配置（简化）
		config.GlobalConfig["raw_config"] = string(data)
	}

	return json.Marshal(config)
}

// collectSMBSessions 收集 SMB 会话.
func (ss *StateSyncer) collectSMBSessions() ([]byte, error) {
	// 在实际实现中，使用 smbstatus -S 获取会话信息
	// 或读取 session.tdb 文件

	sessions := make([]*SMBSession, 0)

	// 模拟获取会话信息
	// 实际实现需要解析 smbstatus 输出
	sessionFile := "/var/lib/samba/session.tdb"
	if _, err := os.Stat(sessionFile); err == nil {
		// 可以使用 tdbtool 读取
		// 这里简化处理
	}

	return json.Marshal(sessions)
}

// collectSMBShares 收集 SMB 共享状态.
func (ss *StateSyncer) collectSMBShares() ([]byte, error) {
	shares := make([]*SMBShareState, 0)

	// 在实际实现中，使用 smbstatus -S 获取共享连接信息

	return json.Marshal(shares)
}

// collectSMBLocks 收集 SMB 锁定状态.
func (ss *StateSyncer) collectSMBLocks() ([]byte, error) {
	locks := make([]*SMBLockState, 0)

	// 在实际实现中，使用 smbstatus -L 获取锁定信息
	// 或读取 locking.tdb 文件

	return json.Marshal(locks)
}

// collectFullState 收集完整状态.
func (ss *StateSyncer) collectFullState() ([]byte, error) {
	state, err := ss.collectLocalSMBState()
	if err != nil {
		return nil, err
	}

	return json.Marshal(state)
}

// sendSyncData 发送同步数据到目标节点.
func (ss *StateSyncer) sendSyncData(peer PeerNode, task *SyncTask) error {
	ctx, cancel := context.WithTimeout(ss.ctx, ss.config.SyncTimeout)
	defer cancel()

	url := fmt.Sprintf("http://%s:%d/api/v1/ha/sync", peer.Address, peer.Port)

	body, err := json.Marshal(task)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Source-Node", ss.config.NodeID)
	req.Header.Set("X-Sync-Type", string(task.Type))
	req.Header.Set("X-Sync-ID", task.ID)

	client := &http.Client{Timeout: ss.config.SyncTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sync failed: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// ReceiveSyncData 接收同步数据.
func (ss *StateSyncer) ReceiveSyncData(task *SyncTask) error {
	ss.logger.Info("Received sync data",
		zap.String("type", string(task.Type)),
		zap.String("source", task.Source),
	)

	// 应用同步数据
	switch task.Type {
	case SyncTypeSMBConfig:
		return ss.applySMBConfig(task.Data)
	case SyncTypeSMBSession:
		return ss.applySMBSessions(task.Data)
	case SyncTypeSMBShare:
		return ss.applySMBShares(task.Data)
	case SyncTypeSMBLock:
		return ss.applySMBLocks(task.Data)
	case SyncTypeFullSync:
		return ss.applyFullState(task.Data)
	default:
		return errors.New("unknown sync type")
	}
}

// applySMBConfig 应用 SMB 配置.
func (ss *StateSyncer) applySMBConfig(data []byte) error {
	var config SMBConfigState
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	// 在实际实现中，这里需要：
	// 1. 写入 smb.conf 文件
	// 2. 更新 passdb.tdb 用户数据库
	// 3. 重载 smb 服务

	ss.logger.Info("SMB config applied")
	return nil
}

// applySMBSessions 应用 SMB 会话状态.
func (ss *StateSyncer) applySMBSessions(data []byte) error {
	var sessions []*SMBSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return err
	}

	// 在实际实现中，这里需要：
	// 1. 写入 session.tdb
	// 2. 恢复会话状态
	// 这样客户端在故障转移后可以无缝恢复

	ss.logger.Info("SMB sessions applied",
		zap.Int("count", len(sessions)),
	)
	return nil
}

// applySMBShares 应用 SMB 共享状态.
func (ss *StateSyncer) applySMBShares(data []byte) error {
	var shares []*SMBShareState
	if err := json.Unmarshal(data, &shares); err != nil {
		return err
	}

	ss.logger.Info("SMB shares applied",
		zap.Int("count", len(shares)),
	)
	return nil
}

// applySMBLocks 应用 SMB 锁定状态.
func (ss *StateSyncer) applySMBLocks(data []byte) error {
	var locks []*SMBLockState
	if err := json.Unmarshal(data, &locks); err != nil {
		return err
	}

	// 在实际实现中，这里需要：
	// 1. 写入 locking.tdb
	// 2. 恢复文件锁定状态
	// 确保故障转移后锁定状态一致

	ss.logger.Info("SMB locks applied",
		zap.Int("count", len(locks)),
	)
	return nil
}

// applyFullState 应用完整状态.
func (ss *StateSyncer) applyFullState(data []byte) error {
	var state SMBState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	// 依次应用所有状态
	if state.Config != nil {
		configData, _ := json.Marshal(state.Config)
		_ = ss.applySMBConfig(configData)
	}

	if state.Sessions != nil {
		sessionData, _ := json.Marshal(state.Sessions)
		_ = ss.applySMBSessions(sessionData)
	}

	if state.Shares != nil {
		shareData, _ := json.Marshal(state.Shares)
		_ = ss.applySMBShares(shareData)
	}

	if state.Locks != nil {
		lockData, _ := json.Marshal(state.Locks)
		_ = ss.applySMBLocks(lockData)
	}

	ss.logger.Info("Full state applied")
	return nil
}

// EnqueueSyncTask 添加同步任务到队列.
func (ss *StateSyncer) EnqueueSyncTask(task SyncTask) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.syncQueue = append(ss.syncQueue, task)

	// 按优先级排序
	// sort.Slice(ss.syncQueue, func(i, j int) bool {
	// 	return ss.syncQueue[i].Priority > ss.syncQueue[j].Priority
	// })
}

// updateProgress 更新进度.
func (ss *StateSyncer) updateProgress() {
	if len(ss.syncQueue) == 0 {
		ss.progress = 100.0
	} else {
		// 基于完成的任务计算
		ss.progress = 0.0
	}
}

// Progress 获取同步进度.
func (ss *StateSyncer) Progress() float64 {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.progress
}

// GetSyncStatus 获取同步状态.
func (ss *StateSyncer) GetSyncStatus(nodeID string) *SyncStatus {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.syncStatus[nodeID]
}

// GetAllSyncStatus 获取所有同步状态.
func (ss *StateSyncer) GetAllSyncStatus() map[string]*SyncStatus {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	result := make(map[string]*SyncStatus)
	for k, v := range ss.syncStatus {
		result[k] = v
	}
	return result
}

// updateSyncStatus 更新同步状态.
func (ss *StateSyncer) updateSyncStatus(nodeID string, success bool) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	status, exists := ss.syncStatus[nodeID]
	if !exists {
		status = &SyncStatus{NodeID: nodeID}
		ss.syncStatus[nodeID] = status
	}

	status.LastSync = time.Now()
	status.TotalSyncs++

	if !success {
		status.FailedSyncs++
	}

	if status.TotalSyncs > 0 {
		status.SuccessRate = float64(status.TotalSyncs-status.FailedSyncs) / float64(status.TotalSyncs)
	}
}

// saveLocalState 保存本地状态.
func (ss *StateSyncer) saveLocalState(state *SMBState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	stateFile := filepath.Join(ss.config.DataDir, "smb_state.json")
	return os.WriteFile(stateFile, data, 0600)
}

// loadLocalState 加载本地状态.
func (ss *StateSyncer) loadLocalState() (*SMBState, error) {
	stateFile := filepath.Join(ss.config.DataDir, "smb_state.json")

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &SMBState{}, nil
		}
		return nil, err
	}

	var state SMBState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}
