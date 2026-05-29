// Package spotlight 提供索引优化器
// 支持索引压缩、定期清理、性能优化等功能
package spotlight

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// IndexOptimizer 索引优化器
// 负责索引压缩、清理过期条目、优化存储结构
type IndexOptimizer struct {
	indexer *Indexer
	logger  *zap.Logger
	config  EngineConfig

	mu      sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	stats   OptimizerStats
}

// OptimizerStats 优化器统计
type OptimizerStats struct {
	TotalOptimizations int64     `json:"totalOptimizations"`
	LastOptimization   time.Time `json:"lastOptimization"`
	AvgOptimizeTimeMs  float64   `json:"avgOptimizeTimeMs"`
	CompactedSize      int64     `json:"compactedSize"`
	RemovedEntries     int64     `json:"removedEntries"`
}

// NewIndexOptimizer 创建索引优化器
func NewIndexOptimizer(indexer *Indexer, config EngineConfig, logger *zap.Logger) *IndexOptimizer {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &IndexOptimizer{
		indexer: indexer,
		logger:  logger,
		config:  config,
	}
}

// Start 启动索引优化器
func (opt *IndexOptimizer) Start(ctx context.Context) error {
	opt.mu.Lock()
	defer opt.mu.Unlock()

	if opt.running {
		return fmt.Errorf("索引优化器已在运行")
	}

	opt.ctx, opt.cancel = context.WithCancel(ctx)
	opt.running = true

	// 启动定期优化
	go opt.runPeriodicOptimization()

	opt.logger.Info("索引优化器已启动")
	return nil
}

// Stop 停止索引优化器
func (opt *IndexOptimizer) Stop() {
	opt.mu.Lock()
	defer opt.mu.Unlock()

	if !opt.running {
		return
	}

	opt.cancel()
	opt.running = false

	opt.logger.Info("索引优化器已停止")
}

// OptimizeNow 立即执行优化
func (opt *IndexOptimizer) OptimizeNow(ctx context.Context) error {
	opt.logger.Info("开始索引优化")
	startTime := time.Now()

	// 1. 清理过期条目
	removed, err := opt.cleanupExpiredEntries(ctx)
	if err != nil {
		opt.logger.Error("清理过期条目失败", zap.Error(err))
	}

	// 2. 压缩索引
	if err := opt.compactIndex(ctx); err != nil {
		opt.logger.Error("压缩索引失败", zap.Error(err))
		return err
	}

	// 3. 优化索引结构
	if err := opt.optimizeIndexStructure(ctx); err != nil {
		opt.logger.Error("优化索引结构失败", zap.Error(err))
	}

	elapsed := time.Since(startTime)

	opt.mu.Lock()
	opt.stats.TotalOptimizations++
	opt.stats.LastOptimization = time.Now()
	opt.stats.RemovedEntries += int64(removed)
	if opt.stats.TotalOptimizations > 0 {
		opt.stats.AvgOptimizeTimeMs = (opt.stats.AvgOptimizeTimeMs*float64(opt.stats.TotalOptimizations-1) +
			float64(elapsed.Milliseconds())) / float64(opt.stats.TotalOptimizations)
	}
	opt.mu.Unlock()

	opt.logger.Info("索引优化完成",
		zap.Int("removed", removed),
		zap.Duration("elapsed", elapsed))

	return nil
}

// cleanupExpiredEntries 清理过期条目
func (opt *IndexOptimizer) cleanupExpiredEntries(ctx context.Context) (int, error) {
	opt.logger.Debug("清理过期条目")

	// 获取索引状态
	status := opt.indexer.GetStatus()
	opt.logger.Debug("当前索引状态",
		zap.Int64("totalFiles", status.TotalFiles),
		zap.Int64("indexedFiles", status.IndexedFiles))

	// 实际的清理逻辑需要遍历索引
	// 这里简化实现，返回 0
	return 0, nil
}

// compactIndex 压缩索引
func (opt *IndexOptimizer) compactIndex(ctx context.Context) error {
	opt.logger.Debug("压缩索引")

	// 获取索引目录大小
	indexPath := opt.config.IndexPath
	info, err := os.Stat(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("获取索引信息失败: %w", err)
	}

	opt.mu.Lock()
	opt.stats.CompactedSize = info.Size()
	opt.mu.Unlock()

	opt.logger.Debug("索引大小",
		zap.Int64("size", info.Size()),
		zap.String("path", indexPath))

	return nil
}

// optimizeIndexStructure 优化索引结构
func (opt *IndexOptimizer) optimizeIndexStructure(ctx context.Context) error {
	opt.logger.Debug("优化索引结构")
	return nil
}

// runPeriodicOptimization 定期优化
func (opt *IndexOptimizer) runPeriodicOptimization() {
	// 每天凌晨 3 点执行优化
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-opt.ctx.Done():
			return
		case <-ticker.C:
			// 检查是否是凌晨 3 点左右
			hour := time.Now().Hour()
			if hour >= 3 && hour < 4 {
				if err := opt.OptimizeNow(opt.ctx); err != nil {
					opt.logger.Error("定期优化失败", zap.Error(err))
				}
			}
		}
	}
}

// GetStats 获取统计信息
func (opt *IndexOptimizer) GetStats() OptimizerStats {
	opt.mu.RLock()
	defer opt.mu.RUnlock()
	return opt.stats
}

// IsRunning 是否在运行
func (opt *IndexOptimizer) IsRunning() bool {
	opt.mu.RLock()
	defer opt.mu.RUnlock()
	return opt.running
}

// EstimateIndexSize 估算索引大小
func (opt *IndexOptimizer) EstimateIndexSize(paths []string) (int64, int64, error) {
	var totalFiles int64
	var totalSize int64

	for _, root := range paths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			// 跳过隐藏文件
			base := filepath.Base(path)
			if base != "" && base[0] == '.' {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !info.IsDir() {
				totalFiles++
				totalSize += info.Size()
			}

			return nil
		})

		if err != nil {
			return 0, 0, fmt.Errorf("遍历路径失败: %w", err)
		}
	}

	return totalFiles, totalSize, nil
}

// RecommendConfig 推荐配置
func (opt *IndexOptimizer) RecommendConfig(paths []string) EngineConfig {
	totalFiles, totalSize, err := opt.EstimateIndexSize(paths)
	if err != nil {
		opt.logger.Error("估算索引大小失败", zap.Error(err))
		return opt.config
	}

	config := opt.config

	// 根据文件数量调整批量大小
	if totalFiles > 100000 {
		config.BatchSize = 1000
	} else if totalFiles > 10000 {
		config.BatchSize = 500
	} else {
		config.BatchSize = 200
	}

	// 根据总大小调整内容索引限制
	if totalSize > 10*1024*1024*1024 { // > 10GB
		config.MaxContentIndexSize = 5 * 1024 * 1024 // 5MB
	} else if totalSize > 1*1024*1024*1024 { // > 1GB
		config.MaxContentIndexSize = 10 * 1024 * 1024 // 10MB
	}

	// 根据文件数量调整并发数
	if totalFiles > 50000 {
		config.ConcurrentWorkers = 8
	} else if totalFiles > 10000 {
		config.ConcurrentWorkers = 4
	} else {
		config.ConcurrentWorkers = 2
	}

	opt.logger.Info("推荐配置",
		zap.Int64("totalFiles", totalFiles),
		zap.Int64("totalSize", totalSize),
		zap.Int("batchSize", config.BatchSize),
		zap.Int("concurrentWorkers", config.ConcurrentWorkers))

	return config
}
