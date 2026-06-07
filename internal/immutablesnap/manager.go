package immutablesnap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 不可变快照核心管理器
type Manager struct {
	mu              sync.RWMutex
	logger          *slog.Logger
	snapshots       map[string]*Snapshot
	policies        map[string]*SnapshotPolicy
	retentionRules  map[string]*RetentionRule
	replicationJobs map[string]*ReplicationJob
	threatEvents    []ThreatEvent
	alertConfig     AlertConfig
}

// NewManager 创建管理器
func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		logger:          logger,
		snapshots:       make(map[string]*Snapshot),
		policies:        make(map[string]*SnapshotPolicy),
		retentionRules:  make(map[string]*RetentionRule),
		replicationJobs: make(map[string]*ReplicationJob),
		alertConfig: AlertConfig{
			Enabled:               true,
			ModifiedRateThreshold: 0.3,
			AutoSnapshotOnAlert:   true,
		},
	}
}

// ==================== 快照创建/删除/锁定 ====================

// CreateSnapshot 创建不可变快照
func (m *Manager) CreateSnapshot(datasetName, sourcePath, storagePath string, retentionHours int, tags []string, worm bool) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if datasetName == "" {
		return nil, fmt.Errorf("dataset name is required")
	}

	// 确定保留时长
	hours := retentionHours
	if hours < 1 {
		hours = 24
	}

	now := time.Now()
	id := uuid.New().String()

	snap := &Snapshot{
		ID:          id,
		DatasetName: datasetName,
		Status:      StatusPending,
		CreatedAt:   now,
		ExpiresAt:   now.Add(time.Duration(hours) * time.Hour),
		Locked:      false,
		WORM:        worm,
		Size:        0,
		Tags:        tags,
		SourcePath:  sourcePath,
		StoragePath: storagePath,
	}

	m.snapshots[id] = snap
	m.logger.Info("snapshot created",
		"id", id,
		"dataset", datasetName,
		"worm", worm,
		"expires_at", snap.ExpiresAt,
	)

	return snap, nil
}

// LockSnapshot 锁定快照，使其不可变
func (m *Manager) LockSnapshot(id string, worm bool) error {
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

	now := time.Now()
	snap.Locked = true
	snap.WORM = snap.WORM || worm
	snap.Status = StatusLocked
	snap.LockedAt = &now

	m.logger.Info("snapshot locked (immutable)",
		"id", id,
		"worm", snap.WORM,
		"expires_at", snap.ExpiresAt,
	)
	return nil
}

// UnlockSnapshot 解锁快照（仅限未过期的非 WORM 快照）
func (m *Manager) UnlockSnapshot(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[id]
	if !ok {
		return fmt.Errorf("snapshot %s not found", id)
	}
	if snap.WORM {
		return fmt.Errorf("snapshot %s is WORM protected and cannot be unlocked", id)
	}
	if snap.Status == StatusExpired {
		return fmt.Errorf("snapshot %s has expired", id)
	}
	if !snap.Locked {
		return fmt.Errorf("snapshot %s is not locked", id)
	}

	snap.Locked = false
	snap.Status = StatusPending
	snap.LockedAt = nil

	m.logger.Info("snapshot unlocked", "id", id)
	return nil
}

// DeleteSnapshot 删除快照（仅限未锁定或已过期的快照）
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
	// WORM 快照即使过期也不可删除
	if snap.WORM && snap.Status != StatusExpired {
		return fmt.Errorf("snapshot %s is WORM protected", id)
	}

	delete(m.snapshots, id)
	m.logger.Info("snapshot deleted", "id", id)
	return nil
}

// GetSnapshot 获取快照
func (m *Manager) GetSnapshot(id string) (*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap, ok := m.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("snapshot %s not found", id)
	}
	return snap, nil
}

