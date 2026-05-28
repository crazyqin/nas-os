package smartbackupsched

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 智能备份调度管理器.
type Manager struct {
	mu sync.RWMutex

	// 调度配置
	configs map[string]*ScheduleConfig

	// 备份任务
	tasks map[string]*BackupTask

	// 任务取消函数
	cancels map[string]context.CancelFunc

	// 变更分析数据
	patterns map[string]*ChangePattern

	// 审计日志
	auditLog []AuditEntry

	// 持久化路径
	configPath string
	auditPath  string

	// 默认配置
	defaultMaxRetries    int
	defaultRetryInterval time.Duration
}

// NewManager 创建智能备份调度管理器.
func NewManager(configPath, auditPath string) *Manager {
	return &Manager{
		configs:              make(map[string]*ScheduleConfig),
		tasks:                make(map[string]*BackupTask),
		cancels:              make(map[string]context.CancelFunc),
		patterns:             make(map[string]*ChangePattern),
		auditLog:             make([]AuditEntry, 0),
		configPath:           configPath,
		auditPath:            auditPath,
		defaultMaxRetries:    3,
		defaultRetryInterval: 5 * time.Minute,
	}
}

// Initialize 初始化管理器.
func (m *Manager) Initialize() error {
	if err := m.loadConfigs(); err != nil {
		m.mu.Lock()
		m.configs = make(map[string]*ScheduleConfig)
		m.mu.Unlock()
	}
	if err := m.loadAuditLog(); err != nil {
		m.mu.Lock()
		m.auditLog = make([]AuditEntry, 0)
		m.mu.Unlock()
	}
	return nil
}

// ========== 配置管理 ==========

// ListConfigs 列出所有调度配置.
func (m *Manager) ListConfigs() []*ScheduleConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]*ScheduleConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		configs = append(configs, cfg)
	}
	return configs
}

// GetConfig 获取指定调度配置.
func (m *Manager) GetConfig(id string) (*ScheduleConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, ok := m.configs[id]
	if !ok {
		return nil, fmt.Errorf("调度配置不存在：%s", id)
	}
	return cfg, nil
}

// CreateConfig 创建调度配置.
func (m *Manager) CreateConfig(config ScheduleConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ID == "" {
		config.ID = generateID()
	}
	if config.Name == "" {
		return fmt.Errorf("配置名称不能为空")
	}
	if config.SourcePath == "" {
		return fmt.Errorf("源路径不能为空")
	}

	// 默认值
	if config.Strategy == "" {
		config.Strategy = StrategyAuto
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = m.defaultMaxRetries
	}
	if config.RetryInterval == 0 {
		config.RetryInterval = m.defaultRetryInterval
	}
	if config.RetentionDays == 0 {
		config.RetentionDays = 30
	}
	if config.MaxBackups == 0 {
		config.MaxBackups = 10
	}
	if config.StorageWarnPercent == 0 {
		config.StorageWarnPercent = 80
	}
	if config.StorageLimitPercent == 0 {
		config.StorageLimitPercent = 95
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	m.configs[config.ID] = &config

	if err := m.saveConfigs(); err != nil {
		delete(m.configs, config.ID)
		return fmt.Errorf("保存配置失败：%w", err)
	}

	m.appendAudit("create", config.ID, "", "system", fmt.Sprintf("创建调度配置：%s", config.Name), true)
	return nil
}

// UpdateConfig 更新调度配置.
func (m *Manager) UpdateConfig(id string, config ScheduleConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.configs[id]
	if !ok {
		return fmt.Errorf("调度配置不存在：%s", id)
	}

	config.ID = id
	config.CreatedAt = existing.CreatedAt
	config.UpdatedAt = time.Now()

	m.configs[id] = &config

	if err := m.saveConfigs(); err != nil {
		return fmt.Errorf("保存配置失败：%w", err)
	}

	m.appendAudit("update", id, "", "system", fmt.Sprintf("更新调度配置：%s", config.Name), true)
	return nil
}

// DeleteConfig 删除调度配置.
func (m *Manager) DeleteConfig(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, ok := m.configs[id]
	if !ok {
		return fmt.Errorf("调度配置不存在：%s", id)
	}

	delete(m.configs, id)

	if err := m.saveConfigs(); err != nil {
		return fmt.Errorf("保存配置失败：%w", err)
	}

	m.appendAudit("delete", id, "", "system", fmt.Sprintf("删除调度配置：%s", cfg.Name), true)
	return nil
}

// EnableConfig 启用/禁用调度配置.
func (m *Manager) EnableConfig(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, ok := m.configs[id]
	if !ok {
		return fmt.Errorf("调度配置不存在：%s", id)
	}

	cfg.Enabled = enabled
	cfg.UpdatedAt = time.Now()

	if err := m.saveConfigs(); err != nil {
		return fmt.Errorf("保存配置失败：%w", err)
	}

	action := "disable"
	if enabled {
		action = "enable"
	}
	m.appendAudit(action, id, "", "system", fmt.Sprintf("调度配置已%s", map[bool]string{true: "启用", false: "禁用"}[enabled]), true)
	return nil
}

// ========== 任务管理 ==========

// ListTasks 列出所有任务.
func (m *Manager) ListTasks() []*BackupTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*BackupTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// GetTask 获取指定任务.
func (m *Manager) GetTask(id string) (*BackupTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("任务不存在：%s", id)
	}
	return task, nil
}

