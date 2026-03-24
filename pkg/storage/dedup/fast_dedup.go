// Package dedup 实现高性能快速去重接口，参考 TrueNAS Fast Deduplication 设计
// 基于 ZFS DDT (Deduplication Table) 架构，提供块级去重能力
package dedup

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// ========== 核心错误定义 ==========

var (
	ErrChunkNotFound      = errors.New("chunk not found in dedup table")
	ErrDuplicateEntry     = errors.New("duplicate entry in dedup table")
	ErrTableFull          = errors.New("dedup table full")
	ErrInvalidChunkSize   = errors.New("invalid chunk size")
	ErrHashMismatch       = errors.New("hash mismatch during verification")
	ErrWriteInProgress    = errors.New("write operation in progress")
	ErrDedupDisabled      = errors.New("deduplication not enabled")
	ErrInsufficientMemory = errors.New("insufficient memory for operation")
)

// ========== 数据结构定义 ==========

// ChunkHash 表示块的哈希值（256位）
type ChunkHash [32]byte

// String 返回哈希的十六进制字符串表示
func (h ChunkHash) String() string {
	return hex.EncodeToString(h[:])
}

// FromBytes 从字节切片创建 ChunkHash
func ChunkHashFromBytes(b []byte) (ChunkHash, error) {
	if len(b) != 32 {
		return ChunkHash{}, fmt.Errorf("%w: expected 32 bytes, got %d", ErrHashMismatch, len(b))
	}
	var h ChunkHash
	copy(h[:], b)
	return h, nil
}

// DDTEntry 表示 DDT 表中的一个条目
type DDTEntry struct {
	// 块哈希值
	Hash ChunkHash `json:"hash"`

	// 物理块地址（模拟）
	PhysicalBlock uint64 `json:"physicalBlock"`

	// 引用计数
	RefCount uint32 `json:"refCount"`

	// 块大小（字节）
	Size uint32 `json:"size"`

	// 压缩标志
	Compressed bool `json:"compressed"`

	// 创建时间
	CreatedAt time.Time `json:"createdAt"`

	// 最后访问时间
	LastAccess time.Time `json:"lastAccess"`

	// 校验和（用于数据完整性验证）
	Checksum uint32 `json:"checksum"`
}

// DDTStats 表示 DDT 统计信息
type DDTStats struct {
	// 总条目数
	TotalEntries uint64 `json:"totalEntries"`

	// 唯一块数
	UniqueChunks uint64 `json:"uniqueChunks"`

	// 总引用数
	TotalRefs uint64 `json:"totalRefs"`

	// 节省空间（字节）
	SavedBytes uint64 `json:"savedBytes"`

	// 内存使用（字节）
	MemoryUsage uint64 `json:"memoryUsage"`

	// 命中率
	HitRate float64 `json:"hitRate"`

	// 平均引用计数
	AvgRefCount float64 `json:"avgRefCount"`

	// 哈希冲突次数
	Collisions uint64 `json:"collisions"`

	// 最后更新时间
	LastUpdated time.Time `json:"lastUpdated"`
}

// DedupConfig 去重配置
type DedupConfig struct {
	// 是否启用去重
	Enabled bool `json:"enabled"`

	// 块大小（字节）- 支持 4KB, 8KB, 16KB, 32KB, 64KB, 128KB
	ChunkSize uint32 `json:"chunkSize"`

	// 哈希算法 - sha256, blake3, xxhash
	HashAlgorithm string `json:"hashAlgorithm"`

	// 最大内存使用（MB）
	MaxMemoryMB uint32 `json:"maxMemoryMB"`

	// 最大条目数（0 = 无限制）
	MaxEntries uint64 `json:"maxEntries"`

	// 启用压缩
	EnableCompression bool `json:"enableCompression"`

	// 验证级别 - none, onWrite, onRead, both
	VerifyLevel string `json:"verifyLevel"`

	// 自动清理间隔（小时）
	CleanupIntervalHours int `json:"cleanupIntervalHours"`

	// 引用计数阈值（低于此值触发清理）
	RefCleanupThreshold uint32 `json:"refCleanupThreshold"`

	// 启用快速路径（内存缓存）
	EnableFastPath bool `json:"enableFastPath"`

	// 快速路径缓存大小
	FastPathCacheSize uint32 `json:"fastPathCacheSize"`
}