// ListSnapshots 列出快照，支持状态过滤和数据集过滤
func (m *Manager) ListSnapshots(statusFilter SnapshotStatus, datasetFilter string) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Snapshot
	for _, snap := range m.snapshots {
		if statusFilter != "" && snap.Status != statusFilter {
			continue
		}
		if datasetFilter != "" && snap.DatasetName != datasetFilter {
			continue
		}
		result = append(result, snap)
	}

	// 按创建时间降序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// SetChecksum 设置快照校验和
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

// SetSize 设置快照大小
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

// ==================== 完整性校验 ====================

// VerifyIntegrity 验证快照完整性
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
		m.logger.Info("snapshot integrity verified", "id", id)
	}

	return result, nil
}

// VerifyAndUpdateStatus 验证完整性并更新状态
func (m *Manager) VerifyAndUpdateStatus(id string, actualChecksum string) (*IntegrityResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

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
		m.logger.Warn("snapshot integrity check failed", "id", id)
	} else {
		snap.Status = StatusVerified
		m.logger.Info("snapshot integrity verified", "id", id)
	}

	return result, nil
}

// GenerateChecksum 计算 SHA-256 校验和
func GenerateChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ==================== 自动快照策略 ====================

// CreatePolicy 创建自动快照策略
func (m *Manager) CreatePolicy(policy *SnapshotPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	if policy.DatasetName == "" {
		return fmt.Errorf("dataset name is required")
	}

	policy.ID = uuid.New().String()
	policy.CreatedAt = time.Now()

	// 计算下次运行时间
	nextRun := m.calculateNextRun(policy.Schedule)
	policy.NextRunAt = &nextRun

	m.policies[policy.ID] = policy
	m.logger.Info("snapshot policy created",
		"id", policy.ID,
		"name", policy.Name,
		"schedule", policy.Schedule,
	)
	return nil
}

// UpdatePolicy 更新策略
func (m *Manager) UpdatePolicy(id string, policy *SnapshotPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.policies[id]
	if !ok {
		return fmt.Errorf("policy %s not found", id)
	}

	policy.ID = id
	policy.CreatedAt = existing.CreatedAt
	policy.LastRunAt = existing.LastRunAt

	if policy.Schedule != existing.Schedule {
		nextRun := m.calculateNextRun(policy.Schedule)
		policy.NextRunAt = &nextRun
	} else {
		policy.NextRunAt = existing.NextRunAt
	}

	m.policies[id] = policy
	m.logger.Info("snapshot policy updated", "id", id)
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
	m.logger.Info("snapshot policy deleted", "id", id)
	return nil
}

// GetPolicy 获取策略
func (m *Manager) GetPolicy(id string) (*SnapshotPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	return policy, nil
}

// ListPolicies 列出所有策略
func (m *Manager) ListPolicies() []*SnapshotPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*SnapshotPolicy
	for _, p := range m.policies {
		result = append(result, p)
	}
	return result
}

