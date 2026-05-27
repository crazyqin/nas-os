// Package filesnapmgr 提供文件系统快照管理核心逻辑
package filesnapmgr

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 文件系统快照管理器
type Manager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	config    *SnapshotConfig
	snapshots map[string]*FilesystemSnapshot
	policies  map[string]*SnapshotPolicy
	stopChan  chan struct{}
	running   bool
}

// NewManager 创建快照管理器
func NewManager(logger *zap.Logger, config *SnapshotConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultSnapshotConfig()
	}
	return &Manager{
		logger:    logger,
		config:    config,
		snapshots: make(map[string]*FilesystemSnapshot),
		policies:  make(map[string]*SnapshotPolicy),
		stopChan:  make(chan struct{}),
	}
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateSnapshot 创建快照
func (m *Manager) CreateSnapshot(volume, name, description string, snapshotType SnapshotType, tags []string) (*FilesystemSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证卷是否存在
	if volume == "" {
		return nil, fmt.Errorf("volume is required")
	}

	// 生成快照 ID
	id := generateID()

	// 构建快照路径
	snapPath := fmt.Sprintf("%s/%s@%s", volume, name, id)

	// 创建快照对象
	snap := &FilesystemSnapshot{
		ID:          id,
		Name:        name,
		Volume:      volume,
		Path:        snapPath,
		Type:        snapshotType,
		Status:      SnapshotStatusActive,
		Description: description,
		SizeBytes:   0,
		CreatedAt:   time.Now(),
		Tags:        tags,
		Metadata:    make(map[string]string),
	}

	// 模拟创建快照（实际应调用 ZFS/Btrfs 命令）
	if err := m.createSnapshotOnDisk(snap); err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}

	m.snapshots[id] = snap
	m.logger.Info("snapshot created",
		zap.String("id", id),
		zap.String("name", name),
		zap.String("volume", volume),
		zap.String("type", string(snapshotType)),
	)

	return snap, nil
}

// createSnapshotOnDisk 在磁盘上创建快照（模拟）
func (m *Manager) createSnapshotOnDisk(snap *FilesystemSnapshot) error {
	// 确保快照目录存在
	snapDir := filepath.Join(m.config.TempDir, snap.ID)
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	// 保存快照元数据
	metaPath := filepath.Join(snapDir, "meta.json")
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, 0644)
}

// DeleteSnapshot 删除快照
func (m *Manager) DeleteSnapshot(id string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[id]
	if !ok {
		return fmt.Errorf("snapshot %s not found", id)
	}

	if snap.Status == SnapshotStatusDeleted {
		return fmt.Errorf("snapshot %s already deleted", id)
	}

	// 检查是否有子快照
	if !force && len(snap.ChildrenIDs) > 0 {
		return fmt.Errorf("snapshot %s has %d children, use force to delete", id, len(snap.ChildrenIDs))
	}

	// 检查是否已挂载
	if snap.Status == SnapshotStatusMounted {
		return fmt.Errorf("snapshot %s is mounted, unmount before deleting", id)
	}

	snap.Status = SnapshotStatusDeleting

	// 删除快照目录
	snapDir := filepath.Join(m.config.TempDir, id)
	if err := os.RemoveAll(snapDir); err != nil {
		m.logger.Warn("failed to remove snapshot dir", zap.String("id", id), zap.Error(err))
	}

	// 从父快照中移除引用
	if snap.ParentID != "" {
		if parent, ok := m.snapshots[snap.ParentID]; ok {
			for i, childID := range parent.ChildrenIDs {
				if childID == id {
					parent.ChildrenIDs = append(parent.ChildrenIDs[:i], parent.ChildrenIDs[i+1:]...)
					break
				}
			}
		}
	}

	snap.Status = SnapshotStatusDeleted
	m.logger.Info("snapshot deleted", zap.String("id", id))

	return nil
}

// ListSnapshots 列出快照
func (m *Manager) ListSnapshots(volume string, snapshotType SnapshotType) []*FilesystemSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*FilesystemSnapshot, 0)
	for _, snap := range m.snapshots {
		if snap.Status == SnapshotStatusDeleted {
			continue
		}
		if volume != "" && snap.Volume != volume {
			continue
		}
		if snapshotType != "" && snap.Type != snapshotType {
			continue
		}
		result = append(result, snap)
	}

	// 按创建时间倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// GetSnapshot 获取快照详情
