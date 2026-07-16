package filelifecycle

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var idCounter uint64

func generateID(prefix string) string {
	id := atomic.AddUint64(&idCounter, 1)
	return fmt.Sprintf("%s-%d", prefix, id)
}

// Manager 文件生命周期管理器.
type Manager struct {
	mu                sync.RWMutex
	config            FileLifecycleConfig
	tieringPolicies   []TieringPolicy
	retentionPolicies []RetentionPolicy
	records           []FileRecord
	holds             []ComplianceHold
	migrations        []FileMigration
	destructions      []DestructionRecord
	auditLog          []AuditEntry
}

// NewManager 创建管理器实例.
func NewManager() *Manager {
	return &Manager{
		config:            DefaultConfig(),
		tieringPolicies:   make([]TieringPolicy, 0),
		retentionPolicies: make([]RetentionPolicy, 0),
		records:           make([]FileRecord, 0),
		holds:             make([]ComplianceHold, 0),
		migrations:        make([]FileMigration, 0),
		destructions:      make([]DestructionRecord, 0),
		auditLog:          make([]AuditEntry, 0),
	}
}

// ==================== 配置 ====================

// GetConfig 获取当前配置.
func (m *Manager) GetConfig() FileLifecycleConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg FileLifecycleConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	m.addAuditEntryLocked("update_config", "", "配置已更新", "system", true)
}

// ==================== 分层策略 ====================

// CreateTieringPolicy 创建分层策略.
func (m *Manager) CreateTieringPolicy(policy TieringPolicy) (*TieringPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.Name == "" {
		return nil, errors.New("策略名称不能为空")
	}

	for _, p := range m.tieringPolicies {
		if p.Name == policy.Name {
			return nil, fmt.Errorf("策略名称已存在: %s", policy.Name)
		}
	}

	policy.ID = generateID("tp")
	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now
	m.tieringPolicies = append(m.tieringPolicies, policy)
	m.addAuditEntryLocked("create_tiering_policy", policy.ID, fmt.Sprintf("创建分层策略: %s", policy.Name), "system", true)
	return &policy, nil
}

// ListTieringPolicies 列出分层策略.
func (m *Manager) ListTieringPolicies(enabled *bool) []TieringPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if enabled == nil {
		result := make([]TieringPolicy, len(m.tieringPolicies))
		copy(result, m.tieringPolicies)
		return result
	}

	result := make([]TieringPolicy, 0)
	for _, p := range m.tieringPolicies {
		if p.Enabled == *enabled {
			result = append(result, p)
		}
	}
	return result
}

// GetTieringPolicy 获取分层策略.
func (m *Manager) GetTieringPolicy(id string) (*TieringPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.tieringPolicies {
		if m.tieringPolicies[i].ID == id {
			p := m.tieringPolicies[i]
			return &p, nil
		}
	}
	return nil, fmt.Errorf("分层策略不存在: %s", id)
}

// UpdateTieringPolicy 更新分层策略.
func (m *Manager) UpdateTieringPolicy(id string, policy TieringPolicy) (*TieringPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.tieringPolicies {
		if m.tieringPolicies[i].ID == id {
			policy.ID = id
			policy.CreatedAt = m.tieringPolicies[i].CreatedAt
			policy.UpdatedAt = time.Now()
			m.tieringPolicies[i] = policy
			m.addAuditEntryLocked("update_tiering_policy", id, fmt.Sprintf("更新分层策略: %s", policy.Name), "system", true)
			return &policy, nil
		}
	}
	return nil, fmt.Errorf("分层策略不存在: %s", id)
}

// DeleteTieringPolicy 删除分层策略.
func (m *Manager) DeleteTieringPolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.tieringPolicies {
		if m.tieringPolicies[i].ID == id {
			name := m.tieringPolicies[i].Name
			m.tieringPolicies = append(m.tieringPolicies[:i], m.tieringPolicies[i+1:]...)
			m.addAuditEntryLocked("delete_tiering_policy", id, fmt.Sprintf("删除分层策略: %s", name), "system", true)
			return nil
		}
	}
	return fmt.Errorf("分层策略不存在: %s", id)
}

