package tiering

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 存储分层管理器
type Manager struct {
	mu sync.RWMutex

	// 配置
	configPath string
	config     PolicyEngineConfig

	// 存储层配置
	tiers map[TierType]*TierConfig

	// 分层策略
	policies map[string]*Policy

	// 访问追踪
	tracker *AccessTracker

	// 迁移调度器
	migrator *Migrator

	// 迁移任务
	tasks map[string]*MigrateTask

	// 回调
	onPolicyComplete func(policyID string, task *MigrateTask)
}

// NewManager 创建存储分层管理器
func NewManager(configPath string, config PolicyEngineConfig) *Manager {
	m := &Manager{
		configPath: configPath,
		config:     config,
		tiers:      make(map[TierType]*TierConfig),
		policies:   make(map[string]*Policy),
		tasks:      make(map[string]*MigrateTask),
	}

	// 初始化访问追踪器
	m.tracker = NewAccessTracker(config)

	// 初始化迁移调度器
	m.migrator = NewMigrator(config)

	return m
}

// Initialize 初始化管理器
func (m *Manager) Initialize() error {
	// 加载配置
	if err := m.loadConfig(); err != nil {
		// 配置文件不存在时使用默认配置
		m.mu.Lock()
		m.initDefaultTiers()
		m.mu.Unlock()
	}

	// 启动访问追踪
	if err := m.tracker.Start(); err != nil {
		return fmt.Errorf("启动访问追踪器失败: %w", err)
	}

	// 启动迁移调度器
	m.migrator.Start()

	// 启动策略引擎
	if m.config.EnableAutoTier {
		go m.runPolicyEngine()
	}

	return nil
}

// initDefaultTiers 初始化默认存储层
func (m *Manager) initDefaultTiers() {
	m.tiers = map[TierType]*TierConfig{
		TierTypeSSD: {
			Type:      TierTypeSSD,
			Name:      "SSD 缓存层",
			Path:      "/mnt/ssd",
			Priority:  100,
			Enabled:   true,
			Threshold: 80,
		},
		TierTypeHDD: {
			Type:      TierTypeHDD,
			Name:      "HDD 存储层",
			Path:      "/mnt/hdd",
			Priority:  50,
			Enabled:   true,
			Threshold: 90,
		},
		TierTypeCloud: {
			Type:      TierTypeCloud,
			Name:      "云存储归档层",
			Path:      "/mnt/cloud",
			Priority:  10,
			Enabled:   false,
			Threshold: 95,
		},
	}
}

// ==================== 存储层管理 ====================

// CreateTier 创建存储层
func (m *Manager) CreateTier(tierType TierType, config TierConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tiers[tierType]; ok {
		return fmt.Errorf("存储层已存在: %s", tierType)
	}

	config.Type = tierType
	m.tiers[tierType] = &config

	return m.saveConfigLocked()
}

// GetTier 获取存储层配置
func (m *Manager) GetTier(tierType TierType) (*TierConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tier, ok := m.tiers[tierType]
	if !ok {
		return nil, fmt.Errorf("存储层不存在: %s", tierType)
	}

	return tier, nil
}

// ListTiers 列出所有存储层
func (m *Manager) ListTiers() []*TierConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*TierConfig, 0, len(m.tiers))
	for _, tier := range m.tiers {
		list = append(list, tier)
	}
	return list
}

// UpdateTier 更新存储层配置
func (m *Manager) UpdateTier(tierType TierType, config TierConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tiers[tierType]; !ok {
		return fmt.Errorf("存储层不存在: %s", tierType)
	}

	config.Type = tierType
	m.tiers[tierType] = &config

	return m.saveConfigLocked()
}

// DeleteTier 删除存储层
func (m *Manager) DeleteTier(tierType TierType) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tiers[tierType]; !ok {
		return fmt.Errorf("存储层不存在: %s", tierType)
	}

	delete(m.tiers, tierType)

	return m.saveConfigLocked()
}

// ==================== 策略管理 ====================

