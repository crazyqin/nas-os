package smartdatalifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 智能数据生命周期管理器
// 协调归档、清理、迁移和保留策略
type Manager struct {
	mu     sync.RWMutex
	config Config
	logger *zap.Logger

	// 子模块
	archiver     *Archiver
	cleaner      *Cleaner
	migrator     *Migrator
	retentionMgr *RetentionManager
	deduplicator *Deduplicator

	// 存储
	dataItems       map[string]*DataItem        // id -> item
	policies        map[string]*RetentionPolicy // id -> policy
	archivePolicies map[string]*ArchivePolicy   // id -> policy
	cleanupRules    map[string]*CleanupRule     // id -> rule
	events          []*LifecycleEvent
	migrationTasks  map[string]*MigrationTask // id -> task

	// 状态
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// 统计
	stats *LifecycleStats
}

// NewManager 创建生命周期管理器
func NewManager(config Config, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		config:          config,
		logger:          logger,
		dataItems:       make(map[string]*DataItem),
		policies:        make(map[string]*RetentionPolicy),
		archivePolicies: make(map[string]*ArchivePolicy),
		cleanupRules:    make(map[string]*CleanupRule),
		events:          make([]*LifecycleEvent, 0),
		migrationTasks:  make(map[string]*MigrationTask),
		stats:           &LifecycleStats{LastUpdated: time.Now()},
		stopCh:          make(chan struct{}),
	}

	// 初始化子模块
	m.archiver = NewArchiver(config.Archive, m, logger)
	m.cleaner = NewCleaner(config.Cleanup, m, logger)
	m.migrator = NewMigrator(config.Migration, m, logger)
	m.retentionMgr = NewRetentionManager(config.Retention, m, logger)
	m.deduplicator = NewDeduplicator(config.Dedup, m, logger)

	return m
}

// Start 启动生命周期管理器
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("lifecycle manager already running")
	}
	m.running = true
	m.mu.Unlock()

	// 启动各子模块
	if m.config.Archive.Enabled {
		m.wg.Add(1)
		go m.runArchiver(ctx)
	}

	if m.config.Cleanup.Enabled {
		m.wg.Add(1)
		go m.runCleaner(ctx)
	}

	if m.config.Migration.Enabled {
		m.wg.Add(1)
		go m.runMigrator(ctx)
	}

	if m.config.Retention.Enabled {
		m.wg.Add(1)
		go m.runRetentionChecker(ctx)
	}

	if m.config.Dedup.Enabled {
		m.wg.Add(1)
		go m.runDeduplicator(ctx)
	}

	// 统计更新
	m.wg.Add(1)
	go m.runStatsUpdater(ctx)

	m.logger.Info("smart data lifecycle manager started")
	return nil
}

// Stop 停止生命周期管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopCh)
	m.mu.Unlock()

	m.wg.Wait()
	m.logger.Info("smart data lifecycle manager stopped")
}

// IsRunning 是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetConfig 获取配置
func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// ============================================================
// 数据项管理
// ============================================================

// RegisterItem 注册数据项
func (m *Manager) RegisterItem(item *DataItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if item.ID == "" {
		return fmt.Errorf("item ID is required")
	}

	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	if item.ModifiedAt.IsZero() {
		item.ModifiedAt = time.Now()
	}
	if item.AccessedAt.IsZero() {
		item.AccessedAt = time.Now()
	}
	if item.Stage == "" {
		item.Stage = StageActive
	}

	m.dataItems[item.ID] = item
	m.logger.Debug("data item registered", zap.String("id", item.ID), zap.String("path", item.Path))
	return nil
}

// GetItem 获取数据项
func (m *Manager) GetItem(id string) (*DataItem, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.dataItems[id]
	return item, ok
}

