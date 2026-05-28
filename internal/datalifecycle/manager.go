// Package datalifecycle - 数据生命周期管理器
// 支持数据归档、数据迁移、数据清理策略、自动分层存储
package datalifecycle

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 数据生命周期管理器
type Manager struct {
	mu sync.RWMutex

	// 策略管理
	policies map[string]*LifecyclePolicy

	// 数据记录
	records map[string]*DataRecord

	// 合规保留
	holds map[string]*ComplianceHold

	// 迁移任务
	migrations map[string]*DataMigration

	// 销毁记录
	destructions map[string]*DestructionRecord

	// 策略模板
	templates map[string]*PolicyTemplate

	// 审计日志
	auditLog []LifecycleAuditEntry

	// 模块状态
	enabled bool
}

// NewManager 创建数据生命周期管理器
func NewManager() *Manager {
	return &Manager{
		policies:     make(map[string]*LifecyclePolicy),
		records:      make(map[string]*DataRecord),
		holds:        make(map[string]*ComplianceHold),
		migrations:   make(map[string]*DataMigration),
		destructions: make(map[string]*DestructionRecord),
		templates:    make(map[string]*PolicyTemplate),
		auditLog:     make([]LifecycleAuditEntry, 0),
		enabled:      true,
	}
}

// ============================================================
// 策略管理
// ============================================================

// CreatePolicy 创建生命周期策略
func (m *Manager) CreatePolicy(policy LifecyclePolicy) (*LifecyclePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}

	if _, exists := m.policies[policy.ID]; exists {
		return nil, fmt.Errorf("策略 %s 已存在", policy.ID)
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = &policy

	m.addAuditEntry("create_policy", policy.ID, fmt.Sprintf("创建策略: %s", policy.Name), true)

	log.Printf("[数据生命周期] 创建策略: %s - %s", policy.ID, policy.Name)
	return &policy, nil
}

// GetPolicy 获取策略
func (m *Manager) GetPolicy(id string) (*LifecyclePolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, exists := m.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略 %s 不存在", id)
	}
	return policy, nil
}

// ListPolicies 列出策略
func (m *Manager) ListPolicies(enabled *bool) []*LifecyclePolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*LifecyclePolicy
	for _, policy := range m.policies {
		if enabled != nil && policy.Enabled != *enabled {
			continue
		}
		result = append(result, policy)
	}
	return result
}

// UpdatePolicy 更新策略
func (m *Manager) UpdatePolicy(id string, policy LifecyclePolicy) (*LifecyclePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略 %s 不存在", id)
	}

	policy.ID = id
	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = time.Now()
	m.policies[id] = &policy

	m.addAuditEntry("update_policy", id, fmt.Sprintf("更新策略: %s", policy.Name), true)

	log.Printf("[数据生命周期] 更新策略: %s", id)
	return &policy, nil
}

// DeletePolicy 删除策略
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return fmt.Errorf("策略 %s 不存在", id)
	}

	delete(m.policies, id)
	m.addAuditEntry("delete_policy", id, "删除策略", true)

	log.Printf("[数据生命周期] 删除策略: %s", id)
	return nil
}

// ============================================================
// 数据记录管理
// ============================================================

// CreateRecord 创建数据记录
func (m *Manager) CreateRecord(record DataRecord) (*DataRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	if _, exists := m.records[record.ID]; exists {
		return nil, fmt.Errorf("记录 %s 已存在", record.ID)
	}

	record.CreatedAt = time.Now()
	record.ModifiedAt = time.Now()
	record.LastAccessedAt = time.Now()
	record.CurrentPhase = PhaseActive
	record.CurrentTier = TierHot
	m.records[record.ID] = &record

	m.addAuditEntry("create_record", record.ID, fmt.Sprintf("创建记录: %s", record.Path), true)

	log.Printf("[数据生命周期] 创建记录: %s - %s", record.ID, record.Path)
	return &record, nil
}