// CreatePolicy 创建分层策略
func (m *Manager) CreatePolicy(policy Policy) (*Policy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证
	if policy.Name == "" {
		return nil, fmt.Errorf("策略名称不能为空")
	}

	// 验证存储层
	if _, ok := m.tiers[policy.SourceTier]; !ok {
		return nil, fmt.Errorf("源存储层不存在: %s", policy.SourceTier)
	}

	if _, ok := m.tiers[policy.TargetTier]; !ok {
		return nil, fmt.Errorf("目标存储层不存在: %s", policy.TargetTier)
	}

	// 生成 ID
	if policy.ID == "" {
		policy.ID = "policy_" + uuid.New().String()[:8]
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	policy.Status = PolicyStatusEnabled
	if !policy.Enabled {
		policy.Status = PolicyStatusDisabled
	}

	// 设置默认值
	if policy.ScheduleType == "" {
		policy.ScheduleType = ScheduleTypeManual
	}

	m.policies[policy.ID] = &policy

	if err := m.saveConfigLocked(); err != nil {
		delete(m.policies, policy.ID)
		return nil, err
	}

	return &policy, nil
}

// GetPolicy 获取策略
func (m *Manager) GetPolicy(id string) (*Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("策略不存在: %s", id)
	}

	return policy, nil
}

// ListPolicies 列出所有策略
func (m *Manager) ListPolicies() []*Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*Policy
	for _, policy := range m.policies {
		list = append(list, policy)
	}
	return list
}

// UpdatePolicy 更新策略
func (m *Manager) UpdatePolicy(id string, policy Policy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("策略不存在: %s", id)
	}

	policy.ID = id
	policy.UpdatedAt = time.Now()
	policy.Status = PolicyStatusEnabled
	if !policy.Enabled {
		policy.Status = PolicyStatusDisabled
	}

	m.policies[id] = &policy

	return m.saveConfigLocked()
}

// DeletePolicy 删除策略
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("策略不存在: %s", id)
	}

	delete(m.policies, id)

	return m.saveConfigLocked()
}