// RunPolicy 执行策略（创建快照）
func (m *Manager) RunPolicy(policyID string) (*Snapshot, error) {
	m.mu.Lock()

	policy, ok := m.policies[policyID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("policy %s not found", policyID)
	}
	if !policy.Enabled {
		m.mu.Unlock()
		return nil, fmt.Errorf("policy %s is disabled", policyID)
	}

	// 计算保留时长
	retentionHours := m.calculateRetentionHours(policy)
	m.mu.Unlock()

	// 创建快照
	snap, err := m.CreateSnapshot(
		policy.DatasetName,
		"",
		fmt.Sprintf("/snapshots/%s/%s", policy.DatasetName, time.Now().Format("20060102-150405")),
		retentionHours,
		policy.Tags,
		false,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	// 自动锁定
	if policy.AutoLock {
		if err := m.LockSnapshot(snap.ID, false); err != nil {
			m.logger.Error("failed to auto-lock snapshot", "id", snap.ID, "error", err)
		}
	}

	// 更新策略运行时间
	m.mu.Lock()
	now := time.Now()
	policy.LastRunAt = &now
	nextRun := m.calculateNextRun(policy.Schedule)
	policy.NextRunAt = &nextRun
	m.mu.Unlock()

	m.logger.Info("policy executed",
		"policy_id", policyID,
		"snapshot_id", snap.ID,
	)

	return snap, nil
}

// calculateRetentionHours 计算保留时长
func (m *Manager) calculateRetentionHours(policy *SnapshotPolicy) int {
	switch policy.RetentionType {
	case RetentionGFS:
		// GFS 模式下，至少保留 son 配置的天数
		if policy.GFSSon > 0 {
			return policy.GFSSon * 24
		}
		return 14 * 24 // 默认 14 天
	default:
		if policy.RetentionHours > 0 {
			return policy.RetentionHours
		}
		return 24
	}
}

// calculateNextRun 计算下次运行时间
func (m *Manager) calculateNextRun(schedule ScheduleType) time.Time {
	now := time.Now()
	switch schedule {
	case ScheduleHourly:
		return now.Add(time.Hour)
	case ScheduleDaily:
		return now.Add(24 * time.Hour)
	case ScheduleWeekly:
		return now.Add(7 * 24 * time.Hour)
	case ScheduleMonthly:
		return now.Add(30 * 24 * time.Hour)
	default:
		return now.Add(24 * time.Hour)
	}
}

// ==================== 保留策略（GFS） ====================

// CreateRetentionRule 创建保留规则
func (m *Manager) CreateRetentionRule(rule *RetentionRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if rule.DatasetName == "" {
		return fmt.Errorf("dataset name is required")
	}

	rule.ID = uuid.New().String()
	rule.CreatedAt = time.Now()

	m.retentionRules[rule.ID] = rule
	m.logger.Info("retention rule created", "id", rule.ID, "name", rule.Name)
	return nil
}

// DeleteRetentionRule 删除保留规则
func (m *Manager) DeleteRetentionRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.retentionRules[id]; !ok {
		return fmt.Errorf("retention rule %s not found", id)
	}

	delete(m.retentionRules, id)
	m.logger.Info("retention rule deleted", "id", id)
	return nil
}

// ListRetentionRules 列出保留规则
func (m *Manager) ListRetentionRules() []*RetentionRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*RetentionRule
	for _, r := range m.retentionRules {
		result = append(result, r)
	}
	return result
}

// ApplyRetention 应用保留策略，删除过期快照
func (m *Manager) ApplyRetention() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 标记过期快照
	now := time.Now()
	expiredCount := 0
	for _, snap := range m.snapshots {
		if snap.Status == StatusLocked && now.After(snap.ExpiresAt) {
			snap.Status = StatusExpired
			snap.Locked = false
			snap.LockedAt = nil
			expiredCount++
			m.logger.Info("snapshot expired", "id", snap.ID)
		}
	}

	// 2. 应用 GFS 规则
	gfsDeleted := 0
	for _, rule := range m.retentionRules {
		if rule.Type == RetentionGFS {
			count := m.applyGFSRule(rule)
			gfsDeleted += count
		}
	}

	return expiredCount + gfsDeleted, nil
}