// GetRecord 获取数据记录
func (m *Manager) GetRecord(id string) (*DataRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, exists := m.records[id]
	if !exists {
		return nil, fmt.Errorf("记录 %s 不存在", id)
	}
	return record, nil
}

// ListRecords 列出数据记录
func (m *Manager) ListRecords(phase LifecyclePhase, tier StorageTier) []*DataRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*DataRecord
	for _, record := range m.records {
		if phase != "" && record.CurrentPhase != phase {
			continue
		}
		if tier != "" && record.CurrentTier != tier {
			continue
		}
		result = append(result, record)
	}
	return result
}

// ============================================================
// 阶段转换
// ============================================================

// TransitionPhase 转换阶段
func (m *Manager) TransitionPhase(recordID string, targetPhase LifecyclePhase, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, exists := m.records[recordID]
	if !exists {
		return fmt.Errorf("记录 %s 不存在", recordID)
	}

	// 验证转换合法性
	fromOrder, fromExists := PhaseOrder[record.CurrentPhase]
	toOrder, toExists := PhaseOrder[targetPhase]
	if !fromExists || !toExists {
		return fmt.Errorf("无效的阶段转换")
	}

	// 阶段只能向前或保持
	if toOrder < fromOrder {
		return fmt.Errorf("不能从 %s 回退到 %s", record.CurrentPhase, targetPhase)
	}

	// 记录转换历史
	transition := PhaseTransition{
		FromPhase: record.CurrentPhase,
		ToPhase:   targetPhase,
		Timestamp: time.Now(),
		Reason:    reason,
	}

	record.PhaseHistory = append(record.PhaseHistory, transition)
	record.CurrentPhase = targetPhase
	record.ModifiedAt = time.Now()

	// 根据阶段更新存储层
	switch targetPhase {
	case PhaseActive:
		record.CurrentTier = TierHot
	case PhaseReference:
		record.CurrentTier = TierWarm
	case PhaseArchive:
		record.CurrentTier = TierCold
	case PhaseRetained, PhaseExpired:
		record.CurrentTier = TierArchive
	}

	m.addAuditEntry("phase_change", recordID, fmt.Sprintf("阶段转换: %s -> %s, 原因: %s", transition.FromPhase, targetPhase, reason), true)

	log.Printf("[数据生命周期] 阶段转换: %s: %s -> %s", recordID, record.CurrentPhase, targetPhase)
	return nil
}

// ============================================================
// 合规保留
// ============================================================

// CreateHold 创建合规保留
func (m *Manager) CreateHold(hold ComplianceHold) (*ComplianceHold, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if hold.ID == "" {
		hold.ID = uuid.New().String()
	}

	if _, exists := m.holds[hold.ID]; exists {
		return nil, fmt.Errorf("合规保留 %s 已存在", hold.ID)
	}

	hold.CreatedAt = time.Now()
	hold.Active = true
	m.holds[hold.ID] = &hold

	// 更新关联记录
	for _, path := range hold.FilePaths {
		for _, record := range m.records {
			if record.Path == path {
				record.HoldIDs = append(record.HoldIDs, hold.ID)
			}
		}
	}

	m.addAuditEntry("hold_create", hold.ID, fmt.Sprintf("创建合规保留: %s", hold.Name), true)

	log.Printf("[数据生命周期] 创建合规保留: %s - %s", hold.ID, hold.Name)
	return &hold, nil
}

// ReleaseHold 释放合规保留
func (m *Manager) ReleaseHold(id string, releasedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hold, exists := m.holds[id]
	if !exists {
		return fmt.Errorf("合规保留 %s 不存在", id)
	}

	if !hold.Active {
		return fmt.Errorf("合规保留 %s 已释放", id)
	}

	now := time.Now()
	hold.Active = false
	hold.ReleasedAt = &now
	hold.ReleasedBy = releasedBy

	// 更新关联记录
	for _, record := range m.records {
		var newHoldIDs []string
		for _, holdID := range record.HoldIDs {
			if holdID != id {
				newHoldIDs = append(newHoldIDs, holdID)
			}
		}
		record.HoldIDs = newHoldIDs
	}

	m.addAuditEntry("hold_release", id, fmt.Sprintf("释放合规保留, 操作人: %s", releasedBy), true)

	log.Printf("[数据生命周期] 释放合规保留: %s", id)
	return nil
}