// ==================== 保留策略 ====================

// CreateRetentionPolicy 创建保留策略.
func (m *Manager) CreateRetentionPolicy(policy RetentionPolicy) (*RetentionPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.Name == "" {
		return nil, errors.New("策略名称不能为空")
	}

	for _, p := range m.retentionPolicies {
		if p.Name == policy.Name {
			return nil, fmt.Errorf("策略名称已存在: %s", policy.Name)
		}
	}

	policy.ID = generateID("rp")
	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now
	m.retentionPolicies = append(m.retentionPolicies, policy)
	m.addAuditEntryLocked("create_retention_policy", policy.ID, fmt.Sprintf("创建保留策略: %s", policy.Name), "system", true)
	return &policy, nil
}

// ListRetentionPolicies 列出保留策略.
func (m *Manager) ListRetentionPolicies(enabled *bool) []RetentionPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if enabled == nil {
		result := make([]RetentionPolicy, len(m.retentionPolicies))
		copy(result, m.retentionPolicies)
		return result
	}

	result := make([]RetentionPolicy, 0)
	for _, p := range m.retentionPolicies {
		if p.Enabled == *enabled {
			result = append(result, p)
		}
	}
	return result
}

// GetRetentionPolicy 获取保留策略.
func (m *Manager) GetRetentionPolicy(id string) (*RetentionPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.retentionPolicies {
		if m.retentionPolicies[i].ID == id {
			p := m.retentionPolicies[i]
			return &p, nil
		}
	}
	return nil, fmt.Errorf("保留策略不存在: %s", id)
}

// UpdateRetentionPolicy 更新保留策略.
func (m *Manager) UpdateRetentionPolicy(id string, policy RetentionPolicy) (*RetentionPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.retentionPolicies {
		if m.retentionPolicies[i].ID == id {
			policy.ID = id
			policy.CreatedAt = m.retentionPolicies[i].CreatedAt
			policy.UpdatedAt = time.Now()
			m.retentionPolicies[i] = policy
			m.addAuditEntryLocked("update_retention_policy", id, fmt.Sprintf("更新保留策略: %s", policy.Name), "system", true)
			return &policy, nil
		}
	}
	return nil, fmt.Errorf("保留策略不存在: %s", id)
}

// DeleteRetentionPolicy 删除保留策略.
func (m *Manager) DeleteRetentionPolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.retentionPolicies {
		if m.retentionPolicies[i].ID == id {
			name := m.retentionPolicies[i].Name
			m.retentionPolicies = append(m.retentionPolicies[:i], m.retentionPolicies[i+1:]...)
			m.addAuditEntryLocked("delete_retention_policy", id, fmt.Sprintf("删除保留策略: %s", name), "system", true)
			return nil
		}
	}
	return fmt.Errorf("保留策略不存在: %s", id)
}

// ==================== 文件记录 ====================

// CreateFileRecord 创建文件记录.
func (m *Manager) CreateFileRecord(record FileRecord) (*FileRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record.Path == "" {
		return nil, errors.New("文件路径不能为空")
	}

	for _, r := range m.records {
		if r.Path == record.Path {
			return nil, fmt.Errorf("文件记录已存在: %s", record.Path)
		}
	}

	record.ID = generateID("fr")
	now := time.Now()
	record.CreatedAt = now
	if record.CurrentTier == "" {
		record.CurrentTier = TierHot
	}
	if record.CurrentStage == "" {
		record.CurrentStage = StageActive
	}
	if record.LastAccessedAt.IsZero() {
		record.LastAccessedAt = now
	}
	m.records = append(m.records, record)
	m.addAuditEntryLocked("create_file_record", record.ID, fmt.Sprintf("创建文件记录: %s", record.Path), "system", true)
	return &record, nil
}

