package snapshottimeline

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 快照时间线管理器
type Manager struct {
	mu        sync.RWMutex
	snapshots map[string]*SnapshotEntry
	datasets  map[string][]string // dataset -> snapshot IDs
}

// NewManager 创建新的快照时间线管理器
func NewManager() *Manager {
	return &Manager{
		snapshots: make(map[string]*SnapshotEntry),
		datasets:  make(map[string][]string),
	}
}

// CreateSnapshot 创建新快照
func (m *Manager) CreateSnapshot(poolID, dataset, name, description string, tags []string) (*SnapshotEntry, error) {
	if poolID == "" || dataset == "" || name == "" {
		return nil, fmt.Errorf("pool_id, dataset and name are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entry := &SnapshotEntry{
		ID:          uuid.New().String(),
		PoolID:      poolID,
		Dataset:     dataset,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		SizeBytes:   0,
		State:       SnapshotStateActive,
		Tags:        tags,
		Metadata:    make(map[string]string),
	}

	m.snapshots[entry.ID] = entry
	m.datasets[dataset] = append(m.datasets[dataset], entry.ID)

	return entry, nil
}

// GetSnapshot 获取快照详情
func (m *Manager) GetSnapshot(id string) (*SnapshotEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.snapshots[id]
	if !exists {
		return nil, fmt.Errorf("snapshot %s not found", id)
	}

	return entry, nil
}

// ListSnapshots 根据过滤条件列出快照
func (m *Manager) ListSnapshots(filter TimelineFilter) ([]*SnapshotEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*SnapshotEntry

	// 如果指定了 dataset，只遍历该 dataset 的快照
	if filter.Dataset != "" {
		ids, exists := m.datasets[filter.Dataset]
		if !exists {
			return results, nil
		}

		for _, id := range ids {
			entry := m.snapshots[id]
			if m.matchFilter(entry, filter) {
				results = append(results, entry)
			}
		}
	} else {
		// 遍历所有快照
		for _, entry := range m.snapshots {
			if m.matchFilter(entry, filter) {
				results = append(results, entry)
			}
		}
	}

	// 按创建时间倒序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	// 应用分页
	total := len(results)
	start := filter.Offset
	if start > total {
		start = total
	}

	end := total
	if filter.Limit > 0 && start+filter.Limit < total {
		end = start + filter.Limit
	}

	return results[start:end], nil
}

// matchFilter 检查快照是否匹配过滤条件
func (m *Manager) matchFilter(entry *SnapshotEntry, filter TimelineFilter) bool {
	// 过滤 pool
	if filter.PoolID != "" && entry.PoolID != filter.PoolID {
		return false
	}

	// 过滤状态
	if filter.State != "" && entry.State != filter.State {
		return false
	}

	// 过滤时间范围
	if !filter.Since.IsZero() && entry.CreatedAt.Before(filter.Since) {
		return false
	}

	if !filter.Until.IsZero() && entry.CreatedAt.After(filter.Until) {
		return false
	}

	// 过滤标签
	if len(filter.Tags) > 0 {
		hasTag := false
		for _, filterTag := range filter.Tags {
			for _, entryTag := range entry.Tags {
				if filterTag == entryTag {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	return true
}

// DeleteSnapshot 删除快照
func (m *Manager) DeleteSnapshot(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.snapshots[id]
	if !exists {
		return fmt.Errorf("snapshot %s not found", id)
	}

	// 检查是否有子快照
	if len(entry.Children) > 0 {
		return fmt.Errorf("cannot delete snapshot %s: has %d children", id, len(entry.Children))
	}

	// 从父快照的 children 列表中移除
	if entry.ParentID != "" {
		if parent, ok := m.snapshots[entry.ParentID]; ok {
			for i, childID := range parent.Children {
				if childID == id {
					parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
					break
				}
			}
		}
	}

	// 从 dataset 索引中移除
	if ids, ok := m.datasets[entry.Dataset]; ok {
		for i, snapshotID := range ids {
			if snapshotID == id {
				m.datasets[entry.Dataset] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	delete(m.snapshots, id)
	return nil
}

// RestoreSnapshot 恢复快照
func (m *Manager) RestoreSnapshot(ctx context.Context, req RestoreRequest) (*RestoreResult, error) {
	if req.SnapshotID == "" {
		return nil, fmt.Errorf("snapshot_id is required")
	}

	m.mu.RLock()
	entry, exists := m.snapshots[req.SnapshotID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("snapshot %s not found", req.SnapshotID)
	}

	startTime := time.Now()

	if req.CreateClone {
		// 创建克隆
		clone, err := m.CreateSnapshot(
			entry.PoolID,
			entry.Dataset,
			fmt.Sprintf("%s-clone", entry.Name),
			fmt.Sprintf("Clone of %s", entry.Name),
			entry.Tags,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create clone: %w", err)
		}

		m.mu.Lock()
		clone.ParentID = entry.ID
		entry.Children = append(entry.Children, clone.ID)
		entry.State = SnapshotStateCloned
		m.mu.Unlock()

		targetPath := req.TargetPath
		if targetPath == "" {
			targetPath = fmt.Sprintf("/%s/%s-clone", entry.Dataset, entry.Name)
		}

		return &RestoreResult{
			Success:      true,
			RestoredPath: targetPath,
			RestoreType:  "clone",
			Duration:     time.Since(startTime).String(),
			Message:      fmt.Sprintf("Snapshot %s cloned to %s", entry.ID, clone.ID),
		}, nil
	}

	// 回滚模式
	if !req.Force && entry.State == SnapshotStateCloned {
		return nil, fmt.Errorf("snapshot %s is cloned, use force=true to rollback", req.SnapshotID)
	}

	m.mu.Lock()
	entry.State = SnapshotStateRollback
	m.mu.Unlock()

	targetPath := req.TargetPath
	if targetPath == "" {
		targetPath = fmt.Sprintf("/%s", entry.Dataset)
	}

	return &RestoreResult{
		Success:      true,
		RestoredPath: targetPath,
		RestoreType:  "rollback",
		Duration:     time.Since(startTime).String(),
		Message:      fmt.Sprintf("Rolled back to snapshot %s", entry.ID),
	}, nil
}

// GetTimeline 获取指定 dataset 的快照时间线
func (m *Manager) GetTimeline(dataset string, since, until time.Time) ([]*SnapshotEntry, error) {
	if dataset == "" {
		return nil, fmt.Errorf("dataset is required")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	ids, exists := m.datasets[dataset]
	if !exists {
		return []*SnapshotEntry{}, nil
	}

	var results []*SnapshotEntry
	for _, id := range ids {
		entry := m.snapshots[id]

		if !since.IsZero() && entry.CreatedAt.Before(since) {
			continue
		}

		if !until.IsZero() && entry.CreatedAt.After(until) {
			continue
		}

		results = append(results, entry)
	}

	// 按创建时间正序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})

	return results, nil
}

// GetStats 获取快照统计信息
func (m *Manager) GetStats() *TimelineStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &TimelineStats{
		ByDataset: make(map[string]int),
	}

	if len(m.snapshots) == 0 {
		return stats
	}

	var totalSize int64
	var oldest, newest time.Time
	first := true

	for _, entry := range m.snapshots {
		stats.TotalSnapshots++
		totalSize += entry.SizeBytes
		stats.ByDataset[entry.Dataset]++

		if first {
			oldest = entry.CreatedAt
			newest = entry.CreatedAt
			first = false
		} else {
			if entry.CreatedAt.Before(oldest) {
				oldest = entry.CreatedAt
			}
			if entry.CreatedAt.After(newest) {
				newest = entry.CreatedAt
			}
		}
	}

	stats.TotalSizeBytes = totalSize
	stats.OldestSnapshot = oldest
	stats.NewestSnapshot = newest
	stats.AvgSnapshotSize = totalSize / int64(stats.TotalSnapshots)

	return stats
}

// CompareSnapshots 对比两个快照
func (m *Manager) CompareSnapshots(id1, id2 string) (*SnapshotDiff, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry1, exists := m.snapshots[id1]
	if !exists {
		return nil, fmt.Errorf("snapshot %s not found", id1)
	}

	entry2, exists := m.snapshots[id2]
	if !exists {
		return nil, fmt.Errorf("snapshot %s not found", id2)
	}

	diff := &SnapshotDiff{
		Snapshot1:    entry1,
		Snapshot2:    entry2,
		SizeDelta:    entry2.SizeBytes - entry1.SizeBytes,
		StateChanged: entry1.State != entry2.State,
	}

	// 计算时间差
	diff.TimeDelta = entry2.CreatedAt.Sub(entry1.CreatedAt).String()

	// 计算标签差异
	tags1Set := make(map[string]bool)
	for _, tag := range entry1.Tags {
		tags1Set[tag] = true
	}

	tags2Set := make(map[string]bool)
	for _, tag := range entry2.Tags {
		tags2Set[tag] = true
	}

	// 找出新增的标签
	for tag := range tags2Set {
		if !tags1Set[tag] {
			diff.TagsAdded = append(diff.TagsAdded, tag)
		}
	}

	// 找出移除的标签
	for tag := range tags1Set {
		if !tags2Set[tag] {
			diff.TagsRemoved = append(diff.TagsRemoved, tag)
		}
	}

	return diff, nil
}
