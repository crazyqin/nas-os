package filecache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// cacheStore 缓存存储接口.
type cacheStore interface {
	Get(key string) (*CacheEntry, bool)
	Put(key string, entry *CacheEntry)
	Delete(key string) bool
	Contains(key string) bool
	Peek(key string) (*CacheEntry, bool)
	Keys() []string
	Len() int
}

// Manager 文件缓存管理器.
type Manager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	config   *CacheConfig
	running  bool
	stopChan chan struct{}

	// 多级缓存存储
	memoryCache cacheStore
	ssdCache    cacheStore
	hddCache    cacheStore

	// 统计信息
	stats     *CacheStats
	hits      int64
	misses    int64
	evictions int64
	warmups   int64

	// 条目索引（全局）
	entryIndex map[string]*CacheEntry
}

// NewManager 创建缓存管理器.
func NewManager(logger *zap.Logger, config *CacheConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultCacheConfig()
	}
	// 防止 CleanupInterval 为 0 导致 NewTicker panic
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 10 * time.Minute
	}

	// 保护 CleanupInterval 为零值的情况
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 10 * time.Minute
	}

	m := &Manager{
		logger:     logger,
		config:     config,
		stopChan:   make(chan struct{}),
		entryIndex: make(map[string]*CacheEntry),
		stats: &CacheStats{
			LevelStats: make(map[CacheLevel]*LevelStats),
			StartedAt:  time.Now(),
		},
	}

	// 初始化多级缓存
	m.initCacheStores()

	return m
}

// initCacheStores 初始化缓存存储.
func (m *Manager) initCacheStores() {
	// 内存缓存（始终启用）
	switch m.config.Policy {
	case PolicyLRU:
		m.memoryCache = newLRUCache(m.config.MemoryMaxEntries)
	case PolicyLFU:
		m.memoryCache = newLFUCache(m.config.MemoryMaxEntries)
	case PolicyHybrid:
		m.memoryCache = newHybridCache(
			m.config.MemoryMaxEntries,
			m.config.HybridLRUWeight,
			m.config.HybridLFUWeight,
		)
	default:
		m.memoryCache = newHybridCache(
			m.config.MemoryMaxEntries,
			0.4,
			0.6,
		)
	}

	// SSD 缓存
	if m.config.SSDEnabled {
		switch m.config.Policy {
		case PolicyLRU:
			m.ssdCache = newLRUCache(m.config.MemoryMaxEntries * 10)
		case PolicyLFU:
			m.ssdCache = newLFUCache(m.config.MemoryMaxEntries * 10)
		default:
			m.ssdCache = newHybridCache(
				m.config.MemoryMaxEntries*10,
				m.config.HybridLRUWeight,
				m.config.HybridLFUWeight,
			)
		}
	}

	// HDD 缓存
	if m.config.HDDEnabled {
		switch m.config.Policy {
		case PolicyLRU:
			m.hddCache = newLRUCache(m.config.MemoryMaxEntries * 100)
		case PolicyLFU:
			m.hddCache = newLFUCache(m.config.MemoryMaxEntries * 100)
		default:
			m.hddCache = newHybridCache(
				m.config.MemoryMaxEntries*100,
				m.config.HybridLRUWeight,
				m.config.HybridLFUWeight,
			)
		}
	}

	// 初始化统计
	m.stats.LevelStats[LevelMemory] = &LevelStats{
		Level:   LevelMemory,
		MaxSize: m.config.MemoryMaxSize,
	}
	if m.config.SSDEnabled {
		m.stats.LevelStats[LevelSSD] = &LevelStats{
			Level:   LevelSSD,
			MaxSize: m.config.SSDMaxSize,
		}
	}
	if m.config.HDDEnabled {
		m.stats.LevelStats[LevelHDD] = &LevelStats{
			Level:   LevelHDD,
			MaxSize: m.config.HDDMaxSize,
		}
	}
}

// Start 启动缓存管理器.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	m.running = true
	m.stats.StartedAt = time.Now()

	// 启动清理协程
	go m.cleanupLoop()

	m.logger.Info("filecache manager started",
		zap.String("policy", string(m.config.Policy)),
		zap.Int("memory_max_entries", m.config.MemoryMaxEntries),
		zap.Bool("ssd_enabled", m.config.SSDEnabled),
		zap.Bool("hdd_enabled", m.config.HDDEnabled),
	)

	return nil
}

// Stop 停止缓存管理器.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	close(m.stopChan)

	m.logger.Info("filecache manager stopped")
	return nil
}