// CancelTask 取消任务.
func (m *Manager) CancelTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("任务不存在：%s", id)
	}
	if task.Status != StatusRunning && task.Status != StatusPending && task.Status != StatusRetrying {
		return fmt.Errorf("任务状态不允许取消：%s", task.Status)
	}

	if cancel, exists := m.cancels[id]; exists {
		cancel()
		delete(m.cancels, id)
	}
	task.Status = StatusCancelled
	task.EndTime = time.Now()

	m.appendAudit("cancel", task.ConfigID, id, "user", "取消备份任务", true)
	return nil
}

// RunBackup 手动触发备份.
func (m *Manager) RunBackup(configID string) (*BackupTask, error) {
	return m.RunBackupWithContext(context.Background(), configID, "manual")
}

// RunBackupWithContext 带上下文执行备份.
func (m *Manager) RunBackupWithContext(ctx context.Context, configID, createdBy string) (*BackupTask, error) {
	m.mu.RLock()
	cfg, ok := m.configs[configID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("调度配置不存在：%s", configID)
	}
	m.mu.RUnlock()

	// 确定策略
	strategy := cfg.Strategy
	if cfg.AutoStrategy || strategy == StrategyAuto {
		rec := m.RecommendStrategy(configID)
		strategy = rec.Recommended
	}

	// 确定目标 Tier（按优先级）
	tier, targetPath := m.selectTarget(cfg)
	if targetPath == "" {
		return nil, fmt.Errorf("无可用备份目标")
	}

	// 风险评估
	risk := m.AssessRisk(configID)

	taskCtx, cancel := context.WithCancel(ctx)

	task := &BackupTask{
		ID:             generateID(),
		ConfigID:       configID,
		Strategy:       strategy,
		Tier:           tier,
		Status:         StatusPending,
		SourcePath:     cfg.SourcePath,
		TargetPath:     targetPath,
		MaxRetries:     cfg.MaxRetries,
		RiskAssessment: risk,
		CreatedBy:      createdBy,
		StartTime:      time.Now(),
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.cancels[task.ID] = cancel
	m.mu.Unlock()

	m.appendAudit("execute", configID, task.ID, createdBy,
		fmt.Sprintf("启动备份：策略=%s, Tier=%s, 目标=%s", strategy, tier, targetPath), true)

	go m.executeTask(taskCtx, cfg, task)

	return task, nil
}

// selectTarget 按优先级选择备份目标.
func (m *Manager) selectTarget(cfg *ScheduleConfig) (BackupTier, string) {
	targets := make([]TargetPath, len(cfg.TargetPaths))
	copy(targets, cfg.TargetPaths)
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Enabled != targets[j].Enabled {
			return targets[i].Enabled
		}
		return targets[i].Priority < targets[j].Priority
	})

	for _, t := range targets {
		if t.Enabled {
			return t.Tier, t.Path
		}
	}
	return "", ""
}

