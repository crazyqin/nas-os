// Package aisearch 提供搜索结果缓存功能
package aisearch

import (
	"sync"
	"time"
)

// NewSearchCache 创建搜索缓存
func NewSearchCache(maxSize int, ttl time.Duration) *SearchCache {
	cache := &SearchCache{
		items:   make(map[string]*CacheItem),
		maxSize: maxSize,
		ttl:     ttl,
	}

	// 启动清理协程
	go cache.cleanup()

	return cache
}

// Get 获取缓存项
func (c *SearchCache) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		c.misses++
		return nil
	}

	// 检查是否过期
	if time.Now().After(item.ExpiresAt) {
		c.misses++
		return nil
	}

	c.hits++
	return item.Value
}

// Set 设置缓存项
func (c *SearchCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果缓存已满，删除最旧的项
	if len(c.items) >= c.maxSize {
		c.evictOldest()
	}

	c.items[key] = &CacheItem{
		Key:       key,
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Delete 删除缓存项
func (c *SearchCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Clear 清空缓存
func (c *SearchCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*CacheItem)
	c.hits = 0
	c.misses = 0
}

// Size 获取缓存大小
func (c *SearchCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

// Hits 获取命中次数
func (c *SearchCache) Hits() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.hits
}

// Misses 获取未命中次数
func (c *SearchCache) Misses() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.misses
}

// HitRate 获取命中率
func (c *SearchCache) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	if total == 0 {
		return 0
	}

	return float64(c.hits) / float64(total)
}

// evictOldest 驱逐最旧的项
func (c *SearchCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range c.items {
		if oldestKey == "" || item.ExpiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.ExpiresAt
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

// cleanup 定期清理过期项
func (c *SearchCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.ExpiresAt) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

// LRUCache LRU 缓存实现
type LRUCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*LRUNode
	head     *LRUNode
	tail     *LRUNode
	hits     int64
	misses   int64
}

// LRUNode LRU 节点
type LRUNode struct {
	key       string
	value     interface{}
	expiresAt time.Time
	prev      *LRUNode
	next      *LRUNode
}

// NewLRUCache 创建 LRU 缓存
func NewLRUCache(capacity int, ttl time.Duration) *LRUCache {
	cache := &LRUCache{
		capacity: capacity,
		items:    make(map[string]*LRUNode),
	}

	// 创建哨兵节点
	cache.head = &LRUNode{}
	cache.tail = &LRUNode{}
	cache.head.next = cache.tail
	cache.tail.prev = cache.head

	// 启动清理协程
	go cache.cleanup(ttl)

	return cache
}

// Get 获取缓存项
func (c *LRUCache) Get(key string) interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, ok := c.items[key]
	if !ok {
		c.misses++
		return nil
	}

	// 检查是否过期
	if time.Now().After(node.expiresAt) {
		c.removeNode(node)
		delete(c.items, key)
		c.misses++
		return nil
	}

	// 移动到头部
	c.moveToHead(node)
	c.hits++

	return node.value
}

// Set 设置缓存项
func (c *LRUCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，更新
	if node, ok := c.items[key]; ok {
		node.value = value
		node.expiresAt = time.Now().Add(ttl)
		c.moveToHead(node)
		return
	}

	// 如果缓存已满，删除尾部
	if len(c.items) >= c.capacity {
		oldest := c.tail.prev
		c.removeNode(oldest)
		delete(c.items, oldest.key)
	}

	// 创建新节点
	node := &LRUNode{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}

	c.items[key] = node
	c.addToHead(node)
}

// Delete 删除缓存项
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.items[key]; ok {
		c.removeNode(node)
		delete(c.items, key)
	}
}

// Clear 清空缓存
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*LRUNode)
	c.head.next = c.tail
	c.tail.prev = c.head
	c.hits = 0
	c.misses = 0
}

// Size 获取缓存大小
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

// HitRate 获取命中率
func (c *LRUCache) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	if total == 0 {
		return 0
	}

	return float64(c.hits) / float64(total)
}

// addToHead 添加到头部
func (c *LRUCache) addToHead(node *LRUNode) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

// removeNode 移除节点
func (c *LRUCache) removeNode(node *LRUNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

// moveToHead 移动到头部
func (c *LRUCache) moveToHead(node *LRUNode) {
	c.removeNode(node)
	c.addToHead(node)
}

// cleanup 定期清理过期项
func (c *LRUCache) cleanup(ttl time.Duration) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, node := range c.items {
			if now.After(node.expiresAt) {
				c.removeNode(node)
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

// SearchCacheManager 搜索缓存管理器
type SearchCacheManager struct {
	queryCache *SearchCache
	resultCache *LRUCache
	vectorCache *SearchCache
}

// NewSearchCacheManager 创建搜索缓存管理器
func NewSearchCacheManager() *SearchCacheManager {
	return &SearchCacheManager{
		queryCache:  NewSearchCache(1000, 10*time.Minute),
		resultCache: NewLRUCache(5000, 30*time.Minute),
		vectorCache: NewSearchCache(10000, 1*time.Hour),
	}
}

// GetQueryCache 获取查询缓存
func (m *SearchCacheManager) GetQueryCache() *SearchCache {
	return m.queryCache
}

// GetResultCache 获取结果缓存
func (m *SearchCacheManager) GetResultCache() *LRUCache {
	return m.resultCache
}

// GetVectorCache 获取向量缓存
func (m *SearchCacheManager) GetVectorCache() *SearchCache {
	return m.vectorCache
}

// ClearAll 清空所有缓存
func (m *SearchCacheManager) ClearAll() {
	m.queryCache.Clear()
	m.resultCache.Clear()
	m.vectorCache.Clear()
}

// Stats 获取缓存统计
func (m *SearchCacheManager) Stats() map[string]interface{} {
	return map[string]interface{}{
		"queryCache": map[string]interface{}{
			"size":    m.queryCache.Size(),
			"hits":    m.queryCache.Hits(),
			"misses":  m.queryCache.Misses(),
			"hitRate": m.queryCache.HitRate(),
		},
		"resultCache": map[string]interface{}{
			"size":    m.resultCache.Size(),
			"hitRate": m.resultCache.HitRate(),
		},
		"vectorCache": map[string]interface{}{
			"size":    m.vectorCache.Size(),
			"hits":    m.vectorCache.Hits(),
			"misses":  m.vectorCache.Misses(),
			"hitRate": m.vectorCache.HitRate(),
		},
	}
}