// IsRunning 运行状态.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// Get 获取缓存条目.
func (m *Manager) Get(key string) (*CacheEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 从内存缓存获取
	if entry, ok := m.memoryCache.Get(key); ok {
		atomic.AddInt64(&m.hits, 1)
		m.updateLevelStats(LevelMemory, true)
		return entry, true
	}

	// 从 SSD 缓存获取
	if m.ssdCache != nil {
		if entry, ok := m.ssdCache.Get(key); ok {
			atomic.AddInt64(&m.hits, 1)
			m.updateLevelStats(LevelSSD, true)
			// 提升到内存缓存
			go m.promoteToMemory(key, entry)
			return entry, true
		}
	}

	// 从 HDD 缓存获取
	if m.hddCache != nil {
		if entry, ok := m.hddCache.Get(key); ok {
			atomic.AddInt64(&m.hits, 1)
			m.updateLevelStats(LevelHDD, true)
			// 提升到 SSD 缓存
			go m.promoteToSSD(key, entry)
			return entry, true
		}
	}

	atomic.AddInt64(&m.misses, 1)
	return nil, false
}

// Put 放入缓存.
func (m *Manager) Put(key string, entry *CacheEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 设置默认值
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if entry.LastAccess.IsZero() {
		entry.LastAccess = time.Now()
	}
	if entry.Level == 0 {
		entry.Level = LevelMemory
	}

	// 根据目标层级放入
	switch entry.Level {
	case LevelMemory:
		m.memoryCache.Put(key, entry)
	case LevelSSD:
		if m.ssdCache != nil {
			m.ssdCache.Put(key, entry)
		} else {
			m.memoryCache.Put(key, entry)
		}
	case LevelHDD:
		if m.hddCache != nil {
			m.hddCache.Put(key, entry)
		} else if m.ssdCache != nil {
			m.ssdCache.Put(key, entry)
		} else {
			m.memoryCache.Put(key, entry)
		}
	default:
		m.memoryCache.Put(key, entry)
	}

	// 更新索引
	m.entryIndex[key] = entry

	m.logger.Debug("cache put",
		zap.String("key", key),
		zap.String("level", entry.Level.String()),
		zap.Int64("size", entry.Size),
	)

	return nil
}

// Delete 删除缓存条目.
func (m *Manager) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	found := m.memoryCache.Delete(key)

	if m.ssdCache != nil && m.ssdCache.Delete(key) {
		found = true
	}
	if m.hddCache != nil && m.hddCache.Delete(key) {
		found = true
	}

	delete(m.entryIndex, key)

	if !found {
		return fmt.Errorf("key %s not found", key)
	}

	m.logger.Debug("cache delete", zap.String("key", key))
	return nil
}

// promoteToMemory 提升到内存缓存.
func (m *Manager) promoteToMemory(key string, entry *CacheEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 创建新条目（内存级别）
	memEntry := *entry
	memEntry.Level = LevelMemory
	m.memoryCache.Put(key, &memEntry)
	m.entryIndex[key] = &memEntry
}

// promoteToSSD 提升到 SSD 缓存.
func (m *Manager) promoteToSSD(key string, entry *CacheEntry) {
	if m.ssdCache == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ssdEntry := *entry
	ssdEntry.Level = LevelSSD
	m.ssdCache.Put(key, &ssdEntry)
	m.entryIndex[key] = &ssdEntry
}

// updateLevelStats 更新层级统计.
func (m *Manager) updateLevelStats(level CacheLevel, hit bool) {
	stats, ok := m.stats.LevelStats[level]
	if !ok {
		return
	}

	if hit {
		stats.Hits++
	} else {
		stats.Misses++
	}

	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}
}