// executeTask 执行备份任务.
func (m *Manager) executeTask(ctx context.Context, cfg *ScheduleConfig, task *BackupTask) {
	m.mu.Lock()
	task.Status = StatusRunning
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		if task.Status == StatusRunning {
			task.Status = StatusCompleted
		}
		task.EndTime = time.Now()
		delete(m.cancels, task.ID)
		m.saveAuditLog()
		m.mu.Unlock()
	}()

	select {
	case <-ctx.Done():
		m.mu.Lock()
		task.Status = StatusCancelled
		task.Error = ctx.Err().Error()
		m.mu.Unlock()
		return
	default:
	}

	// 检查是否在高峰期
	if m.isInPeakHours(cfg) {
		m.mu.Lock()
		task.Status = StatusPending
		task.Error = "当前处于业务高峰期，推迟执行"
		m.mu.Unlock()
		m.appendAudit("postpone", cfg.ID, task.ID, "scheduler",
			"处于业务高峰期，推迟备份", true)
		return
	}

	// 模拟备份执行（实际项目中会调用底层备份引擎）
	err := m.doBackup(ctx, task)

	m.mu.Lock()
	if err != nil {
		if ctx.Err() != nil {
			task.Status = StatusCancelled
			task.Error = "备份已取消"
		} else {
			task.Status = StatusFailed
			task.Error = err.Error()

			m.appendAudit("fail", cfg.ID, task.ID, "scheduler",
				fmt.Sprintf("备份失败：%v", err), false)

			// 自动重试
			if task.RetryCount < task.MaxRetries {
				m.scheduleRetry(cfg, task)
			} else if cfg.DegradedOnFail {
				m.degradeTask(cfg, task)
			}
		}
	} else {
		task.Status = StatusCompleted
		task.Progress = 100
		m.appendAudit("complete", cfg.ID, task.ID, "scheduler",
			fmt.Sprintf("备份完成：策略=%s, Tier=%s", task.Strategy, task.Tier), true)
	}
	m.mu.Unlock()
}

// doBackup 执行实际备份操作.
func (m *Manager) doBackup(ctx context.Context, task *BackupTask) error {
	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(task.TargetPath), 0750); err != nil {
		return fmt.Errorf("创建目标目录失败：%w", err)
	}

	// 检查源路径
	if _, err := os.Stat(task.SourcePath); err != nil {
		return fmt.Errorf("源路径不存在：%w", err)
	}

	// 根据策略选择备份方式
	switch task.Strategy {
	case StrategyFull:
		return m.doFullBackup(ctx, task)
	case StrategyIncremental:
		return m.doIncrementalBackup(ctx, task)
	case StrategyDifferential:
		return m.doDifferentialBackup(ctx, task)
	default:
		return m.doFullBackup(ctx, task)
	}
}

// doFullBackup 全量备份.
func (m *Manager) doFullBackup(_ context.Context, task *BackupTask) error {
	// 全量备份 - 实际项目中会调用 tar/rsync 等
	task.TotalFiles = 100
	task.TotalSize = 1024 * 1024 * 100 // 100MB 估算
	task.Speed = 1024 * 1024 * 10       // 10MB/s
	task.Progress = 100
	return nil
}

// doIncrementalBackup 增量备份.
func (m *Manager) doIncrementalBackup(_ context.Context, task *BackupTask) error {
	// 增量备份 - 只备份自上次以来变更的数据
	task.TotalFiles = 20
	task.TotalSize = 1024 * 1024 * 10 // 10MB 估算
	task.Speed = 1024 * 1024 * 20      // 20MB/s
	task.Progress = 100
	return nil
}

// doDifferentialBackup 差异备份.
func (m *Manager) doDifferentialBackup(_ context.Context, task *BackupTask) error {
	// 差异备份 - 备份自上次全量以来的所有变更
	task.TotalFiles = 50
	task.TotalSize = 1024 * 1024 * 50 // 50MB 估算
	task.Speed = 1024 * 1024 * 15      // 15MB/s
	task.Progress = 100
	return nil
}