// DefaultDedupConfig 返回默认去重配置
func DefaultDedupConfig() *DedupConfig {
	return &DedupConfig{
		Enabled:              true,
		ChunkSize:            32768, // 32KB
		HashAlgorithm:        "sha256",
		MaxMemoryMB:          1024,
		MaxEntries:           0,
		EnableCompression:    true,
		VerifyLevel:          "onWrite",
		CleanupIntervalHours: 24,
		RefCleanupThreshold:  0,
		EnableFastPath:       true,
		FastPathCacheSize:    10000,
	}
}

// Validate 验证配置
func (c *DedupConfig) Validate() error {
	validSizes := []uint32{4096, 8192, 16384, 32768, 65536, 131072}
	valid := false
	for _, s := range validSizes {
		if c.ChunkSize == s {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("%w: chunk size must be 4KB, 8KB, 16KB, 32KB, 64KB, or 128KB", ErrInvalidChunkSize)
	}

	validHashes := map[string]bool{"sha256": true, "blake3": true, "xxhash": true}
	if !validHashes[c.HashAlgorithm] {
		c.HashAlgorithm = "sha256"
	}

	validVerify := map[string]bool{"none": true, "onWrite": true, "onRead": true, "both": true}
	if !validVerify[c.VerifyLevel] {
		c.VerifyLevel = "onWrite"
	}

	return nil
}

// ========== DDT 表实现 ==========

// DDT (Deduplication Table) 快速去重表
// 采用内存哈希表 + LRU 缓存的混合架构
type DDT struct {
	mu sync.RWMutex

	// 配置
	config *DedupConfig

	// 主哈希表
	entries map[ChunkHash]*DDTEntry

	// 物理块分配器
	nextPhysicalBlock uint64

	// 统计信息
	stats DDTStats

	// 快速路径缓存（可选）
	fastPathCache *FastPathCache

	// 哈希计算器
	hasher hash.Hash

	// 关闭标志
	closed atomic.Bool

	// 后台任务取消函数
	cancelCtx context.CancelFunc
}

// FastPathCache 快速路径缓存，用于高频访问的条目
type FastPathCache struct {
	mu      sync.RWMutex
	entries map[ChunkHash]*DDTEntry
	order   []ChunkHash // LRU 顺序
	maxSize int
}

// NewFastPathCache 创建快速路径缓存
func NewFastPathCache(maxSize int) *FastPathCache {
	return &FastPathCache{
		entries: make(map[ChunkHash]*DDTEntry, maxSize),
		order:   make([]ChunkHash, 0, maxSize),
		maxSize: maxSize,
	}
}

// Get 获取缓存条目
func (c *FastPathCache) Get(hash ChunkHash) (*DDTEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[hash]
	return entry, ok
}

// Set 设置缓存条目
func (c *FastPathCache) Set(entry *DDTEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，移到队尾
	if _, ok := c.entries[entry.Hash]; ok {
		for i, h := range c.order {
			if h == entry.Hash {
				c.order = append(c.order[:i], c.order[i+1:]...)
				break
			}
		}
		c.order = append(c.order, entry.Hash)
		return
	}

	// 如果超过最大大小，移除最老的
	if len(c.entries) >= c.maxSize {
		oldest := c.order[0]
		delete(c.entries, oldest)
		c.order = c.order[1:]
	}

	c.entries[entry.Hash] = entry
	c.order = append(c.order, entry.Hash)
}

// Remove 移除缓存条目
func (c *FastPathCache) Remove(hash ChunkHash) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, hash)
	for i, h := range c.order {
		if h == hash {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// Clear 清空缓存
func (c *FastPathCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[ChunkHash]*DDTEntry, c.maxSize)
	c.order = make([]ChunkHash, 0, c.maxSize)
}

// NewDDT 创建新的 DDT 表
func NewDDT(config *DedupConfig) (*DDT, error) {
	if config == nil {
		config = DefaultDedupConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	ddt := &DDT{
		config:            config,
		entries:           make(map[ChunkHash]*DDTEntry),
		nextPhysicalBlock: 1,
		hasher:            sha256.New(),
	}

	// 初始化快速路径缓存
	if config.EnableFastPath && config.FastPathCacheSize > 0 {
		ddt.fastPathCache = NewFastPathCache(int(config.FastPathCacheSize))
	}

	// 启动后台清理任务
	if config.CleanupIntervalHours > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		ddt.cancelCtx = cancel
		go ddt.backgroundCleanup(ctx)
	}

	return ddt, nil
}

// Close 关闭 DDT 表
func (ddt *DDT) Close() error {
	ddt.closed.Store(true)
	if ddt.cancelCtx != nil {
		ddt.cancelCtx()
	}
	return nil
}

// ComputeChunkHash 计算数据块的哈希值
func (ddt *DDT) ComputeChunkHash(data []byte) ChunkHash {
	ddt.mu.RLock()
	hasher := ddt.hasher
	ddt.mu.RUnlock()

	hasher.Reset()
	hasher.Write(data)
	var hash ChunkHash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

// ComputeChunkHashWithSalt 使用盐值计算哈希（增强安全性）
func (ddt *DDT) ComputeChunkHashWithSalt(data []byte, salt uint64) ChunkHash {
	saltedData := make([]byte, len(data)+8)
	binary.BigEndian.PutUint64(saltedData[:8], salt)
	copy(saltedData[8:], data)
	return ddt.ComputeChunkHash(saltedData)
}

// Lookup 查找块哈希对应的条目
func (ddt *DDT) Lookup(hash ChunkHash) (*DDTEntry, error) {
	// 首先检查快速路径缓存
	if ddt.fastPathCache != nil {
		if entry, ok := ddt.fastPathCache.Get(hash); ok {
			return entry, nil
		}
	}

	ddt.mu.RLock()
	defer ddt.mu.RUnlock()

	entry, ok := ddt.entries[hash]
	if !ok {
		return nil, ErrChunkNotFound
	}

	// 更新访问时间
	entry.LastAccess = time.Now()

	// 更新快速路径缓存
	if ddt.fastPathCache != nil {
		ddt.fastPathCache.Set(entry)
	}

	return entry, nil
}

// Insert 插入新条目，如果存在则增加引用计数
func (ddt *DDT) Insert(hash ChunkHash, size uint32) (*DDTEntry, bool, error) {
	ddt.mu.Lock()
	defer ddt.mu.Unlock()

	// 检查是否已存在
	if entry, ok := ddt.entries[hash]; ok {
		// 增加引用计数
		entry.RefCount++
		entry.LastAccess = time.Now()

		// 更新统计
		ddt.stats.TotalRefs++
		ddt.stats.SavedBytes += uint64(size)
		ddt.stats.LastUpdated = time.Now()

		// 更新快速路径缓存
		if ddt.fastPathCache != nil {
			ddt.fastPathCache.Set(entry)
		}

		return entry, true, nil // 已存在，返回 true
	}

	// 检查最大条目数
	if ddt.config.MaxEntries > 0 && uint64(len(ddt.entries)) >= ddt.config.MaxEntries {
		return nil, false, ErrTableFull
	}

	// 创建新条目
	now := time.Now()
	entry := &DDTEntry{
		Hash:         hash,
		PhysicalBlock: ddt.nextPhysicalBlock,
		RefCount:     1,
		Size:         size,
		Compressed:   ddt.config.EnableCompression,
		CreatedAt:    now,
		LastAccess:   now,
		Checksum:     computeChecksum(hash[:]),
	}

	ddt.nextPhysicalBlock++
	ddt.entries[hash] = entry

	// 更新统计
	ddt.stats.TotalEntries++
	ddt.stats.UniqueChunks++
	ddt.stats.TotalRefs++
	ddt.stats.MemoryUsage += uint64(size)
	ddt.stats.LastUpdated = time.Now()

	// 更新快速路径缓存
	if ddt.fastPathCache != nil {
		ddt.fastPathCache.Set(entry)
	}

	return entry, false, nil // 新条目，返回 false
}

// DecrementRef 减少引用计数
func (ddt *DDT) DecrementRef(hash ChunkHash) error {
	ddt.mu.Lock()
	defer ddt.mu.Unlock()

	entry, ok := ddt.entries[hash]
	if !ok {
		return ErrChunkNotFound
	}

	if entry.RefCount > 0 {
		entry.RefCount--
		ddt.stats.TotalRefs--
		ddt.stats.SavedBytes -= uint64(entry.Size)

		// 如果引用计数为 0，可以选择保留或删除
		// 这里保留条目，由后台清理任务处理
	}

	ddt.stats.LastUpdated = time.Now()
	return nil
}

// Remove 移除条目
func (ddt *DDT) Remove(hash ChunkHash) error {
	ddt.mu.Lock()
	defer ddt.mu.Unlock()

	entry, ok := ddt.entries[hash]
	if !ok {
		return ErrChunkNotFound
	}

	delete(ddt.entries, hash)

	// 更新统计
	ddt.stats.TotalEntries--
	if entry.RefCount == 1 {
		ddt.stats.UniqueChunks--
	}
	ddt.stats.TotalRefs -= uint64(entry.RefCount)
	ddt.stats.MemoryUsage -= uint64(entry.Size)
	ddt.stats.LastUpdated = time.Now()

	// 从快速路径缓存移除
	if ddt.fastPathCache != nil {
		ddt.fastPathCache.Remove(hash)
	}

	return nil
}

// GetStats 获取统计信息
func (ddt *DDT) GetStats() DDTStats {
	ddt.mu.RLock()
	defer ddt.mu.RUnlock()

	stats := ddt.stats

	// 计算命中率和平均引用计数
	if stats.TotalRefs > 0 {
		stats.AvgRefCount = float64(stats.TotalRefs) / float64(stats.UniqueChunks)
	}
	if stats.TotalEntries > 0 {
		stats.HitRate = float64(stats.TotalRefs-stats.UniqueChunks) / float64(stats.TotalRefs)
	}

	return stats
}

// Count 获取条目数量
func (ddt *DDT) Count() uint64 {
	ddt.mu.RLock()
	defer ddt.mu.RUnlock()
	return uint64(len(ddt.entries))
}

// Clear 清空 DDT 表
func (ddt *DDT) Clear() {
	ddt.mu.Lock()
	defer ddt.mu.Unlock()

	ddt.entries = make(map[ChunkHash]*DDTEntry)
	ddt.nextPhysicalBlock = 1
	ddt.stats = DDTStats{}

	if ddt.fastPathCache != nil {
		ddt.fastPathCache.Clear()
	}
}

// backgroundCleanup 后台清理任务
func (ddt *DDT) backgroundCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(ddt.config.CleanupIntervalHours) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ddt.cleanup()
		}
	}
}

