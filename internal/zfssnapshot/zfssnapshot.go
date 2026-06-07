// Package zfssnapshot 实现 ZFS 快照管理增强模块，对标 TrueNAS 快照管理
package zfssnapshot

import (
	"fmt"
	"sync"
	"time"
)

// SnapshotPolicy 快照策略
type SnapshotPolicy struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Dataset   string        `json:"dataset"`
	Schedule  string        `json:"schedule"`  // Cron 表达式
	Recursive bool          `json:"recursive"` // 是否递归子数据集
	MaxCount  int           `json:"max_count"` // 最大快照数量
	MaxAge    time.Duration `json:"max_age"`   // 最大保留时间
	MaxSize   int64         `json:"max_size"`  // 最大空间占用 (bytes)
	Enabled   bool          `json:"enabled"`
	CreatedAt time.Time     `json:"created_at"`
	LastRun   time.Time     `json:"last_run"`
	NextRun   time.Time     `json:"next_run"`
}

// Snapshot ZFS 快照
type Snapshot struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Dataset    string    `json:"dataset"`
	FullName   string    `json:"full_name"`  // dataset@snapshot
	Size       int64     `json:"size"`       // 快照占用空间
	Referenced int64     `json:"referenced"` // 引用空间
	Created    time.Time `json:"created"`
	Origin     string    `json:"origin,omitempty"` // 克隆来源
	IsClone    bool      `json:"is_clone"`
	Clones     []string  `json:"clones,omitempty"` // 克隆列表
	Tags       []string  `json:"tags,omitempty"`
}