// ListFileRecords 列出文件记录.
func (m *Manager) ListFileRecords(tier FileTier, stage LifecycleStage, category FileCategory) []FileRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]FileRecord, 0)
	for _, r := range m.records {
		if tier != "" && r.CurrentTier != tier {
			continue
		}
		if stage != "" && r.CurrentStage != stage {
			continue
		}
		if category != "" && r.Category != category {
			continue
		}
		result = append(result, r)
	}
	return result
}

// GetFileRecord 获取文件记录.
func (m *Manager) GetFileRecord(id string) (*FileRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.records {
		if m.records[i].ID == id {
			r := m.records[i]
			return &r, nil
		}
	}
	return nil, fmt.Errorf("文件记录不存在: %s", id)
}

// ChangeFileTier 变更文件存储层级.
func (m *Manager) ChangeFileTier(id string, tier FileTier, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.records {
		if m.records[i].ID == id {
			oldTier := m.records[i].CurrentTier
			m.records[i].CurrentTier = tier
			m.records[i].TierHistory = append(m.records[i].TierHistory, TierTransition{
				FromTier:  oldTier,
				ToTier:    tier,
				Timestamp: time.Now(),
				Reason:    reason,
			})
			m.addAuditEntryLocked("tier_change", id, fmt.Sprintf("存储层级变更: %s -> %s, 原因: %s", oldTier, tier, reason), "system", true)
			return nil
		}
	}
	return fmt.Errorf("文件记录不存在: %s", id)
}

// ChangeFileStage 变更文件生命周期阶段.
func (m *Manager) ChangeFileStage(id string, stage LifecycleStage, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.records {
		if m.records[i].ID == id {
			oldStage := m.records[i].CurrentStage
			m.records[i].CurrentStage = stage
			m.records[i].StageHistory = append(m.records[i].StageHistory, StageTransition{
				FromStage: oldStage,
				ToStage:   stage,
				Timestamp: time.Now(),
				Reason:    reason,
			})
			m.addAuditEntryLocked("stage_change", id, fmt.Sprintf("生命周期阶段变更: %s -> %s, 原因: %s", oldStage, stage, reason), "system", true)
			return nil
		}
	}
	return fmt.Errorf("文件记录不存在: %s", id)
}

// ==================== 合规保留 ====================

// CreateHold 创建合规保留.
func (m *Manager) CreateHold(hold ComplianceHold) (*ComplianceHold, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if hold.Name == "" {
		return nil, errors.New("保留名称不能为空")
	}
	if len(hold.FilePaths) == 0 {
		return nil, errors.New("保留文件路径不能为空")
	}

	hold.ID = generateID("ch")
	hold.Active = true
	hold.CreatedAt = time.Now()
	m.holds = append(m.holds, hold)
	m.addAuditEntryLocked("create_hold", hold.ID, fmt.Sprintf("创建合规保留: %s", hold.Name), "system", true)
	return &hold, nil
}

// ListHolds 列出合规保留.
func (m *Manager) ListHolds(active *bool) []ComplianceHold {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if active == nil {
		result := make([]ComplianceHold, len(m.holds))
		copy(result, m.holds)
		return result
	}

	result := make([]ComplianceHold, 0)
	for _, h := range m.holds {
		if h.Active == *active {
			result = append(result, h)
		}
	}
	return result
}

// ReleaseHold 释放合规保留.
func (m *Manager) ReleaseHold(id string, releasedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.holds {
		if m.holds[i].ID == id {
			if !m.holds[i].Active {
				return fmt.Errorf("合规保留已释放: %s", id)
			}
			now := time.Now()
			m.holds[i].Active = false
			m.holds[i].ReleasedAt = &now
			m.holds[i].ReleasedBy = releasedBy
			m.addAuditEntryLocked("release_hold", id, fmt.Sprintf("释放合规保留: %s, 操作人: %s", m.holds[i].Name, releasedBy), releasedBy, true)
			return nil
		}
	}
	return fmt.Errorf("合规保留不存在: %s", id)
}

