// Package perf 提供性能优化功能
package perf

import (
	"container/list"
	"encoding/json"
	"sync"
	"time"
)

// QueryCache 查询结果缓存系统
// 支持 LRU 淘汰策略和 TTL 过期
type QueryCache struct {
	mu sync.RWMutex

	// LRU 队列
	lruList  *list.List
	lruIndex map[string]*list.Element // key -> element

	// 缓存数据
	items map[string]*CacheItem

	// 配置
	config CacheConfig

	// 统计
	stats CacheStats
}

// CacheConfig 缓存配置
type CacheConfig struct {
	// 最大条目数
	MaxItems int
	// 最大内存占用（字节）
	MaxMemory int64
	// 默认 TTL
	DefaultTTL time.Duration
	// 清理间隔
	CleanupInterval time.Duration
	// 是否启用统计
	EnableStats bool
	// 键前缀
	KeyPrefix string
}

// DefaultCacheConfig 默认缓存配置
var DefaultCacheConfig = CacheConfig{
	MaxItems:        10000,
	MaxMemory:       100 * 1024 * 1024, // 100MB
	DefaultTTL:      5 * time.Minute,
	CleanupInterval: 1 * time.Minute,
	EnableStats:     true,
	KeyPrefix:       "",
}

// CacheItem 缓存条目
type CacheItem struct {
	Key         string      `json:"key"`
	Value       interface{} `json:"value"`
	Size        int64       `json:"size"`        // 估算大小（字节）
	CreatedAt   time.Time   `json:"createdAt"`   // 创建时间
	ExpiresAt   time.Time   `json:"expiresAt"`   // 过期时间
	AccessAt    time.Time   `json:"accessAt"`    // 最后访问时间
	AccessCount int64       `json:"accessCount"` // 访问次数
	HitCount    int64       `json:"hitCount"`    // 命中次数
	Tags        []string    `json:"tags"`        // 标签（用于批量失效）
}

// CacheStats 缓存统计
type CacheStats struct {
	mu sync.RWMutex

	TotalItems    int64 `json:"totalItems"`
	TotalMemory   int64 `json:"totalMemory"`
	Hits          int64 `json:"hits"`
	Misses        int64 `json:"misses"`
	Evictions     int64 `json:"evictions"`
	Expirations   int64 `json:"expirations"`
	TotalSets     int64 `json:"totalSets"`
	TotalDeletes  int64 `json:"totalDeletes"`
	LastCleanupAt int64 `json:"lastCleanupAt"`
}

// HitRate 计算命中率
func (s *CacheStats) HitRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total)
}

// NewQueryCache 创建查询缓存
func NewQueryCache(config CacheConfig) *QueryCache {
	if config.MaxItems <= 0 {
		config.MaxItems = DefaultCacheConfig.MaxItems
	}
	if config.MaxMemory <= 0 {
		config.MaxMemory = DefaultCacheConfig.MaxMemory
	}
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = DefaultCacheConfig.DefaultTTL
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = DefaultCacheConfig.CleanupInterval
	}

	c := &QueryCache{
		lruList:  list.New(),
		lruIndex: make(map[string]*list.Element),
		items:    make(map[string]*CacheItem),
		config:   config,
	}

	// 启动清理协程
	if config.CleanupInterval > 0 {
		go c.cleanupLoop()
	}

	return c
}

// lruEntry LRU 条目
type lruEntry struct {
	key string
}

