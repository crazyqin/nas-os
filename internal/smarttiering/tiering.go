package smarttiering

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 智能分层管理器
// 协调预测器、迁移引擎、成本优化器和监控器
type Manager struct {
	mu     sync.RWMutex
	config Config
	logger *zap.Logger

	predictor     *Predictor
	migrator      *Migrator
	costOptimizer *CostOptimizer
	monitor       *Monitor

	policies map[string]*TierPolicy

	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewManager 创建智能分层管理器
func NewManager(config Config, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	predictor := NewPredictor(config.Predictor, logger)
	migrator := NewMigrator(config.Migrator, predictor, logger)
	costOptimizer := NewCostOptimizer(config.CostOptimizer, predictor, config.TierCosts, logger)
	monitor := NewMonitor(config.Monitor, predictor, migrator, logger)

	return &Manager{
		config:        config,
		logger:        logger,
		predictor:     predictor,
		migrator:      migrator,
		costOptimizer: costOptimizer,
		monitor:       monitor,
		policies:      make(map[string]*TierPolicy),
		stopCh:        make(chan struct{}),
	}
}

// Start 启动智能分层系统
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("smart tiering manager already running")
	}
	m.running = true
	m.mu.Unlock()

	// 启动各组件
	if err := m.migrator.Start(ctx); err != nil {
		m.logger.Error("failed to start migrator", zap.Error(err))
	}

	if err := m.monitor.Start(ctx); err != nil {
		m.logger.Error("failed to start monitor", zap.Error(err))
	}

	// 启动热度评分更新器
	m.wg.Add(1)
	go m.heatUpdater(ctx)

	m.logger.Info("smart tiering manager started")
	return nil
}

// Stop 停止智能分层系统
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopCh)
	m.mu.Unlock()

	m.monitor.Stop()
	m.migrator.Stop()

	m.wg.Wait()
	m.logger.Info("smart tiering manager stopped")
}

// heatUpdater 定期更新热度评分
func (m *Manager) heatUpdater(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Duration(m.config.Predictor.UpdateIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			count, err := m.predictor.UpdateHeatScores(ctx)
			if err != nil {
				m.logger.Error("failed to update heat scores", zap.Error(err))
			} else {
				m.logger.Debug("heat scores updated", zap.Int("files", count))
			}
		}
	}
}

// RecordAccess 记录文件访问
func (m *Manager) RecordAccess(record AccessRecord) {
	m.predictor.RecordAccess(record)

	// 更新监控命中率
	tier, _ := m.predictor.PredictTier(record.Path, m.config.Migrator)
	if record.OpType == "read" {
		meta, exists := m.predictor.GetAllFiles()[record.Path]
		if exists && meta.CurrentTier == tier {
			m.monitor.RecordAccessHit(tier)
		} else {
			m.monitor.RecordAccessMiss(tier)
		}
	}
}

// RegisterFile 注册文件
func (m *Manager) RegisterFile(meta FileMetadata) {
	m.predictor.RegisterFile(meta)
}

// GetFileHeat 获取文件热度
func (m *Manager) GetFileHeat(path string) (float64, bool) {
	return m.predictor.GetFileHeat(path)
}

// GetPredictorConfig 获取预测器配置
func (m *Manager) GetPredictorConfig() PredictorConfig {
	return m.predictor.GetConfig()
}

// UpdatePredictorConfig 更新预测器配置
func (m *Manager) UpdatePredictorConfig(config PredictorConfig) {
	m.predictor.UpdateConfig(config)
}

// GetMigratorConfig 获取迁移器配置
func (m *Manager) GetMigratorConfig() MigratorConfig {
	return m.migrator.GetConfig()
}

// UpdateMigratorConfig 更新迁移器配置
func (m *Manager) UpdateMigratorConfig(config MigratorConfig) {
	m.migrator.UpdateConfig(config)
}

// GetCostOptimizerConfig 获取成本优化器配置
func (m *Manager) GetCostOptimizerConfig() CostOptimizerConfig {
	return m.costOptimizer.GetConfig()
}

