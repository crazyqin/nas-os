package filecache

import (
	"container/list"
	"sync"
	"time"
)

// lruCache LRU 缓存实现
type lruCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*list.Element
	order    *list.List // 最近访问的在前面
}

// lruItem LRU 列表元素
type lruItem struct {
	key   string
	entry *CacheEntry
}

// newLRUCache 创建 LRU 缓存
func newLRUCache(capacity int) *lruCache {
	return &lruCache{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

// Get 获取缓存条目
func (c *lruCache) Get(key string) (*CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}

	// 移动到最前面
	c.order.MoveToFront(elem)
	item := elem.Value.(*lruItem)
	item.entry.LastAccess = time.Now()
	item.entry.HitCount++

	return item.entry, true
}

// Put 放入缓存条目
func (c *lruCache) Put(key string, entry *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，更新并移到前面
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		elem.Value.(*lruItem).entry = entry
		return
	}

	// 检查容量
	if c.order.Len() >= c.capacity {
		c.evict()
	}

	// 添加新条目
	item := &lruItem{key: key, entry: entry}
	elem := c.order.PushFront(item)
	c.items[key] = elem
}

// Delete 删除缓存条目
func (c *lruCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return false
	}

	c.removeElement(elem)
	return true
}

// evict 淘汰最久未访问的条目
func (c *lruCache) evict() *CacheEntry {
	elem := c.order.Back()
	if elem == nil {
		return nil
	}

	item := elem.Value.(*lruItem)
	c.removeElement(elem)
	return item.entry
}

// removeElement 移除元素
func (c *lruCache) removeElement(elem *list.Element) {
	item := elem.Value.(*lruItem)
	c.order.Remove(elem)
	delete(c.items, item.key)
}

// Len 返回缓存条目数
func (c *lruCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

// Contains 检查是否包含键
func (c *lruCache) Contains(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.items[key]
	return ok
}

// Peek 查看但不更新访问顺序
func (c *lruCache) Peek(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}

	item := elem.Value.(*lruItem)
	return item.entry, true
}

// Keys 返回所有键（按访问顺序）
func (c *lruCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, c.order.Len())
	for elem := c.order.Front(); elem != nil; elem = elem.Next() {
		keys = append(keys, elem.Value.(*lruItem).key)
	}
	return keys
}

// lfuCache LFU 缓存实现
type lfuCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*lfuItem
	freqList *list.List // 按频率排序的链表
	minFreq  int        // 最小频率
}

// lfuItem LFU 缓存条目
type lfuItem struct {
	key      string
	entry    *CacheEntry
	freq     int
	freqElem *list.Element // 在频率链表中的位置
}

// freqNode 频率节点
type freqNode struct {
	freq  int
	items *list.List // 该频率下的所有条目
}

// newLFUCache 创建 LFU 缓存
func newLFUCache(capacity int) *lfuCache {
	c := &lfuCache{
		capacity: capacity,
		items:    make(map[string]*lfuItem),
		freqList: list.New(),
		minFreq:  0,
	}
	return c
}

// Get 获取缓存条目
func (c *lfuCache) Get(key string) (*CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		return nil, false
	}

	// 增加频率
	c.incrementFreq(item)
	item.entry.LastAccess = time.Now()
	item.entry.HitCount++

	return item.entry, true
}

// Put 放入缓存条目
func (c *lfuCache) Put(key string, entry *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，更新
	if item, ok := c.items[key]; ok {
		item.entry = entry
		c.incrementFreq(item)
		return
	}

	// 检查容量
	if len(c.items) >= c.capacity {
		c.evict()
	}

	// 创建新条目
	item := &lfuItem{
		key:   key,
		entry: entry,
		freq:  1,
	}

	// 确保频率 1 的节点存在
	freqNode := c.getOrCreateFreqNode(1)
	item.freqElem = freqNode.items.PushBack(item)

	c.items[key] = item
	c.minFreq = 1
}

// Delete 删除缓存条目
func (c *lfuCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		return false
	}

	c.removeItem(item)
	return true
}

// incrementFreq 增加条目频率
func (c *lfuCache) incrementFreq(item *lfuItem) {
	oldFreq := item.freq
	newFreq := oldFreq + 1
	item.freq = newFreq

	// 从旧频率节点移除
	oldFreqNode := c.getFreqNode(oldFreq)
	if oldFreqNode != nil {
		oldFreqNode.items.Remove(item.freqElem)

		// 如果旧频率节点为空，移除它
		if oldFreqNode.items.Len() == 0 {
			c.removeFreqNode(oldFreqNode)
			// 更新最小频率
			if c.minFreq == oldFreq {
				c.minFreq = newFreq
			}
		}
	}

	// 添加到新频率节点
	newFreqNode := c.getOrCreateFreqNode(newFreq)
	item.freqElem = newFreqNode.items.PushBack(item)
}

// evict 淘汰频率最低的条目
func (c *lfuCache) evict() *CacheEntry {
	freqNode := c.getFreqNode(c.minFreq)
	if freqNode == nil || freqNode.items.Len() == 0 {
		return nil
	}

	// 淘汰该频率下最早添加的条目
	elem := freqNode.items.Front()
	item := elem.Value.(*lfuItem)

	c.removeItem(item)
	return item.entry
}

// removeItem 移除条目
func (c *lfuCache) removeItem(item *lfuItem) {
	freqNode := c.getFreqNode(item.freq)
	if freqNode != nil {
		freqNode.items.Remove(item.freqElem)
		if freqNode.items.Len() == 0 {
			c.removeFreqNode(freqNode)
		}
	}
	delete(c.items, item.key)
}

