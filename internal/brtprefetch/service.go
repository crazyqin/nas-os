package brtprefetch

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 错误定义 ==========

var (
	// ErrBRTNotFound BRT 元数据未找到.
	ErrBRTNotFound = errors.New("BRT 元数据未找到")
	// ErrEntryNotFound 条目未找到.
	ErrEntryNotFound = errors.New("BRT 条目未找到")
	// ErrEntryAlreadyExists 条目已存在.
	ErrEntryAlreadyExists = errors.New("BRT 条目已存在")
	// ErrPrefetchDisabled 预取未启用.
	ErrPrefetchDisabled = errors.New("BRT 预取未启用")
	// ErrCacheFull 缓存已满.
	ErrCacheFull = errors.New("缓存已满")
	// ErrInvalidPoolID 无效存储池 ID.
	ErrInvalidPoolID = errors.New("无效的存储池 ID")
	// ErrTaskNotFound 预取任务未找到.
	ErrTaskNotFound = errors.New("预取任务未找到")
)

// ========== 服务定义 ==========

// Service BRT 预取加速服务.
type Service struct {
	mu         sync.RWMutex
	brtData    map[string]*BRTMetadata     // poolID -> BRT 元数据
	cache      map[uint64]*CacheEntry       // blockID -> 缓存条目
	tasks      map[string]*PrefetchTask     // taskID -> 预取任务
	config     PrefetchConfig               // 预取配置
	cacheOrder []uint64                     // 缓存淘汰顺序（用于 LRU/FIFO）
	hits       uint64                       // 命中计数
	misses     uint64                       // 未命中计数
	evictions  uint64                       // 驱逐计数
	prefetches uint64                       // 预取次数
}

// NewService 创建 BRT 预取加速服务.
func NewService() *Service {
	return &Service{
		brtData: make(map[string]*BRTMetadata),
		cache:   make(map[uint64]*CacheEntry),
		tasks:   make(map[string]*PrefetchTask),
		config: PrefetchConfig{
			Enabled:         true,
			Strategy:        StrategyAdaptive,
			CachePolicy:     CachePolicyLRU,
			CacheSize:       1024,
			MaxBlockSize:    65536,
			PrefetchDepth:   8,
			TTLSeconds:      3600,
			MinRefThreshold: 1,
		},
	}
}

// ========== BRT 元数据管理 ==========

