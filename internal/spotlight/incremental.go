// Package spotlight 提供增量索引引擎
// 支持文件变更实时监控和增量更新，无需全量重建索引
package spotlight

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ChangeType 变更类型
type ChangeType int

const (
	ChangeCreate ChangeType = iota
	ChangeModify
	ChangeDelete
)

// FileChange 文件变更事件
type FileChange struct {
	Path      string
	OldPath   string // 重命名时的旧路径
	Type      ChangeType
	Timestamp time.Time
	Size      int64
}

// IncrementalIndexer 增量索引器
// 监控文件系统变更，实时更新索引
type IncrementalIndexer struct {
	indexer   *Indexer
	logger    *zap.Logger
	config    EngineConfig

	mu        sync.RWMutex
	running   bool
	ctx       context.Context
	cancel    context.CancelFunc

	// 变更队列
	changes   chan FileChange
	batchSize int

	// 文件监控状态
	watchedPaths map[string]time.Time // path -> lastModified
	deletedPaths map[string]time.Time // path -> deleteTime

	// 统计
	stats IncrementalStats
}

// IncrementalStats 增量索引统计
type IncrementalStats struct {
	TotalChanges   int64     `json:"totalChanges"`
	Created        int64     `json:"created"`
	Modified       int64     `json:"modified"`
	Deleted        int64     `json:"deleted"`
	LastUpdateTime time.Time `json:"lastUpdateTime"`
	BatchCount     int64     `json:"batchCount"`
	AvgBatchTime   float64   `json:"avgBatchTimeMs"`
}

// NewIncrementalIndexer 创建增量索引器
func NewIncrementalIndexer(indexer *Indexer, config EngineConfig, logger *zap.Logger) *IncrementalIndexer {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &IncrementalIndexer{
		indexer:      indexer,
		logger:       logger,
		config:       config,
		changes:      make(chan FileChange, 10000),
		batchSize:    config.BatchSize,
		watchedPaths: make(map[string]time.Time),
		deletedPaths: make(map[string]time.Time),
	}
}

// Start 启动增量索引器
func (inc *IncrementalIndexer) Start(ctx context.Context) error {
	inc.mu.Lock()
	defer inc.mu.Unlock()

	if inc.running {
		return fmt.Errorf("增量索引器已在运行")
	}

	inc.ctx, inc.cancel = context.WithCancel(ctx)
	inc.running = true

	// 启动变更处理协程
	go inc.processChanges()

	// 启动文件系统监控
	go inc.watchFileSystem()

	// 启动清理协程
	go inc.cleanupLoop()

	inc.logger.Info("增量索引器已启动",
		zap.Int("batchSize", inc.batchSize))

	return nil
}

// Stop 停止增量索引器
func (inc *IncrementalIndexer) Stop() {
	inc.mu.Lock()
	defer inc.mu.Unlock()

	if !inc.running {
		return
	}

	inc.cancel()
	inc.running = false

	// 处理剩余变更
	inc.flushPendingChanges()

	inc.logger.Info("增量索引器已停止")
}

// NotifyChange 通知文件变更
func (inc *IncrementalIndexer) NotifyChange(change FileChange) {
	inc.mu.RLock()
	running := inc.running
	inc.mu.RUnlock()

	if !running {
		return
	}

	// 非阻塞发送到队列
	select {
	case inc.changes <- change:
		inc.mu.Lock()
		inc.stats.TotalChanges++
		switch change.Type {
		case ChangeCreate:
			inc.stats.Created++
		case ChangeModify:
			inc.stats.Modified++
		case ChangeDelete:
			inc.stats.Deleted++
		}
		inc.mu.Unlock()
	default:
		inc.logger.Warn("变更队列已满，丢弃变更",
			zap.String("path", change.Path))
	}
}

// NotifyCreate 通知文件创建
func (inc *IncrementalIndexer) NotifyCreate(path string) {
	inc.NotifyChange(FileChange{
		Path:      path,
		Type:      ChangeCreate,
		Timestamp: time.Now(),
	})
}