// ==================== 迁移任务 ====================

// CreateMigration 创建迁移任务.
func (m *Manager) CreateMigration(migration FileMigration) (*FileMigration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration.ID = generateID("mg")
	migration.State = MigrationPending
	migration.CreatedAt = time.Now()

	totalFiles := len(migration.Files)
	totalBytes := int64(0)
	for _, f := range migration.Files {
		totalBytes += f.Size
	}
	migration.TotalFiles = totalFiles
	migration.TotalBytes = totalBytes

	m.migrations = append(m.migrations, migration)
	m.addAuditEntryLocked("create_migration", migration.ID, fmt.Sprintf("创建迁移任务: %s -> %s, %d 文件", migration.SourceTier, migration.TargetTier, totalFiles), "system", true)
	return &migration, nil
}

// ListMigrations 列出迁移任务.
func (m *Manager) ListMigrations(state MigrationState) []FileMigration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]FileMigration, 0)
	for _, mg := range m.migrations {
		if state != "" && mg.State != state {
			continue
		}
		result = append(result, mg)
	}
	return result
}

// GetMigration 获取迁移任务.
func (m *Manager) GetMigration(id string) (*FileMigration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.migrations {
		if m.migrations[i].ID == id {
			mg := m.migrations[i]
			return &mg, nil
		}
	}
	return nil, fmt.Errorf("迁移任务不存在: %s", id)
}

// StartMigration 启动迁移任务.
func (m *Manager) StartMigration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.migrations {
		if m.migrations[i].ID == id {
			if m.migrations[i].State != MigrationPending {
				return fmt.Errorf("迁移任务状态不允许启动: %s", m.migrations[i].State)
			}
			m.migrations[i].State = MigrationInProgress
			m.migrations[i].StartedAt = time.Now()
			m.addAuditEntryLocked("start_migration", id, "启动迁移任务", "system", true)
			return nil
		}
	}
	return fmt.Errorf("迁移任务不存在: %s", id)
}

// CancelMigration 取消迁移任务.
func (m *Manager) CancelMigration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.migrations {
		if m.migrations[i].ID == id {
			state := m.migrations[i].State
			if state == MigrationDone || state == MigrationCancelled {
				return fmt.Errorf("迁移任务已完成或已取消: %s", state)
			}
			m.migrations[i].State = MigrationCancelled
			now := time.Now()
			m.migrations[i].CompletedAt = now
			m.addAuditEntryLocked("cancel_migration", id, "取消迁移任务", "system", true)
			return nil
		}
	}
	return fmt.Errorf("迁移任务不存在: %s", id)
}

// ==================== 批量迁移 ====================

