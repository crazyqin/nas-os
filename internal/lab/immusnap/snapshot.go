package immusnap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Manager 不可变快照管理器.
type Manager struct {
	mu           sync.RWMutex
	logger       *slog.Logger
	snapshots    map[string]*ImmutableSnapshot
	policy       RetentionPolicy
	threatEvents []ThreatEvent
}

// NewManager 创建不可变快照管理器.
func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		logger:    logger,
		snapshots: make(map[string]*ImmutableSnapshot),
		policy:    DefaultRetentionPolicy(),
	}
}

// generateID 生成快照 ID.
func generateID() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (8 * (i % 8)))
	}
	return hex.EncodeToString(b)
}

// CreateSnapshot 创建不可变快照
// 创建后默认处于 pending 状态，需调用 Lock 才真正不可变.
func (m *Manager) CreateSnapshot(datasetName, sourcePath, storagePath string, retentionHours int, tags []string) (*ImmutableSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if datasetName == "" {
		return nil, fmt.Errorf("dataset name is required")
	}

	// 检查快照数量限制
	if m.policy.MaxSnapshots > 0 && m.countActiveSnapshots() >= m.policy.MaxSnapshots {
		return nil, fmt.Errorf("maximum snapshot count (%d) reached", m.policy.MaxSnapshots)
	}

	// 确定保留时长
	hours := retentionHours
	if hours < m.policy.MinRetentionHours {
		hours = m.policy.MinRetentionHours
	}

	now := time.Now()
	id := generateID()

	snap := &ImmutableSnapshot{
		ID:          id,
		DatasetName: datasetName,
		Status:      StatusPending,
		CreatedAt:   now,
		ExpiresAt:   now.Add(time.Duration(hours) * time.Hour),
		Locked:      false,
		Size:        0,
		Tags:        tags,
		SourcePath:  sourcePath,
		StoragePath: storagePath,
		ThreatLevel: ThreatLevelNormal,
	}

	m.snapshots[id] = snap
	m.logger.Info("immutable snapshot created",
		"id", id,
		"dataset", datasetName,
		"expires_at", snap.ExpiresAt,
	)

	return snap, nil
}

// Lock 锁定快照，使其不可变.
func (m *Manager) Lock(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[id]
	if !ok {
		return fmt.Errorf("snapshot %s not found", id)
	}
	if snap.Status == StatusLocked {
		return fmt.Errorf("snapshot %s is already locked", id)
	}
	if snap.Status == StatusExpired {
		return fmt.Errorf("snapshot %s has expired", id)
	}

	snap.Locked = true
	snap.Status = StatusLocked

	m.logger.Info("snapshot locked (immutable)",
		"id", id,
		"expires_at", snap.ExpiresAt,
	)
	return nil
}

// GetSnapshot 获取快照信息.
func (m *Manager) GetSnapshot(id string) (*ImmutableSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap, ok := m.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("snapshot %s not found", id)
	}
	return snap, nil
}

// ListSnapshots 列出快照，可按状态过滤.
func (m *Manager) ListSnapshots(statusFilter SnapshotStatus) []*ImmutableSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ImmutableSnapshot
	for _, snap := range m.snapshots {
		if statusFilter == "" || snap.Status == statusFilter {
			result = append(result, snap)
		}
	}
	return result
}

// DeleteSnapshot 删除快照（仅限未锁定或已过期的快照）.
func (m *Manager) DeleteSnapshot(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[id]
	if !ok {
		return fmt.Errorf("snapshot %s not found", id)
	}

	// 锁定且未过期的快照不可删除
	if snap.Locked && time.Now().Before(snap.ExpiresAt) {
		return fmt.Errorf("snapshot %s is immutable until %s",
			id, snap.ExpiresAt.Format(time.RFC3339))
	}

	delete(m.snapshots, id)
	m.logger.Info("snapshot deleted", "id", id)
	return nil
}

// GetPolicy 获取保留策略.
func (m *Manager) GetPolicy() RetentionPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policy
}

// UpdatePolicy 更新保留策略.
func (m *Manager) UpdatePolicy(policy RetentionPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.MinRetentionHours < 1 {
		return fmt.Errorf("min_retention_hours must be at least 1")
	}
	if policy.MaxSnapshots < 1 {
		return fmt.Errorf("max_snapshots must be at least 1")
	}

	m.policy = policy
	m.logger.Info("retention policy updated",
		"min_retention_hours", policy.MinRetentionHours,
		"max_snapshots", policy.MaxSnapshots,
		"auto_lock_on_threat", policy.AutoLockOnThreat,
	)
	return nil
}

// VerifyIntegrity 验证快照完整性.
func (m *Manager) VerifyIntegrity(id string, actualChecksum string) (*IntegrityResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap, ok := m.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("snapshot %s not found", id)
	}

	valid := snap.Checksum == actualChecksum
	result := &IntegrityResult{
		SnapshotID:   id,
		Valid:        valid,
		ExpectedHash: snap.Checksum,
		ActualHash:   actualChecksum,
		VerifiedAt:   time.Now(),
	}

	if !valid {
		result.Details = "checksum mismatch: snapshot data may have been tampered"
		m.logger.Warn("snapshot integrity check failed",
			"id", id,
			"expected", snap.Checksum,
			"actual", actualChecksum,
		)
	} else {
		snap.Status = StatusVerified
		m.logger.Info("snapshot integrity verified", "id", id)
	}

	return result, nil
}