// ExecutePolicy 执行策略
func (m *Manager) ExecutePolicy(policyID string) (*MigrateTask, error) {
	m.mu.RLock()
	policy, ok := m.policies[policyID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("策略不存在: %s", policyID)
	}
	m.mu.RUnlock()

	// 查找符合条件的文件
	files, err := m.findFilesForPolicy(policy)
	if err != nil {
		return nil, fmt.Errorf("查找文件失败: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("没有符合条件的文件")
	}

	// 创建迁移任务
	task := &MigrateTask{
		ID:         "task_" + uuid.New().String()[:8],
		PolicyID:   policyID,
		Status:     MigrateStatusPending,
		CreatedAt:  time.Now(),
		SourceTier: policy.SourceTier,
		TargetTier: policy.TargetTier,
		Action:     policy.Action,
		Files:      files,
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 提交迁移任务
	go m.executeMigrateTask(task, policy)

	return task, nil
}

// findFilesForPolicy 查找符合策略的文件
func (m *Manager) findFilesForPolicy(policy *Policy) ([]MigrateFile, error) {
	// 从访问追踪器获取文件记录
	records := m.tracker.GetRecordsByTier(policy.SourceTier)

	var files []MigrateFile
	for _, record := range records {
		if m.matchPolicy(record, policy) {
			files = append(files, MigrateFile{
				Path:    record.Path,
				Size:    record.Size,
				ModTime: record.ModTime,
				Status:  "pending",
			})
		}
	}

	return files, nil
}

// matchPolicy 检查文件是否匹配策略
func (m *Manager) matchPolicy(record *FileAccessRecord, policy *Policy) bool {
	// 检查访问次数
	if policy.MinAccessCount > 0 && record.AccessCount < policy.MinAccessCount {
		return false
	}

	// 检查访问时间
	if policy.MaxAccessAge > 0 {
		age := time.Since(record.AccessTime)
		if age > policy.MaxAccessAge {
			return false
		}
	}

	// 检查文件大小
	if policy.MinFileSize > 0 && record.Size < policy.MinFileSize {
		return false
	}

	if policy.MaxFileSize > 0 && record.Size > policy.MaxFileSize {
		return false
	}

	// 检查文件模式匹配
	if len(policy.FilePatterns) > 0 {
		matched := false
		for _, pattern := range policy.FilePatterns {
			if m, err := filepath.Match(pattern, filepath.Base(record.Path)); err == nil && m {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 检查排除模式
	for _, pattern := range policy.ExcludePatterns {
		if m, err := filepath.Match(pattern, filepath.Base(record.Path)); err == nil && m {
			return false
		}
	}

	return true
}

// ==================== 手动迁移 ====================

// Migrate 手动迁移
func (m *Manager) Migrate(req MigrateRequest) (*MigrateTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 验证存储层
	if _, ok := m.tiers[req.SourceTier]; !ok {
		return nil, fmt.Errorf("源存储层不存在: %s", req.SourceTier)
	}

	if _, ok := m.tiers[req.TargetTier]; !ok {
		return nil, fmt.Errorf("目标存储层不存在: %s", req.TargetTier)
	}

	// 收集文件
	var files []MigrateFile
	for _, path := range req.Paths {
		fileList, err := m.collectFiles(path, req)
		if err != nil {
			return nil, fmt.Errorf("收集文件失败: %w", err)
		}
		files = append(files, fileList...)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("没有符合条件的文件")
	}

	// 创建迁移任务
	task := &MigrateTask{
		ID:         "task_" + uuid.New().String()[:8],
		Status:     MigrateStatusPending,
		CreatedAt:  time.Now(),
		SourceTier: req.SourceTier,
		TargetTier: req.TargetTier,
		Action:     req.Action,
		Files:      files,
	}

	m.tasks[task.ID] = task

	// 提交迁移任务
	go m.executeMigrateTask(task, &Policy{
		DryRun:         req.DryRun,
		PreserveOrigin: req.Preserve,
	})

	return task, nil
}

// collectFiles 收集文件
func (m *Manager) collectFiles(path string, req MigrateRequest) ([]MigrateFile, error) {
	var files []MigrateFile

	// 检查路径是否存在
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		// 单个文件
		if m.matchMigrateRequest(path, info, req) {
			files = append(files, MigrateFile{
				Path:    path,
				Size:    info.Size(),
				ModTime: info.ModTime(),
				Status:  "pending",
			})
		}
		return files, nil
	}

	// 目录，遍历
	err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误继续
		}

		if info.IsDir() {
			return nil
		}

		if m.matchMigrateRequest(filePath, info, req) {
			files = append(files, MigrateFile{
				Path:    filePath,
				Size:    info.Size(),
				ModTime: info.ModTime(),
				Status:  "pending",
			})
		}

		return nil
	})

	return files, err
}

// matchMigrateRequest 检查文件是否匹配迁移请求
func (m *Manager) matchMigrateRequest(path string, info os.FileInfo, req MigrateRequest) bool {
	// 检查文件大小
	if req.MinSize > 0 && info.Size() < req.MinSize {
		return false
	}

	if req.MaxSize > 0 && info.Size() > req.MaxSize {
		return false
	}

	// 检查文件年龄
	if req.MinAge > 0 {
		age := time.Since(info.ModTime())
		if age < req.MinAge {
			return false
		}
	}

	// 检查文件模式匹配
	if req.Pattern != "" {
		matched, err := filepath.Match(req.Pattern, filepath.Base(path))
		if err != nil || !matched {
			return false
		}
	}

	return true
}

// executeMigrateTask 执行迁移任务
func (m *Manager) executeMigrateTask(task *MigrateTask, policy *Policy) {
	m.mu.Lock()
	task.Status = MigrateStatusRunning
	task.StartedAt = time.Now()

	// 计算总量
	for _, file := range task.Files {
		task.TotalFiles++
		task.TotalBytes += file.Size
	}
	m.mu.Unlock()

	// 执行迁移
	for i := range task.Files {
		file := &task.Files[i]

		err := m.migrator.MigrateFile(file, policy)

		m.mu.Lock()
		task.ProcessedFiles++
		task.ProcessedBytes += file.Size

		if err != nil {
			file.Status = "failed"
			file.Error = err.Error()
			task.FailedFiles++
			task.Errors = append(task.Errors, MigrateError{
				Path:    file.Path,
				Message: err.Error(),
				Time:    time.Now(),
			})
		} else {
			file.Status = "completed"
		}
		m.mu.Unlock()
	}

	// 更新任务状态
	m.mu.Lock()
	task.CompletedAt = time.Now()
	if task.FailedFiles > 0 && task.FailedFiles == task.TotalFiles {
		task.Status = MigrateStatusFailed
	} else {
		task.Status = MigrateStatusCompleted
	}
	m.mu.Unlock()

	// 回调
	if m.onPolicyComplete != nil && policy.ID != "" {
		m.onPolicyComplete(policy.ID, task)
	}
}

// ==================== 状态查询 ====================

// GetStatus 获取分层状态
func (m *Manager) GetStatus() *Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 统计运行中的任务
	var running, pending int
	for _, task := range m.tasks {
		switch task.Status {
		case MigrateStatusRunning:
			running++
		case MigrateStatusPending:
			pending++
		}
	}

	// 统计活跃策略
	var activePolicy int
	for _, policy := range m.policies {
		if policy.Enabled {
			activePolicy++
		}
	}

	// 最后迁移时间
	var lastMigration time.Time
	for _, task := range m.tasks {
		if task.CompletedAt.After(lastMigration) {
			lastMigration = task.CompletedAt
		}
	}

	// 复制存储层配置
	tiers := make(map[TierType]*TierConfig)
	for k, v := range m.tiers {
		tiers[k] = v
	}

	return &Status{
		Enabled:       m.config.EnableAutoTier,
		RunningTasks:  running,
		PendingTasks:  pending,
		LastMigration: lastMigration,
		Tiers:         tiers,
		Policies:      len(m.policies),
		ActivePolicy:  activePolicy,
	}
}