// Set 设置缓存
func (c *QueryCache) Set(key string, value interface{}, ttl ...time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 计算过期时间
	expiry := time.Now()
	if len(ttl) > 0 && ttl[0] > 0 {
		expiry = expiry.Add(ttl[0])
	} else {
		expiry = expiry.Add(c.config.DefaultTTL)
	}

	// 计算大小
	size := estimateSize(value)

	// 如果已存在，先删除
	if _, exists := c.items[key]; exists {
		c.deleteLocked(key)
	}

	// 检查是否需要淘汰
	for c.needEvictionLocked(size) {
		c.evictLRULocked()
	}

	// 创建缓存条目
	item := &CacheItem{
		Key:         key,
		Value:       value,
		Size:        size,
		CreatedAt:   time.Now(),
		ExpiresAt:   expiry,
		AccessAt:    time.Now(),
		AccessCount: 0,
		HitCount:    0,
	}

	// 添加到缓存
	c.items[key] = item

	// 添加到 LRU 队列（头部）
	element := c.lruList.PushFront(&lruEntry{key: key})
	c.lruIndex[key] = element

	// 更新统计
	if c.config.EnableStats {
		c.stats.mu.Lock()
		c.stats.TotalItems++
		c.stats.TotalMemory += size
		c.stats.TotalSets++
		c.stats.mu.Unlock()
	}

	return nil
}

// Get 获取缓存
func (c *QueryCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		if c.config.EnableStats {
			c.stats.mu.Lock()
			c.stats.Misses++
			c.stats.mu.Unlock()
		}
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(item.ExpiresAt) {
		c.deleteLocked(key)
		if c.config.EnableStats {
			c.stats.mu.Lock()
			c.stats.Misses++
			c.stats.Expirations++
			c.stats.mu.Unlock()
		}
		return nil, false
	}

	// 更新访问信息
	item.AccessAt = time.Now()
	item.AccessCount++
	item.HitCount++

	// 移动到 LRU 头部
	if element, ok := c.lruIndex[key]; ok {
		c.lruList.MoveToFront(element)
	}

	// 更新统计
	if c.config.EnableStats {
		c.stats.mu.Lock()
		c.stats.Hits++
		c.stats.mu.Unlock()
	}

	return item.Value, true
}

// GetWithTTL 获取缓存并返回剩余 TTL
func (c *QueryCache) GetWithTTL(key string) (interface{}, time.Duration, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		if c.config.EnableStats {
			c.stats.mu.Lock()
			c.stats.Misses++
			c.stats.mu.Unlock()
		}
		return nil, 0, false
	}

	// 检查是否过期
	now := time.Now()
	if now.After(item.ExpiresAt) {
		c.deleteLocked(key)
		if c.config.EnableStats {
			c.stats.mu.Lock()
			c.stats.Misses++
			c.stats.Expirations++
			c.stats.mu.Unlock()
		}
		return nil, 0, false
	}

	// 计算剩余 TTL
	remainingTTL := item.ExpiresAt.Sub(now)

	// 更新访问信息
	item.AccessAt = now
	item.AccessCount++
	item.HitCount++

	// 移动到 LRU 头部
	if element, ok := c.lruIndex[key]; ok {
		c.lruList.MoveToFront(element)
	}

	// 更新统计
	if c.config.EnableStats {
		c.stats.mu.Lock()
		c.stats.Hits++
		c.stats.mu.Unlock()
	}

	return item.Value, remainingTTL, true
}

// Delete 删除缓存
func (c *QueryCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.items[key]; !exists {
		return false
	}

	c.deleteLocked(key)

	if c.config.EnableStats {
		c.stats.mu.Lock()
		c.stats.TotalDeletes++
		c.stats.mu.Unlock()
	}

	return true
}

// deleteLocked 删除缓存（已持有锁）
func (c *QueryCache) deleteLocked(key string) {
	item, exists := c.items[key]
	if !exists {
		return
	}

	// 从 LRU 队列删除
	if element, ok := c.lruIndex[key]; ok {
		c.lruList.Remove(element)
		delete(c.lruIndex, key)
	}

	// 从缓存删除
	delete(c.items, key)

	// 更新统计
	if c.config.EnableStats {
		c.stats.mu.Lock()
		c.stats.TotalItems--
		c.stats.TotalMemory -= item.Size
		c.stats.mu.Unlock()
	}
}

// Clear 清空缓存
func (c *QueryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*CacheItem)
	c.lruList = list.New()
	c.lruIndex = make(map[string]*list.Element)

	if c.config.EnableStats {
		c.stats.mu.Lock()
		c.stats.TotalItems = 0
		c.stats.TotalMemory = 0
		c.stats.mu.Unlock()
	}
}

