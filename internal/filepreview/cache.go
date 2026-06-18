package filepreview

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// PreviewCache 预览缓存管理器.
type PreviewCache struct {
	config    CacheConfig
	entries   map[string]*CacheEntry
	mu        sync.RWMutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
	stats     CacheStats
	statsMu   sync.Mutex
}

// NewPreviewCache 创建预览缓存管理器.
func NewPreviewCache(config CacheConfig) *PreviewCache {
	if config.CacheDir == "" {
		config.CacheDir = "/var/cache/nas-os/preview"
	}
	if config.MaxSize <= 0 {
		config.MaxSize = 1 << 30 // 1GB
	}
	if config.MaxEntries <= 0 {
		config.MaxEntries = 10000
	}
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = 7 * 24 * time.Hour
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 1 * time.Hour
	}

	cache := &PreviewCache{
		config:  config,
		entries: make(map[string]*CacheEntry),
		stopCh:  make(chan struct{}),
	}

	// 加载现有缓存索引.
	cache.loadIndex()

	// 启动清理协程.
	cache.wg.Add(1)
	go cache.cleanupLoop()

	return cache
}

// Get 获取缓存条目.
func (c *PreviewCache) Get(key string) (*CacheEntry, error) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		c.recordMiss()
		return nil, ErrCacheMiss
	}

	// 检查是否过期.
	if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
		c.recordMiss()
		return nil, ErrCacheExpired
	}

	// 检查文件是否存在.
	if _, err := os.Stat(entry.FilePath); err != nil {
		c.Delete(key)
		c.recordMiss()
		return nil, ErrCacheMiss
	}

	// 检查源文件是否已修改.
	sourceInfo, err := os.Stat(entry.SourcePath)
	if err == nil && sourceInfo.ModTime().After(entry.SourceModTime) {
		c.Delete(key)
		c.recordMiss()
		return nil, ErrCacheExpired
	}

	// 更新访问时间.
	c.mu.Lock()
	entry.AccessedAt = time.Now()
	entry.AccessCount++
	c.mu.Unlock()

	c.recordHit()
	return entry, nil
}

// Set 设置缓存条目.
func (c *PreviewCache) Set(key string, entry *CacheEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否需要淘汰.
	if err := c.evictIfNeeded(); err != nil {
		return err
	}

	// 设置默认过期时间.
	if entry.ExpiresAt == nil {
		expiry := time.Now().Add(c.config.DefaultTTL)
		entry.ExpiresAt = &expiry
	}

	// 设置访问时间.
	entry.AccessedAt = time.Now()
	entry.CreatedAt = time.Now()

	c.entries[key] = entry

	// 保存索引.
	go c.saveIndex()

	return nil
}

// Delete 删除缓存条目.
func (c *PreviewCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil
	}

	// 删除预览文件.
	if entry.FilePath != "" {
		os.Remove(entry.FilePath)
	}

	delete(c.entries, key)

	// 保存索引.
	go c.saveIndex()

	return nil
}

// DeleteBySource 删除指定源文件的所有缓存.
func (c *PreviewCache) DeleteBySource(sourcePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var keysToDelete []string
	for key, entry := range c.entries {
		if entry.SourcePath == sourcePath {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		entry := c.entries[key]
		if entry.FilePath != "" {
			os.Remove(entry.FilePath)
		}
		delete(c.entries, key)
	}

	if len(keysToDelete) > 0 {
		go c.saveIndex()
	}

	return nil
}

// Clear 清空缓存.
func (c *PreviewCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 删除所有预览文件.
	for _, entry := range c.entries {
		if entry.FilePath != "" {
			os.Remove(entry.FilePath)
		}
	}

	c.entries = make(map[string]*CacheEntry)

	// 删除索引文件.
	indexPath := filepath.Join(c.config.CacheDir, "index.json")
	os.Remove(indexPath)

	return nil
}

// GetStats 获取缓存统计.
func (c *PreviewCache) GetStats() CacheStats {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()

	c.mu.RLock()
	totalEntries := len(c.entries)
	var totalSize int64
	for _, entry := range c.entries {
		totalSize += entry.FileSize
	}
	c.mu.RUnlock()

	c.stats.TotalEntries = totalEntries
	c.stats.TotalSize = totalSize
	c.stats.MaxSize = c.config.MaxSize

	return c.stats
}

// ListEntries 列出所有缓存条目.
func (c *PreviewCache) ListEntries() []*CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := make([]*CacheEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, entry)
	}

	// 按访问时间排序.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].AccessedAt.After(entries[j].AccessedAt)
	})

	return entries
}

// Cleanup 手动触发清理.
func (c *PreviewCache) Cleanup() (int, int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.cleanup()
}

// Close 关闭缓存管理器.
func (c *PreviewCache) Close() error {
	close(c.stopCh)
	c.wg.Wait()
	return c.saveIndex()
}

// GenerateCacheKey 生成缓存键.
func GenerateCacheKey(filePath string, width, height int, options ...string) string {
	key := fmt.Sprintf("%s_%dx%d", filePath, width, height)
	for _, opt := range options {
		key += "_" + opt
	}
	return key
}

// evictIfNeeded 淘汰旧条目.
func (c *PreviewCache) evictIfNeeded() error {
	// 检查条目数限制.
	for len(c.entries) >= c.config.MaxEntries {
		if err := c.evictLRU(); err != nil {
			return err
		}
	}

	// 检查大小限制.
	var totalSize int64
	for _, entry := range c.entries {
		totalSize += entry.FileSize
	}

	for totalSize > c.config.MaxSize {
		evicted, err := c.evictLRUSize()
		if err != nil {
			return err
		}
		totalSize -= evicted
	}

	return nil
}

