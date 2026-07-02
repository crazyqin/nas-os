// 备份生命周期管理器
package smartlifebackup

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Manager 备份生命周期管理器.
type Manager struct {
	mu sync.RWMutex

	// 配置
	configPath  string
	storagePath string
	dataPath    string

	// 策略
	policies map[string]*BackupPolicy

	// 备份项
	backups map[string]*BackupItem

	// 任务
	tasks map[string]*LifecycleTask

	// 调度器
	scheduler *Scheduler

	// 成本计算器
	costCalc *CostCalculator

	// 当前活跃策略ID
	activePolicyID string
}

// NewManager 创建备份生命周期管理器.
func NewManager(configPath, storagePath string) *Manager {
	return &Manager{
		configPath:  configPath,
		storagePath: storagePath,
		dataPath:    filepath.Join(configPath, "smartlifebackup"),
		policies:    make(map[string]*BackupPolicy),
		backups:     make(map[string]*BackupItem),
		tasks:       make(map[string]*LifecycleTask),
	}
}

// Initialize 初始化管理器.
func (m *Manager) Initialize() error {
	// 创建数据目录
	if err := os.MkdirAll(m.dataPath, 0750); err != nil {
		return fmt.Errorf("创建数据目录失败：%w", err)
	}

	// 加载数据
	if err := m.loadData(); err != nil {
		// 数据不存在是正常的
		m.policies = make(map[string]*BackupPolicy)
		m.backups = make(map[string]*BackupItem)
		m.tasks = make(map[string]*LifecycleTask)
	}

	// 如果没有策略，创建默认策略
	if len(m.policies) == 0 {
		defaultPolicy := DefaultBackupPolicy()
		m.policies[defaultPolicy.ID] = defaultPolicy
		m.activePolicyID = defaultPolicy.ID
	}

	// 初始化调度器
	m.scheduler = NewScheduler(DefaultScheduleConfig())

	// 初始化成本计算器
	m.costCalc = NewCostCalculator(DefaultStorageCost())

	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() error {
	if m.scheduler != nil {
		m.scheduler.Stop()
	}
	return m.saveData()
}

// ============================================================================
// 策略管理
// ============================================================================

// ListPolicies 列出所有策略.
func (m *Manager) ListPolicies() []*BackupPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*BackupPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// GetPolicy 获取策略.
func (m *Manager) GetPolicy(id string) (*BackupPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("策略不存在：%s", id)
	}
	return policy, nil
}

// CreatePolicy 创建策略.
func (m *Manager) CreatePolicy(policy *BackupPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = generateID()
	}

	if _, exists := m.policies[policy.ID]; exists {
		return fmt.Errorf("策略已存在：%s", policy.ID)
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	m.policies[policy.ID] = policy
	return m.saveData()
}

// UpdatePolicy 更新策略.
func (m *Manager) UpdatePolicy(id string, policy *BackupPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return fmt.Errorf("策略不存在：%s", id)
	}

	policy.ID = id
	policy.UpdatedAt = time.Now()
	m.policies[id] = policy
	return m.saveData()
}

// DeletePolicy 删除策略.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return fmt.Errorf("策略不存在：%s", id)
	}

	// 不能删除活跃策略
	if m.activePolicyID == id {
		return fmt.Errorf("不能删除活跃策略")
	}

	delete(m.policies, id)
	return m.saveData()
}

// SetActivePolicy 设置活跃策略.
func (m *Manager) SetActivePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return fmt.Errorf("策略不存在：%s", id)
	}

	m.activePolicyID = id
	return m.saveData()
}

// GetActivePolicy 获取当前活跃策略.
func (m *Manager) GetActivePolicy() (*BackupPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.activePolicyID == "" {
		return nil, fmt.Errorf("未设置活跃策略")
	}

	policy, ok := m.policies[m.activePolicyID]
	if !ok {
		return nil, fmt.Errorf("活跃策略不存在")
	}
	return policy, nil
}

// ============================================================================
// 备份项管理
// ============================================================================

// RegisterBackup 注册备份项.
func (m *Manager) RegisterBackup(item *BackupItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if item.ID == "" {
		item.ID = generateID()
	}

	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}

	// 根据策略设置过期时间
	policy, ok := m.policies[m.activePolicyID]
	if ok && len(policy.RetentionRules) > 0 {
		// 使用第一条规则的保留天数设置初始过期时间
		item.ExpiresAt = item.CreatedAt.AddDate(0, 0, policy.RetentionRules[0].RetainDays)
	}

	m.backups[item.ID] = item
	return m.saveData()
}