// ListHolds 列出合规保留
func (m *Manager) ListHolds(active *bool) []*ComplianceHold {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ComplianceHold
	for _, hold := range m.holds {
		if active != nil && hold.Active != *active {
			continue
		}
		result = append(result, hold)
	}
	return result
}

// ============================================================
// 数据迁移
// ============================================================

// CreateMigration 创建迁移任务
func (m *Manager) CreateMigration(migration DataMigration) (*DataMigration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if migration.ID == "" {
		migration.ID = uuid.New().String()
	}

	migration.Status = MigrationPending
	migration.CreatedAt = time.Now()
	m.migrations[migration.ID] = &migration

	m.addAuditEntry("migrate", migration.ID, fmt.Sprintf("创建迁移任务: %s -> %s", migration.SourceTier, migration.TargetTier), true)

	log.Printf("[数据生命周期] 创建迁移任务: %s", migration.ID)
	return &migration, nil
}

// StartMigration 启动迁移
func (m *Manager) StartMigration(id string) error {
	m.mu.Lock()
	migration, exists := m.migrations[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("迁移任务 %s 不存在", id)
	}

	if migration.Status != MigrationPending {
		m.mu.Unlock()
		return fmt.Errorf("迁移任务 %s 状态不是待执行", id)
	}

	migration.Status = MigrationRunning
	migration.StartedAt = time.Now()
	m.mu.Unlock()

	// 异步执行迁移
	go m.executeMigration(id)

	log.Printf("[数据生命周期] 启动迁移: %s", id)
	return nil
}

// executeMigration 执行迁移
func (m *Manager) executeMigration(id string) {
	m.mu.RLock()
	migration, exists := m.migrations[id]
	if !exists {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	// 模拟迁移过程
	for i := 0; i < len(migration.Files); i++ {
		m.mu.Lock()
		migration.ProcessedFiles++
		migration.ProcessedBytes += migration.Files[i].Size
		migration.Files[i].Status = "completed"
		m.mu.Unlock()

		time.Sleep(10 * time.Millisecond) // 模拟处理时间
	}

	m.mu.Lock()
	migration.Status = MigrationCompleted
	migration.CompletedAt = time.Now()
	m.mu.Unlock()

	m.addAuditEntry("migrate", id, "迁移完成", true)

	log.Printf("[数据生命周期] 迁移完成: %s", id)
}

// GetMigration 获取迁移任务
func (m *Manager) GetMigration(id string) (*DataMigration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	migration, exists := m.migrations[id]
	if !exists {
		return nil, fmt.Errorf("迁移任务 %s 不存在", id)
	}
	return migration, nil
}

// ListMigrations 列出迁移任务
func (m *Manager) ListMigrations(status MigrationStatus) []*DataMigration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*DataMigration
	for _, migration := range m.migrations {
		if status != "" && migration.Status != status {
			continue
		}
		result = append(result, migration)
	}
	return result
}

// ============================================================
// 数据销毁
// ============================================================

// CreateDestruction 创建销毁记录
func (m *Manager) CreateDestruction(destruction DestructionRecord) (*DestructionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if destruction.ID == "" {
		destruction.ID = uuid.New().String()
	}

	destruction.Status = DestructionPending
	destruction.CreatedAt = time.Now()

	// 检查合规保留
	for _, filePath := range destruction.FilePaths {
		for _, hold := range m.holds {
			if hold.Active {
				for _, holdPath := range hold.FilePaths {
					if holdPath == filePath {
						destruction.HoldID = hold.ID
						destruction.RequiresApproval = true
						break
					}
				}
			}
		}
	}

	m.destructions[destruction.ID] = &destruction

	m.addAuditEntry("destroy", destruction.ID, fmt.Sprintf("创建销毁记录, 文件数: %d", len(destruction.FilePaths)), true)

	log.Printf("[数据生命周期] 创建销毁记录: %s", destruction.ID)
	return &destruction, nil
}