// isInPeakHours 判断当前是否在业务高峰期.
func (m *Manager) isInPeakHours(cfg *ScheduleConfig) bool {
	if len(cfg.PeakHours) == 0 {
		return false
	}

	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()

	for _, peak := range cfg.PeakHours {
		startParts := strings.Split(peak.Start, ":")
		endParts := strings.Split(peak.End, ":")
		if len(startParts) != 2 || len(endParts) != 2 {
			continue
		}

		startH, startM := parseHourMin(startParts[0], startParts[1])
		endH, endM := parseHourMin(endParts[0], endParts[1])
		startMinutes := startH*60 + startM
		endMinutes := endH*60 + endM

		if startMinutes <= endMinutes {
			if currentMinutes >= startMinutes && currentMinutes <= endMinutes {
				return true
			}
		} else {
			// 跨午夜
			if currentMinutes >= startMinutes || currentMinutes <= endMinutes {
				return true
			}
		}
	}
	return false
}

func parseHourMin(h, m string) (int, int) {
	hour := 0
	minute := 0
	fmt.Sscanf(h, "%d", &hour)
	fmt.Sscanf(m, "%d", &minute)
	return hour, minute
}

// scheduleRetry 安排重试.
func (m *Manager) scheduleRetry(cfg *ScheduleConfig, task *BackupTask) {
	task.RetryCount++
	task.Status = StatusRetrying
	nextRetry := time.Now().Add(cfg.RetryInterval)
	task.NextRetryAt = &nextRetry

	m.appendAudit("retry", cfg.ID, task.ID, "scheduler",
		fmt.Sprintf("安排第 %d 次重试，下次重试时间：%s", task.RetryCount, nextRetry.Format("15:04:05")), true)

	go func() {
		time.Sleep(cfg.RetryInterval)
		m.mu.Lock()
		if task.Status == StatusRetrying {
			m.mu.Unlock()
			ctx, cancel := context.WithCancel(context.Background())
			m.mu.Lock()
			m.cancels[task.ID] = cancel
			m.mu.Unlock()
			m.executeTask(ctx, cfg, task)
		} else {
			m.mu.Unlock()
		}
	}()
}

// degradeTask 降级任务到下一优先级 Tier.
func (m *Manager) degradeTask(cfg *ScheduleConfig, task *BackupTask) {
	// 找到下一个可用的 Tier
	currentTier := task.Tier
	tierOrder := []BackupTier{TierLocal, TierRemote, TierCloud}

	currentIdx := -1
	for i, t := range tierOrder {
		if t == currentTier {
			currentIdx = i
			break
		}
	}

	for i := currentIdx + 1; i < len(tierOrder); i++ {
		for _, tp := range cfg.TargetPaths {
			if tp.Tier == tierOrder[i] && tp.Enabled {
				task.Tier = tp.Tier
				task.TargetPath = tp.Path
				task.Status = StatusDegraded
				task.Strategy = StrategyIncremental // 降级时使用增量策略
				task.RetryCount = 0                 // 重置重试计数

				m.appendAudit("degrade", cfg.ID, task.ID, "scheduler",
					fmt.Sprintf("任务降级：Tier %s → %s，策略改为增量", currentTier, task.Tier), true)

				go func() {
					ctx, cancel := context.WithCancel(context.Background())
					m.mu.Lock()
					m.cancels[task.ID] = cancel
					m.mu.Unlock()
					m.executeTask(ctx, cfg, task)
				}()
				return
			}
		}
	}

	// 无法降级
	task.Status = StatusFailed
	task.Error = "所有备份目标均失败，无法降级"
	m.appendAudit("degrade_failed", cfg.ID, task.ID, "scheduler",
		"降级失败：无可用备用目标", false)
}

// ========== AI 策略推荐 ==========

