package systemrollback

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Manager 系统快照回滚管理器.
type Manager struct {
	mu        sync.RWMutex
	config    RollbackConfig
	snapshots map[string]*SystemSnapshot
	policies  map[string]*SnapshotPolicy
	rolling   bool
}

// NewManager 创建管理器.
func NewManager(cfg RollbackConfig) *Manager {
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = 1
	}
	if cfg.CompressDefault == "" {
		cfg.CompressDefault = "zstd"
	}
	return &Manager{
		config:    cfg,
		snapshots: make(map[string]*SystemSnapshot),
		policies:  make(map[string]*SnapshotPolicy),
	}
}

// ========== 快照管理 ==========

// CreateSnapshot 创建快照.
func (m *Manager) CreateSnapshot(snap *SystemSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.snapshots[snap.ID]; exists {
		return ErrSnapshotExists
	}
	snap.Status = StatusReady
	snap.CreatedAt = time.Now()
	if snap.Type == "" {
		snap.Type = SnapshotManual
	}
	m.snapshots[snap.ID] = snap
	return nil
}

// DeleteSnapshot 删除快照.
func (m *Manager) DeleteSnapshot(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.snapshots[id]; !ok {
		return ErrSnapshotNotFound
	}
	delete(m.snapshots, id)
	return nil
}

// GetSnapshot 获取快照.
func (m *Manager) GetSnapshot(id string) (*SystemSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snap, ok := m.snapshots[id]
	if !ok {
		return nil, ErrSnapshotNotFound
	}
	return snap, nil
}

// ListSnapshots 列出快照.
func (m *Manager) ListSnapshots(snapType SnapshotType, limit int) []*SystemSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*SystemSnapshot
	for _, s := range m.snapshots {
		if snapType != "" && s.Type != snapType {
			continue
		}
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// ========== 回滚操作 ==========

// Rollback 执行回滚.
func (m *Manager) Rollback(req RollbackRequest) (*RollbackResult, error) {
	m.mu.Lock()
	snap, ok := m.snapshots[req.SnapshotID]
	if !ok {
		m.mu.Unlock()
		return nil, ErrSnapshotNotFound
	}
	if m.rolling {
		m.mu.Unlock()
		return nil, ErrSystemBusy
	}
	m.rolling = true
	m.mu.Unlock()

	start := time.Now()
	result := &RollbackResult{
		SnapshotID: req.SnapshotID,
	}

	// 模拟回滚过程
	if req.BackupCurrent {
		backupID := fmt.Sprintf("pre-rollback-%d", time.Now().Unix())
		_ = m.CreateSnapshot(&SystemSnapshot{
			ID:          backupID,
			Name:        "回滚前自动备份",
			Description: "执行回滚前自动创建的系统快照",
			Type:        SnapshotAuto,
		})
		result.BackupID = backupID
	}

	if !req.DryRun {
		snap.Status = StatusRolling
		time.Sleep(500 * time.Millisecond) // 模拟回滚
		snap.Status = StatusReady
		snap.RollbackCount++
		snap.LastRollback = time.Now()
		result.Success = true
		result.RebootPending = req.RebootAfter
	} else {
		result.Success = true
	}

	result.Duration = time.Since(start).Round(time.Millisecond).String()

	m.mu.Lock()
	m.rolling = false
	m.mu.Unlock()

	return result, nil
}

// DiffSnapshots 对比两个快照.
func (m *Manager) DiffSnapshots(id1, id2 string) (*DiffResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.snapshots[id1]; !ok {
		return nil, ErrSnapshotNotFound
	}
	if _, ok := m.snapshots[id2]; !ok {
		return nil, ErrSnapshotNotFound
	}
	return &DiffResult{
		Snapshot1: id1,
		Snapshot2: id2,
		ChangeSet: ChangeSet{
			FilesAdded:    42,
			FilesModified: 128,
			FilesDeleted:  15,
			BytesChanged:  1024 * 1024 * 5,
		},
		GeneratedAt: time.Now(),
	}, nil
}

// ========== 策略管理 ==========

// CreatePolicy 创建策略.
func (m *Manager) CreatePolicy(policy *SnapshotPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	policy.Enabled = true
	policy.CreatedAt = time.Now()
	m.policies[policy.ID] = policy
	return nil
}

// DeletePolicy 删除策略.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.policies[id]; !ok {
		return ErrPolicyNotFound
	}
	delete(m.policies, id)
	return nil
}

// ListPolicies 列出策略.
func (m *Manager) ListPolicies() []*SnapshotPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*SnapshotPolicy
	for _, p := range m.policies {
		result = append(result, p)
	}
	return result
}

// ========== 统计 ==========

// GetStats 获取统计.
func (m *Manager) GetStats() RollbackStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := RollbackStats{
		TotalSnapshots:   len(m.snapshots),
		TotalPolicies:    len(m.policies),
		TypeDistribution: make(map[string]int),
	}
	var totalSize int64
	for _, s := range m.snapshots {
		totalSize += s.Size
		stats.TypeDistribution[string(s.Type)]++
		if s.Status == StatusReady {
			stats.ReadySnapshots++
		}
		if s.CreatedAt.After(stats.LastSnapshot) {
			stats.LastSnapshot = s.CreatedAt
		}
		if s.LastRollback.After(stats.LastRollback) {
			stats.LastRollback = s.LastRollback
		}
	}
	stats.TotalSize = totalSize
	for _, p := range m.policies {
		if p.Enabled {
			stats.ActivePolicies++
		}
	}
	return stats
}

// CleanupExpired 清理过期快照.
func (m *Manager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	var cleaned int
	now := time.Now()
	for id, s := range m.snapshots {
		if s.ExpiresAt != nil && now.After(*s.ExpiresAt) {
			delete(m.snapshots, id)
			cleaned++
		}
	}
	return cleaned
}