// ApproveDestruction 批准销毁
func (m *Manager) ApproveDestruction(id string, approvedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	destruction, exists := m.destructions[id]
	if !exists {
		return fmt.Errorf("销毁记录 %s 不存在", id)
	}

	if destruction.Status != DestructionPending {
		return fmt.Errorf("销毁记录 %s 状态不是待处理", id)
	}

	now := time.Now()
	destruction.Status = DestructionApproved
	destruction.ApprovedAt = &now
	destruction.ApprovedBy = approvedBy

	m.addAuditEntry("destroy", id, fmt.Sprintf("批准销毁, 操作人: %s", approvedBy), true)

	log.Printf("[数据生命周期] 批准销毁: %s", id)
	return nil
}

// ExecuteDestruction 执行销毁
func (m *Manager) ExecuteDestruction(id string) error {
	m.mu.Lock()
	destruction, exists := m.destructions[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("销毁记录 %s 不存在", id)
	}

	if destruction.Status != DestructionApproved {
		m.mu.Unlock()
		return fmt.Errorf("销毁记录 %s 未批准", id)
	}

	destruction.Status = DestructionInProgress
	m.mu.Unlock()

	// 模拟销毁过程
	time.Sleep(100 * time.Millisecond)

	m.mu.Lock()
	destruction.Status = DestructionCompleted
	destruction.DestroyedSize = destruction.TotalSize
	now := time.Now()
	destruction.CompletedAt = &now

	// 生成销毁证书
	cert := &DestructionCertification{
		ID:            uuid.New().String(),
		DestructionID: id,
		IssuedAt:      now,
		Method:        destruction.Method,
		FileCount:     len(destruction.FilePaths),
		TotalSize:     destruction.TotalSize,
		VerifiedBy:    "system",
		Signature:     "mock-signature",
	}
	destruction.Certification = cert
	m.mu.Unlock()

	m.addAuditEntry("destroy", id, "销毁完成", true)

	log.Printf("[数据生命周期] 销毁完成: %s", id)
	return nil
}

// GetDestruction 获取销毁记录
func (m *Manager) GetDestruction(id string) (*DestructionRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	destruction, exists := m.destructions[id]
	if !exists {
		return nil, fmt.Errorf("销毁记录 %s 不存在", id)
	}
	return destruction, nil
}

// ============================================================
// 策略模板
// ============================================================

// CreateTemplate 创建策略模板
func (m *Manager) CreateTemplate(template PolicyTemplate) (*PolicyTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if template.ID == "" {
		template.ID = uuid.New().String()
	}

	if _, exists := m.templates[template.ID]; exists {
		return nil, fmt.Errorf("模板 %s 已存在", template.ID)
	}

	template.CreatedAt = time.Now()
	m.templates[template.ID] = &template

	log.Printf("[数据生命周期] 创建模板: %s - %s", template.ID, template.Name)
	return &template, nil
}

// ListTemplates 列出策略模板
func (m *Manager) ListTemplates() []*PolicyTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*PolicyTemplate
	for _, template := range m.templates {
		result = append(result, template)
	}
	return result
}

// ============================================================
// 批量操作
// ============================================================

