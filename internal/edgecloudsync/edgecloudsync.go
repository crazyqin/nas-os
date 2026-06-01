// Package edgecloudsync 实现边缘-云混合同步引擎
// 提供边缘设备与云端之间的智能数据同步、冲突解决、离线支持
package edgecloudsync

import (
	"fmt"
	"sync"
	"time"
)

// SyncMode 同步模式
type SyncMode string

const (
	ModeEdgeToCloud   SyncMode = "edge_to_cloud"
	ModeCloudToEdge   SyncMode = "cloud_to_edge"
	ModeBidirectional SyncMode = "bidirectional"
	ModeLocalOnly     SyncMode = "local_only"
)

// ConflictResolution 冲突解决策略
type ConflictResolution string

const (
	ConflictLocalWins  ConflictResolution = "local_wins"
	ConflictRemoteWins ConflictResolution = "remote_wins"
	ConflictNewest     ConflictResolution = "newest_wins"
	ConflictManual     ConflictResolution = "manual"
)

// EdgeNode 边缘节点
type EdgeNode struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Address       string            `json:"address"`
	Status        string            `json:"status"`
	LastSync      time.Time         `json:"last_sync"`
	PendingItems  int               `json:"pending_items"`
	SyncMode      SyncMode          `json:"sync_mode"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// SyncTask 同步任务
type SyncTask struct {
	ID            string           `json:"id"`
	SourceNode    string           `json:"source_node"`
	TargetNode    string           `json:"target_node"`
	Mode          SyncMode         `json:"mode"`
	ConflictRes   ConflictResolution `json:"conflict_resolution"`
	Status        string           `json:"status"`
	TotalItems    int              `json:"total_items"`
	SyncedItems   int              `json:"synced_items"`
	FailedItems   int              `json:"failed_items"`
	StartedAt     time.Time        `json:"started_at"`
	CompletedAt   *time.Time       `json:"completed_at,omitempty"`
	LastError     string           `json:"last_error,omitempty"`
}

// SyncItem 同步项
type SyncItem struct {
	ID           string    `json:"id"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	Hash         string    `json:"hash"`
	ModifiedAt   time.Time `json:"modified_at"`
	SyncStatus   string    `json:"sync_status"`
	ConflictWith string    `json:"conflict_with,omitempty"`
}

// EdgeCloudSync 边缘云同步器
type EdgeCloudSync struct {
	mu          sync.RWMutex
	nodes       map[string]*EdgeNode
	tasks       map[string]*SyncTask
	items       map[string]*SyncItem
	conflictRes ConflictResolution
	offlineQueue []*SyncItem
	maxQueueSize int
}

// Config 配置
type Config struct {
	ConflictRes   ConflictResolution `json:"conflict_resolution"`
	MaxQueueSize  int                `json:"max_queue_size"`
}

// New 创建同步器
func New(cfg Config) *EdgeCloudSync {
	if cfg.MaxQueueSize <= 0 {
		cfg.MaxQueueSize = 10000
	}
	if cfg.ConflictRes == "" {
		cfg.ConflictRes = ConflictNewest
	}

	return &EdgeCloudSync{
		nodes:        make(map[string]*EdgeNode),
		tasks:        make(map[string]*SyncTask),
		items:        make(map[string]*SyncItem),
		conflictRes:  cfg.ConflictRes,
		offlineQueue: make([]*SyncItem, 0, cfg.MaxQueueSize),
		maxQueueSize: cfg.MaxQueueSize,
	}
}

// RegisterNode 注册边缘节点
func (s *EdgeCloudSync) RegisterNode(node *EdgeNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node.ID == "" {
		return fmt.Errorf("节点ID不能为空")
	}

	node.Status = "online"
	node.LastSync = time.Now()
	s.nodes[node.ID] = node
	return nil
}

// UnregisterNode 注销边缘节点
func (s *EdgeCloudSync) UnregisterNode(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[nodeID]; !exists {
		return fmt.Errorf("节点不存在: %s", nodeID)
	}

	delete(s.nodes, nodeID)
	return nil
}

// CreateSyncTask 创建同步任务
func (s *EdgeCloudSync) CreateSyncTask(source, target string, mode SyncMode) (*SyncTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[source]; !exists {
		return nil, fmt.Errorf("源节点不存在: %s", source)
	}
	if _, exists := s.nodes[target]; !exists {
		return nil, fmt.Errorf("目标节点不存在: %s", target)
	}

	task := &SyncTask{
		ID:          fmt.Sprintf("task-%d", time.Now().UnixNano()),
		SourceNode:  source,
		TargetNode:  target,
		Mode:        mode,
		ConflictRes: s.conflictRes,
		Status:      "pending",
		StartedAt:   time.Now(),
	}

	s.tasks[task.ID] = task
	return task, nil
}

// QueueOfflineItem 添加离线同步项
func (s *EdgeCloudSync) QueueOfflineItem(item *SyncItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.offlineQueue) >= s.maxQueueSize {
		return fmt.Errorf("离线队列已满")
	}

	item.SyncStatus = "queued"
	s.offlineQueue = append(s.offlineQueue, item)
	s.items[item.ID] = item
	return nil
}

// ProcessOfflineQueue 处理离线队列
func (s *EdgeCloudSync) ProcessOfflineQueue() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	processed := 0
	for _, item := range s.offlineQueue {
		if item.SyncStatus == "queued" {
			item.SyncStatus = "syncing"
			processed++
		}
	}

	return processed
}

// ResolveConflict 解决冲突
func (s *EdgeCloudSync) ResolveConflict(itemID, choose string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.items[itemID]
	if !exists {
		return fmt.Errorf("同步项不存在: %s", itemID)
	}

	item.ConflictWith = ""
	item.SyncStatus = "resolved"
	return nil
}

// GetNodeStatus 获取节点状态
func (s *EdgeCloudSync) GetNodeStatus(nodeID string) (*EdgeNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("节点不存在: %s", nodeID)
	}

	return node, nil
}

// GetSyncStats 获取同步统计
func (s *EdgeCloudSync) GetSyncStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pending := 0
	completed := 0
	failed := 0

	for _, task := range s.tasks {
		switch task.Status {
		case "pending", "running":
			pending++
		case "completed":
			completed++
		case "failed":
			failed++
		}
	}

	return map[string]interface{}{
		"total_nodes":     len(s.nodes),
		"total_tasks":     len(s.tasks),
		"pending_tasks":   pending,
		"completed_tasks": completed,
		"failed_tasks":    failed,
		"offline_queue":   len(s.offlineQueue),
	}
}