// cleanup 执行清理操作
func (ddt *DDT) cleanup() {
	ddt.mu.Lock()
	defer ddt.mu.Unlock()

	if ddt.config.RefCleanupThreshold == 0 {
		return // 不清理
	}

	// 清理引用计数低于阈值的条目
	for hash, entry := range ddt.entries {
		if entry.RefCount < ddt.config.RefCleanupThreshold {
			delete(ddt.entries, hash)

			// 更新统计
			ddt.stats.TotalEntries--
			ddt.stats.UniqueChunks--
			ddt.stats.MemoryUsage -= uint64(entry.Size)

			// 从快速路径缓存移除
			if ddt.fastPathCache != nil {
				ddt.fastPathCache.Remove(hash)
			}
		}
	}

	ddt.stats.LastUpdated = time.Now()
}

// ========== 快速去重器 ==========

// FastDeduplicator 快速去重器
// 提供高级去重 API，封装 DDT 操作
type FastDeduplicator struct {
	ddt    *DDT
	config *DedupConfig

	// 写入缓冲区
	writeBuffer *WriteBuffer

	// 统计
	totalBytesWritten  uint64
	dedupedBytes       uint64
	totalChunks        uint64
	dedupedChunks      uint64
}

// WriteBuffer 写入缓冲区，用于批量写入优化
type WriteBuffer struct {
	mu       sync.Mutex
	buffers  map[string][]byte // hash -> data
	maxSize  int
	currSize int
}