// ListItems 列出数据项
func (m *Manager) ListItems(stage LifecycleStage, limit, offset int) []*DataItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DataItem, 0)
	count := 0
	for _, item := range m.dataItems {
		if stage != "" && item.Stage != stage {
			continue
		}
		count++
		if count <= offset {
			continue
		}
		result = append(result, item)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// UpdateItemStage 更新数据项阶段
func (m *Manager) UpdateItemStage(id string, newStage LifecycleStage, triggeredBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.dataItems[id]
	if !ok {
		return fmt.Errorf("item not found: %s", id)
	}

	// 法律冻结检查
	if item.LegalHold && newStage == StageDeleted {
		return fmt.Errorf("item %s is under legal hold, cannot delete", id)
	}

	// 合规策略检查
	if item.RetentionPolicyID != "" {
		policy, ok := m.policies[item.RetentionPolicyID]
		if ok && policy.CompliancePolicy {
			if newStage == StageDeleted && item.ExpiresAt != nil && time.Now().Before(*item.ExpiresAt) {
				return fmt.Errorf("item %s is under compliance policy, cannot delete before expiration", id)
			}
		}
	}

	oldStage := item.Stage
	item.Stage = newStage

	if newStage == StageArchive {
		now := time.Now()
		item.ArchivedAt = &now
	}
	if newStage == StageDeleted {
		now := time.Now()
		item.DeletedAt = &now
	}

	// 记录事件
	event := &LifecycleEvent{
		ID:          fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		EventType:   m.stageToEventType(newStage),
		ItemID:      id,
		ItemPath:    item.Path,
		OldStage:    oldStage,
		NewStage:    newStage,
		TriggeredBy: triggeredBy,
		CreatedAt:   time.Now(),
	}
	m.events = append(m.events, event)

	m.logger.Info("item stage updated",
		zap.String("id", id),
		zap.String("old_stage", string(oldStage)),
		zap.String("new_stage", string(newStage)),
		zap.String("triggered_by", triggeredBy))

	return nil
}

// RecordAccess 记录访问
func (m *Manager) RecordAccess(id string, opType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.dataItems[id]
	if !ok {
		return fmt.Errorf("item not found: %s", id)
	}

	item.AccessedAt = time.Now()
	item.AccessCount++

	switch opType {
	case "read":
		item.ReadCount++
	case "write":
		item.WriteCount++
		item.ModifiedAt = time.Now()
	}

	// 如果在归档/冷阶段被访问，可能需要升级
	if item.Stage == StageCold || item.Stage == StageArchive {
		m.logger.Info("cold/archive data accessed, may need promotion",
			zap.String("id", id),
			zap.String("stage", string(item.Stage)))
	}

	return nil
}

// stageToEventType 阶段到事件类型映射
func (m *Manager) stageToEventType(stage LifecycleStage) EventType {
	switch stage {
	case StageArchive:
		return EventArchived
	case StageDeleted:
		return EventCleaned
	case StageExpired:
		return EventExpired
	default:
		return EventMigrated
	}
}

// ============================================================
// 策略管理
// ============================================================

// AddRetentionPolicy 添加保留策略
func (m *Manager) AddRetentionPolicy(policy *RetentionPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
}

// GetRetentionPolicy 获取保留策略
func (m *Manager) GetRetentionPolicy(id string) (*RetentionPolicy, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	policy, ok := m.policies[id]
	return policy, ok
}

// ListRetentionPolicies 列出保留策略
func (m *Manager) ListRetentionPolicies() []*RetentionPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*RetentionPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		result = append(result, p)
	}
	return result
}

// UpdateRetentionPolicy 更新保留策略
func (m *Manager) UpdateRetentionPolicy(policy *RetentionPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[policy.ID]; !ok {
		return fmt.Errorf("policy not found: %s", policy.ID)
	}

	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
	return nil
}

// DeleteRetentionPolicy 删除保留策略
func (m *Manager) DeleteRetentionPolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("policy not found: %s", id)
	}

	// 检查是否有数据项引用此策略
	for _, item := range m.dataItems {
		if item.RetentionPolicyID == id {
			return fmt.Errorf("policy %s is still in use by item %s", id, item.ID)
		}
	}

	delete(m.policies, id)
	return nil
}

// AddArchivePolicy 添加归档策略
func (m *Manager) AddArchivePolicy(policy *ArchivePolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.archivePolicies[policy.ID] = policy
}

// GetArchivePolicy 获取归档策略
func (m *Manager) GetArchivePolicy(id string) (*ArchivePolicy, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	policy, ok := m.archivePolicies[id]
	return policy, ok
}

// ListArchivePolicies 列出归档策略
func (m *Manager) ListArchivePolicies() []*ArchivePolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ArchivePolicy, 0, len(m.archivePolicies))
	for _, p := range m.archivePolicies {
		result = append(result, p)
	}
	return result
}

// DeleteArchivePolicy 删除归档策略
func (m *Manager) DeleteArchivePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.archivePolicies[id]; !ok {
		return fmt.Errorf("archive policy not found: %s", id)
	}
	delete(m.archivePolicies, id)
	return nil
}

// AddCleanupRule 添加清理规则
func (m *Manager) AddCleanupRule(rule *CleanupRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.cleanupRules[rule.ID] = rule
}

// GetCleanupRule 获取清理规则
func (m *Manager) GetCleanupRule(id string) (*CleanupRule, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rule, ok := m.cleanupRules[id]
	return rule, ok
}