// getFreqNode 获取频率节点
func (c *lfuCache) getFreqNode(freq int) *freqNode {
	for elem := c.freqList.Front(); elem != nil; elem = elem.Next() {
		fn := elem.Value.(*freqNode)
		if fn.freq == freq {
			return fn
		}
	}
	return nil
}

// getOrCreateFreqNode 获取或创建频率节点
func (c *lfuCache) getOrCreateFreqNode(freq int) *freqNode {
	fn := c.getFreqNode(freq)
	if fn != nil {
		return fn
	}

	fn = &freqNode{
		freq:  freq,
		items: list.New(),
	}

	// 按频率顺序插入
	inserted := false
	for elem := c.freqList.Front(); elem != nil; elem = elem.Next() {
		existing := elem.Value.(*freqNode)
		if existing.freq > freq {
			c.freqList.InsertBefore(fn, elem)
			inserted = true
			break
		}
	}
	if !inserted {
		c.freqList.PushBack(fn)
	}

	return fn
}

// removeFreqNode 移除频率节点
func (c *lfuCache) removeFreqNode(fn *freqNode) {
	for elem := c.freqList.Front(); elem != nil; elem = elem.Next() {
		if elem.Value.(*freqNode) == fn {
			c.freqList.Remove(elem)
			return
		}
	}
}

// Len 返回缓存条目数
func (c *lfuCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Contains 检查是否包含键
func (c *lfuCache) Contains(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.items[key]
	return ok
}

// Peek 查看但不更新频率
func (c *lfuCache) Peek(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		return nil, false
	}

	return item.entry, true
}

// Keys 返回所有键
func (c *lfuCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for key := range c.items {
		keys = append(keys, key)
	}
	return keys
}

// hybridCache LRU/LFU 混合缓存
type hybridCache struct {
	mu        sync.RWMutex
	capacity  int
	lruWeight float64
	lfuWeight float64
	items     map[string]*hybridItem
	order     *list.List // 按混合分数排序
}

// hybridItem 混合缓存条目
type hybridItem struct {
	key      string
	entry    *CacheEntry
	lruScore float64 // LRU 分数
	lfuScore float64 // LFU 分数
	score    float64 // 混合分数
	elem     *list.Element
}

// newHybridCache 创建混合缓存
func newHybridCache(capacity int, lruWeight, lfuWeight float64) *hybridCache {
	return &hybridCache{
		capacity:  capacity,
		lruWeight: lruWeight,
		lfuWeight: lfuWeight,
		items:     make(map[string]*hybridItem),
		order:     list.New(),
	}
}

// Get 获取缓存条目
func (c *hybridCache) Get(key string) (*CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		return nil, false
	}

	// 更新分数
	item.entry.LastAccess = time.Now()
	item.entry.HitCount++
	c.updateScore(item)

	// 移动到正确位置
	c.reorder(item)

	return item.entry, true
}

// Put 放入缓存条目
func (c *hybridCache) Put(key string, entry *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，更新
	if item, ok := c.items[key]; ok {
		item.entry = entry
		c.updateScore(item)
		c.reorder(item)
		return
	}

	// 检查容量
	if len(c.items) >= c.capacity {
		c.evict()
	}

	// 创建新条目
	item := &hybridItem{
		key:   key,
		entry: entry,
	}
	c.updateScore(item)
	item.elem = c.order.PushBack(item)
	c.items[key] = item

	// 调整位置
	c.reorder(item)
}

// Delete 删除缓存条目
func (c *hybridCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		return false
	}

	c.order.Remove(item.elem)
	delete(c.items, key)
	return true
}

// evict 淘汰分数最低的条目
func (c *hybridCache) evict() *CacheEntry {
	elem := c.order.Front()
	if elem == nil {
		return nil
	}

	item := elem.Value.(*hybridItem)
	c.order.Remove(elem)
	delete(c.items, item.key)

	return item.entry
}

// updateScore 更新混合分数
func (c *hybridCache) updateScore(item *hybridItem) {
	now := time.Now()

	// LRU 分数：基于最近访问时间（越近越高）
	timeSinceAccess := now.Sub(item.entry.LastAccess).Seconds()
	if timeSinceAccess < 1 {
		timeSinceAccess = 1
	}
	item.lruScore = 1.0 / timeSinceAccess

	// LFU 分数：基于访问频率
	elapsed := now.Sub(item.entry.CreatedAt).Hours()
	if elapsed < 0.01 {
		elapsed = 0.01
	}
	item.lfuScore = float64(item.entry.HitCount) / elapsed

	// 混合分数
	item.score = c.lruWeight*item.lruScore + c.lfuWeight*item.lfuScore
	item.entry.score = item.score
}

// reorder 重新排序
func (c *hybridCache) reorder(item *hybridItem) {
	// 从当前位置移除
	c.order.Remove(item.elem)

	// 找到正确位置插入（按分数降序）
	inserted := false
	for elem := c.order.Front(); elem != nil; elem = elem.Next() {
		existing := elem.Value.(*hybridItem)
		if item.score > existing.score {
			item.elem = c.order.InsertBefore(item, elem)
			inserted = true
			break
		}
	}

	if !inserted {
		item.elem = c.order.PushBack(item)
	}
}

// Len 返回缓存条目数
func (c *hybridCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Contains 检查是否包含键
func (c *hybridCache) Contains(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.items[key]
	return ok
}

// Peek 查看但不更新分数
func (c *hybridCache) Peek(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		return nil, false
	}

	return item.entry, true
}

// Keys 返回所有键（按分数排序）
func (c *hybridCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for elem := c.order.Front(); elem != nil; elem = elem.Next() {
		keys = append(keys, elem.Value.(*hybridItem).key)
	}
	return keys
}