// NotifyModify 通知文件修改
func (inc *IncrementalIndexer) NotifyModify(path string) {
	inc.NotifyChange(FileChange{
		Path:      path,
		Type:      ChangeModify,
		Timestamp: time.Now(),
	})
}

// NotifyDelete 通知文件删除
func (inc *IncrementalIndexer) NotifyDelete(path string) {
	inc.NotifyChange(FileChange{
		Path:      path,
		Type:      ChangeDelete,
		Timestamp: time.Now(),
	})
}

// NotifyRename 通知文件重命名
func (inc *IncrementalIndexer) NotifyRename(oldPath, newPath string) {
	// 删除旧路径
	inc.NotifyChange(FileChange{
		Path:      oldPath,
		Type:      ChangeDelete,
		Timestamp: time.Now(),
	})
	// 创建新路径
	inc.NotifyChange(FileChange{
		Path:      newPath,
		Type:      ChangeCreate,
		Timestamp: time.Now(),
	})
}

// processChanges 处理变更队列
func (inc *IncrementalIndexer) processChanges() {
	batch := make([]FileChange, 0, inc.batchSize)
	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-inc.ctx.Done():
			// 处理剩余变更
			if len(batch) > 0 {
				inc.processBatch(batch)
			}
			return

		case change := <-inc.changes:
			batch = append(batch, change)

			// 达到批量大小，立即处理
			if len(batch) >= inc.batchSize {
				inc.processBatch(batch)
				batch = batch[:0]
				timer.Reset(1 * time.Second)
			}

		case <-timer.C:
			// 超时，处理当前批次
			if len(batch) > 0 {
				inc.processBatch(batch)
				batch = batch[:0]
			}
			timer.Reset(1 * time.Second)
		}
	}
}

// processBatch 处理一批变更
func (inc *IncrementalIndexer) processBatch(changes []FileChange) {
	startTime := time.Now()

	inc.logger.Debug("处理变更批次",
		zap.Int("count", len(changes)))

	processed := 0
	for _, change := range changes {
		var err error
		switch change.Type {
		case ChangeCreate, ChangeModify:
			err = inc.processFileChange(change)
		case ChangeDelete:
			err = inc.processFileDelete(change)
		}

		if err != nil {
			inc.logger.Error("处理变更失败",
				zap.String("path", change.Path),
				zap.Int("type", int(change.Type)),
				zap.Error(err))
		} else {
			processed++
		}
	}

	elapsed := time.Since(startTime)

	inc.mu.Lock()
	inc.stats.BatchCount++
	inc.stats.LastUpdateTime = time.Now()
	if inc.stats.BatchCount > 0 {
		inc.stats.AvgBatchTime = (inc.stats.AvgBatchTime*float64(inc.stats.BatchCount-1) +
			float64(elapsed.Milliseconds())) / float64(inc.stats.BatchCount)
	}
	inc.mu.Unlock()

	inc.logger.Debug("变更批次处理完成",
		zap.Int("processed", processed),
		zap.Duration("elapsed", elapsed))
}

// processFileChange 处理文件创建/修改
func (inc *IncrementalIndexer) processFileChange(change FileChange) error {
	// 检查文件是否存在
	info, err := os.Stat(change.Path)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，从索引中移除
			return inc.indexer.RemoveFromIndex(inc.ctx, change.Path)
		}
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 检查是否需要重新索引
	inc.mu.RLock()
	lastMod, exists := inc.watchedPaths[change.Path]
	inc.mu.RUnlock()

	if exists && !info.ModTime().After(lastMod) {
		// 文件未修改，跳过
		return nil
	}

	// 索引文件
	if err := inc.indexer.IndexFile(inc.ctx, change.Path); err != nil {
		return fmt.Errorf("索引文件失败: %w", err)
	}

	// 更新监控状态
	inc.mu.Lock()
	inc.watchedPaths[change.Path] = info.ModTime()
	delete(inc.deletedPaths, change.Path)
	inc.mu.Unlock()

	return nil
}

