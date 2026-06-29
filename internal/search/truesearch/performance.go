// Package truesearch 实现全文搜索引擎 (TrueSearch Phase 2)
// 本文件实现性能优化：SSD 索引缓存、查询结果缓存、批量索引与增量更新。
package truesearch

import (
	"container/list"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"go.uber.org/zap"
)

// ─── SSD 索引缓存 ─────────────────────────────────────────────

// IndexCache 是基于内存的热索引缓存。
// 在 SSD 上保留最近/最频繁访问的索引段，减少磁盘 I/O。
type IndexCache struct {
	mu       sync.RWMutex
	maxItems int
	items    map[string]*list.Element
	lru      *list.List
	stats    CacheStats
}

// cacheEntry 缓存条目。
type cacheEntry struct {
	key       string
	value     []byte
	size      int
	hits      int64
	lastAccess time.Time
}

// CacheStats 缓存统计。
type CacheStats struct {
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Evictions  int64 `json:"evictions"`
	SizeBytes  int64 `json:"size_bytes"`
	ItemsCount int   `json:"items_count"`
}

// NewIndexCache 创建索引缓存。
// maxItems 为最大缓存条目数。
func NewIndexCache(maxItems int) *IndexCache {
	if maxItems <= 0 {
		maxItems = 1000
	}
	return &IndexCache{
		maxItems: maxItems,
		items:    make(map[string]*list.Element),
		lru:      list.New(),
	}
}

// Get 从缓存中获取条目。
func (c *IndexCache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*cacheEntry)
		entry.hits++
		entry.lastAccess = time.Now()
		c.lru.MoveToFront(elem)
		atomic.AddInt64(&c.stats.Hits, 1)
		return entry.value, true
	}
	atomic.AddInt64(&c.stats.Misses, 1)
	return nil, false
}

// Put 将条目放入缓存。
func (c *IndexCache) Put(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，更新值
	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*cacheEntry)
		c.stats.SizeBytes -= int64(entry.size)
		entry.value = value
		entry.size = len(value)
		c.stats.SizeBytes += int64(entry.size)
		c.lru.MoveToFront(elem)
		return
	}

	// 新条目
	entry := &cacheEntry{
		key:        key,
		value:      value,
		size:       len(value),
		lastAccess: time.Now(),
	}
	elem := c.lru.PushFront(entry)
	c.items[key] = elem
	c.stats.SizeBytes += int64(entry.size)
	c.stats.ItemsCount = c.lru.Len()

	// 驱逐最旧条目
	for c.lru.Len() > c.maxItems {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		oldEntry := oldest.Value.(*cacheEntry)
		c.stats.SizeBytes -= int64(oldEntry.size)
		delete(c.items, oldEntry.key)
		c.lru.Remove(oldest)
		atomic.AddInt64(&c.stats.Evictions, 1)
	}
	c.stats.ItemsCount = c.lru.Len()
}

// Remove 从缓存中移除条目。
func (c *IndexCache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*cacheEntry)
		c.stats.SizeBytes -= int64(entry.size)
		delete(c.items, key)
		c.lru.Remove(elem)
		c.stats.ItemsCount = c.lru.Len()
	}
}

// Clear 清空缓存。
func (c *IndexCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.lru = list.New()
	c.stats = CacheStats{}
}

// Stats 返回缓存统计信息。
func (c *IndexCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheStats{
		Hits:       atomic.LoadInt64(&c.stats.Hits),
		Misses:     atomic.LoadInt64(&c.stats.Misses),
		Evictions:  atomic.LoadInt64(&c.stats.Evictions),
		SizeBytes:  c.stats.SizeBytes,
		ItemsCount: c.lru.Len(),
	}
}

// ─── 查询结果缓存 ─────────────────────────────────────────────

// QueryCache 缓存查询结果以实现亚秒级响应。
// 使用 LRU + TTL 策略，相同查询在 TTL 内直接返回缓存结果。
type QueryCache struct {
	mu       sync.RWMutex
	maxItems int
	ttl      time.Duration
	items    map[string]*list.Element
	lru      *list.List
	stats    QueryCacheStats
}