// BatchApplyPolicy 批量应用策略
func (m *Manager) BatchApplyPolicy(req BatchApplyRequest) (*BatchApplyResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[req.PolicyID]
	if !exists {
		return nil, fmt.Errorf("策略 %s 不存在", req.PolicyID)
	}

	result := &BatchApplyResult{}

	// 查找匹配的记录
	for _, record := range m.records {
		matched := false
		for _, path := range req.Paths {
			if record.Path == path {
				matched = true
				break
			}
		}

		if matched {
			result.TotalFiles++
			if record.PolicyID != "" && !req.Force {
				result.SkippedFiles++
			} else {
				record.PolicyID = policy.ID
				record.ModifiedAt = time.Now()
				result.AppliedFiles++
			}
		}
	}

	m.addAuditEntry("apply_policy", req.PolicyID, fmt.Sprintf("批量应用策略, 成功: %d, 跳过: %d", result.AppliedFiles, result.SkippedFiles), true)

	log.Printf("[数据生命周期] 批量应用策略: %s, 结果: %+v", req.PolicyID, result)
	return result, nil
}

// ============================================================
// 访问分析
// ============================================================

// GenerateAccessReport 生成访问分析报告
func (m *Manager) GenerateAccessReport() *AccessAnalysisReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &AccessAnalysisReport{
		GeneratedAt: time.Now(),
		TierStats:   make(map[StorageTier]*TierStatistics),
		PhaseStats:  make(map[LifecyclePhase]int),
	}

	// 统计各层信息
	for _, record := range m.records {
		report.TotalFiles++
		report.TotalSize += record.Size
		report.PhaseStats[record.CurrentPhase]++

		if _, exists := report.TierStats[record.CurrentTier]; !exists {
			report.TierStats[record.CurrentTier] = &TierStatistics{
				Tier: record.CurrentTier,
			}
		}
		tierStat := report.TierStats[record.CurrentTier]
		tierStat.FileCount++
		tierStat.TotalSize += record.Size
	}

	// 生成分层建议
	for _, record := range m.records {
		if record.AccessCount < 10 && record.CurrentTier == TierHot {
			report.Suggestions = append(report.Suggestions, TierSuggestion{
				Path:            record.Path,
				CurrentTier:     record.CurrentTier,
				RecommendedTier: TierWarm,
				Reason:          "访问频率低，建议迁移到温存储",
				Priority:        1,
			})
		}
	}

	log.Printf("[数据生命周期] 生成访问分析报告, 文件数: %d", report.TotalFiles)
	return report
}

// ============================================================
// 审计日志
// ============================================================

// addAuditEntry 添加审计日志
func (m *Manager) addAuditEntry(action, target, details string, success bool) {
	entry := LifecycleAuditEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Action:    action,
		Target:    target,
		Details:   details,
		Operator:  "system",
		Success:   success,
	}
	m.auditLog = append(m.auditLog, entry)
}

// GetAuditLog 获取审计日志
func (m *Manager) GetAuditLog(limit int) []LifecycleAuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.auditLog) {
		limit = len(m.auditLog)
	}

	// 返回最近的日志
	start := len(m.auditLog) - limit
	if start < 0 {
		start = 0
	}
	return m.auditLog[start:]
}

// ============================================================
// 状态查询
// ============================================================

// GetStatus 获取模块状态
func (m *Manager) GetStatus() *LifecycleStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &LifecycleStatus{
		Enabled:           m.enabled,
		TotalPolicies:     len(m.policies),
		TotalRecords:      len(m.records),
		PhaseDistribution: make(map[LifecyclePhase]int),
		TierDistribution:  make(map[StorageTier]int),
	}

	// 统计活跃策略
	for _, policy := range m.policies {
		if policy.Enabled {
			status.ActivePolicies++
		}
	}

	// 统计活跃保留
	for _, hold := range m.holds {
		if hold.Active {
			status.ActiveHolds++
		}
	}

	// 统计运行中的迁移
	for _, migration := range m.migrations {
		if migration.Status == MigrationRunning {
			status.RunningMigrations++
		}
	}

	// 统计待销毁
	for _, destruction := range m.destructions {
		if destruction.Status == DestructionPending || destruction.Status == DestructionApproved {
			status.PendingDestructions++
		}
	}

	// 阶段和层级分布
	for _, record := range m.records {
		status.PhaseDistribution[record.CurrentPhase]++
		status.TierDistribution[record.CurrentTier]++
	}

	return status
}