// Warmup 缓存预热.
func (m *Manager) Warmup(req *WarmupRequest) (*WarmupResult, error) {
	startTime := time.Now()
	result := &WarmupResult{}

	maxFiles := req.MaxFiles
	if maxFiles <= 0 {
		maxFiles = m.config.WarmupMaxFiles
	}
	maxSize := req.MaxSize
	if maxSize <= 0 {
		maxSize = m.config.WarmupMaxSize
	}

	// 目标层级
	targetLevel := LevelMemory
	switch req.Level {
	case "ssd":
		targetLevel = LevelSSD
	case "hdd":
		targetLevel = LevelHDD
	}

	var totalSize int64
	var errors []string

	for _, path := range req.Paths {
		if result.CachedFiles >= maxFiles {
			break
		}
		if totalSize >= maxSize {
			break
		}

		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() {
				return nil
			}

			if result.CachedFiles >= maxFiles {
				return filepath.SkipDir
			}
			if totalSize >= maxSize {
				return filepath.SkipDir
			}

			// 计算文件 hash
			checksum, err := m.computeFileChecksum(filePath)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", filePath, err))
				result.FailedFiles++
				return nil
			}

			// 生成缓存键
			key := m.generateKey(filePath)

			// 创建缓存条目
			entry := &CacheEntry{
				Key:        key,
				Path:       filePath,
				Size:       info.Size(),
				Level:      targetLevel,
				Checksum:   checksum,
				CreatedAt:  time.Now(),
				LastAccess: time.Now(),
			}

			// 设置 TTL
			if m.config.DefaultTTL > 0 {
				expiresAt := time.Now().Add(m.config.DefaultTTL)
				entry.ExpiresAt = &expiresAt
			}

			if err := m.Put(key, entry); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", filePath, err))
				result.FailedFiles++
				return nil
			}

			result.CachedFiles++
			result.CachedSize += info.Size()
			totalSize += info.Size()
			result.TotalFiles++

			return nil
		})

		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
		}
	}

	result.TotalSize = totalSize
	result.Duration = time.Since(startTime)
	result.Errors = errors

	atomic.AddInt64(&m.warmups, int64(result.CachedFiles))
	now := time.Now()
	m.stats.LastWarmup = &now
	m.stats.WarmupCount += int64(result.CachedFiles)

	m.logger.Info("cache warmup completed",
		zap.Int("cached_files", result.CachedFiles),
		zap.Int64("cached_size", result.CachedSize),
		zap.Int("failed_files", result.FailedFiles),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}

// Evict 缓存淘汰.
func (m *Manager) Evict(req *EvictionRequest) (*EvictionResult, error) {
	startTime := time.Now()
	result := &EvictionResult{}

	// 确定目标层级
	var targetCache cacheStore
	switch req.Level {
	case "memory":
		targetCache = m.memoryCache
	case "ssd":
		targetCache = m.ssdCache
	case "hdd":
		targetCache = m.hddCache
	default:
		// 默认淘汰内存缓存
		targetCache = m.memoryCache
	}

	if targetCache == nil {
		return nil, fmt.Errorf("cache level %s not available", req.Level)
	}

	// 获取所有键并按分数排序
	keys := targetCache.Keys()
	type keyScore struct {
		key   string
		score float64
	}

	scores := make([]keyScore, 0, len(keys))
	for _, key := range keys {
		if entry, ok := targetCache.Peek(key); ok {
			// 跳过固定的条目
			if entry.Pinned {
				continue
			}
			scores = append(scores, keyScore{key: key, score: entry.score})
		}
	}

	// 按分数升序排序（分数最低的先淘汰）
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score < scores[j].score
	})

	// 执行淘汰
	for _, ks := range scores {
		// 检查是否达到目标
		if req.TargetSize > 0 {
			var currentSize int64
			for _, key := range targetCache.Keys() {
				if entry, ok := targetCache.Peek(key); ok {
					currentSize += entry.Size
				}
			}
			if currentSize <= req.TargetSize {
				break
			}
		}

		if req.MaxEntries > 0 && targetCache.Len() <= req.MaxEntries {
			break
		}

		if !req.DryRun {
			if entry, ok := targetCache.Peek(ks.key); ok {
				result.FreedSize += entry.Size
				targetCache.Delete(ks.key)
				delete(m.entryIndex, ks.key)
			}
		} else {
			if entry, ok := targetCache.Peek(ks.key); ok {
				result.FreedSize += entry.Size
			}
		}
		result.EvictedCount++
	}

	result.Remaining = targetCache.Len()
	result.DryRun = req.DryRun
	result.Duration = time.Since(startTime)

	atomic.AddInt64(&m.evictions, int64(result.EvictedCount))
	now := time.Now()
	m.stats.LastEviction = &now
	m.stats.EvictionCount += int64(result.EvictedCount)

	m.logger.Info("cache eviction completed",
		zap.Int("evicted", result.EvictedCount),
		zap.Int64("freed_size", result.FreedSize),
		zap.Int("remaining", result.Remaining),
		zap.Bool("dry_run", req.DryRun),
	)

	return result, nil
}

