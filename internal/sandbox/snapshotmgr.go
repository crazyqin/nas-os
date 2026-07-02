// Package sandbox 提供安全沙箱隔离环境管理功能
package sandbox

import (
	"fmt"
	"sync"
	"time"
)

// SnapshotStats 快照统计信息.
type SnapshotStats struct {
	TotalCount int
	TotalSize  int64
	BySandbox  map[string]int
}

// SnapshotManager 快照管理器.
type SnapshotManager struct {
	mu        sync.RWMutex
	snapshots map[string]*Snapshot
	basePath  string
}

// NewSnapshotManager 创建快照管理器.
func NewSnapshotManager(basePath string) *SnapshotManager {
	return &SnapshotManager{
		snapshots: make(map[string]*Snapshot),
		basePath:  basePath,
	}
}

// Create 创建快照.
func (sm *SnapshotManager) Create(sandboxID string, req *CreateSnapshotRequest) (*Snapshot, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查快照名称是否已存在
	for _, snap := range sm.snapshots {
		if snap.SandboxID == sandboxID && snap.Name == req.Name {
			return nil, ErrSnapshotAlreadyExists
		}
	}

	// 生成快照ID
	id := fmt.Sprintf("snap_%d", time.Now().UnixNano())

	// 设置默认类型
	snapType := req.Type
	if snapType == "" {
		snapType = SnapshotTypeFull
	}

	// 创建快照
	snapshot := &Snapshot{
		ID:          id,
		SandboxID:   sandboxID,
		Name:        req.Name,
		Description: req.Description,
		SizeBytes:   sm.calculateSnapshotSize(sandboxID, snapType),
		Type:        snapType,
		CreatedAt:   time.Now(),
		Labels:      req.Labels,
	}

	// 如果是增量快照，设置父快照
	if snapType == SnapshotTypeIncremental {
		parentID := sm.findLatestSnapshot(sandboxID)
		if parentID != "" {
			snapshot.ParentID = parentID
		}
	}

	sm.snapshots[id] = snapshot
	return snapshot, nil
}

// Get 获取快照.
func (sm *SnapshotManager) Get(id string) (*Snapshot, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	snapshot, exists := sm.snapshots[id]
	if !exists {
		return nil, ErrSnapshotNotFound
	}
	return snapshot, nil
}

// ListBySandbox 列出沙箱的所有快照.
func (sm *SnapshotManager) ListBySandbox(sandboxID string) []*Snapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var list []*Snapshot
	for _, snapshot := range sm.snapshots {
		if snapshot.SandboxID == sandboxID {
			list = append(list, snapshot)
		}
	}
	return list
}

// List 列出所有快照.
func (sm *SnapshotManager) List() []*Snapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	list := make([]*Snapshot, 0, len(sm.snapshots))
	for _, snapshot := range sm.snapshots {
		list = append(list, snapshot)
	}
	return list
}

// Delete 删除快照.
func (sm *SnapshotManager) Delete(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	snapshot, exists := sm.snapshots[id]
	if !exists {
		return ErrSnapshotNotFound
	}

	// 检查是否有子快照依赖
	for _, snap := range sm.snapshots {
		if snap.ParentID == id {
			return fmt.Errorf("快照 %s 被其他增量快照依赖，无法删除", id)
		}
	}

	// 清理快照数据
	if err := sm.cleanupSnapshotData(snapshot); err != nil {
		return err
	}

	delete(sm.snapshots, id)
	return nil
}

// DeleteBySandbox 删除沙箱的所有快照.
func (sm *SnapshotManager) DeleteBySandbox(sandboxID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var toDelete []string
	for id, snapshot := range sm.snapshots {
		if snapshot.SandboxID == sandboxID {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		snapshot := sm.snapshots[id]
		if err := sm.cleanupSnapshotData(snapshot); err != nil {
			return err
		}
		delete(sm.snapshots, id)
	}

	return nil
}

// Restore 从快照恢复.
func (sm *SnapshotManager) Restore(snapshotID string, targetPath string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	snapshot, exists := sm.snapshots[snapshotID]
	if !exists {
		return ErrSnapshotNotFound
	}

	// 模拟从快照恢复
	// 在实际实现中，这里会：
	// 1. 如果是增量快照，需要恢复父快照链
	// 2. 解压快照数据到目标路径
	// 3. 恢复文件权限和属性
	_ = snapshot

	return nil
}

// GetStats 获取快照统计信息.
func (sm *SnapshotManager) GetStats() *SnapshotStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := &SnapshotStats{
		TotalCount: len(sm.snapshots),
		BySandbox:  make(map[string]int),
	}

	for _, snapshot := range sm.snapshots {
		stats.TotalSize += snapshot.SizeBytes
		stats.BySandbox[snapshot.SandboxID]++
	}

	return stats
}

// Count 获取快照总数.
func (sm *SnapshotManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return len(sm.snapshots)
}

// calculateSnapshotSize 计算快照大小.
func (sm *SnapshotManager) calculateSnapshotSize(sandboxID string, snapType SnapshotType) int64 {
	// 模拟计算快照大小
	// 在实际实现中，这里会扫描沙箱文件系统并计算大小
	baseSize := int64(1024 * 1024 * 100) // 100MB 基础大小

	if snapType == SnapshotTypeIncremental {
		// 增量快照通常更小
		return baseSize / 10
	}

	return baseSize
}

// findLatestSnapshot 查找最新的快照.
func (sm *SnapshotManager) findLatestSnapshot(sandboxID string) string {
	var latest *Snapshot
	for _, snapshot := range sm.snapshots {
		if snapshot.SandboxID == sandboxID {
			if latest == nil || snapshot.CreatedAt.After(latest.CreatedAt) {
				latest = snapshot
			}
		}
	}
	if latest != nil {
		return latest.ID
	}
	return ""
}

// cleanupSnapshotData 清理快照数据.
func (sm *SnapshotManager) cleanupSnapshotData(snapshot *Snapshot) error {
	// 模拟清理快照数据
	// 在实际实现中，这里会删除快照文件
	return nil
}
