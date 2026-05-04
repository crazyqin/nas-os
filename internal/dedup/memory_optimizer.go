// Package dedup 数据去重 - 内存优化器
// 通过 bloom filter、LRU 缓存和分页加载降低内存占用
package dedup

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"runtime"
	"sync"
	"time"
)

// ========== Bloom Filter ==========

// BloomFilter 布隆过滤器，用于快速判断 hash 是否可能存在。
// 误报（false positive）可接受，漏报（false negative）不允许。
type BloomFilter struct {
	mu        sync.RWMutex
	bits      []bool     // 位数组
	size      uint64     // 位数组大小
	hashCount int        // 哈希函数数量
	count     int64      // 已插入元素数量
	seeds     []uint64   // 各哈希函数的种子
}

// NewBloomFilter 创建布隆过滤器。
// expectedItems: 预期元素数量
// fpRate: 期望误报率（如 0.01 表示 1%）
func NewBloomFilter(expectedItems int, fpRate float64) *BloomFilter {
	if expectedItems <= 0 {
		expectedItems = 10000
	}
	if fpRate <= 0 || fpRate >= 1 {
		fpRate = 0.01
	}

	// 计算最优位数组大小: m = -n*ln(p) / (ln2)^2
	ln2sq := 0.4804530139182014 // (ln2)^2
	size := uint64(float64(expectedItems) * (-1.0 * ln(fpRate)) / ln2sq)
	if size < 64 {
		size = 64
	}

	// 计算最优哈希函数数量: k = (m/n)*ln2
	hashCount := int(float64(size) / float64(expectedItems) * 0.6931471805599453)
	if hashCount < 1 {
		hashCount = 1
	}
	if hashCount > 16 {
		hashCount = 16
	}

	// 生成种子
	seeds := make([]uint64, hashCount)
	for i := range seeds {
		seeds[i] = uint64(i*1327217885 + 1054645833) // 简单的种子生成
	}

	return &BloomFilter{
		bits:      make([]bool, size),
		size:      size,
		hashCount: hashCount,
		seeds:     seeds,
	}
}

// ln 计算自然对数。
func ln(x float64) float64 {
	return math.Log(x)
}

// Insert 插入一个元素。
func (bf *BloomFilter) Insert(data string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	for i := 0; i < bf.hashCount; i++ {
		h := bf.hashValue(data, bf.seeds[i])
		bf.bits[h%bf.size] = true
	}
	bf.count++
}

// Contains 检查元素是否可能存在。
// 返回 true 表示可能存在（需进一步确认），false 表示一定不存在。
func (bf *BloomFilter) Contains(data string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	for i := 0; i < bf.hashCount; i++ {
		h := bf.hashValue(data, bf.seeds[i])
		if !bf.bits[h%bf.size] {
			return false
		}
	}
	return true
}

// Count 返回已插入元素数量。
func (bf *BloomFilter) Count() int64 {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return bf.count
}

// hashValue 使用 FNV-1a 计算带种子的哈希值。
func (bf *BloomFilter) hashValue(data string, seed uint64) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("%d:%s", seed, data)))
	return h.Sum64()
}

// Reset 重置布隆过滤器。
func (bf *BloomFilter) Reset() {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	for i := range bf.bits {
		bf.bits[i] = false
	}
	bf.count = 0
}

// ========== LRU 缓存 ==========

// lruEntry LRU 缓存条目。
type lruEntry struct {
	key       string
	value     *Chunk
	prev      *lruEntry
	next      *lruEntry
}

// LRUCache LRU（最近最少使用）缓存，用于管理 chunk 元数据的内存占用。
type LRUCache struct {
	mu       sync.Mutex
	capacity int
	size     int
	entries  map[string]*lruEntry // key -> entry
	head     *lruEntry            // 最近使用的（双向链表头）
	tail     *lruEntry            // 最久未使用的（双向链表尾）
	stats    LRUCacheStats
}