// applyGFSRule 应用 GFS 保留规则（调用者需持有锁）
func (m *Manager) applyGFSRule(rule *RetentionRule) int {
	now := time.Now()
	var datasetSnapshots []*Snapshot

	for _, snap := range m.snapshots {
		if snap.DatasetName == rule.DatasetName && snap.Status != StatusExpired {
			datasetSnapshots = append(datasetSnapshots, snap)
		}
	}

	// 按创建时间降序排序
	sort.Slice(datasetSnapshots, func(i, j int) bool {
		return datasetSnapshots[i].CreatedAt.After(datasetSnapshots[j].CreatedAt)
	})

	deleted := 0
	grandfatherCutoff := now.AddDate(0, -rule.GFSGrandfather, 0)
	fatherCutoff := now.AddDate(0, 0, -rule.GFSFather*7)
	sonCutoff := now.AddDate(0, 0, -rule.GFSSon)

	for _, snap := range datasetSnapshots {
		age := now.Sub(snap.CreatedAt)

		// 保留规则：son 天内全部保留，father 周内每周保留一个，grandfather 月内每月保留一个
		if snap.CreatedAt.After(sonCutoff) {
			continue // son 期内，保留
		}

		if snap.CreatedAt.After(fatherCutoff) {
			// father 期内，保留每周第一个
			weekNum := int(now.Sub(snap.CreatedAt).Hours() / (7 * 24))
			if weekNum > 0 && snap.CreatedAt.Weekday() != time.Monday {
				if !snap.WORM && !snap.Locked {
					delete(m.snapshots, snap.ID)
					deleted++
				}
			}
			continue
		}

		if snap.CreatedAt.After(grandfatherCutoff) {
			// grandfather 期内，保留每月第一个
			if snap.CreatedAt.Day() != 1 {
				if !snap.WORM && !snap.Locked {
					delete(m.snapshots, snap.ID)
					deleted++
				}
			}
			continue
		}

		// 超过 grandfather 期限，删除（除非 WORM/locked）
		if age > time.Duration(rule.GFSGrandfather)*30*24*time.Hour {
			if !snap.WORM && !snap.Locked {
				delete(m.snapshots, snap.ID)
				deleted++
			}
		}
	}

	return deleted
}

// ==================== 快照复制 ====================

// CreateReplicationJob 创建复制任务
func (m *Manager) CreateReplicationJob(snapshotID, remoteEndpoint, remotePath string, maxRetries int) (*ReplicationJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[snapshotID]
	if !ok {
		return nil, fmt.Errorf("snapshot %s not found", snapshotID)
	}

	if maxRetries < 1 {
		maxRetries = 3
	}

	job := &ReplicationJob{
		ID:             uuid.New().String(),
		SnapshotID:     snapshotID,
		RemoteEndpoint: remoteEndpoint,
		RemotePath:     remotePath,
		Status:         RepStatusPending,
		Progress:       0,
		MaxRetries:     maxRetries,
		CreatedAt:      time.Now(),
	}

	m.replicationJobs[job.ID] = job

	// 更新快照状态
	snap.Status = StatusReplicate

	m.logger.Info("replication job created",
		"job_id", job.ID,
		"snapshot_id", snapshotID,
		"remote", remoteEndpoint,
	)

	return job, nil
}

// StartReplication 启动复制（异步模拟）
func (m *Manager) StartReplication(jobID string) error {
	m.mu.Lock()

	job, ok := m.replicationJobs[jobID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("replication job %s not found", jobID)
	}
	if job.Status != RepStatusPending {
		m.mu.Unlock()
		return fmt.Errorf("replication job %s is not pending", jobID)
	}

	now := time.Now()
	job.Status = RepStatusRunning
	job.StartedAt = &now
	m.mu.Unlock()

	// 异步模拟复制过程
	go m.simulateReplication(jobID)

	return nil
}

// simulateReplication 模拟复制过程
func (m *Manager) simulateReplication(jobID string) {
	// 模拟复制进度
	for i := 0; i <= 10; i++ {
		m.mu.Lock()
		job, ok := m.replicationJobs[jobID]
		if !ok {
			m.mu.Unlock()
			return
		}
		job.Progress = float64(i) / 10.0
		m.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.replicationJobs[jobID]
	if !ok {
		return
	}

	now := time.Now()
	job.Status = RepStatusCompleted
	job.Progress = 1.0
	job.CompletedAt = &now

	// 更新快照的复制目标
	if snap, ok := m.snapshots[job.SnapshotID]; ok {
		snap.ReplicatedTo = append(snap.ReplicatedTo, job.RemoteEndpoint)
		snap.Status = StatusLocked // 复制完成后恢复锁定状态
	}

	m.logger.Info("replication completed",
		"job_id", jobID,
		"snapshot_id", job.SnapshotID,
	)
}

// GetReplicationJob 获取复制任务
func (m *Manager) GetReplicationJob(jobID string) (*ReplicationJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.replicationJobs[jobID]
	if !ok {
		return nil, fmt.Errorf("replication job %s not found", jobID)
	}
	return job, nil
}

// ListReplicationJobs 列出复制任务
func (m *Manager) ListReplicationJobs(snapshotID string) []*ReplicationJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ReplicationJob
	for _, job := range m.replicationJobs {
		if snapshotID == "" || job.SnapshotID == snapshotID {
			result = append(result, job)
		}
	}
	return result
}

