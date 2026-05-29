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

	predictor    *Predictor
	migrator     *Migrator
	costOptimizer *CostOptimizer
	monitor      *Monitor

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