// queryCacheEntry 查询缓存条目。
type queryCacheEntry struct {
	key       string
	response  *SearchResponse
	expiresAt time.Time
	hits      int64
}

// QueryCacheStats 查询缓存统计。
type QueryCacheStats struct {
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Evictions  int64 `json:"evictions"`
	ItemsCount int   `json:"items_count"`
}

// NewQueryCache 创建查询结果缓存。
// maxItems 为最大缓存条目数，ttl 为条目生存时间。
func NewQueryCache(maxItems int, ttl time.Duration) *QueryCache {
	if maxItems <= 0 {
		maxItems = 500
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &QueryCache{
		maxItems: maxItems,
		ttl:      ttl,
		items:    make(map[string]*list.Element),
		lru:      list.New(),
	}
}

// cacheKey 生成查询缓存键。
func (qc *QueryCache) cacheKey(req SearchRequest) string {
	return fmt.Sprintf("%s|%s|%v|%d|%v", req.Query, req.Path, req.Types, req.MaxResults, req.Highlight)
}

// Get 从缓存中获取查询结果。
func (qc *QueryCache) Get(req SearchRequest) (*SearchResponse, bool) {
	key := qc.cacheKey(req)
	qc.mu.Lock()
	defer qc.mu.Unlock()

	if elem, ok := qc.items[key]; ok {
		entry := elem.Value.(*queryCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			entry.hits++
			qc.lru.MoveToFront(elem)
			atomic.AddInt64(&qc.stats.Hits, 1)
			// 返回副本，避免调用者修改缓存数据
			return cloneSearchResponse(entry.response), true
		}
		// 已过期，删除
		delete(qc.items, key)
		qc.lru.Remove(elem)
		qc.stats.ItemsCount = qc.lru.Len()
	}
	atomic.AddInt64(&qc.stats.Misses, 1)
	return nil, false
}

// Put 将查询结果放入缓存。
func (qc *QueryCache) Put(req SearchRequest, resp *SearchResponse) {
	key := qc.cacheKey(req)
	qc.mu.Lock()
	defer qc.mu.Unlock()

	// 如果已存在，更新
	if elem, ok := qc.items[key]; ok {
		entry := elem.Value.(*queryCacheEntry)
		entry.response = cloneSearchResponse(resp)
		entry.expiresAt = time.Now().Add(qc.ttl)
		qc.lru.MoveToFront(elem)
		return
	}

	entry := &queryCacheEntry{
		key:       key,
		response:  cloneSearchResponse(resp),
		expiresAt: time.Now().Add(qc.ttl),
	}
	elem := qc.lru.PushFront(entry)
	qc.items[key] = elem
	qc.stats.ItemsCount = qc.lru.Len()

	// 驱逐最旧条目
	for qc.lru.Len() > qc.maxItems {
		oldest := qc.lru.Back()
		if oldest == nil {
			break
		}
		oldEntry := oldest.Value.(*queryCacheEntry)
		delete(qc.items, oldEntry.key)
		qc.lru.Remove(oldest)
		atomic.AddInt64(&qc.stats.Evictions, 1)
	}
	qc.stats.ItemsCount = qc.lru.Len()
}

// Invalidate 使所有缓存条目失效。
func (qc *QueryCache) Invalidate() {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.items = make(map[string]*list.Element)
	qc.lru = list.New()
	qc.stats = QueryCacheStats{}
}

// Stats 返回查询缓存统计信息。
func (qc *QueryCache) Stats() QueryCacheStats {
	qc.mu.RLock()
	defer qc.mu.RUnlock()
	return QueryCacheStats{
		Hits:       atomic.LoadInt64(&qc.stats.Hits),
		Misses:     atomic.LoadInt64(&qc.stats.Misses),
		Evictions:  atomic.LoadInt64(&qc.stats.Evictions),
		ItemsCount: qc.lru.Len(),
	}
}

// cloneSearchResponse 深拷贝 SearchResponse。
func cloneSearchResponse(resp *SearchResponse) *SearchResponse {
	if resp == nil {
		return nil
	}
	clone := &SearchResponse{
		Total:  resp.Total,
		TookMs: resp.TookMs,
		Results: make([]SearchResult, len(resp.Results)),
	}
	copy(clone.Results, resp.Results)
	return clone
}

// ─── 性能优化管理器 ───────────────────────────────────────────

// PerformanceManager 管理索引缓存、查询缓存和批量索引操作。
// 集成到 Indexer 中以提供亚秒级查询响应和大规模文件索引支持。
type PerformanceManager struct {
	indexCache  *IndexCache
	queryCache  *QueryCache
	logger      *zap.Logger
	enableCache bool

	// 增量更新追踪
	incrementalMu sync.RWMutex
	fileHashes    map[string]uint64
}

// PerformanceConfig 性能配置。
type PerformanceConfig struct {
	EnableIndexCache bool   `json:"enableIndexCache"`
	IndexCacheSize   int    `json:"indexCacheSize"`   // 索引缓存最大条目数
	EnableQueryCache bool   `json:"enableQueryCache"`
	QueryCacheSize   int    `json:"queryCacheSize"`   // 查询缓存最大条目数
	QueryCacheTTL    string `json:"queryCacheTTL"`    // 查询缓存 TTL（如 "5m"）
	BatchSize        int    `json:"batchSize"`        // 批量索引大小
}

// DefaultPerformanceConfig 返回默认性能配置。
func DefaultPerformanceConfig() PerformanceConfig {
	return PerformanceConfig{
		EnableIndexCache: true,
		IndexCacheSize:   1000,
		EnableQueryCache: true,
		QueryCacheSize:   500,
		QueryCacheTTL:    "5m",
		BatchSize:        100,
	}
}

// NewPerformanceManager 创建性能管理器。
func NewPerformanceManager(cfg PerformanceConfig, logger *zap.Logger) *PerformanceManager {
	pm := &PerformanceManager{
		logger:      logger,
		enableCache: cfg.EnableQueryCache,
		fileHashes:  make(map[string]uint64),
	}

	if cfg.EnableIndexCache {
		pm.indexCache = NewIndexCache(cfg.IndexCacheSize)
	}
	if cfg.EnableQueryCache {
		ttl, err := time.ParseDuration(cfg.QueryCacheTTL)
		if err != nil {
			ttl = 5 * time.Minute
		}
		pm.queryCache = NewQueryCache(cfg.QueryCacheSize, ttl)
	}

	return pm
}

// IndexCacheStats 返回索引缓存统计。
func (pm *PerformanceManager) IndexCacheStats() *CacheStats {
	if pm.indexCache == nil {
		return nil
	}
	s := pm.indexCache.Stats()
	return &s
}

// QueryCacheStats 返回查询缓存统计。
func (pm *PerformanceManager) QueryCacheStats() *QueryCacheStats {
	if pm.queryCache == nil {
		return nil
	}
	s := pm.queryCache.Stats()
	return &s
}

// InvalidateQueryCache 使查询缓存失效（索引更新后调用）。
func (pm *PerformanceManager) InvalidateQueryCache() {
	if pm.queryCache != nil {
		pm.queryCache.Invalidate()
	}
}

// GetCachedQuery 从缓存获取查询结果。
func (pm *PerformanceManager) GetCachedQuery(req SearchRequest) (*SearchResponse, bool) {
	if pm.queryCache == nil {
		return nil, false
	}
	return pm.queryCache.Get(req)
}

// PutCachedQuery 将查询结果存入缓存。
func (pm *PerformanceManager) PutCachedQuery(req SearchRequest, resp *SearchResponse) {
	if pm.queryCache == nil {
		return
	}
	pm.queryCache.Put(req, resp)
}

// SetFileHash 记录文件哈希用于增量更新判断。
func (pm *PerformanceManager) SetFileHash(path string, hash uint64) {
	pm.incrementalMu.Lock()
	defer pm.incrementalMu.Unlock()
	pm.fileHashes[path] = hash
}

// GetFileHash 获取文件哈希。
func (pm *PerformanceManager) GetFileHash(path string) (uint64, bool) {
	pm.incrementalMu.RLock()
	defer pm.incrementalMu.RUnlock()
	hash, ok := pm.fileHashes[path]
	return hash, ok
}

// RemoveFileHash 移除文件哈希记录。
func (pm *PerformanceManager) RemoveFileHash(path string) {
	pm.incrementalMu.Lock()
	defer pm.incrementalMu.Unlock()
	delete(pm.fileHashes, path)
}

// HasFileChanged 判断文件是否发生变化。
// 返回 true 表示文件是新的或已修改，需要重新索引。
func (pm *PerformanceManager) HasFileChanged(path string, hash uint64) bool {
	oldHash, exists := pm.GetFileHash(path)
	if !exists {
		return true
	}
	return oldHash != hash
}

// IncrementalStats 增量索引统计。
type IncrementalStats struct {
	TrackedFiles int `json:"trackedFiles"`
	NewFiles     int `json:"newFiles"`
	UpdatedFiles int `json:"updatedFiles"`
	DeletedFiles int `json:"deletedFiles"`
}

// IncrementalStats 返回增量索引统计。
func (pm *PerformanceManager) IncrementalStats() IncrementalStats {
	pm.incrementalMu.RLock()
	defer pm.incrementalMu.RUnlock()
	return IncrementalStats{
		TrackedFiles: len(pm.fileHashes),
	}
}

// Close 关闭性能管理器，释放资源。
func (pm *PerformanceManager) Close() {
	if pm.indexCache != nil {
		pm.indexCache.Clear()
	}
	if pm.queryCache != nil {
		pm.queryCache.Invalidate()
	}
}

// ─── 批量索引器 ───────────────────────────────────────────────

// BatchIndexer 批量索引器，支持大规模文件索引和增量更新。
type BatchIndexer struct {
	indexer     *Indexer
	extractor   *Extractor
	perfManager *PerformanceManager
	logger      *zap.Logger
	batchSize   int
}

// NewBatchIndexer 创建批量索引器。
func NewBatchIndexer(indexer *Indexer, extractor *Extractor, pm *PerformanceManager, logger *zap.Logger) *BatchIndexer {
	batchSize := indexer.config.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	return &BatchIndexer{
		indexer:     indexer,
		extractor:   extractor,
		perfManager: pm,
		logger:      logger,
		batchSize:   batchSize,
	}
}

// BatchIndexResult 批量索引结果。
type BatchIndexResult struct {
	Total        int           `json:"total"`
	Indexed      int           `json:"indexed"`
	Skipped      int           `json:"skipped"`
	Failed       int           `json:"failed"`
	DeletedFiles int           `json:"deleted_files,omitempty"`
	Errors       []string      `json:"errors,omitempty"`
	Duration     time.Duration `json:"duration"`
}

// BatchIndexFiles 批量索引文件列表。
// 使用增量更新策略，跳过未修改的文件。
func (bi *BatchIndexer) BatchIndexFiles(paths []string) *BatchIndexResult {
	start := time.Now()
	result := &BatchIndexResult{Total: len(paths)}

	batch := bi.indexer.index.NewBatch()
	batchCount := 0

	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: abs path: %v", path, err))
			continue
		}

		info, err := os.Stat(absPath)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: stat: %v", path, err))
			continue
		}
		if info.IsDir() {
			continue
		}
		if !bi.indexer.shouldIndex(absPath) {
			result.Skipped++
			continue
		}

		// 增量更新检查
		fileHash := computeFileHash(absPath, info)
		if bi.perfManager != nil && !bi.perfManager.HasFileChanged(absPath, fileHash) {
			result.Skipped++
			continue
		}

		content, _ := bi.extractor.Extract(absPath)

		doc := indexedDoc{
			Path:    absPath,
			Name:    info.Name(),
			Ext:     filepath.Ext(info.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Content: content,
		}

		batch.Index(absPath, doc)
		batchCount++
		result.Indexed++

		if bi.perfManager != nil {
			bi.perfManager.SetFileHash(absPath, fileHash)
		}

		if batchCount >= bi.batchSize {
			if err := bi.indexer.index.Batch(batch); err != nil {
				bi.logger.Error("batch index failed", zap.Error(err))
				result.Failed += batchCount
				result.Indexed -= batchCount
			}
			batch = bi.indexer.index.NewBatch()
			batchCount = 0
		}
	}

	if batchCount > 0 {
		if err := bi.indexer.index.Batch(batch); err != nil {
			bi.logger.Error("final batch index failed", zap.Error(err))
			result.Failed += batchCount
			result.Indexed -= batchCount
		}
	}

	// 索引更新后使查询缓存失效
	if bi.perfManager != nil && result.Indexed > 0 {
		bi.perfManager.InvalidateQueryCache()
	}

	bi.indexer.mu.Lock()
	bi.indexer.stats.lastUpdate = time.Now()
	bi.indexer.mu.Unlock()

	result.Duration = time.Since(start)
	return result
}