// CreateBRT 创建 BRT 元数据.
func (s *Service) CreateBRT(req BRTMetadataRequest) (*BRTMetadata, error) {
	if req.PoolID == "" {
		return nil, ErrInvalidPoolID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.brtData[req.PoolID]; exists {
		return nil, fmt.Errorf("%w: pool %s", ErrEntryAlreadyExists, req.PoolID)
	}

	now := time.Now()
	brt := &BRTMetadata{
		ID:          uuid.New().String(),
		PoolID:      req.PoolID,
		TotalBlocks: req.TotalBlocks,
		UpdatedAt:   now,
		CreatedAt:   now,
	}

	s.brtData[req.PoolID] = brt
	return brt, nil
}

// GetBRT 获取 BRT 元数据.
func (s *Service) GetBRT(poolID string) (*BRTMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	brt, ok := s.brtData[poolID]
	if !ok {
		return nil, ErrBRTNotFound
	}
	return brt, nil
}

// ListBRT 列出所有 BRT 元数据.
func (s *Service) ListBRT() []*BRTMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*BRTMetadata, 0, len(s.brtData))
	for _, brt := range s.brtData {
		result = append(result, brt)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// DeleteBRT 删除 BRT 元数据.
func (s *Service) DeleteBRT(poolID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.brtData[poolID]; !ok {
		return ErrBRTNotFound
	}
	delete(s.brtData, poolID)
	return nil
}

// AddEntry 添加 BRT 条目.
func (s *Service) AddEntry(req AddEntryRequest) (*BRTEntry, error) {
	if req.PoolID == "" {
		return nil, ErrInvalidPoolID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	brt, ok := s.brtData[req.PoolID]
	if !ok {
		// 自动创建 BRT 元数据
		now := time.Now()
		brt = &BRTMetadata{
			ID:        uuid.New().String(),
			PoolID:    req.PoolID,
			UpdatedAt: now,
			CreatedAt: now,
		}
		s.brtData[req.PoolID] = brt
	}

	// 检查是否已存在
	for _, e := range brt.Entries {
		if e.BlockID == req.BlockID {
			return nil, fmt.Errorf("%w: block %d", ErrEntryAlreadyExists, req.BlockID)
		}
	}

	now := time.Now()
	refCount := req.RefCount
	if refCount == 0 {
		refCount = 1
	}

	entry := BRTEntry{
		BlockID:      req.BlockID,
		RefCount:     refCount,
		BlockSize:    req.BlockSize,
		Checksum:     req.Checksum,
		StoragePath:  req.StoragePath,
		State:        BlockStateActive,
		LastAccessed: now,
		CreatedAt:    now,
		PoolID:       req.PoolID,
	}

	brt.Entries = append(brt.Entries, entry)
	brt.TotalBlocks++
	if refCount > 1 {
		brt.RefCounted++
		brt.SavedSpace += uint64(req.BlockSize) * uint64(refCount-1)
	}
	brt.UpdatedAt = now

	return &entry, nil
}

// GetEntry 获取 BRT 条目.
func (s *Service) GetEntry(poolID string, blockID uint64) (*BRTEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	brt, ok := s.brtData[poolID]
	if !ok {
		return nil, ErrBRTNotFound
	}

	for i := range brt.Entries {
		if brt.Entries[i].BlockID == blockID {
			return &brt.Entries[i], nil
		}
	}
	return nil, ErrEntryNotFound
}

// IncrementRef 增加块引用计数.
func (s *Service) IncrementRef(poolID string, blockID uint64) (*BRTEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	brt, ok := s.brtData[poolID]
	if !ok {
		return nil, ErrBRTNotFound
	}

	for i := range brt.Entries {
		if brt.Entries[i].BlockID == blockID {
			brt.Entries[i].RefCount++
			brt.Entries[i].LastAccessed = time.Now()
			brt.Entries[i].State = BlockStateActive
			if brt.Entries[i].RefCount > 1 {
				brt.SavedSpace += uint64(brt.Entries[i].BlockSize)
			}
			brt.RefCounted++
			brt.UpdatedAt = time.Now()
			return &brt.Entries[i], nil
		}
	}
	return nil, ErrEntryNotFound
}

// DecrementRef 减少块引用计数.
func (s *Service) DecrementRef(poolID string, blockID uint64) (*BRTEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	brt, ok := s.brtData[poolID]
	if !ok {
		return nil, ErrBRTNotFound
	}

	for i := range brt.Entries {
		if brt.Entries[i].BlockID == blockID {
			if brt.Entries[i].RefCount > 0 {
				brt.Entries[i].RefCount--
				brt.Entries[i].LastAccessed = time.Now()
				if brt.Entries[i].RefCount == 0 {
					brt.Entries[i].State = BlockStateFree
				}
				brt.UpdatedAt = time.Now()
			}
			return &brt.Entries[i], nil
		}
	}
	return nil, ErrEntryNotFound
}

// ListEntries 列出 BRT 条目.
func (s *Service) ListEntries(poolID string) ([]BRTEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	brt, ok := s.brtData[poolID]
	if !ok {
		return nil, ErrBRTNotFound
	}

	result := make([]BRTEntry, len(brt.Entries))
	copy(result, brt.Entries)
	return result, nil
}

// ========== 预取功能 ==========

// Prefetch 执行预取.
func (s *Service) Prefetch(req PrefetchRequest) (*PrefetchResponse, error) {
	if !s.config.Enabled {
		return nil, ErrPrefetchDisabled
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	brt, ok := s.brtData[req.PoolID]
	if !ok {
		return nil, ErrBRTNotFound
	}

	// 根据策略选择要预取的块
	blocksToPrefetch := s.selectBlocksForPrefetch(brt, req.BlockIDs)

	taskID := uuid.New().String()
	task := &PrefetchTask{
		ID:        taskID,
		PoolID:    req.PoolID,
		Strategy:  s.config.Strategy,
		Status:    "running",
		Blocks:     blocksToPrefetch,
		CreatedAt: time.Now(),
	}

	// 模拟执行预取
	for _, blockID := range blocksToPrefetch {
		// 找到对应条目并标记为预取
		for i := range brt.Entries {
			if brt.Entries[i].BlockID == blockID {
				brt.Entries[i].State = BlockStatePrefetched
				brt.Entries[i].LastAccessed = time.Now()
				break
			}
		}
		// 将块加入缓存
		s.prefetchToCache(blockID)
	}

	s.prefetches += uint64(len(blocksToPrefetch))

	// 标记任务完成
	now := time.Now()
	task.Status = "completed"
	task.CompletedAt = &now
	s.tasks[taskID] = task

	return &PrefetchResponse{
		TaskID:   taskID,
		BlockIDs: blocksToPrefetch,
		Status:   "completed",
		Message:  fmt.Sprintf("预取了 %d 个块", len(blocksToPrefetch)),
	}, nil
}

// selectBlocksForPrefetch 根据策略选择预取块.
func (s *Service) selectBlocksForPrefetch(brt *BRTMetadata, requested []uint64) []uint64 {
	var result []uint64

	// 先加入请求的块
	for _, bid := range requested {
		if s.isValidBlockForPrefetch(brt, bid) {
			result = append(result, bid)
		}
	}

	// 根据策略追加额外预取块
	switch s.config.Strategy {
	case StrategySequential:
		// 顺序预取：追加请求块之后的块
		for _, bid := range requested {
			for i := 1; i <= s.config.PrefetchDepth; i++ {
				nextID := bid + uint64(i)
				if s.isValidBlockForPrefetch(brt, nextID) && !contains(result, nextID) {
					result = append(result, nextID)
				}
			}
		}

	case StrategyAdaptive:
		// 自适应：根据引用计数排序，优先预取高引用块
		candidates := s.getAdaptiveCandidates(brt, requested)
		for _, c := range candidates {
			if len(result) >= s.config.PrefetchDepth {
				break
			}
			if !contains(result, c) {
				result = append(result, c)
			}
		}

	case StrategyLookahead:
		// 前瞻：基于最后访问时间预测
		candidates := s.getLookaheadCandidates(brt, requested)
		for _, c := range candidates {
			if !contains(result, c) {
				result = append(result, c)
			}
		}

	case StrategyOnDemand:
		// 按需：仅预取请求的块
		// 不追加额外块
	}

	return result
}

// isValidBlockForPrefetch 检查块是否适合预取.
func (s *Service) isValidBlockForPrefetch(brt *BRTMetadata, blockID uint64) bool {
	for i := range brt.Entries {
		if brt.Entries[i].BlockID == blockID {
			if brt.Entries[i].RefCount < s.config.MinRefThreshold {
				return false
			}
			if brt.Entries[i].BlockSize > s.config.MaxBlockSize {
				return false
			}
			return true
		}
	}
	return false
}

// getAdaptiveCandidates 获取自适应预取候选块.
func (s *Service) getAdaptiveCandidates(brt *BRTMetadata, requested []uint64) []uint64 {
	type candidate struct {
		blockID  uint64
		refCount int
		lastAccess time.Time
	}

	var candidates []candidate
	for _, entry := range brt.Entries {
		if contains(requested, entry.BlockID) {
			continue
		}
		if entry.RefCount < s.config.MinRefThreshold {
			continue
		}
		candidates = append(candidates, candidate{
			blockID:    entry.BlockID,
			refCount:  entry.RefCount,
			lastAccess: entry.LastAccessed,
		})
	}

	// 按引用计数降序排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].refCount > candidates[j].refCount
	})

	result := make([]uint64, 0, len(candidates))
	for _, c := range candidates {
		result = append(result, c.blockID)
	}
	return result
}

// getLookaheadCandidates 获取前瞻预取候选块.
func (s *Service) getLookaheadCandidates(brt *BRTMetadata, requested []uint64) []uint64 {
	type candidate struct {
		blockID    uint64
		lastAccess time.Time
	}

	var candidates []candidate
	for _, entry := range brt.Entries {
		if contains(requested, entry.BlockID) {
			continue
		}
		candidates = append(candidates, candidate{
			blockID:    entry.BlockID,
			lastAccess: entry.LastAccessed,
		})
	}

	// 按最后访问时间降序排序（最近访问的优先）
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].lastAccess.After(candidates[j].lastAccess)
	})

	result := make([]uint64, 0, len(candidates))
	maxCount := s.config.PrefetchDepth
	if maxCount > len(candidates) {
		maxCount = len(candidates)
	}
	for i := 0; i < maxCount; i++ {
		result = append(result, candidates[i].blockID)
	}
	return result
}

