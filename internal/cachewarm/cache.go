// Package cachewarm 提供智能缓存预热功能
// 基于访问模式预测和预加载热点数据
// 支持 LRU/LFU/ARC 策略 / 定时预热 / 智能预测 / 缓存统计
package cachewarm

import (
	"container/list"
	"sync"
	"time"
)

// CachePolicy 缓存策略.
type CachePolicy string

const (
	PolicyLRU  CachePolicy = "lru"  // 最近最少使用
	PolicyLFU  CachePolicy = "lfu"  // 最不经常使用
	PolicyARC  CachePolicy = "arc"  // 自适应替换缓存
	PolicyFIFO CachePolicy = "fifo" // 先进先出
)

// WarmStrategy 预热策略.
type WarmStrategy string

const (
	WarmScheduled  WarmStrategy = "scheduled"  // 定时预热
	WarmPredictive WarmStrategy = "predictive" // 智能预测预热
	WarmOnDemand   WarmStrategy = "ondemand"   // 按需预热
)

// CacheEntry 缓存条目.
type CacheEntry struct {
	Key       string        `json:"key"`
	Value     interface{}   `json:"value"`
	Size      int64         `json:"size"`
	HitCount  int64         `json:"hitCount"`
	CreatedAt time.Time     `json:"createdAt"`
	LastHit   time.Time     `json:"lastHit"`
	Frequency int           `json:"frequency"` // 访问频率
	TTL       time.Duration `json:"ttl"`
	ExpiresAt time.Time     `json:"expiresAt"`
}

// WarmTask 预热任务.
type WarmTask struct {
	ID       string       `json:"id"`
	Keys     []string     `json:"keys"`
	Strategy WarmStrategy `json:"strategy"`
	Schedule string       `json:"schedule"` // cron表达式
	Status   string       `json:"status"`
	LastRun  time.Time    `json:"lastRun"`
	NextRun  time.Time    `json:"nextRun"`
	Warmed   int          `json:"warmed"`
	Failed   int          `json:"failed"`
}

// AccessPattern 访问模式.
type AccessPattern struct {
	Key        string    `json:"key"`
	AccessTime time.Time `json:"accessTime"`
	Hour       int       `json:"hour"`
	DayOfWeek  int       `json:"dayOfWeek"`
}

// CacheStats 缓存统计.
type CacheStats struct {
	mu         sync.RWMutex
	Hits       int64   `json:"hits"`
	Misses     int64   `json:"misses"`
	HitRate    float64 `json:"hitRate"`
	TotalSize  int64   `json:"totalSize"`
	EntryCount int     `json:"entryCount"`
	MaxSize    int64   `json:"maxSize"`
	Evictions  int64   `json:"evictions"`
	WarmHits   int64   `json:"warmHits"`   // 预热命中
	WarmMisses int64   `json:"warmMisses"` // 预热未命中
}

// SmartCache 智能缓存.
type SmartCache struct {
	mu          sync.RWMutex
	policy      CachePolicy
	maxSize     int64
	currentSize int64
	entries     map[string]*list.Element
	lruList     *list.List
	freqMap     map[int][]string // frequency -> keys
	accessLog   []AccessPattern
	stats       *CacheStats
	warmTasks   map[string]*WarmTask
}

// NewSmartCache 创建智能缓存.
func NewSmartCache(policy CachePolicy, maxSize int64) *SmartCache {
	return &SmartCache{
		policy:    policy,
		maxSize:   maxSize,
		entries:   make(map[string]*list.Element),
		lruList:   list.New(),
		freqMap:   make(map[int][]string),
		accessLog: make([]AccessPattern, 0, 1000),
		stats:     &CacheStats{MaxSize: maxSize},
		warmTasks: make(map[string]*WarmTask),
	}
}

// Get 获取缓存.
func (c *SmartCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[key]
	if !ok {
		c.stats.mu.Lock()
		c.stats.Misses++
		c.updateHitRate()
		c.stats.mu.Unlock()
		return nil, false
	}

	entry := elem.Value.(*CacheEntry)

	// 检查过期
	if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
		c.removeElement(elem)
		c.stats.mu.Lock()
		c.stats.Misses++
		c.updateHitRate()
		c.stats.mu.Unlock()
		return nil, false
	}

	// 更新访问信息
	entry.HitCount++
	entry.LastHit = time.Now()
	entry.Frequency++

	// 记录访问模式
	c.accessLog = append(c.accessLog, AccessPattern{
		Key:        key,
		AccessTime: time.Now(),
		Hour:       time.Now().Hour(),
		DayOfWeek:  int(time.Now().Weekday()),
	})

	// LRU: 移到前面
	if c.policy == PolicyLRU || c.policy == PolicyARC {
		c.lruList.MoveToFront(elem)
	}

	c.stats.mu.Lock()
	c.stats.Hits++
	c.updateHitRate()
	c.stats.mu.Unlock()

	return entry.Value, true
}

