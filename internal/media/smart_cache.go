// Package media provides smart caching for metadata
package media

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SmartCache 智能缓存 - 支持TTL、持久化、统计
type SmartCache struct {
	items     map[string]*CacheItem
	stats     CacheStats
	ttl       time.Duration
	persistPath string
	mu        sync.RWMutex
}

// CacheItem 缓存项
type CacheItem struct {
	Key       string
	Value     interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
	HitCount  int
	Size      int64
}

// CacheStats 缓存统计
type CacheStats struct {
	TotalItems   int
	TotalHits    int
	TotalMisses  int
	TotalSize    int64
	EvictedItems int
	LastCleanup  time.Time
}

// NewSmartCache 创建智能缓存
func NewSmartCache(ttl time.Duration) *SmartCache {
	return &SmartCache{
		items: make(map[string]*CacheItem),
		ttl:   ttl,
	}
}

// Set 设置缓存
func (c *SmartCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 确定过期时间
	if ttl == 0 {
		ttl = c.ttl
	}

	item := &CacheItem{
		Key:       key,
		Value:     value,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		HitCount:  0,
		Size:      estimateSize(value),
	}

	c.items[key] = item
	c.stats.TotalItems++
	c.stats.TotalSize += item.Size
}

// Get 获取缓存
func (c *SmartCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		c.stats.TotalMisses++
		return nil, false
	}

	// 检查过期
	if time.Now().After(item.ExpiresAt) {
		delete(c.items, key)
		c.stats.TotalItems--
		c.stats.EvictedItems++
		c.stats.TotalMisses++
		return nil, false
	}

	item.HitCount++
	c.stats.TotalHits++
	return item.Value, true
}

// GetScrapeResult 获取刮削结果缓存
func (c *SmartCache) GetScrapeResult(filePath string) (*ScrapeResult, bool) {
	val, ok := c.Get(filePath)
	if !ok {
		return nil, false
	}

	if sr, ok := val.(*ScrapeResult); ok {
		return sr, true
	}
	return nil, false
}

// Delete 删除缓存
func (c *SmartCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.items[key]; ok {
		c.stats.TotalItems--
		c.stats.TotalSize -= item.Size
		delete(c.items, key)
	}
}

// Clear 清空缓存
func (c *SmartCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*CacheItem)
	c.stats.TotalItems = 0
	c.stats.TotalSize = 0
	c.stats.LastCleanup = time.Now()
}

// Cleanup 清理过期缓存
func (c *SmartCache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	evicted := 0

	for key, item := range c.items {
		if now.After(item.ExpiresAt) {
			c.stats.TotalSize -= item.Size
			delete(c.items, key)
			evicted++
			c.stats.EvictedItems++
		}
	}

	c.stats.TotalItems -= evicted
	c.stats.LastCleanup = now
	return evicted
}

// Stats 获取缓存统计
func (c *SmartCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// Size 获取缓存大小
func (c *SmartCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// HitRate 获取命中率
func (c *SmartCache) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.stats.TotalHits + c.stats.TotalMisses
	if total == 0 {
		return 0
	}
	return float64(c.stats.TotalHits) / float64(total)
}

// Persist 持久化缓存到文件
func (c *SmartCache) Persist(path string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data := struct {
		Items     map[string]*CacheItem `json:"items"`
		Stats     CacheStats            `json:"stats"`
		SavedAt   time.Time             `json:"saved_at"`
	}{
		Items:   c.items,
		Stats:   c.stats,
		SavedAt: time.Now(),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, jsonData, 0644)
}

// Load 从文件加载缓存
func (c *SmartCache) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var loaded struct {
		Items     map[string]*CacheItem `json:"items"`
		Stats     CacheStats            `json:"stats"`
		SavedAt   time.Time             `json:"saved_at"`
	}

	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 过滤已过期的项
	now := time.Now()
	c.items = make(map[string]*CacheItem)
	for key, item := range loaded.Items {
		if now.Before(item.ExpiresAt) {
			c.items[key] = item
			c.stats.TotalItems++
			c.stats.TotalSize += item.Size
		}
	}

	c.persistPath = path
	return nil
}

// AutoPersist 自动持久化（定期）
func (c *SmartCache) AutoPersist(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		for range ticker.C {
			if c.persistPath != "" {
				_ = c.Persist(c.persistPath)
			}
		}
	}()
}

// estimateSize 估算值大小
func estimateSize(value interface{}) int64 {
	// 简化估算
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case *ScrapeResult:
		// 粗略估算
		return 1024 // 1KB per result
	case *MediaMetadata:
		return 2048 // 2KB per metadata
	default:
		// JSON序列化估算
		data, _ := json.Marshal(v)
		return int64(len(data))
	}
}

// ====== 缓存键生成 ======

// GenerateCacheKeyV2 生成缓存键
func GenerateCacheKeyV2(mediaType MediaType, title string, year int) string {
	return string(mediaType) + ":" + title + ":" + string(year)
}

// GenerateFileCacheKeyV2 生成文件缓存键（基于路径hash）
func GenerateFileCacheKeyV2(filePath string) string {
	// 使用文件路径作为键
	return filePath
}