// RecommendStrategy 基于变更模式推荐备份策略.
func (m *Manager) RecommendStrategy(configID string) *StrategyRecommendation {
	m.mu.RLock()
	pattern, hasPattern := m.patterns[configID]
	m.mu.RUnlock()

	rec := &StrategyRecommendation{
		Recommended: StrategyFull,
		Confidence:  0.5,
		Reasons:     []string{"使用默认全量备份策略"},
	}

	if !hasPattern || pattern.TotalChanges == 0 {
		return rec
	}

	// 基于变更频率和变更率推荐策略
	changeRate := pattern.ChangeRate
	freq := pattern.ChangeFrequency

	reasons := []string{}

	switch {
	case changeRate < 0.01 && freq < 0.1:
		// 变更极少 → 全量备份即可
		rec.Recommended = StrategyFull
		rec.Confidence = 0.9
		reasons = append(reasons, "数据变更率极低（<1%），推荐全量备份")
		rec.EstimatedDuration = 30 * time.Minute
		rec.EstimatedSize = pattern.AvgChangeSize * 100

	case changeRate < 0.05 && freq < 1.0:
		// 变更较少 → 增量备份更高效
		rec.Recommended = StrategyIncremental
		rec.Confidence = 0.85
		reasons = append(reasons, "数据变更率较低（<5%），推荐增量备份以节省存储和时间")
		rec.EstimatedDuration = 10 * time.Minute
		rec.EstimatedSize = int64(float64(pattern.AvgChangeSize) * freq * 24)

	case changeRate < 0.20:
		// 变更中等 → 差异备份
		rec.Recommended = StrategyDifferential
		rec.Confidence = 0.8
		reasons = append(reasons, "数据变更率中等（<20%），推荐差异备份平衡恢复速度和存储")
		rec.EstimatedDuration = 20 * time.Minute
		rec.EstimatedSize = int64(float64(pattern.AvgChangeSize) * freq * 24 * 3)

	default:
		// 变更频繁 → 增量备份
		rec.Recommended = StrategyIncremental
		rec.Confidence = 0.75
		reasons = append(reasons, "数据变更频繁（>20%），推荐高频增量备份")
		rec.EstimatedDuration = 15 * time.Minute
		rec.EstimatedSize = int64(float64(pattern.AvgChangeSize) * freq * 6)
	}

	// 考虑变更高峰
	if pattern.PeakChangeHour >= 9 && pattern.PeakChangeHour <= 18 {
		reasons = append(reasons, fmt.Sprintf("变更高峰在 %d:00，建议避开此时段", pattern.PeakChangeHour))
	}

	rec.Reasons = reasons
	return rec
}

// ========== 风险评估 ==========

// AssessRisk 评估备份风险.
func (m *Manager) AssessRisk(configID string) *RiskAssessment {
	m.mu.RLock()
	cfg, hasCfg := m.configs[configID]
	pattern, hasPattern := m.patterns[configID]
	m.mu.RUnlock()

	assessment := &RiskAssessment{
		Level:      RiskLow,
		Score:      0,
		SuccessRate: 95,
		Factors:    []RiskFactor{},
		Recommendations: []string{},
		AssessedAt: time.Now(),
	}

	if !hasCfg {
		return assessment
	}

	// 因素1：历史成功率
	recentTasks := m.getRecentTasks(configID, 7*24*time.Hour)
	successRate := calculateSuccessRate(recentTasks)
	failFactor := (100 - successRate) * 0.4
	if failFactor > 0 {
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:        "历史成功率",
			Impact:      failFactor,
			Description: fmt.Sprintf("近7天成功率 %.1f%%", successRate),
		})
		assessment.Score += failFactor
	}

	// 因素2：变更频率风险
	if hasPattern && pattern.ChangeFrequency > 10 {
		freqRisk := math.Min(pattern.ChangeFrequency, 30) * 0.5
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:        "变更频率",
			Impact:      freqRisk,
			Description: fmt.Sprintf("每小时变更 %.1f 次", pattern.ChangeFrequency),
		})
		assessment.Score += freqRisk
	}

	// 因素3：存储空间风险
	// 使用默认值模拟（实际项目中会查询真实存储状态）
	storageRisk := 0.0
	assessment.Factors = append(assessment.Factors, RiskFactor{
		Name:        "存储空间",
		Impact:      storageRisk,
		Description: "存储空间充足",
	})
	assessment.Score += storageRisk

	// 因素4：备份窗口风险
	if len(cfg.BackupWindows) == 0 {
		windowRisk := 5.0
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:        "备份窗口",
			Impact:      windowRisk,
			Description: "未配置备份窗口，可能影响业务",
		})
		assessment.Score += windowRisk
	}

	// 计算预测成功率
	assessment.SuccessRate = math.Max(0, 100-assessment.Score)

	// 确定风险等级
	switch {
	case assessment.Score >= 70:
		assessment.Level = RiskCritical
		assessment.Recommendations = append(assessment.Recommendations,
			"风险极高，建议立即检查备份配置和存储状态")
	case assessment.Score >= 50:
		assessment.Level = RiskHigh
		assessment.Recommendations = append(assessment.Recommendations,
			"风险较高，建议增加备份频率或检查存储容量")
	case assessment.Score >= 30:
		assessment.Level = RiskMedium
		assessment.Recommendations = append(assessment.Recommendations,
			"风险中等，建议定期检查备份任务状态")
	default:
		assessment.Level = RiskLow
	}

	return assessment
}