// ==================== 勒索软件防护 ====================

// ReportThreat 报告威胁事件
func (m *Manager) ReportThreat(level ThreatLevel, modifiedRate float64, description, datasetName string) (*ThreatEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	event := ThreatEvent{
		ID:           uuid.New().String(),
		Timestamp:    time.Now(),
		Level:        level,
		ModifiedRate: modifiedRate,
		Description:  description,
		DatasetName:  datasetName,
	}

	// 检查是否需要告警
	if m.alertConfig.Enabled && modifiedRate >= m.alertConfig.ModifiedRateThreshold {
		event.AlertSent = true
		m.logger.Warn("ransomware alert triggered",
			"level", level,
			"modified_rate", modifiedRate,
			"dataset", datasetName,
		)
	}

	// 自动创建快照
	if m.alertConfig.AutoSnapshotOnAlert && event.AlertSent && datasetName != "" {
		// 在锁内直接创建并锁定快照
		snap := m.createAutoSnapshotLocked(datasetName, level)
		if snap != nil {
			event.SnapshotID = snap.ID
		}
	}

	m.threatEvents = append(m.threatEvents, event)
	return &event, nil
}

// createAutoSnapshotLocked 创建自动快照（调用者需持有锁）
func (m *Manager) createAutoSnapshotLocked(datasetName string, level ThreatLevel) *Snapshot {
	now := time.Now()
	id := uuid.New().String()

	snap := &Snapshot{
		ID:          id,
		DatasetName: datasetName,
		Status:      StatusLocked,
		CreatedAt:   now,
		ExpiresAt:   now.Add(7 * 24 * time.Hour), // 默认保留 7 天
		Locked:      true,
		LockedAt:    &now,
		WORM:        true, // 自动创建的快照默认 WORM
		Tags:        []string{"auto-threat-response", string(level)},
	}

	m.snapshots[id] = snap
	m.logger.Info("auto-snapshot created on threat",
		"snapshot_id", id,
		"threat_level", level,
		"dataset", datasetName,
	)

	return snap
}

// GetThreatEvents 获取威胁事件
func (m *Manager) GetThreatEvents() []ThreatEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ThreatEvent, len(m.threatEvents))
	copy(result, m.threatEvents)
	return result
}

// SetAlertConfig 设置告警配置
func (m *Manager) SetAlertConfig(config AlertConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertConfig = config
	m.logger.Info("alert config updated", "enabled", config.Enabled)
}

// GetAlertConfig 获取告警配置
func (m *Manager) GetAlertConfig() AlertConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alertConfig
}

// ==================== 空间统计 ====================

