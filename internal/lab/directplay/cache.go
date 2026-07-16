package directplay

import (
	"sync"
	"time"
)

// LinkCache 直链缓存
type LinkCache struct {
	items    map[string]*cacheItem
	ttl      time.Duration
	maxItems int
	mu       sync.RWMutex
}

type cacheItem struct {
	link      *DirectLinkInfo
	expiresAt time.Time
}

// NewLinkCache 创建直链缓存
func NewLinkCache(ttl time.Duration, maxItems int) *LinkCache {
	c := &LinkCache{
		items:    make(map[string]*cacheItem),
		ttl:      ttl,
		maxItems: maxItems,
	}

	// 启动后台清理
	go c.cleanup()

	return c
}

// Get 获取缓存
func (c *LinkCache) Get(key string) (*DirectLinkInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.link, true
}

// Set 设置缓存
func (c *LinkCache) Set(key string, link *DirectLinkInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查容量
	if len(c.items) >= c.maxItems {
		c.evictOldest()
	}

	expiresAt := link.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(c.ttl)
	}

	c.items[key] = &cacheItem{
		link:      link,
		expiresAt: expiresAt,
	}
}

// Delete 删除缓存
func (c *LinkCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Clear 清空缓存
func (c *LinkCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheItem)
}

// Size 获取缓存大小
func (c *LinkCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

// evictOldest 淘汰最旧的缓存
func (c *LinkCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for k, v := range c.items {
		if oldestKey == "" || v.expiresAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.expiresAt
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

// cleanup 定期清理过期缓存
func (c *LinkCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.expiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}