func (m *Manager) getRecentTasks(configID string, duration time.Duration) []*BackupTask {
	cutoff := time.Now().Add(-duration)
	var result []*BackupTask
	for _, t := range m.tasks {
		if t.ConfigID == configID && t.StartTime.After(cutoff) {
			result = append(result, t)
		}
	}
	return result
}

func calculateSuccessRate(tasks []*BackupTask) float64 {
	if len(tasks) == 0 {
		return 100 // 无历史记录默认为100
	}
	success := 0
	for _, t := range tasks {
		if t.Status == StatusCompleted {
			success++
		}
	}
	return float64(success) / float64(len(tasks)) * 100
}

// ========== 容量规划 ==========

// ForecastCapacity 预测存储容量.
func (m *Manager) ForecastCapacity(configID string) (*CapacityForecast, error) {
	m.mu.RLock()
	_, ok := m.configs[configID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("调度配置不存在：%s", configID)
	}

	// 模拟容量数据（实际项目中会查询文件系统）
	forecast := &CapacityForecast{
		CurrentUsage:    1024 * 1024 * 1024 * 50, // 50GB
		TotalCapacity:   1024 * 1024 * 1024 * 200, // 200GB
		UsagePercent:    25.0,
		PredictedGrowth: 1024 * 1024 * 1024 * 5, // 5GB/week
		DaysUntilFull:   210,
		ForecastDate:    time.Now(),
	}

	// 基于变更模式调整预测
	m.mu.RLock()
	if pattern, ok := m.patterns[configID]; ok {
		dailyGrowth := int64(float64(pattern.AvgChangeSize) * pattern.ChangeFrequency * 24)
		if dailyGrowth > 0 {
			forecast.PredictedGrowth = dailyGrowth * 7
			remaining := forecast.TotalCapacity - forecast.CurrentUsage
			if dailyGrowth > 0 {
				forecast.DaysUntilFull = int(remaining / dailyGrowth)
			}
		}
	}
	m.mu.RUnlock()

	return forecast, nil
}

// ========== 变更模式管理 ==========

// UpdateChangePattern 更新变更模式数据.
func (m *Manager) UpdateChangePattern(configID string, pattern *ChangePattern) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patterns[configID] = pattern
}

// GetChangePattern 获取变更模式.
func (m *Manager) GetChangePattern(configID string) (*ChangePattern, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.patterns[configID]
	if !ok {
		return nil, fmt.Errorf("未找到变更模式数据：%s", configID)
	}
	return p, nil
}

// ========== 审计日志 ==========

// GetAuditLog 获取审计日志.
func (m *Manager) GetAuditLog(configID string, limit int) []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []AuditEntry
	// 倒序返回
	for i := len(m.auditLog) - 1; i >= 0; i-- {
		entry := m.auditLog[i]
		if configID != "" && entry.ConfigID != configID {
			continue
		}
		result = append(result, entry)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

func (m *Manager) appendAudit(action, configID, taskID, actor, details string, success bool) {
	entry := AuditEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		Action:    action,
		ConfigID:  configID,
		TaskID:    taskID,
		Actor:     actor,
		Details:   details,
		Success:   success,
	}
	m.auditLog = append(m.auditLog, entry)

	// 限制审计日志条数
	const maxAuditEntries = 10000
	if len(m.auditLog) > maxAuditEntries {
		m.auditLog = m.auditLog[len(m.auditLog)-maxAuditEntries:]
	}
}

// ========== 统计 ==========