// ========== 缓存管理 ==========

// prefetchToCache 将块预取到缓存（模拟）.
func (s *Service) prefetchToCache(blockID uint64) {
	// 如果已存在则更新
	if entry, ok := s.cache[blockID]; ok {
		entry.LastAccess = time.Now()
		entry.HitCount++
		s.updateCacheOrder(blockID)
		return
	}

	// 检查缓存容量
	if s.config.CacheSize > 0 && len(s.cache) >= s.config.CacheSize {
		s.evictFromCache()
	}

	now := time.Now()
	ttl := time.Duration(s.config.TTLSeconds) * time.Second
	s.cache[blockID] = &CacheEntry{
		BlockID:    blockID,
		Size:       0, // 模拟数据
		HitCount:   1,
		LastAccess: now,
		CachedAt:   now,
		ExpiresAt:  now.Add(ttl),
	}
	s.cacheOrder = append(s.cacheOrder, blockID)
}

// GetBlock 从缓存获取块.
func (s *Service) GetBlock(poolID string, blockID uint64) (*CacheEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证 BRT 中存在该块
	brt, ok := s.brtData[poolID]
	if !ok {
		s.misses++
		return nil, ErrBRTNotFound
	}

	found := false
	for i := range brt.Entries {
		if brt.Entries[i].BlockID == blockID {
			found = true
			brt.Entries[i].LastAccessed = time.Now()
			break
		}
	}
	if !found {
		s.misses++
		return nil, ErrEntryNotFound
	}

	// 检查缓存
	entry, ok := s.cache[blockID]
	if !ok {
		s.misses++
		return nil, nil // 块存在但不在缓存中
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		delete(s.cache, blockID)
		s.removeFromCacheOrder(blockID)
		s.misses++
		return nil, nil
	}

	entry.LastAccess = time.Now()
	entry.HitCount++
	s.hits++
	s.updateCacheOrder(blockID)
	return entry, nil
}