// needEvictionLocked 检查是否需要淘汰
func (c *QueryCache) needEvictionLocked(newSize int64) bool {
	// 检查条目数
	if len(c.items) >= c.config.MaxItems {
		return true
	}

	// 检查内存
	if c.config.MaxMemory > 0 {
		var totalSize int64
		for _, item := range c.items {
			totalSize += item.Size
		}
		if totalSize+newSize > c.config.MaxMemory {
			return true
		}
	}

	return false
}

// evictLRULocked 淘汰 LRU 条目
func (c *QueryCache) evictLRULocked() {
	// 从 LRU 尾部淘汰
	element := c.lruList.Back()
	if element == nil {
		return
	}

	entry, ok := element.Value.(*lruEntry)
	if !ok {
		return
	}
	c.deleteLocked(entry.key)

	if c.config.EnableStats {
		c.stats.mu.Lock()
		c.stats.Evictions++
		c.stats.mu.Unlock()
	}
}

// cleanupLoop 定期清理过期条目
func (c *QueryCache) cleanupLoop() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.Cleanup()
	}
}

// Cleanup 清理过期条目
func (c *QueryCache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	// 找出所有过期条目
	for key, item := range c.items {
		if now.After(item.ExpiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// 删除过期条目
	for _, key := range expiredKeys {
		c.deleteLocked(key)
		if c.config.EnableStats {
			c.stats.mu.Lock()
			c.stats.Expirations++
			c.stats.mu.Unlock()
		}
	}

	if c.config.EnableStats {
		c.stats.mu.Lock()
		c.stats.LastCleanupAt = now.Unix()
		c.stats.mu.Unlock()
	}

	return len(expiredKeys)
}

// Stats 获取统计信息
func (c *QueryCache) Stats() CacheStats {
	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()
	// 手动复制字段，避免复制 mutex
	return CacheStats{
		TotalItems:    c.stats.TotalItems,
		TotalMemory:   c.stats.TotalMemory,
		Hits:          c.stats.Hits,
		Misses:        c.stats.Misses,
		Evictions:     c.stats.Evictions,
		Expirations:   c.stats.Expirations,
		TotalSets:     c.stats.TotalSets,
		TotalDeletes:  c.stats.TotalDeletes,
		LastCleanupAt: c.stats.LastCleanupAt,
	}
}

// Keys 获取所有键
func (c *QueryCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for key := range c.items {
		keys = append(keys, key)
	}
	return keys
}

// Size 获取缓存大小
func (c *QueryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Memory 获取内存占用
func (c *QueryCache) Memory() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var total int64
	for _, item := range c.items {
		total += item.Size
	}
	return total
}

// Has 检查键是否存在
func (c *QueryCache) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return false
	}

	// 检查是否过期
	if time.Now().After(item.ExpiresAt) {
		return false
	}

	return true
}

// SetWithTags 设置带标签的缓存
func (c *QueryCache) SetWithTags(key string, value interface{}, tags []string, ttl ...time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 计算过期时间
	expiry := time.Now()
	if len(ttl) > 0 && ttl[0] > 0 {
		expiry = expiry.Add(ttl[0])
	} else {
		expiry = expiry.Add(c.config.DefaultTTL)
	}

	// 计算大小
	size := estimateSize(value)

	// 如果已存在，先删除
	if _, exists := c.items[key]; exists {
		c.deleteLocked(key)
	}

	// 检查是否需要淘汰
	for c.needEvictionLocked(size) {
		c.evictLRULocked()
	}

	// 创建缓存条目
	item := &CacheItem{
		Key:         key,
		Value:       value,
		Size:        size,
		CreatedAt:   time.Now(),
		ExpiresAt:   expiry,
		AccessAt:    time.Now(),
		AccessCount: 0,
		HitCount:    0,
		Tags:        tags,
	}

	// 添加到缓存
	c.items[key] = item

	// 添加到 LRU 队列（头部）
	element := c.lruList.PushFront(&lruEntry{key: key})
	c.lruIndex[key] = element

	// 更新统计
	if c.config.EnableStats {
		c.stats.mu.Lock()
		c.stats.TotalItems++
		c.stats.TotalMemory += size
		c.stats.TotalSets++
		c.stats.mu.Unlock()
	}

	return nil
}