// GetStats 获取缓存统计.
func (m *Manager) GetStats() *CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	stats.TotalHits = atomic.LoadInt64(&m.hits)
	stats.TotalMisses = atomic.LoadInt64(&m.misses)
	stats.EvictionCount = atomic.LoadInt64(&m.evictions)
	stats.WarmupCount = atomic.LoadInt64(&m.warmups)

	total := stats.TotalHits + stats.TotalMisses
	if total > 0 {
		stats.HitRate = float64(stats.TotalHits) / float64(total)
		stats.MissRate = float64(stats.TotalMisses) / float64(total)
	}

	// 更新层级统计
	stats.LevelStats = make(map[CacheLevel]*LevelStats)
	if ls, ok := m.stats.LevelStats[LevelMemory]; ok {
		ls.Entries = int64(m.memoryCache.Len())
		stats.LevelStats[LevelMemory] = ls
	}
	if m.ssdCache != nil {
		if ls, ok := m.stats.LevelStats[LevelSSD]; ok {
			ls.Entries = int64(m.ssdCache.Len())
			stats.LevelStats[LevelSSD] = ls
		}
	}
	if m.hddCache != nil {
		if ls, ok := m.stats.LevelStats[LevelHDD]; ok {
			ls.Entries = int64(m.hddCache.Len())
			stats.LevelStats[LevelHDD] = ls
		}
	}

	stats.TotalEntries = int64(m.memoryCache.Len())
	if m.ssdCache != nil {
		stats.TotalEntries += int64(m.ssdCache.Len())
	}
	if m.hddCache != nil {
		stats.TotalEntries += int64(m.hddCache.Len())
	}

	return &stats
}

// GetEntry 获取缓存条目信息.
func (m *Manager) GetEntry(key string) (*CacheEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.entryIndex[key]
	if !ok {
		return nil, false
	}

	entryCopy := *entry
	return &entryCopy, true
}

// ListEntries 列出缓存条目.
func (m *Manager) ListEntries(req *ListRequest) *ListResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]*CacheEntry, 0)

	// 收集所有条目
	var allKeys []string
	switch req.Level {
	case "memory":
		allKeys = m.memoryCache.Keys()
	case "ssd":
		if m.ssdCache != nil {
			allKeys = m.ssdCache.Keys()
		}
	case "hdd":
		if m.hddCache != nil {
			allKeys = m.hddCache.Keys()
		}
	default:
		allKeys = make([]string, 0)
		for key := range m.entryIndex {
			allKeys = append(allKeys, key)
		}
	}

	// 过滤
	for _, key := range allKeys {
		if req.Prefix != "" && !strings.HasPrefix(key, req.Prefix) {
			continue
		}

		entry, ok := m.entryIndex[key]
		if !ok {
			continue
		}

		entryCopy := *entry
		entries = append(entries, &entryCopy)
	}

	// 分页
	total := len(entries)
	start := req.Offset
	if start > total {
		start = total
	}
	end := start + req.Limit
	if req.Limit <= 0 || end > total {
		end = total
	}

	return &ListResponse{
		Entries: entries[start:end],
		Total:   total,
		Limit:   req.Limit,
		Offset:  req.Offset,
	}
}

// computeFileChecksum 计算文件校验和.
func (m *Manager) computeFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// generateKey 生成缓存键.
func (m *Manager) generateKey(path string) string {
	switch m.config.KeyFunc {
	case "hash":
		hash := sha256.Sum256([]byte(path))
		return hex.EncodeToString(hash[:])
	default:
		return path
	}
}

// cleanupLoop 清理循环.
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

// cleanup 清理过期条目.
func (m *Manager) cleanup() {
	if !m.config.ExpiredCheck {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	// 检查所有条目
	for key, entry := range m.entryIndex {
		if entry.ExpiresAt != nil && now.After(*entry.ExpiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// 删除过期条目
	for _, key := range expiredKeys {
		m.memoryCache.Delete(key)
		if m.ssdCache != nil {
			m.ssdCache.Delete(key)
		}
		if m.hddCache != nil {
			m.hddCache.Delete(key)
		}
		delete(m.entryIndex, key)
	}

	if len(expiredKeys) > 0 {
		m.logger.Info("cleaned up expired entries", zap.Int("count", len(expiredKeys)))
	}
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *CacheConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg *CacheConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// Clear 清空所有缓存.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空所有缓存
	for _, key := range m.memoryCache.Keys() {
		m.memoryCache.Delete(key)
	}
	if m.ssdCache != nil {
		for _, key := range m.ssdCache.Keys() {
			m.ssdCache.Delete(key)
		}
	}
	if m.hddCache != nil {
		for _, key := range m.hddCache.Keys() {
			m.hddCache.Delete(key)
		}
	}

	m.entryIndex = make(map[string]*CacheEntry)
	atomic.StoreInt64(&m.hits, 0)
	atomic.StoreInt64(&m.misses, 0)
	atomic.StoreInt64(&m.evictions, 0)

	m.logger.Info("cache cleared")
}

// Contains 检查是否包含键.
func (m *Manager) Contains(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.entryIndex[key]
	return ok
}

// Len 返回总条目数.
func (m *Manager) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entryIndex)
}
