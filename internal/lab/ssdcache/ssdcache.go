// Package ssdcache implements SSD read/write caching acceleration.
// Inspired by Synology SSD Cache, provides block-level read/write caching
// with LRU/LFU eviction, write-back/write-through modes, and cache monitoring.
package ssdcache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CacheMode defines the caching policy.
type CacheMode int

const (
	// ModeWriteThrough writes to both cache and backend simultaneously.
	ModeWriteThrough CacheMode = iota
	// ModeWriteBack writes to cache first, flushes to backend asynchronously.
	ModeWriteBack
)

// EvictionPolicy defines how cache entries are evicted.
type EvictionPolicy int

const (
	// PolicyLRU evicts least recently used entries.
	PolicyLRU EvictionPolicy = iota
	// PolicyLFU evicts least frequently used entries.
	PolicyLFU
)

// CacheConfig holds SSD cache configuration.
type CacheConfig struct {
	// DevicePath is the SSD device path (e.g., /dev/nvme0n1).
	DevicePath string `json:"devicePath"`
	// CacheSizeMB is the cache size in megabytes.
	CacheSizeMB int `json:"cacheSizeMB"`
	// BlockSize is the cache block size in bytes (default 4096).
	BlockSize int `json:"blockSize"`
	// Mode is the caching mode (write-through or write-back).
	Mode CacheMode `json:"mode"`
	// Policy is the eviction policy.
	Policy EvictionPolicy `json:"policy"`
	// DirtyRatio is the max dirty block ratio before forced flush (write-back mode).
	DirtyRatio float64 `json:"dirtyRatio"`
	// FlushInterval is the interval for flushing dirty blocks.
	FlushInterval time.Duration `json:"flushInterval"`
	// MaxConcurrentIO is the max concurrent I/O operations.
	MaxConcurrentIO int `json:"maxConcurrentIO"`
}

// DefaultConfig returns a default SSD cache configuration.
func DefaultConfig() CacheConfig {
	return CacheConfig{
		DevicePath:      "/dev/nvme0n1",
		CacheSizeMB:     1024,
		BlockSize:       4096,
		Mode:            ModeWriteThrough,
		Policy:          PolicyLRU,
		DirtyRatio:      0.4,
		FlushInterval:   30 * time.Second,
		MaxConcurrentIO: 32,
	}
}

// CacheEntry represents a single cache block.
type CacheEntry struct {
	Key       uint64
	Data      []byte
	Dirty     bool
	Frequency int64
	AccessAt  time.Time
	CreateAt  time.Time
	Size      int
}

// CacheStats holds runtime cache statistics.
type CacheStats struct {
	TotalBlocks   int64   `json:"totalBlocks"`
	UsedBlocks    int64   `json:"usedBlocks"`
	DirtyBlocks   int64   `json:"dirtyBlocks"`
	HitCount      int64   `json:"hitCount"`
	MissCount     int64   `json:"missCount"`
	HitRatio      float64 `json:"hitRatio"`
	ReadBytes     int64   `json:"readBytes"`
	WriteBytes    int64   `json:"writeBytes"`
	FlushBytes    int64   `json:"flushBytes"`
	EvictCount    int64   `json:"evictCount"`
	AvgReadLatUs  float64 `json:"avgReadLatencyUs"`
	AvgWriteLatUs float64 `json:"avgWriteLatencyUs"`
}

// Manager manages the SSD read/write cache.
type Manager struct {
	config    CacheConfig
	cache     map[uint64]*CacheEntry
	order     []uint64 // LRU order
	freqOrder []uint64 // LFU order
	mu        sync.RWMutex
	stats     CacheStats
	ctx       context.Context
	cancel    context.CancelFunc
	flushCh   chan struct{}
	closed    int32
	sem       chan struct{} // concurrency limiter
}