// processFileDelete 处理文件删除
func (inc *IncrementalIndexer) processFileDelete(change FileChange) error {
	if err := inc.indexer.RemoveFromIndex(inc.ctx, change.Path); err != nil {
		// 文件可能已经不在索引中
		inc.logger.Debug("从索引移除文件失败",
			zap.String("path", change.Path),
			zap.Error(err))
	}

	inc.mu.Lock()
	delete(inc.watchedPaths, change.Path)
	inc.deletedPaths[change.Path] = time.Now()
	inc.mu.Unlock()

	return nil
}

// watchFileSystem 监控文件系统
func (inc *IncrementalIndexer) watchFileSystem() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-inc.ctx.Done():
			return
		case <-ticker.C:
			inc.scanForChanges()
		}
	}
}

// scanForChanges 扫描文件系统变更
func (inc *IncrementalIndexer) scanForChanges() {
	inc.mu.RLock()
	paths := inc.config.IndexPaths
	inc.mu.RUnlock()

	for _, root := range paths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			// 检查上下文
			select {
			case <-inc.ctx.Done():
				return inc.ctx.Err()
			default:
			}

			// 跳过隐藏文件和目录
			if inc.shouldSkip(path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// 检查文件是否已修改
			inc.mu.RLock()
			lastMod, exists := inc.watchedPaths[path]
			inc.mu.RUnlock()

			if !exists {
				// 新文件
				inc.NotifyCreate(path)
			} else if info.ModTime().After(lastMod) {
				// 文件已修改
				inc.NotifyModify(path)
			}

			return nil
		})

		if err != nil && err != context.Canceled {
			inc.logger.Error("扫描文件系统失败",
				zap.String("root", root),
				zap.Error(err))
		}
	}
}

// shouldSkip 是否应该跳过
func (inc *IncrementalIndexer) shouldSkip(path string) bool {
	// 检查隐藏文件
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") && base != "." {
		return true
	}

	// 检查排除路径
	for _, exclude := range inc.config.ExcludePaths {
		if strings.HasPrefix(path, exclude) {
			return true
		}
	}

	return false
}

// cleanupLoop 清理过期的删除记录
func (inc *IncrementalIndexer) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-inc.ctx.Done():
			return
		case <-ticker.C:
			inc.cleanupDeletedPaths()
		}
	}
}

// cleanupDeletedPaths 清理过期的删除记录
func (inc *IncrementalIndexer) cleanupDeletedPaths() {
	inc.mu.Lock()
	defer inc.mu.Unlock()

	// 保留最近 24 小时的记录
	cutoff := time.Now().Add(-24 * time.Hour)
	for path, deleteTime := range inc.deletedPaths {
		if deleteTime.Before(cutoff) {
			delete(inc.deletedPaths, path)
		}
	}
}

// flushPendingChanges 处理所有待处理的变更
func (inc *IncrementalIndexer) flushPendingChanges() {
	batch := make([]FileChange, 0)
	for {
		select {
		case change := <-inc.changes:
			batch = append(batch, change)
		default:
			if len(batch) > 0 {
				inc.processBatch(batch)
			}
			return
		}
	}
}

// GetStats 获取统计信息
func (inc *IncrementalIndexer) GetStats() IncrementalStats {
	inc.mu.RLock()
	defer inc.mu.RUnlock()
	return inc.stats
}

// GetWatchedCount 获取监控文件数
func (inc *IncrementalIndexer) GetWatchedCount() int {
	inc.mu.RLock()
	defer inc.mu.RUnlock()
	return len(inc.watchedPaths)
}

// IsRunning 是否在运行
func (inc *IncrementalIndexer) IsRunning() bool {
	inc.mu.RLock()
	defer inc.mu.RUnlock()
	return inc.running
}

// ScanNow 立即扫描文件系统变更
func (inc *IncrementalIndexer) ScanNow() {
	inc.scanForChanges()
}
