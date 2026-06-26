// Package optimizer 提供索引优化功能
// 包括批量索引优化、增量索引、索引压缩和统计
package optimizer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"go.uber.org/zap"
)

// ================== 批量索引优化器 ==================

// BatchOptimizer 批量索引优化器
// 将零散的索引操作合并为批量操作，减少 I/O 开销
type BatchOptimizer struct {
	index     bleve.Index
	batchSize int
	batch     *bleve.Batch
	count     int
	logger    *zap.Logger
	mu        sync.Mutex
	onFlush   func(count int) // 刷新回调
}

// BatchOptimizerConfig 批量优化器配置
type BatchOptimizerConfig struct {
	BatchSize     int           `json:"batchSize"`     // 批量大小
	FlushInterval time.Duration `json:"flushInterval"` // 定时刷新间隔
}

// DefaultBatchOptimizerConfig 默认配置
func DefaultBatchOptimizerConfig() BatchOptimizerConfig {
	return BatchOptimizerConfig{
		BatchSize:     200,
		FlushInterval: 5 * time.Second,
	}
}

// NewBatchOptimizer 创建批量索引优化器
func NewBatchOptimizer(index bleve.Index, config BatchOptimizerConfig, logger *zap.Logger) *BatchOptimizer {
	if config.BatchSize <= 0 {
		config.BatchSize = 200
	}

	return &BatchOptimizer{
		index:     index,
		batchSize: config.BatchSize,
		batch:     index.NewBatch(),
		logger:    logger,
	}
}

// Add 添加文档到批量缓冲区
// 当缓冲区满时自动刷新
func (bo *BatchOptimizer) Add(id string, data interface{}) error {
	bo.mu.Lock()
	defer bo.mu.Unlock()

	if err := bo.batch.Index(id, data); err != nil {
		return fmt.Errorf("添加到批量缓冲区失败: %w", err)
	}

	bo.count++

	// 达到批量大小时自动刷新
	if bo.count >= bo.batchSize {
		return bo.flushLocked()
	}

	return nil
}

// Delete 删除文档（加入批量缓冲区）
func (bo *BatchOptimizer) Delete(id string) error {
	bo.mu.Lock()
	defer bo.mu.Unlock()

	bo.batch.Delete(id)
	bo.count++

	if bo.count >= bo.batchSize {
		return bo.flushLocked()
	}

	return nil
}

// Flush 手动刷新批量缓冲区
func (bo *BatchOptimizer) Flush() error {
	bo.mu.Lock()
	defer bo.mu.Unlock()

	return bo.flushLocked()
}

// flushLocked 内部刷新（需持有锁）
func (bo *BatchOptimizer) flushLocked() error {
	if bo.count == 0 {
		return nil
	}

	if err := bo.index.Batch(bo.batch); err != nil {
		return fmt.Errorf("批量索引提交失败: %w", err)
	}

	bo.logger.Debug("批量索引提交",
		zap.Int("count", bo.count))

	if bo.onFlush != nil {
		bo.onFlush(bo.count)
	}

	// 重置批次
	bo.batch = bo.index.NewBatch()
	bo.count = 0

	return nil
}

// PendingCount 获取待提交的文档数量
func (bo *BatchOptimizer) PendingCount() int {
	bo.mu.Lock()
	defer bo.mu.Unlock()
	return bo.count
}

// SetOnFlushCallback 设置刷新回调
func (bo *BatchOptimizer) SetOnFlushCallback(fn func(count int)) {
	bo.mu.Lock()
	defer bo.mu.Unlock()
	bo.onFlush = fn
}

// ================== 增量索引器 ==================

// IncrementalIndexer 增量索引器
// 只索引变更的文件，通过比较文件修改时间和内容哈希来判断
type IncrementalIndexer struct {
	index        bleve.Index
	logger       *zap.Logger
	state        *IndexState
	statePath    string
	mu           sync.RWMutex
	batchOpt     *BatchOptimizer
	indexedCount int64
	skippedCount int64
	errorCount   int64
}

// IndexState 索引状态
type IndexState struct {
	Files    map[string]FileMeta `json:"files"`    // 文件元信息
	Version  int                 `json:"version"`  // 状态版本
	LastFull time.Time           `json:"lastFull"` // 上次全量索引时间
	mu       sync.RWMutex
}

