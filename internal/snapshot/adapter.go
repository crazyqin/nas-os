package snapshot

import (
	"nas-os/internal/storage"
)

// StorageAdapter 存储管理器适配器
// 将 storage.Manager 适配为 snapshot.StorageManager 接口
type StorageAdapter struct {
	manager *storage.Manager
}

// NewStorageAdapter 创建存储适配器
func NewStorageAdapter(mgr *storage.Manager) *StorageAdapter {
	return &StorageAdapter{manager: mgr}
}

// CreateSnapshot 创建快照
func (a *StorageAdapter) CreateSnapshot(volumeName, subvolName, snapshotName string, readOnly bool) (interface{}, error) {
	return a.manager.CreateSnapshot(volumeName, subvolName, snapshotName, readOnly)
}

// DeleteSnapshot 删除快照
func (a *StorageAdapter) DeleteSnapshot(volumeName, snapshotName string) error {
	return a.manager.DeleteSnapshot(volumeName, snapshotName)
}

// ListSnapshots 列出快照
func (a *StorageAdapter) ListSnapshots(volumeName string) ([]interface{}, error) {
	snapshots, err := a.manager.ListSnapshots(volumeName)
	if err != nil {
		return nil, err
	}

	// 转换为接口切片
	result := make([]interface{}, len(snapshots))
	for i, snap := range snapshots {
		result[i] = SnapshotInfo{
			Name:      snap.Name,
			Path:      snap.Path,
			CreatedAt: snap.CreatedAt,
			Size:      int64(snap.Size),
		}
	}
	return result, nil
}

// GetVolume 获取卷
func (a *StorageAdapter) GetVolume(volumeName string) interface{} {
	return a.manager.GetVolume(volumeName)
}

// NewPolicyManagerWithStorage 创建带存储适配器的策略管理器
func NewPolicyManagerWithStorage(configPath string, mgr *storage.Manager) *PolicyManager {
	adapter := NewStorageAdapter(mgr)
	return NewPolicyManager(configPath, adapter)
}