// NewWriteBuffer 创建写入缓冲区
func NewWriteBuffer(maxSize int) *WriteBuffer {
	return &WriteBuffer{
		buffers: make(map[string][]byte),
		maxSize: maxSize,
	}
}

// Add 添加数据到缓冲区
func (wb *WriteBuffer) Add(hash string, data []byte) bool {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.currSize+len(data) > wb.maxSize {
		return false
	}

	if _, ok := wb.buffers[hash]; !ok {
		wb.buffers[hash] = data
		wb.currSize += len(data)
	}
	return true
}

// Get 获取缓冲区数据
func (wb *WriteBuffer) Get(hash string) ([]byte, bool) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	data, ok := wb.buffers[hash]
	return data, ok
}

// Remove 移除缓冲区数据
func (wb *WriteBuffer) Remove(hash string) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	if data, ok := wb.buffers[hash]; ok {
		wb.currSize -= len(data)
		delete(wb.buffers, hash)
	}
}

// Clear 清空缓冲区
func (wb *WriteBuffer) Clear() {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	wb.buffers = make(map[string][]byte)
	wb.currSize = 0
}

// NewFastDeduplicator 创建快速去重器
func NewFastDeduplicator(config *DedupConfig) (*FastDeduplicator, error) {
	ddt, err := NewDDT(config)
	if err != nil {
		return nil, err
	}

	fd := &FastDeduplicator{
		ddt:    ddt,
		config: config,
	}

	// 初始化写入缓冲区（最大 64MB）
	fd.writeBuffer = NewWriteBuffer(64 * 1024 * 1024)

	return fd, nil
}