// InvalidateByTag 按标签失效缓存
func (c *QueryCache) InvalidateByTag(tag string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for key, item := range c.items {
		for _, t := range item.Tags {
			if t == tag {
				c.deleteLocked(key)
				count++
				break
			}
		}
	}

	return count
}

// InvalidateByTags 按多个标签失效缓存
func (c *QueryCache) InvalidateByTags(tags []string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	tagSet := make(map[string]bool)
	for _, t := range tags {
		tagSet[t] = true
	}

	count := 0
	for key, item := range c.items {
		for _, t := range item.Tags {
			if tagSet[t] {
				c.deleteLocked(key)
				count++
				break
			}
		}
	}

	return count
}

// GetOrSet 获取或设置缓存
func (c *QueryCache) GetOrSet(key string, fn func() (interface{}, error), ttl ...time.Duration) (interface{}, error) {
	// 先尝试获取
	if value, ok := c.Get(key); ok {
		return value, nil
	}

	// 执行函数获取值
	value, err := fn()
	if err != nil {
		return nil, err
	}

	// 设置缓存
	if len(ttl) > 0 {
		_ = c.Set(key, value, ttl[0])
	} else {
		_ = c.Set(key, value)
	}

	return value, nil
}

// GetItem 获取缓存条目信息
func (c *QueryCache) GetItem(key string) (*CacheItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(item.ExpiresAt) {
		return nil, false
	}

	return item, true
}

// estimateSize 估算值的大小
func estimateSize(value interface{}) int64 {
	if value == nil {
		return 0
	}

	// 尝试 JSON 序列化估算大小
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
		return 8
	default:
		// JSON 序列化估算
		if data, err := json.Marshal(value); err == nil {
			return int64(len(data))
		}
		return 64 // 默认估算
	}
}

// QueryCacheEntry 查询缓存条目（用于导出）
type QueryCacheEntry struct {
	Key         string      `json:"key"`
	Value       interface{} `json:"value,omitempty"`
	Size        int64       `json:"size"`
	CreatedAt   time.Time   `json:"createdAt"`
	ExpiresAt   time.Time   `json:"expiresAt"`
	AccessAt    time.Time   `json:"accessAt"`
	AccessCount int64       `json:"accessCount"`
	HitCount    int64       `json:"hitCount"`
	TTL         int64       `json:"ttl"` // 剩余秒数
}

// Export 导出缓存条目
func (c *QueryCache) Export() []QueryCacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	entries := make([]QueryCacheEntry, 0, len(c.items))

	for key, item := range c.items {
		ttl := item.ExpiresAt.Sub(now).Seconds()
		if ttl < 0 {
			ttl = 0
		}

		entries = append(entries, QueryCacheEntry{
			Key:         key,
			Size:        item.Size,
			CreatedAt:   item.CreatedAt,
			ExpiresAt:   item.ExpiresAt,
			AccessAt:    item.AccessAt,
			AccessCount: item.AccessCount,
			HitCount:    item.HitCount,
			TTL:         int64(ttl),
		})
	}

	return entries
}

// Warmup 预热缓存
func (c *QueryCache) Warmup(items map[string]interface{}, ttl time.Duration) int {
	count := 0
	for key, value := range items {
		if err := c.Set(key, value, ttl); err == nil {
			count++
		}
	}
	return count
}

// ResetStats 重置统计
func (c *QueryCache) ResetStats() {
	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()

	// 只重置统计字段，不覆盖互斥锁
	c.stats.Hits = 0
	c.stats.Misses = 0
	c.stats.Evictions = 0
	c.stats.Expirations = 0
	c.stats.TotalSets = 0
	c.stats.TotalDeletes = 0
	c.stats.LastCleanupAt = 0
}