// LRUCacheStats LRU 缓存统计。
type LRUCacheStats struct {
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	Evictions int64 `json:"evictions"`
}

// NewLRUCache 创建 LRU 缓存。
func NewLRUCache(capacity int) *LRUCache {
	if capacity <= 0 {
		capacity = 10000
	}
	head := &lruEntry{}
	tail := &lruEntry{}
	head.next = tail
	tail.prev = head

	return &LRUCache{
		capacity: capacity,
		entries:  make(map[string]*lruEntry, capacity),
		head:     head,
		tail:     tail,
	}
}

// Get 获取缓存中的 chunk，同时将其标记为最近使用。
func (c *LRUCache) Get(hash string) (*Chunk, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[hash]
	if !exists {
		c.stats.Misses++
		return nil, false
	}

	// 移到链表头部（标记为最近使用）
	c.moveToFront(entry)
	c.stats.Hits++
	return entry.value, true
}

// Put 将 chunk 放入缓存。
func (c *LRUCache) Put(hash string, chunk *Chunk) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 已存在则更新
	if entry, exists := c.entries[hash]; exists {
		entry.value = chunk
		c.moveToFront(entry)
		return
	}

	// 容量满则淘汰最久未使用的
	if c.size >= c.capacity {
		c.evict()
	}

	// 插入新条目
	entry := &lruEntry{key: hash, value: chunk}
	c.entries[hash] = entry
	c.addToFront(entry)
	c.size++
}

// Remove 从缓存中移除指定条目。
func (c *LRUCache) Remove(hash string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[hash]
	if !exists {
		return false
	}

	c.removeEntry(entry)
	delete(c.entries, hash)
	c.size--
	return true
}

// Len 返回缓存中的条目数量。
func (c *LRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.size
}

// Stats 返回缓存统计。
func (c *LRUCache) Stats() LRUCacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

// Purge 清空缓存。
func (c *LRUCache) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*lruEntry, c.capacity)
	c.head.next = c.tail
	c.tail.prev = c.head
	c.size = 0
}

// Resize 调整缓存容量。若新容量小于当前大小，淘汰多余的条目。
func (c *LRUCache) Resize(newCapacity int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if newCapacity <= 0 {
		return
	}
	c.capacity = newCapacity
	for c.size > c.capacity {
		c.evict()
	}
}

// evict 淘汰最久未使用的条目（链表尾部）。
func (c *LRUCache) evict() {
	if c.size == 0 {
		return
	}
	victim := c.tail.prev
	c.removeEntry(victim)
	delete(c.entries, victim.key)
	c.size--
	c.stats.Evictions++
}

// moveToFront 将条目移到链表头部。
func (c *LRUCache) moveToFront(entry *lruEntry) {
	c.removeEntry(entry)
	c.addToFront(entry)
}

// addToFront 将条目添加到链表头部。
func (c *LRUCache) addToFront(entry *lruEntry) {
	entry.prev = c.head
	entry.next = c.head.next
	c.head.next.prev = entry
	c.head.next = entry
}

// removeEntry 从链表中移除条目。
func (c *LRUCache) removeEntry(entry *lruEntry) {
	entry.prev.next = entry.next
	entry.next.prev = entry.prev
}

// ========== 分页 Chunk 加载器 ==========

// PageIndex 分页索引。
type PageIndex struct {
	Page      int      `json:"page"`
	Hashes    []string `json:"hashes"` // 当前页的 hash 列表
	Total     int      `json:"total"`  // 总条目数
	TotalPage int      `json:"totalPage"`
	PageSize  int      `json:"pageSize"`
}

// PagedChunkLoader 分页加载器，避免一次性将所有 chunk 加载到内存。
type PagedChunkLoader struct {
	mu         sync.RWMutex
	indexPath  string        // 索引文件路径
	pageSize   int           // 每页条目数
	total      int           // 总条目数
	hashes     []string      // 全部 hash 列表（轻量，只存 hash 字符串）
	chunks     map[string]*Chunk // 已加载的 chunk（按需加载）
	loaded     map[int]bool  // 已加载的页
	maxPages   int           // 内存中最多保留的页数
	loadedPages []int        // 已加载页的顺序（用于 LRU 淘汰）
}