// BatchIndexDirectory 批量索引目录（增量模式）。
// 只索引新增或修改的文件，删除已不存在的文件索引。
func (bi *BatchIndexer) BatchIndexDirectory(root string) *BatchIndexResult {
	start := time.Now()
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return &BatchIndexResult{
			Total:    0,
			Failed:   1,
			Errors:   []string{fmt.Sprintf("abs path: %v", err)},
			Duration: time.Since(start),
		}
	}

	result := &BatchIndexResult{}
	foundPaths := make(map[string]bool)

	batch := bi.indexer.index.NewBatch()
	batchCount := 0

	err = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !bi.indexer.shouldIndex(path) {
			return nil
		}

		result.Total++
		foundPaths[path] = true

		// 增量更新检查
		fileHash := computeFileHash(path, info)
		if bi.perfManager != nil && !bi.perfManager.HasFileChanged(path, fileHash) {
			result.Skipped++
			return nil
		}

		content, _ := bi.extractor.Extract(path)

		doc := indexedDoc{
			Path:    path,
			Name:    info.Name(),
			Ext:     filepath.Ext(info.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Content: content,
		}

		batch.Index(path, doc)
		batchCount++
		result.Indexed++

		if bi.perfManager != nil {
			bi.perfManager.SetFileHash(path, fileHash)
		}

		if batchCount >= bi.batchSize {
			if err := bi.indexer.index.Batch(batch); err != nil {
				bi.logger.Error("batch index failed", zap.Error(err))
			}
			batch = bi.indexer.index.NewBatch()
			batchCount = 0
		}

		return nil
	})

	if batchCount > 0 {
		if batchErr := bi.indexer.index.Batch(batch); batchErr != nil {
			bi.logger.Error("final batch index failed", zap.Error(batchErr))
		}
	}

	if err != nil {
		result.Failed++
		result.Errors = append(result.Errors, err.Error())
	}

	// 检查已删除的文件
	if bi.perfManager != nil {
		bi.perfManager.incrementalMu.RLock()
		trackedPaths := make([]string, 0, len(bi.perfManager.fileHashes))
		for p := range bi.perfManager.fileHashes {
			if strings.HasPrefix(p, absRoot) {
				trackedPaths = append(trackedPaths, p)
			}
		}
		bi.perfManager.incrementalMu.RUnlock()

		deleteBatch := bi.indexer.index.NewBatch()
		deleteCount := 0
		for _, p := range trackedPaths {
			if !foundPaths[p] {
				deleteBatch.Delete(p)
				bi.perfManager.RemoveFileHash(p)
				result.DeletedFiles++
				deleteCount++
				if deleteCount >= bi.batchSize {
					if err := bi.indexer.index.Batch(deleteBatch); err != nil {
						bi.logger.Error("batch delete failed", zap.Error(err))
					}
					deleteBatch = bi.indexer.index.NewBatch()
					deleteCount = 0
				}
			}
		}
		if deleteCount > 0 {
			if err := bi.indexer.index.Batch(deleteBatch); err != nil {
				bi.logger.Error("final batch delete failed", zap.Error(err))
			}
		}
	}

	// 索引更新后使查询缓存失效
	if bi.perfManager != nil && (result.Indexed > 0 || result.DeletedFiles > 0) {
		bi.perfManager.InvalidateQueryCache()
	}

	bi.indexer.mu.Lock()
	bi.indexer.stats.lastUpdate = time.Now()
	bi.indexer.mu.Unlock()

	result.Duration = time.Since(start)
	return result
}

