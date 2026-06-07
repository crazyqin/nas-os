// Package aistorageoptim 提供AI驱动的存储分层优化引擎
package aistorageoptim

import (
	"log"
	"sync"
	"time"
)

// Manager AI存储优化管理器
type Manager struct {
	policy    TieringPolicy
	optimizer *Optimizer
	predictor *Predictor

	// 文件统计
	fileStats map[string]*FileAccessStats
	statsMu   sync.RWMutex

	// 存储层信息
	tiers  map[StorageTier]*TierConfig
	tierMu sync.RWMutex

	// 优化统计
	optimStats OptimizationStats

	// 迁移记录
	migrations  []MigrationRecord
	migrationMu sync.RWMutex

	// 控制
	running bool
	stopCh  chan struct{}
	mu      sync.RWMutex

	// 回调
	onMigrate func(decision OptimizationDecision) error
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	Policy      TieringPolicy `json:"policy"`
	Tiers       []TierConfig  `json:"tiers"`
	HistorySize int           `json:"historySize"` // 预测器历史大小
}

// DefaultManagerConfig 返回默认配置
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		Policy:      DefaultTieringPolicy(),
		Tiers:       nil,
		HistorySize: 100,
	}
}

// NewManager 创建管理器
func NewManager(config ManagerConfig) *Manager {
	m := &Manager{
		policy:     config.Policy,
		optimizer:  NewOptimizer(config.Policy),
		predictor:  NewPredictor(config.HistorySize),
		fileStats:  make(map[string]*FileAccessStats),
		tiers:      make(map[StorageTier]*TierConfig),
		migrations: make([]MigrationRecord, 0),
		stopCh:     make(chan struct{}),
	}

	// 初始化存储层
	for _, tier := range config.Tiers {
		m.tiers[tier.Tier] = &tier
	}

	return m
}

// SetMigrateCallback 设置迁移回调
func (m *Manager) SetMigrateCallback(fn func(decision OptimizationDecision) error) {
	m.onMigrate = fn
}

// Start 启动管理器
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.analysisLoop()
	log.Printf("[AIStorageOptim] manager started, interval=%v", m.policy.AnalysisInterval)
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	close(m.stopCh)
	m.running = false
	log.Printf("[AIStorageOptim] manager stopped")
}

// IsRunning 返回是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// RecordAccess 记录文件访问
func (m *Manager) RecordAccess(filePath string, fileSize int64, fileType string, bytesRead, bytesWritten int64) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	stats, exists := m.fileStats[filePath]
	if !exists {
		stats = &FileAccessStats{
			FilePath:    filePath,
			FileSize:    fileSize,
			FileType:    fileType,
			CurrentTier: TierHDD, // 默认HDD
		}
		m.fileStats[filePath] = stats
	}

	m.predictor.UpdateAccessStats(stats, bytesRead, bytesWritten, time.Now())

	// 更新访问模式
	stats.AccessPattern = m.predictor.PredictAccessPattern(stats, time.Now())
}

// SetFileTier 设置文件当前层级
func (m *Manager) SetFileTier(filePath string, tier StorageTier) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	if stats, exists := m.fileStats[filePath]; exists {
		stats.CurrentTier = tier
	}
}

// AnalyzeAndOptimize 分析并生成优化决策
func (m *Manager) AnalyzeAndOptimize(path string, dryRun bool) ([]OptimizationDecision, OptimizationStats) {
	m.statsMu.RLock()
	// 复制统计数据
	statsList := make([]*FileAccessStats, 0, len(m.fileStats))
	for _, stats := range m.fileStats {
		statsList = append(statsList, stats)
	}
	m.statsMu.RUnlock()

	// 批量优化
	decisions, stats := m.optimizer.BatchOptimize(statsList, time.Now())

	// 更新统计
	m.mu.Lock()
	m.optimStats = stats
	m.mu.Unlock()

	// 如果不是dryRun，执行迁移
	if !dryRun {
		m.executeDecisions(decisions)
	}

	return decisions, stats
}

// GetOptimizationScores 获取所有文件的优化评分
func (m *Manager) GetOptimizationScores() []OptimizationScore {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()

	now := time.Now()
	scores := make([]OptimizationScore, 0, len(m.fileStats))

	for _, stats := range m.fileStats {
		score := m.optimizer.CalculateScore(stats, now)
		scores = append(scores, score)
	}

	return scores
}

