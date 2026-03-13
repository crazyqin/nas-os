package tiering

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	var list []*TierConfig
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

	// TODO: 检查文件模式匹配

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

	// TODO: 检查文件模式匹配

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
func (m *Manager) GetStatus() *TieringStatus {
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

	return &TieringStatus{
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

// ListTasks 列出迁移任务
func (m *Manager) ListTasks(limit int) []*MigrateTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*MigrateTask
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
			// TODO: 计算下次执行时间
		}
		m.mu.Unlock()
	}
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