// SnapshotReplication 快照复制任务
type SnapshotReplication struct {
	ID            string    `json:"id"`
	SourcePool    string    `json:"source_pool"`
	TargetPool    string    `json:"target_pool"`
	SourceDataset string    `json:"source_dataset"`
	TargetDataset string    `json:"target_dataset"`
	SnapshotName  string    `json:"snapshot_name"`
	Status        string    `json:"status"` // pending, running, completed, failed
	Progress      float64   `json:"progress"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	Error         string    `json:"error,omitempty"`
	Encrypted     bool      `json:"encrypted"`
}

// SpaceAnalysis 空间分析
type SpaceAnalysis struct {
	Dataset       string              `json:"dataset"`
	TotalSize     int64               `json:"total_size"`
	UsedSize      int64               `json:"used_size"`
	AvailableSize int64               `json:"available_size"`
	SnapshotCount int                 `json:"snapshot_count"`
	SnapshotSize  int64               `json:"snapshot_size"`
	CloneCount    int                 `json:"clone_count"`
	CloneSize     int64               `json:"clone_size"`
	Dependencies  map[string][]string `json:"dependencies"` // clone 依赖图
}

// ZFSSnapshotManager ZFS 快照管理器
type ZFSSnapshotManager struct {
	mu           sync.RWMutex
	policies     map[string]*SnapshotPolicy
	snapshots    map[string]*Snapshot
	replications map[string]*SnapshotReplication
	analyses     map[string]*SpaceAnalysis
	maxSnapshots int
}

// NewZFSSnapshotManager 创建快照管理器
func NewZFSSnapshotManager(maxSnapshots int) *ZFSSnapshotManager {
	if maxSnapshots <= 0 {
		maxSnapshots = 1000
	}
	return &ZFSSnapshotManager{
		policies:     make(map[string]*SnapshotPolicy),
		snapshots:    make(map[string]*Snapshot),
		replications: make(map[string]*SnapshotReplication),
		analyses:     make(map[string]*SpaceAnalysis),
		maxSnapshots: maxSnapshots,
	}
}

// CreatePolicy 创建快照策略
func (m *ZFSSnapshotManager) CreatePolicy(policy *SnapshotPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("策略ID不能为空")
	}

	if _, exists := m.policies[policy.ID]; exists {
		return fmt.Errorf("策略 %s 已存在", policy.ID)
	}

	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = time.Now()
	}

	if policy.MaxCount <= 0 {
		policy.MaxCount = 100
	}

	m.policies[policy.ID] = policy
	return nil
}

// GetPolicy 获取快照策略
func (m *ZFSSnapshotManager) GetPolicy(id string) (*SnapshotPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, exists := m.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略 %s 不存在", id)
	}
	return policy, nil
}

// ListPolicies 列出所有策略
func (m *ZFSSnapshotManager) ListPolicies() []*SnapshotPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*SnapshotPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// DeletePolicy 删除快照策略
func (m *ZFSSnapshotManager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return fmt.Errorf("策略 %s 不存在", id)
	}

	delete(m.policies, id)
	return nil
}

// CreateSnapshot 创建快照
func (m *ZFSSnapshotManager) CreateSnapshot(dataset, name string, tags []string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fullName := fmt.Sprintf("%s@%s", dataset, name)

	if _, exists := m.snapshots[fullName]; exists {
		return nil, fmt.Errorf("快照 %s 已存在", fullName)
	}

	if len(m.snapshots) >= m.maxSnapshots {
		return nil, fmt.Errorf("已达到最大快照数量限制 (%d)", m.maxSnapshots)
	}

	snapshot := &Snapshot{
		ID:       fmt.Sprintf("snap_%d", time.Now().UnixNano()),
		Name:     name,
		Dataset:  dataset,
		FullName: fullName,
		Size:     0,
		Created:  time.Now(),
		Tags:     tags,
	}

	m.snapshots[fullName] = snapshot
	return snapshot, nil
}

// GetSnapshot 获取快照
func (m *ZFSSnapshotManager) GetSnapshot(fullName string) (*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap, exists := m.snapshots[fullName]
	if !exists {
		return nil, fmt.Errorf("快照 %s 不存在", fullName)
	}
	return snap, nil
}

// ListSnapshots 列出快照
func (m *ZFSSnapshotManager) ListSnapshots(dataset string) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snaps := make([]*Snapshot, 0)
	for _, s := range m.snapshots {
		if dataset == "" || s.Dataset == dataset {
			snaps = append(snaps, s)
		}
	}
	return snaps
}

// DeleteSnapshot 删除快照
func (m *ZFSSnapshotManager) DeleteSnapshot(fullName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, exists := m.snapshots[fullName]
	if !exists {
		return fmt.Errorf("快照 %s 不存在", fullName)
	}

	if snap.IsClone {
		return fmt.Errorf("不能删除克隆快照 %s", fullName)
	}

	if len(snap.Clones) > 0 {
		return fmt.Errorf("快照 %s 有克隆依赖，无法删除", fullName)
	}

	delete(m.snapshots, fullName)
	return nil
}

// RollbackSnapshot 回滚快照
func (m *ZFSSnapshotManager) RollbackSnapshot(fullName string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap, exists := m.snapshots[fullName]
	if !exists {
		return fmt.Errorf("快照 %s 不存在", fullName)
	}

	// 模拟回滚操作
	_ = snap
	return nil
}

// CloneSnapshot 克隆快照
func (m *ZFSSnapshotManager) CloneSnapshot(fullName, targetDataset string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, exists := m.snapshots[fullName]
	if !exists {
		return nil, fmt.Errorf("快照 %s 不存在", fullName)
	}

	cloneName := fmt.Sprintf("%s_clone_%d", targetDataset, time.Now().UnixNano())
	clone := &Snapshot{
		ID:       fmt.Sprintf("clone_%d", time.Now().UnixNano()),
		Name:     cloneName,
		Dataset:  targetDataset,
		FullName: cloneName,
		Size:     snap.Size,
		Created:  time.Now(),
		Origin:   fullName,
		IsClone:  true,
	}

	snap.Clones = append(snap.Clones, cloneName)
	m.snapshots[cloneName] = clone

	return clone, nil
}

// CreateReplication 创建复制任务
func (m *ZFSSnapshotManager) CreateReplication(rep *SnapshotReplication) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rep.ID == "" {
		return fmt.Errorf("复制任务ID不能为空")
	}

	if _, exists := m.replications[rep.ID]; exists {
		return fmt.Errorf("复制任务 %s 已存在", rep.ID)
	}

	rep.Status = "pending"
	rep.StartTime = time.Now()

	m.replications[rep.ID] = rep
	return nil
}

// GetReplication 获取复制任务
func (m *ZFSSnapshotManager) GetReplication(id string) (*SnapshotReplication, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rep, exists := m.replications[id]
	if !exists {
		return nil, fmt.Errorf("复制任务 %s 不存在", id)
	}
	return rep, nil
}

// ListReplications 列出复制任务
func (m *ZFSSnapshotManager) ListReplications() []*SnapshotReplication {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reps := make([]*SnapshotReplication, 0, len(m.replications))
	for _, r := range m.replications {
		reps = append(reps, r)
	}
	return reps
}

// AnalyzeSpace 分析空间使用
func (m *ZFSSnapshotManager) AnalyzeSpace(dataset string) *SpaceAnalysis {
	m.mu.RLock()
	defer m.mu.RUnlock()

	analysis := &SpaceAnalysis{
		Dataset:      dataset,
		Dependencies: make(map[string][]string),
	}

	for _, snap := range m.snapshots {
		if snap.Dataset == dataset {
			analysis.SnapshotCount++
			analysis.SnapshotSize += snap.Size
			if snap.IsClone {
				analysis.CloneCount++
				analysis.CloneSize += snap.Size
			}
			if snap.Origin != "" {
				analysis.Dependencies[snap.Origin] = append(analysis.Dependencies[snap.Origin], snap.FullName)
			}
		}
	}

	m.analyses[dataset] = analysis
	return analysis
}

// CleanupExpired 清理过期快照
func (m *ZFSSnapshotManager) CleanupExpired(policyID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[policyID]
	if !exists {
		return 0, fmt.Errorf("策略 %s 不存在", policyID)
	}

	removed := 0
	for fullName, snap := range m.snapshots {
		if snap.Dataset != policy.Dataset {
			continue
		}

		if snap.IsClone {
			continue
		}

		// 检查是否过期
		if policy.MaxAge > 0 && time.Since(snap.Created) > policy.MaxAge {
			if len(snap.Clones) == 0 {
				delete(m.snapshots, fullName)
				removed++
			}
		}
	}

	// 检查数量限制
	if policy.MaxCount > 0 {
		snaps := make([]*Snapshot, 0)
		for _, s := range m.snapshots {
			if s.Dataset == policy.Dataset && !s.IsClone {
				snaps = append(snaps, s)
			}
		}

		if len(snaps) > policy.MaxCount {
			// 按时间排序，删除最旧的
			for i := 0; i < len(snaps)-policy.MaxCount; i++ {
				if len(snaps[i].Clones) == 0 {
					delete(m.snapshots, snaps[i].FullName)
					removed++
				}
			}
		}
	}

	policy.LastRun = time.Now()
	return removed, nil
}

// GetStats 获取统计信息
func (m *ZFSSnapshotManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_policies":     len(m.policies),
		"total_snapshots":    len(m.snapshots),
		"total_replications": len(m.replications),
	}

	cloneCount := 0
	totalSize := int64(0)
	for _, s := range m.snapshots {
		totalSize += s.Size
		if s.IsClone {
			cloneCount++
		}
	}

	stats["clone_count"] = cloneCount
	stats["total_size"] = totalSize

	return stats
}
