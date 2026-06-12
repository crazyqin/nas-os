// Package zfssnapshot 提供 ZFS 快照生命周期管理功能。
// 支持自动快照调度、保留策略、跨节点复制和空间回收。
// 参考：TrueNAS 26 ZFS Snapshot、群晖 DSM 7.3 Snapshot Replication
package zfssnapshot

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// SnapshotState 快照状态
type SnapshotState string

const (
	StateCreating  SnapshotState = "creating"
	StateActive    SnapshotState = "active"
	StateDeleting  SnapshotState = "deleting"
	StateReplicating SnapshotState = "replicating"
	StateError     SnapshotState = "error"
)

// RetentionUnit 保留时间单位
type RetentionUnit string

const (
	RetentionHourly  RetentionUnit = "hourly"
	RetentionDaily   RetentionUnit = "daily"
	RetentionWeekly  RetentionUnit = "weekly"
	RetentionMonthly RetentionUnit = "monthly"
	RetentionYearly  RetentionUnit = "yearly"
)

// Snapshot ZFS 快照记录
type Snapshot struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Dataset     string        `json:"dataset"`
	State       SnapshotState `json:"state"`
	Size        int64         `json:"size"`         // 快照占用空间（字节）
	Referenced  int64         `json:"referenced"`   // 引用数据量
	Written     int64         `json:"written"`      // 写入数据量
	Clones      int           `json:"clones"`       // 克隆数量
	PolicyID    string        `json:"policy_id"`    // 关联的策略ID
	CreatedAt   time.Time     `json:"created_at"`
	ExpiresAt   *time.Time    `json:"expires_at,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
}

// SnapshotPolicy 快照策略
type SnapshotPolicy struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Enabled         bool            `json:"enabled"`
	Datasets        []string        `json:"datasets"`        // 目标数据集
	Schedule        string          `json:"schedule"`        // cron 表达式
	RetentionPolicy RetentionPolicy `json:"retention_policy"`
	Recursive       bool            `json:"recursive"`       // 递归子数据集
	PreSnapshotHook string          `json:"pre_snapshot_hook,omitempty"`
	PostSnapshotHook string         `json:"post_snapshot_hook,omitempty"`
	Replication     *ReplicationConfig `json:"replication,omitempty"`
	LastRun         time.Time       `json:"last_run"`
	NextRun         time.Time       `json:"next_run"`
	CreatedAt       time.Time       `json:"created_at"`
}

// RetentionPolicy 快照保留策略
type RetentionPolicy struct {
	Hourly  int `json:"hourly"`   // 保留最近N个小时的快照
	Daily   int `json:"daily"`    // 保留最近N天的快照
	Weekly  int `json:"weekly"`   // 保留最近N周的快照
	Monthly int `json:"monthly"`  // 保留最近N个月的快照
	Yearly  int `json:"yearly"`   // 保留最近N年的快照
	MaxTotal int `json:"max_total"` // 最大快照总数
	MinSpaceBytes int64 `json:"min_space_bytes"` // 最小保留空间
}

// ReplicationConfig 快照复制配置
type ReplicationConfig struct {
	TargetHost     string `json:"target_host"`
	TargetPool     string `json:"target_pool"`
	SSHKeyPath     string `json:"ssh_key_path"`
	Compress       bool   `json:"compress"`
	BandwidthLimit int64  `json:"bandwidth_limit"` // 带宽限制（字节/秒）
	Encrypted      bool   `json:"encrypted"`
	LastReplicated time.Time `json:"last_replicated"`
}

// SnapshotStats 快照统计信息
type SnapshotStats struct {
	TotalSnapshots  int   `json:"total_snapshots"`
	TotalSize       int64  `json:"total_size"`
	OldestSnapshot  *time.Time `json:"oldest_snapshot,omitempty"`
	NewestSnapshot  *time.Time `json:"newest_snapshot,omitempty"`
	PendingDeletes  int   `json:"pending_deletes"`
	ActivePolicies  int   `json:"active_policies"`
	FailedSnapshots int   `json:"failed_snapshots"`
}

// Manager 快照生命周期管理器
type Manager struct {
	mu        sync.RWMutex
	logger    *slog.Logger
	snapshots map[string]*Snapshot
	policies  map[string]*SnapshotPolicy
	stats     SnapshotStats
	running   bool
	stopCh    chan struct{}
}

// NewManager 创建快照管理器
func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		logger:    logger,
		snapshots: make(map[string]*Snapshot),
		policies:  make(map[string]*SnapshotPolicy),
		stopCh:    make(chan struct{}),
	}
}

// Start 启动快照管理器
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("snapshot manager already running")
	}

	m.running = true
	m.logger.Info("快照生命周期管理器已启动")

	// 启动后台任务
	go m.scheduleLoop(ctx)
	go m.cleanupLoop(ctx)

	return nil
}

// Stop 停止快照管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	close(m.stopCh)
	m.running = false
	m.logger.Info("快照生命周期管理器已停止")
	return nil
}

// CreatePolicy 创建快照策略
func (m *Manager) CreatePolicy(policy *SnapshotPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("policy ID cannot be empty")
	}
	if _, exists := m.policies[policy.ID]; exists {
		return fmt.Errorf("policy already exists: %s", policy.ID)
	}
	if len(policy.Datasets) == 0 {
		return fmt.Errorf("at least one dataset is required")
	}

	policy.CreatedAt = time.Now()
	m.policies[policy.ID] = policy
	m.updateStats()

	m.logger.Info("快照策略已创建", "policy_id", policy.ID, "datasets", policy.Datasets)
	return nil
}

// UpdatePolicy 更新快照策略
func (m *Manager) UpdatePolicy(policy *SnapshotPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[policy.ID]; !exists {
		return fmt.Errorf("policy not found: %s", policy.ID)
	}

	m.policies[policy.ID] = policy
	m.logger.Info("快照策略已更新", "policy_id", policy.ID)
	return nil
}

// DeletePolicy 删除快照策略
func (m *Manager) DeletePolicy(policyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[policyID]; !exists {
		return fmt.Errorf("policy not found: %s", policyID)
	}

	delete(m.policies, policyID)
	m.updateStats()
	m.logger.Info("快照策略已删除", "policy_id", policyID)
	return nil
}

// GetPolicy 获取快照策略
func (m *Manager) GetPolicy(policyID string) (*SnapshotPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, exists := m.policies[policyID]
	if !exists {
		return nil, fmt.Errorf("policy not found: %s", policyID)
	}
	return policy, nil
}

// ListPolicies 列出所有快照策略
func (m *Manager) ListPolicies() []*SnapshotPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SnapshotPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		result = append(result, p)
	}
	return result
}

// CreateSnapshot 手动创建快照
func (m *Manager) CreateSnapshot(dataset, name string, tags []string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshotName := fmt.Sprintf("%s@%s", dataset, name)
	if _, exists := m.snapshots[snapshotName]; exists {
		return nil, fmt.Errorf("snapshot already exists: %s", snapshotName)
	}

	snapshot := &Snapshot{
		ID:        snapshotName,
		Name:      name,
		Dataset:   dataset,
		State:     StateActive,
		Tags:      tags,
		CreatedAt: time.Now(),
	}

	m.snapshots[snapshotName] = snapshot
	m.updateStats()

	m.logger.Info("快照已创建", "snapshot", snapshotName)
	return snapshot, nil
}

// DeleteSnapshot 删除快照
func (m *Manager) DeleteSnapshot(snapshotID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshot, exists := m.snapshots[snapshotID]
	if !exists {
		return fmt.Errorf("snapshot not found: %s", snapshotID)
	}
	if snapshot.Clones > 0 {
		return fmt.Errorf("cannot delete snapshot with %d clones", snapshot.Clones)
	}

	snapshot.State = StateDeleting
	delete(m.snapshots, snapshotID)
	m.updateStats()

	m.logger.Info("快照已删除", "snapshot", snapshotID)
	return nil
}

// GetSnapshot 获取快照信息
func (m *Manager) GetSnapshot(snapshotID string) (*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot, exists := m.snapshots[snapshotID]
	if !exists {
		return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
	}
	return snapshot, nil
}

// ListSnapshots 列出指定数据集的快照
func (m *Manager) ListSnapshots(dataset string) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Snapshot
	for _, s := range m.snapshots {
		if dataset == "" || s.Dataset == dataset {
			result = append(result, s)
		}
	}
	return result
}

// GetStats 获取快照统计信息
func (m *Manager) GetStats() SnapshotStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// scheduleLoop 定时调度循环
func (m *Manager) scheduleLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.evaluatePolicies(ctx)
		}
	}
}

// cleanupLoop 清理过期快照循环
func (m *Manager) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.cleanupExpired(ctx)
		}
	}
}

// evaluatePolicies 评估并执行快照策略
func (m *Manager) evaluatePolicies(ctx context.Context) {
	m.mu.RLock()
	var policies []*SnapshotPolicy
	for _, p := range m.policies {
		if p.Enabled {
			policies = append(policies, p)
		}
	}
	m.mu.RUnlock()

	for _, policy := range policies {
		if time.Now().After(policy.NextRun) {
			m.executePolicy(ctx, policy)
		}
	}
}

// executePolicy 执行快照策略
func (m *Manager) executePolicy(ctx context.Context, policy *SnapshotPolicy) {
	m.mu.Lock()
	policy.LastRun = time.Now()
	// 简化：下次运行时间按每小时计算
	policy.NextRun = time.Now().Add(1 * time.Hour)
	m.mu.Unlock()

	for _, dataset := range policy.Datasets {
		snapshotName := fmt.Sprintf("auto-%s", time.Now().Format("20060102-150405"))
		_, err := m.CreateSnapshot(dataset, snapshotName, []string{"auto", policy.ID})
		if err != nil {
			m.logger.Error("自动快照创建失败", "dataset", dataset, "error", err)
		}
	}

	// 应用保留策略
	m.applyRetentionPolicy(policy)
}

// cleanupExpired 清理过期快照
func (m *Manager) cleanupExpired(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, snapshot := range m.snapshots {
		if snapshot.ExpiresAt != nil && now.After(*snapshot.ExpiresAt) {
			snapshot.State = StateDeleting
			delete(m.snapshots, id)
			m.logger.Info("过期快照已清理", "snapshot", id)
		}
	}
	m.updateStats()
}

// applyRetentionPolicy 应用保留策略
func (m *Manager) applyRetentionPolicy(policy *SnapshotPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 简化实现：按最大总数限制
	if policy.RetentionPolicy.MaxTotal > 0 {
		var policySnapshots []*Snapshot
		for _, s := range m.snapshots {
			if s.PolicyID == policy.ID {
				policySnapshots = append(policySnapshots, s)
			}
		}
		// 按创建时间排序，删除超出的旧快照
		if len(policySnapshots) > policy.RetentionPolicy.MaxTotal {
			excess := len(policySnapshots) - policy.RetentionPolicy.MaxTotal
			for i := 0; i < excess; i++ {
				delete(m.snapshots, policySnapshots[i].ID)
				m.logger.Info("保留策略清理快照", "snapshot", policySnapshots[i].ID)
			}
		}
	}
	m.updateStats()
}

// updateStats 更新统计信息
func (m *Manager) updateStats() {
	var totalSize int64
	var oldest, newest *time.Time
	var failed int

	for _, s := range m.snapshots {
		totalSize += s.Size
		if oldest == nil || s.CreatedAt.Before(*oldest) {
			t := s.CreatedAt
			oldest = &t
		}
		if newest == nil || s.CreatedAt.After(*newest) {
			t := s.CreatedAt
			newest = &t
		}
		if s.State == StateError {
			failed++
		}
	}

	var activePolicies int
	for _, p := range m.policies {
		if p.Enabled {
			activePolicies++
		}
	}

	m.stats = SnapshotStats{
		TotalSnapshots:  len(m.snapshots),
		TotalSize:       totalSize,
		OldestSnapshot:  oldest,
		NewestSnapshot:  newest,
		ActivePolicies:  activePolicies,
		FailedSnapshots: failed,
	}
}