// evictLRU 淘汰最近最少使用的条目.
func (c *PreviewCache) evictLRU() error {
	if len(c.entries) == 0 {
		return nil
	}

	// 找到最久未访问的条目.
	var oldestKey string
	var oldestTime time.Time
	for key, entry := range c.entries {
		if oldestKey == "" || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
		}
	}

	if oldestKey != "" {
		entry := c.entries[oldestKey]
		if entry.FilePath != "" {
			os.Remove(entry.FilePath)
		}
		delete(c.entries, oldestKey)
		c.recordEviction()
	}

	return nil
}

// evictLRUSize 淘汰条目释放空间.
func (c *PreviewCache) evictLRUSize() (int64, error) {
	if len(c.entries) == 0 {
		return 0, nil
	}

	// 找到最久未访问的条目.
	var oldestKey string
	var oldestTime time.Time
	for key, entry := range c.entries {
		if oldestKey == "" || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
		}
	}

	if oldestKey != "" {
		entry := c.entries[oldestKey]
		size := entry.FileSize
		if entry.FilePath != "" {
			os.Remove(entry.FilePath)
		}
		delete(c.entries, oldestKey)
		c.recordEviction()
		return size, nil
	}

	return 0, nil
}

// cleanup 清理过期条目.
func (c *PreviewCache) cleanup() (int, int64, error) {
	now := time.Now()
	var deletedCount int
	var deletedSize int64

	var keysToDelete []string
	for key, entry := range c.entries {
		// 检查过期.
		if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
			keysToDelete = append(keysToDelete, key)
			continue
		}

		// 检查文件是否存在.
		if _, err := os.Stat(entry.FilePath); err != nil {
			keysToDelete = append(keysToDelete, key)
			continue
		}

		// 检查源文件是否已修改.
		sourceInfo, err := os.Stat(entry.SourcePath)
		if err == nil && sourceInfo.ModTime().After(entry.SourceModTime) {
			keysToDelete = append(keysToDelete, key)
			continue
		}
	}

	for _, key := range keysToDelete {
		entry := c.entries[key]
		deletedSize += entry.FileSize
		deletedCount++
		if entry.FilePath != "" {
			os.Remove(entry.FilePath)
		}
		delete(c.entries, key)
	}

	return deletedCount, deletedSize, nil
}

// cleanupLoop 定期清理.
func (c *PreviewCache) cleanupLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			c.cleanup()
			c.mu.Unlock()
			c.saveIndex()
		case <-c.stopCh:
			return
		}
	}
}

// loadIndex 加载缓存索引.
func (c *PreviewCache) loadIndex() error {
	indexPath := filepath.Join(c.config.CacheDir, "index.json")

	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var entries map[string]*CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	// 验证条目.
	now := time.Now()
	for key, entry := range entries {
		// 检查过期.
		if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
			continue
		}

		// 检查文件是否存在.
		if _, err := os.Stat(entry.FilePath); err != nil {
			continue
		}

		c.entries[key] = entry
	}

	return nil
}

// saveIndex 保存缓存索引.
func (c *PreviewCache) saveIndex() error {
	c.mu.RLock()
	entries := make(map[string]*CacheEntry, len(c.entries))
	for k, v := range c.entries {
		entries[k] = v
	}
	c.mu.RUnlock()

	// 确保目录存在.
	if err := os.MkdirAll(c.config.CacheDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	indexPath := filepath.Join(c.config.CacheDir, "index.json")
	return os.WriteFile(indexPath, data, 0o644)
}

// recordHit 记录缓存命中.
func (c *PreviewCache) recordHit() {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	c.stats.HitCount++
}

// recordMiss 记录缓存未命中.
func (c *PreviewCache) recordMiss() {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	c.stats.MissCount++
}

// recordEviction 记录淘汰.
func (c *PreviewCache) recordEviction() {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	c.stats.EvictionCount++
}

// PreviewCacheWithTTL 带自定义 TTL 的缓存设置.
func (c *PreviewCache) SetWithTTL(key string, entry *CacheEntry, ttl time.Duration) error {
	expiry := time.Now().Add(ttl)
	entry.ExpiresAt = &expiry
	return c.Set(key, entry)
}

// Touch 更新缓存条目的访问时间.
func (c *PreviewCache) Touch(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		return ErrCacheMiss
	}

	entry.AccessedAt = time.Now()
	entry.AccessCount++

	return nil
}

// Size 获取当前缓存大小.
func (c *PreviewCache) Size() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var total int64
	for _, entry := range c.entries {
		total += entry.FileSize
	}
	return total
}

// Count 获取当前条目数.
func (c *PreviewCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Has 检查缓存是否存在且有效.
func (c *PreviewCache) Has(key string) bool {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		return false
	}

	// 检查过期.
	if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
		return false
	}

	// 检查文件存在.
	if _, err := os.Stat(entry.FilePath); err != nil {
		return false
	}

	return true
}

// InvalidateSource 使指定源文件的所有缓存失效.
func (c *PreviewCache) InvalidateSource(sourcePath string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	var count int
	for key, entry := range c.entries {
		if entry.SourcePath == sourcePath {
			if entry.FilePath != "" {
				os.Remove(entry.FilePath)
			}
			delete(c.entries, key)
			count++
		}
	}

	if count > 0 {
		go c.saveIndex()
	}

	return count
}
