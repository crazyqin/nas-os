// Package crossnasreplication 提供跨 NAS 设备数据复制与同步
// 对标 TrueNAS Replication + 群晖 Hyper Backup，支持跨设备
package crossnasreplication

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ReplicationState 复制状态
type ReplicationState string

const (
	StatePending    ReplicationState = "pending"
	StateRunning    ReplicationState = "running"
	StateCompleted  ReplicationState = "completed"
	StateFailed     ReplicationState = "failed"
	StatePaused     ReplicationState = "paused"
	StateCancelled  ReplicationState = "cancelled"
)

// RemoteNode 远程节点
type RemoteNode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"` // ssh/https/smb
	AuthType    string `json:"auth_type"` // key/token/password
	Fingerprint string `json:"fingerprint,omitempty"`
	Status      string `json:"status"` // online/offline/unknown
	LastSeen    time.Time `json:"last_seen"`
}

// ReplicationTask 复制任务
type ReplicationTask struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	SourceNode    string           `json:"source_node"`
	SourcePath    string           `json:"source_path"`
	TargetNode    string           `json:"target_node"`
	TargetPath    string           `json:"target_path"`
	State         ReplicationState `json:"state"`
	Schedule      string           `json:"schedule,omitempty"` // cron expression
	Enabled       bool             `json:"enabled"`
	Compress      bool             `json:"compress"`
	Encrypt       bool             `json:"encrypt"`
	Bandwidth     int              `json:"bandwidth_limit"` // MB/s, 0 = unlimited
	TotalBytes    int64            `json:"total_bytes"`
	SyncedBytes   int64            `json:"synced_bytes"`
	TotalFiles    int64            `json:"total_files"`
	SyncedFiles   int64            `json:"synced_files"`
	LastSync      time.Time        `json:"last_sync,omitempty"`
	NextSync      time.Time        `json:"next_sync,omitempty"`
	Error         string           `json:"error,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// SyncResult 同步结果
type SyncResult struct {
	TaskID       string        `json:"task_id"`
	State        ReplicationState `json:"state"`
	BytesSynced  int64         `json:"bytes_synced"`
	FilesSynced  int64         `json:"files_synced"`
	Duration     time.Duration `json:"duration"`
	Throughput   float64       `json:"throughput_mbps"`
	Errors       []string      `json:"errors,omitempty"`
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	MaxConcurrent    int           `json:"max_concurrent"`
	RetryAttempts    int           `json:"retry_attempts"`
	RetryDelay       time.Duration `json:"retry_delay"`
	VerifySync       bool          `json:"verify_sync"`
	SnapshotBefore   bool          `json:"snapshot_before"`
	CompressionLevel int          `json:"compression_level"`
}

// DefaultManagerConfig 默认配置
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		MaxConcurrent:    3,
		RetryAttempts:    3,
		RetryDelay:       5 * time.Minute,
		VerifySync:       true,
		SnapshotBefore:   true,
		CompressionLevel: 6,
	}
}

// Manager 管理器
type Manager struct {
	config  *ManagerConfig
	nodes   map[string]*RemoteNode
	tasks   map[string]*ReplicationTask
	results map[string][]SyncResult
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewManager 创建管理器
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:  config,
		nodes:   make(map[string]*RemoteNode),
		tasks:   make(map[string]*ReplicationTask),
		results: make(map[string][]SyncResult),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start 启动管理器
func (m *Manager) Start() {
	go m.scheduleLoop()
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.cancel()
}

// RegisterNode 注册远程节点
func (m *Manager) RegisterNode(node *RemoteNode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[node.ID] = node
}

// RemoveNode 移除节点
func (m *Manager) RemoveNode(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.nodes[id]; ok {
		delete(m.nodes, id)
		return true
	}
	return false
}

// GetNodes 获取所有节点
func (m *Manager) GetNodes() []RemoteNode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	nodes := make([]RemoteNode, 0, len(m.nodes))
	for _, n := range m.nodes {
		nodes = append(nodes, *n)
	}
	return nodes
}

// CreateTask 创建复制任务
func (m *Manager) CreateTask(task *ReplicationTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, ok := m.nodes[task.SourceNode]; !ok {
		return fmt.Errorf("source node %s not found", task.SourceNode)
	}
	if _, ok := m.nodes[task.TargetNode]; !ok {
		return fmt.Errorf("target node %s not found", task.TargetNode)
	}
	
	task.State = StatePending
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	m.tasks[task.ID] = task
	return nil
}

// GetTask 获取任务
func (m *Manager) GetTask(id string) (*ReplicationTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[id]
	return t, ok
}

// GetTasks 获取所有任务
func (m *Manager) GetTasks() []ReplicationTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tasks := make([]ReplicationTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, *t)
	}
	return tasks
}

// DeleteTask 删除任务
func (m *Manager) DeleteTask(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[id]; ok {
		delete(m.tasks, id)
		delete(m.results, id)
		return true
	}
	return false
}

// StartSync 启动同步
func (m *Manager) StartSync(taskID string) (*SyncResult, error) {
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	if task.State == StateRunning {
		m.mu.Unlock()
		return nil, fmt.Errorf("task %s is already running", taskID)
	}
	task.State = StateRunning
	task.UpdatedAt = time.Now()
	m.mu.Unlock()
	
	// 模拟同步
	start := time.Now()
	result := &SyncResult{
		TaskID: taskID,
		State:  StateCompleted,
	}
	
	// 模拟数据传输
	task.TotalBytes = 1024 * 1024 * 100 // 100MB
	task.SyncedBytes = task.TotalBytes
	task.TotalFiles = 150
	task.SyncedFiles = 150
	task.LastSync = time.Now()
	
	result.BytesSynced = task.SyncedBytes
	result.FilesSynced = task.SyncedFiles
	result.Duration = time.Since(start)
	if result.Duration.Seconds() > 0 {
		result.Throughput = float64(result.BytesSynced) / result.Duration.Seconds() / 1024 / 1024
	}
	
	m.mu.Lock()
	task.State = StateCompleted
	task.UpdatedAt = time.Now()
	m.results[taskID] = append(m.results[taskID], *result)
	m.mu.Unlock()
	
	return result, nil
}

// GetTaskResults 获取任务同步历史
func (m *Manager) GetTaskResults(taskID string) []SyncResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := m.results[taskID]
	out := make([]SyncResult, len(results))
	copy(out, results)
	return out
}

// GetReplicationStats 获取复制统计
func (m *Manager) GetReplicationStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	totalTasks := len(m.tasks)
	completed := 0
	failed := 0
	running := 0
	
	for _, t := range m.tasks {
		switch t.State {
		case StateCompleted:
			completed++
		case StateFailed:
			failed++
		case StateRunning:
			running++
		}
	}
	
	return map[string]interface{}{
		"total_tasks":    totalTasks,
		"completed":      completed,
		"failed":         failed,
		"running":        running,
		"total_nodes":    len(m.nodes),
	}
}

// scheduleLoop 调度循环
func (m *Manager) scheduleLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			// 检查到期任务
		}
	}
}
