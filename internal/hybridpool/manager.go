package hybridpool

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 混合存储池管理器.
type Manager struct {
	mu sync.RWMutex

	logger *zap.Logger
	config *ManagerConfig

	// 存储池
	pools map[string]*HybridPool

	// 迁移任务
	tasks map[string]*MigrationTask

	// 性能监控
	performance *PerformanceCollector

	// 容量预测
	predictor *CapacityPredictor
}

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	MonitorInterval time.Duration `json:"monitorInterval"` // 监控间隔
	RetentionDays   int           `json:"retentionDays"`   // 数据保留天数
	EnableAutoTier  bool          `json:"enableAutoTier"`  // 启用自动分层
}

// DefaultManagerConfig 默认配置.
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		MonitorInterval: 1 * time.Minute,
		RetentionDays:   30,
		EnableAutoTier:  true,
	}
}

// NewManager 创建混合存储池管理器.
func NewManager(logger *zap.Logger, config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}

	m := &Manager{
		logger:      logger,
		config:      config,
		pools:       make(map[string]*HybridPool),
		tasks:       make(map[string]*MigrationTask),
		performance: NewPerformanceCollector(),
		predictor:   NewCapacityPredictor(),
	}

	return m
}

// Initialize 初始化管理器.
func (m *Manager) Initialize() error {
	m.logger.Info("初始化混合存储池管理器")

	// 启动性能监控
	go m.startPerformanceMonitor()

	return nil
}

// CreatePool 创建混合存储池.
func (m *Manager) CreatePool(req *CreatePoolRequest) (*HybridPool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证请求
	if req.Name == "" {
		return nil, fmt.Errorf("池名称不能为空")
	}

	// 创建池配置
	pool := &HybridPool{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Status:      PoolStatusOnline,
		Description: req.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		FlashTier: &TierConfig{
			Type:     TierTypeFlash,
			Name:     "Flash 层",
			Devices:  req.FlashDevices,
			Path:     req.FlashPath,
			Capacity: req.FlashCapacity,
			Enabled:  true,
			Healthy:  true,
		},
		HDDTier: &TierConfig{
			Type:     TierTypeHDD,
			Name:     "HDD 层",
			Devices:  req.HDDDevices,
			Path:     req.HDDPath,
			Capacity: req.HDDCapacity,
			Enabled:  true,
			Healthy:  true,
		},
		MigrationPolicy: &MigrationPolicy{
			ID:                      uuid.New().String(),
			Name:                    "默认迁移策略",
			Enabled:                 true,
			Trigger:                 MigrationTriggerAccess,
			HotAccessCount:          100,
			HotAccessWindow:         24 * time.Hour,
			ColdAgeThreshold:        30 * 24 * time.Hour,
			ColdAccessCount:         5,
			MaxConcurrentMigrations: 3,
			ReserveFlashPct:         10,
			VerifyAfterMove:         true,
		},
		TotalCapacity: req.FlashCapacity + req.HDDCapacity,
	}

	m.pools[pool.ID] = pool
	m.logger.Info("创建混合存储池成功",
		zap.String("poolId", pool.ID),
		zap.String("name", pool.Name),
	)

	return pool, nil
}

// GetPool 获取存储池.
func (m *Manager) GetPool(poolID string) (*HybridPool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池不存在: %s", poolID)
	}

	return pool, nil
}

// ListPools 列出所有存储池.
func (m *Manager) ListPools() []*HybridPool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pools := make([]*HybridPool, 0, len(m.pools))
	for _, pool := range m.pools {
		pools = append(pools, pool)
	}

	return pools
}

// DeletePool 删除存储池.
func (m *Manager) DeletePool(poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return fmt.Errorf("存储池不存在: %s", poolID)
	}

	if pool.Status == PoolStatusOnline && pool.UsedCapacity > 0 {
		return fmt.Errorf("存储池仍有数据，无法删除")
	}

	delete(m.pools, poolID)
	m.logger.Info("删除存储池成功", zap.String("poolId", poolID))

	return nil
}

// AddTier 添加存储层.
func (m *Manager) AddTier(poolID string, tierType TierType, config *TierConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return fmt.Errorf("存储池不存在: %s", poolID)
	}

	switch tierType {
	case TierTypeFlash:
		pool.FlashTier = config
	case TierTypeHDD:
		pool.HDDTier = config
	default:
		return fmt.Errorf("不支持的存储层类型: %s", tierType)
	}

	pool.TotalCapacity = pool.FlashTier.Capacity + pool.HDDTier.Capacity
	pool.UpdatedAt = time.Now()

	m.logger.Info("添加存储层成功",
		zap.String("poolId", poolID),
		zap.String("tierType", string(tierType)),
	)

	return nil
}