// evictFromCache 根据策略驱逐缓存.
func (s *Service) evictFromCache() {
	if len(s.cacheOrder) == 0 {
		return
	}

	var evictID uint64
	switch s.config.CachePolicy {
	case CachePolicyLRU:
		// 移除最久未访问的
		evictID = s.cacheOrder[0]

	case CachePolicyFIFO:
		// 移除最先进入的
		evictID = s.cacheOrder[0]

	case CachePolicyLFU:
		// 移除访问次数最少的
		minHits := -1
		for _, bid := range s.cacheOrder {
			if entry, ok := s.cache[bid]; ok {
				if minHits == -1 || entry.HitCount < minHits {
					minHits = entry.HitCount
					evictID = bid
				}
			}
		}

	case CachePolicyTTL:
		// 移除最快过期的
		earliestExpiry := time.Time{}
		for _, bid := range s.cacheOrder {
			if entry, ok := s.cache[bid]; ok {
				if earliestExpiry.IsZero() || entry.ExpiresAt.Before(earliestExpiry) {
					earliestExpiry = entry.ExpiresAt
					evictID = bid
				}
			}
		}

	default:
		evictID = s.cacheOrder[0]
	}

	delete(s.cache, evictID)
	s.removeFromCacheOrder(evictID)
	s.evictions++
}

// updateCacheOrder 更新缓存顺序（LRU 用）.
func (s *Service) updateCacheOrder(blockID uint64) {
	s.removeFromCacheOrder(blockID)
	s.cacheOrder = append(s.cacheOrder, blockID)
}

// removeFromCacheOrder 从缓存顺序中移除.
func (s *Service) removeFromCacheOrder(blockID uint64) {
	for i, id := range s.cacheOrder {
		if id == blockID {
			s.cacheOrder = append(s.cacheOrder[:i], s.cacheOrder[i+1:]...)
			return
		}
	}
}

// ClearCache 清空缓存.
func (s *Service) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[uint64]*CacheEntry)
	s.cacheOrder = nil
}

// ========== 统计信息 ==========

// GetCacheStats 获取缓存统计.
func (s *Service) GetCacheStats() CacheStatsResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalSize := 0
	for _, entry := range s.cache {
		totalSize += entry.Size
	}

	total := s.hits + s.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(s.hits) / float64(total)
	}

	return CacheStatsResponse{
		TotalEntries:  len(s.cache),
		TotalSize:     totalSize,
		HitRate:       hitRate,
		MissRate:      1 - hitRate,
		Hits:          s.hits,
		Misses:        s.misses,
		Evictions:     s.evictions,
		PrefetchCount: s.prefetches,
	}
}

// ========== 配置管理 ==========

// GetConfig 获取预取配置.
func (s *Service) GetConfig() PrefetchConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig 更新预取配置.
func (s *Service) UpdateConfig(cfg PrefetchConfig) PrefetchConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
	if s.config.CacheSize == 0 {
		s.config.CacheSize = 1024
	}
	if s.config.PrefetchDepth == 0 {
		s.config.PrefetchDepth = 8
	}
	if s.config.TTLSeconds == 0 {
		s.config.TTLSeconds = 3600
	}
	if s.config.MaxBlockSize == 0 {
		s.config.MaxBlockSize = 65536
	}
	if s.config.MinRefThreshold == 0 {
		s.config.MinRefThreshold = 1
	}
	return s.config
}

// ========== 任务管理 ==========

// GetTask 获取预取任务.
func (s *Service) GetTask(taskID string) (*PrefetchTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有预取任务.
func (s *Service) ListTasks() []*PrefetchTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PrefetchTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		result = append(result, task)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// ========== 辅助函数 ==========

// contains 检查切片是否包含某元素.
func contains(slice []uint64, val uint64) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}