// computeFileHash 计算文件哈希用于增量更新判断。
// 使用文件大小 + 修改时间作为快速哈希，避免读取整个文件。
func computeFileHash(path string, info os.FileInfo) uint64 {
	// 使用修改时间和大小作为快速哈希
	// 这是一个轻量级的方案，适用于大多数场景
	modTime := info.ModTime().UnixNano()
	size := info.Size()
	hash := uint64(size)
	hash ^= uint64(modTime)
	hash *= 1099511628211 // FNV prime
	return hash
}

// ─── 优化查询执行 ─────────────────────────────────────────────

// OptimizedSearch 执行优化后的搜索。
// 优先检查查询缓存，命中则直接返回；未命中则执行实际搜索并缓存结果。
func OptimizedSearch(indexer *Indexer, pm *PerformanceManager, req SearchRequest) (*SearchResponse, error) {
	// 检查查询缓存
	if pm != nil {
		if cached, ok := pm.GetCachedQuery(req); ok {
			return cached, nil
		}
	}

	// 执行实际搜索
	resp, err := indexer.Search(req)
	if err != nil {
		return nil, err
	}

	// 缓存结果
	if pm != nil {
		pm.PutCachedQuery(req, resp)
	}

	return resp, nil
}

// ─── 辅助函数 ─────────────────────────────────────────────────