// UpdateCostOptimizerConfig 更新成本优化器配置
func (m *Manager) UpdateCostOptimizerConfig(config CostOptimizerConfig) {
	m.costOptimizer.UpdateConfig(config)
}

// GetMonitorConfig 获取监控器配置
func (m *Manager) GetMonitorConfig() MonitorConfig {
	return m.monitor.GetConfig()
}

// UpdateMonitorConfig 更新监控器配置
func (m *Manager) UpdateMonitorConfig(config MonitorConfig) {
	m.monitor.UpdateConfig(config)
}

// GetCostReport 获取成本报告
func (m *Manager) GetCostReport(ctx context.Context) (*CostReport, error) {
	return m.costOptimizer.GenerateReport(ctx)
}

// GetMetrics 获取最新指标
func (m *Manager) GetMetrics() *TieringMetrics {
	return m.monitor.GetLatestMetrics()
}

// GetMetricsHistory 获取指标历史
func (m *Manager) GetMetricsHistory(limit int) []TieringMetrics {
	return m.monitor.GetMetricsHistory(limit)
}

// GetMigrationEvents 获取迁移事件
func (m *Manager) GetMigrationEvents(limit int) []MigrationEvent {
	return m.migrator.GetMigrationEvents(limit)
}

// GetTierSummary 获取层级摘要
func (m *Manager) GetTierSummary() map[string]interface{} {
	return m.monitor.GetTierSummary()
}

// ForceMigrate 强制迁移文件
func (m *Manager) ForceMigrate(ctx context.Context, path string, fromTier, toTier StorageTier, fileSize int64) error {
	return m.migrator.ForceMigrate(ctx, path, fromTier, toTier, fileSize)
}

// IsRunning 检查是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetConfig 获取总配置
func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新总配置
func (m *Manager) UpdateConfig(config Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	m.predictor.UpdateConfig(config.Predictor)
	m.migrator.UpdateConfig(config.Migrator)
	m.costOptimizer.UpdateConfig(config.CostOptimizer)
	m.monitor.UpdateConfig(config.Monitor)
}

// ============================================================
// 分层策略管理
// ============================================================

// ListPolicies 列出所有分层策略
func (m *Manager) ListPolicies() []*TierPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*TierPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		cp := *p
		result = append(result, &cp)
	}
	return result
}

// CreatePolicy 创建分层策略
func (m *Manager) CreatePolicy(req TierPolicy) (*TierPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("策略名称不能为空")
	}

	if req.ID == "" {
		req.ID = fmt.Sprintf("policy-%d", time.Now().UnixNano())
	}

	// 为规则生成 ID
	for i := range req.Rules {
		if req.Rules[i].ID == "" {
			req.Rules[i].ID = fmt.Sprintf("rule-%d-%d", time.Now().UnixNano(), i)
		}
	}

	now := time.Now()
	req.CreatedAt = now
	req.UpdatedAt = now
	if req.Priority == 0 {
		req.Priority = 100
	}

	m.policies[req.ID] = &req
	m.logger.Info("tier policy created", zap.String("id", req.ID), zap.String("name", req.Name))
	return &req, nil
}

// DeletePolicy 删除分层策略
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("策略 %s 不存在", id)
	}
	delete(m.policies, id)
	m.logger.Info("tier policy deleted", zap.String("id", id))
	return nil
}

// GetPolicy 获取单个策略
func (m *Manager) GetPolicy(id string) (*TierPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("策略 %s 不存在", id)
	}
	cp := *p
	return &cp, nil
}

// ============================================================
// 数据放置
// ============================================================

// GetPlacement 获取文件推荐放置层级
func (m *Manager) GetPlacement(filePath string) (*DataPlacement, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := m.predictor.GetAllFiles()
	meta, exists := files[filePath]
	if !exists {
		return nil, fmt.Errorf("文件 %s 未注册", filePath)
	}

	predictedTier, _ := m.predictor.PredictTier(filePath, m.config.Migrator)

	placement := &DataPlacement{
		FilePath:        filePath,
		CurrentTier:     meta.CurrentTier,
		RecommendedTier: predictedTier,
		HeatScore:       meta.HeatScore,
		FileSize:        meta.Size,
		LastAccess:      meta.AccessedAt,
		AccessCount:     meta.AccessCount,
		Confidence:      0.85,
	}

	if meta.CurrentTier == predictedTier {
		placement.Reason = "当前层级已最优"
	} else {
		placement.Reason = fmt.Sprintf("热度评分 %.1f 建议从 %s 迁移到 %s", meta.HeatScore, meta.CurrentTier, predictedTier)
	}

	return placement, nil
}