// NewPagedChunkLoader 创建分页加载器。
func NewPagedChunkLoader(indexPath string, pageSize int, maxPages int) *PagedChunkLoader {
	if pageSize <= 0 {
		pageSize = 1000
	}
	if maxPages <= 0 {
		maxPages = 10
	}
	return &PagedChunkLoader{
		indexPath: indexPath,
		pageSize:  pageSize,
		hashes:    make([]string, 0),
		chunks:    make(map[string]*Chunk),
		loaded:    make(map[int]bool),
		maxPages:  maxPages,
	}
}

// LoadIndex 从文件加载索引到内存（只加载 hash 列表，不加载完整 chunk 数据）。
func (pl *PagedChunkLoader) LoadIndex() error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	data, err := os.ReadFile(pl.indexPath)
	if err != nil {
		return fmt.Errorf("读取索引文件失败: %w", err)
	}

	var stored struct {
		Hashes []string        `json:"hashes"`
		Chunks map[string]*Chunk `json:"chunks"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return fmt.Errorf("解析索引文件失败: %w", err)
	}

	pl.hashes = stored.Hashes
	pl.total = len(pl.hashes)

	// 将存量 chunk 分页存储，但只保留前 maxPages 页在内存
	if stored.Chunks != nil {
		start := 0
		for page := 0; start < len(stored.Chunks) && page < pl.maxPages; page++ {
			end := start + pl.pageSize
			if end > len(stored.Hashes) {
				end = len(stored.Hashes)
			}
			for i := start; i < end; i++ {
				if h := stored.Hashes[i]; stored.Chunks[h] != nil {
					pl.chunks[h] = stored.Chunks[h]
				}
			}
			pl.loaded[page] = true
			pl.loadedPages = append(pl.loadedPages, page)
			start = end
		}
	}

	return nil
}

// Total 返回总条目数。
func (pl *PagedChunkLoader) Total() int {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	return pl.total
}

// PageCount 返回总页数。
func (pl *PagedChunkLoader) PageCount() int {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	if pl.total == 0 {
		return 0
	}
	return (pl.total + pl.pageSize - 1) / pl.pageSize
}

// GetPage 获取指定页的数据（按需加载）。
func (pl *PagedChunkLoader) GetPage(page int, index *ChunkIndex) (*PageIndex, error) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	totalPage := (pl.total + pl.pageSize - 1) / pl.pageSize
	if page < 0 || page >= totalPage {
		return nil, fmt.Errorf("页码超出范围: %d (总页数: %d)", page, totalPage)
	}

	start := page * pl.pageSize
	end := start + pl.pageSize
	if end > pl.total {
		end = pl.total
	}

	pageHashes := pl.hashes[start:end]

	// 按需加载当前页的 chunk 数据到内存
	if !pl.loaded[page] {
		// 内存页数满时，淘汰最旧的页
		for len(pl.loadedPages) >= pl.maxPages {
			evictPage := pl.loadedPages[0]
			pl.loadedPages = pl.loadedPages[1:]
			pl.unloadPage(evictPage)
		}

		// 从 ChunkIndex 加载
		if index != nil {
			index.mu.RLock()
			for _, h := range pageHashes {
				if chunk, ok := index.chunks[h]; ok {
					pl.chunks[h] = chunk
				}
			}
			index.mu.RUnlock()
		}

		pl.loaded[page] = true
		pl.loadedPages = append(pl.loadedPages, page)
	}

	return &PageIndex{
		Page:      page,
		Hashes:    pageHashes,
		Total:     pl.total,
		TotalPage: totalPage,
		PageSize:  pl.pageSize,
	}, nil
}

// GetChunk 获取指定 hash 的 chunk（优先从内存缓存中查找）。
func (pl *PagedChunkLoader) GetChunk(hash string, index *ChunkIndex) (*Chunk, bool) {
	pl.mu.RLock()
	if chunk, ok := pl.chunks[hash]; ok {
		pl.mu.RUnlock()
		return chunk, true
	}
	pl.mu.RUnlock()

	// 内存中没有，从 ChunkIndex 加载
	if index != nil {
		index.mu.RLock()
		chunk, ok := index.chunks[hash]
		index.mu.RUnlock()
		if ok {
			pl.mu.Lock()
			pl.chunks[hash] = chunk
			pl.mu.Unlock()
			return chunk, true
		}
	}

	return nil, false
}

// unloadPage 卸载指定页的 chunk 数据以释放内存。
func (pl *PagedChunkLoader) unloadPage(page int) {
	start := page * pl.pageSize
	end := start + pl.pageSize
	if end > pl.total {
		end = pl.total
	}

	for i := start; i < end; i++ {
		if i < len(pl.hashes) {
			delete(pl.chunks, pl.hashes[i])
		}
	}
	delete(pl.loaded, page)
}

// ========== 内存监控 ==========

// MemoryStats 内存统计。
type MemoryStats struct {
	AllocMB      float64 `json:"allocMB"`      // 当前分配的内存 (MB)
	TotalAllocMB float64 `json:"totalAllocMB"`  // 累计分配的内存 (MB)
	SysMB        float64 `json:"sysMB"`         // 从系统获取的内存 (MB)
	NumGC        uint32  `json:"numGC"`         // GC 次数
	GCPauseMs    float64 `json:"gcPauseMs"`     // 上次 GC 暂停时间 (ms)
	HeapInuseMB  float64 `json:"heapInuseMB"`   // 堆内存使用 (MB)
	HeapIdleMB   float64 `json:"heapIdleMB"`    // 堆内存空闲 (MB)
}

// getMemoryStats 获取当前内存统计。
func getMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return MemoryStats{
		AllocMB:      float64(m.Alloc) / 1024 / 1024,
		TotalAllocMB: float64(m.TotalAlloc) / 1024 / 1024,
		SysMB:        float64(m.Sys) / 1024 / 1024,
		NumGC:        m.NumGC,
		GCPauseMs:    float64(m.PauseNs[(m.NumGC+255)%256]) / 1e6,
		HeapInuseMB:  float64(m.HeapInuse) / 1024 / 1024,
		HeapIdleMB:   float64(m.HeapIdle) / 1024 / 1024,
	}
}

// ========== 内存优化器 ==========

// MemoryOptimizerConfig 内存优化器配置。
type MemoryOptimizerConfig struct {
	// Bloom Filter 配置
	BloomExpectedItems int     `json:"bloomExpectedItems"` // 预期元素数量
	BloomFPRate        float64 `json:"bloomFPRate"`        // 期望误报率

	// LRU 缓存配置
	LRUCapacity int `json:"lruCapacity"` // LRU 缓存容量

	// 分页加载配置
	PageSize int `json:"pageSize"` // 每页条目数
	MaxPages int `json:"maxPages"` // 内存中最多保留的页数

	// 内存阈值配置
	MemoryThresholdMB   int `json:"memoryThresholdMB"`   // 触发优化的内存阈值 (MB)
	GCThresholdPercent  int `json:"gcThresholdPercent"`   // 触发 GC 的堆使用百分比
	CacheEvictPercent   int `json:"cacheEvictPercent"`    // 触发缓存淘汰时淘汰的百分比

	// 监控配置
	MonitorInterval time.Duration `json:"monitorInterval"` // 内存监控间隔
}

// DefaultMemoryOptimizerConfig 默认内存优化器配置。
func DefaultMemoryOptimizerConfig() *MemoryOptimizerConfig {
	return &MemoryOptimizerConfig{
		BloomExpectedItems:  100000,
		BloomFPRate:         0.01,
		LRUCapacity:         10000,
		PageSize:            1000,
		MaxPages:            10,
		MemoryThresholdMB:   512,
		GCThresholdPercent:  80,
		CacheEvictPercent:   30,
		MonitorInterval:     30 * time.Second,
	}
}

// MemoryOptimizer 内存优化器，整合 bloom filter、LRU 缓存和分页加载。
type MemoryOptimizer struct {
	mu          sync.RWMutex
	config      *MemoryOptimizerConfig
	bloom       *BloomFilter
	lruCache    *LRUCache
	pagedLoader *PagedChunkLoader
	index       *ChunkIndex // 引用外部 ChunkIndex

	// 状态
	running   bool
	stopCh    chan struct{}
	stats     OptimizerStats
	onEvict   func(hash string, chunk *Chunk) // 淘汰回调
}

// OptimizerStats 优化器统计。
type OptimizerStats struct {
	mu                sync.RWMutex
	TotalLookups      int64        `json:"totalLookups"`
	BloomMisses       int64        `json:"bloomMisses"`       // bloom filter 快速拒绝次数
	CacheHits         int64        `json:"cacheHits"`         // LRU 缓存命中次数
	CacheMisses       int64        `json:"cacheMisses"`       // LRU 缓存未命中次数
	GCTriggers        int64        `json:"gcTriggers"`        // GC 触发次数
	EvictTriggers     int64        `json:"evictTriggers"`     // 缓存淘汰触发次数
	LastMemoryStats   MemoryStats  `json:"lastMemoryStats"`
	LastGCTime        time.Time    `json:"lastGCTime"`
	LastEvictTime     time.Time    `json:"lastEvictTime"`
}

// GetSnapshot 获取统计快照。
func (os *OptimizerStats) GetSnapshot() OptimizerSnapshot {
	os.mu.RLock()
	defer os.mu.RUnlock()
	return OptimizerSnapshot{
		TotalLookups:    os.TotalLookups,
		BloomMisses:     os.BloomMisses,
		CacheHits:       os.CacheHits,
		CacheMisses:     os.CacheMisses,
		GCTriggers:      os.GCTriggers,
		EvictTriggers:   os.EvictTriggers,
		LastMemoryStats: os.LastMemoryStats,
		LastGCTime:      os.LastGCTime,
		LastEvictTime:   os.LastEvictTime,
	}
}

// OptimizerSnapshot 优化器统计快照（不含锁）。
type OptimizerSnapshot struct {
	TotalLookups    int64       `json:"totalLookups"`
	BloomMisses     int64       `json:"bloomMisses"`
	CacheHits       int64       `json:"cacheHits"`
	CacheMisses     int64       `json:"cacheMisses"`
	GCTriggers      int64       `json:"gcTriggers"`
	EvictTriggers   int64       `json:"evictTriggers"`
	LastMemoryStats MemoryStats `json:"lastMemoryStats"`
	LastGCTime      time.Time   `json:"lastGCTime"`
	LastEvictTime   time.Time   `json:"lastEvictTime"`
}

// NewMemoryOptimizer 创建内存优化器。
func NewMemoryOptimizer(config *MemoryOptimizerConfig, index *ChunkIndex) *MemoryOptimizer {
	if config == nil {
		config = DefaultMemoryOptimizerConfig()
	}

	return &MemoryOptimizer{
		config:      config,
		bloom:       NewBloomFilter(config.BloomExpectedItems, config.BloomFPRate),
		lruCache:    NewLRUCache(config.LRUCapacity),
		pagedLoader: NewPagedChunkLoader("", config.PageSize, config.MaxPages),
		index:       index,
		stopCh:      make(chan struct{}),
	}
}

// NewMemoryOptimizerWithIndexFile 创建带索引文件的内存优化器。
func NewMemoryOptimizerWithIndexFile(config *MemoryOptimizerConfig, index *ChunkIndex, indexPath string) *MemoryOptimizer {
	opt := NewMemoryOptimizer(config, index)
	opt.pagedLoader = NewPagedChunkLoader(indexPath, config.PageSize, config.MaxPages)
	return opt
}

// Start 启动内存监控后台协程。
func (mo *MemoryOptimizer) Start() {
	mo.mu.Lock()
	if mo.running {
		mo.mu.Unlock()
		return
	}
	mo.running = true
	mo.stopCh = make(chan struct{})
	mo.mu.Unlock()

	go mo.monitorLoop()
}

// Stop 停止内存监控。
func (mo *MemoryOptimizer) Stop() {
	mo.mu.Lock()
	defer mo.mu.Unlock()
	if mo.running {
		close(mo.stopCh)
		mo.running = false
	}
}

// InitBloomFromIndex 从 ChunkIndex 初始化布隆过滤器。
func (mo *MemoryOptimizer) InitBloomFromIndex() int {
	if mo.index == nil {
		return 0
	}

	mo.index.mu.RLock()
	count := 0
	for hash := range mo.index.chunks {
		mo.bloom.Insert(hash)
		count++
	}
	mo.index.mu.RUnlock()

	return count
}

// LookupChunk 查找 chunk：bloom filter 快速过滤 → LRU 缓存 → ChunkIndex。
// 返回 chunk 和是否找到。
func (mo *MemoryOptimizer) LookupChunk(hash string) (*Chunk, bool) {
	mo.stats.mu.Lock()
	mo.stats.TotalLookups++
	mo.stats.mu.Unlock()

	// 第一层：Bloom filter 快速判断
	if !mo.bloom.Contains(hash) {
		mo.stats.mu.Lock()
		mo.stats.BloomMisses++
		mo.stats.mu.Unlock()
		return nil, false
	}

	// 第二层：LRU 缓存
	if chunk, ok := mo.lruCache.Get(hash); ok {
		mo.stats.mu.Lock()
		mo.stats.CacheHits++
		mo.stats.mu.Unlock()
		return chunk, true
	}

	mo.stats.mu.Lock()
	mo.stats.CacheMisses++
	mo.stats.mu.Unlock()

	// 第三层：从 ChunkIndex 加载并放入缓存
	if mo.index != nil {
		mo.index.mu.RLock()
		chunk, ok := mo.index.chunks[hash]
		mo.index.mu.RUnlock()
		if ok {
			mo.lruCache.Put(hash, chunk)
			return chunk, true
		}
	}

	// 第四层：从分页加载器获取
	if chunk, ok := mo.pagedLoader.GetChunk(hash, mo.index); ok {
		mo.lruCache.Put(hash, chunk)
		return chunk, true
	}

	return nil, false
}

// InsertChunk 插入 chunk 到优化器（同时更新 bloom filter 和 LRU 缓存）。
func (mo *MemoryOptimizer) InsertChunk(hash string, chunk *Chunk) {
	mo.bloom.Insert(hash)
	mo.lruCache.Put(hash, chunk)
}

// RemoveChunk 从优化器中移除 chunk。
func (mo *MemoryOptimizer) RemoveChunk(hash string) {
	mo.lruCache.Remove(hash)
	// bloom filter 不支持删除，这是预期行为（少量 false positive 可接受）
}

// LoadPage 加载指定页的 chunk 数据。
func (mo *MemoryOptimizer) LoadPage(page int) (*PageIndex, error) {
	return mo.pagedLoader.GetPage(page, mo.index)
}

// LoadIndex 加载索引文件。
func (mo *MemoryOptimizer) LoadIndex() error {
	return mo.pagedLoader.LoadIndex()
}

// GetStats 获取优化器统计。
func (mo *MemoryOptimizer) GetStats() OptimizerSnapshot {
	return mo.stats.GetSnapshot()
}

// GetMemoryStats 获取当前内存统计。
func (mo *MemoryOptimizer) GetMemoryStats() MemoryStats {
	stats := getMemoryStats()
	mo.stats.mu.Lock()
	mo.stats.LastMemoryStats = stats
	mo.stats.mu.Unlock()
	return stats
}

// ForceGC 强制触发 GC 并返回释放的内存统计。
func (mo *MemoryOptimizer) ForceGC() MemoryStats {
	before := getMemoryStats()
	runtime.GC()
	after := getMemoryStats()

	mo.stats.mu.Lock()
	mo.stats.GCTriggers++
	mo.stats.LastGCTime = time.Now()
	mo.stats.mu.Unlock()

	_ = before // 未来可用于计算差值
	return after
}

// EvictCache 按百分比淘汰 LRU 缓存。
func (mo *MemoryOptimizer) EvictCache(percent int) int {
	if percent <= 0 {
		percent = mo.config.CacheEvictPercent
	}
	if percent > 100 {
		percent = 100
	}

	currentLen := mo.lruCache.Len()
	toEvict := currentLen * percent / 100
	if toEvict == 0 {
		return 0
	}

	// 调整 LRU 容量来触发淘汰
	newCapacity := currentLen - toEvict
	if newCapacity < 100 {
		newCapacity = 100
	}
	mo.lruCache.Resize(newCapacity)

	mo.stats.mu.Lock()
	mo.stats.EvictTriggers++
	mo.stats.LastEvictTime = time.Now()
	mo.stats.mu.Unlock()

	// 恢复原始容量
	originalCapacity := mo.config.LRUCapacity
	mo.lruCache.Resize(originalCapacity)

	return toEvict
}

// SetEvictionCallback 设置淘汰回调。
func (mo *MemoryOptimizer) SetEvictionCallback(fn func(hash string, chunk *Chunk)) {
	mo.mu.Lock()
	defer mo.mu.Unlock()
	mo.onEvict = fn
}

// monitorLoop 后台内存监控循环。
func (mo *MemoryOptimizer) monitorLoop() {
	interval := mo.config.MonitorInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-mo.stopCh:
			return
		case <-ticker.C:
			mo.checkAndOptimize()
		}
	}
}

// checkAndOptimize 检查内存使用并触发优化。
func (mo *MemoryOptimizer) checkAndOptimize() {
	memStats := getMemoryStats()

	mo.stats.mu.Lock()
	mo.stats.LastMemoryStats = memStats
	mo.stats.mu.Unlock()

	// 检查是否超过内存阈值
	thresholdMB := float64(mo.config.MemoryThresholdMB)
	if thresholdMB <= 0 {
		thresholdMB = 512
	}

	if memStats.AllocMB > thresholdMB {
		// 触发缓存淘汰
		mo.EvictCache(mo.config.CacheEvictPercent)

		// 检查堆使用率，决定是否触发 GC
		if memStats.HeapInuseMB > 0 {
			totalHeap := memStats.HeapInuseMB + memStats.HeapIdleMB
			if totalHeap > 0 {
				usagePercent := int(memStats.HeapInuseMB * 100 / totalHeap)
				if usagePercent >= mo.config.GCThresholdPercent {
					mo.ForceGC()
				}
			}
		}
	}
}

// ========== 便捷方法：集成到 Manager ==========

// WithMemoryOptimizer 为 Manager 添加内存优化器。
// 返回优化器实例，调用方需负责启停。
func (m *Manager) WithMemoryOptimizer(config *MemoryOptimizerConfig) *MemoryOptimizer {
	if config == nil {
		config = DefaultMemoryOptimizerConfig()
	}

	index := &ChunkIndex{
		chunks: m.chunks,
	}

	optimizer := NewMemoryOptimizer(config, index)

	// 用已有的 chunks 初始化布隆过滤器
	m.mu.RLock()
	for hash := range m.chunks {
		optimizer.bloom.Insert(hash)
	}
	m.mu.RUnlock()

	return optimizer
}