func (m *Manager) GetSnapshot(id string) (*FilesystemSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap, ok := m.snapshots[id]
	if !ok || snap.Status == SnapshotStatusDeleted {
		return nil, fmt.Errorf("snapshot %s not found", id)
	}

	return snap, nil
}

// MountSnapshot 挂载快照
func (m *Manager) MountSnapshot(req *MountRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[req.SnapshotID]
	if !ok || snap.Status == SnapshotStatusDeleted {
		return fmt.Errorf("snapshot %s not found", req.SnapshotID)
	}

	if snap.Status == SnapshotStatusMounted {
		return fmt.Errorf("snapshot %s already mounted at %s", req.SnapshotID, snap.Metadata["mount_point"])
	}

	// 创建挂载点
	if err := os.MkdirAll(req.MountPoint, 0755); err != nil {
		return fmt.Errorf("create mount point: %w", err)
	}

	snap.Status = SnapshotStatusMounted
	snap.Metadata["mount_point"] = req.MountPoint
	snap.Metadata["mount_readonly"] = fmt.Sprintf("%v", req.ReadOnly)
	snap.Metadata["mounted_at"] = time.Now().Format(time.RFC3339)

	// 保存元数据
	if err := m.saveSnapshotMeta(snap); err != nil {
		m.logger.Warn("failed to save snapshot meta", zap.Error(err))
	}

	m.logger.Info("snapshot mounted",
		zap.String("id", req.SnapshotID),
		zap.String("mount_point", req.MountPoint),
		zap.Bool("readonly", req.ReadOnly),
	)

	return nil
}

// UnmountSnapshot 卸载快照
func (m *Manager) UnmountSnapshot(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[id]
	if !ok || snap.Status == SnapshotStatusDeleted {
		return fmt.Errorf("snapshot %s not found", id)
	}

	if snap.Status != SnapshotStatusMounted {
		return fmt.Errorf("snapshot %s is not mounted", id)
	}

	delete(snap.Metadata, "mount_point")
	delete(snap.Metadata, "mount_readonly")
	delete(snap.Metadata, "mounted_at")

	snap.Status = SnapshotStatusActive

	if err := m.saveSnapshotMeta(snap); err != nil {
		m.logger.Warn("failed to save snapshot meta", zap.Error(err))
	}

	m.logger.Info("snapshot unmounted", zap.String("id", id))
	return nil
}

// RollbackSnapshot 回滚快照
func (m *Manager) RollbackSnapshot(req *RollbackRequest) (*RollbackResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[req.SnapshotID]
	if !ok || snap.Status == SnapshotStatusDeleted {
		return nil, fmt.Errorf("snapshot %s not found", req.SnapshotID)
	}

	if snap.Status == SnapshotStatusMounted {
		return nil, fmt.Errorf("snapshot %s is mounted, unmount before rollback", req.SnapshotID)
	}

	// 如果需要，先创建备份快照
	var backupID string
	if req.CreateSnapshot {
		backupID = generateID()
		backup := &FilesystemSnapshot{
			ID:        backupID,
			Name:      fmt.Sprintf("pre-rollback-%s", time.Now().Format("20060102150405")),
			Volume:    snap.Volume,
			Type:      snap.Type,
			Status:    SnapshotStatusActive,
			CreatedAt: time.Now(),
			Metadata:  map[string]string{"reason": "pre-rollback-backup"},
		}
		m.snapshots[backupID] = backup
		m.logger.Info("created pre-rollback backup", zap.String("backup_id", backupID))
	}

	snap.Status = SnapshotStatusRolling

	// 模拟回滚操作
	result := &RollbackResult{
		SnapshotID:    req.SnapshotID,
		BackupID:      backupID,
		RolledBackAt:  time.Now(),
		FilesRestored: 100, // 模拟值
		SizeRestored:  snap.SizeBytes,
	}

	snap.Status = SnapshotStatusActive

	m.logger.Info("snapshot rollback completed",
		zap.String("id", req.SnapshotID),
		zap.String("backup_id", backupID),
	)

	return result, nil
}