// GetTask 获取迁移任务
func (m *Manager) GetTask(id string) (*MigrateTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", id)
	}

	return task, nil
}

// CancelTask 取消迁移任务
func (m *Manager) CancelTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("任务不存在: %s", id)
	}

	// 只有 pending 或 running 状态的任务可以取消
	if task.Status == MigrateStatusPending || task.Status == MigrateStatusRunning {
		task.Status = MigrateStatusCancelled
		return nil
	}

	return fmt.Errorf("任务状态不允许取消: %s", task.Status)
}

// ListTasks 列出迁移任务
func (m *Manager) ListTasks(limit int) []*MigrateTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*MigrateTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		list = append(list, task)
		if limit > 0 && len(list) >= limit {
			break
		}
	}
	return list
}

// GetAccessStats 获取访问统计
func (m *Manager) GetAccessStats() *AccessStats {
	return m.tracker.GetStats()
}

// GetTierStats 获取存储层统计
func (m *Manager) GetTierStats(tierType TierType) (*TierStats, error) {
	m.mu.RLock()
	tier, ok := m.tiers[tierType]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("存储层不存在: %s", tierType)
	}

	return m.tracker.GetTierStats(tierType, tier)
}

// ==================== v2.4.0 存储分层增强 ====================

// OptimizeSSDCache SSD缓存层优化
// 根据访问频率和缓存命中率优化SSD缓存内容
func (m *Manager) OptimizeSSDCache() (*SSDCacheOptimizeResult, error) {
	m.mu.RLock()
	ssdTier, ssdExists := m.tiers[TierTypeSSD]
	hddTier, hddExists := m.tiers[TierTypeHDD]
	m.mu.RUnlock()

	if !ssdExists || !ssdTier.Enabled {
		return nil, fmt.Errorf("SSD缓存层未启用")
	}

	result := &SSDCacheOptimizeResult{
		StartTime: time.Now(),
		Tier:      TierTypeSSD,
	}

	// 1. 获取SSD上的冷数据（应该降级到HDD）
	coldFiles := m.tracker.GetColdFiles(TierTypeSSD, 0)
	result.ColdFilesIdentified = len(coldFiles)

	// 2. 获取HDD上的热数据（应该提升到SSD）
	var hotFilesToPromote []*FileAccessRecord
	if hddExists && hddTier.Enabled {
		hotFiles := m.tracker.GetHotFiles(TierTypeHDD, 0)
		// 计算SSD可用空间
		ssdAvailable := ssdTier.Capacity - ssdTier.Used
		var promoteSize int64
		for _, file := range hotFiles {
			if promoteSize+file.Size <= ssdAvailable*80/100 { // 保留20%空间
				hotFilesToPromote = append(hotFilesToPromote, file)
				promoteSize += file.Size
			}
		}
		result.HotFilesIdentified = len(hotFilesToPromote)
	}

	// 3. 执行冷数据降级
	for _, file := range coldFiles {
		if hddExists && hddTier.Enabled {
			task, err := m.Migrate(MigrateRequest{
				Paths:      []string{file.Path},
				SourceTier: TierTypeSSD,
				TargetTier: TierTypeHDD,
				Action:     PolicyActionMove,
				Preserve:   false,
			})
			if err == nil {
				result.DemotedFiles++
				result.DemotedBytes += file.Size
				result.Tasks = append(result.Tasks, task.ID)
			} else {
				result.FailedDemotions++
			}
		}
	}

	// 4. 执行热数据提升
	for _, file := range hotFilesToPromote {
		task, err := m.Migrate(MigrateRequest{
			Paths:      []string{file.Path},
			SourceTier: TierTypeHDD,
			TargetTier: TierTypeSSD,
			Action:     PolicyActionCopy,
			Preserve:   true, // 保留HDD上的副本作为备份
		})
		if err == nil {
			result.PromotedFiles++
			result.PromotedBytes += file.Size
			result.Tasks = append(result.Tasks, task.ID)
		} else {
			result.FailedPromotions++
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// AutoMigrate 自动数据迁移算法
// 基于访问频率自动迁移数据到合适的存储层
func (m *Manager) AutoMigrate() (*AutoMigrateResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := &AutoMigrateResult{
		StartTime: time.Now(),
		Tiers:     make(map[TierType]*TierMigrationStats),
	}

	// 获取所有存储层
	tiers := m.ListTiers()
	if len(tiers) < 2 {
		return result, nil // 至少需要2层才能迁移
	}

	// 按优先级排序存储层（SSD > HDD > Cloud）
	tierPriority := []TierType{TierTypeSSD, TierTypeHDD, TierTypeCloud}

	for i, tierType := range tierPriority {
		tier, exists := m.tiers[tierType]
		if !exists || !tier.Enabled {
			continue
		}

		stats := &TierMigrationStats{
			TierType: tierType,
		}

		records := m.tracker.GetRecordsByTier(tierType)
		for _, record := range records {
			// 根据访问频率决定迁移目标
			var targetTier TierType
			var shouldMigrate bool

			switch record.Frequency {
			case AccessFrequencyHot:
				// 热数据：迁移到更高优先级层（如果不在SSD）
				if i > 0 && tierPriority[0] == TierTypeSSD {
					if ssdTier, ok := m.tiers[TierTypeSSD]; ok && ssdTier.Enabled {
						targetTier = TierTypeSSD
						shouldMigrate = true
					}
				}
			case AccessFrequencyCold:
				// 冷数据：迁移到更低优先级层（归档）
				if i < len(tierPriority)-1 {
					nextTier := tierPriority[i+1]
					if nextTierTier, ok := m.tiers[nextTier]; ok && nextTierTier.Enabled {
						targetTier = nextTier
						shouldMigrate = true
					}
				}
			}

			if shouldMigrate && targetTier != "" {
				stats.FilesToMigrate = append(stats.FilesToMigrate, record)
				stats.TotalMigrateBytes += record.Size
			}
		}

		result.Tiers[tierType] = stats
	}

	// 执行迁移（异步）
	for tierType, stats := range result.Tiers {
		if len(stats.FilesToMigrate) == 0 {
			continue
		}

		// 确定目标层
		var targetTier TierType
		records := m.tracker.GetRecordsByTier(tierType)
		if len(records) > 0 && records[0].Frequency == AccessFrequencyHot {
			targetTier = TierTypeSSD
		} else if len(records) > 0 && records[0].Frequency == AccessFrequencyCold {
			// 选择下一层
			for i, t := range tierPriority {
				if t == tierType && i < len(tierPriority)-1 {
					targetTier = tierPriority[i+1]
					break
				}
			}
		}

		if targetTier == "" || targetTier == tierType {
			continue
		}

		// 批量迁移
		var paths []string
		for _, file := range stats.FilesToMigrate {
			paths = append(paths, file.Path)
		}

		go func(sourceTier, target TierType, files []string) {
			_, _ = m.Migrate(MigrateRequest{
				Paths:      files,
				SourceTier: sourceTier,
				TargetTier: target,
				Action:     PolicyActionMove,
			})
		}(tierType, targetTier, paths)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// GetAllTierStats 分层统计报告
// 返回所有存储层的详细统计信息
func (m *Manager) GetAllTierStats() (*StatsReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &StatsReport{
		GeneratedAt: time.Now(),
		Tiers:       make(map[TierType]*TierStats),
	}

	// 收集每层统计
	for tierType, tier := range m.tiers {
		if !tier.Enabled {
			continue
		}

		stats, err := m.tracker.GetTierStats(tierType, tier)
		if err != nil {
			continue
		}
		report.Tiers[tierType] = stats
	}

	// 计算总体统计
	report.Summary = &Summary{
		TotalFiles:   0,
		TotalBytes:   0,
		TotalHot:     0,
		TotalWarm:    0,
		TotalCold:    0,
		HotPercent:   0,
		WarmPercent:  0,
		ColdPercent:  0,
		HitRateSSD:   0,
		MigrateTasks: len(m.tasks),
		ActivePolicy: 0,
	}

	for _, stats := range report.Tiers {
		report.Summary.TotalFiles += stats.TotalFiles
		report.Summary.TotalBytes += stats.TotalBytes
		report.Summary.TotalHot += stats.HotFiles
		report.Summary.TotalWarm += stats.WarmFiles
		report.Summary.TotalCold += stats.ColdFiles
	}

	// 计算百分比
	if report.Summary.TotalFiles > 0 {
		report.Summary.HotPercent = float64(report.Summary.TotalHot) / float64(report.Summary.TotalFiles) * 100
		report.Summary.WarmPercent = float64(report.Summary.TotalWarm) / float64(report.Summary.TotalFiles) * 100
		report.Summary.ColdPercent = float64(report.Summary.TotalCold) / float64(report.Summary.TotalFiles) * 100
	}

	// 统计活跃策略
	for _, policy := range m.policies {
		if policy.Enabled {
			report.Summary.ActivePolicy++
		}
	}

	// 计算SSD命中率
	if ssdStats, ok := report.Tiers[TierTypeSSD]; ok {
		totalAccess := ssdStats.HotFiles + ssdStats.WarmFiles
		if totalAccess > 0 {
			report.Summary.HitRateSSD = float64(ssdStats.HotFiles) / float64(totalAccess) * 100
		}
	}

	return report, nil
}

// ==================== 策略引擎 ====================

// runPolicyEngine 运行策略引擎
func (m *Manager) runPolicyEngine() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.executeAutoPolicies()
	}
}

// executeAutoPolicies 执行自动策略
func (m *Manager) executeAutoPolicies() {
	m.mu.RLock()
	policies := make([]*Policy, 0)
	for _, policy := range m.policies {
		if policy.Enabled && policy.ScheduleType != ScheduleTypeManual {
			// 检查是否需要执行
			if policy.NextRun.IsZero() || time.Now().After(policy.NextRun) {
				policies = append(policies, policy)
			}
		}
	}
	m.mu.RUnlock()

	for _, policy := range policies {
		// 异步执行
		go func() { _, _ = m.ExecutePolicy(policy.ID) }()

		// 更新下次执行时间
		m.mu.Lock()
		if p, ok := m.policies[policy.ID]; ok {
			p.LastRun = time.Now()
			p.NextRun = m.calculateNextRun(p)
		}
		m.mu.Unlock()
	}
}

// calculateNextRun 计算下次执行时间
func (m *Manager) calculateNextRun(policy *Policy) time.Time {
	now := time.Now()

	switch policy.ScheduleType {
	case ScheduleTypeInterval:
		// 基于间隔执行
		// 如果 ScheduleExpr 是数字字符串，解析为小时数
		var interval time.Duration
		if policy.ScheduleExpr != "" {
			// 尝试解析为小时数
			if hours, err := time.ParseDuration(policy.ScheduleExpr); err == nil {
				interval = hours
			} else {
				// 尝试解析为纯数字（小时）
				if h, err := time.ParseDuration(policy.ScheduleExpr + "h"); err == nil {
					interval = h
				} else {
					// 默认 1 小时
					interval = time.Hour
				}
			}
		} else {
			// 使用配置的检查间隔
			interval = m.config.CheckInterval
		}
		return now.Add(interval)

	case ScheduleTypeCron:
		// 基于 Cron 表达式执行
		if policy.ScheduleExpr != "" {
			if nextTime, err := parseCronExpression(policy.ScheduleExpr, now); err == nil {
				return nextTime
			}
		}
		// 解析失败，默认 24 小时后
		return now.Add(24 * time.Hour)

	default:
		// 手动执行，不设置下次运行时间
		return time.Time{}
	}
}

// parseCronExpression 解析 Cron 表达式并计算下次执行时间
// 支持 5 字段格式：分 时 日 月 周
// 例如: "0 2 * * *" 表示每天凌晨 2 点执行
func parseCronExpression(expr string, from time.Time) (time.Time, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return time.Time{}, fmt.Errorf("无效的 cron 表达式: %s", expr)
	}

	// 解析各字段
	minute, err := parseCronField(parts[0], 0, 59)
	if err != nil {
		return time.Time{}, fmt.Errorf("分钟字段错误: %w", err)
	}
	hour, err := parseCronField(parts[1], 0, 23)
	if err != nil {
		return time.Time{}, fmt.Errorf("小时字段错误: %w", err)
	}
	day, err := parseCronField(parts[2], 1, 31)
	if err != nil {
		return time.Time{}, fmt.Errorf("日期字段错误: %w", err)
	}
	month, err := parseCronField(parts[3], 1, 12)
	if err != nil {
		return time.Time{}, fmt.Errorf("月份字段错误: %w", err)
	}
	weekday, err := parseCronField(parts[4], 0, 6)
	if err != nil {
		return time.Time{}, fmt.Errorf("周字段错误: %w", err)
	}

	// 从下一分钟开始查找
	next := from.Add(time.Minute).Truncate(time.Minute)

	// 最多查找 366 天
	for i := 0; i < 366*24*60; i++ {
		// 检查月份
		if !month[int(next.Month())] {
			next = time.Date(next.Year(), next.Month()+1, 1, 0, 0, 0, 0, next.Location())
			continue
		}

		// 检查日期
		if !day[next.Day()] {
			next = next.AddDate(0, 0, 1).Truncate(24 * time.Hour)
			continue
		}

		// 检查星期
		if !weekday[int(next.Weekday())] {
			next = next.AddDate(0, 0, 1).Truncate(24 * time.Hour)
			continue
		}

		// 检查小时
		if !hour[next.Hour()] {
			next = next.Add(time.Hour).Truncate(time.Hour)
			continue
		}

		// 检查分钟
		if !minute[next.Minute()] {
			next = next.Add(time.Minute)
			continue
		}

		// 找到匹配的时间
		return next, nil
	}

	return time.Time{}, fmt.Errorf("未找到有效的下次执行时间")
}

// parseCronField 解析 cron 字段
// 支持格式: "*", "1,2,3", "1-5", "*/2"
func parseCronField(field string, min, max int) (map[int]bool, error) {
	result := make(map[int]bool)

	if field == "*" {
		for i := min; i <= max; i++ {
			result[i] = true
		}
		return result, nil
	}

	// 处理逗号分隔的多个值
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)

		// 处理 */n 格式（步长）
		if strings.HasPrefix(part, "*/") {
			stepStr := strings.TrimPrefix(part, "*/")
			step, err := time.ParseDuration(stepStr + "h")
			if err != nil {
				// 尝试解析为纯数字
				var stepInt int
				if _, err := fmt.Sscanf(stepStr, "%d", &stepInt); err == nil && stepInt > 0 {
					for i := min; i <= max; i += stepInt {
						result[i] = true
					}
					continue
				}
				return nil, fmt.Errorf("无效的步长: %s", stepStr)
			}
			_ = step // 避免未使用变量警告
			for i := min; i <= max; i++ {
				result[i] = true
			}
			continue
		}

		// 处理范围 a-b
		if strings.Contains(part, "-") {
			var start, end int
			if _, err := fmt.Sscanf(part, "%d-%d", &start, &end); err != nil {
				return nil, fmt.Errorf("无效的范围: %s", part)
			}
			if start < min || end > max || start > end {
				return nil, fmt.Errorf("范围超出边界: %s", part)
			}
			for i := start; i <= end; i++ {
				result[i] = true
			}
			continue
		}

		// 处理单个数字
		var value int
		if _, err := fmt.Sscanf(part, "%d", &value); err != nil {
			return nil, fmt.Errorf("无效的值: %s", part)
		}
		if value < min || value > max {
			return nil, fmt.Errorf("值超出边界: %d", value)
		}
		result[value] = true
	}

	return result, nil
}

// ==================== 配置持久化 ====================

type configData struct {
	Tiers    map[TierType]*TierConfig `json:"tiers"`
	Policies map[string]*Policy       `json:"policies"`
	Config   PolicyEngineConfig       `json:"config"`
}

func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var cfg configData
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	m.mu.Lock()
	m.tiers = cfg.Tiers
	m.policies = cfg.Policies
	if cfg.Config.CheckInterval > 0 {
		m.config = cfg.Config
	}
	m.mu.Unlock()

	return nil
}

func (m *Manager) saveConfigLocked() error {
	cfg := configData{
		Tiers:    m.tiers,
		Policies: m.policies,
		Config:   m.config,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(m.configPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.tracker.Stop()
	m.migrator.Stop()
}