// BatchMigrate 批量迁移.
func (m *Manager) BatchMigrate(req BatchMigrateRequest) (*BatchMigrateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.TargetTier == "" {
		return nil, errors.New("目标存储层不能为空")
	}

	// 收集待迁移文件
	files := make([]MigrationFileEntry, 0)
	skipped := 0

	// 按 ID 查找
	for _, fid := range req.FileIDs {
		found := false
		for i := range m.records {
			if m.records[i].ID == fid {
				found = true
				if m.records[i].CurrentTier == req.TargetTier {
					skipped++
					break
				}
				files = append(files, MigrationFileEntry{
					FileID: fid,
					Path:   m.records[i].Path,
					Size:   m.records[i].Size,
					State:  "pending",
				})
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("文件记录不存在: %s", fid)
		}
	}

	// 按路径查找
	for _, path := range req.Paths {
		found := false
		for i := range m.records {
			if m.records[i].Path == path {
				found = true
				if m.records[i].CurrentTier == req.TargetTier {
					skipped++
					break
				}
				files = append(files, MigrationFileEntry{
					FileID: m.records[i].ID,
					Path:   path,
					Size:   m.records[i].Size,
					State:  "pending",
				})
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("文件记录不存在: %s", path)
		}
	}

	// 创建迁移任务
	if len(files) == 0 {
		return &BatchMigrateResult{
			TotalFiles:    len(req.FileIDs) + len(req.Paths),
			AcceptedFiles: 0,
			SkippedFiles:  skipped,
			FailedFiles:   0,
		}, nil
	}

	migration := FileMigration{
		ID:         generateID("mg"),
		State:      MigrationPending,
		TargetTier: req.TargetTier,
		Files:      files,
		TotalFiles: len(files),
		DryRun:     req.DryRun,
		CreatedAt:  time.Now(),
	}

	for _, f := range files {
		migration.TotalBytes += f.Size
		// 确定源层级
		for i := range m.records {
			if m.records[i].ID == f.FileID {
				migration.SourceTier = m.records[i].CurrentTier
				break
			}
		}
	}

	m.migrations = append(m.migrations, migration)
	m.addAuditEntryLocked("batch_migrate", migration.ID, fmt.Sprintf("批量迁移: %d 文件 -> %s", len(files), req.TargetTier), "system", true)

	return &BatchMigrateResult{
		TotalFiles:    len(req.FileIDs) + len(req.Paths),
		AcceptedFiles: len(files),
		SkippedFiles:  skipped,
		FailedFiles:   0,
		MigrationID:   migration.ID,
	}, nil
}

// ==================== 销毁 ====================

// CreateDestruction 创建销毁记录.
func (m *Manager) CreateDestruction(record DestructionRecord) (*DestructionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(record.FilePaths) == 0 {
		return nil, errors.New("销毁文件路径不能为空")
	}

	record.ID = generateID("ds")
	record.State = DestroyPending
	record.CreatedAt = time.Now()
	if record.RequiresApproval {
		record.State = DestroyPending
	}

	m.destructions = append(m.destructions, record)
	m.addAuditEntryLocked("create_destruction", record.ID, fmt.Sprintf("创建销毁记录: %d 文件", len(record.FilePaths)), "system", true)
	return &record, nil
}

// GetDestruction 获取销毁记录.
func (m *Manager) GetDestruction(id string) (*DestructionRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.destructions {
		if m.destructions[i].ID == id {
			d := m.destructions[i]
			return &d, nil
		}
	}
	return nil, fmt.Errorf("销毁记录不存在: %s", id)
}

// ApproveDestruction 批准销毁.
func (m *Manager) ApproveDestruction(id string, approvedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.destructions {
		if m.destructions[i].ID == id {
			if m.destructions[i].State != DestroyPending {
				return fmt.Errorf("销毁记录状态不允许批准: %s", m.destructions[i].State)
			}
			m.destructions[i].State = DestroyApproved
			m.destructions[i].ApprovedBy = approvedBy
			now := time.Now()
			m.destructions[i].ApprovedAt = &now
			m.addAuditEntryLocked("approve_destruction", id, fmt.Sprintf("批准销毁, 审批人: %s", approvedBy), approvedBy, true)
			return nil
		}
	}
	return fmt.Errorf("销毁记录不存在: %s", id)
}

// ExecuteDestruction 执行销毁.
func (m *Manager) ExecuteDestruction(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.destructions {
		if m.destructions[i].ID == id {
			state := m.destructions[i].State
			if m.destructions[i].RequiresApproval && state != DestroyApproved {
				return fmt.Errorf("销毁需要先批准，当前状态: %s", state)
			}
			if !m.destructions[i].RequiresApproval && state != DestroyPending {
				return fmt.Errorf("销毁记录状态不允许执行: %s", state)
			}
			m.destructions[i].State = DestroyDone
			m.destructions[i].DestroyedSize = m.destructions[i].TotalSize
			now := time.Now()
			m.destructions[i].CompletedAt = &now
			m.addAuditEntryLocked("execute_destruction", id, fmt.Sprintf("执行销毁: %d 文件", len(m.destructions[i].FilePaths)), "system", true)
			return nil
		}
	}
	return fmt.Errorf("销毁记录不存在: %s", id)
}

// ==================== 自动化 ====================

// RunAutoMigrateNow 立即执行自动迁移.
func (m *Manager) RunAutoMigrateNow() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled || !m.config.AutoMigrate {
		return 0
	}

	count := 0
	for i := range m.records {
		r := &m.records[i]
		if r.CurrentStage == StageDestroyed || r.CurrentStage == StagePendingDestroy {
			continue
		}
		if len(r.HoldIDs) > 0 {
			continue
		}

		targetTier := m.evaluateTierForRecord(r)
		if targetTier != "" && targetTier != r.CurrentTier {
			oldTier := r.CurrentTier
			r.CurrentTier = targetTier
			r.TierHistory = append(r.TierHistory, TierTransition{
				FromTier:  oldTier,
				ToTier:    targetTier,
				Timestamp: time.Now(),
				Reason:    "自动迁移",
			})
			count++
		}
	}

	if count > 0 {
		m.addAuditEntryLocked("auto_migrate", "", fmt.Sprintf("自动迁移完成: %d 文件", count), "system", true)
	}
	return count
}

// RunAutoCleanupNow 立即执行自动清理.
func (m *Manager) RunAutoCleanupNow() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled || !m.config.AutoCleanup {
		return 0
	}

	count := 0
	now := time.Now()
	for i := range m.records {
		r := &m.records[i]
		if r.CurrentStage == StageDestroyed {
			continue
		}
		if len(r.HoldIDs) > 0 {
			continue
		}

		// 检查保留策略
		if r.PolicyID != "" {
			for _, p := range m.retentionPolicies {
				if p.ID == r.PolicyID && p.Kind == RetentionIndefinite {
					continue
				}
			}
		}

		// 空闲超过归档天数且未被保留
		idleDays := int(now.Sub(r.LastAccessedAt).Hours() / 24)
		if r.CurrentStage == StageArchived && idleDays > m.config.ArchiveIdleDays {
			r.CurrentStage = StageExpired
			r.StageHistory = append(r.StageHistory, StageTransition{
				FromStage: StageArchived,
				ToStage:   StageExpired,
				Timestamp: now,
				Reason:    "自动清理: 超过归档保留期",
			})
			count++
		}
	}

	if count > 0 {
		m.addAuditEntryLocked("auto_cleanup", "", fmt.Sprintf("自动清理完成: %d 文件", count), "system", true)
	}
	return count
}