// GetStats 获取优化统计
func (m *Manager) GetStats() OptimizationStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.optimStats

	// 计算存储层使用率
	m.tierMu.RLock()
	for tier, config := range m.tiers {
		if config.Capacity > 0 {
			usage := m.calculateTierUsage(tier)
			switch tier {
			case TierNVMe:
				stats.NVMeUsage = usage
			case TierSSD:
				stats.SSDUsage = usage
			case TierHDD:
				stats.HDDUsage = usage
			}
		}
	}
	m.tierMu.RUnlock()

	return stats
}

// GetFileStats 获取文件统计
func (m *Manager) GetFileStats(filePath string) *FileAccessStats {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()

	if stats, exists := m.fileStats[filePath]; exists {
		// 返回副本
		copy := *stats
		return &copy
	}
	return nil
}

// GetAllFileStats 获取所有文件统计
func (m *Manager) GetAllFileStats() map[string]*FileAccessStats {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()

	result := make(map[string]*FileAccessStats, len(m.fileStats))
	for k, v := range m.fileStats {
		copy := *v
		result[k] = &copy
	}
	return result
}

// GetMigrationHistory 获取迁移历史
func (m *Manager) GetMigrationHistory() []MigrationRecord {
	m.migrationMu.RLock()
	defer m.migrationMu.RUnlock()

	result := make([]MigrationRecord, len(m.migrations))
	copy(result, m.migrations)
	return result
}

// UpdatePolicy 更新分层策略
func (m *Manager) UpdatePolicy(policy TieringPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
	m.optimizer = NewOptimizer(policy)
}

// GetPolicy 获取当前策略
func (m *Manager) GetPolicy() TieringPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policy
}

// executeDecisions 执行优化决策
func (m *Manager) executeDecisions(decisions []OptimizationDecision) {
	for _, decision := range decisions {
		if decision.Action == "keep" {
			continue
		}

		record := MigrationRecord{
			ID:        time.Now().Format("20060102150405") + "-" + decision.FilePath,
			FilePath:  decision.FilePath,
			FromTier:  decision.FromTier,
			ToTier:    decision.ToTier,
			Timestamp: time.Now(),
			Status:    "pending",
			Score:     decision.Score,
		}

		// 执行迁移
		if m.onMigrate != nil {
			record.Status = "running"
			if err := m.onMigrate(decision); err != nil {
				record.Status = "failed"
				record.Error = err.Error()
				log.Printf("[AIStorageOptim] migration failed: %s -> %s: %v", decision.FilePath, decision.ToTier, err)
			} else {
				record.Status = "completed"

				// 更新文件层级
				m.statsMu.Lock()
				if stats, exists := m.fileStats[decision.FilePath]; exists {
					stats.CurrentTier = decision.ToTier
				}
				m.statsMu.Unlock()

				log.Printf("[AIStorageOptim] migrated %s: %s -> %s (score=%.1f)",
					decision.FilePath, decision.FromTier, decision.ToTier, decision.Score)
			}
		}

		// 记录迁移
		m.migrationMu.Lock()
		m.migrations = append(m.migrations, record)
		// 保持历史记录在合理范围内
		if len(m.migrations) > 1000 {
			m.migrations = m.migrations[len(m.migrations)-1000:]
		}
		m.migrationMu.Unlock()
	}
}

// calculateTierUsage 计算存储层使用率
func (m *Manager) calculateTierUsage(tier StorageTier) float64 {
	var totalSize int64
	for _, stats := range m.fileStats {
		if stats.CurrentTier == tier {
			totalSize += stats.FileSize
		}
	}

	config, exists := m.tiers[tier]
	if !exists || config.Capacity <= 0 {
		return 0
	}

	return float64(totalSize) / float64(config.Capacity) * 100
}

// analysisLoop 分析循环
func (m *Manager) analysisLoop() {
	ticker := time.NewTicker(m.policy.AnalysisInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runAnalysis()
		}
	}
}

// runAnalysis 运行分析
func (m *Manager) runAnalysis() {
	decisions, stats := m.AnalyzeAndOptimize("", false)

	if len(decisions) > 0 {
		log.Printf("[AIStorageOptim] analysis completed: %d decisions (promote=%d, demote=%d, avgScore=%.1f)",
			len(decisions), stats.PromoteCount, stats.DemoteCount, stats.AvgScore)
	}
}