// CloneSnapshot 克隆快照
func (m *Manager) CloneSnapshot(req *CloneRequest) (*CloneResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[req.SnapshotID]
	if !ok || snap.Status == SnapshotStatusDeleted {
		return nil, fmt.Errorf("snapshot %s not found", req.SnapshotID)
	}

	snap.Status = SnapshotStatusCloning

	cloneID := generateID()
	clonePath := filepath.Join(m.config.CloneBaseDir, req.CloneName)

	clone := &FilesystemSnapshot{
		ID:        cloneID,
		Name:      req.CloneName,
		Volume:    snap.Volume,
		Path:      clonePath,
		Type:      snap.Type,
		Status:    SnapshotStatusActive,
		CreatedAt: time.Now(),
		ParentID:  req.SnapshotID,
		Metadata:  map[string]string{"cloned_from": req.SnapshotID},
	}

	// 创建克隆目录
	if err := os.MkdirAll(clonePath, 0755); err != nil {
		snap.Status = SnapshotStatusActive
		return nil, fmt.Errorf("create clone dir: %w", err)
	}

	m.snapshots[cloneID] = clone

	// 添加到父快照的子列表
	snap.ChildrenIDs = append(snap.ChildrenIDs, cloneID)
	snap.Status = SnapshotStatusActive

	result := &CloneResult{
		CloneID:    cloneID,
		CloneName:  req.CloneName,
		SnapshotID: req.SnapshotID,
		MountPoint: req.MountPoint,
		SizeBytes:  snap.SizeBytes,
		CreatedAt:  time.Now(),
	}

	m.logger.Info("snapshot cloned",
		zap.String("snapshot_id", req.SnapshotID),
		zap.String("clone_id", cloneID),
		zap.String("clone_name", req.CloneName),
	)

	return result, nil
}

// DiffSnapshots 比较两个快照的差异
func (m *Manager) DiffSnapshots(snapshot1ID, snapshot2ID string) (*DiffResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap1, ok := m.snapshots[snapshot1ID]
	if !ok || snap1.Status == SnapshotStatusDeleted {
		return nil, fmt.Errorf("snapshot %s not found", snapshot1ID)
	}

	snap2, ok := m.snapshots[snapshot2ID]
	if !ok || snap2.Status == SnapshotStatusDeleted {
		return nil, fmt.Errorf("snapshot %s not found", snapshot2ID)
	}

	// 模拟差异计算
	result := &DiffResult{
		Snapshot1ID: snapshot1ID,
		Snapshot2ID: snapshot2ID,
		Added:       make([]FileChange, 0),
		Modified:    make([]FileChange, 0),
		Deleted:     make([]FileChange, 0),
		SizeDelta:   snap2.SizeBytes - snap1.SizeBytes,
	}

	return result, nil
}

// ========== 策略管理 ==========

// CreatePolicy 创建快照策略
func (m *Manager) CreatePolicy(name, volume string, snapshotType SnapshotType, schedule string, retention Retention) (*SnapshotPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证 cron 表达式格式（简化验证）
	if !isValidCron(schedule) {
		return nil, fmt.Errorf("invalid cron expression: %s", schedule)
	}

	id := generateID()
	now := time.Now()

	policy := &SnapshotPolicy{
		ID:         id,
		Name:       name,
		Volume:     volume,
		Type:       snapshotType,
		Enabled:    true,
		Schedule:   schedule,
		Retention:  retention,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	m.policies[id] = policy
	m.logger.Info("snapshot policy created",
		zap.String("id", id),
		zap.String("name", name),
		zap.String("volume", volume),
		zap.String("schedule", schedule),
	)

	return policy, nil
}

// ListPolicies 列出快照策略
func (m *Manager) ListPolicies() []*SnapshotPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SnapshotPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		result = append(result, p)
	}
	return result
}

// GetPolicy 获取策略详情
func (m *Manager) GetPolicy(id string) (*SnapshotPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	return policy, nil
}

// UpdatePolicy 更新策略
func (m *Manager) UpdatePolicy(id string, enabled *bool, schedule *string, retention *Retention) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, ok := m.policies[id]
	if !ok {
		return fmt.Errorf("policy %s not found", id)
	}

	if enabled != nil {
		policy.Enabled = *enabled
	}
	if schedule != nil {
		if !isValidCron(*schedule) {
			return fmt.Errorf("invalid cron expression: %s", *schedule)
		}
		policy.Schedule = *schedule
	}
	if retention != nil {
		policy.Retention = *retention
	}
	policy.UpdatedAt = time.Now()

	m.logger.Info("snapshot policy updated", zap.String("id", id))
	return nil
}

// DeletePolicy 删除策略
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("policy %s not found", id)
	}

	delete(m.policies, id)
	m.logger.Info("snapshot policy deleted", zap.String("id", id))
	return nil
}