// evaluateTierForRecord 评估文件应迁移到的层级.
func (m *Manager) evaluateTierForRecord(r *FileRecord) FileTier {
	now := time.Now()
	idleDays := int(now.Sub(r.LastAccessedAt).Hours() / 24)

	if r.DailyAccessCount >= m.config.HotAccessThreshold {
		return TierHot
	}
	if r.DailyAccessCount >= m.config.WarmAccessThreshold {
		return TierWarm
	}
	if idleDays >= m.config.ArchiveIdleDays {
		return TierArchive
	}
	if idleDays >= m.config.ColdIdleDays {
		return TierCold
	}
	return ""
}

// SetAutoMigrate 设置自动迁移开关.
func (m *Manager) SetAutoMigrate(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.AutoMigrate = enabled
	m.addAuditEntryLocked("toggle_auto_migrate", "", fmt.Sprintf("自动迁移: %v", enabled), "system", true)
}

// SetAutoCleanup 设置自动清理开关.
func (m *Manager) SetAutoCleanup(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.AutoCleanup = enabled
	m.addAuditEntryLocked("toggle_auto_cleanup", "", fmt.Sprintf("自动清理: %v", enabled), "system", true)
}

// ==================== 报告 ====================

// GenerateReport 生成生命周期分析报告.
func (m *Manager) GenerateReport() *LifecycleReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &LifecycleReport{
		GeneratedAt: time.Now(),
		TotalFiles:  len(m.records),
	}

	tierDist := make(map[FileTier]*TierDistribution)
	stageDist := make(map[LifecycleStage]*StageDistribution)

	for _, r := range m.records {
		report.TotalSize += r.Size

		// 层级分布
		td, ok := tierDist[r.CurrentTier]
		if !ok {
			td = &TierDistribution{Tier: r.CurrentTier}
			tierDist[r.CurrentTier] = td
		}
		td.FileCount++
		td.TotalSize += r.Size

		// 阶段分布
		sd, ok := stageDist[r.CurrentStage]
		if !ok {
			sd = &StageDistribution{Stage: r.CurrentStage}
			stageDist[r.CurrentStage] = sd
		}
		sd.FileCount++
		sd.TotalSize += r.Size

		// 统计
		if r.CurrentStage == StageExpired {
			report.ExpiredFiles++
		}
	}

	// 计算百分比
	for _, td := range tierDist {
		if report.TotalSize > 0 {
			td.Percent = float64(td.TotalSize) / float64(report.TotalSize) * 100
		}
		report.TierDistributions = append(report.TierDistributions, *td)
	}

	for _, sd := range stageDist {
		report.StageDistributions = append(report.StageDistributions, *sd)
	}

	// 待迁移数
	for _, mg := range m.migrations {
		if mg.State == MigrationPending || mg.State == MigrationInProgress {
			report.PendingMigrations += mg.TotalFiles - mg.MigratedFiles
		}
	}

	// 活跃保留数
	for _, h := range m.holds {
		if h.Active {
			report.ActiveHolds++
		}
	}

	return report
}