// SetChecksum 设置快照校验和（通常在创建快照时由存储层计算并设置）.
func (m *Manager) SetChecksum(id, checksum string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[id]
	if !ok {
		return fmt.Errorf("snapshot %s not found", id)
	}
	snap.Checksum = checksum
	return nil
}

// SetSize 设置快照大小.
func (m *Manager) SetSize(id string, size int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[id]
	if !ok {
		return fmt.Errorf("snapshot %s not found", id)
	}
	snap.Size = size
	return nil
}

// ReportThreat 报告威胁事件，根据策略自动创建不可变快照.
func (m *Manager) ReportThreat(level ThreatLevel, modifiedRate float64, description string, datasetName string) (*ThreatEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	event := ThreatEvent{
		Timestamp:    time.Now(),
		Level:        level,
		ModifiedRate: modifiedRate,
		Description:  description,
	}
	m.threatEvents = append(m.threatEvents, event)

	m.logger.Warn("threat event reported",
		"level", level,
		"modified_rate", modifiedRate,
		"description", description,
	)

	// 自动锁定策略：可疑或危急级别时自动创建不可变快照
	if m.policy.AutoLockOnThreat && (level == ThreatLevelSuspicious || level == ThreatLevelCritical) && datasetName != "" {
		snap, err := m.createAutoSnapshotLocked(datasetName, level)
		if err != nil {
			m.logger.Error("failed to create auto-snapshot on threat",
				"error", err,
				"dataset", datasetName,
			)
			return &event, err
		}
		event.SnapshotID = snap.ID
		m.threatEvents[len(m.threatEvents)-1].SnapshotID = snap.ID

		m.logger.Info("auto-snapshot created on threat",
			"snapshot_id", snap.ID,
			"threat_level", level,
			"dataset", datasetName,
		)
	}

	return &event, nil
}

// createAutoSnapshotLocked 创建并自动锁定快照（调用者需持有锁）.
func (m *Manager) createAutoSnapshotLocked(datasetName string, level ThreatLevel) (*ImmutableSnapshot, error) {
	if m.policy.MaxSnapshots > 0 && m.countActiveSnapshots() >= m.policy.MaxSnapshots {
		return nil, fmt.Errorf("maximum snapshot count (%d) reached", m.policy.MaxSnapshots)
	}

	now := time.Now()
	id := generateID()

	snap := &ImmutableSnapshot{
		ID:          id,
		DatasetName: datasetName,
		Status:      StatusLocked,
		CreatedAt:   now,
		ExpiresAt:   now.Add(time.Duration(m.policy.MinRetentionHours) * time.Hour),
		Locked:      true,
		Size:        0,
		Tags:        []string{"auto-threat-response"},
		ThreatLevel: level,
	}

	m.snapshots[id] = snap
	return snap, nil
}

// GetThreatEvents 获取威胁事件列表.
func (m *Manager) GetThreatEvents() []ThreatEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ThreatEvent, len(m.threatEvents))
	copy(result, m.threatEvents)
	return result
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := Stats{}
	var oldestTime, newestTime *time.Time

	for _, snap := range m.snapshots {
		stats.TotalSnapshots++
		stats.TotalSizeBytes += snap.Size

		switch snap.Status {
		case StatusLocked:
			stats.LockedSnapshots++
		case StatusExpired:
			stats.ExpiredSnapshots++
		}

		if oldestTime == nil || snap.CreatedAt.Before(*oldestTime) {
			t := snap.CreatedAt
			oldestTime = &t
		}
		if newestTime == nil || snap.CreatedAt.After(*newestTime) {
			t := snap.CreatedAt
			newestTime = &t
		}
	}

	if oldestTime != nil {
		s := oldestTime.Format(time.RFC3339)
		stats.OldestSnapshot = &s
	}
	if newestTime != nil {
		s := newestTime.Format(time.RFC3339)
		stats.NewestSnapshot = &s
	}

	stats.ThreatEvents = len(m.threatEvents)
	return stats
}

// ExpireSnapshots 检查并标记过期快照.
func (m *Manager) ExpireSnapshots() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	count := 0
	for _, snap := range m.snapshots {
		if snap.Status == StatusLocked && now.After(snap.ExpiresAt) {
			snap.Status = StatusExpired
			snap.Locked = false
			m.logger.Info("snapshot expired", "id", snap.ID)
			count++
		}
	}
	return count
}

// countActiveSnapshots 计算活跃快照数（调用者需持有锁）.
func (m *Manager) countActiveSnapshots() int {
	count := 0
	for _, snap := range m.snapshots {
		if snap.Status != StatusExpired {
			count++
		}
	}
	return count
}

// GenerateChecksum 计算 SHA-256 校验和（供外部使用）.
func GenerateChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
