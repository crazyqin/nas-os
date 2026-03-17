package cache

import (
	"container/list"
	"sync"
	"time"
)

// LRUCache implements a thread-safe LRU cache with TTL support
type LRUCache struct {
	capacity int
	cache    map[interface{}]*list.Element
	lru      *list.List
	mu       sync.RWMutex
	ttl      time.Duration
}

type cacheItem struct {
	key        interface{}
	value      interface{}
	expiry     time.Time
	hitCount   int64
	lastAccess time.Time
}

// NewLRUCache creates a new LRU cache with given capacity and TTL
func NewLRUCache(capacity int, ttl time.Duration) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[interface{}]*list.Element),
		lru:      list.New(),
		ttl:      ttl,
	}
}

// Get retrieves a value from the cache
func (c *LRUCache) Get(key interface{}) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.cache[key]
	if !ok {
		return nil, false
	}

	item, ok := elem.Value.(*cacheItem)
	if !ok {
		c.removeElement(elem)
		return nil, false
	}

	// Check if expired
	if time.Now().After(item.expiry) {
		c.removeElement(elem)
		return nil, false
	}

	// Move to front (most recently used)
	c.lru.MoveToFront(elem)
	item.hitCount++
	item.lastAccess = time.Now()

	return item.value, true
}

// Set stores a value in the cache
func (c *LRUCache) Set(key, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiry := now.Add(c.ttl)

	// Check if key already exists
	if elem, ok := c.cache[key]; ok {
		item, ok := elem.Value.(*cacheItem)
		if !ok {
			c.removeElement(elem)
			return
		}
		item.value = value
		item.expiry = expiry
		item.hitCount++
		item.lastAccess = now
		c.lru.MoveToFront(elem)
		return
	}

	// Evict if at capacity
	if c.lru.Len() >= c.capacity {
		c.evictOldest()
	}

	// Add new item
	item := &cacheItem{
		key:        key,
		value:      value,
		expiry:     expiry,
		hitCount:   0,
		lastAccess: now,
	}
	elem := c.lru.PushFront(item)
	c.cache[key] = elem
}

// Delete removes a key from the cache
func (c *LRUCache) Delete(key interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[key]; ok {
		c.removeElement(elem)
	}
}

// Clear removes all items from the cache
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[interface{}]*list.Element)
	c.lru = list.New()
}

// Len returns the current number of items in the cache
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Len()
}

// Capacity returns the cache capacity
func (c *LRUCache) Capacity() int {
	return c.capacity
}

// removeElement removes an element from both the list and map
func (c *LRUCache) removeElement(elem *list.Element) {
	item, ok := elem.Value.(*cacheItem)
	if !ok {
		return
	}
	delete(c.cache, item.key)
	c.lru.Remove(elem)
}

// evictOldest removes the least recently used item
func (c *LRUCache) evictOldest() {
	elem := c.lru.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

// Cleanup removes all expired items
func (c *LRUCache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0

	for elem := c.lru.Front(); elem != nil; {
		next := elem.Next()
		item, ok := elem.Value.(*cacheItem)
		if ok && now.After(item.expiry) {
			c.removeElement(elem)
			removed++
		}
		elem = next
	}

	return removed
}