// ==================== 审计日志 ====================

// GetAuditLog 获取审计日志.
func (m *Manager) GetAuditLog(limit int, action string) []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AuditEntry, 0)
	// 从最新开始
	for i := len(m.auditLog) - 1; i >= 0; i-- {
		if action != "" && m.auditLog[i].Action != action {
			continue
		}
		result = append(result, m.auditLog[i])
		if len(result) >= limit {
			break
		}
	}
	return result
}

// addAuditEntryLocked 添加审计日志（需持锁）.
func (m *Manager) addAuditEntryLocked(action, target, details, operator string, success bool) {
	entry := AuditEntry{
		ID:        generateID("ae"),
		Timestamp: time.Now(),
		Action:    action,
		Target:    target,
		Details:   details,
		Operator:  operator,
		Success:   success,
	}

	// 从 target 推断额外信息
	if strings.HasPrefix(target, "fr-") {
		for _, r := range m.records {
			if r.ID == target {
				entry.FileTier = r.CurrentTier
				entry.Stage = r.CurrentStage
				break
			}
		}
	}

	m.auditLog = append(m.auditLog, entry)
}

// ==================== 状态 ====================

// GetStatus 获取模块运行状态.
func (m *Manager) GetStatus() *ModuleStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &ModuleStatus{
		Enabled:           m.config.Enabled,
		AutoMigrate:       m.config.AutoMigrate,
		AutoCleanup:       m.config.AutoCleanup,
		TotalPolicies:     len(m.tieringPolicies) + len(m.retentionPolicies),
		TotalRecords:      len(m.records),
		TierDistribution:  make(map[FileTier]int),
		StageDistribution: make(map[LifecycleStage]int),
	}

	for _, p := range m.tieringPolicies {
		if p.Enabled {
			status.ActivePolicies++
		}
	}
	for _, p := range m.retentionPolicies {
		if p.Enabled {
			status.ActivePolicies++
		}
	}

	for _, r := range m.records {
		status.TierDistribution[r.CurrentTier]++
		status.StageDistribution[r.CurrentStage]++
	}

	for _, h := range m.holds {
		if h.Active {
			status.ActiveHolds++
		}
	}

	for _, mg := range m.migrations {
		if mg.State == MigrationInProgress {
			status.RunningMigrations++
		}
	}

	for _, d := range m.destructions {
		if d.State == DestroyPending || d.State == DestroyApproved {
			status.PendingDestructions++
		}
	}

	return status
}