// SetMigrationPolicy 设置迁移策略.
func (m *Manager) SetMigrationPolicy(poolID string, policy *MigrationPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return fmt.Errorf("存储池不存在: %s", poolID)
	}

	pool.MigrationPolicy = policy
	pool.UpdatedAt = time.Now()

	m.logger.Info("设置迁移策略成功",
		zap.String("poolId", poolID),
		zap.String("policyId", policy.ID),
	)

	return nil
}

// GetPoolStats 获取存储池统计.
func (m *Manager) GetPoolStats(poolID string) (*PoolStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池不存在: %s", poolID)
	}

	stats := &PoolStats{
		PoolID:        pool.ID,
		PoolName:      pool.Name,
		Timestamp:     time.Now(),
		TotalCapacity: pool.TotalCapacity,
		UsedCapacity:  pool.UsedCapacity,
		FreeCapacity:  pool.TotalCapacity - pool.UsedCapacity,
		FlashCapacity: pool.FlashTier.Capacity,
		FlashUsed:     pool.FlashUsed,
		FlashFree:     pool.FlashTier.Capacity - pool.FlashUsed,
		HDDCapacity:   pool.HDDTier.Capacity,
		HDDUsed:       pool.HDDUsed,
		HDDFree:       pool.HDDTier.Capacity - pool.HDDUsed,
		Performance:   m.performance.GetLatest(poolID),
	}

	if pool.TotalCapacity > 0 {
		stats.UsagePercent = float64(pool.UsedCapacity) / float64(pool.TotalCapacity) * 100
	}
	if pool.FlashTier.Capacity > 0 {
		stats.FlashUsagePct = float64(pool.FlashUsed) / float64(pool.FlashTier.Capacity) * 100
	}
	if pool.HDDTier.Capacity > 0 {
		stats.HDDUsagePct = float64(pool.HDDUsed) / float64(pool.HDDTier.Capacity) * 100
	}

	return stats, nil
}

// MigrateData 触发数据迁移.
func (m *Manager) MigrateData(poolID string, req *DataMigrationRequest) (*MigrationTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池不存在: %s", poolID)
	}

	if pool.Status != PoolStatusOnline {
		return nil, fmt.Errorf("存储池状态异常: %s", pool.Status)
	}

	task := &MigrationTask{
		ID:         uuid.New().String(),
		PoolID:     poolID,
		SourceTier: req.SourceTier,
		TargetTier: req.TargetTier,
		SourcePath: req.SourcePath,
		TargetPath: req.TargetPath,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	m.tasks[task.ID] = task
	m.logger.Info("创建迁移任务",
		zap.String("taskId", task.ID),
		zap.String("poolId", poolID),
		zap.String("source", string(req.SourceTier)),
		zap.String("target", string(req.TargetTier)),
	)

	// 异步执行迁移
	go m.executeMigration(task)

	return task, nil
}

// GetMigrationTask 获取迁移任务.
func (m *Manager) GetMigrationTask(taskID string) (*MigrationTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("迁移任务不存在: %s", taskID)
	}

	return task, nil
}

// ListMigrationTasks 列出迁移任务.
func (m *Manager) ListMigrationTasks(poolID string) []*MigrationTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*MigrationTask, 0)
	for _, task := range m.tasks {
		if poolID == "" || task.PoolID == poolID {
			tasks = append(tasks, task)
		}
	}

	return tasks
}

// PredictCapacity 预测容量.
func (m *Manager) PredictCapacity(poolID string) (*CapacityPrediction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池不存在: %s", poolID)
	}

	prediction := m.predictor.Predict(pool)
	return prediction, nil
}

// executeMigration 执行迁移任务.
func (m *Manager) executeMigration(task *MigrationTask) {
	m.mu.Lock()
	task.Status = "running"
	task.StartedAt = time.Now()
	m.mu.Unlock()

	m.logger.Info("开始执行迁移任务", zap.String("taskId", task.ID))

	// 模拟迁移过程
	time.Sleep(2 * time.Second)

	m.mu.Lock()
	task.Status = "completed"
	task.EndedAt = time.Now()
	task.Progress = 100
	m.mu.Unlock()

	m.logger.Info("迁移任务完成", zap.String("taskId", task.ID))
}