// GetBackup 获取备份项.
func (m *Manager) GetBackup(id string) (*BackupItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.backups[id]
	if !ok {
		return nil, fmt.Errorf("备份项不存在：%s", id)
	}
	return item, nil
}

// ListBackups 列出所有备份项.
func (m *Manager) ListBackups() []*BackupItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backups := make([]*BackupItem, 0, len(m.backups))
	for _, b := range m.backups {
		backups = append(backups, b)
	}
	return backups
}

// DeleteBackup 删除备份项.
func (m *Manager) DeleteBackup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.backups[id]; !exists {
		return fmt.Errorf("备份项不存在：%s", id)
	}

	delete(m.backups, id)
	return m.saveData()
}

// ============================================================================
// 生命周期执行
// ============================================================================

// ExecuteLifecycle 执行生命周期管理.
func (m *Manager) ExecuteLifecycle(ctx context.Context, options *ExecuteOptions) (*LifecycleTask, error) {
	m.mu.RLock()
	policy, ok := m.policies[m.activePolicyID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("未设置活跃策略")
	}

	// 创建主任务
	task := &LifecycleTask{
		ID:        generateID(),
		Type:      TaskTypeRetention,
		Status:    TaskStatusRunning,
		StartTime: time.Now(),
		Metadata: map[string]interface{}{
			"dry_run": options != nil && options.DryRun,
		},
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 在后台执行
	go m.executeLifecycleTask(ctx, policy, task, options)

	return task, nil
}

// executeLifecycleTask 执行生命周期任务.
func (m *Manager) executeLifecycleTask(ctx context.Context, policy *BackupPolicy, task *LifecycleTask, options *ExecuteOptions) {
	defer func() {
		task.EndTime = time.Now()
		if task.Status == TaskStatusRunning {
			task.Status = TaskStatusCompleted
		}
		m.saveData()
	}()

	dryRun := options != nil && options.DryRun

	// 步骤1: 应用保留策略
	m.applyRetentionPolicy(ctx, policy, task, dryRun)

	// 步骤2: 存储层级迁移
	m.migrateStorageTiers(ctx, policy, task, dryRun)

	// 步骤3: 压缩优化
	if policy.CompressionType != CompressionNone {
		m.optimizeCompression(ctx, policy, task, dryRun)
	}

	// 步骤4: 去重处理
	if policy.Deduplication {
		m.deduplicateBackups(ctx, task, dryRun)
	}

	// 步骤5: 过期清理
	m.cleanupExpired(ctx, task, dryRun)
}

// applyRetentionPolicy 应用保留策略.
func (m *Manager) applyRetentionPolicy(ctx context.Context, policy *BackupPolicy, task *LifecycleTask, dryRun bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for _, rule := range policy.RetentionRules {
		// 获取符合此规则的备份
		var matching []*BackupItem
		for _, item := range m.backups {
			if m.matchesRule(item, rule, now) {
				matching = append(matching, item)
			}
		}

		// 按创建时间排序
		sort.Slice(matching, func(i, j int) bool {
			return matching[i].CreatedAt.After(matching[j].CreatedAt)
		})

		// 如果超过保留数量，删除最旧的
		if rule.KeepCount > 0 && len(matching) > rule.KeepCount {
			toDelete := matching[rule.KeepCount:]
			for _, item := range toDelete {
				if !dryRun {
					item.ExpiresAt = now
				}
				log.Printf("[SmartLifeBackup] 标记过期：%s (规则：%s)", item.Name, rule.Name)
			}
		}

		// 更新存储层级
		for _, item := range matching {
			if !dryRun {
				item.Tier = rule.StorageTier
			}
		}
	}
}

// migrateStorageTiers 迁移存储层级.
func (m *Manager) migrateStorageTiers(ctx context.Context, policy *BackupPolicy, task *LifecycleTask, dryRun bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for _, item := range m.backups {
		age := now.Sub(item.CreatedAt)

		// 根据年龄决定存储层级
		var newTier StorageTier
		switch {
		case age < 7*24*time.Hour:
			newTier = StorageTierHot
		case age < 30*24*time.Hour:
			newTier = StorageTierWarm
		case age < 365*24*time.Hour:
			newTier = StorageTierCold
		default:
			newTier = StorageTierArchive
		}

		if item.Tier != newTier {
			if !dryRun {
				oldTier := item.Tier
				item.Tier = newTier
				log.Printf("[SmartLifeBackup] 迁移：%s %s -> %s", item.Name, oldTier, newTier)
			}
		}
	}
}

// optimizeCompression 优化压缩.
func (m *Manager) optimizeCompression(ctx context.Context, policy *BackupPolicy, task *LifecycleTask, dryRun bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range m.backups {
		if !item.Compressed && item.Size > 0 {
			if !dryRun {
				item.Compressed = true
				// 模拟压缩效果
				item.CompressSize = int64(float64(item.Size) * 0.6)
				log.Printf("[SmartLifeBackup] 压缩：%s %.2fMB -> %.2fMB",
					item.Name,
					float64(item.Size)/(1024*1024),
					float64(item.CompressSize)/(1024*1024))
			}
		}
	}
}

// deduplicateBackups 去重处理.
func (m *Manager) deduplicateBackups(ctx context.Context, task *LifecycleTask, dryRun bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 按校验和分组
	checksumMap := make(map[string][]*BackupItem)
	for _, item := range m.backups {
		if item.Checksum != "" {
			checksumMap[item.Checksum] = append(checksumMap[item.Checksum], item)
		}
	}

	// 标记重复项
	for checksum, items := range checksumMap {
		if len(items) > 1 {
			// 保留最新的，标记其他为已去重
			sort.Slice(items, func(i, j int) bool {
				return items[i].CreatedAt.After(items[j].CreatedAt)
			})

			for _, item := range items[1:] {
				if !dryRun {
					item.Deduplicated = true
					log.Printf("[SmartLifeBackup] 去重：%s (校验和：%s)", item.Name, checksum[:8])
				}
			}
		}
	}
}

// cleanupExpired 清理过期备份.
func (m *Manager) cleanupExpired(ctx context.Context, task *LifecycleTask, dryRun bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var expired []string

	for id, item := range m.backups {
		if !item.ExpiresAt.IsZero() && now.After(item.ExpiresAt) {
			expired = append(expired, id)
		}
	}

	if !dryRun {
		for _, id := range expired {
			delete(m.backups, id)
			log.Printf("[SmartLifeBackup] 清理过期备份：%s", id)
		}
	}

	if len(expired) > 0 {
		log.Printf("[SmartLifeBackup] 清理了 %d 个过期备份", len(expired))
	}
}

// matchesRule 检查备份项是否符合规则.
func (m *Manager) matchesRule(item *BackupItem, rule RetentionRule, now time.Time) bool {
	age := now.Sub(item.CreatedAt)
	ageDays := int(age.Hours() / 24)

	return ageDays <= rule.RetainDays
}

// ============================================================================
// 任务管理
// ============================================================================

// GetTask 获取任务.
func (m *Manager) GetTask(id string) (*LifecycleTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("任务不存在：%s", id)
	}
	return task, nil
}

