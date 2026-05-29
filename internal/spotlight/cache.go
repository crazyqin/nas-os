package spotlight

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SearchCache 搜索缓存
// 提供 LRU 缓存机制，优化重复查询性能
type SearchCache struct {
	mu       sync.RWMutex
	entries  map[string]*CacheEntry
	order    []string // LRU 顺序
	maxSize  int
	ttl      time.Duration
	stopChan chan struct{}
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Key       string         `json:"key"`
	Response  *SearchResponse `json:"response"`
	CreatedAt time.Time      `json:"createdAt"`
	ExpiresAt time.Time      `json:"expiresAt"`
	HitCount  int64          `json:"hitCount"`
}

// CacheStats 缓存统计
type CacheStats struct {
	Size      int     `json:"size"`
	MaxSize   int     `json:"maxSize"`
	HitCount  int64   `json:"hitCount"`
	MissCount int64   `json:"missCount"`
	HitRate   float64 `json:"hitRate"`
}

// NewSearchCache 创建搜索缓存
func NewSearchCache(maxSize int, ttl time.Duration) *SearchCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	c := &SearchCache{
		entries:  make(map[string]*CacheEntry),
		order:    make([]string, 0, maxSize),
		maxSize:  maxSize,
		ttl:      ttl,
		stopChan: make(chan struct{}),
	}

	// 启动清理协程
	go c.cleanupLoop()

	return c
}

// Get 获取缓存
func (c *SearchCache) Get(key string) (*SearchResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		c.deleteEntry(key)
		return nil, false
	}

	// 更新 LRU 顺序
	c.moveToFront(key)

	// 更新命中计数
	entry.HitCount++

	return entry.Response, true
}

// Set 设置缓存
func (c *SearchCache) Set(key string, response *SearchResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，更新
	if entry, exists := c.entries[key]; exists {
		entry.Response = response
		entry.CreatedAt = time.Now()
		entry.ExpiresAt = time.Now().Add(c.ttl)
		c.moveToFront(key)
		return
	}

	// 检查是否需要淘汰
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	// 创建新条目
	entry := &CacheEntry{
		Key:       key,
		Response:  response,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(c.ttl),
		HitCount:  0,
	}

	c.entries[key] = entry
	c.order = append(c.order, key)
}

// Delete 删除缓存
func (c *SearchCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteEntry(key)
}

// Clear 清空缓存
func (c *SearchCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.order = make([]string, 0, c.maxSize)
}

// GenerateKey 生成缓存键
func (c *SearchCache) GenerateKey(req SearchRequest) string {
	// 将请求序列化为 JSON 并计算哈希
	data, _ := json.Marshal(req)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:16])
}

// GetStats 获取缓存统计
func (c *SearchCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		Size:    len(c.entries),
		MaxSize: c.maxSize,
	}

	for _, entry := range c.entries {
		stats.HitCount += entry.HitCount
	}

	total := stats.HitCount + stats.MissCount
	if total > 0 {
		stats.HitRate = float64(stats.HitCount) / float64(total)
	}

	return stats
}

// moveToFront 移动到最前面（最近使用）
func (c *SearchCache) moveToFront(key string) {
	// 找到当前位置
	for i, k := range c.order {
		if k == key {
			// 移除当前位置
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	// 添加到最前面
	c.order = append([]string{key}, c.order...)
}

// evictOldest 淘汰最旧的条目
func (c *SearchCache) evictOldest() {
	if len(c.order) == 0 {
		return
	}

	// 淘汰最后一个（最久未使用）
	oldestKey := c.order[len(c.order)-1]
	c.deleteEntry(oldestKey)
}

// deleteEntry 删除条目
func (c *SearchCache) deleteEntry(key string) {
	delete(c.entries, key)

	// 从 order 中移除
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// cleanupLoop 定期清理过期条目
func (c *SearchCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.cleanup()
		}
	}
}

// cleanup 清理过期条目
func (c *SearchCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		c.deleteEntry(key)
	}
}

// Stop 停止缓存
func (c *SearchCache) Stop() {
	close(c.stopChan)
}