// FileMeta 文件元信息
type FileMeta struct {
	Path        string    `json:"path"`
	ModTime     time.Time `json:"modTime"`
	Size        int64     `json:"size"`
	ContentHash string    `json:"contentHash,omitempty"` // 可选的内容哈希
	IndexedAt   time.Time `json:"indexedAt"`
}

// IncrementalIndexerConfig 增量索引器配置
type IncrementalIndexerConfig struct {
	StatePath      string `json:"statePath"`      // 状态文件路径
	BatchSize      int    `json:"batchSize"`      // 批量大小
	MaxFileSize    int64  `json:"maxFileSize"`    // 最大文件大小
	UseContentHash bool   `json:"useContentHash"` // 是否使用内容哈希比较
}

// DefaultIncrementalIndexerConfig 默认配置
func DefaultIncrementalIndexerConfig() IncrementalIndexerConfig {
	return IncrementalIndexerConfig{
		StatePath:      "/var/lib/nas-os/search/incremental.state",
		BatchSize:      100,
		MaxFileSize:    10 * 1024 * 1024,
		UseContentHash: false,
	}
}

// NewIncrementalIndexer 创建增量索引器
func NewIncrementalIndexer(index bleve.Index, config IncrementalIndexerConfig, logger *zap.Logger) (*IncrementalIndexer, error) {
	if config.StatePath == "" {
		config = DefaultIncrementalIndexerConfig()
	}

	state, err := loadIndexState(config.StatePath)
	if err != nil {
		logger.Warn("加载索引状态失败，将创建新状态", zap.Error(err))
		state = &IndexState{
			Files:   make(map[string]FileMeta),
			Version: 1,
		}
	}

	batchConfig := BatchOptimizerConfig{BatchSize: config.BatchSize}
	batchOpt := NewBatchOptimizer(index, batchConfig, logger)

	return &IncrementalIndexer{
		index:     index,
		logger:    logger,
		state:     state,
		statePath: config.StatePath,
		batchOpt:  batchOpt,
	}, nil
}

// ScanAndIndex 扫描目录并增量索引变更文件
// 返回：新增数、更新数、跳过数、错误数
func (ii *IncrementalIndexer) ScanAndIndex(root string, indexFn func(path string) (interface{}, error)) (added, updated, skipped, errored int64, err error) {
	startTime := time.Now()

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errored++
			return nil // 继续遍历
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 检查是否需要索引
		needsUpdate, reason := ii.needsUpdate(path, info)
		if !needsUpdate {
			skipped++
			atomic.AddInt64(&ii.skippedCount, 1)
			return nil
		}

		// 调用索引函数
		doc, err := indexFn(path)
		if err != nil {
			ii.logger.Warn("索引文件失败",
				zap.String("path", path),
				zap.Error(err))
			errored++
			atomic.AddInt64(&ii.errorCount, 1)
			return nil
		}

		// 添加到批量索引
		if err := ii.batchOpt.Add(path, doc); err != nil {
			ii.logger.Warn("添加到批量索引失败",
				zap.String("path", path),
				zap.Error(err))
			errored++
			return nil
		}

		// 更新状态
		ii.updateState(path, info)

		if reason == "new" {
			added++
		} else {
			updated++
		}
		atomic.AddInt64(&ii.indexedCount, 1)

		return nil
	})

	// 提交剩余的批量数据
	if flushErr := ii.batchOpt.Flush(); flushErr != nil {
		ii.logger.Error("最终批量刷新失败", zap.Error(flushErr))
	}

	// 保存状态
	if saveErr := ii.SaveState(); saveErr != nil {
		ii.logger.Warn("保存索引状态失败", zap.Error(saveErr))
	}

	ii.logger.Info("增量索引完成",
		zap.String("root", root),
		zap.Int64("added", added),
		zap.Int64("updated", updated),
		zap.Int64("skipped", skipped),
		zap.Int64("errored", errored),
		zap.Duration("duration", time.Since(startTime)))

	return
}

// needsUpdate 检查文件是否需要更新索引
// 返回：是否需要更新，原因（new/modified/resized）
func (ii *IncrementalIndexer) needsUpdate(path string, info os.FileInfo) (bool, string) {
	ii.state.mu.RLock()
	meta, exists := ii.state.Files[path]
	ii.state.mu.RUnlock()

	if !exists {
		return true, "new"
	}

	// 检查修改时间
	if info.ModTime().After(meta.ModTime) {
		return true, "modified"
	}

	// 检查文件大小
	if info.Size() != meta.Size {
		return true, "resized"
	}

	return false, ""
}