// ExecutePolicy 执行策略（手动触发）
func (m *Manager) ExecutePolicy(id string) (*FilesystemSnapshot, error) {
	m.mu.RLock()
	policy, ok := m.policies[id]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("policy %s not found", id)
	}
	m.mu.RUnlock()

	// 创建快照
	name := fmt.Sprintf("auto-%s", time.Now().Format("20060102150405"))
	snap, err := m.CreateSnapshot(policy.Volume, name, "Auto snapshot by policy: "+policy.Name, policy.Type, policy.Tags)
	if err != nil {
		policy.ErrorCount++
		return nil, err
	}

	// 更新策略状态
	m.mu.Lock()
	policy.RunCount++
	now := time.Now()
	policy.LastRunAt = &now
	m.mu.Unlock()

	// 执行保留策略清理
	m.applyRetentionPolicy(policy)

	return snap, nil
}

// applyRetentionPolicy 应用保留策略
func (m *Manager) applyRetentionPolicy(policy *SnapshotPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取该卷的所有快照
	var volumeSnapshots []*FilesystemSnapshot
	for _, snap := range m.snapshots {
		if snap.Volume == policy.Volume && snap.Status == SnapshotStatusDeleted {
			continue
		}
		if snap.Volume == policy.Volume {
			volumeSnapshots = append(volumeSnapshots, snap)
		}
	}

	// 按创建时间排序
	sort.Slice(volumeSnapshots, func(i, j int) bool {
		return volumeSnapshots[i].CreatedAt.After(volumeSnapshots[j].CreatedAt)
	})

	// 应用保留策略
	retention := policy.Retention
	if retention.MinKeep <= 0 {
		retention.MinKeep = 2
	}

	// 删除超过最大数量的快照
	if retention.MaxCount > 0 && len(volumeSnapshots) > retention.MaxCount {
		for i := retention.MaxCount; i < len(volumeSnapshots); i++ {
			if i < retention.MinKeep {
				continue // 保持最少保留数
			}
			snap := volumeSnapshots[i]
			if snap.Status != SnapshotStatusMounted {
				snap.Status = SnapshotStatusDeleted
				m.logger.Info("snapshot expired by retention policy",
					zap.String("id", snap.ID),
					zap.String("policy", policy.Name),
				)
			}
		}
	}

	// 删除超过最大年龄的快照
	if retention.MaxAgeDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -retention.MaxAgeDays)
		activeCount := 0
		for _, snap := range volumeSnapshots {
			if snap.Status == SnapshotStatusDeleted {
				continue
			}
			activeCount++
			if snap.CreatedAt.Before(cutoff) && activeCount > retention.MinKeep {
				snap.Status = SnapshotStatusDeleted
				m.logger.Info("snapshot expired by age",
					zap.String("id", snap.ID),
					zap.Time("created_at", snap.CreatedAt),
				)
			}
		}
	}
}

// GetStats 获取快照统计
func (m *Manager) GetStats() *SnapshotStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SnapshotStats{
		ByType:   make(map[string]int),
		ByVolume: make(map[string]int),
	}

	for _, snap := range m.snapshots {
		if snap.Status == SnapshotStatusDeleted {
			continue
		}
		stats.TotalSnapshots++
		stats.ActiveSnapshots++
		stats.TotalSizeBytes += snap.SizeBytes
		stats.ByType[string(snap.Type)]++
		stats.ByVolume[snap.Volume]++

		if stats.OldestSnapshot == nil || snap.CreatedAt.Before(*stats.OldestSnapshot) {
			stats.OldestSnapshot = &snap.CreatedAt
		}
		if stats.NewestSnapshot == nil || snap.CreatedAt.After(*stats.NewestSnapshot) {
			stats.NewestSnapshot = &snap.CreatedAt
		}
	}

	for _, policy := range m.policies {
		stats.PolicyCount++
		if policy.Enabled {
			stats.ActivePolicies++
		}
	}

	return stats
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *SnapshotConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *SnapshotConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// saveSnapshotMeta 保存快照元数据
func (m *Manager) saveSnapshotMeta(snap *FilesystemSnapshot) error {
	snapDir := filepath.Join(m.config.TempDir, snap.ID)
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return err
	}
	metaPath := filepath.Join(snapDir, "meta.json")
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, 0644)
}

// isValidCron 验证 cron 表达式格式（简化）
func isValidCron(expr string) bool {
	parts := strings.Fields(expr)
	return len(parts) == 5 || len(parts) == 6
}