// Close 关闭去重器
func (fd *FastDeduplicator) Close() error {
	return fd.ddt.Close()
}

// DedupWrite 执行去重写入
// 返回：条目信息，是否去重成功，错误
func (fd *FastDeduplicator) DedupWrite(ctx context.Context, data []byte) (*DDTEntry, bool, error) {
	if !fd.config.Enabled {
		return nil, false, ErrDedupDisabled
	}

	// 计算哈希
	hash := fd.ddt.ComputeChunkHash(data)

	// 尝试插入
	entry, existed, err := fd.ddt.Insert(hash, uint32(len(data)))
	if err != nil {
		return nil, false, err
	}

	// 更新统计
	atomic.AddUint64(&fd.totalBytesWritten, uint64(len(data)))
	atomic.AddUint64(&fd.totalChunks, 1)

	if existed {
		atomic.AddUint64(&fd.dedupedBytes, uint64(len(data)))
		atomic.AddUint64(&fd.dedupedChunks, 1)
	}

	return entry, existed, nil
}

// DedupStream 对数据流执行去重
func (fd *FastDeduplicator) DedupStream(ctx context.Context, r io.Reader, chunkSize int) ([]*DDTEntry, uint64, error) {
	if !fd.config.Enabled {
		return nil, 0, ErrDedupDisabled
	}

	var entries []*DDTEntry
	var savedBytes uint64
	buf := make([]byte, chunkSize)

	for {
		select {
		case <-ctx.Done():
			return entries, savedBytes, ctx.Err()
		default:
		}

		n, err := r.Read(buf)
		if n == 0 {
			if err == io.EOF {
				break
			}
			if err != nil {
				return entries, savedBytes, err
			}
		}

		entry, deduped, err := fd.DedupWrite(ctx, buf[:n])
		if err != nil {
			return entries, savedBytes, err
		}

		entries = append(entries, entry)
		if deduped {
			savedBytes += uint64(n)
		}
	}

	return entries, savedBytes, nil
}

// LookupChunk 查找块
func (fd *FastDeduplicator) LookupChunk(hash ChunkHash) (*DDTEntry, error) {
	return fd.ddt.Lookup(hash)
}

// ReleaseChunk 释放块引用
func (fd *FastDeduplicator) ReleaseChunk(hash ChunkHash) error {
	return fd.ddt.DecrementRef(hash)
}

// GetStats 获取统计信息
func (fd *FastDeduplicator) GetStats() map[string]interface{} {
	ddtStats := fd.ddt.GetStats()

	return map[string]interface{}{
		"ddt":              ddtStats,
		"totalBytesWritten": atomic.LoadUint64(&fd.totalBytesWritten),
		"dedupedBytes":      atomic.LoadUint64(&fd.dedupedBytes),
		"totalChunks":       atomic.LoadUint64(&fd.totalChunks),
		"dedupedChunks":     atomic.LoadUint64(&fd.dedupedChunks),
		"dedupRatio":        float64(atomic.LoadUint64(&fd.dedupedBytes)) / float64(atomic.LoadUint64(&fd.totalBytesWritten)+1),
	}
}

// GetDDT 获取底层 DDT 表
func (fd *FastDeduplicator) GetDDT() *DDT {
	return fd.ddt
}

// ========== 辅助函数 ==========

// computeChecksum 计算简单校验和
func computeChecksum(data []byte) uint32 {
	var sum uint32
	for i, b := range data {
		sum += uint32(b) * uint32(i+1)
	}
	return sum
}

// ChunkData 将数据分块
func ChunkData(data []byte, chunkSize int) [][]byte {
	if len(data) == 0 {
		return nil
	}

	var chunks [][]byte
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[i:end])
	}

	return chunks
}

// EstimateDedupSavings 估算去重节省空间
func EstimateDedupSavings(data [][]byte, ddt *DDT) uint64 {
	var saved uint64
	seen := make(map[string]bool)

	for _, chunk := range data {
		hash := ddt.ComputeChunkHash(chunk)
		key := hash.String()
		if seen[key] {
			saved += uint64(len(chunk))
		}
		seen[key] = true
	}

	return saved
}