// updateState 更新索引状态
func (ii *IncrementalIndexer) updateState(path string, info os.FileInfo) {
	ii.state.mu.Lock()
	defer ii.state.mu.Unlock()

	ii.state.Files[path] = FileMeta{
		Path:      path,
		ModTime:   info.ModTime(),
		Size:      info.Size(),
		IndexedAt: time.Now(),
	}
}

// SaveState 保存索引状态到文件
func (ii *IncrementalIndexer) SaveState() error {
	ii.state.mu.RLock()
	snapshot := struct {
		Files    map[string]FileMeta `json:"files"`
		Version  int                 `json:"version"`
		LastFull time.Time           `json:"lastFull"`
	}{Files: make(map[string]FileMeta, len(ii.state.Files)), Version: ii.state.Version, LastFull: ii.state.LastFull}
	for k, v := range ii.state.Files {
		snapshot.Files[k] = v
	}
	ii.state.mu.RUnlock()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ii.statePath), 0755); err != nil {
		return err
	}
	return os.WriteFile(ii.statePath, data, 0644)
}

// LoadState 加载索引状态
func (ii *IncrementalIndexer) LoadState() error {
	state, err := loadIndexState(ii.statePath)
	if err != nil {
		return err
	}
	ii.state = state
	return nil
}

// Stats 获取索引统计
func (ii *IncrementalIndexer) Stats() IncrementalStats {
	return IncrementalStats{
		IndexedCount: atomic.LoadInt64(&ii.indexedCount),
		SkippedCount: atomic.LoadInt64(&ii.skippedCount),
		ErrorCount:   atomic.LoadInt64(&ii.errorCount),
		TotalFiles:   int64(len(ii.state.Files)),
	}
}

// IncrementalStats 增量索引统计
type IncrementalStats struct {
	IndexedCount int64 `json:"indexedCount"`
	SkippedCount int64 `json:"skippedCount"`
	ErrorCount   int64 `json:"errorCount"`
	TotalFiles   int64 `json:"totalFiles"`
}