// ListTasks 列出所有任务.
func (m *Manager) ListTasks() []*LifecycleTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*LifecycleTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// CancelTask 取消任务.
func (m *Manager) CancelTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("任务不存在：%s", id)
	}

	if task.Status != TaskStatusRunning {
		return fmt.Errorf("任务不在运行中")
	}

	task.Status = TaskStatusCancelled
	task.EndTime = time.Now()
	return nil
}

// ============================================================================
// 统计信息
// ============================================================================

// GetStats 获取统计信息.
func (m *Manager) GetStats() *LifecycleStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &LifecycleStats{
		TierDistribution: make(map[StorageTier]int64),
	}

	var totalSize int64
	var totalCompressSize int64
	var expiredCount int64
	now := time.Now()

	for _, item := range m.backups {
		stats.TotalBackups++
		totalSize += item.Size
		totalCompressSize += item.CompressSize

		if !item.ExpiresAt.IsZero() && now.After(item.ExpiresAt) {
			expiredCount++
		}

		stats.TierDistribution[item.Tier]++
	}

	stats.ActiveBackups = stats.TotalBackups - expiredCount
	stats.ExpiredBackups = expiredCount
	stats.TotalSizeGB = float64(totalSize) / (1024 * 1024 * 1024)

	if totalSize > 0 {
		stats.CompressionRatio = 1 - (float64(totalCompressSize) / float64(totalSize))
	}

	// 统计去重比例
	var dedupCount int64
	for _, item := range m.backups {
		if item.Deduplicated {
			dedupCount++
		}
	}
	if stats.TotalBackups > 0 {
		stats.DeduplicationRatio = float64(dedupCount) / float64(stats.TotalBackups)
	}

	// 统计任务处理数
	stats.TasksProcessed = int64(len(m.tasks))

	return stats
}