// detectSSD 检测路径是否位于 SSD 上。
// 通过检查 /sys/block 设端的旋转标志来判断。
func detectSSD(path string) bool {
	// 检查 /sys/block 下的设备旋转标志
	// 这是一个 Linux 特定的实现
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		// 无法检测时假设 SSD（启用缓存更安全）
		return true
	}

	for _, entry := range entries {
		name := entry.Name()
		// 跳过 loop、ram 等虚拟设备
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "sr") {
			continue
		}

		rotationalPath := filepath.Join("/sys/block", name, "queue", "rotational")
		data, err := os.ReadFile(rotationalPath)
		if err != nil {
			continue
		}

		rotational := strings.TrimSpace(string(data))
		if rotational == "0" {
			// 非旋转设备 = SSD
			return true
		}
	}

	return false
}

// SSDAwareConfig 根据存储类型返回优化的性能配置。
func SSDAwareConfig(indexPath string) PerformanceConfig {
	cfg := DefaultPerformanceConfig()

	if detectSSD(indexPath) {
		// SSD: 更大的缓存，更长的 TTL
		cfg.IndexCacheSize = 2000
		cfg.QueryCacheSize = 1000
		cfg.QueryCacheTTL = "10m"
	} else {
		// HDD: 较小缓存，较短 TTL
		cfg.IndexCacheSize = 500
		cfg.QueryCacheSize = 200
		cfg.QueryCacheTTL = "2m"
	}

	return cfg
}

// PerformanceStatus 性能状态汇总。
type PerformanceStatus struct {
	IndexCache    *CacheStats        `json:"indexCache,omitempty"`
	QueryCache    *QueryCacheStats   `json:"queryCache,omitempty"`
	Incremental   IncrementalStats   `json:"incremental"`
	IsSSD         bool               `json:"isSSD"`
}

// GetPerformanceStatus 获取性能状态汇总。
func (pm *PerformanceManager) GetPerformanceStatus(indexPath string) PerformanceStatus {
	status := PerformanceStatus{
		Incremental: pm.IncrementalStats(),
		IsSSD:       detectSSD(indexPath),
	}
	if pm.indexCache != nil {
		s := pm.indexCache.Stats()
		status.IndexCache = &s
	}
	if pm.queryCache != nil {
		s := pm.queryCache.Stats()
		status.QueryCache = &s
	}
	return status
}

// ensure bleve import is used
var _ bleve.Index