// startPerformanceMonitor 启动性能监控.
func (m *Manager) startPerformanceMonitor() {
	ticker := time.NewTicker(m.config.MonitorInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.collectPerformanceMetrics()
	}
}

// collectPerformanceMetrics 收集性能指标.
func (m *Manager) collectPerformanceMetrics() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for poolID, pool := range m.pools {
		if pool.Status == PoolStatusOnline {
			metrics := m.performance.Collect(poolID, pool)
			pool.Performance = metrics
		}
	}
}

// CreatePoolRequest 创建池请求.
type CreatePoolRequest struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	FlashDevices  []string `json:"flashDevices"`
	FlashPath     string   `json:"flashPath"`
	FlashCapacity int64    `json:"flashCapacity"`
	HDDDevices    []string `json:"hddDevices"`
	HDDPath       string   `json:"hddPath"`
	HDDCapacity   int64    `json:"hddCapacity"`
}

// PerformanceCollector 性能收集器.
type PerformanceCollector struct {
	metrics map[string][]*PerformanceMetrics
}

// NewPerformanceCollector 创建性能收集器.
func NewPerformanceCollector() *PerformanceCollector {
	return &PerformanceCollector{
		metrics: make(map[string][]*PerformanceMetrics),
	}
}

// Collect 收集性能指标.
func (pc *PerformanceCollector) Collect(poolID string, pool *HybridPool) *PerformanceMetrics {
	// 这里实现实际的性能收集逻辑
	// 简化版本返回模拟数据
	metrics := &PerformanceMetrics{
		Timestamp:       time.Now(),
		FlashIOPS:       10000,
		HDDIOPS:         200,
		TotalIOPS:       10200,
		FlashThroughput: 500 * 1024 * 1024, // 500 MB/s
		HDDThroughput:   200 * 1024 * 1024, // 200 MB/s
		TotalThroughput: 700 * 1024 * 1024, // 700 MB/s
		FlashLatencyAvg: 100,               // 100μs
		HDDLatencyAvg:   5000,              // 5ms
		TotalLatencyAvg: 200,
		CacheHitRate:    85.5,
		CacheMissRate:   14.5,
	}

	pc.metrics[poolID] = append(pc.metrics[poolID], metrics)

	// 保留最近 100 条记录
	if len(pc.metrics[poolID]) > 100 {
		pc.metrics[poolID] = pc.metrics[poolID][len(pc.metrics[poolID])-100:]
	}

	return metrics
}

// GetLatest 获取最新性能指标.
func (pc *PerformanceCollector) GetLatest(poolID string) *PerformanceMetrics {
	metrics, exists := pc.metrics[poolID]
	if !exists || len(metrics) == 0 {
		return nil
	}
	return metrics[len(metrics)-1]
}

// CapacityPredictor 容量预测器.
type CapacityPredictor struct {
}

// NewCapacityPredictor 创建容量预测器.
func NewCapacityPredictor() *CapacityPredictor {
	return &CapacityPredictor{}
}

// Predict 预测容量.
func (cp *CapacityPredictor) Predict(pool *HybridPool) *CapacityPrediction {
	// 简化版本的容量预测逻辑
	// 实际实现应该基于历史数据进行趋势分析
	dailyGrowth := int64(100 * 1024 * 1024) // 100MB/day

	prediction := &CapacityPrediction{
		PoolID:           pool.ID,
		PredictedAt:      time.Now(),
		CurrentUsage:     pool.UsedCapacity,
		DailyGrowthBytes: dailyGrowth,
		GrowthRate:       0.1, // 0.1% per day
	}

	if pool.TotalCapacity > 0 {
		prediction.CurrentUsagePct = float64(pool.UsedCapacity) / float64(pool.TotalCapacity) * 100

		remaining := pool.TotalCapacity - pool.UsedCapacity
		if dailyGrowth > 0 {
			prediction.DaysUntilFull = int(remaining / dailyGrowth)
			prediction.PredictedFullDate = time.Now().AddDate(0, 0, prediction.DaysUntilFull)
		}
	}

	// 添加建议
	if prediction.DaysUntilFull < 30 {
		prediction.Recommendations = append(prediction.Recommendations, "容量将在30天内用尽，建议扩容或清理数据")
	}
	if prediction.DaysUntilFull < 7 {
		prediction.Recommendations = append(prediction.Recommendations, "容量即将用尽，请立即采取行动")
	}

	return prediction
}