// GetPlacements 批量获取文件放置推荐
func (m *Manager) GetPlacements() []DataPlacement {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := m.predictor.GetAllFiles()
	result := make([]DataPlacement, 0, len(files))
	for path, meta := range files {
		predictedTier, _ := m.predictor.PredictTier(path, m.config.Migrator)
		placement := DataPlacement{
			FilePath:        path,
			CurrentTier:     meta.CurrentTier,
			RecommendedTier: predictedTier,
			HeatScore:       meta.HeatScore,
			FileSize:        meta.Size,
			LastAccess:      meta.AccessedAt,
			AccessCount:     meta.AccessCount,
			Confidence:      0.85,
		}
		if meta.CurrentTier == predictedTier {
			placement.Reason = "当前层级已最优"
		} else {
			placement.Reason = fmt.Sprintf("建议迁移到 %s", predictedTier)
		}
		result = append(result, placement)
	}
	return result
}

// ============================================================
// 统计
// ============================================================

// GetStats 获取分层统计
func (m *Manager) GetStats() *TierStats {
	metrics := m.monitor.GetLatestMetrics()
	if metrics == nil {
		return &TierStats{
			GeneratedAt:      time.Now(),
			TierDistribution: make(map[string]int64),
			TierSizesGB:      make(map[string]float64),
			AvgHeatScores:    make(map[string]float64),
			HitRates:         make(map[string]float64),
		}
	}

	policies := m.ListPolicies()
	activeMigrations := len(m.migrator.GetMigrationEvents(1000))

	stats := &TierStats{
		GeneratedAt:      metrics.Timestamp,
		TotalFiles:       metrics.TotalFiles,
		TotalSizeGB:      metrics.TotalSizeGB,
		TierDistribution: make(map[string]int64),
		TierSizesGB:      make(map[string]float64),
		AvgHeatScores:    make(map[string]float64),
		HitRates:         make(map[string]float64),
		MigrationCount:   metrics.MigrationCount,
		MigrationBytesGB: metrics.MigrationBytesGB,
		PolicyCount:      len(policies),
		ActiveMigrations: activeMigrations,
	}

	for tier, count := range metrics.TierDistribution {
		stats.TierDistribution[tier.String()] = count
	}
	for tier, size := range metrics.TierSizesGB {
		stats.TierSizesGB[tier.String()] = size
	}
	for tier, score := range metrics.AvgHeatScores {
		stats.AvgHeatScores[tier.String()] = score
	}
	for tier, rate := range metrics.HitRates {
		stats.HitRates[tier.String()] = rate
	}

	return stats
}

// GetAccessPattern 获取文件访问模式
func (m *Manager) GetAccessPattern(filePath string) (*AccessPattern, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := m.predictor.GetAllFiles()
	meta, exists := files[filePath]
	if !exists {
		return nil, fmt.Errorf("文件 %s 未注册", filePath)
	}

	var pattern string
	switch {
	case meta.HeatScore >= 70:
		pattern = "steady"
	case meta.HeatScore >= 40:
		pattern = "periodic"
	case meta.HeatScore >= 15:
		pattern = "burst"
	default:
		pattern = "cold"
	}

	predictedTier, _ := m.predictor.PredictTier(filePath, m.config.Migrator)

	return &AccessPattern{
		FilePath:      filePath,
		TotalAccesses: meta.AccessCount,
		ReadCount:     meta.ReadCount,
		WriteCount:    meta.WriteCount,
		LastAccess:    meta.AccessedAt,
		Pattern:       pattern,
		HeatScore:     meta.HeatScore,
		PredictedTier: predictedTier,
	}, nil
}