// GetSpaceUsage 获取空间使用统计
func (m *Manager) GetSpaceUsage(datasetName string) []SpaceUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	datasetMap := make(map[string]*SpaceUsage)

	for _, snap := range m.snapshots {
		if datasetName != "" && snap.DatasetName != datasetName {
			continue
		}

		usage, ok := datasetMap[snap.DatasetName]
		if !ok {
			usage = &SpaceUsage{
				DatasetName: snap.DatasetName,
			}
			datasetMap[snap.DatasetName] = usage
		}

		usage.TotalSnapshots++
		usage.TotalSizeBytes += snap.Size

		if snap.Locked {
			usage.LockedSize += snap.Size
		} else {
			usage.UnlockedSize += snap.Size
		}

		if usage.OldestSnapshot == nil || snap.CreatedAt.Before(parseTime(*usage.OldestSnapshot)) {
			s := snap.CreatedAt.Format(time.RFC3339)
			usage.OldestSnapshot = &s
		}
		if usage.NewestSnapshot == nil || snap.CreatedAt.After(parseTime(*usage.NewestSnapshot)) {
			s := snap.CreatedAt.Format(time.RFC3339)
			usage.NewestSnapshot = &s
		}
	}

	var result []SpaceUsage
	for _, usage := range datasetMap {
		if usage.TotalSnapshots > 0 {
			usage.AvgSnapshotSize = usage.TotalSizeBytes / int64(usage.TotalSnapshots)
		}
		result = append(result, *usage)
	}

	return result
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// GetStats 获取全局统计
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := Stats{}

	for _, snap := range m.snapshots {
		stats.TotalSnapshots++
		stats.TotalSizeBytes += snap.Size

		switch snap.Status {
		case StatusLocked:
			stats.LockedSnapshots++
		case StatusExpired:
			stats.ExpiredSnapshots++
		case StatusReplicate:
			stats.ReplicatingCount++
		}
	}

	stats.ThreatEvents = len(m.threatEvents)
	stats.ActivePolicies = len(m.policies)
	stats.ReplicationJobs = len(m.replicationJobs)
	stats.SpaceByDataset = m.getSpaceUsageLocked()

	return stats
}

// getSpaceUsageLocked 获取空间使用（调用者需持有锁）
func (m *Manager) getSpaceUsageLocked() []SpaceUsage {
	datasetMap := make(map[string]*SpaceUsage)

	for _, snap := range m.snapshots {
		usage, ok := datasetMap[snap.DatasetName]
		if !ok {
			usage = &SpaceUsage{
				DatasetName: snap.DatasetName,
			}
			datasetMap[snap.DatasetName] = usage
		}

		usage.TotalSnapshots++
		usage.TotalSizeBytes += snap.Size

		if snap.Locked {
			usage.LockedSize += snap.Size
		} else {
			usage.UnlockedSize += snap.Size
		}
	}

	var result []SpaceUsage
	for _, usage := range datasetMap {
		if usage.TotalSnapshots > 0 {
			usage.AvgSnapshotSize = usage.TotalSizeBytes / int64(usage.TotalSnapshots)
		}
		result = append(result, *usage)
	}

	return result
}

// ==================== 快照恢复 ====================

// RollbackToSnapshot 回滚到指定快照
func (m *Manager) RollbackToSnapshot(snapshotID string) (*RollbackResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[snapshotID]
	if !ok {
		return nil, fmt.Errorf("snapshot %s not found", snapshotID)
	}

	if snap.Status == StatusExpired {
		return nil, fmt.Errorf("snapshot %s has expired and cannot be used for rollback", snapshotID)
	}

	if snap.Status != StatusLocked && snap.Status != StatusVerified {
		return nil, fmt.Errorf("snapshot %s is not in a valid state for rollback (status: %s)", snapshotID, snap.Status)
	}

	// 增加回滚计数
	snap.RollbackCount++

	result := &RollbackResult{
		SnapshotID:   snapshotID,
		DatasetName:  snap.DatasetName,
		RolledBack:   true,
		RolledBackAt: time.Now(),
		Details:      fmt.Sprintf("rollback to snapshot %s (created at %s)", snapshotID, snap.CreatedAt.Format(time.RFC3339)),
	}

	m.logger.Info("rollback completed",
		"snapshot_id", snapshotID,
		"dataset", snap.DatasetName,
		"rollback_count", snap.RollbackCount,
	)

	return result, nil
}

// GetSnapshotsForDataset 获取数据集的所有快照（按创建时间排序）
func (m *Manager) GetSnapshotsForDataset(datasetName string) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Snapshot
	for _, snap := range m.snapshots {
		if snap.DatasetName == datasetName {
			result = append(result, snap)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result
}
