package smarttiering

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Monitor 性能监控器
// 实时监控分层效果，收集各层级命中率和性能指标
type Monitor struct {
	mu        sync.RWMutex
	config    MonitorConfig
	logger    *zap.Logger
	predictor *Predictor
	migrator  *Migrator

	// 指标存储
	metrics    []TieringMetrics
	accessHits map[StorageTier]int64 // 各层级命中次数
	accessMiss map[StorageTier]int64 // 各层级未命中次数

	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewMonitor 创建性能监控器
func NewMonitor(config MonitorConfig, predictor *Predictor, migrator *Migrator, logger *zap.Logger) *Monitor {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Monitor{
		config:     config,
		logger:     logger,
		predictor:  predictor,
		migrator:   migrator,
		metrics:    make([]TieringMetrics, 0),
		accessHits: make(map[StorageTier]int64),
		accessMiss: make(map[StorageTier]int64),
		stopCh:     make(chan struct{}),
	}
}

// Start 启动监控
func (m *Monitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.mu.Unlock()

	m.wg.Add(1)
	go m.collector(ctx)

	m.logger.Info("tiering monitor started",
		zap.Int("interval_sec", m.config.MetricsIntervalSec))
	return nil
}

// Stop 停止监控
func (m *Monitor) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopCh)
	m.mu.Unlock()

	m.wg.Wait()
	m.logger.Info("tiering monitor stopped")
}

// collector 指标收集器
func (m *Monitor) collector(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Duration(m.config.MetricsIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.collectMetrics()
		}
	}
}

// collectMetrics 收集当前指标
func (m *Monitor) collectMetrics() {
	files := m.predictor.GetAllFiles()
	if len(files) == 0 {
		return
	}

	metrics := TieringMetrics{
		Timestamp:      time.Now(),
		TierDistribution: make(map[StorageTier]int64),
		TierSizesGB:    make(map[StorageTier]float64),
		AvgHeatScores:  make(map[StorageTier]float64),
		HitRates:       make(map[StorageTier]float64),
	}

	// 统计各层级文件数和大小
	heatSums := make(map[StorageTier]float64)
	heatCounts := make(map[StorageTier]int64)

	for _, meta := range files {
		metrics.TierDistribution[meta.CurrentTier]++
		sizeGB := float64(meta.Size) / (1024 * 1024 * 1024)
		metrics.TierSizesGB[meta.CurrentTier] += sizeGB
		heatSums[meta.CurrentTier] += meta.HeatScore
		heatCounts[meta.CurrentTier]++
		metrics.TotalFiles++
		metrics.TotalSizeGB += sizeGB
	}

	// 计算平均热度
	for tier, sum := range heatSums {
		count := heatCounts[tier]
		if count > 0 {
			metrics.AvgHeatScores[tier] = sum / float64(count)
		}
	}

	// 计算命中率
	m.mu.RLock()
	for tier := StorageTier(0); tier <= TierArchive; tier++ {
		hits := m.accessHits[tier]
		misses := m.accessMiss[tier]
		total := hits + misses
		if total > 0 {
			metrics.HitRates[tier] = float64(hits) / float64(total) * 100
		}
	}
	m.mu.RUnlock()

	// 获取迁移统计
	if m.migrator != nil {
		events := m.migrator.GetMigrationEvents(1000)
		metrics.MigrationCount = int64(len(events))
		for _, e := range events {
			if e.Status == "completed" {
				metrics.MigrationBytesGB += float64(e.FileSize) / (1024 * 1024 * 1024)
			}
		}
	}

	m.mu.Lock()
	m.metrics = append(m.metrics, metrics)

	// 裁剪过期指标
	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)
	start := 0
	for i, met := range m.metrics {
		if met.Timestamp.After(cutoff) {
			start = i
			break
		}
	}
	if start > 0 {
		m.metrics = m.metrics[start:]
	}
	m.mu.Unlock()

	m.logger.Debug("metrics collected",
		zap.Int64("total_files", metrics.TotalFiles),
		zap.Float64("total_size_gb", metrics.TotalSizeGB))
}

// RecordAccessHit 记录访问命中
func (m *Monitor) RecordAccessHit(tier StorageTier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accessHits[tier]++
}

// RecordAccessMiss 记录访问未命中
func (m *Monitor) RecordAccessMiss(tier StorageTier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accessMiss[tier]++
}

// GetLatestMetrics 获取最新指标
func (m *Monitor) GetLatestMetrics() *TieringMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.metrics) == 0 {
		return nil
	}
	latest := m.metrics[len(m.metrics)-1]
	return &latest
}

// GetMetricsHistory 获取指标历史
func (m *Monitor) GetMetricsHistory(limit int) []TieringMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.metrics) {
		limit = len(m.metrics)
	}
	start := len(m.metrics) - limit
	result := make([]TieringMetrics, limit)
	copy(result, m.metrics[start:])
	return result
}

// GetTierSummary 获取层级摘要
func (m *Monitor) GetTierSummary() map[string]interface{} {
	latest := m.GetLatestMetrics()
	if latest == nil {
		return map[string]interface{}{
			"status": "no data",
		}
	}

	return map[string]interface{}{
		"total_files":       latest.TotalFiles,
		"total_size_gb":     latest.TotalSizeGB,
		"tier_distribution": latest.TierDistribution,
		"tier_sizes_gb":     latest.TierSizesGB,
		"avg_heat_scores":   latest.AvgHeatScores,
		"hit_rates":         latest.HitRates,
		"migration_count":   latest.MigrationCount,
		"last_updated":      latest.Timestamp,
	}
}

// UpdateConfig 更新配置
func (m *Monitor) UpdateConfig(config MonitorConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// GetConfig 获取配置
func (m *Monitor) GetConfig() MonitorConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}