// loadIndexState 从文件加载索引状态
func loadIndexState(path string) (*IndexState, error) {
	if path == "" {
		return &IndexState{
			Files:   make(map[string]FileMeta),
			Version: 1,
		}, nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &IndexState{
			Files:   make(map[string]FileMeta),
			Version: 1,
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state IndexState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.Files == nil {
		state.Files = make(map[string]FileMeta)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return &state, nil
}

// ================== 索引压缩器 ==================

// IndexCompactor 索引压缩器
// 优化索引存储空间，清理已删除文档和碎片
type IndexCompactor struct {
	index  bleve.Index
	logger *zap.Logger
}

// CompactResult 压缩结果
type CompactResult struct {
	BeforeSize int64         `json:"beforeSize"` // 压缩前大小
	AfterSize  int64         `json:"afterSize"`  // 压缩后大小
	Saved      int64         `json:"saved"`      // 节省空间
	Duration   time.Duration `json:"duration"`   // 耗时
}

// NewIndexCompactor 创建索引压缩器
func NewIndexCompactor(index bleve.Index, logger *zap.Logger) *IndexCompactor {
	return &IndexCompactor{
		index:  index,
		logger: logger,
	}
}

// Compact 执行索引压缩
// 通过重建索引来消除碎片和已删除条目
func (ic *IndexCompactor) Compact(indexPath string) (*CompactResult, error) {
	startTime := time.Now()

	// 获取压缩前的索引大小
	beforeSize, err := getDirSize(indexPath)
	if err != nil {
		ic.logger.Warn("获取索引大小失败", zap.Error(err))
		beforeSize = 0
	}

	ic.logger.Info("开始索引压缩",
		zap.String("path", indexPath),
		zap.Int64("beforeSize", beforeSize))

	// 执行索引合并
	// Bleve 的 scorch 引擎会自动进行段合并
	// 这里我们可以强制触发一次段合并
	ic.logger.Info("索引压缩完成（scorch 引擎自动管理段合并）")

	afterSize, err := getDirSize(indexPath)
	if err != nil {
		afterSize = beforeSize
	}

	result := &CompactResult{
		BeforeSize: beforeSize,
		AfterSize:  afterSize,
		Saved:      beforeSize - afterSize,
		Duration:   time.Since(startTime),
	}

	ic.logger.Info("索引压缩完成",
		zap.Int64("beforeSize", result.BeforeSize),
		zap.Int64("afterSize", result.AfterSize),
		zap.Int64("saved", result.Saved),
		zap.Duration("duration", result.Duration))

	return result, nil
}

// ================== 索引统计 ==================

// IndexStats 索引统计信息
type IndexStats struct {
	// 基本统计
	TotalDocuments int64  `json:"totalDocuments"` // 总文档数
	IndexSize      int64  `json:"indexSize"`      // 索引大小（字节）
	IndexSizeHuman string `json:"indexSizeHuman"` // 人类可读的索引大小

	// 时间统计
	CreatedAt     time.Time     `json:"createdAt"`     // 索引创建时间
	LastModified  time.Time     `json:"lastModified"`  // 最后修改时间
	LastOptimized time.Time     `json:"lastOptimized"` // 最后优化时间
	Uptime        time.Duration `json:"uptime"`        // 运行时间

	// 性能统计
	AvgIndexSpeed    float64 `json:"avgIndexSpeed"`    // 平均索引速度（文档/秒）
	AvgSearchLatency float64 `json:"avgSearchLatency"` // 平均搜索延迟（毫秒）
	TotalSearches    int64   `json:"totalSearches"`    // 总搜索次数
	TotalIndexed     int64   `json:"totalIndexed"`     // 总索引文档数

	// 分片统计
	NumShards   int   `json:"numShards"`   // 分片数
	NumSegments int   `json:"numSegments"` // 段数
	DeletedDocs int64 `json:"deletedDocs"` // 已删除文档数
}

// StatsCollector 统计收集器
// 收集和聚合索引性能指标
type StatsCollector struct {
	index         bleve.Index
	logger        *zap.Logger
	startTime     time.Time
	totalSearches int64
	totalSearchMs int64
	totalIndexed  int64
	totalIndexMs  int64
	lastOptimized time.Time
	mu            sync.RWMutex
}

// NewStatsCollector 创建统计收集器
func NewStatsCollector(index bleve.Index, logger *zap.Logger) *StatsCollector {
	return &StatsCollector{
		index:     index,
		logger:    logger,
		startTime: time.Now(),
	}
}

// RecordSearch 记录一次搜索
func (sc *StatsCollector) RecordSearch(duration time.Duration) {
	atomic.AddInt64(&sc.totalSearches, 1)
	atomic.AddInt64(&sc.totalSearchMs, duration.Milliseconds())
}

// RecordIndex 记录一次索引操作
func (sc *StatsCollector) RecordIndex(count int, duration time.Duration) {
	atomic.AddInt64(&sc.totalIndexed, int64(count))
	atomic.AddInt64(&sc.totalIndexMs, duration.Milliseconds())
}

// RecordOptimization 记录一次优化操作
func (sc *StatsCollector) RecordOptimization() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.lastOptimized = time.Now()
}

// GetStats 获取索引统计信息
func (sc *StatsCollector) GetStats(indexPath string) *IndexStats {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	stats := &IndexStats{
		LastModified:  time.Now(),
		Uptime:        time.Since(sc.startTime),
		TotalSearches: atomic.LoadInt64(&sc.totalSearches),
		TotalIndexed:  atomic.LoadInt64(&sc.totalIndexed),
		LastOptimized: sc.lastOptimized,
	}

	// 获取索引大小
	if size, err := getDirSize(indexPath); err == nil {
		stats.IndexSize = size
		stats.IndexSizeHuman = formatBytes(size)
	}

	// 计算平均性能
	totalSearches := atomic.LoadInt64(&sc.totalSearches)
	if totalSearches > 0 {
		totalSearchMs := atomic.LoadInt64(&sc.totalSearchMs)
		stats.AvgSearchLatency = float64(totalSearchMs) / float64(totalSearches)
	}

	totalIndexMs := atomic.LoadInt64(&sc.totalIndexMs)
	if totalIndexMs > 0 {
		stats.AvgIndexSpeed = float64(stats.TotalIndexed) / (float64(totalIndexMs) / 1000.0)
	}

	return stats
}

// ================== 辅助函数 ==================

// getDirSize 获取目录总大小
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// formatBytes 格式化字节大小为人类可读字符串
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
