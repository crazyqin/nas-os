// Package scraper provides metadata caching functionality.
package scraper

import (
	"sync"
	"time"
)

// MetadataCache caches scraped metadata.
type MetadataCache struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem
	ttl     time.Duration
	maxSize int
}

type cacheItem struct {
	data      interface{}
	expiresAt time.Time
}

// NewMetadataCache creates a new metadata cache.
func NewMetadataCache(ttl time.Duration) *MetadataCache {
	return &MetadataCache{
		items:   make(map[string]*cacheItem),
		ttl:     ttl,
		maxSize: 1000,
	}
}

// Get retrieves cached metadata list.
func (c *MetadataCache) Get(key string) ([]MediaMetadata, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}

	if data, ok := item.data.([]MediaMetadata); ok {
		return data, true
	}
	return nil, false
}

// GetSingle retrieves single cached metadata.
func (c *MetadataCache) GetSingle(key string) (*MediaMetadata, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}

	if data, ok := item.data.(*MediaMetadata); ok {
		return data, true
	}
	return nil, false
}

// Set caches metadata list.
func (c *MetadataCache) Set(key string, data []MediaMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.pruneIfNeeded()
	c.items[key] = &cacheItem{
		data:      data,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// SetSingle caches single metadata.
func (c *MetadataCache) SetSingle(key string, data *MediaMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.pruneIfNeeded()
	c.items[key] = &cacheItem{
		data:      data,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes cached item.
func (c *MetadataCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear clears all cached items.
func (c *MetadataCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
}

// pruneIfNeeded removes expired items if cache is too large.
func (c *MetadataCache) pruneIfNeeded() {
	if len(c.items) < c.maxSize {
		return
	}

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, key)
		}
	}

	// If still too large, remove oldest items
	if len(c.items) >= c.maxSize {
		// Simple approach: clear half the cache
		count := 0
		for key := range c.items {
			if count >= c.maxSize/2 {
				break
			}
			delete(c.items, key)
			count++
		}
	}
}

// Stats returns cache statistics.
func (c *MetadataCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	valid := 0
	expired := 0
	now := time.Now()

	for _, item := range c.items {
		if now.After(item.expiresAt) {
			expired++
		} else {
			valid++
		}
	}

	return CacheStats{
		TotalItems:   len(c.items),
		ValidItems:   valid,
		ExpiredItems: expired,
		MaxSize:      c.maxSize,
	}
}

// CacheStats represents cache statistics.
type CacheStats struct {
	TotalItems   int `json:"total_items"`
	ValidItems   int `json:"valid_items"`
	ExpiredItems int `json:"expired_items"`
	MaxSize      int `json:"max_size"`
}