// NewManager creates a new SSD cache manager.
func NewManager(cfg CacheConfig) *Manager {
	if cfg.BlockSize <= 0 {
		cfg.BlockSize = 4096
	}
	if cfg.CacheSizeMB <= 0 {
		cfg.CacheSizeMB = 1024
	}
	if cfg.DirtyRatio <= 0 || cfg.DirtyRatio > 1.0 {
		cfg.DirtyRatio = 0.4
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 30 * time.Second
	}
	if cfg.MaxConcurrentIO <= 0 {
		cfg.MaxConcurrentIO = 32
	}

	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		config:  cfg,
		cache:   make(map[uint64]*CacheEntry),
		order:   make([]uint64, 0, cfg.CacheSizeMB*1024*1024/cfg.BlockSize),
		ctx:     ctx,
		cancel:  cancel,
		flushCh: make(chan struct{}, 1),
		sem:     make(chan struct{}, cfg.MaxConcurrentIO),
	}
	m.stats.TotalBlocks = int64(cfg.CacheSizeMB * 1024 * 1024 / cfg.BlockSize)

	// Start background flush goroutine for write-back mode.
	if cfg.Mode == ModeWriteBack {
		go m.flushLoop()
	}

	return m
}

// Read reads a block from cache. Returns nil on cache miss.
func (m *Manager) Read(key uint64) ([]byte, bool) {
	if atomic.LoadInt32(&m.closed) == 1 {
		return nil, false
	}

	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	start := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.cache[key]
	if !ok {
		atomic.AddInt64(&m.stats.MissCount, 1)
		m.updateHitRatio()
		return nil, false
	}

	// Cache hit
	entry.AccessAt = time.Now()
	atomic.AddInt64(&entry.Frequency, 1)
	atomic.AddInt64(&m.stats.HitCount, 1)
	atomic.AddInt64(&m.stats.ReadBytes, int64(len(entry.Data)))
	m.updateHitRatio()
	m.promoteEntry(key)

	latency := float64(time.Since(start).Microseconds())
	m.updateAvgLatency(&m.stats.AvgReadLatUs, latency)

	return entry.Data, true
}

// Write writes a block to cache.
func (m *Manager) Write(key uint64, data []byte) error {
	if atomic.LoadInt32(&m.closed) == 1 {
		return fmt.Errorf("cache is closed")
	}

	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	start := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we need to evict
	if _, exists := m.cache[key]; !exists {
		if int64(len(m.cache)) >= m.stats.TotalBlocks {
			m.evict()
		}
	}

	dirty := m.config.Mode == ModeWriteBack
	entry := &CacheEntry{
		Key:       key,
		Data:      data,
		Dirty:     dirty,
		Frequency: 1,
		AccessAt:  time.Now(),
		CreateAt:  time.Now(),
		Size:      len(data),
	}
	m.cache[key] = entry
	m.order = append(m.order, key)

	atomic.AddInt64(&m.stats.UsedBlocks, int64(1))
	if dirty {
		atomic.AddInt64(&m.stats.DirtyBlocks, int64(1))
	}
	atomic.AddInt64(&m.stats.WriteBytes, int64(len(data)))

	latency := float64(time.Since(start).Microseconds())
	m.updateAvgLatency(&m.stats.AvgWriteLatUs, latency)

	// Check dirty ratio for write-back mode
	if m.config.Mode == ModeWriteBack {
		dirtyRatio := float64(atomic.LoadInt64(&m.stats.DirtyBlocks)) / float64(m.stats.TotalBlocks)
		if dirtyRatio >= m.config.DirtyRatio {
			select {
			case m.flushCh <- struct{}{}:
			default:
			}
		}
	}

	return nil
}

// Invalidate removes a block from cache.
func (m *Manager) Invalidate(key uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, ok := m.cache[key]; ok {
		if entry.Dirty {
			atomic.AddInt64(&m.stats.DirtyBlocks, -1)
		}
		delete(m.cache, key)
		atomic.AddInt64(&m.stats.UsedBlocks, -1)
		m.removeFromOrder(key)
	}
}

// Flush writes all dirty blocks to backend.
func (m *Manager) Flush() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	flushed := 0
	for key, entry := range m.cache {
		if entry.Dirty {
			// In production, this would write to the backend storage.
			entry.Dirty = false
			atomic.AddInt64(&m.stats.DirtyBlocks, -1)
			atomic.AddInt64(&m.stats.FlushBytes, int64(entry.Size))
			flushed++
		}
		_ = key
	}
	return flushed
}

