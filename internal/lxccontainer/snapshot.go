package lxccontainer

import (
	"fmt"
	"sync"
	"time"
)

// SnapshotManager 容器快照管理.
type SnapshotManager struct {
	mu        sync.RWMutex
	snapshots map[string]*Snapshot // key: snapshot ID
	byParent  map[string][]string  // containerID -> snapshot IDs
}

// NewSnapshotManager 创建快照管理器.
func NewSnapshotManager() *SnapshotManager {
	return &SnapshotManager{
		snapshots: make(map[string]*Snapshot),
		byParent:  make(map[string][]string),
	}
}

// CreateSnapshot 创建快照.
func (sm *SnapshotManager) CreateSnapshot(containerID, name string, container *Container) (*Snapshot, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if containerID == "" {
		return nil, fmt.Errorf("容器 ID 不能为空")
	}
	if name == "" {
		return nil, fmt.Errorf("快照名称不能为空")
	}
	if container == nil {
		return nil, fmt.Errorf("容器不存在")
	}

	// 检查同名快照
	for _, sid := range sm.byParent[containerID] {
		if sm.snapshots[sid].Name == name {
			return nil, fmt.Errorf("快照 %s 已存在", name)
		}
	}

	now := time.Now()
	snap := &Snapshot{
		ID:          fmt.Sprintf("snap-%d", now.UnixNano()),
		ContainerID: containerID,
		Name:        name,
		Status:      SnapshotReady,
		CreatedAt:   now,
		SizeMB:      container.Resources.DiskGB * 1024 / 2,
		State:       container.Status,
		Metadata: map[string]string{
			"template": container.Template,
			"hostname": container.Hostname,
		},
	}

	sm.snapshots[snap.ID] = snap
	sm.byParent[containerID] = append(sm.byParent[containerID], snap.ID)
	return snap, nil
}

// RestoreSnapshot 恢复快照.
func (sm *SnapshotManager) RestoreSnapshot(snapshotID string, container *Container) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	snap, ok := sm.snapshots[snapshotID]
	if !ok {
		return fmt.Errorf("快照 %s 不存在", snapshotID)
	}
	if snap.Status != SnapshotReady {
		return fmt.Errorf("快照 %s 状态 %s 不允许恢复", snapshotID, snap.Status)
	}

	// 恢复容器状态
	container.Status = snap.State
	container.UpdatedAt = time.Now()
	return nil
}

// DeleteSnapshot 删除快照.
func (sm *SnapshotManager) DeleteSnapshot(snapshotID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	snap, ok := sm.snapshots[snapshotID]
	if !ok {
		return fmt.Errorf("快照 %s 不存在", snapshotID)
	}

	containerID := snap.ContainerID
	delete(sm.snapshots, snapshotID)

	// 从 byParent 索引中移除
	ids := sm.byParent[containerID]
	for i, id := range ids {
		if id == snapshotID {
			sm.byParent[containerID] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	if len(sm.byParent[containerID]) == 0 {
		delete(sm.byParent, containerID)
	}

	return nil
}

// ListSnapshots 列出容器所有快照.
func (sm *SnapshotManager) ListSnapshots(containerID string) []*Snapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	ids := sm.byParent[containerID]
	result := make([]*Snapshot, 0, len(ids))
	for _, id := range ids {
		result = append(result, sm.snapshots[id])
	}
	return result
}

// GetSnapshot 获取快照.
func (sm *SnapshotManager) GetSnapshot(snapshotID string) (*Snapshot, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	snap, ok := sm.snapshots[snapshotID]
	if !ok {
		return nil, fmt.Errorf("快照 %s 不存在", snapshotID)
	}
	return snap, nil
}

// Count 返回快照总数.
func (sm *SnapshotManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.snapshots)
}

// DeleteByContainer 删除容器的所有快照.
func (sm *SnapshotManager) DeleteByContainer(containerID string) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ids := sm.byParent[containerID]
	count := len(ids)
	for _, id := range ids {
		delete(sm.snapshots, id)
	}
	delete(sm.byParent, containerID)
	return count
}
