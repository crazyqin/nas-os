package lxccontainer

// ===== 快照操作 =====

// CreateSnapshot 创建容器快照.
func (m *Manager) CreateSnapshot(req SnapshotCreateRequest) (*Snapshot, error) {
	c, err := m.containers.Get(req.ContainerID)
	if err != nil {
		return nil, err
	}
	return m.snapshots.CreateSnapshot(req.ContainerID, req.Name, c)
}

// RestoreSnapshot 恢复快照.
func (m *Manager) RestoreSnapshot(req SnapshotRestoreRequest) error {
	c, err := m.containers.Get(req.ContainerID)
	if err != nil {
		return err
	}
	return m.snapshots.RestoreSnapshot(req.SnapshotID, c)
}

// DeleteSnapshot 删除快照.
func (m *Manager) DeleteSnapshot(snapshotID string) error {
	return m.snapshots.DeleteSnapshot(snapshotID)
}

// ListSnapshots 列出容器快照.
func (m *Manager) ListSnapshots(containerID string) []*Snapshot {
	return m.snapshots.ListSnapshots(containerID)
}

// GetSnapshot 获取快照.
func (m *Manager) GetSnapshot(snapshotID string) (*Snapshot, error) {
	return m.snapshots.GetSnapshot(snapshotID)
}