// GetStats returns a snapshot of cache statistics.
func (m *Manager) GetStats() CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.stats
	stats.UsedBlocks = int64(len(m.cache))
	if stats.HitCount+stats.MissCount > 0 {
		stats.HitRatio = float64(stats.HitCount) / float64(stats.HitCount+stats.MissCount)
	}
	return stats
}

// Close stops the cache manager and flushes dirty blocks.
func (m *Manager) Close() error {
	if !atomic.CompareAndSwapInt32(&m.closed, 0, 1) {
		return nil
	}
	m.cancel()
	if m.config.Mode == ModeWriteBack {
		m.Flush()
	}
	return nil
}

// evict removes the least recently/least frequently used entry.
func (m *Manager) evict() {
	if len(m.order) == 0 {
		return
	}

	var victimKey uint64
	if m.config.Policy == PolicyLFU {
		// Find least frequently used
		var minFreq int64 = 1<<63 - 1
		for k, e := range m.cache {
			if e.Frequency < minFreq {
				minFreq = e.Frequency
				victimKey = k
			}
		}
	} else {
		// LRU: evict first in order (oldest access)
		victimKey = m.order[0]
	}

	if entry, ok := m.cache[victimKey]; ok {
		if entry.Dirty {
			atomic.AddInt64(&m.stats.DirtyBlocks, -1)
		}
		delete(m.cache, victimKey)
		atomic.AddInt64(&m.stats.UsedBlocks, -1)
		atomic.AddInt64(&m.stats.EvictCount, 1)
		m.removeFromOrder(victimKey)
	}
}

// promoteEntry moves an entry to the end of LRU order (most recently used).
func (m *Manager) promoteEntry(key uint64) {
	m.removeFromOrder(key)
	m.order = append(m.order, key)
}

// removeFromOrder removes a key from the order slice.
func (m *Manager) removeFromOrder(key uint64) {
	for i, k := range m.order {
		if k == key {
			m.order = append(m.order[:i], m.order[i+1:]...)
			return
		}
	}
}

// updateHitRatio recalculates the hit ratio.
func (m *Manager) updateHitRatio() {
	hits := atomic.LoadInt64(&m.stats.HitCount)
	misses := atomic.LoadInt64(&m.stats.MissCount)
	total := hits + misses
	if total > 0 {
		m.stats.HitRatio = float64(hits) / float64(total)
	}
}

// updateAvgLatency updates the running average latency.
func (m *Manager) updateAvgLatency(avg *float64, newVal float64) {
	if *avg == 0 {
		*avg = newVal
	} else {
		*avg = (*avg*0.9 + newVal*0.1)
	}
}

// flushLoop periodically flushes dirty blocks in write-back mode.
func (m *Manager) flushLoop() {
	ticker := time.NewTicker(m.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.Flush()
		case <-m.flushCh:
			m.Flush()
		}
	}
}

// GetCacheEntries returns information about all cached entries (for monitoring).
func (m *Manager) GetCacheEntries() []CacheEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]CacheEntry, 0, len(m.cache))
	for _, e := range m.cache {
		entries = append(entries, CacheEntry{
			Key:       e.Key,
			Dirty:     e.Dirty,
			Frequency: e.Frequency,
			AccessAt:  e.AccessAt,
			CreateAt:  e.CreateAt,
			Size:      e.Size,
		})
	}
	return entries
}

// Resize dynamically resizes the cache (in blocks).
func (m *Manager) Resize(newSizeMB int) error {
	if newSizeMB <= 0 {
		return fmt.Errorf("invalid cache size: %d", newSizeMB)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	newTotalBlocks := int64(newSizeMB * 1024 * 1024 / m.config.BlockSize)
	m.config.CacheSizeMB = newSizeMB
	m.stats.TotalBlocks = newTotalBlocks

	// Evict if over capacity
	for int64(len(m.cache)) > newTotalBlocks {
		m.evict()
	}

	return nil
}