// Set 设置缓存.
func (c *SmartCache) Set(key string, value interface{}, size int64, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 已存在则更新
	if elem, ok := c.entries[key]; ok {
		entry := elem.Value.(*CacheEntry)
		c.currentSize -= entry.Size
		entry.Value = value
		entry.Size = size
		entry.TTL = ttl
		if ttl > 0 {
			entry.ExpiresAt = time.Now().Add(ttl)
		}
		c.currentSize += size
		c.lruList.MoveToFront(elem)
		return
	}

	// 淘汰直到有空间
	for c.currentSize+size > c.maxSize && c.lruList.Len() > 0 {
		c.evict()
	}

	// 创建新条目
	entry := &CacheEntry{
		Key:       key,
		Value:     value,
		Size:      size,
		CreatedAt: time.Now(),
		LastHit:   time.Now(),
		Frequency: 1,
		TTL:       ttl,
	}
	if ttl > 0 {
		entry.ExpiresAt = time.Now().Add(ttl)
	}

	elem := c.lruList.PushFront(entry)
	c.entries[key] = elem
	c.currentSize += size

	c.stats.mu.Lock()
	c.stats.EntryCount = len(c.entries)
	c.stats.TotalSize = c.currentSize
	c.stats.mu.Unlock()
}

// evict 淘汰缓存条目.
func (c *SmartCache) evict() {
	switch c.policy {
	case PolicyLRU, PolicyFIFO:
		c.evictLRU()
	case PolicyLFU:
		c.evictLFU()
	case PolicyARC:
		c.evictLRU()
	}
}

// evictLRU 淘汰最久未使用.
func (c *SmartCache) evictLRU() {
	elem := c.lruList.Back()
	if elem == nil {
		return
	}
	c.removeElement(elem)

	c.stats.mu.Lock()
	c.stats.Evictions++
	c.stats.mu.Unlock()
}

// evictLFU 淘汰最不频繁使用.
func (c *SmartCache) evictLFU() {
	var minFreq int = 1<<31 - 1
	var minKey string

	for key, elem := range c.entries {
		entry := elem.Value.(*CacheEntry)
		if entry.Frequency < minFreq {
			minFreq = entry.Frequency
			minKey = key
		}
	}

	if minKey != "" {
		if elem, ok := c.entries[minKey]; ok {
			c.removeElement(elem)
		}
	}

	c.stats.mu.Lock()
	c.stats.Evictions++
	c.stats.mu.Unlock()
}

// removeElement 移除元素.
func (c *SmartCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*CacheEntry)
	c.lruList.Remove(elem)
	delete(c.entries, entry.Key)
	c.currentSize -= entry.Size

	c.stats.mu.Lock()
	c.stats.EntryCount = len(c.entries)
	c.stats.TotalSize = c.currentSize
	c.stats.mu.Unlock()
}

// Delete 删除缓存.
func (c *SmartCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[key]
	if !ok {
		return false
	}
	c.removeElement(elem)
	return true
}

// CreateWarmTask 创建预热任务.
func (c *SmartCache) CreateWarmTask(id string, keys []string, strategy WarmStrategy) *WarmTask {
	c.mu.Lock()
	defer c.mu.Unlock()

	task := &WarmTask{
		ID:       id,
		Keys:     keys,
		Strategy: strategy,
		Status:   "pending",
	}
	c.warmTasks[id] = task
	return task
}

// ExecuteWarmTask 执行预热任务.
func (c *SmartCache) ExecuteWarmTask(id string, loader func(string) (interface{}, int64, error)) error {
	c.mu.RLock()
	task, ok := c.warmTasks[id]
	c.mu.RUnlock()
	if !ok {
		return nil
	}

	task.Status = "running"
	task.LastRun = time.Now()

	for _, key := range task.Keys {
		value, size, err := loader(key)
		if err != nil {
			task.Failed++
			continue
		}
		c.Set(key, value, size, 0)
		task.Warmed++
	}

	task.Status = "completed"
	return nil
}

// PredictHotKeys 预测热点Key.
func (c *SmartCache) PredictHotKeys(limit int) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	type keyScore struct {
		key   string
		score float64
	}

	var scores []keyScore
	for _, elem := range c.entries {
		entry := elem.Value.(*CacheEntry)
		// 基于频率和最近访问计算热度
		recency := time.Since(entry.LastHit).Seconds()
		freq := float64(entry.Frequency)
		score := freq / (1 + recency/3600) // 频率/时间衰减
		scores = append(scores, keyScore{entry.Key, score})
	}

	// 简单排序
	for i := 0; i < len(scores); i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[i].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	result := make([]string, 0, limit)
	for i := 0; i < len(scores) && i < limit; i++ {
		result = append(result, scores[i].key)
	}
	return result
}

// GetStats 获取缓存统计.
func (c *SmartCache) GetStats() *CacheStats {
	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()
	return &CacheStats{
		Hits:       c.stats.Hits,
		Misses:     c.stats.Misses,
		HitRate:    c.stats.HitRate,
		TotalSize:  c.stats.TotalSize,
		EntryCount: c.stats.EntryCount,
		MaxSize:    c.stats.MaxSize,
		Evictions:  c.stats.Evictions,
		WarmHits:   c.stats.WarmHits,
		WarmMisses: c.stats.WarmMisses,
	}
}

// updateHitRate 更新命中率.
func (c *SmartCache) updateHitRate() {
	total := c.stats.Hits + c.stats.Misses
	if total > 0 {
		c.stats.HitRate = float64(c.stats.Hits) / float64(total)
	}
}