// ListCleanupRules 列出清理规则
func (m *Manager) ListCleanupRules() []*CleanupRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*CleanupRule, 0, len(m.cleanupRules))
	for _, r := range m.cleanupRules {
		result = append(result, r)
	}
	return result
}

// DeleteCleanupRule 删除清理规则
func (m *Manager) DeleteCleanupRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.cleanupRules[id]; !ok {
		return fmt.Errorf("cleanup rule not found: %s", id)
	}
	delete(m.cleanupRules, id)
	return nil
}

// ============================================================
// 迁移任务管理
// ============================================================

// AddMigrationTask 添加迁移任务
func (m *Manager) AddMigrationTask(task *MigrationTask) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task.CreatedAt = time.Now()
	task.Status = MigrationPending
	m.migrationTasks[task.ID] = task
}

// GetMigrationTask 获取迁移任务
func (m *Manager) GetMigrationTask(id string) (*MigrationTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.migrationTasks[id]
	return task, ok
}

// ListMigrationTasks 列出迁移任务
func (m *Manager) ListMigrationTasks(status MigrationStatus, limit, offset int) []*MigrationTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MigrationTask, 0)
	count := 0
	for _, task := range m.migrationTasks {
		if status != "" && task.Status != status {
			continue
		}
		count++
		if count <= offset {
			continue
		}
		result = append(result, task)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// ============================================================
// 事件查询
// ============================================================

// GetEvents 获取事件列表
func (m *Manager) GetEvents(limit, offset int) []*LifecycleEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if offset >= len(m.events) {
		return []*LifecycleEvent{}
	}

	end := offset + limit
	if end > len(m.events) || limit <= 0 {
		end = len(m.events)
	}

	// 返回最近的事件（倒序）
	result := make([]*LifecycleEvent, 0, end-offset)
	for i := len(m.events) - 1 - offset; i >= 0 && len(result) < end-offset; i-- {
		result = append(result, m.events[i])
	}
	return result
}

// ============================================================
// 统计
// ============================================================

// GetStats 获取统计信息
func (m *Manager) GetStats() *LifecycleStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// updateStats 更新统计信息
func (m *Manager) updateStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := &LifecycleStats{
		StageDistribution: make(map[LifecycleStage]int64),
		StageSizes:        make(map[LifecycleStage]int64),
		LastUpdated:       time.Now(),
	}

	for _, item := range m.dataItems {
		stats.TotalItems++
		stats.TotalSize += item.Size
		stats.StageDistribution[item.Stage]++
		stats.StageSizes[item.Stage] += item.Size

		// 保留策略统计
		if item.ExpiresAt != nil {
			daysUntilExpiry := time.Until(*item.ExpiresAt).Hours() / 24
			if daysUntilExpiry <= 7 {
				stats.ExpiringThisWeek++
			}
			if daysUntilExpiry <= 30 {
				stats.ExpiringThisMonth++
			}
		}
		if item.LegalHold {
			stats.LegalHolds++
		}
	}

	// 迁移统计
	for _, task := range m.migrationTasks {
		switch task.Status {
		case MigrationPending:
			stats.MigrationsPending++
		case MigrationRunning:
			stats.MigrationsRunning++
		}
		stats.TotalMigrations++
	}

	m.stats = stats
}

// ============================================================
// 后台任务
// ============================================================

func (m *Manager) runArchiver(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Duration(m.config.Archive.CheckIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			if err := m.archiver.Run(ctx); err != nil {
				m.logger.Error("archiver run failed", zap.Error(err))
			}
		}
	}
}

func (m *Manager) runCleaner(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Duration(m.config.Cleanup.CheckIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			if err := m.cleaner.Run(ctx); err != nil {
				m.logger.Error("cleaner run failed", zap.Error(err))
			}
		}
	}
}

func (m *Manager) runMigrator(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Duration(m.config.Migration.CheckIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			if err := m.migrator.Run(ctx); err != nil {
				m.logger.Error("migrator run failed", zap.Error(err))
			}
		}
	}
}

func (m *Manager) runRetentionChecker(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Duration(m.config.Retention.CheckIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			if err := m.retentionMgr.Run(ctx); err != nil {
				m.logger.Error("retention checker run failed", zap.Error(err))
			}
		}
	}
}

func (m *Manager) runDeduplicator(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Duration(m.config.Dedup.CheckIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			if err := m.deduplicator.Run(ctx); err != nil {
				m.logger.Error("deduplicator run failed", zap.Error(err))
			}
		}
	}
}

func (m *Manager) runStatsUpdater(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.updateStats()
		}
	}
}