// GetStats 获取调度器统计信息.
func (m *Manager) GetStats() *SchedulerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SchedulerStats{}
	stats.TotalConfigs = len(m.configs)

	for _, cfg := range m.configs {
		if cfg.Enabled {
			stats.EnabledConfigs++
		}
	}

	stats.TotalTasks = len(m.tasks)
	var totalDuration time.Duration
	var completedCount int

	for _, t := range m.tasks {
		switch t.Status {
		case StatusRunning:
			stats.RunningTasks++
		case StatusCompleted:
			stats.CompletedTasks++
			if !t.EndTime.IsZero() {
				totalDuration += t.EndTime.Sub(t.StartTime)
			}
			completedCount++
			stats.TotalBackupSize += t.TotalSize
		case StatusFailed:
			stats.FailedTasks++
		case StatusRetrying:
			stats.RetryingTasks++
		}
	}

	total := stats.CompletedTasks + stats.FailedTasks
	if total > 0 {
		stats.SuccessRate = float64(stats.CompletedTasks) / float64(total) * 100
	}
	if completedCount > 0 {
		stats.AvgDuration = int64(totalDuration.Seconds()) / int64(completedCount)
	}

	return stats
}

// ========== 健康检查 ==========

// HealthCheck 健康检查.
func (m *Manager) HealthCheck() *HealthCheckResult {
	result := &HealthCheckResult{
		Status:    "healthy",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	m.mu.RLock()
	result.Details["config_count"] = len(m.configs)
	result.Details["task_count"] = len(m.tasks)
	result.Details["audit_entries"] = len(m.auditLog)

	running := 0
	for _, t := range m.tasks {
		if t.Status == StatusRunning || t.Status == StatusRetrying {
			running++
		}
	}
	result.Details["active_tasks"] = running
	m.mu.RUnlock()

	if running > 10 {
		result.Status = "degraded"
		result.Details["warning"] = "过多活跃任务"
	}

	return result
}

// ========== 调度窗口检查 ==========

// IsInBackupWindow 判断当前时间是否在备份窗口内.
func (m *Manager) IsInBackupWindow(configID string) (bool, string) {
	m.mu.RLock()
	cfg, ok := m.configs[configID]
	m.mu.RUnlock()
	if !ok || len(cfg.BackupWindows) == 0 {
		return true, "" // 无窗口限制，允许随时备份
	}

	now := time.Now()
	currentDay := strings.ToLower(now.Weekday().String())
	currentMinutes := now.Hour()*60 + now.Minute()

	for _, window := range cfg.BackupWindows {
		// 检查日期
		dayMatch := false
		for _, d := range window.Days {
			if strings.EqualFold(d, currentDay) {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			continue
		}

		// 检查时间
		startParts := strings.Split(window.StartTime, ":")
		endParts := strings.Split(window.EndTime, ":")
		if len(startParts) != 2 || len(endParts) != 2 {
			continue
		}

		startH, startM := parseHourMin(startParts[0], startParts[1])
		endH, endM := parseHourMin(endParts[0], endParts[1])
		startMinutes := startH*60 + startM
		endMinutes := endH*60 + endM

		if currentMinutes >= startMinutes && currentMinutes <= endMinutes {
			return true, window.Name
		}
	}

	return false, ""
}

// ========== 清理 ==========

// CleanupCompletedTasks 清理已完成的任务.
func (m *Manager) CleanupCompletedTasks() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, task := range m.tasks {
		if task.Status == StatusCompleted || task.Status == StatusFailed || task.Status == StatusCancelled {
			delete(m.tasks, id)
			delete(m.cancels, id)
			count++
		}
	}
	return count
}

// ========== 持久化 ==========

func (m *Manager) loadConfigs() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &m.configs)
}

func (m *Manager) saveConfigs() error {
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.configs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.configPath, data, 0600)
}

func (m *Manager) loadAuditLog() error {
	data, err := os.ReadFile(m.auditPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &m.auditLog)
}

func (m *Manager) saveAuditLog() error {
	dir := filepath.Dir(m.auditPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		log.Printf("创建审计日志目录失败：%v", err)
		return nil // 非致命错误
	}
	data, err := json.MarshalIndent(m.auditLog, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.auditPath, data, 0600)
}

// ========== 辅助函数 ==========

func generateID() string {
	return time.Now().Format("20060102-150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(1) // 保证不同位有不同值
	}
	return string(b)
}