// GetCostReport 获取成本报告.
func (m *Manager) GetCostReport() *CostReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.costCalc.GenerateReport(m.backups)
}

// ============================================================================
// 调度管理
// ============================================================================

// GetScheduleConfig 获取调度配置.
func (m *Manager) GetScheduleConfig() *ScheduleConfig {
	if m.scheduler == nil {
		return DefaultScheduleConfig()
	}
	return m.scheduler.GetConfig()
}

// UpdateScheduleConfig 更新调度配置.
func (m *Manager) UpdateScheduleConfig(config *ScheduleConfig) error {
	if m.scheduler == nil {
		return fmt.Errorf("调度器未初始化")
	}
	return m.scheduler.UpdateConfig(config)
}

// ScheduleLifecycle 调度生命周期任务.
func (m *Manager) ScheduleLifecycle(schedule string) error {
	if m.scheduler == nil {
		return fmt.Errorf("调度器未初始化")
	}

	return m.scheduler.ScheduleTask(schedule, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		_, err := m.ExecuteLifecycle(ctx, nil)
		if err != nil {
			log.Printf("[SmartLifeBackup] 调度执行失败：%v", err)
		}
	})
}

// ============================================================================
// 数据持久化
// ============================================================================

// saveData 保存数据.
func (m *Manager) saveData() error {
	data := struct {
		Policies       map[string]*BackupPolicy  `json:"policies"`
		Backups        map[string]*BackupItem    `json:"backups"`
		Tasks          map[string]*LifecycleTask `json:"tasks"`
		ActivePolicyID string                    `json:"active_policy_id"`
	}{
		Policies:       m.policies,
		Backups:        m.backups,
		Tasks:          m.tasks,
		ActivePolicyID: m.activePolicyID,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化数据失败：%w", err)
	}

	filePath := filepath.Join(m.dataPath, "data.json")
	return os.WriteFile(filePath, jsonData, 0600)
}

// loadData 加载数据.
func (m *Manager) loadData() error {
	filePath := filepath.Join(m.dataPath, "data.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var saved struct {
		Policies       map[string]*BackupPolicy  `json:"policies"`
		Backups        map[string]*BackupItem    `json:"backups"`
		Tasks          map[string]*LifecycleTask `json:"tasks"`
		ActivePolicyID string                    `json:"active_policy_id"`
	}

	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("反序列化数据失败：%w", err)
	}

	m.policies = saved.Policies
	m.backups = saved.Backups
	m.tasks = saved.Tasks
	m.activePolicyID = saved.ActivePolicyID

	// 确保map不为nil
	if m.policies == nil {
		m.policies = make(map[string]*BackupPolicy)
	}
	if m.backups == nil {
		m.backups = make(map[string]*BackupItem)
	}
	if m.tasks == nil {
		m.tasks = make(map[string]*LifecycleTask)
	}

	return nil
}

// ============================================================================
// 辅助函数
// ============================================================================

// generateID 生成唯一ID.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// humanReadableSize 人类可读的大小.
func humanReadableSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case size >= TB:
		return fmt.Sprintf("%.2f TB", float64(size)/float64(TB))
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// GetHealthCheck 获取健康检查结果.
func (m *Manager) GetHealthCheck() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.GetStats()

	result := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"stats":     stats,
	}

	// 检查是否有活跃策略
	if m.activePolicyID == "" {
		result["status"] = "degraded"
		result["warning"] = "未设置活跃策略"
	}

	// 检查是否有过期备份
	if stats.ExpiredBackups > 100 {
		result["warning"] = fmt.Sprintf("有 %d 个过期备份待清理", stats.ExpiredBackups)
	}

	return result
}
