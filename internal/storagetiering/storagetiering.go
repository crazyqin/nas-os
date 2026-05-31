package storagetiering

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Engine 智能存储分层引擎
// 协调分析器、策略引擎、迁移器，实现 SSD/HDD/Cold 三级自动分层
type Engine struct {
	mu     sync.RWMutex
	config Config
	logger *zap.Logger

	analyzer *Analyzer
	policy   *Policy
	migrator *Migrator

	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewEngine 创建智能存储分层引擎
func NewEngine(config Config, backend StorageBackend, logger *zap.Logger) (*Engine, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	analyzer := NewAnalyzer(config.Analyzer, config.Policy, logger)
	policy := NewPolicy(config.Policy, config.Tiers, logger)
	migrator := NewMigrator(config.Migrator, backend, logger)

	return &Engine{
		config:   config,
		logger:   logger,
		analyzer: analyzer,
		policy:   policy,
		migrator: migrator,
		stopCh:   make(chan struct{}),
	}, nil
}

// Start 启动分层引擎
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("engine already running")
	}
	e.running = true
	e.stopCh = make(chan struct{})
	e.mu.Unlock()

	// 启动迁移器
	if err := e.migrator.Start(ctx); err != nil {
		e.logger.Error("failed to start migrator", zap.Error(err))
	}

	// 启动周期性分析
	e.wg.Add(1)
	go e.analysisLoop(ctx)

	// 启动容量监控
	e.wg.Add(1)
	go e.capacityMonitor(ctx)

	e.logger.Info("storage tiering engine started")
	return nil
}

// Stop 停止分层引擎
func (e *Engine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	close(e.stopCh)
	e.mu.Unlock()

	e.migrator.Stop()
	e.wg.Wait()
	e.logger.Info("storage tiering engine stopped")
}

// RegisterFile 注册文件到分层系统
func (e *Engine) RegisterFile(entry FileEntry) {
	e.analyzer.RegisterFile(entry)
}

// RecordAccess 记录文件访问
func (e *Engine) RecordAccess(record AccessRecord) {
	e.analyzer.RecordAccess(record)
}

// RunAnalysis 手动触发分析
func (e *Engine) RunAnalysis(ctx context.Context) (int, error) {
	tasks, err := e.analyzer.Analyze(ctx)
	if err != nil {
		return 0, err
	}

	if len(tasks) == 0 {
		return 0, nil
	}

	// 过滤并提交迁移任务
	submitted := e.migrator.SubmitBatch(tasks)
	e.logger.Info("analysis triggered migration",
		zap.Int("candidates", len(tasks)),
		zap.Int("submitted", submitted))
	return submitted, nil
}

// Stats 返回统计信息
func (e *Engine) Stats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var tierStats []TierStats
	for _, tc := range e.config.Tiers {
		used, total := e.policy.GetTierUsage(tc.Tier)
		free := total - used
		if free < 0 {
			free = 0
		}
		var ratio float64
		if total > 0 {
			ratio = float64(used) / float64(total)
		}
		tierStats = append(tierStats, TierStats{
			Tier:       tc.Tier,
			TotalBytes: total,
			UsedBytes:  used,
			FreeBytes:  free,
			UsageRatio: ratio,
		})
	}

	history := e.migrator.GetHistory(20)

	return Stats{
		Tiers:            tierStats,
		TotalMigrations:  e.migrator.TotalMigrations(),
		ActiveMigrations: e.migrator.ActiveCount(),
		HitRate:          e.analyzer.HitRate(),
		RecentHistory:    history,
		LastAnalysis:     e.analyzer.LastAnalysis(),
	}
}

// Migrator 返回迁移器引用
func (e *Engine) Migrator() *Migrator {
	return e.migrator
}

// Analyzer 返回分析器引用
func (e *Engine) Analyzer() *Analyzer {
	return e.analyzer
}

// Policy 返回策略引用
func (e *Engine) Policy() *Policy {
	return e.policy
}

// PauseMigrations 暂停迁移
func (e *Engine) PauseMigrations() error {
	return e.migrator.Pause()
}

// ResumeMigrations 恢复迁移
func (e *Engine) ResumeMigrations() error {
	return e.migrator.Resume()
}

// CancelMigration 取消迁移任务
func (e *Engine) CancelMigration(taskID string) error {
	return e.migrator.CancelTask(taskID)
}

// ============================================================
// 内部循环
// ============================================================

// analysisLoop 周期性分析循环
func (e *Engine) analysisLoop(ctx context.Context) {
	defer e.wg.Done()

	interval := e.config.Analyzer.AnalysisInterval
	if interval <= 0 {
		interval = 30 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			tasks, err := e.analyzer.Analyze(ctx)
			if err != nil {
				e.logger.Error("analysis failed", zap.Error(err))
				continue
			}
			if len(tasks) > 0 {
				e.migrator.SubmitBatch(tasks)
			}
		}
	}
}

// capacityMonitor 容量监控循环
func (e *Engine) capacityMonitor(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.checkCapacity()
		}
	}
}

// checkCapacity 检查各层容量，触发紧急迁移
func (e *Engine) checkCapacity() {
	for _, tc := range e.config.Tiers {
		if e.policy.NeedsEviction(tc.Tier) {
			ratio := e.policy.TierUsageRatio(tc.Tier)
			e.logger.Warn("tier capacity threshold exceeded",
				zap.String("tier", tc.Tier.String()),
				zap.Float64("usage_ratio", ratio),
				zap.Float64("threshold", e.policy.GetConfig().CapacityHighPct))

			// 标记需要紧急迁移
			// 具体的驱逐逻辑由分析器在下次分析时处理
		}
	}
}
